package userstore

import (
	"fmt"
	"github.com/NookMux/NookMux/internal/common"
	"github.com/NookMux/NookMux/internal/infra/redis"
	"github.com/NookMux/NookMux/internal/store/db"
	"github.com/NookMux/NookMux/internal/store/token"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"testing"
)

func setupUserCacheInvalidationTestDB(t *testing.T) {
	t.Helper()

	oldDB := dbstore.DB
	oldRedisEnabled := redis.RedisEnabled
	oldMemoryCacheEnabled := common.MemoryCacheEnabled
	t.Cleanup(func() {
		if dbstore.DB != nil {
			if sqlDB, err := dbstore.DB.DB(); err == nil {
				_ = sqlDB.Close()
			}
		}
		dbstore.DB = oldDB
		redis.RedisEnabled = oldRedisEnabled
		common.MemoryCacheEnabled = oldMemoryCacheEnabled
	})

	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite test db: %v", err)
	}
	if err := db.AutoMigrate(&User{}, &tokenstore.Token{}); err != nil {
		t.Fatalf("migrate sqlite test db: %v", err)
	}
	dbstore.DB = db
	redis.RedisEnabled = false
	common.MemoryCacheEnabled = false
}

func createCacheInvalidationUserWithToken(t *testing.T, username string) User {
	t.Helper()

	user := User{
		Username: username,
		Password: "password",
		Role:     common.RoleCommonUser,
		Status:   common.UserStatusEnabled,
	}
	if err := dbstore.DB.Create(&user).Error; err != nil {
		t.Fatalf("create user: %v", err)
	}
	token := tokenstore.Token{
		UserId: user.Id,
		Key:    username + "-token",
		Name:   username + "-token",
		Status: common.TokenStatusEnabled,
	}
	if err := dbstore.DB.Create(&token).Error; err != nil {
		t.Fatalf("create token: %v", err)
	}
	return user
}

func TestHardDeleteUserByIdTxDeletesUserRow(t *testing.T) {
	setupUserCacheInvalidationTestDB(t)
	user := createCacheInvalidationUserWithToken(t, "cache-hard-delete-user")
	if err := dbstore.DB.Transaction(func(tx *gorm.DB) error {
		if err := tokenstore.DeleteTokensByUserIdTx(tx, user.Id); err != nil {
			return err
		}
		return HardDeleteUserByIdTx(tx, user.Id)
	}); err != nil {
		t.Fatalf("hard delete user tx: %v", err)
	}

	var count int64
	if err := dbstore.DB.Unscoped().Model(&User{}).Where("id = ?", user.Id).Count(&count).Error; err != nil {
		t.Fatalf("count user: %v", err)
	}
	if count != 0 {
		t.Fatalf("deleted user count = %d, want 0", count)
	}
	if err := dbstore.DB.Unscoped().Model(&tokenstore.Token{}).Where("user_id = ?", user.Id).Count(&count).Error; err != nil {
		t.Fatalf("count tokens: %v", err)
	}
	if count != 0 {
		t.Fatalf("deleted user token count = %d, want 0 (auth data must be purged with the user)", count)
	}
}

func TestSoftDeleteUserInvalidatesTokenCacheWithoutRedis(t *testing.T) {
	setupUserCacheInvalidationTestDB(t)
	user := createCacheInvalidationUserWithToken(t, "cache-soft-delete-user")
	if err := user.Delete(); err != nil {
		t.Fatalf("Delete: %v", err)
	}

	var count int64
	if err := dbstore.DB.Model(&User{}).Where("id = ?", user.Id).Count(&count).Error; err != nil {
		t.Fatalf("count user: %v", err)
	}
	if count != 0 {
		t.Fatalf("active user count = %d, want 0", count)
	}
}
