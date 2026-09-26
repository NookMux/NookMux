package logstore_test

import (
	"fmt"
	"github.com/NookMux/NookMux/internal/common"
	"github.com/NookMux/NookMux/internal/infra/redis"
	"github.com/NookMux/NookMux/internal/store/db"
	"github.com/NookMux/NookMux/internal/store/log"
	"github.com/NookMux/NookMux/internal/store/user"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func setupLogAdminInfoTestDB(t *testing.T) {
	t.Helper()

	oldDB := dbstore.DB
	oldLogDB := dbstore.LOG_DB
	oldRedisEnabled := redis.RedisEnabled
	oldMemoryCacheEnabled := common.MemoryCacheEnabled

	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite test db: %v", err)
	}
	if err := db.AutoMigrate(&userstore.User{}, &logstore.Log{}); err != nil {
		t.Fatalf("migrate sqlite test db: %v", err)
	}
	dbstore.DB = db
	dbstore.LOG_DB = db
	redis.RedisEnabled = false
	common.MemoryCacheEnabled = false

	t.Cleanup(func() {
		if sqlDB, err := db.DB(); err == nil {
			_ = sqlDB.Close()
		}
		dbstore.DB = oldDB
		dbstore.LOG_DB = oldLogDB
		redis.RedisEnabled = oldRedisEnabled
		common.MemoryCacheEnabled = oldMemoryCacheEnabled
	})
}

func TestRecordLogWithAdminInfoIsStrippedFromUserLogs(t *testing.T) {
	setupLogAdminInfoTestDB(t)

	user := &userstore.User{
		Id:       1,
		Username: "target-user",
		Status:   common.UserStatusEnabled,
		AffCode:  "log-admin-info-target",
	}
	if err := dbstore.DB.Create(user).Error; err != nil {
		t.Fatalf("create user: %v", err)
	}

	userstore.RecordLogWithAdminInfo(user.Id, logstore.LogTypeManage, "管理员强制禁用了用户的两步验证", map[string]interface{}{
		"admin_id":       99,
		"admin_username": "root-admin",
	})

	var logs []*logstore.Log
	if err := dbstore.LOG_DB.Where("user_id = ?", user.Id).Find(&logs).Error; err != nil {
		t.Fatalf("query logs: %v", err)
	}
	if len(logs) != 1 {
		t.Fatalf("log count = %d, want 1", len(logs))
	}
	if !strings.Contains(logs[0].Other, "admin_info") {
		t.Fatalf("stored log does not contain admin_info: %s", logs[0].Other)
	}

	logstore.FormatUserLogs(logs, 0)
	if strings.Contains(logs[0].Other, "admin_info") || strings.Contains(logs[0].Other, "root-admin") {
		t.Fatalf("formatted user log leaked admin info: %s", logs[0].Other)
	}
	if strings.Contains(logs[0].Content, "99") || strings.Contains(logs[0].Content, "root-admin") {
		t.Fatalf("log content leaked admin identity: %s", logs[0].Content)
	}
}

// TestRecordTopupLogStoresAuditInfoAndStripsItFromUserLogs 验证充值日志写入
// 服务器 IP / 回调 IP / 支付方式 / 系统版本等审计字段，并确认普通用户查询
// 路径（FormatUserLogs）会剥离整块 admin_info。
func TestRecordTopupLogStoresAuditInfoAndStripsItFromUserLogs(t *testing.T) {
	setupLogAdminInfoTestDB(t)

	user := &userstore.User{
		Id:       1,
		Username: "topup-audit-user",
		Status:   common.UserStatusEnabled,
		AffCode:  "log-topup-audit",
	}
	if err := dbstore.DB.Create(user).Error; err != nil {
		t.Fatalf("create user: %v", err)
	}

	userstore.RecordTopupLog(user.Id, "管理员补单成功，充值金额: 1，支付金额：1.000000", "198.51.100.7", "alipay", "admin")

	var logs []*logstore.Log
	if err := dbstore.LOG_DB.Where("user_id = ?", user.Id).Find(&logs).Error; err != nil {
		t.Fatalf("query logs: %v", err)
	}
	if len(logs) != 1 {
		t.Fatalf("log count = %d, want 1", len(logs))
	}
	if logs[0].Type != logstore.LogTypeTopup {
		t.Fatalf("log type = %d, want %d", logs[0].Type, logstore.LogTypeTopup)
	}
	for _, want := range []string{
		`"admin_info"`,
		`"server_ip"`,
		`"caller_ip":"198.51.100.7"`,
		`"payment_method":"alipay"`,
		`"callback_payment_method":"admin"`,
		`"version"`,
	} {
		if !strings.Contains(logs[0].Other, want) {
			t.Fatalf("stored topup log %s missing %s", logs[0].Other, want)
		}
	}

	logstore.FormatUserLogs(logs, 0)
	if strings.Contains(logs[0].Other, "admin_info") || strings.Contains(logs[0].Other, "198.51.100.7") {
		t.Fatalf("formatted user topup log leaked audit info: %s", logs[0].Other)
	}
}

func TestRecordErrorLogStoresAndFiltersUpstreamRequestId(t *testing.T) {
	setupLogAdminInfoTestDB(t)
	gin.SetMode(gin.TestMode)

	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, "http://example.test/v1/chat/completions", nil)
	c.Set("username", "target-user")
	c.Set(common.RequestIdKey, "local-id")
	c.Set(common.UpstreamRequestIdKey, "upstream-id")

	logstore.RecordErrorLog(c, 1, 0, "gpt-test", "token", "upstream failed", 0, 10, false, "default", nil)

	var stored logstore.Log
	if err := dbstore.LOG_DB.First(&stored).Error; err != nil {
		t.Fatalf("query stored log: %v", err)
	}
	if stored.RequestId != "local-id" {
		t.Fatalf("stored request id = %q, want local-id", stored.RequestId)
	}
	if stored.UpstreamRequestId != "upstream-id" {
		t.Fatalf("stored upstream request id = %q, want upstream-id", stored.UpstreamRequestId)
	}

	logs, total, err := logstore.GetAllLogs(logstore.LogTypeUnknown, 0, 0, "", "", "", 0, 20, 0, "", "", "upstream-id", "", "", "", "")
	if err != nil {
		t.Fatalf("GetAllLogs error = %v", err)
	}
	if total != 1 || len(logs) != 1 || logs[0].UpstreamRequestId != "upstream-id" {
		t.Fatalf("filtered logs total=%d logs=%#v, want one upstream-id log", total, logs)
	}
}

func TestRecordErrorLogTruncatesStoredContent(t *testing.T) {
	setupLogAdminInfoTestDB(t)
	gin.SetMode(gin.TestMode)
	oldDebug := common.DebugEnabled
	common.DebugEnabled = false
	t.Cleanup(func() {
		common.DebugEnabled = oldDebug
	})

	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, "http://example.test/v1/chat/completions", nil)
	c.Set("username", "target-user")

	content := strings.Repeat("x", common.LocalLogContentLimit+128)
	logstore.RecordErrorLog(c, 1, 0, "gpt-test", "token", content, 0, 10, false, "default", nil)

	var stored logstore.Log
	if err := dbstore.LOG_DB.First(&stored).Error; err != nil {
		t.Fatalf("query stored log: %v", err)
	}
	if !strings.Contains(stored.Content, "[truncated") {
		t.Fatalf("stored content was not truncated: length=%d content=%q", len(stored.Content), stored.Content)
	}
	if strings.Contains(stored.Content, strings.Repeat("x", common.LocalLogContentLimit+1)) {
		t.Fatal("stored content includes more than the allowed preview")
	}
}

func TestRecordErrorLogStoresObjectForNilOther(t *testing.T) {
	setupLogAdminInfoTestDB(t)
	gin.SetMode(gin.TestMode)

	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, "http://example.test/v1/chat/completions", nil)
	c.Set("username", "target-user")

	// nil Other must serialize to a JSON object, not the literal "null".
	logstore.RecordErrorLog(c, 1, 0, "gpt-test", "token", "boom", 0, 10, false, "default", nil)

	var stored logstore.Log
	if err := dbstore.LOG_DB.First(&stored).Error; err != nil {
		t.Fatalf("query stored log: %v", err)
	}
	if stored.Other != "{}" {
		t.Fatalf("stored Other for nil input = %q, want {}", stored.Other)
	}
}
