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
import { useDialogs } from '@/hooks/use-dialog'
import { Button } from '@/components/ui/button'
import { StatusBadge } from '@/components/status-badge'
import {
  SettingsGroup,
  SettingsRow,
} from '@/features/system-settings/components/settings-group'
import { type TwoFAState } from '../../hooks'
import { TwoFABackupDialog } from '../dialogs/two-fa-backup-dialog'
import { TwoFADisableDialog } from '../dialogs/two-fa-disable-dialog'
import { TwoFASetupDialog } from '../dialogs/two-fa-setup-dialog'
import { SectionSkeleton } from './section-skeleton'

interface TwoFASectionProps {
  twoFA: TwoFAState
}

/**
 * 两步验证：未启用时标题行右端是「启用」；启用后说明里显示剩余恢复码，
 * 下方两行可重新生成恢复码、禁用两步验证。
 */
export function TwoFASection({ twoFA }: TwoFASectionProps) {
  const { t } = useTranslation()
  const dialogs = useDialogs<'setup' | 'backup' | 'disable'>()
  const { status, loading, refetch } = twoFA
  const enabled = status.enabled

  return (
    <>
      <SettingsGroup
        title={
          <span className='inline-flex flex-wrap items-center gap-2'>
            {t('profile.fields.twoFactorAuthentication')}
            {status.locked && (
              <StatusBadge
                label={t('profile.fields.locked')}
                variant='danger'
                copyable={false}
              />
            )}
          </span>
        }
        description={
          enabled
            ? t('profile.tips.backupCodesRemainingCount', {
                count: status.backup_codes_remaining,
              })
            : t('profile.actions.addAnExtraLayerOfSecurityToYourAccount')
        }
        actions={
          enabled || loading ? undefined : (
            <Button
              variant='outline'
              size='sm'
              onClick={() => dialogs.open('setup')}
            >
              {t('channels.actions.enable')}
            </Button>
          )
        }
      >
        {loading ? (
          <SectionSkeleton rows={1} />
        ) : enabled ? (
          <>
            <SettingsRow
              label={t('profile.fields.backupCodes')}
              description={t(
                'profile.tips.generateNewBackupCodesForAccountRecovery'
              )}
              trailing
            >
              <Button
                variant='outline'
                size='sm'
                onClick={() => dialogs.open('backup')}
              >
                {t('profile.fields.regenerateBackupCodes')}
              </Button>
            </SettingsRow>
            <SettingsRow
              label={t('profile.actions.disableTwoFactorAuthentication')}
              description={t('profile.tips.disable2FaDescription')}
              trailing
            >
              <Button
                variant='destructive'
                size='sm'
                onClick={() => dialogs.open('disable')}
              >
                {t('profile.actions.disable2Fa')}
              </Button>
            </SettingsRow>
          </>
        ) : null}
      </SettingsGroup>
      <TwoFASetupDialog
        open={dialogs.isOpen('setup')}
        onOpenChange={(open) =>
          open ? dialogs.open('setup') : dialogs.close('setup')
        }
        onSuccess={refetch}
      />
      <TwoFABackupDialog
        open={dialogs.isOpen('backup')}
        onOpenChange={(open) =>
          open ? dialogs.open('backup') : dialogs.close('backup')
        }
      />
      <TwoFADisableDialog
        open={dialogs.isOpen('disable')}
        onOpenChange={(open) =>
          open ? dialogs.open('disable') : dialogs.close('disable')
        }
      />
    </>
  )
}
