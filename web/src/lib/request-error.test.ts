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
import {
  MAX_QUERY_RETRIES,
  getHttpStatusCode,
  shouldRetryQuery,
} from './request-error'

function httpError(status: number) {
  return Object.assign(new Error(`Request failed with status code ${status}`), {
    response: { status },
  })
}

describe('getHttpStatusCode', () => {
  test('取出 HTTP 错误的状态码', () => {
    assert.equal(getHttpStatusCode(httpError(429)), 429)
    assert.equal(getHttpStatusCode(httpError(500)), 500)
  })

  test('非 HTTP 错误没有状态码', () => {
    assert.equal(getHttpStatusCode(new Error('Network Error')), undefined)
    assert.equal(getHttpStatusCode({ response: { status: 'boom' } }), undefined)
    assert.equal(getHttpStatusCode(null), undefined)
  })
})

describe('shouldRetryQuery', () => {
  test('凭证与限流类状态码立即失败，不重试', () => {
    for (const status of [401, 403, 429]) {
      assert.equal(shouldRetryQuery(0, httpError(status)), false)
    }
  })

  test('其余错误在重试次数内继续重试，次数用完后停止', () => {
    for (const failureCount of [0, 1, 2, 3]) {
      assert.equal(shouldRetryQuery(failureCount, httpError(500)), true)
      assert.equal(
        shouldRetryQuery(failureCount, new Error('Network Error')),
        true
      )
    }
    assert.equal(shouldRetryQuery(MAX_QUERY_RETRIES, httpError(500)), false)
  })
})
