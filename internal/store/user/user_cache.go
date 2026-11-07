package userstore

import (
	"fmt"
	"time"

	"github.com/NookMux/NookMux/internal/common"
	"github.com/NookMux/NookMux/internal/domain/shared"
	"github.com/NookMux/NookMux/internal/httpapi"
	"github.com/NookMux/NookMux/internal/infra/redis"
	"github.com/NookMux/NookMux/internal/infra/runtime"
	"github.com/NookMux/NookMux/internal/infra/security"
	"github.com/NookMux/NookMux/internal/store/db"
	"github.com/NookMux/NookMux/pkg/jsonx"
	"github.com/gin-gonic/gin"
)

// userCacheSchemaVersion 用户缓存哈希格式版本；格式演进时递增使旧哈希
// 校验不过并回源重建。
const userCacheSchemaVersion = 1

// UserBase struct remains the same as it represents the cached data structure
type UserBase struct {
	Id       int    `json:"id"`
	Group    string `json:"group"`
	Email    string `json:"email"`
	Quota    int    `json:"quota"`
	Status   int    `json:"status"`
	Role     int    `json:"role"`
	Username string `json:"username"`
	Setting  string `json:"setting"`
	// AuthVersion 凭证版本：改密/关 2FA 等凭证变更自增，纳入签名防回放。
	AuthVersion int `json:"-"`
	// CacheSchema/CacheExpiresAt/CacheMAC 完整性元数据，仅存于哈希。
	CacheSchema    int    `json:"-"`
	CacheExpiresAt int64  `json:"-"`
	CacheMAC       string `json:"-"`
}

// userCacheMAC 计算绑定缓存键名的完整性签名。Quota 由原子增量路径维护、
// 不参与签名；其余鉴权判定输入（角色/状态/分组/凭证版本等）与缓存模式、
// 时效界全部纳入（经包装结构体显式序列化，绕开 json:"-" 跳过）。
// Redis 中被篡改的条目因签名不符在读取时被拒绝并回源。
func (user UserBase) userCacheMAC() (string, error) {
	user.Quota = 0
	user.CacheMAC = ""
	return security.AuthCacheMAC(getUserCacheKey(user.Id), struct {
		User           UserBase
		AuthVersion    int
		CacheSchema    int
		CacheExpiresAt int64
	}{user, user.AuthVersion, user.CacheSchema, user.CacheExpiresAt})
}

func (user *UserBase) WriteContext(c *gin.Context) {
	httpapi.SetContextKey(c, common.ContextKeyUserGroup, user.Group)
	httpapi.SetContextKey(c, common.ContextKeyUserQuota, user.Quota)
	httpapi.SetContextKey(c, common.ContextKeyUserStatus, user.Status)
	httpapi.SetContextKey(c, common.ContextKeyUserEmail, user.Email)
	httpapi.SetContextKey(c, common.ContextKeyUserName, user.Username)
	httpapi.SetContextKey(c, common.ContextKeyUserSetting, user.GetSetting())
}

func (user *UserBase) GetSetting() shared.UserSetting {
	setting := shared.UserSetting{}
	if user.Setting != "" {
		err := jsonx.Unmarshal([]byte(user.Setting), &setting)
		if err != nil {
			common.SysLog("failed to unmarshal setting: " + err.Error())
		}
	}
	return setting
}

// getUserCacheKey returns the key for user cache
func getUserCacheKey(userId int) string {
	return fmt.Sprintf("user:%d", userId)
}

func userCacheTTLSeconds() int {
	ttl := redis.RedisKeyCacheSeconds()
	if ttl <= 0 {
		return 60
	}
	return ttl
}

// invalidateUserCache clears user cache
func invalidateUserCache(userId int) error {
	if !redis.RedisEnabled {
		return nil
	}
	return redis.RedisDelKey(getUserCacheKey(userId))
}

// InvalidateUserCache clears the Redis user cache for security-sensitive updates.
func InvalidateUserCache(userId int) error {
	return invalidateUserCache(userId)
}

// stampUserCacheMAC 为即将写入的快照打上缓存模式与时效界并计算签名。
func stampUserCacheMAC(base *UserBase) error {
	base.CacheSchema = userCacheSchemaVersion
	base.CacheExpiresAt = time.Now().Add(time.Duration(userCacheTTLSeconds()) * time.Second).UnixMilli()
	mac, err := base.userCacheMAC()
	if err != nil {
		return err
	}
	base.CacheMAC = mac
	return nil
}

// updateUserCache 刷新用户哈希的全部非配额字段并随写更新签名。配额
// （Quota）只经 CacheIncrUserQuota 原子增量路径维护，用户记录更新路径的
// 快照不得覆盖它；仅写已存在且带 TTL 的哈希，不复活已过期的键。
func updateUserCache(user User) error {
	if !redis.RedisEnabled {
		return nil
	}
	base := user.ToBaseUser()
	if err := stampUserCacheMAC(base); err != nil {
		return err
	}
	return redis.RedisHSetFields(getUserCacheKey(base.Id), map[string]interface{}{
		"Id":             fmt.Sprintf("%d", base.Id),
		"Group":          base.Group,
		"Email":          base.Email,
		"Status":         fmt.Sprintf("%d", base.Status),
		"Role":           fmt.Sprintf("%d", base.Role),
		"Username":       base.Username,
		"Setting":        base.Setting,
		"AuthVersion":    fmt.Sprintf("%d", base.AuthVersion),
		"CacheSchema":    fmt.Sprintf("%d", base.CacheSchema),
		"CacheExpiresAt": fmt.Sprintf("%d", base.CacheExpiresAt),
		"CacheMAC":       base.CacheMAC,
	})
}

// initOrRepairUserCache 冷初始化或修复用户哈希：哈希缺失时以 DB 快照整体
// 建哈希（含配额）；哈希存在但签名校验不过（被篡改、旧格式或字段损坏）
// 时，保留仍为整数的配额计数（原子增量可能领先于 DB 快照）并整哈希重写。
func initOrRepairUserCache(user User) error {
	if !redis.RedisEnabled {
		return nil
	}
	key := getUserCacheKey(user.Id)
	if !redis.RedisKeyExists(key) {
		base := user.ToBaseUser()
		if err := stampUserCacheMAC(base); err != nil {
			return err
		}
		return redis.RedisHSetObj(key, base, time.Duration(userCacheTTLSeconds())*time.Second)
	}

	var existing UserBase
	readErr := redis.RedisHGetObj(key, &existing)
	if readErr == nil &&
		security.ValidAuthCacheMAC(mustUserCacheMAC(&existing), existing.CacheMAC, existing.CacheExpiresAt) {
		return nil
	}

	base := user.ToBaseUser()
	if err := stampUserCacheMAC(base); err != nil {
		return err
	}
	metadata := map[string]string{
		"Id":             fmt.Sprintf("%d", base.Id),
		"Group":          base.Group,
		"Email":          base.Email,
		"Status":         fmt.Sprintf("%d", base.Status),
		"Role":           fmt.Sprintf("%d", base.Role),
		"Username":       base.Username,
		"Setting":        base.Setting,
		"AuthVersion":    fmt.Sprintf("%d", base.AuthVersion),
		"CacheSchema":    fmt.Sprintf("%d", base.CacheSchema),
		"CacheExpiresAt": fmt.Sprintf("%d", base.CacheExpiresAt),
		"CacheMAC":       base.CacheMAC,
	}
	// 原子修复：元数据整组重写、仍为整数的配额计数保留，与并发 HINCRBY
	// 无交错。
	return redis.RedisHSetMetadataPreserveCounters(key, metadata,
		map[string]string{"Quota": fmt.Sprintf("%d", base.Quota)}, userCacheTTLSeconds())
}

// mustUserCacheMAC 供已读取哈希的签名校验用；元数据序列化只含基础类型，
// 不应失败，失败按签名无效处理。
func mustUserCacheMAC(base *UserBase) string {
	mac, err := base.userCacheMAC()
	if err != nil {
		return ""
	}
	return mac
}

// GetUserCache gets complete user cache from hash
func GetUserCache(userId int) (userCache *UserBase, err error) {
	var user *User
	var fromDB bool
	defer func() {
		// 哈希缺失或签名校验不过时以 DB 快照冷初始化/修复（含配额，配额
		// 仅在修复时保留仍为整数的原值）；已存在且健康的哈希由原子增量
		// 路径维护，回写不得覆盖并发扣减后的值。
		if dbstore.ShouldUpdateRedis(fromDB, err) && user != nil {
			u := *user
			runtime.RelayGo(func() {
				if err := initOrRepairUserCache(u); err != nil {
					common.SysLog("failed to init user status cache: " + err.Error())
				}
			})
		}
	}()

	// Try getting from Redis first
	userCache, err = cacheGetUserBase(userId)
	if err == nil {
		return userCache, nil
	}

	// If Redis fails, get from DB
	fromDB = true
	user, err = GetUserById(userId, false)
	if err != nil {
		return nil, err // Return nil and error if DB lookup fails
	}

	// Create cache object from user data
	userCache = user.ToBaseUser()

	return userCache, nil
}

func cacheGetUserBase(userId int) (*UserBase, error) {
	if !redis.RedisEnabled {
		return nil, fmt.Errorf("redis is not enabled")
	}
	var userCache UserBase
	// Try getting from Redis first
	err := redis.RedisHGetObj(getUserCacheKey(userId), &userCache)
	if err != nil {
		return nil, err
	}
	if userCache.Id != userId || userCache.CacheSchema != userCacheSchemaVersion || userCache.AuthVersion <= 0 {
		return nil, fmt.Errorf("user cache schema is stale")
	}
	if !security.ValidAuthCacheMAC(mustUserCacheMAC(&userCache), userCache.CacheMAC, userCache.CacheExpiresAt) {
		return nil, fmt.Errorf("user cache integrity check failed")
	}
	return &userCache, nil
}

// Add atomic quota operations using hash fields
func CacheIncrUserQuota(userId int, delta int64) error {
	if !redis.RedisEnabled {
		return nil
	}
	return redis.RedisHIncrBy(getUserCacheKey(userId), "Quota", delta)
}

func cacheDecrUserQuota(userId int, delta int64) error {
	return CacheIncrUserQuota(userId, -delta)
}

// Helper functions to get individual fields if needed
func getUserGroupCache(userId int) (string, error) {
	cache, err := GetUserCache(userId)
	if err != nil {
		return "", err
	}
	return cache.Group, nil
}

func getUserQuotaCache(userId int) (int, error) {
	cache, err := GetUserCache(userId)
	if err != nil {
		return 0, err
	}
	return cache.Quota, nil
}

func getUserNameCache(userId int) (string, error) {
	cache, err := GetUserCache(userId)
	if err != nil {
		return "", err
	}
	return cache.Username, nil
}

func getUserSettingCache(userId int) (shared.UserSetting, error) {
	cache, err := GetUserCache(userId)
	if err != nil {
		return shared.UserSetting{}, err
	}
	return cache.GetSetting(), nil
}

// New functions for individual field updates
func updateUserQuotaCache(userId int, quota int) error {
	if !redis.RedisEnabled {
		return nil
	}
	// Quota 不参与完整性签名，原子增量路径之外的单点回写保持不变。
	return redis.RedisHSetField(getUserCacheKey(userId), "Quota", fmt.Sprintf("%d", quota))
}
