package logstore_test

import (
	"fmt"
	"testing"
	"time"

	"github.com/NookMux/NookMux/internal/store/db"
	"github.com/NookMux/NookMux/internal/store/log"
)

// simpleBillingDetailsJSON 构造只有文本输入/输出、其余字段为 0 的 canonical
// JSON，输入侧总量与输出总量即 text_input/text_output。
func simpleBillingDetailsJSON(textInput int, textOutput int) string {
	return fmt.Sprintf(`{"schema_version":1,"tokens":{"input":{"text_input":%d,"image_input":0,"audio_input":0,"video_input":0,"document_input":0},"output":{"text_output":%d,"audio_output":0,"image_output":0,"reasoning_output":0,"accepted_prediction":0,"rejected_prediction":0},"cache":{"read_cache":0,"write_cache":0,"write_cache_5m":0,"write_cache_1h":0}}}`, textInput, textOutput)
}

func TestSumUsedQuotaAppliesLogOnlyFilters(t *testing.T) {
	setupLogAdminInfoTestDB(t)

	now := time.Now().Unix()
	logs := []*logstore.Log{
		{
			UserId:            1,
			CreatedAt:         now,
			Type:              logstore.LogTypeConsume,
			Username:          "target-user",
			TokenName:         "target-token",
			ModelName:         "gpt-target",
			Quota:             120,
			ChannelId:         1,
			Group:             "default",
			Ip:                "203.0.113.10",
			RequestId:         "target-request",
			UpstreamRequestId: "target-upstream",
			Ua:                "TargetAgent/1.0",
			XTitle:            "Target Title",
			HttpReferer:       "https://target.example/path",
			BillingDetails:    strPtr(simpleBillingDetailsJSON(10, 20)),
		},
		{
			UserId:            2,
			CreatedAt:         now,
			Type:              logstore.LogTypeConsume,
			Username:          "other-user",
			TokenName:         "other-token",
			ModelName:         "gpt-other",
			Quota:             300,
			ChannelId:         2,
			Group:             "vip",
			Ip:                "198.51.100.20",
			RequestId:         "other-request",
			UpstreamRequestId: "other-upstream",
			Ua:                "OtherAgent/1.0",
			XTitle:            "Other Title",
			HttpReferer:       "https://other.example/path",
		},
	}
	if err := dbstore.LOG_DB.Create(&logs).Error; err != nil {
		t.Fatalf("create logs: %v", err)
	}

	var matched int64
	if err := dbstore.LOG_DB.Model(&logstore.Log{}).Where("request_id = ?", "target-request").Count(&matched).Error; err != nil {
		t.Fatalf("count target log: %v", err)
	}
	if matched != 1 {
		t.Fatalf("target log count = %d, want 1", matched)
	}

	stat, err := logstore.SumUsedQuota(logstore.LogTypeConsume, now-1, now+1, logstore.LogStatFilter{
		RequestId:         "target-request",
		UpstreamRequestId: "target-upstream",
		Ip:                "203.0.113",
		Ua:                "TargetAgent",
		XTitle:            "Target Title",
		HttpReferer:       "target.example",
	})
	if err != nil {
		t.Fatalf("SumUsedQuota error = %v", err)
	}

	if stat.Quota != 120 {
		t.Fatalf("quota = %d, want 120", stat.Quota)
	}
	if stat.Rpm != 1 {
		t.Fatalf("rpm = %d, want 1", stat.Rpm)
	}
	// TPM 来自 billing_details 唯一汇总公式：输入侧总量 10 + 输出总量 20。
	if stat.Tpm != 30 {
		t.Fatalf("tpm = %d, want 30", stat.Tpm)
	}
	if stat.SuccessCount != 1 {
		t.Fatalf("success_count = %d, want 1", stat.SuccessCount)
	}
}

// TestSumUsedQuotaTpmSumsProcessedTotals 验证 TPM 按 billing_details 的处理
// 总量求和：缓存计入输入侧总量，reasoning 不重复相加，NULL 明细计 0，
// 损坏 JSON 显式报错。
func TestSumUsedQuotaTpmSumsProcessedTotals(t *testing.T) {
	setupLogAdminInfoTestDB(t)

	now := time.Now().Unix()
	// 输入侧总量 = 100 + 200 + 300 + 400 = 1000；输出总量 = 500；处理总量 1500。
	withCache := `{"schema_version":1,"tokens":{"input":{"text_input":100,"image_input":200,"audio_input":0,"video_input":0,"document_input":0},"output":{"text_output":500,"audio_output":0,"image_output":0,"reasoning_output":50,"accepted_prediction":0,"rejected_prediction":0},"cache":{"read_cache":300,"write_cache":400,"write_cache_5m":400,"write_cache_1h":0}}}`
	logs := []*logstore.Log{
		{UserId: 1, CreatedAt: now, Type: logstore.LogTypeConsume, Username: "u1", Group: "default", Quota: 1, BillingDetails: strPtr(withCache)},
		{UserId: 1, CreatedAt: now, Type: logstore.LogTypeConsume, Username: "u1", Group: "default", Quota: 1, BillingDetails: strPtr(simpleBillingDetailsJSON(10, 20))},
		{UserId: 1, CreatedAt: now, Type: logstore.LogTypeConsume, Username: "u1", Group: "default", Quota: 1},
	}
	if err := dbstore.LOG_DB.Create(&logs).Error; err != nil {
		t.Fatalf("create logs: %v", err)
	}

	stat, err := logstore.SumUsedQuota(logstore.LogTypeConsume, now-1, now+1, logstore.LogStatFilter{Username: "u1"})
	if err != nil {
		t.Fatalf("SumUsedQuota error = %v", err)
	}
	if stat.Tpm != 1530 {
		t.Fatalf("tpm = %d, want 1530 (1500 + 30, NULL details contribute 0)", stat.Tpm)
	}

	corrupt := &logstore.Log{
		UserId: 1, CreatedAt: now, Type: logstore.LogTypeConsume,
		Username: "u1", Group: "default", Quota: 1,
		BillingDetails: strPtr(`{"schema_version":9,"tokens":{}}`),
	}
	if err := dbstore.LOG_DB.Create(corrupt).Error; err != nil {
		t.Fatalf("create corrupt log: %v", err)
	}
	if _, err := logstore.SumUsedQuota(logstore.LogTypeConsume, now-1, now+1, logstore.LogStatFilter{Username: "u1"}); err == nil {
		t.Fatal("SumUsedQuota must fail explicitly on corrupt billing_details, not treat it as zero usage")
	}
}

// TestQueryRpmTpmSumsBillingDetails 验证 DataExport 模式复用的实时 TPM 查询
// 同样切换到 billing_details 来源。
func TestQueryRpmTpmSumsBillingDetails(t *testing.T) {
	setupLogAdminInfoTestDB(t)

	now := time.Now().Unix()
	logs := []*logstore.Log{
		{UserId: 1, CreatedAt: now, Type: logstore.LogTypeConsume, Group: "default", Quota: 1, BillingDetails: strPtr(simpleBillingDetailsJSON(7, 11))},
		{UserId: 2, CreatedAt: now, Type: logstore.LogTypeConsume, Group: "default", Quota: 1, BillingDetails: strPtr(simpleBillingDetailsJSON(1, 2))},
	}
	if err := dbstore.LOG_DB.Create(&logs).Error; err != nil {
		t.Fatalf("create logs: %v", err)
	}

	rpm, tpm, err := logstore.QueryRpmTpm(logstore.LogStatFilter{})
	if err != nil {
		t.Fatalf("QueryRpmTpm error = %v", err)
	}
	if rpm != 2 {
		t.Fatalf("rpm = %d, want 2", rpm)
	}
	if tpm != 21 {
		t.Fatalf("tpm = %d, want 21", tpm)
	}
}

func strPtr(s string) *string {
	return &s
}
