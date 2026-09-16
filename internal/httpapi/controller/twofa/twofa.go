package twofacontroller

import (
	"errors"
	"github.com/NookMux/NookMux/internal/common"
	audit "github.com/NookMux/NookMux/internal/domain/audit"
	"github.com/NookMux/NookMux/internal/httpapi"
	secureverificationcontroller "github.com/NookMux/NookMux/internal/httpapi/controller/secure_verification"
	usercontroller "github.com/NookMux/NookMux/internal/httpapi/controller/user"
	"github.com/NookMux/NookMux/internal/i18n"
	"github.com/NookMux/NookMux/internal/infra/security"
	"github.com/NookMux/NookMux/internal/store/audit"
	"github.com/NookMux/NookMux/internal/store/log"
	"github.com/NookMux/NookMux/internal/store/twofa"
	"github.com/NookMux/NookMux/internal/store/user"
	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
	"net/http"
	"strconv"
	"time"
)

// Setup2FARequest 设置2FA请求结构
type Setup2FARequest struct {
	Code string `json:"code" binding:"required"`
}

// Verify2FARequest 验证2FA请求结构
type Verify2FARequest struct {
	Code string `json:"code" binding:"required"`
}

// Setup2FAResponse 设置2FA响应结构
type Setup2FAResponse struct {
	Secret      string   `json:"secret"`
	QRCodeData  string   `json:"qr_code_data"`
	BackupCodes []string `json:"backup_codes"`
}

// Setup2FA 初始化2FA设置
func Setup2FA(c *gin.Context) {
	userId := c.GetInt("id")

	// 检查用户是否已经启用2FA
	existing, err := twofastore.GetTwoFAByUserId(userId)
	if err != nil {
		common.SysError("get twofa by user id failed: " + err.Error())
		httpapi.ApiErrorI18n(c, i18n.MsgDatabaseError)
		return
	}
	if existing != nil && existing.IsEnabled {
		httpapi.ApiErrorI18n(c, i18n.MsgTwoFAAlreadyEnabledReset)
		return
	}

	// 如果存在已禁用的2FA记录，先删除它
	if existing != nil && !existing.IsEnabled {
		if err := existing.Delete(); err != nil {
			common.SysError("delete disabled twofa failed: " + err.Error())
			httpapi.ApiErrorI18n(c, i18n.MsgDatabaseError)
			return
		}
		existing = nil // 重置为nil，后续将创建新记录
	}

	// 获取用户信息
	user, err := userstore.GetUserById(userId, false)
	if err != nil {
		common.SysError("get user by id failed: " + err.Error())
		httpapi.ApiErrorI18n(c, i18n.MsgDatabaseError)
		return
	}

	// 生成TOTP密钥
	key, err := security.GenerateTOTPSecret(user.Username)
	if err != nil {
		httpapi.ApiErrorI18n(c, i18n.MsgTwoFASecretGenerateFailed)
		common.SysLog("生成TOTP密钥失败: " + err.Error())
		return
	}

	// 生成备用码
	backupCodes, err := security.GenerateBackupCodes()
	if err != nil {
		httpapi.ApiErrorI18n(c, i18n.MsgTwoFABackupGenerateFailed)
		common.SysLog("生成备用码失败: " + err.Error())
		return
	}

	// 生成二维码数据
	qrCodeData := security.GenerateQRCodeData(key.Secret(), user.Username)

	// 创建或更新2FA记录（暂未启用）
	twoFA := &twofastore.TwoFA{
		UserId:    userId,
		Secret:    key.Secret(),
		IsEnabled: false,
	}

	if existing != nil {
		// 更新现有记录
		twoFA.Id = existing.Id
		err = twoFA.Update()
	} else {
		// 创建新记录
		err = twoFA.Create()
	}

	if err != nil {
		common.SysError("create or update twofa failed: " + err.Error())
		httpapi.ApiErrorI18n(c, i18n.MsgDatabaseError)
		return
	}

	// 创建备用码记录
	if err := twofastore.CreateBackupCodes(userId, backupCodes); err != nil {
		httpapi.ApiErrorI18n(c, i18n.MsgTwoFABackupSaveFailed)
		common.SysLog("保存备用码失败: " + err.Error())
		return
	}

	// 记录操作日志
	userstore.RecordLog(userId, logstore.LogTypeSystem, "开始设置两步验证")

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": i18n.T(c, i18n.MsgTwoFASetupInitSuccess),
		"data": Setup2FAResponse{
			Secret:      key.Secret(),
			QRCodeData:  qrCodeData,
			BackupCodes: backupCodes,
		},
	})
}

// Enable2FA 启用2FA
func Enable2FA(c *gin.Context) {
	var req Setup2FARequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpapi.ApiErrorI18n(c, i18n.MsgInvalidParams)
		return
	}

	userId := c.GetInt("id")

	// 获取2FA记录
	twoFA, err := twofastore.GetTwoFAByUserId(userId)
	if err != nil {
		common.SysError("get twofa by user id failed: " + err.Error())
		httpapi.ApiErrorI18n(c, i18n.MsgDatabaseError)
		return
	}
	if twoFA == nil {
		httpapi.ApiErrorI18n(c, i18n.MsgTwoFASetupRequired)
		return
	}
	if twoFA.IsEnabled {
		httpapi.ApiErrorI18n(c, i18n.MsgTwoFAAlreadyEnabled)
		return
	}

	// 验证TOTP验证码
	cleanCode, err := security.ValidateNumericCode(req.Code)
	if err != nil {
		httpapi.ApiErrorI18n(c, i18n.MsgInvalidParams)
		return
	}

	if !security.ValidateTOTPCode(twoFA.Secret, cleanCode) {
		httpapi.ApiErrorI18n(c, i18n.MsgTwoFACodeInvalid)
		return
	}

	// 启用2FA
	if err := twoFA.Enable(); err != nil {
		common.SysError("enable twofa failed: " + err.Error())
		httpapi.ApiErrorI18n(c, i18n.MsgDatabaseError)
		return
	}

	// 记录操作日志
	userstore.RecordLog(userId, logstore.LogTypeSystem, "成功启用两步验证")

	httpapi.ApiSuccessI18n(c, i18n.MsgTwoFAEnableSuccess, nil)
}

// Disable2FA 禁用2FA
func Disable2FA(c *gin.Context) {
	var req Verify2FARequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpapi.ApiErrorI18n(c, i18n.MsgInvalidParams)
		return
	}

	userId := c.GetInt("id")

	// 获取2FA记录
	twoFA, err := twofastore.GetTwoFAByUserId(userId)
	if err != nil {
		common.SysError("get twofa by user id failed: " + err.Error())
		httpapi.ApiErrorI18n(c, i18n.MsgDatabaseError)
		return
	}
	if twoFA == nil || !twoFA.IsEnabled {
		httpapi.ApiErrorI18n(c, i18n.MsgTwoFANotEnabled)
		return
	}

	// 验证TOTP验证码或备用码
	cleanCode, err := security.ValidateNumericCode(req.Code)
	isValidTOTP := false
	isValidBackup := false

	if err == nil {
		// 尝试验证TOTP
		isValidTOTP, _ = twoFA.ValidateTOTPAndUpdateUsage(cleanCode)
	}

	if !isValidTOTP {
		// 尝试验证备用码
		isValidBackup, err = twoFA.ValidateBackupCodeAndUpdateUsage(req.Code)
		if err != nil {
			httpapi.ApiErrorI18n(c, i18n.MsgTwoFACodeInvalid)
			return
		}
	}

	if !isValidTOTP && !isValidBackup {
		httpapi.ApiErrorI18n(c, i18n.MsgTwoFACodeInvalid)
		return
	}

	// 禁用2FA
	if err := twofastore.DisableTwoFA(userId); err != nil {
		common.SysError("disable twofa failed: " + err.Error())
		httpapi.ApiErrorI18n(c, i18n.MsgDatabaseError)
		return
	}

	// 记录操作日志
	userstore.RecordLog(userId, logstore.LogTypeSystem, "禁用两步验证")

	httpapi.ApiSuccessI18n(c, i18n.MsgTwoFADisableSuccess, nil)
}

// Get2FAStatus 获取用户2FA状态
func Get2FAStatus(c *gin.Context) {
	userId := c.GetInt("id")

	twoFA, err := twofastore.GetTwoFAByUserId(userId)
	if err != nil {
		common.SysError("get twofa by user id failed: " + err.Error())
		httpapi.ApiErrorI18n(c, i18n.MsgDatabaseError)
		return
	}

	status := map[string]interface{}{
		"enabled": false,
		"locked":  false,
	}

	if twoFA != nil {
		status["enabled"] = twoFA.IsEnabled
		status["locked"] = twoFA.IsLocked()
		if twoFA.IsEnabled {
			// 获取剩余备用码数量
			backupCount, err := twofastore.GetUnusedBackupCodeCount(userId)
			if err != nil {
				common.SysLog("获取备用码数量失败: " + err.Error())
			} else {
				status["backup_codes_remaining"] = backupCount
			}
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    status,
	})
}

// RegenerateBackupCodes 重新生成备用码
func RegenerateBackupCodes(c *gin.Context) {
	var req Verify2FARequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpapi.ApiErrorI18n(c, i18n.MsgInvalidParams)
		return
	}

	userId := c.GetInt("id")

	// 获取2FA记录
	twoFA, err := twofastore.GetTwoFAByUserId(userId)
	if err != nil {
		common.SysError("get twofa by user id failed: " + err.Error())
		httpapi.ApiErrorI18n(c, i18n.MsgDatabaseError)
		return
	}
	if twoFA == nil || !twoFA.IsEnabled {
		httpapi.ApiErrorI18n(c, i18n.MsgTwoFANotEnabled)
		return
	}

	// 验证TOTP验证码
	cleanCode, err := security.ValidateNumericCode(req.Code)
	if err != nil {
		httpapi.ApiErrorI18n(c, i18n.MsgInvalidParams)
		return
	}

	valid, err := twoFA.ValidateTOTPAndUpdateUsage(cleanCode)
	if err != nil {
		httpapi.ApiErrorI18n(c, i18n.MsgTwoFACodeInvalid)
		return
	}
	if !valid {
		httpapi.ApiErrorI18n(c, i18n.MsgTwoFACodeInvalid)
		return
	}

	// 生成新的备用码
	backupCodes, err := security.GenerateBackupCodes()
	if err != nil {
		httpapi.ApiErrorI18n(c, i18n.MsgTwoFABackupGenerateFailed)
		common.SysLog("生成备用码失败: " + err.Error())
		return
	}

	// 保存新的备用码
	if err := twofastore.CreateBackupCodes(userId, backupCodes); err != nil {
		httpapi.ApiErrorI18n(c, i18n.MsgTwoFABackupSaveFailed)
		common.SysLog("保存备用码失败: " + err.Error())
		return
	}

	// 记录操作日志
	userstore.RecordLog(userId, logstore.LogTypeSystem, "重新生成两步验证备用码")

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": i18n.T(c, i18n.MsgTwoFABackupRegenerateOK),
		"data": map[string]interface{}{
			"backup_codes": backupCodes,
		},
	})
}

// Verify2FALogin 登录时验证2FA
func Verify2FALogin(c *gin.Context) {
	var req Verify2FARequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpapi.ApiErrorI18n(c, i18n.MsgInvalidParams)
		return
	}

	// 从会话中获取pending用户信息
	session := sessions.Default(c)
	pendingUserId := session.Get("pending_user_id")
	if pendingUserId == nil {
		httpapi.ApiErrorI18n(c, i18n.MsgTwoFASessionExpired)
		return
	}
	userId, ok := pendingUserId.(int)
	if !ok {
		httpapi.ApiErrorI18n(c, i18n.MsgTwoFASessionInvalid)
		return
	}
	// 校验 pending session 时效：时间戳缺失（旧 session）或已超过
	// SecureVerificationTimeout 的 pending session 一律拒绝（fail-closed），
	// 防止攻击者在长 cookie 有效期内反复尝试 TOTP/备用码。
	pendingSetAt := session.Get("pending_2fa_set_at")
	setAt, ok := pendingSetAt.(int64)
	if !ok || time.Now().Unix()-setAt >= secureverificationcontroller.SecureVerificationTimeout {
		session.Delete("pending_username")
		session.Delete("pending_user_id")
		session.Delete("pending_2fa_set_at")
		if err := session.Save(); err != nil {
			common.SysError("clear expired pending 2fa session failed: " + err.Error())
		}
		httpapi.ApiErrorI18n(c, i18n.MsgTwoFASessionExpired)
		return
	}
	// 获取用户信息
	user, err := userstore.GetUserById(userId, false)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": i18n.T(c, i18n.MsgUserNotExists),
		})
		return
	}

	// 获取2FA记录
	twoFA, err := twofastore.GetTwoFAByUserId(user.Id)
	if err != nil {
		common.SysError("get twofa by user id failed: " + err.Error())
		httpapi.ApiErrorI18n(c, i18n.MsgDatabaseError)
		return
	}
	if twoFA == nil || !twoFA.IsEnabled {
		httpapi.ApiErrorI18n(c, i18n.MsgTwoFANotEnabled)
		return
	}

	// 验证TOTP验证码或备用码
	cleanCode, err := security.ValidateNumericCode(req.Code)
	isValidTOTP := false
	isValidBackup := false

	if err == nil {
		// 尝试验证TOTP
		isValidTOTP, _ = twoFA.ValidateTOTPAndUpdateUsage(cleanCode)
	}

	if !isValidTOTP {
		// 尝试验证备用码
		isValidBackup, err = twoFA.ValidateBackupCodeAndUpdateUsage(req.Code)
		if err != nil {
			httpapi.ApiErrorI18n(c, i18n.MsgTwoFACodeInvalid)
			return
		}
	}

	if !isValidTOTP && !isValidBackup {
		httpapi.ApiErrorI18n(c, i18n.MsgTwoFACodeInvalid)
		return
	}

	// 2FA验证成功，清理pending会话信息并完成登录
	session.Delete("pending_username")
	session.Delete("pending_user_id")
	session.Delete("pending_2fa_set_at")
	if err := session.Save(); err != nil {
		common.SysError("failed to save session after 2fa verification: " + err.Error())
		httpapi.ApiErrorI18n(c, i18n.MsgUserSessionSaveFailed)
		return
	}

	usercontroller.SetupLogin(user, c)
}

// Admin2FAStats 管理员获取2FA统计信息
func Admin2FAStats(c *gin.Context) {
	stats, err := twofastore.GetTwoFAStats()
	if err != nil {
		common.SysError("get twofa stats failed: " + err.Error())
		httpapi.ApiErrorI18n(c, i18n.MsgDatabaseError)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    stats,
	})
}

// AdminDisable2FA 管理员强制禁用用户2FA
func AdminDisable2FA(c *gin.Context) {
	userIdStr := c.Param("id")
	userId, err := strconv.Atoi(userIdStr)
	if err != nil {
		httpapi.ApiErrorI18n(c, i18n.MsgPasskeyInvalidUserID)
		return
	}

	// 检查目标用户权限
	targetUser, err := userstore.GetUserById(userId, false)
	if err != nil {
		common.SysError("get user by id failed: " + err.Error())
		httpapi.ApiErrorI18n(c, i18n.MsgDatabaseError)
		return
	}

	myRole := c.GetInt("role")
	if myRole <= targetUser.Role && myRole != common.RoleRootUser {
		httpapi.ApiErrorI18n(c, i18n.MsgTwoFAAdminForbidden)
		return
	}

	// 禁用2FA
	if err := twofastore.DisableTwoFA(userId); err != nil {
		if errors.Is(err, twofastore.ErrTwoFANotEnabled) {
			httpapi.ApiErrorI18n(c, i18n.MsgTwoFANotEnabled)
			return
		}
		common.SysError("disable twofa failed: " + err.Error())
		httpapi.ApiErrorI18n(c, i18n.MsgDatabaseError)
		return
	}

	// 记录操作日志：管理员身份放入 admin_info，避免目标用户可见内容泄露操作者。
	adminId := c.GetInt("id")
	adminName := c.GetString("username")
	adminInfo := map[string]interface{}{
		"admin_id":       adminId,
		"admin_username": adminName,
	}
	userstore.RecordLogWithAdminInfo(userId, logstore.LogTypeManage,
		"管理员强制禁用了用户的两步验证", adminInfo)

	audit.RecordAudit(c, auditstore.AuditModuleUser, auditstore.AuditActionDelete, "禁用用户 2FA: "+targetUser.Username, nil, map[string]interface{}{"user_id": userId})
	httpapi.ApiSuccessI18n(c, i18n.MsgTwoFAAdminDisabled, nil)
}
