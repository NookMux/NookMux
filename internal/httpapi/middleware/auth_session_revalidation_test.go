package middleware

import (
	"github.com/NookMux/NookMux/internal/common"
	"github.com/NookMux/NookMux/internal/store/db"
	"github.com/NookMux/NookMux/internal/store/user"
	"github.com/gin-contrib/sessions"
	"github.com/gin-contrib/sessions/cookie"
	"github.com/gin-gonic/gin"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// createSessionUser inserts a user with the given role/status into the test DB.
func createSessionUser(t *testing.T, id int, username string, role, status int) userstore.User {
	t.Helper()
	user := userstore.User{
		Id:       id,
		Username: username,
		Password: "password123",
		Role:     role,
		Status:   status,
		Group:    "default",
		AffCode:  "session-aff-" + username,
	}
	if err := dbstore.DB.Create(&user).Error; err != nil {
		t.Fatalf("create user %s: %v", username, err)
	}
	return user
}

// loginViaSession mimics setupLogin by writing the session snapshot based on
// the user's *original* role/status. This is what the cookie stores at login
// time. Subsequent DB mutations should be reflected by authHelper's
// re-validation.
func loginViaSession(t *testing.T, router *gin.Engine, user userstore.User) []*http.Cookie {
	t.Helper()
	loginRecorder := httptest.NewRecorder()
	loginReq := httptest.NewRequest(http.MethodGet, "/login", nil)
	router.ServeHTTP(loginRecorder, loginReq)
	if loginRecorder.Code != http.StatusNoContent {
		t.Fatalf("login status = %d, body = %s", loginRecorder.Code, loginRecorder.Body.String())
	}
	return loginRecorder.Result().Cookies()
}

func newSessionAuthRouter(t *testing.T) (*gin.Engine, []string) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(sessions.Sessions("session", cookie.NewStore([]byte("session-revalidation-test"))))
	router.GET("/login", func(c *gin.Context) {
		// Pre-populate session with role=admin, status=enabled to simulate
		// a cookie issued before the user was demoted/disabled.
		session := sessions.Default(c)
		session.Set("username", "admin")
		session.Set("role", common.RoleAdminUser)
		session.Set("id", 1)
		session.Set("status", common.UserStatusEnabled)
		session.Set("group", "default")
		// 与 SetupLogin 保持一致：登录时快照凭证版本（建表默认值为 1）。
		session.Set("auth_version", 1)
		if err := session.Save(); err != nil {
			t.Fatalf("save session: %v", err)
		}
		c.Status(http.StatusNoContent)
	})
	var cookies []string
	router.GET("/protected", AdminAuth(), func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"id":      c.GetInt("id"),
			"role":    c.GetInt("role"),
		})
	})
	return router, cookies
}

// doProtectedRequest runs a request against /protected with the given session
// cookies and returns the recorder.
func doProtectedRequest(router *gin.Engine, cookies []*http.Cookie) *httptest.ResponseRecorder {
	recorder := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	req.Header.Set("New-Api-User", "1")
	for _, c := range cookies {
		req.AddCookie(c)
	}
	router.ServeHTTP(recorder, req)
	return recorder
}

// sessionCleared compares the request cookies with the response Set-Cookie
// values. cookie-based session stores clear the session by rewriting the
// cookie value to an empty session payload, so we check that the Set-Cookie
// value differs from the original.
func sessionCleared(originalCookies []*http.Cookie, recorder *httptest.ResponseRecorder) bool {
	var originalSession string
	for _, c := range originalCookies {
		if c.Name == "session" {
			originalSession = c.Value
		}
	}
	for _, c := range recorder.Result().Cookies() {
		if c.Name == "session" && c.Value != originalSession {
			return true
		}
	}
	return false
}

// TestSessionAuthRejectsDisabledUserWithStaleCookie proves that an admin
// disabling a user invalidates the user's existing session cookie: the next
// request with the old cookie is rejected and the cookie is cleared.
func TestSessionAuthRejectsDisabledUserWithStaleCookie(t *testing.T) {
	setupAuthAccessTokenTestDB(t)
	// The user exists and is enabled at "login" time (session snapshot status=enabled).
	createSessionUser(t, 1, "admin", common.RoleAdminUser, common.UserStatusEnabled)

	router, _ := newSessionAuthRouter(t)
	cookies := loginViaSession(t, router, userstore.User{})

	// Admin now disables the user.
	if err := dbstore.DB.Model(&userstore.User{}).Where("id = ?", 1).
		Update("status", common.UserStatusDisabled).Error; err != nil {
		t.Fatalf("disable user: %v", err)
	}
	invalidateSecuritySensitiveUserCachesForTest(t, 1)

	recorder := doProtectedRequest(router, cookies)
	body := recorder.Body.String()
	if strings.Contains(body, `"success":true`) {
		t.Fatalf("disabled user with stale cookie was admitted, body: %s", body)
	}
	if !strings.Contains(body, "用户已被封禁") {
		t.Fatalf("expected disabled user rejection, got: %s", body)
	}
	// The response should also clear the session cookie.
	if !sessionCleared(cookies, recorder) {
		t.Fatalf("expected Set-Cookie to rewrite the session, got headers: %v", recorder.Header()["Set-Cookie"])
	}
}

// TestSessionAuthRejectsDemotedUserWithStaleCookie proves that demoting an
// admin to a common user invalidates their admin session immediately.
func TestSessionAuthRejectsDemotedUserWithStaleCookie(t *testing.T) {
	setupAuthAccessTokenTestDB(t)
	createSessionUser(t, 1, "admin", common.RoleAdminUser, common.UserStatusEnabled)

	router, _ := newSessionAuthRouter(t)
	cookies := loginViaSession(t, router, userstore.User{})

	// Admin demotes the user.
	if err := dbstore.DB.Model(&userstore.User{}).Where("id = ?", 1).
		Update("role", common.RoleCommonUser).Error; err != nil {
		t.Fatalf("demote user: %v", err)
	}
	invalidateSecuritySensitiveUserCachesForTest(t, 1)

	recorder := doProtectedRequest(router, cookies)
	body := recorder.Body.String()
	if strings.Contains(body, `"success":true`) {
		t.Fatalf("demoted user with stale admin cookie was admitted, body: %s", body)
	}
	if !strings.Contains(body, "权限不足") {
		t.Fatalf("expected permission-denied response, got: %s", body)
	}
	if !sessionCleared(cookies, recorder) {
		t.Fatalf("expected Set-Cookie to rewrite the session, got headers: %v", recorder.Header()["Set-Cookie"])
	}
}

// TestSessionAuthRejectsDeletedUserWithStaleCookie proves that deleting a user
// invalidates their session.
func TestSessionAuthRejectsDeletedUserWithStaleCookie(t *testing.T) {
	setupAuthAccessTokenTestDB(t)
	createSessionUser(t, 1, "admin", common.RoleAdminUser, common.UserStatusEnabled)

	router, _ := newSessionAuthRouter(t)
	cookies := loginViaSession(t, router, userstore.User{})

	// Hard delete the user.
	if err := dbstore.DB.Unscoped().Where("id = ?", 1).Delete(&userstore.User{}).Error; err != nil {
		t.Fatalf("delete user: %v", err)
	}
	invalidateSecuritySensitiveUserCachesForTest(t, 1)

	recorder := doProtectedRequest(router, cookies)
	body := recorder.Body.String()
	if strings.Contains(body, `"success":true`) {
		t.Fatalf("deleted user with stale cookie was admitted, body: %s", body)
	}
	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 for deleted user, got status %d, body: %s", recorder.Code, body)
	}
	if !sessionCleared(cookies, recorder) {
		t.Fatalf("expected Set-Cookie to rewrite the session, got headers: %v", recorder.Header()["Set-Cookie"])
	}
}

// TestSessionAuthAllowsValidUser proves the happy path still works: an
// unchanged admin session can still access admin routes.
func TestSessionAuthAllowsValidUser(t *testing.T) {
	setupAuthAccessTokenTestDB(t)
	createSessionUser(t, 1, "admin", common.RoleAdminUser, common.UserStatusEnabled)

	router, _ := newSessionAuthRouter(t)
	cookies := loginViaSession(t, router, userstore.User{})

	recorder := doProtectedRequest(router, cookies)
	if recorder.Code != http.StatusOK {
		t.Fatalf("expected 200 for valid admin, got %d, body: %s", recorder.Code, recorder.Body.String())
	}
	if !strings.Contains(recorder.Body.String(), `"success":true`) {
		t.Fatalf("expected success response, got: %s", recorder.Body.String())
	}
}

func invalidateSecuritySensitiveUserCachesForTest(t *testing.T, userId int) {
	t.Helper()
	if err := userstore.InvalidateUserCache(userId); err != nil {
		t.Fatalf("invalidate user cache: %v", err)
	}
}
