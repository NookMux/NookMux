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
import { cn } from '@/lib/utils'
import { Skeleton } from '@/components/ui/skeleton'

/** 分区加载期间的占位：若干「标题加说明在左、按钮在右」的行。 */
export function SectionSkeleton({ rows }: { rows: number }) {
  return (
    <>
      {Array.from({ length: rows }).map((_, index) => (
        <div
          key={index}
          className='flex items-center justify-between gap-8 py-3'
        >
          <div className='space-y-2'>
            <Skeleton className='h-4 w-24' />
            <Skeleton className='h-3 w-40 max-w-full' />
          </div>
          <Skeleton className='h-7 w-16' />
        </div>
      ))}
    </>
  )
}

/** 登录方式的平铺网格：窄容器一列，32rem 起两列，64rem 起三列；方式增多时自动换行。 */
export const METHOD_GRID_CLASS =
  'grid grid-cols-1 gap-3 py-2 @lg:grid-cols-2 @5xl:grid-cols-3'

/** 平铺网格加载时的骨架：与登录方式的网格同样的列数。 */
export function TilesSkeleton() {
  return (
    <div className={METHOD_GRID_CLASS}>
      {[0, 1, 2].map((index) => (
        <div
          key={index}
          className={cn(
            'bg-card ring-foreground/10 flex items-center justify-between gap-6 rounded-xl px-4 py-3',
            'ring-1'
          )}
        >
          <div className='flex min-w-0 flex-1 flex-col gap-2'>
            <Skeleton className='h-4 w-24' />
            <Skeleton className='h-4 w-40 max-w-full' />
          </div>
          <Skeleton className='h-7 w-16 rounded-md' />
        </div>
      ))}
    </div>
  )
}
