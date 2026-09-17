package middleware

import (
	"bytes"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/NookMux/NookMux/internal/common"
	"github.com/NookMux/NookMux/internal/config/ratio"
	"github.com/NookMux/NookMux/internal/httpapi"
	cachepkg "github.com/NookMux/NookMux/internal/infra/cache"
	"github.com/NookMux/NookMux/pkg/jsonx"
	"github.com/gin-gonic/gin"
)

// newMappingContext 构造带令牌映射与 JSON body 的 gin 测试上下文。
// mapping 为映射 JSON 字符串，按 SetupContextForToken 的方式解析为 map 后写入。
func newMappingContext(t *testing.T, method, path, contentType, body string, mapping string) *gin.Context {
	t.Helper()
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	var reader *strings.Reader
	if body == "" {
		reader = strings.NewReader("")
	} else {
		reader = strings.NewReader(body)
	}
	c.Request = httptest.NewRequest(method, path, reader)
	if contentType != "" {
		c.Request.Header.Set("Content-Type", contentType)
	}
	if mapping != "" {
		modelMap := make(map[string]string)
		if err := jsonx.Unmarshal([]byte(mapping), &modelMap); err != nil {
			t.Fatalf("invalid mapping fixture: %v", err)
		}
		httpapi.SetContextKey(c, common.ContextKeyTokenModelMapping, modelMap)
	}
	return c
}

func TestResolveTokenModelMapping(t *testing.T) {
	cases := []struct {
		name      string
		mapping   map[string]string
		origin    string
		want      string
		wantCycle bool
	}{
		{"单次直接映射", map[string]string{"claude-3-5-sonnet": "glm-4-plus"}, "claude-3-5-sonnet", "glm-4-plus", false},
		{"链式映射 A->B->C", map[string]string{"a": "b", "b": "c"}, "a", "c", false},
		{"起点自引用视为未映射", map[string]string{"a": "a"}, "a", "a", false},
		{"链中自引用停在 B", map[string]string{"a": "b", "b": "b"}, "a", "b", false},
		{"两节点环形报错", map[string]string{"a": "b", "b": "a"}, "a", "", true},
		{"三节点环形报错", map[string]string{"a": "b", "b": "c", "c": "a"}, "a", "", true},
		{"无命中透传", map[string]string{"x": "y"}, "a", "a", false},
		{"目标为空串视为无映射", map[string]string{"a": ""}, "a", "a", false},
		{"链尾目标为空串停在上一跳", map[string]string{"a": "b", "b": ""}, "a", "b", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := resolveTokenModelMapping(tc.mapping, tc.origin)
			if tc.wantCycle {
				if !errors.Is(err, ErrTokenModelMappingCycle) {
					t.Fatalf("err = %v, want ErrTokenModelMappingCycle", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected err: %v", err)
			}
			if got != tc.want {
				t.Fatalf("resolve = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestApplyTokenModelMappingRewritesJsonBody(t *testing.T) {
	body := `{"model":"claude-3-5-sonnet","seed":123456789012345678,"messages":[{"role":"user","content":"hi"}],"temperature":0.7}`
	c := newMappingContext(t, http.MethodPost, "/v1/chat/completions", "application/json", body,
		`{"claude-3-5-sonnet": "glm-4-plus"}`)
	modelRequest := &ModelRequest{Model: "claude-3-5-sonnet"}

	if err := applyTokenModelMapping(c, modelRequest); err != nil {
		t.Fatalf("applyTokenModelMapping: %v", err)
	}

	if modelRequest.Model != "glm-4-plus" {
		t.Fatalf("modelRequest.Model = %q, want glm-4-plus", modelRequest.Model)
	}

	// 后续 UnmarshalBodyReusable 应读到改写后的模型，且其他字段原样保留
	var parsed struct {
		Model       string           `json:"model"`
		Seed        int64            `json:"seed"`
		Messages    []map[string]any `json:"messages"`
		Temperature float64          `json:"temperature"`
	}
	if err := httpapi.UnmarshalBodyReusable(c, &parsed); err != nil {
		t.Fatalf("unmarshal rewritten body: %v", err)
	}
	if parsed.Model != "glm-4-plus" {
		t.Fatalf("rewritten body model = %q, want glm-4-plus", parsed.Model)
	}
	if parsed.Seed != 123456789012345678 {
		t.Fatalf("seed precision lost: got %d", parsed.Seed)
	}
	if len(parsed.Messages) != 1 || parsed.Messages[0]["role"] != "user" {
		t.Fatalf("messages lost after rewrite: %v", parsed.Messages)
	}
	if parsed.Temperature != 0.7 {
		t.Fatalf("temperature lost after rewrite: %v", parsed.Temperature)
	}

	// c.Request.Body 同步重置（Claude 路径 c.ShouldBindJSON 直读 body）
	reqBody, err := httpapi.GetRequestBody(c)
	if err != nil {
		t.Fatalf("get request body: %v", err)
	}
	if !bytes.Contains(reqBody, []byte(`"glm-4-plus"`)) {
		t.Fatalf("request body not rewritten: %s", reqBody)
	}
	if c.Request.ContentLength != int64(len(reqBody)) {
		t.Fatalf("ContentLength = %d, want %d", c.Request.ContentLength, len(reqBody))
	}
}

func TestApplyTokenModelMappingPassthroughWhenNotMatched(t *testing.T) {
	body := `{"model":"gpt-4o","messages":[]}`
	c := newMappingContext(t, http.MethodPost, "/v1/chat/completions", "application/json", body,
		`{"claude-3-5-sonnet": "glm-4-plus"}`)
	modelRequest := &ModelRequest{Model: "gpt-4o"}

	if err := applyTokenModelMapping(c, modelRequest); err != nil {
		t.Fatalf("applyTokenModelMapping: %v", err)
	}
	if modelRequest.Model != "gpt-4o" {
		t.Fatalf("model changed unexpectedly: %q", modelRequest.Model)
	}
	got, err := httpapi.GetRequestBody(c)
	if err != nil {
		t.Fatalf("get request body: %v", err)
	}
	if string(got) != body {
		t.Fatalf("body changed unexpectedly: %s", got)
	}
}

func TestApplyTokenModelMappingSkipsWhenNoMappingConfigured(t *testing.T) {
	c := newMappingContext(t, http.MethodPost, "/v1/chat/completions", "application/json", `{"model":"a"}`, "")
	modelRequest := &ModelRequest{Model: "a"}
	if err := applyTokenModelMapping(c, modelRequest); err != nil {
		t.Fatalf("applyTokenModelMapping: %v", err)
	}
	if modelRequest.Model != "a" {
		t.Fatalf("model changed unexpectedly: %q", modelRequest.Model)
	}
}

func TestApplyTokenModelMappingCycleAborts(t *testing.T) {
	c := newMappingContext(t, http.MethodPost, "/v1/chat/completions", "application/json", `{"model":"a"}`,
		`{"a": "b", "b": "a"}`)
	modelRequest := &ModelRequest{Model: "a"}
	err := applyTokenModelMapping(c, modelRequest)
	if !errors.Is(err, ErrTokenModelMappingCycle) {
		t.Fatalf("err = %v, want ErrTokenModelMappingCycle", err)
	}
}

func TestApplyTokenModelMappingRewritesGeminiPath(t *testing.T) {
	c := newMappingContext(t, http.MethodPost, "/v1beta/models/gemini-2.0-flash:streamGenerateContent?alt=sse",
		"application/json", `{"contents":[]}`, `{"gemini-2.0-flash": "gemini-2.5-pro"}`)
	modelRequest := &ModelRequest{Model: "gemini-2.0-flash"}

	if err := applyTokenModelMapping(c, modelRequest); err != nil {
		t.Fatalf("applyTokenModelMapping: %v", err)
	}
	if modelRequest.Model != "gemini-2.5-pro" {
		t.Fatalf("modelRequest.Model = %q, want gemini-2.5-pro", modelRequest.Model)
	}
	wantPath := "/v1beta/models/gemini-2.5-pro:streamGenerateContent"
	if c.Request.URL.Path != wantPath {
		t.Fatalf("path = %q, want %q", c.Request.URL.Path, wantPath)
	}
	if c.Request.URL.RawQuery != "alt=sse" {
		t.Fatalf("query lost: %q", c.Request.URL.RawQuery)
	}
	// action 检测依赖的 URL.Path 后缀应保留
	if !strings.HasSuffix(c.Request.URL.Path, ":streamGenerateContent") {
		t.Fatal("action suffix lost after rewrite")
	}
}

func TestApplyTokenModelMappingRewritesRealtimeQuery(t *testing.T) {
	c := newMappingContext(t, http.MethodGet, "/v1/realtime?model=gpt-4o-realtime-preview", "", "",
		`{"gpt-4o-realtime-preview": "glm-realtime"}`)
	modelRequest := &ModelRequest{Model: "gpt-4o-realtime-preview"}

	if err := applyTokenModelMapping(c, modelRequest); err != nil {
		t.Fatalf("applyTokenModelMapping: %v", err)
	}
	if modelRequest.Model != "glm-realtime" {
		t.Fatalf("modelRequest.Model = %q, want glm-realtime", modelRequest.Model)
	}
	if c.Request.URL.Query().Get("model") != "glm-realtime" {
		t.Fatalf("query model = %q, want glm-realtime", c.Request.URL.Query().Get("model"))
	}
}

func TestApplyTokenModelMappingRewritesFormBody(t *testing.T) {
	c := newMappingContext(t, http.MethodPost, "/v1/audio/speech",
		"application/x-www-form-urlencoded", "model=tts-1&input=hello&speed=1.5",
		`{"tts-1": "glm-tts"}`)
	modelRequest := &ModelRequest{Model: "tts-1"}

	if err := applyTokenModelMapping(c, modelRequest); err != nil {
		t.Fatalf("applyTokenModelMapping: %v", err)
	}
	if modelRequest.Model != "glm-tts" {
		t.Fatalf("modelRequest.Model = %q, want glm-tts", modelRequest.Model)
	}
	// parseFormData 把表单值均按字符串承载（既有行为），断言字段完整保留
	var parsed struct {
		Model string `json:"model"`
		Input string `json:"input"`
		Speed string `json:"speed"`
	}
	if err := httpapi.UnmarshalBodyReusable(c, &parsed); err != nil {
		t.Fatalf("unmarshal rewritten form: %v", err)
	}
	if parsed.Model != "glm-tts" || parsed.Input != "hello" || parsed.Speed != "1.5" {
		t.Fatalf("rewritten form = %+v", parsed)
	}
}

func TestApplyTokenModelMappingSkipsMultipart(t *testing.T) {
	// multipart 无法安全改写：保持一致性透传（模型与计费都不变）
	c := newMappingContext(t, http.MethodPost, "/v1/audio/transcriptions",
		"multipart/form-data; boundary=xyz", "--xyz\r\nContent-Disposition: form-data; name=\"model\"\r\n\r\nwhisper-1\r\n--xyz--\r\n",
		`{"whisper-1": "glm-audio"}`)
	modelRequest := &ModelRequest{Model: "whisper-1"}

	if err := applyTokenModelMapping(c, modelRequest); err != nil {
		t.Fatalf("applyTokenModelMapping: %v", err)
	}
	if modelRequest.Model != "whisper-1" {
		t.Fatalf("multipart model should stay untouched, got %q", modelRequest.Model)
	}
}

func TestApplyTokenModelMappingSkipsJsonWithoutModelField(t *testing.T) {
	// body 未携带 model（由分发逻辑填充默认值）时不注入字段
	c := newMappingContext(t, http.MethodPost, "/v1/moderations", "application/json", `{"input":"text"}`,
		`{"text-moderation-stable": "glm-moderation"}`)
	modelRequest := &ModelRequest{Model: "text-moderation-stable"}

	if err := applyTokenModelMapping(c, modelRequest); err != nil {
		t.Fatalf("applyTokenModelMapping: %v", err)
	}
	if modelRequest.Model != "text-moderation-stable" {
		t.Fatalf("model changed unexpectedly: %q", modelRequest.Model)
	}
	got, _ := httpapi.GetRequestBody(c)
	if bytes.Contains(got, []byte("glm-moderation")) {
		t.Fatalf("model should not be injected into body: %s", got)
	}
}

// TestApplyTokenModelMappingRewritesDiskBackedBody 验证超过磁盘缓存阈值的
// 大请求体走磁盘存储时改写仍然闭环：SetRequestBody 的磁盘分支（Seek/替换
// storage/重置 Body 与 ContentLength）与 UnmarshalBodyReusable 的磁盘分支兼容。
func TestApplyTokenModelMappingRewritesDiskBackedBody(t *testing.T) {
	oldConfig := cachepkg.GetDiskCacheConfig()
	cachepkg.SetDiskCacheConfig(cachepkg.DiskCacheConfig{
		Enabled:     true,
		ThresholdMB: 1,
		MaxSizeMB:   1024,
		Path:        t.TempDir(),
	})

	// 构造超过 1MB 阈值的 JSON body（长 padding 字段）
	padding := strings.Repeat("x", 1100*1024)
	body := `{"model":"claude-3-5-sonnet","pad":"` + padding + `"}`
	c := newMappingContext(t, http.MethodPost, "/v1/chat/completions", "application/json", body,
		`{"claude-3-5-sonnet": "glm-4-plus"}`)
	t.Cleanup(func() {
		// 关闭磁盘 body storage，否则 TempDir 清理时文件仍被打开
		httpapi.CleanupBodyStorage(c)
		cachepkg.SetDiskCacheConfig(oldConfig)
	})
	modelRequest := &ModelRequest{Model: "claude-3-5-sonnet"}

	if err := applyTokenModelMapping(c, modelRequest); err != nil {
		t.Fatalf("applyTokenModelMapping: %v", err)
	}
	if modelRequest.Model != "glm-4-plus" {
		t.Fatalf("modelRequest.Model = %q, want glm-4-plus", modelRequest.Model)
	}

	// 三通道均应读到改写后的内容
	var parsed struct {
		Model string `json:"model"`
		Pad   string `json:"pad"`
	}
	if err := httpapi.UnmarshalBodyReusable(c, &parsed); err != nil {
		t.Fatalf("unmarshal rewritten disk body: %v", err)
	}
	if parsed.Model != "glm-4-plus" {
		t.Fatalf("rewritten disk body model = %q, want glm-4-plus", parsed.Model)
	}
	if len(parsed.Pad) != len(padding) {
		t.Fatalf("pad length = %d, want %d", len(parsed.Pad), len(padding))
	}
	got, err := httpapi.GetRequestBody(c)
	if err != nil {
		t.Fatalf("get request body: %v", err)
	}
	if !bytes.Contains(got, []byte(`"glm-4-plus"`)) {
		t.Fatal("GetRequestBody does not see rewritten model on disk path")
	}
	if c.Request.ContentLength != int64(len(got)) {
		t.Fatalf("ContentLength = %d, want %d", c.Request.ContentLength, len(got))
	}
	reqBody, err := io.ReadAll(c.Request.Body)
	if err != nil {
		t.Fatalf("read c.Request.Body: %v", err)
	}
	if !bytes.Contains(reqBody, []byte(`"glm-4-plus"`)) {
		t.Fatal("c.Request.Body does not see rewritten model on disk path")
	}
}

// TestGetModelRequestAppliesTokenMapping 端到端验证 distributor 集成点：
// getModelRequest 解析出的模型在返回前已被令牌映射改写，body 同步更新，
// 后续 original_model / 定价 / 渠道调度均以改写后模型为准。
func TestGetModelRequestAppliesTokenMapping(t *testing.T) {
	c := newMappingContext(t, http.MethodPost, "/v1/chat/completions", "application/json",
		`{"model":"claude-opus-4","messages":[{"role":"user","content":"hi"}]}`,
		`{"claude-opus-4": "glm-4-plus"}`)

	modelRequest, _, err := getModelRequest(c)
	if err != nil {
		t.Fatalf("getModelRequest: %v", err)
	}
	if modelRequest.Model != "glm-4-plus" {
		t.Fatalf("modelRequest.Model = %q, want glm-4-plus", modelRequest.Model)
	}
	var parsed struct {
		Model string `json:"model"`
	}
	if err := httpapi.UnmarshalBodyReusable(c, &parsed); err != nil {
		t.Fatalf("unmarshal body: %v", err)
	}
	if parsed.Model != "glm-4-plus" {
		t.Fatalf("body model = %q, want glm-4-plus", parsed.Model)
	}
}

// TestGetModelRequestTokenMappingCycleRejected 验证环形映射在分发层被拦截
// （getModelRequest 返回错误，Distribute 以 400 拒绝请求）。
func TestGetModelRequestTokenMappingCycleRejected(t *testing.T) {
	c := newMappingContext(t, http.MethodPost, "/v1/chat/completions", "application/json",
		`{"model":"a"}`, `{"a": "b", "b": "a"}`)
	if _, _, err := getModelRequest(c); !errors.Is(err, ErrTokenModelMappingCycle) {
		t.Fatalf("err = %v, want ErrTokenModelMappingCycle", err)
	}
}

// TestSetupContextForSelectedChannelUsesMappedModel 验证改写后的模型进入
// original_model 上下文——GenRelayInfo/定价预扣费/日志统计的模型均取自该键，
// 即令牌重定向后全链路按目标模型执行。
func TestSetupContextForSelectedChannelUsesMappedModel(t *testing.T) {
	c := newMappingContext(t, http.MethodPost, "/v1/chat/completions", "application/json",
		`{"model":"claude-opus-4","messages":[]}`, `{"claude-opus-4": "glm-4-plus"}`)

	modelRequest, _, err := getModelRequest(c)
	if err != nil {
		t.Fatalf("getModelRequest: %v", err)
	}
	// channel 为 nil 时返回错误，但 original_model 在校验前已写入
	if setupErr := SetupContextForSelectedChannel(c, nil, modelRequest.Model); setupErr == nil {
		t.Fatal("expected error for nil channel")
	}
	if got := httpapi.GetContextKeyString(c, common.ContextKeyOriginalModel); got != "glm-4-plus" {
		t.Fatalf("original_model = %q, want glm-4-plus", got)
	}
}

// TestGetModelRequestCompactSuffixAppliedAfterMapping 验证 responses/compact
// 的压缩后缀在令牌映射之后套用：映射基于客户端原始模型名查找，
// 重定向目标再携带 compact 后缀（与渠道级 ModelMappedHelper 的分层一致）。
func TestGetModelRequestCompactSuffixAppliedAfterMapping(t *testing.T) {
	c := newMappingContext(t, http.MethodPost, "/v1/responses/compact", "application/json",
		`{"model":"gpt-4o","input":"item"}`, `{"gpt-4o": "glm-4-plus"}`)

	modelRequest, _, err := getModelRequest(c)
	if err != nil {
		t.Fatalf("getModelRequest: %v", err)
	}
	if modelRequest.Model != "glm-4-plus"+ratio.CompactModelSuffix {
		t.Fatalf("modelRequest.Model = %q, want %q", modelRequest.Model, "glm-4-plus"+ratio.CompactModelSuffix)
	}
	var parsed struct {
		Model string `json:"model"`
	}
	if err := httpapi.UnmarshalBodyReusable(c, &parsed); err != nil {
		t.Fatalf("unmarshal body: %v", err)
	}
	// body 携带的是不带后缀的目标模型（后缀只存在于分发上下文模型）
	if parsed.Model != "glm-4-plus" {
		t.Fatalf("body model = %q, want glm-4-plus", parsed.Model)
	}
}
