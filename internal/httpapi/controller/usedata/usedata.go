package usedatacontroller

import (
	"github.com/NookMux/NookMux/internal/common"
	"github.com/NookMux/NookMux/internal/config/dashboard"
	rankings "github.com/NookMux/NookMux/internal/domain/rankings"
	"github.com/NookMux/NookMux/internal/httpapi"
	"github.com/NookMux/NookMux/internal/i18n"
	"github.com/NookMux/NookMux/internal/store/stored_media"
	"github.com/NookMux/NookMux/internal/store/usedata"
	"github.com/gin-gonic/gin"
	"net/http"
	"strconv"
)

func isUserQuotaRangeTooLong(startTimestamp, endTimestamp int64) bool {
	cfg := dashboard.GetDashboardConfig()
	maxDays := cfg.MaxTimeRangeDays
	if maxDays <= 0 {
		// 防御性兜底：配置异常（被显式置 0/负数）时保留默认 31 天行为
		maxDays = 31
	}
	maxRange := int64(maxDays) * 24 * 60 * 60
	if startTimestamp <= 0 || endTimestamp <= 0 || endTimestamp <= startTimestamp {
		return false
	}
	return endTimestamp-startTimestamp > maxRange
}

func GetAllQuotaDates(c *gin.Context) {
	dashboardConfig := dashboard.GetDashboardConfig()
	if !dashboardConfig.QuotaDataEnabled {
		httpapi.ApiSuccessI18n(c, i18n.MsgDashboardQuotaDataDisabled, []interface{}{})
		return
	}

	startTimestamp, _ := strconv.ParseInt(c.Query("start_timestamp"), 10, 64)
	endTimestamp, _ := strconv.ParseInt(c.Query("end_timestamp"), 10, 64)
	if isUserQuotaRangeTooLong(startTimestamp, endTimestamp) {
		httpapi.ApiErrorI18n(c, i18n.MsgDashboardTimeRangeTooLong)
		return
	}
	username := c.Query("username")
	dates, err := usedatastore.GetAllQuotaDates(startTimestamp, endTimestamp, username)
	if err != nil {
		common.SysError("failed to get all quota dates: " + err.Error())
		httpapi.ApiErrorI18n(c, i18n.MsgDatabaseError)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    dates,
	})
}

func GetQuotaDataGroupByUser(c *gin.Context) {
	dashboardConfig := dashboard.GetDashboardConfig()
	if !dashboardConfig.UserAnalyticsEnabled {
		httpapi.ApiSuccessI18n(c, i18n.MsgDashboardUserAnalyticsDisabled, []interface{}{})
		return
	}

	startTimestamp, _ := strconv.ParseInt(c.Query("start_timestamp"), 10, 64)
	endTimestamp, _ := strconv.ParseInt(c.Query("end_timestamp"), 10, 64)
	if isUserQuotaRangeTooLong(startTimestamp, endTimestamp) {
		httpapi.ApiErrorI18n(c, i18n.MsgDashboardTimeRangeTooLong)
		return
	}
	dates, err := usedatastore.GetQuotaDataGroupByUserCached(startTimestamp, endTimestamp)
	if err != nil {
		common.SysError("failed to get quota data group by user: " + err.Error())
		httpapi.ApiErrorI18n(c, i18n.MsgDatabaseError)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    dates,
	})
}

func GetUserQuotaDates(c *gin.Context) {
	dashboardConfig := dashboard.GetDashboardConfig()
	if !dashboardConfig.QuotaDataEnabled {
		httpapi.ApiSuccessI18n(c, i18n.MsgDashboardQuotaDataDisabled, []interface{}{})
		return
	}

	userId := c.GetInt("id")
	startTimestamp, _ := strconv.ParseInt(c.Query("start_timestamp"), 10, 64)
	endTimestamp, _ := strconv.ParseInt(c.Query("end_timestamp"), 10, 64)
	if isUserQuotaRangeTooLong(startTimestamp, endTimestamp) {
		httpapi.ApiErrorI18n(c, i18n.MsgDashboardTimeRangeTooLong)
		return
	}
	dates, err := usedatastore.GetQuotaDataByUserId(userId, startTimestamp, endTimestamp)
	if err != nil {
		common.SysError("failed to get quota data by user id: " + err.Error())
		httpapi.ApiErrorI18n(c, i18n.MsgDatabaseError)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    dates,
	})
}

// GetAllMediaConvertStats 管理员查询所有用户的图片/视频转URL统计
func GetAllMediaConvertStats(c *gin.Context) {
	dashboardConfig := dashboard.GetDashboardConfig()
	if !dashboardConfig.MediaConvertStatsEnabled {
		httpapi.ApiSuccessI18n(c, i18n.MsgDashboardMediaConvertDisabled, map[string]interface{}{"image_count": 0, "video_count": 0})
		return
	}

	startTimestamp, _ := strconv.ParseInt(c.Query("start_timestamp"), 10, 64)
	endTimestamp, _ := strconv.ParseInt(c.Query("end_timestamp"), 10, 64)

	stats, err := storedmediastore.GetAllMediaConvertStats(startTimestamp, endTimestamp)
	if err != nil {
		common.SysError("failed to get all media convert stats: " + err.Error())
		httpapi.ApiErrorI18n(c, i18n.MsgDatabaseError)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    stats,
	})
}

// RecalculateQuotaData 管理员触发重新计算指定时间范围的数据看板
func RecalculateQuotaData(c *gin.Context) {
	startTimestamp, _ := strconv.ParseInt(c.Query("start_timestamp"), 10, 64)
	endTimestamp, _ := strconv.ParseInt(c.Query("end_timestamp"), 10, 64)

	if startTimestamp <= 0 || endTimestamp <= 0 || endTimestamp <= startTimestamp {
		httpapi.ApiErrorI18n(c, i18n.MsgDashboardInvalidTimeRange)
		return
	}

	err := usedatastore.RecalculateQuotaData(startTimestamp, endTimestamp)
	if err != nil {
		common.SysError("failed to recalculate quota data: " + err.Error())
		httpapi.ApiErrorI18n(c, i18n.MsgDatabaseError)
		return
	}
	rankings.ClearRankingsCache()
	httpapi.ApiSuccessI18n(c, i18n.MsgDashboardRecalculateComplete, nil)
}

// GetUserMediaConvertStats 普通用户查询自己的图片/视频转URL统计
func GetUserMediaConvertStats(c *gin.Context) {
	dashboardConfig := dashboard.GetDashboardConfig()
	if !dashboardConfig.MediaConvertStatsEnabled {
		httpapi.ApiSuccessI18n(c, i18n.MsgDashboardMediaConvertDisabled, map[string]interface{}{"image_count": 0, "video_count": 0})
		return
	}

	userId := c.GetInt("id")
	startTimestamp, _ := strconv.ParseInt(c.Query("start_timestamp"), 10, 64)
	endTimestamp, _ := strconv.ParseInt(c.Query("end_timestamp"), 10, 64)

	stats, err := storedmediastore.GetMediaConvertStatsByUserId(userId, startTimestamp, endTimestamp)
	if err != nil {
		common.SysError("failed to get media convert stats by user id: " + err.Error())
		httpapi.ApiErrorI18n(c, i18n.MsgDatabaseError)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    stats,
	})
}
