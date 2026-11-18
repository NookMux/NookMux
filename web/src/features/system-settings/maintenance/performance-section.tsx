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
import { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import * as z from 'zod'
import { useForm } from 'react-hook-form'
import { zodResolver } from '@hookform/resolvers/zod'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { api } from '@/lib/api'
import { Alert, AlertDescription } from '@/components/ui/alert'
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
  AlertDialogTrigger,
} from '@/components/ui/alert-dialog'
import { Button } from '@/components/ui/button'
import {
  Form,
  FormControl,
  FormDescription,
  FormField,
  FormItem,
  FormLabel,
} from '@/components/ui/form'
import { Input } from '@/components/ui/input'
import { Progress } from '@/components/ui/progress'
import { Separator } from '@/components/ui/separator'
import { Switch } from '@/components/ui/switch'
import { StatusBadge } from '@/components/status-badge'
import { DisabledSettingsNotice } from '../components/disabled-settings-notice'
import {
  SettingsForm,
  SettingsSwitchContent,
  SettingsSwitchItem,
} from '../components/settings-form-layout'
import { SettingsPageFormActions } from '../components/settings-page-context'
import { SettingsSection } from '../components/settings-section'
import { useResetForm } from '../hooks/use-reset-form'
import { useUpdateOption } from '../hooks/use-update-option'

/**
 * Performance form uses nested schema because field names contain dots.
 * React Hook Form treats dotted names as nested paths internally
 * (IS_KEY_RE = /^\w*$/), so the schema must mirror that nested structure.
 */
const perfSchema = z.object({
  performance_setting: z.object({
    disk_cache_enabled: z.boolean(),
    disk_cache_threshold_mb: z.coerce
      .number()
      .min(1, 'systemSettings.errors.mustBeAtLeast1'),
    disk_cache_max_size_mb: z.coerce
      .number()
      .min(100, 'systemSettings.errors.mustBeAtLeast100'),
    disk_cache_path: z.string().optional(),
    monitor_enabled: z.boolean(),
    monitor_cpu_threshold: z.coerce
      .number()
      .min(0, 'systemSettings.errors.mustBeNonNegative'),
    monitor_memory_threshold: z.coerce
      .number()
      .min(0, 'common.errors.mustBeBetween0And100')
      .max(100, 'common.errors.mustBeBetween0And100'),
    monitor_disk_threshold: z.coerce
      .number()
      .min(0, 'common.errors.mustBeBetween0And100')
      .max(100, 'common.errors.mustBeBetween0And100'),
  }),
})

type PerfFormValues = z.output<typeof perfSchema>
type PerfFormInput = z.input<typeof perfSchema>

/**
 * Flat option keys matching the backend system-option key names.
 * Used to map between the nested form shape and the flat option store.
 */
type FlatPerfOptionKey =
  | 'performance_setting.disk_cache_enabled'
  | 'performance_setting.disk_cache_threshold_mb'
  | 'performance_setting.disk_cache_max_size_mb'
  | 'performance_setting.disk_cache_path'
  | 'performance_setting.monitor_enabled'
  | 'performance_setting.monitor_cpu_threshold'
  | 'performance_setting.monitor_memory_threshold'
  | 'performance_setting.monitor_disk_threshold'

type FlatPerfValues = {
  'performance_setting.disk_cache_enabled': boolean
  'performance_setting.disk_cache_threshold_mb': number
  'performance_setting.disk_cache_max_size_mb': number
  'performance_setting.disk_cache_path': string
  'performance_setting.monitor_enabled': boolean
  'performance_setting.monitor_cpu_threshold': number
  'performance_setting.monitor_memory_threshold': number
  'performance_setting.monitor_disk_threshold': number
}

function formatBytes(bytes: number, decimals = 2): string {
  if (!bytes || isNaN(bytes)) return '0 Bytes'
  if (bytes === 0) return '0 Bytes'
  if (bytes < 0) return '-' + formatBytes(-bytes, decimals)
  const k = 1024
  const sizes = ['Bytes', 'KB', 'MB', 'GB', 'TB']
  const i = Math.floor(Math.log(Math.abs(bytes)) / Math.log(k))
  if (i < 0 || i >= sizes.length) return bytes + ' Bytes'
  return parseFloat((bytes / Math.pow(k, i)).toFixed(decimals)) + ' ' + sizes[i]
}

/** Props use flat keys (matching backend option names); the form uses nested shape. */
interface Props {
  defaultValues: FlatPerfValues
}

const buildFormDefaults = (
  defaults: Props['defaultValues']
): PerfFormInput => ({
  performance_setting: {
    disk_cache_enabled: defaults['performance_setting.disk_cache_enabled'],
    disk_cache_threshold_mb:
      defaults['performance_setting.disk_cache_threshold_mb'],
    disk_cache_max_size_mb:
      defaults['performance_setting.disk_cache_max_size_mb'],
    disk_cache_path: defaults['performance_setting.disk_cache_path'] ?? '',
    monitor_enabled: defaults['performance_setting.monitor_enabled'],
    monitor_cpu_threshold:
      defaults['performance_setting.monitor_cpu_threshold'],
    monitor_memory_threshold:
      defaults['performance_setting.monitor_memory_threshold'],
    monitor_disk_threshold:
      defaults['performance_setting.monitor_disk_threshold'],
  },
})

const flattenFormValues = (values: PerfFormValues): FlatPerfValues => ({
  'performance_setting.disk_cache_enabled':
    values.performance_setting.disk_cache_enabled,
  'performance_setting.disk_cache_threshold_mb':
    values.performance_setting.disk_cache_threshold_mb,
  'performance_setting.disk_cache_max_size_mb':
    values.performance_setting.disk_cache_max_size_mb,
  'performance_setting.disk_cache_path':
    values.performance_setting.disk_cache_path ?? '',
  'performance_setting.monitor_enabled':
    values.performance_setting.monitor_enabled,
  'performance_setting.monitor_cpu_threshold':
    values.performance_setting.monitor_cpu_threshold,
  'performance_setting.monitor_memory_threshold':
    values.performance_setting.monitor_memory_threshold,
  'performance_setting.monitor_disk_threshold':
    values.performance_setting.monitor_disk_threshold,
})

const flatDefaultsToBaseline = (
  defaults: Props['defaultValues']
): FlatPerfValues => ({
  'performance_setting.disk_cache_enabled':
    defaults['performance_setting.disk_cache_enabled'],
  'performance_setting.disk_cache_threshold_mb':
    defaults['performance_setting.disk_cache_threshold_mb'],
  'performance_setting.disk_cache_max_size_mb':
    defaults['performance_setting.disk_cache_max_size_mb'],
  'performance_setting.disk_cache_path':
    defaults['performance_setting.disk_cache_path'] ?? '',
  'performance_setting.monitor_enabled':
    defaults['performance_setting.monitor_enabled'],
  'performance_setting.monitor_cpu_threshold':
    defaults['performance_setting.monitor_cpu_threshold'],
  'performance_setting.monitor_memory_threshold':
    defaults['performance_setting.monitor_memory_threshold'],
  'performance_setting.monitor_disk_threshold':
    defaults['performance_setting.monitor_disk_threshold'],
})

type PerformanceStats = {
  cache_stats?: {
    current_disk_usage_bytes: number
    disk_cache_max_bytes: number
    active_disk_files: number
    disk_cache_hits: number
    current_memory_usage_bytes: number
    active_memory_buffers: number
    memory_cache_hits: number
  }
  disk_space_info?: {
    total: number
    free: number
    used: number
    used_percent: number
  }
  memory_stats?: {
    alloc: number
    total_alloc: number
    sys: number
    num_gc: number
    num_goroutine: number
  }
  disk_cache_info?: {
    path: string
    file_count: number
    total_size: number
  }
  config?: {
    is_running_in_container: boolean
  }
}

export function PerformanceSection(props: Props) {
  const { t } = useTranslation()
  const updateOption = useUpdateOption()
  const [stats, setStats] = useState<PerformanceStats | null>(null)

  const formDefaults = useMemo(
    () => buildFormDefaults(props.defaultValues),
    [props.defaultValues]
  )

  const form = useForm<PerfFormInput, unknown, PerfFormValues>({
    resolver: zodResolver(perfSchema),
    defaultValues: formDefaults,
  })

  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  useResetForm(form as any, formDefaults)

  const baselineRef = useRef<FlatPerfValues>(
    flatDefaultsToBaseline(props.defaultValues)
  )

  const fetchStats = useCallback(async () => {
    try {
      const res = await api.get('/api/performance/stats')
      if (res.data.success) setStats(res.data.data)
    } catch (error) {
      toast.error(
        error instanceof Error
          ? error.message
          : t('systemSettings.errors.failedToFetchPerformanceStats')
      )
    }
  }, [t])

  useEffect(() => {
    fetchStats()
  }, [fetchStats])

  const onSubmit = async (data: PerfFormValues) => {
    const normalized = flattenFormValues(data)
    const updates = (Object.keys(normalized) as FlatPerfOptionKey[]).filter(
      (key) => normalized[key] !== baselineRef.current[key]
    )
    if (updates.length === 0) {
      toast.info(t('channels.fields.noChangesToSave'))
      return
    }
    for (const key of updates) {
      await updateOption.mutateAsync({
        key,
        value: normalized[key],
      })
    }
    baselineRef.current = normalized
    toast.success(t('systemSettings.status.savedSuccessfully'))
    fetchStats()
  }

  const clearDiskCache = async () => {
    try {
      const res = await api.delete('/api/performance/disk_cache')
      if (res.data.success) {
        toast.success(t('systemSettings.fields.diskCacheCleared'))
        fetchStats()
      }
    } catch {
      toast.error(t('systemSettings.status.cleanupFailed'))
    }
  }

  const resetStats = async () => {
    try {
      const res = await api.post('/api/performance/reset_stats')
      if (res.data.success) {
        toast.success(t('systemSettings.fields.statisticsReset'))
        fetchStats()
      }
    } catch {
      toast.error(t('systemSettings.actions.resetFailed'))
    }
  }

  const forceGC = async () => {
    try {
      const res = await api.post('/api/performance/gc')
      if (res.data.success) {
        toast.success(t('systemSettings.fields.gcExecuted'))
        fetchStats()
      }
    } catch {
      toast.error(t('systemSettings.status.gcExecutionFailed'))
    }
  }

  const diskEnabled = form.watch('performance_setting.disk_cache_enabled')
  const monitorEnabled = form.watch('performance_setting.monitor_enabled')
  const maxCacheSizeMbRaw = form.watch(
    'performance_setting.disk_cache_max_size_mb'
  )
  const maxCacheSizeMb =
    typeof maxCacheSizeMbRaw === 'number' && Number.isFinite(maxCacheSizeMbRaw)
      ? maxCacheSizeMbRaw
      : 0
  const lowDiskSpace =
    diskEnabled &&
    stats?.disk_space_info &&
    stats.disk_space_info.free > 0 &&
    maxCacheSizeMb > 0 &&
    stats.disk_space_info.free < maxCacheSizeMb * 1024 * 1024

  const diskCachePercent =
    stats?.cache_stats?.disk_cache_max_bytes &&
    stats.cache_stats.disk_cache_max_bytes > 0
      ? Math.round(
          (stats.cache_stats.current_disk_usage_bytes /
            stats.cache_stats.disk_cache_max_bytes) *
            100
        )
      : 0

  return (
    <SettingsSection title={t('systemSettings.titles.performanceSettings')}>
      <Form {...form}>
        <SettingsForm onSubmit={form.handleSubmit(onSubmit)}>
          <SettingsPageFormActions
            onSave={form.handleSubmit(onSubmit)}
            isSaving={updateOption.isPending}
            saveLabel='common.actions.saveChanges'
            savingLabel='common.tips.saving'
          />
          {/* Disk Cache Settings */}
          <div>
            <h4 className='font-medium'>
              {t('systemSettings.titles.diskCacheSettings')}
            </h4>
            <p className='text-muted-foreground mt-1 text-xs'>
              {t(
                'systemSettings.status.enabledLargeRequestBodiesAreTemporarilyStoredOnDisk'
              )}
            </p>
          </div>

          <DisabledSettingsNotice enabled={diskEnabled} />

          <div data-settings-form-span='full' className='space-y-4'>
            <FormField
              control={form.control}
              name='performance_setting.disk_cache_enabled'
              render={({ field }) => (
                <SettingsSwitchItem className='border-b-2 pb-3'>
                  <SettingsSwitchContent>
                    <FormLabel>
                      {t('systemSettings.actions.enableDiskCache')}
                    </FormLabel>
                  </SettingsSwitchContent>
                  <FormControl>
                    <Switch
                      checked={field.value}
                      onCheckedChange={field.onChange}
                    />
                  </FormControl>
                </SettingsSwitchItem>
              )}
            />

            <div className='border-border/40 space-y-4 border-l pl-4'>
              <div className='grid grid-cols-1 gap-4 md:grid-cols-2'>
                <FormField
                  control={form.control}
                  name='performance_setting.disk_cache_threshold_mb'
                  render={({ field }) => (
                    <FormItem>
                      <FormLabel>
                        {t('systemSettings.fields.diskCacheThresholdMb')}
                      </FormLabel>
                      <FormControl>
                        <Input
                          type='number'
                          min={1}
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
                          disabled={!diskEnabled}
                        />
                      </FormControl>
                      <FormDescription>
                        {t(
                          'systemSettings.actions.useDiskCacheWhenRequestBodyExceedsThisSize'
                        )}
                      </FormDescription>
                    </FormItem>
                  )}
                />
                <FormField
                  control={form.control}
                  name='performance_setting.disk_cache_max_size_mb'
                  render={({ field }) => (
                    <FormItem>
                      <FormLabel>
                        {t('systemSettings.fields.maxDiskCacheSizeMb')}
                      </FormLabel>
                      <FormControl>
                        <Input
                          type='number'
                          min={100}
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
                          disabled={!diskEnabled}
                        />
                      </FormControl>
                      {stats?.disk_space_info &&
                        stats.disk_space_info.total > 0 && (
                          <FormDescription>
                            {t('systemSettings.tips.freeFreeTotalTotal', {
                              free: formatBytes(stats.disk_space_info.free),
                              total: formatBytes(stats.disk_space_info.total),
                            })}
                          </FormDescription>
                        )}
                    </FormItem>
                  )}
                />
              </div>

              {lowDiskSpace && (
                <Alert variant='destructive'>
                  <AlertDescription>
                    {`${t('systemSettings.tips.warning')}: ${t('systemSettings.fields.availableDiskSpace')} (${formatBytes(stats?.disk_space_info?.free ?? 0)}) ${t('systemSettings.tips.lessThanTheConfiguredMaximumCacheSize')} (${maxCacheSizeMb} MB). ${t('systemSettings.tips.mayCauseCacheFailures')}`}
                  </AlertDescription>
                </Alert>
              )}

              {!stats?.config?.is_running_in_container && (
                <FormField
                  control={form.control}
                  name='performance_setting.disk_cache_path'
                  render={({ field }) => (
                    <FormItem className='max-w-md'>
                      <FormLabel>
                        {t('systemSettings.fields.cacheDirectory')}
                      </FormLabel>
                      <FormControl>
                        <Input
                          placeholder={t(
                            'systemSettings.tips.leaveEmptyToUseSystemTempDirectory'
                          )}
                          {...field}
                          value={field.value ?? ''}
                          disabled={!diskEnabled}
                        />
                      </FormControl>
                    </FormItem>
                  )}
                />
              )}
            </div>
          </div>

          <Separator />

          {/* System Performance Monitor */}
          <div>
            <h4 className='font-medium'>
              {t('systemSettings.titles.performanceMonitoring')}
            </h4>
            <p className='text-muted-foreground mt-1 text-xs'>
              {t(
                'systemSettings.status.performanceMonitoringIsEnabledAndSystemResourceUsageExceeds'
              )}
            </p>
          </div>

          <DisabledSettingsNotice enabled={monitorEnabled} />

          <div data-settings-form-span='full' className='space-y-4'>
            <FormField
              control={form.control}
              name='performance_setting.monitor_enabled'
              render={({ field }) => (
                <SettingsSwitchItem className='border-b-2 pb-3'>
                  <SettingsSwitchContent>
                    <FormLabel>
                      {t('systemSettings.actions.enablePerformanceMonitoring')}
                    </FormLabel>
                  </SettingsSwitchContent>
                  <FormControl>
                    <Switch
                      checked={field.value}
                      onCheckedChange={field.onChange}
                    />
                  </FormControl>
                </SettingsSwitchItem>
              )}
            />

            <div className='border-border/40 grid grid-cols-1 gap-4 border-l pl-4 md:grid-cols-3'>
              <FormField
                control={form.control}
                name='performance_setting.monitor_cpu_threshold'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>
                      {t('systemSettings.fields.cpuThreshold')}
                    </FormLabel>
                    <FormControl>
                      <Input
                        type='number'
                        min={0}
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
                        disabled={!monitorEnabled}
                      />
                    </FormControl>
                  </FormItem>
                )}
              />
              <FormField
                control={form.control}
                name='performance_setting.monitor_memory_threshold'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>
                      {t('systemSettings.fields.memoryThreshold')}
                    </FormLabel>
                    <FormControl>
                      <Input
                        type='number'
                        min={0}
                        max={100}
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
                        disabled={!monitorEnabled}
                      />
                    </FormControl>
                  </FormItem>
                )}
              />
              <FormField
                control={form.control}
                name='performance_setting.monitor_disk_threshold'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>
                      {t('systemSettings.fields.diskThreshold')}
                    </FormLabel>
                    <FormControl>
                      <Input
                        type='number'
                        min={0}
                        max={100}
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
                        disabled={!monitorEnabled}
                      />
                    </FormControl>
                  </FormItem>
                )}
              />
            </div>
          </div>
        </SettingsForm>
      </Form>

      <Separator />

      {/* Performance Stats Dashboard */}
      <div className='space-y-4'>
        <div className='flex items-center gap-2'>
          <h4 className='font-medium'>
            {t('systemSettings.fields.performanceMonitor')}
          </h4>
          <Button variant='outline' size='sm' onClick={fetchStats}>
            {t('systemSettings.actions.refreshStats')}
          </Button>
          <AlertDialog>
            <AlertDialogTrigger render={<Button variant='outline' size='sm' />}>
              {t('systemSettings.actions.cleanUpInactiveCache')}
            </AlertDialogTrigger>
            <AlertDialogContent>
              <AlertDialogHeader>
                <AlertDialogTitle>
                  {t(
                    'systemSettings.actions.confirmCleanupOfInactiveDiskCache'
                  )}
                </AlertDialogTitle>
                <AlertDialogDescription>
                  {t(
                    'systemSettings.status.deleteTemporaryCacheFilesThatHaveNotBeenUsed'
                  )}
                </AlertDialogDescription>
              </AlertDialogHeader>
              <AlertDialogFooter>
                <AlertDialogCancel>
                  {t('common.actions.cancel')}
                </AlertDialogCancel>
                <AlertDialogAction onClick={clearDiskCache}>
                  {t('common.actions.confirm')}
                </AlertDialogAction>
              </AlertDialogFooter>
            </AlertDialogContent>
          </AlertDialog>
          <Button variant='outline' size='sm' onClick={resetStats}>
            {t('systemSettings.actions.resetStats')}
          </Button>
          <Button variant='outline' size='sm' onClick={forceGC}>
            {t('systemSettings.actions.runGc')}
          </Button>
        </div>

        {stats && (
          <>
            <div className='grid grid-cols-1 gap-4 md:grid-cols-2'>
              <div className='space-y-2 rounded-lg border p-4'>
                <p className='text-sm font-medium'>
                  {t('systemSettings.fields.requestBodyDiskCache')}
                </p>
                <Progress value={diskCachePercent} />
                <div className='text-muted-foreground flex justify-between text-xs'>
                  <span>
                    {formatBytes(
                      stats.cache_stats?.current_disk_usage_bytes ?? 0
                    )}{' '}
                    /{' '}
                    {formatBytes(stats.cache_stats?.disk_cache_max_bytes ?? 0)}
                  </span>
                  <span>
                    {t('systemSettings.status.activeFiles')}:{' '}
                    {stats.cache_stats?.active_disk_files ?? 0}
                  </span>
                </div>
                <StatusBadge variant='neutral' copyable={false}>
                  {t('systemSettings.fields.diskHits')}:{' '}
                  {stats.cache_stats?.disk_cache_hits ?? 0}
                </StatusBadge>
              </div>
              <div className='space-y-2 rounded-lg border p-4'>
                <p className='text-sm font-medium'>
                  {t('systemSettings.fields.requestBodyMemoryCache')}
                </p>
                <div className='text-muted-foreground flex justify-between text-xs'>
                  <span>
                    {t('systemSettings.fields.currentCacheSize')}:{' '}
                    {formatBytes(
                      stats.cache_stats?.current_memory_usage_bytes ?? 0
                    )}
                  </span>
                  <span>
                    {t('systemSettings.status.activeCacheCount')}:{' '}
                    {stats.cache_stats?.active_memory_buffers ?? 0}
                  </span>
                </div>
                <StatusBadge variant='neutral' copyable={false}>
                  {t('systemSettings.fields.memoryHits')}:{' '}
                  {stats.cache_stats?.memory_cache_hits ?? 0}
                </StatusBadge>
              </div>
            </div>

            {stats.disk_space_info && stats.disk_space_info.total > 0 && (
              <div className='rounded-lg border p-4'>
                <p className='mb-2 text-sm font-medium'>
                  {t('systemSettings.fields.cacheDirectoryDiskSpace')}
                </p>
                <Progress
                  value={Math.round(stats.disk_space_info.used_percent)}
                />
                <div className='text-muted-foreground mt-2 flex justify-between text-xs'>
                  <span>
                    {t('common.status.used')}:{' '}
                    {formatBytes(stats.disk_space_info.used)}
                  </span>
                  <span>
                    {t('channels.fields.available')}:{' '}
                    {formatBytes(stats.disk_space_info.free)}
                  </span>
                  <span>
                    {t('dashboard.fields.total')}:{' '}
                    {formatBytes(stats.disk_space_info.total)}
                  </span>
                </div>
              </div>
            )}

            {stats.memory_stats && (
              <div className='rounded-lg border p-4'>
                <p className='mb-2 text-sm font-medium'>
                  {t('systemSettings.titles.memoryStats')}
                </p>
                <div className='grid grid-cols-2 gap-2 text-xs md:grid-cols-5'>
                  <div>
                    <span className='text-muted-foreground'>
                      {t('systemSettings.fields.allocatedMemory')}:
                    </span>{' '}
                    {formatBytes(stats.memory_stats.alloc)}
                  </div>
                  <div>
                    <span className='text-muted-foreground'>
                      {t('systemSettings.fields.totalAllocated')}:
                    </span>{' '}
                    {formatBytes(stats.memory_stats.total_alloc)}
                  </div>
                  <div>
                    <span className='text-muted-foreground'>
                      {t('systemSettings.titles.memory')}:
                    </span>{' '}
                    {formatBytes(stats.memory_stats.sys)}
                  </div>
                  <div>
                    <span className='text-muted-foreground'>
                      {t('systemSettings.fields.gcCount')}:
                    </span>{' '}
                    {stats.memory_stats.num_gc}
                  </div>
                  <div>
                    <span className='text-muted-foreground'>
                      {t('systemSettings.fields.goroutines')}:
                    </span>{' '}
                    {stats.memory_stats.num_goroutine}
                  </div>
                </div>
              </div>
            )}

            {stats.disk_cache_info && (
              <div className='rounded-lg border p-4'>
                <p className='mb-2 text-sm font-medium'>
                  {t('systemSettings.fields.cacheDirectoryInfo')}
                </p>
                <div className='grid grid-cols-3 gap-2 text-xs'>
                  <div>
                    <span className='text-muted-foreground'>
                      {t('systemSettings.fields.cacheDirectory')}:
                    </span>{' '}
                    <span className='font-mono'>
                      {stats.disk_cache_info.path}
                    </span>
                  </div>
                  <div>
                    <span className='text-muted-foreground'>
                      {t('systemSettings.fields.directoryFileCount')}:
                    </span>{' '}
                    {stats.disk_cache_info.file_count}
                  </div>
                  <div>
                    <span className='text-muted-foreground'>
                      {t('systemSettings.fields.directoryTotalSize')}:
                    </span>{' '}
                    {formatBytes(stats.disk_cache_info.total_size)}
                  </div>
                </div>
              </div>
            )}
          </>
        )}
      </div>
    </SettingsSection>
  )
}
