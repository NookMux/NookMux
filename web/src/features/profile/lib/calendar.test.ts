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
import assert from 'node:assert/strict'
import { describe, test } from 'node:test'
import { buildCalendarDays, getWeekdayLabels, toDateString } from './calendar'

describe('buildCalendarDays', () => {
  test('starts on Sunday and pads to whole weeks', () => {
    const november = buildCalendarDays(2026, 10)

    assert.equal(november.length % 7, 0)
    assert.equal(november[0].date.getDate(), 1)
    assert.equal(november.filter((day) => day.inMonth).length, 30)
  })

  test('keeps a month that already fills whole weeks unpadded', () => {
    const february = buildCalendarDays(2026, 1)

    assert.equal(february.length, 28)
    assert.equal(
      february.every((day) => day.inMonth),
      true
    )
  })

  test('puts the tail of the previous month in the leading cells', () => {
    const august = buildCalendarDays(2026, 7)

    assert.equal(august.length, 42)
    assert.equal(august[0].date.getMonth(), 6)
    assert.equal(august[0].date.getDate(), 26)
    assert.equal(august[0].inMonth, false)
    assert.equal(august[6].date.getDate(), 1)
    assert.equal(august[6].inMonth, true)
  })
})

describe('toDateString', () => {
  test('pads month and day', () => {
    assert.equal(toDateString(new Date(2026, 10, 5)), '2026-11-05')
  })
})

describe('getWeekdayLabels', () => {
  test('lists Sunday first in both interface languages', () => {
    assert.deepEqual(getWeekdayLabels('zh-CN'), [
      '日',
      '一',
      '二',
      '三',
      '四',
      '五',
      '六',
    ])
    assert.deepEqual(getWeekdayLabels('en-US'), [
      'Sun',
      'Mon',
      'Tue',
      'Wed',
      'Thu',
      'Fri',
      'Sat',
    ])
  })
})
