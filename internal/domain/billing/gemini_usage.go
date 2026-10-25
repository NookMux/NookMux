package billing

import (
	"strings"

	"github.com/NookMux/NookMux/internal/domain/shared"
)

// HasGeminiUsageMetadata reports whether Gemini returned any usage payload worth mapping.
func HasGeminiUsageMetadata(metadata shared.GeminiUsageMetadata) bool {
	return metadata.PromptTokenCount > 0 ||
		metadata.ToolUsePromptTokenCount > 0 ||
		metadata.CandidatesTokenCount > 0 ||
		metadata.TotalTokenCount > 0 ||
		metadata.ThoughtsTokenCount > 0 ||
		metadata.CachedContentTokenCount > 0 ||
		len(metadata.PromptTokensDetails) > 0 ||
		len(metadata.ToolUsePromptTokensDetails) > 0 ||
		len(metadata.CandidatesTokensDetails) > 0
}

// GeminiUsageMetadataToOpenAIUsage normalizes Gemini usage metadata into the local OpenAI-compatible shape.
func GeminiUsageMetadataToOpenAIUsage(metadata shared.GeminiUsageMetadata) shared.Usage {
	promptTokens := metadata.PromptTokenCount + metadata.ToolUsePromptTokenCount

	// Gemini 的 usageMetadata 中 candidatesTokenCount 是否已包含 thoughtsTokenCount，
	// 在不同端点（AI Studio 与 Vertex）及不同模型版本间口径不一致：当
	// promptTokenCount + candidatesTokenCount + toolUsePromptTokenCount == totalTokenCount
	// 时，candidatesTokenCount 已含 thoughts，单独累加会造成输出侧重复计费。用
	// totalTokenCount 做运行时检测，参照 litellm 的 is_candidate_token_count_inclusive。
	completionTokens := metadata.CandidatesTokenCount
	if !isCandidateTokenCountInclusive(metadata) && metadata.ThoughtsTokenCount > 0 {
		completionTokens += metadata.ThoughtsTokenCount
	}

	usage := shared.Usage{
		PromptTokens:     promptTokens,
		CompletionTokens: completionTokens,
		TotalTokens:      metadata.TotalTokenCount,
		InputTokens:      promptTokens,
		OutputTokens:     completionTokens,
	}

	if usage.TotalTokens == 0 {
		usage.TotalTokens = usage.PromptTokens + usage.CompletionTokens
	}

	usage.PromptCacheHitTokens = metadata.CachedContentTokenCount
	usage.PromptTokensDetails.CachedTokens = metadata.CachedContentTokenCount
	usage.CompletionTokenDetails.ReasoningTokens = metadata.ThoughtsTokenCount

	for _, detail := range append(metadata.PromptTokensDetails, metadata.ToolUsePromptTokensDetails...) {
		switch strings.ToUpper(strings.TrimSpace(detail.Modality)) {
		case "TEXT":
			usage.PromptTokensDetails.TextTokens += detail.TokenCount
		case "AUDIO":
			usage.PromptTokensDetails.AudioTokens += detail.TokenCount
		case "IMAGE":
			usage.PromptTokensDetails.ImageTokens += detail.TokenCount
		}
	}

	for _, detail := range metadata.CandidatesTokensDetails {
		switch strings.ToUpper(strings.TrimSpace(detail.Modality)) {
		case "TEXT":
			usage.CompletionTokenDetails.TextTokens = detail.TokenCount
		case "AUDIO":
			usage.CompletionTokenDetails.AudioTokens = detail.TokenCount
		}
	}

	// Gemini often omits modality breakdowns; keep a text fallback only when no finer detail exists.
	if len(metadata.PromptTokensDetails) == 0 && len(metadata.ToolUsePromptTokensDetails) == 0 && usage.PromptTokens > 0 {
		usage.PromptTokensDetails.TextTokens = usage.PromptTokens
	}
	if len(metadata.CandidatesTokensDetails) == 0 && metadata.CandidatesTokenCount > 0 {
		usage.CompletionTokenDetails.TextTokens = metadata.CandidatesTokenCount
	}

	return usage
}

// isCandidateTokenCountInclusive reports whether Gemini's candidatesTokenCount already
// includes thoughtsTokenCount, detected by checking whether the non-thinking token sum
// equals totalTokenCount. Google's usageMetadata is inconsistent across AI Studio /
// Vertex and model versions, so this is detected at runtime rather than per model.
// Mirrors litellm's is_candidate_token_count_inclusive.
func isCandidateTokenCountInclusive(metadata shared.GeminiUsageMetadata) bool {
	nonThinkingTokens := metadata.PromptTokenCount +
		metadata.CandidatesTokenCount +
		metadata.ToolUsePromptTokenCount
	return nonThinkingTokens == metadata.TotalTokenCount
}

// OpenAIUsageToGeminiUsage rewrites OpenAI-compatible usage into Gemini's usage schema.
func OpenAIUsageToGeminiUsage(usage shared.Usage) shared.GeminiUsageMetadata {
	promptDetails := usage.PromptTokensDetails
	if usage.InputTokensDetails != nil && promptDetails == (shared.InputTokenDetails{}) {
		promptDetails = *usage.InputTokensDetails
	}

	promptTokens := usage.PromptTokens
	if promptTokens == 0 {
		promptTokens = usage.InputTokens
	}
	completionTokens := usage.CompletionTokens
	if completionTokens == 0 {
		completionTokens = usage.OutputTokens
	}

	metadata := shared.GeminiUsageMetadata{
		PromptTokenCount:        promptTokens,
		CandidatesTokenCount:    completionTokens,
		TotalTokenCount:         usage.TotalTokens,
		CachedContentTokenCount: promptDetails.CachedTokens,
		PromptTokensDetails:     buildGeminiPromptTokenDetails(promptDetails),
	}

	if metadata.TotalTokenCount == 0 {
		metadata.TotalTokenCount = promptTokens + completionTokens
	}

	if reasoningTokens := usage.CompletionTokenDetails.ReasoningTokens; reasoningTokens > 0 {
		metadata.ThoughtsTokenCount = reasoningTokens
		if reasoningTokens <= completionTokens {
			metadata.CandidatesTokenCount = completionTokens - reasoningTokens
		}
	}

	metadata.CandidatesTokensDetails = buildGeminiCandidateTokenDetails(usage.CompletionTokenDetails)
	return metadata
}

func buildGeminiPromptTokenDetails(details shared.InputTokenDetails) []shared.GeminiPromptTokensDetails {
	out := make([]shared.GeminiPromptTokensDetails, 0, 3)
	if details.TextTokens > 0 {
		out = append(out, shared.GeminiPromptTokensDetails{Modality: "TEXT", TokenCount: details.TextTokens})
	}
	if details.AudioTokens > 0 {
		out = append(out, shared.GeminiPromptTokensDetails{Modality: "AUDIO", TokenCount: details.AudioTokens})
	}
	if details.ImageTokens > 0 {
		out = append(out, shared.GeminiPromptTokensDetails{Modality: "IMAGE", TokenCount: details.ImageTokens})
	}
	return out
}

func buildGeminiCandidateTokenDetails(details shared.OutputTokenDetails) []shared.GeminiPromptTokensDetails {
	out := make([]shared.GeminiPromptTokensDetails, 0, 2)
	if details.TextTokens > 0 {
		out = append(out, shared.GeminiPromptTokensDetails{Modality: "TEXT", TokenCount: details.TextTokens})
	}
	if details.AudioTokens > 0 {
		out = append(out, shared.GeminiPromptTokensDetails{Modality: "AUDIO", TokenCount: details.AudioTokens})
	}
	return out
}
