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
import { Copy, RefreshCw } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { api } from '@/lib/api'
import { useCopyToClipboard } from '@/hooks/use-copy-to-clipboard'
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
import { Form, FormControl, FormField } from '@/components/ui/form'
import { Input } from '@/components/ui/input'
import { Skeleton } from '@/components/ui/skeleton'
import { SettingsForm } from '../components/settings-form-layout'
import {
  SettingsControl,
  SettingsGroup,
  SettingsGroupSwitch,
  SettingsRow,
} from '../components/settings-group'
import { SettingsNotice } from '../components/settings-notice'
import { SettingsPageFormActions } from '../components/settings-page-context'
import {
  SettingsMeter,
  SettingsPanel,
  SettingsStat,
  SettingsStatGrid,
} from '../components/settings-panel'
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

const STAT_SKELETON_KEYS = ['disk', 'memory', 'files', 'buffers']

export function PerformanceSection(props: Props) {
  const { t } = useTranslation()
  const updateOption = useUpdateOption()
  const [stats, setStats] = useState<PerformanceStats | null>(null)
  const [cleanConfirmOpen, setCleanConfirmOpen] = useState(false)

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
  const diskSpace = stats?.disk_space_info
  const lowDiskSpace = Boolean(
    diskEnabled &&
    diskSpace !== undefined &&
    diskSpace.free > 0 &&
    maxCacheSizeMb > 0 &&
    diskSpace.free < maxCacheSizeMb * 1024 * 1024
  )

  const monitorThresholdFields = [
    {
      name: 'monitor_cpu_threshold',
      label: t('systemSettings.fields.cpuThreshold'),
      max: undefined,
    },
    {
      name: 'monitor_memory_threshold',
      label: t('systemSettings.fields.memoryThreshold'),
      max: 100,
    },
    {
      name: 'monitor_disk_threshold',
      label: t('systemSettings.fields.diskThreshold'),
      max: 100,
    },
  ] as const

  return (
    <SettingsSection title={t('systemSettings.titles.performanceSettings')}>
      <Form {...form}>
        <SettingsForm onSubmit={form.handleSubmit(onSubmit)}>
          <SettingsPageFormActions
            onSave={form.handleSubmit(onSubmit)}
            isSaving={updateOption.isPending}
          />

          <SettingsPanel
            title={t('systemSettings.fields.performanceMonitor')}
            actions={
              <>
                <Button
                  type='button'
                  variant='outline'
                  size='sm'
                  onClick={fetchStats}
                >
                  <RefreshCw data-icon='inline-start' />
                  {t('systemSettings.actions.refreshStats')}
                </Button>
                <AlertDialog
                  open={cleanConfirmOpen}
                  onOpenChange={setCleanConfirmOpen}
                >
                  <AlertDialogTrigger
                    render={
                      <Button type='button' variant='outline' size='sm' />
                    }
                  >
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
                <Button
                  type='button'
                  variant='outline'
                  size='sm'
                  onClick={resetStats}
                >
                  {t('systemSettings.actions.resetStats')}
                </Button>
                <Button
                  type='button'
                  variant='outline'
                  size='sm'
                  onClick={forceGC}
                >
                  {t('systemSettings.actions.runGc')}
                </Button>
              </>
            }
          >
            <div className='py-2'>
              {stats === null ? (
                <SettingsStatGrid aria-busy='true'>
                  {STAT_SKELETON_KEYS.map((key) => (
                    <div key={key} className='flex flex-col gap-2'>
                      <Skeleton className='h-3 w-20' />
                      <Skeleton className='h-5 w-28' />
                    </div>
                  ))}
                </SettingsStatGrid>
              ) : (
                <PerformanceStatsBody stats={stats} />
              )}
            </div>
          </SettingsPanel>

          <SettingsGroup
            title={t('systemSettings.titles.diskCacheSettings')}
            description={t(
              'systemSettings.status.enabledLargeRequestBodiesAreTemporarilyStoredOnDisk'
            )}
            control={
              <SettingsGroupSwitch name='performance_setting.disk_cache_enabled' />
            }
            enabled={diskEnabled}
          >
            <FormField
              control={form.control}
              name='performance_setting.disk_cache_threshold_mb'
              render={({ field }) => (
                <SettingsRow
                  label={t('systemSettings.fields.diskCacheThresholdMb')}
                  description={t(
                    'systemSettings.actions.useDiskCacheWhenRequestBodyExceedsThisSize'
                  )}
                >
                  <SettingsControl>
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
                      />
                    </FormControl>
                  </SettingsControl>
                </SettingsRow>
              )}
            />
            <FormField
              control={form.control}
              name='performance_setting.disk_cache_max_size_mb'
              render={({ field }) => (
                <SettingsRow
                  label={t('systemSettings.fields.maxDiskCacheSizeMb')}
                  hint={
                    diskSpace !== undefined && diskSpace.total > 0
                      ? t('systemSettings.tips.freeFreeTotalTotal', {
                          free: formatBytes(diskSpace.free),
                          total: formatBytes(diskSpace.total),
                        })
                      : undefined
                  }
                >
                  <SettingsControl>
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
                      />
                    </FormControl>
                  </SettingsControl>
                </SettingsRow>
              )}
            />
            {!stats?.config?.is_running_in_container && (
              <FormField
                control={form.control}
                name='performance_setting.disk_cache_path'
                render={({ field }) => (
                  <SettingsRow
                    label={t('systemSettings.fields.cacheDirectory')}
                    hint={t(
                      'systemSettings.tips.leaveEmptyToUseSystemTempDirectory'
                    )}
                  >
                    <SettingsControl>
                      <FormControl>
                        <Input
                          {...field}
                          value={field.value ?? ''}
                          onChange={(event) =>
                            field.onChange(event.target.value)
                          }
                        />
                      </FormControl>
                    </SettingsControl>
                  </SettingsRow>
                )}
              />
            )}
            {lowDiskSpace && diskSpace !== undefined && (
              <SettingsNotice
                tone='warning'
                title={t('systemSettings.tips.warning')}
              >
                {t('systemSettings.fields.availableDiskSpace')} (
                {formatBytes(diskSpace.free)}){' '}
                {t('systemSettings.tips.lessThanTheConfiguredMaximumCacheSize')}{' '}
                ({maxCacheSizeMb} MB).{' '}
                {t('systemSettings.tips.mayCauseCacheFailures')}
              </SettingsNotice>
            )}
          </SettingsGroup>

          <SettingsGroup
            title={t('systemSettings.titles.performanceMonitoring')}
            description={t(
              'systemSettings.status.performanceMonitoringIsEnabledAndSystemResourceUsageExceeds'
            )}
            control={
              <SettingsGroupSwitch name='performance_setting.monitor_enabled' />
            }
            enabled={monitorEnabled}
          >
            {monitorThresholdFields.map(({ name, label, max }) => (
              <FormField
                key={name}
                control={form.control}
                name={`performance_setting.${name}`}
                render={({ field }) => (
                  <SettingsRow label={label}>
                    <SettingsControl>
                      <FormControl>
                        <Input
                          type='number'
                          min={0}
                          max={max}
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
                    </SettingsControl>
                  </SettingsRow>
                )}
              />
            ))}
          </SettingsGroup>
        </SettingsForm>
      </Form>
    </SettingsSection>
  )
}

function StatDetail({ label, value }: { label: string; value: number }) {
  return (
    <span className='inline-flex gap-1.5'>
      <span>{label}</span>
      <span className='text-foreground font-medium tabular-nums'>{value}</span>
    </span>
  )
}

function PerformanceStatsBody({ stats }: { stats: PerformanceStats }) {
  const { t } = useTranslation()
  const { copyToClipboard } = useCopyToClipboard()
  const cache = stats.cache_stats
  const memory = stats.memory_stats
  const diskSpace = stats.disk_space_info
  const diskCache = stats.disk_cache_info

  const diskCacheUsed = cache?.current_disk_usage_bytes ?? 0
  const diskCacheMax = cache?.disk_cache_max_bytes ?? 0
  const diskCachePercent =
    diskCacheMax > 0 ? (diskCacheUsed / diskCacheMax) * 100 : 0

  return (
    <div className='flex flex-col gap-5'>
      <SettingsStatGrid columns={4}>
        <SettingsStat
          label={t('systemSettings.fields.requestBodyDiskCache')}
          value={`${formatBytes(diskCacheUsed)} / ${formatBytes(diskCacheMax)}`}
          detail={
            cache && (
              <StatDetail
                label={t('systemSettings.fields.diskHits')}
                value={cache.disk_cache_hits}
              />
            )
          }
        />
        <SettingsStat
          label={t('systemSettings.fields.requestBodyMemoryCache')}
          value={formatBytes(cache?.current_memory_usage_bytes ?? 0)}
          detail={
            cache && (
              <StatDetail
                label={t('systemSettings.fields.memoryHits')}
                value={cache.memory_cache_hits}
              />
            )
          }
        />
        <SettingsStat
          label={t('systemSettings.status.activeFiles')}
          value={cache?.active_disk_files ?? 0}
        />
        <SettingsStat
          label={t('systemSettings.status.activeCacheCount')}
          value={cache?.active_memory_buffers ?? 0}
        />
      </SettingsStatGrid>

      <div className='grid gap-5 lg:grid-cols-2'>
        {diskSpace !== undefined && diskSpace.total > 0 && (
          <SettingsMeter
            label={t('systemSettings.fields.cacheDirectoryDiskSpace')}
            valueText={t('systemSettings.tips.freeFreeTotalTotal', {
              free: formatBytes(diskSpace.free),
              total: formatBytes(diskSpace.total),
            })}
            percent={diskSpace.used_percent}
          />
        )}
        {cache && (
          <SettingsMeter
            label={t('systemSettings.fields.requestBodyDiskCache')}
            valueText={`${formatBytes(diskCacheUsed)} / ${formatBytes(diskCacheMax)}`}
            percent={diskCachePercent}
          />
        )}
      </div>

      {memory && (
        <div className='flex flex-col gap-3'>
          <h4 className='text-sm leading-snug font-medium'>
            {t('systemSettings.titles.memoryStats')}
          </h4>
          <SettingsStatGrid columns={5}>
            <SettingsStat
              label={t('systemSettings.fields.allocatedMemory')}
              value={formatBytes(memory.alloc)}
            />
            <SettingsStat
              label={t('systemSettings.fields.totalAllocated')}
              value={formatBytes(memory.total_alloc)}
            />
            <SettingsStat
              label={t('systemSettings.titles.memory')}
              value={formatBytes(memory.sys)}
            />
            <SettingsStat
              label={t('systemSettings.fields.gcCount')}
              value={memory.num_gc}
            />
            <SettingsStat
              label={t('systemSettings.fields.goroutines')}
              value={memory.num_goroutine}
            />
          </SettingsStatGrid>
        </div>
      )}

      {diskCache && (
        <div className='flex flex-col gap-3'>
          <h4 className='text-sm leading-snug font-medium'>
            {t('systemSettings.fields.cacheDirectoryInfo')}
          </h4>
          <div className='text-muted-foreground flex min-w-0 flex-wrap items-center gap-x-4 gap-y-1 text-sm'>
            <span className='flex min-w-0 items-center gap-1'>
              <span className='text-xs'>
                {t('systemSettings.fields.cacheDirectory')}
              </span>
              <code className='truncate font-mono text-sm'>
                {diskCache.path}
              </code>
              <Button
                variant='ghost'
                size='icon-sm'
                aria-label={t('common.actions.copyToClipboard')}
                onClick={() => void copyToClipboard(diskCache.path)}
              >
                <Copy />
              </Button>
            </span>
            <span className='inline-flex gap-1.5 text-xs'>
              <span>{t('systemSettings.fields.directoryFileCount')}</span>
              <span className='text-foreground text-sm font-medium tabular-nums'>
                {diskCache.file_count}
              </span>
            </span>
            <span className='inline-flex gap-1.5 text-xs'>
              <span>{t('systemSettings.fields.directoryTotalSize')}</span>
              <span className='text-foreground text-sm font-medium tabular-nums'>
                {formatBytes(diskCache.total_size)}
              </span>
            </span>
          </div>
        </div>
      )}
    </div>
  )
}
