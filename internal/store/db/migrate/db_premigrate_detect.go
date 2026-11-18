package dbmigrate

import (
	"strings"

	"github.com/NookMux/NookMux/internal/common"
	"github.com/NookMux/NookMux/internal/i18n"
	"github.com/NookMux/NookMux/internal/infra/db"
)

func currentMainDBType() (string, error) {
	// 具体产品旗标必须先于 UsingPostgreSQL 判断：openGauss/金仓主库会同时
	// 置 UsingPostgreSQL=true（PG 兼容内核），先判家族旗标会误报为 postgres。
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
		return "", i18n.NewError(i18n.MsgDBMigrateDetectMainTypeFail)
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
		return "", i18n.NewError(i18n.MsgDBMigrateDetectLogTypeFail)
	}
}

// preMigratableTargets 是预迁移允许的目标数据库类型及展示名的有序清单
// （SQLite 仅可作为源）。金仓 KingbaseES 为 PG 兼容内核，与现有四库经
// GORM 通用拷贝链路互迁的列类型映射一致。校验集合与报错文案均由本清单
// 派生，保持单一真值。
var preMigratableTargets = []struct{ Type, Name string }{
	{db.DatabaseTypeMySQL, "MySQL"},
	{db.DatabaseTypePostgreSQL, "PostgreSQL"},
	{db.DatabaseTypeOpenGauss, "openGauss"},
	{db.DatabaseTypeDM, "DM"},
	{db.DatabaseTypeKingbase, "KingbaseES"},
}

var preMigratableTargetTypes = func() map[string]bool {
	types := make(map[string]bool, len(preMigratableTargets))
	for _, t := range preMigratableTargets {
		types[t.Type] = true
	}
	return types
}()

func preMigrateDBName(dbType string) string {
	if dbType == db.DatabaseTypeSQLite {
		return "SQLite"
	}
	for _, t := range preMigratableTargets {
		if t.Type == dbType {
			return t.Name
		}
	}
	return dbType
}

func preMigratableTargetNames() string {
	names := make([]string, 0, len(preMigratableTargets))
	for _, t := range preMigratableTargets {
		names = append(names, t.Name)
	}
	return strings.Join(names, " / ")
}

func validateDBPreMigrateDirection(sourceType string, targetType string) error {
	if sourceType == "" || targetType == "" {
		return i18n.NewError(i18n.MsgDBMigrateTypeEmpty)
	}
	if sourceType == targetType {
		return i18n.NewError(i18n.MsgDBMigrateTypesEqual)
	}
	if sourceType != db.DatabaseTypeSQLite && !preMigratableTargetTypes[sourceType] {
		return i18n.NewError(i18n.MsgDBMigratePreUnsupportedSrc)
	}
	if !preMigratableTargetTypes[targetType] {
		return i18n.NewError(i18n.MsgDBMigratePreOnlySupported, map[string]any{
			"Source": preMigrateDBName(sourceType),
			"Target": preMigratableTargetNames(),
		})
	}
	return nil
}
