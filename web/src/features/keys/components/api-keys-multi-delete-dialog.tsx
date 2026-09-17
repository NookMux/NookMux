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
import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { type RowData, type Table } from '@/lib/tanstack-table'
import { ConfirmDialog } from '@/components/confirm-dialog'
import { batchDeleteApiKeys } from '../api'
import { ERROR_MESSAGES } from '../constants'
import { type ApiKey } from '../types'
import { useApiKeys } from './api-keys-provider'

type ApiKeysMultiDeleteDialogProps<TData extends RowData> = {
  open: boolean
  onOpenChange: (open: boolean) => void
  table: Table<TData>
}

export function ApiKeysMultiDeleteDialog<TData extends RowData>({
  open,
  onOpenChange,
  table,
}: ApiKeysMultiDeleteDialogProps<TData>) {
  const { t } = useTranslation()
  const { triggerRefresh } = useApiKeys()
  const [isDeleting, setIsDeleting] = useState(false)
  const selectedRows = table.getFilteredSelectedRowModel().rows

  const handleConfirm = async () => {
    setIsDeleting(true)
    try {
      const ids = selectedRows.map((row) => (row.original as ApiKey).id)
      const result = await batchDeleteApiKeys(ids)

      if (result.success) {
        const count = result.data || ids.length
        toast.success(
          t('keys.status.successfullyDeletedCountApiKeyS', { count })
        )
        table.resetRowSelection()
        triggerRefresh()
        onOpenChange(false)
      } else {
        toast.error(result.message || t(ERROR_MESSAGES.BATCH_DELETE_FAILED))
      }
    } catch (_error) {
      toast.error(t(ERROR_MESSAGES.UNEXPECTED))
    } finally {
      setIsDeleting(false)
    }
  }

  return (
    <ConfirmDialog
      destructive
      open={open}
      onOpenChange={onOpenChange}
      handleConfirm={handleConfirm}
      isLoading={isDeleting}
      className='max-w-md'
      title={t('keys.actions.deleteCountApiKeyS', {
        count: selectedRows.length,
      })}
      desc={
        <>
          {t('keys.tips.aboutToDeleteCountApiKeyS', {
            count: selectedRows.length,
          })}{' '}
          <br />
          {t('keys.errors.actionCannotBeUndone951f49')}
        </>
      }
      confirmText={t('common.actions.delete')}
    />
  )
}
