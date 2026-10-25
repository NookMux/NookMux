package dbstore

import (
	"strings"
	"testing"
)

// TestChooseDBRejectsUnknownDSNPrefix 验证非法前缀的 DSN 在建立连接前
// 即返回错误，不再兜底按 MySQL 处理。
func TestChooseDBRejectsUnknownDSNPrefix(t *testing.T) {
	tests := []struct {
		name string
		dsn  string
	}{
		{name: "裸 MySQL 驱动 DSN", dsn: "user:password@tcp(127.0.0.1:3306)/nookmux"},
		{name: "local 前缀", dsn: "localhost"},
		{name: "MySQL URL 缺数据库名", dsn: "mysql://user:password@127.0.0.1:3306"},
		{name: "MySQL URL 口令含@", dsn: "mysql://user:p%40ss@127.0.0.1:3306/nookmux"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("SQL_DSN", tt.dsn)
			_, err := ChooseDB("SQL_DSN", false)
			if err == nil {
				t.Fatalf("ChooseDB(SQL_DSN=%q) 期望报错，实际成功", tt.dsn)
			}
		})
	}
}

// TestChooseDBRejectsMySQLURLParseFailure 验证 mysql:// 前缀但结构非法的
// DSN 报错信息可定位问题。
func TestChooseDBRejectsMySQLURLParseFailure(t *testing.T) {
	t.Setenv("SQL_DSN", "mysql://user:password@127.0.0.1:3306")
	_, err := ChooseDB("SQL_DSN", false)
	if err == nil {
		t.Fatal("ChooseDB 期望报错，实际成功")
	}
	if !strings.Contains(err.Error(), "数据库名") {
		t.Fatalf("错误信息未指出缺失数据库名: %v", err)
	}
}
