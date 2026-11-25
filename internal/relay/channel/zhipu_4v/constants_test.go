package zhipu_4v

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
		"glm-4.5", "glm-4.5-air", "glm-4.5-airx", "glm-4.5-flash",
		"glm-4.6", "glm-4.6v", "glm-4.6v-flash", "glm-4.6v-flashx",
		"glm-4.7", "glm-4.7-flash", "glm-4.7-flashx",
		"glm-5", "glm-5-turbo", "glm-5.1", "glm-5.2",
		"glm-5.3", "glm-5.3-flash", "glm-5.3-flashx", "glm-5.3-flashx-legal",
		"glm-5v-turbo", "glm-ocr",
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
