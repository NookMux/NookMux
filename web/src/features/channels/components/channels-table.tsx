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
import { useState, useMemo, useEffect } from 'react'
import { useQuery } from '@tanstack/react-query'
import { getRouteApi } from '@tanstack/react-router'
import { useMediaQuery } from '@/hooks'
import { useTranslation } from 'react-i18next'
import { getLobeIcon } from '@/lib/lobe-icon'
import {
  appTableFeatures,
  useTable,
  type OnChangeFn,
  type SortingState,
  type ExpandedState,
  type Row,
} from '@/lib/tanstack-table'
import { useTableUrlState } from '@/hooks/use-table-url-state'
import {
  DISABLED_ROW_DESKTOP,
  DISABLED_ROW_MOBILE,
  DataTablePage,
  usePersistentColumnVisibility,
} from '@/components/data-table'
import { getChannels, searchChannels, getGroups } from '../api'
import { DEFAULT_PAGE_SIZE, CHANNEL_STATUS } from '../constants'
import {
  channelsQueryKeys,
  aggregateChannelsByTag,
  isTagAggregateRow,
  getChannelTypeIcon,
  getChannelTypeLabel,
} from '../lib'
import type { Channel, ChannelSortBy } from '../types'
import { useChannelsColumns } from './channels-columns'
import { ChannelsFilterBar } from './channels-filter-bar'
import { useChannels } from './channels-provider'
import { DataTableBulkActions } from './data-table-bulk-actions'

const route = getRouteApi('/_authenticated/channels/')

const CHANNEL_SORTABLE_COLUMNS = new Set<ChannelSortBy>([
  'id',
  'name',
  'priority',
  'balance',
  'response_time',
  'test_time',
])

function isDisabledChannelRow(channel: Channel) {
  return (
    !isTagAggregateRow(channel) && channel.status !== CHANNEL_STATUS.ENABLED
  )
}

export function ChannelsTable() {
  const { t } = useTranslation()
  const { enableTagMode, idSort } = useChannels()
  const isMobile = useMediaQuery('(max-width: 640px)')

  // Table state
  const [sorting, setSorting] = useState<SortingState>([])
  const [columnVisibility, setColumnVisibility] = usePersistentColumnVisibility(
    'channels-table',
    { models: false, tag: false }
  )
  const [rowSelection, setRowSelection] = useState({})
  const [expanded, setExpanded] = useState<ExpandedState>({})

  // URL state management — 仅保留分页；过滤字段由 ChannelsFilterBar 接管
  const { pagination, onPaginationChange, ensurePageInRange } =
    useTableUrlState({
      search: route.useSearch(),
      navigate: route.useNavigate(),
      pagination: {
        defaultPage: 1,
        defaultPageSize: isMobile ? 10 : DEFAULT_PAGE_SIZE,
      },
    })

  // 过滤字段直接从 URL 读取（filter-bar 负责写入）
  const searchParams = route.useSearch()
  const idFilter = searchParams.id ?? ''
  const nameFilter = searchParams.name ?? ''
  const modelFilter = searchParams.model ?? ''
  const tagFilter = searchParams.tag ?? ''
  const typeFilterArr = searchParams.type ?? []
  const statusFilterArr = searchParams.status ?? []
  const groupFilterArr = searchParams.group ?? []

  const typeFilterValue =
    typeFilterArr.length > 0 && !typeFilterArr.includes('all')
      ? typeFilterArr[0]
      : ''
  const statusFilterValue =
    statusFilterArr.length > 0 && !statusFilterArr.includes('all')
      ? statusFilterArr[0]
      : ''
  const groupFilterValue =
    groupFilterArr.length > 0 && !groupFilterArr.includes('all')
      ? groupFilterArr[0]
      : ''

  // 任一过滤字段非空就走 search API
  const shouldSearch = Boolean(
    idFilter.trim() ||
    nameFilter.trim() ||
    modelFilter.trim() ||
    tagFilter.trim() ||
    typeFilterValue ||
    statusFilterValue ||
    groupFilterValue
  )

  const sortParams = useMemo(() => {
    const activeSort = sorting[0]
    if (
      !activeSort ||
      !CHANNEL_SORTABLE_COLUMNS.has(activeSort.id as ChannelSortBy)
    ) {
      return {}
    }

    return {
      sort_by: activeSort.id as ChannelSortBy,
      sort_order: activeSort.desc ? 'desc' : 'asc',
    } as const
  }, [sorting])

  const handleSortingChange: OnChangeFn<SortingState> = (updater) => {
    setSorting((previous) => {
      const next = typeof updater === 'function' ? updater(previous) : updater
      if (pagination.pageIndex > 0) {
        onPaginationChange({ ...pagination, pageIndex: 0 })
      }
      return next
    })
  }

  // Fetch groups for filter
  const { data: groupsData } = useQuery({
    queryKey: ['groups'],
    queryFn: getGroups,
  })

  const groupOptions = useMemo(
    () =>
      (groupsData?.data || []).map((g) => ({
        label: g,
        value: g,
      })),
    [groupsData]
  )

  // Fetch channels data
  // eslint-disable-next-line @tanstack/query/exhaustive-deps
  const { data, isLoading, isFetching } = useQuery({
    queryKey: channelsQueryKeys.list({
      id: idFilter,
      name: nameFilter,
      model: modelFilter,
      tag: tagFilter,
      type: typeFilterValue,
      status: statusFilterValue,
      group: groupFilterValue,
      tag_mode: enableTagMode,
      id_sort: idSort,
      ...sortParams,
      p: pagination.pageIndex + 1,
      page_size: pagination.pageSize,
    }),
    queryFn: async () => {
      if (shouldSearch) {
        return searchChannels({
          id: idFilter.trim() ? Number(idFilter) : undefined,
          name: nameFilter.trim() || undefined,
          model: modelFilter.trim() || undefined,
          tag: tagFilter.trim() || undefined,
          type: typeFilterValue ? Number(typeFilterValue) : undefined,
          status: statusFilterValue || undefined,
          group: groupFilterValue || undefined,
          tag_mode: enableTagMode,
          id_sort: idSort,
          ...sortParams,
          p: pagination.pageIndex + 1,
          page_size: pagination.pageSize,
        })
      } else {
        return getChannels({
          type: typeFilterValue ? Number(typeFilterValue) : undefined,
          status: statusFilterValue || undefined,
          group: groupFilterValue || undefined,
          tag_mode: enableTagMode,
          id_sort: idSort,
          ...sortParams,
          p: pagination.pageIndex + 1,
          page_size: pagination.pageSize,
        })
      }
    },
    placeholderData: (previousData) => previousData,
  })

  // Apply tag aggregation if tag mode is enabled
  const channels = useMemo(() => {
    const rawChannels = data?.data?.items || []

    if (enableTagMode && rawChannels.length > 0) {
      return aggregateChannelsByTag(rawChannels)
    }

    return rawChannels
  }, [data, enableTagMode])

  const totalCount = data?.data?.total || 0
  const typeCounts = data?.data?.type_counts

  // Columns configuration
  const columns = useChannelsColumns()

  // React Table instance
  const table = useTable({
    features: appTableFeatures,
    data: channels,
    columns,
    pageCount: Math.ceil(totalCount / pagination.pageSize),
    state: {
      sorting,
      columnVisibility,
      rowSelection,
      pagination,
      expanded,
    },
    enableRowSelection: (row: Row<Channel>) => !isTagAggregateRow(row.original),
    onRowSelectionChange: setRowSelection,
    getRowId: (row) =>
      isTagAggregateRow(row) ? `tag:${row.key}` : String(row.id),
    onSortingChange: handleSortingChange,
    onColumnVisibilityChange: setColumnVisibility,
    onPaginationChange,
    onExpandedChange: setExpanded,
    getSubRows: (row: Channel & { children?: Channel[] }) => row.children,
    manualPagination: true,
    manualSorting: true,
    manualFiltering: true,
    // 标签聚合模式下，编辑/添加/刷新等 refetch 会重建行结构；TanStack 默认
    // 在行结构变化时自动重置 expanded，导致已展开标签收起。行 ID（tag:xxx /
    // channel id）本身稳定，关闭自动重置让展开状态在数据重载后得以保留。
    autoResetExpanded: false,
  })

  // Ensure page is in range when total count changes
  const pageCount = table.getPageCount()
  useEffect(() => {
    ensurePageInRange(pageCount)
  }, [pageCount, ensurePageInRange])

  // 准备类型筛选选项（基于搜索结果的 type_counts），传给筛选区
  const typeFilterOptions = useMemo(() => {
    const counts = typeCounts || {}
    const typeIds = Object.entries(counts)
      .map(([type, count]) => ({
        type: Number(type),
        count: Number(count) || 0,
      }))
      .filter((item) => item.type > 0 && item.count > 0)
      .sort((a, b) => {
        const labelA = t(getChannelTypeLabel(a.type))
        const labelB = t(getChannelTypeLabel(b.type))
        return labelA.localeCompare(labelB)
      })

    if (typeFilterValue) {
      const selectedTypeId = Number(typeFilterValue)
      const alreadyIncluded = typeIds.some(
        (item) => item.type === selectedTypeId
      )
      if (selectedTypeId > 0 && !alreadyIncluded) {
        typeIds.push({
          type: selectedTypeId,
          count: Number(counts[typeFilterValue]) || 0,
        })
      }
    }

    return typeIds.map((item) => {
      const iconName = getChannelTypeIcon(item.type)
      return {
        label: getChannelTypeLabel(item.type),
        value: String(item.type),
        iconNode: getLobeIcon(`${iconName}.Color`, 16),
      }
    })
  }, [t, typeCounts, typeFilterValue])

  const groupFilterOptions = groupOptions

  return (
    <DataTablePage
      table={table}
      columns={columns}
      isLoading={isLoading}
      isFetching={isFetching}
      emptyTitle={t('channels.titles.noChannelsFound')}
      emptyDescription={t(
        'channels.tips.noChannelsAvailableCreateYourFirstChannelToGet'
      )}
      skeletonKeyPrefix='channel-skeleton'
      applyHeaderSize
      tableClassName='overflow-x-auto'
      tableHeaderClassName='bg-muted/30 sticky top-0 z-10'
      toolbar={
        <ChannelsFilterBar
          table={table}
          typeOptions={typeFilterOptions}
          groupOptions={groupFilterOptions}
        />
      }
      getRowClassName={(row, { isMobile }) =>
        isDisabledChannelRow(row.original)
          ? isMobile
            ? DISABLED_ROW_MOBILE
            : DISABLED_ROW_DESKTOP
          : undefined
      }
      bulkActions={<DataTableBulkActions table={table} />}
    />
  )
}
