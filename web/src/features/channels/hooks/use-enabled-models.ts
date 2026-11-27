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
import { useQuery } from '@tanstack/react-query'
import { getEnabledModels } from '../api'
import { channelsQueryKeys } from '../lib/channel-actions'

/**
 * 全站已启用的模型（各个已启用渠道的模型列表的并集）。
 *
 * 渠道页之外的页面也会读到这份列表，而改动它的入口分散在渠道的
 * 增删改、启停与批量操作里，所以不按新鲜缓存复用：每次进入使用它的页面
 * 都重新拉取，上一次的结果只在拉取期间过渡展示。
 */
export const enabledModelsQueryOptions = {
  queryKey: channelsQueryKeys.enabledModels(),
  queryFn: getEnabledModels,
  staleTime: 0,
}

export function useEnabledModels() {
  return useQuery(enabledModelsQueryOptions)
}
