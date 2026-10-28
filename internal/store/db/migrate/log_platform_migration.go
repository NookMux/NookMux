package dbmigrate

import (
	"fmt"
	"github.com/NookMux/NookMux/internal/common"
	"github.com/NookMux/NookMux/internal/store/db"
	"github.com/NookMux/NookMux/internal/store/log"
	"github.com/NookMux/NookMux/internal/store/option"
)

// logPlatformBackfillMarker 是一次性数据迁移标记，存储在主库 options 表。
// 删除该 options 行可强制重新执行回填（迁移本身幂等）。
const logPlatformBackfillMarker = "data_migration.log_platform.v1.done"

// backfillLogPlatforms 是一次性数据迁移：
// 用历史日志已落库的 ua / x_title / http_referer 三列识别客户端平台，
// 把结果回填到 platform 列，使「热门 Agent」等看板维度覆盖存量流量。
//
// 挂载顺序要求在 backfillLogClientHeaderColumns 之后执行，保证更早期的
// 历史行（三头尚在 Other JSON 中）已完成列回填，否则会被本迁移跳过且
// marker 落地后不再补跑。
//
// 幂等性：通过 options 表的 data-migration marker 保证只完整执行一次。
// 迁移按 id 游标分批推进、单条更新。任意批次查询失败，或任意单行更新失败时，
// 均不写 marker，下次启动会重试未完成的行（迁移本身幂等）。marker 存于
// 主库 DB，日志数据在 LOG_DB，二者可分离（marker 仅作全局开关）。
func backfillLogPlatforms() {
	if dbstore.LOG_DB == nil {
		return
	}
	if optionstore.IsDataMigrationDone(logPlatformBackfillMarker) {
		return
	}

	const batchSize = 1000
	updated := 0
	failed := 0
	maxId := 0
	for {
		type logHeaderRow struct {
			Id          int    `gorm:"column:id"`
			Ua          string `gorm:"column:ua"`
			XTitle      string `gorm:"column:x_title"`
			HttpReferer string `gorm:"column:http_referer"`
		}
		var rows []logHeaderRow
		err := dbstore.LOG_DB.Model(&logstore.Log{}).
			Select("id, ua, x_title, http_referer").
			Where("id > ? AND platform = '' AND (ua <> '-' OR x_title <> '-' OR http_referer <> '-')", maxId).
			Order("id asc").
			Limit(batchSize).
			Find(&rows).Error
		if err != nil {
			// 查询失败保守终止且不写 marker，下次启动重试。
			common.SysError(fmt.Sprintf("backfillLogPlatforms: query batch failed (maxId=%d): %v", maxId, err))
			return
		}
		if len(rows) == 0 {
			break
		}

		for _, row := range rows {
			platform := logstore.DetectPlatform(row.Ua, row.XTitle, row.HttpReferer)
			if platform == "" {
				continue
			}
			if uerr := dbstore.LOG_DB.Model(&logstore.Log{}).Where("id = ?", row.Id).
				Updates(map[string]interface{}{"platform": platform}).Error; uerr != nil {
				// 单行更新失败：计数并继续，但最终不写 marker，保证下次启动可重试。
				failed++
				common.SysError(fmt.Sprintf("backfillLogPlatforms: update id=%d failed: %v", row.Id, uerr))
				continue
			}
			updated++
		}

		maxId = rows[len(rows)-1].Id
		if len(rows) < batchSize {
			break
		}
	}

	// 仅当所有候选行都成功更新时才写 marker；存在失败时让下次启动补跑。
	if failed > 0 {
		common.SysError(fmt.Sprintf("backfillLogPlatforms: finished with %d rows updated, %d failed; marker not set, will retry next startup", updated, failed))
		return
	}
	optionstore.MarkDataMigrationDone(logPlatformBackfillMarker)
	common.SysLog(fmt.Sprintf("backfillLogPlatforms: completed, %d rows updated", updated))
}
