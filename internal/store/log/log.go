package logstore

import (
	"context"
	"errors"
	"fmt"
	"github.com/NookMux/NookMux/internal/common"
	"github.com/NookMux/NookMux/internal/domain/shared"
	logger "github.com/NookMux/NookMux/internal/infra/log"
	"github.com/NookMux/NookMux/internal/infra/runtime"
	"github.com/NookMux/NookMux/internal/store/db"
	"github.com/NookMux/NookMux/internal/store/token"
	"github.com/NookMux/NookMux/internal/store/usedata"
	"github.com/NookMux/NookMux/internal/store/vendor_meta"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	"strings"
	"time"
)

type Log struct {
	Id               int    `json:"id" gorm:"index:idx_created_at_id,priority:1;index:idx_user_id_id,priority:2"`
	UserId           int    `json:"user_id" gorm:"index;index:idx_user_id_id,priority:1"`
	CreatedAt        int64  `json:"created_at" gorm:"bigint;index:idx_created_at_id,priority:2;index:idx_created_at_type"`
	Type             int    `json:"type" gorm:"index:idx_created_at_type"`
	Content          string `json:"content" gorm:"type:text"`
	Username         string `json:"username" gorm:"index;index:index_username_model_name,priority:2;default:''"`
	TokenName        string `json:"token_name" gorm:"index;default:''"`
	ModelName        string `json:"model_name" gorm:"index;index:index_username_model_name,priority:1;default:''"`
	Quota            int    `json:"quota" gorm:"default:0"`
	PromptTokens     int    `json:"prompt_tokens" gorm:"default:0"`
	CompletionTokens int    `json:"completion_tokens" gorm:"default:0"`
	UseTime          int    `json:"use_time" gorm:"default:0"`
	IsStream         bool   `json:"is_stream"`
	ChannelId        int    `json:"channel" gorm:"index"`
	ChannelName      string `json:"channel_name" gorm:"->"`
	TokenId          int    `json:"token_id" gorm:"default:0;index"`
	Group            string `json:"group" gorm:"index"`
	Ip               string `json:"ip" gorm:"index;default:''"`
	// 客户端请求头，原存于 Other JSON，现独立为列以便检索与裁剪。
	// 使用显式 text 类型而非依赖 string+default:''，避免 MySQL 下因带默认值
	// 被生成为 varchar(191)，导致长 UA/Referer 在严格模式下写入失败或被截断。
	Ua                string `json:"ua" gorm:"column:ua;type:text"`
	XTitle            string `json:"x_title" gorm:"column:x_title;type:text"`
	HttpReferer       string `json:"http_referer" gorm:"column:http_referer;type:text"`
	RequestId         string `json:"request_id,omitempty" gorm:"type:varchar(64);index:idx_logs_request_id;default:''"`
	UpstreamRequestId string `json:"upstream_request_id,omitempty" gorm:"type:varchar(128);index:idx_logs_upstream_request_id;default:''"`
	// Other 存用量统计等 JSON 详情，可超长，显式 text 与其余大文本列语义一致。
	Other     string `json:"other" gorm:"type:text"`
	ModelIcon string `json:"model_icon,omitempty" gorm:"-"`
}

// don't use iota, avoid change log type value
const (
	LogTypeUnknown = 0
	LogTypeTopup   = 1
	LogTypeConsume = 2
	LogTypeManage  = 3
	LogTypeSystem  = 4
	LogTypeError   = 5
	LogTypeRefund  = 6
)

func FormatUserLogs(logs []*Log, startIdx int) {
	for i := range logs {
		logs[i].ChannelName = ""
		var otherMap map[string]interface{}
		otherMap, _ = common.StrToMap(logs[i].Other)
		if otherMap != nil {
			// Remove admin-only debug fields.
			delete(otherMap, "admin_info")
			delete(otherMap, "reject_reason")
		}
		logs[i].Other = common.MapToJsonStr(otherMap)
		logs[i].Id = startIdx + i + 1
	}
}

// enrichLogModelIcons 批量填充日志的模型图标字段。
// 优先使用 models 表的 icon，其次使用关联 vendors 表的 icon。
func enrichLogModelIcons(logs []*Log) {
	hasModelName := false
	for _, l := range logs {
		if l.ModelName != "" {
			hasModelName = true
			break
		}
	}
	if !hasModelName {
		return
	}

	var models []vendormetastore.Model
	if err := dbstore.DB.Where("status = ?", 1).Find(&models).Error; err != nil {
		return
	}

	vendorIDs := make(map[int]struct{})
	for _, m := range models {
		if m.VendorID != 0 {
			vendorIDs[m.VendorID] = struct{}{}
		}
	}

	vendorIconMap := make(map[int]string)
	if len(vendorIDs) > 0 {
		ids := make([]int, 0, len(vendorIDs))
		for id := range vendorIDs {
			ids = append(ids, id)
		}
		type vendorRow struct {
			Id   int    `gorm:"column:id"`
			Icon string `gorm:"column:icon"`
		}
		var vendors []vendorRow
		if err := dbstore.DB.Table("vendors").
			Select("id, icon").
			Where("id IN ? AND deleted_at IS NULL", ids).
			Find(&vendors).Error; err == nil {
			for _, v := range vendors {
				if v.Icon != "" {
					vendorIconMap[v.Id] = v.Icon
				}
			}
		}
	}

	for _, l := range logs {
		if l.ModelName == "" {
			continue
		}
		matched := vendormetastore.MatchModelMeta(l.ModelName, models)
		if matched == nil {
			continue
		}
		if matched.Icon != "" {
			l.ModelIcon = matched.Icon
			continue
		}
		if matched.VendorID != 0 {
			if icon, ok := vendorIconMap[matched.VendorID]; ok {
				l.ModelIcon = icon
			}
		}
	}
}

func GetLogByTokenId(tokenId int) (logs []*Log, err error) {
	err = dbstore.LOG_DB.Model(&Log{}).Where("token_id = ?", tokenId).Order("created_at desc, id desc").Limit(common.MaxRecentItems).Find(&logs).Error
	FormatUserLogs(logs, 0)
	enrichLogModelIcons(logs)
	return logs, err
}

// GetLogsByTokenIdParams 封装按 token_id 分页查询日志所需的筛选条件。
// 仅供 TokenAuthReadOnly 只读接口使用：tokenId 由后端从认证上下文强制注入，
// 调用方无法跨 token 查询。
type GetLogsByTokenIdParams struct {
	TokenId        int
	StartTimestamp int64
	EndTimestamp   int64
	ModelName      string
	LogType        int
	StartIdx       int
	Num            int
}

// GetLogsByTokenId 按 token_id 分页查询日志，支持时间段、模型筛选。
// 复用 GetUserLogs 的脱敏/图标填充逻辑，确保与普通用户日志接口返回一致。
func GetLogsByTokenId(params GetLogsByTokenIdParams) (logs []*Log, total int64, err error) {
	if params.TokenId <= 0 {
		return nil, 0, errors.New("token_id is required")
	}

	tx := dbstore.LOG_DB.Where("token_id = ?", params.TokenId)
	if params.LogType != LogTypeUnknown {
		tx = tx.Where("type = ?", params.LogType)
	}
	if params.ModelName != "" {
		modelNamePattern, patternErr := tokenstore.SanitizeLikePattern(params.ModelName)
		if patternErr != nil {
			return nil, 0, patternErr
		}
		tx = tx.Where("model_name LIKE ? ESCAPE '!'", modelNamePattern)
	}
	if params.StartTimestamp != 0 {
		tx = tx.Where("created_at >= ?", params.StartTimestamp)
	}
	if params.EndTimestamp != 0 {
		tx = tx.Where("created_at <= ?", params.EndTimestamp)
	}

	err = tx.Model(&Log{}).Limit(logSearchCountLimit).Count(&total).Error
	if err != nil {
		common.SysError("failed to count logs by token id: " + err.Error())
		return nil, 0, errors.New("查询日志失败")
	}

	err = tx.Order("created_at desc, id desc").Limit(params.Num).Offset(params.StartIdx).Find(&logs).Error
	if err != nil {
		common.SysError("failed to query logs by token id: " + err.Error())
		return nil, 0, errors.New("查询日志失败")
	}

	FormatUserLogs(logs, params.StartIdx)
	enrichLogModelIcons(logs)
	return logs, total, err
}

func RecordErrorLog(c *gin.Context, userId int, channelId int, modelName string, tokenName string, content string, tokenId int, useTimeMs int,
	isStream bool, group string, other map[string]interface{}) {
	contentPreview := common.LocalLogPreview(content)
	logger.LogInfo(c, fmt.Sprintf("record error log: userId=%d, channelId=%d, modelName=%s, tokenName=%s, content=%s", userId, channelId, modelName, tokenName, contentPreview))
	username := c.GetString("username")
	requestId := c.GetString(common.RequestIdKey)
	upstreamRequestId := c.GetString(common.UpstreamRequestIdKey)
	headers := extractClientHeaders(c)
	otherStr := serializeLogOther(other)
	// 记录请求与错误日志的 IP（强制开启，用于滥用追踪）
	log := &Log{
		UserId:            userId,
		Username:          username,
		CreatedAt:         common.GetTimestamp(),
		Type:              LogTypeError,
		Content:           contentPreview,
		PromptTokens:      0,
		CompletionTokens:  0,
		TokenName:         tokenName,
		ModelName:         modelName,
		Quota:             0,
		ChannelId:         channelId,
		TokenId:           tokenId,
		UseTime:           useTimeMs,
		IsStream:          isStream,
		Group:             group,
		Ip:                c.ClientIP(),
		Ua:                headers.ua,
		XTitle:            headers.xTitle,
		HttpReferer:       headers.httpReferer,
		RequestId:         requestId,
		UpstreamRequestId: upstreamRequestId,
		Other:             otherStr,
	}
	err := dbstore.LOG_DB.Create(log).Error
	if err != nil {
		logger.LogError(c, "failed to record log: "+err.Error())
	}
	if common.DataExportEnabled {
		runtime.RelayGo(func() {
			usedatastore.LogQuotaErrorData(userId, username, modelName, common.GetTimestamp())
		})
	}
}

type RecordConsumeLogParams struct {
	ChannelId        int                    `json:"channel_id"`
	PromptTokens     int                    `json:"prompt_tokens"`
	CompletionTokens int                    `json:"completion_tokens"`
	ModelName        string                 `json:"model_name"`
	TokenName        string                 `json:"token_name"`
	Quota            int                    `json:"quota"`
	Content          string                 `json:"content"`
	TokenId          int                    `json:"token_id"`
	UseTimeMs        int                    `json:"use_time_ms"`
	IsStream         bool                   `json:"is_stream"`
	Group            string                 `json:"group"`
	Other            map[string]interface{} `json:"other"`
	LogType          int                    `json:"log_type"` // 日志类型，0 表示使用默认的 LogTypeConsume
}

func RecordConsumeLog(c *gin.Context, userId int, params RecordConsumeLogParams) {
	if !common.LogConsumeEnabled {
		return
	}
	logger.LogInfo(c, fmt.Sprintf("record consume log: userId=%d, params=%s", userId, common.GetJsonString(params)))
	username := c.GetString("username")
	requestId := c.GetString(common.RequestIdKey)
	upstreamRequestId := c.GetString(common.UpstreamRequestIdKey)
	createdAt := common.GetTimestamp()
	clientIP := c.ClientIP()
	headers := extractClientHeaders(c)
	otherStr := serializeLogOther(params.Other)
	logType := params.LogType
	if logType == 0 {
		logType = LogTypeConsume
	}
	// 记录请求与错误日志的 IP（强制开启，用于滥用追踪）
	log := &Log{
		UserId:            userId,
		Username:          username,
		CreatedAt:         createdAt,
		Type:              logType,
		Content:           params.Content,
		PromptTokens:      params.PromptTokens,
		CompletionTokens:  params.CompletionTokens,
		TokenName:         params.TokenName,
		ModelName:         params.ModelName,
		Quota:             params.Quota,
		ChannelId:         params.ChannelId,
		TokenId:           params.TokenId,
		UseTime:           params.UseTimeMs,
		IsStream:          params.IsStream,
		Group:             params.Group,
		Ip:                clientIP,
		Ua:                headers.ua,
		XTitle:            headers.xTitle,
		HttpReferer:       headers.httpReferer,
		RequestId:         requestId,
		UpstreamRequestId: upstreamRequestId,
		Other:             otherStr,
	}
	// 消费日志不影响主流程，异步写入以避免高并发下在请求尾部阻塞数据库。
	runtime.RelayGo(func() {
		err := dbstore.LOG_DB.Create(log).Error
		if err != nil {
			common.SysError(fmt.Sprintf("failed to record consume log (request_id=%s): %s", requestId, err.Error()))
		}
		if common.DataExportEnabled {
			if logType == LogTypeError {
				usedatastore.LogQuotaErrorData(userId, username, params.ModelName, createdAt)
			} else {
				usedatastore.LogQuotaData(userId, username, params.ModelName, params.Quota, createdAt, params.PromptTokens+params.CompletionTokens)
			}
		}
	})
}

// logClientHeaders holds the three client request headers extracted from the
// gin context. They are stored in dedicated Log columns (not the Other JSON).
type logClientHeaders struct {
	ua          string
	xTitle      string
	httpReferer string
}

// extractClientHeaders reads the OpenAI-compatible client request headers
// (User-Agent, X-Title, HTTP-Referer/Referer) from the request context and
// returns single-line sanitized values. The caller writes them to the
// dedicated Log columns.
func extractClientHeaders(c *gin.Context) logClientHeaders {
	if c == nil {
		return logClientHeaders{}
	}
	// Prefer the OpenAI-compatible `HTTP-Referer` header, fall back to standard `Referer`.
	httpReferer := c.GetHeader("HTTP-Referer")
	if strings.TrimSpace(httpReferer) == "" {
		httpReferer = c.GetHeader("Referer")
	}
	ua := c.GetHeader("User-Agent")
	if strings.TrimSpace(ua) == "" && c.Request != nil {
		ua = c.Request.UserAgent()
	}
	return logClientHeaders{
		ua:          sanitizeConsumeLogHeaderValue(ua),
		xTitle:      sanitizeConsumeLogHeaderValue(c.GetHeader("X-Title")),
		httpReferer: sanitizeConsumeLogHeaderValue(httpReferer),
	}
}

func sanitizeConsumeLogHeaderValue(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "-"
	}
	// Keep DB logs single-line to avoid rendering/log injection issues in the UI.
	value = strings.ReplaceAll(value, "\r", " ")
	value = strings.ReplaceAll(value, "\n", " ")
	value = strings.TrimSpace(value)
	if value == "" {
		return "-"
	}
	return value
}

// serializeLogOther normalizes the caller-supplied `other` map to a JSON
// object string. A nil map historically serialized to "{}" (empty object);
// without normalization json.Marshal(nil) produces the literal "null", which
// regresses the Other column semantics and complicates downstream parsing.
// Keep the column storing a JSON object for both nil and empty maps.
func serializeLogOther(other map[string]interface{}) string {
	if other == nil {
		return "{}"
	}
	return common.MapToJsonStr(other)
}

// sanitizeLikeLiteral escapes SQL LIKE wildcards (% and _) and the ESCAPE
// character (!) in a user-provided search term, following the project-wide
// ESCAPE '!' convention (see sanitizeLikePattern in token.go).
// Use this when wrapping user input with surrounding % wildcards for
// substring matching.
func sanitizeLikeLiteral(input string) string {
	input = strings.ReplaceAll(input, "!", "!!")
	input = strings.ReplaceAll(input, "_", "!_")
	input = strings.ReplaceAll(input, "%", "!%")
	return input
}

func GetAllLogs(logType int, startTimestamp int64, endTimestamp int64, modelName string, username string, tokenName string, startIdx int, num int, channel int, group string, requestId string, upstreamRequestId string, ip string, ua string, xTitle string, httpReferer string) (logs []*Log, total int64, err error) {
	var tx *gorm.DB
	if logType == LogTypeUnknown {
		tx = dbstore.LOG_DB
	} else {
		tx = dbstore.LOG_DB.Where("logs.type = ?", logType)
	}

	if modelName != "" {
		tx = tx.Where("logs.model_name like ?", modelName)
	}
	if username != "" {
		tx = tx.Where("logs.username = ?", username)
	}
	if tokenName != "" {
		tx = tx.Where("logs.token_name = ?", tokenName)
	}
	if requestId != "" {
		tx = tx.Where("logs.request_id = ?", requestId)
	}
	if upstreamRequestId != "" {
		tx = tx.Where("logs.upstream_request_id = ?", upstreamRequestId)
	}
	if ip != "" {
		tx = tx.Where("logs.ip LIKE ? ESCAPE '!'", "%"+sanitizeLikeLiteral(ip)+"%")
	}
	if ua != "" {
		tx = tx.Where("logs.ua LIKE ? ESCAPE '!'", "%"+sanitizeLikeLiteral(ua)+"%")
	}
	if xTitle != "" {
		tx = tx.Where("logs.x_title LIKE ? ESCAPE '!'", "%"+sanitizeLikeLiteral(xTitle)+"%")
	}
	if httpReferer != "" {
		tx = tx.Where("logs.http_referer LIKE ? ESCAPE '!'", "%"+sanitizeLikeLiteral(httpReferer)+"%")
	}
	if startTimestamp != 0 {
		tx = tx.Where("logs.created_at >= ?", startTimestamp)
	}
	if endTimestamp != 0 {
		tx = tx.Where("logs.created_at <= ?", endTimestamp)
	}
	if channel != 0 {
		tx = tx.Where("logs.channel_id = ?", channel)
	}
	if group != "" {
		tx = tx.Where("logs."+dbstore.LogGroupCol+" = ?", group)
	}
	err = tx.Model(&Log{}).Count(&total).Error
	if err != nil {
		return nil, 0, err
	}
	err = tx.Order("logs.created_at desc, logs.id desc").Limit(num).Offset(startIdx).Find(&logs).Error
	if err != nil {
		return nil, 0, err
	}

	channelIds := shared.NewSet[int]()
	for _, log := range logs {
		if log.ChannelId != 0 {
			channelIds.Add(log.ChannelId)
		}
	}

	if channelIds.Len() > 0 {
		var channels []struct {
			Id   int    `gorm:"column:id"`
			Name string `gorm:"column:name"`
		}
		if err = dbstore.DB.Table("channels").Select("id, name").Where("id IN ?", channelIds.Items()).Find(&channels).Error; err != nil {
			return logs, total, err
		}
		channelMap := make(map[int]string, len(channels))
		for _, channel := range channels {
			channelMap[channel.Id] = channel.Name
		}
		for i := range logs {
			logs[i].ChannelName = channelMap[logs[i].ChannelId]
		}
	}

	enrichLogModelIcons(logs)

	return logs, total, err
}

const logSearchCountLimit = 10000

func GetUserLogs(userId int, logType int, startTimestamp int64, endTimestamp int64, modelName string, tokenName string, startIdx int, num int, group string, requestId string, upstreamRequestId string, ip string, ua string, xTitle string, httpReferer string) (logs []*Log, total int64, err error) {
	var tx *gorm.DB
	if logType == LogTypeUnknown {
		tx = dbstore.LOG_DB.Where("logs.user_id = ?", userId)
	} else {
		tx = dbstore.LOG_DB.Where("logs.user_id = ? and logs.type = ?", userId, logType)
	}

	if modelName != "" {
		modelNamePattern, err := tokenstore.SanitizeLikePattern(modelName)
		if err != nil {
			return nil, 0, err
		}
		tx = tx.Where("logs.model_name LIKE ? ESCAPE '!'", modelNamePattern)
	}
	if tokenName != "" {
		tx = tx.Where("logs.token_name = ?", tokenName)
	}
	if requestId != "" {
		tx = tx.Where("logs.request_id = ?", requestId)
	}
	if upstreamRequestId != "" {
		tx = tx.Where("logs.upstream_request_id = ?", upstreamRequestId)
	}
	if ip != "" {
		tx = tx.Where("logs.ip LIKE ? ESCAPE '!'", "%"+sanitizeLikeLiteral(ip)+"%")
	}
	if ua != "" {
		tx = tx.Where("logs.ua LIKE ? ESCAPE '!'", "%"+sanitizeLikeLiteral(ua)+"%")
	}
	if xTitle != "" {
		tx = tx.Where("logs.x_title LIKE ? ESCAPE '!'", "%"+sanitizeLikeLiteral(xTitle)+"%")
	}
	if httpReferer != "" {
		tx = tx.Where("logs.http_referer LIKE ? ESCAPE '!'", "%"+sanitizeLikeLiteral(httpReferer)+"%")
	}
	if startTimestamp != 0 {
		tx = tx.Where("logs.created_at >= ?", startTimestamp)
	}
	if endTimestamp != 0 {
		tx = tx.Where("logs.created_at <= ?", endTimestamp)
	}
	if group != "" {
		tx = tx.Where("logs."+dbstore.LogGroupCol+" = ?", group)
	}
	err = tx.Model(&Log{}).Limit(logSearchCountLimit).Count(&total).Error
	if err != nil {
		common.SysError("failed to count user logs: " + err.Error())
		return nil, 0, errors.New("查询日志失败")
	}
	err = tx.Order("logs.created_at desc, logs.id desc").Limit(num).Offset(startIdx).Find(&logs).Error
	if err != nil {
		common.SysError("failed to search user logs: " + err.Error())
		return nil, 0, errors.New("查询日志失败")
	}

	FormatUserLogs(logs, startIdx)
	enrichLogModelIcons(logs)
	return logs, total, err
}

type Stat struct {
	Quota        int `json:"quota"`
	Rpm          int `json:"rpm"`
	Tpm          int `json:"tpm"`
	SuccessCount int `json:"success_count"`
	FailCount    int `json:"fail_count"`
}

// LogStatFilter contains optional filters shared by log list and stat queries.
type LogStatFilter struct {
	Username          string
	TokenName         string
	ModelName         string
	Channel           int
	Group             string
	RequestId         string
	UpstreamRequestId string
	Ip                string
	Ua                string
	XTitle            string
	HttpReferer       string
}

// HasLogOnlyFilters reports whether the filter needs fields unavailable in quota_data.
func (filter LogStatFilter) HasLogOnlyFilters() bool {
	return filter.RequestId != "" || filter.UpstreamRequestId != "" || filter.Ip != "" || filter.Ua != "" || filter.XTitle != "" || filter.HttpReferer != ""
}

// buildStatConditions 构建统计查询的通用 WHERE 条件
func buildStatConditions(tx *gorm.DB, filter LogStatFilter, startTimestamp int64, endTimestamp int64) (*gorm.DB, error) {
	if filter.Username != "" {
		tx = tx.Where("logs.username = ?", filter.Username)
	}
	if filter.TokenName != "" {
		tx = tx.Where("logs.token_name = ?", filter.TokenName)
	}
	if startTimestamp != 0 {
		tx = tx.Where("logs.created_at >= ?", startTimestamp)
	}
	if endTimestamp != 0 {
		tx = tx.Where("logs.created_at <= ?", endTimestamp)
	}
	if filter.ModelName != "" {
		modelNamePattern, err := tokenstore.SanitizeLikePattern(filter.ModelName)
		if err != nil {
			return nil, err
		}
		tx = tx.Where("logs.model_name LIKE ? ESCAPE '!'", modelNamePattern)
	}
	if filter.Channel != 0 {
		tx = tx.Where("logs.channel_id = ?", filter.Channel)
	}
	if filter.Group != "" {
		tx = tx.Where("logs."+dbstore.LogGroupCol+" = ?", filter.Group)
	}
	if filter.RequestId != "" {
		tx = tx.Where("logs.request_id = ?", filter.RequestId)
	}
	if filter.UpstreamRequestId != "" {
		tx = tx.Where("logs.upstream_request_id = ?", filter.UpstreamRequestId)
	}
	if filter.Ip != "" {
		tx = tx.Where("logs.ip LIKE ? ESCAPE '!'", "%"+sanitizeLikeLiteral(filter.Ip)+"%")
	}
	if filter.Ua != "" {
		tx = tx.Where("logs.ua LIKE ? ESCAPE '!'", "%"+sanitizeLikeLiteral(filter.Ua)+"%")
	}
	if filter.XTitle != "" {
		tx = tx.Where("logs.x_title LIKE ? ESCAPE '!'", "%"+sanitizeLikeLiteral(filter.XTitle)+"%")
	}
	if filter.HttpReferer != "" {
		tx = tx.Where("logs.http_referer LIKE ? ESCAPE '!'", "%"+sanitizeLikeLiteral(filter.HttpReferer)+"%")
	}
	return tx, nil
}

// SumUsedQuota returns quota, realtime RPM/TPM, and success/failure counts for logs matching the filter.
func SumUsedQuota(logType int, startTimestamp int64, endTimestamp int64, filter LogStatFilter) (stat Stat, err error) {
	// 额度统计查询
	tx := dbstore.LOG_DB.Table("logs").Select("sum(quota) quota")
	tx, err = buildStatConditions(tx, filter, startTimestamp, endTimestamp)
	if err != nil {
		return stat, err
	}
	if logType != LogTypeUnknown {
		tx = tx.Where("type = ?", logType)
	} else {
		tx = tx.Where("type = ?", LogTypeConsume)
	}

	// rpm和tpm查询（最近60秒）
	rpmTpmQuery := dbstore.LOG_DB.Table("logs").Select("count(*) rpm, sum(prompt_tokens) + sum(completion_tokens) tpm")
	rpmTpmQuery, err = buildStatConditions(rpmTpmQuery, filter, 0, 0)
	if err != nil {
		return stat, err
	}
	rpmTpmQuery = rpmTpmQuery.Where("type = ?", LogTypeConsume)
	rpmTpmQuery = rpmTpmQuery.Where("created_at >= ?", time.Now().Add(-60*time.Second).Unix())

	// 成功次数查询
	successQuery := dbstore.LOG_DB.Table("logs").Select("count(*) success_count")
	successQuery, err = buildStatConditions(successQuery, filter, startTimestamp, endTimestamp)
	if err != nil {
		return stat, err
	}
	if logType != LogTypeUnknown {
		successQuery = successQuery.Where("type = ?", logType)
	} else {
		successQuery = successQuery.Where("type = ?", LogTypeConsume)
	}

	// 失败次数查询
	failQuery := dbstore.LOG_DB.Table("logs").Select("count(*) fail_count")
	failQuery, err = buildStatConditions(failQuery, filter, startTimestamp, endTimestamp)
	if err != nil {
		return stat, err
	}
	failQuery = failQuery.Where("type = ?", LogTypeError)

	var quotaResult struct {
		Quota int `json:"quota"`
	}
	var rpmTpmResult struct {
		Rpm int `json:"rpm"`
		Tpm int `json:"tpm"`
	}
	var successResult struct {
		SuccessCount int `json:"success_count"`
	}
	var failResult struct {
		FailCount int `json:"fail_count"`
	}

	// 执行查询
	if err := tx.Scan(&quotaResult).Error; err != nil {
		common.SysError("failed to query log stat: " + err.Error())
		return stat, errors.New("查询统计数据失败")
	}
	if err := rpmTpmQuery.Scan(&rpmTpmResult).Error; err != nil {
		common.SysError("failed to query rpm/tpm stat: " + err.Error())
		return stat, errors.New("查询统计数据失败")
	}
	if err := successQuery.Scan(&successResult).Error; err != nil {
		common.SysError("failed to query success count stat: " + err.Error())
		return stat, errors.New("查询统计数据失败")
	}
	if err := failQuery.Scan(&failResult).Error; err != nil {
		common.SysError("failed to query fail count stat: " + err.Error())
		return stat, errors.New("查询统计数据失败")
	}

	stat.Quota = quotaResult.Quota
	stat.Rpm = rpmTpmResult.Rpm
	stat.Tpm = rpmTpmResult.Tpm
	stat.SuccessCount = successResult.SuccessCount
	stat.FailCount = failResult.FailCount

	return stat, nil
}

// QueryRpmTpm 实时查询最近60秒的 RPM 和 TPM，供 DataExport 模式复用
func QueryRpmTpm(filter LogStatFilter) (rpm int, tpm int, err error) {
	q := dbstore.LOG_DB.Table("logs").Select("count(*) rpm, sum(prompt_tokens) + sum(completion_tokens) tpm")
	q, buildErr := buildStatConditions(q, filter, 0, 0)
	if buildErr != nil {
		return 0, 0, buildErr
	}
	q = q.Where("type = ?", LogTypeConsume)
	q = q.Where("created_at >= ?", time.Now().Add(-60*time.Second).Unix())

	var result struct {
		Rpm int `json:"rpm"`
		Tpm int `json:"tpm"`
	}
	if err := q.Scan(&result).Error; err != nil {
		common.SysError("failed to query rpm/tpm stat: " + err.Error())
		return 0, 0, errors.New("查询RPM/TPM统计数据失败")
	}
	return result.Rpm, result.Tpm, nil
}

func SumUsedToken(logType int, startTimestamp int64, endTimestamp int64, modelName string, username string, tokenName string) (token int) {
	tx := dbstore.LOG_DB.Table("logs").Select("ifnull(sum(prompt_tokens),0) + ifnull(sum(completion_tokens),0)")
	if username != "" {
		tx = tx.Where("username = ?", username)
	}
	if tokenName != "" {
		tx = tx.Where("token_name = ?", tokenName)
	}
	if startTimestamp != 0 {
		tx = tx.Where("created_at >= ?", startTimestamp)
	}
	if endTimestamp != 0 {
		tx = tx.Where("created_at <= ?", endTimestamp)
	}
	if modelName != "" {
		tx = tx.Where("model_name = ?", modelName)
	}
	tx.Where("type = ?", LogTypeConsume).Scan(&token)
	return token
}

// DeleteOldLog 删除 created_at 早于 targetTimestamp 的日志，分批避免单次事务过大。
// 已被 DeleteLogsInRange 取代，保留用于兼容。
func DeleteOldLog(ctx context.Context, targetTimestamp int64, limit int) (int64, error) {
	return DeleteLogsInRange(ctx, 0, targetTimestamp, limit)
}

// DeleteLogsInRange 删除 created_at 在 [startTimestamp, endTimestamp] 区间内的日志。
// startTimestamp 为 0 表示不限下界，endTimestamp 为 0 表示不限上界。
// 分批删除避免单次事务过大。
func DeleteLogsInRange(ctx context.Context, startTimestamp, endTimestamp int64, limit int) (int64, error) {
	if limit <= 0 {
		limit = 100
	}
	var total int64 = 0

	for {
		if nil != ctx.Err() {
			return total, ctx.Err()
		}

		tx := dbstore.LOG_DB.Where("created_at <= ?", endTimestamp)
		if startTimestamp > 0 {
			tx = tx.Where("created_at >= ?", startTimestamp)
		}
		result := tx.Limit(limit).Delete(&Log{})
		if nil != result.Error {
			return total, result.Error
		}

		total += result.RowsAffected

		if result.RowsAffected < int64(limit) {
			break
		}
	}

	return total, nil
}
