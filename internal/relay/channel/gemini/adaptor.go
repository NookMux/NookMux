package gemini

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/NookMux/NookMux/internal/config/model"
	"github.com/NookMux/NookMux/internal/domain/shared"
	"github.com/NookMux/NookMux/internal/relay/channel"
	"github.com/NookMux/NookMux/internal/relay/channel/openai"
	relaycommon "github.com/NookMux/NookMux/internal/relay/common"
	"github.com/NookMux/NookMux/internal/relay/constant"

	"github.com/NookMux/NookMux/internal/relay/helper"
	"github.com/gin-gonic/gin"
)

type Adaptor struct {
}

func (a *Adaptor) ConvertGeminiRequest(c *gin.Context, info *relaycommon.RelayInfo, request *shared.GeminiChatRequest) (any, error) {
	if len(request.Contents) > 0 {
		for i, content := range request.Contents {
			if i == 0 {
				if request.Contents[0].Role == "" {
					request.Contents[0].Role = "user"
				}
			}
			for _, part := range content.Parts {
				if part.FileData != nil {
					if part.FileData.MimeType == "" && strings.Contains(part.FileData.FileUri, "www.youtube.com") {
						part.FileData.MimeType = "video/webm"
					}
				}
			}
		}
	}
	return request, nil
}

func (a *Adaptor) ConvertClaudeRequest(c *gin.Context, info *relaycommon.RelayInfo, req *shared.ClaudeRequest) (any, error) {
	oaiReq, err := helper.ClaudeToOpenAIRequest(*req, info)
	if err != nil {
		return nil, err
	}
	// Gemini 渠道对 Claude 入站走官方 OpenAI 兼容层直通：经 ClaudeToOpenAIRequest
	// 转成 OpenAI 请求后透传上游，thinking 已映射为 ReasoningEffort；不再注入
	// openrouter 风格 reasoning 载荷（兼容层不识别）。
	return a.ConvertOpenAIRequest(c, info, oaiReq)
}

func (a *Adaptor) ConvertAudioRequest(c *gin.Context, info *relaycommon.RelayInfo, request shared.AudioRequest) (io.Reader, error) {
	//TODO implement me
	return nil, errors.New("not implemented")
}

func (a *Adaptor) ConvertImageRequest(c *gin.Context, info *relaycommon.RelayInfo, request shared.ImageRequest) (any, error) {
	if !strings.HasPrefix(info.UpstreamModelName, "imagen") {
		return nil, errors.New("not supported model for image generation")
	}

	// convert size to aspect ratio but allow user to specify aspect ratio
	aspectRatio := "1:1" // default aspect ratio
	size := strings.TrimSpace(request.Size)
	if size != "" {
		if strings.Contains(size, ":") {
			aspectRatio = size
		} else {
			switch size {
			case "256x256", "512x512", "1024x1024":
				aspectRatio = "1:1"
			case "1536x1024":
				aspectRatio = "3:2"
			case "1024x1536":
				aspectRatio = "2:3"
			case "1024x1792":
				aspectRatio = "9:16"
			case "1792x1024":
				aspectRatio = "16:9"
			}
		}
	}

	// build gemini imagen request
	geminiRequest := shared.GeminiImageRequest{
		Instances: []shared.GeminiImageInstance{
			{
				Prompt: request.Prompt,
			},
		},
		Parameters: shared.GeminiImageParameters{
			SampleCount:      int(request.N),
			AspectRatio:      aspectRatio,
			PersonGeneration: "allow_adult", // default allow adult
		},
	}

	// Set imageSize when quality parameter is specified
	// Map quality parameter to imageSize (only supported by Standard and Ultra models)
	// quality values: auto, high, medium, low (for gpt-image-1), hd, standard (for dall-e-3)
	// imageSize values: 1K (default), 2K
	// https://ai.google.dev/gemini-api/docs/imagen
	// https://platform.openai.com/docs/api-reference/images/create
	if request.Quality != "" {
		var imageSize string
		switch request.Quality {
		case "hd", "high", "2K":
			imageSize = "2K"
		case "standard", "medium", "low", "auto", "1K":
			imageSize = "1K"
		default:
			// unknown quality value, default to 1K
			imageSize = "1K"
		}
		geminiRequest.Parameters.ImageSize = imageSize
	}

	return geminiRequest, nil
}

func (a *Adaptor) Init(info *relaycommon.RelayInfo) {

}

func (a *Adaptor) GetRequestURL(info *relaycommon.RelayInfo) (string, error) {
	version := model.GetGeminiVersionSetting(info.UpstreamModelName)

	if strings.HasPrefix(info.UpstreamModelName, "imagen") {
		return fmt.Sprintf("%s/%s/models/%s:predict", info.ChannelBaseUrl, version, info.UpstreamModelName), nil
	}

	if strings.HasPrefix(info.UpstreamModelName, "text-embedding") ||
		strings.HasPrefix(info.UpstreamModelName, "embedding") ||
		strings.HasPrefix(info.UpstreamModelName, "gemini-embedding") {
		action := "embedContent"
		if info.IsGeminiBatchEmbedding {
			action = "batchEmbedContents"
		}
		return fmt.Sprintf("%s/%s/models/%s:%s", info.ChannelBaseUrl, version, info.UpstreamModelName, action), nil
	}

	if info.RelayMode == constant.RelayModeGemini && strings.Contains(info.RequestURLPath, ":countTokens") {
		return fmt.Sprintf("%s/%s/models/%s:countTokens", info.ChannelBaseUrl, version, info.UpstreamModelName), nil
	}

	// OpenAI / Claude 入站 chat 走 Google 官方 OpenAI 兼容层直通，固定 v1beta/openai
	// 路径（不吃 GetGeminiVersionSetting），标准 OpenAI SSE，鉴权 Bearer。
	if info.RelayFormat == constant.RelayFormatOpenAI || info.RelayFormat == constant.RelayFormatClaude {
		return fmt.Sprintf("%s/v1beta/openai/chat/completions", info.ChannelBaseUrl), nil
	}

	action := "generateContent"
	if info.IsStream {
		action = "streamGenerateContent?alt=sse"
		if info.RelayMode == constant.RelayModeGemini {
			info.DisablePing = true
		}
	}
	return fmt.Sprintf("%s/%s/models/%s:%s", info.ChannelBaseUrl, version, info.UpstreamModelName, action), nil
}

func (a *Adaptor) SetupRequestHeader(c *gin.Context, req *http.Header, info *relaycommon.RelayInfo) error {
	channel.SetupApiRequestHeader(info, c, req)
	// 兼容层使用标准 Authorization: Bearer；原生 Gemini 端点用 x-goog-api-key。
	if info.RelayFormat == constant.RelayFormatOpenAI || info.RelayFormat == constant.RelayFormatClaude {
		if info.ApiKey != "" {
			req.Set("Authorization", "Bearer "+info.ApiKey)
		}
		return nil
	}
	req.Set("x-goog-api-key", info.ApiKey)
	return nil
}

func (a *Adaptor) ConvertOpenAIRequest(c *gin.Context, info *relaycommon.RelayInfo, request *shared.GeneralOpenAIRequest) (any, error) {
	if request == nil {
		return nil, errors.New("request is nil")
	}
	// 直通官方 OpenAI 兼容层：请求体恒等透传，仅在流式下补 stream_options.include_usage
	// 以保证计费 usage 准确（Google 兼容层支持）。
	if info.IsStream && info.SupportStreamOptions {
		request.StreamOptions = &shared.StreamOptions{IncludeUsage: true}
	}
	return request, nil
}

func (a *Adaptor) ConvertRerankRequest(c *gin.Context, relayMode int, request shared.RerankRequest) (any, error) {
	return nil, nil
}

func (a *Adaptor) ConvertEmbeddingRequest(c *gin.Context, info *relaycommon.RelayInfo, request shared.EmbeddingRequest) (any, error) {
	if request.Input == nil {
		return nil, errors.New("input is required")
	}

	inputs := request.ParseInput()
	if len(inputs) == 0 {
		return nil, errors.New("input is empty")
	}
	// We always build a batch-style payload with `requests`, so ensure we call the
	// batch endpoint upstream to avoid payload/endpoint mismatches.
	info.IsGeminiBatchEmbedding = true
	// process all inputs
	geminiRequests := make([]map[string]interface{}, 0, len(inputs))
	for _, input := range inputs {
		geminiRequest := map[string]interface{}{
			"model": fmt.Sprintf("models/%s", info.UpstreamModelName),
			"content": shared.GeminiChatContent{
				Parts: []shared.GeminiPart{
					{
						Text: input,
					},
				},
			},
		}

		// set specific parameters for different models
		// https://ai.google.dev/api/embeddings?hl=zh-cn#method:-models.embedcontent
		switch info.UpstreamModelName {
		case "text-embedding-004", "gemini-embedding-exp-03-07", "gemini-embedding-001", "gemini-embedding-2":
			// Only newer models introduced after 2024 support OutputDimensionality
			if request.Dimensions > 0 {
				geminiRequest["outputDimensionality"] = request.Dimensions
			}
		}
		geminiRequests = append(geminiRequests, geminiRequest)
	}

	return map[string]interface{}{
		"requests": geminiRequests,
	}, nil
}

func (a *Adaptor) ConvertOpenAIResponsesRequest(c *gin.Context, info *relaycommon.RelayInfo, request shared.OpenAIResponsesRequest) (any, error) {
	// TODO implement me
	return nil, errors.New("not implemented")
}

func (a *Adaptor) DoRequest(c *gin.Context, info *relaycommon.RelayInfo, requestBody io.Reader) (any, error) {
	return channel.DoApiRequest(a, c, info, requestBody)
}

func (a *Adaptor) DoResponse(c *gin.Context, resp *http.Response, info *relaycommon.RelayInfo) (usage any, err *shared.NookMuxError) {
	// OpenAI / Claude 入站 chat 直通官方兼容层：响应为 OpenAI wire，委托 openai
	// adaptor 处理（其内部按 RelayFormatClaude 回写 Claude Messages 格式）。
	if info.RelayFormat == constant.RelayFormatOpenAI || info.RelayFormat == constant.RelayFormatClaude {
		delegate := &openai.Adaptor{}
		delegate.Init(info)
		return delegate.DoResponse(c, resp, info)
	}

	if info.RelayMode == constant.RelayModeGemini {
		if strings.Contains(info.RequestURLPath, ":embedContent") ||
			strings.Contains(info.RequestURLPath, ":batchEmbedContents") {
			return NativeGeminiEmbeddingHandler(c, resp, info)
		}
		if strings.Contains(info.RequestURLPath, ":countTokens") {
			return NativeGeminiCountTokensHandler(c, resp, info)
		}
		if info.IsStream {
			return GeminiTextGenerationStreamHandler(c, info, resp)
		} else {
			return GeminiTextGenerationHandler(c, info, resp)
		}
	}

	if strings.HasPrefix(info.UpstreamModelName, "imagen") {
		return GeminiImageHandler(c, info, resp)
	}

	// check if the model is an embedding model
	if strings.HasPrefix(info.UpstreamModelName, "text-embedding") ||
		strings.HasPrefix(info.UpstreamModelName, "embedding") ||
		strings.HasPrefix(info.UpstreamModelName, "gemini-embedding") {
		return GeminiEmbeddingHandler(c, info, resp)
	}

	// OpenAI/Claude chat 已由兼容层分支委托，原生 Gemini 已由上方 RelayModeGemini
	// 分支处理，到达此处说明入站格式与模型均不在 Gemini 渠道支持范围内。
	return nil, shared.NewOpenAIError(
		fmt.Errorf("unsupported relay format/mode for gemini channel: format=%s mode=%d model=%s",
			info.RelayFormat, info.RelayMode, info.UpstreamModelName),
		shared.ErrorCodeInvalidRequest,
		http.StatusBadRequest,
	)
}

func (a *Adaptor) GetModelList() []string {
	return ModelList
}

func (a *Adaptor) GetChannelName() string {
	return ChannelName
}
