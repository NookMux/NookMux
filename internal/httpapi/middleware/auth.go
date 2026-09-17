package middleware

import (
	"errors"
	"fmt"
	"github.com/NookMux/NookMux/internal/common"
	"github.com/NookMux/NookMux/internal/config/ratio"
	domaingroup "github.com/NookMux/NookMux/internal/domain/group"
	"github.com/NookMux/NookMux/internal/domain/shared"
	"github.com/NookMux/NookMux/internal/httpapi"
	"github.com/NookMux/NookMux/internal/infra/log"
	"github.com/NookMux/NookMux/internal/infra/security"
	"github.com/NookMux/NookMux/internal/store/db"
	"github.com/NookMux/NookMux/internal/store/token"
	"github.com/NookMux/NookMux/internal/store/user"
	"github.com/NookMux/NookMux/pkg/jsonx"
	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
	"net"
	"net/http"
	"strconv"
	"strings"
)

func validUserInfo(username string, role int) bool {
	// check username is empty
	if strings.TrimSpace(username) == "" {
		return false
	}
	if !common.IsValidateRole(role) {
		return false
	}
	return true
}

// clearAndReject clears the session (so the client's stale cookie is replaced
// with an empty one) and rejects the request with the given status/message.
// Used when a session is no longer trustworthy (e.g. user disabled, demoted
// or deleted by an admin).
func clearAndReject(c *gin.Context, session sessions.Session, status int, message string) {
	session.Clear()
	if err := session.Save(); err != nil {
		common.SysLog("failed to save cleared session: " + err.Error())
	}
	c.JSON(status, gin.H{
		"success": false,
		"message": message,
	})
	c.Abort()
}

func authHelper(c *gin.Context, minRole int) {
	session := sessions.Default(c)
	username := session.Get("username")
	var role interface{}
	id := session.Get("id")
	var status interface{}
	// group 用于写入上下文的最新分组。session/access-token 各分支会从权威来源覆盖。
	// 不直接复用 session.Get("group")，因为管理员改组后旧 cookie 快照会残留过期分组。
	var group string
	useAccessToken := false
	if username == nil {
		// Check access token
		accessToken := c.Request.Header.Get("Authorization")
		if accessToken == "" {
			c.JSON(http.StatusUnauthorized, gin.H{
				"success": false,
				"message": "无权进行此操作，未登录且未提供 access token",
			})
			c.Abort()
			return
		}
		user, authErr := userstore.ValidateAccessToken(accessToken)
		if authErr != nil {
			if errors.Is(authErr, dbstore.ErrDatabase) {
				common.SysLog("ValidateAccessToken database error: " + authErr.Error())
			}
			c.JSON(http.StatusOK, gin.H{
				"success": false,
				"message": "无权进行此操作，access token 无效",
			})
			c.Abort()
			return
		}
		if user != nil && user.Username != "" {
			if !validUserInfo(user.Username, user.Role) {
				c.JSON(http.StatusOK, gin.H{
					"success": false,
					"message": "无权进行此操作，用户信息无效",
				})
				c.Abort()
				return
			}
			// Token is valid
			username = user.Username
			role = user.Role
			id = user.Id
			status = user.Status
			group = user.Group
			useAccessToken = true
		} else {
			c.JSON(http.StatusOK, gin.H{
				"success": false,
				"message": "无权进行此操作，access token 无效",
			})
			c.Abort()
			return
		}
	} else {
		// Session-based auth: re-validate the user's role/status against the
		// latest DB state. The session snapshot is written once at login time
		// (setupLogin) and stored in a signed cookie, so admin actions like
		// disable/demote/delete would otherwise not take effect until the
		// cookie expires (up to 30 days). We query the DB directly rather than
		// GetUserCache because the Redis cache entry may have been written
		// before the Role field was added to UserBase (returning 0, which
		// collides with RoleGuestUser). See middleware/AGENTS.md.
		userId, ok := id.(int)
		if !ok || userId <= 0 {
			clearAndReject(c, session, http.StatusOK, "无权进行此操作，用户信息无效")
			return
		}
		latestUser, dbErr := userstore.GetUserById(userId, false)
		if dbErr != nil || latestUser == nil || latestUser.Id == 0 {
			// User likely deleted, or DB unavailable. Fail closed: drop the
			// stale session and force re-login.
			common.SysLog(fmt.Sprintf("authHelper session re-validation failed for user %d: %v", userId, dbErr))
			clearAndReject(c, session, http.StatusUnauthorized, "登录状态已失效，请重新登录")
			return
		}
		// Override the session snapshot with the authoritative values.
		role = latestUser.Role
		status = latestUser.Status
		// group 同步最新值，避免管理员改组后旧 cookie 携带过期分组，
		// 影响模型可用范围、倍率、动态规则、额度和业务权限。
		group = latestUser.Group
	}
	// get header New-Api-User
	apiUserIdStr := c.Request.Header.Get("New-Api-User")
	if apiUserIdStr == "" {
		c.JSON(http.StatusUnauthorized, gin.H{
			"success": false,
			"message": "无权进行此操作，未提供 New-Api-User",
		})
		c.Abort()
		return
	}
	apiUserId, err := strconv.Atoi(apiUserIdStr)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{
			"success": false,
			"message": "无权进行此操作，New-Api-User 格式错误",
		})
		c.Abort()
		return

	}
	if id != apiUserId {
		c.JSON(http.StatusUnauthorized, gin.H{
			"success": false,
			"message": "无权进行此操作，New-Api-User 与登录用户不匹配",
		})
		c.Abort()
		return
	}
	if status.(int) == common.UserStatusDisabled {
		// Session-based path: clear the stale cookie so the client re-logs in.
		if !useAccessToken {
			clearAndReject(c, session, http.StatusOK, "用户已被封禁")
			return
		}
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": "用户已被封禁",
		})
		c.Abort()
		return
	}
	if role.(int) < minRole {
		// Session-based path: clear the stale cookie so the client re-logs in
		// with the demoted role.
		if !useAccessToken {
			clearAndReject(c, session, http.StatusOK, "无权进行此操作，权限不足")
			return
		}
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": "无权进行此操作，权限不足",
		})
		c.Abort()
		return
	}
	if !validUserInfo(username.(string), role.(int)) {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": "无权进行此操作，用户信息无效",
		})
		c.Abort()
		return
	}
	c.Set("username", username)
	c.Set("role", role)
	c.Set("id", id)
	c.Set("group", group)
	c.Set("user_group", group)
	c.Set("use_access_token", useAccessToken)

	c.Next()
}

func TryUserAuth() func(c *gin.Context) {
	return func(c *gin.Context) {
		session := sessions.Default(c)
		id := session.Get("id")
		if id != nil {
			c.Set("id", id)
		}
		c.Next()
	}
}

// PricingAuth 根据模型广场的 requireAuth 配置动态决定认证策略：
// 如果开启了 requireAuth，则强制校验用户登录态（token/session 必须有效）；
// 如果未开启，则使用宽松认证（TryUserAuth），不强制登录。
func PricingAuth() func(c *gin.Context) {
	return func(c *gin.Context) {
		common.OptionMapRWMutex.RLock()
		headerNavModulesStr := common.OptionMap["HeaderNavModules"]
		common.OptionMapRWMutex.RUnlock()

		if headerNavModulesStr != "" {
			var modules struct {
				Pricing struct {
					RequireAuth bool `json:"requireAuth"`
				} `json:"pricing"`
			}
			if err := jsonx.Unmarshal([]byte(headerNavModulesStr), &modules); err == nil {
				if modules.Pricing.RequireAuth {
					authHelper(c, common.RoleCommonUser)
					return
				}
			}
		}
		// 未开启 requireAuth 或配置解析失败，使用宽松认证
		TryUserAuth()(c)
	}
}

func UserAuth() func(c *gin.Context) {
	return func(c *gin.Context) {
		authHelper(c, common.RoleCommonUser)
	}
}

func AdminAuth() func(c *gin.Context) {
	return func(c *gin.Context) {
		authHelper(c, common.RoleAdminUser)
	}
}

func RootAuth() func(c *gin.Context) {
	return func(c *gin.Context) {
		authHelper(c, common.RoleRootUser)
	}
}

func WssAuth(c *gin.Context) {

}

// TokenAuthReadOnly 宽松版本的令牌认证中间件，用于只读查询接口。
// 只验证令牌 key 是否存在，不检查令牌状态、过期时间、额度和 IP 限制。
// 即使令牌已过期、已耗尽或已禁用，也允许访问。
// 仍然检查用户是否被封禁。
func TokenAuthReadOnly() func(c *gin.Context) {
	return func(c *gin.Context) {
		key := c.Request.Header.Get("Authorization")
		if key == "" {
			c.JSON(http.StatusUnauthorized, gin.H{
				"success": false,
				"message": "未提供 Authorization 请求头",
			})
			c.Abort()
			return
		}
		if strings.HasPrefix(key, "Bearer ") || strings.HasPrefix(key, "bearer ") {
			key = strings.TrimSpace(key[7:])
		}
		key = strings.TrimPrefix(key, "sk-")
		parts := strings.Split(key, "-")
		key = parts[0]

		token, err := tokenstore.GetTokenByKey(key, false)
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{
				"success": false,
				"message": "无效的令牌",
			})
			c.Abort()
			return
		}

		userCache, err := userstore.GetUserCache(token.UserId)
		if err != nil {
			common.SysLog("TokenAuthReadOnly user cache error: " + err.Error())
			c.JSON(http.StatusInternalServerError, gin.H{
				"success": false,
				"message": "数据库错误，请稍后重试",
			})
			c.Abort()
			return
		}
		if userCache.Status != common.UserStatusEnabled {
			c.JSON(http.StatusForbidden, gin.H{
				"success": false,
				"message": "用户已被封禁",
			})
			c.Abort()
			return
		}

		c.Set("id", token.UserId)
		httpapi.SetContextKey(c, common.ContextKeyTokenId, token.Id)
		httpapi.SetContextKey(c, common.ContextKeyTokenKey, token.Key)
		httpapi.SetContextKey(c, common.ContextKeyTokenUnlimited, token.UnlimitedQuota)

		quotaType := token.QuotaType
		if quotaType == 0 && !token.UnlimitedQuota {
			quotaType = 1
		}
		httpapi.SetContextKey(c, common.ContextKeyTokenQuotaType, quotaType)

		c.Next()
	}
}

func TokenAuth() func(c *gin.Context) {
	return func(c *gin.Context) {
		// 先检测是否为ws
		if c.Request.Header.Get("Sec-WebSocket-Protocol") != "" {
			// Sec-WebSocket-Protocol: realtime, openai-insecure-api-key.sk-xxx, openai-beta.realtime-v1
			// read sk from Sec-WebSocket-Protocol
			key := c.Request.Header.Get("Sec-WebSocket-Protocol")
			parts := strings.Split(key, ",")
			for _, part := range parts {
				part = strings.TrimSpace(part)
				if strings.HasPrefix(part, "openai-insecure-api-key") {
					key = strings.TrimPrefix(part, "openai-insecure-api-key.")
					break
				}
			}
			c.Request.Header.Set("Authorization", "Bearer "+key)
		}
		// 检查path包含/v1/messages 或 /v1/models
		if strings.Contains(c.Request.URL.Path, "/v1/messages") || strings.Contains(c.Request.URL.Path, "/v1/models") {
			anthropicKey := c.Request.Header.Get("x-api-key")
			if anthropicKey != "" {
				c.Request.Header.Set("Authorization", "Bearer "+anthropicKey)
			}
		}
		// gemini api 从query中获取key
		if strings.HasPrefix(c.Request.URL.Path, "/v1beta/models") ||
			strings.HasPrefix(c.Request.URL.Path, "/v1beta/openai/models") ||
			strings.HasPrefix(c.Request.URL.Path, "/v1/models/") {
			skKey := c.Query("key")
			if skKey != "" {
				c.Request.Header.Set("Authorization", "Bearer "+skKey)
			}
			// 从x-goog-api-key header中获取key
			xGoogKey := c.Request.Header.Get("x-goog-api-key")
			if xGoogKey != "" {
				c.Request.Header.Set("Authorization", "Bearer "+xGoogKey)
			}
		}
		key := c.Request.Header.Get("Authorization")
		if strings.HasPrefix(key, "Bearer ") || strings.HasPrefix(key, "bearer ") {
			key = strings.TrimSpace(key[7:])
		}
		key = strings.TrimPrefix(key, "sk-")
		parts := strings.Split(key, "-")
		key = parts[0]
		token, err := tokenstore.ValidateUserToken(key)
		if token != nil {
			id := c.GetInt("id")
			if id == 0 {
				c.Set("id", token.UserId)
			}
		}
		if err != nil {
			if errors.Is(err, dbstore.ErrDatabase) {
				common.SysLog("ValidateUserToken database error: " + err.Error())
				abortWithOpenAiMessage(c, http.StatusInternalServerError, "数据库错误，请稍后重试")
			} else {
				abortWithOpenAiMessage(c, http.StatusUnauthorized, "无效的令牌")
			}
			return
		}

		allowIps := token.GetIpLimits()
		if len(allowIps) > 0 {
			clientIp := c.ClientIP()
			log.LogDebug(c, "Token has IP restrictions, checking client IP %s", clientIp)
			ip := net.ParseIP(clientIp)
			if ip == nil {
				abortWithOpenAiMessage(c, http.StatusForbidden, "无法解析客户端 IP 地址")
				return
			}
			if !security.IsIpInCIDRList(ip, allowIps) {
				abortWithOpenAiMessage(c, http.StatusForbidden, "您的 IP 不在令牌允许访问的列表中", shared.ErrorCodeAccessDenied)
				return
			}
			log.LogDebug(c, "Client IP %s passed the token IP restrictions check", clientIp)
		}

		userCache, err := userstore.GetUserCache(token.UserId)
		if err != nil {
			common.SysLog("TokenAuth user cache error: " + err.Error())
			abortWithOpenAiMessage(c, http.StatusInternalServerError, "数据库错误，请稍后重试")
			return
		}
		userEnabled := userCache.Status == common.UserStatusEnabled
		if !userEnabled {
			abortWithOpenAiMessage(c, http.StatusForbidden, "用户已被封禁")
			return
		}

		userCache.WriteContext(c)

		userGroup := userCache.Group
		tokenGroup := token.Group
		if tokenGroup != "" {
			// check common.UserUsableGroups[userGroup]
			if _, ok := domaingroup.GetUserUsableGroups(userGroup)[tokenGroup]; !ok {
				abortWithOpenAiMessage(c, http.StatusForbidden, fmt.Sprintf("无权访问 %s 分组", tokenGroup))
				return
			}
			// check group in common.GroupRatio
			if !ratio.ContainsGroupRatio(tokenGroup) {
				if tokenGroup != "auto" {
					abortWithOpenAiMessage(c, http.StatusForbidden, fmt.Sprintf("分组 %s 已被弃用", tokenGroup))
					return
				}
			}
			userGroup = tokenGroup
		}
		httpapi.SetContextKey(c, common.ContextKeyUsingGroup, userGroup)

		err = SetupContextForToken(c, token, parts...)
		if err != nil {
			return
		}
		c.Next()
	}
}

func SetupContextForToken(c *gin.Context, token *tokenstore.Token, parts ...string) error {
	if token == nil {
		return fmt.Errorf("token is nil")
	}
	c.Set("id", token.UserId)
	httpapi.SetContextKey(c, common.ContextKeyTokenId, token.Id)
	httpapi.SetContextKey(c, common.ContextKeyTokenKey, token.Key)
	c.Set("token_name", token.Name)
	httpapi.SetContextKey(c, common.ContextKeyTokenUnlimited, token.UnlimitedQuota)
	// 兼容旧数据：quota_type=0 && !unlimited_quota 应视为永久限额
	quotaType := token.QuotaType
	if quotaType == 0 && !token.UnlimitedQuota {
		quotaType = 1
	}
	httpapi.SetContextKey(c, common.ContextKeyTokenQuotaType, quotaType)
	if !token.UnlimitedQuota {
		// 将认证阶段读取到的额度快照写入上下文，后续预扣费无需再次查 token。
		switch quotaType {
		case 2:
			httpapi.SetContextKey(c, common.ContextKeyTokenQuota, token.WindowQuota-token.WindowUsedQuota)
		case 3:
			windowRemain := token.WindowQuota - token.WindowUsedQuota
			cycleRemain := token.CycleQuota - token.CycleUsedQuota
			if windowRemain < cycleRemain {
				httpapi.SetContextKey(c, common.ContextKeyTokenQuota, windowRemain)
			} else {
				httpapi.SetContextKey(c, common.ContextKeyTokenQuota, cycleRemain)
			}
		default:
			httpapi.SetContextKey(c, common.ContextKeyTokenQuota, token.RemainQuota)
		}
	}
	if token.ModelLimitsEnabled {
		c.Set("token_model_limit_enabled", true)
		c.Set("token_model_limit", token.GetModelLimitsMap())
	} else {
		c.Set("token_model_limit_enabled", false)
	}
	// 令牌级模型重定向规则，由 distributor 在选路前改写请求模型。
	// 非法 JSON（历史脏数据/手改库）解析为 nil，跳过映射保持透传。
	if mappingStr := token.GetModelMapping(); mappingStr != "" {
		if modelMap := token.GetModelMappingMap(); len(modelMap) > 0 {
			httpapi.SetContextKey(c, common.ContextKeyTokenModelMapping, modelMap)
		} else {
			common.SysError("invalid token model_mapping json, skip mapping: " + mappingStr)
		}
	}
	httpapi.SetContextKey(c, common.ContextKeyTokenGroup, token.Group)
	httpapi.SetContextKey(c, common.ContextKeyTokenCrossGroupRetry, token.CrossGroupRetry)
	if len(parts) > 1 {
		if userstore.IsAdmin(token.UserId) {
			c.Set("specific_channel_id", parts[1])
		} else {
			abortWithOpenAiMessage(c, http.StatusForbidden, "普通用户不支持指定渠道")
			return fmt.Errorf("普通用户不支持指定渠道")
		}
	}
	return nil
}
