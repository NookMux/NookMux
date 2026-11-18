package planquota

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/NookMux/NookMux/internal/i18n"
	httpclient "github.com/NookMux/NookMux/internal/infra/httpclient"
	"github.com/NookMux/NookMux/pkg/jsonx"
)

// KimiPlanQuotaData Kimi 套餐额度数据
type KimiPlanQuotaData struct {
	PlanName   string     `json:"plan_name"`
	Tiers      []KimiTier `json:"tiers"`
	Credential string     `json:"credential"` // "valid" | "expired" | "error"
}

// KimiTier Kimi 单个限额维度
type KimiTier struct {
	Name       string  `json:"name"`                // "five_hour" | "weekly_limit"
	Percentage int     `json:"percentage"`          // 已用百分比 0-100
	Used       float64 `json:"used"`                // 已用量
	Limit      float64 `json:"limit"`               // 总量
	Remaining  float64 `json:"remaining"`           // 剩余量
	ResetsAt   string  `json:"resets_at,omitempty"` // 重置时间 RFC3339
	Status     string  `json:"status"`              // "充裕" | "适中" | "紧张"
}

// kimiUsageResp Kimi /coding/v1/usages 接口返回格式
type kimiUsageResp struct {
	Limits []struct {
		Detail struct {
			Limit     json.Number `json:"limit"`
			Remaining json.Number `json:"remaining"`
			ResetTime interface{} `json:"resetTime"`
		} `json:"detail"`
	} `json:"limits"`
	Usage struct {
		Limit     json.Number `json:"limit"`
		Remaining json.Number `json:"remaining"`
		ResetTime interface{} `json:"resetTime"`
	} `json:"usage"`
}

// parseFloatNumber 兼容 JSON 中数字可能编码为字符串的情况
func parseFloatNumber(n json.Number) float64 {
	if n.String() == "" {
		return 0
	}
	f, err := n.Float64()
	if err == nil {
		return f
	}
	// 回退：尝试直接解析字符串
	parsed, err := strconv.ParseFloat(string(n), 64)
	if err != nil {
		return 0
	}
	return parsed
}

// FetchKimiPlanQuota 查询 Kimi 套餐额度；proxyURL 非空时请求经渠道代理发出。
func FetchKimiPlanQuota(apiKey string, proxyURL string) (*KimiPlanQuotaData, error) {
	url := "https://api.kimi.com/coding/v1/usages"

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+strings.TrimSpace(apiKey))
	req.Header.Set("Accept", "application/json")

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
		return &KimiPlanQuotaData{
			PlanName:   "kimi-coding-plan",
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

	var resp kimiUsageResp
	if err := jsonx.Unmarshal(body, &resp); err != nil {
		return &KimiPlanQuotaData{
			PlanName:   "kimi-coding-plan",
			Credential: "error",
		}, nil
	}

	data := &KimiPlanQuotaData{
		PlanName:   "kimi-coding-plan",
		Credential: "valid",
	}

	// 解析 5 小时窗口限额 (limits 数组)
	for _, limit := range resp.Limits {
		d := limit.Detail
		lim := parseFloatNumber(d.Limit)
		rem := parseFloatNumber(d.Remaining)
		if lim <= 0 {
			continue
		}
		used := lim - rem
		if used < 0 {
			used = 0
		}
		pct := int((used / lim) * 100)
		if pct > 100 {
			pct = 100
		}
		resetStr := parseKimiResetTime(d.ResetTime)
		data.Tiers = append(data.Tiers, KimiTier{
			Name:       "five_hour",
			Percentage: pct,
			Used:       used,
			Limit:      lim,
			Remaining:  rem,
			ResetsAt:   resetStr,
			Status:     getUsageStatus(pct),
		})
	}

	// 解析每周限额 (usage 对象)
	weeklyLim := parseFloatNumber(resp.Usage.Limit)
	weeklyRem := parseFloatNumber(resp.Usage.Remaining)
	if weeklyLim > 0 {
		used := weeklyLim - weeklyRem
		if used < 0 {
			used = 0
		}
		pct := int((used / weeklyLim) * 100)
		if pct > 100 {
			pct = 100
		}
		resetStr := parseKimiResetTime(resp.Usage.ResetTime)
		data.Tiers = append(data.Tiers, KimiTier{
			Name:       "weekly_limit",
			Percentage: pct,
			Used:       used,
			Limit:      weeklyLim,
			Remaining:  weeklyRem,
			ResetsAt:   resetStr,
			Status:     getUsageStatus(pct),
		})
	}

	return data, nil
}

// parseKimiResetTime 兼容 Kimi resetTime 的多种格式（字符串、数字时间戳）
func parseKimiResetTime(raw interface{}) string {
	if raw == nil {
		return ""
	}

	// 尝试作为字符串解析
	if s, ok := raw.(string); ok {
		s = strings.TrimSpace(s)
		if s == "" {
			return ""
		}
		// 尝试解析为时间戳字符串
		if isNumericTimestamp(s) {
			parsed, err := strconv.ParseInt(s, 10, 64)
			if err == nil {
				return normalizeUnixTimestamp(parsed).Format(time.RFC3339)
			}
		}
		// 尝试 RFC3339 或 ISO8601 格式
		t, err := time.Parse(time.RFC3339, s)
		if err == nil {
			return t.Format(time.RFC3339)
		}
		return s
	}

	// 尝试作为数字解析 (float64 from JSON)
	if f, ok := raw.(float64); ok {
		return normalizeUnixTimestamp(int64(f)).Format(time.RFC3339)
	}

	return fmt.Sprintf("%v", raw)
}

// getUsageStatus 根据百分比返回使用状态
func getUsageStatus(percentage int) string {
	if percentage >= 80 {
		return "紧张"
	}
	if percentage >= 50 {
		return "适中"
	}
	return "充裕"
}
