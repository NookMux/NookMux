package operation

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestValidateLogCleanupUnit(t *testing.T) {
	for _, unit := range []string{"year", "month", "week", "day"} {
		require.NoError(t, ValidateLogCleanupUnit(unit), unit)
	}
	for _, unit := range []string{"", "years", "hour", "minute", "YEAR"} {
		require.Error(t, ValidateLogCleanupUnit(unit), unit)
	}
}

func TestValidateLogCleanupValue(t *testing.T) {
	require.Error(t, ValidateLogCleanupValue(0))
	require.Error(t, ValidateLogCleanupValue(-1))
	require.NoError(t, ValidateLogCleanupValue(1))
	// X 为用户输入，不做上限限制
	require.NoError(t, ValidateLogCleanupValue(36500))
}

func TestValidateLogCleanupTargets(t *testing.T) {
	for _, raw := range []string{
		`[]`,
		`["logs"]`,
		`["logs","quota_data"]`,
		`["logs","quota_data","stored_images","stored_videos","audit_logs"]`,
	} {
		require.NoError(t, ValidateLogCleanupTargets(raw), raw)
	}
	for _, raw := range []string{
		``,
		`not-json`,
		`"logs"`,
		`["unknown"]`,
		`["logs","logs"]`,
	} {
		require.Error(t, ValidateLogCleanupTargets(raw), raw)
	}
}

func TestLogCleanupSettingDefaults(t *testing.T) {
	setting := GetLogCleanupSetting()
	require.False(t, setting.AutoCleanEnabled)
	require.Equal(t, 2, setting.RetentionValue)
	require.Equal(t, LogCleanupUnitYear, setting.RetentionUnit)
	require.Equal(t, 1, setting.IntervalValue)
	require.Equal(t, LogCleanupUnitWeek, setting.IntervalUnit)
	require.Equal(t, []string{LogCleanupTargetLogs, LogCleanupTargetQuotaData}, setting.Targets)
}
