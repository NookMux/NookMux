package dbmigrate

import (
	"fmt"
	"github.com/NookMux/NookMux/internal/common"
	infradb "github.com/NookMux/NookMux/internal/infra/db"
	"github.com/NookMux/NookMux/internal/store/audit"
	"github.com/NookMux/NookMux/internal/store/channel"
	"github.com/NookMux/NookMux/internal/store/checkin"
	"github.com/NookMux/NookMux/internal/store/db"
	"github.com/NookMux/NookMux/internal/store/db/cleanup"
	"github.com/NookMux/NookMux/internal/store/log"
	"github.com/NookMux/NookMux/internal/store/minimax_voice"
	"github.com/NookMux/NookMux/internal/store/option"
	"github.com/NookMux/NookMux/internal/store/passkey"
	"github.com/NookMux/NookMux/internal/store/prefill_group"
	"github.com/NookMux/NookMux/internal/store/pricing"
	"github.com/NookMux/NookMux/internal/store/redemption"
	"github.com/NookMux/NookMux/internal/store/stored_media"
	"github.com/NookMux/NookMux/internal/store/ticket"
	"github.com/NookMux/NookMux/internal/store/token"
	"github.com/NookMux/NookMux/internal/store/topup"
	"github.com/NookMux/NookMux/internal/store/twofa"
	"github.com/NookMux/NookMux/internal/store/usedata"
	"github.com/NookMux/NookMux/internal/store/user"
	"github.com/NookMux/NookMux/internal/store/vendor_meta"
	"gorm.io/gorm"
	"os"
	"strings"
	"time"
)

// InitDB 打开主库连接并执行数据库迁移。AutoMigrate 需要引用全部资源包的
// 模型，因此迁移编排位于 dbmigrate 而非 dbstore（放 dbstore 会与资源包
// 反向依赖成环）。
func InitDB() (err error) {
	dbHandle, err := dbstore.ChooseDB("SQL_DSN", false)
	if err == nil {
		if common.DebugEnabled {
			dbHandle = dbHandle.Debug()
		}
		dbstore.DB = dbHandle
		// MySQL charset/collation startup check: ensure Chinese-capable charset
		if infradb.UsingMySQL {
			if err := dbstore.CheckMySQLChineseSupport(dbstore.DB); err != nil {
				panic(err)
			}
		}
		sqlDB, err := dbstore.DB.DB()
		if err != nil {
			return err
		}
		sqlDB.SetMaxIdleConns(common.GetEnvOrDefault("SQL_MAX_IDLE_CONNS", dbstore.DefaultSQLMaxIdleConns))
		sqlDB.SetMaxOpenConns(common.GetEnvOrDefault("SQL_MAX_OPEN_CONNS", 1000))
		sqlDB.SetConnMaxLifetime(time.Second * time.Duration(common.GetEnvOrDefault("SQL_MAX_LIFETIME", 60)))

		if !common.IsMasterNode {
			if os.Getenv("LOG_SQL_DSN") == "" {
				if err := ensureLogBillingDetailsColumn(dbstore.DB, "main"); err != nil {
					return err
				}
			}
			if err := ensureModelPriceTableTables(dbstore.DB, "main"); err != nil {
				return err
			}
			return nil
		}
		if infradb.UsingMySQL {
			//_, _ = sqlDB.Exec("ALTER TABLE channels MODIFY model_mapping TEXT;") // TODO: delete this line when most users have upgraded
		}
		common.SysLog("database migration started")
		err = migrateDB()
		return err
	} else {
		common.FatalLog(err)
	}
	return err
}

func InitLogDB() (err error) {
	if os.Getenv("LOG_SQL_DSN") == "" {
		dbstore.LOG_DB = dbstore.DB
		// 日志与主库同库：Log 表已由 migrateDB 的 AutoMigrate 处理，这里仅做数据回填。
		if common.IsMasterNode {
			backfillLogClientHeaderColumns()
			if err := backfillLogBillingTokenDetails(); err != nil {
				return err
			}
			if err := dropLegacyLogTokenAggregateColumns(); err != nil {
				return err
			}
		}
		return
	}
	dbHandle, err := dbstore.ChooseDB("LOG_SQL_DSN", true)
	if err == nil {
		if common.DebugEnabled {
			dbHandle = dbHandle.Debug()
		}
		dbstore.LOG_DB = dbHandle
		// If log DB is MySQL, also ensure Chinese-capable charset
		if infradb.LogSqlType == infradb.DatabaseTypeMySQL {
			if err := dbstore.CheckMySQLChineseSupport(dbstore.LOG_DB); err != nil {
				panic(err)
			}
		}
		sqlDB, err := dbstore.LOG_DB.DB()
		if err != nil {
			return err
		}
		sqlDB.SetMaxIdleConns(common.GetEnvOrDefault("SQL_MAX_IDLE_CONNS", dbstore.DefaultSQLMaxIdleConns))
		sqlDB.SetMaxOpenConns(common.GetEnvOrDefault("SQL_MAX_OPEN_CONNS", 1000))
		sqlDB.SetConnMaxLifetime(time.Second * time.Duration(common.GetEnvOrDefault("SQL_MAX_LIFETIME", 60)))

		if !common.IsMasterNode {
			if err := ensureLogBillingDetailsColumn(dbstore.LOG_DB, "log"); err != nil {
				return err
			}
			return nil
		}
		common.SysLog("database migration started")
		err = migrateLOGDB()
		if err != nil {
			return err
		}
		// 独立日志库：AutoMigrate 完成后回填历史日志的客户端请求头列。
		backfillLogClientHeaderColumns()
		if err := backfillLogBillingTokenDetails(); err != nil {
			return err
		}
		return dropLegacyLogTokenAggregateColumns()
	} else {
		common.FatalLog(err)
	}
	return err
}

func migrateDB() error {
	// 清理旧版唯一约束/索引，防止 GORM AutoMigrate 的 MigrateColumnUnique 报 SQLSTATE 42704。
	// 详见 cleanupPrefillGroupLegacyIndex 和 dbstore.CleanupLegacyUniqueConstraints 的注释。
	cleanupLegacyUniqueIndexes()

	// PostgreSQL：把旧库中可能漂移成 json/jsonb 的渠道 JSON-like 列改回 TEXT，
	// 避免写入空字符串/非 JSON 内容时触发 SQLSTATE 22P02。必须在 AutoMigrate 之前执行。
	cleanupLegacyChannelJSONColumns()

	// PostgreSQL：把旧版 tokens.model_limits 从 varchar(1024) 迁移为 text，
	// 避免超过 1024 字符的模型限制字符串写入失败。必须在 AutoMigrate 之前执行。
	if err := migrateTokenModelLimitsToText(); err != nil {
		return err
	}

	err := dbstore.DB.AutoMigrate(
		&channelstore.Channel{},
		&ticketstore.Ticket{},
		&ticketstore.TicketEntry{},
		&tokenstore.Token{},
		&userstore.User{},
		&passkeystore.PasskeyCredential{},
		&optionstore.Option{},
		&redemptionstore.Redemption{},
		&channelstore.Ability{},
		&logstore.Log{},
		&storedmediastore.StoredImage{},
		&storedmediastore.StoredVideo{},
		&topupstore.TopUp{},
		&usedatastore.QuotaData{},
		&vendormetastore.Model{},
		&vendormetastore.Vendor{},
		&pricingstore.ModelPricePlan{},
		&pricingstore.ModelPriceComponent{},
		&prefillgroupstore.PrefillGroup{},
		&optionstore.Setup{},
		&twofastore.TwoFA{},
		&twofastore.TwoFABackupCode{},
		&checkinstore.Checkin{},
		&channelstore.DynamicRatioRule{},
		&auditstore.AuditLog{},
		&minimaxvoicestore.MiniMaxVoice{},
	)
	if err != nil {
		return err
	}

	if err := userstore.CleanupEmptyAccessTokens(); err != nil {
		return err
	}
	if err := dbcleanup.CleanupRemovedChatPlaygroundData(); err != nil {
		return err
	}
	dbcleanup.CleanupRemovedOAuth()
	dbcleanup.CleanupRemovedTokenSetting()
	if err := dbcleanup.CleanupRemovedMultimodalTextMode(); err != nil {
		return err
	}
	dbcleanup.CleanupRemovedQuotaDataCacheStats(dbstore.DB)
	return nil
}

func migrateLOGDB() error {
	var err error
	if err = dbstore.LOG_DB.AutoMigrate(&logstore.Log{}); err != nil {
		return err
	}
	return nil
}

// cleanupLegacyUniqueIndexes 清理所有从旧版 uniqueIndex tag 迁移到新版复合/部分索引后
// 可能残留的 UNIQUE 约束和索引。仅影响 PostgreSQL。
//
// 根因：GORM AutoMigrate 的 MigrateColumnUnique 会查询 information_schema.table_constraints，
// 如果检测到列有 UNIQUE 约束但模型定义中 field.Unique 为 false（因为用的是 uniqueIndex tag），
// 就会调用 DropConstraint（不带 IF EXISTS），导致 SQLSTATE 42704。
//
// 此函数在 AutoMigrate 之前执行，动态查询并删除所有相关的旧 UNIQUE 约束和索引。
func cleanupLegacyUniqueIndexes() {
	if !infradb.UsingPostgreSQL {
		return
	}
	// PrefillGroup: 旧版 gorm:"uniqueIndex" → 新版 gorm:"uniqueIndex:uk_prefill_name,where:deleted_at IS NULL"
	dbstore.CleanupLegacyUniqueConstraints(dbstore.DB, "prefill_groups", "name", []string{"uni_prefill_groups_name", "idx_prefill_groups_name"})
	// Model: 旧版 gorm:"uniqueIndex" → 新版 gorm:"uniqueIndex:uk_model_name_delete_at,priority:1"
	dbstore.CleanupLegacyUniqueConstraints(dbstore.DB, "models", "model_name", []string{"uni_models_model_name", "idx_models_model_name"})
	// Vendor: 旧版 gorm:"uniqueIndex" → 新版 gorm:"uniqueIndex:uk_vendor_name_delete_at,priority:1"
	dbstore.CleanupLegacyUniqueConstraints(dbstore.DB, "vendors", "name", []string{"uni_vendors_name", "idx_vendors_name"})
}

// cleanupLegacyChannelJSONColumns 把 PostgreSQL 旧库中可能漂移成 json/jsonb 的渠道
// JSON-like 列改回 TEXT。当前模型意图是把这些字段当作 JSON 文本存储（TEXT），
// 这样写入空字符串或非 JSON 内容不会被 PostgreSQL 校验拒绝。
//
// 仅影响 PostgreSQL；对 MySQL/SQLite 无操作。必须在 AutoMigrate(&Channel{}) 之前执行，
// 以免 AutoMigrate 检测到类型不一致时再触发列类型变更或报错。
//
// 幂等：只对实际类型为 json/jsonb 的列执行 ALTER，列已是 text 时跳过。
func cleanupLegacyChannelJSONColumns() {
	if !infradb.UsingPostgreSQL {
		return
	}
	// 当前模型中这些字段都是 TEXT 存储意图（gorm:"type:text" 或无 JSON 类型）
	columns := []string{"other", "setting", "param_override", "header_override", "settings"}
	for _, col := range columns {
		alterChannelColumnToTextIfJSON(dbstore.DB, "channels", col)
	}
}

// alterChannelColumnToTextIfJSON 检查指定列的数据类型，若为 json/jsonb 则转为 text。
// 仅 PostgreSQL 调用。使用 information_schema 探测类型，避免硬编码 schema。
func alterChannelColumnToTextIfJSON(dbHandle *gorm.DB, tableName string, columnName string) {
	var dataType string
	err := dbHandle.Raw(
		`SELECT data_type FROM information_schema.columns WHERE table_name = ? AND column_name = ? LIMIT 1`,
		tableName, columnName,
	).Row().Scan(&dataType)
	if err != nil {
		// 表或列可能尚不存在（首次初始化），直接跳过，交给 AutoMigrate 创建。
		return
	}
	// data_type 为 'json' 或 'USER-DEFINED'（jsonb 在 information_schema 中显示为 USER-DEFINED，
	// 需进一步用 pg_attribute/format_type 精确判断）。这里同时处理两种情况。
	needsMigrate := false
	switch strings.ToLower(dataType) {
	case "json":
		needsMigrate = true
	case "user-defined":
		// 进一步用 pg_catalog.format_type 判断 jsonb
		var udtName string
		err = dbHandle.Raw(
			`SELECT format_type(a.atttypid, a.atttypmod) FROM pg_attribute a
			 JOIN pg_class c ON a.attrelid = c.oid
			 JOIN pg_namespace n ON c.relnamespace = n.oid
			 WHERE c.relname = ? AND a.attname = ? AND a.attnum > 0`,
			tableName, columnName,
		).Row().Scan(&udtName)
		if err == nil && (strings.Contains(strings.ToLower(udtName), "json")) {
			needsMigrate = true
		}
	}
	if !needsMigrate {
		return
	}
	// USING 子句把现有 JSON 值转换为 text；NULL 保持 NULL。
	// 表名/列名来自常量，不拼接外部输入，避免 SQL 注入。
	stmt := fmt.Sprintf(`ALTER TABLE %s ALTER COLUMN %s TYPE text USING %s::text`, tableName, columnName, columnName)
	if err := dbHandle.Exec(stmt).Error; err != nil {
		common.SysError(fmt.Sprintf("failed to migrate channel column %s.%s from JSON to TEXT: %v", tableName, columnName, err))
	} else {
		common.SysLog(fmt.Sprintf("migrated channel column %s.%s from JSON to TEXT", tableName, columnName))
	}
}

// migrateTokenModelLimitsToText 把 PostgreSQL 旧库中的 tokens.model_limits 列
// 从 varchar(1024) 迁移为 text。旧版定义为 varchar(1024)，当模型限制字符串
// 超过 1024 字符时 PostgreSQL 会写入失败。
//
// 仅影响 PostgreSQL；对 MySQL/SQLite 无操作。必须在 AutoMigrate(&Token{}) 之前
// 执行，以免 AutoMigrate 检测到类型不一致时再触发列类型变更或报错。
//
// 幂等：列类型已经是 text 时跳过。
func migrateTokenModelLimitsToText() error {
	if !infradb.UsingPostgreSQL {
		return nil
	}
	var dataType string
	err := dbstore.DB.Raw(
		`SELECT data_type FROM information_schema.columns WHERE table_name = ? AND column_name = ? LIMIT 1`,
		"tokens", "model_limits",
	).Row().Scan(&dataType)
	if err != nil {
		// 表或列可能尚不存在（首次初始化），直接跳过，交给 AutoMigrate 创建。
		return nil
	}
	if strings.EqualFold(dataType, "text") {
		return nil
	}
	// 表名/列名来自常量，不拼接外部输入，避免 SQL 注入。
	stmt := `ALTER TABLE tokens ALTER COLUMN model_limits TYPE text`
	if err := dbstore.DB.Exec(stmt).Error; err != nil {
		return fmt.Errorf("failed to migrate tokens.model_limits to text: %w", err)
	}
	common.SysLog("migrated tokens.model_limits to text")
	return nil
}
