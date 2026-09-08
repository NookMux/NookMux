package billing

import (
	"math"
	"strings"
	"testing"

	relayconstant "github.com/NookMux/NookMux/internal/relay/constant"

	"github.com/NookMux/NookMux/internal/domain/shared"

	"github.com/NookMux/NookMux/pkg/jsonx"
)

// buildUsageForTest 组装 shared.Usage，避免各用例重复整段字面量。
func buildUsageForTest(modify func(*shared.Usage)) *shared.Usage {
	usage := &shared.Usage{}
	if modify != nil {
		modify(usage)
	}
	return usage
}

// TestBuildBillingUsageClaudeScenarios 覆盖 Claude 规则：无缓存、缓存读取、
// 缓存写入、5m/1h 分档与未分档转换。
func TestBuildBillingUsageClaudeScenarios(t *testing.T) {
	tests := []struct {
		name         string
		usage        *shared.Usage
		want         string
		wantIn       int // 普通输入 = aggregate - read - write
		wantUntiered int
	}{
		{
			name: "no cache",
			usage: buildUsageForTest(func(u *shared.Usage) {
				u.PromptTokens = 100
				u.CompletionTokens = 50
				u.TotalTokens = 150
			}),
			want:   `{"schema_version":1,"tokens":{"input":{"text_input":100,"image_input":0,"audio_input":0,"video_input":0,"document_input":0},"output":{"text_output":50,"audio_output":0,"image_output":0,"reasoning_output":0,"accepted_prediction":0,"rejected_prediction":0},"cache":{"read_cache":0,"write_cache":0,"write_cache_5m":0,"write_cache_1h":0}}}`,
			wantIn: 100,
		},
		{
			name: "cache read only",
			usage: buildUsageForTest(func(u *shared.Usage) {
				u.PromptTokens = 100 + 30
				u.CompletionTokens = 50
				u.PromptTokensDetails.CachedTokens = 30
			}),
			want:   `{"schema_version":1,"tokens":{"input":{"text_input":100,"image_input":0,"audio_input":0,"video_input":0,"document_input":0},"output":{"text_output":50,"audio_output":0,"image_output":0,"reasoning_output":0,"accepted_prediction":0,"rejected_prediction":0},"cache":{"read_cache":30,"write_cache":0,"write_cache_5m":0,"write_cache_1h":0}}}`,
			wantIn: 100,
		},
		{
			name: "cache write untiered converts to 5m",
			usage: buildUsageForTest(func(u *shared.Usage) {
				u.PromptTokens = 100 + 20
				u.CompletionTokens = 50
				u.PromptTokensDetails.CachedCreationTokens = 20
			}),
			want:         `{"schema_version":1,"tokens":{"input":{"text_input":100,"image_input":0,"audio_input":0,"video_input":0,"document_input":0},"output":{"text_output":50,"audio_output":0,"image_output":0,"reasoning_output":0,"accepted_prediction":0,"rejected_prediction":0},"cache":{"read_cache":0,"write_cache":20,"write_cache_5m":20,"write_cache_1h":0}}}`,
			wantIn:       100,
			wantUntiered: 0,
		},
		{
			name: "cache write with official 5m and 1h tiers",
			usage: buildUsageForTest(func(u *shared.Usage) {
				u.PromptTokens = 100 + 30 + 10
				u.CompletionTokens = 50
				u.PromptTokensDetails.CachedCreationTokens = 40
				u.ClaudeCacheCreation5mTokens = 30
				u.ClaudeCacheCreation1hTokens = 10
			}),
			want:         `{"schema_version":1,"tokens":{"input":{"text_input":100,"image_input":0,"audio_input":0,"video_input":0,"document_input":0},"output":{"text_output":50,"audio_output":0,"image_output":0,"reasoning_output":0,"accepted_prediction":0,"rejected_prediction":0},"cache":{"read_cache":0,"write_cache":40,"write_cache_5m":30,"write_cache_1h":10}}}`,
			wantIn:       100,
			wantUntiered: 0,
		},
		{
			name: "cache write with partial tiers keeps untiered diff",
			usage: buildUsageForTest(func(u *shared.Usage) {
				u.PromptTokens = 100 + 25
				u.CompletionTokens = 50
				u.PromptTokensDetails.CachedCreationTokens = 25
				u.ClaudeCacheCreation1hTokens = 10
			}),
			want:         `{"schema_version":1,"tokens":{"input":{"text_input":100,"image_input":0,"audio_input":0,"video_input":0,"document_input":0},"output":{"text_output":50,"audio_output":0,"image_output":0,"reasoning_output":0,"accepted_prediction":0,"rejected_prediction":0},"cache":{"read_cache":0,"write_cache":25,"write_cache_5m":0,"write_cache_1h":10}}}`,
			wantIn:       100,
			wantUntiered: 15,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			bu, warnings, err := BuildBillingUsage(relayconstant.UsageSourceClaude, tt.usage, nil)
			if err != nil {
				t.Fatalf("BuildBillingUsage() error = %v", err)
			}
			if len(warnings) != 0 {
				t.Fatalf("warnings = %v, want empty", warnings)
			}
			if got := bu.InputTokens(); got != tt.wantIn {
				t.Fatalf("InputTokens() = %d, want %d", got, tt.wantIn)
			}
			if got := bu.UntieredCacheWriteTokens(); got != tt.wantUntiered {
				t.Fatalf("UntieredCacheWriteTokens() = %d, want %d", got, tt.wantUntiered)
			}
			got, err := SerializeBillingUsage(bu)
			if err != nil {
				t.Fatalf("SerializeBillingUsage() error = %v", err)
			}
			if got != tt.want {
				t.Fatalf("JSON = %s, want %s", got, tt.want)
			}
		})
	}
}

// TestBuildBillingUsageFourSourcesSameSemantics 验收标准：四个来源在
// 无缓存、缓存读取、缓存写入、模态明细、reasoning 样本下生成相同语义的 JSON。
func TestBuildBillingUsageFourSourcesSameSemantics(t *testing.T) {
	// 同一语义样本：raw 输入 200（其中缓存读取 30、缓存写入 20），
	// 输入音频 25、输入图像 15、输出 100（文本 60、音频 30、推理 10）。
	claudeUsage := buildUsageForTest(func(u *shared.Usage) {
		u.PromptTokens = 200 // Claude: input(150)+read(30)+write(20)
		u.CompletionTokens = 100
		u.PromptTokensDetails.CachedTokens = 30
		u.PromptTokensDetails.CachedCreationTokens = 20
	})
	openaiChatUsage := buildUsageForTest(func(u *shared.Usage) {
		u.PromptTokens = 200
		u.CompletionTokens = 100
		u.PromptTokensDetails = shared.InputTokenDetails{
			CachedTokens:         30,
			CachedCreationTokens: 20,
			AudioTokens:          25,
			ImageTokens:          15,
		}
		u.CompletionTokenDetails = shared.OutputTokenDetails{
			TextTokens:      60,
			AudioTokens:     30,
			ReasoningTokens: 10,
		}
	})
	responsesUsage := buildUsageForTest(func(u *shared.Usage) {
		u.InputTokens = 200
		u.OutputTokens = 100
		u.PromptTokens = 200
		u.CompletionTokens = 100
		u.InputTokensDetails = &shared.InputTokenDetails{CachedTokens: 30, CachedCreationTokens: 20}
		u.OutputTokensDetails = &shared.OutputTokenDetails{ReasoningTokens: 10}
	})
	geminiMetadata := &shared.GeminiUsageMetadata{
		PromptTokenCount:        200,
		CandidatesTokenCount:    90,
		ThoughtsTokenCount:      10,
		CachedContentTokenCount: 30,
		PromptTokensDetails: []shared.GeminiPromptTokensDetails{
			{Modality: "TEXT", TokenCount: 160},
			{Modality: "AUDIO", TokenCount: 25},
			{Modality: "IMAGE", TokenCount: 15},
		},
		CandidatesTokensDetails: []shared.GeminiPromptTokensDetails{
			{Modality: "TEXT", TokenCount: 60},
			{Modality: "AUDIO", TokenCount: 30},
		},
	}

	// 每个来源可观测的官方拆分不同（Claude 无模态字段、Responses 无输出音频），
	// 但同一来源内部的语义字段（InputTokens/OutputTokens/缓存拆分）必须一致。
	claudeBU, _, err := BuildBillingUsage(relayconstant.UsageSourceClaude, claudeUsage, nil)
	if err != nil {
		t.Fatalf("claude: %v", err)
	}
	if claudeBU.InputTokens() != 150 || claudeBU.OutputTokens != 100 ||
		claudeBU.CacheReadTokens != 30 || claudeBU.CacheWriteTokens != 20 {
		t.Fatalf("claude semantic = input:%d output:%d read:%d write:%d",
			claudeBU.InputTokens(), claudeBU.OutputTokens, claudeBU.CacheReadTokens, claudeBU.CacheWriteTokens)
	}

	chatBU, _, err := BuildBillingUsage(relayconstant.UsageSourceOpenAIChat, openaiChatUsage, nil)
	if err != nil {
		t.Fatalf("openai chat: %v", err)
	}
	if chatBU.InputTokens() != 150 || chatBU.OutputTokens != 100 ||
		chatBU.CacheReadTokens != 30 || chatBU.CacheWriteTokens != 20 {
		t.Fatalf("openai chat semantic = input:%d output:%d read:%d write:%d",
			chatBU.InputTokens(), chatBU.OutputTokens, chatBU.CacheReadTokens, chatBU.CacheWriteTokens)
	}

	responsesBU, _, err := BuildBillingUsage(relayconstant.UsageSourceOpenAIResponses, responsesUsage, nil)
	if err != nil {
		t.Fatalf("openai responses: %v", err)
	}
	if responsesBU.InputTokens() != 150 || responsesBU.OutputTokens != 100 ||
		responsesBU.CacheReadTokens != 30 || responsesBU.CacheWriteTokens != 20 {
		t.Fatalf("openai responses semantic = input:%d output:%d read:%d write:%d",
			responsesBU.InputTokens(), responsesBU.OutputTokens, responsesBU.CacheReadTokens, responsesBU.CacheWriteTokens)
	}

	geminiBU, _, err := BuildBillingUsage(relayconstant.UsageSourceGemini, nil, geminiMetadata)
	if err != nil {
		t.Fatalf("gemini: %v", err)
	}
	if geminiBU.InputTokens() != 170 || geminiBU.OutputTokens != 100 || geminiBU.CacheReadTokens != 30 {
		t.Fatalf("gemini semantic = input:%d output:%d read:%d",
			geminiBU.InputTokens(), geminiBU.OutputTokens, geminiBU.CacheReadTokens)
	}
	if geminiBU.ToolUsePromptTokens != nil {
		t.Fatalf("tool use prompt tokens should be audit only, got %d", *geminiBU.ToolUsePromptTokens)
	}

	// JSON 断言：OpenAI Chat 与 Responses 的官方拆分差异仅体现在
	// 上游真实返回的字段上，缓存与 reasoning 语义完全一致。
	chatJSON, err := SerializeBillingUsage(chatBU)
	if err != nil {
		t.Fatalf("serialize chat: %v", err)
	}
	wantChat := `{"schema_version":1,"tokens":{"input":{"text_input":110,"image_input":15,"audio_input":25,"video_input":0,"document_input":0},"output":{"text_output":60,"audio_output":30,"image_output":0,"reasoning_output":10,"accepted_prediction":0,"rejected_prediction":0},"cache":{"read_cache":30,"write_cache":20,"write_cache_5m":20,"write_cache_1h":0}}}`
	if chatJSON != wantChat {
		t.Fatalf("chat JSON = %s, want %s", chatJSON, wantChat)
	}
	responsesJSON, err := SerializeBillingUsage(responsesBU)
	if err != nil {
		t.Fatalf("serialize responses: %v", err)
	}
	wantResponses := `{"schema_version":1,"tokens":{"input":{"text_input":150,"image_input":0,"audio_input":0,"video_input":0,"document_input":0},"output":{"text_output":100,"audio_output":0,"image_output":0,"reasoning_output":10,"accepted_prediction":0,"rejected_prediction":0},"cache":{"read_cache":30,"write_cache":20,"write_cache_5m":20,"write_cache_1h":0}}}`
	if responsesJSON != wantResponses {
		t.Fatalf("responses JSON = %s, want %s", responsesJSON, wantResponses)
	}
	geminiJSON, err := SerializeBillingUsage(geminiBU)
	if err != nil {
		t.Fatalf("serialize gemini: %v", err)
	}
	wantGemini := `{"schema_version":1,"tokens":{"input":{"text_input":130,"image_input":15,"audio_input":25,"video_input":0,"document_input":0},"output":{"text_output":60,"audio_output":30,"image_output":0,"reasoning_output":10,"accepted_prediction":0,"rejected_prediction":0},"cache":{"read_cache":30,"write_cache":0,"write_cache_5m":0,"write_cache_1h":0}}}`
	if geminiJSON != wantGemini {
		t.Fatalf("gemini JSON = %s, want %s", geminiJSON, wantGemini)
	}
}

// TestBuildBillingUsageOpenAIResponsesDoesNotFabricateModalities 验证
// Responses 来源不虚构不存在的模态明细。
func TestBuildBillingUsageOpenAIResponsesDoesNotFabricateModalities(t *testing.T) {
	usage := buildUsageForTest(func(u *shared.Usage) {
		u.InputTokens = 100
		u.OutputTokens = 50
		u.PromptTokens = 100
		u.CompletionTokens = 50
	})
	bu, _, err := BuildBillingUsage(relayconstant.UsageSourceOpenAIResponses, usage, nil)
	if err != nil {
		t.Fatalf("BuildBillingUsage() error = %v", err)
	}
	if bu.TextInputTokens != nil || bu.AudioInputTokens != nil || bu.ImageInputTokens != nil ||
		bu.AudioOutputTokens != nil || bu.ImageOutputTokens != nil {
		t.Fatalf("responses must not fabricate modality details: %+v", bu)
	}
}

// TestBuildBillingUsageGeminiToolUseNotInPricingInput 验证 Gemini 的
// toolUsePromptTokenCount 只审计，不进入输入总量与模态明细。
func TestBuildBillingUsageGeminiToolUseNotInPricingInput(t *testing.T) {
	metadata := &shared.GeminiUsageMetadata{
		PromptTokenCount:        151,
		ToolUsePromptTokenCount: 18329,
		CandidatesTokenCount:    1089,
		ThoughtsTokenCount:      1120,
		PromptTokensDetails: []shared.GeminiPromptTokensDetails{
			{Modality: "TEXT", TokenCount: 151},
		},
		ToolUsePromptTokensDetails: []shared.GeminiPromptTokensDetails{
			{Modality: "TEXT", TokenCount: 18329},
		},
	}
	bu, _, err := BuildBillingUsage(relayconstant.UsageSourceGemini, nil, metadata)
	if err != nil {
		t.Fatalf("BuildBillingUsage() error = %v", err)
	}
	if bu.PromptAggregateTokens != 151 {
		t.Fatalf("prompt aggregate = %d, want 151 (official total excludes tool use)", bu.PromptAggregateTokens)
	}
	if bu.TextInputTokens == nil || *bu.TextInputTokens != 151 {
		t.Fatalf("text input = %v, want 151 (tool use details excluded)", bu.TextInputTokens)
	}
	if bu.ToolUsePromptTokens == nil || *bu.ToolUsePromptTokens != 18329 {
		t.Fatalf("tool use audit = %v, want 18329", bu.ToolUsePromptTokens)
	}
	if bu.TotalProcessedTokens() != 151+1089+1120 {
		t.Fatalf("total processed = %d, want %d (tool use excluded)", bu.TotalProcessedTokens(), 151+1089+1120)
	}
	json, err := SerializeBillingUsage(bu)
	if err != nil {
		t.Fatalf("serialize: %v", err)
	}
	// metadata 未返回 CandidatesTokensDetails，text_output 不虚构。
	want := `{"schema_version":1,"tokens":{"input":{"text_input":151,"image_input":0,"audio_input":0,"video_input":0,"document_input":0},"output":{"text_output":2209,"audio_output":0,"image_output":0,"reasoning_output":1120,"accepted_prediction":0,"rejected_prediction":0},"cache":{"read_cache":0,"write_cache":0,"write_cache_5m":0,"write_cache_1h":0}}}`
	if json != want {
		t.Fatalf("JSON = %s, want %s", json, want)
	}
}

// TestBuildBillingUsageGeminiUnknownModalityWarns 验证 schema v1 不能表达的
// 新 Gemini 模态不会被静默丢成“没有异常”；至少保留可诊断告警，供上游协议
// 升级时定位。
func TestBuildBillingUsageGeminiUnknownModalityWarns(t *testing.T) {
	metadata := &shared.GeminiUsageMetadata{
		PromptTokenCount:     12,
		CandidatesTokenCount: 8,
		PromptTokensDetails: []shared.GeminiPromptTokensDetails{
			{Modality: "NEW_INPUT", TokenCount: 5},
		},
		CandidatesTokensDetails: []shared.GeminiPromptTokensDetails{
			{Modality: "NEW_OUTPUT", TokenCount: 3},
		},
	}
	_, warnings, err := BuildBillingUsage(relayconstant.UsageSourceGemini, nil, metadata)
	if err != nil {
		t.Fatalf("BuildBillingUsage() error = %v", err)
	}
	joined := strings.Join(warnings, "\n")
	for _, want := range []string{
		`unknown gemini input modality "NEW_INPUT" with 5 tokens`,
		`unknown gemini output modality "NEW_OUTPUT" with 3 tokens`,
	} {
		if !strings.Contains(joined, want) {
			t.Fatalf("warnings = %v, want substring %q", warnings, want)
		}
	}
}

// TestBuildBillingUsageRejectsIntegerOverflow 验证 token 求和不因 int 溢出
// 回绕成“合法的小值”。
func TestBuildBillingUsageRejectsIntegerOverflow(t *testing.T) {
	outputMetadata := &shared.GeminiUsageMetadata{
		CandidatesTokenCount: math.MaxInt,
		ThoughtsTokenCount:   1,
	}
	if _, _, err := BuildBillingUsage(relayconstant.UsageSourceGemini, nil, outputMetadata); err == nil {
		t.Fatal("gemini output total overflow must fail explicitly")
	}

	modalityMetadata := &shared.GeminiUsageMetadata{
		PromptTokenCount: math.MaxInt,
		PromptTokensDetails: []shared.GeminiPromptTokensDetails{
			{Modality: "TEXT", TokenCount: math.MaxInt},
			{Modality: "TEXT", TokenCount: math.MaxInt},
		},
	}
	if _, _, err := BuildBillingUsage(relayconstant.UsageSourceGemini, nil, modalityMetadata); err == nil {
		t.Fatal("modality sum overflow must fail explicitly")
	}
}

// TestBuildBillingUsageRealtime 验证 Realtime/WSS 用量按 Responses 同族规则归一化。
func TestBuildBillingUsageRealtime(t *testing.T) {
	usage := &shared.RealtimeUsage{
		TotalTokens:  300,
		InputTokens:  200,
		OutputTokens: 100,
		InputTokenDetails: shared.InputTokenDetails{
			CachedTokens: 40,
			TextTokens:   120,
			AudioTokens:  40,
		},
		OutputTokenDetails: shared.OutputTokenDetails{
			TextTokens:  70,
			AudioTokens: 30,
		},
	}
	bu, warnings, err := BuildRealtimeBillingUsage(relayconstant.UsageSourceOpenAIResponses, usage)
	if err != nil {
		t.Fatalf("BuildRealtimeBillingUsage() error = %v", err)
	}
	if len(warnings) != 0 {
		t.Fatalf("warnings = %v", warnings)
	}
	json, err := SerializeBillingUsage(bu)
	if err != nil {
		t.Fatalf("serialize: %v", err)
	}
	want := `{"schema_version":1,"tokens":{"input":{"text_input":120,"image_input":0,"audio_input":40,"video_input":0,"document_input":0},"output":{"text_output":70,"audio_output":30,"image_output":0,"reasoning_output":0,"accepted_prediction":0,"rejected_prediction":0},"cache":{"read_cache":40,"write_cache":0,"write_cache_5m":0,"write_cache_1h":0}}}`
	if json != want {
		t.Fatalf("JSON = %s, want %s", json, want)
	}
	if _, _, err := BuildRealtimeBillingUsage(relayconstant.UsageSourceClaude, usage); err == nil {
		t.Fatalf("realtime usage with claude source should fail explicitly")
	}
}

func TestBuildBillingUsageRealtimeKeepsExplicitZeroCache(t *testing.T) {
	usage := &shared.RealtimeUsage{
		InputTokens:  10,
		OutputTokens: 5,
		InputTokenDetails: shared.InputTokenDetails{
			CachedTokens:        0,
			CachedTokensPresent: true,
		},
	}
	bu, _, err := BuildRealtimeBillingUsage(relayconstant.UsageSourceOpenAIResponses, usage)
	if err != nil {
		t.Fatalf("BuildRealtimeBillingUsage() error = %v", err)
	}
	if !bu.CacheReadPresent {
		t.Fatalf("CacheReadPresent = false, want explicit upstream zero retained")
	}
	json, err := SerializeBillingUsage(bu)
	if err != nil {
		t.Fatalf("serialize: %v", err)
	}
	want := `{"schema_version":1,"tokens":{"input":{"text_input":10,"image_input":0,"audio_input":0,"video_input":0,"document_input":0},"output":{"text_output":5,"audio_output":0,"image_output":0,"reasoning_output":0,"accepted_prediction":0,"rejected_prediction":0},"cache":{"read_cache":0,"write_cache":0,"write_cache_5m":0,"write_cache_1h":0}}}`
	if json != want {
		t.Fatalf("JSON = %s, want %s", json, want)
	}
}

func TestClaudeParsedExplicitZerosSurviveNormalization(t *testing.T) {
	raw := []byte(`{
		"input_tokens": 0,
		"cache_creation_input_tokens": 0,
		"cache_read_input_tokens": 0,
		"output_tokens": 0,
		"output_tokens_details": {"thinking_tokens": 0},
		"cache_creation": {
			"ephemeral_5m_input_tokens": 0,
			"ephemeral_1h_input_tokens": 0
		}
	}`)
	var claudeUsage shared.ClaudeUsage
	if err := jsonx.Unmarshal(raw, &claudeUsage); err != nil {
		t.Fatalf("unmarshal Claude usage: %v", err)
	}
	usage := shared.ClaudeUsageToOpenAIUsage(&claudeUsage)
	bu, _, err := BuildBillingUsage(relayconstant.UsageSourceClaude, usage, nil)
	if err != nil {
		t.Fatalf("BuildBillingUsage: %v", err)
	}
	got, err := SerializeBillingUsage(bu)
	if err != nil {
		t.Fatalf("serialize: %v", err)
	}
	want := `{"schema_version":1,"tokens":{"input":{"text_input":0,"image_input":0,"audio_input":0,"video_input":0,"document_input":0},"output":{"text_output":0,"audio_output":0,"image_output":0,"reasoning_output":0,"accepted_prediction":0,"rejected_prediction":0},"cache":{"read_cache":0,"write_cache":0,"write_cache_5m":0,"write_cache_1h":0}}}`
	if got != want {
		t.Fatalf("JSON = %s, want %s", got, want)
	}
}

func TestGeminiParsedExplicitZeroCacheSurvivesNormalization(t *testing.T) {
	raw := []byte(`{
		"promptTokenCount": 10,
		"candidatesTokenCount": 5,
		"cachedContentTokenCount": 0,
		"promptTokensDetails": [{"modality": "TEXT", "tokenCount": 10}]
	}`)
	var metadata shared.GeminiUsageMetadata
	if err := jsonx.Unmarshal(raw, &metadata); err != nil {
		t.Fatalf("unmarshal Gemini metadata: %v", err)
	}
	if !metadata.CachedContentTokenCountPresent {
		t.Fatalf("explicit cachedContentTokenCount=0 lost during parsing")
	}
	bu, _, err := BuildBillingUsage(relayconstant.UsageSourceGemini, nil, &metadata)
	if err != nil {
		t.Fatalf("BuildBillingUsage: %v", err)
	}
	got, err := SerializeBillingUsage(bu)
	if err != nil {
		t.Fatalf("serialize: %v", err)
	}
	want := `{"schema_version":1,"tokens":{"input":{"text_input":10,"image_input":0,"audio_input":0,"video_input":0,"document_input":0},"output":{"text_output":5,"audio_output":0,"image_output":0,"reasoning_output":0,"accepted_prediction":0,"rejected_prediction":0},"cache":{"read_cache":0,"write_cache":0,"write_cache_5m":0,"write_cache_1h":0}}}`
	if got != want {
		t.Fatalf("JSON = %s, want %s", got, want)
	}
}

// TestBuildBillingUsageExplicitFailures 验证负数、分档大于总量、未知来源
// 显式失败。
func TestBuildBillingUsageExplicitFailures(t *testing.T) {
	tests := []struct {
		name   string
		source relayconstant.UsageSource
		usage  *shared.Usage
	}{
		{
			name:   "negative prompt tokens",
			source: relayconstant.UsageSourceOpenAIChat,
			usage:  buildUsageForTest(func(u *shared.Usage) { u.PromptTokens = -1 }),
		},
		{
			name:   "negative completion tokens",
			source: relayconstant.UsageSourceOpenAIChat,
			usage:  buildUsageForTest(func(u *shared.Usage) { u.CompletionTokens = -5 }),
		},
		{
			name:   "negative modality detail",
			source: relayconstant.UsageSourceOpenAIChat,
			usage: buildUsageForTest(func(u *shared.Usage) {
				u.PromptTokens = 100
				u.PromptTokensDetails.AudioTokens = -3
			}),
		},
		{
			name:   "cache tiers exceed total",
			source: relayconstant.UsageSourceClaude,
			usage: buildUsageForTest(func(u *shared.Usage) {
				u.PromptTokens = 100
				u.PromptTokensDetails.CachedCreationTokens = 50
				u.ClaudeCacheCreation5mTokens = 30
				u.ClaudeCacheCreation1hTokens = 30
			}),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, _, err := BuildBillingUsage(tt.source, tt.usage, nil)
			if err == nil {
				t.Fatalf("expected explicit error")
			}
		})
	}

	if _, _, err := BuildBillingUsage(relayconstant.UsageSourceNone, buildUsageForTest(func(u *shared.Usage) { u.PromptTokens = 1 }), nil); err == nil {
		t.Fatalf("unknown source should fail explicitly")
	}
	if _, _, err := BuildBillingUsage(relayconstant.UsageSourceGemini, buildUsageForTest(func(u *shared.Usage) { u.PromptTokens = 1 }), nil); err == nil {
		t.Fatalf("gemini source without metadata should fail explicitly")
	}
}

// TestBuildBillingUsageWarnings 验证明细大于官方总量保留可诊断告警，
// 且完整 JSON 序列化不得把矛盾明细裁剪或补成负数。
func TestBuildBillingUsageWarnings(t *testing.T) {
	usage := buildUsageForTest(func(u *shared.Usage) {
		u.PromptTokens = 100
		u.CompletionTokens = 50
		u.PromptTokensDetails.AudioTokens = 150 // 超出官方总量的矛盾明细
	})
	bu, warnings, err := BuildBillingUsage(relayconstant.UsageSourceOpenAIChat, usage, nil)
	if err != nil {
		t.Fatalf("BuildBillingUsage() error = %v", err)
	}
	if len(warnings) != 1 {
		t.Fatalf("warnings = %v, want 1 entry", warnings)
	}
	if _, err := SerializeBillingUsage(bu); err == nil {
		t.Fatalf("serialize contradictory modality details must fail explicitly")
	}
}

// TestBuildBillingUsageAcceptedRejectedPrediction 透传 Chat Completions
// 官方 accepted/rejected prediction 审计拆分。
func TestBuildBillingUsageAcceptedRejectedPrediction(t *testing.T) {
	usage := buildUsageForTest(func(u *shared.Usage) {
		u.PromptTokens = 100
		u.CompletionTokens = 50
		u.CompletionTokenDetails = shared.OutputTokenDetails{
			AcceptedPredictionTokens: 12,
			RejectedPredictionTokens: 3,
		}
	})
	bu, _, err := BuildBillingUsage(relayconstant.UsageSourceOpenAIChat, usage, nil)
	if err != nil {
		t.Fatalf("BuildBillingUsage() error = %v", err)
	}
	json, err := SerializeBillingUsage(bu)
	if err != nil {
		t.Fatalf("serialize: %v", err)
	}
	want := `{"schema_version":1,"tokens":{"input":{"text_input":100,"image_input":0,"audio_input":0,"video_input":0,"document_input":0},"output":{"text_output":50,"audio_output":0,"image_output":0,"reasoning_output":0,"accepted_prediction":12,"rejected_prediction":3},"cache":{"read_cache":0,"write_cache":0,"write_cache_5m":0,"write_cache_1h":0}}}`
	if json != want {
		t.Fatalf("JSON = %s, want %s", json, want)
	}
}

func TestBuildBillingUsageKeepsParsedExplicitZeros(t *testing.T) {
	raw := []byte(`{
		"prompt_tokens": 0,
		"completion_tokens": 0,
		"prompt_tokens_details": {
			"cached_tokens": 0,
			"cache_write_tokens": 0,
			"text_tokens": 0,
			"audio_tokens": 0,
			"image_tokens": 0
		},
		"completion_tokens_details": {
			"text_tokens": 0,
			"audio_tokens": 0,
			"reasoning_tokens": 0,
			"accepted_prediction_tokens": 0,
			"rejected_prediction_tokens": 0
		}
	}`)
	var usage shared.Usage
	if err := jsonx.Unmarshal(raw, &usage); err != nil {
		t.Fatalf("unmarshal usage: %v", err)
	}
	bu, _, err := BuildBillingUsage(relayconstant.UsageSourceOpenAIChat, &usage, nil)
	if err != nil {
		t.Fatalf("BuildBillingUsage: %v", err)
	}
	got, err := SerializeBillingUsage(bu)
	if err != nil {
		t.Fatalf("serialize: %v", err)
	}
	want := `{"schema_version":1,"tokens":{"input":{"text_input":0,"image_input":0,"audio_input":0,"video_input":0,"document_input":0},"output":{"text_output":0,"audio_output":0,"image_output":0,"reasoning_output":0,"accepted_prediction":0,"rejected_prediction":0},"cache":{"read_cache":0,"write_cache":0,"write_cache_5m":0,"write_cache_1h":0}}}`
	if got != want {
		t.Fatalf("JSON = %s, want %s", got, want)
	}
}
