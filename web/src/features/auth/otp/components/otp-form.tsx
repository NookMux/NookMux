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
import { useEffect, useRef, useState } from 'react'
import type { z } from 'zod'
import { useForm } from 'react-hook-form'
import { zodResolver } from '@hookform/resolvers/zod'
import { useSearch } from '@tanstack/react-router'
import { Loader2 } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { useAuthStore } from '@/stores/auth-store'
import { cn } from '@/lib/utils'
import { Button } from '@/components/ui/button'
import {
  Form,
  FormControl,
  FormField,
  FormItem,
  FormLabel,
  FormMessage,
  FormDescription,
} from '@/components/ui/form'
import { Input } from '@/components/ui/input'
import {
  InputOTP,
  InputOTPGroup,
  InputOTPSlot,
  InputOTPSeparator,
} from '@/components/ui/input-otp'
import { login2fa } from '@/features/auth/api'
import {
  otpFormSchema,
  OTP_LENGTH,
  BACKUP_CODE_LENGTH,
} from '@/features/auth/constants'
import { useAuthRedirect } from '@/features/auth/hooks/use-auth-redirect'
import { saveUserId } from '@/features/auth/lib/storage'
import {
  isValidOTP,
  isValidBackupCode,
  formatBackupCode,
  cleanBackupCode,
} from '@/features/auth/lib/validation'
import type { User } from '@/features/users/types'

type OtpFormProps = React.HTMLAttributes<HTMLFormElement>

export function OtpForm({ className, ...props }: OtpFormProps) {
  const { t } = useTranslation()
  const [isLoading, setIsLoading] = useState(false)
  const [useBackupCode, setUseBackupCode] = useState(false)

  const { auth } = useAuthStore()
  const { redirectToLogin } = useAuthRedirect()
  const { redirect } = useSearch({ from: '/(auth)/otp' })

  const form = useForm<z.infer<typeof otpFormSchema>>({
    resolver: zodResolver(otpFormSchema),
    defaultValues: { otp: '' },
  })

  const otp = form.watch('otp')

  async function onSubmit(data: z.infer<typeof otpFormSchema>) {
    // Validate based on mode
    if (useBackupCode) {
      if (!isValidBackupCode(data.otp)) {
        toast.error(t('auth.errors.backupCodeMustBeInFormatXxxxXxxx'))
        return
      }
    } else {
      if (!isValidOTP(data.otp)) {
        toast.error(t('auth.errors.verificationCodeMustBe6Digits'))
        return
      }
    }

    setIsLoading(true)
    try {
      // Remove all hyphens from backup code before sending to backend
      const code = useBackupCode ? cleanBackupCode(data.otp) : data.otp
      const res = await login2fa({ code })

      if (!res.success) {
        toast.error(res.message || t('auth.errors.invalidCode'))
        return
      }

      // Handle user data from 2FA login response
      const userData = res.data
      if (!userData) {
        throw new Error(t('auth.errors.noUserDataReceived'))
      }

      // Update auth store
      auth.setUser(userData as User)

      // Store user ID in localStorage for compatibility
      if (userData.id) {
        saveUserId(userData.id)
      }

      toast.success(t('auth.status.signedIn'))
      redirectToLogin(redirect)
    } catch (error) {
      // eslint-disable-next-line no-console
      console.error('2FA verification error:', error)
      const errorMessage =
        error instanceof Error
          ? error.message
          : t('auth.status.verificationFailed')
      toast.error(errorMessage)
    } finally {
      setIsLoading(false)
    }
  }

  function handleToggleMode() {
    setUseBackupCode(!useBackupCode)
    form.setValue('otp', '')
  }

  function handleBackToLogin() {
    redirectToLogin(redirect)
  }

  const isFormValid = useBackupCode
    ? otp.length >= BACKUP_CODE_LENGTH
    : otp.length >= OTP_LENGTH

  // Track whether auto-submit has already fired for the current input,
  // so we don't repeatedly fire requests while the value stays complete.
  const autoSubmittedRef = useRef(false)

  // Auto-submit when the input reaches its full valid length:
  // - OTP mode: 6 digits entered
  // - Backup code mode: full XXXX-XXXX format and passes validation
  useEffect(() => {
    const ready = useBackupCode
      ? otp.length >= BACKUP_CODE_LENGTH && isValidBackupCode(otp)
      : otp.length >= OTP_LENGTH

    if (ready && !isLoading && !autoSubmittedRef.current) {
      autoSubmittedRef.current = true
      form.handleSubmit(onSubmit)()
    }
    // Reset the guard when the user edits back below the valid threshold,
    // allowing a fresh auto-submit on the next complete input.
    if (!ready) {
      autoSubmittedRef.current = false
    }
    // onSubmit intentionally excluded: it's a non-memoized closure and we only
    // want this effect to fire on otp/mode/loading changes, not every render.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [otp, useBackupCode, isLoading])

  return (
    <Form {...form}>
      <form
        onSubmit={form.handleSubmit(onSubmit)}
        className={cn('grid gap-4', className)}
        {...props}
      >
        <FormField
          control={form.control}
          name='otp'
          render={({ field }) => (
            <FormItem>
              <FormLabel>
                {useBackupCode
                  ? t('auth.fields.backupCode')
                  : t('auth.fields.verificationCode')}
              </FormLabel>
              <FormControl>
                {useBackupCode ? (
                  <Input
                    placeholder={t(
                      'auth.placeholders.enterBackupCodeEGCawdOqdv'
                    )}
                    {...field}
                    maxLength={BACKUP_CODE_LENGTH}
                    autoComplete='off'
                    className='font-mono uppercase'
                    onChange={(e) => {
                      const formatted = formatBackupCode(e.target.value)
                      field.onChange(formatted)
                    }}
                  />
                ) : (
                  <InputOTP
                    maxLength={OTP_LENGTH}
                    {...field}
                    containerClassName='justify-between sm:[&>[data-slot="input-otp-group"]>div]:w-12'
                  >
                    <InputOTPGroup>
                      <InputOTPSlot index={0} />
                      <InputOTPSlot index={1} />
                    </InputOTPGroup>
                    <InputOTPSeparator />
                    <InputOTPGroup>
                      <InputOTPSlot index={2} />
                      <InputOTPSlot index={3} />
                    </InputOTPGroup>
                    <InputOTPSeparator />
                    <InputOTPGroup>
                      <InputOTPSlot index={4} />
                      <InputOTPSlot index={5} />
                    </InputOTPGroup>
                  </InputOTP>
                )}
              </FormControl>
              <FormDescription className='text-muted-foreground text-xs'>
                {useBackupCode
                  ? t('auth.status.eachBackupCodeCanOnlyBeUsedOnce')
                  : t('auth.tips.verificationCodeUpdatesEvery30Seconds')}
              </FormDescription>
              <FormMessage />
            </FormItem>
          )}
        />

        <Button
          type='submit'
          className='mt-2 w-full'
          disabled={!isFormValid || isLoading}
        >
          {isLoading ? <Loader2 className='h-4 w-4 animate-spin' /> : null}
          {t('auth.actions.verifyAndSignIn')}
        </Button>

        <div className='flex items-center justify-center gap-2 text-sm'>
          <Button
            type='button'
            variant='link'
            size='sm'
            className='text-primary h-auto p-0'
            onClick={handleToggleMode}
          >
            {useBackupCode
              ? t('auth.actions.useAuthenticatorCode')
              : t('auth.actions.useBackupCode')}
          </Button>
          <span className='text-muted-foreground'>·</span>
          <Button
            type='button'
            variant='link'
            size='sm'
            className='text-primary h-auto p-0'
            onClick={handleBackToLogin}
          >
            {t('auth.actions.backToLogin')}
          </Button>
        </div>
      </form>
    </Form>
  )
}
