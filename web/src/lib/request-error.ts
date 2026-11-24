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

/** 查询失败后最多重试的次数。 */
export const MAX_QUERY_RETRIES = 4

/** 重试也不会改变结果的状态码：401、403 是凭证问题，429 要等限流窗口过去。 */
const NON_RETRYABLE_STATUSES = new Set([401, 403, 429])

/** 取错误对象上的 HTTP 响应状态码；不是 HTTP 错误时返回 undefined。 */
export function getHttpStatusCode(error: unknown): number | undefined {
  if (typeof error !== 'object' || error === null) return undefined
  const response = (error as Record<string, unknown>).response
  if (typeof response !== 'object' || response === null) return undefined
  const status = (response as Record<string, unknown>).status
  return typeof status === 'number' ? status : undefined
}

/** 查询失败后是否重试：凭证与限流类状态码直接失败，其余错误最多重试 MAX_QUERY_RETRIES 次。 */
export function shouldRetryQuery(
  failureCount: number,
  error: unknown
): boolean {
  if (failureCount >= MAX_QUERY_RETRIES) return false
  const status = getHttpStatusCode(error)
  return status === undefined || !NON_RETRYABLE_STATUSES.has(status)
}
