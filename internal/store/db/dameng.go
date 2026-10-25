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
//   - 实例必须运行于 COMPATIBLE_MODE=0（DM 原生模式，dm.ini 服务器参数），
//     并在连接时校验：适配与实机验证均基于原生模式完成，Oracle/MySQL 等
//     兼容模式会改变内核语法与语义（整数除法、空串、标识符解析、MERGE
//     行为），属未验证路径；
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
	var compatMode string
	if err := gormDB.Raw("SELECT PARA_VALUE FROM V$DM_INI WHERE PARA_NAME = 'COMPATIBLE_MODE'").Row().Scan(&compatMode); err != nil {
		_ = closeDB(gormDB)
		return nil, fmt.Errorf("读取达梦 COMPATIBLE_MODE 配置失败: %w", err)
	}
	if compatMode != "0" {
		_ = closeDB(gormDB)
		return nil, fmt.Errorf("达梦实例 COMPATIBLE_MODE=%s（%s），NookMux 适配并验证的是 DM 原生模式（COMPATIBLE_MODE=0）：请在 dm.ini 中将 COMPATIBLE_MODE 置 0 并重启实例，或切换到符合要求的库", compatMode, dmCompatModeName(compatMode))
	}
	return gormDB, nil
}

// dmCompatModeName 返回 COMPATIBLE_MODE 取值对应的兼容模式名（仅用于
// 报错提示；0 为 DM 原生，无兼容目标）。
func dmCompatModeName(mode string) string {
	switch mode {
	case "1":
		return "SQL92 兼容"
	case "2":
		return "Oracle 兼容"
	case "3":
		return "SQL Server 兼容"
	case "4":
		return "MySQL 兼容"
	case "5":
		return "DM6 兼容"
	case "6":
		return "Teradata 兼容"
	case "7":
		return "PostgreSQL 兼容"
	case "8":
		return "DB2 兼容"
	default:
		return "未知模式"
	}
}

// validateDMDsnPassword 在建立连接前校验 dm:// DSN 用户名/口令中的非法
// 字符，切分规则与驱动 parseDSN（dm8/n.go）一致：去掉 dm:// 前缀后，先以
// 最后一个 '?' 剥离 query 段，再对剩余 URL 段按最后一个 '@' 切分 userinfo
// （query 值中的 '@' 不参与切分），userinfo 中第一个 ':' 之后的全部内容
// 为口令。
//
//   - 剥离 query 后的 URL 段中包含 '?' 时报错：口令携带 '?' 且其后存在
//     真正 query 参数时，最后一个 '?' 作为 query 起始、口令中的 '?' 残留
//     在 userinfo，会被当作口令字面值送认证（静默失败）；口令含 '?' 而
//     无 query 参数时，'?' 即 query 起始，userinfo 被截断后无 '@'，由驱动
//     以 DSN_INVALID_FORMAT 报错暴露；
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
	if qi := strings.LastIndex(body, "?"); qi >= 0 {
		body = body[:qi]
	}
	ai := strings.LastIndex(body, "@")
	if ai == -1 {
		// 剥离 query 后已无 '@'：最后一个 '?' 必然落在 userinfo/主机段内，
		// 典型形态是口令含 '?' 且 DSN 无真正 query（'?' 吞掉了主机段）。
		return errors.New("达梦 DSN 的用户名/口令包含非法字符 '?'：驱动解析 DSN 时不做 percent-decode，以最后一个 '?' 作为 query 参数起始，口令携带该字符时会被截断或当作字面值送认证；dm:// 口令须按字面值写入（不要 percent-encode），请改用不含 '?' 的口令")
	}
	userInfo := body[:ai]
	if strings.Contains(userInfo, "?") {
		return errors.New("达梦 DSN 的用户名/口令包含非法字符 '?'：驱动解析 DSN 时不做 percent-decode，以最后一个 '?' 作为 query 参数起始，口令携带该字符时会被截断或当作字面值送认证；dm:// 口令须按字面值写入（不要 percent-encode），请改用不含 '?' 的口令")
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
