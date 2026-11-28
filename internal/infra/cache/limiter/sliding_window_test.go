package limiter

import (
	"context"
	"fmt"
	"reflect"
	"strconv"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/go-redis/redis/v8"
)

// slidingWindowClock 拨快内嵌 Redis 的时间。内嵌 Redis 的键生存期不随时钟走，需要与时钟一并扣减，
// 两者同步推进才等价于真实 Redis 里时间流逝。
type slidingWindowClock struct {
	server *miniredis.Miniredis
	now    time.Time
}

func (c *slidingWindowClock) advance(d time.Duration) {
	c.now = c.now.Add(d)
	c.server.SetTime(c.now)
	c.server.FastForward(d)
}

func newSlidingWindowRedis(t *testing.T) (*slidingWindowClock, *redis.Client) {
	t.Helper()

	clock := &slidingWindowClock{server: miniredis.RunT(t), now: time.Date(2026, 11, 27, 10, 0, 0, 0, time.UTC)}
	clock.server.SetTime(clock.now)
	client := redis.NewClient(&redis.Options{Addr: clock.server.Addr()})
	t.Cleanup(func() { _ = client.Close() })
	return clock, client
}

func TestSlidingWindowTakeCountsWindowFromOldestRecord(t *testing.T) {
	clock, client := newSlidingWindowRedis(t)
	ctx := context.Background()
	const key, maximum, window = "sliding-window:oldest", 2, 37 * time.Second

	take := func() bool {
		t.Helper()
		allowed, err := SlidingWindowTake(ctx, client, key, maximum, window)
		if err != nil {
			t.Fatalf("SlidingWindowTake: %v", err)
		}
		return allowed
	}

	// 窗口未满时放行，放行即记录，计数的生存期等于窗口长度
	if !take() {
		t.Fatal("first take rejected, want allowed")
	}
	clock.advance(20 * time.Second)
	if !take() {
		t.Fatal("second take rejected, want allowed")
	}
	if got := clock.server.TTL(key); got != window {
		t.Fatalf("TTL after an allowed take = %v, want %v", got, window)
	}

	// 记录数达到上限且最旧一条仍在窗口内：拒绝，被拒的请求不续期
	clock.advance(10 * time.Second)
	if take() {
		t.Fatal("take beyond the maximum inside the window allowed, want rejected")
	}
	if got, want := clock.server.TTL(key), window-10*time.Second; got != want {
		t.Fatalf("TTL after a rejected take = %v, want %v", got, want)
	}

	// 窗口按最旧一条记录计时：最旧记录满一个窗口后放行一次，下一条记录仍在窗口内则继续拒绝
	clock.advance(7 * time.Second)
	if !take() {
		t.Fatal("take after the oldest record left the window rejected, want allowed")
	}
	clock.advance(time.Second)
	if take() {
		t.Fatal("take while the second record is still inside the window allowed, want rejected")
	}
	if got, want := clock.server.TTL(key), window-time.Second; got != want {
		t.Fatalf("TTL after a rejected take = %v, want %v", got, want)
	}
	clock.advance(19 * time.Second)
	if !take() {
		t.Fatal("take after the second record left the window rejected, want allowed")
	}

	// 一个窗口内没有放行时计数到期，不在 Redis 里残留
	clock.advance(window)
	if clock.server.Exists(key) {
		t.Fatal("counter still present one window after the last allowed take")
	}
	if !take() {
		t.Fatal("take after the counter expired rejected, want allowed")
	}
}

func TestSlidingWindowTakeStoresServerClockInMillis(t *testing.T) {
	clock, client := newSlidingWindowRedis(t)
	ctx := context.Background()
	const key = "sliding-window:clock"

	clock.advance(1500 * time.Millisecond)
	if allowed, err := SlidingWindowTake(ctx, client, key, 1, time.Minute); err != nil || !allowed {
		t.Fatalf("SlidingWindowTake: allowed = %v, err = %v, want allowed", allowed, err)
	}
	values, err := client.LRange(ctx, key, 0, -1).Result()
	if err != nil {
		t.Fatalf("LRange: %v", err)
	}
	// 记录是 Redis 服务端时钟的毫秒时间戳，与实例自己的时钟无关
	want := strconv.FormatInt(clock.now.UnixMilli(), 10)
	if len(values) != 1 || values[0] != want {
		t.Fatalf("stored records = %v, want [%s]", values, want)
	}
}

func TestSlidingWindowTakeIsAtomicUnderConcurrency(t *testing.T) {
	_, client := newSlidingWindowRedis(t)
	const requests, maximum = 20, 7
	key := "sliding-window:concurrent"

	var allowed atomic.Int64
	var wg sync.WaitGroup
	failures := make(chan error, requests)
	for i := 0; i < requests; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			ok, err := SlidingWindowTake(context.Background(), client, key, maximum, time.Minute)
			if err != nil {
				failures <- err
				return
			}
			if ok {
				allowed.Add(1)
			}
		}()
	}
	wg.Wait()
	close(failures)
	for err := range failures {
		t.Fatalf("SlidingWindowTake under concurrency: %v", err)
	}
	if allowed.Load() != maximum {
		t.Fatalf("allowed %d of %d concurrent takes, want exactly %d", allowed.Load(), requests, maximum)
	}
	if got, err := client.LLen(context.Background(), key).Result(); err != nil || got != maximum {
		t.Fatalf("stored records = %d, %v, want %d", got, err, maximum)
	}
}

// 占位与了结：reserve 判定并记录，返回写入窗口的时间戳；窗口已满时返回 nil，不续期。Complete(true) 保留
// 记录，Complete(false) 只撤回自己那一条，且只生效一次；撤回后窗口放得下新请求。
func TestSlidingWindowReservationKeepsOrWithdrawsItsOwnRecord(t *testing.T) {
	clock, client := newSlidingWindowRedis(t)
	ctx := context.Background()
	const key, maximum, window = "sliding-window:reservation", 2, time.Minute

	reserve := func() *SlidingWindowReservation {
		t.Helper()
		slot, err := SlidingWindowReserve(ctx, client, key, maximum, window)
		if err != nil {
			t.Fatalf("SlidingWindowReserve: %v", err)
		}
		return slot
	}
	complete := func(slot *SlidingWindowReservation, succeeded bool) {
		t.Helper()
		if err := slot.Complete(ctx, succeeded); err != nil {
			t.Fatalf("Complete(%v): %v", succeeded, err)
		}
	}
	records := func() []string {
		t.Helper()
		values, err := client.LRange(ctx, key, 0, -1).Result()
		if err != nil {
			t.Fatalf("LRange: %v", err)
		}
		return values
	}

	// 窗口未满：两次占位都放行，写入的是 Redis 服务端时钟的毫秒时间戳
	first := reserve()
	if first == nil {
		t.Fatal("first reserve rejected, want allowed")
	}
	clock.advance(20 * time.Second)
	second := reserve()
	if second == nil {
		t.Fatal("second reserve rejected, want allowed")
	}
	newest := strconv.FormatInt(clock.now.UnixMilli(), 10)
	older := strconv.FormatInt(clock.now.Add(-20*time.Second).UnixMilli(), 10)
	if got := records(); !reflect.DeepEqual(got, []string{newest, older}) {
		t.Fatalf("records = %v, want [%s %s]", got, newest, older)
	}
	if first.stamp != older || second.stamp != newest {
		t.Fatalf("stamps of the reservations = %q, %q, want %q, %q", first.stamp, second.stamp, older, newest)
	}

	// 窗口已满且最旧一条仍在窗口内：拒绝，不续期
	clock.advance(10 * time.Second)
	if third := reserve(); third != nil {
		t.Fatal("reserve beyond the maximum inside the window allowed, want rejected")
	}
	if got, want := clock.server.TTL(key), window-10*time.Second; got != want {
		t.Fatalf("TTL after a rejected reserve = %v, want %v", got, want)
	}

	// 成功的请求保留记录；再次了结不改变窗口
	complete(first, true)
	complete(first, false)
	if got := records(); len(got) != 2 {
		t.Fatalf("records after completing a successful request = %v, want both kept", got)
	}

	// 失败的请求只撤回自己那一条，再次了结不再撤回；撤回后窗口放得下新请求
	complete(second, false)
	complete(second, false)
	if got := records(); !reflect.DeepEqual(got, []string{older}) {
		t.Fatalf("records after completing a failed request = %v, want [%s]", got, older)
	}
	third := reserve()
	if third == nil {
		t.Fatal("reserve after a withdrawn reservation rejected, want allowed")
	}
	if got := records(); len(got) != 2 {
		t.Fatalf("records after a new reserve = %v, want two", got)
	}

	// 记录离开窗口、计数到期后再了结：没有可撤回的记录，不报错
	clock.advance(window)
	complete(third, false)
	if clock.server.Exists(key) {
		t.Fatal("completing a reservation whose record left the window recreated the counter")
	}
}

// 并发占位不会各自放行：同时发起的占位只有上限个成功；了结为失败的占位让出名额后，新的占位才能放行。
func TestSlidingWindowReserveIsAtomicUnderConcurrency(t *testing.T) {
	_, client := newSlidingWindowRedis(t)
	ctx := context.Background()
	const requests, maximum = 20, 7
	key := "sliding-window:concurrent-reserve"

	slots := make(chan *SlidingWindowReservation, requests)
	failures := make(chan error, requests)
	var wg sync.WaitGroup
	for i := 0; i < requests; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			slot, err := SlidingWindowReserve(ctx, client, key, maximum, time.Minute)
			if err != nil {
				failures <- err
				return
			}
			if slot != nil {
				slots <- slot
			}
		}()
	}
	wg.Wait()
	close(failures)
	close(slots)
	for err := range failures {
		t.Fatalf("SlidingWindowReserve under concurrency: %v", err)
	}
	var held []*SlidingWindowReservation
	for slot := range slots {
		held = append(held, slot)
	}
	if len(held) != maximum {
		t.Fatalf("reserved %d of %d concurrent requests, want exactly %d", len(held), requests, maximum)
	}

	// 三个占位了结为失败：让出三个名额，之后的三个占位放行，第四个被拒
	for _, slot := range held[:3] {
		if err := slot.Complete(ctx, false); err != nil {
			t.Fatalf("Complete(false): %v", err)
		}
	}
	if got, err := client.LLen(ctx, key).Result(); err != nil || got != maximum-3 {
		t.Fatalf("stored records after three withdrawals = %d, %v, want %d", got, err, maximum-3)
	}
	for i := 0; i < 3; i++ {
		if slot, err := SlidingWindowReserve(ctx, client, key, maximum, time.Minute); err != nil || slot == nil {
			t.Fatalf("reserve %d after the withdrawals: slot = %v, err = %v, want a reservation", i+1, slot, err)
		}
	}
	if slot, err := SlidingWindowReserve(ctx, client, key, maximum, time.Minute); err != nil || slot != nil {
		t.Fatalf("reserve beyond the maximum: slot = %v, err = %v, want rejected", slot, err)
	}
	if got, err := client.LLen(ctx, key).Result(); err != nil || got != maximum {
		t.Fatalf("stored records = %d, %v, want %d", got, err, maximum)
	}
}

func TestSlidingWindowRejectsInvalidArguments(t *testing.T) {
	_, client := newSlidingWindowRedis(t)
	ctx := context.Background()

	cases := []struct {
		name    string
		client  redis.Scripter
		key     string
		maximum int
		window  time.Duration
	}{
		{"missing client", nil, "sliding-window:invalid", 1, time.Minute},
		{"empty key", client, "", 1, time.Minute},
		{"non-positive maximum", client, "sliding-window:invalid", 0, time.Minute},
	}
	for _, tc := range cases {
		if _, err := SlidingWindowTake(ctx, tc.client, tc.key, tc.maximum, tc.window); err == nil {
			t.Errorf("%s: SlidingWindowTake returned no error", tc.name)
		}
		if _, err := SlidingWindowReserve(ctx, tc.client, tc.key, tc.maximum, tc.window); err == nil {
			t.Errorf("%s: SlidingWindowReserve returned no error", tc.name)
		}
	}
}

func TestSlidingWindowReportsUnusableCounters(t *testing.T) {
	_, client := newSlidingWindowRedis(t)
	ctx := context.Background()
	key := "sliding-window:unusable"

	takes := []struct {
		name string
		call func() error
	}{
		{"SlidingWindowTake", func() error { _, err := SlidingWindowTake(ctx, client, key, 1, time.Minute); return err }},
		{"SlidingWindowReserve", func() error { _, err := SlidingWindowReserve(ctx, client, key, 1, time.Minute); return err }},
	}
	withdraw := func() error {
		slot := &SlidingWindowReservation{rdb: client, key: key, maxRequestNum: 1, window: time.Minute, stamp: "1"}
		return slot.Complete(ctx, false)
	}

	// 键上不是列表：判定与撤回都报错
	if err := client.Set(ctx, key, "not-a-list", 0).Err(); err != nil {
		t.Fatalf("seed a string key: %v", err)
	}
	for _, tc := range takes {
		if err := tc.call(); err == nil {
			t.Errorf("%s on a string key returned no error", tc.name)
		}
	}
	if err := withdraw(); err == nil {
		t.Error("Complete on a string key returned no error")
	}

	// 列表元素不是时间戳
	if err := client.Del(ctx, key).Err(); err != nil {
		t.Fatalf("remove the string key: %v", err)
	}
	if err := client.RPush(ctx, key, "2026-11-27T10:00:00.000Z").Err(); err != nil {
		t.Fatalf("seed a non-numeric record: %v", err)
	}
	for _, tc := range takes {
		if err := tc.call(); err == nil {
			t.Errorf("%s on a non-numeric record returned no error", tc.name)
		}
	}
}

// 窗口不是正数表示不限流，判定与内存限流器一致；Redis 侧放行且不留记录。
func TestSlidingWindowDoesNotLimitWhenTheWindowIsNotPositive(t *testing.T) {
	clock, client := newSlidingWindowRedis(t)
	ctx := context.Background()

	for _, window := range []time.Duration{0, -time.Minute} {
		key := fmt.Sprintf("sliding-window:unlimited:%d", window)
		for i := 0; i < 4; i++ {
			if allowed, err := SlidingWindowTake(ctx, client, key, 1, window); err != nil || !allowed {
				t.Fatalf("take %d with window %v: allowed = %v, err = %v, want allowed", i+1, window, allowed, err)
			}
			slot, err := SlidingWindowReserve(ctx, client, key, 1, window)
			if err != nil || slot == nil {
				t.Fatalf("reserve %d with window %v: slot = %v, err = %v, want a reservation", i+1, window, slot, err)
			}
			if err := slot.Complete(ctx, false); err != nil {
				t.Fatalf("completing a reservation with window %v: %v", window, err)
			}
		}
		if clock.server.Exists(key) {
			t.Fatalf("window %v left a counter in redis", window)
		}
	}
}
