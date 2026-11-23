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
  claimChunkReload,
  resolveHeadingStatus,
  type ReloadMarkerStorage,
} from './general-error'

// Rspack 分块加载失败时的真实错误形态
const CHUNK_ERROR_MESSAGE =
  'Loading chunk fk failed.\n(error: http://127.0.0.1:3210/static/js/async/fk.afde09c853.js)'

function chunkLoadError(message: string = CHUNK_ERROR_MESSAGE): Error {
  return Object.assign(new Error(message), { name: 'ChunkLoadError' })
}

function createStorage() {
  const entries = new Map<string, string>()
  const storage: ReloadMarkerStorage = {
    getItem: (key) => entries.get(key) ?? null,
    setItem: (key, value) => {
      entries.set(key, value)
    },
  }
  return { storage, entries }
}

describe('resolveHeadingStatus', () => {
  test('直接渲染的 /500 页面没有错误对象，标题为 500', () => {
    assert.equal(resolveHeadingStatus(undefined), 500)
  })

  test('HTTP 错误展示真实状态码', () => {
    assert.equal(resolveHeadingStatus({ response: { status: 429 } }), 429)
    assert.equal(resolveHeadingStatus({ response: { status: 503 } }), 503)
  })

  test('非 HTTP 错误不展示状态码', () => {
    assert.equal(resolveHeadingStatus(chunkLoadError()), undefined)
    assert.equal(resolveHeadingStatus(new Error('Render failed')), undefined)
    assert.equal(
      resolveHeadingStatus({ response: { status: 'boom' } }),
      undefined
    )
    assert.equal(resolveHeadingStatus(null), undefined)
  })
})

describe('claimChunkReload', () => {
  test('同一个分块加载失败只自动刷新一次，之后停留在错误页', () => {
    const { storage } = createStorage()
    const error = chunkLoadError()

    assert.equal(claimChunkReload(storage, error), true)
    assert.equal(claimChunkReload(storage, error), false)
    assert.equal(claimChunkReload(storage, chunkLoadError()), false)
  })

  test('不同分块的失败各自只刷新一次', () => {
    const { storage } = createStorage()

    assert.equal(
      claimChunkReload(storage, chunkLoadError('Loading chunk a failed.')),
      true
    )
    assert.equal(
      claimChunkReload(storage, chunkLoadError('Loading chunk b failed.')),
      true
    )
    assert.equal(
      claimChunkReload(storage, chunkLoadError('Loading chunk a failed.')),
      false
    )
  })

  test('CSS 分块加载失败同样只刷新一次', () => {
    const { storage } = createStorage()
    const error = new Error('Loading CSS chunk 42 failed. (/static/css/42.css)')

    assert.equal(claimChunkReload(storage, error), true)
    assert.equal(claimChunkReload(storage, error), false)
  })

  test('非分块加载错误不刷新也不写标记', () => {
    const { storage, entries } = createStorage()

    assert.equal(
      claimChunkReload(storage, { response: { status: 429 } }),
      false
    )
    assert.equal(claimChunkReload(storage, new Error('Render failed')), false)
    assert.equal(
      claimChunkReload(storage, new Error('Loading chunk failed.')),
      false
    )
    assert.equal(claimChunkReload(storage, CHUNK_ERROR_MESSAGE), false)
    assert.equal(entries.size, 0)
  })

  test('没有可用存储时不刷新', () => {
    assert.equal(claimChunkReload(undefined, chunkLoadError()), false)
  })

  test('存储读取或写入抛错时不刷新', () => {
    const getItemFails: ReloadMarkerStorage = {
      getItem: () => {
        throw new Error('Storage blocked')
      },
      setItem: () => {},
    }
    const setItemFails: ReloadMarkerStorage = {
      getItem: () => null,
      setItem: () => {
        throw new Error('Storage blocked')
      },
    }

    assert.equal(claimChunkReload(getItemFails, chunkLoadError()), false)
    assert.equal(claimChunkReload(setItemFails, chunkLoadError()), false)
  })
})
