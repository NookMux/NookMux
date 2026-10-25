package dbmigrate

import (
	"errors"
	"github.com/NookMux/NookMux/internal/common"
	"github.com/NookMux/NookMux/internal/infra/db"
	"github.com/NookMux/NookMux/internal/store/db"
	"gorm.io/driver/mysql"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"strings"
)

// migrateTargetMaxOpenConns 迁移目标库连接池上限：拷贝按表串行、按批
// 事务写入，少量连接即可满足；收紧上限避免无界目标池叠加实例会话上限
// （金仓默认 max_connections=100、DM 受 MAX_SESSIONS 约束）在迁移期间挤占
// 线上会话。
const migrateTargetMaxOpenConns = 8

func openDBByType(dsn string, dbType string) (*gorm.DB, error) {
	dsn = strings.TrimSpace(dsn)
	if dsn == "" {
		return nil, errors.New("DSN 不能为空")
	}
	g, err := openDBByTypeUncapped(dsn, dbType)
	if err != nil {
		return nil, err
	}
	sqlDB, err := g.DB()
	if err != nil {
		_ = closeGormDB(g)
		return nil, err
	}
	sqlDB.SetMaxOpenConns(migrateTargetMaxOpenConns)
	return g, nil
}

func openDBByTypeUncapped(dsn string, dbType string) (*gorm.DB, error) {
	switch dbType {
	case db.DatabaseTypePostgreSQL:
		return gorm.Open(postgres.New(postgres.Config{
			DSN:                  dsn,
			PreferSimpleProtocol: true,
		}), &gorm.Config{
			PrepareStmt: common.GetEnvOrDefaultBool("SQL_PREPARE_STMT", false),
		})
	case db.DatabaseTypeOpenGauss:
		return dbstore.OpenOpenGaussGorm(dsn, &gorm.Config{
			PrepareStmt: common.GetEnvOrDefaultBool("SQL_PREPARE_STMT", false),
		})
	case db.DatabaseTypeMySQL:
		driverDSN, err := db.MySQLDsnFromURL(dsn)
		if err != nil {
			return nil, err
		}
		return gorm.Open(mysql.Open(db.EnsureMySQLParseTime(driverDSN)), &gorm.Config{
			PrepareStmt: common.GetEnvOrDefaultBool("SQL_PREPARE_STMT", false),
		})
	case db.DatabaseTypeDM:
		return dbstore.OpenDMGorm(dsn, &gorm.Config{
			PrepareStmt: common.GetEnvOrDefaultBool("SQL_PREPARE_STMT", false),
		})
	case db.DatabaseTypeKingbase:
		return dbstore.OpenKingbaseGorm(dsn, &gorm.Config{
			PrepareStmt: common.GetEnvOrDefaultBool("SQL_PREPARE_STMT", false),
		})
	default:
		return nil, errors.New("不支持的数据库类型")
	}
}

func closeGormDB(db *gorm.DB) error {
	if db == nil {
		return nil
	}
	sqlDB, err := db.DB()
	if err != nil {
		return err
	}
	return sqlDB.Close()
}
