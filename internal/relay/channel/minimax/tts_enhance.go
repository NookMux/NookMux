package minimax

import (
	configmodel "github.com/NookMux/NookMux/internal/config/model"
)

// applyModelRedirect 按 TTS 模型重定向表查找。
// 返回映射后的模型名；未命中返回原值。
func applyModelRedirect(originModel string, cfg *configmodel.MiniMaxSettings) string {
	return configmodel.ApplyMiniMaxModelRedirect(originModel, cfg)
}

// extractEmotion 用情绪正则识别文本中的情绪标签。
func extractEmotion(text string, pattern string, redirect map[string]string) (emotion string, cleaned string) {
	return configmodel.ExtractMiniMaxEmotion(text, pattern, redirect)
}

// replaceToneWords 用语气词正则识别文本中的语气词标签，原地替换括号内文本。
func replaceToneWords(text string, pattern string, redirect map[string]string) string {
	return configmodel.ReplaceMiniMaxToneWords(text, pattern, redirect)
}

// extractParenContent 从 "(content)" 格式的字符串提取 content。
func extractParenContent(s string) string {
	return configmodel.ExtractMiniMaxParenContent(s)
}
