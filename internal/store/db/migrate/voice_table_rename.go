package dbmigrate

import (
	"fmt"
	"github.com/NookMux/NookMux/internal/common"
	"github.com/NookMux/NookMux/internal/store/db"
	"github.com/NookMux/NookMux/internal/store/voice"
)

const legacyMinimaxVoicesTable = "minimax_voices"

// legacyMinimaxVoiceIndexes 是旧表 minimax_voices 上的旧命名索引。
// RenameTable 不会重命名索引，需要删除后由 AutoMigrate 按新模型重建。
var legacyMinimaxVoiceIndexes = []string{
	"idx_minimax_voice_created_at",
	"idx_minimax_voice_type",
	"idx_minimax_voice_operator_id",
	"uk_minimax_voice_id",
}

// renameLegacyMinimaxVoicesTable 把旧表 minimax_voices 重命名为 voices。
// 定制音色不再绑定 MiniMax 单一供应商，表名同步去供应商化；列结构不变，仅表与索引改名。
// 必须在 AutoMigrate(&voicestore.Voice{}) 之前执行，幂等可重复运行。
func renameLegacyMinimaxVoicesTable() error {
	m := dbstore.DB.Migrator()
	if !m.HasTable(legacyMinimaxVoicesTable) {
		return nil
	}
	if m.HasTable(voicestore.Voice{}) {
		// 新旧表并存不是正常升级路径（通常是先升级、回滚旧版本、再升级）。
		// 旧表为空则清理；仍有数据则直接失败，避免静默丢弃数据。
		var cnt int64
		if err := dbstore.DB.Table(legacyMinimaxVoicesTable).Count(&cnt).Error; err != nil {
			return fmt.Errorf("检查旧表 %s 行数失败: %w", legacyMinimaxVoicesTable, err)
		}
		if cnt > 0 {
			return fmt.Errorf("表 %s 与 voices 同时存在且旧表仍有 %d 行数据，请先手动处理后再启动", legacyMinimaxVoicesTable, cnt)
		}
		if err := m.DropTable(legacyMinimaxVoicesTable); err != nil {
			return fmt.Errorf("清理空的旧表 %s 失败: %w", legacyMinimaxVoicesTable, err)
		}
		return nil
	}
	if err := m.RenameTable(legacyMinimaxVoicesTable, voicestore.Voice{}); err != nil {
		return fmt.Errorf("重命名表 %s 为 voices 失败: %w", legacyMinimaxVoicesTable, err)
	}
	for _, idx := range legacyMinimaxVoiceIndexes {
		if !m.HasIndex(voicestore.Voice{}, idx) {
			continue
		}
		if err := m.DropIndex(voicestore.Voice{}, idx); err != nil {
			return fmt.Errorf("删除旧索引 %s 失败: %w", idx, err)
		}
	}
	common.SysLog("renamed table minimax_voices to voices")
	return nil
}
