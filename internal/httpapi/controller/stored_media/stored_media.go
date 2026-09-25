package storedmediacontroller

import (
	"errors"
	"fmt"
	"github.com/NookMux/NookMux/internal/common"
	"github.com/NookMux/NookMux/internal/config/system"
	"github.com/NookMux/NookMux/internal/httpapi"
	"github.com/NookMux/NookMux/internal/i18n"
	"github.com/NookMux/NookMux/internal/infra/security"
	"github.com/NookMux/NookMux/internal/store/stored_media"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

type storedMediaListRow struct {
	Id        string `json:"id"`
	MediaType string `json:"media_type"`
	CreatedAt int64  `json:"created_at"`
	MimeType  string `json:"mime_type"`
	SizeBytes int    `json:"size_bytes"`
	Url       string `json:"url"`
}

type storedMediaDetailResponse struct {
	Id        string `json:"id"`
	MediaType string `json:"media_type"`
	CreatedAt int64  `json:"created_at"`
	MimeType  string `json:"mime_type"`
	SizeBytes int    `json:"size_bytes"`
	Url       string `json:"url"`
}

type storedMediaBatchItem struct {
	Id        string `json:"id"`
	MediaType string `json:"media_type"`
}

type storedMediaBatchRequest struct {
	Items []storedMediaBatchItem `json:"items"`
}

const defaultStoredMediaSignedURLTTL = time.Hour

func GetAllStoredMedia(c *gin.Context) {
	pageInfo := common.GetPageQuery(c)
	startTimestamp, _ := strconv.ParseInt(c.Query("start_timestamp"), 10, 64)
	endTimestamp, _ := strconv.ParseInt(c.Query("end_timestamp"), 10, 64)

	items, total, err := storedmediastore.GetAllStoredMedia(c.Request.Context(), startTimestamp, endTimestamp, pageInfo.GetStartIdx(), pageInfo.GetPageSize())
	if err != nil {
		common.SysError("get all stored media failed: " + err.Error())
		httpapi.ApiErrorI18n(c, i18n.MsgDatabaseError)
		return
	}

	rows := make([]storedMediaListRow, 0, len(items))
	for i := range items {
		rows = append(rows, storedMediaListRow{
			Id:        items[i].Id,
			MediaType: items[i].MediaType,
			CreatedAt: items[i].CreatedAt,
			MimeType:  items[i].MimeType,
			SizeBytes: items[i].SizeBytes,
			Url:       buildStoredMediaURL(c, items[i].MediaType, items[i].Id),
		})
	}

	pageInfo.SetTotal(int(total))
	pageInfo.SetItems(rows)
	httpapi.ApiSuccess(c, pageInfo)
}

func GetSelfStoredMedia(c *gin.Context) {
	pageInfo := common.GetPageQuery(c)
	userId := c.GetInt("id")
	startTimestamp, _ := strconv.ParseInt(c.Query("start_timestamp"), 10, 64)
	endTimestamp, _ := strconv.ParseInt(c.Query("end_timestamp"), 10, 64)

	items, total, err := storedmediastore.GetUserStoredMedia(c.Request.Context(), userId, startTimestamp, endTimestamp, pageInfo.GetStartIdx(), pageInfo.GetPageSize())
	if err != nil {
		common.SysError("get user stored media failed: " + err.Error())
		httpapi.ApiErrorI18n(c, i18n.MsgDatabaseError)
		return
	}

	rows := make([]storedMediaListRow, 0, len(items))
	for i := range items {
		rows = append(rows, storedMediaListRow{
			Id:        items[i].Id,
			MediaType: items[i].MediaType,
			CreatedAt: items[i].CreatedAt,
			MimeType:  items[i].MimeType,
			SizeBytes: items[i].SizeBytes,
			Url:       buildStoredMediaURL(c, items[i].MediaType, items[i].Id),
		})
	}

	pageInfo.SetTotal(int(total))
	pageInfo.SetItems(rows)
	httpapi.ApiSuccess(c, pageInfo)
}

// loadStoredMediaMeta 读取指定类型的媒体元数据并完成归属校验。
// 类型不匹配或记录不存在均按未找到处理，与分表时期 /image/:id 查不到视频记录的行为一致。
func loadStoredMediaMeta(c *gin.Context, mediaType string, id string) (*storedmediastore.StoredMedia, bool) {
	meta, err := storedmediastore.GetStoredMediaMetaByID(c.Request.Context(), id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			httpapi.ApiErrorI18n(c, i18n.MsgStoredMediaNotFound)
			return nil, false
		}
		common.SysError("get stored media meta failed: " + err.Error())
		httpapi.ApiErrorI18n(c, i18n.MsgDatabaseError)
		return nil, false
	}
	if meta.MediaType != mediaType {
		httpapi.ApiErrorI18n(c, i18n.MsgStoredMediaNotFound)
		return nil, false
	}

	userId := c.GetInt("id")
	role := c.GetInt("role")
	if role < common.RoleAdminUser && meta.UserId != userId {
		httpapi.ApiErrorI18n(c, i18n.MsgStoredMediaForbidden)
		return nil, false
	}
	return meta, true
}

func parseStoredMediaPathParams(c *gin.Context) (mediaType string, id string, ok bool) {
	mediaType = strings.TrimSpace(strings.ToLower(c.Param("media_type")))
	id = strings.TrimSpace(c.Param("id"))
	if id == "" {
		httpapi.ApiErrorI18n(c, i18n.MsgStoredMediaIDRequired)
		return "", "", false
	}
	if !storedmediastore.ValidMediaType(mediaType) {
		httpapi.ApiErrorI18n(c, i18n.MsgStoredMediaMediaTypeInvalid)
		return "", "", false
	}
	return mediaType, id, true
}

func GetStoredMediaDetail(c *gin.Context) {
	mediaType, id, ok := parseStoredMediaPathParams(c)
	if !ok {
		return
	}

	meta, ok := loadStoredMediaMeta(c, mediaType, id)
	if !ok {
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data": storedMediaDetailResponse{
			Id:        meta.Id,
			MediaType: meta.MediaType,
			CreatedAt: meta.CreatedAt,
			MimeType:  meta.MimeType,
			SizeBytes: meta.SizeBytes,
			Url:       buildStoredMediaURL(c, meta.MediaType, meta.Id),
		},
	})
}

func DeleteStoredMedia(c *gin.Context) {
	mediaType, id, ok := parseStoredMediaPathParams(c)
	if !ok {
		return
	}

	if _, ok := loadStoredMediaMeta(c, mediaType, id); !ok {
		return
	}

	userId := c.GetInt("id")
	role := c.GetInt("role")
	ownerId := userId
	if role >= common.RoleAdminUser {
		ownerId = 0
	}

	deleted, err := storedmediastore.DeleteStoredMediaByIDs(c.Request.Context(), []string{id}, ownerId)
	if err != nil {
		common.SysError("delete stored media failed: " + err.Error())
		httpapi.ApiErrorI18n(c, i18n.MsgDatabaseError)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    deleted,
	})
}

func DeleteStoredMediaBatch(c *gin.Context) {
	req := storedMediaBatchRequest{}
	if err := c.ShouldBindJSON(&req); err != nil || len(req.Items) == 0 {
		httpapi.ApiErrorI18n(c, i18n.MsgInvalidParams)
		return
	}

	userId := c.GetInt("id")
	role := c.GetInt("role")

	ids := make([]string, 0, len(req.Items))
	for i := range req.Items {
		id := strings.TrimSpace(req.Items[i].Id)
		if id == "" {
			continue
		}
		if !storedmediastore.ValidMediaType(strings.TrimSpace(strings.ToLower(req.Items[i].MediaType))) {
			// ignore unknown types
			continue
		}
		ids = append(ids, id)
	}

	if len(ids) == 0 {
		httpapi.ApiErrorI18n(c, i18n.MsgStoredMediaNoValidIDs)
		return
	}

	ownerId := userId
	if role >= common.RoleAdminUser {
		ownerId = 0
	}

	totalDeleted, err := storedmediastore.DeleteStoredMediaByIDs(c.Request.Context(), ids, ownerId)
	if err != nil {
		common.SysError("batch delete stored media failed: " + err.Error())
		httpapi.ApiErrorI18n(c, i18n.MsgDatabaseError)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    totalDeleted,
	})
}

func buildStoredMediaURL(c *gin.Context, mediaType string, id string) string {
	mediaType = strings.TrimSpace(strings.ToLower(mediaType))
	id = strings.TrimSpace(id)
	if id == "" {
		return ""
	}

	var scope string
	switch mediaType {
	case storedmediastore.MediaTypeImage:
		scope = "stored_image"
	case storedmediastore.MediaTypeVideo:
		scope = "stored_video"
	default:
		return ""
	}

	exp := time.Now().Add(defaultStoredMediaSignedURLTTL).Unix()
	sig := security.GenerateHMAC(fmt.Sprintf("%s:%s:%d", scope, id, exp))
	path := fmt.Sprintf("/mcp/%s/%s?exp=%d&sig=%s", mediaType, url.PathEscape(id), exp, sig)

	base := strings.TrimRight(strings.TrimSpace(system.ServerAddress), "/")
	if base == "" {
		base = guessBaseURLFromRequest(c)
	}
	if base == "" {
		return path
	}
	return base + path
}

func guessBaseURLFromRequest(c *gin.Context) string {
	if c == nil || c.Request == nil {
		return ""
	}

	proto := strings.TrimSpace(strings.Split(c.GetHeader("X-Forwarded-Proto"), ",")[0])
	if proto == "" {
		if c.Request.TLS != nil {
			proto = "https"
		} else {
			proto = "http"
		}
	}

	host := strings.TrimSpace(strings.Split(c.GetHeader("X-Forwarded-Host"), ",")[0])
	if host == "" {
		host = c.Request.Host
	}
	host = strings.TrimSpace(host)
	if host == "" {
		return ""
	}

	return proto + "://" + host
}
