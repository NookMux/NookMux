package middleware

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/NookMux/NookMux/internal/common"
	"github.com/NookMux/NookMux/internal/infra/redis"
	"github.com/gin-contrib/static"
	"github.com/gin-gonic/gin"
)

func TestSearchRateLimitDisabled(t *testing.T) {
	gin.SetMode(gin.TestMode)

	origEnabled := common.SearchRateLimitEnable
	common.SearchRateLimitEnable = false
	t.Cleanup(func() { common.SearchRateLimitEnable = origEnabled })

	router := gin.New()
	router.GET("/search", func(c *gin.Context) {
		c.Set("id", 42)
		c.Next()
	}, SearchRateLimit(), func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	recorder := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/search", nil)
	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected 200 when search rate limit disabled, got %d", recorder.Code)
	}
}

func TestSearchRateLimitEnabled(t *testing.T) {
	gin.SetMode(gin.TestMode)

	origEnabled := common.SearchRateLimitEnable
	origNum := common.SearchRateLimitNum
	origDuration := common.SearchRateLimitDuration

	common.SearchRateLimitEnable = true
	common.SearchRateLimitNum = 2
	common.SearchRateLimitDuration = 3600

	redis.RedisEnabled = false
	inMemoryRateLimiter.Init(common.RateLimitKeyExpirationDuration)

	t.Cleanup(func() {
		common.SearchRateLimitEnable = origEnabled
		common.SearchRateLimitNum = origNum
		common.SearchRateLimitDuration = origDuration
	})

	router := gin.New()
	router.GET("/search", func(c *gin.Context) {
		c.Set("id", 999001)
		c.Next()
	}, SearchRateLimit(), func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	// First two requests within the limit should succeed.
	for i := 0; i < 2; i++ {
		recorder := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/search", nil)
		router.ServeHTTP(recorder, req)
		if recorder.Code != http.StatusOK {
			t.Fatalf("expected 200 for request %d, got %d", i+1, recorder.Code)
		}
	}

	// The third request within the same window should be rate limited.
	recorder := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/search", nil)
	router.ServeHTTP(recorder, req)
	if recorder.Code != http.StatusTooManyRequests {
		t.Fatalf("expected 429 after limit exceeded, got %d", recorder.Code)
	}
}

func TestSearchRateLimitHonorsConfiguredDuration(t *testing.T) {
	gin.SetMode(gin.TestMode)

	origEnabled := common.SearchRateLimitEnable
	origNum := common.SearchRateLimitNum
	origDuration := common.SearchRateLimitDuration

	common.SearchRateLimitEnable = true
	common.SearchRateLimitNum = 1
	common.SearchRateLimitDuration = 1

	redis.RedisEnabled = false
	inMemoryRateLimiter.Init(common.RateLimitKeyExpirationDuration)

	t.Cleanup(func() {
		common.SearchRateLimitEnable = origEnabled
		common.SearchRateLimitNum = origNum
		common.SearchRateLimitDuration = origDuration
	})

	router := gin.New()
	router.GET("/search", func(c *gin.Context) {
		c.Set("id", 999002)
		c.Next()
	}, SearchRateLimit(), func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	recorder := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/search", nil)
	router.ServeHTTP(recorder, req)
	if recorder.Code != http.StatusOK {
		t.Fatalf("expected first request 200, got %d", recorder.Code)
	}

	recorder = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/search", nil)
	router.ServeHTTP(recorder, req)
	if recorder.Code != http.StatusTooManyRequests {
		t.Fatalf("expected 429 while still within window, got %d", recorder.Code)
	}
}

func TestSearchRateLimitUnauthenticated(t *testing.T) {
	gin.SetMode(gin.TestMode)

	origEnabled := common.SearchRateLimitEnable
	common.SearchRateLimitEnable = true
	t.Cleanup(func() { common.SearchRateLimitEnable = origEnabled })

	router := gin.New()
	router.GET("/search", SearchRateLimit(), func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	recorder := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/search", nil)
	router.ServeHTTP(recorder, req)

	// Without a user id the limiter is not in a valid state to decide; it
	// rejects the request as 401 to signal that UserAuth is missing upstream.
	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 when user id is missing, got %d", recorder.Code)
	}
}

// newFrontendFileServer 在临时目录放置一个构建产物文件，模拟前端静态文件系统。
func newFrontendFileServer(t *testing.T) static.ServeFileSystem {
	t.Helper()

	dir := t.TempDir()
	file := filepath.Join(dir, "static", "js", "app.js")
	if err := os.MkdirAll(filepath.Dir(file), 0o755); err != nil {
		t.Fatalf("create frontend dir: %v", err)
	}
	if err := os.WriteFile(file, []byte("app"), 0o644); err != nil {
		t.Fatalf("write frontend file: %v", err)
	}
	return static.LocalFile(dir, false)
}

// setWebRateLimitQuota 覆盖 Web / 静态限流的开关与额度，不改存储模式，测试结束后恢复。
func setWebRateLimitQuota(t *testing.T, webEnable bool, webNum int, staticEnable bool, staticNum int, duration int64) {
	t.Helper()

	origWebEnable, origWebNum, origWebDuration := common.GlobalWebRateLimitEnable, common.GlobalWebRateLimitNum, common.GlobalWebRateLimitDuration
	origStaticEnable, origStaticNum, origStaticDuration := common.GlobalStaticRateLimitEnable, common.GlobalStaticRateLimitNum, common.GlobalStaticRateLimitDuration

	common.GlobalWebRateLimitEnable, common.GlobalWebRateLimitNum, common.GlobalWebRateLimitDuration = webEnable, webNum, duration
	common.GlobalStaticRateLimitEnable, common.GlobalStaticRateLimitNum, common.GlobalStaticRateLimitDuration = staticEnable, staticNum, duration

	t.Cleanup(func() {
		common.GlobalWebRateLimitEnable, common.GlobalWebRateLimitNum, common.GlobalWebRateLimitDuration = origWebEnable, origWebNum, origWebDuration
		common.GlobalStaticRateLimitEnable, common.GlobalStaticRateLimitNum, common.GlobalStaticRateLimitDuration = origStaticEnable, origStaticNum, origStaticDuration
	})
}

// setWebRateLimitForTest 覆盖 Web / 静态限流配置并强制走内存限流，测试结束后恢复。
func setWebRateLimitForTest(t *testing.T, webEnable bool, webNum int, staticEnable bool, staticNum int, duration int64) {
	t.Helper()

	origRedisEnabled := redis.RedisEnabled
	redis.RedisEnabled = false
	inMemoryRateLimiter.Init(common.RateLimitKeyExpirationDuration)
	t.Cleanup(func() { redis.RedisEnabled = origRedisEnabled })

	setWebRateLimitQuota(t, webEnable, webNum, staticEnable, staticNum, duration)
}

// serveFromClient 以固定客户端 IP 发起请求。限流计数按 IP 隔离，各用例使用不同 IP。
func serveFromClient(router *gin.Engine, clientIP string, target string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet, target, nil)
	req.RemoteAddr = clientIP + ":12345"
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	return resp
}

func TestGlobalWebRateLimitSeparatesFrontendFilesFromPages(t *testing.T) {
	gin.SetMode(gin.TestMode)
	// 静态限流保持默认关闭
	setWebRateLimitForTest(t, true, 2, false, 1000, 41)

	router := gin.New()
	router.NoRoute(GlobalWebRateLimit(newFrontendFileServer(t)), func(c *gin.Context) {
		c.Status(http.StatusNoContent)
	})
	const clientIP = "192.0.2.10"

	// 构建产物不占用页面额度：请求数远超页面额度仍全部放行
	for i := 0; i < 5; i++ {
		if resp := serveFromClient(router, clientIP, "/static/js/app.js"); resp.Code != http.StatusNoContent {
			t.Fatalf("frontend file request %d: code = %d, want %d", i+1, resp.Code, http.StatusNoContent)
		}
	}

	// 页面额度独立计数，超出后 429 并带 Retry-After（整个窗口时长）
	for i := 0; i < 2; i++ {
		if resp := serveFromClient(router, clientIP, "/console/overview"); resp.Code != http.StatusNoContent {
			t.Fatalf("page request %d: code = %d, want %d", i+1, resp.Code, http.StatusNoContent)
		}
	}
	resp := serveFromClient(router, clientIP, "/console/overview")
	if resp.Code != http.StatusTooManyRequests {
		t.Fatalf("page request 3: code = %d, want %d", resp.Code, http.StatusTooManyRequests)
	}
	if got, want := resp.Header().Get("Retry-After"), strconv.FormatInt(41, 10); got != want {
		t.Fatalf("Retry-After = %q, want %q", got, want)
	}

	// 页面额度耗尽后构建产物仍可访问；不存在的产物路径不占页面额度，照常进入回退链
	if resp := serveFromClient(router, clientIP, "/static/js/app.js"); resp.Code != http.StatusNoContent {
		t.Fatalf("frontend file after pages exhausted: code = %d, want %d", resp.Code, http.StatusNoContent)
	}
	if resp := serveFromClient(router, clientIP, "/static/js/missing.js"); resp.Code != http.StatusNoContent {
		t.Fatalf("missing frontend file after pages exhausted: code = %d, want %d", resp.Code, http.StatusNoContent)
	}
}

func TestGlobalWebRateLimitCountsUnmatchedPathsSeparatelyFromPages(t *testing.T) {
	gin.SetMode(gin.TestMode)
	setWebRateLimitForTest(t, true, 2, false, 1000, 41)

	router := gin.New()
	router.NoRoute(GlobalWebRateLimit(newFrontendFileServer(t)), func(c *gin.Context) {
		c.Status(http.StatusNoContent)
	})
	const clientIP = "192.0.2.15"

	// 未匹配路径共用一份额度：接口、中继与不存在的构建产物路径一起计数，超出后 429
	for _, target := range []string{"/v1/unknown", "/api/unknown"} {
		if resp := serveFromClient(router, clientIP, target); resp.Code != http.StatusNoContent {
			t.Fatalf("unmatched request %s: code = %d, want %d", target, resp.Code, http.StatusNoContent)
		}
	}
	resp := serveFromClient(router, clientIP, "/static/js/missing.js")
	if resp.Code != http.StatusTooManyRequests {
		t.Fatalf("unmatched request beyond quota: code = %d, want %d", resp.Code, http.StatusTooManyRequests)
	}
	if got, want := resp.Header().Get("Retry-After"), strconv.FormatInt(41, 10); got != want {
		t.Fatalf("Retry-After = %q, want %q", got, want)
	}

	// 未匹配路径的额度用尽不影响页面：页面按自己的额度放行两次，第三次才 429
	for i := 0; i < 2; i++ {
		if resp := serveFromClient(router, clientIP, "/console/overview"); resp.Code != http.StatusNoContent {
			t.Fatalf("page request %d after unmatched quota exhausted: code = %d, want %d", i+1, resp.Code, http.StatusNoContent)
		}
	}
	if resp := serveFromClient(router, clientIP, "/console/overview"); resp.Code != http.StatusTooManyRequests {
		t.Fatalf("page request 3: code = %d, want %d", resp.Code, http.StatusTooManyRequests)
	}
}

func TestIsNonPageRequest(t *testing.T) {
	cases := map[string]bool{
		"/v1/chat/completions":  true,
		"/v1beta/models":        true,
		"/api/status":           true,
		"/api/status?page=1":    true,
		"/static/js/async/x.js": true,
		"/assets/logo.png":      true,
		"/":                     false,
		"/console/overview":     false,
		"/pricing/gpt-4":        false,
	}
	for requestURI, want := range cases {
		if got := IsNonPageRequest(requestURI); got != want {
			t.Fatalf("IsNonPageRequest(%q) = %v, want %v", requestURI, got, want)
		}
	}
}

func TestGlobalWebRateLimitStaticBucketEnabled(t *testing.T) {
	gin.SetMode(gin.TestMode)
	setWebRateLimitForTest(t, true, 2, true, 3, 43)

	router := gin.New()
	router.NoRoute(GlobalWebRateLimit(newFrontendFileServer(t)), func(c *gin.Context) {
		c.Status(http.StatusNoContent)
	})
	const clientIP = "192.0.2.11"

	// 静态限流开启后构建产物按自己的额度计数
	for i := 0; i < 3; i++ {
		if resp := serveFromClient(router, clientIP, "/static/js/app.js"); resp.Code != http.StatusNoContent {
			t.Fatalf("frontend file request %d: code = %d, want %d", i+1, resp.Code, http.StatusNoContent)
		}
	}
	resp := serveFromClient(router, clientIP, "/static/js/app.js")
	if resp.Code != http.StatusTooManyRequests {
		t.Fatalf("frontend file request 4: code = %d, want %d", resp.Code, http.StatusTooManyRequests)
	}
	if got, want := resp.Header().Get("Retry-After"), strconv.FormatInt(43, 10); got != want {
		t.Fatalf("Retry-After = %q, want %q", got, want)
	}

	// 静态额度耗尽不影响页面额度
	if resp := serveFromClient(router, clientIP, "/console/overview"); resp.Code != http.StatusNoContent {
		t.Fatalf("page after frontend files exhausted: code = %d, want %d", resp.Code, http.StatusNoContent)
	}
}

func TestGlobalWebRateLimitDisabledSkipsBothBuckets(t *testing.T) {
	gin.SetMode(gin.TestMode)
	setWebRateLimitForTest(t, false, 2, false, 1, 41)

	router := gin.New()
	router.NoRoute(GlobalWebRateLimit(newFrontendFileServer(t)), func(c *gin.Context) {
		c.Status(http.StatusNoContent)
	})
	const clientIP = "192.0.2.12"

	for i := 0; i < 5; i++ {
		for _, target := range []string{"/static/js/app.js", "/console/overview"} {
			if resp := serveFromClient(router, clientIP, target); resp.Code != http.StatusNoContent {
				t.Fatalf("%s request %d with web limit disabled: code = %d, want %d", target, i+1, resp.Code, http.StatusNoContent)
			}
		}
	}
}

func TestSearchRateLimitRetryAfter(t *testing.T) {
	gin.SetMode(gin.TestMode)

	origEnabled := common.SearchRateLimitEnable
	origNum := common.SearchRateLimitNum
	origDuration := common.SearchRateLimitDuration

	common.SearchRateLimitEnable = true
	common.SearchRateLimitNum = 1
	common.SearchRateLimitDuration = 97

	redis.RedisEnabled = false
	inMemoryRateLimiter.Init(common.RateLimitKeyExpirationDuration)

	t.Cleanup(func() {
		common.SearchRateLimitEnable = origEnabled
		common.SearchRateLimitNum = origNum
		common.SearchRateLimitDuration = origDuration
	})

	router := gin.New()
	router.GET("/search", func(c *gin.Context) {
		c.Set("id", 999003)
		c.Next()
	}, SearchRateLimit(), func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	first := httptest.NewRecorder()
	router.ServeHTTP(first, httptest.NewRequest(http.MethodGet, "/search", nil))
	if first.Code != http.StatusOK {
		t.Fatalf("expected 200 for first request, got %d", first.Code)
	}

	// 用户级限流被拒时同样带 Retry-After（整个窗口时长）
	second := httptest.NewRecorder()
	router.ServeHTTP(second, httptest.NewRequest(http.MethodGet, "/search", nil))
	if second.Code != http.StatusTooManyRequests {
		t.Fatalf("expected 429 after limit exceeded, got %d", second.Code)
	}
	if got, want := second.Header().Get("Retry-After"), strconv.FormatInt(97, 10); got != want {
		t.Fatalf("Retry-After = %q, want %q", got, want)
	}
}

func TestApiKeyQueryRateLimitCountsSeparatelyFromCriticalOperations(t *testing.T) {
	gin.SetMode(gin.TestMode)

	origRedisEnabled := redis.RedisEnabled
	origKeyQueryEnable, origKeyQueryNum, origKeyQueryDuration := common.ApiKeyQueryRateLimitEnable, common.ApiKeyQueryRateLimitNum, common.ApiKeyQueryRateLimitDuration
	origCriticalEnable, origCriticalNum := common.CriticalRateLimitEnable, common.CriticalRateLimitNum
	redis.RedisEnabled = false
	common.ApiKeyQueryRateLimitEnable, common.ApiKeyQueryRateLimitNum, common.ApiKeyQueryRateLimitDuration = true, 2, 53
	common.CriticalRateLimitEnable, common.CriticalRateLimitNum = true, 1
	inMemoryRateLimiter.Init(common.RateLimitKeyExpirationDuration)
	t.Cleanup(func() {
		redis.RedisEnabled = origRedisEnabled
		common.ApiKeyQueryRateLimitEnable, common.ApiKeyQueryRateLimitNum, common.ApiKeyQueryRateLimitDuration = origKeyQueryEnable, origKeyQueryNum, origKeyQueryDuration
		common.CriticalRateLimitEnable, common.CriticalRateLimitNum = origCriticalEnable, origCriticalNum
	})

	router := gin.New()
	router.GET("/usage/token/", ApiKeyQueryRateLimit(), func(c *gin.Context) {
		c.Status(http.StatusNoContent)
	})
	router.GET("/user/login", CriticalRateLimit(), func(c *gin.Context) {
		c.Status(http.StatusNoContent)
	})
	const clientIP = "192.0.2.13"

	for i := 0; i < 2; i++ {
		if resp := serveFromClient(router, clientIP, "/usage/token/"); resp.Code != http.StatusNoContent {
			t.Fatalf("key query request %d: code = %d, want %d", i+1, resp.Code, http.StatusNoContent)
		}
	}
	resp := serveFromClient(router, clientIP, "/usage/token/")
	if resp.Code != http.StatusTooManyRequests {
		t.Fatalf("key query request 3: code = %d, want %d", resp.Code, http.StatusTooManyRequests)
	}
	if got, want := resp.Header().Get("Retry-After"), strconv.FormatInt(53, 10); got != want {
		t.Fatalf("Retry-After = %q, want %q", got, want)
	}

	// 密钥查询额度用尽后，同一 IP 的关键操作额度没有被占用，登录照常放行；
	// 关键操作自身的额度仍是 1 次，第二次登录才被拒
	if resp := serveFromClient(router, clientIP, "/user/login"); resp.Code != http.StatusNoContent {
		t.Fatalf("login after key queries exhausted: code = %d, want %d", resp.Code, http.StatusNoContent)
	}
	if resp := serveFromClient(router, clientIP, "/user/login"); resp.Code != http.StatusTooManyRequests {
		t.Fatalf("second login: code = %d, want %d", resp.Code, http.StatusTooManyRequests)
	}
}

func TestApiKeyQueryRateLimitDisabled(t *testing.T) {
	gin.SetMode(gin.TestMode)

	origEnabled := common.ApiKeyQueryRateLimitEnable
	common.ApiKeyQueryRateLimitEnable = false
	t.Cleanup(func() { common.ApiKeyQueryRateLimitEnable = origEnabled })

	router := gin.New()
	router.GET("/usage/token/", ApiKeyQueryRateLimit(), func(c *gin.Context) {
		c.Status(http.StatusOK)
	})
	const clientIP = "192.0.2.14"

	for i := 0; i < 5; i++ {
		if resp := serveFromClient(router, clientIP, "/usage/token/"); resp.Code != http.StatusOK {
			t.Fatalf("expected 200 with key query rate limit disabled, request %d got %d", i+1, resp.Code)
		}
	}
}
