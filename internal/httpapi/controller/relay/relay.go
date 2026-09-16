package relaycontroller

import (
	"bytes"
	"errors"
	"fmt"
	"github.com/NookMux/NookMux/internal/common"
	"github.com/NookMux/NookMux/internal/config"
	"github.com/NookMux/NookMux/internal/config/operation"
	billing "github.com/NookMux/NookMux/internal/domain/billing"
	domainchannel "github.com/NookMux/NookMux/internal/domain/channel"
	sensitive "github.com/NookMux/NookMux/internal/domain/sensitive"
	"github.com/NookMux/NookMux/internal/domain/shared"
	"github.com/NookMux/NookMux/internal/httpapi"
	"github.com/NookMux/NookMux/internal/httpapi/middleware"
	"github.com/NookMux/NookMux/internal/i18n"
	"github.com/NookMux/NookMux/internal/infra/cache"
	"github.com/NookMux/NookMux/internal/infra/log"
	"github.com/NookMux/NookMux/internal/infra/runtime"
	tokenizer "github.com/NookMux/NookMux/internal/infra/tokenizer"
	"github.com/NookMux/NookMux/internal/relay"
	relaycommon "github.com/NookMux/NookMux/internal/relay/common"
	relayconstant "github.com/NookMux/NookMux/internal/relay/constant"
	"github.com/NookMux/NookMux/internal/relay/helper"
	"github.com/NookMux/NookMux/internal/store/channel"
	"github.com/NookMux/NookMux/internal/store/log"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
)

func relayHandler(c *gin.Context, info *relaycommon.RelayInfo) *shared.NookMuxError {
	var err *shared.NookMuxError
	switch info.RelayMode {
	case relayconstant.RelayModeImagesGenerations, relayconstant.RelayModeImagesEdits:
		err = relay.ImageHelper(c, info)
	case relayconstant.RelayModeAudioSpeech:
		fallthrough
	case relayconstant.RelayModeAudioTranslation:
		fallthrough
	case relayconstant.RelayModeAudioTranscription:
		err = relay.AudioHelper(c, info)
	case relayconstant.RelayModeRerank:
		err = relay.RerankHelper(c, info)
	case relayconstant.RelayModeEmbeddings:
		err = relay.EmbeddingHelper(c, info)
	case relayconstant.RelayModeChatCompletions, relayconstant.RelayModeResponses, relayconstant.RelayModeResponsesCompact:
		err = relay.OpenAIWireHelper(c, info)
	default:
		err = relay.TextHelper(c, info)
	}
	return err
}

func geminiRelayHandler(c *gin.Context, info *relaycommon.RelayInfo) *shared.NookMuxError {
	var err *shared.NookMuxError
	if strings.Contains(c.Request.URL.Path, "embed") {
		err = relay.GeminiEmbeddingHandler(c, info)
	} else {
		err = relay.GeminiHelper(c, info)
	}
	return err
}

func Relay(c *gin.Context, relayFormat relayconstant.RelayFormat) {

	requestId := c.GetString(common.RequestIdKey)
	//group := httpapi.GetContextKeyString(c, common.ContextKeyUsingGroup)
	//originalModel := httpapi.GetContextKeyString(c, common.ContextKeyOriginalModel)

	var (
		newAPIError *shared.NookMuxError
		ws          *websocket.Conn
	)

	if relayFormat == relayconstant.RelayFormatOpenAIRealtime {
		var err error
		ws, err = upgrader.Upgrade(c.Writer, c.Request, nil)
		if err != nil {
			helper.WssError(c, ws, shared.NewError(err, shared.ErrorCodeGetChannelFailed, shared.ErrOptionWithSkipRetry()).ToOpenAIError())
			return
		}
		defer ws.Close()
	}

	defer func() {
		if newAPIError != nil {
			log.LogError(c, fmt.Sprintf("relay error: %s", common.LocalLogPreview(newAPIError.Error())))
			newAPIError.SetExemptStrings(
				httpapi.GetContextKeyString(c, common.ContextKeyOriginalModel),
				httpapi.GetContextKeyString(c, common.ContextKeyUsingGroup),
			)
			newAPIError.SetMessage(common.MessageWithRequestId(newAPIError.Error(), requestId))
			switch relayFormat {
			case relayconstant.RelayFormatOpenAIRealtime:
				helper.WssError(c, ws, newAPIError.ToOpenAIError())
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
		}
	}()

	request, err := helper.GetAndValidateRequest(c, relayFormat)
	if err != nil {
		// Map "request body too large" to 413 so clients can handle it correctly
		if cache.IsRequestBodyTooLargeError(err) || errors.Is(err, cache.ErrRequestBodyTooLarge) {
			newAPIError = shared.NewErrorWithStatusCode(err, shared.ErrorCodeReadRequestBodyFailed, http.StatusRequestEntityTooLarge, shared.ErrOptionWithSkipRetry())
		} else {
			newAPIError = shared.NewError(err, shared.ErrorCodeInvalidRequest)
		}
		return
	}

	relayInfo, err := relaycommon.GenRelayInfo(c, relayFormat, request, ws)
	if err != nil {
		newAPIError = shared.NewError(err, shared.ErrorCodeGenRelayInfoFailed)
		return
	}

	needSensitiveCheck := config.ShouldCheckPromptSensitive()
	needCountToken := shared.CountToken
	// Avoid building huge CombineText (strings.Join) when token counting and sensitive check are both disabled.
	var meta *shared.TokenCountMeta
	if needSensitiveCheck || needCountToken {
		meta = request.GetTokenCountMeta()
	} else {
		meta = fastTokenCountMetaForPricing(request)
	}

	if needSensitiveCheck && meta != nil {
		contains, words := sensitive.CheckSensitiveText(meta.CombineText)
		if contains {
			log.LogWarn(c, fmt.Sprintf("user sensitive words detected: %s", strings.Join(words, ", ")))
			newAPIError = shared.NewError(err, shared.ErrorCodeSensitiveWordsDetected)
			return
		}
	}

	tokens, err := tokenizer.EstimateRequestToken(c, meta, relayInfo)
	if err != nil {
		newAPIError = shared.NewError(err, shared.ErrorCodeCountTokenFailed)
		return
	}

	relayInfo.SetEstimatePromptTokens(tokens)

	priceData, err := helper.ModelPriceHelper(c, relayInfo, tokens, meta)
	if err != nil {
		newAPIError = shared.NewError(err, shared.ErrorCodeModelPriceError)
		return
	}

	// httpapi.SetContextKey(c, common.ContextKeyTokenCountMeta, meta)

	if priceData.FreeModel {
		log.LogInfo(c, fmt.Sprintf("模型 %s 免费，跳过预扣费", relayInfo.OriginModelName))
	} else {
		newAPIError = billing.PreConsumeBilling(c, priceData.QuotaToPreConsume, relayInfo)
		if newAPIError != nil {
			return
		}
	}

	defer func() {
		// Only return quota if downstream failed and quota was actually pre-consumed
		if newAPIError != nil {
			newAPIError = billing.NormalizeViolationFeeError(newAPIError)
			if relayInfo.Billing != nil {
				relayInfo.Billing.Refund(c)
			}
			billing.ChargeViolationFeeIfNeeded(c, relayInfo, newAPIError)
		}
	}()

	retryParam := &domainchannel.RetryParam{
		Ctx:         c,
		TokenGroup:  relayInfo.TokenGroup,
		ModelName:   relayInfo.OriginModelName,
		Retry:       common.GetPointer(0),
		RelayFormat: relayFormat,
	}
	lastFailedChannelId := 0
	allowFailedChannelFallback := false

	// When the global automatic-retry toggle is off, only the first attempt runs.
	// The configured RetryTimes and status-code rules are preserved so toggling
	// retry back on later resumes the previous behavior without data loss.
	maxRetry := common.RetryTimes
	if !common.AutomaticRetryEnabled {
		maxRetry = 0
	}

	for ; retryParam.GetRetry() <= maxRetry; retryParam.IncreaseRetry() {
		// retry%2==1 means same-priority retry: exclude the previously failed channel
		// retry%2==1 表示同优先级重试：排除上次失败的渠道
		if retryParam.GetRetry()%2 == 1 {
			retryParam.ExcludeChannelId = lastFailedChannelId
			retryParam.AllowExcludedChannelFallback = allowFailedChannelFallback
		} else {
			retryParam.ExcludeChannelId = 0
			retryParam.AllowExcludedChannelFallback = false
		}

		channel, channelErr := getChannel(c, relayInfo, retryParam)
		if channelErr != nil {
			log.LogError(c, channelErr.Error())
			newAPIError = channelErr
			break
		}

		addUsedChannel(c, channel.Id)
		lastFailedChannelId = channel.Id
		requestBody, bodyErr := httpapi.GetRequestBody(c)
		if bodyErr != nil {
			// Ensure consistent 413 for oversized bodies even when error occurs later (e.g., retry path)
			if cache.IsRequestBodyTooLargeError(bodyErr) || errors.Is(bodyErr, cache.ErrRequestBodyTooLarge) {
				newAPIError = shared.NewErrorWithStatusCode(bodyErr, shared.ErrorCodeReadRequestBodyFailed, http.StatusRequestEntityTooLarge, shared.ErrOptionWithSkipRetry())
			} else {
				newAPIError = shared.NewErrorWithStatusCode(bodyErr, shared.ErrorCodeReadRequestBodyFailed, http.StatusBadRequest, shared.ErrOptionWithSkipRetry())
			}
			break
		}
		c.Request.Body = io.NopCloser(bytes.NewBuffer(requestBody))

		attemptStartTime := time.Now()
		httpapi.SetContextKey(c, common.ContextKeyRequestStartTime, attemptStartTime)
		relayInfo.ResetForAttempt(attemptStartTime)

		switch relayFormat {
		case relayconstant.RelayFormatOpenAIRealtime:
			newAPIError = relay.WssHelper(c, relayInfo)
		case relayconstant.RelayFormatClaude:
			newAPIError = relay.ClaudeHelper(c, relayInfo)
		case relayconstant.RelayFormatGemini:
			newAPIError = geminiRelayHandler(c, relayInfo)
		default:
			newAPIError = relayHandler(c, relayInfo)
		}

		if newAPIError == nil {
			return
		}

		newAPIError = billing.NormalizeViolationFeeError(newAPIError)

		channelWillBeDisabled := ProcessChannelError(c, *domainchannel.NewChannelError(channel.Id, channel.Type, channel.Name, channel.ChannelInfo.IsMultiKey, httpapi.GetContextKeyString(c, common.ContextKeyChannelKey), channel.GetAutoBan()), newAPIError)
		allowFailedChannelFallback = !channelWillBeDisabled

		if !shouldRetry(c, newAPIError, maxRetry-retryParam.GetRetry()) {
			break
		}
	}

	useChannel := c.GetStringSlice("use_channel")
	if len(useChannel) > 1 {
		retryLogStr := fmt.Sprintf("重试：%s", strings.Trim(strings.Join(strings.Fields(fmt.Sprint(useChannel)), "->"), "[]"))
		log.LogInfo(c, retryLogStr)
	}
}

var upgrader = websocket.Upgrader{
	Subprotocols: []string{"realtime"}, // WS 握手支持的协议，如果有使用 Sec-WebSocket-Protocol，则必须在此声明对应的 Protocol TODO add other protocol
	CheckOrigin: func(r *http.Request) bool {
		return true // 允许跨域
	},
}

func addUsedChannel(c *gin.Context, channelId int) {
	useChannel := c.GetStringSlice("use_channel")
	useChannel = append(useChannel, fmt.Sprintf("%d", channelId))
	c.Set("use_channel", useChannel)
}

func fastTokenCountMetaForPricing(request shared.Request) *shared.TokenCountMeta {
	if request == nil {
		return &shared.TokenCountMeta{}
	}
	meta := &shared.TokenCountMeta{
		TokenType: shared.TokenTypeTokenizer,
	}
	switch r := request.(type) {
	case *shared.GeneralOpenAIRequest:
		if r.MaxCompletionTokens > r.MaxTokens {
			meta.MaxTokens = int(r.MaxCompletionTokens)
		} else {
			meta.MaxTokens = int(r.MaxTokens)
		}
	case *shared.OpenAIResponsesRequest:
		meta.MaxTokens = int(r.MaxOutputTokens)
	case *shared.ClaudeRequest:
		meta.MaxTokens = int(r.MaxTokens)
	case *shared.ImageRequest:
		// Pricing for image requests depends on ImagePriceRatio; safe to compute even when CountToken is disabled.
		return r.GetTokenCountMeta()
	default:
		// Best-effort: leave CombineText empty to avoid large allocations.
	}
	return meta
}

func getChannel(c *gin.Context, info *relaycommon.RelayInfo, retryParam *domainchannel.RetryParam) (*channelstore.Channel, *shared.NookMuxError) {
	if info.ChannelMeta == nil {
		autoBan := c.GetBool("auto_ban")
		autoBanInt := 1
		if !autoBan {
			autoBanInt = 0
		}
		return &channelstore.Channel{
			Id:      c.GetInt("channel_id"),
			Type:    c.GetInt("channel_type"),
			Name:    c.GetString("channel_name"),
			AutoBan: &autoBanInt,
		}, nil
	}
	channel, selectGroup, err := domainchannel.CacheGetRandomSatisfiedChannel(retryParam)

	info.PriceData.GroupRatioInfo = helper.HandleGroupRatio(c, info)

	if err != nil {
		return nil, shared.NewError(fmt.Errorf("%s", i18n.T(c, i18n.MsgRelayRetryGetChannelFailed, map[string]any{
			"Group": selectGroup,
			"Model": info.OriginModelName,
			"Error": err.Error(),
		})), shared.ErrorCodeGetChannelFailed, shared.ErrOptionWithSkipRetry())
	}
	if channel == nil {
		return nil, shared.NewError(fmt.Errorf("%s", i18n.T(c, i18n.MsgRelayRetryChannelNotFound, map[string]any{
			"Group": selectGroup,
			"Model": info.OriginModelName,
		})), shared.ErrorCodeGetChannelFailed, shared.ErrOptionWithSkipRetry())
	}

	newAPIError := middleware.SetupContextForSelectedChannel(c, channel, info.OriginModelName)
	if newAPIError != nil {
		return nil, newAPIError
	}
	return channel, nil
}

func shouldRetry(c *gin.Context, openaiErr *shared.NookMuxError, retryTimes int) bool {
	if openaiErr == nil {
		return false
	}
	if domainchannel.ShouldSkipRetryAfterChannelAffinityFailure(c) {
		return false
	}
	if shared.IsChannelError(openaiErr) {
		return true
	}
	if shared.IsSkipRetryError(openaiErr) {
		return false
	}
	if retryTimes <= 0 {
		return false
	}
	if _, ok := c.Get("specific_channel_id"); ok {
		return false
	}
	if shouldRetryByHTTPStatusCode(openaiErr.OriginalStatusCode) {
		return true
	}
	if shouldRetryByHTTPStatusCode(openaiErr.StatusCode) {
		return true
	}

	return shouldRetryByNumericErrorCode(openaiErr)
}

func shouldRetryByHTTPStatusCode(statusCode int) bool {
	if statusCode == 0 {
		return false
	}
	if statusCode >= 200 && statusCode < 300 {
		return false
	}
	if statusCode < 100 || statusCode > 9999 {
		return true
	}
	return operation.ShouldRetryByStatusCode(statusCode)
}

func shouldRetryByNumericErrorCode(openaiErr *shared.NookMuxError) bool {
	if openaiErr == nil {
		return false
	}
	codeStr := strings.TrimSpace(string(openaiErr.GetErrorCode()))
	if codeStr == "" {
		return false
	}
	code, err := strconv.Atoi(codeStr)
	if err != nil {
		return false
	}
	return operation.ShouldRetryByStatusCode(code)
}

func ProcessChannelError(c *gin.Context, channelError domainchannel.ChannelError, err *shared.NookMuxError) bool {
	log.LogError(c, fmt.Sprintf("channel error (channel #%d, status code: %d): %s", channelError.ChannelId, err.StatusCode, common.LocalLogPreview(err.Error())))
	// 不要使用context获取渠道信息，异步处理时可能会出现渠道信息不一致的情况
	// do not use context to get channel info, there may be inconsistent channel info when processing asynchronously
	channelWillBeDisabled := domainchannel.ShouldDisableChannel(channelError.ChannelType, err) && channelError.AutoBan
	if channelWillBeDisabled {
		runtime.RelayGo(func() {
			domainchannel.DisableChannel(channelError, err.ErrorWithStatusCode())
		})
	}

	if shared.ErrorLogEnabled && shared.IsRecordErrorLog(err) {
		// 保存错误日志到mysql中
		userId := c.GetInt("id")
		tokenName := c.GetString("token_name")
		modelName := c.GetString("original_model")
		tokenId := c.GetInt("token_id")
		userGroup := c.GetString("group")
		channelId := c.GetInt("channel_id")
		other := make(map[string]interface{})
		if c.Request != nil && c.Request.URL != nil {
			other["request_path"] = c.Request.URL.Path
		}
		other["error_type"] = err.GetErrorType()
		other["error_code"] = err.GetErrorCode()
		other["status_code"] = err.StatusCode
		other["channel_id"] = channelId
		other["channel_name"] = c.GetString("channel_name")
		other["channel_type"] = c.GetInt("channel_type")
		adminInfo := make(map[string]interface{})
		adminInfo["use_channel"] = c.GetStringSlice("use_channel")
		isMultiKey := httpapi.GetContextKeyBool(c, common.ContextKeyChannelIsMultiKey)
		if isMultiKey {
			adminInfo["is_multi_key"] = true
			adminInfo["multi_key_index"] = httpapi.GetContextKeyInt(c, common.ContextKeyChannelMultiKeyIndex)
		}
		domainchannel.AppendChannelAffinityAdminInfo(c, adminInfo)
		other["admin_info"] = adminInfo
		startTime := httpapi.GetContextKeyTime(c, common.ContextKeyRequestStartTime)
		if startTime.IsZero() {
			startTime = time.Now()
		}
		useTimeMs := int(time.Since(startTime).Milliseconds())
		err.SetExemptStrings(modelName, userGroup)
		logstore.RecordErrorLog(c, userId, channelId, modelName, tokenName, err.MaskSensitiveErrorWithStatusCode(), tokenId, useTimeMs, false, userGroup, other)
	}

	return channelWillBeDisabled
}

func RelayNotImplemented(c *gin.Context) {
	err := shared.OpenAIError{
		Message: "API not implemented",
		Type:    "new_api_error",
		Param:   "",
		Code:    "api_not_implemented",
	}
	c.JSON(http.StatusNotImplemented, gin.H{
		"error": err,
	})
}

func RelayNotFound(c *gin.Context) {
	err := shared.OpenAIError{
		Message: fmt.Sprintf("Invalid URL (%s %s)", c.Request.Method, c.Request.URL.Path),
		Type:    "invalid_request_error",
		Param:   "",
		Code:    "",
	}
	c.JSON(http.StatusNotFound, gin.H{
		"error": err,
	})
}
