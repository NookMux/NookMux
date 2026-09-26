package router

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/NookMux/NookMux/internal/common"
	"github.com/NookMux/NookMux/internal/domain/shared"
	"github.com/gin-gonic/gin"
)

func TestAnonymousPostRoutesRejectOversizedBodiesBeforeControllers(t *testing.T) {
	oldLimit := shared.AnonymousRequestBodyLimitKB
	oldGlobalRateLimit := common.GlobalApiRateLimitEnable
	shared.AnonymousRequestBodyLimitKB = 1
	common.GlobalApiRateLimitEnable = false
	t.Cleanup(func() {
		shared.AnonymousRequestBodyLimitKB = oldLimit
		common.GlobalApiRateLimitEnable = oldGlobalRateLimit
	})

	gin.SetMode(gin.TestMode)
	engine := gin.New()
	SetApiRouter(engine)

	routes := []string{
		"/api/setup",
		"/api/user/reset",
		"/api/stripe/webhook",
		"/api/user/register",
		"/api/user/login",
		"/api/user/login/2fa",
		"/api/user/passkey/login/begin",
		"/api/user/passkey/login/finish",
		"/api/user/epay/notify",
	}

	for _, route := range routes {
		t.Run(route, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, route, strings.NewReader(strings.Repeat("x", 1025)))
			recorder := httptest.NewRecorder()

			engine.ServeHTTP(recorder, req)

			if recorder.Code != http.StatusRequestEntityTooLarge {
				t.Fatalf("status = %d, want %d, body = %s", recorder.Code, http.StatusRequestEntityTooLarge, recorder.Body.String())
			}
		})
	}
}
