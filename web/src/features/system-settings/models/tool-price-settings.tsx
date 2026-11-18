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
import { memo, useCallback, useEffect, useMemo, useState } from 'react'
import {
  ChevronDown,
  ChevronRight,
  Code2,
  Copy,
  Eye,
  Plus,
  Trash2,
} from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { Alert, AlertDescription } from '@/components/ui/alert'
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
import { Textarea } from '@/components/ui/textarea'
import { SettingsPageActionsPortal } from '../components/settings-page-context'
import { useUpdateOption } from '../hooks/use-update-option'

const OPTION_KEY = 'tool_billing_setting.rules'

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

type ToolBillingRow = ToolBillingRule & {
  rowId: number
  conditionsExpanded: boolean
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
  return rows.map(
    ({ rowId: _rowId, conditionsExpanded: _conditionsExpanded, ...rule }) =>
      normalizeRule(rule)
  )
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
    if (!['web_search', 'image_generation'].includes(rule.tool_type)) {
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
  onReset: () => void
  isResetting: boolean
}

export const ToolPriceSettings = memo(function ToolPriceSettings({
  defaultValue,
  onReset,
  isResetting,
}: ToolPriceSettingsProps) {
  const { t } = useTranslation()
  const updateOption = useUpdateOption()
  const [editMode, setEditMode] = useState<'visual' | 'json'>('visual')
  const [rows, setRows] = useState<ToolBillingRow[]>([])
  const [jsonText, setJsonText] = useState('[]')
  const [jsonError, setJsonError] = useState('')
  const [nextRowId, setNextRowId] = useState(1)

  useEffect(() => {
    try {
      const rules = parseInitialRules(defaultValue, t)
      const initialRows = rules.map((rule, index) => ({
        ...rule,
        rowId: index + 1,
        conditionsExpanded: false,
      }))
      setRows(initialRows)
      setJsonText(JSON.stringify(rules, null, 2))
      setJsonError('')
      setNextRowId(initialRows.length + 1)
    } catch (error) {
      setRows([])
      setJsonText(defaultValue || '[]')
      setJsonError(
        error instanceof Error
          ? error.message
          : t('systemSettings.errors.invalidJson')
      )
    }
  }, [defaultValue, t])

  const currentRules = useMemo(() => rowsToRules(rows), [rows])

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
          conditionsExpanded: false,
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

  const toggleConditionsExpanded = useCallback(
    (rowId: number) => {
      setRows(
        rows.map((row) =>
          row.rowId === rowId
            ? { ...row, conditionsExpanded: !row.conditionsExpanded }
            : row
        )
      )
    },
    [rows]
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
          return {
            ...row,
            conditions: [...(row.conditions ?? []), newCond],
            conditionsExpanded: true,
          }
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
      conditionsExpanded: false,
    }
    setNextRowId((prev) => prev + 1)
    syncFromRows([...rows, newRow])
  }, [nextRowId, rows, syncFromRows])

  const removeRow = useCallback(
    (rowId: number) => {
      syncFromRows(rows.filter((row) => row.rowId !== rowId))
    },
    [rows, syncFromRows]
  )

  const handleCopyJson = useCallback(async () => {
    try {
      await navigator.clipboard.writeText(jsonText)
      toast.success(t('systemSettings.status.copiedToClipboard'))
    } catch {
      toast.error(t('systemSettings.errors.failedToCopy'))
    }
  }, [jsonText, t])

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

  return (
    <div className='space-y-4'>
      <Alert>
        <AlertDescription className='space-y-1 text-sm'>
          <div>
            {t(
              'systemSettings.actions.configureToolBillingRulesPricesAreUsdPerCall'
            )}
          </div>
          <div>
            <code className='bg-muted rounded px-1 py-0.5 text-xs'>
              tool_billing_setting.rules
            </code>
          </div>
        </AlertDescription>
      </Alert>

      <div className='flex flex-wrap items-center justify-between gap-2'>
        <div className='flex flex-wrap items-center gap-2'>
          {editMode === 'visual' ? (
            <Button variant='outline' size='sm' onClick={addRow}>
              <Plus className='mr-2 h-4 w-4' />
              {t('channels.actions.add')}
            </Button>
          ) : (
            <Button variant='ghost' size='sm' onClick={handleCopyJson}>
              <Copy className='mr-2 h-4 w-4' />
              {t('channels.actions.copy')}
            </Button>
          )}
        </div>
        <Button variant='outline' size='sm' onClick={toggleEditMode}>
          {editMode === 'visual' ? (
            <>
              <Code2 className='mr-2 h-4 w-4' />
              {t('systemSettings.actions.switchToJson')}
            </>
          ) : (
            <>
              <Eye className='mr-2 h-4 w-4' />
              {t('systemSettings.actions.switchToVisual')}
            </>
          )}
        </Button>
      </div>

      {editMode === 'visual' ? (
        <div className='space-y-3'>
          {rows.length === 0 ? (
            <div className='text-muted-foreground rounded-md border py-8 text-center text-sm'>
              {t('systemSettings.errors.noRulesConfigured')}
            </div>
          ) : (
            rows.map((row) => (
              <div key={row.rowId} className='rounded-md border'>
                {/* 主行 */}
                <div className='flex flex-wrap items-center gap-2 border-b p-3'>
                  <div className='min-w-[140px] flex-1'>
                    <label className='text-muted-foreground mb-1 block text-xs'>
                      ID
                    </label>
                    <Input
                      value={row.id}
                      placeholder='web_search_openai'
                      className='h-8'
                      onChange={(e) =>
                        updateRow(row.rowId, 'id', e.target.value)
                      }
                    />
                  </div>
                  <div className='min-w-[140px] flex-1'>
                    <label className='text-muted-foreground mb-1 block text-xs'>
                      {t('channels.fields.name')}
                    </label>
                    <Input
                      value={row.name}
                      placeholder='OpenAI Web Search'
                      className='h-8'
                      onChange={(e) =>
                        updateRow(row.rowId, 'name', e.target.value)
                      }
                    />
                  </div>
                  <div className='min-w-[120px]'>
                    <label className='text-muted-foreground mb-1 block text-xs'>
                      {t('systemSettings.fields.toolType')}
                    </label>
                    <Select
                      value={row.tool_type}
                      onValueChange={(v) =>
                        updateRow(row.rowId, 'tool_type', v ?? '')
                      }
                    >
                      <SelectTrigger className='h-8'>
                        <SelectValue />
                      </SelectTrigger>
                      <SelectContent>
                        <SelectItem value='web_search'>
                          {t('common.fields.webSearch')}
                        </SelectItem>
                        <SelectItem value='image_generation'>
                          {t('common.fields.imageGeneration')}
                        </SelectItem>
                      </SelectContent>
                    </Select>
                  </div>
                  <div className='min-w-[100px]'>
                    <label className='text-muted-foreground mb-1 block text-xs'>
                      {t('pricing.fields.price')}
                    </label>
                    <Input
                      type='number'
                      min={0}
                      step={0.001}
                      value={row.price}
                      className='h-8'
                      onChange={(e) =>
                        updateRow(
                          row.rowId,
                          'price',
                          Number(e.target.value) || 0
                        )
                      }
                    />
                  </div>
                  <div className='flex items-end gap-2'>
                    <label className='text-muted-foreground flex items-center gap-1 text-xs'>
                      <Switch
                        checked={row.enabled}
                        onCheckedChange={(checked) =>
                          updateRow(row.rowId, 'enabled', checked)
                        }
                      />
                      {t('channels.status.enabled')}
                    </label>
                  </div>
                  <div className='flex items-end gap-1'>
                    <Button
                      variant='ghost'
                      size='sm'
                      className='h-8'
                      onClick={() => toggleConditionsExpanded(row.rowId)}
                    >
                      {row.conditionsExpanded ? (
                        <ChevronDown className='h-4 w-4' />
                      ) : (
                        <ChevronRight className='h-4 w-4' />
                      )}
                      <span className='ml-1 text-xs'>
                        {t('channels.fields.conditions')} (
                        {row.conditions?.length ?? 0})
                      </span>
                    </Button>
                  </div>
                  <div className='flex items-end'>
                    <Button
                      variant='ghost'
                      size='icon'
                      className='h-8'
                      onClick={() => removeRow(row.rowId)}
                      aria-label={t('common.actions.delete')}
                    >
                      <Trash2 className='text-destructive h-4 w-4' />
                    </Button>
                  </div>
                </div>

                {/* 条件展开区 */}
                {row.conditionsExpanded && (
                  <div className='space-y-2 p-3'>
                    {/* Logic 选择器 */}
                    <div className='flex items-center gap-2'>
                      <span className='text-muted-foreground text-xs'>
                        {t('systemSettings.actions.match')}:
                      </span>
                      <Select
                        value={row.logic || 'AND'}
                        onValueChange={(v) =>
                          updateRow(row.rowId, 'logic', v ?? 'AND')
                        }
                      >
                        <SelectTrigger className='h-7 w-[140px]'>
                          <SelectValue />
                        </SelectTrigger>
                        <SelectContent>
                          {CONDITION_LOGICS.map((l) => (
                            <SelectItem key={l.value} value={l.value}>
                              {t(l.label)}
                            </SelectItem>
                          ))}
                        </SelectContent>
                      </Select>
                      <span className='text-muted-foreground text-xs'>
                        {t('systemSettings.fields.ofTheFollowingConditions')}
                      </span>
                    </div>

                    {/* 条件列表 */}
                    {(row.conditions ?? []).map((cond, ci) => (
                      <div
                        key={ci}
                        className='flex flex-wrap items-center gap-2 rounded border p-2'
                      >
                        <div className='flex items-center'>
                          <span className='text-muted-foreground mr-1 font-mono text-xs'>
                            {ci === 0
                              ? 'IF'
                              : (row.logic || 'AND') === 'AND'
                                ? 'AND'
                                : 'OR'}
                          </span>
                        </div>
                        <Input
                          value={cond.field}
                          placeholder='model / provider / quality / size'
                          className='h-7 min-w-[160px] flex-1'
                          onChange={(e) =>
                            updateCondition(
                              row.rowId,
                              ci,
                              'field',
                              e.target.value
                            )
                          }
                        />
                        <Select
                          value={cond.mode}
                          onValueChange={(v) =>
                            updateCondition(row.rowId, ci, 'mode', v ?? 'eq')
                          }
                        >
                          <SelectTrigger className='h-7 w-[150px]'>
                            <SelectValue />
                          </SelectTrigger>
                          <SelectContent>
                            {CONDITION_MODES.map((m) => (
                              <SelectItem key={m.value} value={m.value}>
                                {t(m.label)}
                              </SelectItem>
                            ))}
                          </SelectContent>
                        </Select>
                        <Input
                          value={String(cond.value ?? '')}
                          placeholder={t(
                            'systemSettings.placeholders.conditionValue'
                          )}
                          className='h-7 min-w-[120px] flex-1'
                          onChange={(e) =>
                            updateCondition(
                              row.rowId,
                              ci,
                              'value',
                              e.target.value
                            )
                          }
                        />
                        <label className='text-muted-foreground flex items-center gap-1 text-xs'>
                          <Switch
                            checked={cond.invert ?? false}
                            onCheckedChange={(checked) =>
                              updateCondition(row.rowId, ci, 'invert', checked)
                            }
                          />
                          {t('systemSettings.actions.invert')}
                        </label>
                        <Button
                          variant='ghost'
                          size='icon'
                          className='h-7'
                          onClick={() => removeCondition(row.rowId, ci)}
                          aria-label={t('common.actions.delete')}
                        >
                          <Trash2 className='text-destructive h-3.5 w-3.5' />
                        </Button>
                      </div>
                    ))}

                    <Button
                      variant='outline'
                      size='sm'
                      className='h-7'
                      onClick={() => addCondition(row.rowId)}
                    >
                      <Plus className='mr-1 h-3.5 w-3.5' />
                      {t('systemSettings.actions.addCondition')}
                    </Button>
                  </div>
                )}
              </div>
            ))
          )}
        </div>
      ) : (
        <div className='space-y-2'>
          <Textarea
            value={jsonText}
            onChange={(event) => handleJsonChange(event.target.value)}
            className='font-mono text-sm'
            rows={16}
            spellCheck={false}
          />
          {jsonError && <p className='text-destructive text-sm'>{jsonError}</p>}
        </div>
      )}

      <SettingsPageActionsPortal>
        <Button
          type='button'
          variant='destructive'
          size='sm'
          onClick={onReset}
          disabled={isResetting || updateOption.isPending}
        >
          {t('systemSettings.actions.resetToolBillingRules')}
        </Button>
        <Button
          type='button'
          size='sm'
          onClick={handleSave}
          disabled={
            updateOption.isPending ||
            isResetting ||
            (editMode === 'json' && !!jsonError)
          }
        >
          {updateOption.isPending
            ? t('channels.tips.saving')
            : t('systemSettings.actions.saveToolBillingRules')}
        </Button>
      </SettingsPageActionsPortal>
    </div>
  )
})
