package dbmigrate

import (
	"strings"
	"testing"

	"github.com/NookMux/NookMux/internal/store/token"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

// TestMySQLDDLHasNoDefaultOnTextLikeColumns 验证主库全部模型在 MySQL 方言下
// 生成的列 DDL 中，TEXT/BLOB/JSON 类列不携带 DEFAULT 子句。
//
// MySQL（含 5.7 与 8.0）禁止 BLOB/TEXT/GEOMETRY/JSON 列设置字面量默认值
// （Error 1101），GORM 对显式声明 type:text 且带 default tag 的字段没有
// 自动降级保护（仅对未声明 type 的 string 字段强制 varchar(191)），建表与
// 列类型变更都会直接失败。该测试在 CI 中拦截此类模型定义回归。
func TestMySQLDDLHasNoDefaultOnTextLikeColumns(t *testing.T) {
	db, err := gorm.Open(mysql.New(mysql.Config{
		SkipInitializeWithVersion: true,
		DSN:                       "gorm:gorm@tcp(127.0.0.1:1)/gorm?charset=utf8mb4&parseTime=True",
	}), &gorm.Config{DisableAutomaticPing: true})
	if err != nil {
		t.Fatalf("open offline mysql dialector: %v", err)
	}

	mysqlTextLike := func(dataType string) bool {
		switch {
		case strings.HasPrefix(dataType, "text"),
			strings.HasPrefix(dataType, "tinytext"),
			strings.HasPrefix(dataType, "mediumtext"),
			strings.HasPrefix(dataType, "longtext"),
			strings.HasPrefix(dataType, "blob"),
			strings.HasPrefix(dataType, "tinyblob"),
			strings.HasPrefix(dataType, "mediumblob"),
			strings.HasPrefix(dataType, "longblob"),
			strings.HasPrefix(dataType, "json"):
			return true
		}
		return false
	}

	for _, model := range mainDBModels() {
		stmt := &gorm.Statement{DB: db}
		if err := stmt.Parse(model); err != nil {
			t.Fatalf("parse schema %T: %v", model, err)
		}
		for _, field := range stmt.Schema.Fields {
			if field.DBName == "" || field.IgnoreMigration {
				continue
			}
			fullType := db.Migrator().FullDataTypeOf(field)
			dataType := strings.ToLower(strings.Fields(fullType.SQL)[0])
			if !mysqlTextLike(dataType) {
				continue
			}
			if strings.Contains(strings.ToUpper(fullType.SQL), "DEFAULT") {
				t.Errorf("%s.%s: MySQL 不允许 %s 列携带 DEFAULT（Error 1101），DDL: %s；请移除该字段的 default tag",
					stmt.Schema.Table, field.DBName, dataType, fullType.SQL)
			}
		}
	}
}

// TestTokenModelLimitsMySQLDDLIsPlainText 锚定 tokens.model_limits 的回归：
// 该列必须为无 DEFAULT 的 text，否则旧库（varchar(1024) 默认空字符串）升级与
// 全新 MySQL 建表都会因 Error 1101 失败。
func TestTokenModelLimitsMySQLDDLIsPlainText(t *testing.T) {
	db, err := gorm.Open(mysql.New(mysql.Config{
		SkipInitializeWithVersion: true,
		DSN:                       "gorm:gorm@tcp(127.0.0.1:1)/gorm?charset=utf8mb4&parseTime=True",
	}), &gorm.Config{DisableAutomaticPing: true})
	if err != nil {
		t.Fatalf("open offline mysql dialector: %v", err)
	}

	stmt := &gorm.Statement{DB: db}
	if err := stmt.Parse(&tokenstore.Token{}); err != nil {
		t.Fatalf("parse Token schema: %v", err)
	}
	field := stmt.Schema.LookUpField("ModelLimits")
	if field == nil {
		t.Fatalf("field Token.ModelLimits not found")
	}
	fullType := db.Migrator().FullDataTypeOf(field)
	if got := strings.ToUpper(fullType.SQL); got != "TEXT" {
		t.Fatalf("tokens.model_limits DDL = %q, want %q (no DEFAULT / NOT NULL clauses)", got, "TEXT")
	}
}
