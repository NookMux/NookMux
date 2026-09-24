package voicestore

import (
	"github.com/NookMux/NookMux/internal/store/channel"
	"github.com/NookMux/NookMux/internal/store/db"
	"gorm.io/gorm"
	"strings"
	"time"
)

// 音色记录的状态/类型常量。
// 这些取值需稳定，前端、控制器、relay 层通过常量引用。
const (
	// VoiceTypePreview 表示“试听中”：仅在定制音色页面可使用，且成功确认定制后转为 Created。
	VoiceTypePreview = "preview"
	// VoiceTypeCreated 表示“已创建”：可用于后续 TTS（受白名单与 allowed 标记约束）。
	VoiceTypeCreated = "created"
)

// Voice 是定制音色的数据库记录。定制音色不绑定单一供应商：MiniMax 为首个支持的上游。
//
// 设计要点：
//   - 音色白名单与重定向不再以系统设置 JSON 存储，而是落到本表。
//   - VoiceID 是用户/对外暴露的音色 ID；RedirectID 是可选的真实上游音色 ID。
//     TTS 请求按 VoiceID 校验白名单，校验通过后用 RedirectID（为空则用 VoiceID）发给上游，
//     避免用户直接传 RedirectID 绕过白名单。
//   - Allowed 控制单条音色是否可用；类型必须为 Created 才允许用于 TTS。
//   - 用户的创建/确认流程写入 OperatorKind=user，管理员新建为 admin。
type Voice struct {
	Id        int64 `json:"id" gorm:"primaryKey;autoIncrement"`
	CreatedAt int64 `json:"created_at" gorm:"bigint;index:idx_voices_created_at"`
	UpdatedAt int64 `json:"updated_at" gorm:"bigint"`
	// Type 状态/类型：preview（试听中）或 created（已创建）。
	Type string `json:"type" gorm:"type:varchar(16);index:idx_voices_type;not null"`
	// OperatorId 操作人 ID：用户创建音色为用户 ID，管理员新建为管理员 ID。
	OperatorId   int    `json:"operator_id" gorm:"index:idx_voices_operator_id"`
	OperatorKind string `json:"operator_kind" gorm:"type:varchar(16)"` // user / admin
	// VoiceId 对外音色 ID（用户使用时的 ID）。
	VoiceId string `json:"voice_id" gorm:"type:varchar(256);column:voice_id;uniqueIndex:uk_voices_voice_id;not null"`
	// QuotaCost 创建该音色时扣减的额度（仅用于审计展示，实际扣费在 service 层完成）。
	QuotaCost int `json:"quota_cost" gorm:"bigint;default:0"`
	// RedirectId 重定向到上游的真实音色 ID；为空则直接使用 VoiceId 发给上游。
	RedirectId string `json:"redirect_id" gorm:"type:varchar(256)"`
	// Allowed 是否允许用于 TTS。Type=created 时才生效；预览中的音色不受此开关约束（仅在定制页面可用）。
	Allowed bool `json:"allowed" gorm:"default:false"`
	// Remark 备注（可选）。
	Remark string `json:"remark" gorm:"type:varchar(255)"`
}

func (Voice) TableName() string {
	return "voices"
}

// InsertVoice 插入一条音色记录，自动填充时间戳。
// 调用方应在此之前完成 VoiceID 查重（IsVoiceIdExists）。
func InsertVoice(voice *Voice) error {
	now := time.Now().Unix()
	if voice.CreatedAt == 0 {
		voice.CreatedAt = now
	}
	voice.UpdatedAt = now
	return dbstore.DB.Create(voice).Error
}

// IsVoiceIdExists 判断给定音色 ID 是否已存在（任意状态）。
func IsVoiceIdExists(voiceId string) (bool, error) {
	voiceId = strings.TrimSpace(voiceId)
	if voiceId == "" {
		return false, nil
	}
	var cnt int64
	err := dbstore.DB.Model(&Voice{}).Where("voice_id = ?", voiceId).Count(&cnt).Error
	return cnt > 0, err
}

// GetVoiceByVoiceId 按对外音色 ID 查询单条记录。
func GetVoiceByVoiceId(voiceId string) (*Voice, error) {
	var voice Voice
	if err := dbstore.DB.Where("voice_id = ?", strings.TrimSpace(voiceId)).First(&voice).Error; err != nil {
		return nil, err
	}
	return &voice, nil
}

// GetVoiceById 按主键查询。
func GetVoiceById(id int64) (*Voice, error) {
	var voice Voice
	if err := dbstore.DB.First(&voice, "id = ?", id).Error; err != nil {
		return nil, err
	}
	return &voice, nil
}

// UpdateVoice 保存音色记录（更新 UpdatedAt）。
func UpdateVoice(voice *Voice) error {
	voice.UpdatedAt = time.Now().Unix()
	return dbstore.DB.Save(voice).Error
}

// UpdateVoiceType 仅更新状态字段（用于确认定制时的 preview -> created 流转）。
// 通过条件更新避免并发覆盖其他字段。
func UpdateVoiceType(id int64, newType string) error {
	return dbstore.DB.Model(&Voice{}).Where("id = ?", id).
		Updates(map[string]interface{}{"type": newType, "updated_at": time.Now().Unix()}).Error
}

// ConfirmVoice 原子地把“试听中”记录流转为“已创建”并写入扣费额度。
// 仅当当前 type=preview 且操作人匹配时更新成功，避免并发/越权覆盖。
// 返回是否更新成功（rowsAffected>0）。
//
// 修复要点：旧实现先 UpdateVoiceType 再 UpdateVoice(voice)，
// 内存里的 voice.Type 仍是 preview，DB.Save 会把 type 覆盖回 preview。
// 这里改为一次性条件更新 type + quota_cost，杜绝状态回滚风险。
func ConfirmVoice(id int64, operatorId int, quotaCost int) (bool, error) {
	updates := map[string]interface{}{
		"type":       VoiceTypeCreated,
		"quota_cost": quotaCost,
		"updated_at": time.Now().Unix(),
	}
	tx := dbstore.DB.Model(&Voice{}).
		Where("id = ? AND type = ?", id, VoiceTypePreview)
	if operatorId > 0 {
		tx = tx.Where("operator_id = ?", operatorId)
	}
	res := tx.Updates(updates)
	return res.RowsAffected > 0, res.Error
}

// DeleteVoiceById 按主键删除。
func DeleteVoiceById(id int64) error {
	return dbstore.DB.Delete(&Voice{}, "id = ?", id).Error
}

// DeleteExpiredVoicePreviews 删除“试听中”且创建时间早于 cutoff 的记录。
//
// 用途：定制音色流程中清理长期未确认的试听记录（默认 7 天）。
// 该删除属于系统自动清理，不经过 controller 的删除路径，因此不写审计日志。
// 返回受影响的行数。
//
// 使用 GORM 条件删除以保持 SQLite/MySQL/PostgreSQL 兼容；DB 未初始化时安全跳过。
func DeleteExpiredVoicePreviews(cutoff int64) (int64, error) {
	if dbstore.DB == nil {
		return 0, nil
	}
	if cutoff <= 0 {
		return 0, nil
	}
	res := dbstore.DB.Where("type = ? AND created_at < ?", VoiceTypePreview, cutoff).
		Delete(&Voice{})
	return res.RowsAffected, res.Error
}

// VoiceListParams 音色管理列表查询参数。
type VoiceListParams struct {
	Type       string // 可选：preview/created
	OperatorId int    // 可选：操作人 ID
	VoiceId    string // 可选：音色 ID 模糊匹配
	StartTime  int64  // 可选：起始时间（秒）
	EndTime    int64  // 可选：结束时间（秒）
	Page       int
	PageSize   int
}

// VoiceListResult 列表查询结果。
type VoiceListResult struct {
	Items    []*Voice `json:"items"`
	Total    int64    `json:"total"`
	Page     int      `json:"page"`
	PageSize int      `json:"page_size"`
}

// ListVoices 分页查询音色记录，支持多条件筛选。
func ListVoices(params VoiceListParams) (*VoiceListResult, error) {
	if params.Page < 1 {
		params.Page = 1
	}
	if params.PageSize < 1 {
		params.PageSize = 10
	}
	if params.PageSize > 100 {
		params.PageSize = 100
	}

	tx := dbstore.DB.Model(&Voice{})
	if params.Type != "" {
		tx = tx.Where("type = ?", params.Type)
	}
	if params.OperatorId > 0 {
		tx = tx.Where("operator_id = ?", params.OperatorId)
	}
	if vid := strings.TrimSpace(params.VoiceId); vid != "" {
		tx = tx.Where("voice_id LIKE ?", "%"+vid+"%")
	}
	if params.StartTime > 0 {
		tx = tx.Where("created_at >= ?", params.StartTime)
	}
	if params.EndTime > 0 {
		tx = tx.Where("created_at <= ?", params.EndTime)
	}

	var total int64
	if err := tx.Count(&total).Error; err != nil {
		return nil, err
	}

	var items []*Voice
	offset := (params.Page - 1) * params.PageSize
	if err := tx.Order("id desc").Limit(params.PageSize).Offset(offset).Find(&items).Error; err != nil {
		return nil, err
	}

	return &VoiceListResult{
		Items:    items,
		Total:    total,
		Page:     params.Page,
		PageSize: params.PageSize,
	}, nil
}

// ResolveVoiceForTTS 根据用户请求的原始音色 ID 查询可用音色。
// 返回:
//   - found: 是否存在该 VoiceID 的记录。
//   - upstreamVoiceId: 发给上游的音色 ID（RedirectId 优先，为空则用 VoiceId）。
//   - allowed: 是否允许用于 TTS（仅当 Type=created 且 Allowed=true 时为 true）。
//
// 该函数是 relay 层校验的唯一入口，确保按原始音色 ID 查库，避免重定向 ID 绕过白名单。
func ResolveVoiceForTTS(voiceId string) (found bool, upstreamVoiceId string, allowed bool, err error) {
	voiceId = strings.TrimSpace(voiceId)
	if voiceId == "" {
		return false, "", false, nil
	}
	// 防御：DB 未初始化（如单元测试）时放行原 ID，避免 nil panic。
	if dbstore.DB == nil {
		return false, voiceId, false, nil
	}
	var voice Voice
	if err := dbstore.DB.Select("voice_id, redirect_id, type, allowed").Where("voice_id = ?", voiceId).First(&voice).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return false, "", false, nil
		}
		return false, "", false, err
	}
	upstream := voice.RedirectId
	if upstream == "" {
		upstream = voice.VoiceId
	}
	allowed = voice.Type == VoiceTypeCreated && voice.Allowed
	return true, upstream, allowed, nil
}

// CountVoices 返回音色总数（用于统计展示）。
func CountVoices() (int64, error) {
	var cnt int64
	if err := dbstore.DB.Model(&Voice{}).Count(&cnt).Error; err != nil {
		return 0, err
	}
	return cnt, nil
}

// GetEnabledMiniMaxChannelForGroup 查找一个启用中的 MiniMax 类型渠道，其所属分组包含指定 group。
// 用于定制音色流程的上游调用（文件上传、voice_clone）。返回的渠道包含 key 等敏感字段。
// group 为空时匹配默认分组。
func GetEnabledMiniMaxChannelForGroup(group string) (*channelstore.Channel, error) {
	query := dbstore.DB.Model(&channelstore.Channel{}).Where("type = ?", 35).Where("status = ?", 1)
	query = channelstore.ApplyChannelGroupFilter(query, group)
	var channel channelstore.Channel
	if err := query.Order("priority desc, id asc").First(&channel).Error; err != nil {
		return nil, err
	}
	return &channel, nil
}
