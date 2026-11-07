package secureverificationcontroller

import (
	"fmt"
	"github.com/NookMux/NookMux/internal/common"
	"github.com/NookMux/NookMux/internal/config/system"
	"github.com/NookMux/NookMux/internal/httpapi"
	"github.com/NookMux/NookMux/internal/i18n"
	passkeysvc "github.com/NookMux/NookMux/internal/infra/passkey"
	"github.com/NookMux/NookMux/internal/infra/security"
	"github.com/NookMux/NookMux/internal/store/log"
	"github.com/NookMux/NookMux/internal/store/passkey"
	"github.com/NookMux/NookMux/internal/store/twofa"
	"github.com/NookMux/NookMux/internal/store/user"
	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
	"net/http"
	"time"
)

const (
	// SecureVerificationSessionKey means the user has fully passed secure verification.
	SecureVerificationSessionKey = "secure_verified_at"
	// SecureVerificationUserIDSessionKey 绑定安全验证通过的用户，避免验证态被其他身份复用
	SecureVerificationUserIDSessionKey = "secure_verified_user_id"
	SecureVerificationMethodSessionKey = "secure_verified_method"
	SecureVerificationMethod2FA        = "2fa"
	SecureVerificationMethodPasskey    = "passkey"
	SecureVerificationMethodPassword   = "password"
	// SecureVerificationScopeSessionKey 绑定验证用途：单次证明的作用域，
	// 消费时逐字匹配，防止一次验证跨操作重放。
	SecureVerificationScopeSessionKey = "secure_verified_scope"
	// PasskeyReadySessionKey means WebAuthn finished and /api/verify can finalize step-up verification.
	PasskeyReadySessionKey = "secure_passkey_ready_at"
	// SecureVerificationTimeout 验证有效期（秒）
	SecureVerificationTimeout = 300 // 5分钟
	// PasskeyReadyTimeout passkey ready 标记有效期（秒）
	PasskeyReadyTimeout = 60
)

// 敏感操作的安全验证作用域：验证请求携带、消费时逐字比对。
const (
	ScopeAccessToken     = "access_token"
	ScopeEmailBind       = "email_bind"
	ScopeAccountDelete   = "account_delete"
	ScopeAdminUserManage = "admin_user_manage"
	Scope2FASetup        = "2fa_setup"
	ScopeOAuthBind       = "oauth_bind"
)

type UniversalVerifyRequest struct {
	Method string `json:"method"` // "2fa"、"passkey" 或 "password"
	Code   string `json:"code,omitempty"`
	// Password 用于 method=password 的原密码再认证（无二因子账户的退路）。
	Password string `json:"password,omitempty"`
	// Scope 可选的验证用途作用域：敏感操作要求证明绑定具体动作。
	Scope string `json:"scope,omitempty"`
}

type VerificationStatusResponse struct {
	Verified  bool  `json:"verified"`
	ExpiresAt int64 `json:"expires_at,omitempty"`
}

// UniversalVerify 通用验证接口
// 支持 2FA 和 Passkey 验证，验证成功后在 session 中记录时间戳
func UniversalVerify(c *gin.Context) {
	userId := c.GetInt("id")
	if userId == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{
			"success": false,
			"message": i18n.T(c, i18n.MsgNotLoggedIn),
		})
		return
	}

	var req UniversalVerifyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpapi.ApiErrorI18n(c, i18n.MsgInvalidRequestBody)
		return
	}

	// 获取用户信息
	user := &userstore.User{Id: userId}
	if err := user.FillUserById(); err != nil {
		httpapi.ApiErrorI18n(c, i18n.MsgPasskeyUserInfoFailed)
		return
	}

	if user.Status != common.UserStatusEnabled {
		httpapi.ApiErrorI18n(c, i18n.MsgPasskeyUserDisabled)
		return
	}

	// 检查用户的验证方式
	twoFA, _ := twofastore.GetTwoFAByUserId(userId)
	has2FA := twoFA != nil && twoFA.IsEnabled

	passkey, passkeyErr := passkeystore.GetPasskeyByUserID(userId)
	hasPasskey := passkeyErr == nil && passkey != nil

	// 根据验证方式进行验证
	var verified bool
	var verifyMethod string
	var err error

	switch req.Method {
	case "2fa":
		if !has2FA {
			httpapi.ApiErrorI18n(c, i18n.MsgSecureVerifyTwoFANotEnabled)
			return
		}
		if req.Code == "" {
			httpapi.ApiErrorI18n(c, i18n.MsgSecureVerifyCodeRequired)
			return
		}
		verified = validateTwoFactorAuth(twoFA, req.Code)
		verifyMethod = "2FA"

	case "passkey":
		if !hasPasskey {
			httpapi.ApiErrorI18n(c, i18n.MsgSecureVerifyPasskeyNotEnabled)
			return
		}
		verified, err = consumePasskeyReady(c, userId)
		if err != nil {
			httpapi.ApiErrorI18n(c, i18n.MsgSecureVerifyInvalidPasskeyState)
			return
		}
		if !verified {
			httpapi.ApiErrorI18n(c, i18n.MsgSecureVerifyPasskeyRequired)
			return
		}
		verifyMethod = "Passkey"

	case "password":
		// 原密码再认证：无二因子账户完成敏感操作验证的退路。
		if user.Password == "" {
			httpapi.ApiErrorI18n(c, i18n.MsgSecureVerifyPasswordNotSet)
			return
		}
		if req.Password == "" {
			httpapi.ApiErrorI18n(c, i18n.MsgSecureVerifyCodeRequired)
			return
		}
		verified = security.ValidatePasswordAndHash(req.Password, user.Password)
		verifyMethod = "Password"

	default:
		httpapi.ApiErrorI18n(c, i18n.MsgSecureVerifyUnsupportedMethod, map[string]any{"Method": req.Method})
		return
	}

	if !verified {
		httpapi.ApiErrorI18n(c, i18n.MsgSecureVerifyFailed)
		return
	}

	// 验证成功，在 session 中记录时间戳、绑定当前用户与验证用途作用域
	now, err := setSecureVerificationSession(c, userId, req.Method, req.Scope)
	if err != nil {
		httpapi.ApiErrorI18n(c, i18n.MsgPasskeySaveVerifyFailed)
		return
	}

	// 记录日志
	userstore.RecordLog(userId, logstore.LogTypeSystem, fmt.Sprintf("通用安全验证成功 (验证方式: %s)", verifyMethod))

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": i18n.T(c, i18n.MsgSecureVerifySuccess),
		"data": gin.H{
			"verified":   true,
			"expires_at": now + SecureVerificationTimeout,
		},
	})
}

// PasskeyVerifyAndMarkReadySession writes a short-lived marker consumed by /api/verify.
func PasskeyVerifyAndMarkReadySession(c *gin.Context, userId int) (int64, error) {
	session := sessions.Default(c)
	now := time.Now().Unix()
	session.Set(PasskeyReadySessionKey, now)
	session.Set(SecureVerificationUserIDSessionKey, userId)
	session.Delete(SecureVerificationSessionKey)
	session.Delete(SecureVerificationMethodSessionKey)
	return now, session.Save()
}

func setSecureVerificationSession(c *gin.Context, userId int, method string, scope string) (int64, error) {
	session := sessions.Default(c)
	now := time.Now().Unix()
	session.Delete(PasskeyReadySessionKey)
	session.Set(SecureVerificationSessionKey, now)
	session.Set(SecureVerificationUserIDSessionKey, userId)
	session.Set(SecureVerificationMethodSessionKey, method)
	session.Set(SecureVerificationScopeSessionKey, scope)
	return now, session.Save()
}

// RequireScopeVerification 校验绑定指定作用域的安全验证：验证必须绑定当
// 前用户、未过期、作用域逐字匹配（可选 method 白名单进一步收紧验证方
// 式）。cookie 会话由客户端持有、服务端无法强制单次消费，证明在窗口期内
// 对本作用域有效（与上游语义一致）；失败时写入 403 响应并返回 false。
func RequireScopeVerification(c *gin.Context, scope string, allowedMethods ...string) bool {
	// 优先取中间件注入的用户 ID；OAuth 回调等未挂 UserAuth 的路径回退读会话。
	userId := c.GetInt("id")
	session := sessions.Default(c)
	if userId == 0 {
		if sid, ok := sessionInt(session.Get("id")); ok {
			userId = sid
		}
	}
	if userId == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{
			"success": false,
			"message": i18n.T(c, i18n.MsgNotLoggedIn),
		})
		return false
	}
	verifiedAtRaw := session.Get(SecureVerificationSessionKey)
	verifiedAt, ok := verifiedAtRaw.(int64)
	if verifiedAtRaw == nil || !ok {
		respondScopeVerificationError(c, i18n.MsgSecureVerifyRequired)
		return false
	}
	verifiedUserId, ok := sessionInt(session.Get(SecureVerificationUserIDSessionKey))
	if !ok || verifiedUserId != userId {
		ClearSecureVerification(c)
		respondScopeVerificationError(c, i18n.MsgSecureVerifyInvalid)
		return false
	}
	if time.Now().Unix()-verifiedAt >= SecureVerificationTimeout {
		ClearSecureVerification(c)
		respondScopeVerificationError(c, i18n.MsgSecureVerifyExpired)
		return false
	}
	if gotScope, _ := session.Get(SecureVerificationScopeSessionKey).(string); gotScope != scope {
		respondScopeVerificationError(c, i18n.MsgSecureVerifyRequired)
		return false
	}
	if len(allowedMethods) > 0 {
		method, _ := session.Get(SecureVerificationMethodSessionKey).(string)
		allowed := false
		for _, m := range allowedMethods {
			if method == m {
				allowed = true
				break
			}
		}
		if !allowed {
			ClearSecureVerification(c)
			respondScopeVerificationError(c, i18n.MsgSecureVerifyInvalid)
			return false
		}
	}
	// 验证在作用域内窗口期有效，不逐次消费。
	return true
}

func respondScopeVerificationError(c *gin.Context, msgKey string) {
	c.JSON(http.StatusForbidden, gin.H{
		"success": false,
		"message": i18n.T(c, msgKey),
		"code":    "VERIFICATION_REQUIRED",
	})
	c.Abort()
}

// ClearSecureVerification 清除安全验证态（含作用域），用于登出或验证失效。
func ClearSecureVerification(c *gin.Context) {
	session := sessions.Default(c)
	session.Delete(SecureVerificationSessionKey)
	session.Delete(SecureVerificationUserIDSessionKey)
	session.Delete(SecureVerificationMethodSessionKey)
	session.Delete(SecureVerificationScopeSessionKey)
	session.Delete(PasskeyReadySessionKey)
	_ = session.Save()
}

func sessionInt(raw interface{}) (int, bool) {
	switch value := raw.(type) {
	case int:
		return value, true
	case int32:
		return int(value), true
	case int64:
		return int(value), true
	default:
		return 0, false
	}
}

func hasSecureVerificationForUser(c *gin.Context, userId int) (bool, int64) {
	session := sessions.Default(c)
	verifiedAt, ok := session.Get(SecureVerificationSessionKey).(int64)
	if !ok {
		return false, 0
	}
	verifiedUserId, ok := sessionInt(session.Get(SecureVerificationUserIDSessionKey))
	if !ok || verifiedUserId != userId {
		return false, 0
	}
	if time.Now().Unix()-verifiedAt >= SecureVerificationTimeout {
		return false, 0
	}
	return true, verifiedAt
}

// HasSecureVerificationMethodForUser 报告指定用户是否已通过指定方式（2fa/passkey）的安全验证。
func HasSecureVerificationMethodForUser(c *gin.Context, userId int, method string) bool {
	ok, _ := hasSecureVerificationForUser(c, userId)
	if !ok {
		return false
	}
	session := sessions.Default(c)
	verifiedMethod, ok := session.Get(SecureVerificationMethodSessionKey).(string)
	return ok && verifiedMethod == method
}

// validateTwoFactorAuth 统一的2FA验证函数
func validateTwoFactorAuth(twoFA *twofastore.TwoFA, code string) bool {
	// 尝试验证TOTP
	if cleanCode, err := security.ValidateNumericCode(code); err == nil {
		if isValid, _ := twoFA.ValidateTOTPAndUpdateUsage(cleanCode); isValid {
			return true
		}
	}

	// 尝试验证备用码
	if isValid, err := twoFA.ValidateBackupCodeAndUpdateUsage(code); err == nil && isValid {
		return true
	}

	return false
}

func consumePasskeyReady(c *gin.Context, userId int) (bool, error) {
	session := sessions.Default(c)
	readyAtRaw := session.Get(PasskeyReadySessionKey)
	if readyAtRaw == nil {
		return false, nil
	}

	readyAt, ok := readyAtRaw.(int64)
	if !ok {
		session.Delete(PasskeyReadySessionKey)
		_ = session.Save()
		return false, fmt.Errorf("%s", i18n.T(c, i18n.MsgSecureVerifyInvalidPasskeyState))
	}

	verifiedUserId, ok := sessionInt(session.Get(SecureVerificationUserIDSessionKey))
	if !ok || verifiedUserId != userId {
		session.Delete(PasskeyReadySessionKey)
		_ = session.Save()
		return false, nil
	}

	session.Delete(PasskeyReadySessionKey)
	if err := session.Save(); err != nil {
		return false, err
	}
	if time.Now().Unix()-readyAt >= PasskeyReadyTimeout {
		return false, nil
	}
	return true, nil
}

// PasskeyVerifyForSecure 用于安全验证的 Passkey 验证流程
// 整合了 begin 和 finish 流程
func PasskeyVerifyForSecure(c *gin.Context) {
	if !system.GetPasskeySettings().Enabled {
		httpapi.ApiErrorI18n(c, i18n.MsgPasskeyLoginDisabled)
		return
	}

	userId := c.GetInt("id")
	if userId == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{
			"success": false,
			"message": i18n.T(c, i18n.MsgNotLoggedIn),
		})
		return
	}

	user := &userstore.User{Id: userId}
	if err := user.FillUserById(); err != nil {
		httpapi.ApiErrorI18n(c, i18n.MsgPasskeyUserInfoFailed)
		return
	}

	if user.Status != common.UserStatusEnabled {
		httpapi.ApiErrorI18n(c, i18n.MsgPasskeyUserDisabled)
		return
	}

	credential, err := passkeystore.GetPasskeyByUserID(userId)
	if err != nil {
		httpapi.ApiErrorI18n(c, i18n.MsgPasskeyNotBound)
		return
	}

	wa, err := passkeysvc.BuildWebAuthn(c.Request)
	if err != nil {
		httpapi.ApiErrorI18n(c, i18n.MsgSecureVerifyFailed)
		return
	}

	waUser := passkeysvc.NewWebAuthnUser(user, credential)
	sessionData, err := passkeysvc.PopSessionData(c, passkeysvc.VerifySessionKey)
	if err != nil {
		httpapi.ApiErrorI18n(c, i18n.MsgPasskeyInvalidSession)
		return
	}

	_, err = wa.FinishLogin(waUser, *sessionData, c.Request)
	if err != nil {
		httpapi.ApiErrorI18n(c, i18n.MsgSecureVerifyFailed)
		return
	}

	// 更新凭证的最后使用时间
	usedAt := time.Now()
	credential.LastUsedAt = &usedAt
	if err := passkeystore.UpsertPasskeyCredential(credential); err != nil {
		common.SysError("upsert passkey credential failed: " + err.Error())
		httpapi.ApiErrorI18n(c, i18n.MsgDatabaseError)
		return
	}

	// 验证成功，设置 session
	_, err = PasskeyVerifyAndMarkReadySession(c, userId)
	if err != nil {
		httpapi.ApiErrorI18n(c, i18n.MsgPasskeySaveVerifyFailed)
		return
	}

	// 记录日志
	userstore.RecordLog(userId, logstore.LogTypeSystem, "Passkey 安全验证成功")

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": i18n.T(c, i18n.MsgPasskeyVerifyOK),
	})
}
