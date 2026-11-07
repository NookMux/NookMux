package dbmigrate

import (
	"github.com/NookMux/NookMux/internal/common"
	"github.com/NookMux/NookMux/internal/store/db"
	"github.com/NookMux/NookMux/internal/store/log"
	"github.com/NookMux/NookMux/internal/store/option"
	"testing"
)

func TestBackfillLogPlatformsIdentifiesHistoricalRows(t *testing.T) {
	setupBackfillTestDB(t)
	if err := dbstore.DB.AutoMigrate(&optionstore.Option{}); err != nil {
		t.Fatalf("migrate options table: %v", err)
	}

	now := common.GetTimestamp()
	rows := []logstore.Log{
		{UserId: 1, CreatedAt: now, Type: logstore.LogTypeConsume, Ua: "claude-cli/2.1.0 (external, cli)", XTitle: "-", HttpReferer: "-"},
		{UserId: 1, CreatedAt: now, Type: logstore.LogTypeConsume, Ua: "-", XTitle: "-", HttpReferer: "https://cherry-ai.com"},
		{UserId: 1, CreatedAt: now, Type: logstore.LogTypeError, Ua: "Mozilla/5.0 (Windows NT 10.0)", XTitle: "-", HttpReferer: "-"},
		{UserId: 1, CreatedAt: now, Type: logstore.LogTypeConsume, Ua: "-", XTitle: "-", HttpReferer: "-", Platform: "opencode"},
	}
	for i := range rows {
		if err := dbstore.LOG_DB.Create(&rows[i]).Error; err != nil {
			t.Fatalf("create log %d: %v", i, err)
		}
	}

	if err := dbstore.DB.Where(&optionstore.Option{Key: logPlatformBackfillMarker}).Delete(&optionstore.Option{}).Error; err != nil {
		t.Fatalf("clear marker before backfill: %v", err)
	}
	backfillLogPlatforms()

	assertPlatform := func(id int, want string) {
		t.Helper()
		// 每次断言使用全新结构体：复用已带主键的目标会让 First 叠加主键条件。
		var row logstore.Log
		if err := dbstore.LOG_DB.First(&row, id).Error; err != nil {
			t.Fatalf("query log %d: %v", id, err)
		}
		if row.Platform != want {
			t.Fatalf("log %d platform = %q, want %q", id, row.Platform, want)
		}
	}
	assertPlatform(rows[0].Id, "claude-code")
	assertPlatform(rows[1].Id, "cherry-studio")
	// 无法识别的行保持空值，已有值的行不被覆盖。
	assertPlatform(rows[2].Id, "")
	assertPlatform(rows[3].Id, "opencode")

	var count int64
	if err := dbstore.DB.Model(&optionstore.Option{}).Where(&optionstore.Option{Key: logPlatformBackfillMarker}).Count(&count).Error; err != nil {
		t.Fatalf("count marker after backfill: %v", err)
	}
	if count != 1 {
		t.Fatalf("marker count after successful backfill = %d, want 1", count)
	}

	// marker 已落地时重跑应为 no-op：识别结果不变。
	backfillLogPlatforms()
	assertPlatform(rows[0].Id, "claude-code")
}

func TestBackfillLogPlatformsSkipsWhenMarkerSet(t *testing.T) {
	setupBackfillTestDB(t)
	if err := dbstore.DB.AutoMigrate(&optionstore.Option{}); err != nil {
		t.Fatalf("migrate options table: %v", err)
	}

	row := &logstore.Log{UserId: 1, CreatedAt: common.GetTimestamp(), Type: logstore.LogTypeConsume, Ua: "codex_cli_rs/0.1.0 (Linux)", XTitle: "-", HttpReferer: "-"}
	if err := dbstore.LOG_DB.Create(row).Error; err != nil {
		t.Fatalf("create log: %v", err)
	}

	optionstore.MarkDataMigrationDone(logPlatformBackfillMarker)
	backfillLogPlatforms()

	var stored logstore.Log
	if err := dbstore.LOG_DB.First(&stored, row.Id).Error; err != nil {
		t.Fatalf("query log: %v", err)
	}
	if stored.Platform != "" {
		t.Fatalf("platform = %q, want empty when marker already set", stored.Platform)
	}
}
