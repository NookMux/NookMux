package channelcontroller

import (
	"github.com/NookMux/NookMux/internal/domain/channel/constant"
	"github.com/NookMux/NookMux/internal/httpapi"
	"github.com/gin-gonic/gin"
)

// GetBuiltinChannelURLs 返回各内置渠道类型的 base_url 预设选项
// （value=渠道存储值，label_key=前端 i18n key），供渠道编辑表单渲染
// "预设下拉 + 自定义"。只读元数据接口，无审计要求。
func GetBuiltinChannelURLs(c *gin.Context) {
	httpapi.ApiSuccess(c, constant.BuiltinChannelURLOptions)
}
