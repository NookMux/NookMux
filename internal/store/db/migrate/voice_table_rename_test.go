package dbmigrate

import (
	"fmt"
	"testing"

	dbstore "github.com/NookMux/NookMux/internal/store/db"
	"github.com/NookMux/NookMux/internal/store/voice"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func setupRenameTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	oldDB := dbstore.DB
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite test db: %v", err)
	}
	dbstore.DB = db
	t.Cleanup(func() {
		if sqlDB, err := db.DB(); err == nil {
			_ = sqlDB.Close()
		}
		dbstore.DB = oldDB
	})
	return db
}

func createLegacyMinimaxVoicesTable(t *testing.T, db *gorm.DB, rows int) {
	t.Helper()
	if err := db.Exec(`CREATE TABLE minimax_voices (
		id integer PRIMARY KEY AUTOINCREMENT,
		created_at bigint,
		updated_at bigint,
		type varchar(16) NOT NULL,
		operator_id integer,
		operator_kind varchar(16),
		voice_id varchar(256) NOT NULL,
		quota_cost bigint DEFAULT 0,
		redirect_id varchar(256),
		allowed numeric DEFAULT 0,
		remark varchar(255)
	)`).Error; err != nil {
		t.Fatalf("create legacy table: %v", err)
	}
	for _, idx := range legacyMinimaxVoiceIndexes {
		unique := ""
		if idx == "uk_minimax_voice_id" {
			unique = "UNIQUE "
		}
		if err := db.Exec(fmt.Sprintf("CREATE %sINDEX %s ON minimax_voices (%s)", unique, idx, legacyIndexColumn(idx))).Error; err != nil {
			t.Fatalf("create legacy index %s: %v", idx, err)
		}
	}
	for i := range rows {
		if err := db.Exec(`INSERT INTO minimax_voices (voice_id, type, allowed, created_at, updated_at) VALUES (?, ?, 1, 1700000000, 1700000000)`,
			fmt.Sprintf("voice-%d", i), voicestore.VoiceTypeCreated).Error; err != nil {
			t.Fatalf("seed legacy row: %v", err)
		}
	}
}

func legacyIndexColumn(name string) string {
	switch name {
	case "idx_minimax_voice_created_at":
		return "created_at"
	case "idx_minimax_voice_type":
		return "type"
	case "idx_minimax_voice_operator_id":
		return "operator_id"
	case "uk_minimax_voice_id":
		return "voice_id"
	}
	return "id"
}

func TestRenameLegacyMinimaxVoicesTable_RenamesAndKeepsData(t *testing.T) {
	db := setupRenameTestDB(t)
	createLegacyMinimaxVoicesTable(t, db, 2)

	if err := renameLegacyMinimaxVoicesTable(); err != nil {
		t.Fatalf("renameLegacyMinimaxVoicesTable error: %v", err)
	}
	if err := db.AutoMigrate(&voicestore.Voice{}); err != nil {
		t.Fatalf("AutoMigrate after rename: %v", err)
	}

	if db.Migrator().HasTable("minimax_voices") {
		t.Fatalf("legacy table should be gone")
	}
	if !db.Migrator().HasTable(&voicestore.Voice{}) {
		t.Fatalf("voices table should exist")
	}
	var cnt int64
	if err := db.Model(&voicestore.Voice{}).Count(&cnt).Error; err != nil {
		t.Fatalf("count voices: %v", err)
	}
	if cnt != 2 {
		t.Fatalf("row count = %d, want 2 (data must survive rename)", cnt)
	}
	for _, idx := range []string{"idx_voices_created_at", "idx_voices_type", "idx_voices_operator_id", "uk_voices_voice_id"} {
		if !db.Migrator().HasIndex(&voicestore.Voice{}, idx) {
			t.Fatalf("new index %s should exist after AutoMigrate", idx)
		}
	}
	for _, idx := range legacyMinimaxVoiceIndexes {
		if db.Migrator().HasIndex(&voicestore.Voice{}, idx) {
			t.Fatalf("legacy index %s should be dropped", idx)
		}
	}

	// 幂等：再次运行不应报错。
	if err := renameLegacyMinimaxVoicesTable(); err != nil {
		t.Fatalf("second run should be a no-op, got: %v", err)
	}
}

func TestRenameLegacyMinimaxVoicesTable_SkipsWhenAbsent(t *testing.T) {
	db := setupRenameTestDB(t)
	if err := db.AutoMigrate(&voicestore.Voice{}); err != nil {
		t.Fatalf("AutoMigrate: %v", err)
	}
	if err := renameLegacyMinimaxVoicesTable(); err != nil {
		t.Fatalf("expected no-op on fresh install, got: %v", err)
	}
}

func TestRenameLegacyMinimaxVoicesTable_MergesRowsWhenBothExist(t *testing.T) {
	db := setupRenameTestDB(t)
	createLegacyMinimaxVoicesTable(t, db, 1)
	if err := db.AutoMigrate(&voicestore.Voice{}); err != nil {
		t.Fatalf("AutoMigrate: %v", err)
	}
	// voices 已有重命名迁移带来的记录（id=1，voice-0）与一条占用 id=2 的无关记录；
	// 旧表含与 voices 重复的 voice-0 及回滚窗口新增的 legacy-only（旧表 id=2，
	// 与 voices 的 id=2 冲突）。合并必须跳过重复 voice_id、为 legacy-only 重分配主键。
	for _, vid := range []string{"voice-0", "filler"} {
		v := voicestore.Voice{CreatedAt: 1, UpdatedAt: 1, Type: voicestore.VoiceTypeCreated, VoiceId: vid}
		if err := db.Create(&v).Error; err != nil {
			t.Fatalf("seed voices row %s: %v", vid, err)
		}
	}
	if err := db.Exec(`INSERT INTO minimax_voices (voice_id, type, allowed, created_at, updated_at) VALUES (?, ?, 1, 1700000000, 1700000000)`,
		"legacy-only", voicestore.VoiceTypeCreated).Error; err != nil {
		t.Fatalf("seed legacy-only row: %v", err)
	}

	if err := renameLegacyMinimaxVoicesTable(); err != nil {
		t.Fatalf("expected legacy rows to be merged, got: %v", err)
	}
	if db.Migrator().HasTable("minimax_voices") {
		t.Fatalf("legacy table should be dropped after merge")
	}
	var cnt int64
	if err := db.Model(&voicestore.Voice{}).Count(&cnt).Error; err != nil {
		t.Fatalf("count voices: %v", err)
	}
	if cnt != 3 {
		t.Fatalf("row count = %d, want 3 (duplicate skipped, legacy-only migrated)", cnt)
	}
	var migrated voicestore.Voice
	if err := db.Where("voice_id = ?", "legacy-only").First(&migrated).Error; err != nil {
		t.Fatalf("legacy-only row should survive merge: %v", err)
	}
	if migrated.Id == 2 {
		t.Fatalf("migrated row should get a fresh primary key instead of the conflicting legacy id 2")
	}
	if migrated.Type != voicestore.VoiceTypeCreated || migrated.CreatedAt != 1700000000 || !migrated.Allowed {
		t.Fatalf("migrated row fields should be preserved, got %+v", migrated)
	}

	// 幂等：再次运行不应报错。
	if err := renameLegacyMinimaxVoicesTable(); err != nil {
		t.Fatalf("second run should be a no-op, got: %v", err)
	}
}

func TestRenameLegacyMinimaxVoicesTable_DropsEmptyLegacyWhenBothExist(t *testing.T) {
	db := setupRenameTestDB(t)
	createLegacyMinimaxVoicesTable(t, db, 0)
	if err := db.AutoMigrate(&voicestore.Voice{}); err != nil {
		t.Fatalf("AutoMigrate: %v", err)
	}
	if err := renameLegacyMinimaxVoicesTable(); err != nil {
		t.Fatalf("expected empty legacy table to be dropped, got: %v", err)
	}
	if db.Migrator().HasTable("minimax_voices") {
		t.Fatalf("empty legacy table should be dropped")
	}
}

func TestRenameLegacyMinimaxVoicesTable_CleansResidualLegacyIndexesWithoutLegacyTable(t *testing.T) {
	db := setupRenameTestDB(t)
	if err := db.AutoMigrate(&voicestore.Voice{}); err != nil {
		t.Fatalf("AutoMigrate: %v", err)
	}
	// 模拟改名成功后 DropIndex 中途失败：旧表已不存在，voices 上残留旧名索引。
	if err := db.Exec("CREATE INDEX idx_minimax_voice_type ON voices (type)").Error; err != nil {
		t.Fatalf("seed residual legacy index: %v", err)
	}
	if err := renameLegacyMinimaxVoicesTable(); err != nil {
		t.Fatalf("expected residual legacy index cleanup, got: %v", err)
	}
	if db.Migrator().HasIndex(&voicestore.Voice{}, "idx_minimax_voice_type") {
		t.Fatalf("residual legacy index should be dropped")
	}
}
