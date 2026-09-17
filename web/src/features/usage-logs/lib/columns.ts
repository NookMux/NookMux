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
 * Column definitions factory
 */
import type { ColumnDef } from '@/lib/tanstack-table'
import { useCommonLogsColumns } from '../components/columns/common-logs-columns'
import { useDrawingLogsColumns } from '../components/columns/drawing-logs-columns'
import { useTaskLogsColumns } from '../components/columns/task-logs-columns'
import type { LogCategory } from '../types'

// eslint-disable-next-line @typescript-eslint/no-explicit-any
type MixedLogColumns = ColumnDef<any>[]

/**
 * Get column definitions based on log category
 * Returns any[] due to different log types (UsageLog, MidjourneyLog, TaskLog)
 */
export function useColumnsByCategory(
  logCategory: LogCategory,
  isAdmin: boolean
): MixedLogColumns {
  const commonColumns = useCommonLogsColumns(isAdmin) as MixedLogColumns
  const drawingColumns = useDrawingLogsColumns(isAdmin) as MixedLogColumns
  const taskColumns = useTaskLogsColumns(isAdmin) as MixedLogColumns

  switch (logCategory) {
    case 'common':
      return commonColumns
    case 'drawing':
      return drawingColumns
    case 'task':
      return taskColumns
    default:
      return commonColumns
  }
}
