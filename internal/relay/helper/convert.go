package helper

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/NookMux/NookMux/internal/common"
	billing "github.com/NookMux/NookMux/internal/domain/billing"
	"github.com/NookMux/NookMux/internal/domain/channel/constant"
	"github.com/NookMux/NookMux/internal/domain/shared"
	"github.com/NookMux/NookMux/internal/relay/channel/openrouter"
	relaycommon "github.com/NookMux/NookMux/internal/relay/common"
	"github.com/NookMux/NookMux/internal/relay/reasonmap"
	"github.com/NookMux/NookMux/pkg/jsonx"
)

func claudeThinkingBudgetToReasoningEffort(budget int) string {
	switch {
	case budget <= 0:
		return ""
	case budget <= 1280:
		return "low"
	case budget <= 2048:
		return "medium"
	default:
		return "high"
	}
}

func claudeWebSearchMaxUsesToContextSize(maxUses int) string {
	switch {
	case maxUses <= 0:
		return ""
	case maxUses <= 1:
		return "low"
	case maxUses <= 5:
		return "medium"
	default:
		return "high"
	}
}

func extractClaudeOutputConfigEffort(outputConfig json.RawMessage) string {
	if len(outputConfig) == 0 {
		return ""
	}

	var config shared.ClaudeOutputConfig
	if err := jsonx.Unmarshal(outputConfig, &config); err != nil {
		return ""
	}
	if config.Effort == "max" {
		return "xhigh"
	}
	return config.Effort
}

func extractClaudeReasoningEffort(claudeRequest shared.ClaudeRequest) string {
	if effort := extractClaudeOutputConfigEffort(claudeRequest.OutputConfig); effort != "" {
		return effort
	}
	if claudeRequest.Thinking == nil {
		return ""
	}
	if claudeRequest.Thinking.Type == "adaptive" {
		return "high"
	}
	return claudeThinkingBudgetToReasoningEffort(claudeRequest.Thinking.GetBudgetTokens())
}

func geminiThinkingBudgetToReasoningEffort(budget int) string {
	switch {
	case budget <= 0:
		return ""
	case budget <= 1024:
		return "low"
	case budget <= 8192:
		return "medium"
	default:
		return "high"
	}
}

func geminiThinkingLevelToReasoningEffort(level string) string {
	switch strings.ToLower(strings.TrimSpace(level)) {
	case "", "none":
		return ""
	case "minimal", "low":
		return "low"
	case "medium":
		return "medium"
	case "high", "xhigh", "max":
		return "high"
	default:
		return strings.ToLower(strings.TrimSpace(level))
	}
}

func buildOpenAIReasoningPayload(maxTokens int) (json.RawMessage, error) {
	if maxTokens <= 0 {
		return nil, nil
	}

	payload, err := jsonx.Marshal(openrouter.RequestReasoning{MaxTokens: maxTokens})
	if err != nil {
		return nil, fmt.Errorf("failed to marshal reasoning payload: %w", err)
	}
	return payload, nil
}

func buildOpenRouterClaudeReasoningPayload(claudeRequest shared.ClaudeRequest) (json.RawMessage, error) {
	effort := extractClaudeOutputConfigEffort(claudeRequest.OutputConfig)
	if effort != "" {
		payload, err := jsonx.Marshal(openrouter.RequestReasoning{
			Enabled: true,
			Effort:  effort,
		})
		if err != nil {
			return nil, fmt.Errorf("failed to marshal reasoning: %w", err)
		}
		return payload, nil
	}

	if claudeRequest.Thinking == nil {
		return nil, nil
	}

	var reasoning openrouter.RequestReasoning
	switch claudeRequest.Thinking.Type {
	case "enabled":
		reasoning.Enabled = true
		reasoning.MaxTokens = claudeRequest.Thinking.GetBudgetTokens()
	case "adaptive":
		reasoning.Enabled = true
		reasoning.Effort = "high"
	default:
		return nil, nil
	}

	payload, err := jsonx.Marshal(reasoning)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal reasoning: %w", err)
	}
	return payload, nil
}

func convertClaudeToolChoiceToOpenAI(toolChoice any) (any, *bool) {
	var claudeToolChoice *shared.ClaudeToolChoice
	switch v := toolChoice.(type) {
	case *shared.ClaudeToolChoice:
		claudeToolChoice = v
	case shared.ClaudeToolChoice:
		claudeToolChoice = &v
	}
	if claudeToolChoice == nil {
		return nil, nil
	}

	var openAIToolChoice any
	switch claudeToolChoice.Type {
	case "auto":
		openAIToolChoice = "auto"
	case "any":
		openAIToolChoice = "required"
	case "none":
		openAIToolChoice = "none"
	case "tool":
		openAIToolChoice = map[string]any{
			"type": "function",
			"function": map[string]any{
				"name": claudeToolChoice.Name,
			},
		}
	}

	var parallelToolCalls *bool
	if claudeToolChoice.Type != "none" && claudeToolChoice.DisableParallelToolUse {
		parallelToolCalls = common.GetPointer(false)
	}
	return openAIToolChoice, parallelToolCalls
}

func convertGeminiToolConfigToOpenAI(toolConfig *shared.ToolConfig) any {
	if toolConfig == nil || toolConfig.FunctionCallingConfig == nil {
		return nil
	}

	config := toolConfig.FunctionCallingConfig
	switch strings.ToUpper(string(config.Mode)) {
	case "AUTO":
		return "auto"
	case "NONE":
		return "none"
	case "ANY":
		if len(config.AllowedFunctionNames) == 1 {
			return map[string]any{
				"type": "function",
				"function": map[string]any{
					"name": config.AllowedFunctionNames[0],
				},
			}
		}
		return "required"
	default:
		return nil
	}
}

func buildOpenAIWebSearchOptions(tool *shared.ClaudeWebSearchTool) *shared.WebSearchOptions {
	if tool == nil {
		return nil
	}

	options := &shared.WebSearchOptions{
		SearchContextSize: claudeWebSearchMaxUsesToContextSize(tool.MaxUses),
	}
	if tool.UserLocation != nil {
		payload, err := jsonx.Marshal(map[string]any{
			"approximate": tool.UserLocation,
		})
		if err == nil {
			options.UserLocation = payload
		}
	}
	return options
}

// NormalizeCacheCreationSplit keeps Anthropic's aggregate cache-creation total
// consistent with its optional 5m/1h split. If the provider only reports an
// aggregate total, the remainder defaults to the 5m bucket.
func NormalizeCacheCreationSplit(totalTokens int, tokens5m int, tokens1h int) (int, int) {
	remainder := totalTokens - tokens5m - tokens1h
	if remainder < 0 {
		remainder = 0
	}
	return tokens5m + remainder, tokens1h
}

func buildClaudeUsageFromOpenAIUsage(openAIUsage *shared.Usage) *shared.ClaudeUsage {
	usage := shared.OpenAIUsageToClaudeUsage(openAIUsage)
	if usage == nil {
		return nil
	}
	cacheCreation5m := 0
	cacheCreation1h := 0
	if usage.CacheCreation != nil {
		cacheCreation5m = usage.CacheCreation.Ephemeral5mInputTokens
		cacheCreation1h = usage.CacheCreation.Ephemeral1hInputTokens
	}
	cacheCreation5m, cacheCreation1h = NormalizeCacheCreationSplit(usage.CacheCreationInputTokens, cacheCreation5m, cacheCreation1h)
	if cacheCreation5m > 0 || cacheCreation1h > 0 {
		usage.CacheCreation = &shared.ClaudeCacheCreationUsage{
			Ephemeral5mInputTokens: cacheCreation5m,
			Ephemeral1hInputTokens: cacheCreation1h,
		}
	}
	return usage
}

func appendOpenAIToolMessage(openAIMessages *[]shared.Message, claudeRequest shared.ClaudeRequest, mediaMsg shared.ClaudeMediaMessage) {
	toolName := mediaMsg.Name
	if toolName == "" {
		toolName = claudeRequest.SearchToolNameByToolCallId(mediaMsg.ToolUseId)
	}

	oaiToolMessage := shared.Message{
		Role:            "tool",
		Name:            &toolName,
		ToolCallId:      mediaMsg.ToolUseId,
		ToolCallIsError: mediaMsg.IsError,
	}
	if mediaMsg.IsStringContent() {
		oaiToolMessage.SetStringContent(mediaMsg.GetStringContent())
	} else {
		mediaContents := mediaMsg.ParseMediaContent()
		if len(mediaContents) == 0 {
			oaiToolMessage.SetStringContent("")
		} else {
			encodeJSON, _ := jsonx.Marshal(mediaContents)
			oaiToolMessage.SetStringContent(string(encodeJSON))
		}
	}
	*openAIMessages = append(*openAIMessages, oaiToolMessage)
}

func ClaudeToOpenAIRequest(claudeRequest shared.ClaudeRequest, info *relaycommon.RelayInfo) (*shared.GeneralOpenAIRequest, error) {
	openAIRequest := shared.GeneralOpenAIRequest{
		Model:       claudeRequest.Model,
		MaxTokens:   claudeRequest.MaxTokens,
		Temperature: claudeRequest.Temperature,
		TopP:        claudeRequest.TopP,
		Stream:      claudeRequest.Stream,
	}

	isOpenRouter := info.ChannelType == constant.ChannelTypeOpenRouter
	if isOpenRouter {
		reasoning, err := buildOpenRouterClaudeReasoningPayload(claudeRequest)
		if err != nil {
			return nil, err
		}
		if len(reasoning) > 0 {
			openAIRequest.Reasoning = reasoning
		}
	} else {
		openAIRequest.ReasoningEffort = extractClaudeReasoningEffort(claudeRequest)
	}

	if claudeRequest.Thinking != nil && claudeRequest.Thinking.Type == "enabled" {
		if !isOpenRouter {
			thinkingSuffix := "-thinking"
			if strings.HasSuffix(info.OriginModelName, thinkingSuffix) &&
				!strings.HasSuffix(openAIRequest.Model, thinkingSuffix) {
				openAIRequest.Model = openAIRequest.Model + thinkingSuffix
			}
		}
	}
	if claudeRequest.Thinking != nil && claudeRequest.Thinking.Type == "adaptive" {
		if !isOpenRouter && openAIRequest.ReasoningEffort == "" {
			// Claude adaptive thinking has no direct Chat Completions equivalent, so we
			// expose it as high effort to preserve the intent when routing through OpenAI-style channels.
			openAIRequest.ReasoningEffort = "high"
		}
	}

	// Convert stop sequences
	if len(claudeRequest.StopSequences) == 1 {
		openAIRequest.Stop = claudeRequest.StopSequences[0]
	} else if len(claudeRequest.StopSequences) > 1 {
		openAIRequest.Stop = claudeRequest.StopSequences
	}

	// Convert tools
	openAITools := make([]shared.ToolCallRequest, 0)
	normalTools, webSearchTools := shared.ProcessTools(claudeRequest.GetTools())
	for _, claudeTool := range normalTools {
		openAITool := shared.ToolCallRequest{
			Type: "function",
			Function: shared.FunctionRequest{
				Name:        claudeTool.Name,
				Description: claudeTool.Description,
				Parameters:  claudeTool.InputSchema,
			},
		}
		openAITools = append(openAITools, openAITool)
	}
	openAIRequest.Tools = openAITools
	openAIRequest.ToolChoice, openAIRequest.ParallelTooCalls = convertClaudeToolChoiceToOpenAI(claudeRequest.ToolChoice)
	if len(webSearchTools) > 0 {
		openAIRequest.WebSearchOptions = buildOpenAIWebSearchOptions(webSearchTools[0])
	}

	// Convert messages
	openAIMessages := make([]shared.Message, 0)

	// Add system message if present
	if claudeRequest.System != nil {
		if claudeRequest.IsStringSystem() && claudeRequest.GetStringSystem() != "" {
			openAIMessage := shared.Message{
				Role: "system",
			}
			openAIMessage.SetStringContent(claudeRequest.GetStringSystem())
			openAIMessages = append(openAIMessages, openAIMessage)
		} else {
			systems := claudeRequest.ParseSystem()
			if len(systems) > 0 {
				openAIMessage := shared.Message{
					Role: "system",
				}
				isOpenRouterClaude := isOpenRouter && strings.HasPrefix(info.UpstreamModelName, "anthropic/claude")
				if isOpenRouterClaude {
					systemMediaMessages := make([]shared.MediaContent, 0, len(systems))
					for _, system := range systems {
						message := shared.MediaContent{
							Type:         "text",
							Text:         system.GetText(),
							CacheControl: system.CacheControl,
						}
						systemMediaMessages = append(systemMediaMessages, message)
					}
					openAIMessage.SetMediaContent(systemMediaMessages)
				} else {
					systemStr := ""
					for _, system := range systems {
						if system.Text != nil {
							systemStr += *system.Text
						}
					}
					openAIMessage.SetStringContent(systemStr)
				}
				openAIMessages = append(openAIMessages, openAIMessage)
			}
		}
	}
	for _, claudeMessage := range claudeRequest.Messages {
		openAIMessage := shared.Message{
			Role: claudeMessage.Role,
		}

		if claudeMessage.IsStringContent() {
			openAIMessage.SetStringContent(claudeMessage.GetStringContent())
		} else {
			content, err := claudeMessage.ParseContent()
			if err != nil {
				return nil, err
			}
			contents := content
			var toolCalls []shared.ToolCallRequest
			mediaMessages := make([]shared.MediaContent, 0, len(contents))
			var reasoningBuilder strings.Builder
			var reasoningSignature string
			var redactedReasoningBuilder strings.Builder

			for _, mediaMsg := range contents {
				switch mediaMsg.Type {
				case "thinking":
					if mediaMsg.Thinking != nil {
						reasoningBuilder.WriteString(*mediaMsg.Thinking)
					}
					if mediaMsg.Signature != "" {
						reasoningSignature = mediaMsg.Signature
					}
				case "redacted_thinking":
					redactedReasoningBuilder.WriteString(mediaMsg.Data)
				case "text":
					message := shared.MediaContent{
						Type:         "text",
						Text:         mediaMsg.GetText(),
						CacheControl: mediaMsg.CacheControl,
					}
					mediaMessages = append(mediaMessages, message)
				case "image":
					if mediaMsg.Source == nil {
						continue
					}
					imageURL := mediaMsg.Source.Url
					if imageURL == "" {
						// Claude base64 images are converted back into OpenAI data URLs.
						imageURL = fmt.Sprintf("data:%s;base64,%s", mediaMsg.Source.MediaType, mediaMsg.Source.Data)
					}
					mediaMessage := shared.MediaContent{
						Type:     "image_url",
						ImageUrl: &shared.MessageImageUrl{Url: imageURL},
					}
					mediaMessages = append(mediaMessages, mediaMessage)
				case "tool_use":
					toolCall := shared.ToolCallRequest{
						ID:   mediaMsg.Id,
						Type: "function",
						Function: shared.FunctionRequest{
							Name:      mediaMsg.Name,
							Arguments: toJSONString(mediaMsg.Input),
						},
					}
					toolCalls = append(toolCalls, toolCall)
				case "tool_result":
					appendOpenAIToolMessage(&openAIMessages, claudeRequest, mediaMsg)
				}
			}

			if reasoningBuilder.Len() > 0 {
				openAIMessage.SetReasoningContent(reasoningBuilder.String())
			}
			if reasoningSignature != "" {
				openAIMessage.ReasoningSignature = reasoningSignature
			}
			if redactedReasoningBuilder.Len() > 0 {
				openAIMessage.RedactedReasoningContent = redactedReasoningBuilder.String()
			}

			if len(toolCalls) > 0 {
				openAIMessage.SetToolCalls(toolCalls)
			}

			if len(mediaMessages) > 0 && len(toolCalls) == 0 {
				openAIMessage.SetMediaContent(mediaMessages)
			}
		}
		if len(openAIMessage.ParseContent()) > 0 || len(openAIMessage.ToolCalls) > 0 || openAIMessage.ReasoningContent != nil || openAIMessage.ReasoningSignature != "" || openAIMessage.RedactedReasoningContent != "" {
			openAIMessages = append(openAIMessages, openAIMessage)
		}
	}

	openAIRequest.Messages = openAIMessages

	return &openAIRequest, nil
}

func generateStopBlock(index int) *shared.ClaudeResponse {
	return &shared.ClaudeResponse{
		Type:  "content_block_stop",
		Index: common.GetPointer[int](index),
	}
}

func StreamResponseOpenAI2Claude(openAIResponse *shared.ChatCompletionsStreamResponse, info *relaycommon.RelayInfo) []*shared.ClaudeResponse {
	if info.ClaudeConvertInfo.Done {
		return nil
	}

	var claudeResponses []*shared.ClaudeResponse
	// stopOpenBlocks emits the required content_block_stop event(s) for the currently open block(s)
	// according to Anthropic's SSE streaming state machine:
	// content_block_start -> content_block_delta* -> content_block_stop (per index).
	//
	// For text/thinking, there is at most one open block at info.ClaudeConvertInfo.Index.
	// For tools, OpenAI tool_calls can stream multiple parallel tool_use blocks (indexed from 0),
	// so we may have multiple open blocks and must stop each one explicitly.
	stopOpenBlocks := func() {
		switch info.ClaudeConvertInfo.LastMessagesType {
		case relaycommon.LastMessageTypeText, relaycommon.LastMessageTypeThinking:
			claudeResponses = append(claudeResponses, generateStopBlock(info.ClaudeConvertInfo.Index))
		case relaycommon.LastMessageTypeTools:
			base := info.ClaudeConvertInfo.ToolCallBaseIndex
			for offset := 0; offset <= info.ClaudeConvertInfo.ToolCallMaxIndexOffset; offset++ {
				claudeResponses = append(claudeResponses, generateStopBlock(base+offset))
			}
		}
	}
	// stopOpenBlocksAndAdvance closes the currently open block(s) and advances the content block index
	// to the next available slot for subsequent content_block_start events.
	//
	// This prevents invalid streams where a content_block_delta (e.g. thinking_delta) is emitted for an
	// index whose active content_block type is different (the typical cause of "Mismatched content block type").
	stopOpenBlocksAndAdvance := func() {
		if info.ClaudeConvertInfo.LastMessagesType == relaycommon.LastMessageTypeNone {
			return
		}
		stopOpenBlocks()
		switch info.ClaudeConvertInfo.LastMessagesType {
		case relaycommon.LastMessageTypeTools:
			info.ClaudeConvertInfo.Index = info.ClaudeConvertInfo.ToolCallBaseIndex + info.ClaudeConvertInfo.ToolCallMaxIndexOffset + 1
			info.ClaudeConvertInfo.ToolCallBaseIndex = 0
			info.ClaudeConvertInfo.ToolCallMaxIndexOffset = 0
		default:
			info.ClaudeConvertInfo.Index++
		}
		info.ClaudeConvertInfo.LastMessagesType = relaycommon.LastMessageTypeNone
	}
	flushPendingThinkingSignature := func(closeImmediately bool) {
		if info.ClaudeConvertInfo.PendingReasoningSignature == "" {
			return
		}
		if info.ClaudeConvertInfo.LastMessagesType != relaycommon.LastMessageTypeThinking {
			stopOpenBlocksAndAdvance()
			idx := info.ClaudeConvertInfo.Index
			claudeResponses = append(claudeResponses, &shared.ClaudeResponse{
				Index: &idx,
				Type:  "content_block_start",
				ContentBlock: &shared.ClaudeMediaMessage{
					Type:     "thinking",
					Thinking: common.GetPointer[string](""),
				},
			})
			info.ClaudeConvertInfo.LastMessagesType = relaycommon.LastMessageTypeThinking
		}
		idx := info.ClaudeConvertInfo.Index
		signature := info.ClaudeConvertInfo.PendingReasoningSignature
		claudeResponses = append(claudeResponses, &shared.ClaudeResponse{
			Index: &idx,
			Type:  "content_block_delta",
			Delta: &shared.ClaudeMediaMessage{
				Type:      "signature_delta",
				Signature: signature,
			},
		})
		info.ClaudeConvertInfo.PendingReasoningSignature = ""
		if closeImmediately {
			stopOpenBlocksAndAdvance()
		}
	}
	if info.SendResponseCount == 1 {
		msg := &shared.ClaudeMediaMessage{
			Id:    openAIResponse.Id,
			Model: openAIResponse.Model,
			Type:  "message",
			Role:  "assistant",
			Usage: &shared.ClaudeUsage{
				InputTokens:  info.GetEstimatePromptTokens(),
				OutputTokens: 0,
			},
		}
		msg.SetContent(make([]any, 0))
		claudeResponses = append(claudeResponses, &shared.ClaudeResponse{
			Type:    "message_start",
			Message: msg,
		})
		//claudeResponses = append(claudeResponses, &shared.ClaudeResponse{
		//	Type: "ping",
		//})
		if openAIResponse.IsToolCall() {
			info.ClaudeConvertInfo.LastMessagesType = relaycommon.LastMessageTypeTools
			info.ClaudeConvertInfo.ToolCallBaseIndex = 0
			info.ClaudeConvertInfo.ToolCallMaxIndexOffset = 0
			var toolCall shared.ToolCallResponse
			if len(openAIResponse.Choices) > 0 && len(openAIResponse.Choices[0].Delta.ToolCalls) > 0 {
				toolCall = openAIResponse.Choices[0].Delta.ToolCalls[0]
			} else {
				first := openAIResponse.GetFirstToolCall()
				if first != nil {
					toolCall = *first
				} else {
					toolCall = shared.ToolCallResponse{}
				}
			}
			resp := &shared.ClaudeResponse{
				Type: "content_block_start",
				ContentBlock: &shared.ClaudeMediaMessage{
					Id:    toolCall.ID,
					Type:  "tool_use",
					Name:  toolCall.Function.Name,
					Input: map[string]interface{}{},
				},
			}
			resp.SetIndex(0)
			claudeResponses = append(claudeResponses, resp)
			// 首块包含工具 delta，则追加 input_json_delta
			if toolCall.Function.Arguments != "" {
				idx := 0
				claudeResponses = append(claudeResponses, &shared.ClaudeResponse{
					Index: &idx,
					Type:  "content_block_delta",
					Delta: &shared.ClaudeMediaMessage{
						Type:        "input_json_delta",
						PartialJson: &toolCall.Function.Arguments,
					},
				})
			}
		}
		// 判断首个响应是否存在内容（非标准的 OpenAI 响应）
		if len(openAIResponse.Choices) > 0 {
			reasoning := openAIResponse.Choices[0].Delta.GetReasoningContent()
			reasoningSignature := ""
			if openAIResponse.Choices[0].Delta.ReasoningSignature != nil {
				reasoningSignature = *openAIResponse.Choices[0].Delta.ReasoningSignature
			}
			redactedReasoning := ""
			if openAIResponse.Choices[0].Delta.RedactedReasoningContent != nil {
				redactedReasoning = *openAIResponse.Choices[0].Delta.RedactedReasoningContent
			}
			content := openAIResponse.Choices[0].Delta.GetContentString()

			if reasoning != "" {
				if info.ClaudeConvertInfo.LastMessagesType != relaycommon.LastMessageTypeThinking {
					stopOpenBlocksAndAdvance()
				}
				idx := info.ClaudeConvertInfo.Index
				claudeResponses = append(claudeResponses, &shared.ClaudeResponse{
					Index: &idx,
					Type:  "content_block_start",
					ContentBlock: &shared.ClaudeMediaMessage{
						Type:     "thinking",
						Thinking: common.GetPointer[string](""),
					},
				})
				idx2 := idx
				claudeResponses = append(claudeResponses, &shared.ClaudeResponse{
					Index: &idx2,
					Type:  "content_block_delta",
					Delta: &shared.ClaudeMediaMessage{
						Type:     "thinking_delta",
						Thinking: &reasoning,
					},
				})
				if reasoningSignature != "" {
					info.ClaudeConvertInfo.PendingReasoningSignature = reasoningSignature
					flushPendingThinkingSignature(false)
				}
				info.ClaudeConvertInfo.LastMessagesType = relaycommon.LastMessageTypeThinking
			} else {
				if reasoningSignature != "" {
					info.ClaudeConvertInfo.PendingReasoningSignature = reasoningSignature
				}
				if redactedReasoning != "" {
					flushPendingThinkingSignature(true)
					stopOpenBlocksAndAdvance()
					idx := info.ClaudeConvertInfo.Index
					claudeResponses = append(claudeResponses, &shared.ClaudeResponse{
						Index: &idx,
						Type:  "content_block_start",
						ContentBlock: &shared.ClaudeMediaMessage{
							Type: "redacted_thinking",
							Data: redactedReasoning,
						},
					})
					claudeResponses = append(claudeResponses, generateStopBlock(idx))
					info.ClaudeConvertInfo.Index++
				}
				if content != "" {
					if info.ClaudeConvertInfo.LastMessagesType != relaycommon.LastMessageTypeText {
						stopOpenBlocksAndAdvance()
					}
					idx := info.ClaudeConvertInfo.Index
					claudeResponses = append(claudeResponses, &shared.ClaudeResponse{
						Index: &idx,
						Type:  "content_block_start",
						ContentBlock: &shared.ClaudeMediaMessage{
							Type: "text",
							Text: common.GetPointer[string](""),
						},
					})
					idx2 := idx
					claudeResponses = append(claudeResponses, &shared.ClaudeResponse{
						Index: &idx2,
						Type:  "content_block_delta",
						Delta: &shared.ClaudeMediaMessage{
							Type: "text_delta",
							Text: common.GetPointer[string](content),
						},
					})
					info.ClaudeConvertInfo.LastMessagesType = relaycommon.LastMessageTypeText
				}
			}
		}

		// 如果首块就带 finish_reason，需要立即发送停止块
		if len(openAIResponse.Choices) > 0 && openAIResponse.Choices[0].FinishReason != nil && *openAIResponse.Choices[0].FinishReason != "" {
			info.FinishReason = *openAIResponse.Choices[0].FinishReason
			flushPendingThinkingSignature(true)
			stopOpenBlocks()
			oaiUsage := openAIResponse.Usage
			if oaiUsage == nil {
				oaiUsage = info.ClaudeConvertInfo.Usage
			}
			if oaiUsage != nil {
				claudeUsage := buildClaudeUsageFromOpenAIUsage(oaiUsage)
				claudeResponses = append(claudeResponses, &shared.ClaudeResponse{
					Type:  "message_delta",
					Usage: claudeUsage,
					Delta: &shared.ClaudeMediaMessage{
						StopReason: common.GetPointer[string](stopReasonOpenAI2Claude(info.FinishReason)),
					},
				})
			}
			claudeResponses = append(claudeResponses, &shared.ClaudeResponse{
				Type: "message_stop",
			})
			info.ClaudeConvertInfo.Done = true
		}
		return claudeResponses
	}

	if len(openAIResponse.Choices) == 0 {
		// Some OpenAI-compatible upstreams end with a usage-only SSE chunk.
		oaiUsage := openAIResponse.Usage
		if oaiUsage == nil {
			oaiUsage = info.ClaudeConvertInfo.Usage
		}
		if oaiUsage != nil {
			flushPendingThinkingSignature(true)
			stopOpenBlocks()
			stopReason := stopReasonOpenAI2Claude(info.FinishReason)
			if stopReason == "" {
				stopReason = "end_turn"
			}
			claudeResponses = append(claudeResponses, &shared.ClaudeResponse{
				Type:  "message_delta",
				Usage: buildClaudeUsageFromOpenAIUsage(oaiUsage),
				Delta: &shared.ClaudeMediaMessage{
					StopReason: common.GetPointer[string](stopReason),
				},
			})
			claudeResponses = append(claudeResponses, &shared.ClaudeResponse{
				Type: "message_stop",
			})
			info.ClaudeConvertInfo.Done = true
		}
		return claudeResponses
	} else {
		chosenChoice := openAIResponse.Choices[0]
		doneChunk := chosenChoice.FinishReason != nil && *chosenChoice.FinishReason != ""
		if doneChunk {
			info.FinishReason = *chosenChoice.FinishReason
			if openAIResponse.Usage == nil && info.ClaudeConvertInfo.Usage == nil {
				// Some upstreams emit finish_reason first, then a final usage-only chunk.
				// Defer message_delta/message_stop so final Claude output carries usage.
				return claudeResponses
			}
		}
		if chosenChoice.Delta.ReasoningSignature != nil && *chosenChoice.Delta.ReasoningSignature != "" && chosenChoice.Delta.GetReasoningContent() == "" {
			info.ClaudeConvertInfo.PendingReasoningSignature = *chosenChoice.Delta.ReasoningSignature
		}

		var claudeResponse shared.ClaudeResponse
		var isEmpty bool
		claudeResponse.Type = "content_block_delta"
		if len(chosenChoice.Delta.ToolCalls) > 0 {
			flushPendingThinkingSignature(true)
			toolCalls := chosenChoice.Delta.ToolCalls
			if info.ClaudeConvertInfo.LastMessagesType != relaycommon.LastMessageTypeTools {
				stopOpenBlocksAndAdvance()
				info.ClaudeConvertInfo.ToolCallBaseIndex = info.ClaudeConvertInfo.Index
				info.ClaudeConvertInfo.ToolCallMaxIndexOffset = 0
			}
			info.ClaudeConvertInfo.LastMessagesType = relaycommon.LastMessageTypeTools
			base := info.ClaudeConvertInfo.ToolCallBaseIndex
			maxOffset := info.ClaudeConvertInfo.ToolCallMaxIndexOffset

			for i, toolCall := range toolCalls {
				offset := 0
				if toolCall.Index != nil {
					offset = *toolCall.Index
				} else {
					offset = i
				}
				if offset > maxOffset {
					maxOffset = offset
				}
				blockIndex := base + offset

				idx := blockIndex
				if toolCall.Function.Name != "" {
					claudeResponses = append(claudeResponses, &shared.ClaudeResponse{
						Index: &idx,
						Type:  "content_block_start",
						ContentBlock: &shared.ClaudeMediaMessage{
							Id:    toolCall.ID,
							Type:  "tool_use",
							Name:  toolCall.Function.Name,
							Input: map[string]interface{}{},
						},
					})
				}

				if len(toolCall.Function.Arguments) > 0 {
					claudeResponses = append(claudeResponses, &shared.ClaudeResponse{
						Index: &idx,
						Type:  "content_block_delta",
						Delta: &shared.ClaudeMediaMessage{
							Type:        "input_json_delta",
							PartialJson: &toolCall.Function.Arguments,
						},
					})
				}
			}
			info.ClaudeConvertInfo.ToolCallMaxIndexOffset = maxOffset
			info.ClaudeConvertInfo.Index = base + maxOffset
		} else {
			reasoning := chosenChoice.Delta.GetReasoningContent()
			redactedReasoning := ""
			if chosenChoice.Delta.RedactedReasoningContent != nil {
				redactedReasoning = *chosenChoice.Delta.RedactedReasoningContent
			}
			textContent := chosenChoice.Delta.GetContentString()
			if reasoning != "" || redactedReasoning != "" || textContent != "" {
				if reasoning != "" {
					if info.ClaudeConvertInfo.LastMessagesType != relaycommon.LastMessageTypeThinking {
						stopOpenBlocksAndAdvance()
						idx := info.ClaudeConvertInfo.Index
						claudeResponses = append(claudeResponses, &shared.ClaudeResponse{
							Index: &idx,
							Type:  "content_block_start",
							ContentBlock: &shared.ClaudeMediaMessage{
								Type:     "thinking",
								Thinking: common.GetPointer[string](""),
							},
						})
					}
					info.ClaudeConvertInfo.LastMessagesType = relaycommon.LastMessageTypeThinking
					claudeResponse.Delta = &shared.ClaudeMediaMessage{
						Type:     "thinking_delta",
						Thinking: &reasoning,
					}
					if chosenChoice.Delta.ReasoningSignature != nil && *chosenChoice.Delta.ReasoningSignature != "" {
						info.ClaudeConvertInfo.PendingReasoningSignature = *chosenChoice.Delta.ReasoningSignature
					}
				} else if redactedReasoning != "" {
					flushPendingThinkingSignature(true)
					stopOpenBlocksAndAdvance()
					idx := info.ClaudeConvertInfo.Index
					claudeResponses = append(claudeResponses, &shared.ClaudeResponse{
						Index: &idx,
						Type:  "content_block_start",
						ContentBlock: &shared.ClaudeMediaMessage{
							Type: "redacted_thinking",
							Data: redactedReasoning,
						},
					})
					claudeResponses = append(claudeResponses, generateStopBlock(idx))
					info.ClaudeConvertInfo.Index++
				} else {
					flushPendingThinkingSignature(true)
					if info.ClaudeConvertInfo.LastMessagesType != relaycommon.LastMessageTypeText {
						stopOpenBlocksAndAdvance()
						idx := info.ClaudeConvertInfo.Index
						claudeResponses = append(claudeResponses, &shared.ClaudeResponse{
							Index: &idx,
							Type:  "content_block_start",
							ContentBlock: &shared.ClaudeMediaMessage{
								Type: "text",
								Text: common.GetPointer[string](""),
							},
						})
					}
					info.ClaudeConvertInfo.LastMessagesType = relaycommon.LastMessageTypeText
					claudeResponse.Delta = &shared.ClaudeMediaMessage{
						Type: "text_delta",
						Text: common.GetPointer[string](textContent),
					}
				}
			} else {
				isEmpty = true
			}
		}

		claudeResponse.Index = common.GetPointer[int](info.ClaudeConvertInfo.Index)
		if !isEmpty && claudeResponse.Delta != nil {
			claudeResponses = append(claudeResponses, &claudeResponse)
			if claudeResponse.Delta.Type == "thinking_delta" {
				flushPendingThinkingSignature(false)
			}
		}

		if doneChunk || info.ClaudeConvertInfo.Done {
			flushPendingThinkingSignature(true)
			stopOpenBlocks()
			oaiUsage := openAIResponse.Usage
			if oaiUsage == nil {
				oaiUsage = info.ClaudeConvertInfo.Usage
			}
			if oaiUsage != nil {
				claudeUsage := buildClaudeUsageFromOpenAIUsage(oaiUsage)
				claudeResponses = append(claudeResponses, &shared.ClaudeResponse{
					Type:  "message_delta",
					Usage: claudeUsage,
					Delta: &shared.ClaudeMediaMessage{
						StopReason: common.GetPointer[string](stopReasonOpenAI2Claude(info.FinishReason)),
					},
				})
			}
			claudeResponses = append(claudeResponses, &shared.ClaudeResponse{
				Type: "message_stop",
			})
			info.ClaudeConvertInfo.Done = true
			return claudeResponses
		}
	}

	return claudeResponses
}

func ResponseOpenAI2Claude(openAIResponse *shared.OpenAITextResponse, info *relaycommon.RelayInfo) *shared.ClaudeResponse {
	var stopReason string
	contents := make([]shared.ClaudeMediaMessage, 0)
	claudeResponse := &shared.ClaudeResponse{
		Id:    openAIResponse.Id,
		Type:  "message",
		Role:  "assistant",
		Model: openAIResponse.Model,
	}
	for _, choice := range openAIResponse.Choices {
		stopReason = stopReasonOpenAI2Claude(choice.FinishReason)

		reasoningText := choice.Message.GetReasoningContent()
		if reasoningText != "" || choice.Message.ReasoningSignature != "" {
			claudeContent := shared.ClaudeMediaMessage{Type: "thinking", Signature: choice.Message.ReasoningSignature}
			claudeContent.Thinking = common.GetPointer[string](reasoningText)
			contents = append(contents, claudeContent)
		}
		if choice.Message.RedactedReasoningContent != "" {
			contents = append(contents, shared.ClaudeMediaMessage{
				Type: "redacted_thinking",
				Data: choice.Message.RedactedReasoningContent,
			})
		}

		for _, toolUse := range choice.Message.ParseToolCalls() {
			claudeContent := shared.ClaudeMediaMessage{Type: "tool_use", Id: toolUse.ID, Name: toolUse.Function.Name}
			var mapParams map[string]interface{}
			if err := jsonx.Unmarshal([]byte(toolUse.Function.Arguments), &mapParams); err == nil {
				claudeContent.Input = mapParams
			} else {
				claudeContent.Input = toolUse.Function.Arguments
			}
			contents = append(contents, claudeContent)
		}

		if choice.Message.IsStringContent() {
			text := choice.Message.StringContent()
			if text != "" {
				claudeContent := shared.ClaudeMediaMessage{Type: "text"}
				claudeContent.SetText(text)
				contents = append(contents, claudeContent)
			}
			continue
		}

		for _, media := range choice.Message.ParseContent() {
			switch media.Type {
			case "text":
				claudeContent := shared.ClaudeMediaMessage{Type: "text"}
				claudeContent.SetText(media.Text)
				contents = append(contents, claudeContent)
			case "image_url":
				imageURL := media.GetImageMedia()
				if imageURL == nil || imageURL.Url == "" {
					continue
				}
				claudeContent := shared.ClaudeMediaMessage{Type: "image", Source: &shared.ClaudeMessageSource{}}
				if strings.HasPrefix(imageURL.Url, "data:") {
					mimeType := imageURL.MimeType
					base64Data := imageURL.Url
					if strings.Contains(imageURL.Url, ",") {
						parts := strings.SplitN(imageURL.Url, ",", 2)
						base64Data = parts[1]
						if mimeType == "" && strings.HasPrefix(parts[0], "data:") {
							mimeType = strings.TrimPrefix(strings.TrimSuffix(parts[0], ";base64"), "data:")
						}
					}
					claudeContent.Source.Type = "base64"
					claudeContent.Source.MediaType = mimeType
					claudeContent.Source.Data = base64Data
				} else {
					claudeContent.Source.Type = "url"
					claudeContent.Source.Url = imageURL.Url
				}
				contents = append(contents, claudeContent)
			}
		}
	}
	claudeResponse.Content = contents
	claudeResponse.StopReason = stopReason
	claudeResponse.Usage = buildClaudeUsageFromOpenAIUsage(&openAIResponse.Usage)

	return claudeResponse
}

func stopReasonOpenAI2Claude(reason string) string {
	return reasonmap.OpenAIFinishReasonToClaudeStopReason(reason)
}

func toJSONString(v interface{}) string {
	b, err := jsonx.Marshal(v)
	if err != nil {
		return "{}"
	}
	return string(b)
}

func GeminiToOpenAIRequest(geminiRequest *shared.GeminiChatRequest, info *relaycommon.RelayInfo) (*shared.GeneralOpenAIRequest, error) {
	openaiRequest := &shared.GeneralOpenAIRequest{
		Model:  info.UpstreamModelName,
		Stream: info.IsStream,
	}
	if geminiRequest.GenerationConfig.ThinkingConfig != nil && geminiRequest.GenerationConfig.ThinkingConfig.IncludeThoughts {
		if effort := geminiThinkingLevelToReasoningEffort(geminiRequest.GenerationConfig.ThinkingConfig.ThinkingLevel); effort != "" {
			openaiRequest.ReasoningEffort = effort
		}
		if geminiRequest.GenerationConfig.ThinkingConfig.ThinkingBudget != nil {
			if openaiRequest.ReasoningEffort == "" {
				openaiRequest.ReasoningEffort = geminiThinkingBudgetToReasoningEffort(*geminiRequest.GenerationConfig.ThinkingConfig.ThinkingBudget)
			}
			reasoningPayload, err := buildOpenAIReasoningPayload(*geminiRequest.GenerationConfig.ThinkingConfig.ThinkingBudget)
			if err != nil {
				return nil, err
			}
			openaiRequest.Reasoning = reasoningPayload
		}
	}

	// 转换 messages
	var messages []shared.Message
	toolCallCounter := 0
	toolCallIDsByName := make(map[string][]string)
	for _, content := range geminiRequest.Contents {
		message := shared.Message{
			Role: convertGeminiRoleToOpenAI(content.Role),
		}

		// 处理 parts
		var mediaContents []shared.MediaContent
		var toolCalls []shared.ToolCallRequest
		var reasoningTexts []string
		reasoningSignature := ""
		for _, part := range content.Parts {
			if signature := part.GetThoughtSignature(); signature != "" && reasoningSignature == "" {
				reasoningSignature = signature
			}
			if part.Thought {
				if part.Text != "" {
					reasoningTexts = append(reasoningTexts, part.Text)
				}
				continue
			}
			if part.Text != "" {
				mediaContent := shared.MediaContent{
					Type: "text",
					Text: part.Text,
				}
				mediaContents = append(mediaContents, mediaContent)
			} else if part.InlineData != nil {
				mediaContent := shared.MediaContent{
					Type: "image_url",
					ImageUrl: &shared.MessageImageUrl{
						Url:      fmt.Sprintf("data:%s;base64,%s", part.InlineData.MimeType, part.InlineData.Data),
						Detail:   "auto",
						MimeType: part.InlineData.MimeType,
					},
				}
				mediaContents = append(mediaContents, mediaContent)
			} else if part.FileData != nil {
				mediaContent := shared.MediaContent{
					Type: "image_url",
					ImageUrl: &shared.MessageImageUrl{
						Url:      part.FileData.FileUri,
						Detail:   "auto",
						MimeType: part.FileData.MimeType,
					},
				}
				mediaContents = append(mediaContents, mediaContent)
			} else if part.FunctionCall != nil {
				// 处理 Gemini 的工具调用
				toolCallCounter++
				toolCallID := fmt.Sprintf("call_%d", toolCallCounter)
				toolCall := shared.ToolCallRequest{
					ID:   toolCallID,
					Type: "function",
					Function: shared.FunctionRequest{
						Name:      part.FunctionCall.FunctionName,
						Arguments: toJSONString(part.FunctionCall.Arguments),
					},
				}
				toolCallIDsByName[part.FunctionCall.FunctionName] = append(toolCallIDsByName[part.FunctionCall.FunctionName], toolCallID)
				toolCalls = append(toolCalls, toolCall)
			} else if part.FunctionResponse != nil {
				// 处理 Gemini 的工具响应，创建单独的 tool 消息
				toolCallID := part.FunctionResponse.GetID()
				if toolCallID == "" {
					queuedIDs := toolCallIDsByName[part.FunctionResponse.Name]
					if len(queuedIDs) == 0 {
						return nil, fmt.Errorf("functionResponse for %s is missing a matching tool call id", part.FunctionResponse.Name)
					}
					toolCallID = queuedIDs[0]
					toolCallIDsByName[part.FunctionResponse.Name] = queuedIDs[1:]
				}
				toolMessage := shared.Message{
					Role:       "tool",
					ToolCallId: toolCallID,
				}
				toolMessage.SetStringContent(toJSONString(part.FunctionResponse.Response))
				messages = append(messages, toolMessage)
			}
		}
		if len(reasoningTexts) > 0 {
			message.SetReasoningContent(strings.Join(reasoningTexts, "\n"))
		}
		if reasoningSignature != "" {
			message.ReasoningSignature = reasoningSignature
		}

		// 设置消息内容
		if len(toolCalls) > 0 {
			// 如果有工具调用，设置工具调用
			message.SetToolCalls(toolCalls)
		} else if len(mediaContents) == 1 && mediaContents[0].Type == "text" {
			// 如果只有一个文本内容，直接设置字符串
			message.Content = mediaContents[0].Text
		} else if len(mediaContents) > 0 {
			// 如果有多个内容或包含媒体，设置为数组
			message.SetMediaContent(mediaContents)
		}

		// 只有当消息有内容或工具调用时才添加
		if len(message.ParseContent()) > 0 || len(message.ToolCalls) > 0 || message.ReasoningContent != nil || message.ReasoningSignature != "" {
			messages = append(messages, message)
		}
	}

	openaiRequest.Messages = messages

	if geminiRequest.GenerationConfig.Temperature != nil {
		openaiRequest.Temperature = geminiRequest.GenerationConfig.Temperature
	}
	if geminiRequest.GenerationConfig.TopP > 0 {
		openaiRequest.TopP = geminiRequest.GenerationConfig.TopP
	}
	if geminiRequest.GenerationConfig.TopK > 0 {
		openaiRequest.TopK = int(geminiRequest.GenerationConfig.TopK)
	}
	if geminiRequest.GenerationConfig.MaxOutputTokens > 0 {
		openaiRequest.MaxTokens = geminiRequest.GenerationConfig.MaxOutputTokens
	}
	// gemini stop sequences 最多 5 个，openai stop 最多 4 个，按实际长度截断到 min(len, 4)
	if seqs := geminiRequest.GenerationConfig.StopSequences; len(seqs) > 0 {
		n := len(seqs)
		if n > 4 {
			n = 4
		}
		openaiRequest.Stop = seqs[:n]
	}
	if geminiRequest.GenerationConfig.CandidateCount > 0 {
		openaiRequest.N = geminiRequest.GenerationConfig.CandidateCount
	}

	// 转换工具调用
	if len(geminiRequest.GetTools()) > 0 {
		var tools []shared.ToolCallRequest
		for _, tool := range geminiRequest.GetTools() {
			if tool.FunctionDeclarations != nil {
				functionDeclarations, err := common.Any2Type[[]shared.FunctionRequest](tool.FunctionDeclarations)
				if err != nil {
					common.SysError(fmt.Sprintf("failed to parse gemini function declarations: %v (type=%T)", err, tool.FunctionDeclarations))
					continue
				}
				for _, function := range functionDeclarations {
					openAITool := shared.ToolCallRequest{
						Type: "function",
						Function: shared.FunctionRequest{
							Name:        function.Name,
							Description: function.Description,
							Parameters:  function.Parameters,
						},
					}
					tools = append(tools, openAITool)
				}
			}
		}
		if len(tools) > 0 {
			openaiRequest.Tools = tools
		}
	}
	openaiRequest.ToolChoice = convertGeminiToolConfigToOpenAI(geminiRequest.ToolConfig)

	// gemini system instructions
	if geminiRequest.SystemInstructions != nil {
		// 将系统指令作为第一条消息插入
		systemMessage := shared.Message{
			Role:    "system",
			Content: extractTextFromGeminiParts(geminiRequest.SystemInstructions.Parts),
		}
		openaiRequest.Messages = append([]shared.Message{systemMessage}, openaiRequest.Messages...)
	}

	return openaiRequest, nil
}

func convertGeminiRoleToOpenAI(geminiRole string) string {
	switch geminiRole {
	case "user":
		return "user"
	case "model":
		return "assistant"
	case "function":
		return "function"
	default:
		return "user"
	}
}

func extractTextFromGeminiParts(parts []shared.GeminiPart) string {
	var texts []string
	for _, part := range parts {
		if part.Text != "" {
			texts = append(texts, part.Text)
		}
	}
	return strings.Join(texts, "\n")
}

// ResponseOpenAI2Gemini 将 OpenAI 响应转换为 Gemini 格式
func ResponseOpenAI2Gemini(openAIResponse *shared.OpenAITextResponse, info *relaycommon.RelayInfo) *shared.GeminiChatResponse {
	geminiResponse := &shared.GeminiChatResponse{
		Candidates:    make([]shared.GeminiChatCandidate, 0, len(openAIResponse.Choices)),
		UsageMetadata: billing.OpenAIUsageToGeminiUsage(openAIResponse.Usage),
	}

	for _, choice := range openAIResponse.Choices {
		candidate := shared.GeminiChatCandidate{
			Index:         int64(choice.Index),
			SafetyRatings: []shared.GeminiChatSafetyRating{},
		}

		// 设置结束原因
		var finishReason string
		switch choice.FinishReason {
		case "stop":
			finishReason = "STOP"
		case "length":
			finishReason = "MAX_TOKENS"
		case "content_filter":
			finishReason = "SAFETY"
		case "tool_calls":
			finishReason = "STOP"
		default:
			finishReason = "STOP"
		}
		candidate.FinishReason = &finishReason

		// 转换消息内容
		content := shared.GeminiChatContent{
			Role:  "model",
			Parts: make([]shared.GeminiPart, 0),
		}

		reasoningContent := choice.Message.GetReasoningContent()
		if reasoningContent != "" || choice.Message.ReasoningSignature != "" {
			thoughtPart := shared.GeminiPart{
				Text:    reasoningContent,
				Thought: true,
			}
			thoughtPart.SetThoughtSignature(choice.Message.ReasoningSignature)
			content.Parts = append(content.Parts, thoughtPart)
		}

		if textContent := choice.Message.StringContent(); textContent != "" {
			content.Parts = append(content.Parts, shared.GeminiPart{Text: textContent})
		}

		for _, toolCall := range choice.Message.ParseToolCalls() {
			content.Parts = append(content.Parts, shared.GeminiPart{
				FunctionCall: &shared.FunctionCall{
					FunctionName: toolCall.Function.Name,
					Arguments:    parseOpenAIFunctionArguments(toolCall.Function.Arguments),
				},
			})
		}

		candidate.Content = content
		geminiResponse.Candidates = append(geminiResponse.Candidates, candidate)
	}

	return geminiResponse
}

func ensureGeminiConvertInfo(info *relaycommon.RelayInfo) *relaycommon.GeminiConvertInfo {
	if info == nil {
		return &relaycommon.GeminiConvertInfo{
			ToolCallArguments: make(map[int]map[int]string),
			ToolCallNames:     make(map[int]map[int]string),
			ToolCallIDs:       make(map[int]map[int]string),
		}
	}
	if info.GeminiConvertInfo == nil {
		info.GeminiConvertInfo = &relaycommon.GeminiConvertInfo{}
	}
	if info.GeminiConvertInfo.ToolCallArguments == nil {
		info.GeminiConvertInfo.ToolCallArguments = make(map[int]map[int]string)
	}
	if info.GeminiConvertInfo.ToolCallNames == nil {
		info.GeminiConvertInfo.ToolCallNames = make(map[int]map[int]string)
	}
	if info.GeminiConvertInfo.ToolCallIDs == nil {
		info.GeminiConvertInfo.ToolCallIDs = make(map[int]map[int]string)
	}
	return info.GeminiConvertInfo
}

func ensureGeminiChoiceToolCallState[K any](state map[int]map[int]K, choiceIndex int) map[int]K {
	choiceState := state[choiceIndex]
	if choiceState == nil {
		choiceState = make(map[int]K)
		state[choiceIndex] = choiceState
	}
	return choiceState
}

// StreamResponseOpenAI2Gemini 将 OpenAI 流式响应转换为 Gemini 格式
func StreamResponseOpenAI2Gemini(openAIResponse *shared.ChatCompletionsStreamResponse, info *relaycommon.RelayInfo) *shared.GeminiChatResponse {
	geminiResponse := &shared.GeminiChatResponse{
		Candidates: make([]shared.GeminiChatCandidate, 0, len(openAIResponse.Choices)),
		UsageMetadata: shared.GeminiUsageMetadata{
			PromptTokenCount:     info.GetEstimatePromptTokens(),
			CandidatesTokenCount: 0, // 流式响应中可能没有完整的 usage 信息
			TotalTokenCount:      info.GetEstimatePromptTokens(),
		},
	}

	if openAIResponse.Usage != nil {
		if usageMetadata := billing.OpenAIUsageToGeminiUsage(*openAIResponse.Usage); billing.HasGeminiUsageMetadata(usageMetadata) {
			geminiResponse.UsageMetadata = usageMetadata
		}
	}

	toolState := ensureGeminiConvertInfo(info)

	for _, choice := range openAIResponse.Choices {
		candidate := shared.GeminiChatCandidate{
			Index:         int64(choice.Index),
			SafetyRatings: []shared.GeminiChatSafetyRating{},
		}

		// 设置结束原因
		if choice.FinishReason != nil {
			var finishReason string
			switch *choice.FinishReason {
			case "stop":
				finishReason = "STOP"
			case "length":
				finishReason = "MAX_TOKENS"
			case "content_filter":
				finishReason = "SAFETY"
			case "tool_calls":
				finishReason = "STOP"
			default:
				finishReason = "STOP"
			}
			candidate.FinishReason = &finishReason
		}

		// 转换消息内容
		content := shared.GeminiChatContent{
			Role:  "model",
			Parts: make([]shared.GeminiPart, 0),
		}

		reasoningContent := choice.Delta.GetReasoningContent()
		reasoningSignature := ""
		if choice.Delta.ReasoningSignature != nil {
			reasoningSignature = *choice.Delta.ReasoningSignature
		}
		if reasoningContent != "" || reasoningSignature != "" {
			thoughtPart := shared.GeminiPart{
				Text:    reasoningContent,
				Thought: true,
			}
			thoughtPart.SetThoughtSignature(reasoningSignature)
			content.Parts = append(content.Parts, thoughtPart)
		}

		if textContent := choice.Delta.GetContentString(); textContent != "" {
			content.Parts = append(content.Parts, shared.GeminiPart{Text: textContent})
		}

		argsState := ensureGeminiChoiceToolCallState(toolState.ToolCallArguments, choice.Index)
		nameState := ensureGeminiChoiceToolCallState(toolState.ToolCallNames, choice.Index)
		idState := ensureGeminiChoiceToolCallState(toolState.ToolCallIDs, choice.Index)
		for toolOffset, toolCall := range choice.Delta.ToolCalls {
			toolIndex := toolOffset
			if toolCall.Index != nil {
				toolIndex = *toolCall.Index
			}
			if toolCall.Function.Name != "" {
				nameState[toolIndex] = toolCall.Function.Name
			}
			if toolCall.ID != "" {
				idState[toolIndex] = toolCall.ID
			}
			if toolCall.Function.Arguments != "" {
				argsState[toolIndex] += toolCall.Function.Arguments
			}

			aggregatedArgs := strings.TrimSpace(argsState[toolIndex])
			shouldFlush := aggregatedArgs == "" || jsonx.Valid([]byte(aggregatedArgs))
			if choice.FinishReason != nil && *choice.FinishReason == "tool_calls" {
				shouldFlush = true
			}
			if !shouldFlush {
				continue
			}

			functionName := nameState[toolIndex]
			if functionName == "" {
				functionName = toolCall.Function.Name
			}
			if functionName == "" {
				continue
			}

			content.Parts = append(content.Parts, shared.GeminiPart{
				FunctionCall: &shared.FunctionCall{
					FunctionName: functionName,
					Arguments:    parseOpenAIFunctionArguments(aggregatedArgs),
				},
			})
			delete(argsState, toolIndex)
		}

		if len(content.Parts) == 0 && candidate.FinishReason == nil {
			continue
		}
		candidate.Content = content
		geminiResponse.Candidates = append(geminiResponse.Candidates, candidate)
	}

	if len(geminiResponse.Candidates) == 0 && !billing.HasGeminiUsageMetadata(geminiResponse.UsageMetadata) {
		return nil
	}
	return geminiResponse
}

// parseOpenAIFunctionArguments keeps Gemini function-call args structured when the OpenAI payload is valid JSON.
func parseOpenAIFunctionArguments(arguments string) any {
	if strings.TrimSpace(arguments) == "" {
		return map[string]interface{}{}
	}

	var args any
	if err := jsonx.Unmarshal([]byte(arguments), &args); err == nil {
		return args
	}
	return map[string]interface{}{"arguments": arguments}
}
