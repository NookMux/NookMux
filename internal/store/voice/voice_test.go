package voicestore

import (
	"fmt"
	"github.com/NookMux/NookMux/internal/store/db"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"testing"
	"time"
)

func setupVoiceTestDB(t *testing.T) {
	t.Helper()

	oldDB := dbstore.DB
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite test db: %v", err)
	}
	if err := db.AutoMigrate(&Voice{}); err != nil {
		t.Fatalf("migrate sqlite test db: %v", err)
	}
	dbstore.DB = db

	t.Cleanup(func() {
		if sqlDB, err := db.DB(); err == nil {
			_ = sqlDB.Close()
		}
		dbstore.DB = oldDB
	})
}

// createVoice 创建一条音色记录，createdAt 由调用方指定（秒级时间戳）。
func createVoice(t *testing.T, voiceId, voiceType string, createdAt int64) *Voice {
	t.Helper()
	voice := &Voice{
		Type:         voiceType,
		OperatorId:   1,
		OperatorKind: "user",
		VoiceId:      voiceId,
		CreatedAt:    createdAt,
		UpdatedAt:    createdAt,
	}
	if err := dbstore.DB.Create(voice).Error; err != nil {
		t.Fatalf("create voice %s: %v", voiceId, err)
	}
	return voice
}

func TestDeleteExpiredVoicePreviews_DeletesOnlyExpiredPreviews(t *testing.T) {
	setupVoiceTestDB(t)

	now := time.Now().Unix()
	// 超过 7 天的试听记录：应被删除。
	createVoice(t, "expired-preview-1", VoiceTypePreview, now-8*24*3600)
	// 未超过 7 天的试听记录：应保留。
	recent := createVoice(t, "recent-preview-1", VoiceTypePreview, now-1*24*3600)
	// 超过 7 天的“已创建”记录：不应被删除（仅清理试听中）。
	createdOld := createVoice(t, "created-old-1", VoiceTypeCreated, now-30*24*3600)

	cutoff := now - 7*24*3600
	affected, err := DeleteExpiredVoicePreviews(cutoff)
	if err != nil {
		t.Fatalf("DeleteExpiredVoicePreviews error: %v", err)
	}
	if affected != 1 {
		t.Fatalf("affected = %d, want 1", affected)
	}

	// 验证过期试听已被删除。
	var cnt int64
	if err := dbstore.DB.Model(&Voice{}).Where("voice_id = ?", "expired-preview-1").Count(&cnt).Error; err != nil {
		t.Fatalf("count expired preview: %v", err)
	}
	if cnt != 0 {
		t.Fatalf("expired preview still exists")
	}

	// 验证近期试听保留。
	var gotRecent Voice
	if err := dbstore.DB.Where("voice_id = ?", recent.VoiceId).First(&gotRecent).Error; err != nil {
		t.Fatalf("recent preview should remain: %v", err)
	}

	// 验证旧的 created 保留。
	var gotCreated Voice
	if err := dbstore.DB.Where("voice_id = ?", createdOld.VoiceId).First(&gotCreated).Error; err != nil {
		t.Fatalf("old created should remain: %v", err)
	}
}

func TestDeleteExpiredVoicePreviews_BoundaryKeepsExactlyCutoffAge(t *testing.T) {
	setupVoiceTestDB(t)

	// created_at == cutoff 的记录（恰好 7 天）不应被删除（条件为 created_at < cutoff）。
	now := time.Now().Unix()
	cutoff := now - 7*24*3600
	createVoice(t, "boundary-preview", VoiceTypePreview, cutoff)

	affected, err := DeleteExpiredVoicePreviews(cutoff)
	if err != nil {
		t.Fatalf("DeleteExpiredVoicePreviews error: %v", err)
	}
	if affected != 0 {
		t.Fatalf("affected = %d, want 0 (boundary record should be kept)", affected)
	}

	var cnt int64
	if err := dbstore.DB.Model(&Voice{}).Where("voice_id = ?", "boundary-preview").Count(&cnt).Error; err != nil {
		t.Fatalf("count boundary preview: %v", err)
	}
	if cnt != 1 {
		t.Fatalf("boundary preview should remain, got count %d", cnt)
	}
}

func TestDeleteExpiredVoicePreviews_NoopOnNilDB(t *testing.T) {
	// DB 未初始化时应安全跳过，不 panic。
	oldDB := dbstore.DB
	dbstore.DB = nil
	t.Cleanup(func() { dbstore.DB = oldDB })

	affected, err := DeleteExpiredVoicePreviews(time.Now().Unix())
	if err != nil {
		t.Fatalf("unexpected error on nil DB: %v", err)
	}
	if affected != 0 {
		t.Fatalf("affected = %d, want 0 on nil DB", affected)
	}
}

// TestConfirmVoice_AtomicTransitionAndQuotaCost 验证确认定制时：
//  1. preview -> created 流转成功；
//  2. quota_cost 被写入；
//  3. 再次确认返回 false（幂等，防止重复扣费）。
//
// 这是回归保护：旧实现先 UpdateVoiceType 再 UpdateVoice(voice)，
// 内存里的 voice.Type 仍是 preview，DB.Save 会把 type 覆盖回 preview。
func TestConfirmVoice_AtomicTransitionAndQuotaCost(t *testing.T) {
	setupVoiceTestDB(t)

	voice := createVoice(t, "confirm-target-1", VoiceTypePreview, time.Now().Unix())

	ok, err := ConfirmVoice(voice.Id, voice.OperatorId, 220)
	if err != nil {
		t.Fatalf("ConfirmVoice error: %v", err)
	}
	if !ok {
		t.Fatalf("ConfirmVoice should return true for a preview record")
	}

	var got Voice
	if err := dbstore.DB.Where("voice_id = ?", voice.VoiceId).First(&got).Error; err != nil {
		t.Fatalf("query voice after confirm: %v", err)
	}
	if got.Type != VoiceTypeCreated {
		t.Fatalf("type = %s, want %s (must not revert to preview)", got.Type, VoiceTypeCreated)
	}
	if got.QuotaCost != 220 {
		t.Fatalf("quota_cost = %d, want 220", got.QuotaCost)
	}

	// 再次确认同一记录应返回 false（已不是 preview），避免重复扣费。
	ok2, err := ConfirmVoice(voice.Id, voice.OperatorId, 220)
	if err != nil {
		t.Fatalf("second ConfirmVoice error: %v", err)
	}
	if ok2 {
		t.Fatalf("second ConfirmVoice should return false for an already-created record")
	}
}

// TestConfirmVoice_OperatorMismatch 验证操作人不匹配时不流转状态（防越权）。
func TestConfirmVoice_OperatorMismatch(t *testing.T) {
	setupVoiceTestDB(t)

	voice := createVoice(t, "confirm-target-2", VoiceTypePreview, time.Now().Unix())

	// 用不同的 operatorId 确认，应返回 false 且不改变状态。
	ok, err := ConfirmVoice(voice.Id, voice.OperatorId+999, 100)
	if err != nil {
		t.Fatalf("ConfirmVoice error: %v", err)
	}
	if ok {
		t.Fatalf("ConfirmVoice should return false when operator mismatch")
	}

	var got Voice
	if err := dbstore.DB.Where("voice_id = ?", voice.VoiceId).First(&got).Error; err != nil {
		t.Fatalf("query voice: %v", err)
	}
	if got.Type != VoiceTypePreview {
		t.Fatalf("type should remain preview on operator mismatch, got %s", got.Type)
	}
}
