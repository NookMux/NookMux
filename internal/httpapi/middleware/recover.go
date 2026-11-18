package middleware

import (
	"fmt"
	"net/http"
	"runtime/debug"

	"github.com/NookMux/NookMux/internal/common"
	"github.com/NookMux/NookMux/internal/i18n"
	"github.com/gin-gonic/gin"
)

func RelayPanicRecover() gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if err := recover(); err != nil {
				common.SysLog(fmt.Sprintf("panic detected: %v", err))
				common.SysLog(fmt.Sprintf("stacktrace from panic: %s", string(debug.Stack())))
				c.JSON(http.StatusInternalServerError, gin.H{
					"error": gin.H{
						"message": i18n.T(c, i18n.MsgInternalPanicError, map[string]any{"Error": fmt.Sprintf("%v", err)}),
						"type":    "nookmux_panic",
					},
				})
				c.Abort()
			}
		}()
		c.Next()
	}
}
