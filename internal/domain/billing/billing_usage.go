package billing

import (
	"fmt"
	"math"

	"strings"

	"github.com/NookMux/NookMux/internal/common"
	"github.com/NookMux/NookMux/internal/domain/shared"
	"github.com/NookMux/NookMux/internal/httpapi"
	"github.com/NookMux/NookMux/internal/infra/log"
	relaycommon "github.com/NookMux/NookMux/internal/relay/common"
	relayconstant "github.com/NookMux/NookMux/internal/relay/constant"

	"github.com/gin-gonic/gin"
)

// 本文件实现计费 PRD 阶段 1：三规范（Claude Messages、OpenAI Chat/Responses、
// Gemini）Usage 归一化为内部 BillingUsage，并序列化成 Log.billing_details 的
// canonical JSON（schema 见 PRD 第 4 章）。
//
// 阶段边界：本文件只负责"转换、校验、落库 JSON 与新旧读取兼容"，
// 不切换 quota 公式；现有计费快照与 Other 继续按原逻辑写入。

// BillingDetailsSchemaVersion 当前 billing_details JSON 的 schema 版本（PRD 4.2）。
const BillingDetailsSchemaVersion = 1

// BillingUsage 是归一化后的语义化 Token 用量（PRD 3.1 语义映射表）。
//
// 口径约定：
//   - PromptAggregateTokens 是官方 raw 输入总量（OpenAI/Gemini 已含缓存读取，
//     Claude 按 input+cache_read+cache_creation 三项相加）。
//   - InputTokens() 是扣除缓存后的普通输入，模态明细不默认从中二次扣除。
//   - 输出模态与 reasoning/accepted/rejected 是输出总量子集，不得加回。
//   - 指针字段为 nil 表示上游未返回该拆分，官方明确返回 0 时保存 0；
//     该上游 presence 供归一化判断文本拆分是否需要由总量恢复。除文本
//     输入/输出外，持久化 JSON 统一把缺失与显式 0 都写成数字 0。
type BillingUsage struct {
	Source relayconstant.UsageSource

	// 官方总量（计费基准与 TPM 口径）
	PromptAggregateTokens int
	OutputTokens          int

	CacheReadTokens  int
	CacheWriteTokens int
	// presence 标记只服务内部序列化：历史/程序化调用允许只填非零数值，
	// 新解析路径则用它保留官方显式 0。
	CacheReadPresent  bool
	CacheWritePresent bool
	// CacheWrite5mTokens / CacheWrite1hTokens 仅在官方明确返回分档时非 nil；
	// 无分档总量按 PRD 转换规则令 5m 分档等于总量（见 finalizeBillingUsage）。
	CacheWrite5mTokens *int
	CacheWrite1hTokens *int

	// 输入模态明细（官方字段；与普通输入是子集关系，不与缓存互计）
	TextInputTokens     *int
	ImageInputTokens    *int
	AudioInputTokens    *int
	VideoInputTokens    *int
	DocumentInputTokens *int

	// 输出拆分（输出总量子集/审计拆分）
	TextOutputTokens         *int
	AudioOutputTokens        *int
	ImageOutputTokens        *int
	ReasoningTokens          *int
	AcceptedPredictionTokens *int
	RejectedPredictionTokens *int

	// Gemini toolUsePromptTokenCount：官方总量公式未把它加入输入总量、
	// TPM 或处理总量，只作独立审计字段（PRD 3.1）。
	ToolUsePromptTokens *int

	// Warnings 保留可诊断异常（如明细大于官方总量），不阻断落库。
	Warnings []string
}

// InputTokens 普通输入 = raw 输入总量 - 缓存读取 - 缓存写入。
// Claude 的 raw 总量已按官方三项相加、OpenAI/Gemini 的 raw 总量本身含缓存，
// 因此三个规范族都用同一减法得到"不含缓存的普通输入"。
func (bu *BillingUsage) InputTokens() int {
	return bu.PromptAggregateTokens - bu.CacheReadTokens - bu.CacheWriteTokens
}

// UntieredCacheWriteTokens 未分档缓存写入 = 写入总量 - 官方 5m/1h 分档。
// 归一化校验保证非负；无分档总量经转换规则后该值为 0（已并入 5m 分档）。
func (bu *BillingUsage) UntieredCacheWriteTokens() int {
	untiered := bu.CacheWriteTokens - intValue(bu.CacheWrite5mTokens) - intValue(bu.CacheWrite1hTokens)
	if untiered < 0 {
		return 0
	}
	return untiered
}

// TotalProcessedTokens 处理总量，只用于 TPM；按官方总量公式去重，
// 不加入 Gemini toolUsePromptTokenCount（PRD 3.1）。
func (bu *BillingUsage) TotalProcessedTokens() int {
	return bu.PromptAggregateTokens + bu.OutputTokens
}

// BuildBillingUsage 把上游 usage 归一化为语义化 BillingUsage。
// source 必须由响应解析点显式写入（RelayInfo.UsageSource），禁止用
// FinalRequestRelayFormat 反推语义。Gemini 来源必须携带原始 usageMetadata
// （RelayInfo.UsageGeminiMetadata），因为转换后的 shared.Usage 已把
// toolUsePromptTokenCount 并入 prompt 总量与模态明细，无法还原官方拆分。
//
// 返回的 warnings 是不阻断落库的可诊断异常；error 表示必须放弃写入
// billing_details（负数、分档大于总量等显式失败）。
func BuildBillingUsage(source relayconstant.UsageSource, usage *shared.Usage, geminiMetadata *shared.GeminiUsageMetadata) (*BillingUsage, []string, error) {
	switch source {
	case relayconstant.UsageSourceClaude:
		return buildClaudeBillingUsage(usage)
	case relayconstant.UsageSourceOpenAIChat:
		return buildOpenAIChatBillingUsage(usage)
	case relayconstant.UsageSourceOpenAIResponses:
		return buildOpenAIResponsesBillingUsage(usage)
	case relayconstant.UsageSourceGemini:
		return buildGeminiBillingUsage(geminiMetadata)
	default:
		return nil, nil, fmt.Errorf("unknown usage source: %q", source)
	}
}

// BuildRealtimeBillingUsage 归一化 OpenAI Realtime（wss）用量。Realtime 的
// usage schema（input_tokens/input_token_details/output_tokens）与 Responses
// 同族，因此来源按 UsageSourceOpenAIResponses 归一化。
func BuildRealtimeBillingUsage(source relayconstant.UsageSource, usage *shared.RealtimeUsage) (*BillingUsage, []string, error) {
	if usage == nil {
		return nil, nil, fmt.Errorf("realtime usage is nil")
	}
	if source != relayconstant.UsageSourceOpenAIResponses {
		return nil, nil, fmt.Errorf("unexpected realtime usage source: %q", source)
	}
	bu := &BillingUsage{
		Source:                source,
		PromptAggregateTokens: usage.InputTokens,
		OutputTokens:          usage.OutputTokens,
		CacheReadTokens:       usage.InputTokenDetails.CachedTokens,
		CacheReadPresent:      usage.InputTokenDetails.CachedTokensPresent,
	}
	// 明细非零即透传；解析路径的显式 0 通过 presence 保留，负数由
	// finalizeBillingUsage 显式拒绝，
	// 避免静默裁剪上游矛盾数据。
	if value := usage.InputTokenDetails.TextTokens; value != 0 || usage.InputTokenDetails.TextTokensPresent {
		bu.TextInputTokens = &value
	}
	if value := usage.InputTokenDetails.AudioTokens; value != 0 || usage.InputTokenDetails.AudioTokensPresent {
		bu.AudioInputTokens = &value
	}
	if value := usage.OutputTokenDetails.TextTokens; value != 0 || usage.OutputTokenDetails.TextTokensPresent {
		bu.TextOutputTokens = &value
	}
	if value := usage.OutputTokenDetails.AudioTokens; value != 0 || usage.OutputTokenDetails.AudioTokensPresent {
		bu.AudioOutputTokens = &value
	}
	if value := usage.OutputTokenDetails.ReasoningTokens; value != 0 || usage.OutputTokenDetails.ReasoningTokensPresent {
		bu.ReasoningTokens = &value
	}
	return finalizeBillingUsage(bu)
}

// buildClaudeBillingUsage 归一化 Claude Messages 语义。Claude 原生、AWS
// Bedrock 复用路径、Vertex Claude 模式与 OpenRouter 原生 /api/v1/messages
// 都经 claude 包收敛到 shared.Usage 的同一批字段（PromptTokensDetails 缓存
// 读写、ClaudeCacheCreation5m/1hTokens 分档），因此共用这一条 Claude 规则。
// Claude 无标准输入/输出模态与预测拆分，对应字段保持 nil，不虚构。
func buildClaudeBillingUsage(usage *shared.Usage) (*BillingUsage, []string, error) {
	if usage == nil {
		return nil, nil, fmt.Errorf("usage is nil")
	}
	bu := &BillingUsage{
		Source:                relayconstant.UsageSourceClaude,
		PromptAggregateTokens: usage.PromptTokens,
		OutputTokens:          usage.CompletionTokens,
		CacheReadTokens: resolveCacheReadTokens(
			false,
			usage.PromptTokensDetails,
			usage.InputTokensDetails,
			usage.PromptCacheHitTokens,
		),
		CacheReadPresent: usage.PromptTokensDetails.CachedTokensPresent ||
			inputDetailsCachedTokensPresent(usage.InputTokensDetails),
		CacheWriteTokens: resolveCacheWriteTokens(
			false,
			usage.PromptTokensDetails,
			usage.InputTokensDetails,
		),
		CacheWritePresent: usage.PromptTokensDetails.CachedCreationTokensPresent ||
			inputDetailsCachedCreationPresent(usage.InputTokensDetails),
	}
	// ClaudeCacheCreation5m/1hTokens 是 ClaudeUsageToOpenAIUsage 从官方
	// cache_creation 分档程序化填入的；0 视为官方未返回。
	// 分档/推理明细非零即透传；解析路径的显式 0 通过 presence 保留，
	// 负数由 finalizeBillingUsage 显式拒绝。
	if usage.ClaudeCacheCreation5mTokens != 0 || usage.ClaudeCacheCreation5mPresent {
		bu.CacheWrite5mTokens = &usage.ClaudeCacheCreation5mTokens
	}
	if usage.ClaudeCacheCreation1hTokens != 0 || usage.ClaudeCacheCreation1hPresent {
		bu.CacheWrite1hTokens = &usage.ClaudeCacheCreation1hTokens
	}
	reasoningTokens := choosePresentInt(
		usage.CompletionTokenDetails.ReasoningTokens, usage.CompletionTokenDetails.ReasoningTokensPresent,
		outputDetailsReasoningTokens(usage.OutputTokensDetails), outputDetailsReasoningPresent(usage.OutputTokensDetails),
	)
	if reasoningTokens != 0 || usage.CompletionTokenDetails.ReasoningTokensPresent || outputDetailsReasoningPresent(usage.OutputTokensDetails) {
		bu.ReasoningTokens = &reasoningTokens
	}
	return finalizeBillingUsage(bu)
}

// buildOpenAIChatBillingUsage 归一化 Chat Completions 官方语义及 OpenAI 兼容
// 渠道来源。prompt_tokens 已含缓存读取/写入，模态明细从 prompt_tokens_details
// 透传；5m/1h 写入分档无标准字段，不虚构。
func buildOpenAIChatBillingUsage(usage *shared.Usage) (*BillingUsage, []string, error) {
	if usage == nil {
		return nil, nil, fmt.Errorf("usage is nil")
	}
	bu := &BillingUsage{
		Source:                relayconstant.UsageSourceOpenAIChat,
		PromptAggregateTokens: usage.PromptTokens,
		OutputTokens:          usage.CompletionTokens,
		CacheReadTokens: resolveCacheReadTokens(
			false,
			usage.PromptTokensDetails,
			usage.InputTokensDetails,
			usage.PromptCacheHitTokens,
		),
		CacheReadPresent: usage.PromptTokensDetails.CachedTokensPresent ||
			inputDetailsCachedTokensPresent(usage.InputTokensDetails),
		CacheWriteTokens: resolveCacheWriteTokens(
			false,
			usage.PromptTokensDetails,
			usage.InputTokensDetails,
		),
		CacheWritePresent: usage.PromptTokensDetails.CachedCreationTokensPresent ||
			inputDetailsCachedCreationPresent(usage.InputTokensDetails),
	}
	fillInputModalities(bu, usage.PromptTokensDetails, usage.InputTokensDetails)
	fillOutputSplits(bu, usage.CompletionTokenDetails, usage.OutputTokensDetails)
	return finalizeBillingUsage(bu)
}

// buildOpenAIResponsesBillingUsage 归一化 Responses 官方语义。Responses 侧
// 没有标准输入图片/音频、输出图像/音频字段，只有上游明确返回的明细才透传，
// 不从其他入口补造（PRD 阶段 1）。
func buildOpenAIResponsesBillingUsage(usage *shared.Usage) (*BillingUsage, []string, error) {
	if usage == nil {
		return nil, nil, fmt.Errorf("usage is nil")
	}
	bu := &BillingUsage{
		Source:                relayconstant.UsageSourceOpenAIResponses,
		PromptAggregateTokens: firstNonZero(usage.InputTokens, usage.PromptTokens),
		OutputTokens:          firstNonZero(usage.OutputTokens, usage.CompletionTokens),
		CacheReadTokens: resolveCacheReadTokens(
			true,
			usage.PromptTokensDetails,
			usage.InputTokensDetails,
			usage.PromptCacheHitTokens,
		),
		CacheReadPresent: inputDetailsCachedTokensPresent(usage.InputTokensDetails) ||
			usage.PromptTokensDetails.CachedTokensPresent,
		CacheWriteTokens: resolveCacheWriteTokens(
			true,
			usage.PromptTokensDetails,
			usage.InputTokensDetails,
		),
		CacheWritePresent: inputDetailsCachedCreationPresent(usage.InputTokensDetails) ||
			usage.PromptTokensDetails.CachedCreationTokensPresent,
	}
	// Responses v1 有标准 cache 读写与 reasoning，但没有标准 text/image/audio
	// 模态或 prediction 明细；不能从 Chat 同名字段“顺手”补造 Responses 明细。
	reasoningTokens := choosePresentInt(
		usage.CompletionTokenDetails.ReasoningTokens, usage.CompletionTokenDetails.ReasoningTokensPresent,
		outputDetailsReasoningTokens(usage.OutputTokensDetails), outputDetailsReasoningPresent(usage.OutputTokensDetails),
	)
	if reasoningTokens != 0 || usage.CompletionTokenDetails.ReasoningTokensPresent ||
		outputDetailsReasoningPresent(usage.OutputTokensDetails) {
		bu.ReasoningTokens = &reasoningTokens
	}
	return finalizeBillingUsage(bu)
}

// buildGeminiBillingUsage 从 Gemini 原始 usageMetadata 归一化。必须使用原始
// metadata：GeminiUsageMetadataToOpenAIUsage 会把 toolUsePromptTokenCount
// 并入 prompt 总量并把 toolUsePromptTokensDetails 并入输入模态明细，而
// tool-use 只允许审计、不进入计价输入（PRD 3.1）。Gemini 无标准缓存写入
// 字段，不虚构。
func buildGeminiBillingUsage(metadata *shared.GeminiUsageMetadata) (*BillingUsage, []string, error) {
	if metadata == nil {
		return nil, nil, fmt.Errorf("gemini usage metadata is nil")
	}
	var warnings []string
	geminiOutputTokens, err := checkedAdd(metadata.CandidatesTokenCount, metadata.ThoughtsTokenCount, "gemini output tokens")
	if err != nil {
		return nil, nil, err
	}
	bu := &BillingUsage{
		Source:                relayconstant.UsageSourceGemini,
		PromptAggregateTokens: metadata.PromptTokenCount,
		OutputTokens:          geminiOutputTokens,
		CacheReadTokens:       metadata.CachedContentTokenCount,
		CacheReadPresent:      metadata.CachedContentTokenCountPresent,
	}
	for _, detail := range metadata.PromptTokensDetails {
		count := detail.TokenCount
		if count < 0 {
			return nil, nil, fmt.Errorf("negative gemini input modality %s token count=%d", detail.Modality, count)
		}
		switch strings.ToUpper(strings.TrimSpace(detail.Modality)) {
		case "TEXT":
			// TEXT 官方明细是 raw prompt 的模态拆分，可能包含已单独放入
			// read_cache 的 token。schema v1 的 input 明细不包含缓存，因此
			// 归一化时移除缓存部分；矛盾数据显式失败，不做启发式猜测。
			textCount := count - metadata.CachedContentTokenCount
			if textCount < 0 {
				return nil, nil, fmt.Errorf(
					"gemini text_input (%d) is less than cached_content (%d)",
					count, metadata.CachedContentTokenCount,
				)
			}
			if err := addModality(&bu.TextInputTokens, textCount); err != nil {
				return nil, nil, err
			}
		case "AUDIO":
			if err := addModality(&bu.AudioInputTokens, count); err != nil {
				return nil, nil, err
			}
		case "IMAGE":
			if err := addModality(&bu.ImageInputTokens, count); err != nil {
				return nil, nil, err
			}
		case "VIDEO":
			if err := addModality(&bu.VideoInputTokens, count); err != nil {
				return nil, nil, err
			}
		case "DOCUMENT":
			if err := addModality(&bu.DocumentInputTokens, count); err != nil {
				return nil, nil, err
			}
		default:
			warnings = append(warnings, fmt.Sprintf(
				"unknown gemini input modality %q with %d tokens; detail omitted from schema v1",
				detail.Modality, count,
			))
		}
	}
	// 官方口径注意：Gemini 的 promptTokensDetails 是 promptTokenCount（含缓存
	// 部分）的按模态拆分，TEXT 明细可能包含 cachedContentTokenCount。schema
	// v1 要求 input 明细不含缓存，因此上方已经移除 read_cache；这里不能再次
	// 用缓存部分伪造 TEXT 拆分。
	for _, detail := range metadata.CandidatesTokensDetails {
		count := detail.TokenCount
		if count < 0 {
			return nil, nil, fmt.Errorf("negative gemini output modality %s token count=%d", detail.Modality, count)
		}
		switch strings.ToUpper(strings.TrimSpace(detail.Modality)) {
		case "TEXT":
			if err := addModality(&bu.TextOutputTokens, count); err != nil {
				return nil, nil, err
			}
		case "AUDIO":
			if err := addModality(&bu.AudioOutputTokens, count); err != nil {
				return nil, nil, err
			}
		case "IMAGE":
			if err := addModality(&bu.ImageOutputTokens, count); err != nil {
				return nil, nil, err
			}
		default:
			warnings = append(warnings, fmt.Sprintf(
				"unknown gemini output modality %q with %d tokens; detail omitted from schema v1",
				detail.Modality, count,
			))
		}
	}
	if metadata.ThoughtsTokenCount != 0 {
		bu.ReasoningTokens = &metadata.ThoughtsTokenCount
	}
	if metadata.ToolUsePromptTokenCount != 0 {
		bu.ToolUsePromptTokens = &metadata.ToolUsePromptTokenCount
	}
	finalized, finalizeWarnings, err := finalizeBillingUsage(bu)
	if err != nil {
		return nil, nil, err
	}
	return finalized, append(warnings, finalizeWarnings...), nil
}

// finalizeBillingUsage 应用缓存写入转换规则、负数/分档校验与可诊断异常收集。
func finalizeBillingUsage(bu *BillingUsage) (*BillingUsage, []string, error) {
	nonNegative := map[string]int{
		"prompt_aggregate_tokens": bu.PromptAggregateTokens,
		"output_tokens":           bu.OutputTokens,
		"read_cache":              bu.CacheReadTokens,
		"write_cache":             bu.CacheWriteTokens,
	}
	for name, value := range nonNegative {
		if value < 0 {
			return nil, nil, fmt.Errorf("negative token count %s=%d", name, value)
		}
	}
	for name, value := range map[string]*int{
		"write_cache_5m":      bu.CacheWrite5mTokens,
		"write_cache_1h":      bu.CacheWrite1hTokens,
		"text_input":          bu.TextInputTokens,
		"image_input":         bu.ImageInputTokens,
		"audio_input":         bu.AudioInputTokens,
		"video_input":         bu.VideoInputTokens,
		"document_input":      bu.DocumentInputTokens,
		"text_output":         bu.TextOutputTokens,
		"audio_output":        bu.AudioOutputTokens,
		"image_output":        bu.ImageOutputTokens,
		"reasoning_output":    bu.ReasoningTokens,
		"accepted_prediction": bu.AcceptedPredictionTokens,
		"rejected_prediction": bu.RejectedPredictionTokens,
		"tool_use_prompt":     bu.ToolUsePromptTokens,
	} {
		if value != nil && *value < 0 {
			return nil, nil, fmt.Errorf("negative token count %s=%d", name, *value)
		}
	}

	// 官方分档存在时，校验其和不大于写入总量；否则是上游数据矛盾，显式失败。
	if bu.CacheWrite5mTokens != nil || bu.CacheWrite1hTokens != nil {
		officialTiered, err := checkedAdd(
			intValue(bu.CacheWrite5mTokens), intValue(bu.CacheWrite1hTokens),
			"cache write tiers",
		)
		if err != nil {
			return nil, nil, err
		}
		if officialTiered > bu.CacheWriteTokens {
			return nil, nil, fmt.Errorf("cache write tiers (%d) exceed write_cache total (%d)",
				officialTiered, bu.CacheWriteTokens)
		}
	}

	// 缓存写入转换规则：存在 write_cache 且官方未返回任何分档时，
	// 令 write_cache_5m = write_cache（未分档写入按 5 分钟档计价）。
	// 注意拷贝一份，避免 5m 指针与总量字段自别名。
	if bu.CacheWriteTokens > 0 && bu.CacheWrite5mTokens == nil && bu.CacheWrite1hTokens == nil {
		converted5m := bu.CacheWriteTokens
		bu.CacheWrite5mTokens = &converted5m
	}

	// 明细大于官方总量是可诊断异常：记录告警但不静默裁剪、不伪造空明细。
	var warnings []string
	inputDetail, err := checkedAdd(
		intValue(bu.TextInputTokens), intValue(bu.ImageInputTokens), "input modality details",
	)
	if err != nil {
		return nil, nil, err
	}
	inputDetail, err = checkedAdd(inputDetail, intValue(bu.AudioInputTokens), "input modality details")
	if err != nil {
		return nil, nil, err
	}
	inputDetail, err = checkedAdd(inputDetail, intValue(bu.VideoInputTokens), "input modality details")
	if err != nil {
		return nil, nil, err
	}
	inputDetail, err = checkedAdd(inputDetail, intValue(bu.DocumentInputTokens), "input modality details")
	if err != nil {
		return nil, nil, err
	}
	if inputDetail > bu.PromptAggregateTokens && bu.PromptAggregateTokens > 0 {
		warnings = append(warnings, fmt.Sprintf("input modality details (%d) exceed prompt aggregate (%d)", inputDetail, bu.PromptAggregateTokens))
	}
	outputDetail, err := checkedAdd(
		intValue(bu.TextOutputTokens), intValue(bu.AudioOutputTokens), "output splits",
	)
	if err != nil {
		return nil, nil, err
	}
	outputDetail, err = checkedAdd(outputDetail, intValue(bu.ImageOutputTokens), "output splits")
	if err != nil {
		return nil, nil, err
	}
	outputDetail, err = checkedAdd(outputDetail, intValue(bu.ReasoningTokens), "output splits")
	if err != nil {
		return nil, nil, err
	}
	outputDetail, err = checkedAdd(outputDetail, intValue(bu.AcceptedPredictionTokens), "output splits")
	if err != nil {
		return nil, nil, err
	}
	outputDetail, err = checkedAdd(outputDetail, intValue(bu.RejectedPredictionTokens), "output splits")
	if err != nil {
		return nil, nil, err
	}
	if outputDetail > bu.OutputTokens && bu.OutputTokens > 0 {
		warnings = append(warnings, fmt.Sprintf("output splits (%d) exceed output total (%d)", outputDetail, bu.OutputTokens))
	}
	cacheUsed, err := checkedAdd(bu.CacheReadTokens, bu.CacheWriteTokens, "cache usage")
	if err != nil {
		return nil, nil, err
	}
	if cacheUsed > bu.PromptAggregateTokens {
		return nil, nil, fmt.Errorf("cache usage (%d) exceeds prompt aggregate (%d); ordinary input would be negative", cacheUsed, bu.PromptAggregateTokens)
	}
	if bu.CacheReadTokens > bu.PromptAggregateTokens && bu.PromptAggregateTokens > 0 {
		warnings = append(warnings, fmt.Sprintf("read_cache (%d) exceeds prompt aggregate (%d)", bu.CacheReadTokens, bu.PromptAggregateTokens))
	}
	if bu.CacheWriteTokens > bu.PromptAggregateTokens && bu.PromptAggregateTokens > 0 {
		warnings = append(warnings, fmt.Sprintf("write_cache (%d) exceeds prompt aggregate (%d)", bu.CacheWriteTokens, bu.PromptAggregateTokens))
	}
	return bu, warnings, nil
}

func checkedAdd(left, right int, name string) (int, error) {
	if right > 0 && left > math.MaxInt-right {
		return 0, fmt.Errorf("%s overflow (%d + %d)", name, left, right)
	}
	if right < 0 && left < math.MinInt-right {
		return 0, fmt.Errorf("%s overflow (%d + %d)", name, left, right)
	}
	return left + right, nil
}

func fillInputModalities(bu *BillingUsage, promptDetails shared.InputTokenDetails, inputDetails *shared.InputTokenDetails) {
	// 明细非零即透传；解析路径的显式 0 通过 presence 保留，负数由
	// finalizeBillingUsage 显式拒绝。
	textTokens := choosePresentInt(
		promptDetails.TextTokens, promptDetails.TextTokensPresent,
		inputDetailsTextTokens(inputDetails), inputDetailsTextPresent(inputDetails),
	)
	if textTokens != 0 || promptDetails.TextTokensPresent || inputDetailsTextPresent(inputDetails) {
		bu.TextInputTokens = &textTokens
	}
	imageTokens := choosePresentInt(
		promptDetails.ImageTokens, promptDetails.ImageTokensPresent,
		inputDetailsImageTokens(inputDetails), inputDetailsImagePresent(inputDetails),
	)
	if imageTokens != 0 || promptDetails.ImageTokensPresent || inputDetailsImagePresent(inputDetails) {
		bu.ImageInputTokens = &imageTokens
	}
	audioTokens := choosePresentInt(
		promptDetails.AudioTokens, promptDetails.AudioTokensPresent,
		inputDetailsAudioTokens(inputDetails), inputDetailsAudioPresent(inputDetails),
	)
	if audioTokens != 0 || promptDetails.AudioTokensPresent || inputDetailsAudioPresent(inputDetails) {
		bu.AudioInputTokens = &audioTokens
	}
}

func fillOutputSplits(bu *BillingUsage, completionDetails shared.OutputTokenDetails, outputDetails *shared.OutputTokenDetails) {
	textTokens := choosePresentInt(
		completionDetails.TextTokens, completionDetails.TextTokensPresent,
		outputDetailsTextTokens(outputDetails), outputDetailsTextPresent(outputDetails),
	)
	if textTokens != 0 || completionDetails.TextTokensPresent || outputDetailsTextPresent(outputDetails) {
		bu.TextOutputTokens = &textTokens
	}
	audioTokens := choosePresentInt(
		completionDetails.AudioTokens, completionDetails.AudioTokensPresent,
		outputDetailsAudioTokens(outputDetails), outputDetailsAudioPresent(outputDetails),
	)
	if audioTokens != 0 || completionDetails.AudioTokensPresent || outputDetailsAudioPresent(outputDetails) {
		bu.AudioOutputTokens = &audioTokens
	}
	reasoningTokens := choosePresentInt(
		completionDetails.ReasoningTokens, completionDetails.ReasoningTokensPresent,
		outputDetailsReasoningTokens(outputDetails), outputDetailsReasoningPresent(outputDetails),
	)
	if reasoningTokens != 0 || completionDetails.ReasoningTokensPresent || outputDetailsReasoningPresent(outputDetails) {
		bu.ReasoningTokens = &reasoningTokens
	}
	acceptedTokens := choosePresentInt(
		completionDetails.AcceptedPredictionTokens, completionDetails.AcceptedPredictionPresent,
		outputDetailsAcceptedPredictionTokens(outputDetails), outputDetailsAcceptedPredictionPresent(outputDetails),
	)
	if acceptedTokens != 0 || completionDetails.AcceptedPredictionPresent || outputDetailsAcceptedPredictionPresent(outputDetails) {
		bu.AcceptedPredictionTokens = &acceptedTokens
	}
	rejectedTokens := choosePresentInt(
		completionDetails.RejectedPredictionTokens, completionDetails.RejectedPredictionPresent,
		outputDetailsRejectedPredictionTokens(outputDetails), outputDetailsRejectedPredictionPresent(outputDetails),
	)
	if rejectedTokens != 0 || completionDetails.RejectedPredictionPresent || outputDetailsRejectedPredictionPresent(outputDetails) {
		bu.RejectedPredictionTokens = &rejectedTokens
	}
}

func addModality(existing **int, count int) error {
	if existing == nil {
		return fmt.Errorf("modality target is nil")
	}
	if count < 0 {
		return fmt.Errorf("negative modality detail=%d", count)
	}
	if *existing == nil {
		value := count
		*existing = &value
		return nil
	}
	merged, err := checkedAdd(**existing, count, "modality details")
	if err != nil {
		return err
	}
	*existing = &merged
	return nil
}

func firstNonZero(values ...int) int {
	for _, value := range values {
		if value != 0 {
			return value
		}
	}
	return 0
}

func choosePresentInt(primary int, primaryPresent bool, fallback int, fallbackPresent bool) int {
	if primaryPresent {
		return primary
	}
	if fallbackPresent {
		return fallback
	}
	return firstNonZero(primary, fallback)
}

func resolveCacheReadTokens(responsesPreference bool, promptDetails shared.InputTokenDetails, inputDetails *shared.InputTokenDetails, promptCacheHitTokens int) int {
	if responsesPreference {
		if inputDetails != nil && inputDetails.CachedTokensPresent {
			return inputDetails.CachedTokens
		}
	if promptDetails.CachedTokensPresent {
		return promptDetails.CachedTokens
	}
	return firstNonZero(
		inputDetailsCachedTokens(inputDetails),
		promptDetails.CachedTokens,
		promptCacheHitTokens,
	)
}
	if promptDetails.CachedTokensPresent {
		return promptDetails.CachedTokens
	}
	if inputDetails != nil && inputDetails.CachedTokensPresent {
		return inputDetails.CachedTokens
	}
	return firstNonZero(
		promptDetails.CachedTokens,
		inputDetailsCachedTokens(inputDetails),
		promptCacheHitTokens,
	)
}

func resolveCacheWriteTokens(responsesPreference bool, promptDetails shared.InputTokenDetails, inputDetails *shared.InputTokenDetails) int {
	if responsesPreference {
		if inputDetails != nil && inputDetails.CachedCreationTokensPresent {
			return inputDetails.CachedCreationTokens
		}
		if promptDetails.CachedCreationTokensPresent {
			return promptDetails.CachedCreationTokens
		}
		return firstNonZero(
			inputDetailsCachedCreationTokens(inputDetails),
			promptDetails.CachedCreationTokens,
		)
	}
	if promptDetails.CachedCreationTokensPresent {
		return promptDetails.CachedCreationTokens
	}
	if inputDetails != nil && inputDetails.CachedCreationTokensPresent {
		return inputDetails.CachedCreationTokens
	}
	return firstNonZero(
		promptDetails.CachedCreationTokens,
		inputDetailsCachedCreationTokens(inputDetails),
	)
}

func intValue(value *int) int {
	if value == nil {
		return 0
	}
	return *value
}

func inputDetailsCachedTokens(details *shared.InputTokenDetails) int {
	if details == nil {
		return 0
	}
	return details.CachedTokens
}

func inputDetailsCachedTokensPresent(details *shared.InputTokenDetails) bool {
	if details == nil {
		return false
	}
	return details.CachedTokensPresent
}

func inputDetailsCachedCreationTokens(details *shared.InputTokenDetails) int {
	if details == nil {
		return 0
	}
	return details.CachedCreationTokens
}

func inputDetailsCachedCreationPresent(details *shared.InputTokenDetails) bool {
	if details == nil {
		return false
	}
	return details.CachedCreationTokensPresent
}

func inputDetailsTextTokens(details *shared.InputTokenDetails) int {
	if details == nil {
		return 0
	}
	return details.TextTokens
}

func inputDetailsTextPresent(details *shared.InputTokenDetails) bool {
	if details == nil {
		return false
	}
	return details.TextTokensPresent
}

func inputDetailsImageTokens(details *shared.InputTokenDetails) int {
	if details == nil {
		return 0
	}
	return details.ImageTokens
}

func inputDetailsImagePresent(details *shared.InputTokenDetails) bool {
	if details == nil {
		return false
	}
	return details.ImageTokensPresent
}

func inputDetailsAudioTokens(details *shared.InputTokenDetails) int {
	if details == nil {
		return 0
	}
	return details.AudioTokens
}

func inputDetailsAudioPresent(details *shared.InputTokenDetails) bool {
	if details == nil {
		return false
	}
	return details.AudioTokensPresent
}

func outputDetailsTextTokens(details *shared.OutputTokenDetails) int {
	if details == nil {
		return 0
	}
	return details.TextTokens
}

func outputDetailsTextPresent(details *shared.OutputTokenDetails) bool {
	if details == nil {
		return false
	}
	return details.TextTokensPresent
}

func outputDetailsAudioTokens(details *shared.OutputTokenDetails) int {
	if details == nil {
		return 0
	}
	return details.AudioTokens
}

func outputDetailsAudioPresent(details *shared.OutputTokenDetails) bool {
	if details == nil {
		return false
	}
	return details.AudioTokensPresent
}

func outputDetailsReasoningTokens(details *shared.OutputTokenDetails) int {
	if details == nil {
		return 0
	}
	return details.ReasoningTokens
}

func outputDetailsReasoningPresent(details *shared.OutputTokenDetails) bool {
	if details == nil {
		return false
	}
	return details.ReasoningTokensPresent
}

func outputDetailsAcceptedPredictionTokens(details *shared.OutputTokenDetails) int {
	if details == nil {
		return 0
	}
	return details.AcceptedPredictionTokens
}

func outputDetailsAcceptedPredictionPresent(details *shared.OutputTokenDetails) bool {
	if details == nil {
		return false
	}
	return details.AcceptedPredictionPresent
}

func outputDetailsRejectedPredictionTokens(details *shared.OutputTokenDetails) int {
	if details == nil {
		return 0
	}
	return details.RejectedPredictionTokens
}

func outputDetailsRejectedPredictionPresent(details *shared.OutputTokenDetails) bool {
	if details == nil {
		return false
	}
	return details.RejectedPredictionPresent
}

// BuildBillingDetailsForLog 把有 Token 用量的消费入口归一化并序列化为
// billing_details JSON（RecordConsumeLogParams.BillingDetails）。
// 返回空串表示不写该列：
//   - 本地估算/非 token 口径的伪 usage（ContextKeyLocalCountTokens）；
//   - 无 token 用量（总量与明细全零，如纯工具费、违规费入口）；
//   - 来源未标识、归一化或序列化失败——走 LogWarn/LogError 可诊断路径，
//     不伪造空拆分。
func BuildBillingDetailsForLog(ctx *gin.Context, relayInfo *relaycommon.RelayInfo, usage *shared.Usage) string {
	if usage == nil {
		return ""
	}
	if httpapi.GetContextKeyBool(ctx, common.ContextKeyLocalCountTokens) {
		return ""
	}
	if usage.PromptTokens == 0 && usage.CompletionTokens == 0 {
		return ""
	}
	if relayInfo == nil || relayInfo.UsageSource == relayconstant.UsageSourceNone {
		log.LogError(ctx, fmt.Sprintf("billing_details skipped: usage source not identified, model=%s", usageSourceModelName(relayInfo)))
		return ""
	}
	bu, warnings, err := BuildBillingUsage(relayInfo.UsageSource, usage, relayInfo.UsageGeminiMetadata)
	if err != nil {
		log.LogError(ctx, "billing_details normalization failed: "+err.Error())
		return ""
	}
	logBillingWarnings(ctx, warnings)
	return serializeForLog(ctx, bu)
}

// BuildRealtimeBillingDetailsForLog 同 BuildBillingDetailsForLog，用于
// Realtime/WSS 消费入口（usage 为 shared.RealtimeUsage）。
func BuildRealtimeBillingDetailsForLog(ctx *gin.Context, relayInfo *relaycommon.RelayInfo, usage *shared.RealtimeUsage) string {
	if usage == nil {
		return ""
	}
	if httpapi.GetContextKeyBool(ctx, common.ContextKeyLocalCountTokens) {
		return ""
	}
	if usage.InputTokens == 0 && usage.OutputTokens == 0 {
		return ""
	}
	if relayInfo == nil || relayInfo.UsageSource == relayconstant.UsageSourceNone {
		log.LogError(ctx, fmt.Sprintf("billing_details skipped: usage source not identified, model=%s", usageSourceModelName(relayInfo)))
		return ""
	}
	bu, warnings, err := BuildRealtimeBillingUsage(relayInfo.UsageSource, usage)
	if err != nil {
		log.LogError(ctx, "billing_details normalization failed: "+err.Error())
		return ""
	}
	logBillingWarnings(ctx, warnings)
	return serializeForLog(ctx, bu)
}

func serializeForLog(ctx *gin.Context, bu *BillingUsage) string {
	payload, err := SerializeBillingUsage(bu)
	if err != nil {
		log.LogError(ctx, "billing_details serialization failed: "+err.Error())
		return ""
	}
	return payload
}

func logBillingWarnings(ctx *gin.Context, warnings []string) {
	for _, warning := range warnings {
		log.LogWarn(ctx, "billing_details anomaly: "+warning)
	}
}

func usageSourceModelName(relayInfo *relaycommon.RelayInfo) string {
	if relayInfo == nil {
		return ""
	}
	return relayInfo.OriginModelName
}
