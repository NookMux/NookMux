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
import { ArrowDown, ArrowUp, Pencil, Trash2 } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { type ColumnDef } from '@/lib/tanstack-table'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Switch } from '@/components/ui/switch'
import type { DynamicRatioRule } from '../types'

const WEEKDAYS = [
  { value: 0, label: 'Sun' },
  { value: 1, label: 'Mon' },
  { value: 2, label: 'Tue' },
  { value: 3, label: 'Wed' },
  { value: 4, label: 'Thu' },
  { value: 5, label: 'Fri' },
  { value: 6, label: 'Sat' },
]

function formatWeekdays(value: string, everyDayLabel: string): string {
  if (!value) return everyDayLabel
  const parsed = JSON.parse(value) as unknown
  if (!Array.isArray(parsed) || parsed.length === 0) return everyDayLabel
  return parsed
    .map((day) => {
      const weekday = WEEKDAYS.find((item) => item.value === Number(day))
      return weekday?.label ?? String(day)
    })
    .join(', ')
}

function getRatioVariant(ratio: number) {
  if (ratio > 3) return 'destructive'
  if (ratio > 1.5) return 'secondary'
  return 'outline'
}

function formatModels(value: string, allModelsLabel: string): string {
  if (!value) return allModelsLabel
  try {
    const parsed = JSON.parse(value) as unknown
    if (!Array.isArray(parsed) || parsed.length === 0) return allModelsLabel
    return parsed.join(', ')
  } catch {
    return value || allModelsLabel
  }
}

export interface DynamicRatioColumnActions {
  canEdit: boolean
  onToggleEnable: (rule: DynamicRatioRule, enable: boolean) => void
  onMoveUp: (index: number) => void
  onMoveDown: (index: number) => void
  onEdit: (rule: DynamicRatioRule) => void
  onDelete: (rule: DynamicRatioRule) => void
  rulesCount: number
}

export function useDynamicRatioColumns(
  actions: DynamicRatioColumnActions
): ColumnDef<DynamicRatioRule>[] {
  const { t } = useTranslation()

  return [
    {
      id: 'enable',
      header: t('channels.status.enabled'),
      cell: ({ row }) => (
        <Switch
          size='sm'
          checked={row.original.enable !== false}
          disabled={!actions.canEdit}
          onCheckedChange={(checked) =>
            actions.onToggleEnable(row.original, checked)
          }
        />
      ),
      enableSorting: false,
      size: 64,
    },
    {
      accessorKey: 'group',
      header: t('common.fields.group'),
      cell: ({ row }) => <Badge variant='outline'>{row.original.group}</Badge>,
    },
    {
      accessorKey: 'models',
      header: t('channels.titles.models'),
      cell: ({ row }) => (
        <span className='block max-w-48 truncate'>
          {formatModels(
            row.original.models,
            t('dynamicRatio.titles.allModels')
          )}
        </span>
      ),
    },
    {
      accessorKey: 'concurrency',
      header: t('dynamicRatio.fields.concurrency'),
      cell: ({ row }) =>
        row.original.concurrency ? (
          <Badge variant='secondary'>{row.original.concurrency}</Badge>
        ) : (
          <span className='text-muted-foreground'>
            {t('dynamicRatio.fields.any')}
          </span>
        ),
    },
    {
      accessorKey: 'weekdays',
      header: t('dynamicRatio.fields.weekdays'),
      cell: ({ row }) =>
        formatWeekdays(row.original.weekdays, t('dynamicRatio.fields.daily')),
    },
    {
      id: 'time_range',
      header: t('dynamicRatio.fields.timeRange'),
      cell: ({ row }) =>
        row.original.start_time && row.original.end_time ? (
          `${row.original.start_time} - ${row.original.end_time}`
        ) : (
          <span className='text-muted-foreground'>
            {t('dynamicRatio.fields.any')}
          </span>
        ),
    },
    {
      accessorKey: 'ratio',
      header: t('dynamicRatio.fields.ratio794f65'),
      cell: ({ row }) => (
        <Badge variant={getRatioVariant(row.original.ratio)}>
          {row.original.ratio}x
        </Badge>
      ),
    },
    {
      accessorKey: 'priority',
      header: t('channels.fields.priority'),
      size: 80,
    },
    {
      id: 'actions',
      header: () => (
        <span className='text-right'>{t('channels.fields.actions')}</span>
      ),
      cell: ({ row }) => {
        const index = row.index
        return (
          <div className='flex justify-end gap-1'>
            <Button
              size='icon-sm'
              variant='ghost'
              disabled={!actions.canEdit || index === 0}
              onClick={() => actions.onMoveUp(index)}
            >
              <ArrowUp />
              <span className='sr-only'>{t('dynamicRatio.fields.moveUp')}</span>
            </Button>
            <Button
              size='icon-sm'
              variant='ghost'
              disabled={!actions.canEdit || index === actions.rulesCount - 1}
              onClick={() => actions.onMoveDown(index)}
            >
              <ArrowDown />
              <span className='sr-only'>
                {t('dynamicRatio.fields.moveDown')}
              </span>
            </Button>
            <Button
              size='icon-sm'
              variant='ghost'
              disabled={!actions.canEdit}
              onClick={() => actions.onEdit(row.original)}
            >
              <Pencil />
              <span className='sr-only'>{t('channels.actions.edit')}</span>
            </Button>
            <Button
              size='icon-sm'
              variant='destructive'
              disabled={!actions.canEdit}
              onClick={() => actions.onDelete(row.original)}
            >
              <Trash2 />
              <span className='sr-only'>{t('common.actions.delete')}</span>
            </Button>
          </div>
        )
      },
      enableSorting: false,
      enableHiding: false,
      size: 160,
    },
  ]
}
