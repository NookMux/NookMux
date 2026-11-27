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
import { QueryClient, QueryObserver } from '@tanstack/react-query'
import assert from 'node:assert/strict'
import { describe, test } from 'node:test'
import { enabledModelsQueryOptions } from './use-enabled-models.ts'

type EnabledModelsResponse = { success: boolean; data: string[] }

/**
 * 模拟进入一次使用这份列表的页面：订阅后等到查询结束，随后离开页面。
 */
function visitPage(
  queryClient: QueryClient,
  options: typeof enabledModelsQueryOptions
): Promise<EnabledModelsResponse> {
  const observer = new QueryObserver(queryClient, options)
  return new Promise((resolve, reject) => {
    let unsubscribe = () => {}
    const timer = setTimeout(() => {
      unsubscribe()
      reject(new Error('查询在限定时间内没有结束'))
    }, 2000)
    const settle = (data: unknown) => {
      clearTimeout(timer)
      unsubscribe()
      resolve(data as EnabledModelsResponse)
    }
    unsubscribe = observer.subscribe((result) => {
      if (result.isSuccess && !result.isFetching) settle(result.data)
    })
    // 进入页面时没有开始新的拉取：按当时看到的结果返回
    const current = observer.getCurrentResult()
    if (current.isSuccess && !current.isFetching) settle(current.data)
  })
}

describe('useEnabledModels', () => {
  test('再次进入页面时重新拉取启用模型', async () => {
    const queryClient = new QueryClient()
    try {
      let servedModels = ['model-a']
      let requests = 0
      const options = {
        ...enabledModelsQueryOptions,
        queryFn: async (): Promise<EnabledModelsResponse> => {
          requests += 1
          return { success: true, data: [...servedModels] }
        },
      }

      const firstVisit = await visitPage(queryClient, options)
      assert.deepEqual(firstVisit.data, ['model-a'])
      assert.equal(requests, 1)

      // 在渠道页给渠道加了新模型
      servedModels = ['model-a', 'model-b']

      const secondVisit = await visitPage(queryClient, options)
      assert.equal(requests, 2)
      assert.deepEqual(secondVisit.data, ['model-a', 'model-b'])
    } finally {
      queryClient.clear()
    }
  })
})
