package gemini

import (
	"strings"
	"testing"

	"github.com/NookMux/NookMux/internal/common"
	"github.com/NookMux/NookMux/internal/domain/shared"
	relaycommon "github.com/NookMux/NookMux/internal/relay/common"
	relayconstant "github.com/NookMux/NookMux/internal/relay/constant"
)

// TestAdaptorGetRequestURL_CountTokensAction verifies the Gemini-native
// countTokens action maps to the upstream :countTokens endpoint.
func TestAdaptorGetRequestURL_CountTokensAction(t *testing.T) {
	adaptor := &Adaptor{}
	info := &relaycommon.RelayInfo{
		ChannelMeta: &relaycommon.ChannelMeta{
			ChannelBaseUrl:    "https://generativelanguage.googleapis.com",
			UpstreamModelName: "gemini-2.5-pro",
		},
		RelayMode:      relayconstant.RelayModeGemini,
		RequestURLPath: "/v1beta/models/gemini-2.5-pro:countTokens",
	}

	got, err := adaptor.GetRequestURL(info)
	if err != nil {
		t.Fatalf("GetRequestURL error = %v", err)
	}
	want := "https://generativelanguage.googleapis.com/v1beta/models/gemini-2.5-pro:countTokens"
	if got != want {
		t.Fatalf("countTokens URL = %q, want %q", got, want)
	}

	// 非 Gemini 原生入站（OpenAI 协议等）不应命中 countTokens 分支。
	info.RelayMode = relayconstant.RelayModeChatCompletions
	got, err = adaptor.GetRequestURL(info)
	if err != nil {
		t.Fatalf("GetRequestURL error = %v", err)
	}
	if strings.Contains(got, "countTokens") {
		t.Fatalf("OpenAI-inbound URL should keep generateContent action, got: %s", got)
	}
}

func TestAdaptorConvertClaudeRequestPreservesThinkingToolChoiceAndToolResults(t *testing.T) {
	request := &shared.ClaudeRequest{
		Model: "claude-3-7-sonnet",
		Thinking: &shared.Thinking{
			Type:         "enabled",
			BudgetTokens: common.GetPointer(4096),
		},
		ToolChoice: &shared.ClaudeToolChoice{
			Type: "tool",
			Name: "weather",
		},
		Messages: []shared.ClaudeMessage{
			{
				Role: "assistant",
				Content: []shared.ClaudeMediaMessage{
					{Type: "thinking", Thinking: common.GetPointer[string]("plan"), Signature: "sig_123"},
					{Type: "tool_use", Id: "call_1", Name: "weather", Input: map[string]any{"city": "Shanghai"}},
				},
			},
			{
				Role: "user",
				Content: []shared.ClaudeMediaMessage{
					{Type: "tool_result", ToolUseId: "call_1", Content: "{\"temp\":\"20\"}"},
				},
			},
		},
	}
	request.System = []shared.ClaudeMediaMessage{{Type: "text", Text: common.GetPointer[string]("follow the system")}}
	request.AddTool(&shared.Tool{Name: "weather", InputSchema: map[string]interface{}{"type": "object"}})

	adaptor := &Adaptor{}
	convertedAny, err := adaptor.ConvertClaudeRequest(nil, &relaycommon.RelayInfo{ChannelMeta: &relaycommon.ChannelMeta{UpstreamModelName: "gemini-2.5-pro"}}, request)
	if err != nil {
		t.Fatalf("ConvertClaudeRequest error = %v", err)
	}

	converted, ok := convertedAny.(*shared.GeminiChatRequest)
	if !ok {
		t.Fatalf("converted request type = %T, want *shared.GeminiChatRequest", convertedAny)
	}
	if converted.GenerationConfig.ThinkingConfig == nil || converted.GenerationConfig.ThinkingConfig.ThinkingBudget == nil || *converted.GenerationConfig.ThinkingConfig.ThinkingBudget != 4096 {
		t.Fatalf("thinking config = %+v, want budget 4096", converted.GenerationConfig.ThinkingConfig)
	}
	if !converted.GenerationConfig.ThinkingConfig.IncludeThoughts {
		t.Fatal("expected includeThoughts to be enabled")
	}
	if converted.ToolConfig == nil || converted.ToolConfig.FunctionCallingConfig == nil {
		t.Fatalf("tool config = %+v, want function calling config", converted.ToolConfig)
	}
	if converted.ToolConfig.FunctionCallingConfig.Mode != shared.FunctionCallingConfigMode("ANY") || len(converted.ToolConfig.FunctionCallingConfig.AllowedFunctionNames) != 1 || converted.ToolConfig.FunctionCallingConfig.AllowedFunctionNames[0] != "weather" {
		t.Fatalf("tool config = %+v, want ANY weather", converted.ToolConfig.FunctionCallingConfig)
	}
	if converted.SystemInstructions == nil || len(converted.SystemInstructions.Parts) != 1 || converted.SystemInstructions.Parts[0].Text != "follow the system" {
		t.Fatalf("system instructions = %+v, want one system text part", converted.SystemInstructions)
	}
	if len(converted.Contents) != 2 {
		t.Fatalf("contents len = %d, want 2", len(converted.Contents))
	}

	modelParts := converted.Contents[0].Parts
	if len(modelParts) < 2 {
		t.Fatalf("model parts len = %d, want at least 2", len(modelParts))
	}
	if !modelParts[0].Thought || modelParts[0].Text != "plan" || modelParts[0].GetThoughtSignature() != "sig_123" {
		t.Fatalf("first model part = %+v, want thought plan with sig_123", modelParts[0])
	}
	if modelParts[1].FunctionCall == nil || modelParts[1].FunctionCall.FunctionName != "weather" {
		t.Fatalf("second model part = %+v, want weather function call", modelParts[1])
	}

	toolParts := converted.Contents[1].Parts
	if len(toolParts) != 1 || toolParts[0].FunctionResponse == nil {
		t.Fatalf("tool response parts = %+v, want one function response", toolParts)
	}
	if toolParts[0].FunctionResponse.GetID() != "call_1" {
		t.Fatalf("function response id = %q, want call_1", toolParts[0].FunctionResponse.GetID())
	}
}

// TestConvertToolChoiceToGeminiConfigRequired确保 channel-test 用 string "required" 作为
// tool_choice 时,Gemini adaptor 能正确转换为 mode:"ANY",保证唯一工具被强制调用。
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
