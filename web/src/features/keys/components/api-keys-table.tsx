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
import { useEffect, useState } from 'react'
import { useQuery } from '@tanstack/react-query'
import { getRouteApi } from '@tanstack/react-router'
import { Database } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import {
  appTableFeatures,
  type SortingState,
  type Table,
  useTable,
} from '@/lib/tanstack-table'
import { cn } from '@/lib/utils'
import { useTableUrlState } from '@/hooks/use-table-url-state'
import {
  Empty,
  EmptyDescription,
  EmptyHeader,
  EmptyMedia,
  EmptyTitle,
} from '@/components/ui/empty'
import { Skeleton } from '@/components/ui/skeleton'
import {
  DISABLED_ROW_DESKTOP,
  DISABLED_ROW_MOBILE,
  DataTablePage,
  usePersistentColumnVisibility,
} from '@/components/data-table'
import { StatusBadge } from '@/components/status-badge'
import { getApiKeys, searchApiKeys } from '../api'
import { API_KEY_STATUS, API_KEY_STATUSES, ERROR_MESSAGES } from '../constants'
import { type ApiKey } from '../types'
import { ApiKeyCell, ApiKeyQuotaCell } from './api-keys-cells'
import { useApiKeysColumns } from './api-keys-columns'
import { ApiKeysFilterBar } from './api-keys-filter-bar'
import { useApiKeys } from './api-keys-provider'
import { DataTableBulkActions } from './data-table-bulk-actions'
import { DataTableRowActions } from './data-table-row-actions'

const route = getRouteApi('/_authenticated/keys/')

function isDisabledApiKeyRow(apiKey: ApiKey) {
  return apiKey.status !== API_KEY_STATUS.ENABLED
}

function ApiKeysMobileSkeleton() {
  return (
    <div className='divide-border overflow-hidden rounded-lg border'>
      {Array.from({ length: 5 }).map((_, index) => (
        <div
          key={index}
          className='space-y-2 border-b px-3 py-2.5 last:border-b-0'
        >
          <div className='flex items-center justify-between'>
            <Skeleton className='h-4 w-32' />
            <Skeleton className='h-5 w-16 rounded-md' />
          </div>
          <div className='flex items-center justify-between gap-3'>
            <Skeleton className='h-7 w-44' />
            <Skeleton className='h-8 w-16' />
          </div>
          <Skeleton className='h-3 w-28' />
        </div>
      ))}
    </div>
  )
}

function ApiKeysMobileList({
  table,
  isLoading,
}: {
  table: Table<ApiKey>
  isLoading: boolean
}) {
  const { t } = useTranslation()
  const rows = table.getRowModel().rows

  if (isLoading) return <ApiKeysMobileSkeleton />

  if (!rows.length) {
    return (
      <div className='rounded-lg border p-8'>
        <Empty className='border-none p-0'>
          <EmptyHeader>
            <EmptyMedia variant='icon'>
              <Database className='size-6' />
            </EmptyMedia>
            <EmptyTitle>{t('keys.fields.noApiKeysFound')}</EmptyTitle>
            <EmptyDescription>
              {t('keys.tips.noApiKeysAvailableCreateYourFirstApiKey')}
            </EmptyDescription>
          </EmptyHeader>
        </Empty>
      </div>
    )
  }

  return (
    <div className='divide-border overflow-hidden rounded-lg border'>
      {rows.map((row) => {
        const apiKey = row.original
        const statusConfig = API_KEY_STATUSES[apiKey.status]

        return (
          <div
            key={row.id}
            className={cn(
              'bg-card space-y-2.5 border-b px-3 py-2.5 last:border-b-0',
              isDisabledApiKeyRow(apiKey) && DISABLED_ROW_MOBILE
            )}
          >
            <div className='flex items-start justify-between gap-3'>
              <div className='min-w-0'>
                <div className='truncate text-sm font-semibold'>
                  {apiKey.name}
                </div>
                <div className='text-muted-foreground text-[11px]'>
                  {t('channels.fields.apiKey')}
                </div>
              </div>
              {statusConfig && (
                <StatusBadge
                  label={t(statusConfig.label)}
                  variant={statusConfig.variant}
                  copyable={false}
                />
              )}
            </div>

            <div className='flex min-w-0 items-center justify-between gap-2'>
              <div className='min-w-0 flex-1 [&_button:first-child]:max-w-full [&_button:first-child]:truncate [&_button:first-child]:px-0'>
                <ApiKeyCell apiKey={apiKey} />
              </div>
              <DataTableRowActions row={row} />
            </div>

            <div className='space-y-1.5'>
              <span className='text-muted-foreground text-xs'>
                {t('keys.fields.quota')}
              </span>
              <ApiKeyQuotaCell apiKey={apiKey} className='w-full' />
            </div>
          </div>
        )
      })}
    </div>
  )
}

export function ApiKeysTable() {
  const { t } = useTranslation()
  const { refreshTrigger } = useApiKeys()
  const columns = useApiKeysColumns()
  const [rowSelection, setRowSelection] = useState({})
  const [sorting, setSorting] = useState<SortingState>([
    { id: 'name', desc: false },
  ])
  const [columnVisibility, setColumnVisibility] = usePersistentColumnVisibility(
    'api-keys-table',
    {}
  )

  const searchParams = route.useSearch()
  const nameFilter = searchParams.name ?? ''
  const keyFilter = searchParams.key ?? ''
  const groupFilter = searchParams.group ?? ''
  const statusFilter = (searchParams.status ?? [])[0] ?? ''
  const statusFilterNum = statusFilter ? Number(statusFilter) : 0

  const {
    columnFilters,
    onColumnFiltersChange,
    pagination,
    onPaginationChange,
    ensurePageInRange,
  } = useTableUrlState({
    search: searchParams,
    navigate: route.useNavigate(),
    pagination: { defaultPage: 1, defaultPageSize: 20 },
    // status 改为服务端过滤，不再作为行级 columnFilter；
    // 否则只会过滤当前页数据，跨页结果不完整。
    columnFilters: [],
  })

  // Fetch data with React Query
  // eslint-disable-next-line @tanstack/query/exhaustive-deps
  const { data, isLoading, isFetching } = useQuery({
    queryKey: [
      'keys',
      pagination.pageIndex + 1,
      pagination.pageSize,
      nameFilter,
      keyFilter,
      groupFilter,
      statusFilter,
      refreshTrigger,
    ],
    queryFn: async () => {
      // name/key/group/status 均为服务端过滤；任一非空都走 /api/token/search。
      const hasServerFilter =
        nameFilter.trim() ||
        keyFilter.trim() ||
        groupFilter ||
        statusFilterNum > 0

      if (hasServerFilter) {
        const wrapSearchTerm = (value: string) =>
          value.length >= 2 && !value.includes('%') ? `%${value}%` : value
        const normalizedTokenFilter = keyFilter.startsWith('sk-')
          ? keyFilter.slice(3)
          : keyFilter
        const result = await searchApiKeys({
          keyword: wrapSearchTerm(nameFilter.trim()),
          token: wrapSearchTerm(normalizedTokenFilter.trim()),
          group: groupFilter || undefined,
          status: statusFilterNum > 0 ? statusFilterNum : undefined,
          all: true,
          p: pagination.pageIndex + 1,
          size: pagination.pageSize,
        })
        if (!result.success) {
          toast.error(result.message || t(ERROR_MESSAGES.SEARCH_FAILED))
          return { items: [], total: 0 }
        }
        return {
          items: result.data?.items || [],
          total: result.data?.total || 0,
        }
      }

      // Otherwise use pagination
      const result = await getApiKeys({
        p: pagination.pageIndex + 1,
        size: pagination.pageSize,
      })

      if (!result.success) {
        toast.error(result.message || t(ERROR_MESSAGES.LOAD_FAILED))
        return { items: [], total: 0 }
      }

      return {
        items: result.data?.items || [],
        total: result.data?.total || 0,
      }
    },
    placeholderData: (previousData) => previousData,
  })

  const apiKeys = data?.items || []

  const table = useTable({
    features: appTableFeatures,
    data: apiKeys,
    columns,
    state: {
      sorting,
      columnVisibility,
      rowSelection,
      columnFilters,
      pagination,
    },
    enableRowSelection: true,
    onRowSelectionChange: setRowSelection,
    getRowId: (row) => String(row.id),
    onSortingChange: setSorting,
    onColumnVisibilityChange: setColumnVisibility,
    onPaginationChange,
    onColumnFiltersChange,
    manualPagination: true,
    pageCount: Math.ceil((data?.total || 0) / pagination.pageSize),
  })

  const pageCount = table.getPageCount()
  useEffect(() => {
    ensurePageInRange(pageCount)
  }, [pageCount, ensurePageInRange])

  return (
    <DataTablePage
      table={table}
      columns={columns}
      isLoading={isLoading}
      isFetching={isFetching}
      emptyTitle={t('keys.fields.noApiKeysFound')}
      emptyDescription={t('keys.tips.noApiKeysAvailableCreateYourFirstApiKey')}
      skeletonKeyPrefix='api-keys-skeleton'
      tableClassName='overflow-x-auto'
      tableHeaderClassName='bg-muted/30 sticky top-0 z-10'
      toolbar={<ApiKeysFilterBar table={table} />}
      mobile={<ApiKeysMobileList table={table} isLoading={isLoading} />}
      getRowClassName={(row) =>
        isDisabledApiKeyRow(row.original) ? DISABLED_ROW_DESKTOP : undefined
      }
      bulkActions={<DataTableBulkActions table={table} />}
    />
  )
}
