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
)

const (
	ModelRequestRateLimitCountMark        = "MRRL"
	ModelRequestRateLimitSuccessCountMark = "MRRLS"
)

// completeSuccessReservation 了结成功请求数的占位：请求成功时保留记录，否则退回。退回失败只记日志，
// 窗口里会多留一条记录，直到它离开窗口。
func completeSuccessReservation(ctx context.Context, slot *limiter.SlidingWindowReservation, succeeded bool) {
	if err := slot.Complete(ctx, succeeded); err != nil {
		log.LogError(ctx, err.Error())
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

		// 1. 成功请求数：处理前占位，占位与窗口内已成功的记录合计达到上限时拒绝（当successMaxCount为0时跳过）
		var successSlot *limiter.SlidingWindowReservation
		if successMaxCount > 0 {
			successKey := redisUserRateLimitKey(ModelRequestRateLimitSuccessCountMark, c.GetInt("id"))
			slot, err := limiter.SlidingWindowReserve(ctx, rdb, successKey, successMaxCount, time.Duration(duration)*time.Second)
			if err != nil {
				fmt.Println("检查成功请求数限制失败:", err.Error())
				abortWithOpenAiMessage(c, http.StatusInternalServerError, i18n.T(c, i18n.MsgRateLimitCheckFailed))
				return
			}
			if slot == nil {
				abortModelRequestRateLimited(c, i18n.MsgRateLimitSuccessReached, successMaxCount, duration)
				return
			}
			successSlot = slot
		}
		// 请求没有成功完成时退回占位：被请求数限额拒绝、处理失败，或处理过程中 panic
		defer completeSuccessReservation(ctx, successSlot, false)

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

		// 4. 请求成功则保留占位，失败则退回
		completeSuccessReservation(ctx, successSlot, c.Writer.Status() < 400)
	}
}

// 内存限流处理器
func memoryRateLimitHandler(duration int64, totalMaxCount, successMaxCount int) gin.HandlerFunc {
	inMemoryRateLimiter.Init(time.Duration(config.ModelRequestRateLimitDurationMinutes) * time.Minute)

	return func(c *gin.Context) {
		userId := strconv.Itoa(c.GetInt("id"))
		totalKey := ModelRequestRateLimitCountMark + userId
		successKey := ModelRequestRateLimitSuccessCountMark + userId

		// 1. 成功请求数：处理前占位，占位与窗口内已成功的记录合计达到上限时拒绝（当successMaxCount为0时跳过）
		var successSlot *common.RateLimitReservation
		if successMaxCount > 0 {
			if successSlot = inMemoryRateLimiter.Reserve(successKey, successMaxCount, duration); successSlot == nil {
				abortModelRequestRateLimited(c, i18n.MsgRateLimitSuccessReached, successMaxCount, duration)
				return
			}
		}
		// 请求没有成功完成时退回占位：被请求数限额拒绝、处理失败，或处理过程中 panic
		defer successSlot.Complete(false)

		// 2. 检查总请求数限制并记录总请求（当totalMaxCount为0时跳过）
		if totalMaxCount > 0 && !inMemoryRateLimiter.Request(totalKey, totalMaxCount, duration) {
			abortModelRequestRateLimited(c, i18n.MsgRateLimitTotalReached, totalMaxCount, duration)
			return
		}

		// 3. 处理请求
		c.Next()

		// 4. 请求成功则保留占位，失败则退回
		successSlot.Complete(c.Writer.Status() < 400)
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
