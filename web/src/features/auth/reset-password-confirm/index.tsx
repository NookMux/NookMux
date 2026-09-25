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
import { useState, type FormEvent } from 'react'
import { useNavigate } from '@tanstack/react-router'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { api } from '@/lib/api'
import { useCountdown } from '@/hooks/use-countdown'
import { Alert, AlertDescription } from '@/components/ui/alert'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { PasswordInput } from '@/components/password-input'
import { AuthLayout } from '../auth-layout'

export type ResetPasswordSearchParams = {
  email?: string
  token?: string
}

type ResetPasswordConfirmProps = ResetPasswordSearchParams

export function ResetPasswordConfirm({
  email,
  token,
}: ResetPasswordConfirmProps) {
  const { t } = useTranslation()
  const navigate = useNavigate()
  const [newPassword, setNewPassword] = useState('')
  const [confirmPassword, setConfirmPassword] = useState('')
  const [loading, setLoading] = useState(false)
  const [succeeded, setSucceeded] = useState(false)
  const {
    secondsLeft,
    isActive,
    start: startCountdown,
  } = useCountdown({ initialSeconds: 30 })

  const isValidResetLink = Boolean(email && token)

  async function handleSubmit(e: FormEvent<HTMLFormElement>) {
    e.preventDefault()
    if (!isValidResetLink || !email || !token) {
      toast.error(
        t('auth.errors.invalidResetLinkPleaseRequestANewPasswordReset')
      )
      return
    }
    if (!newPassword) {
      toast.error(t('auth.errors.pleaseEnterANewPassword'))
      return
    }
    if (newPassword.length < 8) {
      toast.error(t('auth.errors.passwordMustBeAtLeast8Characters'))
      return
    }
    if (newPassword !== confirmPassword) {
      toast.error(t('auth.errors.passwordsDoNotMatch'))
      return
    }

    startCountdown()
    setLoading(true)
    try {
      const res = await api.post(
        '/api/user/reset',
        { email, token, new_password: newPassword },
        { skipBusinessError: true } as Record<string, unknown>
      )

      if (res?.data?.success) {
        setSucceeded(true)
        toast.success(t('auth.status.resetPasswordConfirmSuccess'))
      } else {
        toast.error(
          res?.data?.message || t('auth.errors.failedToResetPassword')
        )
      }
    } catch {
      // Errors handled by global interceptor
    } finally {
      setLoading(false)
    }
  }

  return (
    <AuthLayout>
      <div className='w-full space-y-8'>
        <div className='space-y-2'>
          <h2 className='text-center text-2xl font-semibold tracking-tight sm:text-left'>
            {t('auth.actions.resetPassword')}
          </h2>
          <p className='text-muted-foreground text-left text-sm sm:text-base'>
            {succeeded
              ? t('auth.status.resetPasswordConfirmSuccess')
              : t('auth.tips.resetPasswordConfirmDescription')}
          </p>
        </div>

        {succeeded ? (
          <Button
            className='w-full'
            onClick={() => navigate({ to: '/sign-in', replace: true })}
          >
            {t('auth.actions.backToLogin')}
          </Button>
        ) : (
          <form className='space-y-4' onSubmit={handleSubmit}>
            {!isValidResetLink && (
              <Alert variant='destructive'>
                <AlertDescription>
                  {t(
                    'auth.errors.invalidResetLinkPleaseRequestANewPasswordReset896797'
                  )}
                </AlertDescription>
              </Alert>
            )}

            <div className='space-y-2'>
              <Label htmlFor='email'>{t('auth.fields.email')}</Label>
              <Input
                id='email'
                type='email'
                value={email || ''}
                disabled
                placeholder={t('auth.status.waitingForEmail')}
              />
            </div>

            <div className='space-y-2'>
              <Label htmlFor='newPassword'>
                {t('auth.fields.newPassword')}
              </Label>
              <PasswordInput
                id='newPassword'
                value={newPassword}
                onChange={(e) => setNewPassword(e.target.value)}
                disabled={loading}
                required
                minLength={8}
                maxLength={20}
                autoComplete='new-password'
              />
              <p className='text-muted-foreground text-xs'>
                {t('auth.placeholders.enterPassword820Characters')}
              </p>
            </div>

            <div className='space-y-2'>
              <Label htmlFor='confirmPassword'>
                {t('auth.fields.confirmNewPassword')}
              </Label>
              <PasswordInput
                id='confirmPassword'
                value={confirmPassword}
                onChange={(e) => setConfirmPassword(e.target.value)}
                disabled={loading}
                required
                autoComplete='new-password'
              />
            </div>

            <Button
              type='submit'
              className='w-full'
              disabled={loading || isActive || !isValidResetLink}
            >
              {isActive
                ? t('auth.tips.resetPasswordConfirmRetry', {
                    seconds: secondsLeft,
                  })
                : t('auth.tips.resetPasswordConfirmConfirm')}
            </Button>

            <Button
              type='button'
              variant='link'
              className='w-full'
              onClick={() => navigate({ to: '/sign-in', replace: true })}
            >
              {t('auth.actions.backToLogin')}
            </Button>
          </form>
        )}
      </div>
    </AuthLayout>
  )
}
