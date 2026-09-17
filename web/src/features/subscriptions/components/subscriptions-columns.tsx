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
import { useMemo } from 'react'
import { useTranslation } from 'react-i18next'
import { formatQuota } from '@/lib/format'
import { type ColumnDef } from '@/lib/tanstack-table'
import { DataTableColumnHeader } from '@/components/data-table'
import { GroupBadge } from '@/components/group-badge'
import { StatusBadge } from '@/components/status-badge'
import { TableId } from '@/components/table-id'
import { formatDuration, formatResetPeriod } from '../lib'
import type { PlanRecord } from '../types'
import { DataTableRowActions } from './data-table-row-actions'

export function useSubscriptionsColumns(): ColumnDef<PlanRecord>[] {
  const { t } = useTranslation()

  return useMemo(
    (): ColumnDef<PlanRecord>[] => [
      {
        accessorFn: (row) => row.plan.id,
        id: 'id',
        meta: { label: t('channels.fields.id'), mobileHidden: true },
        header: ({ column }) => (
          <DataTableColumnHeader
            column={column}
            title={t('channels.fields.id')}
          />
        ),
        cell: ({ row }) => <TableId value={row.original.plan.id} />,
        size: 60,
      },
      {
        accessorFn: (row) => row.plan.title,
        id: 'title',
        meta: { label: t('subscriptions.fields.plan'), mobileTitle: true },
        header: ({ column }) => (
          <DataTableColumnHeader
            column={column}
            title={t('subscriptions.fields.plan')}
          />
        ),
        cell: ({ row }) => {
          const plan = row.original.plan
          return (
            <div className='max-w-[200px]'>
              <div className='truncate font-medium'>{plan.title}</div>
              {plan.subtitle && (
                <div className='text-muted-foreground truncate text-xs'>
                  {plan.subtitle}
                </div>
              )}
            </div>
          )
        },
        size: 200,
      },
      {
        accessorFn: (row) => row.plan.price_amount,
        id: 'price',
        meta: { label: t('pricing.fields.price') },
        header: ({ column }) => (
          <DataTableColumnHeader
            column={column}
            title={t('pricing.fields.price')}
          />
        ),
        cell: ({ row }) => (
          <span className='font-semibold text-emerald-600'>
            ${Number(row.original.plan.price_amount || 0).toFixed(2)}
          </span>
        ),
        size: 100,
      },
      {
        id: 'duration',
        meta: { label: t('subscriptions.fields.validityPeriod') },
        header: ({ column }) => (
          <DataTableColumnHeader
            column={column}
            title={t('subscriptions.fields.validityPeriod')}
          />
        ),
        cell: ({ row }) => (
          <span className='text-muted-foreground'>
            {formatDuration(row.original.plan, t)}
          </span>
        ),
        size: 100,
      },
      {
        id: 'reset',
        meta: {
          label: t('subscriptions.fields.quotaReset'),
          mobileHidden: true,
        },
        header: ({ column }) => (
          <DataTableColumnHeader
            column={column}
            title={t('subscriptions.fields.quotaReset')}
          />
        ),
        cell: ({ row }) => (
          <span className='text-muted-foreground'>
            {formatResetPeriod(row.original.plan, t)}
          </span>
        ),
        size: 80,
      },
      {
        accessorFn: (row) => row.plan.sort_order,
        id: 'sort_order',
        meta: { label: t('channels.fields.priority'), mobileHidden: true },
        header: ({ column }) => (
          <DataTableColumnHeader
            column={column}
            title={t('channels.fields.priority')}
          />
        ),
        cell: ({ row }) => (
          <span className='text-muted-foreground'>
            {row.original.plan.sort_order}
          </span>
        ),
        size: 80,
      },
      {
        accessorFn: (row) => row.plan.enabled,
        id: 'enabled',
        meta: { label: t('channels.fields.status'), mobileBadge: true },
        header: ({ column }) => (
          <DataTableColumnHeader
            column={column}
            title={t('channels.fields.status')}
          />
        ),
        cell: ({ row }) =>
          row.original.plan.enabled ? (
            <StatusBadge
              label={t('channels.actions.enable')}
              variant='success'
              copyable={false}
            />
          ) : (
            <StatusBadge
              label={t('channels.actions.disable')}
              variant='neutral'
              copyable={false}
            />
          ),
        size: 80,
      },
      {
        id: 'payment',
        meta: {
          label: t('subscriptions.fields.paymentChannel'),
          mobileHidden: true,
        },
        header: ({ column }) => (
          <DataTableColumnHeader
            column={column}
            title={t('subscriptions.fields.paymentChannel')}
          />
        ),
        cell: ({ row }) => {
          const plan = row.original.plan
          return (
            <div className='flex gap-1'>
              {plan.stripe_price_id && (
                <StatusBadge
                  label='Stripe'
                  variant='neutral'
                  copyable={false}
                />
              )}
              {plan.creem_product_id && (
                <StatusBadge label='Creem' variant='neutral' copyable={false} />
              )}
              {plan.waffo_pancake_product_id && (
                <StatusBadge
                  label='Waffo Pancake'
                  variant='neutral'
                  copyable={false}
                />
              )}
            </div>
          )
        },
        size: 140,
      },
      {
        id: 'total_amount',
        meta: {
          label: t('subscriptions.fields.receivedAmount'),
          mobileHidden: true,
        },
        header: ({ column }) => (
          <DataTableColumnHeader
            column={column}
            title={t('subscriptions.fields.receivedAmount')}
          />
        ),
        cell: ({ row }) => {
          const total = Number(row.original.plan.total_amount || 0)
          return (
            <span className='text-muted-foreground'>
              {total > 0 ? formatQuota(total) : t('keyQuery.fields.unlimited')}
            </span>
          )
        },
        size: 100,
      },
      {
        id: 'upgrade_group',
        meta: {
          label: t('subscriptions.fields.upgradeGroup'),
          mobileHidden: true,
        },
        header: ({ column }) => (
          <DataTableColumnHeader
            column={column}
            title={t('subscriptions.fields.upgradeGroup')}
          />
        ),
        cell: ({ row }) => {
          const group = row.original.plan.upgrade_group
          if (!group) {
            return (
              <span className='text-muted-foreground'>
                {t('subscriptions.fields.noUpgrade')}
              </span>
            )
          }
          return <GroupBadge group={group} />
        },
        size: 100,
      },
      {
        id: 'actions',
        cell: ({ row }) => <DataTableRowActions row={row} />,
        size: 80,
      },
    ],
    [t]
  )
}
