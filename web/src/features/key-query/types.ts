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

/**
 * 与后端 controller.GetTokenUsage 返回结构对齐。
 *
 * `object`/`code`/`message` 字段沿用旧版 OpenAI 风格响应，
 * `data` 内字段为新增的额度类型与周期信息。
 */
export interface KeyUsageData {
  object: 'token_usage'
  name: string
  total_granted: number
  total_used: number
  total_available: number
  unlimited_quota: boolean
  model_limits: Record<string, boolean> | null
  model_limits_enabled: boolean
  expires_at: number
  quota_type: number
  created_time: number
  accessed_time: number
  window_hours: number
  window_quota: number
  window_used_quota: number
  window_start_hour: number
  window_start_time: number
  window_next_reset_time: number
  cycle_days: number
  cycle_quota: number
  cycle_used_quota: number
  cycle_start_time: number
  cycle_next_reset_time: number
}

export interface KeyUsageResponse {
  code: boolean
  message: string
  data: KeyUsageData
}

/**
 * 使用日志条目，与 usage-logs 的 UsageLog 结构保持兼容，
 * 方便复用 details-dialog 中的详情主体。
 */
export interface KeyQueryLog {
  id: number
  user_id: number
  created_at: number
  type: number
  content: string
  username: string
  token_name: string
  model_name: string
  quota: number
  prompt_tokens: number
  completion_tokens: number
  use_time: number
  is_stream: boolean
  channel: number
  channel_name: string | null
  token_id: number
  group: string
  ip: string
  ua: string
  x_title: string
  http_referer: string
  request_id?: string
  upstream_request_id?: string
  other: string
  model_icon: string
}

interface KeyQueryLogsPaginatedData {
  items: KeyQueryLog[]
  total: number
  page: number
  page_size: number
}

export interface KeyQueryLogsParams {
  page: number
  pageSize: number
  startTime?: Date
  endTime?: Date
  model?: string
}

export interface KeyQueryLogsResponse {
  success: boolean
  message?: string
  data?: KeyQueryLogsPaginatedData
}

/**
 * 字段可见性响应，与使用日志的 useUsageLogFieldVisibility 期望一致。
 */
export interface KeyQueryUsageLogFieldsResponse {
  success: boolean
  message?: string
  data?: {
    enabled: boolean
    fields: string[]
  }
}

export class KeyQueryError extends Error {
  readonly messageKey: string
  constructor(messageKey: string, fallbackMessage?: string) {
    super(fallbackMessage ?? messageKey)
    this.name = 'KeyQueryError'
    this.messageKey = messageKey
  }
}
