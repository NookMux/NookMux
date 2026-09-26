package dbmigrate

import (
	"github.com/NookMux/NookMux/internal/store/audit"
	"github.com/NookMux/NookMux/internal/store/channel"
	"github.com/NookMux/NookMux/internal/store/checkin"
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
)

var dbPreMigrateMainSteps = []dbPreMigrateStep{
	gormTableCopyStep[userstore.User]{name: "users", batchSize: dbPreMigrateBatchDefault},
	gormTableCopyStep[optionstore.Option]{name: "options", batchSize: dbPreMigrateBatchDefault},
	gormTableCopyStep[channelstore.Channel]{name: "channels", batchSize: dbPreMigrateBatchDefault},
	gormTableCopyStep[tokenstore.Token]{name: "tokens", batchSize: dbPreMigrateBatchDefault},
	gormTableCopyStep[passkeystore.PasskeyCredential]{name: "passkey_credentials", batchSize: dbPreMigrateBatchDefault},
	gormTableCopyStep[redemptionstore.Redemption]{name: "redemptions", batchSize: dbPreMigrateBatchDefault},
	gormTableCopyStep[channelstore.Ability]{name: "abilities", batchSize: dbPreMigrateBatchDefault},
	gormTableCopyStep[storedmediastore.StoredMedia]{name: "stored_media", batchSize: dbPreMigrateBatchBlob},
	gormTableCopyStep[topupstore.TopUp]{name: "top_ups", batchSize: dbPreMigrateBatchDefault},
	gormTableCopyStep[usedatastore.QuotaData]{name: "quota_data", batchSize: dbPreMigrateBatchDefault},
	gormTableCopyStep[vendormetastore.Model]{name: "models", batchSize: dbPreMigrateBatchDefault},
	gormTableCopyStep[vendormetastore.Vendor]{name: "vendors", batchSize: dbPreMigrateBatchDefault},
	gormTableCopyStep[prefillgroupstore.PrefillGroup]{name: "prefill_groups", batchSize: dbPreMigrateBatchDefault},
	gormTableCopyStep[optionstore.Setup]{name: "setups", batchSize: dbPreMigrateBatchDefault},
	gormTableCopyStep[twofastore.TwoFA]{name: "two_fas", batchSize: dbPreMigrateBatchDefault},
	gormTableCopyStep[twofastore.TwoFABackupCode]{name: "two_fa_backup_codes", batchSize: dbPreMigrateBatchDefault},
	gormTableCopyStep[checkinstore.Checkin]{name: "checkins", batchSize: dbPreMigrateBatchDefault},
	// Tables kept in sync with dbmigrate.migrateDB()'s AutoMigrate list. They were
	// previously omitted here, which would drop their data on a pre-/same-type DB
	// migration. Voice uses a custom TableName ("voices").
	gormTableCopyStep[ticketstore.Ticket]{name: "tickets", batchSize: dbPreMigrateBatchDefault},
	gormTableCopyStep[ticketstore.TicketEntry]{name: "ticket_entries", batchSize: dbPreMigrateBatchDefault},
	gormTableCopyStep[channelstore.DynamicRatioRule]{name: "dynamic_ratio_rules", batchSize: dbPreMigrateBatchDefault},
	gormTableCopyStep[auditstore.AuditLog]{name: "audit_logs", batchSize: dbPreMigrateBatchDefault},
	gormTableCopyStep[voicestore.Voice]{name: "voices", batchSize: dbPreMigrateBatchDefault},
}

var dbPreMigrateLogStep = gormTableCopyStep[logstore.Log]{name: "logs", batchSize: dbPreMigrateBatchLog}
