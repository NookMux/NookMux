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
import { useTranslation } from 'react-i18next'
import { Skeleton } from '@/components/ui/skeleton'
import { Switch } from '@/components/ui/switch'
import { SettingsRow } from '../../components/settings-group'
import { SettingsPanel } from '../../components/settings-panel'
import {
  useDashboardConfig,
  useUpdateDashboardConfig,
} from '../hooks/use-dashboard-config'

export function DashboardMetricsSection() {
  const { t } = useTranslation()
  const { data: config, isLoading } = useDashboardConfig()
  const updateConfig = useUpdateDashboardConfig()

  const title = t('systemSettings.fields.dataMetrics')

  if (isLoading) {
    return (
      <SettingsPanel title={title}>
        <div className='flex flex-col gap-4 py-2'>
          <Skeleton className='h-10 w-full' />
          <Skeleton className='h-10 w-full' />
          <Skeleton className='h-10 w-full' />
        </div>
      </SettingsPanel>
    )
  }

  if (!config) return null

  const handleToggle = (field: string, value: boolean) => {
    updateConfig.mutate({ [field]: value })
  }

  return (
    <SettingsPanel
      title={title}
      description={
        <>
          <span className='block'>
            {t(
              'systemSettings.tips.controlWhichDataMetricsAreCalculatedAndDisplayedInThe'
            )}
          </span>
          <span className='block'>
            {t('systemSettings.tips.togglesTakeEffectImmediately')}
          </span>
        </>
      }
      divided
    >
      <SettingsRow
        label={t('systemSettings.fields.quotaData')}
        description={t(
          'systemSettings.actions.enableTimeSeriesQuotaDataDisplayDisablingWillHideModel'
        )}
      >
        <Switch
          checked={config.quota_data_enabled}
          onCheckedChange={(checked) =>
            handleToggle('quota_data_enabled', checked)
          }
        />
      </SettingsRow>

      <SettingsRow
        label={t('systemSettings.titles.userAnalytics')}
        description={t(
          'systemSettings.actions.enableUserConsumptionRankingsAndTrendsDisablingWillReduceDatabase'
        )}
      >
        <Switch
          checked={config.user_analytics_enabled}
          onCheckedChange={(checked) =>
            handleToggle('user_analytics_enabled', checked)
          }
        />
      </SettingsRow>

      <SettingsRow
        label={t('rankings.titles.value')}
        description={t(
          'systemSettings.actions.enableModelAndVendorRankingsThisIsAHighCost'
        )}
      >
        <Switch
          checked={config.rankings_enabled}
          onCheckedChange={(checked) =>
            handleToggle('rankings_enabled', checked)
          }
        />
      </SettingsRow>

      <SettingsRow
        label={t('systemSettings.fields.mediaConversionStats')}
        description={t(
          'systemSettings.actions.enableImageVideoToUrlConversionStatisticsDisplay'
        )}
      >
        <Switch
          checked={config.media_convert_stats_enabled}
          onCheckedChange={(checked) =>
            handleToggle('media_convert_stats_enabled', checked)
          }
        />
      </SettingsRow>

      <SettingsRow
        label={t('systemSettings.fields.trackTokenUsage')}
        description={t(
          'systemSettings.tips.recordTokenUsedInQuotaDataDisablingReducesWriteCost'
        )}
      >
        <Switch
          checked={config.quota_data_track_tokens}
          onCheckedChange={(checked) =>
            handleToggle('quota_data_track_tokens', checked)
          }
        />
      </SettingsRow>

      <SettingsRow
        label={t('systemSettings.tips.aggregateByModel')}
        description={t(
          'systemSettings.tips.aggregateQuotaDataPerModelDisablingCollapsesAllModelsInto'
        )}
      >
        <Switch
          checked={config.quota_data_track_by_model}
          onCheckedChange={(checked) =>
            handleToggle('quota_data_track_by_model', checked)
          }
        />
      </SettingsRow>

      <SettingsRow
        label={t('systemSettings.tips.aggregateByUser')}
        description={t(
          'systemSettings.tips.aggregateQuotaDataPerUserDisablingCollapsesAllUsersInto'
        )}
      >
        <Switch
          checked={config.quota_data_track_by_user}
          onCheckedChange={(checked) =>
            handleToggle('quota_data_track_by_user', checked)
          }
        />
      </SettingsRow>
    </SettingsPanel>
  )
}
