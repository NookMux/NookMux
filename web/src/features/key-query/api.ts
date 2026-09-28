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
import {
  KeyQueryError,
  type KeyQueryLog,
  type KeyQueryLogsParams,
  type KeyQueryLogsResponse,
  type KeyQueryUsageLogFieldsResponse,
  type KeyUsageData,
  type KeyUsageResponse,
} from './types'

const KEY_PATTERN = /^sk-[a-zA-Z0-9]{48}$/

interface BackendErrorResponse {
  success?: boolean
  message?: string
  error?: { message?: string; type?: string }
}

function getTokenHeaders(key: string) {
  return {
    Authorization: `Bearer ${key}`,
  }
}

function validateKey(rawKey: string): string {
  const key = rawKey.trim()
  if (!KEY_PATTERN.test(key)) {
    throw new KeyQueryError('keyQuery.errors.invalidKeyFormat')
  }
  return key
}

async function fetchJson<T>(url: string, key: string): Promise<T> {
  const res = await fetch(url, {
    headers: getTokenHeaders(key),
  })
  let data: unknown
  try {
    data = await res.json()
  } catch {
    throw new KeyQueryError(
      'keyQuery.errors.networkError',
      'Failed to parse response'
    )
  }

  const errorBody = data as BackendErrorResponse
  if (!res.ok || errorBody?.error) {
    const fallback =
      errorBody?.error?.message || errorBody?.message || 'Key query failed'
    throw new KeyQueryError('keyQuery.errors.keyInvalidOrExpired', fallback)
  }
  return data as T
}

export async function fetchKeyUsage(rawKey: string): Promise<KeyUsageData> {
  const key = validateKey(rawKey)
  const data = await fetchJson<KeyUsageResponse>('/api/usage/token/', key)
  if (!data?.data) {
    throw new KeyQueryError(
      'keyQuery.errors.keyInvalidOrExpired',
      data?.message
    )
  }
  return data.data
}

export async function fetchKeyLogs(
  rawKey: string,
  params: KeyQueryLogsParams
): Promise<{ items: KeyQueryLog[]; total: number }> {
  const key = validateKey(rawKey)

  const queryParams = new URLSearchParams()
  queryParams.append('p', String(params.page))
  queryParams.append('page_size', String(params.pageSize))
  if (params.startTime) {
    queryParams.append(
      'start_timestamp',
      String(Math.floor(params.startTime.getTime() / 1000))
    )
  }
  if (params.endTime) {
    queryParams.append(
      'end_timestamp',
      String(Math.floor(params.endTime.getTime() / 1000))
    )
  }
  if (params.model) {
    queryParams.append('model_name', params.model)
  }

  const url = `/api/log/token?${queryParams.toString()}`
  const data = await fetchJson<KeyQueryLogsResponse>(url, key)

  if (!data?.success) {
    throw new KeyQueryError('keyQuery.errors.failedToLoadLogs', data?.message)
  }

  // 后端在传入分页参数时返回 PageInfo 结构 { page, page_size, total, items }。
  // 若旧后端未升级，可能回退为数组结构，这里做一次兼容适配。
  const rawData = data.data as unknown
  if (Array.isArray(rawData)) {
    return { items: rawData as KeyQueryLog[], total: rawData.length }
  }

  const paginated = data.data
  if (!paginated) {
    return { items: [], total: 0 }
  }
  return { items: paginated.items ?? [], total: paginated.total ?? 0 }
}

export async function fetchKeyUsageLogFields(
  rawKey: string
): Promise<{ enabled: boolean; fields: string[] }> {
  const key = validateKey(rawKey)
  const data = await fetchJson<KeyQueryUsageLogFieldsResponse>(
    '/api/log/token/usage_log_fields',
    key
  )
  if (!data?.success || !data.data) {
    throw new KeyQueryError(
      'keyQuery.errors.failedToLoadFieldVisibility',
      data?.message
    )
  }
  return data.data
}
