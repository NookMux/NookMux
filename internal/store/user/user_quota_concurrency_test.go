package userstore

import (
	"fmt"
	"sync"
	"testing"

	"github.com/NookMux/NookMux/internal/infra/redis"
	"github.com/NookMux/NookMux/internal/infra/runtime"
	"github.com/NookMux/NookMux/internal/store/db"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupUserQuotaConcurrencyTestDB(t *testing.T) func() {
	t.Helper()

	oldDB := dbstore.DB
	oldRedisEnabled := redis.RedisEnabled
	// 测试进程不装配 Redis 客户端，显式关闭开关避免配额缓存投递路径空指针
	redis.RedisEnabled = false
	// 单连接串行化 SQLite 写入，条件 UPDATE 的防超卖语义由 SQL 原子性保证
	gdb, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())), &gorm.Config{})
	require.NoError(t, err, "open sqlite test db")
	require.NoError(t, gdb.AutoMigrate(&User{}), "migrate sqlite test db")
	sqlDB, err := gdb.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)

	dbstore.ResetBatchUpdateStores()
	dbstore.DB = gdb

	return func() {
		// 排空后台协程池中的异步配额缓存任务，避免与全局变量恢复产生数据竞争
		runtime.WaitRelayTasks()
		dbstore.ResetBatchUpdateStores()
		dbstore.DB = oldDB
		redis.RedisEnabled = oldRedisEnabled
	}
}

func TestDecreaseUserQuotaInsufficientReturnsError(t *testing.T) {
	cleanup := setupUserQuotaConcurrencyTestDB(t)
	defer cleanup()

	user := createBatchUpdateTestUser(t, "quota-insufficient", "quota-aff-ins", 100, 0, 0)

	err := DecreaseUserQuota(user.Id, 200)
	assert.ErrorIs(t, err, ErrUserQuotaNotEnough)

	got := loadUser(t, user.Id)
	assert.Equal(t, 100, got.Quota, "余额不足时不得落库")
}

func TestDecreaseUserQuotaConcurrentNoOverspend(t *testing.T) {
	cleanup := setupUserQuotaConcurrencyTestDB(t)
	defer cleanup()

	const initialQuota = 1000
	const workers = 50
	const cost = 100
	// 理论上最多成功 initialQuota/cost = 10 次
	user := createBatchUpdateTestUser(t, "quota-concurrent", "quota-aff-conc", initialQuota, 0, 0)

	var wg sync.WaitGroup
	var successCount int
	var mu sync.Mutex
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := DecreaseUserQuota(user.Id, cost); err == nil {
				mu.Lock()
				successCount++
				mu.Unlock()
			}
		}()
	}
	wg.Wait()

	got := loadUser(t, user.Id)
	assert.Equal(t, initialQuota/cost, successCount, "成功扣减次数不得超过余额容纳的次数")
	assert.GreaterOrEqual(t, got.Quota, 0, "余额绝不允许为负")
	assert.Equal(t, initialQuota-cost*successCount, got.Quota)
}
