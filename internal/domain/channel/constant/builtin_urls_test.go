package constant

import "testing"

// TestBuiltinChannelURLOptionsConsistency 校验内置 URL 预设表与渠道常量保持一致：
// 预设 value 必须是可被后端识别的存储值（官方默认地址或套餐标识），
// 且所有套餐标识都必须暴露给前端下拉，防止新增套餐时遗漏。
func TestBuiltinChannelURLOptionsConsistency(t *testing.T) {
	// 智谱国际站无 ChannelBaseURLs 对应项（默认为国内站），作为独立站点预设放行
	const zhipuInternationalSite = "https://api.z.ai"

	exposedPlans := make(map[string]bool)
	for channelType, options := range BuiltinChannelURLOptions {
		seenValues := make(map[string]bool, len(options))
		for _, option := range options {
			if option.Value == "" {
				t.Errorf("type %d: preset value must not be empty", channelType)
				continue
			}
			if option.LabelKey == "" {
				t.Errorf("type %d: preset %q has empty label_key", channelType, option.Value)
			}
			if seenValues[option.Value] {
				t.Errorf("type %d: duplicate preset value %q", channelType, option.Value)
			}
			seenValues[option.Value] = true

			if _, isPlan := ChannelSpecialBases[option.Value]; isPlan {
				exposedPlans[option.Value] = true
				continue
			}
			if channelType == ChannelTypeZhipu_v4 && option.Value == zhipuInternationalSite {
				continue
			}
			if option.Value != ChannelBaseURLs[channelType] {
				t.Errorf("type %d: preset %q does not match ChannelBaseURLs default %q",
					channelType, option.Value, ChannelBaseURLs[channelType])
			}
		}
	}

	for planKey := range ChannelSpecialBases {
		if !exposedPlans[planKey] {
			t.Errorf("plan key %q is not exposed in any BuiltinChannelURLOptions", planKey)
		}
	}
}
