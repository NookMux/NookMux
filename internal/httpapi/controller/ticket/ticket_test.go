package ticketcontroller

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/NookMux/NookMux/internal/httpapi"
	"github.com/NookMux/NookMux/internal/i18n"
	"github.com/NookMux/NookMux/internal/store/db"
	"github.com/NookMux/NookMux/internal/store/ticket"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func setupTicketTestEnv(t *testing.T) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	if err := i18n.Init(); err != nil {
		t.Fatalf("i18n.Init failed: %v", err)
	}
	prevTranslate := httpapi.TranslateMessage
	httpapi.TranslateMessage = i18n.T
	oldDB := dbstore.DB

	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&ticketstore.Ticket{}, &ticketstore.TicketEntry{}); err != nil {
		t.Fatalf("migrate ticket tables: %v", err)
	}
	dbstore.DB = db

	t.Cleanup(func() {
		httpapi.TranslateMessage = prevTranslate
		if sqlDB, err := db.DB(); err == nil {
			_ = sqlDB.Close()
		}
		dbstore.DB = oldDB
	})
}

func TestTicketValidationAndNotFoundErrorsAreLocalized(t *testing.T) {
	setupTicketTestEnv(t)

	for _, tc := range []struct {
		name string
		lang string
		want string
	}{
		{name: "zh empty title", lang: "zh", want: "工单标题不能为空"},
		{name: "en empty title", lang: "en", want: "Ticket title cannot be empty"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Set("id", 1)
			c.Set("username", "tester")
			c.Set("role", 1)
			c.Request = httptest.NewRequest(http.MethodPost, "/api/ticket/", strings.NewReader(`{"title":"   ","type":"bug","content":"hello"}`))
			c.Request.Header.Set("Content-Type", "application/json")
			c.Request.Header.Set("Accept-Language", tc.lang)

			CreateTicket(c)

			var resp struct {
				Success bool   `json:"success"`
				Message string `json:"message"`
			}
			if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
				t.Fatalf("decode response: %v", err)
			}
			if resp.Success || resp.Message != tc.want {
				t.Fatalf("got success=%v message=%q, want false / %q", resp.Success, resp.Message, tc.want)
			}
		})
	}

	t.Run("en not found does not mask behind MsgDatabaseError", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Set("id", 1)
		c.Set("role", 1)
		c.Params = gin.Params{{Key: "id", Value: "999"}}
		c.Request = httptest.NewRequest(http.MethodGet, "/api/ticket/999", nil)
		c.Request.Header.Set("Accept-Language", "en")

		GetTicketDetail(c)

		var resp struct {
			Success bool   `json:"success"`
			Message string `json:"message"`
		}
		if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
			t.Fatalf("decode response: %v", err)
		}
		if resp.Success || resp.Message != "Ticket not found" {
			t.Fatalf("got success=%v message=%q, want false / %q", resp.Success, resp.Message, "Ticket not found")
		}
	})
}
