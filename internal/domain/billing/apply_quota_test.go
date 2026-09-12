package billing

import (
	"fmt"
	"testing"
	"time"

	"github.com/NookMux/NookMux/internal/common"
	"github.com/NookMux/NookMux/internal/domain/shared"
	redis "github.com/NookMux/NookMux/internal/infra/redis"
	relaycommon "github.com/NookMux/NookMux/internal/relay/common"
	relayconstant "github.com/NookMux/NookMux/internal/relay/constant"
	channelstore "github.com/NookMux/NookMux/internal/store/channel"
	dbstore "github.com/NookMux/NookMux/internal/store/db"
	logstore "github.com/NookMux/NookMux/internal/store/log"
	pricingstore "github.com/NookMux/NookMux/internal/store/pricing"
	tokenstore "github.com/NookMux/NookMux/internal/store/token"
	userstore "github.com/NookMux/NookMux/internal/store/user"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// ApplyQuota 落账侧测试：CalculateUsage 的计算正确性由 usage_test.go 覆盖，
// 这里验证的是"算好的额度如何写库"——用户/渠道已用量、请求数、结算差额与
// 消费日志（RecordConsumeLog 的 13 个参数与 OtherInfo 组装）。
// 计费落账错一笔就是真实扣费错误，这些用例是 relay IR 重构再次改动
// ApplyQuota / PostConsumeQuota 前的防护网。

const (
	applyQuotaTestUserId    = 101
	applyQuotaTestChannelId = 7
	applyQuotaTestTokenId   = 3
	applyQuotaTestTokenKey  = "sk-apply-quota-test-key"
)

// setupApplyQuotaTestDB 复刻 internal/store/log/log_query_test.go 的 fixture
// 范式：独立内存 SQLite + 迁移所需表 + 全局量保存/恢复。
func setupApplyQuotaTestDB(t *testing.T) {
	t.Helper()

	oldDB := dbstore.DB
	oldLogDB := dbstore.LOG_DB
	oldMemoryCacheEnabled := common.MemoryCacheEnabled
	oldBatchUpdateEnabled := common.BatchUpdateEnabled
	oldLogConsumeEnabled := common.LogConsumeEnabled
	// 这些全局会被 DecreaseUserQuota / RecordConsumeLog 的异步池任务并发
	// 读取；即使写入相同值也是数据竞争（-race 实跑可证），因此只在取值
	// 不同时写入。redis.RedisEnabled 的包初始值为 true（InitRedisClient 不
	// 会在测试二进制中运行），fixture 置 false 后保持、不在 cleanup 写回：
	// 恢复写会与迟到的异步池任务构成竞争，且本包测试二进制内没有依赖
	// true 的用例。
	setGlobalIfChanged := func(current *bool, want bool) {
		if *current != want {
			*current = want
		}
	}

	testDB, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite test db: %v", err)
	}
	if err := testDB.AutoMigrate(&userstore.User{}, &channelstore.Channel{}, &tokenstore.Token{}, &logstore.Log{}, &pricingstore.ModelPricePlan{}, &pricingstore.ModelPriceComponent{}); err != nil {
		t.Fatalf("migrate sqlite test db: %v", err)
	}
	pricingstore.InvalidateModelPricePlanCache()
	dbstore.DB = testDB
	dbstore.LOG_DB = testDB
	setGlobalIfChanged(&redis.RedisEnabled, false)
	setGlobalIfChanged(&common.MemoryCacheEnabled, false)
	// 批量更新模式下计数器只进内存暂存，测试统一走同步直写路径。
	setGlobalIfChanged(&common.BatchUpdateEnabled, false)
	setGlobalIfChanged(&common.LogConsumeEnabled, true)

	if err := testDB.Create(&userstore.User{
		Id:       applyQuotaTestUserId,
		Username: "apply-quota-tester",
		Quota:    1000000,
	}).Error; err != nil {
		t.Fatalf("seed user: %v", err)
	}
	if err := testDB.Create(&channelstore.Channel{
		Id:   applyQuotaTestChannelId,
		Name: "apply-quota-channel",
	}).Error; err != nil {
		t.Fatalf("seed channel: %v", err)
	}
	if err := testDB.Create(&tokenstore.Token{
		Id:     applyQuotaTestTokenId,
		UserId: applyQuotaTestUserId,
		Key:    applyQuotaTestTokenKey,
	}).Error; err != nil {
		t.Fatalf("seed token: %v", err)
	}

	t.Cleanup(func() {
		if sqlDB, err := testDB.DB(); err == nil {
			_ = sqlDB.Close()
		}
		dbstore.DB = oldDB
		dbstore.LOG_DB = oldLogDB
		setGlobalIfChanged(&common.MemoryCacheEnabled, oldMemoryCacheEnabled)
		setGlobalIfChanged(&common.BatchUpdateEnabled, oldBatchUpdateEnabled)
		setGlobalIfChanged(&common.LogConsumeEnabled, oldLogConsumeEnabled)
	})
}

func newApplyQuotaTestContext() *gin.Context {
	c := newUsageTestContext()
	c.Set("username", "apply-quota-tester")
	c.Set("token_name", "tk-main")
	c.Set("use_channel", []string{"7"})
	return c
}

func newApplyQuotaTestRelayInfo() *relaycommon.RelayInfo {
	relayInfo := newUsageTestRelayInfo()
	// ChannelMeta 以指针内嵌且默认 nil，未初始化时访问 ChannelId 等提升字段会 panic
	relayInfo.ChannelMeta = &relaycommon.ChannelMeta{ChannelId: applyQuotaTestChannelId}
	relayInfo.UserId = applyQuotaTestUserId
	relayInfo.TokenId = applyQuotaTestTokenId
	relayInfo.TokenKey = applyQuotaTestTokenKey
	relayInfo.TokenQuotaType = 0
	relayInfo.UsingGroup = "default"
	relayInfo.UserQuota = 1000000
	// 倒退 2 秒让 useTimeMs 落在可断言的区间内（CalculateUsage 用真实时钟取耗时）
	relayInfo.StartTime = time.Now().Add(-2 * time.Second)
	return relayInfo
}

func newApplyQuotaTestUsage() *shared.Usage {
	usage := &shared.Usage{PromptTokens: 100, CompletionTokens: 50, TotalTokens: 150}
	usage.PromptTokensDetails.CachedTokens = 40
	return usage
}

// waitForConsumeLog 轮询消费日志：RecordConsumeLog 经 RelayGo 异步落库。
func waitForConsumeLog(t *testing.T, userId int) logstore.Log {
	t.Helper()
	var stored logstore.Log
	require.Eventually(t, func() bool {
		err := dbstore.LOG_DB.Where("user_id = ? AND type = ?", userId, logstore.LogTypeConsume).
			Order("id DESC").First(&stored).Error
		return err == nil
	}, 2*time.Second, 10*time.Millisecond, "consume log row should be written asynchronously")
	return stored
}

// 常规路径（CalculateUsage 端到端）：已用量/请求数/渠道用量同步累加，
// 结算差额为零，消费日志 13 个参数逐一落列。
func TestApplyQuotaUpdatesCountersAndWritesConsumeLog(t *testing.T) {
	setupApplyQuotaTestDB(t)
	ctx := newApplyQuotaTestContext()
	relayInfo := newApplyQuotaTestRelayInfo()
	usage := newApplyQuotaTestUsage()

	settlement, apiErr := CalculateUsage(ctx, relayInfo, usage, "耗时 0.2 秒")
	require.Nil(t, apiErr)
	// (100-40) + 40*0.5 + 50*3 = 230；230 * 2 * 1 = 460
	require.Equal(t, 460, settlement.quota)
	// 预扣 = 实扣，结算零差额，不再动用户剩余额度
	relayInfo.FinalPreConsumedQuota = settlement.quota

	apiErr = ApplyQuota(ctx, relayInfo, settlement)
	require.Nil(t, apiErr)

	var user userstore.User
	require.NoError(t, dbstore.DB.First(&user, applyQuotaTestUserId).Error)
	assert.Equal(t, 460, user.UsedQuota, "user used_quota should accumulate settlement quota")
	assert.Equal(t, 1, user.RequestCount, "user request_count should increment by 1")
	assert.Equal(t, 1000000, user.Quota, "zero settle delta must not touch remaining quota")

	var channel channelstore.Channel
	require.NoError(t, dbstore.DB.First(&channel, applyQuotaTestChannelId).Error)
	assert.Equal(t, int64(460), channel.UsedQuota, "channel used_quota should accumulate settlement quota")

	stored := waitForConsumeLog(t, applyQuotaTestUserId)
	assert.Equal(t, applyQuotaTestUserId, stored.UserId)
	assert.Equal(t, "apply-quota-tester", stored.Username)
	assert.Equal(t, logstore.LogTypeConsume, stored.Type)
	assert.Equal(t, applyQuotaTestChannelId, stored.ChannelId)
	assert.Equal(t, applyQuotaTestTokenId, stored.TokenId)
	assert.Equal(t, "gpt-4o", stored.ModelName)
	assert.Equal(t, "tk-main", stored.TokenName)
	assert.Equal(t, 460, stored.Quota)
	// 旧聚合列已删除：输入侧/输出总量从 billing_details 按唯一汇总公式还原
	assertStoredTokenTotals(t, stored, 100, 50)
	// 起点回拨 2 秒：UseTime 落库值应在 [1000, 4000] 区间（真实时钟，防抖动）
	assert.GreaterOrEqual(t, stored.UseTime, 1000, "use_time should flow from settlement.useTimeMs")
	assert.LessOrEqual(t, stored.UseTime, 4000)
	assert.False(t, stored.IsStream)
	assert.Equal(t, "default", stored.Group)
	assert.Equal(t, "耗时 0.2 秒", stored.Content)

	other, err := common.StrToMap(stored.Other)
	require.NoError(t, err)
	assert.Equal(t, float64(2), other["model_ratio"])
	assert.Equal(t, float64(1), other["group_ratio"])
	assert.Equal(t, float64(3), other["completion_ratio"])
	assert.NotContains(t, other, "cache_tokens")
	assert.Equal(t, "/v1/chat/completions", other["request_path"])
	snapshot, ok := other["billing_price_snapshot"].(map[string]interface{})
	require.True(t, ok, "stage 4 price snapshot should be written to Other")
	assert.Equal(t, "legacy", snapshot["source"])
	assert.Equal(t, "token", snapshot["billing_mode"])
	components, ok := snapshot["components"].([]interface{})
	require.True(t, ok)
	require.Len(t, components, 3)
	componentByComponent := make(map[string]map[string]interface{}, len(components))
	for _, raw := range components {
		component, ok := raw.(map[string]interface{})
		require.True(t, ok)
		name, ok := component["component"].(string)
		require.True(t, ok)
		componentByComponent[name] = component
	}
	for name, price := range map[string]string{
		"text_input":  "4",
		"text_output": "12",
		"cache_read":  "2",
	} {
		component, ok := componentByComponent[name]
		require.True(t, ok, "missing %s price component", name)
		assert.Equal(t, price, component["unit_price"])
		assert.Equal(t, "1", component["group_multiplier"])
	}
	assert.Equal(t, float64(60), componentByComponent["text_input"]["quantity"])
	assert.Equal(t, float64(50), componentByComponent["text_output"]["quantity"])
	assert.Equal(t, float64(40), componentByComponent["cache_read"]["quantity"])
	adminInfo, ok := other["admin_info"].(map[string]interface{})
	require.True(t, ok, "admin_info should be assembled into Other")
	assert.Equal(t, []interface{}{"7"}, adminInfo["use_channel"])
}

// total tokens 为 0 且无工具费：不落已用量/请求数，但消费日志仍必须记录。
func TestApplyQuotaTotalTokensZeroWithoutToolFeesSkipsCounters(t *testing.T) {
	setupApplyQuotaTestDB(t)
	ctx := newApplyQuotaTestContext()
	relayInfo := newApplyQuotaTestRelayInfo()
	relayInfo.FinalPreConsumedQuota = 0

	settlement := &UsageSettlement{
		modelName: "gpt-4o", tokenName: "tk-main",
		promptTokens: 0, completionTokens: 0,
		totalTokensZero: true,
	}

	apiErr := ApplyQuota(ctx, relayInfo, settlement)
	require.Nil(t, apiErr)

	var user userstore.User
	require.NoError(t, dbstore.DB.First(&user, applyQuotaTestUserId).Error)
	assert.Zero(t, user.UsedQuota, "no tool fees: used_quota must stay untouched")
	assert.Zero(t, user.RequestCount, "no tool fees: request_count must stay untouched")

	var channel channelstore.Channel
	require.NoError(t, dbstore.DB.First(&channel, applyQuotaTestChannelId).Error)
	assert.Zero(t, channel.UsedQuota)

	stored := waitForConsumeLog(t, applyQuotaTestUserId)
	assert.Equal(t, logstore.LogTypeConsume, stored.Type)
	assert.Zero(t, stored.Quota)
}

// total tokens 为 0 但有工具费：仍要按工具费额度落用户/渠道已用量。
func TestApplyQuotaTotalTokensZeroWithToolFeesStillUpdatesCounters(t *testing.T) {
	setupApplyQuotaTestDB(t)
	ctx := newApplyQuotaTestContext()
	relayInfo := newApplyQuotaTestRelayInfo()

	settlement := &UsageSettlement{
		modelName: "gpt-4o", tokenName: "tk-main",
		quota:           77,
		totalTokensZero: true, hasToolFees: true,
	}
	relayInfo.FinalPreConsumedQuota = settlement.quota

	apiErr := ApplyQuota(ctx, relayInfo, settlement)
	require.Nil(t, apiErr)

	var user userstore.User
	require.NoError(t, dbstore.DB.First(&user, applyQuotaTestUserId).Error)
	assert.Equal(t, 77, user.UsedQuota, "tool fees: used_quota must still be recorded")
	assert.Equal(t, 1, user.RequestCount)

	var channel channelstore.Channel
	require.NoError(t, dbstore.DB.First(&channel, applyQuotaTestChannelId).Error)
	assert.Equal(t, int64(77), channel.UsedQuota)

	stored := waitForConsumeLog(t, applyQuotaTestUserId)
	assert.Equal(t, 77, stored.Quota)
}

// OtherInfo 组装：动态倍率下原始分组倍率 = 当前倍率 / 动态倍率；
	// 缓存创建倍率、拒付原因、gizmo 模型名改写都要进入日志；Token 明细只能
	// 来自调用方生成的 billing_details，不能从 settlement 隐式反推。
// （阶段 2 删除了请求侧语义打标 other["claude"]/other["usage_semantic"]，
// usage 语义由 UsageSource 与 billing_details 承载。）
func TestApplyQuotaOtherInfoAssembly(t *testing.T) {
	setupApplyQuotaTestDB(t)
	ctx := newApplyQuotaTestContext()
	relayInfo := newApplyQuotaTestRelayInfo()
	relayInfo.PriceData.GroupRatioInfo.DynamicRatio = 0.5

	settlement := &UsageSettlement{
		modelName: "gpt-4o-gizmo-preview-0414", tokenName: "tk-main",
		quota: 30, useTimeMs: 50,
		modelRatio: 2, completionRatio: 3, cacheRatio: 0.5, groupRatio: 2,
		// ratio 置 0 以区分守卫条件：写 cache_creation_* 键的依据必须是 tokens != 0
		cachedCreationTokens: 12, cachedCreationRatio: 0,
		adminRejectReason: "risk_control",
	}
	relayInfo.FinalPreConsumedQuota = settlement.quota

	apiErr := ApplyQuota(ctx, relayInfo, settlement)
	require.Nil(t, apiErr)

	stored := waitForConsumeLog(t, applyQuotaTestUserId)
	// gizmo 前缀模型名在日志里改写为通配，原模型名追加进 Content
	assert.Equal(t, "gpt-4o-gizmo-*", stored.ModelName)
	assert.Contains(t, stored.Content, "模型 gpt-4o-gizmo-preview-0414")

	other, err := common.StrToMap(stored.Other)
	require.NoError(t, err)
	// originalGroupRatio = groupRatio / dynamicRatio = 2 / 0.5 = 4
	assert.Equal(t, float64(4), other["group_ratio"], "group_ratio must be the pre-dynamic original ratio")
	assert.Equal(t, float64(0.5), other["dynamic_ratio"])
	assert.NotContains(t, other, "usage_semantic", "usage semantic param removed in stage 2")
	assert.NotContains(t, other, "cache_creation_tokens")
	assert.Equal(t, float64(0), other["cache_creation_ratio"], "ratio 为 0 时键也必须写入（守卫看 tokens 而非 ratio）")
	assert.Nil(t, stored.BillingDetails, "synthetic settlement without normalized usage must not create token details")
	assert.Equal(t, "risk_control", other["reject_reason"])
	assert.Equal(t, float64(2), other["model_ratio"])
	assert.Equal(t, float64(3), other["completion_ratio"])
}

// 结算差额路径：实际消耗高于预扣时，差额经 PostConsumeQuota 扣减用户剩余额度
// 并累计令牌已用额度（无限额度令牌只记已用）。
func TestApplyQuotaSettlesDeltaViaPostConsumeQuota(t *testing.T) {
	setupApplyQuotaTestDB(t)
	ctx := newApplyQuotaTestContext()
	relayInfo := newApplyQuotaTestRelayInfo()

	settlement := &UsageSettlement{
		modelName: "gpt-4o", tokenName: "tk-main",
		quota: 460, promptTokens: 100, completionTokens: 50,
	}
	// 预扣 100，实扣 460 → 结算差额 360 从用户剩余额度补扣
	relayInfo.FinalPreConsumedQuota = 100

	apiErr := ApplyQuota(ctx, relayInfo, settlement)
	require.Nil(t, apiErr)

	var user userstore.User
	require.NoError(t, dbstore.DB.First(&user, applyQuotaTestUserId).Error)
	assert.Equal(t, 1000000-360, user.Quota, "settle delta should decrease remaining quota")
	assert.Equal(t, 460, user.UsedQuota)

	var token tokenstore.Token
	require.NoError(t, dbstore.DB.First(&token, applyQuotaTestTokenId).Error)
	assert.Equal(t, 360, token.UsedQuota, "unlimited token still tracks used quota by delta")

	stored := waitForConsumeLog(t, applyQuotaTestUserId)
	assert.Equal(t, 460, stored.Quota)
}

// 生产主路径：PreConsumeBilling 创建 BillingSession 后，ApplyQuota 经
// SettleBilling 走 BillingSession.Settle——预扣 100、实扣 460 → 补扣 360，
// 用户钱包两次扣减、无限额度令牌累计已用 100+360。
func TestApplyQuotaSettlesViaBillingSession(t *testing.T) {
	setupApplyQuotaTestDB(t)
	ctx := newApplyQuotaTestContext()
	relayInfo := newApplyQuotaTestRelayInfo()

	// 用户额度 100 万 < 信任阈值（10*QuotaPerUnit=500 万），必然真实预扣
	apiErr := PreConsumeBilling(ctx, 100, relayInfo)
	require.Nil(t, apiErr)
	require.NotNil(t, relayInfo.Billing)
	assert.Equal(t, 100, relayInfo.FinalPreConsumedQuota)

	var user userstore.User
	require.NoError(t, dbstore.DB.First(&user, applyQuotaTestUserId).Error)
	assert.Equal(t, 1000000-100, user.Quota, "pre-consume should deduct from wallet")

	settlement := &UsageSettlement{
		modelName: "gpt-4o", tokenName: "tk-main",
		quota: 460, promptTokens: 100, completionTokens: 50,
	}

	apiErr = ApplyQuota(ctx, relayInfo, settlement)
	require.Nil(t, apiErr)

	require.NoError(t, dbstore.DB.First(&user, applyQuotaTestUserId).Error)
	assert.Equal(t, 1000000-100-360, user.Quota, "settle should top up the 360 delta via BillingSession")
	assert.Equal(t, 460, user.UsedQuota)

	var token tokenstore.Token
	require.NoError(t, dbstore.DB.First(&token, applyQuotaTestTokenId).Error)
	assert.Equal(t, 460, token.UsedQuota, "unlimited token: pre-consume 100 + settle delta 360")

	var channel channelstore.Channel
	require.NoError(t, dbstore.DB.First(&channel, applyQuotaTestChannelId).Error)
	assert.Equal(t, int64(460), channel.UsedQuota)

	stored := waitForConsumeLog(t, applyQuotaTestUserId)
	assert.Equal(t, 460, stored.Quota)
	other, err := common.StrToMap(stored.Other)
	require.NoError(t, err)
	assert.Equal(t, "wallet", other["billing_source"], "billing source should flow into log OtherInfo")
}

// 阶段 1 计费 PRD：携带上游真实 Token 用量的消费日志写 billing_details
// canonical JSON；上游无计费信息（rawUsage == nil，本地估算）不写该列。
func TestApplyQuotaBillingDetailsWrittenOnlyForUpstreamUsage(t *testing.T) {
	setupApplyQuotaTestDB(t)

	waitForConsumeLogByToken := func(t *testing.T, tokenName string) logstore.Log {
		t.Helper()
		var stored logstore.Log
		require.Eventually(t, func() bool {
			err := dbstore.LOG_DB.Where("user_id = ? AND type = ? AND token_name = ?",
				applyQuotaTestUserId, logstore.LogTypeConsume, tokenName).Order("id DESC").First(&stored).Error
			return err == nil
		}, 2*time.Second, 10*time.Millisecond, "consume log row should be written asynchronously")
		return stored
	}

	t.Run("real upstream usage writes parseable json", func(t *testing.T) {
		ctx := newApplyQuotaTestContext()
		ctx.Set("token_name", "tk-bd-real")
		relayInfo := newApplyQuotaTestRelayInfo()
		relayInfo.UsageSource = relayconstant.UsageSourceOpenAIChat
		usage := newApplyQuotaTestUsage()

		settlement, apiErr := CalculateUsage(ctx, relayInfo, usage)
		require.Nil(t, apiErr)
		require.Nil(t, ApplyQuota(ctx, relayInfo, settlement))

		stored := waitForConsumeLogByToken(t, "tk-bd-real")
		require.NotNil(t, stored.BillingDetails, "upstream usage must persist billing_details")
		payload, err := ParseBillingDetailsJSON(*stored.BillingDetails)
		require.NoError(t, err)
		require.Equal(t, 40, payload.Tokens.Cache.ReadCache)
	})

	t.Run("estimated usage keeps billing_details null", func(t *testing.T) {
		ctx := newApplyQuotaTestContext()
		ctx.Set("token_name", "tk-bd-estimate")
		relayInfo := newApplyQuotaTestRelayInfo()
		relayInfo.UsageSource = relayconstant.UsageSourceOpenAIChat
		// rawUsage == nil：usage 来自本地估算（GetEstimatePromptTokens），
		// settlement.estimatedUsage 置位，billing_details 不落列。
		relayInfo.SetEstimatePromptTokens(80)

		settlement, apiErr := CalculateUsage(ctx, relayInfo, nil)
		require.Nil(t, apiErr)
		require.True(t, settlement.estimatedUsage, "nil rawUsage must mark estimated usage")
		require.Nil(t, ApplyQuota(ctx, relayInfo, settlement))

		stored := waitForConsumeLogByToken(t, "tk-bd-estimate")
		require.Nil(t, stored.BillingDetails, "estimated usage must not write billing_details")
	})
}
