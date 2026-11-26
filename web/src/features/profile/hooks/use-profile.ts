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
import { useCallback, useEffect, useState } from 'react'
import { getUserProfile } from '../api'
import type { UserProfile } from '../types'

/**
 * 当前登录用户的资料。首次加载失败时置 failed，由页面给出重试入口。
 */
export function useProfile() {
  const [profile, setProfile] = useState<UserProfile | null>(null)
  const [loading, setLoading] = useState(true)
  const [failed, setFailed] = useState(false)

  const fetchProfile = useCallback(async () => {
    try {
      setLoading(true)
      const response = await getUserProfile()
      if (response.success && response.data) {
        setProfile(response.data)
        setFailed(false)
        return response.data
      }
      setFailed(true)
      return null
    } catch (error) {
      // eslint-disable-next-line no-console
      console.error('Failed to fetch profile:', error)
      setFailed(true)
      return null
    } finally {
      setLoading(false)
    }
  }, [])

  // 静默刷新：不切换加载状态，供保存后同步最新资料。
  const refreshProfile = useCallback(async () => {
    try {
      const response = await getUserProfile()
      if (response.success && response.data) {
        setProfile(response.data)
        return response.data
      }
      return null
    } catch (error) {
      // eslint-disable-next-line no-console
      console.error('Failed to refresh profile:', error)
      return null
    }
  }, [])

  useEffect(() => {
    fetchProfile()
  }, [fetchProfile])

  return { profile, loading, failed, fetchProfile, refreshProfile }
}
