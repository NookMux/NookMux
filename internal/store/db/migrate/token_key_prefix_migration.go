package dbmigrate

import (
	"fmt"

	"github.com/NookMux/NookMux/internal/common"
	"github.com/NookMux/NookMux/internal/config/operation"
	"github.com/NookMux/NookMux/internal/store/db"
	"github.com/NookMux/NookMux/internal/store/option"
	"github.com/NookMux/NookMux/internal/store/token"
)

// tokenKeyPrefixMigrationMarker 是一次性数据迁移标记，存储在主库 options 表。
// 删除该 options 行可强制重新执行迁移（迁移本身幂等）。
const tokenKeyPrefixMigrationMarker = "data_migration.token_key_prefix.v1.done"

// migrateTokenKeyPrefixes 是一次性数据迁移：为历史令牌 key 补默认密钥前缀 sk-。
//
// 背景：历史版本 tokens.key 仅存储 48 位随机串（不含前缀），鉴权时由中间件
// 剥离请求中的 sk- 后查库，展示时由前端拼回 sk-。现完整密钥（密钥前缀 +
// 随机串，前缀可在系统设置中自定义）成为唯一存储形态，鉴权与展示均直接
// 使用，历史 key 需统一补齐默认前缀。
//
// 判定依据：随机部分字符集为纯字母数字、不含 '-'，且任何合法前缀均以 '-'
// 结尾，因此「key 不含 '-'」即视为尚未迁移。
//
// 幂等性：通过 options 表 marker 保证整体只执行一次；单行更新失败时立即返回
// 错误且不写 marker，下次启动重试。已补前缀的行含 '-'，重复执行会自动跳过。
func migrateTokenKeyPrefixes() error {
	if optionstore.IsDataMigrationDone(tokenKeyPrefixMigrationMarker) {
		return nil
	}

	const batchSize = 1000
	updated := 0
	maxId := 0
	for {
		type tokenKeyRow struct {
			Id  int    `gorm:"column:id"`
			Key string `gorm:"column:key"`
		}
		var rows []tokenKeyRow
		err := dbstore.DB.Model(&tokenstore.Token{}).
			Select("id, "+dbstore.CommonKeyCol).
			Where("id > ? AND "+dbstore.CommonKeyCol+" NOT LIKE ?", maxId, "%-%").
			Order("id asc").
			Limit(batchSize).
			Find(&rows).Error
		if err != nil {
			return fmt.Errorf("migrateTokenKeyPrefixes: query batch failed (maxId=%d): %w", maxId, err)
		}
		if len(rows) == 0 {
			break
		}

		for _, row := range rows {
			if err := dbstore.DB.Model(&tokenstore.Token{}).
				Where("id = ?", row.Id).
				Update(dbstore.CommonKeyCol, operation.DefaultTokenKeyPrefix+row.Key).Error; err != nil {
				return fmt.Errorf("migrateTokenKeyPrefixes: update id=%d failed: %w", row.Id, err)
			}
			updated++
		}

		maxId = rows[len(rows)-1].Id
		if len(rows) < batchSize {
			break
		}
	}

	if updated > 0 {
		common.SysLog(fmt.Sprintf("migrateTokenKeyPrefixes: prefixed %d legacy token keys", updated))
	}
	optionstore.MarkDataMigrationDone(tokenKeyPrefixMigrationMarker)
	return nil
}
