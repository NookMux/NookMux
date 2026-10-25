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

func openDBByType(dsn string, dbType string) (*gorm.DB, error) {
	dsn = strings.TrimSpace(dsn)
	if dsn == "" {
		return nil, errors.New("DSN 不能为空")
	}
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
