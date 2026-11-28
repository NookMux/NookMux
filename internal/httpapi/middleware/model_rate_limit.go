package middleware

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/NookMux/NookMux/internal/common"
	"github.com/NookMux/NookMux/internal/config"
	"github.com/NookMux/NookMux/internal/i18n"
	"github.com/NookMux/NookMux/internal/infra/cache/limiter"
	"github.com/NookMux/NookMux/internal/infra/log"

	"github.com/NookMux/NookMux/internal/httpapi"
	infraredis "github.com/NookMux/NookMux/internal/infra/redis"
	"github.com/gin-gonic/gin"
	"github.com/go-redis/redis/v8"
)

const (
	ModelRequestRateLimitCountMark        = "MRRL"
	ModelRequestRateLimitSuccessCountMark = "MRRLS"
)

// 检查Redis中的请求限制：只判定，请求成功后才由 recordRedisRequest 记入窗口
func checkRedisRateLimit(ctx context.Context, rdb *redis.Client, key string, maxCount int, duration int64) (bool, error) {
	// 如果maxCount为0，表示不限制
	if maxCount == 0 {
		return true, nil
	}

	return limiter.SlidingWindowPeek(ctx, rdb, key, maxCount, time.Duration(duration)*time.Second)
}

// 记录Redis请求
func recordRedisRequest(ctx context.Context, rdb *redis.Client, key string, maxCount int, duration int64) {
	// 如果maxCount为0，不记录请求
	if maxCount == 0 {
		return
	}

	if err := limiter.SlidingWindowRecord(ctx, rdb, key, maxCount, time.Duration(duration)*time.Second); err != nil {
		log.LogError(ctx, fmt.Sprintf("rate limit record failed (key=%s): %v", key, err))
	}
}

// abortModelRequestRateLimited 以 429 终止模型请求，两种存储模式的响应相同：响应体说明命中的限额，
// Retry-After 给出完整的限制周期（秒）。
func abortModelRequestRateLimited(c *gin.Context, messageKey string, maxCount int, duration int64) {
	setRetryAfter(c, duration)
	abortWithOpenAiMessage(c, http.StatusTooManyRequests, i18n.T(c, messageKey, map[string]any{
		"Minutes": config.ModelRequestRateLimitDurationMinutes,
		"Max":     maxCount,
	}))
}

// Redis限流处理器
func redisRateLimitHandler(duration int64, totalMaxCount, successMaxCount int) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := context.Background()
		rdb := infraredis.RDB

		// 1. 检查成功请求数限制
		successKey := redisUserRateLimitKey(ModelRequestRateLimitSuccessCountMark, c.GetInt("id"))
		allowed, err := checkRedisRateLimit(ctx, rdb, successKey, successMaxCount, duration)
		if err != nil {
			fmt.Println("检查成功请求数限制失败:", err.Error())
			abortWithOpenAiMessage(c, http.StatusInternalServerError, i18n.T(c, i18n.MsgRateLimitCheckFailed))
			return
		}
		if !allowed {
			abortModelRequestRateLimited(c, i18n.MsgRateLimitSuccessReached, successMaxCount, duration)
			return
		}

		// 2. 检查总请求数限制并记录总请求（当totalMaxCount为0时跳过）
		if totalMaxCount > 0 {
			totalKey := redisUserRateLimitKey(ModelRequestRateLimitCountMark, c.GetInt("id"))
			allowed, err := limiter.SlidingWindowTake(ctx, rdb, totalKey, totalMaxCount, time.Duration(duration)*time.Second)
			if err != nil {
				fmt.Println("检查总请求数限制失败:", err.Error())
				abortWithOpenAiMessage(c, http.StatusInternalServerError, i18n.T(c, i18n.MsgRateLimitCheckFailed))
				return
			}
			if !allowed {
				abortModelRequestRateLimited(c, i18n.MsgRateLimitTotalReached, totalMaxCount, duration)
				return
			}
		}

		// 3. 处理请求
		c.Next()

		// 4. 如果请求成功，记录成功请求
		if c.Writer.Status() < 400 {
			recordRedisRequest(ctx, rdb, successKey, successMaxCount, duration)
		}
	}
}

// 内存限流处理器
func memoryRateLimitHandler(duration int64, totalMaxCount, successMaxCount int) gin.HandlerFunc {
	inMemoryRateLimiter.Init(time.Duration(config.ModelRequestRateLimitDurationMinutes) * time.Minute)

	return func(c *gin.Context) {
		userId := strconv.Itoa(c.GetInt("id"))
		totalKey := ModelRequestRateLimitCountMark + userId
		successKey := ModelRequestRateLimitSuccessCountMark + userId

		// 1. 检查成功请求数限制：只判定，请求成功后才记入窗口（当successMaxCount为0时跳过）
		if successMaxCount > 0 && !inMemoryRateLimiter.Peek(successKey, successMaxCount, duration) {
			abortModelRequestRateLimited(c, i18n.MsgRateLimitSuccessReached, successMaxCount, duration)
			return
		}

		// 2. 检查总请求数限制并记录总请求（当totalMaxCount为0时跳过）
		if totalMaxCount > 0 && !inMemoryRateLimiter.Request(totalKey, totalMaxCount, duration) {
			abortModelRequestRateLimited(c, i18n.MsgRateLimitTotalReached, totalMaxCount, duration)
			return
		}

		// 3. 处理请求
		c.Next()

		// 4. 如果请求成功，记录成功请求（当successMaxCount为0时不记录）
		if c.Writer.Status() < 400 && successMaxCount > 0 {
			inMemoryRateLimiter.Record(successKey, successMaxCount)
		}
	}
}

// ModelRequestRateLimit 模型请求限流中间件
func ModelRequestRateLimit() func(c *gin.Context) {
	return func(c *gin.Context) {
		// 在每个请求时检查是否启用限流
		if !config.ModelRequestRateLimitEnabled {
			c.Next()
			return
		}

		// 计算限流参数
		duration := int64(config.ModelRequestRateLimitDurationMinutes * 60)
		totalMaxCount := config.ModelRequestRateLimitCount
		successMaxCount := config.ModelRequestRateLimitSuccessCount

		// 获取分组
		group := httpapi.GetContextKeyString(c, common.ContextKeyTokenGroup)
		if group == "" {
			group = httpapi.GetContextKeyString(c, common.ContextKeyUserGroup)
		}

		//获取分组的限流配置
		groupTotalCount, groupSuccessCount, found := config.GetGroupRateLimit(group)
		if found {
			totalMaxCount = groupTotalCount
			successMaxCount = groupSuccessCount
		}

		// 根据存储类型选择并执行限流处理器
		if infraredis.RedisEnabled {
			redisRateLimitHandler(duration, totalMaxCount, successMaxCount)(c)
		} else {
			memoryRateLimitHandler(duration, totalMaxCount, successMaxCount)(c)
		}
	}
}
