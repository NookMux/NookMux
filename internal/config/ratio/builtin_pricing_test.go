package ratio

import (
	"fmt"
	"math"
	"reflect"
	"strconv"
	"testing"
)

// usdPerMillionPerRatio 是倍率折算美元单价的系数：倍率 1 对应 $0.002 / 1K tokens。
const usdPerMillionPerRatio = 2

func formatBuiltinRatio(value *float64) string {
	if value == nil {
		return "<nil>"
	}
	return strconv.FormatFloat(*value, 'g', -1, 64)
}

func TestGetBuiltinModelPricingPerRequest(t *testing.T) {
	got := GetBuiltinModelPricing("gpt-4o-mini-tts")

	if !got.Available {
		t.Fatal("gpt-4o-mini-tts should have a built-in per-request price")
	}
	if got.ModelName != "gpt-4o-mini-tts" {
		t.Fatalf("ModelName = %q, want %q", got.ModelName, "gpt-4o-mini-tts")
	}
	if got.ModelPrice == nil || *got.ModelPrice != defaultModelPrice["gpt-4o-mini-tts"] {
		t.Fatalf("ModelPrice = %s, want %v", formatBuiltinRatio(got.ModelPrice), defaultModelPrice["gpt-4o-mini-tts"])
	}
	if got.ModelRatio != nil {
		t.Fatalf("per-request model must not carry a model ratio, got %s", formatBuiltinRatio(got.ModelRatio))
	}
}

func TestGetBuiltinModelPricingReturnsRawTableEntries(t *testing.T) {
	cases := []struct {
		model string
		want  map[string]float64
	}{
		{
			model: "claude-sonnet-4.5",
			want: map[string]float64{
				"model_ratio":        defaultModelRatio["claude-sonnet-4.5"],
				"completion_ratio":   defaultCompletionRatio["claude-sonnet-4.5"],
				"cache_ratio":        defaultCacheRatio["claude-sonnet-4.5"],
				"create_cache_ratio": defaultCreateCacheRatio["claude-sonnet-4.5"],
			},
		},
		{
			model: "gemini-2.5-pro",
			want: map[string]float64{
				"model_ratio":        defaultModelRatio["gemini-2.5-pro"],
				"completion_ratio":   defaultCompletionRatio["gemini-2.5-pro"],
				"cache_ratio":        defaultCacheRatio["gemini-2.5-pro"],
				"create_cache_ratio": defaultCreateCacheRatio["gemini-2.5-pro"],
				"audio_ratio":        defaultAudioRatio["gemini-2.5-pro"],
			},
		},
		{
			model: "gpt-audio",
			want: map[string]float64{
				"model_ratio":            defaultModelRatio["gpt-audio"],
				"completion_ratio":       defaultCompletionRatio["gpt-audio"],
				"audio_ratio":            defaultAudioRatio["gpt-audio"],
				"audio_completion_ratio": defaultAudioCompletionRatio["gpt-audio"],
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.model, func(t *testing.T) {
			got := GetBuiltinModelPricing(tc.model)
			if !got.Available {
				t.Fatalf("%s should have built-in pricing", tc.model)
			}
			if got.ModelPrice != nil {
				t.Fatalf("per-token model must not carry a per-request price, got %s", formatBuiltinRatio(got.ModelPrice))
			}

			fields := map[string]*float64{
				"model_ratio":            got.ModelRatio,
				"completion_ratio":       got.CompletionRatio,
				"cache_ratio":            got.CacheRatio,
				"create_cache_ratio":     got.CreateCacheRatio,
				"audio_ratio":            got.AudioRatio,
				"audio_completion_ratio": got.AudioCompletionRatio,
			}
			for field, value := range fields {
				want, hasEntry := tc.want[field]
				if !hasEntry {
					if value != nil {
						t.Errorf("%s = %s, want <nil>", field, formatBuiltinRatio(value))
					}
					continue
				}
				if value == nil || *value != want {
					t.Errorf("%s = %s, want %v", field, formatBuiltinRatio(value), want)
				}
			}

			wantContextPricing, hasContextPricing := defaultContextPricing[tc.model]
			if hasContextPricing {
				if !reflect.DeepEqual(got.ContextPricing, &wantContextPricing) {
					t.Fatalf("ContextPricing = %+v, want %+v", got.ContextPricing, wantContextPricing)
				}
			} else if got.ContextPricing != nil {
				t.Fatalf("ContextPricing = %+v, want <nil>", got.ContextPricing)
			}
		})
	}
}

func TestGetBuiltinModelPricingMatchesNormalizedModelNames(t *testing.T) {
	cases := []struct {
		model        string
		tableKey     string
		wantComplete bool
	}{
		{model: "gemini-2.5-pro-thinking-1024", tableKey: "gemini-2.5-pro-thinking-*"},
		{model: "gpt-4o-gizmo-js", tableKey: "gpt-4o-gizmo-*", wantComplete: true},
	}

	for _, tc := range cases {
		t.Run(tc.model, func(t *testing.T) {
			got := GetBuiltinModelPricing(tc.model)
			if got.ModelName != tc.model {
				t.Fatalf("ModelName = %q, want %q", got.ModelName, tc.model)
			}
			if !got.Available {
				t.Fatalf("%s should resolve to built-in entry %q", tc.model, tc.tableKey)
			}
			if got.ModelRatio == nil || *got.ModelRatio != defaultModelRatio[tc.tableKey] {
				t.Errorf("ModelRatio = %s, want %v", formatBuiltinRatio(got.ModelRatio), defaultModelRatio[tc.tableKey])
			}
			if tc.wantComplete {
				if got.CompletionRatio == nil || *got.CompletionRatio != defaultCompletionRatio[tc.tableKey] {
					t.Errorf("CompletionRatio = %s, want %v", formatBuiltinRatio(got.CompletionRatio), defaultCompletionRatio[tc.tableKey])
				}
			}
			// 缓存倍率按原始名称匹配，通配条目不应落到具体的模型名上。
			if got.CacheRatio != nil || got.CreateCacheRatio != nil {
				t.Errorf("CacheRatio = %s, CreateCacheRatio = %s, want <nil>", formatBuiltinRatio(got.CacheRatio), formatBuiltinRatio(got.CreateCacheRatio))
			}
		})
	}
}

func TestGetBuiltinModelPricingUnknownModel(t *testing.T) {
	got := GetBuiltinModelPricing("no-such-model")

	if got.Available {
		t.Fatal("unknown model must not be available")
	}
	if want := (BuiltinModelPricing{ModelName: "no-such-model"}); !reflect.DeepEqual(got, want) {
		t.Fatalf("got %+v, want %+v", got, want)
	}
}

func TestGetBuiltinModelPricingCopiesContextPricingTiers(t *testing.T) {
	got := GetBuiltinModelPricing("claude-sonnet-4.5")

	got.ContextPricing.Tiers[0].ModelRatio = -1
	*got.ContextPricing.Tiers[0].MaxTokens = -1

	builtin := defaultContextPricing["claude-sonnet-4.5"]
	if builtin.Tiers[0].ModelRatio == -1 {
		t.Fatal("built-in tier model ratio must not be modified through the returned value")
	}
	if builtin.Tiers[0].MaxTokens == nil || *builtin.Tiers[0].MaxTokens == -1 {
		t.Fatal("built-in tier max_tokens must not be modified through the returned value")
	}
}

// TestBuiltinClaudePricingMatchesOfficialPrices 按 Claude API 模型 ID 核对内置表的标准档价格
// （https://platform.claude.com/docs/en/about-claude/pricing），由倍率反算 $/1M tokens。
// 1 小时缓存写入价由 5 分钟写入价按固定倍数推导，一并核对。
func TestBuiltinClaudePricingMatchesOfficialPrices(t *testing.T) {
	cases := []struct {
		model                                                string
		input, cacheRead, cacheWrite5m, cacheWrite1h, output float64
	}{
		{"claude-fable-5-2", 8, 0.20, 10, 16, 40},
		{"claude-mythos-5-2", 8, 0.20, 10, 16, 40},
		{"claude-fable-5-1", 10, 0.25, 12.50, 20, 50},
		{"claude-mythos-5-1", 10, 0.25, 12.50, 20, 50},
		{"claude-fable-5", 10, 1, 12.50, 20, 50},
		{"claude-mythos-5", 10, 1, 12.50, 20, 50},
		{"claude-opus-5-5", 4, 0.20, 5, 8, 20},
		{"claude-opus-5", 5, 0.50, 6.25, 10, 25},
		{"claude-opus-4-8", 5, 0.50, 6.25, 10, 25},
		{"claude-opus-4-7", 5, 0.50, 6.25, 10, 25},
		{"claude-opus-4-6", 5, 0.50, 6.25, 10, 25},
		{"claude-opus-4-5", 5, 0.50, 6.25, 10, 25},
		{"claude-opus-4-5-20251101", 5, 0.50, 6.25, 10, 25},
		{"claude-opus-4-1-20250805", 15, 1.50, 18.75, 30, 75},
		{"claude-sonnet-5-5", 2, 0.10, 2.50, 4, 10},
		{"claude-sonnet-5", 2, 0.20, 2.50, 4, 10},
		{"claude-sonnet-4-6", 3, 0.30, 3.75, 6, 15},
		{"claude-haiku-4-5", 1, 0.10, 1.25, 2, 5},
		{"claude-haiku-4-5-20251001", 1, 0.10, 1.25, 2, 5},
	}

	for _, tc := range cases {
		t.Run(tc.model, func(t *testing.T) {
			got := GetBuiltinModelPricing(tc.model)
			if !got.Available {
				t.Fatalf("%s should have built-in pricing", tc.model)
			}
			if got.ModelPrice != nil {
				t.Fatalf("per-token model must not carry a per-request price, got %s", formatBuiltinRatio(got.ModelPrice))
			}
			if got.ModelRatio == nil || got.CompletionRatio == nil || got.CacheRatio == nil || got.CreateCacheRatio == nil {
				t.Fatalf("%s should carry model, completion, cache and create-cache ratios, got %+v", tc.model, got)
			}

			inputPrice := *got.ModelRatio * usdPerMillionPerRatio
			prices := []struct {
				name string
				got  float64
				want float64
			}{
				{"input", inputPrice, tc.input},
				{"cache_read", inputPrice * *got.CacheRatio, tc.cacheRead},
				{"cache_write_5m", inputPrice * *got.CreateCacheRatio, tc.cacheWrite5m},
				{"cache_write_1h", inputPrice * *got.CreateCacheRatio * ClaudeCacheCreation1hMultiplier, tc.cacheWrite1h},
				{"output", inputPrice * *got.CompletionRatio, tc.output},
			}
			for _, price := range prices {
				if math.Abs(price.got-price.want) > 1e-9 {
					t.Errorf("%s = $%.6f / 1M tokens, want $%.6f", price.name, price.got, price.want)
				}
			}
		})
	}
}

// TestBuiltinLongContextPricing 核对按提示长度分段计费的模型在档位边界两侧适用的价格：
// Claude Sonnet 4.5 超过 200K tokens、Haiku 5.5 超过 100K tokens、OpenAI gpt-5.4 及之后的模型
// 超过 272K tokens、MiniMax M3 超过 512K tokens、豆包 Seed 2.0 超过 128K tokens、
// 阿里云百炼 Qwen Plus / Flash / Coder 超过 32K、128K 或 256K tokens 后适用更高一档的价格。
func TestBuiltinLongContextPricing(t *testing.T) {
	InitRatioSettings()
	t.Cleanup(func() {
		_ = UpdateContextPricingByJSONString("{}")
	})

	cases := []struct {
		model                    string
		contextTokens            int
		tier                     string
		input, cacheRead, output float64
	}{
		{"claude-sonnet-4-5", 199_999, "base", 3, 0.30, 15},
		{"claude-sonnet-4-5", 200_000, "tier_1", 6, 0.60, 22.50},
		{"claude-sonnet-4-5-20250929", 250_000, "tier_1", 6, 0.60, 22.50},
		{"claude-haiku-5-5", 99_999, "base", 0.10, 0.01, 0.50},
		{"claude-haiku-5-5", 100_000, "tier_1", 0.50, 0.05, 2.50},
		{"MiniMax-M3", 524_287, "base", 0.30, 0.06, 1.20},
		{"MiniMax-M3", 524_288, "tier_1", 0.60, 0.12, 2.40},
		{"minimax-m3", 600_000, "tier_1", 0.60, 0.12, 2.40},
		{"seed-2-0-lite-260428", 127_999, "base", 0.25, 0.05, 2.00},
		{"seed-2-0-lite-260428", 128_000, "tier_1", 0.50, 0.10, 4.00},
		{"seed-2.0-mini", 200_000, "tier_1", 0.20, 0.04, 0.80},
		{"seed-2-0-pro-260328", 200_000, "tier_1", 1.00, 0.20, 6.00},
		{"gpt-6.2-astra", 271_999, "base", 8, 0.40, 40},
		{"gpt-6.2-astra", 272_000, "tier_1", 16, 0.80, 60},
		{"gpt-6-sol", 300_000, "tier_1", 4, 0.40, 15},
		{"gpt-5.6-terra", 271_999, "base", 2, 0.20, 12},
		{"gpt-5.6-terra", 272_000, "tier_1", 4, 0.40, 18},
		{"gpt-5.6-luna", 272_000, "tier_1", 0.40, 0.04, 1.80},
		{"gpt-5.5-2026-04-23", 300_000, "tier_1", 10, 1, 45},
		{"gpt-5.4-2026-03-05", 300_000, "tier_1", 5, 0.50, 22.50},
		{"gpt-5.4-pro-2026-03-05", 300_000, "tier_1", 60, 0, 270},
		{"qwen3-coder-flash", 31_999, "base", 0.30, 0.06, 1.50},
		{"qwen3-coder-flash", 32_000, "tier_1", 0.50, 0.10, 2.50},
		{"qwen3-coder-flash", 128_000, "tier_2", 0.80, 0.16, 4},
		{"qwen3-coder-flash", 256_000, "tier_3", 1.60, 0.32, 9.60},
		{"qwen3-vl-plus", 32_000, "tier_1", 0.30, 0.06, 2.40},
		{"qwen3-vl-plus", 128_000, "tier_2", 0.60, 0.12, 4.80},
		{"qwen3.7-plus", 255_999, "base", 0.40, 0.08, 1.60},
		{"qwen3.7-plus", 256_000, "tier_1", 1.20, 0.24, 4.80},
		{"qwen3.6-plus", 256_000, "tier_1", 2, 0.20, 6},
		{"qwen3.7-flash", 32_000, "tier_1", 0.10, 0.02, 0.40},
		{"qwen3.7-flash", 256_000, "tier_2", 0.20, 0.04, 0.80},
		{"qwen-flash", 256_000, "tier_1", 0.25, 0.05, 2},
		{"qwen/qwen3.7-plus", 255_999, "base", 0.32, 0.064, 1.28},
		{"qwen/qwen3.7-plus", 256_000, "tier_1", 0.96, 0.192, 3.84},
		{"qwen/qwen3-coder-flash", 32_000, "tier_1", 0.325, 0.065, 1.625},
		{"qwen/qwen3-coder-flash", 128_000, "tier_2", 0.52, 0.104, 2.60},
		{"qwen/qwen3.6-plus", 256_000, "tier_1", 1.30, 1.30, 3.90},
		{"qwen/qwen-plus", 300_000, "tier_1", 0.78, 0.156, 2.34},
	}

	for _, tc := range cases {
		t.Run(fmt.Sprintf("%s@%d", tc.model, tc.contextTokens), func(t *testing.T) {
			result, enabled, err := MatchContextPricingTier(tc.model, tc.contextTokens)
			if err != nil {
				t.Fatalf("MatchContextPricingTier returned error: %v", err)
			}
			if !enabled || result == nil {
				t.Fatal("expected context pricing to be enabled and matched")
			}
			if result.TierName != tc.tier {
				t.Fatalf("tier name = %q, want %q", result.TierName, tc.tier)
			}
			inputPrice := result.Prices.ModelRatio * usdPerMillionPerRatio
			outputPrice := inputPrice * result.Prices.CompletionRatio
			cacheReadPrice := inputPrice * result.Prices.CacheRatio
			if math.Abs(inputPrice-tc.input) > 1e-9 || math.Abs(outputPrice-tc.output) > 1e-9 ||
				math.Abs(cacheReadPrice-tc.cacheRead) > 1e-9 {
				t.Fatalf("tier prices = $%.4f / $%.4f / $%.4f per 1M tokens (input / cache read / output), "+
					"want $%.4f / $%.4f / $%.4f",
					inputPrice, cacheReadPrice, outputPrice, tc.input, tc.cacheRead, tc.output)
			}
		})
	}
}

// TestBuiltinVendorPricingMatchesOfficialPrices 按厂商第一方官方标准价（$/1M tokens）核对内置表里的
// 在售模型计费倍率，取价入口与 relay/helper 的计费取价一致。价格来源：智谱取 Z.ai 美元价；
// DeepSeek 取官方美元高峰价（闲时半价属时段折扣，不计入）；Gemini 取 Developer API 标准档现行生效价；
// Kimi 取 Moonshot 国际站标准价；MiniMax 取国际站标准档；豆包取 ModelArk 国际站标准档；
// OpenAI 取标准档（Batch、Fast、Flex 等服务档不计入）；Qwen 的百炼模型名取阿里云百炼国际站
// 新加坡地域牌价，qwen/ 前缀写法取 OpenRouter 第一方端点价。
// cacheRead 为 -1 表示厂商不提供缓存计价；input 为 0 表示官方免费模型；
// cacheWrite 为 0 表示厂商不收缓存写入费（内置表不带缓存写入倍率），否则为 5 分钟缓存写价。
func TestBuiltinVendorPricingMatchesOfficialPrices(t *testing.T) {
	InitRatioSettings()

	cases := []struct {
		model                                string
		input, cacheRead, output, cacheWrite float64
	}{
		// 智谱 GLM
		{"glm-5.3", 1.4, 0.26, 4.4, 0},
		{"glm-5.3-flash", 0.15, 0.03, 0.50, 0},
		{"glm-5.3-flashx", 0.37, 0.075, 1.25, 0},
		{"glm-5.3-flashx-legal", 0.37, 0.075, 1.25, 0},
		{"glm-5.2", 1.4, 0.26, 4.4, 0},
		{"glm-5.1", 1.4, 0.26, 4.4, 0},
		{"glm-5", 1.0, 0.20, 3.2, 0},
		{"glm-5-turbo", 1.2, 0.24, 4.0, 0},
		{"glm-5v-turbo", 1.2, 0.24, 4.0, 0},
		{"glm-4.7", 0.6, 0.11, 2.2, 0},
		{"glm-4.7-flashx", 0.07, 0.01, 0.4, 0},
		{"glm-4.7-flash", 0, -1, 0, 0},
		{"glm-4.6v-flashx", 0.04, 0.004, 0.4, 0},
		{"glm-4.6v", 0.3, 0.05, 0.9, 0},
		{"glm-4.6v-flash", 0, -1, 0, 0},
		{"glm-4.6", 0.6, 0.11, 2.2, 0},
		{"glm-4.5-airx", 1.1, 0.22, 4.5, 0},
		{"glm-4.5-air", 0.2, 0.03, 1.1, 0},
		{"glm-4.5-x", 2.2, 0.45, 8.9, 0},
		{"glm-4.5v", 0.6, 0.11, 1.8, 0},
		{"glm-4.5-flash", 0, -1, 0, 0},
		{"glm-4.5", 0.6, 0.11, 2.2, 0},
		{"glm-ocr", 0.03, -1, 0.03, 0},
		{"glm-4-32b-0414-128k", 0.1, -1, 0.1, 0},
		// DeepSeek
		{"deepseek-flash", 0.3, 0.006, 1.2, 0},
		{"deepseek-v4-flash", 0.3, 0.006, 1.2, 0},
		{"deepseek-v4-flash-vision-exp", 0.3, 0.006, 1.2, 0},
		// Gemini
		{"gemini-4-neon", 3, 0.15, 15, 0},
		{"gemini-4-carbon", 2, 0.10, 10, 0},
		{"gemini-4-hydrogen", 1.5, 0.075, 7.5, 0},
		{"gemini-4-argon", 2, 0.10, 10, 0},
		{"gemini-3.8-flash", 0.75, 0.075, 3.75, 0},
		{"gemini-3.7-flash", 0.75, 0.075, 3.75, 0},
		{"gemini-3.6-flash", 0.75, 0.075, 3.75, 0},
		{"gemini-3.5-flash-lite", 0.30, 0.03, 2.50, 0},
		{"gemini-nano-banana-2.1", 1.5, -1, 7.5, 0},
		{"gemini-robotics-er-2-preview", 1, 0.10, 5, 0},
		{"gemini-robotics-er-2-streaming-preview", 1, -1, 5, 0},
		// Kimi
		{"kimi-k3.1-pro", 2, 0.20, 10, 2},
		{"kimi-k3.1-pro-highspeed", 4, 0.20, 20, 4},
		{"kimi-k3", 3, 0.30, 15, 3},
		{"kimi-k2.7-code-highspeed", 1.9, 0.38, 8, 0},
		{"kimi-k2.7-code", 0.95, 0.19, 4, 0},
		{"kimi-k2.6", 0.95, 0.16, 4, 0},
		// MiniMax
		{"MiniMax-M3", 0.30, 0.06, 1.20, 0},
		{"minimax-m3", 0.30, 0.06, 1.20, 0},
		{"MiniMax-M3.1-Flash", 0.15, 0.015, 0.60, 0},
		{"minimax-m3.1-flash", 0.15, 0.015, 0.60, 0},
		// 豆包
		{"dola-seed-2-1-turbo-260628", 0.50, 0.10, 2.50, 0},
		{"seed-2-0-lite-260428", 0.25, 0.05, 2.00, 0},
		{"seed-2-0-mini-260428", 0.10, 0.02, 0.40, 0},
		{"seed-2-0-pro-260328", 0.50, 0.10, 3.00, 0},
		{"seed-2-0-code-preview-260328", 0.50, 0.10, 3.00, 0},
		// OpenAI
		{"gpt-6.2-astra", 8, 0.40, 40, 10},
		{"gpt-6.2-sol", 2, 0.10, 10, 2.5},
		{"gpt-6-astra", 10, 1.00, 50, 12.5},
		{"gpt-6.1-sol", 2, 0.10, 10, 2.5},
		{"gpt-6.1-terra", 1, 0.05, 5, 1.25},
		{"gpt-6-sol", 2, 0.20, 10, 2.5},
		{"gpt-6-luna", 0.10, 0.01, 0.50, 0.125},
		{"gpt-5.6", 5, 0.50, 30, 6.25},
		{"gpt-5.6-terra", 2, 0.20, 12, 2.5},
		{"gpt-5.6-luna", 0.20, 0.02, 1.20, 0.25},
		{"gpt-5.5-2026-04-23", 5, 0.50, 30, 0},
		{"gpt-5.5-pro-2026-04-23", 30, -1, 180, 0},
		{"gpt-5.4-mini-2026-03-17", 0.75, 0.075, 4.50, 0},
		{"gpt-5.4-nano-2026-03-17", 0.20, 0.02, 1.25, 0},
		{"chat-latest", 5, 0.50, 30, 0},
		{"gpt-image-2.6-sunburst", 8, 2, 30, 0},
		{"gpt-realtime-2.1", 4, 0.40, 24, 0},
		{"gpt-realtime-2.1-mini", 0.60, 0.06, 2.40, 0},
		{"gpt-audio-1.5", 2.50, -1, 10, 0},
		// Qwen
		{"qwen3.8-max", 2, 0.25, 6, 2.5},
		{"qwen3.7-max", 2.5, 0.50, 7.5, 3.125},
		{"qwen3.7-plus", 0.4, 0.08, 1.6, 0.5},
		{"qwen3.7-flash", 0.03, 0.006, 0.13, 0.0375},
		{"qwen3.6-plus", 0.5, 0.05, 3, 0.625},
		{"qwen3.6-flash", 0.25, 0.025, 1.5, 0.3125},
		{"qwen3.8-flash", 0.15, 0.016, 0.47, 0.2},
		{"qwen3.8-27b", 0.5, 0.1, 3, 0.625},
		{"qwen-max", 1.6, 0.32, 6.4, 0},
		{"qwen-flash", 0.05, 0.01, 0.4, 0.0625},
		{"qwen3-coder-flash", 0.3, 0.06, 1.5, 0.375},
		{"qwen3-vl-plus", 0.2, 0.04, 1.6, 0.25},
		{"qwen3.6-35b-a3b", 0.375, -1, 2.25, 0},
		{"qwen3.5-35b-a3b", 0.25, -1, 2, 0},
		{"qwen/qwen4-max", 1.425, 0.07125, 4.275, 1.78125},
		{"qwen/qwen3.8-max", 2, 0.25, 6, 2.5},
		{"qwen/qwen3.8-flash", 0.15, 0.016, 0.47, 0.2},
		{"qwen/qwen3.7-plus", 0.32, 0.064, 1.28, 0.4},
		{"qwen/qwen3.6-plus", 0.325, -1, 1.95, 0.40625},
		{"qwen/qwen3-coder-flash", 0.195, 0.039, 0.975, 0.24375},
	}

	for _, tc := range cases {
		t.Run(tc.model, func(t *testing.T) {
			modelRatio, ok, matchName := GetModelRatio(tc.model)
			if !ok {
				t.Fatalf("%s should have a built-in model ratio, got none for %s", tc.model, matchName)
			}

			checkPrice := func(name string, got, want float64) {
				if math.Abs(got-want) > 1e-5 {
					t.Errorf("%s: %s = $%.6f / 1M tokens, want $%.6f", tc.model, name, got, want)
				}
			}
			inputPrice := modelRatio * usdPerMillionPerRatio
			checkPrice("input", inputPrice, tc.input)
			if tc.input == 0 {
				return
			}
			checkPrice("output", inputPrice*GetCompletionRatio(tc.model), tc.output)

			createRatio, hasCreateRatio := GetCreateCacheRatio(tc.model)
			if tc.cacheWrite == 0 {
				if hasCreateRatio {
					t.Errorf("%s: cache write ratio = %v, want none", tc.model, createRatio)
				}
			} else {
				if !hasCreateRatio {
					t.Fatalf("%s should carry a cache write ratio, got none", tc.model)
				}
				checkPrice("cache_write", inputPrice*createRatio, tc.cacheWrite)
			}

			cacheRatio, hasCacheRatio := GetCacheRatio(tc.model)
			if tc.cacheRead < 0 {
				if hasCacheRatio {
					t.Errorf("%s: cache ratio = %v, want none", tc.model, cacheRatio)
				}
				return
			}
			if !hasCacheRatio {
				t.Fatalf("%s should carry a cache ratio, got none", tc.model)
			}
			checkPrice("cache_read", inputPrice*cacheRatio, tc.cacheRead)
		})
	}
}

// TestBuiltinCompletionRatioEntryOverridesPrefixRule 核对补全倍率的前缀推导只在没有精确条目时生效：
// gpt-5 系列在表里有条目时按条目计价，没有条目时输出价按输入价的 8 倍推导。
func TestBuiltinCompletionRatioEntryOverridesPrefixRule(t *testing.T) {
	InitRatioSettings()

	cases := []struct {
		model string
		want  float64
	}{
		{"gpt-5.4-mini", 6},
		{"gpt-5.4-nano", 6.25},
		{"gpt-5.4-image-2", 1.875},
		{"gpt-5.5-pro", 6},
		{"gpt-5.6-sol-pro", 6},
		{"gpt-5.3-codex", 8},
		{"gpt-5-not-in-table", 8},
	}

	for _, tc := range cases {
		t.Run(tc.model, func(t *testing.T) {
			if got := GetCompletionRatio(tc.model); got != tc.want {
				t.Fatalf("GetCompletionRatio(%s) = %v, want %v", tc.model, got, tc.want)
			}
		})
	}
}
