package dbstore

import (
	"database/sql"
	"fmt"
	"net/url"
	"strings"

	infradb "github.com/NookMux/NookMux/internal/infra/db"
	"github.com/godoes/gorm-kingbase"
	"gorm.io/gorm"
)

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
// =on 类取值时连接期直接拒绝（与 enable_ci 校验同为硬拒绝，不让破坏性
// 语义推迟到运行期以数据异常暴露）。
func OpenKingbaseGorm(dsn string, cfg *gorm.Config) (*gorm.DB, error) {
	if v, carried := infradb.DSNQueryParam(dsn, "ora_input_emptystr_isnull"); carried {
		switch strings.ToLower(strings.TrimSpace(v)) {
		case "on", "true", "1", "yes":
			return nil, fmt.Errorf("金仓 DSN 显式设置 ora_input_emptystr_isnull=%s 会把空字符串当 NULL（NOT NULL 列违约、等值过滤失配），破坏 NookMux 六库统一的空串与 NULL 严格区分语义；请移除该参数，连接层会自动下发 off", strings.TrimSpace(v))
		}
	}
	gormDB, err := gorm.Open(kingbase.New(kingbase.Config{
		DriverName: kingbase.DriverName,
		DSN:        EnsureKingbaseOraEmptyStrIsNotNull(dsn),
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
// SQLite/MySQL/PostgreSQL/openGauss/DM 的空串与 NULL 严格区分语义：未携带
// 或仅携带空值时补为 off（Gokb 的 URL 解析会整体丢弃空值参数，等价于未
// 携带）；显式携带有效值时原样保留（=on 类取值由 OpenKingbaseGorm 连接
// 前拒绝）。基于 net/url 仅读写 query 段，不受 userinfo/口令中同名子串
// 干扰。
func EnsureKingbaseOraEmptyStrIsNotNull(dsn string) string {
	const param = "ora_input_emptystr_isnull"
	if v, carried := infradb.DSNQueryParam(dsn, param); carried && v != "" {
		return dsn
	}
	u, err := url.Parse(dsn)
	if err != nil {
		return dsn
	}
	query := u.Query()
	query.Set(param, "off")
	u.RawQuery = query.Encode()
	return u.String()
}
