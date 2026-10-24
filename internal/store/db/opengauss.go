package dbstore

import (
	"database/sql"

	_ "gitcode.com/opengauss/openGauss-connector-go-pq" // 注册驱动名 "opengauss"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

// openOpenGaussSQL 使用 openGauss 官方 Go 驱动建立 database/sql 连接。
//
// openGauss 默认的 sha256 口令认证是其私有协议，pgx/libpq 无法完成认证，
// 因此连接 openGauss 必须使用该官方 connector；随后以
// postgres.New(postgres.Config{Conn: ...}) 把连接交给 GORM 的 postgres
// 方言（openGauss 为 PG 兼容内核）。
//
// DSN 支持官方 connector 的两种形式：URL（opengauss:// 或 postgres:// 前缀，
// userinfo 按标准 URL percent-encode）与 key=value（口令含空格或单引号时
// 用单引号包裹）。
func OpenOpenGaussSQL(dsn string) (*sql.DB, error) {
	return sql.Open("opengauss", dsn)
}

// OpenOpenGaussGorm 统一封装 openGauss 的 GORM 连接建立：以官方 connector
// 打开 database/sql 连接，交给 GORM 的 postgres 方言（openGauss 为 PG 兼容
// 内核）。
//
// postgres.Config 必须显式设置 DriverName：其 migrator 在 PrepareStmt 关闭
// 且 DriverName 为空或 "pgx" 时，会向迁移查询的参数头部注入 pgx 的
// QueryExecModeSimpleProtocol 协议开关参数——pgx 驱动能识别并吞掉该参数，
// 而 openGauss 官方 connector 会把它当作真实 SQL 参数发给服务端，导致对
// 已有表执行 AutoMigrate 列比较时报 "got N parameters but the statement
// requires M"。设置为非 "pgx" 的非空驱动名即可跳过该注入；由于连接已通过
// Conn 注入，DriverName 不参与连接建立，仅用于方言的驱动识别。
func OpenOpenGaussGorm(dsn string, cfg *gorm.Config) (*gorm.DB, error) {
	sqlDB, err := OpenOpenGaussSQL(dsn)
	if err != nil {
		return nil, err
	}
	return gorm.Open(postgres.New(postgres.Config{
		Conn:       sqlDB,
		DriverName: "opengauss",
	}), cfg)
}
