package dbmigrate

import (
	"fmt"
	"testing"

	"github.com/NookMux/NookMux/internal/domain/billing"
	"github.com/NookMux/NookMux/internal/store/db"
	"github.com/NookMux/NookMux/internal/store/log"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

// legacyAggregateColumns 模拟计费重构前的历史 schema：logs 表曾有的两个
// 聚合列。当前模型不再声明它们（读取为 wire 投影字段），测试用原始 SQL
// 添加列、按结构体声明值回填，等价还原旧版数据库。
var legacyAggregateColumns = [2]string{"prompt_tokens", "completion_tokens"}

func addLegacyAggregateColumns(t *testing.T, dbHandle *gorm.DB) {
	t.Helper()
	for _, col := range legacyAggregateColumns {
		if err := dbHandle.Exec("ALTER TABLE logs ADD COLUMN " + col + " BIGINT DEFAULT 0").Error; err != nil {
			t.Fatalf("simulate legacy schema by adding %s: %v", col, err)
		}
	}
}

func seedLegacyAggregates(t *testing.T, dbHandle *gorm.DB, id int, promptTokens int, completionTokens int) {
	t.Helper()
	if err := dbHandle.Exec("UPDATE logs SET prompt_tokens = ?, completion_tokens = ? WHERE id = ?", promptTokens, completionTokens, id).Error; err != nil {
		t.Fatalf("seed legacy aggregate columns for log id=%d: %v", id, err)
	}
}

func assertLegacyAggregates(t *testing.T, dbHandle *gorm.DB, id int, promptTokens int, completionTokens int) {
	t.Helper()
	var got struct {
		PromptTokens     int
		CompletionTokens int
	}
	if err := dbHandle.Raw("SELECT prompt_tokens, completion_tokens FROM logs WHERE id = ?", id).Row().Scan(&got.PromptTokens, &got.CompletionTokens); err != nil {
		t.Fatalf("read legacy aggregate columns for log id=%d: %v", id, err)
	}
	if got.PromptTokens != promptTokens || got.CompletionTokens != completionTokens {
		t.Fatalf("legacy aggregates for log id=%d = (%d, %d), want (%d, %d)",
			id, got.PromptTokens, got.CompletionTokens, promptTokens, completionTokens)
	}
}

func setupTokenDetailsMigrationDB(t *testing.T) *gorm.DB {
	t.Helper()
	dbHandle, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := dbHandle.AutoMigrate(&logstore.Log{}); err != nil {
		t.Fatalf("migrate sqlite: %v", err)
	}
	addLegacyAggregateColumns(t, dbHandle)
	return dbHandle
}

func seedTokenMigrationLog(t *testing.T, dbHandle *gorm.DB, row logstore.Log) logstore.Log {
	t.Helper()
	if err := dbHandle.Create(&row).Error; err != nil {
		t.Fatalf("seed log: %v", err)
	}
	// 聚合字段是 gorm:"-" 投影字段不落库；历史 schema 模拟需要旧列真实数据，
	// 按结构体声明值回填。
	if row.PromptTokens != 0 || row.CompletionTokens != 0 {
		seedLegacyAggregates(t, dbHandle, row.Id, row.PromptTokens, row.CompletionTokens)
	}
	return row
}

func tokenMigrationStoredRow(t *testing.T, dbHandle *gorm.DB, id int) logstore.Log {
	t.Helper()
	var stored logstore.Log
	if err := dbHandle.First(&stored, id).Error; err != nil {
		t.Fatalf("reload log id=%d: %v", id, err)
	}
	return stored
}

func requireTokenDetails(t *testing.T, raw *string) *billing.BillingDetailsPayload {
	t.Helper()
	if raw == nil {
		t.Fatal("billing_details is nil")
	}
	payload, err := billing.ParseBillingDetailsJSON(*raw)
	if err != nil {
		t.Fatalf("parse billing_details %q: %v", *raw, err)
	}
	return payload
}

func requireTokenValue(t *testing.T, value int, want int) {
	t.Helper()
	if value != want {
		t.Fatalf("token value = %d, want %d", value, want)
	}
}

func TestBackfillLogBillingTokenDetailsHistoricalRows(t *testing.T) {
	dbHandle := setupTokenDetailsMigrationDB(t)
	oldLogDB := dbstore.LOG_DB
	oldMainDB := dbstore.DB
	dbstore.LOG_DB = dbHandle
	dbstore.DB = dbHandle
	t.Cleanup(func() {
		dbstore.LOG_DB = oldLogDB
		dbstore.DB = oldMainDB
	})

	generic := seedTokenMigrationLog(t, dbHandle, logstore.Log{
		Type:             logstore.LogTypeConsume,
		PromptTokens:     100,
		CompletionTokens: 50,
		Other:            `{"cache_tokens":20,"cache_ratio":0.5,"request_path":"/v1/chat"}`,
	})
	claude := seedTokenMigrationLog(t, dbHandle, logstore.Log{
		Type:             logstore.LogTypeConsume,
		PromptTokens:     150,
		CompletionTokens: 50,
		Other:            `{"cache_tokens":30,"cache_creation_tokens":40,"cache_creation_tokens_5m":30,"cache_creation_tokens_1h":10,"cache_creation_ratio":2,"claude":true}`,
	})
	audio := seedTokenMigrationLog(t, dbHandle, logstore.Log{
		Type:             logstore.LogTypeConsume,
		PromptTokens:     120,
		CompletionTokens: 50,
		Other:            `{"audio_input":20,"audio_output":10,"text_input":100,"text_output":40,"audio":true}`,
	})
	image := seedTokenMigrationLog(t, dbHandle, logstore.Log{
		Type:             logstore.LogTypeConsume,
		PromptTokens:     10,
		CompletionTokens: 30,
		Other:            `{"image_output":5}`,
	})
	explicitZero := seedTokenMigrationLog(t, dbHandle, logstore.Log{
		Type:  logstore.LogTypeConsume,
		Other: `{"cache_tokens":0,"audio_input_token_count":0,"model_ratio":1}`,
	})

	if err := backfillLogBillingTokenDetails(); err != nil {
		t.Fatalf("backfill: %v", err)
	}

	genericRow := tokenMigrationStoredRow(t, dbHandle, generic.Id)
	if genericRow.BillingDetailsVersion != logstore.LogBillingDetailsVersion {
		t.Fatalf("generic version = %d", genericRow.BillingDetailsVersion)
	}
	genericPayload := requireTokenDetails(t, genericRow.BillingDetails)
	requireTokenValue(t, genericPayload.Tokens.Input.TextInput, 80)
	requireTokenValue(t, genericPayload.Tokens.Output.TextOutput, 50)
	requireTokenValue(t, genericPayload.Tokens.Cache.ReadCache, 20)
	if genericRow.Other != `{"cache_ratio":0.5,"request_path":"/v1/chat"}` {
		t.Fatalf("generic other = %q", genericRow.Other)
	}

	claudeRow := tokenMigrationStoredRow(t, dbHandle, claude.Id)
	claudePayload := requireTokenDetails(t, claudeRow.BillingDetails)
	requireTokenValue(t, claudePayload.Tokens.Input.TextInput, 80)
	requireTokenValue(t, claudePayload.Tokens.Output.TextOutput, 50)
	requireTokenValue(t, claudePayload.Tokens.Cache.ReadCache, 30)
	requireTokenValue(t, claudePayload.Tokens.Cache.WriteCache, 40)
	requireTokenValue(t, claudePayload.Tokens.Cache.WriteCache5m, 30)
	requireTokenValue(t, claudePayload.Tokens.Cache.WriteCache1h, 10)
	if claudeRow.Other != `{"cache_creation_ratio":2,"claude":true}` {
		t.Fatalf("claude other = %q", claudeRow.Other)
	}

	audioRow := tokenMigrationStoredRow(t, dbHandle, audio.Id)
	audioPayload := requireTokenDetails(t, audioRow.BillingDetails)
	requireTokenValue(t, audioPayload.Tokens.Input.TextInput, 100)
	requireTokenValue(t, audioPayload.Tokens.Input.AudioInput, 20)
	requireTokenValue(t, audioPayload.Tokens.Output.TextOutput, 40)
	requireTokenValue(t, audioPayload.Tokens.Output.AudioOutput, 10)
	if audioRow.Other != `{"audio":true}` {
		t.Fatalf("audio other = %q", audioRow.Other)
	}

	imageRow := tokenMigrationStoredRow(t, dbHandle, image.Id)
	imagePayload := requireTokenDetails(t, imageRow.BillingDetails)
	requireTokenValue(t, imagePayload.Tokens.Output.TextOutput, 25)
	requireTokenValue(t, imagePayload.Tokens.Output.ImageOutput, 5)
	if imageRow.Other != `{}` {
		t.Fatalf("image other = %q", imageRow.Other)
	}

	zeroRow := tokenMigrationStoredRow(t, dbHandle, explicitZero.Id)
	zeroPayload := requireTokenDetails(t, zeroRow.BillingDetails)
	requireTokenValue(t, zeroPayload.Tokens.Input.TextInput, 0)
	requireTokenValue(t, zeroPayload.Tokens.Input.AudioInput, 0)
	requireTokenValue(t, zeroPayload.Tokens.Cache.ReadCache, 0)
	if zeroRow.Other != `{"model_ratio":1}` {
		t.Fatalf("zero other = %q", zeroRow.Other)
	}

	before := make(map[int]string, 4)
	for _, row := range []logstore.Log{genericRow, claudeRow, audioRow, imageRow, zeroRow} {
		before[row.Id] = *row.BillingDetails
	}
	if err := backfillLogBillingTokenDetails(); err != nil {
		t.Fatalf("repeat backfill: %v", err)
	}
	for id, details := range before {
		stored := tokenMigrationStoredRow(t, dbHandle, id)
		if stored.BillingDetails == nil || *stored.BillingDetails != details {
			t.Fatalf("id=%d billing_details changed on second run: %v, want %q", id, stored.BillingDetails, details)
		}
	}
}

func TestBackfillLogBillingTokenDetailsPreservesValidDetails(t *testing.T) {
	dbHandle := setupTokenDetailsMigrationDB(t)
	oldLogDB := dbstore.LOG_DB
	dbstore.LOG_DB = dbHandle
	t.Cleanup(func() { dbstore.LOG_DB = oldLogDB })

	existing := `{"schema_version":1,"tokens":{"input":{"text_input":10},"output":{},"cache":{"read_cache":4}}}`
	complete := `{"schema_version":1,"tokens":{"input":{"text_input":10,"image_input":0,"audio_input":0,"video_input":0,"document_input":0},"output":{"text_output":20,"audio_output":0,"image_output":0,"reasoning_output":0,"accepted_prediction":0,"rejected_prediction":0},"cache":{"read_cache":4,"write_cache":0,"write_cache_5m":0,"write_cache_1h":0}}}`
	row := seedTokenMigrationLog(t, dbHandle, logstore.Log{
		Type:             logstore.LogTypeConsume,
		PromptTokens:     100,
		CompletionTokens: 20,
		Other:            `{"cache_tokens":4,"cache_ratio":1}`,
		BillingDetails:   &existing,
	})

	if err := backfillLogBillingTokenDetails(); err != nil {
		t.Fatalf("backfill: %v", err)
	}
	stored := tokenMigrationStoredRow(t, dbHandle, row.Id)
	if stored.BillingDetails == nil || *stored.BillingDetails != complete {
		t.Fatalf("billing_details = %v, want complete payload %q", stored.BillingDetails, complete)
	}
	if stored.Other != `{"cache_ratio":1}` {
		t.Fatalf("other = %q", stored.Other)
	}
}

func TestBackfillLogBillingTokenDetailsFailureBlocks(t *testing.T) {
	tests := []struct {
		name  string
		other string
	}{
		{name: "malformed other", other: `{`},
		{name: "negative detail", other: `{"cache_tokens":-1}`},
		{name: "fractional detail", other: `{"cache_tokens":1.5}`},
		{name: "details exceed aggregate", other: `{"cache_tokens":101}`},
		{name: "conflicting existing details", other: `{"cache_tokens":5}`},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			dbHandle := setupTokenDetailsMigrationDB(t)
			oldLogDB := dbstore.LOG_DB
			dbstore.LOG_DB = dbHandle
			t.Cleanup(func() { dbstore.LOG_DB = oldLogDB })

			row := logstore.Log{
				Type:             logstore.LogTypeConsume,
				PromptTokens:     100,
				CompletionTokens: 50,
				Other:            test.other,
			}
			if test.name == "conflicting existing details" {
				details := `{"schema_version":1,"tokens":{"input":{},"output":{},"cache":{"read_cache":4}}}`
				row.BillingDetails = &details
			}
			row = seedTokenMigrationLog(t, dbHandle, row)

			err := backfillLogBillingTokenDetails()
			if err == nil {
				t.Fatalf("%s: backfill succeeded, want explicit failure", test.name)
			}
			var version int
			if err := dbHandle.Raw("SELECT billing_details_version FROM logs WHERE id = ?", row.Id).Row().Scan(&version); err != nil {
				t.Fatalf("read version: %v", err)
			}
			if version != 0 {
				t.Fatalf("failed row was marked version %d", version)
			}
		})
	}
}

func TestBackfillLogBillingTokenDetailsBatchFailureRollsBack(t *testing.T) {
	dbHandle := setupTokenDetailsMigrationDB(t)
	oldLogDB := dbstore.LOG_DB
	dbstore.LOG_DB = dbHandle
	t.Cleanup(func() { dbstore.LOG_DB = oldLogDB })

	valid := seedTokenMigrationLog(t, dbHandle, logstore.Log{
		Type:             logstore.LogTypeConsume,
		PromptTokens:     100,
		CompletionTokens: 50,
		Other:            `{"cache_tokens":20,"model_ratio":1}`,
	})
	invalid := seedTokenMigrationLog(t, dbHandle, logstore.Log{
		Type:             logstore.LogTypeConsume,
		PromptTokens:     100,
		CompletionTokens: 50,
		Other:            `{"cache_tokens":101}`,
	})

	err := backfillLogBillingTokenDetails()
	if err == nil {
		t.Fatal("backfill succeeded, want batch failure")
	}
	for _, row := range []logstore.Log{valid, invalid} {
		var version int
		var other string
		if err := dbHandle.Raw("SELECT billing_details_version, other FROM logs WHERE id = ?", row.Id).Row().Scan(&version, &other); err != nil {
			t.Fatalf("read row id=%d: %v", row.Id, err)
		}
		if version != 0 {
			t.Fatalf("row id=%d version=%d, want 0 after batch rollback", row.Id, version)
		}
	}
}
