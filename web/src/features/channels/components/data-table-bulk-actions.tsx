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
import { Power, PowerOff, Tag, Trash2 } from 'lucide-react'
import { useTranslation } from 'react-i18next'
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
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import {
  Tooltip,
  TooltipContent,
  TooltipTrigger,
} from '@/components/ui/tooltip'
import { DataTableBulkActions as BulkActionsToolbar } from '@/components/data-table'
import {
  handleBatchDelete,
  handleBatchDisable,
  handleBatchEnable,
  handleBatchSetTag,
} from '../lib'
import type { Channel } from '../types'

interface DataTableBulkActionsProps<TData extends RowData> {
  table: Table<TData>
}

export function DataTableBulkActions<TData extends RowData>({
  table,
}: DataTableBulkActionsProps<TData>) {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const [showTagDialog, setShowTagDialog] = useState(false)
  const [showDeleteConfirm, setShowDeleteConfirm] = useState(false)
  const [tagValue, setTagValue] = useState('')

  const selectedRows = table.getFilteredSelectedRowModel().rows
  const selectedIds = selectedRows.reduce<number[]>((ids, row) => {
    const id = (row.original as Channel).id

    if (typeof id === 'number') {
      ids.push(id)
    }

    return ids
  }, [])

  const handleClearSelection = () => {
    table.resetRowSelection()
  }

  const handleEnableAll = () => {
    handleBatchEnable(selectedIds, queryClient, handleClearSelection)
  }

  const handleDisableAll = () => {
    handleBatchDisable(selectedIds, queryClient, handleClearSelection)
  }

  const handleDeleteAll = () => {
    handleBatchDelete(selectedIds, queryClient, () => {
      setShowDeleteConfirm(false)
      handleClearSelection()
    })
  }

  const handleSetTag = () => {
    handleBatchSetTag(selectedIds, tagValue || null, queryClient, () => {
      setShowTagDialog(false)
      setTagValue('')
      handleClearSelection()
    })
  }

  return (
    <>
      <BulkActionsToolbar table={table} entityName='channel'>
        <Tooltip>
          <TooltipTrigger
            render={
              <Button
                variant='outline'
                size='icon'
                onClick={handleEnableAll}
                className='size-8'
                aria-label={t('channels.actions.enableSelectedChannels')}
                title={t('channels.actions.enableSelectedChannels')}
              />
            }
          >
            <Power />
            <span className='sr-only'>
              {t('channels.actions.enableSelectedChannels')}
            </span>
          </TooltipTrigger>
          <TooltipContent>
            <p>{t('channels.actions.enableSelectedChannels')}</p>
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
                aria-label={t('channels.actions.disableSelectedChannels')}
                title={t('channels.actions.disableSelectedChannels')}
              />
            }
          >
            <PowerOff />
            <span className='sr-only'>
              {t('channels.actions.disableSelectedChannels')}
            </span>
          </TooltipTrigger>
          <TooltipContent>
            <p>{t('channels.actions.disableSelectedChannels')}</p>
          </TooltipContent>
        </Tooltip>

        <Tooltip>
          <TooltipTrigger
            render={
              <Button
                variant='outline'
                size='icon'
                onClick={() => setShowTagDialog(true)}
                className='size-8'
                aria-label={t('channels.titles.setTagForSelectedChannels')}
                title={t('channels.titles.setTagForSelectedChannels')}
              />
            }
          >
            <Tag />
            <span className='sr-only'>
              {t('channels.titles.setTagForSelectedChannels')}
            </span>
          </TooltipTrigger>
          <TooltipContent>
            <p>{t('channels.titles.setTagForSelectedChannels')}</p>
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
                aria-label={t('channels.actions.deleteSelectedChannels')}
                title={t('channels.actions.deleteSelectedChannels')}
              />
            }
          >
            <Trash2 />
            <span className='sr-only'>
              {t('channels.actions.deleteSelectedChannels')}
            </span>
          </TooltipTrigger>
          <TooltipContent>
            <p>{t('channels.actions.deleteSelectedChannels')}</p>
          </TooltipContent>
        </Tooltip>
      </BulkActionsToolbar>

      {/* Set Tag Dialog */}
      <Dialog open={showTagDialog} onOpenChange={setShowTagDialog}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>{t('channels.fields.setTag')}</DialogTitle>
            <DialogDescription>
              {t('channels.fields.setATagFor')} {selectedIds.length}{' '}
              {t('channels.placeholders.selectedChannelSLeaveEmptyToRemoveTag')}
            </DialogDescription>
          </DialogHeader>

          <div className='grid gap-4 py-4'>
            <div className='grid gap-2'>
              <Label htmlFor='tag'>{t('channels.fields.tag')}</Label>
              <Input
                id='tag'
                placeholder={t('channels.placeholders.enterTagNameOptional')}
                value={tagValue}
                onChange={(e) => setTagValue(e.target.value)}
              />
            </div>
          </div>

          <DialogFooter>
            <Button
              variant='outline'
              onClick={() => {
                setShowTagDialog(false)
                setTagValue('')
              }}
            >
              {t('common.actions.cancel')}
            </Button>
            <Button onClick={handleSetTag}>
              {t('channels.fields.setTag')}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      {/* Delete Confirmation Dialog */}
      <Dialog open={showDeleteConfirm} onOpenChange={setShowDeleteConfirm}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>{t('channels.actions.deleteChannels')}</DialogTitle>
            <DialogDescription>
              {t('channels.fields.sureYouWantToDelete')} {selectedIds.length}{' '}
              {t('channels.errors.channelSThisActionCannotBeUndone')}
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
