package dbmigrate

import (
	infradb "github.com/NookMux/NookMux/internal/infra/db"
	"github.com/NookMux/NookMux/internal/store/db"
	"github.com/NookMux/NookMux/internal/store/db/cleanup"
	"github.com/NookMux/NookMux/internal/store/log"
	"gorm.io/gorm"
)

func autoMigrateTargetMainSchema(db *gorm.DB) error {
	// 同 dbmigrate.cleanupLegacyUniqueIndexes 的说明，pre-migrate 路径也需要清理；
	// 此处 db 是迁移目标库连接，必须按目标方言门控，而不是当前源库的全局旗标
	//（openGauss 目标复用 postgres 方言，Dialector.Name 同为 "postgres"）。
	switch db.Dialector.Name() {
	case infradb.DatabaseTypePostgreSQL, infradb.DatabaseTypeKingbase:
		dbstore.CleanupLegacyUniqueConstraints(db, "prefill_groups", "name", []string{"uni_prefill_groups_name", "idx_prefill_groups_name"})
		dbstore.CleanupLegacyUniqueConstraints(db, "models", "model_name", []string{"uni_models_model_name", "idx_models_model_name"})
		dbstore.CleanupLegacyUniqueConstraints(db, "vendors", "name", []string{"uni_vendors_name", "idx_vendors_name"})
		dbstore.CleanupLegacyUniqueConstraints(db, "passkey_credentials", "user_id", []string{"uni_passkey_credentials_user_id", "idx_passkey_credentials_user_id"})
	case infradb.DatabaseTypeSQLite, infradb.DatabaseTypeMySQL:
		dbstore.CleanupLegacyUniqueConstraints(db, "passkey_credentials", "user_id", []string{"uni_passkey_credentials_user_id", "idx_passkey_credentials_user_id"})
	}
	if db.Dialector.Name() == infradb.DatabaseTypeDM {
		if err := dbstore.NormalizeDMBoolDefaultLiterals(db, mainDBModels()); err != nil {
			return err
		}
	}
	if err := db.AutoMigrate(mainDBModels()...); err != nil {
		return err
	}
	dbcleanup.CleanupRemovedQuotaDataCacheStats(db)
	return nil
}

func autoMigrateTargetLogSchema(db *gorm.DB) error {
	if db.Dialector.Name() == infradb.DatabaseTypeDM {
		if err := dbstore.NormalizeDMBoolDefaultLiterals(db, []any{&logstore.Log{}}); err != nil {
			return err
		}
	}
	return db.AutoMigrate(&logstore.Log{})
}
