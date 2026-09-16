package handler

import (
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/NookMux/NookMux/internal/common"
	"github.com/NookMux/NookMux/internal/config/model"
	"github.com/NookMux/NookMux/internal/domain/shared"
	relaycommon "github.com/NookMux/NookMux/internal/relay/common"
	"github.com/NookMux/NookMux/internal/relay/core"
	"github.com/NookMux/NookMux/internal/relay/helper"

	"github.com/NookMux/NookMux/pkg/jsonx"

	billing "github.com/NookMux/NookMux/internal/domain/billing"
	"github.com/NookMux/NookMux/internal/httpapi"
	"github.com/gin-gonic/gin"
)

func ClaudeHelper(c *gin.Context, info *relaycommon.RelayInfo) (newAPIError *shared.NookMuxError) {

	info.InitChannelMeta(c)

	claudeReq, ok := info.Request.(*shared.ClaudeRequest)

	if !ok {
		return shared.NewErrorWithStatusCode(fmt.Errorf("invalid request type, expected *shared.ClaudeRequest, got %T", info.Request), shared.ErrorCodeInvalidRequest, http.StatusBadRequest, shared.ErrOptionWithSkipRetry())
	}

	request, err := common.DeepCopy(claudeReq)
	if err != nil {
		return shared.NewError(fmt.Errorf("failed to copy request to ClaudeRequest: %w", err), shared.ErrorCodeInvalidRequest, shared.ErrOptionWithSkipRetry())
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

	if request.MaxTokens == 0 {
		request.MaxTokens = uint(model.GetClaudeSettings().GetDefaultMaxTokens(request.Model))
	}

	var requestBody io.Reader
	if info.ChannelSetting.PassThroughBodyEnabled {
		storage, err := httpapi.GetBodyStorage(c)
		if err != nil {
			return shared.NewErrorWithStatusCode(err, shared.ErrorCodeReadRequestBodyFailed, http.StatusBadRequest, shared.ErrOptionWithSkipRetry())
		}
		jsonData, err := storage.Bytes()
		if err != nil {
			return shared.NewErrorWithStatusCode(err, shared.ErrorCodeReadRequestBodyFailed, http.StatusBadRequest, shared.ErrOptionWithSkipRetry())
		}
		jsonData, err = relaycommon.RemoveClaudeDisabledFields(jsonData, info.ChannelOtherSettings)
		if err != nil {
			return shared.NewError(err, shared.ErrorCodeConvertRequestFailed, shared.ErrOptionWithSkipRetry())
		}
		body, size, closer, err := relaycommon.NewOutboundJSONBody(jsonData)
		if err != nil {
			return shared.NewError(err, shared.ErrorCodeConvertRequestFailed, shared.ErrOptionWithSkipRetry())
		}
		defer closer.Close()
		info.UpstreamRequestBodySize = size
		requestBody = body
	} else {
		convertedRequest, err := adaptor.ConvertClaudeRequest(c, info, request)
		if err != nil {
			return shared.NewError(err, shared.ErrorCodeConvertRequestFailed, shared.ErrOptionWithSkipRetry())
		}
		relaycommon.AppendRequestConversionFromRequest(info, convertedRequest)
		jsonData, err := jsonx.Marshal(convertedRequest)
		if err != nil {
			return shared.NewError(err, shared.ErrorCodeConvertRequestFailed, shared.ErrOptionWithSkipRetry())
		}

		// remove disabled fields for Claude API
		jsonData, err = relaycommon.RemoveClaudeDisabledFields(jsonData, info.ChannelOtherSettings)
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

	statusCodeMappingStr := c.GetString("status_code_mapping")
	var httpResp *http.Response
	resp, err := adaptor.DoRequest(c, info, requestBody)
	if err != nil {
		return shared.NewOpenAIError(err, shared.ErrorCodeDoRequestFailed, http.StatusInternalServerError)
	}

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

	usage, newAPIError := adaptor.DoResponse(c, httpResp, info)
	//log.Printf("usage: %v", usage)
	if newAPIError != nil {
		// reset status code 重置状态码
		helper.ResetStatusCode(newAPIError, statusCodeMappingStr)
		return newAPIError
	}

	if apiErr := billing.PostClaudeConsumeQuota(c, info, usage.(*shared.Usage)); apiErr != nil {
		return apiErr
	}
	return nil
}
