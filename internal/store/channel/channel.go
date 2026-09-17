package channelstore

import (
	"database/sql/driver"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/NookMux/NookMux/internal/common"
	"github.com/NookMux/NookMux/internal/domain/channel/constant"
	"github.com/NookMux/NookMux/internal/domain/shared"
	infradb "github.com/NookMux/NookMux/internal/infra/db"
	"github.com/NookMux/NookMux/internal/store/db"
	"github.com/NookMux/NookMux/pkg/jsonx"
	"github.com/samber/lo"
	"gorm.io/gorm"
	"math/rand"
	"strings"
	"sync"
)

type Channel struct {
	Id                 int     `json:"id"`
	Type               int     `json:"type" gorm:"default:0"`
	Key                string  `json:"key"`
	OpenAIOrganization *string `json:"openai_organization"`
	TestModel          *string `json:"test_model"`
	Status             int     `json:"status" gorm:"default:1"`
	Name               string  `json:"name" gorm:"index"`
	Weight             *uint   `json:"weight" gorm:"default:0"`
	CreatedTime        int64   `json:"created_time" gorm:"bigint"`
	TestTime           int64   `json:"test_time" gorm:"bigint"`
	ResponseTime       int     `json:"response_time"` // in milliseconds
	BaseURL            *string `json:"base_url" gorm:"column:base_url;default:''"`
	Other              string  `json:"other"`
	Balance            float64 `json:"balance"` // in USD
	BalanceUpdatedTime int64   `json:"balance_updated_time" gorm:"bigint"`
	Models             string  `json:"models"`
	Group              string  `json:"group" gorm:"type:varchar(64);default:'default'"`
	UsedQuota          int64   `json:"used_quota" gorm:"bigint;default:0"`
	ModelMapping       *string `json:"model_mapping" gorm:"type:text"`
	//MaxInputTokens     *int    `json:"max_input_tokens" gorm:"default:0"`
	StatusCodeMapping *string `json:"status_code_mapping" gorm:"type:varchar(1024);default:''"`
	Priority          *int64  `json:"priority" gorm:"bigint;default:0"`
	AutoBan           *int    `json:"auto_ban" gorm:"default:1"`
	OtherInfo         string  `json:"other_info"`
	Tag               *string `json:"tag" gorm:"index"`
	Setting           *string `json:"setting" gorm:"type:text"` // 渠道额外设置
	ParamOverride     *string `json:"param_override" gorm:"type:text"`
	HeaderOverride    *string `json:"header_override" gorm:"type:text"`
	Remark            *string `json:"remark" gorm:"type:varchar(255)" validate:"max=255"`
	// add after v0.8.5
	ChannelInfo ChannelInfo `json:"channel_info" gorm:"type:json"`

	OtherSettings string `json:"settings" gorm:"column:settings"` // 其他设置，存储azure版本等不需要检索的信息，详见dto.ChannelOtherSettings

	// cache info
	Keys []string `json:"-" gorm:"-"`
}

type ChannelInfo struct {
	IsMultiKey             bool                  `json:"is_multi_key"`                        // 是否多Key模式
	MultiKeySize           int                   `json:"multi_key_size"`                      // 多Key模式下的Key数量
	MultiKeyStatusList     map[int]int           `json:"multi_key_status_list"`               // key状态列表，key index -> status
	MultiKeyDisabledReason map[int]string        `json:"multi_key_disabled_reason,omitempty"` // key禁用原因列表，key index -> reason
	MultiKeyDisabledTime   map[int]int64         `json:"multi_key_disabled_time,omitempty"`   // key禁用时间列表，key index -> time
	MultiKeyPollingIndex   int                   `json:"multi_key_polling_index"`             // 多Key模式下轮询的key索引
	MultiKeyMode           constant.MultiKeyMode `json:"multi_key_mode"`
	IsPlan                 bool                  `json:"is_plan"`   // 是否为套餐渠道
	PlanName               string                `json:"plan_name"` // 套餐标识名称，对应 ChannelSpecialBases 的 key
}

func NormalizeChannelGroupFilter(group string) string {
	group = strings.TrimSpace(group)
	if group == "" || strings.EqualFold(group, "all") || strings.EqualFold(group, "null") {
		return ""
	}
	return group
}

func channelGroupFilterCondition() string {
	if infradb.UsingMySQL {
		return `CONCAT(',', ` + dbstore.CommonGroupCol + `, ',') LIKE ? ESCAPE '!'`
	}
	return `(',' || ` + dbstore.CommonGroupCol + ` || ',') LIKE ? ESCAPE '!'`
}

func channelGroupFilterPattern(group string) string {
	group = strings.NewReplacer(
		"!", "!!",
		"%", "!%",
		"_", "!_",
	).Replace(group)
	return "%," + group + ",%"
}

func ApplyChannelGroupFilter(query *gorm.DB, group string) *gorm.DB {
	group = NormalizeChannelGroupFilter(group)
	if group == "" {
		return query
	}
	return query.Where(channelGroupFilterCondition(), channelGroupFilterPattern(group))
}

// Value implements driver.Valuer interface.
// PostgreSQL 的 json/jsonb 列在 pgx 驱动下对 []byte 参数支持不稳定，
// 返回 string 形式的 JSON 能同时兼容 SQLite/MySQL/PostgreSQL。
func (c ChannelInfo) Value() (driver.Value, error) {
	bytes, err := jsonx.Marshal(&c)
	if err != nil {
		return nil, err
	}
	return string(bytes), nil
}

// Scan implements sql.Scanner interface.
// 兼容 nil、string、[]byte，以及空值（恢复为零值 ChannelInfo），
// 避免旧数据/NULL/空字符串导致 Scan 失败。
func (c *ChannelInfo) Scan(value interface{}) error {
	if value == nil {
		*c = ChannelInfo{}
		return nil
	}
	var raw []byte
	switch v := value.(type) {
	case []byte:
		raw = v
	case string:
		raw = []byte(v)
	default:
		return fmt.Errorf("failed to scan ChannelInfo: unsupported type %T", value)
	}
	if len(strings.TrimSpace(string(raw))) == 0 {
		*c = ChannelInfo{}
		return nil
	}
	return jsonx.Unmarshal(raw, c)
}

func (channel *Channel) GetKeys() []string {
	if channel.Key == "" {
		return []string{}
	}
	if len(channel.Keys) > 0 {
		return channel.Keys
	}
	trimmed := strings.TrimSpace(channel.Key)
	// If the key starts with '[', try to parse it as a JSON array (e.g., for Vertex AI scenarios)
	if strings.HasPrefix(trimmed, "[") {
		var arr []json.RawMessage
		if err := jsonx.Unmarshal([]byte(trimmed), &arr); err == nil {
			res := make([]string, len(arr))
			for i, v := range arr {
				res[i] = string(v)
			}
			return res
		}
	}
	// Otherwise, fall back to splitting by newline
	keys := strings.Split(strings.Trim(channel.Key, "\n"), "\n")
	return keys
}

// DetectPlan 检测渠道的 BaseURL 是否为内置套餐地址，并更新 ChannelInfo
func (channel *Channel) DetectPlan() {
	baseURL := ""
	if channel.BaseURL != nil {
		baseURL = strings.TrimSpace(*channel.BaseURL)
	}
	planName, isPlan := constant.IsChannelPlan(baseURL)
	channel.ChannelInfo.IsPlan = isPlan
	channel.ChannelInfo.PlanName = planName
}

func (channel *Channel) GetNextEnabledKey() (string, int, *shared.NookMuxError) {
	// If not in multi-key mode, return the original key string directly.
	if !channel.ChannelInfo.IsMultiKey {
		return channel.Key, 0, nil
	}

	// Obtain all keys (split by \n)
	keys := channel.GetKeys()
	if len(keys) == 0 {
		// No keys available, return error, should disable the channel
		return "", 0, shared.NewError(errors.New("no keys available"), shared.ErrorCodeChannelNoAvailableKey)
	}

	lock := GetChannelPollingLock(channel.Id)
	lock.Lock()
	defer lock.Unlock()

	statusList := channel.ChannelInfo.MultiKeyStatusList
	// helper to get key status, default to enabled when missing
	getStatus := func(idx int) int {
		if statusList == nil {
			return common.ChannelStatusEnabled
		}
		if status, ok := statusList[idx]; ok {
			return status
		}
		return common.ChannelStatusEnabled
	}

	// Collect indexes of enabled keys
	enabledIdx := make([]int, 0, len(keys))
	for i := range keys {
		if getStatus(i) == common.ChannelStatusEnabled {
			enabledIdx = append(enabledIdx, i)
		}
	}
	// If no specific status list or none enabled, return an explicit error so caller can
	// properly handle a channel with no available keys (e.g. mark channel disabled).
	// Returning the first key here caused requests to keep using an already-disabled key.
	if len(enabledIdx) == 0 {
		return "", 0, shared.NewError(errors.New("no enabled keys"), shared.ErrorCodeChannelNoAvailableKey)
	}

	switch channel.ChannelInfo.MultiKeyMode {
	case constant.MultiKeyModeRandom:
		// Randomly pick one enabled key
		selectedIdx := enabledIdx[rand.Intn(len(enabledIdx))]
		return keys[selectedIdx], selectedIdx, nil
	case constant.MultiKeyModePolling:
		// Use channel-specific lock to ensure thread-safe polling

		channelInfo, err := CacheGetChannelInfo(channel.Id)
		if err != nil {
			return "", 0, shared.NewError(err, shared.ErrorCodeGetChannelFailed, shared.ErrOptionWithSkipRetry())
		}
		//println("before polling index:", channel.ChannelInfo.MultiKeyPollingIndex)
		defer func() {
			if common.DebugEnabled {
				println(fmt.Sprintf("channel %d polling index: %d", channel.Id, channel.ChannelInfo.MultiKeyPollingIndex))
			}
			if !common.MemoryCacheEnabled {
				_ = channel.SaveChannelInfo()
			}
			// 内存缓存开启时由缓存侧统一失效，无需落库。
		}()
		// Start from the saved polling index and look for the next enabled key
		start := channelInfo.MultiKeyPollingIndex
		if start < 0 || start >= len(keys) {
			start = 0
		}
		for i := 0; i < len(keys); i++ {
			idx := (start + i) % len(keys)
			if getStatus(idx) == common.ChannelStatusEnabled {
				// update polling index for next call (point to the next position)
				channel.ChannelInfo.MultiKeyPollingIndex = (idx + 1) % len(keys)
				return keys[idx], idx, nil
			}
		}
		// Fallback – should not happen, but return first enabled key
		return keys[enabledIdx[0]], enabledIdx[0], nil
	default:
		// Unknown mode, default to first enabled key (or original key string)
		return keys[enabledIdx[0]], enabledIdx[0], nil
	}
}

func (channel *Channel) SaveChannelInfo() error {
	return dbstore.DB.Model(channel).Update("channel_info", channel.ChannelInfo).Error
}

func (channel *Channel) GetModels() []string {
	if channel.Models == "" {
		return []string{}
	}
	return strings.Split(strings.Trim(channel.Models, ","), ",")
}

func (channel *Channel) GetGroups() []string {
	if channel.Group == "" {
		return []string{}
	}
	groups := strings.Split(strings.Trim(channel.Group, ","), ",")
	for i, group := range groups {
		groups[i] = strings.TrimSpace(group)
	}
	return groups
}

func (channel *Channel) GetOtherInfo() map[string]interface{} {
	otherInfo := make(map[string]interface{})
	if channel.OtherInfo != "" {
		err := jsonx.Unmarshal([]byte(channel.OtherInfo), &otherInfo)
		if err != nil {
			common.SysLog(fmt.Sprintf("failed to unmarshal other info: channel_id=%d, tag=%s, name=%s, error=%v", channel.Id, channel.GetTag(), channel.Name, err))
		}
	}
	return otherInfo
}

func (channel *Channel) SetOtherInfo(otherInfo map[string]interface{}) {
	otherInfoBytes, err := jsonx.Marshal(otherInfo)
	if err != nil {
		common.SysLog(fmt.Sprintf("failed to marshal other info: channel_id=%d, tag=%s, name=%s, error=%v", channel.Id, channel.GetTag(), channel.Name, err))
		return
	}
	channel.OtherInfo = string(otherInfoBytes)
}

func (channel *Channel) GetTag() string {
	if channel.Tag == nil {
		return ""
	}
	return *channel.Tag
}

func (channel *Channel) SetTag(tag string) {
	channel.Tag = &tag
}

func (channel *Channel) GetAutoBan() bool {
	if channel.AutoBan == nil {
		return false
	}
	return *channel.AutoBan == 1
}

func (channel *Channel) Save() error {
	return dbstore.DB.Save(channel).Error
}

func (channel *Channel) SaveWithoutKey() error {
	if channel.Id == 0 {
		return errors.New("channel ID is 0")
	}
	return dbstore.DB.Omit("key").Save(channel).Error
}

func GetAllChannels(startIdx int, num int, selectAll bool, idSort bool) ([]*Channel, error) {
	var channels []*Channel
	var err error
	order := "LOWER(name) asc, id asc"
	if idSort {
		order = "id desc"
	}
	if selectAll {
		err = dbstore.DB.Order(order).Find(&channels).Error
	} else {
		err = dbstore.DB.Order(order).Limit(num).Offset(startIdx).Omit("key").Find(&channels).Error
	}
	return channels, err
}

func GetChannelsByTag(tag string, idSort bool, selectAll bool) ([]*Channel, error) {
	return GetChannelsByTagWithGroup(tag, "", idSort, selectAll)
}

func GetChannelsByTagWithGroup(tag string, group string, idSort bool, selectAll bool) ([]*Channel, error) {
	var channels []*Channel
	// 标签聚合模式下固定按优先级、权重排序，忽略 idSort
	order := "priority desc, weight desc"
	query := ApplyChannelGroupFilter(dbstore.DB.Where("tag = ?", tag), group).Order(order)
	if !selectAll {
		query = query.Omit("key")
	}
	err := query.Find(&channels).Error
	return channels, err
}

// searchChannelsDefaultLimit / searchChannelsMaxLimit 限定渠道搜索的单页加载量：
// 搜索结果历史上无 LIMIT 全量加载，渠道量大时放大内存/CPU 消耗。
const (
	searchChannelsDefaultLimit = 100
	searchChannelsMaxLimit     = 500
)

// normalizeSearchChannelsLimit 规范化 limit：<=0 取默认上限，>上限截断防滥用。
func normalizeSearchChannelsLimit(limit int) int {
	if limit <= 0 {
		return searchChannelsDefaultLimit
	}
	if limit > searchChannelsMaxLimit {
		return searchChannelsMaxLimit
	}
	return limit
}

// NormalizeChannelSearchLimit 供 controller 在计算分页 offset 前统一 page_size，
// 与 SQL LIMIT 使用同一有效值，避免 page_size 超上限时因两者不一致丢数据。
func NormalizeChannelSearchLimit(pageSize int) int {
	return normalizeSearchChannelsLimit(pageSize)
}

// buildSearchChannelsQuery 构造渠道搜索的基础查询（keyword/group/model/id/name/tag
// 条件），供分页查询与 total/type_counts 聚合查询复用，避免三份 where 复制。
func buildSearchChannelsQuery(keyword string, group string, model string, idFilter int, nameFilter string, tagFilter string) *gorm.DB {
	modelsCol := "`models`"

	// 如果是 PostgreSQL，使用双引号
	if infradb.UsingPostgreSQL {
		modelsCol = `"models"`
	}

	baseURLCol := "`base_url`"
	// 如果是 PostgreSQL，使用双引号
	if infradb.UsingPostgreSQL {
		baseURLCol = `"base_url"`
	}

	baseQuery := dbstore.DB.Model(&Channel{}).Omit("key")

	whereClause := "(id = ? OR name LIKE ? OR " + dbstore.CommonKeyCol + " = ? OR " + baseURLCol + " LIKE ?) AND " + modelsCol + " LIKE ?"
	args := []interface{}{common.String2Int(keyword), "%" + keyword + "%", keyword, "%" + keyword + "%", "%" + model + "%"}
	baseQuery = ApplyChannelGroupFilter(baseQuery.Where(whereClause, args...), group)

	// 独立过滤条件（AND 叠加），支持字段级服务端过滤
	if idFilter > 0 {
		baseQuery = baseQuery.Where("id = ?", idFilter)
	}
	if nameFilter != "" {
		baseQuery = baseQuery.Where("name LIKE ?", "%"+nameFilter+"%")
	}
	if tagFilter != "" {
		baseQuery = baseQuery.Where("tag LIKE ?", "%"+tagFilter+"%")
	}
	return baseQuery
}

// searchChannelsOrder 返回渠道搜索的排序子句。
func searchChannelsOrder(idSort bool) string {
	if idSort {
		return "id desc"
	}
	return "LOWER(name) asc, id asc"
}

func SearchChannels(keyword string, group string, model string, idSort bool, idFilter int, nameFilter string, tagFilter string, offset int, limit int) ([]*Channel, error) {
	var channels []*Channel

	// 执行查询（SQL 层分页，避免全量加载）
	err := buildSearchChannelsQuery(keyword, group, model, idFilter, nameFilter, tagFilter).
		Order(searchChannelsOrder(idSort)).
		Offset(offset).
		Limit(normalizeSearchChannelsLimit(limit)).
		Find(&channels).Error
	if err != nil {
		return nil, err
	}
	return channels, nil
}

// SearchChannelsWithMeta 在 SearchChannels 的分页查询之外，一并返回跨页聚合：
// total 为满足 where 条件的记录总数，typeCounts 为按 type 分组的计数。
//
// statusFilter 语义与控制器历史内存过滤保持一致，直接下推到 SQL：
//   - 传入 common.ChannelStatusEnabled（1）→ 只统计 enabled；
//   - 传入 0 → 只统计非 enabled（status != 1）；
//   - 传入 -1 → 不过滤。
//
// typeFilter 只作用于 items 与 total，不参与 typeCounts 聚合（与原实现
// "status 过滤后、type 过滤前统计 type_counts" 的语义一致）。
func SearchChannelsWithMeta(keyword string, group string, model string, idSort bool, idFilter int, nameFilter, tagFilter string, statusFilter int, typeFilter int, offset int, limit int) ([]*Channel, int64, map[int64]int64, error) {
	applyStatusFilter := func(query *gorm.DB) *gorm.DB {
		if statusFilter == common.ChannelStatusEnabled {
			return query.Where("status = ?", common.ChannelStatusEnabled)
		}
		if statusFilter == 0 {
			return query.Where("status != ?", common.ChannelStatusEnabled)
		}
		return query
	}

	// type_counts：where（keyword/group/id/name/tag + status）聚合，不带 type 条件
	typeCounts := make(map[int64]int64)
	type typeCountResult struct {
		Type  int64 `gorm:"column:type"`
		Count int64 `gorm:"column:count"`
	}
	var typeCountResults []typeCountResult
	err := applyStatusFilter(buildSearchChannelsQuery(keyword, group, model, idFilter, nameFilter, tagFilter)).
		Select("type, count(*) as count").
		Group("type").
		Find(&typeCountResults).Error
	if err != nil {
		return nil, 0, nil, err
	}
	for _, r := range typeCountResults {
		typeCounts[r.Type] = r.Count
	}

	// total：where + status + type
	var total int64
	countQuery := applyStatusFilter(buildSearchChannelsQuery(keyword, group, model, idFilter, nameFilter, tagFilter))
	if typeFilter >= 0 {
		countQuery = countQuery.Where("type = ?", typeFilter)
	}
	if err := countQuery.Count(&total).Error; err != nil {
		return nil, 0, nil, err
	}

	// items：where + status + type + 分页
	itemsQuery := applyStatusFilter(buildSearchChannelsQuery(keyword, group, model, idFilter, nameFilter, tagFilter))
	if typeFilter >= 0 {
		itemsQuery = itemsQuery.Where("type = ?", typeFilter)
	}
	var channels []*Channel
	err = itemsQuery.
		Order(searchChannelsOrder(idSort)).
		Offset(offset).
		Limit(normalizeSearchChannelsLimit(limit)).
		Find(&channels).Error
	if err != nil {
		return nil, 0, nil, err
	}
	return channels, total, typeCounts, nil
}

func GetChannelById(id int, selectAll bool) (*Channel, error) {
	channel := &Channel{Id: id}
	var err error
	if selectAll {
		err = dbstore.DB.First(channel, "id = ?", id).Error
	} else {
		err = dbstore.DB.Omit("key").First(channel, "id = ?", id).Error
	}
	if err != nil {
		return nil, err
	}
	return channel, nil
}

func BatchInsertChannels(channels []Channel) error {
	if len(channels) == 0 {
		return nil
	}
	tx := dbstore.DB.Begin()
	if tx.Error != nil {
		return tx.Error
	}
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	for _, chunk := range lo.Chunk(channels, 50) {
		if err := tx.Create(&chunk).Error; err != nil {
			tx.Rollback()
			return err
		}
		for _, channel_ := range chunk {
			if err := channel_.AddAbilities(tx); err != nil {
				tx.Rollback()
				return err
			}
		}
	}
	return tx.Commit().Error
}

func BatchDeleteChannels(ids []int) error {
	if len(ids) == 0 {
		return nil
	}
	// 使用事务 分批删除channel表和abilities表
	tx := dbstore.DB.Begin()
	if tx.Error != nil {
		return tx.Error
	}
	for _, chunk := range lo.Chunk(ids, 200) {
		if err := tx.Where("id in (?)", chunk).Delete(&Channel{}).Error; err != nil {
			tx.Rollback()
			return err
		}
		if err := tx.Where("channel_id in (?)", chunk).Delete(&Ability{}).Error; err != nil {
			tx.Rollback()
			return err
		}
	}
	return tx.Commit().Error
}

func (channel *Channel) GetPriority() int64 {
	if channel.Priority == nil {
		return 0
	}
	return *channel.Priority
}

func (channel *Channel) GetWeight() int {
	if channel.Weight == nil {
		return 0
	}
	return int(*channel.Weight)
}

func (channel *Channel) GetBaseURL() string {
	if channel.BaseURL == nil {
		return ""
	}
	url := *channel.BaseURL
	if url == "" {
		url = constant.ChannelBaseURLs[channel.Type]
	}
	return url
}

func (channel *Channel) GetModelMapping() string {
	if channel.ModelMapping == nil {
		return ""
	}
	return *channel.ModelMapping
}

func (channel *Channel) GetStatusCodeMapping() string {
	if channel.StatusCodeMapping == nil {
		return ""
	}
	return *channel.StatusCodeMapping
}

func (channel *Channel) Insert() error {
	var err error
	err = dbstore.DB.Create(channel).Error
	if err != nil {
		return err
	}
	err = channel.AddAbilities(nil)
	return err
}

func (channel *Channel) Update() error {
	// If this is a multi-key channel, recalculate MultiKeySize based on the current key list to avoid inconsistency after editing keys
	if channel.ChannelInfo.IsMultiKey {
		var keyStr string
		if channel.Key != "" {
			keyStr = channel.Key
		} else {
			// If key is not provided, read the existing key from the database
			if existing, err := GetChannelById(channel.Id, true); err == nil {
				keyStr = existing.Key
			}
		}
		// Parse the key list (supports newline separation or JSON array)
		keys := []string{}
		if keyStr != "" {
			trimmed := strings.TrimSpace(keyStr)
			if strings.HasPrefix(trimmed, "[") {
				var arr []json.RawMessage
				if err := jsonx.Unmarshal([]byte(trimmed), &arr); err == nil {
					keys = make([]string, len(arr))
					for i, v := range arr {
						keys[i] = string(v)
					}
				}
			}
			if len(keys) == 0 { // fallback to newline split
				keys = strings.Split(strings.Trim(keyStr, "\n"), "\n")
			}
		}
		channel.ChannelInfo.MultiKeySize = len(keys)
		// Clean up status data that exceeds the new key count to prevent index out of range
		if channel.ChannelInfo.MultiKeyStatusList != nil {
			for idx := range channel.ChannelInfo.MultiKeyStatusList {
				if idx >= channel.ChannelInfo.MultiKeySize {
					delete(channel.ChannelInfo.MultiKeyStatusList, idx)
				}
			}
		}
	}
	var err error
	err = dbstore.DB.Model(channel).Updates(channel).Error
	if err != nil {
		return err
	}
	dbstore.DB.Model(channel).First(channel, "id = ?", channel.Id)
	err = channel.UpdateAbilities(nil)
	return err
}

func (channel *Channel) UpdateResponseTime(responseTime int64) {
	err := dbstore.DB.Model(channel).Select("response_time", "test_time").Updates(Channel{
		TestTime:     common.GetTimestamp(),
		ResponseTime: int(responseTime),
	}).Error
	if err != nil {
		common.SysLog(fmt.Sprintf("failed to update response time: channel_id=%d, error=%v", channel.Id, err))
	}
}

func (channel *Channel) UpdateBalance(balance float64) {
	err := dbstore.DB.Model(channel).Select("balance_updated_time", "balance").Updates(Channel{
		BalanceUpdatedTime: common.GetTimestamp(),
		Balance:            balance,
	}).Error
	if err != nil {
		common.SysLog(fmt.Sprintf("failed to update balance: channel_id=%d, error=%v", channel.Id, err))
	}
}

func (channel *Channel) Delete() error {
	var err error
	err = dbstore.DB.Delete(channel).Error
	if err != nil {
		return err
	}
	err = channel.DeleteAbilities()
	return err
}

var channelStatusLock sync.Mutex

// channelPollingLocks stores locks for each channel.id to ensure thread-safe polling
var channelPollingLocks sync.Map

// GetChannelPollingLock returns or creates a mutex for the given channel ID
func GetChannelPollingLock(channelId int) *sync.Mutex {
	if lock, exists := channelPollingLocks.Load(channelId); exists {
		return lock.(*sync.Mutex)
	}
	// Create new lock for this channel
	newLock := &sync.Mutex{}
	actual, _ := channelPollingLocks.LoadOrStore(channelId, newLock)
	return actual.(*sync.Mutex)
}

// CleanupChannelPollingLocks removes locks for channels that no longer exist
// This is optional and can be called periodically to prevent memory leaks
func CleanupChannelPollingLocks() {
	var activeChannelIds []int
	dbstore.DB.Model(&Channel{}).Pluck("id", &activeChannelIds)

	activeChannelSet := make(map[int]bool)
	for _, id := range activeChannelIds {
		activeChannelSet[id] = true
	}

	channelPollingLocks.Range(func(key, value interface{}) bool {
		channelId := key.(int)
		if !activeChannelSet[channelId] {
			channelPollingLocks.Delete(channelId)
		}
		return true
	})
}

func handlerMultiKeyUpdate(channel *Channel, usingKey string, status int, reason string) {
	keys := channel.GetKeys()
	if len(keys) == 0 {
		channel.Status = status
	} else {
		keyIndex := -1
		for i, key := range keys {
			if key == usingKey {
				keyIndex = i
				break
			}
		}
		if keyIndex < 0 {
			return
		}
		if channel.ChannelInfo.MultiKeyStatusList == nil {
			channel.ChannelInfo.MultiKeyStatusList = make(map[int]int)
		}
		if status == common.ChannelStatusEnabled {
			delete(channel.ChannelInfo.MultiKeyStatusList, keyIndex)
			if channel.ChannelInfo.MultiKeyDisabledReason != nil {
				delete(channel.ChannelInfo.MultiKeyDisabledReason, keyIndex)
			}
			if channel.ChannelInfo.MultiKeyDisabledTime != nil {
				delete(channel.ChannelInfo.MultiKeyDisabledTime, keyIndex)
			}
		} else {
			channel.ChannelInfo.MultiKeyStatusList[keyIndex] = status
			if channel.ChannelInfo.MultiKeyDisabledReason == nil {
				channel.ChannelInfo.MultiKeyDisabledReason = make(map[int]string)
			}
			if channel.ChannelInfo.MultiKeyDisabledTime == nil {
				channel.ChannelInfo.MultiKeyDisabledTime = make(map[int]int64)
			}
			channel.ChannelInfo.MultiKeyDisabledReason[keyIndex] = reason
			channel.ChannelInfo.MultiKeyDisabledTime[keyIndex] = common.GetTimestamp()
		}

		enabledKeys := 0
		for i := range keys {
			if channel.ChannelInfo.MultiKeyStatusList[i] == common.ChannelStatusEnabled || channel.ChannelInfo.MultiKeyStatusList[i] == 0 {
				enabledKeys++
			}
		}
		if enabledKeys == 0 {
			channel.Status = common.ChannelStatusAutoDisabled
			info := channel.GetOtherInfo()
			info["status_reason"] = "All keys are disabled"
			info["status_time"] = common.GetTimestamp()
			channel.SetOtherInfo(info)
		} else if status == common.ChannelStatusEnabled {
			channel.Status = common.ChannelStatusEnabled
		}
	}
}

func UpdateChannelStatus(channelId int, usingKey string, status int, reason string) bool {
	if common.MemoryCacheEnabled {
		channelStatusLock.Lock()
		defer channelStatusLock.Unlock()

		channelCache, _ := CacheGetChannel(channelId)
		if channelCache == nil {
			return false
		}
		if channelCache.ChannelInfo.IsMultiKey {
			// Use per-channel lock to prevent concurrent map read/write with GetNextEnabledKey
			pollingLock := GetChannelPollingLock(channelId)
			pollingLock.Lock()
			beforeStatus := channelCache.Status
			// 如果是多Key模式，更新缓存中的状态
			handlerMultiKeyUpdate(channelCache, usingKey, status, reason)
			pollingLock.Unlock()
			if beforeStatus != channelCache.Status {
				CacheUpdateChannelStatus(channelId, channelCache.Status)
			}
		} else {
			// 如果缓存渠道存在，且状态已是目标状态，直接返回
			if channelCache.Status == status {
				return false
			}
			CacheUpdateChannelStatus(channelId, status)
		}
	}

	shouldUpdateAbilities := false
	defer func() {
		if shouldUpdateAbilities {
			err := UpdateAbilityStatus(channelId, status == common.ChannelStatusEnabled)
			if err != nil {
				common.SysLog(fmt.Sprintf("failed to update ability status: channel_id=%d, error=%v", channelId, err))
			}
		}
	}()
	channel, err := GetChannelById(channelId, true)
	if err != nil {
		return false
	} else {
		if channel.Status == status {
			return false
		}

		if channel.ChannelInfo.IsMultiKey {
			beforeStatus := channel.Status
			// Protect map writes with the same per-channel lock used by readers
			pollingLock := GetChannelPollingLock(channelId)
			pollingLock.Lock()
			handlerMultiKeyUpdate(channel, usingKey, status, reason)
			pollingLock.Unlock()
			if beforeStatus != channel.Status {
				shouldUpdateAbilities = true
			}
		} else {
			info := channel.GetOtherInfo()
			info["status_reason"] = reason
			info["status_time"] = common.GetTimestamp()
			channel.SetOtherInfo(info)
			channel.Status = status
			shouldUpdateAbilities = true
		}
		err = channel.SaveWithoutKey()
		if err != nil {
			common.SysLog(fmt.Sprintf("failed to update channel status: channel_id=%d, status=%d, error=%v", channel.Id, status, err))
			return false
		}
	}
	return true
}

func EnableChannelByTag(tag string) error {
	err := dbstore.DB.Model(&Channel{}).Where("tag = ?", tag).Update("status", common.ChannelStatusEnabled).Error
	if err != nil {
		return err
	}
	err = UpdateAbilityStatusByTag(tag, true)
	return err
}

func DisableChannelByTag(tag string) error {
	err := dbstore.DB.Model(&Channel{}).Where("tag = ?", tag).Update("status", common.ChannelStatusManuallyDisabled).Error
	if err != nil {
		return err
	}
	err = UpdateAbilityStatusByTag(tag, false)
	return err
}

func EditChannelByTag(tag string, newTag *string, modelMapping *string, models *string, group *string, paramOverride *string, headerOverride *string) error {
	updateData := Channel{}
	shouldReCreateAbilities := false
	updatedTag := tag
	// 如果 newTag 不为空且不等于 tag，则更新 tag
	if newTag != nil && *newTag != tag {
		updateData.Tag = newTag
		updatedTag = *newTag
	}
	if modelMapping != nil && *modelMapping != "" {
		updateData.ModelMapping = modelMapping
	}
	if models != nil && *models != "" {
		shouldReCreateAbilities = true
		updateData.Models = *models
	}
	if group != nil && *group != "" {
		shouldReCreateAbilities = true
		updateData.Group = *group
	}
	if paramOverride != nil {
		updateData.ParamOverride = paramOverride
	}
	if headerOverride != nil {
		updateData.HeaderOverride = headerOverride
	}

	err := dbstore.DB.Model(&Channel{}).Where("tag = ?", tag).Updates(updateData).Error
	if err != nil {
		return err
	}
	if shouldReCreateAbilities {
		channels, err := GetChannelsByTag(updatedTag, false, false)
		if err == nil {
			for _, channel := range channels {
				err = channel.UpdateAbilities(nil)
				if err != nil {
					common.SysLog(fmt.Sprintf("failed to update abilities: channel_id=%d, tag=%s, error=%v", channel.Id, channel.GetTag(), err))
				}
			}
		}
	} else {
		err := UpdateAbilityByTag(tag, newTag)
		if err != nil {
			return err
		}
	}
	return nil
}

func UpdateChannelUsedQuota(id int, quota int) {
	if common.BatchUpdateEnabled {
		dbstore.AddNewRecord(dbstore.BatchUpdateTypeChannelUsedQuota, id, quota)
		return
	}
	updateChannelUsedQuota(id, quota)
}

func updateChannelUsedQuota(id int, quota int) {
	err := dbstore.DB.Model(&Channel{}).Where("id = ?", id).Update("used_quota", gorm.Expr("used_quota + ?", quota)).Error
	if err != nil {
		common.SysLog(fmt.Sprintf("failed to update channel used quota: channel_id=%d, delta_quota=%d, error=%v", id, quota, err))
	}
}

func DeleteChannelByStatus(status int64) (int64, error) {
	result := dbstore.DB.Where("status = ?", status).Delete(&Channel{})
	return result.RowsAffected, result.Error
}

func DeleteDisabledChannel() (int64, error) {
	result := dbstore.DB.Where("status = ? or status = ?", common.ChannelStatusAutoDisabled, common.ChannelStatusManuallyDisabled).Delete(&Channel{})
	return result.RowsAffected, result.Error
}

func GetPaginatedTags(offset int, limit int) ([]*string, error) {
	return GetPaginatedChannelTags(dbstore.DB.Model(&Channel{}), offset, limit)
}

func GetPaginatedChannelTags(query *gorm.DB, offset int, limit int) ([]*string, error) {
	var tags []*string
	query = query.Select("DISTINCT tag").Where("tag is not null AND tag != ''")
	if infradb.UsingPostgreSQL {
		query = query.Order("tag asc")
	} else {
		query = query.Order("LOWER(tag) asc")
	}
	err := query.Offset(offset).Limit(limit).Find(&tags).Error
	return tags, err
}

func SearchTags(keyword string, group string, model string, idSort bool, idFilter int, nameFilter string, tagFilter string) ([]*string, error) {
	var tags []*string
	modelsCol := "`models`"

	// 如果是 PostgreSQL，使用双引号
	if infradb.UsingPostgreSQL {
		modelsCol = `"models"`
	}

	baseURLCol := "`base_url`"
	// 如果是 PostgreSQL，使用双引号
	if infradb.UsingPostgreSQL {
		baseURLCol = `"base_url"`
	}

	order := "LOWER(name) asc, id asc"
	if idSort {
		order = "id desc"
	}

	// 构造基础查询
	baseQuery := dbstore.DB.Model(&Channel{}).Omit("key")

	whereClause := "(id = ? OR name LIKE ? OR " + dbstore.CommonKeyCol + " = ? OR " + baseURLCol + " LIKE ?) AND " + modelsCol + " LIKE ?"
	args := []interface{}{common.String2Int(keyword), "%" + keyword + "%", keyword, "%" + keyword + "%", "%" + model + "%"}
	baseQuery = ApplyChannelGroupFilter(baseQuery.Where(whereClause, args...), group)

	// 独立过滤条件（AND 叠加），与 SearchChannels 保持一致
	if idFilter > 0 {
		baseQuery = baseQuery.Where("id = ?", idFilter)
	}
	if nameFilter != "" {
		baseQuery = baseQuery.Where("name LIKE ?", "%"+nameFilter+"%")
	}
	if tagFilter != "" {
		baseQuery = baseQuery.Where("tag LIKE ?", "%"+tagFilter+"%")
	}

	subQuery := baseQuery.
		Select("tag").
		Where("tag != ''").
		Order(order)

	err := dbstore.DB.Table("(?) as sub", subQuery).
		Select("DISTINCT tag").
		Find(&tags).Error

	if err != nil {
		return nil, err
	}

	return tags, nil
}

func (channel *Channel) ValidateSettings() error {
	channelParams := &shared.ChannelSettings{}
	if channel.Setting != nil && *channel.Setting != "" {
		err := jsonx.Unmarshal([]byte(*channel.Setting), channelParams)
		if err != nil {
			return err
		}

		// 拒绝已移除的字段，避免旧客户端静默写入导致"配置存在但运行时不生效"
		var rawMap map[string]json.RawMessage
		if err := jsonx.Unmarshal([]byte(*channel.Setting), &rawMap); err == nil {
			if _, ok := rawMap["system_prompt"]; ok {
				return fmt.Errorf("channel setting \"system_prompt\" 已移除，请删除该字段")
			}
			if _, ok := rawMap["system_prompt_override"]; ok {
				return fmt.Errorf("channel setting \"system_prompt_override\" 已移除，请删除该字段")
			}
		}
	}

	if channel.OtherSettings != "" {
		otherSettings := &shared.ChannelOtherSettings{}
		if err := jsonx.UnmarshalJsonStr(channel.OtherSettings, otherSettings); err != nil {
			return err
		}
		if _, ok := otherSettings.ParseImageAutoConvertToURLMode(); !ok {
			return fmt.Errorf("invalid image_auto_convert_to_url_mode")
		}

		var rawMap map[string]json.RawMessage
		if err := jsonx.Unmarshal([]byte(channel.OtherSettings), &rawMap); err == nil {
			if _, ok := rawMap["image_auto_convert_to_url"]; ok {
				return fmt.Errorf("channel other setting \"image_auto_convert_to_url\" 已移除，请删除该字段")
			}
			if rawMode, ok := rawMap["image_auto_convert_to_url_mode"]; ok {
				var mode string
				if err := jsonx.Unmarshal(rawMode, &mode); err != nil {
					return fmt.Errorf("channel other setting \"image_auto_convert_to_url_mode\" 必须是字符串")
				}
				if strings.EqualFold(strings.TrimSpace(mode), "third_party_model") {
					return fmt.Errorf("channel other setting \"image_auto_convert_to_url_mode=third_party_model\" 已移除，请改用 \"mcp\"")
				}
			}
		}

		if otherSettings.OpenRouterRouting != nil {
			if channel.Type != constant.ChannelTypeOpenRouter {
				return fmt.Errorf("channel other setting \"openrouter_routing\" 仅支持 OpenRouter 渠道")
			}
			if err := otherSettings.OpenRouterRouting.Validate(); err != nil {
				return err
			}
			// 拒绝 openrouter_routing 内的未知字段：拼写错误的配置若被静默
			// 丢弃，会出现"保存成功但运行时永不生效"的隐形失效。
			var routingRaw map[string]json.RawMessage
			if err := jsonx.Unmarshal(rawMap["openrouter_routing"], &routingRaw); err != nil {
				return fmt.Errorf("channel other setting \"openrouter_routing\" 必须是对象: %w", err)
			}
			for key := range routingRaw {
				if !shared.KnownOpenRouterRoutingKeys[key] {
					return fmt.Errorf("channel other setting \"openrouter_routing.%s\" 不是有效字段，请检查拼写", key)
				}
			}
			if sortRaw, ok := routingRaw["sort"]; ok {
				var sortRawMap map[string]json.RawMessage
				if err := jsonx.Unmarshal(sortRaw, &sortRawMap); err == nil {
					for key := range sortRawMap {
						if key != "by" && key != "partition" {
							return fmt.Errorf("channel other setting \"openrouter_routing.sort.%s\" 不是有效字段，请检查拼写", key)
						}
					}
				}
			}
		}
	}

	// 校验 param_override / header_override：非空时必须是合法 JSON，避免在数据库边界报 SQLSTATE 22P02
	if err := validateOptionalJSONField(channel.ParamOverride, "param_override"); err != nil {
		return err
	}
	if err := validateOptionalJSONField(channel.HeaderOverride, "header_override"); err != nil {
		return err
	}
	// 规整化：空白指针统一为 nil，避免把空字符串写入可能的 JSON 列
	channel.ParamOverride = normalizeOptionalJSONPointer(channel.ParamOverride)
	channel.HeaderOverride = normalizeOptionalJSONPointer(channel.HeaderOverride)
	channel.Setting = normalizeOptionalJSONPointer(channel.Setting)
	return nil
}

// validateOptionalJSONField 校验可选 JSON 字符串指针字段：nil 或空白视为未设置，非空必须是合法 JSON。
func validateOptionalJSONField(value *string, fieldName string) error {
	if value == nil {
		return nil
	}
	trimmed := strings.TrimSpace(*value)
	if trimmed == "" {
		return nil
	}
	var raw json.RawMessage
	if err := jsonx.Unmarshal([]byte(trimmed), &raw); err != nil {
		return fmt.Errorf("channel %s 必须是合法的 JSON：%v", fieldName, err)
	}
	return nil
}

// normalizeOptionalJSONPointer 把空白/纯空白的指针字段归一化为 nil，
// 非空白则去除首尾空白，避免向数据库写入可能被解析为 JSON 的空字符串。
func normalizeOptionalJSONPointer(value *string) *string {
	if value == nil {
		return nil
	}
	trimmed := strings.TrimSpace(*value)
	if trimmed == "" {
		return nil
	}
	return &trimmed
}

func (channel *Channel) GetSetting() shared.ChannelSettings {
	setting := shared.ChannelSettings{}
	if channel.Setting != nil && *channel.Setting != "" {
		raw := []byte(*channel.Setting)
		err := jsonx.Unmarshal(raw, &setting)
		if err != nil {
			common.SysLog(fmt.Sprintf("failed to unmarshal setting: channel_id=%d, error=%v", channel.Id, err))
			channel.Setting = nil // 清空设置以避免后续错误
			_ = channel.Save()    // 保存修改
			setting.PassThroughHeadersEnabled = true
			setting.OpenAIWireAPI = shared.OpenAIWireAPIBoth
			return setting
		}

		// Backward-compatible defaulting:
		// - Existing channels may have a JSON setting without "pass_through_headers_enabled".
		// - Default is enabled (true) unless explicitly set to false.
		var rawMap map[string]json.RawMessage
		if err := jsonx.Unmarshal(raw, &rawMap); err == nil {
			if _, ok := rawMap["pass_through_headers_enabled"]; !ok {
				setting.PassThroughHeadersEnabled = true
			}
			if _, ok := rawMap["openai_wire_api"]; !ok {
				setting.OpenAIWireAPI = shared.OpenAIWireAPIBoth
			}
		}
	} else {
		setting.PassThroughHeadersEnabled = true
		setting.OpenAIWireAPI = shared.OpenAIWireAPIBoth
	}
	return setting
}

func (channel *Channel) SetSetting(setting shared.ChannelSettings) {
	settingBytes, err := jsonx.Marshal(setting)
	if err != nil {
		common.SysLog(fmt.Sprintf("failed to marshal setting: channel_id=%d, error=%v", channel.Id, err))
		return
	}
	channel.Setting = common.GetPointer[string](string(settingBytes))
}

func (channel *Channel) GetOtherSettings() shared.ChannelOtherSettings {
	setting := shared.ChannelOtherSettings{}
	if channel.OtherSettings != "" {
		err := jsonx.UnmarshalJsonStr(channel.OtherSettings, &setting)
		if err != nil {
			common.SysLog(fmt.Sprintf("failed to unmarshal setting: channel_id=%d, error=%v", channel.Id, err))
			channel.OtherSettings = "{}" // 清空设置以避免后续错误
			_ = channel.Save()           // 保存修改
		}
	}
	return setting
}

func (channel *Channel) SetOtherSettings(setting shared.ChannelOtherSettings) {
	settingBytes, err := jsonx.Marshal(setting)
	if err != nil {
		common.SysLog(fmt.Sprintf("failed to marshal setting: channel_id=%d, error=%v", channel.Id, err))
		return
	}
	channel.OtherSettings = string(settingBytes)
}

func (channel *Channel) GetParamOverride() map[string]interface{} {
	paramOverride := make(map[string]interface{})
	if channel.ParamOverride != nil && *channel.ParamOverride != "" {
		err := jsonx.Unmarshal([]byte(*channel.ParamOverride), &paramOverride)
		if err != nil {
			common.SysLog(fmt.Sprintf("failed to unmarshal param override: channel_id=%d, error=%v", channel.Id, err))
		}
	}
	return paramOverride
}

func (channel *Channel) GetHeaderOverride() map[string]interface{} {
	headerOverride := make(map[string]interface{})
	if channel.HeaderOverride != nil && *channel.HeaderOverride != "" {
		err := jsonx.Unmarshal([]byte(*channel.HeaderOverride), &headerOverride)
		if err != nil {
			common.SysLog(fmt.Sprintf("failed to unmarshal header override: channel_id=%d, error=%v", channel.Id, err))
		}
	}
	return headerOverride
}

func GetChannelsByIds(ids []int) ([]*Channel, error) {
	var channels []*Channel
	err := dbstore.DB.Where("id in (?)", ids).Find(&channels).Error
	return channels, err
}

func BatchSetChannelTag(ids []int, tag *string) error {
	// 开启事务
	tx := dbstore.DB.Begin()
	if tx.Error != nil {
		return tx.Error
	}

	// 更新标签
	err := tx.Model(&Channel{}).Where("id in (?)", ids).Update("tag", tag).Error
	if err != nil {
		tx.Rollback()
		return err
	}

	// update ability status
	channels, err := GetChannelsByIds(ids)
	if err != nil {
		tx.Rollback()
		return err
	}

	for _, channel := range channels {
		err = channel.UpdateAbilities(tx)
		if err != nil {
			tx.Rollback()
			return err
		}
	}

	// 提交事务
	return tx.Commit().Error
}

// CountAllChannels returns total channels in DB
func CountAllChannels() (int64, error) {
	var total int64
	err := dbstore.DB.Model(&Channel{}).Count(&total).Error
	return total, err
}

// CountAllTags returns number of non-empty distinct tags
func CountAllTags() (int64, error) {
	return CountChannelTags(dbstore.DB.Model(&Channel{}))
}

func CountChannelTags(query *gorm.DB) (int64, error) {
	var total int64
	err := query.Where("tag is not null AND tag != ''").Distinct("tag").Count(&total).Error
	return total, err
}

// Get channels of specified type with pagination
func GetChannelsByType(startIdx int, num int, idSort bool, channelType int) ([]*Channel, error) {
	var channels []*Channel
	order := "LOWER(name) asc, id asc"
	if idSort {
		order = "id desc"
	}
	err := dbstore.DB.Where("type = ?", channelType).Order(order).Limit(num).Offset(startIdx).Omit("key").Find(&channels).Error
	return channels, err
}

// Count channels of specific type
func CountChannelsByType(channelType int) (int64, error) {
	var count int64
	err := dbstore.DB.Model(&Channel{}).Where("type = ?", channelType).Count(&count).Error
	return count, err
}

// Return map[type]count for all channels
func CountChannelsGroupByType() (map[int64]int64, error) {
	type result struct {
		Type  int64 `gorm:"column:type"`
		Count int64 `gorm:"column:count"`
	}
	var results []result
	err := dbstore.DB.Model(&Channel{}).Select("type, count(*) as count").Group("type").Find(&results).Error
	if err != nil {
		return nil, err
	}
	counts := make(map[int64]int64)
	for _, r := range results {
		counts[r.Type] = r.Count
	}
	return counts, nil
}

// init 把渠道已用额度的批量落库函数注册到 dbstore 的批量更新器。
// BatchUpdateTypeChannelUsedQuota 的批量条目只会由本包写入。
func init() {
	dbstore.RegisterBatchFlushers(dbstore.BatchFlushers{
		ChannelUsedQuota: updateChannelUsedQuota,
	})
}
