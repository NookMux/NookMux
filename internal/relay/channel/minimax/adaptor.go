package minimax

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/NookMux/NookMux/internal/config/model"
	"github.com/NookMux/NookMux/internal/domain/shared"
	"github.com/NookMux/NookMux/internal/relay/channel"
	"github.com/NookMux/NookMux/internal/relay/channel/claude"
	"github.com/NookMux/NookMux/internal/relay/channel/openai"
	relaycommon "github.com/NookMux/NookMux/internal/relay/common"
	"github.com/NookMux/NookMux/internal/relay/constant"
	"github.com/NookMux/NookMux/internal/relay/helper"

	"github.com/NookMux/NookMux/pkg/jsonx"

	"github.com/gin-gonic/gin"
)

type Adaptor struct {
}

const miniMaxTTSSupportedAudioFormats = "mp3, opus, flac, wav, pcm"

func normalizeMiniMaxTTSAudioFormat(responseFormat string) (string, error) {
	format := strings.ToLower(strings.TrimSpace(responseFormat))
	if format == "" {
		return "mp3", nil
	}
	switch format {
	case "mp3", "opus", "flac", "wav", "pcm":
		return format, nil
	case "aac":
		return "", fmt.Errorf("upstream TTS does not support response_format %q; supported formats: %s", responseFormat, miniMaxTTSSupportedAudioFormats)
	default:
		return "", fmt.Errorf("unsupported response_format %q for upstream TTS; supported formats: %s", responseFormat, miniMaxTTSSupportedAudioFormats)
	}
}

func (a *Adaptor) ConvertGeminiRequest(*gin.Context, *relaycommon.RelayInfo, *shared.GeminiChatRequest) (any, error) {
	return nil, errors.New("not implemented")
}

func (a *Adaptor) ConvertClaudeRequest(c *gin.Context, info *relaycommon.RelayInfo, req *shared.ClaudeRequest) (any, error) {
	adaptor := claude.Adaptor{}
	return adaptor.ConvertClaudeRequest(c, info, req)
}

func (a *Adaptor) ConvertAudioRequest(c *gin.Context, info *relaycommon.RelayInfo, request shared.AudioRequest) (io.Reader, error) {
	if info.RelayMode != constant.RelayModeAudioSpeech {
		return nil, errors.New("unsupported audio relay mode")
	}

	audioFormat, err := normalizeMiniMaxTTSAudioFormat(request.ResponseFormat)
	if err != nil {
		return nil, err
	}

	// 1) 先用用户请求原始值构造基础请求（不应用管理员重定向）。
	//    使用 UpstreamModelName（渠道级 model_mapping 已由 ModelMappedHelper 处理），
	//    作为 TTS 专用重定向的输入，支持 alias -> tts-1-hd -> speech-02-hd 链式映射。
	minimaxRequest := MiniMaxTTSRequest{
		Model: info.UpstreamModelName,
		Text:  request.Input,
		VoiceSetting: VoiceSetting{
			VoiceID: request.Voice,
			Speed:   request.Speed,
		},
		AudioSetting: &AudioSetting{
			Format: audioFormat,
		},
		OutputFormat: "hex",
	}

	// 2) 合并用户 metadata（扩展字段的厂商自定义值）。
	//    必须在管理员强制策略之前合并，使管理员映射具有最高优先级，
	//    防止用户通过 metadata 覆盖 model/voice_id 等策略字段 (issue #107)。
	if len(request.Metadata) > 0 {
		if err := jsonx.Unmarshal(request.Metadata, &minimaxRequest); err != nil {
			return nil, fmt.Errorf("error unmarshalling metadata to TTS request: %w", err)
		}
	}
	clientVoice := minimaxRequest.VoiceSetting.VoiceID
	if clientVoice == "" {
		clientVoice = request.Voice
	}
	// 音色白名单/重定向已迁移到数据库音色表：始终按用户原始音色 ID 查库，
	// 校验通过后再用 redirect_id 替换发给上游，避免直接传 redirect_id 绕过白名单。
	if request.Voice != "" {
		resolvedRequestVoice, vErr := helper.ResolveVoiceForTTSUpstream(c, request.Voice)
		if vErr != nil {
			return nil, vErr
		}
		minimaxRequest.VoiceSetting.VoiceID = resolvedRequestVoice
	}
	if clientVoice != "" && clientVoice != request.Voice {
		resolvedClientVoice, vErr := helper.ResolveVoiceForTTSUpstream(c, clientVoice)
		if vErr != nil {
			return nil, vErr
		}
		minimaxRequest.VoiceSetting.VoiceID = resolvedClientVoice
	}

	// 3) 应用管理员强制策略：在 metadata 合并之后覆盖策略字段。
	//    仅当 cfg.Enabled 时生效。注意：音色重定向已由上一步数据库解析完成，
	//    策略不再覆盖 VoiceSetting.VoiceID，避免回退掉 redirect_id。
	policy := model.ApplyMiniMaxTTSPolicy(info.UpstreamModelName, request.Voice, request.Input, audioFormat)
	if policy.Enabled {
		minimaxRequest.Model = policy.Model
		minimaxRequest.VoiceSetting.Emotion = policy.Emotion
		minimaxRequest.Text = policy.Text
		if minimaxRequest.AudioSetting == nil {
			minimaxRequest.AudioSetting = &AudioSetting{}
		}
		minimaxRequest.AudioSetting.Format = audioFormat
	}

	if minimaxRequest.AudioSetting == nil {
		minimaxRequest.AudioSetting = &AudioSetting{}
	}
	if minimaxRequest.AudioSetting.Format == "" {
		minimaxRequest.AudioSetting.Format = audioFormat
	}
	minimaxRequest.OutputFormat = "hex"
	c.Set("response_format", "hex")
	c.Set("minimax_audio_format", minimaxRequest.AudioSetting.Format)

	jsonData, err := jsonx.Marshal(minimaxRequest)
	if err != nil {
		return nil, fmt.Errorf("error marshalling TTS request: %w", err)
	}

	// 音色日志：只记录用户请求中的 OpenAI TTS voice，避免暴露重定向后的真实上游 voice_id。
	// 由上层 audio_handler.go 通过 extraContent 传给 PostAudioConsumeQuota。
	if request.Voice != "" {
		c.Set("minimax_voice_id", request.Voice)
	}

	return bytes.NewReader(jsonData), nil
}

func (a *Adaptor) ConvertImageRequest(c *gin.Context, info *relaycommon.RelayInfo, request shared.ImageRequest) (any, error) {
	if info.RelayMode != constant.RelayModeImagesGenerations {
		return nil, fmt.Errorf("unsupported image relay mode: %d", info.RelayMode)
	}
	return oaiImage2MiniMaxImageRequest(request), nil
}

func (a *Adaptor) Init(info *relaycommon.RelayInfo) {
}

func (a *Adaptor) GetRequestURL(info *relaycommon.RelayInfo) (string, error) {
	return GetRequestURL(info)
}

func (a *Adaptor) SetupRequestHeader(c *gin.Context, req *http.Header, info *relaycommon.RelayInfo) error {
	channel.SetupApiRequestHeader(info, c, req)
	req.Set("Authorization", "Bearer "+info.ApiKey)
	return nil
}

func (a *Adaptor) ConvertOpenAIRequest(c *gin.Context, info *relaycommon.RelayInfo, request *shared.GeneralOpenAIRequest) (any, error) {
	if request == nil {
		return nil, errors.New("request is nil")
	}
	return request, nil
}

func (a *Adaptor) ConvertRerankRequest(c *gin.Context, relayMode int, request shared.RerankRequest) (any, error) {
	return nil, nil
}

func (a *Adaptor) ConvertEmbeddingRequest(c *gin.Context, info *relaycommon.RelayInfo, request shared.EmbeddingRequest) (any, error) {
	return request, nil
}

func (a *Adaptor) ConvertOpenAIResponsesRequest(c *gin.Context, info *relaycommon.RelayInfo, request shared.OpenAIResponsesRequest) (any, error) {
	return request, nil
}

func (a *Adaptor) DoRequest(c *gin.Context, info *relaycommon.RelayInfo, requestBody io.Reader) (any, error) {
	return channel.DoApiRequest(a, c, info, requestBody)
}

func (a *Adaptor) DoResponse(c *gin.Context, resp *http.Response, info *relaycommon.RelayInfo) (usage any, err *shared.NookMuxError) {
	switch info.RelayFormat {
	case constant.RelayFormatClaude:
		adaptor := claude.Adaptor{}
		return adaptor.DoResponse(c, resp, info)
	default:
	}

	if info.RelayMode == constant.RelayModeAudioSpeech {
		return handleTTSResponse(c, resp, info)
	}
	if info.RelayMode == constant.RelayModeImagesGenerations {
		return miniMaxImageHandler(c, resp, info)
	}

	adaptor := openai.Adaptor{}
	return adaptor.DoResponse(c, resp, info)
}

func (a *Adaptor) GetModelList() []string {
	return ModelList
}

func (a *Adaptor) GetChannelName() string {
	return ChannelName
}
