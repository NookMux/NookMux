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
import { useQuery } from '@tanstack/react-query'
import { VChart } from '@visactor/react-vchart'
import type { ISpec } from '@visactor/vchart'
import { Activity, RotateCw, Timer, Zap } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import dayjs from '@/lib/dayjs'
import { useChartTheme } from '@/lib/use-chart-theme'
import { cn } from '@/lib/utils'
import { VCHART_OPTION } from '@/lib/vchart'
import { Button } from '@/components/ui/button'
import { Skeleton } from '@/components/ui/skeleton'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'
import { GroupBadge } from '@/components/group-badge'
import { getModelPerfMetrics } from '@/features/performance-metrics/api'
import {
  formatLatency,
  formatSuccessRate,
  formatThroughput,
  getSuccessRateTone,
} from '@/features/performance-metrics/lib/format'
import type { PerfSeriesPoint } from '@/features/performance-metrics/types'

const PERF_WINDOW_HOURS = 24

function SectionTitle(props: { children: React.ReactNode }) {
  return (
    <h2 className='text-muted-foreground mb-3 text-xs font-semibold tracking-wider uppercase'>
      {props.children}
    </h2>
  )
}

function formatBucketTime(ts: number): string {
  return dayjs(ts * 1000).format('MM-DD HH:mm')
}

// Chart series colors come from the theme chart palette tokens (--chart-*).
function getChartPaletteColor(name: '--chart-1' | '--chart-2'): string {
  if (typeof document === 'undefined') return ''
  const bodyStyle = window.getComputedStyle(document.body)
  const rootStyle = window.getComputedStyle(document.documentElement)
  return (bodyStyle.getPropertyValue(name) || rootStyle.getPropertyValue(name))
    .toString()
    .trim()
}

type TrendKind = 'ttft' | 'success-rate'

function buildTrendSpec(
  points: PerfSeriesPoint[],
  kind: TrendKind,
  title: string,
  color: string,
  theme: string
): ISpec {
  const values = points.map((point) => ({
    time: formatBucketTime(point.ts),
    value: kind === 'ttft' ? point.avg_ttft_ms : point.success_rate,
  }))
  const formatValue =
    kind === 'ttft'
      ? (value: number) => formatLatency(value)
      : (value: number) => formatSuccessRate(value)

  const spec: Record<string, unknown> = {
    type: 'line',
    data: [{ id: `${kind}Trend`, values }],
    xField: 'time',
    yField: 'value',
    title: { visible: true, text: title },
    legends: { visible: false },
    line: { style: { lineWidth: 2, curveType: 'monotone' } },
    point: { visible: values.length <= 30, style: { size: 5 } },
    tooltip: {
      mark: {
        content: [
          {
            key: (datum: Record<string, unknown>) => String(datum?.time ?? ''),
            value: (datum: Record<string, unknown>) =>
              formatValue(Number(datum?.value) || 0),
          },
        ],
      },
    },
    background: 'transparent',
    theme,
  }
  if (color) spec.color = color
  if (kind === 'success-rate') {
    spec.axes = [
      {
        orient: 'left',
        type: 'linear',
        min: 0,
        max: 100,
        label: { formatMethod: (value: number) => `${value}%` },
      },
    ]
  }
  return spec as unknown as ISpec
}

function PerfKpi(props: {
  icon: React.ComponentType<{ className?: string }>
  label: string
  value: React.ReactNode
  valueClassName?: string
}) {
  const Icon = props.icon
  return (
    <div className='bg-muted/20 flex min-w-0 flex-col gap-0.5 rounded-lg border px-3 py-2.5'>
      <span className='text-muted-foreground inline-flex min-w-0 items-center gap-1 text-[10px] font-medium tracking-wider uppercase'>
        <Icon className='size-3 shrink-0' />
        <span className='truncate'>{props.label}</span>
      </span>
      <span
        className={cn(
          'text-foreground truncate font-mono text-sm font-semibold tabular-nums',
          props.valueClassName
        )}
      >
        {props.value}
      </span>
    </div>
  )
}

function SuccessRateValue(props: { rate: number }) {
  const tone = getSuccessRateTone(props.rate)
  return (
    <span className={cn('inline-flex items-center gap-1.5', tone.textClass)}>
      <span
        className={cn('inline-block size-1.5 rounded-full', tone.dotClass)}
      />
      {formatSuccessRate(props.rate)}
    </span>
  )
}

export interface ModelDetailsPerformanceProps {
  modelName: string
}

export function ModelDetailsPerformance(props: ModelDetailsPerformanceProps) {
  const { t } = useTranslation()
  const { resolvedTheme, themeReady } = useChartTheme()

  const { data, isLoading, isError, refetch } = useQuery({
    queryKey: ['model-perf-detail', props.modelName],
    queryFn: () => getModelPerfMetrics(props.modelName, PERF_WINDOW_HOURS),
    staleTime: 60 * 1000,
    retry: false,
    enabled: Boolean(props.modelName),
  })

  const themeName = resolvedTheme === 'dark' ? 'dark' : 'light'
  const ttftSpec = useMemo(
    () =>
      data && data.series.length > 0
        ? buildTrendSpec(
            data.series,
            'ttft',
            t('pricing.titles.ttftTrend'),
            getChartPaletteColor('--chart-1'),
            themeName
          )
        : null,
    [data, t, themeName]
  )
  const successRateSpec = useMemo(
    () =>
      data && data.series.length > 0
        ? buildTrendSpec(
            data.series,
            'success-rate',
            t('pricing.titles.successRateTrend'),
            getChartPaletteColor('--chart-2'),
            themeName
          )
        : null,
    [data, t, themeName]
  )

  if (isLoading) {
    return (
      <div className='space-y-4'>
        <div className='grid grid-cols-1 gap-2 sm:grid-cols-3'>
          <Skeleton className='h-16' />
          <Skeleton className='h-16' />
          <Skeleton className='h-16' />
        </div>
        <Skeleton className='h-40' />
        <div className='grid gap-4 @2xl/details:grid-cols-2'>
          <Skeleton className='h-[240px]' />
          <Skeleton className='h-[240px]' />
        </div>
      </div>
    )
  }

  if (isError) {
    return (
      <div className='flex flex-col items-start gap-3 py-2'>
        <p className='text-muted-foreground/60 italic'>
          {t('pricing.errors.failedToLoadPerformanceData')}
        </p>
        <Button
          type='button'
          variant='outline'
          size='sm'
          className='gap-1.5'
          onClick={() => refetch()}
        >
          <RotateCw className='size-3.5' />
          {t('common.actions.retry')}
        </Button>
      </div>
    )
  }

  const summary = data?.summary ?? null
  const series = data?.series ?? []
  if (!summary || series.length === 0) {
    return (
      <p className='text-muted-foreground/60 py-2 italic'>
        {t('pricing.tips.noPerformanceData')}
      </p>
    )
  }

  const groups = data?.groups ?? []
  const thClass =
    'text-muted-foreground py-2 text-[10px] font-medium tracking-wider uppercase'

  return (
    <div className='space-y-6'>
      <section>
        <div className='mb-3 flex items-baseline justify-between gap-3'>
          <SectionTitle>{t('pricing.titles.performance')}</SectionTitle>
          <span className='text-muted-foreground/40 text-[10px]'>
            {t('pricing.tips.perfWindowHours', { hours: PERF_WINDOW_HOURS })}
          </span>
        </div>
        <div className='grid grid-cols-1 gap-2 sm:grid-cols-3'>
          <PerfKpi
            icon={Zap}
            label={t('pricing.fields.throughput')}
            value={formatThroughput(summary.avg_tps)}
          />
          <PerfKpi
            icon={Timer}
            label={t('pricing.fields.averageLatency')}
            value={formatLatency(summary.avg_latency_ms)}
          />
          <PerfKpi
            icon={Activity}
            label={t('pricing.status.successRate')}
            value={<SuccessRateValue rate={summary.success_rate} />}
          />
        </div>
      </section>

      <section>
        <SectionTitle>{t('pricing.titles.groupPerformance')}</SectionTitle>
        {groups.length === 0 ? (
          <p className='text-muted-foreground/60 italic'>
            {t('pricing.tips.noPerformanceData')}
          </p>
        ) : (
          <div className='-mx-4 overflow-x-auto sm:mx-0'>
            <Table className='text-sm'>
              <TableHeader>
                <TableRow className='hover:bg-transparent'>
                  <TableHead className={thClass}>
                    {t('common.fields.group')}
                  </TableHead>
                  <TableHead className={`${thClass} text-right`}>
                    {t('pricing.fields.firstTokenLatency')}
                  </TableHead>
                  <TableHead className={`${thClass} text-right`}>
                    {t('pricing.fields.averageLatency')}
                  </TableHead>
                  <TableHead className={`${thClass} text-right`}>
                    {t('pricing.status.successRate')}
                  </TableHead>
                  <TableHead className={`${thClass} text-right`}>
                    {t('pricing.fields.throughput')}
                  </TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {groups.map((group) => (
                  <TableRow key={group.group}>
                    <TableCell className='py-2.5'>
                      <GroupBadge group={group.group} size='sm' />
                    </TableCell>
                    <TableCell className='py-2.5 text-right font-mono'>
                      {formatLatency(group.avg_ttft_ms)}
                    </TableCell>
                    <TableCell className='py-2.5 text-right font-mono'>
                      {formatLatency(group.avg_latency_ms)}
                    </TableCell>
                    <TableCell className='py-2.5 text-right'>
                      <SuccessRateValue rate={group.success_rate} />
                    </TableCell>
                    <TableCell className='py-2.5 text-right font-mono'>
                      {formatThroughput(group.avg_tps)}
                    </TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          </div>
        )}
      </section>

      <section className='grid gap-4 @2xl/details:grid-cols-2'>
        <div className='overflow-hidden rounded-lg border'>
          <div className='h-[240px] p-1.5'>
            {themeReady && ttftSpec && (
              <VChart
                key={`ttft-${themeName}-${series.length}`}
                spec={ttftSpec}
                option={VCHART_OPTION}
              />
            )}
          </div>
        </div>
        <div className='overflow-hidden rounded-lg border'>
          <div className='h-[240px] p-1.5'>
            {themeReady && successRateSpec && (
              <VChart
                key={`success-rate-${themeName}-${series.length}`}
                spec={successRateSpec}
                option={VCHART_OPTION}
              />
            )}
          </div>
        </div>
      </section>
    </div>
  )
}
