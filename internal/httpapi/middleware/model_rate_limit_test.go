package middleware

import (
	"encoding/json"
	"net/http"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/NookMux/NookMux/internal/common"
	"github.com/NookMux/NookMux/internal/config"
	"github.com/NookMux/NookMux/internal/domain/shared"
	"github.com/NookMux/NookMux/internal/i18n"
	"github.com/NookMux/NookMux/internal/infra/redis"
	"github.com/gin-gonic/gin"
)

// modelRateLimitModes 是模型请求限流的两种存储模式。
var modelRateLimitModes = []string{"redis", "memory"}

// nextModelRateLimitUserID 为每个用例分配互不重复的用户 ID：内存模式的用例共用进程内限流器，
// 其他用例留下的额度不影响本用例。
var nextModelRateLimitUserID atomic.Int64

// modelRateLimitUserID 返回限流用例使用的用户 ID，从 7000 起，不与其他限流用例使用的 ID 相重。
func modelRateLimitUserID() int {
	return 7000 + int(nextModelRateLimitUserID.Add(1))
}

// modelRateLimitStorage 让模型请求限流走 mode 对应的存储模式，返回限流处理器与拨快时间的函数：
// Redis 模式拨快内嵌 Redis 的时钟，内存模式按进程时钟计时，只能真实等待。
func modelRateLimitStorage(t *testing.T, mode string) (handler func(int64, int, int) gin.HandlerFunc, advance func(time.Duration)) {
	t.Helper()

	if mode == "redis" {
		_, _, advance := useRateLimitRedis(t)
		return redisRateLimitHandler, advance
	}
	previousEnabled := redis.RedisEnabled
	redis.RedisEnabled = false
	t.Cleanup(func() { redis.RedisEnabled = previousEnabled })
	return memoryRateLimitHandler, time.Sleep
}

// 同一串请求在两种存储模式下得到相同的判定：只有成功的请求计入成功请求数，失败的请求不占成功额度，
// 被拒的请求不延长窗口，窗口按最旧一条记录计时。
func TestModelRequestRateLimitDecidesAlikeInBothStorageModes(t *testing.T) {
	gin.SetMode(gin.TestMode)
	// 窗口 3 秒。内存限流器按整秒计时，窗口内与窗口过后的请求都和窗口边界留出一秒以上。
	const window = 3
	steps := []struct {
		name    string
		wait    time.Duration // 发起这一步请求前流逝的时间
		status  int           // 处理函数返回的状态
		limited bool          // 这一步被限流器拒绝，请求不到达处理函数
	}{
		{name: "失败的请求", status: http.StatusInternalServerError},
		{name: "再次失败的请求", status: http.StatusServiceUnavailable},
		{name: "成功的请求", status: http.StatusOK},
		{name: "成功额度用尽后", status: http.StatusOK, limited: true},
		{name: "窗口内再请求", wait: time.Second, status: http.StatusOK, limited: true},
		{name: "窗口过后", wait: 2100 * time.Millisecond, status: http.StatusOK},
		{name: "额度再次用尽", status: http.StatusOK, limited: true},
	}

	for _, mode := range modelRateLimitModes {
		t.Run(mode, func(t *testing.T) {
			handler, advance := modelRateLimitStorage(t, mode)
			userID := modelRateLimitUserID()
			status, reached := http.StatusOK, false
			router := gin.New()
			router.GET("/relay",
				func(c *gin.Context) { c.Set("id", userID) },
				handler(window, 0, 1),
				func(c *gin.Context) {
					reached = true
					c.Status(status)
				},
			)

			for _, step := range steps {
				advance(step.wait)
				status, reached = step.status, false
				resp := serveFromClient(router, "192.0.2.70", "/relay")
				want := step.status
				if step.limited {
					want = http.StatusTooManyRequests
				}
				if resp.Code != want || reached != !step.limited {
					t.Fatalf("%s: code = %d, reached the handler = %v, want code %d and reached = %v", step.name, resp.Code, reached, want, !step.limited)
				}
			}
		})
	}
}

// 两种存储模式对同一串接连发生的请求给出相同的判定：失败的请求不占成功额度，成功请求数为 0 时不限制，
// 限制周期为 0 时不限流，请求数计入每一个放行的请求（含失败的），成功请求数的判定先于请求数。
func TestModelRequestRateLimitAppliesTheConfiguredLimitsAlike(t *testing.T) {
	gin.SetMode(gin.TestMode)
	const ok, failed = http.StatusOK, http.StatusInternalServerError
	cases := []struct {
		name           string
		window         int64 // 限制周期的秒数
		total, success int
		results        []int // 处理函数依次返回的状态
		want           []int // 依次得到的状态码
	}{
		{
			name:    "失败的请求不占成功额度",
			window:  60,
			success: 2,
			results: []int{failed, ok, failed, ok, ok, ok},
			want:    []int{failed, ok, failed, ok, http.StatusTooManyRequests, http.StatusTooManyRequests},
		},
		{
			name:    "成功请求数为 0 时不限制",
			window:  60,
			results: []int{ok, failed, ok, ok, ok},
			want:    []int{ok, failed, ok, ok, ok},
		},
		{
			name:    "限制周期为 0 时不限流",
			window:  0,
			total:   1,
			success: 1,
			results: []int{ok, failed, ok, ok, ok},
			want:    []int{ok, failed, ok, ok, ok},
		},
		{
			name:    "请求数计入每一个放行的请求",
			window:  60,
			total:   2,
			results: []int{ok, failed, ok},
			want:    []int{ok, failed, http.StatusTooManyRequests},
		},
		{
			name:    "请求数用尽后成功的请求也被拒",
			window:  60,
			total:   1,
			success: 3,
			results: []int{failed, ok},
			want:    []int{failed, http.StatusTooManyRequests},
		},
		{
			name:    "成功请求数先于请求数判定",
			window:  60,
			total:   3,
			success: 1,
			results: []int{ok, ok, failed, ok},
			want:    []int{ok, http.StatusTooManyRequests, http.StatusTooManyRequests, http.StatusTooManyRequests},
		},
	}

	for _, mode := range modelRateLimitModes {
		t.Run(mode, func(t *testing.T) {
			handler, _ := modelRateLimitStorage(t, mode)
			for _, tc := range cases {
				t.Run(tc.name, func(t *testing.T) {
					userID := modelRateLimitUserID()
					status := 0
					router := gin.New()
					router.GET("/relay",
						func(c *gin.Context) { c.Set("id", userID) },
						handler(tc.window, tc.total, tc.success),
						func(c *gin.Context) { c.Status(status) },
					)

					for i, result := range tc.results {
						status = result
						if resp := serveFromClient(router, "192.0.2.70", "/relay"); resp.Code != tc.want[i] {
							t.Fatalf("request %d: code = %d, want %d", i+1, resp.Code, tc.want[i])
						}
					}
				})
			}
		})
	}
}

// 请求数按滑动窗口计数，两种存储模式对同一串请求得到相同的判定：限制周期内放行的请求数达到上限后拒绝，
// 被拒的请求不计数、不延长窗口，窗口按最早一次放行的请求计时。被拒的请求返回 429，Retry-After 给出
// 完整的限制周期。
func TestModelRequestRateLimitCountsRequestsInASlidingWindowAlike(t *testing.T) {
	gin.SetMode(gin.TestMode)
	// 窗口 3 秒。内存限流器按整秒计时，窗口内与窗口过后的请求都和窗口边界留出一秒以上。
	const window = 3
	steps := []struct {
		name    string
		wait    time.Duration // 发起这一步请求前流逝的时间
		allowed bool
	}{
		{name: "额度内的第一次请求", allowed: true},
		{name: "额度内的第二次请求", allowed: true},
		{name: "请求数用尽后"},
		{name: "窗口内再请求", wait: time.Second},
		{name: "最早两次放行离开窗口后", wait: 2100 * time.Millisecond, allowed: true},
		{name: "窗口内放行的第二次", allowed: true},
		{name: "请求数再次用尽"},
	}

	for _, mode := range modelRateLimitModes {
		t.Run(mode, func(t *testing.T) {
			handler, advance := modelRateLimitStorage(t, mode)
			userID := modelRateLimitUserID()
			reached := false
			router := gin.New()
			router.GET("/relay",
				func(c *gin.Context) { c.Set("id", userID) },
				handler(window, 2, 0),
				func(c *gin.Context) {
					reached = true
					c.Status(http.StatusOK)
				},
			)

			for _, step := range steps {
				advance(step.wait)
				reached = false
				resp := serveFromClient(router, "192.0.2.70", "/relay")
				want := http.StatusTooManyRequests
				if step.allowed {
					want = http.StatusOK
				}
				if resp.Code != want || reached != step.allowed {
					t.Fatalf("%s: code = %d, reached the handler = %v, want code %d and reached = %v", step.name, resp.Code, reached, want, step.allowed)
				}
				if retryAfter := resp.Header().Get("Retry-After"); !step.allowed && retryAfter != "3" {
					t.Fatalf("%s: Retry-After = %q, want 3", step.name, retryAfter)
				}
			}
		})
	}
}

// 被任一项限额拒绝时两种存储模式的响应相同：状态码 429，Retry-After 给出完整的限制周期，响应体是
// 说明命中哪一项限额的 OpenAI 风格错误。
func TestModelRequestRateLimitRespondsAlikeWhenLimited(t *testing.T) {
	gin.SetMode(gin.TestMode)
	const window = 60
	cases := []struct {
		name       string
		total      int
		success    int
		max        int // 被用尽的限额
		status     int // 处理函数返回的状态
		messageKey string
	}{
		{name: "成功请求数用尽", success: 1, max: 1, status: http.StatusOK, messageKey: i18n.MsgRateLimitSuccessReached},
		{name: "请求数用尽", total: 1, max: 1, status: http.StatusInternalServerError, messageKey: i18n.MsgRateLimitTotalReached},
	}

	type response struct {
		code        int
		retryAfter  string
		contentType string
		body        string
	}
	type limitError struct {
		Error struct {
			Message string `json:"message"`
			Type    string `json:"type"`
		} `json:"error"`
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// 每个存储模式下依次发出三个请求：第一个放行并用掉额度，后两个被拒
			responses := map[string][]response{}
			for _, mode := range modelRateLimitModes {
				t.Run(mode, func(t *testing.T) {
					handler, _ := modelRateLimitStorage(t, mode)
					userID := modelRateLimitUserID()
					router := gin.New()
					router.GET("/relay",
						func(c *gin.Context) { c.Set("id", userID) },
						handler(window, tc.total, tc.success),
						func(c *gin.Context) { c.Status(tc.status) },
					)

					for i := 0; i < 3; i++ {
						resp := serveFromClient(router, "192.0.2.70", "/relay")
						responses[mode] = append(responses[mode], response{
							code:        resp.Code,
							retryAfter:  resp.Header().Get("Retry-After"),
							contentType: resp.Header().Get("Content-Type"),
							body:        resp.Body.String(),
						})
					}
				})
			}
			if !reflect.DeepEqual(responses["redis"], responses["memory"]) {
				t.Fatalf("responses differ between the storage modes:\nredis:  %+v\nmemory: %+v", responses["redis"], responses["memory"])
			}

			wantMessage := common.MessageWithRequestId(i18n.Translate(i18n.LangZh, tc.messageKey, map[string]any{
				"Minutes": config.ModelRequestRateLimitDurationMinutes,
				"Max":     tc.max,
			}), "")
			for i, got := range responses["memory"] {
				if i == 0 {
					if got.code != tc.status || got.retryAfter != "" {
						t.Fatalf("admitted request: code = %d, Retry-After = %q, want %d without Retry-After", got.code, got.retryAfter, tc.status)
					}
					continue
				}
				var body limitError
				if err := json.Unmarshal([]byte(got.body), &body); err != nil {
					t.Fatalf("request %d: body %q is not JSON: %v", i+1, got.body, err)
				}
				if got.code != http.StatusTooManyRequests || got.retryAfter != "60" || !strings.HasPrefix(got.contentType, "application/json") {
					t.Fatalf("request %d: code = %d, Retry-After = %q, Content-Type = %q, want 429, 60 and JSON", i+1, got.code, got.retryAfter, got.contentType)
				}
				if body.Error.Message != wantMessage || body.Error.Type != string(shared.ErrorTypeNookMuxError) {
					t.Fatalf("request %d: error = %+v, want message %q of type %q", i+1, body.Error, wantMessage, shared.ErrorTypeNookMuxError)
				}
			}
		})
	}
}
