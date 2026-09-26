package userstore

import (
	"errors"
	"fmt"
	"github.com/NookMux/NookMux/internal/common"
	"github.com/NookMux/NookMux/internal/infra/security"
	"github.com/NookMux/NookMux/internal/store/db"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	"testing"
)

func setupUserValidateTestDB(t *testing.T) {
	t.Helper()

	oldDB := dbstore.DB
	gdb, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())), &gorm.Config{})
	require.NoError(t, err, "open sqlite test db")
	require.NoError(t, gdb.AutoMigrate(&User{}), "migrate sqlite test db")
	dbstore.DB = gdb

	t.Cleanup(func() {
		if sqlDB, err := gdb.DB(); err == nil {
			_ = sqlDB.Close()
		}
		dbstore.DB = oldDB
	})
}

func createValidateTestUser(t *testing.T, username, email, password string, status int) User {
	t.Helper()

	hashed, err := security.Password2Hash(password)
	require.NoError(t, err, "hash test user password")

	user := User{
		Username: username,
		Password: hashed,
		Email:    email,
		Role:     common.RoleCommonUser,
		Status:   status,
	}
	require.NoError(t, dbstore.DB.Create(&user).Error, "create test user")
	return user
}

func TestValidateAndFillLoginSucceedsForEnabledUser(t *testing.T) {
	setupUserValidateTestDB(t)

	createValidateTestUser(t, "login-alice", "alice@example.com", "right-password", common.UserStatusEnabled)

	user := User{Username: "login-alice", Password: "right-password"}
	require.NoError(t, user.ValidateAndFill(), "valid login should pass")
	assert.NotZero(t, user.Id, "user record should be filled on success")
}

func TestValidateAndFillWrongPasswordFails(t *testing.T) {
	setupUserValidateTestDB(t)

	createValidateTestUser(t, "login-bob", "bob@example.com", "right-password", common.UserStatusEnabled)

	user := User{Username: "login-bob", Password: "wrong-password"}
	err := user.ValidateAndFill()
	require.Error(t, err, "wrong password should fail")
	assert.ErrorIs(t, err, dbstore.ErrInvalidCredentials)
}

func TestValidateAndFillDisabledUserFails(t *testing.T) {
	setupUserValidateTestDB(t)

	createValidateTestUser(t, "login-carol", "carol@example.com", "right-password", common.UserStatusDisabled)

	user := User{Username: "login-carol", Password: "right-password"}
	err := user.ValidateAndFill()
	require.Error(t, err, "disabled user should fail")
	assert.ErrorIs(t, err, dbstore.ErrInvalidCredentials)
}

func TestValidateAndFillUnknownUserFails(t *testing.T) {
	setupUserValidateTestDB(t)

	createValidateTestUser(t, "login-dave", "dave@example.com", "right-password", common.UserStatusEnabled)

	user := User{Username: "nobody", Password: "whatever-password"}
	err := user.ValidateAndFill()
	require.Error(t, err, "unknown username should fail")
	assert.ErrorIs(t, err, dbstore.ErrInvalidCredentials)
}

func TestValidateAndFillEmptyCredentialsFails(t *testing.T) {
	setupUserValidateTestDB(t)

	user := User{Username: "", Password: "some-password"}
	err := user.ValidateAndFill()
	require.Error(t, err, "empty username should fail")
	assert.ErrorIs(t, err, dbstore.ErrUserEmptyCredentials)

	user = User{Username: "someone", Password: ""}
	err = user.ValidateAndFill()
	require.Error(t, err, "empty password should fail")
	assert.ErrorIs(t, err, dbstore.ErrUserEmptyCredentials)
}

// TestValidateAndFillUnknownUserErrorMatchesWrongPasswordError 保证账号不存在
// 分支与密码错误分支返回相同的哨兵错误，哑哈希时序均衡不改变错误语义。
func TestValidateAndFillUnknownUserErrorMatchesWrongPasswordError(t *testing.T) {
	setupUserValidateTestDB(t)

	createValidateTestUser(t, "login-eve", "eve@example.com", "right-password", common.UserStatusEnabled)

	unknownUserErr := (&User{Username: "ghost", Password: "whatever-password"}).ValidateAndFill()
	require.Error(t, unknownUserErr, "unknown username should fail")

	wrongPasswordErr := (&User{Username: "login-eve", Password: "wrong-password"}).ValidateAndFill()
	require.Error(t, wrongPasswordErr, "wrong password should fail")

	assert.True(t, errors.Is(unknownUserErr, dbstore.ErrInvalidCredentials), "unknown user should return ErrInvalidCredentials")
	assert.True(t, errors.Is(wrongPasswordErr, dbstore.ErrInvalidCredentials), "wrong password should return ErrInvalidCredentials")
	assert.Equal(t, wrongPasswordErr, unknownUserErr, "both failure branches must return the identical sentinel error")
}
