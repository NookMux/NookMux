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
import {
  Activity,
  Box,
  FileText,
  Files,
  FlaskConical,
  Gauge,
  Key,
  LayoutDashboard,
  LifeBuoy,
  Radio,
  ReceiptText,
  Settings,
  ShieldCheck,
  SlidersHorizontal,
  Ticket,
  User,
  Users,
  Wallet,
  WandSparkles,
} from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { type SidebarData } from '@/components/layout/types'

/**
 * Root navigation groups for the application sidebar.
 *
 * These are shown when the URL does not match any nested sidebar view
 * registered in `layout/lib/sidebar-view-registry.ts`.
 */
export function useSidebarData(): SidebarData {
  const { t } = useTranslation()

  return {
    navGroups: [
      {
        id: 'chat',
        title: t('common.fields.experienceCenter'),
        items: [
          {
            title: t('systemSettings.titles.playground'),
            url: '/playground',
            icon: FlaskConical,
          },
          {
            title: t('common.fields.multimodal'),
            icon: WandSparkles,
            items: [
              {
                title: t('multimodal.fields.customVoice'),
                url: '/multimodal/custom-voice',
              },
            ],
          },
        ],
      },
      {
        id: 'general',
        title: t('common.fields.general'),
        items: [
          {
            title: t('common.titles.overview'),
            url: '/dashboard/overview',
            icon: Activity,
          },
          {
            title: t('systemSettings.titles.dashboard'),
            url: '/dashboard/models',
            icon: LayoutDashboard,
          },
          {
            title: t('dashboard.fields.apiKeys'),
            url: '/keys',
            icon: Key,
          },
          {
            title: t('dashboard.titles.usageLogs'),
            url: '/usage-logs/common',
            icon: FileText,
          },
          {
            title: t('multimodalFiles.fields.files'),
            url: '/multimodal-files',
            icon: Files,
          },
        ],
      },
      {
        id: 'personal',
        title: t('common.fields.personal'),
        items: [
          {
            title: t('layout.titles.wallet'),
            url: '/wallet',
            icon: Wallet,
          },
          {
            title: t('orderQuery.titles.query'),
            url: '/order-query',
            icon: ReceiptText,
          },
          {
            title: t('layout.titles.profile'),
            url: '/profile',
            icon: User,
          },
        ],
      },
      {
        id: 'support',
        title: t('common.fields.support'),
        items: [
          {
            title: t('systemSettings.fields.tickets'),
            url: '/ticket',
            icon: LifeBuoy,
          },
        ],
      },
      {
        id: 'admin',
        title: t('systemSettings.fields.admin'),
        items: [
          {
            title: t('dynamicRatio.fields.ratio'),
            url: '/dynamic-ratio',
            icon: Gauge,
          },
          {
            title: t('channels.titles.value'),
            url: '/channels',
            icon: Radio,
          },
          {
            title: t('channels.titles.models'),
            url: '/models/metadata',
            icon: Box,
          },
          {
            title: t('systemSettings.titles.users'),
            url: '/users',
            icon: Users,
          },
          {
            title: t('redemptionCodes.fields.codes'),
            url: '/redemption-codes',
            icon: Ticket,
          },
          {
            title: t('auditLogs.titles.logs'),
            url: '/audit-logs',
            icon: ShieldCheck,
          },
          {
            title: t('layout.titles.moreSettings'),
            icon: SlidersHorizontal,
            items: [
              {
                title: t('voiceManagement.titles.value'),
                url: '/voice-management',
              },
            ],
          },
          {
            title: t('common.titles.systemSettings'),
            url: '/system-settings/site',
            activeUrls: ['/system-settings'],
            icon: Settings,
          },
        ],
      },
    ],
  }
}
