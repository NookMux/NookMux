package auditcontroller

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/NookMux/NookMux/internal/i18n"
	auditstore "github.com/NookMux/NookMux/internal/store/audit"
	"github.com/gin-gonic/gin"
)

func TestGetAuditModulesLocalized(t *testing.T) {
	gin.SetMode(gin.TestMode)
	if err := i18n.Init(); err != nil {
		t.Fatalf("i18n.Init failed: %v", err)
	}

	for _, tc := range []struct {
		lang       string
		wantFirst  string
		wantConfig string
	}{
		{lang: "zh", wantFirst: "系统设置", wantConfig: "数据看板配置"},
		{lang: "en", wantFirst: "System Settings", wantConfig: "Dashboard Config"},
	} {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(http.MethodGet, "/api/audit/modules", nil)
		c.Request.Header.Set("Accept-Language", tc.lang)

		GetAuditModules(c)

		var resp struct {
			Success bool                         `json:"success"`
			Data    []auditstore.AuditModuleInfo `json:"data"`
		}
		if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
			t.Fatalf("decode response: %v", err)
		}
		if !resp.Success || len(resp.Data) != len(auditstore.AuditModuleList) {
			t.Fatalf("unexpected modules payload: success=%v len=%d", resp.Success, len(resp.Data))
		}
		if resp.Data[0].Label != tc.wantFirst {
			t.Fatalf("lang=%s first label = %q, want %q", tc.lang, resp.Data[0].Label, tc.wantFirst)
		}
		var dashLabel string
		for _, item := range resp.Data {
			if item.Value == auditstore.AuditModuleDashboardConfig {
				dashLabel = item.Label
				break
			}
		}
		if dashLabel != tc.wantConfig {
			t.Fatalf("lang=%s dashboard_config label = %q, want %q", tc.lang, dashLabel, tc.wantConfig)
		}
	}
}
