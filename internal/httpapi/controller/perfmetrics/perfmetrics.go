package perfmetricscontroller

import (
	"math"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/NookMux/NookMux/internal/config/ratio"
	"github.com/NookMux/NookMux/internal/httpapi"
	"github.com/NookMux/NookMux/internal/i18n"
	perfmetricsstore "github.com/NookMux/NookMux/internal/store/perfmetrics"
	"github.com/gin-gonic/gin"
)

// seriesSchema 响应结构指纹：series / groups 字段结构变更时更新，
// 供客户端识别缓存失效。
const seriesSchema = "v1"

const (
	defaultQueryHours = 24
	maxQueryHours     = 24 * 30
)

type seriesPoint struct {
	Ts           int64   `json:"ts"`
	AvgTtftMs    int64   `json:"avg_ttft_ms"`
	AvgLatencyMs int64   `json:"avg_latency_ms"`
	SuccessRate  float64 `json:"success_rate"`
	AvgTps       float64 `json:"avg_tps"`
}

type groupResult struct {
	Group        string        `json:"group"`
	AvgTtftMs    int64         `json:"avg_ttft_ms"`
	AvgLatencyMs int64         `json:"avg_latency_ms"`
	SuccessRate  float64       `json:"success_rate"`
	AvgTps       float64       `json:"avg_tps"`
	Series       []seriesPoint `json:"series"`
}

type modelSummary struct {
	ModelName    string  `json:"model_name"`
	AvgLatencyMs int64   `json:"avg_latency_ms"`
	SuccessRate  float64 `json:"success_rate"`
	AvgTps       float64 `json:"avg_tps"`
}

// allowedGroups 返回可对外暴露性能数据的分组集合（组倍率表中的分组），
// 防止未在组倍率配置中的内部或异常分组名外泄。
func allowedGroups() []string {
	ratioCopy := ratio.GetGroupRatioCopy()
	groups := make([]string, 0, len(ratioCopy))
	for g := range ratioCopy {
		groups = append(groups, g)
	}
	sort.Strings(groups)
	return groups
}

func parseQueryWindow(c *gin.Context) (int64, int64, bool) {
	hours := defaultQueryHours
	if raw := c.Query("hours"); raw != "" {
		v, err := strconv.Atoi(raw)
		if err != nil || v < 1 || v > maxQueryHours {
			httpapi.ApiErrorI18n(c, i18n.MsgPerfMetricsHoursInvalid)
			return 0, 0, false
		}
		hours = v
	}
	end := time.Now().Unix()
	return end - int64(hours)*3600, end, true
}

// GetPerfMetrics 返回单模型在查询窗口内的性能明细：模型级汇总与逐桶序列、
// 各分组聚合与分组级逐桶序列。
func GetPerfMetrics(c *gin.Context) {
	modelName := strings.TrimSpace(c.Query("model"))
	if modelName == "" {
		httpapi.ApiErrorI18n(c, i18n.MsgPerfMetricsModelRequired)
		return
	}
	windowStart, windowEnd, ok := parseQueryWindow(c)
	if !ok {
		return
	}
	groups := allowedGroups()

	var rows []perfmetricsstore.BucketAgg
	if len(groups) > 0 {
		var err error
		rows, err = perfmetricsstore.GetPerfMetricBuckets(modelName, groups, windowStart, windowEnd)
		if err != nil {
			c.JSON(http.StatusOK, gin.H{"success": false, "message": err.Error()})
			return
		}
	}

	bucketTotals := make(map[int64]*perfmetricsstore.BucketAgg)
	groupTotals := make(map[string]*perfmetricsstore.BucketAgg)
	groupBuckets := make(map[string]map[int64]*perfmetricsstore.BucketAgg)
	for i := range rows {
		row := &rows[i]
		addToBucketTotals(bucketTotals, row.BucketStart, row)
		addToGroupTotals(groupTotals, row.Group, row)
		gb, ok := groupBuckets[row.Group]
		if !ok {
			gb = make(map[int64]*perfmetricsstore.BucketAgg)
			groupBuckets[row.Group] = gb
		}
		addToBucketTotals(gb, row.BucketStart, row)
	}

	var req, succ, latency, tokens, generation int64
	for _, row := range groupTotals {
		req += row.RequestCount
		succ += row.SuccessCount
		latency += row.TotalLatencyMs
		tokens += row.OutputTokens
		generation += row.GenerationMs
	}

	httpapi.ApiSuccess(c, gin.H{
		"model_name":    modelName,
		"series_schema": seriesSchema,
		"summary":       perfmetricsstore.BuildPerfSummary(req, succ, latency, tokens, generation),
		"series":        buildSeries(bucketTotals),
		"window_start":  windowStart,
		"window_end":    windowEnd,
		"groups":        buildGroups(groupTotals, groupBuckets),
	})
}

// GetPerfMetricsSummary 返回查询窗口内全站汇总与各模型性能摘要（按请求量降序）。
func GetPerfMetricsSummary(c *gin.Context) {
	windowStart, windowEnd, ok := parseQueryWindow(c)
	if !ok {
		return
	}
	groups := allowedGroups()

	var modelAggs []perfmetricsstore.ModelAgg
	if len(groups) > 0 {
		var err error
		modelAggs, err = perfmetricsstore.GetAllPerfModelAggs(groups, windowStart, windowEnd)
		if err != nil {
			c.JSON(http.StatusOK, gin.H{"success": false, "message": err.Error()})
			return
		}
	}

	var req, succ, latency, tokens, generation int64
	models := make([]modelSummary, 0, len(modelAggs))
	for _, agg := range modelAggs {
		summary := perfmetricsstore.BuildPerfSummary(agg.RequestCount, agg.SuccessCount, agg.TotalLatencyMs, agg.OutputTokens, agg.GenerationMs)
		if summary == nil {
			continue
		}
		models = append(models, modelSummary{
			ModelName:    agg.ModelName,
			AvgLatencyMs: summary.AvgLatencyMs,
			SuccessRate:  summary.SuccessRate,
			AvgTps:       summary.AvgTps,
		})
		req += agg.RequestCount
		succ += agg.SuccessCount
		latency += agg.TotalLatencyMs
		tokens += agg.OutputTokens
		generation += agg.GenerationMs
	}

	httpapi.ApiSuccess(c, gin.H{
		"summary":      perfmetricsstore.BuildPerfSummary(req, succ, latency, tokens, generation),
		"window_start": windowStart,
		"window_end":   windowEnd,
		"models":       models,
	})
}

func addToBucketTotals(target map[int64]*perfmetricsstore.BucketAgg, ts int64, row *perfmetricsstore.BucketAgg) {
	total, ok := target[ts]
	if !ok {
		clone := *row
		target[ts] = &clone
		return
	}
	total.RequestCount += row.RequestCount
	total.SuccessCount += row.SuccessCount
	total.TotalLatencyMs += row.TotalLatencyMs
	total.TotalFrtMs += row.TotalFrtMs
	total.FrtCount += row.FrtCount
	total.OutputTokens += row.OutputTokens
	total.GenerationMs += row.GenerationMs
}

func addToGroupTotals(target map[string]*perfmetricsstore.BucketAgg, group string, row *perfmetricsstore.BucketAgg) {
	total, ok := target[group]
	if !ok {
		clone := *row
		target[group] = &clone
		return
	}
	total.RequestCount += row.RequestCount
	total.SuccessCount += row.SuccessCount
	total.TotalLatencyMs += row.TotalLatencyMs
	total.TotalFrtMs += row.TotalFrtMs
	total.FrtCount += row.FrtCount
	total.OutputTokens += row.OutputTokens
	total.GenerationMs += row.GenerationMs
}

func buildSeries(buckets map[int64]*perfmetricsstore.BucketAgg) []seriesPoint {
	points := make([]seriesPoint, 0, len(buckets))
	tsList := make([]int64, 0, len(buckets))
	for ts := range buckets {
		tsList = append(tsList, ts)
	}
	sort.Slice(tsList, func(i, j int) bool { return tsList[i] < tsList[j] })
	for _, ts := range tsList {
		points = append(points, buildSeriesPoint(ts, buckets[ts]))
	}
	return points
}

func buildSeriesPoint(ts int64, agg *perfmetricsstore.BucketAgg) seriesPoint {
	point := seriesPoint{Ts: ts}
	if agg.RequestCount > 0 {
		point.AvgLatencyMs = agg.TotalLatencyMs / agg.RequestCount
		point.SuccessRate = math.Round(float64(agg.SuccessCount)/float64(agg.RequestCount)*10000) / 100
	}
	if agg.FrtCount > 0 {
		point.AvgTtftMs = agg.TotalFrtMs / agg.FrtCount
	}
	if agg.GenerationMs > 0 {
		point.AvgTps = float64(agg.OutputTokens) / (float64(agg.GenerationMs) / 1000)
	}
	return point
}

func buildGroups(groupTotals map[string]*perfmetricsstore.BucketAgg, groupBuckets map[string]map[int64]*perfmetricsstore.BucketAgg) []groupResult {
	names := make([]string, 0, len(groupTotals))
	for name := range groupTotals {
		names = append(names, name)
	}
	sort.Strings(names)

	results := make([]groupResult, 0, len(names))
	for _, name := range names {
		total := groupTotals[name]
		result := groupResult{Group: name, Series: buildSeries(groupBuckets[name])}
		if total.RequestCount > 0 {
			result.AvgLatencyMs = total.TotalLatencyMs / total.RequestCount
			result.SuccessRate = math.Round(float64(total.SuccessCount)/float64(total.RequestCount)*10000) / 100
		}
		if total.FrtCount > 0 {
			result.AvgTtftMs = total.TotalFrtMs / total.FrtCount
		}
		if total.GenerationMs > 0 {
			result.AvgTps = float64(total.OutputTokens) / (float64(total.GenerationMs) / 1000)
		}
		results = append(results, result)
	}
	return results
}
