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
import { Plus, RefreshCw } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { useAuthStore } from '@/stores/auth-store'
import { ROLE } from '@/lib/roles'
import {
  appTableFeatures,
  type ColumnDef,
  useTable,
} from '@/lib/tanstack-table'
import { cn } from '@/lib/utils'
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
import { Checkbox } from '@/components/ui/checkbox'
import {
  Dialog,
  DialogContent,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import {
  Select,
  SelectContent,
  SelectGroup,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { Switch } from '@/components/ui/switch'
import { DataTablePage } from '@/components/data-table'
import { SectionPageLayout } from '@/components/layout'
import { getGroups } from '@/features/users/api'
import {
  createDynamicRatioRule,
  deleteDynamicRatioRule,
  getDynamicRatioRules,
  getDynamicRatioStatus,
  reorderDynamicRatioRules,
  setDynamicRatioEnabled,
  updateDynamicRatioRule,
} from './api'
import { useDynamicRatioColumns } from './components/dynamic-ratio-columns'
import type { DynamicRatioRule, DynamicRatioRulePayload } from './types'

type RuleFormState = {
  group: string
  models: string
  concurrency: string
  weekdays: number[]
  start_time: string
  end_time: string
  ratio: string
  priority: string
  enable: boolean
}

const DEFAULT_FORM: RuleFormState = {
  group: '',
  models: '',
  concurrency: '',
  weekdays: [],
  start_time: '',
  end_time: '',
  ratio: '1.5',
  priority: '0',
  enable: true,
}

const WEEKDAYS = [
  { value: 0, label: 'Sun' },
  { value: 1, label: 'Mon' },
  { value: 2, label: 'Tue' },
  { value: 3, label: 'Wed' },
  { value: 4, label: 'Thu' },
  { value: 5, label: 'Fri' },
  { value: 6, label: 'Sat' },
]

const queryKeys = {
  rules: ['dynamic-ratio', 'rules'] as const,
  status: ['dynamic-ratio', 'status'] as const,
  groups: ['dynamic-ratio', 'groups'] as const,
}

function ruleToForm(rule: DynamicRatioRule | null): RuleFormState {
  if (!rule) return DEFAULT_FORM

  let weekdays: number[] = []
  if (rule.weekdays) {
    const parsed = JSON.parse(rule.weekdays) as unknown
    if (!Array.isArray(parsed)) {
      throw new Error('Invalid weekday data')
    }
    weekdays = parsed.map((day: unknown) => Number(day))
  }

  let models = ''
  if (rule.models) {
    try {
      const parsed = JSON.parse(rule.models) as unknown
      if (Array.isArray(parsed) && parsed.length > 0) {
        models = parsed.join(', ')
      }
    } catch {
      models = rule.models
    }
  }

  return {
    group: rule.group,
    models,
    concurrency: rule.concurrency == null ? '' : String(rule.concurrency),
    weekdays,
    start_time: rule.start_time || '',
    end_time: rule.end_time || '',
    ratio: String(rule.ratio),
    priority: String(rule.priority ?? 0),
    enable: rule.enable !== false,
  }
}

function buildPayload(form: RuleFormState): DynamicRatioRulePayload {
  const group = form.group.trim()
  if (!group) throw new Error('Please select a group')

  const ratio = Number(form.ratio)
  if (!Number.isFinite(ratio) || ratio <= 0) {
    throw new Error('Ratio must be greater than 0')
  }

  const modelsText = form.models.trim()
  let models = ''
  if (modelsText) {
    const modelList = modelsText
      .split(',')
      .map((m) => m.trim())
      .filter((m) => m.length > 0)
    if (modelList.length > 0) {
      models = JSON.stringify(modelList)
    }
  }

  const concurrencyText = form.concurrency.trim()
  const concurrency =
    concurrencyText === '' ? null : Number.parseInt(concurrencyText, 10)
  if (
    concurrency !== null &&
    (!Number.isFinite(concurrency) || concurrency <= 0)
  ) {
    throw new Error('Concurrency threshold must be greater than 0')
  }

  const startTime = form.start_time.trim()
  const endTime = form.end_time.trim()
  if ((startTime && !endTime) || (!startTime && endTime)) {
    throw new Error('Start time and end time must be set together')
  }

  const priorityText = form.priority.trim()
  const priority = priorityText === '' ? 0 : Number.parseInt(priorityText, 10)
  if (!Number.isFinite(priority)) {
    throw new Error('Priority must be a number')
  }

  return {
    group,
    models,
    concurrency,
    weekdays: form.weekdays.length > 0 ? JSON.stringify(form.weekdays) : '',
    start_time: startTime,
    end_time: endTime,
    ratio,
    priority,
    enable: form.enable,
  }
}

export function DynamicRatio() {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const authUser = useAuthStore((state) => state.auth.user)
  const canEdit = (authUser?.role ?? 0) >= ROLE.SUPER_ADMIN
  const [dialogOpen, setDialogOpen] = useState(false)
  const [editingRule, setEditingRule] = useState<DynamicRatioRule | null>(null)
  const [deleteTarget, setDeleteTarget] = useState<DynamicRatioRule | null>(
    null
  )
  const [form, setForm] = useState<RuleFormState>(DEFAULT_FORM)

  const rulesQuery = useQuery({
    queryKey: queryKeys.rules,
    queryFn: getDynamicRatioRules,
  })

  const statusQuery = useQuery({
    queryKey: queryKeys.status,
    queryFn: getDynamicRatioStatus,
  })

  const groupsQuery = useQuery({
    queryKey: queryKeys.groups,
    queryFn: async () => {
      const res = await getGroups()
      if (!res.success) throw new Error(res.message || 'Failed to load groups')
      if (!res.data) throw new Error('Failed to load groups')
      return res.data
    },
  })

  const rules = rulesQuery.data ?? []
  const groups = groupsQuery.data ?? []
  const globalEnabled = Boolean(statusQuery.data?.enabled)

  const refreshAll = async () => {
    await Promise.all([
      queryClient.invalidateQueries({ queryKey: queryKeys.rules }),
      queryClient.invalidateQueries({ queryKey: queryKeys.status }),
    ])
  }

  const setEnabledMutation = useMutation({
    mutationFn: setDynamicRatioEnabled,
    onSuccess: async (_, enabled) => {
      toast.success(
        enabled
          ? t('dynamicRatio.status.ratioHasBeenEnabled')
          : t('dynamicRatio.status.ratioHasBeenDisabled')
      )
      await queryClient.invalidateQueries({ queryKey: queryKeys.status })
    },
    onError: (error) => toast.error(error.message),
  })

  const saveDialogRuleMutation = useMutation({
    mutationFn: (payload: DynamicRatioRulePayload) =>
      editingRule
        ? updateDynamicRatioRule({ ...payload, id: editingRule.id })
        : createDynamicRatioRule(payload),
    onSuccess: async () => {
      toast.success(
        editingRule
          ? t('dynamicRatio.status.updatedSuccessfully')
          : t('dynamicRatio.status.created')
      )
      setDialogOpen(false)
      setEditingRule(null)
      setForm(DEFAULT_FORM)
      await refreshAll()
    },
    onError: (error) => toast.error(error.message),
  })

  const updateRuleMutation = useMutation({
    mutationFn: updateDynamicRatioRule,
    onSuccess: async () => {
      toast.success(t('dynamicRatio.status.updatedSuccessfully'))
      await refreshAll()
    },
    onError: (error) => toast.error(error.message),
  })

  const deleteMutation = useMutation({
    mutationFn: deleteDynamicRatioRule,
    onSuccess: async () => {
      toast.success(t('dynamicRatio.status.deletedSuccessfully'))
      setDeleteTarget(null)
      await refreshAll()
    },
    onError: (error) => toast.error(error.message),
  })

  const reorderMutation = useMutation({
    mutationFn: reorderDynamicRatioRules,
    onSuccess: async () => {
      toast.success(t('dynamicRatio.status.orderUpdated'))
      await refreshAll()
    },
    onError: (error) => toast.error(error.message),
  })

  const openCreateDialog = () => {
    setEditingRule(null)
    setForm(DEFAULT_FORM)
    setDialogOpen(true)
  }

  const openEditDialog = (rule: DynamicRatioRule) => {
    try {
      setEditingRule(rule)
      setForm(ruleToForm(rule))
      setDialogOpen(true)
    } catch (error) {
      toast.error(
        error instanceof Error
          ? error.message
          : t('dynamicRatio.errors.invalidRule')
      )
    }
  }

  const handleSubmit = () => {
    try {
      saveDialogRuleMutation.mutate(buildPayload(form))
    } catch (error) {
      toast.error(
        error instanceof Error
          ? error.message
          : t('dynamicRatio.errors.invalidForm')
      )
    }
  }

  const handleMove = (index: number, direction: -1 | 1) => {
    const nextIndex = index + direction
    if (nextIndex < 0 || nextIndex >= rules.length) return
    const reordered = [...rules]
    const current = reordered[index]
    reordered[index] = reordered[nextIndex]
    reordered[nextIndex] = current
    reorderMutation.mutate(reordered.map((rule) => rule.id))
  }

  const toggleWeekday = (day: number, checked: boolean) => {
    setForm((current) => ({
      ...current,
      weekdays: checked
        ? [...current.weekdays, day].sort((a, b) => a - b)
        : current.weekdays.filter((item) => item !== day),
    }))
  }

  const error = rulesQuery.error || statusQuery.error || groupsQuery.error
  const errorMessage = error instanceof Error ? error.message : null

  const { mutate: updateRuleMutate } = updateRuleMutation
  const { mutate: reorderMutate } = reorderMutation

  const columnActions = useMemo(
    () => ({
      canEdit,
      onToggleEnable: (rule: DynamicRatioRule, enable: boolean) =>
        updateRuleMutate({ ...rule, enable }),
      onMoveUp: (index: number) => handleMove(index, -1),
      onMoveDown: (index: number) => handleMove(index, 1),
      onEdit: (rule: DynamicRatioRule) => openEditDialog(rule),
      onDelete: (rule: DynamicRatioRule) => setDeleteTarget(rule),
      rulesCount: rules.length,
    }),
    // eslint-disable-next-line react-hooks/exhaustive-deps
    [canEdit, rules.length, updateRuleMutate, reorderMutate]
  )

  const columns = useDynamicRatioColumns(
    columnActions
  ) as ColumnDef<DynamicRatioRule>[]

  const table = useTable({
    features: appTableFeatures,
    data: rules,
    columns,
    state: {},
  })

  return (
    <>
      <SectionPageLayout>
        <SectionPageLayout.Title>
          {t('dynamicRatio.fields.ratio')}
        </SectionPageLayout.Title>
        <SectionPageLayout.Actions>
          <div className='flex items-center gap-2 rounded-lg border px-2.5 py-1.5'>
            <span className='text-muted-foreground text-sm'>
              {t('dynamicRatio.fields.global')}
            </span>
            <Switch
              checked={globalEnabled}
              disabled={!canEdit || statusQuery.isLoading}
              onCheckedChange={(checked) => setEnabledMutation.mutate(checked)}
            />
          </div>
          <Button variant='outline' onClick={() => void refreshAll()}>
            <RefreshCw />
            {t('channels.actions.refresh')}
          </Button>
          <Button onClick={openCreateDialog} disabled={!canEdit}>
            <Plus />
            {t('dynamicRatio.fields.newRule')}
          </Button>
        </SectionPageLayout.Actions>
        <SectionPageLayout.Content>
          {errorMessage && (
            <div className='border-destructive/40 text-destructive mb-2.5 rounded-lg border px-3 py-2 text-sm'>
              {errorMessage}
            </div>
          )}

          <DataTablePage
            table={table}
            columns={columns}
            isLoading={rulesQuery.isLoading}
            showPagination={false}
            hideMobile
            tableClassName='overflow-x-auto'
            tableHeaderClassName='bg-muted/30 sticky top-0 z-10'
            afterTable={
              <div className='grid gap-3 sm:grid-cols-3'>
                <div>
                  <div className='text-muted-foreground text-xs'>
                    {t('channels.fields.status')}
                  </div>
                  <div className='mt-1 text-sm font-medium'>
                    {globalEnabled
                      ? t('channels.status.enabled')
                      : t('channels.status.disabled')}
                  </div>
                </div>
                <div>
                  <div className='text-muted-foreground text-xs'>
                    {t('dynamicRatio.status.activeRatio')}
                  </div>
                  <div className='mt-1 text-sm font-medium'>
                    {statusQuery.data?.active_ratio ?? '-'}x
                  </div>
                </div>
                <div>
                  <div className='text-muted-foreground text-xs'>
                    {t('dynamicRatio.fields.timezone')}
                  </div>
                  <div className='mt-1 text-sm font-medium'>
                    {statusQuery.data?.timezone || '-'}
                  </div>
                </div>
              </div>
            }
          />
        </SectionPageLayout.Content>
      </SectionPageLayout>

      <Dialog open={dialogOpen} onOpenChange={setDialogOpen}>
        <DialogContent className='sm:max-w-xl'>
          <DialogHeader>
            <DialogTitle>
              {editingRule
                ? t('dynamicRatio.actions.editRule')
                : t('dynamicRatio.fields.newRule')}
            </DialogTitle>
          </DialogHeader>
          <div className='grid gap-4'>
            <div className='grid gap-1.5'>
              <Label htmlFor='dynamic-ratio-group'>
                {t('common.fields.group')}
              </Label>
              <Select
                value={form.group || null}
                onValueChange={(value) =>
                  setForm((current) => ({
                    ...current,
                    group: value ?? '',
                  }))
                }
              >
                <SelectTrigger id='dynamic-ratio-group' className='w-full'>
                  <SelectValue
                    placeholder={t('dynamicRatio.placeholders.selectAGroup')}
                  />
                </SelectTrigger>
                <SelectContent alignItemWithTrigger={false}>
                  <SelectGroup>
                    {groups.map((group) => (
                      <SelectItem key={group} value={group}>
                        {group}
                      </SelectItem>
                    ))}
                  </SelectGroup>
                </SelectContent>
              </Select>
            </div>

            <div className='grid gap-1.5'>
              <Label htmlFor='dynamic-ratio-models'>
                {t('channels.titles.models')}
              </Label>
              <Input
                id='dynamic-ratio-models'
                value={form.models}
                placeholder={t('dynamicRatio.tips.eGGpt4Claude3OpusEmptyAll')}
                onChange={(event) =>
                  setForm((current) => ({
                    ...current,
                    models: event.target.value,
                  }))
                }
              />
              <p className='text-muted-foreground text-xs'>
                {t(
                  'dynamicRatio.tips.commaSeparatedSupportsWildcardLeaveEmptyForAllModels'
                )}
              </p>
            </div>

            <div className='grid gap-3 sm:grid-cols-2'>
              <div className='grid gap-1.5'>
                <Label htmlFor='dynamic-ratio-concurrency'>
                  {t('dynamicRatio.fields.concurrencyThreshold')}
                </Label>
                <Input
                  id='dynamic-ratio-concurrency'
                  type='number'
                  min='1'
                  value={form.concurrency}
                  placeholder={t('dynamicRatio.fields.any')}
                  onChange={(event) =>
                    setForm((current) => ({
                      ...current,
                      concurrency: event.target.value,
                    }))
                  }
                />
              </div>
              <div className='grid gap-1.5'>
                <Label htmlFor='dynamic-ratio-ratio'>
                  {t('dynamicRatio.fields.ratio794f65')}
                </Label>
                <Input
                  id='dynamic-ratio-ratio'
                  type='number'
                  min='0.01'
                  step='0.1'
                  value={form.ratio}
                  onChange={(event) =>
                    setForm((current) => ({
                      ...current,
                      ratio: event.target.value,
                    }))
                  }
                />
              </div>
            </div>

            <div className='grid gap-3 sm:grid-cols-2'>
              <div className='grid gap-1.5'>
                <Label htmlFor='dynamic-ratio-start-time'>
                  {t('dashboard.actions.startTime')}
                </Label>
                <Input
                  id='dynamic-ratio-start-time'
                  value={form.start_time}
                  placeholder='HH:MM'
                  onChange={(event) =>
                    setForm((current) => ({
                      ...current,
                      start_time: event.target.value,
                    }))
                  }
                />
              </div>
              <div className='grid gap-1.5'>
                <Label htmlFor='dynamic-ratio-end-time'>
                  {t('dashboard.fields.endTime')}
                </Label>
                <Input
                  id='dynamic-ratio-end-time'
                  value={form.end_time}
                  placeholder='HH:MM'
                  onChange={(event) =>
                    setForm((current) => ({
                      ...current,
                      end_time: event.target.value,
                    }))
                  }
                />
              </div>
            </div>

            <div className='grid gap-2'>
              <Label>{t('dynamicRatio.fields.weekdays')}</Label>
              <div className='grid grid-cols-2 gap-2 sm:grid-cols-4'>
                {WEEKDAYS.map((weekday) => {
                  const checked = form.weekdays.includes(weekday.value)
                  return (
                    <label
                      key={weekday.value}
                      className={cn(
                        'flex items-center gap-2 rounded-lg border px-2.5 py-2 text-sm',
                        checked && 'border-primary/60 bg-primary/5'
                      )}
                    >
                      <Checkbox
                        checked={checked}
                        onCheckedChange={(value) =>
                          toggleWeekday(weekday.value, value === true)
                        }
                      />
                      {t(weekday.label)}
                    </label>
                  )
                })}
              </div>
            </div>

            <div className='grid gap-3 sm:grid-cols-2'>
              <div className='grid gap-1.5'>
                <Label htmlFor='dynamic-ratio-priority'>
                  {t('channels.fields.priority')}
                </Label>
                <Input
                  id='dynamic-ratio-priority'
                  type='number'
                  value={form.priority}
                  onChange={(event) =>
                    setForm((current) => ({
                      ...current,
                      priority: event.target.value,
                    }))
                  }
                />
              </div>
              <div className='flex items-center justify-between rounded-lg border px-3 py-2'>
                <div>
                  <Label>{t('channels.status.enabled')}</Label>
                  <p className='text-muted-foreground text-xs'>
                    {t('dynamicRatio.actions.enableThisRuleAfterSaving')}
                  </p>
                </div>
                <Switch
                  checked={form.enable}
                  onCheckedChange={(checked) =>
                    setForm((current) => ({ ...current, enable: checked }))
                  }
                />
              </div>
            </div>
          </div>
          <DialogFooter>
            <Button variant='outline' onClick={() => setDialogOpen(false)}>
              {t('common.actions.cancel')}
            </Button>
            <Button
              onClick={handleSubmit}
              disabled={saveDialogRuleMutation.isPending}
            >
              {editingRule
                ? t('channels.actions.saveChanges')
                : t('channels.actions.create')}
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
              {t('dynamicRatio.actions.deleteRule')}
            </AlertDialogTitle>
            <AlertDialogDescription>
              {t('dynamicRatio.status.ratioRuleWillBeDeleted')}
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel>{t('common.actions.cancel')}</AlertDialogCancel>
            <AlertDialogAction
              variant='destructive'
              disabled={deleteMutation.isPending}
              onClick={() => {
                if (deleteTarget) deleteMutation.mutate(deleteTarget.id)
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
