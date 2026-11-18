package operation

import (
	"fmt"

	"github.com/NookMux/NookMux/internal/config/manager"
	"github.com/NookMux/NookMux/internal/i18n"
	"github.com/NookMux/NookMux/pkg/jsonx"
)

// 日志自动清理的时间单位。执行时刻对齐到服务器本地时区的自然边界：
// day 为每日 00:00，week 为每周一 00:00，month 为每月 1 日 00:00，year 为每年 1 月 1 日 00:00。
const (
	LogCleanupUnitYear  = "year"
	LogCleanupUnitMonth = "month"
	LogCleanupUnitWeek  = "week"
	LogCleanupUnitDay   = "day"
)

// 日志清理目标对应的物理表：
// logs 为使用日志（消费/错误/充值/管理/系统），quota_data 为数据看板用量聚合，
// stored_images / stored_videos 为媒体转 URL 的缓存字节，audit_logs 为管理员操作审计记录。
const (
	LogCleanupTargetLogs         = "logs"
	LogCleanupTargetQuotaData    = "quota_data"
	LogCleanupTargetStoredImages = "stored_images"
	LogCleanupTargetStoredVideos = "stored_videos"
	LogCleanupTargetAuditLogs    = "audit_logs"
)

var (
	logCleanupUnits = map[string]struct{}{
		LogCleanupUnitYear:  {},
		LogCleanupUnitMonth: {},
		LogCleanupUnitWeek:  {},
		LogCleanupUnitDay:   {},
	}
	logCleanupTargets = map[string]struct{}{
		LogCleanupTargetLogs:         {},
		LogCleanupTargetQuotaData:    {},
		LogCleanupTargetStoredImages: {},
		LogCleanupTargetStoredVideos: {},
		LogCleanupTargetAuditLogs:    {},
	}
)

// LogCleanupSetting 日志维护设置：历史日志手动清理之外的自动清理策略。
// 数值与单位组合表示"最多保存 X 个单位"与"每 X 个单位清理一次"，X 为正整数，不做上限限制。
type LogCleanupSetting struct {
	// AutoCleanEnabled 自动清理总开关，默认关闭。
	AutoCleanEnabled bool `json:"auto_clean_enabled"`
	// RetentionValue 数据最长保留时长数值，默认 2。
	RetentionValue int `json:"retention_value"`
	// RetentionUnit 数据最长保留时长单位，默认 year（保留近两年）。
	RetentionUnit string `json:"retention_unit"`
	// IntervalValue 自动清理间隔数值，默认 1。
	IntervalValue int `json:"interval_value"`
	// IntervalUnit 自动清理间隔单位，默认 week（每周 00:00 清理）。
	IntervalUnit string `json:"interval_unit"`
	// Targets 自动清理目标集合，默认清理使用日志与数据看板聚合。
	Targets []string `json:"targets"`
}

// 默认配置
var logCleanupSetting = LogCleanupSetting{
	AutoCleanEnabled: false,
	RetentionValue:   2,
	RetentionUnit:    LogCleanupUnitYear,
	IntervalValue:    1,
	IntervalUnit:     LogCleanupUnitWeek,
	Targets:          []string{LogCleanupTargetLogs, LogCleanupTargetQuotaData},
}

func init() {
	// 注册到全局配置管理器
	manager.GlobalConfig.Register("log_cleanup_setting", &logCleanupSetting)
}

// GetLogCleanupSetting 获取日志维护设置
func GetLogCleanupSetting() *LogCleanupSetting {
	return &logCleanupSetting
}

// ValidateLogCleanupUnit 校验时间单位。
func ValidateLogCleanupUnit(unit string) error {
	if _, ok := logCleanupUnits[unit]; !ok {
		return fmt.Errorf("时间单位无效：必须为 year/month/week/day 之一，当前为 %q", unit)
	}
	return nil
}

// ValidateLogCleanupValue 校验清理时长数值：必须为正整数，不做上限限制。
func ValidateLogCleanupValue(value int) error {
	if value <= 0 {
		return fmt.Errorf("数值必须为正整数，当前为 %d", value)
	}
	return nil
}

// ValidateLogCleanupTargets 校验清理目标 JSON 数组：每个元素必须是已知目标，
// 允许为空数组（表示自动清理开启但暂不清理任何表）。
func ValidateLogCleanupTargets(raw string) error {
	var targets []string
	if err := jsonx.UnmarshalJsonStr(raw, &targets); err != nil {
		return i18n.NewErrorWithFallback(fmt.Sprintf("清理目标必须为字符串数组，解析失败: %v", err), i18n.MsgOptionLogCleanupTargetsParse, map[string]any{"Error": err.Error()}).WithCause(err)
	}
	seen := make(map[string]struct{}, len(targets))
	for _, target := range targets {
		if _, ok := logCleanupTargets[target]; !ok {
			return i18n.NewErrorWithFallback(fmt.Sprintf("清理目标无效: %q", target), i18n.MsgOptionLogCleanupTargetInvalid, map[string]any{"Target": target})
		}
		if _, dup := seen[target]; dup {
			return i18n.NewErrorWithFallback(fmt.Sprintf("清理目标重复: %q", target), i18n.MsgOptionLogCleanupTargetDuplicate, map[string]any{"Target": target})
		}
		seen[target] = struct{}{}
	}
	return nil
}
