package common

import (
	"testing"

	modelconfig "github.com/NookMux/NookMux/internal/config/model"
)

func TestApplyParamOverridePassHeadersSkipsClaudeCodeBillingHeader(t *testing.T) {
	settings := modelconfig.GetClaudeSettings()
	original := settings.RemoveClaudeCodeBillingHeaderEnabled
	settings.RemoveClaudeCodeBillingHeaderEnabled = true
	t.Cleanup(func() { settings.RemoveClaudeCodeBillingHeaderEnabled = original })

	input := []byte(`{"model":"claude-opus-4-8"}`)
	override := map[string]interface{}{"operations": []interface{}{
		map[string]interface{}{"mode": "pass_headers", "value": []interface{}{modelconfig.ClaudeCodeBillingHeader, "x-trace-id"}},
	}}
	context := map[string]interface{}{
		"request_headers": map[string]interface{}{
			modelconfig.ClaudeCodeBillingHeader: "client-billing",
			"x-trace-id":                        "trace-123",
		},
	}

	if _, err := ApplyParamOverride(input, override, context); err != nil {
		t.Fatal(err)
	}
	headers, ok := context["header_override"].(map[string]interface{})
	if !ok {
		t.Fatalf("header override context = %#v", context["header_override"])
	}
	if _, exists := headers[modelconfig.ClaudeCodeBillingHeader]; exists {
		t.Fatal("billing header was copied by pass_headers")
	}
	if headers["x-trace-id"] != "trace-123" {
		t.Fatalf("trace header = %q", headers["x-trace-id"])
	}
}
