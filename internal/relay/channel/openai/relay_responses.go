package openai

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/NookMux/NookMux/internal/domain/shared"
	"github.com/NookMux/NookMux/internal/infra/log"
	relaycommon "github.com/NookMux/NookMux/internal/relay/common"
	"github.com/NookMux/NookMux/internal/relay/helper"
	"github.com/NookMux/NookMux/internal/relay/wire/convert"
	"github.com/NookMux/NookMux/internal/relay/wire/stream"

	"github.com/NookMux/NookMux/pkg/jsonx"

	tokenizer "github.com/NookMux/NookMux/internal/infra/tokenizer"
	relayconstant "github.com/NookMux/NookMux/internal/relay/constant"
	"github.com/gin-gonic/gin"
)

func OaiResponsesHandler(c *gin.Context, info *relaycommon.RelayInfo, resp *http.Response) (*shared.Usage, *shared.NookMuxError) {
	defer helper.CloseResponseBodyGracefully(resp)

	// read response body
	var responsesResponse shared.OpenAIResponsesResponse
	responseBody, err := helper.ReadResponseBody(resp.Body)
	if err != nil {
		return nil, shared.NewOpenAIError(err, shared.ErrorCodeReadResponseBodyFailed, http.StatusInternalServerError)
	}
	err = jsonx.Unmarshal(responseBody, &responsesResponse)
	if err != nil {
		return nil, shared.NewOpenAIError(err, shared.ErrorCodeBadResponseBody, http.StatusInternalServerError)
	}
	if oaiError := responsesResponse.GetOpenAIError(); oaiError != nil && oaiError.Message != "" {
		return nil, shared.WithOpenAIError(*oaiError, upstreamErrorStatusCode(resp.StatusCode, oaiError))
	}

	if responsesResponse.HasImageGenerationCall() {
		c.Set("image_generation_call", true)
		c.Set("image_generation_call_quality", responsesResponse.GetQuality())
		c.Set("image_generation_call_size", responsesResponse.GetSize())
	}
	helper.MaskResponsesResponseModel(&responsesResponse, info)

	// compute usage
	usage := shared.Usage{}
	convert.ApplyResponsesUsageToChatUsage(&usage, responsesResponse.Usage)

	if info != nil && info.RelayFormat == relayconstant.RelayFormatClaude {
		responseBody, err = convertResponsesBodyToClaudeBody(&responsesResponse, &usage, info)
		if err != nil {
			return nil, shared.NewError(err, shared.ErrorCodeBadResponseBody)
		}
	} else {
		responseBody = helper.MaskTopLevelModelJSON(responseBody, info)
	}

	// 写入新的 response body
	helper.IOCopyBytesGracefully(c, resp, responseBody)

	if info == nil || info.ResponsesUsageInfo == nil || info.ResponsesUsageInfo.BuiltInTools == nil {
		return &usage, nil
	}
	// 解析 Output 中的内置工具调用次数（web_search_call、file_search_call 等）
	// 注意：不能遍历 responsesResponse.Tools，那是请求工具配置的回显，不是实际调用结果
	for _, output := range responsesResponse.Output {
		var toolType string
		switch output.Type {
		case shared.BuildInCallWebSearchCall:
			toolType = shared.BuildInToolWebSearchPreview
		case shared.BuildInCallFileSearchCall:
			toolType = shared.BuildInToolFileSearch
		default:
			continue
		}
		buildToolinfo, ok := info.ResponsesUsageInfo.BuiltInTools[toolType]
		if !ok || buildToolinfo == nil {
			log.LogError(c, fmt.Sprintf("BuiltInTools not found for tool type: %v", toolType))
			continue
		}
		buildToolinfo.CallCount++
	}
	return &usage, nil
}

func OaiResponsesStreamHandler(c *gin.Context, info *relaycommon.RelayInfo, resp *http.Response) (*shared.Usage, *shared.NookMuxError) {
	if resp == nil || resp.Body == nil {
		log.LogError(c, "invalid response or response body")
		return nil, shared.NewError(fmt.Errorf("invalid response"), shared.ErrorCodeBadResponse)
	}

	defer helper.CloseResponseBodyGracefully(resp)

	var usage = &shared.Usage{}
	var responseTextBuilder strings.Builder
	var responsesToChat stream.OpenAIWireStreamConverter
	var streamApiErr *shared.NookMuxError
	if info.RelayFormat == relayconstant.RelayFormatClaude {
		responsesToChat = stream.NewResponsesToChatStreamConverter(false)
	}

	if streamErr := helper.StreamScannerHandler(c, resp, info, func(data string) bool {
		maskedData := data

		// 检查当前数据是否包含 completed 状态和 usage 信息
		var streamResponse shared.ResponsesStreamResponse
		if err := jsonx.UnmarshalJsonStr(data, &streamResponse); err == nil {
			// 上游在 HTTP 200 流内返回错误载荷：官方 type:"error" 事件、
			// 部分网关转成 200 下发的裸 {"error":{...}} 帧、response.failed
			// 事件（错误可能在顶层或 response 内）。识别后保留真实上游错误，
			// 避免计费阶段因 totalTokens=0 被误记为
			// 「502 上游没有返回计费信息」。
			if streamApiErr == nil {
				if oaiError := streamResponse.GetOpenAIError(); oaiError != nil {
					streamApiErr = shared.WithOpenAIError(*oaiError, upstreamErrorStatusCode(resp.StatusCode, oaiError))
					return false
				}
			}
			helper.MaskResponsesStreamResponseModel(&streamResponse, info)
			maskedData = string(helper.MaskResponseEventModelJSON(jsonx.StringToByteSlice(data), info))
			switch streamResponse.Type {
			case "response.completed":
				if streamResponse.Response != nil {
					if streamResponse.Response.Usage != nil {
						convert.ApplyResponsesUsageToChatUsage(usage, streamResponse.Response.Usage)
						if info.RelayFormat == relayconstant.RelayFormatClaude && info.ClaudeConvertInfo != nil {
							info.ClaudeConvertInfo.Usage = usage
						}
					}
					if streamResponse.Response.HasImageGenerationCall() {
						c.Set("image_generation_call", true)
						c.Set("image_generation_call_quality", streamResponse.Response.GetQuality())
						c.Set("image_generation_call_size", streamResponse.Response.GetSize())
					}
				}
			case "response.output_text.delta":
				// 处理输出文本
				responseTextBuilder.WriteString(streamResponse.Delta)
			case shared.ResponsesOutputTypeItemDone:
				// 内置工具调用计数
				if streamResponse.Item != nil && info != nil && info.ResponsesUsageInfo != nil && info.ResponsesUsageInfo.BuiltInTools != nil {
					switch streamResponse.Item.Type {
					case shared.BuildInCallWebSearchCall:
						if webSearchTool, exists := info.ResponsesUsageInfo.BuiltInTools[shared.BuildInToolWebSearchPreview]; exists && webSearchTool != nil {
							webSearchTool.CallCount++
						}
					case shared.BuildInCallFileSearchCall:
						if fileSearchTool, exists := info.ResponsesUsageInfo.BuiltInTools[shared.BuildInToolFileSearch]; exists && fileSearchTool != nil {
							fileSearchTool.CallCount++
						}
					}
				}
			}
		} else {
			log.LogError(c, "failed to unmarshal stream response: "+err.Error())
		}

		if info.RelayFormat == relayconstant.RelayFormatClaude {
			if err := writeResponsesStreamAsClaude(c, info, responsesToChat, maskedData); err != nil {
				log.LogError(c, "failed to convert responses stream to claude: "+err.Error())
				return false
			}
			return true
		}

		if streamResponse.Type != "" {
			sendResponsesStreamData(c, streamResponse, maskedData)
		}
		return true
	}); streamErr != nil {
		// 流异常终止（超时/连接断开）：记录真实原因供排查；已收到的部分内容
		// 仍按既有兜底估算计费，避免重试造成重复下发，故不向上返回错误。
		log.LogError(c, "responses stream terminated abnormally: "+streamErr.Error())
	}

	if streamApiErr != nil {
		// 上游在流内返回了 failed 事件：真实错误已识别，直接向上暴露，不再伪造 usage。
		helper.ResetStatusCode(streamApiErr, c.GetString("status_code_mapping"))
		return nil, streamApiErr
	}

	if usage.CompletionTokens == 0 {
		// 计算输出文本的 token 数量
		tempStr := responseTextBuilder.String()
		if len(tempStr) > 0 {
			// 非正常结束，使用输出文本的 token 数量
			completionTokens := tokenizer.CountTextToken(tempStr, info.UpstreamModelName)
			usage.CompletionTokens = completionTokens
		}
	}

	if usage.PromptTokens == 0 && usage.CompletionTokens != 0 {
		usage.PromptTokens = info.GetEstimatePromptTokens()
	}

	if usage.TotalTokens == 0 {
		usage.TotalTokens = usage.PromptTokens + usage.CompletionTokens
	}

	return usage, nil
}

func convertResponsesBodyToClaudeBody(responsesResponse *shared.OpenAIResponsesResponse, usage *shared.Usage, info *relaycommon.RelayInfo) ([]byte, error) {
	cv := convert.NewConverter(shared.OpenAIWireAPIResponses, shared.OpenAIWireAPIChat)
	chatResponse, err := cv.ConvertResponsesToChatResponse(responsesResponse)
	if err != nil {
		return nil, err
	}
	if usage != nil {
		chatResponse.Usage = *usage
	}
	claudeResp := helper.ResponseOpenAI2Claude(chatResponse, info)
	return jsonx.Marshal(claudeResp)
}

func writeResponsesStreamAsClaude(c *gin.Context, info *relaycommon.RelayInfo, converter stream.OpenAIWireStreamConverter, data string) error {
	if converter == nil {
		return fmt.Errorf("responses to chat stream converter is nil")
	}
	if info.ClaudeConvertInfo == nil {
		info.ClaudeConvertInfo = &relaycommon.ClaudeConvertInfo{LastMessagesType: relaycommon.LastMessageTypeNone}
	}
	var streamResponse shared.ResponsesStreamResponse
	if err := jsonx.UnmarshalJsonStr(data, &streamResponse); err == nil && streamResponse.Response != nil && streamResponse.Response.Usage != nil {
		usage := &shared.Usage{}
		convert.ApplyResponsesUsageToChatUsage(usage, streamResponse.Response.Usage)
		info.ClaudeConvertInfo.Usage = usage
	}
	out, err := converter.ConvertFrame("", data, "data: "+data+"\n\n")
	if err != nil {
		return err
	}
	for _, chatData := range chatDataFrames(out) {
		if chatData == "[DONE]" {
			continue
		}
		info.SendResponseCount++
		if err := handleClaudeFormat(c, chatData, info); err != nil {
			return err
		}
	}
	return nil
}

func chatDataFrames(s string) []string {
	frames := strings.Split(strings.ReplaceAll(s, "\r\n", "\n"), "\n\n")
	out := make([]string, 0, len(frames))
	for _, frame := range frames {
		frame = strings.TrimSpace(frame)
		if frame == "" || strings.HasPrefix(frame, ":") {
			continue
		}
		var dataLines []string
		for _, line := range strings.Split(frame, "\n") {
			if strings.HasPrefix(line, "data:") {
				dataLines = append(dataLines, strings.TrimPrefix(strings.TrimPrefix(line, "data:"), " "))
			}
		}
		if len(dataLines) > 0 {
			out = append(out, strings.Join(dataLines, "\n"))
		}
	}
	return out
}
