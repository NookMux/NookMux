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
	case db.UsingKingbase:
		return db.DatabaseTypeKingbase, nil
	case db.UsingPostgreSQL:
		return db.DatabaseTypePostgreSQL, nil
	case db.UsingMySQL:
		return db.DatabaseTypeMySQL, nil
	case db.UsingDM:
		return db.DatabaseTypeDM, nil
	case db.UsingSQLite:
		return db.DatabaseTypeSQLite, nil
	default:
		return "", errors.New("无法识别当前主数据库类型（UsingMySQL/UsingPostgreSQL/UsingDM/UsingSQLite 均为 false）")
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
	case db.DatabaseTypePostgreSQL, db.DatabaseTypeOpenGauss, db.DatabaseTypeMySQL, db.DatabaseTypeDM, db.DatabaseTypeSQLite, db.DatabaseTypeKingbase:
		return db.LogSqlType, nil
	default:
		return "", errors.New("无法识别当前日志数据库类型")
	}
}

// preMigratableTargetTypes 是预迁移允许的目标数据库类型集合（SQLite 仅可
// 作为源）。金仓 KingbaseES 为 PG 兼容内核，与现有四库经 GORM 通用拷贝
// 链路互迁的列类型映射一致。
var preMigratableTargetTypes = map[string]bool{
	db.DatabaseTypeMySQL:      true,
	db.DatabaseTypePostgreSQL: true,
	db.DatabaseTypeOpenGauss:  true,
	db.DatabaseTypeDM:         true,
	db.DatabaseTypeKingbase:   true,
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
		if !preMigratableTargetTypes[targetType] {
			return errors.New("仅支持 MySQL -> PostgreSQL / MySQL -> openGauss / MySQL -> DM / MySQL -> KingbaseES 的预迁移")
		}
	case db.DatabaseTypePostgreSQL:
		if !preMigratableTargetTypes[targetType] {
			return errors.New("仅支持 PostgreSQL -> MySQL / PostgreSQL -> openGauss / PostgreSQL -> DM / PostgreSQL -> KingbaseES 的预迁移")
		}
	case db.DatabaseTypeOpenGauss:
		if !preMigratableTargetTypes[targetType] {
			return errors.New("仅支持 openGauss -> MySQL / openGauss -> PostgreSQL / openGauss -> DM / openGauss -> KingbaseES 的预迁移")
		}
	case db.DatabaseTypeDM:
		if !preMigratableTargetTypes[targetType] {
			return errors.New("仅支持 DM -> MySQL / DM -> PostgreSQL / DM -> openGauss / DM -> KingbaseES 的预迁移")
		}
	case db.DatabaseTypeKingbase:
		if !preMigratableTargetTypes[targetType] {
			return errors.New("仅支持 KingbaseES -> MySQL / KingbaseES -> PostgreSQL / KingbaseES -> openGauss / KingbaseES -> DM 的预迁移")
		}
	case db.DatabaseTypeSQLite:
		if !preMigratableTargetTypes[targetType] {
			return errors.New("仅支持 SQLite -> PostgreSQL / SQLite -> MySQL / SQLite -> openGauss / SQLite -> DM / SQLite -> KingbaseES 的预迁移")
		}
	default:
		return errors.New("不支持的源数据库类型")
	}
	return nil
}
