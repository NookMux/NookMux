// openGauss 冒烟测试：将 openGauss 适配评审的一次性实测固化为仓库内可重复
// 执行的用例集，走 NookMux 生产连接路径（官方 connector 的 database/sql 连接
// 注入 GORM postgres 方言），覆盖连接认证、AutoMigrate 幂等与 ALTER、
// RETURNING 自增回填、序列探测与 setval、整数除法/FLOOR 分桶语义、
// ON CONFLICT 与事务内 FOR UPDATE。
//
// 用法：export OG_SMOKE_DSN=... && bash scripts/og-smoke.sh
// 所有表使用 og_smoke_ 前缀，正常与失败路径均尽力 DROP 清理；
// 任一步失败即非零退出。
package main

import (
	"database/sql"
	"errors"
	"fmt"
	"net/url"
	"os"
	"strings"
	"time"

	_ "gitcode.com/opengauss/openGauss-connector-go-pq" // 注册驱动名 "opengauss"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// tablePrefix 统一表前缀：结束时按前缀扫描 public schema 并逐一 DROP，
// 保证库内无残留；前缀同时是防误删护栏（只清理本程序创建的表）。
const tablePrefix = "og_smoke_"

// SmokeUser 复刻项目实体的典型类型面：自增主键、TEXT 持久化 JSON、bool、
// int64、可空字符串、唯一索引与普通索引。
type SmokeUser struct {
	ID          uint   `gorm:"primaryKey;autoIncrement"`
	Username    string `gorm:"size:64;uniqueIndex"`
	Extra       string `gorm:"type:text"`
	Enabled     bool
	Quota       int64
	AccessToken *string `gorm:"size:128;index"`
	CreatedAt   int64
}

func (SmokeUser) TableName() string { return tablePrefix + "users" }

// SmokeUserV2 模拟表结构演进后的模型：扩宽唯一列并新增备注列（ALTER 路径）。
type SmokeUserV2 struct {
	ID          uint   `gorm:"primaryKey;autoIncrement"`
	Username    string `gorm:"size:128;uniqueIndex"`
	Extra       string `gorm:"type:text"`
	Enabled     bool
	Quota       int64
	AccessToken *string `gorm:"size:128;index"`
	CreatedAt   int64
	Remark      string `gorm:"size:200"`
}

func (SmokeUserV2) TableName() string { return tablePrefix + "users" }

// SmokeLog 覆盖保留字列名（group）与普通索引。
type SmokeLog struct {
	ID        uint   `gorm:"primaryKey;autoIncrement"`
	GroupName string `gorm:"column:group;size:64"`
	ModelName string `gorm:"size:128;index"`
	Tokens    int64
	CreatedAt int64
}

func (SmokeLog) TableName() string { return tablePrefix + "logs" }

var gdb *gorm.DB

func step(name string, fn func() error) {
	start := time.Now()
	if err := fn(); err != nil {
		fmt.Printf("FAIL  %-44s %v\n", name, err)
		cleanupBestEffort()
		os.Exit(1)
	}
	fmt.Printf("PASS  %-44s (%s)\n", name, time.Since(start).Round(time.Millisecond))
}

// cleanup 按前缀扫描 public schema 并逐一 DROP；正常路径作为末尾步骤，
// 失败路径由 step 兜底调用。表名刚从 catalog 读出且带本程序前缀，
// 经 %q 标识符引用后拼接是安全的。
func cleanup() error {
	var tables []string
	if err := gdb.Raw(
		"SELECT tablename FROM pg_tables WHERE schemaname = ? AND tablename LIKE ?",
		"public", tablePrefix+"%",
	).Scan(&tables).Error; err != nil {
		return err
	}
	for _, t := range tables {
		if err := gdb.Exec(fmt.Sprintf("DROP TABLE IF EXISTS %q CASCADE", t)).Error; err != nil {
			return fmt.Errorf("DROP %s: %w", t, err)
		}
		fmt.Printf("      已清理表 %s\n", t)
	}
	if len(tables) == 0 {
		fmt.Printf("      库内无 %s 前缀表\n", tablePrefix)
	}
	return nil
}

func cleanupBestEffort() {
	if gdb == nil {
		return
	}
	if err := cleanup(); err != nil {
		fmt.Printf("WARN  清理失败，库中可能有 %s 前缀残留表，请手动 DROP: %v\n", tablePrefix, err)
	}
}

func main() {
	dsn := os.Getenv("OG_SMOKE_DSN")
	if dsn == "" {
		fmt.Fprintln(os.Stderr, "FATAL: 缺少环境变量 OG_SMOKE_DSN（必须指向 openGauss 实例，无替代数据库）")
		fmt.Fprintln(os.Stderr, `示例: export OG_SMOKE_DSN='opengauss://nookmux:<口令>@127.0.0.1:5433/nookmux_pg?sslmode=disable'`)
		os.Exit(1)
	}

	// ---- 1. 连接：URL DSN + 官方 connector ----
	//
	// openGauss 默认的 sha256 口令认证是私有协议，pgx/libpq 无法完成认证，
	// 必须使用官方 connector 注册的 "opengauss" 驱动建立 database/sql 连接。
	//
	// postgres 方言必须显式携带 DriverName："opengauss"。其 migrator 的表结构
	// 探测路径（GetRows/ColumnTypes）在 DriverName 为空（默认按 pgx 对待）且
	// 未启用 PrepareStmt 时，会向查询参数头部注入 pgx 私有的 simple-protocol
	// 开关值；官方 connector 不接受该参数，导致第二次 AutoMigrate 即报错。
	// 显式声明驱动名后方言按 database/sql 通用路径处理，不再注入。
	step("1 URL DSN 连接(官方 connector)", func() error {
		sqlDB, err := sql.Open("opengauss", dsn)
		if err != nil {
			return err
		}
		g, err := gorm.Open(postgres.New(postgres.Config{Conn: sqlDB, DriverName: "opengauss"}), &gorm.Config{})
		if err != nil {
			return err
		}
		gdb = g
		return nil
	})

	// 错误口令必须被拒绝：sha256 认证失败路径需暴露，不得静默放行。
	step("1b 错误口令被拒绝(负向)", func() error {
		u, err := url.Parse(dsn)
		if err != nil {
			// url.Parse 的错误原文内嵌整条 DSN（含口令），随 FAIL 行打印会
			// 泄漏凭据，不回显原文。
			return errors.New("解析 OG_SMOKE_DSN 失败：URL 不合法（常见原因：口令未按 URL 规则 percent-encode，如 %2 应为 %2F）")
		}
		if u.Scheme != "opengauss" || u.User == nil {
			return fmt.Errorf("OG_SMOKE_DSN 须为 opengauss://user:pass@host:port/db 形式的 URL DSN")
		}
		u.User = url.UserPassword(u.User.Username(), "og-smoke-wrong-password")
		sqlDB, err := sql.Open("opengauss", u.String())
		if err != nil {
			return err
		}
		defer sqlDB.Close()
		if err := sqlDB.Ping(); err == nil {
			return fmt.Errorf("错误口令竟然连接成功（认证失效）")
		}
		return nil
	})

	// ---- 2. AutoMigrate：首次建表、二次幂等、加列/扩宽 ALTER ----
	step("2a AutoMigrate 首次建表(含保留字列)", func() error {
		return gdb.AutoMigrate(&SmokeUser{}, &SmokeLog{})
	})
	step("2b 写入种子行", func() error {
		return gdb.Create(&SmokeUser{Username: "seed", Extra: "x", Enabled: true, CreatedAt: time.Now().Unix()}).Error
	})
	step("2c 二次 AutoMigrate 幂等", func() error {
		return gdb.AutoMigrate(&SmokeUser{}, &SmokeLog{})
	})
	step("2d 三次 AutoMigrate 加列/扩宽(ALTER)", func() error {
		return gdb.AutoMigrate(&SmokeUserV2{}, &SmokeLog{})
	})
	step("2e 新列可写读", func() error {
		if err := gdb.Model(&SmokeUserV2{}).Where("username = ?", "seed").Update("remark", "hello").Error; err != nil {
			return err
		}
		var got SmokeUserV2
		if err := gdb.Where("username = ?", "seed").First(&got).Error; err != nil {
			return err
		}
		if got.Remark != "hello" {
			return fmt.Errorf("remark 读回不一致: %q", got.Remark)
		}
		return nil
	})

	// ---- 3. 写入/读取：RETURNING 自增回填、批量、参数化 UPDATE/DELETE ----
	step("3a INSERT RETURNING 自增回填", func() error {
		token := "sk-og-smoke-token-1"
		u := SmokeUser{Username: "alice", Extra: `{"plan":"pro","tags":["a","b"]}`,
			Enabled: true, Quota: 100, AccessToken: &token, CreatedAt: time.Now().Unix()}
		if err := gdb.Create(&u).Error; err != nil {
			return err
		}
		if u.ID == 0 {
			return fmt.Errorf("自增主键未回填（RETURNING 未生效）")
		}
		return nil
	})
	step("3b 批量插入 CreateInBatches", func() error {
		var rows []SmokeUser
		for i := 0; i < 100; i++ {
			rows = append(rows, SmokeUser{Username: fmt.Sprintf("u%03d", i),
				Extra: `{"i":` + fmt.Sprint(i) + `}`, Enabled: i%2 == 0, Quota: int64(i), CreatedAt: time.Now().Unix()})
		}
		return gdb.CreateInBatches(rows, 50).Error
	})
	step("3c 读回 TEXT-JSON 与布尔", func() error {
		var rt SmokeUserV2
		if err := gdb.Where("username = ?", "alice").First(&rt).Error; err != nil {
			return err
		}
		if rt.Extra != `{"plan":"pro","tags":["a","b"]}` || !rt.Enabled || rt.Quota != 100 {
			return fmt.Errorf("读回数据不一致: %+v", rt)
		}
		return nil
	})
	step("3d 参数化 UPDATE/DELETE", func() error {
		res := gdb.Model(&SmokeUserV2{}).Where("username = ?", "alice").Update("quota", 200)
		if res.Error != nil || res.RowsAffected != 1 {
			return fmt.Errorf("update affected=%d err=%v", res.RowsAffected, res.Error)
		}
		res = gdb.Where("username LIKE ?", "u09%").Delete(&SmokeUserV2{})
		if res.Error != nil || res.RowsAffected != 10 {
			return fmt.Errorf("delete affected=%d err=%v", res.RowsAffected, res.Error)
		}
		return nil
	})

	// ---- 4. 迁移链路 SQL：序列探测 + setval 修复 ----
	// openGauss 序列的 pg_class.relkind 为 'z'（PostgreSQL 为 'S'），
	// 探测必须同时覆盖两者，否则迁移后的序列修复会空跑。
	step("4a 序列探测(relkind S/z + deptype a)", func() error {
		const q = `SELECT ns.nspname AS schema_name, s.relname AS sequence_name,
	t.relname AS table_name, a.attname AS column_name
FROM pg_class s
JOIN pg_namespace ns ON ns.oid = s.relnamespace
JOIN pg_depend d ON d.objid = s.oid
JOIN pg_class t ON t.oid = d.refobjid
JOIN pg_attribute a ON a.attrelid = t.oid AND a.attnum = d.refobjsubid
WHERE s.relkind IN ('S', 'z') AND d.deptype = 'a'
ORDER BY ns.nspname, t.relname, a.attname`
		type row struct {
			SchemaName   string
			SequenceName string
			TableName    string
			ColumnName   string
		}
		var rows []row
		if err := gdb.Raw(q).Scan(&rows).Error; err != nil {
			return err
		}
		found := map[string]bool{}
		for _, r := range rows {
			if r.TableName == tablePrefix+"users" || r.TableName == tablePrefix+"logs" {
				found[r.TableName+"."+r.ColumnName] = true
			}
		}
		fmt.Printf("      命中: %v\n", found)
		if !found[tablePrefix+"users.id"] || !found[tablePrefix+"logs.id"] {
			return fmt.Errorf("新建自增表的序列未被探测到，迁移后序列修复将空跑")
		}
		return nil
	})
	step("4b setval(?::regclass, ?, ?)", func() error {
		return gdb.Exec("SELECT setval(?::regclass, ?, ?)", "public."+tablePrefix+"users_id_seq", int64(5000), true).Error
	})
	step("4c setval 后插入取号无冲突", func() error {
		u := SmokeUserV2{Username: "post-setval", CreatedAt: time.Now().Unix()}
		if err := gdb.Create(&u).Error; err != nil {
			return err
		}
		if u.ID != 5001 {
			return fmt.Errorf("setval 后新行 ID=%d，应为 5001（setval 未生效）", u.ID)
		}
		return nil
	})

	// ---- 5. 整数除法语义与 FLOOR 分桶 ----
	// openGauss 的整数相除返回 double precision（不截断），与 PG 的整型截断
	// 不同；时间分桶等截断语义必须显式 FLOOR。
	step("5a pg_typeof(7/2) 为 double precision", func() error {
		var typ string
		if err := gdb.Raw("SELECT pg_typeof(7/2)::text || '=' || (7/2)::text").Scan(&typ).Error; err != nil {
			return err
		}
		fmt.Printf("      7/2 => %s\n", typ)
		if !strings.HasPrefix(typ, "double precision=") {
			return fmt.Errorf("整数除法类型为 %q，应为 double precision", typ)
		}
		return nil
	})
	// 5000 行 created_at = 1700000000 + i（秒）在小数 800 秒处起跨、i=2800 处
	// 跨过小时边界：裸除法（double 不截断）每行一桶，FLOOR 后聚合为 2 个
	// 整数对齐桶。
	step("5b FLOOR 桶数=2 / 裸除法桶数=行数", func() error {
		var rows []SmokeLog
		for i := 0; i < 5000; i++ {
			rows = append(rows, SmokeLog{GroupName: "g1", ModelName: "m1", Tokens: int64(i), CreatedAt: int64(1700000000 + i)})
		}
		if err := gdb.CreateInBatches(rows, 500).Error; err != nil {
			return err
		}
		var nRows, nRaw, nFloor int64
		if err := gdb.Raw(fmt.Sprintf(
			"SELECT count(*), count(DISTINCT (created_at / 3600) * 3600), count(DISTINCT FLOOR(created_at / 3600) * 3600) FROM %s",
			tablePrefix+"logs",
		)).Row().Scan(&nRows, &nRaw, &nFloor); err != nil {
			return err
		}
		fmt.Printf("      行数=%d 裸除法桶数=%d FLOOR桶数=%d\n", nRows, nRaw, nFloor)
		if nFloor != 2 {
			return fmt.Errorf("FLOOR 桶数=%d，应为 2（时间对齐被破坏）", nFloor)
		}
		if nRaw != nRows {
			return fmt.Errorf("裸除法桶数=%d，应为行数 %d（整数除法未按 double precision 语义）", nRaw, nRows)
		}
		return nil
	})
	// FLOOR 桶表达式继承整数除法的 double precision 类型，驱动以 float64
	// 回传；断言取值正确（min=1700000000 → 桶 472222*3600 = 1699999200）。
	step("5c FLOOR 桶值取数(double precision)", func() error {
		var bucket float64
		if err := gdb.Raw(fmt.Sprintf(
			"SELECT FLOOR(min(created_at) / 3600) * 3600 FROM %s", tablePrefix+"logs"),
		).Scan(&bucket).Error; err != nil {
			return err
		}
		if bucket != 1699999200 {
			return fmt.Errorf("FLOOR 桶值=%v，应为 1699999200", bucket)
		}
		return nil
	})

	// ---- 6. ON CONFLICT 与事务内 FOR UPDATE ----
	step("6a ON CONFLICT DO NOTHING", func() error {
		var before, after int64
		if err := gdb.Model(&SmokeUserV2{}).Count(&before).Error; err != nil {
			return err
		}
		err := gdb.Clauses(clause.OnConflict{DoNothing: true}).
			Create(&SmokeUserV2{Username: "alice", Extra: "dup", CreatedAt: time.Now().Unix()}).Error
		if err != nil {
			return err
		}
		if err := gdb.Model(&SmokeUserV2{}).Count(&after).Error; err != nil {
			return err
		}
		if after != before {
			return fmt.Errorf("冲突插入后行数 %d != %d（DO NOTHING 未生效）", after, before)
		}
		return nil
	})
	step("6b 事务内 FOR UPDATE + 提交", func() error {
		err := gdb.Transaction(func(tx *gorm.DB) error {
			var u SmokeUserV2
			if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
				Where("username = ?", "alice").First(&u).Error; err != nil {
				return err
			}
			return tx.Model(&SmokeUserV2{}).Where("id = ?", u.ID).Update("quota", 333).Error
		})
		if err != nil {
			return err
		}
		var rt SmokeUserV2
		if err := gdb.Where("username = ?", "alice").First(&rt).Error; err != nil {
			return err
		}
		if rt.Quota != 333 {
			return fmt.Errorf("事务提交后 quota=%d，应为 333", rt.Quota)
		}
		return nil
	})

	// ---- 7. 清理 ----
	step("7 DROP 清理 og_smoke_* 表", cleanup)

	fmt.Println("ALL PASS")
}
