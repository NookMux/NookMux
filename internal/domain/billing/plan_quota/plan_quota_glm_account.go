package planquota

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/NookMux/NookMux/internal/i18n"
	"github.com/NookMux/NookMux/pkg/jsonx"
)

// ============================================================================
// GLM 账户资金报告 (account/query-customer-account-report)
// ============================================================================

// glmAccountReportPath 智谱账户报告接口路径，返回余额、充值、赠金、消耗等
// 资金汇总。上游 base URL 按套餐区域划分，复用 getGlmApiBase：
// 国内套餐与未标识套餐走 www.bigmodel.cn，国际套餐走 api.z.ai。
const glmAccountReportPath = "/api/biz/account/query-customer-account-report"

// glmAccountReportRaw 上游账户报告 data 字段原始结构（保持上游 camelCase）。
// 金额字段全部用指针：上游对无值字段返回 null（如 todaySpendAmount），
// 且未建模字段可能随时在 null 与数字间变化（如 creditBalance、creditStatus），
// 指针 + 忽略未建模字段保证解析兼容，不会因上游字段变化而报错。
type glmAccountReportRaw struct {
	Balance          *float64 `json:"balance"`          // 当前余额
	RechargeAmount   *float64 `json:"rechargeAmount"`   // 累计现金充值的余额
	GiveAmount       *float64 `json:"giveAmount"`       // 系统赠金
	TotalSpendAmount *float64 `json:"totalSpendAmount"` // 累计现金消耗额度
	AvailableBalance *float64 `json:"availableBalance"` // 活跃可用余额
	FrozenBalance    *float64 `json:"frozenBalance"`    // 冻结余额
}

// GlmAccountReportData 账户资金指标的对外展示结构（snake_case），
// 仅暴露需要展示的字段，金额单位为人民币原值。
type GlmAccountReportData struct {
	Balance          *float64 `json:"balance"`            // 当前余额
	RechargeAmount   *float64 `json:"recharge_amount"`    // 累计现金充值的余额
	GiveAmount       *float64 `json:"give_amount"`        // 系统赠金
	TotalSpendAmount *float64 `json:"total_spend_amount"` // 累计现金消耗额度
	AvailableBalance *float64 `json:"available_balance"`  // 活跃可用余额
	FrozenBalance    *float64 `json:"frozen_balance"`     // 冻结余额
}

// glmAccountReportResp 账户报告响应外壳。code/success 用指针：
// 缺失视为未给出，只有显式失败才判为失败，避免上游精简响应体时误报。
type glmAccountReportResp struct {
	Code    *int                 `json:"code"`
	Msg     string               `json:"msg"`
	Success *bool                `json:"success"`
	Data    *glmAccountReportRaw `json:"data"`
}

// glmAccountReportHeaders 构建账户报告接口请求头。该接口要求浏览器 UA，
// 缺失时上游拒绝请求；Accept 与官方前端保持一致传 */*。
func glmAccountReportHeaders() http.Header {
	h := http.Header{}
	h.Set("Accept", "*/*")
	h.Set("User-Agent", glmBrowserUserAgent)
	return h
}

// FetchGlmAccountReport 查询智谱账户资金报告。Key 由后端注入
// （Authorization: Bearer <key>），不在浏览器直连智谱后台；
// proxyURL 非空时请求经渠道代理发出。认证失败（code=1000）由
// fetchGlmAPI 升级为 ErrGlmKeyInvalid 返回。
func FetchGlmAccountReport(apiKey string, planBaseURL string, proxyURL string) (*GlmAccountReportData, error) {
	apiBase := getGlmApiBase(planBaseURL)
	if apiBase == "" {
		return nil, i18n.NewError(i18n.MsgPlanQuotaAPIBaseUnknown)
	}

	// 与联系方式查询的官方调用形式一致，Authorization 需带 Bearer 前缀；
	// fetchGlmAPI 以 apiKey 参数为准注入 Authorization，故在此拼接。
	auth := "Bearer " + strings.TrimSpace(apiKey)
	body, err := fetchGlmAPI(apiBase, glmAccountReportPath, auth, proxyURL, glmAccountReportHeaders())
	if err != nil {
		return nil, err
	}

	return parseGlmAccountReport(body)
}

// parseGlmAccountReport 解析账户报告响应。code/success 只有显式失败才判失败，
// 缺失视为未给出；data 缺失属于上游异常状态，直接报错暴露。
func parseGlmAccountReport(body []byte) (*GlmAccountReportData, error) {
	var resp glmAccountReportResp
	if err := jsonx.Unmarshal(body, &resp); err != nil {
		return nil, i18n.NewError(i18n.MsgChannelParseResponseFailed, map[string]any{"Error": err.Error()}).WithCause(err)
	}
	if (resp.Code != nil && *resp.Code != 200) || (resp.Success != nil && !*resp.Success) {
		msg := strings.TrimSpace(resp.Msg)
		code := "nil"
		if resp.Code != nil {
			code = strconv.Itoa(*resp.Code)
		}
		if msg == "" {
			return nil, i18n.NewError(i18n.MsgPlanQuotaUpstreamNonSuccess)
		}
		return nil, i18n.NewErrorWithFallback(fmt.Sprintf("%s (code=%s)", msg, code), i18n.MsgPlanQuotaUpstreamCodeError, map[string]any{"Message": msg, "Code": code})
	}
	if resp.Data == nil {
		return nil, i18n.NewError(i18n.MsgPlanQuotaNoReportData)
	}

	raw := resp.Data
	return &GlmAccountReportData{
		Balance:          raw.Balance,
		RechargeAmount:   raw.RechargeAmount,
		GiveAmount:       raw.GiveAmount,
		TotalSpendAmount: raw.TotalSpendAmount,
		AvailableBalance: raw.AvailableBalance,
		FrozenBalance:    raw.FrozenBalance,
	}, nil
}

// PickGlmPersistBalance 从账户报告中选取落库用的余额（人民币原值）：
// 优先活跃可用余额（冻结金额不计入可用），缺失时回退当前余额；
// 两者均缺失说明上游数据异常，直接报错暴露问题。
func PickGlmPersistBalance(report *GlmAccountReportData) (float64, error) {
	if report == nil {
		return 0, i18n.NewError(i18n.MsgPlanQuotaEmptyReport)
	}
	if report.AvailableBalance != nil {
		return *report.AvailableBalance, nil
	}
	if report.Balance != nil {
		return *report.Balance, nil
	}
	return 0, i18n.NewError(i18n.MsgPlanQuotaNoBalanceData)
}
