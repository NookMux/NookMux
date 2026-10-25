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
import { Database, HardDrive, Server } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert'
import { StatusBadge } from '@/components/status-badge'
import type { SetupStatus } from '../types'

interface DatabaseStepProps {
  status?: SetupStatus
}

const DATABASE_META: Record<
  string,
  {
    label: string
    descriptionKey: string
    variant: 'info' | 'success' | 'warning'
  }
> = {
  sqlite: {
    label: 'SQLite',
    descriptionKey: 'common.tips.sqliteStoresAllDataInASingleFileMake',
    variant: 'warning',
  },
  mysql: {
    label: 'MySQL',
    descriptionKey: 'common.tips.mySqlIsAProductionReadyRelationalDatabaseKeep',
    variant: 'success',
  },
  postgres: {
    label: 'PostgreSQL',
    descriptionKey:
      'common.tips.postgreSqlOffersAdvancedReliabilityAndDataIntegrityFor',
    variant: 'success',
  },
  opengauss: {
    label: 'openGauss',
    descriptionKey: 'common.tips.openGaussIsAPostgresCompatibleEnterprise',
    variant: 'success',
  },
  dameng: {
    label: '达梦 DM8',
    descriptionKey: 'common.tips.damengIsAnOracleLineageDomesticDatabaseFor',
    variant: 'success',
  },
  kingbase: {
    label: '人大金仓 KingbaseES',
    descriptionKey: 'common.tips.kingbaseIsAnOracleCompatibleDomestic',
    variant: 'success',
  },
}

function resolveDatabaseMeta(type?: string) {
  if (!type) return null
  const normalized = type.toLowerCase()
  return (
    DATABASE_META[normalized] ?? {
      label: type,
      descriptionKey: 'common.tips.customDatabaseDriverDetected',
      variant: 'info' as const,
    }
  )
}

export function DatabaseStep({ status }: DatabaseStepProps) {
  const { t } = useTranslation()
  const meta = resolveDatabaseMeta(status?.database_type)
  const electronApi =
    typeof window !== 'undefined'
      ? ((window as unknown as Record<string, unknown>)?.electron as
          | Record<string, unknown>
          | undefined)
      : undefined
  const isElectron = Boolean(electronApi?.isElectron)
  const electronDataDir = electronApi?.dataDir as string | undefined

  return (
    <div className='space-y-4'>
      <div className='bg-card flex items-center justify-between rounded-lg border p-4'>
        <div className='space-y-1'>
          <p className='text-muted-foreground text-sm font-medium'>
            {t('setup.fields.detectedDatabase')}
          </p>
          <p className='text-foreground text-base font-semibold'>
            {meta?.label ?? t('channels.fields.unknown')}
          </p>
          <p className='text-muted-foreground text-sm'>
            {t(
              meta?.descriptionKey ??
                'setup.tips.setupWizardWillUseThisDatabaseDuringInitialization'
            )}
          </p>
        </div>
        <StatusBadge
          label={meta?.label ?? t('channels.fields.unknown')}
          variant={meta?.variant ?? 'info'}
          className='cursor-default'
          copyable={false}
          icon={Database}
        />
      </div>

      {status?.database_type === 'sqlite' && (
        <Alert className='border-amber-200 bg-amber-50 dark:border-amber-900/60 dark:bg-amber-950/40'>
          <AlertTitle className='flex items-center gap-2'>
            <HardDrive className='size-4 text-amber-500' />
            {t('setup.fields.persistYourDataFile')}
          </AlertTitle>
          <AlertDescription>
            <p>
              {t(
                'setup.tips.runningInContainersOrEphemeralEnvironmentsEnsureTheSqlite'
              )}
            </p>
            {isElectron && electronDataDir && (
              <p className='mt-3 rounded-md bg-amber-100/70 px-3 py-2 font-mono text-xs text-amber-800 dark:bg-amber-900/30 dark:text-amber-200'>
                {t('setup.fields.dataDirectory')} {electronDataDir}
              </p>
            )}
            {isElectron && !electronDataDir && (
              <p className='text-muted-foreground mt-3 text-xs'>
                {t('setup.tips.dataIsStoredLocallyOnThisDeviceUseSystem')}
              </p>
            )}
          </AlertDescription>
        </Alert>
      )}

      {status?.database_type === 'mysql' && (
        <Alert className='border-emerald-200 bg-emerald-50 dark:border-emerald-900/60 dark:bg-emerald-950/40'>
          <AlertTitle className='flex items-center gap-2'>
            <Server className='size-4 text-emerald-500' />
            {t('setup.fields.mySqlDetected')}
          </AlertTitle>
          <AlertDescription>
            {t('setup.tips.mySqlIsProductionReadyEnsureAutomatedBackupsAnd')}
          </AlertDescription>
        </Alert>
      )}

      {status?.database_type === 'postgres' && (
        <Alert className='border-sky-200 bg-sky-50 dark:border-sky-900/60 dark:bg-sky-950/40'>
          <AlertTitle className='flex items-center gap-2'>
            <Server className='size-4 text-sky-500' />
            {t('setup.fields.postgreSqlDetected')}
          </AlertTitle>
          <AlertDescription>
            {t(
              'setup.tips.postgreSqlOffersStrongReliabilityGuaranteesDoubleCheckYour'
            )}
          </AlertDescription>
        </Alert>
      )}

      {status?.database_type === 'opengauss' && (
        <Alert className='border-violet-200 bg-violet-50 dark:border-violet-900/60 dark:bg-violet-950/40'>
          <AlertTitle className='flex items-center gap-2'>
            <Server className='size-4 text-violet-500' />
            {t('setup.fields.openGaussDetected')}
          </AlertTitle>
          <AlertDescription>
            {t('setup.tips.openGaussRequiresTheTargetDatabaseCreatedWith')}
          </AlertDescription>
        </Alert>
      )}
      {status?.database_type === 'dameng' && (
        <Alert className='border-sky-200 bg-sky-50 dark:border-sky-900/60 dark:bg-sky-950/40'>
          <AlertTitle className='flex items-center gap-2'>
            <Server className='size-4 text-sky-500' />
            {t('setup.fields.damengDetected')}
          </AlertTitle>
          <AlertDescription>
            {t('setup.tips.damengRequiresCaseInsensitiveInstanceAnd')}
          </AlertDescription>
        </Alert>
      )}
    </div>
  )
}
