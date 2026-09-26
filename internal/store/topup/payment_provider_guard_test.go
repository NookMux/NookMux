package topupstore

import (
	"fmt"
	"github.com/NookMux/NookMux/internal/common"
	"github.com/NookMux/NookMux/internal/infra/redis"
	"github.com/NookMux/NookMux/internal/store/db"
	"github.com/NookMux/NookMux/internal/store/log"
	"github.com/NookMux/NookMux/internal/store/user"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"testing"
	"time"
)

func setupPaymentProviderGuardTestDB(t *testing.T) {
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

func createPaymentProviderGuardUser(t *testing.T, id int) {
	t.Helper()

	user := &userstore.User{
		Id:       id,
		Username: fmt.Sprintf("payment-provider-user-%d", id),
		Status:   common.UserStatusEnabled,
		Quota:    0,
	}
	if err := dbstore.DB.Create(user).Error; err != nil {
		t.Fatalf("create user: %v", err)
	}
}

func createPaymentProviderGuardTopUp(t *testing.T, tradeNo string, provider string, method string, money float64) {
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

func paymentProviderGuardTopUpStatus(t *testing.T, tradeNo string) string {
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

func paymentProviderGuardUserQuota(t *testing.T, userId int) int {
	t.Helper()

	var user userstore.User
	if err := dbstore.DB.Select("quota").Where("id = ?", userId).First(&user).Error; err != nil {
		t.Fatalf("get user quota: %v", err)
	}
	return user.Quota
}

func TestCompleteEpayTopUpRejectsMismatchedProvider(t *testing.T) {
	setupPaymentProviderGuardTestDB(t)
	createPaymentProviderGuardUser(t, 1)
	createPaymentProviderGuardTopUp(t, "provider-mismatch", PaymentProviderStripe, PaymentMethodStripe, 9.99)

	err := CompleteEpayTopUp("provider-mismatch", "alipay", "9.99", testCallerIp)
	if err != ErrPaymentProviderMismatch {
		t.Fatalf("CompleteEpayTopUp error = %v, want %v", err, ErrPaymentProviderMismatch)
	}
	if got := paymentProviderGuardTopUpStatus(t, "provider-mismatch"); got != common.TopUpStatusPending {
		t.Fatalf("topup status = %s, want %s", got, common.TopUpStatusPending)
	}
	if got := paymentProviderGuardUserQuota(t, 1); got != 0 {
		t.Fatalf("user quota = %d, want 0", got)
	}
}

func TestCompleteEpayTopUpRejectsMismatchedMethod(t *testing.T) {
	setupPaymentProviderGuardTestDB(t)
	createPaymentProviderGuardUser(t, 1)
	createPaymentProviderGuardTopUp(t, "method-mismatch", PaymentProviderEpay, "wechat", 9.99)

	err := CompleteEpayTopUp("method-mismatch", "alipay", "9.99", testCallerIp)
	if err != ErrPaymentMethodMismatch {
		t.Fatalf("CompleteEpayTopUp error = %v, want %v", err, ErrPaymentMethodMismatch)
	}
	if got := paymentProviderGuardUserQuota(t, 1); got != 0 {
		t.Fatalf("user quota = %d, want 0", got)
	}
}

func TestCompleteEpayTopUpRejectsMismatchedAmount(t *testing.T) {
	setupPaymentProviderGuardTestDB(t)
	createPaymentProviderGuardUser(t, 1)
	createPaymentProviderGuardTopUp(t, "amount-mismatch", PaymentProviderEpay, "alipay", 9.99)

	err := CompleteEpayTopUp("amount-mismatch", "alipay", "8.99", testCallerIp)
	if err != ErrPaymentAmountMismatch {
		t.Fatalf("CompleteEpayTopUp error = %v, want %v", err, ErrPaymentAmountMismatch)
	}
	if got := paymentProviderGuardUserQuota(t, 1); got != 0 {
		t.Fatalf("user quota = %d, want 0", got)
	}
}

func TestCompleteEpayTopUpCreditsMatchingOrder(t *testing.T) {
	setupPaymentProviderGuardTestDB(t)
	createPaymentProviderGuardUser(t, 1)
	createPaymentProviderGuardTopUp(t, "matching-epay", PaymentProviderEpay, "alipay", 9.99)

	if err := CompleteEpayTopUp("matching-epay", "alipay", "9.99", testCallerIp); err != nil {
		t.Fatalf("CompleteEpayTopUp error = %v", err)
	}
	if got := paymentProviderGuardTopUpStatus(t, "matching-epay"); got != common.TopUpStatusSuccess {
		t.Fatalf("topup status = %s, want %s", got, common.TopUpStatusSuccess)
	}
	if got := paymentProviderGuardUserQuota(t, 1); got != 200 {
		t.Fatalf("user quota = %d, want 200", got)
	}
}

func TestUpdatePendingTopUpStatusRejectsMismatchedProvider(t *testing.T) {
	setupPaymentProviderGuardTestDB(t)
	createPaymentProviderGuardUser(t, 1)
	createPaymentProviderGuardTopUp(t, "stripe-expire", PaymentProviderEpay, "alipay", 9.99)

	err := UpdatePendingTopUpStatus("stripe-expire", PaymentProviderStripe, common.TopUpStatusExpired)
	if err != ErrPaymentProviderMismatch {
		t.Fatalf("UpdatePendingTopUpStatus error = %v, want %v", err, ErrPaymentProviderMismatch)
	}
	if got := paymentProviderGuardTopUpStatus(t, "stripe-expire"); got != common.TopUpStatusPending {
		t.Fatalf("topup status = %s, want %s", got, common.TopUpStatusPending)
	}
}
