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
  PerfMetricsResponse,
  PerfMetricsSummary,
  PerfModelMetrics,
} from './types'

// ----------------------------------------------------------------------------
// Perf metrics APIs
// ----------------------------------------------------------------------------

function unwrapPerfResponse<T>(payload: PerfMetricsResponse<T>): T {
  if (!payload.success) {
    throw new Error(payload.message || 'perf metrics request failed')
  }
  return payload.data
}

// Get per-model performance detail within the query window (hours 1-720, default 24)
export async function getModelPerfMetrics(
  model: string,
  hours = 24
): Promise<PerfModelMetrics> {
  const res = await api.get<PerfMetricsResponse<PerfModelMetrics>>(
    '/api/perf-metrics',
    { params: { model, hours } }
  )
  return unwrapPerfResponse(res.data)
}

// Get site-wide per-model performance summary within the query window
export async function getPerfMetricsSummary(
  hours = 24
): Promise<PerfMetricsSummary> {
  const res = await api.get<PerfMetricsResponse<PerfMetricsSummary>>(
    '/api/perf-metrics/summary',
    { params: { hours } }
  )
  return unwrapPerfResponse(res.data)
}
