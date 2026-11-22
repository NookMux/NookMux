package ratio

import (
	"reflect"
	"strconv"
	"testing"
)

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
