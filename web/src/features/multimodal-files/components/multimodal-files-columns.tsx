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
import { Copy, Eye, Trash2 } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { formatTimestampToDate } from '@/lib/format'
import { type ColumnDef } from '@/lib/tanstack-table'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Checkbox } from '@/components/ui/checkbox'
import type { StoredMediaItem } from '../types'

function formatSize(size: number) {
  if (!Number.isFinite(size) || size <= 0) return '-'
  if (size < 1024) return `${size} B`
  if (size < 1024 * 1024) return `${(size / 1024).toFixed(1)} KB`
  return `${(size / 1024 / 1024).toFixed(2)} MB`
}

export interface MultimodalFilesColumnActions {
  onView: (item: StoredMediaItem) => void
  onCopy: (url: string) => void
  onDelete: (item: StoredMediaItem) => void
}

export function useMultimodalFilesColumns(
  actions: MultimodalFilesColumnActions
): ColumnDef<StoredMediaItem>[] {
  const { t } = useTranslation()

  return [
    {
      id: 'select',
      header: ({ table }) => (
        <Checkbox
          checked={table.getIsAllPageRowsSelected()}
          indeterminate={
            table.getIsSomePageRowsSelected() &&
            !table.getIsAllPageRowsSelected()
          }
          onCheckedChange={(value) => table.toggleAllPageRowsSelected(!!value)}
          aria-label={t('multimodalFiles.placeholders.selectVisibleFiles')}
          className='translate-y-[2px]'
        />
      ),
      cell: ({ row }) => (
        <Checkbox
          checked={row.getIsSelected()}
          onCheckedChange={(value) => row.toggleSelected(!!value)}
          aria-label={t('multimodalFiles.placeholders.selectFile')}
          className='translate-y-[2px]'
        />
      ),
      enableSorting: false,
      enableHiding: false,
      size: 40,
    },
    {
      accessorKey: 'media_type',
      header: t('channels.fields.type'),
      cell: ({ row }) => (
        <Badge variant='outline'>{t(row.original.media_type)}</Badge>
      ),
      size: 80,
    },
    {
      accessorKey: 'id',
      header: t('channels.fields.id'),
      cell: ({ row }) => (
        <span className='max-w-48 truncate font-mono text-xs'>
          {row.original.id}
        </span>
      ),
    },
    {
      accessorKey: 'created_at',
      header: t('multimodalFiles.status.createdAt'),
      cell: ({ row }) => formatTimestampToDate(row.original.created_at),
    },
    {
      accessorKey: 'mime_type',
      header: t('multimodalFiles.fields.mime'),
      cell: ({ row }) => row.original.mime_type || '-',
    },
    {
      accessorKey: 'size_bytes',
      header: t('channels.fields.size'),
      cell: ({ row }) => formatSize(row.original.size_bytes),
    },
    {
      accessorKey: 'url',
      header: t('multimodalFiles.fields.convertedUrl'),
      cell: ({ row }) => (
        <span className='block max-w-72 truncate'>
          {row.original.url || '-'}
        </span>
      ),
    },
    {
      id: 'actions',
      header: () => (
        <span className='text-right'>{t('channels.fields.actions')}</span>
      ),
      cell: ({ row }) => {
        const item = row.original
        return (
          <div className='flex justify-end gap-1'>
            <Button
              size='icon-sm'
              variant='ghost'
              onClick={() => actions.onView(item)}
            >
              <Eye />
              <span className='sr-only'>{t('common.actions.view')}</span>
            </Button>
            <Button
              size='icon-sm'
              variant='ghost'
              disabled={!item.url}
              onClick={() => actions.onCopy(item.url)}
            >
              <Copy />
              <span className='sr-only'>{t('channels.actions.copy')}</span>
            </Button>
            <Button
              size='icon-sm'
              variant='destructive'
              onClick={() => actions.onDelete(item)}
            >
              <Trash2 />
              <span className='sr-only'>{t('common.actions.delete')}</span>
            </Button>
          </div>
        )
      },
      enableSorting: false,
      enableHiding: false,
      size: 144,
    },
  ]
}
