package db

const (
	DatabaseTypeMySQL      = "mysql"
	DatabaseTypeSQLite     = "sqlite"
	DatabaseTypePostgreSQL = "postgres"
	DatabaseTypeOpenGauss  = "opengauss"
	DatabaseTypeDM         = "dm"
)

var UsingSQLite = false
var UsingPostgreSQL = false
var UsingOpenGauss = false
var UsingDM = false
var LogSqlType = DatabaseTypeSQLite // Default to SQLite for logging SQL queries
var UsingMySQL = false
var UsingClickHouse = false

var SQLitePath = "one-api.db?_busy_timeout=30000"
