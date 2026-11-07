package redis

import (
	"context"
	"errors"
	"fmt"
	"os"
	"reflect"
	"strconv"
	"time"

	"github.com/NookMux/NookMux/internal/common"
	"github.com/go-redis/redis/v8"
	"gorm.io/gorm"
)

var RDB *redis.Client
var RedisEnabled = true

func RedisKeyCacheSeconds() int {
	return common.SyncFrequency
}

// InitRedisClient This function is called after init()
func InitRedisClient() (err error) {
	if os.Getenv("REDIS_CONN_STRING") == "" {
		RedisEnabled = false
		common.SysLog("REDIS_CONN_STRING not set, Redis is not enabled")
		return nil
	}
	if os.Getenv("SYNC_FREQUENCY") == "" {
		common.SysLog("SYNC_FREQUENCY not set, use default value 60")
		common.SyncFrequency = 60
	}
	common.SysLog("Redis is enabled")
	opt, err := redis.ParseURL(os.Getenv("REDIS_CONN_STRING"))
	if err != nil {
		common.FatalLog("failed to parse Redis connection string: " + err.Error())
	}
	opt.PoolSize = common.GetEnvOrDefault("REDIS_POOL_SIZE", 50)
	// Keep a few idle connections warm so the first requests don't pay the
	// TLS/TCP handshake cost. The previous configuration had no MinIdleConns,
	// so under burst traffic every new connection was established on-demand.
	opt.MinIdleConns = common.GetEnvOrDefault("REDIS_MIN_IDLE_CONNS", 5)
	// Bound dial/read/write timeouts so a stalled Redis does not pin goroutines
	// indefinitely. Without these the defaults (no write timeout) can cause
	// goroutine and connection leaks under network partitions.
	opt.DialTimeout = time.Duration(common.GetEnvOrDefault("REDIS_DIAL_TIMEOUT", 5)) * time.Second
	opt.ReadTimeout = time.Duration(common.GetEnvOrDefault("REDIS_READ_TIMEOUT", 3)) * time.Second
	opt.WriteTimeout = time.Duration(common.GetEnvOrDefault("REDIS_WRITE_TIMEOUT", 3)) * time.Second
	// Retry on transient errors instead of failing the request immediately.
	opt.MaxRetries = common.GetEnvOrDefault("REDIS_MAX_RETRIES", 3)
	RDB = redis.NewClient(opt)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err = RDB.Ping(ctx).Result()
	if err != nil {
		common.FatalLog("Redis ping test failed: " + err.Error())
	}
	if common.DebugEnabled {
		common.SysLog(fmt.Sprintf("Redis connected to %s", opt.Addr))
		common.SysLog(fmt.Sprintf("Redis database: %d", opt.DB))
	}
	return err
}

func ParseRedisOption() *redis.Options {
	opt, err := redis.ParseURL(os.Getenv("REDIS_CONN_STRING"))
	if err != nil {
		common.FatalLog("failed to parse Redis connection string: " + err.Error())
	}
	return opt
}

func RedisSet(key string, value string, expiration time.Duration) error {
	if common.DebugEnabled {
		common.SysLog(fmt.Sprintf("Redis SET: key=%s, value=%s, expiration=%v", key, value, expiration))
	}
	ctx := context.Background()
	return RDB.Set(ctx, key, value, expiration).Err()
}

func RedisGet(key string) (string, error) {
	if common.DebugEnabled {
		common.SysLog(fmt.Sprintf("Redis GET: key=%s", key))
	}
	ctx := context.Background()
	val, err := RDB.Get(ctx, key).Result()
	return val, err
}

//func RedisExpire(key string, expiration time.Duration) error {
//	ctx := context.Background()
//	return RDB.Expire(ctx, key, expiration).Err()
//}
//
//func RedisGetEx(key string, expiration time.Duration) (string, error) {
//	ctx := context.Background()
//	return RDB.GetSet(ctx, key, expiration).Result()
//}

func RedisDel(key string) error {
	if common.DebugEnabled {
		common.SysLog(fmt.Sprintf("Redis DEL: key=%s", key))
	}
	ctx := context.Background()
	return RDB.Del(ctx, key).Err()
}

func RedisDelKey(key string) error {
	if common.DebugEnabled {
		common.SysLog(fmt.Sprintf("Redis DEL Key: key=%s", key))
	}
	ctx := context.Background()
	return RDB.Del(ctx, key).Err()
}

// RedisKeyExists reports whether the key currently exists in Redis.
func RedisKeyExists(key string) bool {
	ctx := context.Background()
	n, err := RDB.Exists(ctx, key).Result()
	if err != nil {
		return false
	}
	return n > 0
}

// RedisHGetField 读取哈希单字段的原始字符串值；字段或键不存在时返回错误。
func RedisHGetField(key, field string) (string, error) {
	value, err := RDB.HGet(context.Background(), key, field).Result()
	if err != nil {
		return "", fmt.Errorf("failed to get hash field %s of %s: %w", field, key, err)
	}
	return value, nil
}

// repairHashScript 以单命令原子完成"整组重写元数据 + 保留仍为整数的运行
// 时计数 + 刷新 TTL"：元数据字段（含新签名与时效界）直接 HSET；计数字段
// 仅当现有值缺失或非整数时以快照值替换，与并发 HINCRBY 无交错窗口。
const repairHashScript = `
local ttl = tonumber(ARGV[1])
local metaCount = tonumber(ARGV[2])
local args = {}
for n = 1, metaCount * 2 do
  table.insert(args, ARGV[2 + n])
end
redis.call('HSET', KEYS[1], unpack(args))
local i = 3 + metaCount * 2
while i < #ARGV do
  local name = ARGV[i]
  local snap = ARGV[i + 1]
  local cur = redis.call('HGET', KEYS[1], name) or ''
  if not string.match(cur, '^-?%d+$') then
    redis.call('HSET', KEYS[1], name, snap)
  end
  i = i + 2
end
redis.call('EXPIRE', KEYS[1], ttl)`

// RedisHSetMetadataPreserveCounters 原子修复哈希：整组写入元数据字段，
// 保留仍为整数的运行时计数字段（非整数/缺失以快照值替换），刷新 TTL。
// 用于完整性签名校验不过时的回源修复，避免覆盖并发原子增量。
func RedisHSetMetadataPreserveCounters(key string, metadata map[string]string, counters map[string]string, ttlSeconds int) error {
	argv := make([]interface{}, 0, 3+2*len(metadata)+2*len(counters))
	argv = append(argv, ttlSeconds, len(metadata))
	for field, value := range metadata {
		argv = append(argv, field, value)
	}
	for field, value := range counters {
		argv = append(argv, field, value)
	}
	if err := RDB.Eval(context.Background(), repairHashScript, []string{key}, argv...).Err(); err != nil {
		return fmt.Errorf("failed to atomically repair hash %s: %w", key, err)
	}
	return nil
}

func RedisHSetObj(key string, obj interface{}, expiration time.Duration) error {
	if common.DebugEnabled {
		common.SysLog(fmt.Sprintf("Redis HSET: key=%s, obj=%+v, expiration=%v", key, obj, expiration))
	}
	ctx := context.Background()

	data := make(map[string]interface{})

	// 使用反射遍历结构体字段
	v := reflect.ValueOf(obj).Elem()
	t := v.Type()
	for i := 0; i < v.NumField(); i++ {
		field := t.Field(i)
		value := v.Field(i)

		// Skip DeletedAt field
		if field.Type.String() == "gorm.DeletedAt" {
			continue
		}

		// 处理指针类型
		if value.Kind() == reflect.Ptr {
			if value.IsNil() {
				data[field.Name] = ""
				continue
			}
			value = value.Elem()
		}

		// 处理布尔类型
		if value.Kind() == reflect.Bool {
			data[field.Name] = strconv.FormatBool(value.Bool())
			continue
		}

		// 其他类型直接转换为字符串
		data[field.Name] = fmt.Sprintf("%v", value.Interface())
	}

	txn := RDB.TxPipeline()
	txn.HSet(ctx, key, data)

	// 只有在 expiration 大于 0 时才设置过期时间
	if expiration > 0 {
		txn.Expire(ctx, key, expiration)
	}

	_, err := txn.Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed to execute transaction: %w", err)
	}
	return nil
}

func RedisHGetObj(key string, obj interface{}) error {
	if common.DebugEnabled {
		common.SysLog(fmt.Sprintf("Redis HGETALL: key=%s", key))
	}
	ctx := context.Background()

	result, err := RDB.HGetAll(ctx, key).Result()
	if err != nil {
		return fmt.Errorf("failed to load hash from Redis: %w", err)
	}

	if len(result) == 0 {
		return fmt.Errorf("key %s not found in Redis", key)
	}

	// Handle both pointer and non-pointer values
	val := reflect.ValueOf(obj)
	if val.Kind() != reflect.Ptr {
		return fmt.Errorf("obj must be a pointer to a struct, got %T", obj)
	}

	v := val.Elem()
	if v.Kind() != reflect.Struct {
		return fmt.Errorf("obj must be a pointer to a struct, got pointer to %T", v.Interface())
	}

	t := v.Type()
	for i := 0; i < v.NumField(); i++ {
		field := t.Field(i)
		fieldName := field.Name
		if value, ok := result[fieldName]; ok {
			fieldValue := v.Field(i)

			// Handle pointer types
			if fieldValue.Kind() == reflect.Ptr {
				if value == "" {
					continue
				}
				if fieldValue.IsNil() {
					fieldValue.Set(reflect.New(fieldValue.Type().Elem()))
				}
				fieldValue = fieldValue.Elem()
			}

			// Enhanced type handling for Token struct
			switch fieldValue.Kind() {
			case reflect.String:
				fieldValue.SetString(value)
			case reflect.Int, reflect.Int64:
				intValue, err := strconv.ParseInt(value, 10, 64)
				if err != nil {
					return fmt.Errorf("failed to parse int field %s: %w", fieldName, err)
				}
				fieldValue.SetInt(intValue)
			case reflect.Bool:
				boolValue, err := strconv.ParseBool(value)
				if err != nil {
					return fmt.Errorf("failed to parse bool field %s: %w", fieldName, err)
				}
				fieldValue.SetBool(boolValue)
			case reflect.Struct:
				// Special handling for gorm.DeletedAt
				if fieldValue.Type().String() == "gorm.DeletedAt" {
					if value != "" {
						timeValue, err := time.Parse(time.RFC3339, value)
						if err != nil {
							return fmt.Errorf("failed to parse DeletedAt field %s: %w", fieldName, err)
						}
						fieldValue.Set(reflect.ValueOf(gorm.DeletedAt{Time: timeValue, Valid: true}))
					}
				}
			default:
				return fmt.Errorf("unsupported field type: %s for field %s", fieldValue.Kind(), fieldName)
			}
		}
	}

	return nil
}

// RedisIncr Add this function to handle atomic increments
func RedisIncr(key string, delta int64) error {
	if common.DebugEnabled {
		common.SysLog(fmt.Sprintf("Redis INCR: key=%s, delta=%d", key, delta))
	}
	// 检查键的剩余生存时间
	ttlCmd := RDB.TTL(context.Background(), key)
	ttl, err := ttlCmd.Result()
	if err != nil && !errors.Is(err, redis.Nil) {
		return fmt.Errorf("failed to get TTL: %w", err)
	}

	// 只有在 key 存在且有 TTL 时才需要特殊处理
	if ttl > 0 {
		ctx := context.Background()
		// 开始一个Redis事务
		txn := RDB.TxPipeline()

		// 减少余额
		decrCmd := txn.IncrBy(ctx, key, delta)
		if err := decrCmd.Err(); err != nil {
			return err // 如果减少失败，则直接返回错误
		}

		// 重新设置过期时间，使用原来的过期时间
		txn.Expire(ctx, key, ttl)

		// 执行事务
		_, err = txn.Exec(ctx)
		return err
	}
	return nil
}

func RedisHIncrBy(key, field string, delta int64) error {
	if common.DebugEnabled {
		common.SysLog(fmt.Sprintf("Redis HINCRBY: key=%s, field=%s, delta=%d", key, field, delta))
	}
	ttlCmd := RDB.TTL(context.Background(), key)
	ttl, err := ttlCmd.Result()
	if err != nil && !errors.Is(err, redis.Nil) {
		return fmt.Errorf("failed to get TTL: %w", err)
	}

	if ttl > 0 {
		ctx := context.Background()
		txn := RDB.TxPipeline()

		incrCmd := txn.HIncrBy(ctx, key, field, delta)
		if err := incrCmd.Err(); err != nil {
			return err
		}

		txn.Expire(ctx, key, ttl)

		_, err = txn.Exec(ctx)
		return err
	}
	return nil
}

func RedisHSetField(key, field string, value interface{}) error {
	if common.DebugEnabled {
		common.SysLog(fmt.Sprintf("Redis HSET field: key=%s, field=%s, value=%v", key, field, value))
	}
	ttlCmd := RDB.TTL(context.Background(), key)
	ttl, err := ttlCmd.Result()
	if err != nil && !errors.Is(err, redis.Nil) {
		return fmt.Errorf("failed to get TTL: %w", err)
	}

	if ttl > 0 {
		ctx := context.Background()
		txn := RDB.TxPipeline()

		hsetCmd := txn.HSet(ctx, key, field, value)
		if err := hsetCmd.Err(); err != nil {
			return err
		}

		txn.Expire(ctx, key, ttl)

		_, err = txn.Exec(ctx)
		return err
	}
	return nil
}

// RedisHSetFields updates multiple hash fields in one pipeline. Like
// RedisHSetField it only touches hashes that already exist with a TTL, so a
// field refresh never re-creates an expired or evicted hash from a stale
// snapshot.
func RedisHSetFields(key string, fields map[string]interface{}) error {
	if common.DebugEnabled {
		common.SysLog(fmt.Sprintf("Redis HSET fields: key=%s, fields=%v", key, fields))
	}
	if len(fields) == 0 {
		return nil
	}
	ttlCmd := RDB.TTL(context.Background(), key)
	ttl, err := ttlCmd.Result()
	if err != nil && !errors.Is(err, redis.Nil) {
		return fmt.Errorf("failed to get TTL: %w", err)
	}

	if ttl > 0 {
		ctx := context.Background()
		txn := RDB.TxPipeline()

		hsetCmd := txn.HSet(ctx, key, fields)
		if err := hsetCmd.Err(); err != nil {
			return err
		}

		txn.Expire(ctx, key, ttl)

		_, err = txn.Exec(ctx)
		return err
	}
	return nil
}
