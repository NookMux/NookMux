package billing

import (
	"testing"
	"time"

	"github.com/NookMux/NookMux/internal/common"
	"github.com/NookMux/NookMux/internal/domain/billing/contract"
	"github.com/NookMux/NookMux/internal/domain/shared"
	"github.com/NookMux/NookMux/internal/httpapi"
	relaycommon "github.com/NookMux/NookMux/internal/relay/common"
	relayconstant "github.com/NookMux/NookMux/internal/relay/constant"
	dbstore "github.com/NookMux/NookMux/internal/store/db"
	logstore "github.com/NookMux/NookMux/internal/store/log"
	tokenstore "github.com/NookMux/NookMux/internal/store/token"
	userstore "github.com/NookMux/NookMux/internal/store/user"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// 计费 PRD 阶段 2 入口级测试：四个计费入口（Claude / audio / realtime WSS /
// 通用）切换到归一化 BillingUsage 后，同一条日志的 quota（PRD 3.4 公式）、
// billing_details Token 明细与现有计费快照能分别解释。

func newEntryTestContext(tokenName string) *gin.Context {
	c := newApplyQuotaTestContext()
	c.Set("token_name", tokenName)
	return c
}

func newEntryTestRelayInfo(source relayconstant.UsageSource) *relaycommon.RelayInfo {
	relayInfo := newApplyQuotaTestRelayInfo()
	relayInfo.UsageSource = source
	relayInfo.PriceData = contract.PriceData{
		ModelRatio:           2,
		CompletionRatio:      3,
		CacheRatio:           0.5,
		CacheCreationRatio:   1.25,
		CacheCreation5mRatio: 1.25,
		CacheCreation1hRatio: 2.0,
		AudioRatio:           8,
		AudioCompletionRatio: 2,
		GroupRatioInfo:       contract.GroupRatioInfo{GroupRatio: 1},
	}
	relayInfo.StartTime = time.Now().Add(-2 * time.Second)
	return relayInfo
}

// Claude 入口（PRD 3.4）：普通输入×1 + 缓存读取×读取价 + 5m/1h 分档写入 +
// 输出×补全倍率；输入侧/输出总量按唯一汇总公式从 billing_details 还原；
// 计费快照分别可解释。
func TestPostClaudeConsumeQuotaNormalizedFormula(t *testing.T) {
	setupApplyQuotaTestDB(t)
	ctx := newEntryTestContext("tk-claude")
	relayInfo := newEntryTestRelayInfo(relayconstant.UsageSourceClaude)

	usage := &shared.Usage{
		PromptTokens:     1000, // 聚合 = input(700) + read(200) + write(100)
		CompletionTokens: 500,
		TotalTokens:      1500,
	}
	usage.PromptTokensDetails.CachedTokens = 200
	usage.PromptTokensDetails.CachedCreationTokens = 100
	usage.ClaudeCacheCreation5mTokens = 60
	usage.ClaudeCacheCreation1hTokens = 40

	require.Nil(t, PostClaudeConsumeQuota(ctx, relayInfo, usage))

	stored := waitForConsumeLogByTokenName(t, "tk-claude")
	// quota = (700×1 + 200×0.5 + 60×1.25 + 40×2.0 + 500×3) × 2 = 2455×2 = 4910
	assert.Equal(t, 4910, stored.Quota)
	// 旧聚合列已删除：输入侧/输出总量从 billing_details 按唯一汇总公式还原
	assertStoredTokenTotals(t, stored, 1000, 500)

	require.NotNil(t, stored.BillingDetails)
	payload, err := ParseBillingDetailsJSON(*stored.BillingDetails)
	require.NoError(t, err)
	assert.Equal(t, 200, payload.Tokens.Cache.ReadCache)
	assert.Equal(t, 100, payload.Tokens.Cache.WriteCache)
	assert.Equal(t, 60, payload.Tokens.Cache.WriteCache5m)
	assert.Equal(t, 40, payload.Tokens.Cache.WriteCache1h)

	other, err := common.StrToMap(stored.Other)
	require.NoError(t, err)
	// 现有计费快照仍写入现有位置
	assert.NotContains(t, other, "cache_tokens")
	assert.NotContains(t, other, "cache_creation_tokens")
	assert.NotContains(t, other, "cache_creation_tokens_5m")
	assert.NotContains(t, other, "cache_creation_tokens_1h")
	assert.Equal(t, float64(2), other["model_ratio"])
}

// audio 入口：缓存读取按缓存单价计费（旧公式漏计，PRD 3.4 修正），
// 音频输入/输出按 AudioRatio 族差异化计价。
func TestPostAudioConsumeQuotaNormalizedFormula(t *testing.T) {
	setupApplyQuotaTestDB(t)
	ctx := newEntryTestContext("tk-audio")
	relayInfo := newEntryTestRelayInfo(relayconstant.UsageSourceOpenAIChat)

	usage := &shared.Usage{
		PromptTokens:     1000, // text(700) + audio(200) + cached(100)
		CompletionTokens: 500,  // text(400) + audio(100)
		TotalTokens:      1500,
	}
	usage.PromptTokensDetails.TextTokens = 700
	usage.PromptTokensDetails.AudioTokens = 200
	usage.PromptTokensDetails.CachedTokens = 100
	usage.CompletionTokenDetails.TextTokens = 400
	usage.CompletionTokenDetails.AudioTokens = 100

	require.Nil(t, PostAudioConsumeQuota(ctx, relayInfo, usage, ""))

	stored := waitForConsumeLogByTokenName(t, "tk-audio")
	// quota = (700×1 + 200×8 + 100×0.5 + 400×3 + 100×8×2) × 2 = 5150×2 = 10300
	assert.Equal(t, 10300, stored.Quota)
	assertStoredTokenTotals(t, stored, 1000, 500)

	require.NotNil(t, stored.BillingDetails)
	payload, err := ParseBillingDetailsJSON(*stored.BillingDetails)
	require.NoError(t, err)
	assert.Equal(t, 200, payload.Tokens.Input.AudioInput)
	assert.Equal(t, 100, payload.Tokens.Output.AudioOutput)
	assert.Equal(t, 100, payload.Tokens.Cache.ReadCache)

	other, err := common.StrToMap(stored.Other)
	require.NoError(t, err)
	assert.NotContains(t, other, "audio_input")
	assert.NotContains(t, other, "audio_output")
	assert.Equal(t, float64(8), other["audio_ratio"])
}

// audio 入口的本地字符数伪 usage（MiniMax TTS 风格）：quota 走归一化公式，
// billing_details 按本地计数标记跳过。
func TestPostAudioConsumeQuotaLocalCountSkipsBillingDetails(t *testing.T) {
	setupApplyQuotaTestDB(t)
	ctx := newEntryTestContext("tk-tts-local")
	httpapi.SetContextKey(ctx, common.ContextKeyLocalCountTokens, true)
	relayInfo := newEntryTestRelayInfo(relayconstant.UsageSourceOpenAIChat)

	usage := &shared.Usage{
		PromptTokens:     100,
		CompletionTokens: 100,
		TotalTokens:      200,
	}
	usage.PromptTokensDetails.TextTokens = 100
	usage.CompletionTokenDetails.AudioTokens = 100

	require.Nil(t, PostAudioConsumeQuota(ctx, relayInfo, usage, ""))

	stored := waitForConsumeLogByTokenName(t, "tk-tts-local")
	// quota = (100×1 + 100×8×2) × 2 = 3400
	assert.Equal(t, 3400, stored.Quota)
	require.Nil(t, stored.BillingDetails, "local-count pseudo usage must not write billing_details")
}

// realtime WSS 入口：quota 由归一化用量计算（含缓存读取），
// 与会话增量预扣公式保持同一线性口径。
func TestPostWssConsumeQuotaNormalizedFormula(t *testing.T) {
	setupApplyQuotaTestDB(t)
	ctx := newEntryTestContext("tk-wss")
	relayInfo := newEntryTestRelayInfo(relayconstant.UsageSourceOpenAIResponses)

	usage := &shared.RealtimeUsage{
		TotalTokens:  1500,
		InputTokens:  1000,
		OutputTokens: 500,
		InputTokenDetails: shared.InputTokenDetails{
			TextTokens:   700,
			AudioTokens:  200,
			CachedTokens: 100,
		},
		OutputTokenDetails: shared.OutputTokenDetails{
			TextTokens:  400,
			AudioTokens: 100,
		},
	}

	require.Nil(t, PostWssConsumeQuota(ctx, relayInfo, relayInfo.OriginModelName, usage, ""))

	stored := waitForConsumeLogByTokenName(t, "tk-wss")
	assert.Equal(t, 10300, stored.Quota)
	assertStoredTokenTotals(t, stored, 1000, 500)

	require.NotNil(t, stored.BillingDetails)
	payload, err := ParseBillingDetailsJSON(*stored.BillingDetails)
	require.NoError(t, err)
	assert.Equal(t, 100, payload.Tokens.Cache.ReadCache)
}

// 通用入口的 Gemini 音频输入（每百万美元单价）端到端：
// 音频输入差异化计价并写入独立费用快照。
func TestCalculateUsageGeminiAudioSeparatePriceSnapshot(t *testing.T) {
	setupApplyQuotaTestDB(t)
	ctx := newEntryTestContext("tk-gemini-audio")
	relayInfo := newEntryTestRelayInfo(relayconstant.UsageSourceGemini)
	relayInfo.OriginModelName = "gemini-2.5-flash"

	metadata := &shared.GeminiUsageMetadata{
		PromptTokenCount:        1000,
		CandidatesTokenCount:    500,
		CachedContentTokenCount: 200,
		PromptTokensDetails: []shared.GeminiPromptTokensDetails{
			{Modality: "AUDIO", TokenCount: 100},
		},
	}
	usage := GeminiUsageMetadataToOpenAIUsage(*metadata)
	relayInfo.UsageGeminiMetadata = metadata

	settlement, apiErr := CalculateUsage(ctx, relayInfo, &usage)
	require.Nil(t, apiErr)
	require.Nil(t, ApplyQuota(ctx, relayInfo, settlement))

	stored := waitForConsumeLogByTokenName(t, "tk-gemini-audio")
	// 普通输入 = (1000-200-100)=700；quota = (700×1 + 200×0.5 + 500×3) × 2 + 音频独立费 50 = 4600+50 = 4650
	assert.Equal(t, 4650, stored.Quota)

	other, err := common.StrToMap(stored.Other)
	require.NoError(t, err)
	assert.Equal(t, true, other["audio_input_seperate_price"])
	assert.NotContains(t, other, "audio_input_token_count")
	assert.Equal(t, float64(1.0), other["audio_input_price"])
}

func waitForConsumeLogByTokenName(t *testing.T, tokenName string) logstore.Log {
	t.Helper()
	var stored logstore.Log
	require.Eventually(t, func() bool {
		err := dbstore.LOG_DB.Where("user_id = ? AND type = ? AND token_name = ?",
			applyQuotaTestUserId, logstore.LogTypeConsume, tokenName).Order("id DESC").First(&stored).Error
		return err == nil
	}, 2*time.Second, 10*time.Millisecond, "consume log row should be written asynchronously")
	return stored
}

// assertStoredTokenTotals 按"billing_details 唯一权威来源"验收口径断言总量：
// 旧聚合列已删除，输入侧/输出总量只能从落库 JSON 按唯一汇总公式还原。
func assertStoredTokenTotals(t *testing.T, stored logstore.Log, wantInputSide int, wantOutput int) {
	t.Helper()
	require.NotNil(t, stored.BillingDetails, "token-bearing entry must persist billing_details")
	payload, err := ParseBillingDetailsJSON(*stored.BillingDetails)
	require.NoError(t, err)
	inputSide, err := payload.InputSideTotal()
	require.NoError(t, err)
	assert.Equal(t, wantInputSide, inputSide)
	output, err := payload.OutputTotal()
	require.NoError(t, err)
	assert.Equal(t, wantOutput, output)
}

// realtime 按事件增量预扣：归一化公式（含缓存读取）计算本轮增量额度并实扣，
// user/token 配额检查与 FinalPreConsumedQuota 累加语义保持不变。
func TestPreWssConsumeQuotaDeductsPerEventDelta(t *testing.T) {
	setupApplyQuotaTestDB(t)
	ctx := newEntryTestContext("tk-prewss")
	relayInfo := newEntryTestRelayInfo(relayconstant.UsageSourceOpenAIResponses)
	relayInfo.TokenUnlimited = true

	usage := &shared.RealtimeUsage{
		TotalTokens:  1500,
		InputTokens:  1000,
		OutputTokens: 500,
		InputTokenDetails: shared.InputTokenDetails{
			TextTokens:   700,
			AudioTokens:  200,
			CachedTokens: 100,
		},
		OutputTokenDetails: shared.OutputTokenDetails{
			TextTokens:  400,
			AudioTokens: 100,
		},
	}

	require.NoError(t, PreWssConsumeQuota(ctx, relayInfo, usage))
	// quota = (700×1 + 200×8 + 100×0.5 + 400×3 + 100×8×2) × 2 = 10300
	assert.Equal(t, 10300, relayInfo.FinalPreConsumedQuota, "pre-consumed quota must accumulate per-event delta")

	var user userstore.User
	require.NoError(t, dbstore.DB.First(&user, applyQuotaTestUserId).Error)
	assert.Equal(t, 1000000-10300, user.Quota, "wallet should be deducted by the per-event quota")

	// 令牌无限额度：只累计已用额度
	var token tokenstore.Token
	require.NoError(t, dbstore.DB.First(&token, applyQuotaTestTokenId).Error)
	assert.Equal(t, 10300, token.UsedQuota)
}

// 预扣配额不足：拒绝并返回错误，不产生扣款。
func TestPreWssConsumeQuotaRejectsWhenUserQuotaInsufficient(t *testing.T) {
	setupApplyQuotaTestDB(t)
	ctx := newEntryTestContext("tk-prewss-poor")
	relayInfo := newEntryTestRelayInfo(relayconstant.UsageSourceOpenAIResponses)
	relayInfo.TokenUnlimited = true
	// PreWssConsumeQuota 经 GetUserQuota 从数据库读取用户余额，直接压低余额
	require.NoError(t, dbstore.DB.Model(&userstore.User{}).Where("id = ?", applyQuotaTestUserId).
		Update("quota", 100).Error)

	usage := &shared.RealtimeUsage{
		TotalTokens:  1500,
		InputTokens:  1000,
		OutputTokens: 500,
		InputTokenDetails: shared.InputTokenDetails{
			TextTokens:  700,
			AudioTokens: 200,
		},
		OutputTokenDetails: shared.OutputTokenDetails{
			TextTokens:  400,
			AudioTokens: 100,
		},
	}

	err := PreWssConsumeQuota(ctx, relayInfo, usage)
	require.Error(t, err)
	assert.Equal(t, 0, relayInfo.FinalPreConsumedQuota)

	var user userstore.User
	require.NoError(t, dbstore.DB.First(&user, applyQuotaTestUserId).Error)
	assert.Equal(t, 100, user.Quota, "insufficient quota must not deduct wallet")
}
