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
import { useMemo, useState } from 'react'
import { RefreshCw } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import {
  appTableFeatures,
  type ColumnDef,
  useTable,
} from '@/lib/tanstack-table'
import { Button } from '@/components/ui/button'
import { ConfirmDialog } from '@/components/confirm-dialog'
import { DataTablePage } from '@/components/data-table'
import { SectionPageLayout } from '@/components/layout'
import { useBillingHistory } from '@/features/wallet/hooks/use-billing-history'
import type { TopupRecord } from '@/features/wallet/types'
import { useOrderQueryColumns } from './components/order-query-columns'
import { OrderQueryFilterBar } from './components/order-query-filter-bar'

export function OrderQuery() {
  const { t } = useTranslation()
  const {
    records,
    total,
    page,
    pageSize,
    keyword,
    loading,
    completing,
    isAdmin,
    handlePageChange,
    handlePageSizeChange,
    handleSearch,
    handleCompleteOrder,
    refresh,
  } = useBillingHistory()
  const [confirmTradeNo, setConfirmTradeNo] = useState<string | null>(null)

  const columnActions = useMemo(
    () => ({
      onComplete: (tradeNo: string) => setConfirmTradeNo(tradeNo),
      completing,
    }),
    [completing]
  )

  const columns = useOrderQueryColumns(
    isAdmin,
    columnActions
  ) as ColumnDef<TopupRecord>[]

  const table = useTable({
    features: appTableFeatures,
    data: records,
    columns,
    state: {
      pagination: {
        pageIndex: page - 1,
        pageSize,
      },
    },
    onPaginationChange: (updater) => {
      if (typeof updater === 'function') {
        const next = updater({
          pageIndex: page - 1,
          pageSize,
        })
        if (next.pageSize !== pageSize) {
          handlePageSizeChange(next.pageSize)
        } else if (next.pageIndex !== page - 1) {
          handlePageChange(next.pageIndex + 1)
        }
      }
    },
    manualPagination: true,
    pageCount: Math.ceil(total / pageSize),
  })

  const confirmComplete = async () => {
    if (!confirmTradeNo) return
    const ok = await handleCompleteOrder(confirmTradeNo)
    if (ok) setConfirmTradeNo(null)
  }

  return (
    <>
      <SectionPageLayout>
        <SectionPageLayout.Title>
          {t('orderQuery.titles.query')}
        </SectionPageLayout.Title>
        <SectionPageLayout.Actions>
          <Button variant='outline' onClick={() => refresh()}>
            <RefreshCw className={loading ? 'size-4 animate-spin' : 'size-4'} />
            {t('channels.actions.refresh')}
          </Button>
        </SectionPageLayout.Actions>
        <SectionPageLayout.Content>
          <DataTablePage
            table={table}
            columns={columns}
            isLoading={loading}
            emptyTitle={t('orderQuery.fields.noOrdersFound')}
            skeletonKeyPrefix='order-query-skeleton'
            tableClassName='overflow-x-auto'
            tableHeaderClassName='bg-muted/30 sticky top-0 z-10'
            toolbar={
              <OrderQueryFilterBar
                keyword={keyword}
                onKeywordChange={handleSearch}
              />
            }
          />
        </SectionPageLayout.Content>
      </SectionPageLayout>

      <ConfirmDialog
        open={Boolean(confirmTradeNo)}
        onOpenChange={(open) => {
          if (!open && !completing) setConfirmTradeNo(null)
        }}
        title={t('orderQuery.fields.completeOrder')}
        desc={t('orderQuery.tips.sureYouWantToManuallyCompleteThisOrderThe')}
        confirmText={t('orderQuery.fields.completeOrder')}
        isLoading={completing}
        handleConfirm={confirmComplete}
      />
    </>
  )
}
