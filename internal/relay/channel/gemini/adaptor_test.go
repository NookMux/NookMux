package gemini

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/NookMux/NookMux/internal/common"
	"github.com/NookMux/NookMux/internal/domain/shared"
	relaycommon "github.com/NookMux/NookMux/internal/relay/common"
	relayconstant "github.com/NookMux/NookMux/internal/relay/constant"
	"github.com/gin-gonic/gin"
)

const geminiTestBase = "https://generativelanguage.googleapis.com"

func newCompatInfo(format relayconstant.RelayFormat, model string) *relaycommon.RelayInfo {
	return &relaycommon.RelayInfo{
		RelayFormat: format,
		RelayMode:   relayconstant.RelayModeChatCompletions,
		ChannelMeta: &relaycommon.ChannelMeta{
			ChannelBaseUrl:    geminiTestBase,
			UpstreamModelName: model,
		},
	}
}

// TestAdaptorGetRequestURL_CountTokensAction verifies the Gemini-native
// countTokens action maps to the upstream :countTokens endpoint.
func TestAdaptorGetRequestURL_CountTokensAction(t *testing.T) {
	adaptor := &Adaptor{}
	info := &relaycommon.RelayInfo{
		ChannelMeta: &relaycommon.ChannelMeta{
			ChannelBaseUrl:    geminiTestBase,
			UpstreamModelName: "gemini-2.5-pro",
		},
		RelayMode:      relayconstant.RelayModeGemini,
		RelayFormat:    relayconstant.RelayFormatGemini,
		RequestURLPath: "/v1beta/models/gemini-2.5-pro:countTokens",
	}

	got, err := adaptor.GetRequestURL(info)
	if err != nil {
		t.Fatalf("GetRequestURL error = %v", err)
	}
	want := geminiTestBase + "/v1beta/models/gemini-2.5-pro:countTokens"
	if got != want {
		t.Fatalf("countTokens URL = %q, want %q", got, want)
	}
}

// TestAdaptorGetRequestURL_ChatCompatPath 校验 OpenAI/Claude 入站 chat 直通官方
// OpenAI 兼容层路径，流式与非流式同 URL（标准 SSE，不加 ?alt=sse）。
func TestAdaptorGetRequestURL_ChatCompatPath(t *testing.T) {
	adaptor := &Adaptor{}
	want := geminiTestBase + "/v1beta/openai/chat/completions"

	for _, format := range []relayconstant.RelayFormat{
		relayconstant.RelayFormatOpenAI,
		relayconstant.RelayFormatClaude,
	} {
		info := newCompatInfo(format, "gemini-2.5-pro")
		got, err := adaptor.GetRequestURL(info)
		if err != nil {
			t.Fatalf("GetRequestURL(%s) error = %v", format, err)
		}
		if got != want {
			t.Fatalf("GetRequestURL(%s) = %q, want %q", format, got, want)
		}
		if strings.Contains(got, "alt=sse") {
			t.Fatalf("compat URL must not append ?alt=sse, got: %s", got)
		}

		// 流式也走同一 URL。
		info.IsStream = true
		got, err = adaptor.GetRequestURL(info)
		if err != nil {
			t.Fatalf("GetRequestURL(%s, stream) error = %v", format, err)
		}
		if got != want {
			t.Fatalf("GetRequestURL(%s, stream) = %q, want %q", format, got, want)
		}
	}
}

// TestAdaptorGetRequestURL_NativePathsUnchanged 校验原生 Gemini 入站各端点路径
// 不受兼容层适配影响。
func TestAdaptorGetRequestURL_NativePathsUnchanged(t *testing.T) {
	adaptor := &Adaptor{}
	nativeInfo := func(model string, stream bool) *relaycommon.RelayInfo {
		return &relaycommon.RelayInfo{
			RelayMode:   relayconstant.RelayModeGemini,
			RelayFormat: relayconstant.RelayFormatGemini,
			IsStream:    stream,
			ChannelMeta: &relaycommon.ChannelMeta{
				ChannelBaseUrl:    geminiTestBase,
				UpstreamModelName: model,
			},
		}
	}

	cases := []struct {
		name   string
		model  string
		stream bool
		want   string
	}{
		{"generateContent", "gemini-2.5-pro", false, geminiTestBase + "/v1beta/models/gemini-2.5-pro:generateContent"},
		{"streamGenerateContent", "gemini-2.5-pro", true, geminiTestBase + "/v1beta/models/gemini-2.5-pro:streamGenerateContent?alt=sse"},
		{"embedContent", "text-embedding-004", false, geminiTestBase + "/v1beta/models/text-embedding-004:embedContent"},
		{"imagen predict", "imagen-3.0", false, geminiTestBase + "/v1beta/models/imagen-3.0:predict"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := adaptor.GetRequestURL(nativeInfo(tc.model, tc.stream))
			if err != nil {
				t.Fatalf("GetRequestURL error = %v", err)
			}
			if got != tc.want {
				t.Fatalf("GetRequestURL(%s) = %q, want %q", tc.name, got, tc.want)
			}
		})
	}
}

// TestAdaptorSetupRequestHeader_AuthByFormat 校验兼容层用 Bearer、原生用 x-goog-api-key。
func TestAdaptorSetupRequestHeader_AuthByFormat(t *testing.T) {
	gin.SetMode(gin.TestMode)
	adaptor := &Adaptor{}
	const key = "test-key"

	for _, tc := range []struct {
		name   string
		format relayconstant.RelayFormat
		want   string
	}{
		{"OpenAI compat", relayconstant.RelayFormatOpenAI, "Bearer test-key"},
		{"Claude compat", relayconstant.RelayFormatClaude, "Bearer test-key"},
		{"Gemini native", relayconstant.RelayFormatGemini, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
			info := &relaycommon.RelayInfo{
				RelayFormat: tc.format,
				ChannelMeta: &relaycommon.ChannelMeta{
					ChannelBaseUrl:    geminiTestBase,
					UpstreamModelName: "gemini-2.5-pro",
					ApiKey:            key,
				},
			}
			req := http.Header{}
			if err := adaptor.SetupRequestHeader(c, &req, info); err != nil {
				t.Fatalf("SetupRequestHeader error = %v", err)
			}
			if tc.want != "" {
				if got := req.Get("Authorization"); got != tc.want {
					t.Fatalf("Authorization = %q, want %q", got, tc.want)
				}
				if v := req.Get("x-goog-api-key"); v != "" {
					t.Fatalf("x-goog-api-key should be empty for compat layer, got %q", v)
				}
			} else {
				if got := req.Get("x-goog-api-key"); got != key {
					t.Fatalf("x-goog-api-key = %q, want %q", got, key)
				}
				if v := req.Get("Authorization"); v != "" {
					t.Fatalf("Authorization should be empty for native path, got %q", v)
				}
			}
		})
	}
}

// TestAdaptorConvertOpenAIRequest_Identity 校验 OpenAI 入站请求恒等透传，仅流式下补
// stream_options.include_usage。
func TestAdaptorConvertOpenAIRequest_Identity(t *testing.T) {
	adaptor := &Adaptor{}

	// 非流式：原样返回，不注入 stream_options。
	req := &shared.GeneralOpenAIRequest{Model: "gpt-4o", Stream: false}
	info := newCompatInfo(relayconstant.RelayFormatOpenAI, "gemini-2.5-pro")
	got, err := adaptor.ConvertOpenAIRequest(nil, info, req)
	if err != nil {
		t.Fatalf("ConvertOpenAIRequest error = %v", err)
	}
	out, ok := got.(*shared.GeneralOpenAIRequest)
	if !ok {
		t.Fatalf("converted type = %T, want *shared.GeneralOpenAIRequest", got)
	}
	if out.StreamOptions != nil {
		t.Fatalf("non-stream should not inject stream_options, got %+v", out.StreamOptions)
	}

	// 流式 + SupportStreamOptions：注入 include_usage=true。
	info.IsStream = true
	info.SupportStreamOptions = true
	_, err = adaptor.ConvertOpenAIRequest(nil, info, req)
	if err != nil {
		t.Fatalf("ConvertOpenAIRequest(stream) error = %v", err)
	}
	if out.StreamOptions == nil || !out.StreamOptions.IncludeUsage {
		t.Fatalf("stream should inject stream_options.include_usage=true, got %+v", out.StreamOptions)
	}
}

// TestAdaptorConvertClaudeRequestDelegatesToOpenAI 校验 Claude 入站经
// ClaudeToOpenAIRequest 转为 OpenAI 请求透传（thinking 映射为 ReasoningEffort）。
func TestAdaptorConvertClaudeRequestDelegatesToOpenAI(t *testing.T) {
	budget := 4096
	request := &shared.ClaudeRequest{
		Model: "claude-3-7-sonnet",
		Thinking: &shared.Thinking{
			Type:         "enabled",
			BudgetTokens: &budget,
		},
		Messages: []shared.ClaudeMessage{
			{Role: "user", Content: []shared.ClaudeMediaMessage{{Type: "text", Text: common.GetPointer[string]("hi")}}},
		},
	}
	request.AddTool(&shared.Tool{Name: "weather", InputSchema: map[string]interface{}{"type": "object"}})

	adaptor := &Adaptor{}
	info := newCompatInfo(relayconstant.RelayFormatClaude, "gemini-2.5-pro")
	got, err := adaptor.ConvertClaudeRequest(nil, info, request)
	if err != nil {
		t.Fatalf("ConvertClaudeRequest error = %v", err)
	}
	out, ok := got.(*shared.GeneralOpenAIRequest)
	if !ok {
		t.Fatalf("converted type = %T, want *shared.GeneralOpenAIRequest (passthrough)", got)
	}
	if out.ReasoningEffort == "" {
		t.Fatal("Claude thinking enabled should map to non-empty ReasoningEffort")
	}
	if len(out.Tools) != 1 {
		t.Fatalf("tools len = %d, want 1 (preserved through conversion)", len(out.Tools))
	}
}

// TestAdaptorDoResponse_DelegatesToOpenAIForCompat 校验 OpenAI 入站 chat 响应委托
// openai adaptor 处理（非流式），usage 正常提取。
func TestAdaptorDoResponse_DelegatesToOpenAIForCompat(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)

	info := &relaycommon.RelayInfo{
		RelayFormat:       relayconstant.RelayFormatOpenAI,
		RelayMode:         relayconstant.RelayModeChatCompletions,
		OriginModelName:   "alias-model",
		ResponseModelName: "alias-model",
		ChannelMeta: &relaycommon.ChannelMeta{
			ChannelBaseUrl:    geminiTestBase,
			UpstreamModelName: "gemini-2.5-pro",
			ChannelType:       24,
		},
	}

	body := `{"id":"chatcmpl-1","object":"chat.completion","model":"alias-model","choices":[{"index":0,"message":{"role":"assistant","content":"hi"},"finish_reason":"stop"}],"usage":{"prompt_tokens":5,"completion_tokens":2,"total_tokens":7}}`
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(strings.NewReader(body)),
	}

	adaptor := &Adaptor{}
	usage, apiErr := adaptor.DoResponse(c, resp, info)
	if apiErr != nil {
		t.Fatalf("DoResponse error = %v", apiErr)
	}
	u, ok := usage.(*shared.Usage)
	if !ok {
		t.Fatalf("usage type = %T, want *shared.Usage", usage)
	}
	if u.TotalTokens != 7 {
		t.Fatalf("TotalTokens = %d, want 7", u.TotalTokens)
	}
	if !strings.Contains(w.Body.String(), "alias-model") {
		t.Fatalf("response body should contain alias model, got: %s", w.Body.String())
	}
}

// TestAdaptorDoResponse_UnsupportedFallback 校验未覆盖的入站格式/模型落到错误分支
// 而非旧 chat 转换 handler（已删除）。
func TestAdaptorDoResponse_UnsupportedFallback(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/audio/speech", nil)

	info := &relaycommon.RelayInfo{
		RelayFormat: relayconstant.RelayFormatOpenAIAudio,
		ChannelMeta: &relaycommon.ChannelMeta{
			ChannelBaseUrl:    geminiTestBase,
			UpstreamModelName: "tts-1",
		},
	}
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(strings.NewReader(`{}`)),
	}

	adaptor := &Adaptor{}
	_, apiErr := adaptor.DoResponse(c, resp, info)
	if apiErr == nil {
		t.Fatal("expected error for unsupported relay format on gemini channel, got nil")
	}
}

// TestConvertToolChoiceToGeminiConfigRequired 确保 channel-test 用 string "required" 作为
// tool_choice 时,Gemini adaptor 能正确转换为 mode:"ANY",保证唯一工具被强制调用。
// （该转换器由 Gemini 渠道原生路径与 Vertex 渠道共享复用。）
func TestConvertToolChoiceToGeminiConfigRequired(t *testing.T) {
	config := convertToolChoiceToGeminiConfig("required")
	if config == nil || config.FunctionCallingConfig == nil {
		t.Fatalf("expected non-nil tool config for required choice, got %+v", config)
	}
	if mode := string(config.FunctionCallingConfig.Mode); mode != "ANY" {
		t.Fatalf("expected mode \"ANY\" for required tool_choice, got %q", mode)
	}
	if len(config.FunctionCallingConfig.AllowedFunctionNames) != 0 {
		t.Fatalf("expected no allowed_function_names restriction for string required, got %+v",
			config.FunctionCallingConfig.AllowedFunctionNames)
	}
}
