package operation

import "github.com/NookMux/NookMux/internal/config/manager"

type GeneralSetting struct {
	DocsLink            string `json:"docs_link"`
	PingIntervalEnabled bool   `json:"ping_interval_enabled"`
	PingIntervalSeconds int    `json:"ping_interval_seconds"`
}

// 默认配置
var generalSetting = GeneralSetting{
	DocsLink:            "https://github.com/NookMux/NookMux/tree/main/docs/zh",
	PingIntervalEnabled: false,
	PingIntervalSeconds: 60,
}

func init() {
	// 注册到全局配置管理器
	manager.GlobalConfig.Register("general_setting", &generalSetting)
}

func GetGeneralSetting() *GeneralSetting {
	return &generalSetting
}
