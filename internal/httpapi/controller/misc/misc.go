package misccontroller

import (
	"encoding/json"
	"fmt"
	"github.com/NookMux/NookMux/internal/common"
	"github.com/NookMux/NookMux/internal/config"
	"github.com/NookMux/NookMux/internal/config/console"
	"github.com/NookMux/NookMux/internal/config/dashboard"
	"github.com/NookMux/NookMux/internal/config/operation"
	"github.com/NookMux/NookMux/internal/config/system"
	"github.com/NookMux/NookMux/internal/domain/shared"
	"github.com/NookMux/NookMux/internal/httpapi"
	"github.com/NookMux/NookMux/internal/httpapi/middleware"
	"github.com/NookMux/NookMux/internal/i18n"
	infraemail "github.com/NookMux/NookMux/internal/infra/email"
	"github.com/NookMux/NookMux/internal/infra/security"
	"github.com/NookMux/NookMux/internal/store/db"
	"github.com/NookMux/NookMux/internal/store/user"
	"github.com/NookMux/NookMux/pkg/jsonx"
	"github.com/gin-gonic/gin"
	"net/http"
	"strings"
)

func TestStatus(c *gin.Context) {
	err := dbstore.PingDB()
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"success": false,
			"message": i18n.T(c, i18n.MsgMiscDBConnectionFailed),
		})
		return
	}
	// 获取HTTP统计信息
	httpStats := middleware.GetStats()
	c.JSON(http.StatusOK, gin.H{
		"success":    true,
		"message":    i18n.T(c, i18n.MsgMiscServerRunning),
		"http_stats": httpStats,
	})
}

func GetStatus(c *gin.Context) {

	// 面板启用开关的单一权威源是 dashboard_config（#112）。
	// console_setting 仍用于读取内容数据（ApiInfo/Announcements/FAQ）。
	dc := dashboard.GetDashboardConfig()
	common.OptionMapRWMutex.RLock()
	defer common.OptionMapRWMutex.RUnlock()

	passkeySetting := system.GetPasskeySettings()
	legalSetting := system.GetLegalSettings()

	data := gin.H{
		"email_verification":          common.EmailVerificationEnabled,
		"github_oauth":                common.GitHubOAuthEnabled,
		"github_client_id":            common.GitHubClientId,
		"linuxdo_oauth":               common.LinuxDOOAuthEnabled,
		"linuxdo_client_id":           common.LinuxDOClientId,
		"linuxdo_minimum_trust_level": common.LinuxDOMinimumTrustLevel,
		"system_name":                 common.SystemName,
		"logo":                        common.Logo,
		"footer_html":                 common.Footer,
		"server_address":              system.ServerAddress,
		"turnstile_check":             common.TurnstileCheckEnabled,
		"turnstile_site_key":          common.TurnstileSiteKey,
		"docs_link":                   operation.GetGeneralSetting().DocsLink,
		"quota_per_unit":              common.QuotaPerUnit,
		"enable_batch_update":         common.BatchUpdateEnabled,
		"enable_data_export":          common.DataExportEnabled,
		"data_export_interval":        common.DataExportInterval,
		"data_export_default_time":    common.DataExportDefaultTime,
		"default_collapse_sidebar":    common.DefaultCollapseSidebar,
		"default_use_auto_group":      config.DefaultUseAutoGroup,

		"price":             operation.Price,
		"stripe_unit_price": config.StripeUnitPrice,

		// 面板启用开关（单一配置源：dashboard_config）
		"api_info_enabled":      dc.ApiInfoEnabled,
		"uptime_kuma_enabled":   dc.UptimeKumaEnabled,
		"announcements_enabled": dc.AnnouncementsEnabled,
		"faq_enabled":           dc.FAQEnabled,

		// 模块管理配置
		// HeaderNavModules 是前台导航开关（pricing/rankings 访问提示），登录前渲染壳层需要。
		// SidebarModulesAdmin 是管理后台侧栏模块开关，属于 admin 受限配置，
		// 由 GET /api/status/admin_modules（AdminAuth）单独返回，不得进公开响应。
		"HeaderNavModules":          common.OptionMap["HeaderNavModules"],
		"passkey_login":             passkeySetting.Enabled,
		"passkey_display_name":      passkeySetting.RPDisplayName,
		"passkey_rp_id":             passkeySetting.RPID,
		"passkey_origins":           passkeySetting.Origins,
		"passkey_allow_insecure":    passkeySetting.AllowInsecureOrigin,
		"passkey_user_verification": passkeySetting.UserVerification,
		"passkey_attachment":        passkeySetting.AttachmentPreference,
		"setup":                     shared.Setup,
		"user_agreement_enabled":    legalSetting.UserAgreement != "",
		"privacy_policy_enabled":    legalSetting.PrivacyPolicy != "",
		"checkin_enabled":           operation.GetCheckinSetting().Enabled,
		"_qn":                       "nookmux",
	}

	// 根据启用状态注入可选内容（开关读 dashboard_config，内容数据读 console_setting）
	if dc.ApiInfoEnabled {
		data["api_info"] = console.GetApiInfo()
	}
	if dc.AnnouncementsEnabled {
		data["announcements"] = console.GetAnnouncements()
	}
	if dc.FAQEnabled {
		data["faq"] = console.GetFAQ()
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    data,
	})
}

// GetStatusSystemInfo 返回构建版本号（AdminAuth）。
// 版本指纹属于管理语义，不得进匿名可达的 /api/status 公开响应，
// 由本接口在确认管理员身份后单独下发。
func GetStatusSystemInfo(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data": gin.H{
			"version": common.Version,
		},
	})
}

// GetStatusAdminModules 返回管理后台侧栏模块开关配置。
// 该配置属于 admin 受限数据（后台能力结构），不能由公开 /api/status 承载，
// 前端侧栏在确认管理员身份后请求本接口。
func GetStatusAdminModules(c *gin.Context) {
	common.OptionMapRWMutex.RLock()
	sidebarModulesAdmin := common.OptionMap["SidebarModulesAdmin"]
	common.OptionMapRWMutex.RUnlock()

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data": gin.H{
			"SidebarModulesAdmin": sidebarModulesAdmin,
		},
	})
}

// GetStatusUserModules 返回当前登录用户可见的侧栏模块开关配置（UserAuth）。
//
// 该配置同时约束普通用户侧栏（chat/console/personal/support 段）与路由守卫，
// 只下发给已登录调用者，不进公开 /api/status。管理段（admin section）描述
// 后台能力结构，非管理员调用者会被剥离，仅管理员可见。
func GetStatusUserModules(c *gin.Context) {
	common.OptionMapRWMutex.RLock()
	sidebarModulesAdmin := common.OptionMap["SidebarModulesAdmin"]
	common.OptionMapRWMutex.RUnlock()

	role := c.GetInt("role")
	if role >= common.RoleAdminUser {
		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"message": "",
			"data": gin.H{
				"SidebarModulesAdmin": sidebarModulesAdmin,
			},
		})
		return
	}

	// 非管理员：剥离 admin 段后再下发。原始值为空/非法 JSON 时直接透传，
	// 前端解析失败会回落默认配置（与旧行为一致）。
	stripped, ok := stripAdminSidebarSection(sidebarModulesAdmin)
	if !ok {
		stripped = sidebarModulesAdmin
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data": gin.H{
			"SidebarModulesAdmin": stripped,
		},
	})
}

// stripAdminSidebarSection 从 SidebarModulesAdmin JSON 中移除 admin 段。
// 返回 (结果JSON, 是否成功)；输入为空或非法 JSON 时返回 false。
func stripAdminSidebarSection(raw string) (string, bool) {
	if strings.TrimSpace(raw) == "" {
		return "", false
	}
	var config map[string]json.RawMessage
	if err := jsonx.Unmarshal([]byte(raw), &config); err != nil {
		return "", false
	}
	if _, exists := config["admin"]; !exists {
		// 没有 admin 段无需重序列化
		return raw, true
	}
	delete(config, "admin")
	out, err := jsonx.Marshal(config)
	if err != nil {
		return "", false
	}
	return string(out), true
}

func GetNotice(c *gin.Context) {
	common.OptionMapRWMutex.RLock()
	defer common.OptionMapRWMutex.RUnlock()
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    common.OptionMap["Notice"],
	})
}

func GetAbout(c *gin.Context) {
	common.OptionMapRWMutex.RLock()
	defer common.OptionMapRWMutex.RUnlock()
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    common.OptionMap["About"],
	})
}

func GetUserAgreement(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    system.GetLegalSettings().UserAgreement,
	})
}

func GetPrivacyPolicy(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    system.GetLegalSettings().PrivacyPolicy,
	})
}

func GetMidjourney(c *gin.Context) {
	common.OptionMapRWMutex.RLock()
	defer common.OptionMapRWMutex.RUnlock()
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    common.OptionMap["Midjourney"],
	})
}

func GetHomePageContent(c *gin.Context) {
	common.OptionMapRWMutex.RLock()
	defer common.OptionMapRWMutex.RUnlock()
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    common.OptionMap["HomePageContent"],
	})
}

func SendEmailVerification(c *gin.Context) {
	email := c.Query("email")
	if err := security.Validate.Var(email, "required,email"); err != nil {
		httpapi.ApiErrorI18n(c, i18n.MsgInvalidParams)
		return
	}
	parts := strings.Split(email, "@")
	if len(parts) != 2 {
		httpapi.ApiErrorI18n(c, i18n.MsgSettingEmailInvalid)
		return
	}
	localPart := parts[0]
	domainPart := parts[1]
	if common.EmailDomainRestrictionEnabled {
		allowed := false
		for _, domain := range common.EmailDomainWhitelist {
			if domainPart == domain {
				allowed = true
				break
			}
		}
		if !allowed {
			c.JSON(http.StatusOK, gin.H{
				"success": false,
				"message": i18n.T(c, i18n.MsgMiscEmailDomainWhitelistRejected),
			})
			return
		}
	}
	if common.EmailAliasRestrictionEnabled {
		containsSpecialSymbols := strings.Contains(localPart, "+") || strings.Contains(localPart, ".")
		if containsSpecialSymbols {
			httpapi.ApiErrorI18n(c, i18n.MsgMiscEmailAliasRejected)
			return
		}
	}

	if userstore.IsEmailAlreadyTaken(email) {
		httpapi.ApiErrorI18n(c, i18n.MsgMiscEmailTaken)
		return
	}
	code := security.GenerateVerificationCode(6)
	security.RegisterVerificationCodeWithKey(email, code, security.EmailVerificationPurpose)
	subject := fmt.Sprintf("%s邮箱验证邮件", common.SystemName)
	content := fmt.Sprintf("<p>您好，你正在进行%s邮箱验证。</p>"+
		"<p>您的验证码为: <strong>%s</strong></p>"+
		"<p>验证码 %d 分钟内有效，如果不是本人操作，请忽略。</p>", common.SystemName, code, security.VerificationValidMinutes)
	err := infraemail.SendEmail(subject, email, content)
	if err != nil {
		common.SysError(fmt.Sprintf("failed to send email verification to %s: %v", email, err))
		httpapi.ApiErrorI18n(c, i18n.MsgMiscEmailSendFailed)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
	})
}

func SendPasswordResetEmail(c *gin.Context) {
	email := c.Query("email")
	if err := security.Validate.Var(email, "required,email"); err != nil {
		httpapi.ApiErrorI18n(c, i18n.MsgInvalidParams)
		return
	}
	if userstore.IsEmailAlreadyTaken(email) {
		code := security.GenerateVerificationCode(0)
		security.RegisterVerificationCodeWithKey(email, code, security.PasswordResetPurpose)
		link := fmt.Sprintf("%s/user/reset?email=%s&token=%s", system.ServerAddress, email, code)
		subject := fmt.Sprintf("%s密码重置", common.SystemName)
		content := fmt.Sprintf("<p>您好，你正在进行%s密码重置。</p>"+
			"<p>点击 <a href='%s'>此处</a> 进行密码重置。</p>"+
			"<p>如果链接无法点击，请尝试点击下面的链接或将其复制到浏览器中打开：<br> %s </p>"+
			"<p>重置链接 %d 分钟内有效，如果不是本人操作，请忽略。</p>", common.SystemName, link, link, security.VerificationValidMinutes)
		if err := infraemail.SendEmail(subject, email, content); err != nil {
			common.SysError(fmt.Sprintf("failed to send password reset email to %s: %v", email, err))
		}
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
	})
}

type PasswordResetRequest struct {
	Email string `json:"email"`
	Token string `json:"token"`
	// NewPassword 由请求者自行设置，服务端校验强度后哈希入库，
	// 响应体不再返回任何凭据。
	NewPassword string `json:"new_password"`
}

func ResetPassword(c *gin.Context) {
	var req PasswordResetRequest
	err := jsonx.DecodeJson(c.Request.Body, &req)
	if err != nil || req.Email == "" || req.Token == "" || req.NewPassword == "" {
		httpapi.ApiErrorI18n(c, i18n.MsgInvalidParams)
		return
	}
	if !security.VerifyCodeWithKey(req.Email, req.Token, security.PasswordResetPurpose) {
		httpapi.ApiErrorI18n(c, i18n.MsgMiscPasswordResetLinkInvalid)
		return
	}
	// 复用注册/更新用户时 userstore.User Password 字段的长度约束。
	if err := security.Validate.Var(req.NewPassword, "min=8,max=20"); err != nil {
		httpapi.ApiErrorI18n(c, i18n.MsgMiscPasswordInvalid)
		return
	}
	if err := userstore.ResetUserPasswordByEmail(req.Email, req.NewPassword); err != nil {
		common.SysError("reset user password by email failed: " + err.Error())
		httpapi.ApiErrorI18n(c, i18n.MsgDatabaseError)
		return
	}
	security.DeleteKey(req.Email, security.PasswordResetPurpose)
	httpapi.ApiSuccessI18n(c, i18n.MsgMiscPasswordResetSuccess, nil)
}

// GetUsageLogFieldsVisible 公开接口：返回当前用户角色下使用日志详情弹窗的字段可见性配置。
// 普通用户和管理员都可访问，区别在于 isAdmin 由中间件注入。
func GetUsageLogFieldsVisible(c *gin.Context) {
	role := c.GetInt("role")
	isAdmin := role >= common.RoleAdminUser

	// 总开关关闭时，返回空字段列表，前端据此隐藏详情按钮
	if !console.IsUsageLogDetailsEnabled(isAdmin) {
		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"message": "",
			"data": gin.H{
				"enabled": false,
				"fields":  []string{},
			},
		})
		return
	}

	fieldsMap, err := console.GetUsageLogFieldsVisible()
	if err != nil {
		common.SysError("failed to parse usage_log_fields setting: " + err.Error())
		httpapi.ApiErrorI18n(c, i18n.MsgMiscUsageLogFieldsParseFailed)
		return
	}

	// 返回当前角色可见的字段 key 列表
	visibleFields := make([]string, 0, len(fieldsMap))
	for key, cfg := range fieldsMap {
		if isAdmin && cfg.Admin {
			visibleFields = append(visibleFields, key)
		} else if !isAdmin && cfg.User {
			visibleFields = append(visibleFields, key)
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data": gin.H{
			"enabled": true,
			"fields":  visibleFields,
		},
	})
}
