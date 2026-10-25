package db

import (
	"github.com/go-sql-driver/mysql"
	"testing"
)

func TestParseDBTypeFromDSN(t *testing.T) {
	tests := []struct {
		name    string
		dsn     string
		want    string
		wantErr bool
	}{
		{name: "postgres URL", dsn: "postgres://user:pass@127.0.0.1:5432/nookmux", want: DatabaseTypePostgreSQL},
		{name: "postgresql 官方别名", dsn: "postgresql://user:pass@127.0.0.1:5432/nookmux", want: DatabaseTypePostgreSQL},
		{name: "opengauss URL", dsn: "opengauss://user:pass@127.0.0.1:5432/nookmux", want: DatabaseTypeOpenGauss},
		{name: "opengauss URL 口令含特殊字符", dsn: "opengauss://user:p%2B%3A%3Fss@127.0.0.1:5432/nookmux?sslmode=disable", want: DatabaseTypeOpenGauss},
		{name: "mysql URL", dsn: "mysql://user:pass@127.0.0.1:3306/nookmux", want: DatabaseTypeMySQL},
		{name: "dm URL", dsn: "dm://user:pass@127.0.0.1:5236?schema=NOOKMUX", want: DatabaseTypeDM},
		{name: "dm URL 口令含@", dsn: "dm://user:p@ss@127.0.0.1:5236?schema=NOOKMUX", want: DatabaseTypeDM},
		{name: "kingbase URL", dsn: "kingbase://user:pass@127.0.0.1:54321/nookmux", want: DatabaseTypeKingbase},
		{name: "kingbase URL 口令 percent-encode", dsn: "kingbase://user:p%40ss@127.0.0.1:54321/nookmux?sslmode=disable", want: DatabaseTypeKingbase},
		{name: "允许首尾空白", dsn: "  mysql://user:pass@127.0.0.1:3306/nookmux\n", want: DatabaseTypeMySQL},
		{name: "裸 MySQL 驱动 DSN 非法", dsn: "user:pass@tcp(127.0.0.1:3306)/nookmux", wantErr: true},
		{name: "local 前缀非法", dsn: "local", wantErr: true},
		{name: "空串", dsn: "", wantErr: true},
		{name: "纯空白", dsn: "   ", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseDBTypeFromDSN(tt.dsn)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("ParseDBTypeFromDSN(%q) 期望报错，实际返回 %q", tt.dsn, got)
				}
				return
			}
			if err != nil {
				t.Fatalf("ParseDBTypeFromDSN(%q) 报错: %v", tt.dsn, err)
			}
			if got != tt.want {
				t.Fatalf("ParseDBTypeFromDSN(%q) = %q, 期望 %q", tt.dsn, got, tt.want)
			}
		})
	}
}

func TestMySQLDsnFromURL(t *testing.T) {
	tests := []struct {
		name    string
		dsnURL  string
		want    string
		wantErr bool
	}{
		{
			name:   "完整形态",
			dsnURL: "mysql://user:password@127.0.0.1:3306/nookmux",
			want:   "user:password@tcp(127.0.0.1:3306)/nookmux",
		},
		{
			name:   "无端口",
			dsnURL: "mysql://user:password@db-host/nookmux",
			want:   "user:password@tcp(db-host)/nookmux",
		},
		{
			name:   "无口令",
			dsnURL: "mysql://user@127.0.0.1:3306/nookmux",
			want:   "user@tcp(127.0.0.1:3306)/nookmux",
		},
		{
			name:   "保留查询参数",
			dsnURL: "mysql://user:password@127.0.0.1:3306/nookmux?charset=utf8mb4&parseTime=true",
			want:   "user:password@tcp(127.0.0.1:3306)/nookmux?charset=utf8mb4&parseTime=true",
		},
		{
			name:   "口令中的冒号 URL 解码",
			dsnURL: "mysql://user:p%3Ass@127.0.0.1:3306/nookmux",
			want:   "user:p:ss@tcp(127.0.0.1:3306)/nookmux",
		},
		{name: "缺少用户名", dsnURL: "mysql://127.0.0.1:3306/nookmux", wantErr: true},
		{name: "缺少主机", dsnURL: "mysql://user:password@/nookmux", wantErr: true},
		{name: "缺少数据库名", dsnURL: "mysql://user:password@127.0.0.1:3306", wantErr: true},
		{name: "数据库名含斜杠", dsnURL: "mysql://user:password@127.0.0.1:3306/a/b", wantErr: true},
		{name: "用户名含@", dsnURL: "mysql://u%40ser:password@127.0.0.1:3306/nookmux", wantErr: true},
		{name: "口令含@", dsnURL: "mysql://user:p%40ss@127.0.0.1:3306/nookmux", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := MySQLDsnFromURL(tt.dsnURL)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("MySQLDsnFromURL(%q) 期望报错，实际返回 %q", tt.dsnURL, got)
				}
				return
			}
			if err != nil {
				t.Fatalf("MySQLDsnFromURL(%q) 报错: %v", tt.dsnURL, err)
			}
			if got != tt.want {
				t.Fatalf("MySQLDsnFromURL(%q) = %q, 期望 %q", tt.dsnURL, got, tt.want)
			}
		})
	}
}

// TestMySQLDsnFromURLDriverRoundTrip 验证转换结果可被 go-sql-driver
// 正确解析回等价的连接配置。
func TestMySQLDsnFromURLDriverRoundTrip(t *testing.T) {
	driverDSN, err := MySQLDsnFromURL("mysql://user:password@127.0.0.1:3306/nookmux?charset=utf8mb4")
	if err != nil {
		t.Fatalf("MySQLDsnFromURL 报错: %v", err)
	}
	driverDSN = EnsureMySQLParseTime(driverDSN)
	cfg, err := mysql.ParseDSN(driverDSN)
	if err != nil {
		t.Fatalf("驱动解析转换结果失败: %v", err)
	}
	if cfg.User != "user" || cfg.Passwd != "password" {
		t.Fatalf("驱动解析 userinfo = %q:%q, 期望 user:password", cfg.User, cfg.Passwd)
	}
	if cfg.Addr != "127.0.0.1:3306" {
		t.Fatalf("驱动解析 addr = %q, 期望 127.0.0.1:3306", cfg.Addr)
	}
	if cfg.DBName != "nookmux" {
		t.Fatalf("驱动解析 dbname = %q, 期望 nookmux", cfg.DBName)
	}
	if !cfg.ParseTime {
		t.Fatal("转换结果未携带 parseTime=true")
	}
}

func TestEnsureMySQLParseTime(t *testing.T) {
	tests := []struct {
		name string
		dsn  string
		want string
	}{
		{name: "无参数", dsn: "user@tcp(host)/db", want: "user@tcp(host)/db?parseTime=true"},
		{name: "已有其他参数", dsn: "user@tcp(host)/db?charset=utf8mb4", want: "user@tcp(host)/db?charset=utf8mb4&parseTime=true"},
		{name: "已含 parseTime", dsn: "user@tcp(host)/db?parseTime=true", want: "user@tcp(host)/db?parseTime=true"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := EnsureMySQLParseTime(tt.dsn); got != tt.want {
				t.Fatalf("EnsureMySQLParseTime(%q) = %q, 期望 %q", tt.dsn, got, tt.want)
			}
		})
	}
}

// TestDSNQueryParam 验证仅解析 URL DSN 的 query 段判定参数是否显式携带：
// userinfo/口令中的同名字符串不构成携带，空值参数视为携带且值为空。
func TestDSNQueryParam(t *testing.T) {
	tests := []struct {
		name    string
		dsn     string
		key     string
		wantVal string
		wantCar bool
	}{
		{name: "显式携带", dsn: "kingbase://system:p%40ss@127.0.0.1:54321/nookmux?sslmode=disable&ora_input_emptystr_isnull=off", key: "ora_input_emptystr_isnull", wantVal: "off", wantCar: true},
		{name: "空值参数视为携带", dsn: "kingbase://system:pass@127.0.0.1:54321/nookmux?ora_input_emptystr_isnull=", key: "ora_input_emptystr_isnull", wantVal: "", wantCar: true},
		{name: "口令同名子串不算携带", dsn: "kingbase://system:xxora_input_emptystr_isnull=y@127.0.0.1:54321/nookmux", key: "ora_input_emptystr_isnull", wantVal: "", wantCar: false},
		{name: "未携带", dsn: "kingbase://system:pass@127.0.0.1:54321/nookmux", key: "sslmode", wantVal: "", wantCar: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotVal, gotCar := DSNQueryParam(tt.dsn, tt.key)
			if gotCar != tt.wantCar || gotVal != tt.wantVal {
				t.Fatalf("DSNQueryParam(%q, %q) = (%q, %v), 期望 (%q, %v)", tt.dsn, tt.key, gotVal, gotCar, tt.wantVal, tt.wantCar)
			}
		})
	}
}
