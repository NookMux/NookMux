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
import { CircleAlert } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { useStatus } from '@/hooks/use-status'
import { Button } from '@/components/ui/button'
import {
  Empty,
  EmptyHeader,
  EmptyMedia,
  EmptyTitle,
} from '@/components/ui/empty'
import { SettingsPageFrame } from '@/features/system-settings/components/settings-page'
import { AccountOverviewPanel } from './components/account-overview-panel'
import { CheckinPanel } from './components/checkin-panel'
import { NotificationForm } from './components/notification-form'
import { ProfileSkeleton } from './components/profile-skeleton'
import { useProfile } from './hooks'

/** 个人资料：账户概览、通知设置、每日签到。登录方式与危险操作在「安全与访问」页。 */
export function Profile() {
  const { t } = useTranslation()
  const { profile, loading, failed, fetchProfile, refreshProfile } =
    useProfile()
  const { status } = useStatus()

  const checkinEnabled = status?.checkin_enabled === true
  const turnstileEnabled = !!(
    status?.turnstile_check && status?.turnstile_site_key
  )
  const turnstileSiteKey = status?.turnstile_site_key || ''

  return (
    <SettingsPageFrame title={t('layout.titles.profile')}>
      {loading ? (
        <ProfileSkeleton />
      ) : failed || !profile ? (
        <Empty>
          <EmptyHeader>
            <EmptyMedia variant='icon'>
              <CircleAlert />
            </EmptyMedia>
            <EmptyTitle>{t('profile.errors.failedToLoadProfile')}</EmptyTitle>
          </EmptyHeader>
          <Button
            variant='outline'
            size='sm'
            onClick={() => void fetchProfile()}
          >
            {t('common.actions.retry')}
          </Button>
        </Empty>
      ) : (
        <>
          <AccountOverviewPanel profile={profile} />
          <NotificationForm profile={profile} onSaved={refreshProfile} />
        </>
      )}
      {checkinEnabled && (
        <CheckinPanel
          turnstileEnabled={turnstileEnabled}
          turnstileSiteKey={turnstileSiteKey}
        />
      )}
    </SettingsPageFrame>
  )
}
