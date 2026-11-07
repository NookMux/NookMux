package dbstore

import (
	"database/sql"
	"errors"
	"fmt"
	"net/url"
	"strings"

	"github.com/godoes/gorm-kingbase"
	"gorm.io/gorm"
)

// kingbaseOraEmptyStrParam 空串语义 GUC 名。金仓 GUC 名服务端大小写不敏感，
// DSN 中任意大小写变体都会作用于同一 GUC，守卫与归一化必须按大小写不敏感
// 匹配全部取值。
const kingbaseOraEmptyStrParam = "ora_input_emptystr_isnull"

// OpenKingbaseGorm 统一封装金仓 KingbaseES 的 GORM 连接建立，使用
// gorm-kingbase 方言（内置金仓官方 Gokb 驱动，database/sql 驱动名
// "kingbase"，Migrator 基于 pg_catalog / information_schema 探测表结构）。
//
// DSN 采用 Gokb 驱动原生识别的 URL 形式：
//
//	kingbase://user:password@host:54321/dbname?sslmode=disable
//
// userinfo 与查询参数按标准 URL 规则 percent-encode（口令含 '@'、':' 等特殊
// 字符时须编码，如 '%40'）。除驱动已识别的连接参数外，URL 中其余键值对会
// 作为会话启动参数随连接下发给服务端。
//
// Oracle 兼容模式（目标部署形态）下金仓默认把输入的空字符串当作 NULL
// （GUC ora_input_emptystr_isnull 默认 on），而本项目的跨库语义是空串与
// NULL 严格区分：空串入列会在 NOT NULL 列上直接违反约束，且
// `WHERE col = ”` 类过滤条件因空串入库为 NULL 而永远失配。连接建立时由
// EnsureKingbaseOraEmptyStrIsNotNull 统一归一该参数为 off；DSN 显式携带
// =on 类取值（任意大小写变体、任意携带位置）时连接期直接拒绝（与 enable_ci
// 校验同为硬拒绝，不让破坏性语义推迟到运行期以数据异常暴露）。
func OpenKingbaseGorm(dsn string, cfg *gorm.Config) (*gorm.DB, error) {
	u, err := parseKingbaseURL(dsn)
	if err != nil {
		return nil, err
	}
	for _, v := range kingbaseGUCValues(u.Query(), kingbaseOraEmptyStrParam) {
		switch strings.ToLower(strings.TrimSpace(v)) {
		case "on", "true", "1", "yes":
			return nil, fmt.Errorf("金仓 DSN 显式设置 %s=%s 会把空字符串当 NULL（NOT NULL 列违约、等值过滤失配），破坏 NookMux 六库统一的空串与 NULL 严格区分语义；请移除该参数，连接层会自动下发 off", kingbaseOraEmptyStrParam, strings.TrimSpace(v))
		}
	}
	normalized := normalizeKingbaseOraEmptyStr(u)
	gormDB, err := gorm.Open(kingbase.New(kingbase.Config{
		DriverName: kingbase.DriverName,
		DSN:        normalized,
	}), cfg)
	if err != nil {
		return nil, err
	}
	// 校验失败分支必须关闭底层连接池再返回：预迁移是管理端 HTTP 运行时
	// 可反复触发的动作，句柄不关闭会持续泄漏；关闭失败以主错误为准
	// （复用包级 closeDB）。
	var enableCI sql.NullString
	if err := gormDB.Raw("SELECT current_setting('enable_ci', true)").Row().Scan(&enableCI); err != nil {
		_ = closeDB(gormDB)
		return nil, fmt.Errorf("读取金仓 enable_ci 配置失败: %w", err)
	}
	if strings.EqualFold(strings.TrimSpace(enableCI.String), "on") {
		_ = closeDB(gormDB)
		return nil, fmt.Errorf("金仓实例开启了 enable_ci（字符串比较大小写不敏感），令牌密钥、用户名等关键等值比较会跨大小写误匹配，NookMux 需要 enable_ci=off 的实例（该参数为实例级 initdb 选项，须在不携带 --enable-ci 的实例上运行）")
	}
	return gormDB, nil
}

// EnsureKingbaseOraEmptyStrIsNotNull 为 kingbase:// URL DSN 归一
// ora_input_emptystr_isnull 启动参数，对齐项目内
// SQLite/MySQL/PostgreSQL/openGauss/DM 的空串与 NULL 严格区分语义。仅解析
// query 段，不受 userinfo/口令中同名字符串干扰；URL 解析失败立即报错，不
// 放行未归一化的 DSN（空串即 NULL 语义必须在连接期杜绝）。
func EnsureKingbaseOraEmptyStrIsNotNull(dsn string) (string, error) {
	u, err := parseKingbaseURL(dsn)
	if err != nil {
		return "", err
	}
	return normalizeKingbaseOraEmptyStr(u), nil
}

// parseKingbaseURL 解析 kingbase:// DSN。解析失败即报错（net/url 对 userinfo
// 中非法 percent-escape 会在此暴露）；错误信息不回显 URL 原文，避免口令随
// 日志泄漏。
func parseKingbaseURL(dsn string) (*url.URL, error) {
	u, err := url.Parse(dsn)
	if err != nil {
		return nil, errors.New("解析 kingbase:// DSN 失败：URL 不合法（常见原因：口令未按 URL 规则 percent-encode，如 %2 应为 %2F；为避免口令泄漏不回显解析原文）")
	}
	return u, nil
}

// kingbaseGUCValues 返回 query 段中与 name 大小写不敏感匹配的全部取值。
// Gokb 对同名键后值覆盖（后写生效），守卫必须逐值检查而非只看首个。
func kingbaseGUCValues(query url.Values, name string) []string {
	var values []string
	for k, vs := range query {
		if strings.EqualFold(k, name) {
			values = append(values, vs...)
		}
	}
	return values
}

// normalizeKingbaseOraEmptyStr 把 query 段中 ora_input_emptystr_isnull 的全部
// 大小写变体收敛为单个小写键：仅在唯一键、唯一取值且非空时原样保留
// （Gokb 会整体丢弃空值参数，等价于未携带，须补为 off）；否则（未携带、
// 空值、重复携带或多个大小写变体）统一改写为单一 off，避免多个变体共存时
// 由驱动按 map 顺序随机生效。=on 类取值由 OpenKingbaseGorm 在归一化前拒绝。
func normalizeKingbaseOraEmptyStr(u *url.URL) string {
	query := u.Query()
	values := kingbaseGUCValues(query, kingbaseOraEmptyStrParam)
	if len(values) == 1 && values[0] != "" {
		return u.String()
	}
	for k := range query {
		if strings.EqualFold(k, kingbaseOraEmptyStrParam) {
			query.Del(k)
		}
	}
	query.Set(kingbaseOraEmptyStrParam, "off")
	u.RawQuery = query.Encode()
	return u.String()
}
