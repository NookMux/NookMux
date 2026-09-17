package tokencontroller

import (
	"fmt"
	"strings"
	"testing"
)

func TestValidateTokenModelMapping(t *testing.T) {
	cases := []struct {
		name    string
		in      *string
		wantErr bool
	}{
		{"nil 合法", nil, false},
		{"空串合法", strp(""), false},
		{"空白串合法", strp("   "), false},
		{"空对象合法", strp("{}"), false},
		{"合法映射", strp(`{"claude-3-5-sonnet": "glm-4-plus"}`), false},
		{"多条映射合法", strp(`{"a": "b", "c": "d"}`), false},
		{"非法 JSON", strp(`{"a": `), true},
		{"JSON 数组拒绝", strp(`["a"]`), true},
		{"嵌套对象值拒绝", strp(`{"a": {"b": "c"}}`), true},
		{"数值目标拒绝", strp(`{"a": 1}`), true},
		{"空键拒绝", strp(`{"": "b"}`), true},
		{"空白键拒绝", strp(`{"  ": "b"}`), true},
		{"空目标拒绝", strp(`{"a": ""}`), true},
		{"空白目标拒绝", strp(`{"a": "  "}`), true},
		{"目标含冒号拒绝（干扰 Gemini action 判定）", strp(`{"a": "b:generateContent"}`), true},
		{"目标含斜杠拒绝（路径分隔符）", strp(`{"a": "b/c"}`), true},
		{"键含冒号拒绝", strp(`{"a:b": "c"}`), true},
		{"目标含空格拒绝", strp(`{"a": "b c"}`), true},
		{"超长目标拒绝", strp(`{"a": "` + strings.Repeat("x", 257) + `"}`), true},
		{"接近上限的目标放行", strp(`{"a": "` + strings.Repeat("x", 256) + `"}`), false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := validateTokenModelMapping(tc.in)
			if tc.wantErr && err == nil {
				t.Fatal("expected error, got nil")
			}
			if !tc.wantErr && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

func TestNormalizeTokenModelMapping(t *testing.T) {
	if got := normalizeTokenModelMapping(nil); got != nil {
		t.Fatalf("nil input should stay nil, got %v", *got)
	}
	if got := normalizeTokenModelMapping(strp("")); got != nil {
		t.Fatalf("empty input should normalize to nil, got %v", *got)
	}
	if got := normalizeTokenModelMapping(strp("  ")); got != nil {
		t.Fatalf("blank input should normalize to nil, got %v", *got)
	}
	if got := normalizeTokenModelMapping(strp(` {"a":"b"} `)); got == nil || *got != `{"a":"b"}` {
		t.Fatalf("valid mapping should be trimmed, got %v", got)
	}
}

// TestValidateTokenModelMappingTotalSizeLimit 键值均合法但总长超过 64KB
// 上限时整体拒绝，防止恶意超大映射在每次请求的解析中放大 CPU 消耗。
func TestValidateTokenModelMappingTotalSizeLimit(t *testing.T) {
	var sb strings.Builder
	sb.WriteByte('{')
	for i := 0; i < 2600; i++ {
		if i > 0 {
			sb.WriteByte(',')
		}
		sb.WriteString(fmt.Sprintf(`"model-%04d": "target-%04d"`, i, i))
	}
	sb.WriteByte('}')
	if len(sb.String()) <= maxTokenModelMappingBytes {
		t.Fatalf("fixture too small: %d bytes", len(sb.String()))
	}
	if err := validateTokenModelMapping(strp(sb.String())); err == nil {
		t.Fatal("expected size limit error, got nil")
	}
}

func strp(s string) *string { return &s }
