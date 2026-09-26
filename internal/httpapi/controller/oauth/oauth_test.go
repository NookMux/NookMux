package oauthcontroller

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/NookMux/NookMux/internal/common"
	"github.com/NookMux/NookMux/internal/httpapi/controller/testsupport"
	"github.com/NookMux/NookMux/internal/i18n"
	"github.com/NookMux/NookMux/internal/oauth"
	"github.com/NookMux/NookMux/internal/store/db"
	"github.com/NookMux/NookMux/internal/store/user"
	"github.com/gin-contrib/sessions"
	"github.com/gin-contrib/sessions/cookie"
	"github.com/gin-gonic/gin"
	"net/http"
	"net/http/httptest"
	"testing"
)

type oauthBindTestProvider struct{}

func (p *oauthBindTestProvider) GetName() string { return "OAuthBindTest" }

func (p *oauthBindTestProvider) IsEnabled() bool { return true }

func (p *oauthBindTestProvider) ExchangeToken(ctx context.Context, code string, c *gin.Context) (*oauth.OAuthToken, error) {
	return &oauth.OAuthToken{AccessToken: "bind-token"}, nil
}

func (p *oauthBindTestProvider) GetUserInfo(ctx context.Context, token *oauth.OAuthToken) (*oauth.OAuthUser, error) {
	return &oauth.OAuthUser{
		ProviderUserID: "provider-user-1",
		Username:       "bind-user",
		Extra:          map[string]any{},
	}, nil
}

func (p *oauthBindTestProvider) IsUserIDTaken(providerUserID string) bool { return false }

func (p *oauthBindTestProvider) FillUserByProviderID(user *userstore.User, providerUserID string) error {
	return nil
}

func (p *oauthBindTestProvider) SetProviderUserID(user *userstore.User, providerUserID string) {
	user.GitHubId = providerUserID
}

func TestHandleOAuthBindReturnsStructuredBindAction(t *testing.T) {
	testsupport.SetupSecureVerificationTestDB(t)
	gin.SetMode(gin.TestMode)

	oauth.Register("oauth-bind-test", &oauthBindTestProvider{})

	user := testsupport.CreateSecureVerificationTestUser(t, 1, "oauth-bind-access-token")

	router := gin.New()
	router.Use(sessions.Sessions("session", cookie.NewStore([]byte("test-secret"))))
	router.Use(func(c *gin.Context) {
		session := sessions.Default(c)
		session.Set("id", user.Id)
		session.Set("username", user.Username)
		session.Set("oauth_state", "state-123")
		_ = session.Save()
		c.Next()
	})
	router.GET("/api/oauth/:provider", HandleOAuth)

	req := httptest.NewRequest(http.MethodGet, "/api/oauth/oauth-bind-test?code=abc&state=state-123", nil)
	recorder := httptest.NewRecorder()

	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d, body = %s", recorder.Code, http.StatusOK, recorder.Body.String())
	}

	var body struct {
		Success bool   `json:"success"`
		Message string `json:"message"`
		Data    struct {
			Action string `json:"action"`
		} `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if !body.Success {
		t.Fatalf("expected success response, got: %s", recorder.Body.String())
	}
	if body.Message == "" {
		t.Fatalf("expected translated bind success message, got empty response body: %s", recorder.Body.String())
	}
	if body.Data.Action != "bind" {
		t.Fatalf("data.action = %q, want %q", body.Data.Action, "bind")
	}

	updatedUser, err := userstore.GetUserById(user.Id, true)
	if err != nil {
		t.Fatalf("get updated user: %v", err)
	}
	if updatedUser.GitHubId != "provider-user-1" {
		t.Fatalf("github_id = %q, want %q", updatedUser.GitHubId, "provider-user-1")
	}
}

type oauthLoginTestProvider struct {
	userInfo *oauth.OAuthUser
}

func (p *oauthLoginTestProvider) GetName() string { return "OAuthLoginTest" }

func (p *oauthLoginTestProvider) IsEnabled() bool { return true }

func (p *oauthLoginTestProvider) ExchangeToken(ctx context.Context, code string, c *gin.Context) (*oauth.OAuthToken, error) {
	return &oauth.OAuthToken{AccessToken: "login-token"}, nil
}

func (p *oauthLoginTestProvider) GetUserInfo(ctx context.Context, token *oauth.OAuthToken) (*oauth.OAuthUser, error) {
	return p.userInfo, nil
}

func (p *oauthLoginTestProvider) IsUserIDTaken(providerUserID string) bool {
	return userstore.IsGitHubIdAlreadyTaken(providerUserID)
}

func (p *oauthLoginTestProvider) FillUserByProviderID(user *userstore.User, providerUserID string) error {
	user.GitHubId = providerUserID
	return user.FillUserByGitHubId()
}

func (p *oauthLoginTestProvider) SetProviderUserID(user *userstore.User, providerUserID string) {
	user.GitHubId = providerUserID
}

type oauthLoginResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
	Data    struct {
		Id int `json:"id"`
	} `json:"data"`
}

func createOAuthTestUser(t *testing.T, id int, githubId string) userstore.User {
	t.Helper()

	u := userstore.User{
		Id:          id,
		Username:    fmt.Sprintf("oauth-user-%d", id),
		Password:    "password123",
		Role:        common.RoleCommonUser,
		Status:      common.UserStatusEnabled,
		DisplayName: fmt.Sprintf("OAuth User %d", id),
		Group:       "default",
		GitHubId:    githubId,
	}
	if err := dbstore.DB.Create(&u).Error; err != nil {
		t.Fatalf("create user: %v", err)
	}
	return u
}

func getOAuthTestUser(t *testing.T, id int) userstore.User {
	t.Helper()

	var u userstore.User
	if err := dbstore.DB.First(&u, id).Error; err != nil {
		t.Fatalf("get user %d: %v", id, err)
	}
	return u
}

// newOAuthLoginRouter builds a router whose session carries only the OAuth state,
// so the callback takes the login path instead of the bind path.
func newOAuthLoginRouter() *gin.Engine {
	router := gin.New()
	router.Use(sessions.Sessions("session", cookie.NewStore([]byte("test-secret"))))
	router.Use(func(c *gin.Context) {
		session := sessions.Default(c)
		session.Set("oauth_state", "state-123")
		_ = session.Save()
		c.Next()
	})
	router.GET("/api/oauth/:provider", HandleOAuth)
	return router
}

func performOAuthLogin(t *testing.T, router *gin.Engine, providerName string) (*httptest.ResponseRecorder, oauthLoginResponse) {
	t.Helper()

	req := httptest.NewRequest(http.MethodGet, "/api/oauth/"+providerName+"?code=abc&state=state-123", nil)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	var body oauthLoginResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response %q: %v", recorder.Body.String(), err)
	}
	return recorder, body
}

func TestHandleOAuthNonNumericLegacyIDDoesNotAuthenticateLegacyAccount(t *testing.T) {
	if err := i18n.Init(); err != nil {
		t.Fatalf("init i18n: %v", err)
	}
	testsupport.SetupSecureVerificationTestDB(t)
	gin.SetMode(gin.TestMode)

	provider := &oauthLoginTestProvider{userInfo: &oauth.OAuthUser{
		ProviderUserID: "987654321",
		Username:       "github_login",
		Extra:          map[string]any{"legacy_id": "github_login"},
	}}
	oauth.Register("oauth-login-non-numeric", provider)

	legacyUser := createOAuthTestUser(t, 21, "github_login")

	oldRegisterEnabled := common.RegisterEnabled
	common.RegisterEnabled = true
	t.Cleanup(func() { common.RegisterEnabled = oldRegisterEnabled })

	recorder, body := performOAuthLogin(t, newOAuthLoginRouter(), "oauth-login-non-numeric")

	if recorder.Code != http.StatusOK || !body.Success {
		t.Fatalf("response = %s, want success", recorder.Body.String())
	}
	if body.Data.Id == legacyUser.Id {
		t.Fatalf("authenticated user id = %d, want a new user (non-numeric legacy_id must not match the legacy account)", body.Data.Id)
	}

	attacker := userstore.User{}
	if err := dbstore.DB.Where("github_id = ?", "987654321").First(&attacker).Error; err != nil {
		t.Fatalf("find newly created user bound to numeric provider id: %v", err)
	}
	if attacker.Id != body.Data.Id {
		t.Fatalf("session user id = %d, want newly created user %d", body.Data.Id, attacker.Id)
	}

	if got := getOAuthTestUser(t, legacyUser.Id).GitHubId; got != "github_login" {
		t.Fatalf("legacy github_id = %q, want unchanged %q", got, "github_login")
	}
}

func TestHandleOAuthNumericLegacyIDMigratesLegacyAccount(t *testing.T) {
	if err := i18n.Init(); err != nil {
		t.Fatalf("init i18n: %v", err)
	}
	testsupport.SetupSecureVerificationTestDB(t)
	gin.SetMode(gin.TestMode)

	provider := &oauthLoginTestProvider{userInfo: &oauth.OAuthUser{
		ProviderUserID: "123456789",
		Username:       "legacy-user",
		Extra:          map[string]any{"legacy_id": "583231"},
	}}
	oauth.Register("oauth-login-numeric", provider)

	legacyUser := createOAuthTestUser(t, 31, "583231")

	recorder, body := performOAuthLogin(t, newOAuthLoginRouter(), "oauth-login-numeric")

	if recorder.Code != http.StatusOK || !body.Success {
		t.Fatalf("response = %s, want success", recorder.Body.String())
	}
	if body.Data.Id != legacyUser.Id {
		t.Fatalf("authenticated user id = %d, want legacy user %d", body.Data.Id, legacyUser.Id)
	}

	if got := getOAuthTestUser(t, legacyUser.Id).GitHubId; got != "123456789" {
		t.Fatalf("github_id = %q, want migrated %q", got, "123456789")
	}
}

func TestHandleOAuthBoundProviderUserIDLogsInWithoutMigration(t *testing.T) {
	if err := i18n.Init(); err != nil {
		t.Fatalf("init i18n: %v", err)
	}
	testsupport.SetupSecureVerificationTestDB(t)
	gin.SetMode(gin.TestMode)

	provider := &oauthLoginTestProvider{userInfo: &oauth.OAuthUser{
		ProviderUserID: "583231",
		Username:       "bound-user",
		Extra:          map[string]any{"legacy_id": "some-login"},
	}}
	oauth.Register("oauth-login-bound", provider)

	boundUser := createOAuthTestUser(t, 41, "583231")

	recorder, body := performOAuthLogin(t, newOAuthLoginRouter(), "oauth-login-bound")

	if recorder.Code != http.StatusOK || !body.Success {
		t.Fatalf("response = %s, want success", recorder.Body.String())
	}
	if body.Data.Id != boundUser.Id {
		t.Fatalf("authenticated user id = %d, want bound user %d", body.Data.Id, boundUser.Id)
	}

	if got := getOAuthTestUser(t, boundUser.Id).GitHubId; got != "583231" {
		t.Fatalf("github_id = %q, want unchanged %q", got, "583231")
	}
}
