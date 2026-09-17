package aws

import (
	"fmt"
	"io"
	"net/http"

	"github.com/NookMux/NookMux/internal/domain/shared"
	"github.com/NookMux/NookMux/internal/relay/channel/claude"
	relaycommon "github.com/NookMux/NookMux/internal/relay/common"

	"github.com/aws/aws-sdk-go-v2/service/bedrockruntime"
	"github.com/pkg/errors"

	media "github.com/NookMux/NookMux/internal/infra/media"
	"github.com/gin-gonic/gin"
)

type Adaptor struct {
	AwsClient *bedrockruntime.Client
	AwsReq    any
	IsNova    bool
}

func (a *Adaptor) ConvertGeminiRequest(*gin.Context, *relaycommon.RelayInfo, *shared.GeminiChatRequest) (any, error) {
	//TODO implement me
	return nil, errors.New("not implemented")
}

func (a *Adaptor) ConvertClaudeRequest(c *gin.Context, info *relaycommon.RelayInfo, request *shared.ClaudeRequest) (any, error) {
	for i, message := range request.Messages {
		updated := false
		if !message.IsStringContent() {
			content, err := message.ParseContent()
			if err != nil {
				return nil, errors.Wrap(err, "failed to parse message content")
			}
			for i2, mediaMessage := range content {
				if mediaMessage.Source != nil {
					if mediaMessage.Source.Type == "url" {
						// 使用统一的文件服务获取图片数据
						source := shared.NewURLFileSource(mediaMessage.Source.Url)
						base64Data, mimeType, err := media.GetBase64Data(c, source, "formatting image for Claude")
						if err != nil {
							return nil, fmt.Errorf("get file base64 from url failed: %s", err.Error())
						}
						mediaMessage.Source.MediaType = mimeType
						mediaMessage.Source.Data = base64Data
						mediaMessage.Source.Url = ""
						mediaMessage.Source.Type = "base64"
						content[i2] = mediaMessage
						updated = true
					}
				}
			}
			if updated {
				message.SetContent(content)
			}
		}
		if updated {
			request.Messages[i] = message
		}
	}
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
	if _, err := parseAwsCredential(info.ApiKey, info.ChannelOtherSettings.AwsKeyType); err != nil {
		return "", err
	}
	return "", nil
}

func (a *Adaptor) SetupRequestHeader(c *gin.Context, req *http.Header, info *relaycommon.RelayInfo) error {
	claude.CommonClaudeHeadersOperation(c, req, info)
	return nil
}

func (a *Adaptor) ConvertOpenAIRequest(c *gin.Context, info *relaycommon.RelayInfo, request *shared.GeneralOpenAIRequest) (any, error) {
	if request == nil {
		return nil, errors.New("request is nil")
	}
	// 检查是否为Nova模型
	if isNovaModel(request.Model) {
		novaReq := convertToNovaRequest(request)
		a.IsNova = true
		return novaReq, nil
	}

	// 原有的Claude模型处理逻辑
	claudeReq, err := claude.RequestOpenAI2ClaudeMessage(c, info, *request)
	if err != nil {
		return nil, errors.Wrap(err, "failed to convert openai request to claude request")
	}
	info.UpstreamModelName = claudeReq.Model
	return claudeReq, err
}

func (a *Adaptor) ConvertRerankRequest(c *gin.Context, relayMode int, request shared.RerankRequest) (any, error) {
	return nil, nil
}

func (a *Adaptor) ConvertEmbeddingRequest(c *gin.Context, info *relaycommon.RelayInfo, request shared.EmbeddingRequest) (any, error) {
	//TODO implement me
	return nil, errors.New("not implemented")
}

func (a *Adaptor) ConvertOpenAIResponsesRequest(c *gin.Context, info *relaycommon.RelayInfo, request shared.OpenAIResponsesRequest) (any, error) {
	// TODO implement me
	return nil, errors.New("not implemented")
}

func (a *Adaptor) DoRequest(c *gin.Context, info *relaycommon.RelayInfo, requestBody io.Reader) (any, error) {
	return doAwsClientRequest(c, info, a, requestBody)
}

func (a *Adaptor) DoResponse(c *gin.Context, _ *http.Response, info *relaycommon.RelayInfo) (usage any, err *shared.NookMuxError) {
	if a.IsNova {
		err, usage = handleNovaRequest(c, info, a)
	} else if info.IsStream {
		err, usage = awsStreamHandler(c, info, a)
	} else {
		err, usage = awsHandler(c, info, a)
	}
	return
}

func (a *Adaptor) GetModelList() (models []string) {
	for n := range awsModelIDMap {
		models = append(models, n)
	}

	return
}

func (a *Adaptor) GetChannelName() string {
	return ChannelName
}
