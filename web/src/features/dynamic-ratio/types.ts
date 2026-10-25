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

export type ApiResponse<T = unknown> = {
  success: boolean
  message?: string
  data?: T
}

export type DynamicRatioRule = {
  id: number
  enable: boolean
  group: string
  models: string
  concurrency: number | null
  weekdays: string
  start_time: string
  end_time: string
  ratio: number
  priority: number
  created_at?: number
  updated_at?: number
}

export type DynamicRatioStatus = {
  enabled: boolean
  active_ratio: number
  active_group?: string
  timezone: string
  rules_count: number
  rules: DynamicRatioSummary[]
}

type DynamicRatioSummary = {
  group: string
  concurrency: number | null
  weekdays: string
  start_time: string
  end_time: string
  ratio: number
  priority: number
}

export type DynamicRatioRulePayload = {
  id?: number
  enable: boolean
  group: string
  models: string
  concurrency: number | null
  weekdays: string
  start_time: string
  end_time: string
  ratio: number
  priority: number
}
