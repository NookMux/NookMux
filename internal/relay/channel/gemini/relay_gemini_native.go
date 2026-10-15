package gemini

import (
	"fmt"
	"net/http"

	"github.com/NookMux/NookMux/internal/common"
	"github.com/NookMux/NookMux/internal/domain/shared"
	"github.com/NookMux/NookMux/internal/infra/log"
	relaycommon "github.com/NookMux/NookMux/internal/relay/common"
	"github.com/NookMux/NookMux/internal/relay/helper"

	"github.com/NookMux/NookMux/pkg/jsonx"

	billing "github.com/NookMux/NookMux/internal/domain/billing"
	"github.com/NookMux/NookMux/internal/httpapi"
	"github.com/gin-gonic/gin"
)

func GeminiTextGenerationHandler(c *gin.Context, info *relaycommon.RelayInfo, resp *http.Response) (*shared.Usage, *shared.NookMuxError) {
	defer helper.CloseResponseBodyGracefully(resp)

	// 读取响应体
	responseBody, err := helper.ReadResponseBody(resp.Body)
	if err != nil {
		return nil, shared.NewOpenAIError(err, shared.ErrorCodeBadResponseBody, http.StatusInternalServerError)
	}

	if common.DebugEnabled {
		println(string(responseBody))
	}

	// 解析为 Gemini 原生响应格式
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

	if len(geminiResponse.Candidates) == 0 && geminiResponse.PromptFeedback != nil && geminiResponse.PromptFeedback.BlockReason != nil {
		httpapi.SetContextKey(c, common.ContextKeyAdminRejectReason, fmt.Sprintf("gemini_block_reason=%s", *geminiResponse.PromptFeedback.BlockReason))
	}

	// 计算使用量（基于 UsageMetadata）
	usage := billing.GeminiUsageMetadataToOpenAIUsage(geminiResponse.UsageMetadata)

	helper.IOCopyBytesGracefully(c, resp, responseBody)

	return &usage, nil
}

func NativeGeminiEmbeddingHandler(c *gin.Context, resp *http.Response, info *relaycommon.RelayInfo) (*shared.Usage, *shared.NookMuxError) {
	defer helper.CloseResponseBodyGracefully(resp)

	responseBody, err := helper.ReadEmbeddingResponseBody(resp.Body)
	if err != nil {
		return nil, shared.NewOpenAIError(err, shared.ErrorCodeBadResponseBody, http.StatusInternalServerError)
	}

	if common.DebugEnabled {
		println(string(responseBody))
	}

	usage := billing.ResponseText2Usage(c, "", info.UpstreamModelName, info.GetEstimatePromptTokens())

	if info.IsGeminiBatchEmbedding {
		var geminiResponse shared.GeminiBatchEmbeddingResponse
		err = jsonx.Unmarshal(responseBody, &geminiResponse)
		if err != nil {
			return nil, shared.NewOpenAIError(err, shared.ErrorCodeBadResponseBody, http.StatusInternalServerError)
		}
	} else {
		var geminiResponse shared.GeminiEmbeddingResponse
		err = jsonx.Unmarshal(responseBody, &geminiResponse)
		if err != nil {
			return nil, shared.NewOpenAIError(err, shared.ErrorCodeBadResponseBody, http.StatusInternalServerError)
		}
	}

	helper.IOCopyBytesGracefully(c, resp, responseBody)

	return usage, nil
}

// NativeGeminiCountTokensHandler 校验并原样回写上游 countTokens 响应。计费沿用
// embedding 处理器的口径按输入 token 结算；若返回零用量会被计费层判定为上游
// 缺失计费信息并触发 502 重试，因此优先采用上游返回的精确 totalTokens。
func NativeGeminiCountTokensHandler(c *gin.Context, resp *http.Response, info *relaycommon.RelayInfo) (*shared.Usage, *shared.NookMuxError) {
	defer helper.CloseResponseBodyGracefully(resp)

	responseBody, err := helper.ReadResponseBody(resp.Body)
	if err != nil {
		return nil, shared.NewOpenAIError(err, shared.ErrorCodeBadResponseBody, http.StatusInternalServerError)
	}

	if common.DebugEnabled {
		println(string(responseBody))
	}

	usage := billing.ResponseText2Usage(c, "", info.UpstreamModelName, info.GetEstimatePromptTokens())

	var geminiResponse shared.GeminiCountTokensResponse
	if err := jsonx.Unmarshal(responseBody, &geminiResponse); err != nil {
		return nil, shared.NewOpenAIError(err, shared.ErrorCodeBadResponseBody, http.StatusInternalServerError)
	}
	if geminiResponse.TotalTokens > 0 {
		usage.PromptTokens = geminiResponse.TotalTokens
		usage.TotalTokens = geminiResponse.TotalTokens
	}

	helper.IOCopyBytesGracefully(c, resp, responseBody)

	return usage, nil
}

func GeminiTextGenerationStreamHandler(c *gin.Context, info *relaycommon.RelayInfo, resp *http.Response) (*shared.Usage, *shared.NookMuxError) {
	helper.SetEventStreamHeaders(c)

	return geminiStreamHandler(c, info, resp, func(data string, geminiResponse *shared.GeminiChatResponse) bool {
		err := helper.StringData(c, data)
		if err != nil {
			log.LogError(c, "failed to write stream data: "+err.Error())
			return false
		}
		info.SendResponseCount++
		return true
	})
}
