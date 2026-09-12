package openai

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"

	relaycommon "github.com/NookMux/NookMux/internal/relay/common"
	relayconstant "github.com/NookMux/NookMux/internal/relay/constant"

	"github.com/NookMux/NookMux/internal/common"
	"github.com/NookMux/NookMux/internal/domain/billing"
	"github.com/NookMux/NookMux/internal/domain/shared"
	redis "github.com/NookMux/NookMux/internal/infra/redis"
	"github.com/NookMux/NookMux/internal/httpapi"
	channelstore "github.com/NookMux/NookMux/internal/store/channel"
	dbstore "github.com/NookMux/NookMux/internal/store/db"
	pricingstore "github.com/NookMux/NookMux/internal/store/pricing"
	tokenstore "github.com/NookMux/NookMux/internal/store/token"
	userstore "github.com/NookMux/NookMux/internal/store/user"
	"github.com/NookMux/NookMux/pkg/jsonx"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

// TestOpenaiSTTHandlerTagsUsageSourceAndSkipsEstimate 验证 STT 解析点：
// 上游返回真实 usage 时显式标识 OpenAI Chat 来源并可归一化落列；
// 上游无 usage 的本地估算分支打 local 标志、billing_details 不落列。
func TestOpenaiSTTHandlerTagsUsageSourceAndSkipsEstimate(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("real upstream usage", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(http.MethodPost, "/v1/audio/transcriptions", nil)
		info := &relaycommon.RelayInfo{
			ChannelMeta: &relaycommon.ChannelMeta{UpstreamModelName: "gpt-4o-transcribe"},
		}
		body := `{"text":"hello","usage":{"type":"tokens","input_tokens":120,"input_token_details":{"cached_tokens":40,"text_tokens":60,"audio_tokens":20},"output_tokens":30,"total_tokens":150}}`
		resp := &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(body))}

		apiErr, usage := OpenaiSTTHandler(c, resp, info, "json")
		if apiErr != nil {
			t.Fatalf("OpenaiSTTHandler error: %v", apiErr)
		}
		if usage.PromptTokens != 120 || usage.CompletionTokens != 30 {
			t.Fatalf("usage = %+v, want real upstream tokens", usage)
		}
		if info.UsageSource != relayconstant.UsageSourceOpenAIChat {
			t.Fatalf("usage source = %q, want %q", info.UsageSource, relayconstant.UsageSourceOpenAIChat)
		}
		// STT 上游 usage 用单数 input_token_details 承载输入明细，解析点补入
		// InputTokensDetails（只写该字段，不写 PromptTokensDetails，避免改变
		// audio_handler 的计费分流），billing_details 记录官方缓存/文本/音频拆分。
		if usage.InputTokensDetails == nil || usage.InputTokensDetails.CachedTokens != 40 ||
			usage.InputTokensDetails.TextTokens != 60 || usage.InputTokensDetails.AudioTokens != 20 {
			t.Fatalf("input_token_details must be mapped into InputTokensDetails, got %+v", usage.InputTokensDetails)
		}
		if usage.PromptTokensDetails.AudioTokens != 0 || usage.PromptTokensDetails.CachedTokens != 0 {
			t.Fatalf("PromptTokensDetails must stay empty to keep existing audio billing path, got %+v", usage.PromptTokensDetails)
		}
		got := billing.BuildBillingDetailsForLog(c, info, usage)
		want := `{"schema_version":1,"tokens":{"input":{"text_input":60,"image_input":0,"audio_input":20,"video_input":0,"document_input":0},"output":{"text_output":30,"audio_output":0,"image_output":0,"reasoning_output":0,"accepted_prediction":0,"rejected_prediction":0},"cache":{"read_cache":40,"write_cache":0,"write_cache_5m":0,"write_cache_1h":0}}}`
		if got != want {
			t.Fatalf("billing_details = %s, want %s", got, want)
		}
	})

	t.Run("estimate fallback flagged local", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(http.MethodPost, "/v1/audio/transcriptions", nil)
		info := &relaycommon.RelayInfo{
			ChannelMeta: &relaycommon.ChannelMeta{UpstreamModelName: "whisper-1"},
		}
		info.SetEstimatePromptTokens(25)
		resp := &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(`{"text":"hi"}`))}

		apiErr, usage := OpenaiSTTHandler(c, resp, info, "json")
		if apiErr != nil {
			t.Fatalf("OpenaiSTTHandler error: %v", apiErr)
		}
		if usage.PromptTokens != 25 {
			t.Fatalf("usage = %+v, want estimate prompt tokens", usage)
		}
		if !httpapi.GetContextKeyBool(c, common.ContextKeyLocalCountTokens) {
			t.Fatalf("estimate fallback must set ContextKeyLocalCountTokens")
		}
	})
}

// TestOpenaiTTSHandlerBillingDetailsLocalCounting 验证 TTS 流式解析点：
// 上游出现 usage 事件时按官方用量落列；整条流没有 usage 事件时初始估算值
// 继续用于计费（现有行为），但必须打 local 标志、billing_details 不落列。
func TestOpenaiTTSHandlerBillingDetailsLocalCounting(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("stream without upstream usage flags local", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(http.MethodPost, "/v1/audio/speech", nil)
		info := &relaycommon.RelayInfo{
			ChannelMeta: &relaycommon.ChannelMeta{UpstreamModelName: "gpt-4o-mini-tts"},
			IsStream:    true,
		}
		info.SetEstimatePromptTokens(30)
		resp := &http.Response{
			StatusCode: http.StatusOK,
			Header:     http.Header{},
			Body:       io.NopCloser(strings.NewReader("data: {\"audio\":\"AAAA\"}\n\ndata: {\"audio\":\"BBBB\"}\n\n")),
		}

		usage := OpenaiTTSHandler(c, resp, info)
		if usage.PromptTokens != 30 {
			t.Fatalf("usage = %+v, want estimate prompt tokens", usage)
		}
		if !httpapi.GetContextKeyBool(c, common.ContextKeyLocalCountTokens) {
			t.Fatalf("stream without upstream usage must set ContextKeyLocalCountTokens")
		}
		if got := billing.BuildBillingDetailsForLog(c, info, usage); got != "" {
			t.Fatalf("billing_details must be skipped for estimated tts usage, got %s", got)
		}
	})

	t.Run("stream with upstream usage keeps billing details", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(http.MethodPost, "/v1/audio/speech", nil)
		info := &relaycommon.RelayInfo{
			ChannelMeta: &relaycommon.ChannelMeta{UpstreamModelName: "gpt-4o-mini-tts"},
			IsStream:    true,
		}
		info.SetEstimatePromptTokens(30)
		resp := &http.Response{
			StatusCode: http.StatusOK,
			Header:     http.Header{},
			Body:       io.NopCloser(strings.NewReader("data: {\"audio\":\"AAAA\"}\n\ndata: {\"usage\":{\"input_tokens\":10,\"output_tokens\":100,\"total_tokens\":110}}\n\n")),
		}

		usage := OpenaiTTSHandler(c, resp, info)
		if usage.PromptTokens != 10 || usage.CompletionTokens != 100 {
			t.Fatalf("usage = %+v, want upstream usage 10/100", usage)
		}
		if httpapi.GetContextKeyBool(c, common.ContextKeyLocalCountTokens) {
			t.Fatalf("upstream usage must not be flagged as local counting")
		}
		want := `{"schema_version":1,"tokens":{"input":{"text_input":10,"image_input":0,"audio_input":0,"video_input":0,"document_input":0},"output":{"text_output":100,"audio_output":0,"image_output":0,"reasoning_output":0,"accepted_prediction":0,"rejected_prediction":0},"cache":{"read_cache":0,"write_cache":0,"write_cache_5m":0,"write_cache_1h":0}}}`
		if got := billing.BuildBillingDetailsForLog(c, info, usage); got != want {
			t.Fatalf("billing_details = %s, want %s", got, want)
		}
	})
}

// TestOpenaiRealtimeHandlerTagsUsageSource 验证 Realtime 解析点按 Responses
// 同族规则标识来源。
func TestOpenaiRealtimeHandlerTagsUsageSource(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/v1/realtime", nil)
	info := &relaycommon.RelayInfo{
		RelayFormat: relayconstant.RelayFormatOpenAIRealtime,
		ChannelMeta: &relaycommon.ChannelMeta{UpstreamModelName: "gpt-4o-realtime-preview"},
	}

	// OpenaiRealtimeHandler 需要建立 websocket 连接，这里无法直接驱动完整流程；
	// 但打标必须发生在函数入口（任何提前返回也要有来源标识），
	// 因此只验证入口打标行为：调用后立即检查（连接失败也会先打标）。
	_, _ = OpenaiRealtimeHandler(c, info)
	if info.UsageSource != relayconstant.UsageSourceOpenAIResponses {
		t.Fatalf("usage source = %q, want %q", info.UsageSource, relayconstant.UsageSourceOpenAIResponses)
	}
}

// newRealtimeWssPair 建立一对内存 websocket：返回被测侧连接与测试侧对端。
func newRealtimeWssPair(t *testing.T) (tested *websocket.Conn, peer *websocket.Conn) {
	t.Helper()
	upgrader := websocket.Upgrader{CheckOrigin: func(*http.Request) bool { return true }}
	serverConnCh := make(chan *websocket.Conn, 1)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		serverConnCh <- conn
	}))
	t.Cleanup(srv.Close)

	dialed, _, err := websocket.DefaultDialer.Dial("ws"+strings.TrimPrefix(srv.URL, "http"), nil)
	if err != nil {
		t.Fatalf("dial websocket: %v", err)
	}
	t.Cleanup(func() { _ = dialed.Close() })
	select {
	case conn := <-serverConnCh:
		t.Cleanup(func() { _ = conn.Close() })
		return dialed, conn
	case <-time.After(3 * time.Second):
		t.Fatalf("websocket upgrade timeout")
		return nil, nil
	}
}

// runRealtimeHandler 驱动 OpenaiRealtimeHandler 并在 5 秒内等待其返回。
func runRealtimeHandler(t *testing.T, c *gin.Context, info *relaycommon.RelayInfo) *shared.RealtimeUsage {
	t.Helper()
	done := make(chan *shared.RealtimeUsage, 1)
	go func() {
		apiErr, sumUsage := OpenaiRealtimeHandler(c, info)
		if apiErr != nil {
			t.Errorf("OpenaiRealtimeHandler error: %v", apiErr)
		}
		done <- sumUsage
	}()
	select {
	case sumUsage := <-done:
		return sumUsage
	case <-time.After(5 * time.Second):
		t.Fatalf("OpenaiRealtimeHandler did not finish in time")
		return nil
	}
}

// runRealtimeHandlerResult 驱动 OpenaiRealtimeHandler 并同时返回错误与汇总
// 用量，不做错误断言，供预期失败的场景使用。
func runRealtimeHandlerResult(t *testing.T, c *gin.Context, info *relaycommon.RelayInfo) (*shared.NookMuxError, *shared.RealtimeUsage) {
	t.Helper()
	type handlerResult struct {
		apiErr   *shared.NookMuxError
		sumUsage *shared.RealtimeUsage
	}
	done := make(chan handlerResult, 1)
	go func() {
		apiErr, sumUsage := OpenaiRealtimeHandler(c, info)
		done <- handlerResult{apiErr: apiErr, sumUsage: sumUsage}
	}()
	select {
	case result := <-done:
		return result.apiErr, result.sumUsage
	case <-time.After(5 * time.Second):
		t.Fatalf("OpenaiRealtimeHandler did not finish in time")
		return nil, nil
	}
}

const (
	realtimeBillingTestUserId     = 901
	realtimeBillingTestChannelId  = 17
	realtimeBillingTestTokenId    = 71
	realtimeBillingTestTokenKey   = "sk-realtime-billing-test"
	realtimeBillingTestSeedQuota = 1000000
)

// setupRealtimeBillingTestDB 提供事件实扣所需的内存 SQLite fixture：
// UsePrice 免库旁路删除后（P1-4），realtime 会话计费在 handler 测试内
// 真实落库，与 billing 包的 fixture 范式保持一致。
func setupRealtimeBillingTestDB(t *testing.T) {
	t.Helper()

	oldDB := dbstore.DB
	oldLogDB := dbstore.LOG_DB
	oldMemoryCacheEnabled := common.MemoryCacheEnabled
	oldBatchUpdateEnabled := common.BatchUpdateEnabled
	// 这些全局会被 DecreaseUserQuota 的异步池任务并发读取；即使写入相同
	// 值也是数据竞争，因此只在取值不同时写入。redis.RedisEnabled 的包初始
	// 值为 true（InitRedisClient 不会在测试二进制中运行），fixture 置 false
	// 后保持、不在 cleanup 写回：恢复写会与迟到的异步池任务构成竞争，且本
	// 包测试二进制内没有依赖 true 的用例。
	setGlobal := func(current *bool, want bool) {
		if *current != want {
			*current = want
		}
	}

	testDB, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite test db: %v", err)
	}
	if err := testDB.AutoMigrate(&userstore.User{}, &channelstore.Channel{}, &tokenstore.Token{},
		&pricingstore.ModelPricePlan{}, &pricingstore.ModelPriceComponent{}); err != nil {
		t.Fatalf("migrate sqlite test db: %v", err)
	}
	pricingstore.InvalidateModelPricePlanCache()
	dbstore.DB = testDB
	dbstore.LOG_DB = testDB
	setGlobal(&redis.RedisEnabled, false)
	setGlobal(&common.MemoryCacheEnabled, false)
	setGlobal(&common.BatchUpdateEnabled, false)

	if err := testDB.Create(&userstore.User{
		Id:       realtimeBillingTestUserId,
		Username: "realtime-billing-tester",
		Quota:    realtimeBillingTestSeedQuota,
	}).Error; err != nil {
		t.Fatalf("seed user: %v", err)
	}
	if err := testDB.Create(&channelstore.Channel{
		Id:   realtimeBillingTestChannelId,
		Name: "realtime-billing-channel",
	}).Error; err != nil {
		t.Fatalf("seed channel: %v", err)
	}
	if err := testDB.Create(&tokenstore.Token{
		Id:     realtimeBillingTestTokenId,
		UserId: realtimeBillingTestUserId,
		Key:    realtimeBillingTestTokenKey,
	}).Error; err != nil {
		t.Fatalf("seed token: %v", err)
	}

	t.Cleanup(func() {
		if sqlDB, err := testDB.DB(); err == nil {
			_ = sqlDB.Close()
		}
		dbstore.DB = oldDB
		dbstore.LOG_DB = oldLogDB
		setGlobal(&common.MemoryCacheEnabled, oldMemoryCacheEnabled)
		setGlobal(&common.BatchUpdateEnabled, oldBatchUpdateEnabled)
		pricingstore.InvalidateModelPricePlanCache()
	})
}

// newRealtimeBillingTestRelayInfo 构造 ratio 模式（全 1 倍率）的会话：
// 每事件 quota 等于 token 数之和，便于精确断言。
func newRealtimeBillingTestRelayInfo() *relaycommon.RelayInfo {
	info := &relaycommon.RelayInfo{
		RelayFormat: relayconstant.RelayFormatOpenAIRealtime,
		OriginModelName: "gpt-4o-realtime-preview",
		ChannelMeta: &relaycommon.ChannelMeta{
			ChannelId:         realtimeBillingTestChannelId,
			UpstreamModelName: "gpt-4o-realtime-preview",
		},
		UserId:         realtimeBillingTestUserId,
		TokenId:        realtimeBillingTestTokenId,
		TokenKey:       realtimeBillingTestTokenKey,
		TokenUnlimited: true,
		TokenQuotaType: 0,
		UsingGroup:     "default",
		StartTime:      time.Now(),
	}
	info.PriceData.ModelRatio = 1
	info.PriceData.CompletionRatio = 1
	info.PriceData.CacheRatio = 1
	info.PriceData.AudioRatio = 1
	info.PriceData.AudioCompletionRatio = 1
	info.PriceData.GroupRatioInfo.GroupRatio = 1
	return info
}

func newRealtimeBillingTestContext() *gin.Context {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/v1/realtime", nil)
	c.Set("token_name", "tk-realtime-billing")
	return c
}

func realtimeTestUserQuota(t *testing.T) int {
	t.Helper()
	var user userstore.User
	if err := dbstore.DB.First(&user, realtimeBillingTestUserId).Error; err != nil {
		t.Fatalf("load realtime billing test user: %v", err)
	}
	return user.Quota
}

// TestOpenaiRealtimeHandlerBillingDetailsLocalCounting 验证 realtime 会话中
// 上游 done 事件缺失 usage 时按本地 tokenizer 计数计费（现有计费行为不变），
// 但 billing_details 必须跳过：本地估算不得伪装成上游官方用量落列。
func TestOpenaiRealtimeHandlerBillingDetailsLocalCounting(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("done without usage bills local counting and skips billing details", func(t *testing.T) {
		setupRealtimeBillingTestDB(t)
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(http.MethodGet, "/v1/realtime", nil)
		c.Set("token_name", "tk-realtime-billing")
		info := newRealtimeBillingTestRelayInfo()

		clientWs, clientPeer := newRealtimeWssPair(t)
		info.ClientWs = clientWs
		targetWs, targetPeer := newRealtimeWssPair(t)
		info.TargetWs = targetWs

		// target 对端：丢弃转发消息；收到被转发的 session.update（证明输入
		// token 已计数）后下发 response 非空但 usage 缺失的 response.done
		//（走本地计数计费分支），随后断开触发 handler 收尾。
		go func() {
			for {
				_, msg, err := targetPeer.ReadMessage()
				if err != nil {
					return
				}
				var probe map[string]any
				if jsonx.Unmarshal(msg, &probe) == nil && probe["type"] == "session.update" {
					_ = targetPeer.WriteMessage(websocket.TextMessage, []byte(`{"type":"response.done","response":{}}`))
					time.Sleep(50 * time.Millisecond)
					_ = targetPeer.Close()
					return
				}
			}
		}()
		if err := clientPeer.WriteMessage(websocket.TextMessage, []byte(`{"type":"session.update","session":{"instructions":"hello realtime billing details"}}`)); err != nil {
			t.Fatalf("write client event: %v", err)
		}

		sumUsage := runRealtimeHandler(t, c, info)

		if sumUsage.InputTokens == 0 {
			t.Fatalf("local counting should still be billed, sumUsage = %+v", sumUsage)
		}
		if info.WssEventConsumedQuota == 0 {
			t.Fatalf("local counting must be billed per event, eventConsumedQuota = %d", info.WssEventConsumedQuota)
		}
		if got := realtimeTestUserQuota(t); got != realtimeBillingTestSeedQuota-info.WssEventConsumedQuota {
			t.Fatalf("wallet = %d, want seed - eventConsumedQuota (%d)", got, realtimeBillingTestSeedQuota-info.WssEventConsumedQuota)
		}
		if !httpapi.GetContextKeyBool(c, common.ContextKeyLocalCountTokens) {
			t.Fatalf("session billed with local counting must set ContextKeyLocalCountTokens")
		}
		if got := billing.BuildRealtimeBillingDetailsForLog(c, info, sumUsage); got != "" {
			t.Fatalf("billing_details must be skipped when local counting was billed, got %s", got)
		}
	})

	t.Run("leftover local usage billed at close flags local", func(t *testing.T) {
		setupRealtimeBillingTestDB(t)
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(http.MethodGet, "/v1/realtime", nil)
		c.Set("token_name", "tk-realtime-billing")
		info := newRealtimeBillingTestRelayInfo()

		clientWs, _ := newRealtimeWssPair(t)
		info.ClientWs = clientWs
		targetWs, targetPeer := newRealtimeWssPair(t)
		info.TargetWs = targetWs

		// target 对端：只下发输出 delta 事件（本地计数进 localUsage.OutputTokens），
		// 从不下发 response.done，随后断开——触发连接收尾时 leftover localUsage
		// 计费分支。
		go func() {
			_ = targetPeer.WriteMessage(websocket.TextMessage, []byte(`{"type":"response.audio_transcript.delta","delta":"hello realtime"}`))
			_ = targetPeer.WriteMessage(websocket.TextMessage, []byte(`{"type":"response.audio_transcript.delta","delta":" leftover events"}`))
			time.Sleep(50 * time.Millisecond)
			_ = targetPeer.Close()
		}()

		sumUsage := runRealtimeHandler(t, c, info)

		if sumUsage.OutputTokens == 0 {
			t.Fatalf("leftover local counting should still be billed, sumUsage = %+v", sumUsage)
		}
		if info.WssEventConsumedQuota == 0 {
			t.Fatalf("leftover local counting must be billed at close, eventConsumedQuota = %d", info.WssEventConsumedQuota)
		}
		if got := realtimeTestUserQuota(t); got != realtimeBillingTestSeedQuota-info.WssEventConsumedQuota {
			t.Fatalf("wallet = %d, want seed - eventConsumedQuota (%d)", got, realtimeBillingTestSeedQuota-info.WssEventConsumedQuota)
		}
		if !httpapi.GetContextKeyBool(c, common.ContextKeyLocalCountTokens) {
			t.Fatalf("session billed with leftover local counting must set ContextKeyLocalCountTokens")
		}
		if got := billing.BuildRealtimeBillingDetailsForLog(c, info, sumUsage); got != "" {
			t.Fatalf("billing_details must be skipped when leftover local counting was billed, got %s", got)
		}
	})

	t.Run("done with usage keeps billing details", func(t *testing.T) {
		setupRealtimeBillingTestDB(t)
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(http.MethodGet, "/v1/realtime", nil)
		c.Set("token_name", "tk-realtime-billing")
		info := newRealtimeBillingTestRelayInfo()

		clientWs, _ := newRealtimeWssPair(t)
		info.ClientWs = clientWs
		targetWs, targetPeer := newRealtimeWssPair(t)
		info.TargetWs = targetWs

		// target 对端：下发带官方 usage 的 response.done 后断开。
		go func() {
			_ = targetPeer.WriteMessage(websocket.TextMessage, []byte(`{"type":"response.done","response":{"usage":{"total_tokens":100,"input_tokens":60,"output_tokens":40,"input_token_details":{"cached_tokens":10,"text_tokens":40,"audio_tokens":10},"output_token_details":{"text_tokens":30,"audio_tokens":10}}}}`))
			time.Sleep(50 * time.Millisecond)
			_ = targetPeer.Close()
		}()

		sumUsage := runRealtimeHandler(t, c, info)

		if sumUsage.InputTokens != 60 || sumUsage.OutputTokens != 40 {
			t.Fatalf("sumUsage = %+v, want upstream usage 60/40", sumUsage)
		}
		// 全 1 倍率：quota = token 数之和 = 60 + 40 = 100，事件级实扣。
		if info.WssEventConsumedQuota != 100 {
			t.Fatalf("eventConsumedQuota = %d, want 100", info.WssEventConsumedQuota)
		}
		if got := realtimeTestUserQuota(t); got != realtimeBillingTestSeedQuota-100 {
			t.Fatalf("wallet = %d, want %d", got, realtimeBillingTestSeedQuota-100)
		}
		if httpapi.GetContextKeyBool(c, common.ContextKeyLocalCountTokens) {
			t.Fatalf("upstream usage session must not be flagged as local counting")
		}
		want := `{"schema_version":1,"tokens":{"input":{"text_input":40,"image_input":0,"audio_input":10,"video_input":0,"document_input":0},"output":{"text_output":30,"audio_output":10,"image_output":0,"reasoning_output":0,"accepted_prediction":0,"rejected_prediction":0},"cache":{"read_cache":10,"write_cache":0,"write_cache_5m":0,"write_cache_1h":0}}}`
		if got := billing.BuildRealtimeBillingDetailsForLog(c, info, sumUsage); got != want {
			t.Fatalf("billing_details = %s, want %s", got, want)
		}
	})
}

// TestOpenaiRealtimeHandlerConcurrentInterleaveIsLossless 双向并发交错用例：
// client 输入事件与 target response.done 事件并发注入，断言 sumUsage 精确
// 等于全部官方 usage 之和、钱包净扣等于事件实扣累计（无丢失更新）。
