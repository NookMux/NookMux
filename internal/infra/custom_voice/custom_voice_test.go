package customvoice

import (
	configmodel "github.com/NookMux/NookMux/internal/config/model"
	"github.com/NookMux/NookMux/internal/config/system"
	"github.com/NookMux/NookMux/internal/i18n"
	"github.com/NookMux/NookMux/internal/store/channel"
	"github.com/NookMux/NookMux/pkg/jsonx"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

// withMiniMaxSettings 临时覆盖全局 MiniMax 设置，函数返回后恢复原值。
// 用于隔离 buildVoiceClonePayload 的策略相关测试。
func withMiniMaxSettings(cfg configmodel.MiniMaxSettings, fn func()) {
	prev := *configmodel.GetMiniMaxSettings()
	defer func() { *configmodel.GetMiniMaxSettings() = prev }()
	*configmodel.GetMiniMaxSettings() = cfg
	fn()
}

// TestBuildVoiceClonePayload_NoPreviewText 无试听文本时只透传基础字段。
func TestBuildVoiceClonePayload_NoPreviewText(t *testing.T) {
	withMiniMaxSettings(configmodel.MiniMaxSettings{Enabled: true}, func() {
		payload := buildVoiceClonePayload("file-1", CustomVoicePreviewRequest{
			Model:   "tts-2-hd",
			VoiceId: "voice-id",
		})
		if payload["file_id"] != "file-1" {
			t.Errorf("file_id = %v, want file-1", payload["file_id"])
		}
		if _, ok := payload["text"]; ok {
			t.Errorf("text should not be set when preview text is empty")
		}
		if _, ok := payload["emotion"]; ok {
			t.Errorf("emotion should not be set when preview text is empty")
		}
		if _, ok := payload["model"]; ok {
			t.Errorf("model should not be set when preview text is empty")
		}
	})
}

// TestBuildVoiceClonePayload_PolicyDisabled 增强未启用时原样透传文本与模型。
func TestBuildVoiceClonePayload_PolicyDisabled(t *testing.T) {
	withMiniMaxSettings(configmodel.MiniMaxSettings{Enabled: false}, func() {
		payload := buildVoiceClonePayload("file-1", CustomVoicePreviewRequest{
			Model:       "tts-2-hd",
			VoiceId:     "voice-id",
			PreviewText: "(happy)你好",
		})
		if payload["text"] != "(happy)你好" {
			t.Errorf("text = %v, want (happy)你好 (passthrough when disabled)", payload["text"])
		}
		if payload["model"] != "tts-2-hd" {
			t.Errorf("model = %v, want tts-2-hd", payload["model"])
		}
		if _, ok := payload["emotion"]; ok {
			t.Errorf("emotion should not be set when disabled")
		}
	})
}

// TestBuildVoiceClonePayload_EmotionRedirect 用户输入源标签 (1)，
// 上游 payload 应使用重定向目标 emotion 值 (2)，并剥离文本中的情绪标签。
func TestBuildVoiceClonePayload_EmotionRedirect(t *testing.T) {
	withMiniMaxSettings(configmodel.MiniMaxSettings{
		Enabled:         true,
		EmotionPattern:  `\(([^()]+)\)`,
		EmotionRedirect: map[string]string{"1": "2"},
	}, func() {
		payload := buildVoiceClonePayload("file-1", CustomVoicePreviewRequest{
			Model:       "tts-2-hd",
			VoiceId:     "voice-id",
			PreviewText: "(1)你好",
		})
		if payload["emotion"] != "2" {
			t.Errorf("emotion = %v, want 2 (redirected value)", payload["emotion"])
		}
		if payload["text"] != "你好" {
			t.Errorf("text = %v, want 你好 (emotion tag stripped)", payload["text"])
		}
	})
}

// TestBuildVoiceClonePayload_ToneWordRedirect 用户输入源标签 (1)，
// 上游 payload 应使用重定向后的语气词值 (2)。
func TestBuildVoiceClonePayload_ToneWordRedirect(t *testing.T) {
	withMiniMaxSettings(configmodel.MiniMaxSettings{
		Enabled:          true,
		ToneWordPattern:  `\(([^()]+)\)`,
		ToneWordRedirect: map[string]string{"1": "2"},
	}, func() {
		payload := buildVoiceClonePayload("file-1", CustomVoicePreviewRequest{
			Model:       "tts-2-hd",
			VoiceId:     "voice-id",
			PreviewText: "前(1)后",
		})
		if payload["text"] != "前(2)后" {
			t.Errorf("text = %v, want 前(2)后 (redirected tone word)", payload["text"])
		}
	})
}

// TestBuildVoiceClonePayload_ModelRedirect 模型重定向表命中时使用上游模型名。
func TestBuildVoiceClonePayload_ModelRedirect(t *testing.T) {
	withMiniMaxSettings(configmodel.MiniMaxSettings{
		Enabled:       true,
		ModelRedirect: map[string]string{"tts-1-hd": "speech-02-hd"},
	}, func() {
		payload := buildVoiceClonePayload("file-1", CustomVoicePreviewRequest{
			Model:       "tts-1-hd",
			VoiceId:     "voice-id",
			PreviewText: "你好",
		})
		if payload["model"] != "speech-02-hd" {
			t.Errorf("model = %v, want speech-02-hd (redirected)", payload["model"])
		}
	})
}

// TestBuildVoiceClonePayload_ModelRedirectWhenEnhanceDisabled ensures custom
// voice always applies the administrator model redirect before calling MiniMax,
// even when the broader TTS enhance switch is disabled.
func TestBuildVoiceClonePayload_ModelRedirectWhenEnhanceDisabled(t *testing.T) {
	withMiniMaxSettings(configmodel.MiniMaxSettings{
		Enabled:       false,
		ModelRedirect: map[string]string{"tts-2-hd": "speech-2.8-hd"},
	}, func() {
		payload := buildVoiceClonePayload("file-1", CustomVoicePreviewRequest{
			Model:       "tts-2-hd",
			VoiceId:     "voice-id",
			PreviewText: "你好",
		})
		if payload["model"] != "speech-2.8-hd" {
			t.Errorf("model = %v, want speech-2.8-hd (redirected with enhance disabled)", payload["model"])
		}
	})
}

// TestCustomVoiceFileID_NumericRoundTrip covers MiniMax upload responses where
// file_id is a JSON number. The frontend still receives a string for display,
// but the upstream voice_clone payload must preserve the numeric JSON type.
func TestCustomVoiceFileID_NumericRoundTrip(t *testing.T) {
	var resp struct {
		File struct {
			FileId customVoiceFileID `json:"file_id"`
		} `json:"file"`
	}
	if err := jsonx.Unmarshal([]byte(`{"file":{"file_id":123456789}}`), &resp); err != nil {
		t.Fatalf("unmarshal numeric file_id failed: %v", err)
	}
	if got := resp.File.FileId.String(); got != "123456789" {
		t.Fatalf("display file_id = %q, want 123456789", got)
	}

	payload := buildVoiceClonePayload(resp.File.FileId, CustomVoicePreviewRequest{VoiceId: "voice-id"})
	body, err := jsonx.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal voice_clone payload failed: %v", err)
	}
	if !strings.Contains(string(body), `"file_id":123456789`) {
		t.Fatalf("payload should preserve numeric file_id, got %s", string(body))
	}
}

// TestResolveCustomVoiceGroupRatio 验证分组倍率解析包含动态倍率叠加，
// 且未配置动态倍率时返回纯分组倍率。
func TestResolveCustomVoiceGroupRatio(t *testing.T) {
	// "default" 分组默认倍率为 1.0。
	if r := resolveCustomVoiceGroupRatio("default", "tts-3-turbo"); r != 1.0 {
		t.Errorf("default group ratio = %v, want 1.0", r)
	}
	// 未配置分组时也返回默认 1.0（GetGroupRatio 找不到时回退 1）。
	if r := resolveCustomVoiceGroupRatio("nonexistent-group", "tts-3-turbo"); r != 1.0 {
		t.Errorf("nonexistent group ratio = %v, want 1.0", r)
	}
}

// overrideCustomVoiceSSRFSettingForTest 允许测试用 httptest 本地地址出站：
// 关闭 SSRF 防护（仅测试进程内生效，结束恢复）。
func overrideCustomVoiceSSRFSettingForTest(t *testing.T) {
	t.Helper()
	setting := system.GetFetchSetting()
	old := *setting
	t.Cleanup(func() { *setting = old })
	setting.EnableSSRFProtection = false
}

// TestCloneVoiceUpstreamEncodesGroupIdQuery 验证上游 voice_clone 请求的
// GroupId 查询串经过 URL 编码：groupId 含 &、=、# 等保留字符时必须整体作为
// GroupId 的值传输，而不是被解析成额外的 query 参数或 fragment。
// 修复前 url += "?GroupId=" + up.groupId 未编码，`a&b=c#d` 会污染上游请求。
func TestCloneVoiceUpstreamEncodesGroupIdQuery(t *testing.T) {
	overrideCustomVoiceSSRFSettingForTest(t)

	var gotQuery url.Values
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotQuery = r.URL.Query()
		_, _ = w.Write([]byte(`{"demo_audio":"https://example.com/a.mp3","base_resp":{"status_code":0}}`))
	}))
	defer server.Close()

	up := &minimaxUpstream{
		baseURL: server.URL,
		apiKey:  "test-key",
		groupId: "a&b=c#d",
		channel: &channelstore.Channel{},
	}

	demoAudio, err := cloneVoiceUpstream(i18n.DefaultLang, up, customVoiceFileID{Display: "1"}, CustomVoicePreviewRequest{
		Model:   "speech-02-hd",
		VoiceId: "voice-id",
	})
	if err != nil {
		t.Fatalf("cloneVoiceUpstream failed: %v", err)
	}
	if demoAudio != "https://example.com/a.mp3" {
		t.Fatalf("unexpected demo_audio: %s", demoAudio)
	}
	if got := gotQuery.Get("GroupId"); got != "a&b=c#d" {
		t.Fatalf("GroupId should be URL-encoded and round-trip intact, got: %q", got)
	}
	// 编码后的 `&`/`=` 属于 GroupId 值，不得产生额外参数
	if len(gotQuery) != 1 {
		t.Fatalf("expected exactly one query param, got: %v", gotQuery)
	}
}

// TestCloneVoiceUpstreamNoGroupIdNoQuery 验证 groupId 为空时不追加查询串。
func TestCloneVoiceUpstreamNoGroupIdNoQuery(t *testing.T) {
	overrideCustomVoiceSSRFSettingForTest(t)

	var gotRawQuery string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotRawQuery = r.URL.RawQuery
		_, _ = w.Write([]byte(`{"demo_audio":"https://example.com/a.mp3","base_resp":{"status_code":0}}`))
	}))
	defer server.Close()

	up := &minimaxUpstream{
		baseURL: server.URL,
		apiKey:  "test-key",
		groupId: "",
		channel: &channelstore.Channel{},
	}

	if _, err := cloneVoiceUpstream(i18n.DefaultLang, up, customVoiceFileID{Display: "1"}, CustomVoicePreviewRequest{
		Model:   "speech-02-hd",
		VoiceId: "voice-id",
	}); err != nil {
		t.Fatalf("cloneVoiceUpstream failed: %v", err)
	}
	if gotRawQuery != "" {
		t.Fatalf("expected no query string when groupId empty, got: %q", gotRawQuery)
	}
}

// TestBuildGroupIdQuery 直接断言编码输出格式。
func TestBuildGroupIdQuery(t *testing.T) {
	if got := buildGroupIdQuery("a&b=c#d"); got != "GroupId=a%26b%3Dc%23d" {
		t.Fatalf("unexpected encoded query: %q", got)
	}
	if got := buildGroupIdQuery("plain"); got != "GroupId=plain" {
		t.Fatalf("unexpected encoded query: %q", got)
	}
}
