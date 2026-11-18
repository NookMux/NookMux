package planquota

import (
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/NookMux/NookMux/internal/i18n"
	httpclient "github.com/NookMux/NookMux/internal/infra/httpclient"
	"github.com/NookMux/NookMux/pkg/jsonx"
)

// MiniMaxPlanQuotaData MiniMax 套餐额度数据
type MiniMaxPlanQuotaData struct {
	PlanName   string        `json:"plan_name"`
	Tiers      []MiniMaxTier `json:"tiers"`
	Credential string        `json:"credential"` // "valid" | "expired" | "error"
}

// MiniMaxTier MiniMax 单个限额维度
type MiniMaxTier struct {
	Name       string  `json:"name"`                // "five_hour" | "weekly_limit"
	Percentage int     `json:"percentage"`          // 已用百分比 0-100
	Used       float64 `json:"used"`                // 已用量
	Limit      float64 `json:"limit"`               // 总量
	Remaining  float64 `json:"remaining"`           // 剩余量
	ResetsAt   string  `json:"resets_at,omitempty"` // 重置时间 RFC3339
	Status     string  `json:"status"`              // "充裕" | "适中" | "紧张"
}

// minimaxResp MiniMax coding_plan/remains 接口返回格式
type minimaxResp struct {
	BaseResp struct {
		StatusCode int64  `json:"status_code"`
		StatusMsg  string `json:"status_msg"`
	} `json:"base_resp"`
	ModelRemains []struct {
		CurrentIntervalTotalCount float64 `json:"current_interval_total_count"`
		CurrentIntervalUsageCount float64 `json:"current_interval_usage_count"`
		EndTime                   int64   `json:"end_time"`
		CurrentWeeklyTotalCount   float64 `json:"current_weekly_total_count"`
		CurrentWeeklyUsageCount   float64 `json:"current_weekly_usage_count"`
		WeeklyEndTime             int64   `json:"weekly_end_time"`
	} `json:"model_remains"`
}

// FetchMiniMaxPlanQuota 查询 MiniMax 套餐额度；proxyURL 非空时请求经渠道代理发出。
// planName: "minimax-coding-plan" 或 "minimax-coding-plan-international"
func FetchMiniMaxPlanQuota(apiKey string, planName string, proxyURL string) (*MiniMaxPlanQuotaData, error) {
	apiDomain := "api.minimaxi.com"
	if planName == "minimax-coding-plan-international" {
		apiDomain = "api.minimax.io"
	}
	url := fmt.Sprintf("https://%s/v1/api/openplatform/coding_plan/remains", apiDomain)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+strings.TrimSpace(apiKey))
	req.Header.Set("Content-Type", "application/json")

	baseClient, err := httpclient.GetHttpClientWithProxy(proxyURL)
	if err != nil {
		return nil, i18n.NewError(i18n.MsgChannelProxyClientCreateFailed, map[string]any{"Error": err.Error()}).WithCause(err)
	}
	client := &http.Client{
		Transport:     baseClient.Transport,
		CheckRedirect: baseClient.CheckRedirect,
		Timeout:       10 * time.Second,
	}
	res, err := client.Do(req)
	if err != nil {
		return nil, i18n.NewError(i18n.MsgChannelUpstreamRequestFailed, map[string]any{"Error": err.Error()}).WithCause(err)
	}
	defer res.Body.Close()

	if res.StatusCode == http.StatusUnauthorized || res.StatusCode == http.StatusForbidden {
		return &MiniMaxPlanQuotaData{
			PlanName:   planName,
			Credential: "expired",
		}, nil
	}

	if res.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(res.Body)
		return nil, i18n.NewError(i18n.MsgChannelUpstreamStatusError, map[string]any{"Status": res.StatusCode, "Body": string(body)})
	}

	body, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, i18n.NewError(i18n.MsgChannelResponseReadFailed, map[string]any{"Error": err.Error()}).WithCause(err)
	}

	var resp minimaxResp
	if err := jsonx.Unmarshal(body, &resp); err != nil {
		return &MiniMaxPlanQuotaData{
			PlanName:   planName,
			Credential: "error",
		}, nil
	}

	// 检查业务层错误
	if resp.BaseResp.StatusCode != 0 {
		return nil, i18n.NewErrorWithFallback(fmt.Sprintf("MiniMax API error (code %d): %s", resp.BaseResp.StatusCode, resp.BaseResp.StatusMsg), i18n.MsgPlanQuotaUpstreamCodeError, map[string]any{"Message": resp.BaseResp.StatusMsg, "Code": resp.BaseResp.StatusCode})
	}

	data := &MiniMaxPlanQuotaData{
		PlanName:   planName,
		Credential: "valid",
	}

	// 只取第一个模型（MiniMax-M*，主要编码模型）
	if len(resp.ModelRemains) == 0 {
		return data, nil
	}

	item := resp.ModelRemains[0]

	// 5 小时窗口限额
	// 注意：current_interval_usage_count 实际上是剩余量（字段命名有误导性）
	if item.CurrentIntervalTotalCount > 0 {
		used := item.CurrentIntervalTotalCount - item.CurrentIntervalUsageCount
		if used < 0 {
			used = 0
		}
		pct := int((used / item.CurrentIntervalTotalCount) * 100)
		if pct > 100 {
			pct = 100
		}
		resetsAt := ""
		if item.EndTime > 0 {
			resetsAt = normalizeUnixTimestamp(item.EndTime).Format(time.RFC3339)
		}
		data.Tiers = append(data.Tiers, MiniMaxTier{
			Name:       "five_hour",
			Percentage: pct,
			Used:       used,
			Limit:      item.CurrentIntervalTotalCount,
			Remaining:  item.CurrentIntervalUsageCount,
			ResetsAt:   resetsAt,
			Status:     getUsageStatus(pct),
		})
	}

	// 每周限额
	// 同样：current_weekly_usage_count 实际上是剩余量
	if item.CurrentWeeklyTotalCount > 0 {
		used := item.CurrentWeeklyTotalCount - item.CurrentWeeklyUsageCount
		if used < 0 {
			used = 0
		}
		pct := int((used / item.CurrentWeeklyTotalCount) * 100)
		if pct > 100 {
			pct = 100
		}
		resetsAt := ""
		if item.WeeklyEndTime > 0 {
			resetsAt = normalizeUnixTimestamp(item.WeeklyEndTime).Format(time.RFC3339)
		}
		data.Tiers = append(data.Tiers, MiniMaxTier{
			Name:       "weekly_limit",
			Percentage: pct,
			Used:       used,
			Limit:      item.CurrentWeeklyTotalCount,
			Remaining:  item.CurrentWeeklyUsageCount,
			ResetsAt:   resetsAt,
			Status:     getUsageStatus(pct),
		})
	}

	return data, nil
}
