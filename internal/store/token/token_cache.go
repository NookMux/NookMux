package tokenstore

import (
	"fmt"
	"strconv"
	"time"

	"github.com/NookMux/NookMux/internal/domain/shared"
	"github.com/NookMux/NookMux/internal/infra/redis"
	"github.com/NookMux/NookMux/internal/infra/security"
)

// tokenCacheCounterFields 是运行时计数字段：由原子增量（HINCRBY）或快照
// 回写维护，不纳入完整性签名；修复哈希时其中仍为整数的计数被原样保留，
// 避免快照回滚并发预扣。
var tokenCacheCounterFields = []string{
	"RemainQuota",
	"UsedQuota",
	"AccessedTime",
	"WindowUsedQuota",
	"WindowStartTime",
	"CycleUsedQuota",
	"CycleStartTime",
}

func tokenCacheKey(key string) string {
	return fmt.Sprintf("token:%s", security.GenerateHMAC(key))
}

func tokenCacheTTLSeconds() int {
	ttl := redis.RedisKeyCacheSeconds()
	if ttl <= 0 {
		return 60
	}
	return ttl
}

// tokenCacheMAC 计算绑定缓存键名的完整性签名。密钥与全部运行时计数不参
// 与签名（不是鉴权判定输入），其余字段（状态、限额配置、分组等）与时效
// 界全部纳入；空字符串指针与 nil 在哈希往返中等价，签名前统一归一化。
func (token Token) tokenCacheMAC(cacheKey string) (string, error) {
	token.Key = ""
	token.RemainQuota = 0
	token.UsedQuota = 0
	token.AccessedTime = 0
	token.WindowUsedQuota = 0
	token.WindowStartTime = 0
	token.CycleUsedQuota = 0
	token.CycleStartTime = 0
	if token.ModelMapping != nil && *token.ModelMapping == "" {
		token.ModelMapping = nil
	}
	if token.AllowIps != nil && *token.AllowIps == "" {
		token.AllowIps = nil
	}
	token.CacheMAC = ""
	return security.AuthCacheMAC(cacheKey, struct {
		Token          Token
		CacheExpiresAt int64
	}{token, token.CacheExpiresAt})
}

// stampTokenCacheMAC 为即将写入的快照打上时效界并计算签名；cacheKey 由
// 调用方以完整令牌密钥计算并显式传入（Clean 置空 Key 后无法再自算），
// 保证签名与读取路径始终绑定同一键名。
func stampTokenCacheMAC(token *Token, cacheKey string) error {
	token.CacheExpiresAt = time.Now().Add(time.Duration(tokenCacheTTLSeconds()) * time.Second).UnixMilli()
	mac, err := token.tokenCacheMAC(cacheKey)
	if err != nil {
		return err
	}
	token.CacheMAC = mac
	return nil
}

func cacheSetToken(token Token) error {
	key := tokenCacheKey(token.Key)
	token.Clean()
	if err := stampTokenCacheMAC(&token, key); err != nil {
		return err
	}
	return redis.RedisHSetObj(key, &token, time.Duration(tokenCacheTTLSeconds())*time.Second)
}

// initOrRepairTokenCache 冷初始化或修复令牌哈希：哈希缺失时以 DB 快照整
// 体建哈希；哈希存在但签名校验不过（被篡改、旧格式或字段损坏）时，保留
// 仍为整数的运行时计数并整组重写其余元数据与签名。
func initOrRepairTokenCache(token Token) error {
	if !redis.RedisEnabled {
		return nil
	}
	cacheKey := tokenCacheKey(token.Key)
	if !redis.RedisKeyExists(cacheKey) {
		return cacheSetToken(token)
	}

	var existing Token
	readErr := redis.RedisHGetObj(cacheKey, &existing)
	if readErr == nil {
		if mac, err := existing.tokenCacheMAC(cacheKey); err == nil &&
			security.ValidAuthCacheMAC(mac, existing.CacheMAC, existing.CacheExpiresAt) {
			return nil
		}
	}

	if err := stampTokenCacheMAC(&token, cacheKey); err != nil {
		return err
	}
	counters := make(map[string]string, len(tokenCacheCounterFields))
	for _, counter := range tokenCacheCounterFields {
		counters[counter] = tokenCounterSnapshotValue(&token, counter)
	}
	// 原子修复：元数据整组重写、仍为整数的计数保留，与并发 HINCRBY 无交错。
	return redis.RedisHSetMetadataPreserveCounters(cacheKey, tokenMetadataFields(&token), counters, tokenCacheTTLSeconds())
}

// tokenMetadataFields 构建除运行时计数外的全部哈希字段（字符串形态）。
func tokenMetadataFields(token *Token) map[string]string {
	modelMapping := ""
	if token.ModelMapping != nil {
		modelMapping = *token.ModelMapping
	}
	allowIps := ""
	if token.AllowIps != nil {
		allowIps = *token.AllowIps
	}
	return map[string]string{
		"Id":                 fmt.Sprintf("%d", token.Id),
		"UserId":             fmt.Sprintf("%d", token.UserId),
		"Status":             fmt.Sprintf("%d", token.Status),
		"Name":               token.Name,
		"CreatedTime":        fmt.Sprintf("%d", token.CreatedTime),
		"ExpiredTime":        fmt.Sprintf("%d", token.ExpiredTime),
		"UnlimitedQuota":     strconv.FormatBool(token.UnlimitedQuota),
		"ModelLimitsEnabled": strconv.FormatBool(token.ModelLimitsEnabled),
		"ModelLimits":        token.ModelLimits,
		"ModelMapping":       modelMapping,
		"AllowIps":           allowIps,
		"Group":              token.Group,
		"CrossGroupRetry":    strconv.FormatBool(token.CrossGroupRetry),
		"QuotaType":          fmt.Sprintf("%d", token.QuotaType),
		"WindowHours":        fmt.Sprintf("%d", token.WindowHours),
		"WindowQuota":        fmt.Sprintf("%d", token.WindowQuota),
		"WindowStartHour":    fmt.Sprintf("%d", token.WindowStartHour),
		"CycleDays":          fmt.Sprintf("%d", token.CycleDays),
		"CycleQuota":         fmt.Sprintf("%d", token.CycleQuota),
		"CacheExpiresAt":     fmt.Sprintf("%d", token.CacheExpiresAt),
		"CacheMAC":           token.CacheMAC,
	}
}

func tokenCounterSnapshotValue(token *Token, field string) string {
	switch field {
	case "RemainQuota":
		return fmt.Sprintf("%d", token.RemainQuota)
	case "UsedQuota":
		return fmt.Sprintf("%d", token.UsedQuota)
	case "AccessedTime":
		return fmt.Sprintf("%d", token.AccessedTime)
	case "WindowUsedQuota":
		return fmt.Sprintf("%d", token.WindowUsedQuota)
	case "WindowStartTime":
		return fmt.Sprintf("%d", token.WindowStartTime)
	case "CycleUsedQuota":
		return fmt.Sprintf("%d", token.CycleUsedQuota)
	case "CycleStartTime":
		return fmt.Sprintf("%d", token.CycleStartTime)
	default:
		return "0"
	}
}

func cacheDeleteToken(key string) error {
	key = security.GenerateHMAC(key)
	err := redis.RedisDelKey(fmt.Sprintf("token:%s", key))
	if err != nil {
		return err
	}
	return nil
}

func cacheIncrTokenQuota(key string, increment int64) error {
	key = security.GenerateHMAC(key)
	err := redis.RedisHIncrBy(fmt.Sprintf("token:%s", key), shared.TokenFiledRemainQuota, increment)
	if err != nil {
		return err
	}
	return nil
}

func cacheDecrTokenQuota(key string, decrement int64) error {
	return cacheIncrTokenQuota(key, -decrement)
}

func cacheIncrTokenUsedQuota(key string, increment int64) error {
	key = security.GenerateHMAC(key)
	err := redis.RedisHIncrBy(fmt.Sprintf("token:%s", key), shared.TokenFieldUsedQuota, increment)
	if err != nil {
		return err
	}
	return nil
}

func cacheIncrWindowUsedQuota(key string, increment int64) error {
	key = security.GenerateHMAC(key)
	err := redis.RedisHIncrBy(fmt.Sprintf("token:%s", key), shared.TokenFieldWindowUsedQuota, increment)
	if err != nil {
		return err
	}
	return nil
}

func cacheIncrCycleUsedQuota(key string, increment int64) error {
	key = security.GenerateHMAC(key)
	err := redis.RedisHIncrBy(fmt.Sprintf("token:%s", key), shared.TokenFieldCycleUsedQuota, increment)
	if err != nil {
		return err
	}
	return nil
}

// CacheGetTokenByKey 从缓存中获取 token，如果缓存中不存在，则从数据库中获取
func cacheGetTokenByKey(key string) (*Token, error) {
	hmacKey := security.GenerateHMAC(key)
	if !redis.RedisEnabled {
		return nil, fmt.Errorf("redis is not enabled")
	}
	var token Token
	err := redis.RedisHGetObj(fmt.Sprintf("token:%s", hmacKey), &token)
	if err != nil {
		return nil, err
	}
	token.Key = key
	mac, err := token.tokenCacheMAC(fmt.Sprintf("token:%s", hmacKey))
	if err != nil || !security.ValidAuthCacheMAC(mac, token.CacheMAC, token.CacheExpiresAt) {
		return nil, fmt.Errorf("token cache integrity check failed")
	}
	return &token, nil
}
