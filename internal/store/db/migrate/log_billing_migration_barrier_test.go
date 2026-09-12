package dbmigrate

import (
	"errors"
	"strings"
	"testing"

	"github.com/NookMux/NookMux/internal/store/db"
	"github.com/NookMux/NookMux/internal/store/log"
	"gorm.io/gorm"
)

func TestBillingMigrationBarrierRejectsPartialAndWrongDatabase(t *testing.T) {
	mainDB, logDB := setupBillingDetailsMigrateTestDB(t)
	for _, db := range []*gorm.DB{mainDB, logDB} {
		if err := db.AutoMigrate(&logstore.Log{}); err != nil {
			t.Fatal(err)
		}
		addLegacyAggregateColumns(t, db)
	}
	seedTokenMigrationLog(t, logDB, logstore.Log{Type: logstore.LogTypeConsume, Other: `{"cache_tokens":-1}`})
	// A completion marker in the main database cannot authorize an independent log DB.
	if err := mainDB.AutoMigrate(&logBillingMigrationState{}); err != nil {
		t.Fatal(err)
	}
	if err := mainDB.Create(&logBillingMigrationState{Version: 1}).Error; err != nil {
		t.Fatal(err)
	}
	if err := ensureLogBillingDetailsColumn(logDB, "log"); err == nil {
		t.Fatal("accepted wrong database marker")
	}
	if err := backfillLogBillingTokenDetails(); err == nil {
		t.Fatal("accepted invalid history")
	}
	if err := ensureLogBillingDetailsColumn(logDB, "log"); err == nil {
		t.Fatal("accepted partial migration")
	}
	if err := logDB.Model(&logstore.Log{}).Where("id > ?", 0).Update("other", "{}").Error; err != nil {
		t.Fatal(err)
	}
	if err := backfillLogBillingTokenDetails(); err != nil {
		t.Fatal(err)
	}
	if err := ensureLogBillingDetailsColumn(logDB, "log"); err != nil {
		t.Fatal(err)
	}
	// Importing legacy data requires stopped readers/writers and explicit marker invalidation.
	if err := logDB.Where("version = ?", 1).Delete(&logBillingMigrationState{}).Error; err != nil {
		t.Fatal(err)
	}
	seedTokenMigrationLog(t, logDB, logstore.Log{Other: `{"cache_tokens":-1}`})
	if err := backfillLogBillingTokenDetails(); err == nil {
		t.Fatal("accepted invalid imported history")
	}
	if err := ensureLogBillingDetailsColumn(logDB, "log"); err == nil {
		t.Fatal("stale marker survived failed migration")
	}
}

func TestBillingMigrationRetriesAtomicBatchAndAdvancesCursor(t *testing.T) {
	db := setupTokenDetailsMigrationDB(t)
	oldDB, oldSize := dbstore.LOG_DB, logBillingTokenDetailsBatchSize
	dbstore.LOG_DB, logBillingTokenDetailsBatchSize = db, 2
	t.Cleanup(func() { dbstore.LOG_DB, logBillingTokenDetailsBatchSize = oldDB, oldSize })
	for i := 0; i < 5; i++ {
		seedTokenMigrationLog(t, db, logstore.Log{Type: logstore.LogTypeConsume, PromptTokens: 5, Other: "{}"})
	}
	attempts := 0
	if err := db.Callback().Update().Before("gorm:update").Register("test:transient", func(tx *gorm.DB) {
		if tx.Statement.Table == "logs" {
			attempts++
			if attempts == 2 {
				tx.AddError(errors.New("temporary database failure"))
			}
		}
	}); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Callback().Update().Remove("test:transient") })
	var cursors []int
	if err := db.Callback().Query().After("gorm:query").Register("test:cursor", func(tx *gorm.DB) {
		if tx.Statement.Table == "logs" && strings.Contains(tx.Statement.SQL.String(), "id >") {
			cursors = append(cursors, tx.Statement.Vars[0].(int))
		}
	}); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Callback().Query().Remove("test:cursor") })
	if err := backfillLogBillingTokenDetails(); err != nil {
		t.Fatal(err)
	}
	if attempts != 7 {
		t.Fatalf("update attempts=%d, want 7 including rolled-back batch", attempts)
	}
	if len(cursors) != 4 || cursors[0] != 0 || cursors[1] != 2 || cursors[2] != 4 || cursors[3] != 5 {
		t.Fatalf("cursors=%v", cursors)
	}
	for id := 1; id <= 5; id++ {
		row := tokenMigrationStoredRow(t, db, id)
		if row.BillingDetailsVersion != 1 || row.BillingDetails == nil {
			t.Fatalf("unmigrated row %d", id)
		}
	}
}

func TestBillingMigrationEmptyUsagePreservesExplicitZero(t *testing.T) {
	empty, _, _, err := migrateLegacyBillingDetails(legacyBillingDetailsRow{Type: logstore.LogTypeConsume, Other: "{}"})
	if err != nil || empty != nil {
		t.Fatalf("empty usage: details=%v err=%v", empty, err)
	}
	explicit, _, _, err := migrateLegacyBillingDetails(legacyBillingDetailsRow{Type: logstore.LogTypeConsume, Other: `{"text_input":0}`})
	if err != nil || explicit == nil {
		t.Fatalf("explicit zero: details=%v err=%v", explicit, err)
	}
	requireTokenValue(t, requireTokenDetails(t, explicit).Tokens.Input.TextInput, 0)
}

func TestBillingMigrationDatabaseRetriesAreBounded(t *testing.T) {
	db := setupTokenDetailsMigrationDB(t)
	attempts := 0
	failure := errors.New("database unavailable")
	err := retryLogBillingMigration(db, func(tx *gorm.DB) error {
		attempts++
		if _, ok := tx.Statement.Context.Deadline(); !ok {
			t.Fatal("missing database timeout")
		}
		return failure
	})
	if !errors.Is(err, failure) || attempts != 3 {
		t.Fatalf("attempts=%d err=%v", attempts, err)
	}
}

func TestBillingMigrationCompletionMarkerFailureBlocksSlave(t *testing.T) {
	db := setupTokenDetailsMigrationDB(t)
	oldDB := dbstore.LOG_DB
	dbstore.LOG_DB = db
	t.Cleanup(func() { dbstore.LOG_DB = oldDB })
	seedTokenMigrationLog(t, db, logstore.Log{Type: logstore.LogTypeConsume, PromptTokens: 2, Other: "{}"})
	if err := db.Callback().Create().Before("gorm:create").Register("test:marker-failure", func(tx *gorm.DB) {
		if tx.Statement.Table == "log_billing_migration_states" {
			tx.AddError(errors.New("marker write unavailable"))
		}
	}); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Callback().Create().Remove("test:marker-failure") })
	if err := backfillLogBillingTokenDetails(); err == nil {
		t.Fatal("marker write failure must block startup")
	}
	if err := ensureLogBillingDetailsColumn(db, "log"); err == nil {
		t.Fatal("slave accepted missing marker")
	}
	if tokenMigrationStoredRow(t, db, 1).BillingDetailsVersion != 1 {
		t.Fatal("committed batch lost")
	}
	db.Callback().Create().Remove("test:marker-failure")
	if err := backfillLogBillingTokenDetails(); err != nil {
		t.Fatal(err)
	}
	// Completed restarts must not query the large logs table again.
	if err := db.Callback().Query().Before("gorm:query").Register("test:no-rescan", func(tx *gorm.DB) {
		if tx.Statement.Table == "logs" {
			tx.AddError(errors.New("unexpected logs rescan"))
		}
	}); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Callback().Query().Remove("test:no-rescan") })
	if err := backfillLogBillingTokenDetails(); err != nil {
		t.Fatal(err)
	}
}
