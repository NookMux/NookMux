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
