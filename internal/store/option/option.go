package optionstore

import (
	"errors"
	"fmt"
	"github.com/NookMux/NookMux/internal/common"
	"github.com/NookMux/NookMux/internal/config"
	"github.com/NookMux/NookMux/internal/config/manager"
	"github.com/NookMux/NookMux/internal/config/model"
	"github.com/NookMux/NookMux/internal/config/operation"
	"github.com/NookMux/NookMux/internal/config/performance"
	"github.com/NookMux/NookMux/internal/config/ratio"
	"github.com/NookMux/NookMux/internal/config/system"
	"github.com/NookMux/NookMux/internal/store/db"
	"github.com/NookMux/NookMux/internal/store/usedata"
	"strconv"
	"strings"
	"time"
)

type Option struct {
	Key   string `json:"key" gorm:"primaryKey"`
	Value string `json:"value"`
}

func AllOption() ([]*Option, error) {
	var options []*Option
	err := dbstore.DB.Find(&options).Error
	return options, err
}

func InitOptionMap() {
	common.OptionMapRWMutex.Lock()
	common.OptionMap = make(map[string]string)

	// 添加原有的系统配置
	common.OptionMap["FileUploadPermission"] = strconv.Itoa(common.FileUploadPermission)
	common.OptionMap["FileDownloadPermission"] = strconv.Itoa(common.FileDownloadPermission)
	common.OptionMap["ImageUploadPermission"] = strconv.Itoa(common.ImageUploadPermission)
	common.OptionMap["ImageDownloadPermission"] = strconv.Itoa(common.ImageDownloadPermission)
	common.OptionMap["PasswordLoginEnabled"] = strconv.FormatBool(common.PasswordLoginEnabled)
	common.OptionMap["PasswordRegisterEnabled"] = strconv.FormatBool(common.PasswordRegisterEnabled)
	common.OptionMap["EmailVerificationEnabled"] = strconv.FormatBool(common.EmailVerificationEnabled)
	common.OptionMap["GitHubOAuthEnabled"] = strconv.FormatBool(common.GitHubOAuthEnabled)
	common.OptionMap["LinuxDOOAuthEnabled"] = strconv.FormatBool(common.LinuxDOOAuthEnabled)
	common.OptionMap["TurnstileCheckEnabled"] = strconv.FormatBool(common.TurnstileCheckEnabled)
	common.OptionMap["RegisterEnabled"] = strconv.FormatBool(common.RegisterEnabled)
	common.OptionMap["AutomaticDisableChannelEnabled"] = strconv.FormatBool(common.AutomaticDisableChannelEnabled)
	common.OptionMap["AutomaticEnableChannelEnabled"] = strconv.FormatBool(common.AutomaticEnableChannelEnabled)
	common.OptionMap["LogConsumeEnabled"] = strconv.FormatBool(common.LogConsumeEnabled)
	common.OptionMap["DataExportEnabled"] = strconv.FormatBool(common.DataExportEnabled)
	common.OptionMap["ChannelDisableThreshold"] = strconv.FormatFloat(common.ChannelDisableThreshold, 'f', -1, 64)
	common.OptionMap["EmailDomainRestrictionEnabled"] = strconv.FormatBool(common.EmailDomainRestrictionEnabled)
	common.OptionMap["EmailAliasRestrictionEnabled"] = strconv.FormatBool(common.EmailAliasRestrictionEnabled)
	common.OptionMap["EmailDomainWhitelist"] = strings.Join(common.EmailDomainWhitelist, ",")
	common.OptionMap["SMTPServer"] = ""
	common.OptionMap["SMTPFrom"] = ""
	common.OptionMap["SMTPPort"] = strconv.Itoa(common.SMTPPort)
	common.OptionMap["SMTPAccount"] = ""
	common.OptionMap["SMTPToken"] = ""
	common.OptionMap["SMTPSSLEnabled"] = strconv.FormatBool(common.SMTPSSLEnabled)
	common.OptionMap["SMTPForceLoginAuthEnabled"] = strconv.FormatBool(common.SMTPForceLoginAuthEnabled)
	common.OptionMap["Notice"] = ""
	common.OptionMap["About"] = ""
	common.OptionMap["HomePageContent"] = ""
	common.OptionMap["Footer"] = common.Footer
	common.OptionMap["SystemName"] = common.SystemName
	common.OptionMap["Logo"] = common.Logo
	common.OptionMap["ServerAddress"] = ""
	common.OptionMap["WorkerUrl"] = system.WorkerUrl
	common.OptionMap["WorkerValidKey"] = system.WorkerValidKey
	common.OptionMap["WorkerAllowHttpImageRequestEnabled"] = strconv.FormatBool(system.WorkerAllowHttpImageRequestEnabled)
	common.OptionMap["PayAddress"] = ""
	common.OptionMap["CustomCallbackAddress"] = ""
	common.OptionMap["EpayId"] = ""
	common.OptionMap["EpayKey"] = ""
	common.OptionMap["Price"] = strconv.FormatFloat(operation.Price, 'f', -1, 64)
	common.OptionMap["MinTopUp"] = strconv.Itoa(operation.MinTopUp)
	common.OptionMap["StripeMinTopUp"] = strconv.Itoa(config.StripeMinTopUp)
	common.OptionMap["StripeApiSecret"] = config.StripeApiSecret
	common.OptionMap["StripeWebhookSecret"] = config.StripeWebhookSecret
	common.OptionMap["StripePriceId"] = config.StripePriceId
	common.OptionMap["StripeUnitPrice"] = strconv.FormatFloat(config.StripeUnitPrice, 'f', -1, 64)
	common.OptionMap["StripePromotionCodesEnabled"] = strconv.FormatBool(config.StripePromotionCodesEnabled)
	common.OptionMap["TopupGroupRatio"] = common.TopupGroupRatio2JSONString()
	common.OptionMap["AutoGroups"] = config.AutoGroups2JsonString()
	common.OptionMap["DefaultUseAutoGroup"] = strconv.FormatBool(config.DefaultUseAutoGroup)
	common.OptionMap["PayMethods"] = operation.PayMethods2JsonString()
	common.OptionMap["GitHubClientId"] = ""
	common.OptionMap["GitHubClientSecret"] = ""
	common.OptionMap["TurnstileSiteKey"] = ""
	common.OptionMap["TurnstileSecretKey"] = ""
	common.OptionMap["QuotaForNewUser"] = strconv.Itoa(common.QuotaForNewUser)
	common.OptionMap["QuotaForInviter"] = strconv.Itoa(common.QuotaForInviter)
	common.OptionMap["QuotaForInvitee"] = strconv.Itoa(common.QuotaForInvitee)
	common.OptionMap["QuotaRemindThreshold"] = strconv.Itoa(common.QuotaRemindThreshold)
	common.OptionMap["PreConsumedQuota"] = strconv.Itoa(common.PreConsumedQuota)
	common.OptionMap["ModelRequestRateLimitCount"] = strconv.Itoa(config.ModelRequestRateLimitCount)
	common.OptionMap["ModelRequestRateLimitDurationMinutes"] = strconv.Itoa(config.ModelRequestRateLimitDurationMinutes)
	common.OptionMap["ModelRequestRateLimitSuccessCount"] = strconv.Itoa(config.ModelRequestRateLimitSuccessCount)
	common.OptionMap["ModelRequestRateLimitGroup"] = config.ModelRequestRateLimitGroup2JSONString()
	common.OptionMap["ModelRatio"] = ratio.ModelRatio2JSONString()
	common.OptionMap["ModelPrice"] = ratio.ModelPrice2JSONString()
	common.OptionMap["CacheRatio"] = ratio.CacheRatio2JSONString()
	common.OptionMap["CreateCacheRatio"] = ratio.CreateCacheRatio2JSONString()
	common.OptionMap["ContextPricing"] = ratio.ContextPricing2JSONString()
	common.OptionMap["GroupRatio"] = ratio.GroupRatio2JSONString()
	common.OptionMap["GroupGroupRatio"] = ratio.GroupGroupRatio2JSONString()
	common.OptionMap["UserUsableGroups"] = config.UserUsableGroups2JSONString()
	common.OptionMap["CompletionRatio"] = ratio.CompletionRatio2JSONString()
	common.OptionMap["AudioRatio"] = ratio.AudioRatio2JSONString()
	common.OptionMap["AudioCompletionRatio"] = ratio.AudioCompletionRatio2JSONString()
	common.OptionMap["TopUpLink"] = common.TopUpLink
	common.OptionMap["QuotaPerUnit"] = strconv.FormatFloat(common.QuotaPerUnit, 'f', -1, 64)
	common.OptionMap["RetryTimes"] = strconv.Itoa(common.RetryTimes)
	common.OptionMap["AutomaticRetryEnabled"] = strconv.FormatBool(common.AutomaticRetryEnabled)
	common.OptionMap["DataExportInterval"] = strconv.Itoa(common.DataExportInterval)
	common.OptionMap["DataExportDefaultTime"] = common.DataExportDefaultTime
	common.OptionMap["DefaultCollapseSidebar"] = strconv.FormatBool(common.DefaultCollapseSidebar)
	common.OptionMap["CheckSensitiveEnabled"] = strconv.FormatBool(config.CheckSensitiveEnabled)

	common.OptionMap["ModelRequestRateLimitEnabled"] = strconv.FormatBool(config.ModelRequestRateLimitEnabled)
	common.OptionMap["CheckSensitiveOnPromptEnabled"] = strconv.FormatBool(config.CheckSensitiveOnPromptEnabled)
	common.OptionMap["StopOnSensitiveEnabled"] = strconv.FormatBool(config.StopOnSensitiveEnabled)
	common.OptionMap["SensitiveWords"] = config.SensitiveWordsToString()
	common.OptionMap["StreamCacheQueueLength"] = strconv.Itoa(config.StreamCacheQueueLength)
	common.OptionMap["AutomaticDisableKeywords"] = operation.AutomaticDisableKeywordsToString()
	common.OptionMap["AutomaticDisableStatusCodes"] = operation.AutomaticDisableStatusCodesToString()
	common.OptionMap["AutomaticRetryStatusCodes"] = operation.AutomaticRetryStatusCodesToString()
	common.OptionMap["DynamicRatioEnabled"] = strconv.FormatBool(common.DynamicRatioEnabled)

	// 自动添加所有注册的模型配置
	modelConfigs := manager.GlobalConfig.ExportAllConfigs()
	for k, v := range modelConfigs {
		common.OptionMap[k] = v
	}

	common.OptionMapRWMutex.Unlock()
	loadOptionsFromDatabase()
}

func loadOptionsFromDatabase() {
	options, _ := AllOption()
	toolBillingMigrated, migratedToolBillingRules := migrateLegacyToolBillingRulesInOptions(options)

	for _, option := range options {
		err := updateOptionMap(option.Key, option.Value)
		if err != nil {
			common.SysLog("failed to update option map: " + err.Error())
		}
	}
	if toolBillingMigrated {
		persistMigratedToolBillingRules(migratedToolBillingRules)
	}

	// One-time migration: if the removed toggle "quota_setting.enable_free_model_pre_consume"
	// exists in DB and is "false", override FreeModelPreConsumedQuota to 0 so that
	// users who had disabled free-model pre-consumption don't suddenly get charged
	// after upgrade. Then persist the migrated value. Always delete the stale row
	// regardless of its value since the toggle has been removed entirely.
	for _, option := range options {
		if option.Key == "quota_setting.enable_free_model_pre_consume" {
			if option.Value == "false" {
				quotaSetting := operation.GetQuotaSetting()
				quotaSetting.FreeModelPreConsumedQuota = 0
				// Sync to OptionMap so API returns the correct value
				common.OptionMap["quota_setting.free_model_pre_consumed_quota"] = "0"
				// Persist the migrated quota to DB so it survives restart
				migratedOption := Option{Key: "quota_setting.free_model_pre_consumed_quota"}
				dbstore.DB.FirstOrCreate(&migratedOption, Option{Key: "quota_setting.free_model_pre_consumed_quota"})
				dbstore.DB.Model(&migratedOption).Update("value", "0")
				common.SysLog("migrated quota_setting.enable_free_model_pre_consume=false -> free_model_pre_consumed_quota=0")
			}
			// Delete the stale toggle row regardless of value
			dbstore.DB.Where(dbstore.CommonKeyCol+" = ?", "quota_setting.enable_free_model_pre_consume").Delete(&Option{})
			break
		}
	}

	// Backward-compatibility migration: AutomaticRetryEnabled was introduced as a
	// standalone toggle. On older deployments the row may not exist yet. To avoid
	// silently turning off an already-configured retry setup after upgrade, derive
	// the initial enabled state from the existing RetryTimes: if an admin had set
	// RetryTimes > 0 we treat automatic retry as enabled.
	retryEnabledExists := false
	for _, option := range options {
		if option.Key == "AutomaticRetryEnabled" {
			retryEnabledExists = true
			break
		}
	}
	if !retryEnabledExists {
		derivedEnabled := common.RetryTimes > 0
		common.AutomaticRetryEnabled = derivedEnabled
		common.OptionMap["AutomaticRetryEnabled"] = strconv.FormatBool(derivedEnabled)
		migratedOption := Option{Key: "AutomaticRetryEnabled"}
		dbstore.DB.FirstOrCreate(&migratedOption, Option{Key: "AutomaticRetryEnabled"})
		dbstore.DB.Model(&migratedOption).Update("value", strconv.FormatBool(derivedEnabled))
	}

}

func migrateLegacyToolBillingRulesInOptions(options []*Option) (bool, string) {
	for i := range options {
		if options[i] == nil || options[i].Key != "tool_billing_setting.rules" || options[i].Value == "" {
			continue
		}
		migrated, didMigrate, err := operation.MigrateLegacyRules(options[i].Value)
		if err != nil {
			common.SysError("failed to migrate tool_billing_setting.rules: " + err.Error())
			return false, ""
		}
		if !didMigrate {
			return false, ""
		}
		options[i].Value = migrated
		return true, migrated
	}
	return false, ""
}

func persistMigratedToolBillingRules(migrated string) {
	migratedOption := Option{Key: "tool_billing_setting.rules"}
	if err := dbstore.DB.FirstOrCreate(&migratedOption, Option{Key: "tool_billing_setting.rules"}).Error; err != nil {
		common.SysError("failed to create migrated tool_billing_setting.rules: " + err.Error())
		return
	}
	if err := dbstore.DB.Model(&migratedOption).Update("value", migrated).Error; err != nil {
		common.SysError("failed to persist migrated tool_billing_setting.rules: " + err.Error())
		return
	}
	common.SysLog("migrated tool_billing_setting.rules to conditions format")
}

func SyncOptions(frequency int) {
	for {
		time.Sleep(time.Duration(frequency) * time.Second)
		common.SysLog("syncing options from database")
		loadOptionsFromDatabase()
	}
}

func UpdateOption(key string, value string) error {
	switch key {
	case "Chats", "ChatLink", "ChatLink2",
		"WeChatAuthEnabled", "WeChatServerAddress", "WeChatServerToken", "WeChatAccountQRCodeImageURL",
		"oidc.enabled", "oidc.client_id", "oidc.client_secret", "oidc.well_known", "oidc.authorization_endpoint", "oidc.token_endpoint", "oidc.user_info_endpoint",
		"CreemApiKey", "CreemProducts", "CreemTestMode", "CreemWebhookSecret",
		"ExposeRatioEnabled", "ImageRatio",
		"global.third_party_multimodal_model_id", "global.third_party_multimodal_call_api_type",
		"global.third_party_multimodal_system_prompt", "global.third_party_multimodal_first_user_prompt",
		"global.third_party_multimodal_user_agent", "global.third_party_multimodal_x_title",
		"global.third_party_multimodal_http_referer",
		"quota_setting.enable_free_model_pre_consume":
		return errors.New("option removed")
	}
	if err := validateConfigUpdate(key, value); err != nil {
		return err
	}
	// PayAddress 是易支付商户密钥明文外发的目标网关，保存前强制 https 校验，
	// 非 https 直接拒绝落库。
	if key == "PayAddress" {
		if err := operation.ValidatePayAddress(value); err != nil {
			return err
		}
	}
	// Save to database first
	option := Option{
		Key: key,
	}
	// https://gorm.io/docs/update.html#Save-All-Fields
	if err := dbstore.DB.FirstOrCreate(&option, Option{Key: key}).Error; err != nil {
		return err
	}
	option.Value = value
	// Save is a combination function.
	// If save value does not contain primary key, it will execute Create,
	// otherwise it will execute Update (with all fields).
	if err := dbstore.DB.Save(&option).Error; err != nil {
		return err
	}
	// Update OptionMap
	return updateOptionMap(key, value)
}

func updateOptionMap(key string, value string) (err error) {
	common.OptionMapRWMutex.Lock()
	defer common.OptionMapRWMutex.Unlock()

	// Removed legacy options: keep DB compatibility but do not load them into memory.
	// (Avoid resurrecting removed features via SyncOptions or stale DB rows.)
	switch key {
	case "Chats", "ChatLink", "ChatLink2",
		"WeChatAuthEnabled", "WeChatServerAddress", "WeChatServerToken", "WeChatAccountQRCodeImageURL",
		"oidc.enabled", "oidc.client_id", "oidc.client_secret", "oidc.well_known", "oidc.authorization_endpoint", "oidc.token_endpoint", "oidc.user_info_endpoint",
		"CreemApiKey", "CreemProducts", "CreemTestMode", "CreemWebhookSecret",
		"ExposeRatioEnabled", "ImageRatio",
		"global.third_party_multimodal_model_id", "global.third_party_multimodal_call_api_type",
		"global.third_party_multimodal_system_prompt", "global.third_party_multimodal_first_user_prompt",
		"global.third_party_multimodal_user_agent", "global.third_party_multimodal_x_title",
		"global.third_party_multimodal_http_referer",
		"quota_setting.enable_free_model_pre_consume":
		delete(common.OptionMap, key)
		return nil
	}
	// 检查是否是模型配置 - 使用更规范的方式处理
	if handled, cfgErr := handleConfigUpdate(key, value); handled {
		if cfgErr != nil {
			return cfgErr
		}
		common.OptionMap[key] = value
		return nil // 已由配置系统处理
	}
	common.OptionMap[key] = value

	// 处理传统配置项...
	if strings.HasSuffix(key, "Permission") {
		intValue, _ := strconv.Atoi(value)
		switch key {
		case "FileUploadPermission":
			common.FileUploadPermission = intValue
		case "FileDownloadPermission":
			common.FileDownloadPermission = intValue
		case "ImageUploadPermission":
			common.ImageUploadPermission = intValue
		case "ImageDownloadPermission":
			common.ImageDownloadPermission = intValue
		}
	}
	if strings.HasSuffix(key, "Enabled") || key == "DefaultCollapseSidebar" || key == "DefaultUseAutoGroup" {
		boolValue := value == "true"
		switch key {
		case "PasswordRegisterEnabled":
			common.PasswordRegisterEnabled = boolValue
		case "PasswordLoginEnabled":
			common.PasswordLoginEnabled = boolValue
		case "EmailVerificationEnabled":
			common.EmailVerificationEnabled = boolValue
		case "GitHubOAuthEnabled":
			common.GitHubOAuthEnabled = boolValue
		case "LinuxDOOAuthEnabled":
			common.LinuxDOOAuthEnabled = boolValue
		case "TurnstileCheckEnabled":
			common.TurnstileCheckEnabled = boolValue
		case "RegisterEnabled":
			common.RegisterEnabled = boolValue
		case "EmailDomainRestrictionEnabled":
			common.EmailDomainRestrictionEnabled = boolValue
		case "EmailAliasRestrictionEnabled":
			common.EmailAliasRestrictionEnabled = boolValue
		case "AutomaticDisableChannelEnabled":
			common.AutomaticDisableChannelEnabled = boolValue
		case "AutomaticEnableChannelEnabled":
			common.AutomaticEnableChannelEnabled = boolValue
		case "LogConsumeEnabled":
			common.LogConsumeEnabled = boolValue
		case "DataExportEnabled":
			common.DataExportEnabled = boolValue
		case "DefaultCollapseSidebar":
			common.DefaultCollapseSidebar = boolValue
		case "CheckSensitiveEnabled":
			config.CheckSensitiveEnabled = boolValue
		case "CheckSensitiveOnPromptEnabled":
			config.CheckSensitiveOnPromptEnabled = boolValue
		case "ModelRequestRateLimitEnabled":
			config.ModelRequestRateLimitEnabled = boolValue
		case "StopOnSensitiveEnabled":
			config.StopOnSensitiveEnabled = boolValue
		case "SMTPSSLEnabled":
			common.SMTPSSLEnabled = boolValue
		case "WorkerAllowHttpImageRequestEnabled":
			system.WorkerAllowHttpImageRequestEnabled = boolValue
		case "DefaultUseAutoGroup":
			config.DefaultUseAutoGroup = boolValue
		case "DynamicRatioEnabled":
			common.DynamicRatioEnabled = boolValue
		case "AutomaticRetryEnabled":
			common.AutomaticRetryEnabled = boolValue
		}
	}
	switch key {
	case "EmailDomainWhitelist":
		common.EmailDomainWhitelist = strings.Split(value, ",")
	case "SMTPServer":
		common.SMTPServer = value
	case "SMTPPort":
		intValue, _ := strconv.Atoi(value)
		common.SMTPPort = intValue
	case "SMTPAccount":
		common.SMTPAccount = value
	case "SMTPFrom":
		common.SMTPFrom = value
	case "SMTPToken":
		common.SMTPToken = value
	case "ServerAddress":
		system.ServerAddress = value
	case "WorkerUrl":
		system.WorkerUrl = value
	case "WorkerValidKey":
		system.WorkerValidKey = value
	case "PayAddress":
		operation.PayAddress = value
	case "AutoGroups":
		err = config.UpdateAutoGroupsByJsonString(value)
	case "CustomCallbackAddress":
		operation.CustomCallbackAddress = value
	case "EpayId":
		operation.EpayId = value
	case "EpayKey":
		operation.EpayKey = value
	case "Price":
		operation.Price, _ = strconv.ParseFloat(value, 64)
	case "MinTopUp":
		operation.MinTopUp, _ = strconv.Atoi(value)
	case "StripeApiSecret":
		config.StripeApiSecret = value
	case "StripeWebhookSecret":
		config.StripeWebhookSecret = value
	case "StripePriceId":
		config.StripePriceId = value
	case "StripeUnitPrice":
		config.StripeUnitPrice, _ = strconv.ParseFloat(value, 64)
	case "StripeMinTopUp":
		config.StripeMinTopUp, _ = strconv.Atoi(value)
	case "StripePromotionCodesEnabled":
		config.StripePromotionCodesEnabled = value == "true"
	case "TopupGroupRatio":
		err = common.UpdateTopupGroupRatioByJSONString(value)
	case "GitHubClientId":
		common.GitHubClientId = value
	case "GitHubClientSecret":
		common.GitHubClientSecret = value
	case "LinuxDOClientId":
		common.LinuxDOClientId = value
	case "LinuxDOClientSecret":
		common.LinuxDOClientSecret = value
	case "LinuxDOMinimumTrustLevel":
		common.LinuxDOMinimumTrustLevel, _ = strconv.Atoi(value)
	case "Footer":
		common.Footer = value
	case "SystemName":
		common.SystemName = value
	case "Logo":
		common.Logo = value
	case "TurnstileSiteKey":
		common.TurnstileSiteKey = value
	case "TurnstileSecretKey":
		common.TurnstileSecretKey = value
	case "QuotaForNewUser":
		common.QuotaForNewUser, _ = strconv.Atoi(value)
	case "QuotaForInviter":
		common.QuotaForInviter, _ = strconv.Atoi(value)
	case "QuotaForInvitee":
		common.QuotaForInvitee, _ = strconv.Atoi(value)
	case "QuotaRemindThreshold":
		common.QuotaRemindThreshold, _ = strconv.Atoi(value)
	case "PreConsumedQuota":
		common.PreConsumedQuota, _ = strconv.Atoi(value)
	case "ModelRequestRateLimitCount":
		config.ModelRequestRateLimitCount, _ = strconv.Atoi(value)
	case "ModelRequestRateLimitDurationMinutes":
		config.ModelRequestRateLimitDurationMinutes, _ = strconv.Atoi(value)
	case "ModelRequestRateLimitSuccessCount":
		config.ModelRequestRateLimitSuccessCount, _ = strconv.Atoi(value)
	case "ModelRequestRateLimitGroup":
		err = config.UpdateModelRequestRateLimitGroupByJSONString(value)
	case "RetryTimes":
		common.RetryTimes, _ = strconv.Atoi(value)
	case "DataExportInterval":
		common.DataExportInterval, _ = strconv.Atoi(value)
	case "DataExportDefaultTime":
		common.DataExportDefaultTime = value
	case "ModelRatio":
		err = ratio.UpdateModelRatioByJSONString(value)
	case "GroupRatio":
		err = ratio.UpdateGroupRatioByJSONString(value)
	case "GroupGroupRatio":
		err = ratio.UpdateGroupGroupRatioByJSONString(value)
	case "UserUsableGroups":
		err = config.UpdateUserUsableGroupsByJSONString(value)
	case "CompletionRatio":
		err = ratio.UpdateCompletionRatioByJSONString(value)
	case "ModelPrice":
		err = ratio.UpdateModelPriceByJSONString(value)
	case "CacheRatio":
		err = ratio.UpdateCacheRatioByJSONString(value)
	case "CreateCacheRatio":
		err = ratio.UpdateCreateCacheRatioByJSONString(value)
	case "ContextPricing":
		err = ratio.UpdateContextPricingByJSONString(value)
	case "AudioRatio":
		err = ratio.UpdateAudioRatioByJSONString(value)
	case "AudioCompletionRatio":
		err = ratio.UpdateAudioCompletionRatioByJSONString(value)
	case "TopUpLink":
		common.TopUpLink = value
	case "ChannelDisableThreshold":
		common.ChannelDisableThreshold, _ = strconv.ParseFloat(value, 64)
	case "QuotaPerUnit":
		common.QuotaPerUnit, _ = strconv.ParseFloat(value, 64)
	case "SensitiveWords":
		config.SensitiveWordsFromString(value)
	case "AutomaticDisableKeywords":
		operation.AutomaticDisableKeywordsFromString(value)
	case "AutomaticDisableStatusCodes":
		err = operation.AutomaticDisableStatusCodesFromString(value)
	case "AutomaticRetryStatusCodes":
		err = operation.AutomaticRetryStatusCodesFromString(value)
	case "StreamCacheQueueLength":
		config.StreamCacheQueueLength, _ = strconv.Atoi(value)
	case "PayMethods":
		err = operation.UpdatePayMethodsByJsonString(value)
	}
	return err
}

func validateConfigUpdate(key, value string) error {
	parts := strings.SplitN(key, ".", 2)
	if len(parts) != 2 {
		return nil
	}

	cfg := manager.GlobalConfig.Get(parts[0])
	if cfg == nil {
		return nil
	}
	if parts[0] == "minimax" {
		return model.WithMiniMaxSettingsReadLock(func() error {
			return manager.ValidateConfigFromMap(cfg, map[string]string{parts[1]: value})
		})
	}

	return manager.ValidateConfigFromMap(cfg, map[string]string{parts[1]: value})
}

// handleConfigUpdate 处理分层配置更新，返回是否已处理
func handleConfigUpdate(key, value string) (bool, error) {
	parts := strings.SplitN(key, ".", 2)
	if len(parts) != 2 {
		return false, nil // 不是分层配置
	}

	configName := parts[0]
	configKey := parts[1]

	// 获取配置对象
	cfg := manager.GlobalConfig.Get(configName)
	if cfg == nil {
		return false, nil // 未注册的配置
	}

	// 更新配置
	configMap := map[string]string{
		configKey: value,
	}
	updateConfig := func() error {
		return manager.UpdateConfigFromMap(cfg, configMap)
	}
	var err error
	if configName == "minimax" {
		err = model.WithMiniMaxSettingsWriteLock(updateConfig)
	} else {
		err = updateConfig()
	}
	if err != nil {
		return true, err
	}

	// 特定配置的后处理
	if configName == "performance_setting" {
		// 同步磁盘缓存配置到 common 包
		performance.UpdateAndSync()
	}
	if configName == "dashboard_config" {
		// 同步写入层聚合粒度配置到 model 包级变量，避免 model 反向依赖 dashboard_setting
		syncDashboardTrackingConfig(cfg)
	}

	return true, nil // 已处理
}

// syncDashboardTrackingConfig 从已更新的 dashboard_config 配置对象读取写入层配置，同步到 model 包级变量。
// 通过 manager.ConfigToMap 反射读取，不直接依赖 dashboard.DashboardConfig 类型，避免循环导入。
func syncDashboardTrackingConfig(cfg interface{}) {
	m, err := manager.ConfigToMap(cfg)
	if err != nil {
		common.SysError(fmt.Sprintf("syncDashboardTrackingConfig: 解析 dashboard_config 失败: %s", err))
		return
	}
	tokens, _ := strconv.ParseBool(m["quota_data_track_tokens"])
	byModel, _ := strconv.ParseBool(m["quota_data_track_by_model"])
	byUser, _ := strconv.ParseBool(m["quota_data_track_by_user"])
	usedatastore.SyncQuotaDataTrackingConfig(tokens, byModel, byUser)
}
