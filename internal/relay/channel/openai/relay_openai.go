package openai

import (
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"

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
	info.UsageSource = relayconstant.UsageSourceOpenAIChat
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

	helper.StreamScannerHandler(c, resp, info, func(data string) bool {
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
	})

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

	var serviceTierFrame struct {
		ServiceTier string `json:"service_tier"`
	}
	if jsonx.UnmarshalJsonStr(lastStreamData, &serviceTierFrame) == nil {
		info.SetEffectiveServiceTier(serviceTierFrame.ServiceTier)
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
	info.UsageSource = relayconstant.UsageSourceOpenAIChat
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
	info.SetEffectiveServiceTier(simpleResponse.ServiceTier)

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
		// usage 为本地估算，不属于上游 Token 用量，billing_details 不落列。
		httpapi.SetContextKey(c, common.ContextKeyLocalCountTokens, true)
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

// realtimeMeterEvent 是 reader 转交结算循环的计量事件。reader 只负责读、
// 解析与转发；token 计数、计量状态（pendingUsage/localUsage/sumUsage）与
// 事件计费全部收敛到结算循环单一属主，消除双向读取与收尾之间的数据竞态
// （P1-14）。
type realtimeMeterEvent struct {
	fromClient bool
	event      *shared.RealtimeEvent
}

func OpenaiRealtimeHandler(c *gin.Context, info *relaycommon.RelayInfo) (*shared.NookMuxError, *shared.RealtimeUsage) {
	// Realtime usage（input_tokens/input_token_details/output_tokens）与
	// Responses 同族，归一化按 openai_responses 规则。
	info.UsageSource = relayconstant.UsageSourceOpenAIResponses
	if info == nil || info.ClientWs == nil || info.TargetWs == nil {
		return shared.NewError(fmt.Errorf("invalid websocket connection"), shared.ErrorCodeBadResponse), nil
	}

	info.IsStream = true
	clientConn := info.ClientWs
	targetConn := info.TargetWs

	clientClosed := make(chan struct{})
	targetClosed := make(chan struct{})
	// errChan 缓冲 3：两个 reader 与结算循环各至多投递一次错误，投递方
	// 永不阻塞退出。
	errChan := make(chan error, 3)
	meterChan := make(chan realtimeMeterEvent, 64)

	var readerWG sync.WaitGroup
	readerWG.Add(2)

	// client reader：读客户端事件并转发上游，计量经 meterChan 交给结算循环。
	go func() {
		defer readerWG.Done()
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

				meterChan <- realtimeMeterEvent{fromClient: true, event: realtimeEvent}

				err = helper.WssString(c, targetConn, string(message))
				if err != nil {
					errChan <- fmt.Errorf("error writing to target: %v", err)
					return
				}
			}
		}
	}()

	// target reader：读上游事件并转发客户端，计量经 meterChan 交给结算循环。
	go func() {
		defer readerWG.Done()
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

				// 计量、计费与会话元数据（音频格式）的更新全部收敛到结算
				// 循环（单一属主），reader 只负责读、解析与转发。

				meterChan <- realtimeMeterEvent{fromClient: false, event: realtimeEvent}

				message = helper.MaskRealtimeEventModelJSON(message, info)
				err = helper.WssString(c, clientConn, string(message))
				if err != nil {
					errChan <- fmt.Errorf("error writing to client: %v", err)
					return
				}
			}
		}
	}()

	// 结算循环：pendingUsage/localUsage/sumUsage、事件计费与会话元数据
	// （音频格式、RealtimeTools）的唯一属主。实扣成功后才把该份用量累计进
	// sumUsage；失败记录 settleErr 并经 errChan 上抛一次，随后转入 drain
	// 模式继续消费 meterChan 但不再处理——reader 的投递永不阻塞，否则父
	// goroutine 的 readerWG.Wait() 会永久挂死（缺陷 33）；该份用量不累计、
	// 不重试（P1-12：不重复结算、失败可观测）。
	var sumUsage shared.RealtimeUsage
	pendingUsage := &shared.RealtimeUsage{}
	localUsage := &shared.RealtimeUsage{}
	settleDone := make(chan struct{})
	var settleErr error
	// mixedLocalCount 标记会话内是否混入本地估算计数：由结算循环写、父
	// goroutine 在 join（<-settleDone）后读。gin context 并发写不安全，
	// 本地计数标志统一由父 goroutine 在 join 后设置，而不是在结算循环内
	// 与 reader 的日志读取并发写 context。
	var mixedLocalCount bool
	go func() {
		defer close(settleDone)
		for meterEvent := range meterChan {
			if settleErr != nil {
				// drain 模式：结算已失败，只消费不处理（见上方注释）。
				continue
			}
			realtimeEvent := meterEvent.event
			if meterEvent.fromClient {
				if realtimeEvent.Type == shared.RealtimeEventTypeSessionUpdate {
					if realtimeEvent.Session != nil {
						if realtimeEvent.Session.Tools != nil {
							info.RealtimeTools = realtimeEvent.Session.Tools
						}
					}
				}
				textToken, audioToken, err := tokenizer.CountTokenRealtime(info, *realtimeEvent, info.UpstreamModelName)
				if err != nil {
					settleErr = fmt.Errorf("error counting text token: %v", err)
					errChan <- settleErr
					continue
				}
				log.LogDebug(c, "realtime event type=%s textToken=%d audioToken=%d", realtimeEvent.Type, textToken, audioToken)
				localUsage.TotalTokens += textToken + audioToken
				localUsage.InputTokens += textToken + audioToken
				localUsage.InputTokenDetails.TextTokens += textToken
				localUsage.InputTokenDetails.AudioTokens += audioToken
				continue
			}

			switch realtimeEvent.Type {
			case shared.RealtimeEventTypeResponseDone:
				if realtimeEvent.Response != nil && realtimeEvent.Response.Usage != nil {
					accumulateRealtimeUsage(pendingUsage, realtimeEvent.Response.Usage)
					if err := preConsumeUsage(c, info, pendingUsage, &sumUsage); err != nil {
						settleErr = fmt.Errorf("error consume usage: %v", err)
						errChan <- settleErr
						continue
					}
					// 本次计费完成，清除
					pendingUsage = &shared.RealtimeUsage{}

					localUsage = &shared.RealtimeUsage{}
				} else {
					textToken, audioToken, err := tokenizer.CountTokenRealtime(info, *realtimeEvent, info.UpstreamModelName)
					if err != nil {
						settleErr = fmt.Errorf("error counting text token: %v", err)
						errChan <- settleErr
						continue
					}
					log.LogDebug(c, "realtime event type=%s textToken=%d audioToken=%d", realtimeEvent.Type, textToken, audioToken)
					localUsage.TotalTokens += textToken + audioToken
					info.IsFirstRequest = false
					localUsage.InputTokens += textToken + audioToken
					localUsage.InputTokenDetails.TextTokens += textToken
					localUsage.InputTokenDetails.AudioTokens += audioToken
					// 上游 response.done 未携带 usage，本轮按本地 tokenizer 计数计费；
					// 会话内混入本地估算，billing_details 不落列（标志由父
					// goroutine 在 join 后统一设置）。
					mixedLocalCount = true
					if err := preConsumeUsage(c, info, localUsage, &sumUsage); err != nil {
						settleErr = fmt.Errorf("error consume usage: %v", err)
						errChan <- settleErr
						continue
					}
					// 本次计费完成，清除
					localUsage = &shared.RealtimeUsage{}
				}
				log.LogDebug(c, "realtime streaming sumUsage=%v localUsage=%v", sumUsage, localUsage)
			case shared.RealtimeEventTypeSessionUpdated, shared.RealtimeEventTypeSessionCreated:
				// 会话元数据（音频格式）与计量同属结算循环单一属主，消除
				// target reader 写与结算循环经 CountTokenRealtime 读之间
				// 的无同步并发访问。
				if realtimeEvent.Session != nil {
					info.InputAudioFormat = common.GetStringIfEmpty(realtimeEvent.Session.InputAudioFormat, info.InputAudioFormat)
					info.OutputAudioFormat = common.GetStringIfEmpty(realtimeEvent.Session.OutputAudioFormat, info.OutputAudioFormat)
				}
			default:
				textToken, audioToken, err := tokenizer.CountTokenRealtime(info, *realtimeEvent, info.UpstreamModelName)
				if err != nil {
					settleErr = fmt.Errorf("error counting text token: %v", err)
					errChan <- settleErr
					continue
				}
				log.LogDebug(c, "realtime event type=%s textToken=%d audioToken=%d", realtimeEvent.Type, textToken, audioToken)
				localUsage.TotalTokens += textToken + audioToken
				localUsage.OutputTokens += textToken + audioToken
				localUsage.OutputTokenDetails.TextTokens += textToken
				localUsage.OutputTokenDetails.AudioTokens += audioToken
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

	// 主动关闭双向连接解除 reader 的 ReadMessage 阻塞，等待 reader 与结算
	// 循环全部退出后再独占处理收尾计量（P1-14：join 后无共享计量状态）。
	_ = clientConn.Close()
	_ = targetConn.Close()
	readerWG.Wait()
	close(meterChan)
	<-settleDone

	// 会话内事件计费失败：错误必须显式上抛（不可重试——整场会话已交付，
	// 重跑等于双重计费），不再静默吞掉（P1-12）。
	if settleErr != nil {
		log.LogError(c, "realtime settlement error: "+settleErr.Error()+
			fmt.Sprintf(", eventConsumedQuota=%d", info.WssEventConsumedQuota))
		return shared.NewError(settleErr, shared.ErrorCodeUpdateDataError, shared.ErrOptionWithSkipRetry()), &sumUsage
	}

	// 会话内混入过本地估算（response.done 无 usage 分支），或收尾仍有本地
	// 计数待结算：统一在此打本地计数标志——join 后父 goroutine 是 gin
	// context 的唯一写入者，结算循环内直接 Set 会与 reader 的日志读取
	// 并发读写 context。
	if mixedLocalCount || localUsage.TotalTokens != 0 {
		httpapi.SetContextKey(c, common.ContextKeyLocalCountTokens, true)
	}

	// 收尾剩余用量结算：pendingUsage 是官方 usage（不打本地标志），
	// localUsage 是本地计数（local 标志已在上方统一设置，billing_details
	// 不落列）。
	// 失败同样显式上抛并跳过重试（P1-12：不吞错、不重复结算）。
	if pendingUsage.TotalTokens != 0 {
		if err := preConsumeUsage(c, info, pendingUsage, &sumUsage); err != nil {
			log.LogError(c, "realtime closing consume failed: "+err.Error()+
				fmt.Sprintf(", eventConsumedQuota=%d", info.WssEventConsumedQuota))
			return shared.NewError(err, shared.ErrorCodeUpdateDataError, shared.ErrOptionWithSkipRetry()), &sumUsage
		}
		pendingUsage = &shared.RealtimeUsage{}
	}
	if localUsage.TotalTokens != 0 {
		// 连接结束时剩余未结算事件按本地计数计费；本地计数标志已在
		// 上方统一设置，billing_details 不落列。
		if err := preConsumeUsage(c, info, localUsage, &sumUsage); err != nil {
			log.LogError(c, "realtime closing consume failed: "+err.Error()+
				fmt.Sprintf(", eventConsumedQuota=%d", info.WssEventConsumedQuota))
			return shared.NewError(err, shared.ErrorCodeUpdateDataError, shared.ErrOptionWithSkipRetry()), &sumUsage
		}
		localUsage = &shared.RealtimeUsage{}
	}

	// 交给 WssHelper 的 PostWssConsumeQuota 做收尾补差与汇总落库。

	return nil, &sumUsage
}

func preConsumeUsage(ctx *gin.Context, info *relaycommon.RelayInfo, usage *shared.RealtimeUsage, totalUsage *shared.RealtimeUsage) error {
	if usage == nil || totalUsage == nil {
		return fmt.Errorf("invalid usage pointer")
	}

	// 实扣成功才累计进汇总（P1-12）：失败时该份用量不进入 sumUsage，
	// 连接收尾也不会再处理它，杜绝失败事件被重复累计。
	if err := billing.PreWssConsumeQuota(ctx, info, usage); err != nil {
		return err
	}
	accumulateRealtimeUsage(totalUsage, usage)
	return nil
}

func accumulateRealtimeUsage(dst *shared.RealtimeUsage, src *shared.RealtimeUsage) {
	dst.TotalTokens += src.TotalTokens
	dst.InputTokens += src.InputTokens
	dst.OutputTokens += src.OutputTokens
	dst.InputTokenDetails.CachedTokens += src.InputTokenDetails.CachedTokens
	dst.InputTokenDetails.CachedTokensPresent =
		dst.InputTokenDetails.CachedTokensPresent || src.InputTokenDetails.CachedTokensPresent
	dst.InputTokenDetails.TextTokens += src.InputTokenDetails.TextTokens
	dst.InputTokenDetails.TextTokensPresent =
		dst.InputTokenDetails.TextTokensPresent || src.InputTokenDetails.TextTokensPresent
	dst.InputTokenDetails.AudioTokens += src.InputTokenDetails.AudioTokens
	dst.InputTokenDetails.AudioTokensPresent =
		dst.InputTokenDetails.AudioTokensPresent || src.InputTokenDetails.AudioTokensPresent
	dst.OutputTokenDetails.TextTokens += src.OutputTokenDetails.TextTokens
	dst.OutputTokenDetails.TextTokensPresent =
		dst.OutputTokenDetails.TextTokensPresent || src.OutputTokenDetails.TextTokensPresent
	dst.OutputTokenDetails.AudioTokens += src.OutputTokenDetails.AudioTokens
	dst.OutputTokenDetails.AudioTokensPresent =
		dst.OutputTokenDetails.AudioTokensPresent || src.OutputTokenDetails.AudioTokensPresent
	dst.OutputTokenDetails.ReasoningTokens += src.OutputTokenDetails.ReasoningTokens
	dst.OutputTokenDetails.ReasoningTokensPresent =
		dst.OutputTokenDetails.ReasoningTokensPresent || src.OutputTokenDetails.ReasoningTokensPresent
}

func OpenaiHandlerWithUsage(c *gin.Context, info *relaycommon.RelayInfo, resp *http.Response) (*shared.Usage, *shared.NookMuxError) {
	info.UsageSource = relayconstant.UsageSourceOpenAIChat
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
		if !usage.PromptTokensDetails.CachedTokensPresent &&
			usage.PromptTokensDetails.CachedTokens == 0 && usage.PromptCacheHitTokens != 0 {
			usage.PromptTokensDetails.CachedTokens = usage.PromptCacheHitTokens
		}
	case channelconstant.ChannelTypeZhipu_v4:
		// 智普的cached_tokens在标准位置: usage.prompt_tokens_details.cached_tokens
		if !usage.PromptTokensDetails.CachedTokensPresent && usage.PromptTokensDetails.CachedTokens == 0 {
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
		if !usage.PromptTokensDetails.CachedTokensPresent && usage.PromptTokensDetails.CachedTokens == 0 {
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
		if !usage.PromptTokensDetails.CachedTokensPresent && usage.PromptTokensDetails.CachedTokens == 0 {
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
