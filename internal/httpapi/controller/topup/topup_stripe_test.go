package topupcontroller

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/NookMux/NookMux/internal/config"
	"github.com/gin-gonic/gin"
)

func TestStripeWebhookSecretGuard(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.POST("/api/stripe/webhook", StripeWebhook)

	doPost := func() *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodPost, "/api/stripe/webhook", strings.NewReader("{}"))
		recorder := httptest.NewRecorder()
		router.ServeHTTP(recorder, req)
		return recorder
	}

	t.Run("unconfigured secret is rejected with 403 before verification", func(t *testing.T) {
		// 空 secret 时 HMAC 验签可被伪造（空密钥签名可自行计算），必须在验签前拒绝。
		oldSecret := config.StripeWebhookSecret
		config.StripeWebhookSecret = ""
		t.Cleanup(func() { config.StripeWebhookSecret = oldSecret })

		recorder := doPost()
		if recorder.Code != http.StatusForbidden {
			t.Fatalf("status = %d, want %d, body = %s", recorder.Code, http.StatusForbidden, recorder.Body.String())
		}
	})

	t.Run("configured secret rejects unsigned payload with 400", func(t *testing.T) {
		oldSecret := config.StripeWebhookSecret
		config.StripeWebhookSecret = "whsec_test_dummy"
		t.Cleanup(func() { config.StripeWebhookSecret = oldSecret })

		recorder := doPost()
		if recorder.Code != http.StatusBadRequest {
			t.Fatalf("status = %d, want %d, body = %s", recorder.Code, http.StatusBadRequest, recorder.Body.String())
		}
	})
}
