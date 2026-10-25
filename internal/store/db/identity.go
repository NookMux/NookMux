package dbstore

import (
	"database/sql"
	"fmt"
	infradb "github.com/NookMux/NookMux/internal/infra/db"
	"gorm.io/gorm"
)

// dbIdentity 用于唯一标识一个数据库连接
type dbIdentity struct {
	Host       string
	Port       string
	Database   string
	ServerUUID string // MySQL: @@server_uuid; PostgreSQL: system_identifier; openGauss: pg_postmaster_start_time 实例指纹; DM: V$INSTANCE 实例名
}

func (id dbIdentity) String() string {
	if id.Host == "" && id.Port == "" {
		// openGauss 身份由实例指纹标识，不含监听地址
		return fmt.Sprintf("%s (instance=%s)", id.Database, id.ServerUUID)
	}
	return fmt.Sprintf("%s:%s/%s (uuid=%s)", id.Host, id.Port, id.Database, id.ServerUUID)
}

// getDBIdentity 通过数据库连接获取唯一标识
func GetDBIdentity(db *gorm.DB, dbType string) (dbIdentity, error) {
	switch dbType {
	case infradb.DatabaseTypePostgreSQL:
		return getPostgresIdentity(db)
	case infradb.DatabaseTypeOpenGauss:
		return getOpenGaussIdentity(db)
	case infradb.DatabaseTypeMySQL:
		return getMySQLIdentity(db)
	case infradb.DatabaseTypeDM:
		return getDMIdentity(db)
	default:
		return dbIdentity{}, fmt.Errorf("不支持的数据库类型：%s", dbType)
	}
}

func getOpenGaussIdentity(db *gorm.DB) (dbIdentity, error) {
	// openGauss 内核无 pg_control_system()/system_identifier，以
	// pg_postmaster_start_time() 的 epoch 作为实例指纹：epoch 是绝对时间，
	// 同一实例的任意连接恒返回同一值（不受连接地址族与会话时区影响），
	// 不同实例的启动时刻几乎必异。身份仅由实例指纹 + 数据库名构成：
	// inet_server_addr()/inet_server_port() 描述的是连接路径而非实例
	// （同一实例双栈监听时 v4/v6 连接取值不同，unix socket 下为 NULL），
	// 不参与同库判定。
	var dbName, fingerprint string
	row := db.Raw("SELECT current_database(), extract(epoch from pg_postmaster_start_time())").Row()
	if err := row.Scan(&dbName, &fingerprint); err != nil {
		return dbIdentity{}, fmt.Errorf("获取 openGauss 连接标识失败：%w", err)
	}
	return dbIdentity{Database: dbName, ServerUUID: fingerprint}, nil
}

func getPostgresIdentity(db *gorm.DB) (dbIdentity, error) {
	var host, port, dbName string
	var sysID string
	// pg 后端进程监听地址 + system_identifier（唯一标识一个 PG 实例）
	row := db.Raw("SELECT inet_server_addr(), inet_server_port(), current_database(), system_identifier FROM pg_control_system()").Row()
	if err := row.Scan(&host, &port, &dbName, &sysID); err != nil {
		return dbIdentity{}, fmt.Errorf("获取 PostgreSQL 连接标识失败：%w", err)
	}
	return dbIdentity{Host: host, Port: port, Database: dbName, ServerUUID: sysID}, nil
}

// getDMIdentity 以 V$INSTANCE 实例名 + 监听端口构成 DM 实例指纹。
// DM 的逻辑"数据库"概念是 schema（当前模式），以 SYS_CONTEXT('USERENV',
// 'CURRENT_SCHEMA') 取值；HOST_NAME 描述服务端主机而非监听地址，与
// MySQL 一样仅用于展示，不参与同库判定。
func getDMIdentity(db *gorm.DB) (dbIdentity, error) {
	var host, schema, instanceName string
	var port sql.NullString
	row := db.Raw(`SELECT HOST_NAME,
		(SELECT PARA_VALUE FROM V$DM_INI WHERE PARA_NAME = 'PORT_NUM' LIMIT 1),
		SYS_CONTEXT('USERENV', 'CURRENT_SCHEMA'), NAME FROM V$INSTANCE`).Row()
	if err := row.Scan(&host, &port, &schema, &instanceName); err != nil {
		return dbIdentity{}, fmt.Errorf("获取达梦连接标识失败：%w", err)
	}
	p := ""
	if port.Valid {
		p = port.String
	}
	return dbIdentity{Host: host, Port: p, Database: schema, ServerUUID: instanceName}, nil
}

func getMySQLIdentity(db *gorm.DB) (dbIdentity, error) {
	var host, port sql.NullString
	var dbName, serverUUID string
	// @@server_uuid 全局唯一标识一个 MySQL 实例（5.6+）
	row := db.Raw("SELECT @@hostname, @@port, DATABASE(), @@server_uuid").Row()
	if err := row.Scan(&host, &port, &dbName, &serverUUID); err != nil {
		return dbIdentity{}, fmt.Errorf("获取 MySQL 连接标识失败：%w", err)
	}
	h := ""
	if host.Valid {
		h = host.String
	}
	p := ""
	if port.Valid {
		p = port.String
	}
	return dbIdentity{Host: h, Port: p, Database: dbName, ServerUUID: serverUUID}, nil
}
