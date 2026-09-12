package dbmigrate

import (
	"fmt"

	"github.com/NookMux/NookMux/internal/common"
	"github.com/NookMux/NookMux/internal/store/db"
	"github.com/NookMux/NookMux/internal/store/log"
	"gorm.io/gorm"
)

// legacyLogTokenAggregateColumns 是计费重构前的 logs 聚合列。billing_details
// 历史回填完成后它们不再是权威来源，必须删除（验收标准：完成迁移后删除旧列，
// 不保留旧 Token 存储和业务回退读取）。本文件是旧字段唯一允许存在的升级代码
// 之外的例外：只做结构清理，不读取列数据。
var legacyLogTokenAggregateColumns = []string{
	"prompt_tokens",
	"completion_tokens",
}

// dropLegacyLogTokenAggregateColumns 在 backfillLogBillingTokenDetails 成功
// 返回后调用：删除 logs.prompt_tokens / logs.completion_tokens 两个旧聚合列。
//
// 幂等：列不存在（全新空库或已删除）时跳过，因此每次启动都执行也安全。
// 失败必须阻断启动，不得留下"部分删除"后继续写日志。
//
// 回滚约束：删除列后不得仅切换二进制恢复服务，必须先停止新版读写，再恢复
// 迁移前的数据库备份。
func dropLegacyLogTokenAggregateColumns() error {
	if dbstore.LOG_DB == nil {
		return fmt.Errorf("log database is not initialized")
	}
	for _, col := range legacyLogTokenAggregateColumns {
		present, err := logColumnPresent(dbstore.LOG_DB.Migrator(), col)
		if err != nil {
			return fmt.Errorf("inspect legacy log column %s: %w", col, err)
		}
		if !present {
			continue
		}
		if err := dropLogColumnWithVerification(col); err != nil {
			return err
		}
		common.SysLog("dropLegacyLogTokenAggregateColumns: dropped logs." + col)
	}
	return nil
}

// dropLogColumnWithVerification 删除单列并复核。优先用原生
// ALTER TABLE ... DROP COLUMN（三库均支持，SQLite >= 3.35，本仓库内置
// 3.50；旧聚合列无索引，不受原生删列限制）；失败再退回 GORM Migrator
// （SQLite 方言按模型重建表）。每步删除后必须复核列确实消失：
// glebarez/sqlite 的表重建式 DropColumn 对 sqlite_master 中未加引号的列
// 定义（历史 ALTER TABLE ADD COLUMN 产生）解析失配时会静默不删。
func dropLogColumnWithVerification(column string) error {
	if err := retryLogBillingMigration(dbstore.LOG_DB, func(db *gorm.DB) error {
		return db.Exec("ALTER TABLE logs DROP COLUMN " + column).Error
	}); err != nil {
		common.SysError(fmt.Sprintf("native drop for logs.%s failed, falling back to gorm migrator: %v", column, err))
		if err := retryLogBillingMigration(dbstore.LOG_DB, func(db *gorm.DB) error {
			return db.Migrator().DropColumn(&logstore.Log{}, column)
		}); err != nil {
			return fmt.Errorf("drop legacy log token column %s: %w", column, err)
		}
	}
	present, err := logColumnPresent(dbstore.LOG_DB.Migrator(), column)
	if err != nil {
		return fmt.Errorf("verify legacy log column %s: %w", column, err)
	}
	if present {
		return fmt.Errorf("legacy log token column logs.%s still present after drop", column)
	}
	return nil
}

// logColumnPresent 按列名探测 logs 表的列。模型已不声明旧列，必须按原始
// 列名探测；表不存在时视为列不存在。
func logColumnPresent(migrator gorm.Migrator, column string) (bool, error) {
	if !migrator.HasTable("logs") {
		return false, nil
	}
	return migrator.HasColumn(&logstore.Log{}, column), nil
}
