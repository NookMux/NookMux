package channelcontroller

import (
	audit "github.com/NookMux/NookMux/internal/domain/audit"
	domainchannel "github.com/NookMux/NookMux/internal/domain/channel"
	"github.com/NookMux/NookMux/internal/i18n"
	"github.com/NookMux/NookMux/internal/store/audit"
	"github.com/gin-gonic/gin"
	"net/http"
	"strings"
)

func GetChannelAffinityCacheStats(c *gin.Context) {
	stats := domainchannel.GetChannelAffinityCacheStats()
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    stats,
	})
}

func ClearChannelAffinityCache(c *gin.Context) {
	all := strings.TrimSpace(c.Query("all"))
	ruleName := strings.TrimSpace(c.Query("rule_name"))

	if all == "true" {
		deleted := domainchannel.ClearChannelAffinityCacheAll()
		audit.RecordAudit(c, auditstore.AuditModuleOption, auditstore.AuditActionDelete, "清空渠道亲和缓存", nil, nil)
		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"message": "",
			"data": gin.H{
				"deleted": deleted,
			},
		})
		return
	}

	if ruleName == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": i18n.T(c, i18n.MsgDashboardMissingAffinityClearArgs),
		})
		return
	}

	deleted, err := domainchannel.ClearChannelAffinityCacheByRuleName(c, ruleName)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": i18n.ResolveContextError(c, err, i18n.MsgDatabaseError),
		})
		return
	}

	audit.RecordAudit(c, auditstore.AuditModuleOption, auditstore.AuditActionDelete, "清空渠道亲和缓存: "+ruleName, nil, map[string]interface{}{"rule_name": ruleName})
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data": gin.H{
			"deleted": deleted,
		},
	})
}

func GetChannelAffinityUsageCacheStats(c *gin.Context) {
	ruleName := strings.TrimSpace(c.Query("rule_name"))
	usingGroup := strings.TrimSpace(c.Query("using_group"))
	modelName := strings.TrimSpace(c.Query("model_name"))
	keyFp := strings.TrimSpace(c.Query("key_fp"))

	if ruleName == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": i18n.T(c, i18n.MsgChannelRuleNameRequired),
		})
		return
	}
	if keyFp == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": i18n.T(c, i18n.MsgChannelKeyFpRequired),
		})
		return
	}

	stats := domainchannel.GetChannelAffinityUsageCacheStats(ruleName, usingGroup, modelName, keyFp)
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    stats,
	})
}
