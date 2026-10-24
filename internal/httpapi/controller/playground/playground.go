package playgroundcontroller

import (
	"errors"
	"fmt"
	"github.com/NookMux/NookMux/internal/domain/shared"
	relaycontroller "github.com/NookMux/NookMux/internal/httpapi/controller/relay"
	"github.com/NookMux/NookMux/internal/httpapi/middleware"
	"github.com/NookMux/NookMux/internal/i18n"
	relaycommon "github.com/NookMux/NookMux/internal/relay/common"
	relayconstant "github.com/NookMux/NookMux/internal/relay/constant"
	"github.com/NookMux/NookMux/internal/store/token"
	"github.com/NookMux/NookMux/internal/store/user"
	"github.com/gin-gonic/gin"
)

func Playground(c *gin.Context) {
	var newAPIError *shared.NookMuxError

	defer func() {
		if newAPIError != nil {
			c.JSON(newAPIError.StatusCode, gin.H{
				"error": newAPIError.ToOpenAIError(),
			})
		}
	}()

	useAccessToken := c.GetBool("use_access_token")
	if useAccessToken {
		newAPIError = shared.NewError(errors.New(i18n.T(c, i18n.MsgPlaygroundAccessTokenUnsupported)), shared.ErrorCodeAccessDenied, shared.ErrOptionWithSkipRetry())
		return
	}

	relayInfo, err := relaycommon.GenRelayInfo(c, relayconstant.RelayFormatOpenAI, nil, nil)
	if err != nil {
		newAPIError = shared.NewError(err, shared.ErrorCodeInvalidRequest, shared.ErrOptionWithSkipRetry())
		return
	}

	userId := c.GetInt("id")

	// Write user context to ensure acceptUnsetRatio is available
	userCache, err := userstore.GetUserCache(userId)
	if err != nil {
		newAPIError = shared.NewError(err, shared.ErrorCodeQueryDataError, shared.ErrOptionWithSkipRetry())
		return
	}
	userCache.WriteContext(c)

	tempToken := &tokenstore.Token{
		UserId:         userId,
		Name:           fmt.Sprintf("playground-%s", relayInfo.UsingGroup),
		Group:          relayInfo.UsingGroup,
		UnlimitedQuota: true, // Playground uses wallet billing, skip token quota checks
		RemainQuota:    0,
	}
	_ = middleware.SetupContextForToken(c, tempToken, "")

	relaycontroller.Relay(c, relayconstant.RelayFormatOpenAI)
}
