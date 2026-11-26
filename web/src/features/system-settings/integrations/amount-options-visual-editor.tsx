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
import { Plus, X } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { Button } from '@/components/ui/button'
import { Empty, EmptyDescription, EmptyHeader } from '@/components/ui/empty'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { StatusBadge } from '@/components/status-badge'
import { safeJsonParseWithValidation } from '../utils/json-parser'
import { isArray } from '../utils/json-validators'

type AmountOptionsVisualEditorProps = {
  value: string
  onChange: (value: string) => void
}

export function AmountOptionsVisualEditor({
  value,
  onChange,
}: AmountOptionsVisualEditorProps) {
  const { t } = useTranslation()
  const [newAmount, setNewAmount] = useState('')

  const amounts = useMemo(() => {
    const parsed = safeJsonParseWithValidation<unknown[]>(value, {
      fallback: [],
      validator: isArray,
      validatorMessage: t(
        'systemSettings.errors.amountOptionsMustBeAJsonArray'
      ),
      context: 'amount options',
    })

    return parsed
      .filter((item) => typeof item === 'number' || !isNaN(Number(item)))
      .map(Number)
      .sort((a, b) => a - b)
  }, [value, t])

  const handleAdd = () => {
    const amount = parseFloat(newAmount)
    if (isNaN(amount) || amount <= 0) {
      return
    }

    try {
      const updatedAmounts = [...amounts, amount]
        .filter((v, i, a) => a.indexOf(v) === i) // Remove duplicates
        .sort((a, b) => a - b)

      onChange(JSON.stringify(updatedAmounts, null, 2))
      setNewAmount('')
    } catch (_error) {
      // eslint-disable-next-line no-console
      console.error('Failed to add amount:', _error)
    }
  }

  const handleRemove = (amount: number) => {
    try {
      const updatedAmounts = amounts.filter((a) => a !== amount)
      onChange(JSON.stringify(updatedAmounts, null, 2))
    } catch (_error) {
      // eslint-disable-next-line no-console
      console.error('Failed to remove amount:', _error)
    }
  }

  const handleKeyDown = (e: React.KeyboardEvent<HTMLInputElement>) => {
    if (e.key === 'Enter') {
      e.preventDefault()
      handleAdd()
    }
  }

  return (
    <div className='space-y-4'>
      <div>
        {amounts.length === 0 ? (
          <Empty>
            <EmptyHeader>
              <EmptyDescription>
                {t(
                  'systemSettings.tips.noAmountOptionsConfiguredAddAmountsBelowToGet'
                )}
              </EmptyDescription>
            </EmptyHeader>
          </Empty>
        ) : (
          <div className='flex flex-wrap gap-2'>
            {amounts.map((amount) => (
              <StatusBadge
                key={amount}
                variant='neutral'
                className='text-base'
                copyable={false}
              >
                <span className='font-mono'>${amount}</span>
                <Button
                  type='button'
                  variant='ghost'
                  size='icon-sm'
                  onClick={(e) => {
                    e.preventDefault()
                    e.stopPropagation()
                    handleRemove(amount)
                  }}
                  className='hover:bg-muted-foreground/20 size-auto p-0.5'
                  aria-label={t('systemSettings.actions.removeAmount', {
                    amount,
                  })}
                >
                  <X className='h-3.5 w-3.5' />
                </Button>
              </StatusBadge>
            ))}
          </div>
        )}
      </div>

      <div className='flex flex-col gap-2 sm:flex-row sm:items-end'>
        <div className='w-full sm:max-w-56'>
          <Label htmlFor='new-amount' className='mb-2 block'>
            {t('systemSettings.actions.addNewAmount')}
          </Label>
          <Input
            id='new-amount'
            type='number'
            step='0.01'
            min='0'
            placeholder={t('systemSettings.placeholders.eG100')}
            value={newAmount}
            onChange={(e) => setNewAmount(e.target.value)}
            onKeyDown={handleKeyDown}
          />
        </div>
        <Button
          type='button'
          variant='outline'
          size='sm'
          onClick={(e) => {
            e.preventDefault()
            e.stopPropagation()
            handleAdd()
          }}
          disabled={!newAmount || parseFloat(newAmount) <= 0}
          className='w-full sm:w-auto'
        >
          <Plus data-icon='inline-start' />
          <span className='sm:inline'>{t('channels.actions.add')}</span>
        </Button>
      </div>
    </div>
  )
}
