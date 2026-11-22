package billingcontroller

import (
	"strings"

	"github.com/NookMux/NookMux/internal/common"
	"github.com/NookMux/NookMux/internal/config/operation"
	"github.com/NookMux/NookMux/internal/config/ratio"
	audit "github.com/NookMux/NookMux/internal/domain/audit"
	domaingroup "github.com/NookMux/NookMux/internal/domain/group"
	"github.com/NookMux/NookMux/internal/httpapi"
	"github.com/NookMux/NookMux/internal/i18n"
	"github.com/NookMux/NookMux/internal/store/audit"
	"github.com/NookMux/NookMux/internal/store/option"
	"github.com/NookMux/NookMux/internal/store/pricing"
	"github.com/NookMux/NookMux/internal/store/user"
	"github.com/gin-gonic/gin"
)

func GetPricing(c *gin.Context) {
	pricing := pricingstore.GetPricing()
	userId, exists := c.Get("id")
	var usableGroup map[string]string
	groupRatio := map[string]float64{}
	for s, f := range ratio.GetGroupRatioCopy() {
		groupRatio[s] = f
	}
	var group string
	if exists {
		user, err := userstore.GetUserCache(userId.(int))
		if err == nil {
			group = user.Group
			for g := range groupRatio {
				ratio, ok := ratio.GetGroupGroupRatio(group, g)
				if ok {
					groupRatio[g] = ratio
				}
			}
		}
	}

	usableGroup = domaingroup.GetUserUsableGroups(group)
	// check groupRatio contains usableGroup
	for group := range ratio.GetGroupRatioCopy() {
		if _, ok := usableGroup[group]; !ok {
			delete(groupRatio, group)
		}
	}

	// enable_groups 是内部组分类信息，只对已识别身份的调用者返回；
	// pricing 配置为匿名公开时不暴露各组内部组名。
	// GetPricing 返回共享缓存切片，必须复制后清空，不能就地修改缓存。
	if !exists {
		anonPricing := make([]pricingstore.Pricing, len(pricing))
		for i, p := range pricing {
			anonPricing[i] = p
			anonPricing[i].EnableGroup = nil
		}
		pricing = anonPricing
	}

	c.JSON(200, gin.H{
		"success":            true,
		"data":               pricing,
		"vendors":            pricingstore.GetVendors(),
		"group_ratio":        groupRatio,
		"usable_group":       usableGroup,
		"supported_endpoint": pricingstore.GetSupportedEndpointMap(),
		"auto_groups":        domaingroup.GetUserAutoGroup(group),
	})
}

// GetBuiltinModelPricing 返回模型在内置定价表中的原始条目，供模型定价可视化编辑器导入。
func GetBuiltinModelPricing(c *gin.Context) {
	model := strings.TrimSpace(c.Query("model"))
	if model == "" {
		httpapi.ApiErrorI18n(c, i18n.MsgPricingModelNameRequired)
		return
	}
	httpapi.ApiSuccess(c, ratio.GetBuiltinModelPricing(model))
}

func ResetModelRatio(c *gin.Context) {
	defaultStr := ratio.DefaultModelRatio2JSONString()
	err := optionstore.UpdateOption("ModelRatio", defaultStr)
	if err != nil {
		common.SysError("failed to update ModelRatio option: " + err.Error())
		httpapi.ApiErrorI18n(c, i18n.MsgDatabaseError)
		return
	}
	err = ratio.UpdateModelRatioByJSONString(defaultStr)
	if err != nil {
		common.SysError("failed to update ModelRatio memory cache: " + err.Error())
		httpapi.ApiErrorI18n(c, i18n.MsgDatabaseError)
		return
	}
	audit.RecordAudit(c, auditstore.AuditModuleOption, auditstore.AuditActionUpdate, "重置模型倍率", nil, nil)
	httpapi.ApiSuccessI18n(c, i18n.MsgPricingResetModelRatioSuccess, nil)
}

// ResetToolBillingRules restores tool_billing_setting.rules to the built-in
// default rule set. The dotted key is registered via config.GlobalConfig, so
// optionstore.UpdateOption takes care of both the DB write and the in-memory refresh.
func ResetToolBillingRules(c *gin.Context) {
	defaultStr := operation.DefaultToolBillingRules2JSONString()
	err := optionstore.UpdateOption("tool_billing_setting.rules", defaultStr)
	if err != nil {
		common.SysError("failed to update tool_billing_setting.rules option: " + err.Error())
		httpapi.ApiErrorI18n(c, i18n.MsgDatabaseError)
		return
	}
	audit.RecordAudit(c, auditstore.AuditModuleOption, auditstore.AuditActionUpdate, "重置工具计费规则", nil, nil)
	httpapi.ApiSuccessI18n(c, i18n.MsgPricingResetToolBillingRulesSuccess, nil)
}
