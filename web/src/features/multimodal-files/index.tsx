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
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { Copy, RefreshCw, Trash2 } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { useAuthStore } from '@/stores/auth-store'
import dayjs from '@/lib/dayjs'
import { ROLE } from '@/lib/roles'
import {
  appTableFeatures,
  type ColumnDef,
  useTable,
} from '@/lib/tanstack-table'
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from '@/components/ui/alert-dialog'
import { Button } from '@/components/ui/button'
import {
  Dialog,
  DialogContent,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { DataTablePage } from '@/components/data-table'
import { SectionPageLayout } from '@/components/layout'
import {
  batchDeleteStoredMedia,
  deleteStoredMedia,
  getStoredMedia,
  getStoredMediaDetail,
} from './api'
import { MultimodalFilesBulkActions } from './components/multimodal-files-bulk-actions'
import { useMultimodalFilesColumns } from './components/multimodal-files-columns'
import { MultimodalFilesFilterBar } from './components/multimodal-files-filter-bar'
import type { StoredMediaBatchItem, StoredMediaItem } from './types'

const DEFAULT_PAGE_SIZE = 20
const EMPTY_STORED_MEDIA_ITEMS: StoredMediaItem[] = []

type DeleteTarget =
  | { mode: 'single'; item: StoredMediaItem }
  | { mode: 'batch'; items: StoredMediaBatchItem[] }

function toInputDateTime(date: Date) {
  return dayjs(date).format('YYYY-MM-DDTHH:mm')
}

function toUnixSeconds(value: string) {
  const ms = Date.parse(value)
  if (!Number.isFinite(ms)) return 0
  return Math.floor(ms / 1000)
}

function getDefaultDateRange() {
  const now = new Date()
  const start = new Date(now)
  start.setHours(0, 0, 0, 0)
  const end = new Date(now.getTime() + 60 * 60 * 1000)
  return {
    start: toInputDateTime(start),
    end: toInputDateTime(end),
  }
}

function copyToClipboard(text: string) {
  return navigator.clipboard.writeText(text)
}

export function MultimodalFiles() {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const authUser = useAuthStore((state) => state.auth.user)
  const isAdmin = (authUser?.role ?? 0) >= ROLE.ADMIN
  const defaultRange = useMemo(() => getDefaultDateRange(), [])
  const [pagination, setPagination] = useState({
    pageIndex: 0,
    pageSize: DEFAULT_PAGE_SIZE,
  })
  const [startTime, setStartTime] = useState(defaultRange.start)
  const [endTime, setEndTime] = useState(defaultRange.end)
  const [rowSelection, setRowSelection] = useState({})
  const [detailOpen, setDetailOpen] = useState(false)
  const [detailItem, setDetailItem] = useState<StoredMediaItem | null>(null)
  const [deleteTarget, setDeleteTarget] = useState<DeleteTarget | null>(null)

  const startTimestamp = toUnixSeconds(startTime)
  const endTimestamp = toUnixSeconds(endTime)

  const queryKey = [
    'stored-media',
    isAdmin,
    pagination.pageIndex + 1,
    pagination.pageSize,
    startTimestamp,
    endTimestamp,
  ] as const

  const mediaQuery = useQuery({
    queryKey,
    queryFn: () =>
      getStoredMedia({
        page: pagination.pageIndex + 1,
        pageSize: pagination.pageSize,
        startTimestamp,
        endTimestamp,
        isAdmin,
      }),
    placeholderData: (previousData) => previousData,
  })

  const items = mediaQuery.data?.items ?? EMPTY_STORED_MEDIA_ITEMS
  const total = mediaQuery.data?.total ?? 0

  const refresh = async () => {
    await queryClient.invalidateQueries({ queryKey: ['stored-media'] })
  }

  const detailMutation = useMutation({
    mutationFn: (item: StoredMediaItem) =>
      getStoredMediaDetail(item.media_type, item.id),
    onSuccess: (item) => {
      setDetailItem(item)
      setDetailOpen(true)
    },
    onError: (error) => toast.error(error.message),
  })

  const deleteMutation = useMutation({
    mutationFn: async (target: DeleteTarget) => {
      if (target.mode === 'single') {
        return deleteStoredMedia(target.item.media_type, target.item.id)
      }
      return batchDeleteStoredMedia(target.items)
    },
    onSuccess: async (deleted) => {
      toast.success(
        t('multimodalFiles.status.deletedCountFileS', { count: deleted })
      )
      setDeleteTarget(null)
      setRowSelection({})
      await refresh()
    },
    onError: (error) => toast.error(error.message),
  })

  const copyUrl = async (url: string) => {
    if (!url) return
    try {
      await copyToClipboard(url)
      toast.success(t('common.status.copied'))
    } catch {
      toast.error(t('keyQuery.actions.copyFailed'))
    }
  }

  const resetFilters = () => {
    setStartTime(defaultRange.start)
    setEndTime(defaultRange.end)
    setPagination({ pageIndex: 0, pageSize: DEFAULT_PAGE_SIZE })
    setRowSelection({})
  }

  const columnActions = useMemo(
    () => ({
      onView: (item: StoredMediaItem) => detailMutation.mutate(item),
      onCopy: (url: string) => void copyUrl(url),
      onDelete: (item: StoredMediaItem) =>
        setDeleteTarget({ mode: 'single', item }),
    }),
    // detailMutation/copyUrl stable enough; refresh via setDeleteTarget
    // eslint-disable-next-line react-hooks/exhaustive-deps
    []
  )

  const columns = useMultimodalFilesColumns(
    columnActions
  ) as ColumnDef<StoredMediaItem>[]

  const table = useTable({
    features: appTableFeatures,
    data: items,
    columns,
    state: {
      rowSelection,
      pagination,
    },
    enableRowSelection: true,
    onRowSelectionChange: setRowSelection,
    onPaginationChange: setPagination,
    manualPagination: true,
    pageCount: Math.ceil(total / pagination.pageSize),
  })

  const handleDeleteSelected = () => {
    const selectedRows = table.getFilteredSelectedRowModel().rows
    if (selectedRows.length === 0) return
    const batchItems: StoredMediaBatchItem[] = selectedRows.map((row) => ({
      id: row.original.id,
      media_type: row.original.media_type,
    }))
    setDeleteTarget({ mode: 'batch', items: batchItems })
  }

  return (
    <>
      <SectionPageLayout>
        <SectionPageLayout.Title>
          {t('multimodalFiles.fields.files')}
        </SectionPageLayout.Title>
        <SectionPageLayout.Actions>
          <Button
            variant='outline'
            disabled={table.getFilteredSelectedRowModel().rows.length === 0}
            onClick={handleDeleteSelected}
          >
            <Trash2 />
            {t('multimodalFiles.actions.deleteSelected')}
          </Button>
          <Button variant='outline' onClick={() => void refresh()}>
            <RefreshCw />
            {t('channels.actions.refresh')}
          </Button>
        </SectionPageLayout.Actions>
        <SectionPageLayout.Content>
          {mediaQuery.error instanceof Error && (
            <div className='border-destructive/40 text-destructive mb-2.5 rounded-lg border px-3 py-2 text-sm'>
              {mediaQuery.error.message}
            </div>
          )}

          <DataTablePage
            table={table}
            columns={columns}
            isLoading={mediaQuery.isLoading}
            isFetching={mediaQuery.isFetching}
            emptyTitle={t('multimodalFiles.fields.noMultimodalFiles')}
            skeletonKeyPrefix='multimodal-files-skeleton'
            tableClassName='overflow-x-auto'
            tableHeaderClassName='bg-muted/30 sticky top-0 z-10'
            toolbar={
              <MultimodalFilesFilterBar
                startTime={startTime}
                endTime={endTime}
                onStartTimeChange={(value) => {
                  setStartTime(value)
                  setPagination({ ...pagination, pageIndex: 0 })
                  setRowSelection({})
                }}
                onEndTimeChange={(value) => {
                  setEndTime(value)
                  setPagination({ ...pagination, pageIndex: 0 })
                  setRowSelection({})
                }}
                onQuery={() => void refresh()}
                onReset={resetFilters}
                onRefresh={() => void refresh()}
              />
            }
            bulkActions={
              <MultimodalFilesBulkActions
                table={table}
                onDeleteSelected={handleDeleteSelected}
              />
            }
          />
        </SectionPageLayout.Content>
      </SectionPageLayout>

      <Dialog open={detailOpen} onOpenChange={setDetailOpen}>
        <DialogContent className='sm:max-w-2xl'>
          <DialogHeader>
            <DialogTitle>{t('multimodalFiles.fields.files')}</DialogTitle>
          </DialogHeader>
          {detailItem && (
            <div className='space-y-3'>
              <div className='grid gap-2 text-sm sm:grid-cols-2'>
                <div>
                  <span className='text-muted-foreground'>
                    {t('channels.fields.id')}:{' '}
                  </span>
                  <span className='font-mono text-xs'>{detailItem.id}</span>
                </div>
                <div>
                  <span className='text-muted-foreground'>
                    {t('multimodalFiles.status.createdAt')}:{' '}
                  </span>
                  {dayjs
                    .unix(detailItem.created_at)
                    .format('YYYY-MM-DD HH:mm:ss')}
                </div>
                <div>
                  <span className='text-muted-foreground'>
                    {t('channels.fields.type')}:{' '}
                  </span>
                  {detailItem.media_type}
                </div>
                <div>
                  <span className='text-muted-foreground'>
                    {t('channels.fields.size')}:{' '}
                  </span>
                  {detailItem.size_bytes} B
                </div>
              </div>
              <div className='grid gap-1.5'>
                <Label htmlFor='stored-media-url'>
                  {t('multimodalFiles.fields.convertedUrl')}
                </Label>
                <Input id='stored-media-url' value={detailItem.url} readOnly />
              </div>
              {detailItem.url && detailItem.media_type === 'image' && (
                <div className='flex justify-center'>
                  <img
                    src={detailItem.url}
                    alt={detailItem.id}
                    className='max-h-80 max-w-full rounded-lg border object-contain'
                  />
                </div>
              )}
              {detailItem.url && detailItem.media_type === 'video' && (
                <div className='flex justify-center'>
                  <video
                    src={detailItem.url}
                    controls
                    className='max-h-80 max-w-full rounded-lg border'
                  />
                </div>
              )}
            </div>
          )}
          <DialogFooter>
            <Button variant='outline' onClick={() => setDetailOpen(false)}>
              {t('common.actions.close')}
            </Button>
            <Button
              disabled={!detailItem?.url}
              onClick={() => detailItem && void copyUrl(detailItem.url)}
            >
              <Copy />
              {t('dashboard.actions.copyUrl')}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      <AlertDialog
        open={deleteTarget != null}
        onOpenChange={(open) => {
          if (!open) setDeleteTarget(null)
        }}
      >
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>
              {t('multimodalFiles.actions.deleteFile')}
            </AlertDialogTitle>
            <AlertDialogDescription>
              {deleteTarget?.mode === 'batch'
                ? t('multimodalFiles.actions.deleteCountSelectedFileS', {
                    count: deleteTarget.items.length,
                  })
                : t('multimodalFiles.actions.deleteThisMultimodalFile')}
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel>{t('common.actions.cancel')}</AlertDialogCancel>
            <AlertDialogAction
              variant='destructive'
              disabled={deleteMutation.isPending}
              onClick={() => {
                if (deleteTarget) deleteMutation.mutate(deleteTarget)
              }}
            >
              {t('common.actions.delete')}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </>
  )
}
