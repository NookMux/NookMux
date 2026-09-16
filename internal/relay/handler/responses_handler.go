package handler

import (
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/NookMux/NookMux/internal/common"
	appconstant "github.com/NookMux/NookMux/internal/domain/channel/constant"
	"github.com/NookMux/NookMux/internal/domain/shared"
	relaycommon "github.com/NookMux/NookMux/internal/relay/common"
	relayconstant "github.com/NookMux/NookMux/internal/relay/constant"
	"github.com/NookMux/NookMux/internal/relay/core"
	"github.com/NookMux/NookMux/internal/relay/helper"

	"github.com/NookMux/NookMux/pkg/jsonx"

	billing "github.com/NookMux/NookMux/internal/domain/billing"
	"github.com/NookMux/NookMux/internal/httpapi"
	"github.com/NookMux/NookMux/internal/infra/cache"
	"github.com/gin-gonic/gin"
)

func ResponsesHelper(c *gin.Context, info *relaycommon.RelayInfo) (newAPIError *shared.NookMuxError) {
	info.InitChannelMeta(c)
	if info.RelayMode == relayconstant.RelayModeResponsesCompact {
		switch info.ApiType {
		case appconstant.APITypeOpenAI:
		default:
			return shared.NewErrorWithStatusCode(
				fmt.Errorf("unsupported endpoint %q for api type %d", "/v1/responses/compact", info.ApiType),
				shared.ErrorCodeInvalidRequest,
				http.StatusBadRequest,
				shared.ErrOptionWithSkipRetry(),
			)
		}
	}

	var responsesReq *shared.OpenAIResponsesRequest
	switch req := info.Request.(type) {
	case *shared.OpenAIResponsesRequest:
		responsesReq = req
	case *shared.OpenAIResponsesCompactionRequest:
		responsesReq = &shared.OpenAIResponsesRequest{
			Model:              req.Model,
			Input:              req.Input,
			Instructions:       req.Instructions,
			PreviousResponseID: req.PreviousResponseID,
		}
	default:
		return shared.NewErrorWithStatusCode(
			fmt.Errorf("invalid request type, expected shared.OpenAIResponsesRequest or shared.OpenAIResponsesCompactionRequest, got %T", info.Request),
			shared.ErrorCodeInvalidRequest,
			http.StatusBadRequest,
			shared.ErrOptionWithSkipRetry(),
		)
	}

	request, err := common.DeepCopy(responsesReq)
	if err != nil {
		return shared.NewError(fmt.Errorf("failed to copy request to GeneralOpenAIRequest: %w", err), shared.ErrorCodeInvalidRequest, shared.ErrOptionWithSkipRetry())
	}

	err = helper.ModelMappedHelper(c, info, request)
	if err != nil {
		return shared.NewError(err, shared.ErrorCodeChannelModelMappedError, shared.ErrOptionWithSkipRetry())
	}

	adaptor := core.GetAdaptor(info.ApiType)
	if adaptor == nil {
		return shared.NewError(fmt.Errorf("invalid api type: %d", info.ApiType), shared.ErrorCodeInvalidApiType, shared.ErrOptionWithSkipRetry())
	}
	adaptor.Init(info)
	var requestBody io.Reader
	if info.ChannelSetting.PassThroughBodyEnabled {
		storage, err := httpapi.GetBodyStorage(c)
		if err != nil {
			return shared.NewError(err, shared.ErrorCodeReadRequestBodyFailed, shared.ErrOptionWithSkipRetry())
		}
		info.UpstreamRequestBodySize = storage.Size()
		requestBody = cache.ReaderOnly(storage)
	} else {
		convertedRequest, err := adaptor.ConvertOpenAIResponsesRequest(c, info, *request)
		if err != nil {
			return shared.NewError(err, shared.ErrorCodeConvertRequestFailed, shared.ErrOptionWithSkipRetry())
		}
		relaycommon.AppendRequestConversionFromRequest(info, convertedRequest)
		jsonData, err := jsonx.Marshal(convertedRequest)
		if err != nil {
			return shared.NewError(err, shared.ErrorCodeConvertRequestFailed, shared.ErrOptionWithSkipRetry())
		}

		// remove disabled fields for OpenAI Responses API
		jsonData, err = relaycommon.RemoveDisabledFields(jsonData, info.ChannelOtherSettings)
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

		if common.DebugEnabled {
			println("requestBody: ", string(jsonData))
		}
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

	var httpResp *http.Response
	resp, err := adaptor.DoRequest(c, info, requestBody)
	if err != nil {
		return shared.NewOpenAIError(err, shared.ErrorCodeDoRequestFailed, http.StatusInternalServerError)
	}

	statusCodeMappingStr := c.GetString("status_code_mapping")

	if resp != nil {
		httpResp = resp.(*http.Response)

		if httpResp.StatusCode != http.StatusOK {
			newAPIError = helper.RelayErrorHandler(c.Request.Context(), httpResp, false)
			// reset status code 重置状态码
			helper.ResetStatusCode(newAPIError, statusCodeMappingStr)
			return newAPIError
		}
	}

	usage, newAPIError := adaptor.DoResponse(c, httpResp, info)
	if newAPIError != nil {
		// reset status code 重置状态码
		helper.ResetStatusCode(newAPIError, statusCodeMappingStr)
		return newAPIError
	}

	usageDto := usage.(*shared.Usage)
	if info.RelayMode == relayconstant.RelayModeResponsesCompact {
		originModelName := info.OriginModelName
		originPriceData := info.PriceData

		_, err := helper.ModelPriceHelper(c, info, info.GetEstimatePromptTokens(), &shared.TokenCountMeta{})
		if err != nil {
			info.OriginModelName = originModelName
			info.PriceData = originPriceData
			return shared.NewError(err, shared.ErrorCodeModelPriceError, shared.ErrOptionWithSkipRetry())
		}
		settlement, apiErr := billing.CalculateUsage(c, info, usageDto)
		if apiErr != nil {
			info.OriginModelName = originModelName
			info.PriceData = originPriceData
			return apiErr
		}
		if apiErr := billing.ApplyQuota(c, info, settlement); apiErr != nil {
			info.OriginModelName = originModelName
			info.PriceData = originPriceData
			return apiErr
		}

		info.OriginModelName = originModelName
		info.PriceData = originPriceData
		return nil
	}

	if strings.HasPrefix(info.OriginModelName, "gpt-4o-audio") {
		if apiErr := billing.PostAudioConsumeQuota(c, info, usageDto, ""); apiErr != nil {
			return apiErr
		}
	} else {
		settlement, apiErr := billing.CalculateUsage(c, info, usageDto)
		if apiErr != nil {
			return apiErr
		}
		if apiErr := billing.ApplyQuota(c, info, settlement); apiErr != nil {
			return apiErr
		}
	}
	return nil
}
