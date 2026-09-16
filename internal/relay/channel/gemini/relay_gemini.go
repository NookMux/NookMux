package gemini

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/NookMux/NookMux/internal/common"
	"github.com/NookMux/NookMux/internal/config/model"
	"github.com/NookMux/NookMux/internal/domain/shared"
	"github.com/NookMux/NookMux/internal/infra/log"
	"github.com/NookMux/NookMux/internal/relay/channel/openai"
	"github.com/NookMux/NookMux/internal/relay/channel/openrouter"
	relaycommon "github.com/NookMux/NookMux/internal/relay/common"
	"github.com/NookMux/NookMux/internal/relay/helper"

	billing "github.com/NookMux/NookMux/internal/domain/billing"
	channelconstant "github.com/NookMux/NookMux/internal/domain/channel/constant"
	"github.com/NookMux/NookMux/internal/httpapi"
	httpclient "github.com/NookMux/NookMux/internal/infra/httpclient"
	media "github.com/NookMux/NookMux/internal/infra/media"
	relayconstant "github.com/NookMux/NookMux/internal/relay/constant"
	"github.com/NookMux/NookMux/pkg/jsonx"
	"github.com/gin-gonic/gin"
)

const thoughtSignatureBypassValue = "context_engineering_is_the_way_to_go"

func reasoningEffortToGeminiThinkingLevel(effort string) string {
	switch strings.ToLower(strings.TrimSpace(effort)) {
	case "", "none":
		return ""
	case "minimal", "low":
		return "low"
	case "medium":
		return "medium"
	case "high", "xhigh", "max":
		return "high"
	default:
		return strings.ToLower(strings.TrimSpace(effort))
	}
}

func openAIReasoningToGeminiThinkingConfig(textRequest shared.GeneralOpenAIRequest) (*shared.GeminiThinkingConfig, error) {
	if len(textRequest.Reasoning) > 0 {
		var reasoning openrouter.RequestReasoning
		if err := jsonx.Unmarshal(textRequest.Reasoning, &reasoning); err != nil {
			return nil, fmt.Errorf("invalid reasoning payload: %w", err)
		}
		if reasoning.MaxTokens > 0 {
			return &shared.GeminiThinkingConfig{
				IncludeThoughts: true,
				ThinkingBudget:  common.GetPointer(reasoning.MaxTokens),
			}, nil
		}
	}

	if level := reasoningEffortToGeminiThinkingLevel(textRequest.ReasoningEffort); level != "" {
		return &shared.GeminiThinkingConfig{
			IncludeThoughts: true,
			ThinkingLevel:   level,
		}, nil
	}
	return nil, nil
}

// https://cloud.google.com/vertex-ai/generative-ai/docs/model-reference/inference?hl=zh-cn#blob
var geminiSupportedMimeTypes = map[string]bool{
	"application/pdf": true,
	"audio/mpeg":      true,
	"audio/mp3":       true,
	"audio/wav":       true,
	"image/png":       true,
	"image/jpeg":      true,
	"image/jpg":       true,
	"image/webp":      true,
	"image/heic":      true,
	"image/heif":      true,
	"text/plain":      true,
	"video/mov":       true,
	"video/mpeg":      true,
	"video/mp4":       true,
	"video/mpg":       true,
	"video/avi":       true,
	"video/wmv":       true,
	"video/mpegps":    true,
	"video/flv":       true,
}

// CovertOpenAI2Gemini 将OpenAI请求转换为Gemini请求格式
func CovertOpenAI2Gemini(c *gin.Context, textRequest shared.GeneralOpenAIRequest, info *relaycommon.RelayInfo) (*shared.GeminiChatRequest, error) {

	geminiRequest := shared.GeminiChatRequest{
		Contents: make([]shared.GeminiChatContent, 0, len(textRequest.Messages)),
		GenerationConfig: shared.GeminiChatGenerationConfig{
			Temperature:     textRequest.Temperature,
			TopP:            textRequest.TopP,
			MaxOutputTokens: textRequest.GetMaxTokens(),
			Seed:            int64(textRequest.Seed),
		},
	}

	var googleExtraBody map[string]interface{}
	attachThoughtSignatureRequested := false
	thoughtSignatureValue := thoughtSignatureBypassValue
	if len(textRequest.ExtraBody) > 0 {
		var extraBody map[string]interface{}
		if err := jsonx.Unmarshal(textRequest.ExtraBody, &extraBody); err != nil {
			return nil, fmt.Errorf("invalid extra body: %w", err)
		}
		// eg. {"google":{"thinking_config":{"thinking_budget":5324,"include_thoughts":true}}}
		if googleBody, ok := extraBody["google"].(map[string]interface{}); ok {
			googleExtraBody = googleBody
			if signatureValue, ok, err := parseFunctionCallThoughtSignature(googleBody); err != nil {
				return nil, err
			} else if ok {
				attachThoughtSignatureRequested = true
				thoughtSignatureValue = signatureValue
			}
		}
	}

	attachThoughtSignature := (info.ChannelType == channelconstant.ChannelTypeGemini ||
		info.ChannelType == channelconstant.ChannelTypeVertexAi) &&
		attachThoughtSignatureRequested
	thoughtSignature := json.RawMessage(strconv.Quote(thoughtSignatureValue))

	if model.IsGeminiModelSupportImagine(info.UpstreamModelName) {
		geminiRequest.GenerationConfig.ResponseModalities = []string{
			"TEXT",
			"IMAGE",
		}
	}
	if stopSequences := parseStopSequences(textRequest.Stop); len(stopSequences) > 0 {
		// Gemini supports up to 5 stop sequences
		if len(stopSequences) > 5 {
			stopSequences = stopSequences[:5]
		}
		geminiRequest.GenerationConfig.StopSequences = stopSequences
	}

	// patch extra_body
	if googleExtraBody != nil {
		// check error param name like thinkingConfig, should be thinking_config
		if _, hasErrorParam := googleExtraBody["thinkingConfig"]; hasErrorParam {
			return nil, errors.New("extra_body.google.thinkingConfig is not supported, use extra_body.google.thinking_config instead")
		}

		if thinkingConfig, ok := googleExtraBody["thinking_config"].(map[string]interface{}); ok {
			// check error param name like thinkingBudget, should be thinking_budget
			if _, hasErrorParam := thinkingConfig["thinkingBudget"]; hasErrorParam {
				return nil, errors.New("extra_body.google.thinking_config.thinkingBudget is not supported, use extra_body.google.thinking_config.thinking_budget instead")
			}
			if budget, ok := thinkingConfig["thinking_budget"].(float64); ok {
				budgetInt := int(budget)
				geminiRequest.GenerationConfig.ThinkingConfig = &shared.GeminiThinkingConfig{
					ThinkingBudget:  common.GetPointer(budgetInt),
					IncludeThoughts: true,
				}
			} else {
				geminiRequest.GenerationConfig.ThinkingConfig = &shared.GeminiThinkingConfig{
					IncludeThoughts: true,
				}
			}
		}

		// check error param name like imageConfig, should be image_config
		if _, hasErrorParam := googleExtraBody["imageConfig"]; hasErrorParam {
			return nil, errors.New("extra_body.google.imageConfig is not supported, use extra_body.google.image_config instead")
		}

		if imageConfig, ok := googleExtraBody["image_config"].(map[string]interface{}); ok {
			// check error param name like aspectRatio, should be aspect_ratio
			if _, hasErrorParam := imageConfig["aspectRatio"]; hasErrorParam {
				return nil, errors.New("extra_body.google.image_config.aspectRatio is not supported, use extra_body.google.image_config.aspect_ratio instead")
			}
			// check error param name like imageSize, should be image_size
			if _, hasErrorParam := imageConfig["imageSize"]; hasErrorParam {
				return nil, errors.New("extra_body.google.image_config.imageSize is not supported, use extra_body.google.image_config.image_size instead")
			}

			// convert snake_case to camelCase for Gemini API
			geminiImageConfig := make(map[string]interface{})
			if aspectRatio, ok := imageConfig["aspect_ratio"]; ok {
				geminiImageConfig["aspectRatio"] = aspectRatio
			}
			if imageSize, ok := imageConfig["image_size"]; ok {
				geminiImageConfig["imageSize"] = imageSize
			}

			if len(geminiImageConfig) > 0 {
				imageConfigBytes, err := jsonx.Marshal(geminiImageConfig)
				if err != nil {
					return nil, fmt.Errorf("failed to marshal image_config: %w", err)
				}
				geminiRequest.GenerationConfig.ImageConfig = imageConfigBytes
			}
		}
	}
	if geminiRequest.GenerationConfig.ThinkingConfig == nil {
		thinkingConfig, err := openAIReasoningToGeminiThinkingConfig(textRequest)
		if err != nil {
			return nil, err
		}
		if thinkingConfig != nil {
			geminiRequest.GenerationConfig.ThinkingConfig = thinkingConfig
		}
	}

	safetySettings := make([]shared.GeminiChatSafetySettings, 0, len(SafetySettingList))
	for _, category := range SafetySettingList {
		safetySettings = append(safetySettings, shared.GeminiChatSafetySettings{
			Category:  category,
			Threshold: model.GetGeminiSafetySetting(category),
		})
	}
	geminiRequest.SafetySettings = safetySettings

	// openaiContent.FuncToToolCalls()
	if textRequest.Tools != nil {
		functions := make([]shared.FunctionRequest, 0, len(textRequest.Tools))
		googleSearch := false
		codeExecution := false
		urlContext := false
		for _, tool := range textRequest.Tools {
			if tool.Function.Name == "googleSearch" {
				googleSearch = true
				continue
			}
			if tool.Function.Name == "codeExecution" {
				codeExecution = true
				continue
			}
			if tool.Function.Name == "urlContext" {
				urlContext = true
				continue
			}
			if tool.Function.Parameters != nil {

				params, ok := tool.Function.Parameters.(map[string]interface{})
				if ok {
					if props, hasProps := params["properties"].(map[string]interface{}); hasProps {
						if len(props) == 0 {
							tool.Function.Parameters = nil
						}
					}
				}
			}
			// Clean the parameters before appending
			cleanedParams := cleanFunctionParameters(tool.Function.Parameters)
			tool.Function.Parameters = cleanedParams
			functions = append(functions, tool.Function)
		}
		geminiTools := geminiRequest.GetTools()
		if codeExecution {
			geminiTools = append(geminiTools, shared.GeminiChatTool{
				CodeExecution: make(map[string]string),
			})
		}
		if googleSearch {
			geminiTools = append(geminiTools, shared.GeminiChatTool{
				GoogleSearch: make(map[string]string),
			})
			// 标记请求使用了 Google Search，用于后续计费
			c.Set("gemini_web_search_requests", 1)
		}
		if urlContext {
			geminiTools = append(geminiTools, shared.GeminiChatTool{
				URLContext: make(map[string]string),
			})
		}
		if len(functions) > 0 {
			geminiTools = append(geminiTools, shared.GeminiChatTool{
				FunctionDeclarations: functions,
			})
		}
		geminiRequest.SetTools(geminiTools)

		// [NEW] Convert OpenAI tool_choice to Gemini toolConfig.functionCallingConfig
		// Mapping: "auto" -> "AUTO", "none" -> "NONE", "required" -> "ANY"
		// Object format: {"type": "function", "function": {"name": "xxx"}} -> "ANY" + allowedFunctionNames
		if textRequest.ToolChoice != nil {
			geminiRequest.ToolConfig = convertToolChoiceToGeminiConfig(textRequest.ToolChoice)
		}
	}

	if textRequest.ResponseFormat != nil && (textRequest.ResponseFormat.Type == "json_schema" || textRequest.ResponseFormat.Type == "json_object") {
		geminiRequest.GenerationConfig.ResponseMimeType = "application/json"

		if len(textRequest.ResponseFormat.JsonSchema) > 0 {
			// 先将json.RawMessage解析
			var jsonSchema shared.FormatJsonSchema
			if err := jsonx.Unmarshal(textRequest.ResponseFormat.JsonSchema, &jsonSchema); err == nil {
				cleanedSchema := removeAdditionalPropertiesWithDepth(jsonSchema.Schema, 0)
				geminiRequest.GenerationConfig.ResponseSchema = cleanedSchema
			}
		}
	}
	tool_call_ids := make(map[string]string)
	var system_content []string
	//shouldAddDummyModelMessage := false
	for _, message := range textRequest.Messages {
		if message.Role == "system" || message.Role == "developer" {
			system_content = append(system_content, message.StringContent())
			continue
		} else if message.Role == "tool" || message.Role == "function" {
			if len(geminiRequest.Contents) == 0 || geminiRequest.Contents[len(geminiRequest.Contents)-1].Role == "model" {
				geminiRequest.Contents = append(geminiRequest.Contents, shared.GeminiChatContent{
					Role: "user",
				})
			}
			var parts = &geminiRequest.Contents[len(geminiRequest.Contents)-1].Parts
			name := ""
			if message.Name != nil {
				name = *message.Name
			} else if val, exists := tool_call_ids[message.ToolCallId]; exists {
				name = val
			}
			var contentMap map[string]interface{}
			contentStr := message.StringContent()

			// 1. 尝试解析为 JSON 对象
			if err := jsonx.Unmarshal([]byte(contentStr), &contentMap); err != nil {
				// 2. 如果失败，尝试解析为 JSON 数组
				var contentSlice []interface{}
				if err := jsonx.Unmarshal([]byte(contentStr), &contentSlice); err == nil {
					// 如果是数组，包装成对象
					contentMap = map[string]interface{}{"result": contentSlice}
				} else {
					// 3. 如果再次失败，作为纯文本处理
					contentMap = map[string]interface{}{"content": contentStr}
				}
			}

			functionResp := &shared.GeminiFunctionResponse{
				Name:     name,
				Response: contentMap,
			}
			functionResp.SetID(message.ToolCallId)

			*parts = append(*parts, shared.GeminiPart{
				FunctionResponse: functionResp,
			})
			continue
		}
		var parts []shared.GeminiPart
		content := shared.GeminiChatContent{
			Role: message.Role,
		}
		reasoningText := message.GetReasoningContent()
		if reasoningText != "" || message.ReasoningSignature != "" {
			reasoningPart := shared.GeminiPart{
				Text:    reasoningText,
				Thought: true,
			}
			reasoningPart.SetThoughtSignature(message.ReasoningSignature)
			parts = append(parts, reasoningPart)
		}
		shouldAttachThoughtSignature := attachThoughtSignature && (message.Role == "assistant" || message.Role == "model")
		signatureAttached := false
		// isToolCall := false
		if message.ToolCalls != nil {
			// message.Role = "model"
			// isToolCall = true
			for _, call := range message.ParseToolCalls() {
				args := map[string]interface{}{}
				if call.Function.Arguments != "" {
					if jsonx.Unmarshal([]byte(call.Function.Arguments), &args) != nil {
						return nil, fmt.Errorf("invalid arguments for function %s, args: %s", call.Function.Name, call.Function.Arguments)
					}
				}
				toolCall := shared.GeminiPart{
					FunctionCall: &shared.FunctionCall{
						FunctionName: call.Function.Name,
						Arguments:    args,
					},
				}
				if shouldAttachThoughtSignature && !signatureAttached && hasFunctionCallContent(toolCall.FunctionCall) && len(toolCall.ThoughtSignature) == 0 {
					toolCall.ThoughtSignature = thoughtSignature
					signatureAttached = true
				}
				parts = append(parts, toolCall)
				tool_call_ids[call.ID] = call.Function.Name
			}
		}

		openaiContent := message.ParseContent()
		for _, part := range openaiContent {
			if part.Type == shared.ContentTypeText {
				if part.Text == "" {
					continue
				}
				// check markdown image ![image](data:image/jpeg;base64,xxxxxxxxxxxx)
				// 使用字符串查找而非正则，避免大文本性能问题
				text := part.Text
				hasMarkdownImage := false
				for {
					// 快速检查是否包含 markdown 图片标记
					startIdx := strings.Index(text, "![")
					if startIdx == -1 {
						break
					}
					// 找到 ](
					bracketIdx := strings.Index(text[startIdx:], "](data:")
					if bracketIdx == -1 {
						break
					}
					bracketIdx += startIdx
					// 找到闭合的 )
					closeIdx := strings.Index(text[bracketIdx+2:], ")")
					if closeIdx == -1 {
						break
					}
					closeIdx += bracketIdx + 2

					hasMarkdownImage = true
					// 添加图片前的文本
					if startIdx > 0 {
						textBefore := text[:startIdx]
						if textBefore != "" {
							parts = append(parts, shared.GeminiPart{
								Text: textBefore,
							})
						}
					}
					// 提取 data URL (从 "](" 后面开始，到 ")" 之前)
					dataUrl := text[bracketIdx+2 : closeIdx]
					format, base64String, err := media.DecodeBase64FileData(dataUrl)
					if err != nil {
						return nil, fmt.Errorf("decode markdown base64 image data failed: %s", err.Error())
					}
					imgPart := shared.GeminiPart{
						InlineData: &shared.GeminiInlineData{
							MimeType: format,
							Data:     base64String,
						},
					}
					if shouldAttachThoughtSignature {
						imgPart.ThoughtSignature = thoughtSignature
					}
					parts = append(parts, imgPart)
					// 继续处理剩余文本
					text = text[closeIdx+1:]
				}
				// 添加剩余文本或原始文本（如果没有找到 markdown 图片）
				if !hasMarkdownImage {
					parts = append(parts, shared.GeminiPart{
						Text: part.Text,
					})
				}
			} else if part.Type == shared.ContentTypeImageURL {
				// 使用统一的文件服务获取图片数据
				var source *shared.FileSource
				imageUrl := part.GetImageMedia().Url
				if strings.HasPrefix(imageUrl, "http") {
					source = shared.NewURLFileSource(imageUrl)
				} else {
					source = shared.NewBase64FileSource(imageUrl, "")
				}
				base64Data, mimeType, err := media.GetBase64Data(c, source, "formatting image for Gemini")
				if err != nil {
					return nil, fmt.Errorf("get file data from '%s' failed: %w", source.GetIdentifier(), err)
				}

				// 校验 MimeType 是否在 Gemini 支持的白名单中
				if _, ok := geminiSupportedMimeTypes[strings.ToLower(mimeType)]; !ok {
					return nil, fmt.Errorf("mime type is not supported by Gemini: '%s', url: '%s', supported types are: %v", mimeType, source.GetIdentifier(), getSupportedMimeTypesList())
				}

				parts = append(parts, shared.GeminiPart{
					InlineData: &shared.GeminiInlineData{
						MimeType: mimeType,
						Data:     base64Data,
					},
				})
			} else if part.Type == shared.ContentTypeFile {
				if part.GetFile().FileId != "" {
					return nil, fmt.Errorf("only base64 file is supported in gemini")
				}
				fileSource := shared.NewBase64FileSource(part.GetFile().FileData, "")
				base64Data, mimeType, err := media.GetBase64Data(c, fileSource, "formatting file for Gemini")
				if err != nil {
					return nil, fmt.Errorf("decode base64 file data failed: %s", err.Error())
				}
				parts = append(parts, shared.GeminiPart{
					InlineData: &shared.GeminiInlineData{
						MimeType: mimeType,
						Data:     base64Data,
					},
				})
			} else if part.Type == shared.ContentTypeInputAudio {
				if part.GetInputAudio().Data == "" {
					return nil, fmt.Errorf("only base64 audio is supported in gemini")
				}
				audioSource := shared.NewBase64FileSource(part.GetInputAudio().Data, "audio/"+part.GetInputAudio().Format)
				base64Data, mimeType, err := media.GetBase64Data(c, audioSource, "formatting audio for Gemini")
				if err != nil {
					return nil, fmt.Errorf("decode base64 audio data failed: %s", err.Error())
				}
				parts = append(parts, shared.GeminiPart{
					InlineData: &shared.GeminiInlineData{
						MimeType: mimeType,
						Data:     base64Data,
					},
				})
			}
		}

		// 如果需要附加签名但还没有附加（没有 tool_calls 或 tool_calls 为空），
		// 则在第一个文本 part 上附加 thoughtSignature
		if shouldAttachThoughtSignature && !signatureAttached && len(parts) > 0 {
			for i := range parts {
				if parts[i].Text != "" {
					parts[i].ThoughtSignature = thoughtSignature
					break
				}
			}
		}

		content.Parts = parts

		// there's no assistant role in gemini and API shall vomit if Role is not user or model
		if content.Role == "assistant" {
			content.Role = "model"
		}
		if len(content.Parts) > 0 {
			geminiRequest.Contents = append(geminiRequest.Contents, content)
		}
	}

	if len(system_content) > 0 {
		geminiRequest.SystemInstructions = &shared.GeminiChatContent{
			Parts: []shared.GeminiPart{
				{
					Text: strings.Join(system_content, "\n"),
				},
			},
		}
	}

	return &geminiRequest, nil
}

// parseStopSequences 解析停止序列，支持字符串或字符串数组
func parseStopSequences(stop any) []string {
	if stop == nil {
		return nil
	}

	switch v := stop.(type) {
	case string:
		if v != "" {
			return []string{v}
		}
	case []string:
		return v
	case []interface{}:
		sequences := make([]string, 0, len(v))
		for _, item := range v {
			if str, ok := item.(string); ok && str != "" {
				sequences = append(sequences, str)
			}
		}
		return sequences
	}
	return nil
}

func hasFunctionCallContent(call *shared.FunctionCall) bool {
	if call == nil {
		return false
	}
	if strings.TrimSpace(call.FunctionName) != "" {
		return true
	}

	switch v := call.Arguments.(type) {
	case nil:
		return false
	case string:
		return strings.TrimSpace(v) != ""
	case map[string]interface{}:
		return len(v) > 0
	case []interface{}:
		return len(v) > 0
	default:
		return true
	}
}

func parseFunctionCallThoughtSignature(googleBody map[string]interface{}) (string, bool, error) {
	if googleBody == nil {
		return "", false, nil
	}

	if v, ok := googleBody["thought_signature"]; ok {
		return normalizeFunctionCallThoughtSignatureValue("thought_signature", v)
	}
	if v, ok := googleBody["thoughtSignature"]; ok {
		return normalizeFunctionCallThoughtSignatureValue("thoughtSignature", v)
	}

	return "", false, nil
}

func normalizeFunctionCallThoughtSignatureValue(field string, v interface{}) (string, bool, error) {
	switch value := v.(type) {
	case nil:
		return thoughtSignatureBypassValue, true, nil
	case string:
		if strings.TrimSpace(value) == "" {
			return thoughtSignatureBypassValue, true, nil
		}
		return value, true, nil
	default:
		return "", false, fmt.Errorf("extra_body.google.%s must be a string or null", field)
	}
}

// Helper function to get a list of supported MIME types for error messages
func getSupportedMimeTypesList() []string {
	keys := make([]string, 0, len(geminiSupportedMimeTypes))
	for k := range geminiSupportedMimeTypes {
		keys = append(keys, k)
	}
	return keys
}

var geminiOpenAPISchemaAllowedFields = map[string]struct{}{
	"anyOf":            {},
	"default":          {},
	"description":      {},
	"enum":             {},
	"example":          {},
	"format":           {},
	"items":            {},
	"maxItems":         {},
	"maxLength":        {},
	"maxProperties":    {},
	"maximum":          {},
	"minItems":         {},
	"minLength":        {},
	"minProperties":    {},
	"minimum":          {},
	"nullable":         {},
	"pattern":          {},
	"properties":       {},
	"propertyOrdering": {},
	"required":         {},
	"title":            {},
	"type":             {},
}

const geminiFunctionSchemaMaxDepth = 64

// cleanFunctionParameters recursively removes unsupported fields from Gemini function parameters.
func cleanFunctionParameters(params interface{}) interface{} {
	return cleanFunctionParametersWithDepth(params, 0)
}

func cleanFunctionParametersWithDepth(params interface{}, depth int) interface{} {
	if params == nil {
		return nil
	}

	if depth >= geminiFunctionSchemaMaxDepth {
		return cleanFunctionParametersShallow(params)
	}

	switch v := params.(type) {
	case map[string]interface{}:
		// Keep only Gemini-supported OpenAPI schema subset fields (per official SDK Schema).
		cleanedMap := make(map[string]interface{}, len(v))
		for k, val := range v {
			if _, ok := geminiOpenAPISchemaAllowedFields[k]; ok {
				cleanedMap[k] = val
			}
		}

		normalizeGeminiSchemaTypeAndNullable(cleanedMap)

		// Clean properties
		if props, ok := cleanedMap["properties"].(map[string]interface{}); ok && props != nil {
			cleanedProps := make(map[string]interface{})
			for propName, propValue := range props {
				cleanedProps[propName] = cleanFunctionParametersWithDepth(propValue, depth+1)
			}
			cleanedMap["properties"] = cleanedProps
		}

		// Recursively clean items in arrays
		if items, ok := cleanedMap["items"].(map[string]interface{}); ok && items != nil {
			cleanedMap["items"] = cleanFunctionParametersWithDepth(items, depth+1)
		}
		// OpenAPI tuple-style items is not supported by Gemini SDK Schema; keep first to avoid API rejection.
		if itemsArray, ok := cleanedMap["items"].([]interface{}); ok && len(itemsArray) > 0 {
			cleanedMap["items"] = cleanFunctionParametersWithDepth(itemsArray[0], depth+1)
		}

		// Recursively clean anyOf
		if nested, ok := cleanedMap["anyOf"].([]interface{}); ok && nested != nil {
			cleanedNested := make([]interface{}, len(nested))
			for i, item := range nested {
				cleanedNested[i] = cleanFunctionParametersWithDepth(item, depth+1)
			}
			cleanedMap["anyOf"] = cleanedNested
		}

		return cleanedMap

	case []interface{}:
		// Handle arrays of schemas
		cleanedArray := make([]interface{}, len(v))
		for i, item := range v {
			cleanedArray[i] = cleanFunctionParametersWithDepth(item, depth+1)
		}
		return cleanedArray

	default:
		// Not a map or array, return as is (e.g., could be a primitive)
		return params
	}
}

func cleanFunctionParametersShallow(params interface{}) interface{} {
	switch v := params.(type) {
	case map[string]interface{}:
		cleanedMap := make(map[string]interface{}, len(v))
		for k, val := range v {
			if _, ok := geminiOpenAPISchemaAllowedFields[k]; ok {
				cleanedMap[k] = val
			}
		}
		normalizeGeminiSchemaTypeAndNullable(cleanedMap)
		// Stop recursion and avoid retaining huge nested structures.
		delete(cleanedMap, "properties")
		delete(cleanedMap, "items")
		delete(cleanedMap, "anyOf")
		return cleanedMap
	case []interface{}:
		// Prefer an empty list over deep recursion on attacker-controlled inputs.
		return []interface{}{}
	default:
		return params
	}
}

func normalizeGeminiSchemaTypeAndNullable(schema map[string]interface{}) {
	rawType, ok := schema["type"]
	if !ok || rawType == nil {
		return
	}

	normalize := func(t string) (string, bool) {
		switch strings.ToLower(strings.TrimSpace(t)) {
		case "object":
			return "OBJECT", false
		case "array":
			return "ARRAY", false
		case "string":
			return "STRING", false
		case "integer":
			return "INTEGER", false
		case "number":
			return "NUMBER", false
		case "boolean":
			return "BOOLEAN", false
		case "null":
			return "", true
		default:
			return t, false
		}
	}

	switch t := rawType.(type) {
	case string:
		normalized, isNull := normalize(t)
		if isNull {
			schema["nullable"] = true
			delete(schema, "type")
			return
		}
		schema["type"] = normalized
	case []interface{}:
		nullable := false
		var chosen string
		for _, item := range t {
			if s, ok := item.(string); ok {
				normalized, isNull := normalize(s)
				if isNull {
					nullable = true
					continue
				}
				if chosen == "" {
					chosen = normalized
				}
			}
		}
		if nullable {
			schema["nullable"] = true
		}
		if chosen != "" {
			schema["type"] = chosen
		} else {
			delete(schema, "type")
		}
	}
}

func removeAdditionalPropertiesWithDepth(schema interface{}, depth int) interface{} {
	if depth >= 5 {
		return schema
	}

	v, ok := schema.(map[string]interface{})
	if !ok || len(v) == 0 {
		return schema
	}
	// 删除所有的title字段
	delete(v, "title")
	delete(v, "$schema")
	// 如果type不为object和array，则直接返回
	if typeVal, exists := v["type"]; !exists || (typeVal != "object" && typeVal != "array") {
		return schema
	}
	switch v["type"] {
	case "object":
		delete(v, "additionalProperties")
		// 处理 properties
		if properties, ok := v["properties"].(map[string]interface{}); ok {
			for key, value := range properties {
				properties[key] = removeAdditionalPropertiesWithDepth(value, depth+1)
			}
		}
		for _, field := range []string{"allOf", "anyOf", "oneOf"} {
			if nested, ok := v[field].([]interface{}); ok {
				for i, item := range nested {
					nested[i] = removeAdditionalPropertiesWithDepth(item, depth+1)
				}
			}
		}
	case "array":
		if items, ok := v["items"].(map[string]interface{}); ok {
			v["items"] = removeAdditionalPropertiesWithDepth(items, depth+1)
		}
	}

	return v
}

func getResponseToolCall(item *shared.GeminiPart) *shared.ToolCallResponse {
	var argsBytes []byte
	var err error
	// 直接序列化结构化参数，避免把 Gemini 的函数参数再额外转义一次。
	// JSON 序列化/反序列化已经正确处理了转义字符
	argsBytes, err = jsonx.Marshal(item.FunctionCall.Arguments)

	if err != nil {
		return nil
	}
	return &shared.ToolCallResponse{
		ID:   fmt.Sprintf("call_%s", common.GetUUID()),
		Type: "function",
		Function: shared.FunctionResponse{
			Arguments: string(argsBytes),
			Name:      item.FunctionCall.FunctionName,
		},
	}
}

func responseGeminiChat2OpenAI(c *gin.Context, response *shared.GeminiChatResponse) *shared.OpenAITextResponse {
	fullTextResponse := shared.OpenAITextResponse{
		Id:      helper.GetResponseID(c),
		Object:  "chat.completion",
		Created: common.GetTimestamp(),
		Choices: make([]shared.OpenAITextResponseChoice, 0, len(response.Candidates)),
	}
	for _, candidate := range response.Candidates {
		choice := shared.OpenAITextResponseChoice{
			Index: int(candidate.Index),
			Message: shared.Message{
				Role:    "assistant",
				Content: "",
			},
			FinishReason: relayconstant.FinishReasonStop,
		}
		hasToolCalls := false
		if len(candidate.Content.Parts) > 0 {
			var content strings.Builder
			var inlineGrow int
			for _, part := range candidate.Content.Parts {
				if part.InlineData != nil {
					inlineGrow += len(part.InlineData.MimeType) + len(part.InlineData.Data) + 32
				}
			}
			if inlineGrow > 0 {
				content.Grow(inlineGrow)
			}
			appended := 0
			writeSep := func() {
				if appended > 0 {
					content.WriteByte('\n')
				}
				appended++
			}
			var reasoningTexts []string
			reasoningSignature := ""
			var toolCalls []shared.ToolCallResponse
			for _, part := range candidate.Content.Parts {
				if signature := part.GetThoughtSignature(); signature != "" && reasoningSignature == "" {
					reasoningSignature = signature
				}
				if part.InlineData != nil {
					// 媒体内容
					if strings.HasPrefix(part.InlineData.MimeType, "image") {
						writeSep()
						content.WriteString("![image](data:")
						content.WriteString(part.InlineData.MimeType)
						content.WriteString(";base64,")
						content.WriteString(part.InlineData.Data)
						content.WriteByte(')')
					} else {
						// 其他媒体类型，直接显示链接
						writeSep()
						content.WriteString("[media](data:")
						content.WriteString(part.InlineData.MimeType)
						content.WriteString(";base64,")
						content.WriteString(part.InlineData.Data)
						content.WriteByte(')')
					}
				} else if part.FunctionCall != nil {
					choice.FinishReason = relayconstant.FinishReasonToolCalls
					if call := getResponseToolCall(&part); call != nil {
						toolCalls = append(toolCalls, *call)
					}
				} else if part.Thought {
					reasoningTexts = append(reasoningTexts, part.Text)
				} else {
					if part.ExecutableCode != nil {
						writeSep()
						content.WriteString("```")
						content.WriteString(part.ExecutableCode.Language)
						content.WriteByte('\n')
						content.WriteString(part.ExecutableCode.Code)
						content.WriteString("\n```")
					} else if part.CodeExecutionResult != nil {
						writeSep()
						content.WriteString("```output\n")
						content.WriteString(part.CodeExecutionResult.Output)
						content.WriteString("\n```")
					} else {
						// 过滤掉空行
						if part.Text != "\n" {
							writeSep()
							content.WriteString(part.Text)
						}
					}
				}
			}
			if len(toolCalls) > 0 {
				choice.Message.SetToolCalls(toolCalls)
				hasToolCalls = true
				choice.FinishReason = relayconstant.FinishReasonToolCalls
			}
			if len(reasoningTexts) > 0 {
				choice.Message.SetReasoningContent(strings.Join(reasoningTexts, "\n"))
			}
			if reasoningSignature != "" {
				choice.Message.ReasoningSignature = reasoningSignature
			}
			choice.Message.SetStringContent(content.String())

		}
		if candidate.FinishReason != nil {
			switch *candidate.FinishReason {
			case "STOP":
				choice.FinishReason = relayconstant.FinishReasonStop
			case "MAX_TOKENS":
				choice.FinishReason = relayconstant.FinishReasonLength
			case "SAFETY":
				// Safety filter triggered
				choice.FinishReason = relayconstant.FinishReasonContentFilter
			case "RECITATION":
				// Recitation (citation) detected
				choice.FinishReason = relayconstant.FinishReasonContentFilter
			case "BLOCKLIST":
				// Blocklist triggered
				choice.FinishReason = relayconstant.FinishReasonContentFilter
			case "PROHIBITED_CONTENT":
				// Prohibited content detected
				choice.FinishReason = relayconstant.FinishReasonContentFilter
			case "SPII":
				// Sensitive personally identifiable information
				choice.FinishReason = relayconstant.FinishReasonContentFilter
			case "OTHER":
				// Other reasons
				choice.FinishReason = relayconstant.FinishReasonContentFilter
			default:
				choice.FinishReason = relayconstant.FinishReasonContentFilter
			}
		}
		if hasToolCalls {
			choice.FinishReason = relayconstant.FinishReasonToolCalls
		}
		fullTextResponse.Choices = append(fullTextResponse.Choices, choice)
	}
	return &fullTextResponse
}

func streamResponseGeminiChat2OpenAI(geminiResponse *shared.GeminiChatResponse) (*shared.ChatCompletionsStreamResponse, bool) {
	choices := make([]shared.ChatCompletionsStreamResponseChoice, 0, len(geminiResponse.Candidates))
	isStop := false
	for _, candidate := range geminiResponse.Candidates {
		finishReason := ""
		if candidate.FinishReason != nil {
			finishReason = *candidate.FinishReason
		}
		if finishReason == "STOP" {
			isStop = true
			finishReason = ""
		}
		choice := shared.ChatCompletionsStreamResponseChoice{
			Index: int(candidate.Index),
			Delta: shared.ChatCompletionsStreamResponseChoiceDelta{
				//Role: "assistant",
			},
		}
		var content strings.Builder
		var inlineGrow int
		for _, part := range candidate.Content.Parts {
			if part.InlineData != nil {
				inlineGrow += len(part.InlineData.MimeType) + len(part.InlineData.Data) + 32
			}
		}
		if inlineGrow > 0 {
			content.Grow(inlineGrow)
		}
		appended := 0
		writeSep := func() {
			if appended > 0 {
				content.WriteByte('\n')
			}
			appended++
		}
		var reasoningTexts []string
		reasoningSignature := ""
		isTools := false
		if finishReason != "" {
			// Map Gemini FinishReason to OpenAI finish_reason
			switch finishReason {
			case "STOP":
				// Normal completion
				choice.FinishReason = &relayconstant.FinishReasonStop
			case "MAX_TOKENS":
				// Reached maximum token limit
				choice.FinishReason = &relayconstant.FinishReasonLength
			case "SAFETY":
				// Safety filter triggered
				choice.FinishReason = &relayconstant.FinishReasonContentFilter
			case "RECITATION":
				// Recitation (citation) detected
				choice.FinishReason = &relayconstant.FinishReasonContentFilter
			case "BLOCKLIST":
				// Blocklist triggered
				choice.FinishReason = &relayconstant.FinishReasonContentFilter
			case "PROHIBITED_CONTENT":
				// Prohibited content detected
				choice.FinishReason = &relayconstant.FinishReasonContentFilter
			case "SPII":
				// Sensitive personally identifiable information
				choice.FinishReason = &relayconstant.FinishReasonContentFilter
			case "OTHER":
				// Other reasons
				choice.FinishReason = &relayconstant.FinishReasonContentFilter
			default:
				// Unknown reason, treat as content filter
				choice.FinishReason = &relayconstant.FinishReasonContentFilter
			}
		}
		for _, part := range candidate.Content.Parts {
			if signature := part.GetThoughtSignature(); signature != "" && reasoningSignature == "" {
				reasoningSignature = signature
			}
			if part.InlineData != nil {
				if strings.HasPrefix(part.InlineData.MimeType, "image") {
					writeSep()
					content.WriteString("![image](data:")
					content.WriteString(part.InlineData.MimeType)
					content.WriteString(";base64,")
					content.WriteString(part.InlineData.Data)
					content.WriteByte(')')
				}
			} else if part.FunctionCall != nil {
				isTools = true
				if call := getResponseToolCall(&part); call != nil {
					call.SetIndex(len(choice.Delta.ToolCalls))
					choice.Delta.ToolCalls = append(choice.Delta.ToolCalls, *call)
				}

			} else if part.Thought {
				reasoningTexts = append(reasoningTexts, part.Text)
			} else {
				if part.ExecutableCode != nil {
					writeSep()
					content.WriteString("```")
					content.WriteString(part.ExecutableCode.Language)
					content.WriteByte('\n')
					content.WriteString(part.ExecutableCode.Code)
					content.WriteString("\n```\n")
				} else if part.CodeExecutionResult != nil {
					writeSep()
					content.WriteString("```output\n")
					content.WriteString(part.CodeExecutionResult.Output)
					content.WriteString("\n```\n")
				} else {
					if part.Text != "\n" {
						writeSep()
						content.WriteString(part.Text)
					}
				}
			}
		}
		if content.Len() > 0 {
			choice.Delta.SetContentString(content.String())
		}
		if len(reasoningTexts) > 0 {
			choice.Delta.SetReasoningContent(strings.Join(reasoningTexts, "\n"))
		}
		if reasoningSignature != "" {
			choice.Delta.ReasoningSignature = common.GetPointer(reasoningSignature)
		}
		if isTools {
			choice.FinishReason = &relayconstant.FinishReasonToolCalls
		}
		choices = append(choices, choice)
	}

	var response shared.ChatCompletionsStreamResponse
	response.Object = "chat.completion.chunk"
	response.Choices = choices
	return &response, isStop
}

func handleStream(c *gin.Context, info *relaycommon.RelayInfo, resp *shared.ChatCompletionsStreamResponse) error {
	streamData, err := jsonx.Marshal(resp)
	if err != nil {
		return fmt.Errorf("failed to marshal stream response: %w", err)
	}
	err = openai.HandleStreamFormat(c, info, string(streamData), info.ChannelSetting.ForceFormat)
	if err != nil {
		return fmt.Errorf("failed to handle stream format: %w", err)
	}
	return nil
}

func handleFinalStream(c *gin.Context, info *relaycommon.RelayInfo, resp *shared.ChatCompletionsStreamResponse) error {
	streamData, err := jsonx.Marshal(resp)
	if err != nil {
		return fmt.Errorf("failed to marshal stream response: %w", err)
	}
	openai.HandleFinalResponse(c, info, string(streamData), resp.Id, resp.Created, resp.Model, resp.GetSystemFingerprint(), resp.Usage, false)
	return nil
}

func geminiStreamHandler(c *gin.Context, info *relaycommon.RelayInfo, resp *http.Response, callback func(data string, geminiResponse *shared.GeminiChatResponse) bool) (*shared.Usage, *shared.NookMuxError) {
	var usage = &shared.Usage{}
	var imageCount int
	responseText := strings.Builder{}
	var streamApiErr *shared.NookMuxError

	if streamErr := helper.StreamScannerHandler(c, resp, info, func(data string) bool {
		var geminiResponse shared.GeminiChatResponse
		err := jsonx.UnmarshalJsonStr(data, &geminiResponse)
		if err != nil {
			log.LogError(c, "error unmarshalling stream response: "+err.Error())
			return false
		}

		// 上游在 HTTP 200 流内返回错误载荷（Gemini/中间网关会把 429/5xx 转成
		// 200 + error 帧下发）：保留真实上游错误，避免计费阶段因 totalTokens=0
		// 被误记为「502 上游没有返回计费信息」。
		if geminiResponse.Error != nil && geminiResponse.Error.Message != "" {
			streamApiErr = shared.WithOpenAIError(shared.OpenAIError{
				Message: geminiResponse.Error.Message,
				Type:    "upstream_error",
				Code:    geminiResponse.Error.Status,
			}, helper.UpstreamErrorStatusCode(resp.StatusCode, geminiResponse.Error.Code))
			return false
		}

		if len(geminiResponse.Candidates) == 0 && geminiResponse.PromptFeedback != nil && geminiResponse.PromptFeedback.BlockReason != nil {
			httpapi.SetContextKey(c, common.ContextKeyAdminRejectReason, fmt.Sprintf("gemini_block_reason=%s", *geminiResponse.PromptFeedback.BlockReason))
		}

		// 统计图片数量
		for _, candidate := range geminiResponse.Candidates {
			for _, part := range candidate.Content.Parts {
				if part.InlineData != nil && part.InlineData.MimeType != "" {
					imageCount++
				}
				if part.Text != "" {
					responseText.WriteString(part.Text)
				}
			}
		}

		// Gemini 兼容实现可能返回 snake_case usage，这里统一走标准化映射。
		if billing.HasGeminiUsageMetadata(geminiResponse.UsageMetadata) {
			convertedUsage := billing.GeminiUsageMetadataToOpenAIUsage(geminiResponse.UsageMetadata)
			*usage = convertedUsage
		}

		return callback(data, &geminiResponse)
	}); streamErr != nil {
		// 流异常终止（超时/连接断开）：记录真实原因供排查；已收到的部分内容
		// 仍按既有兜底估算计费，避免重试造成重复下发，故不向上返回错误。
		log.LogError(c, "gemini stream terminated abnormally: "+streamErr.Error())
	}

	if streamApiErr != nil {
		// 上游在流内返回了错误载荷：真实错误已识别，直接向上暴露，不再伪造 usage。
		helper.ResetStatusCode(streamApiErr, c.GetString("status_code_mapping"))
		return nil, streamApiErr
	}

	if imageCount != 0 {
		if usage.CompletionTokens == 0 {
			usage.CompletionTokens = imageCount * 1400
		}
	}

	if usage.CompletionTokens <= 0 {
		if info.ReceivedResponseCount > 0 {
			usage = billing.ResponseText2Usage(c, responseText.String(), info.UpstreamModelName, info.GetEstimatePromptTokens())
		} else {
			usage = &shared.Usage{}
		}
	}

	return usage, nil
}

func GeminiChatStreamHandler(c *gin.Context, info *relaycommon.RelayInfo, resp *http.Response) (*shared.Usage, *shared.NookMuxError) {
	id := helper.GetResponseID(c)
	createAt := common.GetTimestamp()
	responseModel := info.GetResponseModelName()
	finishReason := relayconstant.FinishReasonStop
	toolCallIndexByChoice := make(map[int]map[string]int)
	nextToolCallIndexByChoice := make(map[int]int)

	usage, err := geminiStreamHandler(c, info, resp, func(data string, geminiResponse *shared.GeminiChatResponse) bool {
		response, isStop := streamResponseGeminiChat2OpenAI(geminiResponse)

		response.Id = id
		response.Created = createAt
		response.Model = responseModel
		for choiceIdx := range response.Choices {
			choiceKey := response.Choices[choiceIdx].Index
			indexState := toolCallIndexByChoice[choiceKey]
			if indexState == nil {
				indexState = make(map[string]int)
				toolCallIndexByChoice[choiceKey] = indexState
			}
			for toolIdx := range response.Choices[choiceIdx].Delta.ToolCalls {
				tool := &response.Choices[choiceIdx].Delta.ToolCalls[toolIdx]
				toolID := tool.ID
				if toolID == "" {
					toolID = fmt.Sprintf("call_%s", common.GetUUID())
					tool.ID = toolID
				}
				stableIndex, ok := indexState[toolID]
				if !ok {
					stableIndex = nextToolCallIndexByChoice[choiceKey]
					indexState[toolID] = stableIndex
					nextToolCallIndexByChoice[choiceKey] = stableIndex + 1
				}
				tool.SetIndex(stableIndex)
			}
		}
		if response.IsToolCall() {
			finishReason = relayconstant.FinishReasonToolCalls
			if info.RelayFormat == relayconstant.RelayFormatClaude {
				for choiceIdx := range response.Choices {
					response.Choices[choiceIdx].FinishReason = nil
				}
			}
		}

		log.LogDebug(c, fmt.Sprintf("info.SendResponseCount = %d", info.SendResponseCount))
		if info.SendResponseCount == 0 {
			// send first response
			emptyResponse := helper.GenerateStartEmptyResponse(id, createAt, responseModel, nil)
			if response.IsToolCall() {
				if len(emptyResponse.Choices) > 0 && len(response.Choices) > 0 {
					toolCalls := response.Choices[0].Delta.ToolCalls
					copiedToolCalls := make([]shared.ToolCallResponse, len(toolCalls))
					for idx := range toolCalls {
						copiedToolCalls[idx] = toolCalls[idx]
						copiedToolCalls[idx].Function.Arguments = ""
					}
					emptyResponse.Choices[0].Delta.ToolCalls = copiedToolCalls
				}
				finishReason = relayconstant.FinishReasonToolCalls
				err := handleStream(c, info, emptyResponse)
				if err != nil {
					log.LogError(c, err.Error())
				}

				response.ClearToolCalls()
				if response.IsFinished() {
					response.Choices[0].FinishReason = nil
				}
			} else {
				err := handleStream(c, info, emptyResponse)
				if err != nil {
					log.LogError(c, err.Error())
				}
			}
		}

		err := handleStream(c, info, response)
		if err != nil {
			log.LogError(c, err.Error())
		}
		if isStop && info.RelayFormat != relayconstant.RelayFormatClaude {
			_ = handleStream(c, info, helper.GenerateStopResponse(id, createAt, responseModel, finishReason))
		}
		return true
	})

	if err != nil {
		return usage, err
	}

	response := helper.GenerateFinalUsageResponse(id, createAt, responseModel, *usage)
	if info.RelayFormat == relayconstant.RelayFormatClaude && info.ClaudeConvertInfo != nil && !info.ClaudeConvertInfo.Done {
		response = helper.GenerateStopResponse(id, createAt, responseModel, finishReason)
		response.Usage = usage
	}
	handleErr := handleFinalStream(c, info, response)
	if handleErr != nil {
		common.SysLog("send final response failed: " + handleErr.Error())
	}
	return usage, nil
}

func GeminiChatHandler(c *gin.Context, info *relaycommon.RelayInfo, resp *http.Response) (*shared.Usage, *shared.NookMuxError) {
	defer helper.CloseResponseBodyGracefully(resp)

	responseBody, err := helper.ReadResponseBody(resp.Body)
	if err != nil {
		return nil, shared.NewOpenAIError(err, shared.ErrorCodeBadResponseBody, http.StatusInternalServerError)
	}
	if common.DebugEnabled {
		println(string(responseBody))
	}
	var geminiResponse shared.GeminiChatResponse
	err = jsonx.Unmarshal(responseBody, &geminiResponse)
	if err != nil {
		return nil, shared.NewOpenAIError(err, shared.ErrorCodeBadResponseBody, http.StatusInternalServerError)
	}
	// 上游在 HTTP 200 中携带错误载荷（Gemini/中间网关会把 429/5xx 转成
	// 200 + {"error":{...}} 下发）：保留真实上游错误，避免计费阶段因
	// totalTokens=0 被误记为「502 上游没有返回计费信息」。
	if geminiResponse.Error != nil && geminiResponse.Error.Message != "" {
		return nil, shared.WithOpenAIError(shared.OpenAIError{
			Message: geminiResponse.Error.Message,
			Type:    "upstream_error",
			Code:    geminiResponse.Error.Status,
		}, helper.UpstreamErrorStatusCode(resp.StatusCode, geminiResponse.Error.Code))
	}
	if len(geminiResponse.Candidates) == 0 {
		usage := billing.GeminiUsageMetadataToOpenAIUsage(geminiResponse.UsageMetadata)
		if usage.PromptTokens <= 0 {
			usage.PromptTokens = info.GetEstimatePromptTokens()
			if usage.TotalTokens < usage.PromptTokens {
				usage.TotalTokens = usage.PromptTokens + usage.CompletionTokens
			}
		}

		var newAPIError *shared.NookMuxError
		if geminiResponse.PromptFeedback != nil && geminiResponse.PromptFeedback.BlockReason != nil {
			httpapi.SetContextKey(c, common.ContextKeyAdminRejectReason, fmt.Sprintf("gemini_block_reason=%s", *geminiResponse.PromptFeedback.BlockReason))
			newAPIError = shared.NewOpenAIError(
				errors.New("request blocked by Gemini API: "+*geminiResponse.PromptFeedback.BlockReason),
				shared.ErrorCodePromptBlocked,
				http.StatusBadRequest,
			)
		} else {
			httpapi.SetContextKey(c, common.ContextKeyAdminRejectReason, "gemini_empty_candidates")
			newAPIError = shared.NewOpenAIError(
				errors.New("empty response from Gemini API"),
				shared.ErrorCodeEmptyResponse,
				http.StatusInternalServerError,
			)
		}

		helper.ResetStatusCode(newAPIError, c.GetString("status_code_mapping"))

		switch info.RelayFormat {
		case relayconstant.RelayFormatClaude:
			c.JSON(newAPIError.StatusCode, gin.H{
				"type":  "error",
				"error": newAPIError.ToClaudeError(),
			})
		default:
			c.JSON(newAPIError.StatusCode, gin.H{
				"error": newAPIError.ToOpenAIError(),
			})
		}
		return &usage, nil
	}
	fullTextResponse := responseGeminiChat2OpenAI(c, &geminiResponse)
	fullTextResponse.Model = info.GetResponseModelName()
	usage := billing.GeminiUsageMetadataToOpenAIUsage(geminiResponse.UsageMetadata)

	fullTextResponse.Usage = usage

	switch info.RelayFormat {
	case relayconstant.RelayFormatOpenAI:
		responseBody, err = jsonx.Marshal(fullTextResponse)
		if err != nil {
			return nil, shared.NewError(err, shared.ErrorCodeBadResponseBody)
		}
	case relayconstant.RelayFormatClaude:
		claudeResp := helper.ResponseOpenAI2Claude(fullTextResponse, info)
		claudeRespStr, err := jsonx.Marshal(claudeResp)
		if err != nil {
			return nil, shared.NewError(err, shared.ErrorCodeBadResponseBody)
		}
		responseBody = claudeRespStr
	case relayconstant.RelayFormatGemini:
		break
	}

	helper.IOCopyBytesGracefully(c, resp, responseBody)

	return &usage, nil
}

func GeminiEmbeddingHandler(c *gin.Context, info *relaycommon.RelayInfo, resp *http.Response) (*shared.Usage, *shared.NookMuxError) {
	defer helper.CloseResponseBodyGracefully(resp)

	responseBody, readErr := helper.ReadEmbeddingResponseBody(resp.Body)
	if readErr != nil {
		return nil, shared.NewOpenAIError(readErr, shared.ErrorCodeBadResponseBody, http.StatusInternalServerError)
	}

	var geminiResponse shared.GeminiBatchEmbeddingResponse
	if jsonErr := jsonx.Unmarshal(responseBody, &geminiResponse); jsonErr != nil {
		return nil, shared.NewOpenAIError(jsonErr, shared.ErrorCodeBadResponseBody, http.StatusInternalServerError)
	}

	// convert to openai format response
	openAIResponse := shared.OpenAIEmbeddingResponse{
		Object: "list",
		Data:   make([]shared.OpenAIEmbeddingResponseItem, 0, len(geminiResponse.Embeddings)),
		Model:  info.GetResponseModelName(),
	}

	for i, embedding := range geminiResponse.Embeddings {
		openAIResponse.Data = append(openAIResponse.Data, shared.OpenAIEmbeddingResponseItem{
			Object:    "embedding",
			Embedding: embedding.Values,
			Index:     i,
		})
	}

	// calculate usage
	// https://ai.google.dev/gemini-api/docs/pricing?hl=zh-cn#text-embedding-004
	// Google has not yet clarified how embedding models will be billed
	// refer to openai billing method to use input tokens billing
	// https://platform.openai.com/docs/guides/embeddings#what-are-embeddings
	usage := billing.ResponseText2Usage(c, "", info.UpstreamModelName, info.GetEstimatePromptTokens())
	openAIResponse.Usage = *usage

	jsonResponse, jsonErr := jsonx.Marshal(openAIResponse)
	if jsonErr != nil {
		return nil, shared.NewOpenAIError(jsonErr, shared.ErrorCodeBadResponseBody, http.StatusInternalServerError)
	}

	helper.IOCopyBytesGracefully(c, resp, jsonResponse)
	return usage, nil
}

func GeminiImageHandler(c *gin.Context, info *relaycommon.RelayInfo, resp *http.Response) (*shared.Usage, *shared.NookMuxError) {
	defer helper.CloseResponseBodyGracefully(resp)

	responseBody, readErr := helper.ReadMediaResponseBody(resp.Body)
	if readErr != nil {
		return nil, shared.NewOpenAIError(readErr, shared.ErrorCodeBadResponseBody, http.StatusInternalServerError)
	}

	var geminiResponse shared.GeminiImageResponse
	if jsonErr := jsonx.Unmarshal(responseBody, &geminiResponse); jsonErr != nil {
		return nil, shared.NewOpenAIError(jsonErr, shared.ErrorCodeBadResponseBody, http.StatusInternalServerError)
	}

	if len(geminiResponse.Predictions) == 0 {
		return nil, shared.NewOpenAIError(errors.New("no images generated"), shared.ErrorCodeBadResponseBody, http.StatusInternalServerError)
	}

	// convert to openai format response
	openAIResponse := shared.ImageResponse{
		Created: common.GetTimestamp(),
		Data:    make([]shared.ImageData, 0, len(geminiResponse.Predictions)),
	}

	for _, prediction := range geminiResponse.Predictions {
		if prediction.RaiFilteredReason != "" {
			continue // skip filtered image
		}
		openAIResponse.Data = append(openAIResponse.Data, shared.ImageData{
			B64Json: prediction.BytesBase64Encoded,
		})
	}

	jsonResponse, jsonErr := jsonx.Marshal(openAIResponse)
	if jsonErr != nil {
		return nil, shared.NewError(jsonErr, shared.ErrorCodeBadResponseBody)
	}

	c.Writer.Header().Set("Content-Type", "application/json")
	c.Writer.WriteHeader(resp.StatusCode)
	_, _ = c.Writer.Write(jsonResponse)

	// https://github.com/google-gemini/cookbook/blob/719a27d752aac33f39de18a8d3cb42a70874917e/quickstarts/Counting_Tokens.ipynb
	// each image has fixed 258 tokens
	const imageTokens = 258
	generatedImages := len(openAIResponse.Data)

	usage := &shared.Usage{
		PromptTokens:     imageTokens * generatedImages, // each generated image has fixed 258 tokens
		CompletionTokens: 0,                             // image generation does not calculate completion tokens
		TotalTokens:      imageTokens * generatedImages,
	}

	return usage, nil
}

type GeminiModelsResponse struct {
	Models        []shared.GeminiModel `json:"models"`
	NextPageToken string               `json:"nextPageToken"`
}

func FetchGeminiModels(baseURL, apiKey, proxyURL string) ([]string, error) {
	return FetchGeminiModelsWithHeaders(baseURL, apiKey, proxyURL, nil)
}

func FetchGeminiModelsWithHeaders(baseURL, apiKey, proxyURL string, headers http.Header) ([]string, error) {
	client, err := httpclient.GetHttpClientWithProxy(proxyURL)
	if err != nil {
		return nil, fmt.Errorf("创建HTTP客户端失败: %v", err)
	}

	allModels := make([]string, 0)
	nextPageToken := ""
	maxPages := 100 // Safety limit to prevent infinite loops

	for page := 0; page < maxPages; page++ {
		url := fmt.Sprintf("%s/v1beta/models", baseURL)
		if nextPageToken != "" {
			url = fmt.Sprintf("%s?pageToken=%s", url, nextPageToken)
		}

		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		request, err := http.NewRequestWithContext(ctx, "GET", url, nil)
		if err != nil {
			cancel()
			return nil, fmt.Errorf("创建请求失败: %v", err)
		}

		if headers != nil {
			// Prefer explicitly provided auth headers.
			if strings.TrimSpace(headers.Get("x-goog-api-key")) == "" && strings.TrimSpace(headers.Get("Authorization")) == "" {
				request.Header.Set("x-goog-api-key", apiKey)
			}

			for name, values := range headers {
				trimmedValues := make([]string, 0, len(values))
				for _, raw := range values {
					value := strings.TrimSpace(raw)
					if value == "" {
						continue
					}
					trimmedValues = append(trimmedValues, value)
				}
				if len(trimmedValues) == 0 {
					continue
				}

				request.Header.Del(name)
				for _, value := range trimmedValues {
					request.Header.Add(name, value)
				}
			}
		} else {
			request.Header.Set("x-goog-api-key", apiKey)
		}

		response, err := client.Do(request)
		if err != nil {
			cancel()
			return nil, fmt.Errorf("请求失败: %v", err)
		}

		if response.StatusCode != http.StatusOK {
			body, _ := helper.ReadErrorResponseBody(response.Body)
			response.Body.Close()
			cancel()
			return nil, fmt.Errorf("服务器返回错误 %d: %s", response.StatusCode, string(body))
		}

		body, err := helper.ReadModelListResponseBody(response.Body)
		response.Body.Close()
		cancel()
		if err != nil {
			return nil, fmt.Errorf("读取响应失败: %v", err)
		}

		var modelsResponse GeminiModelsResponse
		if err = jsonx.Unmarshal(body, &modelsResponse); err != nil {
			return nil, fmt.Errorf("解析响应失败: %v", err)
		}

		for _, model := range modelsResponse.Models {
			modelNameValue, ok := model.Name.(string)
			if !ok {
				continue
			}
			modelName := strings.TrimPrefix(modelNameValue, "models/")
			allModels = append(allModels, modelName)
		}

		nextPageToken = modelsResponse.NextPageToken
		if nextPageToken == "" {
			break
		}
	}

	return allModels, nil
}

// convertToolChoiceToGeminiConfig converts OpenAI tool_choice to Gemini toolConfig
// OpenAI tool_choice values:
//   - "auto": Let the model decide (default)
//   - "none": Don't call any tools
//   - "required": Must call at least one tool
//   - {"type": "function", "function": {"name": "xxx"}}: Call specific function
//
// Gemini functionCallingConfig.mode values:
//   - "AUTO": Model decides whether to call functions
//   - "NONE": Model won't call functions
//   - "ANY": Model must call at least one function
func convertToolChoiceToGeminiConfig(toolChoice any) *shared.ToolConfig {
	if toolChoice == nil {
		return nil
	}

	// Handle string values: "auto", "none", "required"
	if toolChoiceStr, ok := toolChoice.(string); ok {
		config := &shared.ToolConfig{
			FunctionCallingConfig: &shared.FunctionCallingConfig{},
		}
		switch toolChoiceStr {
		case "auto":
			config.FunctionCallingConfig.Mode = "AUTO"
		case "none":
			config.FunctionCallingConfig.Mode = "NONE"
		case "required":
			config.FunctionCallingConfig.Mode = "ANY"
		default:
			// Unknown string value, default to AUTO
			config.FunctionCallingConfig.Mode = "AUTO"
		}
		return config
	}

	// Handle object value: {"type": "function", "function": {"name": "xxx"}}
	if toolChoiceMap, ok := toolChoice.(map[string]interface{}); ok {
		if toolChoiceMap["type"] == "function" {
			config := &shared.ToolConfig{
				FunctionCallingConfig: &shared.FunctionCallingConfig{
					Mode: "ANY",
				},
			}
			// Extract function name if specified
			if function, ok := toolChoiceMap["function"].(map[string]interface{}); ok {
				if name, ok := function["name"].(string); ok && name != "" {
					config.FunctionCallingConfig.AllowedFunctionNames = []string{name}
				}
			}
			return config
		}
		// Unsupported map structure (type is not "function"), return nil
		return nil
	}

	// Unsupported type, return nil
	return nil
}
