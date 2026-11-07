package userstore

import (
	"context"
	"fmt"
	"github.com/NookMux/NookMux/internal/common"
	"github.com/NookMux/NookMux/internal/infra/redis"
	"github.com/NookMux/NookMux/internal/infra/runtime"
	"github.com/NookMux/NookMux/internal/store/db"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

type userUpdateCountingLogger struct {
	logger.Interface
	userUpdates *int64
}

func (l userUpdateCountingLogger) Trace(ctx context.Context, begin time.Time, fc func() (string, int64), err error) {
	sql, rows := fc()
	normalizedSQL := strings.ToLower(strings.TrimSpace(sql))
	if strings.HasPrefix(normalizedSQL, "update") &&
		(strings.Contains(normalizedSQL, "`users`") || strings.Contains(normalizedSQL, `"users"`)) {
		atomic.AddInt64(l.userUpdates, 1)
	}
	l.Interface.Trace(ctx, begin, func() (string, int64) {
		return sql, rows
	}, err)
}

func setupUserBatchUpdateTestDB(t *testing.T, userUpdateCounter *int64) func() {
	t.Helper()

	oldDB := dbstore.DB
	oldRedisEnabled := redis.RedisEnabled
	// 测试进程不装配 Redis 客户端，显式关闭开关避免配额缓存投递路径空指针
	redis.RedisEnabled = false

	config := &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)}
	if userUpdateCounter != nil {
		config.Logger = userUpdateCountingLogger{
			Interface:   logger.Default.LogMode(logger.Silent),
			userUpdates: userUpdateCounter,
		}
	}
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())), config)
	require.NoError(t, err, "open sqlite test db")
	require.NoError(t, db.AutoMigrate(&User{}), "migrate sqlite test db")

	dbstore.ResetBatchUpdateStores()
	dbstore.DB = db

	return func() {
		// 排空后台协程池中的异步配额缓存任务，避免与全局变量恢复产生数据竞争
		runtime.WaitRelayTasks()
		dbstore.ResetBatchUpdateStores()
		dbstore.DB = oldDB
		redis.RedisEnabled = oldRedisEnabled
	}
}

func createBatchUpdateTestUser(t *testing.T, username, affCode string, quota, usedQuota, requestCount int) User {
	t.Helper()

	user := User{
		Username:     username,
		Password:     "password123",
		Role:         common.RoleCommonUser,
		Status:       common.UserStatusEnabled,
		AffCode:      affCode,
		Quota:        quota,
		UsedQuota:    usedQuota,
		RequestCount: requestCount,
	}
	require.NoError(t, dbstore.DB.Create(&user).Error, "create user")
	return user
}

func loadUser(t *testing.T, id int) User {
	t.Helper()

	var user User
	require.NoError(t, dbstore.DB.First(&user, id).Error, "load user")
	return user
}

// applyRelayBilling 模拟一次计费落账：余额扣减同步执行，已用额度与请求
// 计数进入批量队列（与线上行为一致）。
func applyRelayBilling(t *testing.T, userID int, cost int) {
	t.Helper()
	require.NoError(t, DecreaseUserQuota(userID, cost))
	dbstore.AddNewRecord(dbstore.BatchUpdateTypeUsedQuota, userID, cost)
	dbstore.AddNewRecord(dbstore.BatchUpdateTypeRequestCount, userID, 1)
}

func TestBatchUpdateFlushesQuotaUsageAndRequestCountTogether(t *testing.T) {
	cleanup := setupUserBatchUpdateTestDB(t, nil)
	defer cleanup()

	user := createBatchUpdateTestUser(t, "batch-u1", "batch-aff-1", 10_000, 0, 0)
	applyRelayBilling(t, user.Id, 300)
	dbstore.BatchUpdate()

	got := loadUser(t, user.Id)
	assert.Equal(t, 9700, got.Quota)
	assert.Equal(t, 300, got.UsedQuota)
	assert.Equal(t, 1, got.RequestCount)
}

func TestBatchUpdateAccumulatesMultiplePendingUserUpdates(t *testing.T) {
	cleanup := setupUserBatchUpdateTestDB(t, nil)
	defer cleanup()

	user := createBatchUpdateTestUser(t, "batch-u2", "batch-aff-2", 10_000, 100, 5)
	applyRelayBilling(t, user.Id, 200)
	applyRelayBilling(t, user.Id, 150)
	dbstore.BatchUpdate()

	got := loadUser(t, user.Id)
	assert.Equal(t, 9650, got.Quota)
	assert.Equal(t, 450, got.UsedQuota)
	assert.Equal(t, 7, got.RequestCount)
}

func TestBatchUpdateDoesNotMixUsers(t *testing.T) {
	cleanup := setupUserBatchUpdateTestDB(t, nil)
	defer cleanup()

	alice := createBatchUpdateTestUser(t, "batch-alice", "batch-aff-a", 5_000, 0, 0)
	bob := createBatchUpdateTestUser(t, "batch-bob", "batch-aff-b", 8_000, 50, 2)

	applyRelayBilling(t, alice.Id, 100)
	applyRelayBilling(t, bob.Id, 200)
	dbstore.BatchUpdate()

	gotAlice := loadUser(t, alice.Id)
	gotBob := loadUser(t, bob.Id)

	assert.Equal(t, 4900, gotAlice.Quota)
	assert.Equal(t, 100, gotAlice.UsedQuota)
	assert.Equal(t, 1, gotAlice.RequestCount)
	assert.Equal(t, 7800, gotBob.Quota)
	assert.Equal(t, 250, gotBob.UsedQuota)
	assert.Equal(t, 3, gotBob.RequestCount)
}

func TestBatchUpdateWritesUserQuotaUsageAndRequestCountOncePerUser(t *testing.T) {
	var userUpdateCount int64
	cleanup := setupUserBatchUpdateTestDB(t, &userUpdateCount)
	defer cleanup()

	user := createBatchUpdateTestUser(t, "batch-count", "batch-aff-count", 10_000, 0, 0)
	atomic.StoreInt64(&userUpdateCount, 0)

	applyRelayBilling(t, user.Id, 200)
	applyRelayBilling(t, user.Id, 150)
	dbstore.BatchUpdate()

	got := loadUser(t, user.Id)
	assert.Equal(t, 9650, got.Quota)
	assert.Equal(t, 350, got.UsedQuota)
	assert.Equal(t, 2, got.RequestCount)
	// 两次计费各产生一次同步余额扣减 UPDATE，统计字段合并为一次批量 UPDATE
	assert.Equal(t, int64(3), atomic.LoadInt64(&userUpdateCount))
}
