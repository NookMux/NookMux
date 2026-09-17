package convert

import (
	"testing"

	"github.com/NookMux/NookMux/internal/domain/shared"
	"github.com/NookMux/NookMux/pkg/jsonx"
)

func TestConvertResponsesRequestToChatCompletionsRequestComputerUseHistory(t *testing.T) {
	inputRaw, err := jsonx.Marshal([]map[string]any{
		{
			"type":    openAIResponsesInputItemTypeComputerCall,
			"call_id": "call_computer",
			"action":  map[string]any{"type": "screenshot"},
		},
		{
			"type":    openAIResponsesInputItemTypeComputerCallOutput,
			"call_id": "call_computer",
			"output":  map[string]any{"type": "input_text", "text": "screen text"},
		},
		{
			"type":    "message",
			"role":    "user",
			"content": "continue",
		},
	})
	if err != nil {
		t.Fatalf("marshal input error: %v", err)
	}

	got, err := ConvertResponsesRequestToChatCompletionsRequest(&shared.OpenAIResponsesRequest{
		Model: "claude-opus-4-8",
		Input: inputRaw,
	})
	if err != nil {
		t.Fatalf("ConvertResponsesRequestToChatCompletionsRequest() error: %v", err)
	}

	if len(got.Messages) != 2 {
		t.Fatalf("messages len = %d, want 2: %#v", len(got.Messages), got.Messages)
	}
	if got.Messages[0].Role != "user" || got.Messages[0].ToolCallId != "" {
		t.Fatalf("computer output message = %#v, want a user message without a tool call id", got.Messages[0])
	}
	wantPrefix := "computer_call_output call_computer output:\n"
	content, ok := got.Messages[0].Content.(string)
	if !ok || len(content) < len(wantPrefix) || content[:len(wantPrefix)] != wantPrefix {
		t.Fatalf("computer output content = %#v, want prefix %q", got.Messages[0].Content, wantPrefix)
	}
	var output map[string]any
	if err := jsonx.Unmarshal([]byte(content[len(wantPrefix):]), &output); err != nil {
		t.Fatalf("unmarshal normalized computer output: %v", err)
	}
	if output["text"] != "screen text" {
		t.Fatalf("normalized computer output = %#v, want screen text", output)
	}
	if got.Messages[1].Role != "user" || got.Messages[1].StringContent() != "continue" {
		t.Fatalf("following message = %#v, want user message %q", got.Messages[1], "continue")
	}
}

func TestConvertResponsesRequestToChatCompletionsRequestDropsImageOnlyComputerCallOutput(t *testing.T) {
	inputRaw, err := jsonx.Marshal([]map[string]any{
		{
			"type":    openAIResponsesInputItemTypeComputerCallOutput,
			"call_id": "call_computer",
			"output": map[string]any{
				"type":      openAIResponsesInputTypeImage,
				"image_url": "data:image/png;base64,AAAA",
			},
		},
		{
			"type":    "message",
			"role":    "user",
			"content": "continue",
		},
	})
	if err != nil {
		t.Fatalf("marshal input error: %v", err)
	}

	got, err := ConvertResponsesRequestToChatCompletionsRequest(&shared.OpenAIResponsesRequest{
		Model: "claude-opus-4-8",
		Input: inputRaw,
	})
	if err != nil {
		t.Fatalf("ConvertResponsesRequestToChatCompletionsRequest() error: %v", err)
	}
	if len(got.Messages) != 1 || got.Messages[0].Role != "user" || got.Messages[0].StringContent() != "continue" {
		t.Fatalf("messages = %#v, want only the following user message", got.Messages)
	}
}

func TestConvertResponsesRequestToChatCompletionsRequestNormalizesCodexHistory(t *testing.T) {
	inputRaw, err := jsonx.Marshal([]map[string]any{
		{
			"type":    "message",
			"role":    "developer",
			"content": "developer note",
		},
		{
			"type":    openAIResponsesInputItemTypeLocalShellCallOutput,
			"call_id": "call_shell",
			"output":  "shell ok",
		},
		{
			"type": openAIResponsesInputItemTypeImageGenerationCall,
			"id":   "ig_123",
		},
		{
			"type":    "message",
			"role":    "user",
			"content": "continue",
		},
	})
	if err != nil {
		t.Fatalf("marshal input error: %v", err)
	}

	got, err := ConvertResponsesRequestToChatCompletionsRequest(&shared.OpenAIResponsesRequest{
		Model: "claude-opus-4-8",
		Input: inputRaw,
	})
	if err != nil {
		t.Fatalf("ConvertResponsesRequestToChatCompletionsRequest() error: %v", err)
	}
	if len(got.Messages) != 3 {
		t.Fatalf("messages len = %d, want 3: %#v", len(got.Messages), got.Messages)
	}
	if got.Messages[0].Role != "system" || got.Messages[0].StringContent() != "developer note" {
		t.Fatalf("developer message = %#v, want system message %q", got.Messages[0], "developer note")
	}
	if got.Messages[1].Role != "user" || got.Messages[1].StringContent() != "local_shell_call_output call_shell output:\nshell ok" {
		t.Fatalf("local shell output message = %#v", got.Messages[1])
	}
	if got.Messages[2].Role != "user" || got.Messages[2].StringContent() != "continue" {
		t.Fatalf("following message = %#v, want user message %q", got.Messages[2], "continue")
	}
}
