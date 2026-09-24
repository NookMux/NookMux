package usercontroller

import (
	"errors"
	"fmt"
	"github.com/NookMux/NookMux/internal/common"
	"github.com/NookMux/NookMux/internal/config"
	audit "github.com/NookMux/NookMux/internal/domain/audit"
	domaingroup "github.com/NookMux/NookMux/internal/domain/group"
	"github.com/NookMux/NookMux/internal/domain/shared"
	"github.com/NookMux/NookMux/internal/httpapi"
	"github.com/NookMux/NookMux/internal/i18n"
	"github.com/NookMux/NookMux/internal/infra/log"
	"github.com/NookMux/NookMux/internal/infra/security"
	"github.com/NookMux/NookMux/internal/store/audit"
	"github.com/NookMux/NookMux/internal/store/channel"
	"github.com/NookMux/NookMux/internal/store/db"
	"github.com/NookMux/NookMux/internal/store/log"
	"github.com/NookMux/NookMux/internal/store/redemption"
	"github.com/NookMux/NookMux/internal/store/token"
	"github.com/NookMux/NookMux/internal/store/twofa"
	"github.com/NookMux/NookMux/internal/store/user"
	"github.com/NookMux/NookMux/pkg/jsonx"
	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
	"math"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"
)

type LoginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

func invalidateSecuritySensitiveUserCaches(userId int) {
	if userId == 0 {
		return
	}
	if err := userstore.InvalidateUserCache(userId); err != nil {
		common.SysLog(fmt.Sprintf("failed to invalidate user cache for user %d: %v", userId, err))
	}
	if err := tokenstore.InvalidateUserTokensCache(userId); err != nil {
		common.SysLog(fmt.Sprintf("failed to invalidate user token cache for user %d: %v", userId, err))
	}
}

func addWouldOverflowInt(base int, delta int) bool {
	return delta > 0 && base > math.MaxInt-delta
}

func Login(c *gin.Context) {
	if !common.PasswordLoginEnabled {
		httpapi.ApiErrorI18n(c, i18n.MsgUserPasswordLoginDisabled)
		return
	}
	var loginRequest LoginRequest
	err := jsonx.DecodeJson(c.Request.Body, &loginRequest)
	if err != nil {
		httpapi.ApiErrorI18n(c, i18n.MsgInvalidParams)
		return
	}
	username := loginRequest.Username
	password := loginRequest.Password
	if username == "" || password == "" {
		httpapi.ApiErrorI18n(c, i18n.MsgInvalidParams)
		return
	}
	user := userstore.User{
		Username: username,
		Password: password,
	}
	err = user.ValidateAndFill()
	if err != nil {
		switch {
		case errors.Is(err, dbstore.ErrDatabase):
			common.SysLog(fmt.Sprintf("Login database error for user %s: %v", username, err))
			httpapi.ApiErrorI18n(c, i18n.MsgUserLoginUnavailable)
		case errors.Is(err, dbstore.ErrUserEmptyCredentials):
			httpapi.ApiErrorI18n(c, i18n.MsgInvalidParams)
		default:
			httpapi.ApiErrorI18n(c, i18n.MsgUserUsernameOrPasswordError)
		}
		return
	}

	// 检查是否启用2FA
	if twofastore.IsTwoFAEnabled(user.Id) {
		// 设置pending session，等待2FA验证；记录时间戳用于校验时效
		session := sessions.Default(c)
		session.Set("pending_username", user.Username)
		session.Set("pending_user_id", user.Id)
		session.Set("pending_2fa_set_at", time.Now().Unix())
		err := session.Save()
		if err != nil {
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

	SetupLogin(&user, c)
}

// setup session & cookies and then return user info
func SetupLogin(user *userstore.User, c *gin.Context) {
	session := sessions.Default(c)
	session.Set("id", user.Id)
	session.Set("username", user.Username)
	session.Set("role", user.Role)
	session.Set("status", user.Status)
	session.Set("group", user.Group)
	err := session.Save()
	if err != nil {
		httpapi.ApiErrorI18n(c, i18n.MsgUserSessionSaveFailed)
		return
	}

	// 登录成功后更新最后登录时间
	if err := userstore.UpdateLastLoginAt(user.Id); err != nil {
		common.SysError(fmt.Sprintf("failed to update last_login_at for user %d: %v", user.Id, err))
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "",
		"success": true,
		"data": map[string]any{
			"id":           user.Id,
			"username":     user.Username,
			"display_name": user.DisplayName,
			"role":         user.Role,
			"status":       user.Status,
			"group":        user.Group,
		},
	})
}

func Logout(c *gin.Context) {
	session := sessions.Default(c)
	session.Clear()
	err := session.Save()
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"message": err.Error(),
			"success": false,
		})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"message": "",
		"success": true,
	})
}

func Register(c *gin.Context) {
	if !common.RegisterEnabled {
		httpapi.ApiErrorI18n(c, i18n.MsgUserRegisterDisabled)
		return
	}
	if !common.PasswordRegisterEnabled {
		httpapi.ApiErrorI18n(c, i18n.MsgUserPasswordRegisterDisabled)
		return
	}
	var user userstore.User
	err := jsonx.DecodeJson(c.Request.Body, &user)
	if err != nil {
		httpapi.ApiErrorI18n(c, i18n.MsgInvalidParams)
		return
	}
	if err := security.Validate.Struct(&user); err != nil {
		httpapi.ApiErrorI18n(c, i18n.MsgUserInputInvalid, map[string]any{"Error": err.Error()})
		return
	}
	if common.EmailVerificationEnabled {
		if user.Email == "" || user.VerificationCode == "" {
			httpapi.ApiErrorI18n(c, i18n.MsgUserEmailVerificationRequired)
			return
		}
		if !security.VerifyCodeWithKey(user.Email, user.VerificationCode, security.EmailVerificationPurpose) {
			httpapi.ApiErrorI18n(c, i18n.MsgUserVerificationCodeError)
			return
		}
	}
	exist, err := userstore.CheckUserExistOrDeleted(user.Username, user.Email)
	if err != nil {
		httpapi.ApiErrorI18n(c, i18n.MsgDatabaseError)
		common.SysLog(fmt.Sprintf("CheckUserExistOrDeleted error: %v", err))
		return
	}
	if exist {
		httpapi.ApiErrorI18n(c, i18n.MsgUserExists)
		return
	}
	affCode := user.AffCode // this code is the inviter's code, not the user's own code
	inviterId, _ := userstore.GetUserIdByAffCode(affCode)
	cleanUser := userstore.User{
		Username:    user.Username,
		Password:    user.Password,
		DisplayName: user.Username,
		InviterId:   inviterId,
		Role:        common.RoleCommonUser, // 明确设置角色为普通用户
	}
	if common.EmailVerificationEnabled {
		cleanUser.Email = user.Email
	}
	if err := cleanUser.Insert(inviterId); err != nil {
		common.SysError("failed to insert user: " + err.Error())
		httpapi.ApiErrorI18n(c, i18n.MsgDatabaseError)
		return
	}

	// 获取插入后的用户ID
	var insertedUser userstore.User
	if err := dbstore.DB.Where("username = ?", cleanUser.Username).First(&insertedUser).Error; err != nil {
		httpapi.ApiErrorI18n(c, i18n.MsgUserRegisterFailed)
		return
	}
	// 生成默认令牌
	if shared.GenerateDefaultToken {
		key, err := common.GenerateKey()
		if err != nil {
			httpapi.ApiErrorI18n(c, i18n.MsgUserDefaultTokenFailed)
			common.SysLog("failed to generate token key: " + err.Error())
			return
		}
		// 生成默认令牌
		token := tokenstore.Token{
			UserId:             insertedUser.Id, // 使用插入后的用户ID
			Name:               cleanUser.Username + "的初始令牌",
			Key:                key,
			CreatedTime:        common.GetTimestamp(),
			AccessedTime:       common.GetTimestamp(),
			ExpiredTime:        -1,     // 永不过期
			RemainQuota:        500000, // 示例额度
			UnlimitedQuota:     true,
			ModelLimitsEnabled: false,
		}
		if config.DefaultUseAutoGroup {
			token.Group = "auto"
		}
		if err := token.Insert(); err != nil {
			httpapi.ApiErrorI18n(c, i18n.MsgUserDefaultTokenFailed)
			return
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
	})
}

func GetAllUsers(c *gin.Context) {
	pageInfo := common.GetPageQuery(c)
	users, total, err := userstore.GetAllUsers(pageInfo)
	if err != nil {
		common.SysError("failed to get all users: " + err.Error())
		httpapi.ApiErrorI18n(c, i18n.MsgDatabaseError)
		return
	}

	pageInfo.SetTotal(int(total))
	pageInfo.SetItems(users)

	httpapi.ApiSuccess(c, pageInfo)
}

func SearchUsers(c *gin.Context) {
	keyword := c.Query("keyword")
	username := c.Query("username")
	displayName := c.Query("display_name")
	email := c.Query("email")
	linuxDoId := c.Query("linux_do_id")
	githubId := c.Query("github_id")
	group := c.Query("group")
	status := c.Query("status")
	role := c.Query("role")
	pageInfo := common.GetPageQuery(c)

	// 如果使用旧的 keyword 参数，保持向后兼容
	if keyword != "" && username == "" && displayName == "" && email == "" {
		users, total, err := userstore.SearchUsers(keyword, group, status, pageInfo.GetStartIdx(), pageInfo.GetPageSize())
		if err != nil {
			common.SysError("failed to search users: " + err.Error())
			httpapi.ApiErrorI18n(c, i18n.MsgDatabaseError)
			return
		}
		pageInfo.SetTotal(int(total))
		pageInfo.SetItems(users)
		httpapi.ApiSuccess(c, pageInfo)
		return
	}

	// 新的多字段搜索
	users, total, err := userstore.SearchUsersAdvanced(username, displayName, email, linuxDoId, githubId, status, role, group, pageInfo.GetStartIdx(), pageInfo.GetPageSize())
	if err != nil {
		common.SysError("failed to search users: " + err.Error())
		httpapi.ApiErrorI18n(c, i18n.MsgDatabaseError)
		return
	}

	pageInfo.SetTotal(int(total))
	pageInfo.SetItems(users)
	httpapi.ApiSuccess(c, pageInfo)
}

func GetUser(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		httpapi.ApiErrorI18n(c, i18n.MsgInvalidParams)
		return
	}
	user, err := userstore.GetUserById(id, false)
	if err != nil {
		common.SysError("failed to get user by id: " + err.Error())
		httpapi.ApiErrorI18n(c, i18n.MsgDatabaseError)
		return
	}
	myRole := c.GetInt("role")
	if myRole <= user.Role && myRole != common.RoleRootUser {
		httpapi.ApiErrorI18n(c, i18n.MsgUserNoPermissionSameLevel)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    user,
	})
}

func GenerateAccessToken(c *gin.Context) {
	id := c.GetInt("id")
	user, err := userstore.GetUserById(id, true)
	if err != nil {
		common.SysError("failed to get user by id: " + err.Error())
		httpapi.ApiErrorI18n(c, i18n.MsgDatabaseError)
		return
	}
	// get rand int 28-32
	randI := common.GetRandomInt(4)
	key, err := common.GenerateRandomKey(29 + randI)
	if err != nil {
		httpapi.ApiErrorI18n(c, i18n.MsgGenerateFailed)
		common.SysLog("failed to generate key: " + err.Error())
		return
	}
	user.SetAccessToken(key)

	if dbstore.DB.Where("access_token = ?", user.AccessToken).First(user).RowsAffected != 0 {
		httpapi.ApiErrorI18n(c, i18n.MsgUuidDuplicate)
		return
	}

	if err := user.Update(false); err != nil {
		common.SysError("failed to update user access token: " + err.Error())
		httpapi.ApiErrorI18n(c, i18n.MsgDatabaseError)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    user.AccessToken,
	})
}

type TransferAffQuotaRequest struct {
	Quota int `json:"quota" binding:"required"`
}

func TransferAffQuota(c *gin.Context) {
	id := c.GetInt("id")
	user, err := userstore.GetUserById(id, true)
	if err != nil {
		common.SysError("failed to get user by id: " + err.Error())
		httpapi.ApiErrorI18n(c, i18n.MsgDatabaseError)
		return
	}
	tran := TransferAffQuotaRequest{}
	if err := c.ShouldBindJSON(&tran); err != nil {
		httpapi.ApiErrorI18n(c, i18n.MsgInvalidRequestBody)
		return
	}
	err = user.TransferAffQuotaToQuota(tran.Quota)
	if err != nil {
		httpapi.ApiErrorI18n(c, i18n.MsgUserTransferFailed, map[string]any{"Error": err.Error()})
		return
	}
	httpapi.ApiSuccessI18n(c, i18n.MsgUserTransferSuccess, nil)
}

func GetAffCode(c *gin.Context) {
	id := c.GetInt("id")
	user, err := userstore.GetUserById(id, true)
	if err != nil {
		common.SysError("failed to get user by id: " + err.Error())
		httpapi.ApiErrorI18n(c, i18n.MsgDatabaseError)
		return
	}
	if user.AffCode == "" {
		user.AffCode = common.GetRandomString(4)
		if err := user.Update(false); err != nil {
			c.JSON(http.StatusOK, gin.H{
				"success": false,
				"message": err.Error(),
			})
			return
		}
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    user.AffCode,
	})
}

func GetSelf(c *gin.Context) {
	id := c.GetInt("id")
	user, err := userstore.GetUserById(id, false)
	if err != nil {
		common.SysError("failed to get user by id: " + err.Error())
		httpapi.ApiErrorI18n(c, i18n.MsgDatabaseError)
		return
	}
	// Hide admin remarks: set to empty to trigger omitempty tag, ensuring the remark field is not included in JSON returned to regular users
	user.Remark = ""

	// 获取用户设置
	userSetting := user.GetSetting()

	// 重新序列化用户设置，确保已移除的字段不会出现在响应中
	settingJSON, err := jsonx.Marshal(userSetting)
	if err != nil {
		common.SysError("failed to marshal user setting: " + err.Error())
		httpapi.ApiErrorI18n(c, i18n.MsgDatabaseError)
		return
	}

	// 构建响应数据
	responseData := map[string]interface{}{
		"id":                    user.Id,
		"username":              user.Username,
		"display_name":          user.DisplayName,
		"role":                  user.Role,
		"status":                user.Status,
		"email":                 user.Email,
		"github_id":             user.GitHubId,
		"group":                 user.Group,
		"quota":                 user.Quota,
		"used_quota":            user.UsedQuota,
		"request_count":         user.RequestCount,
		"aff_code":              user.AffCode,
		"aff_count":             user.AffCount,
		"aff_quota":             user.AffQuota,
		"aff_history_quota":     user.AffHistoryQuota,
		"inviter_id":            user.InviterId,
		"linux_do_id":           user.LinuxDOId,
		"setting":               string(settingJSON),
		"stripe_customer":       user.StripeCustomer,
		"image_converted_count": user.ImageConvertedCount,
		"video_converted_count": user.VideoConvertedCount,
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    responseData,
	})
}

func GetUserModels(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		id = c.GetInt("id")
	}
	user, err := userstore.GetUserCache(id)
	if err != nil {
		common.SysError("failed to get user cache: " + err.Error())
		httpapi.ApiErrorI18n(c, i18n.MsgDatabaseError)
		return
	}
	groups := domaingroup.GetUserUsableGroups(user.Group)
	var models []string
	for group := range groups {
		for _, g := range channelstore.GetGroupEnabledModels(group) {
			if !common.StringsContains(models, g) {
				models = append(models, g)
			}
		}
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    models,
	})
}

type UpdateUserRequest struct {
	Id          int    `json:"id"`
	Username    string `json:"username"`
	Password    string `json:"password"`
	DisplayName string `json:"display_name"`
	Role        int    `json:"role"`
	Quota       *int   `json:"quota"`
	Group       string `json:"group"`
	Remark      string `json:"remark"`
}

func UpdateUser(c *gin.Context) {
	var req UpdateUserRequest
	err := jsonx.DecodeJson(c.Request.Body, &req)
	if err != nil || req.Id == 0 {
		httpapi.ApiErrorI18n(c, i18n.MsgInvalidParams)
		return
	}
	originUser, err := userstore.GetUserById(req.Id, false)
	if err != nil {
		common.SysError("failed to get user by id: " + err.Error())
		httpapi.ApiErrorI18n(c, i18n.MsgDatabaseError)
		return
	}
	quota := originUser.Quota
	if req.Quota != nil {
		quota = *req.Quota
	}
	updatedUser := userstore.User{
		Id:          req.Id,
		Username:    req.Username,
		Password:    req.Password,
		DisplayName: req.DisplayName,
		Role:        req.Role,
		Quota:       quota,
		Group:       req.Group,
		Remark:      req.Remark,
	}
	if updatedUser.Password == "" {
		updatedUser.Password = "$I_LOVE_U" // make Validator happy :)
	}
	if err := security.Validate.Struct(&updatedUser); err != nil {
		httpapi.ApiErrorI18n(c, i18n.MsgUserInputInvalid, map[string]any{"Error": err.Error()})
		return
	}
	myRole := c.GetInt("role")
	if myRole <= originUser.Role && myRole != common.RoleRootUser {
		httpapi.ApiErrorI18n(c, i18n.MsgUserNoPermissionHigherLevel)
		return
	}
	if myRole <= updatedUser.Role && myRole != common.RoleRootUser {
		httpapi.ApiErrorI18n(c, i18n.MsgUserCannotCreateHigherLevel)
		return
	}
	if updatedUser.Password == "$I_LOVE_U" {
		updatedUser.Password = "" // rollback to what it should be
	}
	updatePassword := updatedUser.Password != ""
	if err := updatedUser.Edit(updatePassword); err != nil {
		common.SysError("failed to edit user: " + err.Error())
		httpapi.ApiErrorI18n(c, i18n.MsgDatabaseError)
		return
	}
	invalidateSecuritySensitiveUserCaches(updatedUser.Id)
	if originUser.Quota != updatedUser.Quota {
		userstore.RecordLog(originUser.Id, logstore.LogTypeManage, fmt.Sprintf("管理员将用户额度从 %s修改为 %s", log.LogQuota(originUser.Quota), log.LogQuota(updatedUser.Quota)))
	}
	audit.RecordAudit(c, auditstore.AuditModuleUser, auditstore.AuditActionUpdate, "修改用户: "+updatedUser.Username,
		map[string]interface{}{"id": originUser.Id, "username": originUser.Username, "display_name": originUser.DisplayName, "role": originUser.Role, "status": originUser.Status, "quota": originUser.Quota, "group": originUser.Group},
		map[string]interface{}{"id": updatedUser.Id, "username": updatedUser.Username, "display_name": updatedUser.DisplayName, "role": updatedUser.Role, "quota": updatedUser.Quota, "group": updatedUser.Group, "remark": updatedUser.Remark})
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
	})
}

func UpdateSelf(c *gin.Context) {
	var user userstore.User
	err := jsonx.DecodeJson(c.Request.Body, &user)
	if err != nil {
		httpapi.ApiErrorI18n(c, i18n.MsgInvalidParams)
		return
	}

	if user.Password == "" {
		user.Password = "$I_LOVE_U" // make Validator happy :)
	}
	if err := security.Validate.Struct(&user); err != nil {
		httpapi.ApiErrorI18n(c, i18n.MsgInvalidInput)
		return
	}

	cleanUser := userstore.User{
		Id:          c.GetInt("id"),
		Username:    user.Username,
		Password:    user.Password,
		DisplayName: user.DisplayName,
	}
	if user.Password == "$I_LOVE_U" {
		user.Password = "" // rollback to what it should be
		cleanUser.Password = ""
	}
	updatePassword, err := checkUpdatePassword(c, user.OriginalPassword, user.Password, cleanUser.Id)
	if err != nil {
		common.SysError("failed to check update password: " + err.Error())
		httpapi.ApiErrorI18n(c, i18n.MsgDatabaseError)
		return
	}
	if err := cleanUser.Update(updatePassword); err != nil {
		common.SysError("failed to update user: " + err.Error())
		httpapi.ApiErrorI18n(c, i18n.MsgDatabaseError)
		return
	}
	invalidateSecuritySensitiveUserCaches(cleanUser.Id)

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
	})
}

func checkUpdatePassword(c *gin.Context, originalPassword string, newPassword string, userId int) (updatePassword bool, err error) {
	var currentUser *userstore.User
	currentUser, err = userstore.GetUserById(userId, true)
	if err != nil {
		return
	}

	// 密码不为空,需要验证原密码
	// 支持第一次账号绑定时原密码为空的情况
	if !security.ValidatePasswordAndHash(originalPassword, currentUser.Password) && currentUser.Password != "" {
		err = fmt.Errorf("%s", i18n.T(c, i18n.MsgUserOriginalPasswordError))
		return
	}
	if newPassword == "" {
		return
	}
	updatePassword = true
	return
}

func DeleteUser(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		httpapi.ApiErrorI18n(c, i18n.MsgInvalidParams)
		return
	}
	originUser, err := userstore.GetUserById(id, false)
	if err != nil {
		common.SysError("failed to get user by id: " + err.Error())
		httpapi.ApiErrorI18n(c, i18n.MsgDatabaseError)
		return
	}
	myRole := c.GetInt("role")
	if myRole <= originUser.Role {
		httpapi.ApiErrorI18n(c, i18n.MsgUserNoPermissionHigherLevel)
		return
	}
	err = userstore.HardDeleteUserById(id)
	if err != nil {
		common.SysError("failed to hard delete user: " + err.Error())
		httpapi.ApiErrorI18n(c, i18n.MsgDatabaseError)
		return
	}
	invalidateSecuritySensitiveUserCaches(id)
	audit.RecordAudit(c, auditstore.AuditModuleUser, auditstore.AuditActionDelete, "删除用户: "+originUser.Username,
		map[string]interface{}{"id": originUser.Id, "username": originUser.Username, "display_name": originUser.DisplayName, "role": originUser.Role, "status": originUser.Status}, nil)
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
	})
}

func DeleteSelf(c *gin.Context) {
	id := c.GetInt("id")
	user, _ := userstore.GetUserById(id, false)

	if user.Role == common.RoleRootUser {
		httpapi.ApiErrorI18n(c, i18n.MsgUserCannotDeleteRootUser)
		return
	}

	err := userstore.DeleteUserById(id)
	if err != nil {
		common.SysError("failed to delete user: " + err.Error())
		httpapi.ApiErrorI18n(c, i18n.MsgDatabaseError)
		return
	}
	invalidateSecuritySensitiveUserCaches(id)
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
	})
}

func CreateUser(c *gin.Context) {
	var user userstore.User
	err := jsonx.DecodeJson(c.Request.Body, &user)
	user.Username = strings.TrimSpace(user.Username)
	if err != nil || user.Username == "" || user.Password == "" {
		httpapi.ApiErrorI18n(c, i18n.MsgInvalidParams)
		return
	}
	if err := security.Validate.Struct(&user); err != nil {
		httpapi.ApiErrorI18n(c, i18n.MsgUserInputInvalid, map[string]any{"Error": err.Error()})
		return
	}
	if user.DisplayName == "" {
		user.DisplayName = user.Username
	}
	myRole := c.GetInt("role")
	if user.Role >= myRole {
		httpapi.ApiErrorI18n(c, i18n.MsgUserCannotCreateHigherLevel)
		return
	}
	// Even for admin users, we cannot fully trust them!
	cleanUser := userstore.User{
		Username:    user.Username,
		Password:    user.Password,
		DisplayName: user.DisplayName,
		Role:        user.Role, // 保持管理员设置的角色
	}
	if err := cleanUser.Insert(0); err != nil {
		common.SysError("failed to insert user: " + err.Error())
		httpapi.ApiErrorI18n(c, i18n.MsgDatabaseError)
		return
	}

	audit.RecordAudit(c, auditstore.AuditModuleUser, auditstore.AuditActionCreate, "新增用户: "+cleanUser.Username, nil, map[string]interface{}{"username": cleanUser.Username, "display_name": cleanUser.DisplayName, "role": cleanUser.Role})
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
	})
}

type ManageRequest struct {
	Id     int    `json:"id"`
	Action string `json:"action"`
	Mode   string `json:"mode,omitempty"`   // add, subtract, override (for add_quota)
	Value  int    `json:"value,omitempty"`  // quota value in quota units (for add_quota)
	Remark string `json:"remark,omitempty"` // optional ban note stored on the user (for disable), ≤255 chars
}

// ManageUser Only admin user can do this
func ManageUser(c *gin.Context) {
	var req ManageRequest
	err := jsonx.DecodeJson(c.Request.Body, &req)

	if err != nil {
		httpapi.ApiErrorI18n(c, i18n.MsgInvalidParams)
		return
	}
	req.Remark = strings.TrimSpace(req.Remark)
	if len(req.Remark) > 255 {
		httpapi.ApiErrorI18n(c, i18n.MsgUserBanRemarkTooLong)
		return
	}
	user := userstore.User{
		Id: req.Id,
	}
	// Fill attributes
	dbstore.DB.Unscoped().Where(&user).First(&user)
	if user.Id == 0 {
		httpapi.ApiErrorI18n(c, i18n.MsgUserNotExists)
		return
	}
	myRole := c.GetInt("role")
	if myRole <= user.Role && myRole != common.RoleRootUser {
		httpapi.ApiErrorI18n(c, i18n.MsgUserNoPermissionHigherLevel)
		return
	}
	originRemark := user.Remark
	switch req.Action {
	case "disable":
		user.Status = common.UserStatusDisabled
		if user.Role == common.RoleRootUser {
			httpapi.ApiErrorI18n(c, i18n.MsgUserCannotDisableRootUser)
			return
		}
		if req.Remark != "" {
			user.Remark = req.Remark
		}
	case "enable":
		user.Status = common.UserStatusEnabled
	case "delete":
		if user.Role == common.RoleRootUser {
			httpapi.ApiErrorI18n(c, i18n.MsgUserCannotDeleteRootUser)
			return
		}
		if err := user.Delete(); err != nil {
			c.JSON(http.StatusOK, gin.H{
				"success": false,
				"message": err.Error(),
			})
			return
		}
	case "promote":
		if myRole != common.RoleRootUser {
			httpapi.ApiErrorI18n(c, i18n.MsgUserAdminCannotPromote)
			return
		}
		if user.Role >= common.RoleAdminUser {
			httpapi.ApiErrorI18n(c, i18n.MsgUserAlreadyAdmin)
			return
		}
		user.Role = common.RoleAdminUser
	case "demote":
		if user.Role == common.RoleRootUser {
			httpapi.ApiErrorI18n(c, i18n.MsgUserCannotDemoteRootUser)
			return
		}
		if user.Role == common.RoleCommonUser {
			httpapi.ApiErrorI18n(c, i18n.MsgUserAlreadyCommon)
			return
		}
		user.Role = common.RoleCommonUser
	case "add_quota":
		if req.Value < 0 {
			httpapi.ApiErrorI18n(c, i18n.MsgInvalidParams)
			return
		}
		originQuota := user.Quota
		switch req.Mode {
		case "add":
			if addWouldOverflowInt(originQuota, req.Value) {
				httpapi.ApiErrorI18n(c, i18n.MsgInvalidParams)
				return
			}
			if err := userstore.IncreaseUserQuota(user.Id, req.Value, true); err != nil {
				common.SysError("failed to increase user quota: " + err.Error())
				httpapi.ApiErrorI18n(c, i18n.MsgDatabaseError)
				return
			}
			user.Quota = originQuota + req.Value
		case "subtract":
			if req.Value > user.Quota {
				httpapi.ApiErrorI18n(c, i18n.MsgInvalidParams)
				return
			}
			if err := userstore.DecreaseUserQuota(user.Id, req.Value); err != nil {
				common.SysError("failed to decrease user quota: " + err.Error())
				httpapi.ApiErrorI18n(c, i18n.MsgDatabaseError)
				return
			}
			user.Quota = originQuota - req.Value
		case "override":
			maxQuotaLimit := common.QuotaUpperLimit(100_000_000)
			if req.Value > maxQuotaLimit {
				httpapi.ApiErrorI18n(c, i18n.MsgInvalidParams)
				return
			}
			if err := dbstore.DB.Model(&userstore.User{}).Where("id = ?", user.Id).Update("quota", req.Value).Error; err != nil {
				common.SysError("failed to override user quota: " + err.Error())
				httpapi.ApiErrorI18n(c, i18n.MsgDatabaseError)
				return
			}
			user.Quota = req.Value
		default:
			httpapi.ApiErrorI18n(c, i18n.MsgInvalidParams)
			return
		}
		userstore.RecordLog(user.Id, logstore.LogTypeManage, fmt.Sprintf("管理员调整用户额度从 %s 到 %s", log.LogQuota(originQuota), log.LogQuota(user.Quota)))
		invalidateSecuritySensitiveUserCaches(user.Id)
		audit.RecordAudit(c, auditstore.AuditModuleUser, auditstore.AuditActionUpdate, "管理用户: 调整额度",
			map[string]interface{}{"id": user.Id, "username": user.Username, "quota": originQuota},
			map[string]interface{}{"id": user.Id, "username": user.Username, "quota": user.Quota, "mode": req.Mode, "value": req.Value})
		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"message": "",
			"data": userstore.User{
				Id:        user.Id,
				Username:  user.Username,
				Quota:     user.Quota,
				UsedQuota: user.UsedQuota,
			},
		})
		return
	}

	if err := user.Update(false); err != nil {
		common.SysError("failed to update user: " + err.Error())
		httpapi.ApiErrorI18n(c, i18n.MsgDatabaseError)
		return
	}
	invalidateSecuritySensitiveUserCaches(user.Id)
	auditAction := auditstore.AuditActionUpdate
	if req.Action == "delete" {
		auditAction = auditstore.AuditActionDelete
	}
	var auditBefore map[string]interface{}
	if req.Action == "disable" {
		auditBefore = map[string]interface{}{"username": user.Username, "remark": originRemark}
	}
	audit.RecordAudit(c, auditstore.AuditModuleUser, auditAction, "管理用户: "+req.Action, auditBefore, map[string]interface{}{"username": user.Username, "action": req.Action, "remark": user.Remark})
	clearUser := userstore.User{
		Role:   user.Role,
		Status: user.Status,
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    clearUser,
	})
}

type emailBindRequest struct {
	Email string `json:"email"`
	Code  string `json:"code"`
}

func EmailBind(c *gin.Context) {
	var req emailBindRequest
	if err := jsonx.DecodeJson(c.Request.Body, &req); err != nil {
		httpapi.ApiErrorI18n(c, i18n.MsgInvalidRequestBody)
		return
	}
	email := strings.TrimSpace(req.Email)
	code := strings.TrimSpace(req.Code)
	if email == "" || code == "" {
		httpapi.ApiErrorI18n(c, i18n.MsgInvalidRequestBody)
		return
	}
	if !security.VerifyCodeWithKey(email, code, security.EmailVerificationPurpose) {
		httpapi.ApiErrorI18n(c, i18n.MsgUserVerificationCodeError)
		return
	}
	session := sessions.Default(c)
	id := session.Get("id")
	user := userstore.User{
		Id: id.(int),
	}
	err := user.FillUserById()
	if err != nil {
		common.SysError("failed to fill user by id: " + err.Error())
		httpapi.ApiErrorI18n(c, i18n.MsgDatabaseError)
		return
	}
	// Re-check the authoritative status from DB; the session snapshot may be
	// stale if an admin disabled this user after login.
	if user.Status != common.UserStatusEnabled {
		session.Clear()
		_ = session.Save()
		httpapi.ApiErrorI18n(c, i18n.MsgUserBanned)
		return
	}
	user.Email = email
	// no need to check if this email already taken, because we have used verification code to check it
	err = user.Update(false)
	if err != nil {
		common.SysError("failed to update user email: " + err.Error())
		httpapi.ApiErrorI18n(c, i18n.MsgDatabaseError)
		return
	}
	invalidateSecuritySensitiveUserCaches(user.Id)
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
	})
}

type topUpRequest struct {
	Key string `json:"key"`
}

var topUpLocks sync.Map
var topUpCreateLock sync.Mutex

type topUpTryLock struct {
	ch chan struct{}
}

func newTopUpTryLock() *topUpTryLock {
	return &topUpTryLock{ch: make(chan struct{}, 1)}
}

func (l *topUpTryLock) TryLock() bool {
	select {
	case l.ch <- struct{}{}:
		return true
	default:
		return false
	}
}

func (l *topUpTryLock) Unlock() {
	select {
	case <-l.ch:
	default:
	}
}

func getTopUpLock(userID int) *topUpTryLock {
	if v, ok := topUpLocks.Load(userID); ok {
		return v.(*topUpTryLock)
	}
	topUpCreateLock.Lock()
	defer topUpCreateLock.Unlock()
	if v, ok := topUpLocks.Load(userID); ok {
		return v.(*topUpTryLock)
	}
	l := newTopUpTryLock()
	topUpLocks.Store(userID, l)
	return l
}

func TopUp(c *gin.Context) {
	id := c.GetInt("id")
	lock := getTopUpLock(id)
	if !lock.TryLock() {
		httpapi.ApiErrorI18n(c, i18n.MsgUserTopUpProcessing)
		return
	}
	defer lock.Unlock()
	req := topUpRequest{}
	err := c.ShouldBindJSON(&req)
	if err != nil {
		httpapi.ApiErrorI18n(c, i18n.MsgInvalidRequestBody)
		return
	}
	quota, err := redemptionstore.Redeem(req.Key, id)
	if err != nil {
		if errors.Is(err, redemptionstore.ErrRedeemFailed) {
			httpapi.ApiErrorI18n(c, i18n.MsgRedeemFailed)
			return
		}
		common.SysError("failed to redeem: " + err.Error())
		httpapi.ApiErrorI18n(c, i18n.MsgDatabaseError)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    quota,
	})
}

type UpdateUserSettingRequest struct {
	QuotaWarningType      string  `json:"notify_type"`
	QuotaWarningThreshold float64 `json:"quota_warning_threshold"`
	WebhookUrl            string  `json:"webhook_url,omitempty"`
	WebhookSecret         string  `json:"webhook_secret,omitempty"`
	NotificationEmail     string  `json:"notification_email,omitempty"`
	BarkUrl               string  `json:"bark_url,omitempty"`
	GotifyUrl             string  `json:"gotify_url,omitempty"`
	GotifyToken           string  `json:"gotify_token,omitempty"`
	GotifyPriority        int     `json:"gotify_priority,omitempty"`
}

func UpdateUserSetting(c *gin.Context) {
	var req UpdateUserSettingRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpapi.ApiErrorI18n(c, i18n.MsgInvalidParams)
		return
	}

	// 验证预警类型
	if req.QuotaWarningType != shared.NotifyTypeEmail && req.QuotaWarningType != shared.NotifyTypeWebhook && req.QuotaWarningType != shared.NotifyTypeBark && req.QuotaWarningType != shared.NotifyTypeGotify {
		httpapi.ApiErrorI18n(c, i18n.MsgSettingInvalidType)
		return
	}

	// 验证预警阈值
	if req.QuotaWarningThreshold <= 0 {
		httpapi.ApiErrorI18n(c, i18n.MsgQuotaThresholdGtZero)
		return
	}

	// 如果是webhook类型,验证webhook地址
	if req.QuotaWarningType == shared.NotifyTypeWebhook {
		if req.WebhookUrl == "" {
			httpapi.ApiErrorI18n(c, i18n.MsgSettingWebhookEmpty)
			return
		}
		// 验证URL格式
		if _, err := url.ParseRequestURI(req.WebhookUrl); err != nil {
			httpapi.ApiErrorI18n(c, i18n.MsgSettingWebhookInvalid)
			return
		}
	}

	// 如果是邮件类型，验证邮箱地址
	if req.QuotaWarningType == shared.NotifyTypeEmail && req.NotificationEmail != "" {
		// 验证邮箱格式
		if !strings.Contains(req.NotificationEmail, "@") {
			httpapi.ApiErrorI18n(c, i18n.MsgSettingEmailInvalid)
			return
		}
	}

	// 如果是Bark类型，验证Bark URL
	if req.QuotaWarningType == shared.NotifyTypeBark {
		if req.BarkUrl == "" {
			httpapi.ApiErrorI18n(c, i18n.MsgSettingBarkUrlEmpty)
			return
		}
		// 验证URL格式
		if _, err := url.ParseRequestURI(req.BarkUrl); err != nil {
			httpapi.ApiErrorI18n(c, i18n.MsgSettingBarkUrlInvalid)
			return
		}
		// 检查是否是HTTP或HTTPS
		if !strings.HasPrefix(req.BarkUrl, "https://") && !strings.HasPrefix(req.BarkUrl, "http://") {
			httpapi.ApiErrorI18n(c, i18n.MsgSettingUrlMustHttp)
			return
		}
	}

	// 如果是Gotify类型，验证Gotify URL和Token
	if req.QuotaWarningType == shared.NotifyTypeGotify {
		if req.GotifyUrl == "" {
			httpapi.ApiErrorI18n(c, i18n.MsgSettingGotifyUrlEmpty)
			return
		}
		if req.GotifyToken == "" {
			httpapi.ApiErrorI18n(c, i18n.MsgSettingGotifyTokenEmpty)
			return
		}
		// 验证URL格式
		if _, err := url.ParseRequestURI(req.GotifyUrl); err != nil {
			httpapi.ApiErrorI18n(c, i18n.MsgSettingGotifyUrlInvalid)
			return
		}
		// 检查是否是HTTP或HTTPS
		if !strings.HasPrefix(req.GotifyUrl, "https://") && !strings.HasPrefix(req.GotifyUrl, "http://") {
			httpapi.ApiErrorI18n(c, i18n.MsgSettingUrlMustHttp)
			return
		}
	}

	userId := c.GetInt("id")
	user, err := userstore.GetUserById(userId, true)
	if err != nil {
		common.SysError("failed to get user by id: " + err.Error())
		httpapi.ApiErrorI18n(c, i18n.MsgDatabaseError)
		return
	}

	// 构建设置
	settings := shared.UserSetting{
		NotifyType:            req.QuotaWarningType,
		QuotaWarningThreshold: req.QuotaWarningThreshold,
	}

	// 如果是webhook类型,添加webhook相关设置
	if req.QuotaWarningType == shared.NotifyTypeWebhook {
		settings.WebhookUrl = req.WebhookUrl
		if req.WebhookSecret != "" {
			settings.WebhookSecret = req.WebhookSecret
		}
	}

	// 如果提供了通知邮箱，添加到设置中
	if req.QuotaWarningType == shared.NotifyTypeEmail && req.NotificationEmail != "" {
		settings.NotificationEmail = req.NotificationEmail
	}

	// 如果是Bark类型，添加Bark URL到设置中
	if req.QuotaWarningType == shared.NotifyTypeBark {
		settings.BarkUrl = req.BarkUrl
	}

	// 如果是Gotify类型，添加Gotify配置到设置中
	if req.QuotaWarningType == shared.NotifyTypeGotify {
		settings.GotifyUrl = req.GotifyUrl
		settings.GotifyToken = req.GotifyToken
		// Gotify优先级范围0-10，超出范围则使用默认值5
		if req.GotifyPriority < 0 || req.GotifyPriority > 10 {
			settings.GotifyPriority = 5
		} else {
			settings.GotifyPriority = req.GotifyPriority
		}
	}

	// 更新用户设置
	user.SetSetting(settings)
	if err := user.Update(false); err != nil {
		httpapi.ApiErrorI18n(c, i18n.MsgUpdateFailed)
		return
	}
	invalidateSecuritySensitiveUserCaches(userId)

	httpapi.ApiSuccessI18n(c, i18n.MsgSettingSaved, nil)
}
