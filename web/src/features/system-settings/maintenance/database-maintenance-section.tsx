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
import { useCallback, useEffect, useMemo, useState } from 'react'
import { AlertTriangle, Database, Loader2, RefreshCw } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { cn } from '@/lib/utils'
import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert'
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
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Progress } from '@/components/ui/progress'
import { Switch } from '@/components/ui/switch'
import { Textarea } from '@/components/ui/textarea'
import {
  getDatabaseMigrationInfo,
  getDatabaseMigrationJob,
  startDatabaseMigration,
} from '../api'
import { SettingsSection } from '../components/settings-section'
import type {
  DatabaseMigrationInfo,
  DatabaseMigrationJob,
  DatabaseMigrationMode,
  DatabaseMigrationStartRequest,
} from '../types'

type MigrationCardConfig = {
  mode: DatabaseMigrationMode
  title: string
  description: string
  help: string
  startLabel: string
  confirmTitle: string
  confirmDescription: string
}

type MigrationFormState = {
  targetDsn: string
  targetLogDsn: string
  includeLogs: boolean
  force: boolean
}

const POLL_INTERVAL_MS = 2000

const defaultFormState: MigrationFormState = {
  targetDsn: '',
  targetLogDsn: '',
  includeLogs: false,
  force: false,
}

function formatDbType(value?: string) {
  if (!value) return '-'
  return value
}

function formatTimestamp(seconds?: number) {
  if (!seconds) return '-'
  return new Date(seconds * 1000).toLocaleString()
}

function calculateProgress(job: DatabaseMigrationJob | null) {
  if (!job || job.tables.length === 0) return 0
  const total = job.tables.reduce((sum, table) => sum + table.total, 0)
  const copied = job.tables.reduce((sum, table) => sum + table.copied, 0)
  if (total <= 0) return job.status === 'success' ? 100 : 0
  return Math.min(100, Math.round((copied / total) * 100))
}

function statusVariant(status?: DatabaseMigrationJob['status']) {
  if (status === 'success') return 'default'
  if (status === 'failed') return 'destructive'
  return 'secondary'
}

export function DatabaseMaintenanceSection() {
  const { t } = useTranslation()

  const configs = useMemo<MigrationCardConfig[]>(
    () => [
      {
        mode: 'pre_migrate',
        title: t('systemSettings.fields.databaseMigration'),
        description: t(
          'systemSettings.actions.copyTheCurrentDatabaseToANewDatabaseOf'
        ),
        help: t('systemSettings.tips.supportsSqliteToMySqlPostgreSqlMySql'),
        startLabel: t('systemSettings.actions.startDatabaseMigration'),
        confirmTitle: t('systemSettings.actions.confirmDatabaseMigration'),
        confirmDescription: t(
          'systemSettings.tips.migrationTaskWillCopyDataToTheTargetDatabase'
        ),
      },
      {
        mode: 'same_type_migrate',
        title: t('systemSettings.fields.databaseSync'),
        description: t(
          'systemSettings.actions.copyTheCurrentDatabaseToAnotherDatabaseOfThe'
        ),
        help: t('systemSettings.tips.supportsMySqlToMySqlAndPostgreSql'),
        startLabel: t('systemSettings.actions.startDatabaseSync'),
        confirmTitle: t('systemSettings.actions.confirmDatabaseSync'),
        confirmDescription: t(
          'systemSettings.tips.syncTaskWillCopyDataToTheTargetDatabase'
        ),
      },
    ],
    [t]
  )

  return (
    <SettingsSection title={t('systemSettings.fields.databaseMaintenance')}>
      <Alert variant='danger'>
        <AlertTriangle className='size-4' />
        <AlertTitle>{t('systemSettings.fields.highRiskOperation')}</AlertTitle>
        <AlertDescription>
          {t('systemSettings.tips.databaseMigrationAndSyncRunOnTheServerAnd')}
        </AlertDescription>
      </Alert>

      <div className='grid gap-4 xl:grid-cols-2'>
        {configs.map((config) => (
          <DatabaseMigrationCard key={config.mode} config={config} />
        ))}
      </div>
    </SettingsSection>
  )
}

function DatabaseMigrationCard({ config }: { config: MigrationCardConfig }) {
  const { t } = useTranslation()
  const [info, setInfo] = useState<DatabaseMigrationInfo | null>(null)
  const [job, setJob] = useState<DatabaseMigrationJob | null>(null)
  const [formState, setFormState] =
    useState<MigrationFormState>(defaultFormState)
  const [isLoadingInfo, setIsLoadingInfo] = useState(false)
  const [isStarting, setIsStarting] = useState(false)
  const [isRefreshingJob, setIsRefreshingJob] = useState(false)
  const [confirmOpen, setConfirmOpen] = useState(false)

  const progress = calculateProgress(job)
  const isRunning = job?.status === 'running'

  const loadInfo = useCallback(async () => {
    setIsLoadingInfo(true)
    try {
      const res = await getDatabaseMigrationInfo(config.mode)
      if (!res.success) {
        throw new Error(
          res.message ||
            t('systemSettings.errors.failedToLoadDatabaseInformation')
        )
      }
      setInfo(res.data)
    } catch (error) {
      toast.error(
        error instanceof Error
          ? error.message
          : t('systemSettings.errors.failedToLoadDatabaseInformation')
      )
    } finally {
      setIsLoadingInfo(false)
    }
  }, [config.mode, t])

  const refreshJob = useCallback(
    async (jobId = job?.id, silent = false) => {
      if (!jobId) return
      if (!silent) setIsRefreshingJob(true)
      try {
        const res = await getDatabaseMigrationJob(config.mode, jobId)
        if (!res.success) {
          throw new Error(
            res.message ||
              t('systemSettings.errors.failedToRefreshMigrationJob')
          )
        }
        setJob(res.data)
      } catch (error) {
        if (!silent) {
          toast.error(
            error instanceof Error
              ? error.message
              : t('systemSettings.errors.failedToRefreshMigrationJob')
          )
        }
      } finally {
        if (!silent) setIsRefreshingJob(false)
      }
    },
    [config.mode, job?.id, t]
  )

  useEffect(() => {
    loadInfo()
  }, [loadInfo])

  useEffect(() => {
    if (!isRunning || !job?.id) return undefined
    const timer = window.setInterval(() => {
      refreshJob(job.id, true)
    }, POLL_INTERVAL_MS)
    return () => window.clearInterval(timer)
  }, [isRunning, job?.id, refreshJob])

  const request: DatabaseMigrationStartRequest = useMemo(
    () => ({
      target_dsn: formState.targetDsn.trim(),
      target_log_dsn: formState.targetLogDsn.trim() || undefined,
      include_logs: formState.includeLogs,
      force: formState.force,
    }),
    [formState]
  )

  const canStart = request.target_dsn.length > 0 && !isStarting && !isRunning

  const handleOpenConfirm = () => {
    if (!request.target_dsn) {
      toast.error(t('systemSettings.errors.targetDatabaseDsnIsRequired'))
      return
    }
    setConfirmOpen(true)
  }

  const handleStart = async () => {
    setIsStarting(true)
    try {
      const res = await startDatabaseMigration(config.mode, request)
      if (!res.success || !res.data?.job_id) {
        throw new Error(
          res.message || t('systemSettings.errors.failedToStartMigrationTask')
        )
      }
      toast.success(t('systemSettings.fields.migrationTaskStarted'))
      setConfirmOpen(false)
      await refreshJob(res.data.job_id, true)
    } catch (error) {
      toast.error(
        error instanceof Error
          ? error.message
          : t('systemSettings.errors.failedToStartMigrationTask')
      )
    } finally {
      setIsStarting(false)
    }
  }

  return (
    <div className='flex min-w-0 flex-col gap-4 rounded-xl border p-4'>
      <div className='flex items-start justify-between gap-4'>
        <div className='min-w-0 space-y-1'>
          <div className='flex items-center gap-2'>
            <Database className='text-muted-foreground size-4' />
            <h4 className='font-medium'>{config.title}</h4>
          </div>
          <p className='text-muted-foreground text-sm'>{config.description}</p>
        </div>
        {job ? (
          <Badge variant={statusVariant(job.status)}>{t(job.status)}</Badge>
        ) : null}
      </div>

      <p className='text-muted-foreground text-xs'>{config.help}</p>

      <div className='bg-muted/30 grid gap-3 rounded-lg p-3 text-sm sm:grid-cols-3'>
        <InfoItem
          label={t('systemSettings.fields.mainDatabase')}
          value={
            isLoadingInfo
              ? t('common.tips.loading')
              : formatDbType(info?.main_db_type)
          }
        />
        <InfoItem
          label={t('systemSettings.fields.logDatabase')}
          value={
            isLoadingInfo
              ? t('common.tips.loading')
              : formatDbType(info?.log_db_type)
          }
        />
        <InfoItem
          label={t('systemSettings.fields.separatedLogDatabase')}
          value={
            info?.log_db_is_separated
              ? t('systemSettings.actions.yes')
              : t('systemSettings.actions.no')
          }
        />
      </div>

      <div className='space-y-3'>
        <div className='space-y-1.5'>
          <Label htmlFor={`${config.mode}-target-dsn`}>
            {t('systemSettings.fields.targetDatabaseDsn')}
          </Label>
          <Input
            id={`${config.mode}-target-dsn`}
            type='password'
            autoComplete='off'
            value={formState.targetDsn}
            onChange={(event) =>
              setFormState((prev) => ({
                ...prev,
                targetDsn: event.target.value,
              }))
            }
            placeholder={t('systemSettings.placeholders.enterTargetSqlDsn')}
            disabled={isRunning}
          />
        </div>

        <div className='space-y-1.5'>
          <Label htmlFor={`${config.mode}-target-log-dsn`}>
            {t('systemSettings.fields.targetLogDatabaseDsn')}
          </Label>
          <Input
            id={`${config.mode}-target-log-dsn`}
            type='password'
            autoComplete='off'
            value={formState.targetLogDsn}
            onChange={(event) =>
              setFormState((prev) => ({
                ...prev,
                targetLogDsn: event.target.value,
              }))
            }
            placeholder={t(
              'systemSettings.tips.optionalLeaveEmptyToUseTargetDatabaseDsn'
            )}
            disabled={isRunning || !formState.includeLogs}
          />
        </div>

        <SwitchRow
          label={t('systemSettings.titles.includeUsageLogs')}
          description={t(
            'systemSettings.actions.copyTheLogDatabaseTableInAdditionToMain'
          )}
          checked={formState.includeLogs}
          disabled={isRunning}
          onCheckedChange={(checked) =>
            setFormState((prev) => ({ ...prev, includeLogs: checked }))
          }
        />
        <SwitchRow
          label={t('systemSettings.fields.forceOverwriteTargetDatabase')}
          description={t(
            'systemSettings.tips.allowMigrationWhenTargetTablesAlreadyContainDataUse'
          )}
          checked={formState.force}
          disabled={isRunning}
          onCheckedChange={(checked) =>
            setFormState((prev) => ({ ...prev, force: checked }))
          }
        />
      </div>

      <div className='flex flex-wrap gap-2'>
        <Button onClick={handleOpenConfirm} disabled={!canStart}>
          {isStarting ? <Loader2 className='size-4 animate-spin' /> : null}
          {config.startLabel}
        </Button>
        <Button
          type='button'
          variant='outline'
          onClick={() => refreshJob()}
          disabled={!job?.id || isRefreshingJob}
        >
          <RefreshCw
            className={cn('size-4', isRefreshingJob && 'animate-spin')}
          />
          {t('systemSettings.actions.refreshTask')}
        </Button>
      </div>

      <JobPanel job={job} progress={progress} />

      <AlertDialog open={confirmOpen} onOpenChange={setConfirmOpen}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>{config.confirmTitle}</AlertDialogTitle>
            <AlertDialogDescription>
              {config.confirmDescription}{' '}
              {formState.force
                ? t(
                    'systemSettings.status.forceOverwriteIsEnabledTargetDataMayBeReplaced'
                  )
                : t(
                    'systemSettings.status.targetDatabaseShouldBeEmptyUnlessForceOverwriteIs'
                  )}
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel disabled={isStarting}>
              {t('common.actions.cancel')}
            </AlertDialogCancel>
            <AlertDialogAction onClick={handleStart} disabled={isStarting}>
              {isStarting
                ? t('systemSettings.tips.starting')
                : t('systemSettings.actions.confirmAndStart')}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </div>
  )
}

function InfoItem({ label, value }: { label: string; value: string }) {
  return (
    <div className='min-w-0'>
      <div className='text-muted-foreground text-xs'>{label}</div>
      <div className='truncate font-medium'>{value}</div>
    </div>
  )
}

function SwitchRow({
  label,
  description,
  checked,
  disabled,
  onCheckedChange,
}: {
  label: string
  description: string
  checked: boolean
  disabled?: boolean
  onCheckedChange: (checked: boolean) => void
}) {
  return (
    <div className='flex items-center justify-between gap-4 rounded-lg border px-3 py-2.5'>
      <div className='min-w-0 space-y-0.5'>
        <Label className='text-sm font-medium'>{label}</Label>
        <p className='text-muted-foreground text-xs'>{description}</p>
      </div>
      <Switch
        checked={checked}
        onCheckedChange={onCheckedChange}
        disabled={disabled}
      />
    </div>
  )
}

function JobPanel({
  job,
  progress,
}: {
  job: DatabaseMigrationJob | null
  progress: number
}) {
  const { t } = useTranslation()

  if (!job) {
    return (
      <div className='text-muted-foreground rounded-lg border border-dashed p-4 text-sm'>
        {t('systemSettings.tips.noMigrationTaskHasBeenStartedInThisSession')}
      </div>
    )
  }

  return (
    <div className='space-y-3 rounded-lg border p-3'>
      <div className='flex flex-wrap items-center justify-between gap-2'>
        <div className='min-w-0'>
          <div className='text-sm font-medium'>
            {t('systemSettings.fields.taskProgress')}
          </div>
          <div className='text-muted-foreground text-xs break-all'>
            {t('systemSettings.fields.taskId')}: {job.id}
          </div>
        </div>
        <Badge variant={statusVariant(job.status)}>{t(job.status)}</Badge>
      </div>

      <Progress value={progress} />
      <div className='text-muted-foreground flex flex-wrap gap-x-4 gap-y-1 text-xs'>
        <span>
          {t('systemSettings.fields.progress')}: {progress}%
        </span>
        <span>
          {t('systemSettings.fields.currentStep')}: {job.current_step || '-'}
        </span>
        <span>
          {t('subscriptions.fields.source')}: {formatDbType(job.source_db_type)}
        </span>
        <span>
          {t('systemSettings.fields.target')}:{' '}
          {formatDbType(job.target_db_type)}
        </span>
        <span>
          {t('systemSettings.fields.startedAt')}:{' '}
          {formatTimestamp(job.started_at)}
        </span>
        {job.finished_at ? (
          <span>
            {t('systemSettings.fields.finishedAt')}:{' '}
            {formatTimestamp(job.finished_at)}
          </span>
        ) : null}
      </div>

      {job.error ? (
        <Alert variant='destructive'>
          <AlertTitle>{t('systemSettings.status.taskFailed')}</AlertTitle>
          <AlertDescription>{job.error}</AlertDescription>
        </Alert>
      ) : null}

      {job.tables.length > 0 ? (
        <div className='overflow-x-auto rounded-md border'>
          <table className='w-full text-sm'>
            <thead className='bg-muted/50 text-muted-foreground'>
              <tr>
                <th className='px-3 py-2 text-left font-medium'>
                  {t('systemSettings.fields.table')}
                </th>
                <th className='px-3 py-2 text-right font-medium'>
                  {t('common.status.copied')}
                </th>
                <th className='px-3 py-2 text-right font-medium'>
                  {t('dashboard.fields.total')}
                </th>
              </tr>
            </thead>
            <tbody>
              {job.tables.map((table) => (
                <tr key={table.name} className='border-t'>
                  <td className='px-3 py-2 font-mono text-xs'>{table.name}</td>
                  <td className='px-3 py-2 text-right tabular-nums'>
                    {table.copied}
                  </td>
                  <td className='px-3 py-2 text-right tabular-nums'>
                    {table.total}
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      ) : null}

      {job.logs.length > 0 ? (
        <Textarea
          readOnly
          value={job.logs.join('\n')}
          className='min-h-40 resize-y font-mono text-xs'
        />
      ) : null}
    </div>
  )
}
