package dbmigrate

import (
	infradb "github.com/NookMux/NookMux/internal/infra/db"
	"github.com/NookMux/NookMux/internal/store/audit"
	"github.com/NookMux/NookMux/internal/store/channel"
	"github.com/NookMux/NookMux/internal/store/checkin"
	"github.com/NookMux/NookMux/internal/store/db"
	"github.com/NookMux/NookMux/internal/store/db/cleanup"
	"github.com/NookMux/NookMux/internal/store/log"
	"github.com/NookMux/NookMux/internal/store/option"
	"github.com/NookMux/NookMux/internal/store/passkey"
	"github.com/NookMux/NookMux/internal/store/prefill_group"
	"github.com/NookMux/NookMux/internal/store/redemption"
	"github.com/NookMux/NookMux/internal/store/stored_media"
	"github.com/NookMux/NookMux/internal/store/ticket"
	"github.com/NookMux/NookMux/internal/store/token"
	"github.com/NookMux/NookMux/internal/store/topup"
	"github.com/NookMux/NookMux/internal/store/twofa"
	"github.com/NookMux/NookMux/internal/store/usedata"
	"github.com/NookMux/NookMux/internal/store/user"
	"github.com/NookMux/NookMux/internal/store/vendor_meta"
	"github.com/NookMux/NookMux/internal/store/voice"
	"gorm.io/gorm"
)

func autoMigrateTargetMainSchema(db *gorm.DB) error {
	// 同 dbmigrate.cleanupLegacyUniqueIndexes 的说明，pre-migrate 路径也需要清理
	if infradb.UsingPostgreSQL {
		dbstore.CleanupLegacyUniqueConstraints(db, "prefill_groups", "name", []string{"uni_prefill_groups_name", "idx_prefill_groups_name"})
		dbstore.CleanupLegacyUniqueConstraints(db, "models", "model_name", []string{"uni_models_model_name", "idx_models_model_name"})
		dbstore.CleanupLegacyUniqueConstraints(db, "vendors", "name", []string{"uni_vendors_name", "idx_vendors_name"})
		dbstore.CleanupLegacyUniqueConstraints(db, "passkey_credentials", "user_id", []string{"uni_passkey_credentials_user_id", "idx_passkey_credentials_user_id"})
	}
	if infradb.UsingSQLite {
		dbstore.CleanupLegacyUniqueConstraints(db, "passkey_credentials", "user_id", []string{"uni_passkey_credentials_user_id", "idx_passkey_credentials_user_id"})
	}
	if infradb.UsingMySQL {
		dbstore.CleanupLegacyUniqueConstraints(db, "passkey_credentials", "user_id", []string{"uni_passkey_credentials_user_id", "idx_passkey_credentials_user_id"})
	}
	if db.Dialector.Name() == infradb.DatabaseTypeDM {
		if err := dbstore.NormalizeDMBoolDefaultLiterals(db, targetMainModels()); err != nil {
			return err
		}
	}
	if err := db.AutoMigrate(targetMainModels()...); err != nil {
		return err
	}
	dbcleanup.CleanupRemovedQuotaDataCacheStats(db)
	return nil
}

// targetMainModels 迁移目标库的主库 AutoMigrate 模型清单（与 mainDBModels
// 的差异仅在于这是面向"目标库"的重复清单，见上方注释）。
func targetMainModels() []any {
	return []any{
		&channelstore.Channel{},
		&tokenstore.Token{},
		&userstore.User{},
		&passkeystore.PasskeyCredential{},
		&optionstore.Option{},
		&redemptionstore.Redemption{},
		&channelstore.Ability{},
		&logstore.Log{},
		&storedmediastore.StoredMedia{},
		&topupstore.TopUp{},
		&usedatastore.QuotaData{},
		&vendormetastore.Model{},
		&vendormetastore.Vendor{},
		&prefillgroupstore.PrefillGroup{},
		&optionstore.Setup{},
		&twofastore.TwoFA{},
		&twofastore.TwoFABackupCode{},
		&checkinstore.Checkin{},
		// The following tables are also part of the main startup AutoMigrate and
		// must be created on the target DB as well, otherwise the pre-migration /
		// same-type migration would leave them missing and their data un-copied.
		&ticketstore.Ticket{},
		&ticketstore.TicketEntry{},
		&channelstore.DynamicRatioRule{},
		&auditstore.AuditLog{},
		&voicestore.Voice{},
	}
}

func autoMigrateTargetLogSchema(db *gorm.DB) error {
	if db.Dialector.Name() == infradb.DatabaseTypeDM {
		if err := dbstore.NormalizeDMBoolDefaultLiterals(db, []any{&logstore.Log{}}); err != nil {
			return err
		}
	}
	return db.AutoMigrate(&logstore.Log{})
}
