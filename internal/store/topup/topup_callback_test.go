package topupstore

import (
	"errors"
	"fmt"
	"github.com/NookMux/NookMux/internal/common"
	"github.com/NookMux/NookMux/internal/infra/redis"
	"github.com/NookMux/NookMux/internal/store/db"
	"github.com/NookMux/NookMux/internal/store/log"
	"github.com/NookMux/NookMux/internal/store/user"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"strings"
	"testing"
	"time"
)

// listTopupLogs 返回当前库中所有充值日志（按写入顺序）。
func listTopupLogs(t *testing.T) []*logstore.Log {
	t.Helper()

	var logs []*logstore.Log
	if err := dbstore.LOG_DB.Where("type = ?", logstore.LogTypeTopup).Order("id asc").Find(&logs).Error; err != nil {
		t.Fatalf("query topup logs: %v", err)
	}
	return logs
}

// assertTopupAuditLog 断言充值日志携带完整审计字段。
func assertTopupAuditLog(t *testing.T, log *logstore.Log, paymentMethod string, callbackPaymentMethod string) {
	t.Helper()

	for _, want := range []string{
		`"admin_info"`,
		`"server_ip"`,
		`"caller_ip":"` + testCallerIp + `"`,
		`"payment_method":"` + paymentMethod + `"`,
		`"callback_payment_method":"` + callbackPaymentMethod + `"`,
		`"version"`,
	} {
		if !strings.Contains(log.Other, want) {
			t.Fatalf("topup log other %s missing %s", log.Other, want)
		}
	}
}

// testCallerIp 充值审计日志测试用的调用方 IP（TEST-NET-2 保留网段）。
const testCallerIp = "198.51.100.7"

func setupTopUpCallbackTestDB(t *testing.T) {
	t.Helper()

	oldDB := dbstore.DB
	oldLogDB := dbstore.LOG_DB
	oldQuotaPerUnit := common.QuotaPerUnit
	oldRedisEnabled := redis.RedisEnabled
	oldMemoryCacheEnabled := common.MemoryCacheEnabled

	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite test db: %v", err)
	}
	if err := db.AutoMigrate(&userstore.User{}, &TopUp{}, &logstore.Log{}); err != nil {
		t.Fatalf("migrate sqlite test db: %v", err)
	}

	dbstore.DB = db
	dbstore.LOG_DB = db
	common.QuotaPerUnit = 100
	redis.RedisEnabled = false
	common.MemoryCacheEnabled = false

	t.Cleanup(func() {
		if sqlDB, err := db.DB(); err == nil {
			_ = sqlDB.Close()
		}
		dbstore.DB = oldDB
		dbstore.LOG_DB = oldLogDB
		common.QuotaPerUnit = oldQuotaPerUnit
		redis.RedisEnabled = oldRedisEnabled
		common.MemoryCacheEnabled = oldMemoryCacheEnabled
	})
}

func createTopUpCallbackUser(t *testing.T, id int) {
	t.Helper()

	user := &userstore.User{
		Id:       id,
		Username: fmt.Sprintf("topup-callback-user-%d", id),
		Status:   common.UserStatusEnabled,
		Quota:    0,
	}
	if err := dbstore.DB.Create(user).Error; err != nil {
		t.Fatalf("create user: %v", err)
	}
}

func createTopUpCallbackTopUp(t *testing.T, tradeNo string, provider string, method string, money float64) {
	t.Helper()

	topUp := &TopUp{
		UserId:          1,
		Amount:          2,
		Money:           money,
		TradeNo:         tradeNo,
		PaymentMethod:   method,
		PaymentProvider: provider,
		CreateTime:      time.Now().Unix(),
		Status:          common.TopUpStatusPending,
	}
	if err := topUp.Insert(); err != nil {
		t.Fatalf("create topup: %v", err)
	}
}

func topUpCallbackStatus(t *testing.T, tradeNo string) string {
	t.Helper()

	topUp, err := GetTopUpByTradeNo(tradeNo)
	if err != nil {
		t.Fatalf("get topup %s: %v", tradeNo, err)
	}
	if topUp == nil {
		t.Fatalf("topup %s not found", tradeNo)
	}
	return topUp.Status
}

// TestRechargeCreditsPendingOrder 验证 Recharge 对 pending 订单成功入账。
func TestRechargeCreditsPendingOrder(t *testing.T) {
	setupTopUpCallbackTestDB(t)
	createTopUpCallbackUser(t, 1)
	createTopUpCallbackTopUp(t, "recharge-ok", PaymentProviderStripe, PaymentMethodStripe, 9.99)

	if err := Recharge("recharge-ok", "cus_test_123", testCallerIp); err != nil {
		t.Fatalf("Recharge error = %v", err)
	}
	if got := topUpCallbackStatus(t, "recharge-ok"); got != common.TopUpStatusSuccess {
		t.Fatalf("topup status = %s, want %s", got, common.TopUpStatusSuccess)
	}
	var user userstore.User
	if err := dbstore.DB.Select("quota", "stripe_customer").Where("id = ?", 1).First(&user).Error; err != nil {
		t.Fatalf("get user: %v", err)
	}
	// Recharge 的入账额度 = Money * QuotaPerUnit = 9.99 * 100。
	if user.Quota != 999 {
		t.Fatalf("user quota = %d, want 999", user.Quota)
	}
	if user.StripeCustomer != "cus_test_123" {
		t.Fatalf("stripe_customer = %q, want %q", user.StripeCustomer, "cus_test_123")
	}
}

// TestRechargeDuplicateReturnsStatusInvalidError 验证对已成功订单再次调用 Recharge
// 返回可被 errors.Is 判断的 ErrTopUpStatusInvalid sentinel error。
// 这是 StripeWebhook 侧区分"重复投递已处理"与"真实入账失败"的依据。
func TestRechargeDuplicateReturnsStatusInvalidError(t *testing.T) {
	setupTopUpCallbackTestDB(t)
	createTopUpCallbackUser(t, 1)
	createTopUpCallbackTopUp(t, "recharge-dup", PaymentProviderStripe, PaymentMethodStripe, 9.99)

	if err := Recharge("recharge-dup", "cus_test_123", testCallerIp); err != nil {
		t.Fatalf("first Recharge error = %v", err)
	}

	err := Recharge("recharge-dup", "cus_test_123", testCallerIp)
	if err == nil {
		t.Fatal("expected error on duplicate Recharge, got nil")
	}
	if !errors.Is(err, ErrTopUpStatusInvalid) {
		t.Fatalf("Recharge error = %v, want ErrTopUpStatusInvalid (errors.Is)", err)
	}

	// 重复投递不得二次加款。
	var user userstore.User
	if err := dbstore.DB.Select("quota").Where("id = ?", 1).First(&user).Error; err != nil {
		t.Fatalf("get user: %v", err)
	}
	if user.Quota != 999 {
		t.Fatalf("user quota = %d, want 999 (no double credit)", user.Quota)
	}
}

// TestCompleteEpayTopUpDuplicateReturnsStatusInvalidError 验证 CompleteEpayTopUp
// 的幂等性：已成功订单再次调用返回 ErrTopUpStatusInvalid，且不重复加款。
func TestCompleteEpayTopUpDuplicateReturnsStatusInvalidError(t *testing.T) {
	setupTopUpCallbackTestDB(t)
	createTopUpCallbackUser(t, 1)
	createTopUpCallbackTopUp(t, "epay-dup", PaymentProviderEpay, "alipay", 9.99)

	if err := CompleteEpayTopUp("epay-dup", "alipay", "9.99", testCallerIp); err != nil {
		t.Fatalf("first CompleteEpayTopUp error = %v", err)
	}

	err := CompleteEpayTopUp("epay-dup", "alipay", "9.99", testCallerIp)
	if err == nil {
		t.Fatal("expected error on duplicate CompleteEpayTopUp, got nil")
	}
	if !errors.Is(err, ErrTopUpStatusInvalid) {
		t.Fatalf("CompleteEpayTopUp error = %v, want ErrTopUpStatusInvalid (errors.Is)", err)
	}

	var user userstore.User
	if err := dbstore.DB.Select("quota").Where("id = ?", 1).First(&user).Error; err != nil {
		t.Fatalf("get user: %v", err)
	}
	// CompleteEpayTopUp 的入账额度 = Amount * QuotaPerUnit = 2 * 100。
	if user.Quota != 200 {
		t.Fatalf("user quota = %d, want 200 (no double credit)", user.Quota)
	}
}

// TestRechargeRecordsTopupAuditLog Stripe 回调入账成功后，充值日志必须带上
// 回调 IP、订单支付方式、回调来源与系统版本等审计字段。
func TestRechargeRecordsTopupAuditLog(t *testing.T) {
	setupTopUpCallbackTestDB(t)
	createTopUpCallbackUser(t, 1)
	createTopUpCallbackTopUp(t, "recharge-audit", PaymentProviderStripe, PaymentMethodStripe, 9.99)

	if err := Recharge("recharge-audit", "cus_test_123", testCallerIp); err != nil {
		t.Fatalf("Recharge error = %v", err)
	}

	logs := listTopupLogs(t)
	if len(logs) != 1 {
		t.Fatalf("log count = %d, want 1", len(logs))
	}
	assertTopupAuditLog(t, logs[0], PaymentMethodStripe, PaymentProviderStripe)
}

// TestCompleteEpayTopUpRecordsTopupAuditLog 易支付回调入账成功后，日志需记录
// 订单支付方式与 epay 回调来源。
func TestCompleteEpayTopUpRecordsTopupAuditLog(t *testing.T) {
	setupTopUpCallbackTestDB(t)
	createTopUpCallbackUser(t, 1)
	createTopUpCallbackTopUp(t, "epay-audit", PaymentProviderEpay, "alipay", 9.99)

	if err := CompleteEpayTopUp("epay-audit", "alipay", "9.99", testCallerIp); err != nil {
		t.Fatalf("CompleteEpayTopUp error = %v", err)
	}

	logs := listTopupLogs(t)
	if len(logs) != 1 {
		t.Fatalf("log count = %d, want 1", len(logs))
	}
	assertTopupAuditLog(t, logs[0], "alipay", PaymentProviderEpay)
}

// TestManualCompleteTopUpRecordsTopupAuditLog 补单成功写入带审计信息的日志；
// 幂等分支（订单此前已成功）不产生新的充值日志。
func TestManualCompleteTopUpRecordsTopupAuditLog(t *testing.T) {
	setupTopUpCallbackTestDB(t)
	createTopUpCallbackUser(t, 1)
	createTopUpCallbackTopUp(t, "manual-audit", PaymentProviderEpay, "alipay", 9.99)

	if err := ManualCompleteTopUp("manual-audit", testCallerIp); err != nil {
		t.Fatalf("ManualCompleteTopUp error = %v", err)
	}

	logs := listTopupLogs(t)
	if len(logs) != 1 {
		t.Fatalf("log count = %d, want 1", len(logs))
	}
	assertTopupAuditLog(t, logs[0], "alipay", CallbackPaymentMethodAdmin)

	if err := ManualCompleteTopUp("manual-audit", testCallerIp); err != nil {
		t.Fatalf("idempotent ManualCompleteTopUp error = %v", err)
	}
	if logs := listTopupLogs(t); len(logs) != 1 {
		t.Fatalf("log count after idempotent manual complete = %d, want 1", len(logs))
	}
}
