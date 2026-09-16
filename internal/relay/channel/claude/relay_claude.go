package claude

import (
	"encoding/base64"
	"fmt"
	"net/http"
	"path/filepath"
	"strings"

	"github.com/NookMux/NookMux/internal/common"
	"github.com/NookMux/NookMux/internal/config/model"
	"github.com/NookMux/NookMux/internal/domain/shared"
	"github.com/NookMux/NookMux/internal/infra/log"
	"github.com/NookMux/NookMux/internal/relay/channel/openrouter"
	relaycommon "github.com/NookMux/NookMux/internal/relay/common"
	"github.com/NookMux/NookMux/internal/relay/helper"
	"github.com/NookMux/NookMux/internal/relay/reasonmap"
	"github.com/NookMux/NookMux/internal/relay/wire/convert"
	"github.com/NookMux/NookMux/internal/relay/wire/stream"

	"github.com/NookMux/NookMux/pkg/jsonx"

	billing "github.com/NookMux/NookMux/internal/domain/billing"
	channelconstant "github.com/NookMux/NookMux/internal/domain/channel/constant"
	"github.com/NookMux/NookMux/internal/httpapi"
	media "github.com/NookMux/NookMux/internal/infra/media"
	relayconstant "github.com/NookMux/NookMux/internal/relay/constant"
	"github.com/gin-gonic/gin"
	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
)

const (
	WebSearchMaxUsesLow    = 1
	WebSearchMaxUsesMedium = 5
	WebSearchMaxUsesHigh   = 10
)

var claudeReasoningEffortSuffixes = []string{"-max", "-xhigh", "-high", "-medium", "-low", "-none"}

func stopReasonClaude2OpenAI(reason string) string {
	return reasonmap.ClaudeStopReasonToOpenAIFinishReason(reason)
}

func maybeMarkClaudeRefusal(c *gin.Context, stopReason string) {
	if c == nil {
		return
	}
	if strings.EqualFold(stopReason, "refusal") {
		httpapi.SetContextKey(c, common.ContextKeyAdminRejectReason, "claude_stop_reason=refusal")
	}
}

func createClaudeFileSource(file *shared.MessageFile) *shared.FileSource {
	if file == nil || file.FileData == "" {
		return nil
	}
	if strings.HasPrefix(file.FileData, "http://") || strings.HasPrefix(file.FileData, "https://") {
		return shared.NewURLFileSource(file.FileData)
	}
	mimeType := ""
	if ext := strings.TrimPrefix(strings.ToLower(filepath.Ext(file.FileName)), "."); ext != "" {
		if detected := media.GetMimeTypeByExtension(ext); detected != "application/octet-stream" {
			mimeType = detected
		}
	}
	return shared.NewBase64FileSource(file.FileData, mimeType)
}

func buildClaudeFileMessage(c *gin.Context, file *shared.MessageFile) (*shared.ClaudeMediaMessage, error) {
	source := createClaudeFileSource(file)
	if source == nil {
		return nil, nil
	}
	base64Data, mimeType, err := media.GetBase64Data(c, source, "formatting document for Claude")
	if err != nil {
		return nil, fmt.Errorf("get file data failed: %w", err)
	}
	switch strings.ToLower(mimeType) {
	case "application/pdf":
		return &shared.ClaudeMediaMessage{
			Type: "document",
			Source: &shared.ClaudeMessageSource{
				Type:      "base64",
				MediaType: mimeType,
				Data:      base64Data,
			},
		}, nil
	case "text/plain":
		decodedData, err := base64.StdEncoding.DecodeString(base64Data)
		if err != nil {
			return nil, fmt.Errorf("decode text file data failed: %w", err)
		}
		return &shared.ClaudeMediaMessage{
			Type: "text",
			Text: common.GetPointer(string(decodedData)),
		}, nil
	default:
		msg := fmt.Sprintf("claude: skip unsupported file content, filename=%q, mime=%q", file.FileName, mimeType)
		if c != nil {
			log.LogInfo(c, msg)
		} else {
			common.SysLog(msg)
		}
		return nil, nil
	}
}

func applyOpenAIReasoningToClaudeRequest(info *relaycommon.RelayInfo, textRequest *shared.GeneralOpenAIRequest, claudeRequest *shared.ClaudeRequest) error {
	if textRequest == nil || claudeRequest == nil {
		return nil
	}

	effort := strings.ToLower(strings.TrimSpace(textRequest.ReasoningEffort))
	if suffixEffort, model := parseClaudeReasoningEffortFromModelSuffix(claudeRequest.Model); suffixEffort != "" {
		effort = suffixEffort
		claudeRequest.Model = model
		textRequest.Model = model
		if info != nil && info.ChannelMeta != nil {
			info.UpstreamModelName = model
		}
	}

	reasoningBudget := 0
	if textRequest.Reasoning != nil {
		var reasoning openrouter.RequestReasoning
		if err := jsonx.Unmarshal(textRequest.Reasoning, &reasoning); err != nil {
			return err
		}
		if strings.TrimSpace(reasoning.Effort) != "" {
			effort = strings.ToLower(strings.TrimSpace(reasoning.Effort))
		}
		reasoningBudget = reasoning.MaxTokens
	}

	if effort == "" && reasoningBudget <= 0 {
		return nil
	}

	if effort == "none" {
		if info != nil {
			info.ReasoningEffort = effort
		}
		claudeRequest.Thinking = &shared.Thinking{Type: "disabled"}
		claudeRequest.OutputConfig = nil
		return nil
	}

	if effort != "" && !isSupportedClaudeReasoningEffort(effort) {
		return fmt.Errorf("unsupported reasoning_effort for Claude request: %s", effort)
	}
	if info != nil && effort != "" {
		info.ReasoningEffort = effort
	}

	claudeEffort := normalizeClaudeOutputEffort(effort)
	if claudeEffort != "" && shouldUseClaudeOutputConfigEffort(info, claudeRequest.Model) {
		if shouldUseClaudeAdaptiveThinking(info, claudeRequest.Model) {
			claudeRequest.Thinking = &shared.Thinking{Type: "adaptive"}
			if isClaudeOpus47Model(claudeRequest.Model) {
				claudeRequest.Thinking.Display = "summarized"
				claudeRequest.Temperature = nil
				claudeRequest.TopP = 0
				claudeRequest.TopK = 0
			}
		} else {
			claudeRequest.Thinking = nil
		}
		outputConfig, err := jsonx.Marshal(shared.ClaudeOutputConfig{Effort: claudeEffort})
		if err != nil {
			return fmt.Errorf("failed to marshal claude output_config: %w", err)
		}
		claudeRequest.OutputConfig = outputConfig
		return nil
	}

	if reasoningBudget <= 0 {
		reasoningBudget = claudeThinkingBudgetForEffort(effort)
	}
	if reasoningBudget > 0 {
		claudeRequest.Thinking = &shared.Thinking{
			Type:         "enabled",
			BudgetTokens: &reasoningBudget,
		}
	}
	return nil
}

func parseClaudeReasoningEffortFromModelSuffix(model string) (string, string) {
	if strings.HasSuffix(model, "-thinking") && isClaudeOpus47Model(model) {
		return "high", strings.TrimSuffix(model, "-thinking")
	}
	for _, suffix := range claudeReasoningEffortSuffixes {
		if strings.HasSuffix(model, suffix) {
			return strings.TrimPrefix(suffix, "-"), strings.TrimSuffix(model, suffix)
		}
	}
	return "", model
}

func normalizeClaudeOutputEffort(effort string) string {
	switch strings.ToLower(strings.TrimSpace(effort)) {
	case "low", "medium", "high", "max":
		return strings.ToLower(strings.TrimSpace(effort))
	case "xhigh":
		return "max"
	default:
		return ""
	}
}

func isSupportedClaudeReasoningEffort(effort string) bool {
	switch strings.ToLower(strings.TrimSpace(effort)) {
	case "low", "medium", "high", "xhigh", "max":
		return true
	default:
		return false
	}
}

func shouldUseClaudeOutputConfigEffort(info *relaycommon.RelayInfo, model string) bool {
	if info != nil && info.ChannelMeta != nil && info.ChannelType == channelconstant.ChannelTypeDeepSeek {
		return true
	}

	model = strings.ToLower(model)
	return isClaudeAdaptiveOutputModel(model)
}

func shouldUseClaudeAdaptiveThinking(info *relaycommon.RelayInfo, model string) bool {
	if info != nil && info.ChannelMeta != nil && info.ChannelType == channelconstant.ChannelTypeDeepSeek {
		return false
	}

	model = strings.ToLower(model)
	return isClaudeAdaptiveOutputModel(model)
}

func isClaudeAdaptiveOutputModel(model string) bool {
	return strings.Contains(model, "claude-opus-4-6") || strings.Contains(model, "claude-opus-4-7") || strings.Contains(model, "claude-opus-4-8")
}

func isClaudeOpus47Model(model string) bool {
	return strings.Contains(strings.ToLower(model), "claude-opus-4-7")
}

func claudeThinkingBudgetForEffort(effort string) int {
	switch strings.ToLower(strings.TrimSpace(effort)) {
	case "low":
		return 1280
	case "medium":
		return 2048
	case "high":
		return 4096
	case "xhigh", "max":
		return 8192
	default:
		return 0
	}
}

func RequestOpenAI2ClaudeMessage(c *gin.Context, info *relaycommon.RelayInfo, textRequest shared.GeneralOpenAIRequest) (*shared.ClaudeRequest, error) {
	claudeTools := make([]any, 0, len(textRequest.Tools))

	for _, tool := range textRequest.Tools {
		if params, ok := tool.Function.Parameters.(map[string]any); ok {
			claudeTool := shared.Tool{
				Name:        tool.Function.Name,
				Description: tool.Function.Description,
			}
			claudeTool.InputSchema = make(map[string]interface{})
			if params["type"] != nil {
				claudeTool.InputSchema["type"] = params["type"].(string)
			}
			claudeTool.InputSchema["properties"] = params["properties"]
			claudeTool.InputSchema["required"] = params["required"]
			for s, a := range params {
				if s == "type" || s == "properties" || s == "required" {
					continue
				}
				claudeTool.InputSchema[s] = a
			}
			claudeTools = append(claudeTools, &claudeTool)
		}
	}

	// Web search tool
	// https://docs.anthropic.com/en/docs/agents-and-tools/tool-use/web-search-tool
	if textRequest.WebSearchOptions != nil {
		webSearchTool := shared.ClaudeWebSearchTool{
			Type: "web_search_20250305",
			Name: "web_search",
		}

		// 处理 user_location
		if textRequest.WebSearchOptions.UserLocation != nil {
			anthropicUserLocation := &shared.ClaudeWebSearchUserLocation{
				Type: "approximate", // 固定为 "approximate"
			}

			// 解析 UserLocation JSON
			var userLocationMap map[string]interface{}
			if err := jsonx.Unmarshal(textRequest.WebSearchOptions.UserLocation, &userLocationMap); err == nil {
				// 检查是否有 approximate 字段
				if approximateData, ok := userLocationMap["approximate"].(map[string]interface{}); ok {
					if timezone, ok := approximateData["timezone"].(string); ok && timezone != "" {
						anthropicUserLocation.Timezone = timezone
					}
					if country, ok := approximateData["country"].(string); ok && country != "" {
						anthropicUserLocation.Country = country
					}
					if region, ok := approximateData["region"].(string); ok && region != "" {
						anthropicUserLocation.Region = region
					}
					if city, ok := approximateData["city"].(string); ok && city != "" {
						anthropicUserLocation.City = city
					}
				}
			}

			webSearchTool.UserLocation = anthropicUserLocation
		}

		// 处理 search_context_size 转换为 max_uses
		if textRequest.WebSearchOptions.SearchContextSize != "" {
			switch textRequest.WebSearchOptions.SearchContextSize {
			case "low":
				webSearchTool.MaxUses = WebSearchMaxUsesLow
			case "medium":
				webSearchTool.MaxUses = WebSearchMaxUsesMedium
			case "high":
				webSearchTool.MaxUses = WebSearchMaxUsesHigh
			}
		}

		claudeTools = append(claudeTools, &webSearchTool)
	}

	claudeRequest := shared.ClaudeRequest{
		Model:         textRequest.Model,
		MaxTokens:     textRequest.GetMaxTokens(),
		StopSequences: nil,
		Temperature:   textRequest.Temperature,
		TopP:          textRequest.TopP,
		TopK:          textRequest.TopK,
		Stream:        textRequest.Stream,
		Tools:         claudeTools,
	}

	// 处理 tool_choice 和 parallel_tool_calls
	if textRequest.ToolChoice != nil || textRequest.ParallelTooCalls != nil {
		claudeToolChoice := mapToolChoice(textRequest.ToolChoice, textRequest.ParallelTooCalls)
		if claudeToolChoice != nil {
			claudeRequest.ToolChoice = claudeToolChoice
		}
	}

	if claudeRequest.MaxTokens == 0 {
		claudeRequest.MaxTokens = uint(model.GetClaudeSettings().GetDefaultMaxTokens(textRequest.Model))
	}

	if err := applyOpenAIReasoningToClaudeRequest(info, &textRequest, &claudeRequest); err != nil {
		return nil, err
	}

	if textRequest.Stop != nil {
		// stop maybe string/array string, convert to array string
		switch textRequest.Stop.(type) {
		case string:
			claudeRequest.StopSequences = []string{textRequest.Stop.(string)}
		case []interface{}:
			stopSequences := make([]string, 0)
			for _, stop := range textRequest.Stop.([]interface{}) {
				stopSequences = append(stopSequences, stop.(string))
			}
			claudeRequest.StopSequences = stopSequences
		}
	}
	formatMessages := make([]shared.Message, 0)
	lastMessage := shared.Message{
		Role: "tool",
	}
	for i, message := range textRequest.Messages {
		if message.Role == "" {
			textRequest.Messages[i].Role = "user"
		}
		fmtMessage := shared.Message{
			Role:                     message.Role,
			Content:                  message.Content,
			ReasoningContent:         message.ReasoningContent,
			Reasoning:                message.Reasoning,
			ReasoningSignature:       message.ReasoningSignature,
			RedactedReasoningContent: message.RedactedReasoningContent,
		}
		if message.Role == "tool" {
			fmtMessage.ToolCallId = message.ToolCallId
		}
		if message.Role == "assistant" && message.ToolCalls != nil {
			fmtMessage.ToolCalls = message.ToolCalls
		}
		if lastMessage.Role == message.Role && lastMessage.Role != "tool" {
			if lastMessage.IsStringContent() && message.IsStringContent() {
				fmtMessage.SetStringContent(strings.Trim(fmt.Sprintf("%s %s", lastMessage.StringContent(), message.StringContent()), "\""))
				// delete last message
				formatMessages = formatMessages[:len(formatMessages)-1]
			}
		}
		formatMessages = append(formatMessages, fmtMessage)
		lastMessage = fmtMessage
	}

	claudeMessages := make([]shared.ClaudeMessage, 0)
	isFirstMessage := true
	// 初始化system消息数组，用于累积多个system消息
	var systemMessages []shared.ClaudeMediaMessage

	for _, message := range formatMessages {
		if message.Role == "system" {
			// 根据Claude API规范，system字段使用数组格式更有通用性
			if message.IsStringContent() {
				systemMessages = append(systemMessages, shared.ClaudeMediaMessage{
					Type: "text",
					Text: common.GetPointer[string](message.StringContent()),
				})
			} else {
				// 支持复合内容的system消息（虽然不常见，但需要考虑完整性）
				for _, ctx := range message.ParseContent() {
					if ctx.Type == "text" {
						systemMessages = append(systemMessages, shared.ClaudeMediaMessage{
							Type: "text",
							Text: common.GetPointer[string](ctx.Text),
						})
					}
					// 未来可以在这里扩展对图片等其他类型的支持
				}
			}
		} else {
			if isFirstMessage {
				isFirstMessage = false
				if message.Role != "user" {
					// fix: first message is assistant, add user message
					claudeMessage := shared.ClaudeMessage{
						Role: "user",
						Content: []shared.ClaudeMediaMessage{
							{
								Type: "text",
								Text: common.GetPointer[string]("..."),
							},
						},
					}
					claudeMessages = append(claudeMessages, claudeMessage)
				}
			}
			claudeMessage := shared.ClaudeMessage{
				Role: message.Role,
			}
			if message.Role == "tool" {
				if len(claudeMessages) > 0 && claudeMessages[len(claudeMessages)-1].Role == "user" {
					lastMessage := claudeMessages[len(claudeMessages)-1]
					if content, ok := lastMessage.Content.(string); ok {
						lastMessage.Content = []shared.ClaudeMediaMessage{
							{
								Type: "text",
								Text: common.GetPointer[string](content),
							},
						}
					}
					lastMessage.Content = append(lastMessage.Content.([]shared.ClaudeMediaMessage), shared.ClaudeMediaMessage{
						Type:      "tool_result",
						ToolUseId: message.ToolCallId,
						Content:   message.Content,
						IsError:   message.ToolCallIsError,
					})
					claudeMessages[len(claudeMessages)-1] = lastMessage
					continue
				} else {
					claudeMessage.Role = "user"
					claudeMessage.Content = []shared.ClaudeMediaMessage{
						{
							Type:      "tool_result",
							ToolUseId: message.ToolCallId,
							Content:   message.Content,
							IsError:   message.ToolCallIsError,
						},
					}
				}
			} else if message.IsStringContent() && message.ToolCalls == nil && message.ReasoningContent == nil && message.ReasoningSignature == "" && message.RedactedReasoningContent == "" {
				stringContent := message.StringContent()
				// AWS Bedrock 等上游拒绝空字符串内容（返回 400），用占位符兜底
				if strings.TrimSpace(stringContent) == "" {
					stringContent = "..."
				}
				claudeMessage.Content = stringContent
			} else {
				claudeMediaMessages := make([]shared.ClaudeMediaMessage, 0)
				if message.Role == "assistant" {
					if message.ReasoningContent != nil || message.ReasoningSignature != "" {
						claudeThinking := shared.ClaudeMediaMessage{Type: "thinking", Signature: message.ReasoningSignature}
						reasoningText := message.GetReasoningContent()
						claudeThinking.Thinking = common.GetPointer[string](reasoningText)
						claudeMediaMessages = append(claudeMediaMessages, claudeThinking)
					}
					if message.RedactedReasoningContent != "" {
						claudeMediaMessages = append(claudeMediaMessages, shared.ClaudeMediaMessage{
							Type: "redacted_thinking",
							Data: message.RedactedReasoningContent,
						})
					}
				}
				for _, mediaMessage := range message.ParseContent() {
					switch mediaMessage.Type {
					case "text":
						// AWS Bedrock 等上游拒绝空文本块（返回 400），跳过空 text 部分
						if strings.TrimSpace(mediaMessage.Text) == "" {
							continue
						}
						claudeMediaMessages = append(claudeMediaMessages, shared.ClaudeMediaMessage{
							Type: "text",
							Text: common.GetPointer[string](mediaMessage.Text),
						})
					case shared.ContentTypeImageURL:
						claudeMediaMessage := shared.ClaudeMediaMessage{
							Type: "image",
							Source: &shared.ClaudeMessageSource{
								Type: "base64",
							},
						}
						imageUrl := mediaMessage.GetImageMedia()
						if imageUrl == nil {
							continue
						}
						// 使用统一的文件服务获取图片数据
						var source *shared.FileSource
						if strings.HasPrefix(imageUrl.Url, "http") {
							source = shared.NewURLFileSource(imageUrl.Url)
						} else {
							source = shared.NewBase64FileSource(imageUrl.Url, "")
						}
						base64Data, mimeType, err := media.GetBase64Data(c, source, "formatting image for Claude")
						if err != nil {
							return nil, fmt.Errorf("get file data failed: %s", err.Error())
						}
						claudeMediaMessage.Source.MediaType = mimeType
						claudeMediaMessage.Source.Data = base64Data
						claudeMediaMessages = append(claudeMediaMessages, claudeMediaMessage)
					case shared.ContentTypeFile:
						claudeFileMessage, err := buildClaudeFileMessage(c, mediaMessage.GetFile())
						if err != nil {
							return nil, err
						}
						if claudeFileMessage != nil {
							claudeMediaMessages = append(claudeMediaMessages, *claudeFileMessage)
						}
					default:
						continue
					}
				}
				if message.ToolCalls != nil {
					for _, toolCall := range message.ParseToolCalls() {
						inputObj := make(map[string]any)
						if args := toolCall.Function.Arguments; args != "" {
							if err := jsonx.Unmarshal([]byte(args), &inputObj); err != nil {
								common.SysLog("tool call function arguments is not a map[string]any: " + fmt.Sprintf("%v", toolCall.Function.Arguments))
							}
						}
						claudeMediaMessages = append(claudeMediaMessages, shared.ClaudeMediaMessage{
							Type:  "tool_use",
							Id:    toolCall.ID,
							Name:  toolCall.Function.Name,
							Input: inputObj,
						})
					}
				}
				// AWS Bedrock 等上游拒绝空 content 数组（返回 400），用占位符兜底
				if len(claudeMediaMessages) == 0 {
					claudeMessage.Content = []shared.ClaudeMediaMessage{{Type: "text", Text: common.GetPointer[string]("...")}}
				} else {
					claudeMessage.Content = claudeMediaMessages
				}
			}
			claudeMessages = append(claudeMessages, claudeMessage)
		}
	}

	// 设置累积的system消息
	if len(systemMessages) > 0 {
		claudeRequest.System = systemMessages
	}

	claudeRequest.Prompt = ""
	claudeRequest.Messages = claudeMessages
	return &claudeRequest, nil
}

func StreamResponseClaude2OpenAI(claudeResponse *shared.ClaudeResponse) *shared.ChatCompletionsStreamResponse {
	var response shared.ChatCompletionsStreamResponse
	response.Object = "chat.completion.chunk"
	response.Model = claudeResponse.Model
	response.Choices = make([]shared.ChatCompletionsStreamResponseChoice, 0)
	tools := make([]shared.ToolCallResponse, 0)
	fcIdx := 0
	if claudeResponse.Index != nil {
		fcIdx = *claudeResponse.Index
	}
	var choice shared.ChatCompletionsStreamResponseChoice
	if claudeResponse.Type == "message_start" {
		if claudeResponse.Message != nil {
			response.Id = claudeResponse.Message.Id
			response.Model = claudeResponse.Message.Model
		}
		//claudeUsage = &claudeResponse.Message.Usage
		choice.Delta.SetContentString("")
		choice.Delta.Role = "assistant"
	} else if claudeResponse.Type == "content_block_start" {
		if claudeResponse.ContentBlock != nil {
			// 如果是文本块，尽可能发送首段文本（若存在）
			if claudeResponse.ContentBlock.Type == "text" && claudeResponse.ContentBlock.Text != nil {
				choice.Delta.SetContentString(*claudeResponse.ContentBlock.Text)
			}
			if claudeResponse.ContentBlock.Type == "redacted_thinking" && claudeResponse.ContentBlock.Data != "" {
				choice.Delta.RedactedReasoningContent = common.GetPointer[string](claudeResponse.ContentBlock.Data)
			}
			if claudeResponse.ContentBlock.Type == "tool_use" {
				tools = append(tools, shared.ToolCallResponse{
					Index: common.GetPointer(fcIdx),
					ID:    claudeResponse.ContentBlock.Id,
					Type:  "function",
					Function: shared.FunctionResponse{
						Name:      claudeResponse.ContentBlock.Name,
						Arguments: "",
					},
				})
			}
		} else {
			return nil
		}
	} else if claudeResponse.Type == "content_block_delta" {
		if claudeResponse.Delta != nil {
			choice.Delta.Content = claudeResponse.Delta.Text
			switch claudeResponse.Delta.Type {
			case "input_json_delta":
				// 防御空 partial_json（部分上游或断流场景 delta 携带 nil 字段），避免解引用 nil 崩溃
				arguments := ""
				if claudeResponse.Delta.PartialJson != nil {
					arguments = *claudeResponse.Delta.PartialJson
				}
				tools = append(tools, shared.ToolCallResponse{
					Type:  "function",
					Index: common.GetPointer(fcIdx),
					Function: shared.FunctionResponse{
						Arguments: arguments,
					},
				})
			case "signature_delta":
				if claudeResponse.Delta.Signature != "" {
					choice.Delta.ReasoningSignature = common.GetPointer[string](claudeResponse.Delta.Signature)
				}
			case "thinking_delta":
				choice.Delta.ReasoningContent = claudeResponse.Delta.Thinking
			}
		}
	} else if claudeResponse.Type == "message_delta" {
		if claudeResponse.Delta != nil && claudeResponse.Delta.StopReason != nil {
			finishReason := stopReasonClaude2OpenAI(*claudeResponse.Delta.StopReason)
			if finishReason != "null" {
				choice.FinishReason = &finishReason
			}
		}
		//claudeUsage = &claudeResponse.Usage
	} else if claudeResponse.Type == "message_stop" {
		return nil
	} else {
		return nil
	}
	if len(tools) > 0 {
		choice.Delta.Content = nil // compatible with other OpenAI derivative applications, like LobeOpenAICompatibleFactory ...
		choice.Delta.ToolCalls = tools
	}
	response.Choices = append(response.Choices, choice)

	return &response
}

func ResponseClaude2OpenAI(claudeResponse *shared.ClaudeResponse) *shared.OpenAITextResponse {
	choices := make([]shared.OpenAITextResponseChoice, 0)
	fullTextResponse := shared.OpenAITextResponse{
		Id:      fmt.Sprintf("chatcmpl-%s", common.GetUUID()),
		Object:  "chat.completion",
		Created: common.GetTimestamp(),
	}
	tools := make([]shared.ToolCallResponse, 0)
	var responseText strings.Builder
	var thinkingContent strings.Builder
	var thinkingSignature string
	var redactedReasoning strings.Builder

	fullTextResponse.Id = claudeResponse.Id
	for _, message := range claudeResponse.Content {
		switch message.Type {
		case "tool_use":
			args, _ := jsonx.Marshal(message.Input)
			tools = append(tools, shared.ToolCallResponse{
				ID:   message.Id,
				Type: "function", // compatible with other OpenAI derivative applications
				Function: shared.FunctionResponse{
					Name:      message.Name,
					Arguments: string(args),
				},
			})
		case "thinking":
			if message.Thinking != nil {
				thinkingContent.WriteString(*message.Thinking)
			}
			if message.Signature != "" {
				thinkingSignature = message.Signature
			}
		case "redacted_thinking":
			redactedReasoning.WriteString(message.Data)
		case "text":
			responseText.WriteString(message.GetText())
		}
	}
	choice := shared.OpenAITextResponseChoice{
		Index: 0,
		Message: shared.Message{
			Role: "assistant",
		},
		FinishReason: stopReasonClaude2OpenAI(claudeResponse.StopReason),
	}
	choice.SetStringContent(responseText.String())
	if len(tools) > 0 {
		choice.Message.SetToolCalls(tools)
	}
	choice.Message.SetReasoningContent(thinkingContent.String())
	choice.Message.ReasoningSignature = thinkingSignature
	choice.Message.RedactedReasoningContent = redactedReasoning.String()
	fullTextResponse.Model = claudeResponse.Model
	choices = append(choices, choice)
	fullTextResponse.Choices = choices
	if usage := shared.ClaudeUsageToOpenAIUsage(claudeResponse.Usage); usage != nil {
		fullTextResponse.Usage = *usage
	}
	return &fullTextResponse
}

type ClaudeResponseInfo struct {
	ResponseId                string
	Created                   int64
	Model                     string
	ResponseText              strings.Builder
	Usage                     *shared.Usage
	Done                      bool
	ResponsesStreamConverter  stream.OpenAIWireStreamConverter
	ResponsesCompletedEmitted bool
}

func mergeClaudeUsageIntoOpenAIUsage(current *shared.Usage, claudeUsage *shared.ClaudeUsage) *shared.Usage {
	if current == nil {
		current = &shared.Usage{}
	}
	if claudeUsage == nil {
		current.TotalTokens = current.PromptTokens + current.CompletionTokens
		return current
	}

	cacheReadTokens := current.PromptTokensDetails.CachedTokens
	if claudeUsage.CacheReadInputTokens > 0 {
		cacheReadTokens = claudeUsage.CacheReadInputTokens
	}

	cacheCreationTokens := current.PromptTokensDetails.CachedCreationTokens
	if incomingCacheCreation := claudeUsage.GetCacheCreationTotalTokens(); incomingCacheCreation > 0 {
		cacheCreationTokens = incomingCacheCreation
	}

	cacheCreation5m := current.ClaudeCacheCreation5mTokens
	cacheCreation1h := current.ClaudeCacheCreation1hTokens
	if incoming5m := claudeUsage.GetCacheCreation5mTokens(); incoming5m > 0 {
		cacheCreation5m = incoming5m
	}
	if incoming1h := claudeUsage.GetCacheCreation1hTokens(); incoming1h > 0 {
		cacheCreation1h = incoming1h
	}
	cacheCreation5m, cacheCreation1h = helper.NormalizeCacheCreationSplit(cacheCreationTokens, cacheCreation5m, cacheCreation1h)

	if claudeUsage.InputTokens > 0 {
		current.PromptTokens = claudeUsage.InputTokens + cacheReadTokens + cacheCreationTokens
	} else if current.PromptTokens == 0 && (cacheReadTokens > 0 || cacheCreationTokens > 0) {
		current.PromptTokens = cacheReadTokens + cacheCreationTokens
	}
	current.InputTokens = current.PromptTokens
	current.PromptCacheHitTokens = cacheReadTokens
	current.PromptTokensDetails.CachedTokens = cacheReadTokens
	current.PromptTokensDetails.CachedCreationTokens = cacheCreationTokens
	if current.InputTokensDetails == nil {
		current.InputTokensDetails = &shared.InputTokenDetails{}
	}
	current.InputTokensDetails.CachedTokens = cacheReadTokens
	current.InputTokensDetails.CachedCreationTokens = cacheCreationTokens
	current.ClaudeCacheCreation5mTokens = cacheCreation5m
	current.ClaudeCacheCreation1hTokens = cacheCreation1h

	if claudeUsage.OutputTokens > 0 {
		current.CompletionTokens = claudeUsage.OutputTokens
		current.OutputTokens = claudeUsage.OutputTokens
	}
	current.TotalTokens = current.PromptTokens + current.CompletionTokens
	return current
}

func buildMessageDeltaPatchUsage(claudeResponse *shared.ClaudeResponse, claudeInfo *ClaudeResponseInfo) *shared.ClaudeUsage {
	usage := &shared.ClaudeUsage{}
	if claudeResponse != nil && claudeResponse.Usage != nil {
		*usage = *claudeResponse.Usage
	}

	if claudeInfo == nil || claudeInfo.Usage == nil {
		return usage
	}
	localUsage := shared.OpenAIUsageToClaudeUsage(claudeInfo.Usage)
	if localUsage == nil {
		return usage
	}

	if usage.InputTokens == 0 && localUsage.InputTokens > 0 {
		usage.InputTokens = localUsage.InputTokens
	}
	if usage.CacheReadInputTokens == 0 && localUsage.CacheReadInputTokens > 0 {
		usage.CacheReadInputTokens = localUsage.CacheReadInputTokens
	}
	if usage.CacheCreationInputTokens == 0 && localUsage.CacheCreationInputTokens > 0 {
		usage.CacheCreationInputTokens = localUsage.CacheCreationInputTokens
	}
	cacheCreation5m := 0
	cacheCreation1h := 0
	if usage.CacheCreation != nil {
		cacheCreation5m = usage.CacheCreation.Ephemeral5mInputTokens
		cacheCreation1h = usage.CacheCreation.Ephemeral1hInputTokens
	} else if localUsage.CacheCreation != nil {
		cacheCreation5m = localUsage.CacheCreation.Ephemeral5mInputTokens
		cacheCreation1h = localUsage.CacheCreation.Ephemeral1hInputTokens
	}
	cacheCreation5m, cacheCreation1h = helper.NormalizeCacheCreationSplit(usage.CacheCreationInputTokens, cacheCreation5m, cacheCreation1h)
	if cacheCreation5m > 0 || cacheCreation1h > 0 {
		usage.CacheCreation = &shared.ClaudeCacheCreationUsage{
			Ephemeral5mInputTokens: cacheCreation5m,
			Ephemeral1hInputTokens: cacheCreation1h,
		}
	}
	return usage
}

func shouldSkipClaudeMessageDeltaUsagePatch(info *relaycommon.RelayInfo) bool {
	if info == nil {
		return false
	}
	return info.ChannelSetting.PassThroughBodyEnabled
}

func patchClaudeMessageDeltaUsageData(data string, usage *shared.ClaudeUsage) string {
	if data == "" || usage == nil {
		return data
	}

	data = setMessageDeltaUsageInt(data, "usage.input_tokens", usage.InputTokens)
	data = setMessageDeltaUsageInt(data, "usage.cache_read_input_tokens", usage.CacheReadInputTokens)
	data = setMessageDeltaUsageInt(data, "usage.cache_creation_input_tokens", usage.CacheCreationInputTokens)

	if usage.CacheCreation != nil {
		data = setMessageDeltaUsageInt(data, "usage.cache_creation.ephemeral_5m_input_tokens", usage.CacheCreation.Ephemeral5mInputTokens)
		data = setMessageDeltaUsageInt(data, "usage.cache_creation.ephemeral_1h_input_tokens", usage.CacheCreation.Ephemeral1hInputTokens)
	}

	return data
}

func setMessageDeltaUsageInt(data string, path string, localValue int) string {
	if localValue <= 0 {
		return data
	}

	upstreamValue := gjson.Get(data, path)
	if upstreamValue.Exists() && upstreamValue.Int() > 0 {
		return data
	}

	patchedData, err := sjson.Set(data, path, localValue)
	if err != nil {
		return data
	}
	return patchedData
}

func FormatClaudeResponseInfo(claudeResponse *shared.ClaudeResponse, oaiResponse *shared.ChatCompletionsStreamResponse, claudeInfo *ClaudeResponseInfo) bool {
	if claudeInfo == nil {
		return false
	}
	if claudeInfo.Usage == nil {
		claudeInfo.Usage = &shared.Usage{}
	}
	if claudeResponse.Type == "message_start" {
		if claudeResponse.Message != nil {
			claudeInfo.ResponseId = claudeResponse.Message.Id
			claudeInfo.Model = claudeResponse.Message.Model
		}

		// message_start, 获取usage
		if claudeResponse.Message != nil && claudeResponse.Message.Usage != nil {
			claudeInfo.Usage = mergeClaudeUsageIntoOpenAIUsage(claudeInfo.Usage, claudeResponse.Message.Usage)
		}
	} else if claudeResponse.Type == "content_block_delta" {
		if claudeResponse.Delta != nil {
			if claudeResponse.Delta.Text != nil {
				claudeInfo.ResponseText.WriteString(*claudeResponse.Delta.Text)
			}
			if claudeResponse.Delta.Thinking != nil {
				claudeInfo.ResponseText.WriteString(*claudeResponse.Delta.Thinking)
			}
		}
	} else if claudeResponse.Type == "message_delta" {
		// 最终的usage获取：只合并上游非零字段，避免 message_delta 缺 cache 字段时覆盖 message_start 已记录的数据。
		if claudeResponse.Usage != nil {
			claudeInfo.Usage = mergeClaudeUsageIntoOpenAIUsage(claudeInfo.Usage, claudeResponse.Usage)
		}

		// 判断是否完整
		claudeInfo.Done = true
	} else if claudeResponse.Type == "content_block_start" {
	} else {
		return false
	}
	if oaiResponse != nil {
		oaiResponse.Id = claudeInfo.ResponseId
		oaiResponse.Created = claudeInfo.Created
		oaiResponse.Model = claudeInfo.Model
	}
	return true
}

func HandleStreamResponseData(c *gin.Context, info *relaycommon.RelayInfo, claudeInfo *ClaudeResponseInfo, data string) *shared.NookMuxError {
	var claudeResponse shared.ClaudeResponse
	err := jsonx.UnmarshalJsonStr(data, &claudeResponse)
	if err != nil {
		common.SysLog("error unmarshalling stream response: " + err.Error())
		return shared.NewError(err, shared.ErrorCodeBadResponseBody)
	}
	if claudeError := claudeResponse.GetClaudeError(); claudeError != nil && claudeError.Type != "" {
		// 按官方文档把错误类型还原为真实 HTTP 状态码（如 overloaded_error→529），
		// 保持重试与渠道禁用判断的准确性。
		return shared.WithClaudeError(*claudeError, shared.ClaudeErrorStatusCode(claudeError.Type))
	}
	if claudeResponse.StopReason != "" {
		maybeMarkClaudeRefusal(c, claudeResponse.StopReason)
	}
	if claudeResponse.Delta != nil && claudeResponse.Delta.StopReason != nil {
		maybeMarkClaudeRefusal(c, *claudeResponse.Delta.StopReason)
	}
	// 流式响应中提取 ServerToolUse（web search 调用次数），用于后续计费
	// Claude 在 message_delta 事件的 usage 中返回 server_tool_use.web_search_requests
	if claudeResponse.Usage != nil && claudeResponse.Usage.ServerToolUse != nil && claudeResponse.Usage.ServerToolUse.WebSearchRequests > 0 {
		c.Set("claude_web_search_requests", claudeResponse.Usage.ServerToolUse.WebSearchRequests)
	}
	if info.RelayFormat == relayconstant.RelayFormatClaude {
		FormatClaudeResponseInfo(&claudeResponse, nil, claudeInfo)

		if claudeResponse.Type == "message_start" {
			// message_start, 获取usage
			if claudeResponse.Message != nil {
				info.UpstreamModelName = claudeResponse.Message.Model
			}
		} else if claudeResponse.Type == "message_delta" {
			// 确保 message_delta 的 usage 包含完整的 input_tokens 和 cache 相关字段
			// 解决 AWS Bedrock 等上游返回的 message_delta 缺少这些字段的问题
			if !shouldSkipClaudeMessageDeltaUsagePatch(info) {
				data = patchClaudeMessageDeltaUsageData(data, buildMessageDeltaPatchUsage(&claudeResponse, claudeInfo))
			}
		}
		data = string(helper.MaskClaudeEventModelJSON(jsonx.StringToByteSlice(data), info))
		helper.ClaudeChunkData(c, claudeResponse, data)
	} else if info.RelayFormat == relayconstant.RelayFormatOpenAI {
		response := StreamResponseClaude2OpenAI(&claudeResponse)

		if !FormatClaudeResponseInfo(&claudeResponse, response, claudeInfo) {
			return nil
		}
		helper.MaskChatStreamResponseModel(response, info)

		err = helper.ObjectData(c, response)
		if err != nil {
			log.LogError(c, "send_stream_response_failed: "+err.Error())
		}
	} else if info.RelayFormat == relayconstant.RelayFormatOpenAIResponses {
		response := StreamResponseClaude2OpenAI(&claudeResponse)

		if !FormatClaudeResponseInfo(&claudeResponse, response, claudeInfo) {
			return nil
		}
		if response == nil {
			return nil
		}
		helper.MaskChatStreamResponseModel(response, info)

		if err := writeClaudeChatChunkAsResponsesEvent(c, info, claudeInfo, response); err != nil {
			return err
		}
	} else if info.RelayFormat == relayconstant.RelayFormatGemini {
		response := StreamResponseClaude2OpenAI(&claudeResponse)

		if !FormatClaudeResponseInfo(&claudeResponse, response, claudeInfo) {
			return nil
		}
		if response == nil {
			return nil
		}
		helper.MaskChatStreamResponseModel(response, info)

		geminiResponse := helper.StreamResponseOpenAI2Gemini(response, info)
		if geminiResponse == nil {
			return nil
		}

		geminiResponseStr, marshalErr := jsonx.Marshal(geminiResponse)
		if marshalErr != nil {
			return shared.NewError(marshalErr, shared.ErrorCodeBadResponseBody)
		}
		c.Render(-1, &common.CustomEvent{Data: "data: " + string(geminiResponseStr)})
		_ = helper.FlushWriter(c)
	}
	return nil
}

func HandleStreamFinalResponse(c *gin.Context, info *relaycommon.RelayInfo, claudeInfo *ClaudeResponseInfo) {
	if claudeInfo.Usage == nil {
		claudeInfo.Usage = &shared.Usage{}
	}
	if claudeInfo.Usage.CompletionTokens == 0 || !claudeInfo.Done {
		if common.DebugEnabled {
			common.SysLog("claude response usage is not complete, maybe upstream error")
		}
		// 只补缺失字段，不整份覆盖，保留 message_start 已拿到的 cache 计费字段。
		fallback := billing.ResponseText2Usage(c, claudeInfo.ResponseText.String(), info.UpstreamModelName, claudeInfo.Usage.PromptTokens)
		if claudeInfo.Usage.CompletionTokens == 0 || (!claudeInfo.Done && fallback.CompletionTokens > claudeInfo.Usage.CompletionTokens) {
			claudeInfo.Usage.CompletionTokens = fallback.CompletionTokens
			claudeInfo.Usage.OutputTokens = fallback.CompletionTokens
		}
		if claudeInfo.Usage.PromptTokens == 0 {
			claudeInfo.Usage.PromptTokens = fallback.PromptTokens
			claudeInfo.Usage.InputTokens = fallback.PromptTokens
		}
		claudeInfo.Usage.TotalTokens = claudeInfo.Usage.PromptTokens + claudeInfo.Usage.CompletionTokens
	}

	if info.RelayFormat == relayconstant.RelayFormatClaude {
		//
	} else if info.RelayFormat == relayconstant.RelayFormatOpenAI {
		if info.ShouldIncludeUsage {
			response := helper.GenerateFinalUsageResponse(claudeInfo.ResponseId, claudeInfo.Created, info.GetResponseModelName(), *claudeInfo.Usage)
			err := helper.ObjectData(c, response)
			if err != nil {
				common.SysLog("send final response failed: " + err.Error())
			}
		}
		helper.Done(c)
	} else if info.RelayFormat == relayconstant.RelayFormatOpenAIResponses {
		if err := writeClaudeResponsesFinalEvent(c, info, claudeInfo); err != nil {
			common.SysLog("send final responses response failed: " + err.Error())
		}
	} else if info.RelayFormat == relayconstant.RelayFormatGemini {
		response := helper.GenerateFinalUsageResponse(claudeInfo.ResponseId, claudeInfo.Created, info.GetResponseModelName(), *claudeInfo.Usage)
		geminiResponse := helper.StreamResponseOpenAI2Gemini(response, info)
		if geminiResponse == nil {
			return
		}
		geminiResponseStr, err := jsonx.Marshal(geminiResponse)
		if err != nil {
			common.SysLog("send final gemini response failed: " + err.Error())
			return
		}
		c.Render(-1, &common.CustomEvent{Data: "data: " + string(geminiResponseStr)})
		_ = helper.FlushWriter(c)
	}
}

func ensureClaudeResponsesStreamConverter(info *relaycommon.RelayInfo, claudeInfo *ClaudeResponseInfo) stream.OpenAIWireStreamConverter {
	if claudeInfo.ResponsesStreamConverter == nil {
		claudeInfo.ResponsesStreamConverter = stream.NewChatToResponsesStreamConverter(info.OpenAIResponsesToolContext)
	}
	return claudeInfo.ResponsesStreamConverter
}

func writeClaudeChatChunkAsResponsesEvent(c *gin.Context, info *relaycommon.RelayInfo, claudeInfo *ClaudeResponseInfo, response *shared.ChatCompletionsStreamResponse) *shared.NookMuxError {
	converter := ensureClaudeResponsesStreamConverter(info, claudeInfo)
	data, err := jsonx.Marshal(response)
	if err != nil {
		return shared.NewError(err, shared.ErrorCodeBadResponseBody)
	}
	out, err := converter.ConvertFrame("", string(data), "data: "+string(data)+"\n\n")
	if err != nil {
		return shared.NewError(err, shared.ErrorCodeBadResponseBody)
	}
	if out == "" {
		return nil
	}
	if _, err := c.Writer.Write([]byte(out)); err != nil {
		return shared.NewError(err, shared.ErrorCodeBadResponseBody)
	}
	if err := helper.FlushWriter(c); err != nil {
		return shared.NewError(err, shared.ErrorCodeBadResponseBody)
	}
	return nil
}

func writeClaudeResponsesFinalEvent(c *gin.Context, info *relaycommon.RelayInfo, claudeInfo *ClaudeResponseInfo) *shared.NookMuxError {
	if claudeInfo.ResponsesCompletedEmitted {
		return nil
	}
	converter := ensureClaudeResponsesStreamConverter(info, claudeInfo)

	if claudeInfo.Usage != nil {
		usageChunk := helper.GenerateFinalUsageResponse(claudeInfo.ResponseId, claudeInfo.Created, info.GetResponseModelName(), *claudeInfo.Usage)
		data, err := jsonx.Marshal(usageChunk)
		if err != nil {
			return shared.NewError(err, shared.ErrorCodeBadResponseBody)
		}
		if _, err := converter.ConvertFrame("", string(data), "data: "+string(data)+"\n\n"); err != nil {
			return shared.NewError(err, shared.ErrorCodeBadResponseBody)
		}
	}

	out, err := converter.ConvertFrame("", "[DONE]", "data: [DONE]\n\n")
	if err != nil {
		return shared.NewError(err, shared.ErrorCodeBadResponseBody)
	}
	if out != "" {
		if _, err := c.Writer.Write([]byte(out)); err != nil {
			return shared.NewError(err, shared.ErrorCodeBadResponseBody)
		}
		if err := helper.FlushWriter(c); err != nil {
			return shared.NewError(err, shared.ErrorCodeBadResponseBody)
		}
	}
	claudeInfo.ResponsesCompletedEmitted = true
	return nil
}

func ClaudeStreamHandler(c *gin.Context, resp *http.Response, info *relaycommon.RelayInfo) (*shared.Usage, *shared.NookMuxError) {
	claudeInfo := &ClaudeResponseInfo{
		ResponseId:   helper.GetResponseID(c),
		Created:      common.GetTimestamp(),
		Model:        info.UpstreamModelName,
		ResponseText: strings.Builder{},
		Usage:        &shared.Usage{},
	}
	var err *shared.NookMuxError
	streamErr := helper.StreamScannerHandler(c, resp, info, func(data string) bool {
		err = HandleStreamResponseData(c, info, claudeInfo, data)
		return err == nil
	})
	if err != nil {
		return nil, err
	}
	// 流异常终止（超时/连接断开）：若尚未收到任何计费数据，直接上报真实错误，
	// 避免计费层因 usage 为空伪造「502 上游没有返回计费信息」掩盖根因。
	// 若已收到部分数据（message_start 的 usage 或正文），保持既有兜底：
	// 补完流结束并按部分内容估算计费。
	if streamErr != nil && !claudeInfo.Done &&
		claudeInfo.Usage.PromptTokens == 0 && claudeInfo.ResponseText.Len() == 0 {
		return nil, streamErr
	}

	HandleStreamFinalResponse(c, info, claudeInfo)
	return claudeInfo.Usage, nil
}

func HandleClaudeResponseData(c *gin.Context, info *relaycommon.RelayInfo, claudeInfo *ClaudeResponseInfo, httpResp *http.Response, data []byte) *shared.NookMuxError {
	var claudeResponse shared.ClaudeResponse
	err := jsonx.Unmarshal(data, &claudeResponse)
	if err != nil {
		return shared.NewError(err, shared.ErrorCodeBadResponseBody)
	}
	if claudeError := claudeResponse.GetClaudeError(); claudeError != nil && claudeError.Type != "" {
		// 按官方文档把错误类型还原为真实 HTTP 状态码，保持重试与渠道禁用判断的准确性。
		return shared.WithClaudeError(*claudeError, shared.ClaudeErrorStatusCode(claudeError.Type))
	}
	maybeMarkClaudeRefusal(c, claudeResponse.StopReason)
	if claudeInfo.Usage == nil {
		claudeInfo.Usage = &shared.Usage{}
	}
	if claudeResponse.Usage != nil {
		claudeInfo.Usage = shared.ClaudeUsageToOpenAIUsage(claudeResponse.Usage)
	}
	var responseData []byte
	switch info.RelayFormat {
	case relayconstant.RelayFormatOpenAI:
		openaiResponse := ResponseClaude2OpenAI(&claudeResponse)
		helper.MaskTextResponseModel(openaiResponse, info)
		openaiResponse.Usage = *claudeInfo.Usage
		responseData, err = jsonx.Marshal(openaiResponse)
		if err != nil {
			return shared.NewError(err, shared.ErrorCodeBadResponseBody)
		}
	case relayconstant.RelayFormatOpenAIResponses:
		openaiResponse := ResponseClaude2OpenAI(&claudeResponse)
		helper.MaskTextResponseModel(openaiResponse, info)
		openaiResponse.Usage = *claudeInfo.Usage
		cv := convert.NewConverter(shared.OpenAIWireAPIChat, shared.OpenAIWireAPIResponses)
		cv.ToolContext = info.OpenAIResponsesToolContext
		responsesResp, convErr := cv.ConvertChatToResponsesResponse(openaiResponse)
		if convErr != nil {
			return shared.NewError(convErr, shared.ErrorCodeBadResponseBody)
		}
		responseData, err = jsonx.Marshal(responsesResp)
		if err != nil {
			return shared.NewError(err, shared.ErrorCodeBadResponseBody)
		}
	case relayconstant.RelayFormatClaude:
		responseData = helper.MaskTopLevelModelJSON(data, info)
	case relayconstant.RelayFormatGemini:
		openaiResponse := ResponseClaude2OpenAI(&claudeResponse)
		helper.MaskTextResponseModel(openaiResponse, info)
		openaiResponse.Usage = *claudeInfo.Usage
		geminiResponse := helper.ResponseOpenAI2Gemini(openaiResponse, info)
		responseData, err = jsonx.Marshal(geminiResponse)
		if err != nil {
			return shared.NewError(err, shared.ErrorCodeBadResponseBody)
		}
	}

	if claudeResponse.Usage != nil && claudeResponse.Usage.ServerToolUse != nil && claudeResponse.Usage.ServerToolUse.WebSearchRequests > 0 {
		c.Set("claude_web_search_requests", claudeResponse.Usage.ServerToolUse.WebSearchRequests)
	}

	helper.IOCopyBytesGracefully(c, httpResp, responseData)
	return nil
}

func ClaudeHandler(c *gin.Context, resp *http.Response, info *relaycommon.RelayInfo) (*shared.Usage, *shared.NookMuxError) {
	defer helper.CloseResponseBodyGracefully(resp)

	claudeInfo := &ClaudeResponseInfo{
		ResponseId:   helper.GetResponseID(c),
		Created:      common.GetTimestamp(),
		Model:        info.UpstreamModelName,
		ResponseText: strings.Builder{},
		Usage:        &shared.Usage{},
	}
	responseBody, err := helper.ReadResponseBody(resp.Body)
	if err != nil {
		return nil, shared.NewError(err, shared.ErrorCodeBadResponseBody)
	}
	if common.DebugEnabled {
		println("responseBody: ", string(responseBody))
	}
	handleErr := HandleClaudeResponseData(c, info, claudeInfo, resp, responseBody)
	if handleErr != nil {
		return nil, handleErr
	}
	return claudeInfo.Usage, nil
}

func mapToolChoice(toolChoice any, parallelToolCalls *bool) *shared.ClaudeToolChoice {
	var claudeToolChoice *shared.ClaudeToolChoice

	// 处理 tool_choice 字符串值
	if toolChoiceStr, ok := toolChoice.(string); ok {
		switch toolChoiceStr {
		case "auto":
			claudeToolChoice = &shared.ClaudeToolChoice{
				Type: "auto",
			}
		case "required":
			claudeToolChoice = &shared.ClaudeToolChoice{
				Type: "any",
			}
		case "none":
			claudeToolChoice = &shared.ClaudeToolChoice{
				Type: "none",
			}
		}
	} else if toolChoiceMap, ok := toolChoice.(map[string]interface{}); ok {
		// 处理 tool_choice 对象值
		if function, ok := toolChoiceMap["function"].(map[string]interface{}); ok {
			if toolName, ok := function["name"].(string); ok {
				claudeToolChoice = &shared.ClaudeToolChoice{
					Type: "tool",
					Name: toolName,
				}
			}
		}
	}

	// 处理 parallel_tool_calls
	if parallelToolCalls != nil {
		if claudeToolChoice == nil {
			// 如果没有 tool_choice，但有 parallel_tool_calls，创建默认的 auto 类型
			claudeToolChoice = &shared.ClaudeToolChoice{
				Type: "auto",
			}
		}

		// Anthropic schema: tool_choice.type=none does not accept extra fields.
		// When tools are disabled, parallel_tool_calls is irrelevant, so we drop it.
		if claudeToolChoice.Type != "none" {
			// 如果 parallel_tool_calls 为 true，则 disable_parallel_tool_use 为 false
			claudeToolChoice.DisableParallelToolUse = !*parallelToolCalls
		}
	}

	return claudeToolChoice
}
