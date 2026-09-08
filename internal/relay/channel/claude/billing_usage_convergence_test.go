package claude

import (
	"net/http"
	"net/http/httptest"
	"testing"

	relaycommon "github.com/NookMux/NookMux/internal/relay/common"
	relayconstant "github.com/NookMux/NookMux/internal/relay/constant"

	"github.com/NookMux/NookMux/internal/domain/billing"
	"github.com/NookMux/NookMux/internal/domain/shared"
	"github.com/NookMux/NookMux/pkg/jsonx"

	"github.com/gin-gonic/gin"
)

// wantBillingDetails 是同一 Claude 返回（input 150 / output 100 /
// cache_read 30 / cache_creation 40，其中 5m 30、1h 10）在任何路径下
// 都应产生的 billing_details JSON。
const wantBillingDetails = `{"schema_version":1,"tokens":{"input":{"text_input":150,"image_input":0,"audio_input":0,"video_input":0,"document_input":0},"output":{"text_output":100,"audio_output":0,"image_output":0,"reasoning_output":0,"accepted_prediction":0,"rejected_prediction":0},"cache":{"read_cache":30,"write_cache":40,"write_cache_5m":30,"write_cache_1h":10}}}`

func claudeUsageFixture() *shared.ClaudeUsage {
	return &shared.ClaudeUsage{
		InputTokens:              150,
		OutputTokens:             100,
		CacheReadInputTokens:     30,
		CacheCreationInputTokens: 40,
		CacheCreation: &shared.ClaudeCacheCreationUsage{
			Ephemeral5mInputTokens: 30,
			Ephemeral1hInputTokens: 10,
		},
	}
}

func mustBuildBillingDetails(t *testing.T, usage *shared.Usage) string {
	t.Helper()
	bu, warnings, err := billing.BuildBillingUsage(relayconstant.UsageSourceClaude, usage, nil)
	if err != nil {
		t.Fatalf("BuildBillingUsage() error = %v", err)
	}
	if len(warnings) != 0 {
		t.Fatalf("warnings = %v", warnings)
	}
	raw, err := billing.SerializeBillingUsage(bu)
	if err != nil {
		t.Fatalf("SerializeBillingUsage() error = %v", err)
	}
	return raw
}

// TestClaudeUsagePathsConvergeToSameBillingDetails 验收标准：同一 Claude 返回
// 经原生非流式（ClaudeUsageToOpenAIUsage）与流式
// （mergeClaudeUsageIntoOpenAIUsage）路径转换后的语义字段一致。
// AWS Bedrock 复用路径与 OpenRouter 原生 /api/v1/messages 都调用这两条
// claude 包入口（awsHandler/HandleClaudeResponseData、awsStreamHandler/
// HandleStreamResponseData），因此天然收敛到同一规则。
func TestClaudeUsagePathsConvergeToSameBillingDetails(t *testing.T) {
	nativeJSON := mustBuildBillingDetails(t, shared.ClaudeUsageToOpenAIUsage(claudeUsageFixture()))

	// 流式：message_start 携带完整用量，message_delta 只补输出总量。
	streamUsage := mergeClaudeUsageIntoOpenAIUsage(&shared.Usage{}, claudeUsageFixture())
	streamJSON := mustBuildBillingDetails(t, streamUsage)

	if nativeJSON != wantBillingDetails {
		t.Fatalf("native path JSON = %s, want %s", nativeJSON, wantBillingDetails)
	}
	if streamJSON != wantBillingDetails {
		t.Fatalf("stream path JSON = %s, want %s", streamJSON, wantBillingDetails)
	}
}

// TestClaudeStreamMergePreservesOfficialTiers 验证流式 message_delta 只带
// 输出总量时，官方 5m/1h 分档不被未分档转换规则覆盖。
func TestClaudeStreamMergePreservesOfficialTiers(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/messages", nil)
	info := &relaycommon.RelayInfo{
		RelayFormat: relayconstant.RelayFormatClaude,
		ChannelMeta: &relaycommon.ChannelMeta{UpstreamModelName: "claude-sonnet-4-5"},
	}
	claudeInfo := &ClaudeResponseInfo{Usage: &shared.Usage{}}

	messageStart := `{"type":"message_start","message":{"id":"msg_1","model":"claude-sonnet-4-5","usage":{"input_tokens":150,"output_tokens":10,"cache_read_input_tokens":30,"cache_creation_input_tokens":40,"cache_creation":{"ephemeral_5m_input_tokens":30,"ephemeral_1h_input_tokens":10}}}}`
	if apiErr := HandleStreamResponseData(c, info, claudeInfo, messageStart); apiErr != nil {
		t.Fatalf("message_start error: %v", apiErr)
	}
	messageDelta := `{"type":"message_delta","delta":{"stop_reason":"end_turn"},"usage":{"output_tokens":100}}`
	if apiErr := HandleStreamResponseData(c, info, claudeInfo, messageDelta); apiErr != nil {
		t.Fatalf("message_delta error: %v", apiErr)
	}

	if got := mustBuildBillingDetails(t, claudeInfo.Usage); got != wantBillingDetails {
		t.Fatalf("stream events JSON = %s, want %s", got, wantBillingDetails)
	}
	if info.UsageSource != relayconstant.UsageSourceClaude {
		t.Fatalf("usage source = %q, want %q", info.UsageSource, relayconstant.UsageSourceClaude)
	}
}

// TestHandleClaudeResponseDataTagsUsageSource 验证非流式解析点显式标识
// Claude 来源（原生、AWS、Vertex、OpenRouter /messages 共用该入口）。
func TestHandleClaudeResponseDataTagsUsageSource(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/messages", nil)
	info := &relaycommon.RelayInfo{
		RelayFormat: relayconstant.RelayFormatClaude,
		ChannelMeta: &relaycommon.ChannelMeta{UpstreamModelName: "claude-sonnet-4-5"},
	}
	claudeInfo := &ClaudeResponseInfo{Usage: &shared.Usage{}}

	responseBody, err := jsonx.Marshal(shared.ClaudeResponse{
		Type:  "message",
		Id:    "msg_1",
		Model: "claude-sonnet-4-5",
		Usage: claudeUsageFixture(),
	})
	if err != nil {
		t.Fatalf("marshal response: %v", err)
	}
	if apiErr := HandleClaudeResponseData(c, info, claudeInfo, nil, responseBody); apiErr != nil {
		t.Fatalf("HandleClaudeResponseData error: %v", apiErr)
	}
	if info.UsageSource != relayconstant.UsageSourceClaude {
		t.Fatalf("usage source = %q, want %q", info.UsageSource, relayconstant.UsageSourceClaude)
	}
	if got := mustBuildBillingDetails(t, claudeInfo.Usage); got != wantBillingDetails {
		t.Fatalf("non-stream JSON = %s, want %s", got, wantBillingDetails)
	}
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", w.Code, w.Body.String())
	}
}
