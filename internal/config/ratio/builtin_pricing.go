package ratio

import (
	"github.com/NookMux/NookMux/internal/domain/billing/contract"
)

// BuiltinModelPricing 是模型在内置定价表（随版本发布的默认价格与倍率）中的原始条目。
// 指针字段为 nil 表示内置定价表没有该模型的对应条目。
type BuiltinModelPricing struct {
	ModelName            string                         `json:"model_name"`
	Available            bool                           `json:"available"`
	ModelPrice           *float64                       `json:"model_price,omitempty"`
	ModelRatio           *float64                       `json:"model_ratio,omitempty"`
	CompletionRatio      *float64                       `json:"completion_ratio,omitempty"`
	CacheRatio           *float64                       `json:"cache_ratio,omitempty"`
	CreateCacheRatio     *float64                       `json:"create_cache_ratio,omitempty"`
	AudioRatio           *float64                       `json:"audio_ratio,omitempty"`
	AudioCompletionRatio *float64                       `json:"audio_completion_ratio,omitempty"`
	ContextPricing       *contract.ContextPricingConfig `json:"context_pricing,omitempty"`
}

// GetBuiltinModelPricing 返回模型在内置定价表中的原始条目。
// 模型名的匹配规则与运行时计费取价一致：按次价格、倍率、补全倍率、音频倍率与上下文定价按
// FormatMatchingModelName 归一化后的名称匹配，缓存倍率按原始名称匹配。
// 内置定价表没有该模型可计费的条目（按次价格、输入倍率或启用的阶梯定价）时，
// Available 为 false 且不返回任何条目。
func GetBuiltinModelPricing(model string) BuiltinModelPricing {
	name := FormatMatchingModelName(model)

	result := BuiltinModelPricing{
		ModelName:            model,
		ModelPrice:           lookupBuiltinRatio(defaultModelPrice, name),
		ModelRatio:           lookupBuiltinRatio(defaultModelRatio, name),
		CompletionRatio:      lookupBuiltinRatio(defaultCompletionRatio, name),
		CacheRatio:           lookupBuiltinRatio(defaultCacheRatio, model),
		CreateCacheRatio:     lookupBuiltinRatio(defaultCreateCacheRatio, model),
		AudioRatio:           lookupBuiltinRatio(defaultAudioRatio, name),
		AudioCompletionRatio: lookupBuiltinRatio(defaultAudioCompletionRatio, name),
	}
	if config, ok := defaultContextPricing[name]; ok {
		result.ContextPricing = copyContextPricingConfig(config)
	}

	hasContextPricing := result.ContextPricing != nil &&
		result.ContextPricing.Enabled && len(result.ContextPricing.Tiers) > 0
	if result.ModelPrice == nil && result.ModelRatio == nil && !hasContextPricing {
		return BuiltinModelPricing{ModelName: model}
	}
	result.Available = true
	return result
}

func lookupBuiltinRatio(table map[string]float64, name string) *float64 {
	value, ok := table[name]
	if !ok {
		return nil
	}
	return &value
}

// copyContextPricingConfig 深拷贝阶梯定价配置，避免调用方经返回值改动内置定价表。
func copyContextPricingConfig(config contract.ContextPricingConfig) *contract.ContextPricingConfig {
	tiers := make([]contract.ContextPricingTier, len(config.Tiers))
	for i, tier := range config.Tiers {
		if tier.MaxTokens != nil {
			maxTokens := *tier.MaxTokens
			tier.MaxTokens = &maxTokens
		}
		tiers[i] = tier
	}
	return &contract.ContextPricingConfig{Enabled: config.Enabled, Tiers: tiers}
}
