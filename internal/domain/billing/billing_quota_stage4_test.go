package billing

import (
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/NookMux/NookMux/internal/common"
	"github.com/NookMux/NookMux/internal/config/ratio"
	"github.com/NookMux/NookMux/internal/domain/billing/contract"
	"github.com/NookMux/NookMux/internal/domain/shared"
	relayconstant "github.com/NookMux/NookMux/internal/relay/constant"
	dbstore "github.com/NookMux/NookMux/internal/store/db"
	logstore "github.com/NookMux/NookMux/internal/store/log"
	pricingstore "github.com/NookMux/NookMux/internal/store/pricing"
	"github.com/glebarez/sqlite"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// setupEmptyExplicitPricePlans gives CalculateUsage tests a real, empty
// component table without replacing the lossless legacy fallback.
func setupEmptyExplicitPricePlans(t *testing.T) {
	t.Helper()
	oldDB := dbstore.DB
	oldLogDB := dbstore.LOG_DB
	testDB, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())), &gorm.Config{})
	if err != nil {
		t.Fatalf("open empty price plan sqlite: %v", err)
	}
	if err := testDB.AutoMigrate(&pricingstore.ModelPricePlan{}, &pricingstore.ModelPriceComponent{}); err != nil {
		t.Fatalf("migrate empty price plan sqlite: %v", err)
	}
	dbstore.DB = testDB
	dbstore.LOG_DB = testDB
	pricingstore.InvalidateModelPricePlanCache()
	t.Cleanup(func() {
		if sqlDB, err := testDB.DB(); err == nil {
			_ = sqlDB.Close()
		}
		dbstore.DB = oldDB
		dbstore.LOG_DB = oldLogDB
		pricingstore.InvalidateModelPricePlanCache()
	})
}

func stage4TokenPlan(id int64, components ...contract.ModelPriceComponent) contract.ModelPricePlan {
	return contract.ModelPricePlan{
		ID:                    id,
		ModelName:             "model-a",
		BillingMode:           contract.BillingModeToken,
		Currency:              "USD",
		ExchangeRate:          "1",
		PricePrecision:        12,
		RoundingMode:          contract.PriceRoundingHalfUp,
		GroupMultiplierSource: contract.GroupMultiplierSourceInherit,
		Components:            components,
		Source:                contract.PricePlanSourceExplicit,
	}
}

func stage4Component(component contract.PriceComponent, price string) contract.ModelPriceComponent {
	return contract.ModelPriceComponent{Component: component, Unit: contract.PriceUnitPerMillionTokens, UnitPrice: price}
}

func stage4Usage() *BillingUsage {
	write5m := 50
	return &BillingUsage{
		PromptAggregateTokens: 850,
		OutputTokens:          500,
		CacheReadTokens:       100,
		CacheWriteTokens:      50,
		CacheWrite5mTokens:    &write5m,
	}
}

func TestStage4ExplicitTokenPlanMatchesEquivalentLegacyQuota(t *testing.T) {
	bu := stage4Usage()
	priceData := normalizedQuotaTestPriceData()
	legacy := mustQuota(t, bu, priceData, AudioPricingAbsolute, "model-a")

	// The legacy ratio projection converts quota points/token into USD/1M:
	// ratio * 1,000,000 / QuotaPerUnit.
	quotaPerUnit := decimal.NewFromInt(500_000)
	legacyPrice := func(ratio string, multiplier string) string {
		r, err := decimal.NewFromString(ratio)
		if err != nil {
			t.Fatal(err)
		}
		m, err := decimal.NewFromString(multiplier)
		if err != nil {
			t.Fatal(err)
		}
		return r.Mul(m).Mul(decimal.NewFromInt(1_000_000)).Div(quotaPerUnit).String()
	}
	explicit := stage4TokenPlan(11,
		stage4Component(contract.PriceComponentInput, legacyPrice("2", "1")),
		stage4Component(contract.PriceComponentOutput, legacyPrice("2", "3")),
		stage4Component(contract.PriceComponentCacheRead, legacyPrice("2", "0.5")),
		stage4Component(contract.PriceComponentCacheWrite5m, legacyPrice("2", "1.25")),
		stage4Component(contract.PriceComponentCacheWrite1h, legacyPrice("2", "2")),
	)
	plans := append([]contract.ModelPricePlan{explicit}, legacyPricePlansForRelay("model-a", priceData)...)
	got, err := calculateNormalizedQuotaWithPlans(bu, priceData, AudioPricingAbsolute, plans, contract.ModelPricePlanQuery{ModelName: "model-a"})
	if err != nil {
		t.Fatalf("explicit token quota: %v", err)
	}
	if got.TokenTotal.Cmp(legacy.TokenTotal) != 0 {
		t.Fatalf("explicit quota %s != legacy quota %s", got.TokenTotal, legacy.TokenTotal)
	}
	if got.PriceSnapshot == nil || got.PriceSnapshot.Source != contract.PricePlanSourceExplicit {
		t.Fatalf("explicit snapshot = %+v", got.PriceSnapshot)
	}
	if len(got.PriceSnapshot.Components) != 4 {
		t.Fatalf("snapshot components = %+v", got.PriceSnapshot.Components)
	}
}

func TestStage4ChildComponentsDoNotDoubleChargeParents(t *testing.T) {
	bu := &BillingUsage{
		PromptAggregateTokens: 1000,
		OutputTokens:          500,
		AudioInputTokens:      intPtr(100),
		AudioOutputTokens:     intPtr(50),
	}
	priceData := normalizedQuotaTestPriceData()
	plans := []contract.ModelPricePlan{stage4TokenPlan(12,
		stage4Component(contract.PriceComponentTextInput, "8"),
		stage4Component(contract.PriceComponentTextOutput, "24"),
		stage4Component(contract.PriceComponentAudioInput, "32"),
		stage4Component(contract.PriceComponentAudioOutput, "64"),
	)}
	allPlans := append(plans, legacyPricePlansForRelay("model-a", priceData)...)
	result, err := calculateNormalizedQuotaWithPlans(bu, priceData, AudioPricingAbsolute, allPlans, contract.ModelPricePlanQuery{ModelName: "model-a"})
	if err != nil {
		t.Fatalf("split component quota: %v", err)
	}
	// TokenTotal holds 900 text input + 450 text output + 50 audio output.
	// The absolute audio input fee remains separate: 100 * 16 = 1,600.
	assertQuotaTotal(t, result, 900*4+450*12+50*32)
	if result.AudioInputQuota.Cmp(decimal.NewFromInt(1600)) != 0 {
		t.Fatalf("audio input quota = %s, want 1600", result.AudioInputQuota)
	}
	if result.AudioInputQuota.IsZero() {
		t.Fatal("absolute audio input must remain a separate fee")
	}
	if input, output := bu.InputTokens(), bu.OutputTokens; input != 1000 || output != 500 {
		t.Fatalf("billing usage mutated: input=%d output=%d", input, output)
	}
}

func TestStage4PerRequestFreeServiceTierAndContext(t *testing.T) {
	priceData := normalizedQuotaTestPriceData()
	bu := stage4Usage()

	perRequest := stage4TokenPlan(20, stage4Component(contract.PriceComponentInput, "1"))
	perRequest.BillingMode = contract.BillingModePerRequest
	perRequest.Components = []contract.ModelPriceComponent{{
		Component: contract.PriceComponentRequest,
		Unit:      contract.PriceUnitPerRequest,
		UnitPrice: "0.03",
	}}
	result, err := calculateNormalizedQuotaWithPlans(bu, priceData, AudioPricingAbsolute, []contract.ModelPricePlan{perRequest}, contract.ModelPricePlanQuery{ModelName: "model-a"})
	if err != nil {
		t.Fatalf("per-request quota: %v", err)
	}
	assertQuotaTotal(t, result, 15000)

	free := stage4TokenPlan(21)
	free.BillingMode = contract.BillingModeFree
	free.Components = nil
	result, err = calculateNormalizedQuotaWithPlans(bu, priceData, AudioPricingAbsolute, []contract.ModelPricePlan{free}, contract.ModelPricePlanQuery{ModelName: "model-a"})
	if err != nil {
		t.Fatalf("free quota: %v", err)
	}
	assertQuotaTotal(t, result, 0)

	priority := stage4TokenPlan(22,
		stage4Component(contract.PriceComponentInput, "4"),
		stage4Component(contract.PriceComponentOutput, "12"),
		stage4Component(contract.PriceComponentCacheRead, "2"),
		stage4Component(contract.PriceComponentCacheWrite5m, "5"),
		stage4Component(contract.PriceComponentCacheWrite1h, "4"),
	)
	priority.ServiceTier = "priority"
	priority.ContextMinTokens = 1000
	legacy := mustQuota(t, bu, priceData, AudioPricingAbsolute, "model-a")
	defaultPlan := stage4TokenPlan(23,
		stage4Component(contract.PriceComponentInput, "2"),
		stage4Component(contract.PriceComponentOutput, "6"),
		stage4Component(contract.PriceComponentCacheRead, "0.5"),
		stage4Component(contract.PriceComponentCacheWrite5m, "1.25"),
		stage4Component(contract.PriceComponentCacheWrite1h, "2"),
	)
	query := contract.ModelPricePlanQuery{ModelName: "model-a", ServiceTier: "priority", ContextTokens: 1400}
	result, err = calculateNormalizedQuotaWithPlans(bu, priceData, AudioPricingAbsolute, []contract.ModelPricePlan{priority, defaultPlan}, query)
	if err != nil {
		t.Fatalf("service-tier quota: %v", err)
	}
	if result.PriceSnapshot.ServiceTier != "priority" || result.PriceSnapshot.ContextMinTokens != 1000 {
		t.Fatalf("tier snapshot = %+v", result.PriceSnapshot)
	}
	// Priority scope is selected even when it is priced equivalently to the
	// legacy path; snapshot dimensions prove which plan won.
	if result.TokenTotal.Cmp(legacy.TokenTotal) != 0 {
		t.Logf("result lines: %+v; legacy lines: %+v", result.Lines, legacy.Lines)
		t.Fatalf("service-tier quota %s, want %s", result.TokenTotal, legacy.TokenTotal)
	}
}

func TestStage4FixedMultiplierExchangeAndFinalRounding(t *testing.T) {
	bu := &BillingUsage{PromptAggregateTokens: 1_000_000}
	priceData := normalizedQuotaTestPriceData()
	plan := stage4TokenPlan(30, stage4Component(contract.PriceComponentInput, "0.10101"))
	plan.Currency = "USD"
	plan.ExchangeRate = "1"
	plan.GroupMultiplierSource = contract.GroupMultiplierSourceFixed
	plan.GroupMultiplier = "1.1"
	plan.RoundingMode = contract.PriceRoundingFloor
	plans := append([]contract.ModelPricePlan{plan}, legacyPricePlansForRelay("model-a", priceData)...)
	result, err := calculateNormalizedQuotaWithPlans(bu, priceData, AudioPricingAbsolute, plans, contract.ModelPricePlanQuery{ModelName: "model-a"})
	if err != nil {
		t.Fatalf("fixed multiplier quota: %v", err)
	}
	// 1,000,000 * 0.10101 / 1,000,000 * 500,000 * 1.1 = 55,555.5. The
	// unrounded settlement stays exact; final rounding happens separately.
	if result.TokenTotal.Cmp(decimal.NewFromFloat(55555.5)) != 0 {
		t.Fatalf("unrounded total = %s, want 55555.5", result.TokenTotal)
	}
	if got := result.PriceSnapshot.GroupMultiplier; got != "1.1" {
		t.Fatalf("snapshot multiplier = %s, want 1.1", got)
	}
	if got := roundEntryQuota(result.TokenTotal, result, false); got != 55555 {
		t.Fatalf("explicit floor rounding = %d", got)
	}
}

func TestStage4FinalRoundingModes(t *testing.T) {
	half := decimal.NewFromInt(55555).Add(decimal.NewFromFloat(.5))
	oneAndHalf := decimal.NewFromInt(55556).Add(decimal.NewFromFloat(.5))
	tests := []struct {
		mode contract.PriceRoundingMode
		want int
	}{
		{contract.PriceRoundingHalfUp, 55556},
		{contract.PriceRoundingHalfEven, 55556},
		{contract.PriceRoundingFloor, 55555},
		{contract.PriceRoundingCeil, 55556},
	}
	for _, test := range tests {
		if got := RoundBillingQuota(half, test.mode); got != test.want {
			t.Fatalf("half value %s mode %s = %d, want %d", half, test.mode, got, test.want)
		}
	}
	if got := RoundBillingQuota(oneAndHalf, contract.PriceRoundingHalfEven); got != 55556 {
		t.Fatalf("banker's tie = %d, want 55556", got)
	}
}

func TestStage4CrossPlanFallbackRecordsComponentMultiplier(t *testing.T) {
	bu := stage4Usage()
	priceData := normalizedQuotaTestPriceData()
	priceData.GroupRatioInfo.GroupRatio = 3
	outer := stage4TokenPlan(40, stage4Component(contract.PriceComponentTextInput, "2"))
	outer.GroupMultiplierSource = contract.GroupMultiplierSourceFixed
	outer.GroupMultiplier = "2"
	plans := append([]contract.ModelPricePlan{outer}, legacyPricePlansForRelay("model-a", priceData)...)
	result, err := calculateNormalizedQuotaWithPlans(bu, priceData, AudioPricingAbsolute, plans, contract.ModelPricePlanQuery{ModelName: "model-a"})
	if err != nil {
		t.Fatalf("cross-plan quota: %v", err)
	}
	if result.PriceSnapshot.GroupMultiplier != "2" {
		t.Fatalf("effective multiplier = %s, want 2", result.PriceSnapshot.GroupMultiplier)
	}
	components := make(map[contract.PriceComponent]BillingPriceComponentSnapshot)
	for _, component := range result.PriceSnapshot.Components {
		components[component.Component] = component
	}
	if got := components[contract.PriceComponentTextInput].GroupMultiplier; got != "2" {
		t.Fatalf("input component multiplier = %s, want effective plan multiplier 2", got)
	}
	if got := components[contract.PriceComponentTextOutput].GroupMultiplier; got != "3" {
		t.Fatalf("fallback output multiplier = %s, want inherited multiplier 3", got)
	}
}

func TestStage4LegacyAbsoluteAudioSnapshotExplainsRemainders(t *testing.T) {
	bu := &BillingUsage{
		PromptAggregateTokens: 1000,
		OutputTokens:          500,
		AudioInputTokens:      intPtr(200),
	}
	priceData := normalizedQuotaTestPriceData()
	legacyPlan := legacyPricePlansForRelay("gemini-2.5-flash", priceData)[0]
	result, err := calculateNormalizedQuotaWithPlans(
		bu, priceData, AudioPricingAbsolute, []contract.ModelPricePlan{legacyPlan},
		contract.ModelPricePlanQuery{ModelName: "gemini-2.5-flash"},
	)
	if err != nil {
		t.Fatalf("legacy absolute audio quota: %v", err)
	}
	components := make(map[contract.PriceComponent]BillingPriceComponentSnapshot)
	for _, component := range result.PriceSnapshot.Components {
		components[component.Component] = component
	}
	if got := components[contract.PriceComponentTextInput]; got.UnitPrice != "4" {
		t.Fatalf("ordinary input snapshot = %+v", got)
	}
	if got := components[contract.PriceComponentTextOutput]; got.UnitPrice != "12" {
		t.Fatalf("ordinary output snapshot = %+v", got)
	}
	if got := components[contract.PriceComponentAudioInput]; got.UnitPrice != "1" || got.GroupMultiplier != "1" {
		t.Fatalf("audio input snapshot = %+v", got)
	}
	for _, duplicate := range []contract.PriceComponent{
		contract.PriceComponentInput,
		contract.PriceComponentAudioOutput,
	} {
		if _, ok := components[duplicate]; ok {
			t.Fatalf("unexpected %s snapshot component", duplicate)
		}
	}
}

func TestStage4SnapshotRemainsUsableAfterPriceMutation(t *testing.T) {
	bu := &BillingUsage{PromptAggregateTokens: 1_000_000, OutputTokens: 1}
	priceData := normalizedQuotaTestPriceData()
	plan := stage4TokenPlan(31,
		stage4Component(contract.PriceComponentInput, "4"),
		stage4Component(contract.PriceComponentOutput, "12"),
		stage4Component(contract.PriceComponentCacheRead, "1"),
		stage4Component(contract.PriceComponentCacheWrite5m, "2.5"),
		stage4Component(contract.PriceComponentCacheWrite1h, "4"),
	)
	result, err := calculateNormalizedQuotaWithPlans(bu, priceData, AudioPricingAbsolute, []contract.ModelPricePlan{plan}, contract.ModelPricePlanQuery{ModelName: "model-a"})
	if err != nil {
		t.Fatalf("snapshot quota: %v", err)
	}
	before := result.TokenTotal.String()
	for i := range plan.Components {
		plan.Components[i].UnitPrice = "999"
	}
	if result.TokenTotal.String() != before {
		t.Fatalf("settled decimal changed after plan mutation: %s != %s", result.TokenTotal, before)
	}
	if result.PriceSnapshot.Components[0].UnitPrice != "4" {
		t.Fatalf("snapshot price changed: %s", result.PriceSnapshot.Components[0].UnitPrice)
	}
}

func TestStage4ExplicitPlanSettlesAndPersistsSnapshot(t *testing.T) {
	setupApplyQuotaTestDB(t)
	plan := stage4TokenPlan(0,
		stage4Component(contract.PriceComponentTextInput, "4"),
		stage4Component(contract.PriceComponentTextOutput, "12"),
		stage4Component(contract.PriceComponentCacheRead, "2"),
	)
	plan.ModelName = "gpt-4o"
	plans, err := NormalizeAndValidateModelPricePlans([]contract.ModelPricePlan{plan})
	if err != nil {
		t.Fatalf("validate explicit plan: %v", err)
	}
	if err := pricingstore.ReplaceModelPricePlans(plans); err != nil {
		t.Fatalf("persist explicit plan: %v", err)
	}

	relayInfo := newApplyQuotaTestRelayInfo()
	usage := newApplyQuotaTestUsage()
	settlement, apiErr := CalculateUsage(newUsageTestContext(), relayInfo, usage)
	if apiErr != nil {
		t.Fatalf("explicit plan CalculateUsage: %v", apiErr)
	}
	if settlement.quota != 460 {
		t.Fatalf("explicit plan quota = %d, want equivalent legacy quota 460", settlement.quota)
	}
	relayInfo.FinalPreConsumedQuota = settlement.quota
	if apiErr := ApplyQuota(newApplyQuotaTestContext(), relayInfo, settlement); apiErr != nil {
		t.Fatalf("explicit plan ApplyQuota: %v", apiErr)
	}

	var stored logstore.Log
	require.Eventually(t, func() bool {
		return dbstore.LOG_DB.Where("user_id = ?", applyQuotaTestUserId).
			Order("id DESC").First(&stored).Error == nil
	}, 2*time.Second, 10*time.Millisecond)
	other, err := common.StrToMap(stored.Other)
	if err != nil {
		t.Fatalf("parse consume-log Other: %v", err)
	}
	snapshot, ok := other["billing_price_snapshot"].(map[string]interface{})
	if !ok {
		t.Fatalf("billing price snapshot missing in %+v", other)
	}
	if snapshot["source"] != string(contract.PricePlanSourceExplicit) {
		t.Fatalf("snapshot source = %#v, want explicit", snapshot["source"])
	}
	components, ok := snapshot["components"].([]interface{})
	if !ok || len(components) != 3 {
		t.Fatalf("snapshot components = %#v, want three settled components", snapshot["components"])
	}

	// Replace the live table after settlement. The already persisted Other map
	// must continue to explain the original quota from its embedded snapshot.
	mutatedPlan := stage4TokenPlan(0,
		stage4Component(contract.PriceComponentTextInput, "999"),
		stage4Component(contract.PriceComponentTextOutput, "999"),
		stage4Component(contract.PriceComponentCacheRead, "999"),
	)
	mutatedPlan.ModelName = "gpt-4o"
	mutatedPlans, err := NormalizeAndValidateModelPricePlans([]contract.ModelPricePlan{mutatedPlan})
	if err != nil {
		t.Fatalf("validate mutated plan: %v", err)
	}
	if err := pricingstore.ReplaceModelPricePlans(mutatedPlans); err != nil {
		t.Fatalf("replace price table: %v", err)
	}
	inputComponent, ok := components[0].(map[string]interface{})
	if !ok || inputComponent["unit_price"] != "4" {
		t.Fatalf("persisted input snapshot = %#v, want original unit price 4", components[0])
	}
}

func installStage4Plans(t *testing.T, plan contract.ModelPricePlan) {
	t.Helper()
	plans, err := NormalizeAndValidateModelPricePlans([]contract.ModelPricePlan{plan})
	if err != nil {
		t.Fatalf("validate stage 4 plan: %v", err)
	}
	if err := pricingstore.ReplaceModelPricePlans(plans); err != nil {
		t.Fatalf("persist stage 4 plan: %v", err)
	}
}

func TestStage4ExplicitPerRequestProductionPath(t *testing.T) {
	setupApplyQuotaTestDB(t)
	plan := stage4TokenPlan(0)
	plan.ModelName = "gpt-4o"
	plan.BillingMode = contract.BillingModePerRequest
	plan.Components = []contract.ModelPriceComponent{{
		Component: contract.PriceComponentRequest,
		Unit:      contract.PriceUnitPerRequest,
		UnitPrice: "0.03",
	}}
	installStage4Plans(t, plan)

	settlement, apiErr := CalculateUsage(newUsageTestContext(), newApplyQuotaTestRelayInfo(), newApplyQuotaTestUsage())
	if apiErr != nil {
		t.Fatalf("per-request CalculateUsage: %v", apiErr)
	}
	if settlement.quota != 15000 {
		t.Fatalf("per-request quota = %d, want 15000", settlement.quota)
	}
}

func TestStage4ExplicitFreeProductionPath(t *testing.T) {
	setupApplyQuotaTestDB(t)
	plan := stage4TokenPlan(0)
	plan.ModelName = "gpt-4o"
	plan.BillingMode = contract.BillingModeFree
	plan.Components = nil
	installStage4Plans(t, plan)

	settlement, apiErr := CalculateUsage(newUsageTestContext(), newApplyQuotaTestRelayInfo(), newApplyQuotaTestUsage())
	if apiErr != nil {
		t.Fatalf("free CalculateUsage: %v", apiErr)
	}
	if settlement.quota != 0 {
		t.Fatalf("free quota = %d, want 0", settlement.quota)
	}
}

func TestStage4ExplicitServiceTierAndContextProductionPath(t *testing.T) {
	setupApplyQuotaTestDB(t)
	plan := stage4TokenPlan(0,
		stage4Component(contract.PriceComponentTextInput, "4"),
		stage4Component(contract.PriceComponentTextOutput, "12"),
	)
	plan.ModelName = "gpt-4o"
	plan.ServiceTier = "priority"
	plan.ContextMinTokens = 1000
	installStage4Plans(t, plan)

	relayInfo := newApplyQuotaTestRelayInfo()
	relayInfo.ServiceTierEffective = "priority"
	usage := &shared.Usage{PromptTokens: 1100, CompletionTokens: 0, TotalTokens: 1100}
	settlement, apiErr := CalculateUsage(newApplyQuotaTestContext(), relayInfo, usage)
	if apiErr != nil {
		t.Fatalf("scoped CalculateUsage: %v", apiErr)
	}
	// 1,100 ordinary input tokens at $4/1M with QuotaPerUnit=500,000.
	if settlement.quota != 2200 {
		t.Fatalf("scoped quota = %d, want 2200", settlement.quota)
	}
	if settlement.priceSnapshot == nil ||
		settlement.priceSnapshot.ServiceTier != "priority" ||
		settlement.priceSnapshot.ContextMinTokens != 1000 ||
		settlement.priceSnapshot.ContextTokens != 1100 {
		t.Fatalf("scoped snapshot = %+v", settlement.priceSnapshot)
	}
}

// 旧 ratio 投影的最终取整必须逐入口保持阶段 4 之前的行为：通用/audio/
// realtime 对按量结果 half-up、按次截断；Claude 两种模式都截断。显式价格表
// 按计划配置的 rounding_mode 取整，不受入口行为约束。
func TestStage4LegacyEntryRoundingMatchesPreStage4Behavior(t *testing.T) {
	fractional := decimal.NewFromFloat(7.5)
	legacy := &BillingPriceSnapshot{Source: contract.PricePlanSourceLegacy}
	explicit := &BillingPriceSnapshot{Source: contract.PricePlanSourceExplicit}
	legacyResult := BillingQuotaResult{PriceSnapshot: legacy}
	legacyUsePrice := BillingQuotaResult{PriceSnapshot: legacy, UsePrice: true}
	explicitHalfUp := BillingQuotaResult{PriceSnapshot: explicit, RoundingMode: contract.PriceRoundingHalfUp}
	explicitFloor := BillingQuotaResult{PriceSnapshot: explicit, RoundingMode: contract.PriceRoundingFloor}

	// Claude 入口：截断（与阶段 2 前 int(float64) 与阶段 2 IntPart 一致）。
	assert.Equal(t, 7, roundEntryQuota(fractional, legacyResult, true), "claude legacy must truncate")
	assert.Equal(t, 7, roundEntryQuota(fractional, legacyUsePrice, true), "claude legacy per-price must truncate")
	// audio/realtime 入口：按量 half-up、按次截断。
	assert.Equal(t, 8, roundEntryQuota(fractional, legacyResult, false), "ratio legacy must round half-up")
	assert.Equal(t, 7, roundEntryQuota(fractional, legacyUsePrice, false), "per-price legacy must truncate")
	// 显式价格表拥有自己的取整规则。
	assert.Equal(t, 8, roundEntryQuota(fractional, explicitHalfUp, true))
	assert.Equal(t, 7, roundEntryQuota(fractional, explicitFloor, false))
}

// Claude 入口端到端：旧 ratio 路径最终 quota 截断（7.5 → 7），若误改为
// half-up 会得到 8 并与影子基线 int(float) 产生未分类对拍告警。
func TestPostClaudeConsumeQuotaLegacyTruncatesFractionalQuota(t *testing.T) {
	setupApplyQuotaTestDB(t)
	ctx := newEntryTestContext("tk-claude-truncate")
	relayInfo := newEntryTestRelayInfo(relayconstant.UsageSourceClaude)
	relayInfo.PriceData.ModelRatio = 1.25

	usage := &shared.Usage{PromptTokens: 3, CompletionTokens: 1, TotalTokens: 4}
	require.Nil(t, PostClaudeConsumeQuota(ctx, relayInfo, usage))

	stored := waitForConsumeLogByTokenName(t, "tk-claude-truncate")
	// (3×1 + 1×3) × 1.25 = 7.5 → 旧口径截断为 7。
	assert.Equal(t, 7, stored.Quota)
}

// 未配置组件的显式回退规则：组件级解析先跨计划回退（含 legacy 投影），
// 全部候选都无法提供组件时必须报 ErrBillingPriceConfigMissing，而不是
// 静默按 0 计费。
func TestStage4UnconfiguredComponentsFailExplicitly(t *testing.T) {
	bu := &BillingUsage{
		PromptAggregateTokens: 1000,
		OutputTokens:          500,
		CacheReadTokens:       100,
		CacheWriteTokens:      50,
	}
	priceData := normalizedQuotaTestPriceData()

	t.Run("missing cache component errors instead of silent zero", func(t *testing.T) {
		plan := stage4TokenPlan(60,
			stage4Component(contract.PriceComponentTextInput, "4"),
			stage4Component(contract.PriceComponentTextOutput, "12"),
		)
		_, err := calculateNormalizedQuotaWithPlans(bu, priceData, AudioPricingAbsolute,
			[]contract.ModelPricePlan{plan}, contract.ModelPricePlanQuery{ModelName: "model-a"})
		require.Error(t, err)
		require.ErrorIs(t, err, ErrBillingPriceConfigMissing)
		require.Contains(t, err.Error(), string(contract.PriceComponentCacheRead))
	})

	t.Run("missing input side errors", func(t *testing.T) {
		plan := stage4TokenPlan(61,
			stage4Component(contract.PriceComponentTextOutput, "12"),
			stage4Component(contract.PriceComponentCacheRead, "1"),
			stage4Component(contract.PriceComponentCacheWrite5m, "2.5"),
			stage4Component(contract.PriceComponentCacheWrite1h, "4"),
		)
		_, err := calculateNormalizedQuotaWithPlans(bu, priceData, AudioPricingAbsolute,
			[]contract.ModelPricePlan{plan}, contract.ModelPricePlanQuery{ModelName: "model-a"})
		require.Error(t, err)
		require.ErrorIs(t, err, ErrBillingPriceConfigMissing)
	})
}

// service tier 必须真正改变结算金额，而不只是决定选哪个计划。
func TestStage4ServiceTierPriceChangesSettlement(t *testing.T) {
	bu := stage4Usage()
	priceData := normalizedQuotaTestPriceData()
	defaultPlan := stage4TokenPlan(70,
		stage4Component(contract.PriceComponentInput, "4"),
		stage4Component(contract.PriceComponentOutput, "12"),
		stage4Component(contract.PriceComponentCacheRead, "1"),
		stage4Component(contract.PriceComponentCacheWrite5m, "2.5"),
		stage4Component(contract.PriceComponentCacheWrite1h, "4"),
	)
	priorityPlan := stage4TokenPlan(71,
		stage4Component(contract.PriceComponentInput, "8"),
		stage4Component(contract.PriceComponentOutput, "24"),
		stage4Component(contract.PriceComponentCacheRead, "2"),
		stage4Component(contract.PriceComponentCacheWrite5m, "5"),
		stage4Component(contract.PriceComponentCacheWrite1h, "8"),
	)
	priorityPlan.ServiceTier = "priority"
	plans := []contract.ModelPricePlan{priorityPlan, defaultPlan}

	priorityResult, err := calculateNormalizedQuotaWithPlans(bu, priceData, AudioPricingAbsolute, plans,
		contract.ModelPricePlanQuery{ModelName: "model-a", ServiceTier: "priority"})
	if err != nil {
		t.Fatalf("priority tier quota: %v", err)
	}
	// (700×8 + 500×24 + 100×2 + 50×5) × QuotaPerUnit/1e6 = 9025
	// （InputTokens = 850 聚合 − 100 缓存读取 − 50 缓存写入）
	assertQuotaTotal(t, priorityResult, 9025)
	if priorityResult.PriceSnapshot.ServiceTier != "priority" ||
		priorityResult.PriceSnapshot.Components[0].UnitPrice != "8" {
		t.Fatalf("priority snapshot = %+v", priorityResult.PriceSnapshot)
	}

	defaultResult, err := calculateNormalizedQuotaWithPlans(bu, priceData, AudioPricingAbsolute, plans,
		contract.ModelPricePlanQuery{ModelName: "model-a"})
	if err != nil {
		t.Fatalf("default tier quota: %v", err)
	}
	// 默认档价格为 priority 一半：9025 / 2 = 4512.5，未取整中间结果保留。
	if defaultResult.TokenTotal.Cmp(decimal.NewFromFloat(4512.5)) != 0 {
		t.Fatalf("default tier quota = %s, want 4512.5", defaultResult.TokenTotal)
	}
	if defaultResult.PriceSnapshot.ServiceTier != "" ||
		defaultResult.PriceSnapshot.Components[0].UnitPrice != "4" {
		t.Fatalf("default snapshot = %+v", defaultResult.PriceSnapshot)
	}
}

// image/video/document 与 image 输出组件按各自单价结算，并从父级
// input/output 计费基数中移除，禁止父子重复计费。
func TestStage4ModalComponentPricesSettleAndExcludeParents(t *testing.T) {
	bu := &BillingUsage{
		PromptAggregateTokens: 3000, // text 1000 + image 800 + video 600 + document 600
		OutputTokens:          2000, // text 1500 + image 500
		ImageInputTokens:      intPtr(800),
		VideoInputTokens:      intPtr(600),
		DocumentInputTokens:   intPtr(600),
		ImageOutputTokens:     intPtr(500),
	}
	priceData := normalizedQuotaTestPriceData()
	plan := stage4TokenPlan(80,
		stage4Component(contract.PriceComponentTextInput, "4"),
		stage4Component(contract.PriceComponentImageInput, "10"),
		stage4Component(contract.PriceComponentVideoInput, "20"),
		stage4Component(contract.PriceComponentDocumentInput, "40"),
		stage4Component(contract.PriceComponentTextOutput, "12"),
		stage4Component(contract.PriceComponentImageOutput, "24"),
		stage4Component(contract.PriceComponentCacheRead, "1"),
		stage4Component(contract.PriceComponentCacheWrite5m, "2.5"),
		stage4Component(contract.PriceComponentCacheWrite1h, "4"),
	)
	result, err := calculateNormalizedQuotaWithPlans(bu, priceData, AudioPricingAbsolute,
		[]contract.ModelPricePlan{plan}, contract.ModelPricePlanQuery{ModelName: "model-a"})
	if err != nil {
		t.Fatalf("modal component quota: %v", err)
	}
	// (1000×4 + 800×10 + 600×20 + 600×40 + 1500×12 + 500×24) × 0.5 = 39000
	assertQuotaTotal(t, result, 39000)
	components := make(map[contract.PriceComponent]BillingPriceComponentSnapshot)
	for _, component := range result.PriceSnapshot.Components {
		components[component.Component] = component
	}
	for component, price := range map[contract.PriceComponent]string{
		contract.PriceComponentTextInput:     "4",
		contract.PriceComponentImageInput:    "10",
		contract.PriceComponentVideoInput:    "20",
		contract.PriceComponentDocumentInput: "40",
		contract.PriceComponentTextOutput:    "12",
		contract.PriceComponentImageOutput:   "24",
	} {
		got, ok := components[component]
		if !ok {
			t.Fatalf("missing %s snapshot component", component)
		}
		if got.UnitPrice != price {
			t.Fatalf("%s unit price = %s, want %s", component, got.UnitPrice, price)
		}
	}
	if _, ok := components[contract.PriceComponentInput]; ok {
		t.Fatal("parent input must not appear next to settled children")
	}
	if _, ok := components[contract.PriceComponentOutput]; ok {
		t.Fatal("parent output must not appear next to settled children")
	}
}

// 汇率参与结算金额并写入快照，价格修改后历史日志可据此解释当时结算。
func TestStage4ExchangeRateAppliesAndIsSnapshotted(t *testing.T) {
	bu := &BillingUsage{PromptAggregateTokens: 1_000_000}
	priceData := normalizedQuotaTestPriceData()
	plan := stage4TokenPlan(90,
		stage4Component(contract.PriceComponentInput, "4"),
		stage4Component(contract.PriceComponentOutput, "12"),
		stage4Component(contract.PriceComponentCacheRead, "1"),
		stage4Component(contract.PriceComponentCacheWrite5m, "2.5"),
		stage4Component(contract.PriceComponentCacheWrite1h, "4"),
	)
	plan.ExchangeRate = "7.2"
	result, err := calculateNormalizedQuotaWithPlans(bu, priceData, AudioPricingAbsolute,
		[]contract.ModelPricePlan{plan}, contract.ModelPricePlanQuery{ModelName: "model-a"})
	if err != nil {
		t.Fatalf("exchange rate quota: %v", err)
	}
	// 1,000,000 × 4 / 1e6 × 7.2 × 1 × 500,000 = 14,400,000
	if result.TokenTotal.Cmp(decimal.NewFromInt(14_400_000)) != 0 {
		t.Fatalf("exchange rate total = %s, want 14400000", result.TokenTotal)
	}
	for _, component := range result.PriceSnapshot.Components {
		if component.ExchangeRate != "7.2" {
			t.Fatalf("%s snapshot exchange rate = %s, want 7.2", component.Component, component.ExchangeRate)
		}
	}
}

// 显式免模型计划拥有最终定价权：最低消费 quota=1 是旧 ratio 配置的安全网，
// 只允许作用于 legacy 投影结算。显式 free 计划在所有入口都必须结算 0，
// 即使旧 ModelRatio 仍配置为非零（优先级固定，不允许入口各自选择）。
func TestStage4ExplicitFreePlanStaysZeroAtEveryEntry(t *testing.T) {
	setupApplyQuotaTestDB(t)
	free := stage4TokenPlan(0)
	free.ModelName = "gpt-4o"
	free.BillingMode = contract.BillingModeFree
	free.Components = nil
	installStage4Plans(t, free)

	t.Run("claude entry", func(t *testing.T) {
		ctx := newEntryTestContext("tk-free-claude")
		relayInfo := newEntryTestRelayInfo(relayconstant.UsageSourceClaude)
		usage := &shared.Usage{PromptTokens: 3, CompletionTokens: 1, TotalTokens: 4}
		require.Nil(t, PostClaudeConsumeQuota(ctx, relayInfo, usage))
		assert.Equal(t, 0, waitForConsumeLogByTokenName(t, "tk-free-claude").Quota,
			"explicit free plan must not be bumped to the legacy minimum quota")
	})

	t.Run("audio entry", func(t *testing.T) {
		ctx := newEntryTestContext("tk-free-audio")
		relayInfo := newEntryTestRelayInfo(relayconstant.UsageSourceOpenAIChat)
		usage := &shared.Usage{PromptTokens: 100, CompletionTokens: 100, TotalTokens: 200}
		usage.PromptTokensDetails.TextTokens = 100
		usage.CompletionTokenDetails.TextTokens = 100
		require.Nil(t, PostAudioConsumeQuota(ctx, relayInfo, usage, ""))
		assert.Equal(t, 0, waitForConsumeLogByTokenName(t, "tk-free-audio").Quota,
			"explicit free plan must not be bumped to the legacy minimum quota")
	})

	t.Run("realtime wss entry", func(t *testing.T) {
		ctx := newEntryTestContext("tk-free-wss")
		relayInfo := newEntryTestRelayInfo(relayconstant.UsageSourceOpenAIResponses)
		usage := &shared.RealtimeUsage{
			TotalTokens:  200,
			InputTokens:  100,
			OutputTokens: 100,
			InputTokenDetails: shared.InputTokenDetails{
				TextTokens: 100,
			},
			OutputTokenDetails: shared.OutputTokenDetails{
				TextTokens: 100,
			},
		}
		require.Nil(t, PostWssConsumeQuota(ctx, relayInfo, relayInfo.OriginModelName, usage, ""))
		assert.Equal(t, 0, waitForConsumeLogByTokenName(t, "tk-free-wss").Quota,
			"explicit free plan must not be bumped to the legacy minimum quota")
	})
}

// 音频入口的旧路径与价格表路径等价性（PRD 验收标准）：把旧 ratio 投影原样
// 转换为显式计划（等价配置）后，同一 usage 走 PostAudioConsumeQuota，
// quota 必须与旧投影路径一致，且价格来源切换为 explicit。
func TestStage4AudioEntryExplicitPlanMatchesLegacyQuota(t *testing.T) {
	setupApplyQuotaTestDB(t)
	newAudioUsage := func() *shared.Usage {
		usage := &shared.Usage{
			PromptTokens:     1000, // text(700) + audio(200) + cached(100)
			CompletionTokens: 500,  // text(400) + audio(100)
			TotalTokens:      1500,
		}
		usage.PromptTokensDetails.TextTokens = 700
		usage.PromptTokensDetails.AudioTokens = 200
		usage.PromptTokensDetails.CachedTokens = 100
		usage.CompletionTokenDetails.TextTokens = 400
		usage.CompletionTokenDetails.AudioTokens = 100
		return usage
	}

	legacyRelayInfo := newEntryTestRelayInfo(relayconstant.UsageSourceOpenAIChat)
	require.Nil(t, PostAudioConsumeQuota(newEntryTestContext("tk-audio-equiv-legacy"), legacyRelayInfo, newAudioUsage(), ""))
	legacyLog := waitForConsumeLogByTokenName(t, "tk-audio-equiv-legacy")
	// 独立期望值：(200×32 + 700×4 + 100×2 + 400×12 + 100×64) × 0.5 = 10300。
	// 固定数值让等价性断言可失败：投影或组件结算任一侧漂移都会暴露。
	require.Equal(t, 10300, legacyLog.Quota)

	// 旧投影转等价显式计划：价格与作用域原样保留，仅来源切换。
	legacyPlans := legacyPricePlansForRelay(legacyRelayInfo.OriginModelName, legacyRelayInfo.PriceData)
	require.NotEmpty(t, legacyPlans)
	equivalent := legacyPlans[0]
	equivalent.ID = 0
	equivalent.Source = contract.PricePlanSourceExplicit
	equivalent.ReadOnly = false
	installStage4Plans(t, equivalent)

	explicitRelayInfo := newEntryTestRelayInfo(relayconstant.UsageSourceOpenAIChat)
	require.Nil(t, PostAudioConsumeQuota(newEntryTestContext("tk-audio-equiv-explicit"), explicitRelayInfo, newAudioUsage(), ""))
	explicitLog := waitForConsumeLogByTokenName(t, "tk-audio-equiv-explicit")
	assert.Equal(t, legacyLog.Quota, explicitLog.Quota,
		"等价配置下价格表路径的 quota 必须与旧 ratio 路径一致")
}

// 入口级显式失败：UsePrice 模型挂显式 token 计划但缺缓存组件时，legacy 按
// 次投影不能作为组件回退（组件级候选只含 token 计划），必须以
// billing_config_missing 显式失败（502、不重试、不落消费日志），而不是
// 静默按 0 计费或误用按次价格。
func TestStage4MissingComponentFailsAtEntry(t *testing.T) {
	setupApplyQuotaTestDB(t)
	plan := stage4TokenPlan(0,
		stage4Component(contract.PriceComponentTextInput, "4"),
		stage4Component(contract.PriceComponentTextOutput, "12"),
	)
	plan.ModelName = "gpt-4o"
	installStage4Plans(t, plan)

	relayInfo := newApplyQuotaTestRelayInfo()
	relayInfo.PriceData.UsePrice = true
	relayInfo.PriceData.ModelPrice = 0.01
	relayInfo.PriceData.ModelRatio = 0
	relayInfo.PriceData.CompletionRatio = 0
	relayInfo.PriceData.CacheRatio = 0
	relayInfo.PriceData.CacheCreationRatio = 0
	relayInfo.PriceData.CacheCreation5mRatio = 0
	relayInfo.PriceData.CacheCreation1hRatio = 0

	settlement, apiErr := CalculateUsage(newUsageTestContext(), relayInfo, newApplyQuotaTestUsage())
	require.NotNil(t, apiErr, "missing cache component must fail the settlement explicitly")
	require.Nil(t, settlement)
	assert.Equal(t, http.StatusBadGateway, apiErr.StatusCode)

	// 失败路径不落任何消费日志（调用方保留预扣退款）。
	var count int64
	require.Never(t, func() bool {
		dbstore.LOG_DB.Model(&logstore.Log{}).Where("user_id = ?", applyQuotaTestUserId).Count(&count)
		return count != 0
	}, 300*time.Millisecond, 50*time.Millisecond, "failed settlement must not write a consume log")
}

// 子组件匹配必须遵循与计划解析相同的固定优先级（PRD：优先级禁止调用方各自
// 选择）：多个显式计划都配置同一组件时，按 分组 > 端点 > tier > 上下文 >
// 时间的特异性取价，而不是持久化顺序（store 按 id ASC 返回）。
func TestStage4ChildComponentResolvesByPlanPrecedence(t *testing.T) {
	bu := &BillingUsage{
		PromptAggregateTokens: 1800, // text 1000 + image 800
		OutputTokens:          500,
		ImageInputTokens:      intPtr(800),
	}
	priceData := normalizedQuotaTestPriceData()

	plain := stage4TokenPlan(100,
		stage4Component(contract.PriceComponentInput, "4"),
		stage4Component(contract.PriceComponentOutput, "12"),
		stage4Component(contract.PriceComponentCacheRead, "1"),
		stage4Component(contract.PriceComponentCacheWrite5m, "2.5"),
		stage4Component(contract.PriceComponentCacheWrite1h, "4"),
	)
	plain.Components = append(plain.Components, stage4Component(contract.PriceComponentImageInput, "10"))
	groupScoped := stage4TokenPlan(101,
		stage4Component(contract.PriceComponentTextInput, "6"),
		stage4Component(contract.PriceComponentOutput, "12"),
		stage4Component(contract.PriceComponentCacheRead, "1"),
		stage4Component(contract.PriceComponentCacheWrite5m, "2.5"),
		stage4Component(contract.PriceComponentCacheWrite1h, "4"),
	)
	groupScoped.EffectiveGroup = "svn"
	groupScoped.Components = append(groupScoped.Components, stage4Component(contract.PriceComponentImageInput, "30"))

	// 持久化顺序（id ASC）把无分组计划放在前面；分组计划必须仍按优先级胜出。
	result, err := calculateNormalizedQuotaWithPlans(bu, priceData, AudioPricingAbsolute,
		[]contract.ModelPricePlan{plain, groupScoped},
		contract.ModelPricePlanQuery{ModelName: "model-a", EffectiveGroup: "svn"})
	if err != nil {
		t.Fatalf("precedence quota: %v", err)
	}
	// (1000×6 + 800×30 + 500×12) × 0.5 = 18000；若误用持久化顺序里的
	// image_input=10，会得到 (1000×6 + 800×10 + 500×12) × 0.5 = 10000。
	assertQuotaTotal(t, result, 18000)
	for _, component := range result.PriceSnapshot.Components {
		if component.Component == contract.PriceComponentImageInput && component.UnitPrice != "30" {
			t.Fatalf("image input price = %s, want group-scoped plan price 30", component.UnitPrice)
		}
	}
}

// 通用文本入口的旧路径与价格表路径等价性（PRD 验收标准）：同一 usage 先走
// 旧 ratio 投影，再把投影原样转为显式计划（价格与作用域保留、仅来源切换），
// quota 必须一致且价格来源切换为 explicit。
func TestStage4GenericEntryExplicitPlanMatchesLegacyQuota(t *testing.T) {
	setupApplyQuotaTestDB(t)
	newGenericUsage := func() *shared.Usage {
		usage := &shared.Usage{
			PromptTokens:     1000, // text(850) + cached(100) + write(50)
			CompletionTokens: 500,
			TotalTokens:      1500,
		}
		usage.PromptTokensDetails.CachedTokens = 100
		usage.PromptTokensDetails.CachedCreationTokens = 50
		return usage
	}

	legacyRelayInfo := newEntryTestRelayInfo(relayconstant.UsageSourceOpenAIChat)
	settlement, apiErr := CalculateUsage(newEntryTestContext("tk-generic-equiv-legacy"), legacyRelayInfo, newGenericUsage())
	require.Nil(t, apiErr)
	// 独立期望值：(850×2 + 100×1 + 50×2.5 + 500×6) × 1 = 4925。
	// 固定数值让等价性断言可失败：投影或组件结算任一侧漂移都会暴露。
	require.Equal(t, 4925, settlement.quota)

	legacyPlans := legacyPricePlansForRelay(legacyRelayInfo.OriginModelName, legacyRelayInfo.PriceData)
	require.NotEmpty(t, legacyPlans)
	equivalent := legacyPlans[0]
	equivalent.ID = 0
	equivalent.Source = contract.PricePlanSourceExplicit
	equivalent.ReadOnly = false
	installStage4Plans(t, equivalent)

	explicitRelayInfo := newEntryTestRelayInfo(relayconstant.UsageSourceOpenAIChat)
	explicitSettlement, apiErr := CalculateUsage(newEntryTestContext("tk-generic-equiv-explicit"), explicitRelayInfo, newGenericUsage())
	require.Nil(t, apiErr)
	assert.Equal(t, settlement.quota, explicitSettlement.quota,
		"等价配置下价格表路径的 quota 必须与旧 ratio 路径一致")
	require.NotNil(t, explicitSettlement.priceSnapshot)
	assert.Equal(t, contract.PricePlanSourceExplicit, explicitSettlement.priceSnapshot.Source)
}

// Claude 入口的旧路径与价格表路径等价性（PRD 验收标准），含缓存读取与
// 5m/1h 写入分档。
func TestStage4ClaudeEntryExplicitPlanMatchesLegacyQuota(t *testing.T) {
	setupApplyQuotaTestDB(t)
	newClaudeUsage := func() *shared.Usage {
		usage := &shared.Usage{
			PromptTokens:     1000, // input(700) + read(200) + write(100)
			CompletionTokens: 500,
			TotalTokens:      1500,
		}
		usage.PromptTokensDetails.CachedTokens = 200
		usage.PromptTokensDetails.CachedCreationTokens = 100
		usage.ClaudeCacheCreation5mTokens = 60
		usage.ClaudeCacheCreation1hTokens = 40
		return usage
	}

	legacyRelayInfo := newEntryTestRelayInfo(relayconstant.UsageSourceClaude)
	require.Nil(t, PostClaudeConsumeQuota(newEntryTestContext("tk-claude-equiv-legacy"), legacyRelayInfo, newClaudeUsage()))
	legacyLog := waitForConsumeLogByTokenName(t, "tk-claude-equiv-legacy")
	// (700×2 + 200×1 + 60×2.5 + 40×4 + 500×6) × 1 = 4910。
	require.Equal(t, 4910, legacyLog.Quota)

	legacyPlans := legacyPricePlansForRelay(legacyRelayInfo.OriginModelName, legacyRelayInfo.PriceData)
	require.NotEmpty(t, legacyPlans)
	equivalent := legacyPlans[0]
	equivalent.ID = 0
	equivalent.Source = contract.PricePlanSourceExplicit
	equivalent.ReadOnly = false
	installStage4Plans(t, equivalent)

	explicitRelayInfo := newEntryTestRelayInfo(relayconstant.UsageSourceClaude)
	require.Nil(t, PostClaudeConsumeQuota(newEntryTestContext("tk-claude-equiv-explicit"), explicitRelayInfo, newClaudeUsage()))
	explicitLog := waitForConsumeLogByTokenName(t, "tk-claude-equiv-explicit")
	assert.Equal(t, legacyLog.Quota, explicitLog.Quota,
		"等价配置下价格表路径的 quota 必须与旧 ratio 路径一致")
}

// 模态组件差价的生产链路（Gemini metadata → CalculateUsage → ApplyQuota →
// 消费日志）：image/video/document 输入与 image 输出按组件价结算，父级
// input/output 不再重复计价；billing_details 仍只保存 token 用量，价格
// 快照完整落入 Other。
func TestStage4ModalComponentPricesProductionPath(t *testing.T) {
	setupApplyQuotaTestDB(t)
	plan := stage4TokenPlan(0,
		stage4Component(contract.PriceComponentTextInput, "4"),
		stage4Component(contract.PriceComponentImageInput, "10"),
		stage4Component(contract.PriceComponentVideoInput, "20"),
		stage4Component(contract.PriceComponentDocumentInput, "40"),
		stage4Component(contract.PriceComponentTextOutput, "12"),
		stage4Component(contract.PriceComponentImageOutput, "24"),
		stage4Component(contract.PriceComponentCacheRead, "1"),
		stage4Component(contract.PriceComponentCacheWrite5m, "2.5"),
		stage4Component(contract.PriceComponentCacheWrite1h, "4"),
	)
	plan.ModelName = "gemini-2.5-flash"
	installStage4Plans(t, plan)

	ctx := newEntryTestContext("tk-modal-prod")
	relayInfo := newEntryTestRelayInfo(relayconstant.UsageSourceGemini)
	relayInfo.OriginModelName = "gemini-2.5-flash"
	metadata := &shared.GeminiUsageMetadata{
		PromptTokenCount:     3000, // text 1000 + image 800 + video 600 + document 600
		CandidatesTokenCount: 2000, // text 1500 + image 500
		PromptTokensDetails: []shared.GeminiPromptTokensDetails{
			{Modality: "TEXT", TokenCount: 1000},
			{Modality: "IMAGE", TokenCount: 800},
			{Modality: "VIDEO", TokenCount: 600},
			{Modality: "DOCUMENT", TokenCount: 600},
		},
		CandidatesTokensDetails: []shared.GeminiPromptTokensDetails{
			{Modality: "TEXT", TokenCount: 1500},
			{Modality: "IMAGE", TokenCount: 500},
		},
	}
	usage := GeminiUsageMetadataToOpenAIUsage(*metadata)
	relayInfo.UsageGeminiMetadata = metadata

	settlement, apiErr := CalculateUsage(ctx, relayInfo, &usage)
	require.Nil(t, apiErr)
	// (1000×4 + 800×10 + 600×20 + 600×40 + 1500×12 + 500×24) × 0.5 = 39000
	require.Equal(t, 39000, settlement.quota)
	relayInfo.FinalPreConsumedQuota = settlement.quota
	require.Nil(t, ApplyQuota(ctx, relayInfo, settlement))

	stored := waitForConsumeLogByTokenName(t, "tk-modal-prod")
	assert.Equal(t, 39000, stored.Quota)

	// 价格快照逐组件可解释：六行组件价全部来自显式计划。
	other, err := common.StrToMap(stored.Other)
	require.NoError(t, err)
	snapshot, ok := other["billing_price_snapshot"].(map[string]interface{})
	require.True(t, ok, "billing price snapshot missing in %+v", other)
	assert.Equal(t, string(contract.PricePlanSourceExplicit), snapshot["source"])
	componentsRaw, ok := snapshot["components"].([]interface{})
	require.True(t, ok)
	prices := make(map[string]string)
	for _, raw := range componentsRaw {
		component, ok := raw.(map[string]interface{})
		require.True(t, ok)
		prices[component["component"].(string)] = component["unit_price"].(string)
	}
	wantPrices := map[string]string{
		string(contract.PriceComponentTextInput):     "4",
		string(contract.PriceComponentImageInput):    "10",
		string(contract.PriceComponentVideoInput):    "20",
		string(contract.PriceComponentDocumentInput): "40",
		string(contract.PriceComponentTextOutput):    "12",
		string(contract.PriceComponentImageOutput):   "24",
	}
	assert.Equal(t, wantPrices, prices)

	// billing_details 仍只保存归一化 token 用量：模态明细完整，无价格字段。
	require.NotNil(t, stored.BillingDetails)
	payload, err := ParseBillingDetailsJSON(*stored.BillingDetails)
	require.NoError(t, err)
	assert.Equal(t, 600, payload.Tokens.Input.VideoInput)
	assert.Equal(t, 600, payload.Tokens.Input.DocumentInput)
	assert.Equal(t, 800, payload.Tokens.Input.ImageInput)
	assert.Equal(t, 500, payload.Tokens.Output.ImageOutput)
	assert.NotContains(t, *stored.BillingDetails, "price")
	assert.NotContains(t, *stored.BillingDetails, "quota")
}

// legacy 免模型（ModelRatio=0 投影为 free 计划）：所有入口结算 0，最低
// 消费规则不抬额（旧规则以 ratio!=0 为前提，投影保持该前提）。
func TestStage4LegacyFreeModelStaysZeroAtEveryEntry(t *testing.T) {
	setupApplyQuotaTestDB(t)

	t.Run("generic entry", func(t *testing.T) {
		relayInfo := newApplyQuotaTestRelayInfo()
		relayInfo.PriceData.ModelRatio = 0
		settlement, apiErr := CalculateUsage(newUsageTestContext(), relayInfo, newApplyQuotaTestUsage())
		require.Nil(t, apiErr)
		assert.Equal(t, 0, settlement.quota)
		require.NotNil(t, settlement.priceSnapshot)
		assert.Equal(t, contract.BillingModeFree, settlement.priceSnapshot.BillingMode)
	})

	t.Run("claude entry", func(t *testing.T) {
		ctx := newEntryTestContext("tk-legacy-free-claude")
		relayInfo := newEntryTestRelayInfo(relayconstant.UsageSourceClaude)
		relayInfo.PriceData.ModelRatio = 0
		usage := &shared.Usage{PromptTokens: 3, CompletionTokens: 1, TotalTokens: 4}
		require.Nil(t, PostClaudeConsumeQuota(ctx, relayInfo, usage))
		assert.Equal(t, 0, waitForConsumeLogByTokenName(t, "tk-legacy-free-claude").Quota,
			"legacy free model must not be bumped to the minimum quota")
	})

	t.Run("audio entry", func(t *testing.T) {
		ctx := newEntryTestContext("tk-legacy-free-audio")
		relayInfo := newEntryTestRelayInfo(relayconstant.UsageSourceOpenAIChat)
		relayInfo.PriceData.ModelRatio = 0
		usage := &shared.Usage{PromptTokens: 100, CompletionTokens: 100, TotalTokens: 200}
		usage.PromptTokensDetails.TextTokens = 100
		usage.CompletionTokenDetails.TextTokens = 100
		require.Nil(t, PostAudioConsumeQuota(ctx, relayInfo, usage, ""))
		assert.Equal(t, 0, waitForConsumeLogByTokenName(t, "tk-legacy-free-audio").Quota,
			"legacy free model must not be bumped to the minimum quota")
	})

	t.Run("realtime wss entry", func(t *testing.T) {
		ctx := newEntryTestContext("tk-legacy-free-wss")
		relayInfo := newEntryTestRelayInfo(relayconstant.UsageSourceOpenAIResponses)
		relayInfo.PriceData.ModelRatio = 0
		usage := &shared.RealtimeUsage{
			TotalTokens:  200,
			InputTokens:  100,
			OutputTokens: 100,
			InputTokenDetails: shared.InputTokenDetails{
				TextTokens: 100,
			},
			OutputTokenDetails: shared.OutputTokenDetails{
				TextTokens: 100,
			},
		}
		require.Nil(t, PostWssConsumeQuota(ctx, relayInfo, relayInfo.OriginModelName, usage, ""))
		assert.Equal(t, 0, waitForConsumeLogByTokenName(t, "tk-legacy-free-wss").Quota,
			"legacy free model must not be bumped to the minimum quota")
	})
}

// legacy 上下文分档端到端：命中档位后 quota 按档位倍率结算，未命中保持
// 基础倍率；价格快照记录命中的文档位与档位 tokens。
func TestStage4LegacyContextTierSettlement(t *testing.T) {
	setupApplyQuotaTestDB(t)
	previous := ratio.ContextPricing2JSONString()
	t.Cleanup(func() {
		require.NoError(t, ratio.UpdateContextPricingByJSONString(previous))
	})
	config := `{"gpt-4o":{"enabled":true,"tiers":[
		{"min_tokens":0,"max_tokens":1000,"model_ratio":2,"completion_ratio":3,"cache_ratio":0.5,"create_cache_ratio":1.25,"audio_ratio":8,"audio_completion_ratio":2},
		{"min_tokens":1000,"model_ratio":4,"completion_ratio":6,"cache_ratio":1,"create_cache_ratio":2.5,"audio_ratio":16,"audio_completion_ratio":4}
	]}}`
	require.NoError(t, ratio.UpdateContextPricingByJSONString(config))

	t.Run("below tier boundary keeps base ratios", func(t *testing.T) {
		relayInfo := newApplyQuotaTestRelayInfo()
		usage := &shared.Usage{PromptTokens: 600, CompletionTokens: 300, TotalTokens: 900}
		settlement, apiErr := CalculateUsage(newUsageTestContext(), relayInfo, usage)
		require.Nil(t, apiErr)
		// 基础档：(600 + 300×3) × 2 = 3000（补全倍率相对模型倍率）。
		assert.Equal(t, 3000, settlement.quota)
	})

	t.Run("hit tier settles with tier ratios and records snapshot", func(t *testing.T) {
		relayInfo := newApplyQuotaTestRelayInfo()
		usage := &shared.Usage{PromptTokens: 1100, CompletionTokens: 500, TotalTokens: 1600}
		settlement, apiErr := CalculateUsage(newUsageTestContext(), relayInfo, usage)
		require.Nil(t, apiErr)
		// 命中 [1000,∞) 档：(1100 + 500×6) × 4 = 16400。
		assert.Equal(t, 16400, settlement.quota)
		require.NotNil(t, settlement.priceSnapshot)
		assert.Equal(t, 1000, settlement.priceSnapshot.ContextMinTokens)
		assert.Equal(t, 1600, settlement.priceSnapshot.ContextTokens)
	})
}

func intPtr(value int) *int {
	return &value
}
