package middleware

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/NookMux/NookMux/internal/i18n"
	"github.com/NookMux/NookMux/internal/infra/redis"
	"github.com/alicebob/miniredis/v2"
	"github.com/gin-gonic/gin"
	goredis "github.com/go-redis/redis/v8"
)

// useRateLimitRedis 让限流走 Redis 模式并接入内嵌 Redis。内嵌 Redis 的键生存期不随时钟走，
// 返回的 advance 把它的时钟与键的剩余生存期一并拨快，等价于真实 Redis 里时间流逝。
// 测试结束后恢复原来的开关与客户端。
func useRateLimitRedis(t *testing.T) (server *miniredis.Miniredis, client *goredis.Client, advance func(time.Duration)) {
	t.Helper()

	previousEnabled, previousClient := redis.RedisEnabled, redis.RDB
	server = miniredis.RunT(t)
	now := time.Date(2026, 11, 27, 10, 0, 0, 0, time.UTC)
	server.SetTime(now)
	client = goredis.NewClient(&goredis.Options{Addr: server.Addr()})
	if err := client.Ping(context.Background()).Err(); err != nil {
		t.Fatalf("ping embedded redis: %v", err)
	}
	redis.RedisEnabled, redis.RDB = true, client
	t.Cleanup(func() {
		_ = client.Close()
		redis.RedisEnabled, redis.RDB = previousEnabled, previousClient
	})
	advance = func(d time.Duration) {
		now = now.Add(d)
		server.SetTime(now)
		server.FastForward(d)
	}
	return server, client, advance
}

func TestRedisRateLimiterCountsPerAddressUntilTheWindowEnds(t *testing.T) {
	gin.SetMode(gin.TestMode)
	server, _, advance := useRateLimitRedis(t)

	router := gin.New()
	router.GET("/limited", rateLimitFactory(2, 37, "TEST"), func(c *gin.Context) { c.Status(http.StatusNoContent) })
	const clientIP, otherIP = "192.0.2.30", "198.51.100.30"
	const key, legacyKey = "rateLimit:v2:ip:TEST:192.0.2.30", "rateLimit:TEST192.0.2.30"
	if _, err := server.Push(legacyKey, "legacy"); err != nil {
		t.Fatalf("seed a counter of the older format: %v", err)
	}

	// 额度内放行两次，第三次 429 并给出完整的窗口时长
	for i := 0; i < 2; i++ {
		if resp := serveFromClient(router, clientIP, "/limited"); resp.Code != http.StatusNoContent {
			t.Fatalf("request %d: code = %d, want %d", i+1, resp.Code, http.StatusNoContent)
		}
	}
	if got := server.TTL(key); got != 37*time.Second {
		t.Fatalf("TTL after allowed requests = %v, want 37s", got)
	}
	rejected := serveFromClient(router, clientIP, "/limited")
	if rejected.Code != http.StatusTooManyRequests || rejected.Header().Get("Retry-After") != "37" {
		t.Fatalf("request beyond the quota: code = %d, Retry-After = %q, want 429 and 37", rejected.Code, rejected.Header().Get("Retry-After"))
	}

	// 被拒的请求不延长窗口
	advance(12 * time.Second)
	if resp := serveFromClient(router, clientIP, "/limited"); resp.Code != http.StatusTooManyRequests {
		t.Fatalf("request beyond the quota inside the window: code = %d, want 429", resp.Code)
	}
	if got, want := server.TTL(key), 25*time.Second; got != want {
		t.Fatalf("TTL after a rejected request = %v, want %v", got, want)
	}

	// 窗口过后恢复放行；其他地址不共用额度；旧格式的计数键不被读写
	advance(25 * time.Second)
	for _, ip := range []string{clientIP, otherIP} {
		if resp := serveFromClient(router, ip, "/limited"); resp.Code != http.StatusNoContent {
			t.Fatalf("request from %s after the window: code = %d, want %d", ip, resp.Code, http.StatusNoContent)
		}
	}
	if !server.Exists(legacyKey) {
		t.Fatal("the counter of the older format was removed")
	}
}

// 同一串请求在 Redis 模式与内存模式下得到相同的判定。
func TestRedisRateLimiterDecidesLikeTheInMemoryLimiter(t *testing.T) {
	want := []int{http.StatusNoContent, http.StatusNoContent, http.StatusTooManyRequests, http.StatusNoContent, http.StatusNoContent, http.StatusTooManyRequests}
	for _, mode := range []string{"redis", "memory"} {
		t.Run(mode, func(t *testing.T) {
			gin.SetMode(gin.TestMode)
			var advance func(time.Duration)
			if mode == "redis" {
				_, _, advance = useRateLimitRedis(t)
			} else {
				previousEnabled := redis.RedisEnabled
				redis.RedisEnabled = false
				t.Cleanup(func() { redis.RedisEnabled = previousEnabled })
			}

			router := gin.New()
			router.GET("/limited", rateLimitFactory(2, 1, "PARITY-"+mode), func(c *gin.Context) { c.Status(http.StatusNoContent) })

			codes := []int{}
			for i := 0; i < 3; i++ {
				codes = append(codes, serveFromClient(router, "192.0.2.31", "/limited").Code)
			}
			if advance != nil {
				advance(1100 * time.Millisecond)
			} else {
				time.Sleep(1100 * time.Millisecond)
			}
			for i := 0; i < 3; i++ {
				codes = append(codes, serveFromClient(router, "192.0.2.31", "/limited").Code)
			}
			if fmt.Sprint(codes) != fmt.Sprint(want) {
				t.Fatalf("decisions in %s mode = %v, want %v", mode, codes, want)
			}
		})
	}
}

func TestRedisUserRateLimiterCountsByUserAcrossAddresses(t *testing.T) {
	gin.SetMode(gin.TestMode)
	server, _, advance := useRateLimitRedis(t)

	router := gin.New()
	router.GET("/limited",
		func(c *gin.Context) { c.Set("id", 42) },
		userRateLimitFactory(1, 23, "TESTUSER"),
		func(c *gin.Context) { c.Status(http.StatusNoContent) },
	)
	const key = "rateLimit:v2:user:TESTUSER:42"

	if resp := serveFromClient(router, "192.0.2.40", "/limited"); resp.Code != http.StatusNoContent {
		t.Fatalf("first request: code = %d, want %d", resp.Code, http.StatusNoContent)
	}
	// 换一个来源地址仍是同一个用户：窗口内被拒
	rejected := serveFromClient(router, "198.51.100.40", "/limited")
	if rejected.Code != http.StatusTooManyRequests || rejected.Header().Get("Retry-After") != "23" {
		t.Fatalf("second request from another address: code = %d, Retry-After = %q, want 429 and 23", rejected.Code, rejected.Header().Get("Retry-After"))
	}
	if got, want := server.TTL(key), 23*time.Second; got != want {
		t.Fatalf("TTL = %v, want %v", got, want)
	}

	advance(23 * time.Second)
	if resp := serveFromClient(router, "198.51.100.40", "/limited"); resp.Code != http.StatusNoContent {
		t.Fatalf("request after the window: code = %d, want %d", resp.Code, http.StatusNoContent)
	}
}

// 用掉额度的实例与读取计数的实例时区不同，是共用一个 Redis 的多实例部署里的常态。
func TestRedisRateLimiterIsIndependentOfTheProcessTimeZone(t *testing.T) {
	gin.SetMode(gin.TestMode)
	_, _, advance := useRateLimitRedis(t)
	original := time.Local
	t.Cleanup(func() { time.Local = original })

	router := gin.New()
	router.GET("/limited", rateLimitFactory(1, 60, "ZONE"), func(c *gin.Context) { c.Status(http.StatusNoContent) })
	request := func(clientIP string) int {
		return serveFromClient(router, clientIP, "/limited").Code
	}

	zones := []*time.Location{time.FixedZone("UTC+8", 8*3600), time.UTC, time.FixedZone("UTC-8", -8*3600)}

	// 每个地址由一个时区的实例用掉额度，其余时区的实例读到同一份计数，窗口内都被拒
	for i, writer := range zones {
		clientIP := fmt.Sprintf("192.0.2.%d", 60+i)
		time.Local = writer
		if code := request(clientIP); code != http.StatusNoContent {
			t.Fatalf("first request with local zone %s: code = %d, want %d", writer, code, http.StatusNoContent)
		}
		if code := request(clientIP); code != http.StatusTooManyRequests {
			t.Fatalf("second request with local zone %s: code = %d, want %d", writer, code, http.StatusTooManyRequests)
		}
		for _, reader := range zones {
			if reader == writer {
				continue
			}
			time.Local = reader
			if code := request(clientIP); code != http.StatusTooManyRequests {
				t.Fatalf("request with local zone %s on a window used up under zone %s: code = %d, want %d", reader, writer, code, http.StatusTooManyRequests)
			}
		}
	}

	// 窗口过后换一个时区的实例读取：照常放行一次，额度随即再次用尽
	advance(time.Minute)
	for i := range zones {
		clientIP := fmt.Sprintf("192.0.2.%d", 60+i)
		time.Local = zones[(i+1)%len(zones)]
		if code := request(clientIP); code != http.StatusNoContent {
			t.Fatalf("request after the window with local zone %s: code = %d, want %d", time.Local, code, http.StatusNoContent)
		}
		time.Local = zones[(i+2)%len(zones)]
		if code := request(clientIP); code != http.StatusTooManyRequests {
			t.Fatalf("request after the window was used up again with local zone %s: code = %d, want %d", time.Local, code, http.StatusTooManyRequests)
		}
	}
}

// Redis 不可用时无法判定额度：请求以 500 终止，不按放行处理。
func TestRedisRateLimiterRejectsWithInternalServerErrorWhenRedisIsUnavailable(t *testing.T) {
	gin.SetMode(gin.TestMode)
	_, client, _ := useRateLimitRedis(t)
	if err := client.Close(); err != nil {
		t.Fatalf("close the embedded redis client: %v", err)
	}

	router := gin.New()
	router.GET("/ip", rateLimitFactory(1, 30, "FAIL"), func(c *gin.Context) { c.Status(http.StatusNoContent) })
	router.GET("/user",
		func(c *gin.Context) { c.Set("id", 7) },
		userRateLimitFactory(1, 30, "FAIL"),
		func(c *gin.Context) { c.Status(http.StatusNoContent) },
	)

	for _, target := range []string{"/ip", "/user"} {
		resp := serveFromClient(router, "192.0.2.80", target)
		if resp.Code != http.StatusInternalServerError || resp.Body.Len() != 0 {
			t.Fatalf("GET %s with redis unavailable: code = %d, body = %q, want 500 with an empty body", target, resp.Code, resp.Body.String())
		}
	}
}

func TestModelRequestRateLimitRedisHandlerCountsOnlySuccessfulRequests(t *testing.T) {
	gin.SetMode(gin.TestMode)
	server, _, advance := useRateLimitRedis(t)

	status := http.StatusInternalServerError
	router := gin.New()
	router.GET("/relay",
		func(c *gin.Context) { c.Set("id", 7) },
		redisRateLimitHandler(60, 0, 1),
		func(c *gin.Context) { c.Status(status) },
	)
	const key = "rateLimit:v2:user:MRRLS:7"

	// 失败的请求不计入窗口
	for i := 0; i < 3; i++ {
		if resp := serveFromClient(router, "192.0.2.70", "/relay"); resp.Code != status {
			t.Fatalf("failed request %d: code = %d, want %d", i+1, resp.Code, status)
		}
	}
	if server.Exists(key) {
		t.Fatal("failed requests were counted")
	}

	// 成功的请求计入一次，窗口内再请求被拒，换来源地址也一样
	status = http.StatusOK
	if resp := serveFromClient(router, "192.0.2.70", "/relay"); resp.Code != http.StatusOK {
		t.Fatalf("successful request: code = %d, want %d", resp.Code, http.StatusOK)
	}
	if resp := serveFromClient(router, "198.51.100.70", "/relay"); resp.Code != http.StatusTooManyRequests {
		t.Fatalf("second request inside the window: code = %d, want %d", resp.Code, http.StatusTooManyRequests)
	}
	if got := server.TTL(key); got != time.Minute {
		t.Fatalf("TTL = %v, want %v", got, time.Minute)
	}

	// 窗口过后恢复放行
	advance(time.Minute)
	if resp := serveFromClient(router, "198.51.100.70", "/relay"); resp.Code != http.StatusOK {
		t.Fatalf("request after the window: code = %d, want %d", resp.Code, http.StatusOK)
	}
}

// 请求数由滑动窗口脚本计数：计数键按用户 ID 拼在 rateLimit:v2:user 下，生存期等于限制周期，被拒的请求
// 不续期，与成功请求数各记各的；令牌桶的计数键 rateLimit:<用户ID> 不被读写。
func TestModelRequestRateLimitRedisHandlerCountsRequestsWithTheSlidingWindow(t *testing.T) {
	gin.SetMode(gin.TestMode)
	server, _, advance := useRateLimitRedis(t)

	const totalKey, legacyKey, successKey = "rateLimit:v2:user:MRRL:71", "rateLimit:71", "rateLimit:v2:user:MRRLS:71"
	server.HSet(legacyKey, "tokens", "0", "last_time", "0")

	status := http.StatusInternalServerError
	router := gin.New()
	router.GET("/relay",
		func(c *gin.Context) { c.Set("id", 71) },
		redisRateLimitHandler(37, 2, 1),
		func(c *gin.Context) { c.Status(status) },
	)
	stored := func(key string) int {
		records, err := server.List(key)
		if err != nil {
			t.Fatalf("records under %s: %v", key, err)
		}
		return len(records)
	}

	// 额度内的两个请求都放行并计入请求数；它们都失败了，不占成功请求数
	for i := 0; i < 2; i++ {
		if resp := serveFromClient(router, "192.0.2.71", "/relay"); resp.Code != status {
			t.Fatalf("failed request %d: code = %d, want %d", i+1, resp.Code, status)
		}
	}
	if got, want := stored(totalKey), 2; got != want {
		t.Fatalf("records under %s = %d, want %d", totalKey, got, want)
	}
	if got := server.TTL(totalKey); got != 37*time.Second {
		t.Fatalf("TTL after allowed requests = %v, want 37s", got)
	}
	if server.Exists(successKey) {
		t.Fatal("failed requests were counted as successful requests")
	}

	// 请求数用尽：429 并给出完整的限制周期，被拒的请求不续期、不计数
	rejected := serveFromClient(router, "192.0.2.71", "/relay")
	if rejected.Code != http.StatusTooManyRequests || rejected.Header().Get("Retry-After") != "37" {
		t.Fatalf("request beyond the quota: code = %d, Retry-After = %q, want 429 and 37", rejected.Code, rejected.Header().Get("Retry-After"))
	}
	advance(12 * time.Second)
	if resp := serveFromClient(router, "198.51.100.71", "/relay"); resp.Code != http.StatusTooManyRequests {
		t.Fatalf("request beyond the quota inside the window: code = %d, want %d", resp.Code, http.StatusTooManyRequests)
	}
	if got, want := server.TTL(totalKey), 25*time.Second; got != want {
		t.Fatalf("TTL after a rejected request = %v, want %v", got, want)
	}
	if got, want := stored(totalKey), 2; got != want {
		t.Fatalf("records under %s after a rejected request = %d, want %d", totalKey, got, want)
	}

	// 窗口过后恢复放行：上一周期的计数已到期清除，成功的请求计入成功请求数，请求数从这一次重新记起
	advance(25 * time.Second)
	status = http.StatusOK
	if resp := serveFromClient(router, "198.51.100.71", "/relay"); resp.Code != http.StatusOK {
		t.Fatalf("request after the window: code = %d, want %d", resp.Code, http.StatusOK)
	}
	if got, want := stored(successKey), 1; got != want {
		t.Fatalf("records under %s = %d, want %d", successKey, got, want)
	}
	if got, want := stored(totalKey), 1; got != want {
		t.Fatalf("records under %s after the window = %d, want %d", totalKey, got, want)
	}
	if !server.Exists(legacyKey) {
		t.Fatal("the counter of the older format was removed")
	}
}

// Redis 不可用时无法判定额度：成功请求数与请求数的检查失败都以 500 终止请求，不按放行处理。
func TestModelRequestRateLimitRedisHandlerRejectsWithInternalServerErrorWhenRedisIsUnavailable(t *testing.T) {
	gin.SetMode(gin.TestMode)
	_, client, _ := useRateLimitRedis(t)
	if err := client.Close(); err != nil {
		t.Fatalf("close the embedded redis client: %v", err)
	}

	router := gin.New()
	setUser := func(c *gin.Context) { c.Set("id", 72) }
	admitted := func(c *gin.Context) { c.Status(http.StatusOK) }
	router.GET("/success-limit", setUser, redisRateLimitHandler(60, 0, 1), admitted)
	router.GET("/total-limit", setUser, redisRateLimitHandler(60, 1, 0), admitted)

	want := i18n.Translate(i18n.LangZh, i18n.MsgRateLimitCheckFailed)
	for _, target := range []string{"/success-limit", "/total-limit"} {
		resp := serveFromClient(router, "192.0.2.72", target)
		if resp.Code != http.StatusInternalServerError || !strings.Contains(resp.Body.String(), want) {
			t.Fatalf("GET %s with redis unavailable: code = %d, body = %q, want 500 with %q", target, resp.Code, resp.Body.String(), want)
		}
	}
}

func TestGlobalWebRateLimitSeparatesPagesUnmatchedPathsAndStaticFilesInRedisMode(t *testing.T) {
	gin.SetMode(gin.TestMode)
	server, client, advance := useRateLimitRedis(t)
	setWebRateLimitQuota(t, true, 2, false, 1000, 41)

	router := gin.New()
	router.NoRoute(GlobalWebRateLimit(newFrontendFileServer(t)), func(c *gin.Context) {
		c.Status(http.StatusNoContent)
	})
	const clientIP = "192.0.2.16"
	const (
		pageKey      = "rateLimit:v2:ip:GW:192.0.2.16"
		unmatchedKey = "rateLimit:v2:ip:GN:192.0.2.16"
		staticKey    = "rateLimit:v2:ip:GS:192.0.2.16"
	)
	request := func(target string) *httptest.ResponseRecorder {
		return serveFromClient(router, clientIP, target)
	}
	stored := func(key string) int64 {
		length, err := client.LLen(context.Background(), key).Result()
		if err != nil {
			t.Fatalf("LLEN %s: %v", key, err)
		}
		return length
	}

	// 构建产物默认不限流
	for i := 0; i < 5; i++ {
		if resp := request("/static/js/app.js"); resp.Code != http.StatusNoContent {
			t.Fatalf("frontend file request %d: code = %d, want %d", i+1, resp.Code, http.StatusNoContent)
		}
	}

	// 未匹配路径共用一份额度，超出后 429
	for _, target := range []string{"/v1/unknown", "/api/unknown"} {
		if resp := request(target); resp.Code != http.StatusNoContent {
			t.Fatalf("unmatched request %s: code = %d, want %d", target, resp.Code, http.StatusNoContent)
		}
	}
	rejected := request("/static/js/missing.js")
	if rejected.Code != http.StatusTooManyRequests || rejected.Header().Get("Retry-After") != "41" {
		t.Fatalf("unmatched request beyond the quota: code = %d, Retry-After = %q, want 429 and 41", rejected.Code, rejected.Header().Get("Retry-After"))
	}

	// 页面按自己的额度放行两次，第三次 429
	for i := 0; i < 2; i++ {
		if resp := request("/console/overview"); resp.Code != http.StatusNoContent {
			t.Fatalf("page request %d: code = %d, want %d", i+1, resp.Code, http.StatusNoContent)
		}
	}
	if resp := request("/console/overview"); resp.Code != http.StatusTooManyRequests {
		t.Fatalf("page request 3: code = %d, want %d", resp.Code, http.StatusTooManyRequests)
	}

	// 页面与未匹配路径各自计数，被拒的请求不计数，不限流的构建产物不留计数
	if got, want := stored(pageKey), int64(2); got != want {
		t.Fatalf("records under %s = %d, want %d", pageKey, got, want)
	}
	if got, want := stored(unmatchedKey), int64(2); got != want {
		t.Fatalf("records under %s = %d, want %d", unmatchedKey, got, want)
	}
	if server.Exists(staticKey) {
		t.Fatalf("frontend file requests were counted under %s although static rate limit is disabled", staticKey)
	}

	// 窗口过后两类请求都恢复放行
	advance(41 * time.Second)
	for _, target := range []string{"/console/overview", "/v1/unknown"} {
		if resp := request(target); resp.Code != http.StatusNoContent {
			t.Fatalf("%s after the window: code = %d, want %d", target, resp.Code, http.StatusNoContent)
		}
	}
}

// 窗口不是正数表示不限流，两种存储模式的判定相同。
func TestRateLimitersDoNotLimitWhenTheWindowIsNotPositive(t *testing.T) {
	gin.SetMode(gin.TestMode)
	const clientIP = "192.0.2.90"

	for _, mode := range []string{"redis", "memory"} {
		t.Run(mode, func(t *testing.T) {
			var server *miniredis.Miniredis
			if mode == "redis" {
				server, _, _ = useRateLimitRedis(t)
			} else {
				previousEnabled := redis.RedisEnabled
				redis.RedisEnabled = false
				t.Cleanup(func() { redis.RedisEnabled = previousEnabled })
			}

			router := gin.New()
			router.GET("/limited", rateLimitFactory(2, 0, "ZERO-"+mode), func(c *gin.Context) { c.Status(http.StatusNoContent) })

			for i := 0; i < 6; i++ {
				if resp := serveFromClient(router, clientIP, "/limited"); resp.Code != http.StatusNoContent {
					t.Fatalf("request %d in %s mode: code = %d, want %d", i+1, mode, resp.Code, http.StatusNoContent)
				}
			}
			if server != nil && server.Exists("rateLimit:v2:ip:ZERO-redis:"+clientIP) {
				t.Fatal("a non-positive window left a counter in redis")
			}
		})
	}
}

// 限制周期填 0 时两项限额都不生效：Redis 模式下每个请求照常通过，不留计数。
func TestModelRequestRateLimitRedisHandlerDoesNotLimitWithZeroWindow(t *testing.T) {
	gin.SetMode(gin.TestMode)
	server, _, _ := useRateLimitRedis(t)

	router := gin.New()
	router.GET("/relay",
		func(c *gin.Context) { c.Set("id", 9) },
		redisRateLimitHandler(0, 1, 1),
		func(c *gin.Context) { c.Status(http.StatusOK) },
	)

	for i := 0; i < 4; i++ {
		if resp := serveFromClient(router, "192.0.2.91", "/relay"); resp.Code != http.StatusOK {
			t.Fatalf("request %d: code = %d, want %d", i+1, resp.Code, http.StatusOK)
		}
	}
	for _, key := range []string{"rateLimit:v2:user:MRRL:9", "rateLimit:v2:user:MRRLS:9"} {
		if server.Exists(key) {
			t.Fatalf("a zero window left a counter under %s", key)
		}
	}
}

// 成功请求数在处理前占位：请求处理期间名额已被占住，此时到来的请求被拒；处理失败后退回名额，计数键
// 随之消失，处理成功后记录保留，生存期等于限制周期。
func TestModelRequestRateLimitRedisHandlerHoldsTheSuccessSlotWhileTheRequestIsInFlight(t *testing.T) {
	gin.SetMode(gin.TestMode)
	_, client, _ := useRateLimitRedis(t)

	status := http.StatusOK
	block := true
	started := make(chan struct{}, 4)
	finish := make(chan struct{})
	router := gin.New()
	router.GET("/relay",
		func(c *gin.Context) { c.Set("id", 8) },
		redisRateLimitHandler(60, 0, 1),
		func(c *gin.Context) {
			if block {
				started <- struct{}{}
				<-finish
			}
			c.Status(status)
		},
	)
	const key = "rateLimit:v2:user:MRRLS:8"
	send := func() int { return serveFromClient(router, "192.0.2.8", "/relay").Code }
	stored := func() int64 {
		length, err := client.LLen(context.Background(), key).Result()
		if err != nil {
			t.Fatalf("LLEN %s: %v", key, err)
		}
		return length
	}

	// 请求处理期间：窗口里有它的记录，此时到来的请求被拒
	codes := make(chan int, 4)
	go func() { codes <- send() }()
	select {
	case <-started:
	case <-time.After(5 * time.Second):
		t.Fatal("the request did not reach the handler")
	}
	if got := stored(); got != 1 {
		t.Fatalf("records while the request is in flight = %d, want 1", got)
	}
	if code := send(); code != http.StatusTooManyRequests {
		t.Fatalf("a request sent while one was in flight got %d, want %d", code, http.StatusTooManyRequests)
	}

	// 请求失败：退回名额，计数键随之消失，之后的请求恢复放行
	status = http.StatusInternalServerError
	close(finish)
	if code := <-codes; code != status {
		t.Fatalf("the in-flight request finished with %d, want %d", code, status)
	}
	if client.Exists(context.Background(), key).Val() != 0 {
		t.Fatal("a failed request kept its record")
	}
	block = false
	if code := send(); code != status {
		t.Fatalf("a request after a failed one got %d, want %d", code, status)
	}
	if client.Exists(context.Background(), key).Val() != 0 {
		t.Fatal("a failed request kept its record")
	}

	// 请求成功：记录保留，生存期等于限制周期，窗口内再请求被拒
	status = http.StatusOK
	if code := send(); code != status {
		t.Fatalf("a request after failed ones got %d, want %d", code, status)
	}
	if got := client.TTL(context.Background(), key).Val(); got != time.Minute {
		t.Fatalf("TTL after a successful request = %v, want %v", got, time.Minute)
	}
	if code := send(); code != http.StatusTooManyRequests {
		t.Fatalf("a request inside the window after a successful one got %d, want %d", code, http.StatusTooManyRequests)
	}
}
