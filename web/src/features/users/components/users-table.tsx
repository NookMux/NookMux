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
import { useMediaQuery } from '@/hooks'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import {
  appTableFeatures,
  type SortingState,
  type VisibilityState,
  useTable,
} from '@/lib/tanstack-table'
import { useTableUrlState } from '@/hooks/use-table-url-state'
import {
  DISABLED_ROW_DESKTOP,
  DISABLED_ROW_MOBILE,
  DataTablePage,
} from '@/components/data-table'
import { getUsers, searchUsers } from '../api'
import { USER_STATUS, isUserDeleted } from '../constants'
import type { User } from '../types'
import { DataTableBulkActions } from './data-table-bulk-actions'
import { useUsersColumns } from './users-columns'
import { UsersFilterBar } from './users-filter-bar'
import { useUsers } from './users-provider'

const route = getRouteApi('/_authenticated/users/')

function isDisabledUserRow(user: User) {
  return isUserDeleted(user) || user.status === USER_STATUS.DISABLED
}

export function UsersTable() {
  const { t } = useTranslation()
  const columns = useUsersColumns()
  const { refreshTrigger } = useUsers()
  const isMobile = useMediaQuery('(max-width: 640px)')
  const [rowSelection, setRowSelection] = useState({})
  const [sorting, setSorting] = useState<SortingState>([])
  const [columnVisibility, setColumnVisibility] = useState<VisibilityState>({})

  const searchParams = route.useSearch()
  const { pagination, onPaginationChange, ensurePageInRange } =
    useTableUrlState({
      search: route.useSearch(),
      navigate: route.useNavigate(),
      pagination: { defaultPage: 1, defaultPageSize: isMobile ? 10 : 20 },
    })

  const username = searchParams.username ?? ''
  const displayName = searchParams.display_name ?? ''
  const email = searchParams.email ?? ''
  const linuxDoId = searchParams.linux_do_id ?? ''
  const githubId = searchParams.github_id ?? ''
  const status = searchParams.status ?? ''
  const role = searchParams.role ?? ''

  // Fetch data with React Query
  const { data, isLoading, isFetching } = useQuery({
    queryKey: [
      'users',
      pagination.pageIndex + 1,
      pagination.pageSize,
      username,
      displayName,
      email,
      linuxDoId,
      githubId,
      status,
      role,
      refreshTrigger,
    ],
    queryFn: async () => {
      const hasFilter =
        username ||
        displayName ||
        email ||
        linuxDoId ||
        githubId ||
        status ||
        role
      const params = {
        p: pagination.pageIndex + 1,
        page_size: pagination.pageSize,
      }

      const result = hasFilter
        ? await searchUsers({
            ...params,
            username,
            display_name: displayName,
            email,
            linux_do_id: linuxDoId,
            github_id: githubId,
            status,
            role,
          })
        : await getUsers(params)

      if (!result.success) {
        toast.error(
          result.message || `Failed to ${hasFilter ? 'search' : 'load'} users`
        )
        return { items: [], total: 0 }
      }

      return {
        items: result.data?.items || [],
        total: result.data?.total || 0,
      }
    },
    placeholderData: (previousData) => previousData,
  })

  const users = data?.items || []

  const table = useTable({
    features: appTableFeatures,
    data: users,
    columns,
    state: {
      sorting,
      columnVisibility,
      rowSelection,
      pagination,
    },
    enableRowSelection: true,
    onRowSelectionChange: setRowSelection,
    getRowId: (row) => String(row.id),
    onSortingChange: setSorting,
    onColumnVisibilityChange: setColumnVisibility,
    onPaginationChange,
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
      emptyTitle={t('users.titles.noUsersFound')}
      emptyDescription={t(
        'users.tips.noUsersAvailableTryAdjustingYourSearchOrFilters'
      )}
      skeletonKeyPrefix='users-skeleton'
      tableClassName='overflow-x-auto'
      tableHeaderClassName='bg-muted/30 sticky top-0 z-10'
      toolbar={<UsersFilterBar table={table} />}
      getRowClassName={(row, { isMobile }) =>
        isDisabledUserRow(row.original)
          ? isMobile
            ? DISABLED_ROW_MOBILE
            : DISABLED_ROW_DESKTOP
          : undefined
      }
      bulkActions={<DataTableBulkActions table={table} />}
    />
  )
}
