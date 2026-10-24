package dbstore

import (
	"fmt"
	"strings"

	"github.com/godoes/gorm-dameng"
	"gorm.io/gorm"
	"gorm.io/gorm/schema"
)

// OpenDMGorm 统一封装达梦 DM8 的 GORM 连接建立，使用 gorm-dameng 方言
// （内置达梦官方 Go 驱动，Create 回调把 GORM 的 OnConflict 翻译为
// MERGE INTO，Migrator 基于 SYS 系统视图实现）。
//
// DSN 采用达梦官方 Go 驱动原生识别的 dm:// URL 形式：
//
//	dm://user:password@host:5236?schema=NOOKMUX
//
// 注意驱动 parseDSN 不做 percent-decode：口令按字面值写入（含 '@' 无碍，
// 按 DSN 中最后一个 '@' 切分 userinfo 与主机），但口令不得包含 ':' 与
// '?'（':' 为用户名/口令分隔符，'?' 会被当作 query 起始）；schema 指定
// 登录后的当前模式，缺省为该用户的默认模式（同名 schema）。
//
// 实例与账号要求：
//   - 库必须以 CASE_SENSITIVE=N 初始化（dminit 参数），并在连接时校验：
//     GORM 以双引号小写标识符建表，而项目内手写的 WHERE/ORDER 条件多为
//     不带引号的标识符，CASE_SENSITIVE=Y 的库会把后者解析为大写列名导致
//     "无效的列名"错误；
//   - 业务用户需要 SELECT ANY DICTIONARY 权限：gorm-dameng 的 Migrator
//     通过 SYS.SYSOBJECTS/SYSCOLUMNS 系统视图探测表结构，无该权限时
//     AutoMigrate 的 HasTable 会静默判否并重复建表。
//
// VarcharSizeIsCharLength 开启后字符串列按 VARCHAR(n CHAR) 字符长度建列：
// 达梦默认 VARCHAR(n) 计字节长度，UTF-8 下每个中文字符占 3 字节，name、
// remark 等带 size 上限的中文列会在字节口径下意外截断/超长报错，字符口径
// 与其他数据库的长度语义保持一致。
func OpenDMGorm(dsn string, cfg *gorm.Config) (*gorm.DB, error) {
	gormDB, err := gorm.Open(dameng.New(dameng.Config{
		DSN:                     dsn,
		VarcharSizeIsCharLength: true,
	}), cfg)
	if err != nil {
		return nil, err
	}
	var caseSensitive int
	if err := gormDB.Raw("SELECT SF_GET_CASE_SENSITIVE_FLAG()").Row().Scan(&caseSensitive); err != nil {
		return nil, fmt.Errorf("读取达梦 CASE_SENSITIVE 标志失败: %w", err)
	}
	if caseSensitive != 0 {
		return nil, fmt.Errorf("达梦实例为 CASE_SENSITIVE=Y（大小写敏感），NookMux 需要以 CASE_SENSITIVE=N 初始化的实例（dminit ... CASE_SENSITIVE=N），请重建实例或切换到符合要求的库")
	}
	return gormDB, nil
}

// NormalizeDMBoolDefaultLiterals 把模型布尔字段的默认值字面量 true/false 归一为
// 1/0，必须在达梦库 AutoMigrate 之前调用（可重复调用，幂等）。
//
// 背景：gorm-dameng 建表时会把布尔默认值转换为 1/0 落库（达梦 BIT 列的
// DEFAULT 表达式不接受 true/false 字面量），但其列迁移比较仍取模型 tag 的
// true/false 字面量，每次 AutoMigrate 都会误判"默认值漂移"，生成
// `ALTER TABLE ... DEFAULT true` 并被达梦以 -2670（DEFAULT 约束表达式无效）
// 拒绝。归一化同时改写 field.DefaultValue 与 TagSettings["DEFAULT"]：前者
// 使漂移比较相等从而不触发 ALTER，后者保证即便列因其他原因变更，生成的
// DDL 中的 DEFAULT 1/0 也合法。Create 路径使用的 DefaultValueInterface 在
// schema 解析时已定型，不受影响。
func NormalizeDMBoolDefaultLiterals(gormDB *gorm.DB, models []any) error {
	for _, model := range models {
		stmt := &gorm.Statement{DB: gormDB}
		if err := stmt.Parse(model); err != nil {
			return fmt.Errorf("解析模型 schema 失败（达梦布尔默认值归一化）: %w", err)
		}
		for _, field := range stmt.Schema.Fields {
			if field.DataType != schema.Bool || !field.HasDefaultValue {
				continue
			}
			switch strings.ToLower(field.DefaultValue) {
			case "true":
				field.DefaultValue = "1"
				field.TagSettings["DEFAULT"] = "1"
			case "false":
				field.DefaultValue = "0"
				field.TagSettings["DEFAULT"] = "0"
			}
		}
	}
	return nil
}
