package dbmigrate

import (
	"fmt"
	"testing"

	"github.com/NookMux/NookMux/internal/store/db"
	"github.com/NookMux/NookMux/internal/store/option"
	"github.com/NookMux/NookMux/internal/store/token"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

// setupTokenKeyPrefixTestDB 建立带 tokens/options 表的内存 SQLite 库，
// 并将 dbstore.DB 指向该库，供密钥前缀迁移测试使用。
func setupTokenKeyPrefixTestDB(t *testing.T) {
	t.Helper()

	oldDB := dbstore.DB

	dbHandle, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite test db: %v", err)
	}
	if err := dbHandle.AutoMigrate(&tokenstore.Token{}, &optionstore.Option{}); err != nil {
		t.Fatalf("migrate sqlite test db: %v", err)
	}
	dbstore.DB = dbHandle

	t.Cleanup(func() {
		if sqlDB, err := dbHandle.DB(); err == nil {
			_ = sqlDB.Close()
		}
		dbstore.DB = oldDB
	})
}

func insertTokenRow(t *testing.T, userId int, key string) {
	t.Helper()
	row := tokenstore.Token{UserId: userId, Key: key, Name: "t", Status: 1, ExpiredTime: -1}
	if err := dbstore.DB.Create(&row).Error; err != nil {
		t.Fatalf("insert token %q: %v", key, err)
	}
}

func getTokenKey(t *testing.T, id int) string {
	t.Helper()
	var row tokenstore.Token
	if err := dbstore.DB.First(&row, "id = ?", id).Error; err != nil {
		t.Fatalf("load token %d: %v", id, err)
	}
	return row.Key
}

func TestMigrateTokenKeyPrefixesPrefixesLegacyKeys(t *testing.T) {
	setupTokenKeyPrefixTestDB(t)

	insertTokenRow(t, 1, "AbCdEf0123456789AbCdEf0123456789AbCdEf0123456789") // 历史无前缀 key
	insertTokenRow(t, 1, "sk-AbCdEf0123456789AbCdEf0123456789AbCdEf012345")  // 已带前缀
	insertTokenRow(t, 2, "XyZ0123456789AbCdEf0123456789AbCdEf0123456789AB")  // 历史无前缀 key

	if err := migrateTokenKeyPrefixes(); err != nil {
		t.Fatalf("migrateTokenKeyPrefixes: %v", err)
	}

	if got := getTokenKey(t, 1); got != "sk-AbCdEf0123456789AbCdEf0123456789AbCdEf0123456789" {
		t.Fatalf("legacy key not prefixed: %q", got)
	}
	if got := getTokenKey(t, 2); got != "sk-AbCdEf0123456789AbCdEf0123456789AbCdEf012345" {
		t.Fatalf("already-prefixed key changed: %q", got)
	}
	if got := getTokenKey(t, 3); got != "sk-XyZ0123456789AbCdEf0123456789AbCdEf0123456789AB" {
		t.Fatalf("legacy key not prefixed: %q", got)
	}
	if !optionstore.IsDataMigrationDone(tokenKeyPrefixMigrationMarker) {
		t.Fatal("migration marker not persisted")
	}

	// marker 已写入：重复执行必须直接跳过，不再触碰数据。
	insertTokenRow(t, 1, "ShouldNotBeTouched01234567890123456789012345678")
	if err := migrateTokenKeyPrefixes(); err != nil {
		t.Fatalf("second migrateTokenKeyPrefixes: %v", err)
	}
	if got := getTokenKey(t, 4); got != "ShouldNotBeTouched01234567890123456789012345678" {
		t.Fatalf("marker skipped run still mutated data: %q", got)
	}
}

// TestMigrateTokenKeyPrefixesIdempotentWithoutMarker 验证迁移自身的幂等性：
// marker 丢失时重复执行，已补前缀（含 '-'）的行天然跳过，不会二次加前缀。
func TestMigrateTokenKeyPrefixesIdempotentWithoutMarker(t *testing.T) {
	setupTokenKeyPrefixTestDB(t)

	insertTokenRow(t, 1, "AbCdEf0123456789AbCdEf0123456789AbCdEf0123456789")
	if err := migrateTokenKeyPrefixes(); err != nil {
		t.Fatalf("first run: %v", err)
	}

	// 模拟 marker 丢失后重跑。
	if err := dbstore.DB.Where(dbstore.CommonKeyCol+" = ?", tokenKeyPrefixMigrationMarker).
		Delete(&optionstore.Option{}).Error; err != nil {
		t.Fatalf("delete marker: %v", err)
	}
	if err := migrateTokenKeyPrefixes(); err != nil {
		t.Fatalf("second run: %v", err)
	}

	if got := getTokenKey(t, 1); got != "sk-AbCdEf0123456789AbCdEf0123456789AbCdEf0123456789" {
		t.Fatalf("key double-prefixed or changed: %q", got)
	}
}

// TestMigrateTokenKeyPrefixesEmptyTable 空表也应正常完成并写入 marker。
func TestMigrateTokenKeyPrefixesEmptyTable(t *testing.T) {
	setupTokenKeyPrefixTestDB(t)

	if err := migrateTokenKeyPrefixes(); err != nil {
		t.Fatalf("empty table run: %v", err)
	}
	if !optionstore.IsDataMigrationDone(tokenKeyPrefixMigrationMarker) {
		t.Fatal("migration marker not persisted on empty table")
	}
}
