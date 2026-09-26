package optionstore

import (
	"fmt"
	"github.com/NookMux/NookMux/internal/common"
	"github.com/NookMux/NookMux/internal/config/operation"
	"github.com/NookMux/NookMux/internal/store/db"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"strings"
	"testing"
)

func TestLoadOptionsMigratesToolBillingRulesAndRefreshesRuntimeConfig(t *testing.T) {
	oldDB := dbstore.DB
	oldRules := append([]operation.ToolBillingRule(nil), operation.GetToolBillingRules()...)
	common.OptionMapRWMutex.Lock()
	oldOptionMap := common.OptionMap
	common.OptionMap = map[string]string{}
	common.OptionMapRWMutex.Unlock()

	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite test db: %v", err)
	}
	if err := db.AutoMigrate(&Option{}); err != nil {
		t.Fatalf("migrate option table: %v", err)
	}
	dbstore.DB = db

	t.Cleanup(func() {
		if sqlDB, err := db.DB(); err == nil {
			_ = sqlDB.Close()
		}
		dbstore.DB = oldDB
		operation.UpdateToolBillingRules(oldRules)
		common.OptionMapRWMutex.Lock()
		common.OptionMap = oldOptionMap
		common.OptionMapRWMutex.Unlock()
	})

	legacyRules := `[
		{
			"id": "legacy_web_search_exact",
			"name": "Legacy Web Search Exact",
			"tool_type": "web_search",
			"billing_mode": "per_call",
			"price": 0.02,
			"model_filter": "gpt-4o",
			"provider": "openai",
			"enabled": true
		}
	]`
	if err := dbstore.DB.Create(&Option{Key: "tool_billing_setting.rules", Value: legacyRules}).Error; err != nil {
		t.Fatalf("insert legacy option: %v", err)
	}

	loadOptionsFromDatabase()

	price, ok := operation.GetToolBillingPrice("web_search", map[string]string{
		"model":    "gpt-4o",
		"provider": "openai",
	})
	if !ok || price != 0.02 {
		t.Fatalf("migrated runtime rule should match gpt-4o with price 0.02, got ok=%v price=%v", ok, price)
	}

	_, ok = operation.GetToolBillingPrice("web_search", map[string]string{
		"model":    "gpt-4o-mini",
		"provider": "openai",
	})
	if ok {
		t.Fatal("migrated exact legacy model_filter should not match gpt-4o-mini")
	}

	var stored Option
	if err := dbstore.DB.First(&stored, "key = ?", "tool_billing_setting.rules").Error; err != nil {
		t.Fatalf("query migrated option: %v", err)
	}
	if strings.Contains(stored.Value, "model_filter") || !strings.Contains(stored.Value, "conditions") {
		t.Fatalf("stored option was not migrated to conditions format: %s", stored.Value)
	}
}

func TestMigrateLegacyToolBillingRulesInOptionsBeforeRuntimeLoad(t *testing.T) {
	oldRules := append([]operation.ToolBillingRule(nil), operation.GetToolBillingRules()...)
	common.OptionMapRWMutex.Lock()
	oldOptionMap := common.OptionMap
	common.OptionMap = map[string]string{}
	common.OptionMapRWMutex.Unlock()

	t.Cleanup(func() {
		operation.UpdateToolBillingRules(oldRules)
		common.OptionMapRWMutex.Lock()
		common.OptionMap = oldOptionMap
		common.OptionMapRWMutex.Unlock()
	})

	options := []*Option{{
		Key: "tool_billing_setting.rules",
		Value: `[
			{
				"id": "legacy_web_search_exact",
				"name": "Legacy Web Search Exact",
				"tool_type": "web_search",
				"billing_mode": "per_call",
				"price": 0.02,
				"model_filter": "gpt-4o",
				"provider": "openai",
				"enabled": true
			}
		]`,
	}}

	didMigrate, _ := migrateLegacyToolBillingRulesInOptions(options)
	if !didMigrate {
		t.Fatal("expected legacy tool billing rules to migrate before runtime load")
	}
	if strings.Contains(options[0].Value, "model_filter") || !strings.Contains(options[0].Value, "conditions") {
		t.Fatalf("option value was not migrated before runtime load: %s", options[0].Value)
	}
	if err := updateOptionMap(options[0].Key, options[0].Value); err != nil {
		t.Fatalf("load migrated tool billing rules: %v", err)
	}

	_, ok := operation.GetToolBillingPrice("web_search", map[string]string{
		"model":    "gpt-4o-mini",
		"provider": "openai",
	})
	if ok {
		t.Fatal("legacy exact model_filter should not become an unconditional runtime rule")
	}
}

// TestUpdateOptionValidatesPayAddressScheme 保证易支付网关地址保存时强制 https：
// 非 https 值必须被拒绝且不落库、不更新运行时配置，https 与空值（未启用）正常保存。
func TestUpdateOptionValidatesPayAddressScheme(t *testing.T) {
	oldDB := dbstore.DB
	oldPayAddress := operation.PayAddress
	common.OptionMapRWMutex.Lock()
	oldOptionMap := common.OptionMap
	common.OptionMap = map[string]string{}
	common.OptionMapRWMutex.Unlock()

	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite test db: %v", err)
	}
	if err := db.AutoMigrate(&Option{}); err != nil {
		t.Fatalf("migrate option table: %v", err)
	}
	dbstore.DB = db

	t.Cleanup(func() {
		if sqlDB, err := db.DB(); err == nil {
			_ = sqlDB.Close()
		}
		dbstore.DB = oldDB
		operation.PayAddress = oldPayAddress
		common.OptionMapRWMutex.Lock()
		common.OptionMap = oldOptionMap
		common.OptionMapRWMutex.Unlock()
	})

	insecureAddresses := []string{
		"http://pay.example.com",
		"ftp://pay.example.com",
		"pay.example.com",
	}
	for _, addr := range insecureAddresses {
		if err := UpdateOption("PayAddress", addr); err == nil {
			t.Fatalf("UpdateOption(PayAddress, %q) expected error, got nil", addr)
		} else if !strings.Contains(err.Error(), "https") {
			t.Fatalf("UpdateOption(PayAddress, %q) error should mention https requirement, got %q", addr, err.Error())
		}
		var stored Option
		if err := dbstore.DB.First(&stored, "key = ?", "PayAddress").Error; err == nil {
			t.Fatalf("insecure PayAddress %q must not be persisted, found row value %q", addr, stored.Value)
		}
		if operation.PayAddress != "" {
			t.Fatalf("rejected PayAddress %q must not update runtime config, got %q", addr, operation.PayAddress)
		}
	}

	if err := UpdateOption("PayAddress", "https://pay.example.com"); err != nil {
		t.Fatalf("UpdateOption(PayAddress, https) error = %v", err)
	}
	if operation.PayAddress != "https://pay.example.com" {
		t.Fatalf("operation.PayAddress = %q, want https://pay.example.com", operation.PayAddress)
	}
	var stored Option
	if err := dbstore.DB.First(&stored, "key = ?", "PayAddress").Error; err != nil {
		t.Fatalf("query persisted PayAddress: %v", err)
	}
	if stored.Value != "https://pay.example.com" {
		t.Fatalf("persisted PayAddress = %q, want https://pay.example.com", stored.Value)
	}

	// 空值是允许的初始态，表示未启用易支付
	if err := UpdateOption("PayAddress", ""); err != nil {
		t.Fatalf("UpdateOption(PayAddress, empty) error = %v", err)
	}
	if operation.PayAddress != "" {
		t.Fatalf("operation.PayAddress = %q, want empty", operation.PayAddress)
	}
}
