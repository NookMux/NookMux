package handler

import (
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"github.com/NookMux/NookMux/internal/common"
	"github.com/NookMux/NookMux/internal/config/ratio"
	billing "github.com/NookMux/NookMux/internal/domain/billing"
	"github.com/NookMux/NookMux/internal/domain/shared"
	"github.com/NookMux/NookMux/internal/httpapi"
	"github.com/NookMux/NookMux/internal/infra/cache"
	"github.com/NookMux/NookMux/internal/infra/log"
	media "github.com/NookMux/NookMux/internal/infra/media"
	relaycommon "github.com/NookMux/NookMux/internal/relay/common"
	"github.com/NookMux/NookMux/internal/relay/core"
	"github.com/NookMux/NookMux/internal/relay/helper"
	"github.com/NookMux/NookMux/internal/store/stored_media"
	"github.com/NookMux/NookMux/internal/store/user"
	"github.com/NookMux/NookMux/pkg/jsonx"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	"io"
	"net/http"
	"strings"
)

func TextHelper(c *gin.Context, info *relaycommon.RelayInfo) (newAPIError *shared.NookMuxError) {
	info.InitChannelMeta(c)

	textReq, ok := info.Request.(*shared.GeneralOpenAIRequest)
	if !ok {
		return shared.NewErrorWithStatusCode(fmt.Errorf("invalid request type, expected shared.GeneralOpenAIRequest, got %T", info.Request), shared.ErrorCodeInvalidRequest, http.StatusBadRequest, shared.ErrOptionWithSkipRetry())
	}

	request, err := common.DeepCopy(textReq)
	if err != nil {
		return shared.NewError(fmt.Errorf("failed to copy request to GeneralOpenAIRequest: %w", err), shared.ErrorCodeInvalidRequest, shared.ErrOptionWithSkipRetry())
	}

	if request.WebSearchOptions != nil {
		c.Set("chat_completion_web_search_context_size", request.WebSearchOptions.SearchContextSize)
	}

	err = helper.ModelMappedHelper(c, info, request)
	if err != nil {
		return shared.NewError(err, shared.ErrorCodeChannelModelMappedError, shared.ErrOptionWithSkipRetry())
	}

	mediaMode, modeOK := info.ChannelOtherSettings.ParseImageAutoConvertToURLMode()
	if !modeOK {
		return shared.NewErrorWithStatusCode(fmt.Errorf("invalid image_auto_convert_to_url_mode: %q", info.ChannelOtherSettings.ImageAutoConvertToURLMode), shared.ErrorCodeInvalidRequest, http.StatusBadRequest, shared.ErrOptionWithSkipRetry())
	}

	// Channel-level multimodal handling for text-only upstream models.
	if mediaMode != shared.ImageAutoConvertToURLModeOff {
		storedURLBySHA := make(map[string]string)
		imageMaxBytes := int64(shared.MaxImageUploadMB) * 1024 * 1024
		videoMaxBytes := int64(shared.MaxVideoUploadMB) * 1024 * 1024
		imagePoolMaxBytes := int64(shared.StoredImagePoolMB) * 1024 * 1024
		videoPoolMaxBytes := int64(shared.StoredVideoPoolMB) * 1024 * 1024
		// 跟踪本次请求中新增存储的图片/视频数量（去重命中不计入）
		newImageCount := 0
		newVideoCount := 0

		resolveURL := func(rawURL string, mediaContentType string) (string, error) {
			rawURL = strings.TrimSpace(rawURL)
			if rawURL == "" {
				return "", nil
			}
			if strings.HasPrefix(rawURL, "http://") || strings.HasPrefix(rawURL, "https://") {
				return rawURL, nil
			}

			mimeType, b64, err := media.DecodeBase64FileData(rawURL)
			if err != nil {
				return "", shared.NewErrorWithStatusCode(fmt.Errorf("decode media data failed: %w", err), shared.ErrorCodeInvalidRequest, http.StatusBadRequest, shared.ErrOptionWithSkipRetry())
			}
			mimeType = strings.TrimSpace(mimeType)
			if mimeType == "" {
				return "", shared.NewErrorWithStatusCode(fmt.Errorf("invalid media mime type: %q", mimeType), shared.ErrorCodeInvalidRequest, http.StatusBadRequest, shared.ErrOptionWithSkipRetry())
			}
			lowerMime := strings.ToLower(mimeType)
			isImage := mediaContentType == shared.ContentTypeImageURL
			isVideo := mediaContentType == shared.ContentTypeVideoUrl
			if isImage && !strings.HasPrefix(lowerMime, "image/") {
				return "", shared.NewErrorWithStatusCode(fmt.Errorf("invalid image mime type: %q", mimeType), shared.ErrorCodeInvalidRequest, http.StatusBadRequest, shared.ErrOptionWithSkipRetry())
			}
			if isVideo && !strings.HasPrefix(lowerMime, "video/") {
				return "", shared.NewErrorWithStatusCode(fmt.Errorf("invalid video mime type: %q", mimeType), shared.ErrorCodeInvalidRequest, http.StatusBadRequest, shared.ErrOptionWithSkipRetry())
			}
			if !isImage && !isVideo {
				return "", shared.NewErrorWithStatusCode(fmt.Errorf("unsupported media content type: %q", mediaContentType), shared.ErrorCodeInvalidRequest, http.StatusBadRequest, shared.ErrOptionWithSkipRetry())
			}

			b64 = strings.TrimSpace(b64)
			data, err := base64.StdEncoding.DecodeString(b64)
			if err != nil {
				return "", shared.NewErrorWithStatusCode(fmt.Errorf("decode media base64 failed: %w", err), shared.ErrorCodeInvalidRequest, http.StatusBadRequest, shared.ErrOptionWithSkipRetry())
			}
			if len(data) == 0 {
				return "", shared.NewErrorWithStatusCode(fmt.Errorf("media data is empty"), shared.ErrorCodeInvalidRequest, http.StatusBadRequest, shared.ErrOptionWithSkipRetry())
			}

			if isImage && imageMaxBytes > 0 && int64(len(data)) > imageMaxBytes {
				return "", shared.NewErrorWithStatusCode(fmt.Errorf("image size %d exceeds limit %d bytes", len(data), imageMaxBytes), shared.ErrorCodeInvalidRequest, http.StatusBadRequest, shared.ErrOptionWithSkipRetry())
			}
			if isVideo && videoMaxBytes > 0 && int64(len(data)) > videoMaxBytes {
				return "", shared.NewErrorWithStatusCode(fmt.Errorf("video size %d exceeds limit %d bytes", len(data), videoMaxBytes), shared.ErrorCodeInvalidRequest, http.StatusBadRequest, shared.ErrOptionWithSkipRetry())
			}

			sha := hex.EncodeToString(common.Sha256Raw(data))
			cacheKey := mediaContentType + ":" + sha
			if existing, ok := storedURLBySHA[cacheKey]; ok {
				return existing, nil
			}

			if isImage {
				// Cross-request dedupe: same user + same sha -> reuse existing asset URL.
				if existing, err := storedmediastore.GetStoredImageByUserAndSha(c.Request.Context(), info.UserId, sha); err == nil && existing != nil && existing.Id != "" {
					u := core.BuildStoredImageURL(c, existing.Id)
					storedURLBySHA[cacheKey] = u
					return u, nil
				} else if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
					return "", shared.NewError(fmt.Errorf("query stored image failed: %w", err), shared.ErrorCodeQueryDataError, shared.ErrOptionWithSkipRetry())
				}

				img := &storedmediastore.StoredImage{
					UserId:    info.UserId,
					ChannelId: info.ChannelId,
					MimeType:  mimeType,
					SizeBytes: len(data),
					Sha256:    sha,
					Data:      storedmediastore.LargeBlob(data),
				}
				if err := img.Insert(c.Request.Context()); err != nil {
					return "", shared.NewError(fmt.Errorf("store image failed: %w", err), shared.ErrorCodeUpdateDataError, shared.ErrOptionWithSkipRetry())
				}
				if _, err := storedmediastore.EnsureStoredImagesPoolLimit(c.Request.Context(), imagePoolMaxBytes, 100); err != nil {
					return "", shared.NewError(fmt.Errorf("enforce stored image pool limit failed: %w", err), shared.ErrorCodeUpdateDataError, shared.ErrOptionWithSkipRetry())
				}

				newImageCount++
				u := core.BuildStoredImageURL(c, img.Id)
				storedURLBySHA[cacheKey] = u
				return u, nil
			}

			if existing, err := storedmediastore.GetStoredVideoByUserAndSha(c.Request.Context(), info.UserId, sha); err == nil && existing != nil && existing.Id != "" {
				u := core.BuildStoredVideoURL(c, existing.Id)
				storedURLBySHA[cacheKey] = u
				return u, nil
			} else if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
				return "", shared.NewError(fmt.Errorf("query stored video failed: %w", err), shared.ErrorCodeQueryDataError, shared.ErrOptionWithSkipRetry())
			}

			v := &storedmediastore.StoredVideo{
				UserId:    info.UserId,
				ChannelId: info.ChannelId,
				MimeType:  mimeType,
				SizeBytes: len(data),
				Sha256:    sha,
				Data:      storedmediastore.LargeBlob(data),
			}
			if err := v.Insert(c.Request.Context()); err != nil {
				return "", shared.NewError(fmt.Errorf("store video failed: %w", err), shared.ErrorCodeUpdateDataError, shared.ErrOptionWithSkipRetry())
			}
			if _, err := storedmediastore.EnsureStoredVideosPoolLimit(c.Request.Context(), videoPoolMaxBytes, 50); err != nil {
				return "", shared.NewError(fmt.Errorf("enforce stored video pool limit failed: %w", err), shared.ErrorCodeUpdateDataError, shared.ErrOptionWithSkipRetry())
			}

			newVideoCount++
			u := core.BuildStoredVideoURL(c, v.Id)
			storedURLBySHA[cacheKey] = u
			return u, nil
		}

		if mediaMode != shared.ImageAutoConvertToURLModeMCP {
			return shared.NewErrorWithStatusCode(fmt.Errorf("unsupported image_auto_convert_to_url_mode: %s", mediaMode), shared.ErrorCodeInvalidRequest, http.StatusBadRequest, shared.ErrOptionWithSkipRetry())
		}

		_, convErr := relaycommon.ApplyImageAutoConvertToURL(request, resolveURL)
		if convErr != nil {
			return shared.NewError(convErr, shared.ErrorCodeInvalidRequest, shared.ErrOptionWithSkipRetry())
		}

		// 异步更新用户的多模态适配转换计数
		if newImageCount > 0 || newVideoCount > 0 {
			go userstore.IncrementMediaConvertedCount(info.UserId, newImageCount, newVideoCount)
		}
	}

	includeUsage := true
	// 判断用户是否需要返回使用情况
	if request.StreamOptions != nil {
		includeUsage = request.StreamOptions.IncludeUsage
	}

	// 如果不支持StreamOptions，将StreamOptions设置为nil
	if !info.SupportStreamOptions || !request.Stream {
		request.StreamOptions = nil
	} else {
		// 如果支持StreamOptions，且请求中没有设置StreamOptions，根据配置文件设置StreamOptions
		if shared.ForceStreamOption {
			request.StreamOptions = &shared.StreamOptions{
				IncludeUsage: true,
			}
		}
	}

	info.ShouldIncludeUsage = includeUsage

	adaptor := core.GetAdaptor(info.ApiType)
	if adaptor == nil {
		return shared.NewError(fmt.Errorf("invalid api type: %d", info.ApiType), shared.ErrorCodeInvalidApiType, shared.ErrOptionWithSkipRetry())
	}
	adaptor.Init(info)

	passThroughBody := info.ChannelSetting.PassThroughBodyEnabled
	// Media handling rewrites the structured request; pass-through body would bypass it.
	if mediaMode != shared.ImageAutoConvertToURLModeOff {
		passThroughBody = false
	}

	var requestBody io.Reader

	if passThroughBody {
		storage, err := httpapi.GetBodyStorage(c)
		if err != nil {
			return shared.NewErrorWithStatusCode(err, shared.ErrorCodeReadRequestBodyFailed, http.StatusBadRequest, shared.ErrOptionWithSkipRetry())
		}
		if common.DebugEnabled {
			body, _ := storage.Bytes()
			println("requestBody: ", string(body))
			_, _ = storage.Seek(0, io.SeekStart)
		}
		info.UpstreamRequestBodySize = storage.Size()
		requestBody = cache.ReaderOnly(storage)
	} else {
		convertedRequest, err := adaptor.ConvertOpenAIRequest(c, info, request)
		if err != nil {
			return shared.NewError(err, shared.ErrorCodeConvertRequestFailed, shared.ErrOptionWithSkipRetry())
		}
		relaycommon.AppendRequestConversionFromRequest(info, convertedRequest)

		jsonData, err := jsonx.Marshal(convertedRequest)
		if err != nil {
			return shared.NewError(err, shared.ErrorCodeJsonMarshalFailed, shared.ErrOptionWithSkipRetry())
		}

		// remove disabled fields for OpenAI API
		jsonData, err = relaycommon.RemoveDisabledFields(jsonData, info.ChannelOtherSettings)
		if err != nil {
			return shared.NewError(err, shared.ErrorCodeConvertRequestFailed, shared.ErrOptionWithSkipRetry())
		}

		// apply OpenRouter provider routing preferences (channel overrides client)
		jsonData, err = relaycommon.ApplyProviderRouting(jsonData, info)
		if err != nil {
			return shared.NewError(err, shared.ErrorCodeConvertRequestFailed, shared.ErrOptionWithSkipRetry())
		}

		// apply param override
		if len(info.ParamOverride) > 0 {
			jsonData, err = relaycommon.ApplyParamOverrideWithRelayInfo(jsonData, info)
			if err != nil {
				return shared.NewError(err, shared.ErrorCodeChannelParamOverrideInvalid, shared.ErrOptionWithSkipRetry())
			}
		}

		log.LogDebug(c, fmt.Sprintf("text request body: %s", string(jsonData)))

		body, size, closer, err := relaycommon.NewOutboundJSONBody(jsonData)
		if err != nil {
			return shared.NewError(err, shared.ErrorCodeConvertRequestFailed, shared.ErrOptionWithSkipRetry())
		}
		defer closer.Close()
		info.UpstreamRequestBodySize = size
		requestBody = body
	}

	// 通用思维强度解析：adaptor 未设置时从原始请求兜底提取
	relaycommon.EnsureReasoningEffort(info, info.Request)

	var httpResp *http.Response
	resp, err := adaptor.DoRequest(c, info, requestBody)
	if err != nil {
		return shared.NewOpenAIError(err, shared.ErrorCodeDoRequestFailed, http.StatusInternalServerError)
	}

	statusCodeMappingStr := c.GetString("status_code_mapping")

	if resp != nil {
		httpResp = resp.(*http.Response)
		info.IsStream = info.IsStream || strings.HasPrefix(httpResp.Header.Get("Content-Type"), "text/event-stream")
		if httpResp.StatusCode != http.StatusOK {
			newApiErr := helper.RelayErrorHandler(c.Request.Context(), httpResp, false)
			// reset status code 重置状态码
			helper.ResetStatusCode(newApiErr, statusCodeMappingStr)
			return newApiErr
		}
	}

	usage, newApiErr := adaptor.DoResponse(c, httpResp, info)
	if newApiErr != nil {
		// reset status code 重置状态码
		helper.ResetStatusCode(newApiErr, statusCodeMappingStr)
		return newApiErr
	}

	var containAudioTokens = usage.(*shared.Usage).CompletionTokenDetails.AudioTokens > 0 || usage.(*shared.Usage).PromptTokensDetails.AudioTokens > 0
	var containsAudioRatios = ratio.ContainsAudioRatio(info.OriginModelName) || ratio.ContainsAudioCompletionRatio(info.OriginModelName)

	if containAudioTokens && containsAudioRatios {
		if apiErr := billing.PostAudioConsumeQuota(c, info, usage.(*shared.Usage), ""); apiErr != nil {
			return apiErr
		}
	} else {
		settlement, apiErr := billing.CalculateUsage(c, info, usage.(*shared.Usage))
		if apiErr != nil {
			return apiErr
		}
		if apiErr := billing.ApplyQuota(c, info, settlement); apiErr != nil {
			return apiErr
		}
	}
	return nil
}
