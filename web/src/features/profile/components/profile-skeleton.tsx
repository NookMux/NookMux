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
import { Skeleton } from '@/components/ui/skeleton'
import { SettingsPanel } from '@/features/system-settings/components/settings-panel'

const STAT_SLOTS = ['balance', 'usage', 'requests'] as const
const GROUP_ROW_COUNTS = [4, 1]

/** 资料加载期间的占位：账户概览面板与两组设置行。 */
export function ProfileSkeleton() {
  return (
    <>
      <SettingsPanel>
        <div className='flex flex-col gap-4 py-2'>
          <div className='flex items-center gap-4'>
            <Skeleton className='size-12 rounded-xl sm:size-16 sm:rounded-2xl' />
            <div className='flex-1 space-y-2'>
              <Skeleton className='h-5 w-40' />
              <Skeleton className='h-4 w-64 max-w-full' />
            </div>
          </div>
          <div className='grid grid-cols-3 gap-6 border-t pt-4'>
            {STAT_SLOTS.map((slot) => (
              <div key={slot} className='space-y-2'>
                <Skeleton className='h-4 w-20' />
                <Skeleton className='h-5 w-28' />
                <Skeleton className='h-4 w-24' />
              </div>
            ))}
          </div>
        </div>
      </SettingsPanel>
      <div className='flex flex-col gap-8'>
        {GROUP_ROW_COUNTS.map((rowCount, groupIndex) => (
          <div key={groupIndex} className='flex flex-col'>
            <Skeleton className='mb-3 h-5 w-24' />
            {Array.from({ length: rowCount }).map((_, rowIndex) => (
              <div
                key={rowIndex}
                className='flex items-center justify-between gap-8 border-t py-3'
              >
                <div className='space-y-2'>
                  <Skeleton className='h-4 w-32' />
                  <Skeleton className='h-3 w-48 max-w-full' />
                </div>
                <Skeleton className='h-8 w-48' />
              </div>
            ))}
          </div>
        ))}
      </div>
    </>
  )
}
