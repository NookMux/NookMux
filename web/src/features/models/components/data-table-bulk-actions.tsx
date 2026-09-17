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
import { useQueryClient } from '@tanstack/react-query'
import { Power, PowerOff, Trash2, Copy } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { copyToClipboard } from '@/lib/copy-to-clipboard'
import { type RowData, type Table } from '@/lib/tanstack-table'
import { Button } from '@/components/ui/button'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import {
  Tooltip,
  TooltipContent,
  TooltipTrigger,
} from '@/components/ui/tooltip'
import { DataTableBulkActions as BulkActionsToolbar } from '@/components/data-table'
import {
  handleBatchEnableModels,
  handleBatchDisableModels,
  handleBatchDeleteModels,
} from '../lib'
import type { Model } from '../types'

interface DataTableBulkActionsProps<TData extends RowData> {
  table: Table<TData>
}

export function DataTableBulkActions<TData extends RowData>({
  table,
}: DataTableBulkActionsProps<TData>) {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const [showDeleteConfirm, setShowDeleteConfirm] = useState(false)

  const selectedRows = table.getFilteredSelectedRowModel().rows
  const selectedIds = selectedRows.reduce<number[]>((ids, row) => {
    const id = (row.original as Model).id

    if (typeof id === 'number') {
      ids.push(id)
    }

    return ids
  }, [])

  const selectedModels = selectedRows.map((row) => row.original as Model)

  const handleClearSelection = () => {
    table.resetRowSelection()
  }

  const handleEnableAll = () => {
    handleBatchEnableModels(selectedIds, queryClient, handleClearSelection)
  }

  const handleDisableAll = () => {
    handleBatchDisableModels(selectedIds, queryClient, handleClearSelection)
  }

  const handleDeleteAll = () => {
    handleBatchDeleteModels(selectedIds, queryClient, () => {
      setShowDeleteConfirm(false)
      handleClearSelection()
    })
  }

  const handleCopyNames = async () => {
    const names = selectedModels.map((m) => m.model_name).join(',')
    const success = await copyToClipboard(names)
    if (success) {
      toast.success(t('models.status.modelNamesCopiedToClipboard'))
    } else {
      toast.error(t('models.errors.failedToCopyModelNames'))
    }
  }

  return (
    <>
      <BulkActionsToolbar table={table} entityName='model'>
        <Tooltip>
          <TooltipTrigger
            render={
              <Button
                variant='outline'
                size='icon'
                onClick={handleEnableAll}
                className='size-8'
                aria-label={t('models.actions.enableSelectedModels')}
                title={t('models.actions.enableSelectedModels')}
              />
            }
          >
            <Power />
            <span className='sr-only'>
              {t('models.actions.enableSelectedModels')}
            </span>
          </TooltipTrigger>
          <TooltipContent>
            <p>{t('models.actions.enableSelectedModels')}</p>
          </TooltipContent>
        </Tooltip>

        <Tooltip>
          <TooltipTrigger
            render={
              <Button
                variant='outline'
                size='icon'
                onClick={handleDisableAll}
                className='size-8'
                aria-label={t('models.actions.disableSelectedModels')}
                title={t('models.actions.disableSelectedModels')}
              />
            }
          >
            <PowerOff />
            <span className='sr-only'>
              {t('models.actions.disableSelectedModels')}
            </span>
          </TooltipTrigger>
          <TooltipContent>
            <p>{t('models.actions.disableSelectedModels')}</p>
          </TooltipContent>
        </Tooltip>

        <Tooltip>
          <TooltipTrigger
            render={
              <Button
                variant='outline'
                size='icon'
                onClick={handleCopyNames}
                className='size-8'
                aria-label={t('models.actions.copyModelNames')}
                title={t('models.actions.copyModelNames')}
              />
            }
          >
            <Copy />
            <span className='sr-only'>
              {t('models.actions.copyModelNames')}
            </span>
          </TooltipTrigger>
          <TooltipContent>
            <p>{t('models.actions.copyModelNames')}</p>
          </TooltipContent>
        </Tooltip>

        <Tooltip>
          <TooltipTrigger
            render={
              <Button
                variant='destructive'
                size='icon'
                onClick={() => setShowDeleteConfirm(true)}
                className='size-8'
                aria-label={t('models.actions.deleteSelectedModels')}
                title={t('models.actions.deleteSelectedModels')}
              />
            }
          >
            <Trash2 />
            <span className='sr-only'>
              {t('models.actions.deleteSelectedModels')}
            </span>
          </TooltipTrigger>
          <TooltipContent>
            <p>{t('models.actions.deleteSelectedModels')}</p>
          </TooltipContent>
        </Tooltip>
      </BulkActionsToolbar>

      {/* Delete Confirmation Dialog */}
      <Dialog open={showDeleteConfirm} onOpenChange={setShowDeleteConfirm}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>{t('models.actions.deleteModels')}</DialogTitle>
            <DialogDescription>
              {t('models.errors.sureYouWantToDeleteCountModelSThis', {
                count: selectedIds.length,
              })}
            </DialogDescription>
          </DialogHeader>

          <DialogFooter>
            <Button
              variant='outline'
              onClick={() => setShowDeleteConfirm(false)}
            >
              {t('common.actions.cancel')}
            </Button>
            <Button variant='destructive' onClick={handleDeleteAll}>
              {t('common.actions.delete')}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </>
  )
}
