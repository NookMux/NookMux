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

// Peek 按 Request 的规则判定：记录未满，或记录已满但最旧一条已离开窗口时放行；判定不改变窗口。
func TestInMemoryRateLimiterPeekDecidesWithoutRecording(t *testing.T) {
	const key, maximum, window = "model-request:peek", 2, int64(60)
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
			limiter := rateLimiterWithRecords(t, key, tc.ages...)
			var before []int64
			if queue, ok := limiter.store[key]; ok {
				before = append(before, *queue...)
			}

			if got := limiter.Peek(key, maximum, window); got != tc.allowed {
				t.Fatalf("Peek = %v, want %v", got, tc.allowed)
			}

			after, hasRecords := limiter.store[key]
			switch {
			case hasRecords != (len(tc.ages) > 0):
				t.Fatalf("window has records = %v after Peek, want %v", hasRecords, len(tc.ages) > 0)
			case hasRecords && fmt.Sprint(*after) != fmt.Sprint(before):
				t.Fatalf("records after Peek = %v, want %v", *after, before)
			}
		})
	}
}

// Record 不判定：窗口已满、按规则会拒绝时照样记入并丢掉最旧一条，窗口里始终是最新的记录。
func TestInMemoryRateLimiterRecordsWithoutDeciding(t *testing.T) {
	const key, maximum, window = "model-request:record", 2, int64(60)

	// 窗口里没有记录：记入第一条
	limiter := newRateLimiterForTest(t)
	limiter.Record(key, maximum)
	if got, want := len(*limiter.store[key]), 1; got != want {
		t.Fatalf("records after the first Record = %d, want %d", got, want)
	}

	// 记录未满：追加，原有记录保留
	writtenFrom := time.Now().Unix()
	limiter = rateLimiterWithRecords(t, key, 10)
	kept := (*limiter.store[key])[0]
	limiter.Record(key, maximum)
	records := *limiter.store[key]
	if len(records) != 2 || records[0] != kept || records[1] < writtenFrom {
		t.Fatalf("records after Record on a window with room = %v, want [%d, <now>]", records, kept)
	}

	// 记录已满且按规则会拒绝：照样记入，丢掉最旧一条
	writtenFrom = time.Now().Unix()
	limiter = rateLimiterWithRecords(t, key, 10, 5)
	if limiter.Peek(key, maximum, window) {
		t.Fatal("窗口已满且最旧一条仍在窗口内，Peek 应拒绝")
	}
	kept = (*limiter.store[key])[1]
	limiter.Record(key, maximum)
	records = *limiter.store[key]
	if len(records) != maximum || records[0] != kept || records[1] < writtenFrom {
		t.Fatalf("records after Record on a full window = %v, want [%d, <now>]", records, kept)
	}
}

// 先 Peek 后 Record 与一次 Request 的判定相同，写入的记录也相同。
func TestInMemoryRateLimiterPeekThenRecordWritesWhatRequestWrites(t *testing.T) {
	const key, maximum, window = "model-request:split", 2, int64(60)
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
			taking, split := rateLimiterWithRecords(t, key, tc.ages...), rateLimiterWithRecords(t, key, tc.ages...)

			if got := taking.Request(key, maximum, window); got != tc.allowed {
				t.Fatalf("Request = %v, want %v", got, tc.allowed)
			}
			peeked := split.Peek(key, maximum, window)
			if peeked != tc.allowed {
				t.Fatalf("Peek = %v, want %v", peeked, tc.allowed)
			}
			if peeked {
				split.Record(key, maximum)
			}

			if got, want := fmt.Sprint(allButLatest(*split.store[key])), fmt.Sprint(allButLatest(*taking.store[key])); got != want {
				t.Fatalf("records kept by Peek then Record = %s, want %s as Request keeps", got, want)
			}
			for _, limiter := range []*InMemoryRateLimiter{taking, split} {
				records := *limiter.store[key]
				if latest := records[len(records)-1]; latest < writtenFrom {
					t.Fatalf("latest record = %d, want a time no earlier than %d", latest, writtenFrom)
				}
			}
		})
	}
}

// Peek、Record 与 Request 共用同一把锁：并发使用后窗口里只有最新的记录，条数不超过上限。
func TestInMemoryRateLimiterPeekAndRecordAreSafeForConcurrentUse(t *testing.T) {
	const key, maximum, window = "model-request:concurrent", 7, int64(60)
	limiter := newRateLimiterForTest(t)

	var wg sync.WaitGroup
	for worker := 0; worker < 16; worker++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < 200; i++ {
				if limiter.Peek(key, maximum, window) {
					limiter.Record(key, maximum)
				}
				limiter.Request(key, maximum, window)
			}
		}()
	}
	wg.Wait()

	if got, want := len(*limiter.store[key]), maximum; got != want {
		t.Fatalf("records after concurrent use = %d, want the latest %d", got, want)
	}
}
