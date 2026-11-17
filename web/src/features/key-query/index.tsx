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
import { useMutation } from '@tanstack/react-query'
import { Search } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { formatQuota, formatTimestampToDate } from '@/lib/format'
import { Button } from '@/components/ui/button'
import { Card, CardContent } from '@/components/ui/card'
import { Input } from '@/components/ui/input'
import { PublicLayout } from '@/components/layout'
import { fetchKeyUsage } from './api'
import { KeyInfoCard } from './key-info-card'
import { KeyQueryLogsTable } from './key-query-logs-table'
import { KeyQueryError, type KeyUsageData } from './types'

function resolveEffectiveQuotaType(usage: KeyUsageData): number {
  if (usage.quota_type === 0 && !usage.unlimited_quota) {
    return 1
  }
  return usage.quota_type
}

export function KeyQuery() {
  const { t } = useTranslation()
  const [key, setKey] = useState('')
  const [confirmedKey, setConfirmedKey] = useState<string | null>(null)
  const [usage, setUsage] = useState<KeyUsageData | null>(null)

  const queryMutation = useMutation({
    mutationFn: fetchKeyUsage,
    onSuccess: (data) => {
      setUsage(data)
      setConfirmedKey(key.trim())
    },
    onError: (error) => {
      setUsage(null)
      setConfirmedKey(null)
      if (error instanceof KeyQueryError) {
        toast.error(t(error.messageKey))
      } else {
        toast.error(error.message)
      }
    },
  })

  const runQuery = () => {
    queryMutation.mutate(key)
  }

  const copySummary = async () => {
    if (!usage) return
    const quotaType = resolveEffectiveQuotaType(usage)
    const isUnlimited = quotaType === 0
    const remaining = isUnlimited
      ? t('keyQuery.fields.unlimited')
      : formatQuota(usage.total_available)
    const used = isUnlimited
      ? t('keyQuery.fields.unlimited')
      : formatQuota(usage.total_used)

    const summary = [
      `${t('dashboard.fields.totalQuota')}: ${remaining}`,
      `${t('keyQuery.status.usedQuota')}: ${used}`,
      `${t('keyQuery.fields.expirationTime')}: ${
        usage.expires_at === 0
          ? t('keyQuery.fields.never')
          : formatTimestampToDate(usage.expires_at)
      }`,
    ].join('\n')

    try {
      await navigator.clipboard.writeText(summary)
      toast.success(t('common.status.copied'))
    } catch {
      toast.error(t('keyQuery.actions.copyFailed'))
    }
  }

  return (
    <PublicLayout showMainContainer={false}>
      <div className='relative'>
        <div
          aria-hidden
          className='pointer-events-none absolute inset-x-0 top-0 h-[600px] opacity-20 dark:opacity-[0.10]'
          style={{
            background: [
              'radial-gradient(ellipse 60% 50% at 20% 20%, oklch(0.72 0.18 250 / 80%) 0%, transparent 70%)',
              'radial-gradient(ellipse 50% 40% at 80% 15%, oklch(0.65 0.15 200 / 60%) 0%, transparent 70%)',
              'radial-gradient(ellipse 40% 35% at 50% 70%, oklch(0.70 0.12 280 / 40%) 0%, transparent 70%)',
            ].join(', '),
            maskImage:
              'linear-gradient(to bottom, black 40%, transparent 100%)',
            WebkitMaskImage:
              'linear-gradient(to bottom, black 40%, transparent 100%)',
          }}
        />
        <main className='relative mx-auto flex h-[calc(100vh-4rem)] w-full max-w-6xl flex-col gap-4 px-3 pt-20 pb-8 sm:px-6 sm:pt-24 lg:px-8'>
          <header className='space-y-2'>
            <h1 className='text-2xl font-bold tracking-tight'>
              {t('keyQuery.titles.usageQuery')}
            </h1>
            <p className='text-muted-foreground max-w-2xl text-sm'>
              {t('keyQuery.tips.queryBalanceRecentUsageAndTokenLogsByApi')}
            </p>
          </header>

          <Card size='sm'>
            <CardContent className='flex flex-col gap-2 sm:flex-row'>
              <div className='relative flex-1'>
                <Search className='text-muted-foreground pointer-events-none absolute top-1/2 left-2.5 size-4 -translate-y-1/2' />
                <Input
                  value={key}
                  className='pl-8'
                  placeholder='sk-xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx'
                  onChange={(event) => setKey(event.target.value)}
                  onKeyDown={(event) => {
                    if (event.key === 'Enter') runQuery()
                  }}
                />
              </div>
              <Button
                onClick={runQuery}
                disabled={queryMutation.isPending}
                className='sm:w-28'
              >
                {queryMutation.isPending
                  ? t('channels.tips.querying')
                  : t('keyQuery.titles.query')}
              </Button>
            </CardContent>
          </Card>

          {usage && confirmedKey && (
            <>
              <KeyInfoCard usage={usage} onCopySummary={copySummary} />
              <KeyQueryLogsTable rawKey={confirmedKey} />
            </>
          )}
        </main>
      </div>
    </PublicLayout>
  )
}
