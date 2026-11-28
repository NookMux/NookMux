package common

import (
	"fmt"
	"sync"
	"testing"
	"time"
)

// newRateLimiterForTest 返回不启动清扫协程的限流器：窗口里的记录只随用例的操作变化。
func newRateLimiterForTest(t *testing.T) *InMemoryRateLimiter {
	t.Helper()

	limiter := &InMemoryRateLimiter{}
	limiter.Init(0)
	t.Cleanup(limiter.Reset)
	return limiter
}

// rateLimiterWithRecords 返回 key 的窗口里已放好记录的限流器。ages 是各条记录距今的秒数，
// 由旧到新，与窗口里的存放顺序一致；不给 ages 时窗口里没有记录。
func rateLimiterWithRecords(t *testing.T, key string, ages ...int64) *InMemoryRateLimiter {
	t.Helper()

	limiter := newRateLimiterForTest(t)
	if len(ages) == 0 {
		return limiter
	}
	now := time.Now().Unix()
	records := make([]int64, 0, len(ages))
	for _, age := range ages {
		records = append(records, now-age)
	}
	limiter.store[key] = &records
	return limiter
}

// allButLatest 返回窗口里除最新一条以外的记录。最新一条是写入时刻的秒数，两次写入可能隔着整秒。
func allButLatest(records []int64) []int64 {
	return records[:len(records)-1]
}

// Reserve 与 Request 的判定相同：记录未满，或记录已满但最旧一条已离开窗口时放行，被拒时不改变窗口；
// 放行时写入的记录也相同。
func TestInMemoryRateLimiterReserveDecidesAsRequestDoes(t *testing.T) {
	const key, maximum, window = "model-request:reserve", 2, int64(60)
	cases := []struct {
		name    string
		ages    []int64 // 窗口里已有的记录，距今的秒数
		allowed bool
	}{
		{name: "没有记录", allowed: true},
		{name: "记录未满", ages: []int64{0}, allowed: true},
		{name: "记录未满且已有记录离开窗口", ages: []int64{window + 50}, allowed: true},
		{name: "记录已满且最旧一条仍在窗口内", ages: []int64{10, 0}, allowed: false},
		{name: "记录已满且最旧一条已离开窗口", ages: []int64{window + 50, 0}, allowed: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			writtenFrom := time.Now().Unix()
			requesting, reserving := rateLimiterWithRecords(t, key, tc.ages...), rateLimiterWithRecords(t, key, tc.ages...)
			var before []int64
			if queue, ok := reserving.store[key]; ok {
				before = append(before, *queue...)
			}

			if got := requesting.Request(key, maximum, window); got != tc.allowed {
				t.Fatalf("Request = %v, want %v", got, tc.allowed)
			}
			slot := reserving.Reserve(key, maximum, window)
			if (slot != nil) != tc.allowed {
				t.Fatalf("Reserve returned a reservation = %v, want %v", slot != nil, tc.allowed)
			}
			if !tc.allowed {
				if got, want := fmt.Sprint(*reserving.store[key]), fmt.Sprint(before); got != want {
					t.Fatalf("records after a rejected Reserve = %s, want %s", got, want)
				}
				return
			}
			if got, want := fmt.Sprint(allButLatest(*reserving.store[key])), fmt.Sprint(allButLatest(*requesting.store[key])); got != want {
				t.Fatalf("records kept by Reserve = %s, want %s as Request keeps", got, want)
			}
			for _, limiter := range []*InMemoryRateLimiter{requesting, reserving} {
				records := *limiter.store[key]
				if latest := records[len(records)-1]; latest < writtenFrom {
					t.Fatalf("latest record = %d, want a time no earlier than %d", latest, writtenFrom)
				}
			}
		})
	}
}

// 占位的了结：成功的请求保留记录，失败的请求只撤回自己那一条，且只生效一次；撤回后窗口放得下新请求。
func TestInMemoryRateLimiterReservationKeepsOrWithdrawsItsOwnRecord(t *testing.T) {
	const key, maximum, window = "model-request:complete", 2, int64(60)
	records := func(limiter *InMemoryRateLimiter) string {
		queue, ok := limiter.store[key]
		if !ok {
			return "no records"
		}
		return fmt.Sprint(*queue)
	}

	// 窗口里放好两条不同时刻的记录，各对应一个占位
	limiter := newRateLimiterForTest(t)
	older, newer := time.Now().Unix()-30, time.Now().Unix()-5
	limiter.store[key] = &[]int64{newer, older}
	kept := &RateLimitReservation{limiter: limiter, key: key, stamp: older}
	failed := &RateLimitReservation{limiter: limiter, key: key, stamp: newer}

	// 成功的请求保留记录；再次了结不改变窗口
	kept.Complete(true)
	kept.Complete(false)
	if got, want := records(limiter), fmt.Sprint([]int64{newer, older}); got != want {
		t.Fatalf("records after completing a successful request = %s, want %s", got, want)
	}

	// 失败的请求只撤回自己那一条；再次了结不再撤回。撤回后窗口放得下新请求
	failed.Complete(false)
	failed.Complete(false)
	if got, want := records(limiter), fmt.Sprint([]int64{older}); got != want {
		t.Fatalf("records after completing a failed request = %s, want %s", got, want)
	}
	if limiter.Reserve(key, maximum, window) == nil {
		t.Fatal("reserve after a withdrawn reservation rejected, want allowed")
	}
	if got, want := len(*limiter.store[key]), 2; got != want {
		t.Fatalf("records after a new reserve = %d, want %d", got, want)
	}

	// Reserve 占下的记录随 Complete(false) 撤回：窗口满时拒绝，撤回一条后恢复放行
	limiter = newRateLimiterForTest(t)
	first, second := limiter.Reserve(key, maximum, window), limiter.Reserve(key, maximum, window)
	if first == nil || second == nil {
		t.Fatal("reserve on an empty window rejected, want allowed")
	}
	if limiter.Reserve(key, maximum, window) != nil {
		t.Fatal("reserve on a full window allowed, want rejected")
	}
	first.Complete(false)
	if got, want := records(limiter), fmt.Sprint([]int64{second.stamp}); got != want {
		t.Fatalf("records after withdrawing the first reservation = %s, want only the second's record %d", got, second.stamp)
	}
	if limiter.Reserve(key, maximum, window) == nil {
		t.Fatal("reserve after a withdrawn reservation rejected, want allowed")
	}

	// 记录已被清扫删除后再撤回：不做任何事。没有占位时了结也不做任何事
	delete(limiter.store, key)
	second.Complete(false)
	if _, ok := limiter.store[key]; ok {
		t.Fatal("completing a reservation whose records were swept recreated the window")
	}
	var none *RateLimitReservation
	none.Complete(false)
}

// Reserve、Complete 与 Request 共用同一把锁：并发使用时窗口里的记录不超过上限，且保持由旧到新。
func TestInMemoryRateLimiterReserveAndCompleteAreSafeForConcurrentUse(t *testing.T) {
	const key, maximum, window = "model-request:concurrent", 7, int64(60)
	limiter := newRateLimiterForTest(t)

	var wg sync.WaitGroup
	for worker := 0; worker < 16; worker++ {
		wg.Add(1)
		go func(worker int) {
			defer wg.Done()
			for i := 0; i < 200; i++ {
				switch (worker + i) % 3 {
				case 0:
					limiter.Request(key, maximum, window)
				case 1, 2:
					if slot := limiter.Reserve(key, maximum, window); slot != nil {
						slot.Complete((worker+i)%3 == 1)
					}
				}
			}
		}(worker)
	}
	wg.Wait()
	limiter.Request(key, maximum, window)

	queue, ok := limiter.store[key]
	if !ok || len(*queue) == 0 || len(*queue) > maximum {
		t.Fatalf("records after concurrent use = %v, want between 1 and %d", queue, maximum)
	}
	stored := *queue
	for i := 1; i < len(stored); i++ {
		if stored[i] < stored[i-1] {
			t.Fatalf("records are not ordered from oldest to newest: %v", stored)
		}
	}
}
