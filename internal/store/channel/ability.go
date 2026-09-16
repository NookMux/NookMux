package channelstore

import (
	"errors"
	"fmt"
	"github.com/NookMux/NookMux/internal/common"
	infradb "github.com/NookMux/NookMux/internal/infra/db"
	relayconstant "github.com/NookMux/NookMux/internal/relay/constant"
	"github.com/NookMux/NookMux/internal/store/db"
	"github.com/samber/lo"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"strings"
	"sync"
)

type Ability struct {
	Group     string  `json:"group" gorm:"type:varchar(64);primaryKey;autoIncrement:false"`
	Model     string  `json:"model" gorm:"type:varchar(255);primaryKey;autoIncrement:false"`
	ChannelId int     `json:"channel_id" gorm:"primaryKey;autoIncrement:false;index"`
	Enabled   bool    `json:"enabled"`
	Priority  *int64  `json:"priority" gorm:"bigint;default:0;index"`
	Weight    uint    `json:"weight" gorm:"default:0;index"`
	Tag       *string `json:"tag" gorm:"index"`
}

type AbilityWithChannel struct {
	Ability
	ChannelType int `json:"channel_type"`
}

func GetAllEnableAbilityWithChannels() ([]AbilityWithChannel, error) {
	var abilities []AbilityWithChannel
	err := dbstore.DB.Table("abilities").
		Select("abilities.*, channels.type as channel_type").
		Joins("left join channels on abilities.channel_id = channels.id").
		Where("abilities.enabled = ?", true).
		Scan(&abilities).Error
	return abilities, err
}

func GetGroupEnabledModels(group string) []string {
	var models []string
	// Find distinct models
	dbstore.DB.Table("abilities").Where(dbstore.CommonGroupCol+" = ? and enabled = ?", group, true).Distinct("model").Pluck("model", &models)
	return models
}

func GetEnabledModels() []string {
	var models []string
	// Find distinct models
	dbstore.DB.Table("abilities").Where("enabled = ?", true).Distinct("model").Pluck("model", &models)
	return models
}

func GetAllEnableAbilities() []Ability {
	var abilities []Ability
	dbstore.DB.Find(&abilities, "enabled = ?", true)
	return abilities
}

func getPriority(group string, model string, retry int) (int, error) {

	var priorities []int
	err := dbstore.DB.Model(&Ability{}).
		Select("DISTINCT(priority)").
		Where(dbstore.CommonGroupCol+" = ? and model = ? and enabled = ?", group, model, true).
		Order("priority DESC").              // 按优先级降序排序
		Pluck("priority", &priorities).Error // Pluck用于将查询的结果直接扫描到一个切片中

	if err != nil {
		// 处理错误
		return 0, err
	}

	if len(priorities) == 0 {
		// 如果没有查询到优先级，则返回错误
		return 0, errors.New("数据库一致性被破坏")
	}

	// 确定要使用的优先级
	var priorityToUse int
	if retry >= len(priorities) {
		// 如果重试次数大于优先级数，则使用最小的优先级
		priorityToUse = priorities[len(priorities)-1]
	} else {
		priorityToUse = priorities[retry]
	}
	return priorityToUse, nil
}

// getPriorityCountDB returns the number of distinct priority levels for a group/model pair from DB.
func getPriorityCountDB(group string, model string) int {
	var count int64
	dbstore.DB.Model(&Ability{}).
		Where(dbstore.CommonGroupCol+" = ? and model = ? and enabled = ?", group, model, true).
		Distinct("priority").
		Count(&count)
	return int(count)
}

func getChannelQuery(group string, model string, priorityIndex int, excludeChannelId int) (*gorm.DB, error) {
	maxPrioritySubQuery := dbstore.DB.Model(&Ability{}).Select("MAX(priority)").Where(dbstore.CommonGroupCol+" = ? and model = ? and enabled = ?", group, model, true)
	channelQuery := dbstore.DB.Where(dbstore.CommonGroupCol+" = ? and model = ? and enabled = ? and priority = (?)", group, model, true, maxPrioritySubQuery)
	if priorityIndex != 0 {
		priority, err := getPriority(group, model, priorityIndex)
		if err != nil {
			return nil, err
		}
		channelQuery = dbstore.DB.Where(dbstore.CommonGroupCol+" = ? and model = ? and enabled = ? and priority = ?", group, model, true, priority)
	}
	if excludeChannelId > 0 {
		channelQuery = channelQuery.Where("channel_id != ?", excludeChannelId)
	}
	return channelQuery, nil
}

func GetChannel(group string, model string, priorityIndex int, preferredAPIType int, excludeChannelId int) (*Channel, error) {
	return GetChannelWithRelayFormat(group, model, priorityIndex, preferredAPIType, "", excludeChannelId)
}

func chooseChannelIDFromAbilities(abilities []Ability, preferredAPIType int, relayFormat relayconstant.RelayFormat) int {
	if len(abilities) == 0 {
		return 0
	}
	abilities = preferAbilitiesByRequestFormat(abilities, preferredAPIType, relayFormat)
	weightSum := uint(0)
	for _, ability := range abilities {
		weightSum += ability.Weight + 10
	}
	weight := common.GetRandomInt(int(weightSum))
	for _, ability := range abilities {
		weight -= int(ability.Weight) + 10
		if weight <= 0 {
			return ability.ChannelId
		}
	}
	return 0
}

func GetChannelWithRelayFormat(group string, model string, priorityIndex int, preferredAPIType int, relayFormat relayconstant.RelayFormat, excludeChannelId int) (*Channel, error) {
	return getChannelWithRelayFormat(group, model, priorityIndex, preferredAPIType, relayFormat, excludeChannelId, false)
}

func getChannelWithRelayFormat(group string, model string, priorityIndex int, preferredAPIType int, relayFormat relayconstant.RelayFormat, excludeChannelId int, allowExcludedFallback bool) (*Channel, error) {
	var abilities []Ability

	var err error
	channelQuery, err := getChannelQuery(group, model, priorityIndex, excludeChannelId)
	if err != nil {
		return nil, err
	}
	if infradb.UsingSQLite || infradb.UsingPostgreSQL {
		err = channelQuery.Order("weight DESC").Find(&abilities).Error
	} else {
		err = channelQuery.Order("weight DESC").Find(&abilities).Error
	}
	if err != nil {
		return nil, err
	}
	channel := Channel{Id: chooseChannelIDFromAbilities(abilities, preferredAPIType, relayFormat)}
	if channel.Id == 0 {
		// If exclusion left no candidates at current priority, fall through to next lower priority
		// 如果排除后在当前优先级无候选渠道，降级到下一个优先级
		if excludeChannelId > 0 {
			numPriorities := getPriorityCountDB(group, model)
			if priorityIndex+1 < numPriorities {
				fallbackQuery, fallbackErr := getChannelQuery(group, model, priorityIndex+1, 0)
				if fallbackErr != nil {
					return nil, nil
				}
				if fallbackErr = fallbackQuery.Order("weight DESC").Find(&abilities).Error; fallbackErr != nil {
					return nil, nil
				}
				channel.Id = chooseChannelIDFromAbilities(abilities, preferredAPIType, relayFormat)
			}
		}
		// No alternative exists: retry the original priority without exclusion so
		// one-channel and final-priority setups still honor RetryTimes.
		if channel.Id == 0 && excludeChannelId > 0 && allowExcludedFallback {
			fallbackQuery, fallbackErr := getChannelQuery(group, model, priorityIndex, 0)
			if fallbackErr != nil {
				return nil, fallbackErr
			}
			abilities = nil
			if fallbackErr = fallbackQuery.Order("weight DESC").Find(&abilities).Error; fallbackErr != nil {
				return nil, fallbackErr
			}
			channel.Id = chooseChannelIDFromAbilities(abilities, preferredAPIType, relayFormat)
		}
		if channel.Id == 0 {
			return nil, nil
		}
	}
	err = dbstore.DB.First(&channel, "id = ?", channel.Id).Error
	return &channel, err
}

func (channel *Channel) AddAbilities(tx *gorm.DB) error {
	models_ := strings.Split(channel.Models, ",")
	groups_ := strings.Split(channel.Group, ",")
	abilitySet := make(map[string]struct{})
	abilities := make([]Ability, 0, len(models_))
	for _, model := range models_ {
		for _, group := range groups_ {
			key := group + "|" + model
			if _, exists := abilitySet[key]; exists {
				continue
			}
			abilitySet[key] = struct{}{}
			ability := Ability{
				Group:     group,
				Model:     model,
				ChannelId: channel.Id,
				Enabled:   channel.Status == common.ChannelStatusEnabled,
				Priority:  channel.Priority,
				Weight:    uint(channel.GetWeight()),
				Tag:       channel.Tag,
			}
			abilities = append(abilities, ability)
		}
	}
	if len(abilities) == 0 {
		return nil
	}
	// choose DB or provided tx
	useDB := dbstore.DB
	if tx != nil {
		useDB = tx
	}
	for _, chunk := range lo.Chunk(abilities, 50) {
		err := useDB.Clauses(clause.OnConflict{DoNothing: true}).Create(&chunk).Error
		if err != nil {
			return err
		}
	}
	return nil
}

func (channel *Channel) DeleteAbilities() error {
	return dbstore.DB.Where("channel_id = ?", channel.Id).Delete(&Ability{}).Error
}

// UpdateAbilities updates abilities of this channel.
// Make sure the channel is completed before calling this function.
func (channel *Channel) UpdateAbilities(tx *gorm.DB) error {
	isNewTx := false
	// 如果没有传入事务，创建新的事务
	if tx == nil {
		tx = dbstore.DB.Begin()
		if tx.Error != nil {
			return tx.Error
		}
		isNewTx = true
		defer func() {
			if r := recover(); r != nil {
				tx.Rollback()
			}
		}()
	}

	// First delete all abilities of this channel
	err := tx.Where("channel_id = ?", channel.Id).Delete(&Ability{}).Error
	if err != nil {
		if isNewTx {
			tx.Rollback()
		}
		return err
	}

	// Then add new abilities
	models_ := strings.Split(channel.Models, ",")
	groups_ := strings.Split(channel.Group, ",")
	abilitySet := make(map[string]struct{})
	abilities := make([]Ability, 0, len(models_))
	for _, model := range models_ {
		for _, group := range groups_ {
			key := group + "|" + model
			if _, exists := abilitySet[key]; exists {
				continue
			}
			abilitySet[key] = struct{}{}
			ability := Ability{
				Group:     group,
				Model:     model,
				ChannelId: channel.Id,
				Enabled:   channel.Status == common.ChannelStatusEnabled,
				Priority:  channel.Priority,
				Weight:    uint(channel.GetWeight()),
				Tag:       channel.Tag,
			}
			abilities = append(abilities, ability)
		}
	}

	if len(abilities) > 0 {
		for _, chunk := range lo.Chunk(abilities, 50) {
			err = tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&chunk).Error
			if err != nil {
				if isNewTx {
					tx.Rollback()
				}
				return err
			}
		}
	}

	// 如果是新创建的事务，需要提交
	if isNewTx {
		return tx.Commit().Error
	}

	return nil
}

func UpdateAbilityStatus(channelId int, status bool) error {
	return dbstore.DB.Model(&Ability{}).Where("channel_id = ?", channelId).Select("enabled").Update("enabled", status).Error
}

func UpdateAbilityStatusByTag(tag string, status bool) error {
	return dbstore.DB.Model(&Ability{}).Where("tag = ?", tag).Select("enabled").Update("enabled", status).Error
}

func UpdateAbilityByTag(tag string, newTag *string) error {
	ability := Ability{}
	if newTag != nil {
		ability.Tag = newTag
	}
	return dbstore.DB.Model(&Ability{}).Where("tag = ?", tag).Updates(ability).Error
}

var fixLock = sync.Mutex{}

func FixAbility() (int, int, error) {
	lock := fixLock.TryLock()
	if !lock {
		return 0, 0, errors.New("已经有一个修复任务在运行中，请稍后再试")
	}
	defer fixLock.Unlock()

	// truncate abilities table
	if infradb.UsingSQLite {
		err := dbstore.DB.Exec("DELETE FROM abilities").Error
		if err != nil {
			common.SysLog(fmt.Sprintf("Delete abilities failed: %s", err.Error()))
			return 0, 0, err
		}
	} else {
		err := dbstore.DB.Exec("TRUNCATE TABLE abilities").Error
		if err != nil {
			common.SysLog(fmt.Sprintf("Truncate abilities failed: %s", err.Error()))
			return 0, 0, err
		}
	}
	var channels []*Channel
	// Find all channels
	err := dbstore.DB.Model(&Channel{}).Find(&channels).Error
	if err != nil {
		return 0, 0, err
	}
	if len(channels) == 0 {
		return 0, 0, nil
	}
	successCount := 0
	failCount := 0
	for _, chunk := range lo.Chunk(channels, 50) {
		ids := lo.Map(chunk, func(c *Channel, _ int) int { return c.Id })
		// Delete all abilities of this channel
		err = dbstore.DB.Where("channel_id IN ?", ids).Delete(&Ability{}).Error
		if err != nil {
			common.SysLog(fmt.Sprintf("Delete abilities failed: %s", err.Error()))
			failCount += len(chunk)
			continue
		}
		// Then add new abilities
		for _, channel := range chunk {
			err = channel.AddAbilities(nil)
			if err != nil {
				common.SysLog(fmt.Sprintf("Add abilities for channel %d failed: %s", channel.Id, err.Error()))
				failCount++
			} else {
				successCount++
			}
		}
	}
	InitChannelCache()
	return successCount, failCount, nil
}
