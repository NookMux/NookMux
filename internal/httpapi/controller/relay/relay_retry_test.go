package relaycontroller

import (
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/NookMux/NookMux/internal/common"
	"github.com/NookMux/NookMux/internal/config/operation"
	"github.com/NookMux/NookMux/internal/config/ratio"
	domainchannel "github.com/NookMux/NookMux/internal/domain/channel"
	channelconstant "github.com/NookMux/NookMux/internal/domain/channel/constant"
	"github.com/NookMux/NookMux/internal/domain/shared"
	"github.com/NookMux/NookMux/internal/httpapi"
	"github.com/NookMux/NookMux/internal/httpapi/middleware"
	relayconstant "github.com/NookMux/NookMux/internal/relay/constant"
	"github.com/NookMux/NookMux/internal/relay/helper"
	redisinfra "github.com/NookMux/NookMux/internal/infra/redis"
	"github.com/NookMux/NookMux/internal/infra/httpclient"
	channelstore "github.com/NookMux/NookMux/internal/store/channel"
	dbstore "github.com/NookMux/NookMux/internal/store/db"
	logstore "github.com/NookMux/NookMux/internal/store/log"
	tokenstore "github.com/NookMux/NookMux/internal/store/token"
	userstore "github.com/NookMux/NookMux/internal/store/user"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestShouldRetryUsesNumericUpstreamErrorCode(t *testing.T) {
	orig := operation.AutomaticRetryStatusCodeRanges
	t.Cleanup(func() { operation.AutomaticRetryStatusCodeRanges = orig })
	operation.AutomaticRetryStatusCodeRanges = []operation.StatusCodeRange{{Start: 500, End: 599}}

	for _, code := range []string{"502", "504"} {
		t.Run(code, func(t *testing.T) {
			c, _ := gin.CreateTestContext(httptest.NewRecorder())
			err := shared.WithOpenAIError(shared.OpenAIError{
				Message: "upstream gateway error",
				Type:    "upstream_error",
				Code:    code,
			}, http.StatusOK)

			require.True(t, shouldRetry(c, err, 1))
		})
	}
}

func TestShouldRetryIgnoresNonNumericUpstreamErrorCode(t *testing.T) {
	orig := operation.AutomaticRetryStatusCodeRanges
	t.Cleanup(func() { operation.AutomaticRetryStatusCodeRanges = orig })
	operation.AutomaticRetryStatusCodeRanges = []operation.StatusCodeRange{{Start: 500, End: 599}}

	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	err := shared.WithOpenAIError(shared.OpenAIError{
		Message: "bad request",
		Type:    "invalid_request_error",
		Code:    "invalid_request",
	}, http.StatusBadRequest)

	require.False(t, shouldRetry(c, err, 1))
}

func TestShouldRetryUsesOriginalStatusCodeAfterMapping(t *testing.T) {
	orig := operation.AutomaticRetryStatusCodeRanges
	t.Cleanup(func() { operation.AutomaticRetryStatusCodeRanges = orig })
	operation.AutomaticRetryStatusCodeRanges = []operation.StatusCodeRange{{Start: 429, End: 429}}

	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	err := shared.WithOpenAIError(shared.OpenAIError{
		Message: "rate limited",
		Type:    "upstream_error",
		Code:    "rate_limit_exceeded",
	}, http.StatusTooManyRequests)

	helper.ResetStatusCode(err, `{"429":"200"}`)

	require.Equal(t, http.StatusTooManyRequests, err.OriginalStatusCode)
	require.Equal(t, http.StatusOK, err.StatusCode)
	require.True(t, shouldRetry(c, err, 1))
}

func TestShouldRetryConfiguredTransientStatusCodes(t *testing.T) {
	orig := operation.AutomaticRetryStatusCodeRanges
	t.Cleanup(func() { operation.AutomaticRetryStatusCodeRanges = orig })
	require.NoError(t, operation.AutomaticRetryStatusCodesFromString("100-199,300-399,401-407,409-599"))

	for _, statusCode := range []int{http.StatusTooManyRequests, 529} {
		t.Run(strconv.Itoa(statusCode), func(t *testing.T) {
			c, _ := gin.CreateTestContext(httptest.NewRecorder())
			err := shared.WithOpenAIError(shared.OpenAIError{
				Message: "transient upstream failure",
				Type:    "upstream_error",
				Code:    statusCode,
			}, statusCode)

			require.True(t, shouldRetry(c, err, 20))
		})
	}
}

func TestProcessChannelError_PerAttemptDurationNotCumulative(t *testing.T) {
	gin.SetMode(gin.TestMode)

	oldDB := dbstore.DB
	oldLogDB := dbstore.LOG_DB
	oldErrorLogEnabled := shared.ErrorLogEnabled
	shared.ErrorLogEnabled = true

	testDB, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, testDB.AutoMigrate(&logstore.Log{}))

	dbstore.DB = testDB
	dbstore.LOG_DB = testDB

	t.Cleanup(func() {
		if sqlDB, err := testDB.DB(); err == nil {
			_ = sqlDB.Close()
		}
		dbstore.DB = oldDB
		dbstore.LOG_DB = oldLogDB
		shared.ErrorLogEnabled = oldErrorLogEnabled
	})

	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest("POST", "/v1/chat/completions", nil)
	c.Set("id", 1)
	c.Set("token_name", "test-token")
	c.Set("original_model", "gpt-4o")
	c.Set("token_id", 1)
	c.Set("group", "default")

	// Attempt 1: takes ~100ms
	c.Set("channel_id", 1)
	c.Set("channel_name", "channel-1")
	c.Set("channel_type", 1)
	attempt1Start := time.Now().Add(-100 * time.Millisecond)
	httpapi.SetContextKey(c, common.ContextKeyRequestStartTime, attempt1Start)
	channelError1 := *domainchannel.NewChannelError(1, 1, "channel-1", false, "key-1", false)
	err1 := shared.NewError(errors.New("upstream timeout"), shared.ErrorCodeDoRequestFailed)
	ProcessChannelError(c, channelError1, err1)

	// Attempt 2: retry occurs, start time is reset to attempt 2's start (~30ms ago)
	c.Set("channel_id", 2)
	c.Set("channel_name", "channel-2")
	c.Set("channel_type", 1)
	attempt2Start := time.Now().Add(-30 * time.Millisecond)
	httpapi.SetContextKey(c, common.ContextKeyRequestStartTime, attempt2Start)
	channelError2 := *domainchannel.NewChannelError(2, 1, "channel-2", false, "key-2", false)
	err2 := shared.NewError(errors.New("rate limited"), shared.ErrorCodeDoRequestFailed)
	ProcessChannelError(c, channelError2, err2)

	var logs []logstore.Log
	require.NoError(t, testDB.Order("id asc").Find(&logs).Error)
	require.Len(t, logs, 2)

	// Attempt 1 log
	require.Equal(t, 1, logs[0].ChannelId)
	require.GreaterOrEqual(t, logs[0].UseTime, 90)
	require.LessOrEqual(t, logs[0].UseTime, 300)

	// Attempt 2 log: must reflect only attempt 2's duration (~30ms), NOT 100+30=130ms
	require.Equal(t, 2, logs[1].ChannelId)
	require.GreaterOrEqual(t, logs[1].UseTime, 25)
	require.Less(t, logs[1].UseTime, 90)
}

func TestRelay_RetryLoopPerAttemptDurationNotCumulative(t *testing.T) {
	gin.SetMode(gin.TestMode)

	oldDB := dbstore.DB
	oldLogDB := dbstore.LOG_DB
	oldErrorLogEnabled := shared.ErrorLogEnabled
	oldRetryTimes := common.RetryTimes
	oldAutomaticRetryEnabled := common.AutomaticRetryEnabled
	oldMemoryCacheEnabled := common.MemoryCacheEnabled
	oldRetryRanges := operation.AutomaticRetryStatusCodeRanges
	oldRedisEnabled := redisinfra.RedisEnabled

	shared.ErrorLogEnabled = true
	common.AutomaticRetryEnabled = true
	common.RetryTimes = 1
	common.MemoryCacheEnabled = true
	redisinfra.RedisEnabled = false
	operation.AutomaticRetryStatusCodeRanges = []operation.StatusCodeRange{{Start: 500, End: 599}}
	ratio.InitRatioSettings()
	httpclient.InitHttpClient()

	testDB, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, testDB.AutoMigrate(&userstore.User{}, &tokenstore.Token{}, &channelstore.Channel{}, &channelstore.Ability{}, &logstore.Log{}))

	dbstore.DB = testDB
	dbstore.LOG_DB = testDB

	t.Cleanup(func() {
		if sqlDB, err := testDB.DB(); err == nil {
			_ = sqlDB.Close()
		}
		dbstore.DB = oldDB
		dbstore.LOG_DB = oldLogDB
		shared.ErrorLogEnabled = oldErrorLogEnabled
		common.RetryTimes = oldRetryTimes
		common.AutomaticRetryEnabled = oldAutomaticRetryEnabled
		common.MemoryCacheEnabled = oldMemoryCacheEnabled
		operation.AutomaticRetryStatusCodeRanges = oldRetryRanges
		redisinfra.RedisEnabled = oldRedisEnabled
	})

	var requestCount int
	var requestMu sync.Mutex
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestMu.Lock()
		requestCount++
		count := requestCount
		requestMu.Unlock()

		w.Header().Set("Content-Type", "application/json")
		if count == 1 {
			time.Sleep(100 * time.Millisecond)
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte(`{"error": {"message": "attempt 1 failed", "type": "upstream_error", "code": "500"}}`))
			return
		}
		time.Sleep(30 * time.Millisecond)
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"error": {"message": "attempt 2 failed", "type": "upstream_error", "code": "500"}}`))
	}))
	defer mockServer.Close()

	user := &userstore.User{Id: 1, Username: "test-user", Quota: 1000000, Status: common.UserStatusEnabled}
	require.NoError(t, testDB.Create(user).Error)

	baseURL := mockServer.URL
	priority := int64(0)
	weight := uint(1)
	autoBan := 0
	ch1 := channelstore.Channel{
		Id:       1,
		Type:     channelconstant.ChannelTypeOpenAI,
		Name:     "channel-1",
		BaseURL:  &baseURL,
		Key:      "sk-test1",
		Models:   "gpt-4o",
		Group:    "default",
		Status:   common.ChannelStatusEnabled,
		Priority: &priority,
		Weight:   &weight,
		AutoBan:  &autoBan,
	}
	require.NoError(t, testDB.Create(&ch1).Error)
	require.NoError(t, ch1.AddAbilities(testDB))

	ch2 := channelstore.Channel{
		Id:       2,
		Type:     channelconstant.ChannelTypeOpenAI,
		Name:     "channel-2",
		BaseURL:  &baseURL,
		Key:      "sk-test2",
		Models:   "gpt-4o",
		Group:    "default",
		Status:   common.ChannelStatusEnabled,
		Priority: &priority,
		Weight:   &weight,
		AutoBan:  &autoBan,
	}
	require.NoError(t, testDB.Create(&ch2).Error)
	require.NoError(t, ch2.AddAbilities(testDB))

	channelstore.InitChannelCache()

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	reqBody := `{"model": "gpt-4o", "messages": [{"role": "user", "content": "hi"}]}`
	c.Request = httptest.NewRequest("POST", "/v1/chat/completions", strings.NewReader(reqBody))
	c.Request.Header.Set("Content-Type", "application/json")

	apiErr := middleware.SetupContextForSelectedChannel(c, &ch1, "gpt-4o")
	require.Nil(t, apiErr)
	c.Set("id", 1)
	c.Set("username", "test-user")
	c.Set("token_name", "test-token")
	c.Set("original_model", "gpt-4o")
	c.Set("group", "default")
	httpapi.SetContextKey(c, common.ContextKeyUserId, 1)
	httpapi.SetContextKey(c, common.ContextKeyTokenId, 1)
	httpapi.SetContextKey(c, common.ContextKeyTokenKey, "sk-test")
	httpapi.SetContextKey(c, common.ContextKeyTokenUnlimited, true)
	httpapi.SetContextKey(c, common.ContextKeyOriginalModel, "gpt-4o")
	httpapi.SetContextKey(c, common.ContextKeyUserGroup, "default")
	httpapi.SetContextKey(c, common.ContextKeyUsingGroup, "default")
	httpapi.SetContextKey(c, common.ContextKeyRequestStartTime, time.Now())

	Relay(c, relayconstant.RelayFormatOpenAI)
	time.Sleep(50 * time.Millisecond)

	var logs []logstore.Log
	require.NoError(t, testDB.Order("id asc").Find(&logs).Error)
	require.Len(t, logs, 2)

	require.Equal(t, 1, logs[0].ChannelId)
	require.GreaterOrEqual(t, logs[0].UseTime, 90)

	// Crucial: Attempt 2 ran via Relay()'s real retry loop.
	// Its use_time must reflect ONLY attempt 2's ~30ms, NOT 100+30=130ms.
	require.Equal(t, 2, logs[1].ChannelId)
	require.GreaterOrEqual(t, logs[1].UseTime, 25)
	require.Less(t, logs[1].UseTime, 90)
}
