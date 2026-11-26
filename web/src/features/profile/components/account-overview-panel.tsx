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
import { getUserAvatarFallback, getUserAvatarStyle } from '@/lib/avatar'
import { formatCompactNumber, formatQuota } from '@/lib/format'
import { getRoleLabel } from '@/lib/roles'
import { Avatar, AvatarFallback } from '@/components/ui/avatar'
import { StatusBadge } from '@/components/status-badge'
import {
  SettingsPanel,
  SettingsStat,
  SettingsStatGrid,
} from '@/features/system-settings/components/settings-panel'
import { getDisplayName } from '../lib'
import type { UserProfile } from '../types'

/** 账户概览：头像、身份与三项用量指标，只读。 */
export function AccountOverviewPanel({ profile }: { profile: UserProfile }) {
  const { t } = useTranslation()
  const displayName = getDisplayName(profile)
  const avatarName = profile.username || displayName
  const stats = [
    {
      label: t('channels.fields.currentBalance'),
      value: formatQuota(profile.quota),
      detail: t('keyQuery.fields.remainingQuota'),
    },
    {
      label: t('profile.fields.totalUsage'),
      value: formatQuota(profile.used_quota),
      detail: t('dashboard.fields.totalConsumedQuota'),
    },
    {
      label: t('profile.fields.apiRequests'),
      value: formatCompactNumber(profile.request_count),
      detail: t('dashboard.fields.totalRequestsMade'),
    },
  ]

  return (
    <SettingsPanel>
      <div className='flex min-w-0 flex-col gap-4 py-2'>
        <div className='flex min-w-0 items-center gap-4'>
          <Avatar className='size-12 rounded-xl after:rounded-xl sm:size-16 sm:rounded-2xl sm:after:rounded-2xl'>
            <AvatarFallback
              className='font-semibold'
              style={getUserAvatarStyle(avatarName)}
            >
              {getUserAvatarFallback(avatarName)}
            </AvatarFallback>
          </Avatar>
          <div className='min-w-0 flex-1 space-y-1.5'>
            <div className='flex min-w-0 flex-wrap items-center gap-2'>
              <span className='truncate text-base font-semibold tracking-tight sm:text-lg'>
                {displayName}
              </span>
              <StatusBadge
                label={getRoleLabel(profile.role)}
                variant='neutral'
                copyable={false}
              />
            </div>
            <p className='text-muted-foreground flex flex-wrap items-center gap-x-2 gap-y-0.5 text-sm'>
              <span className='truncate'>@{profile.username}</span>
              {profile.email && (
                <span className='inline-flex min-w-0 items-center gap-x-2'>
                  <span aria-hidden='true'>·</span>
                  <span className='truncate'>{profile.email}</span>
                </span>
              )}
              {profile.group && (
                <span className='inline-flex min-w-0 items-center gap-x-2'>
                  <span aria-hidden='true'>·</span>
                  <span className='truncate'>{profile.group}</span>
                </span>
              )}
            </p>
          </div>
        </div>
        <SettingsStatGrid columns={3} className='border-t pt-4'>
          {stats.map((stat) => (
            <SettingsStat
              key={stat.label}
              label={stat.label}
              value={stat.value}
              detail={stat.detail}
            />
          ))}
        </SettingsStatGrid>
      </div>
    </SettingsPanel>
  )
}
