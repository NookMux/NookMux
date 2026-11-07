package oauth

import (
	"context"
	"encoding/base64"
	"fmt"
	"github.com/NookMux/NookMux/internal/common"
	"github.com/NookMux/NookMux/internal/i18n"
	"github.com/NookMux/NookMux/internal/infra/log"
	"github.com/NookMux/NookMux/internal/store/user"
	"github.com/NookMux/NookMux/pkg/jsonx"
	"github.com/gin-gonic/gin"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

func init() {
	Register("linuxdo", &LinuxDOProvider{})
}

// LinuxDOProvider implements OAuth for Linux DO
type LinuxDOProvider struct{}

type linuxdoUser struct {
	Id         int    `json:"id"`
	Username   string `json:"username"`
	Name       string `json:"name"`
	Active     bool   `json:"active"`
	TrustLevel int    `json:"trust_level"`
	Silenced   bool   `json:"silenced"`
}

func (p *LinuxDOProvider) GetName() string {
	return "Linux DO"
}

func (p *LinuxDOProvider) IsEnabled() bool {
	return common.LinuxDOOAuthEnabled
}

func (p *LinuxDOProvider) ExchangeToken(ctx context.Context, code string, c *gin.Context) (*OAuthToken, error) {
	if code == "" {
		return nil, NewOAuthError(i18n.MsgOAuthInvalidCode, nil)
	}

	log.LogDebug(ctx, "[OAuth-LinuxDO] ExchangeToken: code=%s...", code[:min(len(code), 10)])

	// Get access token using Basic auth
	tokenEndpoint := common.GetEnvOrDefaultString("LINUX_DO_TOKEN_ENDPOINT", "https://connect.linux.do/oauth2/token")
	credentials := common.LinuxDOClientId + ":" + common.LinuxDOClientSecret
	basicAuth := "Basic " + base64.StdEncoding.EncodeToString([]byte(credentials))

	// Get redirect URI from request
	scheme := "http"
	if c.Request.TLS != nil {
		scheme = "https"
	}
	redirectURI := fmt.Sprintf("%s://%s/api/oauth/linuxdo", scheme, c.Request.Host)

	log.LogDebug(ctx, "[OAuth-LinuxDO] ExchangeToken: token_endpoint=%s, redirect_uri=%s", tokenEndpoint, redirectURI)

	data := url.Values{}
	data.Set("grant_type", "authorization_code")
	data.Set("code", code)
	data.Set("redirect_uri", redirectURI)

	req, err := http.NewRequestWithContext(ctx, "POST", tokenEndpoint, strings.NewReader(data.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", basicAuth)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")

	client := http.Client{Timeout: 5 * time.Second}
	res, err := client.Do(req)
	if err != nil {
		log.LogError(ctx, fmt.Sprintf("[OAuth-LinuxDO] ExchangeToken error: %s", err.Error()))
		return nil, NewOAuthErrorWithRaw(i18n.MsgOAuthConnectFailed, map[string]any{"Provider": "Linux DO"}, err.Error())
	}
	defer res.Body.Close()

	log.LogDebug(ctx, "[OAuth-LinuxDO] ExchangeToken response status: %d", res.StatusCode)

	var tokenRes struct {
		AccessToken string `json:"access_token"`
		Message     string `json:"message"`
	}
	if err := jsonx.DecodeJson(res.Body, &tokenRes); err != nil {
		log.LogError(ctx, fmt.Sprintf("[OAuth-LinuxDO] ExchangeToken decode error: %s", err.Error()))
		return nil, err
	}

	if tokenRes.AccessToken == "" {
		log.LogError(ctx, fmt.Sprintf("[OAuth-LinuxDO] ExchangeToken failed: %s", tokenRes.Message))
		return nil, NewOAuthErrorWithRaw(i18n.MsgOAuthTokenFailed, map[string]any{"Provider": "Linux DO"}, tokenRes.Message)
	}

	log.LogDebug(ctx, "[OAuth-LinuxDO] ExchangeToken success")

	return &OAuthToken{
		AccessToken: tokenRes.AccessToken,
	}, nil
}

func (p *LinuxDOProvider) GetUserInfo(ctx context.Context, token *OAuthToken) (*OAuthUser, error) {
	userEndpoint := common.GetEnvOrDefaultString("LINUX_DO_USER_ENDPOINT", "https://connect.linux.do/api/user")

	log.LogDebug(ctx, "[OAuth-LinuxDO] GetUserInfo: user_endpoint=%s", userEndpoint)

	req, err := http.NewRequestWithContext(ctx, "GET", userEndpoint, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token.AccessToken)
	req.Header.Set("Accept", "application/json")

	client := http.Client{Timeout: 5 * time.Second}
	res, err := client.Do(req)
	if err != nil {
		log.LogError(ctx, fmt.Sprintf("[OAuth-LinuxDO] GetUserInfo error: %s", err.Error()))
		return nil, NewOAuthErrorWithRaw(i18n.MsgOAuthConnectFailed, map[string]any{"Provider": "Linux DO"}, err.Error())
	}
	defer res.Body.Close()

	log.LogDebug(ctx, "[OAuth-LinuxDO] GetUserInfo response status: %d", res.StatusCode)

	var linuxdoUser linuxdoUser
	if err := jsonx.DecodeJson(res.Body, &linuxdoUser); err != nil {
		log.LogError(ctx, fmt.Sprintf("[OAuth-LinuxDO] GetUserInfo decode error: %s", err.Error()))
		return nil, err
	}

	if linuxdoUser.Id == 0 {
		log.LogError(ctx, "[OAuth-LinuxDO] GetUserInfo failed: invalid user id")
		return nil, NewOAuthError(i18n.MsgOAuthUserInfoEmpty, map[string]any{"Provider": "Linux DO"})
	}

	log.LogDebug(ctx, "[OAuth-LinuxDO] GetUserInfo: id=%d, username=%s, name=%s, trust_level=%d, active=%v, silenced=%v",
		linuxdoUser.Id, linuxdoUser.Username, linuxdoUser.Name, linuxdoUser.TrustLevel, linuxdoUser.Active, linuxdoUser.Silenced)

	// Check trust level
	if linuxdoUser.TrustLevel < common.LinuxDOMinimumTrustLevel {
		log.LogWarn(ctx, fmt.Sprintf("[OAuth-LinuxDO] GetUserInfo: trust level too low (required=%d, current=%d)",
			common.LinuxDOMinimumTrustLevel, linuxdoUser.TrustLevel))
		return nil, &TrustLevelError{
			Required: common.LinuxDOMinimumTrustLevel,
			Current:  linuxdoUser.TrustLevel,
		}
	}

	log.LogDebug(ctx, "[OAuth-LinuxDO] GetUserInfo success: id=%d, username=%s", linuxdoUser.Id, linuxdoUser.Username)

	return &OAuthUser{
		ProviderUserID: strconv.Itoa(linuxdoUser.Id),
		Username:       linuxdoUser.Username,
		DisplayName:    linuxdoUser.Name,
		Extra: map[string]any{
			"trust_level": linuxdoUser.TrustLevel,
			"active":      linuxdoUser.Active,
			"silenced":    linuxdoUser.Silenced,
		},
	}, nil
}

func (p *LinuxDOProvider) IsUserIDTaken(providerUserID string) bool {
	return userstore.IsLinuxDOIdAlreadyTaken(providerUserID)
}

func (p *LinuxDOProvider) FillUserByProviderID(user *userstore.User, providerUserID string) error {
	user.LinuxDOId = providerUserID
	return user.FillUserByLinuxDOId()
}

func (p *LinuxDOProvider) SetProviderUserID(user *userstore.User, providerUserID string) {
	user.LinuxDOId = providerUserID
}

// BindColumn returns the users table column storing the LinuxDO binding.
func (p *LinuxDOProvider) BindColumn() string {
	return "linux_do_id"
}

// TrustLevelError indicates the user's trust level is too low
type TrustLevelError struct {
	Required int
	Current  int
}

func (e *TrustLevelError) Error() string {
	return "trust level too low"
}
