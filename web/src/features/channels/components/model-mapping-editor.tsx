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
import {
  useCallback,
  useEffect,
  useMemo,
  useRef,
  useState,
  type ReactNode,
} from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { Code, Plus, Save, Table, Trash2 } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { cn } from '@/lib/utils'
import { Button } from '@/components/ui/button'
import { Empty, EmptyDescription, EmptyHeader } from '@/components/ui/empty'
import { Input } from '@/components/ui/input'
import {
  Select,
  SelectContent,
  SelectGroup,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { Textarea } from '@/components/ui/textarea'
import {
  deleteOptionJsonMapEntry,
  getOptionJsonMap,
  getSystemOptionValue,
  upsertOptionJsonMapEntry,
} from '@/features/system-settings/api'

type ModelMappingEditorProps = {
  value: string
  onChange: (value: string) => void
  disabled?: boolean
  /** 可视模式左列标题，默认 "Original Model" */
  fromLabel?: string
  /** 可视模式右列标题，默认 "Replacement Model" */
  toLabel?: string
  /** 左列输入框 placeholder */
  fromPlaceholder?: string
  /** 右列输入框 placeholder */
  toPlaceholder?: string
  /** JSON 模式 placeholder */
  jsonPlaceholder?: string
  /** "填入模板"按钮写入的 JSON 字符串；不传则使用内置默认模板 */
  template?: string
  /** 空状态提示文案，默认复用模型映射默认值 */
  emptyText?: string
  /** 系统设置 option key。传入后可视模式改为服务端分页编辑。 */
  optionKey?: string
  /** 服务端分页模式下，切换到 JSON 模式后回传完整原始值。 */
  onFullValueLoaded?: (value: string) => void
  /**
   * 自行安排「JSON / 可视」切换按钮：拿到当前模式与切换函数，返回要渲染的节点。
   * 不传时按钮显示在编辑器工具栏里。
   */
  renderModeToggle?: (controls: {
    visualMode: boolean
    toggle: () => void
  }) => ReactNode
}

type MappingRow = {
  id: string
  from: string
  to: string
  originalFrom?: string
  originalTo?: string
  isNew?: boolean
}

const PAGE_SIZE_OPTIONS = [10, 20, 50, 100]
const DEFAULT_MODEL_MAPPING_PAGE_SIZE = 20

function formatJsonForEditor(value: string) {
  const trimmed = value.trim()
  if (!trimmed) {
    return '{}'
  }

  try {
    return JSON.stringify(JSON.parse(trimmed), null, 2)
  } catch {
    return value
  }
}

export function ModelMappingEditor({
  value,
  onChange,
  disabled = false,
  fromLabel,
  toLabel,
  fromPlaceholder = 'gpt-3.5-turbo',
  toPlaceholder = 'gpt-3.5-turbo-0125',
  jsonPlaceholder,
  template,
  emptyText,
  optionKey,
  onFullValueLoaded,
  renderModeToggle,
}: ModelMappingEditorProps) {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const [mode, setMode] = useState<'visual' | 'json'>('visual')
  const [rows, setRows] = useState<MappingRow[]>([])
  const [jsonValue, setJsonValue] = useState(value)
  const [pageIndex, setPageIndex] = useState(0)
  const [pageSize, setPageSize] = useState(DEFAULT_MODEL_MAPPING_PAGE_SIZE)
  const nextRowIdRef = useRef(0)
  // Tracks the last JSON string we pushed up via onChange. The external-sync
  // effect uses this to ignore echoes of our own changes, which would otherwise
  // discard in-progress rows that aren't yet representable in JSON (e.g. a row
  // whose "from" is still empty gets dropped by convertRowsToJson -> "{}",
  // then re-parsed back into zero rows).
  const emittedValueRef = useRef<string | null>(null)

  const createRowId = useCallback(() => {
    nextRowIdRef.current += 1
    return `mapping-${nextRowIdRef.current}`
  }, [])

  const isServerPaginated = Boolean(optionKey)
  const failedToLoadSettingsMessage = t('channels.errors.failedToLoadSettings')

  const jsonMapQuery = useQuery({
    queryKey: [
      'system-option-json-map',
      optionKey,
      pageIndex + 1,
      pageSize,
      failedToLoadSettingsMessage,
    ],
    queryFn: async () => {
      const data = await getOptionJsonMap({
        key: optionKey ?? '',
        page: pageIndex + 1,
        pageSize,
      })
      if (!data.success) {
        throw new Error(data.message || failedToLoadSettingsMessage)
      }
      return data.data
    },
    enabled: isServerPaginated && mode === 'visual',
  })

  const fullJsonQuery = useQuery({
    queryKey: ['system-option-value', optionKey, failedToLoadSettingsMessage],
    queryFn: async () => {
      const data = await getSystemOptionValue(optionKey ?? '')
      if (!data.success) {
        throw new Error(data.message || failedToLoadSettingsMessage)
      }
      return data.data.value
    },
    enabled: false,
  })

  const deleteEntryMutation = useMutation({
    mutationFn: async (mapKey: string) => {
      if (!optionKey) return
      const data = await deleteOptionJsonMapEntry({
        key: optionKey,
        map_key: mapKey,
      })
      if (!data.success) {
        throw new Error(
          data.message || t('channels.errors.failedToUpdateSetting')
        )
      }
    },
    onSuccess: async () => {
      await Promise.all([
        queryClient.invalidateQueries({
          queryKey: ['system-option-json-map', optionKey],
        }),
        queryClient.invalidateQueries({
          queryKey: ['system-option-value', optionKey],
        }),
        queryClient.invalidateQueries({ queryKey: ['system-options'] }),
      ])
      toast.success(t('channels.status.settingsUpdatedSuccessfully'))
    },
    onError: (error: Error) => {
      toast.error(error.message || t('channels.errors.failedToUpdateSetting'))
    },
  })

  const upsertEntryMutation = useMutation({
    mutationFn: async (row: MappingRow) => {
      if (!optionKey) return
      const mapKey = row.from.trim()
      if (!mapKey) {
        throw new Error(t('channels.errors.mappingKeyCannotBeEmpty'))
      }
      const data = await upsertOptionJsonMapEntry({
        key: optionKey,
        map_key: mapKey,
        old_map_key: row.isNew ? undefined : row.originalFrom,
        value: row.to.trim(),
      })
      if (!data.success) {
        throw new Error(
          data.message || t('channels.errors.failedToUpdateSetting')
        )
      }
    },
    onSuccess: async () => {
      await Promise.all([
        queryClient.invalidateQueries({
          queryKey: ['system-option-json-map', optionKey],
        }),
        queryClient.invalidateQueries({
          queryKey: ['system-option-value', optionKey],
        }),
        queryClient.invalidateQueries({ queryKey: ['system-options'] }),
      ])
      toast.success(t('channels.status.settingsUpdatedSuccessfully'))
    },
    onError: (error: Error) => {
      toast.error(error.message || t('channels.errors.failedToUpdateSetting'))
    },
  })

  const parseJsonToRows = useCallback(
    (json: string) => {
      try {
        if (!json.trim()) {
          setRows([])
          return
        }
        const parsed = JSON.parse(json)
        const entries = Object.entries(parsed)
        setRows((previousRows) => {
          const remainingRows = [...previousRows]
          return entries.map(([from, to], index) => {
            const toString = String(to)
            const existingIndex = remainingRows.findIndex(
              (row) =>
                row.from === from ||
                (row.from === from && row.to === toString) ||
                previousRows[index]?.id === row.id
            )
            if (existingIndex >= 0) {
              const [existing] = remainingRows.splice(existingIndex, 1)
              return {
                id: existing.id,
                from,
                to: toString,
              }
            }
            return {
              id: createRowId(),
              from,
              to: toString,
            }
          })
        })
      } catch (_error) {
        // Invalid JSON, keep current rows
      }
    },
    [createRowId]
  )

  // Sync external value changes into local rows/json state. Skip the re-parse
  // when the incoming value is just the echo of our own onChange, so in-progress
  // rows (e.g. with an empty "from") are not wiped out.
  useEffect(() => {
    if (isServerPaginated) {
      setJsonValue(value)
      return
    }
    setJsonValue(value)
    if (value === emittedValueRef.current) {
      return
    }
    emittedValueRef.current = null
    parseJsonToRows(value)
  }, [isServerPaginated, parseJsonToRows, value])

  useEffect(() => {
    if (!isServerPaginated || !jsonMapQuery.data) {
      return
    }
    setRows(
      (jsonMapQuery.data.items ?? []).map((item) => ({
        id: `server-${item.key}`,
        from: item.key,
        to: item.value,
        originalFrom: item.key,
        originalTo: item.value,
      }))
    )
  }, [isServerPaginated, jsonMapQuery.data])

  const convertRowsToJson = (updatedRows: MappingRow[]): string => {
    const obj: Record<string, string> = {}
    updatedRows.forEach((row) => {
      if (row.from.trim()) {
        obj[row.from.trim()] = row.to.trim()
      }
    })
    return JSON.stringify(obj, null, 2)
  }

  const unsavedServerRowCount = useMemo(
    () => (isServerPaginated ? rows.filter((row) => row.isNew).length : 0),
    [isServerPaginated, rows]
  )

  const pageCount = useMemo(
    () =>
      Math.max(
        1,
        Math.ceil(
          (isServerPaginated
            ? (jsonMapQuery.data?.total ?? 0) + unsavedServerRowCount
            : rows.length) / pageSize
        )
      ),
    [
      isServerPaginated,
      jsonMapQuery.data?.total,
      pageSize,
      rows.length,
      unsavedServerRowCount,
    ]
  )

  const safePageIndex = Math.min(pageIndex, pageCount - 1)

  const pageRows = useMemo(
    () =>
      rows.slice(safePageIndex * pageSize, safePageIndex * pageSize + pageSize),
    [pageSize, rows, safePageIndex]
  )

  const visibleRows = isServerPaginated ? rows : pageRows
  const totalRows = isServerPaginated
    ? (jsonMapQuery.data?.total ?? 0) + unsavedServerRowCount
    : rows.length
  const isLoadingRows = isServerPaginated && jsonMapQuery.isLoading

  useEffect(() => {
    if (pageIndex !== safePageIndex) {
      setPageIndex(safePageIndex)
    }
  }, [pageIndex, safePageIndex])

  // Push a JSON change up to the parent and remember it so the sync effect
  // treats the resulting prop update as our own and does not re-parse it.
  const emitChange = (json: string) => {
    emittedValueRef.current = json
    setJsonValue(json)
    onChange(json)
  }

  const handleAddRow = () => {
    const newRow: MappingRow = {
      id: createRowId(),
      from: '',
      to: '',
      isNew: true,
    }
    if (isServerPaginated) {
      setRows((currentRows) => [...currentRows, newRow])
      return
    }
    const updatedRows = [...rows, newRow]
    setRows(updatedRows)
    setPageIndex(Math.max(0, Math.ceil(updatedRows.length / pageSize) - 1))
  }

  const handleDeleteRow = (id: string) => {
    if (isServerPaginated) {
      const row = rows.find((item) => item.id === id)
      if (row?.isNew) {
        setRows((currentRows) => currentRows.filter((item) => item.id !== id))
        return
      }
      const mapKey = row?.originalFrom ?? row?.from ?? id
      deleteEntryMutation.mutate(mapKey)
      return
    }
    const updatedRows = rows.filter((row) => row.id !== id)
    setRows(updatedRows)
    emitChange(convertRowsToJson(updatedRows))
  }

  const handleRowChange = (
    id: string,
    field: 'from' | 'to',
    newValue: string
  ) => {
    const updatedRows = rows.map((row) =>
      row.id === id ? { ...row, [field]: newValue } : row
    )
    setRows(updatedRows)
    if (isServerPaginated) {
      return
    }
    emitChange(convertRowsToJson(updatedRows))
  }

  const isRowDirty = (row: MappingRow) =>
    Boolean(
      row.isNew || row.from !== row.originalFrom || row.to !== row.originalTo
    )

  const handleSaveRow = (row: MappingRow) => {
    upsertEntryMutation.mutate(row)
  }

  const handleJsonChange = (newJson: string) => {
    emitChange(newJson)
    if (!isServerPaginated) {
      parseJsonToRows(newJson)
    }
  }

  const handleFillTemplate = () => {
    const templateJson =
      template ??
      JSON.stringify({ 'gpt-3.5-turbo': 'gpt-3.5-turbo-0125' }, null, 2)
    emitChange(templateJson)
    if (!isServerPaginated) {
      parseJsonToRows(templateJson)
    }
  }

  const toggleMode = async () => {
    if (mode === 'visual') {
      if (isServerPaginated) {
        const result = await fullJsonQuery.refetch()
        if (result.isError) {
          toast.error(
            result.error.message || t('channels.errors.failedToLoadSettings')
          )
          return
        }
        const fullValue = result.data ?? '{}'
        onFullValueLoaded?.(fullValue)
        setJsonValue(formatJsonForEditor(fullValue))
      } else {
        // Switching to JSON mode: sync rows to JSON
        emitChange(convertRowsToJson(rows))
      }
      setMode('json')
    } else {
      // Switching to visual mode: sync JSON to rows
      if (!isServerPaginated) {
        parseJsonToRows(jsonValue)
      }
      setMode('visual')
    }
  }

  return (
    <div className='space-y-2'>
      <div className='flex flex-wrap items-center justify-between gap-2'>
        <div className='flex gap-2'>
          {renderModeToggle ? (
            renderModeToggle({
              visualMode: mode === 'visual',
              toggle: toggleMode,
            })
          ) : (
            <Button
              type='button'
              variant='outline'
              size='sm'
              onClick={toggleMode}
              disabled={disabled}
            >
              {mode === 'visual' ? (
                <>
                  <Code className='mr-2 h-4 w-4' />
                  {t('common.fields.jsonMode')}
                </>
              ) : (
                <>
                  <Table className='mr-2 h-4 w-4' />
                  {t('common.fields.visualMode')}
                </>
              )}
            </Button>
          )}
          {!isServerPaginated || mode === 'json' ? (
            <Button
              type='button'
              variant='link'
              size='sm'
              className='h-auto p-0'
              onClick={handleFillTemplate}
              disabled={disabled}
            >
              {t('common.actions.fillTemplate')}
            </Button>
          ) : null}
        </div>
        {mode === 'visual' ? (
          <Button
            type='button'
            variant='outline'
            size='sm'
            onClick={handleAddRow}
            disabled={disabled}
          >
            <Plus className='mr-2 h-4 w-4' />
            {t('channels.actions.addMapping')}
          </Button>
        ) : null}
      </div>

      {mode === 'visual' ? (
        <div className='space-y-2'>
          {isLoadingRows || totalRows > 0 ? (
            <div className='space-y-2'>
              <div className='grid grid-cols-[minmax(0,1fr)_minmax(0,1fr)_auto] gap-2 text-sm font-medium'>
                <div>
                  {fromLabel
                    ? t(fromLabel)
                    : t('channels.fields.originalModel')}
                </div>
                <div>
                  {toLabel ? t(toLabel) : t('channels.fields.replacementModel')}
                </div>
                <div className={isServerPaginated ? 'w-20' : 'w-10'}></div>
              </div>
              {isLoadingRows ? (
                <Empty>
                  <EmptyHeader>
                    <EmptyDescription>
                      {t('common.tips.loading')}
                    </EmptyDescription>
                  </EmptyHeader>
                </Empty>
              ) : null}
              {visibleRows.map((row) => (
                <div
                  key={row.id}
                  className='grid grid-cols-[minmax(0,1fr)_minmax(0,1fr)_auto] gap-2'
                >
                  <Input
                    value={row.from}
                    onChange={(e) =>
                      handleRowChange(row.id, 'from', e.target.value)
                    }
                    placeholder={fromPlaceholder}
                    disabled={disabled}
                    className='min-w-0'
                  />
                  <Input
                    value={row.to}
                    onChange={(e) =>
                      handleRowChange(row.id, 'to', e.target.value)
                    }
                    placeholder={toPlaceholder}
                    disabled={disabled}
                    className='min-w-0'
                  />
                  <div className='flex items-center justify-end gap-1'>
                    {isServerPaginated ? (
                      <Button
                        type='button'
                        variant='ghost'
                        size='icon'
                        onClick={() => handleSaveRow(row)}
                        disabled={
                          disabled ||
                          upsertEntryMutation.isPending ||
                          !row.from.trim() ||
                          !isRowDirty(row)
                        }
                        className='h-10 w-10'
                        aria-label={t('channels.actions.save')}
                      >
                        <Save className='h-4 w-4' />
                      </Button>
                    ) : null}
                    <Button
                      type='button'
                      variant='ghost'
                      size='icon'
                      onClick={() => handleDeleteRow(row.id)}
                      disabled={disabled || deleteEntryMutation.isPending}
                      className='h-10 w-10'
                    >
                      <Trash2 className='h-4 w-4' />
                    </Button>
                  </div>
                </div>
              ))}
            </div>
          ) : (
            <Empty>
              <EmptyHeader>
                <EmptyDescription>
                  {emptyText
                    ? t(emptyText)
                    : t(
                        'common.tips.noModelMappingsConfiguredClickAddMappingToGet'
                      )}
                </EmptyDescription>
              </EmptyHeader>
            </Empty>
          )}
          {totalRows > 0 ? (
            <div className='flex flex-col gap-3 border-t pt-3 sm:flex-row sm:items-center sm:justify-between'>
              <div className='text-muted-foreground flex flex-wrap items-center gap-3 text-xs'>
                <span>
                  {t('channels.tips.showingStartEndOfCountMappings', {
                    start: Math.min(totalRows, safePageIndex * pageSize + 1),
                    end: Math.min(
                      totalRows,
                      safePageIndex * pageSize + visibleRows.length
                    ),
                    count: totalRows,
                  })}
                </span>
                <div className='flex items-center gap-2'>
                  <span>{t('common.fields.rowsPerPage')}</span>
                  <Select
                    value={String(pageSize)}
                    onValueChange={(value) => {
                      setPageSize(Number(value))
                      setPageIndex(0)
                    }}
                  >
                    <SelectTrigger className='h-8 w-[70px]' disabled={disabled}>
                      <SelectValue />
                    </SelectTrigger>
                    <SelectContent alignItemWithTrigger={false}>
                      <SelectGroup>
                        {PAGE_SIZE_OPTIONS.map((size) => (
                          <SelectItem key={size} value={String(size)}>
                            {size}
                          </SelectItem>
                        ))}
                      </SelectGroup>
                    </SelectContent>
                  </Select>
                </div>
              </div>
              <div className='flex items-center gap-2'>
                <Button
                  type='button'
                  variant='outline'
                  size='sm'
                  disabled={disabled || safePageIndex === 0}
                  onClick={() =>
                    setPageIndex(() => Math.max(0, safePageIndex - 1))
                  }
                >
                  {t('common.actions.previous')}
                </Button>
                <span className='text-muted-foreground text-xs'>
                  {safePageIndex + 1} / {pageCount}
                </span>
                <Button
                  type='button'
                  variant='outline'
                  size='sm'
                  disabled={disabled || safePageIndex >= pageCount - 1}
                  onClick={() =>
                    setPageIndex(() =>
                      Math.min(pageCount - 1, safePageIndex + 1)
                    )
                  }
                >
                  {t('common.actions.next')}
                </Button>
              </div>
            </div>
          ) : null}
        </div>
      ) : (
        <div className='max-w-full min-w-0 overflow-hidden'>
          <Textarea
            value={jsonValue}
            onChange={(e) => handleJsonChange(e.target.value)}
            placeholder={
              jsonPlaceholder ?? t('common.tips.originalModelReplacementModel')
            }
            disabled={disabled}
            rows={8}
            wrap='off'
            className={cn(
              '[field-sizing:fixed] h-48 max-h-48 min-h-48 w-full max-w-full min-w-0 resize-none overflow-auto font-mono text-sm whitespace-pre'
            )}
          />
        </div>
      )}
    </div>
  )
}
