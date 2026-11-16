package logcleanup

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/NookMux/NookMux/internal/config/operation"
	"github.com/NookMux/NookMux/internal/domain/rankings"
	auditstore "github.com/NookMux/NookMux/internal/store/audit"
	dbstore "github.com/NookMux/NookMux/internal/store/db"
	logstore "github.com/NookMux/NookMux/internal/store/log"
	storedmediastore "github.com/NookMux/NookMux/internal/store/stored_media"
	usedatastore "github.com/NookMux/NookMux/internal/store/usedata"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func mustTime(t *testing.T, s string) time.Time {
	parsed, err := time.ParseInLocation(time.DateTime, s, time.Local)
	require.NoError(t, err)
	return parsed
}

func TestNextRunTimeDay(t *testing.T) {
	// 周三 10:30，每 1 天 → 明天 00:00
	next, err := NextRunTime(mustTime(t, "2026-11-18 10:30:00"), 1, operation.LogCleanupUnitDay)
	require.NoError(t, err)
	require.Equal(t, mustTime(t, "2026-11-19 00:00:00"), next)

	// 恰好 00:00:00 不执行（严格晚于 base），每 2 天 → 后天 00:00
	next, err = NextRunTime(mustTime(t, "2026-11-18 00:00:00"), 2, operation.LogCleanupUnitDay)
	require.NoError(t, err)
	require.Equal(t, mustTime(t, "2026-11-20 00:00:00"), next)
}

func TestNextRunTimeWeek(t *testing.T) {
	// 2026-11-18 是周三，每 1 周 → 下周一 00:00
	next, err := NextRunTime(mustTime(t, "2026-11-18 10:30:00"), 1, operation.LogCleanupUnitWeek)
	require.NoError(t, err)
	require.Equal(t, mustTime(t, "2026-11-23 00:00:00"), next)

	// 周一 00:00 整点 → 下周一
	next, err = NextRunTime(mustTime(t, "2026-11-23 00:00:00"), 1, operation.LogCleanupUnitWeek)
	require.NoError(t, err)
	require.Equal(t, mustTime(t, "2026-11-30 00:00:00"), next)

	// 周日（weekday=0 归一为第 7 天），每 1 周 → 明天周一 00:00
	next, err = NextRunTime(mustTime(t, "2026-11-22 20:00:00"), 1, operation.LogCleanupUnitWeek)
	require.NoError(t, err)
	require.Equal(t, mustTime(t, "2026-11-23 00:00:00"), next)

	// 每 3 周
	next, err = NextRunTime(mustTime(t, "2026-11-18 10:30:00"), 3, operation.LogCleanupUnitWeek)
	require.NoError(t, err)
	require.Equal(t, mustTime(t, "2026-12-07 00:00:00"), next)
}

func TestNextRunTimeMonth(t *testing.T) {
	next, err := NextRunTime(mustTime(t, "2026-11-18 10:30:00"), 1, operation.LogCleanupUnitMonth)
	require.NoError(t, err)
	require.Equal(t, mustTime(t, "2026-12-01 00:00:00"), next)

	// 每 2 月跨年
	next, err = NextRunTime(mustTime(t, "2026-11-18 10:30:00"), 2, operation.LogCleanupUnitMonth)
	require.NoError(t, err)
	require.Equal(t, mustTime(t, "2027-01-01 00:00:00"), next)
}

func TestNextRunTimeYear(t *testing.T) {
	next, err := NextRunTime(mustTime(t, "2026-11-18 10:30:00"), 1, operation.LogCleanupUnitYear)
	require.NoError(t, err)
	require.Equal(t, mustTime(t, "2027-01-01 00:00:00"), next)

	// 每 2 年
	next, err = NextRunTime(mustTime(t, "2026-11-18 10:30:00"), 2, operation.LogCleanupUnitYear)
	require.NoError(t, err)
	require.Equal(t, mustTime(t, "2028-01-01 00:00:00"), next)
}

func TestNextRunTimeInvalid(t *testing.T) {
	for _, value := range []int{0, -1} {
		_, err := NextRunTime(time.Now(), value, operation.LogCleanupUnitDay)
		require.Error(t, err)
	}
	_, err := NextRunTime(time.Now(), 1, "hour")
	require.Error(t, err)
}

func TestRetentionCutoff(t *testing.T) {
	now := mustTime(t, "2026-11-18 10:30:00")

	// 保留近两年：按日历回退（覆盖闰年 2024-02-29 之后的 2 月对齐）
	cutoff, err := RetentionCutoff(now, 2, operation.LogCleanupUnitYear)
	require.NoError(t, err)
	require.Equal(t, mustTime(t, "2024-11-18 10:30:00"), cutoff)

	cutoff, err = RetentionCutoff(now, 3, operation.LogCleanupUnitMonth)
	require.NoError(t, err)
	require.Equal(t, mustTime(t, "2026-08-18 10:30:00"), cutoff)

	cutoff, err = RetentionCutoff(now, 1, operation.LogCleanupUnitWeek)
	require.NoError(t, err)
	require.Equal(t, mustTime(t, "2026-11-11 10:30:00"), cutoff)

	cutoff, err = RetentionCutoff(now, 30, operation.LogCleanupUnitDay)
	require.NoError(t, err)
	require.Equal(t, mustTime(t, "2026-10-19 10:30:00"), cutoff)

	for _, value := range []int{0, -2} {
		_, err := RetentionCutoff(now, value, operation.LogCleanupUnitYear)
		require.Error(t, err)
	}
	_, err = RetentionCutoff(now, 1, "hour")
	require.Error(t, err)
}

func TestRunCleanupRejectsInvalidRetention(t *testing.T) {
	// 保留时长配置非法时必须显式失败，不得静默跳过或清空全部数据。
	original := snapshotSetting()
	defer func() {
		setting := operation.GetLogCleanupSetting()
		setting.AutoCleanEnabled = original.AutoCleanEnabled
		setting.RetentionValue = original.RetentionValue
		setting.RetentionUnit = original.RetentionUnit
		setting.IntervalValue = original.IntervalValue
		setting.IntervalUnit = original.IntervalUnit
		setting.Targets = original.Targets
	}()

	setting := operation.GetLogCleanupSetting()
	setting.AutoCleanEnabled = true
	setting.RetentionValue = 0
	setting.Targets = []string{operation.LogCleanupTargetLogs}

	_, err := RunCleanup(t.Context())
	require.Error(t, err)
}

// TestRunCleanupDeletesExpiredTargets 在内存 SQLite 上验证一轮自动清理：
// 各目标表仅删除早于保留截止时刻的行，保留时长内的行不受影响。
func TestRunCleanupDeletesExpiredTargets(t *testing.T) {
	oldDB, oldLogDB := dbstore.DB, dbstore.LOG_DB
	setting := operation.GetLogCleanupSetting()
	original := operation.LogCleanupSetting{
		AutoCleanEnabled: setting.AutoCleanEnabled,
		RetentionValue:   setting.RetentionValue,
		RetentionUnit:    setting.RetentionUnit,
		IntervalValue:    setting.IntervalValue,
		IntervalUnit:     setting.IntervalUnit,
		Targets:          setting.Targets,
	}
	t.Cleanup(func() {
		dbstore.DB, dbstore.LOG_DB = oldDB, oldLogDB
		setting.AutoCleanEnabled = original.AutoCleanEnabled
		setting.RetentionValue = original.RetentionValue
		setting.RetentionUnit = original.RetentionUnit
		setting.IntervalValue = original.IntervalValue
		setting.IntervalUnit = original.IntervalUnit
		setting.Targets = original.Targets
		rankings.ClearRankingsCache()
	})

	gdb, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, gdb.AutoMigrate(
		&logstore.Log{},
		&usedatastore.QuotaData{},
		&storedmediastore.StoredMedia{},
		&auditstore.AuditLog{},
	))
	dbstore.DB = gdb
	dbstore.LOG_DB = gdb

	setting.AutoCleanEnabled = true
	setting.RetentionValue = 1
	setting.RetentionUnit = operation.LogCleanupUnitDay
	setting.Targets = []string{
		operation.LogCleanupTargetLogs,
		operation.LogCleanupTargetQuotaData,
		operation.LogCleanupTargetStoredImages,
		operation.LogCleanupTargetStoredVideos,
		operation.LogCleanupTargetAuditLogs,
	}

	now := time.Now()
	expired := now.AddDate(0, 0, -2).Unix()
	retained := now.Unix()

	require.NoError(t, gdb.Create(&logstore.Log{CreatedAt: expired, Type: logstore.LogTypeConsume}).Error)
	require.NoError(t, gdb.Create(&logstore.Log{CreatedAt: retained, Type: logstore.LogTypeConsume}).Error)
	require.NoError(t, gdb.Create(&usedatastore.QuotaData{UserID: 1, Username: "u", ModelName: "m", CreatedAt: expired, Count: 1}).Error)
	require.NoError(t, gdb.Create(&usedatastore.QuotaData{UserID: 1, Username: "u", ModelName: "m", CreatedAt: retained, Count: 1}).Error)
	require.NoError(t, gdb.Create(&storedmediastore.StoredMedia{Id: "img-old", MediaType: storedmediastore.MediaTypeImage, CreatedAt: expired, Sha256: "old-image", Data: storedmediastore.LargeBlob("x")}).Error)
	require.NoError(t, gdb.Create(&storedmediastore.StoredMedia{Id: "img-new", MediaType: storedmediastore.MediaTypeImage, CreatedAt: retained, Sha256: "new-image", Data: storedmediastore.LargeBlob("x")}).Error)
	require.NoError(t, gdb.Create(&storedmediastore.StoredMedia{Id: "vid-old", MediaType: storedmediastore.MediaTypeVideo, CreatedAt: expired, Sha256: "old-video", Data: storedmediastore.LargeBlob("x")}).Error)
	require.NoError(t, gdb.Create(&storedmediastore.StoredMedia{Id: "vid-new", MediaType: storedmediastore.MediaTypeVideo, CreatedAt: retained, Sha256: "new-video", Data: storedmediastore.LargeBlob("x")}).Error)
	require.NoError(t, gdb.Create(&auditstore.AuditLog{CreatedAt: expired, Username: "root", Module: auditstore.AuditModuleOption}).Error)
	require.NoError(t, gdb.Create(&auditstore.AuditLog{CreatedAt: retained, Username: "root", Module: auditstore.AuditModuleOption}).Error)

	result, err := RunCleanup(context.Background())
	require.NoError(t, err)
	require.Equal(t, int64(1), result.Logs)
	require.Equal(t, int64(1), result.QuotaData)
	require.Equal(t, int64(1), result.StoredImages)
	require.Equal(t, int64(1), result.StoredVideos)
	require.Equal(t, int64(1), result.AuditLogs)

	for table, count := range map[string]int64{
		"logs":         1,
		"quota_data":   1,
		"stored_media": 2,
		"audit_logs":   1,
	} {
		var total int64
		require.NoError(t, gdb.Table(table).Count(&total).Error, table)
		require.Equal(t, count, total, table)
	}
}
