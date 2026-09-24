package customvoicecontroller

import (
	"errors"
	"github.com/NookMux/NookMux/internal/common"
	audit "github.com/NookMux/NookMux/internal/domain/audit"
	"github.com/NookMux/NookMux/internal/httpapi"
	"github.com/NookMux/NookMux/internal/i18n"
	"github.com/NookMux/NookMux/internal/store/audit"
	"github.com/NookMux/NookMux/internal/store/voice"
	"github.com/NookMux/NookMux/pkg/jsonx"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// 音色类型校验：只允许 preview / created。
func isValidVoiceType(t string) bool {
	return t == voicestore.VoiceTypePreview || t == voicestore.VoiceTypeCreated
}

// voiceListQuery 将查询参数解析为列表查询参数。
func voiceListQuery(c *gin.Context) voicestore.VoiceListParams {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "10"))
	operatorId, _ := strconv.Atoi(c.Query("operator_id"))
	startTime, _ := strconv.ParseInt(c.Query("start_timestamp"), 10, 64)
	endTime, _ := strconv.ParseInt(c.Query("end_timestamp"), 10, 64)
	return voicestore.VoiceListParams{
		Type:       c.Query("type"),
		OperatorId: operatorId,
		VoiceId:    c.Query("voice_id"),
		StartTime:  startTime,
		EndTime:    endTime,
		Page:       page,
		PageSize:   pageSize,
	}
}

// GetVoices 管理员：分页查询音色记录。
func GetVoices(c *gin.Context) {
	params := voiceListQuery(c)
	result, err := voicestore.ListVoices(params)
	if err != nil {
		common.SysError("list voices failed: " + err.Error())
		httpapi.ApiErrorI18n(c, i18n.MsgDatabaseError)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    result,
	})
}

// voiceUpsertRequest 创建/更新音色请求体。
type voiceUpsertRequest struct {
	VoiceId    string `json:"voice_id"`
	Type       string `json:"type"`
	RedirectId string `json:"redirect_id"`
	Allowed    bool   `json:"allowed"`
	Remark     string `json:"remark"`
}

// CreateVoice 管理员：新建音色记录。
// 操作人 ID 记录为当前管理员，OperatorKind=admin。
func CreateVoice(c *gin.Context) {
	var req voiceUpsertRequest
	if err := jsonx.DecodeJson(c.Request.Body, &req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": i18n.T(c, i18n.MsgInvalidParams)})
		return
	}
	req.VoiceId = strings.TrimSpace(req.VoiceId)
	if req.VoiceId == "" {
		httpapi.ApiErrorI18n(c, i18n.MsgVoiceIDRequired)
		return
	}
	if req.Type == "" {
		req.Type = voicestore.VoiceTypeCreated
	}
	if !isValidVoiceType(req.Type) {
		httpapi.ApiErrorI18n(c, i18n.MsgVoiceInvalidType)
		return
	}

	// 查重：已存在则提示不合规（不暴露“重复”）。
	exists, err := voicestore.IsVoiceIdExists(req.VoiceId)
	if err != nil {
		common.SysError("check voice id exists failed: " + err.Error())
		httpapi.ApiErrorI18n(c, i18n.MsgDatabaseError)
		return
	}
	if exists {
		httpapi.ApiErrorI18n(c, i18n.MsgVoiceInvalidID)
		return
	}

	adminId := c.GetInt("id")
	voice := &voicestore.Voice{
		Type:         req.Type,
		OperatorId:   adminId,
		OperatorKind: "admin",
		VoiceId:      req.VoiceId,
		RedirectId:   strings.TrimSpace(req.RedirectId),
		Allowed:      req.Allowed,
		Remark:       req.Remark,
	}
	if err := voicestore.InsertVoice(voice); err != nil {
		// 唯一约束冲突也归一为不合规。
		if isVoiceDupErr(err) {
			httpapi.ApiErrorI18n(c, i18n.MsgVoiceInvalidID)
			return
		}
		common.SysError("insert voice failed: " + err.Error())
		httpapi.ApiErrorI18n(c, i18n.MsgDatabaseError)
		return
	}

	audit.RecordAudit(
		c,
		auditstore.AuditModuleVoice,
		auditstore.AuditActionCreate,
		"创建音色 "+voice.VoiceId,
		nil,
		voiceAuditMap(voice),
	)
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    voice,
	})
}

// UpdateVoice Root：修改音色记录。
func UpdateVoice(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": i18n.T(c, i18n.MsgVoiceInvalidID)})
		return
	}
	var req voiceUpsertRequest
	if err := jsonx.DecodeJson(c.Request.Body, &req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": i18n.T(c, i18n.MsgInvalidParams)})
		return
	}
	if req.Type != "" && !isValidVoiceType(req.Type) {
		httpapi.ApiErrorI18n(c, i18n.MsgVoiceInvalidType)
		return
	}

	before, err := voicestore.GetVoiceById(id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"success": false, "message": i18n.T(c, i18n.MsgVoiceNotFound)})
			return
		}
		common.SysError("get voice by id failed: " + err.Error())
		httpapi.ApiErrorI18n(c, i18n.MsgDatabaseError)
		return
	}

	// 修改音色 ID 时需查重。
	newVoiceId := strings.TrimSpace(req.VoiceId)
	if newVoiceId != "" && newVoiceId != before.VoiceId {
		exists, derr := voicestore.IsVoiceIdExists(newVoiceId)
		if derr != nil {
			common.SysError("check voice id exists failed: " + derr.Error())
			httpapi.ApiErrorI18n(c, i18n.MsgDatabaseError)
			return
		}
		if exists {
			httpapi.ApiErrorI18n(c, i18n.MsgVoiceInvalidID)
			return
		}
		before.VoiceId = newVoiceId
	}
	if req.Type != "" {
		before.Type = req.Type
	}
	before.RedirectId = strings.TrimSpace(req.RedirectId)
	before.Allowed = req.Allowed
	if req.Remark != "" {
		before.Remark = req.Remark
	}
	before.UpdatedAt = time.Now().Unix()
	if err := voicestore.UpdateVoice(before); err != nil {
		if isVoiceDupErr(err) {
			httpapi.ApiErrorI18n(c, i18n.MsgVoiceInvalidID)
			return
		}
		common.SysError("update voice failed: " + err.Error())
		httpapi.ApiErrorI18n(c, i18n.MsgDatabaseError)
		return
	}

	audit.RecordAudit(
		c,
		auditstore.AuditModuleVoice,
		auditstore.AuditActionUpdate,
		"修改音色 "+before.VoiceId,
		nil,
		voiceAuditMap(before),
	)
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    before,
	})
}

// DeleteVoice Root：删除音色记录。
func DeleteVoice(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": i18n.T(c, i18n.MsgVoiceInvalidID)})
		return
	}
	before, err := voicestore.GetVoiceById(id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"success": false, "message": i18n.T(c, i18n.MsgVoiceNotFound)})
			return
		}
		common.SysError("get voice by id failed: " + err.Error())
		httpapi.ApiErrorI18n(c, i18n.MsgDatabaseError)
		return
	}
	if err := voicestore.DeleteVoiceById(id); err != nil {
		common.SysError("delete voice by id failed: " + err.Error())
		httpapi.ApiErrorI18n(c, i18n.MsgDatabaseError)
		return
	}
	audit.RecordAudit(
		c,
		auditstore.AuditModuleVoice,
		auditstore.AuditActionDelete,
		"删除音色 "+before.VoiceId,
		voiceAuditMap(before),
		nil,
	)
	c.JSON(http.StatusOK, gin.H{"success": true, "message": ""})
}

func voiceAuditMap(v *voicestore.Voice) map[string]interface{} {
	return map[string]interface{}{
		"id":            v.Id,
		"voice_id":      v.VoiceId,
		"type":          v.Type,
		"redirect_id":   v.RedirectId,
		"allowed":       v.Allowed,
		"operator_id":   v.OperatorId,
		"operator_kind": v.OperatorKind,
	}
}

func isVoiceDupErr(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "duplicate") || strings.Contains(msg, "unique constraint")
}
