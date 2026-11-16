package logcleanup

import (
	"context"
	"fmt"
	"time"

	"github.com/NookMux/NookMux/internal/common"
	"github.com/NookMux/NookMux/internal/config/operation"
	"github.com/NookMux/NookMux/internal/domain/rankings"
	auditstore "github.com/NookMux/NookMux/internal/store/audit"
	logstore "github.com/NookMux/NookMux/internal/store/log"
	storedmediastore "github.com/NookMux/NookMux/internal/store/stored_media"
	usedatastore "github.com/NookMux/NookMux/internal/store/usedata"
)

// logCleanupBatchSize 单批删除行数上限，避免单次事务过大长时间锁表。
const logCleanupBatchSize = 100

// workerCheckInterval 后台循环的检查步长：自动清理停用时以该频率轮询配置，
// 启用时段式睡眠确保配置变更至多一个步长内生效。
const workerCheckInterval = 10 * time.Minute

// Result 一轮自动清理各目标的删除行数。
type Result struct {
	Logs         int64 `json:"logs"`
	QuotaData    int64 `json:"quota_data"`
	StoredImages int64 `json:"stored_images"`
	StoredVideos int64 `json:"stored_videos"`
	AuditLogs    int64 `json:"audit_logs"`
}

// snapshotSetting 拷贝当前日志维护配置，隔离配置热更新对执行过程的并发写。
func snapshotSetting() operation.LogCleanupSetting {
	setting := *operation.GetLogCleanupSetting()
	targets := make([]string, len(setting.Targets))
	copy(targets, setting.Targets)
	setting.Targets = targets
	return setting
}

// localMidnight 返回 t 所在时区当天 00:00。
func localMidnight(t time.Time) time.Time {
	y, m, d := t.Date()
	return time.Date(y, m, d, 0, 0, 0, 0, t.Location())
}

// NextRunTime 计算下一次清理执行时刻：从最近的自然边界起每 value 个单位步进，
// 返回第一个晚于 base 的时刻（服务器本地时区）。执行时刻对齐规则：
// day 为每日 00:00，week 为每周一 00:00，month 为每月 1 日 00:00，year 为每年 1 月 1 日 00:00。
func NextRunTime(base time.Time, value int, unit string) (time.Time, error) {
	if err := operation.ValidateLogCleanupValue(value); err != nil {
		return time.Time{}, fmt.Errorf("清理间隔无效: %w", err)
	}
	switch unit {
	case operation.LogCleanupUnitDay:
		next := localMidnight(base)
		for {
			next = next.AddDate(0, 0, value)
			if next.After(base) {
				return next, nil
			}
		}
	case operation.LogCleanupUnitWeek:
		next := localMidnight(base)
		weekday := int(next.Weekday())
		if weekday == 0 {
			weekday = 7
		}
		next = next.AddDate(0, 0, -(weekday - 1))
		for {
			next = next.AddDate(0, 0, 7*value)
			if next.After(base) {
				return next, nil
			}
		}
	case operation.LogCleanupUnitMonth:
		y, m, _ := base.Date()
		next := time.Date(y, m, 1, 0, 0, 0, 0, base.Location())
		for {
			next = next.AddDate(0, value, 0)
			if next.After(base) {
				return next, nil
			}
		}
	case operation.LogCleanupUnitYear:
		y, _, _ := base.Date()
		next := time.Date(y, 1, 1, 0, 0, 0, 0, base.Location())
		for {
			next = next.AddDate(value, 0, 0)
			if next.After(base) {
				return next, nil
			}
		}
	default:
		return time.Time{}, fmt.Errorf("清理间隔单位无效: %q（必须为 year/month/week/day）", unit)
	}
}

// RetentionCutoff 计算数据保留截止时刻：早于该时刻的数据可清理。
// now 往回推 value 个时间单位，year/month 按日历回退，week/day 按 7 天/1 天回退。
func RetentionCutoff(now time.Time, value int, unit string) (time.Time, error) {
	if err := operation.ValidateLogCleanupValue(value); err != nil {
		return time.Time{}, fmt.Errorf("数据最长保存时长无效: %w", err)
	}
	switch unit {
	case operation.LogCleanupUnitYear:
		return now.AddDate(-value, 0, 0), nil
	case operation.LogCleanupUnitMonth:
		return now.AddDate(0, -value, 0), nil
	case operation.LogCleanupUnitWeek:
		return now.AddDate(0, 0, -7*value), nil
	case operation.LogCleanupUnitDay:
		return now.AddDate(0, 0, -value), nil
	default:
		return time.Time{}, fmt.Errorf("数据最长保存时长单位无效: %q（必须为 year/month/week/day）", unit)
	}
}

// RunCleanup 执行一轮自动清理：按当前配置删除各目标中早于保留截止时刻的数据。
// 目标之间互不影响，任一目标失败记录首个错误并继续其余目标，与手动清理语义一致。
func RunCleanup(ctx context.Context) (Result, error) {
	setting := snapshotSetting()
	var result Result
	var firstErr error

	cutoff, err := RetentionCutoff(time.Now(), setting.RetentionValue, setting.RetentionUnit)
	if err != nil {
		return result, err
	}
	cutoffTimestamp := cutoff.Unix()
	common.SysLog(fmt.Sprintf("日志自动清理开始，保留截止时刻 %s，目标: %v", cutoff.Format(time.DateTime), setting.Targets))

	for _, target := range setting.Targets {
		switch target {
		case operation.LogCleanupTargetLogs:
			count, err := logstore.DeleteLogsInRange(ctx, 0, cutoffTimestamp, logCleanupBatchSize)
			if err != nil {
				common.SysError("日志自动清理 logs 失败: " + err.Error())
				if firstErr == nil {
					firstErr = err
				}
				continue
			}
			result.Logs = count
		case operation.LogCleanupTargetQuotaData:
			count, err := usedatastore.DeleteQuotaDataInRange(ctx, 0, cutoffTimestamp, logCleanupBatchSize)
			if err != nil {
				common.SysError("日志自动清理 quota_data 失败: " + err.Error())
				if firstErr == nil {
					firstErr = err
				}
				continue
			}
			result.QuotaData = count
			rankings.ClearRankingsCache()
		case operation.LogCleanupTargetStoredImages:
			count, err := storedmediastore.DeleteStoredMediaInRange(ctx, storedmediastore.MediaTypeImage, 0, cutoffTimestamp, logCleanupBatchSize)
			if err != nil {
				common.SysError("日志自动清理 stored_images 失败: " + err.Error())
				if firstErr == nil {
					firstErr = err
				}
				continue
			}
			result.StoredImages = count
		case operation.LogCleanupTargetStoredVideos:
			count, err := storedmediastore.DeleteStoredMediaInRange(ctx, storedmediastore.MediaTypeVideo, 0, cutoffTimestamp, logCleanupBatchSize)
			if err != nil {
				common.SysError("日志自动清理 stored_videos 失败: " + err.Error())
				if firstErr == nil {
					firstErr = err
				}
				continue
			}
			result.StoredVideos = count
		case operation.LogCleanupTargetAuditLogs:
			count, err := auditstore.DeleteAuditLogsInRange(0, cutoffTimestamp)
			if err != nil {
				common.SysError("日志自动清理 audit_logs 失败: " + err.Error())
				if firstErr == nil {
					firstErr = err
				}
				continue
			}
			result.AuditLogs = count
		}
	}

	common.SysLog(fmt.Sprintf("日志自动清理完成: 使用日志 %d 条, 数据看板 %d 条, 图片 %d 条, 视频 %d 条, 审计 %d 条",
		result.Logs, result.QuotaData, result.StoredImages, result.StoredVideos, result.AuditLogs))
	return result, firstErr
}

// StartWorker 启动自动清理后台循环。执行时刻由下次启动重算（无持久化游标），
// 清理按 created_at 截止删除，天然幂等，重启不会造成重复或遗漏。
// 间隔配置在等待期间变更时，至多一个检查步长内切换到新时刻。
func StartWorker() {
	go func() {
		for {
			setting := snapshotSetting()
			if !setting.AutoCleanEnabled || len(setting.Targets) == 0 {
				time.Sleep(workerCheckInterval)
				continue
			}

			next, err := NextRunTime(time.Now(), setting.IntervalValue, setting.IntervalUnit)
			if err != nil {
				common.SysError("日志自动清理暂缓: " + err.Error())
				time.Sleep(workerCheckInterval)
				continue
			}

			// 段式睡眠到执行时刻，每段醒来核对配置开关与间隔是否变更。
			waitAborted := false
			for time.Now().Before(next) {
				remain := time.Until(next)
				if remain > workerCheckInterval {
					remain = workerCheckInterval
				}
				time.Sleep(remain)

				current := snapshotSetting()
				if !current.AutoCleanEnabled || len(current.Targets) == 0 {
					waitAborted = true
					break
				}
				if current.IntervalValue != setting.IntervalValue || current.IntervalUnit != setting.IntervalUnit {
					next, err = NextRunTime(time.Now(), current.IntervalValue, current.IntervalUnit)
					if err != nil {
						common.SysError("日志自动清理暂缓: " + err.Error())
						waitAborted = true
						break
					}
					setting = current
				}
			}
			if waitAborted {
				continue
			}

			if _, err := RunCleanup(context.Background()); err != nil {
				common.SysError("日志自动清理部分目标失败: " + err.Error())
			}
		}
	}()
}
