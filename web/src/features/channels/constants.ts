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
// ============================================================================
// Channel Types (from constant/channel.go)
// All label/name values are i18n keys; use t(value) when displaying.
// ============================================================================

export const CHANNEL_TYPES = {
  0: 'Unknown',
  1: 'OpenAI',
  3: 'Azure',
  4: 'Ollama',
  6: 'Xiaomi',
  8: 'channels.types.custom',
  14: 'Anthropic',
  20: 'OpenRouter',
  24: 'Gemini',
  25: 'Moonshot',
  26: 'Zhipu V4',
  33: 'AWS',
  35: 'MiniMax',
  40: 'SiliconFlow',
  41: 'Vertex AI',
  43: 'DeepSeek',
  44: 'ByteDance',
} as const

const CHANNEL_TYPE_DISPLAY_ORDER: number[] = [
  14, 33, 3, 43, 24, 35, 25, 44, 4, 1, 20, 40, 41, 26, 6, 8,
]

export const CHANNEL_TYPE_OPTIONS: { value: number; label: string }[] = (() => {
  const ordered: { value: number; label: string }[] = []
  const seen = new Set<number>()
  for (const id of CHANNEL_TYPE_DISPLAY_ORDER) {
    const label = CHANNEL_TYPES[id as keyof typeof CHANNEL_TYPES]
    if (label) {
      ordered.push({ value: id, label })
      seen.add(id)
    }
  }
  for (const [key, label] of Object.entries(CHANNEL_TYPES)) {
    const id = Number(key)
    if (id !== 0 && !seen.has(id)) {
      ordered.push({ value: id, label })
    }
  }
  return ordered
})()

// ============================================================================
// Channel Status (label values are i18n keys; use t(config.label) in components)
// ============================================================================

export const CHANNEL_STATUS = {
  UNKNOWN: 0,
  ENABLED: 1,
  MANUAL_DISABLED: 2,
  AUTO_DISABLED: 3,
} as const

export const CHANNEL_STATUS_LABELS = {
  [CHANNEL_STATUS.UNKNOWN]: 'channels.fields.unknown',
  [CHANNEL_STATUS.ENABLED]: 'channels.status.enabled',
  [CHANNEL_STATUS.MANUAL_DISABLED]: 'channels.status.disabled',
  [CHANNEL_STATUS.AUTO_DISABLED]: 'channels.status.autoDisabled',
} as const

export const CHANNEL_STATUS_OPTIONS = [
  { value: 'all', label: 'channels.fields.allStatus' },
  { value: 'enabled', label: 'channels.status.enabled' },
  { value: 'disabled', label: 'channels.status.disabled' },
] as const

export const CHANNEL_STATUS_CONFIG = {
  [CHANNEL_STATUS.UNKNOWN]: {
    variant: 'neutral' as const,
    label: 'channels.fields.unknown',
  },
  [CHANNEL_STATUS.ENABLED]: {
    variant: 'success' as const,
    label: 'channels.status.enabled',
  },
  [CHANNEL_STATUS.MANUAL_DISABLED]: {
    variant: 'neutral' as const,
    label: 'channels.status.disabled',
  },
  [CHANNEL_STATUS.AUTO_DISABLED]: {
    variant: 'danger' as const,
    label: 'channels.status.autoDisabled',
  },
}

// ============================================================================
// Multi-Key Status
// ============================================================================

export const MULTI_KEY_STATUS = {
  ENABLED: 1,
  MANUAL_DISABLED: 2,
  AUTO_DISABLED: 3,
} as const

export const MULTI_KEY_STATUS_LABELS = {
  [MULTI_KEY_STATUS.ENABLED]: 'channels.status.enabled',
  [MULTI_KEY_STATUS.MANUAL_DISABLED]: 'channels.status.manualDisabled',
  [MULTI_KEY_STATUS.AUTO_DISABLED]: 'channels.status.autoDisabled',
} as const

export const MULTI_KEY_STATUS_CONFIG = {
  [MULTI_KEY_STATUS.ENABLED]: {
    variant: 'success' as const,
    label: 'channels.status.enabled',
  },
  [MULTI_KEY_STATUS.MANUAL_DISABLED]: {
    variant: 'neutral' as const,
    label: 'channels.status.manualDisabled',
  },
  [MULTI_KEY_STATUS.AUTO_DISABLED]: {
    variant: 'danger' as const,
    label: 'channels.status.autoDisabled',
  },
}

// ============================================================================
// Multi-Key Modes
// ============================================================================

export const MULTI_KEY_MODES = [
  { value: 'random', label: 'channels.fields.random' },
  { value: 'polling', label: 'channels.fields.polling' },
] as const

export const ADD_MODE_OPTIONS = [
  { value: 'single', label: 'common.fields.singleKey' },
  { value: 'batch', label: 'common.fields.batchAddOneKeyPerLine' },
  {
    value: 'multi_to_single',
    label: 'common.tips.multiKeyModeMultipleKeysOneChannel',
  },
] as const

// ============================================================================
// Multi-Key Management
// ============================================================================

export const MULTI_KEY_FILTER_OPTIONS = [
  { value: 'all', label: 'channels.fields.allStatus' },
  { value: '1', label: 'channels.status.enabled' },
  { value: '2', label: 'channels.status.manualDisabled' },
  { value: '3', label: 'channels.status.autoDisabled' },
] as const

export const MULTI_KEY_CONFIRM_MESSAGES = {
  DELETE: 'common.errors.sureYouWantToDeleteThisKeyThisAction',
  ENABLE: 'common.actions.enableThisKey',
  DISABLE: 'common.actions.disableThisKey',
  ENABLE_ALL: 'common.tips.sureYouWantToEnableAllKeys',
  DISABLE_ALL: 'common.status.sureYouWantToDisableAllEnabledKeys',
  DELETE_DISABLED: 'common.errors.sureYouWantToDeleteAllAutoDisabledKeys',
} as const

// ============================================================================
// Auto Ban Options
// ============================================================================

export const AUTO_BAN_OPTIONS = [
  { value: 1, label: 'channels.status.enabled' },
  { value: 0, label: 'channels.status.disabled' },
] as const

// ============================================================================
// Error / Success Messages (i18n keys: use t(ERROR_MESSAGES.xxx) when displaying)
// ============================================================================

export const ERROR_MESSAGES = {
  REQUIRED_NAME: 'common.errors.channelNameIsRequired',
  REQUIRED_TYPE: 'common.errors.channelTypeIsRequired',
  REQUIRED_KEY: 'dashboard.fields.apiKeyRequired',
  REQUIRED_MODELS: 'common.titles.modelsAreRequired',
  REQUIRED_GROUP: 'common.errors.groupIsRequired',
  INVALID_JSON: 'common.errors.invalidJsonFormat',
  INVALID_MODEL_MAPPING: 'common.errors.invalidModelMappingFormat',
  CREATE_FAILED: 'common.errors.failedToCreateChannel',
  UPDATE_FAILED: 'common.errors.failedToUpdateChannel',
  DELETE_FAILED: 'common.errors.failedToDeleteChannel',
  TEST_FAILED: 'common.errors.failedToTestChannel',
  BALANCE_QUERY_FAILED: 'channels.errors.failedToQueryBalance',
  FETCH_MODELS_FAILED: 'channels.errors.failedToFetchModels',
} as const

export const SUCCESS_MESSAGES = {
  CREATED: 'common.status.channelCreatedSuccessfully',
  UPDATED: 'common.status.channelUpdatedSuccessfully',
  DELETED: 'common.status.channelDeletedSuccessfully',
  ENABLED: 'common.status.channelEnabledSuccessfully',
  DISABLED: 'common.status.channelDisabledSuccessfully',
  TESTED: 'common.fields.channelTestCompleted',
  BALANCE_QUERIED: 'common.fields.balanceQueriedSuccessfully',
  MODELS_FETCHED: 'common.titles.modelsFetchedSuccessfully',
  COPIED: 'common.status.channelCopiedSuccessfully',
  TAG_SET: 'common.fields.tagSetSuccessfully',
  BATCH_DELETED: 'common.status.channelsDeletedSuccessfully',
} as const

// ============================================================================
// Default Values
// ============================================================================

export const DEFAULT_PAGE_SIZE = 20

export const DEFAULT_CHANNEL_VALUES = {
  name: '',
  type: 0,
  base_url: '',
  key: '',
  models: '',
  group: 'default',
  status: CHANNEL_STATUS.ENABLED,
  priority: 0,
  weight: 0,
  auto_ban: 1,
  remark: '',
} as const

// ============================================================================
// Table Configuration
// ============================================================================

export const CHANNELS_TABLE_PAGE_SIZE_OPTIONS = [10, 20, 50, 100]

// ============================================================================
// Sort Options (label values are i18n keys)
// ============================================================================

export const SORT_OPTIONS = [
  { value: 'priority', label: 'common.fields.priorityDefault' },
  { value: 'id', label: 'channels.fields.id' },
  { value: 'name', label: 'channels.fields.name' },
  { value: 'balance', label: 'usageLogs.fields.balance' },
  { value: 'response_time', label: 'usageLogs.fields.responseTime' },
] as const

// ============================================================================
// Balance Display
// ============================================================================

export const BALANCE_THRESHOLDS = {
  LOW: 1,
  MEDIUM: 10,
  HIGH: 100,
} as const

// ============================================================================
// Response Time Thresholds (in ms)
// ============================================================================

export const RESPONSE_TIME_THRESHOLDS = {
  EXCELLENT: 500,
  GOOD: 1000,
  FAIR: 2000,
  POOR: 5000,
} as const

export const RESPONSE_TIME_CONFIG = {
  EXCELLENT: { variant: 'success' as const, label: 'common.fields.excellent' },
  GOOD: { variant: 'info' as const, label: 'common.fields.good' },
  FAIR: { variant: 'warning' as const, label: 'common.fields.fair' },
  POOR: { variant: 'danger' as const, label: 'common.fields.poor' },
  UNKNOWN: { variant: 'neutral' as const, label: 'channels.fields.notTested' },
} as const

// ============================================================================
// Field Hints and Placeholders (i18n keys; use t() when displaying)
// ============================================================================

export const FIELD_PLACEHOLDERS = {
  NAME: 'common.placeholders.eGOpenAiGpt4Production',
  BASE_URL: 'common.fields.leaveEmptyToUseDefault',
  KEY: 'common.tips.apiKeyOnePerLineForBatchMode',
  MODELS: 'common.tips.commaSeparatedModelNamesEGGpt4Gpt',
  GROUP: 'common.errors.pleaseSelectUserGroupsThatCanAccessThisChannel',
  MODEL_MAPPING: '{"request_model": "actual_model"}',
  TEST_MODEL: 'common.fields.modelToUseForTesting',
  TAG: 'common.tips.optionalTagForGroupingChannels',
  REMARK: 'common.tips.optionalNotesAboutThisChannel',
  PARAM_OVERRIDE: '{"temperature": 0.7}',
  HEADER_OVERRIDE: '{"X-Custom-Header": "value"}',
  STATUS_CODE_MAPPING: '{"400": "500"}',
} as const

export const FIELD_DESCRIPTIONS = {
  NAME: 'common.tips.friendlyNameToIdentifyThisChannel',
  TYPE: 'common.tips.providerTypeOpenAiAnthropicEtc',
  BASE_URL: 'common.tips.customApiBaseUrlLeaveEmptyToUseProvider',
  KEY: 'common.fields.apiKeyFromTheProvider',
  MODELS: 'common.tips.listOfModelsSupportedByThisChannelUseComma',
  GROUP: 'common.tips.userGroupsThatCanAccessThisChannel',
  MODEL_MAPPING: 'common.tips.mapRequestModelNamesToActualProviderModelNames',
  PRIORITY: 'common.tips.higherPriorityChannelsAreSelectedFirst',
  WEIGHT: 'common.status.usedForLoadBalancingHigherWeightMoreRequests',
  TEST_MODEL: 'common.tips.modelToUseWhenTestingChannelConnectivity',
  AUTO_BAN: 'common.tips.automaticallyDisableChannelOnRepeatedFailures',
  STATUS_CODE_MAPPING: 'common.tips.mapResponseStatusCodesJsonFormat',
  TAG: 'common.tips.groupChannelsByTagForBatchOperations',
  REMARK: 'common.tips.internalNotesNotShownToUsers',
  SETTING: 'common.tips.channelSpecificSettingsJsonFormat',
  PARAM_OVERRIDE: 'common.tips.overrideRequestParametersJsonFormat',
  HEADER_OVERRIDE: 'common.tips.overrideRequestHeadersJsonFormat',
  MULTI_KEY_MODE: 'common.tips.howToSelectKeysRandomOrSequentialPolling',
  BATCH_ADD: 'common.actions.createMultipleChannelsFromMultipleKeys',
  OPENAI_ORG: 'common.tips.openAiOrganizationIdOptional',
  OR_ALLOW_FALLBACKS: 'channels.tips.orAllowFallbacksDescription',
  OR_REQUIRE_PARAMETERS: 'channels.tips.orRequireParametersDescription',
  OR_DATA_COLLECTION: 'channels.tips.orDataCollectionDescription',
  OR_QUANTIZATIONS: 'channels.tips.orQuantizationsDescription',
  OR_ORDER: 'channels.tips.orOrderDescription',
  OR_ONLY: 'channels.tips.orOnlyDescription',
  OR_IGNORE: 'channels.tips.orIgnoreDescription',
  OR_SORT: 'channels.tips.orSortDescription',
  OR_PREF_MIN_THROUGHPUT: 'channels.tips.orPrefMinThroughputDescription',
  OR_PREF_MAX_LATENCY: 'channels.tips.orPrefMaxLatencyDescription',
} as const

// ============================================================================
// Channel Type Specific Configurations
// ============================================================================

export const MODEL_FETCHABLE_TYPES = new Set([
  1, 4, 14, 20, 24, 25, 26, 33, 40, 43, 44,
])

export const TYPE_TO_KEY_PROMPT: Record<number, string> = {
  33: 'Format: Ak|Sk|Region',
}

export const CHANNEL_TYPE_WARNINGS: Record<number, string> = {
  3: 'For channels added after May 10, 2025, no need to remove "." from model names during deployment',
  8: 'If connecting to upstream One API or New API relay projects, use OpenAI type instead unless you know what you are doing',
}
