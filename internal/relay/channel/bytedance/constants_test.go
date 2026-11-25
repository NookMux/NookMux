package bytedance

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
		"dola-seed-2-1-turbo-260628", "seed-2-0-code-preview-260328",
		"seed-2-0-lite-260428", "seed-2-0-mini-260428", "seed-2-0-pro-260328",
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
