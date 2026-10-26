package topupcontroller

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/NookMux/NookMux/internal/config"
	"github.com/gin-gonic/gin"
	"github.com/stripe/stripe-go/v87/webhook"
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

func TestStripeWebhookAPIVersionGuard(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.POST("/api/stripe/webhook", StripeWebhook)

	oldSecret := config.StripeWebhookSecret
	config.StripeWebhookSecret = "whsec_test_dummy"
	t.Cleanup(func() { config.StripeWebhookSecret = oldSecret })

	newEvent := func(apiVersion string) []byte {
		return fmt.Appendf(nil, `{"id":"evt_v","object":"event","api_version":%q,"type":"unhandled.event","data":{"object":{"id":"cs_v","object":"checkout.session"}}}`, apiVersion)
	}

	// 签名有效仅代表密钥正确，事件还必须以 2026-09-30.endive 投递（端点创建时选择）。
	doPost := func(payload []byte) *httptest.ResponseRecorder {
		now := time.Now()
		signature := hex.EncodeToString(webhook.ComputeSignature(now, payload, config.StripeWebhookSecret))
		req := httptest.NewRequest(http.MethodPost, "/api/stripe/webhook", bytes.NewReader(payload))
		req.Header.Set("Stripe-Signature", fmt.Sprintf("t=%d,v1=%s", now.Unix(), signature))
		recorder := httptest.NewRecorder()
		router.ServeHTTP(recorder, req)
		return recorder
	}

	t.Run("endive event passes verification", func(t *testing.T) {
		recorder := doPost(newEvent("2026-09-30.endive"))
		if recorder.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d, body = %s", recorder.Code, http.StatusOK, recorder.Body.String())
		}
	})

	t.Run("mismatched version event is rejected with 400", func(t *testing.T) {
		recorder := doPost(newEvent("2026-08-26.dahlia"))
		if recorder.Code != http.StatusBadRequest {
			t.Fatalf("status = %d, want %d, body = %s", recorder.Code, http.StatusBadRequest, recorder.Body.String())
		}
	})
}
