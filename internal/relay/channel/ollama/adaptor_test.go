package ollama

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	channelconstant "github.com/NookMux/NookMux/internal/domain/channel/constant"
	"github.com/NookMux/NookMux/internal/domain/shared"
	"github.com/NookMux/NookMux/internal/i18n"
	relaycommon "github.com/NookMux/NookMux/internal/relay/common"

	relayconstant "github.com/NookMux/NookMux/internal/relay/constant"
	"github.com/gin-gonic/gin"
)

func TestGetRequestURLUsesCompatiblePath(t *testing.T) {
	tests := []struct {
		name           string
		channelBaseURL string
		requestPath    string
		relayFormat    relayconstant.RelayFormat
		expectURL      string
	}{
		{
			name:           "chat completions",
			channelBaseURL: "http://ollama.local",
			requestPath:    "/v1/chat/completions",
			relayFormat:    relayconstant.RelayFormatOpenAI,
			expectURL:      "http://ollama.local/v1/chat/completions",
		},
		{
			name:           "anthropic messages with query",
			channelBaseURL: "http://ollama.local",
			requestPath:    "/v1/messages?beta=true",
			relayFormat:    relayconstant.RelayFormatClaude,
			expectURL:      "http://ollama.local/v1/messages?beta=true",
		},
		{
			name:           "coding plan openai chat",
			channelBaseURL: "ollama-coding-plan",
			requestPath:    "/v1/chat/completions",
			relayFormat:    relayconstant.RelayFormatOpenAI,
			expectURL:      "https://ollama.com/v1/chat/completions",
		},
		{
			name:           "coding plan claude messages",
			channelBaseURL: "ollama-coding-plan",
			requestPath:    "/v1/messages",
			relayFormat:    relayconstant.RelayFormatClaude,
			expectURL:      "https://ollama.com/v1/messages",
		},
		{
			name:           "coding plan openai embeddings",
			channelBaseURL: "ollama-coding-plan",
			requestPath:    "/v1/embeddings",
			relayFormat:    relayconstant.RelayFormatOpenAI,
			expectURL:      "https://ollama.com/v1/embeddings",
		},
	}

	adaptor := &Adaptor{}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			info := &relaycommon.RelayInfo{
				ChannelMeta:    &relaycommon.ChannelMeta{ChannelBaseUrl: tt.channelBaseURL},
				RequestURLPath: tt.requestPath,
				RelayFormat:    tt.relayFormat,
			}

			got, err := adaptor.GetRequestURL(info)
			if err != nil {
				t.Fatalf("GetRequestURL returned error: %v", err)
			}
			if got != tt.expectURL {
				t.Fatalf("GetRequestURL = %q, want %q", got, tt.expectURL)
			}
		})
	}
}

func TestSetupRequestHeaderOpenAI(t *testing.T) {
	gin.SetMode(gin.TestMode)
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	ctx.Request.Header.Set("Content-Type", "application/json")

	info := &relaycommon.RelayInfo{
		RelayFormat: relayconstant.RelayFormatOpenAI,
		IsStream:    true,
		ChannelMeta: &relaycommon.ChannelMeta{ApiKey: "secret-key"},
	}

	headers := http.Header{}
	adaptor := &Adaptor{}
	if err := adaptor.SetupRequestHeader(ctx, &headers, info); err != nil {
		t.Fatalf("SetupRequestHeader returned error: %v", err)
	}

	if got := headers.Get("Authorization"); got != "Bearer secret-key" {
		t.Fatalf("Authorization = %q, want %q", got, "Bearer secret-key")
	}
	if got := headers.Get("Accept"); got != "text/event-stream" {
		t.Fatalf("Accept = %q, want %q", got, "text/event-stream")
	}
}

func TestSetupRequestHeaderClaude(t *testing.T) {
	gin.SetMode(gin.TestMode)
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/messages", nil)
	ctx.Request.Header.Set("Content-Type", "application/json")
	ctx.Request.Header.Set("anthropic-beta", "tools-2025-01-01")

	info := &relaycommon.RelayInfo{
		RelayFormat: relayconstant.RelayFormatClaude,
		ChannelMeta: &relaycommon.ChannelMeta{ApiKey: "secret-key"},
	}

	headers := http.Header{}
	adaptor := &Adaptor{}
	if err := adaptor.SetupRequestHeader(ctx, &headers, info); err != nil {
		t.Fatalf("SetupRequestHeader returned error: %v", err)
	}

	if got := headers.Get("x-api-key"); got != "secret-key" {
		t.Fatalf("x-api-key = %q, want %q", got, "secret-key")
	}
	if got := headers.Get("anthropic-version"); got != "2023-06-01" {
		t.Fatalf("anthropic-version = %q, want %q", got, "2023-06-01")
	}
	if got := headers.Get("anthropic-beta"); got != "tools-2025-01-01" {
		t.Fatalf("anthropic-beta = %q, want %q", got, "tools-2025-01-01")
	}
	if got := headers.Get("Authorization"); got != "" {
		t.Fatalf("Authorization = %q, want empty", got)
	}
}

func TestConvertOpenAIResponsesRequestPassThrough(t *testing.T) {
	adaptor := &Adaptor{}
	request := shared.OpenAIResponsesRequest{Model: "qwen3:8b", Stream: true}

	converted, err := adaptor.ConvertOpenAIResponsesRequest(nil, nil, request)
	if err != nil {
		t.Fatalf("ConvertOpenAIResponsesRequest returned error: %v", err)
	}

	convertedRequest, ok := converted.(shared.OpenAIResponsesRequest)
	if !ok {
		t.Fatalf("converted request type = %T, want shared.OpenAIResponsesRequest", converted)
	}
	if convertedRequest.Model != request.Model || convertedRequest.Stream != request.Stream {
		t.Fatalf("converted request = %+v, want %+v", convertedRequest, request)
	}
}

func TestFetchOllamaModelsUsesOpenAICompatibleEndpoint(t *testing.T) {
	var requestPath string
	var authHeader string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestPath = r.URL.Path
		authHeader = r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":[{"id":"qwen3:8b","created":1710000000,"owned_by":"library"}]}`))
	}))
	defer server.Close()

	models, err := FetchOllamaModels(i18n.DefaultLang, server.URL, "ollama", "")
	if err != nil {
		t.Fatalf("FetchOllamaModels returned error: %v", err)
	}
	if len(models) != 1 {
		t.Fatalf("model count = %d, want 1", len(models))
	}
	if requestPath != "/v1/models" {
		t.Fatalf("request path = %q, want %q", requestPath, "/v1/models")
	}
	if authHeader != "Bearer ollama" {
		t.Fatalf("Authorization = %q, want %q", authHeader, "Bearer ollama")
	}
	if models[0].Name != "qwen3:8b" {
		t.Fatalf("model name = %q, want %q", models[0].Name, "qwen3:8b")
	}
	if models[0].OwnedBy != "library" {
		t.Fatalf("owned_by = %q, want %q", models[0].OwnedBy, "library")
	}
	if models[0].Created != 1710000000 {
		t.Fatalf("created = %d, want %d", models[0].Created, int64(1710000000))
	}
	if _, err := time.Parse(time.RFC3339, models[0].ModifiedAt); err != nil {
		t.Fatalf("modified_at = %q is not RFC3339: %v", models[0].ModifiedAt, err)
	}
}

func TestResolveBaseURL(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "coding plan resolves to real URL",
			input:    "ollama-coding-plan",
			expected: "https://ollama.com",
		},
		{
			name:     "normal URL passes through",
			input:    "http://localhost:11434",
			expected: "http://localhost:11434",
		},
		{
			name:     "https URL passes through",
			input:    "https://ollama.example.com",
			expected: "https://ollama.example.com",
		},
		{
			name:     "empty string passes through",
			input:    "",
			expected: "",
		},
		{
			name:     "whitespace is trimmed before lookup",
			input:    " ollama-coding-plan ",
			expected: "https://ollama.com",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := resolveBaseURL(tt.input)
			if got != tt.expected {
				t.Fatalf("resolveBaseURL(%q) = %q, want %q", tt.input, got, tt.expected)
			}
		})
	}
}

func TestFetchOllamaModelsResolvesPlanBaseURL(t *testing.T) {
	var requestHost string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestHost = r.Host
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":[{"id":"llama3","created":1710000000}]}`))
	}))
	defer server.Close()

	// Temporarily override the plan mapping to point to our test server.
	origURL := channelconstant.ChannelSpecialBases["ollama-coding-plan"]
	channelconstant.ChannelSpecialBases["ollama-coding-plan"] = channelconstant.ChannelSpecialBase{
		OpenAIBaseURL: server.URL,
	}
	defer func() { channelconstant.ChannelSpecialBases["ollama-coding-plan"] = origURL }()

	models, err := FetchOllamaModels(i18n.DefaultLang, "ollama-coding-plan", "test-key", "")
	if err != nil {
		t.Fatalf("FetchOllamaModels returned error: %v", err)
	}
	if len(models) != 1 || models[0].Name != "llama3" {
		t.Fatalf("models = %+v, want one model named llama3", models)
	}
	// Verify the request actually went to the test server, not to "ollama-coding-plan".
	if requestHost == "" {
		t.Fatal("requestHost is empty — request may not have reached the test server")
	}
}

func TestFetchOllamaVersionResolvesPlanBaseURL(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/version" {
			t.Fatalf("request path = %q, want /api/version", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"version":"0.6.5"}`))
	}))
	defer server.Close()

	origURL := channelconstant.ChannelSpecialBases["ollama-coding-plan"]
	channelconstant.ChannelSpecialBases["ollama-coding-plan"] = channelconstant.ChannelSpecialBase{
		OpenAIBaseURL: server.URL,
	}
	defer func() { channelconstant.ChannelSpecialBases["ollama-coding-plan"] = origURL }()

	version, err := FetchOllamaVersion(i18n.DefaultLang, "ollama-coding-plan", "test-key", "")
	if err != nil {
		t.Fatalf("FetchOllamaVersion returned error: %v", err)
	}
	if version != "0.6.5" {
		t.Fatalf("version = %q, want %q", version, "0.6.5")
	}
}

// TestFetchOllamaModelsRoutesThroughProxy 验证配置 proxyURL 后请求经代理转发。
func TestFetchOllamaModelsRoutesThroughProxy(t *testing.T) {
	// 目标上游：普通 httptest 服务，不应被直连访问到。
	upstreamRequests := 0
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upstreamRequests++
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":[{"id":"direct"}]}`))
	}))
	defer upstream.Close()

	// 伪 HTTP 代理：收到绝对形态请求后转发给上游，并记录转发次数。
	proxiedRequests := 0
	proxyServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		proxiedRequests++
		outReq, err := http.NewRequestWithContext(r.Context(), http.MethodGet, r.URL.String(), nil)
		if err != nil {
			t.Errorf("proxy failed to build request: %v", err)
			w.WriteHeader(http.StatusBadGateway)
			return
		}
		resp, err := http.DefaultClient.Do(outReq)
		if err != nil {
			t.Errorf("proxy upstream request failed: %v", err)
			w.WriteHeader(http.StatusBadGateway)
			return
		}
		defer resp.Body.Close()
		w.WriteHeader(resp.StatusCode)
		_, _ = io.Copy(w, resp.Body)
	}))
	defer proxyServer.Close()

	models, err := FetchOllamaModels(i18n.DefaultLang, upstream.URL, "test-key", proxyServer.URL)
	if err != nil {
		t.Fatalf("FetchOllamaModels returned error: %v", err)
	}
	if len(models) != 1 || models[0].Name != "direct" {
		t.Fatalf("models = %+v, want one model named direct", models)
	}
	if proxiedRequests != 1 {
		t.Fatalf("proxied request count = %d, want 1", proxiedRequests)
	}
	if upstreamRequests != 1 {
		t.Fatalf("upstream request count = %d, want 1 (reached via proxy)", upstreamRequests)
	}
}
