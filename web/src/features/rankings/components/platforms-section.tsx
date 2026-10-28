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
import { VChart } from '@visactor/react-vchart'
import { Bot, Trophy } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { getLobeIcon } from '@/lib/lobe-icon'
import { getPlatformMeta } from '@/lib/platform-meta'
import { useChartTheme } from '@/lib/use-chart-theme'
import { VCHART_OPTION } from '@/lib/vchart'
import { formatTokens } from '../lib/format'
import type {
  PlatformHistorySeries,
  PlatformRanking,
  RankingPeriod,
} from '../types'
import { GrowthText } from './growth-text'

const PERIOD_DESCRIPTIONS: Record<RankingPeriod, string> = {
  today: 'common.tips.hourlyTokenUsageByAgentToday',
  week: 'common.tips.dailyTokenUsageByAgentThisWeek',
  month: 'common.tips.dailyTokenUsageByAgentThisMonth',
  year: 'common.tips.weeklyTokenUsageByAgentThisYear',
  all: 'common.tips.tokenUsageByAgentSinceLaunch',
}

const TOOLTIP_MAX_ROWS = 10

type PlatformsSectionProps = {
  history: PlatformHistorySeries
  rows: PlatformRanking[]
  period: RankingPeriod
}

/**
 * Combined "Top Agents" card: a stacked bar chart showing token usage by
 * client platform over time, paired below with a two-column agent
 * leaderboard. Mirrors ModelsSection; platform ids map to display names
 * and icons via src/lib/platform-meta.ts.
 */
export function PlatformsSection(props: PlatformsSectionProps) {
  const { t } = useTranslation()
  const { resolvedTheme, themeReady } = useChartTheme()

  // Order points so the largest platform appears at the bottom of every stack.
  const orderedPoints = useMemo(() => {
    const order = new Map(
      props.history.platforms.map((p, idx) => [p.name, idx] as const)
    )
    return [...props.history.points].sort((a, b) => {
      const tsCmp = a.ts.localeCompare(b.ts)
      if (tsCmp !== 0) return tsCmp
      return (order.get(a.platform) ?? 999) - (order.get(b.platform) ?? 999)
    })
  }, [props.history])

  const totalTokens = useMemo(
    () => props.rows.reduce((s, r) => s + r.total_tokens, 0),
    [props.rows]
  )

  const spec = useMemo(() => {
    if (orderedPoints.length === 0) return null
    const platformLabel = (platform: string) =>
      platform === 'Others'
        ? t('rankings.fields.others')
        : getPlatformMeta(platform).name
    return {
      type: 'bar' as const,
      data: [{ id: 'platforms-history', values: orderedPoints }],
      xField: 'label',
      yField: 'tokens',
      seriesField: 'platform',
      stack: true,
      bar: {
        style: { cornerRadius: 0 },
      },
      legends: { visible: false },
      axes: [
        {
          orient: 'bottom',
          label: {
            style: { fill: 'currentColor', fontSize: 10 },
            autoHide: true,
            autoLimit: true,
          },
          tick: { visible: false },
        },
        {
          orient: 'left',
          label: {
            formatMethod: (val: number | string) => formatTokens(Number(val)),
            style: { fill: 'currentColor', fontSize: 10 },
          },
          grid: { visible: true, style: { lineDash: [3, 3] } },
        },
      ],
      tooltip: {
        mark: {
          content: [
            {
              key: (datum: Record<string, unknown>) =>
                platformLabel(String(datum?.platform ?? '')),
              value: (datum: Record<string, unknown>) =>
                formatTokens(Number(datum?.tokens) || 0),
            },
          ],
        },
        dimension: {
          title: {
            value: (datum: Record<string, unknown>) =>
              String(datum?.label ?? ''),
          },
          content: [
            {
              key: (datum: Record<string, unknown>) =>
                platformLabel(String(datum?.platform ?? '')),
              value: (datum: Record<string, unknown>) =>
                Number(datum?.tokens) || 0,
            },
          ],
          updateContent: (
            array: Array<{ key: string; value: string | number }>
          ) => {
            array.sort((a, b) => Number(b.value) - Number(a.value))
            const sum = array.reduce((s, x) => s + (Number(x.value) || 0), 0)
            const visible = array.slice(0, TOOLTIP_MAX_ROWS)
            const overflow = array.slice(TOOLTIP_MAX_ROWS)
            const result = visible.map((item) => ({
              key: item.key,
              value: formatTokens(Number(item.value) || 0),
            }))
            if (overflow.length > 0) {
              const otherSum = overflow.reduce(
                (s, item) => s + (Number(item.value) || 0),
                0
              )
              result.push({
                key: t('rankings.fields.countMore', { count: overflow.length }),
                value: formatTokens(otherSum),
              })
            }
            result.unshift({
              key: t('channels.fields.labelWithColon', {
                label: t('dashboard.fields.total'),
              }),
              value: formatTokens(sum),
            })
            return result
          },
        },
      },
      animationAppear: { duration: 500 },
    }
  }, [orderedPoints, t])

  return (
    <section className='bg-card overflow-hidden rounded-lg border'>
      {/* Chart block ----------------------------------------------------- */}
      <header className='flex items-start justify-between gap-4 px-5 py-4'>
        <div className='min-w-0 flex-1'>
          <h2 className='text-foreground inline-flex items-center gap-2 text-base font-semibold'>
            <Bot className='text-primary size-4' />
            {t('rankings.titles.topAgents')}
          </h2>
          <p className='text-muted-foreground mt-1 text-sm'>
            {t(PERIOD_DESCRIPTIONS[props.period])}
          </p>
        </div>
        <div className='shrink-0 text-right'>
          <div className='text-foreground font-mono text-2xl font-semibold tabular-nums'>
            {formatTokens(totalTokens)}
          </div>
          <div className='text-muted-foreground/80 text-[10px] font-medium tracking-widest uppercase'>
            {t('rankings.fields.tokens')}
          </div>
        </div>
      </header>

      <div className='px-5 pb-5'>
        <div className='h-60 sm:h-72'>
          {themeReady && spec ? (
            <VChart
              key={`platforms-history-${resolvedTheme}-${props.period}`}
              spec={{
                ...spec,
                theme: resolvedTheme === 'dark' ? 'dark' : 'light',
                background: 'transparent',
              }}
              option={VCHART_OPTION}
            />
          ) : (
            <div className='text-muted-foreground/80 flex h-full items-center justify-center text-xs'>
              {t('rankings.fields.noAgentDataAvailable')}
            </div>
          )}
        </div>
      </div>

      {/* Leaderboard block ----------------------------------------------- */}
      <div className='border-t'>
        <header className='px-5 pt-4 pb-2'>
          <h3 className='text-foreground inline-flex items-center gap-2 text-sm font-semibold'>
            <Trophy className='size-3.5 text-amber-500' />
            {t('rankings.fields.agentLeaderboard')}
          </h3>
          <p className='text-muted-foreground/80 mt-0.5 text-xs'>
            {t('rankings.tips.compareTheMostPopularAgentsOnThePlatform')}
          </p>
        </header>
        {props.rows.length === 0 ? (
          <div className='text-muted-foreground/80 px-5 py-8 text-center text-sm'>
            {t('rankings.fields.noAgentDataAvailable')}
          </div>
        ) : (
          <div className='px-5 pt-1 pb-4'>
            <PlatformLeaderboard rows={props.rows} />
          </div>
        )}
      </div>
    </section>
  )
}

function PlatformLeaderboard(props: { rows: PlatformRanking[] }) {
  const { t } = useTranslation()
  const half = Math.ceil(props.rows.length / 2)
  const columns = [props.rows.slice(0, half), props.rows.slice(half)]

  return (
    <div className='grid grid-cols-1 gap-x-8 md:grid-cols-2'>
      {columns.map((rows, colIdx) => (
        <ul key={colIdx}>
          {rows.map((row) => (
            <li key={row.platform} className='flex items-center gap-3 py-2.5'>
              <span className='text-muted-foreground/80 w-6 shrink-0 text-right font-mono text-xs tabular-nums'>
                {row.rank}.
              </span>
              <span className='shrink-0'>
                {getLobeIcon(getPlatformMeta(row.platform).icon, 22)}
              </span>
              <div className='min-w-0 flex-1'>
                <span className='text-foreground block truncate text-sm font-medium'>
                  {getPlatformMeta(row.platform).name}
                </span>
              </div>
              <div className='shrink-0 text-right'>
                <div className='text-foreground font-mono text-sm font-semibold tabular-nums'>
                  {formatTokens(row.total_tokens)}{' '}
                  <span className='text-muted-foreground/80 font-normal'>
                    {t('rankings.fields.tokens')}
                  </span>
                </div>
                <GrowthText value={row.growth_pct} className='text-[11px]' />
              </div>
            </li>
          ))}
        </ul>
      ))}
    </div>
  )
}
