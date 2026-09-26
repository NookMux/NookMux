package model

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
	"sync"

	"github.com/NookMux/NookMux/internal/config/manager"
	"github.com/NookMux/NookMux/pkg/jsonx"
)

// MiniMaxSettings 定义 MiniMax TTS 增强配置。
// 仅对 MiniMax 渠道的 /v1/audio/speech 生效。
//
// 注意：音色白名单与音色重定向已迁移到数据库表管理（voicestore.Voice），
// 此处只保留一个“是否启用白名单”总开关；具体可用音色、重定向 ID 均由数据库记录决定。
type MiniMaxSettings struct {
	// Enabled 是 TTS 增强（模型重定向、情绪、语气词）总开关，默认关闭。
	Enabled bool `json:"enabled"`
	// ModelRedirect 是 TTS 模型重定向表，键是客户端请求的模型名，值是发给 MiniMax 的真实模型名。
	// 例如 {"tts-1-hd": "speech-02-hd"}
	ModelRedirect map[string]string `json:"model_redirect"`
	// EmotionPattern 识别文本中情绪标签的正则表达式。
	// 默认正则让 emotion 属性可选：带 emotion 属性时提取其值，
	// 不带 emotion 属性（纯 <tts>...</tts>）时也剥离外层标签只保留内部文本，
	// 避免对外的 <tts> 标签原样透传给上游。
	EmotionPattern string `json:"emotion_pattern"`
	// EmotionRedirect 是标签值到 MiniMax voice_setting.emotion 的映射表。
	EmotionRedirect map[string]string `json:"emotion_redirect"`
	// ToneWordPattern 识别文本中语气词标签的正则表达式。
	ToneWordPattern string `json:"tone_word_pattern"`
	// ToneWordRedirect 是语气词标签值到替换值的映射表。
	ToneWordRedirect map[string]string `json:"tone_word_redirect"`

	// VoiceWhitelistEnabled 控制是否启用音色白名单。
	// 启用后，仅允许数据库音色表中“已创建”且“允许使用”的音色 ID 用于 TTS。
	VoiceWhitelistEnabled bool `json:"voice_whitelist_enabled"`

	// CustomVoiceEnabled 是定制音色区域总开关。
	// 关闭后，用户无法使用体验中心的“定制音色”页面发起上传/试听/确认。
	CustomVoiceEnabled bool `json:"custom_voice_enabled"`
	// CustomVoiceGroup 定制音色页面所使用的用户分组。
	// 该分组决定了定制音色整个流程的渠道走向（用于查找 MiniMax 渠道）以及计费分组倍率。
	CustomVoiceGroup string `json:"custom_voice_group"`
	// CustomVoiceBillingModelId 定制音色确认时的扣费模型 ID。
	// 利用 NookMux 的模型消费机制（ModelPrice/ModelRatio）完成扣费，避免出现无扣费或越权计费。
	CustomVoiceBillingModelId string `json:"custom_voice_billing_model_id"`
}

type MiniMaxTTSPolicyResult struct {
	Enabled      bool
	Model        string
	Voice        string
	Text         string
	Emotion      string
	OutputFormat string
}

var defaultMiniMaxSettings = MiniMaxSettings{
	Enabled:          false,
	ModelRedirect:    map[string]string{},
	EmotionPattern:   `<tts(?:\s+emotion="([^"]+)")?>([\s\S]*?)</tts>`,
	EmotionRedirect:  map[string]string{},
	ToneWordPattern:  `\(([^()]+)\)`,
	ToneWordRedirect: map[string]string{},

	VoiceWhitelistEnabled:     false,
	CustomVoiceEnabled:        false,
	CustomVoiceGroup:          "",
	CustomVoiceBillingModelId: "",
}

var minimaxSettings = defaultMiniMaxSettings
var minimaxSettingsMu sync.RWMutex

func init() {
	manager.GlobalConfig.Register("minimax", &minimaxSettings)
}

func GetMiniMaxSettings() *MiniMaxSettings {
	return &minimaxSettings
}

func WithMiniMaxSettingsReadLock(fn func() error) error {
	minimaxSettingsMu.RLock()
	defer minimaxSettingsMu.RUnlock()
	return fn()
}

func WithMiniMaxSettingsWriteLock(fn func() error) error {
	minimaxSettingsMu.Lock()
	defer minimaxSettingsMu.Unlock()
	return fn()
}

func snapshotMiniMaxPolicySettings() MiniMaxSettings {
	minimaxSettingsMu.RLock()
	defer minimaxSettingsMu.RUnlock()
	return MiniMaxSettings{
		Enabled:          minimaxSettings.Enabled,
		ModelRedirect:    cloneStringMap(minimaxSettings.ModelRedirect),
		EmotionPattern:   minimaxSettings.EmotionPattern,
		EmotionRedirect:  cloneStringMap(minimaxSettings.EmotionRedirect),
		ToneWordPattern:  minimaxSettings.ToneWordPattern,
		ToneWordRedirect: cloneStringMap(minimaxSettings.ToneWordRedirect),
	}
}

func cloneStringMap(source map[string]string) map[string]string {
	if len(source) == 0 {
		return map[string]string{}
	}
	cloned := make(map[string]string, len(source))
	for k, v := range source {
		cloned[k] = v
	}
	return cloned
}

func ApplyMiniMaxTTSPolicy(model, voice, text, outputFormat string) MiniMaxTTSPolicyResult {
	cfg := snapshotMiniMaxPolicySettings()
	result := MiniMaxTTSPolicyResult{
		Enabled:      cfg.Enabled,
		Model:        model,
		Voice:        voice,
		Text:         text,
		OutputFormat: outputFormat,
	}
	if !cfg.Enabled {
		return result
	}
	result.Model = ApplyMiniMaxModelRedirect(model, &cfg)
	emotion, cleaned := ExtractMiniMaxEmotion(text, cfg.EmotionPattern, cfg.EmotionRedirect)
	result.Emotion = emotion
	result.Text = ReplaceMiniMaxToneWords(cleaned, cfg.ToneWordPattern, cfg.ToneWordRedirect)
	return result
}

func ApplyMiniMaxModelRedirect(originModel string, cfg *MiniMaxSettings) string {
	if cfg == nil || cfg.ModelRedirect == nil {
		return originModel
	}
	if mapped, ok := cfg.ModelRedirect[originModel]; ok && mapped != "" {
		return mapped
	}
	return originModel
}

func IsMiniMaxVoiceWhitelistEnabled() bool {
	minimaxSettingsMu.RLock()
	defer minimaxSettingsMu.RUnlock()
	return minimaxSettings.VoiceWhitelistEnabled
}

// IsCustomVoiceEnabled 返回定制音色区域是否开放。
func IsCustomVoiceEnabled() bool {
	minimaxSettingsMu.RLock()
	defer minimaxSettingsMu.RUnlock()
	return minimaxSettings.CustomVoiceEnabled
}

// GetCustomVoiceConfig 返回定制音色区域配置的快照（分组、扣费模型 ID）。
func GetCustomVoiceConfig() (group string, billingModelId string) {
	minimaxSettingsMu.RLock()
	defer minimaxSettingsMu.RUnlock()
	return minimaxSettings.CustomVoiceGroup, minimaxSettings.CustomVoiceBillingModelId
}

// CustomVoiceTagsSnapshot 是面向用户侧的 TTS 增强标签只读视图。
// 只暴露标签源值（redirect map 的 key），不暴露上游真实标签（redirect map 的 value），
// 避免前端直接展示上游标签后被现有语气词/情绪逻辑误删除。
type CustomVoiceTagsSnapshot struct {
	Enabled         bool     `json:"enabled"`
	EmotionPattern  string   `json:"emotion_pattern"`
	ToneWordPattern string   `json:"tone_word_pattern"`
	EmotionTags     []string `json:"emotion_tags"`
	ToneWordTags    []string `json:"tone_word_tags"`
}

// GetCustomVoiceTagsSnapshot 返回用户侧可见的 TTS 增强标签快照。
// emotion_tags / tone_word_tags 取各自 redirect map 的 key 并排序，
// 不返回 value，避免把上游真实标签作为用户应输入的标签暴露。
func GetCustomVoiceTagsSnapshot() CustomVoiceTagsSnapshot {
	minimaxSettingsMu.RLock()
	defer minimaxSettingsMu.RUnlock()
	return CustomVoiceTagsSnapshot{
		Enabled:         minimaxSettings.Enabled,
		EmotionPattern:  minimaxSettings.EmotionPattern,
		ToneWordPattern: minimaxSettings.ToneWordPattern,
		EmotionTags:     sortedKeys(minimaxSettings.EmotionRedirect),
		ToneWordTags:    sortedKeys(minimaxSettings.ToneWordRedirect),
	}
}

// sortedKeys 返回 map 的 key 排序后的切片，map 为空时返回 nil（JSON 序列化为 null）。
func sortedKeys(m map[string]string) []string {
	if len(m) == 0 {
		return nil
	}
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func ExtractMiniMaxEmotion(text string, pattern string, redirect map[string]string) (emotion string, cleaned string) {
	if pattern == "" {
		return "", text
	}
	re, err := regexp.Compile(pattern)
	if err != nil {
		return "", text
	}
	matches := re.FindStringSubmatch(text)
	var emotionValue string
	if len(matches) >= 2 {
		emotionValue = resolveMiniMaxTagValue(matches[1], redirect)
	} else if len(matches) == 1 {
		if tagValue := ExtractMiniMaxParenContent(matches[0]); tagValue != "" {
			emotionValue = resolveMiniMaxTagValue(tagValue, redirect)
		}
	}
	cleaned = replaceMiniMaxEmotionTags(text, re)
	cleaned = strings.TrimSpace(cleaned)
	return emotionValue, cleaned
}

func resolveMiniMaxTagValue(tagValue string, redirect map[string]string) string {
	if tagValue == "" {
		return ""
	}
	if len(redirect) == 0 {
		return tagValue
	}
	if mapped, ok := redirect[tagValue]; ok && mapped != "" {
		return mapped
	}
	return ""
}

func replaceMiniMaxEmotionTags(text string, re *regexp.Regexp) string {
	if re.NumSubexp() >= 2 {
		return re.ReplaceAllString(text, "${2}")
	}
	return re.ReplaceAllString(text, "")
}

func ReplaceMiniMaxToneWords(text string, pattern string, redirect map[string]string) string {
	if pattern == "" {
		return text
	}
	re, err := regexp.Compile(pattern)
	if err != nil {
		return text
	}
	targetValues := make(map[string]struct{}, len(redirect))
	for _, v := range redirect {
		if v != "" {
			targetValues[v] = struct{}{}
		}
	}
	return re.ReplaceAllStringFunc(text, func(match string) string {
		tagValue := ExtractMiniMaxParenContent(match)
		if tagValue == "" {
			return match
		}
		if _, isTarget := targetValues[tagValue]; isTarget {
			return ""
		}
		if mapped, ok := redirect[tagValue]; ok && mapped != "" {
			return "(" + mapped + ")"
		}
		return match
	})
}

func ExtractMiniMaxParenContent(s string) string {
	if len(s) < 2 || s[0] != '(' || s[len(s)-1] != ')' {
		return ""
	}
	return s[1 : len(s)-1]
}

// 旧版以 JSON map/array 形式存储的 minimax.* 选项已迁移为数据库表管理。
// 这些键被列入“已移除”集合：系统设置写入时会被拒绝，列表读取时被折叠为空，
// 避免旧前端残留写入又无法生效。

// RemovedMiniMaxLegacyOptions 是已废弃的 minimax.* 大型 JSON 选项键集合。
// GetOptions 在 exclude_large_options 模式下会折叠这些键，UpdateOption 会拒绝写入。
func RemovedMiniMaxLegacyOptions() map[string]struct{} {
	return map[string]struct{}{
		"minimax.voice_whitelist": {},
		"minimax.voice_redirect":  {},
	}
}

// IsMiniMaxLegacyRemovedOption 判断 key 是否为已废弃的 minimax 大型 JSON 选项。
func IsMiniMaxLegacyRemovedOption(key string) bool {
	_, ok := RemovedMiniMaxLegacyOptions()[key]
	return ok
}

// IsMiniMaxStringMapOption 判断 key 是否为仍由系统设置管理的 JSON 映射型 minimax 选项。
// 音色白名单/重定向已迁移到数据库音色表，不再作为系统设置项。
func IsMiniMaxStringMapOption(key string) bool {
	switch key {
	case "minimax.model_redirect", "minimax.emotion_redirect", "minimax.tone_word_redirect":
		return true
	default:
		return false
	}
}

// ValidateMiniMaxOptionValue 校验 minimax.* 系统设置项的值。
// 在 controller.UpdateOption 中保存前调用，校验失败返回错误。
func ValidateMiniMaxOptionValue(key, value string) error {
	switch key {
	case "minimax.model_redirect", "minimax.emotion_redirect", "minimax.tone_word_redirect":
		return validateStringMap(value)
	case "minimax.emotion_pattern", "minimax.tone_word_pattern":
		if value == "" {
			return nil
		}
		if _, err := regexp.Compile(value); err != nil {
			return fmt.Errorf("invalid regex %s: %w", key, err)
		}
		return nil
	case "minimax.custom_voice_billing_model_id":
		// 允许清空，但启用定制音色时不能为空（由 GetCustomVoiceConfigReady 综合判断）。
		// 这里只做 trim，不做存在性校验：模型价格可能是运行时才配置的。
		return nil
	case "minimax.custom_voice_group":
		return nil
	default:
		return nil
	}
}

// GetCustomVoiceConfigReady 返回定制音色功能是否“可用”：
// 开关已开启、扣费模型 ID 非空、分组非空（用于计费分组倍率与渠道路由）。
// 供前端/管理员判断是否真正可用，避免半配置状态导致用户看到定制页面却无法成功扣费。
func GetCustomVoiceConfigReady() bool {
	minimaxSettingsMu.RLock()
	defer minimaxSettingsMu.RUnlock()
	if !minimaxSettings.CustomVoiceEnabled {
		return false
	}
	if strings.TrimSpace(minimaxSettings.CustomVoiceBillingModelId) == "" {
		return false
	}
	if strings.TrimSpace(minimaxSettings.CustomVoiceGroup) == "" {
		return false
	}
	return true
}

// validateStringMap 校验 value 是合法 JSON 且类型为 map[string]string。
// 拒绝空串：空串在 manager.UpdateConfigFromMap 中 unmarshal 失败会被静默跳过，
// 导致配置显示已清空但运行时仍使用旧值。清空请传 "{}"。
func validateStringMap(value string) error {
	var m map[string]string
	if err := jsonx.UnmarshalJsonStr(value, &m); err != nil {
		return fmt.Errorf("invalid JSON for string map: %w", err)
	}
	return nil
}
