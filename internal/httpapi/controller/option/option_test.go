package optioncontroller

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/NookMux/NookMux/internal/common"
	"github.com/gin-gonic/gin"
)

func TestOptionUpdateValueToStringRejectsCompositeValues(t *testing.T) {
	cases := []struct {
		name  string
		value any
		ok    bool
	}{
		{name: "string", value: `{"a":"b"}`, ok: true},
		{name: "bool", value: true, ok: true},
		{name: "number", value: float64(1), ok: true},
		{name: "array", value: []any{"voice-a"}, ok: false},
		{name: "object", value: map[string]any{"a": "b"}, ok: false},
		{name: "nil", value: nil, ok: false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, ok := optionUpdateValueToString(tc.value)
			if ok != tc.ok {
				t.Fatalf("optionUpdateValueToString ok = %v, want %v", ok, tc.ok)
			}
		})
	}
}

// PasswordLoginEnabled/PasswordRegisterEnabled 是布尔功能开关而非 secret，
// GetOptions 必须继续返回它们，否则认证设置页回落到前端默认值。
func TestIsSensitiveOptionKeyExemptsPasswordFeatureToggles(t *testing.T) {
	if isSensitiveOptionKey("PasswordLoginEnabled") {
		t.Fatal("PasswordLoginEnabled is a boolean feature toggle, must not be filtered")
	}
	if isSensitiveOptionKey("PasswordRegisterEnabled") {
		t.Fatal("PasswordRegisterEnabled is a boolean feature toggle, must not be filtered")
	}
	// secret 类 Password 键仍必须被过滤
	for _, key := range []string{"Password", "SMTPPassword", "oauth_password", "PasswordResetSecret"} {
		if !isSensitiveOptionKey(key) {
			t.Fatalf("key %q must be treated as sensitive", key)
		}
	}
}

func TestPricingJsonMapOptionValidation(t *testing.T) {
	if !isPricingJsonMapOptionKey("ModelRatio") {
		t.Fatal("ModelRatio should support JSON map incremental updates")
	}
	if err := validatePricingJsonMapOption("ModelRatio", `{"gpt-4o":2.5,"free":0}`); err != nil {
		t.Fatalf("validate numeric pricing map: %v", err)
	}
	if err := validatePricingJsonMapOption("ModelRatio", `{"gpt-4o":{"bad":true}}`); err == nil {
		t.Fatal("expected non-numeric pricing map value to fail validation")
	}
	if err := validatePricingJsonMapOption("ContextPricing", `{"tiered-model":{"enabled":true,"tiers":[{"name":"default","min_tokens":0,"model_ratio":1,"completion_ratio":2,"cache_ratio":0.5,"create_cache_ratio":1.25,"audio_ratio":1,"audio_completion_ratio":2}]}}`); err != nil {
		t.Fatalf("validate context pricing map: %v", err)
	}
}

func TestMarshalPricingJsonMapOptionPreservesRawValue(t *testing.T) {
	items := map[string]json.RawMessage{
		"free-model": json.RawMessage(`0`),
		"paid-model": json.RawMessage(`0.75`),
	}
	nextValue, err := marshalPricingJsonMapOption("ModelPrice", items)
	if err != nil {
		t.Fatalf("marshal pricing json map option: %v", err)
	}
	if nextValue != `{"free-model":0,"paid-model":0.75}` {
		t.Fatalf("unexpected marshaled value: %s", nextValue)
	}
}

// setOptionMapValueForTest 在 common.OptionMap 中写入指定键值并在测试结束后还原。
func setOptionMapValueForTest(t *testing.T, key, value string) {
	t.Helper()
	common.OptionMapRWMutex.Lock()
	if common.OptionMap == nil {
		common.OptionMap = make(map[string]string)
	}
	old, existed := common.OptionMap[key]
	common.OptionMap[key] = value
	common.OptionMapRWMutex.Unlock()
	t.Cleanup(func() {
		common.OptionMapRWMutex.Lock()
		if existed {
			common.OptionMap[key] = old
		} else {
			delete(common.OptionMap, key)
		}
		common.OptionMapRWMutex.Unlock()
	})
}

// buildNumericJsonMapValue 构造含 n 个键（k01..kNN，数值 0）的 JSON 映射值，
// 键名零填充保证字典序与数值序一致。
func buildNumericJsonMapValue(t *testing.T, n int) string {
	t.Helper()
	entries := make([]string, 0, n)
	for i := 1; i <= n; i++ {
		entries = append(entries, fmt.Sprintf(`"k%02d":0`, i))
	}
	return "{" + strings.Join(entries, ",") + "}"
}

// buildStringJsonMapValue 构造含 n 个键（m01..mNN，字符串值）的 JSON 映射值。
func buildStringJsonMapValue(t *testing.T, n int) string {
	t.Helper()
	entries := make([]string, 0, n)
	for i := 1; i <= n; i++ {
		entries = append(entries, fmt.Sprintf(`"m%02d":"v%02d"`, i, i))
	}
	return "{" + strings.Join(entries, ",") + "}"
}

func performOptionGet(t *testing.T, handler gin.HandlerFunc, target string) *httptest.ResponseRecorder {
	t.Helper()
	gin.SetMode(gin.TestMode)
	resp := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(resp)
	c.Request = httptest.NewRequest(http.MethodGet, target, nil)
	handler(c)
	return resp
}

func decodeOptionPaginationResponse(t *testing.T, resp *httptest.ResponseRecorder, items any) (page, total int) {
	t.Helper()
	if resp.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", resp.Code, resp.Body.String())
	}
	var body struct {
		Success bool `json:"success"`
		Data    struct {
			Items    json.RawMessage `json:"items"`
			Page     int             `json:"page"`
			PageSize int             `json:"page_size"`
			Total    int             `json:"total"`
		} `json:"data"`
	}
	if err := json.Unmarshal(resp.Body.Bytes(), &body); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}
	if !body.Success {
		t.Fatalf("expected success response, got: %s", resp.Body.String())
	}
	if err := json.Unmarshal(body.Data.Items, items); err != nil {
		t.Fatalf("failed to unmarshal items: %v", err)
	}
	return body.Data.Page, body.Data.Total
}

// 回归：页码无上限时 (page-1)*pageSize 的有符号乘法回绕为负的起始下标，
// 触发切片越界 panic。页码必须在相乘前钳制到有效总页数，超界页返回末页数据。
// 这些接口挂 RootAuth，仅根权限可达，但 panic 修复仍需覆盖到处理器本身。
func TestGetOptionJsonMapPaginationClampsPagePricingBranch(t *testing.T) {
	setOptionMapValueForTest(t, "ModelPrice", buildNumericJsonMapValue(t, 25))

	cases := []struct {
		name      string
		page      string
		wantPage  int
		wantTotal int
		wantFirst string
		wantItems int
	}{
		{name: "first page", page: "1", wantPage: 1, wantTotal: 25, wantFirst: "k01", wantItems: 10},
		{name: "middle page", page: "2", wantPage: 2, wantTotal: 25, wantFirst: "k11", wantItems: 10},
		{name: "last page", page: "3", wantPage: 3, wantTotal: 25, wantFirst: "k21", wantItems: 5},
		{name: "beyond total pages clamps to last page", page: "99", wantPage: 3, wantTotal: 25, wantFirst: "k21", wantItems: 5},
		{name: "int64 max page does not panic", page: strconv.FormatInt(int64(1<<63-1), 10), wantPage: 3, wantTotal: 25, wantFirst: "k21", wantItems: 5},
		{name: "wraparound-scale page does not panic", page: "93000000000000000", wantPage: 3, wantTotal: 25, wantFirst: "k21", wantItems: 5},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			resp := performOptionGet(t, GetOptionJsonMap, "/api/option/json_map?key=ModelPrice&page="+tc.page+"&page_size=10")
			var items []optionJsonMapEntry
			page, total := decodeOptionPaginationResponse(t, resp, &items)
			if page != tc.wantPage {
				t.Fatalf("page = %d, want %d", page, tc.wantPage)
			}
			if total != tc.wantTotal {
				t.Fatalf("total = %d, want %d", total, tc.wantTotal)
			}
			if len(items) != tc.wantItems {
				t.Fatalf("items length = %d, want %d", len(items), tc.wantItems)
			}
			if items[0].Key != tc.wantFirst {
				t.Fatalf("first item key = %q, want %q", items[0].Key, tc.wantFirst)
			}
		})
	}
}

func TestGetOptionJsonMapPaginationClampsPageMiniMaxBranch(t *testing.T) {
	setOptionMapValueForTest(t, "minimax.model_redirect", buildStringJsonMapValue(t, 12))

	cases := []struct {
		name      string
		page      string
		wantPage  int
		wantFirst string
		wantItems int
	}{
		{name: "first page", page: "1", wantPage: 1, wantFirst: "m01", wantItems: 10},
		{name: "beyond total pages clamps to last page", page: "99", wantPage: 2, wantFirst: "m11", wantItems: 2},
		{name: "int64 max page does not panic", page: "9223372036854775807", wantPage: 2, wantFirst: "m11", wantItems: 2},
		{name: "wraparound-scale page does not panic", page: "93000000000000000", wantPage: 2, wantFirst: "m11", wantItems: 2},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			resp := performOptionGet(t, GetOptionJsonMap, "/api/option/json_map?key=minimax.model_redirect&page="+tc.page+"&page_size=10")
			var items []optionJsonMapEntry
			page, total := decodeOptionPaginationResponse(t, resp, &items)
			if page != tc.wantPage {
				t.Fatalf("page = %d, want %d", page, tc.wantPage)
			}
			if total != 12 {
				t.Fatalf("total = %d, want 12", total)
			}
			if len(items) != tc.wantItems {
				t.Fatalf("items length = %d, want %d", len(items), tc.wantItems)
			}
			if items[0].Key != tc.wantFirst {
				t.Fatalf("first item key = %q, want %q", items[0].Key, tc.wantFirst)
			}
			if items[0].Value != "v"+tc.wantFirst[1:] {
				t.Fatalf("first item value = %q, want %q", items[0].Value, "v"+tc.wantFirst[1:])
			}
		})
	}
}

// total 为 0 时必须归一为单空页：不除零、不 panic，超界页同样返回空页。
func TestGetOptionJsonMapPaginationEmptyTotal(t *testing.T) {
	setOptionMapValueForTest(t, "ModelPrice", "{}")

	for _, tc := range []struct {
		name string
		page string
	}{
		{name: "first page", page: "1"},
		{name: "int64 max page", page: "9223372036854775807"},
		{name: "wraparound-scale page", page: "93000000000000000"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			resp := performOptionGet(t, GetOptionJsonMap, "/api/option/json_map?key=ModelPrice&page="+tc.page+"&page_size=10")
			var items []optionJsonMapEntry
			page, total := decodeOptionPaginationResponse(t, resp, &items)
			if page != 1 {
				t.Fatalf("page = %d, want 1", page)
			}
			if total != 0 {
				t.Fatalf("total = %d, want 0", total)
			}
			if len(items) != 0 {
				t.Fatalf("items length = %d, want 0", len(items))
			}
		})
	}
}

// GetOptionJsonArray 的数据源已迁移为恒空列表，total 恒为 0：
// 正常页与巨值页都必须返回空页而非 panic。
func TestGetOptionJsonArrayPaginationEmptyTotal(t *testing.T) {
	for _, tc := range []struct {
		name string
		page string
	}{
		{name: "first page", page: "1"},
		{name: "beyond total pages", page: "99"},
		{name: "int64 max page", page: "9223372036854775807"},
		{name: "wraparound-scale page", page: "93000000000000000"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			resp := performOptionGet(t, GetOptionJsonArray, "/api/option/json_array?key=minimax.voice_whitelist&page="+tc.page+"&page_size=10")
			var items []optionJsonArrayEntry
			page, total := decodeOptionPaginationResponse(t, resp, &items)
			if page != 1 {
				t.Fatalf("page = %d, want 1", page)
			}
			if total != 0 {
				t.Fatalf("total = %d, want 0", total)
			}
			if len(items) != 0 {
				t.Fatalf("items length = %d, want 0", len(items))
			}
		})
	}
}
