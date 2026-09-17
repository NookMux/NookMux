package tokenstore

import (
	"fmt"
	"testing"

	"github.com/NookMux/NookMux/internal/infra/redis"
	"github.com/NookMux/NookMux/internal/store/db"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func setupTokenModelMappingTestDB(t *testing.T) func() {
	t.Helper()

	oldDB := dbstore.DB
	oldRedisEnabled := redis.RedisEnabled

	testDB, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite test db: %v", err)
	}
	if err := testDB.AutoMigrate(&Token{}); err != nil {
		t.Fatalf("migrate sqlite test db: %v", err)
	}

	dbstore.DB = testDB
	redis.RedisEnabled = false

	return func() {
		dbstore.DB = oldDB
		redis.RedisEnabled = oldRedisEnabled
	}
}

func TestTokenGetModelMapping(t *testing.T) {
	cases := []struct {
		name    string
		in      *string
		wantStr string
	}{
		{"nil 字段返回空串", nil, ""},
		{"空字符串保持空串", strPtr(""), ""},
		{"合法 JSON 原样返回", strPtr(`{"claude-3-5-sonnet": "glm-4-plus"}`), `{"claude-3-5-sonnet": "glm-4-plus"}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			token := Token{ModelMapping: tc.in}
			if got := token.GetModelMapping(); got != tc.wantStr {
				t.Fatalf("GetModelMapping() = %q, want %q", got, tc.wantStr)
			}
		})
	}
}

func TestTokenGetModelMappingMap(t *testing.T) {
	cases := []struct {
		name  string
		in    *string
		want  map[string]string
		isNil bool
	}{
		{"nil 字段返回 nil", nil, nil, true},
		{"空串返回 nil", strPtr(""), nil, true},
		{"空对象返回 nil", strPtr("{}"), nil, true},
		{"非法 JSON 返回 nil（防御透传）", strPtr(`{"a": `), nil, true},
		{"非对象 JSON 返回 nil", strPtr(`["a"]`), nil, true},
		{"值非字符串返回 nil", strPtr(`{"a": 1}`), nil, true},
		{"合法映射完整解析", strPtr(`{"a": "b", "c": "d"}`), map[string]string{"a": "b", "c": "d"}, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			token := Token{ModelMapping: tc.in}
			got := token.GetModelMappingMap()
			if tc.isNil {
				if got != nil {
					t.Fatalf("GetModelMappingMap() = %v, want nil", got)
				}
				return
			}
			if len(got) != len(tc.want) {
				t.Fatalf("GetModelMappingMap() = %v, want %v", got, tc.want)
			}
			for k, v := range tc.want {
				if got[k] != v {
					t.Fatalf("GetModelMappingMap()[%q] = %q, want %q", k, got[k], v)
				}
			}
		})
	}
}

// TestTokenModelMappingPersistence 验证 model_mapping 列在 SQLite 上的
// Insert/Select 读回与 Update() 列表更新均正常（迁移由 AutoMigrate 建立）。
func TestTokenModelMappingPersistence(t *testing.T) {
	cleanup := setupTokenModelMappingTestDB(t)
	defer cleanup()

	mapping := `{"claude-3-5-sonnet": "glm-4-plus", "claude-3-7-sonnet": "glm-4-plus"}`
	token := Token{
		UserId:         1,
		Key:            "mapping-key",
		Name:           "mapped",
		UnlimitedQuota: true,
		QuotaType:      0,
		ModelMapping:   strPtr(mapping),
	}
	if err := token.Insert(); err != nil {
		t.Fatalf("insert token: %v", err)
	}

	loaded, err := GetTokenByKey("mapping-key", true)
	if err != nil {
		t.Fatalf("load token by key: %v", err)
	}
	if got := loaded.GetModelMapping(); got != mapping {
		t.Fatalf("model_mapping not persisted: got %q, want %q", got, mapping)
	}
	gotMap := loaded.GetModelMappingMap()
	if gotMap["claude-3-5-sonnet"] != "glm-4-plus" {
		t.Fatalf("GetModelMappingMap() = %v, want glm mapping", gotMap)
	}

	// Update() 应更新 model_mapping 列（清空与修改两个方向）
	loaded.ModelMapping = strPtr(`{"a": "b"}`)
	if err := loaded.Update(); err != nil {
		t.Fatalf("update token: %v", err)
	}
	var after Token
	if err := dbstore.DB.First(&after, loaded.Id).Error; err != nil {
		t.Fatalf("reload token: %v", err)
	}
	if after.GetModelMapping() != `{"a": "b"}` {
		t.Fatalf("model_mapping not updated: got %q", after.GetModelMapping())
	}

	loaded.ModelMapping = nil
	if err := loaded.Update(); err != nil {
		t.Fatalf("clear token mapping: %v", err)
	}
	var cleared Token
	if err := dbstore.DB.First(&cleared, loaded.Id).Error; err != nil {
		t.Fatalf("reload cleared token: %v", err)
	}
	if cleared.GetModelMapping() != "" {
		t.Fatalf("model_mapping not cleared: got %q", cleared.GetModelMapping())
	}
}

// TestTokenModelMappingMigrationOnExistingTable 验证已存在（不含新列）的
// tokens 表经 AutoMigrate 补列后可正常写入 model_mapping。
func TestTokenModelMappingMigrationOnExistingTable(t *testing.T) {
	oldDB := dbstore.DB
	oldRedisEnabled := redis.RedisEnabled
	defer func() {
		dbstore.DB = oldDB
		redis.RedisEnabled = oldRedisEnabled
	}()

	testDB, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite test db: %v", err)
	}
	// 先按旧 schema（无 model_mapping 列）建表
	if err := testDB.Exec(`CREATE TABLE tokens (
		id integer primary key autoincrement,
		user_id integer, key text, status integer, name text,
		created_time bigint, accessed_time bigint, expired_time bigint,
		remain_quota integer, unlimited_quota numeric,
		model_limits_enabled numeric, model_limits text, allow_ips text,
		used_quota integer, "group" text, cross_group_retry numeric,
		quota_type integer, window_hours integer, window_quota integer,
		window_start_hour integer, cycle_days integer, cycle_quota integer,
		window_used_quota integer, window_start_time bigint,
		cycle_used_quota integer, cycle_start_time bigint,
		deleted_at bigint
	)`).Error; err != nil {
		t.Fatalf("create legacy table: %v", err)
	}
	// AutoMigrate 补列
	if err := testDB.AutoMigrate(&Token{}); err != nil {
		t.Fatalf("automigrate legacy table: %v", err)
	}

	dbstore.DB = testDB
	redis.RedisEnabled = false

	token := Token{
		UserId:         1,
		Key:            "legacy-key",
		Name:           "legacy",
		UnlimitedQuota: true,
		ModelMapping:   strPtr(`{"old-model": "new-model"}`),
	}
	if err := token.Insert(); err != nil {
		t.Fatalf("insert token on migrated table: %v", err)
	}
	loaded, err := GetTokenByKey("legacy-key", true)
	if err != nil {
		t.Fatalf("load token: %v", err)
	}
	if loaded.GetModelMapping() != `{"old-model": "new-model"}` {
		t.Fatalf("model_mapping lost after legacy migration: got %q", loaded.GetModelMapping())
	}
}

func strPtr(s string) *string { return &s }
