package payment

import (
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// TestLockOrderMutualExclusionSameTradeNo 并发对同一订单号加解锁，
// 通过临界区计数断言任一时刻最多一个持有者，验证互斥语义。
func TestLockOrderMutualExclusionSameTradeNo(t *testing.T) {
	const (
		goroutines = 16
		iterations = 200
	)
	tradeNo := "mutex-trade"
	var (
		wg       sync.WaitGroup
		inCS     atomic.Int32
		violated atomic.Bool
	)
	for g := 0; g < goroutines; g++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < iterations; i++ {
				LockOrder(tradeNo)
				if inCS.Add(1) != 1 {
					violated.Store(true)
				}
				inCS.Add(-1)
				UnlockOrder(tradeNo)
			}
		}()
	}
	wg.Wait()
	if violated.Load() {
		t.Fatal("同一 tradeNo 的临界区出现重叠持有，互斥语义被破坏")
	}
}

// TestLockOrderDifferentTradeNosRunInParallel 选取哈希到不同分片的
// 订单号并发加锁，断言所有持有者同时进入临界区，验证不同订单号之间
// 不会被同一把锁串行阻塞。
func TestLockOrderDifferentTradeNosRunInParallel(t *testing.T) {
	var tradeNos []string
	seen := make(map[*sync.Mutex]struct{})
	for i := 0; len(tradeNos) < 8; i++ {
		tradeNo := fmt.Sprintf("parallel-trade-%d", i)
		lock := shardFor(tradeNo)
		if _, dup := seen[lock]; dup {
			continue
		}
		seen[lock] = struct{}{}
		tradeNos = append(tradeNos, tradeNo)
	}

	entered := make(chan struct{}, len(tradeNos))
	release := make(chan struct{})
	var wg sync.WaitGroup
	for _, tradeNo := range tradeNos {
		wg.Add(1)
		go func(tradeNo string) {
			defer wg.Done()
			LockOrder(tradeNo)
			defer UnlockOrder(tradeNo)
			entered <- struct{}{}
			<-release
		}(tradeNo)
	}

	for range tradeNos {
		select {
		case <-entered:
		case <-time.After(5 * time.Second):
			t.Fatal("不同分片的订单锁未能并行获取，出现串行阻塞")
		}
	}
	close(release)
	wg.Wait()
}

// TestOrderLockTableFootprintConstant 大量不同订单号反复加解锁之后，
// 锁表大小保持恒定，不随订单量增长。
func TestOrderLockTableFootprintConstant(t *testing.T) {
	var wg sync.WaitGroup
	for g := 0; g < 8; g++ {
		wg.Add(1)
		go func(seed int) {
			defer wg.Done()
			for i := 0; i < 500; i++ {
				tradeNo := fmt.Sprintf("footprint-trade-%d-%d", seed, i)
				LockOrder(tradeNo)
				UnlockOrder(tradeNo)
			}
		}(g)
	}
	wg.Wait()
	if len(orderLocks) != orderLockShards {
		t.Fatalf("锁表大小应恒为 %d，实际为 %d", orderLockShards, len(orderLocks))
	}
}
