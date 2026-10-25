package dbstore

import "testing"

// TestEnsureKingbaseOraEmptyStrIsNotNull 验证 kingbase:// DSN 在未显式携带
// ora_input_emptystr_isnull 参数时自动补齐 off，显式设置（含 =on）时原样保留。
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
			name: "已有其他参数时以 & 追加",
			dsn:  "kingbase://system:p%40ss@127.0.0.1:54321/nookmux?sslmode=disable",
			want: "kingbase://system:p%40ss@127.0.0.1:54321/nookmux?sslmode=disable&ora_input_emptystr_isnull=off",
		},
		{
			name: "显式 off 时不重复追加",
			dsn:  "kingbase://system:pass@127.0.0.1:54321/nookmux?ora_input_emptystr_isnull=off",
			want: "kingbase://system:pass@127.0.0.1:54321/nookmux?ora_input_emptystr_isnull=off",
		},
		{
			name: "显式 on 时尊重用户设置",
			dsn:  "kingbase://system:pass@127.0.0.1:54321/nookmux?sslmode=disable&ora_input_emptystr_isnull=on",
			want: "kingbase://system:pass@127.0.0.1:54321/nookmux?sslmode=disable&ora_input_emptystr_isnull=on",
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
