package perfmetrics

import (
	"fmt"

	"github.com/NookMux/NookMux/internal/config/manager"
)

// PerfMetricsSetting 模型性能指标配置，注册键 perf_metrics_setting。
type PerfMetricsSetting struct {
	// Enabled 是否在中继请求边界采集中继性能样本
	Enabled bool `json:"enabled"`
	// FlushInterval 内存热桶落库间隔（分钟），最小 1
	FlushInterval int `json:"flush_interval"`
	// BucketTime 聚合时间桶粒度：minute / 5min / hour
	BucketTime string `json:"bucket_time"`
	// RetentionDays 桶数据保留天数，0 表示永久保留
	RetentionDays int `json:"retention_days"`
}

var perfMetricsSetting = PerfMetricsSetting{
	Enabled:       true,
	FlushInterval: 5,
	BucketTime:    "hour",
	RetentionDays: 30,
}

func init() {
	manager.GlobalConfig.Register("perf_metrics_setting", &perfMetricsSetting)
}

// GetPerfMetricsSetting 获取模型性能指标配置实例。
func GetPerfMetricsSetting() *PerfMetricsSetting {
	return &perfMetricsSetting
}

// BucketTimeSeconds 把桶粒度配置映射为秒，非法取值返回错误。
func BucketTimeSeconds(value string) (int, error) {
	switch value {
	case "minute":
		return 60, nil
	case "5min":
		return 300, nil
	case "hour":
		return 3600, nil
	default:
		return 0, fmt.Errorf("invalid bucket_time: %s", value)
	}
}
