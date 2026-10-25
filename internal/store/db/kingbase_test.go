package dbstore

import (
	"strings"
	"testing"

	"gorm.io/gorm"
)

// TestEnsureKingbaseOraEmptyStrIsNotNull 验证 kingbase:// DSN 的
// ora_input_emptystr_isnull 归一：未携带或携带空值（Gokb 的 URL 解析会整体
// 丢弃空值参数）时补齐 off；显式携带有效值时原样保留；判定仅基于 query
// 段，口令中的同名子串不构成"已携带"。
func TestEnsureKingbaseOraEmptyStrIsNotNull(t *testing.T) {
	tests := []struct {
		name string
		dsn  string
		want string
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
			name: "空值参数归一为 off",
			dsn:  "kingbase://system:pass@127.0.0.1:54321/nookmux?ora_input_emptystr_isnull=",
			want: "kingbase://system:pass@127.0.0.1:54321/nookmux?ora_input_emptystr_isnull=off",
		},
		{
			name: "口令含同名子串不误判为已携带",
			dsn:  "kingbase://system:xxora_input_emptystr_isnull=y@127.0.0.1:54321/nookmux",
			want: "kingbase://system:xxora_input_emptystr_isnull=y@127.0.0.1:54321/nookmux?ora_input_emptystr_isnull=off",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := EnsureKingbaseOraEmptyStrIsNotNull(tt.dsn); got != tt.want {
				t.Fatalf("EnsureKingbaseOraEmptyStrIsNotNull(%q) = %q, 期望 %q", tt.dsn, got, tt.want)
			}
		})
	}
}

// TestOpenKingbaseGormRejectsOraEmptyStrIsNullOn 验证 DSN 显式携带
// ora_input_emptystr_isnull=on 时在建立连接前即拒绝，无需可达的数据库实例。
func TestOpenKingbaseGormRejectsOraEmptyStrIsNullOn(t *testing.T) {
	for _, dsn := range []string{
		"kingbase://system:pass@127.0.0.1:54321/nookmux?ora_input_emptystr_isnull=on",
		"kingbase://system:pass@127.0.0.1:54321/nookmux?sslmode=disable&ora_input_emptystr_isnull=true",
	} {
		_, err := OpenKingbaseGorm(dsn, &gorm.Config{})
		if err == nil || !strings.Contains(err.Error(), "ora_input_emptystr_isnull") {
			t.Fatalf("OpenKingbaseGorm(%q) 应拒绝显式 =on 类取值，得到错误: %v", dsn, err)
		}
	}
}
