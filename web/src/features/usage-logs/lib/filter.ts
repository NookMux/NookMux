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
 * Utility functions for usage logs filters
 */
import type {
  LogCategory,
  LogFilters,
  CommonLogFilters,
  DrawingLogFilters,
  TaskLogFilters,
} from '../types'

// ============================================================================
// Filter Building Functions
// ============================================================================

/**
 * Build search params from filters based on log category.
 *
 * Every filter key is always present in the result (empty filters map to
 * `undefined`) so callers can spread it over the previous search params to
 * update filters without dropping unrelated URL state such as `pageSize`.
 */
export function buildSearchParams(
  filters: LogFilters,
  logCategory: LogCategory
): Record<string, unknown> {
  const baseParams: Record<string, unknown> = {
    startTime: filters.startTime ? filters.startTime.getTime() : undefined,
    endTime: filters.endTime ? filters.endTime.getTime() : undefined,
    channel: filters.channel || undefined,
  }

  switch (logCategory) {
    case 'common': {
      const commonFilters = filters as CommonLogFilters
      return {
        ...baseParams,
        model: commonFilters.model || undefined,
        token: commonFilters.token || undefined,
        group: commonFilters.group || undefined,
        username: commonFilters.username || undefined,
        requestId: commonFilters.requestId || undefined,
        upstreamRequestId: commonFilters.upstreamRequestId || undefined,
        ip: commonFilters.ip || undefined,
        ua: commonFilters.ua || undefined,
        xTitle: commonFilters.xTitle || undefined,
        httpReferer: commonFilters.httpReferer || undefined,
      }
    }
    case 'drawing': {
      const drawingFilters = filters as DrawingLogFilters
      return {
        ...baseParams,
        filter: drawingFilters.mjId || undefined,
      }
    }
    case 'task': {
      const taskFilters = filters as TaskLogFilters
      return {
        ...baseParams,
        filter: taskFilters.taskId || undefined,
      }
    }
    default:
      return baseParams
  }
}
