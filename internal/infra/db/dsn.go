package db

import (
	"errors"
	"fmt"
	"net/url"
	"strings"
)

// ParseDBTypeFromDSN 按 DSN 前缀识别数据库类型，供主库/日志库初始化与
// 数据库迁移链路共用。
//
// 仅接受 URL 形式的 DSN：postgres://（含官方别名 postgresql://）、
// opengauss://（openGauss 官方 connector 原生支持的 URL scheme）、mysql://
// 与 dm://（达梦官方 Go 驱动原生支持的 URL scheme）、kingbase://（金仓
// Gokb 驱动原生支持的 URL scheme），其余前缀一律报错，
// 不按 "非 postgres 即 MySQL" 兜底猜测；SQLite 不通过 DSN 前缀表达，
// 由调用方在 DSN 为空时启用。
func ParseDBTypeFromDSN(dsn string) (string, error) {
	dsn = strings.TrimSpace(dsn)
	switch {
	case dsn == "":
		return "", errors.New("DSN 不能为空")
	case strings.HasPrefix(dsn, "postgres://"), strings.HasPrefix(dsn, "postgresql://"):
		return DatabaseTypePostgreSQL, nil
	case strings.HasPrefix(dsn, "opengauss://"):
		return DatabaseTypeOpenGauss, nil
	case strings.HasPrefix(dsn, "mysql://"):
		return DatabaseTypeMySQL, nil
	case strings.HasPrefix(dsn, "dm://"):
		return DatabaseTypeDM, nil
	case strings.HasPrefix(dsn, "kingbase://"):
		return DatabaseTypeKingbase, nil
	default:
		return "", errors.New("无法识别的 DSN：仅支持 postgres://、postgresql://、opengauss://、mysql://、dm:// 与 kingbase:// 前缀")
	}
}

// MySQLDsnFromURL 将 mysql:// URL 形式的 DSN 转换为 go-sql-driver/mysql
// 识别的标准 DSN 格式：
//
//	mysql://user:password@host:3306/dbname?charset=utf8mb4
//	-> user:password@tcp(host:3306)/dbname?charset=utf8mb4
//
// userinfo 按解码后的字面值拼接：用户名不得包含 ':' 或 '@'、口令不得包含
// '@'，这些字符在驱动的 user:pass@tcp(...) 结构中无法表达，直接报错而非
// 产生错误连接。
func MySQLDsnFromURL(dsnURL string) (string, error) {
	u, err := url.Parse(dsnURL)
	if err != nil {
		return "", fmt.Errorf("MySQL DSN 解析失败: %v", err)
	}
	if u.User == nil {
		return "", errors.New("MySQL DSN 缺少用户名，正确格式为 mysql://user:password@host:3306/dbname")
	}
	username := u.User.Username()
	password, hasPassword := u.User.Password()
	if strings.ContainsAny(username, ":@") {
		return "", errors.New("MySQL DSN 用户名包含非法字符 ':' 或 '@'")
	}
	if strings.Contains(password, "@") {
		return "", errors.New("MySQL DSN 口令包含非法字符 '@'，无法转换为驱动 DSN")
	}
	if u.Host == "" {
		return "", errors.New("MySQL DSN 缺少主机地址，正确格式为 mysql://user:password@host:3306/dbname")
	}
	authPart := username
	if hasPassword {
		authPart += ":" + password
	}
	dbName := strings.TrimPrefix(u.Path, "/")
	if dbName == "" {
		return "", errors.New("MySQL DSN 缺少数据库名，正确格式为 mysql://user:password@host:3306/dbname")
	}
	if strings.Contains(dbName, "/") {
		return "", errors.New("MySQL DSN 数据库名不能包含 '/'")
	}
	dsn := fmt.Sprintf("%s@tcp(%s)/%s", authPart, u.Host, dbName)
	if u.RawQuery != "" {
		dsn += "?" + u.RawQuery
	}
	return dsn, nil
}

// EnsureMySQLParseTime 为驱动 DSN 补齐 parseTime=true 参数（GORM 扫描
// time.Time 列需要），已包含 parseTime 时原样返回。
func EnsureMySQLParseTime(dsn string) string {
	if strings.Contains(dsn, "parseTime") {
		return dsn
	}
	if strings.Contains(dsn, "?") {
		return dsn + "&parseTime=true"
	}
	return dsn + "?parseTime=true"
}

// DSNQueryParam 返回 URL 形式 DSN query 段中指定参数的首个值与是否显式
// 携带。仅解析 query 段，不受 userinfo/口令/主机中同名字符串干扰；形如
// ?key= 的空值参数视为携带、值为空。
func DSNQueryParam(dsn string, key string) (string, bool) {
	u, err := url.Parse(dsn)
	if err != nil {
		return "", false
	}
	values := u.Query()[key]
	if len(values) == 0 {
		return "", false
	}
	return values[0], true
}
