package dbmigrate

import (
	"fmt"
	"testing"

	"github.com/NookMux/NookMux/internal/store/db"
	"github.com/NookMux/NookMux/internal/store/log"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func TestBackfillLogBillingTokenDetailsResumesCommittedBatches(t *testing.T) {
	dbHandle, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := dbHandle.AutoMigrate(&logstore.Log{}); err != nil {
		t.Fatalf("migrate sqlite: %v", err)
	}
	addLegacyAggregateColumns(t, dbHandle)
	oldLogDB := dbstore.LOG_DB
	dbstore.LOG_DB = dbHandle
	t.Cleanup(func() { dbstore.LOG_DB = oldLogDB })

	oldBatchSize := logBillingTokenDetailsBatchSize
	logBillingTokenDetailsBatchSize = 2
	t.Cleanup(func() { logBillingTokenDetailsBatchSize = oldBatchSize })

	rows := []logstore.Log{
		{Type: logstore.LogTypeConsume, PromptTokens: 100, CompletionTokens: 50, Other: `{"cache_tokens":20}`},
		{Type: logstore.LogTypeConsume, PromptTokens: 50, CompletionTokens: 25, Other: `{"cache_tokens":10}`},
		{Type: logstore.LogTypeConsume, PromptTokens: 10, CompletionTokens: 5, Other: `{"cache_tokens":999}`},
	}
	for i := range rows {
		if err := dbHandle.Create(&rows[i]).Error; err != nil {
			t.Fatalf("seed row %d: %v", i, err)
		}
		seedLegacyAggregates(t, dbHandle, rows[i].Id, rows[i].PromptTokens, rows[i].CompletionTokens)
	}

	err = backfillLogBillingTokenDetails()
	if err == nil {
		t.Fatal("backfill succeeded, want failure after the first committed batch")
	}
	var firstBatchDone int64
	if err := dbHandle.Model(&logstore.Log{}).
		Where("id IN ? AND billing_details_version = ?", []int{rows[0].Id, rows[1].Id}, logstore.LogBillingDetailsVersion).
		Count(&firstBatchDone).Error; err != nil {
		t.Fatalf("count committed first batch: %v", err)
	}
	if firstBatchDone != 2 {
		t.Fatalf("committed first batch count = %d, want 2", firstBatchDone)
	}
	var pendingRow logstore.Log
	if err := dbHandle.First(&pendingRow, rows[2].Id).Error; err != nil {
		t.Fatalf("reload pending row: %v", err)
	}
	if pendingRow.BillingDetailsVersion != 0 {
		t.Fatalf("pending version = %d, want 0", pendingRow.BillingDetailsVersion)
	}

	if err := dbHandle.Model(&logstore.Log{}).Where("id = ?", rows[2].Id).
		Update("other", `{"cache_tokens":5}`).Error; err != nil {
		t.Fatalf("repair pending row: %v", err)
	}
	if err := backfillLogBillingTokenDetails(); err != nil {
		t.Fatalf("resume backfill: %v", err)
	}
	var migrated int64
	if err := dbHandle.Model(&logstore.Log{}).
		Where("billing_details_version = ?", logstore.LogBillingDetailsVersion).
		Count(&migrated).Error; err != nil {
		t.Fatalf("count migrated rows: %v", err)
	}
	if migrated != 3 {
		t.Fatalf("migrated rows = %d, want 3", migrated)
	}
}
