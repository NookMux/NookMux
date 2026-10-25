package billing

import (
	"testing"

	"github.com/NookMux/NookMux/internal/domain/shared"
	"github.com/NookMux/NookMux/pkg/jsonx"
)

func TestGeminiUsageMetadataToOpenAIUsage(t *testing.T) {
	usage := GeminiUsageMetadataToOpenAIUsage(shared.GeminiUsageMetadata{
		PromptTokenCount:        10,
		CandidatesTokenCount:    6,
		ThoughtsTokenCount:      4,
		TotalTokenCount:         20,
		CachedContentTokenCount: 3,
		PromptTokensDetails:     []shared.GeminiPromptTokensDetails{{Modality: "TEXT", TokenCount: 8}, {Modality: "AUDIO", TokenCount: 2}},
		CandidatesTokensDetails: []shared.GeminiPromptTokensDetails{{Modality: "TEXT", TokenCount: 6}},
	})

	if usage.PromptTokens != 10 || usage.CompletionTokens != 10 || usage.TotalTokens != 20 {
		t.Fatalf("usage = %+v, want prompt=10 completion=10 total=20", usage)
	}
	if usage.PromptTokensDetails.CachedTokens != 3 || usage.PromptCacheHitTokens != 3 {
		t.Fatalf("usage cache details = %+v, want cached=3", usage)
	}
	if usage.PromptTokensDetails.TextTokens != 8 || usage.PromptTokensDetails.AudioTokens != 2 {
		t.Fatalf("prompt token details = %+v, want text=8 audio=2", usage.PromptTokensDetails)
	}
	if usage.CompletionTokenDetails.TextTokens != 6 || usage.CompletionTokenDetails.ReasoningTokens != 4 {
		t.Fatalf("completion token details = %+v, want text=6 reasoning=4", usage.CompletionTokenDetails)
	}
}

func TestGeminiUsageMetadataToOpenAIUsageIncludesToolUsePromptTokens(t *testing.T) {
	usage := GeminiUsageMetadataToOpenAIUsage(shared.GeminiUsageMetadata{
		PromptTokenCount:        151,
		ToolUsePromptTokenCount: 18329,
		CandidatesTokenCount:    1089,
		ThoughtsTokenCount:      1120,
		TotalTokenCount:         20689,
		PromptTokensDetails: []shared.GeminiPromptTokensDetails{
			{Modality: "TEXT", TokenCount: 151},
		},
		ToolUsePromptTokensDetails: []shared.GeminiPromptTokensDetails{
			{Modality: "TEXT", TokenCount: 18329},
		},
		CandidatesTokensDetails: []shared.GeminiPromptTokensDetails{
			{Modality: "TEXT", TokenCount: 1089},
		},
	})

	if usage.PromptTokens != 18480 || usage.InputTokens != 18480 {
		t.Fatalf("prompt/input tokens = %d/%d, want 18480/18480", usage.PromptTokens, usage.InputTokens)
	}
	if usage.CompletionTokens != 2209 || usage.OutputTokens != 2209 || usage.TotalTokens != 20689 {
		t.Fatalf("completion/output/total = %d/%d/%d, want 2209/2209/20689", usage.CompletionTokens, usage.OutputTokens, usage.TotalTokens)
	}
	if usage.PromptTokensDetails.TextTokens != 18480 {
		t.Fatalf("prompt text token details = %d, want 18480", usage.PromptTokensDetails.TextTokens)
	}
	if usage.CompletionTokenDetails.TextTokens != 1089 || usage.CompletionTokenDetails.ReasoningTokens != 1120 {
		t.Fatalf("completion token details = %+v, want text=1089 reasoning=1120", usage.CompletionTokenDetails)
	}
}

func TestGeminiUsageMetadataToOpenAIUsageAcceptsSnakeCaseToolUseFields(t *testing.T) {
	var metadata shared.GeminiUsageMetadata
	if err := jsonx.Unmarshal([]byte(`{
		"prompt_token_count": 2,
		"tool_use_prompt_token_count": 3,
		"candidates_token_count": 5,
		"tool_use_prompt_tokens_details": [{"modality":"TEXT","tokenCount":3}]
	}`), &metadata); err != nil {
		t.Fatalf("unmarshal metadata: %v", err)
	}

	usage := GeminiUsageMetadataToOpenAIUsage(metadata)
	if usage.PromptTokens != 5 || usage.InputTokens != 5 {
		t.Fatalf("prompt/input tokens = %d/%d, want 5/5", usage.PromptTokens, usage.InputTokens)
	}
	if usage.PromptTokensDetails.TextTokens != 3 {
		t.Fatalf("tool prompt text details = %d, want 3", usage.PromptTokensDetails.TextTokens)
	}
}

func TestGeminiUsageMetadataToOpenAIUsageInclusiveThinking(t *testing.T) {
	// AI Studio 风格：candidatesTokenCount 已包含 thoughtsTokenCount，
	// totalTokenCount == promptTokenCount + candidatesTokenCount，不应再单独累加 thoughts。
	usage := GeminiUsageMetadataToOpenAIUsage(shared.GeminiUsageMetadata{
		PromptTokenCount:     8,
		CandidatesTokenCount: 664,
		ThoughtsTokenCount:   651,
		TotalTokenCount:      672,
	})

	if usage.CompletionTokens != 664 || usage.OutputTokens != 664 {
		t.Fatalf("completion/output = %d/%d, want 664/664 (thoughts already included)", usage.CompletionTokens, usage.OutputTokens)
	}
	if usage.CompletionTokenDetails.ReasoningTokens != 651 {
		t.Fatalf("reasoning tokens = %d, want 651", usage.CompletionTokenDetails.ReasoningTokens)
	}
	if usage.TotalTokens != 672 {
		t.Fatalf("total = %d, want 672", usage.TotalTokens)
	}
}

func TestOpenAIUsageToGeminiUsage(t *testing.T) {
	metadata := OpenAIUsageToGeminiUsage(shared.Usage{
		PromptTokens:     12,
		CompletionTokens: 9,
		TotalTokens:      21,
		PromptTokensDetails: shared.InputTokenDetails{
			CachedTokens: 2,
			TextTokens:   12,
		},
		CompletionTokenDetails: shared.OutputTokenDetails{
			TextTokens:      6,
			ReasoningTokens: 3,
		},
	})

	if metadata.PromptTokenCount != 12 || metadata.CandidatesTokenCount != 6 || metadata.ThoughtsTokenCount != 3 || metadata.TotalTokenCount != 21 {
		t.Fatalf("metadata = %+v, want prompt=12 candidates=6 thoughts=3 total=21", metadata)
	}
	if metadata.CachedContentTokenCount != 2 {
		t.Fatalf("metadata cached_content_token_count = %d, want 2", metadata.CachedContentTokenCount)
	}
	if len(metadata.PromptTokensDetails) != 1 || metadata.PromptTokensDetails[0].TokenCount != 12 {
		t.Fatalf("prompt token details = %+v, want one text detail=12", metadata.PromptTokensDetails)
	}
	if len(metadata.CandidatesTokensDetails) != 1 || metadata.CandidatesTokensDetails[0].TokenCount != 6 {
		t.Fatalf("candidate token details = %+v, want one text detail=6", metadata.CandidatesTokensDetails)
	}
}
