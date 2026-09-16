package middleware

import (
	"errors"
	"fmt"
	"github.com/NookMux/NookMux/internal/common"
	"github.com/NookMux/NookMux/internal/config/ratio"
	domainchannel "github.com/NookMux/NookMux/internal/domain/channel"
	channelconstant "github.com/NookMux/NookMux/internal/domain/channel/constant"
	domaingroup "github.com/NookMux/NookMux/internal/domain/group"
	"github.com/NookMux/NookMux/internal/domain/shared"
	"github.com/NookMux/NookMux/internal/httpapi"
	relayconstant "github.com/NookMux/NookMux/internal/relay/constant"
	"github.com/NookMux/NookMux/internal/store/channel"
	"github.com/gin-gonic/gin"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"time"
)

type ModelRequest struct {
	Model string `json:"model"`
	Group string `json:"group,omitempty"`
}

func Distribute() func(c *gin.Context) {
	return func(c *gin.Context) {
		var channel *channelstore.Channel
		channelId, ok := httpapi.GetContextKey(c, common.ContextKeyTokenSpecificChannelId)
		modelRequest, shouldSelectChannel, err := getModelRequest(c)
		if err != nil {
			abortWithOpenAiMessage(c, http.StatusBadRequest, "Invalid request, "+err.Error())
			return
		}
		if ok {
			id, err := strconv.Atoi(channelId.(string))
			if err != nil {
				abortWithOpenAiMessage(c, http.StatusBadRequest, "无效的渠道 Id")
				return
			}
			channel, err = channelstore.GetChannelById(id, true)
			if err != nil {
				abortWithOpenAiMessage(c, http.StatusBadRequest, "无效的渠道 Id")
				return
			}
			if channel.Status != common.ChannelStatusEnabled {
				abortWithOpenAiMessage(c, http.StatusForbidden, "该渠道已被禁用")
				return
			}
		} else {
			// Select a channel for the user
			// check token model mapping
			modelLimitEnable := httpapi.GetContextKeyBool(c, common.ContextKeyTokenModelLimitEnabled)
			if modelLimitEnable {
				s, ok := httpapi.GetContextKey(c, common.ContextKeyTokenModelLimit)
				if !ok {
					// token model limit is empty, all models are not allowed
					abortWithOpenAiMessage(c, http.StatusForbidden, "该令牌无权访问任何模型")
					return
				}
				var tokenModelLimit map[string]bool
				tokenModelLimit, ok = s.(map[string]bool)
				if !ok {
					tokenModelLimit = map[string]bool{}
				}
				matchName := ratio.FormatMatchingModelName(modelRequest.Model) // match gpts & thinking-*
				if _, ok := tokenModelLimit[matchName]; !ok {
					abortWithOpenAiMessage(c, http.StatusForbidden, "该令牌无权访问模型 "+modelRequest.Model)
					return
				}
			}

			if shouldSelectChannel {
				if modelRequest.Model == "" {
					abortWithOpenAiMessage(c, http.StatusBadRequest, "未指定模型名称，模型名称不能为空")
					return
				}
				var selectGroup string
				usingGroup := httpapi.GetContextKeyString(c, common.ContextKeyUsingGroup)
				relayFormat := guessRelayFormatFromPath(c.Request.URL.Path)

				// Playground: allow user to select a group for the request
				if strings.HasPrefix(c.Request.URL.Path, "/pg/chat/completions") {
					playgroundRequest := &shared.PlayGroundRequest{}
					err = httpapi.UnmarshalBodyReusable(c, playgroundRequest)
					if err != nil {
						abortWithOpenAiMessage(c, http.StatusBadRequest, "无效的游乐场请求: "+err.Error())
						return
					}
					if playgroundRequest.Group != "" {
						if !domaingroup.GroupInUserUsableGroups(usingGroup, playgroundRequest.Group) && playgroundRequest.Group != usingGroup {
							abortWithOpenAiMessage(c, http.StatusForbidden, "无权访问该分组")
							return
						}
						usingGroup = playgroundRequest.Group
						httpapi.SetContextKey(c, common.ContextKeyUsingGroup, usingGroup)
					}
				}

				if preferredChannelID, found := domainchannel.GetPreferredChannelByAffinity(c, modelRequest.Model, usingGroup); found {
					affinityUsable := false
					preferred, err := channelstore.CacheGetChannel(preferredChannelID)
					if err == nil && preferred != nil && preferred.Status == common.ChannelStatusEnabled {
						if usingGroup == "auto" {
							userGroup := httpapi.GetContextKeyString(c, common.ContextKeyUserGroup)
							autoGroups := domaingroup.GetUserAutoGroup(userGroup)
							for _, g := range autoGroups {
								if channelstore.IsChannelEnabledForGroupModel(g, modelRequest.Model, preferred.Id) {
									httpapi.SetContextKey(c, common.ContextKeyAutoGroup, g)
									channel = preferred
									affinityUsable = true
									domainchannel.MarkChannelAffinityUsed(c, g, preferred.Id)
									break
								}
							}
						} else if channelstore.IsChannelEnabledForGroupModel(usingGroup, modelRequest.Model, preferred.Id) {
							channel = preferred
							affinityUsable = true
							domainchannel.MarkChannelAffinityUsed(c, usingGroup, preferred.Id)
						}
					}
					// 亲和渠道被禁用或对当前分组/模型不可用时，按配置决定是否清空粘性条目：
					// 默认清空并改选其他渠道；开启 keep_on_channel_disabled 后保留条目等待渠道恢复。
					if !affinityUsable && !domainchannel.ShouldKeepChannelAffinityOnChannelDisabled() {
						domainchannel.ClearCurrentChannelAffinityCache(c)
					}
				}

				if channel == nil {
					channel, selectGroup, err = domainchannel.CacheGetRandomSatisfiedChannel(&domainchannel.RetryParam{
						Ctx:         c,
						ModelName:   modelRequest.Model,
						TokenGroup:  usingGroup,
						Retry:       common.GetPointer(0),
						RelayFormat: relayFormat,
					})
					if err != nil {
						showGroup := usingGroup
						if usingGroup == "auto" {
							showGroup = fmt.Sprintf("auto(%s)", selectGroup)
						}
						message := fmt.Sprintf("获取分组 %s 下模型 %s 的可用渠道失败（distributor）: %s", showGroup, modelRequest.Model, err.Error())
						// 如果错误，但是渠道不为空，说明是数据库一致性问题
						//if channel != nil {
						//	common.SysError(fmt.Sprintf("渠道不存在：%d", channel.Id))
						//	message = "数据库一致性已被破坏，请联系管理员"
						//}
						abortWithOpenAiMessage(c, http.StatusServiceUnavailable, message, shared.ErrorCodeModelNotFound)
						return
					}
					if channel == nil {
						abortWithOpenAiMessage(c, http.StatusServiceUnavailable, fmt.Sprintf("分组 %s 下模型 %s 无可用渠道（distributor）", usingGroup, modelRequest.Model), shared.ErrorCodeModelNotFound)
						return
					}
				}
			}
		}
		httpapi.SetContextKey(c, common.ContextKeyRequestStartTime, time.Now())
		if apiErr := SetupContextForSelectedChannel(c, channel, modelRequest.Model); apiErr != nil {
			statusCode := apiErr.StatusCode
			if statusCode == 0 {
				statusCode = http.StatusServiceUnavailable
			}
			abortWithOpenAiMessage(c, statusCode, apiErr.Error(), apiErr.GetErrorCode())
			return
		}
		c.Next()
		if channel != nil && c.Writer != nil && c.Writer.Status() < http.StatusBadRequest {
			domainchannel.RecordChannelAffinity(c, channel.Id)
		}
	}
}

// getModelFromRequest 从请求中读取模型信息
// 根据 Content-Type 自动处理：
// - application/json
// - application/x-www-form-urlencoded
// - multipart/form-data
func getModelFromRequest(c *gin.Context) (*ModelRequest, error) {
	var modelRequest ModelRequest
	err := httpapi.UnmarshalBodyReusable(c, &modelRequest)
	if err != nil {
		return nil, errors.New("无效的请求, " + err.Error())
	}
	return &modelRequest, nil
}

func getModelRequest(c *gin.Context) (*ModelRequest, bool, error) {
	var modelRequest ModelRequest
	shouldSelectChannel := true
	if strings.HasPrefix(c.Request.URL.Path, "/v1beta/models/") || strings.HasPrefix(c.Request.URL.Path, "/v1/models/") {
		// Gemini API 路径处理: /v1beta/models/gemini-2.0-flash:generateContent
		relayMode := relayconstant.RelayModeGemini
		modelName := extractModelNameFromGeminiPath(c.Request.URL.Path)
		if modelName != "" {
			modelRequest.Model = modelName
		}
		c.Set("relay_mode", relayMode)
	} else if !strings.HasPrefix(c.Request.URL.Path, "/v1/audio/transcriptions") && !strings.Contains(c.Request.Header.Get("Content-Type"), "multipart/form-data") {
		req, err := getModelFromRequest(c)
		if err != nil {
			return nil, false, err
		}
		modelRequest.Model = req.Model
	}
	if strings.HasPrefix(c.Request.URL.Path, "/v1/realtime") {
		//wss://api.openai.com/v1/realtime?model=gpt-4o-realtime-preview-2024-10-01
		modelRequest.Model = c.Query("model")
	}
	if strings.HasPrefix(c.Request.URL.Path, "/v1/moderations") {
		if modelRequest.Model == "" {
			modelRequest.Model = "text-moderation-stable"
		}
	}
	if strings.HasSuffix(c.Request.URL.Path, "embeddings") {
		if modelRequest.Model == "" {
			modelRequest.Model = c.Param("model")
		}
	}
	if strings.HasPrefix(c.Request.URL.Path, "/v1/images/generations") {
		modelRequest.Model = common.GetStringIfEmpty(modelRequest.Model, "dall-e")
	} else if strings.HasPrefix(c.Request.URL.Path, "/v1/images/edits") {
		//modelRequest.Model = common.GetStringIfEmpty(c.PostForm("model"), "gpt-image-1")
		contentType := c.ContentType()
		if slices.Contains([]string{gin.MIMEPOSTForm, gin.MIMEMultipartPOSTForm}, contentType) {
			req, err := getModelFromRequest(c)
			if err == nil && req.Model != "" {
				modelRequest.Model = req.Model
			}
		}
	}
	if strings.HasPrefix(c.Request.URL.Path, "/v1/audio") {
		relayMode := relayconstant.RelayModeAudioSpeech
		if strings.HasPrefix(c.Request.URL.Path, "/v1/audio/speech") {

			modelRequest.Model = common.GetStringIfEmpty(modelRequest.Model, "tts-1")
		} else if strings.HasPrefix(c.Request.URL.Path, "/v1/audio/translations") {
			// 先尝试从请求读取
			if req, err := getModelFromRequest(c); err == nil && req.Model != "" {
				modelRequest.Model = req.Model
			}
			modelRequest.Model = common.GetStringIfEmpty(modelRequest.Model, "whisper-1")
			relayMode = relayconstant.RelayModeAudioTranslation
		} else if strings.HasPrefix(c.Request.URL.Path, "/v1/audio/transcriptions") {
			// 先尝试从请求读取
			if req, err := getModelFromRequest(c); err == nil && req.Model != "" {
				modelRequest.Model = req.Model
			}
			modelRequest.Model = common.GetStringIfEmpty(modelRequest.Model, "whisper-1")
			relayMode = relayconstant.RelayModeAudioTranscription
		}
		c.Set("relay_mode", relayMode)
	}
	// 令牌级入站模型重定向：在路径压缩后缀（responses/compact）套用之前执行，
	// 使映射查找基于客户端原始模型名，重定向目标再自然携带压缩后缀。
	if err := applyTokenModelMapping(c, &modelRequest); err != nil {
		return nil, false, err
	}
	if strings.HasPrefix(c.Request.URL.Path, "/v1/responses/compact") && modelRequest.Model != "" {
		modelRequest.Model = ratio.WithCompactModelSuffix(modelRequest.Model)
	}
	return &modelRequest, shouldSelectChannel, nil
}

func SetupContextForSelectedChannel(c *gin.Context, channel *channelstore.Channel, modelName string) *shared.NookMuxError {
	c.Set("original_model", modelName) // for retry
	if channel == nil {
		return shared.NewError(errors.New("channel is nil"), shared.ErrorCodeGetChannelFailed, shared.ErrOptionWithSkipRetry())
	}
	if channel.Type == channelconstant.ChannelTypeUnknown {
		return shared.NewError(errors.New("invalid channel type"), shared.ErrorCodeInvalidApiType, shared.ErrOptionWithSkipRetry())
	}
	if _, ok := channelconstant.ChannelTypeNames[channel.Type]; !ok {
		return shared.NewError(fmt.Errorf("unsupported channel type: %d", channel.Type), shared.ErrorCodeInvalidApiType, shared.ErrOptionWithSkipRetry())
	}
	httpapi.SetContextKey(c, common.ContextKeyChannelId, channel.Id)
	httpapi.SetContextKey(c, common.ContextKeyChannelName, channel.Name)
	httpapi.SetContextKey(c, common.ContextKeyChannelType, channel.Type)
	httpapi.SetContextKey(c, common.ContextKeyChannelCreateTime, channel.CreatedTime)
	httpapi.SetContextKey(c, common.ContextKeyChannelSetting, channel.GetSetting())
	httpapi.SetContextKey(c, common.ContextKeyChannelOtherSetting, channel.GetOtherSettings())
	httpapi.SetContextKey(c, common.ContextKeyChannelParamOverride, channel.GetParamOverride())
	httpapi.SetContextKey(c, common.ContextKeyChannelHeaderOverride, channel.GetHeaderOverride())
	if nil != channel.OpenAIOrganization && *channel.OpenAIOrganization != "" {
		httpapi.SetContextKey(c, common.ContextKeyChannelOrganization, *channel.OpenAIOrganization)
	}
	httpapi.SetContextKey(c, common.ContextKeyChannelAutoBan, channel.GetAutoBan())
	httpapi.SetContextKey(c, common.ContextKeyChannelModelMapping, channel.GetModelMapping())
	httpapi.SetContextKey(c, common.ContextKeyChannelStatusCodeMapping, channel.GetStatusCodeMapping())

	key, index, newAPIError := channel.GetNextEnabledKey()
	if newAPIError != nil {
		return newAPIError
	}
	if channel.ChannelInfo.IsMultiKey {
		httpapi.SetContextKey(c, common.ContextKeyChannelIsMultiKey, true)
		httpapi.SetContextKey(c, common.ContextKeyChannelMultiKeyIndex, index)
	} else {
		// 必须设置为 false，否则在重试到单个 key 的时候会导致日志显示错误
		httpapi.SetContextKey(c, common.ContextKeyChannelIsMultiKey, false)
	}
	// c.Request.Header.Set("Authorization", fmt.Sprintf("Bearer %s", key))
	httpapi.SetContextKey(c, common.ContextKeyChannelKey, key)
	httpapi.SetContextKey(c, common.ContextKeyChannelBaseUrl, channel.GetBaseURL())

	// TODO: api_version统一
	switch channel.Type {
	case channelconstant.ChannelTypeAzure:
		c.Set("api_version", channel.Other)
	case channelconstant.ChannelTypeVertexAi:
		c.Set("region", channel.Other)
	case channelconstant.ChannelTypeGemini:
		c.Set("api_version", channel.Other)
	}
	return nil
}

// extractModelNameFromGeminiPath 从 Gemini API URL 路径中提取模型名
// 输入格式: /v1beta/models/gemini-2.0-flash:generateContent
// 输出: gemini-2.0-flash
func extractModelNameFromGeminiPath(path string) string {
	// 查找 "/models/" 的位置
	modelsPrefix := "/models/"
	modelsIndex := strings.Index(path, modelsPrefix)
	if modelsIndex == -1 {
		return ""
	}

	// 从 "/models/" 之后开始提取
	startIndex := modelsIndex + len(modelsPrefix)
	if startIndex >= len(path) {
		return ""
	}

	// 查找 ":" 的位置，模型名在 ":" 之前
	colonIndex := strings.Index(path[startIndex:], ":")
	if colonIndex == -1 {
		// 如果没有找到 ":"，返回从 "/models/" 到路径结尾的部分
		return path[startIndex:]
	}

	// 返回模型名部分
	return path[startIndex : startIndex+colonIndex]
}

// guessRelayFormatFromPath infers the relay format from the request URL path.
// This is used by the distributor to prefer channels matching the request format.
func guessRelayFormatFromPath(path string) relayconstant.RelayFormat {
	switch {
	case strings.HasPrefix(path, "/v1/messages"):
		return relayconstant.RelayFormatClaude
	case strings.HasPrefix(path, "/v1beta/models/"):
		return relayconstant.RelayFormatGemini
	case strings.HasPrefix(path, "/v1/models/") && strings.Contains(path, ":"):
		// POST /v1/models/{model}:{action} is Gemini format relay
		return relayconstant.RelayFormatGemini
	case strings.HasPrefix(path, "/v1/engines/"):
		// POST /v1/engines/{model}/embeddings is Gemini format relay
		return relayconstant.RelayFormatGemini
	case strings.HasPrefix(path, "/v1/realtime"):
		return relayconstant.RelayFormatOpenAIRealtime
	case strings.HasPrefix(path, "/v1/responses/compact"):
		return relayconstant.RelayFormatOpenAIResponsesCompaction
	case strings.HasPrefix(path, "/v1/responses"):
		return relayconstant.RelayFormatOpenAIResponses
	case strings.HasPrefix(path, "/v1/images/"):
		return relayconstant.RelayFormatOpenAIImage
	case strings.HasPrefix(path, "/v1/embeddings"):
		return relayconstant.RelayFormatEmbedding
	case strings.HasPrefix(path, "/v1/audio/"):
		return relayconstant.RelayFormatOpenAIAudio
	case strings.HasPrefix(path, "/v1/rerank"):
		return relayconstant.RelayFormatRerank
	case strings.HasPrefix(path, "/v1/chat/completions"),
		strings.HasPrefix(path, "/v1/completions"),
		strings.HasPrefix(path, "/v1/moderations"),
		strings.HasPrefix(path, "/pg/chat/completions"):
		return relayconstant.RelayFormatOpenAI
	default:
		return ""
	}
}
