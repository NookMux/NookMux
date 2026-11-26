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
import { useState } from 'react'
import { AlertTriangle, Loader2 } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { disable2FA } from '@/lib/api'
import { Alert, AlertDescription } from '@/components/ui/alert'
import { Button } from '@/components/ui/button'
import { Checkbox } from '@/components/ui/checkbox'
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
import { useSignInAgain } from '../../hooks'

interface TwoFADisableDialogProps {
  open: boolean
  onOpenChange: (open: boolean) => void
}

export function TwoFADisableDialog({
  open,
  onOpenChange,
}: TwoFADisableDialogProps) {
  const { t } = useTranslation()
  const signInAgain = useSignInAgain()
  const [loading, setLoading] = useState(false)
  const [code, setCode] = useState('')
  const [confirmed, setConfirmed] = useState(false)

  const handleDisable = async () => {
    if (!code) {
      toast.error(
        t('profile.errors.pleaseEnterYourVerificationCodeOrBackupCode')
      )
      return
    }

    if (!confirmed) {
      toast.error(
        t('profile.tips.pleaseConfirmThatYouUnderstandTheConsequences')
      )
      return
    }

    try {
      setLoading(true)
      const response = await disable2FA(code)

      if (response.success) {
        // 关闭后服务端会让该用户的全部会话失效，回到登录页重新登录。
        toast.success(
          t('profile.status.twoFactorAuthenticationDisabledSignInAgain')
        )
        signInAgain()
      } else {
        toast.error(response.message || t('profile.errors.failedToDisable2Fa'))
      }
    } catch (_error) {
      toast.error(t('profile.errors.failedToDisable2Fa'))
    } finally {
      setLoading(false)
    }
  }

  const handleOpenChange = (open: boolean) => {
    if (!loading) {
      if (!open) {
        setCode('')
        setConfirmed(false)
      }
      onOpenChange(open)
    }
  }

  return (
    <Dialog open={open} onOpenChange={handleOpenChange}>
      <DialogContent className='sm:max-w-md'>
        <DialogHeader>
          <DialogTitle className='text-destructive flex items-center gap-2'>
            <AlertTriangle className='h-5 w-5' />
            {t('profile.actions.disableTwoFactorAuthentication')}
          </DialogTitle>
          <DialogDescription>
            {t('profile.tips.actionWillPermanentlyRemove2FaProtectionFromYour')}
          </DialogDescription>
        </DialogHeader>

        <Alert variant='destructive'>
          <AlertTriangle className='h-4 w-4' />
          <AlertDescription>
            {t('profile.tips.warningDisabling2FaWillMakeYourAccountLess')}
          </AlertDescription>
        </Alert>

        <SettingsRow
          label={t('auth.fields.verificationCode')}
          hint={t(
            'profile.placeholders.enterYourAuthenticatorCodeOrABackupCode'
          )}
          htmlFor='two-fa-disable-code'
        >
          <SettingsControl>
            <Input
              id='two-fa-disable-code'
              value={code}
              onChange={(e) => setCode(e.target.value)}
              placeholder={t('profile.placeholders.enterCodeOrBackupCode')}
              disabled={loading}
            />
          </SettingsControl>
        </SettingsRow>

        <div className='flex items-start space-x-2'>
          <Checkbox
            id='confirm'
            checked={confirmed}
            onCheckedChange={(checked) => setConfirmed(checked === true)}
          />
          <label
            htmlFor='confirm'
            className='text-sm leading-tight font-normal peer-disabled:cursor-not-allowed peer-disabled:opacity-50'
          >
            {t('profile.tips.iUnderstandThatDisabling2FaWillRemoveAll')}
          </label>
        </div>

        <DialogFooter>
          <Button
            variant='outline'
            onClick={() => handleOpenChange(false)}
            disabled={loading}
          >
            {t('common.actions.cancel')}
          </Button>
          <Button
            variant='destructive'
            onClick={handleDisable}
            disabled={loading || !code || !confirmed}
          >
            {loading && (
              <Loader2 className='animate-spin' data-icon='inline-start' />
            )}
            {loading
              ? t('profile.tips.disabling')
              : t('profile.actions.disable2Fa')}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}
