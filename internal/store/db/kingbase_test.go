package dbstore

import (
	"strings"
	"testing"

	"gorm.io/gorm"
)

// TestEnsureKingbaseOraEmptyStrIsNotNull 验证 kingbase:// DSN 的
// ora_input_emptystr_isnull 归一：未携带或携带空值（Gokb 的 URL 解析会整体
// 丢弃空值参数）时补齐 off；唯一键、唯一非空取值时原样保留；重复携带或
// 多个大小写变体时收敛为单个小写 off；判定仅基于 query 段，口令中的同名
// 子串不构成"已携带"。解析失败立即报错且不回显 DSN 原文。
func TestEnsureKingbaseOraEmptyStrIsNotNull(t *testing.T) {
	tests := []struct {
		name    string
		dsn     string
		want    string
		wantErr bool
	}{
		{
			name: "无参数 DSN 追加 off",
			dsn:  "kingbase://system:pass@127.0.0.1:54321/nookmux",
			want: "kingbase://system:pass@127.0.0.1:54321/nookmux?ora_input_emptystr_isnull=off",
		},
		{
			name: "已有其他参数时合并进 query 段",
			dsn:  "kingbase://system:p%40ss@127.0.0.1:54321/nookmux?sslmode=disable",
			want: "kingbase://system:p%40ss@127.0.0.1:54321/nookmux?ora_input_emptystr_isnull=off&sslmode=disable",
		},
		{
			name: "显式 off 时不重复追加",
			dsn:  "kingbase://system:pass@127.0.0.1:54321/nookmux?ora_input_emptystr_isnull=off",
			want: "kingbase://system:pass@127.0.0.1:54321/nookmux?ora_input_emptystr_isnull=off",
		},
		{
			name: "大写单变体非空取值原样保留（GUC 名大小写不敏感，单变体无歧义）",
			dsn:  "kingbase://system:pass@127.0.0.1:54321/nookmux?ORA_INPUT_EMPTYSTR_ISNULL=off",
			want: "kingbase://system:pass@127.0.0.1:54321/nookmux?ORA_INPUT_EMPTYSTR_ISNULL=off",
		},
		{
			name: "空值参数归一为 off",
			dsn:  "kingbase://system:pass@127.0.0.1:54321/nookmux?ora_input_emptystr_isnull=",
			want: "kingbase://system:pass@127.0.0.1:54321/nookmux?ora_input_emptystr_isnull=off",
		},
		{
			name: "重复携带同名参数收敛为单一 off（Gokb 后值覆盖，双值有歧义）",
			dsn:  "kingbase://system:pass@127.0.0.1:54321/nookmux?ora_input_emptystr_isnull=off&ora_input_emptystr_isnull=on",
			want: "kingbase://system:pass@127.0.0.1:54321/nookmux?ora_input_emptystr_isnull=off",
		},
		{
			name: "大小写双变体收敛为单个小写 off（避免驱动按 map 顺序随机生效）",
			dsn:  "kingbase://system:pass@127.0.0.1:54321/nookmux?ora_input_emptystr_isnull=off&ORA_INPUT_EMPTYSTR_ISNULL=off",
			want: "kingbase://system:pass@127.0.0.1:54321/nookmux?ora_input_emptystr_isnull=off",
		},
		{
			name: "口令含同名子串不误判为已携带",
			dsn:  "kingbase://system:xxora_input_emptystr_isnull=y@127.0.0.1:54321/nookmux",
			want: "kingbase://system:xxora_input_emptystr_isnull=y@127.0.0.1:54321/nookmux?ora_input_emptystr_isnull=off",
		},
		{
			name:    "口令含非法 percent-escape 时解析失败即报错（不放行未归一化的 DSN）",
			dsn:     "kingbase://system:p%zz@127.0.0.1:54321/nookmux",
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := EnsureKingbaseOraEmptyStrIsNotNull(tt.dsn)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("EnsureKingbaseOraEmptyStrIsNotNull(%q) 应报错，得到 %q", tt.dsn, got)
				}
				if strings.Contains(err.Error(), tt.dsn) {
					t.Fatalf("错误信息不得回显 DSN 原文（防口令泄漏），得到: %v", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("EnsureKingbaseOraEmptyStrIsNotNull(%q) 报错: %v", tt.dsn, err)
			}
			if got != tt.want {
				t.Fatalf("EnsureKingbaseOraEmptyStrIsNotNull(%q) = %q, 期望 %q", tt.dsn, got, tt.want)
			}
		})
	}
}

// TestOpenKingbaseGormRejectsOraEmptyStrIsNullOn 验证 DSN 显式携带
// ora_input_emptystr_isnull=on 类取值（任意大小写变体、任意携带位置）时在
// 建立连接前即拒绝，无需可达的数据库实例。
func TestOpenKingbaseGormRejectsOraEmptyStrIsNullOn(t *testing.T) {
	for _, dsn := range []string{
		"kingbase://system:pass@127.0.0.1:54321/nookmux?ora_input_emptystr_isnull=on",
		"kingbase://system:pass@127.0.0.1:54321/nookmux?sslmode=disable&ora_input_emptystr_isnull=true",
		// 大写键：GUC 名大小写不敏感，守卫同样命中
		"kingbase://system:pass@127.0.0.1:54321/nookmux?ORA_INPUT_EMPTYSTR_ISNULL=on",
		// 重复携带：Gokb 后值覆盖，任一取值为 on 即拒绝
		"kingbase://system:pass@127.0.0.1:54321/nookmux?ora_input_emptystr_isnull=off&ora_input_emptystr_isnull=on",
	} {
		_, err := OpenKingbaseGorm(dsn, &gorm.Config{})
		if err == nil || !strings.Contains(err.Error(), kingbaseOraEmptyStrParam) {
			t.Fatalf("OpenKingbaseGorm(%q) 应拒绝 on 类取值（含大小写与重复携带变体），得到错误: %v", dsn, err)
		}
	}
}

// TestOpenKingbaseGormRejectsMalformedURL 验证 DSN URL 解析失败时连接前即
// 报错（错误不回显 DSN 原文），不被静默放行。
func TestOpenKingbaseGormRejectsMalformedURL(t *testing.T) {
	dsn := "kingbase://system:p%zz@127.0.0.1:54321/nookmux?ora_input_emptystr_isnull=on"
	_, err := OpenKingbaseGorm(dsn, &gorm.Config{})
	if err == nil || !strings.Contains(err.Error(), "解析 kingbase:// DSN 失败") {
		t.Fatalf("OpenKingbaseGorm 应在连接前报 URL 解析错误，得到: %v", err)
	}
	if strings.Contains(err.Error(), dsn) {
		t.Fatalf("错误信息不得回显 DSN 原文（防口令泄漏），得到: %v", err)
	}
}
