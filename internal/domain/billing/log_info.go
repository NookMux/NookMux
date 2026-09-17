package billing

import (
	"strings"

	"github.com/NookMux/NookMux/internal/common"
	"github.com/NookMux/NookMux/internal/domain/shared"
	relaycommon "github.com/NookMux/NookMux/internal/relay/common"

	"github.com/NookMux/NookMux/internal/domain/billing/contract"
	domainchannel "github.com/NookMux/NookMux/internal/domain/channel"
	"github.com/NookMux/NookMux/internal/httpapi"
	relayconstant "github.com/NookMux/NookMux/internal/relay/constant"
	"github.com/gin-gonic/gin"
)

// CalculateStreamSpeed returns output tokens per second for streaming text responses.
// It prefers the post-first-token generation window and falls back to end-to-end latency
// when the stream only flushes once or the instantaneous value is obviously abnormal.
func CalculateStreamSpeed(useTimeMs int64, frtMs int64, completionTokens int, receivedResponseCount int) (float64, bool) {
	if useTimeMs <= 0 || completionTokens <= 0 {
		return 0, false
	}

	// 部分“流式”响应会在首包后一次性刷出全部内容，回退到端到端耗时避免异常峰值。
	effectiveMs := useTimeMs - frtMs
	if effectiveMs <= 0 || receivedResponseCount <= 1 {
		effectiveMs = useTimeMs
	}

	speed := float64(completionTokens) / (float64(effectiveMs) / 1000.0)
	if speed > 1000 && effectiveMs != useTimeMs {
		speed = float64(completionTokens) / (float64(useTimeMs) / 1000.0)
	}
	if speed <= 0 || speed > 1000 {
		return 0, false
	}
	return speed, true
}

// AppendStreamMetrics enriches consume-log metadata with shared stream-only metrics.
func AppendStreamMetrics(other map[string]interface{}, relayInfo *relaycommon.RelayInfo, useTimeMs int64, completionTokens int) {
	if other == nil || relayInfo == nil || !relayInfo.IsStream {
		return
	}

	frtMs := relayInfo.FirstResponseTime.Sub(relayInfo.StartTime).Milliseconds()
	if frtMs < 0 {
		frtMs = 0
	}
	if speed, ok := CalculateStreamSpeed(useTimeMs, frtMs, completionTokens, relayInfo.ReceivedResponseCount); ok {
		other["speed"] = speed
	}
}

func appendRequestPath(ctx *gin.Context, relayInfo *relaycommon.RelayInfo, other map[string]interface{}) {
	if other == nil {
		return
	}
	if ctx != nil && ctx.Request != nil && ctx.Request.URL != nil {
		if path := ctx.Request.URL.Path; path != "" {
			other["request_path"] = path
			return
		}
	}
	if relayInfo != nil && relayInfo.RequestURLPath != "" {
		path := relayInfo.RequestURLPath
		if idx := strings.Index(path, "?"); idx != -1 {
			path = path[:idx]
		}
		other["request_path"] = path
	}
}

func GenerateTextOtherInfo(ctx *gin.Context, relayInfo *relaycommon.RelayInfo, modelRatio, groupRatio, completionRatio float64,
	cacheTokens int, cacheRatio float64, modelPrice float64, userGroupRatio float64, dynamicRatio float64) map[string]interface{} {
	other := make(map[string]interface{})
	other["model_ratio"] = modelRatio
	other["group_ratio"] = groupRatio
	other["completion_ratio"] = completionRatio
	other["cache_tokens"] = cacheTokens
	other["cache_ratio"] = cacheRatio
	other["model_price"] = modelPrice
	other["user_group_ratio"] = userGroupRatio
	if dynamicRatio > 0 {
		other["dynamic_ratio"] = dynamicRatio
	}
	if relayInfo.HasSendResponse() {
		frt := float64(relayInfo.FirstResponseTime.UnixMilli() - relayInfo.StartTime.UnixMilli())
		if frt >= 0 {
			other["frt"] = frt
		}
	}
	if relayInfo.ReasoningEffort != "" {
		other["reasoning_effort"] = relayInfo.ReasoningEffort
	}
	if relayInfo.IsModelMapped {
		other["is_model_mapped"] = true
		other["upstream_model_name"] = relayInfo.UpstreamModelName
	}

	adminInfo := make(map[string]interface{})
	adminInfo["use_channel"] = ctx.GetStringSlice("use_channel")
	isMultiKey := httpapi.GetContextKeyBool(ctx, common.ContextKeyChannelIsMultiKey)
	if isMultiKey {
		adminInfo["is_multi_key"] = true
		adminInfo["multi_key_index"] = httpapi.GetContextKeyInt(ctx, common.ContextKeyChannelMultiKeyIndex)
	}

	isLocalCountTokens := httpapi.GetContextKeyBool(ctx, common.ContextKeyLocalCountTokens)
	if isLocalCountTokens {
		adminInfo["local_count_tokens"] = isLocalCountTokens
	}

	domainchannel.AppendChannelAffinityAdminInfo(ctx, adminInfo)

	other["admin_info"] = adminInfo
	appendRequestPath(ctx, relayInfo, other)
	appendRequestConversionChain(relayInfo, other)
	appendBillingInfo(relayInfo, other)
	return other
}

func appendBillingInfo(relayInfo *relaycommon.RelayInfo, other map[string]interface{}) {
	if relayInfo == nil || other == nil {
		return
	}
	if relayInfo.BillingSource != "" {
		other["billing_source"] = relayInfo.BillingSource
	}
	if relayInfo.PriceData.ContextPricing != nil && relayInfo.PriceData.ContextPricing.Enabled {
		result := relayInfo.PriceData.ContextPricing
		other["context_pricing_enabled"] = true
		other["context_tokens_for_tier"] = result.ContextTokensForTier
		other["context_pricing_tier_index"] = result.TierIndex
		other["context_pricing_tier_name"] = result.TierName
		other["context_pricing_tier_min_tokens"] = result.MinTokens
		if result.MaxTokens != nil {
			other["context_pricing_tier_max_tokens"] = *result.MaxTokens
		}
		other["context_pricing_prices"] = result.Prices
		other["context_pricing"] = result
	}
}

func appendRequestConversionChain(relayInfo *relaycommon.RelayInfo, other map[string]interface{}) {
	if relayInfo == nil || other == nil {
		return
	}
	if len(relayInfo.RequestConversionChain) == 0 {
		return
	}
	chain := make([]string, 0, len(relayInfo.RequestConversionChain))
	for _, f := range relayInfo.RequestConversionChain {
		switch f {
		case relayconstant.RelayFormatOpenAI:
			chain = append(chain, "OpenAI Compatible")
		case relayconstant.RelayFormatClaude:
			chain = append(chain, "Claude Messages")
		case relayconstant.RelayFormatGemini:
			chain = append(chain, "Google Gemini")
		case relayconstant.RelayFormatOpenAIResponses:
			chain = append(chain, "OpenAI Responses")
		default:
			chain = append(chain, string(f))
		}
	}
	if len(chain) == 0 {
		return
	}
	other["request_conversion"] = chain
}

func GenerateWssOtherInfo(ctx *gin.Context, relayInfo *relaycommon.RelayInfo, usage *shared.RealtimeUsage, modelRatio, groupRatio, completionRatio, audioRatio, audioCompletionRatio, modelPrice, userGroupRatio float64) map[string]interface{} {
	info := GenerateTextOtherInfo(ctx, relayInfo, modelRatio, groupRatio, completionRatio, 0, 0.0, modelPrice, userGroupRatio, 0)
	info["ws"] = true
	info["audio_input"] = usage.InputTokenDetails.AudioTokens
	info["audio_output"] = usage.OutputTokenDetails.AudioTokens
	info["text_input"] = usage.InputTokenDetails.TextTokens
	info["text_output"] = usage.OutputTokenDetails.TextTokens
	info["audio_ratio"] = audioRatio
	info["audio_completion_ratio"] = audioCompletionRatio
	return info
}

func GenerateAudioOtherInfo(ctx *gin.Context, relayInfo *relaycommon.RelayInfo, usage *shared.Usage, modelRatio, groupRatio, completionRatio, audioRatio, audioCompletionRatio, modelPrice, userGroupRatio float64) map[string]interface{} {
	info := GenerateTextOtherInfo(ctx, relayInfo, modelRatio, groupRatio, completionRatio, 0, 0.0, modelPrice, userGroupRatio, 0)
	info["audio"] = true
	info["audio_input"] = usage.PromptTokensDetails.AudioTokens
	info["audio_output"] = usage.CompletionTokenDetails.AudioTokens
	info["text_input"] = usage.PromptTokensDetails.TextTokens
	info["text_output"] = usage.CompletionTokenDetails.TextTokens
	info["audio_ratio"] = audioRatio
	info["audio_completion_ratio"] = audioCompletionRatio
	return info
}

func GenerateClaudeOtherInfo(ctx *gin.Context, relayInfo *relaycommon.RelayInfo, modelRatio, groupRatio, completionRatio float64,
	cacheTokens int, cacheRatio float64,
	cacheCreationTokens int, cacheCreationRatio float64,
	cacheCreationTokens5m int, cacheCreationRatio5m float64,
	cacheCreationTokens1h int, cacheCreationRatio1h float64,
	modelPrice float64, userGroupRatio float64) map[string]interface{} {
	info := GenerateTextOtherInfo(ctx, relayInfo, modelRatio, groupRatio, completionRatio, cacheTokens, cacheRatio, modelPrice, userGroupRatio, 0)
	info["claude"] = true
	info["cache_creation_tokens"] = cacheCreationTokens
	info["cache_creation_ratio"] = cacheCreationRatio
	if cacheCreationTokens5m != 0 {
		info["cache_creation_tokens_5m"] = cacheCreationTokens5m
		info["cache_creation_ratio_5m"] = cacheCreationRatio5m
	}
	if cacheCreationTokens1h != 0 {
		info["cache_creation_tokens_1h"] = cacheCreationTokens1h
		info["cache_creation_ratio_1h"] = cacheCreationRatio1h
	}
	return info
}

func GenerateMjOtherInfo(relayInfo *relaycommon.RelayInfo, priceData contract.PerCallPriceData) map[string]interface{} {
	other := make(map[string]interface{})
	other["model_price"] = priceData.ModelPrice
	other["group_ratio"] = priceData.GroupRatioInfo.GroupRatio
	if priceData.GroupRatioInfo.DynamicRatio > 0 {
		other["dynamic_ratio"] = priceData.GroupRatioInfo.DynamicRatio
		other["group_ratio"] = priceData.GroupRatioInfo.GroupRatio / priceData.GroupRatioInfo.DynamicRatio
	}
	if priceData.GroupRatioInfo.HasSpecialRatio {
		other["user_group_ratio"] = priceData.GroupRatioInfo.GroupSpecialRatio
	}
	appendRequestPath(nil, relayInfo, other)
	return other
}
