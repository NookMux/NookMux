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
import { useState, useEffect, useCallback } from 'react'
import i18next from 'i18next'
import { toast } from 'sonner'
import { useIsAdmin } from '@/hooks/use-admin'
import {
  getUserBillingHistory,
  getAllBillingHistory,
  completeOrder,
  checkOrder,
  isApiSuccess,
} from '../api'
import type { TopupRecord } from '../types'

// ============================================================================
// Billing History Hook
// ============================================================================

interface UseBillingHistoryOptions {
  /** Initial page number */
  initialPage?: number
  /** Initial page size */
  initialPageSize?: number
}

export function useBillingHistory(options: UseBillingHistoryOptions = {}) {
  const { initialPage = 1, initialPageSize = 20 } = options
  const isAdmin = useIsAdmin()

  const [records, setRecords] = useState<TopupRecord[]>([])
  const [total, setTotal] = useState(0)
  const [page, setPage] = useState(initialPage)
  const [pageSize, setPageSize] = useState(initialPageSize)
  const [keyword, setKeyword] = useState('')
  const [loading, setLoading] = useState(false)
  const [completing, setCompleting] = useState(false)
  const [checking, setChecking] = useState(false)

  /**
   * Fetch billing history
   */
  const fetchBillingHistory = useCallback(async () => {
    setLoading(true)
    try {
      const response = isAdmin
        ? await getAllBillingHistory(page, pageSize, keyword)
        : await getUserBillingHistory(page, pageSize, keyword)

      if (isApiSuccess(response) && response.data) {
        setRecords(response.data.items || [])
        setTotal(response.data.total || 0)
      } else {
        toast.error(
          response.message ||
            i18next.t('wallet.errors.failedToLoadBillingHistory')
        )
        setRecords([])
        setTotal(0)
      }
    } catch (error) {
      // eslint-disable-next-line no-console
      console.error('Failed to fetch billing history:', error)
      toast.error(i18next.t('wallet.errors.failedToLoadBillingHistory'))
      setRecords([])
      setTotal(0)
    } finally {
      setLoading(false)
    }
  }, [isAdmin, page, pageSize, keyword])

  /**
   * Complete a pending order (admin only)
   */
  const handleCompleteOrder = useCallback(
    async (tradeNo: string) => {
      if (!isAdmin) {
        toast.error(i18next.t('wallet.fields.adminAccessRequired'))
        return false
      }

      setCompleting(true)
      try {
        const response = await completeOrder({ trade_no: tradeNo })
        if (isApiSuccess(response)) {
          toast.success(i18next.t('wallet.fields.orderCompletedSuccessfully'))
          // Refresh the list
          await fetchBillingHistory()
          return true
        } else {
          toast.error(
            response.message || i18next.t('wallet.errors.failedToCompleteOrder')
          )
          return false
        }
      } catch (error) {
        // eslint-disable-next-line no-console
        console.error('Failed to complete order:', error)
        toast.error(i18next.t('wallet.errors.failedToCompleteOrder'))
        return false
      } finally {
        setCompleting(false)
      }
    },
    [isAdmin, fetchBillingHistory]
  )

  /**
   * Check a pending order against the payment gateway. When the gateway
   * reports the order as paid, the backend completes it (recovered callback).
   */
  const handleCheckOrder = useCallback(
    async (tradeNo: string) => {
      setChecking(true)
      try {
        const response = await checkOrder({ trade_no: tradeNo })
        if (isApiSuccess(response)) {
          if (response.data?.status === 'success') {
            toast.success(i18next.t('wallet.tips.orderCheckPaid'))
          } else {
            toast.info(i18next.t('wallet.tips.orderCheckNotPaid'))
          }
          // 订单状态可能已被本次检查更新，刷新列表
          await fetchBillingHistory()
          return true
        }
        toast.error(
          response.message || i18next.t('wallet.errors.failedToCheckOrder')
        )
        return false
      } catch (error) {
        // eslint-disable-next-line no-console
        console.error('Failed to check order:', error)
        toast.error(i18next.t('wallet.errors.failedToCheckOrder'))
        return false
      } finally {
        setChecking(false)
      }
    },
    [fetchBillingHistory]
  )

  /**
   * Change page
   */
  const handlePageChange = useCallback((newPage: number) => {
    setPage(newPage)
  }, [])

  /**
   * Change page size
   */
  const handlePageSizeChange = useCallback((newPageSize: number) => {
    setPageSize(newPageSize)
    setPage(1) // Reset to first page when changing page size
  }, [])

  /**
   * Search by keyword
   */
  const handleSearch = useCallback((newKeyword: string) => {
    setKeyword(newKeyword)
    setPage(1) // Reset to first page when searching
  }, [])

  // Fetch data when dependencies change
  useEffect(() => {
    fetchBillingHistory()
  }, [fetchBillingHistory])

  return {
    records,
    total,
    page,
    pageSize,
    keyword,
    loading,
    completing,
    checking,
    isAdmin,
    handlePageChange,
    handlePageSizeChange,
    handleSearch,
    handleCompleteOrder,
    handleCheckOrder,
    refresh: fetchBillingHistory,
  }
}
