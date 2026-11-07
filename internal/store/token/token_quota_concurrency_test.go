package tokenstore

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

func setupTokenQuotaConcurrencyTestDB(t *testing.T) func() {
	t.Helper()

	oldDB := dbstore.DB
	oldRedisEnabled := redis.RedisEnabled
	// 测试进程不装配 Redis 客户端，显式关闭开关避免配额缓存投递路径空指针
	redis.RedisEnabled = false
	// 单连接串行化 SQLite 写入，条件 UPDATE 的防超卖语义由 SQL 原子性保证
	gdb, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())), &gorm.Config{})
	require.NoError(t, err, "open sqlite test db")
	require.NoError(t, gdb.AutoMigrate(&Token{}), "migrate sqlite test db")
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

func createQuotaConcurrencyTestToken(t *testing.T, remainQuota int) Token {
	t.Helper()
	token := Token{
		UserId:      1,
		Key:         "quota-conc-key",
		Name:        "quota-concurrent",
		RemainQuota: remainQuota,
		QuotaType:   1,
	}
	require.NoError(t, dbstore.DB.Create(&token).Error, "create token")
	return token
}

func TestDecreaseTokenQuotaInsufficientReturnsError(t *testing.T) {
	cleanup := setupTokenQuotaConcurrencyTestDB(t)
	defer cleanup()

	token := createQuotaConcurrencyTestToken(t, 100)

	err := DecreaseTokenQuota(token.Id, token.Key, 200)
	assert.ErrorIs(t, err, ErrTokenQuotaNotEnough)

	var got Token
	require.NoError(t, dbstore.DB.First(&got, token.Id).Error)
	assert.Equal(t, 100, got.RemainQuota, "额度不足时不得落库")
}

func TestDecreaseTokenQuotaConcurrentNoOverspend(t *testing.T) {
	cleanup := setupTokenQuotaConcurrencyTestDB(t)
	defer cleanup()

	const initialQuota = 1000
	const workers = 50
	const cost = 100
	// 理论上最多成功 initialQuota/cost = 10 次
	token := createQuotaConcurrencyTestToken(t, initialQuota)

	var wg sync.WaitGroup
	var successCount int
	var mu sync.Mutex
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := DecreaseTokenQuota(token.Id, token.Key, cost); err == nil {
				mu.Lock()
				successCount++
				mu.Unlock()
			}
		}()
	}
	wg.Wait()

	var got Token
	require.NoError(t, dbstore.DB.First(&got, token.Id).Error)
	assert.Equal(t, initialQuota/cost, successCount, "成功扣减次数不得超过额度容纳的次数")
	assert.GreaterOrEqual(t, got.RemainQuota, 0, "令牌剩余额度绝不允许为负")
	assert.Equal(t, initialQuota-cost*successCount, got.RemainQuota)
	assert.Equal(t, cost*successCount, got.UsedQuota)
}
