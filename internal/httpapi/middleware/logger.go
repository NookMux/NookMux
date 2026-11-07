package middleware

import (
	"fmt"
	"sort"
	"strings"

	"github.com/NookMux/NookMux/internal/common"
	"github.com/gin-gonic/gin"
)

// maskLogPath 对访问日志中的请求路径做凭据脱敏：OAuth 回调路径的
// 授权码/state 整段截断；其余查询串保留键名、遮蔽值（如 ?key=sk-…
// 落盘为 key=***），仅影响日志输出，不影响处理器读取原始查询参数。
func maskLogPath(param gin.LogFormatterParams) string {
	path := param.Path
	var raw string
	if param.Request != nil && param.Request.URL != nil {
		raw = param.Request.URL.RawQuery
	}
	if raw == "" || !strings.Contains(path, "?") {
		return path
	}
	if strings.HasPrefix(path, "/api/oauth/") || strings.HasPrefix(path, "/oauth/") {
		path, _, _ = strings.Cut(path, "?")
		return path
	}
	q := param.Request.URL.Query()
	keys := make([]string, 0, len(q))
	for k := range q {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, k := range keys {
		parts = append(parts, k+"=***")
	}
	masked := strings.Join(parts, "&")
	base, _, _ := strings.Cut(path, "?")
	if masked == "" {
		return base
	}
	return base + "?" + masked
}

func SetUpLogger(server *gin.Engine) {
	server.Use(gin.LoggerWithFormatter(func(param gin.LogFormatterParams) string {
		var requestID string
		if param.Keys != nil {
			requestID = param.Keys[common.RequestIdKey].(string)
		}
		return fmt.Sprintf("[GIN] %s | %s | %3d | %13v | %15s | %7s %s\n",
			param.TimeStamp.Format("2006/01/02 - 15:04:05"),
			requestID,
			param.StatusCode,
			param.Latency,
			param.ClientIP,
			param.Method,
			maskLogPath(param),
		)
	}))
}
