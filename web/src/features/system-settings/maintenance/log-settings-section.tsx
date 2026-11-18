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
import { Checkbox } from '@/components/ui/checkbox'
import {
  Form,
  FormControl,
  FormDescription,
  FormField,
  FormItem,
  FormLabel,
  FormMessage,
} from '@/components/ui/form'
import { Input } from '@/components/ui/input'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { Switch } from '@/components/ui/switch'
import { DateTimePicker } from '@/components/datetime-picker'
import { cleanLogs } from '../api'
import {
  SettingsControlGroup,
  SettingsForm,
  SettingsSwitchContent,
  SettingsSwitchItem,
} from '../components/settings-form-layout'
import { SettingsPageFormActions } from '../components/settings-page-context'
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

type CleanTargets = {
  cleanLogs: boolean
  cleanQuotaData: boolean
  cleanStoredImages: boolean
  cleanStoredVideos: boolean
  cleanAuditLogs: boolean
}

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

  // 默认仅勾选使用日志
  const [targets, setTargets] = useState<CleanTargets>({
    cleanLogs: true,
    cleanQuotaData: false,
    cleanStoredImages: false,
    cleanStoredVideos: false,
    cleanAuditLogs: false,
  })
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

  const noTargetSelected =
    !targets.cleanLogs &&
    !targets.cleanQuotaData &&
    !targets.cleanStoredImages &&
    !targets.cleanStoredVideos &&
    !targets.cleanAuditLogs

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
        clean_logs: targets.cleanLogs,
        clean_quota_data: targets.cleanQuotaData,
        clean_stored_images: targets.cleanStoredImages,
        clean_stored_videos: targets.cleanStoredVideos,
        clean_audit_logs: targets.cleanAuditLogs,
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
    key: keyof CleanTargets
    target: string
    label: string
    description: string
  }> = [
    {
      key: 'cleanLogs',
      target: 'logs',
      label: t('systemSettings.titles.usageLogs'),
      description: t(
        'systemSettings.tips.consumeErrorTopupManageAndSystemLogs'
      ),
    },
    {
      key: 'cleanQuotaData',
      target: 'quota_data',
      label: t('systemSettings.fields.quotaDataStats'),
      description: t('systemSettings.tips.quotaDataStatsDesc'),
    },
    {
      key: 'cleanStoredImages',
      target: 'stored_images',
      label: t('systemSettings.fields.storedImages'),
      description: t(
        'systemSettings.tips.imageBytesCachedForTheImageToUrlConversion'
      ),
    },
    {
      key: 'cleanStoredVideos',
      target: 'stored_videos',
      label: t('systemSettings.fields.storedVideos'),
      description: t(
        'systemSettings.tips.videoBytesCachedForTheVideoToUrlConversion'
      ),
    },
    {
      key: 'cleanAuditLogs',
      target: 'audit_logs',
      label: t('systemSettings.titles.auditLogs'),
      description: t(
        'systemSettings.errors.administrativeOperationRecordsDeletionCannotBeUndone'
      ),
    },
  ]

  const toggleAutoTarget = (target: string, checked: boolean) => {
    const current = form.getValues('targets')
    const next = checked
      ? [...current, target]
      : current.filter((item) => item !== target)
    form.setValue('targets', next, { shouldDirty: true })
  }

  return (
    <SettingsSection title={t('systemSettings.fields.logMaintenance')}>
      <Form {...form}>
        <SettingsForm onSubmit={form.handleSubmit(onSubmit)}>
          <SettingsPageFormActions
            onSave={form.handleSubmit(onSubmit)}
            isSaving={updateOption.isPending}
            saveLabel='common.actions.saveLogSettings'
          />
          <FormField
            control={form.control}
            name='LogConsumeEnabled'
            render={({ field }) => (
              <SettingsSwitchItem>
                <SettingsSwitchContent>
                  <FormLabel>
                    {t('systemSettings.fields.recordQuotaUsage')}
                  </FormLabel>
                  <FormDescription>
                    {t(
                      'systemSettings.tips.trackPerRequestConsumptionToPowerUsageAnalyticsKeeping'
                    )}
                  </FormDescription>
                </SettingsSwitchContent>
                <FormControl>
                  <Switch
                    checked={field.value}
                    onCheckedChange={field.onChange}
                  />
                </FormControl>
                <FormMessage />
              </SettingsSwitchItem>
            )}
          />

          <SettingsControlGroup className='space-y-3'>
            <div>
              <h4 className='text-sm font-medium'>
                {t('systemSettings.titles.autoLogCleanup')}
              </h4>
              <p className='text-muted-foreground text-sm'>
                {t('systemSettings.tips.autoLogCleanupIntro')}
              </p>
            </div>

            <FormField
              control={form.control}
              name='auto_clean_enabled'
              render={({ field }) => (
                <SettingsSwitchItem>
                  <SettingsSwitchContent>
                    <FormLabel>
                      {t('systemSettings.fields.autoCleanEnabled')}
                    </FormLabel>
                    <FormDescription>
                      {t('systemSettings.tips.autoCleanEnabledDesc')}
                    </FormDescription>
                  </SettingsSwitchContent>
                  <FormControl>
                    <Switch
                      checked={field.value}
                      onCheckedChange={field.onChange}
                    />
                  </FormControl>
                  <FormMessage />
                </SettingsSwitchItem>
              )}
            />

            <div className='grid gap-3 sm:grid-cols-2'>
              <FormField
                control={form.control}
                name='retention_value'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>
                      {t('systemSettings.fields.retentionPeriod')}
                    </FormLabel>
                    <div className='flex gap-2'>
                      <FormControl>
                        <Input
                          type='number'
                          min={1}
                          step={1}
                          disabled={!autoCleanEnabled}
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
                      <FormField
                        control={form.control}
                        name='retention_unit'
                        render={({ field: unitField }) => (
                          <FormItem className='w-28 shrink-0'>
                            <Select
                              value={unitField.value}
                              onValueChange={(value) =>
                                unitField.onChange(value)
                              }
                              disabled={!autoCleanEnabled}
                            >
                              <SelectTrigger className='w-full'>
                                <SelectValue />
                              </SelectTrigger>
                              <SelectContent alignItemWithTrigger={false}>
                                {unitOptions.map((option) => (
                                  <SelectItem
                                    key={option.value}
                                    value={option.value}
                                  >
                                    {option.label}
                                  </SelectItem>
                                ))}
                              </SelectContent>
                            </Select>
                          </FormItem>
                        )}
                      />
                    </div>
                    <FormMessage />
                  </FormItem>
                )}
              />
              <FormField
                control={form.control}
                name='interval_value'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>
                      {t('systemSettings.fields.cleanupInterval')}
                    </FormLabel>
                    <div className='flex gap-2'>
                      <FormControl>
                        <Input
                          type='number'
                          min={1}
                          step={1}
                          disabled={!autoCleanEnabled}
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
                      <FormField
                        control={form.control}
                        name='interval_unit'
                        render={({ field: unitField }) => (
                          <FormItem className='w-28 shrink-0'>
                            <Select
                              value={unitField.value}
                              onValueChange={(value) =>
                                unitField.onChange(value)
                              }
                              disabled={!autoCleanEnabled}
                            >
                              <SelectTrigger className='w-full'>
                                <SelectValue />
                              </SelectTrigger>
                              <SelectContent alignItemWithTrigger={false}>
                                {unitOptions.map((option) => (
                                  <SelectItem
                                    key={option.value}
                                    value={option.value}
                                  >
                                    {option.label}
                                  </SelectItem>
                                ))}
                              </SelectContent>
                            </Select>
                          </FormItem>
                        )}
                      />
                    </div>
                    <FormMessage />
                  </FormItem>
                )}
              />
            </div>

            <div className='space-y-2'>
              <div className='text-sm font-medium'>
                {t('systemSettings.fields.autoCleanTargets')}
              </div>
              <div className='space-y-2'>
                {targetOptions.map((option) => (
                  <label
                    key={option.target}
                    className='hover:bg-accent/50 flex cursor-pointer items-start gap-3 rounded-md border p-3'
                  >
                    <Checkbox
                      checked={selectedAutoTargets.includes(option.target)}
                      onCheckedChange={(value) =>
                        toggleAutoTarget(option.target, value === true)
                      }
                      disabled={!autoCleanEnabled}
                      className='mt-0.5'
                    />
                    <div className='space-y-0.5'>
                      <div className='text-sm leading-none font-medium'>
                        {option.label}
                      </div>
                      <p className='text-muted-foreground text-xs'>
                        {option.description}
                      </p>
                    </div>
                  </label>
                ))}
              </div>
            </div>
          </SettingsControlGroup>

          <SettingsControlGroup className='space-y-3'>
            <div>
              <h4 className='text-sm font-medium'>
                {t('systemSettings.actions.cleanHistoryLogs')}
              </h4>
              <p className='text-muted-foreground text-sm'>
                {t(
                  'systemSettings.placeholders.selectLogTypesAndTimeRangeToRemoveMatching'
                )}
              </p>
            </div>

            <div className='space-y-2'>
              {targetOptions.map((option) => (
                <label
                  key={option.key}
                  className='hover:bg-accent/50 flex cursor-pointer items-start gap-3 rounded-md border p-3'
                >
                  <Checkbox
                    checked={targets[option.key]}
                    onCheckedChange={(value) =>
                      setTargets((prev) => ({
                        ...prev,
                        [option.key]: value === true,
                      }))
                    }
                    className='mt-0.5'
                  />
                  <div className='space-y-0.5'>
                    <div className='text-sm leading-none font-medium'>
                      {option.label}
                    </div>
                    <p className='text-muted-foreground text-xs'>
                      {option.description}
                    </p>
                  </div>
                </label>
              ))}
            </div>

            <div className='space-y-2'>
              <div className='text-sm font-medium'>
                {t('systemSettings.fields.timeRange')}
              </div>
              <div className='grid gap-2 sm:grid-cols-2'>
                <div className='space-y-1'>
                  <div className='text-muted-foreground text-xs'>
                    {t('subscriptions.actions.start')}
                  </div>
                  <DateTimePicker
                    value={startDate}
                    onChange={setStartDate}
                    placeholder={t('systemSettings.fields.noLowerBound')}
                  />
                </div>
                <div className='space-y-1'>
                  <div className='text-muted-foreground text-xs'>
                    {t('subscriptions.fields.end')}
                  </div>
                  <DateTimePicker
                    value={endDate}
                    onChange={setEndDate}
                    placeholder={t('systemSettings.placeholders.requiredField')}
                  />
                </div>
              </div>
            </div>

            <div className='flex flex-wrap gap-3'>
              <Button
                type='button'
                variant='outline'
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
                onClick={() => {
                  setStartDate(getDateDaysAgo(30))
                  setEndDate(getDateDaysAgo(0))
                }}
              >
                {t('systemSettings.fields.last30Days')}
              </Button>
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
            </div>
          </SettingsControlGroup>
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
              {targets.cleanAuditLogs && (
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
