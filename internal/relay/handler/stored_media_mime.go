package handler

import (
	"fmt"
	"strings"
)

// rasterImageMIMEWhitelist 是存储媒体图片入库允许的栅格图像 MIME 白名单。
// image/svg+xml 等可承载脚本的图像类型不在白名单内，从入库入口阻断存储型 XSS 内容。
var rasterImageMIMEWhitelist = map[string]struct{}{
	"image/png":  {},
	"image/jpeg": {},
	"image/jpg":  {},
	"image/gif":  {},
	"image/webp": {},
	"image/bmp":  {},
	"image/tiff": {},
	"image/avif": {},
	"image/heic": {},
	"image/heif": {},
}

// isRasterImageMIME 判断 MIME 是否命中栅格图像白名单，大小写不敏感。
func isRasterImageMIME(mimeType string) bool {
	_, ok := rasterImageMIMEWhitelist[strings.ToLower(strings.TrimSpace(mimeType))]
	return ok
}

// validateStoredImageMIME 校验图片入库声明的 MIME，必须命中栅格图像白名单。
func validateStoredImageMIME(mimeType string) error {
	if !isRasterImageMIME(mimeType) {
		return fmt.Errorf("unsupported image mime type: %q, only raster image mime types are allowed", mimeType)
	}
	return nil
}

// isInlineSafeStoredContentType 判断回放嗅探结果是否可以内联输出：
// 栅格图像白名单命中或 video/* 类型；其余嗅探结果（SVG 嗅探出的
// text/xml、text/plain，以及 application/octet-stream 等）一律以
// attachment 下发，阻止浏览器同源内联执行。
func isInlineSafeStoredContentType(contentType string) bool {
	return isRasterImageMIME(contentType) || strings.HasPrefix(contentType, "video/")
}
