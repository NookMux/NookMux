package dbmigrate

import (
	"context"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/NookMux/NookMux/internal/common"
	"github.com/NookMux/NookMux/internal/domain/billing"
	"github.com/NookMux/NookMux/internal/store/db"
	"github.com/NookMux/NookMux/internal/store/log"
	"github.com/NookMux/NookMux/pkg/jsonx"
	"gorm.io/gorm"
)

// logBillingTokenDetailsBatchSize controls one atomic update transaction. The
// size is deliberately moderate: migrations must be resumable without creating
// long transactions on production log tables.
var logBillingTokenDetailsBatchSize = 1000

// legacyBillingTokenKeys is the closed set of token-detail keys that moved from
// Other to billing_details. Every other key in Other remains untouched.
var legacyBillingTokenKeys = []string{
	"cache_tokens",
	"cache_creation_tokens",
	"cache_creation_tokens_5m",
	"cache_creation_tokens_1h",
	"text_input",
	"text_output",
	"audio_input",
	"audio_output",
	"image_output",
	"audio_input_token_count",
}

type legacyBillingDetailsRow struct {
	Id               int
	Type             int
	PromptTokens     int
	CompletionTokens int
	Other            string
	BillingDetails   *string
}

type legacyTokenValues struct {
	CacheRead             *int
	CacheWrite            *int
	CacheWrite5m          *int
	CacheWrite1h          *int
	TextInput             *int
	TextOutput            *int
	AudioInput            *int
	AudioOutput           *int
	ImageOutput           *int
	HasExplicitTokenValue bool
}

// backfillLogBillingTokenDetails is the one-shot startup data migration. It is
// idempotent through logs.billing_details_version: every update is committed in
// a batch transaction and any failure aborts startup instead of leaving readers
// to guess whether Other or billing_details is authoritative.
func backfillLogBillingTokenDetails() error {
	if dbstore.LOG_DB == nil {
		return fmt.Errorf("log database is not initialized")
	}

	// Keep the marker in the log database: the main DB may serve a different
	// log database. Existing optionstore markers are best-effort and main-DB-only.
	completed := false
	if err := retryLogBillingMigration(dbstore.LOG_DB, func(db *gorm.DB) error {
		if err := db.AutoMigrate(&logBillingMigrationState{}); err != nil {
			return err
		}
		var count int64
		if err := db.Model(&logBillingMigrationState{}).Where("version = ?", logstore.LogBillingDetailsVersion).Count(&count).Error; err != nil {
			return err
		}
		completed = count == 1
		return nil
	}); err != nil {
		return fmt.Errorf("initialize billing migration marker: %w", err)
	}
	if completed {
		common.SysLog("backfillLogBillingTokenDetails: already completed")
		return nil
	}
	// 旧聚合列不存在（全新空库，或已由 dropLegacyLogTokenAggregateColumns
	// 删除）时没有可迁移的来源：直接确保完成标记存在后返回。不能省略这一步：
	// 下方扫描按列名 SELECT 旧列，列缺失时即使空表也会报错并阻断启动。
	legacyPresent, err := legacyLogAggregateColumnsPresent()
	if err != nil {
		return fmt.Errorf("inspect legacy log token columns: %w", err)
	}
	if !legacyPresent {
		if err := saveLogBillingMigrationMarker(); err != nil {
			return fmt.Errorf("complete billing migration marker: %w", err)
		}
		common.SysLog("backfillLogBillingTokenDetails: legacy token aggregate columns absent, marked complete without scan")
		return nil
	}
	migrated := int64(0)
	lastID := 0
	started := time.Now()
	for {
		var rows []legacyBillingDetailsRow
		err := retryLogBillingMigration(dbstore.LOG_DB, func(db *gorm.DB) error {
			rows = nil
			return db.Model(&logstore.Log{}).
				Select("id, type, prompt_tokens, completion_tokens, other, billing_details").
				Where("id > ? AND billing_details_version < ?", lastID, logstore.LogBillingDetailsVersion).
				Order("id ASC").Limit(logBillingTokenDetailsBatchSize).Find(&rows).Error
		})
		if err != nil {
			return fmt.Errorf("query logs for billing details migration: %w", err)
		}
		if len(rows) == 0 {
			if err := saveLogBillingMigrationMarker(); err != nil {
				return fmt.Errorf("complete billing migration marker: %w", err)
			}
			common.SysLog(fmt.Sprintf("backfillLogBillingTokenDetails: completed, %d rows migrated, last_id=%d elapsed=%s", migrated, lastID, time.Since(started)))
			return nil
		}
		// Validate once before opening a transaction. Bad historical data is not a
		// transient database error and must fail immediately without retries.
		updates := make([]map[string]interface{}, len(rows))
		for i, row := range rows {
			details, cleanedOther, changed, err := migrateLegacyBillingDetails(row)
			if err != nil {
				return fmt.Errorf("log id=%d: %w", row.Id, err)
			}
			updates[i] = map[string]interface{}{"billing_details_version": logstore.LogBillingDetailsVersion, "billing_details": details}
			if changed {
				updates[i]["other"] = cleanedOther
			}
		}
		err = retryLogBillingMigration(dbstore.LOG_DB, func(db *gorm.DB) error {
			return db.Transaction(func(tx *gorm.DB) error {
				for i, row := range rows {
					// Version guard also makes retry safe if the commit result was lost.
					if err := tx.Model(&logstore.Log{}).Where("id = ? AND billing_details_version < ?", row.Id, logstore.LogBillingDetailsVersion).Updates(updates[i]).Error; err != nil {
						return fmt.Errorf("log id=%d: update migrated billing details: %w", row.Id, err)
					}
				}
				return nil
			})
		})
		if err != nil {
			return err
		}
		migrated += int64(len(rows))
		lastID = rows[len(rows)-1].Id
		common.SysLog(fmt.Sprintf("backfillLogBillingTokenDetails: rows=%d last_id=%d elapsed=%s", migrated, lastID, time.Since(started)))
	}
}

// Presence means all batches committed. Old writers MUST be drained before
// this release starts; a marker cannot fence an already-running old binary.
type logBillingMigrationState struct {
	Version int `gorm:"primaryKey;autoIncrement:false"`
}

// saveLogBillingMigrationMarker 在实际日志库写入完成标记。写入失败必须重试
// 耗尽后显式报错，不得让后续正常重启重复全表回填。
func saveLogBillingMigrationMarker() error {
	return retryLogBillingMigration(dbstore.LOG_DB, func(db *gorm.DB) error {
		return db.Save(&logBillingMigrationState{Version: logstore.LogBillingDetailsVersion}).Error
	})
}

// legacyLogAggregateColumnsPresent 报告旧聚合列是否仍存在于实际日志库。
// 正常状态是两列同时存在（升级前）或同时不存在（全新库/已删列）；
// 仅存在其一属于异常状态，返回 true 让扫描显式失败暴露问题。
func legacyLogAggregateColumnsPresent() (bool, error) {
	if dbstore.LOG_DB == nil {
		return false, fmt.Errorf("log database is not initialized")
	}
	migrator := dbstore.LOG_DB.Migrator()
	for _, col := range legacyLogTokenAggregateColumns {
		present, err := logColumnPresent(migrator, col)
		if err != nil {
			return false, err
		}
		if present {
			return true, nil
		}
	}
	return false, nil
}

func retryLogBillingMigration(db *gorm.DB, operation func(*gorm.DB) error) error {
	var err error
	for attempt := 0; attempt < 3; attempt++ {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		err = operation(db.WithContext(ctx))
		cancel()
		if err == nil {
			return nil
		}
		common.SysError(fmt.Sprintf("billing details migration database attempt %d/3 failed: %v", attempt+1, err))
		if attempt < 2 {
			time.Sleep(time.Duration(1<<attempt) * 250 * time.Millisecond)
		}
	}
	return err
}

// migrateLegacyBillingDetails converts one historical row. A valid existing
// billing_details payload is authoritative for fields it already contains;
// fields absent from that payload may be filled from legacy Other. The returned
// Other JSON removes only the migrated token-detail keys.
func migrateLegacyBillingDetails(row legacyBillingDetailsRow) (*string, string, bool, error) {
	otherMap, err := decodeOtherObject(row.Other)
	if err != nil {
		return nil, "", false, err
	}

	values, err := extractLegacyTokenValues(otherMap)
	if err != nil {
		return nil, "", false, err
	}

	var payload *billing.BillingDetailsPayload
	presence := map[string]bool{}
	if row.BillingDetails != nil && *row.BillingDetails != "" {
		payload, err = billing.ParseLegacyBillingDetailsJSON(*row.BillingDetails)
		if err != nil {
			return nil, "", false, fmt.Errorf("validate existing billing_details: %w", err)
		}
		presence, err = billingDetailsFieldPresence(*row.BillingDetails)
		if err != nil {
			return nil, "", false, fmt.Errorf("inspect existing billing_details: %w", err)
		}
	}

	if payload != nil {
		if err := mergeLegacyTokenValues(payload, presence, values); err != nil {
			return nil, "", false, err
		}
	} else if (row.Type == logstore.LogTypeConsume && (row.PromptTokens != 0 || row.CompletionTokens != 0)) || values.HasExplicitTokenValue {
		payload = &billing.BillingDetailsPayload{
			SchemaVersion: billing.BillingDetailsSchemaVersion,
		}
		if err := mergeLegacyTokenValues(payload, presence, values); err != nil {
			return nil, "", false, err
		}
	}

	if payload != nil {
		if err := deriveRemainingTokenDetails(payload, presence, row.PromptTokens, row.CompletionTokens); err != nil {
			return nil, "", false, err
		}
	}

	var details *string
	if payload != nil {
		encoded, encodeErr := jsonx.Marshal(payload)
		if encodeErr != nil {
			return nil, "", false, fmt.Errorf("serialize migrated billing_details: %w", encodeErr)
		}
		encodedDetails := string(encoded)
		if _, validateErr := billing.ParseBillingDetailsJSON(encodedDetails); validateErr != nil {
			return nil, "", false, fmt.Errorf("validate migrated billing_details: %w", validateErr)
		}
		details = &encodedDetails
	}

	cleanedOther := otherMap
	changed := false
	for _, key := range legacyBillingTokenKeys {
		if _, exists := cleanedOther[key]; exists {
			delete(cleanedOther, key)
			changed = true
		}
	}
	encodedOther := row.Other
	if changed {
		encodedOther = common.MapToJsonStr(cleanedOther)
		if encodedOther == "" {
			return nil, "", false, fmt.Errorf("serialize cleaned other")
		}
	}
	return details, encodedOther, changed, nil
}

func billingDetailsFieldPresence(raw string) (map[string]bool, error) {
	var probe map[string]interface{}
	if err := jsonx.Unmarshal([]byte(raw), &probe); err != nil {
		return nil, err
	}
	tokens, _ := probe["tokens"].(map[string]interface{})
	result := make(map[string]bool)
	for _, group := range []string{"input", "output", "cache"} {
		groupMap, _ := tokens[group].(map[string]interface{})
		for field, value := range groupMap {
			if value != nil {
				result[group+"."+field] = true
			}
		}
	}
	return result, nil
}

func decodeOtherObject(raw string) (map[string]interface{}, error) {
	if strings.TrimSpace(raw) == "" {
		return map[string]interface{}{}, nil
	}
	values, err := common.StrToMap(raw)
	if err != nil || values == nil {
		return nil, fmt.Errorf("other is not a JSON object: %w", err)
	}
	return values, nil
}

func extractLegacyTokenValues(values map[string]interface{}) (*legacyTokenValues, error) {
	result := &legacyTokenValues{}
	read, err := legacyTokenValue(values, "cache_tokens")
	if err != nil {
		return nil, err
	}
	write, err := legacyTokenValue(values, "cache_creation_tokens")
	if err != nil {
		return nil, err
	}
	write5m, err := legacyTokenValue(values, "cache_creation_tokens_5m")
	if err != nil {
		return nil, err
	}
	write1h, err := legacyTokenValue(values, "cache_creation_tokens_1h")
	if err != nil {
		return nil, err
	}
	textInput, err := legacyTokenValue(values, "text_input")
	if err != nil {
		return nil, err
	}
	textOutput, err := legacyTokenValue(values, "text_output")
	if err != nil {
		return nil, err
	}
	audioInput, err := legacyTokenValue(values, "audio_input")
	if err != nil {
		return nil, err
	}
	audioInputCount, err := legacyTokenValue(values, "audio_input_token_count")
	if err != nil {
		return nil, err
	}
	if audioInput != nil && audioInputCount != nil && *audioInput != *audioInputCount {
		return nil, fmt.Errorf("conflicting audio input token counts: audio_input=%d, audio_input_token_count=%d", *audioInput, *audioInputCount)
	}
	if audioInput == nil {
		audioInput = audioInputCount
	}
	audioOutput, err := legacyTokenValue(values, "audio_output")
	if err != nil {
		return nil, err
	}
	imageOutput, err := legacyTokenValue(values, "image_output")
	if err != nil {
		return nil, err
	}

	result.CacheRead = read
	result.CacheWrite = write
	result.CacheWrite5m = write5m
	result.CacheWrite1h = write1h
	result.TextInput = textInput
	result.TextOutput = textOutput
	result.AudioInput = audioInput
	result.AudioOutput = audioOutput
	result.ImageOutput = imageOutput
	result.HasExplicitTokenValue = anyTokenValuePresent(result)
	return result, nil
}

func mergeLegacyTokenValues(payload *billing.BillingDetailsPayload, presence map[string]bool, values *legacyTokenValues) error {
	if err := mergeLegacyToken(&payload.Tokens.Cache.ReadCache, presence["cache.read_cache"], values.CacheRead, "cache.read_cache"); err != nil {
		return err
	}
	if err := mergeLegacyToken(&payload.Tokens.Cache.WriteCache, presence["cache.write_cache"], values.CacheWrite, "cache.write_cache"); err != nil {
		return err
	}
	if err := mergeLegacyToken(&payload.Tokens.Cache.WriteCache5m, presence["cache.write_cache_5m"], values.CacheWrite5m, "cache.write_cache_5m"); err != nil {
		return err
	}
	if err := mergeLegacyToken(&payload.Tokens.Cache.WriteCache1h, presence["cache.write_cache_1h"], values.CacheWrite1h, "cache.write_cache_1h"); err != nil {
		return err
	}
	if err := mergeLegacyToken(&payload.Tokens.Input.TextInput, presence["input.text_input"], values.TextInput, "input.text_input"); err != nil {
		return err
	}
	if err := mergeLegacyToken(&payload.Tokens.Input.AudioInput, presence["input.audio_input"], values.AudioInput, "input.audio_input"); err != nil {
		return err
	}
	if err := mergeLegacyToken(&payload.Tokens.Output.TextOutput, presence["output.text_output"], values.TextOutput, "output.text_output"); err != nil {
		return err
	}
	if err := mergeLegacyToken(&payload.Tokens.Output.AudioOutput, presence["output.audio_output"], values.AudioOutput, "output.audio_output"); err != nil {
		return err
	}
	if err := mergeLegacyToken(&payload.Tokens.Output.ImageOutput, presence["output.image_output"], values.ImageOutput, "output.image_output"); err != nil {
		return err
	}
	return nil
}

func mergeLegacyToken(destination *int, destinationPresent bool, value *int, field string) error {
	if value == nil {
		return nil
	}
	if destinationPresent && *destination != *value {
		return fmt.Errorf("conflicting token count for %s: existing=%d, other=%d", field, *destination, *value)
	}
	*destination = *value
	return nil
}

// deriveRemainingTokenDetails fills ordinary text input/output from the old
// aggregate columns. Explicit legacy details always win; known details that
// exceed an aggregate are contradictory data and must stop the migration.
func deriveRemainingTokenDetails(payload *billing.BillingDetailsPayload, presence map[string]bool, promptTokens, completionTokens int) error {
	if promptTokens < 0 {
		return fmt.Errorf("negative prompt_tokens=%d", promptTokens)
	}
	if completionTokens < 0 {
		return fmt.Errorf("negative completion_tokens=%d", completionTokens)
	}

	inputOthers := []int{
		payload.Tokens.Input.ImageInput,
		payload.Tokens.Input.AudioInput,
		payload.Tokens.Input.VideoInput,
		payload.Tokens.Input.DocumentInput,
		payload.Tokens.Cache.ReadCache,
		payload.Tokens.Cache.WriteCache,
	}
	inputKnown, err := checkedTokenSum(inputOthers...)
	if err != nil {
		return err
	}
	if !presence["input.text_input"] {
		if promptTokens < inputKnown {
			return fmt.Errorf("input details (%d) exceed prompt_tokens (%d)", inputKnown, promptTokens)
		}
		payload.Tokens.Input.TextInput = promptTokens - inputKnown
	} else {
		inputKnown, err = checkedTokenSum(inputKnown, payload.Tokens.Input.TextInput)
		if err != nil {
			return err
		}
		if promptTokens < inputKnown {
			return fmt.Errorf("input details (%d) exceed prompt_tokens (%d)", inputKnown, promptTokens)
		}
	}

	outputOthers := []int{
		payload.Tokens.Output.AudioOutput,
		payload.Tokens.Output.ImageOutput,
	}
	outputKnown, err := checkedTokenSum(outputOthers...)
	if err != nil {
		return err
	}
	if !presence["output.text_output"] {
		if completionTokens < outputKnown {
			return fmt.Errorf("output details (%d) exceed completion_tokens (%d)", outputKnown, completionTokens)
		}
		payload.Tokens.Output.TextOutput = completionTokens - outputKnown
	} else {
		outputKnown, err = checkedTokenSum(outputKnown, payload.Tokens.Output.TextOutput)
		if err != nil {
			return err
		}
		if completionTokens < outputKnown {
			return fmt.Errorf("output details (%d) exceed completion_tokens (%d)", outputKnown, completionTokens)
		}
	}
	return nil
}

func legacyTokenValue(values map[string]interface{}, key string) (*int, error) {
	raw, exists := values[key]
	if !exists || raw == nil {
		return nil, nil
	}
	number, ok := raw.(float64)
	if !ok {
		return nil, fmt.Errorf("%s is not an integer token count (%T)", key, raw)
	}
	if number != math.Trunc(number) || number < 0 || number > float64(math.MaxInt) {
		return nil, fmt.Errorf("%s is not a non-negative integer token count: %v", key, number)
	}
	value := int(number)
	return &value, nil
}

func anyTokenValuePresent(values *legacyTokenValues) bool {
	return values.CacheRead != nil || values.CacheWrite != nil ||
		values.CacheWrite5m != nil || values.CacheWrite1h != nil ||
		values.TextInput != nil || values.TextOutput != nil ||
		values.AudioInput != nil || values.AudioOutput != nil ||
		values.ImageOutput != nil
}

func checkedTokenSum(values ...int) (int, error) {
	sum := 0
	for _, value := range values {
		if value < 0 {
			return 0, fmt.Errorf("negative token detail value %d", value)
		}
		if value > math.MaxInt-sum {
			return 0, fmt.Errorf("token detail sum overflow")
		}
		sum += value
	}
	return sum, nil
}

func intValue(value *int) int {
	if value == nil {
		return 0
	}
	return *value
}
