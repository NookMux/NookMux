package console

import (
	"encoding/json"
	"errors"
	"net/url"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/NookMux/NookMux/internal/common"
	"github.com/NookMux/NookMux/internal/i18n"
	"github.com/NookMux/NookMux/pkg/jsonx"
)

var (
	urlRegex       = regexp.MustCompile(`^https?://(?:(?:[a-zA-Z0-9](?:[a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?\.)*[a-zA-Z0-9](?:[a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?|(?:(?:25[0-5]|2[0-4][0-9]|[01]?[0-9][0-9]?)\.){3}(?:25[0-5]|2[0-4][0-9]|[01]?[0-9][0-9]?))(?:\:[0-9]{1,5})?(?:/.*)?$`)
	dangerousChars = []string{"<script", "<iframe", "javascript:", "onload=", "onerror=", "onclick="}
	validColors    = map[string]bool{
		"blue": true, "green": true, "cyan": true, "purple": true, "pink": true,
		"red": true, "orange": true, "amber": true, "yellow": true, "lime": true,
		"light-green": true, "teal": true, "light-blue": true, "indigo": true,
		"violet": true, "grey": true,
	}
	slugRegex = regexp.MustCompile(`^[a-zA-Z0-9_-]+$`)
)

func parseJSONArray(jsonStr string, lang string, typeNameKey string) ([]map[string]interface{}, error) {
	var list []map[string]interface{}
	if err := jsonx.UnmarshalJsonStr(jsonStr, &list); err != nil {
		return nil, errors.New(i18n.Translate(lang, i18n.MsgConsoleParseFailed, map[string]any{
			"Type":  i18n.Translate(lang, typeNameKey),
			"Error": err.Error(),
		}))
	}
	return list, nil
}

func validateURL(urlStr string, lang string, index int, itemTypeKey string) error {
	if !urlRegex.MatchString(urlStr) {
		return errors.New(i18n.Translate(lang, i18n.MsgConsoleURLFormatInvalid, map[string]any{
			"Index": index,
			"Type":  i18n.Translate(lang, itemTypeKey),
		}))
	}
	if _, err := url.Parse(urlStr); err != nil {
		return errors.New(i18n.Translate(lang, i18n.MsgConsoleURLParseFailed, map[string]any{
			"Index": index,
			"Type":  i18n.Translate(lang, itemTypeKey),
			"Error": err.Error(),
		}))
	}
	return nil
}

func checkDangerousContent(content string, lang string, index int, itemTypeKey string) error {
	lower := strings.ToLower(content)
	for _, d := range dangerousChars {
		if strings.Contains(lower, d) {
			return errors.New(i18n.Translate(lang, i18n.MsgConsoleDangerousContent, map[string]any{
				"Index": index,
				"Type":  i18n.Translate(lang, itemTypeKey),
			}))
		}
	}
	return nil
}

func getJSONList(jsonStr string) []map[string]interface{} {
	if jsonStr == "" {
		return []map[string]interface{}{}
	}
	var list []map[string]interface{}
	if err := jsonx.UnmarshalJsonStr(jsonStr, &list); err != nil {
		// 控制台配置在写入前已校验；解析失败说明存储值被外部篡改或损坏，
		// 记录真实原因并返回空列表，避免拖垮依赖该数据的公开接口（如 /api/status）。
		common.SysError("failed to parse console json list: " + err.Error())
	}
	return list
}

func ValidateConsoleSettings(lang string, settingsStr string, settingType string) error {
	if settingsStr == "" {
		return nil
	}

	switch settingType {
	case "ApiInfo":
		return validateApiInfo(lang, settingsStr)
	case "Announcements":
		return validateAnnouncements(lang, settingsStr)
	case "FAQ":
		return validateFAQ(lang, settingsStr)
	case "UptimeKumaGroups":
		return validateUptimeKumaGroups(lang, settingsStr)
	case "UsageLogFields":
		return validateUsageLogFields(lang, settingsStr)
	default:
		return errors.New(i18n.Translate(lang, i18n.MsgConsoleUnknownSettingType, map[string]any{"Type": settingType}))
	}
}

// validateUsageLogFields 校验使用日志字段可见性配置 JSON。
// 格式：{ "<fieldKey>": { "admin": bool, "user": bool }, ... }
// 校验：JSON 格式合法，所有 key 在已知字段列表中，每个字段必须包含 admin 和 user 两个布尔成员。
// 拒绝部分配置（如 {"channel":{}}），避免缺少 admin/user 导致字段被静默隐藏。
func validateUsageLogFields(lang string, fieldsStr string) error {
	// 先解析为 map[string]json.RawMessage，确保每个字段对象显式包含 admin 和 user 键。
	rawMap := make(map[string]json.RawMessage)
	if err := jsonx.UnmarshalJsonStr(fieldsStr, &rawMap); err != nil {
		return errors.New(i18n.Translate(lang, i18n.MsgConsoleUsageLogFieldsParseFailed, map[string]any{"Error": err.Error()}))
	}

	// 构建已知字段集合
	knownFields := make(map[string]bool, len(UsageLogFieldsDefaults()))
	for _, d := range UsageLogFieldsDefaults() {
		knownFields[d.Key] = true
	}

	for key, raw := range rawMap {
		if removedUsageLogFields[key] {
			// 已移除字段（表格列字段），静默忽略，向后兼容旧配置
			continue
		}
		if !knownFields[key] {
			return errors.New(i18n.Translate(lang, i18n.MsgConsoleUsageLogFieldUnknown, map[string]any{"Key": key}))
		}
		// 解析单个字段对象，检查 admin 和 user 都存在且为布尔值
		var obj map[string]interface{}
		if err := jsonx.Unmarshal(raw, &obj); err != nil {
			return errors.New(i18n.Translate(lang, i18n.MsgConsoleUsageLogFieldParseFailed, map[string]any{"Key": key, "Error": err.Error()}))
		}
		adminVal, hasAdmin := obj["admin"]
		userVal, hasUser := obj["user"]
		if !hasAdmin {
			return errors.New(i18n.Translate(lang, i18n.MsgConsoleUsageLogFieldMissingAdmin, map[string]any{"Key": key}))
		}
		if !hasUser {
			return errors.New(i18n.Translate(lang, i18n.MsgConsoleUsageLogFieldMissingUser, map[string]any{"Key": key}))
		}
		if _, ok := adminVal.(bool); !ok {
			return errors.New(i18n.Translate(lang, i18n.MsgConsoleUsageLogFieldAdminNotBool, map[string]any{"Key": key}))
		}
		if _, ok := userVal.(bool); !ok {
			return errors.New(i18n.Translate(lang, i18n.MsgConsoleUsageLogFieldUserNotBool, map[string]any{"Key": key}))
		}
	}
	return nil
}

func validateApiInfo(lang string, apiInfoStr string) error {
	apiInfoList, err := parseJSONArray(apiInfoStr, lang, i18n.MsgConsoleTypeApiInfo)
	if err != nil {
		return err
	}

	if len(apiInfoList) > 50 {
		return errors.New(i18n.Translate(lang, i18n.MsgConsoleApiInfoLimitExceeded))
	}

	for i, apiInfo := range apiInfoList {
		index := i + 1
		urlStr, ok := apiInfo["url"].(string)
		if !ok || urlStr == "" {
			return errors.New(i18n.Translate(lang, i18n.MsgConsoleApiInfoMissingURL, map[string]any{"Index": index}))
		}
		route, ok := apiInfo["route"].(string)
		if !ok || route == "" {
			return errors.New(i18n.Translate(lang, i18n.MsgConsoleApiInfoMissingRoute, map[string]any{"Index": index}))
		}
		description, ok := apiInfo["description"].(string)
		if !ok || description == "" {
			return errors.New(i18n.Translate(lang, i18n.MsgConsoleApiInfoMissingDescription, map[string]any{"Index": index}))
		}
		color, ok := apiInfo["color"].(string)
		if !ok || color == "" {
			return errors.New(i18n.Translate(lang, i18n.MsgConsoleApiInfoMissingColor, map[string]any{"Index": index}))
		}

		if err := validateURL(urlStr, lang, index, i18n.MsgConsoleTypeApiInfo); err != nil {
			return err
		}

		if len(urlStr) > 500 {
			return errors.New(i18n.Translate(lang, i18n.MsgConsoleApiInfoURLTooLong, map[string]any{"Index": index}))
		}
		if len(route) > 100 {
			return errors.New(i18n.Translate(lang, i18n.MsgConsoleApiInfoRouteTooLong, map[string]any{"Index": index}))
		}
		if len(description) > 200 {
			return errors.New(i18n.Translate(lang, i18n.MsgConsoleApiInfoDescriptionTooLong, map[string]any{"Index": index}))
		}

		if !validColors[color] {
			return errors.New(i18n.Translate(lang, i18n.MsgConsoleApiInfoColorInvalid, map[string]any{"Index": index}))
		}

		if err := checkDangerousContent(description, lang, index, i18n.MsgConsoleTypeApiInfo); err != nil {
			return err
		}
		if err := checkDangerousContent(route, lang, index, i18n.MsgConsoleTypeApiInfo); err != nil {
			return err
		}
	}
	return nil
}

func GetApiInfo() []map[string]interface{} {
	return getJSONList(GetConsoleSetting().ApiInfo)
}

func validateAnnouncements(lang string, announcementsStr string) error {
	list, err := parseJSONArray(announcementsStr, lang, i18n.MsgConsoleTypeAnnouncement)
	if err != nil {
		return err
	}
	if len(list) > 100 {
		return errors.New(i18n.Translate(lang, i18n.MsgConsoleAnnouncementLimitExceeded))
	}
	validTypes := map[string]bool{
		"default": true, "ongoing": true, "success": true, "warning": true, "error": true,
	}
	for i, ann := range list {
		index := i + 1
		content, ok := ann["content"].(string)
		if !ok || content == "" {
			return errors.New(i18n.Translate(lang, i18n.MsgConsoleAnnouncementMissingContent, map[string]any{"Index": index}))
		}
		publishDateAny, exists := ann["publishDate"]
		if !exists {
			return errors.New(i18n.Translate(lang, i18n.MsgConsoleAnnouncementMissingDate, map[string]any{"Index": index}))
		}
		publishDateStr, ok := publishDateAny.(string)
		if !ok || publishDateStr == "" {
			return errors.New(i18n.Translate(lang, i18n.MsgConsoleAnnouncementDateEmpty, map[string]any{"Index": index}))
		}
		if _, err := time.Parse(time.RFC3339, publishDateStr); err != nil {
			return errors.New(i18n.Translate(lang, i18n.MsgConsoleAnnouncementDateInvalid, map[string]any{"Index": index}))
		}
		if t, exists := ann["type"]; exists {
			if typeStr, ok := t.(string); ok {
				if !validTypes[typeStr] {
					return errors.New(i18n.Translate(lang, i18n.MsgConsoleAnnouncementTypeInvalid, map[string]any{"Index": index}))
				}
			}
		}
		if len(content) > 500 {
			return errors.New(i18n.Translate(lang, i18n.MsgConsoleAnnouncementContentTooLong, map[string]any{"Index": index}))
		}
		if extra, exists := ann["extra"]; exists {
			if extraStr, ok := extra.(string); ok && len(extraStr) > 200 {
				return errors.New(i18n.Translate(lang, i18n.MsgConsoleAnnouncementExtraTooLong, map[string]any{"Index": index}))
			}
		}
	}
	return nil
}

func validateFAQ(lang string, faqStr string) error {
	list, err := parseJSONArray(faqStr, lang, i18n.MsgConsoleTypeFaq)
	if err != nil {
		return err
	}
	if len(list) > 100 {
		return errors.New(i18n.Translate(lang, i18n.MsgConsoleFaqLimitExceeded))
	}
	for i, faq := range list {
		index := i + 1
		question, ok := faq["question"].(string)
		if !ok || question == "" {
			return errors.New(i18n.Translate(lang, i18n.MsgConsoleFaqMissingQuestion, map[string]any{"Index": index}))
		}
		answer, ok := faq["answer"].(string)
		if !ok || answer == "" {
			return errors.New(i18n.Translate(lang, i18n.MsgConsoleFaqMissingAnswer, map[string]any{"Index": index}))
		}
		if len(question) > 200 {
			return errors.New(i18n.Translate(lang, i18n.MsgConsoleFaqQuestionTooLong, map[string]any{"Index": index}))
		}
		if len(answer) > 1000 {
			return errors.New(i18n.Translate(lang, i18n.MsgConsoleFaqAnswerTooLong, map[string]any{"Index": index}))
		}
	}
	return nil
}

func getPublishTime(item map[string]interface{}) time.Time {
	if v, ok := item["publishDate"]; ok {
		if s, ok2 := v.(string); ok2 {
			if t, err := time.Parse(time.RFC3339, s); err == nil {
				return t
			}
		}
	}
	return time.Time{}
}

func GetAnnouncements() []map[string]interface{} {
	list := getJSONList(GetConsoleSetting().Announcements)
	sort.SliceStable(list, func(i, j int) bool {
		return getPublishTime(list[i]).After(getPublishTime(list[j]))
	})
	return list
}

func GetFAQ() []map[string]interface{} {
	return getJSONList(GetConsoleSetting().FAQ)
}

func validateUptimeKumaGroups(lang string, groupsStr string) error {
	groups, err := parseJSONArray(groupsStr, lang, i18n.MsgConsoleTypeUptimeKuma)
	if err != nil {
		return err
	}

	if len(groups) > 20 {
		return errors.New(i18n.Translate(lang, i18n.MsgConsoleUptimeKumaLimitExceeded))
	}

	nameSet := make(map[string]bool)

	for i, group := range groups {
		index := i + 1
		categoryName, ok := group["categoryName"].(string)
		if !ok || categoryName == "" {
			return errors.New(i18n.Translate(lang, i18n.MsgConsoleGroupMissingCategoryName, map[string]any{"Index": index}))
		}
		if nameSet[categoryName] {
			return errors.New(i18n.Translate(lang, i18n.MsgConsoleGroupCategoryNameDuplicate, map[string]any{"Index": index}))
		}
		nameSet[categoryName] = true
		urlStr, ok := group["url"].(string)
		if !ok || urlStr == "" {
			return errors.New(i18n.Translate(lang, i18n.MsgConsoleGroupMissingURL, map[string]any{"Index": index}))
		}
		slug, ok := group["slug"].(string)
		if !ok || slug == "" {
			return errors.New(i18n.Translate(lang, i18n.MsgConsoleGroupMissingSlug, map[string]any{"Index": index}))
		}
		description, ok := group["description"].(string)
		if !ok {
			description = ""
		}

		if err := validateURL(urlStr, lang, index, i18n.MsgConsoleTypeGroup); err != nil {
			return err
		}

		if len(categoryName) > 50 {
			return errors.New(i18n.Translate(lang, i18n.MsgConsoleGroupCategoryNameTooLong, map[string]any{"Index": index}))
		}
		if len(urlStr) > 500 {
			return errors.New(i18n.Translate(lang, i18n.MsgConsoleGroupURLTooLong, map[string]any{"Index": index}))
		}
		if len(slug) > 100 {
			return errors.New(i18n.Translate(lang, i18n.MsgConsoleGroupSlugTooLong, map[string]any{"Index": index}))
		}
		if len(description) > 200 {
			return errors.New(i18n.Translate(lang, i18n.MsgConsoleGroupDescriptionTooLong, map[string]any{"Index": index}))
		}

		if !slugRegex.MatchString(slug) {
			return errors.New(i18n.Translate(lang, i18n.MsgConsoleGroupSlugInvalid, map[string]any{"Index": index}))
		}

		if err := checkDangerousContent(description, lang, index, i18n.MsgConsoleTypeGroup); err != nil {
			return err
		}
		if err := checkDangerousContent(categoryName, lang, index, i18n.MsgConsoleTypeGroup); err != nil {
			return err
		}
	}
	return nil
}

func GetUptimeKumaGroups() []map[string]interface{} {
	return getJSONList(GetConsoleSetting().UptimeKumaGroups)
}
