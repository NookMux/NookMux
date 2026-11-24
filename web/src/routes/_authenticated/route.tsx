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
import { createFileRoute, redirect } from '@tanstack/react-router'
import { useAuthStore } from '@/stores/auth-store'
import { getSelf } from '@/lib/api'
import { isPhoneUA } from '@/lib/is-phone'
import { getHttpStatusCode } from '@/lib/request-error'
import { AuthenticatedLayout } from '@/components/layout'

// 内存中的验证标记，避免同一会话中重复验证
let sessionVerified = false

export const Route = createFileRoute('/_authenticated')({
  beforeLoad: async ({ location }) => {
    // 手机 UA 不渲染控制台：在任何组件渲染前重定向到提示页，首帧即终态；
    // 手机浏览器的「桌面模式」会替换为桌面 UA，因此天然放行
    if (isPhoneUA()) {
      throw redirect({ to: '/mobile-unsupported', replace: true })
    }

    const { auth } = useAuthStore.getState()

    // 如果本地没有用户信息，直接跳转登录页
    if (!auth.user) {
      throw redirect({
        to: '/sign-in',
        search: { redirect: location.href },
      })
    }

    // 本地有用户信息，但需要验证 session 是否有效（每个会话只验证一次）
    if (!sessionVerified) {
      const res = await getSelf().catch((error: unknown) => {
        // 只有服务端明确判定未登录才清除登录态；限流、服务端故障等失败保留登录态，
        // 交由错误边界按状态码展示，刷新后重新验证
        if (getHttpStatusCode(error) === 401) {
          auth.reset()
          throw redirect({
            to: '/sign-in',
            search: { redirect: location.href },
          })
        }
        throw error
      })
      if (res?.success && res.data) {
        // 验证成功，更新用户信息（可能有变化）
        auth.setUser(res.data)
        sessionVerified = true
      } else {
        // 服务端正常响应但未通过校验（如账号被封禁），按未登录处理
        auth.reset()
        throw redirect({
          to: '/sign-in',
          search: { redirect: location.href },
        })
      }
    }
  },
  component: AuthenticatedLayout,
})
