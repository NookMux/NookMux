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
import {
  SettingsGroup,
  SettingsRow,
} from '@/features/system-settings/components/settings-group'
import { AccessTokenDialog } from '../dialogs/access-token-dialog'

export function AccessTokenSection() {
  const { t } = useTranslation()
  const dialogs = useDialogs<'token'>()

  return (
    <>
      <SettingsGroup title={t('profile.titles.accessTokens')}>
        <SettingsRow
          label={t('profile.fields.accessToken')}
          description={t('profile.tips.generateAndManageYourApiAccessToken')}
          trailing
        >
          <Button
            variant='outline'
            size='sm'
            onClick={() => dialogs.open('token')}
          >
            {t('profile.actions.manage')}
          </Button>
        </SettingsRow>
      </SettingsGroup>

      <AccessTokenDialog
        open={dialogs.isOpen('token')}
        onOpenChange={(open) =>
          open ? dialogs.open('token') : dialogs.close('token')
        }
      />
    </>
  )
}
