package billingcontroller

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"

	"github.com/NookMux/NookMux/internal/config/ratio"
	"github.com/NookMux/NookMux/internal/i18n"
	"github.com/gin-gonic/gin"
)

func performBuiltinModelPricingRequest(t *testing.T, target, lang string) *httptest.ResponseRecorder {
	t.Helper()

	gin.SetMode(gin.TestMode)
	if err := i18n.Init(); err != nil {
		t.Fatalf("i18n.Init failed: %v", err)
	}
	resp := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(resp)
	c.Request = httptest.NewRequest(http.MethodGet, target, nil)
	c.Request.Header.Set("Accept-Language", lang)
	GetBuiltinModelPricing(c)
	return resp
}

// 模型名为空时返回本地化的业务错误，且不携带数据。
func TestGetBuiltinModelPricingRequiresModelName(t *testing.T) {
	for lang, want := range map[string]string{
		"zh": "模型名称不能为空",
		"en": "Model name cannot be empty",
	} {
		resp := performBuiltinModelPricingRequest(t, "/api/option/builtin_model_pricing?model=%20%20", lang)
		if resp.Code != http.StatusOK {
			t.Fatalf("lang=%s status = %d, want %d", lang, resp.Code, http.StatusOK)
		}
		var body struct {
			Success bool            `json:"success"`
			Message string          `json:"message"`
			Data    json.RawMessage `json:"data"`
		}
		if err := json.Unmarshal(resp.Body.Bytes(), &body); err != nil {
			t.Fatalf("lang=%s unmarshal response: %v", lang, err)
		}
		if body.Success || body.Message != want {
			t.Fatalf("lang=%s success = %v, message = %q, want failure with %q", lang, body.Success, body.Message, want)
		}
		if len(body.Data) != 0 {
			t.Fatalf("lang=%s failed response must not carry data, got %s", lang, body.Data)
		}
	}
}

// 模型名首尾空白应被去除；没有条目的字段不出现在 JSON 中，有条目的字段原样返回。
func TestGetBuiltinModelPricingReturnsBuiltinEntries(t *testing.T) {
	resp := performBuiltinModelPricingRequest(t, "/api/option/builtin_model_pricing?model=%20claude-sonnet-4.5%20", "zh")
	if resp.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.Code, http.StatusOK)
	}

	var body struct {
		Success bool                      `json:"success"`
		Data    ratio.BuiltinModelPricing `json:"data"`
	}
	if err := json.Unmarshal(resp.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if !body.Success {
		t.Fatalf("success = false, message = %q", resp.Body.String())
	}
	want := ratio.GetBuiltinModelPricing("claude-sonnet-4.5")
	if !reflect.DeepEqual(body.Data, want) {
		t.Fatalf("data = %+v, want %+v", body.Data, want)
	}
	if !body.Data.Available || body.Data.ModelRatio == nil || body.Data.ContextPricing == nil {
		t.Fatalf("claude-sonnet-4.5 should return available ratio and context pricing, got %+v", body.Data)
	}

	var payload struct {
		Data map[string]json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(resp.Body.Bytes(), &payload); err != nil {
		t.Fatalf("unmarshal response payload: %v", err)
	}
	for _, field := range []string{"model_name", "available", "model_ratio", "completion_ratio", "cache_ratio", "create_cache_ratio", "context_pricing"} {
		if _, ok := payload.Data[field]; !ok {
			t.Errorf("response data should contain field %q, got %v", field, payload.Data)
		}
	}
	for _, field := range []string{"model_price", "audio_ratio", "audio_completion_ratio"} {
		if _, ok := payload.Data[field]; ok {
			t.Errorf("response data should omit built-in entry %q that the model does not have, got %s", field, payload.Data[field])
		}
	}
}
