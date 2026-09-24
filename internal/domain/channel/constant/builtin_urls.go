package constant

// BuiltinURLOption 描述某内置渠道类型在编辑表单中可选的 base_url 预设。
// Value 必须与渠道存储值完全一致：普通站点预设取 ChannelBaseURLs 的官方地址，
// 套餐预设取 ChannelSpecialBases 的套餐标识，选中后原样落库，后端无需转换。
// LabelKey 为前端 i18n 完整 key，由前端翻译展示。
type BuiltinURLOption struct {
	Value    string `json:"value"`
	LabelKey string `json:"label_key"`
}

// BuiltinChannelURLOptions 按渠道类型列出 base_url 预设选项；
// 未列出的类型（Azure/Custom/AWS/Vertex 等）由前端保持自由输入。
var BuiltinChannelURLOptions = map[int][]BuiltinURLOption{
	ChannelTypeOpenAI: {
		{Value: "https://api.openai.com", LabelKey: "channels.fields.baseUrlOfficial"},
	},
	ChannelTypeAnthropic: {
		{Value: "https://api.anthropic.com", LabelKey: "channels.fields.baseUrlOfficial"},
	},
	ChannelTypeGemini: {
		{Value: "https://generativelanguage.googleapis.com", LabelKey: "channels.fields.baseUrlOfficial"},
	},
	ChannelTypeOpenRouter: {
		{Value: "https://openrouter.ai/api", LabelKey: "channels.fields.baseUrlOfficial"},
	},
	ChannelTypeSiliconFlow: {
		{Value: "https://api.siliconflow.cn", LabelKey: "channels.fields.baseUrlOfficial"},
	},
	ChannelTypeDeepSeek: {
		{Value: "https://api.deepseek.com", LabelKey: "channels.fields.baseUrlOfficial"},
	},
	ChannelTypeByteDance: {
		{Value: "https://ark.cn-beijing.volces.com/api/v3", LabelKey: "channels.fields.baseUrlOfficial"},
	},
	ChannelTypeMoonshot: {
		{Value: "https://api.moonshot.cn", LabelKey: "channels.fields.baseUrlOfficial"},
		{Value: "kimi-coding-plan", LabelKey: "channels.fields.planKimiCodingPlan"},
	},
	ChannelTypeZhipu_v4: {
		{Value: "https://open.bigmodel.cn", LabelKey: "channels.fields.baseUrlDomestic"},
		// 智谱国际站站点，adaptor 按 {base}/api/paas/v4 与 {base}/api/anthropic 拼路径
		{Value: "https://api.z.ai", LabelKey: "channels.fields.baseUrlInternational"},
		{Value: "glm-coding-plan", LabelKey: "channels.fields.planGlmCodingPlan"},
		{Value: "glm-coding-plan-international", LabelKey: "channels.fields.planGlmCodingPlanInternational"},
	},
	ChannelTypeMiniMax: {
		{Value: "https://api.minimaxi.com/v1", LabelKey: "channels.fields.baseUrlOfficial"},
		{Value: "minimax-coding-plan", LabelKey: "channels.fields.planMinimaxCodingPlan"},
		{Value: "minimax-coding-plan-international", LabelKey: "channels.fields.planMinimaxCodingPlanInternational"},
	},
	ChannelTypeXiaomi: {
		{Value: "https://api.xiaomimimo.com", LabelKey: "channels.fields.baseUrlOfficial"},
		{Value: "xiaomi-coding-plan", LabelKey: "channels.fields.planXiaomiCodingPlan"},
		{Value: "xiaomi-coding-plan-sgp", LabelKey: "channels.fields.planXiaomiCodingPlanSingapore"},
		{Value: "xiaomi-coding-plan-ams", LabelKey: "channels.fields.planXiaomiCodingPlanAmsterdam"},
	},
	ChannelTypeOllama: {
		{Value: "http://localhost:11434", LabelKey: "channels.fields.baseUrlDefault"},
		{Value: "ollama-coding-plan", LabelKey: "channels.fields.planOllamaCodingPlan"},
	},
}
