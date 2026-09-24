package channelcontroller

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/NookMux/NookMux/internal/domain/channel/constant"
	"github.com/NookMux/NookMux/pkg/jsonx"
	"github.com/gin-gonic/gin"
)

func TestGetBuiltinChannelURLs(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/channel/builtin_urls", nil)

	GetBuiltinChannelURLs(c)

	var resp struct {
		Success bool                                   `json:"success"`
		Data    map[string][]constant.BuiltinURLOption `json:"data"`
	}
	if err := jsonx.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if !resp.Success {
		t.Fatalf("expected success response, body: %s", w.Body.String())
	}
	if len(resp.Data) != len(constant.BuiltinChannelURLOptions) {
		t.Fatalf("expected %d channel types, got %d", len(constant.BuiltinChannelURLOptions), len(resp.Data))
	}

	// Go map[int] 序列化后 key 为字符串，前端按 String(type) 取用
	zhipu, ok := resp.Data["26"]
	if !ok {
		t.Fatalf("zhipu presets missing in response: %s", w.Body.String())
	}
	if len(zhipu) != 4 {
		t.Fatalf("zhipu expects 4 presets, got %d", len(zhipu))
	}
	planValues := map[string]bool{}
	for _, option := range zhipu {
		if option.Value == "" || option.LabelKey == "" {
			t.Errorf("zhipu preset has empty value or label_key: %+v", option)
		}
		if _, isPlan := constant.ChannelSpecialBases[option.Value]; isPlan {
			planValues[option.Value] = true
		}
	}
	if !planValues["glm-coding-plan"] || !planValues["glm-coding-plan-international"] {
		t.Errorf("zhipu presets must expose both coding plan keys, got %v", planValues)
	}
}
