package misccontroller

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/NookMux/NookMux/internal/common"
	"github.com/NookMux/NookMux/internal/httpapi"
	"github.com/NookMux/NookMux/internal/httpapi/controller/testsupport"
	"github.com/NookMux/NookMux/internal/i18n"
	"github.com/NookMux/NookMux/internal/infra/security"
	"github.com/NookMux/NookMux/internal/store/db"
	"github.com/NookMux/NookMux/internal/store/user"
	"github.com/gin-gonic/gin"
)

func TestSendPasswordResetEmailHidesUnknownEmail(t *testing.T) {
	testsupport.SetupSecureVerificationTestDB(t)
	gin.SetMode(gin.TestMode)

	router := gin.New()
	router.GET("/api/reset_password", SendPasswordResetEmail)

	req := httptest.NewRequest(http.MethodGet, "/api/reset_password?email=missing@example.com", nil)
	recorder := httptest.NewRecorder()

	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d, body = %s", recorder.Code, http.StatusOK, recorder.Body.String())
	}

	var body struct {
		Success bool   `json:"success"`
		Message string `json:"message"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if !body.Success {
		t.Fatalf("expected success response, got: %s", recorder.Body.String())
	}
	if body.Message != "" {
		t.Fatalf("message = %q, want empty", body.Message)
	}
}

// expectMsg renders an i18n key with the same machinery the handler uses, so
// assertions hold whether or not i18n has been initialized by an earlier test.
func expectMsg(t *testing.T, key string) string {
	t.Helper()

	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, "/", nil)
	return httpapi.TranslateMessage(c, key)
}

type resetPasswordAPIResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
	Data    any    `json:"data"`
}

func createResetPasswordTestUser(t *testing.T, email string) {
	t.Helper()

	u := userstore.User{
		Id:          1,
		Username:    "reset-user",
		Password:    "password123",
		Role:        common.RoleCommonUser,
		Status:      common.UserStatusEnabled,
		DisplayName: "Reset User",
		Email:       email,
		Group:       "default",
		AffCode:     "reset-aff-1",
	}
	if err := dbstore.DB.Create(&u).Error; err != nil {
		t.Fatalf("create user: %v", err)
	}
}

func postResetPassword(t *testing.T, body string) (*httptest.ResponseRecorder, resetPasswordAPIResponse) {
	t.Helper()

	router := gin.New()
	router.POST("/api/user/reset", ResetPassword)

	req := httptest.NewRequest(http.MethodPost, "/api/user/reset", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()

	router.ServeHTTP(recorder, req)

	var parsed resetPasswordAPIResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &parsed); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	return recorder, parsed
}

func getStoredPassword(t *testing.T, email string) string {
	t.Helper()

	var stored userstore.User
	if err := dbstore.DB.Where("email = ?", email).First(&stored).Error; err != nil {
		t.Fatalf("load user %s: %v", email, err)
	}
	return stored.Password
}

func TestResetPasswordUsesRequestedPassword(t *testing.T) {
	testsupport.SetupSecureVerificationTestDB(t)
	gin.SetMode(gin.TestMode)
	if err := i18n.Init(); err != nil {
		t.Fatalf("init i18n: %v", err)
	}

	const (
		email       = "reset-confirm@example.com"
		token       = "reset-token-1"
		newPassword = "NewPassword123"
	)
	createResetPasswordTestUser(t, email)
	security.RegisterVerificationCodeWithKey(email, token, security.PasswordResetPurpose)

	recorder, parsed := postResetPassword(t,
		`{"email":"`+email+`","token":"`+token+`","new_password":"`+newPassword+`"}`)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d, body = %s", recorder.Code, http.StatusOK, recorder.Body.String())
	}
	if !parsed.Success {
		t.Fatalf("expected success, got: %s", recorder.Body.String())
	}
	if parsed.Message != expectMsg(t, i18n.MsgMiscPasswordResetSuccess) {
		t.Fatalf("message = %q, want %q", parsed.Message, expectMsg(t, i18n.MsgMiscPasswordResetSuccess))
	}
	if strings.Contains(recorder.Body.String(), newPassword) {
		t.Fatalf("response body must not contain the new password: %s", recorder.Body.String())
	}
	if parsed.Data != nil {
		t.Fatalf("data = %v, want nil", parsed.Data)
	}

	stored := getStoredPassword(t, email)
	if !security.ValidatePasswordAndHash(newPassword, stored) {
		t.Fatalf("stored password was not updated to the requested password")
	}
	if security.VerifyCodeWithKey(email, token, security.PasswordResetPurpose) {
		t.Fatalf("reset token should be consumed after a successful reset")
	}
}

func TestResetPasswordRequiresNewPassword(t *testing.T) {
	testsupport.SetupSecureVerificationTestDB(t)
	gin.SetMode(gin.TestMode)
	if err := i18n.Init(); err != nil {
		t.Fatalf("init i18n: %v", err)
	}

	const (
		email = "reset-missing@example.com"
		token = "reset-token-2"
	)
	createResetPasswordTestUser(t, email)
	security.RegisterVerificationCodeWithKey(email, token, security.PasswordResetPurpose)
	storedBefore := getStoredPassword(t, email)

	recorder, parsed := postResetPassword(t,
		`{"email":"`+email+`","token":"`+token+`"}`)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d, body = %s", recorder.Code, http.StatusOK, recorder.Body.String())
	}
	if parsed.Success {
		t.Fatalf("expected rejection when new_password is missing, got: %s", recorder.Body.String())
	}
	if parsed.Message != expectMsg(t, i18n.MsgInvalidParams) {
		t.Fatalf("message = %q, want %q", parsed.Message, expectMsg(t, i18n.MsgInvalidParams))
	}
	if stored := getStoredPassword(t, email); stored != storedBefore {
		t.Fatalf("password must stay unchanged when new_password is missing")
	}
}

func TestResetPasswordRejectsWeakPassword(t *testing.T) {
	testsupport.SetupSecureVerificationTestDB(t)
	gin.SetMode(gin.TestMode)
	if err := i18n.Init(); err != nil {
		t.Fatalf("init i18n: %v", err)
	}

	const (
		email        = "reset-weak@example.com"
		token        = "reset-token-3"
		weakPassword = "short"
	)
	createResetPasswordTestUser(t, email)
	security.RegisterVerificationCodeWithKey(email, token, security.PasswordResetPurpose)
	storedBefore := getStoredPassword(t, email)

	recorder, parsed := postResetPassword(t,
		`{"email":"`+email+`","token":"`+token+`","new_password":"`+weakPassword+`"}`)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d, body = %s", recorder.Code, http.StatusOK, recorder.Body.String())
	}
	if parsed.Success {
		t.Fatalf("expected rejection for weak password, got: %s", recorder.Body.String())
	}
	if parsed.Message != expectMsg(t, i18n.MsgMiscPasswordInvalid) {
		t.Fatalf("message = %q, want %q", parsed.Message, expectMsg(t, i18n.MsgMiscPasswordInvalid))
	}
	if stored := getStoredPassword(t, email); stored != storedBefore {
		t.Fatalf("password must stay unchanged when the new password is weak")
	}
}

func TestResetPasswordRejectsInvalidToken(t *testing.T) {
	testsupport.SetupSecureVerificationTestDB(t)
	gin.SetMode(gin.TestMode)
	if err := i18n.Init(); err != nil {
		t.Fatalf("init i18n: %v", err)
	}

	const (
		email       = "reset-invalid-token@example.com"
		token       = "reset-token-4"
		newPassword = "NewPassword123"
	)
	createResetPasswordTestUser(t, email)
	security.RegisterVerificationCodeWithKey(email, token, security.PasswordResetPurpose)
	storedBefore := getStoredPassword(t, email)

	recorder, parsed := postResetPassword(t,
		`{"email":"`+email+`","token":"wrong-token","new_password":"`+newPassword+`"}`)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d, body = %s", recorder.Code, http.StatusOK, recorder.Body.String())
	}
	if parsed.Success {
		t.Fatalf("expected rejection for invalid token, got: %s", recorder.Body.String())
	}
	if parsed.Message != expectMsg(t, i18n.MsgMiscPasswordResetLinkInvalid) {
		t.Fatalf("message = %q, want %q", parsed.Message, expectMsg(t, i18n.MsgMiscPasswordResetLinkInvalid))
	}
	if stored := getStoredPassword(t, email); stored != storedBefore {
		t.Fatalf("password must stay unchanged when the token is invalid")
	}
}

func TestSendPasswordResetEmailReturnsSuccessWhenDeliveryFails(t *testing.T) {
	testsupport.SetupSecureVerificationTestDB(t)
	gin.SetMode(gin.TestMode)

	createResetPasswordTestUser(t, "reset-user@example.com")

	router := gin.New()
	router.GET("/api/reset_password", SendPasswordResetEmail)

	req := httptest.NewRequest(http.MethodGet, "/api/reset_password?email=reset-user@example.com", nil)
	recorder := httptest.NewRecorder()

	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d, body = %s", recorder.Code, http.StatusOK, recorder.Body.String())
	}

	var body struct {
		Success bool   `json:"success"`
		Message string `json:"message"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if !body.Success {
		t.Fatalf("expected success response despite email delivery failure, got: %s", recorder.Body.String())
	}
	if body.Message != "" {
		t.Fatalf("message = %q, want empty", body.Message)
	}
}
