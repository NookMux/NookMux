package dashboard

import (
	"github.com/NookMux/NookMux/internal/config/manager"
)

// DashboardConfig 数据仪表板配置结构
type DashboardConfig struct {
	// === 数据指标启用开关 ===
	QuotaDataEnabled         bool `json:"quota_data_enabled"`          // 配额数据（时间序列）
	UserAnalyticsEnabled     bool `json:"user_analytics_enabled"`      // 用户分析（用户排行/趋势）
	RankingsEnabled          bool `json:"rankings_enabled"`            // 排行榜
	MediaConvertStatsEnabled bool `json:"media_convert_stats_enabled"` // 图片/视频转URL统计

	// === 写入层聚合粒度控制 ===
	QuotaDataTrackTokens  bool `json:"quota_data_track_tokens"`   // 是否记录 token_used，默认 true
	QuotaDataTrackByModel bool `json:"quota_data_track_by_model"` // 是否按模型维度聚合，默认 true
	QuotaDataTrackByUser  bool `json:"quota_data_track_by_user"`  // 是否按用户维度聚合，默认 true

	// === 面板启用开关（整合现有的 console_setting 配置） ===
	ApiInfoEnabled       bool `json:"api_info_enabled"`      // API信息面板
	UptimeKumaEnabled    bool `json:"uptime_kuma_enabled"`   // Uptime Kuma面板
	AnnouncementsEnabled bool `json:"announcements_enabled"` // 公告面板
	FAQEnabled           bool `json:"faq_enabled"`           // FAQ面板

	// === 刷新间隔配置（秒） ===
	QuotaDataRefreshInterval     int `json:"quota_data_refresh_interval"`     // 配额数据刷新间隔，默认3600
	UserAnalyticsRefreshInterval int `json:"user_analytics_refresh_interval"` // 用户分析刷新间隔，默认3600
	RankingsRefreshInterval      int `json:"rankings_refresh_interval"`       // 排行榜刷新间隔，默认300
	UptimeKumaRefreshInterval    int `json:"uptime_kuma_refresh_interval"`    // Uptime Kuma刷新间隔，默认60

	// === 时间范围限制（天） ===
	DefaultTimeRangeDays int `json:"default_time_range_days"` // 默认查询天数，默认7
	MaxTimeRangeDays     int `json:"max_time_range_days"`     // 最大查询天数，默认31

	// === 数据上限配置 ===
	RankingsModelLimit    int `json:"rankings_model_limit"`    // 排行榜模型数量，默认20
	RankingsVendorLimit   int `json:"rankings_vendor_limit"`   // 排行榜供应商数量，默认5
	RankingsPlatformLimit int `json:"rankings_platform_limit"` // 排行榜客户端平台（Agent）数量，默认10
	UserAnalyticsTopN     int `json:"user_analytics_top_n"`    // 用户排行榜TOP N，默认20
}

// 默认配置
var defaultDashboardConfig = DashboardConfig{
	// 核心数据指标默认启用
	QuotaDataEnabled:         true,
	UserAnalyticsEnabled:     true,
	RankingsEnabled:          true,
	MediaConvertStatsEnabled: true,

	// 写入层聚合粒度默认全部启用（保持向后兼容）
	QuotaDataTrackTokens:  true,
	QuotaDataTrackByModel: true,
	QuotaDataTrackByUser:  true,

	// 面板默认启用（保持向后兼容）
	ApiInfoEnabled:       true,
	UptimeKumaEnabled:    true,
	AnnouncementsEnabled: true,
	FAQEnabled:           true,

	// 刷新间隔（秒）
	QuotaDataRefreshInterval:     3600, // 1小时
	UserAnalyticsRefreshInterval: 3600, // 1小时
	RankingsRefreshInterval:      300,  // 5分钟
	UptimeKumaRefreshInterval:    60,   // 1分钟

	// 时间范围
	DefaultTimeRangeDays: 7,
	MaxTimeRangeDays:     31,

	// 数据上限
	RankingsModelLimit:    20,
	RankingsVendorLimit:   5,
	RankingsPlatformLimit: 10,
	UserAnalyticsTopN:     20,
}

// 全局实例
var dashboardConfig = defaultDashboardConfig

func init() {
	// 注册到全局配置管理器，键名为 dashboard_config
	manager.GlobalConfig.Register("dashboard_config", &dashboardConfig)
}

// GetDashboardConfig 获取 DashboardConfig 配置实例
func GetDashboardConfig() *DashboardConfig {
	return &dashboardConfig
}
