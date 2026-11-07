package oauthcontroller

import (
	"fmt"
	"github.com/NookMux/NookMux/internal/common"
	"github.com/NookMux/NookMux/internal/httpapi"
	secureverificationcontroller "github.com/NookMux/NookMux/internal/httpapi/controller/secure_verification"
	usercontroller "github.com/NookMux/NookMux/internal/httpapi/controller/user"
	"github.com/NookMux/NookMux/internal/i18n"
	"github.com/NookMux/NookMux/internal/oauth"
	"github.com/NookMux/NookMux/internal/store/db"
	"github.com/NookMux/NookMux/internal/store/twofa"
	"github.com/NookMux/NookMux/internal/store/user"
	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	"net/http"
	"strings"
	"time"
)

// providerParams returns map with Provider key for i18n templates
func providerParams(name string) map[string]any {
	return map[string]any{"Provider": name}
}

// GenerateOAuthCode generates a state code for OAuth CSRF protection
func GenerateOAuthCode(c *gin.Context) {
	session := sessions.Default(c)
	state := common.GetRandomString(12)
	affCode := c.Query("aff")
	if affCode != "" {
		session.Set("aff", affCode)
	}
	session.Set("oauth_state", state)
	err := session.Save()
	if err != nil {
		common.SysError("failed to save oauth session: " + err.Error())
		httpapi.ApiErrorI18n(c, i18n.MsgDatabaseError)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    state,
	})
}

// HandleOAuth handles OAuth callback for all standard OAuth providers
func HandleOAuth(c *gin.Context) {
	providerName := c.Param("provider")
	provider := oauth.GetProvider(providerName)
	if provider == nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": i18n.T(c, i18n.MsgOAuthUnknownProvider),
		})
		return
	}

	session := sessions.Default(c)

	// 1. Validate state (CSRF protection)
	state := c.Query("state")
	if state == "" || session.Get("oauth_state") == nil || state != session.Get("oauth_state").(string) {
		c.JSON(http.StatusForbidden, gin.H{
			"success": false,
			"message": i18n.T(c, i18n.MsgOAuthStateInvalid),
		})
		return
	}

	// 2. Check if user is already logged in (bind flow)
	username := session.Get("username")
	if username != nil {
		handleOAuthBind(c, provider)
		return
	}

	// 3. Check if provider is enabled
	if !provider.IsEnabled() {
		httpapi.ApiErrorI18n(c, i18n.MsgOAuthNotEnabled, providerParams(provider.GetName()))
		return
	}

	// 4. Handle error from provider
	errorCode := c.Query("error")
	if errorCode != "" {
		errorDescription := c.Query("error_description")
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": errorDescription,
		})
		return
	}

	// 5. Exchange code for token
	code := c.Query("code")
	token, err := provider.ExchangeToken(c.Request.Context(), code, c)
	if err != nil {
		handleOAuthError(c, err)
		return
	}

	// 6. Get user info
	oauthUser, err := provider.GetUserInfo(c.Request.Context(), token)
	if err != nil {
		handleOAuthError(c, err)
		return
	}

	// 7. Find or create user
	user, err := findOrCreateOAuthUser(c, provider, oauthUser, session)
	if err != nil {
		switch err.(type) {
		case *OAuthUserDeletedError:
			httpapi.ApiErrorI18n(c, i18n.MsgOAuthUserDeleted)
		case *OAuthRegistrationDisabledError:
			httpapi.ApiErrorI18n(c, i18n.MsgUserRegisterDisabled)
		default:
			common.SysError("failed to find or create oauth user: " + err.Error())
			httpapi.ApiErrorI18n(c, i18n.MsgUserRegisterFailed)
		}
		return
	}

	// 8. Check user status
	if user.Status != common.UserStatusEnabled {
		httpapi.ApiErrorI18n(c, i18n.MsgOAuthUserBanned)
		return
	}

	// 8.5 已启用 2FA 的账户在 OAuth 传输路径上同样受因子门控：仅凭第三方
	// 会话（GitHub 账号被盗/邮箱合并自动绑定）登录不得绕过 TOTP，与密码
	// 登录路径保持一致，由既有 POST /api/user/login/2fa 完成验证。
	twoFAEnabled, err := twofastore.IsTwoFAEnabled(user.Id)
	if err != nil {
		common.SysLog(fmt.Sprintf("OAuth login 2FA status error for user %d: %v", user.Id, err))
		httpapi.ApiErrorI18n(c, i18n.MsgUserLoginUnavailable)
		return
	}
	if twoFAEnabled {
		session := sessions.Default(c)
		session.Set("pending_username", user.Username)
		session.Set("pending_user_id", user.Id)
		session.Set("pending_2fa_set_at", time.Now().Unix())
		if err := session.Save(); err != nil {
			httpapi.ApiErrorI18n(c, i18n.MsgUserSessionSaveFailed)
			return
		}
		c.JSON(http.StatusOK, gin.H{
			"message": i18n.T(c, i18n.MsgUserRequire2FA),
			"success": true,
			"data": map[string]interface{}{
				"require_2fa": true,
			},
		})
		return
	}

	// 9. Setup login
	usercontroller.SetupLogin(user, c)
}

// handleOAuthBind handles binding OAuth account to existing user
func handleOAuthBind(c *gin.Context, provider oauth.Provider) {
	if !provider.IsEnabled() {
		httpapi.ApiErrorI18n(c, i18n.MsgOAuthNotEnabled, providerParams(provider.GetName()))
		return
	}

	// 绑定属登录凭据变更：要求绑定发起会话先通过单次安全验证
	// （scope=oauth_bind），被窃取的会话没有第二因子/原密码无法植入
	// 持久后门。验证态存于会话 cookie，跨 OAuth 重定向往返保持有效。
	if !secureverificationcontroller.RequireScopeVerification(c, secureverificationcontroller.ScopeOAuthBind) {
		return
	}

	// Exchange code for token
	code := c.Query("code")
	token, err := provider.ExchangeToken(c.Request.Context(), code, c)
	if err != nil {
		handleOAuthError(c, err)
		return
	}

	// Get user info
	oauthUser, err := provider.GetUserInfo(c.Request.Context(), token)
	if err != nil {
		handleOAuthError(c, err)
		return
	}

	// Check if this OAuth account is already bound
	if provider.IsUserIDTaken(oauthUser.ProviderUserID) {
		httpapi.ApiErrorI18n(c, i18n.MsgOAuthAlreadyBound, providerParams(provider.GetName()))
		return
	}

	// Get current user from session; only the status column is needed for the
	// ban re-check, so the read must not carry a full snapshot into the write.
	session := sessions.Default(c)
	id, ok := session.Get("id").(int)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{
			"success": false,
			"message": i18n.T(c, i18n.MsgNotLoggedIn),
		})
		return
	}
	var statusCheck userstore.User
	if err := dbstore.DB.Select("id", "status").Where("id = ?", id).First(&statusCheck).Error; err != nil {
		common.SysError("failed to load user status during oauth bind: " + err.Error())
		httpapi.ApiErrorI18n(c, i18n.MsgDatabaseError)
		return
	}
	// Re-check the authoritative status from DB; the session snapshot may be
	// stale if an admin disabled this user after login.
	if statusCheck.Status != common.UserStatusEnabled {
		session.Clear()
		_ = session.Save()
		httpapi.ApiErrorI18n(c, i18n.MsgOAuthUserBanned)
		return
	}

	// 只写绑定列：并发的封禁/降权/配额变更不可能被旧快照回滚。
	if err := userstore.UpdateUserBindColumn(id, provider.BindColumn(), oauthUser.ProviderUserID); err != nil {
		common.SysError("failed to update user oauth binding: " + err.Error())
		httpapi.ApiErrorI18n(c, i18n.MsgDatabaseError)
		return
	}

	httpapi.ApiSuccessI18n(c, i18n.MsgOAuthBindSuccess, gin.H{
		"action": "bind",
	})
}

// findOrCreateOAuthUser finds existing user or creates new user
func findOrCreateOAuthUser(c *gin.Context, provider oauth.Provider, oauthUser *oauth.OAuthUser, session sessions.Session) (*userstore.User, error) {
	user := &userstore.User{}

	// Check if user already exists with new ID
	if provider.IsUserIDTaken(oauthUser.ProviderUserID) {
		err := provider.FillUserByProviderID(user, oauthUser.ProviderUserID)
		if err != nil {
			return nil, err
		}
		// Check if user has been deleted
		if user.Id == 0 {
			return nil, &OAuthUserDeletedError{}
		}
		return user, nil
	}

	// User doesn't exist, create new user if registration is enabled
	if !common.RegisterEnabled {
		return nil, &OAuthRegistrationDisabledError{}
	}

	// If provider returned an email, try to find exactly one existing user with that email
	// and merge by binding the OAuth provider ID to the existing account.
	// We only merge when the email is unique in the system to avoid binding to the wrong account.
	if oauthUser.Email != "" {
		oauthUser.Email = userstore.NormalizeEmail(oauthUser.Email)
	}
	if oauthUser.Email != "" && userstore.IsEmailAlreadyTaken(oauthUser.Email) {
		existingUser := &userstore.User{}
		existingUser.Email = oauthUser.Email
		if err := existingUser.FillUserByEmail(); err == nil && existingUser.Id != 0 {
			// Verify this email belongs to exactly one active user
			var count int64
			dbstore.DB.Unscoped().Model(&userstore.User{}).Where("email = ?", oauthUser.Email).Count(&count)
			if count != 1 {
				// Multiple users share this email — skip merge, create new user instead
				common.SysLog(fmt.Sprintf("[OAuth] Skipping email merge for %s: %d users share this email",
					oauthUser.Email, count))
			} else {
				if existingUser.Status != common.UserStatusEnabled {
					return nil, &OAuthUserDeletedError{}
				}
				// Check if this provider is already bound to a different user
				if provider.IsUserIDTaken(oauthUser.ProviderUserID) {
					common.SysLog(fmt.Sprintf("[OAuth] Skipping email merge: provider ID %s already bound to another user",
						oauthUser.ProviderUserID))
				} else {
					// Bind OAuth to the existing account
					provider.SetProviderUserID(existingUser, oauthUser.ProviderUserID)
					if err := dbstore.DB.Model(existingUser).Updates(map[string]interface{}{
						"github_id":   existingUser.GitHubId,
						"linux_do_id": existingUser.LinuxDOId,
					}).Error; err != nil {
						return nil, err
					}
					common.SysLog(fmt.Sprintf("[OAuth] Merged OAuth account (provider=%s, provider_uid=%s) into existing user %d by email match",
						provider.GetName(), oauthUser.ProviderUserID, existingUser.Id))
					return existingUser, nil
				}
			}
		}
	}

	// Build username from provider username + provider user ID, truncated to 20 chars
	user.Username = buildOAuthUsername(oauthUser.Username, oauthUser.ProviderUserID)
	if oauthUser.DisplayName != "" {
		user.DisplayName = oauthUser.DisplayName
	} else if oauthUser.Username != "" {
		user.DisplayName = oauthUser.Username
	} else {
		user.DisplayName = provider.GetName() + " User"
	}
	if oauthUser.Email != "" {
		user.Email = oauthUser.Email
	}
	user.Role = common.RoleCommonUser
	user.Status = common.UserStatusEnabled

	// Handle affiliate code
	affCode := session.Get("aff")
	inviterId := 0
	if affCode != nil {
		inviterId, _ = userstore.GetUserIdByAffCode(affCode.(string))
	}

	err := dbstore.DB.Transaction(func(tx *gorm.DB) error {
		if err := user.InsertWithTx(tx, inviterId); err != nil {
			return err
		}

		provider.SetProviderUserID(user, oauthUser.ProviderUserID)
		if err := tx.Model(user).Updates(map[string]interface{}{
			"github_id":   user.GitHubId,
			"linux_do_id": user.LinuxDOId,
		}).Error; err != nil {
			return err
		}

		return nil
	})
	if err != nil {
		return nil, err
	}

	user.FinalizeOAuthUserCreation(inviterId)

	return user, nil
}

// Error types for OAuth
type OAuthUserDeletedError struct{}

func (e *OAuthUserDeletedError) Error() string {
	return "user has been deleted"
}

type OAuthRegistrationDisabledError struct{}

func (e *OAuthRegistrationDisabledError) Error() string {
	return "registration is disabled"
}

// handleOAuthError handles OAuth errors and returns translated message
func handleOAuthError(c *gin.Context, err error) {
	switch e := err.(type) {
	case *oauth.OAuthError:
		if e.Params != nil {
			httpapi.ApiErrorI18n(c, e.MsgKey, e.Params)
		} else {
			httpapi.ApiErrorI18n(c, e.MsgKey)
		}
	case *oauth.TrustLevelError:
		httpapi.ApiErrorI18n(c, i18n.MsgOAuthTrustLevelLow)
	default:
		common.SysError("unhandled oauth error: " + err.Error())
		httpapi.ApiErrorI18n(c, i18n.MsgDatabaseError)
	}
}

const maxUsernameLen = 20

// buildOAuthUsername creates a unique username from the provider username and provider user ID.
// Format: "providerUsername_providerUserID". When truncation is needed, the providerUserID
// suffix is preserved because it is the source of uniqueness.
func buildOAuthUsername(providerUsername string, providerUserID string) string {
	// Use "user" as fallback when provider username is empty
	if providerUsername == "" {
		providerUsername = "user"
	}
	// Only keep ASCII alphanumeric and underscore from provider username
	cleaned := strings.Map(func(r rune) rune {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '_' {
			return r
		}
		return '_'
	}, providerUsername)

	separator := "_"
	suffix := separator + providerUserID
	suffixLen := len(suffix)

	if suffixLen >= maxUsernameLen {
		// Provider ID alone exceeds max length — use it directly (still unique)
		return suffix[1:suffixLen] // trim leading underscore
	}

	available := maxUsernameLen - suffixLen
	if len(cleaned) > available {
		cleaned = cleaned[:available]
	}

	return cleaned + suffix
}
