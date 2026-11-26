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

export function DashboardPanelsSection() {
  const { t } = useTranslation()
  const { data: config, isLoading } = useDashboardConfig()
  const updateConfig = useUpdateDashboardConfig()

  const title = t('systemSettings.titles.dashboardPanels')

  if (isLoading) {
    return (
      <SettingsPanel title={title}>
        <div className='flex flex-col gap-4 py-2'>
          <Skeleton className='h-10 w-full' />
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
              'systemSettings.tips.controlWhichPanelsAreDisplayedInTheOverviewSectionOf'
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
        label={t('systemSettings.fields.apiInformationPanel')}
        description={t(
          'systemSettings.tips.displayApiEndpointInformationAndDocumentationLinks'
        )}
      >
        <Switch
          checked={config.api_info_enabled}
          onCheckedChange={(checked) =>
            handleToggle('api_info_enabled', checked)
          }
        />
      </SettingsRow>

      <SettingsRow
        label={t('systemSettings.tips.uptimeMonitoringPanel')}
        description={t(
          'systemSettings.tips.displayUptimeKumaServiceMonitoringStatusRequiresExternalApiCalls'
        )}
      >
        <Switch
          checked={config.uptime_kuma_enabled}
          onCheckedChange={(checked) =>
            handleToggle('uptime_kuma_enabled', checked)
          }
        />
      </SettingsRow>

      <SettingsRow
        label={t('systemSettings.fields.announcementsPanel')}
        description={t('systemSettings.tips.displaySystemAnnouncementsToUsers')}
      >
        <Switch
          checked={config.announcements_enabled}
          onCheckedChange={(checked) =>
            handleToggle('announcements_enabled', checked)
          }
        />
      </SettingsRow>

      <SettingsRow
        label={t('systemSettings.fields.faqPanel')}
        description={t('systemSettings.tips.displayFrequentlyAskedQuestions')}
      >
        <Switch
          checked={config.faq_enabled}
          onCheckedChange={(checked) => handleToggle('faq_enabled', checked)}
        />
      </SettingsRow>
    </SettingsPanel>
  )
}
