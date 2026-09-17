package model

import (
	"net/http"
	"testing"
)

func TestClaudeSettingsWriteHeadersRemovesBillingHeaderWhenEnabled(t *testing.T) {
	settings := &ClaudeSettings{
		RemoveClaudeCodeBillingHeaderEnabled: true,
		HeadersSettings: map[string]map[string][]string{
			"claude-opus-4-8": {
				ClaudeCodeBillingHeader: {"configured-billing"},
			},
		},
	}

	headers := http.Header{}
	headers.Set(ClaudeCodeBillingHeader, "client-billing")
	settings.WriteHeaders("claude-opus-4-8", &headers)

	if got := headers.Get(ClaudeCodeBillingHeader); got != "" {
		t.Fatalf("billing header = %q, want empty", got)
	}
}

func TestClaudeSettingsWriteHeadersKeepsBillingHeaderWhenDisabled(t *testing.T) {
	settings := &ClaudeSettings{}
	headers := http.Header{}
	headers.Set(ClaudeCodeBillingHeader, "client-billing")
	settings.WriteHeaders("claude-opus-4-8", &headers)

	if got := headers.Get(ClaudeCodeBillingHeader); got != "client-billing" {
		t.Fatalf("billing header = %q, want client-billing", got)
	}
}

func TestShouldRemoveClaudeCodeBillingHeaderIsCaseInsensitive(t *testing.T) {
	settings := GetClaudeSettings()
	original := settings.RemoveClaudeCodeBillingHeaderEnabled
	settings.RemoveClaudeCodeBillingHeaderEnabled = true
	t.Cleanup(func() { settings.RemoveClaudeCodeBillingHeaderEnabled = original })

	if !ShouldRemoveClaudeCodeBillingHeader(" X-Anthropic-Billing-Header ") {
		t.Fatal("expected billing header to be removed")
	}
}
