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
import { useMemo, useRef, useState } from 'react'
import * as z from 'zod'
import { useForm } from 'react-hook-form'
import { zodResolver } from '@hookform/resolvers/zod'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { formatTimestampToDate } from '@/lib/format'
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
import { Form, FormControl, FormField } from '@/components/ui/form'
import { Switch } from '@/components/ui/switch'
import { DateTimePicker } from '@/components/datetime-picker'
import { cleanLogs } from '../api'
import { SettingsForm } from '../components/settings-form-layout'
import {
  SettingsChoiceGrid,
  SettingsControl,
  SettingsGroup,
  SettingsGroupSwitch,
  SettingsRow,
  SettingsUnitField,
  SettingsUnitInput,
  SettingsUnitSelect,
} from '../components/settings-group'
import { SettingsPageFormActions } from '../components/settings-page-context'
import { SettingsPanel } from '../components/settings-panel'
import { SettingsSection } from '../components/settings-section'
import { useResetForm } from '../hooks/use-reset-form'
import { useUpdateOption } from '../hooks/use-update-option'
import type { CleanLogsResult, UpdateOptionRequest } from '../types'

const HOURS_IN_DAY = 24

const cleanupUnitValues = ['year', 'month', 'week', 'day'] as const

export type CleanupUnit = (typeof cleanupUnitValues)[number]

const positiveIntegerMessage = 'systemSettings.errors.positiveIntegerRequired'

const logSettingsSchema = z.object({
  LogConsumeEnabled: z.boolean(),
  auto_clean_enabled: z.boolean(),
  retention_value: z.coerce
    .number()
    .int(positiveIntegerMessage)
    .min(1, positiveIntegerMessage),
  retention_unit: z.enum(cleanupUnitValues),
  interval_value: z.coerce
    .number()
    .int(positiveIntegerMessage)
    .min(1, positiveIntegerMessage),
  interval_unit: z.enum(cleanupUnitValues),
  targets: z.array(z.string()),
})

type LogSettingsFormInput = z.input<typeof logSettingsSchema>
type LogSettingsFormValues = z.output<typeof logSettingsSchema>

// 自动清理的基线快照，保存时按字段差异提交，避免覆盖其他管理员并发修改。
// 时间单位以宽松 string 传入，组件内部收窄到合法枚举。
type AutoCleanBaseline = {
  auto_clean_enabled: boolean
  retention_value: number
  retention_unit: string
  interval_value: number
  interval_unit: string
  targets: string[]
}

type AutoCleanDefaults = AutoCleanBaseline

type LogSettingsSectionProps = {
  defaultEnabled: boolean
  defaultAutoClean: AutoCleanDefaults
}

const toCleanupUnit = (value: string): CleanupUnit =>
  (cleanupUnitValues as readonly string[]).includes(value)
    ? (value as CleanupUnit)
    : 'year'

const getDateHoursAgo = (hours: number) => {
  const date = new Date()
  date.setHours(date.getHours() - hours)
  return date
}

const getDateDaysAgo = (days: number) => getDateHoursAgo(days * HOURS_IN_DAY)

export function LogSettingsSection({
  defaultEnabled,
  defaultAutoClean,
}: LogSettingsSectionProps) {
  const { t } = useTranslation()
  const updateOption = useUpdateOption()

  const formDefaults = useMemo<LogSettingsFormInput>(
    () => ({
      LogConsumeEnabled: defaultEnabled,
      auto_clean_enabled: defaultAutoClean.auto_clean_enabled,
      retention_value: defaultAutoClean.retention_value,
      retention_unit: toCleanupUnit(defaultAutoClean.retention_unit),
      interval_value: defaultAutoClean.interval_value,
      interval_unit: toCleanupUnit(defaultAutoClean.interval_unit),
      targets: defaultAutoClean.targets,
    }),
    [defaultEnabled, defaultAutoClean]
  )
  const baselineRef = useRef<AutoCleanBaseline>({ ...defaultAutoClean })

  const form = useForm<LogSettingsFormInput, unknown, LogSettingsFormValues>({
    resolver: zodResolver(logSettingsSchema),
    defaultValues: formDefaults,
  })

  useResetForm(form, formDefaults)

  // 手动清理默认仅勾选使用日志。
  const [manualTargets, setManualTargets] = useState<string[]>(['logs'])
  const [startDate, setStartDate] = useState<Date | undefined>(() =>
    getDateDaysAgo(30)
  )
  const [endDate, setEndDate] = useState<Date | undefined>(() =>
    getDateDaysAgo(0)
  )
  const [isCleaning, setIsCleaning] = useState(false)
  const [showConfirmDialog, setShowConfirmDialog] = useState(false)

  const startTimestamp = useMemo(() => {
    if (!startDate) return null
    return Math.floor(startDate.getTime() / 1000)
  }, [startDate])

  const endTimestamp = useMemo(() => {
    if (!endDate) return null
    return Math.floor(endDate.getTime() / 1000)
  }, [endDate])

  const formattedRange = useMemo(() => {
    const parts: string[] = []
    if (startTimestamp) {
      parts.push(formatTimestampToDate(startTimestamp * 1000, 'milliseconds'))
    }
    if (endTimestamp) {
      parts.push(formatTimestampToDate(endTimestamp * 1000, 'milliseconds'))
    }
    return parts.join(' ~ ')
  }, [startTimestamp, endTimestamp])

  const noTargetSelected = manualTargets.length === 0

  const autoCleanEnabled = form.watch('auto_clean_enabled')

  const selectedAutoTargets = form.watch('targets')

  const unitOptions: Array<{
    value: (typeof cleanupUnitValues)[number]
    label: string
  }> = [
    { value: 'year', label: t('systemSettings.fields.unitYear') },
    { value: 'month', label: t('systemSettings.fields.unitMonth') },
    { value: 'week', label: t('systemSettings.fields.unitWeek') },
    { value: 'day', label: t('systemSettings.fields.unitDay') },
  ]

  const onSubmit = async (values: LogSettingsFormValues) => {
    const updates: UpdateOptionRequest[] = []
    if (values.LogConsumeEnabled !== defaultEnabled) {
      updates.push({
        key: 'LogConsumeEnabled',
        value: values.LogConsumeEnabled,
      })
    }
    if (values.auto_clean_enabled !== baselineRef.current.auto_clean_enabled) {
      updates.push({
        key: 'log_cleanup_setting.auto_clean_enabled',
        value: values.auto_clean_enabled,
      })
    }
    if (values.retention_value !== baselineRef.current.retention_value) {
      updates.push({
        key: 'log_cleanup_setting.retention_value',
        value: values.retention_value,
      })
    }
    if (values.retention_unit !== baselineRef.current.retention_unit) {
      updates.push({
        key: 'log_cleanup_setting.retention_unit',
        value: values.retention_unit,
      })
    }
    if (values.interval_value !== baselineRef.current.interval_value) {
      updates.push({
        key: 'log_cleanup_setting.interval_value',
        value: values.interval_value,
      })
    }
    if (values.interval_unit !== baselineRef.current.interval_unit) {
      updates.push({
        key: 'log_cleanup_setting.interval_unit',
        value: values.interval_unit,
      })
    }
    const serializedTargets = JSON.stringify(values.targets)
    if (serializedTargets !== JSON.stringify(baselineRef.current.targets)) {
      updates.push({
        key: 'log_cleanup_setting.targets',
        value: serializedTargets,
      })
    }

    if (updates.length === 0) {
      toast.info(t('channels.fields.noChangesToSave'))
      return
    }

    for (const update of updates) {
      await updateOption.mutateAsync(update)
    }
    baselineRef.current = {
      auto_clean_enabled: values.auto_clean_enabled,
      retention_value: values.retention_value,
      retention_unit: values.retention_unit,
      interval_value: values.interval_value,
      interval_unit: values.interval_unit,
      targets: values.targets,
    }
    toast.success(t('systemSettings.status.savedSuccessfully'))
  }

  const handleRequestCleanLogs = () => {
    if (!endTimestamp) {
      toast.error(
        t('systemSettings.placeholders.selectAnEndTimeBeforeClearingLogs')
      )
      return
    }
    if (startTimestamp && endTimestamp && startTimestamp > endTimestamp) {
      toast.error(t('systemSettings.errors.startTimeMustNotBeLaterThanEndTime'))
      return
    }
    if (noTargetSelected) {
      toast.error(
        t('systemSettings.placeholders.selectAtLeastOneLogTypeToClean')
      )
      return
    }

    setShowConfirmDialog(true)
  }

  const handleCleanLogs = async () => {
    if (!endTimestamp) {
      toast.error(
        t('systemSettings.placeholders.selectAnEndTimeBeforeClearingLogs')
      )
      return
    }

    setIsCleaning(true)
    try {
      const res = await cleanLogs({
        start_timestamp: startTimestamp ?? undefined,
        end_timestamp: endTimestamp,
        clean_logs: manualTargets.includes('logs'),
        clean_quota_data: manualTargets.includes('quota_data'),
        clean_stored_images: manualTargets.includes('stored_images'),
        clean_stored_videos: manualTargets.includes('stored_videos'),
        clean_audit_logs: manualTargets.includes('audit_logs'),
      })
      if (!res.success) {
        throw new Error(
          res.message || t('systemSettings.errors.failedToCleanLogs')
        )
      }
      const summary = summarizeResult(res.data)
      if (summary.total > 0) {
        toast.success(
          t('systemSettings.status.countEntriesRemoved', {
            count: summary.total,
          }) +
            ' ' +
            summary.detail
        )
      } else {
        toast.success(
          t('systemSettings.tips.noLogEntriesMatchedTheSelectedRange')
        )
      }
    } catch (error) {
      const message =
        error instanceof Error
          ? error.message
          : t('systemSettings.errors.failedToCleanLogs')
      toast.error(message)
    } finally {
      setIsCleaning(false)
    }
  }

  const summarizeResult = (result?: CleanLogsResult) => {
    const parts: string[] = []
    let total = 0
    if (result?.logs !== undefined) {
      parts.push(`${t('systemSettings.titles.usageLogs')}: ${result.logs}`)
      total += result.logs
    }
    if (result?.quota_data !== undefined) {
      parts.push(
        `${t('systemSettings.fields.quotaDataStats')}: ${result.quota_data}`
      )
      total += result.quota_data
    }
    if (result?.stored_images !== undefined) {
      parts.push(
        `${t('systemSettings.fields.storedImages')}: ${result.stored_images}`
      )
      total += result.stored_images
    }
    if (result?.stored_videos !== undefined) {
      parts.push(
        `${t('systemSettings.fields.storedVideos')}: ${result.stored_videos}`
      )
      total += result.stored_videos
    }
    if (result?.audit_logs !== undefined) {
      parts.push(
        `${t('systemSettings.titles.auditLogs')}: ${result.audit_logs}`
      )
      total += result.audit_logs
    }
    return { total, detail: parts.join(', ') }
  }

  const targetOptions: Array<{
    value: string
    label: string
    description: string
  }> = [
    {
      value: 'logs',
      label: t('systemSettings.titles.usageLogs'),
      description: t(
        'systemSettings.tips.consumeErrorTopupManageAndSystemLogs'
      ),
    },
    {
      value: 'quota_data',
      label: t('systemSettings.fields.quotaDataStats'),
      description: t('systemSettings.tips.quotaDataStatsDesc'),
    },
    {
      value: 'stored_images',
      label: t('systemSettings.fields.storedImages'),
      description: t(
        'systemSettings.tips.imageBytesCachedForTheImageToUrlConversion'
      ),
    },
    {
      value: 'stored_videos',
      label: t('systemSettings.fields.storedVideos'),
      description: t(
        'systemSettings.tips.videoBytesCachedForTheVideoToUrlConversion'
      ),
    },
    {
      value: 'audit_logs',
      label: t('systemSettings.titles.auditLogs'),
      description: t(
        'systemSettings.errors.administrativeOperationRecordsDeletionCannotBeUndone'
      ),
    },
  ]

  return (
    <SettingsSection title={t('systemSettings.fields.logMaintenance')}>
      <Form {...form}>
        <SettingsForm onSubmit={form.handleSubmit(onSubmit)}>
          <SettingsPageFormActions
            onSave={form.handleSubmit(onSubmit)}
            isSaving={updateOption.isPending}
            saveLabel='common.actions.saveLogSettings'
          />

          <SettingsGroup>
            <FormField
              control={form.control}
              name='LogConsumeEnabled'
              render={({ field }) => (
                <SettingsRow
                  label={t('systemSettings.fields.recordQuotaUsage')}
                  description={t(
                    'systemSettings.tips.trackPerRequestConsumptionToPowerUsageAnalyticsKeeping'
                  )}
                >
                  <FormControl>
                    <Switch
                      checked={field.value}
                      onCheckedChange={field.onChange}
                    />
                  </FormControl>
                </SettingsRow>
              )}
            />
          </SettingsGroup>

          <SettingsGroup
            title={t('systemSettings.titles.autoLogCleanup')}
            description={t('systemSettings.tips.autoLogCleanupIntro')}
            control={<SettingsGroupSwitch name='auto_clean_enabled' />}
            enabled={autoCleanEnabled}
          >
            <FormField
              control={form.control}
              name='retention_value'
              render={({ field }) => (
                <SettingsRow
                  label={t('systemSettings.fields.retentionPeriod')}
                  description={t('systemSettings.tips.autoCleanEnabledDesc')}
                >
                  <SettingsControl size='sm'>
                    <SettingsUnitField
                      input={
                        <FormControl>
                          <SettingsUnitInput
                            type='number'
                            min={1}
                            step={1}
                            value={
                              typeof field.value === 'number' &&
                              Number.isFinite(field.value)
                                ? field.value
                                : ''
                            }
                            onChange={(event) =>
                              field.onChange(event.target.valueAsNumber)
                            }
                            name={field.name}
                            onBlur={field.onBlur}
                            ref={field.ref}
                          />
                        </FormControl>
                      }
                      select={
                        <FormField
                          control={form.control}
                          name='retention_unit'
                          render={({ field: unitField }) => (
                            <SettingsUnitSelect
                              aria-label={t(
                                'systemSettings.fields.retentionPeriod'
                              )}
                              value={unitField.value}
                              onValueChange={unitField.onChange}
                              options={unitOptions}
                            />
                          )}
                        />
                      }
                    />
                  </SettingsControl>
                </SettingsRow>
              )}
            />

            <FormField
              control={form.control}
              name='interval_value'
              render={({ field }) => (
                <SettingsRow label={t('systemSettings.fields.cleanupInterval')}>
                  <SettingsControl size='sm'>
                    <SettingsUnitField
                      input={
                        <FormControl>
                          <SettingsUnitInput
                            type='number'
                            min={1}
                            step={1}
                            value={
                              typeof field.value === 'number' &&
                              Number.isFinite(field.value)
                                ? field.value
                                : ''
                            }
                            onChange={(event) =>
                              field.onChange(event.target.valueAsNumber)
                            }
                            name={field.name}
                            onBlur={field.onBlur}
                            ref={field.ref}
                          />
                        </FormControl>
                      }
                      select={
                        <FormField
                          control={form.control}
                          name='interval_unit'
                          render={({ field: unitField }) => (
                            <SettingsUnitSelect
                              aria-label={t(
                                'systemSettings.fields.cleanupInterval'
                              )}
                              value={unitField.value}
                              onValueChange={unitField.onChange}
                              options={unitOptions}
                            />
                          )}
                        />
                      }
                    />
                  </SettingsControl>
                </SettingsRow>
              )}
            />

            <SettingsRow label={t('systemSettings.fields.autoCleanTargets')}>
              <SettingsChoiceGrid
                aria-label={t('systemSettings.fields.autoCleanTargets')}
                options={targetOptions}
                value={selectedAutoTargets}
                onValueChange={(next) =>
                  form.setValue('targets', next, { shouldDirty: true })
                }
              />
            </SettingsRow>
          </SettingsGroup>

          <SettingsPanel
            title={t('systemSettings.actions.cleanHistoryLogs')}
            description={t(
              'systemSettings.placeholders.selectLogTypesAndTimeRangeToRemoveMatching'
            )}
            divided
            footer={
              <Button
                type='button'
                variant='destructive'
                onClick={handleRequestCleanLogs}
                disabled={isCleaning || noTargetSelected}
              >
                {isCleaning
                  ? t('systemSettings.tips.cleaning')
                  : t('systemSettings.actions.cleanLogs')}
              </Button>
            }
          >
            <SettingsRow label={t('systemSettings.fields.logTypes')}>
              <SettingsChoiceGrid
                aria-label={t('systemSettings.fields.logTypes')}
                options={targetOptions}
                value={manualTargets}
                onValueChange={setManualTargets}
              />
            </SettingsRow>
            <SettingsRow label={t('systemSettings.fields.timeRange')}>
              <div className='flex flex-col gap-2'>
                <div className='grid gap-4 sm:grid-cols-2'>
                  <div className='flex flex-col gap-1.5'>
                    <span className='text-muted-foreground text-xs leading-snug'>
                      {t('subscriptions.actions.start')}
                    </span>
                    <DateTimePicker
                      value={startDate}
                      onChange={setStartDate}
                      placeholder={t('systemSettings.fields.noLowerBound')}
                    />
                  </div>
                  <div className='flex flex-col gap-1.5'>
                    <span className='text-muted-foreground text-xs leading-snug'>
                      {t('subscriptions.fields.end')}
                    </span>
                    <DateTimePicker
                      value={endDate}
                      onChange={setEndDate}
                      placeholder={t(
                        'systemSettings.placeholders.requiredField'
                      )}
                    />
                  </div>
                </div>
                <div className='flex flex-wrap gap-1'>
                  <Button
                    type='button'
                    variant='outline'
                    size='xs'
                    onClick={() => {
                      setStartDate(getDateDaysAgo(7))
                      setEndDate(getDateDaysAgo(0))
                    }}
                  >
                    {t('systemSettings.fields.last7Days')}
                  </Button>
                  <Button
                    type='button'
                    variant='outline'
                    size='xs'
                    onClick={() => {
                      setStartDate(getDateDaysAgo(30))
                      setEndDate(getDateDaysAgo(0))
                    }}
                  >
                    {t('systemSettings.fields.last30Days')}
                  </Button>
                </div>
              </div>
            </SettingsRow>
          </SettingsPanel>
        </SettingsForm>
      </Form>
      <AlertDialog open={showConfirmDialog} onOpenChange={setShowConfirmDialog}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>
              {t('systemSettings.actions.confirmLogCleanup')}
            </AlertDialogTitle>
            <AlertDialogDescription>
              {formattedRange
                ? t(
                    'systemSettings.status.permanentlyRemoveLogEntriesCreatedInTheRangeRange',
                    { range: formattedRange }
                  )
                : t(
                    'systemSettings.tips.permanentlyRemoveLogEntriesMatchingTheSelectedRange'
                  )}{' '}
              {t('keys.errors.actionCannotBeUndone951f49')}
              {manualTargets.includes('audit_logs') && (
                <span className='text-destructive mt-2 block font-medium'>
                  {t(
                    'systemSettings.status.auditLogsSelectedAdministrativeOperationRecordsWillBePermanently'
                  )}
                </span>
              )}
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel disabled={isCleaning}>
              {t('common.actions.cancel')}
            </AlertDialogCancel>
            <AlertDialogAction onClick={handleCleanLogs} disabled={isCleaning}>
              {isCleaning
                ? t('systemSettings.tips.cleaning')
                : t('systemSettings.actions.deleteLogs')}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </SettingsSection>
  )
}
