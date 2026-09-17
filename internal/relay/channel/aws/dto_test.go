package aws

import (
	"bytes"
	"net/http"
	"strings"
	"testing"

	"github.com/NookMux/NookMux/internal/domain/shared"
	"github.com/NookMux/NookMux/pkg/jsonx"
)

func TestFormatRequestPreservesContextManagement(t *testing.T) {
	const contextManagement = `{"type":"clear_at_turn","clear_at_turn":{"turns":3}}`
	body := strings.NewReader(`{
		"anthropic_version": "bedrock-2023-05-31",
		"max_tokens": 1024,
		"messages": [{"role": "user", "content": "hello"}],
		"context_management": ` + contextManagement + `
	}`)

	got, err := formatRequest(body, http.Header{})
	if err != nil {
		t.Fatalf("formatRequest: %v", err)
	}
	if got == nil {
		t.Fatal("formatRequest returned nil request")
	}
	if !bytes.Equal(got.ContextManagement, []byte(contextManagement)) {
		t.Fatalf("ContextManagement = %s, want %s", got.ContextManagement, contextManagement)
	}
	if got.AnthropicVersion != "bedrock-2023-05-31" {
		t.Fatalf("AnthropicVersion = %q", got.AnthropicVersion)
	}
}

func TestFormatRequestPreservesExplicitZeroValues(t *testing.T) {
	t.Parallel()

	request, err := formatRequest(strings.NewReader(`{
		"messages":[{"role":"user","content":"hello"}],
		"max_tokens":0,
		"temperature":0,
		"top_p":0,
		"top_k":0
	}`), http.Header{})
	if err != nil {
		t.Fatal(err)
	}
	if request.MaxTokens == nil || *request.MaxTokens != 0 || request.Temperature == nil || *request.Temperature != 0 || request.TopP == nil || *request.TopP != 0 || request.TopK == nil || *request.TopK != 0 {
		t.Fatalf("explicit zero values were not retained: %#v", request)
	}
	payload, err := jsonx.Marshal(request)
	if err != nil {
		t.Fatal(err)
	}
	var values map[string]any
	if err := jsonx.Unmarshal(payload, &values); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"max_tokens", "temperature", "top_p", "top_k"} {
		if _, ok := values[key]; !ok {
			t.Fatalf("payload omitted explicit zero field %q: %s", key, payload)
		}
	}
}

func TestConvertToNovaRequestPreservesNonZeroPointerConfiguration(t *testing.T) {
	t.Parallel()

	maxTokens := uint(64)
	temperature := 0.25
	request := convertToNovaRequest(&shared.GeneralOpenAIRequest{
		MaxTokens:   maxTokens,
		Temperature: &temperature,
		TopP:        0.5,
		TopK:        4,
	})
	if request.InferenceConfig == nil || request.InferenceConfig.MaxTokens == nil || *request.InferenceConfig.MaxTokens != int(maxTokens) || request.InferenceConfig.Temperature == nil || *request.InferenceConfig.Temperature != temperature || request.InferenceConfig.TopP == nil || *request.InferenceConfig.TopP != 0.5 || request.InferenceConfig.TopK == nil || *request.InferenceConfig.TopK != 4 {
		t.Fatalf("nova inference config = %#v", request.InferenceConfig)
	}
}
