package router

import (
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/NookMux/NookMux/internal/common"
	"github.com/NookMux/NookMux/internal/infra/redis"
	"github.com/gin-gonic/gin"
)

// 凭 API Key 匿名查询用量与日志的接口由独立的限流器计数：查询额度用尽后登录等关键操作
// 不受影响，关键操作额度用尽后密钥查询也不受影响。
func TestApiKeyQueryRoutesUseOwnRateLimitBucket(t *testing.T) {
	gin.SetMode(gin.TestMode)

	origRedisEnabled := redis.RedisEnabled
	origGlobalAPIEnable := common.GlobalApiRateLimitEnable
	origKeyQueryEnable, origKeyQueryNum, origKeyQueryDuration := common.ApiKeyQueryRateLimitEnable, common.ApiKeyQueryRateLimitNum, common.ApiKeyQueryRateLimitDuration
	origCriticalEnable, origCriticalNum := common.CriticalRateLimitEnable, common.CriticalRateLimitNum
	redis.RedisEnabled = false
	common.GlobalApiRateLimitEnable = false
	common.ApiKeyQueryRateLimitEnable, common.ApiKeyQueryRateLimitNum, common.ApiKeyQueryRateLimitDuration = true, 3, 47
	common.CriticalRateLimitEnable, common.CriticalRateLimitNum = true, 1
	t.Cleanup(func() {
		redis.RedisEnabled = origRedisEnabled
		common.GlobalApiRateLimitEnable = origGlobalAPIEnable
		common.ApiKeyQueryRateLimitEnable, common.ApiKeyQueryRateLimitNum, common.ApiKeyQueryRateLimitDuration = origKeyQueryEnable, origKeyQueryNum, origKeyQueryDuration
		common.CriticalRateLimitEnable, common.CriticalRateLimitNum = origCriticalEnable, origCriticalNum
	})

	router := gin.New()
	SetApiRouter(router)

	const clientIP = "192.0.2.31"
	do := func(method, target, body string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(method, target, strings.NewReader(body))
		req.RemoteAddr = clientIP + ":12345"
		resp := httptest.NewRecorder()
		router.ServeHTTP(resp, req)
		return resp
	}

	// 不带密钥的查询在鉴权处即返回 401，不访问数据库；每次请求各占一次密钥查询额度
	keyQueryTargets := []string{"/api/usage/token/", "/api/log/token", "/api/log/token/usage_log_fields"}
	for _, target := range keyQueryTargets {
		if resp := do(http.MethodGet, target, ""); resp.Code != http.StatusUnauthorized {
			t.Fatalf("GET %s: code = %d, want %d", target, resp.Code, http.StatusUnauthorized)
		}
	}
	resp := do(http.MethodGet, "/api/log/token", "")
	if resp.Code != http.StatusTooManyRequests {
		t.Fatalf("GET /api/log/token after key query quota exhausted: code = %d, want %d", resp.Code, http.StatusTooManyRequests)
	}
	if got, want := resp.Header().Get("Retry-After"), strconv.FormatInt(47, 10); got != want {
		t.Fatalf("Retry-After = %q, want %q", got, want)
	}

	// 登录请求的凭证为空，在参数校验处返回，不访问数据库。密钥查询用完额度后，
	// 同一 IP 仅剩 1 次的关键操作额度没有被占用，登录请求到达控制器而不是 429
	const emptyCredentials = `{"username":"","password":""}`
	resp = do(http.MethodPost, "/api/user/login", emptyCredentials)
	if resp.Code == http.StatusTooManyRequests || !strings.Contains(resp.Body.String(), `"success":false`) {
		t.Fatalf("POST /api/user/login right after key query quota exhausted: code = %d, body = %s; want the request to reach the controller", resp.Code, resp.Body.String())
	}
	if resp := do(http.MethodPost, "/api/user/login", emptyCredentials); resp.Code != http.StatusTooManyRequests {
		t.Fatalf("POST /api/user/login after critical quota exhausted: code = %d, want %d", resp.Code, http.StatusTooManyRequests)
	}
}
