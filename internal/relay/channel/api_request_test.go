package channel

import (
	"net/http"
	"net/http/httptest"
	"testing"

	modelconfig "github.com/NookMux/NookMux/internal/config/model"
	relaycommon "github.com/NookMux/NookMux/internal/relay/common"
	"github.com/gin-gonic/gin"
)

func TestProcessHeaderOverrideSkipsClaudeCodeBillingHeaderPassthrough(t *testing.T) {
	settings := modelconfig.GetClaudeSettings()
	original := settings.RemoveClaudeCodeBillingHeaderEnabled
	settings.RemoveClaudeCodeBillingHeaderEnabled = true
	t.Cleanup(func() { settings.RemoveClaudeCodeBillingHeaderEnabled = original })

	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/messages", nil)
	ctx.Request.Header.Set("X-Trace-Id", "trace-123")
	ctx.Request.Header.Set(modelconfig.ClaudeCodeBillingHeader, "cache-buster")

	info := &relaycommon.RelayInfo{ChannelMeta: &relaycommon.ChannelMeta{HeadersOverride: map[string]any{"*": ""}}}
	headers, err := processHeaderOverride(info, ctx)
	if err != nil {
		t.Fatal(err)
	}
	if headers["X-Trace-Id"] != "trace-123" {
		t.Fatalf("trace header = %q", headers["X-Trace-Id"])
	}
	for key := range headers {
		if modelconfig.ShouldRemoveClaudeCodeBillingHeader(key) {
			t.Fatalf("billing header passed through as %q", key)
		}
	}
}

func TestProcessHeaderOverrideAllowsExplicitClaudeCodeBillingHeaderOverride(t *testing.T) {
	settings := modelconfig.GetClaudeSettings()
	original := settings.RemoveClaudeCodeBillingHeaderEnabled
	settings.RemoveClaudeCodeBillingHeaderEnabled = true
	t.Cleanup(func() { settings.RemoveClaudeCodeBillingHeaderEnabled = original })

	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/messages", nil)
	info := &relaycommon.RelayInfo{ChannelMeta: &relaycommon.ChannelMeta{HeadersOverride: map[string]any{
		"*":                                 "",
		modelconfig.ClaudeCodeBillingHeader: "explicit-billing",
	}}}

	headers, err := processHeaderOverride(info, ctx)
	if err != nil {
		t.Fatal(err)
	}
	if headers[modelconfig.ClaudeCodeBillingHeader] != "explicit-billing" {
		t.Fatalf("explicit billing override = %q", headers[modelconfig.ClaudeCodeBillingHeader])
	}
}
