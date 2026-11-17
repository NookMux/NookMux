package dbmigrate

import (
	"fmt"
	"github.com/NookMux/NookMux/internal/common"
	"github.com/NookMux/NookMux/internal/store/db"
	"github.com/NookMux/NookMux/internal/store/log"
	"github.com/NookMux/NookMux/internal/store/option"
)

// logChannelNameBackfillMarker 是一次性数据迁移标记，存储在主库 options 表。
// 删除该 options 行可强制重新执行回填（迁移本身幂等）。
const logChannelNameBackfillMarker = "data_migration.log_channel_names.v1.done"

// backfillLogChannelNames 是一次性数据迁移：为历史日志回填渠道名快照列。
//
// 渠道名快照随日志写入持久化；本迁移为升级前的历史行补齐快照：按主库
// channels 现有渠道逐个把渠道名写入其名下快照为空的日志行。渠道已被删除
// （或渠道名为空）的历史行保持空串，与运行时空快照语义一致。
//
// 幂等性：通过 options 表的 data-migration marker 保证只完整执行一次；
// 迁移按 channel_id 条件更新，仅命中快照为空的行，重复执行安全。任意
// 渠道的更新失败时不写 marker，下次启动重试。marker 存于主库 DB，日志
// 数据在 LOG_DB，二者可分离（marker 仅作全局开关）。
func backfillLogChannelNames() {
	if dbstore.LOG_DB == nil || dbstore.DB == nil {
		return
	}
	if optionstore.IsDataMigrationDone(logChannelNameBackfillMarker) {
		return
	}

	type channelNameRow struct {
		Id   int    `gorm:"column:id"`
		Name string `gorm:"column:name"`
	}
	var channels []channelNameRow
	if err := dbstore.DB.Table("channels").Select("id, name").Find(&channels).Error; err != nil {
		common.SysError(fmt.Sprintf("backfillLogChannelNames: query channels failed: %v", err))
		return
	}

	updated := int64(0)
	failed := 0
	for _, ch := range channels {
		if ch.Name == "" {
			continue
		}
		res := dbstore.LOG_DB.Model(&logstore.Log{}).
			Where("channel_id = ? AND (channel_name = '' OR channel_name IS NULL)", ch.Id).
			Update("channel_name", ch.Name)
		if res.Error != nil {
			failed++
			common.SysError(fmt.Sprintf("backfillLogChannelNames: update channel_id=%d failed: %v", ch.Id, res.Error))
			continue
		}
		updated += res.RowsAffected
	}

	// 仅当所有渠道的更新都成功时才写 marker；存在失败时让下次启动补跑。
	if failed > 0 {
		common.SysError(fmt.Sprintf("backfillLogChannelNames: finished with %d rows updated, %d channels failed; marker not set, will retry next startup", updated, failed))
		return
	}
	optionstore.MarkDataMigrationDone(logChannelNameBackfillMarker)
	common.SysLog(fmt.Sprintf("backfillLogChannelNames: completed, %d rows updated", updated))
}
