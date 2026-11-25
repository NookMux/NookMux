package claude

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
		"claude-fable-5", "claude-fable-5-1", "claude-fable-5-2",
		"claude-mythos-5", "claude-mythos-5-1", "claude-mythos-5-2",
		"claude-opus-4.5", "claude-opus-4.6", "claude-opus-4.7", "claude-opus-4.8",
		"claude-opus-5", "claude-opus-5-5",
		"claude-sonnet-4.5", "claude-sonnet-4.6", "claude-sonnet-5", "claude-sonnet-5-5",
		"claude-haiku-4.5", "claude-haiku-5-5",
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
