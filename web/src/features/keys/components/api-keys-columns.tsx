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
import { useQuery } from '@tanstack/react-query'
import { useTranslation } from 'react-i18next'
import { getUserGroups } from '@/lib/api'
import { formatTimestampToDate } from '@/lib/format'
import { type ColumnDef } from '@/lib/tanstack-table'
import { cn } from '@/lib/utils'
import { Checkbox } from '@/components/ui/checkbox'
import {
  Tooltip,
  TooltipContent,
  TooltipTrigger,
} from '@/components/ui/tooltip'
import { DataTableColumnHeader } from '@/components/data-table'
import { GroupBadge } from '@/components/group-badge'
import { StatusBadge } from '@/components/status-badge'
import { API_KEY_STATUSES } from '../constants'
import { type ApiKey } from '../types'
import {
  ApiKeyCell,
  ApiKeyQuotaCell,
  ModelLimitsCell,
  IpRestrictionsCell,
} from './api-keys-cells'
import { DataTableRowActions } from './data-table-row-actions'

function useGroupRatios(): Record<string, number> {
  const { data } = useQuery({
    queryKey: ['user-self-groups'],
    queryFn: getUserGroups,
    staleTime: 5 * 60 * 1000,
    select: (res) => {
      if (!res.success || !res.data) return {}
      const ratios: Record<string, number> = {}
      for (const [group, info] of Object.entries(res.data)) {
        if (typeof info.ratio === 'number') {
          ratios[group] = info.ratio
        }
      }
      return ratios
    },
  })

  return data ?? {}
}

function renderApiKeyCell({ row }: { row: { original: ApiKey } }) {
  return <ApiKeyCell apiKey={row.original} />
}

function renderApiKeyQuotaCell({ row }: { row: { original: ApiKey } }) {
  return <ApiKeyQuotaCell apiKey={row.original} />
}

function renderModelLimitsCell({ row }: { row: { original: ApiKey } }) {
  return <ModelLimitsCell apiKey={row.original} />
}

function renderIpRestrictionsCell({ row }: { row: { original: ApiKey } }) {
  return <IpRestrictionsCell apiKey={row.original} />
}

export function useApiKeysColumns(): ColumnDef<ApiKey>[] {
  const { t } = useTranslation()
  const groupRatios = useGroupRatios()
  return [
    {
      id: 'select',
      header: ({ table }) => (
        <Checkbox
          checked={table.getIsAllPageRowsSelected()}
          indeterminate={
            table.getIsSomePageRowsSelected() &&
            !table.getIsAllPageRowsSelected()
          }
          onCheckedChange={(value) => table.toggleAllPageRowsSelected(!!value)}
          aria-label={t('channels.placeholders.selectAll')}
          className='translate-y-[2px]'
        />
      ),
      cell: ({ row }) => (
        <Checkbox
          checked={row.getIsSelected()}
          onCheckedChange={(value) => row.toggleSelected(!!value)}
          aria-label={t('channels.placeholders.selectRow')}
          className='translate-y-[2px]'
        />
      ),
      enableSorting: false,
      enableHiding: false,
      meta: { label: t('keys.placeholders.select') },
    },
    {
      accessorKey: 'name',
      header: ({ column }) => (
        <DataTableColumnHeader
          column={column}
          title={t('channels.fields.name')}
        />
      ),
      cell: ({ row }) => (
        <div className='max-w-[200px] truncate font-medium'>
          {row.getValue('name')}
        </div>
      ),
      meta: { label: t('channels.fields.name'), mobileTitle: true },
    },
    {
      accessorKey: 'status',
      header: ({ column }) => (
        <DataTableColumnHeader
          column={column}
          title={t('channels.fields.status')}
        />
      ),
      cell: ({ row }) => {
        const statusConfig = API_KEY_STATUSES[row.getValue('status') as number]
        if (!statusConfig) return null
        return (
          <StatusBadge
            label={t(statusConfig.label)}
            variant={statusConfig.variant}
            copyable={false}
          />
        )
      },
      filterFn: (row, id, value) => value.includes(String(row.getValue(id))),
      meta: { label: t('channels.fields.status'), mobileBadge: true },
    },
    {
      id: 'key',
      accessorKey: 'key',
      header: t('channels.fields.apiKey'),
      cell: renderApiKeyCell,
      enableSorting: false,
      meta: { label: t('channels.fields.apiKey') },
    },
    {
      id: 'quota',
      accessorKey: 'remain_quota',
      header: ({ column }) => (
        <DataTableColumnHeader column={column} title={t('keys.fields.quota')} />
      ),
      cell: renderApiKeyQuotaCell,
      meta: { label: t('keys.fields.quota') },
    },
    {
      accessorKey: 'group',
      header: ({ column }) => (
        <DataTableColumnHeader
          column={column}
          title={t('common.fields.group')}
        />
      ),
      cell: ({ row }) => {
        const apiKey = row.original
        const group = row.getValue('group') as string
        const ratio = group && group !== 'auto' ? groupRatios[group] : undefined

        if (group === 'auto') {
          return (
            <Tooltip>
              <TooltipTrigger
                render={
                  <span className='inline-flex items-center gap-1.5 text-xs' />
                }
              >
                <GroupBadge group='auto' />
                {apiKey.cross_group_retry && (
                  <StatusBadge
                    label={t('keys.fields.crossGroup')}
                    variant='info'
                    copyable={false}
                  />
                )}
              </TooltipTrigger>
              <TooltipContent>
                <span className='text-xs'>
                  {t(
                    'keys.tips.automaticallySelectsTheBestAvailableGroupWithCircuitBreaker'
                  )}
                </span>
              </TooltipContent>
            </Tooltip>
          )
        }
        return <GroupBadge group={group} ratio={ratio} />
      },
      meta: { label: t('common.fields.group'), mobileHidden: true },
    },
    {
      id: 'model_limits',
      accessorKey: 'model_limits',
      header: ({ column }) => (
        <DataTableColumnHeader
          column={column}
          title={t('channels.titles.models')}
        />
      ),
      cell: renderModelLimitsCell,
      enableSorting: false,
      meta: { label: t('channels.titles.models'), mobileHidden: true },
    },
    {
      id: 'allow_ips',
      accessorKey: 'allow_ips',
      header: ({ column }) => (
        <DataTableColumnHeader
          column={column}
          title={t('keys.fields.ipRestriction')}
        />
      ),
      cell: renderIpRestrictionsCell,
      enableSorting: false,
      meta: { label: t('keys.fields.ipRestriction'), mobileHidden: true },
    },
    {
      accessorKey: 'created_time',
      header: ({ column }) => (
        <DataTableColumnHeader
          column={column}
          title={t('dynamicRatio.status.created')}
        />
      ),
      cell: ({ row }) => (
        <span className='text-muted-foreground font-mono text-xs tabular-nums'>
          {formatTimestampToDate(row.getValue('created_time'))}
        </span>
      ),
      meta: { label: t('dynamicRatio.status.created'), mobileHidden: true },
    },
    {
      accessorKey: 'accessed_time',
      header: ({ column }) => (
        <DataTableColumnHeader
          column={column}
          title={t('keys.status.lastUsed')}
        />
      ),
      cell: ({ row }) => {
        const accessedTime = row.getValue('accessed_time') as number
        if (!accessedTime) {
          return <span className='text-muted-foreground text-xs'>-</span>
        }
        return (
          <span className='text-muted-foreground font-mono text-xs tabular-nums'>
            {formatTimestampToDate(accessedTime)}
          </span>
        )
      },
      meta: { label: t('keys.status.lastUsed'), mobileHidden: true },
    },
    {
      accessorKey: 'expired_time',
      header: ({ column }) => (
        <DataTableColumnHeader
          column={column}
          title={t('keys.fields.expires')}
        />
      ),
      cell: ({ row }) => {
        const expiredTime = row.getValue('expired_time') as number
        if (expiredTime === -1) {
          return (
            <StatusBadge
              label={t('keyQuery.fields.never')}
              variant='neutral'
              copyable={false}
            />
          )
        }
        const isExpired = expiredTime * 1000 < Date.now()
        return (
          <span
            className={cn(
              'font-mono text-xs tabular-nums',
              isExpired ? 'text-destructive' : 'text-muted-foreground'
            )}
          >
            {formatTimestampToDate(expiredTime)}
          </span>
        )
      },
      meta: { label: t('keys.fields.expires'), mobileHidden: true },
    },
    {
      id: 'actions',
      cell: ({ row }) => <DataTableRowActions row={row} />,
      meta: { label: t('channels.fields.actions') },
      size: 88,
    },
  ]
}
