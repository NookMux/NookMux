package dbstore

import (
	"fmt"
	"github.com/NookMux/NookMux/internal/common"
	"github.com/NookMux/NookMux/internal/infra/db"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"gorm.io/driver/mysql"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	gormlogger "gorm.io/gorm/logger"
	"log"
	"os"
	"strings"
	"sync"
	"time"
)

// CommonGroupCol / CommonKeyCol 是 group/key 保留字列在当前主库方言下的引用
// 形式，供需要拼 raw SQL 片段的查询使用（由 InitCol 按数据库类型初始化）。
var CommonGroupCol string
var CommonKeyCol string

// LogGroupCol 是日志库（LOG_DB）方言下 group 列的引用形式。
var LogGroupCol string

func init() {
	InitCol()
}

func InitCol() {
	// init common column names
	// DM 标识符引用与 PostgreSQL 同为双引号（Oracle 风格），大小写敏感
	// 内核下反引号列名无法识别。
	if db.UsingPostgreSQL || db.UsingDM {
		CommonGroupCol = `"group"`
		CommonKeyCol = `"key"`
	} else {
		CommonGroupCol = "`group`"
		CommonKeyCol = "`key`"
	}
	if os.Getenv("LOG_SQL_DSN") != "" {
		switch db.LogSqlType {
		case db.DatabaseTypePostgreSQL, db.DatabaseTypeOpenGauss, db.DatabaseTypeDM, db.DatabaseTypeKingbase:
			LogGroupCol = `"group"`
		default:
			LogGroupCol = CommonGroupCol
		}
	} else {
		// LOG_SQL_DSN 为空时，日志数据库与主数据库相同
		if db.UsingPostgreSQL || db.UsingDM {
			LogGroupCol = `"group"`
		} else {
			LogGroupCol = CommonGroupCol
		}
	}
	// log sql type and database type
	//common.SysLog("Using Log SQL Type: " + db.LogSqlType)
}

var DB *gorm.DB

var LOG_DB *gorm.DB

// DefaultSQLMaxIdleConns 是连接池 idle 连接数的保守默认值（可被 SQL_MAX_IDLE_CONNS 覆盖），
// 供主库与日志库初始化共用；保持保守以控制低流量/待机场景的常驻内存。
const DefaultSQLMaxIdleConns = 20

// ChooseDB 按环境变量指定的 DSN 打开主库或日志库连接，并设置对应的数据库类型标记。
// 迁移编排（AutoMigrate 等）由 dbmigrate 包负责，本包只承载连接与句柄。
func ChooseDB(envName string, isLog bool) (*gorm.DB, error) {
	defer func() {
		InitCol()
	}()
	dsn := strings.TrimSpace(os.Getenv(envName))
	if dsn == "" {
		// Use SQLite
		common.SysLog("SQL_DSN not set, using SQLite as database")
		db.UsingSQLite = true
		return gorm.Open(sqlite.Open(db.SQLitePath), newGormConfig())
	}
	dbType, err := db.ParseDBTypeFromDSN(dsn)
	if err != nil {
		return nil, err
	}
	switch dbType {
	case db.DatabaseTypePostgreSQL:
		common.SysLog("using PostgreSQL as database")
		if !isLog {
			db.UsingPostgreSQL = true
		} else {
			db.LogSqlType = db.DatabaseTypePostgreSQL
		}
		return gorm.Open(postgres.New(postgres.Config{
			DSN:                  dsn,
			PreferSimpleProtocol: true, // disables implicit prepared statement usage
		}), newGormConfig())
	case db.DatabaseTypeOpenGauss:
		common.SysLog("using openGauss as database")
		if !isLog {
			// openGauss 为 PG 兼容内核，方言行为归入 PostgreSQL 家族；
			// 需要区分具体产品处使用 UsingOpenGauss 判断。
			db.UsingPostgreSQL = true
			db.UsingOpenGauss = true
		} else {
			db.LogSqlType = db.DatabaseTypeOpenGauss
		}
		return OpenOpenGaussGorm(dsn, newGormConfig())
	case db.DatabaseTypeMySQL:
		common.SysLog("using MySQL as database")
		driverDSN, err := db.MySQLDsnFromURL(dsn)
		if err != nil {
			return nil, err
		}
		if !isLog {
			db.UsingMySQL = true
		} else {
			db.LogSqlType = db.DatabaseTypeMySQL
		}
		return gorm.Open(mysql.Open(db.EnsureMySQLParseTime(driverDSN)), newGormConfig())
	case db.DatabaseTypeDM:
		common.SysLog("using DM (达梦) as database")
		if !isLog {
			db.UsingDM = true
		} else {
			db.LogSqlType = db.DatabaseTypeDM
		}
		return OpenDMGorm(dsn, newGormConfig())
	case db.DatabaseTypeKingbase:
		common.SysLog("using KingbaseES (金仓) as database")
		if !isLog {
			// 金仓 KingbaseES 为 PG 兼容内核，方言行为归入 PostgreSQL 家族；
			// 需要区分具体产品处使用 UsingKingbase 判断。
			db.UsingPostgreSQL = true
			db.UsingKingbase = true
		} else {
			db.LogSqlType = db.DatabaseTypeKingbase
		}
		return OpenKingbaseGorm(dsn, newGormConfig())
	default:
		return nil, fmt.Errorf("未知数据库类型: %s", dbType)
	}
}

// newGormConfig 构建统一的 GORM 配置。
//
// 关键点：默认 logger 的 IgnoreRecordNotFoundError 为 false，会把
// gorm.ErrRecordNotFound 当作 SQL 错误日志打印。在本项目中，按 key/id 查
// token、user、channel 等实体时"记录不存在"是合法的业务路径（无效 key、
// 过期、扫描、缓存回填探测等都会命中），不应该污染运行日志。
//
// 这里显式打开 IgnoreRecordNotFoundError，仅屏蔽 not-found 类日志，
// 慢查询和真正的 SQL 错误仍会正常输出。
func newGormConfig() *gorm.Config {
	logLevel := gormlogger.Warn
	if common.DebugEnabled {
		logLevel = gormlogger.Info
	}
	return &gorm.Config{
		// Prepared statement caching improves hot SQL paths but retains driver and
		// GORM statement state while idle. Keep it opt-in for small deployments.
		PrepareStmt: common.GetEnvOrDefaultBool("SQL_PREPARE_STMT", false),
		Logger: gormlogger.New(
			log.New(gin.DefaultWriter, "\n", log.LstdFlags),
			gormlogger.Config{
				SlowThreshold:             200 * time.Millisecond,
				LogLevel:                  logLevel,
				IgnoreRecordNotFoundError: true,
				Colorful:                  true,
			},
		),
	}
}

// CleanupLegacyUniqueConstraints 动态查询并删除指定表/列的所有 UNIQUE 约束和已知旧索引。
// 用于解决 GORM AutoMigrate 的 MigrateColumnUnique 在约束不存在时 DROP CONSTRAINT 不带 IF EXISTS 的问题。
func CleanupLegacyUniqueConstraints(db *gorm.DB, tableName string, columnName string, legacyIndexNames []string) {
	// 1. 动态查询所有与指定列相关的 UNIQUE 约束，逐一删除
	rows, err := db.Raw(
		`SELECT tc.constraint_name FROM information_schema.table_constraints tc
		 JOIN information_schema.constraint_column_usage ccu ON tc.constraint_schema = ccu.constraint_schema AND tc.constraint_name = ccu.constraint_name
		 WHERE tc.constraint_type = 'UNIQUE' AND tc.table_name = ? AND ccu.column_name = ?`,
		tableName, columnName,
	).Rows()
	if err == nil {
		for rows.Next() {
			var constraintName string
			if rows.Scan(&constraintName) == nil {
				_ = db.Exec(`ALTER TABLE ? DROP CONSTRAINT IF EXISTS ?`, clause.Table{Name: tableName}, clause.Column{Name: constraintName}).Error
			}
		}
		rows.Close()
	}

	// 2. 清理已知旧索引名
	for _, idxName := range legacyIndexNames {
		_ = db.Exec(`DROP INDEX IF EXISTS ?`, clause.Column{Name: idxName}).Error
	}
}

func closeDB(db *gorm.DB) error {
	sqlDB, err := db.DB()
	if err != nil {
		return err
	}
	err = sqlDB.Close()
	return err
}

func CloseDB() error {
	if LOG_DB != DB {
		err := closeDB(LOG_DB)
		if err != nil {
			return err
		}
	}
	return closeDB(DB)
}

// CheckMySQLChineseSupport ensures the MySQL connection and current schema
// default charset/collation can store Chinese characters. It allows common
// Chinese-capable charsets (utf8mb4, utf8, gbk, big5, gb18030) and panics otherwise.
func CheckMySQLChineseSupport(db *gorm.DB) error {
	// 仅检测：当前库默认字符集/排序规则 + 各表的排序规则（隐含字符集）

	// Read current schema defaults
	var schemaCharset, schemaCollation string
	err := db.Raw("SELECT DEFAULT_CHARACTER_SET_NAME, DEFAULT_COLLATION_NAME FROM information_schema.SCHEMATA WHERE SCHEMA_NAME = DATABASE()").Row().Scan(&schemaCharset, &schemaCollation)
	if err != nil {
		return fmt.Errorf("读取当前库默认字符集/排序规则失败 / Failed to read schema default charset/collation: %v", err)
	}

	toLower := func(s string) string { return strings.ToLower(s) }
	// Allowed charsets that can store Chinese text
	allowedCharsets := map[string]string{
		"utf8mb4": "utf8mb4_",
		"utf8":    "utf8_",
		"gbk":     "gbk_",
		"big5":    "big5_",
		"gb18030": "gb18030_",
	}
	isChineseCapable := func(cs, cl string) bool {
		csLower := toLower(cs)
		clLower := toLower(cl)
		if prefix, ok := allowedCharsets[csLower]; ok {
			if clLower == "" {
				return true
			}
			return strings.HasPrefix(clLower, prefix)
		}
		// 如果仅提供了排序规则，尝试按排序规则前缀判断
		for _, prefix := range allowedCharsets {
			if strings.HasPrefix(clLower, prefix) {
				return true
			}
		}
		return false
	}

	// 1) 当前库默认值必须支持中文
	if !isChineseCapable(schemaCharset, schemaCollation) {
		return fmt.Errorf("当前库默认字符集/排序规则不支持中文：schema(%s/%s)。请将库设置为 utf8mb4/utf8/gbk/big5/gb18030 / Schema default charset/collation is not Chinese-capable: schema(%s/%s). Please set to utf8mb4/utf8/gbk/big5/gb18030",
			schemaCharset, schemaCollation, schemaCharset, schemaCollation)
	}

	// 2) 所有物理表的排序规则（隐含字符集）必须支持中文
	type tableInfo struct {
		Name      string
		Collation *string
	}
	var tables []tableInfo
	if err := db.Raw("SELECT TABLE_NAME, TABLE_COLLATION FROM information_schema.TABLES WHERE TABLE_SCHEMA = DATABASE() AND TABLE_TYPE = 'BASE TABLE'").Scan(&tables).Error; err != nil {
		return fmt.Errorf("读取表排序规则失败 / Failed to read table collations: %v", err)
	}

	var badTables []string
	for _, t := range tables {
		// NULL 或空表示继承库默认设置，已在上面校验库默认，视为通过
		if t.Collation == nil || *t.Collation == "" {
			continue
		}
		cl := *t.Collation
		// 仅凭排序规则判断是否中文可用
		ok := false
		lower := strings.ToLower(cl)
		for _, prefix := range allowedCharsets {
			if strings.HasPrefix(lower, prefix) {
				ok = true
				break
			}
		}
		if !ok {
			badTables = append(badTables, fmt.Sprintf("%s(%s)", t.Name, cl))
		}
	}

	if len(badTables) > 0 {
		// 限制输出数量以避免日志过长
		maxShow := 20
		shown := badTables
		if len(shown) > maxShow {
			shown = shown[:maxShow]
		}
		return fmt.Errorf(
			"存在不支持中文的表，请修复其排序规则/字符集。示例（最多展示 %d 项）：%v / Found tables not Chinese-capable. Please fix their collation/charset. Examples (showing up to %d): %v",
			maxShow, shown, maxShow, shown,
		)
	}
	return nil
}

var (
	lastPingTime time.Time
	pingMutex    sync.Mutex
)

func PingDB() error {
	pingMutex.Lock()
	defer pingMutex.Unlock()

	if time.Since(lastPingTime) < time.Second*10 {
		return nil
	}

	sqlDB, err := DB.DB()
	if err != nil {
		log.Printf("Error getting sql.DB from GORM: %v", err)
		return err
	}

	err = sqlDB.Ping()
	if err != nil {
		log.Printf("Error pinging DB: %v", err)
		return err
	}

	lastPingTime = time.Now()
	common.SysLog("Database pinged successfully")
	return nil
}
