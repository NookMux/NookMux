package dashboard

import (
	"errors"

	"github.com/NookMux/NookMux/internal/i18n"
)

// ValidateDashboardConfig 校验仪表板配置的有效性
func ValidateDashboardConfig(lang string, config *DashboardConfig) error {
	// 刷新间隔校验：60秒 - 86400秒（1分钟 - 1天）
	if config.QuotaDataRefreshInterval < 60 || config.QuotaDataRefreshInterval > 86400 {
		return errors.New(i18n.Translate(lang, i18n.MsgDashboardQuotaRefreshIntervalRange))
	}
	if config.UserAnalyticsRefreshInterval < 60 || config.UserAnalyticsRefreshInterval > 86400 {
		return errors.New(i18n.Translate(lang, i18n.MsgDashboardAnalyticsRefreshIntervalRng))
	}
	if config.RankingsRefreshInterval < 60 || config.RankingsRefreshInterval > 86400 {
		return errors.New(i18n.Translate(lang, i18n.MsgDashboardRankingsRefreshIntervalRng))
	}
	if config.UptimeKumaRefreshInterval < 30 || config.UptimeKumaRefreshInterval > 3600 {
		return errors.New(i18n.Translate(lang, i18n.MsgDashboardUptimeKumaRefreshInterval))
	}

	// 时间范围校验：1-365天
	if config.MaxTimeRangeDays < 1 || config.MaxTimeRangeDays > 365 {
		return errors.New(i18n.Translate(lang, i18n.MsgDashboardMaxTimeRangeRange))
	}
	if config.DefaultTimeRangeDays < 1 || config.DefaultTimeRangeDays > config.MaxTimeRangeDays {
		return errors.New(i18n.Translate(lang, i18n.MsgDashboardDefaultTimeRangeRange, map[string]any{"Max": config.MaxTimeRangeDays}))
	}

	// 数据上限校验：1-100
	if config.RankingsModelLimit < 1 || config.RankingsModelLimit > 100 {
		return errors.New(i18n.Translate(lang, i18n.MsgDashboardRankingsModelLimitRange))
	}
	if config.RankingsVendorLimit < 1 || config.RankingsVendorLimit > 50 {
		return errors.New(i18n.Translate(lang, i18n.MsgDashboardRankingsVendorLimitRange))
	}
	if config.UserAnalyticsTopN < 1 || config.UserAnalyticsTopN > 100 {
		return errors.New(i18n.Translate(lang, i18n.MsgDashboardUserAnalyticsTopNRange))
	}

	return nil
}

// ValidateDashboardConfigField 校验单个配置字段
// 用于 UpdateOption 场景，仅校验被修改的字段
func ValidateDashboardConfigField(lang string, field string, value interface{}) error {
	switch field {
	case "quota_data_refresh_interval":
		if v, ok := value.(int); ok {
			if v < 60 || v > 86400 {
				return errors.New(i18n.Translate(lang, i18n.MsgDashboardQuotaRefreshIntervalRange))
			}
		}
	case "user_analytics_refresh_interval":
		if v, ok := value.(int); ok {
			if v < 60 || v > 86400 {
				return errors.New(i18n.Translate(lang, i18n.MsgDashboardAnalyticsRefreshIntervalRng))
			}
		}
	case "rankings_refresh_interval":
		if v, ok := value.(int); ok {
			if v < 60 || v > 86400 {
				return errors.New(i18n.Translate(lang, i18n.MsgDashboardRankingsRefreshIntervalRng))
			}
		}
	case "uptime_kuma_refresh_interval":
		if v, ok := value.(int); ok {
			if v < 30 || v > 3600 {
				return errors.New(i18n.Translate(lang, i18n.MsgDashboardUptimeKumaRefreshInterval))
			}
		}
	case "max_time_range_days":
		if v, ok := value.(int); ok {
			if v < 1 || v > 365 {
				return errors.New(i18n.Translate(lang, i18n.MsgDashboardMaxTimeRangeRange))
			}
		}
	case "default_time_range_days":
		if v, ok := value.(int); ok {
			if v < 1 || v > 365 {
				return errors.New(i18n.Translate(lang, i18n.MsgDashboardMaxTimeRangeRange))
			}
			// 需要检查是否超过 max_time_range_days
			config := GetDashboardConfig()
			if v > config.MaxTimeRangeDays {
				return errors.New(i18n.Translate(lang, i18n.MsgDashboardDefaultTimeRangeExceedsMax, map[string]any{"Max": config.MaxTimeRangeDays}))
			}
		}
	case "rankings_model_limit":
		if v, ok := value.(int); ok {
			if v < 1 || v > 100 {
				return errors.New(i18n.Translate(lang, i18n.MsgDashboardRankingsModelLimitRange))
			}
		}
	case "rankings_vendor_limit":
		if v, ok := value.(int); ok {
			if v < 1 || v > 50 {
				return errors.New(i18n.Translate(lang, i18n.MsgDashboardRankingsVendorLimitRange))
			}
		}
	case "user_analytics_top_n":
		if v, ok := value.(int); ok {
			if v < 1 || v > 100 {
				return errors.New(i18n.Translate(lang, i18n.MsgDashboardUserAnalyticsTopNRange))
			}
		}
	}

	return nil
}
