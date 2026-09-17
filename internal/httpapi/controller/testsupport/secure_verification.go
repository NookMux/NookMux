// Package testsupport 提供 controller 子包测试共享的安全验证 fixture。
// 仅可被 _test.go 文件导入，不得在业务代码中引用。
package testsupport

import (
	"fmt"
	"testing"
	"time"

	"github.com/NookMux/NookMux/internal/common"
	secureverificationcontroller "github.com/NookMux/NookMux/internal/httpapi/controller/secure_verification"
	"github.com/NookMux/NookMux/internal/infra/redis"
	"github.com/NookMux/NookMux/internal/infra/runtime"
	"github.com/NookMux/NookMux/internal/store/db"
	"github.com/NookMux/NookMux/internal/store/log"
	"github.com/NookMux/NookMux/internal/store/passkey"
	"github.com/NookMux/NookMux/internal/store/twofa"
	"github.com/NookMux/NookMux/internal/store/user"
	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

// SetupSecureVerificationTestDB 为当前测试切换到独立 sqlite 内存库，
// 并迁移用户/Passkey/2FA/日志表；测试结束后恢复原全局句柄。
func SetupSecureVerificationTestDB(t *testing.T) {
	t.Helper()

	oldDB := dbstore.DB
	oldLogDB := dbstore.LOG_DB
	oldRedisEnabled := redis.RedisEnabled
	oldMemoryCacheEnabled := common.MemoryCacheEnabled
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite test db: %v", err)
	}
	if err := db.AutoMigrate(&userstore.User{}, &passkeystore.PasskeyCredential{}, &twofastore.TwoFA{}, &logstore.Log{}); err != nil {
		t.Fatalf("migrate sqlite test db: %v", err)
	}
	dbstore.DB = db
	dbstore.LOG_DB = db
	redis.RedisEnabled = false
	common.MemoryCacheEnabled = false

	t.Cleanup(func() {
		// 等待后台中继任务（如 IncreaseUserQuota 的异步计费回写）排空，
		// 再改写它们读取的全局量（redis.RedisEnabled 等），避免测试清理
		// 与后台协程的数据竞争。
		runtime.WaitRelayTasks()
		if sqlDB, err := db.DB(); err == nil {
			_ = sqlDB.Close()
		}
		dbstore.DB = oldDB
		dbstore.LOG_DB = oldLogDB
		redis.RedisEnabled = oldRedisEnabled
		common.MemoryCacheEnabled = oldMemoryCacheEnabled
	})
}

// CreateSecureVerificationTestUser 创建一个可直接登录的普通用户。
func CreateSecureVerificationTestUser(t *testing.T, id int, accessToken string) userstore.User {
	t.Helper()

	user := userstore.User{
		Id:          id,
		Username:    fmt.Sprintf("secure-user-%d", id),
		Password:    "password123",
		Role:        common.RoleCommonUser,
		Status:      common.UserStatusEnabled,
		DisplayName: fmt.Sprintf("Secure User %d", id),
		AccessToken: &accessToken,
		Group:       "default",
		AffCode:     fmt.Sprintf("secure-aff-%d", id),
	}
	if err := dbstore.DB.Create(&user).Error; err != nil {
		t.Fatalf("create user: %v", err)
	}
	return user
}

// CreateSecureVerificationTestPasskey 为用户写入一条 Passkey 凭据。
func CreateSecureVerificationTestPasskey(t *testing.T, userId int) {
	t.Helper()

	passkey := passkeystore.PasskeyCredential{
		UserID:          userId,
		CredentialID:    fmt.Sprintf("Y3JlZGVudGlhbC0%d", userId),
		PublicKey:       "cHVibGljLWtleQ==",
		AttestationType: "none",
	}
	if err := dbstore.DB.Create(&passkey).Error; err != nil {
		t.Fatalf("create passkey credential: %v", err)
	}
}

// CreateSecureVerificationTestTwoFA 为用户启用 2FA。
func CreateSecureVerificationTestTwoFA(t *testing.T, userId int) {
	t.Helper()

	twoFA := twofastore.TwoFA{
		UserId:    userId,
		Secret:    "secret",
		IsEnabled: true,
	}
	if err := dbstore.DB.Create(&twoFA).Error; err != nil {
		t.Fatalf("create twofa: %v", err)
	}
}

// SecureVerificationSessionMiddleware 构造一个已通过安全验证的 session，
// method 取 SecureVerificationMethod2FA / SecureVerificationMethodPasskey。
func SecureVerificationSessionMiddleware(userId int, method string) gin.HandlerFunc {
	return func(c *gin.Context) {
		session := sessions.Default(c)
		session.Set("id", userId)
		session.Set(secureverificationcontroller.SecureVerificationSessionKey, time.Now().Unix())
		session.Set(secureverificationcontroller.SecureVerificationUserIDSessionKey, userId)
		session.Set(secureverificationcontroller.SecureVerificationMethodSessionKey, method)
		_ = session.Save()
		c.Next()
	}
}
