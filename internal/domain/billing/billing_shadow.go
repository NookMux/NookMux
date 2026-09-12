package billing

import (
	"fmt"
	"math"
	"strings"

	"github.com/NookMux/NookMux/internal/common"
	"github.com/NookMux/NookMux/internal/config/operation"
	"github.com/NookMux/NookMux/internal/config/ratio"
	"github.com/NookMux/NookMux/internal/domain/billing/contract"
	"github.com/NookMux/NookMux/internal/domain/shared"
	"github.com/NookMux/NookMux/internal/infra/log"
	relaycommon "github.com/NookMux/NookMux/internal/relay/common"
	relayconstant "github.com/NookMux/NookMux/internal/relay/constant"
	"github.com/gin-gonic/gin"
	"github.com/shopspring/decimal"
)

// 计费 PRD 阶段 2 迁移期影子对拍：旧公式与新公式（CalculateNormalizedQuota）
// 在同一请求上并行计算，quota 不一致时输出告警日志。
//
// PRD 要求：上线前保留旧公式影子对拍，确认迁移期间 quota 相同；差异必须先
// 定位为旧公式 bug 或新语义 bug，不允许直接吸收差异。已知预期差异类别在
// classifyShadowDiff 中标注，未标注的差异需要人工定位后在此补充分类。
//
// 迁移确认完成后整文件删除（含 QuotaInfo/legacyAudioQuota 等旧公式）。

// QuotaInfo 旧 audio/realtime 计费入参（影子对拍基线专用）。
type QuotaInfo struct {
	InputDetails         TokenDetails
	OutputDetails        TokenDetails
	ModelName            string
	UsePrice             bool
	ModelPrice           float64
	ModelRatio           float64
	GroupRatio           float64
	CompletionRatio      float64
	AudioRatio           float64
	AudioCompletionRatio float64
}

type TokenDetails struct {
	TextTokens  int
	AudioTokens int
}

// legacyGenericQuota 复刻旧 CalculateUsage 的通用公式（聚合字段 + 语义开关），
// 返回 token 部分费用（含旧最低消费规则）与音频输入独立费用。
func legacyGenericQuota(usage *shared.Usage, claudeSemantic bool, priceData contract.PriceData, modelName string) (decimal.Decimal, decimal.Decimal) {
	if priceData.UsePrice {
		return decimal.NewFromFloat(priceData.ModelPrice).
			Mul(decimal.NewFromFloat(common.QuotaPerUnit)).
			Mul(decimal.NewFromFloat(priceData.GroupRatioInfo.GroupRatio)), decimal.Zero
	}

	dPromptTokens := decimal.NewFromInt(int64(usage.PromptTokens))
	dCacheTokens := decimal.NewFromInt(int64(usage.PromptTokensDetails.CachedTokens))
	dAudioTokens := decimal.NewFromInt(int64(usage.PromptTokensDetails.AudioTokens))
	dCompletionTokens := decimal.NewFromInt(int64(usage.CompletionTokens))
	dCachedCreationTokens := decimal.NewFromInt(int64(usage.PromptTokensDetails.CachedCreationTokens))
	dCompletionRatio := decimal.NewFromFloat(priceData.CompletionRatio)
	dCacheRatio := decimal.NewFromFloat(priceData.CacheRatio)
	dModelRatio := decimal.NewFromFloat(priceData.ModelRatio)
	dGroupRatio := decimal.NewFromFloat(priceData.GroupRatioInfo.GroupRatio)
	dCachedCreationRatio := decimal.NewFromFloat(priceData.CacheCreationRatio)
	dQuotaPerUnit := decimal.NewFromFloat(common.QuotaPerUnit)
	ratio := dModelRatio.Mul(dGroupRatio)

	baseTokens := dPromptTokens
	var cachedTokensWithRatio decimal.Decimal
	if !dCacheTokens.IsZero() {
		if !claudeSemantic {
			baseTokens = baseTokens.Sub(dCacheTokens)
		}
		cachedTokensWithRatio = dCacheTokens.Mul(dCacheRatio)
	}
	var dCachedCreationTokensWithRatio decimal.Decimal
	if !dCachedCreationTokens.IsZero() {
		if !claudeSemantic {
			baseTokens = baseTokens.Sub(dCachedCreationTokens)
		}
		dCachedCreationTokensWithRatio = dCachedCreationTokens.Mul(dCachedCreationRatio)
	}

	// 旧公式的 Gemini 音频输入独立计费（绝对单价，不乘模型倍率）。
	var audioInputQuota decimal.Decimal
	if !dAudioTokens.IsZero() {
		if audioInputPrice := operation.GetGeminiInputAudioPricePerMillionTokens(modelName); audioInputPrice > 0 {
			baseTokens = baseTokens.Sub(dAudioTokens)
			audioInputQuota = decimal.NewFromFloat(audioInputPrice).Div(decimal.NewFromInt(1000000)).
				Mul(dAudioTokens).Mul(dGroupRatio).Mul(dQuotaPerUnit)
		}
	}

	promptQuota := baseTokens.Add(cachedTokensWithRatio).Add(dCachedCreationTokensWithRatio)
	completionQuota := dCompletionTokens.Mul(dCompletionRatio)
	quotaCalculateDecimal := promptQuota.Add(completionQuota).Mul(ratio)
	if !ratio.IsZero() && quotaCalculateDecimal.LessThanOrEqual(decimal.Zero) {
		quotaCalculateDecimal = decimal.NewFromInt(1)
	}
	return quotaCalculateDecimal, audioInputQuota
}

// legacyGenericFinalQuota 复刻旧 CalculateUsage 的完整取整流程：
// token 部分（含最低消费规则）+ 音频独立费 + 工具费 → 其他倍率 → 取整。
func legacyGenericFinalQuota(usage *shared.Usage, claudeSemantic bool, priceData contract.PriceData, modelName string, extraQuota decimal.Decimal, otherRatios map[string]float64) int {
	tokenTotal, audioInputQuota := legacyGenericQuota(usage, claudeSemantic, priceData, modelName)
	total := tokenTotal.Add(audioInputQuota).Add(extraQuota)
	for _, otherRatio := range otherRatios {
		total = total.Mul(decimal.NewFromFloat(otherRatio))
	}
	quota := int(total.Round(0).IntPart())
	ratio := decimal.NewFromFloat(priceData.ModelRatio).Mul(decimal.NewFromFloat(priceData.GroupRatioInfo.GroupRatio))
	if !priceData.UsePrice && !ratio.IsZero() && quota == 0 {
		quota = 1
	}
	return quota
}

func hasCustomModelRatio(modelName string, currentRatio float64) bool {
	defaultRatio, exists := ratio.GetDefaultModelRatioMap()[modelName]
	if !exists {
		return true
	}
	return currentRatio != defaultRatio
}

// legacyClaudeQuota 复刻旧 PostClaudeConsumeQuota 公式：原生 Claude 对聚合
// PromptTokens 按 1 倍计费并叠加缓存读写单价；OpenRouter 先扣减缓存。
// 保留 float 口径与截断取整，用于影子对拍基线。
func legacyClaudeQuota(usage *shared.Usage, openRouter bool, priceData contract.PriceData, modelName string) int {
	promptTokens := usage.PromptTokens
	completionTokens := usage.CompletionTokens
	cacheTokens := usage.PromptTokensDetails.CachedTokens
	cacheCreationTokens := usage.PromptTokensDetails.CachedCreationTokens
	cacheCreationTokens5m := usage.ClaudeCacheCreation5mTokens
	cacheCreationTokens1h := usage.ClaudeCacheCreation1hTokens

	if openRouter {
		promptTokens -= cacheTokens
		isUsingCustomSettings := priceData.UsePrice || hasCustomModelRatio(modelName, priceData.ModelRatio)
		if cacheCreationTokens == 0 && priceData.CacheCreationRatio != 1 && usage.Cost != 0 && !isUsingCustomSettings {
			maybeCacheCreationTokens := CalcOpenRouterCacheCreateTokens(*usage, priceData)
			if maybeCacheCreationTokens >= 0 && promptTokens >= maybeCacheCreationTokens {
				cacheCreationTokens = maybeCacheCreationTokens
			}
		}
		promptTokens -= cacheCreationTokens
	}

	calculateQuota := 0.0
	if !priceData.UsePrice {
		calculateQuota = float64(promptTokens)
		calculateQuota += float64(cacheTokens) * priceData.CacheRatio
		calculateQuota += float64(cacheCreationTokens5m) * priceData.CacheCreation5mRatio
		calculateQuota += float64(cacheCreationTokens1h) * priceData.CacheCreation1hRatio
		remainingCacheCreationTokens := cacheCreationTokens - cacheCreationTokens5m - cacheCreationTokens1h
		if remainingCacheCreationTokens > 0 {
			calculateQuota += float64(remainingCacheCreationTokens) * priceData.CacheCreationRatio
		}
		calculateQuota += float64(completionTokens) * priceData.CompletionRatio
		calculateQuota = calculateQuota * priceData.GroupRatioInfo.GroupRatio * priceData.ModelRatio
	} else {
		calculateQuota = priceData.ModelPrice * common.QuotaPerUnit * priceData.GroupRatioInfo.GroupRatio
	}

	if priceData.ModelRatio != 0 && calculateQuota <= 0 {
		calculateQuota = 1
	}
	return int(calculateQuota)
}

// CalcOpenRouterCacheCreateTokens 旧 OpenRouter 专属逻辑：用上游返回的 cost
// 反推缓存写入 tokens。阶段 2 已从正式计费路径删除（PRD 指令），仅影子对拍
// 保留以复刻旧公式基线。
func CalcOpenRouterCacheCreateTokens(usage shared.Usage, priceData contract.PriceData) int {
	if priceData.CacheCreationRatio == 1 {
		return 0
	}
	quotaPrice := priceData.ModelRatio / common.QuotaPerUnit
	promptCacheCreatePrice := quotaPrice * priceData.CacheCreationRatio
	promptCacheReadPrice := quotaPrice * priceData.CacheRatio
	completionPrice := quotaPrice * priceData.CompletionRatio

	cost, _ := usage.Cost.(float64)
	totalPromptTokens := float64(usage.PromptTokens)
	completionTokens := float64(usage.CompletionTokens)
	promptCacheReadTokens := float64(usage.PromptTokensDetails.CachedTokens)

	return int(math.Round((cost -
		totalPromptTokens*quotaPrice +
		promptCacheReadTokens*(quotaPrice-promptCacheReadPrice) -
		completionTokens*completionPrice) /
		(promptCacheCreatePrice - quotaPrice)))
}

// legacyAudioQuota 复刻旧 calculateAudioQuota：text/audio 明细 + 音频倍率。
func legacyAudioQuota(info QuotaInfo) int {
	if info.UsePrice {
		modelPrice := decimal.NewFromFloat(info.ModelPrice)
		quotaPerUnit := decimal.NewFromFloat(common.QuotaPerUnit)
		groupRatio := decimal.NewFromFloat(info.GroupRatio)

		quota := modelPrice.Mul(quotaPerUnit).Mul(groupRatio)
		return int(quota.IntPart())
	}

	completionRatio := decimal.NewFromFloat(info.CompletionRatio)
	audioRatio := decimal.NewFromFloat(info.AudioRatio)
	audioCompletionRatio := decimal.NewFromFloat(info.AudioCompletionRatio)

	groupRatio := decimal.NewFromFloat(info.GroupRatio)
	modelRatio := decimal.NewFromFloat(info.ModelRatio)
	ratio := groupRatio.Mul(modelRatio)

	inputTextTokens := decimal.NewFromInt(int64(info.InputDetails.TextTokens))
	outputTextTokens := decimal.NewFromInt(int64(info.OutputDetails.TextTokens))
	inputAudioTokens := decimal.NewFromInt(int64(info.InputDetails.AudioTokens))
	outputAudioTokens := decimal.NewFromInt(int64(info.OutputDetails.AudioTokens))

	quota := decimal.Zero
	quota = quota.Add(inputTextTokens)
	quota = quota.Add(outputTextTokens.Mul(completionRatio))
	quota = quota.Add(inputAudioTokens.Mul(audioRatio))
	quota = quota.Add(outputAudioTokens.Mul(audioRatio).Mul(audioCompletionRatio))

	quota = quota.Mul(ratio)

	// If ratio is not zero and quota is less than or equal to zero, set quota to 1
	if !ratio.IsZero() && quota.LessThanOrEqual(decimal.Zero) {
		quota = decimal.NewFromInt(1)
	}

	return int(quota.Round(0).IntPart())
}

// reportBillingShadowMismatch 输出影子对拍差异告警。差异必须先定位为旧公式
// bug 或新语义 bug（PRD 阶段 2）；已知预期差异类别在此标注，其余需要人工
// 定位后补充分类，不允许静默吸收。
func reportBillingShadowMismatch(ctx *gin.Context, entry string, relayInfo *relaycommon.RelayInfo, legacyQuota int, normalizedQuota int, bu *BillingUsage, lines []BillingQuotaLine, openRouter bool, snapshot *BillingPriceSnapshot) {
	tokenLines := make([]string, 0, len(lines))
	for _, line := range lines {
		tokenLines = append(tokenLines, fmt.Sprintf("%s:%d", line.Label, line.Tokens))
	}
	log.LogWarn(ctx, fmt.Sprintf(
		"billing shadow mismatch (entry=%s, model=%s, source=%s): legacy_quota=%d normalized_quota=%d, tokens=[%s], hint=%s",
		entry, relayInfo.OriginModelName, relayInfo.UsageSource, legacyQuota, normalizedQuota,
		strings.Join(tokenLines, ", "), classifyShadowDiff(entry, relayInfo, bu, openRouter, snapshot)))
}

func classifyShadowDiff(entry string, relayInfo *relaycommon.RelayInfo, bu *BillingUsage, openRouter bool, snapshot *BillingPriceSnapshot) string {
	var hints []string
	if snapshot != nil && normalizedPricePlanSource(snapshot.Source) == contract.PricePlanSourceExplicit {
		// 显式价格表拥有独立定价权（含 0 价格计划），与旧 ratio 公式天然不一致。
		hints = append(hints, "显式价格表结算与旧 ratio 公式独立定价（预期差异）")
	}
	if bu != nil {
		switch {
		case entry == "claude" && openRouter && (bu.CacheReadTokens > 0 || bu.CacheWriteTokens > 0):
			hints = append(hints, "OpenRouter cost 反推缓存写入减法已按 PRD 删除（预期差异）")
		case entry == "claude" && (bu.CacheReadTokens > 0 || bu.CacheWriteTokens > 0):
			hints = append(hints, "旧公式对原生 Claude 缓存读写按基础价重复计费，新公式按 PRD 3.4 修正（预期差异）")
		case (entry == "audio" || entry == "wss") && (bu.CacheReadTokens > 0 || bu.CacheWriteTokens > 0):
			hints = append(hints, "旧公式完全无缓存项（读写都漏），新公式按 PRD 3.4 计入缓存单价（预期差异）")
		case entry == "generic" && relayInfo != nil &&
			relayInfo.FinalRequestRelayFormat == relayconstant.RelayFormatClaude && (bu.CacheReadTokens > 0 || bu.CacheWriteTokens > 0):
			hints = append(hints, "旧公式按请求格式跳过缓存扣减，新公式按 usage 来源归一化（预期差异）")
		case entry == "generic" && (bu.CacheReadTokens > 0 || bu.CacheWriteTokens > 0):
			hints = append(hints, "新公式识别 prompt_cache_hit_tokens/input_tokens_details 缓存口径（预期差异）")
		}
		if bu.Source == relayconstant.UsageSourceGemini && bu.ToolUsePromptTokens != nil {
			hints = append(hints, "旧公式把 toolUsePromptTokenCount 并入输入计价，新公式按 PRD 3.1 仅审计（预期差异）")
		}
	}
	if relayInfo != nil && relayInfo.PriceData.ContextPricing != nil && relayInfo.PriceData.ContextPricing.Enabled {
		hints = append(hints, "分段档位 tokens 现含输出维度（PRD 阶段 2）")
	}
	if relayInfo != nil && entry == "wss" && relayInfo.PriceData.UsePrice {
		hints = append(hints, "realtime 按次价按每个 response.done 事件实扣（P1-4/P1-34 口径），旧公式整场仅一份（预期差异）")
	}
	if len(hints) == 0 {
		return "unclassified: 需定位为旧公式 bug 或新语义 bug，禁止吸收差异"
	}
	return strings.Join(hints, "; ")
}
