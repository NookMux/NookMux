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
import { memo, useCallback, useEffect, useMemo, useRef, useState } from 'react'
import { Code2, Copy, Eye, Plus, Trash2 } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { cn } from '@/lib/utils'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { Switch } from '@/components/ui/switch'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'
import { Textarea } from '@/components/ui/textarea'
import {
  SettingsControl,
  SettingsRow,
  SettingsUnitField,
  SettingsUnitInput,
} from '../components/settings-group'
import {
  SettingsPageActionsPortal,
  SettingsPageFormActions,
} from '../components/settings-page-context'
import { SettingsPanel } from '../components/settings-panel'
import { useUpdateOption } from '../hooks/use-update-option'

const OPTION_KEY = 'tool_billing_setting.rules'

const TOOL_TYPES = [
  { value: 'web_search', label: 'common.fields.webSearch' },
  { value: 'image_generation', label: 'common.fields.imageGeneration' },
] as const

/** 相邻的兄弟元素之间画一条横线，首尾不留线。 */
const RULED = '[&>*+*]:border-t'

// --- 条件模式定义（与后端 common/condition.go 保持一致） ---
const CONDITION_MODES = [
  { value: 'eq', label: 'systemSettings.fields.condEq' },
  { value: 'neq', label: 'systemSettings.fields.condNeq' },
  { value: 'prefix', label: 'models.fields.prefix' },
  { value: 'suffix', label: 'models.fields.suffix' },
  { value: 'contains', label: 'models.fields.contains' },
  { value: 'regex', label: 'systemSettings.fields.condRegex' },
  { value: 'gt', label: 'common.fields.greaterThan' },
  { value: 'gte', label: 'common.fields.greaterThanOrEqual' },
  { value: 'lt', label: 'common.fields.lessThan' },
  { value: 'lte', label: 'common.fields.lessThanOrEqual' },
] as const

const CONDITION_LOGICS = [
  { value: 'AND', label: 'systemSettings.fields.logicAllAnd' },
  { value: 'OR', label: 'systemSettings.fields.logicAnyOr' },
] as const

type ConditionMode = (typeof CONDITION_MODES)[number]['value']
type ConditionLogic = (typeof CONDITION_LOGICS)[number]['value']

type ToolBillingCondition = {
  field: string
  mode: ConditionMode | string
  value: unknown
  invert?: boolean
  pass_missing_key?: boolean
}

type ToolBillingRule = {
  id: string
  name: string
  tool_type: 'web_search' | 'image_generation' | string
  billing_mode: 'per_call' | string
  price: number
  conditions?: ToolBillingCondition[]
  logic?: ConditionLogic | string
  enabled: boolean
}

/** 列表与右侧设置共用的规则，rowId 用来跟踪选中项。 */
type ToolBillingRow = ToolBillingRule & {
  rowId: number
}

function normalizeCondition(cond: ToolBillingCondition): ToolBillingCondition {
  return {
    field: cond.field?.trim() ?? '',
    mode: cond.mode || 'eq',
    value: cond.value ?? '',
    invert: cond.invert ?? false,
    pass_missing_key: cond.pass_missing_key ?? false,
  }
}

function normalizeRule(rule: ToolBillingRule): ToolBillingRule {
  return {
    id: rule.id?.trim() ?? '',
    name: rule.name?.trim() ?? '',
    tool_type: rule.tool_type || 'web_search',
    billing_mode: rule.billing_mode || 'per_call',
    price: Number(rule.price) || 0,
    conditions: (rule.conditions ?? []).map(normalizeCondition),
    logic: rule.logic || 'AND',
    enabled: rule.enabled !== false,
  }
}

function rowsToRules(rows: ToolBillingRow[]): ToolBillingRule[] {
  return rows.map(({ rowId: _rowId, ...rule }) => normalizeRule(rule))
}

function validateRules(
  rules: ToolBillingRule[],
  t: (key: string, options?: Record<string, unknown>) => string
): string | null {
  const seen = new Set<string>()
  for (const [index, rule] of rules.entries()) {
    if (!rule.id) {
      return t('systemSettings.errors.toolRuleIdRequired', { index })
    }
    if (seen.has(rule.id)) {
      return t('systemSettings.errors.toolRuleDuplicateId', {
        index,
        id: rule.id,
      })
    }
    seen.add(rule.id)
    if (!rule.name) {
      return t('systemSettings.errors.toolRuleNameRequired', {
        index,
        id: rule.id,
      })
    }
    if (!TOOL_TYPES.some((type) => type.value === rule.tool_type)) {
      return t('systemSettings.errors.toolRuleUnsupportedType', {
        index,
        id: rule.id,
      })
    }
    if (rule.billing_mode !== 'per_call') {
      return t('systemSettings.errors.toolRulePerCallOnly', {
        index,
        id: rule.id,
      })
    }
    if (!Number.isFinite(rule.price) || rule.price < 0) {
      return t('systemSettings.errors.toolRulePriceNonNegative', {
        index,
        id: rule.id,
      })
    }
    // 验证 conditions
    for (const [ci, cond] of (rule.conditions ?? []).entries()) {
      if (!cond.field) {
        return t('systemSettings.errors.toolRuleConditionEmptyField', {
          index,
          id: rule.id,
          conditionIndex: ci,
        })
      }
      if (!CONDITION_MODES.some((m) => m.value === cond.mode)) {
        return t('systemSettings.errors.toolRuleConditionUnsupportedMode', {
          index,
          id: rule.id,
          conditionIndex: ci,
          mode: cond.mode,
        })
      }
    }
  }
  return null
}

function parseInitialRules(
  rawValue: string | undefined,
  t: (key: string) => string
): ToolBillingRule[] {
  if (!rawValue) return []
  const parsed = JSON.parse(rawValue) as unknown
  if (!Array.isArray(parsed)) {
    throw new Error(t('systemSettings.errors.toolBillingRulesMustBeArray'))
  }
  return parsed.map((item) => normalizeRule(item as ToolBillingRule))
}

type ToolPriceSettingsProps = {
  defaultValue: string
  /** 恢复为系统默认的工具计费规则，操作立即生效。 */
  onRestoreDefaults: () => void
  isResetting: boolean
  /** 所在页签是否为当前页签；页头操作栏只在当前页签渲染。 */
  active: boolean
}

export const ToolPriceSettings = memo(function ToolPriceSettings({
  defaultValue,
  onRestoreDefaults,
  isResetting,
  active,
}: ToolPriceSettingsProps) {
  const { t } = useTranslation()
  const updateOption = useUpdateOption()
  const [editMode, setEditMode] = useState<'visual' | 'json'>('visual')
  const [rows, setRows] = useState<ToolBillingRow[]>([])
  const [jsonText, setJsonText] = useState('[]')
  const [jsonError, setJsonError] = useState('')
  const [nextRowId, setNextRowId] = useState(1)
  const [selectedRowId, setSelectedRowId] = useState<number | null>(null)
  const selectedRowRef = useRef<HTMLTableRowElement | null>(null)
  const idInputRef = useRef<HTMLInputElement | null>(null)
  /** 点了「添加」之后把光标放进新规则的 ID 输入框。 */
  const focusIdOnSelect = useRef(false)
  const conditionsListRef = useRef<HTMLDivElement | null>(null)
  /** 上次统计条件数时的规则与条数，用来认出「刚新增了一条条件」。 */
  const conditionCountRef = useRef<{
    rowId: number | null
    count: number
  } | null>(null)
  /** 已保存规则序列化后的样子，用来判断当前有没有未保存的改动。 */
  const [savedSnapshot, setSavedSnapshot] = useState<{
    rules: string
    json: string
  } | null>(null)

  const loadSavedRules = useCallback(() => {
    try {
      const rules = parseInitialRules(defaultValue, t)
      const initialRows = rules.map((rule, index) => ({
        ...rule,
        rowId: index + 1,
      }))
      const prettyJson = JSON.stringify(rules, null, 2)
      setRows(initialRows)
      setJsonText(prettyJson)
      setSavedSnapshot({ rules: JSON.stringify(rules), json: prettyJson })
      setJsonError('')
      setNextRowId(initialRows.length + 1)
    } catch (error) {
      const rawJson = defaultValue || '[]'
      setRows([])
      setJsonText(rawJson)
      setSavedSnapshot({ rules: JSON.stringify([]), json: rawJson })
      setJsonError(
        error instanceof Error
          ? error.message
          : t('systemSettings.errors.invalidJson')
      )
    }
  }, [defaultValue, t])

  useEffect(() => {
    loadSavedRules()
  }, [loadSavedRules])

  const currentRules = useMemo(() => rowsToRules(rows), [rows])
  const isDirty =
    savedSnapshot !== null &&
    (savedSnapshot.rules !== JSON.stringify(currentRules) ||
      savedSnapshot.json !== jsonText)

  const syncFromRows = useCallback(
    (nextRows: ToolBillingRow[]) => {
      const rules = rowsToRules(nextRows)
      setRows(nextRows)
      setJsonText(JSON.stringify(rules, null, 2))
      setJsonError(validateRules(rules, t) ?? '')
    },
    [t]
  )

  const handleJsonChange = useCallback(
    (text: string) => {
      setJsonText(text)
      try {
        const parsed = JSON.parse(text) as unknown
        if (!Array.isArray(parsed)) {
          setJsonError(t('systemSettings.errors.jsonMustBeAnArray'))
          return
        }
        const rules = parsed.map((item) =>
          normalizeRule(item as ToolBillingRule)
        )
        const validationError = validateRules(rules, t)
        if (validationError) {
          setJsonError(validationError)
          return
        }
        const nextRows = rules.map((rule, index) => ({
          ...rule,
          rowId: index + 1,
        }))
        setRows(nextRows)
        setNextRowId(nextRows.length + 1)
        setJsonError('')
      } catch (error) {
        setJsonError(
          error instanceof Error
            ? error.message
            : t('systemSettings.errors.invalidJson')
        )
      }
    },
    [t]
  )

  const updateRow = useCallback(
    (
      rowId: number,
      field: keyof ToolBillingRule,
      value: string | number | boolean
    ) => {
      syncFromRows(
        rows.map((row) =>
          row.rowId === rowId ? { ...row, [field]: value } : row
        )
      )
    },
    [rows, syncFromRows]
  )

  const updateCondition = useCallback(
    (
      rowId: number,
      condIndex: number,
      field: keyof ToolBillingCondition,
      value: string | boolean
    ) => {
      syncFromRows(
        rows.map((row) => {
          if (row.rowId !== rowId) return row
          const conditions = [...(row.conditions ?? [])]
          conditions[condIndex] = {
            ...conditions[condIndex],
            [field]: value,
          }
          return { ...row, conditions }
        })
      )
    },
    [rows, syncFromRows]
  )

  const addCondition = useCallback(
    (rowId: number) => {
      syncFromRows(
        rows.map((row) => {
          if (row.rowId !== rowId) return row
          const newCond: ToolBillingCondition = {
            field: '',
            mode: 'eq',
            value: '',
          }
          return { ...row, conditions: [...(row.conditions ?? []), newCond] }
        })
      )
    },
    [rows, syncFromRows]
  )

  const removeCondition = useCallback(
    (rowId: number, condIndex: number) => {
      syncFromRows(
        rows.map((row) => {
          if (row.rowId !== rowId) return row
          const conditions = (row.conditions ?? []).filter(
            (_, i) => i !== condIndex
          )
          return { ...row, conditions }
        })
      )
    },
    [rows, syncFromRows]
  )

  const addRow = useCallback(() => {
    const newRow: ToolBillingRow = {
      rowId: nextRowId,
      id: '',
      name: '',
      tool_type: 'web_search',
      billing_mode: 'per_call',
      price: 0,
      conditions: [],
      logic: 'AND',
      enabled: true,
    }
    setNextRowId((prev) => prev + 1)
    syncFromRows([...rows, newRow])
    setSelectedRowId(nextRowId)
    focusIdOnSelect.current = true
  }, [nextRowId, rows, syncFromRows])

  const removeRow = useCallback(
    (rowId: number) => {
      const index = rows.findIndex((row) => row.rowId === rowId)
      const nextRows = rows.filter((row) => row.rowId !== rowId)
      syncFromRows(nextRows)
      // 删掉的是选中项时，改选同一位置的规则，没有就选前一条。
      setSelectedRowId((current) => {
        if (current !== rowId) return current
        const neighbor = nextRows[Math.min(index, nextRows.length - 1)]
        return neighbor ? neighbor.rowId : null
      })
    },
    [rows, syncFromRows]
  )

  const selectedRow = useMemo(
    () => rows.find((row) => row.rowId === selectedRowId) ?? null,
    [rows, selectedRowId]
  )

  // 列表在面板内部滚动：选中行落到可视区之外时，把它滚回可视区。
  useEffect(() => {
    const row = selectedRowRef.current
    const container = row?.closest('[data-slot="table-container"]')
    if (!row || !container) return
    const rowBox = row.getBoundingClientRect()
    const containerBox = container.getBoundingClientRect()
    if (rowBox.top < containerBox.top || rowBox.bottom > containerBox.bottom) {
      row.scrollIntoView({ block: 'nearest' })
    }
  }, [selectedRowId])

  useEffect(() => {
    if (!focusIdOnSelect.current || !selectedRow) return
    focusIdOnSelect.current = false
    idInputRef.current?.focus()
  }, [selectedRow])

  // 条件列表在面板内部滚动：新增的条件排在末尾，把它滚进可视区。
  useEffect(() => {
    const rowId = selectedRow?.rowId ?? null
    const count = selectedRow?.conditions?.length ?? 0
    const previous = conditionCountRef.current
    conditionCountRef.current = { rowId, count }
    const added =
      previous !== null && previous.rowId === rowId && count > previous.count
    const list = conditionsListRef.current
    if (added && list) list.scrollTo({ top: list.scrollHeight })
  }, [selectedRow])

  const handleCopyJson = useCallback(async () => {
    try {
      await navigator.clipboard.writeText(jsonText)
      toast.success(t('systemSettings.status.copiedToClipboard'))
    } catch {
      toast.error(t('systemSettings.errors.failedToCopy'))
    }
  }, [jsonText, t])

  const handleDiscard = useCallback(() => {
    loadSavedRules()
    toast.success(t('systemSettings.status.formResetToSavedValues'))
  }, [loadSavedRules, t])

  const handleSave = useCallback(async () => {
    const validationError = validateRules(currentRules, t)
    if (validationError || jsonError) {
      toast.error(validationError || jsonError)
      return
    }
    await updateOption.mutateAsync({
      key: OPTION_KEY,
      value: JSON.stringify(currentRules),
    })
  }, [currentRules, jsonError, t, updateOption])

  const toggleEditMode = useCallback(() => {
    setEditMode((prev) => (prev === 'visual' ? 'json' : 'visual'))
  }, [])

  const toolTypeItems: Array<{ value: string; label: string }> = TOOL_TYPES.map(
    (option) => ({ value: option.value, label: t(option.label) })
  )
  const logicItems: Array<{ value: string; label: string }> =
    CONDITION_LOGICS.map((option) => ({
      value: option.value,
      label: t(option.label),
    }))
  const modeItems: Array<{ value: string; label: string }> =
    CONDITION_MODES.map((option) => ({
      value: option.value,
      label: t(option.label),
    }))

  const rulesDescription = (
    <>
      {t('systemSettings.actions.configureToolBillingRulesPricesAreUsdPerCall')}{' '}
      <code className='bg-muted rounded px-1 py-0.5 text-xs'>{OPTION_KEY}</code>
    </>
  )

  return (
    <div className='@container/editor flex min-h-0 flex-1 flex-col gap-8'>
      {active && (
        <>
          <SettingsPageActionsPortal>
            <Button
              type='button'
              variant='outline'
              size='sm'
              onClick={toggleEditMode}
            >
              {editMode === 'visual' ? (
                <>
                  <Code2 data-icon='inline-start' />
                  {t('systemSettings.actions.switchToJson')}
                </>
              ) : (
                <>
                  <Eye data-icon='inline-start' />
                  {t('systemSettings.actions.switchToVisual')}
                </>
              )}
            </Button>
          </SettingsPageActionsPortal>
          <SettingsPageFormActions
            onSave={() => void handleSave()}
            onReset={handleDiscard}
            isSaving={updateOption.isPending || isResetting}
            isDirty={isDirty}
            isSaveDisabled={editMode === 'json' && jsonError !== ''}
          />
        </>
      )}

      {editMode === 'visual' ? (
        <div className='grid min-h-[560px] shrink-0 gap-4 @4xl/editor:min-h-0 @4xl/editor:flex-1 @4xl/editor:grid-cols-[minmax(0,1.25fr)_minmax(360px,0.75fr)]'>
          <SettingsPanel
            title={t('systemSettings.fields.toolBillingRules')}
            description={rulesDescription}
            actions={
              <Button
                type='button'
                variant='outline'
                size='sm'
                onClick={addRow}
              >
                <Plus data-icon='inline-start' />
                {t('channels.actions.add')}
              </Button>
            }
            flush
            footer={
              <span className='text-muted-foreground text-xs'>
                {t('systemSettings.status.toolRuleCount', {
                  count: rows.length,
                })}
              </span>
            }
          >
            <Table containerClassName='max-h-[70vh] overflow-y-auto @4xl/editor:max-h-none @4xl/editor:h-full'>
              <TableHeader className='bg-card sticky top-0 z-10'>
                <TableRow>
                  <TableHead className='pl-4'>
                    {t('channels.fields.name')}
                  </TableHead>
                  <TableHead>{t('systemSettings.fields.toolType')}</TableHead>
                  <TableHead className='pr-4'>
                    {t('pricing.fields.price')}
                  </TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {rows.length === 0 ? (
                  <TableRow>
                    <TableCell
                      colSpan={3}
                      className='text-muted-foreground h-32 text-center'
                    >
                      {t('systemSettings.errors.noRulesConfigured')}
                    </TableCell>
                  </TableRow>
                ) : (
                  rows.map((row) => {
                    const selected = row.rowId === selectedRowId
                    const toolType = TOOL_TYPES.find(
                      (type) => type.value === row.tool_type
                    )
                    return (
                      <TableRow
                        key={row.rowId}
                        ref={selected ? selectedRowRef : undefined}
                        className={cn(
                          'cursor-pointer',
                          selected && 'bg-muted/70'
                        )}
                        onClick={() => setSelectedRowId(row.rowId)}
                      >
                        <TableCell
                          className={cn(
                            'w-full max-w-0 min-w-[9rem] truncate pl-4 font-medium',
                            !row.name && 'text-muted-foreground font-normal'
                          )}
                        >
                          {row.name ||
                            t('systemSettings.fields.unnamedToolRule')}
                        </TableCell>
                        <TableCell>
                          <Badge variant='outline'>
                            {t(toolType?.label ?? 'common.fields.webSearch')}
                          </Badge>
                        </TableCell>
                        <TableCell className='max-w-[240px] truncate pr-4 text-xs'>
                          {row.enabled
                            ? `$${row.price}`
                            : t('channels.status.disabled')}
                        </TableCell>
                      </TableRow>
                    )
                  })
                )}
              </TableBody>
            </Table>
          </SettingsPanel>

          <SettingsPanel
            title={
              selectedRow
                ? selectedRow.name || t('systemSettings.fields.unnamedToolRule')
                : t('systemSettings.titles.toolRuleSettings')
            }
            actions={
              selectedRow ? (
                <label className='flex cursor-pointer items-center gap-2 text-sm font-medium'>
                  {t('channels.actions.enable')}
                  <Switch
                    checked={selectedRow.enabled}
                    onCheckedChange={(checked) =>
                      updateRow(selectedRow.rowId, 'enabled', checked)
                    }
                  />
                </label>
              ) : undefined
            }
            fill
            footer={
              selectedRow ? (
                <Button
                  type='button'
                  variant='outline'
                  size='sm'
                  onClick={() => removeRow(selectedRow.rowId)}
                >
                  <Trash2
                    data-icon='inline-start'
                    className='text-destructive'
                  />
                  {t('systemSettings.actions.deleteToolRule')}
                </Button>
              ) : undefined
            }
          >
            {!selectedRow ? (
              <div className='text-muted-foreground py-16 text-center text-sm'>
                {t('systemSettings.placeholders.selectAToolRuleToEdit')}
              </div>
            ) : (
              <div
                className='flex min-h-0 flex-1 flex-col overflow-y-auto'
                key={selectedRow.rowId}
              >
                <div className={cn(RULED, 'shrink-0')}>
                  <SettingsRow label='ID' columns>
                    <Input
                      ref={idInputRef}
                      value={selectedRow.id}
                      placeholder='web_search_openai'
                      onChange={(event) =>
                        updateRow(selectedRow.rowId, 'id', event.target.value)
                      }
                    />
                  </SettingsRow>
                  <SettingsRow label={t('channels.fields.name')} columns>
                    <Input
                      value={selectedRow.name}
                      placeholder='OpenAI Web Search'
                      onChange={(event) =>
                        updateRow(selectedRow.rowId, 'name', event.target.value)
                      }
                    />
                  </SettingsRow>
                  <SettingsRow
                    label={t('systemSettings.fields.toolType')}
                    columns
                  >
                    <SettingsControl size='sm'>
                      <Select
                        items={toolTypeItems}
                        value={selectedRow.tool_type}
                        onValueChange={(value) =>
                          updateRow(selectedRow.rowId, 'tool_type', value ?? '')
                        }
                      >
                        <SelectTrigger className='w-full'>
                          <SelectValue />
                        </SelectTrigger>
                        <SelectContent alignItemWithTrigger={false}>
                          {TOOL_TYPES.map((option) => (
                            <SelectItem key={option.value} value={option.value}>
                              {t(option.label)}
                            </SelectItem>
                          ))}
                        </SelectContent>
                      </Select>
                    </SettingsControl>
                  </SettingsRow>
                  <SettingsRow label={t('pricing.fields.price')} columns>
                    <SettingsUnitField
                      input={
                        <SettingsUnitInput
                          type='number'
                          min={0}
                          step={0.001}
                          value={selectedRow.price}
                          onChange={(event) =>
                            updateRow(
                              selectedRow.rowId,
                              'price',
                              Number(event.target.value) || 0
                            )
                          }
                        />
                      }
                      unit='USD'
                    />
                  </SettingsRow>
                </div>

                <div className='flex min-h-0 flex-1 flex-col gap-3 border-t py-3'>
                  <div className='flex flex-wrap items-center gap-2'>
                    <span className='text-sm font-medium'>
                      {t('channels.fields.conditions')}
                    </span>
                    <span className='text-muted-foreground text-sm'>
                      {t('systemSettings.actions.match')}
                    </span>
                    <SettingsControl size='sm'>
                      <Select
                        items={logicItems}
                        value={selectedRow.logic || 'AND'}
                        onValueChange={(value) =>
                          updateRow(selectedRow.rowId, 'logic', value ?? 'AND')
                        }
                      >
                        <SelectTrigger
                          className='w-full'
                          aria-label={t('systemSettings.actions.match')}
                        >
                          <SelectValue />
                        </SelectTrigger>
                        <SelectContent alignItemWithTrigger={false}>
                          {CONDITION_LOGICS.map((option) => (
                            <SelectItem key={option.value} value={option.value}>
                              {t(option.label)}
                            </SelectItem>
                          ))}
                        </SelectContent>
                      </Select>
                    </SettingsControl>
                    <span className='text-muted-foreground text-sm'>
                      {t('systemSettings.fields.ofTheFollowingConditions')}
                    </span>
                  </div>

                  {(selectedRow.conditions ?? []).length > 0 && (
                    <div
                      ref={conditionsListRef}
                      className={cn(
                        RULED,
                        'hover-scrollbar min-h-0 overflow-y-auto'
                      )}
                    >
                      {(selectedRow.conditions ?? []).map((cond, index) => (
                        <div key={index} className='flex flex-col gap-2 py-3'>
                          <div className='flex items-center gap-2'>
                            <span className='text-muted-foreground w-10 shrink-0 font-mono text-xs'>
                              {index === 0
                                ? 'IF'
                                : (selectedRow.logic || 'AND') === 'AND'
                                  ? 'AND'
                                  : 'OR'}
                            </span>
                            <Input
                              value={cond.field}
                              placeholder='model / provider / quality / size'
                              className='min-w-0 flex-1'
                              onChange={(event) =>
                                updateCondition(
                                  selectedRow.rowId,
                                  index,
                                  'field',
                                  event.target.value
                                )
                              }
                            />
                            <Button
                              type='button'
                              variant='ghost'
                              size='icon'
                              onClick={() =>
                                removeCondition(selectedRow.rowId, index)
                              }
                              aria-label={t('common.actions.delete')}
                            >
                              <Trash2 className='text-destructive h-4 w-4' />
                            </Button>
                          </div>
                          <div className='grid gap-2 sm:grid-cols-2'>
                            <Select
                              items={modeItems}
                              value={cond.mode}
                              onValueChange={(value) =>
                                updateCondition(
                                  selectedRow.rowId,
                                  index,
                                  'mode',
                                  value ?? 'eq'
                                )
                              }
                            >
                              <SelectTrigger className='w-full'>
                                <SelectValue />
                              </SelectTrigger>
                              <SelectContent alignItemWithTrigger={false}>
                                {CONDITION_MODES.map((option) => (
                                  <SelectItem
                                    key={option.value}
                                    value={option.value}
                                  >
                                    {t(option.label)}
                                  </SelectItem>
                                ))}
                              </SelectContent>
                            </Select>
                            <Input
                              value={String(cond.value ?? '')}
                              placeholder={t(
                                'systemSettings.placeholders.conditionValue'
                              )}
                              onChange={(event) =>
                                updateCondition(
                                  selectedRow.rowId,
                                  index,
                                  'value',
                                  event.target.value
                                )
                              }
                            />
                          </div>
                          <label className='flex items-center justify-between gap-2 text-sm'>
                            {t('systemSettings.actions.invert')}
                            <Switch
                              checked={cond.invert ?? false}
                              onCheckedChange={(checked) =>
                                updateCondition(
                                  selectedRow.rowId,
                                  index,
                                  'invert',
                                  checked
                                )
                              }
                            />
                          </label>
                        </div>
                      ))}
                    </div>
                  )}

                  <div>
                    <Button
                      type='button'
                      variant='outline'
                      size='sm'
                      onClick={() => addCondition(selectedRow.rowId)}
                    >
                      <Plus data-icon='inline-start' />
                      {t('systemSettings.actions.addCondition')}
                    </Button>
                  </div>
                </div>
              </div>
            )}
          </SettingsPanel>
        </div>
      ) : (
        <SettingsPanel
          title={t('systemSettings.fields.toolBillingRules')}
          description={rulesDescription}
          actions={
            <Button
              type='button'
              variant='outline'
              size='sm'
              onClick={handleCopyJson}
            >
              <Copy data-icon='inline-start' />
              {t('channels.actions.copy')}
            </Button>
          }
          className='min-h-0 flex-1'
          scroll
        >
          <div className='flex flex-col gap-2 py-2'>
            <Textarea
              value={jsonText}
              onChange={(event) => handleJsonChange(event.target.value)}
              className='font-mono text-sm'
              rows={16}
              spellCheck={false}
            />
            {jsonError !== '' && (
              <p className='text-destructive text-sm'>{jsonError}</p>
            )}
          </div>
        </SettingsPanel>
      )}

      <SettingsPanel
        tone='destructive'
        className='shrink-0'
        inline
        title={t('systemSettings.actions.resetToolBillingRules')}
        description={t(
          'systemSettings.tips.clearToolBillingRulesAndRestoreDefaults'
        )}
        actions={
          <Button
            type='button'
            variant='destructive'
            size='sm'
            onClick={onRestoreDefaults}
            disabled={isResetting || updateOption.isPending}
          >
            {t('common.actions.resetToDefault')}
          </Button>
        }
      />
    </div>
  )
})
