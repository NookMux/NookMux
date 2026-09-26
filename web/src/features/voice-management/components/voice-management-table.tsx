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
import { useMemo, type ReactNode } from 'react'
import { Edit, Trash2 } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { formatQuota, formatTimestampToDate } from '@/lib/format'
import {
  appTableFeatures,
  flexRender,
  useTable,
  type ColumnDef,
  type OnChangeFn,
  type PaginationState,
} from '@/lib/tanstack-table'
import { cn } from '@/lib/utils'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { TableCell, TableRow } from '@/components/ui/table'
import { DataTablePage } from '@/components/data-table'
import type { VoiceRecord } from '../api'

type VoiceManagementTableProps = {
  items: VoiceRecord[]
  total: number
  isLoading: boolean
  isFetching: boolean
  isRoot: boolean
  pagination: PaginationState
  onPaginationChange: OnChangeFn<PaginationState>
  onEdit: (voice: VoiceRecord) => void
  onRequestDelete: (voice: VoiceRecord) => void
  toolbar: ReactNode
}

function getVoiceTypeLabelKey(type: string): string {
  if (type === 'created') return 'voiceManagement.fields.voiceStatusPaid'
  if (type === 'preview') return 'voiceManagement.fields.voiceStatusPreview'
  return type || '-'
}

function getOperatorKindLabelKey(kind: string): string {
  if (kind === 'admin') return 'systemSettings.fields.admin'
  if (kind === 'user') return 'systemSettings.fields.user'
  return kind
}

function MonoValue({ value }: { value?: string }) {
  if (!value) return <span className='text-muted-foreground'>-</span>

  return (
    <span
      className='block max-w-[260px] truncate font-mono text-xs'
      title={value}
    >
      {value}
    </span>
  )
}

export function VoiceManagementTable(props: VoiceManagementTableProps) {
  const { t } = useTranslation()

  const columns = useMemo<ColumnDef<VoiceRecord>[]>(() => {
    const baseColumns: ColumnDef<VoiceRecord>[] = [
      {
        accessorKey: 'created_at',
        header: t('auditLogs.fields.time'),
        size: 170,
        cell: ({ row }) => (
          <span className='whitespace-nowrap'>
            {formatTimestampToDate(row.original.created_at)}
          </span>
        ),
      },
      {
        accessorKey: 'type',
        header: t('channels.fields.type'),
        size: 110,
        cell: ({ row }) => (
          <Badge variant='secondary'>
            {t(getVoiceTypeLabelKey(row.original.type))}
          </Badge>
        ),
      },
      {
        accessorKey: 'operator_id',
        header: t('voiceManagement.fields.operatorId'),
        size: 150,
        cell: ({ row }) => (
          <div className='space-y-0.5'>
            <div className='font-medium'>{row.original.operator_id}</div>
            {row.original.operator_kind && (
              <div className='text-muted-foreground text-xs'>
                {t(getOperatorKindLabelKey(row.original.operator_kind))}
              </div>
            )}
          </div>
        ),
      },
      {
        accessorKey: 'voice_id',
        header: t('voiceManagement.fields.voiceId'),
        size: 230,
        cell: ({ row }) => <MonoValue value={row.original.voice_id} />,
      },
      {
        accessorKey: 'quota_cost',
        header: t('keyQuery.fields.cost'),
        size: 130,
        cell: ({ row }) => (
          <span className='whitespace-nowrap'>
            {formatQuota(row.original.quota_cost)}
          </span>
        ),
      },
      {
        accessorKey: 'redirect_id',
        header: t('voiceManagement.fields.redirectId'),
        size: 230,
        cell: ({ row }) => <MonoValue value={row.original.redirect_id} />,
      },
      {
        accessorKey: 'allowed',
        header: t('voiceManagement.fields.whitelist'),
        size: 120,
        cell: ({ row }) => (
          <Badge variant={row.original.allowed ? 'default' : 'outline'}>
            {row.original.allowed
              ? t('voiceManagement.fields.allowed')
              : t('voiceManagement.fields.notAllowed')}
          </Badge>
        ),
      },
      {
        accessorKey: 'remark',
        header: t('channels.fields.remark'),
        size: 220,
        cell: ({ row }) => (
          <span
            className='block max-w-[260px] truncate text-sm'
            title={row.original.remark || undefined}
          >
            {row.original.remark || '-'}
          </span>
        ),
      },
    ]

    if (!props.isRoot) return baseColumns

    return [
      ...baseColumns,
      {
        id: 'actions',
        header: t('channels.fields.actions'),
        size: 150,
        cell: ({ row }) => (
          <div className='flex items-center justify-end gap-2'>
            <Button
              variant='outline'
              size='sm'
              onClick={() => props.onEdit(row.original)}
            >
              <Edit />
              {t('channels.actions.edit')}
            </Button>
            <Button
              variant='destructive'
              size='sm'
              onClick={() => props.onRequestDelete(row.original)}
            >
              <Trash2 />
              {t('common.actions.delete')}
            </Button>
          </div>
        ),
      },
    ]
  }, [props, t])

  const table = useTable({
    features: appTableFeatures,
    data: props.items,
    columns,
    state: {
      pagination: props.pagination,
    },
    manualPagination: true,
    pageCount: Math.max(1, Math.ceil(props.total / props.pagination.pageSize)),
    onPaginationChange: props.onPaginationChange,
  })

  return (
    <DataTablePage
      table={table}
      columns={columns}
      isLoading={props.isLoading}
      isFetching={props.isFetching}
      emptyTitle={t('voiceManagement.fields.noVoicesFound')}
      emptyDescription={t(
        'voiceManagement.tips.voiceRecordsWillAppearHereAfterUsersCreateVoices'
      )}
      skeletonKeyPrefix='voice-management-skeleton'
      tableClassName='overflow-x-auto'
      tableHeaderClassName='bg-muted/30 sticky top-0 z-10'
      toolbar={props.toolbar}
      applyHeaderSize
      renderRow={(row) => (
        <TableRow key={row.id} className='transition-colors'>
          {row.getVisibleCells().map((cell) => (
            <TableCell
              key={cell.id}
              className={cn(cell.column.id === 'actions' && 'text-right')}
            >
              {flexRender(cell.column.columnDef.cell, cell.getContext())}
            </TableCell>
          ))}
        </TableRow>
      )}
    />
  )
}
