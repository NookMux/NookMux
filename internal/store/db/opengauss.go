package dbstore

import (
	"database/sql"

	_ "gitcode.com/opengauss/openGauss-connector-go-pq" // 注册驱动名 "opengauss"
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
