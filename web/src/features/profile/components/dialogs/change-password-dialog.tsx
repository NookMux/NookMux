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
import { Loader2 } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { Button } from '@/components/ui/button'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import { PasswordInput } from '@/components/password-input'
import {
  SettingsControl,
  SettingsRow,
} from '@/features/system-settings/components/settings-group'
import { updateUserProfile } from '../../api'
import { useSignInAgain } from '../../hooks'

interface ChangePasswordDialogProps {
  open: boolean
  onOpenChange: (open: boolean) => void
  username: string
}

export function ChangePasswordDialog({
  open,
  onOpenChange,
  username,
}: ChangePasswordDialogProps) {
  const { t } = useTranslation()
  const signInAgain = useSignInAgain()
  const [loading, setLoading] = useState(false)
  const [formData, setFormData] = useState({
    originalPassword: '',
    newPassword: '',
    confirmPassword: '',
  })

  const handleChange = (field: string, value: string) => {
    setFormData((prev) => ({ ...prev, [field]: value }))
  }

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()

    if (!formData.originalPassword) {
      toast.error(t('profile.errors.pleaseEnterYourCurrentPassword'))
      return
    }

    if (!formData.newPassword) {
      toast.error(t('profile.errors.pleaseEnterANewPassword'))
      return
    }

    if (formData.newPassword.length < 8) {
      toast.error(t('profile.errors.passwordMustBeAtLeast8Characters'))
      return
    }

    if (formData.originalPassword === formData.newPassword) {
      toast.error(
        t('profile.errors.newPasswordMustBeDifferentFromCurrentPassword')
      )
      return
    }

    if (formData.newPassword !== formData.confirmPassword) {
      toast.error(t('profile.fields.passwordsDoNotMatch'))
      return
    }

    try {
      setLoading(true)
      const response = await updateUserProfile({
        original_password: formData.originalPassword,
        password: formData.newPassword,
      })

      if (response.success) {
        // 改密后服务端会让该用户的全部会话失效，回到登录页重新登录。
        toast.success(t('profile.status.passwordChangedSignInAgain'))
        signInAgain()
      } else {
        toast.error(
          response.message || t('profile.errors.failedToChangePassword')
        )
      }
    } catch (_error) {
      toast.error(t('profile.errors.failedToChangePassword'))
    } finally {
      setLoading(false)
    }
  }

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className='sm:max-w-md'>
        <form onSubmit={handleSubmit} className='@container space-y-4'>
          <DialogHeader>
            <DialogTitle>{t('profile.fields.changePassword')}</DialogTitle>
            <DialogDescription>
              {t('profile.tips.updateYourPasswordForAccount')}{' '}
              <strong>{username}</strong>
            </DialogDescription>
          </DialogHeader>

          <SettingsRow
            label={t('profile.fields.currentPassword')}
            htmlFor='current-password'
          >
            <SettingsControl>
              <PasswordInput
                id='current-password'
                value={formData.originalPassword}
                onChange={(e) =>
                  handleChange('originalPassword', e.target.value)
                }
                disabled={loading}
                required
                autoComplete='current-password'
              />
            </SettingsControl>
          </SettingsRow>

          <SettingsRow
            label={t('auth.fields.newPassword')}
            hint={t('profile.errors.mustBeAtLeast8Characters')}
            htmlFor='new-password'
          >
            <SettingsControl>
              <PasswordInput
                id='new-password'
                value={formData.newPassword}
                onChange={(e) => handleChange('newPassword', e.target.value)}
                disabled={loading}
                required
                minLength={8}
                autoComplete='new-password'
              />
            </SettingsControl>
          </SettingsRow>

          <SettingsRow
            label={t('profile.actions.confirmNewPassword')}
            htmlFor='confirm-password'
          >
            <SettingsControl>
              <PasswordInput
                id='confirm-password'
                value={formData.confirmPassword}
                onChange={(e) =>
                  handleChange('confirmPassword', e.target.value)
                }
                disabled={loading}
                required
                autoComplete='new-password'
              />
            </SettingsControl>
          </SettingsRow>

          <DialogFooter>
            <Button
              type='button'
              variant='outline'
              onClick={() => onOpenChange(false)}
              disabled={loading}
            >
              {t('common.actions.cancel')}
            </Button>
            <Button type='submit' disabled={loading}>
              {loading && (
                <Loader2 className='animate-spin' data-icon='inline-start' />
              )}
              {loading
                ? t('profile.tips.changing')
                : t('profile.fields.changePassword')}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  )
}
