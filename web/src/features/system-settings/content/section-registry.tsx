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
import type { ContentSettings } from '../types'
import { createSectionRegistry } from '../utils/section-registry'
import { AnnouncementsSection } from './announcements-section'
import { ApiInfoSection } from './api-info-section'
import { DashboardConfigSection } from './dashboard-config-section'
import { DashboardSection } from './dashboard-section'
import { FAQSection } from './faq-section'
import { UptimeKumaSection } from './uptime-kuma-section'
import { UsageLogFieldsSection } from './usage-log-fields-section'

/**
 * Validate and coerce DataExportDefaultTime to a safe value
 */
function validateDataExportDefaultTime(value: string): 'week' | 'hour' | 'day' {
  if (value === 'week' || value === 'hour' || value === 'day') {
    return value
  }
  // Default to 'hour' if value is unexpected
  return 'hour'
}

const CONTENT_SECTIONS = [
  {
    id: 'dashboard',
    titleKey: 'systemSettings.titles.dataDashboard',
    build: (settings: ContentSettings) => (
      <DashboardSection
        defaultValues={{
          DataExportEnabled: settings.DataExportEnabled,
          DataExportInterval: settings.DataExportInterval,
          DataExportDefaultTime: validateDataExportDefaultTime(
            settings.DataExportDefaultTime
          ),
        }}
      />
    ),
  },
  {
    id: 'dashboard-config',
    titleKey: 'systemSettings.titles.dashboardConfiguration',
    build: () => <DashboardConfigSection />,
  },
  {
    id: 'announcements',
    titleKey: 'dashboard.fields.announcements',
    build: (settings: ContentSettings) => (
      <AnnouncementsSection data={settings['console_setting.announcements']} />
    ),
  },
  {
    id: 'api-info',
    titleKey: 'systemSettings.fields.apiAddresses',
    build: (settings: ContentSettings) => (
      <ApiInfoSection data={settings['console_setting.api_info']} />
    ),
  },
  {
    id: 'faq',
    titleKey: 'dashboard.fields.faq',
    build: (settings: ContentSettings) => (
      <FAQSection data={settings['console_setting.faq']} />
    ),
  },
  {
    id: 'uptime-kuma',
    titleKey: 'systemSettings.fields.uptimeKuma',
    build: (settings: ContentSettings) => (
      <UptimeKumaSection
        data={settings['console_setting.uptime_kuma_groups']}
      />
    ),
  },
  {
    id: 'usage-log-fields',
    titleKey: 'common.fields.usageLogFields',
    build: (settings: ContentSettings) => (
      <UsageLogFieldsSection
        fieldsData={settings['console_setting.usage_log_fields']}
        adminEnabled={
          settings['console_setting.usage_log_fields_admin_enabled']
        }
        userEnabled={settings['console_setting.usage_log_fields_user_enabled']}
      />
    ),
  },
] as const

type ContentSectionId = (typeof CONTENT_SECTIONS)[number]['id']

const contentRegistry = createSectionRegistry<
  ContentSectionId,
  ContentSettings
>({
  sections: CONTENT_SECTIONS,
  defaultSection: 'dashboard',
  basePath: '/system-settings/content',
  urlStyle: 'path',
})

export const CONTENT_SECTION_IDS = contentRegistry.sectionIds
export const CONTENT_DEFAULT_SECTION = contentRegistry.defaultSection
export const getContentSectionNavItems = contentRegistry.getSectionNavItems
export const getContentSectionContent = contentRegistry.getSectionContent
export const getContentSectionMeta = contentRegistry.getSectionMeta
