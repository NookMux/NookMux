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

export interface AuditLog {
  id: number
  created_at: number
  username: string
  ip: string
  module: string
  action_type: string
  description: string
  before_data?: string
  after_data?: string
}

export interface GetAuditLogsParams {
  p?: number
  page_size?: number
  username?: string
  module?: string
  action_type?: string
  start_timestamp?: number
  end_timestamp?: number
}

export interface AuditLogsResponse {
  success: boolean
  message?: string
  data?: {
    items: AuditLog[]
    total: number
    page: number
    page_size: number
  }
}

export async function getAuditLogs(
  params: GetAuditLogsParams
): Promise<AuditLogsResponse> {
  const searchParams = new URLSearchParams()
  if (params.p) searchParams.set('p', String(params.p))
  if (params.page_size) searchParams.set('page_size', String(params.page_size))
  if (params.username) searchParams.set('username', params.username)
  if (params.module) searchParams.set('module', params.module)
  if (params.action_type) searchParams.set('action_type', params.action_type)
  if (params.start_timestamp)
    searchParams.set('start_timestamp', String(params.start_timestamp))
  if (params.end_timestamp)
    searchParams.set('end_timestamp', String(params.end_timestamp))
  const res = await api.get<AuditLogsResponse>(
    `/api/audit/?${searchParams.toString()}`
  )
  return res.data
}
