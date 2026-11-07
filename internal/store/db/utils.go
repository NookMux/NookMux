package dbstore

import (
	"errors"
	"github.com/NookMux/NookMux/internal/common"
	"github.com/NookMux/NookMux/internal/infra/redis"
	"github.com/NookMux/NookMux/internal/infra/runtime"
	"gorm.io/gorm"
	"sync"
	"time"
)

// 批量更新仅承载统计型字段（已用额度、请求计数）；用户余额与令牌剩余额度
// 等准入型扣减始终走同步条件 UPDATE，不进入批量队列。
const (
	BatchUpdateTypeUsedQuota = iota
	BatchUpdateTypeChannelUsedQuota
	BatchUpdateTypeRequestCount
	BatchUpdateTypeTokenUsedQuota
	BatchUpdateTypeCount // if you add a new type, you need to add a new map and a new lock
)

var batchUpdateStores []map[int]int
var batchUpdateLocks []sync.Mutex

func init() {
	for i := 0; i < BatchUpdateTypeCount; i++ {
		batchUpdateStores = append(batchUpdateStores, make(map[int]int))
		batchUpdateLocks = append(batchUpdateLocks, sync.Mutex{})
	}
}

// BatchFlushers 汇总各资源包注册的批量落库回调。
//
// 批量更新器（stores/locks/定时 flush）位于 dbstore，而各类型的实际落库
// 函数留在资源包（token/user/channel）。为避免 dbstore 反向依赖资源包，
// 由资源包在自己的 init() 中把落库函数注册进来。
//
// 不变式：某类型的批量条目只会由注册该类型 flusher 的同一个资源包写入
// （例如 BatchUpdateTypeTokenUsedQuota 仅在 token 包内 AddNewRecord），因此
// 只要包被链接，flusher 一定已注册；flusher 为 nil 且 store 非空只可能
// 是编码错误，届时按类型记录错误并丢弃该批数据，绝不静默吞掉。
type BatchFlushers struct {
	TokenUsedQuota   func(id int, value int) error
	ChannelUsedQuota func(id int, value int)
	// User 处理 UsedQuota/RequestCount 两个 store 的合并落库。
	User func(id int, quota int, usedQuota int, requestCount int)
}

var batchFlushers BatchFlushers
var batchFlushersMutex sync.Mutex

// RegisterBatchFlushers 合并注册非空的 flusher 字段；各资源包在 init() 中调用，
// 不同包的字段互不覆盖。
func RegisterBatchFlushers(f BatchFlushers) {
	batchFlushersMutex.Lock()
	defer batchFlushersMutex.Unlock()
	if f.TokenUsedQuota != nil {
		batchFlushers.TokenUsedQuota = f.TokenUsedQuota
	}
	if f.ChannelUsedQuota != nil {
		batchFlushers.ChannelUsedQuota = f.ChannelUsedQuota
	}
	if f.User != nil {
		batchFlushers.User = f.User
	}
}

func InitBatchUpdater() {
	runtime.RelayGo(func() {
		for {
			time.Sleep(time.Duration(common.BatchUpdateInterval) * time.Second)
			BatchUpdate()
		}
	})
}

// FlushBatchUpdates 在进程停机路径上把批量暂存的统计增量落库，避免退出时
// 丢失窗口内的已用额度与请求计数。与定时 ticker 并发安全（换 map 在锁内
// 原子完成），重复调用幂等。
func FlushBatchUpdates() {
	BatchUpdate()
}

func AddNewRecord(type_ int, id int, value int) {
	batchUpdateLocks[type_].Lock()
	defer batchUpdateLocks[type_].Unlock()
	if _, ok := batchUpdateStores[type_][id]; !ok {
		batchUpdateStores[type_][id] = value
	} else {
		batchUpdateStores[type_][id] += value
	}
}

// ResetBatchUpdateStores 清空全部批量暂存数据，仅供测试在用例间复位状态使用。
func ResetBatchUpdateStores() {
	for i := 0; i < BatchUpdateTypeCount; i++ {
		batchUpdateLocks[i].Lock()
		batchUpdateStores[i] = make(map[int]int)
		batchUpdateLocks[i].Unlock()
	}
}

func BatchUpdate() {
	// check if there's any data to update
	hasData := false
	for i := 0; i < BatchUpdateTypeCount; i++ {
		batchUpdateLocks[i].Lock()
		if len(batchUpdateStores[i]) > 0 {
			hasData = true
			batchUpdateLocks[i].Unlock()
			break
		}
		batchUpdateLocks[i].Unlock()
	}

	if !hasData {
		return
	}

	common.SysLog("batch update started")
	stores := make([]map[int]int, BatchUpdateTypeCount)
	for i := 0; i < BatchUpdateTypeCount; i++ {
		batchUpdateLocks[i].Lock()
		stores[i] = batchUpdateStores[i]
		batchUpdateStores[i] = make(map[int]int)
		batchUpdateLocks[i].Unlock()
	}

	for key, value := range stores[BatchUpdateTypeChannelUsedQuota] {
		if batchFlushers.ChannelUsedQuota == nil {
			common.SysError("batch flusher for BatchUpdateTypeChannelUsedQuota not registered; dropping batch data")
			continue
		}
		batchFlushers.ChannelUsedQuota(key, value)
	}
	for key, value := range stores[BatchUpdateTypeTokenUsedQuota] {
		if batchFlushers.TokenUsedQuota == nil {
			common.SysError("batch flusher for BatchUpdateTypeTokenUsedQuota not registered; dropping batch data")
			continue
		}
		if err := batchFlushers.TokenUsedQuota(key, value); err != nil {
			common.SysLog("failed to batch update token used quota: " + err.Error())
		}
	}

	usedQuotaStore := stores[BatchUpdateTypeUsedQuota]
	requestCountStore := stores[BatchUpdateTypeRequestCount]

	if batchFlushers.User == nil && (len(usedQuotaStore) > 0 || len(requestCountStore) > 0) {
		common.SysError("batch flusher for user quota metrics not registered; dropping batch data")
		common.SysLog("batch update finished")
		return
	}

	userIDs := make(map[int]struct{}, len(usedQuotaStore)+len(requestCountStore))
	for key := range usedQuotaStore {
		userIDs[key] = struct{}{}
	}
	for key := range requestCountStore {
		userIDs[key] = struct{}{}
	}
	for key := range userIDs {
		batchFlushers.User(key, 0, usedQuotaStore[key], requestCountStore[key])
	}
	common.SysLog("batch update finished")
}

func RecordExist(err error) (bool, error) {
	if err == nil {
		return true, nil
	}
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return false, nil
	}
	return false, err
}

func ShouldUpdateRedis(fromDB bool, err error) bool {
	return redis.RedisEnabled && fromDB && err == nil
}
