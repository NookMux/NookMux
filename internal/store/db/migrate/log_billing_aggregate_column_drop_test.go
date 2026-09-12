package dbmigrate

import (
	"fmt"
	"testing"

	"github.com/NookMux/NookMux/internal/store/db"
	"github.com/NookMux/NookMux/internal/store/log"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

// TestDropLegacyLogTokenAggregateColumnsAfterBackfill 验证验收标准"历史转换
// 和校验完成后……再删除 prompt_tokens、completion_tokens 两个旧列"：
// 回填完成后删列、billing_details 保留、重复执行幂等。
func TestDropLegacyLogTokenAggregateColumnsAfterBackfill(t *testing.T) {
	dbHandle := setupTokenDetailsMigrationDB(t)
	oldLogDB := dbstore.LOG_DB
	dbstore.LOG_DB = dbHandle
	t.Cleanup(func() { dbstore.LOG_DB = oldLogDB })

	row := seedTokenMigrationLog(t, dbHandle, logstore.Log{
		Type:             logstore.LogTypeConsume,
		PromptTokens:     100,
		CompletionTokens: 50,
		Other:            `{"cache_tokens":20}`,
	})

	// 前置条件：回填前旧列存在。
	for _, col := range legacyAggregateColumns {
		if !dbHandle.Migrator().HasColumn(&logstore.Log{}, col) {
			t.Fatalf("precondition failed: column %s should exist before drop", col)
		}
	}

	if err := backfillLogBillingTokenDetails(); err != nil {
		t.Fatalf("backfill: %v", err)
	}
	// 未执行 drop 前旧列保留（删除不得先于历史数据恢复）。
	for _, col := range legacyAggregateColumns {
		if !dbHandle.Migrator().HasColumn(&logstore.Log{}, col) {
			t.Fatalf("column %s dropped before drop step ran", col)
		}
	}

	if err := dropLegacyLogTokenAggregateColumns(); err != nil {
		t.Fatalf("drop legacy columns: %v", err)
	}
	for _, col := range legacyAggregateColumns {
		if dbHandle.Migrator().HasColumn(&logstore.Log{}, col) {
			t.Fatalf("column %s still present after drop step", col)
		}
	}

	// 回填产物不受删列影响。
	stored := tokenMigrationStoredRow(t, dbHandle, row.Id)
	if stored.BillingDetailsVersion != logstore.LogBillingDetailsVersion {
		t.Fatalf("billing_details_version = %d, want %d", stored.BillingDetailsVersion, logstore.LogBillingDetailsVersion)
	}
	payload := requireTokenDetails(t, stored.BillingDetails)
	requireTokenValue(t, payload.Tokens.Input.TextInput, 80)
	requireTokenValue(t, payload.Tokens.Cache.ReadCache, 20)

	// 幂等：列已删除后重复执行不得报错。
	if err := dropLegacyLogTokenAggregateColumns(); err != nil {
		t.Fatalf("repeat drop: %v", err)
	}
}

// TestBackfillSkipsScanOnFreshSchemaWithoutLegacyColumns 验证全新空库（模型
// 已无旧列）启动：回填守卫跳过按列名扫描，直接写入完成标记，不得因旧列
// 缺失而阻断启动。
func TestBackfillSkipsScanOnFreshSchemaWithoutLegacyColumns(t *testing.T) {
	dbHandle, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := dbHandle.AutoMigrate(&logstore.Log{}); err != nil {
		t.Fatalf("migrate sqlite: %v", err)
	}
	oldLogDB := dbstore.LOG_DB
	dbstore.LOG_DB = dbHandle
	t.Cleanup(func() { dbstore.LOG_DB = oldLogDB })

	for _, col := range legacyAggregateColumns {
		if dbHandle.Migrator().HasColumn(&logstore.Log{}, col) {
			t.Fatalf("precondition failed: fresh schema must not have column %s", col)
		}
	}

	if err := backfillLogBillingTokenDetails(); err != nil {
		t.Fatalf("backfill on fresh schema: %v", err)
	}
	var count int64
	if err := dbHandle.Model(&logBillingMigrationState{}).Where("version = ?", logstore.LogBillingDetailsVersion).Count(&count).Error; err != nil {
		t.Fatalf("count markers: %v", err)
	}
	if count != 1 {
		t.Fatalf("marker count = %d, want 1", count)
	}

	// 已标记完成后重启不得重新扫描 logs 表。
	if err := dbHandle.Callback().Query().Before("gorm:query").Register("test:no-rescan", func(tx *gorm.DB) {
		if tx.Statement.Table == "logs" {
			tx.AddError(fmt.Errorf("unexpected logs rescan on fresh schema"))
		}
	}); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { dbHandle.Callback().Query().Remove("test:no-rescan") })
	if err := backfillLogBillingTokenDetails(); err != nil {
		t.Fatal(err)
	}
}

// TestDropLegacyColumnsToleratesMissingTable 验证 logs 表不存在（异常环境）
// 时删列步骤不误报，列不存在时跳过。
func TestDropLegacyColumnsToleratesMissingTable(t *testing.T) {
	dbHandle, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	oldLogDB := dbstore.LOG_DB
	dbstore.LOG_DB = dbHandle
	t.Cleanup(func() { dbstore.LOG_DB = oldLogDB })

	if err := dropLegacyLogTokenAggregateColumns(); err != nil {
		t.Fatalf("drop without logs table: %v", err)
	}
}
