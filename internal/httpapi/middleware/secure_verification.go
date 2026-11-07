package middleware

import (
	"net/http"
	"time"

	"github.com/NookMux/NookMux/internal/i18n"
	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
)

const (
	// SecureVerificationSessionKey 安全验证的 session key（与 controller 保持一致）
	SecureVerificationSessionKey = "secure_verified_at"
	// SecureVerificationUserIDSessionKey 绑定安全验证通过的用户（与 controller 保持一致）
	SecureVerificationUserIDSessionKey = "secure_verified_user_id"
	secureVerificationMethodSessionKey = "secure_verified_method"
	secureVerificationScopeSessionKey  = "secure_verified_scope"
	passkeyReadySessionKey             = "secure_passkey_ready_at"
	// SecureVerificationTimeout 验证有效期（秒）
	SecureVerificationTimeout = 300 // 5分钟
)

// SecureVerificationRequired 安全验证中间件：校验用户在有效窗口内通过安
// 全验证（证明绑定用户与作用域；cookie 会话由客户端持有，服务端无法强制
// 单次消费，窗口期语义与上游一致，过期或换作用域即失效）。
func SecureVerificationRequired() gin.HandlerFunc {
	return secureVerificationRequiredFor("")
}

// SecureVerificationRequiredScope 作用域安全验证中间件：验证必须以相同
// 作用域获取（敏感操作证明跨作用域不可重放），窗口期内对本作用域有效。
func SecureVerificationRequiredScope(scope string) gin.HandlerFunc {
	return secureVerificationRequiredFor(scope)
}

func secureVerificationRequiredFor(expectedScope string) gin.HandlerFunc {
	return func(c *gin.Context) {
		// 检查用户是否已登录
		userId := c.GetInt("id")
		if userId == 0 {
			c.JSON(http.StatusUnauthorized, gin.H{
				"success": false,
				"message": i18n.T(c, i18n.MsgNotLoggedIn),
			})
			c.Abort()
			return
		}

		session := sessions.Default(c)
		verifiedAtRaw := session.Get(SecureVerificationSessionKey)

		if verifiedAtRaw == nil {
			c.JSON(http.StatusForbidden, gin.H{
				"success": false,
				"message": i18n.T(c, i18n.MsgSecureVerifyRequired),
				"code":    "VERIFICATION_REQUIRED",
			})
			c.Abort()
			return
		}

		verifiedAt, ok := verifiedAtRaw.(int64)
		if !ok {
			clearSecureVerificationSession(session)
			c.JSON(http.StatusForbidden, gin.H{
				"success": false,
				"message": i18n.T(c, i18n.MsgSecureVerifyInvalid),
				"code":    "VERIFICATION_INVALID",
			})
			c.Abort()
			return
		}

		verifiedUserId, ok := sessionInt(session.Get(SecureVerificationUserIDSessionKey))
		if !ok || verifiedUserId != userId {
			clearSecureVerificationSession(session)
			c.JSON(http.StatusForbidden, gin.H{
				"success": false,
				"message": i18n.T(c, i18n.MsgSecureVerifyInvalid),
				"code":    "VERIFICATION_INVALID",
			})
			c.Abort()
			return
		}

		// 检查验证是否过期
		elapsed := time.Now().Unix() - verifiedAt
		if elapsed >= SecureVerificationTimeout {
			clearSecureVerificationSession(session)
			c.JSON(http.StatusForbidden, gin.H{
				"success": false,
				"message": i18n.T(c, i18n.MsgSecureVerifyExpired),
				"code":    "VERIFICATION_EXPIRED",
			})
			c.Abort()
			return
		}

		// 作用域逐字匹配：为其它动作获取的证明不能用于本路由
		if gotScope, _ := session.Get(secureVerificationScopeSessionKey).(string); gotScope != expectedScope {
			c.JSON(http.StatusForbidden, gin.H{
				"success": false,
				"message": i18n.T(c, i18n.MsgSecureVerifyRequired),
				"code":    "VERIFICATION_REQUIRED",
			})
			c.Abort()
			return
		}

		// 验证在作用域内窗口期有效；过期由上方时效检查拒绝。

		// 验证有效，继续处理请求
		c.Next()
	}
}

// OptionalSecureVerification 可选的安全验证中间件
// 如果用户已验证，则在 context 中设置标记，但不阻止请求继续
// 用于某些需要区分是否已验证的场景
func OptionalSecureVerification() gin.HandlerFunc {
	return func(c *gin.Context) {
		userId := c.GetInt("id")
		if userId == 0 {
			c.Set("secure_verified", false)
			c.Next()
			return
		}

		session := sessions.Default(c)
		verifiedAtRaw := session.Get(SecureVerificationSessionKey)

		if verifiedAtRaw == nil {
			c.Set("secure_verified", false)
			c.Next()
			return
		}

		verifiedAt, ok := verifiedAtRaw.(int64)
		if !ok {
			clearSecureVerificationSession(session)
			c.Set("secure_verified", false)
			c.Next()
			return
		}

		verifiedUserId, ok := sessionInt(session.Get(SecureVerificationUserIDSessionKey))
		if !ok || verifiedUserId != userId {
			clearSecureVerificationSession(session)
			c.Set("secure_verified", false)
			c.Next()
			return
		}

		elapsed := time.Now().Unix() - verifiedAt
		if elapsed >= SecureVerificationTimeout {
			clearSecureVerificationSession(session)
			c.Set("secure_verified", false)
			c.Next()
			return
		}

		c.Set("secure_verified", true)
		c.Set("secure_verified_at", verifiedAt)
		c.Next()
	}
}

// ClearSecureVerification 清除安全验证状态
// 用于用户登出或需要强制重新验证的场景
func ClearSecureVerification(c *gin.Context) {
	session := sessions.Default(c)
	clearSecureVerificationSession(session)
}

func clearSecureVerificationSession(session sessions.Session) {
	session.Delete(SecureVerificationSessionKey)
	session.Delete(SecureVerificationUserIDSessionKey)
	session.Delete(secureVerificationMethodSessionKey)
	session.Delete(secureVerificationScopeSessionKey)
	session.Delete(passkeyReadySessionKey)
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
