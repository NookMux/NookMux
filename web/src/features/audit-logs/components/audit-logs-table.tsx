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
import { useCallback, useEffect, useMemo, useState } from 'react'
import { useQuery, useQueryClient } from '@tanstack/react-query'
import { useNavigate } from '@tanstack/react-router'
import { useMediaQuery } from '@/hooks'
import { Eye } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import dayjs from '@/lib/dayjs'
import {
  appTableFeatures,
  type ColumnDef,
  type PaginationState,
  flexRender,
  useTable,
} from '@/lib/tanstack-table'
import { cn } from '@/lib/utils'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { TableCell, TableRow } from '@/components/ui/table'
import { DataTablePage } from '@/components/data-table'
import { getAuditLogs, type AuditLog } from '../api'
import {
  ACTION_TYPE_BADGE_CLASS,
  AUDIT_ACTION_TYPES,
  AUDIT_MODULES,
  DEFAULT_AUDIT_LOGS_DATA,
} from '../constants'
import { AuditDiffDialog } from './audit-diff-dialog'
import {
  AuditLogsFilterBar,
  type AuditLogFilters,
} from './audit-logs-filter-bar'

export interface AuditLogsSearch {
  page?: number
  pageSize?: number
  username?: string
  module?: string
  action_type?: string
  start_timestamp?: number
  end_timestamp?: number
}

interface AuditLogsTableProps {
  search: AuditLogsSearch
}

function searchToFilters(search: AuditLogsSearch): AuditLogFilters {
  return {
    username: search.username || '',
    module: search.module || '',
    actionType: search.action_type || '',
    startTime: search.start_timestamp
      ? new Date(search.start_timestamp * 1000)
      : undefined,
    endTime: search.end_timestamp
      ? new Date(search.end_timestamp * 1000)
      : undefined,
  }
}

function filtersToSearch(
  filters: AuditLogFilters
): Omit<AuditLogsSearch, 'pageSize'> {
  return {
    page: undefined,
    username: filters.username || undefined,
    module: filters.module || undefined,
    action_type: filters.actionType || undefined,
    start_timestamp: filters.startTime
      ? Math.floor(filters.startTime.getTime() / 1000)
      : undefined,
    end_timestamp: filters.endTime
      ? Math.floor(filters.endTime.getTime() / 1000)
      : undefined,
  }
}

export function AuditLogsTable({ search }: AuditLogsTableProps) {
  const { t } = useTranslation()
  const isMobile = useMediaQuery('(max-width: 640px)')
  const navigate = useNavigate()
  const queryClient = useQueryClient()

  const [diffDialogOpen, setDiffDialogOpen] = useState(false)
  const [selectedLog, setSelectedLog] = useState<AuditLog | null>(null)

  const urlFilters = useMemo(() => searchToFilters(search), [search])

  const page = search.page ?? 1
  const pageSize = search.pageSize ?? (isMobile ? 20 : 20)

  const { data, isLoading, isFetching } = useQuery({
    queryKey: ['audit-logs', page, pageSize, urlFilters, t],
    queryFn: async () => {
      const result = await getAuditLogs({
        p: page,
        page_size: pageSize,
        username: urlFilters.username || undefined,
        module: urlFilters.module || undefined,
        action_type: urlFilters.actionType || undefined,
        start_timestamp: urlFilters.startTime
          ? Math.floor(urlFilters.startTime.getTime() / 1000)
          : undefined,
        end_timestamp: urlFilters.endTime
          ? Math.floor(urlFilters.endTime.getTime() / 1000)
          : undefined,
      })

      if (!result?.success) {
        toast.error(
          result?.message || t('auditLogs.errors.failedToLoadAuditLogs')
        )
        return DEFAULT_AUDIT_LOGS_DATA
      }

      return result.data || DEFAULT_AUDIT_LOGS_DATA
    },
    placeholderData: (previousData) => previousData,
  })

  const handleApplyFilters = (next: AuditLogFilters) => {
    navigate({
      to: '/audit-logs',
      search: (prev) => ({
        ...prev,
        ...filtersToSearch(next),
      }),
    })
    queryClient.invalidateQueries({ queryKey: ['audit-logs'] })
  }

  const handleResetFilters = () => {
    navigate({
      to: '/audit-logs',
      search: (prev) => ({
        ...prev,
        page: undefined,
        username: undefined,
        module: undefined,
        action_type: undefined,
        start_timestamp: undefined,
        end_timestamp: undefined,
      }),
    })
    queryClient.invalidateQueries({ queryKey: ['audit-logs'] })
  }

  const handlePageChange: (
    updaterOrValue:
      | PaginationState
      | ((old: PaginationState) => PaginationState)
  ) => void = useCallback(
    (updaterOrValue) => {
      const next =
        typeof updaterOrValue === 'function'
          ? updaterOrValue({ pageIndex: page - 1, pageSize })
          : updaterOrValue
      navigate({
        to: '/audit-logs',
        search: (prev) => {
          const prevPageSize =
            typeof prev.pageSize === 'number' ? prev.pageSize : pageSize
          return {
            ...prev,
            page: next.pageIndex === 0 ? undefined : next.pageIndex + 1,
            pageSize: next.pageSize === pageSize ? prevPageSize : next.pageSize,
          }
        },
      })
    },
    [navigate, page, pageSize]
  )

  const pagination = useMemo(
    () => ({ pageIndex: page - 1, pageSize }),
    [page, pageSize]
  )

  const columns = useMemo<ColumnDef<AuditLog>[]>(
    () => [
      {
        id: 'created_at',
        header: () => t('auditLogs.fields.time'),
        cell: ({ row }) => {
          const ts = row.original.created_at
          if (!ts) return '-'
          return (
            <span className='text-xs whitespace-nowrap tabular-nums'>
              {dayjs.unix(ts).format('YYYY-MM-DD HH:mm:ss')}
            </span>
          )
        },
        size: 160,
      },
      {
        id: 'username',
        header: () => t('auditLogs.fields.operator'),
        cell: ({ row }) => (
          <span className='text-sm font-medium'>
            {row.original.username || '-'}
          </span>
        ),
        size: 120,
      },
      {
        id: 'ip',
        header: () => t('auditLogs.fields.ip'),
        cell: ({ row }) => (
          <span className='text-muted-foreground text-xs tabular-nums'>
            {row.original.ip || '-'}
          </span>
        ),
        size: 120,
      },
      {
        id: 'module',
        header: () => t('auditLogs.fields.module'),
        cell: ({ row }) => {
          const value = row.original.module
          const labelKey =
            AUDIT_MODULES.find((m) => m.value === value)?.label || value
          return (
            <Badge variant='secondary' className='font-normal'>
              {t(labelKey)}
            </Badge>
          )
        },
        size: 120,
      },
      {
        id: 'action_type',
        header: () => t('auditLogs.fields.actionType'),
        cell: ({ row }) => {
          const value = row.original.action_type
          const labelKey =
            AUDIT_ACTION_TYPES.find((a) => a.value === value)?.label || value
          const badgeClass = ACTION_TYPE_BADGE_CLASS[value] || ''
          return (
            <Badge
              variant='secondary'
              className={cn('font-normal', badgeClass)}
            >
              {t(labelKey)}
            </Badge>
          )
        },
        size: 100,
      },
      {
        id: 'description',
        header: () => t('auditLogs.tips.description'),
        cell: ({ row }) => (
          <span className='text-foreground/90 text-sm'>
            {row.original.description || '-'}
          </span>
        ),
      },
      {
        id: 'actions',
        header: () => t('auditLogs.titles.details'),
        cell: ({ row }) => {
          const log = row.original
          const hasDiff =
            (log.before_data && log.before_data !== '') ||
            (log.after_data && log.after_data !== '')
          if (!hasDiff) {
            return <span className='text-muted-foreground text-xs'>-</span>
          }
          return (
            <Button
              variant='ghost'
              size='sm'
              className='h-7 gap-1 px-2 text-xs'
              onClick={() => {
                setSelectedLog(log)
                setDiffDialogOpen(true)
              }}
            >
              <Eye className='size-3.5' />
              {t('auditLogs.actions.viewDetails')}
            </Button>
          )
        },
        size: 110,
      },
    ],
    [t]
  )

  const logs = data?.items || []

  const table = useTable({
    features: appTableFeatures,
    data: logs,
    columns,
    state: {
      pagination,
    },
    enableRowSelection: false,
    onPaginationChange: handlePageChange,
    manualPagination: true,
    pageCount: Math.ceil((data?.total || 0) / pageSize),
  })

  // Keep URL page in range when the dataset shrinks.
  const pageCountComputed = table.getPageCount()
  useEffect(() => {
    if (pageCountComputed > 0 && page > pageCountComputed) {
      navigate({
        replace: true,
        to: '/audit-logs',
        search: (prev) => ({
          ...prev,
          page: undefined,
        }),
      })
    }
  }, [pageCountComputed, page, navigate])

  return (
    <>
      <DataTablePage
        table={table}
        columns={columns as ColumnDef<AuditLog>[]}
        isLoading={isLoading}
        isFetching={isFetching}
        emptyTitle={t('auditLogs.titles.noAuditLogsFound')}
        emptyDescription={t(
          'auditLogs.tips.noAuditLogsAvailableLogsWillAppearHereOnce'
        )}
        skeletonKeyPrefix='audit-log-skeleton'
        tableClassName='overflow-x-auto'
        tableHeaderClassName='bg-muted/30 sticky top-0 z-10'
        toolbar={
          <AuditLogsFilterBar
            filters={urlFilters}
            onApply={handleApplyFilters}
            onReset={handleResetFilters}
            loading={isFetching}
          />
        }
        renderRow={(row) => (
          <TableRow key={row.id}>
            {row.getVisibleCells().map((cell) => (
              <TableCell key={cell.id} className='py-2.5'>
                {flexRender(cell.column.columnDef.cell, cell.getContext())}
              </TableCell>
            ))}
          </TableRow>
        )}
      />

      <AuditDiffDialog
        open={diffDialogOpen}
        onOpenChange={setDiffDialogOpen}
        auditLog={selectedLog}
      />
    </>
  )
}
