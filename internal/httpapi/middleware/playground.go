package middleware

import (
	"net/http"
	"strings"

	"github.com/NookMux/NookMux/internal/common"
	"github.com/NookMux/NookMux/internal/domain/shared"

	domaingroup "github.com/NookMux/NookMux/internal/domain/group"
	"github.com/NookMux/NookMux/internal/httpapi"
	"github.com/NookMux/NookMux/internal/i18n"
	"github.com/NookMux/NookMux/internal/infra/cache"
	"github.com/gin-gonic/gin"
)

func PlaygroundRequestContext() func(c *gin.Context) {
	return func(c *gin.Context) {
		if c.GetBool("use_access_token") {
			abortWithOpenAiMessage(c, http.StatusForbidden, i18n.T(c, i18n.MsgPlaygroundAccessTokenUnsupported), shared.ErrorCodeAccessDenied)
			return
		}

		playgroundRequest := &shared.PlayGroundRequest{}
		if err := httpapi.UnmarshalBodyReusable(c, playgroundRequest); err != nil {
			statusCode := http.StatusBadRequest
			errorCode := shared.ErrorCodeInvalidRequest
			if cache.IsRequestBodyTooLargeError(err) {
				statusCode = http.StatusRequestEntityTooLarge
				errorCode = shared.ErrorCodeReadRequestBodyFailed
			}
			abortWithOpenAiMessage(c, statusCode, i18n.T(c, i18n.MsgDistributorInvalidPlaygroundReq, map[string]any{"Error": err.Error()}), errorCode)
			return
		}

		selectedGroup := strings.TrimSpace(playgroundRequest.Group)
		if selectedGroup == "" {
			c.Next()
			return
		}

		userGroup := httpapi.GetContextKeyString(c, common.ContextKeyUserGroup)
		if userGroup == "" {
			userGroup = httpapi.GetContextKeyString(c, common.ContextKeyUsingGroup)
		}
		if !domaingroup.GroupInUserUsableGroups(userGroup, selectedGroup) && selectedGroup != userGroup {
			abortWithOpenAiMessage(c, http.StatusForbidden, i18n.T(c, i18n.MsgDistributorGroupAccessDenied), shared.ErrorCodeAccessDenied)
			return
		}

		httpapi.SetContextKey(c, common.ContextKeyUsingGroup, selectedGroup)
		httpapi.SetContextKey(c, common.ContextKeyTokenGroup, selectedGroup)
		c.Next()
	}
}
