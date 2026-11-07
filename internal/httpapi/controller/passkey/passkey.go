package passkeycontroller

import (
	"encoding/base64"
	"errors"
	"fmt"
	"github.com/NookMux/NookMux/internal/common"
	"github.com/NookMux/NookMux/internal/config/system"
	audit "github.com/NookMux/NookMux/internal/domain/audit"
	"github.com/NookMux/NookMux/internal/httpapi"
	secureverificationcontroller "github.com/NookMux/NookMux/internal/httpapi/controller/secure_verification"
	usercontroller "github.com/NookMux/NookMux/internal/httpapi/controller/user"
	"github.com/NookMux/NookMux/internal/i18n"
	passkeysvc "github.com/NookMux/NookMux/internal/infra/passkey"
	"github.com/NookMux/NookMux/internal/store/audit"
	"github.com/NookMux/NookMux/internal/store/passkey"
	"github.com/NookMux/NookMux/internal/store/twofa"
	"github.com/NookMux/NookMux/internal/store/user"
	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
	"github.com/go-webauthn/webauthn/protocol"
	webauthnlib "github.com/go-webauthn/webauthn/webauthn"
	"net/http"
	"strconv"
	"strings"
	"time"
)

type PasskeyRegisterRequest struct {
	DeviceName string `json:"device_name"`
}

const (
	passkeyRegistrationDeviceNameSessionKey = "passkey_registration_device_name"
	maxPasskeyDeviceNameLength              = 255
)

type PasskeyListItem struct {
	ID             int        `json:"id"`
	DeviceName     string     `json:"device_name"`
	Attachment     string     `json:"attachment"`
	BackupEligible bool       `json:"backup_eligible"`
	BackupState    bool       `json:"backup_state"`
	LastUsedAt     *time.Time `json:"last_used_at"`
	CreatedAt      time.Time  `json:"created_at"`
}

func normalizePasskeyDeviceName(deviceName string) string {
	deviceName = strings.TrimSpace(deviceName)
	runes := []rune(deviceName)
	if len(runes) > maxPasskeyDeviceNameLength {
		deviceName = string(runes[:maxPasskeyDeviceNameLength])
	}
	return deviceName
}

func PasskeyRegisterBegin(c *gin.Context) {
	if !system.GetPasskeySettings().Enabled {
		httpapi.ApiErrorI18n(c, i18n.MsgPasskeyLoginDisabled)
		return
	}

	user, err := getSessionUser(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}

	if !requirePasskeyRegistrationVerification(c, user.Id) {
		return
	}

	settings := system.GetPasskeySettings()
	maxPasskeys := settings.MaxPasskeysPerUser
	if maxPasskeys < 1 {
		maxPasskeys = 1
	}

	count, err := passkeystore.CountPasskeysByUserID(user.Id)
	if err != nil {
		common.SysError("count passkeys failed: " + err.Error())
		httpapi.ApiErrorI18n(c, i18n.MsgDatabaseError)
		return
	}

	if count >= int64(maxPasskeys) {
		httpapi.ApiErrorI18n(c, i18n.MsgPasskeyLimitReached, map[string]any{"Max": maxPasskeys})
		return
	}

	credentials, err := passkeystore.GetPasskeysByUserID(user.Id)
	if err != nil {
		common.SysError("get passkeys failed: " + err.Error())
		httpapi.ApiErrorI18n(c, i18n.MsgDatabaseError)
		return
	}

	wa, err := passkeysvc.BuildWebAuthn(c.Request)
	if err != nil {
		common.SysError("build webauthn failed: " + err.Error())
		httpapi.ApiErrorI18n(c, i18n.MsgPasskeyCreateFailed)
		return
	}

	waUser := passkeysvc.NewWebAuthnUser(user, nil)
	var options []webauthnlib.RegistrationOption
	if len(credentials) > 0 {
		excludeList := make([]protocol.CredentialDescriptor, 0, len(credentials))
		for _, cred := range credentials {
			webAuthnCredential := cred.ToWebAuthnCredential()
			excludeList = append(excludeList, webAuthnCredential.Descriptor())
		}
		options = append(options, webauthnlib.WithExclusions(excludeList))
	}

	creation, sessionData, err := wa.BeginRegistration(waUser, options...)
	if err != nil {
		common.SysError("begin passkey registration failed: " + err.Error())
		httpapi.ApiErrorI18n(c, i18n.MsgPasskeyCreateFailed)
		return
	}

	var req PasskeyRegisterRequest
	session := sessions.Default(c)
	if err := c.ShouldBindJSON(&req); err == nil {
		deviceName := normalizePasskeyDeviceName(req.DeviceName)
		if deviceName != "" {
			session.Set(passkeyRegistrationDeviceNameSessionKey, deviceName)
		} else {
			session.Delete(passkeyRegistrationDeviceNameSessionKey)
		}
	} else {
		session.Delete(passkeyRegistrationDeviceNameSessionKey)
	}

	if err := passkeysvc.SaveSessionData(c, passkeysvc.RegistrationSessionKey, sessionData); err != nil {
		common.SysError("save passkey registration session failed: " + err.Error())
		httpapi.ApiErrorI18n(c, i18n.MsgPasskeyCreateFailed)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data": gin.H{
			"options": creation,
		},
	})
}

func PasskeyRegisterFinish(c *gin.Context) {
	if !system.GetPasskeySettings().Enabled {
		httpapi.ApiErrorI18n(c, i18n.MsgPasskeyLoginDisabled)
		return
	}

	user, err := getSessionUser(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}

	if !requirePasskeyRegistrationVerification(c, user.Id) {
		return
	}

	settings := system.GetPasskeySettings()
	maxPasskeys := settings.MaxPasskeysPerUser
	if maxPasskeys < 1 {
		maxPasskeys = 1
	}

	count, err := passkeystore.CountPasskeysByUserID(user.Id)
	if err != nil {
		common.SysError("count passkeys failed: " + err.Error())
		httpapi.ApiErrorI18n(c, i18n.MsgDatabaseError)
		return
	}

	if count >= int64(maxPasskeys) {
		httpapi.ApiErrorI18n(c, i18n.MsgPasskeyLimitReached, map[string]any{"Max": maxPasskeys})
		return
	}

	wa, err := passkeysvc.BuildWebAuthn(c.Request)
	if err != nil {
		common.SysError("build webauthn failed: " + err.Error())
		httpapi.ApiErrorI18n(c, i18n.MsgPasskeyCreateFailed)
		return
	}

	sessionData, err := passkeysvc.PopSessionData(c, passkeysvc.RegistrationSessionKey)
	if err != nil {
		common.SysError("pop passkey registration session failed: " + err.Error())
		httpapi.ApiErrorI18n(c, i18n.MsgPasskeyCreateFailed)
		return
	}

	waUser := passkeysvc.NewWebAuthnUser(user, nil)
	credential, err := wa.FinishRegistration(waUser, *sessionData, c.Request)
	if err != nil {
		common.SysError("finish passkey registration failed: " + err.Error())
		httpapi.ApiErrorI18n(c, i18n.MsgPasskeyCreateFailed)
		return
	}

	passkeyCredential := passkeystore.NewPasskeyCredentialFromWebAuthn(user.Id, credential)
	if passkeyCredential == nil {
		httpapi.ApiErrorI18n(c, i18n.MsgPasskeyCreateFailed)
		return
	}

	session := sessions.Default(c)
	if deviceName, ok := session.Get(passkeyRegistrationDeviceNameSessionKey).(string); ok {
		passkeyCredential.DeviceName = normalizePasskeyDeviceName(deviceName)
	}
	session.Delete(passkeyRegistrationDeviceNameSessionKey)
	_ = session.Save()

	if err := passkeystore.CreatePasskeyCredential(passkeyCredential); err != nil {
		common.SysError("create passkey credential failed: " + err.Error())
		httpapi.ApiErrorI18n(c, i18n.MsgDatabaseError)
		return
	}

	httpapi.ApiSuccessI18n(c, i18n.MsgPasskeyRegisterOK, nil)
}

func PasskeyDelete(c *gin.Context) {
	user, err := getSessionUser(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}

	if !requirePasskeyDeleteVerification(c, user.Id) {
		return
	}

	idStr := c.Param("id")
	if idStr == "" {
		if err := passkeystore.DeletePasskeyByUserID(user.Id); err != nil {
			common.SysError("delete passkey by user id failed: " + err.Error())
			httpapi.ApiErrorI18n(c, i18n.MsgDatabaseError)
			return
		}
	} else {
		id, err := strconv.Atoi(idStr)
		if err != nil {
			httpapi.ApiErrorI18n(c, i18n.MsgPasskeyInvalidID)
			return
		}

		credential, err := passkeystore.GetPasskeyByID(id)
		if err != nil {
			common.SysError("get passkey by id failed: " + err.Error())
			httpapi.ApiErrorI18n(c, i18n.MsgDatabaseError)
			return
		}

		if credential.UserID != user.Id {
			c.JSON(http.StatusForbidden, gin.H{
				"success": false,
				"message": i18n.T(c, i18n.MsgPasskeyDeleteDenied),
			})
			return
		}

		if err := passkeystore.DeletePasskeyByID(id); err != nil {
			common.SysError("delete passkey by id failed: " + err.Error())
			httpapi.ApiErrorI18n(c, i18n.MsgDatabaseError)
			return
		}
	}

	httpapi.ApiSuccessI18n(c, i18n.MsgPasskeyUnbound, nil)
}

func PasskeyStatus(c *gin.Context) {
	user, err := getSessionUser(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}

	credentials, err := passkeystore.GetPasskeysByUserID(user.Id)
	if err != nil {
		common.SysError("get passkeys failed: " + err.Error())
		httpapi.ApiErrorI18n(c, i18n.MsgDatabaseError)
		return
	}

	settings := system.GetPasskeySettings()
	maxPasskeys := settings.MaxPasskeysPerUser
	if maxPasskeys < 1 {
		maxPasskeys = 1
	}

	passkeys := make([]PasskeyListItem, 0, len(credentials))
	for _, cred := range credentials {
		passkeys = append(passkeys, PasskeyListItem{
			ID:             cred.ID,
			DeviceName:     cred.DeviceName,
			Attachment:     cred.Attachment,
			BackupEligible: cred.BackupEligible,
			BackupState:    cred.BackupState,
			LastUsedAt:     cred.LastUsedAt,
			CreatedAt:      cred.CreatedAt,
		})
	}

	data := gin.H{
		"enabled":      len(credentials) > 0,
		"passkeys":     passkeys,
		"count":        len(credentials),
		"max_passkeys": maxPasskeys,
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    data,
	})
}

func PasskeyLoginBegin(c *gin.Context) {
	if !system.GetPasskeySettings().Enabled {
		httpapi.ApiErrorI18n(c, i18n.MsgPasskeyLoginDisabled)
		return
	}

	wa, err := passkeysvc.BuildWebAuthn(c.Request)
	if err != nil {
		common.SysError("build webauthn failed: " + err.Error())
		httpapi.ApiErrorI18n(c, i18n.MsgDatabaseError)
		return
	}

	// 登录仪式强制用户在场验证（UV）：passkey 作为已注册因子替代 TOTP 的
	// 前提是凭据使用经过本人生物特征/PIN 确认，否则凭据文件泄漏即等于
	// 账户泄漏。注册仪式保持系统设置的可配置值不变。
	assertion, sessionData, err := wa.BeginDiscoverableLogin(
		webauthnlib.WithUserVerification(protocol.VerificationRequired),
	)
	if err != nil {
		common.SysError("begin passkey discoverable login failed: " + err.Error())
		httpapi.ApiErrorI18n(c, i18n.MsgDatabaseError)
		return
	}

	if err := passkeysvc.SaveSessionData(c, passkeysvc.LoginSessionKey, sessionData); err != nil {
		common.SysError("save passkey login session failed: " + err.Error())
		httpapi.ApiErrorI18n(c, i18n.MsgDatabaseError)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data": gin.H{
			"options": assertion,
		},
	})
}

func PasskeyLoginFinish(c *gin.Context) {
	if !system.GetPasskeySettings().Enabled {
		httpapi.ApiErrorI18n(c, i18n.MsgPasskeyLoginDisabled)
		return
	}

	wa, err := passkeysvc.BuildWebAuthn(c.Request)
	if err != nil {
		common.SysError("build webauthn failed: " + err.Error())
		httpapi.ApiErrorI18n(c, i18n.MsgDatabaseError)
		return
	}

	sessionData, err := passkeysvc.PopSessionData(c, passkeysvc.LoginSessionKey)
	if err != nil {
		common.SysError("pop passkey login session failed: " + err.Error())
		httpapi.ApiErrorI18n(c, i18n.MsgDatabaseError)
		return
	}

	handler := func(rawID, userHandle []byte) (webauthnlib.User, error) {
		// 首先通过凭证ID查找用户
		credential, err := passkeystore.GetPasskeyByCredentialID(rawID)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", i18n.T(c, i18n.MsgPasskeyCredentialNotFound), err)
		}

		// 通过凭证获取用户
		user := &userstore.User{Id: credential.UserID}
		if err := user.FillUserById(); err != nil {
			return nil, fmt.Errorf("%s: %w", i18n.T(c, i18n.MsgPasskeyUserInfoFailed), err)
		}

		if user.Status != common.UserStatusEnabled {
			return nil, errors.New(i18n.T(c, i18n.MsgPasskeyUserDisabled))
		}

		if len(userHandle) > 0 {
			userID, parseErr := strconv.Atoi(string(userHandle))
			if parseErr != nil {
				// 记录异常但继续验证，因为某些客户端可能使用非数字格式
				common.SysLog(fmt.Sprintf("PasskeyLogin: userHandle parse error for credential, length: %d", len(userHandle)))
			} else if userID != user.Id {
				return nil, errors.New(i18n.T(c, i18n.MsgPasskeyUserHandleMismatch))
			}
		}

		return passkeysvc.NewWebAuthnUser(user, credential), nil
	}

	waUser, credential, err := wa.FinishPasskeyLogin(handler, *sessionData, c.Request)
	if err != nil {
		common.SysError("finish passkey login failed: " + err.Error())
		httpapi.ApiErrorI18n(c, i18n.MsgDatabaseError)
		return
	}

	userWrapper, ok := waUser.(*passkeysvc.WebAuthnUser)
	if !ok {
		httpapi.ApiErrorI18n(c, i18n.MsgPasskeyStateInvalid)
		return
	}

	modelUser := userWrapper.ModelUser()
	if modelUser == nil {
		httpapi.ApiErrorI18n(c, i18n.MsgPasskeyStateInvalid)
		return
	}

	if modelUser.Status != common.UserStatusEnabled {
		httpapi.ApiErrorI18n(c, i18n.MsgPasskeyUserDisabled)
		return
	}

	// 更新凭证信息
	updatedCredential := passkeystore.NewPasskeyCredentialFromWebAuthn(modelUser.Id, credential)
	if updatedCredential == nil {
		httpapi.ApiErrorI18n(c, i18n.MsgPasskeyUpdateFailed)
		return
	}

	existingCred, err := passkeystore.GetPasskeyByCredentialID(credential.ID)
	if err == nil && existingCred != nil {
		updatedCredential.ID = existingCred.ID
		updatedCredential.DeviceName = existingCred.DeviceName
	}

	now := time.Now()
	updatedCredential.LastUsedAt = &now
	if updatedCredential.ID > 0 {
		if err := passkeystore.UpdatePasskeyCredential(updatedCredential); err != nil {
			common.SysError("update passkey credential failed: " + err.Error())
			httpapi.ApiErrorI18n(c, i18n.MsgDatabaseError)
			return
		}
	} else {
		if err := passkeystore.CreatePasskeyCredential(updatedCredential); err != nil {
			common.SysError("create passkey credential failed: " + err.Error())
			httpapi.ApiErrorI18n(c, i18n.MsgDatabaseError)
			return
		}
	}

	usercontroller.SetupLogin(modelUser, c)
}

func AdminResetPasskey(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		httpapi.ApiErrorI18n(c, i18n.MsgPasskeyInvalidUserID)
		return
	}

	user := &userstore.User{Id: id}
	if err := user.FillUserById(); err != nil {
		common.SysError("fill user by id failed: " + err.Error())
		httpapi.ApiErrorI18n(c, i18n.MsgDatabaseError)
		return
	}

	// 角色层级检查：非 root 管理员不能重置同级或更高级用户的 passkey，
	// 匹配 ManageUser / AdminDisable2FA 的既有模式。
	myRole := c.GetInt("role")
	if myRole <= user.Role && myRole != common.RoleRootUser {
		httpapi.ApiErrorI18n(c, i18n.MsgUserNoPermissionHigherLevel)
		return
	}

	count, err := passkeystore.CountPasskeysByUserID(user.Id)
	if err != nil {
		common.SysError("count passkeys failed: " + err.Error())
		httpapi.ApiErrorI18n(c, i18n.MsgDatabaseError)
		return
	}

	if count == 0 {
		httpapi.ApiErrorI18n(c, i18n.MsgPasskeyNotBound)
		return
	}

	if err := passkeystore.DeletePasskeyByUserID(user.Id); err != nil {
		common.SysError("delete passkey by user id failed: " + err.Error())
		httpapi.ApiErrorI18n(c, i18n.MsgDatabaseError)
		return
	}
	// 凭证变更：失效目标用户全部既有会话。
	if err := userstore.BumpAuthVersion(user.Id); err != nil {
		common.SysError("failed to bump auth version after admin resetting passkey: " + err.Error())
	}

	audit.RecordAudit(c, auditstore.AuditModuleUser, auditstore.AuditActionDelete, "重置用户 Passkey: "+user.Username, nil, map[string]interface{}{"user_id": user.Id})
	httpapi.ApiSuccessI18n(c, i18n.MsgPasskeyResetOK, nil)
}

func PasskeyVerifyBegin(c *gin.Context) {
	if !system.GetPasskeySettings().Enabled {
		httpapi.ApiErrorI18n(c, i18n.MsgPasskeyLoginDisabled)
		return
	}

	user, err := getSessionUser(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}

	credentials, err := passkeystore.GetPasskeysByUserID(user.Id)
	if err != nil {
		common.SysError("get passkeys failed: " + err.Error())
		httpapi.ApiErrorI18n(c, i18n.MsgDatabaseError)
		return
	}
	if len(credentials) == 0 {
		httpapi.ApiErrorI18n(c, i18n.MsgPasskeyNotBound)
		return
	}

	wa, err := passkeysvc.BuildWebAuthn(c.Request)
	if err != nil {
		common.SysError("build webauthn failed: " + err.Error())
		httpapi.ApiErrorI18n(c, i18n.MsgDatabaseError)
		return
	}

	waUser := passkeysvc.NewWebAuthnUser(user, nil)
	assertion, sessionData, err := wa.BeginLogin(waUser)
	if err != nil {
		common.SysError("begin passkey login failed: " + err.Error())
		httpapi.ApiErrorI18n(c, i18n.MsgDatabaseError)
		return
	}

	if err := passkeysvc.SaveSessionData(c, passkeysvc.VerifySessionKey, sessionData); err != nil {
		common.SysError("save passkey verify session failed: " + err.Error())
		httpapi.ApiErrorI18n(c, i18n.MsgDatabaseError)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data": gin.H{
			"options": assertion,
		},
	})
}

func PasskeyVerifyFinish(c *gin.Context) {
	if !system.GetPasskeySettings().Enabled {
		httpapi.ApiErrorI18n(c, i18n.MsgPasskeyLoginDisabled)
		return
	}

	user, err := getSessionUser(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}

	wa, err := passkeysvc.BuildWebAuthn(c.Request)
	if err != nil {
		common.SysError("build webauthn failed: " + err.Error())
		httpapi.ApiErrorI18n(c, i18n.MsgDatabaseError)
		return
	}

	credentials, err := passkeystore.GetPasskeysByUserID(user.Id)
	if err != nil {
		common.SysError("get passkeys failed: " + err.Error())
		httpapi.ApiErrorI18n(c, i18n.MsgDatabaseError)
		return
	}
	if len(credentials) == 0 {
		httpapi.ApiErrorI18n(c, i18n.MsgPasskeyNotBound)
		return
	}

	sessionData, err := passkeysvc.PopSessionData(c, passkeysvc.VerifySessionKey)
	if err != nil {
		common.SysError("pop passkey verify session failed: " + err.Error())
		httpapi.ApiErrorI18n(c, i18n.MsgDatabaseError)
		return
	}

	waUser := passkeysvc.NewWebAuthnUser(user, nil)
	validatedCred, err := wa.FinishLogin(waUser, *sessionData, c.Request)
	if err != nil {
		common.SysError("finish passkey login failed: " + err.Error())
		httpapi.ApiErrorI18n(c, i18n.MsgDatabaseError)
		return
	}

	// 查找被验证的凭证并更新最后使用时间
	for _, cred := range credentials {
		credBytes, _ := base64.StdEncoding.DecodeString(cred.CredentialID)
		if string(credBytes) == string(validatedCred.ID) {
			now := time.Now()
			cred.LastUsedAt = &now
			cred.SignCount = validatedCred.Authenticator.SignCount
			if err := passkeystore.UpdatePasskeyCredential(cred); err != nil {
				common.SysError("update passkey credential failed: " + err.Error())
				httpapi.ApiErrorI18n(c, i18n.MsgDatabaseError)
				return
			}
			break
		}
	}

	_, err = secureverificationcontroller.PasskeyVerifyAndMarkReadySession(c, user.Id)
	if err != nil {
		common.SysError("save passkey verify session failed: " + err.Error())
		httpapi.ApiErrorI18n(c, i18n.MsgPasskeySaveVerifyFailed)
		return
	}

	httpapi.ApiSuccessI18n(c, i18n.MsgPasskeyVerifyOK, nil)
}

func getSessionUser(c *gin.Context) (*userstore.User, error) {
	session := sessions.Default(c)
	idRaw := session.Get("id")
	if idRaw == nil {
		return nil, errors.New(i18n.T(c, i18n.MsgNotLoggedIn))
	}
	id, ok := idRaw.(int)
	if !ok {
		return nil, errors.New(i18n.T(c, i18n.MsgPasskeyInvalidSession))
	}
	user := &userstore.User{Id: id}
	if err := user.FillUserById(); err != nil {
		return nil, err
	}
	if user.Status != common.UserStatusEnabled {
		return nil, errors.New(i18n.T(c, i18n.MsgPasskeyUserDisabled))
	}
	return user, nil
}

func requirePasskeyRegistrationVerification(c *gin.Context, userID int) bool {
	twoFA, err := twofastore.GetTwoFAByUserId(userID)
	if err != nil {
		common.SysError("get two fa by user id failed: " + err.Error())
		httpapi.ApiErrorI18n(c, i18n.MsgDatabaseError)
		return false
	}
	if twoFA != nil && twoFA.IsEnabled {
		return requireSecureVerificationMethod(c, userID, secureverificationcontroller.SecureVerificationMethod2FA)
	}
	// 仅-passkey 账户（未启用 2FA）：注册第二把凭据同样要求 passkey 方式
	// 验证，会话劫持者无法静默植入自己的凭据获得跨会话持久访问。
	if _, err := passkeystore.GetPasskeyByUserID(userID); err == nil {
		return requireSecureVerificationMethod(c, userID, secureverificationcontroller.SecureVerificationMethodPasskey)
	}
	return true
}

func requirePasskeyDeleteVerification(c *gin.Context, userID int) bool {
	twoFA, err := twofastore.GetTwoFAByUserId(userID)
	if err != nil {
		common.SysError("get two fa by user id failed: " + err.Error())
		httpapi.ApiErrorI18n(c, i18n.MsgDatabaseError)
		return false
	}
	if twoFA != nil && twoFA.IsEnabled {
		return requireSecureVerificationMethod(c, userID, secureverificationcontroller.SecureVerificationMethod2FA)
	}

	_, err = passkeystore.GetPasskeyByUserID(userID)
	if err != nil {
		if errors.Is(err, passkeystore.ErrPasskeyNotFound) {
			httpapi.ApiErrorI18n(c, i18n.MsgPasskeyNotBound)
			return false
		}
		common.SysError("get passkey by user id failed: " + err.Error())
		httpapi.ApiErrorI18n(c, i18n.MsgDatabaseError)
		return false
	}

	return requireSecureVerificationMethod(c, userID, secureverificationcontroller.SecureVerificationMethodPasskey)
}

type PasskeyUpdateRequest struct {
	DeviceName string `json:"device_name"`
}

func PasskeyUpdate(c *gin.Context) {
	user, err := getSessionUser(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}

	idStr := c.Param("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		httpapi.ApiErrorI18n(c, i18n.MsgPasskeyInvalidID)
		return
	}

	credential, err := passkeystore.GetPasskeyByID(id)
	if err != nil {
		common.SysError("get passkey by id failed: " + err.Error())
		httpapi.ApiErrorI18n(c, i18n.MsgDatabaseError)
		return
	}

	if credential.UserID != user.Id {
		c.JSON(http.StatusForbidden, gin.H{
			"success": false,
			"message": i18n.T(c, i18n.MsgPasskeyUpdateDenied),
		})
		return
	}

	var req PasskeyUpdateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpapi.ApiErrorI18n(c, i18n.MsgPasskeyRequestInvalid)
		return
	}

	credential.DeviceName = normalizePasskeyDeviceName(req.DeviceName)
	if err := passkeystore.UpdatePasskeyCredential(credential); err != nil {
		common.SysError("update passkey credential failed: " + err.Error())
		httpapi.ApiErrorI18n(c, i18n.MsgDatabaseError)
		return
	}

	httpapi.ApiSuccessI18n(c, i18n.MsgPasskeyUpdated, nil)
}

func requireSecureVerificationMethod(c *gin.Context, userID int, method string) bool {
	if !secureverificationcontroller.HasSecureVerificationMethodForUser(c, userID, method) {
		httpapi.ApiErrorI18n(c, i18n.MsgPasskeySecureRequired)
		return false
	}
	return true
}
