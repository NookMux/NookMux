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
	stamp, unlimited, err := reserveSlidingWindow(ctx, rdb, key, maxRequestNum, window)
	return unlimited || stamp != "", err
}

// SlidingWindowReservation 是 SlidingWindowReserve 在窗口里占下的一条记录，对应一个开始处理的请求。
type SlidingWindowReservation struct {
	rdb           redis.Scripter
	key           string
	maxRequestNum int
	window        time.Duration
	stamp         string // 占位时写入窗口的时间戳；窗口不是正数时没有写记录，为空串
	settled       bool
}

// SlidingWindowReserve 按 SlidingWindowTake 的规则判定并记录一次请求，返回占下的记录；窗口已满时返回 nil。
// 供请求处理完毕后才知道成败的限额使用：处理前占位，处理完毕后由 Complete 保留或撤回。
func SlidingWindowReserve(ctx context.Context, rdb redis.Scripter, key string, maxRequestNum int, window time.Duration) (*SlidingWindowReservation, error) {
	stamp, unlimited, err := reserveSlidingWindow(ctx, rdb, key, maxRequestNum, window)
	if err != nil || (stamp == "" && !unlimited) {
		return nil, err
	}
	return &SlidingWindowReservation{rdb: rdb, key: key, maxRequestNum: maxRequestNum, window: window, stamp: stamp}, nil
}

// Complete 了结占位，只生效一次：succeeded 为 true 时记录留在窗口里，否则把这条记录撤回，失败的请求
// 不占额度。记录已离开窗口或计数已到期时没有可撤回的记录，同样返回 nil。nil 占位的 Complete 不做任何事。
func (r *SlidingWindowReservation) Complete(ctx context.Context, succeeded bool) error {
	if r == nil || r.settled {
		return nil
	}
	r.settled = true
	if succeeded || r.stamp == "" {
		return nil
	}
	if _, err := slidingWindow.Run(ctx, r.rdb, []string{r.key}, r.maxRequestNum, r.window.Milliseconds(), "release", r.stamp).Int(); err != nil {
		return fmt.Errorf("sliding window release failed (key=%s, record=%s): %w", r.key, r.stamp, err)
	}
	return nil
}

// reserveSlidingWindow 执行脚本的占位操作，返回写入窗口的时间戳。窗口不是正数表示不限流，判定与内存
// 限流器一致：直接放行、不写记录，此时时间戳为空串、unlimited 为 true。
func reserveSlidingWindow(ctx context.Context, rdb redis.Scripter, key string, maxRequestNum int, window time.Duration) (stamp string, unlimited bool, err error) {
	if rdb == nil {
		return "", false, errors.New("redis client is not initialized")
	}
	if key == "" {
		return "", false, errors.New("rate limit key is empty")
	}
	if maxRequestNum <= 0 {
		return "", false, fmt.Errorf("rate limit maximum must be positive, got %d", maxRequestNum)
	}
	if window <= 0 {
		return "", true, nil
	}

	stamp, err = slidingWindow.Run(ctx, rdb, []string{key}, maxRequestNum, window.Milliseconds(), "reserve").Text()
	if err != nil {
		return "", false, fmt.Errorf("sliding window reserve failed: %w", err)
	}
	return stamp, false, nil
}
