package topupcontroller

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/NookMux/NookMux/internal/common"
	"github.com/NookMux/NookMux/internal/config"
	"github.com/NookMux/NookMux/internal/i18n"
	payment "github.com/NookMux/NookMux/internal/infra/payment"
	"github.com/NookMux/NookMux/internal/infra/security"
	topupstore "github.com/NookMux/NookMux/internal/store/topup"
	userstore "github.com/NookMux/NookMux/internal/store/user"
	"github.com/gin-gonic/gin"
	"github.com/stripe/stripe-go/v81"
	"github.com/stripe/stripe-go/v81/webhook"
	"github.com/thanhpk/randstr"
)

var stripeAdaptor = &StripeAdaptor{}

// StripePayRequest represents a payment request for Stripe checkout.
type StripePayRequest struct {
	// Amount is the quantity of units to purchase.
	Amount int64 `json:"amount"`
	// PaymentMethod specifies the payment method (e.g., "stripe").
	PaymentMethod string `json:"payment_method"`
	// SuccessURL is the optional custom URL to redirect after successful payment.
	// If empty, defaults to the server's console log page.
	SuccessURL string `json:"success_url,omitempty"`
	// CancelURL is the optional custom URL to redirect when payment is canceled.
	// If empty, defaults to the server's console topup page.
	CancelURL string `json:"cancel_url,omitempty"`
}

type StripeAdaptor struct {
}

func (*StripeAdaptor) RequestAmount(c *gin.Context, req *StripePayRequest) {
	if req.Amount < payment.GetStripeMinTopup() {
		respondTopupError(c, i18n.MsgTopupAmountBelowMin, map[string]any{"Min": payment.GetStripeMinTopup()})
		return
	}
	id := c.GetInt("id")
	group, err := userstore.GetUserGroup(id, true)
	if err != nil {
		respondTopupError(c, i18n.MsgTopupGetGroupFailed)
		return
	}
	payMoney := payment.GetStripePayMoney(float64(req.Amount), group)
	if payMoney <= 0.01 {
		respondTopupError(c, i18n.MsgTopupPayAmountTooLow)
		return
	}
	respondTopupSuccess(c, strconv.FormatFloat(payMoney, 'f', 2, 64), "")
}

func (*StripeAdaptor) RequestPay(c *gin.Context, req *StripePayRequest) {
	if req.PaymentMethod != payment.PaymentMethodStripe {
		respondTopupError(c, i18n.MsgTopupUnsupportedChannel)
		return
	}
	if req.Amount < payment.GetStripeMinTopup() {
		respondTopupError(c, i18n.MsgTopupAmountBelowMin, map[string]any{"Min": payment.GetStripeMinTopup()})
		return
	}
	if req.Amount > 10000 {
		respondTopupError(c, i18n.MsgTopupAmountExceedMax)
		return
	}

	if req.SuccessURL != "" && security.ValidateRedirectURL(req.SuccessURL) != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": i18n.T(c, i18n.MsgTopupSuccessUrlUntrusted), "data": nil})
		return
	}

	if req.CancelURL != "" && security.ValidateRedirectURL(req.CancelURL) != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": i18n.T(c, i18n.MsgTopupCancelUrlUntrusted), "data": nil})
		return
	}

	id := c.GetInt("id")
	user, _ := userstore.GetUserById(id, false)
	chargedMoney := payment.GetChargedAmount(float64(req.Amount), *user)

	reference := fmt.Sprintf("nookmux-ref-%d-%d-%s", user.Id, time.Now().UnixMilli(), randstr.String(4))
	referenceId := "ref_" + common.Sha1([]byte(reference))

	payLink, err := payment.GenStripeLink(c, referenceId, user.StripeCustomer, user.Email, req.Amount, req.SuccessURL, req.CancelURL)
	if err != nil {
		log.Println("获取Stripe Checkout支付链接失败", err)
		respondTopupError(c, i18n.MsgTopupPaymentInitFailed)
		return
	}

	topUp := &topupstore.TopUp{
		UserId:          id,
		Amount:          req.Amount,
		Money:           chargedMoney,
		TradeNo:         referenceId,
		PaymentMethod:   topupstore.PaymentMethodStripe,
		PaymentProvider: topupstore.PaymentProviderStripe,
		CreateTime:      time.Now().Unix(),
		Status:          common.TopUpStatusPending,
	}
	err = topUp.Insert()
	if err != nil {
		respondTopupError(c, i18n.MsgTopupOrderCreateFailed)
		return
	}
	respondTopupSuccess(c, gin.H{"pay_link": payLink}, "")
}

func RequestStripeAmount(c *gin.Context) {
	var req StripePayRequest
	err := c.ShouldBindJSON(&req)
	if err != nil {
		respondTopupError(c, i18n.MsgTopupInvalidParams)
		return
	}
	stripeAdaptor.RequestAmount(c, &req)
}

func RequestStripePay(c *gin.Context) {
	var req StripePayRequest
	err := c.ShouldBindJSON(&req)
	if err != nil {
		respondTopupError(c, i18n.MsgTopupInvalidParams)
		return
	}
	stripeAdaptor.RequestPay(c, &req)
}

func StripeWebhook(c *gin.Context) {
	if config.StripeWebhookSecret == "" {
		log.Println("Stripe Webhook Secret 未配置，拒绝处理")
		c.AbortWithStatus(http.StatusForbidden)
		return
	}

	payload, err := io.ReadAll(c.Request.Body)
	if err != nil {
		log.Printf("解析Stripe Webhook参数失败: %v\n", err)
		c.AbortWithStatus(http.StatusServiceUnavailable)
		return
	}

	signature := c.GetHeader("Stripe-Signature")
	event, err := webhook.ConstructEventWithOptions(payload, signature, config.StripeWebhookSecret, webhook.ConstructEventOptions{
		IgnoreAPIVersionMismatch: true,
	})

	if err != nil {
		log.Printf("Stripe Webhook验签失败: %v\n", err)
		c.AbortWithStatus(http.StatusBadRequest)
		return
	}

	var handlerErr error
	switch event.Type {
	case stripe.EventTypeCheckoutSessionCompleted:
		handlerErr = payment.SessionCompleted(event, c.ClientIP())
	case stripe.EventTypeCheckoutSessionExpired:
		handlerErr = payment.SessionExpired(event)
	case stripe.EventTypeCheckoutSessionAsyncPaymentSucceeded:
		handlerErr = payment.SessionAsyncPaymentSucceeded(event, c.ClientIP())
	case stripe.EventTypeCheckoutSessionAsyncPaymentFailed:
		handlerErr = payment.SessionAsyncPaymentFailed(event)
	default:
		log.Printf("不支持的Stripe Webhook事件类型: %s\n", event.Type)
	}

	// 处理失败时返回 5xx 触发 Stripe 重投，避免"已扣款未到账"。
	if handlerErr != nil {
		log.Printf("Stripe Webhook处理失败，返回500等待重投: %v\n", handlerErr)
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}

	c.Status(http.StatusOK)
}
