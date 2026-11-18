package checkincontroller

import (
	"errors"
	"fmt"
	"github.com/NookMux/NookMux/internal/common"
	"github.com/NookMux/NookMux/internal/config/operation"
	"github.com/NookMux/NookMux/internal/httpapi"
	"github.com/NookMux/NookMux/internal/i18n"
	"github.com/NookMux/NookMux/internal/infra/log"
	"github.com/NookMux/NookMux/internal/store/checkin"
	"github.com/NookMux/NookMux/internal/store/log"
	"github.com/NookMux/NookMux/internal/store/user"
	"github.com/gin-gonic/gin"
	"net/http"
	"time"
)

// GetCheckinStatus 获取用户签到状态和历史记录
func GetCheckinStatus(c *gin.Context) {
	setting := operation.GetCheckinSetting()
	if !setting.Enabled {
		httpapi.ApiErrorI18n(c, i18n.MsgCheckinDisabled)
		return
	}
	userId := c.GetInt("id")
	// 获取月份参数，默认为当前月份
	month := c.DefaultQuery("month", time.Now().Format("2006-01"))

	stats, err := checkinstore.GetUserCheckinStats(userId, month)
	if err != nil {
		if !errors.As(err, new(*i18n.Error)) {
			common.SysError("failed to get checkin stats: " + err.Error())
		}
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": i18n.ResolveContextError(c, err, i18n.MsgDatabaseError),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"enabled":   setting.Enabled,
			"min_quota": setting.MinQuota,
			"max_quota": setting.MaxQuota,
			"stats":     stats,
		},
	})
}

// DoCheckin 执行用户签到
func DoCheckin(c *gin.Context) {
	setting := operation.GetCheckinSetting()
	if !setting.Enabled {
		httpapi.ApiErrorI18n(c, i18n.MsgCheckinDisabled)
		return
	}

	userId := c.GetInt("id")

	checkin, err := checkinstore.UserCheckin(userId)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": i18n.ResolveContextError(c, err, i18n.MsgCheckinFailed),
		})
		return
	}
	userstore.RecordLog(userId, logstore.LogTypeSystem, fmt.Sprintf("用户签到，获得额度 %s", log.LogQuota(checkin.QuotaAwarded)))
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": i18n.T(c, i18n.MsgCheckinSuccess),
		"data": gin.H{
			"quota_awarded": checkin.QuotaAwarded,
			"checkin_date":  checkin.CheckinDate},
	})
}
