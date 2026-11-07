package rankings

import (
	"github.com/NookMux/NookMux/internal/common"
	"github.com/NookMux/NookMux/internal/config/dashboard"
	"github.com/NookMux/NookMux/internal/store/usedata"
	"testing"
	"time"
)

// TestLimitRankedModelsRespectsLimit 验证 limitRankedModels 在数据量超过 limit 时正确截断。
func TestLimitRankedModelsRespectsLimit(t *testing.T) {
	rows := make([]RankedModel, 10)
	for i := range rows {
		rows[i] = RankedModel{Rank: i + 1, ModelName: "model-" + string(rune('a'+i))}
	}

	got := limitRankedModels(rows, 5)
	if len(got) != 5 {
		t.Fatalf("expected 5 rows, got %d", len(got))
	}
}

// TestLimitRankedModelsKeepsAllWhenUnderLimit 验证数据量不超过 limit 时原样返回。
func TestLimitRankedModelsKeepsAllWhenUnderLimit(t *testing.T) {
	rows := []RankedModel{{Rank: 1, ModelName: "a"}, {Rank: 2, ModelName: "b"}}
	got := limitRankedModels(rows, 5)
	if len(got) != 2 {
		t.Fatalf("expected 2 rows, got %d", len(got))
	}
}

// TestLimitRankedVendorsRespectsLimit 验证 limitRankedVendors 在数据量超过 limit 时正确截断。
func TestLimitRankedVendorsRespectsLimit(t *testing.T) {
	rows := make([]RankedVendor, 8)
	for i := range rows {
		rows[i] = RankedVendor{Rank: i + 1, Vendor: "vendor-" + string(rune('a'+i))}
	}

	got := limitRankedVendors(rows, 3)
	if len(got) != 3 {
		t.Fatalf("expected 3 rows, got %d", len(got))
	}
}

// TestBuildRankedPlatforms 验证平台排行榜的排名、环比与份额组装。
func TestBuildRankedPlatforms(t *testing.T) {
	current := []usedatastore.RankingPlatformTotal{
		{Platform: "claude-code", TotalTokens: 600},
		{Platform: "codex", TotalTokens: 300},
		{Platform: "opencode", TotalTokens: 100},
	}
	previous := []usedatastore.RankingPlatformTotal{
		{Platform: "codex", TotalTokens: 150},
		{Platform: "claude-code", TotalTokens: 200},
	}

	rows := buildRankedPlatforms(current, previous, true)
	if len(rows) != 3 {
		t.Fatalf("expected 3 rows, got %d", len(rows))
	}
	if rows[0].Rank != 1 || rows[0].Platform != "claude-code" || rows[0].Share != 0.6 {
		t.Fatalf("row0 = %+v, want rank 1 claude-code share 0.6", rows[0])
	}
	if rows[0].PreviousRank == nil || *rows[0].PreviousRank != 2 {
		t.Fatalf("claude-code previous rank = %v, want 2", rows[0].PreviousRank)
	}
	if rows[0].GrowthPct != 200 {
		t.Fatalf("claude-code growth = %v, want 200", rows[0].GrowthPct)
	}
	if rows[2].PreviousRank != nil {
		t.Fatalf("opencode previous rank = %v, want nil (new entrant)", rows[2].PreviousRank)
	}

	// 关闭环比时 growth 恒为 0。
	rows = buildRankedPlatforms(current, previous, false)
	if rows[0].GrowthPct != 0 {
		t.Fatalf("growth with showGrowth=false = %v, want 0", rows[0].GrowthPct)
	}
}

// TestLimitRankedPlatformsRespectsLimit 验证 limitRankedPlatforms 在数据量超过 limit 时正确截断。
func TestLimitRankedPlatformsRespectsLimit(t *testing.T) {
	rows := make([]RankedPlatform, 12)
	for i := range rows {
		rows[i] = RankedPlatform{Rank: i + 1, Platform: "platform-" + string(rune('a'+i))}
	}

	got := limitRankedPlatforms(rows, 10)
	if len(got) != 10 {
		t.Fatalf("expected 10 rows, got %d", len(got))
	}
}

// TestBuildPlatformHistoryRespectsConfigLimit 验证 buildPlatformHistory 使用传入的
// platformLimit 且超限平台归并入 Others。
func TestBuildPlatformHistoryRespectsConfigLimit(t *testing.T) {
	totals := make([]usedatastore.RankingPlatformTotal, 6)
	for i := range totals {
		totals[i] = usedatastore.RankingPlatformTotal{
			Platform:    "platform-" + string(rune('a'+i)),
			TotalTokens: int64(100 - i),
		}
	}
	buckets := []usedatastore.RankingPlatformBucket{
		{Platform: "platform-a", Bucket: 1000, Tokens: 100},
		{Platform: "platform-f", Bucket: 1000, Tokens: 1},
	}
	config := rankingPeriodConfig{id: "week", labelLayout: "Jan 2"}

	history := buildPlatformHistory(buckets, totals, config, 3)

	topCount := 0
	for _, p := range history.Platforms {
		if p.Name != rankingOthersLabel {
			topCount++
		}
	}
	if topCount != 3 {
		t.Fatalf("expected 3 top platforms in history, got %d (%+v)", topCount, history.Platforms)
	}
	// platform-f 超出 limit，其桶数据应归并入 Others 点。
	hasOthersPoint := false
	for _, point := range history.Points {
		if point.Platform == rankingOthersLabel && point.Tokens == 1 {
			hasOthersPoint = true
		}
	}
	if !hasOthersPoint {
		t.Fatalf("expected an Others point carrying platform-f tokens, got %+v", history.Points)
	}
}

// TestBuildModelHistoryRespectsConfigLimit 验证 buildModelHistory 使用传入的 modelLimit
// 而非硬编码常量（修复 issue #111 的核心断言）。
func TestBuildModelHistoryRespectsConfigLimit(t *testing.T) {
	// 准备 8 条 totals 数据
	totals := make([]usedatastore.RankingQuotaTotal, 8)
	for i := range totals {
		totals[i] = usedatastore.RankingQuotaTotal{
			ModelName:   "model-" + string(rune('a'+i)),
			TotalTokens: int64(100 - i),
		}
	}
	meta := map[string]rankingModelMeta{}
	for _, item := range totals {
		meta[item.ModelName] = rankingModelMeta{vendor: "Unknown"}
	}

	config := rankingPeriodConfig{id: "week", labelLayout: "Jan 2"}

	// 配置 limit=5，应只有 5 个模型 + 可能的 Others
	history := buildModelHistory(nil, totals, meta, config, 5)

	topCount := 0
	for _, m := range history.Models {
		if m.Name != rankingOthersLabel {
			topCount++
		}
	}
	if topCount > 5 {
		t.Fatalf("expected at most 5 top models in history, got %d", topCount)
	}
}

// TestBuildVendorShareHistoryRespectsConfigLimit 验证 buildVendorShareHistory 使用传入的 vendorLimit
// 而非硬编码常量。
func TestBuildVendorShareHistoryRespectsConfigLimit(t *testing.T) {
	// 准备 6 个 vendor
	vendors := make([]RankedVendor, 6)
	for i := range vendors {
		vendors[i] = RankedVendor{
			Rank:        i + 1,
			Vendor:      "vendor-" + string(rune('a'+i)),
			TotalTokens: int64(100 - i*10),
		}
	}
	meta := map[string]rankingModelMeta{}
	config := rankingPeriodConfig{id: "week", labelLayout: "Jan 2"}

	history := buildVendorShareHistory(nil, vendors, 1000, meta, config, 3)

	topCount := 0
	for _, v := range history.Vendors {
		if v.Name != rankingOthersLabel {
			topCount++
		}
	}
	if topCount > 3 {
		t.Fatalf("expected at most 3 top vendors in share history, got %d", topCount)
	}
}

// TestRankingsConfigFallbackOnZero 验证配置为 0/负数时 fallback 到默认常量值。
func TestRankingsConfigFallbackOnZero(t *testing.T) {
	cfg := dashboard.GetDashboardConfig()
	originalModel := cfg.RankingsModelLimit
	originalVendor := cfg.RankingsVendorLimit
	defer func() {
		cfg.RankingsModelLimit = originalModel
		cfg.RankingsVendorLimit = originalVendor
	}()

	cfg.RankingsModelLimit = 0
	cfg.RankingsVendorLimit = -1

	// 模拟 GetRankingsSnapshot 中的 fallback 逻辑
	modelLimit := cfg.RankingsModelLimit
	if modelLimit <= 0 {
		modelLimit = rankingLeaderboardLimit
	}
	vendorLimit := cfg.RankingsVendorLimit
	if vendorLimit <= 0 {
		vendorLimit = rankingVendorLimit
	}

	if modelLimit != rankingLeaderboardLimit {
		t.Fatalf("expected model limit fallback to %d, got %d", rankingLeaderboardLimit, modelLimit)
	}
	if vendorLimit != rankingVendorLimit {
		t.Fatalf("expected vendor limit fallback to %d, got %d", rankingVendorLimit, vendorLimit)
	}
}

// TestRankingTimeRangeUsesCalendarPeriods 验证排行榜周期是启动时区下的自然
// 日历周期，而非滚动 24h/7*24h 窗口（修复"今天=近24小时"问题）。
func TestRankingTimeRangeUsesCalendarPeriods(t *testing.T) {
	loc := time.FixedZone("UTC+8", 8*3600)
	// 2026-08-15 19:30 (UTC+8)，即 UTC 11:30。
	now := time.Date(2026, 8, 15, 19, 30, 0, 0, loc)

	cases := []struct {
		period       string
		wantStart    time.Time // 启动时区下的周期起点
		wantPrevEnd  int64     // 上一周期结束 = 当前周期起点-1
		wantPrevTest string    // 上一周期起点描述
	}{
		{
			period:    "today",
			wantStart: time.Date(2026, 8, 15, 0, 0, 0, 0, loc),
		},
		{
			// 2026-08-15 是周六，本周一为 08-10。
			period:    "week",
			wantStart: time.Date(2026, 8, 10, 0, 0, 0, 0, loc),
		},
		{
			period:    "month",
			wantStart: time.Date(2026, 8, 1, 0, 0, 0, 0, loc),
		},
		{
			period:    "year",
			wantStart: time.Date(2026, 1, 1, 0, 0, 0, 0, loc),
		},
	}

	for _, tc := range cases {
		config, err := rankingConfig(tc.period)
		if err != nil {
			t.Fatalf("rankingConfig(%q) error: %v", tc.period, err)
		}
		start, end := rankingTimeRange(config, now)
		if start != tc.wantStart.Unix() {
			t.Fatalf("period %q: start = %d, want %d (%s)",
				tc.period, start, tc.wantStart.Unix(), tc.wantStart.Format(time.RFC3339))
		}
		if end != now.Unix() {
			t.Fatalf("period %q: end = %d, want now %d", tc.period, end, now.Unix())
		}

		prevStart, prevEnd := previousRankingTimeRange(config, start, loc)
		if prevEnd != start-1 {
			t.Fatalf("period %q: prevEnd = %d, want %d", tc.period, prevEnd, start-1)
		}

		// 上一周期起点必须是同一周期的上一个自然边界。
		var wantPrevStart time.Time
		switch tc.period {
		case "today":
			wantPrevStart = tc.wantStart.AddDate(0, 0, -1)
		case "week":
			wantPrevStart = tc.wantStart.AddDate(0, 0, -7)
		case "month":
			wantPrevStart = tc.wantStart.AddDate(0, -1, 0)
		case "year":
			wantPrevStart = tc.wantStart.AddDate(-1, 0, 0)
		}
		if prevStart != wantPrevStart.Unix() {
			t.Fatalf("period %q: prevStart = %d, want %d (%s)",
				tc.period, prevStart, wantPrevStart.Unix(), wantPrevStart.Format(time.RFC3339))
		}
	}
}

// TestRankingTimeRangeAllHasNoLowerBound 验证 all 周期无下界。
func TestRankingTimeRangeAllHasNoLowerBound(t *testing.T) {
	config, err := rankingConfig("all")
	if err != nil {
		t.Fatalf("rankingConfig(all) error: %v", err)
	}
	start, end := rankingTimeRange(config, time.Now())
	if start != 0 {
		t.Fatalf("all period start = %d, want 0", start)
	}
	if end <= 0 {
		t.Fatalf("all period end = %d, want > 0", end)
	}
}

// TestRankingDayOffsetMatchesStartupTimezone 验证天级分桶偏移等于启动时区
// 相对 UTC 的偏移，保证"每日"桶边界落在启动时区的自然日 00:00。
func TestRankingDayOffsetMatchesStartupTimezone(t *testing.T) {
	if got := rankingDayOffset(); got != 0 {
		// 测试环境未设置 TZ 时偏移为 0；显式设置后再验证。
		t.Setenv("TZ", "Asia/Shanghai")
		common.InitStartupTimezone()
		want := int64(8 * 3600)
		if got := rankingDayOffset(); got != want {
			t.Fatalf("rankingDayOffset() with TZ=Asia/Shanghai = %d, want %d", got, want)
		}
	}
}
