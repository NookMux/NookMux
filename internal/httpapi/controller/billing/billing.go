package billingcontroller

import (
	"github.com/NookMux/NookMux/internal/common"
	"github.com/NookMux/NookMux/internal/domain/shared"
	channelcontroller "github.com/NookMux/NookMux/internal/httpapi/controller/channel"
	tokencontroller "github.com/NookMux/NookMux/internal/httpapi/controller/token"
	"github.com/NookMux/NookMux/internal/i18n"
	"github.com/NookMux/NookMux/internal/store/token"
	"github.com/gin-gonic/gin"
)

func GetSubscription(c *gin.Context) {
	var remainQuota int
	var usedQuota int
	var err error
	var token *tokenstore.Token
	var expiredTime int64

	token, err = tokencontroller.GetTokenForFeedback(c)
	if err == nil {
		snapshot := token.GetQuotaSnapshot()
		expiredTime = token.ExpiredTime
		remainQuota = snapshot.TotalAvailable
		usedQuota = snapshot.TotalUsed
	}
	if expiredTime <= 0 {
		expiredTime = 0
	}
	if err != nil {
		common.SysError("failed to get subscription token: " + err.Error())
		openAIError := shared.OpenAIError{
			Message: i18n.T(c, i18n.MsgDatabaseError),
			Type:    "upstream_error",
		}
		c.JSON(200, gin.H{
			"error": openAIError,
		})
		return
	}
	quota := remainQuota + usedQuota
	amount := float64(quota) / common.QuotaPerUnit
	if token != nil && token.UnlimitedQuota {
		amount = 100000000
	}
	subscription := channelcontroller.OpenAISubscriptionResponse{
		Object:             "billing_subscription",
		HasPaymentMethod:   true,
		SoftLimitUSD:       amount,
		HardLimitUSD:       amount,
		SystemHardLimitUSD: amount,
		AccessUntil:        expiredTime,
	}
	c.JSON(200, subscription)
}

func GetUsage(c *gin.Context) {
	var quota int
	var err error
	var token *tokenstore.Token

	token, err = tokencontroller.GetTokenForFeedback(c)
	if err == nil {
		snapshot := token.GetQuotaSnapshot()
		quota = snapshot.TotalUsed
	}
	if err != nil {
		common.SysError("failed to get usage token: " + err.Error())
		openAIError := shared.OpenAIError{
			Message: i18n.T(c, i18n.MsgDatabaseError),
			Type:    string(shared.ErrorTypeNookMuxError),
		}
		c.JSON(200, gin.H{
			"error": openAIError,
		})
		return
	}
	amount := float64(quota) / common.QuotaPerUnit
	usage := channelcontroller.OpenAIUsageResponse{
		Object:     "list",
		TotalUsage: amount * 100,
	}
	c.JSON(200, usage)
}
