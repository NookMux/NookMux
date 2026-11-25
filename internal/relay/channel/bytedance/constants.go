package bytedance

// ModelList 为 BytePlus ModelArk 按 Model ID 直接调用、内置价格表已配价的字节跳动模型
// （https://ai.byteplus.com/ark/region:ap-southeast-1/docs/model-list），对应国际站端点
// https://ark.ap-southeast.bytepluses.com/api/v3。按账号创建的推理端点（如 ep-xxxxx）、
// 国内站 Model ID 与 ModelArk 上的其他厂商模型，由管理员在渠道上手动填写。
var ModelList = []string{
	"dola-seed-2-1-turbo-260628", "seed-2-0-code-preview-260328",
	"seed-2-0-lite-260428", "seed-2-0-mini-260428", "seed-2-0-pro-260328",
}

var ChannelName = "bytedance"
