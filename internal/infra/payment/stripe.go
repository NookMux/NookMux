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
	"github.com/gin-gonic/gin"
	"github.com/stripe/stripe-go/v81"
	"github.com/stripe/stripe-go/v81/checkout/session"
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

	stripe.Key = config.StripeApiSecret

	// Use custom URLs if provided, otherwise use defaults
	if successURL == "" {
		successURL = system.ServerAddress + "/console/log"
	}
	if cancelURL == "" {
		cancelURL = system.ServerAddress + "/console/topup"
	}

	params := &stripe.CheckoutSessionParams{
		ClientReferenceID: stripe.String(referenceId),
		SuccessURL:        stripe.String(successURL),
		CancelURL:         stripe.String(cancelURL),
		LineItems: []*stripe.CheckoutSessionLineItemParams{
			{
				Price:    stripe.String(config.StripePriceId),
				Quantity: stripe.Int64(amount),
			},
		},
		Mode:                stripe.String(string(stripe.CheckoutSessionModePayment)),
		AllowPromotionCodes: stripe.Bool(config.StripePromotionCodesEnabled),
	}

	if "" == customerId {
		if "" != email {
			params.CustomerEmail = stripe.String(email)
		}

		params.CustomerCreation = stripe.String(string(stripe.CheckoutSessionCustomerCreationAlways))
	} else {
		params.Customer = stripe.String(customerId)
	}

	result, err := session.New(params)
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

	err := topupstore.Recharge(referenceId, customerId, callerIp)
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
