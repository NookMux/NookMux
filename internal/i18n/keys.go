package i18n

// Message keys for i18n translations
// Use these constants instead of hardcoded strings

// Common error messages
const (
	MsgInvalidParams      = "common.invalid_params"
	MsgDatabaseError      = "common.database_error"
	MsgGenerateFailed     = "common.generate_failed"
	MsgUpdateFailed       = "common.update_failed"
	MsgConfigFetchFailed  = "common.config_fetch_failed"
	MsgJobIDRequired      = "common.job_id_required"
	MsgTaskNotFound       = "common.task_not_found"
	MsgDeprecatedAPI      = "common.deprecated_api"
	MsgInvalidToken       = "common.invalid_token"
	MsgNotLoggedIn        = "common.not_logged_in"
	MsgInvalidRequestBody = "common.invalid_request_body"
)

// Misc/common API messages
const (
	MsgMiscDBConnectionFailed           = "misc.db_connection_failed"
	MsgMiscEmailAliasRejected           = "misc.email_alias_rejected"
	MsgMiscEmailTaken                   = "misc.email_taken"
	MsgMiscEmailSendFailed              = "misc.email_send_failed"
	MsgMiscPasswordResetLinkInvalid     = "misc.password_reset_link_invalid"
	MsgMiscUsageLogFieldsParseFailed    = "misc.usage_log_fields_parse_failed"
	MsgMiscServerRunning                = "misc.server_running"
	MsgMiscEmailDomainWhitelistRejected = "misc.email_domain_whitelist_rejected"
	MsgMiscMigrated                     = "misc.migrated"
)

// Dashboard related messages
const (
	MsgDashboardQuotaDataDisabled        = "dashboard.quota_data_disabled"
	MsgDashboardUserAnalyticsDisabled    = "dashboard.user_analytics_disabled"
	MsgDashboardMediaConvertDisabled     = "dashboard.media_convert_disabled"
	MsgDashboardTimeRangeTooLong         = "dashboard.time_range_too_long"
	MsgDashboardInvalidTimeRange         = "dashboard.invalid_time_range"
	MsgDashboardRecalculateComplete      = "dashboard.recalculate_complete"
	MsgDashboardMissingAffinityClearArgs = "dashboard.missing_affinity_clear_args"
)

// Dashboard config related messages
const (
	MsgDashboardConfigGetFailed       = "dashboard_config.get_failed"
	MsgDashboardConfigRequestInvalid  = "dashboard_config.request_invalid"
	MsgDashboardConfigNoUpdates       = "dashboard_config.no_updates"
	MsgDashboardConfigUpdateFailed    = "dashboard_config.update_failed"
	MsgDashboardConfigSerializeFailed = "dashboard_config.serialize_failed"
	MsgDashboardConfigResetFailed     = "dashboard_config.reset_failed"
	MsgDashboardConfigUpdated         = "dashboard_config.updated"
	MsgDashboardConfigReset           = "dashboard_config.reset"
)

// Setup related messages
const (
	MsgSetupAlreadyInitialized = "setup.already_initialized"
	MsgSetupRequestInvalid     = "setup.request_invalid"
	MsgSetupUsernameTooLong    = "setup.username_too_long"
	MsgSetupPasswordMismatch   = "setup.password_mismatch"
	MsgSetupPasswordMin        = "setup.password_min"
	MsgSetupSystemError        = "setup.system_error"
	MsgSetupCreateAdminFailed  = "setup.create_admin_failed"
	MsgSetupInitFailed         = "setup.init_failed"
	MsgSetupInitSuccess        = "setup.init_success"
)

// Token related messages
const (
	MsgTokenNameTooLong          = "token.name_too_long"
	MsgTokenQuotaNegative        = "token.quota_negative"
	MsgTokenQuotaExceedMax       = "token.quota_exceed_max"
	MsgTokenGenerateFailed       = "token.generate_failed"
	MsgTokenGetInfoFailed        = "token.get_info_failed"
	MsgTokenExpiredCannotEnable  = "token.expired_cannot_enable"
	MsgTokenExhaustedCannotEable = "token.exhausted_cannot_enable"
	MsgTokenWindowHoursMin       = "token.window_hours_min"
	MsgTokenWindowQuotaPositive  = "token.window_quota_positive"
	MsgTokenWindowStartRange     = "token.window_start_range"
	MsgTokenCycleDaysMin         = "token.cycle_days_min"
	MsgTokenCycleQuotaPositive   = "token.cycle_quota_positive"
	MsgTokenInvalidQuotaType     = "token.invalid_quota_type"
	MsgTokenNoAuthHeader         = "token.no_auth_header"
	MsgTokenInvalidBearer        = "token.invalid_bearer"
	MsgTokenAuthorized           = "token.authorized"
	MsgTokenMaxLimitReached      = "token.max_limit_reached"
	MsgTokenInvalidModelMapping  = "token.invalid_model_mapping"
)

// Redemption related messages
const (
	MsgRedemptionNameLength        = "redemption.name_length"
	MsgRedemptionCountPositive     = "redemption.count_positive"
	MsgRedemptionCountMax          = "redemption.count_max"
	MsgRedemptionCreateFailed      = "redemption.create_failed"
	MsgRedemptionExpireTimeInvalid = "redemption.expire_time_invalid"
)

// User related messages
const (
	MsgUserPasswordLoginDisabled     = "user.password_login_disabled"
	MsgUserRegisterDisabled          = "user.register_disabled"
	MsgUserPasswordRegisterDisabled  = "user.password_register_disabled"
	MsgUserUsernameOrPasswordError   = "user.username_or_password_error"
	MsgUserLoginUnavailable          = "user.login_unavailable"
	MsgUserExists                    = "user.exists"
	MsgUserNotExists                 = "user.not_exists"
	MsgUserSessionSaveFailed         = "user.session_save_failed"
	MsgUserRequire2FA                = "user.require_2fa"
	MsgUserEmailVerificationRequired = "user.email_verification_required"
	MsgUserVerificationCodeError     = "user.verification_code_error"
	MsgUserInputInvalid              = "user.input_invalid"
	MsgUserNoPermissionSameLevel     = "user.no_permission_same_level"
	MsgUserNoPermissionHigherLevel   = "user.no_permission_higher_level"
	MsgUserCannotCreateHigherLevel   = "user.cannot_create_higher_level"
	MsgUserCannotDeleteRootUser      = "user.cannot_delete_root_user"
	MsgUserCannotDisableRootUser     = "user.cannot_disable_root_user"
	MsgUserCannotDemoteRootUser      = "user.cannot_demote_root_user"
	MsgUserAlreadyAdmin              = "user.already_admin"
	MsgUserAlreadyCommon             = "user.already_common"
	MsgUserAdminCannotPromote        = "user.admin_cannot_promote"
	MsgUserTransferSuccess           = "user.transfer_success"
	MsgUserTransferFailed            = "user.transfer_failed"
	MsgUserTopUpProcessing           = "user.topup_processing"
	MsgUserRegisterFailed            = "user.register_failed"
	MsgUserDefaultTokenFailed        = "user.default_token_failed"
	MsgUserBanned                    = "user.banned"
)

// Prefill group related messages
const (
	MsgPrefillGroupNameTypeRequired = "prefill_group.name_type_required"
	MsgPrefillGroupNameExists       = "prefill_group.name_exists"
	MsgPrefillGroupMissingID        = "prefill_group.missing_id"
)

// Model metadata related messages
const (
	MsgModelMetaNameRequired = "model_meta.name_required"
	MsgModelMetaNameExists   = "model_meta.name_exists"
	MsgModelMetaMissingID    = "model_meta.missing_id"
)

// Vendor metadata related messages
const (
	MsgVendorMetaNameRequired = "vendor_meta.name_required"
	MsgVendorMetaNameExists   = "vendor_meta.name_exists"
	MsgVendorMetaMissingID    = "vendor_meta.missing_id"
)

// Dynamic ratio related messages
const (
	MsgDynamicRatioRuleIDRequired = "dynamic_ratio.rule_id_required"
	MsgDynamicRatioInvalidRuleID  = "dynamic_ratio.invalid_rule_id"
	MsgDynamicRatioIDListRequired = "dynamic_ratio.id_list_required"
	MsgDynamicRatioGroupForbidden = "dynamic_ratio.group_forbidden"
)

// Ticket related messages
const (
	MsgTicketInvalidID = "ticket.invalid_id"
)

// Channel related messages
const (
	MsgChannelMultiKeyBalanceUnsupported       = "channel.multi_key_balance_unsupported"
	MsgChannelBalanceNotImplemented            = "channel.balance_not_implemented"
	MsgChannelTestAlreadyRunning               = "channel.test_already_running"
	MsgChannelResponseTimeExceeded             = "channel.response_time_exceeded"
	MsgChannelGetTagsFailed                    = "channel.get_tags_failed"
	MsgChannelCountTagsFailed                  = "channel.count_tags_failed"
	MsgChannelGetTagChannelsFailed             = "channel.get_tag_channels_failed"
	MsgChannelCountFailed                      = "channel.count_failed"
	MsgChannelListFailed                       = "channel.list_failed"
	MsgChannelTypeStatsFailed                  = "channel.type_stats_failed"
	MsgChannelGetKeyFailed                     = "channel.get_key_failed"
	MsgChannelParseResponseFailed              = "channel.parse_response_failed"
	MsgChannelIDFormatError                    = "channel.id_format_error"
	MsgChannelGetInfoFailed                    = "channel.get_info_failed"
	MsgChannelNotFound                         = "channel.not_found"
	MsgChannelGetSuccess                       = "channel.get_success"
	MsgChannelSettingsInvalid                  = "channel.settings_invalid"
	MsgChannelModelNameTooLong                 = "channel.model_name_too_long"
	MsgChannelVertexRegionRequired             = "channel.vertex_region_required"
	MsgChannelVertexRegionJSONInvalid          = "channel.vertex_region_json_invalid"
	MsgChannelVertexRegionDefaultMissing       = "channel.vertex_region_default_missing"
	MsgChannelVertexBatchKeysJSONInvalid       = "channel.vertex_batch_keys_json_invalid"
	MsgChannelVertexKeyJSONEncodeFailed        = "channel.vertex_key_json_encode_failed"
	MsgChannelVertexBatchKeysEmpty             = "channel.vertex_batch_keys_empty"
	MsgChannelAddModeUnsupported               = "channel.add_mode_unsupported"
	MsgChannelParamInvalid                     = "channel.param_invalid"
	MsgChannelTagRequired                      = "channel.tag_required"
	MsgChannelParamOverrideJSONInvalid         = "channel.param_override_json_invalid"
	MsgChannelHeaderOverrideJSONInvalid        = "channel.header_override_json_invalid"
	MsgChannelAppendKeysParseFailed            = "channel.append_keys_parse_failed"
	MsgChannelMultiKeyModeInvalid              = "channel.multi_key_mode_invalid"
	MsgChannelCopyInfoFailed                   = "channel.copy_info_failed"
	MsgChannelCopyFailed                       = "channel.copy_failed"
	MsgChannelNotMultiKey                      = "channel.not_multi_key"
	MsgChannelDisableKeyIndexRequired          = "channel.disable_key_index_required"
	MsgChannelEnableKeyIndexRequired           = "channel.enable_key_index_required"
	MsgChannelDeleteKeyIndexRequired           = "channel.delete_key_index_required"
	MsgChannelKeyIndexOutOfRange               = "channel.key_index_out_of_range"
	MsgChannelKeyDisabled                      = "channel.key_disabled"
	MsgChannelKeyEnabled                       = "channel.key_enabled"
	MsgChannelKeysEnabled                      = "channel.keys_enabled"
	MsgChannelNoKeysToDisable                  = "channel.no_keys_to_disable"
	MsgChannelKeysDisabled                     = "channel.keys_disabled"
	MsgChannelLastKeyCannotDelete              = "channel.last_key_cannot_delete"
	MsgChannelKeyDeleted                       = "channel.key_deleted"
	MsgChannelNoAutoDisabledKeysToDelete       = "channel.no_auto_disabled_keys_to_delete"
	MsgChannelAutoDisabledKeysDeleted          = "channel.auto_disabled_keys_deleted"
	MsgChannelOperationUnsupported             = "channel.operation_unsupported"
	MsgChannelNotPlan                          = "channel.not_plan"
	MsgChannelQuotaQueryFailed                 = "channel.quota_query_failed"
	MsgChannelUsageUnsupported                 = "channel.usage_unsupported"
	MsgChannelTimeFormatInvalid                = "channel.time_format_invalid"
	MsgChannelEndBeforeStart                   = "channel.end_before_start"
	MsgChannelTimeRangeTooLong31Days           = "channel.time_range_too_long_31_days"
	MsgChannelUsageQueryFailed                 = "channel.usage_query_failed"
	MsgChannelRiskUnsupported                  = "channel.risk_unsupported"
	MsgChannelRiskCheckFailed                  = "channel.risk_check_failed"
	MsgChannelContactUnsupported               = "channel.contact_unsupported"
	MsgChannelContactQueryFailed               = "channel.contact_query_failed"
	MsgChannelKeyInvalid                       = "channel.key_invalid"
	MsgChannelActivityUnsupported              = "channel.activity_unsupported"
	MsgChannelActivityQueryFailed              = "channel.activity_query_failed"
	MsgChannelResetCardUnsupported             = "channel.reset_card_unsupported"
	MsgChannelResetCardQueryFailed             = "channel.reset_card_query_failed"
	MsgChannelResetCardUseFailed               = "channel.reset_card_use_failed"
	MsgChannelOllamaGetModelsFailed            = "channel.ollama_get_models_failed"
	MsgChannelGeminiGetModelsFailed            = "channel.gemini_get_models_failed"
	MsgChannelOllamaVersionFailed              = "channel.ollama_version_failed"
	MsgChannelInvalidRequest                   = "channel.invalid_request"
	MsgChannelFetchModelsFailed                = "channel.fetch_models_failed"
	MsgChannelFetchProvidersFailed             = "channel.fetch_providers_failed"
	MsgChannelProvidersOpenRouterOnly          = "channel.providers_openrouter_only"
	MsgChannelInvalidID                        = "channel.invalid_id"
	MsgChannelInvalidRequestParameters         = "channel.invalid_request_parameters"
	MsgChannelIdAndModelRequired               = "channel.id_and_model_required"
	MsgChannelOllamaOnly                       = "channel.ollama_only"
	MsgChannelPullModelFailed                  = "channel.pull_model_failed"
	MsgChannelPullModelSuccess                 = "channel.pull_model_success"
	MsgChannelDeleteModelFailed                = "channel.delete_model_failed"
	MsgChannelDeleteModelSuccess               = "channel.delete_model_success"
	MsgChannelEmpty                            = "channel.empty"
	MsgChannelInvalidType                      = "channel.invalid_type"
	MsgChannelUnsupportedType                  = "channel.unsupported_type"
	MsgChannelTestToolNotSupported             = "channel.test_tool_not_supported"
	MsgChannelTestNotSupported                 = "channel.test_not_supported"
	MsgChannelResponsesCompactionOnlyOpenAI    = "channel.responses_compaction_only_openai"
	MsgChannelInvalidApiType                   = "channel.invalid_api_type"
	MsgChannelInvalidEmbeddingRequest          = "channel.invalid_embedding_request"
	MsgChannelInvalidImageRequest              = "channel.invalid_image_request"
	MsgChannelInvalidRerankRequest             = "channel.invalid_rerank_request"
	MsgChannelInvalidResponseRequest           = "channel.invalid_response_request"
	MsgChannelInvalidResponseCompactionRequest = "channel.invalid_response_compaction_request"
	MsgChannelInvalidGeneralRequest            = "channel.invalid_general_request"
	MsgChannelEmptyModelResponse               = "channel.empty_model_response"
	MsgChannelReasoningOnlyResponse            = "channel.reasoning_only_response"
	MsgChannelRuleNameRequired                 = "channel.rule_name_required"
	MsgChannelKeyFpRequired                    = "channel.key_fp_required"
	MsgChannelAutoGroupsNotEnabled             = "channel.auto_groups_not_enabled"
	MsgChannelProxyEmpty                       = "channel.proxy_empty"
	MsgChannelProxyInvalid                     = "channel.proxy_invalid"
	MsgChannelProxyRequestFailed               = "channel.proxy_request_failed"
	MsgChannelProxyUnexpectedStatus            = "channel.proxy_unexpected_status"
	MsgChannelProxyEmptyResponse               = "channel.proxy_empty_response"
)

// Relay related messages
const (
	MsgRelayRetryGetChannelFailed = "relay.retry_get_channel_failed"
	MsgRelayRetryChannelNotFound  = "relay.retry_channel_not_found"
)

// Option related messages
const (
	MsgOptionKeyRequired                  = "option.key_required"
	MsgOptionReadForbidden                = "option.read_forbidden"
	MsgOptionNotFound                     = "option.not_found"
	MsgOptionJSONMapUnsupported           = "option.json_map_unsupported"
	MsgOptionJSONMapParseFailed           = "option.json_map_parse_failed"
	MsgOptionJSONArrayUnsupported         = "option.json_array_unsupported"
	MsgOptionMigratedToVoiceManagement    = "option.migrated_to_voice_management"
	MsgOptionInvalidParams                = "option.invalid_params"
	MsgOptionMapKeyRequired               = "option.map_key_required"
	MsgOptionMapItemNotFound              = "option.map_item_not_found"
	MsgOptionMiniMaxSettingFailed         = "option.minimax_setting_failed"
	MsgOptionOriginalMapItemNotFound      = "option.original_map_item_not_found"
	MsgOptionMapKeyExists                 = "option.map_key_exists"
	MsgOptionMapValueJSONRequired         = "option.map_value_json_required"
	MsgOptionRemoved                      = "option.removed"
	MsgOptionValueTypeInvalid             = "option.value_type_invalid"
	MsgOptionGitHubOAuthConfigRequired    = "option.github_oauth_config_required"
	MsgOptionLinuxDOOAuthConfigRequired   = "option.linuxdo_oauth_config_required"
	MsgOptionEmailDomainRequired          = "option.email_domain_required"
	MsgOptionTurnstileConfigRequired      = "option.turnstile_config_required"
	MsgOptionAudioRatioFailed             = "option.audio_ratio_failed"
	MsgOptionAudioCompletionRatioFailed   = "option.audio_completion_ratio_failed"
	MsgOptionCreateCacheRatioFailed       = "option.create_cache_ratio_failed"
	MsgOptionContextPricingFailed         = "option.context_pricing_failed"
	MsgOptionRetryTimesRange              = "option.retry_times_range"
	MsgOptionRetryTimesPositiveWhenEnable = "option.retry_times_positive_when_enable"
	MsgOptionRetryEnabledMustBool         = "option.retry_enabled_must_bool"
	MsgOptionRetryEnableNeedsPositive     = "option.retry_enable_needs_positive"
	MsgOptionToolBillingRulesParseFailed  = "option.tool_billing_rules_parse_failed"
	MsgOptionToolBillingRulesSetFailed    = "option.tool_billing_rules_set_failed"
)

// Model sync and audit messages
const (
	MsgModelSyncGetModelsFailed         = "model_sync.get_models_failed"
	MsgModelSyncFetchUpstreamFailed     = "model_sync.fetch_upstream_failed"
	MsgAuditFetchFailed                 = "audit.fetch_failed"
	MsgPlaygroundAccessTokenUnsupported = "playground.access_token_unsupported"
	MsgTopupStripeInvalidAPIKey         = "topup.stripe_invalid_api_key"
	MsgUserOriginalPasswordError        = "user.original_password_error"
)

// Check-in related messages
const (
	MsgCheckinDisabled = "checkin.disabled"
	MsgCheckinSuccess  = "checkin.success"
)

// Custom voice related messages
const (
	MsgCustomVoiceNotLoggedIn         = "custom_voice.not_logged_in"
	MsgCustomVoiceUploadAudioRequired = "custom_voice.upload_audio_required"
)

// Pricing related messages
const (
	MsgPricingResetModelRatioSuccess       = "pricing.reset_model_ratio_success"
	MsgPricingResetToolBillingRulesSuccess = "pricing.reset_tool_billing_rules_success"
)

// Quota related messages
const (
	MsgQuotaThresholdGtZero      = "quota.threshold_gt_zero"
	MsgQuotaUserNotEnough        = "quota.user_not_enough"
	MsgQuotaTokenNotEnough       = "quota.token_not_enough"
	MsgQuotaNegative             = "quota.negative"
	MsgQuotaTokenWindowNotEnough = "quota.token_window_not_enough"
	MsgQuotaTokenCycleNotEnough  = "quota.token_cycle_not_enough"
	MsgQuotaRelayInfoNil         = "quota.relay_info_nil"
	MsgQuotaEmptyUsage           = "quota.empty_usage"
)

// Setting related messages
const (
	MsgSettingInvalidType      = "setting.invalid_type"
	MsgSettingWebhookEmpty     = "setting.webhook_empty"
	MsgSettingWebhookInvalid   = "setting.webhook_invalid"
	MsgSettingEmailInvalid     = "setting.email_invalid"
	MsgSettingBarkUrlEmpty     = "setting.bark_url_empty"
	MsgSettingBarkUrlInvalid   = "setting.bark_url_invalid"
	MsgSettingGotifyUrlEmpty   = "setting.gotify_url_empty"
	MsgSettingGotifyTokenEmpty = "setting.gotify_token_empty"
	MsgSettingGotifyUrlInvalid = "setting.gotify_url_invalid"
	MsgSettingUrlMustHttp      = "setting.url_must_http"
	MsgSettingSaved            = "setting.saved"
)

// OAuth related messages
const (
	MsgOAuthInvalidCode     = "oauth.invalid_code"
	MsgOAuthGetUserErr      = "oauth.get_user_error"
	MsgOAuthUnknownProvider = "oauth.unknown_provider"
	MsgOAuthStateInvalid    = "oauth.state_invalid"
	MsgOAuthNotEnabled      = "oauth.not_enabled"
	MsgOAuthUserDeleted     = "oauth.user_deleted"
	MsgOAuthUserBanned      = "oauth.user_banned"
	MsgOAuthBindSuccess     = "oauth.bind_success"
	MsgOAuthAlreadyBound    = "oauth.already_bound"
	MsgOAuthConnectFailed   = "oauth.connect_failed"
	MsgOAuthTokenFailed     = "oauth.token_failed"
	MsgOAuthUserInfoEmpty   = "oauth.user_info_empty"
	MsgOAuthTrustLevelLow   = "oauth.trust_level_low"
)

// Model layer error messages (for translation in controller)
const (
	MsgRedeemFailed             = "redeem.failed"
	MsgUuidDuplicate            = "common.uuid_duplicate"
	MsgInvalidInput             = "common.invalid_input"
	MsgCommonGetUserGroupFailed = "common.get_user_group_failed"
)

// MiniMax TTS related messages
const (
	MsgMiniMaxVoiceNotAuthorized       = "minimax.voice_not_authorized"
	MsgMiniMaxVoiceNotAuthorizedWithID = "minimax.voice_not_authorized_with_id"
	MsgMiniMaxVoiceIDRequired          = "minimax.voice_id_required"
	MsgMiniMaxVoiceInvalidType         = "minimax.voice_invalid_type"
	MsgMiniMaxVoiceInvalidID           = "minimax.voice_invalid_id"
	MsgMiniMaxVoiceNotFound            = "minimax.voice_not_found"
)

// Passkey related messages
const (
	MsgPasskeyLoginDisabled      = "passkey.login_disabled"
	MsgPasskeyNotBound           = "passkey.not_bound"
	MsgPasskeyLimitReached       = "passkey.limit_reached"
	MsgPasskeyCreateFailed       = "passkey.create_failed"
	MsgPasskeyRegisterOK         = "passkey.register_success"
	MsgPasskeyInvalidID          = "passkey.invalid_id"
	MsgPasskeyDeleteDenied       = "passkey.delete_denied"
	MsgPasskeyUnbound            = "passkey.unbound"
	MsgPasskeyStateInvalid       = "passkey.state_invalid"
	MsgPasskeyUserDisabled       = "passkey.user_disabled"
	MsgPasskeyUpdateFailed       = "passkey.update_failed"
	MsgPasskeyInvalidUserID      = "passkey.invalid_user_id"
	MsgPasskeyResetOK            = "passkey.reset_success"
	MsgPasskeyVerifyOK           = "passkey.verify_success"
	MsgPasskeyUpdateDenied       = "passkey.update_denied"
	MsgPasskeyRequestInvalid     = "passkey.request_invalid"
	MsgPasskeyUpdated            = "passkey.updated"
	MsgPasskeySecureRequired     = "passkey.secure_required"
	MsgPasskeyInvalidSession     = "passkey.invalid_session"
	MsgPasskeyCredentialNotFound = "passkey.credential_not_found"
	MsgPasskeyUserInfoFailed     = "passkey.user_info_failed"
	MsgPasskeyUserHandleMismatch = "passkey.user_handle_mismatch"
	MsgPasskeySaveVerifyFailed   = "passkey.save_verify_failed"
)

// Secure verification related messages
const (
	MsgSecureVerifyNoMethod            = "secure_verification.no_method"
	MsgSecureVerifyTwoFANotEnabled     = "secure_verification.twofa_not_enabled"
	MsgSecureVerifyPasskeyNotEnabled   = "secure_verification.passkey_not_enabled"
	MsgSecureVerifyCodeRequired        = "secure_verification.code_required"
	MsgSecureVerifyPasskeyRequired     = "secure_verification.passkey_required"
	MsgSecureVerifyUnsupportedMethod   = "secure_verification.unsupported_method"
	MsgSecureVerifyFailed              = "secure_verification.failed"
	MsgSecureVerifySuccess             = "secure_verification.success"
	MsgSecureVerifyInvalidPasskeyState = "secure_verification.invalid_passkey_state"
)

// Two-factor authentication related messages
const (
	MsgTwoFAAlreadyEnabledReset  = "twofa.already_enabled_reset"
	MsgTwoFASecretGenerateFailed = "twofa.secret_generate_failed"
	MsgTwoFABackupGenerateFailed = "twofa.backup_generate_failed"
	MsgTwoFABackupSaveFailed     = "twofa.backup_save_failed"
	MsgTwoFASetupInitSuccess     = "twofa.setup_init_success"
	MsgTwoFASetupRequired        = "twofa.setup_required"
	MsgTwoFAAlreadyEnabled       = "twofa.already_enabled"
	MsgTwoFACodeInvalid          = "twofa.code_invalid"
	MsgTwoFAEnableSuccess        = "twofa.enable_success"
	MsgTwoFANotEnabled           = "twofa.not_enabled"
	MsgTwoFADisableSuccess       = "twofa.disable_success"
	MsgTwoFABackupRegenerateOK   = "twofa.backup_regenerate_success"
	MsgTwoFASessionExpired       = "twofa.session_expired"
	MsgTwoFASessionInvalid       = "twofa.session_invalid"
	MsgTwoFAAdminForbidden       = "twofa.admin_forbidden"
	MsgTwoFAAdminDisabled        = "twofa.admin_disabled"
)

// Stored media related messages
const (
	MsgStoredMediaIDRequired       = "stored_media.id_required"
	MsgStoredMediaMediaTypeInvalid = "stored_media.media_type_invalid"
	MsgStoredMediaNotFound         = "stored_media.not_found"
	MsgStoredMediaForbidden        = "stored_media.forbidden"
	MsgStoredMediaNoValidIDs       = "stored_media.no_valid_ids"
)

// Performance related messages
const (
	MsgPerformanceDiskCacheCleared = "performance.disk_cache_cleared"
	MsgPerformanceStatsReset       = "performance.stats_reset"
	MsgPerformanceGCExecuted       = "performance.gc_executed"
)

// Rankings related messages
const (
	MsgRankingsDisabled = "rankings.disabled"
)

// Top-up related messages
const (
	MsgTopupAmountExceedMax       = "topup.amount_exceed_max"
	MsgTopupSuccessUrlUntrusted   = "topup.success_url_untrusted"
	MsgTopupCancelUrlUntrusted    = "topup.cancel_url_untrusted"
	MsgTopupUnsupportedChannel    = "topup.unsupported_channel"
	MsgTopupAmountBelowMin        = "topup.amount_below_min"
	MsgTopupGetGroupFailed        = "topup.get_group_failed"
	MsgTopupPayAmountTooLow       = "topup.pay_amount_too_low"
	MsgTopupPaymentMethodNotFound = "topup.payment_method_not_found"
	MsgTopupPaymentConfigMissing  = "topup.payment_config_missing"
	MsgTopupPaymentInitFailed     = "topup.payment_init_failed"
	MsgTopupOrderCreateFailed     = "topup.order_create_failed"
	MsgTopupInvalidParams         = "topup.invalid_params"
	MsgTopupSuccess               = "topup.success"
	MsgTopupFailed                = "topup.failed"
)

// Channel affinity related messages
const (
	MsgChannelAffinityRuleNameRequired        = "channel_affinity.rule_name_required"
	MsgChannelAffinitySettingNotInitialized   = "channel_affinity.setting_not_initialized"
	MsgChannelAffinityUnknownRuleName         = "channel_affinity.unknown_rule_name"
	MsgChannelAffinityIncludeRuleNameDisabled = "channel_affinity.include_rule_name_disabled"
)

// Billing related messages
const (
	MsgBillingRelayInfoNil       = "billing.relay_info_nil"
	MsgBillingUserQuotaNotEnough = "billing.user_quota_not_enough"
	MsgBillingPrepaidFailed      = "billing.prepaid_failed"
)
