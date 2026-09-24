package userstore

import (
	"errors"
	"fmt"
	"github.com/NookMux/NookMux/internal/common"
	"github.com/NookMux/NookMux/internal/domain/shared"
	"github.com/NookMux/NookMux/internal/infra/log"
	"github.com/NookMux/NookMux/internal/infra/redis"
	"github.com/NookMux/NookMux/internal/infra/runtime"
	"github.com/NookMux/NookMux/internal/infra/security"
	"github.com/NookMux/NookMux/internal/store/db"
	"github.com/NookMux/NookMux/internal/store/log"
	"github.com/NookMux/NookMux/internal/store/token"
	"github.com/NookMux/NookMux/pkg/jsonx"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"strconv"
	"strings"
)

// User if you add sensitive fields, don't forget to clean them in setupLogin function.
// Otherwise, the sensitive information will be saved on local storage in plain text!
type User struct {
	Id       int    `json:"id"`
	Username string `json:"username" gorm:"unique;index" validate:"max=20"`
	// Password 存 bcrypt hash。查询层已 Omit("password")，序列化用 omitempty
	// 避免输出空 "password" 字段；请求体绑定（注册/创建/改密）依赖该 json 标签，不能改为 "-"。
	Password         string         `json:"password,omitempty" gorm:"not null;" validate:"min=8,max=20"`
	OriginalPassword string         `json:"original_password" gorm:"-:all"` // this field is only for Password change verification, don't save it to database!
	DisplayName      string         `json:"display_name" gorm:"index" validate:"max=20"`
	Role             int            `json:"role" gorm:"type:int;default:1"`   // admin, common
	Status           int            `json:"status" gorm:"type:int;default:1"` // enabled, disabled
	Email            string         `json:"email" gorm:"index" validate:"max=50"`
	GitHubId         string         `json:"github_id" gorm:"column:github_id;index"`
	VerificationCode string         `json:"verification_code" gorm:"-:all"`                         // this field is only for Email verification, don't save it to database!
	AccessToken      *string        `json:"-" gorm:"type:char(32);column:access_token;uniqueIndex"` // this token is for system management
	Quota            int            `json:"quota" gorm:"type:int;default:0"`
	UsedQuota        int            `json:"used_quota" gorm:"type:int;default:0;column:used_quota"` // used quota
	RequestCount     int            `json:"request_count" gorm:"type:int;default:0;"`               // request number
	Group            string         `json:"group" gorm:"type:varchar(64);default:'default'"`
	AffCode          string         `json:"aff_code" gorm:"type:varchar(32);column:aff_code;uniqueIndex"`
	AffCount         int            `json:"aff_count" gorm:"type:int;default:0;column:aff_count"`
	AffQuota         int            `json:"aff_quota" gorm:"type:int;default:0;column:aff_quota"`           // 邀请剩余额度
	AffHistoryQuota  int            `json:"aff_history_quota" gorm:"type:int;default:0;column:aff_history"` // 邀请历史额度
	InviterId        int            `json:"inviter_id" gorm:"type:int;column:inviter_id;index"`
	DeletedAt        gorm.DeletedAt `gorm:"index"`
	LinuxDOId        string         `json:"linux_do_id" gorm:"column:linux_do_id;index"`
	// Setting 存储用户通知配置（WebhookSecret、GotifyToken、BarkUrl 等
	// 通知凭证）。JSON 序列化不输出（json:"-"），admin 用户列表/搜索/详情
	// 不得批量暴露所有用户的通知凭证；用户本人通过 GetSelf（显式重建
	// 响应）和 PUT /api/user/setting 读写。
	Setting             string `json:"-" gorm:"type:text;column:setting"`
	Remark              string `json:"remark,omitempty" gorm:"type:varchar(255)" validate:"max=255"`
	StripeCustomer      string `json:"stripe_customer" gorm:"type:varchar(64);column:stripe_customer;index"`
	ImageConvertedCount int    `json:"image_converted_count" gorm:"type:int;default:0;column:image_converted_count"`
	VideoConvertedCount int    `json:"video_converted_count" gorm:"type:int;default:0;column:video_converted_count"`
	CreatedAt           int64  `json:"created_at" gorm:"bigint;index"`
	LastLoginAt         int64  `json:"last_login_at" gorm:"bigint"`
}

func (user *User) ToBaseUser() *UserBase {
	cache := &UserBase{
		Id:       user.Id,
		Group:    user.Group,
		Quota:    user.Quota,
		Status:   user.Status,
		Role:     user.Role,
		Username: user.Username,
		Setting:  user.Setting,
		Email:    user.Email,
	}
	return cache
}

func (user *User) GetAccessToken() string {
	if user.AccessToken == nil {
		return ""
	}
	return *user.AccessToken
}

func (user *User) SetAccessToken(token string) {
	user.AccessToken = &token
}

func (user *User) GetSetting() shared.UserSetting {
	setting := shared.UserSetting{}
	if user.Setting != "" {
		err := jsonx.Unmarshal([]byte(user.Setting), &setting)
		if err != nil {
			common.SysLog("failed to unmarshal setting: " + err.Error())
		}
	}
	return setting
}

func (user *User) SetSetting(setting shared.UserSetting) {
	settingBytes, err := jsonx.Marshal(setting)
	if err != nil {
		common.SysLog("failed to marshal setting: " + err.Error())
		return
	}
	user.Setting = string(settingBytes)
}

// CheckUserExistOrDeleted check if user exist or deleted, if not exist, return false, nil, if deleted or exist, return true, nil
func CheckUserExistOrDeleted(username string, email string) (bool, error) {
	var user User

	// err := DB.Unscoped().First(&user, "username = ? or email = ?", username, email).Error
	// check email if empty
	var err error
	if email == "" {
		err = dbstore.DB.Unscoped().First(&user, "username = ?", username).Error
	} else {
		err = dbstore.DB.Unscoped().First(&user, "username = ? or email = ?", username, email).Error
	}
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			// not exist, return false, nil
			return false, nil
		}
		// other error, return false, err
		return false, err
	}
	// exist, return true, nil
	return true, nil
}

func GetMaxUserId() int {
	var user User
	dbstore.DB.Unscoped().Last(&user)
	return user.Id
}

func GetAllUsers(pageInfo *common.PageInfo) (users []*User, total int64, err error) {
	// Start transaction
	tx := dbstore.DB.Begin()
	if tx.Error != nil {
		return nil, 0, tx.Error
	}
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	// Get total count within transaction
	err = tx.Unscoped().Model(&User{}).Count(&total).Error
	if err != nil {
		tx.Rollback()
		return nil, 0, err
	}

	// Get paginated users within same transaction
	err = tx.Unscoped().Order("id desc").Limit(pageInfo.GetPageSize()).Offset(pageInfo.GetStartIdx()).Omit("password", "access_token").Find(&users).Error
	if err != nil {
		tx.Rollback()
		return nil, 0, err
	}

	// Commit transaction
	if err = tx.Commit().Error; err != nil {
		return nil, 0, err
	}

	return users, total, nil
}

func SearchUsers(keyword string, group string, status string, startIdx int, num int) ([]*User, int64, error) {
	return SearchUsersAdvanced(keyword, "", "", "", "", status, "", group, startIdx, num)
}

func SearchUsersAdvanced(username, displayName, email, linuxDoId, githubId, status, role, group string, startIdx, num int) ([]*User, int64, error) {
	var users []*User
	var total int64

	tx := dbstore.DB.Begin()
	if tx.Error != nil {
		return nil, 0, tx.Error
	}
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	query := tx.Unscoped().Model(&User{})

	if username != "" {
		query = query.Where("username LIKE ?", "%"+username+"%")
	}
	if displayName != "" {
		query = query.Where("display_name LIKE ?", "%"+displayName+"%")
	}
	if email != "" {
		query = query.Where("email LIKE ?", "%"+email+"%")
	}
	if linuxDoId != "" {
		query = query.Where("linux_do_id LIKE ?", "%"+linuxDoId+"%")
	}
	if githubId != "" {
		query = query.Where("github_id LIKE ?", "%"+githubId+"%")
	}

	switch status {
	case "1":
		query = query.Where("status = ? AND deleted_at IS NULL", common.UserStatusEnabled)
	case "2":
		query = query.Where("status = ? AND deleted_at IS NULL", common.UserStatusDisabled)
	case "3":
		query = query.Where("deleted_at IS NOT NULL")
	}

	if role != "" {
		if roleInt, err := strconv.Atoi(role); err == nil {
			query = query.Where("role = ?", roleInt)
		}
	}

	if group != "" {
		query = query.Where(dbstore.CommonGroupCol+" = ?", group)
	}

	err := query.Count(&total).Error
	if err != nil {
		tx.Rollback()
		return nil, 0, err
	}

	err = query.Omit("password", "access_token").Order("id desc").Limit(num).Offset(startIdx).Find(&users).Error
	if err != nil {
		tx.Rollback()
		return nil, 0, err
	}

	if err = tx.Commit().Error; err != nil {
		return nil, 0, err
	}

	return users, total, nil
}

func GetUserById(id int, selectAll bool) (*User, error) {
	if id == 0 {
		return nil, errors.New("id 为空！")
	}
	user := User{Id: id}
	var err error
	if selectAll {
		err = dbstore.DB.First(&user, "id = ?", id).Error
	} else {
		err = dbstore.DB.Omit("password", "access_token").First(&user, "id = ?", id).Error
	}
	return &user, err
}

func GetUserIdByAffCode(affCode string) (int, error) {
	if affCode == "" {
		return 0, errors.New("affCode 为空！")
	}
	var user User
	err := dbstore.DB.Select("id").First(&user, "aff_code = ?", affCode).Error
	return user.Id, err
}

func DeleteUserById(id int) (err error) {
	if id == 0 {
		return errors.New("id 为空！")
	}
	user := User{Id: id}
	return user.Delete()
}

func HardDeleteUserById(id int) error {
	if id == 0 {
		return errors.New("id 为空！")
	}
	if err := dbstore.DB.Unscoped().Delete(&User{}, "id = ?", id).Error; err != nil {
		return err
	}
	if err := InvalidateUserCache(id); err != nil {
		return err
	}
	return tokenstore.InvalidateUserTokensCache(id)
}

func inviteUser(inviterId int) (err error) {
	user, err := GetUserById(inviterId, true)
	if err != nil {
		return err
	}
	user.AffCount++
	user.AffQuota += common.QuotaForInviter
	user.AffHistoryQuota += common.QuotaForInviter
	return dbstore.DB.Save(user).Error
}

func (user *User) TransferAffQuotaToQuota(quota int) error {
	// 检查quota是否小于最小额度
	if float64(quota) < common.QuotaPerUnit {
		return fmt.Errorf("转移额度最小为%s！", log.LogQuota(int(common.QuotaPerUnit)))
	}

	// 开始数据库事务
	tx := dbstore.DB.Begin()
	if tx.Error != nil {
		return tx.Error
	}
	defer tx.Rollback() // 确保在函数退出时事务能回滚

	// 加锁查询用户以确保数据一致性
	err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&user, user.Id).Error
	if err != nil {
		return err
	}

	// 再次检查用户的AffQuota是否足够
	if user.AffQuota < quota {
		return errors.New("邀请额度不足！")
	}

	// 更新用户额度
	user.AffQuota -= quota
	user.Quota += quota

	// 保存用户状态
	if err := tx.Save(user).Error; err != nil {
		return err
	}

	// 提交事务
	return tx.Commit().Error
}

func (user *User) Insert(inviterId int) error {
	var err error
	if user.Password != "" {
		user.Password, err = security.Password2Hash(user.Password)
		if err != nil {
			return err
		}
	}
	user.Quota = common.QuotaForNewUser
	//user.SetAccessToken(common.GetUUID())
	user.AffCode = common.GetRandomString(4)
	user.CreatedAt = common.GetTimestamp()

	// 初始化用户设置，包括默认的边栏配置
	if user.Setting == "" {
		defaultSetting := shared.UserSetting{}
		// 这里暂时不设置SidebarModules，因为需要在用户创建后根据角色设置
		user.SetSetting(defaultSetting)
	}

	result := dbstore.DB.Create(user)
	if result.Error != nil {
		return result.Error
	}

	if common.QuotaForNewUser > 0 {
		RecordLog(user.Id, logstore.LogTypeSystem, fmt.Sprintf("新用户注册赠送 %s", log.LogQuota(common.QuotaForNewUser)))
	}
	if inviterId != 0 {
		if common.QuotaForInvitee > 0 {
			_ = IncreaseUserQuota(user.Id, common.QuotaForInvitee, true)
			RecordLog(user.Id, logstore.LogTypeSystem, fmt.Sprintf("使用邀请码赠送 %s", log.LogQuota(common.QuotaForInvitee)))
		}
		if common.QuotaForInviter > 0 {
			//_ = IncreaseUserQuota(inviterId, common.QuotaForInviter)
			RecordLog(inviterId, logstore.LogTypeSystem, fmt.Sprintf("邀请用户赠送 %s", log.LogQuota(common.QuotaForInviter)))
			_ = inviteUser(inviterId)
		}
	}
	return nil
}

// InsertWithTx inserts a new user within an existing transaction.
// This is used for OAuth registration where user creation and binding need to be atomic.
// Post-creation tasks (sidebar config, logs, inviter rewards) are handled after the transaction commits.
func (user *User) InsertWithTx(tx *gorm.DB, inviterId int) error {
	var err error
	if user.Password != "" {
		user.Password, err = security.Password2Hash(user.Password)
		if err != nil {
			return err
		}
	}
	user.Quota = common.QuotaForNewUser
	user.AffCode = common.GetRandomString(4)
	user.CreatedAt = common.GetTimestamp()

	// 初始化用户设置
	if user.Setting == "" {
		defaultSetting := shared.UserSetting{}
		user.SetSetting(defaultSetting)
	}

	result := tx.Create(user)
	if result.Error != nil {
		return result.Error
	}

	return nil
}

// FinalizeOAuthUserCreation performs post-transaction tasks for OAuth user creation.
// This should be called after the transaction commits successfully.
func (user *User) FinalizeOAuthUserCreation(inviterId int) {
	if common.QuotaForNewUser > 0 {
		RecordLog(user.Id, logstore.LogTypeSystem, fmt.Sprintf("新用户注册赠送 %s", log.LogQuota(common.QuotaForNewUser)))
	}
	if inviterId != 0 {
		if common.QuotaForInvitee > 0 {
			_ = IncreaseUserQuota(user.Id, common.QuotaForInvitee, true)
			RecordLog(user.Id, logstore.LogTypeSystem, fmt.Sprintf("使用邀请码赠送 %s", log.LogQuota(common.QuotaForInvitee)))
		}
		if common.QuotaForInviter > 0 {
			RecordLog(inviterId, logstore.LogTypeSystem, fmt.Sprintf("邀请用户赠送 %s", log.LogQuota(common.QuotaForInviter)))
			_ = inviteUser(inviterId)
		}
	}
}

// InsertBannedPlaceholder inserts an admin pre-created, already-banned placeholder user
// (used by /api/user/ban to block OAuth identifiers of users who never logged in).
// Unlike Insert/InsertWithTx it grants no new-user quota and writes no bonus logs.
// The placeholder has no password (empty string is a valid not-null value, same as
// OAuth-registered users) so it can never pass password login.
func (user *User) InsertBannedPlaceholder() error {
	user.Quota = 0
	user.AffCode = common.GetRandomString(4)
	user.CreatedAt = common.GetTimestamp()
	if user.Setting == "" {
		user.SetSetting(shared.UserSetting{})
	}
	return dbstore.DB.Create(user).Error
}

func (user *User) Update(updatePassword bool) error {
	var err error
	if updatePassword {
		user.Password, err = security.Password2Hash(user.Password)
		if err != nil {
			return err
		}
	}
	newUser := *user
	dbstore.DB.First(&user, user.Id)
	if err = dbstore.DB.Model(user).Updates(newUser).Error; err != nil {
		return err
	}

	// Update cache
	return updateUserCache(*user)
}

func (user *User) Edit(updatePassword bool) error {
	var err error
	if updatePassword {
		user.Password, err = security.Password2Hash(user.Password)
		if err != nil {
			return err
		}
	}

	newUser := *user
	updates := map[string]interface{}{
		"username":     newUser.Username,
		"display_name": newUser.DisplayName,
		"group":        newUser.Group,
		"quota":        newUser.Quota,
		"remark":       newUser.Remark,
	}
	if updatePassword {
		updates["password"] = newUser.Password
	}

	dbstore.DB.First(&user, user.Id)
	if err = dbstore.DB.Model(user).Updates(updates).Error; err != nil {
		return err
	}

	// Update cache
	return updateUserCache(*user)
}

func (user *User) Delete() error {
	if user.Id == 0 {
		return errors.New("id 为空！")
	}
	if err := dbstore.DB.Delete(user).Error; err != nil {
		return err
	}

	if err := InvalidateUserCache(user.Id); err != nil {
		return err
	}
	return tokenstore.InvalidateUserTokensCache(user.Id)
}

func (user *User) HardDelete() error {
	if user.Id == 0 {
		return errors.New("id 为空！")
	}
	if err := dbstore.DB.Unscoped().Delete(user).Error; err != nil {
		return err
	}
	if err := InvalidateUserCache(user.Id); err != nil {
		return err
	}
	return tokenstore.InvalidateUserTokensCache(user.Id)
}

// ValidateAndFill check password & user status
func (user *User) ValidateAndFill() (err error) {
	// When querying with struct, GORM will only query with non-zero fields,
	// that means if your field's value is 0, '', false or other zero values,
	// it won't be used to build query conditions
	password := user.Password
	username := strings.TrimSpace(user.Username)
	if username == "" || password == "" {
		return dbstore.ErrUserEmptyCredentials
	}
	// find buy username or email
	err = dbstore.DB.Where("username = ? OR email = ?", username, username).First(user).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return dbstore.ErrInvalidCredentials
		}
		return fmt.Errorf("%w: %v", dbstore.ErrDatabase, err)
	}
	okay := security.ValidatePasswordAndHash(password, user.Password)
	if !okay || user.Status != common.UserStatusEnabled {
		return dbstore.ErrInvalidCredentials
	}
	return nil
}

func (user *User) FillUserById() error {
	if user.Id == 0 {
		return errors.New("id 为空！")
	}
	dbstore.DB.Where(User{Id: user.Id}).First(user)
	return nil
}

func (user *User) FillUserByEmail() error {
	if user.Email == "" {
		return errors.New("email 为空！")
	}
	dbstore.DB.Where(User{Email: user.Email}).First(user)
	return nil
}

func (user *User) FillUserByGitHubId() error {
	if user.GitHubId == "" {
		return errors.New("GitHub id 为空！")
	}
	dbstore.DB.Where(User{GitHubId: user.GitHubId}).First(user)
	return nil
}

// UpdateGitHubId updates the user's GitHub ID (used for migration from login to numeric ID)
func (user *User) UpdateGitHubId(newGitHubId string) error {
	if user.Id == 0 {
		return errors.New("user id is empty")
	}
	return dbstore.DB.Model(user).Update("github_id", newGitHubId).Error
}

func IsEmailAlreadyTaken(email string) bool {
	return dbstore.DB.Unscoped().Where("email = ?", email).Find(&User{}).RowsAffected == 1
}

func IsGitHubIdAlreadyTaken(githubId string) bool {
	return dbstore.DB.Unscoped().Where("github_id = ?", githubId).Find(&User{}).RowsAffected == 1
}

func ResetUserPasswordByEmail(email string, password string) error {
	if email == "" || password == "" {
		return errors.New("邮箱地址或密码为空！")
	}
	hashedPassword, err := security.Password2Hash(password)
	if err != nil {
		return err
	}
	err = dbstore.DB.Model(&User{}).Where("email = ?", email).Update("password", hashedPassword).Error
	return err
}

func IsAdmin(userId int) bool {
	if userId == 0 {
		return false
	}
	var user User
	err := dbstore.DB.Where("id = ?", userId).Select("role").Find(&user).Error
	if err != nil {
		common.SysLog("no such user " + err.Error())
		return false
	}
	return user.Role >= common.RoleAdminUser
}

func normalizeAccessToken(authorization string) string {
	authorization = strings.TrimSpace(authorization)
	if authorization == "" {
		return ""
	}
	parts := strings.Fields(authorization)
	if len(parts) > 0 && strings.EqualFold(parts[0], "Bearer") {
		if len(parts) != 2 {
			return ""
		}
		return strings.TrimSpace(parts[1])
	}
	return authorization
}

func ValidateAccessToken(authorization string) (user *User, err error) {
	token := normalizeAccessToken(authorization)
	if token == "" {
		return nil, dbstore.ErrTokenInvalid
	}
	user = &User{}
	result := dbstore.DB.Where("access_token = ? AND access_token <> ''", token).Limit(1).Find(user)
	if result.Error != nil {
		return nil, fmt.Errorf("%w: %v", dbstore.ErrDatabase, result.Error)
	}
	if result.RowsAffected == 0 {
		return nil, dbstore.ErrTokenInvalid
	}
	user.AccessToken = nil
	return user, nil
}

// GetUserQuota gets quota from Redis first, falls back to DB if needed
func GetUserQuota(id int, fromDB bool) (quota int, err error) {
	defer func() {
		// Update Redis cache asynchronously on successful DB read
		if dbstore.ShouldUpdateRedis(fromDB, err) {
			runtime.RelayGo(func() {
				if err := updateUserQuotaCache(id, quota); err != nil {
					common.SysLog("failed to update user quota cache: " + err.Error())
				}
			})
		}
	}()
	if !fromDB && redis.RedisEnabled {
		quota, err := getUserQuotaCache(id)
		if err == nil {
			return quota, nil
		}
		// Don't return error - fall through to DB
	}
	fromDB = true
	err = dbstore.DB.Model(&User{}).Where("id = ?", id).Select("quota").Find(&quota).Error
	if err != nil {
		return 0, err
	}

	return quota, nil
}

func GetUserUsedQuota(id int) (quota int, err error) {
	err = dbstore.DB.Model(&User{}).Where("id = ?", id).Select("used_quota").Find(&quota).Error
	return quota, err
}

func GetUserEmail(id int) (email string, err error) {
	err = dbstore.DB.Model(&User{}).Where("id = ?", id).Select("email").Find(&email).Error
	return email, err
}

// GetUserGroup gets group from Redis first, falls back to DB if needed
func GetUserGroup(id int, fromDB bool) (group string, err error) {
	defer func() {
		// Update Redis cache asynchronously on successful DB read
		if dbstore.ShouldUpdateRedis(fromDB, err) {
			runtime.RelayGo(func() {
				if err := updateUserGroupCache(id, group); err != nil {
					common.SysLog("failed to update user group cache: " + err.Error())
				}
			})
		}
	}()
	if !fromDB && redis.RedisEnabled {
		group, err := getUserGroupCache(id)
		if err == nil {
			return group, nil
		}
		// Don't return error - fall through to DB
	}
	fromDB = true
	err = dbstore.DB.Model(&User{}).Where("id = ?", id).Select(dbstore.CommonGroupCol).Find(&group).Error
	if err != nil {
		return "", err
	}

	return group, nil
}

// GetUserSetting gets setting from Redis first, falls back to DB if needed
func GetUserSetting(id int, fromDB bool) (settingMap shared.UserSetting, err error) {
	var setting string
	defer func() {
		// Update Redis cache asynchronously on successful DB read
		if dbstore.ShouldUpdateRedis(fromDB, err) {
			runtime.RelayGo(func() {
				if err := updateUserSettingCache(id, setting); err != nil {
					common.SysLog("failed to update user setting cache: " + err.Error())
				}
			})
		}
	}()
	if !fromDB && redis.RedisEnabled {
		setting, err := getUserSettingCache(id)
		if err == nil {
			return setting, nil
		}
		// Don't return error - fall through to DB
	}
	fromDB = true
	err = dbstore.DB.Model(&User{}).Where("id = ?", id).Select("setting").Find(&setting).Error
	if err != nil {
		return settingMap, err
	}
	userBase := &UserBase{
		Setting: setting,
	}
	return userBase.GetSetting(), nil
}

func IncreaseUserQuota(id int, quota int, db bool) (err error) {
	if quota < 0 {
		return errors.New("quota 不能为负数！")
	}
	runtime.RelayGo(func() {
		err := CacheIncrUserQuota(id, int64(quota))
		if err != nil {
			common.SysLog("failed to increase user quota: " + err.Error())
		}
	})
	if !db && common.BatchUpdateEnabled {
		dbstore.AddNewRecord(dbstore.BatchUpdateTypeUserQuota, id, quota)
		return nil
	}
	return increaseUserQuota(id, quota)
}

func increaseUserQuota(id int, quota int) (err error) {
	err = dbstore.DB.Model(&User{}).Where("id = ?", id).Update("quota", gorm.Expr("quota + ?", quota)).Error
	if err != nil {
		return err
	}
	return err
}

func DecreaseUserQuota(id int, quota int) (err error) {
	if quota < 0 {
		return errors.New("quota 不能为负数！")
	}
	runtime.RelayGo(func() {
		err := cacheDecrUserQuota(id, int64(quota))
		if err != nil {
			common.SysLog("failed to decrease user quota: " + err.Error())
		}
	})
	if common.BatchUpdateEnabled {
		dbstore.AddNewRecord(dbstore.BatchUpdateTypeUserQuota, id, -quota)
		return nil
	}
	return decreaseUserQuota(id, quota)
}

func decreaseUserQuota(id int, quota int) (err error) {
	err = dbstore.DB.Model(&User{}).Where("id = ?", id).Update("quota", gorm.Expr("quota - ?", quota)).Error
	if err != nil {
		return err
	}
	return err
}

func DeltaUpdateUserQuota(id int, delta int) (err error) {
	if delta == 0 {
		return nil
	}
	if delta > 0 {
		return IncreaseUserQuota(id, delta, false)
	} else {
		return DecreaseUserQuota(id, -delta)
	}
}

//func GetRootUserEmail() (email string) {
//	DB.Model(&User{}).Where("role = ?", common.RoleRootUser).Select("email").Find(&email)
//	return email
//}

func GetRootUser() (user *User) {
	dbstore.DB.Where("role = ?", common.RoleRootUser).First(&user)
	return user
}

func UpdateUserUsedQuotaAndRequestCount(id int, quota int) {
	if common.BatchUpdateEnabled {
		dbstore.AddNewRecord(dbstore.BatchUpdateTypeUsedQuota, id, quota)
		dbstore.AddNewRecord(dbstore.BatchUpdateTypeRequestCount, id, 1)
		return
	}
	updateUserUsedQuotaAndRequestCount(id, quota, 1)
}

func updateUserUsedQuotaAndRequestCount(id int, quota int, count int) {
	err := dbstore.DB.Model(&User{}).Where("id = ?", id).Updates(
		map[string]interface{}{
			"used_quota":    gorm.Expr("used_quota + ?", quota),
			"request_count": gorm.Expr("request_count + ?", count),
		},
	).Error
	if err != nil {
		common.SysLog("failed to update user used quota and request count: " + err.Error())
		return
	}

	//// 更新缓存
	//if err := invalidateUserCache(id); err != nil {
	//	common.SysError("failed to invalidate user cache: " + err.Error())
	//}
}

func updateUserQuotaUsedQuotaAndRequestCount(id int, quota int, usedQuota int, requestCount int) {
	if quota == 0 && usedQuota == 0 && requestCount == 0 {
		return
	}

	err := dbstore.DB.Model(&User{}).Where("id = ?", id).Updates(
		map[string]interface{}{
			"quota":         gorm.Expr("quota + ?", quota),
			"used_quota":    gorm.Expr("used_quota + ?", usedQuota),
			"request_count": gorm.Expr("request_count + ?", requestCount),
		},
	).Error
	if err != nil {
		common.SysLog("failed to batch update user quota, used quota and request count: " + err.Error())
	}
}

// IncrementMediaConvertedCount 增加用户的多模态适配转换计数（图片/视频转URL）
func IncrementMediaConvertedCount(id int, imageCount int, videoCount int) {
	if imageCount == 0 && videoCount == 0 {
		return
	}
	updates := map[string]interface{}{}
	if imageCount > 0 {
		updates["image_converted_count"] = gorm.Expr("image_converted_count + ?", imageCount)
	}
	if videoCount > 0 {
		updates["video_converted_count"] = gorm.Expr("video_converted_count + ?", videoCount)
	}
	err := dbstore.DB.Model(&User{}).Where("id = ?", id).Updates(updates).Error
	if err != nil {
		common.SysLog("failed to update user media converted count: " + err.Error())
	}
}

// GetUsernameById gets username from Redis first, falls back to DB if needed
func GetUsernameById(id int, fromDB bool) (username string, err error) {
	defer func() {
		// Update Redis cache asynchronously on successful DB read
		if dbstore.ShouldUpdateRedis(fromDB, err) {
			runtime.RelayGo(func() {
				if err := updateUserNameCache(id, username); err != nil {
					common.SysLog("failed to update user name cache: " + err.Error())
				}
			})
		}
	}()
	if !fromDB && redis.RedisEnabled {
		username, err := getUserNameCache(id)
		if err == nil {
			return username, nil
		}
		// Don't return error - fall through to DB
	}
	fromDB = true
	err = dbstore.DB.Model(&User{}).Where("id = ?", id).Select("username").Find(&username).Error
	if err != nil {
		return "", err
	}

	return username, nil
}

func IsLinuxDOIdAlreadyTaken(linuxDOId string) bool {
	var user User
	err := dbstore.DB.Unscoped().Where("linux_do_id = ?", linuxDOId).First(&user).Error
	return !errors.Is(err, gorm.ErrRecordNotFound)
}

func (user *User) FillUserByLinuxDOId() error {
	if user.LinuxDOId == "" {
		return errors.New("linux do id is empty")
	}
	err := dbstore.DB.Where("linux_do_id = ?", user.LinuxDOId).First(user).Error
	return err
}

// 以下查询服务于管理员「按一键登录标识拉黑」（/api/user/ban）。
// 与 Is*AlreadyTaken 的布尔语义不同，这里返回完整行集合（含软删用户），
// 供调用方区分 0 / 1 / N 命中并决定预创建、封禁还是返回候选清单。
func GetUsersByEmailUnscoped(email string) ([]User, error) {
	var users []User
	err := dbstore.DB.Unscoped().Where("email = ?", email).Find(&users).Error
	return users, err
}

func GetUsersByGitHubIdUnscoped(githubId string) ([]User, error) {
	var users []User
	err := dbstore.DB.Unscoped().Where("github_id = ?", githubId).Find(&users).Error
	return users, err
}

func GetUsersByLinuxDOIdUnscoped(linuxDOId string) ([]User, error) {
	var users []User
	err := dbstore.DB.Unscoped().Where("linux_do_id = ?", linuxDOId).Find(&users).Error
	return users, err
}

func IsUsernameTakenUnscoped(username string) bool {
	var count int64
	dbstore.DB.Unscoped().Model(&User{}).Where("username = ?", username).Count(&count)
	return count > 0
}

// UpdateLastLoginAt updates the user's last login timestamp
func UpdateLastLoginAt(id int) error {
	return dbstore.DB.Model(&User{}).Where("id = ?", id).Update("last_login_at", common.GetTimestamp()).Error
}

func RootUserExists() bool {
	var user User
	err := dbstore.DB.Where("role = ?", common.RoleRootUser).First(&user).Error
	return err == nil
}

// CleanupEmptyAccessTokens 清理历史上写入的空字符串 access_token（应为 NULL）。
// 由 dbmigrate 在主库迁移完成后调用；空字符串会让"未设置"与"已设置为空"混淆。
func CleanupEmptyAccessTokens() error {
	return dbstore.DB.Model(&User{}).Where("access_token = ?", "").Update("access_token", nil).Error
}

// init 把用户配额/已用额度/请求数的合并落库函数注册到 dbstore 的批量更新器。
// 这三类批量条目只会由本包写入。
func init() {
	dbstore.RegisterBatchFlushers(dbstore.BatchFlushers{
		User: updateUserQuotaUsedQuotaAndRequestCount,
	})
}
