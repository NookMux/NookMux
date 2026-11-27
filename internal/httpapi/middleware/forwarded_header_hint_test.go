package middleware

import (
	"bytes"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/NookMux/NookMux/internal/infra/security"
	"github.com/gin-gonic/gin"
)

// captureSystemError 把系统错误日志接到缓冲区，测试结束后恢复。
func captureSystemError(t *testing.T) *bytes.Buffer {
	t.Helper()

	buffer := &bytes.Buffer{}
	original := gin.DefaultErrorWriter
	gin.DefaultErrorWriter = buffer
	t.Cleanup(func() { gin.DefaultErrorWriter = original })
	return buffer
}

// newHintRouter 按生产装配的顺序挂上可信代理与转发头提示中间件。
func newHintRouter(t *testing.T) *gin.Engine {
	t.Helper()

	router := gin.New()
	router.Use(ForwardedHeaderIgnoredHint(security.SetupGinTrustedProxies(router)))
	router.GET("/", func(c *gin.Context) { c.Status(http.StatusNoContent) })
	return router
}

// requestThrough 以指定的直连来源与转发头发一次请求，返回状态码。
func requestThrough(router http.Handler, remoteAddr, header, value string) int {
	request := httptest.NewRequest(http.MethodGet, "/", nil)
	request.RemoteAddr = remoteAddr
	if header != "" {
		request.Header.Set(header, value)
	}
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	return response.Code
}

func TestForwardedHeaderIgnoredHintReportsUntrustedProxyOncePerAddress(t *testing.T) {
	gin.SetMode(gin.TestMode)
	t.Setenv("TRUSTED_PROXIES", "")
	logged := captureSystemError(t)
	router := newHintRouter(t)

	// 来源不在可信代理列表却带着转发头：提示一次，写明来源地址、请求头名称与处理办法，
	// 不回显请求头里的地址
	if code := requestThrough(router, "198.51.100.25:54321", "X-Forwarded-For", "203.0.113.7"); code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d", code, http.StatusNoContent)
	}
	first := logged.String()
	for _, want := range []string{"198.51.100.25", "X-Forwarded-For", "TRUSTED_PROXIES"} {
		if !strings.Contains(first, want) {
			t.Fatalf("hint log lacks %q: %s", want, first)
		}
	}
	if strings.Contains(first, "203.0.113.7") {
		t.Fatalf("hint log must not echo the header value: %s", first)
	}

	// 同一来源在间隔内不再重复提示
	logged.Reset()
	requestThrough(router, "198.51.100.25:54321", "X-Forwarded-For", "203.0.113.8")
	if logged.Len() != 0 {
		t.Fatalf("hint repeated for the same address within the interval: %s", logged.String())
	}

	// 其他转发头同样触发，提示按来源地址分别限频
	requestThrough(router, "203.0.113.9:54321", "CF-Connecting-IP", "203.0.113.8")
	if hint := logged.String(); !strings.Contains(hint, "CF-Connecting-IP") || !strings.Contains(hint, "203.0.113.9") {
		t.Fatalf("hint for a second address with another forwarded header missing: %q", hint)
	}

	// 来源在可信代理列表内是正常部署，不提示；不带转发头的请求也不提示
	logged.Reset()
	requestThrough(router, "172.18.0.2:54321", "X-Forwarded-For", "203.0.113.7")
	requestThrough(router, "203.0.113.10:54321", "", "")
	if logged.Len() != 0 {
		t.Fatalf("hint emitted for a trusted proxy or a request without forwarded headers: %s", logged.String())
	}
}

// 记录表达到条数上限后不再登记新的来源：这些来源每次请求都提示，内存占用保持有界。
func TestForwardedHeaderIgnoredHintBoundsRecordedAddresses(t *testing.T) {
	gin.SetMode(gin.TestMode)
	t.Setenv("TRUSTED_PROXIES", "")
	logged := captureSystemError(t)
	router := newHintRouter(t)

	address := func(i int) string {
		return fmt.Sprintf("198.18.%d.%d:54321", i/254+1, i%254+1)
	}
	sources := forwardedHeaderHintLimit + 1
	for i := 0; i < sources; i++ {
		requestThrough(router, address(i), "X-Forwarded-For", "203.0.113.7")
	}
	if lines := strings.Count(logged.String(), "\n"); lines != sources {
		t.Fatalf("hint lines for %d new addresses = %d, want %d", sources, lines, sources)
	}

	logged.Reset()
	for i := 0; i < sources; i++ {
		requestThrough(router, address(i), "X-Forwarded-For", "203.0.113.7")
	}
	if lines := strings.Count(logged.String(), "\n"); lines != 1 {
		t.Fatalf("hint lines on repeat requests = %d, want 1 (only the address beyond the limit)", lines)
	}
}

func TestForwardedHeaderIgnoredHintSkipsWhenTrustedProxiesDisabled(t *testing.T) {
	gin.SetMode(gin.TestMode)
	t.Setenv("TRUSTED_PROXIES", "none")
	logged := captureSystemError(t)
	router := newHintRouter(t)

	if code := requestThrough(router, "198.51.100.25:54321", "X-Forwarded-For", "203.0.113.7"); code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d", code, http.StatusNoContent)
	}
	if logged.Len() != 0 {
		t.Fatalf("hint emitted although trusted proxies are disabled explicitly: %s", logged.String())
	}
}
