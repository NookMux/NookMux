// Package perfmetricsstore 承载模型性能指标（延迟 / 首字延迟 / 成功率 / 吞吐）
// 的聚合桶持久化：内存热桶按 (model × group × 时间桶) 累加，后台定时把已完成
// 桶以「等值点查 → 条件累加 → 不存在则插入」的六库兼容模式增量落库，查询时
// 合并数据库行与内存热桶得到实时结果。
package perfmetricsstore

import (
	"errors"
	"fmt"
	"math"
	"sort"
	"sync"
	"time"

	"github.com/NookMux/NookMux/internal/common"
	dbstore "github.com/NookMux/NookMux/internal/store/db"

	"gorm.io/gorm"
)

// PerfMetric 模型性能指标桶行。只存可增量累加的原始和，均值一律在查询层
// 做除法得出，保证多实例各自落库后在数据库端按唯一键合流不重不漏。
type PerfMetric struct {
	Id             int    `json:"id"`
	ModelName      string `json:"model_name" gorm:"size:128;uniqueIndex:uk_perf_model_group_bucket,priority:1"`
	Group          string `json:"group" gorm:"size:64;uniqueIndex:uk_perf_model_group_bucket,priority:2"`
	BucketStart    int64  `json:"bucket_start" gorm:"bigint;uniqueIndex:uk_perf_model_group_bucket,priority:3;index:idx_perf_bucket_ts"`
	RequestCount   int64  `json:"-" gorm:"default:0"`
	SuccessCount   int64  `json:"-" gorm:"default:0"`
	TotalLatencyMs int64  `json:"-" gorm:"default:0"`
	TotalFrtMs     int64  `json:"-" gorm:"default:0"`
	FrtCount       int64  `json:"-" gorm:"default:0"`
	OutputTokens   int64  `json:"-" gorm:"default:0"`
	GenerationMs   int64  `json:"-" gorm:"default:0"`
}

// DefaultGroup 分组为空的请求（未命中任何分组配置）归入的桶分组名。
const DefaultGroup = "default"

// Sample 单次请求的采样值。FirstTokenMs 为 -1 表示非流式或未发出首帧，
// 该样本不计入首字延迟统计。
type Sample struct {
	Success      bool
	LatencyMs    int64
	FirstTokenMs int64
	OutputTokens int64
	GenerationMs int64
}

// 运行时配置由 option 同步路径（handleConfigUpdate）维护，store 层不反向
// 依赖 config 包，与 quota_data 管道的既定模式一致。
var (
	runtimeMu         sync.RWMutex
	perfEnabled             = true
	perfFlushInterval       = 5 * time.Minute
	perfBucketSecs    int64 = 3600
	perfRetentionDays       = 30
)

// SyncRuntimeConfig 由 option 同步路径调用，运行时生效采集开关与管道参数。
func SyncRuntimeConfig(enabled bool, flushIntervalMinutes, bucketSeconds, retentionDays int) {
	interval := time.Duration(flushIntervalMinutes) * time.Minute
	if interval < time.Minute {
		interval = time.Minute
	}
	if bucketSeconds < 60 {
		bucketSeconds = 60
	}
	runtimeMu.Lock()
	perfEnabled = enabled
	perfFlushInterval = interval
	perfBucketSecs = int64(bucketSeconds)
	perfRetentionDays = retentionDays
	runtimeMu.Unlock()
}

// Enabled 返回性能指标采集是否启用。
func Enabled() bool {
	runtimeMu.RLock()
	defer runtimeMu.RUnlock()
	return perfEnabled
}

func flushInterval() time.Duration {
	runtimeMu.RLock()
	defer runtimeMu.RUnlock()
	return perfFlushInterval
}

func bucketSeconds() int64 {
	runtimeMu.RLock()
	defer runtimeMu.RUnlock()
	return perfBucketSecs
}

func retentionDays() int {
	runtimeMu.RLock()
	defer runtimeMu.RUnlock()
	return perfRetentionDays
}

type bucketKey struct {
	model       string
	group       string
	bucketStart int64
}

type bucketCounters struct {
	requestCount   int64
	successCount   int64
	totalLatencyMs int64
	totalFrtMs     int64
	frtCount       int64
	outputTokens   int64
	generationMs   int64
}

func (c *bucketCounters) add(sample Sample) {
	c.requestCount++
	if sample.Success {
		c.successCount++
	}
	c.totalLatencyMs += sample.LatencyMs
	if sample.FirstTokenMs >= 0 {
		c.totalFrtMs += sample.FirstTokenMs
		c.frtCount++
	}
	c.outputTokens += sample.OutputTokens
	if sample.GenerationMs > 0 {
		c.generationMs += sample.GenerationMs
	}
}

var (
	hotMu     sync.Mutex
	hotBucket = map[bucketKey]*bucketCounters{}
)

// Record 记录一次请求采样到内存热桶。采集关闭或模型名为空时直接丢弃。
func Record(modelName, group string, ts time.Time, sample Sample) {
	if !Enabled() || modelName == "" {
		return
	}
	if group == "" {
		group = DefaultGroup
	}
	secs := bucketSeconds()
	unix := ts.Unix()
	k := bucketKey{model: modelName, group: group, bucketStart: unix - unix%secs}

	hotMu.Lock()
	defer hotMu.Unlock()
	c, ok := hotBucket[k]
	if !ok {
		c = &bucketCounters{}
		hotBucket[k] = c
	}
	c.add(sample)
}

// UpdatePerfMetrics 后台落库循环：按刷新间隔把已完成桶增量落库，按小时级
// 步长执行保留期清理。禁用采集时以大步长睡眠保持对重新启用的感知。
func UpdatePerfMetrics() {
	lastFlush := time.Time{}
	lastCleanup := time.Time{}
	for {
		interval := flushInterval()
		if Enabled() && time.Since(lastFlush) >= interval {
			SavePerfMetricsCache()
			lastFlush = time.Now()
		}
		if Enabled() && (lastCleanup.IsZero() || time.Since(lastCleanup) >= time.Hour) {
			cleanupExpiredMetrics()
			lastCleanup = time.Now()
		}

		sleepDuration := time.Second
		if !Enabled() {
			sleepDuration = 10 * time.Second
		} else if interval > 5*time.Minute {
			sleepDuration = time.Minute
		}
		time.Sleep(sleepDuration)
	}
}

// SavePerfMetricsCache 把已完成的桶（起点早于当前桶）换出并落库。
// 运行期不落当前桶，避免同一行被反复 upsert。
func SavePerfMetricsCache() {
	current := time.Now().Unix()
	current -= current % bucketSeconds()

	hotMu.Lock()
	pending := make(map[bucketKey]*bucketCounters, len(hotBucket))
	for k, v := range hotBucket {
		if k.bucketStart < current {
			pending[k] = v
			delete(hotBucket, k)
		}
	}
	hotMu.Unlock()
	flushBuckets(pending)
}

// FlushPerfMetrics 停机兜底：把内存中全部桶（含当前未完成桶）落库后清空。
func FlushPerfMetrics() {
	hotMu.Lock()
	pending := hotBucket
	hotBucket = map[bucketKey]*bucketCounters{}
	hotMu.Unlock()
	flushBuckets(pending)
}

func flushBuckets(pending map[bucketKey]*bucketCounters) {
	if len(pending) == 0 {
		return
	}
	saved := 0
	for k, c := range pending {
		if err := upsertPerfMetric(k, *c); err != nil {
			// 落库失败必须回灌内存桶等待下轮重试，不允许静默丢样本。
			common.SysError(fmt.Sprintf("perf metrics upsert failed (model=%s group=%s bucket=%d): %s", k.model, k.group, k.bucketStart, err.Error()))
			hotMu.Lock()
			existing, ok := hotBucket[k]
			if !ok {
				hotBucket[k] = c
			} else {
				existing.requestCount += c.requestCount
				existing.successCount += c.successCount
				existing.totalLatencyMs += c.totalLatencyMs
				existing.totalFrtMs += c.totalFrtMs
				existing.frtCount += c.frtCount
				existing.outputTokens += c.outputTokens
				existing.generationMs += c.generationMs
			}
			hotMu.Unlock()
			continue
		}
		saved++
	}
	if saved > 0 {
		common.SysLog(fmt.Sprintf("perf metrics flushed: %d buckets", saved))
	}
}

// upsertPerfMetric 按唯一键等值点查：命中则条件累加，未命中则插入。
// 六库（含 DM8/金仓）下不使用 OnConflict DoUpdates，与 quota_data 管道同模式。
func upsertPerfMetric(k bucketKey, c bucketCounters) error {
	existing := &PerfMetric{}
	err := dbstore.DB.Table("perf_metrics").
		Where("model_name = ? AND "+dbstore.CommonGroupCol+" = ? AND bucket_start = ?", k.model, k.group, k.bucketStart).
		First(existing).Error
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}
	if existing.Id > 0 {
		return dbstore.DB.Model(&PerfMetric{}).Where("id = ?", existing.Id).Updates(map[string]interface{}{
			"request_count":    gorm.Expr("request_count + ?", c.requestCount),
			"success_count":    gorm.Expr("success_count + ?", c.successCount),
			"total_latency_ms": gorm.Expr("total_latency_ms + ?", c.totalLatencyMs),
			"total_frt_ms":     gorm.Expr("total_frt_ms + ?", c.totalFrtMs),
			"frt_count":        gorm.Expr("frt_count + ?", c.frtCount),
			"output_tokens":    gorm.Expr("output_tokens + ?", c.outputTokens),
			"generation_ms":    gorm.Expr("generation_ms + ?", c.generationMs),
		}).Error
	}
	return dbstore.DB.Create(&PerfMetric{
		ModelName:      k.model,
		Group:          k.group,
		BucketStart:    k.bucketStart,
		RequestCount:   c.requestCount,
		SuccessCount:   c.successCount,
		TotalLatencyMs: c.totalLatencyMs,
		TotalFrtMs:     c.totalFrtMs,
		FrtCount:       c.frtCount,
		OutputTokens:   c.outputTokens,
		GenerationMs:   c.generationMs,
	}).Error
}

func cleanupExpiredMetrics() {
	days := retentionDays()
	if days <= 0 {
		return
	}
	cutoff := time.Now().AddDate(0, 0, -days).Unix()
	total, err := DeletePerfMetricsBefore(cutoff, 500)
	if err != nil {
		common.SysError(fmt.Sprintf("perf metrics cleanup failed: %s", err.Error()))
		return
	}
	if total > 0 {
		common.SysLog(fmt.Sprintf("perf metrics cleanup removed %d expired buckets", total))
	}
}

// DeletePerfMetricsBefore 分批删除 bucket_start 早于 cutoff 的桶行。
func DeletePerfMetricsBefore(cutoff int64, limit int) (int64, error) {
	if limit <= 0 {
		limit = 500
	}
	var total int64
	for {
		result := dbstore.DB.Where("bucket_start < ?", cutoff).Limit(limit).Delete(&PerfMetric{})
		if result.Error != nil {
			return total, result.Error
		}
		total += result.RowsAffected
		if result.RowsAffected < int64(limit) {
			break
		}
	}
	return total, nil
}

// BucketAgg 单个 (model × group × bucket) 聚合值，由数据库行与内存热桶合并而来。
type BucketAgg struct {
	ModelName      string
	Group          string
	BucketStart    int64
	RequestCount   int64
	SuccessCount   int64
	TotalLatencyMs int64
	TotalFrtMs     int64
	FrtCount       int64
	OutputTokens   int64
	GenerationMs   int64
}

const bucketSumSelect = "model_name, %s, bucket_start, sum(request_count) as request_count, sum(success_count) as success_count, sum(total_latency_ms) as total_latency_ms, sum(total_frt_ms) as total_frt_ms, sum(frt_count) as frt_count, sum(output_tokens) as output_tokens, sum(generation_ms) as generation_ms"

// GetPerfMetricBuckets 查询单模型在窗口内、指定分组集合的逐桶聚合
// （数据库行 + 内存热桶合并），按桶起点与分组名排序返回。
func GetPerfMetricBuckets(modelName string, groups []string, startTs, endTs int64) ([]BucketAgg, error) {
	if modelName == "" || len(groups) == 0 {
		return nil, nil
	}
	var rows []BucketAgg
	err := dbstore.DB.Table("perf_metrics").
		Select(fmt.Sprintf(bucketSumSelect, dbstore.CommonGroupCol)).
		Where("model_name = ? AND bucket_start >= ? AND bucket_start <= ? AND "+dbstore.CommonGroupCol+" IN ?", modelName, startTs, endTs, groups).
		Group(fmt.Sprintf("model_name, %s, bucket_start", dbstore.CommonGroupCol)).
		Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	merged := mergeHotBuckets(rows, func(k bucketKey) bool {
		return k.model == modelName && k.bucketStart >= startTs && k.bucketStart <= endTs && containsGroup(groups, k.group)
	})
	sortBuckets(merged)
	return merged, nil
}

// ModelAgg 窗口内单模型的跨分组聚合值。
type ModelAgg struct {
	ModelName      string
	RequestCount   int64
	SuccessCount   int64
	TotalLatencyMs int64
	TotalFrtMs     int64
	FrtCount       int64
	OutputTokens   int64
	GenerationMs   int64
}

// GetAllPerfModelAggs 查询窗口内全部模型（限定分组集合）的聚合值，
// 按请求量降序返回，供模型广场摘要接口使用。
func GetAllPerfModelAggs(groups []string, startTs, endTs int64) ([]ModelAgg, error) {
	if len(groups) == 0 {
		return nil, nil
	}
	var rows []ModelAgg
	err := dbstore.DB.Table("perf_metrics").
		Select("model_name, sum(request_count) as request_count, sum(success_count) as success_count, sum(total_latency_ms) as total_latency_ms, sum(total_frt_ms) as total_frt_ms, sum(frt_count) as frt_count, sum(output_tokens) as output_tokens, sum(generation_ms) as generation_ms").
		Where("bucket_start >= ? AND bucket_start <= ? AND "+dbstore.CommonGroupCol+" IN ?", startTs, endTs, groups).
		Group("model_name").
		Scan(&rows).Error
	if err != nil {
		return nil, err
	}

	groupSet := make(map[string]struct{}, len(groups))
	for _, g := range groups {
		groupSet[g] = struct{}{}
	}
	hotMu.Lock()
	for k, v := range hotBucket {
		if k.bucketStart < startTs || k.bucketStart > endTs {
			continue
		}
		if _, ok := groupSet[k.group]; !ok {
			continue
		}
		var row *ModelAgg
		for i := range rows {
			if rows[i].ModelName == k.model {
				row = &rows[i]
				break
			}
		}
		if row == nil {
			rows = append(rows, ModelAgg{ModelName: k.model})
			row = &rows[len(rows)-1]
		}
		row.RequestCount += v.requestCount
		row.SuccessCount += v.successCount
		row.TotalLatencyMs += v.totalLatencyMs
		row.TotalFrtMs += v.totalFrtMs
		row.FrtCount += v.frtCount
		row.OutputTokens += v.outputTokens
		row.GenerationMs += v.generationMs
	}
	hotMu.Unlock()

	sort.SliceStable(rows, func(i, j int) bool {
		return rows[i].RequestCount > rows[j].RequestCount
	})
	return rows, nil
}

// PerfSummary 窗口汇总指标。SuccessRate 为 0-100 两位小数；AvgTps 为
// 输出 token 数 / 生成阶段秒数；无样本时上层应展示 null 而非零值。
type PerfSummary struct {
	AvgLatencyMs int64   `json:"avg_latency_ms"`
	SuccessRate  float64 `json:"success_rate"`
	AvgTps       float64 `json:"avg_tps"`
}

// BuildPerfSummary 由聚合行计算窗口汇总，无请求样本时返回 nil。
func BuildPerfSummary(requestCount, successCount, totalLatencyMs, outputTokens, generationMs int64) *PerfSummary {
	if requestCount == 0 {
		return nil
	}
	summary := &PerfSummary{
		AvgLatencyMs: totalLatencyMs / requestCount,
		SuccessRate:  math.Round(float64(successCount)/float64(requestCount)*10000) / 100,
	}
	if generationMs > 0 {
		summary.AvgTps = float64(outputTokens) / (float64(generationMs) / 1000)
	}
	return summary
}

func mergeHotBuckets(rows []BucketAgg, match func(bucketKey) bool) []BucketAgg {
	index := make(map[bucketKey]*BucketAgg, len(rows))
	for i := range rows {
		index[bucketKey{model: rows[i].ModelName, group: rows[i].Group, bucketStart: rows[i].BucketStart}] = &rows[i]
	}
	hotMu.Lock()
	for k, v := range hotBucket {
		if !match(k) {
			continue
		}
		row, ok := index[k]
		if !ok {
			rows = append(rows, BucketAgg{ModelName: k.model, Group: k.group, BucketStart: k.bucketStart})
			row = &rows[len(rows)-1]
			index[k] = row
		}
		row.RequestCount += v.requestCount
		row.SuccessCount += v.successCount
		row.TotalLatencyMs += v.totalLatencyMs
		row.TotalFrtMs += v.totalFrtMs
		row.FrtCount += v.frtCount
		row.OutputTokens += v.outputTokens
		row.GenerationMs += v.generationMs
	}
	hotMu.Unlock()
	return rows
}

func containsGroup(groups []string, group string) bool {
	for _, g := range groups {
		if g == group {
			return true
		}
	}
	return false
}

func sortBuckets(rows []BucketAgg) {
	sort.SliceStable(rows, func(i, j int) bool {
		if rows[i].BucketStart != rows[j].BucketStart {
			return rows[i].BucketStart < rows[j].BucketStart
		}
		return rows[i].Group < rows[j].Group
	})
}
