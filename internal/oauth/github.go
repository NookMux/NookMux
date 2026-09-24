package oauth

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"github.com/NookMux/NookMux/internal/common"
	"github.com/NookMux/NookMux/internal/i18n"
	"github.com/NookMux/NookMux/internal/infra/log"
	"github.com/NookMux/NookMux/internal/store/user"
	"github.com/NookMux/NookMux/pkg/jsonx"
	"github.com/gin-gonic/gin"
	"io"
	"net/http"
	"strconv"
	"time"
)

func init() {
	Register("github", &GitHubProvider{})
}

// GitHubProvider implements OAuth for GitHub
type GitHubProvider struct{}

type gitHubOAuthResponse struct {
	AccessToken string `json:"access_token"`
	Scope       string `json:"scope"`
	TokenType   string `json:"token_type"`
}

type gitHubUser struct {
	Id    int64  `json:"id"`    // GitHub numeric ID (permanent, never changes)
	Login string `json:"login"` // GitHub username (can be changed by user)
	Name  string `json:"name"`
	Email string `json:"email"`
}

func (p *GitHubProvider) GetName() string {
	return "GitHub"
}

func (p *GitHubProvider) IsEnabled() bool {
	return common.GitHubOAuthEnabled
}

func (p *GitHubProvider) ExchangeToken(ctx context.Context, code string, c *gin.Context) (*OAuthToken, error) {
	if code == "" {
		return nil, NewOAuthError(i18n.MsgOAuthInvalidCode, nil)
	}

	log.LogDebug(ctx, "[OAuth-GitHub] ExchangeToken: code=%s...", code[:min(len(code), 10)])

	values := map[string]string{
		"client_id":     common.GitHubClientId,
		"client_secret": common.GitHubClientSecret,
		"code":          code,
	}
	jsonData, err := jsonx.Marshal(values)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, "POST", "https://github.com/login/oauth/access_token", bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	client := http.Client{
		Timeout: 20 * time.Second,
	}
	res, err := client.Do(req)
	if err != nil {
		log.LogError(ctx, fmt.Sprintf("[OAuth-GitHub] ExchangeToken error: %s", err.Error()))
		return nil, NewOAuthErrorWithRaw(i18n.MsgOAuthConnectFailed, map[string]any{"Provider": "GitHub"}, err.Error())
	}
	defer res.Body.Close()

	log.LogDebug(ctx, "[OAuth-GitHub] ExchangeToken response status: %d", res.StatusCode)

	var oAuthResponse gitHubOAuthResponse
	err = jsonx.DecodeJson(res.Body, &oAuthResponse)
	if err != nil {
		log.LogError(ctx, fmt.Sprintf("[OAuth-GitHub] ExchangeToken decode error: %s", err.Error()))
		return nil, err
	}

	if oAuthResponse.AccessToken == "" {
		log.LogError(ctx, "[OAuth-GitHub] ExchangeToken failed: empty access token")
		return nil, NewOAuthError(i18n.MsgOAuthTokenFailed, map[string]any{"Provider": "GitHub"})
	}

	log.LogDebug(ctx, "[OAuth-GitHub] ExchangeToken success: scope=%s", oAuthResponse.Scope)

	return &OAuthToken{
		AccessToken: oAuthResponse.AccessToken,
		TokenType:   oAuthResponse.TokenType,
		Scope:       oAuthResponse.Scope,
	}, nil
}

func (p *GitHubProvider) GetUserInfo(ctx context.Context, token *OAuthToken) (*OAuthUser, error) {
	log.LogDebug(ctx, "[OAuth-GitHub] GetUserInfo: fetching user info")

	req, err := http.NewRequestWithContext(ctx, "GET", "https://api.github.com/user", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token.AccessToken))

	client := http.Client{
		Timeout: 20 * time.Second,
	}
	res, err := client.Do(req)
	if err != nil {
		log.LogError(ctx, fmt.Sprintf("[OAuth-GitHub] GetUserInfo error: %s", err.Error()))
		return nil, NewOAuthErrorWithRaw(i18n.MsgOAuthConnectFailed, map[string]any{"Provider": "GitHub"}, err.Error())
	}
	defer res.Body.Close()

	log.LogDebug(ctx, "[OAuth-GitHub] GetUserInfo response status: %d", res.StatusCode)

	// Check for non-200 status codes before attempting to decode
	if res.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(res.Body)
		bodyStr := string(body)
		if len(bodyStr) > 500 {
			bodyStr = bodyStr[:500] + "..."
		}
		log.LogError(ctx, fmt.Sprintf("[OAuth-GitHub] GetUserInfo failed: status=%d, body=%s", res.StatusCode, bodyStr))
		return nil, NewOAuthErrorWithRaw(i18n.MsgOAuthGetUserErr, map[string]any{"Provider": "GitHub"}, fmt.Sprintf("status %d", res.StatusCode))
	}

	var githubUser gitHubUser
	err = jsonx.DecodeJson(res.Body, &githubUser)
	if err != nil {
		log.LogError(ctx, fmt.Sprintf("[OAuth-GitHub] GetUserInfo decode error: %s", err.Error()))
		return nil, err
	}

	if githubUser.Id == 0 || githubUser.Login == "" {
		log.LogError(ctx, "[OAuth-GitHub] GetUserInfo failed: empty id or login field")
		return nil, NewOAuthError(i18n.MsgOAuthUserInfoEmpty, map[string]any{"Provider": "GitHub"})
	}

	log.LogDebug(ctx, "[OAuth-GitHub] GetUserInfo success: id=%d, login=%s, name=%s, email=%s",
		githubUser.Id, githubUser.Login, githubUser.Name, githubUser.Email)

	return &OAuthUser{
		ProviderUserID: strconv.FormatInt(githubUser.Id, 10), // Use numeric ID as primary identifier
		Username:       githubUser.Login,
		DisplayName:    githubUser.Name,
		Email:          githubUser.Email,
		Extra: map[string]any{
			"legacy_id": githubUser.Login, // Store login for migration from old accounts
		},
	}, nil
}

// Sentinel errors for LookupGitHubLogin, mapped by the caller to i18n messages.
var (
	// ErrGitHubLookupNotFound indicates the GitHub username does not exist.
	ErrGitHubLookupNotFound = errors.New("github lookup: user not found")
	// ErrGitHubLookupRateLimited indicates GitHub's unauthenticated rate limit
	// (60 req/h per IP) was hit.
	ErrGitHubLookupRateLimited = errors.New("github lookup: rate limited")
	// ErrGitHubLookupFailed indicates the request failed (network error, 5xx
	// or malformed response).
	ErrGitHubLookupFailed = errors.New("github lookup: request failed")
)

// githubAPIBaseURL is a package-level variable so tests can point it at an httptest server.
var githubAPIBaseURL = "https://api.github.com"

// LookupGitHubLogin resolves a GitHub username (login) to its permanent numeric
// account ID via the unauthenticated GET /users/{login} endpoint. The login must
// be pre-validated by the caller (^[a-zA-Z0-9-]+$) so it cannot alter the URL path.
func LookupGitHubLogin(ctx context.Context, login string) (int64, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, githubAPIBaseURL+"/users/"+login, nil)
	if err != nil {
		return 0, fmt.Errorf("%w: %v", ErrGitHubLookupFailed, err)
	}
	req.Header.Set("Accept", "application/vnd.github+json")

	client := http.Client{
		Timeout: 20 * time.Second,
	}
	res, err := client.Do(req)
	if err != nil {
		log.LogError(ctx, fmt.Sprintf("[OAuth-GitHub] LookupGitHubLogin error: %s", err.Error()))
		return 0, fmt.Errorf("%w: %v", ErrGitHubLookupFailed, err)
	}
	defer res.Body.Close()

	switch res.StatusCode {
	case http.StatusOK:
	case http.StatusNotFound:
		return 0, ErrGitHubLookupNotFound
	case http.StatusForbidden:
		log.LogError(ctx, fmt.Sprintf("[OAuth-GitHub] LookupGitHubLogin rate limited, X-RateLimit-Remaining=%s",
			res.Header.Get("X-RateLimit-Remaining")))
		return 0, ErrGitHubLookupRateLimited
	default:
		log.LogError(ctx, fmt.Sprintf("[OAuth-GitHub] LookupGitHubLogin failed: status=%d", res.StatusCode))
		return 0, fmt.Errorf("%w: status %d", ErrGitHubLookupFailed, res.StatusCode)
	}

	var githubUser gitHubUser
	if err := jsonx.DecodeJson(res.Body, &githubUser); err != nil {
		log.LogError(ctx, fmt.Sprintf("[OAuth-GitHub] LookupGitHubLogin decode error: %s", err.Error()))
		return 0, fmt.Errorf("%w: %v", ErrGitHubLookupFailed, err)
	}
	if githubUser.Id == 0 {
		log.LogError(ctx, "[OAuth-GitHub] LookupGitHubLogin failed: empty id field")
		return 0, fmt.Errorf("%w: empty id", ErrGitHubLookupFailed)
	}

	log.LogDebug(ctx, "[OAuth-GitHub] LookupGitHubLogin success: login=%s, id=%d", login, githubUser.Id)
	return githubUser.Id, nil
}

func (p *GitHubProvider) IsUserIDTaken(providerUserID string) bool {
	return userstore.IsGitHubIdAlreadyTaken(providerUserID)
}

func (p *GitHubProvider) FillUserByProviderID(user *userstore.User, providerUserID string) error {
	user.GitHubId = providerUserID
	return user.FillUserByGitHubId()
}

func (p *GitHubProvider) SetProviderUserID(user *userstore.User, providerUserID string) {
	user.GitHubId = providerUserID
}
