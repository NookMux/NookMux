package openai

import (
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/NookMux/NookMux/internal/common"
	"github.com/NookMux/NookMux/internal/domain/shared"
	"github.com/NookMux/NookMux/internal/infra/log"
	"github.com/NookMux/NookMux/internal/relay/channel/openrouter"
	relaycommon "github.com/NookMux/NookMux/internal/relay/common"
	relayconstant "github.com/NookMux/NookMux/internal/relay/constant"
	"github.com/NookMux/NookMux/internal/relay/helper"
	"github.com/NookMux/NookMux/pkg/jsonx"

	billing "github.com/NookMux/NookMux/internal/domain/billing"
	channelconstant "github.com/NookMux/NookMux/internal/domain/channel/constant"
	"github.com/NookMux/NookMux/internal/httpapi"
	tokenizer "github.com/NookMux/NookMux/internal/infra/tokenizer"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

func sendStreamData(c *gin.Context, info *relaycommon.RelayInfo, data string, forceFormat bool) error {
	if data == "" {
		return nil
	}

	if !forceFormat {
		data = string(helper.MaskTopLevelModelJSON(jsonx.StringToByteSlice(data), info))
		return helper.StringData(c, data)
	}

	var lastStreamResponse shared.ChatCompletionsStreamResponse
	if err := jsonx.UnmarshalJsonStr(data, &lastStreamResponse); err != nil {
		return err
	}
	helper.MaskChatStreamResponseModel(&lastStreamResponse, info)

	return helper.ObjectData(c, lastStreamResponse)
}

func OaiStreamHandler(c *gin.Context, info *relaycommon.RelayInfo, resp *http.Response) (*shared.Usage, *shared.NookMuxError) {
	if resp == nil || resp.Body == nil {
		log.LogError(c, "invalid response or response body")
		return nil, shared.NewOpenAIError(fmt.Errorf("invalid response"), shared.ErrorCodeBadResponse, http.StatusInternalServerError)
	}

	defer helper.CloseResponseBodyGracefully(resp)

	model := info.UpstreamModelName
	var responseId string
	var createAt int64 = 0
	var systemFingerprint string
	var containStreamUsage bool
	var responseTextBuilder strings.Builder
	var toolCount int
	var usage = &shared.Usage{}
	var lastStreamData string
	var secondLastStreamData string // 存储倒数第二个stream data，用于音频模型
	var streamApiErr *shared.NookMuxError

	// 检查是否为音频模型
	isAudioModel := strings.Contains(strings.ToLower(model), "audio")

	if streamErr := helper.StreamScannerHandler(c, resp, info, func(data string) bool {
		// 部分上游/中间网关会把 429/5xx 错误转成 HTTP 200 + SSE error 帧下发。
		// 这里识别错误帧并保留真实上游错误，避免计费阶段因 totalTokens=0
		// 被误记为「502 上游没有返回计费信息」。
		if streamApiErr == nil && strings.Contains(data, `"error"`) {
			var errFrame struct {
				Error any `json:"error"`
			}
			if err := jsonx.UnmarshalJsonStr(data, &errFrame); err == nil && errFrame.Error != nil {
				if oaiError := shared.GetOpenAIError(errFrame.Error); oaiError != nil && oaiError.Message != "" {
					streamApiErr = shared.WithOpenAIError(*oaiError, upstreamErrorStatusCode(resp.StatusCode, oaiError))
					return false
				}
			}
		}
		if lastStreamData != "" {
			err := HandleStreamFormat(c, info, lastStreamData, info.ChannelSetting.ForceFormat)
			if err != nil {
				common.SysLog("error handling stream format: " + err.Error())
			}
			if err := ProcessStreamFrame(info.RelayMode, lastStreamData, &responseTextBuilder, &toolCount); err != nil {
				log.LogError(c, "error processing stream token frame: "+err.Error())
			}
		}
		if len(data) > 0 {
			// 对音频模型，保存倒数第二个stream data
			if isAudioModel && lastStreamData != "" {
				secondLastStreamData = lastStreamData
			}

			lastStreamData = data
		}
		return true
	}); streamErr != nil {
		// 流异常终止（超时/连接断开）：记录真实原因供排查；已收到的部分内容
		// 仍按既有兜底估算计费，避免重试造成重复下发，故不向上返回错误。
		log.LogError(c, "openai stream terminated abnormally: "+streamErr.Error())
	}

	// 对音频模型，从倒数第二个stream data中提取usage信息
	if isAudioModel && secondLastStreamData != "" {
		var streamResp struct {
			Usage *shared.Usage `json:"usage"`
		}
		err := jsonx.Unmarshal([]byte(secondLastStreamData), &streamResp)
		if err == nil && streamResp.Usage != nil && billing.ValidUsage(streamResp.Usage) {
			usage = streamResp.Usage
			containStreamUsage = true

			if common.DebugEnabled {
				log.LogDebug(c, fmt.Sprintf("Audio model usage extracted from second last SSE: PromptTokens=%d, CompletionTokens=%d, TotalTokens=%d, InputTokens=%d, OutputTokens=%d",
					usage.PromptTokens, usage.CompletionTokens, usage.TotalTokens,
					usage.InputTokens, usage.OutputTokens))
			}
		}
	}

	if streamApiErr != nil {
		// 上游在流内返回了错误帧：真实错误已识别，直接向上暴露，不再伪造 usage。
		helper.ResetStatusCode(streamApiErr, c.GetString("status_code_mapping"))
		return nil, streamApiErr
	}

	// 处理最后的响应
	shouldSendLastResp := true
	if err := handleLastResponse(lastStreamData, &responseId, &createAt, &systemFingerprint, &model, &usage,
		&containStreamUsage, info, &shouldSendLastResp); err != nil {
		log.LogError(c, fmt.Sprintf("error handling last response: %s, lastStreamData: [%s]", err.Error(), lastStreamData))
	}

	if info.RelayFormat == relayconstant.RelayFormatOpenAI {
		if shouldSendLastResp {
			_ = sendStreamData(c, info, lastStreamData, info.ChannelSetting.ForceFormat)
		}
	}

	if !containStreamUsage && lastStreamData != "" {
		if err := ProcessStreamFrame(info.RelayMode, lastStreamData, &responseTextBuilder, &toolCount); err != nil {
			log.LogError(c, "error processing final stream token frame: "+err.Error())
		}
	}

	if !containStreamUsage {
		usage = billing.ResponseText2Usage(c, responseTextBuilder.String(), info.UpstreamModelName, info.GetEstimatePromptTokens())
		usage.CompletionTokens += toolCount * 7
	}

	applyUsagePostProcessing(info, usage, jsonx.StringToByteSlice(lastStreamData))

	HandleFinalResponse(c, info, lastStreamData, responseId, createAt, model, systemFingerprint, usage, containStreamUsage)

	return usage, nil
}

func OpenaiHandler(c *gin.Context, info *relaycommon.RelayInfo, resp *http.Response) (*shared.Usage, *shared.NookMuxError) {
	defer helper.CloseResponseBodyGracefully(resp)

	var simpleResponse shared.OpenAITextResponse
	responseBody, err := readOpenAIResponseBody(info, resp.Body)
	if err != nil {
		return nil, shared.NewOpenAIError(err, shared.ErrorCodeReadResponseBodyFailed, http.StatusInternalServerError)
	}
	if common.DebugEnabled {
		println("upstream response body:", string(responseBody))
	}
	// Unmarshal to simpleResponse
	if info.ChannelType == channelconstant.ChannelTypeOpenRouter && info.ChannelOtherSettings.IsOpenRouterEnterprise() {
		// 尝试解析为 openrouter enterprise
		var enterpriseResponse openrouter.OpenRouterEnterpriseResponse
		err = jsonx.Unmarshal(responseBody, &enterpriseResponse)
		if err != nil {
			return nil, shared.NewOpenAIError(err, shared.ErrorCodeBadResponseBody, http.StatusInternalServerError)
		}
		if enterpriseResponse.Success {
			responseBody = enterpriseResponse.Data
		} else {
			log.LogError(c, fmt.Sprintf("openrouter enterprise response success=false, data: %s", enterpriseResponse.Data))
			return nil, shared.NewOpenAIError(fmt.Errorf("openrouter response success=false"), shared.ErrorCodeBadResponseBody, http.StatusInternalServerError)
		}
	}

	err = jsonx.Unmarshal(responseBody, &simpleResponse)
	if err != nil {
		return nil, shared.NewOpenAIError(err, shared.ErrorCodeBadResponseBody, http.StatusInternalServerError)
	}

	if oaiError := simpleResponse.GetOpenAIError(); oaiError != nil && oaiError.Message != "" {
		return nil, shared.WithOpenAIError(*oaiError, upstreamErrorStatusCode(resp.StatusCode, oaiError))
	}

	for _, choice := range simpleResponse.Choices {
		if choice.FinishReason == relayconstant.FinishReasonContentFilter {
			httpapi.SetContextKey(c, common.ContextKeyAdminRejectReason, "openai_finish_reason=content_filter")
			break
		}
	}
	helper.MaskTextResponseModel(&simpleResponse, info)

	forceFormat := false
	if info.ChannelSetting.ForceFormat {
		forceFormat = true
	}

	usageModified := false
	if simpleResponse.Usage.PromptTokens == 0 {
		completionTokens := simpleResponse.Usage.CompletionTokens
		if completionTokens == 0 {
			for _, choice := range simpleResponse.Choices {
				ctkm := tokenizer.CountTextToken(choice.Message.StringContent()+choice.Message.GetReasoningContent(), info.UpstreamModelName)
				completionTokens += ctkm
			}
		}
		simpleResponse.Usage = shared.Usage{
			PromptTokens:     info.GetEstimatePromptTokens(),
			CompletionTokens: completionTokens,
			TotalTokens:      info.GetEstimatePromptTokens() + completionTokens,
		}
		usageModified = true
	}

	applyUsagePostProcessing(info, &simpleResponse.Usage, responseBody)

	switch info.RelayFormat {
	case relayconstant.RelayFormatOpenAI:
		if usageModified {
			var bodyMap map[string]interface{}
			err = jsonx.Unmarshal(responseBody, &bodyMap)
			if err != nil {
				return nil, shared.NewOpenAIError(err, shared.ErrorCodeBadResponseBody, http.StatusInternalServerError)
			}
			bodyMap["usage"] = simpleResponse.Usage
			responseBody, _ = jsonx.Marshal(bodyMap)
		}
		if forceFormat {
			responseBody, err = jsonx.Marshal(simpleResponse)
			if err != nil {
				return nil, shared.NewError(err, shared.ErrorCodeBadResponseBody)
			}
		} else {
			responseBody = helper.MaskTopLevelModelJSON(responseBody, info)
			break
		}
	case relayconstant.RelayFormatClaude:
		claudeResp := helper.ResponseOpenAI2Claude(&simpleResponse, info)
		claudeRespStr, err := jsonx.Marshal(claudeResp)
		if err != nil {
			return nil, shared.NewError(err, shared.ErrorCodeBadResponseBody)
		}
		responseBody = claudeRespStr
	case relayconstant.RelayFormatGemini:
		geminiResp := helper.ResponseOpenAI2Gemini(&simpleResponse, info)
		geminiRespStr, err := jsonx.Marshal(geminiResp)
		if err != nil {
			return nil, shared.NewError(err, shared.ErrorCodeBadResponseBody)
		}
		responseBody = geminiRespStr
	}

	helper.IOCopyBytesGracefully(c, resp, responseBody)

	return &simpleResponse.Usage, nil
}

func OpenaiRealtimeHandler(c *gin.Context, info *relaycommon.RelayInfo) (*shared.NookMuxError, *shared.RealtimeUsage) {
	if info == nil || info.ClientWs == nil || info.TargetWs == nil {
		return shared.NewError(fmt.Errorf("invalid websocket connection"), shared.ErrorCodeBadResponse), nil
	}

	info.IsStream = true
	clientConn := info.ClientWs
	targetConn := info.TargetWs

	clientClosed := make(chan struct{})
	targetClosed := make(chan struct{})
	errChan := make(chan error, 2)

	usage := &shared.RealtimeUsage{}
	localUsage := &shared.RealtimeUsage{}
	sumUsage := &shared.RealtimeUsage{}

	go func() {
		defer func() {
			if r := recover(); r != nil {
				errChan <- fmt.Errorf("panic in client reader: %v", r)
			}
		}()
		for {
			select {
			case <-c.Done():
				return
			default:
				_, message, err := clientConn.ReadMessage()
				if err != nil {
					if !websocket.IsCloseError(err, websocket.CloseNormalClosure, websocket.CloseGoingAway) {
						errChan <- fmt.Errorf("error reading from client: %v", err)
					}
					close(clientClosed)
					return
				}

				realtimeEvent := &shared.RealtimeEvent{}
				err = jsonx.Unmarshal(message, realtimeEvent)
				if err != nil {
					errChan <- fmt.Errorf("error unmarshalling message: %v", err)
					return
				}

				if realtimeEvent.Type == shared.RealtimeEventTypeSessionUpdate {
					if realtimeEvent.Session != nil {
						if realtimeEvent.Session.Tools != nil {
							info.RealtimeTools = realtimeEvent.Session.Tools
						}
					}
				}

				textToken, audioToken, err := tokenizer.CountTokenRealtime(info, *realtimeEvent, info.UpstreamModelName)
				if err != nil {
					errChan <- fmt.Errorf("error counting text token: %v", err)
					return
				}
				log.LogDebug(c, "realtime event type=%s textToken=%d audioToken=%d", realtimeEvent.Type, textToken, audioToken)
				localUsage.TotalTokens += textToken + audioToken
				localUsage.InputTokens += textToken + audioToken
				localUsage.InputTokenDetails.TextTokens += textToken
				localUsage.InputTokenDetails.AudioTokens += audioToken

				err = helper.WssString(c, targetConn, string(message))
				if err != nil {
					errChan <- fmt.Errorf("error writing to target: %v", err)
					return
				}
			}
		}
	}()

	go func() {
		defer func() {
			if r := recover(); r != nil {
				errChan <- fmt.Errorf("panic in target reader: %v", r)
			}
		}()
		for {
			select {
			case <-c.Done():
				return
			default:
				_, message, err := targetConn.ReadMessage()
				if err != nil {
					if !websocket.IsCloseError(err, websocket.CloseNormalClosure, websocket.CloseGoingAway) {
						errChan <- fmt.Errorf("error reading from target: %v", err)
					}
					close(targetClosed)
					return
				}
				info.SetFirstResponseTime()
				realtimeEvent := &shared.RealtimeEvent{}
				err = jsonx.Unmarshal(message, realtimeEvent)
				if err != nil {
					errChan <- fmt.Errorf("error unmarshalling message: %v", err)
					return
				}

				if realtimeEvent.Type == shared.RealtimeEventTypeResponseDone {
					realtimeUsage := realtimeEvent.Response.Usage
					if realtimeUsage != nil {
						usage.TotalTokens += realtimeUsage.TotalTokens
						usage.InputTokens += realtimeUsage.InputTokens
						usage.OutputTokens += realtimeUsage.OutputTokens
						usage.InputTokenDetails.AudioTokens += realtimeUsage.InputTokenDetails.AudioTokens
						usage.InputTokenDetails.CachedTokens += realtimeUsage.InputTokenDetails.CachedTokens
						usage.InputTokenDetails.TextTokens += realtimeUsage.InputTokenDetails.TextTokens
						usage.OutputTokenDetails.AudioTokens += realtimeUsage.OutputTokenDetails.AudioTokens
						usage.OutputTokenDetails.TextTokens += realtimeUsage.OutputTokenDetails.TextTokens
						err := preConsumeUsage(c, info, usage, sumUsage)
						if err != nil {
							errChan <- fmt.Errorf("error consume usage: %v", err)
							return
						}
						// 本次计费完成，清除
						usage = &shared.RealtimeUsage{}

						localUsage = &shared.RealtimeUsage{}
					} else {
						textToken, audioToken, err := tokenizer.CountTokenRealtime(info, *realtimeEvent, info.UpstreamModelName)
						if err != nil {
							errChan <- fmt.Errorf("error counting text token: %v", err)
							return
						}
						log.LogDebug(c, "realtime event type=%s textToken=%d audioToken=%d", realtimeEvent.Type, textToken, audioToken)
						localUsage.TotalTokens += textToken + audioToken
						info.IsFirstRequest = false
						localUsage.InputTokens += textToken + audioToken
						localUsage.InputTokenDetails.TextTokens += textToken
						localUsage.InputTokenDetails.AudioTokens += audioToken
						err = preConsumeUsage(c, info, localUsage, sumUsage)
						if err != nil {
							errChan <- fmt.Errorf("error consume usage: %v", err)
							return
						}
						// 本次计费完成，清除
						localUsage = &shared.RealtimeUsage{}
						// print now usage
					}
					log.LogDebug(c, "realtime streaming sumUsage=%v localUsage=%v", sumUsage, localUsage)

				} else if realtimeEvent.Type == shared.RealtimeEventTypeSessionUpdated || realtimeEvent.Type == shared.RealtimeEventTypeSessionCreated {
					realtimeSession := realtimeEvent.Session
					if realtimeSession != nil {
						// update audio format
						info.InputAudioFormat = common.GetStringIfEmpty(realtimeSession.InputAudioFormat, info.InputAudioFormat)
						info.OutputAudioFormat = common.GetStringIfEmpty(realtimeSession.OutputAudioFormat, info.OutputAudioFormat)
					}
				} else {
					textToken, audioToken, err := tokenizer.CountTokenRealtime(info, *realtimeEvent, info.UpstreamModelName)
					if err != nil {
						errChan <- fmt.Errorf("error counting text token: %v", err)
						return
					}
					log.LogDebug(c, "realtime event type=%s textToken=%d audioToken=%d", realtimeEvent.Type, textToken, audioToken)
					localUsage.TotalTokens += textToken + audioToken
					localUsage.OutputTokens += textToken + audioToken
					localUsage.OutputTokenDetails.TextTokens += textToken
					localUsage.OutputTokenDetails.AudioTokens += audioToken
				}

				message = helper.MaskRealtimeEventModelJSON(message, info)
				err = helper.WssString(c, clientConn, string(message))
				if err != nil {
					errChan <- fmt.Errorf("error writing to client: %v", err)
					return
				}
			}
		}
	}()

	select {
	case <-clientClosed:
	case <-targetClosed:
	case err := <-errChan:
		//return shared.OpenAIErrorWrapper(err, "realtime_error", http.StatusInternalServerError), nil
		log.LogError(c, "realtime error: "+err.Error())
	case <-c.Done():
	}

	if usage.TotalTokens != 0 {
		_ = preConsumeUsage(c, info, usage, sumUsage)
	}

	if localUsage.TotalTokens != 0 {
		_ = preConsumeUsage(c, info, localUsage, sumUsage)
	}

	// check usage total tokens, if 0, use local usage

	return nil, sumUsage
}

func preConsumeUsage(ctx *gin.Context, info *relaycommon.RelayInfo, usage *shared.RealtimeUsage, totalUsage *shared.RealtimeUsage) error {
	if usage == nil || totalUsage == nil {
		return fmt.Errorf("invalid usage pointer")
	}

	totalUsage.TotalTokens += usage.TotalTokens
	totalUsage.InputTokens += usage.InputTokens
	totalUsage.OutputTokens += usage.OutputTokens
	totalUsage.InputTokenDetails.CachedTokens += usage.InputTokenDetails.CachedTokens
	totalUsage.InputTokenDetails.TextTokens += usage.InputTokenDetails.TextTokens
	totalUsage.InputTokenDetails.AudioTokens += usage.InputTokenDetails.AudioTokens
	totalUsage.OutputTokenDetails.TextTokens += usage.OutputTokenDetails.TextTokens
	totalUsage.OutputTokenDetails.AudioTokens += usage.OutputTokenDetails.AudioTokens
	// clear usage
	err := billing.PreWssConsumeQuota(ctx, info, usage)
	return err
}

func OpenaiHandlerWithUsage(c *gin.Context, info *relaycommon.RelayInfo, resp *http.Response) (*shared.Usage, *shared.NookMuxError) {
	defer helper.CloseResponseBodyGracefully(resp)

	responseBody, err := helper.ReadMediaResponseBody(resp.Body)
	if err != nil {
		return nil, shared.NewOpenAIError(err, shared.ErrorCodeReadResponseBodyFailed, http.StatusInternalServerError)
	}

	// 部分上游/中间网关会把 429/5xx 错误转成 HTTP 200 + error body 下发。
	// 识别后向上暴露真实上游错误，避免计费阶段因 usage 全零被误记为
	// 「502 上游没有返回计费信息」。
	var errProbe shared.SimpleResponse
	if probeErr := jsonx.Unmarshal(responseBody, &errProbe); probeErr == nil {
		if oaiError := errProbe.GetOpenAIError(); oaiError != nil && oaiError.Message != "" {
			return nil, shared.WithOpenAIError(*oaiError, upstreamErrorStatusCode(resp.StatusCode, oaiError))
		}
	}

	var usageResp shared.SimpleResponse
	err = jsonx.Unmarshal(responseBody, &usageResp)
	if err != nil {
		return nil, shared.NewOpenAIError(err, shared.ErrorCodeBadResponseBody, http.StatusInternalServerError)
	}

	responseBody = helper.MaskTopLevelModelJSON(responseBody, info)

	// 写入新的 response body
	helper.IOCopyBytesGracefully(c, resp, responseBody)

	// Once we've written to the client, we should not return errors anymore
	// because the upstream has already consumed resources and returned content
	// We should still perform billing even if parsing fails
	// format
	if usageResp.InputTokens > 0 {
		usageResp.PromptTokens += usageResp.InputTokens
	}
	if usageResp.OutputTokens > 0 {
		usageResp.CompletionTokens += usageResp.OutputTokens
	}
	if usageResp.InputTokensDetails != nil {
		usageResp.PromptTokensDetails.ImageTokens += usageResp.InputTokensDetails.ImageTokens
		usageResp.PromptTokensDetails.TextTokens += usageResp.InputTokensDetails.TextTokens
	}
	applyUsagePostProcessing(info, &usageResp.Usage, responseBody)
	return &usageResp.Usage, nil
}

func readOpenAIResponseBody(info *relaycommon.RelayInfo, body io.Reader) ([]byte, error) {
	if info != nil && info.RelayMode == relayconstant.RelayModeEmbeddings {
		return helper.ReadEmbeddingResponseBody(body)
	}
	return helper.ReadResponseBody(body)
}

func applyUsagePostProcessing(info *relaycommon.RelayInfo, usage *shared.Usage, responseBody []byte) {
	if info == nil || usage == nil {
		return
	}

	switch info.ChannelType {
	case channelconstant.ChannelTypeDeepSeek:
		if usage.PromptTokensDetails.CachedTokens == 0 && usage.PromptCacheHitTokens != 0 {
			usage.PromptTokensDetails.CachedTokens = usage.PromptCacheHitTokens
		}
	case channelconstant.ChannelTypeZhipu_v4:
		// 智普的cached_tokens在标准位置: usage.prompt_tokens_details.cached_tokens
		if usage.PromptTokensDetails.CachedTokens == 0 {
			if usage.InputTokensDetails != nil && usage.InputTokensDetails.CachedTokens > 0 {
				usage.PromptTokensDetails.CachedTokens = usage.InputTokensDetails.CachedTokens
			} else if cachedTokens, ok := extractCachedTokensFromBody(responseBody); ok {
				usage.PromptTokensDetails.CachedTokens = cachedTokens
			} else if usage.PromptCacheHitTokens > 0 {
				usage.PromptTokensDetails.CachedTokens = usage.PromptCacheHitTokens
			}
		}
	case channelconstant.ChannelTypeMoonshot:
		// Moonshot的cached_tokens在非标准位置: choices[].usage.cached_tokens
		if usage.PromptTokensDetails.CachedTokens == 0 {
			if usage.InputTokensDetails != nil && usage.InputTokensDetails.CachedTokens > 0 {
				usage.PromptTokensDetails.CachedTokens = usage.InputTokensDetails.CachedTokens
			} else if cachedTokens, ok := extractMoonshotCachedTokensFromBody(responseBody); ok {
				usage.PromptTokensDetails.CachedTokens = cachedTokens
			} else if cachedTokens, ok := extractCachedTokensFromBody(responseBody); ok {
				usage.PromptTokensDetails.CachedTokens = cachedTokens
			} else if usage.PromptCacheHitTokens > 0 {
				usage.PromptTokensDetails.CachedTokens = usage.PromptCacheHitTokens
			}
		}
	case channelconstant.ChannelTypeOpenAI:
		if usage.PromptTokensDetails.CachedTokens == 0 {
			if cachedTokens, ok := extractLlamaCachedTokensFromBody(responseBody); ok {
				usage.PromptTokensDetails.CachedTokens = cachedTokens
			}
		}
	}
}

func extractCachedTokensFromBody(body []byte) (int, bool) {
	if len(body) == 0 {
		return 0, false
	}

	var payload struct {
		Usage struct {
			PromptTokensDetails struct {
				CachedTokens *int `json:"cached_tokens"`
			} `json:"prompt_tokens_details"`
			CachedTokens         *int `json:"cached_tokens"`
			PromptCacheHitTokens *int `json:"prompt_cache_hit_tokens"`
		} `json:"usage"`
	}

	if err := jsonx.Unmarshal(body, &payload); err != nil {
		return 0, false
	}

	if payload.Usage.PromptTokensDetails.CachedTokens != nil {
		return *payload.Usage.PromptTokensDetails.CachedTokens, true
	}
	if payload.Usage.CachedTokens != nil {
		return *payload.Usage.CachedTokens, true
	}
	if payload.Usage.PromptCacheHitTokens != nil {
		return *payload.Usage.PromptCacheHitTokens, true
	}
	return 0, false
}

// extractMoonshotCachedTokensFromBody 从Moonshot的非标准位置提取cached_tokens
// Moonshot的流式响应格式: {"choices":[{"usage":{"cached_tokens":111}}]}
func extractMoonshotCachedTokensFromBody(body []byte) (int, bool) {
	if len(body) == 0 {
		return 0, false
	}

	var payload struct {
		Choices []struct {
			Usage struct {
				CachedTokens *int `json:"cached_tokens"`
			} `json:"usage"`
		} `json:"choices"`
	}

	if err := jsonx.Unmarshal(body, &payload); err != nil {
		return 0, false
	}

	// 遍历choices查找cached_tokens
	for _, choice := range payload.Choices {
		if choice.Usage.CachedTokens != nil && *choice.Usage.CachedTokens > 0 {
			return *choice.Usage.CachedTokens, true
		}
	}

	return 0, false
}

// extractLlamaCachedTokensFromBody 从 llama.cpp/vLLM 兼容响应的非标准 timings.cache_n 提取缓存命中 token。
func extractLlamaCachedTokensFromBody(body []byte) (int, bool) {
	if len(body) == 0 {
		return 0, false
	}

	var payload struct {
		Timings struct {
			CachedTokens *int `json:"cache_n"`
		} `json:"timings"`
	}

	if err := jsonx.Unmarshal(body, &payload); err != nil {
		return 0, false
	}
	if payload.Timings.CachedTokens == nil {
		return 0, false
	}
	return *payload.Timings.CachedTokens, true
}
