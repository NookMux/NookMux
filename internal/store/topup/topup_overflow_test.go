package topupstore

import (
	"strconv"
	"testing"
	"time"

	"github.com/NookMux/NookMux/internal/common"
	"github.com/NookMux/NookMux/internal/store/db"
	"github.com/NookMux/NookMux/internal/store/user"
)

// 32 位构建额度回绕回归测试（docs/差异性/安全性.md `47ba9d2c6` 第 4 点 /
// `2a0ce3475`）：$8599 × QuotaPerUnit(500000) = 4,299,500,000，直接
// int(IntPart()) 在 int 为 32 位的构建（GOARCH=386/arm）上按低 32 位回绕成
// 4,532,704——旧实现静默少入账并把订单标记成功。加固后必须显式报错回滚：
// 订单保持 pending、额度不变。64 位构建上该数值合法，应正常入账。

func createOverflowTopUp(t *testing.T, tradeNo string, provider string, method string, money float64, amount int64) {
	t.Helper()

	topUp := &TopUp{
		UserId:          1,
		Amount:          amount,
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

func assertTopUpUserQuota(t *testing.T, want int64) {
	t.Helper()

	var u userstore.User
	if err := dbstore.DB.Select("quota").Where("id = ?", 1).First(&u).Error; err != nil {
		t.Fatalf("get user: %v", err)
	}
	if int64(u.Quota) != want {
		t.Fatalf("user quota = %d, want %d", u.Quota, want)
	}
}

func TestCompleteEpayTopUpQuotaOverflowFailsClosed(t *testing.T) {
	setupTopUpCallbackTestDB(t)
	common.QuotaPerUnit = 500000
	createTopUpCallbackUser(t, 1)
	createOverflowTopUp(t, "epay-wrap", PaymentProviderEpay, "alipay", 8599.0, 8599)

	err := CompleteEpayTopUp("epay-wrap", "alipay", "8599", testCallerIp)
	if strconv.IntSize == 32 {
		if err == nil {
			t.Fatal("CompleteEpayTopUp succeeded on 32-bit build with wrapped quota 4532704, want explicit error")
		}
		if got := topUpCallbackStatus(t, "epay-wrap"); got != common.TopUpStatusPending {
			t.Fatalf("topup status = %s, want %s (fail closed, transaction rolled back)", got, common.TopUpStatusPending)
		}
		assertTopUpUserQuota(t, 0)
		return
	}
	if err != nil {
		t.Fatalf("CompleteEpayTopUp error = %v on 64-bit build", err)
	}
	if got := topUpCallbackStatus(t, "epay-wrap"); got != common.TopUpStatusSuccess {
		t.Fatalf("topup status = %s, want %s", got, common.TopUpStatusSuccess)
	}
	assertTopUpUserQuota(t, 4299500000)
}

func TestManualCompleteTopUpQuotaOverflowFailsClosed(t *testing.T) {
	setupTopUpCallbackTestDB(t)
	common.QuotaPerUnit = 500000
	createTopUpCallbackUser(t, 1)
	// 易支付分支：Amount × QuotaPerUnit
	createOverflowTopUp(t, "manual-wrap-epay", PaymentProviderEpay, "alipay", 8599.0, 8599)

	err := ManualCompleteTopUp("manual-wrap-epay", testCallerIp)
	if strconv.IntSize == 32 {
		if err == nil {
			t.Fatal("ManualCompleteTopUp succeeded on 32-bit build with wrapped quota 4532704, want explicit error")
		}
		if got := topUpCallbackStatus(t, "manual-wrap-epay"); got != common.TopUpStatusPending {
			t.Fatalf("topup status = %s, want %s (fail closed, transaction rolled back)", got, common.TopUpStatusPending)
		}
		assertTopUpUserQuota(t, 0)
		return
	}
	if err != nil {
		t.Fatalf("ManualCompleteTopUp error = %v on 64-bit build", err)
	}
	if got := topUpCallbackStatus(t, "manual-wrap-epay"); got != common.TopUpStatusSuccess {
		t.Fatalf("topup status = %s, want %s", got, common.TopUpStatusSuccess)
	}
	assertTopUpUserQuota(t, 4299500000)
}

func TestManualCompleteTopUpStripeQuotaOverflowFailsClosed(t *testing.T) {
	setupTopUpCallbackTestDB(t)
	common.QuotaPerUnit = 500000
	createTopUpCallbackUser(t, 1)
	// Stripe 分支：Money × QuotaPerUnit（Amount 不参与换算）
	createOverflowTopUp(t, "manual-wrap-stripe", PaymentProviderStripe, PaymentMethodStripe, 8599.0, 0)

	err := ManualCompleteTopUp("manual-wrap-stripe", testCallerIp)
	if strconv.IntSize == 32 {
		if err == nil {
			t.Fatal("ManualCompleteTopUp(stripe) succeeded on 32-bit build with wrapped quota 4532704, want explicit error")
		}
		if got := topUpCallbackStatus(t, "manual-wrap-stripe"); got != common.TopUpStatusPending {
			t.Fatalf("topup status = %s, want %s (fail closed, transaction rolled back)", got, common.TopUpStatusPending)
		}
		assertTopUpUserQuota(t, 0)
		return
	}
	if err != nil {
		t.Fatalf("ManualCompleteTopUp(stripe) error = %v on 64-bit build", err)
	}
	if got := topUpCallbackStatus(t, "manual-wrap-stripe"); got != common.TopUpStatusSuccess {
		t.Fatalf("topup status = %s, want %s", got, common.TopUpStatusSuccess)
	}
	assertTopUpUserQuota(t, 4299500000)
}

// TestRechargeQuotaOverflowFailsClosedOn32Bit Stripe 自动入账路径：旧实现把
// float64 直传 SQL，32 位构建下虽能写入 4,299,500,000（SQLite 列是 64 位），
// 但后续任何把 quota 读回 Go int 的查询都会 Scan 越界报错（账户被写坏），
// 日志里的 int(quota) 也是实现自定义的垃圾值。加固后超界直接报错回滚。
func TestRechargeQuotaOverflowFailsClosedOn32Bit(t *testing.T) {
	setupTopUpCallbackTestDB(t)
	common.QuotaPerUnit = 500000
	createTopUpCallbackUser(t, 1)
	createOverflowTopUp(t, "recharge-wrap", PaymentProviderStripe, PaymentMethodStripe, 8599.0, 0)

	err := Recharge("recharge-wrap", "cus_overflow", testCallerIp)
	if strconv.IntSize == 32 {
		if err == nil {
			t.Fatal("Recharge succeeded on 32-bit build and wrote quota beyond int range (account poisoned for later reads), want explicit error")
		}
		if got := topUpCallbackStatus(t, "recharge-wrap"); got != common.TopUpStatusPending {
			t.Fatalf("topup status = %s, want %s (fail closed, transaction rolled back)", got, common.TopUpStatusPending)
		}
		assertTopUpUserQuota(t, 0)
		return
	}
	if err != nil {
		t.Fatalf("Recharge error = %v on 64-bit build", err)
	}
	if got := topUpCallbackStatus(t, "recharge-wrap"); got != common.TopUpStatusSuccess {
		t.Fatalf("topup status = %s, want %s", got, common.TopUpStatusSuccess)
	}
	assertTopUpUserQuota(t, 4299500000)
}
