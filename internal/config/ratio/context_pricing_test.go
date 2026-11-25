package ratio

import "testing"

const contextPricingTestConfig = `{
  "tier-test-model": {
    "enabled": true,
    "tiers": [
      {
        "name": "<200K",
        "min_tokens": 0,
        "max_tokens": 200000,
        "model_ratio": 1,
        "completion_ratio": 2,
        "cache_ratio": 0.5,
        "create_cache_ratio": 1.25,
        "audio_ratio": 3,
        "audio_completion_ratio": 4
      },
      {
        "name": ">=200K",
        "min_tokens": 200000,
        "model_ratio": 10,
        "completion_ratio": 20,
        "cache_ratio": 5,
        "create_cache_ratio": 12.5,
        "audio_ratio": 30,
        "audio_completion_ratio": 40
      }
    ]
  }
}`

func TestContextPricingValidationAndLeftClosedBounds(t *testing.T) {
	if err := UpdateContextPricingByJSONString(contextPricingTestConfig); err != nil {
		t.Fatalf("UpdateContextPricingByJSONString returned error: %v", err)
	}
	t.Cleanup(func() {
		_ = UpdateContextPricingByJSONString("{}")
	})

	low, enabled, err := MatchContextPricingTier("tier-test-model", 199999)
	if err != nil {
		t.Fatalf("MatchContextPricingTier low returned error: %v", err)
	}
	if !enabled || low == nil {
		t.Fatalf("expected context pricing to be enabled and matched")
	}
	if low.TierName != "<200K" || low.Prices.ModelRatio != 1 {
		t.Fatalf("low tier = %+v, want <200K with model_ratio 1", low)
	}

	high, enabled, err := MatchContextPricingTier("tier-test-model", 200000)
	if err != nil {
		t.Fatalf("MatchContextPricingTier high returned error: %v", err)
	}
	if !enabled || high == nil {
		t.Fatalf("expected context pricing to be enabled and matched")
	}
	if high.TierName != ">=200K" || high.Prices.ModelRatio != 10 {
		t.Fatalf("high tier = %+v, want >=200K with model_ratio 10", high)
	}
	if high.Prices.CacheCreation1hRatio != high.Prices.CacheCreationRatio*ClaudeCacheCreation1hMultiplier {
		t.Fatalf("1h cache creation ratio = %v, want %v",
			high.Prices.CacheCreation1hRatio,
			high.Prices.CacheCreationRatio*ClaudeCacheCreation1hMultiplier)
	}
}

func TestContextPricingValidationRejectsInvalidConfigs(t *testing.T) {
	tests := []struct {
		name string
		json string
	}{
		{
			name: "overlap",
			json: `{
			  "bad-model": {
			    "enabled": true,
			    "tiers": [
			      {"min_tokens": 0, "max_tokens": 200000, "model_ratio": 1, "completion_ratio": 1, "cache_ratio": 1, "create_cache_ratio": 1, "audio_ratio": 1, "audio_completion_ratio": 1},
			      {"min_tokens": 199999, "model_ratio": 2, "completion_ratio": 2, "cache_ratio": 2, "create_cache_ratio": 2, "audio_ratio": 2, "audio_completion_ratio": 2}
			    ]
			  }
			}`,
		},
		{
			name: "missing required price",
			json: `{
			  "bad-model": {
			    "enabled": true,
			    "tiers": [
			      {"min_tokens": 0, "model_ratio": 1, "completion_ratio": 1, "cache_ratio": 1, "create_cache_ratio": 1, "audio_ratio": 1}
			    ]
			  }
			}`,
		},
		{
			name: "invalid bounds",
			json: `{
			  "bad-model": {
			    "enabled": true,
			    "tiers": [
			      {"min_tokens": 1000, "max_tokens": 1000, "model_ratio": 1, "completion_ratio": 1, "cache_ratio": 1, "create_cache_ratio": 1, "audio_ratio": 1, "audio_completion_ratio": 1}
			    ]
			  }
			}`,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if err := ValidateContextPricing(test.json); err == nil {
				t.Fatalf("ValidateContextPricing expected error")
			}
		})
	}
}

// TestDefaultContextPricingLoaded verifies that InitRatioSettings populates the
// context pricing map with the built-in defaults and that the per-tier match
// logic returns sensible ratios for representative models.
func TestDefaultContextPricingLoaded(t *testing.T) {
	InitRatioSettings()
	t.Cleanup(func() {
		// Reset to empty so other tests don't observe default state.
		_ = UpdateContextPricingByJSONString("{}")
	})

	cases := []struct {
		model         string
		contextTokens int
		wantEnabled   bool
		wantTier      string
		wantMR        float64
	}{
		{model: "claude-sonnet-4.5", contextTokens: 100_000, wantEnabled: true, wantTier: "base", wantMR: 1.5},
		{model: "claude-sonnet-4.5", contextTokens: 250_000, wantEnabled: true, wantTier: "tier_1", wantMR: 3},
		{model: "gpt-5.6-luna", contextTokens: 100_000, wantEnabled: true, wantTier: "base", wantMR: 0.1},
		{model: "gpt-5.6-luna", contextTokens: 300_000, wantEnabled: true, wantTier: "tier_1", wantMR: 0.2},
		{model: "seed-1.6", contextTokens: 64_000, wantEnabled: true, wantTier: "base", wantMR: 0.125},
		{model: "seed-1.6", contextTokens: 200_000, wantEnabled: true, wantTier: "tier_1", wantMR: 0.25},
	}

	for _, tc := range cases {
		t.Run(tc.model+" "+tc.wantTier, func(t *testing.T) {
			res, enabled, err := MatchContextPricingTier(tc.model, tc.contextTokens)
			if err != nil {
				t.Fatalf("MatchContextPricingTier returned error: %v", err)
			}
			if enabled != tc.wantEnabled {
				t.Fatalf("enabled = %v, want %v", enabled, tc.wantEnabled)
			}
			if !enabled {
				return
			}
			if res == nil {
				t.Fatalf("expected non-nil result")
			}
			if res.TierName != tc.wantTier {
				t.Fatalf("tier name = %q, want %q", res.TierName, tc.wantTier)
			}
			if res.Prices.ModelRatio != tc.wantMR {
				t.Fatalf("model_ratio = %v, want %v", res.Prices.ModelRatio, tc.wantMR)
			}
		})
	}
}

// TestDefaultContextPricingSerializationRoundtrip makes sure defaultContextPricing
// values are valid JSON when serialized (no surprises with intPtr / MaxTokens).
func TestDefaultContextPricingSerializationRoundtrip(t *testing.T) {
	InitRatioSettings()
	t.Cleanup(func() {
		_ = UpdateContextPricingByJSONString("{}")
	})

	jsonStr := ContextPricing2JSONString()
	if err := ValidateContextPricing(jsonStr); err != nil {
		t.Fatalf("default context pricing failed validation: %v", err)
	}

	if len(defaultContextPricing) == 0 {
		t.Fatalf("defaultContextPricing is empty")
	}
}
