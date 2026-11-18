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
import i18next from 'i18next'
import { api } from '@/lib/api'
import type {
  ApiResponse,
  DynamicRatioRule,
  DynamicRatioRulePayload,
  DynamicRatioStatus,
} from './types'

function unwrap<T>(res: ApiResponse<T>, fallbackKey: string): T {
  if (!res.success) {
    throw new Error(res.message || i18next.t(fallbackKey))
  }
  if (res.data === undefined) {
    throw new Error(i18next.t(fallbackKey))
  }
  return res.data
}

export async function getDynamicRatioRules(): Promise<DynamicRatioRule[]> {
  const res = await api.get('/api/dynamic_ratio/rules')
  return unwrap<DynamicRatioRule[]>(
    res.data,
    'dynamicRatio.errors.failedToLoadRules'
  )
}

export async function getDynamicRatioStatus(): Promise<DynamicRatioStatus> {
  const res = await api.get('/api/dynamic_ratio/status')
  return unwrap<DynamicRatioStatus>(
    res.data,
    'dynamicRatio.errors.failedToLoadStatus'
  )
}

export async function setDynamicRatioEnabled(enabled: boolean): Promise<void> {
  const res = await api.put('/api/dynamic_ratio/enabled', { enabled })
  unwrap(res.data, 'dynamicRatio.errors.failedToUpdateStatus')
}

export async function createDynamicRatioRule(
  payload: DynamicRatioRulePayload
): Promise<DynamicRatioRule> {
  const res = await api.post('/api/dynamic_ratio/rules', payload)
  return unwrap<DynamicRatioRule>(
    res.data,
    'dynamicRatio.errors.failedToCreateRule'
  )
}

export async function updateDynamicRatioRule(
  payload: DynamicRatioRulePayload & { id: number }
): Promise<DynamicRatioRule> {
  const res = await api.put('/api/dynamic_ratio/rules', payload)
  return unwrap<DynamicRatioRule>(
    res.data,
    'dynamicRatio.errors.failedToUpdateRule'
  )
}

export async function deleteDynamicRatioRule(id: number): Promise<void> {
  const res = await api.delete(`/api/dynamic_ratio/rules/${id}`)
  unwrap(res.data, 'dynamicRatio.errors.failedToDeleteRule')
}

export async function reorderDynamicRatioRules(ids: number[]): Promise<void> {
  const res = await api.put('/api/dynamic_ratio/rules/reorder', { ids })
  unwrap(res.data, 'dynamicRatio.errors.failedToReorderRules')
}
