package handler

import (
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/NookMux/NookMux/internal/infra/security"
	"github.com/NookMux/NookMux/internal/store/db"
	"github.com/NookMux/NookMux/internal/store/stored_media"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func setupStoredMediaHandlerTestDB(t *testing.T) {
	t.Helper()

	oldDB := dbstore.DB
	testDB, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite test db: %v", err)
	}
	if err := testDB.AutoMigrate(&storedmediastore.StoredMedia{}); err != nil {
		t.Fatalf("migrate sqlite test db: %v", err)
	}

	dbstore.DB = testDB
	t.Cleanup(func() {
		if sqlDB, err := testDB.DB(); err == nil {
			_ = sqlDB.Close()
		}
		dbstore.DB = oldDB
	})
}

// serveStoredMedia 通过真实签名 URL 入口回放指定媒体，返回 HTTP 响应。
func serveStoredMedia(t *testing.T, mediaType string, id string) *httptest.ResponseRecorder {
	t.Helper()

	var sigScope string
	var serve func(c *gin.Context)
	switch mediaType {
	case storedmediastore.MediaTypeImage:
		sigScope = "stored_image"
		serve = RelayStoredImage
	case storedmediastore.MediaTypeVideo:
		sigScope = "stored_video"
		serve = RelayStoredVideo
	default:
		t.Fatalf("unknown media type: %q", mediaType)
	}

	exp := time.Now().Add(time.Hour).Unix()
	sig := security.GenerateHMAC(fmt.Sprintf("%s:%s:%d", sigScope, id, exp))

	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Params = gin.Params{{Key: "id", Value: id}}
	c.Request = httptest.NewRequest(http.MethodGet, fmt.Sprintf("/mcp/%s/%s?exp=%d&sig=%s", mediaType, id, exp, sig), nil)

	serve(c)
	return recorder
}

func encodeTestPNG(t *testing.T) []byte {
	t.Helper()

	img := image.NewRGBA(image.Rect(0, 0, 1, 1))
	img.Set(0, 0, color.White)
	buf := &bytes.Buffer{}
	if err := png.Encode(buf, img); err != nil {
		t.Fatalf("encode test png: %v", err)
	}
	return buf.Bytes()
}

// testMP4Bytes 构造一个可被 http.DetectContentType 识别为 video/mp4 的 ftyp box。
func testMP4Bytes() []byte {
	data := make([]byte, 32)
	binary.BigEndian.PutUint32(data[0:4], 24)
	copy(data[4:8], "ftyp")
	copy(data[8:12], "mp42")
	copy(data[16:20], "isom")
	return data
}

func TestRelayStoredMediaSniffsContentTypeAndForcesAttachmentForUnsafePayload(t *testing.T) {
	setupStoredMediaHandlerTestDB(t)

	svgWithXMLDecl := []byte(`<?xml version="1.0" encoding="UTF-8"?><svg xmlns="http://www.w3.org/2000/svg"><script>alert("xss")</script></svg>`)
	bareSVG := []byte("<svg xmlns=\"http://www.w3.org/2000/svg\"><script>alert(\"xss\")</script></svg>")

	tests := []struct {
		name            string
		mediaType       string
		storedMimeType  string
		data            []byte
		wantContentType string
		wantAttachment  bool
	}{
		{
			name:            "svg declared as image is rewritten to sniffed xml type with attachment",
			mediaType:       storedmediastore.MediaTypeImage,
			storedMimeType:  "image/svg+xml",
			data:            svgWithXMLDecl,
			wantContentType: "text/xml; charset=utf-8",
			wantAttachment:  true,
		},
		{
			name:            "bare svg without xml declaration sniffs as plain text with attachment",
			mediaType:       storedmediastore.MediaTypeImage,
			storedMimeType:  "image/svg+xml",
			data:            bareSVG,
			wantContentType: "text/plain; charset=utf-8",
			wantAttachment:  true,
		},
		{
			name:            "png keeps inline rendering with sniffed image type",
			mediaType:       storedmediastore.MediaTypeImage,
			storedMimeType:  "image/png",
			data:            encodeTestPNG(t),
			wantContentType: "image/png",
			wantAttachment:  false,
		},
		{
			name:            "mp4 keeps inline rendering with sniffed video type",
			mediaType:       storedmediastore.MediaTypeVideo,
			storedMimeType:  "video/mp4",
			data:            testMP4Bytes(),
			wantContentType: "video/mp4",
			wantAttachment:  false,
		},
		{
			name:            "unidentifiable video payload is served as octet-stream with attachment",
			mediaType:       storedmediastore.MediaTypeVideo,
			storedMimeType:  "video/quicktime",
			data:            []byte{0x00, 0x01, 0x02, 0x03, 0x04},
			wantContentType: "application/octet-stream",
			wantAttachment:  true,
		},
	}

	for i, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			media := &storedmediastore.StoredMedia{
				UserId:    1,
				MediaType: tt.mediaType,
				MimeType:  tt.storedMimeType,
				SizeBytes: len(tt.data),
				Sha256:    fmt.Sprintf("sha-%d", i),
				Data:      storedmediastore.LargeBlob(tt.data),
			}
			if err := media.Insert(context.Background()); err != nil {
				t.Fatalf("insert stored media: %v", err)
			}

			recorder := serveStoredMedia(t, tt.mediaType, media.Id)

			if recorder.Code != http.StatusOK {
				t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
			}
			if got := recorder.Header().Get("Content-Type"); got != tt.wantContentType {
				t.Fatalf("Content-Type = %q, want %q (stored mime was %q)", got, tt.wantContentType, tt.storedMimeType)
			}
			disposition := recorder.Header().Get("Content-Disposition")
			if tt.wantAttachment && disposition != "attachment" {
				t.Fatalf("Content-Disposition = %q, want attachment", disposition)
			}
			if !tt.wantAttachment && disposition != "" {
				t.Fatalf("Content-Disposition = %q, want empty for inline media", disposition)
			}
			if got := recorder.Body.Bytes(); !bytes.Equal(got, tt.data) {
				t.Fatalf("response body length = %d, want %d", len(got), len(tt.data))
			}
		})
	}
}
