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
import { useId, type ComponentProps, type ReactNode } from 'react'
import type { LucideIcon } from 'lucide-react'
import { cn } from '@/lib/utils'
import {
  Progress,
  ProgressIndicator,
  ProgressTrack,
} from '@/components/ui/progress'

/** 相邻的兄弟元素之间画一条横线，首尾不留线。 */
const RULED = '[&>*+*]:border-t'

const STAT_LABEL_CLASS = 'text-muted-foreground text-xs leading-snug'

type SettingsPanelProps = {
  /** 标题；与页面标题重复时可以省略，例如只放一张列表的面板。 */
  title?: ReactNode
  description?: ReactNode
  icon?: LucideIcon
  /** 标题行右侧的辅助操作。 */
  actions?: ReactNode
  /** 底部操作栏，右对齐，主要操作放最右。 */
  footer?: ReactNode
  /** 危险区用 destructive：边框与标题改用危险色。 */
  tone?: 'default' | 'destructive'
  /** 直接子元素之间画横线，适合直接放 SettingsRow。 */
  divided?: boolean
  /** 内容贴边铺满并占满剩余高度，适合放表格与内部滚动区。 */
  flush?: boolean
  /** 内容占满剩余高度并在内部滚动；面板有确定的高度时才会生效。 */
  scroll?: boolean
  className?: string
  children?: ReactNode
}

/**
 * 面板：设置页里唯一带边框的容器。用于不随「保存」逐项生效的内容——
 * 立即执行的运维操作、只读的监控数据、危险操作，以及自带列表或编辑区、
 * 需要在内部滚动的编辑器。
 */
export function SettingsPanel({
  title,
  description,
  icon: Icon,
  actions,
  footer,
  tone = 'default',
  divided = false,
  flush = false,
  scroll = false,
  className,
  children,
}: SettingsPanelProps) {
  const titleId = useId()

  return (
    <section
      data-slot='settings-panel'
      aria-labelledby={title != null ? titleId : undefined}
      className={cn(
        'bg-card text-card-foreground ring-foreground/10 @container flex min-w-0 flex-col overflow-hidden rounded-xl ring-1',
        tone === 'destructive' && 'ring-destructive/30',
        className
      )}
    >
      {(title != null || description != null || actions != null) && (
        <div className='flex flex-col gap-1 px-4 py-3'>
          <div
            className={cn(
              'flex flex-wrap items-center gap-x-4 gap-y-2',
              title != null ? 'justify-between' : 'justify-end'
            )}
          >
            {title != null && (
              <div className='flex min-w-0 items-center gap-2'>
                {Icon != null && (
                  <Icon
                    className='text-muted-foreground size-4 shrink-0'
                    aria-hidden='true'
                  />
                )}
                <h3
                  id={titleId}
                  className={cn(
                    'text-sm leading-snug font-semibold',
                    tone === 'destructive' && 'text-destructive'
                  )}
                >
                  {title}
                </h3>
              </div>
            )}
            {actions != null && (
              <div className='flex min-w-0 flex-wrap items-center gap-2'>
                {actions}
              </div>
            )}
          </div>
          {description != null && (
            <p className='text-muted-foreground text-sm leading-snug'>
              {description}
            </p>
          )}
        </div>
      )}
      {children != null && (
        <div
          className={cn(
            'min-w-0',
            flush ? 'p-0' : 'px-4 py-2',
            (flush || scroll) && 'min-h-0 flex-1',
            scroll && 'overflow-y-auto',
            divided && RULED
          )}
        >
          {children}
        </div>
      )}
      {footer != null && (
        <div className='flex flex-wrap items-center justify-end gap-2 border-t px-4 py-3'>
          {footer}
        </div>
      )}
    </section>
  )
}

const STAT_GRID_COLUMNS = {
  2: 'grid-cols-2',
  3: 'grid-cols-2 @lg:grid-cols-3',
  4: 'grid-cols-2 @xl:grid-cols-4',
  5: 'grid-cols-2 @lg:grid-cols-3 @4xl:grid-cols-5',
  6: 'grid-cols-2 @lg:grid-cols-3 @5xl:grid-cols-6',
} as const

type SettingsStatGridProps = ComponentProps<'dl'> & {
  /** 容器足够宽时的列数；窄容器退到两列（5、6 列时先退到三列）。 */
  columns?: keyof typeof STAT_GRID_COLUMNS
}

/** 指标块行：一组「标签在上、数值在下」的只读指标。 */
export function SettingsStatGrid({
  columns = 6,
  className,
  ...props
}: SettingsStatGridProps) {
  return (
    <dl
      data-slot='settings-stat-grid'
      className={cn(
        'grid gap-x-6 gap-y-4',
        STAT_GRID_COLUMNS[columns],
        className
      )}
      {...props}
    />
  )
}

type SettingsStatProps = {
  label: ReactNode
  value: ReactNode
  detail?: ReactNode
  className?: string
}

export function SettingsStat({
  label,
  value,
  detail,
  className,
}: SettingsStatProps) {
  return (
    <div
      data-slot='settings-stat'
      className={cn('flex min-w-0 flex-col gap-0.5', className)}
    >
      <dt className={STAT_LABEL_CLASS}>{label}</dt>
      <dd className='text-base leading-snug font-semibold break-words tabular-nums'>
        {value}
      </dd>
      {detail != null && <dd className={STAT_LABEL_CLASS}>{detail}</dd>}
    </div>
  )
}

type SettingsMeterProps = {
  label: ReactNode
  /** 已格式化的读数，例如「已用 95.74 GB，共 117.56 GB」。 */
  valueText: ReactNode
  /** 0 到 100。 */
  percent: number
  className?: string
}

/** 用量条：标题与读数在上，进度条在下。 */
export function SettingsMeter({
  label,
  valueText,
  percent,
  className,
}: SettingsMeterProps) {
  const safePercent = Number.isFinite(percent)
    ? Math.min(100, Math.max(0, percent))
    : 0

  return (
    <div
      data-slot='settings-meter'
      className={cn('flex min-w-0 flex-col gap-2', className)}
    >
      <div className='flex flex-wrap items-baseline justify-between gap-x-4 gap-y-1'>
        <span className='text-sm leading-snug font-medium'>{label}</span>
        <span className='text-muted-foreground text-sm leading-snug tabular-nums'>
          {valueText}
        </span>
      </div>
      <Progress
        value={safePercent}
        aria-label={typeof label === 'string' ? label : undefined}
      >
        <ProgressTrack>
          <ProgressIndicator />
        </ProgressTrack>
      </Progress>
    </div>
  )
}
