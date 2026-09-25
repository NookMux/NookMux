package storedmediastore

import (
	"context"
	"errors"
	"github.com/NookMux/NookMux/internal/common"
	"github.com/NookMux/NookMux/internal/store/db"
	"strings"
)

// StoredMediaListItem is a lightweight row used by the multimodal file management UI.
// It intentionally does NOT include the binary payload.
type StoredMediaListItem struct {
	MediaType string `json:"media_type" gorm:"column:media_type"` // image | video
	Id        string `json:"id" gorm:"column:id"`
	UserId    int    `json:"user_id" gorm:"column:user_id"`
	ChannelId int    `json:"channel_id" gorm:"column:channel_id"`
	CreatedAt int64  `json:"created_at" gorm:"column:created_at"`
	MimeType  string `json:"mime_type" gorm:"column:mime_type"`
	SizeBytes int    `json:"size_bytes" gorm:"column:size_bytes"`
	Sha256    string `json:"sha256" gorm:"column:sha256"`
}

func GetAllStoredMedia(ctx context.Context, startTimestamp, endTimestamp int64, startIdx, pageSize int) ([]StoredMediaListItem, int64, error) {
	return queryStoredMedia(ctx, 0, startTimestamp, endTimestamp, startIdx, pageSize)
}

func GetUserStoredMedia(ctx context.Context, userId int, startTimestamp, endTimestamp int64, startIdx, pageSize int) ([]StoredMediaListItem, int64, error) {
	if userId <= 0 {
		return nil, 0, errors.New("user_id is required")
	}
	return queryStoredMedia(ctx, userId, startTimestamp, endTimestamp, startIdx, pageSize)
}

func queryStoredMedia(ctx context.Context, userId int, startTimestamp, endTimestamp int64, startIdx, pageSize int) ([]StoredMediaListItem, int64, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if startIdx < 0 {
		startIdx = 0
	}
	if pageSize <= 0 {
		pageSize = 10
	}

	tx := dbstore.DB.WithContext(ctx).Model(&StoredMedia{})
	if userId > 0 {
		tx = tx.Where("user_id = ?", userId)
	}
	if startTimestamp > 0 {
		tx = tx.Where("created_at >= ?", startTimestamp)
	}
	if endTimestamp > 0 {
		tx = tx.Where("created_at <= ?", endTimestamp)
	}

	var total int64
	if err := tx.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var items []StoredMediaListItem
	if err := tx.
		Select("media_type", "id", "user_id", "channel_id", "created_at", "mime_type", "size_bytes", "sha256").
		Order("created_at desc").
		Order("id desc").
		Limit(pageSize).
		Offset(startIdx).
		Find(&items).Error; err != nil {
		return nil, 0, err
	}

	return items, total, nil
}

// MediaConvertStats 图片/视频转URL的统计结果
type MediaConvertStats struct {
	ImageCount int64 `json:"image_count"`
	VideoCount int64 `json:"video_count"`
}

// GetMediaConvertStatsByUserId 按用户和时间范围统计转URL数量
func GetMediaConvertStatsByUserId(userId int, startTime, endTime int64) (MediaConvertStats, error) {
	var stats MediaConvertStats
	err := countMediaConvert(&stats, "user_id = ? AND created_at >= ? AND created_at <= ?", userId, startTime, endTime)
	return stats, err
}

// GetAllMediaConvertStats 按时间范围统计所有用户的转URL数量
func GetAllMediaConvertStats(startTime, endTime int64) (MediaConvertStats, error) {
	var stats MediaConvertStats
	err := countMediaConvert(&stats, "created_at >= ? AND created_at <= ?", startTime, endTime)
	return stats, err
}

func countMediaConvert(stats *MediaConvertStats, cond string, args ...any) error {
	var rows []struct {
		MediaType string
		Cnt       int64
	}
	if err := dbstore.DB.Model(&StoredMedia{}).
		Where(cond, args...).
		Select("media_type, count(1) as cnt").
		Group("media_type").
		Scan(&rows).Error; err != nil {
		return err
	}
	for _, r := range rows {
		switch r.MediaType {
		case MediaTypeImage:
			stats.ImageCount = r.Cnt
		case MediaTypeVideo:
			stats.VideoCount = r.Cnt
		}
	}
	return nil
}

// 媒体类型判别值，对应 media_type 列。
const (
	MediaTypeImage = "image"
	MediaTypeVideo = "video"
)

// ValidMediaType 校验外部输入的 media_type（路径参数、批量请求体）。
func ValidMediaType(mediaType string) bool {
	return mediaType == MediaTypeImage || mediaType == MediaTypeVideo
}

// StoredMedia stores user-provided media bytes for the "multimodal auto convert
// to URL" feature (images and videos). These assets are intended for short-term
// tool access (e.g. image/video understanding MCP), and can be cleaned up via
// the existing log cleanup flow.
type StoredMedia struct {
	Id        string    `json:"id" gorm:"primaryKey;type:varchar(64)"`
	UserId    int       `json:"user_id" gorm:"index:idx_stored_media_user_type_sha,priority:1"`
	MediaType string    `json:"media_type" gorm:"type:varchar(16);index:idx_stored_media_user_type_sha,priority:2"`
	ChannelId int       `json:"channel_id"`
	CreatedAt int64     `json:"created_at" gorm:"bigint;index"`
	MimeType  string    `json:"mime_type" gorm:"type:varchar(255);default:''"`
	SizeBytes int       `json:"size_bytes" gorm:"default:0"`
	Sha256    string    `json:"sha256" gorm:"type:char(64);index:idx_stored_media_user_type_sha,priority:3"`
	Data      LargeBlob `json:"-" gorm:"not null"`
}

func (StoredMedia) TableName() string { return "stored_media" }

func (m *StoredMedia) Insert(ctx context.Context) error {
	if m == nil {
		return errors.New("stored media is nil")
	}
	if !ValidMediaType(m.MediaType) {
		return errors.New("stored media type is invalid: " + m.MediaType)
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if m.Id == "" {
		m.Id = common.GetUUID()
	}
	if m.CreatedAt == 0 {
		m.CreatedAt = common.GetTimestamp()
	}
	return dbstore.DB.WithContext(ctx).Create(m).Error
}

func GetStoredMediaByID(ctx context.Context, id string) (*StoredMedia, error) {
	m, err := getStoredMedia(ctx, id, false)
	if err != nil {
		return nil, err
	}
	return m, nil
}

// GetStoredMediaMetaByID 按 id 查询元数据（不含 data 大字段），用于权限校验与详情展示。
func GetStoredMediaMetaByID(ctx context.Context, id string) (*StoredMedia, error) {
	m, err := getStoredMedia(ctx, id, true)
	if err != nil {
		return nil, err
	}
	return m, nil
}

func getStoredMedia(ctx context.Context, id string, metaOnly bool) (*StoredMedia, error) {
	if id == "" {
		return nil, errors.New("id is required")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	query := dbstore.DB.WithContext(ctx).Model(&StoredMedia{})
	if metaOnly {
		query = query.Select("id", "user_id", "media_type", "channel_id", "created_at", "mime_type", "size_bytes", "sha256")
	}
	var m StoredMedia
	if err := query.Where("id = ?", id).First(&m).Error; err != nil {
		return nil, err
	}
	return &m, nil
}

// GetStoredMediaByUserAndSha 按用户和媒体类型做跨请求去重：同类型同内容复用最早的记录。
func GetStoredMediaByUserAndSha(ctx context.Context, userId int, mediaType string, sha256 string) (*StoredMedia, error) {
	if userId <= 0 {
		return nil, errors.New("user_id is required")
	}
	if !ValidMediaType(mediaType) {
		return nil, errors.New("media type is invalid: " + mediaType)
	}
	sha256 = strings.TrimSpace(sha256)
	if sha256 == "" {
		return nil, errors.New("sha256 is required")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	var m StoredMedia
	if err := dbstore.DB.WithContext(ctx).
		Where("user_id = ? AND media_type = ? AND sha256 = ?", userId, mediaType, sha256).
		Order("created_at asc").
		First(&m).Error; err != nil {
		return nil, err
	}
	return &m, nil
}

func DeleteStoredMediaByIDs(ctx context.Context, ids []string, userId int) (int64, error) {
	if len(ids) == 0 {
		return 0, nil
	}
	if ctx == nil {
		ctx = context.Background()
	}
	db := dbstore.DB.WithContext(ctx).Where("id IN ?", ids)
	if userId > 0 {
		db = db.Where("user_id = ?", userId)
	}
	result := db.Delete(&StoredMedia{})
	return result.RowsAffected, result.Error
}

// DeleteStoredMediaInRange 删除指定类型、created_at 在 [startTimestamp, endTimestamp]
// 区间内的媒体。startTimestamp 为 0 表示不限下界，endTimestamp 为 0 表示不限上界。
// 分批删除避免单次事务过大。
func DeleteStoredMediaInRange(ctx context.Context, mediaType string, startTimestamp, endTimestamp int64, limit int) (int64, error) {
	if !ValidMediaType(mediaType) {
		return 0, errors.New("media type is invalid: " + mediaType)
	}
	if limit <= 0 {
		limit = 100
	}
	var total int64 = 0
	if ctx == nil {
		ctx = context.Background()
	}

	for {
		if ctx.Err() != nil {
			return total, ctx.Err()
		}

		tx := dbstore.DB.WithContext(ctx).Where("media_type = ?", mediaType).Where("created_at <= ?", endTimestamp)
		if startTimestamp > 0 {
			tx = tx.Where("created_at >= ?", startTimestamp)
		}
		result := tx.Limit(limit).Delete(&StoredMedia{})
		if result.Error != nil {
			return total, result.Error
		}

		total += result.RowsAffected

		if result.RowsAffected < int64(limit) {
			break
		}
	}

	return total, nil
}

// EnsureStoredMediaPoolLimit 对指定媒体类型执行存储池字节上限：超出时按
// created_at、id 升序分批删除最旧的记录，直至总字节数回到上限内。
func EnsureStoredMediaPoolLimit(ctx context.Context, mediaType string, maxBytes int64, batchSize int) (int64, error) {
	if !ValidMediaType(mediaType) {
		return 0, errors.New("media type is invalid: " + mediaType)
	}
	if maxBytes <= 0 {
		return 0, nil
	}
	if batchSize <= 0 {
		batchSize = 100
	}
	if ctx == nil {
		ctx = context.Background()
	}

	var deleted int64 = 0

	for {
		if ctx.Err() != nil {
			return deleted, ctx.Err()
		}

		var totalBytes int64
		if err := dbstore.DB.WithContext(ctx).Model(&StoredMedia{}).
			Where("media_type = ?", mediaType).
			Select("coalesce(sum(size_bytes),0)").Scan(&totalBytes).Error; err != nil {
			return deleted, err
		}
		if totalBytes <= maxBytes {
			return deleted, nil
		}

		// Delete oldest media of this type in batches until within limit.
		var oldest []StoredMedia
		if err := dbstore.DB.WithContext(ctx).Model(&StoredMedia{}).
			Where("media_type = ?", mediaType).
			Select("id", "size_bytes").
			Order("created_at asc").
			Order("id asc").
			Limit(batchSize).
			Find(&oldest).Error; err != nil {
			return deleted, err
		}
		if len(oldest) == 0 {
			return deleted, nil
		}

		ids := make([]string, 0, len(oldest))
		for i := range oldest {
			if oldest[i].Id != "" {
				ids = append(ids, oldest[i].Id)
			}
		}
		if len(ids) == 0 {
			return deleted, nil
		}

		result := dbstore.DB.WithContext(ctx).Where("id IN ?", ids).Delete(&StoredMedia{})
		if result.Error != nil {
			return deleted, result.Error
		}
		if result.RowsAffected == 0 {
			return deleted, nil
		}
		deleted += result.RowsAffected
	}
}
