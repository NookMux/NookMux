package channelcontroller

import (
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/NookMux/NookMux/internal/common"
	"github.com/NookMux/NookMux/internal/domain/channel/constant"
	"github.com/NookMux/NookMux/internal/httpapi"
	"github.com/NookMux/NookMux/internal/i18n"
	httpclient "github.com/NookMux/NookMux/internal/infra/httpclient"
	"github.com/NookMux/NookMux/internal/store/channel"
	"github.com/NookMux/NookMux/pkg/jsonx"
	"github.com/gin-gonic/gin"
)

func localizedErrOrRaw(c *gin.Context, key string, err error) string {
	msg := i18n.T(c, key, map[string]any{"Error": err.Error()})
	if msg == key {
		return err.Error()
	}
	return msg
}

// OpenRouterProviderEntry is the subset of an https://openrouter.ai/api/v1/providers
// record that matters for provider picking in the channel form.
type OpenRouterProviderEntry struct {
	Slug             string `json:"slug"`
	Name             string `json:"name"`
	Headquarters     string `json:"headquarters,omitempty"`
	PrivacyPolicyURL string `json:"privacy_policy_url,omitempty"`
}

type openRouterProvidersResult struct {
	Data []OpenRouterProviderEntry `json:"data"`
}

// FetchUpstreamProviders fetches the OpenRouter provider list for an existing
// channel, routing the upstream call through the channel's configured proxy.
// Read-only admin endpoint, no audit required.
func FetchUpstreamProviders(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		httpapi.ApiErrorI18n(c, i18n.MsgChannelIDFormatError, map[string]any{"Error": err.Error()})
		return
	}

	channelRow, err := channelstore.GetChannelById(id, false)
	if err != nil {
		common.SysError("failed to get channel by id: " + err.Error())
		httpapi.ApiErrorI18n(c, i18n.MsgDatabaseError)
		return
	}

	if channelRow.Type != constant.ChannelTypeOpenRouter {
		httpapi.ApiErrorI18n(c, i18n.MsgChannelProvidersOpenRouterOnly)
		return
	}

	baseURL := channelRow.GetBaseURL()
	if baseURL == "" {
		baseURL = constant.ChannelBaseURLs[constant.ChannelTypeOpenRouter]
	}
	url := fmt.Sprintf("%s/v1/providers", strings.TrimRight(baseURL, "/"))

	// The providers endpoint is public; no key required. GetResponseBody
	// routes the request through the channel proxy.
	body, err := GetResponseBody("GET", url, channelRow, http.Header{})
	if err != nil {
		common.SysError("failed to fetch openrouter providers: " + err.Error())
		httpapi.ApiErrorI18n(c, i18n.MsgChannelFetchProvidersFailed, map[string]any{"Error": err.Error()})
		return
	}

	respondOpenRouterProviders(c, body)
}

// FetchProviders handles create-mode provider fetching before a channel row
// exists. Mirrors the unproxied FetchModels POST endpoint: SSRF validation on
// the resolved URL, controlled client without channel proxy.
func FetchProviders(c *gin.Context) {
	var req struct {
		BaseURL string `json:"base_url"`
		Type    int    `json:"type"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		httpapi.ApiErrorI18n(c, i18n.MsgChannelInvalidRequest)
		return
	}

	if req.Type != constant.ChannelTypeOpenRouter {
		httpapi.ApiErrorI18n(c, i18n.MsgChannelProvidersOpenRouterOnly)
		return
	}

	baseURL := strings.TrimSpace(req.BaseURL)
	if baseURL == "" {
		baseURL = constant.ChannelBaseURLs[constant.ChannelTypeOpenRouter]
	}
	url := fmt.Sprintf("%s/v1/providers", strings.TrimRight(baseURL, "/"))

	if err := validateFetchModelsURL(url); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": i18n.ResolveContextError(c, err, i18n.MsgChannelInvalidRequestParameters),
		})
		return
	}

	request, err := http.NewRequest("GET", url, nil)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": localizedErrOrRaw(c, i18n.MsgChannelRequestCreateFailed, err),
		})
		return
	}

	headers := http.Header{}
	applyFetchModelsDefaultHeaders(headers)
	request.Header = headers

	response, err := httpclient.GetHttpClient().Do(request)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": localizedErrOrRaw(c, i18n.MsgChannelUpstreamRequestFailed, err),
		})
		return
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		common.SysError(fmt.Sprintf("failed to fetch openrouter providers: status code %d", response.StatusCode))
		httpapi.ApiErrorI18n(c, i18n.MsgChannelFetchProvidersFailed, map[string]any{
			"Error": fmt.Sprintf("status code: %d", response.StatusCode),
		})
		return
	}

	body, err := io.ReadAll(response.Body)
	if err != nil {
		httpapi.ApiErrorI18n(c, i18n.MsgChannelFetchProvidersFailed, map[string]any{"Error": err.Error()})
		return
	}

	respondOpenRouterProviders(c, body)
}

func respondOpenRouterProviders(c *gin.Context, body []byte) {
	var result openRouterProvidersResult
	if err := jsonx.Unmarshal(body, &result); err != nil {
		httpapi.ApiErrorI18n(c, i18n.MsgChannelParseResponseFailed, map[string]any{"Error": err.Error()})
		return
	}

	providers := make([]OpenRouterProviderEntry, 0, len(result.Data))
	for _, provider := range result.Data {
		slug := strings.TrimSpace(provider.Slug)
		if slug == "" {
			continue
		}
		provider.Slug = slug
		provider.Name = strings.TrimSpace(provider.Name)
		providers = append(providers, provider)
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    providers,
	})
}
