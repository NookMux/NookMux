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
import type { BuiltinModelPricing } from '../types'
import type { ModelRatioField } from './model-ratio-visual-editor'

/** 可视化定价编辑器维护的八张定价表，键为模型名。 */
export type ModelRatioMaps = Record<ModelRatioField, Record<string, unknown>>

export const modelRatioFields = [
  'ModelPrice',
  'ModelRatio',
  'CacheRatio',
  'CreateCacheRatio',
  'CompletionRatio',
  'AudioRatio',
  'AudioCompletionRatio',
  'ContextPricing',
] as const satisfies readonly ModelRatioField[]

const numericBuiltinFields = [
  ['model_price', 'ModelPrice'],
  ['model_ratio', 'ModelRatio'],
  ['completion_ratio', 'CompletionRatio'],
  ['cache_ratio', 'CacheRatio'],
  ['create_cache_ratio', 'CreateCacheRatio'],
  ['audio_ratio', 'AudioRatio'],
  ['audio_completion_ratio', 'AudioCompletionRatio'],
] as const

/** 内置定价表中有可计费条目的模型定价。 */
export type AvailableBuiltinModelPricing = BuiltinModelPricing & {
  available: true
}

export function hasBuiltinPricing(
  pricing: BuiltinModelPricing
): pricing is AvailableBuiltinModelPricing {
  return pricing.available
}

/**
 * 用模型的内置定价条目替换八张定价表中该模型的现有条目，其余模型保持不变。
 */
export function applyBuiltinPricing(
  maps: ModelRatioMaps,
  model: string,
  pricing: AvailableBuiltinModelPricing
): ModelRatioMaps {
  const next = {} as ModelRatioMaps
  for (const field of modelRatioFields) {
    next[field] = { ...maps[field] }
    delete next[field][model]
  }
  for (const [key, field] of numericBuiltinFields) {
    const value = pricing[key]
    if (value !== undefined) {
      next[field][model] = value
    }
  }
  if (pricing.context_pricing) {
    next.ContextPricing[model] = pricing.context_pricing
  }
  return next
}
