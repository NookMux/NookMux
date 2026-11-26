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
import type { ComponentProps } from 'react'
import { cn } from '@/lib/utils'

/**
 * 设置页的表单容器：子元素纵向排列，分区之间留 2rem。
 * 它是设置行的查询容器：行是否分成标题、控件两列，由这里的宽度决定。
 */
export function SettingsForm({ className, ...props }: ComponentProps<'form'>) {
  return (
    <form
      className={cn('@container flex w-full min-w-0 flex-col gap-8', className)}
      {...props}
    />
  )
}
