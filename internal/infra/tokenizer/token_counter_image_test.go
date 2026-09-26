package tokenizer

import (
	"encoding/base64"
	"encoding/binary"
	"image"
	"image/color"
	"math"
	"net/http/httptest"
	"testing"

	"github.com/NookMux/NookMux/internal/domain/shared"
	"github.com/gin-gonic/gin"
)

// enableMediaTokenFlags 开启图片 token 统计所需的全局开关，测试结束后还原。
func enableMediaTokenFlags(t *testing.T) {
	t.Helper()

	oldGetMediaToken := shared.GetMediaToken
	oldGetMediaTokenNotStream := shared.GetMediaTokenNotStream
	shared.GetMediaToken = true
	shared.GetMediaTokenNotStream = true
	t.Cleanup(func() {
		shared.GetMediaToken = oldGetMediaToken
		shared.GetMediaTokenNotStream = oldGetMediaTokenNotStream
	})
}

func newImageTokenContext(t *testing.T) *gin.Context {
	t.Helper()

	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	return c
}

// newCachedImageSource 构造携带预置图片尺寸缓存的 base64 来源，
// 用于绕过文件解析、直接驱动 getImageToken 的尺寸计算分支。
func newCachedImageSource(width int, height int) *shared.FileSource {
	source := &shared.FileSource{Type: shared.FileSourceTypeBase64}
	source.SetCache(&shared.CachedFileData{
		ImageConfig: &image.Config{
			Width:      width,
			Height:     height,
			ColorModel: color.RGBAModel,
		},
		ImageFormat: "png",
	})
	return source
}

// buildHEICFile 构造仅含 ftyp 与 meta(ispe) 盒子的最小 HEIC 文件，
// ispe 中声明指定的宽高。
func buildHEICFile(width uint32, height uint32) []byte {
	ispePayload := make([]byte, 12)
	binary.BigEndian.PutUint32(ispePayload[4:8], width)
	binary.BigEndian.PutUint32(ispePayload[8:12], height)
	ispe := makeTestBox("ispe", ispePayload)
	ipco := makeTestBox("ipco", ispe)
	iprp := makeTestBox("iprp", ipco)
	metaPayload := append([]byte{0, 0, 0, 0}, iprp...)
	meta := makeTestBox("meta", metaPayload)

	ftypPayload := append([]byte("heic"), 0, 0, 0, 0)
	ftypPayload = append(ftypPayload, []byte("heic")...)

	return append(makeTestBox("ftyp", ftypPayload), meta...)
}

func makeTestBox(boxType string, payload []byte) []byte {
	box := make([]byte, 8+len(payload))
	binary.BigEndian.PutUint32(box[0:4], uint32(len(box)))
	copy(box[4:8], boxType)
	copy(box[8:], payload)
	return box
}

// TestGetImageTokenRejectsOversizedHEIFDimensions 验证入口层拒绝：
// HEIC 头部声明 uint32 极限宽高时，图片解析未命中，计费链路直接报错而非拿到非法尺寸。
func TestGetImageTokenRejectsOversizedHEIFDimensions(t *testing.T) {
	enableMediaTokenFlags(t)
	c := newImageTokenContext(t)

	data := buildHEICFile(4294967295, 4294967295)
	source := shared.NewBase64FileSource(base64.StdEncoding.EncodeToString(data), "image/heic")
	fileMeta := shared.NewImageFileMeta(source, "high")

	token, err := getImageToken(c, fileMeta, "gpt-4.1-mini", false)
	if err == nil {
		t.Fatalf("getImageToken accepted oversized HEIC dimensions, token = %d", token)
	}
	if token != 0 {
		t.Fatalf("token = %d on rejection, want 0", token)
	}
}

// TestGetImageTokenClampsOversizedPatchArea 验证计算层纵深防御：
// 即使超大尺寸进入计算（uint32 极限值），面积在 float64 域计算且结果钳为非负，
// 不产生修复前 int 回绕导致的负 token / NaN。
func TestGetImageTokenClampsOversizedPatchArea(t *testing.T) {
	enableMediaTokenFlags(t)
	c := newImageTokenContext(t)

	source := newCachedImageSource(math.MaxUint32, math.MaxUint32)
	fileMeta := shared.NewImageFileMeta(source, "high")

	token, err := getImageToken(c, fileMeta, "gpt-4.1-mini", false)
	if err != nil {
		t.Fatalf("getImageToken error: %v", err)
	}
	if token < 0 {
		t.Fatalf("patch-based token = %d, want non-negative", token)
	}
	if token > int(math.Round(1536*1.62)) {
		t.Fatalf("patch-based token = %d, exceeds 1536-patch cap", token)
	}
}

// TestGetImageTokenOversizedTileBasedStaysNonNegative 验证 tile 类模型
// 在超大尺寸下同样不产生负值。
func TestGetImageTokenOversizedTileBasedStaysNonNegative(t *testing.T) {
	enableMediaTokenFlags(t)
	c := newImageTokenContext(t)

	source := newCachedImageSource(math.MaxUint32, math.MaxUint32)
	fileMeta := shared.NewImageFileMeta(source, "high")

	token, err := getImageToken(c, fileMeta, "gpt-4o", false)
	if err != nil {
		t.Fatalf("getImageToken error: %v", err)
	}
	if token < 0 {
		t.Fatalf("tile-based token = %d, want non-negative", token)
	}
}

// TestGetImageTokenNormalDimensionsUnchanged 验证正常尺寸的计费结果与既有语义一致。
func TestGetImageTokenNormalDimensionsUnchanged(t *testing.T) {
	enableMediaTokenFlags(t)
	c := newImageTokenContext(t)

	cases := []struct {
		name   string
		model  string
		width  int
		height int
		want   int
	}{
		{name: "patch-based-below-cap", model: "gpt-4.1-mini", width: 1024, height: 1024, want: 1659},
		{name: "patch-based-at-cap", model: "gpt-4.1-mini", width: 1536, height: 1024, want: 2488},
		{name: "tile-based-4o", model: "gpt-4o", width: 1024, height: 1024, want: 765},
		{name: "tile-based-low-detail", model: "gpt-4o", width: 4096, height: 4096, want: 85},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			fileMeta := shared.NewImageFileMeta(newCachedImageSource(tc.width, tc.height), "high")
			if tc.name == "tile-based-low-detail" {
				fileMeta = shared.NewImageFileMeta(newCachedImageSource(tc.width, tc.height), "low")
			}
			token, err := getImageToken(c, fileMeta, tc.model, false)
			if err != nil {
				t.Fatalf("getImageToken error: %v", err)
			}
			if token != tc.want {
				t.Fatalf("token = %d, want %d", token, tc.want)
			}
		})
	}
}
