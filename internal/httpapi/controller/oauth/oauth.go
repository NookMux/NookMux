package oauthcontroller

import (
	"fmt"
	"github.com/NookMux/NookMux/internal/common"
	"github.com/NookMux/NookMux/internal/httpapi"
	usercontroller "github.com/NookMux/NookMux/internal/httpapi/controller/user"
	"github.com/NookMux/NookMux/internal/i18n"
	"github.com/NookMux/NookMux/internal/oauth"
	"github.com/NookMux/NookMux/internal/store/db"
	"github.com/NookMux/NookMux/internal/store/user"
	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	"net/http"
	"strings"
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

	// 9. Setup login
	usercontroller.SetupLogin(user, c)
}

// handleOAuthBind handles binding OAuth account to existing user
func handleOAuthBind(c *gin.Context, provider oauth.Provider) {
	if !provider.IsEnabled() {
		httpapi.ApiErrorI18n(c, i18n.MsgOAuthNotEnabled, providerParams(provider.GetName()))
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

	// Check if this OAuth account is already bound (check both new ID and legacy ID)
	if provider.IsUserIDTaken(oauthUser.ProviderUserID) {
		httpapi.ApiErrorI18n(c, i18n.MsgOAuthAlreadyBound, providerParams(provider.GetName()))
		return
	}
	// Also check legacy ID to prevent duplicate bindings during migration period
	if legacyID, ok := oauthUser.Extra["legacy_id"].(string); ok && legacyID != "" {
		if provider.IsUserIDTaken(legacyID) {
			httpapi.ApiErrorI18n(c, i18n.MsgOAuthAlreadyBound, providerParams(provider.GetName()))
			return
		}
	}

	// Get current user from session
	session := sessions.Default(c)
	id := session.Get("id")
	user := userstore.User{Id: id.(int)}
	err = user.FillUserById()
	if err != nil {
		common.SysError("failed to fill user by id during oauth bind: " + err.Error())
		httpapi.ApiErrorI18n(c, i18n.MsgDatabaseError)
		return
	}
	// Re-check the authoritative status from DB; the session snapshot may be
	// stale if an admin disabled this user after login.
	if user.Status != common.UserStatusEnabled {
		session.Clear()
		_ = session.Save()
		httpapi.ApiErrorI18n(c, i18n.MsgOAuthUserBanned)
		return
	}

	provider.SetProviderUserID(&user, oauthUser.ProviderUserID)
	err = user.Update(false)
	if err != nil {
		common.SysError("failed to update user during oauth bind: " + err.Error())
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

	// Try to find user with legacy ID (for migration from an old provider ID to the
	// current numeric ID). Only a pure numeric legacy ID (a provider numeric ID) may
	// serve as a migration key: provider-side mutable identifiers (e.g. GitHub login,
	// which can be renamed or re-registered by others) must never authenticate a
	// local account, so non-numeric values fall through to the normal login path.
	if legacyID, ok := oauthUser.Extra["legacy_id"].(string); ok && isNumericProviderID(legacyID) {
		if provider.IsUserIDTaken(legacyID) {
			err := provider.FillUserByProviderID(user, legacyID)
			if err != nil {
				return nil, err
			}
			if user.Id != 0 {
				// Found user with legacy ID, migrate to new ID
				common.SysLog(fmt.Sprintf("[OAuth] Migrating user %d from legacy_id=%s to new_id=%s",
					user.Id, legacyID, oauthUser.ProviderUserID))
				if err := user.UpdateGitHubId(oauthUser.ProviderUserID); err != nil {
					common.SysError(fmt.Sprintf("[OAuth] Failed to migrate user %d: %s", user.Id, err.Error()))
					// Continue with login even if migration fails
				}
				return user, nil
			}
		}
	}

	// User doesn't exist, create new user if registration is enabled
	if !common.RegisterEnabled {
		return nil, &OAuthRegistrationDisabledError{}
	}

	// If provider returned an email, try to find exactly one existing user with that email
	// and merge by binding the OAuth provider ID to the existing account.
	// We only merge when the email is unique in the system to avoid binding to the wrong account.
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

// isNumericProviderID reports whether s is a non-empty decimal digit string,
// i.e. the textual form of a provider numeric ID.
func isNumericProviderID(s string) bool {
	if s == "" {
		return false
	}
	for i := 0; i < len(s); i++ {
		if s[i] < '0' || s[i] > '9' {
			return false
		}
	}
	return true
}

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
