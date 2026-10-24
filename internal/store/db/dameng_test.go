package dbstore

import (
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/schema"
)

// dmBoolNormModel 覆盖布尔默认值归一化的全部分支：true/false 默认值需归一
// 为 1/0，无默认值布尔字段与非布尔默认值字段不得被触碰。
type dmBoolNormModel struct {
	ID        uint `gorm:"primaryKey"`
	FlagTrue  bool `gorm:"default:true"`
	FlagFalse bool `gorm:"default:false"`
	FlagNone  bool
	Name      string `gorm:"default:'backup'"`
	Count     int    `gorm:"default:42"`
}

func TestNormalizeDMBoolDefaultLiterals(t *testing.T) {
	gdb, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("打开内存 SQLite 失败: %v", err)
	}
	// GORM 按模型类型缓存 schema：这里首次解析拿归一前快照，之后
	// NormalizeDMBoolDefaultLiterals 内部再解析命中同一实例，字段指针不变。
	stmt := &gorm.Statement{DB: gdb}
	if err := stmt.Parse(&dmBoolNormModel{}); err != nil {
		t.Fatalf("解析测试模型失败: %v", err)
	}
	sch := stmt.Schema
	snapshot := func() map[string][2]string {
		out := make(map[string][2]string)
		for _, name := range []string{"FlagTrue", "FlagFalse", "FlagNone", "Name", "Count"} {
			f := sch.FieldsByName[name]
			out[name] = [2]string{f.DefaultValue, f.TagSettings["DEFAULT"]}
		}
		return out
	}
	before := snapshot()
	if before["FlagTrue"] != [2]string{"true", "true"} || before["FlagFalse"] != [2]string{"false", "false"} {
		t.Fatalf("测试前提不成立：归一前布尔默认值字面量异常: %v", before)
	}
	if sch.FieldsByName["FlagNone"].HasDefaultValue {
		t.Fatalf("测试前提不成立：FlagNone 不应带有默认值")
	}

	assertNormalized := func(stage string) {
		for name, want := range map[string][2]string{
			"FlagTrue":  {"1", "1"},
			"FlagFalse": {"0", "0"},
		} {
			f := sch.FieldsByName[name]
			if got := [2]string{f.DefaultValue, f.TagSettings["DEFAULT"]}; got != want {
				t.Fatalf("%s：%s 归一结果为 %v，期望 %v", stage, name, got, want)
			}
		}
		for _, name := range []string{"Name", "Count"} {
			f := sch.FieldsByName[name]
			if got := [2]string{f.DefaultValue, f.TagSettings["DEFAULT"]}; got != before[name] {
				t.Fatalf("%s：%s 不应被改动，%v 实际变为 %v", stage, name, before[name], got)
			}
		}
		if sch.FieldsByName["FlagNone"].HasDefaultValue {
			t.Fatalf("%s：FlagNone 不应被赋予默认值", stage)
		}
	}

	if err := NormalizeDMBoolDefaultLiterals(gdb, []any{&dmBoolNormModel{}}); err != nil {
		t.Fatalf("NormalizeDMBoolDefaultLiterals 失败: %v", err)
	}
	assertNormalized("首次归一")

	if err := NormalizeDMBoolDefaultLiterals(gdb, []any{&dmBoolNormModel{}}); err != nil {
		t.Fatalf("NormalizeDMBoolDefaultLiterals 重复调用失败: %v", err)
	}
	assertNormalized("重复归一")
}

// TestValidateDMDsnPassword 校验 dm:// DSN 口令的前置拦截规则，口令提取
// 逻辑对齐驱动 parseDSN（最后一个 '?' 切 query、最后一个 '@' 切 userinfo、
// 第一个 ':' 后全为口令）。
func TestValidateDMDsnPassword(t *testing.T) {
	tests := []struct {
		name    string
		dsn     string
		wantErr bool
	}{
		{name: "常规口令", dsn: "dm://NOOKMUX:secret123@127.0.0.1:5236?schema=NOOKMUX"},
		{name: "口令含冒号（第一个冒号后全为口令，安全）", dsn: "dm://user:pa:ss@127.0.0.1:5236"},
		{name: "口令以冒号开头", dsn: "dm://user::pass@127.0.0.1:5236"},
		{name: "口令含@", dsn: "dm://user:p@ss@127.0.0.1:5236"},
		{name: "口令含中文", dsn: "dm://user:中文口令@127.0.0.1:5236"},
		{name: "口令含%（可能是字面值，仅告警不拦截）", dsn: "dm://user:p%40ss@127.0.0.1:5236"},
		{name: "口令含?且DSN无query参数（驱动会把口令截断为 query）", dsn: "dm://user:pa?ss@127.0.0.1:5236", wantErr: true},
		// 驱动按最后一个 '?' 切 query 时该形态恰好可解析出口令，但正确性
		// 依赖 DSN 整体形状，前置校验一律拦截。
		{name: "口令含?且DSN带query参数", dsn: "dm://user:pa?ss@127.0.0.1:5236?schema=USER", wantErr: true},
		{name: "未携带口令（交由服务器认证暴露）", dsn: "dm://user@127.0.0.1:5236"},
		{name: "非dm前缀（交由驱动 parseDSN 报错）", dsn: "mysql://user:pa?ss@127.0.0.1:3306/db"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateDMDsnPassword(tt.dsn)
			if (err != nil) != tt.wantErr {
				t.Fatalf("validateDMDsnPassword(%q) 返回错误 = %v，期望报错 = %v", tt.dsn, err, tt.wantErr)
			}
		})
	}
}

// TestNormalizeDMBoolDefaultLiteralsSchemaField 断言归一化后字段的
// schema.Field 类型与默认值语义完整（HasDefaultValue 保持 true），
// 防止归一改写 DefaultValue 时意外破坏字段元数据。
func TestNormalizeDMBoolDefaultLiteralsSchemaField(t *testing.T) {
	gdb, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("打开内存 SQLite 失败: %v", err)
	}
	if err := NormalizeDMBoolDefaultLiterals(gdb, []any{&dmBoolNormModel{}}); err != nil {
		t.Fatalf("NormalizeDMBoolDefaultLiterals 失败: %v", err)
	}
	stmt := &gorm.Statement{DB: gdb}
	if err := stmt.Parse(&dmBoolNormModel{}); err != nil {
		t.Fatalf("解析测试模型失败: %v", err)
	}
	for name, want := range map[string]string{"FlagTrue": "1", "FlagFalse": "0"} {
		f := stmt.Schema.FieldsByName[name]
		if f.DataType != schema.Bool || !f.HasDefaultValue {
			t.Fatalf("%s 归一后应为带默认值的 Bool 字段，实际 DataType=%v HasDefaultValue=%v", name, f.DataType, f.HasDefaultValue)
		}
		if f.DefaultValue != want {
			t.Fatalf("%s.DefaultValue = %q，期望 %q", name, f.DefaultValue, want)
		}
	}
}
