package router

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/NookMux/NookMux/internal/common"
	"github.com/gin-contrib/sessions"
	"github.com/gin-contrib/sessions/cookie"
	"github.com/gin-gonic/gin"
)

// /api/oauth/email/bind is mounted behind UserAuth: anonymous requests must be
// rejected with 401 by the middleware and never reach the controller.
func TestOAuthEmailBindRouteRejectsAnonymousRequests(t *testing.T) {
	oldGlobalRateLimit := common.GlobalApiRateLimitEnable
	common.GlobalApiRateLimitEnable = false
	t.Cleanup(func() {
		common.GlobalApiRateLimitEnable = oldGlobalRateLimit
	})

	gin.SetMode(gin.TestMode)
	engine := gin.New()
	// 与 internal/app/server.go 一致：sessions 中间件在路由挂载前注册
	engine.Use(sessions.Sessions("session", cookie.NewStore([]byte("test-secret"))))
	SetApiRouter(engine)

	req := httptest.NewRequest(http.MethodPost, "/api/oauth/email/bind",
		strings.NewReader(`{"email":"attacker@example.com","code":"123456"}`))
	req.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()

	engine.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, body = %s, want 401", recorder.Code, recorder.Body.String())
	}
}
