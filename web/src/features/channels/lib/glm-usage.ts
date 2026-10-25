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
import i18next from 'i18next'

export type UsagePoint = {
  time: string
  value: number
  type: string
}

export type UsageStat = {
  label: string
  display: string
  trend: string
}

export type ModelSummary = {
  modelName?: string
  totalTokens?: number
}

export type FlattenedUsageData = {
  values: UsagePoint[]
  total: number
  summary: ModelSummary[]
  stats: UsageStat[]
  fields: string[]
  times: string[]
}

export function isRecord(value: unknown): value is Record<string, unknown> {
  return !!value && typeof value === 'object' && !Array.isArray(value)
}

export function numberArray(value: unknown): number[] {
  return Array.isArray(value)
    ? value.map((item) => {
        const n = Number(item)
        return Number.isFinite(n) ? n : 0
      })
    : []
}

export function stringArray(value: unknown): string[] {
  return Array.isArray(value) ? value.map((item) => String(item)) : []
}

export function formatCompactNumber(value: unknown): string {
  const num = Number(value)
  if (!Number.isFinite(num)) return '0'
  const abs = Math.abs(num)
  const sign = num < 0 ? '-' : ''
  if (abs >= 1_000_000_000) {
    return sign + (abs / 1_000_000_000).toFixed(1).replace(/\.0$/, '') + 'B'
  }
  if (abs >= 1_000_000) {
    return sign + (abs / 1_000_000).toFixed(1).replace(/\.0$/, '') + 'M'
  }
  if (abs >= 1_000) {
    return sign + (abs / 1_000).toFixed(1).replace(/\.0$/, '') + 'K'
  }
  return sign + abs.toLocaleString()
}

// formatRatePercent 渲染上游返回的比率字段（cacheHitRate 等取值 0-1）。
function formatRatePercent(value: unknown): string {
  const num = Number(value)
  if (!Number.isFinite(num)) return '0.0%'
  return `${(num * 100).toFixed(1)}%`
}

// formatTrendPercent 渲染环比趋势（0.0823 → +8.2%），0 或非法值不展示。
function formatTrendPercent(trend: unknown): string {
  const num = Number(trend)
  if (!Number.isFinite(num) || num === 0) return ''
  const pct = num * 100
  const magnitude =
    Math.abs(pct) >= 100 ? String(Math.round(pct)) : Math.abs(pct).toFixed(1)
  return `${pct > 0 ? '+' : '-'}${magnitude}%`
}

// summaryStatValue 读取 summary 里 { value, trend } 形式的统计块。
function summaryStatValue(block: unknown): { value: string; trend: string } {
  if (!isRecord(block)) return { value: '', trend: '' }
  return {
    value: String(block.value ?? ''),
    trend: String(block.trend ?? ''),
  }
}

// flattenModelUsageData 解析智谱 token-usage/usage-detail 响应：summary 为比率/Token
// 汇总指标；时间轴（xTime）与模型明细（modelDataList 等）均嵌套在 modelUsage 内。
export function flattenModelUsageData(
  data: Record<string, unknown>
): FlattenedUsageData {
  const modelUsage = isRecord(data.modelUsage) ? data.modelUsage : {}
  const times = stringArray(modelUsage.xTime)
  if (times.length === 0) {
    return {
      values: [],
      total: 0,
      summary: [],
      stats: [],
      fields: [],
      times: [],
    }
  }

  const totalUsage = isRecord(modelUsage.totalUsage)
    ? modelUsage.totalUsage
    : {}
  const totalTokens = Number(totalUsage.totalTokens) || 0
  const summaryBlock = isRecord(data.summary) ? data.summary : {}
  const modelList = Array.isArray(modelUsage.modelDataList)
    ? modelUsage.modelDataList.filter(isRecord)
    : []
  const summary = (
    Array.isArray(modelUsage.modelSummaryList)
      ? modelUsage.modelSummaryList
      : []
  )
    .filter(isRecord)
    .map((item) => ({
      modelName: String(item.modelName ?? ''),
      totalTokens: Number(item.totalTokens) || 0,
    }))

  const statBlocks: [
    label: string,
    block: unknown,
    format: (value: unknown) => string,
  ][] = [
    [
      i18next.t('channels.fields.planCacheHitRate'),
      summaryBlock.cacheHitRate,
      formatRatePercent,
    ],
    [
      i18next.t('channels.fields.planOffPeakUsageRate'),
      summaryBlock.offPeakUsageRate,
      formatRatePercent,
    ],
    [
      i18next.t('channels.fields.planTokensTotal'),
      summaryBlock.totalTokens,
      formatCompactNumber,
    ],
    [
      i18next.t('channels.fields.planAvgDailyTokens'),
      summaryBlock.averageDailyTokens,
      formatCompactNumber,
    ],
  ]
  const stats = statBlocks.map(([label, block, format]) => {
    const { value, trend } = summaryStatValue(block)
    return { label, display: format(value), trend: formatTrendPercent(trend) }
  })

  const seriesDefs = [
    {
      key: 'cachedInputTokensUsage',
      labelKey: 'channels.fields.planSeriesCached',
    },
    {
      key: 'uncachedInputTokensUsage',
      labelKey: 'channels.fields.planSeriesUncached',
    },
    {
      key: 'outputTokensUsage',
      labelKey: 'channels.fields.planSeriesOutput',
    },
  ] as const

  const fields: string[] = []
  const values: UsagePoint[] = []
  modelList.forEach((model) => {
    const modelName = String(model.modelName ?? '')
    seriesDefs.forEach(({ key, labelKey }) => {
      const usage = numberArray(model[key])
      const type = i18next.t(labelKey, { model: modelName })
      fields.push(type)
      times.forEach((time, index) => {
        values.push({ time, value: usage[index] || 0, type })
      })
    })
  })

  return { values, total: totalTokens, summary, stats, fields, times }
}
