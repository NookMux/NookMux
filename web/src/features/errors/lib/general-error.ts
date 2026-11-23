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
/** 记录「本标签页已因该失败自动刷新过」的 sessionStorage 键前缀 */
const RELOAD_MARKER_PREFIX = 'tanstack_router_reload:'

/** 按错误对象上的 HTTP 响应取状态码；不是 HTTP 错误时返回 undefined。 */
function getHttpStatus(error: unknown): number | undefined {
  if (typeof error !== 'object' || error === null) return undefined
  const response = (error as Record<string, unknown>).response
  if (typeof response !== 'object' || response === null) return undefined
  const status = (response as Record<string, unknown>).status
  return typeof status === 'number' ? status : undefined
}

/**
 * 错误页大号标题展示的状态码。直接渲染的 /500 页面固定为 500；作为错误边界渲染时
 * 只有错误携带真实 HTTP 状态码才返回状态码，分块加载失败等非 HTTP 错误返回
 * undefined。
 */
export function resolveHeadingStatus(error: unknown): number | undefined {
  return error === undefined ? 500 : getHttpStatus(error)
}

/** 判断错误是否为路由分块（JS / CSS chunk）加载失败。 */
function isChunkLoadError(error: unknown): error is Error {
  if (!(error instanceof Error)) return false
  return (
    error.name === 'ChunkLoadError' ||
    /^Loading (CSS )?chunk .+ failed\./.test(error.message)
  )
}

/** 写入刷新标记所需的最小存储接口。 */
export type ReloadMarkerStorage = Pick<Storage, 'getItem' | 'setItem'>

/**
 * 为一次路由分块加载失败申请自动刷新。同一失败在标签页会话内只刷新一次，之后停留
 * 在错误页，避免持久失败时无限刷新；没有可用存储、无法留下标记时同样不刷新。
 */
export function claimChunkReload(
  storage: ReloadMarkerStorage | undefined,
  error: unknown
): boolean {
  if (!storage || !isChunkLoadError(error)) return false
  const key = RELOAD_MARKER_PREFIX + error.message
  try {
    if (storage.getItem(key) !== null) return false
    storage.setItem(key, '1')
  } catch {
    return false
  }
  return true
}
