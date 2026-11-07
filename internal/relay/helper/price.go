package helper

import (
	"fmt"
	"github.com/NookMux/NookMux/internal/common"
	"github.com/NookMux/NookMux/internal/config/operation"
	"github.com/NookMux/NookMux/internal/config/ratio"
	"github.com/NookMux/NookMux/internal/domain/billing/contract"
	"github.com/NookMux/NookMux/internal/domain/shared"
	"github.com/NookMux/NookMux/internal/infra/log"
	relaycommon "github.com/NookMux/NookMux/internal/relay/common"
	"github.com/NookMux/NookMux/internal/store/channel"
	"github.com/gin-gonic/gin"
)

// https://docs.claude.com/en/docs/build-with-claude/prompt-caching#1-hour-cache-duration
const claudeCacheCreation1hMultiplier = 6 / 3.75

// HandleGroupRatio checks for "auto_group" in the context and updates the group ratio and relayInfo.UsingGroup if present
func HandleGroupRatio(ctx *gin.Context, relayInfo *relaycommon.RelayInfo) contract.GroupRatioInfo {
	groupRatioInfo := contract.GroupRatioInfo{
		GroupRatio:        1.0, // default ratio
		GroupSpecialRatio: -1,
	}

	// check auto group
	autoGroup, exists := ctx.Get("auto_group")
	if exists {
		log.LogDebug(ctx, fmt.Sprintf("final group: %s", autoGroup))
		relayInfo.UsingGroup = autoGroup.(string)
	}

	// check user group special ratio
	userGroupRatio, ok := ratio.GetGroupGroupRatio(relayInfo.UserGroup, relayInfo.UsingGroup)
	if ok {
		// user group special ratio
		groupRatioInfo.GroupSpecialRatio = userGroupRatio
		groupRatioInfo.GroupRatio = userGroupRatio
		groupRatioInfo.HasSpecialRatio = true
	} else {
		// normal group ratio
		groupRatioInfo.GroupRatio = ratio.GetGroupRatio(relayInfo.UsingGroup)
	}

	// 叠加动态倍率
	originalGroupRatio := groupRatioInfo.GroupRatio
	dynamicRatio := channelstore.GetMatchedDynamicRatio(relayInfo.UsingGroup, relayInfo.OriginModelName)
	if dynamicRatio > 0 {
		groupRatioInfo.GroupRatio = originalGroupRatio * dynamicRatio
		groupRatioInfo.DynamicRatio = dynamicRatio
	}

	return groupRatioInfo
}

func ModelPriceHelper(c *gin.Context, info *relaycommon.RelayInfo, promptTokens int, meta *shared.TokenCountMeta) (contract.PriceData, error) {
	modelPrice, usePrice := ratio.GetModelPrice(info.OriginModelName, false)

	groupRatioInfo := HandleGroupRatio(c, info)

	if result, enabled, err := ratio.MatchContextPricingTier(info.OriginModelName, promptTokens); enabled {
		if err != nil {
			return contract.PriceData{}, err
		}
		preConsumedPromptTokens := common.Max(promptTokens, common.PreConsumedQuota)
		preConsumedTokens := float64(preConsumedPromptTokens)
		if meta.MaxTokens != 0 {
			preConsumedTokens += float64(meta.MaxTokens) * result.Prices.CompletionRatio
		}
		preConsumedQuota := int(preConsumedTokens * result.Prices.ModelRatio * groupRatioInfo.GroupRatio)

		freeModel := false
		quotaSetting := operation.GetQuotaSetting()
		tierIsFree := result.Prices.ModelRatio == 0 &&
			result.Prices.CompletionRatio == 0 &&
			result.Prices.CacheRatio == 0 &&
			result.Prices.CacheCreationRatio == 0 &&
			result.Prices.AudioRatio == 0 &&
			result.Prices.AudioCompletionRatio == 0
		if groupRatioInfo.GroupRatio == 0 {
			if quotaSetting.FreeModelPreConsumedQuota <= 0 {
				freeModel = true
			}
		} else if tierIsFree {
			preConsumedQuota = quotaSetting.FreeModelPreConsumedQuota
			if preConsumedQuota <= 0 {
				freeModel = true
			}
		}

		priceData := contract.PriceData{
			FreeModel:         freeModel,
			ModelPrice:        -1,
			GroupRatioInfo:    groupRatioInfo,
			UsePrice:          false,
			QuotaToPreConsume: preConsumedQuota,
		}
		ratio.ApplyContextPricingResult(&priceData, result)
		if common.DebugEnabled {
			println(fmt.Sprintf("model_price_helper context pricing result: %s", priceData.ToSetting()))
		}
		info.PriceData = priceData
		return priceData, nil
	}

	var preConsumedQuota int
	var modelRatio float64
	var completionRatio float64
	var cacheRatio float64
	var cacheCreationRatio float64
	var cacheCreationRatio5m float64
	var cacheCreationRatio1h float64
	var audioRatio float64
	var audioCompletionRatio float64
	var freeModel bool
	if !usePrice {
		preConsumedTokens := common.Max(promptTokens, common.PreConsumedQuota)
		if meta.MaxTokens != 0 {
			preConsumedTokens += meta.MaxTokens
		}
		var success bool
		var matchName string
		modelRatio, success, matchName = ratio.GetModelRatio(info.OriginModelName)
		if !success {
			return contract.PriceData{}, fmt.Errorf("模型 %s 倍率或价格未配置，请联系管理员设置；Model %s ratio or price not set, please contact administrator to configure", matchName, matchName)
		}
		completionRatio = ratio.GetCompletionRatio(info.OriginModelName)
		cacheRatio, _ = ratio.GetCacheRatio(info.OriginModelName)
		cacheCreationRatio, _ = ratio.GetCreateCacheRatio(info.OriginModelName)
		cacheCreationRatio5m = cacheCreationRatio
		// 固定1h和5min缓存写入价格的比例
		cacheCreationRatio1h = cacheCreationRatio * claudeCacheCreation1hMultiplier
		audioRatio = ratio.GetAudioRatio(info.OriginModelName)
		audioCompletionRatio = ratio.GetAudioCompletionRatio(info.OriginModelName)
		ratio := modelRatio * groupRatioInfo.GroupRatio
		preConsumedQuota = common.QuotaFromFloat(float64(preConsumedTokens) * ratio)
	} else {
		if meta.ImagePriceRatio != 0 {
			modelPrice = modelPrice * meta.ImagePriceRatio
		}
		preConsumedQuota = common.QuotaFromFloat(modelPrice * common.QuotaPerUnit * groupRatioInfo.GroupRatio)
	}

	// check free model pre-consume setting
	quotaSetting := operation.GetQuotaSetting()
	isFreeModel := false
	if usePrice {
		if modelPrice == 0 && groupRatioInfo.GroupRatio > 0 {
			isFreeModel = true
		}
	} else {
		if modelRatio == 0 && groupRatioInfo.GroupRatio > 0 {
			isFreeModel = true
		}
	}

	// GroupRatio == 0 means the group itself is free; preConsumedQuota is naturally 0
	if groupRatioInfo.GroupRatio == 0 {
		// keep preConsumedQuota as-is (already 0 from multiplication)
		if quotaSetting.FreeModelPreConsumedQuota <= 0 {
			freeModel = true
		}
	} else if isFreeModel {
		preConsumedQuota = quotaSetting.FreeModelPreConsumedQuota
		if preConsumedQuota <= 0 {
			freeModel = true
		}
	}

	priceData := contract.PriceData{
		FreeModel:            freeModel,
		ModelPrice:           modelPrice,
		ModelRatio:           modelRatio,
		CompletionRatio:      completionRatio,
		GroupRatioInfo:       groupRatioInfo,
		UsePrice:             usePrice,
		CacheRatio:           cacheRatio,
		AudioRatio:           audioRatio,
		AudioCompletionRatio: audioCompletionRatio,
		CacheCreationRatio:   cacheCreationRatio,
		CacheCreation5mRatio: cacheCreationRatio5m,
		CacheCreation1hRatio: cacheCreationRatio1h,
		QuotaToPreConsume:    preConsumedQuota,
	}

	if common.DebugEnabled {
		println(fmt.Sprintf("model_price_helper result: %s", priceData.ToSetting()))
	}
	info.PriceData = priceData
	return priceData, nil
}

// ModelPriceHelperPerCall 按次计费的 PriceHelper (MJ、Task)
func ModelPriceHelperPerCall(c *gin.Context, info *relaycommon.RelayInfo) contract.PerCallPriceData {
	groupRatioInfo := HandleGroupRatio(c, info)

	modelPrice, success := ratio.GetModelPrice(info.OriginModelName, true)
	// 如果没有配置价格，则使用默认价格
	if !success {
		defaultPrice, ok := ratio.GetDefaultModelPriceMap()[info.OriginModelName]
		if !ok {
			modelPrice = 0.1
		} else {
			modelPrice = defaultPrice
		}
	}
	quota := int(modelPrice * common.QuotaPerUnit * groupRatioInfo.GroupRatio)
	priceData := contract.PerCallPriceData{
		ModelPrice:     modelPrice,
		Quota:          quota,
		GroupRatioInfo: groupRatioInfo,
	}
	return priceData
}

func ContainPriceOrRatio(modelName string) bool {
	_, ok := ratio.GetModelPrice(modelName, false)
	if ok {
		return true
	}
	_, ok, _ = ratio.GetModelRatio(modelName)
	return ok
}
