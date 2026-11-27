package router

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"testing/fstest"

	"github.com/NookMux/NookMux/internal/common"
	"github.com/NookMux/NookMux/internal/infra/redis"
	"github.com/gin-gonic/gin"
)

func TestAcceptsBrotli(t *testing.T) {
	tests := []struct {
		name           string
		acceptEncoding string
		want           bool
	}{
		{
			name:           "prefers brotli",
			acceptEncoding: "gzip, br",
			want:           true,
		},
		{
			name:           "skips disabled brotli",
			acceptEncoding: "br;q=0, gzip",
			want:           false,
		},
		{
			name:           "allows weighted brotli",
			acceptEncoding: "gzip, br;q=0.8",
			want:           true,
		},
		{
			name:           "skips zero decimal brotli",
			acceptEncoding: "gzip, br;q=0.0",
			want:           false,
		},
		{
			name:           "empty",
			acceptEncoding: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := acceptsBrotli(tt.acceptEncoding); got != tt.want {
				t.Fatalf("acceptsBrotli() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestStaticAssetCache(t *testing.T) {
	gin.SetMode(gin.TestMode)

	router := gin.New()
	router.Use(staticAssetCache())
	router.GET("/*path", func(c *gin.Context) {
		c.Status(http.StatusNoContent)
	})

	req := httptest.NewRequest(http.MethodGet, "/static/js/app.js", nil)
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if got := resp.Header().Get("Cache-Control"); got != "public, max-age=31536000, immutable" {
		t.Fatalf("Cache-Control = %q, want immutable static asset cache", got)
	}

	req = httptest.NewRequest(http.MethodGet, "/api/status", nil)
	resp = httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if got := resp.Header().Get("Cache-Control"); got != "" {
		t.Fatalf("API Cache-Control = %q, want empty", got)
	}
}

func TestPrecompressedStaticAssetsServesBrotli(t *testing.T) {
	gin.SetMode(gin.TestMode)

	compressed := []byte("pretend-brotli-compressed")

	buildFS := fstest.MapFS{
		"web/dist/static/js/app.js.br": {
			Data: compressed,
		},
	}

	router := gin.New()
	router.Use(staticAssetCache())
	router.Use(precompressedStaticAssetsFS(buildFS, "web/dist"))
	router.GET("/*path", func(c *gin.Context) {
		c.String(http.StatusOK, "fallback")
	})

	req := httptest.NewRequest(http.MethodGet, "/static/js/app.js", nil)
	req.Header.Set("Accept-Encoding", "br, gzip")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.Code)
	}
	if got := resp.Header().Get("Content-Encoding"); got != "br" {
		t.Fatalf("Content-Encoding = %q, want br", got)
	}
	if got := resp.Header().Get("Vary"); got != "Accept-Encoding" {
		t.Fatalf("Vary = %q, want Accept-Encoding", got)
	}
	if got := resp.Header().Get("Cache-Control"); got != "public, max-age=31536000, immutable" {
		t.Fatalf("Cache-Control = %q, want immutable static asset cache", got)
	}

	if got := resp.Body.Bytes(); !bytes.Equal(got, compressed) {
		t.Fatalf("body = %q, want %q", got, compressed)
	}
}

func TestPrecompressedStaticAssetsFallsThrough(t *testing.T) {
	gin.SetMode(gin.TestMode)

	router := gin.New()
	router.Use(precompressedStaticAssetsFS(fstest.MapFS{}, "web/dist"))
	router.GET("/*path", func(c *gin.Context) {
		c.String(http.StatusOK, "fallback")
	})

	req := httptest.NewRequest(http.MethodGet, "/static/js/missing.js", nil)
	req.Header.Set("Accept-Encoding", "gzip")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if got := resp.Body.String(); got != "fallback" {
		t.Fatalf("body = %q, want fallback", got)
	}
	if got := resp.Header().Get("Content-Encoding"); got != "" {
		t.Fatalf("Content-Encoding = %q, want empty", got)
	}
}

func TestSetWebRouterScopesWebRateLimitToFallbackChain(t *testing.T) {
	gin.SetMode(gin.TestMode)

	origRedisEnabled := redis.RedisEnabled
	origWebEnable, origWebNum, origWebDuration := common.GlobalWebRateLimitEnable, common.GlobalWebRateLimitNum, common.GlobalWebRateLimitDuration
	origStaticEnable := common.GlobalStaticRateLimitEnable
	redis.RedisEnabled = false
	common.GlobalWebRateLimitEnable, common.GlobalWebRateLimitNum, common.GlobalWebRateLimitDuration = true, 2, 90
	common.GlobalStaticRateLimitEnable = false
	t.Cleanup(func() {
		redis.RedisEnabled = origRedisEnabled
		common.GlobalWebRateLimitEnable, common.GlobalWebRateLimitNum, common.GlobalWebRateLimitDuration = origWebEnable, origWebNum, origWebDuration
		common.GlobalStaticRateLimitEnable = origStaticEnable
	})

	buildFS := fstest.MapFS{"dist/static/js/app.js": &fstest.MapFile{Data: []byte("app")}}
	indexPage := []byte("<!doctype html><title>spa</title>")
	router := gin.New()
	// API 路由先于 Web 静态路由注册
	router.GET("/api/ping", func(c *gin.Context) {
		c.String(http.StatusOK, "pong")
	})
	SetWebRouter(router, WebAssets{BuildFS: buildFS, IndexPage: indexPage})

	// 限流计数按 IP 隔离：404 隔离检查与页面额度检查各用一个 IP
	get := func(clientIP, target string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodGet, target, nil)
		req.RemoteAddr = clientIP + ":12345"
		resp := httptest.NewRecorder()
		router.ServeHTTP(resp, req)
		return resp
	}
	const clientIP = "192.0.2.20"
	page := func(target string) *httptest.ResponseRecorder {
		return get(clientIP, target)
	}

	// 未匹配的 /api 路径返回 JSON 404，不回退为前端页面
	if resp := get("192.0.2.21", "/api/unknown"); resp.Code != http.StatusNotFound {
		t.Fatalf("GET /api/unknown: code = %d, want %d", resp.Code, http.StatusNotFound)
	}

	// 页面额度：两次放行并返回 SPA 文档，第三次 429 并带 Retry-After
	for i := 0; i < 2; i++ {
		resp := page("/console/overview")
		if resp.Code != http.StatusOK || !bytes.Equal(resp.Body.Bytes(), indexPage) {
			t.Fatalf("GET /console/overview %d: code = %d, body = %q", i+1, resp.Code, resp.Body.String())
		}
	}
	resp := page("/console/overview")
	if resp.Code != http.StatusTooManyRequests {
		t.Fatalf("GET /console/overview 3: code = %d, want %d", resp.Code, http.StatusTooManyRequests)
	}
	if got, want := resp.Header().Get("Retry-After"), strconv.FormatInt(90, 10); got != want {
		t.Fatalf("Retry-After = %q, want %q", got, want)
	}

	// 页面额度耗尽后：构建产物与已注册的 API 路由都不受影响
	if resp := page("/static/js/app.js"); resp.Code != http.StatusOK || resp.Body.String() != "app" {
		t.Fatalf("GET /static/js/app.js: code = %d, body = %q", resp.Code, resp.Body.String())
	}
	if resp := page("/api/ping"); resp.Code != http.StatusOK || resp.Body.String() != "pong" {
		t.Fatalf("GET /api/ping: code = %d, body = %q", resp.Code, resp.Body.String())
	}

	// 未匹配的接口路径不占页面额度：页面额度用尽后仍按自己的额度放行两次，第三次 429
	for i := 0; i < 2; i++ {
		if resp := page("/api/unknown"); resp.Code != http.StatusNotFound {
			t.Fatalf("GET /api/unknown %d after pages exhausted: code = %d, want %d", i+1, resp.Code, http.StatusNotFound)
		}
	}
	if resp := page("/api/unknown"); resp.Code != http.StatusTooManyRequests {
		t.Fatalf("GET /api/unknown 3: code = %d, want %d", resp.Code, http.StatusTooManyRequests)
	}
	if got, want := resp.Header().Get("Retry-After"), strconv.FormatInt(90, 10); got != want {
		t.Fatalf("Retry-After = %q, want %q", got, want)
	}
}

func TestSetWebRouterReturnsNotFoundForNonPagePaths(t *testing.T) {
	gin.SetMode(gin.TestMode)

	origRedisEnabled := redis.RedisEnabled
	origWebEnable := common.GlobalWebRateLimitEnable
	redis.RedisEnabled = false
	common.GlobalWebRateLimitEnable = false
	t.Cleanup(func() {
		redis.RedisEnabled = origRedisEnabled
		common.GlobalWebRateLimitEnable = origWebEnable
	})

	buildFS := fstest.MapFS{"dist/static/js/app.js": &fstest.MapFile{Data: []byte("app")}}
	indexPage := []byte("<!doctype html><title>spa</title>")
	router := gin.New()
	SetWebRouter(router, WebAssets{BuildFS: buildFS, IndexPage: indexPage})

	get := func(target string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodGet, target, nil)
		resp := httptest.NewRecorder()
		router.ServeHTTP(resp, req)
		return resp
	}

	// 未匹配的接口、中继路径与不存在的构建产物返回不可缓存的 JSON 404，不回退为前端页面：
	// 发版后旧标签页请求已删除的分块属于这一类
	nonPage := []struct {
		target     string
		wantInBody string
	}{
		{"/api/unknown", "Invalid URL (GET /api/unknown)"},
		{"/v1/unknown", "Invalid URL (GET /v1/unknown)"},
		{"/assets/missing.js", "Invalid URL (GET /assets/missing.js)"},
		{"/static/js/async/0000.deadbeef.js", "Invalid URL (GET /static/js/async/0000.deadbeef.js)"},
	}
	for _, tt := range nonPage {
		resp := get(tt.target)
		if resp.Code != http.StatusNotFound {
			t.Fatalf("GET %s: code = %d, want %d", tt.target, resp.Code, http.StatusNotFound)
		}
		if body := resp.Body.String(); !bytes.Contains(resp.Body.Bytes(), []byte(tt.wantInBody)) {
			t.Fatalf("GET %s: body = %q, want JSON error mentioning %q", tt.target, body, tt.wantInBody)
		}
		if got := resp.Header().Get("Cache-Control"); got != "no-store" {
			t.Fatalf("GET %s: Cache-Control = %q, want no-store", tt.target, got)
		}
	}

	// 存在的构建产物照常返回并保持长期缓存
	resp := get("/static/js/app.js")
	if resp.Code != http.StatusOK || resp.Body.String() != "app" {
		t.Fatalf("GET /static/js/app.js: code = %d, body = %q", resp.Code, resp.Body.String())
	}
	if got := resp.Header().Get("Cache-Control"); got != "public, max-age=31536000, immutable" {
		t.Fatalf("GET /static/js/app.js: Cache-Control = %q, want immutable static asset cache", got)
	}

	// 页面路径仍返回 SPA 文档
	if resp := get("/console/overview"); resp.Code != http.StatusOK || !bytes.Equal(resp.Body.Bytes(), indexPage) {
		t.Fatalf("GET /console/overview: code = %d, body = %q", resp.Code, resp.Body.String())
	}
}
