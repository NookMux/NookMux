package storedmediastore

import (
	"context"
	"fmt"
	"testing"

	"github.com/NookMux/NookMux/internal/store/db"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func setupStoredMediaTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	oldDB := dbstore.DB
	testDB, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite test db: %v", err)
	}
	if err := testDB.AutoMigrate(&StoredMedia{}); err != nil {
		t.Fatalf("migrate sqlite test db: %v", err)
	}

	dbstore.DB = testDB
	t.Cleanup(func() {
		if sqlDB, err := testDB.DB(); err == nil {
			_ = sqlDB.Close()
		}
		dbstore.DB = oldDB
	})
	return testDB
}

func mustInsertMedia(t *testing.T, m *StoredMedia) {
	t.Helper()
	if err := m.Insert(context.Background()); err != nil {
		t.Fatalf("insert stored media: %v", err)
	}
}

func TestStoredMediaInsertFillsDefaults(t *testing.T) {
	setupStoredMediaTestDB(t)

	m := &StoredMedia{UserId: 1, MediaType: MediaTypeImage, MimeType: "image/png", SizeBytes: 3, Sha256: "abc", Data: LargeBlob("png")}
	mustInsertMedia(t, m)
	if m.Id == "" || m.CreatedAt == 0 {
		t.Fatalf("insert should fill id and created_at, got id=%q created_at=%d", m.Id, m.CreatedAt)
	}

	loaded, err := GetStoredMediaByID(context.Background(), m.Id)
	if err != nil {
		t.Fatalf("load by id: %v", err)
	}
	if loaded.MediaType != MediaTypeImage || string(loaded.Data) != "png" {
		t.Fatalf("loaded row mismatch: %+v", loaded)
	}

	meta, err := GetStoredMediaMetaByID(context.Background(), m.Id)
	if err != nil {
		t.Fatalf("load meta by id: %v", err)
	}
	if len(meta.Data) != 0 {
		t.Fatalf("meta query must not load the binary payload, got %d bytes", len(meta.Data))
	}
}

func TestStoredMediaInsertRejectsUnknownType(t *testing.T) {
	setupStoredMediaTestDB(t)

	m := &StoredMedia{UserId: 1, MediaType: "audio", Sha256: "abc", Data: LargeBlob("x")}
	if err := m.Insert(context.Background()); err == nil {
		t.Fatalf("insert with unknown media type must fail")
	}
}

func TestStoredMediaByUserAndShaDeduplicatesPerType(t *testing.T) {
	setupStoredMediaTestDB(t)

	first := &StoredMedia{UserId: 1, MediaType: MediaTypeImage, CreatedAt: 100, MimeType: "image/png", Sha256: "same-sha", Data: LargeBlob("img")}
	mustInsertMedia(t, first)
	video := &StoredMedia{UserId: 1, MediaType: MediaTypeVideo, CreatedAt: 200, MimeType: "video/mp4", Sha256: "same-sha", Data: LargeBlob("vid")}
	mustInsertMedia(t, video)
	otherUser := &StoredMedia{UserId: 2, MediaType: MediaTypeImage, CreatedAt: 300, Sha256: "same-sha", Data: LargeBlob("img2")}
	mustInsertMedia(t, otherUser)

	img, err := GetStoredMediaByUserAndSha(context.Background(), 1, MediaTypeImage, "same-sha")
	if err != nil {
		t.Fatalf("dedupe image: %v", err)
	}
	if img.Id != first.Id {
		t.Fatalf("dedupe should return the earliest same-type row, got %s want %s", img.Id, first.Id)
	}
	vid, err := GetStoredMediaByUserAndSha(context.Background(), 1, MediaTypeVideo, "same-sha")
	if err != nil {
		t.Fatalf("dedupe video: %v", err)
	}
	if vid.Id != video.Id {
		t.Fatalf("same sha of another type must not be reused across types, got %s want %s", vid.Id, video.Id)
	}
}

func TestDeleteStoredMediaInRangeFiltersByType(t *testing.T) {
	db := setupStoredMediaTestDB(t)

	oldImg := &StoredMedia{UserId: 1, MediaType: MediaTypeImage, CreatedAt: 100, Sha256: "a", Data: LargeBlob("x")}
	mustInsertMedia(t, oldImg)
	oldVid := &StoredMedia{UserId: 1, MediaType: MediaTypeVideo, CreatedAt: 100, Sha256: "b", Data: LargeBlob("y")}
	mustInsertMedia(t, oldVid)
	newImg := &StoredMedia{UserId: 1, MediaType: MediaTypeImage, CreatedAt: 999, Sha256: "c", Data: LargeBlob("z")}
	mustInsertMedia(t, newImg)

	deleted, err := DeleteStoredMediaInRange(context.Background(), MediaTypeImage, 0, 200, 10)
	if err != nil {
		t.Fatalf("delete in range: %v", err)
	}
	if deleted != 1 {
		t.Fatalf("deleted = %d, want 1 (image only)", deleted)
	}

	var imgCnt, vidCnt int64
	db.Model(&StoredMedia{}).Where("media_type = ?", MediaTypeImage).Count(&imgCnt)
	db.Model(&StoredMedia{}).Where("media_type = ?", MediaTypeVideo).Count(&vidCnt)
	if imgCnt != 1 || vidCnt != 1 {
		t.Fatalf("remaining rows: image=%d video=%d, want image=1 video=1", imgCnt, vidCnt)
	}
}

func TestEnsureStoredMediaPoolLimitEvictsOldestPerType(t *testing.T) {
	setupStoredMediaTestDB(t)

	// 图片池 10 字节：老图 6B + 新图 6B 超限，删最老的图；视频不受影响。
	oldImg := &StoredMedia{UserId: 1, MediaType: MediaTypeImage, CreatedAt: 100, SizeBytes: 6, Sha256: "a", Data: LargeBlob("123456")}
	mustInsertMedia(t, oldImg)
	newImg := &StoredMedia{UserId: 1, MediaType: MediaTypeImage, CreatedAt: 200, SizeBytes: 6, Sha256: "b", Data: LargeBlob("123456")}
	mustInsertMedia(t, newImg)
	bigVid := &StoredMedia{UserId: 1, MediaType: MediaTypeVideo, CreatedAt: 100, SizeBytes: 100, Sha256: "c", Data: LargeBlob("v")}
	mustInsertMedia(t, bigVid)

	deleted, err := EnsureStoredMediaPoolLimit(context.Background(), MediaTypeImage, 10, 1)
	if err != nil {
		t.Fatalf("ensure pool limit: %v", err)
	}
	if deleted != 1 {
		t.Fatalf("deleted = %d, want 1", deleted)
	}
	if _, err := GetStoredMediaByID(context.Background(), oldImg.Id); err == nil {
		t.Fatalf("oldest image should be evicted")
	}
	if _, err := GetStoredMediaByID(context.Background(), bigVid.Id); err != nil {
		t.Fatalf("video must not be touched by image pool limit")
	}
}

func TestMediaConvertStatsSplitsByType(t *testing.T) {
	setupStoredMediaTestDB(t)

	mustInsertMedia(t, &StoredMedia{UserId: 1, MediaType: MediaTypeImage, CreatedAt: 100, Sha256: "a", Data: LargeBlob("x")})
	mustInsertMedia(t, &StoredMedia{UserId: 1, MediaType: MediaTypeImage, CreatedAt: 200, Sha256: "b", Data: LargeBlob("x")})
	mustInsertMedia(t, &StoredMedia{UserId: 1, MediaType: MediaTypeVideo, CreatedAt: 300, Sha256: "c", Data: LargeBlob("x")})
	mustInsertMedia(t, &StoredMedia{UserId: 2, MediaType: MediaTypeImage, CreatedAt: 400, Sha256: "d", Data: LargeBlob("x")})

	stats, err := GetMediaConvertStatsByUserId(1, 0, 500)
	if err != nil {
		t.Fatalf("stats by user: %v", err)
	}
	if stats.ImageCount != 2 || stats.VideoCount != 1 {
		t.Fatalf("user stats = %+v, want image=2 video=1", stats)
	}

	all, err := GetAllMediaConvertStats(0, 500)
	if err != nil {
		t.Fatalf("all stats: %v", err)
	}
	if all.ImageCount != 3 || all.VideoCount != 1 {
		t.Fatalf("all stats = %+v, want image=3 video=1", all)
	}
}

func TestQueryStoredMediaListsMixedTypes(t *testing.T) {
	setupStoredMediaTestDB(t)

	mustInsertMedia(t, &StoredMedia{UserId: 1, MediaType: MediaTypeImage, CreatedAt: 100, Sha256: "a", Data: LargeBlob("x")})
	mustInsertMedia(t, &StoredMedia{UserId: 1, MediaType: MediaTypeVideo, CreatedAt: 200, Sha256: "b", Data: LargeBlob("x")})
	mustInsertMedia(t, &StoredMedia{UserId: 2, MediaType: MediaTypeImage, CreatedAt: 300, Sha256: "c", Data: LargeBlob("x")})

	items, total, err := GetUserStoredMedia(context.Background(), 1, 0, 0, 0, 10)
	if err != nil {
		t.Fatalf("query user media: %v", err)
	}
	if total != 2 || len(items) != 2 {
		t.Fatalf("user items = %d/%d, want 2/2", len(items), total)
	}
	if items[0].CreatedAt < items[1].CreatedAt {
		t.Fatalf("items must be ordered by created_at desc")
	}

	all, total, err := GetAllStoredMedia(context.Background(), 0, 0, 0, 10)
	if err != nil {
		t.Fatalf("query all media: %v", err)
	}
	if total != 3 || len(all) != 3 {
		t.Fatalf("all items = %d/%d, want 3/3", len(all), total)
	}
}
