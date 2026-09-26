package dbmigrate

import (
	"fmt"
	"github.com/NookMux/NookMux/internal/common"
	"github.com/NookMux/NookMux/internal/store/db"
	"github.com/NookMux/NookMux/internal/store/voice"
	"gorm.io/gorm"
)

const legacyMinimaxVoicesTable = "minimax_voices"

// 与 stored_media 合并批次一致的单批读取行数。
const mergeVoiceBatchSize = 100

// legacyMinimaxVoiceIndexes 是旧表 minimax_voices 上的旧命名索引。
// RenameTable 不会重命名索引，需要删除后由 AutoMigrate 按新模型重建。
var legacyMinimaxVoiceIndexes = []string{
	"idx_minimax_voice_created_at",
	"idx_minimax_voice_type",
	"idx_minimax_voice_operator_id",
	"uk_minimax_voice_id",
}

// renameLegacyMinimaxVoicesTable 把旧表 minimax_voices 的数据迁移到 voices 并删除旧表。
// 定制音色不再绑定 MiniMax 单一供应商，表名同步去供应商化；列结构不变，仅表与索引改名。
// 必须在 AutoMigrate(&voicestore.Voice{}) 之前执行，幂等可重复运行：
//   - 仅旧表存在：整表重命名，数据原样保留；
//   - 新旧表并存（升级后回滚旧版本，旧版本重新建表写入）：以 voice_id 为合并键
//     把旧表数据并入 voices，主键重新分配，完成后删除旧表；
//   - 并存的旧表为空：直接删除。
//
// 旧名索引的清理不依赖旧表存在：RenameTable 成功后若 DropIndex 中途失败，
// 下次启动时旧表已不存在，凭 voices 表上的残留旧名索引继续完成清理。
func renameLegacyMinimaxVoicesTable() error {
	m := dbstore.DB.Migrator()
	if m.HasTable(legacyMinimaxVoicesTable) {
		if m.HasTable(voicestore.Voice{}) {
			if err := mergeLegacyMinimaxVoicesIntoVoices(m); err != nil {
				return err
			}
		} else {
			if err := m.RenameTable(legacyMinimaxVoicesTable, voicestore.Voice{}); err != nil {
				return fmt.Errorf("重命名表 %s 为 voices 失败: %w", legacyMinimaxVoicesTable, err)
			}
			common.SysLog("renamed table minimax_voices to voices")
		}
	}
	// voices 表尚未创建（全新安装）时无从清理索引，交给 AutoMigrate 建表建索引。
	if !m.HasTable(voicestore.Voice{}) {
		return nil
	}
	for _, idx := range legacyMinimaxVoiceIndexes {
		if !m.HasIndex(voicestore.Voice{}, idx) {
			continue
		}
		if err := m.DropIndex(voicestore.Voice{}, idx); err != nil {
			return fmt.Errorf("删除旧索引 %s 失败: %w", idx, err)
		}
	}
	return nil
}

// mergeLegacyMinimaxVoicesIntoVoices 把旧表数据按 voice_id 去重并入 voices 后删除旧表。
// 合并键取业务唯一键 voice_id 而非主键：两表的自增 id 序列互不相关，直接搬运会
// 主键冲突；voice_id 相同即同一条音色记录，跳过即可。拷贝按 id 分批，进程中断
// 后重启可安全续跑。
func mergeLegacyMinimaxVoicesIntoVoices(m gorm.Migrator) error {
	var total int64
	if err := dbstore.DB.Table(legacyMinimaxVoicesTable).Count(&total).Error; err != nil {
		return fmt.Errorf("检查旧表 %s 行数失败: %w", legacyMinimaxVoicesTable, err)
	}
	if total == 0 {
		if err := m.DropTable(legacyMinimaxVoicesTable); err != nil {
			return fmt.Errorf("清理空的旧表 %s 失败: %w", legacyMinimaxVoicesTable, err)
		}
		return nil
	}

	var copied int64
	var cursor int64
	for {
		var rows []voicestore.Voice
		if err := dbstore.DB.Table(legacyMinimaxVoicesTable).
			Select("id", "created_at", "updated_at", "type", "operator_id", "operator_kind",
				"voice_id", "quota_cost", "redirect_id", "allowed", "remark").
			Where("id > ?", cursor).
			Order("id asc").
			Limit(mergeVoiceBatchSize).
			Find(&rows).Error; err != nil {
			return fmt.Errorf("读取旧表 %s 失败: %w", legacyMinimaxVoicesTable, err)
		}
		if len(rows) == 0 {
			break
		}

		voiceIds := make([]string, 0, len(rows))
		for i := range rows {
			voiceIds = append(voiceIds, rows[i].VoiceId)
		}
		existing, err := findExistingVoiceIds(voiceIds)
		if err != nil {
			return fmt.Errorf("查询 voices 已有音色失败（来自 %s）: %w", legacyMinimaxVoicesTable, err)
		}

		pending := make([]voicestore.Voice, 0, len(rows))
		for i := range rows {
			if _, ok := existing[rows[i].VoiceId]; ok {
				continue
			}
			// 主键由 voices 重新分配，避免旧表自增 id 与既有行冲突。
			row := rows[i]
			row.Id = 0
			pending = append(pending, row)
		}
		if len(pending) > 0 {
			if err := dbstore.DB.CreateInBatches(&pending, len(pending)).Error; err != nil {
				return fmt.Errorf("写入 voices 失败（来自 %s）: %w", legacyMinimaxVoicesTable, err)
			}
			copied += int64(len(pending))
		}
		cursor = rows[len(rows)-1].Id
	}

	// 终态校验：旧表所有行的 voice_id 都已存在于 voices，再删除旧表。
	// 表名/列名来自包内常量与模型定义，不拼接外部输入。
	var missing int64
	if err := dbstore.DB.Raw(fmt.Sprintf(
		"SELECT COUNT(1) FROM %s l WHERE NOT EXISTS (SELECT 1 FROM voices t WHERE t.voice_id = l.voice_id)",
		legacyMinimaxVoicesTable,
	)).Scan(&missing).Error; err != nil {
		return fmt.Errorf("校验旧表 %s 合并结果失败: %w", legacyMinimaxVoicesTable, err)
	}
	if missing > 0 {
		return fmt.Errorf("表 %s 仍有 %d 行未合并到 voices，已中止删除旧表", legacyMinimaxVoicesTable, missing)
	}

	if err := m.DropTable(legacyMinimaxVoicesTable); err != nil {
		return fmt.Errorf("删除旧表 %s 失败: %w", legacyMinimaxVoicesTable, err)
	}
	common.SysLog(fmt.Sprintf("merged legacy table %s into voices (%d rows migrated, %d rows already present)",
		legacyMinimaxVoicesTable, copied, total-copied))
	return nil
}

func findExistingVoiceIds(voiceIds []string) (map[string]struct{}, error) {
	var existing []string
	if err := dbstore.DB.Model(&voicestore.Voice{}).
		Where("voice_id IN ?", voiceIds).
		Pluck("voice_id", &existing).Error; err != nil {
		return nil, err
	}
	set := make(map[string]struct{}, len(existing))
	for _, id := range existing {
		set[id] = struct{}{}
	}
	return set, nil
}
