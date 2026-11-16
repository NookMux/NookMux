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
export type PerfMetricsResponse<T> = {
  success: boolean
  message?: string
  data: T
}

export type PerfSummaryStats = {
  avg_latency_ms: number
  success_rate: number
  avg_tps: number
}

export type PerfSeriesPoint = {
  ts: number
  avg_ttft_ms: number
  avg_latency_ms: number
  success_rate: number
  avg_tps: number
}

export type PerfGroupStats = {
  group: string
  avg_ttft_ms: number
  avg_latency_ms: number
  success_rate: number
  avg_tps: number
  series: PerfSeriesPoint[]
}

export type PerfModelMetrics = {
  model_name: string
  series_schema: 'v1'
  summary: PerfSummaryStats | null
  series: PerfSeriesPoint[]
  window_start: number
  window_end: number
  groups: PerfGroupStats[]
}

export type PerfModelSummary = PerfSummaryStats & {
  model_name: string
}

export type PerfMetricsSummary = {
  summary: PerfSummaryStats | null
  window_start: number
  window_end: number
  models: PerfModelSummary[]
}
