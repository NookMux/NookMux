// Package loginkeystore 存储浏览器登录密码加密协议的服务端密钥材料。密钥
// 表与面向管理员的系统选项隔离，字段不对任何 API 响应暴露。
package loginkeystore

import (
	"errors"
	"fmt"

	"github.com/NookMux/NookMux/internal/common"
	"github.com/NookMux/NookMux/internal/infra/passwordcrypto"
	"github.com/NookMux/NookMux/internal/store/db"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// activeLoginEncryptionKeySlot 单槽位标识；多副本部署经唯一索引收敛到同
// 一把活跃密钥。
const activeLoginEncryptionKeySlot = "active"

type LoginEncryptionKey struct {
	ID            uint   `json:"-" gorm:"primaryKey"`
	Slot          string `json:"-" gorm:"type:varchar(32);not null;uniqueIndex"`
	PrivateKeyPEM string `json:"-" gorm:"type:text;not null"`
}

// InitPasswordEncryptionKey 加载共享登录加密密钥：已存在则直接加载；不存
// 在则生成并持久化。并发副本竞争写入时回读收敛到唯一槽位。
func InitPasswordEncryptionKey() error {
	var stored LoginEncryptionKey
	queryErr := dbstore.DB.Where("slot = ?", activeLoginEncryptionKeySlot).First(&stored).Error
	if queryErr == nil {
		if err := passwordcrypto.LoadPrivateKeyPEM(stored.PrivateKeyPEM); err != nil {
			return fmt.Errorf("load persisted password encryption key: %w", err)
		}
		return nil
	}
	if !errors.Is(queryErr, gorm.ErrRecordNotFound) {
		return fmt.Errorf("read password encryption key: %w", queryErr)
	}

	privateKeyPEM, err := passwordcrypto.GeneratePrivateKeyPEM()
	if err != nil {
		return err
	}
	candidate := LoginEncryptionKey{
		Slot:          activeLoginEncryptionKeySlot,
		PrivateKeyPEM: privateKeyPEM,
	}
	// OnConflict 在部分方言（如 DM8）会降级为普通 INSERT 而撞唯一键，
	// 或另一副本已先写入；两种竞争形态都通过回读唯一槽位收敛。
	if err := dbstore.DB.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "slot"}},
		DoNothing: true,
	}).Create(&candidate).Error; err != nil {
		common.SysLog("password encryption key create raced, reloading: " + err.Error())
	}
	if err := dbstore.DB.Where("slot = ?", activeLoginEncryptionKeySlot).First(&stored).Error; err != nil {
		return fmt.Errorf("reload password encryption key: %w", err)
	}
	if err := passwordcrypto.LoadPrivateKeyPEM(stored.PrivateKeyPEM); err != nil {
		return fmt.Errorf("load persisted password encryption key: %w", err)
	}
	return nil
}
