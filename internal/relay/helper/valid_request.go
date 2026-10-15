package helper

import (
	"errors"
	"fmt"
	"math"
	"strings"

	"github.com/NookMux/NookMux/internal/common"
	"github.com/NookMux/NookMux/internal/domain/shared"
	"github.com/NookMux/NookMux/internal/infra/log"
	relayconstant "github.com/NookMux/NookMux/internal/relay/constant"

	"github.com/NookMux/NookMux/internal/httpapi"
	"github.com/NookMux/NookMux/pkg/jsonx"
	"github.com/gin-gonic/gin"
)

// maxTokensUpperBound guards against integer-overflow / quota-manipulation via
// extremely large max_tokens-like fields that flow into quota pre-consumption
// estimates. Matches the historical max_tokens validation threshold.
const maxTokensUpperBound = math.MaxInt32 / 2

// maxImageN caps the number of images requested in a single image generation
// request. Without it, an enormous N multiplies into ImagePriceRatio and can
// overflow the int quota, turning a charge into a balance increase.
const maxImageN = 100

func GetAndValidateRequest(c *gin.Context, format relayconstant.RelayFormat) (request shared.Request, err error) {
	relayMode := relayconstant.Path2RelayMode(c.Request.URL.Path)

	switch format {
	case relayconstant.RelayFormatOpenAI:
		request, err = GetAndValidateTextRequest(c, relayMode)
	case relayconstant.RelayFormatGemini:
		if strings.Contains(c.Request.URL.Path, ":embedContent") {
			request, err = GetAndValidateGeminiEmbeddingRequest(c)
		} else if strings.Contains(c.Request.URL.Path, ":batchEmbedContents") {
			request, err = GetAndValidateGeminiBatchEmbeddingRequest(c)
		} else if strings.Contains(c.Request.URL.Path, ":countTokens") {
			request, err = GetAndValidateGeminiCountTokensRequest(c)
		} else {
			request, err = GetAndValidateGeminiRequest(c)
		}
	case relayconstant.RelayFormatClaude:
		request, err = GetAndValidateClaudeRequest(c)
	case relayconstant.RelayFormatOpenAIResponses:
		request, err = GetAndValidateResponsesRequest(c)
	case relayconstant.RelayFormatOpenAIResponsesCompaction:
		request, err = GetAndValidateResponsesCompactionRequest(c)

	case relayconstant.RelayFormatOpenAIImage:
		request, err = GetAndValidOpenAIImageRequest(c, relayMode)
	case relayconstant.RelayFormatEmbedding:
		request, err = GetAndValidateEmbeddingRequest(c, relayMode)
	case relayconstant.RelayFormatRerank:
		request, err = GetAndValidateRerankRequest(c)
	case relayconstant.RelayFormatOpenAIAudio:
		request, err = GetAndValidAudioRequest(c, relayMode)
	case relayconstant.RelayFormatOpenAIRealtime:
		request = &shared.BaseRequest{}
	default:
		return nil, fmt.Errorf("unsupported relay format: %s", format)
	}
	return request, err
}

func GetAndValidAudioRequest(c *gin.Context, relayMode int) (*shared.AudioRequest, error) {
	audioRequest := &shared.AudioRequest{}
	err := httpapi.UnmarshalBodyReusable(c, audioRequest)
	if err != nil {
		return nil, err
	}
	switch relayMode {
	case relayconstant.RelayModeAudioSpeech:
		if audioRequest.Model == "" {
			return nil, errors.New("model is required")
		}
	default:
		if audioRequest.Model == "" {
			return nil, errors.New("model is required")
		}
		if audioRequest.ResponseFormat == "" {
			audioRequest.ResponseFormat = "json"
		}
	}
	return audioRequest, nil
}

func GetAndValidateRerankRequest(c *gin.Context) (*shared.RerankRequest, error) {
	var rerankRequest *shared.RerankRequest
	err := httpapi.UnmarshalBodyReusable(c, &rerankRequest)
	if err != nil {
		log.LogError(c, fmt.Sprintf("getAndValidateTextRequest failed: %s", err.Error()))
		return nil, shared.NewError(err, shared.ErrorCodeInvalidRequest, shared.ErrOptionWithSkipRetry())
	}

	if rerankRequest.Query == "" {
		return nil, shared.NewError(fmt.Errorf("query is empty"), shared.ErrorCodeInvalidRequest, shared.ErrOptionWithSkipRetry())
	}
	if len(rerankRequest.Documents) == 0 {
		return nil, shared.NewError(fmt.Errorf("documents is empty"), shared.ErrorCodeInvalidRequest, shared.ErrOptionWithSkipRetry())
	}
	return rerankRequest, nil
}

func GetAndValidateEmbeddingRequest(c *gin.Context, relayMode int) (*shared.EmbeddingRequest, error) {
	var embeddingRequest *shared.EmbeddingRequest
	err := httpapi.UnmarshalBodyReusable(c, &embeddingRequest)
	if err != nil {
		log.LogError(c, fmt.Sprintf("getAndValidateTextRequest failed: %s", err.Error()))
		return nil, shared.NewError(err, shared.ErrorCodeInvalidRequest, shared.ErrOptionWithSkipRetry())
	}

	if embeddingRequest.Input == nil {
		return nil, fmt.Errorf("input is empty")
	}
	if relayMode == relayconstant.RelayModeModerations && embeddingRequest.Model == "" {
		embeddingRequest.Model = "omni-moderation-latest"
	}
	if relayMode == relayconstant.RelayModeEmbeddings && embeddingRequest.Model == "" {
		embeddingRequest.Model = c.Param("model")
	}
	return embeddingRequest, nil
}

func GetAndValidateResponsesRequest(c *gin.Context) (*shared.OpenAIResponsesRequest, error) {
	request := &shared.OpenAIResponsesRequest{}
	err := httpapi.UnmarshalBodyReusable(c, request)
	if err != nil {
		return nil, err
	}
	if request.Model == "" {
		return nil, errors.New("model is required")
	}
	if request.Input == nil {
		return nil, errors.New("input is required")
	}
	if request.MaxOutputTokens > maxTokensUpperBound {
		return nil, errors.New("max_output_tokens is invalid")
	}
	return request, nil
}

func GetAndValidateResponsesCompactionRequest(c *gin.Context) (*shared.OpenAIResponsesCompactionRequest, error) {
	request := &shared.OpenAIResponsesCompactionRequest{}
	if err := httpapi.UnmarshalBodyReusable(c, request); err != nil {
		return nil, err
	}
	if request.Model == "" {
		return nil, errors.New("model is required")
	}
	return request, nil
}

func GetAndValidOpenAIImageRequest(c *gin.Context, relayMode int) (*shared.ImageRequest, error) {
	imageRequest := &shared.ImageRequest{}

	switch relayMode {
	case relayconstant.RelayModeImagesEdits:
		if strings.Contains(c.Request.Header.Get("Content-Type"), "multipart/form-data") {
			_, err := c.MultipartForm()
			if err != nil {
				return nil, fmt.Errorf("failed to parse image edit form request: %w", err)
			}
			formData := c.Request.PostForm
			imageRequest.Prompt = formData.Get("prompt")
			imageRequest.Model = formData.Get("model")
			imageRequest.N = uint(common.String2Int(formData.Get("n")))
			imageRequest.Quality = formData.Get("quality")
			imageRequest.Size = formData.Get("size")
			if imageValue := formData.Get("image"); imageValue != "" {
				imageRequest.Image, _ = jsonx.Marshal(imageValue)
			}

			if imageRequest.Model == "gpt-image-1" {
				if imageRequest.Quality == "" {
					imageRequest.Quality = "standard"
				}
			}
			if imageRequest.N == 0 {
				imageRequest.N = 1
			}

			hasWatermark := formData.Has("watermark")
			if hasWatermark {
				watermark := formData.Get("watermark") == "true"
				imageRequest.Watermark = &watermark
			}
			break
		}
		fallthrough
	default:
		err := httpapi.UnmarshalBodyReusable(c, imageRequest)
		if err != nil {
			return nil, err
		}

		if imageRequest.Model == "" {
			//imageRequest.Model = "dall-e-3"
			return nil, errors.New("model is required")
		}

		if strings.Contains(imageRequest.Size, "×") {
			return nil, errors.New("size an unexpected error occurred in the parameter, please use 'x' instead of the multiplication sign '×'")
		}

		// Not "256x256", "512x512", or "1024x1024"
		if imageRequest.Model == "dall-e-2" || imageRequest.Model == "dall-e" {
			if imageRequest.Size != "" && imageRequest.Size != "256x256" && imageRequest.Size != "512x512" && imageRequest.Size != "1024x1024" {
				return nil, errors.New("size must be one of 256x256, 512x512, or 1024x1024 for dall-e-2 or dall-e")
			}
			if imageRequest.Size == "" {
				imageRequest.Size = "1024x1024"
			}
		} else if imageRequest.Model == "dall-e-3" {
			if imageRequest.Size != "" && imageRequest.Size != "1024x1024" && imageRequest.Size != "1024x1792" && imageRequest.Size != "1792x1024" {
				return nil, errors.New("size must be one of 1024x1024, 1024x1792 or 1792x1024 for dall-e-3")
			}
			if imageRequest.Quality == "" {
				imageRequest.Quality = "standard"
			}
			if imageRequest.Size == "" {
				imageRequest.Size = "1024x1024"
			}
		} else if imageRequest.Model == "gpt-image-1" {
			if imageRequest.Quality == "" {
				imageRequest.Quality = "auto"
			}
		}

		//if imageRequest.Prompt == "" {
		//	return nil, errors.New("prompt is required")
		//}

		if imageRequest.N == 0 {
			imageRequest.N = 1
		}
	}

	if imageRequest.N > maxImageN {
		return nil, fmt.Errorf("n must be between 1 and %d", maxImageN)
	}

	return imageRequest, nil
}

func GetAndValidateClaudeRequest(c *gin.Context) (textRequest *shared.ClaudeRequest, err error) {
	textRequest = &shared.ClaudeRequest{}
	err = c.ShouldBindJSON(textRequest)
	if err != nil {
		return nil, err
	}
	if len(textRequest.Messages) == 0 {
		return nil, errors.New("field messages is required")
	}
	if textRequest.Model == "" {
		return nil, errors.New("field model is required")
	}
	if textRequest.MaxTokens > maxTokensUpperBound {
		return nil, errors.New("max_tokens is invalid")
	}
	if textRequest.MaxTokensToSample > maxTokensUpperBound {
		return nil, errors.New("max_tokens_to_sample is invalid")
	}

	//if textRequest.Stream {
	//	relayInfo.IsStream = true
	//}

	return textRequest, nil
}

func GetAndValidateTextRequest(c *gin.Context, relayMode int) (*shared.GeneralOpenAIRequest, error) {
	textRequest := &shared.GeneralOpenAIRequest{}
	err := httpapi.UnmarshalBodyReusable(c, textRequest)
	if err != nil {
		return nil, err
	}

	if relayMode == relayconstant.RelayModeModerations && textRequest.Model == "" {
		textRequest.Model = "text-moderation-latest"
	}
	if relayMode == relayconstant.RelayModeEmbeddings && textRequest.Model == "" {
		textRequest.Model = c.Param("model")
	}

	if textRequest.MaxTokens > maxTokensUpperBound {
		return nil, errors.New("max_tokens is invalid")
	}
	if textRequest.MaxCompletionTokens > maxTokensUpperBound {
		return nil, errors.New("max_completion_tokens is invalid")
	}
	if textRequest.Model == "" {
		return nil, errors.New("model is required")
	}
	if textRequest.WebSearchOptions != nil {
		if textRequest.WebSearchOptions.SearchContextSize != "" {
			validSizes := map[string]bool{
				"high":   true,
				"medium": true,
				"low":    true,
			}
			if !validSizes[textRequest.WebSearchOptions.SearchContextSize] {
				return nil, errors.New("invalid search_context_size, must be one of: high, medium, low")
			}
		} else {
			textRequest.WebSearchOptions.SearchContextSize = "medium"
		}
	}
	switch relayMode {
	case relayconstant.RelayModeCompletions:
		if textRequest.Prompt == "" {
			return nil, errors.New("field prompt is required")
		}
	case relayconstant.RelayModeChatCompletions:
		// For FIM (Fill-in-the-middle) requests with prefix/suffix, messages is optional
		// It will be filled by provider-specific adaptors if needed (e.g., SiliconFlow)。Or it is allowed by model vendor(s) (e.g., DeepSeek)
		if len(textRequest.Messages) == 0 && textRequest.Prefix == nil && textRequest.Suffix == nil {
			return nil, errors.New("field messages is required")
		}
	case relayconstant.RelayModeEmbeddings:
	case relayconstant.RelayModeModerations:
		if textRequest.Input == nil || textRequest.Input == "" {
			return nil, errors.New("field input is required")
		}
	case relayconstant.RelayModeEdits:
		if textRequest.Instruction == "" {
			return nil, errors.New("field instruction is required")
		}
	}
	return textRequest, nil
}

func GetAndValidateGeminiRequest(c *gin.Context) (*shared.GeminiChatRequest, error) {
	request := &shared.GeminiChatRequest{}
	err := httpapi.UnmarshalBodyReusable(c, request)
	if err != nil {
		return nil, err
	}
	if len(request.Contents) == 0 && len(request.Requests) == 0 {
		return nil, errors.New("contents is required")
	}
	if request.GenerationConfig.MaxOutputTokens > maxTokensUpperBound {
		return nil, errors.New("maxOutputTokens is invalid")
	}

	//if c.Query("alt") == "sse" {
	//	relayInfo.IsStream = true
	//}

	return request, nil
}

func GetAndValidateGeminiCountTokensRequest(c *gin.Context) (*shared.GeminiCountTokensRequest, error) {
	request := &shared.GeminiCountTokensRequest{}
	err := httpapi.UnmarshalBodyReusable(c, request)
	if err != nil {
		return nil, err
	}
	if len(request.Contents) == 0 && request.GenerateContentRequest == nil {
		return nil, errors.New("contents is required")
	}
	return request, nil
}

func GetAndValidateGeminiEmbeddingRequest(c *gin.Context) (*shared.GeminiEmbeddingRequest, error) {
	request := &shared.GeminiEmbeddingRequest{}
	err := httpapi.UnmarshalBodyReusable(c, request)
	if err != nil {
		return nil, err
	}
	return request, nil
}

func GetAndValidateGeminiBatchEmbeddingRequest(c *gin.Context) (*shared.GeminiBatchEmbeddingRequest, error) {
	request := &shared.GeminiBatchEmbeddingRequest{}
	err := httpapi.UnmarshalBodyReusable(c, request)
	if err != nil {
		return nil, err
	}
	return request, nil
}
