package channelcontroller

import (
	"github.com/NookMux/NookMux/internal/common"
	"github.com/NookMux/NookMux/internal/httpapi"
	"github.com/NookMux/NookMux/internal/i18n"
	"github.com/NookMux/NookMux/internal/store/missing_models"
	"github.com/gin-gonic/gin"
	"net/http"
)

// GetMissingModels returns the list of model names that are referenced by channels
// but do not have corresponding records in the models meta table.
// This helps administrators quickly discover models that need configuration.
func GetMissingModels(c *gin.Context) {
	missing, err := missingmodelstore.GetMissingModels()
	if err != nil {
		common.SysError("failed to get missing models: " + err.Error())
		httpapi.ApiErrorI18n(c, i18n.MsgDatabaseError)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    missing,
	})
}
