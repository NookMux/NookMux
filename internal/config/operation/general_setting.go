package operation

import (
	"errors"
	"fmt"
	"strings"
	"unicode"

	"github.com/NookMux/NookMux/internal/config/manager"
)

const (
	// DefaultTokenKeyPrefix 默认密钥前缀，同时作为历史令牌 key 一次性迁移补齐的前缀。
	DefaultTokenKeyPrefix = "sk-"
	// MaxTokenKeyPrefixLen 密钥前缀最大长度（字符数）。
	// 令牌随机部分固定 48 字符，Token.Key 列宽 varchar(64)，故前缀上限 16 字符。
	MaxTokenKeyPrefixLen = 16
)

type GeneralSetting struct {
	DocsLink            string `json:"docs_link"`
	TokenKeyPrefix      string `json:"token_key_prefix"` // 新建令牌密钥的前缀，默认 sk-
	PingIntervalEnabled bool   `json:"ping_interval_enabled"`
	PingIntervalSeconds int    `json:"ping_interval_seconds"`
}

// 默认配置
var generalSetting = GeneralSetting{
	DocsLink:            "https://github.com/NookMux/NookMux/tree/main/docs/zh",
	TokenKeyPrefix:      DefaultTokenKeyPrefix,
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

// ValidateTokenKeyPrefix 校验密钥前缀的合法性。
// 约束：非空、以 '-' 结尾（与随机部分分隔，同时保证完整密钥必含 '-'，
// 与历史无前缀 key 形成稳定区分）、长度不超过 MaxTokenKeyPrefixLen 字符、
// 不含空白、控制字符与 '%'（避免 LIKE 搜索通配语义歧义）。
func ValidateTokenKeyPrefix(prefix string) error {
	if prefix == "" {
		return errors.New("密钥前缀不能为空")
	}
	if !strings.HasSuffix(prefix, "-") {
		return errors.New("密钥前缀必须以 '-' 结尾")
	}
	if prefix == "-" {
		return errors.New("密钥前缀不能仅为 '-'")
	}
	runes := []rune(prefix)
	if len(runes) > MaxTokenKeyPrefixLen {
		return fmt.Errorf("密钥前缀长度不能超过 %d 个字符", MaxTokenKeyPrefixLen)
	}
	for _, r := range runes {
		if r == '%' {
			return errors.New("密钥前缀不能包含 '%'")
		}
		if r < 0x21 || r == 0x7f || unicode.IsSpace(r) || unicode.IsControl(r) {
			return errors.New("密钥前缀不能包含空白或控制字符")
		}
	}
	return nil
}
