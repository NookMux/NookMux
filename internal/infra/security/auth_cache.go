package security

import (
	"crypto/hmac"
	"time"

	"github.com/NookMux/NookMux/internal/common"
	"github.com/NookMux/NookMux/pkg/jsonx"
)

// authCacheMACLabel 将签名用途与密钥名空间绑定到「鉴权缓存」，
// 防止同一密钥签发的其他 MAC 被跨用途重放。
const authCacheMACLabel = "auth-cache-v1"

// AuthCacheMAC 以服务端密钥（CRYPTO_SECRET）对缓存元数据计算绑定 Redis
// 键名的 HMAC-SHA256。只有数据库快照或服务端自身构造的状态才能产出有效
// 签名；Redis 中被伪造或篡改的鉴权缓存条目因签名校验不过而在读取时被拒
// 绝并回源数据库。
func AuthCacheMAC(key string, metadata any) (string, error) {
	payload, err := jsonx.Marshal(metadata)
	if err != nil {
		return "", err
	}
	return GenerateHMACWithKey([]byte(authCacheMACLabel+":"+common.CryptoSecret), key+"\x00"+string(payload)), nil
}

// ValidAuthCacheMAC 常量时间比较签名，并要求签名时嵌入的缓存过期时间
// 尚未到点（哈希自身的 Redis TTL 之外再加一道内层时效界）。
func ValidAuthCacheMAC(expected, actual string, expiresAtMs int64) bool {
	return expiresAtMs > time.Now().UnixMilli() && hmac.Equal([]byte(expected), []byte(actual))
}
