package limiter

import (
	"context"
	_ "embed"
	"errors"
	"fmt"
	"time"

	"github.com/go-redis/redis/v8"
)

//go:embed lua/sliding_window.lua
var slidingWindowScript string

var slidingWindow = redis.NewScript(slidingWindowScript)

// SlidingWindowTake 在一次原子操作里判定并记录一次请求：窗口内已放行的记录数达到 maxRequestNum，
// 且其中最旧一条距今不足 window 时拒绝；其余情况放行，并把当前时刻记入窗口。
// window 不大于 0 表示不限流：直接放行，不写记录。
func SlidingWindowTake(ctx context.Context, rdb redis.Scripter, key string, maxRequestNum int, window time.Duration) (bool, error) {
	return runSlidingWindow(ctx, rdb, key, maxRequestNum, window, "take")
}

// SlidingWindowPeek 只按 SlidingWindowTake 的规则判定，不写入记录，供请求处理完毕后才计入的调用方
// 在处理前预判。
func SlidingWindowPeek(ctx context.Context, rdb redis.Scripter, key string, maxRequestNum int, window time.Duration) (bool, error) {
	return runSlidingWindow(ctx, rdb, key, maxRequestNum, window, "peek")
}

// SlidingWindowRecord 只把当前时刻记入窗口，不判定。
func SlidingWindowRecord(ctx context.Context, rdb redis.Scripter, key string, maxRequestNum int, window time.Duration) error {
	_, err := runSlidingWindow(ctx, rdb, key, maxRequestNum, window, "record")
	return err
}

func runSlidingWindow(ctx context.Context, rdb redis.Scripter, key string, maxRequestNum int, window time.Duration, operation string) (bool, error) {
	if rdb == nil {
		return false, errors.New("redis client is not initialized")
	}
	if key == "" {
		return false, errors.New("rate limit key is empty")
	}
	if maxRequestNum <= 0 {
		return false, fmt.Errorf("rate limit maximum must be positive, got %d", maxRequestNum)
	}
	// 窗口不是正数表示不限流，与内存限流器的规则一致：直接放行，不判定也不写记录。
	if window <= 0 {
		return true, nil
	}

	allowed, err := slidingWindow.Run(ctx, rdb, []string{key}, maxRequestNum, window.Milliseconds(), operation).Int()
	if err != nil {
		return false, fmt.Errorf("sliding window %s failed: %w", operation, err)
	}
	return allowed == 1, nil
}
