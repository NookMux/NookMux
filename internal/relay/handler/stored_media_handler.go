package handler

import (
	"errors"
	"fmt"
	"github.com/NookMux/NookMux/internal/relay/core"
	"github.com/NookMux/NookMux/internal/store/stored_media"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	"net/http"
	"strings"
)

// RelayStoredImage serves images persisted for "multimodal auto convert to URL".
//
// This is intentionally a lightweight unauthenticated endpoint using signed URLs (sig, optional exp).
func RelayStoredImage(c *gin.Context) {
	relayStoredMedia(c, storedmediastore.MediaTypeImage, "stored_image", "stored_image_not_found", "read_stored_image_failed")
}

// RelayStoredVideo serves videos persisted for "multimodal auto convert to URL".
// This is intentionally a lightweight unauthenticated endpoint using signed URLs (sig, optional exp).
func RelayStoredVideo(c *gin.Context) {
	relayStoredMedia(c, storedmediastore.MediaTypeVideo, "stored_video", "stored_video_not_found", "read_stored_video_failed")
}

func relayStoredMedia(c *gin.Context, mediaType string, sigScope string, notFoundErr string, readFailedErr string) {
	id := strings.TrimSpace(c.Param("id"))
	if id == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "id is required",
		})
		return
	}

	exp, now, ok := core.VerifyStoredAssetSignature(c, sigScope, id)
	if !ok {
		return
	}

	m, err := storedmediastore.GetStoredMediaByID(c.Request.Context(), id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusNotFound, gin.H{
				"error": notFoundErr,
			})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": readFailedErr,
		})
		return
	}
	// 签名 URL 按类型签发，媒体类型不匹配视同未找到，保持分表时期的类型隔离语义。
	if m.MediaType != mediaType {
		c.JSON(http.StatusNotFound, gin.H{
			"error": notFoundErr,
		})
		return
	}

	// 以字节内容重新嗅探 Content-Type，不信任入库时声明的 MimeType：
	// 存量数据可能声明为 image/* 而实际是 SVG 等可承载脚本的内容。
	contentType := http.DetectContentType(m.Data)

	// Basic caching: keep private; if exp is present align to it, otherwise use a fixed max-age.
	maxAge := int64(24 * 60 * 60)
	if exp > 0 {
		maxAge = exp - now
		if maxAge < 0 {
			maxAge = 0
		}
	}
	if maxAge > 24*60*60 {
		maxAge = 24 * 60 * 60
	}
	c.Writer.Header().Set("Cache-Control", fmt.Sprintf("private, max-age=%d", maxAge))
	c.Writer.Header().Set("X-Content-Type-Options", "nosniff")

	// 嗅探结果不是栅格图像也不是视频时（如 SVG 嗅探出的 text/xml、text/plain），
	// 附加 attachment 阻止浏览器同源内联执行；正常栅格图与视频保持内联行为。
	if !isInlineSafeStoredContentType(contentType) {
		c.Writer.Header().Set("Content-Disposition", "attachment")
	}

	c.Data(http.StatusOK, contentType, []byte(m.Data))
}
