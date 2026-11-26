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
import { useState, useMemo } from 'react'
import { Pencil, Plus, Trash2 } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { Button } from '@/components/ui/button'
import { Empty, EmptyDescription, EmptyHeader } from '@/components/ui/empty'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'
import { StatusBadge } from '@/components/status-badge'
import { safeJsonParseWithValidation } from '../utils/json-parser'
import { isObjectRecord } from '../utils/json-validators'
import {
  AmountDiscountDialog,
  type AmountDiscountData,
} from './amount-discount-dialog'

type AmountDiscountVisualEditorProps = {
  value: string
  onChange: (value: string) => void
}

export function AmountDiscountVisualEditor({
  value,
  onChange,
}: AmountDiscountVisualEditorProps) {
  const { t } = useTranslation()
  const [dialogOpen, setDialogOpen] = useState(false)
  const [editData, setEditData] = useState<AmountDiscountData | null>(null)

  const discounts = useMemo(() => {
    const parsed = safeJsonParseWithValidation<Record<string, unknown>>(value, {
      fallback: {},
      validator: isObjectRecord,
      validatorMessage: 'Amount discount must be a JSON object',
      context: 'amount discounts',
    })

    return Object.entries(parsed)
      .map(([amount, rate]) => ({
        amount: parseInt(amount, 10),
        discountRate:
          typeof rate === 'number' ? rate : parseFloat(String(rate)),
      }))
      .filter((item) => !isNaN(item.amount) && !isNaN(item.discountRate))
      .sort((a, b) => a.amount - b.amount)
  }, [value])

  const handleSave = (data: AmountDiscountData) => {
    const discountObject = safeJsonParseWithValidation<Record<string, unknown>>(
      value,
      {
        fallback: {},
        validator: isObjectRecord,
        silent: true,
      }
    )

    if (editData && editData.amount !== data.amount) {
      delete discountObject[editData.amount.toString()]
    }

    discountObject[data.amount.toString()] = data.discountRate

    onChange(JSON.stringify(discountObject, null, 2))
  }

  const handleDelete = (amount: number) => {
    const discountObject = safeJsonParseWithValidation<Record<string, unknown>>(
      value,
      {
        fallback: {},
        validator: isObjectRecord,
        silent: true,
      }
    )

    delete discountObject[amount.toString()]

    onChange(JSON.stringify(discountObject, null, 2))
  }

  const handleEdit = (discount: AmountDiscountData) => {
    setEditData(discount)
    setDialogOpen(true)
  }

  const handleAdd = () => {
    setEditData(null)
    setDialogOpen(true)
  }

  const formatPercentage = (rate: number) => {
    if (rate >= 1) return '0%'
    const discount = Math.round((1 - rate) * 100)
    return `${discount}%`
  }

  return (
    <div className='space-y-4'>
      <div className='flex sm:justify-end'>
        <Button
          type='button'
          variant='outline'
          size='sm'
          onClick={(e) => {
            e.preventDefault()
            e.stopPropagation()
            handleAdd()
          }}
          className='w-full sm:w-auto'
        >
          <Plus data-icon='inline-start' />
          {t('systemSettings.actions.addDiscountTier')}
        </Button>
      </div>

      {discounts.length === 0 ? (
        <Empty>
          <EmptyHeader>
            <EmptyDescription>
              {t('common.tips.noDiscountTiersConfiguredClickAddDiscountTierTo')}
            </EmptyDescription>
          </EmptyHeader>
        </Empty>
      ) : (
        <div>
          {/* Desktop table view */}
          <div className='hidden sm:block'>
            <Table className='[&_td:first-child]:pl-0 [&_td:last-child]:pr-0 [&_th:first-child]:pl-0 [&_th:last-child]:pr-0'>
              <TableHeader>
                <TableRow>
                  <TableHead>
                    {t('systemSettings.fields.rechargeAmount')}
                  </TableHead>
                  <TableHead>
                    {t('systemSettings.fields.discountRate')}
                  </TableHead>
                  <TableHead>{t('systemSettings.fields.discount')}</TableHead>
                  <TableHead className='text-right'>
                    {t('channels.fields.actions')}
                  </TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {discounts.map((discount) => (
                  <TableRow key={discount.amount}>
                    <TableCell>
                      <span className='font-mono text-sm'>
                        ${discount.amount}
                      </span>
                    </TableCell>
                    <TableCell>
                      <code className='bg-muted rounded px-1.5 py-0.5 text-sm'>
                        {discount.discountRate.toFixed(2)}
                      </code>
                    </TableCell>
                    <TableCell>
                      <StatusBadge
                        variant={discount.discountRate < 1 ? 'info' : 'neutral'}
                        className='font-mono'
                        copyable={false}
                      >
                        {formatPercentage(discount.discountRate)}{' '}
                        {t('systemSettings.fields.offda7a68')}
                      </StatusBadge>
                    </TableCell>
                    <TableCell className='text-right'>
                      <div className='flex justify-end gap-2'>
                        <Button
                          type='button'
                          variant='ghost'
                          size='sm'
                          onClick={(e) => {
                            e.preventDefault()
                            e.stopPropagation()
                            handleEdit(discount)
                          }}
                          aria-label={t('channels.actions.edit')}
                        >
                          <Pencil className='h-4 w-4' />
                        </Button>
                        <Button
                          type='button'
                          variant='ghost'
                          size='sm'
                          onClick={(e) => {
                            e.preventDefault()
                            e.stopPropagation()
                            handleDelete(discount.amount)
                          }}
                          aria-label={t('common.actions.delete')}
                        >
                          <Trash2 className='h-4 w-4' />
                        </Button>
                      </div>
                    </TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          </div>

          {/* Mobile card view */}
          <div className='divide-y sm:hidden'>
            {discounts.map((discount) => (
              <div key={discount.amount} className='py-3'>
                <div className='mb-3 flex items-start justify-between'>
                  <div className='flex-1'>
                    <div className='mb-2 font-mono text-base font-medium'>
                      ${discount.amount}
                    </div>
                    <StatusBadge
                      variant={discount.discountRate < 1 ? 'info' : 'neutral'}
                      className='font-mono'
                      copyable={false}
                    >
                      {formatPercentage(discount.discountRate)}{' '}
                      {t('systemSettings.fields.offda7a68')}
                    </StatusBadge>
                  </div>
                  <div className='flex gap-1'>
                    <Button
                      type='button'
                      variant='ghost'
                      size='sm'
                      onClick={(e) => {
                        e.preventDefault()
                        e.stopPropagation()
                        handleEdit(discount)
                      }}
                      aria-label={t('channels.actions.edit')}
                    >
                      <Pencil className='h-4 w-4' />
                    </Button>
                    <Button
                      type='button'
                      variant='ghost'
                      size='sm'
                      onClick={(e) => {
                        e.preventDefault()
                        e.stopPropagation()
                        handleDelete(discount.amount)
                      }}
                      aria-label={t('common.actions.delete')}
                    >
                      <Trash2 className='h-4 w-4' />
                    </Button>
                  </div>
                </div>
                <div className='text-sm'>
                  <span className='text-muted-foreground'>
                    {t('channels.fields.labelWithColon', {
                      label: t('systemSettings.fields.discountRate'),
                    })}{' '}
                  </span>
                  <code className='bg-muted rounded px-1.5 py-0.5 text-xs'>
                    {discount.discountRate.toFixed(2)}
                  </code>
                </div>
              </div>
            ))}
          </div>
        </div>
      )}

      <AmountDiscountDialog
        open={dialogOpen}
        onOpenChange={setDialogOpen}
        onSave={handleSave}
        editData={editData}
      />
    </div>
  )
}
