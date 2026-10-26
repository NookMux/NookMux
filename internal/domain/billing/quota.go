package billing

import (
	"errors"
	"fmt"
	"github.com/NookMux/NookMux/internal/common"
	"github.com/NookMux/NookMux/internal/config/ratio"
	"github.com/NookMux/NookMux/internal/config/system"
	"github.com/NookMux/NookMux/internal/domain/billing/contract"
	"github.com/NookMux/NookMux/internal/domain/channel/constant"
	"github.com/NookMux/NookMux/internal/domain/shared"
	"github.com/NookMux/NookMux/internal/i18n"
	"github.com/NookMux/NookMux/internal/infra/log"
	notify "github.com/NookMux/NookMux/internal/infra/notify"
	"github.com/NookMux/NookMux/internal/infra/runtime"
	relaycommon "github.com/NookMux/NookMux/internal/relay/common"
	"github.com/NookMux/NookMux/internal/store/channel"
	"github.com/NookMux/NookMux/internal/store/log"
	"github.com/NookMux/NookMux/internal/store/token"
	"github.com/NookMux/NookMux/internal/store/user"
	"github.com/gin-gonic/gin"
	"github.com/shopspring/decimal"
	"math"
	"net/http"
	"time"
)

type TokenDetails struct {
	TextTokens  int
	AudioTokens int
}

// NewEmptyUsageRetryError returns a retryable upstream error when native-format responses contain no billing usage.
func NewEmptyUsageRetryError(ctx *gin.Context, relayInfo *relaycommon.RelayInfo) *shared.NookMuxError {
	if relayInfo == nil || len(relayInfo.RequestConversionChain) > 1 {
		return nil
	}
	return shared.NewOpenAIError(errors.New(i18n.T(ctx, i18n.MsgQuotaEmptyUsage)), shared.ErrorCodeBadResponse, http.StatusBadGateway)
}

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

func hasCustomModelRatio(modelName string, currentRatio float64) bool {
	defaultRatio, exists := ratio.GetDefaultModelRatioMap()[modelName]
	if !exists {
		return true
	}
	return currentRatio != defaultRatio
}

func calculateAudioQuota(info QuotaInfo) int {
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

func PreWssConsumeQuota(ctx *gin.Context, relayInfo *relaycommon.RelayInfo, usage *shared.RealtimeUsage) error {
	if relayInfo.PriceData.UsePrice {
		return nil
	}
	userQuota, err := userstore.GetUserQuota(relayInfo.UserId, false)
	if err != nil {
		return err
	}

	modelName := relayInfo.OriginModelName
	textInputTokens := usage.InputTokenDetails.TextTokens
	textOutTokens := usage.OutputTokenDetails.TextTokens
	audioInputTokens := usage.InputTokenDetails.AudioTokens
	audioOutTokens := usage.OutputTokenDetails.AudioTokens
	actualGroupRatio := relayInfo.PriceData.GroupRatioInfo.GroupRatio
	modelRatio := relayInfo.PriceData.ModelRatio
	if _, enabled, err := ApplyContextPricingForUsage(modelName, BuildRealtimeContextPricingUsage(usage), &relayInfo.PriceData); enabled {
		if err != nil {
			return err
		}
		actualGroupRatio = relayInfo.PriceData.GroupRatioInfo.GroupRatio
		modelRatio = relayInfo.PriceData.ModelRatio
	}

	quotaInfo := QuotaInfo{
		InputDetails: TokenDetails{
			TextTokens:  textInputTokens,
			AudioTokens: audioInputTokens,
		},
		OutputDetails: TokenDetails{
			TextTokens:  textOutTokens,
			AudioTokens: audioOutTokens,
		},
		ModelName:            modelName,
		UsePrice:             relayInfo.PriceData.UsePrice,
		ModelPrice:           relayInfo.PriceData.ModelPrice,
		ModelRatio:           modelRatio,
		GroupRatio:           actualGroupRatio,
		CompletionRatio:      relayInfo.PriceData.CompletionRatio,
		AudioRatio:           relayInfo.PriceData.AudioRatio,
		AudioCompletionRatio: relayInfo.PriceData.AudioCompletionRatio,
	}

	quota := calculateAudioQuota(quotaInfo)

	if userQuota < quota {
		return fmt.Errorf("%s", i18n.T(ctx, i18n.MsgQuotaUserNotEnough, map[string]any{"UserQuota": log.FormatQuota(userQuota), "NeedQuota": log.FormatQuota(quota)}))
	}

	// 认证阶段已经读取过 token，复用上下文快照避免重复查询数据库。
	if !relayInfo.TokenUnlimited && relayInfo.TokenQuota < quota {
		return fmt.Errorf("%s", i18n.T(ctx, i18n.MsgQuotaTokenNotEnough, map[string]any{"TokenQuota": log.FormatQuota(relayInfo.TokenQuota), "NeedQuota": log.FormatQuota(quota)}))
	}

	err = PostConsumeQuota(ctx, relayInfo, quota, 0, false)
	if err != nil {
		return err
	}
	relayInfo.FinalPreConsumedQuota += quota
	log.LogInfo(ctx, "realtime streaming consume quota success, quota: "+fmt.Sprintf("%d", quota))
	return nil
}

func PostWssConsumeQuota(ctx *gin.Context, relayInfo *relaycommon.RelayInfo, modelName string,
	usage *shared.RealtimeUsage, extraContent string) *shared.NookMuxError {

	useTimeMs := time.Since(relayInfo.StartTime).Milliseconds()
	textInputTokens := usage.InputTokenDetails.TextTokens
	textOutTokens := usage.OutputTokenDetails.TextTokens
	audioInputTokens := usage.InputTokenDetails.AudioTokens
	audioOutTokens := usage.OutputTokenDetails.AudioTokens

	tokenName := ctx.GetString("token_name")
	modelRatio := relayInfo.PriceData.ModelRatio
	groupRatio := relayInfo.PriceData.GroupRatioInfo.GroupRatio
	modelPrice := relayInfo.PriceData.ModelPrice
	usePrice := relayInfo.PriceData.UsePrice
	if _, enabled, err := ApplyContextPricingForUsage(modelName, BuildRealtimeContextPricingUsage(usage), &relayInfo.PriceData); enabled {
		if err != nil {
			log.LogError(ctx, "context pricing failed: "+err.Error())
		} else {
			modelRatio = relayInfo.PriceData.ModelRatio
			groupRatio = relayInfo.PriceData.GroupRatioInfo.GroupRatio
			modelPrice = relayInfo.PriceData.ModelPrice
			usePrice = relayInfo.PriceData.UsePrice
		}
	}
	completionRatio := decimal.NewFromFloat(relayInfo.PriceData.CompletionRatio)
	audioRatio := decimal.NewFromFloat(relayInfo.PriceData.AudioRatio)
	audioCompletionRatio := decimal.NewFromFloat(relayInfo.PriceData.AudioCompletionRatio)

	quotaInfo := QuotaInfo{
		InputDetails: TokenDetails{
			TextTokens:  textInputTokens,
			AudioTokens: audioInputTokens,
		},
		OutputDetails: TokenDetails{
			TextTokens:  textOutTokens,
			AudioTokens: audioOutTokens,
		},
		ModelName:            modelName,
		UsePrice:             usePrice,
		ModelPrice:           modelPrice,
		ModelRatio:           modelRatio,
		GroupRatio:           groupRatio,
		CompletionRatio:      relayInfo.PriceData.CompletionRatio,
		AudioRatio:           relayInfo.PriceData.AudioRatio,
		AudioCompletionRatio: relayInfo.PriceData.AudioCompletionRatio,
	}

	quota := calculateAudioQuota(quotaInfo)

	totalTokens := usage.TotalTokens
	var logContent string
	if !usePrice {
		logContent = fmt.Sprintf("模型倍率 %.2f，补全倍率 %.2f，音频倍率 %.2f，音频补全倍率 %.2f，分组倍率 %.2f",
			modelRatio, completionRatio.InexactFloat64(), audioRatio.InexactFloat64(), audioCompletionRatio.InexactFloat64(), groupRatio)
	} else {
		logContent = fmt.Sprintf("模型价格 %.2f，分组倍率 %.2f", modelPrice, groupRatio)
	}

	// record all the consume log even if quota is 0
	logType := 0 // 0 表示使用默认的 LogTypeConsume
	if totalTokens == 0 {
		if apiErr := NewEmptyUsageRetryError(ctx, relayInfo); apiErr != nil {
			return apiErr
		}
		// in this case, must be some error happened
		// we cannot just return, because we may have to return the pre-consumed quota
		logType = logstore.LogTypeError
		quota = 0
		logContent += "（可能是上游超时）"
		log.LogError(ctx, fmt.Sprintf("total tokens is 0, cannot consume quota, userId %d, channelId %d, "+
			"tokenId %d, model %s， pre-consumed quota %d", relayInfo.UserId, relayInfo.ChannelId, relayInfo.TokenId, modelName, relayInfo.FinalPreConsumedQuota))
	} else {
		userstore.UpdateUserUsedQuotaAndRequestCount(relayInfo.UserId, quota)
		channelstore.UpdateChannelUsedQuota(relayInfo.ChannelId, quota)
	}

	logModel := modelName
	if extraContent != "" {
		logContent += ", " + extraContent
	}
	other := GenerateWssOtherInfo(ctx, relayInfo, usage, modelRatio, groupRatio,
		completionRatio.InexactFloat64(), audioRatio.InexactFloat64(), audioCompletionRatio.InexactFloat64(), modelPrice, relayInfo.PriceData.GroupRatioInfo.GroupSpecialRatio)
	// 记录动态倍率到日志
	if relayInfo.PriceData.GroupRatioInfo.DynamicRatio > 0 {
		other["dynamic_ratio"] = relayInfo.PriceData.GroupRatioInfo.DynamicRatio
		// group_ratio 记录原始分组倍率（不含动态倍率）
		other["group_ratio"] = groupRatio / relayInfo.PriceData.GroupRatioInfo.DynamicRatio
	}
	logstore.RecordConsumeLog(ctx, relayInfo.UserId, logstore.RecordConsumeLogParams{
		ChannelId:        relayInfo.ChannelId,
		PromptTokens:     usage.InputTokens,
		CompletionTokens: usage.OutputTokens,
		ModelName:        logModel,
		TokenName:        tokenName,
		Quota:            quota,
		Content:          logContent,
		TokenId:          relayInfo.TokenId,
		UseTimeMs:        int(useTimeMs),
		IsStream:         relayInfo.IsStream,
		Group:            relayInfo.UsingGroup,
		Other:            other,
		LogType:          logType,
	})
	return nil
}

func PostClaudeConsumeQuota(ctx *gin.Context, relayInfo *relaycommon.RelayInfo, usage *shared.Usage) *shared.NookMuxError {

	useTimeMs := time.Since(relayInfo.StartTime).Milliseconds()
	promptTokens := usage.PromptTokens
	completionTokens := usage.CompletionTokens
	modelName := relayInfo.OriginModelName

	tokenName := ctx.GetString("token_name")
	completionRatio := relayInfo.PriceData.CompletionRatio
	modelRatio := relayInfo.PriceData.ModelRatio
	groupRatio := relayInfo.PriceData.GroupRatioInfo.GroupRatio
	modelPrice := relayInfo.PriceData.ModelPrice
	cacheRatio := relayInfo.PriceData.CacheRatio
	cacheTokens := usage.PromptTokensDetails.CachedTokens

	cacheCreationRatio := relayInfo.PriceData.CacheCreationRatio
	cacheCreationRatio5m := relayInfo.PriceData.CacheCreation5mRatio
	cacheCreationRatio1h := relayInfo.PriceData.CacheCreation1hRatio
	cacheCreationTokens := usage.PromptTokensDetails.CachedCreationTokens
	cacheCreationTokens5m := usage.ClaudeCacheCreation5mTokens
	cacheCreationTokens1h := usage.ClaudeCacheCreation1hTokens

	if relayInfo.ChannelType == constant.ChannelTypeOpenRouter {
		promptTokens -= cacheTokens
		isUsingCustomSettings := relayInfo.PriceData.UsePrice || hasCustomModelRatio(modelName, relayInfo.PriceData.ModelRatio)
		if cacheCreationTokens == 0 && relayInfo.PriceData.CacheCreationRatio != 1 && usage.Cost != 0 && !isUsingCustomSettings {
			maybeCacheCreationTokens := CalcOpenRouterCacheCreateTokens(*usage, relayInfo.PriceData)
			if maybeCacheCreationTokens >= 0 && promptTokens >= maybeCacheCreationTokens {
				cacheCreationTokens = maybeCacheCreationTokens
			}
		}
		promptTokens -= cacheCreationTokens
	}

	isClaudeUsageSemantic := relayInfo.ChannelType != constant.ChannelTypeOpenRouter
	contextUsage := BuildContextPricingUsage(usage, isClaudeUsageSemantic)
	contextUsage.CacheCreationTokens = cacheCreationTokens
	if _, enabled, err := ApplyContextPricingForUsage(modelName, contextUsage, &relayInfo.PriceData); enabled {
		if err != nil {
			log.LogError(ctx, "context pricing failed: "+err.Error())
		} else {
			completionRatio = relayInfo.PriceData.CompletionRatio
			modelRatio = relayInfo.PriceData.ModelRatio
			groupRatio = relayInfo.PriceData.GroupRatioInfo.GroupRatio
			modelPrice = relayInfo.PriceData.ModelPrice
			cacheRatio = relayInfo.PriceData.CacheRatio
			cacheCreationRatio = relayInfo.PriceData.CacheCreationRatio
			cacheCreationRatio5m = relayInfo.PriceData.CacheCreation5mRatio
			cacheCreationRatio1h = relayInfo.PriceData.CacheCreation1hRatio
		}
	}

	calculateQuota := 0.0
	if !relayInfo.PriceData.UsePrice {
		calculateQuota = float64(promptTokens)
		calculateQuota += float64(cacheTokens) * cacheRatio
		calculateQuota += float64(cacheCreationTokens5m) * cacheCreationRatio5m
		calculateQuota += float64(cacheCreationTokens1h) * cacheCreationRatio1h
		remainingCacheCreationTokens := cacheCreationTokens - cacheCreationTokens5m - cacheCreationTokens1h
		if remainingCacheCreationTokens > 0 {
			calculateQuota += float64(remainingCacheCreationTokens) * cacheCreationRatio
		}
		calculateQuota += float64(completionTokens) * completionRatio
		calculateQuota = calculateQuota * groupRatio * modelRatio
	} else {
		calculateQuota = modelPrice * common.QuotaPerUnit * groupRatio
	}

	if modelRatio != 0 && calculateQuota <= 0 {
		calculateQuota = 1
	}

	quota := int(calculateQuota)

	totalTokens := promptTokens + completionTokens

	var logContent string
	// record all the consume log even if quota is 0
	logType := 0 // 0 表示使用默认的 LogTypeConsume
	if totalTokens == 0 {
		if apiErr := NewEmptyUsageRetryError(ctx, relayInfo); apiErr != nil {
			return apiErr
		}
		// in this case, must be some error happened
		// we cannot just return, because we may have to return the pre-consumed quota
		logType = logstore.LogTypeError
		quota = 0
		logContent += "（可能是上游出错）"
		log.LogError(ctx, fmt.Sprintf("total tokens is 0, cannot consume quota, userId %d, channelId %d, "+
			"tokenId %d, model %s， pre-consumed quota %d", relayInfo.UserId, relayInfo.ChannelId, relayInfo.TokenId, modelName, relayInfo.FinalPreConsumedQuota))
	} else {
		userstore.UpdateUserUsedQuotaAndRequestCount(relayInfo.UserId, quota)
		channelstore.UpdateChannelUsedQuota(relayInfo.ChannelId, quota)
	}

	if err := SettleBilling(ctx, relayInfo, quota); err != nil {
		log.LogError(ctx, "error settling billing: "+err.Error())
	}

	other := GenerateClaudeOtherInfo(ctx, relayInfo, modelRatio, groupRatio, completionRatio,
		cacheTokens, cacheRatio,
		cacheCreationTokens, cacheCreationRatio,
		cacheCreationTokens5m, cacheCreationRatio5m,
		cacheCreationTokens1h, cacheCreationRatio1h,
		modelPrice, relayInfo.PriceData.GroupRatioInfo.GroupSpecialRatio)
	// 记录动态倍率到日志
	if relayInfo.PriceData.GroupRatioInfo.DynamicRatio > 0 {
		other["dynamic_ratio"] = relayInfo.PriceData.GroupRatioInfo.DynamicRatio
		other["group_ratio"] = groupRatio / relayInfo.PriceData.GroupRatioInfo.DynamicRatio
	}
	// 共享流式日志指标，避免 Claude /v1/messages 丢失吐字速度展示。
	AppendStreamMetrics(other, relayInfo, useTimeMs, completionTokens)
	logstore.RecordConsumeLog(ctx, relayInfo.UserId, logstore.RecordConsumeLogParams{
		ChannelId:        relayInfo.ChannelId,
		PromptTokens:     promptTokens,
		CompletionTokens: completionTokens,
		ModelName:        modelName,
		TokenName:        tokenName,
		Quota:            quota,
		Content:          logContent,
		TokenId:          relayInfo.TokenId,
		UseTimeMs:        int(useTimeMs),
		IsStream:         relayInfo.IsStream,
		Group:            relayInfo.UsingGroup,
		Other:            other,
		LogType:          logType,
	})
	return nil
}

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

func PostAudioConsumeQuota(ctx *gin.Context, relayInfo *relaycommon.RelayInfo, usage *shared.Usage, extraContent string) *shared.NookMuxError {

	useTimeMs := time.Since(relayInfo.StartTime).Milliseconds()
	textInputTokens := usage.PromptTokensDetails.TextTokens
	textOutTokens := usage.CompletionTokenDetails.TextTokens

	audioInputTokens := usage.PromptTokensDetails.AudioTokens
	audioOutTokens := usage.CompletionTokenDetails.AudioTokens

	tokenName := ctx.GetString("token_name")
	modelRatio := relayInfo.PriceData.ModelRatio
	groupRatio := relayInfo.PriceData.GroupRatioInfo.GroupRatio
	modelPrice := relayInfo.PriceData.ModelPrice
	usePrice := relayInfo.PriceData.UsePrice
	if _, enabled, err := ApplyContextPricingForUsage(relayInfo.OriginModelName, BuildContextPricingUsage(usage, false), &relayInfo.PriceData); enabled {
		if err != nil {
			log.LogError(ctx, "context pricing failed: "+err.Error())
		} else {
			modelRatio = relayInfo.PriceData.ModelRatio
			groupRatio = relayInfo.PriceData.GroupRatioInfo.GroupRatio
			modelPrice = relayInfo.PriceData.ModelPrice
			usePrice = relayInfo.PriceData.UsePrice
		}
	}
	completionRatio := decimal.NewFromFloat(relayInfo.PriceData.CompletionRatio)
	audioRatio := decimal.NewFromFloat(relayInfo.PriceData.AudioRatio)
	audioCompletionRatio := decimal.NewFromFloat(relayInfo.PriceData.AudioCompletionRatio)

	quotaInfo := QuotaInfo{
		InputDetails: TokenDetails{
			TextTokens:  textInputTokens,
			AudioTokens: audioInputTokens,
		},
		OutputDetails: TokenDetails{
			TextTokens:  textOutTokens,
			AudioTokens: audioOutTokens,
		},
		ModelName:            relayInfo.OriginModelName,
		UsePrice:             usePrice,
		ModelPrice:           modelPrice,
		ModelRatio:           modelRatio,
		GroupRatio:           groupRatio,
		CompletionRatio:      relayInfo.PriceData.CompletionRatio,
		AudioRatio:           relayInfo.PriceData.AudioRatio,
		AudioCompletionRatio: relayInfo.PriceData.AudioCompletionRatio,
	}

	quota := calculateAudioQuota(quotaInfo)

	totalTokens := usage.TotalTokens
	var logContent string
	if !usePrice {
		logContent = fmt.Sprintf("模型倍率 %.2f，补全倍率 %.2f，音频倍率 %.2f，音频补全倍率 %.2f，分组倍率 %.2f",
			modelRatio, completionRatio.InexactFloat64(), audioRatio.InexactFloat64(), audioCompletionRatio.InexactFloat64(), groupRatio)
	} else {
		logContent = fmt.Sprintf("模型价格 %.2f，分组倍率 %.2f", modelPrice, groupRatio)
	}

	// record all the consume log even if quota is 0
	logType := 0 // 0 表示使用默认的 LogTypeConsume
	if totalTokens == 0 {
		if apiErr := NewEmptyUsageRetryError(ctx, relayInfo); apiErr != nil {
			return apiErr
		}
		// in this case, must be some error happened
		// we cannot just return, because we may have to return the pre-consumed quota
		logType = logstore.LogTypeError
		quota = 0
		logContent += "（可能是上游超时）"
		log.LogError(ctx, fmt.Sprintf("total tokens is 0, cannot consume quota, userId %d, channelId %d, "+
			"tokenId %d, model %s， pre-consumed quota %d", relayInfo.UserId, relayInfo.ChannelId, relayInfo.TokenId, relayInfo.OriginModelName, relayInfo.FinalPreConsumedQuota))
	} else {
		userstore.UpdateUserUsedQuotaAndRequestCount(relayInfo.UserId, quota)
		channelstore.UpdateChannelUsedQuota(relayInfo.ChannelId, quota)
	}

	if err := SettleBilling(ctx, relayInfo, quota); err != nil {
		log.LogError(ctx, "error settling billing: "+err.Error())
	}

	logModel := relayInfo.OriginModelName
	if extraContent != "" {
		logContent += ", " + extraContent
	}
	other := GenerateAudioOtherInfo(ctx, relayInfo, usage, modelRatio, groupRatio,
		completionRatio.InexactFloat64(), audioRatio.InexactFloat64(), audioCompletionRatio.InexactFloat64(), modelPrice, relayInfo.PriceData.GroupRatioInfo.GroupSpecialRatio)
	// 记录动态倍率到日志
	if relayInfo.PriceData.GroupRatioInfo.DynamicRatio > 0 {
		other["dynamic_ratio"] = relayInfo.PriceData.GroupRatioInfo.DynamicRatio
		other["group_ratio"] = groupRatio / relayInfo.PriceData.GroupRatioInfo.DynamicRatio
	}
	logstore.RecordConsumeLog(ctx, relayInfo.UserId, logstore.RecordConsumeLogParams{
		ChannelId:        relayInfo.ChannelId,
		PromptTokens:     usage.PromptTokens,
		CompletionTokens: usage.CompletionTokens,
		ModelName:        logModel,
		TokenName:        tokenName,
		Quota:            quota,
		Content:          logContent,
		TokenId:          relayInfo.TokenId,
		UseTimeMs:        int(useTimeMs),
		IsStream:         relayInfo.IsStream,
		Group:            relayInfo.UsingGroup,
		Other:            other,
		LogType:          logType,
	})
	return nil
}

func PreConsumeTokenQuota(ctx *gin.Context, relayInfo *relaycommon.RelayInfo, quota int) error {
	if quota < 0 {
		return errors.New(i18n.T(ctx, i18n.MsgQuotaNegative))
	}

	quotaType := relayInfo.TokenQuotaType

	// 无限额度模式只记录已用额度，不扣减剩余额度
	if quotaType == 0 {
		return tokenstore.UpdateTokenUsedQuota(relayInfo.TokenId, relayInfo.TokenKey, quota)
	}

	// 永久限额模式：检查 TokenQuota（即 RemainQuota）
	if quotaType == 1 && relayInfo.TokenQuota < quota {
		return fmt.Errorf("%s", i18n.T(ctx, i18n.MsgQuotaTokenNotEnough, map[string]any{"TokenQuota": log.FormatQuota(relayInfo.TokenQuota), "NeedQuota": log.FormatQuota(quota)}))
	}

	// 时段限额模式 (2, 3)：预扣减窗口额度
	switch quotaType {
	case 2:
		if relayInfo.TokenQuota < quota {
			return fmt.Errorf("%s", i18n.T(ctx, i18n.MsgQuotaTokenWindowNotEnough, map[string]any{"TokenQuota": log.FormatQuota(relayInfo.TokenQuota), "NeedQuota": log.FormatQuota(quota)}))
		}
		err := tokenstore.DecreaseWindowQuota(relayInfo.TokenId, relayInfo.TokenKey, quota)
		if err != nil {
			return err
		}
	case 3:
		if relayInfo.TokenQuota < quota {
			return fmt.Errorf("%s", i18n.T(ctx, i18n.MsgQuotaTokenNotEnough, map[string]any{"TokenQuota": log.FormatQuota(relayInfo.TokenQuota), "NeedQuota": log.FormatQuota(quota)}))
		}
		err := tokenstore.DecreaseWindowQuota(relayInfo.TokenId, relayInfo.TokenKey, quota)
		if err != nil {
			return err
		}
		err = tokenstore.DecreaseCycleQuota(relayInfo.TokenId, relayInfo.TokenKey, quota)
		if err != nil {
			// 回滚窗口扣减
			if rollbackErr := tokenstore.IncreaseWindowQuota(relayInfo.TokenId, relayInfo.TokenKey, quota); rollbackErr != nil {
				common.SysError(fmt.Sprintf("rollback window quota failed after cycle decrease error: %v (rollback: %v)", err, rollbackErr))
			}
			return err
		}
	default:
		// quotaType == 1 或其他：使用原有的 RemainQuota 扣减
		err := tokenstore.DecreaseTokenQuota(relayInfo.TokenId, relayInfo.TokenKey, quota)
		if err != nil {
			return err
		}
	}
	return nil
}

func PostConsumeQuota(ctx *gin.Context, relayInfo *relaycommon.RelayInfo, quota int, preConsumedQuota int, sendEmail bool) (err error) {
	if relayInfo == nil {
		return errors.New(i18n.T(ctx, i18n.MsgQuotaRelayInfoNil))
	}

	// Wallet
	if quota > 0 {
		err = userstore.DecreaseUserQuota(relayInfo.UserId, quota)
	} else {
		err = userstore.IncreaseUserQuota(relayInfo.UserId, -quota)
	}
	if err != nil {
		return err
	}

	// Token quota deduction based on quota type
	quotaType := relayInfo.TokenQuotaType
	if quota > 0 {
		switch quotaType {
		case 0: // 无限额度，只记录已用额度
			err = tokenstore.UpdateTokenUsedQuota(relayInfo.TokenId, relayInfo.TokenKey, quota)
		case 2:
			err = tokenstore.DecreaseWindowQuota(relayInfo.TokenId, relayInfo.TokenKey, quota)
		case 3:
			if err = tokenstore.DecreaseWindowQuota(relayInfo.TokenId, relayInfo.TokenKey, quota); err == nil {
				err = tokenstore.DecreaseCycleQuota(relayInfo.TokenId, relayInfo.TokenKey, quota)
				if err != nil {
					if rollbackErr := tokenstore.IncreaseWindowQuota(relayInfo.TokenId, relayInfo.TokenKey, quota); rollbackErr != nil {
						common.SysError(fmt.Sprintf("rollback window quota failed after cycle decrease error: %v (rollback: %v)", err, rollbackErr))
					}
				}
			}
		default: // 1 或其他
			err = tokenstore.DecreaseTokenQuota(relayInfo.TokenId, relayInfo.TokenKey, quota)
		}
	} else {
		switch quotaType {
		case 0: // 无限额度，只回滚已用额度
			err = tokenstore.UpdateTokenUsedQuota(relayInfo.TokenId, relayInfo.TokenKey, quota)
		case 2:
			err = tokenstore.IncreaseWindowQuota(relayInfo.TokenId, relayInfo.TokenKey, -quota)
		case 3:
			if err = tokenstore.IncreaseWindowQuota(relayInfo.TokenId, relayInfo.TokenKey, -quota); err == nil {
				err = tokenstore.IncreaseCycleQuota(relayInfo.TokenId, relayInfo.TokenKey, -quota)
				if err != nil {
					if rollbackErr := tokenstore.DecreaseWindowQuota(relayInfo.TokenId, relayInfo.TokenKey, -quota); rollbackErr != nil {
						common.SysError(fmt.Sprintf("rollback window quota failed after cycle increase error: %v (rollback: %v)", err, rollbackErr))
					}
				}
			}
		default: // 1 或其他
			err = tokenstore.IncreaseTokenQuota(relayInfo.TokenId, relayInfo.TokenKey, -quota)
		}
	}
	if err != nil {
		return err
	}

	if sendEmail {
		if (quota + preConsumedQuota) != 0 {
			checkAndSendQuotaNotify(relayInfo, quota, preConsumedQuota)
		}
	}

	return nil
}

func checkAndSendQuotaNotify(relayInfo *relaycommon.RelayInfo, quota int, preConsumedQuota int) {
	runtime.RelayGo(func() {
		userSetting := relayInfo.UserSetting
		threshold := common.QuotaRemindThreshold
		if userSetting.QuotaWarningThreshold != 0 {
			threshold = int(userSetting.QuotaWarningThreshold)
		}

		//noMoreQuota := userCache.Quota-(quota+preConsumedQuota) <= 0
		quotaTooLow := false
		consumeQuota := quota + preConsumedQuota
		if relayInfo.UserQuota-consumeQuota < threshold {
			quotaTooLow = true
		}
		if quotaTooLow {
			prompt := "您的额度即将用尽"
			topUpLink := fmt.Sprintf("%s/console/topup", system.ServerAddress)

			// 根据通知方式生成不同的内容格式
			var content string
			var values []interface{}

			notifyType := userSetting.NotifyType
			if notifyType == "" {
				notifyType = shared.NotifyTypeEmail
			}

			if notifyType == shared.NotifyTypeBark {
				// Bark推送使用简短文本，不支持HTML
				content = "{{value}}，剩余额度：{{value}}，请及时充值"
				values = []interface{}{prompt, log.FormatQuota(relayInfo.UserQuota)}
			} else if notifyType == shared.NotifyTypeGotify {
				content = "{{value}}，当前剩余额度为 {{value}}，请及时充值。"
				values = []interface{}{prompt, log.FormatQuota(relayInfo.UserQuota)}
			} else {
				// 默认内容格式，适用于Email和Webhook（支持HTML）
				content = "{{value}}，当前剩余额度为 {{value}}，为了不影响您的使用，请及时充值。<br/>充值链接：<a href='{{value}}'>{{value}}</a>"
				values = []interface{}{prompt, log.FormatQuota(relayInfo.UserQuota), topUpLink, topUpLink}
			}

			err := notify.NotifyUser(relayInfo.UserId, relayInfo.UserEmail, relayInfo.UserSetting, shared.NewNotify(shared.NotifyTypeQuotaExceed, prompt, content, values))
			if err != nil {
				common.SysError(fmt.Sprintf("failed to send quota notify to user %d: %s", relayInfo.UserId, err.Error()))
			}
		}
	})
}
