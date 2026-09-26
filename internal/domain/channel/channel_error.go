package channel

type ChannelError struct {
	ChannelId   int    `json:"channel_id"`
	ChannelType int    `json:"channel_type"`
	ChannelName string `json:"channel_name"`
	IsMultiKey  bool   `json:"is_multi_key"`
	AutoBan     bool   `json:"auto_ban"`
	UsingKey    string `json:"using_key"`
	// Lang 是构造时的请求语言，供后台 goroutine 中的通知文案翻译使用；
	// 为空时按默认语言处理。
	Lang string `json:"lang,omitempty"`
}

func NewChannelError(channelId int, channelType int, channelName string, isMultiKey bool, usingKey string, autoBan bool) *ChannelError {
	return &ChannelError{
		ChannelId:   channelId,
		ChannelType: channelType,
		ChannelName: channelName,
		IsMultiKey:  isMultiKey,
		AutoBan:     autoBan,
		UsingKey:    usingKey,
	}
}
