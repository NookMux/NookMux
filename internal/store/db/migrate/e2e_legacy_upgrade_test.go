//go:build e2edb

// 端到端旧库升级验证：模拟从旧版本（tokens.model_limits 为 varchar(1024)
// 且默认空字符串）升级到当前版本的完整迁移流程。需要真实数据库连接，通过环境
// 变量注入 DSN，未设置时跳过：
//
//	E2E_MYSQL_DSN  root@tcp(127.0.0.1:33061)/e2e_upgrade?charset=utf8mb4&parseTime=True
//	E2E_PG_DSN     postgres://postgres:postgres@127.0.0.1:54321/e2e_upgrade
//
// 运行方式（配合 Docker 起库）：
//
//	./scripts/e2e-db-upgrade-check.sh
//	go test -tags e2edb ./internal/store/db/migrate/ -run TestE2E -v -count=1
//
// 测试会清空 DSN 指向 schema 中的全部数据表，切勿指向生产库。
package dbmigrate

import (
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/NookMux/NookMux/internal/common"
	infradb "github.com/NookMux/NookMux/internal/infra/db"
	"github.com/NookMux/NookMux/internal/infra/redis"
	dbstore "github.com/NookMux/NookMux/internal/store/db"
	"github.com/NookMux/NookMux/internal/store/token"
	"gorm.io/driver/mysql"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

const (
	legacyTokenKey        = "e2e-legacy-token-0000000000000000000000"
	legacyModelLimits     = "gpt-4,claude-3-5-sonnet,gemini-1.5-pro"
	longLimitsTokenKey    = "e2e-long-limits-token-000000000000000000"
	legacySharedVoiceId   = "e2e-voice-shared"
	legacyFillerVoiceId   = "e2e-voice-filler"
	legacyRollbackVoiceId = "e2e-voice-rollback-new"
)

// dropAllTables 清空当前 schema 下的全部数据表，为升级测试提供干净环境。
func dropAllTables(t *testing.T, db *gorm.DB, flavor string) {
	t.Helper()

	var query string
	switch flavor {
	case "mysql":
		query = `SELECT table_name FROM information_schema.tables WHERE table_schema = DATABASE() AND table_type = 'BASE TABLE'`
	case "postgres":
		query = `SELECT table_name FROM information_schema.tables WHERE table_schema = CURRENT_SCHEMA() AND table_type = 'BASE TABLE'`
	}
	var tables []string
	if err := db.Raw(query).Scan(&tables).Error; err != nil {
		t.Fatalf("list tables: %v", err)
	}
	for _, table := range tables {
		if err := db.Exec(fmt.Sprintf(`DROP TABLE IF EXISTS %s CASCADE`, table)).Error; err != nil {
			t.Fatalf("drop table %s: %v", table, err)
		}
	}
}

// createLegacyTokensTable 按 89a3cafd 版本的模型定义重建旧 tokens 表，
// model_limits 为 varchar(1024) 且默认空字符串，是本次升级失败的触发列。
func createLegacyTokensTable(t *testing.T, db *gorm.DB, flavor string) {
	t.Helper()

	var ddl string
	switch flavor {
	case "mysql":
		ddl = `CREATE TABLE tokens (
			id bigint NOT NULL AUTO_INCREMENT,
			user_id bigint, ` + "`key`" + ` char(48) DEFAULT '', status bigint DEFAULT 1,
			name varchar(191) DEFAULT '', created_time bigint, accessed_time bigint,
			expired_time bigint DEFAULT -1, remain_quota bigint DEFAULT 0,
			unlimited_quota tinyint(1) DEFAULT 0, model_limits_enabled tinyint(1) DEFAULT 0,
			model_limits varchar(1024) DEFAULT '',
			allow_ips varchar(191) DEFAULT '', used_quota bigint DEFAULT 0,
			` + "`group`" + ` varchar(191) DEFAULT '', deleted_at datetime DEFAULT NULL,
			PRIMARY KEY (id), UNIQUE KEY uni_tokens_key (` + "`key`" + `), KEY idx_tokens_user_id (user_id)
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`
	case "postgres":
		ddl = `CREATE TABLE tokens (
			id bigserial PRIMARY KEY,
			user_id bigint, key char(48) DEFAULT '', status bigint DEFAULT 1,
			name varchar(191) DEFAULT '', created_time bigint, accessed_time bigint,
			expired_time bigint DEFAULT -1, remain_quota bigint DEFAULT 0,
			unlimited_quota boolean DEFAULT false, model_limits_enabled boolean DEFAULT false,
			model_limits varchar(1024) DEFAULT '',
			allow_ips varchar(191) DEFAULT '', used_quota bigint DEFAULT 0,
			"group" varchar(191) DEFAULT '', deleted_at timestamptz
		)`
	}
	if err := db.Exec(ddl).Error; err != nil {
		t.Fatalf("create legacy tokens table (%s): %v", flavor, err)
	}
	// key 是保留字，按方言转义。
	keyCol := `"key"`
	if flavor == "mysql" {
		keyCol = "`key`"
	}
	if err := db.Exec(
		fmt.Sprintf(`INSERT INTO tokens (user_id, %s, name, model_limits) VALUES (?, ?, ?, ?)`, keyCol),
		1, legacyTokenKey, "legacy", legacyModelLimits,
	).Error; err != nil {
		t.Fatalf("seed legacy row: %v", err)
	}
}

// runLegacyUpgrade 在旧 schema 上执行与生产一致的完整 InitDB 迁移并断言结果。
func runLegacyUpgrade(t *testing.T, dsn, flavor string) {
	t.Helper()

	db, err := openE2EDB(dsn, flavor)
	if err != nil {
		t.Fatalf("open %s: %v", flavor, err)
	}
	dropAllTables(t, db, flavor)
	createLegacyTokensTable(t, db, flavor)

	t.Setenv("SQL_DSN", dsn)
	oldDB, oldMaster := dbstore.DB, common.IsMasterNode
	oldUsingMySQL, oldUsingPG, oldUsingSQLite := infradb.UsingMySQL, infradb.UsingPostgreSQL, infradb.UsingSQLite
	oldRedisEnabled := redis.RedisEnabled
	common.IsMasterNode = true
	// 测试进程不走 app 启动的 InitRedisClient，RedisEnabled 保持默认 true 会
	// 让异步缓存 goroutine 拿到 nil client（gopool recover 后污染日志）。
	redis.RedisEnabled = false
	t.Cleanup(func() {
		dbstore.DB, common.IsMasterNode = oldDB, oldMaster
		infradb.UsingMySQL, infradb.UsingPostgreSQL, infradb.UsingSQLite = oldUsingMySQL, oldUsingPG, oldUsingSQLite
		redis.RedisEnabled = oldRedisEnabled
	})
	if err := InitDB(); err != nil {
		t.Fatalf("InitDB on legacy %s schema failed: %v", flavor, err)
	}

	// 列类型已变为 text。
	var dataType string
	if err := dbstore.DB.Raw(
		`SELECT data_type FROM information_schema.columns WHERE table_name = 'tokens' AND column_name = 'model_limits'`,
	).Row().Scan(&dataType); err != nil {
		t.Fatalf("read model_limits type: %v", err)
	}
	if !strings.EqualFold(dataType, "text") {
		t.Fatalf("model_limits type = %s, want text", dataType)
	}

	// 旧数据行保留且值不变。
	loaded, err := tokenstore.GetTokenByKey(legacyTokenKey, true)
	if err != nil {
		t.Fatalf("load legacy token: %v", err)
	}
	if loaded.ModelLimits != legacyModelLimits {
		t.Fatalf("legacy model_limits = %q, data lost during upgrade", loaded.ModelLimits)
	}

	// 超过 1024 字符的模型限制可正常写入（列 text 化的目标场景）。
	longLimits := strings.Repeat("model-", 300)
	newToken := &tokenstore.Token{
		UserId: 1, Key: longLimitsTokenKey, Name: "long-limits",
		UnlimitedQuota: true, ModelLimits: longLimits,
	}
	if err := newToken.Insert(); err != nil {
		t.Fatalf("insert token with >1024 chars model_limits: %v", err)
	}
	reloaded, err := tokenstore.GetTokenByKey(longLimitsTokenKey, true)
	if err != nil {
		t.Fatalf("reload long-limits token: %v", err)
	}
	if reloaded.ModelLimits != longLimits {
		t.Fatalf("long model_limits not persisted: got %d chars", len(reloaded.ModelLimits))
	}

	// 幂等：重复迁移不再报错。
	if err := migrateDB(); err != nil {
		t.Fatalf("second migrateDB run must be idempotent: %v", err)
	}
}

// runFreshInstall 在空库上执行与生产一致的完整 InitDB 迁移（全新安装路径）
// 并验证 tokens 表可正常读写。
func runFreshInstall(t *testing.T, dsn, flavor string) {
	t.Helper()

	db, err := openE2EDB(dsn, flavor)
	if err != nil {
		t.Fatalf("open %s: %v", flavor, err)
	}
	dropAllTables(t, db, flavor)

	t.Setenv("SQL_DSN", dsn)
	oldDB, oldMaster := dbstore.DB, common.IsMasterNode
	oldUsingMySQL, oldUsingPG, oldUsingSQLite := infradb.UsingMySQL, infradb.UsingPostgreSQL, infradb.UsingSQLite
	oldRedisEnabled := redis.RedisEnabled
	common.IsMasterNode = true
	redis.RedisEnabled = false
	t.Cleanup(func() {
		dbstore.DB, common.IsMasterNode = oldDB, oldMaster
		infradb.UsingMySQL, infradb.UsingPostgreSQL, infradb.UsingSQLite = oldUsingMySQL, oldUsingPG, oldUsingSQLite
		redis.RedisEnabled = oldRedisEnabled
	})
	if err := InitDB(); err != nil {
		t.Fatalf("InitDB fresh install on %s failed: %v", flavor, err)
	}

	token := &tokenstore.Token{
		UserId: 1, Key: "e2e-fresh-token-000000000000000000000000", Name: "fresh",
		UnlimitedQuota: true, ModelLimits: "gpt-4,claude-3-5-sonnet",
	}
	if err := token.Insert(); err != nil {
		t.Fatalf("insert token on fresh install: %v", err)
	}
	loaded, err := tokenstore.GetTokenByKey(token.Key, true)
	if err != nil {
		t.Fatalf("load token on fresh install: %v", err)
	}
	if loaded.ModelLimits != "gpt-4,claude-3-5-sonnet" {
		t.Fatalf("model_limits not persisted on fresh install: %q", loaded.ModelLimits)
	}
}

func openE2EDB(dsn, flavor string) (*gorm.DB, error) {
	if flavor == "postgres" {
		return gorm.Open(postgres.New(postgres.Config{DSN: dsn, PreferSimpleProtocol: true}), &gorm.Config{})
	}
	return gorm.Open(mysql.Open(dsn), &gorm.Config{})
}

// createE2ELegacyMinimaxVoicesTable 按旧版模型在真实库上重建 minimax_voices，
// 模拟回滚到旧版本时旧版本 AutoMigrate 重新建表并写入的状态。主键由该表自身的
// 自增序列分配，与生产写入路径一致（不显式指定 id）。
func createE2ELegacyMinimaxVoicesTable(t *testing.T, db *gorm.DB, flavor string, voiceIds []string) {
	t.Helper()

	var ddl string
	switch flavor {
	case "mysql":
		ddl = `CREATE TABLE minimax_voices (
			id bigint NOT NULL AUTO_INCREMENT,
			created_at bigint, updated_at bigint, type varchar(16) NOT NULL,
			operator_id bigint, operator_kind varchar(16), voice_id varchar(256) NOT NULL,
			quota_cost bigint DEFAULT 0, redirect_id varchar(256), allowed tinyint(1) DEFAULT 0,
			remark varchar(255),
			PRIMARY KEY (id), UNIQUE KEY uk_minimax_voice_id (voice_id),
			KEY idx_minimax_voice_created_at (created_at), KEY idx_minimax_voice_type (type),
			KEY idx_minimax_voice_operator_id (operator_id)
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`
	case "postgres":
		ddl = `CREATE TABLE minimax_voices (
			id bigserial PRIMARY KEY,
			created_at bigint, updated_at bigint, type varchar(16) NOT NULL,
			operator_id bigint, operator_kind varchar(16), voice_id varchar(256) NOT NULL,
			quota_cost bigint DEFAULT 0, redirect_id varchar(256), allowed boolean DEFAULT false,
			remark varchar(255)
		)`
	}
	if err := db.Exec(ddl).Error; err != nil {
		t.Fatalf("create legacy minimax_voices table (%s): %v", flavor, err)
	}
	if flavor == "postgres" {
		for _, idx := range []string{
			`CREATE UNIQUE INDEX uk_minimax_voice_id ON minimax_voices (voice_id)`,
			`CREATE INDEX idx_minimax_voice_created_at ON minimax_voices (created_at)`,
			`CREATE INDEX idx_minimax_voice_type ON minimax_voices (type)`,
			`CREATE INDEX idx_minimax_voice_operator_id ON minimax_voices (operator_id)`,
		} {
			if err := db.Exec(idx).Error; err != nil {
				t.Fatalf("create legacy index (%s): %v", flavor, err)
			}
		}
	}
	for _, voiceId := range voiceIds {
		if err := db.Exec(
			`INSERT INTO minimax_voices (voice_id, type, allowed, operator_kind, created_at, updated_at)
			 VALUES (?, 'created', ?, 'admin', 1700000000, 1700000000)`,
			voiceId, true,
		).Error; err != nil {
			t.Fatalf("seed legacy voice row (%s): %v", flavor, err)
		}
	}
}

// runVoiceRollbackMerge 模拟「升级 → 回滚旧版本重建 minimax_voices 并写入 →
// 再次升级」的完整流程：两表并存时按 voice_id 去重合并、主键重新分配、旧表删除。
func runVoiceRollbackMerge(t *testing.T, dsn, flavor string) {
	t.Helper()

	db, err := openE2EDB(dsn, flavor)
	if err != nil {
		t.Fatalf("open %s: %v", flavor, err)
	}
	dropAllTables(t, db, flavor)
	// 首次升级：旧表整表重命名为 voices，voice-shared 保留。
	createE2ELegacyMinimaxVoicesTable(t, db, flavor, []string{legacySharedVoiceId})

	t.Setenv("SQL_DSN", dsn)
	oldDB, oldMaster := dbstore.DB, common.IsMasterNode
	oldUsingMySQL, oldUsingPG, oldUsingSQLite := infradb.UsingMySQL, infradb.UsingPostgreSQL, infradb.UsingSQLite
	oldRedisEnabled := redis.RedisEnabled
	common.IsMasterNode = true
	redis.RedisEnabled = false
	t.Cleanup(func() {
		dbstore.DB, common.IsMasterNode = oldDB, oldMaster
		infradb.UsingMySQL, infradb.UsingPostgreSQL, infradb.UsingSQLite = oldUsingMySQL, oldUsingPG, oldUsingSQLite
		redis.RedisEnabled = oldRedisEnabled
	})
	if err := InitDB(); err != nil {
		t.Fatalf("first InitDB (legacy rename) on %s failed: %v", flavor, err)
	}

	// 回滚前 voices 已积累第二条记录，占用旧表重建后将要撞上的主键。
	if err := db.Exec(
		`INSERT INTO voices (voice_id, type, allowed, operator_kind, created_at, updated_at)
		 VALUES (?, 'created', ?, 'admin', 1, 1)`,
		legacyFillerVoiceId, true,
	).Error; err != nil {
		t.Fatalf("seed filler voices row (%s): %v", flavor, err)
	}

	// 回滚窗口：旧版本重新建表写入。voice-shared 与 voices 重复，voice-rollback 为新增。
	createE2ELegacyMinimaxVoicesTable(t, db, flavor, []string{legacySharedVoiceId, legacyRollbackVoiceId})

	// 再次升级：合并后删除旧表，两侧数据均保留。
	if err := migrateDB(); err != nil {
		t.Fatalf("second upgrade after rollback on %s failed: %v", flavor, err)
	}
	if db.Migrator().HasTable("minimax_voices") {
		t.Fatalf("legacy table should be dropped after merge (%s)", flavor)
	}
	var cnt int64
	if err := db.Table("voices").Count(&cnt).Error; err != nil {
		t.Fatalf("count voices: %v", err)
	}
	if cnt != 3 {
		t.Fatalf("voices row count = %d, want 3 (duplicate skipped, rollback row merged)", cnt)
	}
	var rollbackId int64
	if err := db.Raw(`SELECT id FROM voices WHERE voice_id = ?`, legacyRollbackVoiceId).Scan(&rollbackId).Error; err != nil || rollbackId <= 2 {
		t.Fatalf("rollback voice should survive merge with a fresh primary key: id=%d err=%v", rollbackId, err)
	}
	var sharedCount int64
	if err := db.Raw(`SELECT COUNT(1) FROM voices WHERE voice_id = ?`, legacySharedVoiceId).Scan(&sharedCount).Error; err != nil || sharedCount != 1 {
		t.Fatalf("shared voice should exist exactly once: count=%d err=%v", sharedCount, err)
	}

	// 幂等：重复迁移不再报错。
	if err := migrateDB(); err != nil {
		t.Fatalf("third migrateDB run must be idempotent: %v", err)
	}
}

func TestE2ELegacyUpgradeMySQL(t *testing.T) {
	dsn := os.Getenv("E2E_MYSQL_DSN")
	if dsn == "" {
		t.Skip("E2E_MYSQL_DSN not set")
	}
	runLegacyUpgrade(t, dsn, "mysql")
}

func TestE2EFreshInstallMySQL(t *testing.T) {
	dsn := os.Getenv("E2E_MYSQL_DSN")
	if dsn == "" {
		t.Skip("E2E_MYSQL_DSN not set")
	}
	runFreshInstall(t, dsn, "mysql")
}

func TestE2ELegacyUpgradePostgreSQL(t *testing.T) {
	dsn := os.Getenv("E2E_PG_DSN")
	if dsn == "" {
		t.Skip("E2E_PG_DSN not set")
	}
	runLegacyUpgrade(t, dsn, "postgres")
}

func TestE2EFreshInstallPostgreSQL(t *testing.T) {
	dsn := os.Getenv("E2E_PG_DSN")
	if dsn == "" {
		t.Skip("E2E_PG_DSN not set")
	}
	runFreshInstall(t, dsn, "postgres")
}

func TestE2EVoiceRollbackMergeMySQL(t *testing.T) {
	dsn := os.Getenv("E2E_MYSQL_DSN")
	if dsn == "" {
		t.Skip("E2E_MYSQL_DSN not set")
	}
	runVoiceRollbackMerge(t, dsn, "mysql")
}

func TestE2EVoiceRollbackMergePostgreSQL(t *testing.T) {
	dsn := os.Getenv("E2E_PG_DSN")
	if dsn == "" {
		t.Skip("E2E_PG_DSN not set")
	}
	runVoiceRollbackMerge(t, dsn, "postgres")
}
