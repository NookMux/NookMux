package aws

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/NookMux/NookMux/internal/domain/shared"
	relaychannel "github.com/NookMux/NookMux/internal/relay/channel"
	relaycommon "github.com/NookMux/NookMux/internal/relay/common"
	"github.com/NookMux/NookMux/pkg/jsonx"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/bedrockruntime"
	"github.com/gin-gonic/gin"
)

func newAwsHeaderTestContext(headers http.Header) *gin.Context {
	gin.SetMode(gin.TestMode)
	request := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewBufferString(`{
		"messages":[{"role":"user","content":"hello"}],
		"max_tokens":8
	}`))
	request.Header = headers
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	ctx.Request = request
	return ctx
}

func newAwsHeaderTestInfo(overrides map[string]interface{}, passThrough bool) *relaycommon.RelayInfo {
	return &relaycommon.RelayInfo{
		ChannelMeta: &relaycommon.ChannelMeta{
			ApiKey:            "test-token|us-east-1",
			UpstreamModelName: "claude-opus-4-8",
			HeadersOverride:   overrides,
			ChannelSetting: shared.ChannelSettings{
				PassThroughHeadersEnabled: passThrough,
			},
			ChannelOtherSettings: shared.ChannelOtherSettings{AwsKeyType: shared.AwsKeyTypeApiKey},
		},
	}
}

func awsRequestBodyMap(t *testing.T, adaptor *Adaptor) map[string]any {
	t.Helper()
	request, ok := adaptor.AwsReq.(*bedrockruntime.InvokeModelInput)
	if !ok {
		t.Fatalf("AWS request type = %T, want *bedrockruntime.InvokeModelInput", adaptor.AwsReq)
	}
	var payload map[string]any
	if err := jsonx.Unmarshal(request.Body, &payload); err != nil {
		t.Fatalf("unmarshal AWS request body: %v", err)
	}
	return payload
}

func TestDoAwsClientRequestPreservesHeaderOverrideForClaudeBeta(t *testing.T) {
	ctx := newAwsHeaderTestContext(http.Header{})
	info := newAwsHeaderTestInfo(map[string]interface{}{"anthropic-beta": "computer-use-2025-01-24"}, false)
	adaptor := &Adaptor{}

	if _, err := doAwsClientRequest(ctx, info, adaptor, bytes.NewBufferString(`{
		"messages":[{"role":"user","content":"hello"}],
		"max_tokens":8
	}`)); err != nil {
		t.Fatalf("doAwsClientRequest: %v", err)
	}

	payload := awsRequestBodyMap(t, adaptor)
	betas, ok := payload["anthropic_beta"].([]any)
	if !ok || len(betas) != 1 || betas[0] != "computer-use-2025-01-24" {
		t.Fatalf("anthropic_beta = %#v, want explicit override", payload["anthropic_beta"])
	}
}

func TestDoAwsClientRequestPassesSafeHeadersAndFiltersBillingHeader(t *testing.T) {
	ctx := newAwsHeaderTestContext(http.Header{
		"Anthropic-Beta":             []string{"computer-use-2025-01-24"},
		"X-Anthropic-Billing-Header": []string{"client-billing"},
		"X-Trace-Id":                 []string{"trace-123"},
	})
	info := newAwsHeaderTestInfo(nil, true)
	adaptor := &Adaptor{}

	if _, err := doAwsClientRequest(ctx, info, adaptor, bytes.NewBufferString(`{
		"messages":[{"role":"user","content":"hello"}],
		"max_tokens":8
	}`)); err != nil {
		t.Fatalf("doAwsClientRequest: %v", err)
	}

	payload := awsRequestBodyMap(t, adaptor)
	betas, ok := payload["anthropic_beta"].([]any)
	if !ok || len(betas) != 1 || betas[0] != "computer-use-2025-01-24" {
		t.Fatalf("anthropic_beta = %#v, want passthrough value", payload["anthropic_beta"])
	}
	if _, ok := payload["x-trace-id"]; ok {
		t.Fatalf("ordinary transport headers must not be copied into Claude body: %#v", payload)
	}
	if _, ok := payload["x-anthropic-billing-header"]; ok {
		t.Fatalf("billing header must not be copied into Claude body: %#v", payload)
	}

	// Keep this assertion coupled to the shared filter so the AWS path cannot
	// accidentally bypass it when the request construction is refactored again.
	filtered := http.Header{}
	relaychannel.MergeClientHeadersToHeader(ctx, filtered)
	if filtered.Get("x-anthropic-billing-header") != "" || filtered.Get("x-trace-id") != "trace-123" {
		t.Fatalf("safe header merge = %#v", filtered)
	}
}

type novaStubRoundTripper struct {
	statusCode int
	body       []byte
}

func (t *novaStubRoundTripper) RoundTrip(*http.Request) (*http.Response, error) {
	recorder := httptest.NewRecorder()
	recorder.Code = t.statusCode
	recorder.Header().Set("Content-Type", "application/json")
	recorder.Body.Write(t.body)
	return recorder.Result(), nil
}

func newNovaTestAdaptor(t *testing.T, responseBody []byte) *Adaptor {
	t.Helper()
	credential, err := parseAwsCredential("test-api-key|us-east-1", shared.AwsKeyTypeApiKey)
	if err != nil {
		t.Fatalf("parseAwsCredential: %v", err)
	}
	httpClient := &http.Client{Transport: &novaStubRoundTripper{statusCode: http.StatusOK, body: responseBody}}
	return &Adaptor{
		AwsClient: newAwsRuntimeClient(credential, httpClient),
		AwsReq: &bedrockruntime.InvokeModelInput{
			ModelId:     aws.String("us.amazon.nova-pro-v1:0"),
			Accept:      aws.String("application/json"),
			ContentType: aws.String("application/json"),
			Body:        []byte(`{"messages":[{"role":"user","content":[{"text":"hi"}]}]}`),
		},
	}
}

func newNovaTestContext() (*gin.Context, *httptest.ResponseRecorder) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	return ctx, recorder
}

func TestHandleNovaRequestEmptyContentReturnsBadResponseError(t *testing.T) {
	cases := []struct {
		name string
		body string
	}{
		{name: "empty content array", body: `{"output":{"message":{"content":[]}},"usage":{"inputTokens":3,"outputTokens":5,"totalTokens":8}}`},
		{name: "missing message", body: `{"output":{},"usage":{"inputTokens":3,"outputTokens":5,"totalTokens":8}}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ctx, _ := newNovaTestContext()
			info := newAwsHeaderTestInfo(nil, false)
			adaptor := newNovaTestAdaptor(t, []byte(tc.body))

			err, usage := handleNovaRequest(ctx, info, adaptor)

			if err == nil {
				t.Fatal("handleNovaRequest() error = nil, want bad response error for empty content")
			}
			if usage != nil {
				t.Fatalf("usage = %#v, want nil when handler fails", usage)
			}
			if err.GetErrorCode() != shared.ErrorCodeBadResponseBody {
				t.Fatalf("error code = %q, want %q", err.GetErrorCode(), shared.ErrorCodeBadResponseBody)
			}
		})
	}
}

func TestHandleNovaRequestSingleContentSucceeds(t *testing.T) {
	ctx, recorder := newNovaTestContext()
	info := newAwsHeaderTestInfo(nil, false)
	adaptor := newNovaTestAdaptor(t, []byte(`{"output":{"message":{"content":[{"text":"hello nova"}]}},"usage":{"inputTokens":3,"outputTokens":5,"totalTokens":8}}`))

	err, usage := handleNovaRequest(ctx, info, adaptor)

	if err != nil {
		t.Fatalf("handleNovaRequest: %v", err)
	}
	if usage == nil || usage.PromptTokens != 3 || usage.CompletionTokens != 5 || usage.TotalTokens != 8 {
		t.Fatalf("usage = %#v, want prompt 3 completion 5 total 8", usage)
	}
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusOK)
	}
	var response shared.OpenAITextResponse
	if err := jsonx.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("unmarshal response body: %v", err)
	}
	if len(response.Choices) != 1 || response.Choices[0].Message.Content != "hello nova" {
		t.Fatalf("choices = %#v, want single choice with content %q", response.Choices, "hello nova")
	}
}
