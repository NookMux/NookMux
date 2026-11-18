package config

import (
	"fmt"
	"math"
	"sync"

	"github.com/NookMux/NookMux/internal/common"
	"github.com/NookMux/NookMux/internal/i18n"
	"github.com/NookMux/NookMux/pkg/jsonx"
)

var ModelRequestRateLimitEnabled = false
var ModelRequestRateLimitDurationMinutes = 1
var ModelRequestRateLimitCount = 0
var ModelRequestRateLimitSuccessCount = 1000
var ModelRequestRateLimitGroup = map[string][2]int{}
var ModelRequestRateLimitMutex sync.RWMutex

func ModelRequestRateLimitGroup2JSONString() string {
	ModelRequestRateLimitMutex.RLock()
	defer ModelRequestRateLimitMutex.RUnlock()

	jsonBytes, err := jsonx.Marshal(ModelRequestRateLimitGroup)
	if err != nil {
		common.SysLog("error marshalling model ratio: " + err.Error())
	}
	return string(jsonBytes)
}

func UpdateModelRequestRateLimitGroupByJSONString(jsonStr string) error {
	ModelRequestRateLimitMutex.RLock()
	defer ModelRequestRateLimitMutex.RUnlock()

	ModelRequestRateLimitGroup = make(map[string][2]int)
	return jsonx.Unmarshal([]byte(jsonStr), &ModelRequestRateLimitGroup)
}

func GetGroupRateLimit(group string) (totalCount, successCount int, found bool) {
	ModelRequestRateLimitMutex.RLock()
	defer ModelRequestRateLimitMutex.RUnlock()

	if ModelRequestRateLimitGroup == nil {
		return 0, 0, false
	}

	limits, found := ModelRequestRateLimitGroup[group]
	if !found {
		return 0, 0, false
	}
	return limits[0], limits[1], true
}

func CheckModelRequestRateLimitGroup(jsonStr string) error {
	checkModelRequestRateLimitGroup := make(map[string][2]int)
	err := jsonx.Unmarshal([]byte(jsonStr), &checkModelRequestRateLimitGroup)
	if err != nil {
		return i18n.NewErrorWithFallback(err.Error(), i18n.MsgOptionGroupRatioInvalidJSON).WithCause(err)
	}
	for group, limits := range checkModelRequestRateLimitGroup {
		if limits[0] < 0 || limits[1] < 1 {
			return i18n.NewErrorWithFallback(fmt.Sprintf("group %s has negative rate limit values: [%d, %d]", group, limits[0], limits[1]), i18n.MsgOptionRateLimitNegativeValues, map[string]any{"Group": group, "Min": limits[0], "Max": limits[1]})
		}
		if limits[0] > math.MaxInt32 || limits[1] > math.MaxInt32 {
			return i18n.NewErrorWithFallback(fmt.Sprintf("group %s [%d, %d] has max rate limits value 2147483647", group, limits[0], limits[1]), i18n.MsgOptionRateLimitExceedsMax, map[string]any{"Group": group, "Min": limits[0], "Max": limits[1]})
		}
	}

	return nil
}
