package ratio

import (
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/NookMux/NookMux/internal/domain/shared"
	"github.com/NookMux/NookMux/pkg/jsonx"

	"github.com/NookMux/NookMux/internal/domain/billing/contract"
)

// Claude 1h cache creation keeps the existing fixed relationship to 5m cache creation pricing.
const ClaudeCacheCreation1hMultiplier = 6 / 3.75

var contextPricingMap = shared.NewRWMap[string, contract.ContextPricingConfig]()

func ContextPricing2JSONString() string {
	return contextPricingMap.MarshalJSONString()
}

func UpdateContextPricingByJSONString(jsonStr string) error {
	if err := ValidateContextPricing(jsonStr); err != nil {
		return err
	}
	return shared.LoadFromJsonString(contextPricingMap, jsonStr)
}

func GetContextPricingCopy() map[string]contract.ContextPricingConfig {
	return contextPricingMap.ReadAll()
}

func GetContextPricingConfig(model string) (contract.ContextPricingConfig, bool) {
	model = FormatMatchingModelName(model)
	cfg, ok := contextPricingMap.Get(model)
	return cfg, ok
}

func MatchContextPricingTier(model string, contextTokens int) (*contract.ContextPricingResult, bool, error) {
	cfg, ok := GetContextPricingConfig(model)
	if !ok || !cfg.Enabled {
		return nil, false, nil
	}
	if contextTokens < 0 {
		contextTokens = 0
	}
	if len(cfg.Tiers) == 0 {
		return nil, true, fmt.Errorf("context pricing enabled for model %s but no tiers configured", model)
	}

	tiers := make([]contract.ContextPricingTier, len(cfg.Tiers))
	copy(tiers, cfg.Tiers)
	sort.SliceStable(tiers, func(i, j int) bool {
		return tiers[i].MinTokens < tiers[j].MinTokens
	})

	for idx, tier := range tiers {
		if contextTokens < tier.MinTokens {
			continue
		}
		if tier.MaxTokens != nil && contextTokens >= *tier.MaxTokens {
			continue
		}
		prices := contract.ContextPricingTierPrices{
			ModelRatio:           tier.ModelRatio,
			CompletionRatio:      tier.CompletionRatio,
			CacheRatio:           tier.CacheRatio,
			CacheCreationRatio:   tier.CreateCacheRatio,
			CacheCreation5mRatio: tier.CreateCacheRatio,
			CacheCreation1hRatio: tier.CreateCacheRatio * ClaudeCacheCreation1hMultiplier,
			AudioRatio:           tier.AudioRatio,
			AudioCompletionRatio: tier.AudioCompletionRatio,
		}
		return &contract.ContextPricingResult{
			Enabled:              true,
			ContextTokensForTier: contextTokens,
			TierIndex:            idx,
			TierName:             tier.Name,
			MinTokens:            tier.MinTokens,
			MaxTokens:            tier.MaxTokens,
			Prices:               prices,
		}, true, nil
	}

	return nil, true, fmt.Errorf("context pricing enabled for model %s but no tier matches %d tokens", model, contextTokens)
}

func ApplyContextPricingResult(priceData *contract.PriceData, result *contract.ContextPricingResult) {
	if priceData == nil || result == nil || !result.Enabled {
		return
	}
	priceData.UsePrice = false
	priceData.ModelPrice = -1
	priceData.ModelRatio = result.Prices.ModelRatio
	priceData.CompletionRatio = result.Prices.CompletionRatio
	priceData.CacheRatio = result.Prices.CacheRatio
	priceData.CacheCreationRatio = result.Prices.CacheCreationRatio
	priceData.CacheCreation5mRatio = result.Prices.CacheCreation5mRatio
	priceData.CacheCreation1hRatio = result.Prices.CacheCreation1hRatio
	priceData.AudioRatio = result.Prices.AudioRatio
	priceData.AudioCompletionRatio = result.Prices.AudioCompletionRatio
	priceData.ContextPricing = result
}

type rawContextPricingConfig struct {
	Enabled bool                    `json:"enabled"`
	Tiers   []rawContextPricingTier `json:"tiers"`
}

type rawContextPricingTier struct {
	Name                 string   `json:"name,omitempty"`
	MinTokens            *int     `json:"min_tokens"`
	MaxTokens            *int     `json:"max_tokens,omitempty"`
	ModelRatio           *float64 `json:"model_ratio"`
	CompletionRatio      *float64 `json:"completion_ratio"`
	CacheRatio           *float64 `json:"cache_ratio"`
	CreateCacheRatio     *float64 `json:"create_cache_ratio"`
	AudioRatio           *float64 `json:"audio_ratio"`
	AudioCompletionRatio *float64 `json:"audio_completion_ratio"`
}

func ValidateContextPricing(jsonStr string) error {
	jsonStr = strings.TrimSpace(jsonStr)
	if jsonStr == "" {
		return errors.New("context pricing must be a JSON object")
	}

	raw := make(map[string]rawContextPricingConfig)
	if err := jsonx.Unmarshal([]byte(jsonStr), &raw); err != nil {
		return err
	}

	for modelName, cfg := range raw {
		if !cfg.Enabled {
			continue
		}
		name := strings.TrimSpace(modelName)
		if name == "" {
			return errors.New("context pricing model name cannot be empty")
		}
		if len(cfg.Tiers) == 0 {
			return fmt.Errorf("context pricing for %s must include at least one tier", name)
		}

		tiers := make([]rawContextPricingTier, len(cfg.Tiers))
		copy(tiers, cfg.Tiers)
		for idx, tier := range tiers {
			if tier.MinTokens == nil {
				return fmt.Errorf("context pricing for %s tier %d missing min_tokens", name, idx+1)
			}
			if *tier.MinTokens < 0 {
				return fmt.Errorf("context pricing for %s tier %d min_tokens must be >= 0", name, idx+1)
			}
			if tier.MaxTokens != nil && *tier.MaxTokens <= *tier.MinTokens {
				return fmt.Errorf("context pricing for %s tier %d max_tokens must be greater than min_tokens", name, idx+1)
			}
			if err := validateContextPricingTierPrices(name, idx+1, tier); err != nil {
				return err
			}
		}

		sort.SliceStable(tiers, func(i, j int) bool {
			return *tiers[i].MinTokens < *tiers[j].MinTokens
		})
		for idx := 1; idx < len(tiers); idx++ {
			prev := tiers[idx-1]
			curr := tiers[idx]
			if prev.MaxTokens == nil {
				return fmt.Errorf("context pricing for %s tier %d has no upper bound and overlaps following tiers", name, idx)
			}
			if *curr.MinTokens < *prev.MaxTokens {
				return fmt.Errorf("context pricing for %s tiers overlap around %d tokens", name, *curr.MinTokens)
			}
		}
	}

	return nil
}

func validateContextPricingTierPrices(modelName string, tierIndex int, tier rawContextPricingTier) error {
	required := []struct {
		name  string
		value *float64
	}{
		{"model_ratio", tier.ModelRatio},
		{"completion_ratio", tier.CompletionRatio},
		{"cache_ratio", tier.CacheRatio},
		{"create_cache_ratio", tier.CreateCacheRatio},
		{"audio_ratio", tier.AudioRatio},
		{"audio_completion_ratio", tier.AudioCompletionRatio},
	}
	for _, item := range required {
		if item.value == nil {
			return fmt.Errorf("context pricing for %s tier %d missing %s", modelName, tierIndex, item.name)
		}
		if *item.value < 0 {
			return fmt.Errorf("context pricing for %s tier %d %s must be >= 0", modelName, tierIndex, item.name)
		}
	}
	return nil
}

// defaultContextPricing provides built-in tiered pricing for models that use
// context-tiered billing (e.g., Anthropic Claude Sonnet 4+ above 200K tokens,
// Claude Haiku 5.5 above 100K tokens, OpenAI gpt-5.4+ above 272K tokens,
// Google Gemini 2.5+ Pro above 200K tokens,
// ByteDance Seed 1.6+ above 128K tokens,
// MiniMax M3 above 512K tokens,
// Alibaba Cloud Model Studio Qwen Plus / Flash / Coder above 32K, 128K or 256K tokens).
// Sources: vendor official pricing pages and the OpenRouter provider catalog.
// When a model appears here, the per-tier ratios override the base model/completion
// ratios from defaultModelRatio/defaultCompletionRatio for matched token ranges.
var defaultContextPricing = map[string]contract.ContextPricingConfig{
	"claude-haiku-5-5": {
		Enabled: true,
		Tiers: []contract.ContextPricingTier{
			{
				Name:                 "base",
				MinTokens:            0,
				MaxTokens:            intPtr(100000),
				ModelRatio:           0.05,
				CompletionRatio:      5,
				CacheRatio:           0.1,
				CreateCacheRatio:     1.25,
				AudioRatio:           1,
				AudioCompletionRatio: 1,
			},
			{
				Name:                 "tier_1",
				MinTokens:            100000,
				MaxTokens:            intPtr(1000000),
				ModelRatio:           0.25,
				CompletionRatio:      5,
				CacheRatio:           0.1,
				CreateCacheRatio:     1.25,
				AudioRatio:           1,
				AudioCompletionRatio: 1,
			},
		},
	},
	"claude-sonnet-4-5": {
		Enabled: true,
		Tiers: []contract.ContextPricingTier{
			{
				Name:                 "base",
				MinTokens:            0,
				MaxTokens:            intPtr(200000),
				ModelRatio:           1.5,
				CompletionRatio:      5,
				CacheRatio:           0.1,
				CreateCacheRatio:     1.25,
				AudioRatio:           1,
				AudioCompletionRatio: 1,
			},
			{
				Name:                 "tier_1",
				MinTokens:            200000,
				MaxTokens:            intPtr(1000000),
				ModelRatio:           3,
				CompletionRatio:      3.75,
				CacheRatio:           0.1,
				CreateCacheRatio:     1.25,
				AudioRatio:           1,
				AudioCompletionRatio: 1,
			},
		},
	},
	"claude-sonnet-4-5-20250929": {
		Enabled: true,
		Tiers: []contract.ContextPricingTier{
			{
				Name:                 "base",
				MinTokens:            0,
				MaxTokens:            intPtr(200000),
				ModelRatio:           1.5,
				CompletionRatio:      5,
				CacheRatio:           0.1,
				CreateCacheRatio:     1.25,
				AudioRatio:           1,
				AudioCompletionRatio: 1,
			},
			{
				Name:                 "tier_1",
				MinTokens:            200000,
				MaxTokens:            intPtr(1000000),
				ModelRatio:           3,
				CompletionRatio:      3.75,
				CacheRatio:           0.1,
				CreateCacheRatio:     1.25,
				AudioRatio:           1,
				AudioCompletionRatio: 1,
			},
		},
	},
	"claude-sonnet-4.5": {
		Enabled: true,
		Tiers: []contract.ContextPricingTier{
			{
				Name:                 "base",
				MinTokens:            0,
				MaxTokens:            intPtr(200000),
				ModelRatio:           1.5,
				CompletionRatio:      5,
				CacheRatio:           0.1,
				CreateCacheRatio:     1.25,
				AudioRatio:           1,
				AudioCompletionRatio: 1,
			},
			{
				Name:                 "tier_1",
				MinTokens:            200000,
				MaxTokens:            intPtr(1000000),
				ModelRatio:           3,
				CompletionRatio:      3.75,
				CacheRatio:           0.1,
				CreateCacheRatio:     1.25,
				AudioRatio:           1,
				AudioCompletionRatio: 1,
			},
		},
	},
	"gemini-2.5-pro": {
		Enabled: true,
		Tiers: []contract.ContextPricingTier{
			{
				Name:                 "base",
				MinTokens:            0,
				MaxTokens:            intPtr(200000),
				ModelRatio:           0.625,
				CompletionRatio:      8,
				CacheRatio:           0.1,
				CreateCacheRatio:     0.3,
				AudioRatio:           1,
				AudioCompletionRatio: 1,
			},
			{
				Name:                 "tier_1",
				MinTokens:            200000,
				MaxTokens:            intPtr(1000000),
				ModelRatio:           1.25,
				CompletionRatio:      6,
				CacheRatio:           0.1,
				CreateCacheRatio:     0.15,
				AudioRatio:           1,
				AudioCompletionRatio: 1,
			},
		},
	},
	"gemini-2.5-pro-preview": {
		Enabled: true,
		Tiers: []contract.ContextPricingTier{
			{
				Name:                 "base",
				MinTokens:            0,
				MaxTokens:            intPtr(200000),
				ModelRatio:           0.625,
				CompletionRatio:      8,
				CacheRatio:           0.1,
				CreateCacheRatio:     0.3,
				AudioRatio:           1,
				AudioCompletionRatio: 1,
			},
			{
				Name:                 "tier_1",
				MinTokens:            200000,
				MaxTokens:            intPtr(1000000),
				ModelRatio:           1.25,
				CompletionRatio:      6,
				CacheRatio:           0.1,
				CreateCacheRatio:     0.15,
				AudioRatio:           1,
				AudioCompletionRatio: 1,
			},
		},
	},
	"gemini-2.5-pro-preview-05-06": {
		Enabled: true,
		Tiers: []contract.ContextPricingTier{
			{
				Name:                 "base",
				MinTokens:            0,
				MaxTokens:            intPtr(200000),
				ModelRatio:           0.625,
				CompletionRatio:      8,
				CacheRatio:           0.1,
				CreateCacheRatio:     0.3,
				AudioRatio:           1,
				AudioCompletionRatio: 1,
			},
			{
				Name:                 "tier_1",
				MinTokens:            200000,
				MaxTokens:            intPtr(1000000),
				ModelRatio:           1.25,
				CompletionRatio:      6,
				CacheRatio:           0.1,
				CreateCacheRatio:     0.15,
				AudioRatio:           1,
				AudioCompletionRatio: 1,
			},
		},
	},
	"gemini-3.1-pro-preview": {
		Enabled: true,
		Tiers: []contract.ContextPricingTier{
			{
				Name:                 "base",
				MinTokens:            0,
				MaxTokens:            intPtr(200000),
				ModelRatio:           1,
				CompletionRatio:      6,
				CacheRatio:           0.1,
				CreateCacheRatio:     0.1875,
				AudioRatio:           1,
				AudioCompletionRatio: 1,
			},
			{
				Name:                 "tier_1",
				MinTokens:            200000,
				MaxTokens:            intPtr(1000000),
				ModelRatio:           2,
				CompletionRatio:      4.5,
				CacheRatio:           0.1,
				CreateCacheRatio:     0.09375,
				AudioRatio:           1,
				AudioCompletionRatio: 1,
			},
		},
	},
	"gemini-3.1-pro-preview-customtools": {
		Enabled: true,
		Tiers: []contract.ContextPricingTier{
			{
				Name:                 "base",
				MinTokens:            0,
				MaxTokens:            intPtr(200000),
				ModelRatio:           1,
				CompletionRatio:      6,
				CacheRatio:           0.1,
				CreateCacheRatio:     0.1875,
				AudioRatio:           1,
				AudioCompletionRatio: 1,
			},
			{
				Name:                 "tier_1",
				MinTokens:            200000,
				MaxTokens:            intPtr(1000000),
				ModelRatio:           2,
				CompletionRatio:      4.5,
				CacheRatio:           0.1,
				CreateCacheRatio:     0.09375,
				AudioRatio:           1,
				AudioCompletionRatio: 1,
			},
		},
	},
	"gpt-5.4": {
		Enabled: true,
		Tiers: []contract.ContextPricingTier{
			{
				Name:                 "base",
				MinTokens:            0,
				MaxTokens:            intPtr(272000),
				ModelRatio:           1.25,
				CompletionRatio:      6,
				CacheRatio:           0.1,
				CreateCacheRatio:     0,
				AudioRatio:           1,
				AudioCompletionRatio: 1,
			},
			{
				Name:                 "tier_1",
				MinTokens:            272000,
				MaxTokens:            intPtr(1000000),
				ModelRatio:           2.5,
				CompletionRatio:      4.5,
				CacheRatio:           0.1,
				CreateCacheRatio:     0,
				AudioRatio:           1,
				AudioCompletionRatio: 1,
			},
		},
	},
	"gpt-5.4-2026-03-05": {
		Enabled: true,
		Tiers: []contract.ContextPricingTier{
			{
				Name:                 "base",
				MinTokens:            0,
				MaxTokens:            intPtr(272000),
				ModelRatio:           1.25,
				CompletionRatio:      6,
				CacheRatio:           0.1,
				CreateCacheRatio:     0,
				AudioRatio:           1,
				AudioCompletionRatio: 1,
			},
			{
				Name:                 "tier_1",
				MinTokens:            272000,
				MaxTokens:            intPtr(1000000),
				ModelRatio:           2.5,
				CompletionRatio:      4.5,
				CacheRatio:           0.1,
				CreateCacheRatio:     0,
				AudioRatio:           1,
				AudioCompletionRatio: 1,
			},
		},
	},
	"gpt-5.4-pro": {
		Enabled: true,
		Tiers: []contract.ContextPricingTier{
			{
				Name:                 "base",
				MinTokens:            0,
				MaxTokens:            intPtr(272000),
				ModelRatio:           15,
				CompletionRatio:      6,
				CacheRatio:           0,
				CreateCacheRatio:     0,
				AudioRatio:           1,
				AudioCompletionRatio: 1,
			},
			{
				Name:                 "tier_1",
				MinTokens:            272000,
				MaxTokens:            intPtr(1000000),
				ModelRatio:           30,
				CompletionRatio:      4.5,
				CacheRatio:           0,
				CreateCacheRatio:     0,
				AudioRatio:           1,
				AudioCompletionRatio: 1,
			},
		},
	},
	"gpt-5.4-pro-2026-03-05": {
		Enabled: true,
		Tiers: []contract.ContextPricingTier{
			{
				Name:                 "base",
				MinTokens:            0,
				MaxTokens:            intPtr(272000),
				ModelRatio:           15,
				CompletionRatio:      6,
				CacheRatio:           0,
				CreateCacheRatio:     0,
				AudioRatio:           1,
				AudioCompletionRatio: 1,
			},
			{
				Name:                 "tier_1",
				MinTokens:            272000,
				MaxTokens:            intPtr(1000000),
				ModelRatio:           30,
				CompletionRatio:      4.5,
				CacheRatio:           0,
				CreateCacheRatio:     0,
				AudioRatio:           1,
				AudioCompletionRatio: 1,
			},
		},
	},
	"gpt-5.5": {
		Enabled: true,
		Tiers: []contract.ContextPricingTier{
			{
				Name:                 "base",
				MinTokens:            0,
				MaxTokens:            intPtr(272000),
				ModelRatio:           2.5,
				CompletionRatio:      6,
				CacheRatio:           0.1,
				CreateCacheRatio:     0,
				AudioRatio:           1,
				AudioCompletionRatio: 1,
			},
			{
				Name:                 "tier_1",
				MinTokens:            272000,
				MaxTokens:            intPtr(1000000),
				ModelRatio:           5,
				CompletionRatio:      4.5,
				CacheRatio:           0.1,
				CreateCacheRatio:     0,
				AudioRatio:           1,
				AudioCompletionRatio: 1,
			},
		},
	},
	"gpt-5.5-2026-04-23": {
		Enabled: true,
		Tiers: []contract.ContextPricingTier{
			{
				Name:                 "base",
				MinTokens:            0,
				MaxTokens:            intPtr(272000),
				ModelRatio:           2.5,
				CompletionRatio:      6,
				CacheRatio:           0.1,
				CreateCacheRatio:     0,
				AudioRatio:           1,
				AudioCompletionRatio: 1,
			},
			{
				Name:                 "tier_1",
				MinTokens:            272000,
				MaxTokens:            intPtr(1000000),
				ModelRatio:           5,
				CompletionRatio:      4.5,
				CacheRatio:           0.1,
				CreateCacheRatio:     0,
				AudioRatio:           1,
				AudioCompletionRatio: 1,
			},
		},
	},
	"gpt-5.5-pro": {
		Enabled: true,
		Tiers: []contract.ContextPricingTier{
			{
				Name:                 "base",
				MinTokens:            0,
				MaxTokens:            intPtr(272000),
				ModelRatio:           15,
				CompletionRatio:      6,
				CacheRatio:           0,
				CreateCacheRatio:     0,
				AudioRatio:           1,
				AudioCompletionRatio: 1,
			},
			{
				Name:                 "tier_1",
				MinTokens:            272000,
				MaxTokens:            intPtr(1000000),
				ModelRatio:           30,
				CompletionRatio:      4.5,
				CacheRatio:           0,
				CreateCacheRatio:     0,
				AudioRatio:           1,
				AudioCompletionRatio: 1,
			},
		},
	},
	"gpt-5.5-pro-2026-04-23": {
		Enabled: true,
		Tiers: []contract.ContextPricingTier{
			{
				Name:                 "base",
				MinTokens:            0,
				MaxTokens:            intPtr(272000),
				ModelRatio:           15,
				CompletionRatio:      6,
				CacheRatio:           0,
				CreateCacheRatio:     0,
				AudioRatio:           1,
				AudioCompletionRatio: 1,
			},
			{
				Name:                 "tier_1",
				MinTokens:            272000,
				MaxTokens:            intPtr(1000000),
				ModelRatio:           30,
				CompletionRatio:      4.5,
				CacheRatio:           0,
				CreateCacheRatio:     0,
				AudioRatio:           1,
				AudioCompletionRatio: 1,
			},
		},
	},
	"gpt-5.6-luna": {
		Enabled: true,
		Tiers: []contract.ContextPricingTier{
			{
				Name:                 "base",
				MinTokens:            0,
				MaxTokens:            intPtr(272000),
				ModelRatio:           0.1,
				CompletionRatio:      6,
				CacheRatio:           0.1,
				CreateCacheRatio:     1.25,
				AudioRatio:           1,
				AudioCompletionRatio: 1,
			},
			{
				Name:                 "tier_1",
				MinTokens:            272000,
				MaxTokens:            intPtr(1000000),
				ModelRatio:           0.2,
				CompletionRatio:      4.5,
				CacheRatio:           0.1,
				CreateCacheRatio:     1.25,
				AudioRatio:           1,
				AudioCompletionRatio: 1,
			},
		},
	},
	"gpt-5.6-luna-pro": {
		Enabled: true,
		Tiers: []contract.ContextPricingTier{
			{
				Name:                 "base",
				MinTokens:            0,
				MaxTokens:            intPtr(272000),
				ModelRatio:           0.1,
				CompletionRatio:      6,
				CacheRatio:           0.1,
				CreateCacheRatio:     1.25,
				AudioRatio:           1,
				AudioCompletionRatio: 1,
			},
			{
				Name:                 "tier_1",
				MinTokens:            272000,
				MaxTokens:            intPtr(1000000),
				ModelRatio:           0.2,
				CompletionRatio:      4.5,
				CacheRatio:           0.1,
				CreateCacheRatio:     1.25,
				AudioRatio:           1,
				AudioCompletionRatio: 1,
			},
		},
	},
	"gpt-5.6-sol": {
		Enabled: true,
		Tiers: []contract.ContextPricingTier{
			{
				Name:                 "base",
				MinTokens:            0,
				MaxTokens:            intPtr(272000),
				ModelRatio:           2.5,
				CompletionRatio:      6,
				CacheRatio:           0.1,
				CreateCacheRatio:     1.25,
				AudioRatio:           1,
				AudioCompletionRatio: 1,
			},
			{
				Name:                 "tier_1",
				MinTokens:            272000,
				MaxTokens:            intPtr(1000000),
				ModelRatio:           5,
				CompletionRatio:      4.5,
				CacheRatio:           0.1,
				CreateCacheRatio:     1.25,
				AudioRatio:           1,
				AudioCompletionRatio: 1,
			},
		},
	},
	"gpt-5.6-sol-pro": {
		Enabled: true,
		Tiers: []contract.ContextPricingTier{
			{
				Name:                 "base",
				MinTokens:            0,
				MaxTokens:            intPtr(272000),
				ModelRatio:           2.5,
				CompletionRatio:      6,
				CacheRatio:           0.1,
				CreateCacheRatio:     1.25,
				AudioRatio:           1,
				AudioCompletionRatio: 1,
			},
			{
				Name:                 "tier_1",
				MinTokens:            272000,
				MaxTokens:            intPtr(1000000),
				ModelRatio:           5,
				CompletionRatio:      4.5,
				CacheRatio:           0.1,
				CreateCacheRatio:     1.25,
				AudioRatio:           1,
				AudioCompletionRatio: 1,
			},
		},
	},
	"gpt-5.6-terra": {
		Enabled: true,
		Tiers: []contract.ContextPricingTier{
			{
				Name:                 "base",
				MinTokens:            0,
				MaxTokens:            intPtr(272000),
				ModelRatio:           1,
				CompletionRatio:      6,
				CacheRatio:           0.1,
				CreateCacheRatio:     1.25,
				AudioRatio:           1,
				AudioCompletionRatio: 1,
			},
			{
				Name:                 "tier_1",
				MinTokens:            272000,
				MaxTokens:            intPtr(1000000),
				ModelRatio:           2,
				CompletionRatio:      4.5,
				CacheRatio:           0.1,
				CreateCacheRatio:     1.25,
				AudioRatio:           1,
				AudioCompletionRatio: 1,
			},
		},
	},
	"gpt-5.6-terra-pro": {
		Enabled: true,
		Tiers: []contract.ContextPricingTier{
			{
				Name:                 "base",
				MinTokens:            0,
				MaxTokens:            intPtr(272000),
				ModelRatio:           1,
				CompletionRatio:      6,
				CacheRatio:           0.1,
				CreateCacheRatio:     1.25,
				AudioRatio:           1,
				AudioCompletionRatio: 1,
			},
			{
				Name:                 "tier_1",
				MinTokens:            272000,
				MaxTokens:            intPtr(1000000),
				ModelRatio:           2,
				CompletionRatio:      4.5,
				CacheRatio:           0.1,
				CreateCacheRatio:     1.25,
				AudioRatio:           1,
				AudioCompletionRatio: 1,
			},
		},
	},
	"gpt-5.6": {
		Enabled: true,
		Tiers: []contract.ContextPricingTier{
			{
				Name:                 "base",
				MinTokens:            0,
				MaxTokens:            intPtr(272000),
				ModelRatio:           2.5,
				CompletionRatio:      6,
				CacheRatio:           0.1,
				CreateCacheRatio:     1.25,
				AudioRatio:           1,
				AudioCompletionRatio: 1,
			},
			{
				Name:                 "tier_1",
				MinTokens:            272000,
				MaxTokens:            intPtr(1000000),
				ModelRatio:           5,
				CompletionRatio:      4.5,
				CacheRatio:           0.1,
				CreateCacheRatio:     1.25,
				AudioRatio:           1,
				AudioCompletionRatio: 1,
			},
		},
	},
	"gpt-6-astra": {
		Enabled: true,
		Tiers: []contract.ContextPricingTier{
			{
				Name:                 "base",
				MinTokens:            0,
				MaxTokens:            intPtr(272000),
				ModelRatio:           5,
				CompletionRatio:      5,
				CacheRatio:           0.1,
				CreateCacheRatio:     1.25,
				AudioRatio:           1,
				AudioCompletionRatio: 1,
			},
			{
				Name:                 "tier_1",
				MinTokens:            272000,
				MaxTokens:            intPtr(1000000),
				ModelRatio:           10,
				CompletionRatio:      3.75,
				CacheRatio:           0.1,
				CreateCacheRatio:     1.25,
				AudioRatio:           1,
				AudioCompletionRatio: 1,
			},
		},
	},
	"gpt-6-luna": {
		Enabled: true,
		Tiers: []contract.ContextPricingTier{
			{
				Name:                 "base",
				MinTokens:            0,
				MaxTokens:            intPtr(272000),
				ModelRatio:           0.05,
				CompletionRatio:      5,
				CacheRatio:           0.1,
				CreateCacheRatio:     1.25,
				AudioRatio:           1,
				AudioCompletionRatio: 1,
			},
			{
				Name:                 "tier_1",
				MinTokens:            272000,
				MaxTokens:            intPtr(1000000),
				ModelRatio:           0.1,
				CompletionRatio:      3.75,
				CacheRatio:           0.1,
				CreateCacheRatio:     1.25,
				AudioRatio:           1,
				AudioCompletionRatio: 1,
			},
		},
	},
	"gpt-6-sol": {
		Enabled: true,
		Tiers: []contract.ContextPricingTier{
			{
				Name:                 "base",
				MinTokens:            0,
				MaxTokens:            intPtr(272000),
				ModelRatio:           1,
				CompletionRatio:      5,
				CacheRatio:           0.1,
				CreateCacheRatio:     1.25,
				AudioRatio:           1,
				AudioCompletionRatio: 1,
			},
			{
				Name:                 "tier_1",
				MinTokens:            272000,
				MaxTokens:            intPtr(1000000),
				ModelRatio:           2,
				CompletionRatio:      3.75,
				CacheRatio:           0.1,
				CreateCacheRatio:     1.25,
				AudioRatio:           1,
				AudioCompletionRatio: 1,
			},
		},
	},
	"gpt-6.1-sol": {
		Enabled: true,
		Tiers: []contract.ContextPricingTier{
			{
				Name:                 "base",
				MinTokens:            0,
				MaxTokens:            intPtr(272000),
				ModelRatio:           1,
				CompletionRatio:      5,
				CacheRatio:           0.05,
				CreateCacheRatio:     1.25,
				AudioRatio:           1,
				AudioCompletionRatio: 1,
			},
			{
				Name:                 "tier_1",
				MinTokens:            272000,
				MaxTokens:            intPtr(1000000),
				ModelRatio:           2,
				CompletionRatio:      3.75,
				CacheRatio:           0.05,
				CreateCacheRatio:     1.25,
				AudioRatio:           1,
				AudioCompletionRatio: 1,
			},
		},
	},
	"gpt-6.1-terra": {
		Enabled: true,
		Tiers: []contract.ContextPricingTier{
			{
				Name:                 "base",
				MinTokens:            0,
				MaxTokens:            intPtr(272000),
				ModelRatio:           0.5,
				CompletionRatio:      5,
				CacheRatio:           0.05,
				CreateCacheRatio:     1.25,
				AudioRatio:           1,
				AudioCompletionRatio: 1,
			},
			{
				Name:                 "tier_1",
				MinTokens:            272000,
				MaxTokens:            intPtr(1000000),
				ModelRatio:           1,
				CompletionRatio:      3.75,
				CacheRatio:           0.05,
				CreateCacheRatio:     1.25,
				AudioRatio:           1,
				AudioCompletionRatio: 1,
			},
		},
	},
	"gpt-6.2-astra": {
		Enabled: true,
		Tiers: []contract.ContextPricingTier{
			{
				Name:                 "base",
				MinTokens:            0,
				MaxTokens:            intPtr(272000),
				ModelRatio:           4,
				CompletionRatio:      5,
				CacheRatio:           0.05,
				CreateCacheRatio:     1.25,
				AudioRatio:           1,
				AudioCompletionRatio: 1,
			},
			{
				Name:                 "tier_1",
				MinTokens:            272000,
				MaxTokens:            intPtr(1000000),
				ModelRatio:           8,
				CompletionRatio:      3.75,
				CacheRatio:           0.05,
				CreateCacheRatio:     1.25,
				AudioRatio:           1,
				AudioCompletionRatio: 1,
			},
		},
	},
	"gpt-6.2-sol": {
		Enabled: true,
		Tiers: []contract.ContextPricingTier{
			{
				Name:                 "base",
				MinTokens:            0,
				MaxTokens:            intPtr(272000),
				ModelRatio:           1,
				CompletionRatio:      5,
				CacheRatio:           0.05,
				CreateCacheRatio:     1.25,
				AudioRatio:           1,
				AudioCompletionRatio: 1,
			},
			{
				Name:                 "tier_1",
				MinTokens:            272000,
				MaxTokens:            intPtr(1000000),
				ModelRatio:           2,
				CompletionRatio:      3.75,
				CacheRatio:           0.05,
				CreateCacheRatio:     1.25,
				AudioRatio:           1,
				AudioCompletionRatio: 1,
			},
		},
	},
	"MiniMax-M3": {
		Enabled: true,
		Tiers: []contract.ContextPricingTier{
			{
				Name:                 "base",
				MinTokens:            0,
				MaxTokens:            intPtr(524288),
				ModelRatio:           0.15,
				CompletionRatio:      4,
				CacheRatio:           0.2,
				CreateCacheRatio:     0,
				AudioRatio:           1,
				AudioCompletionRatio: 1,
			},
			{
				Name:                 "tier_1",
				MinTokens:            524288,
				MaxTokens:            intPtr(1000000),
				ModelRatio:           0.3,
				CompletionRatio:      4,
				CacheRatio:           0.2,
				CreateCacheRatio:     0,
				AudioRatio:           1,
				AudioCompletionRatio: 1,
			},
		},
	},
	"minimax-m3": {
		Enabled: true,
		Tiers: []contract.ContextPricingTier{
			{
				Name:                 "base",
				MinTokens:            0,
				MaxTokens:            intPtr(524288),
				ModelRatio:           0.15,
				CompletionRatio:      4,
				CacheRatio:           0.2,
				CreateCacheRatio:     0,
				AudioRatio:           1,
				AudioCompletionRatio: 1,
			},
			{
				Name:                 "tier_1",
				MinTokens:            524288,
				MaxTokens:            intPtr(1000000),
				ModelRatio:           0.3,
				CompletionRatio:      4,
				CacheRatio:           0.2,
				CreateCacheRatio:     0,
				AudioRatio:           1,
				AudioCompletionRatio: 1,
			},
		},
	},
	"seed-1.6": {
		Enabled: true,
		Tiers: []contract.ContextPricingTier{
			{
				Name:                 "base",
				MinTokens:            0,
				MaxTokens:            intPtr(128000),
				ModelRatio:           0.125,
				CompletionRatio:      8,
				CacheRatio:           0,
				CreateCacheRatio:     0,
				AudioRatio:           1,
				AudioCompletionRatio: 1,
			},
			{
				Name:                 "tier_1",
				MinTokens:            128000,
				MaxTokens:            intPtr(1000000),
				ModelRatio:           0.25,
				CompletionRatio:      8,
				CacheRatio:           0,
				CreateCacheRatio:     0,
				AudioRatio:           1,
				AudioCompletionRatio: 1,
			},
		},
	},
	"seed-1.6-flash": {
		Enabled: true,
		Tiers: []contract.ContextPricingTier{
			{
				Name:                 "base",
				MinTokens:            0,
				MaxTokens:            intPtr(128000),
				ModelRatio:           0.0375,
				CompletionRatio:      4,
				CacheRatio:           0,
				CreateCacheRatio:     0,
				AudioRatio:           1,
				AudioCompletionRatio: 1,
			},
			{
				Name:                 "tier_1",
				MinTokens:            128000,
				MaxTokens:            intPtr(1000000),
				ModelRatio:           0.05,
				CompletionRatio:      8,
				CacheRatio:           0,
				CreateCacheRatio:     0,
				AudioRatio:           1,
				AudioCompletionRatio: 1,
			},
		},
	},
	"seed-2.0-lite": {
		Enabled: true,
		Tiers: []contract.ContextPricingTier{
			{
				Name:                 "base",
				MinTokens:            0,
				MaxTokens:            intPtr(128000),
				ModelRatio:           0.125,
				CompletionRatio:      8,
				CacheRatio:           0.2,
				CreateCacheRatio:     0,
				AudioRatio:           1,
				AudioCompletionRatio: 1,
			},
			{
				Name:                 "tier_1",
				MinTokens:            128000,
				MaxTokens:            intPtr(1000000),
				ModelRatio:           0.25,
				CompletionRatio:      8,
				CacheRatio:           0.2,
				CreateCacheRatio:     0,
				AudioRatio:           1,
				AudioCompletionRatio: 1,
			},
		},
	},
	"seed-2.0-mini": {
		Enabled: true,
		Tiers: []contract.ContextPricingTier{
			{
				Name:                 "base",
				MinTokens:            0,
				MaxTokens:            intPtr(128000),
				ModelRatio:           0.05,
				CompletionRatio:      4,
				CacheRatio:           0.2,
				CreateCacheRatio:     0,
				AudioRatio:           1,
				AudioCompletionRatio: 1,
			},
			{
				Name:                 "tier_1",
				MinTokens:            128000,
				MaxTokens:            intPtr(1000000),
				ModelRatio:           0.1,
				CompletionRatio:      4,
				CacheRatio:           0.2,
				CreateCacheRatio:     0,
				AudioRatio:           1,
				AudioCompletionRatio: 1,
			},
		},
	},
	"seed-2-0-code-preview-260328": {
		Enabled: true,
		Tiers: []contract.ContextPricingTier{
			{
				Name:                 "base",
				MinTokens:            0,
				MaxTokens:            intPtr(128000),
				ModelRatio:           0.25,
				CompletionRatio:      6,
				CacheRatio:           0.2,
				CreateCacheRatio:     0,
				AudioRatio:           1,
				AudioCompletionRatio: 1,
			},
			{
				Name:                 "tier_1",
				MinTokens:            128000,
				MaxTokens:            intPtr(1000000),
				ModelRatio:           0.5,
				CompletionRatio:      6,
				CacheRatio:           0.2,
				CreateCacheRatio:     0,
				AudioRatio:           1,
				AudioCompletionRatio: 1,
			},
		},
	},
	"seed-2-0-lite-260428": {
		Enabled: true,
		Tiers: []contract.ContextPricingTier{
			{
				Name:                 "base",
				MinTokens:            0,
				MaxTokens:            intPtr(128000),
				ModelRatio:           0.125,
				CompletionRatio:      8,
				CacheRatio:           0.2,
				CreateCacheRatio:     0,
				AudioRatio:           1,
				AudioCompletionRatio: 1,
			},
			{
				Name:                 "tier_1",
				MinTokens:            128000,
				MaxTokens:            intPtr(1000000),
				ModelRatio:           0.25,
				CompletionRatio:      8,
				CacheRatio:           0.2,
				CreateCacheRatio:     0,
				AudioRatio:           1,
				AudioCompletionRatio: 1,
			},
		},
	},
	"seed-2-0-mini-260428": {
		Enabled: true,
		Tiers: []contract.ContextPricingTier{
			{
				Name:                 "base",
				MinTokens:            0,
				MaxTokens:            intPtr(128000),
				ModelRatio:           0.05,
				CompletionRatio:      4,
				CacheRatio:           0.2,
				CreateCacheRatio:     0,
				AudioRatio:           1,
				AudioCompletionRatio: 1,
			},
			{
				Name:                 "tier_1",
				MinTokens:            128000,
				MaxTokens:            intPtr(1000000),
				ModelRatio:           0.1,
				CompletionRatio:      4,
				CacheRatio:           0.2,
				CreateCacheRatio:     0,
				AudioRatio:           1,
				AudioCompletionRatio: 1,
			},
		},
	},
	"seed-2-0-pro-260328": {
		Enabled: true,
		Tiers: []contract.ContextPricingTier{
			{
				Name:                 "base",
				MinTokens:            0,
				MaxTokens:            intPtr(128000),
				ModelRatio:           0.25,
				CompletionRatio:      6,
				CacheRatio:           0.2,
				CreateCacheRatio:     0,
				AudioRatio:           1,
				AudioCompletionRatio: 1,
			},
			{
				Name:                 "tier_1",
				MinTokens:            128000,
				MaxTokens:            intPtr(1000000),
				ModelRatio:           0.5,
				CompletionRatio:      6,
				CacheRatio:           0.2,
				CreateCacheRatio:     0,
				AudioRatio:           1,
				AudioCompletionRatio: 1,
			},
		},
	},
	"qwen-flash": {
		Enabled: true,
		Tiers: []contract.ContextPricingTier{
			{
				Name:                 "base",
				MinTokens:            0,
				MaxTokens:            intPtr(256000),
				ModelRatio:           0.025,
				CompletionRatio:      8,
				CacheRatio:           0.2,
				CreateCacheRatio:     1.25,
				AudioRatio:           1,
				AudioCompletionRatio: 1,
			},
			{
				Name:                 "tier_1",
				MinTokens:            256000,
				MaxTokens:            intPtr(1000000),
				ModelRatio:           0.125,
				CompletionRatio:      8,
				CacheRatio:           0.2,
				CreateCacheRatio:     1.25,
				AudioRatio:           1,
				AudioCompletionRatio: 1,
			},
		},
	},
	"qwen3-coder-flash": {
		Enabled: true,
		Tiers: []contract.ContextPricingTier{
			{
				Name:                 "base",
				MinTokens:            0,
				MaxTokens:            intPtr(32000),
				ModelRatio:           0.15,
				CompletionRatio:      5,
				CacheRatio:           0.2,
				CreateCacheRatio:     1.25,
				AudioRatio:           1,
				AudioCompletionRatio: 1,
			},
			{
				Name:                 "tier_1",
				MinTokens:            32000,
				MaxTokens:            intPtr(128000),
				ModelRatio:           0.25,
				CompletionRatio:      5,
				CacheRatio:           0.2,
				CreateCacheRatio:     1.25,
				AudioRatio:           1,
				AudioCompletionRatio: 1,
			},
			{
				Name:                 "tier_2",
				MinTokens:            128000,
				MaxTokens:            intPtr(256000),
				ModelRatio:           0.4,
				CompletionRatio:      5,
				CacheRatio:           0.2,
				CreateCacheRatio:     1.25,
				AudioRatio:           1,
				AudioCompletionRatio: 1,
			},
			{
				Name:                 "tier_3",
				MinTokens:            256000,
				MaxTokens:            intPtr(1000000),
				ModelRatio:           0.8,
				CompletionRatio:      6,
				CacheRatio:           0.2,
				CreateCacheRatio:     1.25,
				AudioRatio:           1,
				AudioCompletionRatio: 1,
			},
		},
	},
	"qwen3-vl-plus": {
		Enabled: true,
		Tiers: []contract.ContextPricingTier{
			{
				Name:                 "base",
				MinTokens:            0,
				MaxTokens:            intPtr(32000),
				ModelRatio:           0.1,
				CompletionRatio:      8,
				CacheRatio:           0.2,
				CreateCacheRatio:     1.25,
				AudioRatio:           1,
				AudioCompletionRatio: 1,
			},
			{
				Name:                 "tier_1",
				MinTokens:            32000,
				MaxTokens:            intPtr(128000),
				ModelRatio:           0.15,
				CompletionRatio:      8,
				CacheRatio:           0.2,
				CreateCacheRatio:     1.25,
				AudioRatio:           1,
				AudioCompletionRatio: 1,
			},
			{
				Name:                 "tier_2",
				MinTokens:            128000,
				MaxTokens:            intPtr(1000000),
				ModelRatio:           0.3,
				CompletionRatio:      8,
				CacheRatio:           0.2,
				CreateCacheRatio:     1.25,
				AudioRatio:           1,
				AudioCompletionRatio: 1,
			},
		},
	},
	"qwen3.6-flash": {
		Enabled: true,
		Tiers: []contract.ContextPricingTier{
			{
				Name:                 "base",
				MinTokens:            0,
				MaxTokens:            intPtr(256000),
				ModelRatio:           0.125,
				CompletionRatio:      6,
				CacheRatio:           0.1,
				CreateCacheRatio:     1.25,
				AudioRatio:           1,
				AudioCompletionRatio: 1,
			},
			{
				Name:                 "tier_1",
				MinTokens:            256000,
				MaxTokens:            intPtr(1000000),
				ModelRatio:           0.5,
				CompletionRatio:      4,
				CacheRatio:           0.1,
				CreateCacheRatio:     1.25,
				AudioRatio:           1,
				AudioCompletionRatio: 1,
			},
		},
	},
	"qwen3.6-plus": {
		Enabled: true,
		Tiers: []contract.ContextPricingTier{
			{
				Name:                 "base",
				MinTokens:            0,
				MaxTokens:            intPtr(256000),
				ModelRatio:           0.25,
				CompletionRatio:      6,
				CacheRatio:           0.1,
				CreateCacheRatio:     1.25,
				AudioRatio:           1,
				AudioCompletionRatio: 1,
			},
			{
				Name:                 "tier_1",
				MinTokens:            256000,
				MaxTokens:            intPtr(1000000),
				ModelRatio:           1,
				CompletionRatio:      3,
				CacheRatio:           0.1,
				CreateCacheRatio:     1.25,
				AudioRatio:           1,
				AudioCompletionRatio: 1,
			},
		},
	},
	"qwen3.7-flash": {
		Enabled: true,
		Tiers: []contract.ContextPricingTier{
			{
				Name:                 "base",
				MinTokens:            0,
				MaxTokens:            intPtr(32000),
				ModelRatio:           0.015,
				CompletionRatio:      4.333333,
				CacheRatio:           0.2,
				CreateCacheRatio:     1.25,
				AudioRatio:           1,
				AudioCompletionRatio: 1,
			},
			{
				Name:                 "tier_1",
				MinTokens:            32000,
				MaxTokens:            intPtr(256000),
				ModelRatio:           0.05,
				CompletionRatio:      4,
				CacheRatio:           0.2,
				CreateCacheRatio:     1.25,
				AudioRatio:           1,
				AudioCompletionRatio: 1,
			},
			{
				Name:                 "tier_2",
				MinTokens:            256000,
				MaxTokens:            intPtr(1000000),
				ModelRatio:           0.1,
				CompletionRatio:      4,
				CacheRatio:           0.2,
				CreateCacheRatio:     1.25,
				AudioRatio:           1,
				AudioCompletionRatio: 1,
			},
		},
	},
	"qwen3.7-plus": {
		Enabled: true,
		Tiers: []contract.ContextPricingTier{
			{
				Name:                 "base",
				MinTokens:            0,
				MaxTokens:            intPtr(256000),
				ModelRatio:           0.2,
				CompletionRatio:      4,
				CacheRatio:           0.2,
				CreateCacheRatio:     1.25,
				AudioRatio:           1,
				AudioCompletionRatio: 1,
			},
			{
				Name:                 "tier_1",
				MinTokens:            256000,
				MaxTokens:            intPtr(1000000),
				ModelRatio:           0.6,
				CompletionRatio:      4,
				CacheRatio:           0.2,
				CreateCacheRatio:     1.25,
				AudioRatio:           1,
				AudioCompletionRatio: 1,
			},
		},
	},
	"qwen/qwen-plus": {
		Enabled: true,
		Tiers: []contract.ContextPricingTier{
			{
				Name:                 "base",
				MinTokens:            0,
				MaxTokens:            intPtr(256000),
				ModelRatio:           0.13,
				CompletionRatio:      3,
				CacheRatio:           0.2,
				CreateCacheRatio:     1.25,
				AudioRatio:           1,
				AudioCompletionRatio: 1,
			},
			{
				Name:                 "tier_1",
				MinTokens:            256000,
				MaxTokens:            intPtr(1000000),
				ModelRatio:           0.39,
				CompletionRatio:      3,
				CacheRatio:           0.2,
				CreateCacheRatio:     1.25,
				AudioRatio:           1,
				AudioCompletionRatio: 1,
			},
		},
	},
	"qwen/qwen3-coder-flash": {
		Enabled: true,
		Tiers: []contract.ContextPricingTier{
			{
				Name:                 "base",
				MinTokens:            0,
				MaxTokens:            intPtr(32000),
				ModelRatio:           0.0975,
				CompletionRatio:      5,
				CacheRatio:           0.2,
				CreateCacheRatio:     1.25,
				AudioRatio:           1,
				AudioCompletionRatio: 1,
			},
			{
				Name:                 "tier_1",
				MinTokens:            32000,
				MaxTokens:            intPtr(128000),
				ModelRatio:           0.1625,
				CompletionRatio:      5,
				CacheRatio:           0.2,
				CreateCacheRatio:     1.25,
				AudioRatio:           1,
				AudioCompletionRatio: 1,
			},
			{
				Name:                 "tier_2",
				MinTokens:            128000,
				MaxTokens:            intPtr(1000000),
				ModelRatio:           0.26,
				CompletionRatio:      5,
				CacheRatio:           0.2,
				CreateCacheRatio:     1.25,
				AudioRatio:           1,
				AudioCompletionRatio: 1,
			},
		},
	},
	"qwen/qwen3.6-flash": {
		Enabled: true,
		Tiers: []contract.ContextPricingTier{
			{
				Name:                 "base",
				MinTokens:            0,
				MaxTokens:            intPtr(256000),
				ModelRatio:           0.09375,
				CompletionRatio:      6,
				CacheRatio:           1,
				CreateCacheRatio:     1.25,
				AudioRatio:           1,
				AudioCompletionRatio: 1,
			},
			{
				Name:                 "tier_1",
				MinTokens:            256000,
				MaxTokens:            intPtr(1000000),
				ModelRatio:           0.375,
				CompletionRatio:      4,
				CacheRatio:           1,
				CreateCacheRatio:     1.25,
				AudioRatio:           1,
				AudioCompletionRatio: 1,
			},
		},
	},
	"qwen/qwen3.6-plus": {
		Enabled: true,
		Tiers: []contract.ContextPricingTier{
			{
				Name:                 "base",
				MinTokens:            0,
				MaxTokens:            intPtr(256000),
				ModelRatio:           0.1625,
				CompletionRatio:      6,
				CacheRatio:           1,
				CreateCacheRatio:     1.25,
				AudioRatio:           1,
				AudioCompletionRatio: 1,
			},
			{
				Name:                 "tier_1",
				MinTokens:            256000,
				MaxTokens:            intPtr(1000000),
				ModelRatio:           0.65,
				CompletionRatio:      3,
				CacheRatio:           1,
				CreateCacheRatio:     1.25,
				AudioRatio:           1,
				AudioCompletionRatio: 1,
			},
		},
	},
	"qwen/qwen3.7-flash": {
		Enabled: true,
		Tiers: []contract.ContextPricingTier{
			{
				Name:                 "base",
				MinTokens:            0,
				MaxTokens:            intPtr(32000),
				ModelRatio:           0.015,
				CompletionRatio:      4.333333,
				CacheRatio:           0.2,
				CreateCacheRatio:     1.25,
				AudioRatio:           1,
				AudioCompletionRatio: 1,
			},
			{
				Name:                 "tier_1",
				MinTokens:            32000,
				MaxTokens:            intPtr(256000),
				ModelRatio:           0.05,
				CompletionRatio:      4,
				CacheRatio:           0.2,
				CreateCacheRatio:     1.25,
				AudioRatio:           1,
				AudioCompletionRatio: 1,
			},
			{
				Name:                 "tier_2",
				MinTokens:            256000,
				MaxTokens:            intPtr(1000000),
				ModelRatio:           0.1,
				CompletionRatio:      4,
				CacheRatio:           0.2,
				CreateCacheRatio:     1.25,
				AudioRatio:           1,
				AudioCompletionRatio: 1,
			},
		},
	},
	"qwen/qwen3.7-plus": {
		Enabled: true,
		Tiers: []contract.ContextPricingTier{
			{
				Name:                 "base",
				MinTokens:            0,
				MaxTokens:            intPtr(256000),
				ModelRatio:           0.16,
				CompletionRatio:      4,
				CacheRatio:           0.2,
				CreateCacheRatio:     1.25,
				AudioRatio:           1,
				AudioCompletionRatio: 1,
			},
			{
				Name:                 "tier_1",
				MinTokens:            256000,
				MaxTokens:            intPtr(1000000),
				ModelRatio:           0.48,
				CompletionRatio:      4,
				CacheRatio:           0.2,
				CreateCacheRatio:     1.25,
				AudioRatio:           1,
				AudioCompletionRatio: 1,
			},
		},
	},
}

// intPtr is a small helper to build *int literals for ContextPricingTier.MaxTokens.
func intPtr(v int) *int { return &v }
