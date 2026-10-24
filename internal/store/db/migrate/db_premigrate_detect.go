package dbmigrate

import (
	"errors"
	"github.com/NookMux/NookMux/internal/common"
	"github.com/NookMux/NookMux/internal/infra/db"
	"strings"
)

func currentMainDBType() (string, error) {
	switch {
	case db.UsingOpenGauss:
		return db.DatabaseTypeOpenGauss, nil
	case db.UsingPostgreSQL:
		return db.DatabaseTypePostgreSQL, nil
	case db.UsingMySQL:
		return db.DatabaseTypeMySQL, nil
	case db.UsingSQLite:
		return db.DatabaseTypeSQLite, nil
	default:
		return "", errors.New("无法识别当前主数据库类型（UsingMySQL/UsingPostgreSQL/UsingOpenGauss/UsingSQLite 均为 false）")
	}
}

func currentLogDBType() (string, error) {
	// LOG_SQL_DSN 为空时，日志库与主库相同；否则用 db.LogSqlType
	mainType, err := currentMainDBType()
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(common.GetEnvOrDefaultString("LOG_SQL_DSN", "")) == "" {
		return mainType, nil
	}
	switch db.LogSqlType {
	case db.DatabaseTypePostgreSQL, db.DatabaseTypeOpenGauss, db.DatabaseTypeMySQL, db.DatabaseTypeSQLite:
		return db.LogSqlType, nil
	default:
		return "", errors.New("无法识别当前日志数据库类型")
	}
}

func validateDBPreMigrateDirection(sourceType string, targetType string) error {
	if sourceType == "" || targetType == "" {
		return errors.New("数据库类型不能为空")
	}
	if sourceType == targetType {
		return errors.New("源数据库与目标数据库类型相同，无需迁移")
	}
	switch sourceType {
	case db.DatabaseTypeMySQL:
		if targetType != db.DatabaseTypePostgreSQL && targetType != db.DatabaseTypeOpenGauss {
			return errors.New("仅支持 MySQL -> PostgreSQL / MySQL -> openGauss 的预迁移")
		}
	case db.DatabaseTypePostgreSQL:
		if targetType != db.DatabaseTypeMySQL && targetType != db.DatabaseTypeOpenGauss {
			return errors.New("仅支持 PostgreSQL -> MySQL / PostgreSQL -> openGauss 的预迁移")
		}
	case db.DatabaseTypeOpenGauss:
		if targetType != db.DatabaseTypeMySQL && targetType != db.DatabaseTypePostgreSQL {
			return errors.New("仅支持 openGauss -> MySQL / openGauss -> PostgreSQL 的预迁移")
		}
	case db.DatabaseTypeSQLite:
		if targetType != db.DatabaseTypePostgreSQL && targetType != db.DatabaseTypeMySQL && targetType != db.DatabaseTypeOpenGauss {
			return errors.New("仅支持 SQLite -> PostgreSQL / SQLite -> MySQL / SQLite -> openGauss 的预迁移")
		}
	default:
		return errors.New("不支持的源数据库类型")
	}
	return nil
}
