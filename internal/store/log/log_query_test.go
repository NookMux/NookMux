package logstore_test

import (
	"fmt"
	"github.com/NookMux/NookMux/internal/common"
	"github.com/NookMux/NookMux/internal/infra/redis"
	"github.com/NookMux/NookMux/internal/store/db"
	"github.com/NookMux/NookMux/internal/store/log"
	"github.com/NookMux/NookMux/internal/store/user"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"testing"
)

// setupLogQueryTestDB prepares an in-memory sqlite DB with the tables needed by
// the log list queries under test.
func setupLogQueryTestDB(t *testing.T) {
	t.Helper()

	oldDB := dbstore.DB
	oldLogDB := dbstore.LOG_DB
	oldRedisEnabled := redis.RedisEnabled
	oldMemoryCacheEnabled := common.MemoryCacheEnabled

	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite test db: %v", err)
	}
	if err := db.AutoMigrate(&userstore.User{}, &logstore.Log{}); err != nil {
		t.Fatalf("migrate sqlite test db: %v", err)
	}
	dbstore.DB = db
	dbstore.LOG_DB = db
	redis.RedisEnabled = false
	common.MemoryCacheEnabled = false

	t.Cleanup(func() {
		if sqlDB, err := db.DB(); err == nil {
			_ = sqlDB.Close()
		}
		dbstore.DB = oldDB
		dbstore.LOG_DB = oldLogDB
		redis.RedisEnabled = oldRedisEnabled
		common.MemoryCacheEnabled = oldMemoryCacheEnabled
	})
}

// TestLogQueryOrderByCreatedAtDesc verifies the behavioral contract that log
// list queries return rows newest-first (by created_at, then a stable
// tiebreaker), regardless of the order rows were inserted.
//
// The dataset deliberately makes auto-increment id order NOT match created_at
// order, and includes two rows sharing the same created_at to exercise the
// tiebreaker. A query that orders by "id desc" alone would produce a different
// sequence and fail.
//
// Assertions observe only the returned ordering (via created_at and a
// per-row marker field). They do NOT inspect SQL text, index names, or gorm
// internals, so the test stays valid if the underlying mechanism changes.
//
// Note on the marker choice: the user-facing GetUserLogs path rewrites Log.Id
// to a display sequence number (FormatUserLogs), so Id cannot be used to
// identify rows on that path. Quota is a plain data column that survives
// formatting, so it is used as the stable row identity across both paths.
func TestLogQueryOrderByCreatedAtDesc(t *testing.T) {
	setupLogQueryTestDB(t)

	// Rows are inserted so ids increase in insertion order, but created_at is
	// non-monotonic. Two rows (quota=31, quota=32) share created_at=base+50.
	const base = int64(1_000_000)
	rows := []*logstore.Log{
		{UserId: 1, CreatedAt: base + 30, Type: logstore.LogTypeConsume, ModelName: "m", ChannelId: 1, Group: "default", Quota: 11},
		{UserId: 1, CreatedAt: base + 10, Type: logstore.LogTypeConsume, ModelName: "m", ChannelId: 1, Group: "default", Quota: 12},
		{UserId: 1, CreatedAt: base + 50, Type: logstore.LogTypeConsume, ModelName: "m", ChannelId: 1, Group: "default", Quota: 31},
		{UserId: 1, CreatedAt: base + 50, Type: logstore.LogTypeConsume, ModelName: "m", ChannelId: 1, Group: "default", Quota: 32},
		{UserId: 1, CreatedAt: base + 20, Type: logstore.LogTypeConsume, ModelName: "m", ChannelId: 1, Group: "default", Quota: 13},
	}
	if err := dbstore.LOG_DB.Create(&rows).Error; err != nil {
		t.Fatalf("create logs: %v", err)
	}

	// Expected order under (created_at DESC, id DESC):
	//   id4/q32 (base+50), id3/q31 (base+50), id1/q11 (base+30), id5/q13 (base+20), id2/q12 (base+10)
	wantCreatedAt := []int64{base + 50, base + 50, base + 30, base + 20, base + 10}
	wantQuota := []int{32, 31, 11, 13, 12}

	t.Run("GetAllLogs returns newest first with stable tiebreaker", func(t *testing.T) {
		// GetAllLogs(logType, startTs, endTs, modelName, username, tokenName, startIdx, num, channel, group, requestId, upstreamRequestId, ip, ua, xTitle, httpReferer)
		logs, total, err := logstore.GetAllLogs(logstore.LogTypeUnknown, 0, 0, "", "", "", 0, 100, 0, "", "", "", "", "", "", "")
		if err != nil {
			t.Fatalf("GetAllLogs error = %v", err)
		}
		if total != int64(len(rows)) {
			t.Fatalf("total = %d, want %d", total, len(rows))
		}
		assertOrder(t, logs, wantCreatedAt, wantQuota)
	})

	t.Run("GetUserLogs returns newest first with stable tiebreaker", func(t *testing.T) {
		// GetUserLogs(userId, logType, startTs, endTs, modelName, tokenName, startIdx, num, group, requestId, upstreamRequestId, ip, ua, xTitle, httpReferer)
		logs, total, err := logstore.GetUserLogs(1, logstore.LogTypeUnknown, 0, 0, "", "", 0, 100, "", "", "", "", "", "", "")
		if err != nil {
			t.Fatalf("GetUserLogs error = %v", err)
		}
		if total != int64(len(rows)) {
			t.Fatalf("total = %d, want %d", total, len(rows))
		}
		assertOrder(t, logs, wantCreatedAt, wantQuota)
	})

	t.Run("GetUserLogs respects created_at range filter", func(t *testing.T) {
		// Range [base+15, base+45] excludes the two base+50 rows and the base+10
		// row, leaving q11 (base+30) and q13 (base+20), newest first.
		logs, total, err := logstore.GetUserLogs(1, logstore.LogTypeUnknown, base+15, base+45, "", "", 0, 100, "", "", "", "", "", "", "")
		if err != nil {
			t.Fatalf("GetUserLogs error = %v", err)
		}
		if total != 2 {
			t.Fatalf("total = %d, want 2", total)
		}
		assertOrder(t, logs, []int64{base + 30, base + 20}, []int{11, 13})
	})

	t.Run("GetUserLogs pagination preserves order", func(t *testing.T) {
		// Page 1 (offset 0, size 2) -> two newest (both base+50, quota desc).
		page1, _, err := logstore.GetUserLogs(1, logstore.LogTypeUnknown, 0, 0, "", "", 0, 2, "", "", "", "", "", "", "")
		if err != nil {
			t.Fatalf("GetUserLogs page1 error = %v", err)
		}
		assertOrder(t, page1, wantCreatedAt[:2], wantQuota[:2])

		// Page 2 (offset 2, size 2) -> next two (base+30, base+20).
		page2, _, err := logstore.GetUserLogs(1, logstore.LogTypeUnknown, 0, 0, "", "", 2, 2, "", "", "", "", "", "", "")
		if err != nil {
			t.Fatalf("GetUserLogs page2 error = %v", err)
		}
		assertOrder(t, page2, wantCreatedAt[2:4], wantQuota[2:4])
	})
}

// assertOrder checks that logs arrive in the expected created_at order, using
// Quota as a stable per-row marker (see TestLogQueryOrderByCreatedAtDesc docs
// for why Id is not used).
func assertOrder(t *testing.T, logs []*logstore.Log, wantCreatedAt []int64, wantQuota []int) {
	t.Helper()
	if len(logs) != len(wantCreatedAt) {
		t.Fatalf("got %d logs, want %d (created_at=%v quota=%v)", len(logs), len(wantCreatedAt), collectCreatedAt(logs), collectQuota(logs))
	}
	for i, l := range logs {
		if l.CreatedAt != wantCreatedAt[i] {
			t.Fatalf("row %d: created_at = %d, want %d (full order: created_at=%v)", i, l.CreatedAt, wantCreatedAt[i], collectCreatedAt(logs))
		}
		if l.Quota != wantQuota[i] {
			t.Fatalf("row %d (created_at=%d): quota = %d, want %d (tiebreaker failed; full quota=%v)", i, l.CreatedAt, l.Quota, wantQuota[i], collectQuota(logs))
		}
	}
}

func collectCreatedAt(logs []*logstore.Log) []int64 {
	out := make([]int64, len(logs))
	for i, l := range logs {
		out[i] = l.CreatedAt
	}
	return out
}

func collectQuota(logs []*logstore.Log) []int {
	out := make([]int, len(logs))
	for i, l := range logs {
		out[i] = l.Quota
	}
	return out
}

func TestGetAllLogsReturnsPersistedChannelNameSnapshot(t *testing.T) {
	setupLogQueryTestDB(t)

	rows := []*logstore.Log{
		{UserId: 1, CreatedAt: 1000, Type: logstore.LogTypeConsume, ChannelId: 7, ChannelName: "渠道甲"},
		{UserId: 1, CreatedAt: 2000, Type: logstore.LogTypeConsume, ChannelId: 9},
	}
	for _, r := range rows {
		if err := dbstore.LOG_DB.Create(r).Error; err != nil {
			t.Fatalf("create log: %v", err)
		}
	}

	// channel_name 快照随日志持久化，是唯一数据来源；测试库刻意不建
	// channels 表且行携带非零 channel_id，约束实现不得联查渠道表。
	logs, _, err := logstore.GetAllLogs(logstore.LogTypeUnknown, 0, 0, "", "", "", 0, 100, 0, "", "", "", "", "", "", "")
	if err != nil {
		t.Fatalf("GetAllLogs error = %v", err)
	}
	if len(logs) != 2 {
		t.Fatalf("got %d logs, want 2", len(logs))
	}
	// newest first：created_at=2000 的行无快照，created_at=1000 的行带快照。
	if logs[0].ChannelName != "" {
		t.Fatalf("logs[0].channel_name = %q, want empty", logs[0].ChannelName)
	}
	if logs[1].ChannelName != "渠道甲" {
		t.Fatalf("logs[1].channel_name = %q, want %q", logs[1].ChannelName, "渠道甲")
	}
}
