/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/
export type SystemOption = {
  key: string
  value: string
}

export type SystemOptionKey = string

export type SystemOptionsResponse = {
  success: boolean
  message: string
  data: SystemOption[]
}

export type SystemOptionValueResponse = {
  success: boolean
  message: string
  data: SystemOption
}

export type OptionJsonMapEntry = {
  key: string
  value: string
}

export type OptionJsonMapResponse = {
  success: boolean
  message: string
  data: {
    items: OptionJsonMapEntry[]
    page: number
    page_size: number
    total: number
  }
}

export type OptionJsonArrayEntry = {
  value: string
}

export type OptionJsonArrayResponse = {
  success: boolean
  message: string
  data: {
    items: OptionJsonArrayEntry[]
    page: number
    page_size: number
    total: number
  }
}

export type DeleteOptionJsonMapEntryRequest = {
  key: string
  map_key: string
}

export type UpsertOptionJsonMapEntryRequest = {
  key: string
  map_key: string
  old_map_key?: string
  value: string
}

export type DeleteOptionJsonArrayEntryRequest = {
  key: string
  value: string
}

export type UpsertOptionJsonArrayEntryRequest = {
  key: string
  value: string
  old_value?: string
}

export type UpdateOptionRequest = {
  key: string
  value: string | boolean | number
}

export type UpdateOptionResponse = {
  success: boolean
  message: string
}

export type CleanLogsParams = {
  start_timestamp?: number
  end_timestamp: number
  clean_logs?: boolean
  clean_stored_images?: boolean
  clean_stored_videos?: boolean
  clean_audit_logs?: boolean
}

export type CleanLogsResult = {
  logs?: number
  stored_images?: number
  stored_videos?: number
  audit_logs?: number
}

export type CleanLogsResponse = {
  success: boolean
  message: string
  data?: CleanLogsResult
}

export type DatabaseMigrationMode = 'pre_migrate' | 'same_type_migrate'

export type DatabaseMigrationInfo = {
  main_db_type: string
  log_db_type: string
  log_db_is_separated: boolean
}

export type DatabaseMigrationTableProgress = {
  name: string
  copied: number
  total: number
}

export type DatabaseMigrationJobStatus = 'running' | 'success' | 'failed'

export type DatabaseMigrationJob = {
  id: string
  status: DatabaseMigrationJobStatus
  started_at: number
  finished_at?: number
  source_db_type: string
  target_db_type: string
  include_logs: boolean
  force: boolean
  current_step: string
  tables: DatabaseMigrationTableProgress[]
  logs: string[]
  error?: string
}

export type DatabaseMigrationStartRequest = {
  target_dsn: string
  target_log_dsn?: string
  include_logs: boolean
  force: boolean
}

export type DatabaseMigrationInfoResponse = {
  success: boolean
  message: string
  data: DatabaseMigrationInfo
}

export type DatabaseMigrationStartResponse = {
  success: boolean
  message: string
  data?: {
    job_id: string
  }
}

export type DatabaseMigrationJobResponse = {
  success: boolean
  message: string
  data: DatabaseMigrationJob
}

export type SiteSettings = {
  Notice: string
  SystemName: string
  Logo: string
  Footer: string
  About: string
  HomePageContent: string
  ServerAddress: string
  'legal.user_agreement': string
  'legal.privacy_policy': string
  'general_setting.docs_link': string
  HeaderNavModules: string
  SidebarModulesAdmin: string
}

export type AuthSettings = {
  PasswordLoginEnabled: boolean
  PasswordRegisterEnabled: boolean
  EmailVerificationEnabled: boolean
  RegisterEnabled: boolean
  EmailDomainRestrictionEnabled: boolean
  EmailAliasRestrictionEnabled: boolean
  EmailDomainWhitelist: string
  GitHubOAuthEnabled: boolean
  GitHubClientId: string
  GitHubClientSecret: string
  LinuxDOOAuthEnabled: boolean
  LinuxDOClientId: string
  LinuxDOClientSecret: string
  LinuxDOMinimumTrustLevel: string
  TurnstileCheckEnabled: boolean
  TurnstileSiteKey: string
  TurnstileSecretKey: string
  'passkey.enabled': boolean
  'passkey.rp_display_name': string
  'passkey.rp_id': string
  'passkey.origins': string
  'passkey.allow_insecure_origin': boolean
  'passkey.user_verification': 'required' | 'preferred' | 'discouraged'
  'passkey.attachment_preference': '' | 'platform' | 'cross-platform'
  'passkey.max_passkeys_per_user': number
}

export type ContentSettings = {
  'console_setting.api_info': string
  'console_setting.announcements': string
  'console_setting.faq': string
  'console_setting.uptime_kuma_groups': string
  'console_setting.usage_log_fields': string
  'console_setting.usage_log_fields_admin_enabled': boolean
  'console_setting.usage_log_fields_user_enabled': boolean
  DataExportEnabled: boolean
  DataExportDefaultTime: string
  DataExportInterval: number
}

export type DashboardSettings = {
  'dashboard_config.quota_data_enabled': boolean
  'dashboard_config.user_analytics_enabled': boolean
  'dashboard_config.rankings_enabled': boolean
  'dashboard_config.media_convert_stats_enabled': boolean
  'dashboard_config.quota_data_track_tokens': boolean
  'dashboard_config.quota_data_track_by_model': boolean
  'dashboard_config.quota_data_track_by_user': boolean
  'dashboard_config.api_info_enabled': boolean
  'dashboard_config.uptime_kuma_enabled': boolean
  'dashboard_config.announcements_enabled': boolean
  'dashboard_config.faq_enabled': boolean
  'dashboard_config.quota_data_refresh_interval': number
  'dashboard_config.user_analytics_refresh_interval': number
  'dashboard_config.rankings_refresh_interval': number
  'dashboard_config.uptime_kuma_refresh_interval': number
  'dashboard_config.default_time_range_days': number
  'dashboard_config.max_time_range_days': number
  'dashboard_config.rankings_model_limit': number
  'dashboard_config.rankings_vendor_limit': number
  'dashboard_config.user_analytics_top_n': number
}

export type ModelSettings = {
  'general_setting.ping_interval_enabled': boolean
  'general_setting.ping_interval_seconds': number
  'gemini.safety_settings': string
  'gemini.version_settings': string
  'gemini.supported_imagine_models': string
  'gemini.function_call_thought_signature_enabled': boolean
  'gemini.remove_function_response_id_enabled': boolean
  'claude.model_headers_settings': string
  'claude.default_max_tokens': string
  'claude.remove_claude_code_billing_header_enabled': boolean
  'grok.violation_deduction_enabled': boolean
  'grok.violation_deduction_amount': number
  'minimax.enabled': boolean
  'minimax.model_redirect': string
  'minimax.voice_whitelist_enabled': boolean
  'minimax.custom_voice_enabled': boolean
  'minimax.custom_voice_group': string
  'minimax.custom_voice_billing_model_id': string
  'minimax.emotion_pattern': string
  'minimax.emotion_redirect': string
  'minimax.tone_word_pattern': string
  'minimax.tone_word_redirect': string
  ModelPrice: string
  ModelRatio: string
  CacheRatio: string
  CreateCacheRatio: string
  CompletionRatio: string
  AudioRatio: string
  AudioCompletionRatio: string
  ContextPricing: string
  'tool_billing_setting.rules': string
  TopupGroupRatio: string
  GroupRatio: string
  UserUsableGroups: string
  GroupGroupRatio: string
  AutoGroups: string
  DefaultUseAutoGroup: boolean
  'group_ratio_setting.group_special_usable_group': string
  'channel_affinity_setting.enabled': boolean
  'channel_affinity_setting.switch_on_success': boolean
  'channel_affinity_setting.keep_on_channel_disabled': boolean
  'channel_affinity_setting.max_entries': number
  'channel_affinity_setting.default_ttl_seconds': number
  'channel_affinity_setting.rules': string
}

export type BillingSettings = {
  QuotaForNewUser: number
  PreConsumedQuota: number
  QuotaForInviter: number
  QuotaForInvitee: number
  TopUpLink: string
  'quota_setting.free_model_pre_consumed_quota': number
  ModelPrice: string
  ModelRatio: string
  CacheRatio: string
  CreateCacheRatio: string
  CompletionRatio: string
  AudioRatio: string
  AudioCompletionRatio: string
  ContextPricing: string
  'tool_billing_setting.rules': string
  TopupGroupRatio: string
  GroupRatio: string
  UserUsableGroups: string
  GroupGroupRatio: string
  AutoGroups: string
  DefaultUseAutoGroup: boolean
  'group_ratio_setting.group_special_usable_group': string
  ServerAddress: string
  PayAddress: string
  EpayId: string
  EpayKey: string
  Price: number
  MinTopUp: number
  CustomCallbackAddress: string
  PayMethods: string
  'payment_setting.amount_options': string
  'payment_setting.amount_discount': string
  StripeApiSecret: string
  StripeWebhookSecret: string
  StripePriceId: string
  StripeUnitPrice: number
  StripeMinTopUp: number
  StripePromotionCodesEnabled: boolean
  'checkin_setting.enabled': boolean
  'checkin_setting.min_quota': number
  'checkin_setting.max_quota': number
}

export type OperationsSettings = {
  RetryTimes: number
  AutomaticRetryEnabled: boolean
  DefaultCollapseSidebar: boolean
  ChannelDisableThreshold: string
  QuotaRemindThreshold: string
  AutomaticDisableChannelEnabled: boolean
  AutomaticEnableChannelEnabled: boolean
  AutomaticDisableKeywords: string
  AutomaticDisableStatusCodes: string
  AutomaticRetryStatusCodes: string
  'monitor_setting.auto_test_channel_enabled': boolean
  'monitor_setting.auto_test_channel_minutes': number
  SMTPServer: string
  SMTPPort: string
  SMTPAccount: string
  SMTPFrom: string
  SMTPToken: string
  SMTPSSLEnabled: boolean
  SMTPForceLoginAuthEnabled: boolean
  WorkerUrl: string
  WorkerValidKey: string
  WorkerAllowHttpImageRequestEnabled: boolean
  LogConsumeEnabled: boolean
  'performance_setting.disk_cache_enabled': boolean
  'performance_setting.disk_cache_threshold_mb': number
  'performance_setting.disk_cache_max_size_mb': number
  'performance_setting.disk_cache_path': string
  'performance_setting.monitor_enabled': boolean
  'performance_setting.monitor_cpu_threshold': number
  'performance_setting.monitor_memory_threshold': number
  'performance_setting.monitor_disk_threshold': number
}

export type SecuritySettings = {
  ModelRequestRateLimitEnabled: boolean
  ModelRequestRateLimitCount: number
  ModelRequestRateLimitSuccessCount: number
  ModelRequestRateLimitDurationMinutes: number
  ModelRequestRateLimitGroup: string
  CheckSensitiveEnabled: boolean
  CheckSensitiveOnPromptEnabled: boolean
  SensitiveWords: string
  'fetch_setting.enable_ssrf_protection': boolean
  'fetch_setting.allow_private_ip': boolean
  'fetch_setting.domain_filter_mode': boolean
  'fetch_setting.ip_filter_mode': boolean
  'fetch_setting.domain_list': string[]
  'fetch_setting.ip_list': string[]
  'fetch_setting.allowed_ports': string[]
  'fetch_setting.apply_ip_filter_for_domain': boolean
  'audit_setting.enabled': boolean
  'audit_setting.modules': string
  'audit_setting.record_ip': boolean
  'audit_setting.record_diff': boolean
}
