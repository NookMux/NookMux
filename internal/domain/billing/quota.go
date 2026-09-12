package billing

import (
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/NookMux/NookMux/internal/common"
	"github.com/NookMux/NookMux/internal/config/system"
	"github.com/NookMux/NookMux/internal/domain/billing/contract"
	"github.com/NookMux/NookMux/internal/domain/channel/constant"
	"github.com/NookMux/NookMux/internal/domain/shared"
	"github.com/NookMux/NookMux/internal/httpapi"
	"github.com/NookMux/NookMux/internal/i18n"
	"github.com/NookMux/NookMux/internal/infra/log"
	notify "github.com/NookMux/NookMux/internal/infra/notify"
	"github.com/NookMux/NookMux/internal/infra/runtime"
	relaycommon "github.com/NookMux/NookMux/internal/relay/common"
	channelstore "github.com/NookMux/NookMux/internal/store/channel"
	logstore "github.com/NookMux/NookMux/internal/store/log"
	tokenstore "github.com/NookMux/NookMux/internal/store/token"
	userstore "github.com/NookMux/NookMux/internal/store/user"
	"github.com/gin-gonic/gin"
	"github.com/shopspring/decimal"
)

// NewEmptyUsageRetryError returns a retryable upstream error when native-format responses contain no billing usage.
func NewEmptyUsageRetryError(ctx *gin.Context, relayInfo *relaycommon.RelayInfo) *shared.NookMuxError {
	if relayInfo == nil || len(relayInfo.RequestConversionChain) > 1 {
		return nil
	}
	return shared.NewOpenAIError(errors.New(i18n.T(ctx, i18n.MsgQuotaEmptyUsage)), shared.ErrorCodeBadResponse, http.StatusBadGateway)
}

// normalizationFailedError 返回归一化失败的显式错误：响应已交付、不可重试，
// 计费与落库一并中止（PRD 阶段 2：归一化失败是独立可观测原因）；
// 具体原因由调用点先记 cause=normalization_failed 日志。
func normalizationFailedError(ctx *gin.Context) *shared.NookMuxError {
	return shared.NewOpenAIError(
		fmt.Errorf("%s", i18n.T(ctx, i18n.MsgQuotaBillingNormalizationFailed)),
		shared.ErrorCodeBadResponse, http.StatusBadGateway, shared.ErrOptionWithSkipRetry())
}

func billingQuotaFailedError(ctx *gin.Context, cause error) *shared.NookMuxError {
	if errors.Is(cause, ErrBillingPriceConfigMissing) {
		return shared.NewOpenAIError(
			fmt.Errorf("%s", billingQuotaFailureMessage(ctx, cause)),
			shared.ErrorCodeBadResponse, http.StatusBadGateway, shared.ErrOptionWithSkipRetry())
	}
	return normalizationFailedError(ctx)
}

func billingQuotaFailureCause(err error) string {
	if errors.Is(err, ErrBillingPriceConfigMissing) {
		return "billing_config_missing"
	}
	return "normalization_failed"
}

func billingQuotaFailureMessage(ctx *gin.Context, err error) string {
	if errors.Is(err, ErrBillingPriceConfigMissing) {
		return i18n.T(ctx, i18n.MsgQuotaBillingPriceConfigFailed)
	}
	return i18n.T(ctx, i18n.MsgQuotaBillingNormalizationFailed)
}

// PreWssConsumeQuota realtime 会话按事件增量实扣：按归一化用量（含缓存
// 读取）计算本轮增量额度并实扣（计费 PRD 阶段 2）。UsePrice（按次价格）与
// ratio 两种模式统一走本入口：按次计划下每个 response.done 事件收一次按次
// 费（P1-4：不再整场免结算）。实扣成功才累计 WssEventConsumedQuota，供
// PostWssConsumeQuota 收尾补差对账。
func PreWssConsumeQuota(ctx *gin.Context, relayInfo *relaycommon.RelayInfo, usage *shared.RealtimeUsage) error {
	userQuota, err := userstore.GetUserQuota(relayInfo.UserId, false)
	if err != nil {
		return err
	}

	modelName := relayInfo.OriginModelName
	bu, warnings, normErr := BuildRealtimeBillingUsage(relayInfo.UsageSource, usage)
	if normErr != nil {
		log.LogError(ctx, "billing normalization failed (cause=normalization_failed): "+normErr.Error())
		return fmt.Errorf("%s", i18n.T(ctx, i18n.MsgQuotaBillingNormalizationFailed))
	}
	for _, warning := range warnings {
		log.LogWarn(ctx, "billing_details anomaly: "+warning)
	}
	if _, enabled, err := ApplyContextPricingForBillingUsage(modelName, bu, &relayInfo.PriceData); enabled && err != nil {
		return err
	}

	quota, _, _, err := normalizedRealtimeQuota(bu, relayInfo.OriginModelName, relayInfo.PriceData, relayInfo)
	if err != nil {
		log.LogError(ctx, "billing normalized quota failed (cause="+billingQuotaFailureCause(err)+"): "+err.Error())
		return fmt.Errorf("%s", billingQuotaFailureMessage(ctx, err))
	}

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
	relayInfo.WssEventConsumedQuota += quota
	log.LogInfo(ctx, "realtime streaming consume quota success, quota: "+fmt.Sprintf("%d", quota))
	return nil
}

// PostWssConsumeQuota realtime 会话收尾记账：quota 由归一化用量计算（与
// PreWssConsumeQuota 同一线性公式，预扣即实扣），消费日志记录汇总用量。
func PostWssConsumeQuota(ctx *gin.Context, relayInfo *relaycommon.RelayInfo, modelName string,
	usage *shared.RealtimeUsage, extraContent string) *shared.NookMuxError {

	useTimeMs := time.Since(relayInfo.StartTime).Milliseconds()
	recordWssBillingFailure := func(causeErr error) *shared.NookMuxError {
		// 会话内已按事件增量实扣的资金必须有日志可对账（预扣不走
		// BillingSession，退款 defer 只覆盖会话前预扣部分），落一条
		// LogTypeError 日志后再返回显式错误；失败原因必须区分计费配置
		// 缺失与归一化失败，不能都归因归一化（PRD：三类可观测原因）。
		cause := billingQuotaFailureCause(causeErr)
		prefix := "realtime 归一化失败"
		if cause == "billing_config_missing" {
			prefix = "realtime 计费配置缺失"
		}
		logstore.RecordConsumeLog(ctx, relayInfo.UserId, logstore.RecordConsumeLogParams{
			ChannelId:    relayInfo.ChannelId,
			ModelName:    modelName,
			TokenName:    ctx.GetString("token_name"),
			Quota:        0,
			Content:      prefix + "，已按事件预扣的资金见 pre-consumed quota（" + causeErr.Error() + "）",
			TokenId:      relayInfo.TokenId,
			UseTimeMs:    int(useTimeMs),
			IsStream:     relayInfo.IsStream,
			Group:        relayInfo.UsingGroup,
			Other:        map[string]interface{}{"ws": true, "billing_cause": cause, "pre_consumed_quota": relayInfo.FinalPreConsumedQuota},
			LogType:      logstore.LogTypeError,
			PromptTokens: usage.InputTokens,
		})
		log.LogError(ctx, "billing wss settlement failed (cause="+cause+"): "+causeErr.Error()+
			fmt.Sprintf(", preConsumedQuota=%d", relayInfo.FinalPreConsumedQuota))
		return billingQuotaFailedError(ctx, causeErr)
	}
	bu, warnings, normErr := BuildRealtimeBillingUsage(relayInfo.UsageSource, usage)
	if normErr != nil {
		return recordWssBillingFailure(normErr)
	}
	for _, warning := range warnings {
		log.LogWarn(ctx, "billing_details anomaly: "+warning)
	}
	if _, enabled, err := ApplyContextPricingForBillingUsage(modelName, bu, &relayInfo.PriceData); enabled {
		if err != nil {
			log.LogError(ctx, "context pricing failed (cause=billing_config_missing): "+err.Error())
		}
	}

	quota, quotaLines, quotaSnapshot, quotaErr := normalizedRealtimeQuota(bu, modelName, relayInfo.PriceData, relayInfo)
	if quotaErr != nil {
		return recordWssBillingFailure(quotaErr)
	}
	// 按次价（UsePrice）为固定价、与 token 用量无关：事件级已按每个
	// response.done 实扣一份（WssEventConsumedQuota = 份数 × 当时单价），
	// 收尾 finalQuota 必须采用同一口径；若沿用对汇总用量重算的单份按次价，
	// 补差会把 N−1 份退还，整场会话净扣只剩一份（缺陷 34）。
	if relayInfo.PriceData.UsePrice {
		quota = relayInfo.WssEventConsumedQuota
	}
	var billingDetailsJSON string
	if !httpapi.GetContextKeyBool(ctx, common.ContextKeyLocalCountTokens) && usage.TotalTokens != 0 {
		payload, serializeErr := SerializeBillingUsage(bu)
		if serializeErr != nil {
			return recordWssBillingFailure(fmt.Errorf("billing_details serialization failed: %w", serializeErr))
		}
		billingDetailsJSON = payload
	}

	// 收尾补差失败的可观测路径（P1-4/P1-12）：事件已实扣资金与补差明细必须
	// 有错误日志可对账，不写伪成功消费日志；错误不可重试（整场会话已交付，
	// 重试会重复计费）。
	recordWssReconcileFailure := func(causeErr error) *shared.NookMuxError {
		eventConsumed := relayInfo.WssEventConsumedQuota
		reconcileOther := map[string]interface{}{
			"ws":                 true,
			"billing_cause":      "closing_reconcile_failed",
			"pre_consumed_quota": eventConsumed,
			"reconcile_delta":    quota - eventConsumed,
			"settle_quota":       quota,
		}
		AppendBillingPriceSnapshot(reconcileOther, &BillingQuotaResult{PriceSnapshot: quotaSnapshot})
		logstore.RecordConsumeLog(ctx, relayInfo.UserId, logstore.RecordConsumeLogParams{
			ChannelId:      relayInfo.ChannelId,
			ModelName:      modelName,
			TokenName:      ctx.GetString("token_name"),
			Quota:          quota,
			Content:        "realtime 收尾补差失败（" + causeErr.Error() + "），quota 为重算值，资金实际净扣以 pre_consumed_quota 为准",
			TokenId:        relayInfo.TokenId,
			UseTimeMs:      int(useTimeMs),
			IsStream:       relayInfo.IsStream,
			Group:          relayInfo.UsingGroup,
			Other:          reconcileOther,
			LogType:        logstore.LogTypeError,
			PromptTokens:   usage.InputTokens,
			BillingDetails: billingDetailsJSON,
		})
		log.LogError(ctx, fmt.Sprintf("billing wss closing reconcile failed: %s, eventConsumedQuota=%d, settleQuota=%d",
			causeErr.Error(), eventConsumed, quota))
		return shared.NewError(causeErr, shared.ErrorCodeUpdateDataError, shared.ErrOptionWithSkipRetry())
	}

	totalTokens := usage.TotalTokens
	var logContent string
	if !relayInfo.PriceData.UsePrice {
		logContent = fmt.Sprintf("模型倍率 %.2f，补全倍率 %.2f，音频倍率 %.2f，音频补全倍率 %.2f，分组倍率 %.2f",
			relayInfo.PriceData.ModelRatio, relayInfo.PriceData.CompletionRatio, relayInfo.PriceData.AudioRatio, relayInfo.PriceData.AudioCompletionRatio, relayInfo.PriceData.GroupRatioInfo.GroupRatio)
	} else {
		logContent = fmt.Sprintf("模型价格 %.2f，分组倍率 %.2f", relayInfo.PriceData.ModelPrice, relayInfo.PriceData.GroupRatioInfo.GroupRatio)
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
		// 收尾补差（P1-4 模型 C）：事件按当时档位与价格逐笔实扣了
		// WssEventConsumedQuota，收尾用最终 PriceData 对汇总用量重算
		// finalQuota，差额多退少补；多事件取整与会话中改价的偏差在此对齐，
		// 保证日志 Quota = finalQuota = 实际净扣款。
		if reconcileErr := reconcileWssClosingQuota(ctx, relayInfo, relayInfo.WssEventConsumedQuota, quota); reconcileErr != nil {
			return recordWssReconcileFailure(reconcileErr)
		}
		userstore.UpdateUserUsedQuotaAndRequestCount(relayInfo.UserId, quota)
		channelstore.UpdateChannelUsedQuota(relayInfo.ChannelId, quota)
	}

	// Per-event realtime charges have already settled funding. A surviving
	// request-level BillingSession represents only the initial preconsume and
	// must settle at zero so it is released without charging the summary again.
	if relayInfo.Billing != nil {
		if err := relayInfo.Billing.Settle(0); err != nil {
			log.LogError(ctx, "error settling realtime billing session: "+err.Error())
		}
	}

	logModel := modelName
	if extraContent != "" {
		logContent += ", " + extraContent
	}
	other := GenerateWssOtherInfo(ctx, relayInfo, relayInfo.PriceData.ModelRatio, relayInfo.PriceData.GroupRatioInfo.GroupRatio,
		relayInfo.PriceData.CompletionRatio, relayInfo.PriceData.AudioRatio, relayInfo.PriceData.AudioCompletionRatio, relayInfo.PriceData.ModelPrice, relayInfo.PriceData.GroupRatioInfo.GroupSpecialRatio)
	// 记录动态倍率到日志
	if relayInfo.PriceData.GroupRatioInfo.DynamicRatio > 0 {
		other["dynamic_ratio"] = relayInfo.PriceData.GroupRatioInfo.DynamicRatio
		// group_ratio 记录原始分组倍率（不含动态倍率）
		other["group_ratio"] = relayInfo.PriceData.GroupRatioInfo.GroupRatio / relayInfo.PriceData.GroupRatioInfo.DynamicRatio
	}
	AppendBillingPriceSnapshot(other, &BillingQuotaResult{PriceSnapshot: quotaSnapshot})
	if totalTokens != 0 {
		// 事件实扣与补差快照：日志 quota、事件实扣累计、补差值三者可核对。
		other["pre_consumed_quota"] = relayInfo.WssEventConsumedQuota
		other["reconcile_delta"] = quota - relayInfo.WssEventConsumedQuota
	}
	logstore.RecordConsumeLog(ctx, relayInfo.UserId, logstore.RecordConsumeLogParams{
		ChannelId:        relayInfo.ChannelId,
		PromptTokens:     usage.InputTokens,
		CompletionTokens: usage.OutputTokens,
		ModelName:        logModel,
		TokenName:        ctx.GetString("token_name"),
		Quota:            quota,
		Content:          logContent,
		TokenId:          relayInfo.TokenId,
		UseTimeMs:        int(useTimeMs),
		IsStream:         relayInfo.IsStream,
		Group:            relayInfo.UsingGroup,
		Other:            other,
		LogType:          logType,
		BillingDetails:   billingDetailsJSON,
	})

	// 影子对拍：旧公式按 text/audio 明细计算，不含缓存读取。
	if totalTokens != 0 {
		legacyQuota := legacyAudioQuota(QuotaInfo{
			InputDetails: TokenDetails{
				TextTokens:  usage.InputTokenDetails.TextTokens,
				AudioTokens: usage.InputTokenDetails.AudioTokens,
			},
			OutputDetails: TokenDetails{
				TextTokens:  usage.OutputTokenDetails.TextTokens,
				AudioTokens: usage.OutputTokenDetails.AudioTokens,
			},
			ModelName:            modelName,
			UsePrice:             relayInfo.PriceData.UsePrice,
			ModelPrice:           relayInfo.PriceData.ModelPrice,
			ModelRatio:           relayInfo.PriceData.ModelRatio,
			GroupRatio:           relayInfo.PriceData.GroupRatioInfo.GroupRatio,
			CompletionRatio:      relayInfo.PriceData.CompletionRatio,
			AudioRatio:           relayInfo.PriceData.AudioRatio,
			AudioCompletionRatio: relayInfo.PriceData.AudioCompletionRatio,
		})
		if legacyQuota != quota {
			reportBillingShadowMismatch(ctx, "wss", relayInfo, legacyQuota, quota, bu, quotaLines, false, quotaSnapshot)
		}
	}
	return nil
}

// reconcileWssClosingQuota 对齐事件实扣与收尾重算的差额（P1-4 模型 C：
// 事件实扣 + 收尾补差）。diff>0 补扣前先做余额检查（与 PreWssConsumeQuota
// 的事件级检查同口径），diff<0 直接走 PostConsumeQuota 的退款路径；任何
// 失败都显式返回，不允许静默放弃差额。
func reconcileWssClosingQuota(ctx *gin.Context, relayInfo *relaycommon.RelayInfo, eventConsumedQuota, settleQuota int) error {
	diff := settleQuota - eventConsumedQuota
	if diff == 0 {
		return nil
	}
	if diff > 0 {
		userQuota, err := userstore.GetUserQuota(relayInfo.UserId, false)
		if err != nil {
			return err
		}
		if userQuota < diff {
			return fmt.Errorf("%s", i18n.T(ctx, i18n.MsgQuotaUserNotEnough, map[string]any{"UserQuota": log.FormatQuota(userQuota), "NeedQuota": log.FormatQuota(diff)}))
		}
	}
	return PostConsumeQuota(ctx, relayInfo, diff, 0, false)
}

// normalizedRealtimeQuota 用归一化公式计算 realtime/WSS 额度，复刻旧
// calculateAudioQuota 的最低消费与取整口径（UsePrice 截断、按量四舍五入）。
func normalizedRealtimeQuota(bu *BillingUsage, modelName string, priceData contract.PriceData, relayInfo *relaycommon.RelayInfo) (int, []BillingQuotaLine, *BillingPriceSnapshot, error) {
	result, err := CalculateNormalizedQuotaForRelay(bu, priceData, AudioPricingRatioModel, modelName, relayInfo)
	if err != nil {
		return 0, nil, nil, err
	}
	if result.UsePrice {
		return roundEntryQuota(result.TokenTotal, result, false), result.Lines, result.PriceSnapshot, nil
	}
	ratio := decimal.NewFromFloat(priceData.ModelRatio).Mul(decimal.NewFromFloat(priceData.GroupRatioInfo.GroupRatio))
	total := result.TokenTotal
	// 最低消费仅约束旧 ratio 投影；显式价格表的 0 价格结算不被抬额。
	if IsLegacyPriceSettlement(result) && !ratio.IsZero() && total.LessThanOrEqual(decimal.Zero) {
		total = decimal.NewFromInt(1)
	}
	return roundEntryQuota(total, result, false), result.Lines, result.PriceSnapshot, nil
}

// roundEntryQuota preserves each pre-stage-4 entry point's rounding behavior
// for legacy projections: the Claude entry truncated the final decimal in both
// per-price and ratio modes (legacyTruncates), while audio/realtime truncated
// only per-price plans and rounded token totals half-up. Explicit price tables
// own their configured final rounding mode, even when that mode differs from
// the legacy entry point.
func roundEntryQuota(value decimal.Decimal, result BillingQuotaResult, legacyTruncates bool) int {
	if result.PriceSnapshot != nil &&
		result.PriceSnapshot.Source == contract.PricePlanSourceExplicit {
		return RoundBillingQuota(value, result.RoundingMode)
	}
	if legacyTruncates || result.UsePrice {
		return int(value.IntPart())
	}
	return int(value.Round(0).IntPart())
}

// PostClaudeConsumeQuota Claude Messages 语义消费入口：quota 由归一化用量按
// PRD 3.4 公式计算（普通输入 × 输入价 + 缓存读取/写入 × 缓存单价 + 输出 ×
// 补全倍率）。OpenRouter 专属的 cost 反推缓存写入减法已删除（PRD 阶段 2）。
func PostClaudeConsumeQuota(ctx *gin.Context, relayInfo *relaycommon.RelayInfo, usage *shared.Usage) *shared.NookMuxError {

	useTimeMs := time.Since(relayInfo.StartTime).Milliseconds()
	promptTokens := usage.PromptTokens
	completionTokens := usage.CompletionTokens
	modelName := relayInfo.OriginModelName
	openRouter := relayInfo.ChannelType == constant.ChannelTypeOpenRouter

	bu, warnings, normErr := BuildBillingUsage(relayInfo.UsageSource, usage, relayInfo.UsageGeminiMetadata)
	if normErr != nil {
		log.LogError(ctx, "billing normalization failed (cause=normalization_failed): "+normErr.Error())
		return normalizationFailedError(ctx)
	}
	for _, warning := range warnings {
		log.LogWarn(ctx, "billing_details anomaly: "+warning)
	}
	if _, enabled, err := ApplyContextPricingForBillingUsage(modelName, bu, &relayInfo.PriceData); enabled {
		if err != nil {
			log.LogError(ctx, "context pricing failed (cause=billing_config_missing): "+err.Error())
		}
	}

	quotaResult, quotaErr := CalculateNormalizedQuotaForRelay(bu, relayInfo.PriceData, AudioPricingAbsolute, modelName, relayInfo)
	if quotaErr != nil {
		log.LogError(ctx, "billing normalized quota failed (cause="+billingQuotaFailureCause(quotaErr)+"): "+quotaErr.Error())
		return billingQuotaFailedError(ctx, quotaErr)
	}
	// Claude 语义映射没有标准音频字段（PRD 3.1），Absolute 模式下音频输入
	// 独立费用不应出现；出现即归一化或入口装配 bug，显式失败而不是静默漏计。
	if !quotaResult.AudioInputQuota.IsZero() {
		log.LogError(ctx, "billing claude entry produced unexpected audio input fee (cause=normalization_failed)")
		return normalizationFailedError(ctx)
	}
	// 旧口径：最低消费判断看模型倍率，最终 quota 截断（UsePrice 与按量一致）。
	// UsePrice 分支不加最低消费（旧代码 UsePrice 时 ModelRatio 恒为 0，条件不
	// 可达；显式守卫防止未来构造出 UsePrice=true 且 ModelRatio≠0 的 PriceData
	// 时误抬额）。最低消费是旧 ratio 配置的安全网，显式价格表拥有自己的定价
	// （含合法的 0 价格计划），不得被抬额。
	quota := roundEntryQuota(quotaResult.TokenTotal, quotaResult, true)
	if !quotaResult.UsePrice && IsLegacyPriceSettlement(quotaResult) && relayInfo.PriceData.ModelRatio != 0 && quotaResult.TokenTotal.LessThanOrEqual(decimal.Zero) {
		quota = 1
	}

	var billingDetailsJSON string
	if !httpapi.GetContextKeyBool(ctx, common.ContextKeyLocalCountTokens) && promptTokens+completionTokens != 0 {
		payload, serializeErr := SerializeBillingUsage(bu)
		if serializeErr != nil {
			log.LogError(ctx, "billing_details serialization failed (cause=billing_details_serialization_failed): "+serializeErr.Error())
			return normalizationFailedError(ctx)
		}
		billingDetailsJSON = payload
	}

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
		userstore.UpdateUserUsedQuotaAndRequestCount(relayInfo.UserId, int(quota))
		channelstore.UpdateChannelUsedQuota(relayInfo.ChannelId, int(quota))
	}

	if err := SettleBilling(ctx, relayInfo, int(quota)); err != nil {
		log.LogError(ctx, "error settling billing: "+err.Error())
	}

	priceData := relayInfo.PriceData
	other := GenerateClaudeOtherInfo(ctx, relayInfo, priceData.ModelRatio, priceData.GroupRatioInfo.GroupRatio, priceData.CompletionRatio,
		priceData.CacheRatio,
		priceData.CacheCreationRatio,
		priceData.CacheCreation5mRatio,
		priceData.CacheCreation1hRatio,
		priceData.ModelPrice, priceData.GroupRatioInfo.GroupSpecialRatio)
	// 记录动态倍率到日志
	if priceData.GroupRatioInfo.DynamicRatio > 0 {
		other["dynamic_ratio"] = priceData.GroupRatioInfo.DynamicRatio
		other["group_ratio"] = priceData.GroupRatioInfo.GroupRatio / priceData.GroupRatioInfo.DynamicRatio
	}
	AppendBillingPriceSnapshot(other, &quotaResult)
	// 共享流式日志指标，避免 Claude /v1/messages 丢失吐字速度展示。
	AppendStreamMetrics(other, relayInfo, useTimeMs, completionTokens)
	logstore.RecordConsumeLog(ctx, relayInfo.UserId, logstore.RecordConsumeLogParams{
		ChannelId:        relayInfo.ChannelId,
		PromptTokens:     promptTokens,
		CompletionTokens: completionTokens,
		ModelName:        modelName,
		TokenName:        ctx.GetString("token_name"),
		Quota:            int(quota),
		Content:          logContent,
		TokenId:          relayInfo.TokenId,
		UseTimeMs:        int(useTimeMs),
		IsStream:         relayInfo.IsStream,
		Group:            relayInfo.UsingGroup,
		Other:            other,
		LogType:          logType,
		BillingDetails:   billingDetailsJSON,
	})

	// 影子对拍（迁移期）：旧公式对原生 Claude 缓存读写按基础价重复计费、
	// OpenRouter 走 cost 反推减法，差异分类见 classifyShadowDiff。
	if totalTokens != 0 {
		legacyQuota := legacyClaudeQuota(usage, openRouter, relayInfo.PriceData, modelName)
		if legacyQuota != int(quota) {
			reportBillingShadowMismatch(ctx, "claude", relayInfo, legacyQuota, int(quota), bu, quotaResult.Lines, openRouter, quotaResult.PriceSnapshot)
		}
	}
	return nil
}

// PostAudioConsumeQuota 音频模态消费入口（Chat 音频模型、TTS/STT）：quota 由
// 归一化用量计算，音频输入/输出按 AudioRatio/AudioCompletionRatio 差异化计价，
// 缓存读取按缓存单价计费（旧公式静默漏计，PRD 3.4 修正）。
func PostAudioConsumeQuota(ctx *gin.Context, relayInfo *relaycommon.RelayInfo, usage *shared.Usage, extraContent string) *shared.NookMuxError {

	useTimeMs := time.Since(relayInfo.StartTime).Milliseconds()
	modelName := relayInfo.OriginModelName

	bu, warnings, normErr := BuildBillingUsage(relayInfo.UsageSource, usage, relayInfo.UsageGeminiMetadata)
	if normErr != nil {
		log.LogError(ctx, "billing normalization failed (cause=normalization_failed): "+normErr.Error())
		return normalizationFailedError(ctx)
	}
	for _, warning := range warnings {
		log.LogWarn(ctx, "billing_details anomaly: "+warning)
	}
	if _, enabled, err := ApplyContextPricingForBillingUsage(modelName, bu, &relayInfo.PriceData); enabled {
		if err != nil {
			log.LogError(ctx, "context pricing failed (cause=billing_config_missing): "+err.Error())
		}
	}
	priceData := relayInfo.PriceData

	quotaResult, quotaErr := CalculateNormalizedQuotaForRelay(bu, priceData, AudioPricingRatioModel, modelName, relayInfo)
	if quotaErr != nil {
		log.LogError(ctx, "billing normalized quota failed (cause="+billingQuotaFailureCause(quotaErr)+"): "+quotaErr.Error())
		return billingQuotaFailedError(ctx, quotaErr)
	}
	var billingDetailsJSON string
	if !httpapi.GetContextKeyBool(ctx, common.ContextKeyLocalCountTokens) && usage.TotalTokens != 0 {
		payload, serializeErr := SerializeBillingUsage(bu)
		if serializeErr != nil {
			log.LogError(ctx, "billing_details serialization failed (cause=billing_details_serialization_failed): "+serializeErr.Error())
			return normalizationFailedError(ctx)
		}
		billingDetailsJSON = payload
	}
	var quota int
	if quotaResult.UsePrice {
		quota = roundEntryQuota(quotaResult.TokenTotal, quotaResult, false)
	} else {
		ratio := decimal.NewFromFloat(priceData.ModelRatio).Mul(decimal.NewFromFloat(priceData.GroupRatioInfo.GroupRatio))
		total := quotaResult.TokenTotal
		// 最低消费仅约束旧 ratio 投影；显式价格表的 0 价格结算不被抬额。
		if IsLegacyPriceSettlement(quotaResult) && !ratio.IsZero() && total.LessThanOrEqual(decimal.Zero) {
			total = decimal.NewFromInt(1)
		}
		quota = roundEntryQuota(total, quotaResult, false)
	}

	totalTokens := usage.TotalTokens
	var logContent string
	if !priceData.UsePrice {
		logContent = fmt.Sprintf("模型倍率 %.2f，补全倍率 %.2f，音频倍率 %.2f，音频补全倍率 %.2f，分组倍率 %.2f",
			priceData.ModelRatio, priceData.CompletionRatio, priceData.AudioRatio, priceData.AudioCompletionRatio, priceData.GroupRatioInfo.GroupRatio)
	} else {
		logContent = fmt.Sprintf("模型价格 %.2f，分组倍率 %.2f", priceData.ModelPrice, priceData.GroupRatioInfo.GroupRatio)
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

	if err := SettleBilling(ctx, relayInfo, quota); err != nil {
		log.LogError(ctx, "error settling billing: "+err.Error())
	}

	logModel := modelName
	if extraContent != "" {
		logContent += ", " + extraContent
	}
	other := GenerateAudioOtherInfo(ctx, relayInfo, priceData.ModelRatio, priceData.GroupRatioInfo.GroupRatio,
		priceData.CompletionRatio, priceData.AudioRatio, priceData.AudioCompletionRatio, priceData.ModelPrice, priceData.GroupRatioInfo.GroupSpecialRatio)
	// 记录动态倍率到日志
	if priceData.GroupRatioInfo.DynamicRatio > 0 {
		other["dynamic_ratio"] = priceData.GroupRatioInfo.DynamicRatio
		other["group_ratio"] = priceData.GroupRatioInfo.GroupRatio / priceData.GroupRatioInfo.DynamicRatio
	}
	AppendBillingPriceSnapshot(other, &quotaResult)
	logstore.RecordConsumeLog(ctx, relayInfo.UserId, logstore.RecordConsumeLogParams{
		ChannelId:        relayInfo.ChannelId,
		PromptTokens:     usage.PromptTokens,
		CompletionTokens: usage.CompletionTokens,
		ModelName:        logModel,
		TokenName:        ctx.GetString("token_name"),
		Quota:            quota,
		Content:          logContent,
		TokenId:          relayInfo.TokenId,
		UseTimeMs:        int(useTimeMs),
		IsStream:         relayInfo.IsStream,
		Group:            relayInfo.UsingGroup,
		Other:            other,
		LogType:          logType,
		BillingDetails:   billingDetailsJSON,
	})

	// 影子对拍（迁移期）：旧公式按官方 text 明细计费且漏计缓存读取。
	if totalTokens != 0 {
		legacyQuota := legacyAudioQuota(QuotaInfo{
			InputDetails: TokenDetails{
				TextTokens:  usage.PromptTokensDetails.TextTokens,
				AudioTokens: usage.PromptTokensDetails.AudioTokens,
			},
			OutputDetails: TokenDetails{
				TextTokens:  usage.CompletionTokenDetails.TextTokens,
				AudioTokens: usage.CompletionTokenDetails.AudioTokens,
			},
			ModelName:            modelName,
			UsePrice:             priceData.UsePrice,
			ModelPrice:           priceData.ModelPrice,
			ModelRatio:           priceData.ModelRatio,
			GroupRatio:           priceData.GroupRatioInfo.GroupRatio,
			CompletionRatio:      priceData.CompletionRatio,
			AudioRatio:           priceData.AudioRatio,
			AudioCompletionRatio: priceData.AudioCompletionRatio,
		})
		if legacyQuota != quota {
			reportBillingShadowMismatch(ctx, "audio", relayInfo, legacyQuota, quota, bu, quotaResult.Lines, false, quotaResult.PriceSnapshot)
		}
	}
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
		err = userstore.IncreaseUserQuota(relayInfo.UserId, -quota, false)
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
