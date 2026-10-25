package helper

import (
	"errors"
	configmodel "github.com/NookMux/NookMux/internal/config/model"
	"github.com/NookMux/NookMux/internal/i18n"
	"github.com/NookMux/NookMux/internal/store/voice"
	"github.com/gin-gonic/gin"
	"strings"
)

// ResolveVoiceForTTSUpstream 按用户请求的原始音色 ID 解析并发往上游的音色 ID，
// 供 MiniMax 原生渠道与 OpenAI 兼容渠道共用。
//
// 校验逻辑（防绕过）：
//   - 始终按原始音色 ID 查库，校验通过后再用 redirect_id 替换发给上游，
//     用户无法通过直接传 redirect_id 绕过白名单。
//   - 库内命中的音色必须为“已创建”且 allowed=true 才能用于 TTS：preview（试听中）
//     与被管理员禁用的记录无论白名单开关如何一律拒绝，预览音色须完成确认定制（支付）
//     流转为 created 后才可调用。
//   - 白名单总开关仅约束库内未命中的音色来源：开启时未命中一律拒绝；
//     关闭时放行原 ID（如上游预置音色），但仍应用库内的 redirect_id。
//
// 返回应发给上游的音色 ID；校验失败时返回面向用户的普通业务错误（不暴露渠道信息）。
func ResolveVoiceForTTSUpstream(c *gin.Context, voiceId string) (string, error) {
	voiceId = strings.TrimSpace(voiceId)
	if voiceId == "" {
		return "", nil
	}
	found, upstreamId, allowed, err := voicestore.ResolveVoiceForTTS(voiceId)
	if err != nil {
		// DB 查询失败时，若白名单开启则 fail-closed，否则放行原 ID。
		if configmodel.IsMiniMaxVoiceWhitelistEnabled() {
			return "", errors.New(i18n.T(c, i18n.MsgVoiceNotAuthorizedWithID, map[string]any{"Voice": voiceId}))
		}
		return voiceId, nil
	}
	// 命中库内记录但状态不可用（preview 或被禁用）：记录状态是库内明确事实，
	// 拒绝不随“限制音色来源”的白名单开关变化。
	if found && !allowed {
		return "", newVoiceNotAllowedError(c, voiceId)
	}
	if !found && configmodel.IsMiniMaxVoiceWhitelistEnabled() {
		return "", newVoiceNotAllowedError(c, voiceId)
	}
	// 命中记录时优先使用 redirect_id；未命中且白名单关闭时用原 ID。
	if found {
		return upstreamId, nil
	}
	return voiceId, nil
}

// newVoiceNotAllowedError returns a localized error for voice whitelist
// rejection. The message language is chosen from the request's Accept-Language
// header. When the voice is non-empty it is included for diagnostics.
func newVoiceNotAllowedError(c *gin.Context, voice string) error {
	trimmed := strings.TrimSpace(voice)
	if trimmed == "" {
		return errors.New(i18n.T(c, i18n.MsgVoiceNotAuthorized))
	}
	return errors.New(i18n.T(c, i18n.MsgVoiceNotAuthorizedWithID, map[string]any{
		"Voice": trimmed,
	}))
}
