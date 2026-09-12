package dbmigrate

import (
	"database/sql"
	"fmt"
	"os"
	"strings"
	"testing"

	infradb "github.com/NookMux/NookMux/internal/infra/db"
	"github.com/NookMux/NookMux/internal/store/db"
	"github.com/NookMux/NookMux/internal/store/log"

	"gorm.io/driver/mysql"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

// 本文件覆盖 docs/PRD/计费.md 阶段 0 的 MySQL / PostgreSQL 迁移路径：
// 空库初始化、历史库启动（DropColumn 模拟）、重复启动迁移幂等、
// 主库（migrateDB）与独立日志库（migrateLOGDB）两条路径下的
// billing_details 建列与读写。
//
// 迁移逻辑与方言无关，SQLite 路径由 log_billing_details_migration_test.go
// 覆盖并默认随 CI 运行；真实 MySQL / PostgreSQL 需要可用的数据库实例，
// 因此通过环境变量门控：未设置 TEST_BILLING_DETAILS_DB_DSN 时跳过，
// 设置后按 DSN 前缀自动选择 PostgreSQL 或 MySQL 驱动，例如：
//
//	TEST_BILLING_DETAILS_DB_DSN='postgres://postgres@127.0.0.1:15432/postgres?sslmode=disable' \
//	    go test ./internal/store/db/migrate/ -run TestBillingDetailsMigrateOnDialect
//	TEST_BILLING_DETAILS_DB_DSN='root@tcp(127.0.0.1:13306)/' \
//	    go test ./internal/store/db/migrate/ -run TestBillingDetailsMigrateOnDialect
//
// 该 DSN 指向一个允许建库的管理库连接；测试会在其上重建两个专用数据库
// （billing_details_test_main / billing_details_test_log），不触碰其他数据。
const billingDetailsDSNEnv = "TEST_BILLING_DETAILS_DB_DSN"

const (
	billingDetailsMainDBName = "billing_details_test_main"
	billingDetailsLogDBName  = "billing_details_test_log"
)

// TestBillingDetailsMigrateOnDialect 在真实 MySQL / PostgreSQL 上验证
// billing_details 的三库迁移验收标准（空库、历史库、重复启动、双路径）。
func TestBillingDetailsMigrateOnDialect(t *testing.T) {
	adminDSN := strings.TrimSpace(os.Getenv(billingDetailsDSNEnv))
	if adminDSN == "" {
		t.Skipf("%s not set; MySQL/PostgreSQL migration path not exercised (SQLite path runs unconditionally)", billingDetailsDSNEnv)
	}

	isPostgres := strings.HasPrefix(adminDSN, "postgres://") || strings.HasPrefix(adminDSN, "postgresql://")

	// 保存并恢复全局连接句柄、方言标记与相关环境变量，避免污染同包其他测试。
	oldDB := dbstore.DB
	oldLogDB := dbstore.LOG_DB
	oldUsingPostgreSQL := infradb.UsingPostgreSQL
	oldUsingMySQL := infradb.UsingMySQL
	oldUsingSQLite := infradb.UsingSQLite
	oldLogSqlType := infradb.LogSqlType
	oldSQLDSN := os.Getenv("SQL_DSN")
	oldLogSQLDSN := os.Getenv("LOG_SQL_DSN")
	t.Cleanup(func() {
		dbstore.DB = oldDB
		dbstore.LOG_DB = oldLogDB
		infradb.UsingPostgreSQL = oldUsingPostgreSQL
		infradb.UsingMySQL = oldUsingMySQL
		infradb.UsingSQLite = oldUsingSQLite
		infradb.LogSqlType = oldLogSqlType
		if err := os.Setenv("SQL_DSN", oldSQLDSN); err != nil {
			t.Errorf("restore SQL_DSN: %v", err)
		}
		if err := os.Setenv("LOG_SQL_DSN", oldLogSQLDSN); err != nil {
			t.Errorf("restore LOG_SQL_DSN: %v", err)
		}
		// InitCol 依据方言标记与环境变量重算保留字列引用，必须在全局量恢复后执行。
		dbstore.InitCol()
	})

	adminDB, err := openBillingDetailsAdminDB(adminDSN, isPostgres)
	if err != nil {
		t.Fatalf("open admin db: %v", err)
	}
	mainDSN, logDSN := recreateBillingDetailsTestDatabases(t, adminDB, adminDSN, isPostgres)
	if err := closeGormDB(adminDB); err != nil {
		t.Fatalf("close admin db: %v", err)
	}

	mainDB := openBillingDetailsDB(t, "SQL_DSN", mainDSN, false)
	logDB := openBillingDetailsDB(t, "LOG_SQL_DSN", logDSN, true)

	// —— 主库路径（同库日志）：空库初始化 ——
	dbstore.LOG_DB = mainDB
	if err := migrateDB(); err != nil {
		t.Fatalf("migrateDB on empty %s: %v", dialectName(isPostgres), err)
	}
	if err := backfillLogBillingTokenDetails(); err != nil {
		t.Fatalf("billing details backfill on empty %s: %v", dialectName(isPostgres), err)
	}
	if !mainDB.Migrator().HasColumn(&logstore.Log{}, "billing_details") {
		t.Fatalf("logs.billing_details missing after migrateDB on empty %s", dialectName(isPostgres))
	}
	// 重复启动迁移：第二次 migrateDB 必须幂等。
	if err := migrateDB(); err != nil {
		t.Fatalf("migrateDB re-run (idempotency) on %s: %v", dialectName(isPostgres), err)
	}

	// 主库写入并读回 billing_details（同库日志路径可读写）。
	insertAndReadBackBillingDetails(t, mainDB, 101, "migrateDB write/read")

	// —— 主库路径：历史库启动 ——
	// 先插入一条"历史"行，再删除列模拟旧 schema，迁移补列后行内容必须原样。
	seedHistoricalLog(t, mainDB, 102)
	dropBillingDetailsColumn(t, mainDB)
	// 导入旧格式数据属于受控升级流程，必须先显式作废既有完成标记，
	// 否则 backfill 会因标记而跳过扫描。
	invalidateBillingMigrationMarker(t, mainDB)
	if err := migrateDB(); err != nil {
		t.Fatalf("migrateDB on historical %s schema: %v", dialectName(isPostgres), err)
	}
	if err := backfillLogBillingTokenDetails(); err != nil {
		t.Fatalf("billing details backfill on historical %s: %v", dialectName(isPostgres), err)
	}
	assertHistoricalLogMigrated(t, mainDB, 102)
	// 再次重复启动迁移验证幂等。
	if err := migrateDB(); err != nil {
		t.Fatalf("migrateDB re-run after historical migration on %s: %v", dialectName(isPostgres), err)
	}
	if err := backfillLogBillingTokenDetails(); err != nil {
		t.Fatalf("billing details backfill re-run on %s: %v", dialectName(isPostgres), err)
	}

	// —— 独立日志库路径（LOG_SQL_DSN）：空库初始化 + 历史库启动 + 幂等 ——
	dbstore.LOG_DB = logDB
	if err := migrateLOGDB(); err != nil {
		t.Fatalf("migrateLOGDB on empty %s log db: %v", dialectName(isPostgres), err)
	}
	if err := backfillLogBillingTokenDetails(); err != nil {
		t.Fatalf("billing details backfill on empty %s log db: %v", dialectName(isPostgres), err)
	}
	seedHistoricalLog(t, logDB, 201)
	dropBillingDetailsColumn(t, logDB)
	// 独立日志库的标记在日志库本库：同样先作废再触发历史迁移。
	invalidateBillingMigrationMarker(t, logDB)
	// 历史库启动 + 重复启动迁移，各跑一次共两次。
	for run := 1; run <= 2; run++ {
		if err := migrateLOGDB(); err != nil {
			t.Fatalf("migrateLOGDB run %d on historical %s log db: %v", run, dialectName(isPostgres), err)
		}
		if err := backfillLogBillingTokenDetails(); err != nil {
			t.Fatalf("billing details backfill run %d on historical %s log db: %v", run, dialectName(isPostgres), err)
		}
	}
	if !logDB.Migrator().HasColumn(&logstore.Log{}, "billing_details") {
		t.Fatalf("logs.billing_details missing after migrateLOGDB on %s", dialectName(isPostgres))
	}
	assertHistoricalLogMigrated(t, logDB, 201)
	insertAndReadBackBillingDetails(t, logDB, 202, "migrateLOGDB write/read")
}

func dialectName(isPostgres bool) string {
	if isPostgres {
		return "postgresql"
	}
	return "mysql"
}

// invalidateBillingMigrationMarker 删除实际日志库中的完成标记，模拟受控
// 升级流程中"导入旧格式数据前显式作废标记"的前置条件。
func invalidateBillingMigrationMarker(t *testing.T, dbHandle *gorm.DB) {
	t.Helper()
	if err := dbHandle.Where("version = ?", logstore.LogBillingDetailsVersion).Delete(&logBillingMigrationState{}).Error; err != nil {
		t.Fatalf("invalidate billing migration marker: %v", err)
	}
}

// openBillingDetailsAdminDB 打开管理连接（用于重建测试专用数据库）。
// PostgreSQL 连接 DSN 中的默认库（通常是 postgres）；MySQL 不带库名连接。
func openBillingDetailsAdminDB(adminDSN string, isPostgres bool) (*gorm.DB, error) {
	if isPostgres {
		return gorm.Open(postgres.Open(adminDSN), &gorm.Config{})
	}
	return gorm.Open(mysql.Open(adminDSN), &gorm.Config{})
}

// recreateBillingDetailsTestDatabases 删除并重建测试专用主库/日志库，
// 返回两条路径的连接 DSN。MySQL 显式使用 utf8mb4，保证与通过
// CheckMySQLChineseSupport 校验的真实部署等价。
func recreateBillingDetailsTestDatabases(t *testing.T, adminDB *gorm.DB, adminDSN string, isPostgres bool) (mainDSN string, logDSN string) {
	t.Helper()

	for _, name := range []string{billingDetailsMainDBName, billingDetailsLogDBName} {
		if err := adminDB.Exec(fmt.Sprintf("DROP DATABASE IF EXISTS %s", name)).Error; err != nil {
			t.Fatalf("drop test database %s: %v", name, err)
		}
		createSQL := "CREATE DATABASE " + name
		if !isPostgres {
			createSQL += " CHARACTER SET utf8mb4 COLLATE utf8mb4_general_ci"
		}
		if err := adminDB.Exec(createSQL).Error; err != nil {
			t.Fatalf("create test database %s: %v", name, err)
		}
	}

	if isPostgres {
		return replacePostgresDBName(adminDSN, billingDetailsMainDBName),
			replacePostgresDBName(adminDSN, billingDetailsLogDBName)
	}
	// MySQL 管理 DSN 采用直接拼接库名的方式，必须以 "/" 结尾（根路径形式），
	// 否则拼出的连接 DSN 指向错误库名，尽早失败给出清晰提示。
	if !strings.HasSuffix(adminDSN, "/") {
		t.Fatalf("MySQL admin DSN must end with '/' (e.g. 'root@tcp(host:3306)/'), got %q", adminDSN)
	}
	return adminDSN + billingDetailsMainDBName, adminDSN + billingDetailsLogDBName
}

// replacePostgresDBName 把 PostgreSQL 管理 DSN 的路径段（库名）替换为指定库。
func replacePostgresDBName(dsn string, dbName string) string {
	idx := strings.LastIndex(dsn, "/")
	if idx < 0 {
		return dsn
	}
	prefix := dsn[:idx+1]
	suffix := dsn[idx+1:]
	if q := strings.Index(suffix, "?"); q >= 0 {
		return prefix + dbName + "?" + suffix[q+1:]
	}
	return prefix + dbName
}

// openBillingDetailsDB 走真实的 dbstore.ChooseDB 路径打开连接并设置方言标记，
// 与 InitDB/InitLogDB 的连接建立方式一致。
func openBillingDetailsDB(t *testing.T, envName string, dsn string, isLog bool) *gorm.DB {
	t.Helper()
	if err := os.Setenv(envName, dsn); err != nil {
		t.Fatalf("set %s: %v", envName, err)
	}
	dbHandle, err := dbstore.ChooseDB(envName, isLog)
	if err != nil {
		t.Fatalf("choose db via %s: %v", envName, err)
	}
	if isLog {
		dbstore.LOG_DB = dbHandle
	} else {
		dbstore.DB = dbHandle
	}
	return dbHandle
}

func seedHistoricalLog(t *testing.T, dbHandle *gorm.DB, userId int) {
	t.Helper()
	// 历史行需要旧聚合列承载 prompt/completion 总量；当前模型已不声明这两列
	//（读取为 wire 投影字段），先补列再以 map 插入真实历史数据。
	addLegacyAggregateColumns(t, dbHandle)
	historical := map[string]interface{}{
		"user_id":           userId,
		"created_at":        1700000000,
		"type":              logstore.LogTypeConsume,
		"username":          "legacy-user",
		"model_name":        "gpt-legacy",
		"quota":             42,
		"prompt_tokens":     100,
		"completion_tokens": 50,
		"group":             "default",
		"other":             `{"cache_tokens":30,"cache_ratio":0.5}`,
	}
	if err := dbHandle.Table("logs").Create(historical).Error; err != nil {
		t.Fatalf("seed historical log (userId=%d): %v", userId, err)
	}
}

// assertHistoricalLogMigrated 验证历史行完成 Token 明细迁移，同时保留
// Other 中的非 Token 字段与旧聚合列数据。
func assertHistoricalLogMigrated(t *testing.T, dbHandle *gorm.DB, userId int) {
	t.Helper()
	var value sql.NullString
	if err := dbHandle.Raw("SELECT billing_details FROM logs WHERE user_id = ?", userId).Row().Scan(&value); err != nil {
		t.Fatalf("query billing_details for historical log (userId=%d): %v", userId, err)
	}
	wantDetails := `{"schema_version":1,"tokens":{"input":{"text_input":70},"output":{"text_output":50},"cache":{"read_cache":30}}}`
	if !value.Valid || value.String != wantDetails {
		t.Fatalf("historical billing_details = %v, want %q", value, wantDetails)
	}
	var legacy struct {
		Quota            int
		PromptTokens     int
		CompletionTokens int
		Other            string
		Version          int
	}
	if err := dbHandle.Raw("SELECT quota, prompt_tokens, completion_tokens, other, billing_details_version FROM logs WHERE user_id = ?", userId).
		Row().Scan(&legacy.Quota, &legacy.PromptTokens, &legacy.CompletionTokens, &legacy.Other, &legacy.Version); err != nil {
		t.Fatalf("reload historical log (userId=%d): %v", userId, err)
	}
	if legacy.Quota != 42 || legacy.PromptTokens != 100 || legacy.CompletionTokens != 50 || legacy.Other != `{"cache_ratio":0.5}` {
		t.Fatalf("historical row mutated by migration: %+v", legacy)
	}
	if legacy.Version != logstore.LogBillingDetailsVersion {
		t.Fatalf("billing_details_version = %d, want %d", legacy.Version, logstore.LogBillingDetailsVersion)
	}
}

// insertAndReadBackBillingDetails 写入带 canonical JSON 的日志并精确读回，
// 验证迁移后的库可正常读写新列。
func insertAndReadBackBillingDetails(t *testing.T, dbHandle *gorm.DB, userId int, scenario string) {
	t.Helper()
	details := billingDetailsFixture
	if err := dbHandle.Create(&logstore.Log{
		UserId:                userId,
		CreatedAt:             1700000001,
		Type:                  logstore.LogTypeConsume,
		ModelName:             "gpt-test",
		Group:                 "default",
		BillingDetails:        &details,
		BillingDetailsVersion: logstore.LogBillingDetailsVersion,
	}).Error; err != nil {
		t.Fatalf("%s: insert log with billing_details: %v", scenario, err)
	}
	var stored logstore.Log
	if err := dbHandle.Where("user_id = ?", userId).First(&stored).Error; err != nil {
		t.Fatalf("%s: reload log with billing_details: %v", scenario, err)
	}
	if stored.BillingDetails == nil || *stored.BillingDetails != billingDetailsFixture {
		t.Fatalf("%s: billing_details = %v, want fixture JSON", scenario, stored.BillingDetails)
	}
}
