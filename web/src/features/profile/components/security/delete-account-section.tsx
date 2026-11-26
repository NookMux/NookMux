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
import { SettingsRow } from '@/features/system-settings/components/settings-group'
import { SettingsNotice } from '@/features/system-settings/components/settings-notice'
import type { UserProfile } from '../../types'
import { DeleteAccountDialog } from '../dialogs/delete-account-dialog'

interface DeleteAccountSectionProps {
  profile: UserProfile | null
}

/** 页底的删除账户：无框的高风险提示在上，操作行在下。 */
export function DeleteAccountSection({ profile }: DeleteAccountSectionProps) {
  const { t } = useTranslation()
  const dialogs = useDialogs<'delete'>()

  if (!profile) return null

  return (
    <>
      <section className='@container flex flex-col gap-4'>
        <SettingsNotice
          tone='danger'
          title={t('systemSettings.fields.highRiskOperation')}
          className='pt-0'
        >
          {t('profile.tips.deletingYourAccountPermanentlyRemovesAllData')}
        </SettingsNotice>
        <SettingsRow
          label={t('profile.actions.deleteAccount')}
          description={t('profile.tips.permanentlyDeleteYourAccountAndAllData')}
          trailing
        >
          <Button
            variant='destructive'
            size='sm'
            onClick={() => dialogs.open('delete')}
          >
            {t('profile.actions.deleteAccount')}
          </Button>
        </SettingsRow>
      </section>
      <DeleteAccountDialog
        open={dialogs.isOpen('delete')}
        onOpenChange={(open) =>
          open ? dialogs.open('delete') : dialogs.close('delete')
        }
        username={profile.username}
      />
    </>
  )
}
