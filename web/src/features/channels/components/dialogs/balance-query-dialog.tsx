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
import { useState } from 'react'
import { useQueryClient } from '@tanstack/react-query'
import { Loader2, RefreshCw, DollarSign } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { formatCurrencyFromUSD } from '@/lib/currency'
import { formatTimestampToDate } from '@/lib/format'
import { Button } from '@/components/ui/button'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import { getGlmAccountReport, updateChannelBalance } from '../../api'
import { channelsQueryKeys, isZhipuChannel } from '../../lib'
import type { GlmAccountReportData } from '../../types'
import { useChannels } from '../channels-provider'

type BalanceQueryDialogProps = {
  open: boolean
  onOpenChange: (open: boolean) => void
}

export function BalanceQueryDialog({
  open,
  onOpenChange,
}: BalanceQueryDialogProps) {
  const { t } = useTranslation()
  const { currentRow, setCurrentRow } = useChannels()
  const queryClient = useQueryClient()
  const [isQuerying, setIsQuerying] = useState(false)
  const [balance, setBalance] = useState<number | null>(null)
  const [balanceUpdatedTime, setBalanceUpdatedTime] = useState<number | null>(
    null
  )
  // 智谱 GLM-4V 渠道的账户资金报告（人民币原值），查询成功后填充
  const [glmReport, setGlmReport] = useState<GlmAccountReportData | null>(null)

  if (!currentRow) return null

  const isZhipu = isZhipuChannel(currentRow)

  const handleQueryBalance = async () => {
    setIsQuerying(true)
    try {
      const response = await updateChannelBalance(currentRow.id)
      if (response.success && response.balance !== undefined) {
        const newBalance = response.balance
        const now = Math.floor(Date.now() / 1000)

        setBalance(newBalance)
        setBalanceUpdatedTime(now)
        toast.success(t('channels.status.balanceUpdatedSuccessfully'))

        // Update currentRow immediately with new balance and timestamp
        setCurrentRow({
          ...currentRow,
          balance: newBalance,
          balance_updated_time: now,
        })

        // Invalidate queries to refresh the table
        await queryClient.invalidateQueries({
          queryKey: channelsQueryKeys.lists(),
        })
      } else {
        toast.error(
          response.message || t('channels.errors.failedToQueryBalance')
        )
      }
    } catch (error: unknown) {
      toast.error(
        error instanceof Error
          ? error.message
          : t('channels.errors.failedToQueryBalance')
      )
    } finally {
      setIsQuerying(false)
    }
  }

  // 智谱 GLM-4V 渠道走账户报告接口：服务端携带数据库保存的 Key 请求智谱后台，
  // 返回余额/充值/赠金/消耗等指标，同时落库折算后的 USD 余额供表格展示。
  const handleQueryGlmAccountReport = async () => {
    setIsQuerying(true)
    try {
      const response = await getGlmAccountReport(currentRow.id)
      if (!response.success || !response.data) {
        throw new Error(
          response.message || t('channels.errors.failedToQueryBalance')
        )
      }

      setGlmReport(response.data)
      toast.success(t('channels.status.balanceUpdatedSuccessfully'))

      if (response.balance !== undefined) {
        const now = Math.floor(Date.now() / 1000)
        setCurrentRow({
          ...currentRow,
          balance: response.balance,
          balance_updated_time: now,
        })
        await queryClient.invalidateQueries({
          queryKey: channelsQueryKeys.lists(),
        })
      }
    } catch (error: unknown) {
      toast.error(
        error instanceof Error
          ? error.message
          : t('channels.errors.failedToQueryBalance')
      )
    } finally {
      setIsQuerying(false)
    }
  }

  const handleClose = () => {
    setBalance(null)
    setBalanceUpdatedTime(null)
    setGlmReport(null)
    onOpenChange(false)
  }

  const formatBalance = (bal: number) =>
    formatCurrencyFromUSD(bal, {
      digitsLarge: 2,
      digitsSmall: 4,
      abbreviate: false,
    })

  // 智谱账户报告金额为人民币原值，直接以 ¥ 展示，保持与智谱后台数字一致
  const formatCNYAmount = (value: number | null | undefined) =>
    value == null || Number.isNaN(value)
      ? '—'
      : `¥${Intl.NumberFormat(undefined, { maximumFractionDigits: 6 }).format(value)}`

  const formatDate = (timestamp: number) => {
    if (!timestamp) return 'Never'
    return formatTimestampToDate(timestamp)
  }

  const glmReportItems = glmReport
    ? [
        {
          label: t('channels.fields.availableBalance'),
          value: glmReport.available_balance,
        },
        {
          label: t('channels.fields.rechargeAmount'),
          value: glmReport.recharge_amount,
        },
        {
          label: t('channels.fields.giveAmount'),
          value: glmReport.give_amount,
        },
        {
          label: t('channels.fields.totalSpendAmount'),
          value: glmReport.total_spend_amount,
        },
        {
          label: t('channels.fields.frozenBalance'),
          value: glmReport.frozen_balance,
        },
      ]
    : []

  return (
    <Dialog open={open} onOpenChange={handleClose}>
      <DialogContent className={isZhipu ? 'sm:max-w-lg' : undefined}>
        <DialogHeader>
          <DialogTitle>{t('channels.titles.queryBalance')}</DialogTitle>
          <DialogDescription>
            {t('channels.fields.updateBalanceFor')}{' '}
            <strong>{currentRow.name}</strong>
          </DialogDescription>
        </DialogHeader>

        <div className='space-y-4 py-4'>
          {/* Current Balance Display */}
          <div className='bg-muted/50 rounded-lg border p-4'>
            <div className='text-muted-foreground mb-2 flex items-center gap-2 text-sm'>
              <DollarSign className='h-4 w-4' />
              <span>{t('channels.fields.currentBalance')}</span>
            </div>
            <div className='text-2xl font-bold'>
              {isZhipu
                ? formatCNYAmount(glmReport?.balance)
                : balance !== null
                  ? formatBalance(balance)
                  : formatBalance(currentRow.balance)}
            </div>
            <div className='text-muted-foreground mt-2 text-xs'>
              {t('channels.status.lastUpdated')}{' '}
              {formatDate(
                balanceUpdatedTime ?? currentRow.balance_updated_time
              )}
            </div>
          </div>

          {/* Zhipu account report details (balance/recharge/gift/spend/frozen) */}
          {isZhipu && glmReportItems.length > 0 && (
            <div className='grid grid-cols-1 gap-2 sm:grid-cols-2'>
              {glmReportItems.map((item) => (
                <div key={item.label} className='rounded-lg border p-3'>
                  <div className='text-muted-foreground text-xs'>
                    {item.label}
                  </div>
                  <div className='mt-1 text-sm font-semibold'>
                    {formatCNYAmount(item.value)}
                  </div>
                </div>
              ))}
            </div>
          )}

          {/* Balance Update Button */}
          <Button
            className='w-full'
            onClick={isZhipu ? handleQueryGlmAccountReport : handleQueryBalance}
            disabled={isQuerying}
          >
            {isQuerying && <Loader2 className='mr-2 h-4 w-4 animate-spin' />}
            {!isQuerying && <RefreshCw className='mr-2 h-4 w-4' />}
            {isQuerying
              ? t('channels.tips.querying')
              : t('channels.fields.updateBalance')}
          </Button>
        </div>

        <DialogFooter>
          <Button variant='outline' onClick={handleClose} disabled={isQuerying}>
            {t('common.actions.close')}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}
