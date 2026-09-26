package tokenstore

import (
	"errors"
	"fmt"
	"github.com/NookMux/NookMux/internal/common"
	"github.com/NookMux/NookMux/internal/infra/redis"
	"github.com/NookMux/NookMux/internal/infra/runtime"
	"github.com/NookMux/NookMux/internal/store/db"
	"github.com/NookMux/NookMux/pkg/jsonx"
	"golang.org/x/sync/singleflight"
	"gorm.io/gorm"
	"strings"
	"sync"
)

// maxUserTokens 每用户最大令牌数量（硬编码）
const maxUserTokens = 1000

// tokenLoadGroup 合并并发的冷缓存 token 查询，避免启动瞬间把相同 key 打到数据库上。
var tokenLoadGroup singleflight.Group

// tokenResetLocks 令牌密钥重置互斥锁，按 token ID 串行化并发重置请求，
// 防止同一令牌同时出现多个有效 key。
var tokenResetLocks sync.Map

func getTokenResetLock(id int) *sync.Mutex {
	v, _ := tokenResetLocks.LoadOrStore(id, &sync.Mutex{})
	return v.(*sync.Mutex)
}

type Token struct {
	Id                 int    `json:"id"`
	UserId             int    `json:"user_id" gorm:"index"`
	Key                string `json:"key" gorm:"type:char(48);uniqueIndex"`
	Status             int    `json:"status" gorm:"default:1"`
	Name               string `json:"name" gorm:"index" `
	CreatedTime        int64  `json:"created_time" gorm:"bigint"`
	AccessedTime       int64  `json:"accessed_time" gorm:"bigint"`
	ExpiredTime        int64  `json:"expired_time" gorm:"bigint;default:-1"` // -1 means never expired
	RemainQuota        int    `json:"remain_quota" gorm:"default:0"`
	UnlimitedQuota     bool   `json:"unlimited_quota"`
	ModelLimitsEnabled bool   `json:"model_limits_enabled"`
	// MySQL 禁止 TEXT 列携带字面量 DEFAULT（Error 1101），且写入路径始终显式
	// 携带该列值，数据库层默认值无意义，因此 tag 不声明 default。
	ModelLimits     string  `json:"model_limits" gorm:"type:text"`
	ModelMapping    *string `json:"model_mapping" gorm:"type:text"` // 令牌级模型重定向规则 (JSON 字符串)
	AllowIps        *string `json:"allow_ips" gorm:"default:''"`
	UsedQuota       int     `json:"used_quota" gorm:"default:0"` // used quota
	Group           string  `json:"group" gorm:"default:''"`
	CrossGroupRetry bool    `json:"cross_group_retry"` // 跨分组重试，仅auto分组有效

	// 限额类型：0=无限额度, 1=永久限额, 2=时段限额, 3=时段+周期限额
	QuotaType int `json:"quota_type" gorm:"default:0"`

	// 时段限额相关字段（quota_type=2,3 时生效）
	WindowHours     int `json:"window_hours" gorm:"default:0"`      // 窗口时长（小时）
	WindowQuota     int `json:"window_quota" gorm:"default:0"`      // 每个窗口的额度
	WindowStartHour int `json:"window_start_hour" gorm:"default:0"` // 窗口起始小时（0-23）

	// 周期限额相关字段（quota_type=3 时生效）
	CycleDays  int `json:"cycle_days" gorm:"default:0"`  // 周期天数
	CycleQuota int `json:"cycle_quota" gorm:"default:0"` // 周期总额度

	// 运行时状态字段（自动计算，不由用户设置）
	WindowUsedQuota int   `json:"window_used_quota" gorm:"default:0"` // 当前窗口已用额度
	WindowStartTime int64 `json:"window_start_time" gorm:"default:0"` // 当前窗口开始时间（unix timestamp）
	CycleUsedQuota  int   `json:"cycle_used_quota" gorm:"default:0"`  // 当前周期已用额度
	CycleStartTime  int64 `json:"cycle_start_time" gorm:"default:0"`  // 当前周期开始时间（unix timestamp）

	DeletedAt gorm.DeletedAt `gorm:"index"`
}

func (token *Token) Clean() {
	token.Key = ""
}

// MaskTokenKey 将令牌密钥脱敏为等长星号，供列表/详情等接口返回，避免泄露真实 key。
// 数据库中的 key 不含 sk- 前缀；前端展示时再拼接 sk-。
func MaskTokenKey(key string) string {
	if key == "" {
		return ""
	}
	return strings.Repeat("*", len(key))
}

// GetFullKey 返回数据库中存储的完整密钥（不含 sk- 前缀）。
func (token *Token) GetFullKey() string {
	return token.Key
}

// GetMaskedKey 返回脱敏后的密钥字段值。
func (token *Token) GetMaskedKey() string {
	return MaskTokenKey(token.Key)
}

func (token *Token) GetIpLimits() []string {
	// delete empty spaces
	//split with \n
	ipLimits := make([]string, 0)
	if token.AllowIps == nil {
		return ipLimits
	}
	cleanIps := strings.ReplaceAll(*token.AllowIps, " ", "")
	if cleanIps == "" {
		return ipLimits
	}
	ips := strings.Split(cleanIps, "\n")
	for _, ip := range ips {
		ip = strings.TrimSpace(ip)
		ip = strings.ReplaceAll(ip, ",", "")
		if ip != "" {
			ipLimits = append(ipLimits, ip)
		}
	}
	return ipLimits
}

func GetAllUserTokens(userId int, startIdx int, num int) ([]*Token, error) {
	var tokens []*Token
	err := dbstore.DB.Where("user_id = ?", userId).Order("id desc").Limit(num).Offset(startIdx).Find(&tokens).Error
	return tokens, err
}

// SanitizeLikePattern 校验并清洗用户输入的 LIKE 搜索模式。
// 规则：
//  1. 转义 ! 和 _（使用 ! 作为 ESCAPE 字符，兼容 MySQL/PostgreSQL/SQLite）
//  2. 连续的 % 合并为单个 %
//  3. 最多允许 2 个 %
//  4. 含 % 时（模糊搜索），去掉 % 后关键词长度必须 >= 2
//  5. 不含 % 时按精确匹配
func SanitizeLikePattern(input string) (string, error) {
	// 1. 先转义 ESCAPE 字符 ! 自身，再转义 _
	//    使用 ! 而非 \ 作为 ESCAPE 字符，避免 MySQL 中反斜杠的字符串转义问题
	input = strings.ReplaceAll(input, "!", "!!")
	input = strings.ReplaceAll(input, `_`, `!_`)

	// 2. 连续的 % 直接拒绝
	if strings.Contains(input, "%%") {
		return "", errors.New("搜索模式中不允许包含连续的 % 通配符")
	}

	// 3. 统计 % 数量，不得超过 2
	count := strings.Count(input, "%")
	if count > 2 {
		return "", errors.New("搜索模式中最多允许包含 2 个 % 通配符")
	}

	// 4. 含 % 时，去掉 % 后关键词长度必须 >= 2
	if count > 0 {
		stripped := strings.ReplaceAll(input, "%", "")
		if len(stripped) < 2 {
			return "", errors.New("使用模糊搜索时，关键词长度至少为 2 个字符")
		}
		return input, nil
	}

	// 5. 无 % 时，精确全匹配
	return input, nil
}

const searchHardLimit = 100

func SearchUserTokens(userId int, keyword string, token string, group string, status int, all bool, offset int, limit int) (tokens []*Token, total int64, err error) {
	// model 层强制截断
	if limit <= 0 || limit > searchHardLimit {
		limit = searchHardLimit
	}
	if offset < 0 {
		offset = 0
	}

	if token != "" {
		token = strings.TrimPrefix(token, "sk-")
	}

	// 超量用户（令牌数超过上限）只允许精确搜索，禁止模糊搜索
	hasFuzzy := strings.Contains(keyword, "%") || strings.Contains(token, "%")
	if hasFuzzy {
		count, err := CountUserTokens(userId)
		if err != nil {
			common.SysLog("failed to count user tokens: " + err.Error())
			return nil, 0, errors.New("获取令牌数量失败")
		}
		if int(count) > maxUserTokens {
			return nil, 0, errors.New("令牌数量超过上限，仅允许精确搜索，请勿使用 % 通配符")
		}
	}

	baseQuery := dbstore.DB.Model(&Token{}).Where("user_id = ?", userId)

	// group 非空才加等值过滤，空字符串保留原有不过滤行为
	if group != "" {
		baseQuery = baseQuery.Where("group = ?", group)
	}

	// status > 0 时按等值过滤（1=启用 2=禁用 3=过期 4=额度耗尽）；
	// 0 或负值表示不过滤，保留原行为。等值条件三库（SQLite/MySQL/PostgreSQL）均兼容。
	if status > 0 {
		baseQuery = baseQuery.Where("status = ?", status)
	}

	// 非空才加 LIKE 条件，空则跳过（不过滤该字段）。all=true 时用于新 UI 的单框搜索，名称或密钥匹配其一即可。
	if all && keyword != "" && token != "" {
		keywordPattern, err := SanitizeLikePattern(keyword)
		if err != nil {
			return nil, 0, err
		}
		tokenPattern, err := SanitizeLikePattern(token)
		if err != nil {
			return nil, 0, err
		}
		baseQuery = baseQuery.Where("(name LIKE ? ESCAPE '!' OR "+dbstore.CommonKeyCol+" LIKE ? ESCAPE '!')", keywordPattern, tokenPattern)
	} else {
		if keyword != "" {
			keywordPattern, err := SanitizeLikePattern(keyword)
			if err != nil {
				return nil, 0, err
			}
			baseQuery = baseQuery.Where("name LIKE ? ESCAPE '!'", keywordPattern)
		}
		if token != "" {
			tokenPattern, err := SanitizeLikePattern(token)
			if err != nil {
				return nil, 0, err
			}
			baseQuery = baseQuery.Where(dbstore.CommonKeyCol+" LIKE ? ESCAPE '!'", tokenPattern)
		}
	}

	// 先查匹配总数（用于分页，受 maxTokens 上限保护，避免全表 COUNT）
	err = baseQuery.Limit(maxUserTokens).Count(&total).Error
	if err != nil {
		common.SysError("failed to count search tokens: " + err.Error())
		return nil, 0, errors.New("搜索令牌失败")
	}

	// 再分页查数据
	err = baseQuery.Order("id desc").Offset(offset).Limit(limit).Find(&tokens).Error
	if err != nil {
		common.SysError("failed to search tokens: " + err.Error())
		return nil, 0, errors.New("搜索令牌失败")
	}
	return tokens, total, nil
}

func ValidateUserToken(key string) (token *Token, err error) {
	if key == "" {
		return nil, dbstore.ErrTokenNotProvided
	}
	token, err = GetTokenByKey(key, false)
	if err == nil {
		if token.Status == common.TokenStatusExhausted {
			return token, dbstore.ErrTokenInvalid
		} else if token.Status == common.TokenStatusExpired {
			return token, dbstore.ErrTokenInvalid
		}
		if token.Status != common.TokenStatusEnabled {
			return token, dbstore.ErrTokenInvalid
		}
		if token.ExpiredTime != -1 && token.ExpiredTime < common.GetTimestamp() {
			if !redis.RedisEnabled {
				token.Status = common.TokenStatusExpired
				err := token.SelectUpdate()
				if err != nil {
					common.SysLog("failed to update token status" + err.Error())
				}
			}
			return token, dbstore.ErrTokenInvalid
		}

		// 兼容旧数据：如果没有 QuotaType，从 UnlimitedQuota 派生
		quotaType := token.QuotaType
		if quotaType == 0 && !token.UnlimitedQuota {
			quotaType = 1
		}

		// 时段/周期额度需要精确的实时状态，强制从 DB 重载以避免 Redis 缓存延迟
		if quotaType == 2 || quotaType == 3 {
			if fresh, freshErr := GetTokenByKey(key, true); freshErr == nil && fresh != nil {
				token = fresh
				// 重载后必须重新计算 quotaType，否则配额模式切换时仍会按旧分支执行
				quotaType = token.QuotaType
				if quotaType == 0 && !token.UnlimitedQuota {
					quotaType = 1
				}
			}
		}

		switch quotaType {
		case 0: // 无限额度:
			// 不做额度检查
		case 1: // 永久限额:
			if token.RemainQuota <= 0 {
				if !redis.RedisEnabled {
					token.Status = common.TokenStatusExhausted
					err := token.SelectUpdate()
					if err != nil {
						common.SysLog("failed to update token status" + err.Error())
					}
				}
				return token, dbstore.ErrTokenInvalid
			}
		case 2: // 时段限额:
			if token.ShouldResetWindow() {
				windowStart, _ := token.GetCurrentWindow()
				if err := ResetWindowQuota(token.Id, token.WindowStartTime, windowStart); err != nil {
					// CAS 竞争失败或 DB 错误，尝试重新加载最新 token
					if fresh, loadErr := GetTokenByKey(key, true); loadErr == nil && fresh != nil {
						token = fresh
					} else {
						common.SysLog("failed to reset window quota: " + err.Error())
						return token, fmt.Errorf("%w: reset window quota: %v", dbstore.ErrDatabase, err)
					}
				} else {
					token.WindowUsedQuota = 0
					token.WindowStartTime = windowStart
					_ = cacheDeleteToken(token.Key)
				}
			}
			if token.WindowUsedQuota >= token.WindowQuota {
				return token, dbstore.ErrTokenInvalid
			}
		case 3: // 时段+周期限额:
			if token.ShouldResetWindow() {
				windowStart, _ := token.GetCurrentWindow()
				if err := ResetWindowQuota(token.Id, token.WindowStartTime, windowStart); err != nil {
					if fresh, loadErr := GetTokenByKey(key, true); loadErr == nil && fresh != nil {
						token = fresh
					} else {
						common.SysLog("failed to reset window quota: " + err.Error())
						return token, fmt.Errorf("%w: reset window quota: %v", dbstore.ErrDatabase, err)
					}
				} else {
					token.WindowUsedQuota = 0
					token.WindowStartTime = windowStart
					_ = cacheDeleteToken(token.Key)
				}
			}
			if token.ShouldResetCycle() {
				cycleStart, _ := token.GetCurrentCycle()
				if err := ResetCycleQuota(token.Id, token.CycleStartTime, cycleStart); err != nil {
					if fresh, loadErr := GetTokenByKey(key, true); loadErr == nil && fresh != nil {
						token = fresh
					} else {
						common.SysLog("failed to reset cycle quota: " + err.Error())
						return token, fmt.Errorf("%w: reset cycle quota: %v", dbstore.ErrDatabase, err)
					}
				} else {
					token.CycleUsedQuota = 0
					token.CycleStartTime = cycleStart
					_ = cacheDeleteToken(token.Key)
				}
			}
			if token.WindowUsedQuota >= token.WindowQuota {
				return token, dbstore.ErrTokenInvalid
			}
			if token.CycleUsedQuota >= token.CycleQuota {
				return token, dbstore.ErrTokenInvalid
			}
		}
		return token, nil
	}
	common.SysLog("ValidateUserToken: failed to get token: " + err.Error())
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, dbstore.ErrTokenInvalid
	}
	return nil, fmt.Errorf("%w: %v", dbstore.ErrDatabase, err)
}

func GetTokenByIds(id int, userId int) (*Token, error) {
	if id == 0 || userId == 0 {
		return nil, errors.New("id 或 userId 为空！")
	}
	token := Token{Id: id, UserId: userId}
	err := dbstore.DB.First(&token, "id = ? and user_id = ?", id, userId).Error
	return &token, err
}

func GetTokenById(id int) (*Token, error) {
	if id == 0 {
		return nil, errors.New("id 为空！")
	}
	token := Token{Id: id}
	err := dbstore.DB.First(&token, "id = ?", id).Error
	if dbstore.ShouldUpdateRedis(true, err) {
		runtime.RelayGo(func() {
			if err := cacheSetToken(token); err != nil {
				common.SysLog("failed to update user status cache: " + err.Error())
			}
		})
	}
	return &token, err
}

func GetTokenByKey(key string, fromDB bool) (token *Token, err error) {
	defer func() {
		// Update Redis cache asynchronously on successful DB read
		if dbstore.ShouldUpdateRedis(fromDB, err) && token != nil {
			runtime.RelayGo(func() {
				if err := cacheSetToken(*token); err != nil {
					common.SysLog("failed to update user status cache: " + err.Error())
				}
			})
		}
	}()
	if !fromDB && redis.RedisEnabled {
		// Try Redis first
		token, err := cacheGetTokenByKey(key)
		if err == nil {
			return token, nil
		}
		// Don't return error - fall through to DB
	}
	fromDB = true
	loaded, loadErr, _ := tokenLoadGroup.Do(key, func() (interface{}, error) {
		var dbToken *Token
		err := dbstore.DB.Where(dbstore.CommonKeyCol+" = ?", key).First(&dbToken).Error
		return dbToken, err
	})
	if loadErr != nil {
		return nil, loadErr
	}
	loadedToken, ok := loaded.(*Token)
	if !ok {
		return nil, fmt.Errorf("unexpected token cache load type %T", loaded)
	}
	return loadedToken, nil
}

func (token *Token) Insert() error {
	err := dbstore.DB.Create(token).Error
	return err
}

// Update Make sure your token's fields is completed, because this will update non-zero values
func (token *Token) Update() (err error) {
	defer func() {
		if dbstore.ShouldUpdateRedis(true, err) {
			// 同步刷新缓存，确保后续请求立即读到最新的配额模式，
			// 避免异步刷新窗口期内从无限/永久切到时段/周期后仍按旧模式校验。
			if cacheErr := cacheSetToken(*token); cacheErr != nil {
				common.SysLog("failed to update token cache: " + cacheErr.Error())
			}
		}
	}()
	err = dbstore.DB.Model(token).Select("name", "status", "expired_time", "remain_quota", "unlimited_quota",
		"model_limits_enabled", "model_limits", "model_mapping", "allow_ips", "group", "cross_group_retry",
		"quota_type", "window_hours", "window_quota", "window_start_hour",
		"cycle_days", "cycle_quota",
		"window_used_quota", "window_start_time", "cycle_used_quota", "cycle_start_time").Updates(token).Error
	return err
}

func (token *Token) SelectUpdate() (err error) {
	defer func() {
		if dbstore.ShouldUpdateRedis(true, err) {
			runtime.RelayGo(func() {
				err := cacheSetToken(*token)
				if err != nil {
					common.SysLog("failed to update token cache: " + err.Error())
				}
			})
		}
	}()
	// This can update zero values
	return dbstore.DB.Model(token).Select("accessed_time", "status").Updates(token).Error
}

func (token *Token) Delete() (err error) {
	defer func() {
		if dbstore.ShouldUpdateRedis(true, err) {
			runtime.RelayGo(func() {
				err := cacheDeleteToken(token.Key)
				if err != nil {
					common.SysLog("failed to delete token cache: " + err.Error())
				}
			})
		}
	}()
	err = dbstore.DB.Delete(token).Error
	return err
}

func (token *Token) IsModelLimitsEnabled() bool {
	return token.ModelLimitsEnabled
}

func (token *Token) GetModelLimits() []string {
	if token.ModelLimits == "" {
		return []string{}
	}
	return strings.Split(token.ModelLimits, ",")
}

func (token *Token) GetModelLimitsMap() map[string]bool {
	limits := token.GetModelLimits()
	limitsMap := make(map[string]bool)
	for _, limit := range limits {
		limitsMap[limit] = true
	}
	return limitsMap
}

// GetModelMapping 返回令牌级模型重定向的 JSON 字符串，未配置时返回空字符串。
func (token *Token) GetModelMapping() string {
	if token.ModelMapping == nil {
		return ""
	}
	return *token.ModelMapping
}

// GetModelMappingMap 将令牌级模型重定向规则解析为 map，未配置或格式非法时返回 nil。
func (token *Token) GetModelMappingMap() map[string]string {
	mappingStr := token.GetModelMapping()
	if mappingStr == "" || mappingStr == "{}" {
		return nil
	}
	modelMap := make(map[string]string)
	if err := jsonx.Unmarshal([]byte(mappingStr), &modelMap); err != nil {
		return nil
	}
	if len(modelMap) == 0 {
		return nil
	}
	return modelMap
}

func DisableModelLimits(tokenId int) error {
	token, err := GetTokenById(tokenId)
	if err != nil {
		return err
	}
	token.ModelLimitsEnabled = false
	token.ModelLimits = ""
	return token.Update()
}

func DeleteTokenById(id int, userId int) (err error) {
	// Why we need userId here? In case user want to delete other's token.
	if id == 0 || userId == 0 {
		return errors.New("id 或 userId 为空！")
	}
	token := Token{Id: id, UserId: userId}
	err = dbstore.DB.Where(token).First(&token).Error
	if err != nil {
		return err
	}
	return token.Delete()
}

func IncreaseTokenQuota(id int, key string, quota int) (err error) {
	if quota < 0 {
		return errors.New("quota 不能为负数！")
	}
	if redis.RedisEnabled {
		runtime.RelayGo(func() {
			if cacheErr := cacheIncrTokenQuota(key, int64(quota)); cacheErr != nil {
				common.SysLog("failed to increase token quota: " + cacheErr.Error())
			}
			if cacheErr := cacheIncrTokenUsedQuota(key, -int64(quota)); cacheErr != nil {
				common.SysLog("failed to decrease token used quota: " + cacheErr.Error())
			}
		})
	}
	if common.BatchUpdateEnabled {
		dbstore.AddNewRecord(dbstore.BatchUpdateTypeTokenQuota, id, quota)
		return nil
	}
	return increaseTokenQuota(id, quota)
}

func increaseTokenQuota(id int, quota int) (err error) {
	err = dbstore.DB.Model(&Token{}).Where("id = ?", id).Updates(
		map[string]interface{}{
			"remain_quota":  gorm.Expr("remain_quota + ?", quota),
			"used_quota":    gorm.Expr("used_quota - ?", quota),
			"accessed_time": common.GetTimestamp(),
		},
	).Error
	return err
}

func DecreaseTokenQuota(id int, key string, quota int) (err error) {
	if quota < 0 {
		return errors.New("quota 不能为负数！")
	}
	if redis.RedisEnabled {
		runtime.RelayGo(func() {
			if cacheErr := cacheDecrTokenQuota(key, int64(quota)); cacheErr != nil {
				common.SysLog("failed to decrease token quota: " + cacheErr.Error())
			}
			if cacheErr := cacheIncrTokenUsedQuota(key, int64(quota)); cacheErr != nil {
				common.SysLog("failed to increase token used quota: " + cacheErr.Error())
			}
		})
	}
	if common.BatchUpdateEnabled {
		dbstore.AddNewRecord(dbstore.BatchUpdateTypeTokenQuota, id, -quota)
		return nil
	}
	return decreaseTokenQuota(id, quota)
}

func decreaseTokenQuota(id int, quota int) (err error) {
	err = dbstore.DB.Model(&Token{}).Where("id = ?", id).Updates(
		map[string]interface{}{
			"remain_quota":  gorm.Expr("remain_quota - ?", quota),
			"used_quota":    gorm.Expr("used_quota + ?", quota),
			"accessed_time": common.GetTimestamp(),
		},
	).Error
	return err
}

func UpdateTokenUsedQuota(id int, key string, delta int) error {
	if delta == 0 {
		return nil
	}
	if redis.RedisEnabled {
		runtime.RelayGo(func() {
			if cacheErr := cacheIncrTokenUsedQuota(key, int64(delta)); cacheErr != nil {
				common.SysLog("failed to update token used quota: " + cacheErr.Error())
			}
		})
	}
	if common.BatchUpdateEnabled {
		dbstore.AddNewRecord(dbstore.BatchUpdateTypeTokenUsedQuota, id, delta)
		return nil
	}
	return updateTokenUsedQuota(id, delta)
}

func updateTokenUsedQuota(id int, delta int) error {
	return dbstore.DB.Model(&Token{}).Where("id = ?", id).Updates(
		map[string]interface{}{
			"used_quota":    gorm.Expr("used_quota + ?", delta),
			"accessed_time": common.GetTimestamp(),
		},
	).Error
}

// IncreaseWindowQuota 增加窗口已用额度（退还额度时使用）
func IncreaseWindowQuota(id int, key string, quota int) (err error) {
	if quota < 0 {
		return errors.New("quota 不能为负数！")
	}
	err = increaseWindowQuota(id, -quota)
	if err != nil {
		return err
	}
	if redis.RedisEnabled {
		runtime.RelayGo(func() {
			err := cacheIncrWindowUsedQuota(key, -int64(quota))
			if err != nil {
				common.SysLog("failed to increase window quota: " + err.Error())
			}
		})
	}
	return nil
}

// DecreaseWindowQuota 减少窗口已用额度（扣费时使用）
func DecreaseWindowQuota(id int, key string, quota int) (err error) {
	if quota < 0 {
		return errors.New("quota 不能为负数！")
	}
	err = decreaseWindowQuota(id, quota)
	if err != nil {
		return err
	}
	if redis.RedisEnabled {
		runtime.RelayGo(func() {
			if cacheErr := cacheIncrWindowUsedQuota(key, int64(quota)); cacheErr != nil {
				common.SysLog("failed to increase window used quota: " + cacheErr.Error())
			}
		})
	}
	return nil
}

func decreaseWindowQuota(id int, quota int) (err error) {
	result := dbstore.DB.Model(&Token{}).Where("id = ? AND window_used_quota + ? <= window_quota", id, quota).Updates(
		map[string]interface{}{
			"window_used_quota": gorm.Expr("window_used_quota + ?", quota),
			"accessed_time":     common.GetTimestamp(),
		},
	)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return errors.New("token window quota is not enough")
	}
	return nil
}

func increaseWindowQuota(id int, quota int) (err error) {
	updates := map[string]interface{}{
		"window_used_quota": gorm.Expr("window_used_quota + ?", quota),
		"accessed_time":     common.GetTimestamp(),
	}
	err = dbstore.DB.Model(&Token{}).Where("id = ?", id).Updates(updates).Error
	return err
}

// IncreaseCycleQuota 增加周期已用额度（退还额度时使用，传入负值）
func IncreaseCycleQuota(id int, key string, quota int) (err error) {
	if quota < 0 {
		return errors.New("quota 不能为负数！")
	}
	err = increaseCycleQuota(id, -quota)
	if err != nil {
		return err
	}
	if redis.RedisEnabled {
		runtime.RelayGo(func() {
			err := cacheIncrCycleUsedQuota(key, -int64(quota))
			if err != nil {
				common.SysLog("failed to increase cycle quota: " + err.Error())
			}
		})
	}
	return nil
}

// DecreaseCycleQuota 减少周期已用额度（扣费时使用）
func DecreaseCycleQuota(id int, key string, quota int) (err error) {
	if quota < 0 {
		return errors.New("quota 不能为负数！")
	}
	err = decreaseCycleQuota(id, quota)
	if err != nil {
		return err
	}
	if redis.RedisEnabled {
		runtime.RelayGo(func() {
			if cacheErr := cacheIncrCycleUsedQuota(key, int64(quota)); cacheErr != nil {
				common.SysLog("failed to increase cycle used quota: " + cacheErr.Error())
			}
		})
	}
	return nil
}

func decreaseCycleQuota(id int, quota int) (err error) {
	result := dbstore.DB.Model(&Token{}).Where("id = ? AND cycle_used_quota + ? <= cycle_quota", id, quota).Updates(
		map[string]interface{}{
			"cycle_used_quota": gorm.Expr("cycle_used_quota + ?", quota),
			"accessed_time":    common.GetTimestamp(),
		},
	)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return errors.New("token cycle quota is not enough")
	}
	return nil
}

func increaseCycleQuota(id int, quota int) (err error) {
	err = dbstore.DB.Model(&Token{}).Where("id = ?", id).Updates(
		map[string]interface{}{
			"cycle_used_quota": gorm.Expr("cycle_used_quota + ?", quota),
			"accessed_time":    common.GetTimestamp(),
		},
	).Error
	return err
}

// ResetWindowQuota 重置窗口额度到新窗口，仅在旧的 window_start_time 匹配时才执行，防止并发边界覆盖其他请求已扣减的额度。
func ResetWindowQuota(id int, oldStart int64, newStart int64) (err error) {
	result := dbstore.DB.Model(&Token{}).Where("id = ? AND window_start_time = ?", id, oldStart).Updates(
		map[string]interface{}{
			"window_used_quota": 0,
			"window_start_time": newStart,
		},
	)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return errors.New("window already reset by another request")
	}
	return nil
}

// ResetCycleQuota 重置周期额度到新周期，仅在旧的 cycle_start_time 匹配时才执行，防止并发边界覆盖其他请求已扣减的额度。
func ResetCycleQuota(id int, oldStart int64, newStart int64) (err error) {
	result := dbstore.DB.Model(&Token{}).Where("id = ? AND cycle_start_time = ?", id, oldStart).Updates(
		map[string]interface{}{
			"cycle_used_quota": 0,
			"cycle_start_time": newStart,
		},
	)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return errors.New("cycle already reset by another request")
	}
	return nil
}

// CountUserTokens returns total number of tokens for the given user, used for pagination
func CountUserTokens(userId int) (int64, error) {
	var total int64
	err := dbstore.DB.Model(&Token{}).Where("user_id = ?", userId).Count(&total).Error
	return total, err
}

// ResetTokenKey 重置令牌密钥，仅更新 key 字段，其他字段不变
// 需要 userId 参数以验证令牌归属，防止越权操作
func ResetTokenKey(id int, userId int) (newKey string, err error) {
	if id == 0 || userId == 0 {
		return "", errors.New("id 或 userId 为空！")
	}

	// 按 token ID 加锁，串行化同一令牌的并发重置请求
	mu := getTokenResetLock(id)
	mu.Lock()
	defer mu.Unlock()

	// 先通过 id+userId 查询，验证令牌归属
	token := Token{Id: id, UserId: userId}
	err = dbstore.DB.First(&token, "id = ? and user_id = ?", id, userId).Error
	if err != nil {
		return "", err
	}
	// 生成新 key
	newKey, err = common.GenerateKey()
	if err != nil {
		return "", err
	}
	oldKey := token.Key
	// 更新数据库中的 key
	err = dbstore.DB.Model(&token).Update("key", newKey).Error
	if err != nil {
		return "", err
	}
	token.Key = newKey

	// 在锁内同步完成旧缓存删除 + 新缓存写入，确保任意时刻只有一个有效 key
	if redis.RedisEnabled {
		if delErr := cacheDeleteToken(oldKey); delErr != nil {
			common.SysError("failed to delete old token cache after reset key: " + delErr.Error())
		}
		if setErr := cacheSetToken(token); setErr != nil {
			common.SysError("failed to update token cache after reset key: " + setErr.Error())
		}
	}
	return newKey, nil
}

// BatchDeleteTokens 删除指定用户的一组令牌，返回成功删除数量
func BatchDeleteTokens(ids []int, userId int) (int, error) {
	if len(ids) == 0 {
		return 0, errors.New("ids 不能为空！")
	}

	tx := dbstore.DB.Begin()

	var tokens []Token
	if err := tx.Where("user_id = ? AND id IN (?)", userId, ids).Find(&tokens).Error; err != nil {
		tx.Rollback()
		return 0, err
	}

	if err := tx.Where("user_id = ? AND id IN (?)", userId, ids).Delete(&Token{}).Error; err != nil {
		tx.Rollback()
		return 0, err
	}

	if err := tx.Commit().Error; err != nil {
		return 0, err
	}

	if redis.RedisEnabled {
		runtime.RelayGo(func() {
			for _, t := range tokens {
				_ = cacheDeleteToken(t.Key)
			}
		})
	}

	return len(tokens), nil
}

func InvalidateUserTokensCache(userId int) error {
	if userId == 0 || !redis.RedisEnabled {
		return nil
	}

	var tokens []Token
	if err := dbstore.DB.Select("key").Where("user_id = ?", userId).Find(&tokens).Error; err != nil {
		return err
	}
	for _, token := range tokens {
		if token.Key == "" {
			continue
		}
		if err := cacheDeleteToken(token.Key); err != nil {
			return err
		}
	}
	return nil
}

// init 把 token 域的批量落库函数注册到 dbstore 的批量更新器。
// BatchUpdateTypeTokenQuota / TokenUsedQuota 等类型的批量条目只会由本包写入，
// 因此注册与写入天然同生命周期，不存在 flusher 缺失。
func init() {
	dbstore.RegisterBatchFlushers(dbstore.BatchFlushers{
		TokenQuota:     increaseTokenQuota,
		WindowQuota:    increaseWindowQuota,
		CycleQuota:     increaseCycleQuota,
		TokenUsedQuota: updateTokenUsedQuota,
	})
}
