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
import {
  Hash,
  Coins,
  Layers,
  Gauge,
  Zap,
  Flame,
  TrendingUp,
  Activity,
  type LucideIcon,
} from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { safeDivide } from '@/features/dashboard/lib'

interface StatCardConfig {
  key: string
  title: string
  description: string
  icon: LucideIcon
  getValue: (stat: Record<string, number>, days?: number) => number
}

export function useModelStatCardsConfig(): StatCardConfig[] {
  const { t } = useTranslation()

  return [
    {
      key: 'count',
      title: t('dashboard.fields.totalCount'),
      description: t('dashboard.fields.statisticalCount'),
      icon: Hash,
      getValue: (stat) => stat?.rpm ?? 0,
    },
    {
      key: 'quota',
      title: t('dashboard.fields.totalQuota'),
      description: t('dashboard.fields.statisticalQuota'),
      icon: Coins,
      getValue: (stat) => stat?.quota ?? 0,
    },
    {
      key: 'tokens',
      title: t('dashboard.fields.totalTokens'),
      description: t('dashboard.fields.statisticalTokens'),
      icon: Layers,
      getValue: (stat) => stat?.tpm ?? 0,
    },
    {
      key: 'avgRpm',
      title: t('dashboard.fields.averageRpm'),
      description: t('dashboard.fields.requestsPerMinute'),
      icon: Gauge,
      getValue: (stat, timeRangeMinutes = 1) =>
        safeDivide(stat?.rpm ?? 0, timeRangeMinutes),
    },
    {
      key: 'avgTpm',
      title: t('dashboard.fields.averageTpm'),
      description: t('dashboard.fields.tokensPerMinute'),
      icon: Zap,
      getValue: (stat, timeRangeMinutes = 1) =>
        safeDivide(stat?.tpm ?? 0, timeRangeMinutes),
    },
  ]
}

export function useSummaryCardsConfig(totals: {
  todayUsageDisplay: string
  usedDisplay: string
  requestCountDisplay: string
  currencyLabel: string
  currencyEnabled: boolean
}) {
  const { t } = useTranslation()

  return [
    {
      key: 'todayUsage',
      title: t('dashboard.fields.last24hUsage'),
      value: totals.todayUsageDisplay,
      description: totals.currencyEnabled
        ? `${t('dashboard.fields.consumedInTheLast24Hours')} (${totals.currencyLabel})`
        : t('dashboard.fields.consumedInTheLast24Hours'),
      icon: Flame,
    },
    {
      key: 'usage',
      title: t('dashboard.fields.historicalUsage'),
      value: totals.usedDisplay,
      description: totals.currencyEnabled
        ? `${t('dashboard.fields.totalConsumed')} (${totals.currencyLabel})`
        : t('dashboard.fields.totalConsumedQuota'),
      icon: TrendingUp,
    },
    {
      key: 'requests',
      title: t('dashboard.fields.requestCount'),
      value: totals.requestCountDisplay,
      description: t('dashboard.fields.totalRequestsMade'),
      icon: Activity,
    },
  ]
}
