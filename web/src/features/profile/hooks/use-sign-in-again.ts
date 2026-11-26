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
import { useCallback } from 'react'
import { useQueryClient } from '@tanstack/react-query'
import { useNavigate, useRouterState } from '@tanstack/react-router'
import { useAuthStore } from '@/stores/auth-store'
import { clearSidebarModulesCaches } from '@/lib/nav-modules'

/**
 * 当前用户改动了登录凭证（密码、两步验证、恢复码）后，服务端会让它的全部会话失效。
 * 此后不再发需要登录的请求：清掉本地登录状态回到登录页，登录后回到当前页面；
 * 查询缓存等离开登录后的页面再清，还挂载着的查询才不会重新发起请求。
 */
export function useSignInAgain() {
  const navigate = useNavigate()
  const queryClient = useQueryClient()
  const href = useRouterState({ select: (state) => state.location.href })

  return useCallback(() => {
    useAuthStore.getState().auth.reset()
    window.localStorage.removeItem('uid')
    clearSidebarModulesCaches()
    void navigate({ to: '/sign-in', search: { redirect: href } }).then(() => {
      queryClient.clear()
    })
  }, [href, navigate, queryClient])
}
