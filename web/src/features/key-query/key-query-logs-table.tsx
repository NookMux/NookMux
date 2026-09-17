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
import { keepPreviousData, useQuery } from '@tanstack/react-query'
import { ChevronRight, Download, Loader2 } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import {
  formatLogQuota,
  formatTimestampToDate,
  formatUseTime,
} from '@/lib/format'
import {
  appTableFeatures,
  type ColumnDef,
  flexRender,
  type Row,
  useTable,
} from '@/lib/tanstack-table'
import { cn } from '@/lib/utils'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import {
  Select,
  SelectContent,
  SelectGroup,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'
import { StatusBadge } from '@/components/status-badge'
import {
  DetailsDialog,
  type UsageLogDetailsVisibility,
} from '@/features/usage-logs/components/dialogs/details-dialog'
import { ModelBadge } from '@/features/usage-logs/components/model-badge'
import {
  getFirstResponseTimeColor,
  getTimeColor,
  parseLogOther,
} from '@/features/usage-logs/lib/format'
import { fetchKeyLogs } from './api'
import type { KeyQueryLog } from './types'
import { useKeyQueryFieldVisibility } from './use-field-visibility'

// 日志类型常量，与 usage-logs/constants 对齐
const LOG_TYPE_CONSUME = 2
const LOG_TYPE_ERROR = 5
const LOG_TYPE_REFUND = 6

function timingTextColorClass(
  variant: 'success' | 'warning' | 'danger'
): string {
  if (variant === 'success') return 'text-emerald-600'
  if (variant === 'warning') return 'text-amber-600'
  return 'text-rose-600'
}

function getLogTypeConfig(type: number): {
  label: string
  variant: 'green' | 'red' | 'blue' | 'neutral'
} {
  if (type === LOG_TYPE_ERROR) {
    return { label: 'common.errors.error', variant: 'red' }
  }
  if (type === LOG_TYPE_REFUND) {
    return { label: 'common.fields.refund', variant: 'blue' }
  }
  if (type === LOG_TYPE_CONSUME) {
    return { label: 'common.fields.consume', variant: 'green' }
  }
  return { label: 'channels.fields.unknown', variant: 'neutral' }
}

function getDefaultTimeRange(): { start: Date; end: Date } {
  const now = new Date()
  const start = new Date(now.getTime() - 24 * 60 * 60 * 1000)
  return { start, end: now }
}

interface KeyQueryLogsTableProps {
  rawKey: string
}

export function KeyQueryLogsTable({ rawKey }: KeyQueryLogsTableProps) {
  const { t } = useTranslation()
  const defaultRange = useMemo(() => getDefaultTimeRange(), [])
  const [startTime, setStartTime] = useState<Date>(defaultRange.start)
  const [endTime, setEndTime] = useState<Date>(defaultRange.end)
  const [model, setModel] = useState('')
  const [pageIndex, setPageIndex] = useState(0)
  const [pageSize, setPageSize] = useState(20)
  const [expandedRowId, setExpandedRowId] = useState<number | null>(null)

  const { visibility } = useKeyQueryFieldVisibility(rawKey)

  const queryParams = useMemo(
    () => ({
      rawKey,
      page: pageIndex + 1,
      pageSize,
      startTime,
      endTime,
      model: model.trim() || undefined,
    }),
    [rawKey, pageIndex, pageSize, startTime, endTime, model]
  )

  const { data, isLoading, isFetching } = useQuery({
    queryKey: ['key-query-logs', queryParams],
    queryFn: () =>
      fetchKeyLogs(queryParams.rawKey, {
        page: queryParams.page,
        pageSize: queryParams.pageSize,
        startTime: queryParams.startTime,
        endTime: queryParams.endTime,
        model: queryParams.model,
      }),
    enabled: !!rawKey,
    placeholderData: keepPreviousData,
  })

  const logs = data?.items ?? []
  const total = data?.total ?? 0

  const exportLogs = () => {
    if (logs.length === 0) return
    const headers = [
      'Time',
      'Type',
      'Model',
      'Duration',
      'FRT',
      'Stream',
      'PromptTokens',
      'CompletionTokens',
      'Cost',
    ]
    const body = [
      headers.join(','),
      ...logs.map((log) => {
        const other = parseLogOther(log.other)
        const row = [
          formatTimestampToDate(log.created_at),
          t(getLogTypeConfig(log.type).label),
          log.model_name || '-',
          log.use_time ? formatUseTime(log.use_time / 1000) : '-',
          other?.frt ? formatUseTime(other.frt / 1000) : '-',
          log.is_stream ? 'yes' : 'no',
          String(log.prompt_tokens || 0),
          String(log.completion_tokens || 0),
          formatLogQuota(log.quota),
        ]
        return row.map((cell) => escapeCsvValue(cell)).join(',')
      }),
    ].join('\n')
    const blob = new Blob([`\uFEFF${body}`], {
      type: 'text/csv;charset=utf-8;',
    })
    const url = URL.createObjectURL(blob)
    const link = document.createElement('a')
    link.href = url
    link.download = `key-usage-${Date.now()}.csv`
    document.body.appendChild(link)
    link.click()
    document.body.removeChild(link)
    URL.revokeObjectURL(url)
  }

  const columns = useMemo<ColumnDef<KeyQueryLog>[]>(
    () => [
      {
        id: 'expand',
        header: () => null,
        cell: ({ row }) => {
          const isOpen = expandedRowId === row.original.id
          return (
            <button
              type='button'
              className='text-muted-foreground hover:text-foreground flex size-6 items-center justify-center rounded'
              onClick={(event) => {
                event.stopPropagation()
                setExpandedRowId(isOpen ? null : row.original.id)
              }}
              aria-label={
                isOpen ? t('common.fields.collapse') : t('common.fields.expand')
              }
            >
              <ChevronRight
                className={cn(
                  'size-4 transition-transform duration-200',
                  isOpen && 'rotate-90'
                )}
              />
            </button>
          )
        },
        size: 40,
        enableHiding: false,
      },
      {
        accessorKey: 'created_at',
        header: () => t('auditLogs.fields.time'),
        cell: ({ row }) => {
          const log = row.original
          const typeConfig = getLogTypeConfig(log.type)
          return (
            <div className='flex flex-col gap-0.5'>
              <span className='font-mono text-xs tabular-nums'>
                {formatTimestampToDate(log.created_at)}
              </span>
              <StatusBadge
                label={t(typeConfig.label)}
                variant={typeConfig.variant}
                size='sm'
                copyable={false}
              />
            </div>
          )
        },
        enableHiding: false,
      },
      {
        accessorKey: 'model_name',
        header: () => t('common.fields.model'),
        cell: ({ row }) => {
          const log = row.original
          if (!log.model_name) {
            return <span className='text-muted-foreground'>-</span>
          }
          const other = parseLogOther(log.other)
          const actualModel =
            other?.is_model_mapped && other.upstream_model_name
              ? other.upstream_model_name
              : undefined
          return (
            <ModelBadge
              modelName={log.model_name}
              modelIcon={log.model_icon}
              actualModel={actualModel}
            />
          )
        },
      },
      {
        id: 'timing',
        header: () => t('keyQuery.fields.timing'),
        cell: ({ row }) => {
          const log = row.original
          const other = parseLogOther(log.other)
          const isApiCall =
            log.type === LOG_TYPE_CONSUME ||
            log.type === LOG_TYPE_ERROR ||
            log.type === LOG_TYPE_REFUND
          if (!isApiCall)
            return <span className='text-muted-foreground'>-</span>

          return (
            <div className='flex flex-col gap-1 text-xs'>
              {log.use_time > 0 && (
                <span
                  className={cn(
                    'font-medium',
                    timingTextColorClass(getTimeColor(log.use_time / 1000))
                  )}
                >
                  {formatUseTime(log.use_time / 1000)}
                </span>
              )}
              {log.is_stream && other?.frt != null && other.frt > 0 && (
                <span
                  className={timingTextColorClass(
                    getFirstResponseTimeColor(other.frt / 1000)
                  )}
                >
                  FRT {formatUseTime(other.frt / 1000)}
                </span>
              )}
              <StatusBadge
                label={
                  log.is_stream
                    ? t('keyQuery.fields.stream')
                    : t('keyQuery.fields.nonStream')
                }
                variant={log.is_stream ? 'blue' : 'neutral'}
                size='sm'
                copyable={false}
              />
            </div>
          )
        },
      },
      {
        id: 'tokens',
        header: () => t('usageLogs.fields.tokens'),
        cell: ({ row }) => {
          const log = row.original
          const isApiCall =
            log.type === LOG_TYPE_CONSUME ||
            log.type === LOG_TYPE_ERROR ||
            log.type === LOG_TYPE_REFUND
          if (!isApiCall) return null

          const promptTokens = log.prompt_tokens || 0
          const completionTokens = log.completion_tokens || 0
          if (promptTokens === 0 && completionTokens === 0) {
            return <span className='text-muted-foreground text-xs'>-</span>
          }

          const other = parseLogOther(log.other)
          const cacheReadTokens = other?.cache_tokens || 0
          const cacheWrite5m = other?.cache_creation_tokens_5m || 0
          const cacheWrite1h = other?.cache_creation_tokens_1h || 0
          const hasSplitCache = cacheWrite5m > 0 || cacheWrite1h > 0
          const cacheWriteTokens = hasSplitCache
            ? cacheWrite5m + cacheWrite1h
            : other?.cache_creation_tokens || 0
          const ordinaryInputTokens = Math.max(
            promptTokens - cacheReadTokens - cacheWriteTokens,
            0
          )

          return (
            <div className='flex flex-col gap-0.5'>
              <span className='font-mono text-xs font-medium tabular-nums'>
                {ordinaryInputTokens.toLocaleString()} /{' '}
                {completionTokens.toLocaleString()}
              </span>
              {(cacheReadTokens > 0 || cacheWriteTokens > 0) && (
                <div className='flex items-center gap-1 text-[11px]'>
                  {cacheReadTokens > 0 && (
                    <span className='text-muted-foreground/60'>
                      {t('pricing.fields.cache')}↓{' '}
                      {cacheReadTokens.toLocaleString()}
                    </span>
                  )}
                  {cacheWriteTokens > 0 && (
                    <span className='text-muted-foreground/60'>
                      ↑ {cacheWriteTokens.toLocaleString()}
                    </span>
                  )}
                </div>
              )}
            </div>
          )
        },
      },
      {
        accessorKey: 'quota',
        header: () => t('keyQuery.fields.cost'),
        cell: ({ row }) => {
          const log = row.original
          const isApiCall =
            log.type === LOG_TYPE_CONSUME ||
            log.type === LOG_TYPE_ERROR ||
            log.type === LOG_TYPE_REFUND
          if (!isApiCall) return null
          return (
            <span className='font-mono text-xs tabular-nums'>
              {formatLogQuota(log.quota)}
            </span>
          )
        },
      },
    ],
    [expandedRowId, t]
  )

  const table = useTable({
    features: appTableFeatures,
    data: logs,
    columns,
    state: {
      pagination: { pageIndex, pageSize },
    },
    enableRowSelection: false,
    manualPagination: true,
    manualFiltering: true,
    pageCount: Math.ceil(total / pageSize) || 1,
    onPaginationChange: (updater) => {
      const next =
        typeof updater === 'function'
          ? updater({ pageIndex, pageSize })
          : updater
      setPageIndex(next.pageIndex)
      setPageSize(next.pageSize)
      setExpandedRowId(null)
    },
  })

  const handleReset = () => {
    const range = getDefaultTimeRange()
    setStartTime(range.start)
    setEndTime(range.end)
    setModel('')
    setPageIndex(0)
  }

  return (
    <Card size='sm'>
      <CardHeader className='border-b'>
        <div className='flex items-center justify-between gap-3'>
          <CardTitle>{t('keyQuery.titles.callDetails')}</CardTitle>
          <Button
            variant='outline'
            onClick={exportLogs}
            disabled={logs.length === 0}
          >
            <Download />
            {t('keyQuery.actions.exportCsv')}
          </Button>
        </div>
      </CardHeader>

      <CardContent className='space-y-3 p-3'>
        {/* 筛选区域 */}
        <div className='bg-card/50 grid grid-cols-1 gap-2 rounded-lg border p-2.5 sm:grid-cols-[1fr_1fr_auto]'>
          <DateTimeRangeInput
            start={startTime}
            end={endTime}
            onStartChange={setStartTime}
            onEndChange={setEndTime}
          />
          <input
            className='border-input bg-background h-8 min-w-0 rounded-md border px-2 text-sm'
            placeholder={t('models.fields.modelName')}
            value={model}
            onChange={(event) => setModel(event.target.value)}
            onKeyDown={(event) => {
              if (event.key === 'Enter') {
                setPageIndex(0)
              }
            }}
          />
          <div className='flex items-center gap-1.5'>
            <Button
              type='button'
              variant='outline'
              size='sm'
              onClick={handleReset}
              disabled={isFetching}
            >
              {t('common.actions.reset')}
            </Button>
            <Button
              type='button'
              size='sm'
              onClick={() => setPageIndex(0)}
              disabled={isFetching}
            >
              {isFetching && <Loader2 className='animate-spin' />}
              {t('common.actions.search')}
            </Button>
          </div>
        </div>

        {/* 表格 */}
        <div className='overflow-x-auto rounded-md border'>
          <Table>
            <TableHeader>
              <TableRow>
                {table.getHeaderGroups().map((headerGroup) =>
                  headerGroup.headers.map((header) => (
                    <TableHead
                      key={header.id}
                      style={{
                        width:
                          header.getSize() && header.getSize() !== 150
                            ? header.getSize()
                            : undefined,
                      }}
                    >
                      {flexRender(
                        header.column.columnDef.header,
                        header.getContext()
                      )}
                    </TableHead>
                  ))
                )}
              </TableRow>
            </TableHeader>
            <TableBody>
              {isLoading ? (
                <TableRow>
                  <TableCell
                    colSpan={columns.length}
                    className='text-muted-foreground h-24 text-center'
                  >
                    <Loader2 className='text-muted-foreground mx-auto animate-spin' />
                  </TableCell>
                </TableRow>
              ) : logs.length === 0 ? (
                <TableRow>
                  <TableCell
                    colSpan={columns.length}
                    className='text-muted-foreground h-24 text-center'
                  >
                    {t('keyQuery.titles.noTokenLogs')}
                  </TableCell>
                </TableRow>
              ) : (
                table.getRowModel().rows.map((row) => {
                  const isExpanded = expandedRowId === row.original.id
                  return (
                    <LogRow
                      key={row.id}
                      row={row}
                      isExpanded={isExpanded}
                      onToggle={() =>
                        setExpandedRowId(isExpanded ? null : row.original.id)
                      }
                      visibility={visibility}
                    />
                  )
                })
              )}
            </TableBody>
          </Table>
        </div>

        {/* 分页 */}
        <SimplePagination
          page={pageIndex + 1}
          pageSize={pageSize}
          total={total}
          onPageChange={(next) => {
            setPageIndex(next - 1)
            setExpandedRowId(null)
          }}
          onPageSizeChange={(next) => {
            setPageSize(next)
            setPageIndex(0)
            setExpandedRowId(null)
          }}
        />
      </CardContent>
    </Card>
  )
}

/**
 * 简易分页器（不依赖 DataTable 上下文，适配公开页独立表格）。
 */
function SimplePagination(props: {
  page: number
  pageSize: number
  total: number
  onPageChange: (page: number) => void
  onPageSizeChange: (pageSize: number) => void
}) {
  const { t } = useTranslation()
  const totalPages = Math.max(1, Math.ceil(props.total / props.pageSize))
  return (
    <div className='text-muted-foreground flex flex-wrap items-center justify-between gap-2 text-xs'>
      <span>
        {t('common.fields.totalCount', { count: props.total })} ·{' '}
        {t('common.fields.pageCurrentOfTotal', {
          current: props.page,
          total: totalPages,
        })}
      </span>
      <div className='flex items-center gap-1.5'>
        <Button
          variant='outline'
          size='sm'
          className='h-7 px-2'
          onClick={() => props.onPageChange(props.page - 1)}
          disabled={props.page <= 1}
        >
          {t('common.actions.previous')}
        </Button>
        <Button
          variant='outline'
          size='sm'
          className='h-7 px-2'
          onClick={() => props.onPageChange(props.page + 1)}
          disabled={props.page >= totalPages}
        >
          {t('common.actions.next')}
        </Button>
        <span className='whitespace-nowrap'>
          {t('common.fields.rowsPerPage')}
        </span>
        <Select
          value={String(props.pageSize)}
          onValueChange={(value) => props.onPageSizeChange(Number(value))}
        >
          <SelectTrigger className='h-7 w-[64px]'>
            <SelectValue />
          </SelectTrigger>
          <SelectContent alignItemWithTrigger={false}>
            <SelectGroup>
              {[10, 20, 50, 100].map((size) => (
                <SelectItem key={size} value={String(size)}>
                  {size}
                </SelectItem>
              ))}
            </SelectGroup>
          </SelectContent>
        </Select>
      </div>
    </div>
  )
}

function DateTimeRangeInput(props: {
  start: Date
  end: Date
  onStartChange: (date: Date) => void
  onEndChange: (date: Date) => void
}) {
  const { t } = useTranslation()
  const toInputValue = (date: Date) => {
    const pad = (num: number) => String(num).padStart(2, '0')
    return `${date.getFullYear()}-${pad(date.getMonth() + 1)}-${pad(
      date.getDate()
    )}T${pad(date.getHours())}:${pad(date.getMinutes())}`
  }
  const fromInputValue = (value: string): Date | null => {
    const date = new Date(value)
    return Number.isNaN(date.getTime()) ? null : date
  }

  return (
    <div className='flex items-center gap-1.5'>
      <input
        type='datetime-local'
        className='border-input bg-background h-8 min-w-0 flex-1 rounded-md border px-2 text-sm tabular-nums'
        value={toInputValue(props.start)}
        onChange={(event) => {
          const next = fromInputValue(event.target.value)
          if (next) props.onStartChange(next)
        }}
        aria-label={t('dashboard.actions.startTime')}
      />
      <span className='text-muted-foreground text-xs'>~</span>
      <input
        type='datetime-local'
        className='border-input bg-background h-8 min-w-0 flex-1 rounded-md border px-2 text-sm tabular-nums'
        value={toInputValue(props.end)}
        onChange={(event) => {
          const next = fromInputValue(event.target.value)
          if (next) props.onEndChange(next)
        }}
        aria-label={t('dashboard.fields.endTime')}
      />
    </div>
  )
}

interface LogRowProps {
  row: Row<KeyQueryLog>
  isExpanded: boolean
  onToggle: () => void
  visibility: UsageLogDetailsVisibility
}

function LogRow({ row, isExpanded, onToggle, visibility }: LogRowProps) {
  const { t } = useTranslation()
  const log = row.original as KeyQueryLog

  const canShowDetails =
    visibility.detailsEnabled && isDisplayableType(log.type)

  return (
    <>
      <TableRow
        className={cn(
          'cursor-pointer transition-colors',
          isExpanded && 'bg-muted/40'
        )}
        onClick={onToggle}
      >
        {row.getVisibleCells().map((cell) => (
          <TableCell key={cell.id} className='py-2'>
            {flexRender(cell.column.columnDef.cell, cell.getContext())}
          </TableCell>
        ))}
      </TableRow>
      {isExpanded && (
        <TableRow>
          <TableCell
            colSpan={row.getVisibleCells().length}
            className='bg-muted/20 p-3'
          >
            {canShowDetails ? (
              <div className='max-h-[60vh] overflow-y-auto rounded-md border p-2'>
                <DetailsDialog
                  log={
                    log as unknown as import('@/features/usage-logs/data/schema').UsageLog
                  }
                  isAdmin={false}
                  open
                  onOpenChange={() => {}}
                  visibilityOverride={visibility}
                  inline
                />
              </div>
            ) : (
              <div className='bg-muted/30 rounded-md border p-3'>
                <p className='text-muted-foreground text-xs'>
                  {t('keyQuery.tips.detailsNotAvailable')}
                </p>
                {log.content && (
                  <p className='mt-2 text-xs wrap-break-word whitespace-pre-wrap'>
                    {log.content}
                  </p>
                )}
              </div>
            )}
          </TableCell>
        </TableRow>
      )}
    </>
  )
}

function isDisplayableType(type: number): boolean {
  return [0, 2, 5, 6].includes(type)
}

function escapeCsvValue(value: string): string {
  const text = String(value ?? '')
  const formulaSafe = /^[=+\-@\t\r]/.test(text) ? `'${text}` : text
  const escaped = formulaSafe.replace(/"/g, '""')
  return /[",\n\r]/.test(escaped) ? `"${escaped}"` : escaped
}
