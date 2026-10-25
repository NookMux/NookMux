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

// 使用日志详情弹窗字段 key 常量。
// 必须与后端 setting/console_setting/config.go 中的 UsageLogField* 常量保持一致。
// 仅包含详情弹窗独有字段；同时出现在列表表格列和详情弹窗中的字段
// （channel/token/group/response_time/content）不在配置范围内。
// 必须与后端 UsageLogField* 常量保持一致。
export type UsageLogFieldKey =
  | 'request_id'
  | 'upstream_request_id'
  | 'retry_chain'
  | 'ip_address'
  | 'client_headers'
  | 'request_conversion'
  | 'reasoning_effort'
  | 'system_prompt_override'
  | 'model_mapping'
  | 'parameter_override'
  | 'billing_source'
  | 'billing_details'
  | 'price_table'
  | 'tiered_pricing'
  | 'violation_fee'
  | 'refund_details'
  | 'subscription_billing'
  | 'token_breakdown'
  | 'audio_tokens'
  | 'topup_audit'
  | 'operator_admin'
  | 'stream_status'

// 字段默认可见性元数据：key → { nameKey, descriptionKey, group, admin, user }
// nameKey / descriptionKey 为 i18n 翻译 key，在渲染时通过 t() 解析。
// 默认值严格映射 details-dialog.tsx 中现有的 isAdmin 条件渲染逻辑。
export interface UsageLogFieldMeta {
  key: UsageLogFieldKey
  nameKey: string
  descriptionKey: string
  group: UsageLogFieldGroup
  admin: boolean
  user: boolean
}

export type UsageLogFieldGroup =
  | 'basic'
  | 'request'
  | 'billing'
  | 'token'
  | 'system'
  | 'other'

export const USAGE_LOG_FIELD_GROUPS: {
  value: UsageLogFieldGroup
  labelKey: string
}[] = [
  { value: 'basic', labelKey: 'common.fields.usageLogFieldGroupBasic' },
  { value: 'request', labelKey: 'common.fields.usageLogFieldGroupRequest' },
  { value: 'billing', labelKey: 'common.fields.usageLogFieldGroupBilling' },
  { value: 'token', labelKey: 'common.fields.usageLogFieldGroupToken' },
  { value: 'system', labelKey: 'common.titles.usageLogFieldGroupSystem' },
  { value: 'other', labelKey: 'common.fields.usageLogFieldGroupOther' },
]

export const USAGE_LOG_FIELDS: UsageLogFieldMeta[] = [
  // 基本信息
  {
    key: 'request_id',
    nameKey: 'common.tips.usageLogFieldNameRequestId',
    descriptionKey: 'common.tips.usageLogFieldDescRequestId',
    group: 'basic',
    admin: true,
    user: true,
  },
  {
    key: 'upstream_request_id',
    nameKey: 'common.tips.usageLogFieldNameUpstreamRequestId',
    descriptionKey: 'common.tips.usageLogFieldDescUpstreamRequestId',
    group: 'basic',
    admin: true,
    user: true,
  },
  {
    key: 'retry_chain',
    nameKey: 'common.tips.usageLogFieldNameRetryChain',
    descriptionKey: 'common.tips.usageLogFieldDescRetryChain',
    group: 'basic',
    admin: true,
    user: false,
  },
  {
    key: 'ip_address',
    nameKey: 'common.tips.usageLogFieldNameIpAddress',
    descriptionKey: 'common.tips.usageLogFieldDescIpAddress',
    group: 'basic',
    admin: true,
    user: true,
  },
  // 请求信息
  {
    key: 'client_headers',
    nameKey: 'common.tips.usageLogFieldNameClientHeaders',
    descriptionKey: 'common.tips.usageLogFieldDescClientHeaders',
    group: 'request',
    admin: true,
    user: true,
  },
  {
    key: 'request_conversion',
    nameKey: 'common.tips.usageLogFieldNameRequestConversion',
    descriptionKey: 'common.tips.usageLogFieldDescRequestConversion',
    group: 'request',
    admin: true,
    user: false,
  },
  {
    key: 'reasoning_effort',
    nameKey: 'common.tips.usageLogFieldNameReasoningEffort',
    descriptionKey: 'common.tips.usageLogFieldDescReasoningEffort',
    group: 'request',
    admin: true,
    user: true,
  },
  {
    key: 'system_prompt_override',
    nameKey: 'common.tips.usageLogFieldNameSystemPromptOverride',
    descriptionKey: 'common.tips.usageLogFieldDescSystemPromptOverride',
    group: 'request',
    admin: true,
    user: true,
  },
  {
    key: 'model_mapping',
    nameKey: 'common.tips.usageLogFieldNameModelMapping',
    descriptionKey: 'common.tips.usageLogFieldDescModelMapping',
    group: 'request',
    admin: true,
    user: true,
  },
  {
    key: 'parameter_override',
    nameKey: 'common.tips.usageLogFieldNameParameterOverride',
    descriptionKey: 'common.tips.usageLogFieldDescParameterOverride',
    group: 'request',
    admin: true,
    user: true,
  },
  // 计费
  {
    key: 'billing_source',
    nameKey: 'common.tips.usageLogFieldNameBillingSource',
    descriptionKey: 'common.tips.usageLogFieldDescBillingSource',
    group: 'billing',
    admin: true,
    user: false,
  },
  {
    key: 'billing_details',
    nameKey: 'common.tips.usageLogFieldNameBillingDetails',
    descriptionKey: 'common.tips.usageLogFieldDescBillingDetails',
    group: 'billing',
    admin: true,
    user: true,
  },
  {
    key: 'price_table',
    nameKey: 'common.tips.usageLogFieldNamePriceTable',
    descriptionKey: 'common.tips.usageLogFieldDescPriceTable',
    group: 'billing',
    admin: true,
    user: true,
  },
  {
    key: 'tiered_pricing',
    nameKey: 'common.tips.usageLogFieldNameTieredPricing',
    descriptionKey: 'common.tips.usageLogFieldDescTieredPricing',
    group: 'billing',
    admin: true,
    user: true,
  },
  {
    key: 'violation_fee',
    nameKey: 'common.tips.usageLogFieldNameViolationFee',
    descriptionKey: 'common.tips.usageLogFieldDescViolationFee',
    group: 'billing',
    admin: true,
    user: true,
  },
  {
    key: 'refund_details',
    nameKey: 'common.tips.usageLogFieldNameRefundDetails',
    descriptionKey: 'common.tips.usageLogFieldDescRefundDetails',
    group: 'billing',
    admin: true,
    user: true,
  },
  {
    key: 'subscription_billing',
    nameKey: 'common.tips.usageLogFieldNameSubscriptionBilling',
    descriptionKey: 'common.tips.usageLogFieldDescSubscriptionBilling',
    group: 'billing',
    admin: true,
    user: true,
  },
  // Token
  {
    key: 'token_breakdown',
    nameKey: 'common.tips.usageLogFieldNameTokenBreakdown',
    descriptionKey: 'common.tips.usageLogFieldDescTokenBreakdown',
    group: 'token',
    admin: true,
    user: true,
  },
  {
    key: 'audio_tokens',
    nameKey: 'common.tips.usageLogFieldNameAudioTokens',
    descriptionKey: 'common.tips.usageLogFieldDescAudioTokens',
    group: 'token',
    admin: true,
    user: true,
  },
  // 系统/管理
  {
    key: 'topup_audit',
    nameKey: 'common.tips.usageLogFieldNameTopupAudit',
    descriptionKey: 'common.tips.usageLogFieldDescTopupAudit',
    group: 'system',
    admin: true,
    user: false,
  },
  {
    key: 'operator_admin',
    nameKey: 'common.tips.usageLogFieldNameOperatorAdmin',
    descriptionKey: 'common.tips.usageLogFieldDescOperatorAdmin',
    group: 'system',
    admin: true,
    user: false,
  },
  {
    key: 'stream_status',
    nameKey: 'common.tips.usageLogFieldNameStreamStatus',
    descriptionKey: 'common.tips.usageLogFieldDescStreamStatus',
    group: 'system',
    admin: true,
    user: false,
  },
]

// 构建 UsageLogFields 配置的默认 JSON 字符串。
// 格式：{ "<fieldKey>": { "admin": true, "user": false }, ... }
export function buildDefaultUsageLogFieldsJSON(): string {
  const m: Record<string, { admin: boolean; user: boolean }> = {}
  for (const f of USAGE_LOG_FIELDS) {
    m[f.key] = { admin: f.admin, user: f.user }
  }
  return JSON.stringify(m)
}

// 解析 UsageLogFields 配置 JSON 字符串为 map。
// 如果配置为空或解析失败，返回 null，由调用方决定是否使用默认值。
function parseUsageLogFieldsConfig(
  raw: string
): Record<string, { admin: boolean; user: boolean }> | null {
  if (!raw) return null
  try {
    const parsed = JSON.parse(raw)
    if (parsed && typeof parsed === 'object' && !Array.isArray(parsed)) {
      return parsed as Record<string, { admin: boolean; user: boolean }>
    }
  } catch {
    // fallthrough
  }
  return null
}

// 根据配置 map 和字段元数据构建完整的字段可见性状态。
// 如果配置中缺少某字段，使用 USAGE_LOG_FIELDS 中的默认值。
export function resolveUsageLogFieldsConfig(
  raw: string
): Record<string, { admin: boolean; user: boolean }> {
  const parsed = parseUsageLogFieldsConfig(raw)
  if (!parsed) {
    // 使用默认值
    const defaults: Record<string, { admin: boolean; user: boolean }> = {}
    for (const f of USAGE_LOG_FIELDS) {
      defaults[f.key] = { admin: f.admin, user: f.user }
    }
    return defaults
  }
  // 合并默认值，确保新字段有默认值
  const merged: Record<string, { admin: boolean; user: boolean }> = {}
  for (const f of USAGE_LOG_FIELDS) {
    const cfg = parsed[f.key]
    merged[f.key] = {
      admin: cfg?.admin ?? f.admin,
      user: cfg?.user ?? f.user,
    }
  }
  return merged
}
