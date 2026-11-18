package channelstore

import (
	"errors"
	"github.com/NookMux/NookMux/internal/common"
	"github.com/NookMux/NookMux/internal/config/ratio"
	"github.com/NookMux/NookMux/internal/domain/channel/constant"
	"github.com/NookMux/NookMux/internal/i18n"
	relayconstant "github.com/NookMux/NookMux/internal/relay/constant"
	"github.com/NookMux/NookMux/internal/store/db"
	"math/rand"
	"sort"
	"strings"
	"sync"
	"time"
)

var group2model2channels map[string]map[string][]int // enabled channel
var channelsIDM map[int]*Channel                     // all channels include disabled
var channelSyncLock sync.RWMutex

func InitChannelCache() {
	if !common.MemoryCacheEnabled {
		return
	}
	newChannelId2channel := make(map[int]*Channel)
	var channels []*Channel
	dbstore.DB.Find(&channels)
	for _, channel := range channels {
		newChannelId2channel[channel.Id] = channel
	}
	var abilities []*Ability
	dbstore.DB.Find(&abilities)
	groups := make(map[string]bool)
	for _, ability := range abilities {
		groups[ability.Group] = true
	}
	newGroup2model2channels := make(map[string]map[string][]int)
	for group := range groups {
		newGroup2model2channels[group] = make(map[string][]int)
	}
	for _, channel := range channels {
		if channel.Status != common.ChannelStatusEnabled {
			continue // skip disabled channels
		}
		groups := strings.Split(channel.Group, ",")
		for _, group := range groups {
			models := strings.Split(channel.Models, ",")
			for _, model := range models {
				if _, ok := newGroup2model2channels[group][model]; !ok {
					newGroup2model2channels[group][model] = make([]int, 0)
				}
				newGroup2model2channels[group][model] = append(newGroup2model2channels[group][model], channel.Id)
			}
		}
	}

	// sort by priority
	for group, model2channels := range newGroup2model2channels {
		for model, channels := range model2channels {
			sort.Slice(channels, func(i, j int) bool {
				return newChannelId2channel[channels[i]].GetPriority() > newChannelId2channel[channels[j]].GetPriority()
			})
			newGroup2model2channels[group][model] = channels
		}
	}

	channelSyncLock.Lock()
	group2model2channels = newGroup2model2channels
	//channelsIDM = newChannelId2channel
	for i, channel := range newChannelId2channel {
		if channel.ChannelInfo.IsMultiKey {
			channel.Keys = channel.GetKeys()
			if channel.ChannelInfo.MultiKeyMode == constant.MultiKeyModePolling {
				if oldChannel, ok := channelsIDM[i]; ok {
					// 存在旧的渠道，如果是多key且轮询，保留轮询索引信息
					if oldChannel.ChannelInfo.IsMultiKey && oldChannel.ChannelInfo.MultiKeyMode == constant.MultiKeyModePolling {
						channel.ChannelInfo.MultiKeyPollingIndex = oldChannel.ChannelInfo.MultiKeyPollingIndex
					}
				}
			}
		}
	}
	channelsIDM = newChannelId2channel
	channelSyncLock.Unlock()
	common.SysLog("channels synced from database")
}

func SyncChannelCache(frequency int) {
	for {
		time.Sleep(time.Duration(frequency) * time.Second)
		common.SysLog("syncing channels from database")
		InitChannelCache()
	}
}

func GetRandomSatisfiedChannel(group string, model string, priorityIndex int, preferredAPIType int, excludeChannelId int) (*Channel, error) {
	return getRandomSatisfiedChannelWithRelayFormat(group, model, priorityIndex, preferredAPIType, "", excludeChannelId, false)
}

// GetRandomSatisfiedChannelForRetry is the non-wire-specific retry selector.
func GetRandomSatisfiedChannelForRetry(group string, model string, priorityIndex int, preferredAPIType int, excludeChannelId int, allowExcludedFallback bool) (*Channel, error) {
	return getRandomSatisfiedChannelWithRelayFormat(group, model, priorityIndex, preferredAPIType, "", excludeChannelId, allowExcludedFallback)
}

func GetRandomSatisfiedChannelWithRelayFormat(group string, model string, priorityIndex int, preferredAPIType int, relayFormat relayconstant.RelayFormat, excludeChannelId int) (*Channel, error) {
	return getRandomSatisfiedChannelWithRelayFormat(group, model, priorityIndex, preferredAPIType, relayFormat, excludeChannelId, false)
}

// GetRandomSatisfiedChannelWithRelayFormatForRetry prefers a different channel
// but can reuse the excluded channel when the caller explicitly allows it and
// no alternative remains.
func GetRandomSatisfiedChannelWithRelayFormatForRetry(group string, model string, priorityIndex int, preferredAPIType int, relayFormat relayconstant.RelayFormat, excludeChannelId int, allowExcludedFallback bool) (*Channel, error) {
	return getRandomSatisfiedChannelWithRelayFormat(group, model, priorityIndex, preferredAPIType, relayFormat, excludeChannelId, allowExcludedFallback)
}

func getRandomSatisfiedChannelWithRelayFormat(group string, model string, priorityIndex int, preferredAPIType int, relayFormat relayconstant.RelayFormat, excludeChannelId int, allowExcludedFallback bool) (*Channel, error) {
	// if memory cache is disabled, get channel directly from database
	if !common.MemoryCacheEnabled {
		return getChannelWithRelayFormat(group, model, priorityIndex, preferredAPIType, relayFormat, excludeChannelId, allowExcludedFallback)
	}

	channelSyncLock.RLock()
	defer channelSyncLock.RUnlock()

	// First, try to find channels with the exact model name.
	channels := group2model2channels[group][model]

	// If no channels found, try to find channels with the normalized model name.
	if len(channels) == 0 {
		normalizedModel := ratio.FormatMatchingModelName(model)
		channels = group2model2channels[group][normalizedModel]
	}

	if len(channels) == 0 {
		return nil, nil
	}

	if len(channels) == 1 {
		if channel, ok := channelsIDM[channels[0]]; ok {
			if excludeChannelId > 0 && channel.Id == excludeChannelId && !allowExcludedFallback {
				return nil, nil
			}
			return channel, nil
		}
		return nil, i18n.NewError(i18n.MsgChannelCacheConsistencyError, map[string]any{"ID": channels[0]})
	}

	uniquePriorities := make(map[int]bool)
	for _, channelId := range channels {
		if channel, ok := channelsIDM[channelId]; ok {
			uniquePriorities[int(channel.GetPriority())] = true
		} else {
			return nil, i18n.NewError(i18n.MsgChannelCacheConsistencyError, map[string]any{"ID": channelId})
		}
	}
	var sortedUniquePriorities []int
	for priority := range uniquePriorities {
		sortedUniquePriorities = append(sortedUniquePriorities, priority)
	}
	sort.Sort(sort.Reverse(sort.IntSlice(sortedUniquePriorities)))

	if priorityIndex >= len(uniquePriorities) {
		priorityIndex = len(uniquePriorities) - 1
	}
	targetPriority := int64(sortedUniquePriorities[priorityIndex])
	originalTargetPriority := targetPriority

	// get the priority for the given priority index
	var sumWeight = 0
	var targetChannels []*Channel
	for _, channelId := range channels {
		if channel, ok := channelsIDM[channelId]; ok {
			if channel.GetPriority() == targetPriority {
				// Exclude the previously failed channel for same-priority retry
				if excludeChannelId > 0 && channel.Id == excludeChannelId {
					continue
				}
				sumWeight += channel.GetWeight()
				targetChannels = append(targetChannels, channel)
			}
		} else {
			return nil, i18n.NewError(i18n.MsgChannelCacheConsistencyError, map[string]any{"ID": channelId})
		}
	}

	// If exclusion left no candidates at current priority, fall through to next lower priority
	// 如果排除后在当前优先级无候选渠道，降级到下一个优先级
	if len(targetChannels) == 0 && excludeChannelId > 0 && priorityIndex+1 < len(sortedUniquePriorities) {
		targetPriority = int64(sortedUniquePriorities[priorityIndex+1])
		for _, channelId := range channels {
			if channel, ok := channelsIDM[channelId]; ok {
				if channel.GetPriority() == targetPriority {
					sumWeight += channel.GetWeight()
					targetChannels = append(targetChannels, channel)
				}
			}
		}
	}

	// If there is no different channel at this or a lower priority, retry the
	// original eligible channel instead of ending the whole retry loop early.
	if len(targetChannels) == 0 && excludeChannelId > 0 && allowExcludedFallback {
		targetPriority = originalTargetPriority
		for _, channelId := range channels {
			if channel, ok := channelsIDM[channelId]; ok && channel.GetPriority() == targetPriority {
				sumWeight += channel.GetWeight()
				targetChannels = append(targetChannels, channel)
			}
		}
	}

	if len(targetChannels) == 0 {
		return nil, nil
	}

	// Prefer explicit OpenAI wire settings before the broader API-type preference.
	// If no matching channels exist at this priority, fall back to all channels (format conversion).
	targetChannels = preferChannelsByRequestFormat(targetChannels, preferredAPIType, relayFormat)

	// Recalculate sumWeight after potential filtering by API type
	sumWeight = 0
	for _, channel := range targetChannels {
		sumWeight += channel.GetWeight()
	}

	// smoothing factor and adjustment
	smoothingFactor := 1
	smoothingAdjustment := 0

	if sumWeight == 0 {
		// when all channels have weight 0, set sumWeight to the number of channels and set smoothing adjustment to 100
		// each channel's effective weight = 100
		sumWeight = len(targetChannels) * 100
		smoothingAdjustment = 100
	} else if sumWeight/len(targetChannels) < 10 {
		// when the average weight is less than 10, set smoothing factor to 100
		smoothingFactor = 100
	}

	// Calculate the total weight of all channels up to endIdx
	totalWeight := sumWeight * smoothingFactor

	// Generate a random value in the range [0, totalWeight)
	randomWeight := rand.Intn(totalWeight)

	// Find a channel based on its weight
	for _, channel := range targetChannels {
		randomWeight -= channel.GetWeight()*smoothingFactor + smoothingAdjustment
		if randomWeight < 0 {
			return channel, nil
		}
	}
	// return null if no channel is not found
	return nil, errors.New("channel not found")
}

// preferChannelsByAPIType filters channels to only include those matching the preferred API type.
// If no channels match, returns the original list (fall back to format conversion).
func preferChannelsByAPIType(channels []*Channel, preferredAPIType int) []*Channel {
	matched := make([]*Channel, 0, len(channels))
	for _, ch := range channels {
		chAPIType, _ := common.ChannelType2APIType(ch.Type)
		if chAPIType == preferredAPIType {
			matched = append(matched, ch)
		}
	}
	if len(matched) > 0 {
		return matched
	}
	return channels
}

// GetPriorityCount returns the number of distinct priority levels for a group/model pair.
func GetPriorityCount(group string, model string) int {
	if !common.MemoryCacheEnabled {
		return getPriorityCountDB(group, model)
	}

	channelSyncLock.RLock()
	defer channelSyncLock.RUnlock()

	channels := group2model2channels[group][model]
	if len(channels) == 0 {
		normalizedModel := ratio.FormatMatchingModelName(model)
		channels = group2model2channels[group][normalizedModel]
	}

	uniquePriorities := make(map[int]bool)
	for _, channelId := range channels {
		if channel, ok := channelsIDM[channelId]; ok {
			uniquePriorities[int(channel.GetPriority())] = true
		}
	}
	return len(uniquePriorities)
}

func CacheGetChannel(id int) (*Channel, error) {
	if !common.MemoryCacheEnabled {
		return GetChannelById(id, true)
	}
	channelSyncLock.RLock()
	defer channelSyncLock.RUnlock()

	c, ok := channelsIDM[id]
	if !ok {
		return nil, i18n.NewError(i18n.MsgChannelCacheNotFound, map[string]any{"ID": id})
	}
	return c, nil
}

func CacheGetChannelInfo(id int) (*ChannelInfo, error) {
	if !common.MemoryCacheEnabled {
		channel, err := GetChannelById(id, true)
		if err != nil {
			return nil, err
		}
		return &channel.ChannelInfo, nil
	}
	channelSyncLock.RLock()
	defer channelSyncLock.RUnlock()

	c, ok := channelsIDM[id]
	if !ok {
		return nil, i18n.NewError(i18n.MsgChannelCacheNotFound, map[string]any{"ID": id})
	}
	return &c.ChannelInfo, nil
}

func CacheUpdateChannelStatus(id int, status int) {
	if !common.MemoryCacheEnabled {
		return
	}
	channelSyncLock.Lock()
	defer channelSyncLock.Unlock()
	channel, ok := channelsIDM[id]
	if !ok {
		return
	}
	channel.Status = status
	if status == common.ChannelStatusEnabled {
		addChannelToGroupModelCache(channel)
	} else {
		removeChannelFromGroupModelCache(id)
	}
}

func addChannelToGroupModelCache(channel *Channel) {
	if channel == nil {
		return
	}
	if group2model2channels == nil {
		group2model2channels = make(map[string]map[string][]int)
	}
	groups := channel.GetGroups()
	models := channel.GetModels()
	for _, group := range groups {
		if group == "" {
			continue
		}
		if _, ok := group2model2channels[group]; !ok {
			group2model2channels[group] = make(map[string][]int)
		}
		for _, model := range models {
			model = strings.TrimSpace(model)
			if model == "" {
				continue
			}
			channels := group2model2channels[group][model]
			if isChannelIDInList(channels, channel.Id) {
				continue
			}
			group2model2channels[group][model] = append(channels, channel.Id)
		}
	}
}

func removeChannelFromGroupModelCache(id int) {
	for group, model2channels := range group2model2channels {
		for model, channels := range model2channels {
			for i, channelId := range channels {
				if channelId == id {
					group2model2channels[group][model] = append(channels[:i], channels[i+1:]...)
					break
				}
			}
		}
	}
}

func CacheUpdateChannel(channel *Channel) {
	if !common.MemoryCacheEnabled {
		return
	}
	channelSyncLock.Lock()
	defer channelSyncLock.Unlock()
	if channel == nil {
		return
	}
	channelsIDM[channel.Id] = channel
}
