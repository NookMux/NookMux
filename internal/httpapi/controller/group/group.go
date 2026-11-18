package groupcontroller

import (
	"github.com/NookMux/NookMux/internal/config"
	"github.com/NookMux/NookMux/internal/config/ratio"
	domaingroup "github.com/NookMux/NookMux/internal/domain/group"
	"github.com/NookMux/NookMux/internal/i18n"
	"github.com/NookMux/NookMux/internal/store/user"
	"github.com/gin-gonic/gin"
	"net/http"
)

func GetGroups(c *gin.Context) {
	groupNames := make([]string, 0)
	for groupName := range ratio.GetGroupRatioCopy() {
		groupNames = append(groupNames, groupName)
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    groupNames,
	})
}

func GetUserGroups(c *gin.Context) {
	usableGroups := make(map[string]map[string]interface{})
	userGroup := ""
	userId := c.GetInt("id")
	userGroup, _ = userstore.GetUserGroup(userId, false)
	userUsableGroups := domaingroup.GetUserUsableGroups(userGroup)
	for groupName := range ratio.GetGroupRatioCopy() {
		// UserUsableGroups contains the groups that the user can use
		if desc, ok := userUsableGroups[groupName]; ok {
			switch desc {
			case "默认分组":
				desc = i18n.T(c, i18n.MsgGroupDescDefault)
			case "vip分组":
				desc = i18n.T(c, i18n.MsgGroupDescVIP)
			}
			usableGroups[groupName] = map[string]interface{}{
				"ratio": domaingroup.GetUserGroupRatio(userGroup, groupName),
				"desc":  desc,
			}
		}
	}
	if _, ok := userUsableGroups["auto"]; ok {
		usableGroups["auto"] = map[string]interface{}{
			"ratio": i18n.T(c, i18n.MsgGroupRatioAuto),
			"desc":  config.GetUsableGroupDescription("auto"),
		}
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    usableGroups,
	})
}
