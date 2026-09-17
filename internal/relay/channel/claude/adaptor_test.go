package claude

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/NookMux/NookMux/internal/common"
	modelconfig "github.com/NookMux/NookMux/internal/config/model"
	"github.com/NookMux/NookMux/internal/domain/channel/constant"
	"github.com/NookMux/NookMux/internal/domain/shared"
	relaycommon "github.com/NookMux/NookMux/internal/relay/common"

	"github.com/NookMux/NookMux/pkg/jsonx"

	relayconstant "github.com/NookMux/NookMux/internal/relay/constant"
	"github.com/gin-gonic/gin"
)

func TestCommonClaudeHeadersOperationRemovesBillingHeader(t *testing.T) {
	settings := modelconfig.GetClaudeSettings()
	original := settings.RemoveClaudeCodeBillingHeaderEnabled
	settings.RemoveClaudeCodeBillingHeaderEnabled = true
	t.Cleanup(func() { settings.RemoveClaudeCodeBillingHeaderEnabled = original })

	request := &http.Request{Header: http.Header{}}
	request.Header.Set(modelconfig.ClaudeCodeBillingHeader, "client-billing")
	request.Header.Set("anthropic-beta", "computer-use-2025-01-24")
	ctx := &gin.Context{Request: request}
	headers := http.Header{}
	CommonClaudeHeadersOperation(ctx, &headers, &relaycommon.RelayInfo{OriginModelName: "claude-opus-4-8"})

	if headers.Get(modelconfig.ClaudeCodeBillingHeader) != "" {
		t.Fatal("billing header was forwarded")
	}
	if headers.Get("anthropic-beta") != "computer-use-2025-01-24" {
		t.Fatalf("anthropic-beta = %q", headers.Get("anthropic-beta"))
	}
}

func TestAdaptorConvertGeminiRequestPreservesThinkingAndToolResults(t *testing.T) {
	budget := 2048
	thoughtPart := shared.GeminiPart{Text: "plan", Thought: true}
	thoughtPart.SetThoughtSignature("sig_123")

	toolResponse := &shared.GeminiFunctionResponse{
		Name:     "weather",
		Response: map[string]interface{}{"temp": "20"},
	}
	toolResponse.SetID("call_1")

	request := &shared.GeminiChatRequest{
		Contents: []shared.GeminiChatContent{
			{
				Role:  "user",
				Parts: []shared.GeminiPart{{Text: "what is the weather"}},
			},
			{
				Role: "model",
				Parts: []shared.GeminiPart{
					thoughtPart,
					{FunctionCall: &shared.FunctionCall{FunctionName: "weather", Arguments: map[string]interface{}{"city": "Shanghai"}}},
				},
			},
			{
				Role:  "user",
				Parts: []shared.GeminiPart{{FunctionResponse: toolResponse}},
			},
		},
		GenerationConfig: shared.GeminiChatGenerationConfig{
			ThinkingConfig: &shared.GeminiThinkingConfig{
				IncludeThoughts: true,
				ThinkingBudget:  common.GetPointer(budget),
			},
		},
		ToolConfig: &shared.ToolConfig{
			FunctionCallingConfig: &shared.FunctionCallingConfig{
				Mode:                 shared.FunctionCallingConfigMode("ANY"),
				AllowedFunctionNames: []string{"weather"},
			},
		},
		SystemInstructions: &shared.GeminiChatContent{Parts: []shared.GeminiPart{{Text: "follow the system"}}},
	}
	request.SetTools([]shared.GeminiChatTool{{
		FunctionDeclarations: []shared.FunctionRequest{{
			Name:       "weather",
			Parameters: map[string]interface{}{"type": "object"},
		}},
	}})

	adaptor := &Adaptor{}
	convertedAny, err := adaptor.ConvertGeminiRequest(nil, &relaycommon.RelayInfo{ChannelMeta: &relaycommon.ChannelMeta{UpstreamModelName: "claude-3-7-sonnet"}}, request)
	if err != nil {
		t.Fatalf("ConvertGeminiRequest error = %v", err)
	}

	converted, ok := convertedAny.(*shared.ClaudeRequest)
	if !ok {
		t.Fatalf("converted request type = %T, want *shared.ClaudeRequest", convertedAny)
	}
	if converted.Thinking == nil || converted.Thinking.GetBudgetTokens() != budget {
		t.Fatalf("thinking = %+v, want enabled budget %d", converted.Thinking, budget)
	}

	toolChoice, ok := converted.ToolChoice.(*shared.ClaudeToolChoice)
	if !ok {
		t.Fatalf("tool choice type = %T, want *shared.ClaudeToolChoice", converted.ToolChoice)
	}
	if toolChoice.Type != "tool" || toolChoice.Name != "weather" {
		t.Fatalf("tool choice = %+v, want named weather tool", toolChoice)
	}

	if len(converted.ParseSystem()) != 1 || converted.ParseSystem()[0].GetText() != "follow the system" {
		t.Fatalf("system = %+v, want one system text block", converted.System)
	}
	if len(converted.Messages) != 3 {
		t.Fatalf("messages len = %d, want 3", len(converted.Messages))
	}

	assistantContent, err := converted.Messages[1].ParseContent()
	if err != nil {
		t.Fatalf("parse assistant content error = %v", err)
	}
	if len(assistantContent) < 2 {
		t.Fatalf("assistant content len = %d, want at least 2, content = %+v", len(assistantContent), assistantContent)
	}
	if assistantContent[0].Type != "thinking" || assistantContent[0].Thinking == nil || *assistantContent[0].Thinking != "plan" || assistantContent[0].Signature != "sig_123" {
		t.Fatalf("assistant thinking block = %+v, want thinking plan with sig_123", assistantContent[0])
	}
	if assistantContent[1].Type != "tool_use" || assistantContent[1].Name != "weather" || assistantContent[1].Id != "call_1" {
		t.Fatalf("assistant tool_use = %+v, want weather/call_1", assistantContent[1])
	}

	toolResultContent, err := converted.Messages[2].ParseContent()
	if err != nil {
		t.Fatalf("parse tool result content error = %v", err)
	}
	if len(toolResultContent) != 1 || toolResultContent[0].Type != "tool_result" || toolResultContent[0].ToolUseId != "call_1" {
		t.Fatalf("tool result content = %+v, want tool_result for call_1", toolResultContent)
	}
}

func TestAdaptorConvertOpenAIResponsesRequestUsesSharedRulesForTools(t *testing.T) {
	toolsRaw, err := jsonx.Marshal([]map[string]any{{
		"type":        "function",
		"name":        "weather",
		"description": "Get weather",
		"parameters": map[string]any{
			"type": "object",
			"properties": map[string]any{
				"city": map[string]any{"type": "string"},
			},
		},
	}})
	if err != nil {
		t.Fatalf("marshal tools error = %v", err)
	}
	inputRaw, err := jsonx.Marshal([]map[string]any{
		{
			"type":    "message",
			"role":    "user",
			"content": "weather in Shanghai?",
		},
		{
			"type":      "function_call",
			"call_id":   "call_weather",
			"name":      "weather",
			"arguments": json.RawMessage(`{"city":"Shanghai"}`),
		},
	})
	if err != nil {
		t.Fatalf("marshal input error = %v", err)
	}

	info := &relaycommon.RelayInfo{
		RelayFormat:            relayconstant.RelayFormatOpenAIResponses,
		RequestConversionChain: []relayconstant.RelayFormat{relayconstant.RelayFormatOpenAIResponses},
		ChannelMeta: &relaycommon.ChannelMeta{
			ChannelType:       1,
			UpstreamModelName: "claude-3-7-sonnet",
		},
	}
	convertedAny, err := (&Adaptor{}).ConvertOpenAIResponsesRequest(nil, info, shared.OpenAIResponsesRequest{
		Model:           "claude-3-7-sonnet",
		Input:           inputRaw,
		Tools:           toolsRaw,
		MaxOutputTokens: 1024,
	})
	if err != nil {
		t.Fatalf("ConvertOpenAIResponsesRequest error = %v", err)
	}
	converted, ok := convertedAny.(*shared.ClaudeRequest)
	if !ok {
		t.Fatalf("converted type = %T, want *shared.ClaudeRequest", convertedAny)
	}
	if info.OpenAIResponsesToolContext == nil {
		t.Fatal("OpenAIResponsesToolContext is nil")
	}
	if got, want := info.RequestConversionChain, []relayconstant.RelayFormat{relayconstant.RelayFormatOpenAIResponses, relayconstant.RelayFormatOpenAI, relayconstant.RelayFormatClaude}; len(got) != len(want) || got[0] != want[0] || got[1] != want[1] || got[2] != want[2] {
		t.Fatalf("RequestConversionChain = %#v, want %#v", got, want)
	}
	if converted.MaxTokens != 1024 {
		t.Fatalf("MaxTokens = %d, want 1024", converted.MaxTokens)
	}
	if len(converted.GetTools()) != 1 {
		t.Fatalf("tools len = %d, want 1", len(converted.GetTools()))
	}
	if len(converted.Messages) != 2 {
		t.Fatalf("messages len = %d, want 2", len(converted.Messages))
	}
	content, err := converted.Messages[1].ParseContent()
	if err != nil {
		t.Fatalf("parse assistant content error = %v", err)
	}
	if len(content) != 1 || content[0].Type != "tool_use" || content[0].Id != "call_weather" || content[0].Name != "weather" {
		t.Fatalf("assistant content = %+v, want weather tool_use", content)
	}
}

func TestAdaptorConvertOpenAIResponsesRequestRemovesUnsupportedComputerCallHistory(t *testing.T) {
	inputRaw, err := jsonx.Marshal([]map[string]any{
		{
			"type":    "computer_call",
			"call_id": "call_computer",
			"action":  map[string]any{"type": "screenshot"},
		},
		{
			"type":    "computer_call_output",
			"call_id": "call_computer",
			"output": map[string]any{
				"type":      "input_image",
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

	info := &relaycommon.RelayInfo{
		RelayFormat:            relayconstant.RelayFormatOpenAIResponses,
		RequestConversionChain: []relayconstant.RelayFormat{relayconstant.RelayFormatOpenAIResponses},
		ChannelMeta: &relaycommon.ChannelMeta{
			ChannelType:       constant.ChannelTypeAnthropic,
			UpstreamModelName: "claude-opus-4-8",
		},
	}
	convertedAny, err := (&Adaptor{}).ConvertOpenAIResponsesRequest(nil, info, shared.OpenAIResponsesRequest{
		Model:           "claude-opus-4-8",
		Input:           inputRaw,
		MaxOutputTokens: 128,
	})
	if err != nil {
		t.Fatalf("ConvertOpenAIResponsesRequest error: %v", err)
	}
	converted, ok := convertedAny.(*shared.ClaudeRequest)
	if !ok {
		t.Fatalf("converted type = %T, want *shared.ClaudeRequest", convertedAny)
	}
	if len(converted.Messages) != 1 {
		t.Fatalf("messages len = %d, want only the supported user message: %#v", len(converted.Messages), converted.Messages)
	}
	if converted.Messages[0].Role != "user" || converted.Messages[0].GetStringContent() != "continue" {
		t.Fatalf("converted message = %#v, want user message %q", converted.Messages[0], "continue")
	}
}

func TestAdaptorConvertOpenAIRequestClaudeEffortToolCallThinkingEnabled(t *testing.T) {
	info := &relaycommon.RelayInfo{
		ChannelMeta: &relaycommon.ChannelMeta{
			ChannelType:       constant.ChannelTypeAnthropic,
			UpstreamModelName: "claude-opus-4-6-high",
		},
	}

	convertedAny, err := (&Adaptor{}).ConvertOpenAIRequest(nil, info, buildOpenAIWeatherToolRequest("claude-opus-4-6-high", ""))
	if err != nil {
		t.Fatalf("ConvertOpenAIRequest error = %v", err)
	}
	converted, ok := convertedAny.(*shared.ClaudeRequest)
	if !ok {
		t.Fatalf("converted type = %T, want *shared.ClaudeRequest", convertedAny)
	}

	if converted.Model != "claude-opus-4-6" {
		t.Fatalf("model = %q, want claude-opus-4-6", converted.Model)
	}
	if converted.Thinking == nil || converted.Thinking.Type != "adaptive" || converted.Thinking.BudgetTokens != nil {
		t.Fatalf("thinking = %+v, want adaptive without budget_tokens", converted.Thinking)
	}
	var outputConfig shared.ClaudeOutputConfig
	if err := jsonx.Unmarshal(converted.OutputConfig, &outputConfig); err != nil {
		t.Fatalf("unmarshal output_config error = %v", err)
	}
	if outputConfig.Effort != "high" {
		t.Fatalf("output_config.effort = %q, want high", outputConfig.Effort)
	}
	if got := converted.GetTools(); len(got) != 1 {
		t.Fatalf("tools len = %d, want 1", len(got))
	}
	if info.ReasoningEffort != "high" {
		t.Fatalf("info.ReasoningEffort = %q, want high", info.ReasoningEffort)
	}
}

func TestAdaptorConvertOpenAIRequestClaudeOpus47UsesSummarizedAdaptiveThinking(t *testing.T) {
	info := &relaycommon.RelayInfo{
		ChannelMeta: &relaycommon.ChannelMeta{
			ChannelType:       constant.ChannelTypeAnthropic,
			UpstreamModelName: "claude-opus-4-7-high",
		},
	}
	request := buildOpenAIWeatherToolRequest("claude-opus-4-7-high", "")
	request.Temperature = common.GetPointer[float64](0.2)
	request.TopP = 0.8
	request.TopK = 40

	convertedAny, err := (&Adaptor{}).ConvertOpenAIRequest(nil, info, request)
	if err != nil {
		t.Fatalf("ConvertOpenAIRequest error = %v", err)
	}
	converted, ok := convertedAny.(*shared.ClaudeRequest)
	if !ok {
		t.Fatalf("converted type = %T, want *shared.ClaudeRequest", convertedAny)
	}

	if converted.Model != "claude-opus-4-7" {
		t.Fatalf("model = %q, want claude-opus-4-7", converted.Model)
	}
	if info.UpstreamModelName != "claude-opus-4-7" {
		t.Fatalf("info.UpstreamModelName = %q, want claude-opus-4-7", info.UpstreamModelName)
	}
	if converted.Thinking == nil || converted.Thinking.Type != "adaptive" || converted.Thinking.Display != "summarized" || converted.Thinking.BudgetTokens != nil {
		t.Fatalf("thinking = %+v, want adaptive summarized without budget_tokens", converted.Thinking)
	}
	var outputConfig shared.ClaudeOutputConfig
	if err := jsonx.Unmarshal(converted.OutputConfig, &outputConfig); err != nil {
		t.Fatalf("unmarshal output_config error = %v", err)
	}
	if outputConfig.Effort != "high" {
		t.Fatalf("output_config.effort = %q, want high", outputConfig.Effort)
	}
	if converted.Temperature != nil || converted.TopP != 0 || converted.TopK != 0 {
		t.Fatalf("sampling params = temperature:%v top_p:%v top_k:%v, want omitted defaults", converted.Temperature, converted.TopP, converted.TopK)
	}
}

func TestAdaptorConvertOpenAIRequestClaudeOpus47ThinkingSuffixUsesHighEffort(t *testing.T) {
	info := &relaycommon.RelayInfo{
		ChannelMeta: &relaycommon.ChannelMeta{
			ChannelType:       constant.ChannelTypeAnthropic,
			UpstreamModelName: "claude-opus-4-7-thinking",
		},
	}

	convertedAny, err := (&Adaptor{}).ConvertOpenAIRequest(nil, info, buildOpenAIWeatherToolRequest("claude-opus-4-7-thinking", ""))
	if err != nil {
		t.Fatalf("ConvertOpenAIRequest error = %v", err)
	}
	converted, ok := convertedAny.(*shared.ClaudeRequest)
	if !ok {
		t.Fatalf("converted type = %T, want *shared.ClaudeRequest", convertedAny)
	}

	if converted.Model != "claude-opus-4-7" {
		t.Fatalf("model = %q, want claude-opus-4-7", converted.Model)
	}
	if converted.Thinking == nil || converted.Thinking.Type != "adaptive" || converted.Thinking.Display != "summarized" {
		t.Fatalf("thinking = %+v, want adaptive summarized", converted.Thinking)
	}
	var outputConfig shared.ClaudeOutputConfig
	if err := jsonx.Unmarshal(converted.OutputConfig, &outputConfig); err != nil {
		t.Fatalf("unmarshal output_config error = %v", err)
	}
	if outputConfig.Effort != "high" {
		t.Fatalf("output_config.effort = %q, want high", outputConfig.Effort)
	}
}

func TestAdaptorConvertOpenAIRequestClaudeOpus48UsesAdaptiveThinking(t *testing.T) {
	info := &relaycommon.RelayInfo{
		ChannelMeta: &relaycommon.ChannelMeta{
			ChannelType:       constant.ChannelTypeAnthropic,
			UpstreamModelName: "claude-opus-4-8-high",
		},
	}

	convertedAny, err := (&Adaptor{}).ConvertOpenAIRequest(nil, info, buildOpenAIWeatherToolRequest("claude-opus-4-8-high", ""))
	if err != nil {
		t.Fatalf("ConvertOpenAIRequest error = %v", err)
	}
	converted, ok := convertedAny.(*shared.ClaudeRequest)
	if !ok {
		t.Fatalf("converted type = %T, want *shared.ClaudeRequest", convertedAny)
	}
	if converted.Model != "claude-opus-4-8" {
		t.Fatalf("model = %q, want claude-opus-4-8", converted.Model)
	}
	if converted.Thinking == nil || converted.Thinking.Type != "adaptive" || converted.Thinking.BudgetTokens != nil {
		t.Fatalf("thinking = %+v, want adaptive without budget_tokens", converted.Thinking)
	}
	var outputConfig shared.ClaudeOutputConfig
	if err := jsonx.Unmarshal(converted.OutputConfig, &outputConfig); err != nil {
		t.Fatalf("unmarshal output_config error = %v", err)
	}
	if outputConfig.Effort != "high" {
		t.Fatalf("output_config.effort = %q, want high", outputConfig.Effort)
	}
}

func TestAdaptorConvertOpenAIRequestClaudeEffortToolCallThinkingDisabled(t *testing.T) {
	info := &relaycommon.RelayInfo{
		ChannelMeta: &relaycommon.ChannelMeta{
			ChannelType:       constant.ChannelTypeAnthropic,
			UpstreamModelName: "claude-opus-4-6",
		},
	}

	convertedAny, err := (&Adaptor{}).ConvertOpenAIRequest(nil, info, buildOpenAIWeatherToolRequest("claude-opus-4-6", "none"))
	if err != nil {
		t.Fatalf("ConvertOpenAIRequest error = %v", err)
	}
	converted, ok := convertedAny.(*shared.ClaudeRequest)
	if !ok {
		t.Fatalf("converted type = %T, want *shared.ClaudeRequest", convertedAny)
	}

	if converted.Thinking == nil || converted.Thinking.Type != "disabled" {
		t.Fatalf("thinking = %+v, want disabled", converted.Thinking)
	}
	if len(converted.OutputConfig) != 0 {
		t.Fatalf("output_config = %s, want empty when thinking is disabled", string(converted.OutputConfig))
	}
	if got := converted.GetTools(); len(got) != 1 {
		t.Fatalf("tools len = %d, want 1", len(got))
	}
	if info.ReasoningEffort != "none" {
		t.Fatalf("info.ReasoningEffort = %q, want none", info.ReasoningEffort)
	}
}

func TestAdaptorConvertOpenAIRequestDeepSeekClaudeEffortUsesOutputConfigOnly(t *testing.T) {
	info := &relaycommon.RelayInfo{
		ChannelMeta: &relaycommon.ChannelMeta{
			ChannelType:       constant.ChannelTypeDeepSeek,
			UpstreamModelName: "deepseek-chat",
		},
	}

	convertedAny, err := (&Adaptor{}).ConvertOpenAIRequest(nil, info, buildOpenAIWeatherToolRequest("deepseek-chat", "max"))
	if err != nil {
		t.Fatalf("ConvertOpenAIRequest error = %v", err)
	}
	converted, ok := convertedAny.(*shared.ClaudeRequest)
	if !ok {
		t.Fatalf("converted type = %T, want *shared.ClaudeRequest", convertedAny)
	}

	if converted.Thinking != nil {
		t.Fatalf("thinking = %+v, want nil for DeepSeek Anthropic-compatible effort", converted.Thinking)
	}
	var outputConfig shared.ClaudeOutputConfig
	if err := jsonx.Unmarshal(converted.OutputConfig, &outputConfig); err != nil {
		t.Fatalf("unmarshal output_config error = %v", err)
	}
	if outputConfig.Effort != "max" {
		t.Fatalf("output_config.effort = %q, want max", outputConfig.Effort)
	}
}

func TestAdaptorConvertOpenAIRequestRejectsUnsupportedClaudeReasoningEffort(t *testing.T) {
	_, err := (&Adaptor{}).ConvertOpenAIRequest(nil, &relaycommon.RelayInfo{}, buildOpenAIWeatherToolRequest("claude-3-7-sonnet-20250219", "banana"))
	if err == nil {
		t.Fatal("ConvertOpenAIRequest error is nil, want unsupported reasoning_effort error")
	}
}

func buildOpenAIWeatherToolRequest(model string, reasoningEffort string) *shared.GeneralOpenAIRequest {
	return &shared.GeneralOpenAIRequest{
		Model:           model,
		ReasoningEffort: reasoningEffort,
		Messages: []shared.Message{
			{
				Role:    "user",
				Content: "What is the weather in Tokyo? Call the tool if needed.",
			},
		},
		Tools: []shared.ToolCallRequest{
			{
				Type: "function",
				Function: shared.FunctionRequest{
					Name:        "get_current_weather",
					Description: "Get the current weather for a city",
					Parameters: map[string]any{
						"type": "object",
						"properties": map[string]any{
							"location": map[string]any{
								"type":        "string",
								"description": "City and country, for example Tokyo, Japan",
							},
						},
						"required": []string{"location"},
					},
				},
			},
		},
	}
}

// TestAdaptorConvertOpenAIRequestRequiredToolChoiceMapsToAny 确保 channel-test 用 string
// "required" 作为 tool_choice 时,Claude adaptor 会把它映射为 anthropic {"type":"any"},
// 即强制模型必须调用工具(在只声明一个工具时等价于强制调用该工具)。
func TestAdaptorConvertOpenAIRequestRequiredToolChoiceMapsToAny(t *testing.T) {
	info := &relaycommon.RelayInfo{
		ChannelMeta: &relaycommon.ChannelMeta{
			ChannelType:       constant.ChannelTypeAnthropic,
			UpstreamModelName: "claude-3-5-sonnet",
		},
	}
	request := buildOpenAIWeatherToolRequest("claude-3-5-sonnet", "")
	request.ToolChoice = "required"

	convertedAny, err := (&Adaptor{}).ConvertOpenAIRequest(nil, info, request)
	if err != nil {
		t.Fatalf("ConvertOpenAIRequest error = %v", err)
	}
	converted, ok := convertedAny.(*shared.ClaudeRequest)
	if !ok {
		t.Fatalf("converted type = %T, want *shared.ClaudeRequest", convertedAny)
	}
	choice, ok := converted.ToolChoice.(*shared.ClaudeToolChoice)
	if !ok {
		t.Fatalf("tool_choice type = %T, want *shared.ClaudeToolChoice", converted.ToolChoice)
	}
	if choice.Type != "any" {
		t.Fatalf("tool_choice.type = %q, want \"any\" for required tool_choice", choice.Type)
	}
	if choice.Name != "" {
		t.Fatalf("tool_choice.name = %q, want empty for required tool_choice", choice.Name)
	}
}
