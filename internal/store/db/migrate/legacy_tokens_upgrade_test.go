package dbmigrate

import (
	"fmt"
	"strings"
	"testing"

	"github.com/NookMux/NookMux/internal/store/token"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

// TestSQLiteLegacyTokensModelLimitsUpgrade 模拟旧版本（tokens.model_limits
// 为 varchar(1024) 且默认空字符串）在 SQLite 上的升级路径：AutoMigrate 需将列
// 演进为 text 且保留既有数据。
func TestSQLiteLegacyTokensModelLimitsUpgrade(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite test db: %v", err)
	}
	// 按 89a3cafd 版本的模型定义建旧表（model_limits varchar(1024) 且默认空字符串）。
	if err := db.Exec(`CREATE TABLE tokens (
		id integer PRIMARY KEY AUTOINCREMENT,
		user_id integer, key varchar(191) DEFAULT '', status integer DEFAULT 1,
		name varchar(191) DEFAULT '', created_time bigint, accessed_time bigint,
		expired_time bigint DEFAULT -1, remain_quota integer DEFAULT 0,
		unlimited_quota numeric DEFAULT 0, model_limits_enabled numeric DEFAULT 0,
		model_limits varchar(1024) DEFAULT '',
		allow_ips varchar(191) DEFAULT '', used_quota integer DEFAULT 0,
		"group" varchar(191) DEFAULT '', deleted_at datetime,
		quota_type integer DEFAULT 0, window_hours integer DEFAULT 0,
		window_quota integer DEFAULT 0, window_start_hour integer DEFAULT 0,
		cycle_days integer DEFAULT 0, cycle_quota integer DEFAULT 0,
		window_used_quota integer DEFAULT 0, window_start_time bigint DEFAULT 0,
		cycle_used_quota integer DEFAULT 0, cycle_start_time bigint DEFAULT 0,
		cross_group_retry numeric DEFAULT 0
	)`).Error; err != nil {
		t.Fatalf("create legacy table: %v", err)
	}
	const legacyLimits = "gpt-4,claude-3-5-sonnet,gemini-1.5-pro"
	if err := db.Exec(
		`INSERT INTO tokens (user_id, key, name, model_limits) VALUES (1, 'sqlite-legacy-key', 'legacy', ?)`,
		legacyLimits,
	).Error; err != nil {
		t.Fatalf("seed legacy row: %v", err)
	}

	if err := db.AutoMigrate(&tokenstore.Token{}); err != nil {
		t.Fatalf("AutoMigrate on legacy table: %v", err)
	}

	var legacy tokenstore.Token
	if err := db.Where("key = ?", "sqlite-legacy-key").First(&legacy).Error; err != nil {
		t.Fatalf("load legacy row: %v", err)
	}
	if legacy.ModelLimits != legacyLimits {
		t.Fatalf("legacy model_limits = %q, data lost during upgrade", legacy.ModelLimits)
	}

	longLimits := strings.Repeat("model-", 300)
	longToken := &tokenstore.Token{
		UserId: 1, Key: "sqlite-long-limits-key", Name: "long-limits",
		UnlimitedQuota: true, ModelLimits: longLimits,
	}
	if err := db.Create(longToken).Error; err != nil {
		t.Fatalf("create token with >1024 chars model_limits: %v", err)
	}
	var reloaded tokenstore.Token
	if err := db.Where("key = ?", "sqlite-long-limits-key").First(&reloaded).Error; err != nil {
		t.Fatalf("reload long-limits token: %v", err)
	}
	if reloaded.ModelLimits != longLimits {
		t.Fatalf("long model_limits not persisted: got %d chars", len(reloaded.ModelLimits))
	}

	// 幂等：重复迁移不再报错。
	if err := db.AutoMigrate(&tokenstore.Token{}); err != nil {
		t.Fatalf("second AutoMigrate must be idempotent: %v", err)
	}
}
