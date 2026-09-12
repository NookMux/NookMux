package logstore_test

import (
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/NookMux/NookMux/internal/common"
	"github.com/NookMux/NookMux/internal/domain/billing"
	"github.com/NookMux/NookMux/internal/infra/redis"
	"github.com/NookMux/NookMux/internal/store/db"
	"github.com/NookMux/NookMux/internal/store/log"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

// billingDetailsFixture 是一段符合 docs/PRD/计费.md 第 4 章 schema 的 canonical
// JSON 样本。阶段 0 的存储层只透传该字符串、不负责归一化（归一化在阶段 1），
// 这里用它验证列的可空语义与精确透传。
const billingDetailsFixture = `{"schema_version":1,"tokens":{"input":{"text_input":12,"image_input":0,"audio_input":0,"video_input":0,"document_input":0},"output":{"text_output":7,"audio_output":0,"image_output":0,"reasoning_output":3,"accepted_prediction":0,"rejected_prediction":0},"cache":{"read_cache":4,"write_cache":5,"write_cache_5m":5,"write_cache_1h":0}}}`

// setupBillingDetailsTestDB 复刻 setupLogAdminInfoTestDB 的 fixture 范式：
// 独立内存 SQLite + 迁移所需表 + 全局量保存/恢复。
func setupBillingDetailsTestDB(t *testing.T) {
	t.Helper()

	oldDB := dbstore.DB
	oldLogDB := dbstore.LOG_DB
	oldRedisEnabled := redis.RedisEnabled
	oldMemoryCacheEnabled := common.MemoryCacheEnabled
	oldDataExportEnabled := common.DataExportEnabled
	oldLogConsumeEnabled := common.LogConsumeEnabled

	logDB, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite test db: %v", err)
	}
	if err := logDB.AutoMigrate(&logstore.Log{}); err != nil {
		t.Fatalf("migrate sqlite test db: %v", err)
	}
	dbstore.DB = logDB
	dbstore.LOG_DB = logDB
	dbstore.ResetBatchUpdateStores()
	redis.RedisEnabled = false
	common.MemoryCacheEnabled = false
	// RecordConsumeLog 异步落库之外还会触发 usedata 统计写入，测试统一关闭，
	// 避免依赖 quota_data 表与批量更新器时序。
	common.DataExportEnabled = false
	common.LogConsumeEnabled = true

	t.Cleanup(func() {
		if sqlDB, err := logDB.DB(); err == nil {
			_ = sqlDB.Close()
		}
		dbstore.DB = oldDB
		dbstore.LOG_DB = oldLogDB
		dbstore.ResetBatchUpdateStores()
		redis.RedisEnabled = oldRedisEnabled
		common.MemoryCacheEnabled = oldMemoryCacheEnabled
		common.DataExportEnabled = oldDataExportEnabled
		common.LogConsumeEnabled = oldLogConsumeEnabled
	})
}

// waitForConsumeLogRow 轮询等待 RecordConsumeLog 的异步落库：写入经 RelayGo
// 提交到有界 goroutine 池，无等待句柄，只能以轮询收尾。
func waitForConsumeLogRow(t *testing.T, query string, args ...interface{}) logstore.Log {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for {
		var stored logstore.Log
		err := dbstore.LOG_DB.Where(query, args...).Order("id DESC").First(&stored).Error
		if err == nil {
			return stored
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			t.Fatalf("query consume log with %q: %v", query, err)
		}
		if time.Now().After(deadline) {
			t.Fatalf("consume log row was not written asynchronously within deadline (query=%s)", query)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

// queryBillingDetailsRaw 直查列值并以 sql.NullString 区分 NULL 与空串：
// 经模型读出时 NULL 与 "" 都表现为 nil/空，无法区分两种"空"状态。
func queryBillingDetailsRaw(t *testing.T, id int) sql.NullString {
	t.Helper()
	var value sql.NullString
	if err := dbstore.LOG_DB.Raw("SELECT billing_details FROM logs WHERE id = ?", id).Row().Scan(&value); err != nil {
		t.Fatalf("query billing_details for log id %d: %v", id, err)
	}
	return value
}

func newBillingDetailsTestContext() *gin.Context {
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, "http://example.test/v1/chat/completions", nil)
	c.Set("username", "billing-details-user")
	return c
}

// TestRecordConsumeLogWritesBillingDetailsOnlyWhenProvided 验证阶段 0 的核心
// 写入契约：带归一化用量的入口精确透传 canonical JSON；无 Token 用量的入口
// （工具费、违规费等）保持 NULL，不写 "{}"/"null" 占位；兼容列与 Other 不受影响。
func TestRecordConsumeLogWritesBillingDetailsOnlyWhenProvided(t *testing.T) {
	setupBillingDetailsTestDB(t)

	logstore.RecordConsumeLog(newBillingDetailsTestContext(), 1, logstore.RecordConsumeLogParams{
		ChannelId:        1,
		PromptTokens:     16,
		CompletionTokens: 10,
		ModelName:        "gpt-test",
		TokenName:        "tk-main",
		Quota:            233,
		Content:          "耗时 0.2 秒",
		UseTimeMs:        200,
		Group:            "default",
		Other:            map[string]interface{}{"model_ratio": 1.5},
		BillingDetails:   billingDetailsFixture,
	})
	withDetails := waitForConsumeLogRow(t, "user_id = ?", 1)
	if withDetails.BillingDetails == nil || *withDetails.BillingDetails != billingDetailsFixture {
		t.Fatalf("billing_details = %v, want exact fixture JSON %q", withDetails.BillingDetails, billingDetailsFixture)
	}
	if withDetails.BillingDetailsVersion != logstore.LogBillingDetailsVersion {
		t.Fatalf("billing_details_version = %d, want %d", withDetails.BillingDetailsVersion, logstore.LogBillingDetailsVersion)
	}
	// 旧聚合列已从模型与全新 schema 中移除；quota 保持原语义。
	for _, col := range []string{"prompt_tokens", "completion_tokens"} {
		if dbstore.LOG_DB.Migrator().HasColumn(&logstore.Log{}, col) {
			t.Fatalf("legacy aggregate column %s must not exist on fresh schema", col)
		}
	}
	if withDetails.Quota != 233 {
		t.Fatalf("quota changed: quota=%d, want 233", withDetails.Quota)
	}
	if withDetails.Other != `{"model_ratio":1.5}` {
		t.Fatalf("other = %q, want unchanged snapshot", withDetails.Other)
	}

	logstore.RecordConsumeLog(newBillingDetailsTestContext(), 2, logstore.RecordConsumeLogParams{
		ChannelId: 1,
		ModelName: "gpt-test",
		Quota:     1,
		Content:   "违规罚款",
		Group:     "default",
		Other:     map[string]interface{}{"fee_type": "violation"},
	})
	withoutDetails := waitForConsumeLogRow(t, "user_id = ?", 2)
	if withoutDetails.BillingDetailsVersion != logstore.LogBillingDetailsVersion {
		t.Fatalf("billing_details_version = %d, want %d", withoutDetails.BillingDetailsVersion, logstore.LogBillingDetailsVersion)
	}
	value := queryBillingDetailsRaw(t, withoutDetails.Id)
	if value.Valid {
		t.Fatalf("billing_details = %q for entry without token usage, want NULL", value.String)
	}
	if withoutDetails.Other != `{"fee_type":"violation"}` {
		t.Fatalf("other = %q, want unchanged snapshot", withoutDetails.Other)
	}
}

// TestBillingDetailsMigrationKeepsHistoricalRowsEmpty 覆盖 SQLite 迁移路径：
// 历史库（无该列、但有旧聚合列）启动时 AutoMigrate 补列且幂等重跑通过；
// 历史行保持 NULL、不回填、旧聚合列与 Other 原样；迁移后新行可正常写入。
func TestBillingDetailsMigrationKeepsHistoricalRowsEmpty(t *testing.T) {
	setupBillingDetailsTestDB(t)

	// 以当前模型建库后补上旧聚合列并插入历史消费日志，再删除 billing_details
	// 列，等价还原新列上线前的历史 schema。
	for _, col := range []string{"prompt_tokens", "completion_tokens"} {
		if err := dbstore.LOG_DB.Exec("ALTER TABLE logs ADD COLUMN " + col + " BIGINT DEFAULT 0").Error; err != nil {
			t.Fatalf("simulate legacy schema by adding %s: %v", col, err)
		}
	}
	historical := map[string]interface{}{
		"user_id":           1,
		"created_at":        1700000000,
		"type":              logstore.LogTypeConsume,
		"username":          "legacy-user",
		"token_name":        "legacy-token",
		"model_name":        "gpt-legacy",
		"quota":             42,
		"prompt_tokens":     100,
		"completion_tokens": 50,
		"group":             "default",
		"other":             `{"cache_read":100}`,
	}
	if err := dbstore.LOG_DB.Table("logs").Create(historical).Error; err != nil {
		t.Fatalf("seed historical log: %v", err)
	}
	if err := dbstore.LOG_DB.Migrator().DropColumn(&logstore.Log{}, "billing_details"); err != nil {
		t.Fatalf("simulate historical schema by dropping column: %v", err)
	}
	if dbstore.LOG_DB.Migrator().HasColumn(&logstore.Log{}, "billing_details") {
		t.Fatal("precondition failed: billing_details should be absent after drop")
	}

	// 启动迁移补列，并重复执行一次验证幂等（重复启动迁移）。
	for run := 1; run <= 2; run++ {
		if err := dbstore.LOG_DB.AutoMigrate(&logstore.Log{}); err != nil {
			t.Fatalf("automigrate run %d: %v", run, err)
		}
	}
	if !dbstore.LOG_DB.Migrator().HasColumn(&logstore.Log{}, "billing_details") {
		t.Fatal("billing_details column missing after AutoMigrate")
	}

	var stored logstore.Log
	if err := dbstore.LOG_DB.First(&stored, 1).Error; err != nil {
		t.Fatalf("reload historical log: %v", err)
	}
	value := queryBillingDetailsRaw(t, stored.Id)
	if value.Valid {
		t.Fatalf("historical billing_details = %q, want NULL (no backfill)", value.String)
	}
	// 旧聚合列的数据在 AutoMigrate 补列阶段不被清理，保持原样（删除由
	// dropLegacyLogTokenAggregateColumns 在回填完成后统一执行）。
	var legacy struct {
		Quota            int
		PromptTokens     int
		CompletionTokens int
		Other            string
	}
	if err := dbstore.LOG_DB.Table("logs").
		Select("quota, prompt_tokens, completion_tokens, other").
		Where("id = ?", stored.Id).Scan(&legacy).Error; err != nil {
		t.Fatalf("reload legacy aggregates: %v", err)
	}
	if legacy.Quota != 42 || legacy.PromptTokens != 100 || legacy.CompletionTokens != 50 || legacy.Other != `{"cache_read":100}` {
		t.Fatalf("historical row mutated by migration: %+v", legacy)
	}

	// 迁移后的库可正常写入新格式，且历史行保持空列。
	logstore.RecordConsumeLog(newBillingDetailsTestContext(), 2, logstore.RecordConsumeLogParams{
		ChannelId:      1,
		ModelName:      "gpt-test",
		Quota:          10,
		Content:        "耗时 0.1 秒",
		Group:          "default",
		BillingDetails: billingDetailsFixture,
	})
	newRow := waitForConsumeLogRow(t, "user_id = ?", 2)
	if newRow.BillingDetails == nil || *newRow.BillingDetails != billingDetailsFixture {
		t.Fatalf("post-migration billing_details = %v, want fixture JSON", newRow.BillingDetails)
	}
	if newRow.BillingDetailsVersion != logstore.LogBillingDetailsVersion {
		t.Fatalf("post-migration billing_details_version = %d, want %d", newRow.BillingDetailsVersion, logstore.LogBillingDetailsVersion)
	}
	if value := queryBillingDetailsRaw(t, stored.Id); value.Valid {
		t.Fatalf("historical row was backfilled: %q", value.String)
	}
}

// TestLogModelReadsHistoricalSchemaWithoutBillingDetails 验证从节点行为：
// 从节点不执行迁移（dbmigrate 按 IsMasterNode 跳过），滚动升级期间可能先于
// 主节点面对无 billing_details 列的共享库；GORM 以 SELECT * 读取，模型新字段
// 保持零值，查询不得报错。
//
// 已知局限（既有滚动升级约定，非本列独有）：该窗口内节点写入日志的 INSERT
// 会显式包含 billing_details 列而失败，consume 日志按既有错误路径记录后继续
// 服务，与 request_id/ua 等历史列上线时的行为一致。
func TestLogModelReadsHistoricalSchemaWithoutBillingDetails(t *testing.T) {
	setupBillingDetailsTestDB(t)

	row := &logstore.Log{
		UserId:    1,
		CreatedAt: 1700000000,
		Type:      logstore.LogTypeConsume,
		ModelName: "gpt-test",
		Group:     "default",
		Other:     "{}",
	}
	if err := dbstore.LOG_DB.Create(row).Error; err != nil {
		t.Fatalf("seed log: %v", err)
	}
	if err := dbstore.LOG_DB.Migrator().DropColumn(&logstore.Log{}, "billing_details"); err != nil {
		t.Fatalf("simulate un-migrated schema: %v", err)
	}

	var logs []*logstore.Log
	if err := dbstore.LOG_DB.Model(&logstore.Log{}).Where("user_id = ?", 1).Find(&logs).Error; err != nil {
		t.Fatalf("read logs on schema without billing_details: %v", err)
	}
	if len(logs) != 1 {
		t.Fatalf("log count = %d, want 1", len(logs))
	}
	if logs[0].BillingDetails != nil {
		t.Fatalf("billing_details = %v on un-migrated schema, want nil", logs[0].BillingDetails)
	}
}

// TestRecordConsumeLogBillingDetailsReadableByParser 验证阶段 1 验收标准
// "新日志能读出 JSON"：归一化产物经 RecordConsumeLog 落库后，读出的列值
// 必须能被读取端解析器还原为同一语义；损坏 JSON 有显式错误路径。
func TestRecordConsumeLogBillingDetailsReadableByParser(t *testing.T) {
	setupBillingDetailsTestDB(t)

	bu := &billing.BillingUsage{
		PromptAggregateTokens: 200,
		OutputTokens:          100,
		CacheReadTokens:       30,
		CacheWriteTokens:      20,
	}
	bu.CacheWrite5mTokens = &bu.CacheWriteTokens
	raw, err := billing.SerializeBillingUsage(bu)
	if err != nil {
		t.Fatalf("serialize billing usage: %v", err)
	}

	ctx := newBillingDetailsTestContext()
	logstore.RecordConsumeLog(ctx, 1, logstore.RecordConsumeLogParams{
		ChannelId:        1,
		PromptTokens:     200,
		CompletionTokens: 100,
		ModelName:        "claude-sonnet-4-5",
		TokenName:        "tk-parser",
		Quota:            42,
		Content:          "billing details parser roundtrip",
		TokenId:          1,
		UseTimeMs:        100,
		IsStream:         false,
		Group:            "default",
		Other:            map[string]interface{}{"model_ratio": 1.0},
		BillingDetails:   raw,
	})
	stored := waitForConsumeLogRow(t, "token_name = ?", "tk-parser")
	if value := queryBillingDetailsRaw(t, stored.Id); value.String != raw || !value.Valid {
		t.Fatalf("stored billing_details = %q (valid=%v), want %q", value.String, value.Valid, raw)
	}

	if stored.BillingDetails == nil || *stored.BillingDetails != raw {
		t.Fatalf("model billing_details = %v, want %q", stored.BillingDetails, raw)
	}
	payload, err := billing.ParseBillingDetailsJSON(*stored.BillingDetails)
	if err != nil {
		t.Fatalf("parse stored billing_details %q: %v", *stored.BillingDetails, err)
	}
	if payload.Tokens.Cache.ReadCache != 30 {
		t.Fatalf("read cache = %v, want 30", payload.Tokens.Cache.ReadCache)
	}
	if payload.Tokens.Cache.WriteCache5m != 20 {
		t.Fatalf("write cache 5m = %v, want 20", payload.Tokens.Cache.WriteCache5m)
	}
	if payload.Tokens.Output.TextOutput != 100 {
		t.Fatalf("text output = %v, want 100", payload.Tokens.Output.TextOutput)
	}
	if payload.Tokens.Input.ImageInput != 0 {
		t.Fatalf("absent image input = %d, want 0", payload.Tokens.Input.ImageInput)
	}

	// 损坏 JSON 读取端显式失败，不做启发式猜测。
	if _, err := billing.ParseBillingDetailsJSON(`{"schema_version":9,"tokens":{}}`); err == nil {
		t.Fatalf("unknown schema version must fail explicitly")
	}
}

// TestLogQueriesProjectTokenAggregatesFromBillingDetails 验证验收环节四
// "必要的 Token 总量由服务端从 billing_details 投影"：查询函数按唯一汇总
// 公式填充 wire 投影字段（prompt_tokens=输入侧总量、completion_tokens=
// 输出总量），NULL 明细保持 0，损坏 JSON 显式报错。
func TestLogQueriesProjectTokenAggregatesFromBillingDetails(t *testing.T) {
	setupBillingDetailsTestDB(t)

	logs := []*logstore.Log{
		{
			UserId:         7,
			CreatedAt:      1700000001,
			Type:           logstore.LogTypeConsume,
			Username:       "proj-user",
			TokenName:      "tk-proj",
			ModelName:      "gpt-proj",
			Quota:          1,
			Group:          "default",
			BillingDetails: strPtr(billingDetailsFixture),
		},
		{
			UserId:    7,
			CreatedAt: 1700000002,
			Type:      logstore.LogTypeConsume,
			Username:  "proj-user",
			Quota:     1,
			Group:     "default",
		},
	}
	if err := dbstore.LOG_DB.Create(&logs).Error; err != nil {
		t.Fatalf("seed logs: %v", err)
	}

	// fixture：输入侧总量 = 12 + 4 + 5 = 21；输出总量 = 7。
	fetched, _, err := logstore.GetUserLogs(7, logstore.LogTypeConsume, 0, 0, "", "", 0, 10, "", "", "", "", "", "", "")
	if err != nil {
		t.Fatalf("GetUserLogs error = %v", err)
	}
	if len(fetched) != 2 {
		t.Fatalf("log count = %d, want 2", len(fetched))
	}
	for _, l := range fetched {
		if l.BillingDetails != nil {
			if l.PromptTokens != 21 || l.CompletionTokens != 7 {
				t.Fatalf("projected aggregates = (%d, %d), want (21, 7)", l.PromptTokens, l.CompletionTokens)
			}
		} else if l.PromptTokens != 0 || l.CompletionTokens != 0 {
			t.Fatalf("NULL details projected as (%d, %d), want (0, 0)", l.PromptTokens, l.CompletionTokens)
		}
	}

	adminLogs, _, err := logstore.GetAllLogs(logstore.LogTypeConsume, 0, 0, "", "proj-user", "", 0, 10, 0, "", "", "", "", "", "", "")
	if err != nil {
		t.Fatalf("GetAllLogs error = %v", err)
	}
	if len(adminLogs) != 2 {
		t.Fatalf("admin log count = %d, want 2", len(adminLogs))
	}
	for _, l := range adminLogs {
		if l.BillingDetails != nil && (l.PromptTokens != 21 || l.CompletionTokens != 7) {
			t.Fatalf("admin path projected aggregates = (%d, %d), want (21, 7)", l.PromptTokens, l.CompletionTokens)
		}
	}

	corrupt := &logstore.Log{
		UserId:         8,
		CreatedAt:      1700000003,
		Type:           logstore.LogTypeConsume,
		Username:       "proj-corrupt",
		Quota:          1,
		Group:          "default",
		BillingDetails: strPtr(`{"schema_version":2,"tokens":{}}`),
	}
	if err := dbstore.LOG_DB.Create(corrupt).Error; err != nil {
		t.Fatalf("seed corrupt log: %v", err)
	}
	if _, _, err := logstore.GetUserLogs(8, logstore.LogTypeConsume, 0, 0, "", "", 0, 10, "", "", "", "", "", "", ""); err == nil {
		t.Fatal("GetUserLogs must fail explicitly on corrupt billing_details, not project zero totals")
	}
}
