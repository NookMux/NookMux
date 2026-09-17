package channelcontroller

import (
	"errors"
	"fmt"
	"github.com/NookMux/NookMux/internal/common"
	planquota "github.com/NookMux/NookMux/internal/domain/billing/plan_quota"
	domainchannel "github.com/NookMux/NookMux/internal/domain/channel"
	"github.com/NookMux/NookMux/internal/domain/channel/constant"
	"github.com/NookMux/NookMux/internal/httpapi"
	"github.com/NookMux/NookMux/internal/i18n"
	httpclient "github.com/NookMux/NookMux/internal/infra/httpclient"
	"github.com/NookMux/NookMux/internal/store/channel"
	"github.com/NookMux/NookMux/pkg/jsonx"
	"github.com/gin-gonic/gin"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// https://github.com/songquanpeng/one-api/issues/79

type OpenAISubscriptionResponse struct {
	Object             string  `json:"object"`
	HasPaymentMethod   bool    `json:"has_payment_method"`
	SoftLimitUSD       float64 `json:"soft_limit_usd"`
	HardLimitUSD       float64 `json:"hard_limit_usd"`
	SystemHardLimitUSD float64 `json:"system_hard_limit_usd"`
	AccessUntil        int64   `json:"access_until"`
}

type OpenAIUsageDailyCost struct {
	Timestamp float64 `json:"timestamp"`
	LineItems []struct {
		Name string  `json:"name"`
		Cost float64 `json:"cost"`
	}
}

type OpenAIUsageResponse struct {
	Object string `json:"object"`
	//DailyCosts []OpenAIUsageDailyCost `json:"daily_costs"`
	TotalUsage float64 `json:"total_usage"` // unit: 0.01 dollar
}

type SiliconFlowUsageResponse struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Status  bool   `json:"status"`
	Data    struct {
		ID            string `json:"id"`
		Name          string `json:"name"`
		Image         string `json:"image"`
		Email         string `json:"email"`
		IsAdmin       bool   `json:"isAdmin"`
		Balance       string `json:"balance"`
		Status        string `json:"status"`
		Introduction  string `json:"introduction"`
		Role          string `json:"role"`
		ChargeBalance string `json:"chargeBalance"`
		TotalBalance  string `json:"totalBalance"`
		Category      string `json:"category"`
	} `json:"data"`
}

// deepSeekBalanceInfo 对应 DeepSeek /user/balance 中单币种的余额条目，
// 金额为字符串形式的十进制数；国内账户为 CNY，国际账户为 USD。
type deepSeekBalanceInfo struct {
	Currency        string `json:"currency"`
	TotalBalance    string `json:"total_balance"`
	GrantedBalance  string `json:"granted_balance"`
	ToppedUpBalance string `json:"topped_up_balance"`
}

type DeepSeekUsageResponse struct {
	IsAvailable  bool                  `json:"is_available"`
	BalanceInfos []deepSeekBalanceInfo `json:"balance_infos"`
}

type OpenRouterCreditResponse struct {
	Data struct {
		TotalCredits float64 `json:"total_credits"`
		TotalUsage   float64 `json:"total_usage"`
	} `json:"data"`
}

// 上游余额币种标识，用于实时刷新链路的货币展示。
const (
	balanceCurrencyCNY = "CNY"
	balanceCurrencyUSD = "USD"
)

// pickDeepSeekBalanceInfo 挑选实时展示用的余额条目：
// 优先取 CNY，缺失时回退 USD，均缺失说明上游返回异常。
func pickDeepSeekBalanceInfo(infos []deepSeekBalanceInfo) (deepSeekBalanceInfo, error) {
	for _, currency := range []string{balanceCurrencyCNY, balanceCurrencyUSD} {
		for _, balanceInfo := range infos {
			if strings.ToUpper(strings.TrimSpace(balanceInfo.Currency)) == currency {
				return balanceInfo, nil
			}
		}
	}
	return deepSeekBalanceInfo{}, errors.New("balance_infos has no CNY or USD entry")
}

// parseDeepSeekAmount 解析赠金/充值余额，字段缺失时按 0 处理。
func parseDeepSeekAmount(value string) (float64, error) {
	if strings.TrimSpace(value) == "" {
		return 0, nil
	}
	return strconv.ParseFloat(value, 64)
}

// GetAuthHeader get auth header
func GetAuthHeader(token string) http.Header {
	h := http.Header{}
	h.Add("Authorization", fmt.Sprintf("Bearer %s", token))
	return h
}

// GetClaudeAuthHeader get claude auth header
func GetClaudeAuthHeader(token string) http.Header {
	h := http.Header{}
	h.Add("x-api-key", token)
	h.Add("anthropic-version", "2023-06-01")
	return h
}

func GetResponseBody(method, url string, channel *channelstore.Channel, headers http.Header) ([]byte, error) {
	req, err := http.NewRequest(method, url, nil)
	if err != nil {
		return nil, err
	}
	for k := range headers {
		req.Header.Add(k, headers.Get(k))
	}
	client, err := httpclient.NewProxyHttpClient(channel.GetSetting().Proxy)
	if err != nil {
		return nil, err
	}
	res, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("status code: %d", res.StatusCode)
	}
	body, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, err
	}
	err = res.Body.Close()
	if err != nil {
		return nil, err
	}
	return body, nil
}

func updateChannelSiliconFlowBalance(channel *channelstore.Channel) (float64, error) {
	url := "https://api.siliconflow.cn/v1/user/info"
	body, err := GetResponseBody("GET", url, channel, GetAuthHeader(channel.Key))
	if err != nil {
		return 0, err
	}
	response := SiliconFlowUsageResponse{}
	err = jsonx.Unmarshal(body, &response)
	if err != nil {
		return 0, err
	}
	if response.Code != 20000 {
		return 0, fmt.Errorf("code: %d, message: %s", response.Code, response.Message)
	}
	balance, err := strconv.ParseFloat(response.Data.TotalBalance, 64)
	if err != nil {
		return 0, err
	}
	channel.UpdateBalance(balance)
	return balance, nil
}

func updateChannelDeepSeekBalance(channel *channelstore.Channel) (float64, string, float64, float64, error) {
	url := "https://api.deepseek.com/user/balance"
	body, err := GetResponseBody("GET", url, channel, GetAuthHeader(channel.Key))
	if err != nil {
		return 0, "", 0, 0, err
	}
	response := DeepSeekUsageResponse{}
	err = jsonx.Unmarshal(body, &response)
	if err != nil {
		return 0, "", 0, 0, err
	}
	balanceInfo, err := pickDeepSeekBalanceInfo(response.BalanceInfos)
	if err != nil {
		return 0, "", 0, 0, err
	}
	balance, err := strconv.ParseFloat(balanceInfo.TotalBalance, 64)
	if err != nil {
		return 0, "", 0, 0, err
	}
	grantedBalance, err := parseDeepSeekAmount(balanceInfo.GrantedBalance)
	if err != nil {
		return 0, "", 0, 0, err
	}
	toppedUpBalance, err := parseDeepSeekAmount(balanceInfo.ToppedUpBalance)
	if err != nil {
		return 0, "", 0, 0, err
	}
	currency := strings.ToUpper(strings.TrimSpace(balanceInfo.Currency))
	channel.UpdateBalance(balance)
	return balance, currency, grantedBalance, toppedUpBalance, nil
}

func updateChannelOpenRouterBalance(channel *channelstore.Channel) (float64, error) {
	url := "https://openrouter.ai/api/v1/credits"
	body, err := GetResponseBody("GET", url, channel, GetAuthHeader(channel.Key))
	if err != nil {
		return 0, err
	}
	response := OpenRouterCreditResponse{}
	err = jsonx.Unmarshal(body, &response)
	if err != nil {
		return 0, err
	}
	balance := response.Data.TotalCredits - response.Data.TotalUsage
	channel.UpdateBalance(balance)
	return balance, nil
}

func updateChannelMoonshotBalance(channel *channelstore.Channel) (float64, error) {
	url := "https://api.moonshot.cn/v1/users/me/balance"
	body, err := GetResponseBody("GET", url, channel, GetAuthHeader(channel.Key))
	if err != nil {
		return 0, err
	}

	type MoonshotBalanceData struct {
		AvailableBalance float64 `json:"available_balance"`
		VoucherBalance   float64 `json:"voucher_balance"`
		CashBalance      float64 `json:"cash_balance"`
	}

	type MoonshotBalanceResponse struct {
		Code   int                 `json:"code"`
		Data   MoonshotBalanceData `json:"data"`
		Scode  string              `json:"scode"`
		Status bool                `json:"status"`
	}

	response := MoonshotBalanceResponse{}
	err = jsonx.Unmarshal(body, &response)
	if err != nil {
		return 0, err
	}
	if !response.Status || response.Code != 0 {
		return 0, fmt.Errorf("failed to update moonshot balance, status: %v, code: %d, scode: %s", response.Status, response.Code, response.Scode)
	}
	// 上游余额为人民币原值，直接落库并返回
	availableBalanceCny := response.Data.AvailableBalance
	channel.UpdateBalance(availableBalanceCny)
	return availableBalanceCny, nil
}

// updateChannelBalance 刷新指定渠道余额并返回实时查询结果：
// balance 为上游原值，currency 标识其币种，DeepSeek 额外返回赠金与充值余额明细。
func updateChannelBalance(c *gin.Context, channel *channelstore.Channel) (float64, string, float64, float64, error) {
	baseURL := constant.ChannelBaseURLs[channel.Type]
	if channel.GetBaseURL() == "" {
		channel.BaseURL = &baseURL
	}
	switch channel.Type {
	case constant.ChannelTypeOpenAI:
		if channel.GetBaseURL() != "" {
			baseURL = channel.GetBaseURL()
		}
	case constant.ChannelTypeAzure:
		return 0, "", 0, 0, errors.New(i18n.T(c, i18n.MsgChannelBalanceNotImplemented))
	case constant.ChannelTypeCustom:
		baseURL = channel.GetBaseURL()
	case constant.ChannelTypeSiliconFlow:
		balance, err := updateChannelSiliconFlowBalance(channel)
		return balance, balanceCurrencyUSD, 0, 0, err
	case constant.ChannelTypeDeepSeek:
		return updateChannelDeepSeekBalance(channel)
	case constant.ChannelTypeOpenRouter:
		balance, err := updateChannelOpenRouterBalance(channel)
		return balance, balanceCurrencyUSD, 0, 0, err
	case constant.ChannelTypeMoonshot:
		balance, err := updateChannelMoonshotBalance(channel)
		return balance, balanceCurrencyCNY, 0, 0, err
	case constant.ChannelTypeZhipu_v4:
		balance, err := updateChannelZhipuBalance(channel)
		return balance, balanceCurrencyCNY, 0, 0, err
	default:
		return 0, "", 0, 0, errors.New(i18n.T(c, i18n.MsgChannelBalanceNotImplemented))
	}
	url := fmt.Sprintf("%s/v1/dashboard/billing/subscription", baseURL)

	body, err := GetResponseBody("GET", url, channel, GetAuthHeader(channel.Key))
	if err != nil {
		return 0, "", 0, 0, err
	}
	subscription := OpenAISubscriptionResponse{}
	err = jsonx.Unmarshal(body, &subscription)
	if err != nil {
		return 0, "", 0, 0, err
	}
	now := time.Now()
	startDate := fmt.Sprintf("%s-01", now.Format("2006-01"))
	endDate := now.Format("2006-01-02")
	if !subscription.HasPaymentMethod {
		startDate = now.AddDate(0, 0, -100).Format("2006-01-02")
	}
	url = fmt.Sprintf("%s/v1/dashboard/billing/usage?start_date=%s&end_date=%s", baseURL, startDate, endDate)
	body, err = GetResponseBody("GET", url, channel, GetAuthHeader(channel.Key))
	if err != nil {
		return 0, "", 0, 0, err
	}
	usage := OpenAIUsageResponse{}
	err = jsonx.Unmarshal(body, &usage)
	if err != nil {
		return 0, "", 0, 0, err
	}
	balance := subscription.HardLimitUSD - usage.TotalUsage/100
	channel.UpdateBalance(balance)
	return balance, balanceCurrencyUSD, 0, 0, nil
}

// updateChannelZhipuBalance 通过智谱账户报告接口刷新 GLM-4V 渠道余额。
// Key 取数据库保存的渠道密钥，请求由服务端发出并经渠道代理。
func updateChannelZhipuBalance(channel *channelstore.Channel) (float64, error) {
	key := strings.Split(channel.Key, "\n")[0]
	report, err := planquota.FetchGlmAccountReport(key, channel.ChannelInfo.PlanName, channel.GetSetting().Proxy)
	if err != nil {
		return 0, err
	}
	balanceCNY, err := planquota.PickGlmPersistBalance(report)
	if err != nil {
		return 0, err
	}
	channel.UpdateBalance(balanceCNY)
	return balanceCNY, nil
}

func UpdateChannelBalance(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		httpapi.ApiErrorI18n(c, i18n.MsgChannelIDFormatError, map[string]any{"Error": err.Error()})
		return
	}
	channel, err := channelstore.CacheGetChannel(id)
	if err != nil {
		common.SysError("failed to cache get channel: " + err.Error())
		httpapi.ApiErrorI18n(c, i18n.MsgDatabaseError)
		return
	}
	if channel.ChannelInfo.IsMultiKey {
		httpapi.ApiErrorI18n(c, i18n.MsgChannelMultiKeyBalanceUnsupported)
		return
	}
	balance, currency, grantedBalance, toppedUpBalance, err := updateChannelBalance(c, channel)
	if err != nil {
		common.SysError("failed to update channel balance: " + err.Error())
		httpapi.ApiErrorI18n(c, i18n.MsgChannelQuotaQueryFailed, map[string]any{"Error": err.Error()})
		return
	}
	response := gin.H{
		"success":  true,
		"message":  "",
		"balance":  balance,
		"currency": currency,
	}
	if grantedBalance != 0 || toppedUpBalance != 0 {
		response["granted_balance"] = grantedBalance
		response["topped_up_balance"] = toppedUpBalance
	}
	c.JSON(http.StatusOK, response)
}

func updateAllChannelsBalance() error {
	channels, err := channelstore.GetAllChannels(0, 0, true, false)
	if err != nil {
		return err
	}
	for _, channel := range channels {
		if channel.Status != common.ChannelStatusEnabled {
			continue
		}
		if channel.ChannelInfo.IsMultiKey {
			continue // skip multi-key channels
		}
		// TODO: support Azure
		//if channel.Type != common.ChannelTypeOpenAI && channel.Type != common.ChannelTypeCustom {
		//	continue
		//}
		balance, _, _, _, err := updateChannelBalance(nil, channel)
		if err != nil {
			continue
		} else {
			// err is nil & balance <= 0 means quota is used up
			if balance <= 0 {
				domainchannel.DisableChannel(*domainchannel.NewChannelError(channel.Id, channel.Type, channel.Name, channel.ChannelInfo.IsMultiKey, "", channel.GetAutoBan()), "余额不足")
			}
		}
		time.Sleep(common.RequestInterval)
	}
	return nil
}

func UpdateAllChannelsBalance(c *gin.Context) {
	// TODO: make it async
	err := updateAllChannelsBalance()
	if err != nil {
		common.SysError("failed to update all channels balance: " + err.Error())
		httpapi.ApiErrorI18n(c, i18n.MsgDatabaseError)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
	})
}

func AutomaticallyUpdateChannels(frequency int) {
	for {
		time.Sleep(time.Duration(frequency) * time.Minute)
		common.SysLog("updating all channels")
		_ = updateAllChannelsBalance()
		common.SysLog("channels update done")
	}
}
