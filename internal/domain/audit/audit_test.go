package audit

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/NookMux/NookMux/pkg/jsonx"
)

// TestSerializeAuditDiff_OptionUpdateBeforeAndAfter 验证修复 issue #101：
// 修改系统设置时，before 必须能被记录，而不是始终为 nil。
//
// 该用例复现 controller.UpdateOption 的调用路径：
//
//	before = {"key":"Foo","value":"old"} （修复前传 nil）
//	after  = {"key":"Foo","value":"new"}
//
// 期望 before_data 与 after_data 同时存在，且各自包含对应值。
func TestSerializeAuditDiff_OptionUpdateBeforeAndAfter(t *testing.T) {
	before := map[string]interface{}{"key": "Foo", "value": "old"}
	after := map[string]interface{}{"key": "Foo", "value": "new"}

	beforeStr, afterStr := serializeAuditDiff(before, after, true)
	if beforeStr == "" {
		t.Fatalf("before_data 不应为空：修复前 before 传 nil 导致审计日志缺少修改前配置")
	}
	if afterStr == "" {
		t.Fatalf("after_data 不应为空")
	}

	var beforeMap map[string]interface{}
	if err := json.Unmarshal([]byte(beforeStr), &beforeMap); err != nil {
		t.Fatalf("before_data 不是合法 JSON: %v", err)
	}
	if beforeMap["value"] != "old" {
		t.Fatalf("before_data.value 应为 old, 实际 %v", beforeMap["value"])
	}

	var afterMap map[string]interface{}
	if err := json.Unmarshal([]byte(afterStr), &afterMap); err != nil {
		t.Fatalf("after_data 不是合法 JSON: %v", err)
	}
	if afterMap["value"] != "new" {
		t.Fatalf("after_data.value 应为 new, 实际 %v", afterMap["value"])
	}
}

// TestSerializeAuditDiff_OptionUpdateNilBefore 兼容首次创建场景：
// OptionMap 中不存在该 key 时 before 为 nil，before_data 应为空，
// after_data 仍记录新值。保持与修复前行为一致，不破坏 create 路径。
func TestSerializeAuditDiff_OptionUpdateNilBefore(t *testing.T) {
	after := map[string]interface{}{"key": "Foo", "value": "new"}

	beforeStr, afterStr := serializeAuditDiff(nil, after, true)
	if beforeStr != "" {
		t.Fatalf("before 为 nil 时 before_data 应为空, 实际: %s", beforeStr)
	}
	if !strings.Contains(afterStr, "new") {
		t.Fatalf("after_data 应包含 new, 实际: %s", afterStr)
	}
}

// TestSerializeAuditDiff_SensitiveKeyRedaction 验证敏感 key 的脱敏
// 同时作用于 before 和 after，避免在审计日志中泄露旧凭证。
func TestSerializeAuditDiff_SensitiveKeyRedaction(t *testing.T) {
	// 模拟 controller 中对敏感 key 的脱敏：调用方将 value 替换为 [REDACTED]。
	before := map[string]interface{}{"key": "GitHubToken", "value": "[REDACTED]"}
	after := map[string]interface{}{"key": "GitHubToken", "value": "[REDACTED]"}

	beforeStr, afterStr := serializeAuditDiff(before, after, true)

	// 脱敏后 before/after 的 value 相同，computeDiff 认为无变化，
	// 二者都会是空串 —— 这是期望行为（敏感值未变不应写入审计日志）。
	if beforeStr != "" || afterStr != "" {
		t.Fatalf("脱敏后无差异时 before/after 应均为空, before=%s after=%s", beforeStr, afterStr)
	}
}

// TestMaskCredential_MasksPasswordKeepsSkeleton 验证 DSN/带凭据 URL 的值级遮蔽：
// 口令替换为 ***，scheme、用户名、主机、端口、库名与查询参数等非敏感骨架保留，
// 便于审计排查。
func TestMaskCredential_MasksPasswordKeepsSkeleton(t *testing.T) {
	cases := []struct {
		name string
		raw  string
		want string
	}{
		{
			name: "postgres URL",
			raw:  "postgres://root:s3cret@10.0.0.9:5432/nookmux?sslmode=disable",
			want: "postgres://root:***@10.0.0.9:5432/nookmux?sslmode=disable",
		},
		{
			name: "mysql driver DSN",
			raw:  "root:s3cret@tcp(10.0.0.9:3306)/nookmux?charset=utf8mb4",
			want: "root:***@tcp(10.0.0.9:3306)/nookmux?charset=utf8mb4",
		},
	}
	for _, tc := range cases {
		if got := MaskCredential(tc.raw); got != tc.want {
			t.Fatalf("%s: MaskCredential(%q) = %q, want %q", tc.name, tc.raw, got, tc.want)
		}
	}
}

// TestMaskCredential_LeavesNonCredentialValuesIntact 验证不含口令的值原样返回，
// 正常业务值不受值级兜底影响。
func TestMaskCredential_LeavesNonCredentialValuesIntact(t *testing.T) {
	cases := []string{
		"",
		"local",
		"/data/nookmux/nookmux.db",
		"https://api.example.com/v1/models",
		"postgres://root@10.0.0.9:5432/nookmux", // 仅用户名、无口令
		"2024-report@team/notes",                // 含 @ 与 / 但不构成 DSN
		"plain audit description text",
	}
	for _, raw := range cases {
		if got := MaskCredential(raw); got != raw {
			t.Fatalf("MaskCredential(%q) = %q, want 原样返回", raw, got)
		}
	}
}

// TestSerializeAuditDiff_DSNFieldFullyRedacted 验证字段级兜底：字段名含 dsn 时
// 值整体脱敏，迁移目标库口令不得进入审计日志。
func TestSerializeAuditDiff_DSNFieldFullyRedacted(t *testing.T) {
	// 创建场景（before 为 nil）：after 中 dsn 字段整体脱敏，非敏感字段保留。
	after := map[string]interface{}{
		"target_dsn":     "root:newsecret@tcp(10.0.0.9:3306)/newdb",
		"target_log_dsn": "postgres://root:logs3cret@10.0.0.9:5432/logs",
		"include_logs":   true,
	}

	_, afterStr := serializeAuditDiff(nil, after, true)

	if strings.Contains(afterStr, "newsecret") || strings.Contains(afterStr, "logs3cret") {
		t.Fatalf("after_data 不应包含口令: %s", afterStr)
	}

	var afterMap map[string]interface{}
	if err := jsonx.Unmarshal([]byte(afterStr), &afterMap); err != nil {
		t.Fatalf("after_data 不是合法 JSON: %v", err)
	}
	if afterMap["target_dsn"] != "[REDACTED]" {
		t.Fatalf("target_dsn 应为 [REDACTED], 实际 %v", afterMap["target_dsn"])
	}
	if afterMap["target_log_dsn"] != "[REDACTED]" {
		t.Fatalf("target_log_dsn 应为 [REDACTED], 实际 %v", afterMap["target_log_dsn"])
	}
	if afterMap["include_logs"] != true {
		t.Fatalf("include_logs 不受脱敏影响, 实际 %v", afterMap["include_logs"])
	}

	// 更新场景：before/after 的 dsn 字段脱敏后同为 [REDACTED]，diff 判定无变化，
	// 口令不落库。与 TestSerializeAuditDiff_SensitiveKeyRedaction 的契约一致。
	beforeUpdate := map[string]interface{}{
		"target_dsn": "postgres://root:s3cret@10.0.0.9:5432/old",
	}
	afterUpdate := map[string]interface{}{
		"target_dsn": "root:newsecret@tcp(10.0.0.9:3306)/newdb",
	}
	beforeStr, afterStr := serializeAuditDiff(beforeUpdate, afterUpdate, true)
	if strings.Contains(beforeStr, "s3cret") || strings.Contains(afterStr, "newsecret") {
		t.Fatalf("更新场景 diff 不应包含口令: before=%s after=%s", beforeStr, afterStr)
	}
	if beforeStr != "" || afterStr != "" {
		t.Fatalf("脱敏后无差异时 before/after 应均为空, before=%s after=%s", beforeStr, afterStr)
	}
}

// TestSerializeAuditDiff_CredentialURLValueMaskedInArbitraryField 验证值级兜底：
// 字段名不含敏感子串、但值为 DSN/带凭据 URL 形态时仅遮蔽口令、保留骨架；
// 嵌套结构同样生效，正常值不受影响。
func TestSerializeAuditDiff_CredentialURLValueMaskedInArbitraryField(t *testing.T) {
	after := map[string]interface{}{
		"endpoint": "postgres://alice:wonder@db.internal:5432/app",
		"config": map[string]interface{}{
			"upstream": "bob:pw123@tcp(10.0.0.5:3306)/meta",
		},
		"remark": "https://console.example.com/settings",
	}

	_, afterStr := serializeAuditDiff(nil, after, true)

	for _, secret := range []string{"wonder", "pw123"} {
		if strings.Contains(afterStr, secret) {
			t.Fatalf("after_data 不应包含口令 %q: %s", secret, afterStr)
		}
	}

	var afterMap map[string]interface{}
	if err := jsonx.Unmarshal([]byte(afterStr), &afterMap); err != nil {
		t.Fatalf("after_data 不是合法 JSON: %v", err)
	}
	if got := afterMap["endpoint"]; got != "postgres://alice:***@db.internal:5432/app" {
		t.Fatalf("endpoint 应遮蔽口令并保留骨架, 实际 %v", got)
	}
	config := afterMap["config"].(map[string]interface{})
	if got := config["upstream"]; got != "bob:***@tcp(10.0.0.5:3306)/meta" {
		t.Fatalf("嵌套 upstream 应遮蔽口令并保留骨架, 实际 %v", got)
	}
	if got := afterMap["remark"]; got != "https://console.example.com/settings" {
		t.Fatalf("普通 URL 不受影响, 实际 %v", got)
	}
}
