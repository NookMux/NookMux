package billing

import (
	"fmt"

	"github.com/NookMux/NookMux/internal/domain/billing/contract"
	"github.com/NookMux/NookMux/pkg/jsonx"
)

// billing_details canonical JSON 的结构、严格解析与唯一汇总公式定义在契约
// 叶子包 contract（billing_details.go / billing_details_totals.go），供
// billing、store、httpapi、迁移等多层引用而不引入依赖环。本文件保留类型与
// 解析入口的门面别名（仓库既有门面先例见 internal/relay/relay.go），以及
// 依赖 billing 主包 BillingUsage 的序列化入口 SerializeBillingUsage。
//
// billing_details 是持久化 Token 用量的唯一权威来源：聚合总量（输入侧总量、
// 输出总量、处理总量）不单独落库，读取端按 contract 的唯一汇总公式从本 JSON
// 还原（wire 投影、TPM、统计、导出共用同一公式）。

type BillingDetailsPayload = contract.BillingDetailsPayload

type BillingTokensDetail = contract.BillingTokensDetail

type BillingInputTokens = contract.BillingInputTokens

type BillingOutputTokens = contract.BillingOutputTokens

type BillingCacheTokens = contract.BillingCacheTokens

// BillingDetailsSchemaVersion 与 contract.BillingDetailsSchemaVersion 保持同代。
const BillingDetailsSchemaVersion = contract.BillingDetailsSchemaVersion

// ParseBillingDetailsJSON 是读取端唯一入口；契约与校验规则见 contract.ParseBillingDetailsJSON。
func ParseBillingDetailsJSON(raw string) (*BillingDetailsPayload, error) {
	return contract.ParseBillingDetailsJSON(raw)
}

// ParseLegacyBillingDetailsJSON 仅供启动迁移统一旧 schema v1 稀疏 JSON。
func ParseLegacyBillingDetailsJSON(raw string) (*BillingDetailsPayload, error) {
	return contract.ParseLegacyBillingDetailsJSON(raw)
}

// SerializeBillingUsage 把 BillingUsage 序列化为 schema v1 canonical JSON。
// 入参必须先经 finalizeBillingUsage 校验（负数已在构建期显式失败）。
// 上游未返回的可选拆分按 0 写入；官方显式 0 同样保留。文本输入/输出是
// 归一化结果：上游未返回文本拆分时按总量与已知独立模态恢复。
func SerializeBillingUsage(bu *BillingUsage) (string, error) {
	if bu == nil {
		return "", fmt.Errorf("billing usage is nil")
	}
	textInput, textOutput, err := deriveCanonicalTextSplits(bu)
	if err != nil {
		return "", err
	}
	payload := BillingDetailsPayload{
		SchemaVersion: BillingDetailsSchemaVersion,
		Tokens: BillingTokensDetail{
			Input: BillingInputTokens{
				TextInput:     textInput,
				ImageInput:    intValue(bu.ImageInputTokens),
				AudioInput:    intValue(bu.AudioInputTokens),
				VideoInput:    intValue(bu.VideoInputTokens),
				DocumentInput: intValue(bu.DocumentInputTokens),
			},
			Output: BillingOutputTokens{
				TextOutput:         textOutput,
				AudioOutput:        intValue(bu.AudioOutputTokens),
				ImageOutput:        intValue(bu.ImageOutputTokens),
				ReasoningOutput:    intValue(bu.ReasoningTokens),
				AcceptedPrediction: intValue(bu.AcceptedPredictionTokens),
				RejectedPrediction: intValue(bu.RejectedPredictionTokens),
			},
			Cache: BillingCacheTokens{
				ReadCache:    bu.CacheReadTokens,
				WriteCache:   bu.CacheWriteTokens,
				WriteCache5m: intValue(bu.CacheWrite5mTokens),
				WriteCache1h: intValue(bu.CacheWrite1hTokens),
			},
		},
	}
	encoded, err := jsonx.Marshal(payload)
	if err != nil {
		return "", err
	}
	return string(encoded), nil
}

func deriveCanonicalTextSplits(bu *BillingUsage) (int, int, error) {
	textInput := intValue(bu.TextInputTokens)
	if bu.TextInputTokens == nil {
		knownInput, err := checkedAdd(
			intValue(bu.ImageInputTokens), intValue(bu.AudioInputTokens),
			"canonical input modality split",
		)
		if err != nil {
			return 0, 0, err
		}
		knownInput, err = checkedAdd(
			knownInput, intValue(bu.VideoInputTokens),
			"canonical input modality split",
		)
		if err != nil {
			return 0, 0, err
		}
		knownInput, err = checkedAdd(
			knownInput, intValue(bu.DocumentInputTokens),
			"canonical input modality split",
		)
		if err != nil {
			return 0, 0, err
		}
		ordinaryInput := bu.InputTokens()
		if ordinaryInput < 0 || ordinaryInput < knownInput {
			return 0, 0, fmt.Errorf("canonical input modality split is negative")
		}
		textInput = ordinaryInput - knownInput
	}

	textOutput := intValue(bu.TextOutputTokens)
	if bu.TextOutputTokens == nil {
		knownOutput, err := checkedAdd(
			intValue(bu.AudioOutputTokens), intValue(bu.ImageOutputTokens),
			"canonical output modality split",
		)
		if err != nil {
			return 0, 0, err
		}
		if bu.OutputTokens < knownOutput {
			return 0, 0, fmt.Errorf("canonical output modality split is negative")
		}
		textOutput = bu.OutputTokens - knownOutput
	}
	return textInput, textOutput, nil
}
