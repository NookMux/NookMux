package claude

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/NookMux/NookMux/internal/domain/shared"
	relaycommon "github.com/NookMux/NookMux/internal/relay/common"
	relayconstant "github.com/NookMux/NookMux/internal/relay/constant"
	"github.com/gin-gonic/gin"
)

// 官方文档（https://docs.anthropic.com/en/api/streaming#error-events）定义的
// 流式错误事件格式：{"type": "error", "error": {"type": ..., "message": ...}}
// 拦截后应按错误类型还原真实 HTTP 状态码（https://docs.anthropic.com/en/api/errors）。
func TestStreamErrorEventMapsToOfficialStatusCode(t *testing.T) {
	cases := []struct {
		name         string
		errorPayload string
		wantStatus   int
	}{
		{
			name:         "overloaded_error maps to 529",
			errorPayload: `{"type": "error", "error": {"type": "overloaded_error", "message": "Overloaded"}}`,
			wantStatus:   529,
		},
		{
			name:         "rate_limit_error maps to 429",
			errorPayload: `{"type": "error", "error": {"type": "rate_limit_error", "message": "Number of requests too high"}}`,
			wantStatus:   http.StatusTooManyRequests,
		},
		{
			name:         "api_error maps to 500",
			errorPayload: `{"type": "error", "error": {"type": "api_error", "message": "Internal server error"}}`,
			wantStatus:   http.StatusInternalServerError,
		},
		{
			name:         "timeout_error maps to 504",
			errorPayload: `{"type": "error", "error": {"type": "timeout_error", "message": "Request timed out"}}`,
			wantStatus:   http.StatusGatewayTimeout,
		},
		{
			name:         "invalid_request_error maps to 400",
			errorPayload: `{"type": "error", "error": {"type": "invalid_request_error", "message": "max_tokens is required"}}`,
			wantStatus:   http.StatusBadRequest,
		},
		{
			name:         "error with request_id keeps mapping",
			errorPayload: `{"type": "error", "error": {"type": "not_found_error", "message": "The requested resource could not be found."}, "request_id": "req_011CSHoEeqs5C35K2UUqR7Fy"}`,
			wantStatus:   http.StatusNotFound,
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Request = httptest.NewRequest(http.MethodPost, "/v1/messages", nil)

			info := &relaycommon.RelayInfo{
				RelayFormat: relayconstant.RelayFormatClaude,
				ChannelMeta: &relaycommon.ChannelMeta{},
			}
			claudeInfo := &ClaudeResponseInfo{Usage: &shared.Usage{}}

			apiErr := HandleStreamResponseData(c, info, claudeInfo, testCase.errorPayload)
			if apiErr == nil {
				t.Fatalf("error event not intercepted")
			}
			if apiErr.StatusCode != testCase.wantStatus {
				t.Errorf("statusCode = %d, want %d", apiErr.StatusCode, testCase.wantStatus)
			}
			if apiErr.GetErrorType() != shared.ErrorTypeClaudeError {
				t.Errorf("errorType = %s, want claude_error", apiErr.GetErrorType())
			}
		})
	}
}

// 流式超时（上游 200 后挂死、未发送任何计费数据）必须作为真实错误上报，
// 而不是静默返回零 usage，让计费层伪造「502 上游没有返回计费信息」。
func TestStreamTimeoutSurfacedAsUpstreamError(t *testing.T) {
	originalTimeout := shared.StreamingTimeout
	shared.StreamingTimeout = 1
	defer func() { shared.StreamingTimeout = originalTimeout }()

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		w.(http.Flusher).Flush()
		<-r.Context().Done()
	}))
	defer upstream.Close()

	req, _ := http.NewRequest(http.MethodPost, upstream.URL, nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("do request: %v", err)
	}
	defer resp.Body.Close()

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/messages", nil)

	info := &relaycommon.RelayInfo{
		RelayFormat:       relayconstant.RelayFormatClaude,
		OriginModelName:   "claude-test",
		ResponseModelName: "claude-test",
		ChannelMeta:       &relaycommon.ChannelMeta{UpstreamModelName: "claude-test"},
	}

	usage, apiErr := ClaudeStreamHandler(c, resp, info)
	if apiErr == nil {
		t.Fatalf("expected timeout error, got nil (usage: %+v) — billing would fabricate empty-usage 502", usage)
	}
	if apiErr.StatusCode != http.StatusGatewayTimeout {
		t.Errorf("statusCode = %d, want 504", apiErr.StatusCode)
	}
	if usage != nil {
		t.Errorf("usage should be nil on stream timeout, got %+v", usage)
	}
}

// 已收到部分数据后超时：保持既有兜底行为，补完流结束并返回已累计的
// usage（按部分内容估算），不因超时直接报错。
func TestStreamTimeoutAfterPartialDataKeepsFallbackBehavior(t *testing.T) {
	originalTimeout := shared.StreamingTimeout
	shared.StreamingTimeout = 2
	defer func() { shared.StreamingTimeout = originalTimeout }()

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		w.(http.Flusher).Flush()
		_, _ = w.Write([]byte("data: {\"type\": \"message_start\", \"message\": {\"id\": \"msg_1\", \"model\": \"claude-test\", \"usage\": {\"input_tokens\": 25, \"output_tokens\": 1}}}\n\n"))
		w.(http.Flusher).Flush()
		<-r.Context().Done()
	}))
	defer upstream.Close()

	req, _ := http.NewRequest(http.MethodPost, upstream.URL, nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("do request: %v", err)
	}
	defer resp.Body.Close()

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/messages", nil)

	info := &relaycommon.RelayInfo{
		RelayFormat:       relayconstant.RelayFormatClaude,
		OriginModelName:   "claude-test",
		ResponseModelName: "claude-test",
		ChannelMeta:       &relaycommon.ChannelMeta{UpstreamModelName: "claude-test"},
	}

	start := time.Now()
	usage, apiErr := ClaudeStreamHandler(c, resp, info)
	if apiErr != nil {
		t.Fatalf("partial-data timeout should keep fallback behavior, got error: %v", apiErr)
	}
	if elapsed := time.Since(start); elapsed < 2*time.Second {
		t.Errorf("expected to wait for streaming timeout, elapsed %v", elapsed)
	}
	if usage == nil || usage.PromptTokens == 0 {
		t.Fatalf("usage from message_start should be preserved, got %+v", usage)
	}
}
