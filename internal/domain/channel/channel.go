package channel

import (
	"fmt"
	"github.com/NookMux/NookMux/internal/common"
	"github.com/NookMux/NookMux/internal/config/operation"
	"github.com/NookMux/NookMux/internal/domain/channel/constant"
	sensitive "github.com/NookMux/NookMux/internal/domain/sensitive"
	"github.com/NookMux/NookMux/internal/domain/shared"
	"github.com/NookMux/NookMux/internal/i18n"
	notify "github.com/NookMux/NookMux/internal/infra/notify"
	"github.com/NookMux/NookMux/internal/store/channel"
	"net/http"
	"strings"
)

func formatNotifyType(channelId int, status int) string {
	return fmt.Sprintf("%s_%d_%d", shared.NotifyTypeChannelUpdate, channelId, status)
}

// disable & notify
func DisableChannel(channelError ChannelError, reason string) {
	lang := channelError.Lang
	if lang == "" {
		lang = i18n.DefaultLang
	}
	reasonPreview := common.LocalLogPreview(reason)
	common.SysLog(fmt.Sprintf("通道「%s」（#%d）发生错误，准备禁用，原因：%s", channelError.ChannelName, channelError.ChannelId, reasonPreview))

	// 检查是否启用自动禁用功能
	if !channelError.AutoBan {
		common.SysLog(fmt.Sprintf("通道「%s」（#%d）未启用自动禁用功能，跳过禁用操作", channelError.ChannelName, channelError.ChannelId))
		return
	}

	success := channelstore.UpdateChannelStatus(channelError.ChannelId, channelError.UsingKey, common.ChannelStatusAutoDisabled, reasonPreview)
	if success {
		args := map[string]any{"Name": channelError.ChannelName, "Id": channelError.ChannelId, "Reason": reasonPreview}
		subject := i18n.Translate(lang, i18n.MsgChannelNotifyDisabledTitle, args)
		content := i18n.Translate(lang, i18n.MsgChannelNotifyDisabledBody, args)
		notify.NotifyRootUser(formatNotifyType(channelError.ChannelId, common.ChannelStatusAutoDisabled), subject, content)
	}
}

func EnableChannel(channelId int, usingKey string, channelName string, lang string) {
	if lang == "" {
		lang = i18n.DefaultLang
	}
	success := channelstore.UpdateChannelStatus(channelId, usingKey, common.ChannelStatusEnabled, "")
	if success {
		args := map[string]any{"Name": channelName, "Id": channelId}
		subject := i18n.Translate(lang, i18n.MsgChannelNotifyEnabledTitle, args)
		content := i18n.Translate(lang, i18n.MsgChannelNotifyEnabledBody, args)
		notify.NotifyRootUser(formatNotifyType(channelId, common.ChannelStatusEnabled), subject, content)
	}
}

func ShouldDisableChannel(channelType int, err *shared.NookMuxError) bool {
	if !common.AutomaticDisableChannelEnabled {
		return false
	}
	if err == nil {
		return false
	}
	if shared.IsChannelError(err) {
		return true
	}
	if shared.IsSkipRetryError(err) {
		return false
	}
	if operation.ShouldDisableByStatusCode(err.StatusCode) {
		return true
	}
	//if err.StatusCode == http.StatusUnauthorized {
	//	return true
	//}
	if err.StatusCode == http.StatusForbidden {
		switch channelType {
		case constant.ChannelTypeGemini:
			return true
		}
	}
	oaiErr := err.ToOpenAIError()
	switch oaiErr.Code {
	case "invalid_api_key":
		return true
	case "account_deactivated":
		return true
	case "billing_not_active":
		return true
	case "pre_consume_token_quota_failed":
		return true
	case "Arrearage":
		return true
	}
	switch oaiErr.Type {
	case "insufficient_quota":
		return true
	case "insufficient_user_quota":
		return true
	// https://docs.anthropic.com/claude/reference/errors
	case "authentication_error":
		return true
	case "permission_error":
		return true
	case "forbidden":
		return true
	}

	lowerMessage := strings.ToLower(err.Error())
	search, _ := sensitive.AcSearch(lowerMessage, operation.AutomaticDisableKeywords, true)
	return search
}

func ShouldEnableChannel(newAPIError *shared.NookMuxError, status int) bool {
	if !common.AutomaticEnableChannelEnabled {
		return false
	}
	if newAPIError != nil {
		return false
	}
	if status != common.ChannelStatusAutoDisabled {
		return false
	}
	return true
}
