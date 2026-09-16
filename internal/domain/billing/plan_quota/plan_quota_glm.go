package planquota

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	httpclient "github.com/NookMux/NookMux/internal/infra/httpclient"
	"github.com/NookMux/NookMux/pkg/jsonx"
	"github.com/google/uuid"
)

// GLM 套餐查询的 API 端点
const (
	glmSubscriptionPath        = "/api/biz/subscription/list?pageSize=10&pageNum=1"
	glmQuotaLimitPath          = "/api/monitor/usage/quota/limit"
	glmCreditUsageActivityPath = "/api/monitor/credit-usage/activity"
	glmActivityAccountPersonal = "1" // 个人套餐

	// GlmActivityAccountPersonal 是导出别名，供 controller 按业务语义引用，
	// 其值仍以 glmActivityAccountPersonal 为单一来源，避免在调用处写字面量。
	GlmActivityAccountPersonal = glmActivityAccountPersonal
)

// glmActivityLookbackDays 智谱 GLM 个人套餐活跃数据展示窗口：当前日期往前推 365 天。
// 智谱活动接口在更长时间窗下响应体可能显著增大，保持 365 天与智谱官方前端一致。
const glmActivityLookbackDays = 365

// 智谱套餐额度类型。TOKENS_LIMIT/TIME_LIMIT 为按 Tokens 计费的 V1/V2 套餐条目，
// CREDIT_LIMIT 为 V3 积分套餐特有的按积分计费条目（参考智谱官网前端 CREDIT_LIMIT 常量）。
const (
	glmLimitTypeTokens = "TOKENS_LIMIT"
	glmLimitTypeTime   = "TIME_LIMIT"
	glmLimitTypeCredit = "CREDIT_LIMIT"
)

// GLM 套餐版本。智谱官方前端按 V1/V2/V3 区分套餐代次：
//   - V1：仅每 5 小时额度（unit=3），无每周额度；
//   - V2：每 5 小时 + 每周额度（unit=6），按 Tokens 计费；
//   - V3：额度条目 type=CREDIT_LIMIT，按积分计费。
const (
	glmPlanVersion1 = "V1"
	glmPlanVersion2 = "V2"
	glmPlanVersion3 = "V3"
)

// glmAuthFailureCode 是智谱 biz/monitor 接口在 API Key 无效或认证失败时返回的固定错误码。
// 智谱在 Key 失效时会返回 {"code":1000,"msg":"Authentication Failed"/"身份验证失败。","success":false}，
// 且 HTTP 状态码仍为 200，必须从响应体识别，否则会被误判为风控或空数据。
const glmAuthFailureCode = 1000

// ErrGlmKeyInvalid 表示智谱 API Key 无效或认证失败。
// Controller 通过 errors.Is 识别该错误，向前端返回明确的「Key 无效」提示，
// 而不是把空数据当作风控或空白用量展示。
var ErrGlmKeyInvalid = errors.New("glm api key invalid or authentication failed")

// glmBizResp 智谱 biz/monitor 接口通用响应外壳，用于识别认证失败等业务层错误。
type glmBizResp struct {
	Code    int    `json:"code"`
	Msg     string `json:"msg"`
	Success bool   `json:"success"`
}

// isGlmAuthFailure 判断响应体是否为智谱认证失败（Key 无效）。
// 智谱的 msg 会随 Accept-Language 在 "Authentication Failed" 与 "身份验证失败。" 间切换，
// 因此以稳定的 code 码作为判定依据，而不是匹配 msg 文案。
func isGlmAuthFailure(body []byte) bool {
	var resp glmBizResp
	if err := jsonx.Unmarshal(body, &resp); err != nil {
		return false
	}
	return resp.Code == glmAuthFailureCode
}

// GlmPlanQuotaData 聚合了智谱 GLM 套餐的所有可展示信息
type GlmPlanQuotaData struct {
	PlanName      string           `json:"plan_name"`
	PlanVersion   string           `json:"plan_version,omitempty"` // 套餐代次: V1 / V2 / V3
	IsCreditPlan  bool             `json:"is_credit_plan"`         // 是否为按积分计费的 V3 套餐
	ProductLevel  string           `json:"product_level"`
	ProductName   string           `json:"product_name"`
	EffectiveDate string           `json:"effective_date"`
	ExpiryDate    string           `json:"expiry_date"`
	AutoRenew     bool             `json:"auto_renew"`
	WeeklyLimit   *GlmLimitInfo    `json:"weekly_limit,omitempty"`
	TokenLimit    *GlmLimitInfo    `json:"token_limit,omitempty"`
	McpToolLimit  *GlmMcpLimitInfo `json:"mcp_tool_limit,omitempty"`
	// CreditLimit 为 V3 积分套餐的 5 小时积分额度（unit=3, type=CREDIT_LIMIT）。
	CreditLimit *GlmCreditLimitInfo `json:"credit_limit,omitempty"`
	// CreditWeeklyLimit 为 V3 积分套餐的周积分额度（unit=6, type=CREDIT_LIMIT）。
	CreditWeeklyLimit *GlmCreditLimitInfo `json:"credit_weekly_limit,omitempty"`
}

// GlmLimitInfo 通用限额信息
type GlmLimitInfo struct {
	Percentage    int    `json:"percentage"`
	NextResetTime string `json:"next_reset_time,omitempty"`
	Status        string `json:"status"`
}

// GlmCreditLimitInfo 积分限额信息。
// 智谱积分套餐会同时返回已用积分（currentValue）与总额度积分（usage），
// 官网以 "currentValue / usage 积分" 形式展示。
type GlmCreditLimitInfo struct {
	Percentage    int    `json:"percentage"`
	CurrentValue  int    `json:"current_value"`
	Usage         int    `json:"usage"`
	NextResetTime string `json:"next_reset_time,omitempty"`
	Status        string `json:"status"`
}

// GlmMcpLimitInfo MCP 工具限额信息
type GlmMcpLimitInfo struct {
	Percentage    int             `json:"percentage"`
	CurrentUsage  string          `json:"current_usage,omitempty"`
	NextResetTime string          `json:"next_reset_time,omitempty"`
	Status        string          `json:"status"`
	Tools         []GlmToolDetail `json:"tools,omitempty"`
}

// GlmToolDetail MCP 工具详情
type GlmToolDetail struct {
	Name  string `json:"name"`
	Usage int    `json:"usage"`
}

// glmResetTime 兼容智谱 nextResetTime 既可能返回字符串，也可能返回 Unix 时间戳。
// 统一规范成 RFC3339 字符串后，再交给前端按 zaicontrol 的展示逻辑格式化。
type glmResetTime string

// UnmarshalJSON 兼容字符串和数字两种时间表示，避免严格 string 反序列化失败。
func (t *glmResetTime) UnmarshalJSON(data []byte) error {
	raw := strings.TrimSpace(string(data))
	if raw == "" || raw == "null" {
		*t = ""
		return nil
	}

	var text string
	if err := jsonx.Unmarshal(data, &text); err == nil {
		normalized, err := normalizeGlmResetTime(text)
		if err != nil {
			return err
		}
		*t = glmResetTime(normalized)
		return nil
	}

	var number json.Number
	if err := jsonx.Unmarshal(data, &number); err == nil {
		normalized, err := normalizeGlmResetTime(number.String())
		if err != nil {
			return err
		}
		*t = glmResetTime(normalized)
		return nil
	}

	return fmt.Errorf("nextResetTime 字段格式不支持: %s", raw)
}

// normalizeGlmResetTime 将时间字符串统一整理成前端稳定可识别的格式。
func normalizeGlmResetTime(value string) (string, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return "", nil
	}
	if !isNumericTimestamp(trimmed) {
		return trimmed, nil
	}

	parsed, err := strconv.ParseInt(trimmed, 10, 64)
	if err != nil {
		return "", fmt.Errorf("解析 nextResetTime 时间戳失败: %w", err)
	}

	return normalizeUnixTimestamp(parsed).Format(time.RFC3339), nil
}

// isNumericTimestamp 判断字符串是否是纯数字时间戳，便于兼容被包装成字符串的时间戳。
func isNumericTimestamp(value string) bool {
	hasDigit := false
	for i, r := range value {
		if r == '-' && i == 0 {
			continue
		}
		if r < '0' || r > '9' {
			return false
		}
		hasDigit = true
	}
	return hasDigit
}

// normalizeUnixTimestamp 按位数推断秒/毫秒/微秒/纳秒时间戳，保持与 JS Date 的时间点语义一致。
func normalizeUnixTimestamp(value int64) time.Time {
	absValue := value
	if absValue < 0 {
		absValue = -absValue
	}

	switch {
	case absValue >= 1_000_000_000_000_000_000:
		return time.Unix(0, value).UTC()
	case absValue >= 1_000_000_000_000_000:
		return time.Unix(0, value*int64(time.Microsecond)).UTC()
	case absValue >= 1_000_000_000_000:
		return time.UnixMilli(value).UTC()
	default:
		return time.Unix(value, 0).UTC()
	}
}

// glmSubscriptionResp 智谱订阅接口返回格式
type glmSubscriptionResp struct {
	Data []struct {
		ProductName      string `json:"productName"`
		CurrentRenewTime string `json:"currentRenewTime"`
		NextRenewTime    string `json:"nextRenewTime"`
		AutoRenew        int    `json:"autoRenew"`
	} `json:"data"`
}

// glmLimitResp 智谱限额接口返回格式
type glmLimitResp struct {
	Data struct {
		Limits []struct {
			Type          string       `json:"type"`
			Unit          int          `json:"unit"`
			Percentage    int          `json:"percentage"`
			CurrentValue  int          `json:"currentValue"`
			Usage         int          `json:"usage"`
			NextResetTime glmResetTime `json:"nextResetTime"`
			UsageDetails  []struct {
				ModelCode string `json:"modelCode"`
				Usage     int    `json:"usage"`
			} `json:"usageDetails"`
		} `json:"limits"`
	} `json:"data"`
}

// GlmRiskCheckResult 风控检测结果
type GlmRiskCheckResult struct {
	IsRisk bool   `json:"is_risk"`
	RawMsg string `json:"raw_msg,omitempty"`
}

// CheckGlmRiskStatus 检测智谱账号是否被风控；proxyURL 非空时请求经渠道代理发出。
// 调用 https://open.bigmodel.cn/api/biz/labelCustomer/isRiskCustomer
func CheckGlmRiskStatus(apiKey string, proxyURL string) (*GlmRiskCheckResult, error) {
	url := "https://open.bigmodel.cn/api/biz/labelCustomer/isRiskCustomer"

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}

	// Key 放在 Authorization 头中做身份验证
	req.Header.Set("Authorization", strings.TrimSpace(apiKey))

	client, err := newGlmHttpClient(proxyURL)
	if err != nil {
		return nil, fmt.Errorf("代理客户端创建失败: %w", err)
	}
	res, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	body, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, err
	}

	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d: %s", res.StatusCode, string(body))
	}

	// 智谱在 Key 无效时仍返回 HTTP 200，但响应体为认证失败（code=1000）。
	// 此时 data 缺失，必须优先识别为 Key 无效，避免被下面的逻辑误判为「已风控」。
	if isGlmAuthFailure(body) {
		return nil, ErrGlmKeyInvalid
	}

	// 解析响应，期望格式: {"code":200,"msg":"操作成功","data":false,"success":true}
	var resp struct {
		Code    int         `json:"code"`
		Msg     string      `json:"msg"`
		Data    interface{} `json:"data"`
		Success bool        `json:"success"`
	}
	if err := jsonx.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("解析响应失败: %w", err)
	}

	// data 不是 false 就视为风控
	isRisk := true
	if boolVal, ok := resp.Data.(bool); ok && !boolVal {
		isRisk = false
	}

	return &GlmRiskCheckResult{
		IsRisk: isRisk,
		RawMsg: resp.Msg,
	}, nil
}

// newGlmHttpClient 返回经渠道代理的 GLM 后端客户端（15s 超时）。
// 超时会覆盖共享客户端，需拷贝实例。
func newGlmHttpClient(proxyURL string) (*http.Client, error) {
	baseClient, err := httpclient.GetHttpClientWithProxy(proxyURL)
	if err != nil {
		return nil, err
	}
	return &http.Client{
		Transport:     baseClient.Transport,
		CheckRedirect: baseClient.CheckRedirect,
		Timeout:       15 * time.Second,
	}, nil
}

// FetchGlmPlanQuota 从智谱后端拉取套餐额度数据
// apiKey: 渠道的 API Key
// planBaseURL: 套餐的基础 URL (glm-coding-plan 或 glm-coding-plan-international)
func FetchGlmPlanQuota(apiKey string, planBaseURL string, proxyURL string) (*GlmPlanQuotaData, error) {
	apiBase := getGlmApiBase(planBaseURL)
	if apiBase == "" {
		return nil, fmt.Errorf("无法确定套餐对应的 API 地址")
	}

	// 并行拉取订阅和限额
	subscriptionCh := make(chan *glmSubscriptionResp)
	limitCh := make(chan *glmLimitResp)
	errCh := make(chan error, 2)

	go func() {
		resp, err := fetchGlmAPI(apiBase, glmSubscriptionPath, apiKey, proxyURL, nil)
		if err != nil {
			errCh <- fmt.Errorf("获取订阅信息失败: %w", err)
			return
		}
		var sub glmSubscriptionResp
		if err := jsonx.Unmarshal(resp, &sub); err != nil {
			errCh <- fmt.Errorf("解析订阅信息失败: %w", err)
			return
		}
		subscriptionCh <- &sub
	}()

	go func() {
		resp, err := fetchGlmAPI(apiBase, glmQuotaLimitPath, apiKey, proxyURL, nil)
		if err != nil {
			errCh <- fmt.Errorf("获取限额信息失败: %w", err)
			return
		}
		var lim glmLimitResp
		if err := jsonx.Unmarshal(resp, &lim); err != nil {
			errCh <- fmt.Errorf("解析限额信息失败: %w", err)
			return
		}
		limitCh <- &lim
	}()

	var subscription *glmSubscriptionResp
	var limits *glmLimitResp

	for i := 0; i < 2; i++ {
		select {
		case sub := <-subscriptionCh:
			subscription = sub
		case lim := <-limitCh:
			limits = lim
		case err := <-errCh:
			return nil, err
		}
	}

	return buildGlmPlanQuotaData(subscription, limits), nil
}

// getGlmApiBase 根据套餐标识返回对应的 API 基础地址
func getGlmApiBase(planBaseURL string) string {
	switch planBaseURL {
	case "glm-coding-plan-international":
		return "https://api.z.ai"
	default:
		return "https://www.bigmodel.cn"
	}
}

// fetchGlmAPI 向智谱后端发送请求，Key 由后端注入，不会暴露给客户端；
// proxyURL 非空时请求经渠道代理发出。extraHeaders 为 nil 时使用默认头，
// 与历史 fetchGlmAPI 行为一致；调用方若需要补充请求头（如 UsageActivity）
// 可通过该参数传入。
func fetchGlmAPI(baseURL, path, apiKey, proxyURL string, extraHeaders http.Header) ([]byte, error) {
	url := strings.TrimRight(baseURL, "/") + path

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Authorization", strings.TrimSpace(apiKey))
	if extraHeaders == nil {
		// 模拟浏览器从智谱官网发起的请求
		req.Header.Set("Referer", "https://bigmodel.cn/")
		req.Header.Set("Origin", "https://bigmodel.cn")
	} else {
		for key, values := range extraHeaders {
			if len(values) == 0 {
				continue
			}
			// Authorization 始终以参数 apiKey 为准，避免被 extraHeaders 覆盖泄露到上游
			if strings.EqualFold(key, "Authorization") {
				continue
			}
			req.Header[key] = append([]string(nil), values...)
		}
	}

	client, err := newGlmHttpClient(proxyURL)
	if err != nil {
		return nil, fmt.Errorf("代理客户端创建失败: %w", err)
	}
	res, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	body, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, err
	}

	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d: %s", res.StatusCode, string(body))
	}

	// 智谱在 Key 无效时仍返回 HTTP 200，响应体为认证失败（code=1000）。
	// 这里返回明确的 Key 无效错误，避免上层把空数据当作用量空白展示。
	if isGlmAuthFailure(body) {
		return nil, ErrGlmKeyInvalid
	}

	return body, nil
}

// createBigModelUsageHeaders 构建访问 BigModel / Z.ai 个人套餐活跃接口所需的请求头。
// Authorization 由上游 OAuth + zcode 服务换取的 Coding Plan API Key 注入；
// Referer/Origin 模拟智谱/BigModel 官网控制台，确保与智谱官方前端请求一致。
// func 形式方便后续在不修改 fetchGlmAPI 调用的前提下扩展头字段。
func createBigModelUsageHeaders(apiKey string) http.Header {
	h := http.Header{}
	if trimmed := strings.TrimSpace(apiKey); trimmed != "" {
		h.Set("Authorization", trimmed)
	}
	h.Set("Referer", "https://bigmodel.cn/")
	h.Set("Origin", "https://bigmodel.cn")
	h.Set("Accept", "application/json")
	return h
}

// buildGlmPlanQuotaData 将原始 API 返回组装为前端展示结构
func buildGlmPlanQuotaData(sub *glmSubscriptionResp, lim *glmLimitResp) *GlmPlanQuotaData {
	data := &GlmPlanQuotaData{}

	// 解析订阅信息
	if sub != nil && len(sub.Data) > 0 {
		pkg := sub.Data[0]
		data.ProductName = pkg.ProductName
		data.ProductLevel = getGlmPackageLevel(pkg.ProductName)
		data.EffectiveDate = pkg.CurrentRenewTime
		data.ExpiryDate = pkg.NextRenewTime
		data.AutoRenew = pkg.AutoRenew != 0
	}

	if lim == nil {
		return data
	}

	hasWeekly := false
	hasCredit := false
	for _, l := range lim.Data.Limits {
		switch {
		case l.Type == glmLimitTypeCredit:
			// 积分额度（V3 套餐）：unit=3 为 5 小时积分额度，unit=6 为周积分额度
			hasCredit = true
			credit := &GlmCreditLimitInfo{
				Percentage:    l.Percentage,
				CurrentValue:  l.CurrentValue,
				Usage:         l.Usage,
				NextResetTime: string(l.NextResetTime),
				Status:        getGlmUsageStatus(l.Percentage),
			}
			if l.Unit == 6 {
				data.CreditWeeklyLimit = credit
			} else {
				data.CreditLimit = credit
			}
		case l.Type == glmLimitTypeTokens && l.Unit == 6:
			// 每周限额（V2 套餐特有）
			hasWeekly = true
			data.WeeklyLimit = &GlmLimitInfo{
				Percentage:    l.Percentage,
				NextResetTime: string(l.NextResetTime),
				Status:        getGlmUsageStatus(l.Percentage),
			}
		case l.Type == glmLimitTypeTokens:
			// 每5小时限额
			data.TokenLimit = &GlmLimitInfo{
				Percentage:    l.Percentage,
				NextResetTime: string(l.NextResetTime),
				Status:        getGlmUsageStatus(l.Percentage),
			}
		case l.Type == glmLimitTypeTime:
			// MCP工具限额
			mcp := &GlmMcpLimitInfo{
				Percentage:    l.Percentage,
				CurrentUsage:  fmt.Sprintf("%d/%d", l.CurrentValue, l.Usage),
				NextResetTime: string(l.NextResetTime),
				Status:        getGlmUsageStatus(l.Percentage),
			}
			toolNameMap := map[string]string{
				"search-prime": "联网搜索",
				"web-reader":   "网页读取",
				"zread":        "开源仓库",
			}
			for _, detail := range l.UsageDetails {
				name := detail.ModelCode
				if mapped, ok := toolNameMap[detail.ModelCode]; ok {
					name = mapped
				}
				mcp.Tools = append(mcp.Tools, GlmToolDetail{
					Name:  name,
					Usage: detail.Usage,
				})
			}
			data.McpToolLimit = mcp
		}
	}

	// 判定套餐代次：与智谱官方前端一致，limits 含 CREDIT_LIMIT 条目即为 V3 积分套餐；
	// 否则按是否含每周额度（unit=6）区分 V2 / V1。
	data.IsCreditPlan = hasCredit
	switch {
	case hasCredit:
		data.PlanVersion = glmPlanVersion3
	case hasWeekly:
		data.PlanVersion = glmPlanVersion2
	default:
		data.PlanVersion = glmPlanVersion1
	}

	return data
}

// getGlmPackageLevel 根据产品名推断套餐等级
func getGlmPackageLevel(productName string) string {
	name := strings.ToLower(productName)
	if strings.Contains(name, "lite") || strings.Contains(name, "基础") {
		return "Lite"
	}
	if strings.Contains(name, "pro") || strings.Contains(name, "专业") {
		return "Pro"
	}
	if strings.Contains(name, "max") || strings.Contains(name, "旗舰") || strings.Contains(name, "企业") {
		return "Max"
	}
	return "Standard"
}

// getGlmUsageStatus 根据百分比返回充裕/适中/紧张
func getGlmUsageStatus(percentage int) string {
	if percentage >= 80 {
		return "紧张"
	}
	if percentage >= 50 {
		return "适中"
	}
	return "充裕"
}

// FetchGlmUsageData 代理拉取 GLM 用量图表数据，直接透传原始 JSON；
// proxyURL 非空时请求经渠道代理发出。
func FetchGlmUsageData(apiKey string, planBaseURL string, dataType string, startTime string, endTime string, proxyURL string) (json.RawMessage, error) {
	apiBase := getGlmApiBase(planBaseURL)
	if apiBase == "" {
		return nil, fmt.Errorf("无法确定套餐对应的 API 地址")
	}

	var path string
	switch dataType {
	case "model":
		path = "/api/monitor/usage/model-usage"
	case "tool":
		path = "/api/monitor/usage/tool-usage"
	case "performance":
		path = "/api/monitor/usage/model-performance-day"
	default:
		return nil, fmt.Errorf("不支持的数据类型: %s", dataType)
	}

	if startTime != "" && endTime != "" {
		path += fmt.Sprintf("?startTime=%s&endTime=%s", url.QueryEscape(startTime), url.QueryEscape(endTime))
	}

	body, err := fetchGlmAPI(apiBase, path, apiKey, proxyURL, nil)
	if err != nil {
		return nil, err
	}

	return json.RawMessage(body), nil
}

// glmActivityResp 智谱 credit-usage/activity 接口响应结构。
// 智谱在 success 字段缺失或为 true 且 code 为 0/200/null 时视为成功；
// 与 glmBizResp 不同之处在于该接口的成功判定更宽松，因此单独建模。
type glmActivityResp struct {
	Code    *int                `json:"code"`
	Msg     string              `json:"msg"`
	Success *bool               `json:"success"`
	Data    *GlmActivityPayload `json:"data"`
}

// GlmActivityPayload 智谱 credit-usage/activity 接口 data 字段结构。
type GlmActivityPayload struct {
	Summary *GlmActivitySummary `json:"summary"`
	Series  []GlmActivityDay    `json:"series"`
}

// GlmActivitySummary 智谱活跃数据汇总：累计 Token、峰值、累计使用时长与连续天数。
type GlmActivitySummary struct {
	TotalTokens          int64   `json:"totalTokens"`
	PeakDailyTokens      int64   `json:"peakDailyTokens"`
	PeakDailyTokensDate  string  `json:"peakDailyTokensDate"`
	TotalUsageDurationMs float64 `json:"totalUsageDurationMs"`
	CurrentStreakDays    int     `json:"currentStreakDays"`
	LongestStreakDays    int     `json:"longestStreakDays"`
}

// GlmActivityDay 单日 Token 活动条目，供前端日历热力图直接消费。
type GlmActivityDay struct {
	Date        string `json:"date"`
	TotalTokens int64  `json:"totalTokens"`
	MCPCalls    int64  `json:"mcpCalls"`
}

// GlmPlanActivityData 智谱个人套餐活跃数据展示结构，与 GlmPlanQuotaData 风格保持一致。
// Series 缺失时（账号无活跃活动）保留为 nil，前端按空数据渲染。
type GlmPlanActivityData struct {
	Summary *GlmActivitySummary `json:"summary,omitempty"`
	Series  []GlmActivityDay    `json:"series,omitempty"`
}

// isGlmActivitySuccess 按用户提供的判定规则判断 credit-usage/activity 响应是否成功：
// code 为 0/200/null 且 success !== false。缺失的指针字段视为未给出，不参与否定。
func isGlmActivitySuccess(resp *glmActivityResp) bool {
	if resp == nil {
		return false
	}
	if resp.Success != nil && !*resp.Success {
		return false
	}
	if resp.Code == nil {
		return true
	}
	switch *resp.Code {
	case 0, 200:
		return true
	default:
		return false
	}
}

// ComputeGlmActivityTimeRange 按服务器时区计算个人套餐活跃接口的查询时间窗：
// startTime = 今天 00:00:00 往前推 glmActivityLookbackDays 天，
// endTime   = 服务器当天 23:59:59。
// 输出格式与智谱官方前端保持一致（YYYY-MM-DD HH:MM:SS），由 controller 在请求时按需生成。
func ComputeGlmActivityTimeRange() (startTime, endTime string) {
	now := time.Now()
	end := time.Date(now.Year(), now.Month(), now.Day(), 23, 59, 59, 0, now.Location())
	start := end.AddDate(0, 0, -glmActivityLookbackDays)
	start = time.Date(start.Year(), start.Month(), start.Day(), 0, 0, 0, 0, start.Location())
	return start.Format("2006-01-02 15:04:05"), end.Format("2006-01-02 15:04:05")
}

// FetchGlmCreditUsageActivity 调用智谱 credit-usage/activity 接口拉取个人套餐活跃数据。
// startTime/endTime 由 controller 按服务器时区计算后传入，避免被前端改值绕开 365 天语义。
// proxyURL 非空时请求经渠道代理发出。
func FetchGlmCreditUsageActivity(apiKey string, planBaseURL string, accountType string, startTime string, endTime string, proxyURL string) (*GlmPlanActivityData, error) {
	apiBase := getGlmApiBase(planBaseURL)
	if apiBase == "" {
		return nil, fmt.Errorf("无法确定套餐对应的 API 地址")
	}
	if accountType == "" {
		accountType = glmActivityAccountPersonal
	}

	path := fmt.Sprintf("%s?type=%s&startTime=%s&endTime=%s",
		glmCreditUsageActivityPath,
		url.QueryEscape(accountType),
		url.QueryEscape(startTime),
		url.QueryEscape(endTime),
	)

	body, err := fetchGlmAPI(apiBase, path, apiKey, proxyURL, createBigModelUsageHeaders(apiKey))
	if err != nil {
		return nil, err
	}

	// 与 other glm 接口一致，智谱在 Key 无效时仍可能返回 200 + 业务 code=1000，
	// fetchGlmAPI 已把该情况升级为 ErrGlmKeyInvalid，这里在解析前再次兜底。
	if isGlmAuthFailure(body) {
		return nil, ErrGlmKeyInvalid
	}

	var resp glmActivityResp
	if err := jsonx.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("解析活跃数据响应失败: %w", err)
	}
	if !isGlmActivitySuccess(&resp) {
		msg := strings.TrimSpace(resp.Msg)
		if msg == "" {
			msg = "智谱返回非成功状态"
		}
		return nil, fmt.Errorf("%s (code=%v)", msg, resp.Code)
	}

	data := &GlmPlanActivityData{}
	if resp.Data != nil {
		data.Summary = resp.Data.Summary
		data.Series = resp.Data.Series
	}
	if data.Series == nil {
		data.Series = []GlmActivityDay{}
	}
	return data, nil
}

// ============================================================================
// GLM 套餐额度重置卡 (customer-package-reset)
// ============================================================================
//
// 智谱个人套餐提供「额度重置卡」机制：用户可在 5h 额度紧张时主动触发重置，
// 或在周额度结算后通过重置卡恢复 5h 额度。重置卡按 resetType 分两类：
//   - FIVE_HOUR：仅重置 5h 额度，不消耗周重置次数；
//   - WEEK：重置周额度，同步重置 5h 额度，不额外消耗 5h 次数。
//
// 列表接口按 targetType (PERSONAL/TEAM) 返回当前可用的卡。本项目套餐均为个人
// 套餐，因此固定 targetType=PERSONAL。接口路径与智谱官方前端一致，按套餐区域
// 划分上游 base URL（国内 bigmodel.cn / 国际 api.z.ai），与 FetchGlmPlanQuota 等
// 复用 getGlmApiBase。

const (
	glmResetCardListPath = "/api/biz/customer-package-reset/list"
	glmResetCardUsePath  = "/api/biz/customer-package-reset/use"

	// GlmResetCardTypePersonal 个人套餐固定 targetType，避免上层暴露给前端可注入。
	GlmResetCardTypePersonal GlmResetCardType = "PERSONAL"
)

// GlmResetCardType 枚举智谱官方 resetType 取值。
type GlmResetCardType string

const (
	GlmResetCardTypeFiveHour GlmResetCardType = "FIVE_HOUR"
	GlmResetCardTypeWeek     GlmResetCardType = "WEEK"
)

// glmResetCardSourceKey 把 resetType 映射到上游列表接口 data 字段名，
// 与智谱官方前端 1.js 第 550-588 行的固定映射保持一致。
var glmResetCardSourceKey = map[GlmResetCardType]string{
	GlmResetCardTypeFiveHour: "fiveHourResets",
	GlmResetCardTypeWeek:     "weekResets",
}

// GlmResetCard 单条重置卡信息。上游 recordId 是数字（数据库自增 ID），
// 智谱官方前端也从列表原样以数字提交回 use 接口，因此用 int64 保持两端一致。
type GlmResetCard struct {
	RecordId   int64  `json:"recordId"`
	ExpireTime string `json:"expireTime"`
	Available  bool   `json:"available"`
	Priority   bool   `json:"priority,omitempty"`
}

// GlmResetCardListData 列表接口聚合后的展示结构，按 resetType 分组。
// 字段名与智谱官方前端消费的 data 字段名一致（fiveHourResets / weekResets），
// 前端可直接消费。
type GlmResetCardListData struct {
	FiveHourResets []GlmResetCard `json:"fiveHourResets"`
	WeekResets     []GlmResetCard `json:"weekResets"`
}

// glmResetCardRaw 上游列表接口 data 字段的原始结构，用于解析后再归一化。
type glmResetCardRaw struct {
	FiveHourResets []GlmResetCard `json:"fiveHourResets"`
	WeekResets     []GlmResetCard `json:"weekResets"`
}

// glmResetCardListResp 上游列表接口响应外壳。
type glmResetCardListResp struct {
	Code    int              `json:"code"`
	Msg     string           `json:"msg"`
	Success bool             `json:"success"`
	Data    *glmResetCardRaw `json:"data"`
}

// GlmResetCardUseResult 使用重置卡的响应结果。RawMsg 透传上游 message，
// 便于前端按 "指定的重置次数不可用，请刷新后重试" 等文案触发自动重新拉列表。
type GlmResetCardUseResult struct {
	Success bool   `json:"success"`
	Message string `json:"message,omitempty"`
}

// postGlmAPI 向智谱后端发送 POST 请求，Key 由后端注入，不会暴露给客户端；
// proxyURL 非空时请求经渠道代理发出。响应体不升级 ErrGlmKeyInvalid，由调用方按
// 业务语义解析（use 接口失败时需透传上游 msg）。
func postGlmAPI(baseURL, path, apiKey, proxyURL string, body []byte) ([]byte, error) {
	url := strings.TrimRight(baseURL, "/") + path

	req, err := http.NewRequest("POST", url, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Authorization", strings.TrimSpace(apiKey))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	// 与智谱官方前端保持一致，避免被识别为非浏览器请求导致返回风控页
	req.Header.Set("Referer", "https://bigmodel.cn/")
	req.Header.Set("Origin", "https://bigmodel.cn")

	client, err := newGlmHttpClient(proxyURL)
	if err != nil {
		return nil, fmt.Errorf("代理客户端创建失败: %w", err)
	}
	res, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	respBody, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, err
	}

	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d: %s", res.StatusCode, string(respBody))
	}

	return respBody, nil
}

// FetchGlmResetCards 拉取智谱套餐的重置卡列表，按 resetType 分组并标注优先卡。
// 接口路径、targetType 与智谱官方前端保持一致，上游 base URL 按套餐区域划分
// (glm-coding-plan → bigmodel.cn / glm-coding-plan-international → api.z.ai)。
func FetchGlmResetCards(apiKey string, planBaseURL string, proxyURL string) (*GlmResetCardListData, error) {
	apiBase := getGlmApiBase(planBaseURL)
	if apiBase == "" {
		return nil, fmt.Errorf("无法确定套餐对应的 API 地址")
	}

	path := fmt.Sprintf("%s?targetType=%s", glmResetCardListPath, url.QueryEscape(string(GlmResetCardTypePersonal)))
	body, err := fetchGlmAPI(apiBase, path, apiKey, proxyURL, nil)
	if err != nil {
		return nil, err
	}

	var resp glmResetCardListResp
	if err := jsonx.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("解析重置卡列表失败: %w", err)
	}

	data := &GlmResetCardListData{}
	if resp.Data != nil {
		data.FiveHourResets = normalizeGlmResetCards(resp.Data.FiveHourResets)
		data.WeekResets = normalizeGlmResetCards(resp.Data.WeekResets)
	}
	return data, nil
}

// normalizeGlmResetCards 按智谱官方前端逻辑归一化重置卡：
//   - 仅保留未使用或近 7 天已过期的卡（available=true 直接保留；
//     available=false 且过期 <=7 天保留为"已过期"，更早的丢弃）。
//   - 可用卡按 expireTime 升序排列，首张标记为 priority=true。
func normalizeGlmResetCards(cards []GlmResetCard) []GlmResetCard {
	if len(cards) == 0 {
		return []GlmResetCard{}
	}

	now := time.Now()
	maxExpiredWindow := 7 * 24 * time.Hour

	kept := make([]GlmResetCard, 0, len(cards))
	for _, card := range cards {
		if card.Available {
			kept = append(kept, card)
			continue
		}
		// 已过期卡按 expireTime 距离 now 是否 <=7 天决定保留与否
		expireTime, err := parseGlmResetCardExpireTime(card.ExpireTime)
		if err != nil {
			// 无法解析时按保守策略保留，避免误删用户可见的卡
			kept = append(kept, card)
			continue
		}
		if expireTime.Add(maxExpiredWindow).After(now) {
			kept = append(kept, card)
		}
	}

	// 可用卡按 expireTime 升序；首张可用卡标注 priority
	firstAvailable := -1
	for i, card := range kept {
		if card.Available {
			firstAvailable = i
			break
		}
	}
	if firstAvailable >= 0 {
		availableCards := make([]GlmResetCard, 0)
		for _, card := range kept {
			if card.Available {
				availableCards = append(availableCards, card)
			}
		}
		sortGlmResetCardsByExpire(availableCards)

		// 重写 kept 中可用卡顺序，保留首张 priority 标记
		writeIdx := 0
		for i := range kept {
			if kept[i].Available {
				kept[i] = availableCards[writeIdx]
				kept[i].Priority = writeIdx == 0
				writeIdx++
			} else {
				kept[i].Priority = false
			}
		}
	}
	return kept
}

// parseGlmResetCardExpireTime 兼容 "2006-01-02 15:04:05" 与 RFC3339 两种格式。
func parseGlmResetCardExpireTime(value string) (time.Time, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return time.Time{}, fmt.Errorf("expireTime 为空")
	}
	if t, err := time.ParseInLocation("2006-01-02 15:04:05", trimmed, time.Local); err == nil {
		return t, nil
	}
	if t, err := time.Parse(time.RFC3339, trimmed); err == nil {
		return t, nil
	}
	return time.Time{}, fmt.Errorf("expireTime 格式不支持: %s", trimmed)
}

// sortGlmResetCardsByExpire 按 expireTime 升序排序（稳定插入排序，规模小）。
func sortGlmResetCardsByExpire(cards []GlmResetCard) {
	for i := 1; i < len(cards); i++ {
		for j := i; j > 0; j-- {
			if cards[j].ExpireTime < cards[j-1].ExpireTime {
				cards[j], cards[j-1] = cards[j-1], cards[j]
			}
		}
	}
}

// GlmResetCardUseRequest controller 解析后的使用请求，requestId 由 service 生成。
type GlmResetCardUseRequest struct {
	TargetType GlmResetCardType `json:"targetType"`
	ResetType  GlmResetCardType `json:"resetType"`
	RecordId   int64            `json:"recordId"`
}

// UseGlmResetCard 调用智谱使用重置卡接口。targetType 固定 PERSONAL，
// requestId 由后端生成 uuid 注入，避免前端重复提交或注入不合法 ID。
func UseGlmResetCard(apiKey string, planBaseURL string, proxyURL string, req GlmResetCardUseRequest) (*GlmResetCardUseResult, error) {
	apiBase := getGlmApiBase(planBaseURL)
	if apiBase == "" {
		return nil, fmt.Errorf("无法确定套餐对应的 API 地址")
	}
	if req.RecordId <= 0 {
		return nil, fmt.Errorf("recordId 无效")
	}
	if _, ok := glmResetCardSourceKey[req.ResetType]; !ok {
		return nil, fmt.Errorf("resetType 不支持: %s", req.ResetType)
	}

	// recordId 与智谱官方前端一致按数字提交（官方前端从列表响应原样透传）
	payload := map[string]any{
		"targetType": string(GlmResetCardTypePersonal),
		"resetType":  string(req.ResetType),
		"recordId":   req.RecordId,
		"requestId":  uuid.NewString(),
	}
	body, err := jsonx.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("构造使用重置卡请求失败: %w", err)
	}

	respBody, err := postGlmAPI(apiBase, glmResetCardUsePath, apiKey, proxyURL, body)
	if err != nil {
		return nil, err
	}

	var resp struct {
		Code    int    `json:"code"`
		Msg     string `json:"msg"`
		Success bool   `json:"success"`
	}
	if err := jsonx.Unmarshal(respBody, &resp); err != nil {
		return nil, fmt.Errorf("解析使用重置卡响应失败: %w", err)
	}

	if !resp.Success {
		return &GlmResetCardUseResult{
			Success: false,
			Message: strings.TrimSpace(resp.Msg),
		}, nil
	}

	return &GlmResetCardUseResult{Success: true}, nil
}

// ============================================================================
// GLM 账户联系方式 (monthlyBill/monthlyBillDetails)
// ============================================================================

// glmContactBillPath 智谱月度账单明细接口路径。
// 账单明细中的 customerAcName 形如 "13121009002(郁有坤)"，携带账户手机号与姓名。
const glmContactBillPath = "/api/finance/monthlyBill/monthlyBillDetails"

// glmBrowserUserAgent 模拟 iPhone Safari 浏览器的 UA。智谱后台 biz/finance
// 系列接口（账单明细、账户报告等）要求浏览器 UA，缺失时上游会拒绝请求。
const glmBrowserUserAgent = "Mozilla/5.0 (iPhone; CPU iPhone OS 18_5 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/18.5 Mobile/15E148 Safari/604.1"

// GlmContactInfo 智谱账户联系信息，由月度账单明细的 customerAcName 解析而来。
// RawName 保留原始字符串，便于前端在格式解析失败时兜底展示。
type GlmContactInfo struct {
	Phone        string `json:"phone,omitempty"`
	Name         string `json:"name,omitempty"`
	RawName      string `json:"raw_name,omitempty"`
	BillingMonth string `json:"billing_month,omitempty"`
}

// getGlmContactApiBase 按套餐区域返回账单明细接口的 base URL：
// 国际套餐走 api.z.ai，国内套餐与未标识套餐的渠道走 bigmodel.cn。
func getGlmContactApiBase(planName string) string {
	if planName == "glm-coding-plan-international" {
		return "https://api.z.ai"
	}
	return "https://bigmodel.cn"
}

// parseGlmCustomerAcName 解析 "手机号(姓名)" 形式的 customerAcName。
// 不匹配该格式时整体作为姓名返回，避免信息丢失。
func parseGlmCustomerAcName(value string) (phone, name string) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return "", ""
	}
	if open := strings.IndexByte(trimmed, '('); open > 0 {
		if close := strings.LastIndexByte(trimmed, ')'); close > open {
			return strings.TrimSpace(trimmed[:open]), strings.TrimSpace(trimmed[open+1 : close])
		}
	}
	return "", trimmed
}

// FetchGlmContactInfo 查询智谱账户联系信息（月度账单明细中的 customerAcName）。
// billingMonth 由服务器时区取当前年月（YYYY-MM）；proxyURL 非空时请求经渠道代理发出。
// Key 由后端注入（Authorization: Bearer <key>），并强制携带浏览器 UA，不在浏览器直连智谱后台。
func FetchGlmContactInfo(apiKey string, planName string, proxyURL string) (*GlmContactInfo, error) {
	base := getGlmContactApiBase(planName)
	query := url.Values{}
	query.Set("billingMonth", time.Now().Format("2006-01"))
	target := base + glmContactBillPath + "?" + query.Encode()

	req, err := http.NewRequest("GET", target, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+strings.TrimSpace(apiKey))
	req.Header.Set("Accept", "*/*")
	req.Header.Set("User-Agent", glmBrowserUserAgent)

	client, err := newGlmHttpClient(proxyURL)
	if err != nil {
		return nil, fmt.Errorf("代理客户端创建失败: %w", err)
	}
	res, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	body, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, err
	}

	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d: %s", res.StatusCode, string(body))
	}

	if isGlmAuthFailure(body) {
		return nil, ErrGlmKeyInvalid
	}

	return parseGlmContactResponse(body)
}

// parseGlmContactResponse 解析月度账单明细响应，提取 customerAcName 中的
// 手机号与姓名。code/success 为指针字段，缺失视为未给出；显式失败时立即
// 报错并透传上游 msg。
func parseGlmContactResponse(body []byte) (*GlmContactInfo, error) {
	var resp struct {
		Code    *int   `json:"code"`
		Msg     string `json:"msg"`
		Success *bool  `json:"success"`
		Data    struct {
			BillingMonth   string `json:"billingMonth"`
			CustomerAcName string `json:"customerAcName"`
		} `json:"data"`
	}
	if err := jsonx.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("解析联系方式响应失败: %w", err)
	}
	if (resp.Code != nil && *resp.Code != 200) || (resp.Success != nil && !*resp.Success) {
		msg := strings.TrimSpace(resp.Msg)
		if msg == "" {
			msg = "智谱返回非成功状态"
		}
		code := "nil"
		if resp.Code != nil {
			code = strconv.Itoa(*resp.Code)
		}
		return nil, fmt.Errorf("%s (code=%s)", msg, code)
	}

	// 成功响应但缺少 customerAcName 属于上游异常状态，直接暴露错误，
	// 不返回空数据让前端弹空白窗。
	rawName := strings.TrimSpace(resp.Data.CustomerAcName)
	if rawName == "" {
		return nil, fmt.Errorf("上游未返回 customerAcName")
	}

	phone, name := parseGlmCustomerAcName(rawName)
	return &GlmContactInfo{
		Phone:        phone,
		Name:         name,
		RawName:      rawName,
		BillingMonth: resp.Data.BillingMonth,
	}, nil
}
