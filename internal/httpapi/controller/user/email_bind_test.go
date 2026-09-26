package usercontroller

import (
	"fmt"
	"github.com/NookMux/NookMux/internal/httpapi/controller/testsupport"
	"github.com/NookMux/NookMux/internal/i18n"
	"github.com/NookMux/NookMux/internal/infra/security"
	"github.com/NookMux/NookMux/internal/store/db"
	"github.com/NookMux/NookMux/internal/store/user"
	"github.com/NookMux/NookMux/pkg/jsonx"
	"github.com/gin-contrib/sessions"
	"github.com/gin-contrib/sessions/cookie"
	"github.com/gin-gonic/gin"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// No session identity but a valid verification code: the handler must reject
// with 401 instead of panicking on the missing session id.
func TestEmailBindRejectsMissingSession(t *testing.T) {
	testsupport.SetupSecureVerificationTestDB(t)
	gin.SetMode(gin.TestMode)

	email := "email-bind-anon@example.com"
	security.RegisterVerificationCodeWithKey(email, "123456", security.EmailVerificationPurpose)

	router := gin.New()
	router.Use(sessions.Sessions("session", cookie.NewStore([]byte("test-secret"))))
	router.POST("/api/oauth/email/bind", EmailBind)

	req := httptest.NewRequest(http.MethodPost, "/api/oauth/email/bind", strings.NewReader(`{"email":"email-bind-anon@example.com","code":"123456"}`))
	req.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()

	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, body = %s, want 401", recorder.Code, recorder.Body.String())
	}
	var parsed struct {
		Success bool   `json:"success"`
		Message string `json:"message"`
	}
	if err := jsonx.Unmarshal(recorder.Body.Bytes(), &parsed); err != nil {
		t.Fatalf("decode response %q: %v", recorder.Body.String(), err)
	}
	if parsed.Success {
		t.Fatal("missing session must not succeed")
	}
	if want := expectMsg(t, i18n.MsgNotLoggedIn); parsed.Message != want {
		t.Fatalf("message = %q, want %q", parsed.Message, want)
	}
}

func TestEmailBindUsesPostJsonBody(t *testing.T) {
	testsupport.SetupSecureVerificationTestDB(t)
	gin.SetMode(gin.TestMode)

	user := testsupport.CreateSecureVerificationTestUser(t, 1, "email-bind-token")
	email := fmt.Sprintf("email-bind-%d@example.com", user.Id)
	security.RegisterVerificationCodeWithKey(email, "123456", security.EmailVerificationPurpose)

	router := gin.New()
	router.Use(sessions.Sessions("session", cookie.NewStore([]byte("test-secret"))))
	router.Use(func(c *gin.Context) {
		session := sessions.Default(c)
		session.Set("id", user.Id)
		_ = session.Save()
		c.Next()
	})
	router.POST("/api/oauth/email/bind", EmailBind)

	body := fmt.Sprintf(`{"email":%q,"code":"123456"}`, email)
	req := httptest.NewRequest(http.MethodPost, "/api/oauth/email/bind?email=attacker@example.com&code=bad", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()

	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
	}

	var updated userstore.User
	if err := dbstore.DB.First(&updated, user.Id).Error; err != nil {
		t.Fatalf("query updated user: %v", err)
	}
	if updated.Email != email {
		t.Fatalf("updated email = %q, want %q", updated.Email, email)
	}
}
