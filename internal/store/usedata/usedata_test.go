package usedatastore_test

import (
	"fmt"
	"github.com/NookMux/NookMux/internal/store/db"
	"github.com/NookMux/NookMux/internal/store/log"
	"github.com/NookMux/NookMux/internal/store/usedata"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"testing"
)

// setupQuotaDataTestDB 初始化内存 SQLite 数据库并迁移 QuotaData 表。
// 恢复全局 DB/缓存和写入层配置，确保各用例之间互不干扰。
func setupQuotaDataTestDB(t *testing.T) {
	t.Helper()

	oldDB := dbstore.DB
	oldLogDB := dbstore.LOG_DB
	oldCache := usedatastore.CacheQuotaData
	oldTokens, oldByModel, oldByUser := usedatastore.GetQuotaDataTrackingConfig()

	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite test db: %v", err)
	}
	if err := db.AutoMigrate(&usedatastore.QuotaData{}, &logstore.Log{}); err != nil {
		t.Fatalf("migrate sqlite test db: %v", err)
	}
	dbstore.DB = db
	dbstore.LOG_DB = db
	usedatastore.CacheQuotaData = make(map[string]*usedatastore.QuotaData)

	t.Cleanup(func() {
		if sqlDB, err := db.DB(); err == nil {
			_ = sqlDB.Close()
		}
		dbstore.DB = oldDB
		dbstore.LOG_DB = oldLogDB
		usedatastore.CacheQuotaData = oldCache
		usedatastore.SyncQuotaDataTrackingConfig(oldTokens, oldByModel, oldByUser)
	})
}

// setTrackingConfig 设置写入层聚合粒度配置。
func setTrackingConfig(t *testing.T, tokens, byModel, byUser bool) {
	t.Helper()
	usedatastore.SyncQuotaDataTrackingConfig(tokens, byModel, byUser)
}

// snapshotCache 返回缓存副本，便于断言。
func snapshotCache(t *testing.T) map[string]*usedatastore.QuotaData {
	t.Helper()
	usedatastore.CacheQuotaDataLock.Lock()
	defer usedatastore.CacheQuotaDataLock.Unlock()
	out := make(map[string]*usedatastore.QuotaData, len(usedatastore.CacheQuotaData))
	for k, v := range usedatastore.CacheQuotaData {
		out[k] = v
	}
	return out
}

// TestGetRankingQuotaBucketsDayOffsetAlignsToTimezone 验证天级分桶在带
// dayOffset 时按目标时区的自然日 00:00 切分，而非 UTC 0 点（修复非 UTC
// 启动时区下"每日"数据错位问题）。
func TestGetRankingQuotaBucketsDayOffsetAlignsToTimezone(t *testing.T) {
	setupQuotaDataTestDB(t)

	offset := int64(8 * 3600) // UTC+8
	// UTC+8 的 2026-08-15 自然日起点 = Unix 1786723200（UTC 8-14 16:00）。
	dayStart := int64(1786723200)
	// 两条数据分别是本地 00:30 和本地 10:30，同属 UTC+8 的 8-15 当天，
	// 但分属 UTC 的 8-14 和 8-15 两天。
	rows := []usedatastore.QuotaData{
		{UserID: 1, Username: "u1", ModelName: "m1", CreatedAt: dayStart + 30*60, TokenUsed: 100},
		{UserID: 1, Username: "u1", ModelName: "m1", CreatedAt: dayStart + 10*3600 + 30*60, TokenUsed: 50},
	}
	for i := range rows {
		if err := dbstore.DB.Create(&rows[i]).Error; err != nil {
			t.Fatalf("seed quota_data: %v", err)
		}
	}

	buckets, err := usedatastore.GetRankingQuotaBuckets(0, dayStart+24*3600, 24*3600, offset)
	if err != nil {
		t.Fatalf("GetRankingQuotaBuckets: %v", err)
	}
	if len(buckets) != 1 {
		t.Fatalf("expected 1 day bucket aligned to UTC+8 midnight, got %d: %+v", len(buckets), buckets)
	}
	if buckets[0].Bucket != dayStart {
		t.Fatalf("bucket = %d, want %d (UTC+8 midnight)", buckets[0].Bucket, dayStart)
	}
	if buckets[0].Tokens != 150 {
		t.Fatalf("tokens = %d, want 150", buckets[0].Tokens)
	}

	// 无偏移（UTC 切分）时同样的数据应产生 2 个桶，证明偏移确实改变了边界。
	utcBuckets, err := usedatastore.GetRankingQuotaBuckets(0, dayStart+24*3600, 24*3600, 0)
	if err != nil {
		t.Fatalf("GetRankingQuotaBuckets (utc): %v", err)
	}
	if len(utcBuckets) != 2 {
		t.Fatalf("expected 2 UTC day buckets, got %d: %+v", len(utcBuckets), utcBuckets)
	}
}

// TestGetRankingQuotaBucketsHourlyIgnoresOffset 验证小时级分桶不受 dayOffset
// 影响（小时边界与时区无关）。
func TestGetRankingQuotaBucketsHourlyIgnoresOffset(t *testing.T) {
	setupQuotaDataTestDB(t)

	base := int64(1786752000)
	rows := []usedatastore.QuotaData{
		{UserID: 1, Username: "u1", ModelName: "m1", CreatedAt: base + 60, TokenUsed: 10},
		{UserID: 1, Username: "u1", ModelName: "m1", CreatedAt: base + 3600 + 60, TokenUsed: 20},
	}
	for i := range rows {
		if err := dbstore.DB.Create(&rows[i]).Error; err != nil {
			t.Fatalf("seed quota_data: %v", err)
		}
	}

	for _, offset := range []int64{0, 8 * 3600} {
		buckets, err := usedatastore.GetRankingQuotaBuckets(base, base+2*3600, 3600, offset)
		if err != nil {
			t.Fatalf("GetRankingQuotaBuckets(offset=%d): %v", offset, err)
		}
		if len(buckets) != 2 {
			t.Fatalf("offset=%d: expected 2 hourly buckets, got %d", offset, len(buckets))
		}
	}
}

// TestLogQuotaDataTrackTokensDisabled 验证禁用 track_tokens 时 token 不累计。
func TestLogQuotaDataTrackTokensDisabled(t *testing.T) {
	setupQuotaDataTestDB(t)
	setTrackingConfig(t, false, true, true)

	usedatastore.LogQuotaData(1, "alice", "gpt-4", "", 100, 1000, 500)

	snap := snapshotCache(t)
	if len(snap) != 1 {
		t.Fatalf("cache size = %d, want 1", len(snap))
	}
	for _, d := range snap {
		if d.TokenUsed != 0 {
			t.Fatalf("TokenUsed = %d, want 0 when track_tokens disabled", d.TokenUsed)
		}
		if d.Count != 1 {
			t.Fatalf("Count = %d, want 1", d.Count)
		}
		if d.Quota != 100 {
			t.Fatalf("Quota = %d, want 100", d.Quota)
		}
	}
}

// TestLogQuotaDataTrackTokensEnabled 验证启用 track_tokens 时 token 正常累计。
func TestLogQuotaDataTrackTokensEnabled(t *testing.T) {
	setupQuotaDataTestDB(t)
	setTrackingConfig(t, true, true, true)

	usedatastore.LogQuotaData(1, "alice", "gpt-4", "", 100, 1000, 500)

	snap := snapshotCache(t)
	for _, d := range snap {
		if d.TokenUsed != 500 {
			t.Fatalf("TokenUsed = %d, want 500", d.TokenUsed)
		}
	}
}

// TestLogQuotaDataTrackByUserDisabled 验证禁用 by_user 时不同用户聚合到同一匿名桶（userId=0）。
func TestLogQuotaDataTrackByUserDisabled(t *testing.T) {
	setupQuotaDataTestDB(t)
	setTrackingConfig(t, true, true, false)

	usedatastore.LogQuotaData(1, "alice", "gpt-4", "", 100, 1000, 10)
	usedatastore.LogQuotaData(2, "bob", "gpt-4", "", 200, 1000, 20)

	snap := snapshotCache(t)
	if len(snap) != 1 {
		t.Fatalf("cache size = %d, want 1 (all users collapse to anonymous bucket)", len(snap))
	}
	for _, d := range snap {
		if d.UserID != 0 {
			t.Fatalf("UserID = %d, want 0 (anonymous bucket)", d.UserID)
		}
		if d.Username != "" {
			t.Fatalf("Username = %q, want empty", d.Username)
		}
		if d.Count != 2 {
			t.Fatalf("Count = %d, want 2 (merged from two users)", d.Count)
		}
		if d.TokenUsed != 30 {
			t.Fatalf("TokenUsed = %d, want 30", d.TokenUsed)
		}
	}
}

// TestLogQuotaDataTrackByModelDisabled 验证禁用 by_model 时不同模型聚合到同一全局模型桶（modelName=""）。
func TestLogQuotaDataTrackByModelDisabled(t *testing.T) {
	setupQuotaDataTestDB(t)
	setTrackingConfig(t, true, false, true)

	usedatastore.LogQuotaData(1, "alice", "gpt-4", "", 100, 1000, 10)
	usedatastore.LogQuotaData(1, "alice", "claude-3", "", 200, 1000, 20)

	snap := snapshotCache(t)
	if len(snap) != 1 {
		t.Fatalf("cache size = %d, want 1 (all models collapse to global bucket)", len(snap))
	}
	for _, d := range snap {
		if d.ModelName != "" {
			t.Fatalf("ModelName = %q, want empty (global bucket)", d.ModelName)
		}
		if d.Count != 2 {
			t.Fatalf("Count = %d, want 2 (merged from two models)", d.Count)
		}
	}
}

// TestLogQuotaErrorDataTrackByUserDisabled 验证失败日志也按配置聚合维度。
func TestLogQuotaErrorDataTrackByUserDisabled(t *testing.T) {
	setupQuotaDataTestDB(t)
	setTrackingConfig(t, true, true, false)

	usedatastore.LogQuotaErrorData(1, "alice", "gpt-4", "", 1000)
	usedatastore.LogQuotaErrorData(2, "bob", "gpt-4", "", 1000)

	snap := snapshotCache(t)
	if len(snap) != 1 {
		t.Fatalf("cache size = %d, want 1 (error data collapsed by user)", len(snap))
	}
	for _, d := range snap {
		if d.UserID != 0 {
			t.Fatalf("UserID = %d, want 0", d.UserID)
		}
		if d.FailCount != 2 {
			t.Fatalf("FailCount = %d, want 2", d.FailCount)
		}
	}
}

// TestLogQuotaDataSeparatesPlatforms 验证同用户/模型/小时的不同平台各自成桶，
// 落盘时 SaveQuotaDataCache 的 WHERE 含 platform，不会把数据累加进未识别桶。
// createdAt 必须用非零整点时间戳：GORM 对零值 CreatedAt 会自动填充当前时间，
// 导致 upsert 无法命中（生产路径 createdAt 恒非零）。
func TestLogQuotaDataSeparatesPlatforms(t *testing.T) {
	setupQuotaDataTestDB(t)
	setTrackingConfig(t, true, true, true)

	hourStart := int64(1793160000)

	usedatastore.LogQuotaData(1, "alice", "gpt-4", "", 100, hourStart+100, 10)
	usedatastore.LogQuotaData(1, "alice", "gpt-4", "claude-code", 200, hourStart+200, 20)
	usedatastore.LogQuotaData(1, "alice", "gpt-4", "claude-code", 300, hourStart+300, 30)

	snap := snapshotCache(t)
	if len(snap) != 2 {
		t.Fatalf("cache size = %d, want 2 (unidentified bucket + claude-code bucket)", len(snap))
	}
	for _, d := range snap {
		switch d.Platform {
		case "":
			if d.Count != 1 || d.Quota != 100 || d.TokenUsed != 10 {
				t.Fatalf("unidentified bucket = %+v", d)
			}
		case "claude-code":
			if d.Count != 2 || d.Quota != 500 || d.TokenUsed != 50 {
				t.Fatalf("claude-code bucket = %+v", d)
			}
		default:
			t.Fatalf("unexpected platform bucket %+v", d)
		}
	}

	usedatastore.SaveQuotaDataCache()

	var rows []usedatastore.QuotaData
	if err := dbstore.DB.Table("quota_data").Order("platform asc").Find(&rows).Error; err != nil {
		t.Fatalf("query quota_data: %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("rows = %d, want 2", len(rows))
	}
	if rows[0].Platform != "" || rows[0].Quota != 100 {
		t.Fatalf("row0 = %+v, want unidentified bucket with quota 100", rows[0])
	}
	if rows[1].Platform != "claude-code" || rows[1].Quota != 500 {
		t.Fatalf("row1 = %+v, want claude-code bucket with quota 500", rows[1])
	}

	// 落盘后再写入未识别桶，增量应命中 platform='' 的既有行而不是 claude-code 行。
	usedatastore.LogQuotaData(1, "alice", "gpt-4", "", 50, hourStart+400, 5)
	usedatastore.SaveQuotaDataCache()
	var unidentified usedatastore.QuotaData
	if err := dbstore.DB.Table("quota_data").Where("platform = ''").First(&unidentified).Error; err != nil {
		t.Fatalf("query unidentified row: %v", err)
	}
	if unidentified.Quota != 150 {
		t.Fatalf("unidentified quota = %d, want 150", unidentified.Quota)
	}
	var claudeCode usedatastore.QuotaData
	if err := dbstore.DB.Table("quota_data").Where("platform = ?", "claude-code").First(&claudeCode).Error; err != nil {
		t.Fatalf("query claude-code row: %v", err)
	}
	if claudeCode.Quota != 500 {
		t.Fatalf("claude-code quota = %d, want 500 (increment must not leak across platforms)", claudeCode.Quota)
	}
}

// TestRecalculateQuotaDataRespectsTrackingConfig 验证重算流程按配置聚合维度和 token 记录。
func TestRecalculateQuotaDataRespectsTrackingConfig(t *testing.T) {
	setupQuotaDataTestDB(t)
	// 全部禁用：所有记录聚合为单条，token 为 0
	setTrackingConfig(t, false, false, false)

	// 构造两条成功日志到 logs 表（type=2），重算入口读取 LOG_DB
	logs := []*logstore.Log{
		{UserId: 1, Username: "alice", ModelName: "gpt-4", CreatedAt: 1000, PromptTokens: 10, CompletionTokens: 20, Quota: 100, Type: logstore.LogTypeConsume},
		{UserId: 2, Username: "bob", ModelName: "claude-3", CreatedAt: 1000, PromptTokens: 30, CompletionTokens: 40, Quota: 200, Type: logstore.LogTypeConsume},
	}
	if err := dbstore.LOG_DB.Create(&logs).Error; err != nil {
		t.Fatalf("create logs: %v", err)
	}

	if err := usedatastore.RecalculateQuotaData(0, 2000); err != nil {
		t.Fatalf("RecalculateQuotaData error: %v", err)
	}

	var rows []*usedatastore.QuotaData
	if err := dbstore.DB.Table("quota_data").Find(&rows).Error; err != nil {
		t.Fatalf("query quota_data: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("quota_data rows = %d, want 1 (all dimensions collapsed)", len(rows))
	}
	r := rows[0]
	if r.UserID != 0 || r.Username != "" || r.ModelName != "" {
		t.Fatalf("dimensions = (user=%d, name=%q, model=%q), want anonymous/global bucket", r.UserID, r.Username, r.ModelName)
	}
	if r.TokenUsed != 0 {
		t.Fatalf("TokenUsed = %d, want 0 when track_tokens disabled", r.TokenUsed)
	}
	if r.Count != 2 {
		t.Fatalf("Count = %d, want 2", r.Count)
	}
	if r.Quota != 300 {
		t.Fatalf("Quota = %d, want 300", r.Quota)
	}
}
