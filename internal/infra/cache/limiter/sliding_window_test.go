package limiter

import (
	"context"
	"fmt"
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

func TestSlidingWindowPeekAndRecordSplitDecisionFromRecording(t *testing.T) {
	clock, client := newSlidingWindowRedis(t)
	ctx := context.Background()
	const key, maximum, window = "sliding-window:peek", 2, time.Minute

	// peek 只判定：窗口未满时放行且不写入记录
	if allowed, err := SlidingWindowPeek(ctx, client, key, maximum, window); err != nil || !allowed {
		t.Fatalf("SlidingWindowPeek on a window with room: allowed = %v, err = %v, want allowed", allowed, err)
	}
	if clock.server.Exists(key) {
		t.Fatal("SlidingWindowPeek wrote a record")
	}

	// record 只记录，不判定
	if err := SlidingWindowRecord(ctx, client, key, maximum, window); err != nil {
		t.Fatalf("SlidingWindowRecord: %v", err)
	}
	clock.advance(20 * time.Second)
	if err := SlidingWindowRecord(ctx, client, key, maximum, window); err != nil {
		t.Fatalf("SlidingWindowRecord: %v", err)
	}
	if got, err := client.LLen(ctx, key).Result(); err != nil || got != maximum {
		t.Fatalf("stored records after two records = %d, %v, want %d", got, err, maximum)
	}

	// 窗口满后 peek 拒绝且不续期
	clock.advance(30 * time.Second)
	if allowed, err := SlidingWindowPeek(ctx, client, key, maximum, window); err != nil || allowed {
		t.Fatalf("SlidingWindowPeek on a full window: allowed = %v, err = %v, want rejected", allowed, err)
	}
	if got, want := clock.server.TTL(key), 30*time.Second; got != want {
		t.Fatalf("TTL after a rejected peek = %v, want %v", got, want)
	}

	// 最旧记录满一个窗口后 peek 重新放行，仍然不写入记录
	clock.advance(10 * time.Second)
	if allowed, err := SlidingWindowPeek(ctx, client, key, maximum, window); err != nil || !allowed {
		t.Fatalf("SlidingWindowPeek after the oldest record left the window: allowed = %v, err = %v, want allowed", allowed, err)
	}
	if got, err := client.LLen(ctx, key).Result(); err != nil || got != maximum {
		t.Fatalf("stored records after an allowed peek = %d, %v, want %d", got, err, maximum)
	}
	if got, want := clock.server.TTL(key), 20*time.Second; got != want {
		t.Fatalf("TTL after an allowed peek = %v, want %v", got, want)
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
		if _, err := SlidingWindowPeek(ctx, tc.client, tc.key, tc.maximum, tc.window); err == nil {
			t.Errorf("%s: SlidingWindowPeek returned no error", tc.name)
		}
		if err := SlidingWindowRecord(ctx, tc.client, tc.key, tc.maximum, tc.window); err == nil {
			t.Errorf("%s: SlidingWindowRecord returned no error", tc.name)
		}
	}
}

func TestSlidingWindowReportsUnusableCounters(t *testing.T) {
	_, client := newSlidingWindowRedis(t)
	ctx := context.Background()
	key := "sliding-window:unusable"

	// 键上不是列表
	if err := client.Set(ctx, key, "not-a-list", 0).Err(); err != nil {
		t.Fatalf("seed a string key: %v", err)
	}
	if _, err := SlidingWindowTake(ctx, client, key, 1, time.Minute); err == nil {
		t.Fatal("SlidingWindowTake on a string key returned no error")
	}

	// 列表元素不是时间戳
	if err := client.Del(ctx, key).Err(); err != nil {
		t.Fatalf("remove the string key: %v", err)
	}
	if err := client.RPush(ctx, key, "2026-11-27T10:00:00.000Z").Err(); err != nil {
		t.Fatalf("seed a non-numeric record: %v", err)
	}
	if _, err := SlidingWindowTake(ctx, client, key, 1, time.Minute); err == nil {
		t.Fatal("SlidingWindowTake on a non-numeric record returned no error")
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
		}
		if allowed, err := SlidingWindowPeek(ctx, client, key, 1, window); err != nil || !allowed {
			t.Fatalf("peek with window %v: allowed = %v, err = %v, want allowed", window, allowed, err)
		}
		if err := SlidingWindowRecord(ctx, client, key, 1, window); err != nil {
			t.Fatalf("record with window %v: %v", window, err)
		}
		if clock.server.Exists(key) {
			t.Fatalf("window %v left a counter in redis", window)
		}
	}
}
