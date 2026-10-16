package helper

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"net/http"
	"strconv"
	"strings"

	"github.com/NookMux/NookMux/internal/common"
	"github.com/NookMux/NookMux/internal/domain/shared"
	"github.com/NookMux/NookMux/internal/infra/log"

	"github.com/NookMux/NookMux/pkg/jsonx"
)

//// OpenAIErrorWrapper wraps an error into an OpenAIErrorWithStatusCode
//func OpenAIErrorWrapper(err error, code string, statusCode int) *shared.OpenAIErrorWithStatusCode {
//	text := err.Error()
//	lowerText := strings.ToLower(text)
//	if !strings.HasPrefix(lowerText, "get file base64 from url") && !strings.HasPrefix(lowerText, "mime type is not supported") {
//		if strings.Contains(lowerText, "post") || strings.Contains(lowerText, "dial") || strings.Contains(lowerText, "http") {
//			common.SysLog(fmt.Sprintf("error: %s", text))
//			text = "请求上游地址失败"
//		}
//	}
//	openAIError := shared.OpenAIError{
//		Message: text,
//		Type:    "nookmux_error",
//		Code:    code,
//	}
//	return &shared.OpenAIErrorWithStatusCode{
//		Error:      openAIError,
//		StatusCode: statusCode,
//	}
//}
//
//func OpenAIErrorWrapperLocal(err error, code string, statusCode int) *shared.OpenAIErrorWithStatusCode {
//	openaiErr := OpenAIErrorWrapper(err, code, statusCode)
//	openaiErr.LocalError = true
//	return openaiErr
//}

func ClaudeErrorWrapper(err error, code string, statusCode int) *shared.ClaudeErrorWithStatusCode {
	text := err.Error()
	lowerText := strings.ToLower(text)
	if !strings.HasPrefix(lowerText, "get file base64 from url") {
		if strings.Contains(lowerText, "post") || strings.Contains(lowerText, "dial") || strings.Contains(lowerText, "http") {
			common.SysLog(fmt.Sprintf("error: %s", text))
			text = "请求上游地址失败"
		}
	}
	claudeError := shared.ClaudeError{
		Message: text,
		Type:    string(shared.ErrorTypeNookMuxError),
	}
	return &shared.ClaudeErrorWithStatusCode{
		Error:      claudeError,
		StatusCode: statusCode,
	}
}

func ClaudeErrorWrapperLocal(err error, code string, statusCode int) *shared.ClaudeErrorWithStatusCode {
	claudeErr := ClaudeErrorWrapper(err, code, statusCode)
	claudeErr.LocalError = true
	return claudeErr
}

func RelayErrorHandler(ctx context.Context, resp *http.Response, showBodyWhenFail bool) (newApiErr *shared.NookMuxError) {
	newApiErr = shared.InitOpenAIError(shared.ErrorCodeBadResponseStatusCode, resp.StatusCode)
	defer CloseResponseBodyGracefully(resp)

	responseBody, err := ReadErrorResponseBody(resp.Body)
	if err != nil {
		newApiErr.Err = fmt.Errorf("failed to read upstream error response body: %w", err)
		return
	}
	var errResponse shared.GeneralErrorResponse
	responseBodyText := string(responseBody)
	responseBodyPreview := common.LocalLogPreview(responseBodyText)
	buildErrWithBody := func(message string) error {
		if message == "" {
			return fmt.Errorf("bad response status code %d, body: %s", resp.StatusCode, responseBodyText)
		}
		return fmt.Errorf("bad response status code %d, message: %s, body: %s", resp.StatusCode, message, responseBodyText)
	}

	err = jsonx.Unmarshal(responseBody, &errResponse)
	if err != nil {
		if showBodyWhenFail {
			newApiErr.Err = buildErrWithBody("")
		} else {
			log.LogError(ctx, fmt.Sprintf("bad response status code %d, body: %s", resp.StatusCode, responseBodyPreview))
			newApiErr.Err = fmt.Errorf("bad response status code %d", resp.StatusCode)
		}
		return
	}

	if jsonx.GetJsonType(errResponse.Error) == "object" {
		// General format error (OpenAI, Anthropic, Gemini, etc.)
		oaiError := errResponse.TryToOpenAIError()
		if oaiError != nil {
			newApiErr = shared.WithOpenAIError(*oaiError, resp.StatusCode)
			if showBodyWhenFail {
				newApiErr.Err = buildErrWithBody(newApiErr.Error())
			}
			return
		}
	}
	newApiErr = shared.NewOpenAIError(errors.New(errResponse.ToMessage()), shared.ErrorCodeBadResponseStatusCode, resp.StatusCode)
	if showBodyWhenFail {
		newApiErr.Err = buildErrWithBody(newApiErr.Error())
	}
	return
}

func ResetStatusCode(newApiErr *shared.NookMuxError, statusCodeMappingStr string) {
	if newApiErr == nil {
		return
	}
	if statusCodeMappingStr == "" || statusCodeMappingStr == "{}" {
		return
	}
	statusCodeMapping := make(map[string]any)
	err := jsonx.Unmarshal([]byte(statusCodeMappingStr), &statusCodeMapping)
	if err != nil {
		return
	}
	if newApiErr.StatusCode == http.StatusOK {
		return
	}
	codeStr := strconv.Itoa(newApiErr.StatusCode)
	if value, ok := statusCodeMapping[codeStr]; ok {
		intCode, ok := parseStatusCodeMappingValue(value)
		if !ok {
			return
		}
		if newApiErr.OriginalStatusCode == 0 {
			newApiErr.OriginalStatusCode = newApiErr.StatusCode
		}
		newApiErr.StatusCode = intCode
	}
}

// upstreamErrorStatusCodeByCode 覆盖 OpenAI 官方错误文档中「非数字 code +
// 固定 HTTP 状态」的组合（billing/quota 类 429、服务端 5xx）。网关把 429/5xx
// 转成 HTTP 200 下发时，这些 code 是还原真实状态码的唯一线索。
// 参考: https://developers.openai.com/api/docs/guides/error-codes
var upstreamErrorStatusCodeByCode = map[string]int{
	// 429 - billing / quota
	"insufficient_quota":                http.StatusTooManyRequests,
	"credit_balance_exhausted":          http.StatusTooManyRequests,
	"organization_spend_limit_exceeded": http.StatusTooManyRequests,
	"project_spend_limit_exceeded":      http.StatusTooManyRequests,
	"organization_usage_limit_exceeded": http.StatusTooManyRequests,
	"rate_limit_exceeded":               http.StatusTooManyRequests,
	// 5xx - server side
	"server_error":            http.StatusInternalServerError,
	"model_overloaded":        http.StatusServiceUnavailable,
	"engine_overloaded":       http.StatusServiceUnavailable,
	"context_length_exceeded": http.StatusBadRequest,
}

// UpstreamErrorStatusCode 处理「上游以 HTTP 200 携带错误载荷」的场景：
// 优先从错误对象的 code 字段还原真实 HTTP 状态码（如 429、503，兼容数字与
// 字符串形式），其次查官方非数字 code 映射（如 insufficient_quota → 429），
// 无法还原时回退为 502，保持与自动重试规则（5xx 可重试）兼容。
func UpstreamErrorStatusCode(httpStatusCode int, errorCode any) int {
	if httpStatusCode >= 400 {
		return httpStatusCode
	}
	switch v := errorCode.(type) {
	case int:
		if v >= 400 && v <= 599 {
			return v
		}
	case float64:
		if v >= 400 && v <= 599 {
			return int(v)
		}
	case string:
		trimmed := strings.TrimSpace(v)
		if n, err := strconv.Atoi(trimmed); err == nil && n >= 400 && n <= 599 {
			return n
		}
		if mapped, ok := upstreamErrorStatusCodeByCode[trimmed]; ok {
			return mapped
		}
	}
	return http.StatusBadGateway
}

func parseStatusCodeMappingValue(value any) (int, bool) {
	switch v := value.(type) {
	case string:
		if v == "" {
			return 0, false
		}
		statusCode, err := strconv.Atoi(v)
		if err != nil {
			return 0, false
		}
		return statusCode, true
	case float64:
		if v != math.Trunc(v) {
			return 0, false
		}
		return int(v), true
	case int:
		return v, true
	case json.Number:
		statusCode, err := strconv.Atoi(v.String())
		if err != nil {
			return 0, false
		}
		return statusCode, true
	default:
		return 0, false
	}
}
