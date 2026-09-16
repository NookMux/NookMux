package openai

import (
	"strings"

	"github.com/NookMux/NookMux/internal/common"
	"github.com/NookMux/NookMux/internal/domain/shared"
	"github.com/NookMux/NookMux/internal/infra/log"
	relaycommon "github.com/NookMux/NookMux/internal/relay/common"
	relayconstant "github.com/NookMux/NookMux/internal/relay/constant"
	"github.com/NookMux/NookMux/internal/relay/helper"

	"github.com/NookMux/NookMux/pkg/jsonx"

	"github.com/samber/lo"

	billing "github.com/NookMux/NookMux/internal/domain/billing"
	"github.com/gin-gonic/gin"
)

// upstreamErrorStatusCode 见 helper.UpstreamErrorStatusCode：还原被网关
// 转成 HTTP 200 的真实上游状态码，无法还原时回退 502（保持可重试语义）。
func upstreamErrorStatusCode(httpStatusCode int, oaiError *shared.OpenAIError) int {
	if oaiError == nil {
		return helper.UpstreamErrorStatusCode(httpStatusCode, nil)
	}
	return helper.UpstreamErrorStatusCode(httpStatusCode, oaiError.Code)
}

// HandleStreamFormat 按下游协议格式转发流式帧。
func HandleStreamFormat(c *gin.Context, info *relaycommon.RelayInfo, data string, forceFormat bool) error {
	info.SendResponseCount++

	switch info.RelayFormat {
	case relayconstant.RelayFormatOpenAI:
		return sendStreamData(c, info, data, forceFormat)
	case relayconstant.RelayFormatClaude:
		return handleClaudeFormat(c, data, info)
	case relayconstant.RelayFormatGemini:
		return handleGeminiFormat(c, data, info)
	}
	return nil
}

func handleClaudeFormat(c *gin.Context, data string, info *relaycommon.RelayInfo) error {
	var streamResponse shared.ChatCompletionsStreamResponse
	if err := jsonx.Unmarshal(jsonx.StringToByteSlice(data), &streamResponse); err != nil {
		return err
	}
	helper.MaskChatStreamResponseModel(&streamResponse, info)

	if streamResponse.Usage != nil {
		info.ClaudeConvertInfo.Usage = streamResponse.Usage
	}
	claudeResponses := helper.StreamResponseOpenAI2Claude(&streamResponse, info)
	for _, resp := range claudeResponses {
		if err := helper.ClaudeData(c, *resp); err != nil {
			return err
		}
	}
	return nil
}

func handleGeminiFormat(c *gin.Context, data string, info *relaycommon.RelayInfo) error {
	var streamResponse shared.ChatCompletionsStreamResponse
	if err := jsonx.Unmarshal(jsonx.StringToByteSlice(data), &streamResponse); err != nil {
		log.LogError(c, "failed to unmarshal stream response: "+err.Error())
		return err
	}
	helper.MaskChatStreamResponseModel(&streamResponse, info)

	geminiResponse := helper.StreamResponseOpenAI2Gemini(&streamResponse, info)

	// 如果返回 nil，表示没有实际内容，跳过发送
	if geminiResponse == nil {
		return nil
	}

	geminiResponseStr, err := jsonx.Marshal(geminiResponse)
	if err != nil {
		log.LogError(c, "failed to marshal gemini response: "+err.Error())
		return err
	}

	// send gemini format response
	c.Render(-1, &common.CustomEvent{Data: "data: " + string(geminiResponseStr)})
	_ = helper.FlushWriter(c)
	return nil
}

func ProcessStreamResponse(streamResponse shared.ChatCompletionsStreamResponse, responseTextBuilder *strings.Builder, toolCount *int) error {
	for _, choice := range streamResponse.Choices {
		responseTextBuilder.WriteString(choice.Delta.GetContentString())
		responseTextBuilder.WriteString(choice.Delta.GetReasoningContent())
		if choice.Delta.ToolCalls != nil {
			if len(choice.Delta.ToolCalls) > *toolCount {
				*toolCount = len(choice.Delta.ToolCalls)
			}
			for _, tool := range choice.Delta.ToolCalls {
				responseTextBuilder.WriteString(tool.Function.Name)
				responseTextBuilder.WriteString(tool.Function.Arguments)
			}
		}
	}
	return nil
}

func ProcessStreamFrame(relayMode int, data string, responseTextBuilder *strings.Builder, toolCount *int) error {
	switch relayMode {
	case relayconstant.RelayModeChatCompletions:
		var streamResponse shared.ChatCompletionsStreamResponse
		if err := jsonx.Unmarshal(jsonx.StringToByteSlice(data), &streamResponse); err != nil {
			return err
		}
		return ProcessStreamResponse(streamResponse, responseTextBuilder, toolCount)
	case relayconstant.RelayModeCompletions:
		var streamResponse shared.CompletionsStreamResponse
		if err := jsonx.Unmarshal(jsonx.StringToByteSlice(data), &streamResponse); err != nil {
			return err
		}
		for _, choice := range streamResponse.Choices {
			responseTextBuilder.WriteString(choice.Text)
		}
	}
	return nil
}

func handleLastResponse(lastStreamData string, responseId *string, createAt *int64,
	systemFingerprint *string, model *string, usage **shared.Usage,
	containStreamUsage *bool, info *relaycommon.RelayInfo,
	shouldSendLastResp *bool) error {

	var lastStreamResponse shared.ChatCompletionsStreamResponse
	if err := jsonx.Unmarshal(jsonx.StringToByteSlice(lastStreamData), &lastStreamResponse); err != nil {
		return err
	}

	*responseId = lastStreamResponse.Id
	*createAt = lastStreamResponse.Created
	*systemFingerprint = lastStreamResponse.GetSystemFingerprint()
	*model = lastStreamResponse.Model

	if billing.ValidUsage(lastStreamResponse.Usage) {
		*containStreamUsage = true
		*usage = lastStreamResponse.Usage
		if !info.ShouldIncludeUsage {
			*shouldSendLastResp = lo.SomeBy(lastStreamResponse.Choices, func(choice shared.ChatCompletionsStreamResponseChoice) bool {
				return choice.Delta.GetContentString() != "" || choice.Delta.GetReasoningContent() != ""
			})
		}
	}

	return nil
}

func HandleFinalResponse(c *gin.Context, info *relaycommon.RelayInfo, lastStreamData string,
	responseId string, createAt int64, model string, systemFingerprint string,
	usage *shared.Usage, containStreamUsage bool) {

	switch info.RelayFormat {
	case relayconstant.RelayFormatOpenAI:
		if info.ShouldIncludeUsage && !containStreamUsage {
			responseModel := model
			if info.GetResponseModelName() != "" {
				responseModel = info.GetResponseModelName()
			}
			response := helper.GenerateFinalUsageResponse(responseId, createAt, responseModel, *usage)
			response.SetSystemFingerprint(systemFingerprint)
			if err := helper.ObjectData(c, response); err != nil {
				log.LogError(c, "failed to write final usage response: "+err.Error())
			}
		}
		helper.Done(c)

	case relayconstant.RelayFormatClaude:
		var streamResponse shared.ChatCompletionsStreamResponse
		if err := jsonx.Unmarshal(jsonx.StringToByteSlice(lastStreamData), &streamResponse); err != nil {
			common.SysLog("error unmarshalling stream response: " + err.Error())
			return
		}
		helper.MaskChatStreamResponseModel(&streamResponse, info)

		info.ClaudeConvertInfo.Usage = usage

		claudeResponses := helper.StreamResponseOpenAI2Claude(&streamResponse, info)
		for _, resp := range claudeResponses {
			_ = helper.ClaudeData(c, *resp)
		}
		info.ClaudeConvertInfo.Done = true

	case relayconstant.RelayFormatGemini:
		var streamResponse shared.ChatCompletionsStreamResponse
		if err := jsonx.Unmarshal(jsonx.StringToByteSlice(lastStreamData), &streamResponse); err != nil {
			common.SysLog("error unmarshalling stream response: " + err.Error())
			return
		}
		helper.MaskChatStreamResponseModel(&streamResponse, info)

		// 这里处理的是 openai 最后一个流响应，其 delta 为空，有 finish_reason 字段
		// 因此相比较于 google 官方的流响应，由 openai 转换而来会多一个 parts 为空，finishReason 为 STOP 的响应
		// 而包含最后一段文本输出的响应（倒数第二个）的 finishReason 为 null
		// 暂不知是否有程序会不兼容。

		geminiResponse := helper.StreamResponseOpenAI2Gemini(&streamResponse, info)

		// openai 流响应开头的空数据
		if geminiResponse == nil {
			return
		}

		geminiResponseStr, err := jsonx.Marshal(geminiResponse)
		if err != nil {
			common.SysLog("error marshalling gemini response: " + err.Error())
			return
		}

		// 发送最终的 Gemini 响应
		c.Render(-1, &common.CustomEvent{Data: "data: " + string(geminiResponseStr)})
		_ = helper.FlushWriter(c)
	}
}

func sendResponsesStreamData(c *gin.Context, streamResponse shared.ResponsesStreamResponse, data string) {
	if data == "" {
		return
	}
	helper.ResponseChunkData(c, streamResponse, data)
}
