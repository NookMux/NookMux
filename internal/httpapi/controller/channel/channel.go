package channelcontroller

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/NookMux/NookMux/internal/common"
	"github.com/NookMux/NookMux/internal/config/system"
	audit "github.com/NookMux/NookMux/internal/domain/audit"
	planquota "github.com/NookMux/NookMux/internal/domain/billing/plan_quota"
	"github.com/NookMux/NookMux/internal/domain/channel/constant"
	"github.com/NookMux/NookMux/internal/domain/shared"
	"github.com/NookMux/NookMux/internal/httpapi"
	"github.com/NookMux/NookMux/internal/i18n"
	httpclient "github.com/NookMux/NookMux/internal/infra/httpclient"
	"github.com/NookMux/NookMux/internal/infra/security"
	awschannel "github.com/NookMux/NookMux/internal/relay/channel/aws"
	"github.com/NookMux/NookMux/internal/relay/channel/gemini"
	"github.com/NookMux/NookMux/internal/relay/channel/ollama"
	"github.com/NookMux/NookMux/internal/store/audit"
	"github.com/NookMux/NookMux/internal/store/channel"
	"github.com/NookMux/NookMux/internal/store/db"
	"github.com/NookMux/NookMux/internal/store/log"
	"github.com/NookMux/NookMux/internal/store/user"
	"github.com/NookMux/NookMux/pkg/jsonx"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	"net/http"
	"strconv"
	"strings"
	"time"
)

type OpenAIModel struct {
	ID         string         `json:"id"`
	Object     string         `json:"object"`
	Created    int64          `json:"created"`
	OwnedBy    string         `json:"owned_by"`
	Metadata   map[string]any `json:"metadata,omitempty"`
	Permission []struct {
		ID                 string `json:"id"`
		Object             string `json:"object"`
		Created            int64  `json:"created"`
		AllowCreateEngine  bool   `json:"allow_create_engine"`
		AllowSampling      bool   `json:"allow_sampling"`
		AllowLogprobs      bool   `json:"allow_logprobs"`
		AllowSearchIndices bool   `json:"allow_search_indices"`
		AllowView          bool   `json:"allow_view"`
		AllowFineTuning    bool   `json:"allow_fine_tuning"`
		Organization       string `json:"organization"`
		Group              string `json:"group"`
		IsBlocking         bool   `json:"is_blocking"`
	} `json:"permission"`
	Root   string `json:"root"`
	Parent string `json:"parent"`
}

type OpenAIModelsResponse struct {
	Data    []OpenAIModel `json:"data"`
	Success bool          `json:"success"`
}

func parseStatusFilter(statusParam string) int {
	switch strings.ToLower(statusParam) {
	case "enabled", "1":
		return common.ChannelStatusEnabled
	case "disabled", "0":
		return 0
	default:
		return -1
	}
}

// normalizeModelID coerces a model id from upstream /v1/models responses into
// a trimmed string. Some providers return non-string ids (numbers, booleans,
// objects) which break downstream UI code that assumes strings. Empty results
// are returned for nil and objects without a useful string form.
func normalizeModelID(raw any) string {
	switch v := raw.(type) {
	case string:
		return strings.TrimSpace(v)
	case float64:
		return strings.TrimSpace(strconv.FormatFloat(v, 'f', -1, 64))
	case int:
		return strings.TrimSpace(strconv.Itoa(v))
	case int64:
		return strings.TrimSpace(strconv.FormatInt(v, 10))
	case json.Number:
		return strings.TrimSpace(v.String())
	case bool:
		if v {
			return "true"
		}
		return "false"
	case nil:
		return ""
	default:
		if bytes, err := jsonx.Marshal(raw); err == nil {
			return strings.Trim(string(bytes), `"`)
		}
		return ""
	}
}

func clearChannelInfo(channel *channelstore.Channel) {
	if channel.ChannelInfo.IsMultiKey {
		channel.ChannelInfo.MultiKeyDisabledReason = nil
		channel.ChannelInfo.MultiKeyDisabledTime = nil
	}
	// 套餐信息保留返回给前端（IsPlan 和 PlanName 不包含敏感信息）
}

func applyChannelStatusFilter(query *gorm.DB, statusFilter int) *gorm.DB {
	if statusFilter == common.ChannelStatusEnabled {
		return query.Where("status = ?", common.ChannelStatusEnabled)
	}
	if statusFilter == 0 {
		return query.Where("status != ?", common.ChannelStatusEnabled)
	}
	return query
}

func buildChannelListQuery(group string, statusFilter int, typeFilter int) *gorm.DB {
	query := dbstore.DB.Model(&channelstore.Channel{})
	query = channelstore.ApplyChannelGroupFilter(query, group)
	query = applyChannelStatusFilter(query, statusFilter)
	if typeFilter >= 0 {
		query = query.Where("type = ?", typeFilter)
	}
	return query
}

func GetAllChannels(c *gin.Context) {
	pageInfo := common.GetPageQuery(c)
	channelData := make([]*channelstore.Channel, 0)
	idSort, _ := strconv.ParseBool(c.Query("id_sort"))
	enableTagMode, _ := strconv.ParseBool(c.Query("tag_mode"))
	groupFilter := channelstore.NormalizeChannelGroupFilter(c.Query("group"))
	statusParam := c.Query("status")
	// statusFilter: -1 all, 1 enabled, 0 disabled (include auto & manual)
	statusFilter := parseStatusFilter(statusParam)
	// type filter
	typeStr := c.Query("type")
	typeFilter := -1
	if typeStr != "" {
		if t, err := strconv.Atoi(typeStr); err == nil {
			typeFilter = t
		}
	}

	var total int64

	if enableTagMode {
		tags, err := channelstore.GetPaginatedChannelTags(buildChannelListQuery(groupFilter, statusFilter, typeFilter), pageInfo.GetStartIdx(), pageInfo.GetPageSize())
		if err != nil {
			common.SysError("failed to get paginated tags: " + err.Error())
			httpapi.ApiErrorI18n(c, i18n.MsgChannelGetTagsFailed)
			return
		}
		total, err = channelstore.CountChannelTags(buildChannelListQuery(groupFilter, statusFilter, typeFilter))
		if err != nil {
			common.SysError("failed to count tags: " + err.Error())
			httpapi.ApiErrorI18n(c, i18n.MsgChannelCountTagsFailed)
			return
		}
		for _, tag := range tags {
			if tag == nil || *tag == "" {
				continue
			}
			var tagChannels []*channelstore.Channel
			err := buildChannelListQuery(groupFilter, statusFilter, typeFilter).
				Where("tag = ?", *tag).
				Order("priority desc, weight desc").
				Omit("key").
				Find(&tagChannels).Error
			if err != nil {
				common.SysError("failed to get channels by tag: " + err.Error())
				httpapi.ApiErrorI18n(c, i18n.MsgChannelGetTagChannelsFailed)
				return
			}
			channelData = append(channelData, tagChannels...)
		}
	} else {
		if err := buildChannelListQuery(groupFilter, statusFilter, typeFilter).Count(&total).Error; err != nil {
			common.SysError("failed to count channels: " + err.Error())
			httpapi.ApiErrorI18n(c, i18n.MsgChannelCountFailed)
			return
		}

		order := "LOWER(name) asc, id asc"
		if idSort {
			order = "id desc"
		}

		err := buildChannelListQuery(groupFilter, statusFilter, typeFilter).
			Order(order).
			Limit(pageInfo.GetPageSize()).
			Offset(pageInfo.GetStartIdx()).
			Omit("key").
			Find(&channelData).Error
		if err != nil {
			common.SysError("failed to get channels: " + err.Error())
			httpapi.ApiErrorI18n(c, i18n.MsgChannelListFailed)
			return
		}
	}

	for _, datum := range channelData {
		clearChannelInfo(datum)
	}

	countQuery := buildChannelListQuery(groupFilter, statusFilter, -1)
	var results []struct {
		Type  int64
		Count int64
	}
	if err := countQuery.Select("type, count(*) as count").Group("type").Find(&results).Error; err != nil {
		common.SysError("failed to count channel types: " + err.Error())
		httpapi.ApiErrorI18n(c, i18n.MsgChannelTypeStatsFailed)
		return
	}
	typeCounts := make(map[int64]int64)
	for _, r := range results {
		typeCounts[r.Type] = r.Count
	}
	httpapi.ApiSuccess(c, gin.H{
		"items":       channelData,
		"total":       total,
		"page":        pageInfo.GetPage(),
		"page_size":   pageInfo.GetPageSize(),
		"type_counts": typeCounts,
	})
}

const (
	fetchModelsDefaultXTitle      = "Cherry Studio"
	fetchModelsDefaultHTTPReferer = "https://cherry-ai.com"
	fetchModelsDefaultUserAgent   = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) CherryStudio/1.7.18 Chrome/140.0.7339.249 Electron/38.7.0 Safari/537.36"
)

func buildFetchModelsHeaders(channel *channelstore.Channel, key string) (http.Header, error) {
	var headers http.Header
	switch channel.Type {
	case constant.ChannelTypeAnthropic:
		headers = GetClaudeAuthHeader(key)
	default:
		headers = GetAuthHeader(key)
	}

	applyFetchModelsDefaultHeaders(headers)

	if err := applyFetchModelsHeaderOverride(headers, channel, key); err != nil {
		return nil, err
	}

	return headers, nil
}

func buildFetchModelsGeminiHeaders(channel *channelstore.Channel, key string) (http.Header, error) {
	headers := http.Header{}
	applyFetchModelsDefaultHeaders(headers)
	if err := applyFetchModelsHeaderOverride(headers, channel, key); err != nil {
		return nil, err
	}
	if strings.TrimSpace(headers.Get("x-goog-api-key")) == "" && strings.TrimSpace(headers.Get("Authorization")) == "" {
		headers.Set("x-goog-api-key", key)
	}
	return headers, nil
}

func applyFetchModelsDefaultHeaders(headers http.Header) {
	if headers == nil {
		return
	}
	if strings.TrimSpace(headers.Get("x-title")) == "" {
		headers.Set("x-title", fetchModelsDefaultXTitle)
	}
	if strings.TrimSpace(headers.Get("http-referer")) == "" {
		headers.Set("http-referer", fetchModelsDefaultHTTPReferer)
	}
	if strings.TrimSpace(headers.Get("user-agent")) == "" {
		headers.Set("user-agent", fetchModelsDefaultUserAgent)
	}
}

func applyFetchModelsHeaderOverride(headers http.Header, channel *channelstore.Channel, key string) error {
	headerOverride := channel.GetHeaderOverride()
	for rawKey, rawValue := range headerOverride {
		trimmedKey := strings.TrimSpace(rawKey)
		if trimmedKey == "" {
			continue
		}
		if isFetchModelsHeaderPassthroughRuleKey(trimmedKey) {
			continue
		}
		if shouldSkipFetchModelsHeaderOverride(trimmedKey) {
			continue
		}

		str, ok := rawValue.(string)
		if !ok {
			return fmt.Errorf("invalid header override for key %s", rawKey)
		}

		trimmedValue := strings.TrimSpace(str)
		if trimmedValue == "" {
			continue
		}

		// {client_header:XXX} placeholders require the original client request header.
		// Fetching models is an admin action and should not forward admin headers upstream.
		// Skip client_header placeholders instead of passing through the literal value.
		if strings.HasPrefix(trimmedValue, "{client_header:") {
			continue
		}

		if strings.Contains(str, "{api_key}") {
			str = strings.ReplaceAll(str, "{api_key}", key)
		}
		str = strings.TrimSpace(str)
		if str == "" {
			continue
		}

		headers.Set(trimmedKey, str)
	}
	return nil
}

func isFetchModelsHeaderPassthroughRuleKey(key string) bool {
	key = strings.TrimSpace(key)
	if key == "" {
		return false
	}
	if key == "*" {
		return true
	}
	lower := strings.ToLower(key)
	return strings.HasPrefix(lower, "re:") || strings.HasPrefix(lower, "regex:")
}

func shouldSkipFetchModelsHeaderOverride(key string) bool {
	switch strings.ToLower(strings.TrimSpace(key)) {
	case "",
		"host",
		"content-length",
		"accept-encoding",
		strings.ToLower(common.RequestIdKey),
		"connection",
		"keep-alive",
		"proxy-authenticate",
		"proxy-authorization",
		"te",
		"trailer",
		"transfer-encoding",
		"upgrade":
		return true
	default:
		return false
	}
}

func FetchUpstreamModels(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		httpapi.ApiErrorI18n(c, i18n.MsgChannelIDFormatError, map[string]any{"Error": err.Error()})
		return
	}

	channel, err := channelstore.GetChannelById(id, true)
	if err != nil {
		common.SysError("failed to get channel by id: " + err.Error())
		httpapi.ApiErrorI18n(c, i18n.MsgDatabaseError)
		return
	}

	baseURL := constant.ChannelBaseURLs[channel.Type]
	if channel.GetBaseURL() != "" {
		baseURL = channel.GetBaseURL()
	}

	// 对于 Ollama 渠道，使用特殊处理
	if channel.Type == constant.ChannelTypeOllama {
		key := strings.Split(channel.Key, "\n")[0]
		models, err := ollama.FetchOllamaModels(baseURL, key, channel.GetSetting().Proxy)
		if err != nil {
			c.JSON(http.StatusOK, gin.H{
				"success": false,
				"message": i18n.T(c, i18n.MsgChannelOllamaGetModelsFailed, map[string]any{"Error": err.Error()}),
			})
			return
		}

		ids := make([]string, 0, len(models))
		for _, modelInfo := range models {
			if modelInfo.Name == "" {
				continue
			}
			ids = append(ids, modelInfo.Name)
		}

		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"message": "",
			"data":    ids,
		})
		return
	}

	if channel.Type == constant.ChannelTypeAws {
		key, _, apiErr := channel.GetNextEnabledKey()
		if apiErr != nil {
			c.JSON(http.StatusOK, gin.H{"success": false, "message": apiErr.Error()})
			return
		}
		models, err := awschannel.FetchClaudeModels(c.Request.Context(), key, channel.GetOtherSettings().AwsKeyType, channel.GetSetting().Proxy)
		if err != nil {
			c.JSON(http.StatusOK, gin.H{"success": false, "message": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"success": true, "message": "", "data": models})
		return
	}

	// 对于 Gemini 渠道，使用特殊处理
	if channel.Type == constant.ChannelTypeGemini {
		// 获取用于请求的可用密钥（多密钥渠道优先使用启用状态的密钥）
		key, _, apiErr := channel.GetNextEnabledKey()
		if apiErr != nil {
			c.JSON(http.StatusOK, gin.H{
				"success": false,
				"message": i18n.T(c, i18n.MsgChannelGetKeyFailed, map[string]any{"Error": apiErr.Error()}),
			})
			return
		}
		key = strings.TrimSpace(key)
		headers, err := buildFetchModelsGeminiHeaders(channel, key)
		if err != nil {
			common.SysError("failed to build fetch models gemini headers: " + err.Error())
			httpapi.ApiErrorI18n(c, i18n.MsgInvalidParams)
			return
		}
		models, err := gemini.FetchGeminiModelsWithHeaders(baseURL, key, channel.GetSetting().Proxy, headers)
		if err != nil {
			c.JSON(http.StatusOK, gin.H{
				"success": false,
				"message": i18n.T(c, i18n.MsgChannelGeminiGetModelsFailed, map[string]any{"Error": err.Error()}),
			})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"message": "",
			"data":    models,
		})
		return
	}

	var url string
	switch channel.Type {
	case constant.ChannelTypeZhipu_v4:
		if plan, ok := constant.ChannelSpecialBases[baseURL]; ok && plan.OpenAIBaseURL != "" {
			url = fmt.Sprintf("%s/models", plan.OpenAIBaseURL)
		} else {
			url = fmt.Sprintf("%s/api/paas/v4/models", baseURL)
		}
	case constant.ChannelTypeMoonshot:
		if plan, ok := constant.ChannelSpecialBases[baseURL]; ok && plan.OpenAIBaseURL != "" {
			url = fmt.Sprintf("%s/models", plan.OpenAIBaseURL)
		} else {
			url = fmt.Sprintf("%s/v1/models", baseURL)
		}
	default:
		url = fmt.Sprintf("%s/v1/models", baseURL)
	}

	// 获取用于请求的可用密钥（多密钥渠道优先使用启用状态的密钥）
	key, _, apiErr := channel.GetNextEnabledKey()
	if apiErr != nil {
		httpapi.ApiErrorI18n(c, i18n.MsgChannelGetKeyFailed, map[string]any{"Error": apiErr.Error()})
		return
	}
	key = strings.TrimSpace(key)

	headers, err := buildFetchModelsHeaders(channel, key)
	if err != nil {
		common.SysError("failed to build fetch models headers: " + err.Error())
		httpapi.ApiErrorI18n(c, i18n.MsgInvalidParams)
		return
	}

	body, err := GetResponseBody("GET", url, channel, headers)
	if err != nil {
		common.SysError("failed to fetch upstream models: " + err.Error())
		httpapi.ApiErrorI18n(c, i18n.MsgChannelParseResponseFailed, map[string]any{"Error": err.Error()})
		return
	}

	var result struct {
		Data []struct {
			ID any `json:"id"`
		} `json:"data"`
	}

	if err = jsonx.Unmarshal(body, &result); err != nil {
		httpapi.ApiErrorI18n(c, i18n.MsgChannelParseResponseFailed, map[string]any{"Error": err.Error()})
		return
	}

	var ids []string
	for _, model := range result.Data {
		id := normalizeModelID(model.ID)
		if channel.Type == constant.ChannelTypeGemini {
			id = strings.TrimPrefix(id, "models/")
		}
		if id == "" {
			continue
		}
		ids = append(ids, id)
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    ids,
	})
}

func FixChannelsAbilities(c *gin.Context) {
	success, fails, err := channelstore.FixAbility()
	if err != nil {
		common.SysError("failed to fix channel abilities: " + err.Error())
		httpapi.ApiErrorI18n(c, i18n.MsgDatabaseError)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data": gin.H{
			"success": success,
			"fails":   fails,
		},
	})
}

func SearchChannels(c *gin.Context) {
	keyword := c.Query("keyword")
	group := c.Query("group")
	modelKeyword := c.Query("model")
	statusParam := c.Query("status")
	statusFilter := parseStatusFilter(statusParam)
	idSort, _ := strconv.ParseBool(c.Query("id_sort"))
	enableTagMode, _ := strconv.ParseBool(c.Query("tag_mode"))

	// 独立字段过滤：id 精确、name/tag 模糊（>0 或非空才生效）
	idFilter := 0
	if idStr := c.Query("id"); idStr != "" {
		if v, err := strconv.Atoi(idStr); err == nil && v > 0 {
			idFilter = v
		}
	}
	nameFilter := c.Query("name")
	tagFilter := c.Query("tag")

	// 分页参数在查询前解析（非 tag_mode 分支下推到 SQL 分页）
	page, _ := strconv.Atoi(c.DefaultQuery("p", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	if page < 1 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 20
	}
	// offset 必须基于与 SQL LIMIT 相同的有效 page size 计算，
	// 否则 page_size 超上限时（如 501）LIMIT 截断到 500 而 offset 按原始值
	// 跳过，导致条目丢失。
	pageSize = channelstore.NormalizeChannelSearchLimit(pageSize)

	channelData := make([]*channelstore.Channel, 0)
	// 非 tag_mode 分支的 type_counts 聚合与 status/type 过滤均下推到 SQL，
	// 由 channelstore.SearchChannelsWithMeta 一并返回，避免全量加载到内存。
	var typeCounts map[int64]int64
	var total int64

	if enableTagMode {
		tags, err := channelstore.SearchTags(keyword, group, modelKeyword, idSort, idFilter, nameFilter, tagFilter)
		if err != nil {
			c.JSON(http.StatusOK, gin.H{
				"success": false,
				"message": err.Error(),
			})
			return
		}
		for _, tag := range tags {
			if tag != nil && *tag != "" {
				tagChannel, err := channelstore.GetChannelsByTagWithGroup(*tag, group, idSort, false)
				if err == nil {
					channelData = append(channelData, tagChannel...)
				}
			}
		}
	} else {
		typeParam := c.Query("type")
		typeFilter := -1
		if typeParam != "" {
			if tp, err := strconv.Atoi(typeParam); err == nil {
				typeFilter = tp
			}
		}

		channels, cnt, counts, err := channelstore.SearchChannelsWithMeta(keyword, group, modelKeyword, idSort, idFilter, nameFilter, tagFilter, statusFilter, typeFilter, (page-1)*pageSize, pageSize)
		if err != nil {
			c.JSON(http.StatusOK, gin.H{
				"success": false,
				"message": err.Error(),
			})
			return
		}
		channelData = channels
		total = cnt
		typeCounts = counts
	}

	if enableTagMode {
		// tag 聚合走不同路径（渠道数受 tag 限制），保留内存过滤与分页。
		// 语义与非 tag_mode 分支一致：status 过滤 → type_counts 统计 →
		// type 过滤 → 分页。
		if statusFilter == common.ChannelStatusEnabled || statusFilter == 0 {
			filtered := make([]*channelstore.Channel, 0, len(channelData))
			for _, ch := range channelData {
				if statusFilter == common.ChannelStatusEnabled && ch.Status != common.ChannelStatusEnabled {
					continue
				}
				if statusFilter == 0 && ch.Status == common.ChannelStatusEnabled {
					continue
				}
				filtered = append(filtered, ch)
			}
			channelData = filtered
		}

		// calculate type counts for search results
		typeCounts = make(map[int64]int64)
		for _, channel := range channelData {
			typeCounts[int64(channel.Type)]++
		}

		typeParam := c.Query("type")
		typeFilter := -1
		if typeParam != "" {
			if tp, err := strconv.Atoi(typeParam); err == nil {
				typeFilter = tp
			}
		}

		if typeFilter >= 0 {
			filtered := make([]*channelstore.Channel, 0, len(channelData))
			for _, ch := range channelData {
				if ch.Type == typeFilter {
					filtered = append(filtered, ch)
				}
			}
			channelData = filtered
		}

		total = int64(len(channelData))
	}

	startIdx := (page - 1) * pageSize
	if startIdx > int(total) {
		startIdx = int(total)
	}
	endIdx := startIdx + pageSize
	if endIdx > int(total) {
		endIdx = int(total)
	}

	pagedData := channelData
	if enableTagMode {
		pagedData = channelData[startIdx:endIdx]
	}

	for _, datum := range pagedData {
		clearChannelInfo(datum)
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data": gin.H{
			"items":       pagedData,
			"total":       total,
			"type_counts": typeCounts,
		},
	})
}

func GetChannel(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		httpapi.ApiErrorI18n(c, i18n.MsgChannelIDFormatError, map[string]any{"Error": err.Error()})
		return
	}
	channel, err := channelstore.GetChannelById(id, false)
	if err != nil {
		common.SysError("failed to get channel by id: " + err.Error())
		httpapi.ApiErrorI18n(c, i18n.MsgDatabaseError)
		return
	}
	if channel != nil {
		clearChannelInfo(channel)
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    channel,
	})
}

// GetChannelKey 获取渠道密钥（需要通过安全验证中间件）
// 此函数依赖 SecureVerificationRequired 中间件，确保用户已通过安全验证
func GetChannelKey(c *gin.Context) {
	userId := c.GetInt("id")
	channelId, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		httpapi.ApiErrorI18n(c, i18n.MsgChannelIDFormatError, map[string]any{"Error": err.Error()})
		return
	}

	// 获取渠道信息（包含密钥）
	channel, err := channelstore.GetChannelById(channelId, true)
	if err != nil {
		httpapi.ApiErrorI18n(c, i18n.MsgChannelGetInfoFailed, map[string]any{"Error": err.Error()})
		return
	}

	if channel == nil {
		httpapi.ApiErrorI18n(c, i18n.MsgChannelNotFound)
		return
	}

	// 记录操作日志
	userstore.RecordLog(userId, logstore.LogTypeSystem, fmt.Sprintf("查看渠道密钥信息 (渠道ID: %d)", channelId))

	// 返回渠道密钥
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": i18n.T(c, i18n.MsgChannelGetSuccess),
		"data": map[string]interface{}{
			"key": channel.Key,
		},
	})
}

// validateChannel 通用的渠道校验函数
func validateChannel(c *gin.Context, channel *channelstore.Channel, isAdd bool) error {
	if channel == nil {
		return fmt.Errorf("%s", i18n.T(c, i18n.MsgChannelEmpty))
	}

	// 校验 channel settings
	if err := channel.ValidateSettings(); err != nil {
		return fmt.Errorf("%s", i18n.T(c, i18n.MsgChannelSettingsInvalid, map[string]any{"Error": err.Error()}))
	}

	// 如果是添加操作，检查模型名称长度
	if isAdd {
		for _, m := range channel.GetModels() {
			if len(m) > 255 {
				return fmt.Errorf("%s", i18n.T(c, i18n.MsgChannelModelNameTooLong, map[string]any{"Model": m}))
			}
		}
	}

	// 校验渠道类型是否合法（已移除/不支持的渠道类型直接拒绝）
	if channel.Type == constant.ChannelTypeUnknown {
		return fmt.Errorf("%s", i18n.T(c, i18n.MsgChannelInvalidType))
	}
	if _, ok := constant.ChannelTypeNames[channel.Type]; !ok {
		return fmt.Errorf("%s", i18n.T(c, i18n.MsgChannelUnsupportedType, map[string]any{"Type": channel.Type}))
	}

	// VertexAI 特殊校验
	if channel.Type == constant.ChannelTypeVertexAi {
		if channel.Other == "" {
			return fmt.Errorf("%s", i18n.T(c, i18n.MsgChannelVertexRegionRequired))
		}

		regionMap, err := common.StrToMap(channel.Other)
		if err != nil {
			return fmt.Errorf("%s", i18n.T(c, i18n.MsgChannelVertexRegionJSONInvalid))
		}

		if regionMap["default"] == nil {
			return fmt.Errorf("%s", i18n.T(c, i18n.MsgChannelVertexRegionDefaultMissing))
		}
	}

	return nil
}

type AddChannelRequest struct {
	Mode                      string                `json:"mode"`
	MultiKeyMode              constant.MultiKeyMode `json:"multi_key_mode"`
	BatchAddSetKeyPrefix2Name bool                  `json:"batch_add_set_key_prefix_2_name"`
	Channel                   *channelstore.Channel `json:"channel"`
}

func getVertexArrayKeys(c *gin.Context, keys string) ([]string, error) {
	if keys == "" {
		return nil, nil
	}
	var keyArray []interface{}
	err := jsonx.Unmarshal([]byte(keys), &keyArray)
	if err != nil {
		return nil, fmt.Errorf("%s", i18n.T(c, i18n.MsgChannelVertexBatchKeysJSONInvalid, map[string]any{"Error": err.Error()}))
	}
	cleanKeys := make([]string, 0, len(keyArray))
	for _, key := range keyArray {
		var keyStr string
		switch v := key.(type) {
		case string:
			keyStr = strings.TrimSpace(v)
		default:
			bytes, err := jsonx.Marshal(v)
			if err != nil {
				return nil, fmt.Errorf("%s", i18n.T(c, i18n.MsgChannelVertexKeyJSONEncodeFailed, map[string]any{"Error": err.Error()}))
			}
			keyStr = string(bytes)
		}
		if keyStr != "" {
			cleanKeys = append(cleanKeys, keyStr)
		}
	}
	if len(cleanKeys) == 0 {
		return nil, fmt.Errorf("%s", i18n.T(c, i18n.MsgChannelVertexBatchKeysEmpty))
	}
	return cleanKeys, nil
}

func AddChannel(c *gin.Context) {
	addChannelRequest := AddChannelRequest{}
	err := c.ShouldBindJSON(&addChannelRequest)
	if err != nil {
		httpapi.ApiErrorI18n(c, i18n.MsgInvalidRequestBody)
		return
	}

	// 使用统一的校验函数
	if err := validateChannel(c, addChannelRequest.Channel, true); err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}

	addChannelRequest.Channel.CreatedTime = common.GetTimestamp()
	// 检测套餐渠道
	addChannelRequest.Channel.DetectPlan()
	var keys []string
	switch addChannelRequest.Mode {
	case "multi_to_single":
		if !addChannelRequest.MultiKeyMode.Valid() {
			httpapi.ApiErrorI18n(c, i18n.MsgChannelMultiKeyModeInvalid, map[string]any{"Mode": addChannelRequest.MultiKeyMode})
			return
		}
		addChannelRequest.Channel.ChannelInfo.IsMultiKey = true
		addChannelRequest.Channel.ChannelInfo.MultiKeyMode = addChannelRequest.MultiKeyMode
		if addChannelRequest.Channel.Type == constant.ChannelTypeVertexAi && addChannelRequest.Channel.GetOtherSettings().VertexKeyType != shared.VertexKeyTypeAPIKey {
			array, err := getVertexArrayKeys(c, addChannelRequest.Channel.Key)
			if err != nil {
				c.JSON(http.StatusOK, gin.H{
					"success": false,
					"message": err.Error(),
				})
				return
			}
			addChannelRequest.Channel.ChannelInfo.MultiKeySize = len(array)
			addChannelRequest.Channel.Key = strings.Join(array, "\n")
		} else {
			cleanKeys := make([]string, 0)
			for _, key := range strings.Split(addChannelRequest.Channel.Key, "\n") {
				if key == "" {
					continue
				}
				key = strings.TrimSpace(key)
				cleanKeys = append(cleanKeys, key)
			}
			addChannelRequest.Channel.ChannelInfo.MultiKeySize = len(cleanKeys)
			addChannelRequest.Channel.Key = strings.Join(cleanKeys, "\n")
		}
		keys = []string{addChannelRequest.Channel.Key}
	case "batch":
		if addChannelRequest.Channel.Type == constant.ChannelTypeVertexAi && addChannelRequest.Channel.GetOtherSettings().VertexKeyType != shared.VertexKeyTypeAPIKey {
			// multi json
			keys, err = getVertexArrayKeys(c, addChannelRequest.Channel.Key)
			if err != nil {
				c.JSON(http.StatusOK, gin.H{
					"success": false,
					"message": err.Error(),
				})
				return
			}
		} else {
			keys = strings.Split(addChannelRequest.Channel.Key, "\n")
		}
	case "single":
		keys = []string{addChannelRequest.Channel.Key}
	default:
		httpapi.ApiErrorI18n(c, i18n.MsgChannelAddModeUnsupported)
		return
	}

	channels := make([]channelstore.Channel, 0, len(keys))
	for _, key := range keys {
		localChannel := addChannelRequest.Channel
		localChannel.Key = key
		if addChannelRequest.BatchAddSetKeyPrefix2Name && len(keys) > 1 {
			keyPrefix := localChannel.Key
			if len(localChannel.Key) > 8 {
				keyPrefix = localChannel.Key[:8]
			}
			localChannel.Name = fmt.Sprintf("%s %s", localChannel.Name, keyPrefix)
		}
		channels = append(channels, *localChannel)
	}
	err = channelstore.BatchInsertChannels(channels)
	if err != nil {
		common.SysError("failed to batch insert channels: " + err.Error())
		httpapi.ApiErrorI18n(c, i18n.MsgDatabaseError)
		return
	}
	httpclient.ResetProxyClientCache()
	audit.RecordAudit(c, auditstore.AuditModuleChannel, auditstore.AuditActionCreate, "新增渠道: "+addChannelRequest.Channel.Name, nil, map[string]interface{}{"name": addChannelRequest.Channel.Name, "type": addChannelRequest.Channel.Type, "models": addChannelRequest.Channel.Models})
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
	})
}

func DeleteChannel(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	channel := channelstore.Channel{Id: id}
	err := channel.Delete()
	if err != nil {
		common.SysError("failed to delete channel: " + err.Error())
		httpapi.ApiErrorI18n(c, i18n.MsgDatabaseError)
		return
	}
	channelstore.InitChannelCache()
	audit.RecordAudit(c, auditstore.AuditModuleChannel, auditstore.AuditActionDelete, "删除渠道 #"+strconv.Itoa(id), nil, map[string]interface{}{"id": id})
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
	})
}

func DeleteDisabledChannel(c *gin.Context) {
	rows, err := channelstore.DeleteDisabledChannel()
	if err != nil {
		common.SysError("failed to delete disabled channels: " + err.Error())
		httpapi.ApiErrorI18n(c, i18n.MsgDatabaseError)
		return
	}
	channelstore.InitChannelCache()
	audit.RecordAudit(c, auditstore.AuditModuleChannel, auditstore.AuditActionDelete, "删除所有已禁用渠道", nil, nil)
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    rows,
	})
}

type ChannelTag struct {
	Tag            string  `json:"tag"`
	NewTag         *string `json:"new_tag"`
	ModelMapping   *string `json:"model_mapping"`
	Models         *string `json:"models"`
	Groups         *string `json:"groups"`
	ParamOverride  *string `json:"param_override"`
	HeaderOverride *string `json:"header_override"`
}

func DisableTagChannels(c *gin.Context) {
	channelTag := ChannelTag{}
	err := c.ShouldBindJSON(&channelTag)
	if err != nil || channelTag.Tag == "" {
		httpapi.ApiErrorI18n(c, i18n.MsgChannelParamInvalid)
		return
	}
	err = channelstore.DisableChannelByTag(channelTag.Tag)
	if err != nil {
		common.SysError("failed to disable channels by tag: " + err.Error())
		httpapi.ApiErrorI18n(c, i18n.MsgDatabaseError)
		return
	}
	channelstore.InitChannelCache()
	audit.RecordAudit(c, auditstore.AuditModuleChannel, auditstore.AuditActionUpdate, "按标签禁用渠道: "+channelTag.Tag, nil, map[string]interface{}{"tag": channelTag.Tag})
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
	})
}

func EnableTagChannels(c *gin.Context) {
	channelTag := ChannelTag{}
	err := c.ShouldBindJSON(&channelTag)
	if err != nil || channelTag.Tag == "" {
		httpapi.ApiErrorI18n(c, i18n.MsgChannelParamInvalid)
		return
	}
	err = channelstore.EnableChannelByTag(channelTag.Tag)
	if err != nil {
		common.SysError("failed to enable channels by tag: " + err.Error())
		httpapi.ApiErrorI18n(c, i18n.MsgDatabaseError)
		return
	}
	channelstore.InitChannelCache()
	audit.RecordAudit(c, auditstore.AuditModuleChannel, auditstore.AuditActionUpdate, "按标签启用渠道: "+channelTag.Tag, nil, map[string]interface{}{"tag": channelTag.Tag})
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
	})
}

func EditTagChannels(c *gin.Context) {
	channelTag := ChannelTag{}
	err := c.ShouldBindJSON(&channelTag)
	if err != nil {
		httpapi.ApiErrorI18n(c, i18n.MsgChannelParamInvalid)
		return
	}
	if channelTag.Tag == "" {
		httpapi.ApiErrorI18n(c, i18n.MsgChannelTagRequired)
		return
	}
	if channelTag.ParamOverride != nil {
		trimmed := strings.TrimSpace(*channelTag.ParamOverride)
		if trimmed != "" && !jsonx.Valid([]byte(trimmed)) {
			httpapi.ApiErrorI18n(c, i18n.MsgChannelParamOverrideJSONInvalid)
			return
		}
		channelTag.ParamOverride = common.GetPointer[string](trimmed)
	}
	if channelTag.HeaderOverride != nil {
		trimmed := strings.TrimSpace(*channelTag.HeaderOverride)
		if trimmed != "" && !jsonx.Valid([]byte(trimmed)) {
			httpapi.ApiErrorI18n(c, i18n.MsgChannelHeaderOverrideJSONInvalid)
			return
		}
		channelTag.HeaderOverride = common.GetPointer[string](trimmed)
	}
	err = channelstore.EditChannelByTag(channelTag.Tag, channelTag.NewTag, channelTag.ModelMapping, channelTag.Models, channelTag.Groups, channelTag.ParamOverride, channelTag.HeaderOverride)
	if err != nil {
		common.SysError("failed to edit channels by tag: " + err.Error())
		httpapi.ApiErrorI18n(c, i18n.MsgDatabaseError)
		return
	}
	channelstore.InitChannelCache()
	audit.RecordAudit(c, auditstore.AuditModuleChannel, auditstore.AuditActionUpdate, "按标签编辑渠道: "+channelTag.Tag, nil, channelTag)
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
	})
}

type ChannelBatch struct {
	Ids []int   `json:"ids"`
	Tag *string `json:"tag"`
}

func DeleteChannelBatch(c *gin.Context) {
	channelBatch := ChannelBatch{}
	err := c.ShouldBindJSON(&channelBatch)
	if err != nil || len(channelBatch.Ids) == 0 {
		httpapi.ApiErrorI18n(c, i18n.MsgChannelParamInvalid)
		return
	}
	err = channelstore.BatchDeleteChannels(channelBatch.Ids)
	if err != nil {
		common.SysError("failed to batch delete channels: " + err.Error())
		httpapi.ApiErrorI18n(c, i18n.MsgDatabaseError)
		return
	}
	channelstore.InitChannelCache()
	audit.RecordAudit(c, auditstore.AuditModuleChannel, auditstore.AuditActionDelete, "批量删除渠道", nil, map[string]interface{}{"ids": channelBatch.Ids})
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    len(channelBatch.Ids),
	})
}

type PatchChannel struct {
	channelstore.Channel
	MultiKeyMode *string `json:"multi_key_mode"`
	KeyMode      *string `json:"key_mode"` // 多key模式下密钥覆盖或者追加
}

func UpdateChannel(c *gin.Context) {
	channel := PatchChannel{}
	err := c.ShouldBindJSON(&channel)
	if err != nil {
		httpapi.ApiErrorI18n(c, i18n.MsgInvalidRequestBody)
		return
	}

	if channel.Id == 0 {
		httpapi.ApiErrorI18n(c, i18n.MsgChannelParamInvalid)
		return
	}

	// Preserve existing ChannelInfo to ensure multi-key channels keep correct state even if the client does not send ChannelInfo in the request.
	originChannel, err := channelstore.GetChannelById(channel.Id, true)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}

	// Some clients (e.g. the dashboard in edit mode) may omit immutable fields like `type` in patch updates.
	// Treat omitted fields as "no change" by inheriting from the original channel, so validation and key-mode logic can work correctly.
	if channel.Type == constant.ChannelTypeUnknown {
		channel.Type = originChannel.Type
	}
	if channel.OtherSettings == "" {
		channel.OtherSettings = originChannel.OtherSettings
	}
	// VertexAI requires `other` (region config). When patch-updating without `other`, inherit it so unrelated edits won't fail validation.
	if channel.Type == constant.ChannelTypeVertexAi && channel.Other == "" {
		channel.Other = originChannel.Other
	}

	// Always copy the original ChannelInfo so that fields like IsMultiKey and MultiKeySize are retained.
	channel.ChannelInfo = originChannel.ChannelInfo

	// 检测套餐渠道（如果 BaseURL 发生变化则更新套餐标记）
	if channel.BaseURL != nil {
		channel.DetectPlan()
	} else {
		channel.ChannelInfo.IsPlan = originChannel.ChannelInfo.IsPlan
		channel.ChannelInfo.PlanName = originChannel.ChannelInfo.PlanName
	}

	// If the request explicitly specifies a new MultiKeyMode, apply it on top of the original info.
	if channel.MultiKeyMode != nil && *channel.MultiKeyMode != "" {
		mode := constant.MultiKeyMode(*channel.MultiKeyMode)
		if !mode.Valid() {
			httpapi.ApiErrorI18n(c, i18n.MsgChannelMultiKeyModeInvalid, map[string]any{"Mode": *channel.MultiKeyMode})
			return
		}
		channel.ChannelInfo.MultiKeyMode = mode
	}

	// 使用统一的校验函数（在继承必要字段后再校验，避免缺字段导致误判）
	if err := validateChannel(c, &channel.Channel, false); err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}

	// 处理多key模式下的密钥追加/覆盖逻辑
	if channel.KeyMode != nil && channel.ChannelInfo.IsMultiKey {
		switch *channel.KeyMode {
		case "append":
			// 追加模式：将新密钥添加到现有密钥列表
			if originChannel.Key != "" {
				var newKeys []string
				var existingKeys []string

				// 解析现有密钥
				if strings.HasPrefix(strings.TrimSpace(originChannel.Key), "[") {
					// JSON数组格式
					var arr []json.RawMessage
					if err := jsonx.Unmarshal([]byte(strings.TrimSpace(originChannel.Key)), &arr); err == nil {
						existingKeys = make([]string, len(arr))
						for i, v := range arr {
							existingKeys[i] = string(v)
						}
					}
				} else {
					// 换行分隔格式
					existingKeys = strings.Split(strings.Trim(originChannel.Key, "\n"), "\n")
				}

				// 处理 Vertex AI 的特殊情况
				if channel.Type == constant.ChannelTypeVertexAi && channel.GetOtherSettings().VertexKeyType != shared.VertexKeyTypeAPIKey {
					// 尝试解析新密钥为JSON数组
					if strings.HasPrefix(strings.TrimSpace(channel.Key), "[") {
						array, err := getVertexArrayKeys(c, channel.Key)
						if err != nil {
							httpapi.ApiErrorI18n(c, i18n.MsgChannelAppendKeysParseFailed, map[string]any{"Error": err.Error()})
							return
						}
						newKeys = array
					} else {
						// 单个JSON密钥
						newKeys = []string{channel.Key}
					}
				} else {
					// 普通渠道的处理
					inputKeys := strings.Split(channel.Key, "\n")
					for _, key := range inputKeys {
						key = strings.TrimSpace(key)
						if key != "" {
							newKeys = append(newKeys, key)
						}
					}
				}

				seen := make(map[string]struct{}, len(existingKeys)+len(newKeys))
				for _, key := range existingKeys {
					normalized := strings.TrimSpace(key)
					if normalized == "" {
						continue
					}
					seen[normalized] = struct{}{}
				}
				dedupedNewKeys := make([]string, 0, len(newKeys))
				for _, key := range newKeys {
					normalized := strings.TrimSpace(key)
					if normalized == "" {
						continue
					}
					if _, ok := seen[normalized]; ok {
						continue
					}
					seen[normalized] = struct{}{}
					dedupedNewKeys = append(dedupedNewKeys, normalized)
				}

				allKeys := append(existingKeys, dedupedNewKeys...)
				channel.Key = strings.Join(allKeys, "\n")
			}
		case "replace":
			// 覆盖模式：直接使用新密钥（默认行为，不需要特殊处理）
		}
	}
	err = channel.Update()
	if err != nil {
		common.SysError("failed to update channel: " + err.Error())
		httpapi.ApiErrorI18n(c, i18n.MsgDatabaseError)
		return
	}
	channelstore.InitChannelCache()
	httpclient.ResetProxyClientCache()
	audit.RecordAudit(c, auditstore.AuditModuleChannel, auditstore.AuditActionUpdate, "修改渠道: "+channel.Name, originChannel, channel)
	channel.Key = ""
	clearChannelInfo(&channel.Channel)
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    channel,
	})
}

func resolveFetchModelsBaseURL(baseURL string) string {
	baseURL = strings.TrimSpace(baseURL)
	if plan, ok := constant.ChannelSpecialBases[baseURL]; ok && plan.OpenAIBaseURL != "" {
		return plan.OpenAIBaseURL
	}
	return baseURL
}

func validateFetchModelsURL(url string) error {
	fetchSetting := system.GetFetchSetting()
	return security.ValidateURLWithFetchSetting(url, fetchSetting.EnableSSRFProtection, fetchSetting.AllowPrivateIp, fetchSetting.DomainFilterMode, fetchSetting.IpFilterMode, fetchSetting.DomainList, fetchSetting.IpList, fetchSetting.AllowedPorts, fetchSetting.ApplyIPFilterForDomain)
}

func FetchModels(c *gin.Context) {
	var req struct {
		BaseURL    string            `json:"base_url"`
		Type       int               `json:"type"`
		Key        string            `json:"key"`
		AwsKeyType shared.AwsKeyType `json:"aws_key_type"`
		Proxy      string            `json:"proxy"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": i18n.T(c, i18n.MsgChannelInvalidRequest),
		})
		return
	}

	baseURL := strings.TrimSpace(req.BaseURL)
	if baseURL == "" {
		baseURL = constant.ChannelBaseURLs[req.Type]
	}

	// remove line breaks and extra spaces.
	key := strings.TrimSpace(req.Key)
	key = strings.Split(key, "\n")[0]

	if req.Type == constant.ChannelTypeOllama {
		if err := validateFetchModelsURL(fmt.Sprintf("%s/v1/models", strings.TrimRight(resolveFetchModelsBaseURL(baseURL), "/"))); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"success": false,
				"message": err.Error(),
			})
			return
		}
		models, err := ollama.FetchOllamaModels(baseURL, key, "")
		if err != nil {
			httpapi.ApiErrorI18n(c, i18n.MsgChannelOllamaGetModelsFailed, map[string]any{"Error": err.Error()})
			return
		}

		names := make([]string, 0, len(models))
		for _, modelInfo := range models {
			names = append(names, modelInfo.Name)
		}

		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"data":    names,
		})
		return
	}

	if req.Type == constant.ChannelTypeAws {
		models, err := awschannel.FetchClaudeModels(c.Request.Context(), key, req.AwsKeyType, strings.TrimSpace(req.Proxy))
		if err != nil {
			c.JSON(http.StatusOK, gin.H{"success": false, "message": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"success": true, "data": models})
		return
	}

	if req.Type == constant.ChannelTypeGemini {
		if err := validateFetchModelsURL(fmt.Sprintf("%s/v1beta/models", strings.TrimRight(baseURL, "/"))); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"success": false,
				"message": err.Error(),
			})
			return
		}
		headers := http.Header{}
		applyFetchModelsDefaultHeaders(headers)
		models, err := gemini.FetchGeminiModelsWithHeaders(baseURL, key, "", headers)
		if err != nil {
			httpapi.ApiErrorI18n(c, i18n.MsgChannelGeminiGetModelsFailed, map[string]any{"Error": err.Error()})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"data":    models,
		})
		return
	}

	// 复用带 CheckRedirect 的受控 client：每次跳转都会复查 SSRF 规则，
	// 防止初始校验通过后经 301/302/307/308 重定向探测内网或外带认证头。
	client := httpclient.GetHttpClient()
	var url string
	switch req.Type {
	case constant.ChannelTypeZhipu_v4:
		if plan, ok := constant.ChannelSpecialBases[baseURL]; ok && plan.OpenAIBaseURL != "" {
			url = fmt.Sprintf("%s/models", plan.OpenAIBaseURL)
		} else {
			url = fmt.Sprintf("%s/api/paas/v4/models", baseURL)
		}
	case constant.ChannelTypeMoonshot:
		if plan, ok := constant.ChannelSpecialBases[baseURL]; ok && plan.OpenAIBaseURL != "" {
			url = fmt.Sprintf("%s/models", plan.OpenAIBaseURL)
		} else {
			url = fmt.Sprintf("%s/v1/models", baseURL)
		}
	default:
		url = fmt.Sprintf("%s/v1/models", baseURL)
	}

	if err := validateFetchModelsURL(url); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}

	request, err := http.NewRequest("GET", url, nil)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}

	var headers http.Header
	switch req.Type {
	case constant.ChannelTypeAnthropic:
		headers = GetClaudeAuthHeader(key)
	default:
		headers = GetAuthHeader(key)
	}
	applyFetchModelsDefaultHeaders(headers)
	request.Header = headers

	response, err := client.Do(request)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}
	//check status code
	if response.StatusCode != http.StatusOK {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": i18n.T(c, i18n.MsgChannelFetchModelsFailed),
		})
		return
	}
	defer response.Body.Close()

	var result struct {
		Data []struct {
			ID any `json:"id"`
		} `json:"data"`
	}

	if err := jsonx.DecodeJson(response.Body, &result); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}

	var models []string
	for _, model := range result.Data {
		id := normalizeModelID(model.ID)
		if id == "" {
			continue
		}
		models = append(models, id)
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    models,
	})
}

func BatchSetChannelTag(c *gin.Context) {
	channelBatch := ChannelBatch{}
	err := c.ShouldBindJSON(&channelBatch)
	if err != nil || len(channelBatch.Ids) == 0 {
		httpapi.ApiErrorI18n(c, i18n.MsgChannelParamInvalid)
		return
	}
	err = channelstore.BatchSetChannelTag(channelBatch.Ids, channelBatch.Tag)
	if err != nil {
		common.SysError("failed to batch set channel tag: " + err.Error())
		httpapi.ApiErrorI18n(c, i18n.MsgDatabaseError)
		return
	}
	channelstore.InitChannelCache()
	audit.RecordAudit(c, auditstore.AuditModuleChannel, auditstore.AuditActionUpdate, "批量设置渠道标签", nil, map[string]interface{}{"ids": channelBatch.Ids, "tag": channelBatch.Tag})
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    len(channelBatch.Ids),
	})
}

func GetTagModels(c *gin.Context) {
	tag := c.Query("tag")
	if tag == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": i18n.T(c, i18n.MsgChannelTagRequired),
		})
		return
	}

	channels, err := channelstore.GetChannelsByTag(tag, false, false) // idSort=false, selectAll=false
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}

	var longestModels string
	maxLength := 0

	// Find the longest models string among all channels with the given tag
	for _, channel := range channels {
		if channel.Models != "" {
			currentModels := strings.Split(channel.Models, ",")
			if len(currentModels) > maxLength {
				maxLength = len(currentModels)
				longestModels = channel.Models
			}
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    longestModels,
	})
}

// CopyChannel handles cloning an existing channel with its key.
// POST /api/channel/copy/:id
// Optional query params:
//
//	suffix         - string appended to the original name (default "_复制")
//	reset_balance  - bool, when true will reset balance & used_quota to 0 (default true)
func CopyChannel(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": i18n.T(c, i18n.MsgChannelInvalidID)})
		return
	}

	suffix := c.DefaultQuery("suffix", "_复制")
	resetBalance := true
	if rbStr := c.DefaultQuery("reset_balance", "true"); rbStr != "" {
		if v, err := strconv.ParseBool(rbStr); err == nil {
			resetBalance = v
		}
	}

	// fetch original channel with key
	origin, err := channelstore.GetChannelById(id, true)
	if err != nil {
		common.SysError("failed to get channel by id: " + err.Error())
		httpapi.ApiErrorI18n(c, i18n.MsgChannelCopyInfoFailed)
		return
	}

	// clone channel
	clone := *origin // shallow copy is sufficient as we will overwrite primitives
	clone.Id = 0     // let DB auto-generate
	clone.CreatedTime = common.GetTimestamp()
	clone.Name = origin.Name + suffix
	clone.TestTime = 0
	clone.ResponseTime = 0
	if resetBalance {
		clone.Balance = 0
		clone.UsedQuota = 0
	}

	// insert
	if err := channelstore.BatchInsertChannels([]channelstore.Channel{clone}); err != nil {
		common.SysError("failed to clone channel: " + err.Error())
		httpapi.ApiErrorI18n(c, i18n.MsgChannelCopyFailed)
		return
	}
	channelstore.InitChannelCache()
	audit.RecordAudit(c, auditstore.AuditModuleChannel, auditstore.AuditActionCreate, "复制渠道: "+clone.Name, map[string]interface{}{"source_id": id}, map[string]interface{}{"name": clone.Name, "type": clone.Type})
	// success
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "", "data": gin.H{"id": clone.Id}})
}

// MultiKeyManageRequest represents the request for multi-key management operations
type MultiKeyManageRequest struct {
	ChannelId int    `json:"channel_id"`
	Action    string `json:"action"`              // "disable_key", "enable_key", "delete_key", "delete_disabled_keys", "get_key_status"
	KeyIndex  *int   `json:"key_index,omitempty"` // for disable_key, enable_key, and delete_key actions
	Page      int    `json:"page,omitempty"`      // for get_key_status pagination
	PageSize  int    `json:"page_size,omitempty"` // for get_key_status pagination
	Status    *int   `json:"status,omitempty"`    // for get_key_status filtering: 1=enabled, 2=manual_disabled, 3=auto_disabled, nil=all
}

// MultiKeyStatusResponse represents the response for key status query
type MultiKeyStatusResponse struct {
	Keys       []KeyStatus `json:"keys"`
	Total      int         `json:"total"`
	Page       int         `json:"page"`
	PageSize   int         `json:"page_size"`
	TotalPages int         `json:"total_pages"`
	// Statistics
	EnabledCount        int `json:"enabled_count"`
	ManualDisabledCount int `json:"manual_disabled_count"`
	AutoDisabledCount   int `json:"auto_disabled_count"`
}

type KeyStatus struct {
	Index        int    `json:"index"`
	Status       int    `json:"status"` // 1: enabled, 2: disabled
	DisabledTime int64  `json:"disabled_time,omitempty"`
	Reason       string `json:"reason,omitempty"`
	KeyPreview   string `json:"key_preview"` // first 10 chars of key for identification
}

// ManageMultiKeys handles multi-key management operations
func ManageMultiKeys(c *gin.Context) {
	request := MultiKeyManageRequest{}
	err := c.ShouldBindJSON(&request)
	if err != nil {
		httpapi.ApiErrorI18n(c, i18n.MsgInvalidRequestBody)
		return
	}

	channel, err := channelstore.GetChannelById(request.ChannelId, true)
	if err != nil {
		httpapi.ApiErrorI18n(c, i18n.MsgChannelNotFound)
		return
	}

	if !channel.ChannelInfo.IsMultiKey {
		httpapi.ApiErrorI18n(c, i18n.MsgChannelNotMultiKey)
		return
	}

	lock := channelstore.GetChannelPollingLock(channel.Id)
	lock.Lock()
	defer lock.Unlock()

	// 保存更新前的 channel 快照（map 副本），用于审计差异对比。
	// 使用 JSON 往返实现深拷贝，避免指针修改导致 before 数据被污染。
	originChannelMap := func() map[string]interface{} {
		bytes, err := jsonx.Marshal(channel)
		if err != nil {
			return nil
		}
		var m map[string]interface{}
		if err := jsonx.Unmarshal(bytes, &m); err != nil {
			return nil
		}
		return m
	}()

	switch request.Action {
	case "get_key_status":
		keys := channel.GetKeys()

		// Default pagination parameters
		page := request.Page
		pageSize := request.PageSize
		if page <= 0 {
			page = 1
		}
		if pageSize <= 0 {
			pageSize = 50 // Default page size
		}

		// Statistics for all keys (unchanged by filtering)
		var enabledCount, manualDisabledCount, autoDisabledCount int

		// Build all key status data first
		var allKeyStatusList []KeyStatus
		for i, key := range keys {
			status := 1 // default enabled
			var disabledTime int64
			var reason string

			if channel.ChannelInfo.MultiKeyStatusList != nil {
				if s, exists := channel.ChannelInfo.MultiKeyStatusList[i]; exists {
					status = s
				}
			}

			// Count for statistics (all keys)
			switch status {
			case 1:
				enabledCount++
			case 2:
				manualDisabledCount++
			case 3:
				autoDisabledCount++
			}

			if status != 1 {
				if channel.ChannelInfo.MultiKeyDisabledTime != nil {
					disabledTime = channel.ChannelInfo.MultiKeyDisabledTime[i]
				}
				if channel.ChannelInfo.MultiKeyDisabledReason != nil {
					reason = channel.ChannelInfo.MultiKeyDisabledReason[i]
				}
			}

			// Create key preview (first 10 chars)
			keyPreview := key
			if len(key) > 10 {
				keyPreview = key[:10] + "..."
			}

			allKeyStatusList = append(allKeyStatusList, KeyStatus{
				Index:        i,
				Status:       status,
				DisabledTime: disabledTime,
				Reason:       reason,
				KeyPreview:   keyPreview,
			})
		}

		// Apply status filter if specified
		var filteredKeyStatusList []KeyStatus
		if request.Status != nil {
			for _, keyStatus := range allKeyStatusList {
				if keyStatus.Status == *request.Status {
					filteredKeyStatusList = append(filteredKeyStatusList, keyStatus)
				}
			}
		} else {
			filteredKeyStatusList = allKeyStatusList
		}

		// Calculate pagination based on filtered results
		filteredTotal := len(filteredKeyStatusList)
		totalPages := (filteredTotal + pageSize - 1) / pageSize
		if totalPages == 0 {
			totalPages = 1
		}
		if page > totalPages {
			page = totalPages
		}

		// Calculate range for current page
		start := (page - 1) * pageSize
		end := start + pageSize
		if end > filteredTotal {
			end = filteredTotal
		}

		// Get the page data
		var pageKeyStatusList []KeyStatus
		if start < filteredTotal {
			pageKeyStatusList = filteredKeyStatusList[start:end]
		}

		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"message": "",
			"data": MultiKeyStatusResponse{
				Keys:                pageKeyStatusList,
				Total:               filteredTotal, // Total of filtered results
				Page:                page,
				PageSize:            pageSize,
				TotalPages:          totalPages,
				EnabledCount:        enabledCount,        // Overall statistics
				ManualDisabledCount: manualDisabledCount, // Overall statistics
				AutoDisabledCount:   autoDisabledCount,   // Overall statistics
			},
		})
		return

	case "disable_key":
		if request.KeyIndex == nil {
			httpapi.ApiErrorI18n(c, i18n.MsgChannelDisableKeyIndexRequired)
			return
		}

		keyIndex := *request.KeyIndex
		if keyIndex < 0 || keyIndex >= channel.ChannelInfo.MultiKeySize {
			httpapi.ApiErrorI18n(c, i18n.MsgChannelKeyIndexOutOfRange)
			return
		}

		if channel.ChannelInfo.MultiKeyStatusList == nil {
			channel.ChannelInfo.MultiKeyStatusList = make(map[int]int)
		}
		if channel.ChannelInfo.MultiKeyDisabledTime == nil {
			channel.ChannelInfo.MultiKeyDisabledTime = make(map[int]int64)
		}
		if channel.ChannelInfo.MultiKeyDisabledReason == nil {
			channel.ChannelInfo.MultiKeyDisabledReason = make(map[int]string)
		}

		channel.ChannelInfo.MultiKeyStatusList[keyIndex] = 2 // disabled

		err = channel.Update()
		if err != nil {
			common.SysError("failed to update channel: " + err.Error())
			httpapi.ApiErrorI18n(c, i18n.MsgDatabaseError)
			return
		}

		channelstore.InitChannelCache()
		audit.RecordAudit(c, auditstore.AuditModuleChannel, auditstore.AuditActionUpdate, "管理多密钥渠道: "+request.Action, originChannelMap, channel)
		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"message": i18n.T(c, i18n.MsgChannelKeyDisabled),
		})
		return

	case "enable_key":
		if request.KeyIndex == nil {
			httpapi.ApiErrorI18n(c, i18n.MsgChannelEnableKeyIndexRequired)
			return
		}

		keyIndex := *request.KeyIndex
		if keyIndex < 0 || keyIndex >= channel.ChannelInfo.MultiKeySize {
			httpapi.ApiErrorI18n(c, i18n.MsgChannelKeyIndexOutOfRange)
			return
		}

		// 从状态列表中删除该密钥的记录，使其回到默认启用状态
		if channel.ChannelInfo.MultiKeyStatusList != nil {
			delete(channel.ChannelInfo.MultiKeyStatusList, keyIndex)
		}
		if channel.ChannelInfo.MultiKeyDisabledTime != nil {
			delete(channel.ChannelInfo.MultiKeyDisabledTime, keyIndex)
		}
		if channel.ChannelInfo.MultiKeyDisabledReason != nil {
			delete(channel.ChannelInfo.MultiKeyDisabledReason, keyIndex)
		}

		err = channel.Update()
		if err != nil {
			common.SysError("failed to update channel: " + err.Error())
			httpapi.ApiErrorI18n(c, i18n.MsgDatabaseError)
			return
		}

		channelstore.InitChannelCache()
		audit.RecordAudit(c, auditstore.AuditModuleChannel, auditstore.AuditActionUpdate, "管理多密钥渠道: "+request.Action, originChannelMap, channel)
		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"message": i18n.T(c, i18n.MsgChannelKeyEnabled),
		})
		return

	case "enable_all_keys":
		// 清空所有禁用状态，使所有密钥回到默认启用状态
		var enabledCount int
		if channel.ChannelInfo.MultiKeyStatusList != nil {
			enabledCount = len(channel.ChannelInfo.MultiKeyStatusList)
		}

		channel.ChannelInfo.MultiKeyStatusList = make(map[int]int)
		channel.ChannelInfo.MultiKeyDisabledTime = make(map[int]int64)
		channel.ChannelInfo.MultiKeyDisabledReason = make(map[int]string)

		err = channel.Update()
		if err != nil {
			common.SysError("failed to update channel: " + err.Error())
			httpapi.ApiErrorI18n(c, i18n.MsgDatabaseError)
			return
		}

		channelstore.InitChannelCache()
		audit.RecordAudit(c, auditstore.AuditModuleChannel, auditstore.AuditActionUpdate, "管理多密钥渠道: "+request.Action, originChannelMap, channel)
		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"message": i18n.T(c, i18n.MsgChannelKeysEnabled, map[string]any{"Count": enabledCount}),
		})
		return

	case "disable_all_keys":
		// 禁用所有启用的密钥
		if channel.ChannelInfo.MultiKeyStatusList == nil {
			channel.ChannelInfo.MultiKeyStatusList = make(map[int]int)
		}
		if channel.ChannelInfo.MultiKeyDisabledTime == nil {
			channel.ChannelInfo.MultiKeyDisabledTime = make(map[int]int64)
		}
		if channel.ChannelInfo.MultiKeyDisabledReason == nil {
			channel.ChannelInfo.MultiKeyDisabledReason = make(map[int]string)
		}

		var disabledCount int
		for i := 0; i < channel.ChannelInfo.MultiKeySize; i++ {
			status := 1 // default enabled
			if s, exists := channel.ChannelInfo.MultiKeyStatusList[i]; exists {
				status = s
			}

			// 只禁用当前启用的密钥
			if status == 1 {
				channel.ChannelInfo.MultiKeyStatusList[i] = 2 // disabled
				disabledCount++
			}
		}

		if disabledCount == 0 {
			httpapi.ApiErrorI18n(c, i18n.MsgChannelNoKeysToDisable)
			return
		}

		err = channel.Update()
		if err != nil {
			common.SysError("failed to update channel: " + err.Error())
			httpapi.ApiErrorI18n(c, i18n.MsgDatabaseError)
			return
		}

		channelstore.InitChannelCache()
		audit.RecordAudit(c, auditstore.AuditModuleChannel, auditstore.AuditActionUpdate, "管理多密钥渠道: "+request.Action, originChannelMap, channel)
		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"message": i18n.T(c, i18n.MsgChannelKeysDisabled, map[string]any{"Count": disabledCount}),
		})
		return

	case "delete_key":
		if request.KeyIndex == nil {
			httpapi.ApiErrorI18n(c, i18n.MsgChannelDeleteKeyIndexRequired)
			return
		}

		keyIndex := *request.KeyIndex
		if keyIndex < 0 || keyIndex >= channel.ChannelInfo.MultiKeySize {
			httpapi.ApiErrorI18n(c, i18n.MsgChannelKeyIndexOutOfRange)
			return
		}

		keys := channel.GetKeys()
		var remainingKeys []string
		var newStatusList = make(map[int]int)
		var newDisabledTime = make(map[int]int64)
		var newDisabledReason = make(map[int]string)

		newIndex := 0
		for i, key := range keys {
			// 跳过要删除的密钥
			if i == keyIndex {
				continue
			}

			remainingKeys = append(remainingKeys, key)

			// 保留其他密钥的状态信息，重新索引
			if channel.ChannelInfo.MultiKeyStatusList != nil {
				if status, exists := channel.ChannelInfo.MultiKeyStatusList[i]; exists && status != 1 {
					newStatusList[newIndex] = status
				}
			}
			if channel.ChannelInfo.MultiKeyDisabledTime != nil {
				if t, exists := channel.ChannelInfo.MultiKeyDisabledTime[i]; exists {
					newDisabledTime[newIndex] = t
				}
			}
			if channel.ChannelInfo.MultiKeyDisabledReason != nil {
				if r, exists := channel.ChannelInfo.MultiKeyDisabledReason[i]; exists {
					newDisabledReason[newIndex] = r
				}
			}
			newIndex++
		}

		if len(remainingKeys) == 0 {
			httpapi.ApiErrorI18n(c, i18n.MsgChannelLastKeyCannotDelete)
			return
		}

		// Update channel with remaining keys
		channel.Key = strings.Join(remainingKeys, "\n")
		channel.ChannelInfo.MultiKeySize = len(remainingKeys)
		channel.ChannelInfo.MultiKeyStatusList = newStatusList
		channel.ChannelInfo.MultiKeyDisabledTime = newDisabledTime
		channel.ChannelInfo.MultiKeyDisabledReason = newDisabledReason

		err = channel.Update()
		if err != nil {
			common.SysError("failed to update channel: " + err.Error())
			httpapi.ApiErrorI18n(c, i18n.MsgDatabaseError)
			return
		}

		channelstore.InitChannelCache()
		audit.RecordAudit(c, auditstore.AuditModuleChannel, auditstore.AuditActionUpdate, "管理多密钥渠道: "+request.Action, originChannelMap, channel)
		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"message": i18n.T(c, i18n.MsgChannelKeyDeleted),
		})
		return

	case "delete_disabled_keys":
		keys := channel.GetKeys()
		var remainingKeys []string
		var deletedCount int
		var newStatusList = make(map[int]int)
		var newDisabledTime = make(map[int]int64)
		var newDisabledReason = make(map[int]string)

		newIndex := 0
		for i, key := range keys {
			status := 1 // default enabled
			if channel.ChannelInfo.MultiKeyStatusList != nil {
				if s, exists := channel.ChannelInfo.MultiKeyStatusList[i]; exists {
					status = s
				}
			}

			// 只删除自动禁用（status == 3）的密钥，保留启用（status == 1）和手动禁用（status == 2）的密钥
			if status == 3 {
				deletedCount++
			} else {
				remainingKeys = append(remainingKeys, key)
				// 保留非自动禁用密钥的状态信息，重新索引
				if status != 1 {
					newStatusList[newIndex] = status
					if channel.ChannelInfo.MultiKeyDisabledTime != nil {
						if t, exists := channel.ChannelInfo.MultiKeyDisabledTime[i]; exists {
							newDisabledTime[newIndex] = t
						}
					}
					if channel.ChannelInfo.MultiKeyDisabledReason != nil {
						if r, exists := channel.ChannelInfo.MultiKeyDisabledReason[i]; exists {
							newDisabledReason[newIndex] = r
						}
					}
				}
				newIndex++
			}
		}

		if deletedCount == 0 {
			httpapi.ApiErrorI18n(c, i18n.MsgChannelNoAutoDisabledKeysToDelete)
			return
		}

		// Update channel with remaining keys
		channel.Key = strings.Join(remainingKeys, "\n")
		channel.ChannelInfo.MultiKeySize = len(remainingKeys)
		channel.ChannelInfo.MultiKeyStatusList = newStatusList
		channel.ChannelInfo.MultiKeyDisabledTime = newDisabledTime
		channel.ChannelInfo.MultiKeyDisabledReason = newDisabledReason

		err = channel.Update()
		if err != nil {
			common.SysError("failed to update channel: " + err.Error())
			httpapi.ApiErrorI18n(c, i18n.MsgDatabaseError)
			return
		}

		channelstore.InitChannelCache()
		audit.RecordAudit(c, auditstore.AuditModuleChannel, auditstore.AuditActionUpdate, "管理多密钥渠道: "+request.Action, originChannelMap, channel)
		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"message": i18n.T(c, i18n.MsgChannelAutoDisabledKeysDeleted, map[string]any{"Count": deletedCount}),
			"data":    deletedCount,
		})
		return

	default:
		httpapi.ApiErrorI18n(c, i18n.MsgChannelOperationUnsupported)
		return
	}
}

// OllamaPullModel 拉取 Ollama 模型
func OllamaPullModel(c *gin.Context) {
	var req struct {
		ChannelID int    `json:"channel_id"`
		ModelName string `json:"model_name"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": i18n.T(c, i18n.MsgChannelInvalidRequestParameters),
		})
		return
	}

	if req.ChannelID == 0 || req.ModelName == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": i18n.T(c, i18n.MsgChannelIdAndModelRequired),
		})
		return
	}

	// 获取渠道信息
	channel, err := channelstore.GetChannelById(req.ChannelID, true)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"success": false,
			"message": i18n.T(c, i18n.MsgChannelNotFound),
		})
		return
	}

	// 检查是否是 Ollama 渠道
	if channel.Type != constant.ChannelTypeOllama {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": i18n.T(c, i18n.MsgChannelOllamaOnly),
		})
		return
	}

	baseURL := constant.ChannelBaseURLs[channel.Type]
	if channel.GetBaseURL() != "" {
		baseURL = channel.GetBaseURL()
	}

	key := strings.Split(channel.Key, "\n")[0]
	err = ollama.PullOllamaModel(baseURL, key, channel.GetSetting().Proxy, req.ModelName)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": i18n.T(c, i18n.MsgChannelPullModelFailed, map[string]any{"Error": err.Error()}),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": i18n.T(c, i18n.MsgChannelPullModelSuccess, map[string]any{"Model": req.ModelName}),
	})
}

// OllamaPullModelStream 流式拉取 Ollama 模型
func OllamaPullModelStream(c *gin.Context) {
	var req struct {
		ChannelID int    `json:"channel_id"`
		ModelName string `json:"model_name"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": i18n.T(c, i18n.MsgChannelInvalidRequestParameters),
		})
		return
	}

	if req.ChannelID == 0 || req.ModelName == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": i18n.T(c, i18n.MsgChannelIdAndModelRequired),
		})
		return
	}

	// 获取渠道信息
	channel, err := channelstore.GetChannelById(req.ChannelID, true)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"success": false,
			"message": i18n.T(c, i18n.MsgChannelNotFound),
		})
		return
	}

	// 检查是否是 Ollama 渠道
	if channel.Type != constant.ChannelTypeOllama {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": i18n.T(c, i18n.MsgChannelOllamaOnly),
		})
		return
	}

	baseURL := constant.ChannelBaseURLs[channel.Type]
	if channel.GetBaseURL() != "" {
		baseURL = channel.GetBaseURL()
	}

	// 设置 SSE 头部
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("Access-Control-Allow-Origin", "*")

	key := strings.Split(channel.Key, "\n")[0]

	// 创建进度回调函数
	progressCallback := func(progress ollama.OllamaPullResponse) {
		data, _ := jsonx.Marshal(progress)
		fmt.Fprintf(c.Writer, "data: %s\n\n", string(data))
		c.Writer.Flush()
	}

	// 执行拉取
	err = ollama.PullOllamaModelStream(baseURL, key, channel.GetSetting().Proxy, req.ModelName, progressCallback)

	if err != nil {
		errorData, _ := jsonx.Marshal(gin.H{
			"error": err.Error(),
		})
		fmt.Fprintf(c.Writer, "data: %s\n\n", string(errorData))
	} else {
		successData, _ := jsonx.Marshal(gin.H{
			"message": i18n.T(c, i18n.MsgChannelPullModelSuccess, map[string]any{"Model": req.ModelName}),
		})
		fmt.Fprintf(c.Writer, "data: %s\n\n", string(successData))
	}

	// 发送结束标志
	fmt.Fprintf(c.Writer, "data: [DONE]\n\n")
	c.Writer.Flush()
}

// OllamaDeleteModel 删除 Ollama 模型
func OllamaDeleteModel(c *gin.Context) {
	var req struct {
		ChannelID int    `json:"channel_id"`
		ModelName string `json:"model_name"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": i18n.T(c, i18n.MsgChannelInvalidRequestParameters),
		})
		return
	}

	if req.ChannelID == 0 || req.ModelName == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": i18n.T(c, i18n.MsgChannelIdAndModelRequired),
		})
		return
	}

	// 获取渠道信息
	channel, err := channelstore.GetChannelById(req.ChannelID, true)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"success": false,
			"message": i18n.T(c, i18n.MsgChannelNotFound),
		})
		return
	}

	// 检查是否是 Ollama 渠道
	if channel.Type != constant.ChannelTypeOllama {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": i18n.T(c, i18n.MsgChannelOllamaOnly),
		})
		return
	}

	baseURL := constant.ChannelBaseURLs[channel.Type]
	if channel.GetBaseURL() != "" {
		baseURL = channel.GetBaseURL()
	}

	key := strings.Split(channel.Key, "\n")[0]
	err = ollama.DeleteOllamaModel(baseURL, key, channel.GetSetting().Proxy, req.ModelName)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": i18n.T(c, i18n.MsgChannelDeleteModelFailed, map[string]any{"Error": err.Error()}),
		})
		return
	}

	audit.RecordAudit(c, auditstore.AuditModuleChannel, auditstore.AuditActionDelete, "删除 Ollama 模型: "+req.ModelName, nil, map[string]interface{}{"model_name": req.ModelName})
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": i18n.T(c, i18n.MsgChannelDeleteModelSuccess, map[string]any{"Model": req.ModelName}),
	})
}

// OllamaVersion 获取 Ollama 服务版本信息
func OllamaVersion(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": i18n.T(c, i18n.MsgChannelInvalidID),
		})
		return
	}

	channel, err := channelstore.GetChannelById(id, true)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"success": false,
			"message": i18n.T(c, i18n.MsgChannelNotFound),
		})
		return
	}

	if channel.Type != constant.ChannelTypeOllama {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": i18n.T(c, i18n.MsgChannelOllamaOnly),
		})
		return
	}

	baseURL := constant.ChannelBaseURLs[channel.Type]
	if channel.GetBaseURL() != "" {
		baseURL = channel.GetBaseURL()
	}

	key := strings.Split(channel.Key, "\n")[0]
	version, err := ollama.FetchOllamaVersion(baseURL, key, channel.GetSetting().Proxy)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": i18n.T(c, i18n.MsgChannelOllamaVersionFailed, map[string]any{"Error": err.Error()}),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"version": version,
		},
	})
}

// QueryPlanQuota 查询套餐渠道的额度使用情况
func QueryPlanQuota(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	if id == 0 {
		httpapi.ApiErrorI18n(c, i18n.MsgChannelParamInvalid)
		return
	}

	channel, err := channelstore.GetChannelById(id, true)
	if err != nil {
		httpapi.ApiErrorI18n(c, i18n.MsgChannelNotFound)
		return
	}

	if !channel.ChannelInfo.IsPlan {
		httpapi.ApiErrorI18n(c, i18n.MsgChannelNotPlan)
		return
	}

	planName := channel.ChannelInfo.PlanName

	switch planName {
	case "glm-coding-plan", "glm-coding-plan-international":
		key := strings.Split(channel.Key, "\n")[0]
		quotaData, err := planquota.FetchGlmPlanQuota(key, planName, channel.GetSetting().Proxy)
		if err != nil {
			if errors.Is(err, planquota.ErrGlmKeyInvalid) {
				httpapi.ApiErrorI18n(c, i18n.MsgChannelKeyInvalid)
				return
			}
			httpapi.ApiErrorI18n(c, i18n.MsgChannelQuotaQueryFailed, map[string]any{"Error": err.Error()})
			return
		}
		quotaData.PlanName = planName
		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"data":    quotaData,
		})
		return
	case "kimi-coding-plan":
		key := strings.Split(channel.Key, "\n")[0]
		quotaData, err := planquota.FetchKimiPlanQuota(key, channel.GetSetting().Proxy)
		if err != nil {
			httpapi.ApiErrorI18n(c, i18n.MsgChannelQuotaQueryFailed, map[string]any{"Error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"data":    quotaData,
		})
		return
	case "minimax-coding-plan", "minimax-coding-plan-international":
		key := strings.Split(channel.Key, "\n")[0]
		quotaData, err := planquota.FetchMiniMaxPlanQuota(key, planName, channel.GetSetting().Proxy)
		if err != nil {
			httpapi.ApiErrorI18n(c, i18n.MsgChannelQuotaQueryFailed, map[string]any{"Error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"data":    quotaData,
		})
		return
	default:
		// 暂未支持实际查询的套餐
		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"data": gin.H{
				"plan_name":       planName,
				"quota_supported": false,
				"channel_id":      channel.Id,
				"channel_name":    channel.Name,
			},
		})
	}
}

// QueryGlmUsage 代理查询 GLM 套餐的用量图表数据
func QueryGlmUsage(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	if id == 0 {
		httpapi.ApiErrorI18n(c, i18n.MsgChannelParamInvalid)
		return
	}

	channel, err := channelstore.GetChannelById(id, true)
	if err != nil {
		httpapi.ApiErrorI18n(c, i18n.MsgChannelNotFound)
		return
	}

	planName := channel.ChannelInfo.PlanName
	if planName != "glm-coding-plan" && planName != "glm-coding-plan-international" {
		httpapi.ApiErrorI18n(c, i18n.MsgChannelUsageUnsupported)
		return
	}

	dataType := c.Query("type")
	if dataType == "" {
		dataType = "model"
	}
	startTime := c.Query("startTime")
	endTime := c.Query("endTime")

	// 校验时间范围不超过31天，防止滥用
	if startTime != "" && endTime != "" {
		layout := "2006-01-02 15:04:05"
		s := strings.ReplaceAll(startTime, "+", " ")
		e := strings.ReplaceAll(endTime, "+", " ")
		tStart, err1 := time.Parse(layout, s)
		tEnd, err2 := time.Parse(layout, e)
		if err1 != nil || err2 != nil {
			httpapi.ApiErrorI18n(c, i18n.MsgChannelTimeFormatInvalid)
			return
		}
		if tEnd.Before(tStart) {
			httpapi.ApiErrorI18n(c, i18n.MsgChannelEndBeforeStart)
			return
		}
		if tEnd.Sub(tStart).Hours() > 31*24 {
			httpapi.ApiErrorI18n(c, i18n.MsgChannelTimeRangeTooLong31Days)
			return
		}
	}

	key := strings.Split(channel.Key, "\n")[0]
	rawData, err := planquota.FetchGlmUsageData(key, planName, dataType, startTime, endTime, channel.GetSetting().Proxy)
	if err != nil {
		if errors.Is(err, planquota.ErrGlmKeyInvalid) {
			httpapi.ApiErrorI18n(c, i18n.MsgChannelKeyInvalid)
			return
		}
		httpapi.ApiErrorI18n(c, i18n.MsgChannelUsageQueryFailed, map[string]any{"Error": err.Error()})
		return
	}

	c.Data(http.StatusOK, "application/json", rawData)
}

// QueryGlmPlanActivity 查询智谱 GLM 个人套餐活跃数据（累计 / 峰值 / 连续天数 / 日历热力图）。
// 时间窗固定为服务器时区「今天 23:59:59 → 往前 365 天 00:00:00」，由 service 端计算并注入，
// 避免前端参数破坏 365 天语义。
func QueryGlmPlanActivity(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	if id == 0 {
		httpapi.ApiErrorI18n(c, i18n.MsgChannelParamInvalid)
		return
	}

	channel, err := channelstore.GetChannelById(id, true)
	if err != nil {
		httpapi.ApiErrorI18n(c, i18n.MsgChannelNotFound)
		return
	}

	if !channel.ChannelInfo.IsPlan {
		httpapi.ApiErrorI18n(c, i18n.MsgChannelNotPlan)
		return
	}

	planName := channel.ChannelInfo.PlanName
	if planName != "glm-coding-plan" && planName != "glm-coding-plan-international" {
		httpapi.ApiErrorI18n(c, i18n.MsgChannelActivityUnsupported)
		return
	}

	key := strings.Split(channel.Key, "\n")[0]
	startTime, endTime := planquota.ComputeGlmActivityTimeRange()

	data, err := planquota.FetchGlmCreditUsageActivity(key, planName, planquota.GlmActivityAccountPersonal, startTime, endTime, channel.GetSetting().Proxy)
	if err != nil {
		if errors.Is(err, planquota.ErrGlmKeyInvalid) {
			httpapi.ApiErrorI18n(c, i18n.MsgChannelKeyInvalid)
			return
		}
		httpapi.ApiErrorI18n(c, i18n.MsgChannelActivityQueryFailed, map[string]any{"Error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    data,
	})
}

// QueryRiskStatus 查询智谱 GLM 套餐渠道的风控状态
func QueryRiskStatus(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	if id == 0 {
		httpapi.ApiErrorI18n(c, i18n.MsgChannelParamInvalid)
		return
	}

	channel, err := channelstore.GetChannelById(id, true)
	if err != nil {
		httpapi.ApiErrorI18n(c, i18n.MsgChannelNotFound)
		return
	}

	if !channel.ChannelInfo.IsPlan {
		httpapi.ApiErrorI18n(c, i18n.MsgChannelNotPlan)
		return
	}

	planName := channel.ChannelInfo.PlanName
	if planName != "glm-coding-plan" && planName != "glm-coding-plan-international" {
		httpapi.ApiErrorI18n(c, i18n.MsgChannelRiskUnsupported)
		return
	}

	key := strings.Split(channel.Key, "\n")[0]
	result, err := planquota.CheckGlmRiskStatus(key, channel.GetSetting().Proxy)
	if err != nil {
		if errors.Is(err, planquota.ErrGlmKeyInvalid) {
			httpapi.ApiErrorI18n(c, i18n.MsgChannelKeyInvalid)
			return
		}
		httpapi.ApiErrorI18n(c, i18n.MsgChannelRiskCheckFailed, map[string]any{"Error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    result,
	})
}

// QueryGlmContactInfo 查询智谱渠道账户的联系方式（月度账单明细 customerAcName）。
// 不限套餐渠道：上游 base URL 按套餐区域划分（国际套餐 api.z.ai，
// 国内套餐与非套餐渠道 bigmodel.cn），Key 始终取数据库保存的渠道密钥。
func QueryGlmContactInfo(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	if id == 0 {
		httpapi.ApiErrorI18n(c, i18n.MsgChannelParamInvalid)
		return
	}

	channel, err := channelstore.GetChannelById(id, true)
	if err != nil {
		httpapi.ApiErrorI18n(c, i18n.MsgChannelNotFound)
		return
	}

	if channel.Type != constant.ChannelTypeZhipu_v4 {
		httpapi.ApiErrorI18n(c, i18n.MsgChannelContactUnsupported)
		return
	}

	key := strings.Split(channel.Key, "\n")[0]
	result, err := planquota.FetchGlmContactInfo(key, channel.ChannelInfo.PlanName, channel.GetSetting().Proxy)
	if err != nil {
		if errors.Is(err, planquota.ErrGlmKeyInvalid) {
			httpapi.ApiErrorI18n(c, i18n.MsgChannelKeyInvalid)
			return
		}
		httpapi.ApiErrorI18n(c, i18n.MsgChannelContactQueryFailed, map[string]any{"Error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    result,
	})
}

// QueryGlmAccountReport 查询智谱 GLM-4V 渠道的账户资金报告
// （余额 / 充值 / 赠金 / 消耗 / 可用 / 冻结）。Key 取数据库保存的渠道密钥由
// 服务端注入并强制携带浏览器 UA，浏览器不直连智谱后台；请求经渠道代理发出。
// 成功后同步把可用余额折算成 USD 落库，与通用余额更新（update_balance）一致。
func QueryGlmAccountReport(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	if id == 0 {
		httpapi.ApiErrorI18n(c, i18n.MsgChannelParamInvalid)
		return
	}

	channel, err := channelstore.GetChannelById(id, true)
	if err != nil {
		httpapi.ApiErrorI18n(c, i18n.MsgChannelNotFound)
		return
	}

	if channel.Type != constant.ChannelTypeZhipu_v4 {
		httpapi.ApiErrorI18n(c, i18n.MsgChannelBalanceNotImplemented)
		return
	}

	key := strings.Split(channel.Key, "\n")[0]
	report, err := planquota.FetchGlmAccountReport(key, channel.ChannelInfo.PlanName, channel.GetSetting().Proxy)
	if err != nil {
		if errors.Is(err, planquota.ErrGlmKeyInvalid) {
			httpapi.ApiErrorI18n(c, i18n.MsgChannelKeyInvalid)
			return
		}
		httpapi.ApiErrorI18n(c, i18n.MsgChannelQuotaQueryFailed, map[string]any{"Error": err.Error()})
		return
	}

	balanceCNY, err := planquota.PickGlmPersistBalance(report)
	if err != nil {
		httpapi.ApiErrorI18n(c, i18n.MsgChannelQuotaQueryFailed, map[string]any{"Error": err.Error()})
		return
	}
	balanceUSD := glmBalanceCNYToUSD(balanceCNY)
	channel.UpdateBalance(balanceUSD)

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    report,
		"balance": balanceUSD,
	})
}

// QueryGlmResetCards 查询智谱 GLM 套餐的重置卡列表。
// 上游 base URL 按套餐区域划分（国内 bigmodel.cn / 国际 api.z.ai），targetType 固定 PERSONAL。
func QueryGlmResetCards(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	if id == 0 {
		httpapi.ApiErrorI18n(c, i18n.MsgChannelParamInvalid)
		return
	}

	channel, err := channelstore.GetChannelById(id, true)
	if err != nil {
		httpapi.ApiErrorI18n(c, i18n.MsgChannelNotFound)
		return
	}

	if !channel.ChannelInfo.IsPlan {
		httpapi.ApiErrorI18n(c, i18n.MsgChannelNotPlan)
		return
	}

	planName := channel.ChannelInfo.PlanName
	if planName != "glm-coding-plan" && planName != "glm-coding-plan-international" {
		httpapi.ApiErrorI18n(c, i18n.MsgChannelResetCardUnsupported)
		return
	}

	key := strings.Split(channel.Key, "\n")[0]
	data, err := planquota.FetchGlmResetCards(key, planName, channel.GetSetting().Proxy)
	if err != nil {
		if errors.Is(err, planquota.ErrGlmKeyInvalid) {
			httpapi.ApiErrorI18n(c, i18n.MsgChannelKeyInvalid)
			return
		}
		httpapi.ApiErrorI18n(c, i18n.MsgChannelResetCardQueryFailed, map[string]any{"Error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    data,
	})
}

// UseGlmResetCard 使用智谱 GLM 套餐的重置卡。请求体含 resetType 与 recordId，
// targetType 由后端固定为 PERSONAL，requestId 由后端生成 uuid 注入，避免前端注入。
func UseGlmResetCard(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	if id == 0 {
		httpapi.ApiErrorI18n(c, i18n.MsgChannelParamInvalid)
		return
	}

	channel, err := channelstore.GetChannelById(id, true)
	if err != nil {
		httpapi.ApiErrorI18n(c, i18n.MsgChannelNotFound)
		return
	}

	if !channel.ChannelInfo.IsPlan {
		httpapi.ApiErrorI18n(c, i18n.MsgChannelNotPlan)
		return
	}

	planName := channel.ChannelInfo.PlanName
	if planName != "glm-coding-plan" && planName != "glm-coding-plan-international" {
		httpapi.ApiErrorI18n(c, i18n.MsgChannelResetCardUnsupported)
		return
	}

	var req struct {
		ResetType string `json:"resetType"`
		RecordId  int64  `json:"recordId"`
	}
	if err := jsonx.DecodeJson(c.Request.Body, &req); err != nil {
		httpapi.ApiErrorI18n(c, i18n.MsgChannelInvalidRequestParameters)
		return
	}
	if req.ResetType == "" || req.RecordId <= 0 {
		httpapi.ApiErrorI18n(c, i18n.MsgChannelInvalidRequestParameters)
		return
	}

	key := strings.Split(channel.Key, "\n")[0]
	result, err := planquota.UseGlmResetCard(key, planName, channel.GetSetting().Proxy, planquota.GlmResetCardUseRequest{
		TargetType: planquota.GlmResetCardTypePersonal,
		ResetType:  planquota.GlmResetCardType(req.ResetType),
		RecordId:   req.RecordId,
	})
	if err != nil {
		if errors.Is(err, planquota.ErrGlmKeyInvalid) {
			httpapi.ApiErrorI18n(c, i18n.MsgChannelKeyInvalid)
			return
		}
		httpapi.ApiErrorI18n(c, i18n.MsgChannelResetCardUseFailed, map[string]any{"Error": err.Error()})
		return
	}

	// 上游失败时 success=false，msg 透传给前端，便于前端按特定文案触发重新拉列表
	if !result.Success {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"msg":     result.Message,
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
	})
}
