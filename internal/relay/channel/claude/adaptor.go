package claude

import (
	"errors"
	"fmt"
	"io"
	"net/http"

	modelconfig "github.com/NookMux/NookMux/internal/config/model"
	"github.com/NookMux/NookMux/internal/domain/shared"
	"github.com/NookMux/NookMux/internal/relay/channel"
	relaycommon "github.com/NookMux/NookMux/internal/relay/common"
	"github.com/NookMux/NookMux/internal/relay/wire/convert"

	relayconstant "github.com/NookMux/NookMux/internal/relay/constant"
	"github.com/NookMux/NookMux/internal/relay/helper"
	"github.com/gin-gonic/gin"
)

type Adaptor struct {
}

func (a *Adaptor) ConvertGeminiRequest(c *gin.Context, info *relaycommon.RelayInfo, request *shared.GeminiChatRequest) (any, error) {
	if request == nil {
		return nil, errors.New("request is nil")
	}

	openAIRequest, err := helper.GeminiToOpenAIRequest(request, info)
	if err != nil {
		return nil, err
	}
	return a.ConvertOpenAIRequest(c, info, openAIRequest)
}

func (a *Adaptor) ConvertClaudeRequest(c *gin.Context, info *relaycommon.RelayInfo, request *shared.ClaudeRequest) (any, error) {
	return request, nil
}

func (a *Adaptor) ConvertAudioRequest(c *gin.Context, info *relaycommon.RelayInfo, request shared.AudioRequest) (io.Reader, error) {
	//TODO implement me
	return nil, errors.New("not implemented")
}

func (a *Adaptor) ConvertImageRequest(c *gin.Context, info *relaycommon.RelayInfo, request shared.ImageRequest) (any, error) {
	//TODO implement me
	return nil, errors.New("not implemented")
}

func (a *Adaptor) Init(info *relaycommon.RelayInfo) {
}

func (a *Adaptor) GetRequestURL(info *relaycommon.RelayInfo) (string, error) {
	baseURL := fmt.Sprintf("%s/v1/messages", info.ChannelBaseUrl)
	if info.IsClaudeBetaQuery {
		baseURL = baseURL + "?beta=true"
	}
	return baseURL, nil
}

func CommonClaudeHeadersOperation(c *gin.Context, req *http.Header, info *relaycommon.RelayInfo) {
	if c == nil || c.Request == nil || req == nil {
		return
	}
	// common headers operation
	anthropicBeta := c.Request.Header.Get("anthropic-beta")
	if anthropicBeta != "" {
		req.Set("anthropic-beta", anthropicBeta)
	}
	originModel := ""
	if info != nil {
		originModel = info.OriginModelName
	}
	modelconfig.GetClaudeSettings().WriteHeaders(originModel, req)
}

func (a *Adaptor) SetupRequestHeader(c *gin.Context, req *http.Header, info *relaycommon.RelayInfo) error {
	channel.SetupApiRequestHeader(info, c, req)
	req.Set("x-api-key", info.ApiKey)
	anthropicVersion := c.Request.Header.Get("anthropic-version")
	if anthropicVersion == "" {
		anthropicVersion = "2023-06-01"
	}
	req.Set("anthropic-version", anthropicVersion)
	CommonClaudeHeadersOperation(c, req, info)
	return nil
}

func (a *Adaptor) ConvertOpenAIRequest(c *gin.Context, info *relaycommon.RelayInfo, request *shared.GeneralOpenAIRequest) (any, error) {
	if request == nil {
		return nil, errors.New("request is nil")
	}
	return RequestOpenAI2ClaudeMessage(c, info, *request)
}

func (a *Adaptor) ConvertRerankRequest(c *gin.Context, relayMode int, request shared.RerankRequest) (any, error) {
	return nil, nil
}

func (a *Adaptor) ConvertEmbeddingRequest(c *gin.Context, info *relaycommon.RelayInfo, request shared.EmbeddingRequest) (any, error) {
	//TODO implement me
	return nil, errors.New("not implemented")
}

func (a *Adaptor) ConvertOpenAIResponsesRequest(c *gin.Context, info *relaycommon.RelayInfo, request shared.OpenAIResponsesRequest) (any, error) {
	cv := convert.NewConverter(shared.OpenAIWireAPIChat, shared.OpenAIWireAPIResponses)
	chatReq, err := cv.ConvertResponsesToChatRequest(&request)
	if err != nil {
		return nil, err
	}
	if info != nil {
		info.OpenAIResponsesToolContext = cv.ToolContext
		relaycommon.AppendRequestConversionFromRequest(info, chatReq)
		if info.ClaudeConvertInfo == nil {
			info.ClaudeConvertInfo = &relaycommon.ClaudeConvertInfo{LastMessagesType: relaycommon.LastMessageTypeNone}
		}
	}
	claudeReq, err := a.ConvertOpenAIRequest(c, info, chatReq)
	if err != nil {
		return nil, err
	}
	relaycommon.AppendRequestConversionFromRequest(info, claudeReq)
	return claudeReq, nil
}

func (a *Adaptor) DoRequest(c *gin.Context, info *relaycommon.RelayInfo, requestBody io.Reader) (any, error) {
	return channel.DoApiRequest(a, c, info, requestBody)
}

func (a *Adaptor) DoResponse(c *gin.Context, resp *http.Response, info *relaycommon.RelayInfo) (usage any, err *shared.NookMuxError) {
	info.FinalRequestRelayFormat = relayconstant.RelayFormatClaude
	if info.IsStream {
		return ClaudeStreamHandler(c, resp, info)
	} else {
		return ClaudeHandler(c, resp, info)
	}
}

func (a *Adaptor) GetModelList() []string {
	return ModelList
}

func (a *Adaptor) GetChannelName() string {
	return ChannelName
}
