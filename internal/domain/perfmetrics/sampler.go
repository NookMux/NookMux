// Package perfmetrics 承载中继请求性能采样的成败分类与样本落桶，
// 采样入口 RecordRelaySample 挂在中继请求边界，每请求恰好执行一次。
package perfmetrics

import (
	"strings"
	"time"

	"github.com/NookMux/NookMux/internal/domain/shared"
	relaycommon "github.com/NookMux/NookMux/internal/relay/common"
	perfmetricsstore "github.com/NookMux/NookMux/internal/store/perfmetrics"
)

type sampleOutcome int

const (
	outcomeSuccess sampleOutcome = iota
	outcomeFailure
	outcomeIgnored
)

// ignoredErrorCodes 网关侧业务拒绝与本地校验失败：请求未被上游受理，不计入
// 上游服务质量统计（成功率不因本地限流、额度拒绝、输入非法而下降）。
var ignoredErrorCodes = map[shared.ErrorCode]struct{}{
	shared.ErrorCodeInvalidRequest:             {},
	shared.ErrorCodeSensitiveWordsDetected:     {},
	shared.ErrorCodeInsufficientUserQuota:      {},
	shared.ErrorCodePreConsumeTokenQuotaFailed: {},
	shared.ErrorCodeCountTokenFailed:           {},
	shared.ErrorCodeModelPriceError:            {},
	shared.ErrorCodeGenRelayInfoFailed:         {},
	shared.ErrorCodeReadRequestBodyFailed:      {},
	shared.ErrorCodeConvertRequestFailed:       {},
	shared.ErrorCodeAccessDenied:               {},
	shared.ErrorCodeBadRequestBody:             {},
	shared.ErrorCodeInvalidApiType:             {},
	shared.ErrorCodeJsonMarshalFailed:          {},
}

// classifyOutcome 判定请求样本的统计归类：成功；业务拒绝（违规费、敏感词、
// 额度）与本地校验失败忽略；其余（渠道错误、上游失败、未知错误码）按失败
// 计入，未知错误不静默滤除。
func classifyOutcome(relayErr *shared.NookMuxError) sampleOutcome {
	if relayErr == nil {
		return outcomeSuccess
	}
	code := relayErr.GetErrorCode()
	if strings.HasPrefix(string(code), "violation_fee.") {
		return outcomeIgnored
	}
	if _, ignored := ignoredErrorCodes[code]; ignored {
		return outcomeIgnored
	}
	return outcomeFailure
}

// buildSampleTimings 从 RelayInfo 计时字段推导采样计时值。首字延迟仅对流式
// 且已发出首帧的请求有效（-1 表示不计入）；吞吐分母取生成阶段耗时
// （总耗时扣除首字延迟），非流式请求退化为端到端总耗时。
func buildSampleTimings(info *relaycommon.RelayInfo, latencyMs int64) (firstTokenMs, generationMs int64) {
	firstTokenMs = -1
	if info.HasSendResponse() {
		firstTokenMs = info.FirstResponseTime.Sub(info.StartTime).Milliseconds()
		if firstTokenMs < 0 {
			firstTokenMs = 0
		}
	}
	generationMs = latencyMs
	if firstTokenMs >= 0 {
		generationMs = latencyMs - firstTokenMs
		if generationMs < 0 {
			generationMs = 0
		}
	}
	return firstTokenMs, generationMs
}

// RecordRelaySample 在中继请求边界记录一次性能样本，每请求恰好执行一次。
func RecordRelaySample(info *relaycommon.RelayInfo, relayErr *shared.NookMuxError, elapsed time.Duration) {
	if info == nil {
		return
	}
	modelName := info.OriginModelName
	if modelName == "" {
		return
	}
	outcome := classifyOutcome(relayErr)
	if outcome == outcomeIgnored {
		return
	}

	latencyMs := elapsed.Milliseconds()
	firstTokenMs, generationMs := buildSampleTimings(info, latencyMs)

	perfmetricsstore.Record(modelName, info.UsingGroup, time.Now(), perfmetricsstore.Sample{
		Success:      outcome == outcomeSuccess,
		LatencyMs:    latencyMs,
		FirstTokenMs: firstTokenMs,
		OutputTokens: info.PerformanceOutputTokens,
		GenerationMs: generationMs,
	})
}
