package dbmigrate

import (
	"fmt"
	"github.com/NookMux/NookMux/internal/common"
	"github.com/NookMux/NookMux/internal/store/channel"
	"github.com/NookMux/NookMux/internal/store/db"
	"github.com/NookMux/NookMux/internal/store/log"
	"github.com/NookMux/NookMux/internal/store/option"
	"github.com/NookMux/NookMux/internal/store/user"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"testing"
)

// setupChannelNameBackfillTestDB 与 setupBackfillTestDB 等价：内存库 +
// User/Log/Channel/Option 表，额外迁移 Channel 与 Option 供渠道名回填使用。
func setupChannelNameBackfillTestDB(t *testing.T) {
	t.Helper()

	oldDB := dbstore.DB
	oldLogDB := dbstore.LOG_DB

	dbHandle, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite test db: %v", err)
	}
	if err := dbHandle.AutoMigrate(&userstore.User{}, &logstore.Log{}, &channelstore.Channel{}, &optionstore.Option{}); err != nil {
		t.Fatalf("migrate sqlite test db: %v", err)
	}
	dbstore.DB = dbHandle
	dbstore.LOG_DB = dbHandle

	t.Cleanup(func() {
		if sqlDB, err := dbHandle.DB(); err == nil {
			_ = sqlDB.Close()
		}
		dbstore.DB = oldDB
		dbstore.LOG_DB = oldLogDB
	})
}

func TestBackfillLogChannelNamesWritesSnapshots(t *testing.T) {
	setupChannelNameBackfillTestDB(t)

	if err := dbstore.DB.Create(&channelstore.Channel{Id: 7, Name: "主渠道"}).Error; err != nil {
		t.Fatalf("create channel 7: %v", err)
	}
	// 渠道名为空的现存渠道：不参与回填，其日志快照保持空串。
	if err := dbstore.DB.Create(&channelstore.Channel{Id: 8, Name: ""}).Error; err != nil {
		t.Fatalf("create channel 8: %v", err)
	}

	mkLog := func(channelId int, channelName string) *logstore.Log {
		l := &logstore.Log{
			UserId:      1,
			CreatedAt:   common.GetTimestamp(),
			Type:        logstore.LogTypeConsume,
			ChannelId:   channelId,
			ChannelName: channelName,
		}
		if err := dbstore.LOG_DB.Create(l).Error; err != nil {
			t.Fatalf("create log (channel_id=%d): %v", channelId, err)
		}
		return l
	}
	blank := mkLog(7, "")           // 快照为空，待回填
	nullNamed := mkLog(7, "")       // 快照为 NULL（部分库加列后历史行为 NULL），待回填
	missing := mkLog(999, "")       // 渠道已删除，保持空串
	emptyName := mkLog(8, "")       // 渠道存在但名称为空，保持空串
	snapshotted := mkLog(7, "已有快照") // 已有快照，不得覆盖
	if err := dbstore.LOG_DB.Exec("UPDATE logs SET channel_name = NULL WHERE id = ?", nullNamed.Id).Error; err != nil {
		t.Fatalf("null out channel_name: %v", err)
	}

	if err := dbstore.DB.Where(&optionstore.Option{Key: logChannelNameBackfillMarker}).Delete(&optionstore.Option{}).Error; err != nil {
		t.Fatalf("clear marker before backfill: %v", err)
	}
	backfillLogChannelNames()

	assertSnapshot := func(id int, want string) {
		t.Helper()
		var stored logstore.Log
		if err := dbstore.LOG_DB.First(&stored, id).Error; err != nil {
			t.Fatalf("query log %d: %v", id, err)
		}
		if stored.ChannelName != want {
			t.Fatalf("log %d channel_name = %q, want %q", id, stored.ChannelName, want)
		}
	}
	assertSnapshot(blank.Id, "主渠道")
	assertSnapshot(nullNamed.Id, "主渠道")
	assertSnapshot(missing.Id, "")
	assertSnapshot(emptyName.Id, "")
	assertSnapshot(snapshotted.Id, "已有快照")

	var count int64
	if err := dbstore.DB.Model(&optionstore.Option{}).Where(&optionstore.Option{Key: logChannelNameBackfillMarker}).Count(&count).Error; err != nil {
		t.Fatalf("count marker after backfill: %v", err)
	}
	if count != 1 {
		t.Fatalf("marker count after successful backfill = %d, want 1", count)
	}
}

func TestBackfillLogChannelNamesSkipsWhenMarkerSet(t *testing.T) {
	setupChannelNameBackfillTestDB(t)

	if err := dbstore.DB.Create(&channelstore.Channel{Id: 7, Name: "主渠道"}).Error; err != nil {
		t.Fatalf("create channel 7: %v", err)
	}
	l := &logstore.Log{
		UserId:    1,
		CreatedAt: common.GetTimestamp(),
		Type:      logstore.LogTypeConsume,
		ChannelId: 7,
	}
	if err := dbstore.LOG_DB.Create(l).Error; err != nil {
		t.Fatalf("create log: %v", err)
	}

	if err := dbstore.DB.Create(&optionstore.Option{Key: logChannelNameBackfillMarker, Value: "true"}).Error; err != nil {
		t.Fatalf("seed marker: %v", err)
	}
	backfillLogChannelNames()

	var stored logstore.Log
	if err := dbstore.LOG_DB.First(&stored, l.Id).Error; err != nil {
		t.Fatalf("query log: %v", err)
	}
	if stored.ChannelName != "" {
		t.Fatalf("channel_name = %q, want empty (marker set, migration skipped)", stored.ChannelName)
	}
}
