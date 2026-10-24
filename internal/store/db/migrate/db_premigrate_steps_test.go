package dbmigrate

import (
	"context"
	"fmt"
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

// copyTableTestRow 显式主键实体，模拟跨库迁移时保留源库主键写入目标库的场景。
type copyTableTestRow struct {
	ID   int64  `gorm:"primaryKey"`
	Name string `gorm:"size:64"`
}

func (copyTableTestRow) TableName() string { return "copy_table_test_rows" }

// openCopyTableTestDB 打开两个互相独立的内存 SQLite 库并建好同名表，作为 src/dst。
func openCopyTableTestDB(t *testing.T) (src *gorm.DB, dst *gorm.DB) {
	t.Helper()
	open := func(name string) *gorm.DB {
		db, err := gorm.Open(sqlite.Open("file:"+name+"?mode=memory&cache=shared"), &gorm.Config{
			Logger: gormlogger.Default.LogMode(gormlogger.Silent),
		})
		if err != nil {
			t.Fatalf("open sqlite %s: %v", name, err)
		}
		if err := db.AutoMigrate(&copyTableTestRow{}); err != nil {
			t.Fatalf("migrate sqlite %s: %v", name, err)
		}
		return db
	}
	src = open(t.Name() + "-src")
	dst = open(t.Name() + "-dst")
	t.Cleanup(func() {
		for _, db := range []*gorm.DB{src, dst} {
			if sqlDB, err := db.DB(); err == nil {
				_ = sqlDB.Close()
			}
		}
	})
	return src, dst
}

func seedCopyTableRows(t *testing.T, db *gorm.DB, ids ...int64) {
	t.Helper()
	for _, id := range ids {
		row := copyTableTestRow{ID: id, Name: fmt.Sprintf("row-%d", id)}
		if err := db.Create(&row).Error; err != nil {
			t.Fatalf("seed row %d: %v", id, err)
		}
	}
}

func listCopyTableRows(t *testing.T, db *gorm.DB) []copyTableTestRow {
	t.Helper()
	var rows []copyTableTestRow
	if err := db.Order("id").Find(&rows).Error; err != nil {
		t.Fatalf("list rows: %v", err)
	}
	return rows
}

func TestCopyTableCopiesRowsAndContent(t *testing.T) {
	src, dst := openCopyTableTestDB(t)
	seedCopyTableRows(t, src, 1, 2, 3)

	copied, total, err := copyTable[copyTableTestRow](context.Background(), src, dst, copyTableOptions{}, nil)
	if err != nil {
		t.Fatalf("copyTable: %v", err)
	}
	if copied != 3 || total != 3 {
		t.Fatalf("expect copied=3 total=3, got copied=%d total=%d", copied, total)
	}
	rows := listCopyTableRows(t, dst)
	if len(rows) != 3 {
		t.Fatalf("expect 3 rows in dst, got %d", len(rows))
	}
	for i, row := range rows {
		wantID := int64(i + 1)
		if row.ID != wantID || row.Name != fmt.Sprintf("row-%d", wantID) {
			t.Fatalf("dst row %d mismatch: %+v", i, row)
		}
	}
}

func TestCopyTableSplitsBatchesAndReportsProgress(t *testing.T) {
	src, dst := openCopyTableTestDB(t)
	seedCopyTableRows(t, src, 1, 2, 3, 4, 5, 6, 7)

	type progress struct{ done, total int64 }
	var records []progress
	copied, total, err := copyTable[copyTableTestRow](context.Background(), src, dst, copyTableOptions{BatchSize: 2}, func(done int64, all int64) {
		records = append(records, progress{done, all})
	})
	if err != nil {
		t.Fatalf("copyTable: %v", err)
	}
	if copied != 7 || total != 7 {
		t.Fatalf("expect copied=7 total=7, got copied=%d total=%d", copied, total)
	}
	if len(records) < 4 {
		t.Fatalf("expect at least 4 progress records for 4 batches, got %d", len(records))
	}
	if records[0].done != 2 || records[0].total != 7 {
		t.Fatalf("first progress expect (2,7), got (%d,%d)", records[0].done, records[0].total)
	}
	for i := 1; i < len(records); i++ {
		if records[i].done < records[i-1].done {
			t.Fatalf("progress not monotonic: %+v then %+v", records[i-1], records[i])
		}
	}
	last := records[len(records)-1]
	if last.done != 7 || last.total != 7 {
		t.Fatalf("last progress expect (7,7), got (%d,%d)", last.done, last.total)
	}
	if got := len(listCopyTableRows(t, dst)); got != 7 {
		t.Fatalf("expect 7 rows in dst, got %d", got)
	}
}

func TestCopyTableClearDst(t *testing.T) {
	src, dst := openCopyTableTestDB(t)
	seedCopyTableRows(t, src, 1, 2)
	seedCopyTableRows(t, dst, 90, 91)

	copied, _, err := copyTable[copyTableTestRow](context.Background(), src, dst, copyTableOptions{ClearDst: true}, nil)
	if err != nil {
		t.Fatalf("copyTable: %v", err)
	}
	if copied != 2 {
		t.Fatalf("expect copied=2, got %d", copied)
	}
	rows := listCopyTableRows(t, dst)
	if len(rows) != 2 || rows[0].ID != 1 || rows[1].ID != 2 {
		t.Fatalf("dst should contain exactly src rows after clear, got %+v", rows)
	}
}

func TestCopyTableFailingBatchRollsBackAndKeepsCommittedBatches(t *testing.T) {
	src, dst := openCopyTableTestDB(t)
	seedCopyTableRows(t, src, 1, 2, 3, 4, 5)
	// 目标库预置 id=5：第 2 批 [4,5] 命中主键冲突后整批回滚（含本可写入的 4），
	// 已提交的第 1 批 [1,2,3] 保留。
	seedCopyTableRows(t, dst, 5)

	copied, _, err := copyTable[copyTableTestRow](context.Background(), src, dst, copyTableOptions{BatchSize: 3}, nil)
	if err == nil {
		t.Fatal("expect copyTable to fail on conflicting batch")
	}
	if copied != 3 {
		t.Fatalf("expect copied=3 (committed batches only), got %d", copied)
	}
	rows := listCopyTableRows(t, dst)
	gotIDs := make([]int64, 0, len(rows))
	for _, r := range rows {
		gotIDs = append(gotIDs, r.ID)
	}
	if fmt.Sprint(gotIDs) != "[1 2 3 5]" {
		t.Fatalf("dst ids expect [1 2 3 5], got %v", gotIDs)
	}
}

func TestCopyTableEmptySource(t *testing.T) {
	src, dst := openCopyTableTestDB(t)

	copied, total, err := copyTable[copyTableTestRow](context.Background(), src, dst, copyTableOptions{}, nil)
	if err != nil {
		t.Fatalf("copyTable: %v", err)
	}
	if copied != 0 || total != 0 {
		t.Fatalf("expect copied=0 total=0, got copied=%d total=%d", copied, total)
	}
	if got := len(listCopyTableRows(t, dst)); got != 0 {
		t.Fatalf("dst should stay empty, got %d rows", got)
	}
}
