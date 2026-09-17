package handler

import (
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/NookMux/NookMux/internal/common"
	billing "github.com/NookMux/NookMux/internal/domain/billing"
	"github.com/NookMux/NookMux/internal/domain/shared"
	"github.com/NookMux/NookMux/internal/infra/log"
	relaycommon "github.com/NookMux/NookMux/internal/relay/common"
	"github.com/NookMux/NookMux/internal/relay/core"
	"github.com/NookMux/NookMux/internal/relay/helper"

	"github.com/NookMux/NookMux/pkg/jsonx"

	"github.com/NookMux/NookMux/internal/httpapi"
	"github.com/NookMux/NookMux/internal/infra/cache"
	"github.com/gin-gonic/gin"
)

func GeminiHelper(c *gin.Context, info *relaycommon.RelayInfo) (newAPIError *shared.NookMuxError) {
	info.InitChannelMeta(c)

	geminiReq, ok := info.Request.(*shared.GeminiChatRequest)
	if !ok {
		return shared.NewErrorWithStatusCode(fmt.Errorf("invalid request type, expected *shared.GeminiChatRequest, got %T", info.Request), shared.ErrorCodeInvalidRequest, http.StatusBadRequest, shared.ErrOptionWithSkipRetry())
	}

	request, err := common.DeepCopy(geminiReq)
	if err != nil {
		return shared.NewError(fmt.Errorf("failed to copy request to GeminiChatRequest: %w", err), shared.ErrorCodeInvalidRequest, shared.ErrOptionWithSkipRetry())
	}

	// model mapped 模型映射
	err = helper.ModelMappedHelper(c, info, request)
	if err != nil {
		return shared.NewError(err, shared.ErrorCodeChannelModelMappedError, shared.ErrOptionWithSkipRetry())
	}

	adaptor := core.GetAdaptor(info.ApiType)
	if adaptor == nil {
		return shared.NewError(fmt.Errorf("invalid api type: %d", info.ApiType), shared.ErrorCodeInvalidApiType, shared.ErrOptionWithSkipRetry())
	}

	adaptor.Init(info)

	// Clean up empty system instruction
	if request.SystemInstructions != nil {
		hasContent := false
		for _, part := range request.SystemInstructions.Parts {
			if part.Text != "" {
				hasContent = true
				break
			}
		}
		if !hasContent {
			request.SystemInstructions = nil
		}
	}

	var requestBody io.Reader
	if info.ChannelSetting.PassThroughBodyEnabled {
		storage, err := httpapi.GetBodyStorage(c)
		if err != nil {
			return shared.NewErrorWithStatusCode(err, shared.ErrorCodeReadRequestBodyFailed, http.StatusBadRequest, shared.ErrOptionWithSkipRetry())
		}
		info.UpstreamRequestBodySize = storage.Size()
		requestBody = cache.ReaderOnly(storage)
	} else {
		// 使用 ConvertGeminiRequest 转换请求格式
		convertedRequest, err := adaptor.ConvertGeminiRequest(c, info, request)
		if err != nil {
			return shared.NewError(err, shared.ErrorCodeConvertRequestFailed, shared.ErrOptionWithSkipRetry())
		}
		relaycommon.AppendRequestConversionFromRequest(info, convertedRequest)
		jsonData, err := jsonx.Marshal(convertedRequest)
		if err != nil {
			return shared.NewError(err, shared.ErrorCodeConvertRequestFailed, shared.ErrOptionWithSkipRetry())
		}

		// apply OpenRouter provider routing preferences (channel overrides client)
		jsonData, err = relaycommon.ApplyProviderRouting(jsonData, info)
		if err != nil {
			return shared.NewError(err, shared.ErrorCodeConvertRequestFailed, shared.ErrOptionWithSkipRetry())
		}

		// apply param override
		if len(info.ParamOverride) > 0 {
			jsonData, err = relaycommon.ApplyParamOverrideWithRelayInfo(jsonData, info)
			if err != nil {
				return shared.NewError(err, shared.ErrorCodeChannelParamOverrideInvalid, shared.ErrOptionWithSkipRetry())
			}
		}

		log.LogDebug(c, "Gemini request body: "+string(jsonData))

		body, size, closer, err := relaycommon.NewOutboundJSONBody(jsonData)
		if err != nil {
			return shared.NewError(err, shared.ErrorCodeConvertRequestFailed, shared.ErrOptionWithSkipRetry())
		}
		defer closer.Close()
		info.UpstreamRequestBodySize = size
		requestBody = body
	}

	// 通用思维强度解析：adaptor 未设置时从原始请求兜底提取
	relaycommon.EnsureReasoningEffort(info, info.Request)

	resp, err := adaptor.DoRequest(c, info, requestBody)
	if err != nil {
		log.LogError(c, "Do gemini request failed: "+err.Error())
		return shared.NewOpenAIError(err, shared.ErrorCodeDoRequestFailed, http.StatusInternalServerError)
	}

	statusCodeMappingStr := c.GetString("status_code_mapping")

	var httpResp *http.Response
	if resp != nil {
		httpResp = resp.(*http.Response)
		info.IsStream = info.IsStream || strings.HasPrefix(httpResp.Header.Get("Content-Type"), "text/event-stream")
		if httpResp.StatusCode != http.StatusOK {
			newAPIError = helper.RelayErrorHandler(c.Request.Context(), httpResp, false)
			// reset status code 重置状态码
			helper.ResetStatusCode(newAPIError, statusCodeMappingStr)
			return newAPIError
		}
	}

	usage, openaiErr := adaptor.DoResponse(c, resp.(*http.Response), info)
	if openaiErr != nil {
		helper.ResetStatusCode(openaiErr, statusCodeMappingStr)
		return openaiErr
	}

	settlement, apiErr := billing.CalculateUsage(c, info, usage.(*shared.Usage))
	if apiErr != nil {
		return apiErr
	}
	if apiErr := billing.ApplyQuota(c, info, settlement); apiErr != nil {
		return apiErr
	}
	return nil
}

func GeminiEmbeddingHandler(c *gin.Context, info *relaycommon.RelayInfo) (newAPIError *shared.NookMuxError) {
	info.InitChannelMeta(c)

	isBatch := strings.HasSuffix(c.Request.URL.Path, "batchEmbedContents")
	info.IsGeminiBatchEmbedding = isBatch

	var req shared.Request
	var err error

	if isBatch {
		batchRequest := &shared.GeminiBatchEmbeddingRequest{}
		err = httpapi.UnmarshalBodyReusable(c, batchRequest)
		if err != nil {
			return shared.NewError(err, shared.ErrorCodeInvalidRequest, shared.ErrOptionWithSkipRetry())
		}
		req = batchRequest
	} else {
		singleRequest := &shared.GeminiEmbeddingRequest{}
		err = httpapi.UnmarshalBodyReusable(c, singleRequest)
		if err != nil {
			return shared.NewError(err, shared.ErrorCodeInvalidRequest, shared.ErrOptionWithSkipRetry())
		}
		req = singleRequest
	}

	err = helper.ModelMappedHelper(c, info, req)
	if err != nil {
		return shared.NewError(err, shared.ErrorCodeChannelModelMappedError, shared.ErrOptionWithSkipRetry())
	}

	req.SetModelName("models/" + info.UpstreamModelName)

	adaptor := core.GetAdaptor(info.ApiType)
	if adaptor == nil {
		return shared.NewError(fmt.Errorf("invalid api type: %d", info.ApiType), shared.ErrorCodeInvalidApiType, shared.ErrOptionWithSkipRetry())
	}
	adaptor.Init(info)

	var requestBody io.Reader
	jsonData, err := jsonx.Marshal(req)
	if err != nil {
		return shared.NewError(err, shared.ErrorCodeConvertRequestFailed, shared.ErrOptionWithSkipRetry())
	}

	// apply param override
	if len(info.ParamOverride) > 0 {
		jsonData, err = relaycommon.ApplyParamOverrideWithRelayInfo(jsonData, info)
		if err != nil {
			return shared.NewError(err, shared.ErrorCodeChannelParamOverrideInvalid, shared.ErrOptionWithSkipRetry())
		}
	}
	log.LogDebug(c, "Gemini embedding request body: "+string(jsonData))
	body, size, closer, err := relaycommon.NewOutboundJSONBody(jsonData)
	if err != nil {
		return shared.NewError(err, shared.ErrorCodeConvertRequestFailed, shared.ErrOptionWithSkipRetry())
	}
	defer closer.Close()
	info.UpstreamRequestBodySize = size
	requestBody = body

	resp, err := adaptor.DoRequest(c, info, requestBody)
	if err != nil {
		log.LogError(c, "Do gemini request failed: "+err.Error())
		return shared.NewOpenAIError(err, shared.ErrorCodeDoRequestFailed, http.StatusInternalServerError)
	}

	statusCodeMappingStr := c.GetString("status_code_mapping")
	var httpResp *http.Response
	if resp != nil {
		httpResp = resp.(*http.Response)
		if httpResp.StatusCode != http.StatusOK {
			newAPIError = helper.RelayErrorHandler(c.Request.Context(), httpResp, false)
			helper.ResetStatusCode(newAPIError, statusCodeMappingStr)
			return newAPIError
		}
	}

	usage, openaiErr := adaptor.DoResponse(c, resp.(*http.Response), info)
	if openaiErr != nil {
		helper.ResetStatusCode(openaiErr, statusCodeMappingStr)
		return openaiErr
	}

	settlement, apiErr := billing.CalculateUsage(c, info, usage.(*shared.Usage))
	if apiErr != nil {
		return apiErr
	}
	if apiErr := billing.ApplyQuota(c, info, settlement); apiErr != nil {
		return apiErr
	}
	return nil
}
