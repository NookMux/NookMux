package logcontroller

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/NookMux/NookMux/internal/i18n"
	logstore "github.com/NookMux/NookMux/internal/store/log"
	"github.com/gin-gonic/gin"
)

func TestValidateUserLogFiltersLocalized(t *testing.T) {
	gin.SetMode(gin.TestMode)
	if err := i18n.Init(); err != nil {
		t.Fatalf("i18n.Init failed: %v", err)
	}

	for _, tc := range []struct {
		lang string
		want string
	}{
		{lang: "zh", want: "无权使用此过滤条件：channel"},
		{lang: "en", want: "No permission to use this filter: channel"},
	} {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(http.MethodGet, "/api/log/self?channel=1", nil)
		c.Request.Header.Set("Accept-Language", tc.lang)

		got := validateUserLogFilters(c, logstore.LogStatFilter{Channel: 1})
		if got != tc.want {
			t.Fatalf("lang=%s got %q, want %q", tc.lang, got, tc.want)
		}
	}
}
