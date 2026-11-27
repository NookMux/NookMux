package limiter

import (
	"context"
	"sync"
	"testing"
)

// 模型请求限流的周期填 0 时，中间件向令牌桶请求 0 个令牌，每个请求都放行。
// New 返回进程内唯一的实例并绑定第一个传入的客户端，用例开头重置它，结果不依赖用例顺序。
func TestRedisLimiterAllowsEveryRequestWhenNothingIsRequested(t *testing.T) {
	_, client := newSlidingWindowRedis(t)
	once, instance = sync.Once{}, nil
	ctx := context.Background()
	bucket := New(ctx, client)

	for i := 0; i < 5; i++ {
		allowed, err := bucket.Allow(ctx, "rate-limit:zero-period", WithCapacity(0), WithRate(3), WithRequested(0))
		if err != nil || !allowed {
			t.Fatalf("request %d: allowed = %v, err = %v, want allowed", i+1, allowed, err)
		}
	}

	// 对照：每次请求 1 个令牌时，额度用完即被拒。内嵌 Redis 的时钟不走，桶不会补充令牌。
	for i := 0; i < 3; i++ {
		allowed, err := bucket.Allow(ctx, "rate-limit:one-token", WithCapacity(2), WithRate(3), WithRequested(1))
		if err != nil {
			t.Fatalf("control request %d: %v", i+1, err)
		}
		if want := i < 2; allowed != want {
			t.Fatalf("control request %d: allowed = %v, want %v", i+1, allowed, want)
		}
	}
}
