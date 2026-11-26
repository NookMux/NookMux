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
import { Database, Loader2, RefreshCw } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
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
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Progress } from '@/components/ui/progress'
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
  getDatabaseMigrationInfo,
  getDatabaseMigrationJob,
  startDatabaseMigration,
} from '../api'
import { SettingsControl, SettingsRow } from '../components/settings-group'
import { SettingsNotice } from '../components/settings-notice'
import {
  SettingsPanel,
  SettingsStat,
  SettingsStatGrid,
} from '../components/settings-panel'
import { SettingsSection } from '../components/settings-section'
import type {
  DatabaseMigrationInfo,
  DatabaseMigrationJob,
  DatabaseMigrationMode,
  DatabaseMigrationStartRequest,
} from '../types'

type MigrationPanelConfig = {
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

const JOB_STATUS_LABEL_KEYS: Record<string, string> = {
  running: 'common.fields.running73989d',
  success: 'channels.status.success',
  failed: 'channels.errors.failed',
}

function getJobStatusLabel(status: string, t: (key: string) => string): string {
  const key = JOB_STATUS_LABEL_KEYS[status]
  return key ? t(key) : status
}

export function DatabaseMaintenanceSection() {
  const { t } = useTranslation()

  const configs = useMemo<MigrationPanelConfig[]>(
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
      <SettingsNotice
        tone='danger'
        title={t('systemSettings.fields.highRiskOperation')}
        className='pt-0'
      >
        {t('systemSettings.tips.databaseMigrationAndSyncRunOnTheServerAnd')}
      </SettingsNotice>

      <div className='flex flex-col gap-8'>
        {configs.map((config) => (
          <DatabaseMigrationPanel key={config.mode} config={config} />
        ))}
      </div>
    </SettingsSection>
  )
}

function DatabaseMigrationPanel({ config }: { config: MigrationPanelConfig }) {
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
    <>
      <SettingsPanel
        title={config.title}
        description={config.description}
        icon={Database}
        divided
        actions={
          job ? (
            <Badge variant={statusVariant(job.status)}>
              {getJobStatusLabel(job.status, t)}
            </Badge>
          ) : undefined
        }
        footer={
          <>
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
            <Button
              type='button'
              onClick={handleOpenConfirm}
              disabled={!canStart}
            >
              {isStarting ? <Loader2 className='size-4 animate-spin' /> : null}
              {config.startLabel}
            </Button>
          </>
        }
      >
        <div className='py-2'>
          <SettingsStatGrid columns={3}>
            <SettingsStat
              label={t('systemSettings.fields.mainDatabase')}
              value={
                isLoadingInfo
                  ? t('common.tips.loading')
                  : formatDbType(info?.main_db_type)
              }
            />
            <SettingsStat
              label={t('systemSettings.fields.logDatabase')}
              value={
                isLoadingInfo
                  ? t('common.tips.loading')
                  : formatDbType(info?.log_db_type)
              }
            />
            <SettingsStat
              label={t('systemSettings.fields.separatedLogDatabase')}
              value={
                info?.log_db_is_separated
                  ? t('systemSettings.actions.yes')
                  : t('systemSettings.actions.no')
              }
            />
          </SettingsStatGrid>
        </div>

        <SettingsRow
          label={t('systemSettings.fields.targetDatabaseDsn')}
          description={config.help}
          htmlFor={`${config.mode}-target-dsn`}
        >
          <SettingsControl>
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
          </SettingsControl>
        </SettingsRow>

        <SettingsRow
          label={t('systemSettings.fields.targetLogDatabaseDsn')}
          description={t(
            'systemSettings.tips.optionalLeaveEmptyToUseTargetDatabaseDsn'
          )}
          htmlFor={`${config.mode}-target-log-dsn`}
        >
          <SettingsControl>
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
              disabled={isRunning || !formState.includeLogs}
            />
          </SettingsControl>
        </SettingsRow>

        <SettingsRow
          label={t('systemSettings.titles.includeUsageLogs')}
          description={t(
            'systemSettings.actions.copyTheLogDatabaseTableInAdditionToMain'
          )}
        >
          <Switch
            checked={formState.includeLogs}
            onCheckedChange={(checked) =>
              setFormState((prev) => ({ ...prev, includeLogs: checked }))
            }
            disabled={isRunning}
          />
        </SettingsRow>

        <SettingsRow
          label={t('systemSettings.fields.forceOverwriteTargetDatabase')}
          description={t(
            'systemSettings.tips.allowMigrationWhenTargetTablesAlreadyContainDataUse'
          )}
        >
          <Switch
            checked={formState.force}
            onCheckedChange={(checked) =>
              setFormState((prev) => ({ ...prev, force: checked }))
            }
            disabled={isRunning}
          />
        </SettingsRow>

        <JobDetails job={job} progress={progress} />
      </SettingsPanel>

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
    </>
  )
}

function JobDetails({
  job,
  progress,
}: {
  job: DatabaseMigrationJob | null
  progress: number
}) {
  const { t } = useTranslation()

  if (!job) {
    return (
      <p className='text-muted-foreground py-6 text-center text-sm'>
        {t('systemSettings.tips.noMigrationTaskHasBeenStartedInThisSession')}
      </p>
    )
  }

  return (
    <>
      <SettingsRow
        label={t('systemSettings.fields.taskProgress')}
        description={`${t('systemSettings.fields.taskId')}: ${job.id}`}
        stacked
      >
        <div className='flex flex-col gap-2'>
          <Progress
            value={progress}
            aria-label={t('systemSettings.fields.taskProgress')}
          />
          <div className='text-muted-foreground flex flex-wrap gap-x-4 gap-y-1 text-xs'>
            <span>
              {t('systemSettings.fields.progress')}: {progress}%
            </span>
            <span>
              {t('systemSettings.fields.currentStep')}:{' '}
              {job.current_step || '-'}
            </span>
            <span>
              {t('subscriptions.fields.source')}:{' '}
              {formatDbType(job.source_db_type)}
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
        </div>
      </SettingsRow>

      {job.error ? (
        <SettingsNotice
          tone='danger'
          title={t('systemSettings.status.taskFailed')}
        >
          {job.error}
        </SettingsNotice>
      ) : null}

      {job.tables.length > 0 ? (
        <SettingsRow label={t('systemSettings.fields.table')} stacked>
          <Table className='[&_td:first-child]:pl-0 [&_td:last-child]:pr-0 [&_th:first-child]:pl-0 [&_th:last-child]:pr-0'>
            <TableHeader>
              <TableRow>
                <TableHead>{t('systemSettings.fields.table')}</TableHead>
                <TableHead className='text-right'>
                  {t('common.status.copied')}
                </TableHead>
                <TableHead className='text-right'>
                  {t('dashboard.fields.total')}
                </TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {job.tables.map((table) => (
                <TableRow key={table.name} className='hover:bg-transparent'>
                  <TableCell className='font-mono text-xs'>
                    {table.name}
                  </TableCell>
                  <TableCell className='text-right tabular-nums'>
                    {table.copied}
                  </TableCell>
                  <TableCell className='text-right tabular-nums'>
                    {table.total}
                  </TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
        </SettingsRow>
      ) : null}

      {job.logs.length > 0 ? (
        <SettingsRow label={t('common.titles.taskLogs')} stacked>
          <Textarea
            readOnly
            value={job.logs.join('\n')}
            className='min-h-40 resize-y font-mono text-xs'
          />
        </SettingsRow>
      ) : null}
    </>
  )
}
