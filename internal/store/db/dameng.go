package dbstore

import (
	"errors"
	"fmt"
	"strings"

	"github.com/NookMux/NookMux/internal/common"
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
// 注意驱动 parseDSN 不做 percent-decode，口令按字面值写入：userinfo 与
// 主机按 DSN 中最后一个 '@' 切分，userinfo 中第一个 ':' 之后的全部内容
// 为口令，因此口令可以包含 ':' 与 '@'；但 '?' 会被 parseDSN 当作 query
// 参数起始（按 DSN 中最后一个 '?' 切分），口令携带 '?' 时存在被截断而
// 静默认证失败的风险，连接前由 validateDMDsnPassword 一律拦截。schema
// 指定登录后的当前模式，缺省为该用户的默认模式（同名 schema）。
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
	if err := validateDMDsnPassword(dsn); err != nil {
		return nil, err
	}
	gormDB, err := gorm.Open(dameng.New(dameng.Config{
		DSN:                     dsn,
		VarcharSizeIsCharLength: true,
	}), cfg)
	if err != nil {
		return nil, err
	}
	// 校验失败分支必须关闭底层连接池再返回：预迁移是管理端 HTTP 运行时
	// 可反复触发的动作，句柄不关闭会持续泄漏；关闭失败以主错误为准
	// （复用包级 closeDB）。
	var caseSensitive int
	if err := gormDB.Raw("SELECT SF_GET_CASE_SENSITIVE_FLAG()").Row().Scan(&caseSensitive); err != nil {
		_ = closeDB(gormDB)
		return nil, fmt.Errorf("读取达梦 CASE_SENSITIVE 标志失败: %w", err)
	}
	if caseSensitive != 0 {
		_ = closeDB(gormDB)
		return nil, fmt.Errorf("达梦实例为 CASE_SENSITIVE=Y（大小写敏感），NookMux 需要以 CASE_SENSITIVE=N 初始化的实例（dminit ... CASE_SENSITIVE=N），请重建实例或切换到符合要求的库")
	}
	return gormDB, nil
}

// validateDMDsnPassword 在建立连接前校验 dm:// DSN 用户名/口令中的非法
// 字符，切分规则与驱动 parseDSN（dm8/n.go）一致：去掉 dm:// 前缀后，
// userinfo 与主机按最后一个 '@' 切分，userinfo 中第一个 ':' 之后的全部
// 内容为口令。
//
//   - userinfo 区间包含 '?' 时报错：驱动以整个 DSN 的最后一个 '?' 切分
//     query 参数，口令携带 '?' 无法与"口令被截断"的形态区分（其后没有
//     真正 query 参数时必然截断，导致静默认证失败或整体解析错误）；
//   - ':' 与 '@' 对口令安全（第一个 ':' 之后、最后一个 '@' 之前全为
//     口令），放行；
//   - '%' 可能是口令字面值，也可能是用户按 URL 习惯 percent-encode 的
//     残留，只记录警告不拦截。
//
// 非 dm:// 前缀、缺少 '@' 或未携带口令的畸形 DSN 不在此拦截，由驱动
// parseDSN 与服务器认证按各自错误暴露。
func validateDMDsnPassword(dsn string) error {
	const scheme = "dm://"
	if !strings.HasPrefix(dsn, scheme) {
		return nil
	}
	body := dsn[len(scheme):]
	ai := strings.LastIndex(body, "@")
	if ai == -1 {
		return nil
	}
	userInfo := body[:ai]
	if strings.Contains(userInfo, "?") {
		return errors.New("达梦 DSN 的用户名/口令包含非法字符 '?'：驱动解析 DSN 时不做 percent-decode，以 '?' 作为 query 参数起始，口令携带该字符会被截断导致静默认证失败；dm:// 口令须按字面值写入（不要 percent-encode），请改用不含 '?' 的口令")
	}
	_, password, hasColon := strings.Cut(userInfo, ":")
	if !hasColon {
		return nil
	}
	if strings.Contains(password, "%") {
		common.SysLog("达梦 DSN 口令包含 '%'：驱动不做 percent-decode、口令按字面值传递，若该字符是 percent-encode 残留（如 '%40' 表示 '@'），请改写为原始口令")
	}
	return nil
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
