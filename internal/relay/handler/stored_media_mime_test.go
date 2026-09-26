package handler

import "testing"

func TestValidateStoredImageMIME(t *testing.T) {
	allowed := []string{
		"image/png",
		"image/jpeg",
		"image/jpg",
		"image/gif",
		"image/webp",
		"image/bmp",
		"image/tiff",
		"image/avif",
		"image/heic",
		"image/heif",
		"IMAGE/WEBP",
	}

	for _, mimeType := range allowed {
		if err := validateStoredImageMIME(mimeType); err != nil {
			t.Fatalf("validateStoredImageMIME(%q) = %v, want nil", mimeType, err)
		}
	}

	rejected := []string{
		"image/svg+xml",
		"IMAGE/SVG+XML",
		"text/html",
		"text/xml",
		"application/octet-stream",
		"image/",
		"",
	}

	for _, mimeType := range rejected {
		if err := validateStoredImageMIME(mimeType); err == nil {
			t.Fatalf("validateStoredImageMIME(%q) = nil, want error", mimeType)
		}
	}
}
