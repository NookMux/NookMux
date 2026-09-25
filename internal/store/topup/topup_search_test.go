package topupstore

import (
	"fmt"
	"github.com/NookMux/NookMux/internal/common"
	"github.com/NookMux/NookMux/internal/store/db"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"testing"
)

func setupTopUpSearchTestDB(t *testing.T) {
	t.Helper()

	oldDB := dbstore.DB
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite test db: %v", err)
	}
	if err := db.AutoMigrate(&TopUp{}); err != nil {
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

func createTopUpSearchTestOrder(t *testing.T, userID int, tradeNo string, createdAt int64, status string) {
	t.Helper()

	topUp := &TopUp{
		UserId:     userID,
		Amount:     1,
		Money:      1,
		TradeNo:    tradeNo,
		CreateTime: createdAt,
		Status:     status,
	}
	if err := topUp.Insert(); err != nil {
		t.Fatalf("create topup: %v", err)
	}
}

func topUpSearchPageInfo() *common.PageInfo {
	return &common.PageInfo{Page: 1, PageSize: 20}
}

func TestGetUserTopUpsLimitsResultsToRecentWindow(t *testing.T) {
	setupTopUpSearchTestDB(t)

	now := common.GetTimestamp()
	createTopUpSearchTestOrder(t, 1, "recent-order", now, common.TopUpStatusSuccess)
	createTopUpSearchTestOrder(t, 1, "old-order", now-topUpQueryWindowSeconds-1, common.TopUpStatusSuccess)

	topups, total, err := GetUserTopUps(1, topUpSearchPageInfo())
	if err != nil {
		t.Fatalf("GetUserTopUps error = %v", err)
	}
	if total != 1 {
		t.Fatalf("total = %d, want 1", total)
	}
	if len(topups) != 1 || topups[0].TradeNo != "recent-order" {
		t.Fatalf("topups = %#v, want only recent-order", topups)
	}
}

// TestGetUserTopUpsKeepsPendingOrdersBeyondWindow 验证待支付订单不受时间窗口限制：
// 回调丢失的滞留订单需保持可见，供用户自查支付状态。
func TestGetUserTopUpsKeepsPendingOrdersBeyondWindow(t *testing.T) {
	setupTopUpSearchTestDB(t)

	now := common.GetTimestamp()
	createTopUpSearchTestOrder(t, 1, "old-pending", now-topUpQueryWindowSeconds-1, common.TopUpStatusPending)
	createTopUpSearchTestOrder(t, 1, "old-success", now-topUpQueryWindowSeconds-1, common.TopUpStatusSuccess)

	topups, total, err := GetUserTopUps(1, topUpSearchPageInfo())
	if err != nil {
		t.Fatalf("GetUserTopUps error = %v", err)
	}
	if total != 1 {
		t.Fatalf("total = %d, want 1", total)
	}
	if len(topups) != 1 || topups[0].TradeNo != "old-pending" {
		t.Fatalf("topups = %#v, want only old-pending", topups)
	}
}

func TestSearchUserTopUpsRejectsUnsafeLikePattern(t *testing.T) {
	setupTopUpSearchTestDB(t)

	_, _, err := SearchUserTopUps(1, "%%", topUpSearchPageInfo())
	if err == nil {
		t.Fatal("SearchUserTopUps accepted unsafe LIKE pattern")
	}
}

func TestSearchUserTopUpsUsesEscapedPatternAndRecentWindow(t *testing.T) {
	setupTopUpSearchTestDB(t)

	now := common.GetTimestamp()
	createTopUpSearchTestOrder(t, 1, "abc_123", now, common.TopUpStatusSuccess)
	createTopUpSearchTestOrder(t, 1, "abcX123", now, common.TopUpStatusSuccess)
	createTopUpSearchTestOrder(t, 1, "old_123", now-topUpQueryWindowSeconds-1, common.TopUpStatusSuccess)

	topups, total, err := SearchUserTopUps(1, "abc_%", topUpSearchPageInfo())
	if err != nil {
		t.Fatalf("SearchUserTopUps error = %v", err)
	}
	if total != 1 {
		t.Fatalf("total = %d, want 1", total)
	}
	if len(topups) != 1 || topups[0].TradeNo != "abc_123" {
		t.Fatalf("topups = %#v, want only abc_123", topups)
	}
}
