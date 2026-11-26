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
import { Loader2 } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { Button } from '@/components/ui/button'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import { Input } from '@/components/ui/input'
import {
  SettingsControl,
  SettingsRow,
} from '@/features/system-settings/components/settings-group'

interface RenamePasskeyDialogProps {
  open: boolean
  onOpenChange: (open: boolean) => void
  name: string
  onNameChange: (name: string) => void
  submitting: boolean
  onSubmit: () => Promise<void>
}

/** 给 Passkey 改名：只有一个字段，回车与「保存」都能提交。 */
export function RenamePasskeyDialog({
  open,
  onOpenChange,
  name,
  onNameChange,
  submitting,
  onSubmit,
}: RenamePasskeyDialogProps) {
  const { t } = useTranslation()

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent>
        <form
          className='@container space-y-4'
          onSubmit={(event) => {
            event.preventDefault()
            void onSubmit()
          }}
        >
          <DialogHeader>
            <DialogTitle>{t('profile.fields.renameDevice')}</DialogTitle>
            <DialogDescription>
              {t('profile.tips.giveThisPasskeyAMemorableName')}
            </DialogDescription>
          </DialogHeader>

          <SettingsRow
            label={t('profile.fields.deviceName')}
            htmlFor='passkey-device-name'
          >
            <SettingsControl>
              <Input
                id='passkey-device-name'
                value={name}
                onChange={(event) => onNameChange(event.target.value)}
                placeholder={t('profile.placeholders.eGMyIPhoneWorkLaptop')}
                disabled={submitting}
              />
            </SettingsControl>
          </SettingsRow>

          <DialogFooter>
            <Button
              type='button'
              variant='outline'
              onClick={() => onOpenChange(false)}
              disabled={submitting}
            >
              {t('common.actions.cancel')}
            </Button>
            <Button type='submit' disabled={submitting}>
              {submitting && (
                <Loader2 className='animate-spin' data-icon='inline-start' />
              )}
              {t('channels.actions.save')}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  )
}
