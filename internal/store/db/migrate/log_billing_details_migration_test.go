package dbmigrate

import (
	"database/sql"
	"fmt"
	"testing"

	"github.com/NookMux/NookMux/internal/store/db"
	"github.com/NookMux/NookMux/internal/store/log"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

// billingDetailsFixture 与 logstore 测试保持一致的 canonical JSON 样本
// （docs/PRD/计费.md 第 4 章 schema）。
const billingDetailsFixture = `{"schema_version":1,"tokens":{"input":{"text_input":12},"output":{"text_output":7,"reasoning_output":3},"cache":{"read_cache":4,"write_cache":5,"write_cache_5m":5}}}`

// setupBillingDetailsMigrateTestDB 打开独立的内存主库/日志库句柄并保存恢复
// 全局量。两个句柄分离，用于模拟 LOG_SQL_DSN 指向独立日志库的部署形态。
func setupBillingDetailsMigrateTestDB(t *testing.T) (mainDB, logDB *gorm.DB) {
	t.Helper()

	oldDB := dbstore.DB
	oldLogDB := dbstore.LOG_DB
	t.Cleanup(func() {
		dbstore.DB = oldDB
		dbstore.LOG_DB = oldLogDB
	})

	var err error
	mainDB, err = gorm.Open(sqlite.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name()+"-main")), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite main db: %v", err)
	}
	logDB, err = gorm.Open(sqlite.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name()+"-log")), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite log db: %v", err)
	}
	dbstore.DB = mainDB
	dbstore.LOG_DB = logDB
	return mainDB, logDB
}

func dropBillingDetailsColumn(t *testing.T, dbHandle *gorm.DB) {
	t.Helper()
	if err := dbHandle.Migrator().DropColumn(&logstore.Log{}, "billing_details"); err != nil {
		t.Fatalf("simulate historical schema by dropping column: %v", err)
	}
	if dbHandle.Migrator().HasColumn(&logstore.Log{}, "billing_details") {
		t.Fatal("precondition failed: billing_details should be absent after drop")
	}
}

func assertBillingDetailsNull(t *testing.T, dbHandle *gorm.DB, id int) {
	t.Helper()
	var value sql.NullString
	if err := dbHandle.Raw("SELECT billing_details FROM logs WHERE id = ?", id).Row().Scan(&value); err != nil {
		t.Fatalf("query billing_details for log id %d: %v", id, err)
	}
	if value.Valid {
		t.Fatalf("historical billing_details = %q, want NULL (no backfill)", value.String)
	}
}

// TestMigrateLOGDBAddsBillingDetailsOnHistoricalLogDB 验证独立日志库路径
// （LOG_SQL_DSN）：历史库启动时 migrateLOGDB 补列、启动数据迁移补齐
// billing_details、重复执行幂等，迁移后可写入新列。
func TestMigrateLOGDBAddsBillingDetailsOnHistoricalLogDB(t *testing.T) {
	_, logDB := setupBillingDetailsMigrateTestDB(t)

	if err := logDB.AutoMigrate(&logstore.Log{}); err != nil {
		t.Fatalf("migrate sqlite log db: %v", err)
	}
	addLegacyAggregateColumns(t, logDB)
	historical := &logstore.Log{
		UserId:           1,
		CreatedAt:        1700000000,
		Type:             logstore.LogTypeConsume,
		Username:         "legacy-user",
		ModelName:        "gpt-legacy",
		Quota:            42,
		PromptTokens:     100,
		CompletionTokens: 50,
		Group:            "default",
		Other:            `{"cache_read":100}`,
	}
	if err := logDB.Create(historical).Error; err != nil {
		t.Fatalf("seed historical log: %v", err)
	}
	seedLegacyAggregates(t, logDB, historical.Id, 100, 50)
	dropBillingDetailsColumn(t, logDB)

	for run := 1; run <= 2; run++ {
		if err := migrateLOGDB(); err != nil {
			t.Fatalf("migrateLOGDB run %d: %v", run, err)
		}
		if err := backfillLogBillingTokenDetails(); err != nil {
			t.Fatalf("backfill run %d: %v", run, err)
		}
	}
	if !logDB.Migrator().HasColumn(&logstore.Log{}, "billing_details") {
		t.Fatal("billing_details column missing after migrateLOGDB")
	}
	var stored logstore.Log
	if err := logDB.First(&stored, historical.Id).Error; err != nil {
		t.Fatalf("reload historical log: %v", err)
	}
	if stored.Quota != 42 || stored.Other != `{"cache_read":100}` {
		t.Fatalf("historical row mutated by migration: %+v", stored)
	}
	assertLegacyAggregates(t, logDB, historical.Id, 100, 50)
	if stored.BillingDetailsVersion != logstore.LogBillingDetailsVersion {
		t.Fatalf("billing_details_version = %d, want %d", stored.BillingDetailsVersion, logstore.LogBillingDetailsVersion)
	}
	payload := requireTokenDetails(t, stored.BillingDetails)
	requireTokenValue(t, payload.Tokens.Input.TextInput, 100)
	requireTokenValue(t, payload.Tokens.Output.TextOutput, 50)

	details := billingDetailsFixture
	if err := logDB.Create(&logstore.Log{
		UserId:                2,
		CreatedAt:             1700000001,
		Type:                  logstore.LogTypeConsume,
		ModelName:             "gpt-test",
		Group:                 "default",
		BillingDetails:        &details,
		BillingDetailsVersion: logstore.LogBillingDetailsVersion,
	}).Error; err != nil {
		t.Fatalf("insert log with billing_details: %v", err)
	}
	var withDetails logstore.Log
	if err := logDB.Where("user_id = ?", 2).First(&withDetails).Error; err != nil {
		t.Fatalf("reload log with billing_details: %v", err)
	}
	if withDetails.BillingDetails == nil || *withDetails.BillingDetails != billingDetailsFixture {
		t.Fatalf("billing_details = %v, want fixture JSON", withDetails.BillingDetails)
	}
}

// TestMigrateDBAddsBillingDetailsOnHistoricalMainDB 验证主库路径（同库日志）的
// 历史库启动：logs 表已存在且无新列时，完整执行 migrateDB 补列、启动数据迁移
// 补齐 billing_details，重复执行幂等。
func TestMigrateDBAddsBillingDetailsOnHistoricalMainDB(t *testing.T) {
	mainDB, _ := setupBillingDetailsMigrateTestDB(t)
	dbstore.LOG_DB = mainDB

	if err := mainDB.AutoMigrate(&logstore.Log{}); err != nil {
		t.Fatalf("migrate sqlite main db: %v", err)
	}
	addLegacyAggregateColumns(t, mainDB)
	historical := &logstore.Log{
		UserId:           1,
		CreatedAt:        1700000000,
		Type:             logstore.LogTypeConsume,
		Username:         "legacy-user",
		ModelName:        "gpt-legacy",
		Quota:            42,
		PromptTokens:     100,
		CompletionTokens: 50,
		Group:            "default",
		Other:            `{"cache_read":100}`,
	}
	if err := mainDB.Create(historical).Error; err != nil {
		t.Fatalf("seed historical log: %v", err)
	}
	seedLegacyAggregates(t, mainDB, historical.Id, 100, 50)
	dropBillingDetailsColumn(t, mainDB)

	for run := 1; run <= 2; run++ {
		if err := migrateDB(); err != nil {
			t.Fatalf("migrateDB run %d on historical main db: %v", run, err)
		}
		if err := backfillLogBillingTokenDetails(); err != nil {
			t.Fatalf("backfill run %d on historical main db: %v", run, err)
		}
	}
	if !mainDB.Migrator().HasColumn(&logstore.Log{}, "billing_details") {
		t.Fatal("billing_details column missing after migrateDB on historical main db")
	}
	var stored logstore.Log
	if err := mainDB.First(&stored, historical.Id).Error; err != nil {
		t.Fatalf("reload historical log: %v", err)
	}
	if stored.Quota != 42 || stored.Other != `{"cache_read":100}` {
		t.Fatalf("historical row mutated by migration: %+v", stored)
	}
	assertLegacyAggregates(t, mainDB, historical.Id, 100, 50)
	if stored.BillingDetailsVersion != logstore.LogBillingDetailsVersion {
		t.Fatalf("billing_details_version = %d, want %d", stored.BillingDetailsVersion, logstore.LogBillingDetailsVersion)
	}
	payload := requireTokenDetails(t, stored.BillingDetails)
	requireTokenValue(t, payload.Tokens.Input.TextInput, 100)
	requireTokenValue(t, payload.Tokens.Output.TextOutput, 50)
}

// TestMigrateDBMigratesLogModelOnFreshDB 验证主库迁移列表不遗漏 Log 模型：
// 空库上完整执行 migrateDB 后 logs 表具备 billing_details 列、重复启动幂等，
// 且迁移后的库可创建并读回该列（同库日志路径）。
func TestMigrateDBBillingDetailsOnFreshDB(t *testing.T) {
	mainDB, _ := setupBillingDetailsMigrateTestDB(t)
	dbstore.LOG_DB = mainDB

	for run := 1; run <= 2; run++ {
		if err := migrateDB(); err != nil {
			t.Fatalf("migrateDB run %d on fresh sqlite: %v", run, err)
		}
		if err := backfillLogBillingTokenDetails(); err != nil {
			t.Fatalf("backfill run %d on fresh sqlite: %v", run, err)
		}
	}
	if !mainDB.Migrator().HasColumn(&logstore.Log{}, "billing_details") {
		t.Fatal("logs.billing_details missing after migrateDB")
	}

	details := billingDetailsFixture
	if err := mainDB.Create(&logstore.Log{
		UserId:                1,
		CreatedAt:             1700000000,
		Type:                  logstore.LogTypeConsume,
		ModelName:             "gpt-test",
		Group:                 "default",
		BillingDetails:        &details,
		BillingDetailsVersion: logstore.LogBillingDetailsVersion,
	}).Error; err != nil {
		t.Fatalf("insert log with billing_details: %v", err)
	}
	var stored logstore.Log
	if err := mainDB.Where("user_id = ?", 1).First(&stored).Error; err != nil {
		t.Fatalf("reload log with billing_details: %v", err)
	}
	if stored.BillingDetails == nil || *stored.BillingDetails != billingDetailsFixture {
		t.Fatalf("billing_details = %v, want fixture JSON", stored.BillingDetails)
	}
}
