package payment

import (
	"hash/fnv"
	"sync"
)

// orderLockShards 订单锁分片数。取 256：锁表在包初始化时一次性建满，
// 固定占用约 2KB（每片一个 sync.Mutex），不随历史订单量增长；256 片
// 足以摊薄支付回调的并发竞争，且分片碰撞只会额外互斥、不影响正确性。
const orderLockShards = 256

// orderLocks 固定分片锁表。同一 tradeNo 始终哈希到同一分片，与原按
// 订单号逐单建锁的实现互斥语义等价；不同 tradeNo 可能共享分片，仅
// 降低并发度，不会削弱互斥性。
var orderLocks [orderLockShards]sync.Mutex

// shardFor 返回订单号所属分片的锁。FNV-1a 为非加密哈希，此处仅用于
// 分片路由，无需抗碰撞能力。
func shardFor(tradeNo string) *sync.Mutex {
	h := fnv.New32a()
	_, _ = h.Write([]byte(tradeNo)) // hash/fnv 的 Write 永不返回错误
	return &orderLocks[h.Sum32()%orderLockShards]
}

// LockOrder 尝试对给定订单号加锁
func LockOrder(tradeNo string) {
	shardFor(tradeNo).Lock()
}

// UnlockOrder 释放给定订单号的锁
func UnlockOrder(tradeNo string) {
	shardFor(tradeNo).Unlock()
}
