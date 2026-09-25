package topupcontroller

import (
	"errors"
	"fmt"
	"github.com/Calcium-Ion/go-epay/epay"
	"github.com/NookMux/NookMux/internal/common"
	"github.com/NookMux/NookMux/internal/config"
	"github.com/NookMux/NookMux/internal/config/operation"
	"github.com/NookMux/NookMux/internal/config/system"
	"github.com/NookMux/NookMux/internal/httpapi"
	"github.com/NookMux/NookMux/internal/i18n"
	payment "github.com/NookMux/NookMux/internal/infra/payment"
	"github.com/NookMux/NookMux/internal/store/topup"
	"github.com/NookMux/NookMux/internal/store/user"
	"github.com/gin-gonic/gin"
	"github.com/samber/lo"
	"github.com/shopspring/decimal"
	"log"
	"net/url"
	"strconv"
	"time"
)

func GetTopUpInfo(c *gin.Context) {
	// 获取支付方式
	payMethods := operation.PayMethods

	// 如果启用了 Stripe 支付，添加到支付方法列表
	if config.StripeApiSecret != "" && config.StripeWebhookSecret != "" && config.StripePriceId != "" {
		// 检查是否已经包含 Stripe
		hasStripe := false
		for _, method := range payMethods {
			if method["type"] == "stripe" {
				hasStripe = true
				break
			}
		}

		if !hasStripe {
			stripeMethod := map[string]string{
				"name":      "Stripe",
				"type":      "stripe",
				"color":     "rgba(var(--semi-purple-5), 1)",
				"min_topup": strconv.Itoa(config.StripeMinTopUp),
			}
			payMethods = append(payMethods, stripeMethod)
		}
	}

	data := gin.H{
		"enable_online_topup": operation.PayAddress != "" && operation.EpayId != "" && operation.EpayKey != "",
		"enable_stripe_topup": config.StripeApiSecret != "" && config.StripeWebhookSecret != "" && config.StripePriceId != "",
		"pay_methods":         payMethods,
		"min_topup":           operation.MinTopUp,
		"stripe_min_topup":    config.StripeMinTopUp,
		"amount_options":      operation.GetPaymentSetting().AmountOptions,
		"discount":            operation.GetPaymentSetting().AmountDiscount,
		"topup_link":          common.TopUpLink,
	}
	httpapi.ApiSuccess(c, data)
}

type EpayRequest struct {
	Amount        int64  `json:"amount"`
	PaymentMethod string `json:"payment_method"`
}

type AmountRequest struct {
	Amount int64 `json:"amount"`
}

func GetEpayClient() *epay.Client {
	if operation.PayAddress == "" || operation.EpayId == "" || operation.EpayKey == "" {
		return nil
	}
	withUrl, err := epay.NewClient(&epay.Config{
		PartnerID: operation.EpayId,
		Key:       operation.EpayKey,
	}, operation.PayAddress)
	if err != nil {
		return nil
	}
	return withUrl
}

func getPayMoney(amount int64, group string) float64 {
	dAmount := decimal.NewFromInt(amount)

	topupGroupRatio := common.GetTopupGroupRatio(group)
	if topupGroupRatio == 0 {
		topupGroupRatio = 1
	}

	dTopupGroupRatio := decimal.NewFromFloat(topupGroupRatio)
	dPrice := decimal.NewFromFloat(operation.Price)
	// apply optional preset discount by the original request amount (if configured), default 1.0
	discount := 1.0
	if ds, ok := operation.GetPaymentSetting().AmountDiscount[int(amount)]; ok {
		if ds > 0 {
			discount = ds
		}
	}
	dDiscount := decimal.NewFromFloat(discount)

	payMoney := dAmount.Mul(dPrice).Mul(dTopupGroupRatio).Mul(dDiscount)

	return payMoney.InexactFloat64()
}

func getMinTopup() int64 {
	return int64(operation.MinTopUp)
}

func respondTopupError(c *gin.Context, key string, args ...map[string]any) {
	c.JSON(200, gin.H{"success": false, "message": i18n.T(c, key, args...), "data": nil})
}

func respondTopupSuccess(c *gin.Context, data any, redirectUrl string) {
	resp := gin.H{"success": true, "message": i18n.T(c, i18n.MsgTopupSuccess), "data": data}
	if redirectUrl != "" {
		resp["url"] = redirectUrl
	}
	c.JSON(200, resp)
}

func RequestEpay(c *gin.Context) {
	var req EpayRequest
	err := c.ShouldBindJSON(&req)
	if err != nil {
		httpapi.ApiErrorI18n(c, i18n.MsgInvalidParams)
		return
	}
	if req.Amount < getMinTopup() {
		respondTopupError(c, i18n.MsgTopupAmountBelowMin, map[string]any{"Min": getMinTopup()})
		return
	}

	id := c.GetInt("id")
	group, err := userstore.GetUserGroup(id, true)
	if err != nil {
		respondTopupError(c, i18n.MsgTopupGetGroupFailed)
		return
	}
	payMoney := getPayMoney(req.Amount, group)
	if payMoney < 0.01 {
		respondTopupError(c, i18n.MsgTopupPayAmountTooLow)
		return
	}

	if !operation.ContainsPayMethod(req.PaymentMethod) {
		respondTopupError(c, i18n.MsgTopupPaymentMethodNotFound)
		return
	}

	callBackAddress := payment.GetCallbackAddress()
	returnUrl, _ := url.Parse(system.ServerAddress + "/console/log")
	notifyUrl, _ := url.Parse(callBackAddress + "/api/user/epay/notify")
	tradeNo := fmt.Sprintf("%s%d", common.GetRandomString(6), time.Now().Unix())
	tradeNo = fmt.Sprintf("USR%dNO%s", id, tradeNo)
	client := GetEpayClient()
	if client == nil {
		respondTopupError(c, i18n.MsgTopupPaymentConfigMissing)
		return
	}
	uri, params, err := client.Purchase(&epay.PurchaseArgs{
		Type:           req.PaymentMethod,
		ServiceTradeNo: tradeNo,
		Name:           fmt.Sprintf("TUC%d", req.Amount),
		Money:          strconv.FormatFloat(payMoney, 'f', 2, 64),
		Device:         epay.PC,
		NotifyUrl:      notifyUrl,
		ReturnUrl:      returnUrl,
	})
	if err != nil {
		respondTopupError(c, i18n.MsgTopupPaymentInitFailed)
		return
	}
	amount := req.Amount
	topUp := &topupstore.TopUp{
		UserId:          id,
		Amount:          amount,
		Money:           payMoney,
		TradeNo:         tradeNo,
		PaymentMethod:   req.PaymentMethod,
		PaymentProvider: topupstore.PaymentProviderEpay,
		CreateTime:      time.Now().Unix(),
		Status:          common.TopUpStatusPending,
	}
	err = topUp.Insert()
	if err != nil {
		respondTopupError(c, i18n.MsgTopupOrderCreateFailed)
		return
	}
	respondTopupSuccess(c, params, uri)
}

// completeEpayTopUpWithLock 执行支付回调与用户查单共用的入账序列：
// 订单锁 + 金额/支付方式校验 + 原子状态翻转。返回 ErrTopUpStatusInvalid
// 表示订单已被并发路径完成入账，调用方应按已支付成功处理。
func completeEpayTopUpWithLock(tradeNo string, paymentMethod string, paidMoney string, callerIp string) error {
	payment.LockOrder(tradeNo)
	defer payment.UnlockOrder(tradeNo)
	return topupstore.CompleteEpayTopUp(tradeNo, paymentMethod, paidMoney, callerIp)
}

// tradeNo lock
func EpayNotify(c *gin.Context) {
	var params map[string]string

	if c.Request.Method == "POST" {
		// POST 请求：从 POST body 解析参数
		if err := c.Request.ParseForm(); err != nil {
			log.Println("易支付回调POST解析失败:", err)
			_, _ = c.Writer.Write([]byte("fail"))
			return
		}
		params = lo.Reduce(lo.Keys(c.Request.PostForm), func(r map[string]string, t string, i int) map[string]string {
			r[t] = c.Request.PostForm.Get(t)
			return r
		}, map[string]string{})
	} else {
		// GET 请求：从 URL Query 解析参数
		params = lo.Reduce(lo.Keys(c.Request.URL.Query()), func(r map[string]string, t string, i int) map[string]string {
			r[t] = c.Request.URL.Query().Get(t)
			return r
		}, map[string]string{})
	}

	if len(params) == 0 {
		log.Println("易支付回调参数为空")
		_, _ = c.Writer.Write([]byte("fail"))
		return
	}
	client := GetEpayClient()
	if client == nil {
		log.Println("易支付回调失败 未找到配置信息")
		_, err := c.Writer.Write([]byte("fail"))
		if err != nil {
			log.Println("易支付回调写入失败")
		}
		return
	}
	verifyInfo, err := client.Verify(params)
	if err != nil || !verifyInfo.VerifyStatus {
		_, err := c.Writer.Write([]byte("fail"))
		if err != nil {
			log.Println("易支付回调写入失败")
		}
		log.Println("易支付回调签名验证失败")
		return
	}

	if verifyInfo.TradeStatus == epay.StatusTradeSuccess {
		log.Println(verifyInfo)
		if err := completeEpayTopUpWithLock(verifyInfo.ServiceTradeNo, verifyInfo.Type, verifyInfo.Money, c.ClientIP()); err != nil {
			log.Printf("易支付回调完成订单失败: trade_no=%s type=%s money=%s err=%v", verifyInfo.ServiceTradeNo, verifyInfo.Type, verifyInfo.Money, err)
			// 订单已非 pending（通常是重复回调且此前已入账），确认 success
			// 避免平台无限重试；其余入账失败写 fail 让平台重试。
			if errors.Is(err, topupstore.ErrTopUpStatusInvalid) {
				_, _ = c.Writer.Write([]byte("success"))
			} else {
				_, _ = c.Writer.Write([]byte("fail"))
			}
			return
		}
		log.Printf("易支付回调更新用户成功 trade_no=%s", verifyInfo.ServiceTradeNo)
		// 入账成功后才向平台确认，避免"已扣款未到账"时平台不再重试。
		if _, err := c.Writer.Write([]byte("success")); err != nil {
			log.Println("易支付回调写入失败")
		}
	} else {
		log.Printf("易支付异常回调: %v", verifyInfo)
		// 平台已知状态、无需重试，确认为已收到。
		if _, err := c.Writer.Write([]byte("success")); err != nil {
			log.Println("易支付回调写入失败")
		}
	}
}

func RequestAmount(c *gin.Context) {
	var req AmountRequest
	err := c.ShouldBindJSON(&req)
	if err != nil {
		httpapi.ApiErrorI18n(c, i18n.MsgInvalidParams)
		return
	}

	if req.Amount < getMinTopup() {
		respondTopupError(c, i18n.MsgTopupAmountBelowMin, map[string]any{"Min": getMinTopup()})
		return
	}
	id := c.GetInt("id")
	group, err := userstore.GetUserGroup(id, true)
	if err != nil {
		respondTopupError(c, i18n.MsgTopupGetGroupFailed)
		return
	}
	payMoney := getPayMoney(req.Amount, group)
	if payMoney <= 0.01 {
		respondTopupError(c, i18n.MsgTopupPayAmountTooLow)
		return
	}
	respondTopupSuccess(c, strconv.FormatFloat(payMoney, 'f', 2, 64), "")
}

func GetUserTopUps(c *gin.Context) {
	userId := c.GetInt("id")
	pageInfo := common.GetPageQuery(c)
	keyword := c.Query("keyword")

	var (
		topups []*topupstore.TopUp
		total  int64
		err    error
	)
	if keyword != "" {
		topups, total, err = topupstore.SearchUserTopUps(userId, keyword, pageInfo)
	} else {
		topups, total, err = topupstore.GetUserTopUps(userId, pageInfo)
	}
	if err != nil {
		common.SysError("get user topups failed: " + err.Error())
		httpapi.ApiErrorI18n(c, i18n.MsgDatabaseError)
		return
	}

	pageInfo.SetTotal(int(total))
	pageInfo.SetItems(topups)
	httpapi.ApiSuccess(c, pageInfo)
}

// GetAllTopUps 管理员获取全平台充值记录
func GetAllTopUps(c *gin.Context) {
	pageInfo := common.GetPageQuery(c)
	keyword := c.Query("keyword")

	var (
		topups []*topupstore.TopUp
		total  int64
		err    error
	)
	if keyword != "" {
		topups, total, err = topupstore.SearchAllTopUps(keyword, pageInfo)
	} else {
		topups, total, err = topupstore.GetAllTopUps(pageInfo)
	}
	if err != nil {
		common.SysError("get all topups failed: " + err.Error())
		httpapi.ApiErrorI18n(c, i18n.MsgDatabaseError)
		return
	}

	pageInfo.SetTotal(int(total))
	pageInfo.SetItems(topups)
	httpapi.ApiSuccess(c, pageInfo)
}

type AdminCompleteTopupRequest struct {
	TradeNo string `json:"trade_no"`
}

// AdminCompleteTopUp 管理员补单接口
func AdminCompleteTopUp(c *gin.Context) {
	var req AdminCompleteTopupRequest
	if err := c.ShouldBindJSON(&req); err != nil || req.TradeNo == "" {
		httpapi.ApiErrorI18n(c, i18n.MsgInvalidParams)
		return
	}

	// 订单级互斥，防止并发补单
	payment.LockOrder(req.TradeNo)
	defer payment.UnlockOrder(req.TradeNo)

	if err := topupstore.ManualCompleteTopUp(req.TradeNo, c.ClientIP()); err != nil {
		common.SysError("manual complete topup failed: " + err.Error())
		httpapi.ApiErrorI18n(c, i18n.MsgDatabaseError)
		return
	}
	httpapi.ApiSuccess(c, nil)
}

type CheckTopupRequest struct {
	TradeNo string `json:"trade_no"`
}

// CheckTopUp 用户自助检查易支付订单支付状态：网关已支付时按回调同路径入账，
// 用于补回丢失的支付回调。普通用户仅能检查本人订单，管理员可检查全部订单。
func CheckTopUp(c *gin.Context) {
	var req CheckTopupRequest
	if err := c.ShouldBindJSON(&req); err != nil || req.TradeNo == "" {
		httpapi.ApiErrorI18n(c, i18n.MsgInvalidParams)
		return
	}

	topUp, err := topupstore.GetTopUpByTradeNo(req.TradeNo)
	if err != nil {
		common.SysError("get topup by trade_no failed: " + err.Error())
		httpapi.ApiErrorI18n(c, i18n.MsgDatabaseError)
		return
	}
	// 非本人订单与不存在的订单返回相同错误，避免泄露其他用户的订单号
	if topUp == nil || (topUp.UserId != c.GetInt("id") && c.GetInt("role") < common.RoleAdminUser) {
		respondTopupError(c, i18n.MsgTopupOrderNotFound)
		return
	}
	if topUp.PaymentProvider != topupstore.PaymentProviderEpay {
		respondTopupError(c, i18n.MsgTopupCheckProviderUnsupported)
		return
	}
	if topUp.Status != common.TopUpStatusPending {
		if topUp.Status == common.TopUpStatusSuccess {
			// 支付回调已并发完成入账而前端列表仍是 pending 快照，直接确认成功
			httpapi.ApiSuccess(c, gin.H{"status": common.TopUpStatusSuccess})
			return
		}
		respondTopupError(c, i18n.MsgTopupCheckStatusNotPending)
		return
	}
	if GetEpayClient() == nil {
		respondTopupError(c, i18n.MsgTopupPaymentConfigMissing)
		return
	}

	result, err := payment.QueryEpayOrder(req.TradeNo)
	if err != nil {
		common.SysError("query epay order failed: " + err.Error())
		respondTopupError(c, i18n.MsgTopupCheckGatewayFailed)
		return
	}
	if result.Code != 1 {
		// 网关业务拒绝（典型为订单不存在于网关，如从未打开支付页的订单），
		// 对用户而言等同尚未支付；保留服务端日志便于排查网关侧异常。
		common.SysError(fmt.Sprintf("query epay order rejected by gateway: trade_no=%s code=%d msg=%s", req.TradeNo, result.Code, result.Msg))
		httpapi.ApiSuccess(c, gin.H{"status": common.TopUpStatusPending})
		return
	}
	if result.Status != 1 {
		// 网关确认订单尚未支付，属正常查询结果
		httpapi.ApiSuccess(c, gin.H{"status": common.TopUpStatusPending})
		return
	}
	if result.Type == "" || result.Money == "" {
		// 网关未回传支付方式/金额时缺少与回调对等的校验依据，拒绝入账
		common.SysError(fmt.Sprintf("query epay order paid response missing type/money: trade_no=%s type=%q money=%q", req.TradeNo, result.Type, result.Money))
		respondTopupError(c, i18n.MsgTopupCheckGatewayFailed)
		return
	}

	// 网关已支付：与回调同路径入账（订单锁 + 金额/支付方式校验 + 原子状态翻转）
	if err := completeEpayTopUpWithLock(req.TradeNo, result.Type, string(result.Money), c.ClientIP()); err != nil {
		if errors.Is(err, topupstore.ErrTopUpStatusInvalid) {
			// 查单期间支付回调并发完成入账，视为已支付成功
			httpapi.ApiSuccess(c, gin.H{"status": common.TopUpStatusSuccess})
			return
		}
		if errors.Is(err, topupstore.ErrPaymentProviderMismatch) ||
			errors.Is(err, topupstore.ErrPaymentMethodMismatch) ||
			errors.Is(err, topupstore.ErrPaymentAmountMismatch) {
			// 网关侧数据与本地订单不一致（如收银台更换了支付渠道、金额不符），
			// 属确定性失败，需人工核对，不应提示重试
			common.SysError("check topup rejected by validation: trade_no=" + req.TradeNo + " err=" + err.Error())
			respondTopupError(c, i18n.MsgTopupCheckOrderMismatch)
			return
		}
		common.SysError("check topup complete failed: trade_no=" + req.TradeNo + " err=" + err.Error())
		httpapi.ApiErrorI18n(c, i18n.MsgDatabaseError)
		return
	}
	httpapi.ApiSuccess(c, gin.H{"status": common.TopUpStatusSuccess})
}
