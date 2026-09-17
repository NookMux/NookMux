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
import { CHANNEL_STATUS } from '../constants'
import type { Channel, UpdateChannelParams } from '../types'

// ============================================================================
// Form Validation Schema
// ============================================================================

export const channelFormSchema = z
  .object({
    name: z.string().min(1, 'Channel name is required'),
    type: z.number().min(0, 'Channel type is required'),
    base_url: z.string().optional(),
    key: z.string(),
    openai_organization: z.string().optional(),
    models: z.string().min(1, 'At least one model is required'),
    group: z.array(z.string()).min(1, 'At least one group is required'),
    model_mapping: z.string().optional(),
    priority: z.number().optional(),
    weight: z.number().optional(),
    test_model: z.string().optional(),
    auto_ban: z.number().optional(),
    status: z.number(),
    status_code_mapping: z.string().optional(),
    tag: z.string().optional(),
    remark: z
      .string()
      .max(255, 'Remark must be less than 255 characters')
      .optional(),
    setting: z.string().optional(),
    param_override: z.string().optional(),
    header_override: z.string().optional(),
    settings: z.string().optional(),
    other: z.string().optional(),
    // Multi-key options (not sent to backend directly)
    multi_key_mode: z.enum(['single', 'batch', 'multi_to_single']).optional(),
    multi_key_type: z.enum(['random', 'polling']).optional(),
    batch_add_set_key_prefix_2_name: z.boolean().optional(),
    key_mode: z.enum(['append', 'replace']).optional(), // For editing multi-key channels
    // Channel extra settings (stored in setting JSON, not sent directly)
    force_format: z.boolean().optional(),
    proxy: z.string().optional(),
    pass_through_body_enabled: z.boolean().optional(),
    pass_through_headers_enabled: z.boolean().optional(),
    openai_wire_api: z.enum(['both', 'chat', 'responses']).optional(),
    // Type-specific settings (stored in settings JSON)
    is_enterprise_account: z.boolean().optional(), // OpenRouter specific
    vertex_key_type: z.enum(['json', 'api_key']).optional(), // Vertex AI specific
    aws_key_type: z.enum(['ak_sk', 'api_key']).optional(), // AWS specific
    azure_responses_version: z.string().optional(), // Azure specific
    image_auto_convert_to_url_mode: z.enum(['off', 'mcp']).optional(),
    // OpenRouter provider routing preferences (stored in settings JSON).
    // Tri-state booleans use '' (unset / follow client), 'true', 'false'.
    or_order: z.string().optional(), // comma-separated provider slugs, ordered
    or_only: z.string().optional(),
    or_ignore: z.string().optional(),
    or_allow_fallbacks: z.enum(['', 'true', 'false']).optional(),
    or_require_parameters: z.enum(['', 'true', 'false']).optional(),
    or_data_collection: z.enum(['', 'allow', 'deny']).optional(),
    or_zdr: z.enum(['', 'true', 'false']).optional(),
    or_enforce_distillable_text: z.enum(['', 'true', 'false']).optional(),
    or_quantizations: z.string().optional(), // comma-separated
    or_sort: z.enum(['', 'price', 'throughput', 'latency']).optional(),
    or_sort_partition: z.enum(['', 'model', 'none']).optional(),
    or_pref_min_throughput: z.string().optional(),
    or_pref_min_throughput_percentile: z
      .enum(['', 'p50', 'p75', 'p90', 'p99'])
      .optional(),
    or_pref_max_latency: z.string().optional(),
    or_pref_max_latency_percentile: z
      .enum(['', 'p50', 'p75', 'p90', 'p99'])
      .optional(),
    or_max_price_prompt: z.string().optional(),
    or_max_price_completion: z.string().optional(),
    or_max_price_request: z.string().optional(),
    or_max_price_image: z.string().optional(),
    // Field passthrough controls (stored in settings JSON)
    allow_cache_control: z.boolean().optional(), // Anthropic cache_control
    allow_speed: z.boolean().optional(), // Anthropic speed
    allow_service_tier: z.boolean().optional(), // OpenAI/Anthropic
    disable_store: z.boolean().optional(), // OpenAI only
    allow_safety_identifier: z.boolean().optional(), // OpenAI only
    claude_beta_query: z.boolean().optional(), // Anthropic: beta query passthrough
  })
  .superRefine((value, ctx) => {
    for (const field of OPENROUTER_NUMERIC_FIELDS) {
      const trimmed = (value[field] || '').trim()
      if (trimmed === '') continue
      const num = Number(trimmed)
      if (!(Number.isFinite(num) && num >= 0)) {
        ctx.addIssue({
          code: 'custom',
          path: [field],
          message: 'Must be a number >= 0',
        })
      }
    }
    for (const [numberField, percentileField] of OPENROUTER_PERCENTILE_PAIRS) {
      if (!value[percentileField]) continue
      const trimmed = (value[numberField] || '').trim()
      const num = trimmed === '' ? Number.NaN : Number(trimmed)
      if (!(Number.isFinite(num) && num >= 0)) {
        ctx.addIssue({
          code: 'custom',
          path: [numberField],
          message: 'A percentile selection requires a numeric value',
        })
      }
    }
  })

// OpenRouter routing numeric fields: non-empty values must be finite numbers
// >= 0, and selecting a percentile requires its paired number, so incomplete
// input fails loudly instead of being silently dropped from the payload.
const OPENROUTER_NUMERIC_FIELDS = [
  'or_pref_min_throughput',
  'or_pref_max_latency',
  'or_max_price_prompt',
  'or_max_price_completion',
  'or_max_price_request',
  'or_max_price_image',
] as const

const OPENROUTER_PERCENTILE_PAIRS = [
  ['or_pref_min_throughput', 'or_pref_min_throughput_percentile'],
  ['or_pref_max_latency', 'or_pref_max_latency_percentile'],
] as const

export type ChannelFormValues = z.infer<typeof channelFormSchema>

// ============================================================================
// Default Form Values
// ============================================================================

export const CHANNEL_FORM_DEFAULT_VALUES: ChannelFormValues = {
  name: '',
  type: 1,
  base_url: '',
  key: '',
  openai_organization: '',
  models: '',
  group: ['default'],
  model_mapping: '',
  priority: 0,
  weight: 0,
  test_model: '',
  auto_ban: 1,
  status: CHANNEL_STATUS.ENABLED,
  status_code_mapping: '',
  tag: '',
  remark: '',
  setting: '',
  param_override: '',
  header_override: '',
  settings: '{}',
  other: '',
  multi_key_mode: 'single',
  multi_key_type: 'random',
  batch_add_set_key_prefix_2_name: false,
  key_mode: 'append',
  // Channel extra settings
  force_format: false,
  proxy: '',
  pass_through_body_enabled: false,
  pass_through_headers_enabled: true,
  openai_wire_api: 'both',
  // Type-specific settings
  is_enterprise_account: false,
  vertex_key_type: 'json',
  aws_key_type: 'ak_sk',
  azure_responses_version: '',
  image_auto_convert_to_url_mode: 'off',
  // OpenRouter provider routing preferences
  or_order: '',
  or_only: '',
  or_ignore: '',
  or_allow_fallbacks: '',
  or_require_parameters: '',
  or_data_collection: '',
  or_zdr: '',
  or_enforce_distillable_text: '',
  or_quantizations: '',
  or_sort: '',
  or_sort_partition: '',
  or_pref_min_throughput: '',
  or_pref_min_throughput_percentile: '',
  or_pref_max_latency: '',
  or_pref_max_latency_percentile: '',
  or_max_price_prompt: '',
  or_max_price_completion: '',
  or_max_price_request: '',
  or_max_price_image: '',
  // Field passthrough controls
  allow_cache_control: false,
  allow_speed: false,
  allow_service_tier: false,
  disable_store: false,
  allow_safety_identifier: false,
  claude_beta_query: false,
}

// ============================================================================
// Transform Functions
// ============================================================================

/** Split a comma-separated slug list into trimmed, de-duplicated entries. */
function parseSlugList(value: string | undefined): string[] {
  return Array.from(
    new Set(
      (value || '')
        .split(',')
        .map((entry) => entry.trim())
        .filter(Boolean)
    )
  )
}

/** Parse an optional non-negative numeric form field; blank → undefined. */
function parseOptionalNumber(value: string | undefined): number | undefined {
  const trimmed = (value || '').trim()
  if (trimmed === '') return undefined
  const num = Number(trimmed)
  return Number.isFinite(num) ? num : undefined
}

type OpenRouterRoutingForm = Pick<
  ChannelFormValues,
  | 'or_order'
  | 'or_only'
  | 'or_ignore'
  | 'or_allow_fallbacks'
  | 'or_require_parameters'
  | 'or_data_collection'
  | 'or_zdr'
  | 'or_enforce_distillable_text'
  | 'or_quantizations'
  | 'or_sort'
  | 'or_sort_partition'
  | 'or_pref_min_throughput'
  | 'or_pref_min_throughput_percentile'
  | 'or_pref_max_latency'
  | 'or_pref_max_latency_percentile'
  | 'or_max_price_prompt'
  | 'or_max_price_completion'
  | 'or_max_price_request'
  | 'or_max_price_image'
>

const EMPTY_OPENROUTER_ROUTING_FORM: OpenRouterRoutingForm = {
  or_order: '',
  or_only: '',
  or_ignore: '',
  or_allow_fallbacks: '',
  or_require_parameters: '',
  or_data_collection: '',
  or_zdr: '',
  or_enforce_distillable_text: '',
  or_quantizations: '',
  or_sort: '',
  or_sort_partition: '',
  or_pref_min_throughput: '',
  or_pref_min_throughput_percentile: '',
  or_pref_max_latency: '',
  or_pref_max_latency_percentile: '',
  or_max_price_prompt: '',
  or_max_price_completion: '',
  or_max_price_request: '',
  or_max_price_image: '',
}

function triStateToBoolean(value: string | undefined): boolean | undefined {
  if (value === 'true') return true
  if (value === 'false') return false
  return undefined
}

/** Map a stored boolean back to the '' / 'true' / 'false' tri-state form value. */
function booleanToTriState(value: unknown): '' | 'true' | 'false' {
  if (value === true) return 'true'
  if (value === false) return 'false'
  return ''
}

/** Return the stored value when it is one of the allowed enum members, else ''. */
function pickEnum<T extends string>(
  value: unknown,
  allowed: readonly T[]
): T | '' {
  if (
    typeof value === 'string' &&
    (allowed as readonly string[]).includes(value)
  ) {
    return value as T
  }
  return ''
}

/** Read an OpenRouter threshold that may be a bare number or {pXX: value}. */
function parseThresholdField(threshold: unknown): {
  value: string
  percentile: string
} {
  if (typeof threshold === 'number') {
    return { value: String(threshold), percentile: '' }
  }
  if (threshold && typeof threshold === 'object') {
    const entries = Object.entries(threshold as Record<string, unknown>)
    if (entries.length === 1) {
      const [percentile, raw] = entries[0]
      if (typeof raw === 'number') {
        return { value: String(raw), percentile }
      }
    }
  }
  return { value: '', percentile: '' }
}

/** Map a stored openrouter_routing object back to form fields. */
function parseOpenRouterRoutingToForm(routing: unknown): OpenRouterRoutingForm {
  if (!routing || typeof routing !== 'object') {
    return { ...EMPTY_OPENROUTER_ROUTING_FORM }
  }
  const raw = routing as Record<string, unknown>
  const slugList = (value: unknown): string =>
    Array.isArray(value)
      ? value
          .filter((entry): entry is string => typeof entry === 'string')
          .join(', ')
      : ''
  const minThroughput = parseThresholdField(raw.preferred_min_throughput)
  const maxLatency = parseThresholdField(raw.preferred_max_latency)
  const maxPrice =
    raw.max_price && typeof raw.max_price === 'object'
      ? (raw.max_price as Record<string, unknown>)
      : {}
  const numberToString = (value: unknown): string =>
    typeof value === 'number' ? String(value) : ''

  return {
    or_order: slugList(raw.order),
    or_only: slugList(raw.only),
    or_ignore: slugList(raw.ignore),
    or_allow_fallbacks: booleanToTriState(raw.allow_fallbacks),
    or_require_parameters: booleanToTriState(raw.require_parameters),
    or_data_collection:
      raw.data_collection === 'allow' || raw.data_collection === 'deny'
        ? raw.data_collection
        : '',
    or_zdr: booleanToTriState(raw.zdr),
    or_enforce_distillable_text: booleanToTriState(
      raw.enforce_distillable_text
    ),
    or_quantizations: slugList(raw.quantizations),
    or_sort:
      raw.sort && typeof raw.sort === 'object'
        ? pickEnum((raw.sort as Record<string, unknown>).by, [
            'price',
            'throughput',
            'latency',
          ] as const)
        : '',
    or_sort_partition:
      raw.sort && typeof raw.sort === 'object'
        ? pickEnum((raw.sort as Record<string, unknown>).partition, [
            'model',
            'none',
          ] as const)
        : '',
    or_pref_min_throughput: minThroughput.value,
    or_pref_min_throughput_percentile: pickEnum(minThroughput.percentile, [
      'p50',
      'p75',
      'p90',
      'p99',
    ] as const),
    or_pref_max_latency: maxLatency.value,
    or_pref_max_latency_percentile: pickEnum(maxLatency.percentile, [
      'p50',
      'p75',
      'p90',
      'p99',
    ] as const),
    or_max_price_prompt: numberToString(maxPrice.prompt),
    or_max_price_completion: numberToString(maxPrice.completion),
    or_max_price_request: numberToString(maxPrice.request),
    or_max_price_image: numberToString(maxPrice.image),
  }
}

/** Build the openrouter_routing settings object; undefined when nothing is configured. */
function buildOpenRouterRouting(
  formData: ChannelFormValues
): Record<string, unknown> | undefined {
  const routing: Record<string, unknown> = {}

  const order = parseSlugList(formData.or_order)
  if (order.length > 0) routing.order = order
  const only = parseSlugList(formData.or_only)
  if (only.length > 0) routing.only = only
  const ignore = parseSlugList(formData.or_ignore)
  if (ignore.length > 0) routing.ignore = ignore

  const allowFallbacks = triStateToBoolean(formData.or_allow_fallbacks)
  if (allowFallbacks !== undefined) routing.allow_fallbacks = allowFallbacks
  const requireParameters = triStateToBoolean(formData.or_require_parameters)
  if (requireParameters !== undefined)
    routing.require_parameters = requireParameters
  const zdr = triStateToBoolean(formData.or_zdr)
  if (zdr !== undefined) routing.zdr = zdr
  const enforceDistillable = triStateToBoolean(
    formData.or_enforce_distillable_text
  )
  if (enforceDistillable !== undefined)
    routing.enforce_distillable_text = enforceDistillable

  if (formData.or_data_collection)
    routing.data_collection = formData.or_data_collection

  const quantizations = parseSlugList(formData.or_quantizations)
  if (quantizations.length > 0) routing.quantizations = quantizations

  if (formData.or_sort) {
    const sort: Record<string, unknown> = { by: formData.or_sort }
    if (formData.or_sort_partition) sort.partition = formData.or_sort_partition
    routing.sort = sort
  }

  const minThroughput = parseOptionalNumber(formData.or_pref_min_throughput)
  if (minThroughput !== undefined) {
    routing.preferred_min_throughput =
      formData.or_pref_min_throughput_percentile
        ? { [formData.or_pref_min_throughput_percentile]: minThroughput }
        : minThroughput
  }
  const maxLatency = parseOptionalNumber(formData.or_pref_max_latency)
  if (maxLatency !== undefined) {
    routing.preferred_max_latency = formData.or_pref_max_latency_percentile
      ? { [formData.or_pref_max_latency_percentile]: maxLatency }
      : maxLatency
  }

  const maxPrice: Record<string, number> = {}
  const prompt = parseOptionalNumber(formData.or_max_price_prompt)
  if (prompt !== undefined) maxPrice.prompt = prompt
  const completion = parseOptionalNumber(formData.or_max_price_completion)
  if (completion !== undefined) maxPrice.completion = completion
  const request = parseOptionalNumber(formData.or_max_price_request)
  if (request !== undefined) maxPrice.request = request
  const image = parseOptionalNumber(formData.or_max_price_image)
  if (image !== undefined) maxPrice.image = image
  if (Object.keys(maxPrice).length > 0) routing.max_price = maxPrice

  return Object.keys(routing).length > 0 ? routing : undefined
}

/**
 * Transform Channel from API to Form default values
 */
export function transformChannelToFormDefaults(
  channel: Channel
): ChannelFormValues {
  // Parse channel extra settings from setting field
  let extraSettings = {
    force_format: false,
    proxy: '',
    pass_through_body_enabled: false,
    pass_through_headers_enabled: true,
    openai_wire_api: 'both' as const,
  }

  if (channel.setting) {
    try {
      const parsed = JSON.parse(channel.setting)
      extraSettings = {
        force_format: parsed.force_format || false,
        proxy: parsed.proxy || '',
        pass_through_body_enabled: parsed.pass_through_body_enabled || false,
        pass_through_headers_enabled:
          parsed.pass_through_headers_enabled !== false,
        openai_wire_api:
          parsed.openai_wire_api === 'chat' ||
          parsed.openai_wire_api === 'responses'
            ? parsed.openai_wire_api
            : 'both',
      }
    } catch (error) {
      // eslint-disable-next-line no-console
      console.error('Failed to parse channel setting:', error)
    }
  }

  // Parse type-specific settings from settings field
  let vertexKeyType: 'json' | 'api_key' = 'json'
  let azureResponsesVersion = ''
  let isEnterpriseAccount = false
  let awsKeyType: 'ak_sk' | 'api_key' = 'ak_sk'
  let imageAutoConvertToUrlMode: 'off' | 'mcp' = 'off'
  let allowCacheControl = false
  let allowSpeed = false
  let allowServiceTier = false
  let disableStore = false
  let allowSafetyIdentifier = false
  let claudeBetaQuery = false
  let openRouterRoutingForm: OpenRouterRoutingForm = {
    ...EMPTY_OPENROUTER_ROUTING_FORM,
  }

  if (channel.settings) {
    try {
      const parsed = JSON.parse(channel.settings)
      vertexKeyType = parsed.vertex_key_type || 'json'
      azureResponsesVersion = parsed.azure_responses_version || ''
      isEnterpriseAccount = parsed.openrouter_enterprise === true
      awsKeyType = parsed.aws_key_type || 'ak_sk'
      imageAutoConvertToUrlMode =
        parsed.image_auto_convert_to_url_mode === 'mcp' ||
        parsed.image_auto_convert_to_url === true
          ? 'mcp'
          : 'off'
      allowServiceTier = parsed.allow_service_tier === true
      allowCacheControl = parsed.allow_cache_control === true
      allowSpeed = parsed.allow_speed === true
      disableStore = parsed.disable_store === true
      allowSafetyIdentifier = parsed.allow_safety_identifier === true
      claudeBetaQuery = parsed.claude_beta_query === true
      openRouterRoutingForm = parseOpenRouterRoutingToForm(
        parsed.openrouter_routing
      )
    } catch (error) {
      // eslint-disable-next-line no-console
      console.error('Failed to parse channel settings:', error)
    }
  }

  return {
    name: channel.name || '',
    type: channel.type,
    base_url: channel.base_url || '',
    key: '', // Never populate key from backend for security
    openai_organization: channel.openai_organization || '',
    models: channel.models || '',
    group: parseGroups(channel.group || 'default'),
    model_mapping: channel.model_mapping || '',
    priority: channel.priority || 0,
    weight: channel.weight || 0,
    test_model: channel.test_model || '',
    auto_ban: channel.auto_ban ?? 1,
    status: channel.status,
    status_code_mapping: channel.status_code_mapping || '',
    tag: channel.tag || '',
    remark: channel.remark || '',
    setting: channel.setting || '',
    param_override: channel.param_override || '',
    header_override: channel.header_override || '',
    settings: channel.settings || '{}',
    other: channel.other || '',
    multi_key_mode: 'single',
    multi_key_type: channel.channel_info.multi_key_mode || 'random',
    batch_add_set_key_prefix_2_name: false,
    key_mode: 'append', // Default to append mode for editing multi-key channels
    // Channel extra settings
    ...extraSettings,
    // Type-specific settings
    is_enterprise_account: isEnterpriseAccount,
    vertex_key_type: vertexKeyType,
    azure_responses_version: azureResponsesVersion,
    aws_key_type: awsKeyType,
    image_auto_convert_to_url_mode: imageAutoConvertToUrlMode,
    allow_cache_control: allowCacheControl,
    allow_speed: allowSpeed,
    allow_service_tier: allowServiceTier,
    disable_store: disableStore,
    claude_beta_query: claudeBetaQuery,
    allow_safety_identifier: allowSafetyIdentifier,
    // OpenRouter provider routing preferences
    ...openRouterRoutingForm,
  }
}

/**
 * Build the setting JSON string from form extra settings
 */
function buildSettingJSON(formData: ChannelFormValues): string {
  const settingObj = {
    force_format: formData.force_format || false,
    proxy: formData.proxy || '',
    pass_through_body_enabled: formData.pass_through_body_enabled || false,
    pass_through_headers_enabled:
      formData.pass_through_headers_enabled !== false,
    openai_wire_api: formData.openai_wire_api || 'both',
  }
  return JSON.stringify(settingObj)
}

/**
 * Build the settings JSON string (for type-specific config like vertex_key_type)
 */
function buildSettingsJSON(formData: ChannelFormValues): string {
  let settingsObj: Record<string, unknown> = {}

  // Try to parse existing settings first
  if (formData.settings && formData.settings !== '{}') {
    try {
      settingsObj = JSON.parse(formData.settings)
    } catch (error) {
      // eslint-disable-next-line no-console
      console.error('Failed to parse existing settings:', error)
    }
  }

  // Add vertex_key_type for Vertex AI channels (type 41)
  if (formData.type === 41) {
    settingsObj.vertex_key_type = formData.vertex_key_type || 'json'
  } else if ('vertex_key_type' in settingsObj) {
    delete settingsObj.vertex_key_type
  }

  // Add azure_responses_version for Azure channels (type 3)
  if (formData.type === 3 && formData.azure_responses_version) {
    settingsObj.azure_responses_version = formData.azure_responses_version
  } else if ('azure_responses_version' in settingsObj) {
    delete settingsObj.azure_responses_version
  }

  // Add enterprise account setting for OpenRouter (type 20)
  if (formData.type === 20) {
    settingsObj.openrouter_enterprise = formData.is_enterprise_account === true
  } else if ('openrouter_enterprise' in settingsObj) {
    delete settingsObj.openrouter_enterprise
  }

  // Add provider routing preferences for OpenRouter (type 20). Any configured
  // field overrides the same-named client field at relay time; an empty
  // configuration removes the key entirely (no intervention).
  if (formData.type === 20) {
    const routing = buildOpenRouterRouting(formData)
    if (routing) {
      settingsObj.openrouter_routing = routing
    } else {
      delete settingsObj.openrouter_routing
    }
  } else if ('openrouter_routing' in settingsObj) {
    delete settingsObj.openrouter_routing
  }

  // Add aws_key_type for AWS channels (type 33)
  if (formData.type === 33) {
    settingsObj.aws_key_type = formData.aws_key_type || 'ak_sk'
  } else if ('aws_key_type' in settingsObj) {
    delete settingsObj.aws_key_type
  }

  // Field passthrough controls:
  // - OpenAI (type 1) and Anthropic (type 14): allow_service_tier
  // - OpenAI only: disable_store, allow_safety_identifier
  if (formData.type === 1 || formData.type === 14) {
    settingsObj.allow_service_tier = formData.allow_service_tier === true
  } else if ('allow_service_tier' in settingsObj) {
    delete settingsObj.allow_service_tier
  }

  if (formData.type === 1) {
    settingsObj.disable_store = formData.disable_store === true
    settingsObj.allow_safety_identifier =
      formData.allow_safety_identifier === true
  } else {
    if ('disable_store' in settingsObj) delete settingsObj.disable_store
    if ('allow_safety_identifier' in settingsObj)
      delete settingsObj.allow_safety_identifier
  }

  // Anthropic (type 14): claude_beta_query
  if (formData.type === 14) {
    settingsObj.claude_beta_query = formData.claude_beta_query === true
    settingsObj.allow_cache_control = formData.allow_cache_control === true
    settingsObj.allow_speed = formData.allow_speed === true
  } else {
    if ('claude_beta_query' in settingsObj) delete settingsObj.claude_beta_query
    if ('allow_cache_control' in settingsObj)
      delete settingsObj.allow_cache_control
    if ('allow_speed' in settingsObj) delete settingsObj.allow_speed
  }

  if (formData.image_auto_convert_to_url_mode === 'mcp') {
    settingsObj.image_auto_convert_to_url_mode = 'mcp'
  } else {
    delete settingsObj.image_auto_convert_to_url_mode
  }
  delete settingsObj.image_auto_convert_to_url
  delete settingsObj.allow_include_obfuscation
  delete settingsObj.allow_inference_geo
  delete settingsObj.upstream_model_update_check_enabled
  delete settingsObj.upstream_model_update_auto_sync_enabled
  delete settingsObj.upstream_model_update_ignored_models
  delete settingsObj.upstream_model_update_last_check_time
  delete settingsObj.upstream_model_update_last_detected_models
  delete settingsObj.upstream_model_update_last_removed_models

  return JSON.stringify(settingsObj)
}

/**
 * Transform form data to API payload for creating channel
 */
export function transformFormDataToCreatePayload(formData: ChannelFormValues): {
  mode: 'single' | 'batch' | 'multi_to_single'
  multi_key_mode?: 'random' | 'polling'
  batch_add_set_key_prefix_2_name?: boolean
  channel: Partial<Channel>
} {
  const mode = formData.multi_key_mode || 'single'

  const channel: Partial<Channel> = {
    name: formData.name,
    type: formData.type,
    base_url: formData.base_url || null,
    key: formData.key,
    openai_organization: formData.openai_organization || null,
    models: formData.models,
    group: formatGroups(formData.group),
    model_mapping: formData.model_mapping || null,
    priority: formData.priority || null,
    weight: formData.weight || null,
    test_model: formData.test_model || null,
    auto_ban: formData.auto_ban ?? 1,
    status: formData.status,
    status_code_mapping: formData.status_code_mapping || null,
    tag: formData.tag || null,
    remark: formData.remark || '',
    setting: buildSettingJSON(formData),
    param_override: formData.param_override || null,
    header_override: formData.header_override || null,
    settings: buildSettingsJSON(formData),
    other: formData.other || '',
  }

  // Clean up empty strings to null for optional fields
  Object.keys(channel).forEach((key) => {
    if (channel[key as keyof typeof channel] === '') {
      ;(channel as Record<string, unknown>)[key] = null
    }
  })

  return {
    mode,
    multi_key_mode:
      mode === 'multi_to_single' ? formData.multi_key_type : undefined,
    batch_add_set_key_prefix_2_name:
      mode === 'batch' ? formData.batch_add_set_key_prefix_2_name : undefined,
    channel,
  }
}

/**
 * Transform form data to API payload for updating channel.
 * isMultiKey 为 true 时（编辑多密钥渠道）额外携带 multi_key_mode，
 * 使保存时可以切换 随机/轮询 取用策略。
 */
export function transformFormDataToUpdatePayload(
  formData: ChannelFormValues,
  channelId: number,
  isMultiKey = false
): UpdateChannelParams {
  const payload: UpdateChannelParams = {
    id: channelId,
    name: formData.name,
    type: formData.type,
    base_url: formData.base_url || null,
    openai_organization: formData.openai_organization || null,
    models: formData.models,
    group: formatGroups(formData.group),
    model_mapping: formData.model_mapping || null,
    priority: formData.priority ?? 0,
    weight: formData.weight ?? 0,
    test_model: formData.test_model || null,
    auto_ban: formData.auto_ban ?? 1,
    status: formData.status,
    status_code_mapping: formData.status_code_mapping || null,
    tag: formData.tag || null,
    remark: formData.remark || '',
    setting: buildSettingJSON(formData),
    param_override: formData.param_override || null,
    header_override: formData.header_override || null,
    settings: buildSettingsJSON(formData),
    other: formData.other || '',
  }

  // Only include key if it was changed (not empty)
  if (formData.key && formData.key.trim()) {
    payload.key = formData.key
  }

  // Multi-key channels carry their key strategy on every update so the
  // random/polling selector stays in effect after each save.
  if (isMultiKey) {
    payload.multi_key_mode = formData.multi_key_type || 'random'
  }

  // Clean up empty strings to null for optional fields
  Object.keys(payload).forEach((key) => {
    if (payload[key as keyof typeof payload] === '') {
      ;(payload as Record<string, unknown>)[key] = null
    }
  })

  // Send explicit empty strings for nullable fields so GORM updates can clear them.
  payload.base_url = formData.base_url || ''
  payload.openai_organization = formData.openai_organization || ''
  payload.test_model = formData.test_model || ''
  payload.tag = formData.tag || ''
  payload.remark = formData.remark || ''
  payload.model_mapping = formData.model_mapping || ''
  payload.status_code_mapping = formData.status_code_mapping || ''
  payload.param_override = formData.param_override || ''
  payload.header_override = formData.header_override || ''

  return payload
}

// ============================================================================
// Validation Helpers
// ============================================================================

/**
 * Validate JSON string
 */
export function validateJSON(value: string): boolean {
  if (!value || value.trim() === '') return true
  try {
    JSON.parse(value)
    return true
  } catch {
    return false
  }
}

/**
 * Validate model mapping format
 */
export function validateModelMapping(value: string): boolean {
  if (!value || value.trim() === '') return true
  return validateJSON(value)
}

/**
 * Parse models string to array
 */
export function parseModels(models: string): string[] {
  if (!models) return []
  return models
    .split(',')
    .map((m) => m.trim())
    .filter((m) => m.length > 0)
}

/**
 * Parse groups string to array
 */
export function parseGroups(groups: string): string[] {
  if (!groups) return []
  return groups
    .split(',')
    .map((g) => g.trim())
    .filter((g) => g.length > 0)
}

/**
 * Format models array to string
 */
export function formatModels(models: string[]): string {
  return models.join(',')
}

/**
 * Format groups array to string
 */
export function formatGroups(groups: string[]): string {
  return groups.join(',')
}
