package billing

import (
	"fmt"
	"strings"
	"time"

	"github.com/NookMux/NookMux/internal/common"
	"github.com/NookMux/NookMux/internal/config/operation"
	domainchannel "github.com/NookMux/NookMux/internal/domain/channel"
	"github.com/NookMux/NookMux/internal/domain/shared"
	"github.com/NookMux/NookMux/internal/httpapi"
	"github.com/NookMux/NookMux/internal/infra/log"
	relaycommon "github.com/NookMux/NookMux/internal/relay/common"
	relayconstant "github.com/NookMux/NookMux/internal/relay/constant"
	"github.com/NookMux/NookMux/internal/store/channel"
	"github.com/NookMux/NookMux/internal/store/log"
	"github.com/NookMux/NookMux/internal/store/user"

	"github.com/gin-gonic/gin"
	"github.com/shopspring/decimal"
)

// UsageSettlement 承载 CalculateUsage 的计算结果，交由 ApplyQuota 落账。
// 字段不导出：调用方把它当作不透明句柄传回即可。
type UsageSettlement struct {
	modelName    string
	tokenName    string
	useTimeMs    int64
	quota        int
	extraContent []string

	promptTokens         int
	completionTokens     int
	cacheTokens          int
	cachedCreationTokens int
	cachedCreationRatio  float64
	audioTokens          int

	modelRatio      float64
	completionRatio float64
	cacheRatio      float64
	modelPrice      float64
	// groupRatio 是计价前快照；ApplyQuota 计算 originalGroupRatio 以它为基数，
	// 不得改为落账时从 PriceData 重读（计价/落账副作用之间不存在改写者，
	// 但快照语义与计价阶段使用的 dGroupRatio 保持同源）。
	groupRatio float64

	isClaudeUsageSemantic bool
	adminRejectReason     string

	webSearchPrice           float64
	claudeWebSearchPrice     float64
	claudeWebSearchCallCount int
	geminiWebSearchPrice     float64
	geminiWebSearchCallCount int
	fileSearchPrice          float64
	imageGenerationCallPrice float64
	audioInputPrice          float64

	dWebSearchQuota           decimal.Decimal
	dClaudeWebSearchQuota     decimal.Decimal
	dGeminiWebSearchQuota     decimal.Decimal
	dFileSearchQuota          decimal.Decimal
	dImageGenerationCallQuota decimal.Decimal
	audioInputQuota           decimal.Decimal

	totalTokensZero bool
	hasToolFees     bool
}

// CalculateUsage 根据原始 usage 与计费上下文（RelayInfo.PriceData 等）计算应扣额度，
// 是通用文本路径 usage 计费的单点入口。rawUsage 为 nil 时按预估 prompt tokens 兜底。
// 返回错误时（上游缺失计费信息且不可重试豁免）调用方应直接中止，不要调用 ApplyQuota。
func CalculateUsage(ctx *gin.Context, relayInfo *relaycommon.RelayInfo, rawUsage *shared.Usage, extraContent ...string) (*UsageSettlement, *shared.NookMuxError) {
	usage := rawUsage
	if usage == nil {
		usage = &shared.Usage{
			PromptTokens:     relayInfo.GetEstimatePromptTokens(),
			CompletionTokens: 0,
			TotalTokens:      relayInfo.GetEstimatePromptTokens(),
		}
		extraContent = append(extraContent, "上游无计费信息")
	}

	if rawUsage != nil {
		domainchannel.ObserveChannelAffinityUsageCacheFromContext(ctx, usage)
	}

	adminRejectReason := httpapi.GetContextKeyString(ctx, common.ContextKeyAdminRejectReason)

	useTimeMs := time.Since(relayInfo.StartTime).Milliseconds()
	promptTokens := usage.PromptTokens
	cacheTokens := usage.PromptTokensDetails.CachedTokens
	audioTokens := usage.PromptTokensDetails.AudioTokens
	completionTokens := usage.CompletionTokens
	cachedCreationTokens := usage.PromptTokensDetails.CachedCreationTokens

	modelName := relayInfo.OriginModelName

	tokenName := ctx.GetString("token_name")
	completionRatio := relayInfo.PriceData.CompletionRatio
	cacheRatio := relayInfo.PriceData.CacheRatio
	modelRatio := relayInfo.PriceData.ModelRatio
	groupRatio := relayInfo.PriceData.GroupRatioInfo.GroupRatio
	modelPrice := relayInfo.PriceData.ModelPrice
	cachedCreationRatio := relayInfo.PriceData.CacheCreationRatio
	isClaudeUsageSemantic := relayInfo.FinalRequestRelayFormat == relayconstant.RelayFormatClaude

	if _, enabled, err := ApplyContextPricingForUsage(modelName, BuildContextPricingUsage(usage, isClaudeUsageSemantic), &relayInfo.PriceData); enabled {
		if err != nil {
			log.LogError(ctx, "context pricing failed: "+err.Error())
			extraContent = append(extraContent, "分段计费匹配失败: "+err.Error())
		} else {
			completionRatio = relayInfo.PriceData.CompletionRatio
			cacheRatio = relayInfo.PriceData.CacheRatio
			modelRatio = relayInfo.PriceData.ModelRatio
			modelPrice = relayInfo.PriceData.ModelPrice
			cachedCreationRatio = relayInfo.PriceData.CacheCreationRatio
		}
	}

	// Convert values to decimal for precise calculation
	dPromptTokens := decimal.NewFromInt(int64(promptTokens))
	dCacheTokens := decimal.NewFromInt(int64(cacheTokens))
	dAudioTokens := decimal.NewFromInt(int64(audioTokens))
	dCompletionTokens := decimal.NewFromInt(int64(completionTokens))
	dCachedCreationTokens := decimal.NewFromInt(int64(cachedCreationTokens))
	dCompletionRatio := decimal.NewFromFloat(completionRatio)
	dCacheRatio := decimal.NewFromFloat(cacheRatio)
	dModelRatio := decimal.NewFromFloat(modelRatio)
	dGroupRatio := decimal.NewFromFloat(groupRatio)
	dModelPrice := decimal.NewFromFloat(modelPrice)
	dCachedCreationRatio := decimal.NewFromFloat(cachedCreationRatio)
	dQuotaPerUnit := decimal.NewFromFloat(common.QuotaPerUnit)

	ratio := dModelRatio.Mul(dGroupRatio)

	// openai web search 工具计费
	var dWebSearchQuota decimal.Decimal
	var webSearchPrice float64
	// response api 格式工具计费
	if relayInfo.ResponsesUsageInfo != nil {
		if webSearchTool, exists := relayInfo.ResponsesUsageInfo.BuiltInTools[shared.BuildInToolWebSearchPreview]; exists && webSearchTool.CallCount > 0 {
			// 优先使用可配置价格，回退到硬编码常量
			if pricePerCall, ok := operation.GetToolBillingPrice("web_search", map[string]string{"model": modelName, "provider": "openai"}); ok {
				webSearchPrice = pricePerCall
				dWebSearchQuota = decimal.NewFromFloat(webSearchPrice).
					Mul(decimal.NewFromInt(int64(webSearchTool.CallCount))).
					Mul(dGroupRatio).Mul(dQuotaPerUnit)
			} else {
				webSearchPrice = operation.GetWebSearchPricePerThousand(modelName, webSearchTool.SearchContextSize)
				dWebSearchQuota = decimal.NewFromFloat(webSearchPrice).
					Mul(decimal.NewFromInt(int64(webSearchTool.CallCount))).
					Div(decimal.NewFromInt(1000)).Mul(dGroupRatio).Mul(dQuotaPerUnit)
			}
			extraContent = append(extraContent, fmt.Sprintf("Web Search 调用 %d 次，上下文大小 %s，调用花费 %s",
				webSearchTool.CallCount, webSearchTool.SearchContextSize, dWebSearchQuota.String()))
		}
	} else if strings.HasSuffix(modelName, "search-preview") {
		// search-preview 模型不支持 response api
		searchContextSize := ctx.GetString("chat_completion_web_search_context_size")
		if searchContextSize == "" {
			searchContextSize = "medium"
		}
		if pricePerCall, ok := operation.GetToolBillingPrice("web_search", map[string]string{"model": modelName, "provider": "openai"}); ok {
			webSearchPrice = pricePerCall
			dWebSearchQuota = decimal.NewFromFloat(webSearchPrice).
				Mul(dGroupRatio).Mul(dQuotaPerUnit)
		} else {
			webSearchPrice = operation.GetWebSearchPricePerThousand(modelName, searchContextSize)
			dWebSearchQuota = decimal.NewFromFloat(webSearchPrice).
				Div(decimal.NewFromInt(1000)).Mul(dGroupRatio).Mul(dQuotaPerUnit)
		}
		extraContent = append(extraContent, fmt.Sprintf("Web Search 调用 1 次，上下文大小 %s，调用花费 %s",
			searchContextSize, dWebSearchQuota.String()))
	}
	// claude web search tool 计费
	var dClaudeWebSearchQuota decimal.Decimal
	var claudeWebSearchPrice float64
	claudeWebSearchCallCount := ctx.GetInt("claude_web_search_requests")
	if claudeWebSearchCallCount > 0 {
		if pricePerCall, ok := operation.GetToolBillingPrice("web_search", map[string]string{"model": modelName, "provider": "claude"}); ok {
			claudeWebSearchPrice = pricePerCall
			dClaudeWebSearchQuota = decimal.NewFromFloat(claudeWebSearchPrice).
				Mul(decimal.NewFromInt(int64(claudeWebSearchCallCount))).
				Mul(dGroupRatio).Mul(dQuotaPerUnit)
		} else {
			claudeWebSearchPrice = operation.GetClaudeWebSearchPricePerThousand()
			dClaudeWebSearchQuota = decimal.NewFromFloat(claudeWebSearchPrice).
				Div(decimal.NewFromInt(1000)).Mul(dGroupRatio).Mul(dQuotaPerUnit).Mul(decimal.NewFromInt(int64(claudeWebSearchCallCount)))
		}
		extraContent = append(extraContent, fmt.Sprintf("Claude Web Search 调用 %d 次，调用花费 %s",
			claudeWebSearchCallCount, dClaudeWebSearchQuota.String()))
	}
	// gemini web search tool 计费
	var dGeminiWebSearchQuota decimal.Decimal
	var geminiWebSearchPrice float64
	geminiWebSearchCallCount := ctx.GetInt("gemini_web_search_requests")
	if geminiWebSearchCallCount > 0 {
		if pricePerCall, ok := operation.GetToolBillingPrice("web_search", map[string]string{"model": modelName, "provider": "gemini"}); ok {
			geminiWebSearchPrice = pricePerCall
			dGeminiWebSearchQuota = decimal.NewFromFloat(geminiWebSearchPrice).
				Mul(decimal.NewFromInt(int64(geminiWebSearchCallCount))).
				Mul(dGroupRatio).Mul(dQuotaPerUnit)
		}
		if !dGeminiWebSearchQuota.IsZero() {
			extraContent = append(extraContent, fmt.Sprintf("Gemini Web Search 调用 %d 次，调用花费 %s",
				geminiWebSearchCallCount, dGeminiWebSearchQuota.String()))
		}
	}
	// file search tool 计费
	var dFileSearchQuota decimal.Decimal
	var fileSearchPrice float64
	if relayInfo.ResponsesUsageInfo != nil {
		if fileSearchTool, exists := relayInfo.ResponsesUsageInfo.BuiltInTools[shared.BuildInToolFileSearch]; exists && fileSearchTool.CallCount > 0 {
			fileSearchPrice = operation.GetFileSearchPricePerThousand()
			dFileSearchQuota = decimal.NewFromFloat(fileSearchPrice).
				Mul(decimal.NewFromInt(int64(fileSearchTool.CallCount))).
				Div(decimal.NewFromInt(1000)).Mul(dGroupRatio).Mul(dQuotaPerUnit)
			extraContent = append(extraContent, fmt.Sprintf("File Search 调用 %d 次，调用花费 %s",
				fileSearchTool.CallCount, dFileSearchQuota.String()))
		}
	}
	var dImageGenerationCallQuota decimal.Decimal
	var imageGenerationCallPrice float64
	if ctx.GetBool("image_generation_call") {
		if pricePerCall, ok := operation.GetToolBillingPrice("image_generation", map[string]string{
			"model":    modelName,
			"provider": "openai",
			"quality":  ctx.GetString("image_generation_call_quality"),
			"size":     ctx.GetString("image_generation_call_size"),
		}); ok {
			imageGenerationCallPrice = pricePerCall
		} else {
			imageGenerationCallPrice = operation.GetGPTImage1PriceOnceCall(ctx.GetString("image_generation_call_quality"), ctx.GetString("image_generation_call_size"))
		}
		dImageGenerationCallQuota = decimal.NewFromFloat(imageGenerationCallPrice).Mul(dGroupRatio).Mul(dQuotaPerUnit)
		extraContent = append(extraContent, fmt.Sprintf("Image Generation Call 花费 %s", dImageGenerationCallQuota.String()))
	}

	var quotaCalculateDecimal decimal.Decimal

	var audioInputQuota decimal.Decimal
	var audioInputPrice float64
	if !relayInfo.PriceData.UsePrice {
		baseTokens := dPromptTokens
		// 减去 cached tokens
		// Anthropic API 的 input_tokens 已经不包含缓存 tokens，不需要减去
		// OpenAI/OpenRouter 等 API 的 prompt_tokens 包含缓存 tokens，需要减去
		var cachedTokensWithRatio decimal.Decimal
		if !dCacheTokens.IsZero() {
			if !isClaudeUsageSemantic {
				baseTokens = baseTokens.Sub(dCacheTokens)
			}
			cachedTokensWithRatio = dCacheTokens.Mul(dCacheRatio)
		}
		var dCachedCreationTokensWithRatio decimal.Decimal
		if !dCachedCreationTokens.IsZero() {
			if !isClaudeUsageSemantic {
				baseTokens = baseTokens.Sub(dCachedCreationTokens)
			}
			dCachedCreationTokensWithRatio = dCachedCreationTokens.Mul(dCachedCreationRatio)
		}

		// 减去 Gemini audio tokens
		if !dAudioTokens.IsZero() {
			audioInputPrice = operation.GetGeminiInputAudioPricePerMillionTokens(modelName)
			if audioInputPrice > 0 {
				// 重新计算 base tokens
				baseTokens = baseTokens.Sub(dAudioTokens)
				audioInputQuota = decimal.NewFromFloat(audioInputPrice).Div(decimal.NewFromInt(1000000)).Mul(dAudioTokens).Mul(dGroupRatio).Mul(dQuotaPerUnit)
				extraContent = append(extraContent, fmt.Sprintf("Audio Input 花费 %s", audioInputQuota.String()))
			}
		}
		promptQuota := baseTokens.Add(cachedTokensWithRatio).
			Add(dCachedCreationTokensWithRatio)

		completionQuota := dCompletionTokens.Mul(dCompletionRatio)

		quotaCalculateDecimal = promptQuota.Add(completionQuota).Mul(ratio)

		if !ratio.IsZero() && quotaCalculateDecimal.LessThanOrEqual(decimal.Zero) {
			quotaCalculateDecimal = decimal.NewFromInt(1)
		}
	} else {
		quotaCalculateDecimal = dModelPrice.Mul(dQuotaPerUnit).Mul(dGroupRatio)
	}
	// 添加 responses tools call 调用的配额
	quotaCalculateDecimal = quotaCalculateDecimal.Add(dWebSearchQuota)
	quotaCalculateDecimal = quotaCalculateDecimal.Add(dClaudeWebSearchQuota)
	quotaCalculateDecimal = quotaCalculateDecimal.Add(dGeminiWebSearchQuota)
	quotaCalculateDecimal = quotaCalculateDecimal.Add(dFileSearchQuota)
	// 添加 audio input 独立计费
	quotaCalculateDecimal = quotaCalculateDecimal.Add(audioInputQuota)
	// 添加 image generation call 计费
	quotaCalculateDecimal = quotaCalculateDecimal.Add(dImageGenerationCallQuota)

	if len(relayInfo.PriceData.OtherRatios) > 0 {
		for key, otherRatio := range relayInfo.PriceData.OtherRatios {
			dOtherRatio := decimal.NewFromFloat(otherRatio)
			quotaCalculateDecimal = quotaCalculateDecimal.Mul(dOtherRatio)
			extraContent = append(extraContent, fmt.Sprintf("其他倍率 %s: %f", key, otherRatio))
		}
	}

	settlement := &UsageSettlement{
		modelName:             modelName,
		tokenName:             tokenName,
		useTimeMs:             useTimeMs,
		promptTokens:          promptTokens,
		completionTokens:      completionTokens,
		cacheTokens:           cacheTokens,
		cachedCreationTokens:  cachedCreationTokens,
		cachedCreationRatio:   cachedCreationRatio,
		audioTokens:           audioTokens,
		modelRatio:            modelRatio,
		completionRatio:       completionRatio,
		cacheRatio:            cacheRatio,
		modelPrice:            modelPrice,
		groupRatio:            groupRatio,
		isClaudeUsageSemantic: isClaudeUsageSemantic,
		adminRejectReason:     adminRejectReason,

		webSearchPrice:           webSearchPrice,
		claudeWebSearchPrice:     claudeWebSearchPrice,
		claudeWebSearchCallCount: claudeWebSearchCallCount,
		geminiWebSearchPrice:     geminiWebSearchPrice,
		geminiWebSearchCallCount: geminiWebSearchCallCount,
		fileSearchPrice:          fileSearchPrice,
		imageGenerationCallPrice: imageGenerationCallPrice,
		audioInputPrice:          audioInputPrice,

		dWebSearchQuota:           dWebSearchQuota,
		dClaudeWebSearchQuota:     dClaudeWebSearchQuota,
		dGeminiWebSearchQuota:     dGeminiWebSearchQuota,
		dFileSearchQuota:          dFileSearchQuota,
		dImageGenerationCallQuota: dImageGenerationCallQuota,
		audioInputQuota:           audioInputQuota,

		extraContent: extraContent,
	}

	quota := int(quotaCalculateDecimal.Round(0).IntPart())
	totalTokens := promptTokens + completionTokens

	// record all the consume log even if quota is 0
	// 未发生规范转换时，totalTokens == 0 代表上游缺失计费信息，需要交给外层重试。
	// 发生规范转换时，可能是转换导致的 token 统计异常，继续记录消费日志但不触发重试。
	if totalTokens == 0 {
		if apiErr := NewEmptyUsageRetryError(ctx, relayInfo); apiErr != nil {
			return nil, apiErr
		}
		settlement.totalTokensZero = true
		// 上游没有返回 token 信息（可能是超时或错误），但如果有工具调用费用，仍需扣费
		toolQuota := dWebSearchQuota.Add(dClaudeWebSearchQuota).Add(dGeminiWebSearchQuota).
			Add(dFileSearchQuota).Add(dImageGenerationCallQuota).Add(audioInputQuota)
		if toolQuota.GreaterThan(decimal.Zero) {
			settlement.hasToolFees = true
			settlement.quota = int(toolQuota.Round(0).IntPart())
			settlement.extraContent = append(settlement.extraContent, "上游没有返回计费信息，但工具调用费用仍需扣除")
		} else {
			settlement.quota = 0
			settlement.extraContent = append(settlement.extraContent, "上游没有返回计费信息，无法扣费（可能是上游超时）")
		}
	} else {
		if !ratio.IsZero() && quota == 0 {
			quota = 1
		}
		settlement.quota = quota
	}
	return settlement, nil
}

// ApplyQuota 将 CalculateUsage 的结果落到配额与账务：用户/渠道已用计数、计费会话结算、消费日志。
// 与 CalculateUsage 成对使用；返回值当前恒为 nil，保持与各 PostXxxConsumeQuota 相同的签名形状。
func ApplyQuota(ctx *gin.Context, relayInfo *relaycommon.RelayInfo, settlement *UsageSettlement) *shared.NookMuxError {
	// record all the consume log even if quota is 0
	logType := 0 // 0 表示使用默认的 LogTypeConsume
	if settlement.totalTokensZero {
		if settlement.hasToolFees {
			userstore.UpdateUserUsedQuotaAndRequestCount(relayInfo.UserId, settlement.quota)
			channelstore.UpdateChannelUsedQuota(relayInfo.ChannelId, settlement.quota)
		}
		log.LogError(ctx, fmt.Sprintf("total tokens is 0, cannot consume quota, userId %d, channelId %d, "+
			"tokenId %d, model %s， pre-consumed quota %d", relayInfo.UserId, relayInfo.ChannelId, relayInfo.TokenId, settlement.modelName, relayInfo.FinalPreConsumedQuota))
	} else {
		userstore.UpdateUserUsedQuotaAndRequestCount(relayInfo.UserId, settlement.quota)
		channelstore.UpdateChannelUsedQuota(relayInfo.ChannelId, settlement.quota)
	}

	if err := SettleBilling(ctx, relayInfo, settlement.quota); err != nil {
		// 结算失败意味着实际消费未能足额扣减（如并发窗口内余额被其他请求
		// 扣尽），平台承担该笔差额损失，必须以系统级错误告警暴露，不允许
		// 静默吞掉。
		common.SysError(fmt.Sprintf(
			"billing settle failed, platform absorbs the shortfall: userId=%d tokenId=%d channelId=%d quota=%d: %s",
			relayInfo.UserId, relayInfo.TokenId, relayInfo.ChannelId, settlement.quota, err.Error()))
	}

	logModel := settlement.modelName
	if strings.HasPrefix(logModel, "gpt-4-gizmo") {
		logModel = "gpt-4-gizmo-*"
		settlement.extraContent = append(settlement.extraContent, fmt.Sprintf("模型 %s", settlement.modelName))
	}
	if strings.HasPrefix(logModel, "gpt-4o-gizmo") {
		logModel = "gpt-4o-gizmo-*"
		settlement.extraContent = append(settlement.extraContent, fmt.Sprintf("模型 %s", settlement.modelName))
	}
	logContent := strings.Join(settlement.extraContent, ", ")
	// 计算原始分组倍率（不含动态倍率），用于日志记录
	groupRatio := settlement.groupRatio
	originalGroupRatio := groupRatio
	dynamicRatio := relayInfo.PriceData.GroupRatioInfo.DynamicRatio
	if dynamicRatio > 0 {
		originalGroupRatio = groupRatio / dynamicRatio
	}
	other := GenerateTextOtherInfo(ctx, relayInfo, settlement.modelRatio, originalGroupRatio, settlement.completionRatio, settlement.cacheTokens, settlement.cacheRatio, settlement.modelPrice, relayInfo.PriceData.GroupRatioInfo.GroupSpecialRatio, dynamicRatio)
	if settlement.adminRejectReason != "" {
		other["reject_reason"] = settlement.adminRejectReason
	}
	// For chat-based calls to the Claude model, tagging is required. Using Claude's rendering logs, the two approaches handle input rendering differently.
	if settlement.isClaudeUsageSemantic {
		other["claude"] = true
		other["usage_semantic"] = "anthropic"
	}
	if settlement.cachedCreationTokens != 0 {
		other["cache_creation_tokens"] = settlement.cachedCreationTokens
		other["cache_creation_ratio"] = settlement.cachedCreationRatio
	}
	if !settlement.dWebSearchQuota.IsZero() {
		if relayInfo.ResponsesUsageInfo != nil {
			if webSearchTool, exists := relayInfo.ResponsesUsageInfo.BuiltInTools[shared.BuildInToolWebSearchPreview]; exists {
				other["web_search"] = true
				other["web_search_call_count"] = webSearchTool.CallCount
				other["web_search_price"] = settlement.webSearchPrice
			}
		} else if strings.HasSuffix(settlement.modelName, "search-preview") {
			other["web_search"] = true
			other["web_search_call_count"] = 1
			other["web_search_price"] = settlement.webSearchPrice
		}
	} else if !settlement.dClaudeWebSearchQuota.IsZero() {
		other["web_search"] = true
		other["web_search_call_count"] = settlement.claudeWebSearchCallCount
		other["web_search_price"] = settlement.claudeWebSearchPrice
	} else if !settlement.dGeminiWebSearchQuota.IsZero() {
		other["web_search"] = true
		other["web_search_call_count"] = settlement.geminiWebSearchCallCount
		other["web_search_price"] = settlement.geminiWebSearchPrice
	}
	if !settlement.dFileSearchQuota.IsZero() && relayInfo.ResponsesUsageInfo != nil {
		if fileSearchTool, exists := relayInfo.ResponsesUsageInfo.BuiltInTools[shared.BuildInToolFileSearch]; exists {
			other["file_search"] = true
			other["file_search_call_count"] = fileSearchTool.CallCount
			other["file_search_price"] = settlement.fileSearchPrice
		}
	}
	if !settlement.audioInputQuota.IsZero() {
		other["audio_input_seperate_price"] = true
		other["audio_input_token_count"] = settlement.audioTokens
		other["audio_input_price"] = settlement.audioInputPrice
	}
	if !settlement.dImageGenerationCallQuota.IsZero() {
		other["image_generation_call"] = true
		other["image_generation_call_price"] = settlement.imageGenerationCallPrice
	}
	// 共享流式日志指标，确保 OpenAI 兼容与 Claude 消费日志展示一致。
	AppendStreamMetrics(other, relayInfo, settlement.useTimeMs, settlement.completionTokens)
	logstore.RecordConsumeLog(ctx, relayInfo.UserId, logstore.RecordConsumeLogParams{
		ChannelId:        relayInfo.ChannelId,
		PromptTokens:     settlement.promptTokens,
		CompletionTokens: settlement.completionTokens,
		ModelName:        logModel,
		TokenName:        settlement.tokenName,
		Quota:            settlement.quota,
		Content:          logContent,
		TokenId:          relayInfo.TokenId,
		UseTimeMs:        int(settlement.useTimeMs),
		IsStream:         relayInfo.IsStream,
		Group:            relayInfo.UsingGroup,
		Other:            other,
		LogType:          logType,
	})
	return nil
}
