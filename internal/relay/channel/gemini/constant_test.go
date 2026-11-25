package gemini

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
		"gemini-2.5-flash-image", "gemini-2.5-flash-lite",
		"gemini-3-flash-preview", "gemini-3-pro-image",
		"gemini-3.1-flash-image", "gemini-3.1-flash-lite", "gemini-3.1-flash-lite-image",
		"gemini-3.1-pro-preview", "gemini-3.1-pro-preview-customtools",
		"gemini-3.5-flash", "gemini-3.5-flash-lite", "gemini-3.6-flash",
		"gemini-3.7-flash", "gemini-3.8-flash",
		"gemini-4-argon", "gemini-4-carbon", "gemini-4-hydrogen", "gemini-4-neon",
		"gemini-embedding-001", "gemini-embedding-2", "gemini-nano-banana-2.1",
		"gemini-robotics-er-2-preview", "gemini-robotics-er-2-streaming-preview",
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
