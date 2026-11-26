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
import { SettingsPageFrame } from '@/features/system-settings/components/settings-page'
import { AccessTokenSection } from './components/security/access-token-section'
import { DeleteAccountSection } from './components/security/delete-account-section'
import { LoginMethodsSection } from './components/security/login-methods-section'
import { PasskeySection } from './components/security/passkey-section'
import { TwoFASection } from './components/security/two-fa-section'
import { useProfile, useTwoFA } from './hooks'

/** 安全与访问：登录方式、Passkey、两步验证、访问令牌，以及页面底部的删除账户。 */
export function ProfileSecurity() {
  const { t } = useTranslation()
  const { profile, loading, failed, fetchProfile, refreshProfile } =
    useProfile()
  const twoFA = useTwoFA()

  return (
    <SettingsPageFrame title={t('layout.titles.securityAccess')}>
      <LoginMethodsSection
        profile={profile}
        loading={loading}
        failed={failed}
        onRetry={fetchProfile}
        onProfileUpdate={refreshProfile}
      />
      <PasskeySection />
      <TwoFASection twoFA={twoFA} />
      <AccessTokenSection />
      <DeleteAccountSection profile={profile} />
    </SettingsPageFrame>
  )
}
