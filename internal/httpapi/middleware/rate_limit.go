package middleware

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/NookMux/NookMux/internal/common"
	"github.com/NookMux/NookMux/internal/infra/cache/limiter"
	"github.com/NookMux/NookMux/internal/infra/log"
	"github.com/NookMux/NookMux/internal/infra/redis"
	"github.com/gin-contrib/static"
	"github.com/gin-gonic/gin"
)

// redisRateLimitNamespace 是 Redis 模式下限流计数的键前缀。计数是毫秒时间戳列表，与按本地时间
// 字符串记录的旧计数键不同名，共用同一个 Redis 的新旧实例互不读写对方的计数。
const redisRateLimitNamespace = "rateLimit:v2"

var inMemoryRateLimiter common.InMemoryRateLimiter

var defNext = func(c *gin.Context) {
	c.Next()
}

func redisIPRateLimitKey(mark string, clientIP string) string {
	return fmt.Sprintf("%s:ip:%s:%s", redisRateLimitNamespace, mark, clientIP)
}

func redisUserRateLimitKey(mark string, userID int) string {
	return fmt.Sprintf("%s:user:%s:%d", redisRateLimitNamespace, mark, userID)
}

// redisRateLimiter 按客户端 IP 计数。判定与记录由一次 Redis 脚本调用完成，窗口按 Redis 服务端
// 时钟计算，各实例的时区与时钟不参与；被拒的请求不写入记录，也不延长计数的生存期。
func redisRateLimiter(c *gin.Context, maxRequestNum int, duration int64, mark string) {
	allowed, err := limiter.SlidingWindowTake(
		c.Request.Context(),
		redis.RDB,
		redisIPRateLimitKey(mark, c.ClientIP()),
		maxRequestNum,
		time.Duration(duration)*time.Second,
	)
	if err != nil {
		log.LogError(c.Request.Context(), fmt.Sprintf("rate limit check failed (mark=%s): %v", mark, err))
		c.Status(http.StatusInternalServerError)
		c.Abort()
		return
	}
	if !allowed {
		writeRateLimited(c, duration)
	}
}

func memoryRateLimiter(c *gin.Context, maxRequestNum int, duration int64, mark string) {
	key := mark + c.ClientIP()
	if !inMemoryRateLimiter.Request(key, maxRequestNum, duration) {
		writeRateLimited(c, duration)
		return
	}
}

// setRetryAfter 通过 Retry-After 告知客户端可重试的等待秒数。限流窗口按窗口内最旧一次请求计时且
// 限流器不回报剩余时间，两种存储模式统一给出完整窗口时长作为保守上界：等待该时长后窗口必然已清空。
func setRetryAfter(c *gin.Context, retryAfterSeconds int64) {
	if retryAfterSeconds > 0 {
		c.Header("Retry-After", strconv.FormatInt(retryAfterSeconds, 10))
	}
}

// writeRateLimited 以 429 终止请求，响应体为空，等待时长由 setRetryAfter 给出。
func writeRateLimited(c *gin.Context, retryAfterSeconds int64) {
	setRetryAfter(c, retryAfterSeconds)
	c.Status(http.StatusTooManyRequests)
	c.Abort()
}

func rateLimitFactory(maxRequestNum int, duration int64, mark string) func(c *gin.Context) {
	if redis.RedisEnabled {
		return func(c *gin.Context) {
			redisRateLimiter(c, maxRequestNum, duration, mark)
		}
	}
	// It's safe to call multi times.
	inMemoryRateLimiter.Init(common.RateLimitKeyExpirationDuration)
	return func(c *gin.Context) {
		memoryRateLimiter(c, maxRequestNum, duration, mark)
	}
}

// nonPagePrefixes 下的路径绝不回退为前端页面：接口与中继路径未匹配、构建产物在当前版本中
// 不存在时，SPA 回退链返回 JSON 404。
var nonPagePrefixes = []string{"/v1", "/api", "/assets", "/static"}

// IsNonPageRequest 判断请求的路径是否落在 nonPagePrefixes 下。SPA 回退链据此决定返回 JSON 404，
// Web 限流据此把这类请求与页面请求分开计数。
func IsNonPageRequest(requestURI string) bool {
	for _, prefix := range nonPagePrefixes {
		if strings.HasPrefix(requestURI, prefix) {
			return true
		}
	}
	return false
}

// GlobalWebRateLimit 按客户端 IP 限制前端页面与未匹配路径的请求，三类请求各自计数、互不占用。
// 命中前端构建产物中实际文件（入口脚本、懒加载分块等）的请求走静态限流器，默认关闭，即构建产物
// 默认不限流；不回退为页面的路径（接口与中继路径未匹配、构建产物已不存在）走未匹配路径限流器，
// 与页面限流器共用 GLOBAL_WEB_RATE_LIMIT 的次数与窗口；其余请求走页面限流器。
func GlobalWebRateLimit(frontendFS static.ServeFileSystem) func(c *gin.Context) {
	pageLimiter, unmatchedLimiter, staticLimiter := defNext, defNext, defNext
	if common.GlobalWebRateLimitEnable {
		pageLimiter = rateLimitFactory(common.GlobalWebRateLimitNum, common.GlobalWebRateLimitDuration, "GW")
		unmatchedLimiter = rateLimitFactory(common.GlobalWebRateLimitNum, common.GlobalWebRateLimitDuration, "GN")
	}
	if common.GlobalStaticRateLimitEnable {
		staticLimiter = rateLimitFactory(common.GlobalStaticRateLimitNum, common.GlobalStaticRateLimitDuration, "GS")
	}
	return func(c *gin.Context) {
		if frontendFS.Exists("/", c.Request.URL.Path) {
			staticLimiter(c)
			return
		}
		if IsNonPageRequest(c.Request.RequestURI) {
			unmatchedLimiter(c)
			return
		}
		pageLimiter(c)
	}
}

func GlobalAPIRateLimit() func(c *gin.Context) {
	if common.GlobalApiRateLimitEnable {
		return rateLimitFactory(common.GlobalApiRateLimitNum, common.GlobalApiRateLimitDuration, "GA")
	}
	return defNext
}

func CriticalRateLimit() func(c *gin.Context) {
	if common.CriticalRateLimitEnable {
		return rateLimitFactory(common.CriticalRateLimitNum, common.CriticalRateLimitDuration, "CT")
	}
	return defNext
}

// UserCriticalRateLimit 用户级关键限流：按已认证用户 ID 计数，防代理
// 轮换绕过 IP 级限流；必须挂在 UserAuth 之后。
func UserCriticalRateLimit(scope string) func(c *gin.Context) {
	if common.CriticalRateLimitEnable {
		return userRateLimitFactory(common.CriticalRateLimitNum, common.CriticalRateLimitDuration, "UC:"+scope)
	}
	return defNext
}

func DownloadRateLimit() func(c *gin.Context) {
	return rateLimitFactory(common.DownloadRateLimitNum, common.DownloadRateLimitDuration, "DW")
}

func UploadRateLimit() func(c *gin.Context) {
	return rateLimitFactory(common.UploadRateLimitNum, common.UploadRateLimitDuration, "UP")
}

// userRateLimitFactory creates a rate limiter keyed by authenticated user ID
// instead of client IP, making it resistant to proxy rotation attacks.
// Must be used AFTER authentication middleware (UserAuth).
func userRateLimitFactory(maxRequestNum int, duration int64, mark string) func(c *gin.Context) {
	if redis.RedisEnabled {
		return func(c *gin.Context) {
			userId := c.GetInt("id")
			if userId == 0 {
				c.Status(http.StatusUnauthorized)
				c.Abort()
				return
			}
			redisUserRateLimiter(c, maxRequestNum, duration, redisUserRateLimitKey(mark, userId))
		}
	}
	// It's safe to call multi times.
	inMemoryRateLimiter.Init(common.RateLimitKeyExpirationDuration)
	return func(c *gin.Context) {
		userId := c.GetInt("id")
		if userId == 0 {
			c.Status(http.StatusUnauthorized)
			c.Abort()
			return
		}
		key := fmt.Sprintf("%s:user:%d", mark, userId)
		if !inMemoryRateLimiter.Request(key, maxRequestNum, duration) {
			writeRateLimited(c, duration)
			return
		}
	}
}

// redisUserRateLimiter 按预先拼好的键计数，供按用户 ID 计数的限流器使用。
func redisUserRateLimiter(c *gin.Context, maxRequestNum int, duration int64, key string) {
	allowed, err := limiter.SlidingWindowTake(
		c.Request.Context(),
		redis.RDB,
		key,
		maxRequestNum,
		time.Duration(duration)*time.Second,
	)
	if err != nil {
		log.LogError(c.Request.Context(), fmt.Sprintf("rate limit check failed (key=%s): %v", key, err))
		c.Status(http.StatusInternalServerError)
		c.Abort()
		return
	}
	if !allowed {
		writeRateLimited(c, duration)
	}
}

// SearchRateLimit returns a per-user rate limiter for search endpoints.
// 10 requests per 60 seconds per user (by user ID, not IP).
func SearchRateLimit() func(c *gin.Context) {
	if common.SearchRateLimitEnable {
		return userRateLimitFactory(common.SearchRateLimitNum, common.SearchRateLimitDuration, "SR")
	}
	return defNext
}

// ApiKeyQueryRateLimit 按客户端 IP 限制凭 API Key 匿名查询用量与日志的接口。这类查询与
// 登录、注册等凭证操作目的不同，独立计数，避免查询密钥用完额度后连登录也被拒。
func ApiKeyQueryRateLimit() func(c *gin.Context) {
	if common.ApiKeyQueryRateLimitEnable {
		return rateLimitFactory(common.ApiKeyQueryRateLimitNum, common.ApiKeyQueryRateLimitDuration, "KQ")
	}
	return defNext
}
