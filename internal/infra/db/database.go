package db

const (
	DatabaseTypeMySQL      = "mysql"
	DatabaseTypeSQLite     = "sqlite"
	DatabaseTypePostgreSQL = "postgres"
	DatabaseTypeOpenGauss  = "opengauss"
	DatabaseTypeDM         = "dm"
	DatabaseTypeKingbase   = "kingbase"
)

var UsingSQLite = false
var UsingPostgreSQL = false
var UsingOpenGauss = false
var UsingDM = false
var UsingKingbase = false
var LogSqlType = DatabaseTypeSQLite // Default to SQLite for logging SQL queries
var UsingMySQL = false
var UsingClickHouse = false

var SQLitePath = "one-api.db?_busy_timeout=30000"

// IntegerDivisionTruncates 报告当前主库的整数除法是否向零截断。
// PostgreSQL/SQLite 截断；MySQL、openGauss、DM、金仓 KingbaseES 的整数除法
// 返回非截断小数，依赖整除截断语义的 SQL（如用量排行时间分桶）必须改用
// FLOOR 显式截断。接入新库时在此集中登记，消费点只读本谓词。
func IntegerDivisionTruncates() bool {
	return !UsingMySQL && !UsingOpenGauss && !UsingDM && !UsingKingbase
}
