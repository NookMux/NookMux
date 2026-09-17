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
import type { TFunction } from 'i18next'
import { parseQuotaFromDollars, quotaUnitsToDollars } from '@/lib/format'
import { DEFAULT_GROUP } from '../constants'
import { type ApiKeyFormData, type ApiKey } from '../types'

// ============================================================================
// Form Schema
// ============================================================================

export function getApiKeyFormSchema(t: TFunction) {
  return z
    .object({
      name: z.string().min(1, t('keys.errors.pleaseEnterAName')),
      quota_type: z.number().min(0).max(3),
      remain_quota_dollars: z.number().optional(),
      expired_time: z.date().optional(),
      unlimited_quota: z.boolean(),
      window_hours: z.number().optional(),
      window_quota_dollars: z.number().optional(),
      window_start_hour: z.number().optional(),
      cycle_days: z.number().optional(),
      cycle_quota_dollars: z.number().optional(),
      model_limits: z.array(z.string()),
      model_mapping: z.string().optional(),
      allow_ips: z.string().optional(),
      group: z.string().optional(),
      cross_group_retry: z.boolean().optional(),
      tokenCount: z.number().min(1).optional(),
    })
    .superRefine((data, ctx) => {
      const modelMapping = (data.model_mapping || '').trim()
      if (modelMapping) {
        // 与后端 validateTokenModelMapping 对齐：JSON 对象且键值均为
        // 非空字符串、不含冒号/斜杠/空白（这些字符无法安全承载在
        // Gemini 路径等改写来源中），单键值不超过 256 字符、总量
        // 不超过 64KB。
        let tooLong = false
        let valid: boolean
        try {
          const parsed: unknown = JSON.parse(modelMapping)
          const entries = Object.entries(
            (parsed ?? {}) as Record<string, unknown>
          )
          tooLong =
            entries.some(
              ([from, to]) =>
                typeof to === 'string' && (from.length > 256 || to.length > 256)
            ) || modelMapping.length > 64 * 1024
          valid =
            !!parsed &&
            typeof parsed === 'object' &&
            !Array.isArray(parsed) &&
            entries.every(
              ([from, to]) =>
                typeof to === 'string' &&
                from.trim() !== '' &&
                to.trim() !== '' &&
                !/[:/\s]/.test(from) &&
                !/[:/\s]/.test(to)
            )
        } catch {
          valid = false
        }
        if (!valid) {
          ctx.addIssue({
            code: 'custom',
            path: ['model_mapping'],
            message: t('keys.errors.invalidModelMapping'),
          })
        } else if (tooLong) {
          ctx.addIssue({
            code: 'custom',
            path: ['model_mapping'],
            message: t('keys.errors.modelMappingEntryTooLong'),
          })
        }
      }

      if (data.quota_type === 0) {
        return
      }

      if (data.quota_type === 1 && (data.remain_quota_dollars ?? -1) < 0) {
        ctx.addIssue({
          code: 'custom',
          path: ['remain_quota_dollars'],
          message: t('keys.errors.quotaMustBeZeroOrGreater'),
        })
      }

      if (data.quota_type === 2 || data.quota_type === 3) {
        if ((data.window_hours ?? 0) < 1) {
          ctx.addIssue({
            code: 'custom',
            path: ['window_hours'],
            message: t('keys.errors.windowDurationMustBeAtLeast1Hour'),
          })
        }
        if ((data.window_quota_dollars ?? 0) <= 0) {
          ctx.addIssue({
            code: 'custom',
            path: ['window_quota_dollars'],
            message: t('keys.errors.windowQuotaMustBeGreaterThanZero'),
          })
        }
        const startHour = data.window_start_hour ?? 0
        if (startHour < 0 || startHour > 23) {
          ctx.addIssue({
            code: 'custom',
            path: ['window_start_hour'],
            message: t('keys.errors.windowStartHourMustBeBetween0And23'),
          })
        }
      }

      if (data.quota_type === 3) {
        if ((data.cycle_days ?? 0) < 1) {
          ctx.addIssue({
            code: 'custom',
            path: ['cycle_days'],
            message: t('keys.errors.cycleDurationMustBeAtLeast1Day'),
          })
        }
        if ((data.cycle_quota_dollars ?? 0) <= 0) {
          ctx.addIssue({
            code: 'custom',
            path: ['cycle_quota_dollars'],
            message: t('keys.errors.cycleQuotaMustBeGreaterThanZero'),
          })
        }
      }
    })
}

export type ApiKeyFormValues = z.infer<ReturnType<typeof getApiKeyFormSchema>>

// ============================================================================
// Form Defaults
// ============================================================================

export const API_KEY_FORM_DEFAULT_VALUES: ApiKeyFormValues = {
  name: '',
  quota_type: 0,
  remain_quota_dollars: 10,
  expired_time: undefined,
  unlimited_quota: true,
  window_hours: 1,
  window_quota_dollars: 10,
  window_start_hour: 0,
  cycle_days: 1,
  cycle_quota_dollars: 10,
  model_limits: [],
  model_mapping: '',
  allow_ips: '',
  group: DEFAULT_GROUP,
  cross_group_retry: true,
  tokenCount: 1,
}

export function getApiKeyFormDefaultValues(
  defaultUseAutoGroup: boolean
): ApiKeyFormValues {
  return {
    ...API_KEY_FORM_DEFAULT_VALUES,
    group: defaultUseAutoGroup ? 'auto' : DEFAULT_GROUP,
    cross_group_retry: defaultUseAutoGroup,
  }
}

// ============================================================================
// Form Data Transformation
// ============================================================================

/**
 * Transform form data to API payload
 */
export function transformFormDataToPayload(
  data: ApiKeyFormValues
): ApiKeyFormData {
  const quotaType = data.quota_type

  return {
    name: data.name,
    remain_quota:
      quotaType === 1
        ? parseQuotaFromDollars(data.remain_quota_dollars || 0)
        : 0,
    expired_time: data.expired_time
      ? Math.floor(data.expired_time.getTime() / 1000)
      : -1,
    unlimited_quota: quotaType === 0,
    model_limits_enabled: data.model_limits.length > 0,
    model_limits: data.model_limits.join(','),
    model_mapping: (data.model_mapping || '').trim(),
    allow_ips: data.allow_ips || '',
    group: data.group || '',
    cross_group_retry: data.group === 'auto' ? !!data.cross_group_retry : false,
    quota_type: quotaType,
    window_hours: quotaType >= 2 ? data.window_hours || 1 : 0,
    window_quota:
      quotaType >= 2
        ? parseQuotaFromDollars(data.window_quota_dollars || 0)
        : 0,
    window_start_hour: quotaType >= 2 ? data.window_start_hour || 0 : 0,
    cycle_days: quotaType === 3 ? data.cycle_days || 1 : 0,
    cycle_quota:
      quotaType === 3
        ? parseQuotaFromDollars(data.cycle_quota_dollars || 0)
        : 0,
  }
}

/**
 * Transform API key data to form defaults
 */
export function transformApiKeyToFormDefaults(
  apiKey: ApiKey
): ApiKeyFormValues {
  const quotaType = apiKey.quota_type || (apiKey.unlimited_quota ? 0 : 1)

  return {
    name: apiKey.name,
    quota_type: quotaType,
    remain_quota_dollars: apiKey.unlimited_quota
      ? 0
      : quotaUnitsToDollars(apiKey.remain_quota),
    expired_time:
      apiKey.expired_time > 0
        ? new Date(apiKey.expired_time * 1000)
        : undefined,
    unlimited_quota: quotaType === 0,
    window_hours: apiKey.window_hours || 1,
    window_quota_dollars: quotaUnitsToDollars(apiKey.window_quota || 0),
    window_start_hour: apiKey.window_start_hour || 0,
    cycle_days: apiKey.cycle_days || 1,
    cycle_quota_dollars: quotaUnitsToDollars(apiKey.cycle_quota || 0),
    model_limits: apiKey.model_limits
      ? apiKey.model_limits.split(',').filter(Boolean)
      : [],
    model_mapping: apiKey.model_mapping || '',
    allow_ips: apiKey.allow_ips || '',
    group: apiKey.group || DEFAULT_GROUP,
    cross_group_retry: !!apiKey.cross_group_retry,
    tokenCount: 1,
  }
}
