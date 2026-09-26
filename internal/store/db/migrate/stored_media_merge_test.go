package dbmigrate

import (
	"fmt"
	"testing"

	"github.com/NookMux/NookMux/internal/store/stored_media"
	"gorm.io/gorm"
)

func createLegacyStoredMediaTable(t *testing.T, db *gorm.DB, table string, rows int, seed string) {
	t.Helper()
	if err := db.Exec(fmt.Sprintf(`CREATE TABLE %s (
		id varchar(64) PRIMARY KEY,
		user_id integer,
		channel_id integer,
		created_at bigint,
		mime_type varchar(255),
		size_bytes integer,
		sha256 char(64),
		data blob
	)`, table)).Error; err != nil {
		t.Fatalf("create legacy table %s: %v", table, err)
	}
	for i := range rows {
		if err := db.Exec(
			fmt.Sprintf("INSERT INTO %s (id, user_id, channel_id, created_at, mime_type, size_bytes, sha256, data) VALUES (?, ?, ?, ?, ?, ?, ?, ?)", table),
			fmt.Sprintf("%s-id-%04d", seed, i), 7, 3, int64(1700000000+i), "application/octet-stream", 10+i,
			fmt.Sprintf("%s-sha-%04d", seed, i), fmt.Appendf(nil, "%s-bytes-%d", seed, i),
		).Error; err != nil {
			t.Fatalf("seed legacy row %s/%d: %v", table, i, err)
		}
	}
}

func countStoredMediaByType(t *testing.T, db *gorm.DB, mediaType string) int64 {
	t.Helper()
	var cnt int64
	if err := db.Model(&storedmediastore.StoredMedia{}).Where("media_type = ?", mediaType).Count(&cnt).Error; err != nil {
		t.Fatalf("count stored_media %s: %v", mediaType, err)
	}
	return cnt
}

func TestMergeLegacyStoredMediaTables_MergesBothLegacyTables(t *testing.T) {
	db := setupRenameTestDB(t)
	createLegacyStoredMediaTable(t, db, legacyStoredImagesTable, 2, "img")
	createLegacyStoredMediaTable(t, db, legacyStoredVideosTable, 3, "vid")

	if err := db.AutoMigrate(&storedmediastore.StoredMedia{}); err != nil {
		t.Fatalf("AutoMigrate: %v", err)
	}
	if err := mergeLegacyStoredMediaTables(); err != nil {
		t.Fatalf("mergeLegacyStoredMediaTables error: %v", err)
	}

	if db.Migrator().HasTable(legacyStoredImagesTable) || db.Migrator().HasTable(legacyStoredVideosTable) {
		t.Fatalf("legacy tables should be dropped after merge")
	}
	if got := countStoredMediaByType(t, db, storedmediastore.MediaTypeImage); got != 2 {
		t.Fatalf("image rows = %d, want 2", got)
	}
	if got := countStoredMediaByType(t, db, storedmediastore.MediaTypeVideo); got != 3 {
		t.Fatalf("video rows = %d, want 3", got)
	}

	// 数据内容随行迁移，不被清空或改写。
	var m storedmediastore.StoredMedia
	if err := db.Where("id = ?", "img-id-0000").First(&m).Error; err != nil {
		t.Fatalf("load migrated image row: %v", err)
	}
	if m.MediaType != storedmediastore.MediaTypeImage || string(m.Data) != "img-bytes-0" || m.Sha256 != "img-sha-0000" {
		t.Fatalf("migrated row content mismatch: %+v", m)
	}
	var v storedmediastore.StoredMedia
	if err := db.Where("id = ?", "vid-id-0002").First(&v).Error; err != nil {
		t.Fatalf("load migrated video row: %v", err)
	}
	if v.MediaType != storedmediastore.MediaTypeVideo || string(v.Data) != "vid-bytes-2" {
		t.Fatalf("migrated video row content mismatch: %+v", v)
	}

	// 幂等：合并完成后再次运行为 no-op。
	if err := mergeLegacyStoredMediaTables(); err != nil {
		t.Fatalf("second run should be a no-op, got: %v", err)
	}
	if got := countStoredMediaByType(t, db, storedmediastore.MediaTypeImage); got != 2 {
		t.Fatalf("image rows after rerun = %d, want 2", got)
	}
}

func TestMergeLegacyStoredMediaTables_SkipsOnFreshInstall(t *testing.T) {
	db := setupRenameTestDB(t)
	if err := db.AutoMigrate(&storedmediastore.StoredMedia{}); err != nil {
		t.Fatalf("AutoMigrate: %v", err)
	}
	if err := mergeLegacyStoredMediaTables(); err != nil {
		t.Fatalf("expected no-op on fresh install, got: %v", err)
	}
}

func TestMergeLegacyStoredMediaTables_ResumesAfterInterruption(t *testing.T) {
	db := setupRenameTestDB(t)
	createLegacyStoredMediaTable(t, db, legacyStoredVideosTable, 2, "vid")

	if err := db.AutoMigrate(&storedmediastore.StoredMedia{}); err != nil {
		t.Fatalf("AutoMigrate: %v", err)
	}
	// 模拟上次合并中途失败：首行已拷入 stored_media，旧表仍在。
	partial := storedmediastore.StoredMedia{
		Id: "vid-id-0000", UserId: 7, MediaType: storedmediastore.MediaTypeVideo,
		ChannelId: 3, CreatedAt: 1700000000, MimeType: "application/octet-stream",
		SizeBytes: 10, Sha256: "vid-sha-0000", Data: storedmediastore.LargeBlob("vid-bytes-0"),
	}
	if err := db.Create(&partial).Error; err != nil {
		t.Fatalf("seed partial migrated row: %v", err)
	}

	if err := mergeLegacyStoredMediaTables(); err != nil {
		t.Fatalf("merge should resume without conflict, got: %v", err)
	}
	if db.Migrator().HasTable(legacyStoredVideosTable) {
		t.Fatalf("legacy table should be dropped after resumed merge")
	}
	if got := countStoredMediaByType(t, db, storedmediastore.MediaTypeVideo); got != 2 {
		t.Fatalf("video rows = %d, want 2 (no duplicated partial row)", got)
	}
}

func TestMergeLegacyStoredMediaTables_MigratesAcrossBatches(t *testing.T) {
	db := setupRenameTestDB(t)
	// 超过单批 100 行，验证 id 游标分页完整搬运。
	createLegacyStoredMediaTable(t, db, legacyStoredImagesTable, 250, "img")

	if err := db.AutoMigrate(&storedmediastore.StoredMedia{}); err != nil {
		t.Fatalf("AutoMigrate: %v", err)
	}
	if err := mergeLegacyStoredMediaTables(); err != nil {
		t.Fatalf("mergeLegacyStoredMediaTables error: %v", err)
	}
	if got := countStoredMediaByType(t, db, storedmediastore.MediaTypeImage); got != 250 {
		t.Fatalf("image rows = %d, want 250", got)
	}
}
