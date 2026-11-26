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
export interface CalendarDay {
  date: Date
  /** 是否属于当前显示的月份；首尾补齐上一月、下一月的日期。 */
  inMonth: boolean
}

/** 以周日为一周之首的月历网格：补齐首尾后行数为整数。 */
export function buildCalendarDays(year: number, month: number): CalendarDay[] {
  const startDayOfWeek = new Date(year, month, 1).getDay()
  const daysInMonth = new Date(year, month + 1, 0).getDate()
  const days: CalendarDay[] = []

  for (let offset = 0; offset < startDayOfWeek; offset += 1) {
    days.push({
      date: new Date(year, month, offset - startDayOfWeek + 1),
      inMonth: false,
    })
  }
  for (let day = 1; day <= daysInMonth; day += 1) {
    days.push({ date: new Date(year, month, day), inMonth: true })
  }
  const remaining = 7 - (days.length % 7)
  for (let day = 1; remaining < 7 && day <= remaining; day += 1) {
    days.push({ date: new Date(year, month + 1, day), inMonth: false })
  }

  return days
}

/** 本地时区的 YYYY-MM-DD。 */
export function toDateString(date: Date): string {
  const month = String(date.getMonth() + 1).padStart(2, '0')
  const day = String(date.getDate()).padStart(2, '0')
  return `${date.getFullYear()}-${month}-${day}`
}

/** 周日至周六的星期简称，随界面语言变化。 */
export function getWeekdayLabels(language: string): string[] {
  const chinese = language.trim().toLowerCase().startsWith('zh')
  const weekday = new Intl.DateTimeFormat(chinese ? 'zh-CN' : 'en-US', {
    weekday: chinese ? 'narrow' : 'short',
  })
  return Array.from({ length: 7 }, (_, index) =>
    weekday.format(new Date(2024, 0, index + 7))
  )
}
