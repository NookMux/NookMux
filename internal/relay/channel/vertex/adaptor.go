package vertex

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/NookMux/NookMux/internal/domain/shared"
	"github.com/NookMux/NookMux/internal/relay/channel"
	"github.com/NookMux/NookMux/internal/relay/channel/claude"
	"github.com/NookMux/NookMux/internal/relay/channel/gemini"
	"github.com/NookMux/NookMux/internal/relay/channel/openai"
	relaycommon "github.com/NookMux/NookMux/internal/relay/common"
	"github.com/NookMux/NookMux/internal/relay/constant"

	"github.com/NookMux/NookMux/pkg/jsonx"

	"github.com/gin-gonic/gin"
)

const (
	RequestModeClaude     = 1
	RequestModeGemini     = 2
	RequestModeOpenSource = 3
)

var claudeModelMap = map[string]string{
	"claude-3-sonnet-20240229":   "claude-3-sonnet@20240229",
	"claude-3-opus-20240229":     "claude-3-opus@20240229",
	"claude-3-haiku-20240307":    "claude-3-haiku@20240307",
	"claude-3-5-sonnet-20240620": "claude-3-5-sonnet@20240620",
	"claude-3-5-sonnet-20241022": "claude-3-5-sonnet-v2@20241022",
	"claude-3-7-sonnet-20250219": "claude-3-7-sonnet@20250219",
	"claude-sonnet-4-20250514":   "claude-sonnet-4@20250514",
	"claude-opus-4-20250514":     "claude-opus-4@20250514",
	"claude-opus-4-1-20250805":   "claude-opus-4-1@20250805",
	"claude-sonnet-4-5-20250929": "claude-sonnet-4-5@20250929",
	"claude-haiku-4-5-20251001":  "claude-haiku-4-5@20251001",
	"claude-opus-4-5-20251101":   "claude-opus-4-5@20251101",
	"claude-opus-4-6":            "claude-opus-4-6",
}

const anthropicVersion = "vertex-2023-10-16"

type Adaptor struct {
	RequestMode        int
	AccountCredentials Credentials
}

func (a *Adaptor) ConvertGeminiRequest(c *gin.Context, info *relaycommon.RelayInfo, request *shared.GeminiChatRequest) (any, error) {
	// Vertex AI does not support functionResponse.id; strip it unconditionally.
	removeFunctionResponseID(request)
	geminiAdaptor := gemini.Adaptor{}
	return geminiAdaptor.ConvertGeminiRequest(c, info, request)
}

func removeFunctionResponseID(request *shared.GeminiChatRequest) {
	if request == nil {
		return
	}

	if len(request.Contents) > 0 {
		for i := range request.Contents {
			if len(request.Contents[i].Parts) == 0 {
				continue
			}
			for j := range request.Contents[i].Parts {
				part := &request.Contents[i].Parts[j]
				if part.FunctionResponse == nil {
					continue
				}
				if len(part.FunctionResponse.ID) > 0 {
					part.FunctionResponse.ID = nil
				}
			}
		}
	}

	if len(request.Requests) > 0 {
		for i := range request.Requests {
			removeFunctionResponseID(&request.Requests[i])
		}
	}
}

func (a *Adaptor) ConvertClaudeRequest(c *gin.Context, info *relaycommon.RelayInfo, request *shared.ClaudeRequest) (any, error) {
	if v, ok := claudeModelMap[info.UpstreamModelName]; ok {
		c.Set("request_model", v)
	} else {
		c.Set("request_model", request.Model)
	}
	vertexClaudeReq := copyRequest(request, anthropicVersion)
	return vertexClaudeReq, nil
}

func (a *Adaptor) ConvertAudioRequest(c *gin.Context, info *relaycommon.RelayInfo, request shared.AudioRequest) (io.Reader, error) {
	//TODO implement me
	return nil, errors.New("not implemented")
}

func (a *Adaptor) ConvertImageRequest(c *gin.Context, info *relaycommon.RelayInfo, request shared.ImageRequest) (any, error) {
	geminiAdaptor := gemini.Adaptor{}
	return geminiAdaptor.ConvertImageRequest(c, info, request)
}

func (a *Adaptor) Init(info *relaycommon.RelayInfo) {
	if strings.HasPrefix(info.UpstreamModelName, "claude") {
		a.RequestMode = RequestModeClaude
	} else if strings.Contains(info.UpstreamModelName, "llama") ||
		// open source models
		strings.Contains(info.UpstreamModelName, "-maas") {
		a.RequestMode = RequestModeOpenSource
	} else {
		a.RequestMode = RequestModeGemini
	}
}

func (a *Adaptor) getRequestUrl(info *relaycommon.RelayInfo, modelName, suffix string) (string, error) {
	region := GetModelRegion(info.ApiVersion, info.OriginModelName)
	if info.ChannelOtherSettings.VertexKeyType != shared.VertexKeyTypeAPIKey {
		adc := &Credentials{}
		if err := jsonx.Unmarshal([]byte(info.ApiKey), adc); err != nil {
			return "", fmt.Errorf("failed to decode credentials file: %w", err)
		}
		a.AccountCredentials = *adc

		if a.RequestMode == RequestModeGemini {
			return BuildGoogleModelURL(info.ChannelBaseUrl, DefaultAPIVersion, adc.ProjectID, region, modelName, suffix), nil
		} else if a.RequestMode == RequestModeClaude {
			return BuildAnthropicModelURL(info.ChannelBaseUrl, DefaultAPIVersion, adc.ProjectID, region, modelName, suffix), nil
		} else if a.RequestMode == RequestModeOpenSource {
			return BuildOpenSourceChatCompletionsURL(info.ChannelBaseUrl, adc.ProjectID, region), nil
		}
	} else {
		if a.RequestMode == RequestModeOpenSource {
			return "", errors.New("vertex api key mode does not support open-source model endpoints; configure service account JSON credentials")
		}
		if a.RequestMode == RequestModeGemini {
			return BuildGoogleModelURL(info.ChannelBaseUrl, DefaultAPIVersion, "", region, modelName, suffix), nil
		} else if a.RequestMode == RequestModeClaude {
			return BuildAnthropicModelURL(info.ChannelBaseUrl, DefaultAPIVersion, "", region, modelName, suffix), nil
		}
	}
	return "", errors.New("unsupported request mode")
}

func (a *Adaptor) GetRequestURL(info *relaycommon.RelayInfo) (string, error) {
	suffix := ""
	if a.RequestMode == RequestModeGemini {
		if info.IsStream {
			suffix = "streamGenerateContent?alt=sse"
		} else {
			suffix = "generateContent"
		}

		if strings.HasPrefix(info.UpstreamModelName, "imagen") {
			suffix = "predict"
		}

		if info.RelayMode == constant.RelayModeGemini && strings.Contains(info.RequestURLPath, ":countTokens") {
			suffix = "countTokens"
		}
		return a.getRequestUrl(info, info.UpstreamModelName, suffix)
	} else if a.RequestMode == RequestModeClaude {
		if info.IsStream {
			suffix = "streamRawPredict?alt=sse"
		} else {
			suffix = "rawPredict"
		}
		model := info.UpstreamModelName
		if v, ok := claudeModelMap[info.UpstreamModelName]; ok {
			model = v
		}
		return a.getRequestUrl(info, model, suffix)
	} else if a.RequestMode == RequestModeOpenSource {
		return a.getRequestUrl(info, "", "")
	}
	return "", errors.New("unsupported request mode")
}

func (a *Adaptor) SetupRequestHeader(c *gin.Context, req *http.Header, info *relaycommon.RelayInfo) error {
	channel.SetupApiRequestHeader(info, c, req)
	if info.ChannelOtherSettings.VertexKeyType != shared.VertexKeyTypeAPIKey {
		accessToken, err := getAccessToken(a, info)
		if err != nil {
			return err
		}
		req.Set("Authorization", "Bearer "+accessToken)
	} else {
		// Google 官方推荐 API key 通过请求头传递，拼进 URL query 会随访问日志泄露。
		req.Set("x-goog-api-key", info.ApiKey)
	}
	if a.AccountCredentials.ProjectID != "" {
		req.Set("x-goog-user-project", a.AccountCredentials.ProjectID)
	}
	if strings.Contains(info.UpstreamModelName, "claude") {
		claude.CommonClaudeHeadersOperation(c, req, info)
	}
	return nil
}

func (a *Adaptor) ConvertOpenAIRequest(c *gin.Context, info *relaycommon.RelayInfo, request *shared.GeneralOpenAIRequest) (any, error) {
	if request == nil {
		return nil, errors.New("request is nil")
	}
	if a.RequestMode == RequestModeGemini && strings.HasPrefix(info.UpstreamModelName, "imagen") {
		prompt := ""
		for _, m := range request.Messages {
			if m.Role == "user" {
				prompt = m.StringContent()
				if prompt != "" {
					break
				}
			}
		}
		if prompt == "" {
			if p, ok := request.Prompt.(string); ok {
				prompt = p
			}
		}
		if prompt == "" {
			return nil, errors.New("prompt is required for image generation")
		}

		imgReq := shared.ImageRequest{
			Model:  request.Model,
			Prompt: prompt,
			N:      1,
			Size:   "1024x1024",
		}
		if request.N > 0 {
			imgReq.N = uint(request.N)
		}
		if request.Size != "" {
			imgReq.Size = request.Size
		}
		if len(request.ExtraBody) > 0 {
			var extra map[string]any
			if err := jsonx.Unmarshal(request.ExtraBody, &extra); err == nil {
				if n, ok := extra["n"].(float64); ok && n > 0 {
					imgReq.N = uint(n)
				}
				if size, ok := extra["size"].(string); ok {
					imgReq.Size = size
				}
				// accept aspectRatio in extra body (top-level or under parameters)
				if ar, ok := extra["aspectRatio"].(string); ok && ar != "" {
					imgReq.Size = ar
				}
				if params, ok := extra["parameters"].(map[string]any); ok {
					if ar, ok := params["aspectRatio"].(string); ok && ar != "" {
						imgReq.Size = ar
					}
				}
			}
		}
		c.Set("request_model", request.Model)
		return a.ConvertImageRequest(c, info, imgReq)
	}
	if a.RequestMode == RequestModeClaude {
		claudeReq, err := claude.RequestOpenAI2ClaudeMessage(c, info, *request)
		if err != nil {
			return nil, err
		}
		vertexClaudeReq := copyRequest(claudeReq, anthropicVersion)
		c.Set("request_model", claudeReq.Model)
		info.UpstreamModelName = claudeReq.Model
		return vertexClaudeReq, nil
	} else if a.RequestMode == RequestModeGemini {
		geminiRequest, err := gemini.CovertOpenAI2Gemini(c, *request, info)
		if err != nil {
			return nil, err
		}
		c.Set("request_model", request.Model)
		return geminiRequest, nil
	} else if a.RequestMode == RequestModeOpenSource {
		return request, nil
	}
	return nil, errors.New("unsupported request mode")
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
	if a.RequestMode != RequestModeGemini || requestBody == nil {
		return channel.DoApiRequest(a, c, info, requestBody)
	}

	bodyBytes, err := io.ReadAll(requestBody)
	if err != nil {
		return nil, fmt.Errorf("read vertex request body failed: %w", err)
	}

	// Only rewrite payload when needed; avoid touching unrelated requests.
	if bytes.Contains(bodyBytes, []byte("functionResponse")) || bytes.Contains(bodyBytes, []byte("function_response")) {
		cleaned, err := stripGeminiFunctionResponseIDFromJSON(bodyBytes)
		if err != nil {
			return nil, err
		}
		bodyBytes = cleaned
	}

	return channel.DoApiRequest(a, c, info, bytes.NewReader(bodyBytes))
}

func (a *Adaptor) DoResponse(c *gin.Context, resp *http.Response, info *relaycommon.RelayInfo) (usage any, err *shared.NookMuxError) {
	claudeAdaptor := claude.Adaptor{}
	if info.IsStream {
		switch a.RequestMode {
		case RequestModeClaude:
			return claudeAdaptor.DoResponse(c, resp, info)
		case RequestModeGemini:
			if info.RelayMode == constant.RelayModeGemini {
				return gemini.GeminiTextGenerationStreamHandler(c, info, resp)
			} else {
				return gemini.GeminiChatStreamHandler(c, info, resp)
			}
		case RequestModeOpenSource:
			return openai.OaiStreamHandler(c, info, resp)
		}
	} else {
		switch a.RequestMode {
		case RequestModeClaude:
			return claudeAdaptor.DoResponse(c, resp, info)
		case RequestModeGemini:
			if info.RelayMode == constant.RelayModeGemini {
				if strings.Contains(info.RequestURLPath, ":countTokens") {
					return gemini.NativeGeminiCountTokensHandler(c, resp, info)
				}
				return gemini.GeminiTextGenerationHandler(c, info, resp)
			} else {
				if strings.HasPrefix(info.UpstreamModelName, "imagen") {
					return gemini.GeminiImageHandler(c, info, resp)
				}
				return gemini.GeminiChatHandler(c, info, resp)
			}
		case RequestModeOpenSource:
			return openai.OpenaiHandler(c, info, resp)
		}
	}
	return
}

func (a *Adaptor) GetModelList() []string {
	var modelList []string
	for i, s := range ModelList {
		modelList = append(modelList, s)
		ModelList[i] = s
	}
	for i, s := range claude.ModelList {
		modelList = append(modelList, s)
		claude.ModelList[i] = s
	}
	for i, s := range gemini.ModelList {
		modelList = append(modelList, s)
		gemini.ModelList[i] = s
	}
	return modelList
}

func (a *Adaptor) GetChannelName() string {
	return ChannelName
}

func stripGeminiFunctionResponseIDFromJSON(jsonData []byte) ([]byte, error) {
	var data map[string]interface{}
	if err := jsonx.Unmarshal(jsonData, &data); err != nil {
		return nil, fmt.Errorf("invalid gemini request body: %w", err)
	}

	stripGeminiFunctionResponseIDFromMap(data)

	cleaned, err := jsonx.Marshal(data)
	if err != nil {
		return nil, fmt.Errorf("marshal cleaned gemini request body failed: %w", err)
	}
	return cleaned, nil
}

func stripGeminiFunctionResponseIDFromMap(data map[string]interface{}) {
	if data == nil {
		return
	}

	// contents[].parts[].functionResponse.id / function_response.id
	if contents, ok := data["contents"].([]interface{}); ok {
		for _, content := range contents {
			contentMap, ok := content.(map[string]interface{})
			if !ok {
				continue
			}
			parts, ok := contentMap["parts"].([]interface{})
			if !ok {
				continue
			}
			for _, part := range parts {
				partMap, ok := part.(map[string]interface{})
				if !ok {
					continue
				}
				if funcResp, ok := partMap["functionResponse"].(map[string]interface{}); ok {
					delete(funcResp, "id")
				}
				if funcResp, ok := partMap["function_response"].(map[string]interface{}); ok {
					delete(funcResp, "id")
				}
			}
		}
	}

	// requests[]: batch style Gemini payload
	if requests, ok := data["requests"].([]interface{}); ok {
		for _, r := range requests {
			if requestMap, ok := r.(map[string]interface{}); ok {
				stripGeminiFunctionResponseIDFromMap(requestMap)
			}
		}
	}
}
