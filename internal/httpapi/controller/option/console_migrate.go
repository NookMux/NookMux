// 用于迁移检测的旧键，该文件下个版本会删除

package optioncontroller

import (
	"github.com/NookMux/NookMux/internal/common"
	"github.com/NookMux/NookMux/internal/i18n"
	"github.com/NookMux/NookMux/internal/store/db"
	"github.com/NookMux/NookMux/internal/store/option"
	"github.com/NookMux/NookMux/pkg/jsonx"
	"github.com/gin-gonic/gin"
	"net/http"
)

// MigrateConsoleSetting 迁移旧的控制台相关配置到 console.*
func MigrateConsoleSetting(c *gin.Context) {
	// 读取全部 option
	opts, err := optionstore.AllOption()
	if err != nil {
		common.SysError("failed to get all options: " + err.Error())
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": i18n.T(c, i18n.MsgConfigFetchFailed)})
		return
	}
	// 建立 map
	valMap := map[string]string{}
	for _, o := range opts {
		valMap[o.Key] = o.Value
	}

	// 迁移任一步写入失败即中止并返回 500，避免出现"部分迁移"的中间状态
	//（旧键已清、新键未写等）导致后续重试无法恢复。
	migrateOption := func(key, value string) bool {
		if err := optionstore.UpdateOption(key, value); err != nil {
			common.SysError("failed to update option " + key + " during console migration: " + err.Error())
			c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": i18n.T(c, i18n.MsgUpdateFailed)})
			return false
		}
		return true
	}

	// 处理 APIInfo
	if v := valMap["ApiInfo"]; v != "" {
		var arr []map[string]interface{}
		if err := jsonx.Unmarshal([]byte(v), &arr); err == nil {
			if len(arr) > 50 {
				arr = arr[:50]
			}
			bytes, _ := jsonx.Marshal(arr)
			if !migrateOption("console.api_info", string(bytes)) {
				return
			}
		}
		if !migrateOption("ApiInfo", "") {
			return
		}
	}
	// Announcements 直接搬
	if v := valMap["Announcements"]; v != "" {
		if !migrateOption("console.announcements", v) {
			return
		}
		if !migrateOption("Announcements", "") {
			return
		}
	}
	// FAQ 转换
	if v := valMap["FAQ"]; v != "" {
		var arr []map[string]interface{}
		if err := jsonx.Unmarshal([]byte(v), &arr); err == nil {
			out := []map[string]interface{}{}
			for _, item := range arr {
				q, _ := item["question"].(string)
				if q == "" {
					q, _ = item["title"].(string)
				}
				a, _ := item["answer"].(string)
				if a == "" {
					a, _ = item["content"].(string)
				}
				if q != "" && a != "" {
					out = append(out, map[string]interface{}{"question": q, "answer": a})
				}
			}
			if len(out) > 50 {
				out = out[:50]
			}
			bytes, _ := jsonx.Marshal(out)
			if !migrateOption("console.faq", string(bytes)) {
				return
			}
		}
		if !migrateOption("FAQ", "") {
			return
		}
	}
	// Uptime Kuma 迁移到新的 groups 结构（console.uptime_kuma_groups）
	url := valMap["UptimeKumaUrl"]
	slug := valMap["UptimeKumaSlug"]
	if url != "" && slug != "" {
		// 仅当同时存在 URL 与 Slug 时才进行迁移
		groups := []map[string]interface{}{
			{
				"id":           1,
				"categoryName": "old",
				"url":          url,
				"slug":         slug,
				"description":  "",
			},
		}
		bytes, _ := jsonx.Marshal(groups)
		if !migrateOption("console.uptime_kuma_groups", string(bytes)) {
			return
		}
	}
	// 清空旧键内容
	if url != "" {
		if !migrateOption("UptimeKumaUrl", "") {
			return
		}
	}
	if slug != "" {
		if !migrateOption("UptimeKumaSlug", "") {
			return
		}
	}

	// 删除旧键记录
	oldKeys := []string{"ApiInfo", "Announcements", "FAQ", "UptimeKumaUrl", "UptimeKumaSlug"}
	dbstore.DB.Where("key IN ?", oldKeys).Delete(&optionstore.Option{})

	// 重新加载 OptionMap
	optionstore.InitOptionMap()
	common.SysLog("console setting migrated")
	c.JSON(http.StatusOK, gin.H{"success": true, "message": i18n.T(c, i18n.MsgMiscMigrated)})
}
