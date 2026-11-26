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
import type { ReactNode } from 'react'
import { CircleAlert, TriangleAlert } from 'lucide-react'
import { cn } from '@/lib/utils'

type SettingsNoticeProps = {
  /** 说明用次要文字色；警告与危险带图标，并用语义色。 */
  tone?: 'info' | 'warning' | 'danger'
  /** 标题，放在正文前面。 */
  title?: ReactNode
  className?: string
  children: ReactNode
}

const NOTICE_TONE_CLASS = {
  info: 'text-muted-foreground',
  warning: 'text-warning',
  danger: 'text-destructive',
} as const

/**
 * 设置页里的一段提示：不另起边框，作为所在分区或面板里的一个条目，用横线与上下分隔。
 */
export function SettingsNotice({
  tone = 'info',
  title,
  className,
  children,
}: SettingsNoticeProps) {
  const Icon =
    tone === 'warning'
      ? TriangleAlert
      : tone === 'danger'
        ? CircleAlert
        : undefined

  return (
    <p
      className={cn(
        'flex items-start gap-2 py-2 text-sm leading-snug',
        NOTICE_TONE_CLASS[tone],
        className
      )}
    >
      {Icon != null && (
        <Icon className='mt-0.5 size-4 shrink-0' aria-hidden='true' />
      )}
      <span className='min-w-0'>
        {title != null && <span className='font-semibold'>{title}</span>}
        {title != null && ' '}
        {children}
      </span>
    </p>
  )
}
