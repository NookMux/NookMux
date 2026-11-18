package auditcontroller

import (
	"github.com/NookMux/NookMux/internal/i18n"
	"github.com/NookMux/NookMux/internal/store/audit"
	"github.com/gin-gonic/gin"
	"net/http"
	"strconv"
)

// GetAuditLogs 分页查询审计日志。
// 支持按用户名、模块、操作类型、时间范围筛选。
// 需要管理员权限（路由层通过 AdminAuth 保证）。
func GetAuditLogs(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("p", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}

	username := c.Query("username")
	module := c.Query("module")
	actionType := c.Query("action_type")

	var startTime, endTime int64
	if s := c.Query("start_timestamp"); s != "" {
		startTime, _ = strconv.ParseInt(s, 10, 64)
	}
	if e := c.Query("end_timestamp"); e != "" {
		endTime, _ = strconv.ParseInt(e, 10, 64)
	}

	logs, total, err := auditstore.GetAllAuditLogs(username, module, actionType, startTime, endTime, page, pageSize)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": i18n.T(c, i18n.MsgAuditFetchFailed) + ": " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data": gin.H{
			"items":     logs,
			"total":     total,
			"page":      page,
			"page_size": pageSize,
		},
	})
}

var auditModuleLabelKeys = map[string]string{
	auditstore.AuditModuleOption:          i18n.MsgAuditModuleOption,
	auditstore.AuditModuleChannel:         i18n.MsgAuditModuleChannel,
	auditstore.AuditModuleUser:            i18n.MsgAuditModuleUser,
	auditstore.AuditModuleToken:           i18n.MsgAuditModuleToken,
	auditstore.AuditModuleRedemption:      i18n.MsgAuditModuleRedemption,
	auditstore.AuditModuleModel:           i18n.MsgAuditModuleModel,
	auditstore.AuditModuleVendor:          i18n.MsgAuditModuleVendor,
	auditstore.AuditModuleDynamicRatio:    i18n.MsgAuditModuleDynamicRatio,
	auditstore.AuditModulePrefillGroup:    i18n.MsgAuditModulePrefillGroup,
	auditstore.AuditModuleDB:              i18n.MsgAuditModuleDB,
	auditstore.AuditModulePerformance:     i18n.MsgAuditModulePerformance,
	auditstore.AuditModuleLog:             i18n.MsgAuditModuleLog,
	auditstore.AuditModuleSetup:           i18n.MsgAuditModuleSetup,
	auditstore.AuditModuleDashboardConfig: i18n.MsgAuditModuleDashboardConfig,
	auditstore.AuditModuleVoice:           i18n.MsgAuditModuleVoice,
}

// GetAuditModules 返回所有审计模块列表，供前端渲染筛选下拉框。
func GetAuditModules(c *gin.Context) {
	modules := make([]auditstore.AuditModuleInfo, 0, len(auditstore.AuditModuleList))
	for _, m := range auditstore.AuditModuleList {
		label := m.Label
		if key, ok := auditModuleLabelKeys[m.Value]; ok {
			label = i18n.T(c, key)
		}
		modules = append(modules, auditstore.AuditModuleInfo{
			Value: m.Value,
			Label: label,
		})
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    modules,
	})
}
