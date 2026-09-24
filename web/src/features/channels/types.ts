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
import { z } from 'zod'

// ============================================================================
// Channel Schema & Types
// ============================================================================

export const channelInfoSchema = z.object({
  is_multi_key: z.boolean().default(false),
  is_plan: z.boolean().default(false),
  plan_name: z.string().default(''),
  multi_key_size: z.number().default(0),
  multi_key_status_list: z.record(z.string(), z.number()).optional(),
  multi_key_disabled_reason: z.record(z.string(), z.string()).optional(),
  multi_key_disabled_time: z.record(z.string(), z.number()).optional(),
  multi_key_polling_index: z.number().default(0),
  multi_key_mode: z.enum(['random', 'polling']).default('random'),
})

export type ChannelInfo = z.infer<typeof channelInfoSchema>

export const channelSchema = z.object({
  id: z.number(),
  type: z.number(),
  key: z.string(),
  openai_organization: z.string().nullish(),
  test_model: z.string().nullish(),
  status: z.number(), // 1: enabled, 0: manual disabled, 2: auto disabled
  name: z.string(),
  weight: z.number().nullish(),
  created_time: z.number(),
  test_time: z.number(),
  response_time: z.number(), // in milliseconds
  base_url: z.string().nullish(),
  other: z.string().default(''),
  balance: z.number().default(0), // in USD
  balance_updated_time: z.number(),
  models: z.string().default(''),
  group: z.string().default('default'),
  used_quota: z.number().default(0),
  model_mapping: z.string().nullish(),
  status_code_mapping: z.string().nullish(),
  priority: z.number().nullish(),
  auto_ban: z.number().nullish(),
  other_info: z.string().default(''),
  tag: z.string().nullish(),
  setting: z.string().nullish(),
  param_override: z.string().nullish(),
  header_override: z.string().nullish(),
  remark: z.string().default(''),
  max_input_tokens: z.number().default(0),
  channel_info: channelInfoSchema.default({
    is_multi_key: false,
    is_plan: false,
    plan_name: '',
    multi_key_size: 0,
    multi_key_polling_index: 0,
    multi_key_mode: 'random',
  }),
  settings: z.string().default('{}'), // other_settings JSON
})

export type Channel = z.infer<typeof channelSchema>

// ============================================================================
// Channel Settings Types
// ============================================================================

export interface ChannelSettings {
  force_format?: boolean
  proxy?: string
  pass_through_body_enabled?: boolean
  pass_through_headers_enabled?: boolean
  openai_wire_api?: 'both' | 'chat' | 'responses'
}

export interface ChannelOtherSettings {
  azure_responses_version?: string
  vertex_key_type?: 'json' | 'api_key'
  openrouter_enterprise?: boolean
  aws_key_type?: 'ak_sk' | 'api_key'
  image_auto_convert_to_url_mode?: 'off' | 'mcp'
  allow_cache_control?: boolean
  allow_speed?: boolean
  allow_service_tier?: boolean
  disable_store?: boolean
  allow_safety_identifier?: boolean
  claude_beta_query?: boolean
}

// ============================================================================
// API Response Types
// ============================================================================

/** 内置渠道类型的 base_url 预设选项（value=渠道存储值，label_key=i18n key） */
export interface BuiltinUrlOption {
  value: string
  label_key: string
}

export interface BuiltinChannelUrlsResponse {
  success: boolean
  message?: string
  /** 渠道类型（字符串化的数字）→ 预设选项列表 */
  data?: Record<string, BuiltinUrlOption[]>
}

export interface GetChannelsResponse {
  success: boolean
  message?: string
  data?: {
    items: Channel[]
    total: number
    page: number
    page_size: number
    type_counts?: Record<string, number>
  }
}

export interface SearchChannelsResponse {
  success: boolean
  message?: string
  data?: {
    items: Channel[]
    total: number
    type_counts?: Record<string, number>
  }
}

export interface GetChannelResponse {
  success: boolean
  message?: string
  data?: Channel
}

export interface ChannelTestResponse {
  success: boolean
  message?: string
  error_code?: string
  /** Response duration in seconds (backend compatibility field). */
  time?: number
  data?: {
    /** Response duration in milliseconds. */
    response_time?: number
    error?: string
  }
}

export interface ChannelBalanceResponse {
  success: boolean
  message?: string
  balance?: number
  currency?: string
  granted_balance?: number
  topped_up_balance?: number
}

export interface FetchModelsResponse {
  success: boolean
  message?: string
  data?: unknown[]
}

export interface OpenRouterProviderInfo {
  slug: string
  name: string
  headquarters?: string
  privacy_policy_url?: string
}

export interface FetchProvidersResponse {
  success: boolean
  message?: string
  data?: OpenRouterProviderInfo[]
}

export type ProxyTestStatus = 'success' | 'invalid' | 'failed'

export interface ProxyTestResultData {
  status: ProxyTestStatus
  ip?: string
  message?: string
}

export interface ProxyTestResponse {
  success: boolean
  message?: string
  data?: ProxyTestResultData
}

export interface CopyChannelResponse {
  success: boolean
  message?: string
  data?: {
    id: number
  }
}

// ============================================================================
// Multi-Key Management Types
// ============================================================================

export interface KeyStatus {
  index: number
  status: number // 1: enabled, 2: manual disabled, 3: auto disabled
  disabled_time?: number
  reason?: string
  key_preview?: string
}

export type MultiKeyConfirmAction = {
  type:
    | 'enable'
    | 'disable'
    | 'delete'
    | 'enable-all'
    | 'disable-all'
    | 'delete-disabled'
  keyIndex?: number
}

export interface MultiKeyStatusResponse {
  success: boolean
  message?: string
  data?: {
    keys: KeyStatus[]
    total: number
    page: number
    page_size: number
    total_pages: number
    enabled_count: number
    manual_disabled_count: number
    auto_disabled_count: number
  }
}

// ============================================================================
// Plan Channel Types
// ============================================================================

export interface PlanLimitInfo {
  percentage?: number
  next_reset_time?: string
  status?: string
}

export interface PlanMcpToolDetail {
  name?: string
  usage?: number
}

export interface PlanMcpLimitInfo extends PlanLimitInfo {
  current_usage?: string
  tools?: PlanMcpToolDetail[]
}

export interface PlanTierInfo {
  name?: string
  percentage?: number
  used?: number
  limit?: number
  remaining?: number
  resets_at?: string
  status?: string
}

export interface PlanQuotaData {
  plan_name?: string
  plan_version?: string
  product_level?: string
  product_name?: string
  effective_date?: string
  expiry_date?: string
  auto_renew?: boolean
  weekly_limit?: PlanLimitInfo
  token_limit?: PlanLimitInfo
  mcp_tool_limit?: PlanMcpLimitInfo
  tiers?: PlanTierInfo[]
  credential?: 'valid' | 'expired' | 'error' | string
  quota_supported?: boolean
  channel_id?: number
  channel_name?: string
}

export interface PlanQuotaResponse {
  success: boolean
  message?: string
  data?: PlanQuotaData
}

export type GlmUsageType = 'model' | 'tool' | 'performance'

export interface GlmUsageParams {
  type?: GlmUsageType
  startTime?: string
  endTime?: string
}

export interface GlmRiskResponse {
  success: boolean
  message?: string
  data?: {
    is_risk?: boolean
    message?: string
  }
}

export interface GlmContactData {
  phone?: string
  name?: string
  raw_name?: string
  billing_month?: string
}

export interface GlmContactResponse {
  success: boolean
  message?: string
  data?: GlmContactData
}

/** 智谱账户资金报告指标，金额为人民币原值；上游无值字段为 null */
export interface GlmAccountReportData {
  balance?: number | null
  recharge_amount?: number | null
  give_amount?: number | null
  total_spend_amount?: number | null
  available_balance?: number | null
  frozen_balance?: number | null
}

export interface GlmAccountReportResponse {
  success: boolean
  message?: string
  data?: GlmAccountReportData
  /** 折算成 USD 后落库的渠道余额，用于同步刷新表格 */
  balance?: number
}

export interface GlmActivityDay {
  date?: string
  totalTokens?: number
  mcpCalls?: number
}

export interface GlmActivitySummary {
  totalTokens?: number
  peakDailyTokens?: number
  peakDailyTokensDate?: string
  totalUsageDurationMs?: number
  currentStreakDays?: number
  longestStreakDays?: number
}

export interface GlmPlanActivityData {
  summary?: GlmActivitySummary
  series?: GlmActivityDay[]
}

export interface GlmPlanActivityResponse {
  success: boolean
  message?: string
  data?: GlmPlanActivityData
}

// ============================================================================
// GLM Reset Card Types
// ============================================================================

export type GlmResetCardType = 'FIVE_HOUR' | 'WEEK'

export interface GlmResetCard {
  recordId?: number
  expireTime?: string
  available?: boolean
  priority?: boolean
}

export interface GlmResetCardListData {
  fiveHourResets?: GlmResetCard[]
  weekResets?: GlmResetCard[]
}

export interface GlmResetCardListResponse {
  success: boolean
  message?: string
  data?: GlmResetCardListData
}

export interface GlmResetCardUseParams {
  resetType: GlmResetCardType
  recordId: number
}

export interface GlmResetCardUseResponse {
  success: boolean
  message?: string
  msg?: string
}

// ============================================================================
// API Request Parameters
// ============================================================================

export type ChannelSortBy =
  | 'id'
  | 'name'
  | 'priority'
  | 'balance'
  | 'response_time'
  | 'test_time'

export type ChannelSortOrder = 'asc' | 'desc'

export interface GetChannelsParams {
  p?: number
  page_size?: number
  status?: string // 'enabled', 'disabled', or empty for all
  type?: number
  group?: string
  id_sort?: boolean
  tag_mode?: boolean
  sort_by?: ChannelSortBy
  sort_order?: ChannelSortOrder
}

export interface SearchChannelsParams {
  keyword?: string
  group?: string
  model?: string
  status?: string
  type?: number
  id?: number
  name?: string
  tag?: string
  id_sort?: boolean
  tag_mode?: boolean
  sort_by?: ChannelSortBy
  sort_order?: ChannelSortOrder
  p?: number
  page_size?: number
}

export interface ChannelTestParams {
  test_model?: string
}

export interface CopyChannelParams {
  suffix?: string
  reset_balance?: boolean
}

export interface MultiKeyManageParams {
  channel_id: number
  action:
    | 'get_key_status'
    | 'disable_key'
    | 'enable_key'
    | 'enable_all_keys'
    | 'disable_all_keys'
    | 'delete_key'
    | 'delete_disabled_keys'
  key_index?: number
  page?: number
  page_size?: number
  status?: number // 1=enabled, 2=manual_disabled, 3=auto_disabled
}

export interface BatchDeleteParams {
  ids: number[]
}

export interface BatchSetTagParams {
  ids: number[]
  tag: string | null
}

export interface TagOperationParams {
  tag: string
  new_tag?: string
  model_mapping?: string
  models?: string
  groups?: string
}

// ============================================================================
// Form Data Types
// ============================================================================

export interface ChannelFormData {
  name: string
  type: number
  base_url: string
  key: string
  openai_organization?: string
  models: string
  group: string
  model_mapping?: string
  priority?: number
  weight?: number
  test_model?: string
  auto_ban?: number
  status: number
  status_code_mapping?: string
  tag?: string
  remark?: string
  setting?: string
  param_override?: string
  header_override?: string
  settings?: string
  other?: string
  // Multi-key specific
  multi_key_mode?: 'single' | 'batch' | 'multi_to_single'
  multi_key_type?: 'random' | 'polling'
  batch_add_set_key_prefix_2_name?: boolean
}

// ============================================================================
// Add Channel Request (special structure)
// ============================================================================

export interface AddChannelRequest {
  mode: 'single' | 'batch' | 'multi_to_single'
  multi_key_mode?: 'random' | 'polling'
  batch_add_set_key_prefix_2_name?: boolean
  channel: Partial<Channel>
}

// ============================================================================
// Update Channel Request (top-level extras beyond Channel fields)
// ============================================================================

export interface UpdateChannelParams extends Partial<Channel> {
  /** 多密钥取用策略，编辑多密钥渠道时随更新提交以支持切换 */
  multi_key_mode?: 'random' | 'polling'
  /** 多密钥渠道编辑时新密钥的写入方式 */
  key_mode?: 'append' | 'replace'
}
