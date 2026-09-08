package billing

import (
	"fmt"
	"math"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/NookMux/NookMux/internal/common"
	"github.com/NookMux/NookMux/internal/httpapi"
	relaycommon "github.com/NookMux/NookMux/internal/relay/common"
	relayconstant "github.com/NookMux/NookMux/internal/relay/constant"

	"github.com/NookMux/NookMux/internal/domain/shared"
	"github.com/gin-gonic/gin"
)

// TestParseBillingDetailsJSONRoundTrip 验证 canonical 写出与读取互逆。
func TestParseBillingDetailsJSONRoundTrip(t *testing.T) {
	bu := &BillingUsage{
		Source:                "openai_chat",
		PromptAggregateTokens: 200,
		OutputTokens:          100,
		CacheReadTokens:       30,
		CacheWriteTokens:      20,
	}
	// 无分档总量按 PRD 转换规则进 5m 分档，读取端应还原同一语义。
	bu.CacheWrite5mTokens = &bu.CacheWriteTokens
	raw, err := SerializeBillingUsage(bu)
	if err != nil {
		t.Fatalf("serialize: %v", err)
	}
	payload, err := ParseBillingDetailsJSON(raw)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if payload.SchemaVersion != BillingDetailsSchemaVersion {
		t.Fatalf("schema version = %d", payload.SchemaVersion)
	}
	if payload.Tokens.Cache.ReadCache != 30 {
		t.Fatalf("read cache = %v", payload.Tokens.Cache.ReadCache)
	}
	if payload.Tokens.Cache.WriteCache != 20 {
		t.Fatalf("write cache = %v", payload.Tokens.Cache.WriteCache)
	}
	if payload.Tokens.Cache.WriteCache5m != 20 {
		t.Fatalf("write cache 5m = %v", payload.Tokens.Cache.WriteCache5m)
	}
	// 上游未返回的模态拆分统一写 0；文本拆分按总量恢复。
	if payload.Tokens.Input.TextInput != 150 {
		t.Fatalf("text input = %d, want 150", payload.Tokens.Input.TextInput)
	}
	if payload.Tokens.Input.ImageInput != 0 {
		t.Fatalf("absent image input = %d, want 0", payload.Tokens.Input.ImageInput)
	}
}

// TestParseBillingDetailsJSONExplicitErrors 验证历史空值、损坏 JSON、未知版本、
// 未知字段、负数与分档大于总量的显式错误路径。
func TestParseBillingDetailsJSONExplicitErrors(t *testing.T) {
	tests := []struct {
		name string
		raw  string
	}{
		{"empty string", ""},
		{"broken json", `{"schema_version":1,"tokens":`},
		{"unknown schema version", `{"schema_version":2,"tokens":{"input":{},"output":{},"cache":{}}}`},
		{"unknown top level field", `{"schema_version":1,"tokens":{"input":{},"output":{},"cache":{}},"provider":"claude"}`},
		{"unknown tokens group", `{"schema_version":1,"tokens":{"input":{},"output":{},"cache":{},"tool_use":{}}}`},
		{"unknown input field", `{"schema_version":1,"tokens":{"input":{"cached_input":5},"output":{},"cache":{}}}`},
		{"unknown output field", `{"schema_version":1,"tokens":{"input":{},"output":{"prediction":5},"cache":{}}}`},
		{"negative token", `{"schema_version":1,"tokens":{"input":{"text_input":-1},"output":{},"cache":{}}}`},
		{"tiers exceed write total", `{"schema_version":1,"tokens":{"input":{},"output":{},"cache":{"write_cache":10,"write_cache_5m":8,"write_cache_1h":8}}}`},
		{
			name: "tier sum overflows",
			raw: fmt.Sprintf(
				`{"schema_version":1,"tokens":{"input":{},"output":{},"cache":{"write_cache":%d,"write_cache_5m":%d,"write_cache_1h":%d}}}`,
				math.MaxInt, math.MaxInt, math.MaxInt,
			),
		},
		{"negative schema version", `{"schema_version":-1,"tokens":{"input":{},"output":{},"cache":{}}}`},
		{"schema version is fractional", `{"schema_version":1.0,"tokens":{"input":{},"output":{},"cache":{}}}`},
		{"schema version is null", `{"schema_version":null,"tokens":{"input":{},"output":{},"cache":{}}}`},
		{"fractional token", `{"schema_version":1,"tokens":{"input":{"text_input":1.5},"output":{},"cache":{}}}`},
		{"tokens not object", `{"schema_version":1,"tokens":[]}`},
		{"missing tokens", `{"schema_version":1}`},
		{"missing all groups", `{"schema_version":1,"tokens":{}}`},
		{"missing output and cache", `{"schema_version":1,"tokens":{"input":{}}}`},
		{"missing cache", `{"schema_version":1,"tokens":{"input":{},"output":{}}}`},
		{"missing token field", `{"schema_version":1,"tokens":{"input":{"text_input":1},"output":{},"cache":{}}}`},
		{"null token field", `{"schema_version":1,"tokens":{"input":{"text_input":null},"output":{},"cache":{}}}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := ParseBillingDetailsJSON(tt.raw); err == nil {
				t.Fatalf("expected explicit error for %q", tt.raw)
			}
		})
	}
}

// TestParseBillingDetailsJSONExplicitZero 验证官方显式 0 合法保留。
func TestParseBillingDetailsJSONExplicitZero(t *testing.T) {
	raw := `{"schema_version":1,"tokens":{"input":{"text_input":0,"image_input":0,"audio_input":0,"video_input":0,"document_input":0},"output":{"text_output":0,"audio_output":0,"image_output":0,"reasoning_output":0,"accepted_prediction":0,"rejected_prediction":0},"cache":{"read_cache":0,"write_cache":0,"write_cache_5m":0,"write_cache_1h":0}}}`
	payload, err := ParseBillingDetailsJSON(raw)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	for _, value := range []int{
		payload.Tokens.Input.TextInput,
		payload.Tokens.Input.ImageInput,
		payload.Tokens.Input.AudioInput,
		payload.Tokens.Input.VideoInput,
		payload.Tokens.Input.DocumentInput,
		payload.Tokens.Output.TextOutput,
		payload.Tokens.Output.AudioOutput,
		payload.Tokens.Output.ImageOutput,
		payload.Tokens.Output.ReasoningOutput,
		payload.Tokens.Output.AcceptedPrediction,
		payload.Tokens.Output.RejectedPrediction,
		payload.Tokens.Cache.ReadCache,
		payload.Tokens.Cache.WriteCache,
		payload.Tokens.Cache.WriteCache5m,
		payload.Tokens.Cache.WriteCache1h,
	} {
		if value != 0 {
			t.Fatalf("explicit zero must be preserved, got %d", value)
		}
	}
}

// TestSerializeBillingUsageCanonicalFormat 验证 canonical JSON：固定字段顺序、
// 空分组保留、无多余空白。
func TestSerializeBillingUsageCanonicalFormat(t *testing.T) {
	writeCache := 20
	bu := &BillingUsage{
		Source:                "claude",
		PromptAggregateTokens: 200,
		OutputTokens:          100,
		CacheWriteTokens:      20,
		CacheWrite5mTokens:    &writeCache,
	}
	raw, err := SerializeBillingUsage(bu)
	if err != nil {
		t.Fatalf("serialize: %v", err)
	}
	if strings.Contains(raw, " ") || strings.Contains(raw, "\n") {
		t.Fatalf("canonical JSON must not contain whitespace: %s", raw)
	}
	// 固定字段顺序：schema_version -> tokens -> input -> output -> cache。
	want := `{"schema_version":1,"tokens":{"input":{"text_input":180,"image_input":0,"audio_input":0,"video_input":0,"document_input":0},"output":{"text_output":100,"audio_output":0,"image_output":0,"reasoning_output":0,"accepted_prediction":0,"rejected_prediction":0},"cache":{"read_cache":0,"write_cache":20,"write_cache_5m":20,"write_cache_1h":0}}}`
	if raw != want {
		t.Fatalf("JSON = %s, want %s", raw, want)
	}
}

// TestBuildBillingDetailsForLogSkips 覆盖落库入口的跳过规则：本地估算、
// 无 token 用量、来源未标识都不生成 JSON；只有上游真实用量才产出。
func TestBuildBillingDetailsForLogSkips(t *testing.T) {
	gin.SetMode(gin.TestMode)

	newCtx := func() *gin.Context {
		c, _ := gin.CreateTestContext(httptest.NewRecorder())
		c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
		return c
	}
	realUsage := buildUsageForTest(func(u *shared.Usage) {
		u.PromptTokens = 100
		u.CompletionTokens = 50
	})

	t.Run("estimated usage skipped", func(t *testing.T) {
		c := newCtx()
		httpapi.SetContextKey(c, common.ContextKeyLocalCountTokens, true)
		if got := BuildBillingDetailsForLog(c, &relaycommon.RelayInfo{UsageSource: relayconstant.UsageSourceOpenAIChat}, realUsage); got != "" {
			t.Fatalf("estimated usage should skip billing_details, got %s", got)
		}
	})

	t.Run("zero usage skipped", func(t *testing.T) {
		c := newCtx()
		if got := BuildBillingDetailsForLog(c, &relaycommon.RelayInfo{UsageSource: relayconstant.UsageSourceOpenAIChat}, &shared.Usage{}); got != "" {
			t.Fatalf("zero usage should skip billing_details, got %s", got)
		}
	})

	t.Run("unidentified source fails with diagnosable path", func(t *testing.T) {
		c := newCtx()
		if got := BuildBillingDetailsForLog(c, &relaycommon.RelayInfo{}, realUsage); got != "" {
			t.Fatalf("unidentified source should skip billing_details, got %s", got)
		}
	})

	t.Run("real usage serialized", func(t *testing.T) {
		c := newCtx()
		relayInfo := &relaycommon.RelayInfo{UsageSource: relayconstant.UsageSourceOpenAIChat}
		got := BuildBillingDetailsForLog(c, relayInfo, realUsage)
		want := `{"schema_version":1,"tokens":{"input":{"text_input":100,"image_input":0,"audio_input":0,"video_input":0,"document_input":0},"output":{"text_output":50,"audio_output":0,"image_output":0,"reasoning_output":0,"accepted_prediction":0,"rejected_prediction":0},"cache":{"read_cache":0,"write_cache":0,"write_cache_5m":0,"write_cache_1h":0}}}`
		if got != want {
			t.Fatalf("JSON = %s, want %s", got, want)
		}
	})
}

// TestBuildRealtimeBillingDetailsForLogSkips 覆盖 Realtime/WSS 落库入口的
// 跳过规则与来源守卫。
func TestBuildRealtimeBillingDetailsForLogSkips(t *testing.T) {
	gin.SetMode(gin.TestMode)

	newCtx := func() *gin.Context {
		c, _ := gin.CreateTestContext(httptest.NewRecorder())
		c.Request = httptest.NewRequest(http.MethodGet, "/v1/realtime", nil)
		return c
	}
	realtimeUsage := &shared.RealtimeUsage{
		InputTokens:  200,
		OutputTokens: 100,
		InputTokenDetails: shared.InputTokenDetails{
			CachedTokens: 30,
			TextTokens:   170,
		},
	}

	t.Run("unidentified source fails with diagnosable path", func(t *testing.T) {
		c := newCtx()
		if got := BuildRealtimeBillingDetailsForLog(c, &relaycommon.RelayInfo{}, realtimeUsage); got != "" {
			t.Fatalf("unidentified source should skip billing_details, got %s", got)
		}
	})

	t.Run("wrong source rejected explicitly", func(t *testing.T) {
		c := newCtx()
		relayInfo := &relaycommon.RelayInfo{UsageSource: relayconstant.UsageSourceClaude}
		if got := BuildRealtimeBillingDetailsForLog(c, relayInfo, realtimeUsage); got != "" {
			t.Fatalf("claude source must not normalize realtime usage, got %s", got)
		}
	})

	t.Run("real usage serialized", func(t *testing.T) {
		c := newCtx()
		relayInfo := &relaycommon.RelayInfo{UsageSource: relayconstant.UsageSourceOpenAIResponses}
		got := BuildRealtimeBillingDetailsForLog(c, relayInfo, realtimeUsage)
		want := `{"schema_version":1,"tokens":{"input":{"text_input":170,"image_input":0,"audio_input":0,"video_input":0,"document_input":0},"output":{"text_output":100,"audio_output":0,"image_output":0,"reasoning_output":0,"accepted_prediction":0,"rejected_prediction":0},"cache":{"read_cache":30,"write_cache":0,"write_cache_5m":0,"write_cache_1h":0}}}`
		if got != want {
			t.Fatalf("billing_details = %s, want %s", got, want)
		}
	})

	t.Run("zero usage skipped", func(t *testing.T) {
		c := newCtx()
		relayInfo := &relaycommon.RelayInfo{UsageSource: relayconstant.UsageSourceOpenAIResponses}
		if got := BuildRealtimeBillingDetailsForLog(c, relayInfo, &shared.RealtimeUsage{}); got != "" {
			t.Fatalf("zero usage should skip billing_details, got %s", got)
		}
	})
}
