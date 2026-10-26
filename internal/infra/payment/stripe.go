package payment

import (
	"errors"
	"fmt"
	"log"
	"strconv"
	"strings"

	"github.com/NookMux/NookMux/internal/common"
	"github.com/NookMux/NookMux/internal/config"
	"github.com/NookMux/NookMux/internal/config/operation"
	"github.com/NookMux/NookMux/internal/config/system"
	"github.com/NookMux/NookMux/internal/i18n"
	topupstore "github.com/NookMux/NookMux/internal/store/topup"
	userstore "github.com/NookMux/NookMux/internal/store/user"
	"github.com/NookMux/NookMux/pkg/jsonx"
	"github.com/gin-gonic/gin"
	"github.com/shopspring/decimal"
	"github.com/stripe/stripe-go/v86"
	"github.com/thanhpk/randstr"
)

const (
	PaymentMethodStripe = "stripe"
)

func GetChargedAmount(count float64, user userstore.User) float64 {
	topUpGroupRatio := common.GetTopupGroupRatio(user.Group)
	if topUpGroupRatio == 0 {
		topUpGroupRatio = 1
	}

	return count * topUpGroupRatio
}

func GetStripePayMoney(amount float64, group string) float64 {
	originalAmount := amount
	// Using float64 for monetary calculations is acceptable here due to the small amounts involved
	topupGroupRatio := common.GetTopupGroupRatio(group)
	if topupGroupRatio == 0 {
		topupGroupRatio = 1
	}
	// apply optional preset discount by the original request amount (if configured), default 1.0
	discount := 1.0
	if ds, ok := operation.GetPaymentSetting().AmountDiscount[int(originalAmount)]; ok {
		if ds > 0 {
			discount = ds
		}
	}
	payMoney := amount * config.StripeUnitPrice * topupGroupRatio * discount
	return payMoney
}

func GetStripeMinTopup() int64 {
	return int64(config.StripeMinTopUp)
}

// stripeZeroDecimalCurrencies 为 Stripe 以主单位直接计量的零小数币种，
// 其余币种的金额字段均以 1/100 主单位表示。
var stripeZeroDecimalCurrencies = map[string]struct{}{
	"bif": {}, "clp": {}, "djf": {}, "gnf": {}, "jpy": {}, "krw": {},
	"mga": {}, "pyg": {}, "vnd": {}, "vuv": {}, "xaf": {}, "xof": {}, "xpf": {},
}

// VerifySessionAmount 在入账前核对 webhook 事件的实付金额与本地单价配置。
// 预期最小单位金额 = quantity × StripeUnitPrice × 币种倍率（零小数币种为 1，其余为 100），
// 促销码折扣以 total_details.amount_discount 找平。Stripe Price 单价与
// StripeUnitPrice 配置漂移时直接返回错误，由 webhook 返回 5xx 暴露问题，不做兜底入账。
func VerifySessionAmount(quantity int64, unitPrice float64, currency string, amountTotal string, amountDiscount string) error {
	if quantity <= 0 {
		return fmt.Errorf("stripe 充值订单数量非法: %d", quantity)
	}
	total, err := strconv.ParseInt(amountTotal, 10, 64)
	if err != nil {
		return fmt.Errorf("stripe webhook 实付金额缺失或非法 amount_total=%q: %w", amountTotal, err)
	}
	var discount int64
	if amountDiscount != "" {
		discount, err = strconv.ParseInt(amountDiscount, 10, 64)
		if err != nil {
			return fmt.Errorf("stripe webhook 促销折扣金额非法 amount_discount=%q: %w", amountDiscount, err)
		}
	}
	scale := decimal.NewFromInt(100)
	if _, ok := stripeZeroDecimalCurrencies[strings.ToLower(currency)]; ok {
		scale = decimal.NewFromInt(1)
	}
	expected := decimal.NewFromFloat(unitPrice).Mul(decimal.NewFromInt(quantity)).Mul(scale)
	if !expected.IsInteger() {
		return fmt.Errorf("stripe 预期实付金额非整数最小单位（unit_price=%.4f, quantity=%d, currency=%s）", unitPrice, quantity, currency)
	}
	expectedMinor := expected.IntPart()
	if total+discount != expectedMinor {
		return fmt.Errorf("stripe 实付金额与本地单价配置不一致: 实付 %d + 折扣 %d != 预期 %d（currency=%s, quantity=%d, unit_price=%.4f）", total, discount, expectedMinor, currency, quantity, unitPrice)
	}
	return nil
}

// amountDiscountOf 安全提取事件中的促销折扣金额（最小货币单位）。
// stripe-go 的 GetObjectValue 对双层路径的首键缺失或为 null 时会 panic
// （"Cannot descend into non-map non-slice object"），而真实无折扣的
// Checkout 事件正是 total_details 缺失/null 形态，故不得直调。
// total_details 缺失、amount_discount 为 null 或非数字时返回空串，
// 语义为"无折扣"，由 VerifySessionAmount 按零折扣核对。
func amountDiscountOf(event stripe.Event) string {
	var obj map[string]any
	if err := jsonx.Unmarshal(event.Data.Raw, &obj); err != nil {
		return ""
	}
	details, ok := obj["total_details"].(map[string]any)
	if !ok {
		return ""
	}
	if v, ok := details["amount_discount"].(float64); ok {
		return strconv.FormatInt(int64(v), 10)
	}
	return ""
}

// GenStripeLink generates a Stripe Checkout session URL for payment.
// It creates a new checkout session with the specified parameters and returns the payment URL.
//
// Parameters:
//   - c: gin context（用于 API Key 非法时的本地化错误文案）
//   - referenceId: unique reference identifier for the transaction
//   - customerId: existing Stripe customer ID (empty string if new customer)
//   - email: customer email address for new customer creation
//   - amount: quantity of units to purchase
//   - successURL: custom URL to redirect after successful payment (empty for default)
//   - cancelURL: custom URL to redirect when payment is canceled (empty for default)
//
// Returns the checkout session URL or an error if the session creation fails.
func GenStripeLink(c *gin.Context, referenceId string, customerId string, email string, amount int64, successURL string, cancelURL string) (string, error) {
	if !strings.HasPrefix(config.StripeApiSecret, "sk_") && !strings.HasPrefix(config.StripeApiSecret, "rk_") {
		return "", fmt.Errorf("%s", i18n.T(c, i18n.MsgTopupStripeInvalidAPIKey))
	}

	sc := stripe.NewClient(config.StripeApiSecret)

	// Use custom URLs if provided, otherwise use defaults
	if successURL == "" {
		successURL = system.ServerAddress + "/console/log"
	}
	if cancelURL == "" {
		cancelURL = system.ServerAddress + "/console/topup"
	}

	params := &stripe.CheckoutSessionCreateParams{
		ClientReferenceID: stripe.String(referenceId),
		SuccessURL:        stripe.String(successURL),
		CancelURL:         stripe.String(cancelURL),
		LineItems: []*stripe.CheckoutSessionCreateLineItemParams{
			{
				Price:    stripe.String(config.StripePriceId),
				Quantity: stripe.Int64(amount),
			},
		},
		Mode:                  stripe.String(string(stripe.CheckoutSessionModePayment)),
		AllowPromotionCodes:   stripe.Bool(config.StripePromotionCodesEnabled),
		IntegrationIdentifier: stripe.String("nookmux-topup-" + randstr.String(8)),
	}

	if "" == customerId {
		if "" != email {
			params.CustomerEmail = stripe.String(email)
		}

		params.CustomerCreation = stripe.String(string(stripe.CheckoutSessionCustomerCreationAlways))
	} else {
		params.Customer = stripe.String(customerId)
	}

	result, err := sc.V1CheckoutSessions.Create(c.Request.Context(), params)
	if err != nil {
		return "", err
	}

	return result.URL, nil
}

func SessionCompleted(event stripe.Event, callerIp string) error {
	customerId := event.GetObjectValue("customer")
	referenceId := event.GetObjectValue("client_reference_id")
	status := event.GetObjectValue("status")
	if "complete" != status {
		log.Println("错误的Stripe Checkout完成状态:", status, ",", referenceId)
		return nil
	}

	paymentStatus := event.GetObjectValue("payment_status")
	if paymentStatus != "paid" {
		log.Printf("Stripe Checkout 支付尚未完成，payment_status: %s, ref: %s（等待异步支付结果）", paymentStatus, referenceId)
		return nil
	}

	return FulfillOrder(event, referenceId, customerId, callerIp)
}

// SessionAsyncPaymentSucceeded handles delayed payment methods (bank transfer, SEPA, etc.)
// that confirm payment after the checkout session completes.
func SessionAsyncPaymentSucceeded(event stripe.Event, callerIp string) error {
	customerId := event.GetObjectValue("customer")
	referenceId := event.GetObjectValue("client_reference_id")
	log.Printf("Stripe 异步支付成功: %s", referenceId)

	return FulfillOrder(event, referenceId, customerId, callerIp)
}

// SessionAsyncPaymentFailed marks orders as failed when delayed payment methods
// ultimately fail (e.g. bank transfer not received, SEPA rejected).
func SessionAsyncPaymentFailed(event stripe.Event) error {
	referenceId := event.GetObjectValue("client_reference_id")
	log.Printf("Stripe 异步支付失败: %s", referenceId)

	if len(referenceId) == 0 {
		log.Println("异步支付失败事件未提供支付单号")
		return nil
	}

	LockOrder(referenceId)
	defer UnlockOrder(referenceId)

	if err := topupstore.UpdatePendingTopUpStatus(referenceId, topupstore.PaymentProviderStripe, common.TopUpStatusFailed); err != nil {
		// 订单已非 pending（重复投递且此前已处理），视为已处理返回 nil。
		if errors.Is(err, topupstore.ErrTopUpStatusInvalid) {
			log.Println("充值订单已处理，跳过失败标记:", referenceId)
			return nil
		}
		log.Printf("标记充值订单失败出错: %v, ref: %s", err, referenceId)
		return err
	}
	log.Printf("充值订单已标记为失败: %s", referenceId)
	return nil
}

// FulfillOrder is the shared logic for crediting quota after payment is confirmed.
// 入账失败时向上返回 error，由 StripeWebhook 返回 5xx 触发 Stripe 重投；
// 订单已处理（ErrTopUpStatusInvalid，重复投递）视为成功返回 nil。
func FulfillOrder(event stripe.Event, referenceId string, customerId string, callerIp string) error {
	if len(referenceId) == 0 {
		log.Println("未提供支付单号")
		return nil
	}

	LockOrder(referenceId)
	defer UnlockOrder(referenceId)

	topUp, err := topupstore.GetTopUpByTradeNo(referenceId)
	if err != nil {
		log.Println("查询充值订单出错:", referenceId, ",", err.Error())
		return err
	}
	if topUp == nil {
		// 订单本地不存在是终态：重投也无法入账，返回 nil 让 webhook 回 2xx，
		// 避免 Stripe 无限重投。
		log.Println("充值订单不存在，视为终态跳过入账:", referenceId)
		return nil
	}

	// 入账前核对实付金额：Stripe Price 单价与 StripeUnitPrice 配置漂移时
	// 拒绝入账并返回错误，由 webhook 返回 5xx 触发重投与告警。
	if err := VerifySessionAmount(
		topUp.Amount,
		config.StripeUnitPrice,
		event.GetObjectValue("currency"),
		event.GetObjectValue("amount_total"),
		amountDiscountOf(event),
	); err != nil {
		log.Println("Stripe 实付金额核对失败:", referenceId, ",", err.Error())
		return err
	}

	err = topupstore.Recharge(referenceId, customerId, callerIp)
	if err != nil {
		if errors.Is(err, topupstore.ErrTopUpStatusInvalid) {
			log.Println("充值订单已处理，跳过重复入账:", referenceId)
			return nil
		}
		// 订单本地不存在是终态：重投也无法入账，返回 nil 让 webhook 回 2xx，
		// 避免 Stripe 无限重投；其余错误（数据库故障等）保持 5xx 触发重试。
		if errors.Is(err, topupstore.ErrTopUpNotFound) {
			log.Println("充值订单不存在，视为终态跳过入账:", referenceId)
			return nil
		}
		log.Println(err.Error(), referenceId)
		return err
	}

	total, _ := strconv.ParseFloat(event.GetObjectValue("amount_total"), 64)
	currency := strings.ToUpper(event.GetObjectValue("currency"))
	log.Printf("收到款项：%s, %.2f(%s)", referenceId, total/100, currency)
	return nil
}

func SessionExpired(event stripe.Event) error {
	referenceId := event.GetObjectValue("client_reference_id")
	status := event.GetObjectValue("status")
	if "expired" != status {
		log.Println("错误的Stripe Checkout过期状态:", status, ",", referenceId)
		return nil
	}

	if len(referenceId) == 0 {
		log.Println("未提供支付单号")
		return nil
	}

	LockOrder(referenceId)
	defer UnlockOrder(referenceId)

	if err := topupstore.UpdatePendingTopUpStatus(referenceId, topupstore.PaymentProviderStripe, common.TopUpStatusExpired); err != nil {
		// 订单已非 pending（重复投递且此前已处理），视为已处理返回 nil。
		if errors.Is(err, topupstore.ErrTopUpStatusInvalid) {
			log.Println("充值订单已处理，跳过过期标记:", referenceId)
			return nil
		}
		log.Println("过期充值订单失败", referenceId, ", err:", err.Error())
		return err
	}

	log.Println("充值订单已过期", referenceId)
	return nil
}
