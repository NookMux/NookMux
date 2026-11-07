package twofastore

import (
	"errors"
	"fmt"
	"github.com/NookMux/NookMux/internal/common"
	"github.com/NookMux/NookMux/internal/infra/security"
	"github.com/NookMux/NookMux/internal/store/db"
	"github.com/NookMux/NookMux/internal/store/user"
	"gorm.io/gorm"
	"time"
)

var ErrTwoFANotEnabled = errors.New("用户未启用2FA")

// TwoFA 用户2FA设置表
type TwoFA struct {
	Id             int            `json:"id" gorm:"primaryKey"`
	UserId         int            `json:"user_id" gorm:"unique;not null;index"`
	Secret         string         `json:"-" gorm:"type:varchar(255);not null"` // TOTP密钥，不返回给前端
	IsEnabled      bool           `json:"is_enabled"`
	FailedAttempts int            `json:"failed_attempts" gorm:"default:0"`
	LockedUntil    *time.Time     `json:"locked_until,omitempty"`
	LastUsedAt     *time.Time     `json:"last_used_at,omitempty"`
	CreatedAt      time.Time      `json:"created_at"`
	UpdatedAt      time.Time      `json:"updated_at"`
	DeletedAt      gorm.DeletedAt `json:"-" gorm:"index"`
}

// TwoFABackupCode 备用码使用记录表
type TwoFABackupCode struct {
	Id        int            `json:"id" gorm:"primaryKey"`
	UserId    int            `json:"user_id" gorm:"not null;index"`
	CodeHash  string         `json:"-" gorm:"type:varchar(255);not null"` // 备用码哈希
	IsUsed    bool           `json:"is_used"`
	UsedAt    *time.Time     `json:"used_at,omitempty"`
	CreatedAt time.Time      `json:"created_at"`
	DeletedAt gorm.DeletedAt `json:"-" gorm:"index"`
}

// GetTwoFAByUserId 根据用户ID获取2FA设置
func GetTwoFAByUserId(userId int) (*TwoFA, error) {
	if userId == 0 {
		return nil, errors.New("用户ID不能为空")
	}

	var twoFA TwoFA
	err := dbstore.DB.Where("user_id = ?", userId).First(&twoFA).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil // 返回nil表示未设置2FA
		}
		return nil, err
	}

	return &twoFA, nil
}

// IsTwoFAEnabled 检查用户是否启用了2FA。查询失败时返回错误，由调用方按
// 失败关闭处理：2FA 状态不可用时宁拒登不放行。
func IsTwoFAEnabled(userId int) (bool, error) {
	twoFA, err := GetTwoFAByUserId(userId)
	if err != nil {
		return false, err
	}
	if twoFA == nil {
		return false, nil
	}
	return twoFA.IsEnabled, nil
}

// DeleteTwoFAByUserIdTx 在事务内硬删除用户的全部 2FA 数据（设置与备用码），
// 供用户硬删除流程编排调用。
func DeleteTwoFAByUserIdTx(tx *gorm.DB, userId int) error {
	if err := tx.Unscoped().Where("user_id = ?", userId).Delete(&TwoFABackupCode{}).Error; err != nil {
		return err
	}
	return tx.Unscoped().Where("user_id = ?", userId).Delete(&TwoFA{}).Error
}

// CreateTwoFA 创建2FA设置
func (t *TwoFA) Create() error {
	// 检查用户是否已存在2FA设置
	existing, err := GetTwoFAByUserId(t.UserId)
	if err != nil {
		return err
	}
	if existing != nil {
		return errors.New("用户已存在2FA设置")
	}

	// 验证用户存在
	var user userstore.User
	if err := dbstore.DB.First(&user, t.UserId).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return errors.New("用户不存在")
		}
		return err
	}

	return dbstore.DB.Create(t).Error
}

// Update 更新2FA设置
func (t *TwoFA) Update() error {
	if t.Id == 0 {
		return errors.New("2FA记录ID不能为空")
	}
	return dbstore.DB.Save(t).Error
}

// Delete 删除2FA设置
func (t *TwoFA) Delete() error {
	if t.Id == 0 {
		return errors.New("2FA记录ID不能为空")
	}

	// 使用事务确保原子性
	return dbstore.DB.Transaction(func(tx *gorm.DB) error {
		// 同时删除相关的备用码记录（硬删除）
		if err := tx.Unscoped().Where("user_id = ?", t.UserId).Delete(&TwoFABackupCode{}).Error; err != nil {
			return err
		}

		// 硬删除2FA记录
		return tx.Unscoped().Delete(t).Error
	})
}

// ResetFailedAttempts 重置失败尝试次数
func (t *TwoFA) ResetFailedAttempts() error {
	t.FailedAttempts = 0
	t.LockedUntil = nil
	return t.Update()
}

// IncrementFailedAttempts 增加失败尝试次数。采用带条件守卫的原子更新：
// 读-改-写全量保存会在并发失败尝试间互相覆盖计数，绕过 MaxFailAttempts
// 锁定；RowsAffected 为 0 说明记录已被并发修改，重读重试。
func (t *TwoFA) IncrementFailedAttempts() error {
	for retry := 0; retry < 5; retry++ {
		lockUntil := time.Time{}
		updates := map[string]interface{}{
			"failed_attempts": t.FailedAttempts + 1,
		}
		if t.FailedAttempts+1 >= security.MaxFailAttempts {
			lockUntil = time.Now().Add(time.Duration(security.LockoutDuration) * time.Second)
			updates["locked_until"] = &lockUntil
		}
		result := dbstore.DB.Model(&TwoFA{}).
			Where("id = ? AND failed_attempts = ? AND (locked_until IS NULL OR locked_until <= ?)", t.Id, t.FailedAttempts, time.Now()).
			Updates(updates)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 1 {
			t.FailedAttempts++
			if !lockUntil.IsZero() {
				lu := lockUntil
				t.LockedUntil = &lu
			}
			return nil
		}
		// 并发变更：重读后按最新计数重试。
		fresh, err := GetTwoFAByUserId(t.UserId)
		if err != nil || fresh == nil {
			return err
		}
		t.FailedAttempts = fresh.FailedAttempts
		t.LockedUntil = fresh.LockedUntil
	}
	return errors.New("更新2FA失败次数冲突重试耗尽")
}

// IsLocked 检查账户是否被锁定
func (t *TwoFA) IsLocked() bool {
	if t.LockedUntil == nil {
		return false
	}
	return time.Now().Before(*t.LockedUntil)
}

// CreateBackupCodes 创建备用码
func CreateBackupCodes(userId int, codes []string) error {
	return dbstore.DB.Transaction(func(tx *gorm.DB) error {
		// 先删除现有的备用码
		if err := tx.Where("user_id = ?", userId).Delete(&TwoFABackupCode{}).Error; err != nil {
			return err
		}

		// 创建新的备用码记录
		for _, code := range codes {
			hashedCode, err := security.HashBackupCode(code)
			if err != nil {
				return err
			}

			backupCode := TwoFABackupCode{
				UserId:   userId,
				CodeHash: hashedCode,
				IsUsed:   false,
			}

			if err := tx.Create(&backupCode).Error; err != nil {
				return err
			}
		}

		return nil
	})
}

// ValidateBackupCode 验证并使用备用码。核销采用条件原子更新
// （WHERE id = ? AND is_used = false），同一备用码并发提交只有一方成功。
func ValidateBackupCode(userId int, code string) (bool, error) {
	if !security.ValidateBackupCode(code) {
		return false, errors.New("验证码或备用码不正确")
	}

	normalizedCode := security.NormalizeBackupCode(code)

	// 查找未使用的备用码
	var backupCodes []TwoFABackupCode
	if err := dbstore.DB.Where("user_id = ? AND is_used = false", userId).Find(&backupCodes).Error; err != nil {
		return false, err
	}

	// 验证备用码
	for _, bc := range backupCodes {
		if security.ValidatePasswordAndHash(normalizedCode, bc.CodeHash) {
			// 标记为已使用：条件更新保证并发重复核销只有一方成功
			now := time.Now()
			result := dbstore.DB.Model(&TwoFABackupCode{}).
				Where("id = ? AND is_used = ?", bc.Id, false).
				Updates(map[string]interface{}{
					"is_used": true,
					"used_at": &now,
				})
			if result.Error != nil {
				return false, result.Error
			}
			if result.RowsAffected != 1 {
				// 该码已被并发请求核销
				return false, nil
			}
			return true, nil
		}
	}

	return false, nil
}

// GetUnusedBackupCodeCount 获取未使用的备用码数量
func GetUnusedBackupCodeCount(userId int) (int, error) {
	var count int64
	err := dbstore.DB.Model(&TwoFABackupCode{}).Where("user_id = ? AND is_used = false", userId).Count(&count).Error
	return int(count), err
}

// DisableTwoFA 禁用用户的2FA
func DisableTwoFA(userId int) error {
	twoFA, err := GetTwoFAByUserId(userId)
	if err != nil {
		return err
	}
	if twoFA == nil {
		return ErrTwoFANotEnabled
	}

	// 删除2FA设置和备用码
	return twoFA.Delete()
}

// EnableTwoFA 启用2FA
func (t *TwoFA) Enable() error {
	t.IsEnabled = true
	t.FailedAttempts = 0
	t.LockedUntil = nil
	return t.Update()
}

// ValidateTOTPAndUpdateUsage 验证TOTP并更新使用记录
func (t *TwoFA) ValidateTOTPAndUpdateUsage(code string) (bool, error) {
	// 检查是否被锁定
	if t.IsLocked() {
		return false, fmt.Errorf("账户已被锁定，请在%v后重试", t.LockedUntil.Format("2006-01-02 15:04:05"))
	}

	// 验证TOTP码
	if !security.ValidateTOTPCode(t.Secret, code) {
		// 增加失败次数
		if err := t.IncrementFailedAttempts(); err != nil {
			common.SysLog("更新2FA失败次数失败: " + err.Error())
		}
		return false, nil
	}

	// 验证成功，重置失败次数并更新最后使用时间
	now := time.Now()
	t.FailedAttempts = 0
	t.LockedUntil = nil
	t.LastUsedAt = &now

	if err := t.Update(); err != nil {
		common.SysLog("更新2FA使用记录失败: " + err.Error())
	}

	return true, nil
}

// ValidateBackupCodeAndUpdateUsage 验证备用码并更新使用记录
func (t *TwoFA) ValidateBackupCodeAndUpdateUsage(code string) (bool, error) {
	// 检查是否被锁定
	if t.IsLocked() {
		return false, fmt.Errorf("账户已被锁定，请在%v后重试", t.LockedUntil.Format("2006-01-02 15:04:05"))
	}

	// 验证备用码
	valid, err := ValidateBackupCode(t.UserId, code)
	if err != nil {
		return false, err
	}

	if !valid {
		// 增加失败次数
		if err := t.IncrementFailedAttempts(); err != nil {
			common.SysLog("更新2FA失败次数失败: " + err.Error())
		}
		return false, nil
	}

	// 验证成功，重置失败次数并更新最后使用时间
	now := time.Now()
	t.FailedAttempts = 0
	t.LockedUntil = nil
	t.LastUsedAt = &now

	if err := t.Update(); err != nil {
		common.SysLog("更新2FA使用记录失败: " + err.Error())
	}

	return true, nil
}

// GetTwoFAStats 获取2FA统计信息（管理员使用）
func GetTwoFAStats() (map[string]interface{}, error) {
	var totalUsers, enabledUsers int64

	// 总用户数
	if err := dbstore.DB.Model(&userstore.User{}).Count(&totalUsers).Error; err != nil {
		return nil, err
	}

	// 启用2FA的用户数
	if err := dbstore.DB.Model(&TwoFA{}).Where("is_enabled = true").Count(&enabledUsers).Error; err != nil {
		return nil, err
	}

	enabledRate := float64(0)
	if totalUsers > 0 {
		enabledRate = float64(enabledUsers) / float64(totalUsers) * 100
	}

	return map[string]interface{}{
		"total_users":   totalUsers,
		"enabled_users": enabledUsers,
		"enabled_rate":  fmt.Sprintf("%.1f%%", enabledRate),
	}, nil
}
