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
import { api } from '@/lib/api'
import type {
  CleanLogsParams,
  CleanLogsResponse,
  DatabaseMigrationInfoResponse,
  DatabaseMigrationJobResponse,
  DatabaseMigrationMode,
  DatabaseMigrationStartRequest,
  DatabaseMigrationStartResponse,
  DeleteOptionJsonMapEntryRequest,
  OptionJsonMapResponse,
  SystemOptionValueResponse,
  SystemOptionsResponse,
  UpdateOptionRequest,
  UpdateOptionResponse,
  UpsertOptionJsonMapEntryRequest,
} from './types'

export async function getSystemOptions(options?: {
  excludeLargeOptions?: boolean
}) {
  const res = await api.get<SystemOptionsResponse>('/api/option/', {
    params: options?.excludeLargeOptions
      ? { exclude_large_options: true }
      : undefined,
  })
  return res.data
}

export async function getSystemOptionValue(key: string) {
  const res = await api.get<SystemOptionValueResponse>('/api/option/value', {
    params: { key },
  })
  return res.data
}

export async function getOptionJsonMap(params: {
  key: string
  page: number
  pageSize: number
  keyword?: string
}) {
  const res = await api.get<OptionJsonMapResponse>('/api/option/json_map', {
    params: {
      key: params.key,
      page: params.page,
      page_size: params.pageSize,
      keyword: params.keyword || undefined,
    },
  })
  return res.data
}

export async function deleteOptionJsonMapEntry(
  request: DeleteOptionJsonMapEntryRequest
) {
  const res = await api.delete<UpdateOptionResponse>('/api/option/json_map', {
    data: request,
  })
  return res.data
}

export async function upsertOptionJsonMapEntry(
  request: UpsertOptionJsonMapEntryRequest
) {
  const res = await api.put<UpdateOptionResponse>(
    '/api/option/json_map',
    request
  )
  return res.data
}

export async function updateSystemOption(request: UpdateOptionRequest) {
  const res = await api.put<UpdateOptionResponse>('/api/option/', request)
  return res.data
}

export async function cleanLogs(params: CleanLogsParams) {
  const res = await api.delete<CleanLogsResponse>('/api/log/', { params })
  return res.data
}

export async function resetModelRatios() {
  const res = await api.post<UpdateOptionResponse>(
    '/api/option/rest_model_ratio'
  )
  return res.data
}

export async function resetToolBillingRules() {
  const res = await api.post<UpdateOptionResponse>(
    '/api/option/reset_tool_billing_rules'
  )
  return res.data
}

export async function getDatabaseMigrationInfo(mode: DatabaseMigrationMode) {
  const res = await api.get<DatabaseMigrationInfoResponse>(
    `/api/db/${mode}/info`
  )
  return res.data
}

export async function startDatabaseMigration(
  mode: DatabaseMigrationMode,
  request: DatabaseMigrationStartRequest
) {
  const res = await api.post<DatabaseMigrationStartResponse>(
    `/api/db/${mode}`,
    request
  )
  return res.data
}

export async function getDatabaseMigrationJob(
  mode: DatabaseMigrationMode,
  jobId: string
) {
  const res = await api.get<DatabaseMigrationJobResponse>(
    `/api/db/${mode}/${jobId}`
  )
  return res.data
}
