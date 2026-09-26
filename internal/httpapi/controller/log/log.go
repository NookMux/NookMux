package logcontroller

import (
	"github.com/NookMux/NookMux/internal/common"
	"github.com/NookMux/NookMux/internal/config/console"
	audit "github.com/NookMux/NookMux/internal/domain/audit"
	"github.com/NookMux/NookMux/internal/httpapi"
	"github.com/NookMux/NookMux/internal/i18n"
	"github.com/NookMux/NookMux/internal/store/audit"
	"github.com/NookMux/NookMux/internal/store/log"
	"github.com/NookMux/NookMux/internal/store/stored_media"
	"github.com/NookMux/NookMux/internal/store/usedata"
	"github.com/gin-gonic/gin"
	"net/http"
	"strconv"
)

func GetAllLogs(c *gin.Context) {
	pageInfo := common.GetPageQuery(c)
	logType, _ := strconv.Atoi(c.Query("type"))
	startTimestamp, _ := strconv.ParseInt(c.Query("start_timestamp"), 10, 64)
	endTimestamp, _ := strconv.ParseInt(c.Query("end_timestamp"), 10, 64)
	username := c.Query("username")
	tokenName := c.Query("token_name")
	modelName := c.Query("model_name")
	channel, _ := strconv.Atoi(c.Query("channel"))
	group := c.Query("group")
	requestId := c.Query("request_id")
	upstreamRequestId := c.Query("upstream_request_id")
	ip := c.Query("ip")
	ua := c.Query("ua")
	xTitle := c.Query("x_title")
	httpReferer := c.Query("http_referer")
	logs, total, err := logstore.GetAllLogs(logType, startTimestamp, endTimestamp, modelName, username, tokenName, pageInfo.GetStartIdx(), pageInfo.GetPageSize(), channel, group, requestId, upstreamRequestId, ip, ua, xTitle, httpReferer)
	if err != nil {
		common.SysError("failed to get all logs: " + err.Error())
		httpapi.ApiErrorI18n(c, i18n.MsgDatabaseError)
		return
	}
	pageInfo.SetTotal(int(total))
	pageInfo.SetItems(logs)
	httpapi.ApiSuccess(c, pageInfo)
}

func GetUserLogs(c *gin.Context) {
	pageInfo := common.GetPageQuery(c)
	userId := c.GetInt("id")
	logType, _ := strconv.Atoi(c.Query("type"))
	startTimestamp, _ := strconv.ParseInt(c.Query("start_timestamp"), 10, 64)
	endTimestamp, _ := strconv.ParseInt(c.Query("end_timestamp"), 10, 64)
	tokenName := c.Query("token_name")
	modelName := c.Query("model_name")
	group := c.Query("group")
	requestId := c.Query("request_id")
	upstreamRequestId := c.Query("upstream_request_id")
	ip := c.Query("ip")
	ua := c.Query("ua")
	xTitle := c.Query("x_title")
	httpReferer := c.Query("http_referer")

	// 普通用户列表接口同样禁止使用不可见字段做过滤条件，
	// 避免通过 total/items 变化做侧信道探测（与 GetLogsSelfStat 一致）。
	filter := logstore.LogStatFilter{
		Username:          c.GetString("username"),
		TokenName:         tokenName,
		ModelName:         modelName,
		Group:             group,
		RequestId:         requestId,
		UpstreamRequestId: upstreamRequestId,
		Ip:                ip,
		Ua:                ua,
		XTitle:            xTitle,
		HttpReferer:       httpReferer,
	}
	if msg := validateUserLogFilters(filter); msg != "" {
		c.JSON(200, gin.H{
			"success": false,
			"message": msg,
		})
		return
	}

	logs, total, err := logstore.GetUserLogs(userId, logType, startTimestamp, endTimestamp, modelName, tokenName, pageInfo.GetStartIdx(), pageInfo.GetPageSize(), group, requestId, upstreamRequestId, ip, ua, xTitle, httpReferer)
	if err != nil {
		common.SysError("failed to get user logs: " + err.Error())
		httpapi.ApiErrorI18n(c, i18n.MsgDatabaseError)
		return
	}
	filterHiddenUsageLogFields(logs)
	pageInfo.SetTotal(int(total))
	pageInfo.SetItems(logs)
	httpapi.ApiSuccess(c, pageInfo)
}

// filterHiddenUsageLogFields 根据使用日志字段可见性配置，清空普通用户不可见的详情弹窗独有字段数据。
//
// 后端安全裁剪范围（此处处理的字段，均为详情弹窗独有、不在列表表格列中）：
//   - 顶层独立字段：request_id、upstream_request_id、ip。
//   - other JSON 内字段：通过 stripHiddenOtherFields 处理。
//
// 不在裁剪范围的字段（同时出现在列表表格列中，始终对普通用户可见，不可配置隐藏）：
//   - content、token_name、group、use_time、channel/channel_name。
//   - 这些字段的产品语义是"表格列字段"，返回值不裁剪，过滤条件不校验。
//   - 详见根 AGENTS.md "使用日志字段可见性" 章节。
//
// admin_info 相关字段（topup_audit/operator_admin/retry_chain）已由 logstore.formatUserLogs 删除。
// stream_status/billing_source/request_conversion 等独立 other 字段在 stripHiddenOtherFields 中过滤。
// 如果配置解析失败，IsUsageLogFieldVisible 会回退到默认值，此处按默认值过滤。
func filterHiddenUsageLogFields(logs []*logstore.Log) {
	// 构建需要过滤的字段集合（普通用户不可见的详情弹窗独有字段）
	hiddenFields := make(map[string]bool)
	for _, d := range console.UsageLogFieldsDefaults() {
		if !console.IsUsageLogFieldVisible(d.Key, false) {
			hiddenFields[d.Key] = true
		}
	}

	totalSwitchOff := !console.IsUsageLogDetailsEnabled(false)

	for _, log := range logs {
		if totalSwitchOff {
			// 总开关关闭，清空所有详情弹窗独有字段
			log.RequestId = ""
			log.UpstreamRequestId = ""
			log.Ip = ""
			log.Ua = ""
			log.XTitle = ""
			log.HttpReferer = ""
			stripHiddenOtherFields(log, nil)
			continue
		}

		// 详情弹窗独有的顶层字段
		if hiddenFields[console.UsageLogFieldRequestID] {
			log.RequestId = ""
		}
		if hiddenFields[console.UsageLogFieldUpstreamRequestID] {
			log.UpstreamRequestId = ""
		}
		if hiddenFields[console.UsageLogFieldIPAddress] {
			log.Ip = ""
		}
		// 客户端请求头已独立为顶层列，按 client_headers 开关裁剪
		if hiddenFields[console.UsageLogFieldClientHeaders] {
			log.Ua = ""
			log.XTitle = ""
			log.HttpReferer = ""
		}
		// other JSON 内的字段
		stripHiddenOtherFields(log, hiddenFields)
	}
}

// stripHiddenOtherFields 从 other JSON 中移除被隐藏字段对应的数据。
// 如果 hiddenFields 为 nil，表示清空所有详情弹窗独有的 other 字段。
func stripHiddenOtherFields(log *logstore.Log, hiddenFields map[string]bool) {
	if log.Other == "" {
		return
	}
	otherMap, err := common.StrToMap(log.Other)
	if err != nil || otherMap == nil {
		return
	}

	clearAll := hiddenFields == nil
	if clearAll {
		// 清空所有详情弹窗独有的 other 字段
		otherKeys := []string{
			"http_referer", "x_title", "ua",
			"reasoning_effort",
			"is_system_prompt_overwritten",
			"is_model_mapped", "upstream_model_name",
			"po",
			"request_path", "request_conversion",
			"billing_mode", "expr_b64", "matched_tier",
			"billing_source",
			"violation_fee_code", "violation_fee_marker", "fee_quota",
			"task_id", "reason",
			"subscription_plan_id", "subscription_plan_title",
			"subscription_id", "subscription_pre_consumed",
			"subscription_post_delta", "subscription_consumed",
			"subscription_remain", "subscription_total",
			"stream_status",
			"audio_input", "audio_output", "audio_input_token_count",
			"text_input", "text_output",
		}
		for _, k := range otherKeys {
			delete(otherMap, k)
		}
	} else {
		// 按字段配置选择性清空
		if hiddenFields[console.UsageLogFieldClientHeaders] {
			delete(otherMap, "http_referer")
			delete(otherMap, "x_title")
			delete(otherMap, "ua")
		}
		if hiddenFields[console.UsageLogFieldReasoningEffort] {
			delete(otherMap, "reasoning_effort")
		}
		if hiddenFields[console.UsageLogFieldSystemPrompt] {
			delete(otherMap, "is_system_prompt_overwritten")
		}
		if hiddenFields[console.UsageLogFieldModelMapping] {
			delete(otherMap, "is_model_mapped")
			delete(otherMap, "upstream_model_name")
		}
		if hiddenFields[console.UsageLogFieldParameterOverride] {
			delete(otherMap, "po")
		}
		if hiddenFields[console.UsageLogFieldRequestConversion] {
			delete(otherMap, "request_path")
			delete(otherMap, "request_conversion")
		}
		if hiddenFields[console.UsageLogFieldBillingSource] {
			delete(otherMap, "billing_source")
		}
		if hiddenFields[console.UsageLogFieldTieredPricing] {
			delete(otherMap, "billing_mode")
			delete(otherMap, "expr_b64")
			delete(otherMap, "matched_tier")
		}
		if hiddenFields[console.UsageLogFieldViolationFee] {
			delete(otherMap, "violation_fee_code")
			delete(otherMap, "violation_fee_marker")
			delete(otherMap, "fee_quota")
		}
		if hiddenFields[console.UsageLogFieldRefundDetails] {
			delete(otherMap, "task_id")
			delete(otherMap, "reason")
		}
		if hiddenFields[console.UsageLogFieldSubscriptionBilling] {
			delete(otherMap, "subscription_plan_id")
			delete(otherMap, "subscription_plan_title")
			delete(otherMap, "subscription_id")
			delete(otherMap, "subscription_pre_consumed")
			delete(otherMap, "subscription_post_delta")
			delete(otherMap, "subscription_consumed")
			delete(otherMap, "subscription_remain")
			delete(otherMap, "subscription_total")
		}
		if hiddenFields[console.UsageLogFieldStreamStatus] {
			delete(otherMap, "stream_status")
		}
		if hiddenFields[console.UsageLogFieldAudioTokens] {
			delete(otherMap, "audio_input")
			delete(otherMap, "audio_output")
			delete(otherMap, "audio_input_token_count")
			delete(otherMap, "text_input")
			delete(otherMap, "text_output")
		}
	}

	log.Other = common.MapToJsonStr(otherMap)
}

func GetLogByKey(c *gin.Context) {
	tokenId := c.GetInt("token_id")
	if tokenId == 0 {
		httpapi.ApiErrorI18n(c, i18n.MsgInvalidToken)
		return
	}

	// 当未传分页参数时，保留旧的"近期日志数组"响应（兼容老前端）。
	// 传入 p / page_size 任意一个即切换到分页响应。
	rawPage := c.Query("p")
	rawPageSize := c.Query("page_size")
	usePagination := rawPage != "" || rawPageSize != ""

	if !usePagination {
		logs, err := logstore.GetLogByTokenId(tokenId)
		if err != nil {
			common.SysError("failed to get log by token id: " + err.Error())
			httpapi.ApiErrorI18n(c, i18n.MsgDatabaseError)
			return
		}
		// 与 GetUserLogs 共用脱敏入口，按使用日志字段可见性配置裁剪普通用户不可见的字段。
		// GetLogByKey 经 TokenAuthReadOnly 认证，访问者为持有该 token 的普通用户。
		filterHiddenUsageLogFields(logs)
		c.JSON(200, gin.H{
			"success": true,
			"message": "",
			"data":    logs,
		})
		return
	}

	// 分页 + 筛选模式：tokenId 由后端从认证上下文强制注入，调用方无法跨 token 查询。
	pageInfo := common.GetPageQuery(c)
	logType, _ := strconv.Atoi(c.Query("type"))
	startTimestamp, _ := strconv.ParseInt(c.Query("start_timestamp"), 10, 64)
	endTimestamp, _ := strconv.ParseInt(c.Query("end_timestamp"), 10, 64)
	modelName := c.Query("model_name")

	logs, total, err := logstore.GetLogsByTokenId(logstore.GetLogsByTokenIdParams{
		TokenId:        tokenId,
		StartTimestamp: startTimestamp,
		EndTimestamp:   endTimestamp,
		ModelName:      modelName,
		LogType:        logType,
		StartIdx:       pageInfo.GetStartIdx(),
		Num:            pageInfo.GetPageSize(),
	})
	if err != nil {
		common.SysError("failed to get logs by token id: " + err.Error())
		httpapi.ApiErrorI18n(c, i18n.MsgDatabaseError)
		return
	}
	filterHiddenUsageLogFields(logs)
	pageInfo.SetTotal(int(total))
	pageInfo.SetItems(logs)
	httpapi.ApiSuccess(c, pageInfo)
}

func GetLogsStat(c *gin.Context) {
	logType, _ := strconv.Atoi(c.Query("type"))
	startTimestamp, _ := strconv.ParseInt(c.Query("start_timestamp"), 10, 64)
	endTimestamp, _ := strconv.ParseInt(c.Query("end_timestamp"), 10, 64)
	channel, _ := strconv.Atoi(c.Query("channel"))
	filter := logstore.LogStatFilter{
		Username:          c.Query("username"),
		TokenName:         c.Query("token_name"),
		ModelName:         c.Query("model_name"),
		Channel:           channel,
		Group:             c.Query("group"),
		RequestId:         c.Query("request_id"),
		UpstreamRequestId: c.Query("upstream_request_id"),
		Ip:                c.Query("ip"),
		Ua:                c.Query("ua"),
		XTitle:            c.Query("x_title"),
		HttpReferer:       c.Query("http_referer"),
	}

	var statData logstore.Stat
	if common.DataExportEnabled && filter.TokenName == "" && filter.Channel == 0 && filter.Group == "" && filter.ModelName == "" && !filter.HasLogOnlyFilters() && logType == 0 {
		var qStat usedatastore.QuotaStat
		var err error
		if filter.Username != "" {
			qStat, err = usedatastore.GetQuotaStatByUsername(filter.Username, startTimestamp, endTimestamp)
		} else {
			qStat, err = usedatastore.GetAllQuotaStat(startTimestamp, endTimestamp)
		}
		if err != nil {
			common.SysError("failed to get quota stat: " + err.Error())
			httpapi.ApiErrorI18n(c, i18n.MsgDatabaseError)
			return
		}
		// 从 logs 表实时查询 RPM/TPM（最近60秒），quota_data 是小时级预聚合无法提供实时指标
		rpm, tpm, err := logstore.QueryRpmTpm(filter)
		if err != nil {
			common.SysError("failed to query rpm tpm: " + err.Error())
			httpapi.ApiErrorI18n(c, i18n.MsgDatabaseError)
			return
		}
		statData = logstore.Stat{
			Quota:        qStat.Quota,
			Rpm:          rpm,
			Tpm:          tpm,
			SuccessCount: qStat.SuccessCount,
			FailCount:    qStat.FailCount,
		}
	} else {
		stat, err := logstore.SumUsedQuota(logType, startTimestamp, endTimestamp, filter)
		if err != nil {
			common.SysError("failed to sum used quota: " + err.Error())
			httpapi.ApiErrorI18n(c, i18n.MsgDatabaseError)
			return
		}
		statData = stat
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    statData,
	})
}

func GetLogsSelfStat(c *gin.Context) {
	userId := c.GetInt("id")
	logType, _ := strconv.Atoi(c.Query("type"))
	startTimestamp, _ := strconv.ParseInt(c.Query("start_timestamp"), 10, 64)
	endTimestamp, _ := strconv.ParseInt(c.Query("end_timestamp"), 10, 64)
	channel, _ := strconv.Atoi(c.Query("channel"))
	filter := logstore.LogStatFilter{
		Username:          c.GetString("username"),
		TokenName:         c.Query("token_name"),
		ModelName:         c.Query("model_name"),
		Channel:           channel,
		Group:             c.Query("group"),
		RequestId:         c.Query("request_id"),
		UpstreamRequestId: c.Query("upstream_request_id"),
		Ip:                c.Query("ip"),
		Ua:                c.Query("ua"),
		XTitle:            c.Query("x_title"),
		HttpReferer:       c.Query("http_referer"),
	}

	// 普通用户统计接口禁止使用不可见字段做过滤条件，避免通过统计结果变化做侧信道探测。
	if msg := validateUserLogFilters(filter); msg != "" {
		c.JSON(200, gin.H{
			"success": false,
			"message": msg,
		})
		return
	}

	var statData logstore.Stat
	if common.DataExportEnabled && filter.TokenName == "" && filter.Channel == 0 && filter.Group == "" && filter.ModelName == "" && !filter.HasLogOnlyFilters() && logType == 0 {
		qStat, err := usedatastore.GetQuotaStatByUserId(userId, startTimestamp, endTimestamp)
		if err != nil {
			common.SysError("failed to get quota stat by user id: " + err.Error())
			httpapi.ApiErrorI18n(c, i18n.MsgDatabaseError)
			return
		}
		// 从 logs 表实时查询 RPM/TPM（最近60秒）
		rpm, tpm, err := logstore.QueryRpmTpm(filter)
		if err != nil {
			common.SysError("failed to query rpm tpm: " + err.Error())
			httpapi.ApiErrorI18n(c, i18n.MsgDatabaseError)
			return
		}
		statData = logstore.Stat{
			Quota:        qStat.Quota,
			Rpm:          rpm,
			Tpm:          tpm,
			SuccessCount: qStat.SuccessCount,
			FailCount:    qStat.FailCount,
		}
	} else {
		stat, err := logstore.SumUsedQuota(logType, startTimestamp, endTimestamp, filter)
		if err != nil {
			common.SysError("failed to sum used quota: " + err.Error())
			httpapi.ApiErrorI18n(c, i18n.MsgDatabaseError)
			return
		}
		statData = stat
	}

	c.JSON(200, gin.H{
		"success": true,
		"message": "",
		"data":    statData,
	})
}

// validateUserLogFilters 校验普通用户日志接口的过滤条件是否使用了不可见字段。
// 返回非空字符串表示校验失败，内容为给用户的错误信息；空字符串表示通过。
// 同时用于 GetUserLogs（列表接口）和 GetLogsSelfStat（统计接口），
// 封堵通过 total/items/stat 结果变化对隐藏字段做侧信道探测。
//
// 字段语义说明（详见根 AGENTS.md "使用日志字段可见性"）：
//   - 表格列字段（token_name/group/content/use_time）不在此校验：始终可过滤。
//   - channel 虽是表格列，但普通用户不应按 channel 过滤，这是硬安全规则，
//     不依赖配置（channel 已从 Defaults 移除，IsUsageLogFieldVisible 回退到 return false）。
//   - 详情弹窗独有字段（request_id/upstream_request_id/ip/ua/x_title/http_referer）
//     受总开关和单字段可见性双重控制。
func validateUserLogFilters(filter logstore.LogStatFilter) string {
	// 详情弹窗独有字段受总开关和单字段可见性双重控制。
	// 总开关关闭时，详情弹窗独有字段都不可作为过滤条件。
	// 但表格列字段（token_name/group/model_name 等）不受此限制，始终可过滤。
	detailsDisabled := !console.IsUsageLogDetailsEnabled(false)

	type filterFieldCheck struct {
		value    string
		fieldKey string
		fieldMsg string
	}
	// filter 字段 → UsageLogField 常量 → 中文展示名
	checks := []filterFieldCheck{
		{strconv.Itoa(filter.Channel), console.UsageLogFieldChannel, "channel"},
		{filter.RequestId, console.UsageLogFieldRequestID, "request_id"},
		{filter.UpstreamRequestId, console.UsageLogFieldUpstreamRequestID, "upstream_request_id"},
		{filter.Ip, console.UsageLogFieldIPAddress, "ip"},
		{filter.Ua, console.UsageLogFieldClientHeaders, "ua"},
		{filter.XTitle, console.UsageLogFieldClientHeaders, "x_title"},
		{filter.HttpReferer, console.UsageLogFieldClientHeaders, "http_referer"},
	}
	for _, chk := range checks {
		// channel=0 表示未传，跳过；其余空值同样跳过。
		if chk.value == "" || (chk.fieldKey == console.UsageLogFieldChannel && chk.value == "0") {
			continue
		}
		// 总开关关闭时，所有详情弹窗独有字段都不可过滤
		if detailsDisabled {
			return "使用日志详情已禁用，不支持按详情字段过滤"
		}
		if !console.IsUsageLogFieldVisible(chk.fieldKey, false) {
			return "无权使用此过滤条件: " + chk.fieldMsg
		}
	}
	return ""
}

func DeleteHistoryLogs(c *gin.Context) {
	startTimestamp, _ := strconv.ParseInt(c.Query("start_timestamp"), 10, 64)
	endTimestamp, _ := strconv.ParseInt(c.Query("end_timestamp"), 10, 64)
	cleanLogs := c.Query("clean_logs") == "true" || c.Query("clean_logs") == "1"
	cleanStoredImages := c.Query("clean_stored_images") == "true" || c.Query("clean_stored_images") == "1"
	cleanStoredVideos := c.Query("clean_stored_videos") == "true" || c.Query("clean_stored_videos") == "1"
	cleanAuditLogs := c.Query("clean_audit_logs") == "true" || c.Query("clean_audit_logs") == "1"

	if endTimestamp == 0 {
		httpapi.ApiErrorI18n(c, i18n.MsgInvalidParams)
		return
	}
	if !cleanLogs && !cleanStoredImages && !cleanStoredVideos && !cleanAuditLogs {
		httpapi.ApiErrorI18n(c, i18n.MsgInvalidParams)
		return
	}

	data := gin.H{}
	// detail 同时作为审计 after 数据，记录每个已选类型的实际删除数量，
	// 便于事后核对清理范围。
	detail := map[string]interface{}{
		"start_timestamp": startTimestamp,
		"end_timestamp":   endTimestamp,
	}
	// 链式执行：任一类型失败记录首个错误，继续执行其余类型，
	// 确保部分成功的清理也有审计记录。
	var firstErr error

	if cleanLogs {
		count, err := logstore.DeleteLogsInRange(c.Request.Context(), startTimestamp, endTimestamp, 100)
		if err != nil && firstErr == nil {
			firstErr = err
		}
		data["logs"] = count
		detail["clean_logs"] = count
	}
	if cleanStoredImages {
		count, err := storedmediastore.DeleteStoredMediaInRange(c.Request.Context(), storedmediastore.MediaTypeImage, startTimestamp, endTimestamp, 100)
		if err != nil && firstErr == nil {
			firstErr = err
		}
		data["stored_images"] = count
		detail["clean_stored_images"] = count
	}
	if cleanStoredVideos {
		count, err := storedmediastore.DeleteStoredMediaInRange(c.Request.Context(), storedmediastore.MediaTypeVideo, startTimestamp, endTimestamp, 100)
		if err != nil && firstErr == nil {
			firstErr = err
		}
		data["stored_videos"] = count
		detail["clean_stored_videos"] = count
	}
	if cleanAuditLogs {
		count, err := auditstore.DeleteAuditLogsInRange(startTimestamp, endTimestamp)
		if err != nil && firstErr == nil {
			firstErr = err
		}
		data["audit_logs"] = count
		detail["clean_audit_logs"] = count
	}

	// 清理日志属于关键运维操作，使用 forceRecord=true 强制审计，
	// 即使审计总开关或 log 模块被关闭也必须记录，避免审计链断裂。
	if firstErr != nil {
		detail["error"] = firstErr.Error()
	}
	audit.RecordAudit(c, auditstore.AuditModuleLog, auditstore.AuditActionDelete, "清理历史日志", nil, detail, true)

	if firstErr != nil {
		common.SysError("failed to delete history logs: " + firstErr.Error())
		httpapi.ApiErrorI18n(c, i18n.MsgDatabaseError)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    data,
	})
}
