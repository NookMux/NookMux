package openai

import (
	"slices"
	"testing"

	"github.com/NookMux/NookMux/internal/config/ratio"
)

// TestModelListCoversPricedCurrentModels 确认厂商第一方在售、内置价格表已配价的型号
// 都在渠道默认模型列表里：默认列表是「获取模型列表」不可用时控制台唯一的型号来源。
func TestModelListCoversPricedCurrentModels(t *testing.T) {
	pricedModels := ratio.GetDefaultModelRatioMap()
	currentModels := []string{
		"chat-latest",
		"gpt-5.2", "gpt-5.2-2025-12-11", "gpt-5.2-pro", "gpt-5.2-pro-2025-12-11",
		"gpt-5.4", "gpt-5.4-2026-03-05", "gpt-5.4-mini", "gpt-5.4-mini-2026-03-17",
		"gpt-5.4-pro", "gpt-5.4-pro-2026-03-05",
		"gpt-5.5", "gpt-5.5-2026-04-23", "gpt-5.5-pro", "gpt-5.5-pro-2026-04-23",
		"gpt-5.6-luna", "gpt-5.6-sol", "gpt-5.6-terra",
		"gpt-6-astra", "gpt-6-luna", "gpt-6-sol", "gpt-6.1-sol", "gpt-6.1-terra",
		"gpt-6.2-astra", "gpt-6.2-sol",
		"gpt-audio-1.5", "gpt-image-2.5-flare", "gpt-image-2.5-sunburst",
		"gpt-image-2.5-sunburst-2026-09-08", "gpt-image-2.6-flare", "gpt-image-2.6-sunburst",
		"gpt-oss-120b", "gpt-oss-20b",
		"gpt-realtime-1.5", "gpt-realtime-2", "gpt-realtime-2.1", "gpt-realtime-2.1-mini",
		"omni-moderation-latest",
		"text-embedding-3-large", "text-embedding-3-small", "text-embedding-ada-002",
	}

	for _, model := range currentModels {
		if _, priced := pricedModels[model]; !priced {
			t.Errorf("builtin ratio table has no entry for %q", model)
		}
		if !slices.Contains(ModelList, model) {
			t.Errorf("ModelList is missing current model %q", model)
		}
	}
}
