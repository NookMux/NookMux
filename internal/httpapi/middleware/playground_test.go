package middleware

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/NookMux/NookMux/internal/common"
	"github.com/NookMux/NookMux/internal/config"

	"github.com/NookMux/NookMux/internal/httpapi"
	"github.com/NookMux/NookMux/internal/infra/redis"
	"github.com/gin-gonic/gin"
)

func TestPlaygroundRequestContextRejectsAccessTokenBeforeNext(t *testing.T) {
	gin.SetMode(gin.TestMode)

	called := false
	router := gin.New()
	router.POST("/pg/chat/completions",
		func(c *gin.Context) {
			c.Set("id", 7)
			c.Set("use_access_token", true)
			c.Next()
		},
		PlaygroundRequestContext(),
		func(c *gin.Context) {
			called = true
			c.JSON(http.StatusOK, gin.H{"success": true})
		},
	)

	req := httptest.NewRequest(http.MethodPost, "/pg/chat/completions", strings.NewReader(`{"model":"gpt-4o","group":"default","messages":[]}`))
	req.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()

	router.ServeHTTP(recorder, req)

	if called {
		t.Fatal("next handler was called for playground access token request")
	}
	if recorder.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d, body = %s", recorder.Code, http.StatusForbidden, recorder.Body.String())
	}
	if !strings.Contains(recorder.Body.String(), "暂不支持使用 access token") {
		t.Fatalf("expected access token rejection, got: %s", recorder.Body.String())
	}
}

func TestPlaygroundSelectedGroupAppliesBeforeModelRateLimit(t *testing.T) {
	gin.SetMode(gin.TestMode)

	oldRedisEnabled := redis.RedisEnabled
	oldRateLimitEnabled := config.ModelRequestRateLimitEnabled
	oldDuration := config.ModelRequestRateLimitDurationMinutes
	oldTotalCount := config.ModelRequestRateLimitCount
	oldSuccessCount := config.ModelRequestRateLimitSuccessCount
	config.ModelRequestRateLimitMutex.RLock()
	oldGroupLimits := config.ModelRequestRateLimitGroup
	config.ModelRequestRateLimitMutex.RUnlock()

	redis.RedisEnabled = false
	config.ModelRequestRateLimitEnabled = true
	config.ModelRequestRateLimitDurationMinutes = 1
	config.ModelRequestRateLimitCount = 0
	config.ModelRequestRateLimitSuccessCount = 1000
	config.ModelRequestRateLimitMutex.Lock()
	config.ModelRequestRateLimitGroup = map[string][2]int{
		"vip": {1, 1000},
	}
	config.ModelRequestRateLimitMutex.Unlock()
	// 复用同一 limiter 实例并清空其状态，而非整体重赋值：避免与仍在运行的
	// 清扫协程竞争同一结构体内存（整体赋值会写 expirationDuration 等字段）。
	inMemoryRateLimiter.Reset()

	t.Cleanup(func() {
		redis.RedisEnabled = oldRedisEnabled
		config.ModelRequestRateLimitEnabled = oldRateLimitEnabled
		config.ModelRequestRateLimitDurationMinutes = oldDuration
		config.ModelRequestRateLimitCount = oldTotalCount
		config.ModelRequestRateLimitSuccessCount = oldSuccessCount
		config.ModelRequestRateLimitMutex.Lock()
		config.ModelRequestRateLimitGroup = oldGroupLimits
		config.ModelRequestRateLimitMutex.Unlock()
		inMemoryRateLimiter.Reset()
	})

	router := gin.New()
	router.POST("/pg/chat/completions",
		func(c *gin.Context) {
			c.Set("id", 42)
			httpapi.SetContextKey(c, common.ContextKeyUserGroup, "default")
			httpapi.SetContextKey(c, common.ContextKeyUsingGroup, "default")
			c.Next()
		},
		PlaygroundRequestContext(),
		ModelRequestRateLimit(),
		func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{
				"group": httpapi.GetContextKeyString(c, common.ContextKeyTokenGroup),
			})
		},
	)

	first := httptest.NewRecorder()
	firstReq := httptest.NewRequest(http.MethodPost, "/pg/chat/completions", strings.NewReader(`{"model":"gpt-4o","group":"vip","messages":[]}`))
	firstReq.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(first, firstReq)

	if first.Code != http.StatusOK {
		t.Fatalf("first status = %d, want %d, body = %s", first.Code, http.StatusOK, first.Body.String())
	}
	if !strings.Contains(first.Body.String(), `"group":"vip"`) {
		t.Fatalf("selected playground group was not written before rate limit, body = %s", first.Body.String())
	}

	second := httptest.NewRecorder()
	secondReq := httptest.NewRequest(http.MethodPost, "/pg/chat/completions", strings.NewReader(`{"model":"gpt-4o","group":"vip","messages":[]}`))
	secondReq.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(second, secondReq)

	if second.Code != http.StatusTooManyRequests {
		t.Fatalf("second status = %d, want %d, body = %s", second.Code, http.StatusTooManyRequests, second.Body.String())
	}
}
