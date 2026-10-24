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
import { useCallback, useEffect, useMemo, useState } from 'react'
import { VChart } from '@visactor/react-vchart'
import i18next from 'i18next'
import { AlertTriangle, CheckCircle2, Loader2, RefreshCw } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { useChartTheme } from '@/lib/use-chart-theme'
import { cn } from '@/lib/utils'
import { VCHART_OPTION } from '@/lib/vchart'
import { Button } from '@/components/ui/button'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import { Progress } from '@/components/ui/progress'
import { ScrollArea } from '@/components/ui/scroll-area'
import {
  Tooltip,
  TooltipContent,
  TooltipTrigger,
} from '@/components/ui/tooltip'
import { StatusBadge, type StatusVariant } from '@/components/status-badge'
import { getGlmPlanActivity, getGlmPlanUsage, getPlanQuota } from '../../api'
import type {
  Channel,
  GlmActivityDay,
  GlmPlanActivityData,
  GlmUsageType,
  PlanLimitInfo,
  PlanMcpLimitInfo,
  PlanQuotaData,
  PlanTierInfo,
} from '../../types'
import { useChannels } from '../channels-provider'

const PLAN_DISPLAY_NAME_KEYS: Record<string, string> = {
  'glm-coding-plan': 'channels.fields.planGlmCodingPlan',
  'glm-coding-plan-international':
    'channels.fields.planGlmCodingPlanInternational',
  'kimi-coding-plan': 'channels.fields.planKimiCodingPlan',
  'minimax-coding-plan': 'channels.fields.planMinimaxCodingPlan',
  'minimax-coding-plan-international':
    'channels.fields.planMinimaxCodingPlanInternational',
  'ollama-coding-plan': 'channels.fields.planOllamaCodingPlan',
}

const FALLBACK_MODEL_COLORS = [
  '#d97757',
  '#6a9bcc',
  '#788c5d',
  '#9b6db7',
  '#c4a44e',
  '#5bb8a9',
]

const CHART_COLOR_VARIABLES = [
  '--chart-1',
  '--chart-2',
  '--chart-3',
  '--chart-4',
  '--chart-5',
] as const

type PlanQuotaDialogProps = {
  open: boolean
  onOpenChange: (open: boolean) => void
}

type UsagePoint = {
  time: string
  value: number
  type: string
}

type UsageStat = {
  label: string
  display: string
  trend: string
}

type ModelSummary = {
  modelName?: string
  totalTokens?: number
}

type FlattenedUsageData = {
  values: UsagePoint[]
  total: number
  summary: ModelSummary[]
  stats: UsageStat[]
  fields: string[]
  times: string[]
}

type PerformanceData = {
  values: UsagePoint[]
  avgSpeed: string
  avgRate: string
  times: string[]
}

function isRecord(value: unknown): value is Record<string, unknown> {
  return !!value && typeof value === 'object' && !Array.isArray(value)
}

function getThemeChartColors(themeKey?: string): string[] {
  if (typeof document === 'undefined') return FALLBACK_MODEL_COLORS
  void themeKey

  const bodyStyle = window.getComputedStyle(document.body)
  const rootStyle = window.getComputedStyle(document.documentElement)
  const colors = CHART_COLOR_VARIABLES.map((name) => {
    return (
      bodyStyle.getPropertyValue(name) || rootStyle.getPropertyValue(name)
    ).trim()
  }).filter(Boolean)

  return colors.length > 0 ? colors : FALLBACK_MODEL_COLORS
}

function numberArray(value: unknown): number[] {
  return Array.isArray(value)
    ? value.map((item) => {
        const n = Number(item)
        return Number.isFinite(n) ? n : 0
      })
    : []
}

function stringArray(value: unknown): string[] {
  return Array.isArray(value) ? value.map((item) => String(item)) : []
}

function clampPercent(value: unknown): number {
  const n = Number(value)
  if (!Number.isFinite(n)) return 0
  return Math.max(0, Math.min(100, Math.round(n)))
}

function formatCompactNumber(value: unknown): string {
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
    Math.abs(pct) >= 100
      ? String(Math.round(pct))
      : String(Math.round(pct * 10) / 10)
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

function getStatusVariant(status?: string, percentage?: number): StatusVariant {
  if (status === '紧张' || (percentage ?? 0) >= 80) return 'danger'
  if (status === '适中' || (percentage ?? 0) >= 50) return 'warning'
  if (status === '充裕') return 'success'
  return 'neutral'
}

function getPlanDisplayName(planName?: string): string {
  if (!planName) return ''
  const key = PLAN_DISPLAY_NAME_KEYS[planName]
  return key ? i18next.t(key) : planName
}

function isGlmPlan(planName?: string) {
  return (
    planName === 'glm-coding-plan' ||
    planName === 'glm-coding-plan-international'
  )
}

function isTierBasedPlan(planName?: string) {
  return (
    planName === 'kimi-coding-plan' ||
    planName === 'minimax-coding-plan' ||
    planName === 'minimax-coding-plan-international'
  )
}

function formatTimeLabel(timeStr: string): string {
  if (!timeStr) return ''
  const match = timeStr.match(/^(\d{4})-(\d{2})-(\d{2})\s*(.*)?$/)
  if (!match) return timeStr
  const date = `${match[2]}-${match[3]}`
  const time = match[4] || ''
  const timeMatch = time.match(/^(\d{2}):(\d{2})/)
  return timeMatch ? `${date} ${timeMatch[1]}:${timeMatch[2]}` : date
}

function sampleTimeLabels(times: string[], maxLabels = 4): string[] {
  if (times.length <= maxLabels) return times
  const step = (times.length - 1) / (maxLabels - 1)
  const result: string[] = []
  for (let i = 0; i < maxLabels; i += 1) {
    result.push(times[Math.round(i * step)])
  }
  return result
}

function toBjDate(date: Date): Date {
  const utc = date.getTime() + date.getTimezoneOffset() * 60000
  return new Date(utc + 8 * 3600000)
}

function formatBjParamDate(date: Date): string {
  const bj = toBjDate(date)
  const pad = (n: number) => n.toString().padStart(2, '0')
  return `${bj.getFullYear()}-${pad(bj.getMonth() + 1)}-${pad(bj.getDate())} ${pad(bj.getHours())}:${pad(bj.getMinutes())}:${pad(bj.getSeconds())}`
}

function getUsageTimeParams(days: number) {
  const now = new Date()
  const end = new Date(now.getTime() - 600000)
  const start = new Date()
  if (days === 0) {
    start.setHours(0, 0, 0, 0)
  } else {
    start.setTime(now.getTime() - days * 86400000)
  }
  return {
    startTime: formatBjParamDate(start),
    endTime: formatBjParamDate(end),
  }
}

function getPerfTimeParams(days: number) {
  const now = new Date()
  const yesterday = new Date(now.getTime() - 86400000)
  const end = new Date(
    yesterday.getFullYear(),
    yesterday.getMonth(),
    yesterday.getDate(),
    23,
    59,
    59
  )
  const start = new Date(end.getTime() - (days - 1) * 86400000)
  return {
    startTime: formatBjParamDate(start),
    endTime: formatBjParamDate(end),
  }
}

function formatResetTime(timeStr?: string): string {
  if (!timeStr) return ''
  const date = new Date(timeStr)
  if (Number.isNaN(date.getTime())) return timeStr
  const month = String(date.getMonth() + 1)
  const day = String(date.getDate())
  const time =
    String(date.getHours()).padStart(2, '0') +
    ':' +
    String(date.getMinutes()).padStart(2, '0')
  return i18next.t('channels.placeholders.planMonthDayReset', {
    month,
    day,
    time,
  })
}

function formatHourReset(timeStr?: string): string {
  if (!timeStr) return ''
  const date = new Date(timeStr)
  if (Number.isNaN(date.getTime())) return timeStr
  const time =
    String(date.getHours()).padStart(2, '0') +
    ':' +
    String(date.getMinutes()).padStart(2, '0')
  return i18next.t('channels.placeholders.planHourReset', { time })
}

function flattenUsageData(
  rawData: Record<string, unknown> | null,
  usageType: GlmUsageType
): FlattenedUsageData {
  const data = isRecord(rawData?.data) ? rawData.data : {}
  // token-usage 明细接口时间轴为驼峰 xTime，tool 旧接口仍为 x_time
  const times = stringArray(usageType === 'model' ? data.xTime : data.x_time)
  if (times.length === 0) {
    return { values: [], total: 0, summary: [], stats: [], fields: [], times: [] }
  }

  if (usageType === 'model') {
    // token-usage/usage-detail 响应：summary 为比率/Token 汇总指标，
    // modelUsage.modelDataList 按模型给出缓存/未缓存/输出 Tokens 时序。
    const modelUsage = isRecord(data.modelUsage) ? data.modelUsage : {}
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

  const networkSearch = numberArray(data.networkSearchCount)
  const webRead = numberArray(data.webReadMcpCount)
  const zread = numberArray(data.zreadMcpCount)
  const total =
    networkSearch.reduce((sum, value) => sum + value, 0) +
    webRead.reduce((sum, value) => sum + value, 0) +
    zread.reduce((sum, value) => sum + value, 0)
  const values: UsagePoint[] = []
  times.forEach((time, index) => {
    values.push({
      time,
      value: networkSearch[index] || 0,
      type: i18next.t('channels.fields.planNetworkSearch'),
    })
    values.push({
      time,
      value: webRead[index] || 0,
      type: i18next.t('channels.fields.planWebRead'),
    })
    values.push({
      time,
      value: zread[index] || 0,
      type: i18next.t('channels.fields.planOpenSourceRepo'),
    })
  })
  return {
    values,
    total,
    summary: [],
    stats: [],
    fields: [
      i18next.t('channels.fields.planNetworkSearch'),
      i18next.t('channels.fields.planWebRead'),
      i18next.t('channels.fields.planOpenSourceRepo'),
    ],
    times,
  }
}

function flattenPerformanceData(
  rawData: Record<string, unknown> | null,
  productLevel?: string
): PerformanceData {
  const data = isRecord(rawData?.data) ? rawData.data : {}
  const times = stringArray(data.x_time)
  if (times.length === 0) {
    return { values: [], avgSpeed: '--', avgRate: '--', times: [] }
  }

  const isLite = productLevel === 'Lite'
  const speedLabel = isLite
    ? i18next.t('channels.fields.planLiteSpeed')
    : i18next.t('channels.fields.planProMaxSpeed')
  const rateLabel = isLite
    ? i18next.t('channels.fields.planLiteSuccessRate')
    : i18next.t('channels.fields.planProMaxSuccessRate')
  const liteSpeed = numberArray(data.liteDecodeSpeed).map((v) =>
    Number(v.toFixed(2))
  )
  const proMaxSpeed = numberArray(data.proMaxDecodeSpeed).map((v) =>
    Number(v.toFixed(2))
  )
  const liteRate = numberArray(data.liteSuccessRate).map((v) =>
    Number((v * 100).toFixed(2))
  )
  const proMaxRate = numberArray(data.proMaxSuccessRate).map((v) =>
    Number((v * 100).toFixed(2))
  )
  const speedArr = isLite ? liteSpeed : proMaxSpeed
  const rateArr = isLite ? liteRate : proMaxRate
  const values: UsagePoint[] = []
  times.forEach((time, index) => {
    values.push({ time, value: speedArr[index] || 0, type: speedLabel })
    values.push({ time, value: rateArr[index] || 0, type: rateLabel })
  })
  const avg = (arr: number[]) =>
    arr.length
      ? (arr.reduce((sum, value) => sum + value, 0) / arr.length).toFixed(1)
      : '0'

  return { values, avgSpeed: avg(speedArr), avgRate: avg(rateArr), times }
}

function LimitCard({
  title,
  data,
  resetLabel,
}: {
  title: string
  data?: PlanLimitInfo | null
  resetLabel?: string
}) {
  if (!data) return null
  const percent = clampPercent(data.percentage)
  const variant = getStatusVariant(data.status, percent)

  return (
    <div className='rounded-lg border p-4'>
      <div className='flex items-center justify-between gap-2'>
        <div className='text-sm font-medium'>{title}</div>
        <StatusBadge
          label={data.status || `${percent}%`}
          variant={variant}
          copyable={false}
        />
      </div>
      <div className='mt-3 flex items-baseline gap-2'>
        <span className='text-2xl font-semibold tabular-nums'>{percent}%</span>
      </div>
      <Progress value={percent} aria-label={`${title}: ${percent}%`} />
      {resetLabel && (
        <div className='text-muted-foreground mt-2 text-xs'>{resetLabel}</div>
      )}
    </div>
  )
}

function McpLimitCard({ data }: { data?: PlanMcpLimitInfo | null }) {
  const { t } = useTranslation()
  if (!data) return null
  const percent = clampPercent(data.percentage)
  const variant = getStatusVariant(data.status, percent)

  return (
    <div className='rounded-lg border p-4'>
      <div className='flex items-center justify-between gap-2'>
        <div className='text-sm font-medium'>
          {t('channels.titles.planMcpToolLimit')}
        </div>
        <StatusBadge
          label={data.status || `${percent}%`}
          variant={variant}
          copyable={false}
        />
      </div>
      <div className='mt-3 flex items-baseline gap-2'>
        <span className='text-2xl font-semibold tabular-nums'>{percent}%</span>
        {data.current_usage && (
          <span className='text-muted-foreground text-xs'>
            {data.current_usage}
          </span>
        )}
      </div>
      <Progress value={percent} aria-label={`MCP: ${percent}%`} />
      <div className='text-muted-foreground mt-2 text-xs'>
        {t('channels.tips.planMonthlyReset')}
      </div>
      {data.tools && data.tools.length > 0 && (
        <div className='mt-3 border-t pt-3'>
          {data.tools.map((tool, index) => (
            <div
              key={`${tool.name ?? 'tool'}-${index}`}
              className='flex items-center justify-between gap-3 py-1 text-xs'
            >
              <span className='text-muted-foreground truncate'>
                {tool.name || '-'}
              </span>
              <span className='font-medium tabular-nums'>
                {formatCompactNumber(tool.usage)}
              </span>
            </div>
          ))}
        </div>
      )}
    </div>
  )
}

function TierLimitCard({ tier }: { tier: PlanTierInfo }) {
  const { t } = useTranslation()
  const title =
    tier.name === 'five_hour'
      ? t('channels.titles.planFiveHourLimit')
      : t('channels.titles.planWeeklyLimit')
  const percent = clampPercent(tier.percentage)
  const reset = tier.resets_at
    ? tier.name === 'five_hour'
      ? formatHourReset(tier.resets_at)
      : formatResetTime(tier.resets_at)
    : ''

  return (
    <div className='space-y-2'>
      <LimitCard
        title={title}
        data={{ percentage: percent, status: tier.status }}
        resetLabel={
          reset
            ? tier.name === 'five_hour'
              ? reset
              : `${t('channels.fields.planNextReset')}: ${reset}`
            : ''
        }
      />
      <div className='text-muted-foreground flex justify-between gap-3 px-1 text-xs'>
        <span>
          {t('channels.fields.planUsed')} {formatCompactNumber(tier.used)}
        </span>
        <span>
          {t('channels.fields.planRemaining')}{' '}
          {formatCompactNumber(tier.remaining)} /{' '}
          {formatCompactNumber(tier.limit)}
        </span>
      </div>
    </div>
  )
}

function UsageChart({ channelId }: { channelId: number }) {
  const { t } = useTranslation()
  const { resolvedTheme, themeReady } = useChartTheme()
  const [usageType, setUsageType] = useState<GlmUsageType>('model')
  const [range, setRange] = useState(7)
  const [loading, setLoading] = useState(false)
  const [rawData, setRawData] = useState<Record<string, unknown> | null>(null)

  const fetchUsage = useCallback(async () => {
    setLoading(true)
    try {
      const response = await getGlmPlanUsage(channelId, {
        type: usageType,
        ...getUsageTimeParams(range),
      })
      setRawData(response)
    } catch {
      setRawData(null)
    } finally {
      setLoading(false)
    }
  }, [channelId, range, usageType])

  useEffect(() => {
    fetchUsage()
  }, [fetchUsage])

  const { values, total, summary, stats, fields, times } = useMemo(
    () => flattenUsageData(rawData, usageType),
    [rawData, usageType]
  )
  const chartColors = useMemo(
    () => getThemeChartColors(resolvedTheme),
    [resolvedTheme]
  )

  const spec = useMemo(() => {
    if (values.length === 0) return null
    const sampledLabels = sampleTimeLabels(times, 4)
    const isModel = usageType === 'model'
    const colorRange = fields.map(
      (_, index) => chartColors[index % chartColors.length]
    )

    return {
      // 模型消耗为按模型堆叠的柱形图；工具调用保持折线/面积趋势图
      type: isModel ? 'bar' : 'area',
      data: [{ id: 'usage', values }],
      xField: 'time',
      yField: 'value',
      seriesField: 'type',
      stack: isModel,
      legends: {
        visible: true,
        position: 'top',
        item: { label: { style: { fontSize: 11 } } },
        autoPage: true,
        maxRow: 1,
      },
      color: { type: 'ordinal', range: colorRange, domain: fields },
      ...(isModel
        ? {
            bar: {
              state: {
                hover: { stroke: '#000', lineWidth: 1 },
              },
            },
          }
        : {
            area: {
              style: {
                fillOpacity: 0.08,
                curveType: 'monotone',
              },
            },
            line: {
              style: {
                lineWidth: 2,
                curveType: 'monotone',
              },
            },
          }),
      point: { visible: false },
      axes: [
        {
          orient: 'bottom',
          type: 'band',
          bandField: 'time',
          label: {
            style: { fontSize: 11 },
            autoRotate: false,
            formatMethod: (value: number | string) => {
              const label = String(value)
              return sampledLabels.includes(label) ? formatTimeLabel(label) : ''
            },
          },
          tick: { visible: false },
        },
        {
          orient: 'left',
          type: 'linear',
          field: 'value',
          label: {
            style: { fontSize: 10 },
            formatMethod: (value: number | string) =>
              formatCompactNumber(value),
          },
          grid: {
            visible: true,
            style: { lineDash: [3, 3], stroke: 'rgba(148, 163, 184, 0.35)' },
          },
        },
      ],
      tooltip: {
        visible: true,
        mark: {
          content: [
            {
              key: (datum: UsagePoint) => datum.type,
              value: (datum: UsagePoint) =>
                formatCompactNumber(datum.value ?? 0),
            },
          ],
        },
      },
      height: 240,
      padding: { top: 10, bottom: 5, left: 10, right: 10 },
      background: 'transparent',
      animation: true,
    }
  }, [chartColors, fields, times, usageType, values])

  const ranges = [
    { key: 0, label: t('channels.placeholders.planToday') },
    { key: 7, label: t('channels.placeholders.plan7Days') },
    { key: 15, label: t('channels.placeholders.plan15Days') },
    { key: 30, label: t('channels.placeholders.plan30Days') },
  ]

  return (
    <section className='space-y-3 rounded-lg border p-4'>
      <div className='flex flex-wrap items-center justify-between gap-2'>
        <div className='bg-muted inline-flex rounded-lg p-0.5'>
          {[
            {
              key: 'model' as const,
              label: t('channels.actions.planModelView'),
            },
            { key: 'tool' as const, label: t('channels.actions.planToolView') },
          ].map((item) => (
            <Button
              key={item.key}
              type='button'
              variant={usageType === item.key ? 'default' : 'ghost'}
              size='sm'
              onClick={() => setUsageType(item.key)}
            >
              {item.label}
            </Button>
          ))}
        </div>
        <div className='flex flex-wrap gap-1'>
          {ranges.map((item) => (
            <Button
              key={item.key}
              type='button'
              variant={range === item.key ? 'default' : 'outline'}
              size='sm'
              onClick={() => setRange(item.key)}
            >
              {item.label}
            </Button>
          ))}
        </div>
      </div>

      <div className='flex flex-wrap items-baseline gap-x-4 gap-y-2'>
        <div>
          <span className='text-muted-foreground text-xs'>
            {usageType === 'model'
              ? t('channels.fields.planTokensTotal')
              : t('channels.fields.planToolCallCount')}
          </span>
          <span className='ml-2 text-xl font-semibold'>
            {formatCompactNumber(total)}
          </span>
        </div>
        {usageType === 'model' && summary.length > 0 && (
          <div className='flex flex-wrap gap-2'>
            {summary.map((item, index) => (
              <span
                key={`${item.modelName ?? 'model'}-${index}`}
                className='inline-flex items-center gap-1 text-xs'
              >
                <span
                  className='h-2 w-2 rounded-full'
                  style={{
                    backgroundColor:
                      chartColors[(index + 1) % chartColors.length],
                  }}
                />
                <span>{item.modelName || '-'}</span>
                <span className='text-muted-foreground'>
                  {formatCompactNumber(item.totalTokens)}
                </span>
              </span>
            ))}
          </div>
        )}
      </div>

      {usageType === 'model' && stats.length > 0 && (
        <div className='grid grid-cols-2 gap-3 lg:grid-cols-4'>
          {stats.map((stat) => (
            <div key={stat.label} className='rounded-lg border p-3'>
              <div className='text-muted-foreground text-xs'>{stat.label}</div>
              <div className='mt-1 flex flex-wrap items-baseline gap-1.5'>
                <span className='text-xl font-semibold tabular-nums'>
                  {stat.display}
                </span>
                {stat.trend && (
                  <span
                    className={cn(
                      'text-xs font-medium tabular-nums',
                      stat.trend.startsWith('-')
                        ? 'text-success'
                        : 'text-destructive'
                    )}
                  >
                    {stat.trend}
                  </span>
                )}
              </div>
            </div>
          ))}
        </div>
      )}

      <div className='min-h-[240px]'>
        {loading ? (
          <div className='flex h-[240px] items-center justify-center'>
            <Loader2 className='text-muted-foreground h-5 w-5 animate-spin' />
          </div>
        ) : themeReady && spec ? (
          <VChart
            key={`plan-usage-${channelId}-${usageType}-${range}-${resolvedTheme}`}
            spec={{
              ...spec,
              theme: resolvedTheme === 'dark' ? 'dark' : 'light',
              background: 'transparent',
            }}
            option={VCHART_OPTION}
          />
        ) : (
          <div className='text-muted-foreground flex h-[120px] items-center justify-center text-sm'>
            {t('channels.fields.noData')}
          </div>
        )}
      </div>
      <div className='text-muted-foreground text-right text-xs'>
        {t('channels.tips.planDataDelay')}
      </div>
    </section>
  )
}

function PerformanceChart({
  channelId,
  productLevel,
}: {
  channelId: number
  productLevel?: string
}) {
  const { t } = useTranslation()
  const { resolvedTheme, themeReady } = useChartTheme()
  const [range, setRange] = useState(7)
  const [loading, setLoading] = useState(false)
  const [rawData, setRawData] = useState<Record<string, unknown> | null>(null)

  const fetchPerformance = useCallback(async () => {
    setLoading(true)
    try {
      const response = await getGlmPlanUsage(channelId, {
        type: 'performance',
        ...getPerfTimeParams(range),
      })
      setRawData(response)
    } catch {
      setRawData(null)
    } finally {
      setLoading(false)
    }
  }, [channelId, range])

  useEffect(() => {
    fetchPerformance()
  }, [fetchPerformance])

  const { values, avgSpeed, avgRate, times } = useMemo(
    () => flattenPerformanceData(rawData, productLevel),
    [productLevel, rawData]
  )
  const chartColors = useMemo(
    () => getThemeChartColors(resolvedTheme),
    [resolvedTheme]
  )

  const spec = useMemo(() => {
    if (values.length === 0) return null
    const sampledLabels = sampleTimeLabels(times, 4)
    const fields = Array.from(new Set(values.map((item) => item.type)))

    return {
      type: 'common',
      data: [{ id: 'performance', values }],
      series: [
        {
          type: 'line',
          xField: 'time',
          yField: 'value',
          seriesField: 'type',
          smooth: true,
          line: {
            style: {
              lineWidth: 2,
              lineDash: (datum: UsagePoint) =>
                String(datum?.type ?? '')
                  .toLowerCase()
                  .includes('success') ||
                String(datum?.type ?? '').includes('成功率')
                  ? [4, 4]
                  : [0],
            },
          },
          point: { visible: false },
        },
      ],
      axes: [
        {
          orient: 'bottom',
          type: 'band',
          bandField: 'time',
          label: {
            style: { fontSize: 11 },
            autoRotate: false,
            formatMethod: (value: number | string) => {
              const label = String(value)
              return sampledLabels.includes(label) ? formatTimeLabel(label) : ''
            },
          },
          tick: { visible: false },
        },
        {
          orient: 'left',
          type: 'linear',
          field: 'value',
          label: {
            style: { fontSize: 10 },
            formatMethod: (value: number | string) =>
              formatCompactNumber(value),
          },
          grid: {
            visible: true,
            style: { lineDash: [3, 3], stroke: 'rgba(148, 163, 184, 0.35)' },
          },
        },
      ],
      color: {
        type: 'ordinal',
        range: [chartColors[0], chartColors[2] ?? chartColors[1]],
        domain: fields,
      },
      legends: {
        visible: true,
        position: 'top',
        item: { label: { style: { fontSize: 11 } } },
      },
      tooltip: {
        visible: true,
        mark: {
          content: [
            {
              key: (datum: UsagePoint) => datum.type,
              value: (datum: UsagePoint) =>
                datum.type.toLowerCase().includes('success') ||
                datum.type.includes('成功率')
                  ? `${Number(datum.value || 0).toFixed(1)}%`
                  : `${Number(datum.value || 0).toFixed(1)} tokens/s`,
            },
          ],
        },
      },
      height: 240,
      padding: { top: 10, bottom: 5, left: 10, right: 10 },
      background: 'transparent',
    }
  }, [chartColors, times, values])

  const ranges = [
    { key: 7, label: t('channels.placeholders.plan7Days') },
    { key: 15, label: t('channels.placeholders.plan15Days') },
    { key: 30, label: t('channels.placeholders.plan30Days') },
  ]

  return (
    <section className='space-y-3 rounded-lg border p-4'>
      <div className='flex flex-wrap items-center justify-between gap-2'>
        <div className='text-sm font-semibold'>
          {t('channels.titles.planSystemHealth')}
        </div>
        <div className='flex flex-wrap gap-1'>
          {ranges.map((item) => (
            <Button
              key={item.key}
              type='button'
              variant={range === item.key ? 'default' : 'outline'}
              size='sm'
              onClick={() => setRange(item.key)}
            >
              {item.label}
            </Button>
          ))}
        </div>
      </div>
      <div className='flex flex-wrap gap-6'>
        <div>
          <span className='text-muted-foreground text-xs'>
            {t('channels.fields.planAvgSpeed')}
          </span>
          <span className='ml-2 text-xl font-semibold'>{avgSpeed}</span>
          <span className='text-muted-foreground ml-1 text-xs'>tokens/s</span>
        </div>
        <div>
          <span className='text-muted-foreground text-xs'>
            {t('channels.fields.planSuccessRate')}
          </span>
          <span className='ml-2 text-xl font-semibold'>{avgRate}%</span>
        </div>
      </div>
      <div className='min-h-[240px]'>
        {loading ? (
          <div className='flex h-[240px] items-center justify-center'>
            <Loader2 className='text-muted-foreground h-5 w-5 animate-spin' />
          </div>
        ) : themeReady && spec ? (
          <VChart
            key={`plan-performance-${channelId}-${range}-${resolvedTheme}`}
            spec={{
              ...spec,
              theme: resolvedTheme === 'dark' ? 'dark' : 'light',
              background: 'transparent',
            }}
            option={VCHART_OPTION}
          />
        ) : (
          <div className='text-muted-foreground flex h-[120px] items-center justify-center text-sm'>
            {t('channels.fields.noData')}
          </div>
        )}
      </div>
    </section>
  )
}

// ============================================================================
// GLM Plan Activity (summary + 365-day heatmap)
// ============================================================================

type ActivityHeatmapCell = {
  date: string
  tokens: number
  mcpCalls: number
}

// 热力图强度色阶：L0 无活动用 muted，L1-L4 按 --chart-1 混合透明度递增，
// 与主题 preset 自动联动（浅色/深色/自定义主题均生效）。
const ACTIVITY_HEAT_COLORS = [
  'color-mix(in oklab, var(--muted) 60%, transparent)',
  'color-mix(in oklab, var(--chart-1) 30%, transparent)',
  'color-mix(in oklab, var(--chart-1) 55%, transparent)',
  'color-mix(in oklab, var(--chart-1) 78%, transparent)',
  'var(--chart-1)',
] as const

function activityHeatLevel(tokens: number, maxTokens: number): number {
  if (tokens <= 0 || maxTokens <= 0) return 0
  const ratio = tokens / maxTokens
  if (ratio <= 0.25) return 1
  if (ratio <= 0.5) return 2
  if (ratio <= 0.75) return 3
  return 4
}

function formatActivityDateKey(date: Date): string {
  const pad = (n: number) => n.toString().padStart(2, '0')
  return `${date.getFullYear()}-${pad(date.getMonth() + 1)}-${pad(date.getDate())}`
}

function formatActivityDuration(ms?: number): string {
  const totalMs = Number(ms) || 0
  if (totalMs <= 0) return '0'
  const totalMinutes = Math.floor(totalMs / 60000)
  const hours = Math.floor(totalMinutes / 60)
  const minutes = totalMinutes % 60
  if (hours > 0 && minutes > 0) {
    return i18next.t('channels.placeholders.planActivityDurationHm', {
      hours,
      minutes,
    })
  }
  if (hours > 0) {
    return i18next.t('channels.placeholders.planActivityDurationH', { hours })
  }
  return i18next.t('channels.placeholders.planActivityDurationM', { minutes })
}

// buildActivityHeatmap 把后端返回的 365 天 series 排布成 GitHub 风格周历：
// 每列为一个自然周（周一起始），缺失的日期按 0 Token 补齐，
// 同时生成月份标签（每月首周）和周一/三/五的星期标签。
function buildActivityHeatmap(series: GlmActivityDay[]): {
  weeks: ActivityHeatmapCell[][]
  monthLabels: string[]
  weekdayLabels: string[]
  maxTokens: number
} {
  const byDate = new Map<string, GlmActivityDay>()
  series.forEach((item) => {
    if (item?.date) byDate.set(item.date, item)
  })

  const today = new Date()
  today.setHours(0, 0, 0, 0)
  const rangeStart = new Date(today)
  rangeStart.setDate(rangeStart.getDate() - 364)
  // 对齐到周一，保证每列恰好是一个自然周
  const startPad = (rangeStart.getDay() + 6) % 7
  const gridStart = new Date(rangeStart)
  gridStart.setDate(gridStart.getDate() - startPad)

  const weeks: ActivityHeatmapCell[][] = []
  let week: ActivityHeatmapCell[] = []
  let maxTokens = 0
  const cursor = new Date(gridStart)
  while (cursor <= today) {
    const key = formatActivityDateKey(cursor)
    const entry = byDate.get(key)
    const tokens = Number(entry?.totalTokens) || 0
    if (tokens > maxTokens) maxTokens = tokens
    week.push({
      date: key,
      tokens,
      mcpCalls: Number(entry?.mcpCalls) || 0,
    })
    if (week.length === 7) {
      weeks.push(week)
      week = []
    }
    cursor.setDate(cursor.getDate() + 1)
  }
  if (week.length > 0) weeks.push(week)

  const monthFormatter = new Intl.DateTimeFormat(i18next.language, {
    month: 'short',
  })
  const weekdayFormatter = new Intl.DateTimeFormat(i18next.language, {
    weekday: 'narrow',
  })
  // 2024-01-01 恰为周一，用它推导周一到周日的窄缩写
  const weekdayLabels = Array.from({ length: 7 }, (_, index) =>
    weekdayFormatter.format(new Date(2024, 0, 1 + index))
  )

  let lastMonth = ''
  const monthLabels = weeks.map((cells) => {
    const firstDate = new Date(`${cells[0].date}T00:00:00`)
    const name = monthFormatter.format(firstDate)
    if (name !== lastMonth) {
      lastMonth = name
      return name
    }
    return ''
  })

  return { weeks, monthLabels, weekdayLabels, maxTokens }
}

function activityCellTitle(cell: ActivityHeatmapCell): string {
  return [
    cell.date,
    i18next.t('channels.tips.planActivityModelUsage', {
      count: formatCompactNumber(cell.tokens),
    }),
    i18next.t('channels.tips.planActivityToolCalls', {
      count: cell.mcpCalls.toLocaleString(),
    }),
  ].join('\n')
}

function ActivityStat({
  label,
  value,
  hint,
}: {
  label: string
  value: string
  hint?: string
}) {
  const stat = (
    <div className='rounded-lg border p-3'>
      <div className='text-muted-foreground text-xs'>{label}</div>
      <div className='mt-1 text-xl font-semibold tabular-nums'>{value}</div>
    </div>
  )
  if (!hint) return stat
  return (
    <Tooltip>
      <TooltipTrigger render={<div />}>{stat}</TooltipTrigger>
      <TooltipContent>{hint}</TooltipContent>
    </Tooltip>
  )
}

function ActivitySection({ channelId }: { channelId: number }) {
  const { t } = useTranslation()
  const [loading, setLoading] = useState(false)
  const [loadError, setLoadError] = useState(false)
  const [activity, setActivity] = useState<GlmPlanActivityData | null>(null)

  const fetchActivity = useCallback(async () => {
    setLoading(true)
    setLoadError(false)
    try {
      const response = await getGlmPlanActivity(channelId)
      if (!response.success) {
        throw new Error(
          response.message || t('channels.tips.planActivityLoadFailed')
        )
      }
      setActivity(response.data ?? null)
    } catch {
      setActivity(null)
      setLoadError(true)
    } finally {
      setLoading(false)
    }
  }, [channelId, t])

  useEffect(() => {
    fetchActivity()
  }, [fetchActivity])

  const summary = activity?.summary
  const series = useMemo(
    () => (Array.isArray(activity?.series) ? activity.series : []),
    [activity]
  )
  const hasData = Boolean(summary) || series.length > 0
  const { weeks, monthLabels, weekdayLabels, maxTokens } = useMemo(
    () => buildActivityHeatmap(series),
    [series]
  )

  const summaryContent = (
    <div className='grid grid-cols-2 gap-3 sm:grid-cols-3 lg:grid-cols-5'>
      <ActivityStat
        label={t('channels.fields.planTotalTokens')}
        value={formatCompactNumber(summary?.totalTokens)}
      />
      <ActivityStat
        label={t('channels.fields.planPeakTokens')}
        value={formatCompactNumber(summary?.peakDailyTokens)}
        hint={
          summary?.peakDailyTokensDate
            ? `${t('channels.fields.planPeakTokensDate')}: ${summary.peakDailyTokensDate}`
            : undefined
        }
      />
      <ActivityStat
        label={t('channels.fields.planTotalUsageDuration')}
        value={formatActivityDuration(summary?.totalUsageDurationMs)}
      />
      <ActivityStat
        label={t('channels.fields.planCurrentStreakDays')}
        value={String(summary?.currentStreakDays ?? 0)}
      />
      <ActivityStat
        label={t('channels.fields.planLongestStreakDays')}
        value={String(summary?.longestStreakDays ?? 0)}
      />
    </div>
  )

  const heatmapContent = (
    <div className='space-y-1.5'>
      <div className='flex items-center justify-between gap-2'>
        <div className='text-muted-foreground text-xs font-medium'>
          {t('channels.fields.planActivityHeatmapTitle')}
        </div>
        <div className='text-muted-foreground flex items-center gap-1 text-xs'>
          {t('channels.fields.planActivityHeatmapLegendLess')}
          {ACTIVITY_HEAT_COLORS.map((color, index) => (
            <span
              key={index}
              className='size-2.5 rounded-[3px]'
              style={{ backgroundColor: color }}
            />
          ))}
          {t('channels.fields.planActivityHeatmapLegendMore')}
        </div>
      </div>
      <div className='overflow-x-auto pb-1'>
        <div className='flex gap-1'>
          <div className='flex flex-col gap-[2px] pt-[18px]'>
            {weekdayLabels.map((label, index) => (
              <div
                key={index}
                className='text-muted-foreground flex flex-1 items-center justify-end pr-1 text-[9px] leading-none'
              >
                {index % 2 === 0 ? label : ''}
              </div>
            ))}
          </div>
          <div className='flex-1'>
            <div className='mb-1 flex h-3.5 items-end gap-[2px]'>
              {monthLabels.map((label, index) => (
                <div
                  key={index}
                  className='text-muted-foreground min-w-0 flex-1 text-[9px] leading-none whitespace-nowrap'
                >
                  {label}
                </div>
              ))}
            </div>
            <div className='flex gap-[2px]'>
              {weeks.map((week, weekIndex) => (
                <div key={weekIndex} className='flex flex-1 flex-col gap-[2px]'>
                  {week.map((cell) => (
                    <div
                      key={cell.date}
                      className='aspect-square w-full min-w-[10px] rounded-[3px]'
                      style={{
                        backgroundColor:
                          ACTIVITY_HEAT_COLORS[
                            activityHeatLevel(cell.tokens, maxTokens)
                          ],
                      }}
                      title={activityCellTitle(cell)}
                      role='img'
                      aria-label={activityCellTitle(cell)}
                    />
                  ))}
                </div>
              ))}
            </div>
          </div>
        </div>
      </div>
    </div>
  )

  if (loading) {
    return (
      <section className='space-y-3 rounded-lg border p-4'>
        <div className='flex flex-wrap items-center justify-between gap-2'>
          <div className='text-sm font-semibold'>
            {t('channels.titles.planActivity')}
          </div>
        </div>
        <div className='flex h-32 items-center justify-center'>
          <Loader2 className='text-muted-foreground h-5 w-5 animate-spin' />
        </div>
      </section>
    )
  }

  if (loadError) {
    return (
      <section className='space-y-3 rounded-lg border p-4'>
        <div className='flex flex-wrap items-center justify-between gap-2'>
          <div className='text-sm font-semibold'>
            {t('channels.titles.planActivity')}
          </div>
          <Button
            type='button'
            variant='outline'
            size='sm'
            onClick={fetchActivity}
            disabled={loading}
          >
            {loading ? (
              <Loader2 className='mr-1.5 h-3.5 w-3.5 animate-spin' />
            ) : (
              <RefreshCw className='mr-1.5 h-3.5 w-3.5' />
            )}
            {t('channels.actions.refresh')}
          </Button>
        </div>
        <div className='text-muted-foreground flex h-24 items-center justify-center text-sm'>
          {t('channels.tips.planActivityLoadFailed')}
        </div>
      </section>
    )
  }

  if (!hasData) {
    return (
      <section className='space-y-3 rounded-lg border p-4'>
        <div className='flex flex-wrap items-center justify-between gap-2'>
          <div className='text-sm font-semibold'>
            {t('channels.titles.planActivity')}
          </div>
        </div>
        <div className='text-muted-foreground flex h-24 items-center justify-center text-sm'>
          {t('channels.fields.planActivityNoData')}
        </div>
      </section>
    )
  }

  return (
    <div className='space-y-3'>
      {/* 板块一：汇总统计 */}
      <section className='rounded-lg border p-4'>
        <div className='mb-3 text-sm font-semibold'>
          {t('channels.titles.planActivity')}
        </div>
        {summaryContent}
      </section>

      {/* 板块二：Token 活动热力图 */}
      <section className='rounded-lg border p-4'>
        <div className='mb-3 flex flex-wrap items-center justify-between gap-2'>
          <div className='text-sm font-semibold'>
            {t('channels.fields.planActivityHeatmapTitle')}
          </div>
          <Button
            type='button'
            variant='outline'
            size='sm'
            onClick={fetchActivity}
            disabled={loading}
          >
            {loading ? (
              <Loader2 className='mr-1.5 h-3.5 w-3.5 animate-spin' />
            ) : (
              <RefreshCw className='mr-1.5 h-3.5 w-3.5' />
            )}
            {t('channels.actions.refresh')}
          </Button>
        </div>
        {heatmapContent}
        <div className='text-muted-foreground mt-2 text-right text-xs'>
          {t('channels.tips.planDataDelay')}
        </div>
      </section>
    </div>
  )
}

function GlmPlanContent({
  channel,
  quotaData,
}: {
  channel: Channel
  quotaData: PlanQuotaData
}) {
  const { t } = useTranslation()
  const visibleLimits = [
    quotaData.token_limit,
    quotaData.weekly_limit,
    quotaData.mcp_tool_limit,
  ].filter(Boolean).length

  return (
    <div className='space-y-4'>
      <div className='rounded-lg border p-4'>
        <div className='flex flex-wrap items-center justify-between gap-2'>
          <div className='text-base font-semibold'>
            {quotaData.product_name || getPlanDisplayName(quotaData.plan_name)}
          </div>
          <div className='flex flex-wrap gap-2'>
            {quotaData.plan_version && (
              <StatusBadge
                label={`${quotaData.plan_version}${t('channels.fields.planPackage')}`}
                variant={
                  quotaData.plan_version === '新' ? 'success' : 'warning'
                }
                copyable={false}
              />
            )}
            {quotaData.product_level && (
              <StatusBadge
                label={quotaData.product_level}
                variant='info'
                copyable={false}
              />
            )}
          </div>
        </div>
        <div className='text-muted-foreground mt-3 grid gap-2 text-xs sm:grid-cols-3'>
          <div>
            {t('channels.fields.planEffectiveDate')}:{' '}
            {quotaData.effective_date || '-'}
          </div>
          <div>
            {t('channels.fields.planExpiryDate')}:{' '}
            {quotaData.expiry_date || '-'}
          </div>
          <div>
            {quotaData.auto_renew ? (
              <span className='text-success inline-flex items-center gap-1'>
                <CheckCircle2 className='h-3.5 w-3.5' />
                {t('channels.actions.planAutoRenew')}
              </span>
            ) : (
              <span className='text-warning inline-flex items-center gap-1'>
                <AlertTriangle className='h-3.5 w-3.5' />
                {t('channels.actions.planNotRenewed')}
              </span>
            )}
          </div>
        </div>
      </div>

      <div
        className={`grid gap-3 ${visibleLimits >= 3 ? 'lg:grid-cols-3' : 'sm:grid-cols-2'}`}
      >
        <LimitCard
          title={t('channels.titles.planFiveHourLimit')}
          data={quotaData.token_limit}
          resetLabel={formatHourReset(quotaData.token_limit?.next_reset_time)}
        />
        <LimitCard
          title={t('channels.titles.planWeeklyLimit')}
          data={quotaData.weekly_limit}
          resetLabel={
            formatResetTime(quotaData.weekly_limit?.next_reset_time)
              ? `${t('channels.fields.planNextReset')}: ${formatResetTime(quotaData.weekly_limit?.next_reset_time)}`
              : ''
          }
        />
        <McpLimitCard data={quotaData.mcp_tool_limit} />
      </div>

      <UsageChart channelId={channel.id} />
      <PerformanceChart
        channelId={channel.id}
        productLevel={quotaData.product_level}
      />
      <ActivitySection channelId={channel.id} />
    </div>
  )
}

function TierPlanContent({ quotaData }: { quotaData: PlanQuotaData }) {
  const { t } = useTranslation()
  return (
    <div className='space-y-4'>
      {quotaData.credential === 'expired' && (
        <div className='border-warning/30 bg-warning/10 text-warning rounded-lg border px-4 py-3 text-sm'>
          {t('channels.tips.planApiKeyInvalidOrExpired')}
        </div>
      )}
      {quotaData.credential === 'error' && (
        <div className='border-destructive/30 bg-destructive/10 text-destructive rounded-lg border px-4 py-3 text-sm'>
          {t('channels.tips.planResponseParseFailed')}
        </div>
      )}

      <div className='rounded-lg border p-4'>
        <div className='flex flex-wrap items-center justify-between gap-2'>
          <div className='text-base font-semibold'>
            {getPlanDisplayName(quotaData.plan_name)}
          </div>
          {quotaData.credential === 'valid' && (
            <StatusBadge
              label={t('channels.actions.planValid')}
              variant='success'
              copyable={false}
            />
          )}
        </div>
      </div>

      {quotaData.tiers && quotaData.tiers.length > 0 ? (
        <div
          className={`grid gap-3 ${quotaData.tiers.length >= 3 ? 'lg:grid-cols-3' : 'sm:grid-cols-2'}`}
        >
          {quotaData.tiers.map((tier, index) => (
            <TierLimitCard
              key={`${tier.name ?? 'tier'}-${index}`}
              tier={tier}
            />
          ))}
        </div>
      ) : (
        <div className='text-muted-foreground rounded-lg border px-4 py-10 text-center text-sm'>
          {t('channels.tips.planNoQuotaData')}
        </div>
      )}
    </div>
  )
}

export function PlanQuotaDialog({ open, onOpenChange }: PlanQuotaDialogProps) {
  const { t } = useTranslation()
  const { currentRow } = useChannels()
  const [loading, setLoading] = useState(false)
  const [quotaData, setQuotaData] = useState<PlanQuotaData | null>(null)

  const fetchQuotaData = useCallback(async () => {
    if (!currentRow?.id) return
    setLoading(true)
    try {
      const response = await getPlanQuota(currentRow.id)
      if (!response.success) {
        throw new Error(
          response.message || t('channels.errors.failedToQueryPlanUsage')
        )
      }
      setQuotaData(response.data ?? null)
    } catch (error) {
      toast.error(
        error instanceof Error
          ? error.message
          : t('channels.errors.failedToQueryPlanUsage')
      )
      setQuotaData(null)
    } finally {
      setLoading(false)
    }
  }, [currentRow?.id, t])

  useEffect(() => {
    if (open && currentRow?.id) {
      fetchQuotaData()
    }
    if (!open) {
      setQuotaData(null)
    }
  }, [currentRow?.id, fetchQuotaData, open])

  if (!currentRow) return null

  const planName = quotaData?.plan_name || currentRow.channel_info?.plan_name
  const planDisplayName = getPlanDisplayName(planName)
  const isGlmData = isGlmPlan(planName) && !!quotaData?.product_name
  const isTierData = isTierBasedPlan(planName)

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className='max-h-[90vh] sm:max-w-5xl'>
        <DialogHeader>
          <DialogTitle>{t('channels.fields.planUsage')}</DialogTitle>
          <DialogDescription>
            {currentRow.name} {currentRow.id ? `#${currentRow.id}` : ''}
            {planDisplayName ? ` · ${planDisplayName}` : ''}
          </DialogDescription>
        </DialogHeader>

        <ScrollArea className='max-h-[calc(90vh-11rem)] pr-3'>
          {loading ? (
            <div className='flex h-64 items-center justify-center'>
              <Loader2 className='text-muted-foreground h-6 w-6 animate-spin' />
            </div>
          ) : quotaData && isGlmData ? (
            <GlmPlanContent channel={currentRow} quotaData={quotaData} />
          ) : quotaData && isTierData ? (
            <TierPlanContent quotaData={quotaData} />
          ) : quotaData?.quota_supported === false ? (
            <div className='text-muted-foreground rounded-lg border px-4 py-12 text-center text-sm'>
              {planDisplayName
                ? t('channels.tips.planQuotaComingSoon', {
                    name: planDisplayName,
                  })
                : t('channels.tips.planQuotaComingSoonGeneric')}
            </div>
          ) : (
            <div className='text-muted-foreground rounded-lg border px-4 py-12 text-center text-sm'>
              {t('channels.fields.noData')}
            </div>
          )}
        </ScrollArea>

        <DialogFooter>
          <Button
            type='button'
            variant='outline'
            onClick={fetchQuotaData}
            disabled={loading}
          >
            {loading ? (
              <Loader2 className='mr-1.5 h-4 w-4 animate-spin' />
            ) : (
              <RefreshCw className='mr-1.5 h-4 w-4' />
            )}
            {t('channels.actions.refresh')}
          </Button>
          <Button type='button' onClick={() => onOpenChange(false)}>
            {t('common.actions.close')}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}
