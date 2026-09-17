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
import { Check, Copy } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { formatCurrencyFromUSD } from '@/lib/currency'
import { formatNumber } from '@/lib/format'
import { type ColumnDef } from '@/lib/tanstack-table'
import { useCopyToClipboard } from '@/hooks/use-copy-to-clipboard'
import { Button } from '@/components/ui/button'
import { StatusBadge } from '@/components/status-badge'
import {
  formatTimestamp,
  getPaymentMethodName,
  getStatusConfig,
} from '@/features/wallet/lib/billing'
import type { TopupRecord } from '@/features/wallet/types'

export interface OrderQueryColumnActions {
  onComplete: (tradeNo: string) => void
  completing: boolean
}

export function useOrderQueryColumns(
  isAdmin: boolean,
  actions: OrderQueryColumnActions
): ColumnDef<TopupRecord>[] {
  const { t } = useTranslation()

  function OrderNumberCell({ record }: { record: TopupRecord }) {
    const { copyToClipboard, copiedText } = useCopyToClipboard({
      notify: false,
    })

    return (
      <div className='flex min-w-0 items-center gap-2'>
        <code className='truncate font-mono text-sm'>{record.trade_no}</code>
        <Button
          variant='ghost'
          size='icon-xs'
          aria-label={t('orderQuery.actions.copyOrderNumber')}
          onClick={() => copyToClipboard(record.trade_no)}
        >
          {copiedText === record.trade_no ? (
            <Check className='size-3' />
          ) : (
            <Copy className='size-3' />
          )}
        </Button>
      </div>
    )
  }

  const columns: ColumnDef<TopupRecord>[] = [
    {
      accessorKey: 'trade_no',
      header: t('orderQuery.fields.number'),
      cell: ({ row }) => <OrderNumberCell record={row.original} />,
      size: 256,
    },
  ]

  if (isAdmin) {
    columns.push({
      accessorKey: 'user_id',
      header: t('orderQuery.fields.userId'),
      size: 112,
    })
  }

  columns.push(
    {
      accessorKey: 'payment_method',
      header: t('orderQuery.fields.paymentMethod'),
      cell: ({ row }) => getPaymentMethodName(row.original.payment_method, t),
      size: 144,
    },
    {
      accessorKey: 'amount',
      header: t('orderQuery.fields.amount'),
      cell: ({ row }) =>
        formatCurrencyFromUSD(row.original.amount, {
          digitsLarge: 2,
          digitsSmall: 2,
          abbreviate: false,
        }),
      size: 128,
    },
    {
      accessorKey: 'money',
      header: t('orderQuery.fields.payment'),
      cell: ({ row }) => formatNumber(row.original.money),
      size: 128,
    },
    {
      accessorKey: 'status',
      header: t('channels.fields.status'),
      cell: ({ row }) => {
        const status = getStatusConfig(row.original.status)
        return (
          <StatusBadge
            label={t(status.label)}
            variant={status.variant}
            showDot
            copyable={false}
          />
        )
      },
      size: 128,
    },
    {
      accessorKey: 'create_time',
      header: t('multimodalFiles.status.createdAt'),
      cell: ({ row }) => formatTimestamp(row.original.create_time),
      size: 160,
    }
  )

  if (isAdmin) {
    columns.push({
      id: 'actions',
      header: () => (
        <span className='text-right'>{t('channels.fields.actions')}</span>
      ),
      cell: ({ row }) => (
        <div className='text-right'>
          {row.original.status === 'pending' ? (
            <Button
              variant='outline'
              size='sm'
              disabled={actions.completing}
              onClick={() => actions.onComplete(row.original.trade_no)}
            >
              {t('orderQuery.fields.completeOrder')}
            </Button>
          ) : null}
        </div>
      ),
      enableSorting: false,
      enableHiding: false,
      size: 128,
    })
  }

  return columns
}
