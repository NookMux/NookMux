package helper

import (
	"fmt"
	configmodel "github.com/NookMux/NookMux/internal/config/model"
	dbstore "github.com/NookMux/NookMux/internal/store/db"
	voicestore "github.com/NookMux/NookMux/internal/store/voice"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"testing"
	"time"
)

// setupVoiceResolveTestDB 初始化内存 SQLite 并注册 voices 表，测试结束恢复全局 DB。
func setupVoiceResolveTestDB(t *testing.T) {
	t.Helper()

	oldDB := dbstore.DB
	gdb, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite test db: %v", err)
	}
	if err := gdb.AutoMigrate(&voicestore.Voice{}); err != nil {
		t.Fatalf("migrate sqlite test db: %v", err)
	}
	dbstore.DB = gdb

	t.Cleanup(func() {
		if sqlDB, err := gdb.DB(); err == nil {
			_ = sqlDB.Close()
		}
		dbstore.DB = oldDB
	})
}

// setVoiceWhitelist 设置音色白名单总开关，测试结束恢复原值。
func setVoiceWhitelist(t *testing.T, enabled bool) {
	t.Helper()

	old := configmodel.IsMiniMaxVoiceWhitelistEnabled()
	applyVoiceWhitelist := func(value bool) {
		if err := configmodel.WithMiniMaxSettingsWriteLock(func() error {
			configmodel.GetMiniMaxSettings().VoiceWhitelistEnabled = value
			return nil
		}); err != nil {
			t.Fatalf("set voice whitelist to %v: %v", value, err)
		}
	}
	applyVoiceWhitelist(enabled)
	t.Cleanup(func() { applyVoiceWhitelist(old) })
}

func insertVoiceRecord(t *testing.T, voiceId, redirectId, voiceType string, allowed bool) {
	t.Helper()

	now := time.Now().Unix()
	record := &voicestore.Voice{
		Type:         voiceType,
		OperatorId:   1,
		OperatorKind: "user",
		VoiceId:      voiceId,
		RedirectId:   redirectId,
		Allowed:      allowed,
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	if err := dbstore.DB.Create(record).Error; err != nil {
		t.Fatalf("insert voice %s: %v", voiceId, err)
	}
}

// TestResolveVoiceForTTSUpstream_PreviewAlwaysRejected 验证 preview（试听中）音色
// 无论白名单开关如何都必须拒绝：预览音色须完成确认定制（支付）流转为 created
// 后才能用于 TTS。回归保护：不可用检查若被包进白名单开关分支，开关关闭（默认值）
// 时 preview 会被直接放行给上游。
func TestResolveVoiceForTTSUpstream_PreviewAlwaysRejected(t *testing.T) {
	for _, whitelist := range []bool{false, true} {
		t.Run(fmt.Sprintf("whitelist=%v", whitelist), func(t *testing.T) {
			setupVoiceResolveTestDB(t)
			setVoiceWhitelist(t, whitelist)
			insertVoiceRecord(t, "preview-voice-1", "", voicestore.VoiceTypePreview, false)

			got, err := ResolveVoiceForTTSUpstream(nil, "preview-voice-1")
			if err == nil {
				t.Fatalf("preview voice must be rejected, got %q", got)
			}
		})
	}
}

// TestResolveVoiceForTTSUpstream_DisabledCreatedRejected 验证 created 但 allowed=false
// （管理员禁用）的音色同样不随白名单开关放行。
func TestResolveVoiceForTTSUpstream_DisabledCreatedRejected(t *testing.T) {
	for _, whitelist := range []bool{false, true} {
		t.Run(fmt.Sprintf("whitelist=%v", whitelist), func(t *testing.T) {
			setupVoiceResolveTestDB(t)
			setVoiceWhitelist(t, whitelist)
			insertVoiceRecord(t, "disabled-voice-1", "", voicestore.VoiceTypeCreated, false)

			got, err := ResolveVoiceForTTSUpstream(nil, "disabled-voice-1")
			if err == nil {
				t.Fatalf("disabled created voice must be rejected, got %q", got)
			}
		})
	}
}

// TestResolveVoiceForTTSUpstream_CreatedAllowedResolvesRedirect 验证已创建且允许的
// 音色在白名单开/关两种状态下都放行，且 redirect_id 优先于 VoiceId 发给上游。
func TestResolveVoiceForTTSUpstream_CreatedAllowedResolvesRedirect(t *testing.T) {
	for _, whitelist := range []bool{false, true} {
		t.Run(fmt.Sprintf("whitelist=%v", whitelist), func(t *testing.T) {
			setupVoiceResolveTestDB(t)
			setVoiceWhitelist(t, whitelist)
			insertVoiceRecord(t, "alias-voice-1", "real-upstream-1", voicestore.VoiceTypeCreated, true)

			got, err := ResolveVoiceForTTSUpstream(nil, "alias-voice-1")
			if err != nil {
				t.Fatalf("created allowed voice should pass: %v", err)
			}
			if got != "real-upstream-1" {
				t.Fatalf("resolved voice = %q, want redirect_id %q", got, "real-upstream-1")
			}

			// 无 redirect_id 时回落到 VoiceId 本身。
			insertVoiceRecord(t, "plain-voice-1", "", voicestore.VoiceTypeCreated, true)
			got, err = ResolveVoiceForTTSUpstream(nil, "plain-voice-1")
			if err != nil {
				t.Fatalf("created allowed voice without redirect should pass: %v", err)
			}
			if got != "plain-voice-1" {
				t.Fatalf("resolved voice = %q, want voice_id %q", got, "plain-voice-1")
			}
		})
	}
}

// TestResolveVoiceForTTSUpstream_UnmatchedDependsOnWhitelist 验证库内未命中的音色
// 仅受白名单开关约束：开启时拒绝，关闭时放行原 ID（上游预置音色场景）。
func TestResolveVoiceForTTSUpstream_UnmatchedDependsOnWhitelist(t *testing.T) {
	t.Run("whitelist=false passes through", func(t *testing.T) {
		setupVoiceResolveTestDB(t)
		setVoiceWhitelist(t, false)

		got, err := ResolveVoiceForTTSUpstream(nil, "builtin-voice-9")
		if err != nil {
			t.Fatalf("unmatched voice should pass when whitelist disabled: %v", err)
		}
		if got != "builtin-voice-9" {
			t.Fatalf("resolved voice = %q, want original %q", got, "builtin-voice-9")
		}
	})

	t.Run("whitelist=true rejected", func(t *testing.T) {
		setupVoiceResolveTestDB(t)
		setVoiceWhitelist(t, true)

		got, err := ResolveVoiceForTTSUpstream(nil, "builtin-voice-9")
		if err == nil {
			t.Fatalf("unmatched voice must be rejected when whitelist enabled, got %q", got)
		}
	})
}

// TestResolveVoiceForTTSUpstream_EmptyVoiceID 验证空音色 ID 直接返回空值不查库。
func TestResolveVoiceForTTSUpstream_EmptyVoiceID(t *testing.T) {
	got, err := ResolveVoiceForTTSUpstream(nil, "   ")
	if err != nil {
		t.Fatalf("empty voice id should not error: %v", err)
	}
	if got != "" {
		t.Fatalf("resolved voice = %q, want empty", got)
	}
}
