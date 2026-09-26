package channelcontroller

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/NookMux/NookMux/internal/common"
	"github.com/NookMux/NookMux/internal/config/operation"
	"github.com/NookMux/NookMux/internal/config/ratio"
	billing "github.com/NookMux/NookMux/internal/domain/billing"
	domainchannel "github.com/NookMux/NookMux/internal/domain/channel"
	channelconstant "github.com/NookMux/NookMux/internal/domain/channel/constant"
	"github.com/NookMux/NookMux/internal/domain/shared"
	"github.com/NookMux/NookMux/internal/httpapi"
	relaycontroller "github.com/NookMux/NookMux/internal/httpapi/controller/relay"
	"github.com/NookMux/NookMux/internal/httpapi/middleware"
	"github.com/NookMux/NookMux/internal/i18n"
	domainnotify "github.com/NookMux/NookMux/internal/infra/notify"
	"github.com/NookMux/NookMux/internal/infra/runtime"
	"github.com/NookMux/NookMux/internal/relay"
	relaycommon "github.com/NookMux/NookMux/internal/relay/common"
	relayconstant "github.com/NookMux/NookMux/internal/relay/constant"
	"github.com/NookMux/NookMux/internal/relay/helper"
	"github.com/NookMux/NookMux/internal/store/channel"
	"github.com/NookMux/NookMux/internal/store/db"
	"github.com/NookMux/NookMux/internal/store/log"
	"github.com/NookMux/NookMux/internal/store/user"
	"github.com/NookMux/NookMux/pkg/jsonx"
	"github.com/gin-gonic/gin"
	"github.com/samber/lo"
	"github.com/tidwall/gjson"
	"io"
	"math"
	"math/rand"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode"
)

const (
	channelTestToolName         = "report_result"
	channelTestToolNotSupported = i18n.MsgChannelTestToolNotSupported
)

type testResult struct {
	context     *gin.Context
	localErr    error
	newAPIError *shared.NookMuxError
}

type channelTestPrompt struct {
	// prompt is the user-facing instruction sent to the model.
	prompt string
	// expectedAnswer is the integer result of the generated arithmetic problem.
	// Zero-value is valid only when isTool is true (tool tests do not use arithmetic).
	expectedAnswer int
	// isTool indicates the request should force a deterministic tool call.
	isTool bool
	// requiresTextAnswer indicates the response body should be validated as arithmetic text.
	requiresTextAnswer bool
}

func normalizeChannelTestEndpoint(channel *channelstore.Channel, modelName, endpointType string) string {
	normalized := strings.TrimSpace(endpointType)
	if normalized != "" {
		return normalized
	}
	if strings.HasSuffix(modelName, ratio.CompactModelSuffix) {
		return string(channelconstant.EndpointTypeOpenAIResponseCompact)
	}
	return normalized
}

func resolveChannelTestUserID(c *gin.Context) (int, error) {
	if c != nil {
		if userID := c.GetInt("id"); userID > 0 {
			return userID, nil
		}
	}

	var rootUser userstore.User
	if err := dbstore.DB.Select("id").Where("role = ?", common.RoleRootUser).First(&rootUser).Error; err != nil {
		return 0, fmt.Errorf("failed to resolve channel test user: %w", err)
	}
	if rootUser.Id == 0 {
		return 0, errors.New("failed to resolve channel test user")
	}
	return rootUser.Id, nil
}

func testChannel(channel *channelstore.Channel, testUserID int, testModel string, endpointType string, isStream bool, isTool bool) testResult {
	tik := time.Now()
	var unsupportedTestChannelTypes = []int{
		channelconstant.ChannelTypeMidjourney,
		channelconstant.ChannelTypeMidjourneyPlus,
		channelconstant.ChannelTypeSunoAPI,
	}
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	httpapi.SetContextKey(c, common.ContextKeyRequestStartTime, tik)
	c.Request = &http.Request{Header: make(http.Header)}
	if lo.Contains(unsupportedTestChannelTypes, channel.Type) {
		channelTypeName := channelconstant.GetChannelTypeName(channel.Type)
		return testResult{
			context:  c,
			localErr: fmt.Errorf("%s", i18n.T(c, i18n.MsgChannelTestNotSupported, map[string]any{"Type": channelTypeName})),
		}
	}

	testModel = strings.TrimSpace(testModel)
	if testModel == "" {
		if channel.TestModel != nil && *channel.TestModel != "" {
			testModel = strings.TrimSpace(*channel.TestModel)
		} else {
			models := channel.GetModels()
			if len(models) > 0 {
				testModel = strings.TrimSpace(models[0])
			}
			if testModel == "" {
				testModel = "gpt-4o-mini"
			}
		}
	}

	endpointType = normalizeChannelTestEndpoint(channel, testModel, endpointType)

	requestPath := "/v1/chat/completions"

	// 如果指定了端点类型，使用指定的端点类型
	if endpointType != "" {
		if endpointInfo, ok := common.GetDefaultEndpointInfo(channelconstant.EndpointType(endpointType)); ok {
			requestPath = endpointInfo.Path
		}
	} else {
		// 如果没有指定端点类型，使用原有的自动检测逻辑

		if isChannelTestRerankModel(testModel) {
			requestPath = "/v1/rerank"
		}

		// 先判断是否为 Embedding 模型
		if isChannelTestEmbeddingModel(testModel) {
			requestPath = "/v1/embeddings" // 修改请求路径
		}

		// responses-only models
		if strings.Contains(strings.ToLower(testModel), "codex") {
			requestPath = "/v1/responses"
		}

		// responses compaction models (must use /v1/responses/compact)
		if strings.HasSuffix(testModel, ratio.CompactModelSuffix) {
			requestPath = "/v1/responses/compact"
		}
	}
	if strings.HasPrefix(requestPath, "/v1/responses/compact") {
		testModel = ratio.WithCompactModelSuffix(testModel)
	}

	c.Request = &http.Request{
		Method: "POST",
		URL:    &url.URL{Path: requestPath}, // 使用动态路径
		Body:   nil,
		Header: make(http.Header),
	}

	cache, err := userstore.GetUserCache(testUserID)
	if err != nil {
		return testResult{
			localErr:    err,
			newAPIError: nil,
		}
	}
	cache.WriteContext(c)
	c.Set("id", testUserID)

	//c.Request.Header.Set("Authorization", "Bearer "+channel.Key)
	c.Request.Header.Set("Content-Type", "application/json")
	c.Set("channel", channel.Type)
	c.Set("base_url", channel.GetBaseURL())
	group, _ := userstore.GetUserGroup(testUserID, false)
	c.Set("group", group)

	newAPIError := middleware.SetupContextForSelectedChannel(c, channel, testModel)
	if newAPIError != nil {
		return testResult{
			context:     c,
			localErr:    newAPIError,
			newAPIError: newAPIError,
		}
	}

	// Determine relay format based on endpoint type or request path
	var relayFormat relayconstant.RelayFormat
	if endpointType != "" {
		// 根据指定的端点类型设置 relayFormat
		switch channelconstant.EndpointType(endpointType) {
		case channelconstant.EndpointTypeOpenAI:
			relayFormat = relayconstant.RelayFormatOpenAI
		case channelconstant.EndpointTypeOpenAIResponse:
			relayFormat = relayconstant.RelayFormatOpenAIResponses
		case channelconstant.EndpointTypeOpenAIResponseCompact:
			relayFormat = relayconstant.RelayFormatOpenAIResponsesCompaction
		case channelconstant.EndpointTypeAnthropic:
			relayFormat = relayconstant.RelayFormatClaude
		case channelconstant.EndpointTypeGemini:
			relayFormat = relayconstant.RelayFormatGemini
		case channelconstant.EndpointTypeJinaRerank:
			relayFormat = relayconstant.RelayFormatRerank
		case channelconstant.EndpointTypeImageGeneration:
			relayFormat = relayconstant.RelayFormatOpenAIImage
		case channelconstant.EndpointTypeEmbeddings:
			relayFormat = relayconstant.RelayFormatEmbedding
		default:
			relayFormat = relayconstant.RelayFormatOpenAI
		}
	} else {
		// 根据请求路径自动检测
		relayFormat = relayconstant.RelayFormatOpenAI
		if c.Request.URL.Path == "/v1/embeddings" {
			relayFormat = relayconstant.RelayFormatEmbedding
		}
		if c.Request.URL.Path == "/v1/images/generations" {
			relayFormat = relayconstant.RelayFormatOpenAIImage
		}
		if c.Request.URL.Path == "/v1/messages" {
			relayFormat = relayconstant.RelayFormatClaude
		}
		if strings.Contains(c.Request.URL.Path, "/v1beta/models") {
			relayFormat = relayconstant.RelayFormatGemini
		}
		if c.Request.URL.Path == "/v1/rerank" || c.Request.URL.Path == "/rerank" {
			relayFormat = relayconstant.RelayFormatRerank
		}
		if c.Request.URL.Path == "/v1/responses" {
			relayFormat = relayconstant.RelayFormatOpenAIResponses
		}
		if strings.HasPrefix(c.Request.URL.Path, "/v1/responses/compact") {
			relayFormat = relayconstant.RelayFormatOpenAIResponsesCompaction
		}
	}

	testPrompt := buildChannelTestPrompt(endpointType, testModel, isTool)
	if isTool && !supportsChannelTestToolForChannel(channel, endpointType, testModel) {
		return testResult{
			context:     c,
			localErr:    errors.New(i18n.T(c, channelTestToolNotSupported)),
			newAPIError: shared.NewError(errors.New(i18n.T(c, channelTestToolNotSupported)), shared.ErrorCodeChannelTestToolUnsupported),
		}
	}
	request := buildTestRequest(testModel, endpointType, channel, isStream, testPrompt)

	info, err := relaycommon.GenRelayInfo(c, relayFormat, request, nil)

	if err != nil {
		return testResult{
			context:     c,
			localErr:    err,
			newAPIError: shared.NewError(err, shared.ErrorCodeGenRelayInfoFailed),
		}
	}

	info.IsChannelTest = true
	info.InitChannelMeta(c)

	err = helper.ModelMappedHelper(c, info, request)
	if err != nil {
		return testResult{
			context:     c,
			localErr:    err,
			newAPIError: shared.NewError(err, shared.ErrorCodeChannelModelMappedError),
		}
	}

	testModel = info.UpstreamModelName
	// 更新请求中的模型名称
	request.SetModelName(testModel)

	// 模型映射后的上游模型可能变成 embedding/rerank/compact/nova 等不支持工具的型号,
	// 这里再校验一次,避免把不兼容的工具测试请求发到上游触发"输入不合规"类错误。
	if isTool && !supportsChannelTestToolForChannel(channel, endpointType, testModel) {
		return testResult{
			context:     c,
			localErr:    errors.New(i18n.T(c, channelTestToolNotSupported)),
			newAPIError: shared.NewError(errors.New(i18n.T(c, channelTestToolNotSupported)), shared.ErrorCodeChannelTestToolUnsupported),
		}
	}

	apiType, _ := common.ChannelType2APIType(channel.Type)
	if info.RelayMode == relayconstant.RelayModeResponsesCompact &&
		apiType != channelconstant.APITypeOpenAI {
		return testResult{
			context:     c,
			localErr:    fmt.Errorf("%s", i18n.T(c, i18n.MsgChannelResponsesCompactionOnlyOpenAI, map[string]any{"Type": apiType})),
			newAPIError: shared.NewError(fmt.Errorf("%s", i18n.T(c, i18n.MsgChannelInvalidApiType, map[string]any{"Type": apiType})), shared.ErrorCodeInvalidApiType),
		}
	}
	adaptor := relay.GetAdaptor(apiType)
	if adaptor == nil {
		return testResult{
			context:     c,
			localErr:    fmt.Errorf("%s", i18n.T(c, i18n.MsgChannelInvalidApiType, map[string]any{"Type": apiType})),
			newAPIError: shared.NewError(fmt.Errorf("%s", i18n.T(c, i18n.MsgChannelInvalidApiType, map[string]any{"Type": apiType})), shared.ErrorCodeInvalidApiType),
		}
	}

	//// 创建一个用于日志的 info 副本，移除 ApiKey
	//logInfo := info
	//logInfo.ApiKey = ""
	common.SysLog(fmt.Sprintf("testing channel %d with model %s , info %+v ", channel.Id, testModel, info.ToString()))

	priceData, err := helper.ModelPriceHelper(c, info, 0, request.GetTokenCountMeta())
	if err != nil {
		return testResult{
			context:     c,
			localErr:    err,
			newAPIError: shared.NewError(err, shared.ErrorCodeModelPriceError),
		}
	}

	adaptor.Init(info)

	var convertedRequest any
	// 根据 RelayMode 选择正确的转换函数
	switch info.RelayMode {
	case relayconstant.RelayModeEmbeddings:
		// Embedding 请求 - request 已经是正确的类型
		if embeddingReq, ok := request.(*shared.EmbeddingRequest); ok {
			convertedRequest, err = adaptor.ConvertEmbeddingRequest(c, info, *embeddingReq)
		} else {
			return testResult{
				context:     c,
				localErr:    errors.New(i18n.T(c, i18n.MsgChannelInvalidEmbeddingRequest)),
				newAPIError: shared.NewError(errors.New(i18n.T(c, i18n.MsgChannelInvalidEmbeddingRequest)), shared.ErrorCodeConvertRequestFailed),
			}
		}
	case relayconstant.RelayModeImagesGenerations:
		// 图像生成请求 - request 已经是正确的类型
		if imageReq, ok := request.(*shared.ImageRequest); ok {
			convertedRequest, err = adaptor.ConvertImageRequest(c, info, *imageReq)
		} else {
			return testResult{
				context:     c,
				localErr:    errors.New(i18n.T(c, i18n.MsgChannelInvalidImageRequest)),
				newAPIError: shared.NewError(errors.New(i18n.T(c, i18n.MsgChannelInvalidImageRequest)), shared.ErrorCodeConvertRequestFailed),
			}
		}
	case relayconstant.RelayModeRerank:
		// Rerank 请求 - request 已经是正确的类型
		if rerankReq, ok := request.(*shared.RerankRequest); ok {
			convertedRequest, err = adaptor.ConvertRerankRequest(c, info.RelayMode, *rerankReq)
		} else {
			return testResult{
				context:     c,
				localErr:    errors.New(i18n.T(c, i18n.MsgChannelInvalidRerankRequest)),
				newAPIError: shared.NewError(errors.New(i18n.T(c, i18n.MsgChannelInvalidRerankRequest)), shared.ErrorCodeConvertRequestFailed),
			}
		}
	case relayconstant.RelayModeResponses:
		// Response 请求 - request 已经是正确的类型
		if responseReq, ok := request.(*shared.OpenAIResponsesRequest); ok {
			convertedRequest, err = adaptor.ConvertOpenAIResponsesRequest(c, info, *responseReq)
		} else {
			return testResult{
				context:     c,
				localErr:    errors.New(i18n.T(c, i18n.MsgChannelInvalidResponseRequest)),
				newAPIError: shared.NewError(errors.New(i18n.T(c, i18n.MsgChannelInvalidResponseRequest)), shared.ErrorCodeConvertRequestFailed),
			}
		}
	case relayconstant.RelayModeResponsesCompact:
		// Response compaction request - convert to OpenAIResponsesRequest before adapting
		switch req := request.(type) {
		case *shared.OpenAIResponsesCompactionRequest:
			convertedRequest, err = adaptor.ConvertOpenAIResponsesRequest(c, info, shared.OpenAIResponsesRequest{
				Model:              req.Model,
				Input:              req.Input,
				Instructions:       req.Instructions,
				PreviousResponseID: req.PreviousResponseID,
			})
		case *shared.OpenAIResponsesRequest:
			convertedRequest, err = adaptor.ConvertOpenAIResponsesRequest(c, info, *req)
		default:
			return testResult{
				context:     c,
				localErr:    errors.New(i18n.T(c, i18n.MsgChannelInvalidResponseCompactionRequest)),
				newAPIError: shared.NewError(errors.New(i18n.T(c, i18n.MsgChannelInvalidResponseCompactionRequest)), shared.ErrorCodeConvertRequestFailed),
			}
		}
	default:
		// Chat/Completion 等其他请求类型
		if generalReq, ok := request.(*shared.GeneralOpenAIRequest); ok {
			convertedRequest, err = adaptor.ConvertOpenAIRequest(c, info, generalReq)
		} else {
			return testResult{
				context:     c,
				localErr:    errors.New(i18n.T(c, i18n.MsgChannelInvalidGeneralRequest)),
				newAPIError: shared.NewError(errors.New(i18n.T(c, i18n.MsgChannelInvalidGeneralRequest)), shared.ErrorCodeConvertRequestFailed),
			}
		}
	}

	if err != nil {
		return testResult{
			context:     c,
			localErr:    err,
			newAPIError: shared.NewError(err, shared.ErrorCodeConvertRequestFailed),
		}
	}
	jsonData, err := jsonx.Marshal(convertedRequest)
	if err != nil {
		return testResult{
			context:     c,
			localErr:    err,
			newAPIError: shared.NewError(err, shared.ErrorCodeJsonMarshalFailed),
		}
	}

	//jsonData, err = relaycommon.RemoveDisabledFields(jsonData, info.ChannelOtherSettings)
	//if err != nil {
	//	return testResult{
	//		context:     c,
	//		localErr:    err,
	//		newAPIError: shared.NewError(err, shared.ErrorCodeConvertRequestFailed),
	//	}
	//}

	if len(info.ParamOverride) > 0 {
		jsonData, err = relaycommon.ApplyParamOverride(jsonData, info.ParamOverride, relaycommon.BuildParamOverrideContext(info))
		if err != nil {
			return testResult{
				context:     c,
				localErr:    err,
				newAPIError: shared.NewError(err, shared.ErrorCodeChannelParamOverrideInvalid),
			}
		}
	}

	requestBody := bytes.NewBuffer(jsonData)
	c.Request.Body = io.NopCloser(bytes.NewBuffer(jsonData))
	resp, err := adaptor.DoRequest(c, info, requestBody)
	if err != nil {
		return testResult{
			context:     c,
			localErr:    err,
			newAPIError: shared.NewOpenAIError(err, shared.ErrorCodeDoRequestFailed, http.StatusInternalServerError),
		}
	}
	var httpResp *http.Response
	if resp != nil {
		httpResp = resp.(*http.Response)
		if httpResp.StatusCode != http.StatusOK {
			err := helper.RelayErrorHandler(c.Request.Context(), httpResp, true)
			common.SysError(fmt.Sprintf(
				"channel test bad response: channel_id=%d name=%s type=%d model=%s endpoint_type=%s status=%d err=%v",
				channel.Id,
				channel.Name,
				channel.Type,
				testModel,
				endpointType,
				httpResp.StatusCode,
				err,
			))
			return testResult{
				context:     c,
				localErr:    err,
				newAPIError: shared.NewOpenAIError(err, shared.ErrorCodeBadResponse, http.StatusInternalServerError),
			}
		}
	}
	usageA, respErr := adaptor.DoResponse(c, httpResp, info)
	if respErr != nil {
		return testResult{
			context:     c,
			localErr:    respErr,
			newAPIError: respErr,
		}
	}
	usage, usageErr := coerceTestUsage(usageA, isStream, info.GetEstimatePromptTokens())
	if usageErr != nil {
		return testResult{
			context:     c,
			localErr:    usageErr,
			newAPIError: shared.NewOpenAIError(usageErr, shared.ErrorCodeBadResponseBody, http.StatusInternalServerError),
		}
	}
	result := w.Result()
	respBody, err := readTestResponseBody(result.Body, isStream)
	if err != nil {
		return testResult{
			context:     c,
			localErr:    err,
			newAPIError: shared.NewOpenAIError(err, shared.ErrorCodeReadResponseBodyFailed, http.StatusInternalServerError),
		}
	}
	if bodyErr := detectErrorFromTestResponseBody(respBody); bodyErr != nil {
		return testResult{
			context:     c,
			localErr:    bodyErr,
			newAPIError: shared.NewOpenAIError(bodyErr, shared.ErrorCodeBadResponseBody, http.StatusInternalServerError),
		}
	}
	if validateErr := validateChannelTestResponse(c, respBody, testPrompt); validateErr != nil {
		return testResult{
			context:     c,
			localErr:    validateErr,
			newAPIError: shared.NewError(validateErr, classifyChannelTestValidationError(validateErr, testPrompt)),
		}
	}
	info.SetEstimatePromptTokens(usage.PromptTokens)

	quota := 0
	if !priceData.UsePrice {
		quota = usage.PromptTokens + int(math.Round(float64(usage.CompletionTokens)*priceData.CompletionRatio))
		quota = int(math.Round(float64(quota) * priceData.ModelRatio))
		if priceData.ModelRatio != 0 && quota <= 0 {
			quota = 1
		}
	} else {
		quota = int(priceData.ModelPrice * common.QuotaPerUnit)
	}
	tok := time.Now()
	milliseconds := tok.Sub(tik).Milliseconds()
	originalGroupRatio := priceData.GroupRatioInfo.GroupRatio
	if priceData.GroupRatioInfo.DynamicRatio > 0 {
		originalGroupRatio = priceData.GroupRatioInfo.GroupRatio / priceData.GroupRatioInfo.DynamicRatio
	}
	other := billing.GenerateTextOtherInfo(c, info, priceData.ModelRatio, originalGroupRatio, priceData.CompletionRatio,
		usage.PromptTokensDetails.CachedTokens, priceData.CacheRatio, priceData.ModelPrice, priceData.GroupRatioInfo.GroupSpecialRatio, priceData.GroupRatioInfo.DynamicRatio)
	// 复用正式消费日志的流式指标，保证模型测试日志与线上展示一致。
	billing.AppendStreamMetrics(other, info, milliseconds, usage.CompletionTokens)
	logstore.RecordConsumeLog(c, testUserID, logstore.RecordConsumeLogParams{
		ChannelId:        channel.Id,
		PromptTokens:     usage.PromptTokens,
		CompletionTokens: usage.CompletionTokens,
		ModelName:        info.OriginModelName,
		TokenName:        "模型测试",
		Quota:            quota,
		Content:          "模型测试",
		UseTimeMs:        int(milliseconds),
		IsStream:         info.IsStream,
		Group:            info.UsingGroup,
		Other:            other,
	})
	common.SysLog(fmt.Sprintf("testing channel #%d, response: \n%s", channel.Id, string(respBody)))
	return testResult{
		context:     c,
		localErr:    nil,
		newAPIError: nil,
	}
}

func coerceTestUsage(usageAny any, isStream bool, estimatePromptTokens int) (*shared.Usage, error) {
	switch u := usageAny.(type) {
	case *shared.Usage:
		return u, nil
	case shared.Usage:
		return &u, nil
	case nil:
		if !isStream {
			return nil, errors.New("usage is nil")
		}
		usage := &shared.Usage{
			PromptTokens: estimatePromptTokens,
		}
		usage.TotalTokens = usage.PromptTokens
		return usage, nil
	default:
		if !isStream {
			return nil, fmt.Errorf("invalid usage type: %T", usageAny)
		}
		usage := &shared.Usage{
			PromptTokens: estimatePromptTokens,
		}
		usage.TotalTokens = usage.PromptTokens
		return usage, nil
	}
}

func readTestResponseBody(body io.ReadCloser, isStream bool) ([]byte, error) {
	defer func() { _ = body.Close() }()
	const maxStreamLogBytes = 8 << 10
	if isStream {
		return io.ReadAll(io.LimitReader(body, maxStreamLogBytes))
	}
	return io.ReadAll(body)
}

func detectErrorFromTestResponseBody(respBody []byte) error {
	b := bytes.TrimSpace(respBody)
	if len(b) == 0 {
		return nil
	}
	if message := detectErrorMessageFromJSONBytes(b); message != "" {
		return fmt.Errorf("upstream error: %s", message)
	}

	for _, line := range bytes.Split(b, []byte{'\n'}) {
		line = bytes.TrimSpace(line)
		if len(line) == 0 {
			continue
		}
		if !bytes.HasPrefix(line, []byte("data:")) {
			continue
		}
		payload := bytes.TrimSpace(bytes.TrimPrefix(line, []byte("data:")))
		if len(payload) == 0 || bytes.Equal(payload, []byte("[DONE]")) {
			continue
		}
		if message := detectErrorMessageFromJSONBytes(payload); message != "" {
			return fmt.Errorf("upstream error: %s", message)
		}
	}

	return nil
}

func detectErrorMessageFromJSONBytes(jsonBytes []byte) string {
	if len(jsonBytes) == 0 {
		return ""
	}
	if jsonBytes[0] != '{' && jsonBytes[0] != '[' {
		return ""
	}
	errVal := gjson.GetBytes(jsonBytes, "error")
	if !errVal.Exists() || errVal.Type == gjson.Null {
		return ""
	}

	message := gjson.GetBytes(jsonBytes, "error.message").String()
	if message == "" {
		message = gjson.GetBytes(jsonBytes, "error.error.message").String()
	}
	if message == "" && errVal.Type == gjson.String {
		message = errVal.String()
	}
	if message == "" {
		message = errVal.Raw
	}
	message = strings.TrimSpace(message)
	if message == "" {
		return "upstream returned error payload"
	}
	return message
}

func buildTestRequest(model string, endpointType string, channel *channelstore.Channel, isStream bool, testPrompt channelTestPrompt) shared.Request {
	userPrompt := testPrompt.prompt
	if strings.TrimSpace(userPrompt) == "" {
		userPrompt = "hi"
	}
	responsesInput, _ := jsonx.Marshal([]map[string]any{
		{
			"role":    "user",
			"content": userPrompt,
		},
	})
	testResponsesInput := json.RawMessage(responsesInput)

	// 根据端点类型构建不同的测试请求
	if endpointType != "" {
		switch channelconstant.EndpointType(endpointType) {
		case channelconstant.EndpointTypeEmbeddings:
			return &shared.EmbeddingRequest{
				Model: model,
				Input: []any{"hello world"},
			}
		case channelconstant.EndpointTypeImageGeneration:
			return &shared.ImageRequest{
				Model:  model,
				Prompt: "a cute cat",
				N:      1,
				Size:   "1024x1024",
			}
		case channelconstant.EndpointTypeJinaRerank:
			return &shared.RerankRequest{
				Model:     model,
				Query:     "What is Deep Learning?",
				Documents: []any{"Deep Learning is a subset of machine learning.", "Machine learning is a field of artificial intelligence."},
				TopN:      2,
			}
		case channelconstant.EndpointTypeOpenAIResponse:
			req := &shared.OpenAIResponsesRequest{
				Model:  model,
				Input:  testResponsesInput,
				Stream: isStream,
			}
			applyChannelTestToolsToResponsesRequest(req, testPrompt)
			return req
		case channelconstant.EndpointTypeOpenAIResponseCompact:
			return &shared.OpenAIResponsesCompactionRequest{
				Model: model,
				Input: testResponsesInput,
			}
		case channelconstant.EndpointTypeAnthropic, channelconstant.EndpointTypeGemini, channelconstant.EndpointTypeOpenAI:
			maxTokens, maxCompletionTokens := resolveChannelTestTokenBudget(model, endpointType, testPrompt.isTool)
			req := &shared.GeneralOpenAIRequest{
				Model:  model,
				Stream: isStream,
				Messages: []shared.Message{
					{
						Role:    "user",
						Content: userPrompt,
					},
				},
				MaxTokens:           maxTokens,
				MaxCompletionTokens: maxCompletionTokens,
			}
			if isStream {
				req.StreamOptions = &shared.StreamOptions{IncludeUsage: true}
			}
			applyChannelTestToolsToChatRequest(req, testPrompt)
			return req
		}
	}

	// 自动检测逻辑（保持原有行为）
	if isChannelTestRerankModel(model) {
		return &shared.RerankRequest{
			Model:     model,
			Query:     "What is Deep Learning?",
			Documents: []any{"Deep Learning is a subset of machine learning.", "Machine learning is a field of artificial intelligence."},
			TopN:      2,
		}
	}

	if isChannelTestEmbeddingModel(model) {
		return &shared.EmbeddingRequest{
			Model: model,
			Input: []any{"hello world"},
		}
	}

	if strings.HasSuffix(model, ratio.CompactModelSuffix) {
		return &shared.OpenAIResponsesCompactionRequest{
			Model: model,
			Input: testResponsesInput,
		}
	}

	if strings.Contains(strings.ToLower(model), "codex") {
		req := &shared.OpenAIResponsesRequest{
			Model:  model,
			Input:  testResponsesInput,
			Stream: isStream,
		}
		applyChannelTestToolsToResponsesRequest(req, testPrompt)
		return req
	}

	testRequest := &shared.GeneralOpenAIRequest{
		Model:  model,
		Stream: isStream,
		Messages: []shared.Message{
			{
				Role:    "user",
				Content: userPrompt,
			},
		},
	}
	if isStream {
		testRequest.StreamOptions = &shared.StreamOptions{IncludeUsage: true}
	}

	maxTokens, maxCompletionTokens := resolveChannelTestTokenBudget(model, "", testPrompt.isTool)
	testRequest.MaxTokens = maxTokens
	testRequest.MaxCompletionTokens = maxCompletionTokens
	applyChannelTestToolsToChatRequest(testRequest, testPrompt)

	return testRequest
}

func supportsChannelTestTool(endpointType string, modelName string) bool {
	normalizedEndpoint := strings.TrimSpace(endpointType)
	if normalizedEndpoint != "" {
		switch channelconstant.EndpointType(normalizedEndpoint) {
		case channelconstant.EndpointTypeOpenAI,
			channelconstant.EndpointTypeOpenAIResponse,
			channelconstant.EndpointTypeAnthropic,
			channelconstant.EndpointTypeGemini:
			return true
		default:
			return false
		}
	}

	if isChannelTestRerankModel(modelName) ||
		isChannelTestEmbeddingModel(modelName) ||
		isChannelTestCompactModel(modelName) {
		return false
	}
	return true
}

// supportsChannelTestToolForChannel 在 supportsChannelTestTool 基础上叠加 channel/api type 维度判断。
// 用于避免对未实现 Responses 工具语义转换、或会丢弃 tools 配置的上游发送工具测试请求。
// 例如 Gemini/Vertex/AWS 当前没有实现 ConvertOpenAIResponsesRequest,AWS Nova 的 Chat 转换会丢弃
// tools/tool_choice,这些路径如果走工具测试会直接触发上游"输入不合规"类错误,应在本地拒绝。
func supportsChannelTestToolForChannel(channel *channelstore.Channel, endpointType string, modelName string) bool {
	if !supportsChannelTestTool(endpointType, modelName) {
		return false
	}
	if channel == nil {
		return true
	}

	apiType, _ := common.ChannelType2APIType(channel.Type)
	normalizedEndpoint := strings.TrimSpace(endpointType)
	resolvedEndpoint := normalizedEndpoint
	if resolvedEndpoint == "" {
		// auto detect: codex / compact 走 Responses 路径,其他走 Chat
		if strings.Contains(strings.ToLower(modelName), "codex") {
			resolvedEndpoint = string(channelconstant.EndpointTypeOpenAIResponse)
		}
	}

	// Responses 端点只允许已实现工具语义转换的 adaptor
	if resolvedEndpoint == string(channelconstant.EndpointTypeOpenAIResponse) {
		switch apiType {
		case channelconstant.APITypeOpenAI, channelconstant.APITypeAnthropic:
			return true
		default:
			return false
		}
	}

	// AWS Bedrock Nova 系列当前 convertToNovaRequest 会丢弃 tools/tool_choice
	if apiType == channelconstant.APITypeAws && isAwsNovaTestModel(modelName) {
		return false
	}

	return true
}

func isAwsNovaTestModel(modelName string) bool {
	return strings.Contains(strings.ToLower(modelName), "nova")
}

// Channel test token budgets.
//
// max_tokens is a ceiling rather than a target, so these values are
// deliberately generous: models that finish early stop on their own and the
// unused budget is never billed. The headroom lets reasoning models finish
// their chain-of-thought and emit the final answer instead of being truncated
// mid-thought (which produces an empty content body that fails the test).
const (
	channelTestPlainMaxTokens         uint = 10240
	channelTestPlainToolMaxTokens     uint = 10240
	channelTestReasoningMaxTokens     uint = 10240
	channelTestReasoningToolMaxTokens uint = 10240
	channelTestGeminiMaxTokens        uint = 10240
)

// isChannelTestOSeriesModel reports whether modelName is an OpenAI o-series
// reasoning model (o1, o3, o4, o5, o1-mini, o3-pro, ...). These models use
// max_completion_tokens (which includes reasoning tokens) instead of
// max_tokens.
func isChannelTestOSeriesModel(modelName string) bool {
	lower := strings.ToLower(strings.TrimSpace(modelName))
	if len(lower) < 2 || lower[0] != 'o' {
		return false
	}
	digit := lower[1]
	if digit < '1' || digit > '9' {
		return false
	}
	if len(lower) == 2 {
		return true
	}
	separator := lower[2]
	return separator == '-' || separator == '.' || separator == '_'
}

// isChannelTestReasoningModel reports whether modelName is a known
// reasoning/thinking model that needs a larger completion budget for
// chain-of-thought tokens. Claude is excluded because its extended-thinking
// budget is managed by the adaptor rather than a plain max_tokens cap.
func isChannelTestReasoningModel(modelName string) bool {
	lower := strings.ToLower(strings.TrimSpace(modelName))
	if lower == "" {
		return false
	}
	if strings.Contains(lower, "claude") {
		return false
	}
	if isChannelTestOSeriesModel(lower) {
		return true
	}
	for _, marker := range []string{"thinking", "reasoner", "reasoning", "qwq"} {
		if strings.Contains(lower, marker) {
			return true
		}
	}
	// R1-style names: deepseek-r1, qwen-r1, etc. Match with a leading
	// separator to reduce false positives on unrelated model names.
	for _, marker := range []string{"-r1", "_r1", "/r1", "deepseek-r"} {
		if strings.Contains(lower, marker) {
			return true
		}
	}
	return false
}

// resolveChannelTestTokenBudget returns the max_tokens / max_completion_tokens
// pair to use for a chat-completions style channel test. o-series models use
// max_completion_tokens (which includes reasoning tokens); everything else
// uses max_tokens.
func resolveChannelTestTokenBudget(modelName, endpointType string, isTool bool) (maxTokens uint, maxCompletionTokens uint) {
	normalizedEndpoint := channelconstant.EndpointType(strings.TrimSpace(endpointType))
	if normalizedEndpoint == channelconstant.EndpointTypeGemini ||
		strings.Contains(strings.ToLower(modelName), "gemini") {
		maxTokens = channelTestGeminiMaxTokens
		return
	}
	if isChannelTestOSeriesModel(modelName) {
		if isTool {
			maxCompletionTokens = channelTestReasoningToolMaxTokens
		} else {
			maxCompletionTokens = channelTestReasoningMaxTokens
		}
		return
	}
	if isChannelTestReasoningModel(modelName) {
		if isTool {
			maxTokens = channelTestReasoningToolMaxTokens
		} else {
			maxTokens = channelTestReasoningMaxTokens
		}
		return
	}
	if isTool {
		maxTokens = channelTestPlainToolMaxTokens
	} else {
		maxTokens = channelTestPlainMaxTokens
	}
	return
}

func isChannelTestRerankModel(modelName string) bool {
	return strings.Contains(strings.ToLower(modelName), "rerank")
}

func isChannelTestEmbeddingModel(modelName string) bool {
	lowerModelName := strings.ToLower(modelName)
	return strings.Contains(lowerModelName, "embedding") ||
		strings.HasPrefix(lowerModelName, "m3e") ||
		strings.Contains(lowerModelName, "bge-") ||
		strings.Contains(lowerModelName, "embed")
}

func isChannelTestCompactModel(modelName string) bool {
	return strings.HasSuffix(modelName, ratio.CompactModelSuffix)
}

func buildChannelTestPrompt(endpointType string, modelName string, isTool bool) channelTestPrompt {
	if isTool {
		if !supportsChannelTestTool(endpointType, modelName) {
			return channelTestPrompt{isTool: true}
		}
		return channelTestPrompt{
			prompt: "Call the report_result tool exactly once with argument value equal to 42. Do not write any other text outside tool calls.",
			isTool: true,
		}
	}

	if !requiresChannelTestTextAnswer(endpointType, modelName) {
		return channelTestPrompt{}
	}

	leftOperand, rightOperand, operator, expectedAnswer := generateChannelTestArithmetic()
	expression := fmt.Sprintf("%d%s%d", leftOperand, operator, rightOperand)
	// Keep the instruction between 50 and 100 characters so the model has clear
	// constraints without overflowing short completion budgets.
	// Example: "Calculate 12+34. Except private reasoning, reply only the final integer. No text/JSON/Markdown."
	prompt := fmt.Sprintf(
		"Calculate %s. Except private reasoning, reply only the final integer. No text/JSON/Markdown.",
		expression,
	)
	return channelTestPrompt{
		prompt:             prompt,
		expectedAnswer:     expectedAnswer,
		requiresTextAnswer: true,
	}
}

func requiresChannelTestTextAnswer(endpointType string, modelName string) bool {
	normalizedEndpoint := strings.TrimSpace(endpointType)
	if normalizedEndpoint != "" {
		switch channelconstant.EndpointType(normalizedEndpoint) {
		case channelconstant.EndpointTypeOpenAI,
			channelconstant.EndpointTypeOpenAIResponse,
			channelconstant.EndpointTypeAnthropic,
			channelconstant.EndpointTypeGemini:
			return true
		default:
			return false
		}
	}

	if isChannelTestRerankModel(modelName) ||
		isChannelTestEmbeddingModel(modelName) ||
		isChannelTestCompactModel(modelName) {
		return false
	}
	return true
}

func generateChannelTestArithmetic() (leftOperand int, rightOperand int, operator string, expectedAnswer int) {
	// Division is forced to divide evenly so the expected answer stays an integer.
	switch rand.Intn(4) {
	case 0:
		leftOperand = rand.Intn(100) + 1
		rightOperand = rand.Intn(100) + 1
		operator = "+"
		expectedAnswer = leftOperand + rightOperand
	case 1:
		leftOperand = rand.Intn(100) + 1
		rightOperand = rand.Intn(leftOperand) + 1
		operator = "-"
		expectedAnswer = leftOperand - rightOperand
	case 2:
		leftOperand = rand.Intn(12) + 1
		rightOperand = rand.Intn(12) + 1
		operator = "*"
		expectedAnswer = leftOperand * rightOperand
	default:
		rightOperand = rand.Intn(12) + 1
		expectedAnswer = rand.Intn(12) + 1
		leftOperand = rightOperand * expectedAnswer
		if leftOperand > 100 {
			expectedAnswer = 1 + rand.Intn(max(1, 100/rightOperand))
			leftOperand = rightOperand * expectedAnswer
		}
		operator = "/"
	}
	return leftOperand, rightOperand, operator, expectedAnswer
}

func applyChannelTestToolsToChatRequest(request *shared.GeneralOpenAIRequest, testPrompt channelTestPrompt) {
	if request == nil || !testPrompt.isTool {
		return
	}
	request.Tools = []shared.ToolCallRequest{
		{
			Type: "function",
			Function: shared.FunctionRequest{
				Name:        channelTestToolName,
				Description: "Report the final numeric result for the channel connectivity test.",
				Parameters: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"value": map[string]any{
							"type":        "integer",
							"description": "Final integer result",
						},
					},
					"required": []string{"value"},
				},
			},
		},
	}
	// OpenAI Chat 标准同时支持 {"type":"function","function":{"name":"report_result"}} 和 "required",
	// 但很多 OpenAI 兼容上游(部分代理、第三方网关、国产兼容实现)只接受字符串 tool_choice,
	// 在只声明一个工具的前提下 "required" 与强制命名工具语义等价,且兼容性更高。
	// Claude/Gemini adaptor 都正确把 "required" 映射为 anthropic {"type":"any"} 与 gemini mode:"ANY"。
	request.ToolChoice = "required"
}

func applyChannelTestToolsToResponsesRequest(request *shared.OpenAIResponsesRequest, testPrompt channelTestPrompt) {
	if request == nil || !testPrompt.isTool {
		return
	}
	toolsJSON, err := jsonx.Marshal([]map[string]any{
		{
			"type":        "function",
			"name":        channelTestToolName,
			"description": "Report the final numeric result for the channel connectivity test.",
			"parameters": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"value": map[string]any{
						"type":        "integer",
						"description": "Final integer result",
					},
				},
				"required": []string{"value"},
			},
		},
	})
	if err != nil {
		return
	}
	// OpenAI Responses 同样接受字符串 tool_choice,在只有一个工具时与命名工具等价。
	// Claude 走 Responses->Chat->Claude 转换时也能正确处理字符串 tool_choice。
	toolChoiceJSON, err := jsonx.Marshal("required")
	if err != nil {
		return
	}
	request.Tools = toolsJSON
	request.ToolChoice = toolChoiceJSON
}

func validateChannelTestResponse(c *gin.Context, respBody []byte, testPrompt channelTestPrompt) error {
	if testPrompt.isTool {
		if responseHasChannelTestToolCall(respBody) {
			return nil
		}
		originalText := strings.TrimSpace(extractChannelTestAIText(respBody))
		if originalText == "" {
			return errors.New(i18n.T(c, channelTestToolNotSupported))
		}
		return fmt.Errorf("%s\n%s", i18n.T(c, channelTestToolNotSupported), originalText)
	}

	if !testPrompt.requiresTextAnswer {
		return nil
	}

	originalText := strings.TrimSpace(extractChannelTestAIText(respBody))
	if originalText == "" {
		// Content is empty. If the model produced private reasoning instead,
		// it is a reasoning/thinking model that exhausted its token budget on
		// chain-of-thought before emitting the final answer — surface a clear,
		// actionable message instead of the generic "empty response".
		if reasoningText := strings.TrimSpace(extractChannelTestReasoningText(respBody)); reasoningText != "" {
			return errors.New(i18n.T(c, i18n.MsgChannelReasoningOnlyResponse))
		}
		return errors.New(i18n.T(c, i18n.MsgChannelEmptyModelResponse))
	}
	if !matchesChannelTestExpectedAnswer(originalText, testPrompt.expectedAnswer) {
		// Return the raw AI text as the error message so the UI can show it
		// directly in toast/status instead of structured JSON.
		return errors.New(originalText)
	}
	return nil
}

func classifyChannelTestValidationError(err error, testPrompt channelTestPrompt) shared.ErrorCode {
	if err == nil {
		return shared.ErrorCodeBadResponseBody
	}
	if testPrompt.isTool {
		return shared.ErrorCodeChannelTestToolUnsupported
	}
	return shared.ErrorCodeBadResponseBody
}

func responseHasChannelTestToolCall(respBody []byte) bool {
	b := bytes.TrimSpace(respBody)
	if len(b) == 0 {
		return false
	}

	if hasChannelTestToolCallInJSON(b) {
		return true
	}

	for _, line := range bytes.Split(b, []byte{'\n'}) {
		line = bytes.TrimSpace(line)
		if len(line) == 0 {
			continue
		}
		payload := line
		if bytes.HasPrefix(line, []byte("data:")) {
			payload = bytes.TrimSpace(bytes.TrimPrefix(line, []byte("data:")))
		}
		if len(payload) == 0 || bytes.Equal(payload, []byte("[DONE]")) {
			continue
		}
		if hasChannelTestToolCallInJSON(payload) {
			return true
		}
	}
	return false
}

func hasChannelTestToolCallInJSON(jsonBytes []byte) bool {
	if len(jsonBytes) == 0 || (jsonBytes[0] != '{' && jsonBytes[0] != '[') {
		return false
	}

	// Chat Completions non-stream and stream deltas.
	for _, path := range []string{
		"choices.#.message.tool_calls.#.function.name",
		"choices.#.delta.tool_calls.#.function.name",
		"choices.0.message.tool_calls.0.function.name",
		"choices.0.delta.tool_calls.0.function.name",
	} {
		result := gjson.GetBytes(jsonBytes, path)
		if result.Exists() && containsChannelTestToolName(result) {
			return true
		}
	}

	// OpenAI Responses style.
	for _, path := range []string{
		"output.#.name",
		"output.#.type",
		"item.name",
		"item.type",
		"response.output.#.name",
		"response.output.#.type",
	} {
		result := gjson.GetBytes(jsonBytes, path)
		if !result.Exists() {
			continue
		}
		if containsChannelTestToolName(result) {
			return true
		}
		if result.IsArray() {
			for _, item := range result.Array() {
				if strings.Contains(strings.ToLower(item.String()), "function_call") ||
					strings.Contains(strings.ToLower(item.String()), "tool_call") {
					// If a function/tool call exists, also require the expected tool name when present.
					if gjson.GetBytes(jsonBytes, "output.#.name").Exists() ||
						gjson.GetBytes(jsonBytes, "item.name").Exists() ||
						gjson.GetBytes(jsonBytes, "response.output.#.name").Exists() {
						if containsChannelTestToolName(gjson.GetBytes(jsonBytes, "output.#.name")) ||
							containsChannelTestToolName(gjson.GetBytes(jsonBytes, "item.name")) ||
							containsChannelTestToolName(gjson.GetBytes(jsonBytes, "response.output.#.name")) {
							return true
						}
						continue
					}
					return true
				}
			}
		} else if strings.Contains(strings.ToLower(result.String()), "function_call") ||
			strings.Contains(strings.ToLower(result.String()), "tool_call") {
			return true
		}
	}

	// Claude style content blocks.
	contentTypes := gjson.GetBytes(jsonBytes, "content.#.type")
	if contentTypes.Exists() {
		for _, item := range contentTypes.Array() {
			if item.String() == "tool_use" {
				names := gjson.GetBytes(jsonBytes, "content.#.name")
				if containsChannelTestToolName(names) {
					return true
				}
			}
		}
	}

	// Gemini style function calls.
	functionNames := gjson.GetBytes(jsonBytes, "candidates.#.content.parts.#.functionCall.name")
	if containsChannelTestToolName(functionNames) {
		return true
	}
	functionNames = gjson.GetBytes(jsonBytes, "candidates.#.content.parts.#.function_call.name")
	return containsChannelTestToolName(functionNames)
}

func containsChannelTestToolName(result gjson.Result) bool {
	if !result.Exists() {
		return false
	}
	if result.IsArray() {
		for _, item := range result.Array() {
			if item.String() == channelTestToolName {
				return true
			}
		}
		return false
	}
	return result.String() == channelTestToolName
}

func extractChannelTestAIText(respBody []byte) string {
	b := bytes.TrimSpace(respBody)
	if len(b) == 0 {
		return ""
	}

	if isChannelTestJSONPayload(b) {
		if text := extractChannelTestAITextFromJSON(b); text != "" {
			return text
		}
	}

	textFromEvents := extractChannelTestAITextFromEventStream(b)
	if textFromEvents != "" {
		return textFromEvents
	}

	if isChannelTestEventStreamPayload(b) {
		return ""
	}

	return strings.TrimSpace(string(b))
}

func extractChannelTestAITextFromEventStream(eventStreamBytes []byte) string {
	var builder strings.Builder
	for _, line := range bytes.Split(eventStreamBytes, []byte{'\n'}) {
		line = bytes.TrimSpace(line)
		if len(line) == 0 {
			continue
		}

		payload := line
		if bytes.HasPrefix(line, []byte("data:")) {
			payload = bytes.TrimSpace(bytes.TrimPrefix(line, []byte("data:")))
		} else if !isChannelTestJSONPayload(line) {
			continue
		}

		if len(payload) == 0 || bytes.Equal(payload, []byte("[DONE]")) {
			continue
		}
		if text := extractChannelTestAITextFromJSON(payload); text != "" {
			builder.WriteString(text)
		}
	}
	return strings.TrimSpace(builder.String())
}

func isChannelTestJSONPayload(payload []byte) bool {
	trimmedPayload := bytes.TrimSpace(payload)
	return len(trimmedPayload) > 0 && (trimmedPayload[0] == '{' || trimmedPayload[0] == '[')
}

func isChannelTestEventStreamPayload(payload []byte) bool {
	for _, line := range bytes.Split(payload, []byte{'\n'}) {
		line = bytes.TrimSpace(line)
		if bytes.HasPrefix(line, []byte("data:")) {
			return true
		}
	}
	return false
}

func extractChannelTestAITextFromJSON(jsonBytes []byte) string {
	if len(jsonBytes) == 0 || !isChannelTestJSONPayload(jsonBytes) {
		return ""
	}

	paths := []string{
		"choices.0.message.content",
		"choices.0.delta.content",
		"choices.0.text",
		"output_text",
		"output.#.content.#.text",
		"response.output_text",
		"response.output.#.content.#.text",
		"content.0.text",
		"delta",
		"text",
		"candidates.0.content.parts.0.text",
	}

	var parts []string
	for _, path := range paths {
		result := gjson.GetBytes(jsonBytes, path)
		if !result.Exists() {
			continue
		}
		if result.IsArray() {
			for _, item := range result.Array() {
				if text := strings.TrimSpace(item.String()); text != "" {
					parts = append(parts, text)
				}
			}
			continue
		}
		if text := strings.TrimSpace(result.String()); text != "" {
			parts = append(parts, text)
		}
	}
	if len(parts) == 0 {
		return ""
	}
	return strings.TrimSpace(strings.Join(parts, ""))
}

// extractChannelTestReasoningText pulls private chain-of-thought text
// (reasoning_content / reasoning) from a non-streaming JSON body or an SSE
// stream. It mirrors extractChannelTestAIText but targets the reasoning
// fields so validateChannelTestResponse can distinguish a reasoning model
// that exhausted its token budget (empty content, non-empty reasoning) from
// a genuinely empty response.
func extractChannelTestReasoningText(respBody []byte) string {
	b := bytes.TrimSpace(respBody)
	if len(b) == 0 {
		return ""
	}
	if isChannelTestJSONPayload(b) {
		if text := extractChannelTestReasoningTextFromJSON(b); text != "" {
			return text
		}
	}
	return extractChannelTestReasoningTextFromEventStream(b)
}

func extractChannelTestReasoningTextFromEventStream(eventStreamBytes []byte) string {
	var builder strings.Builder
	for _, line := range bytes.Split(eventStreamBytes, []byte{'\n'}) {
		line = bytes.TrimSpace(line)
		if len(line) == 0 {
			continue
		}
		payload := line
		if bytes.HasPrefix(line, []byte("data:")) {
			payload = bytes.TrimSpace(bytes.TrimPrefix(line, []byte("data:")))
		} else if !isChannelTestJSONPayload(line) {
			continue
		}
		if len(payload) == 0 || bytes.Equal(payload, []byte("[DONE]")) {
			continue
		}
		if text := extractChannelTestReasoningTextFromJSON(payload); text != "" {
			builder.WriteString(text)
		}
	}
	return strings.TrimSpace(builder.String())
}

func extractChannelTestReasoningTextFromJSON(jsonBytes []byte) string {
	if len(jsonBytes) == 0 || !isChannelTestJSONPayload(jsonBytes) {
		return ""
	}
	paths := []string{
		"choices.0.message.reasoning_content",
		"choices.0.message.reasoning",
		"choices.0.delta.reasoning_content",
		"choices.0.delta.reasoning",
	}
	var parts []string
	for _, path := range paths {
		result := gjson.GetBytes(jsonBytes, path)
		if !result.Exists() {
			continue
		}
		if text := strings.TrimSpace(result.String()); text != "" {
			parts = append(parts, text)
		}
	}
	return strings.TrimSpace(strings.Join(parts, ""))
}

func matchesChannelTestExpectedAnswer(originalText string, expectedAnswer int) bool {
	normalized := normalizeChannelTestAnswerText(originalText)
	expected := strconv.Itoa(expectedAnswer)
	if normalized == expected {
		return true
	}
	// Accept a bare integer with optional surrounding punctuation only.
	// Any extra words should fail so the UI surfaces the original AI text.
	trimmedPunctuation := strings.Trim(normalized, ".,;:!?。；：！？")
	return trimmedPunctuation == expected
}

func normalizeChannelTestAnswerText(text string) string {
	trimmed := strings.TrimSpace(text)
	trimmed = strings.Trim(trimmed, "`\"'")
	trimmed = strings.TrimSpace(trimmed)
	// Collapse whitespace so multi-line answers compare cleanly.
	var builder strings.Builder
	previousWasSpace := false
	for _, character := range trimmed {
		if unicode.IsSpace(character) {
			if !previousWasSpace {
				builder.WriteByte(' ')
				previousWasSpace = true
			}
			continue
		}
		builder.WriteRune(character)
		previousWasSpace = false
	}
	return strings.TrimSpace(builder.String())
}

func writeChannelTestJSON(c *gin.Context, success bool, message string, errorCode shared.ErrorCode, consumedTime float64) {
	payload := gin.H{
		"success": success,
		"message": message,
		"time":    consumedTime,
	}
	if errorCode != "" {
		payload["error_code"] = string(errorCode)
	}
	c.JSON(http.StatusOK, payload)
}

func TestChannel(c *gin.Context) {
	channelId, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		httpapi.ApiErrorI18n(c, i18n.MsgChannelIDFormatError, map[string]any{"Error": err.Error()})
		return
	}
	channel, err := channelstore.CacheGetChannel(channelId)
	if err != nil {
		channel, err = channelstore.GetChannelById(channelId, true)
		if err != nil {
			common.SysError("failed to get channel by id: " + err.Error())
			httpapi.ApiErrorI18n(c, i18n.MsgDatabaseError)
			return
		}
	}
	testModel := c.Query("model")
	endpointType := c.Query("endpoint_type")
	isStream, _ := strconv.ParseBool(c.Query("stream"))
	isTool, _ := strconv.ParseBool(c.Query("tool"))
	testUserID, err := resolveChannelTestUserID(c)
	if err != nil {
		common.SysError("failed to resolve channel test user: " + err.Error())
		httpapi.ApiErrorI18n(c, i18n.MsgDatabaseError)
		return
	}
	tik := time.Now()
	result := testChannel(channel, testUserID, testModel, endpointType, isStream, isTool)
	if result.localErr != nil {
		errorCode := shared.ErrorCode("")
		if result.newAPIError != nil {
			errorCode = result.newAPIError.GetErrorCode()
		}
		writeChannelTestJSON(c, false, result.localErr.Error(), errorCode, 0)
		return
	}
	tok := time.Now()
	milliseconds := tok.Sub(tik).Milliseconds()
	go channel.UpdateResponseTime(milliseconds)
	consumedTime := float64(milliseconds) / 1000.0
	if result.newAPIError != nil {
		writeChannelTestJSON(c, false, result.newAPIError.Error(), result.newAPIError.GetErrorCode(), consumedTime)
		return
	}
	writeChannelTestJSON(c, true, "", "", consumedTime)
}

var testAllChannelsLock sync.Mutex
var testAllChannelsRunning bool = false

func testAllChannels(c *gin.Context, notify bool) error {
	testUserID, err := resolveChannelTestUserID(nil)
	if err != nil {
		return err
	}

	testAllChannelsLock.Lock()
	if testAllChannelsRunning {
		testAllChannelsLock.Unlock()
		return errors.New(i18n.T(c, i18n.MsgChannelTestAlreadyRunning))
	}
	testAllChannelsRunning = true
	testAllChannelsLock.Unlock()
	channels, getChannelErr := channelstore.GetAllChannels(0, 0, true, false)
	if getChannelErr != nil {
		return getChannelErr
	}
	var disableThreshold = int64(common.ChannelDisableThreshold * 1000)
	if disableThreshold == 0 {
		disableThreshold = 10000000 // a impossible value
	}
	// gin 在 HTTP handler 返回后会将请求级 Context 归还对象池并被后续请求复用，
	// 后台 goroutine 不得再触碰；因此在派生前同步解析语言，闭包内改用按语言翻译入口。
	lang := i18n.GetLangFromContext(c)
	runtime.RelayGo(func() {
		// 使用 defer 确保无论如何都会重置运行状态，防止死锁
		defer func() {
			testAllChannelsLock.Lock()
			testAllChannelsRunning = false
			testAllChannelsLock.Unlock()
		}()

		for _, channel := range channels {
			isChannelEnabled := channel.Status == common.ChannelStatusEnabled
			tik := time.Now()
			result := testChannel(channel, testUserID, "", "", false, false)
			tok := time.Now()
			milliseconds := tok.Sub(tik).Milliseconds()

			shouldBanChannel := false
			newAPIError := result.newAPIError
			// request error disables the channel
			if newAPIError != nil {
				shouldBanChannel = domainchannel.ShouldDisableChannel(channel.Type, result.newAPIError)
			}

			// 当错误检查通过，才检查响应时间
			if common.AutomaticDisableChannelEnabled && !shouldBanChannel {
				if milliseconds > disableThreshold {
					err := fmt.Errorf("%s", i18n.Translate(lang, i18n.MsgChannelResponseTimeExceeded, map[string]any{
						"Response":  fmt.Sprintf("%.2f", float64(milliseconds)/1000.0),
						"Threshold": fmt.Sprintf("%.2f", float64(disableThreshold)/1000.0),
					}))
					newAPIError = shared.NewOpenAIError(err, shared.ErrorCodeChannelResponseTimeExceeded, http.StatusRequestTimeout)
					shouldBanChannel = true
				}
			}

			// disable channel
			if isChannelEnabled && shouldBanChannel && channel.GetAutoBan() {
				relaycontroller.ProcessChannelError(result.context, *domainchannel.NewChannelError(channel.Id, channel.Type, channel.Name, channel.ChannelInfo.IsMultiKey, httpapi.GetContextKeyString(result.context, common.ContextKeyChannelKey), channel.GetAutoBan()), newAPIError)
			}

			// enable channel
			if !isChannelEnabled && domainchannel.ShouldEnableChannel(newAPIError, channel.Status) {
				domainchannel.EnableChannel(channel.Id, httpapi.GetContextKeyString(result.context, common.ContextKeyChannelKey), channel.Name, lang)
			}

			channel.UpdateResponseTime(milliseconds)
			time.Sleep(common.RequestInterval)
		}

		if notify {
			domainnotify.NotifyRootUser(shared.NotifyTypeChannelTest,
				i18n.Translate(lang, i18n.MsgChannelNotifyTestCompletedTitle),
				i18n.Translate(lang, i18n.MsgChannelNotifyTestCompletedBody))
		}
	})
	return nil
}

func TestAllChannels(c *gin.Context) {
	err := testAllChannels(c, true)
	if err != nil {
		common.SysError("failed to test all channels: " + err.Error())
		httpapi.ApiErrorI18n(c, i18n.MsgDatabaseError)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
	})
}

var autoTestChannelsOnce sync.Once

func AutomaticallyTestChannels() {
	// 只在Master节点定时测试渠道
	if !common.IsMasterNode {
		return
	}
	autoTestChannelsOnce.Do(func() {
		for {
			if !operation.GetMonitorSetting().AutoTestChannelEnabled {
				time.Sleep(1 * time.Minute)
				continue
			}
			for {
				frequency := operation.GetMonitorSetting().AutoTestChannelMinutes
				time.Sleep(time.Duration(int(math.Round(frequency))) * time.Minute)
				common.SysLog(fmt.Sprintf("automatically test channels with interval %f minutes", frequency))
				common.SysLog("automatically testing all channels")
				_ = testAllChannels(nil, false)
				common.SysLog("automatically channel test finished")
				if !operation.GetMonitorSetting().AutoTestChannelEnabled {
					break
				}
			}
		}
	})
}
