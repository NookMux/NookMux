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
import { useCountdown } from '@/hooks/use-countdown'
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
import { Label } from '@/components/ui/label'
import {
  SecureVerificationDialog,
  useSecureVerification,
} from '@/features/auth/secure-verification'
import { sendEmailVerification, bindEmail } from '../../api'

// ============================================================================
// Email Bind Dialog Component
// ============================================================================

interface EmailBindDialogProps {
  open: boolean
  onOpenChange: (open: boolean) => void
  currentEmail?: string
  onSuccess: () => void
}

export function EmailBindDialog({
  open,
  onOpenChange,
  currentEmail,
  onSuccess,
}: EmailBindDialogProps) {
  const { t } = useTranslation()
  const [loading, setLoading] = useState(false)
  const [sendingCode, setSendingCode] = useState(false)
  const [email, setEmail] = useState('')
  const [code, setCode] = useState('')
  const {
    secondsLeft,
    isActive,
    start: startCountdown,
    reset: resetCountdown,
  } = useCountdown({
    initialSeconds: 60,
  })
  const {
    open: verificationOpen,
    methods: verificationMethods,
    state: verificationState,
    executeVerification,
    withVerification,
    cancel: cancelVerification,
    setCode: setVerificationCode,
    switchMethod: switchVerificationMethod,
  } = useSecureVerification({ scope: 'email_bind' })

  const handleSendCode = async () => {
    if (!email || !email.includes('@')) {
      toast.error(t('profile.errors.pleaseEnterAValidEmailAddress'))
      return
    }

    try {
      setSendingCode(true)
      const response = await sendEmailVerification(email)

      if (response.success) {
        toast.success(
          t('profile.tips.verificationCodeSentPleaseCheckYourEmail')
        )
        startCountdown()
      } else {
        toast.error(
          response.message || t('profile.errors.failedToSendVerificationCode')
        )
      }
    } catch (_error) {
      toast.error(t('profile.errors.failedToSendVerificationCode'))
    } finally {
      setSendingCode(false)
    }
  }

  const handleBind = async () => {
    if (!email || !code) {
      toast.error(t('profile.errors.pleaseEnterEmailAndVerificationCode'))
      return
    }

    try {
      setLoading(true)
      const response = await withVerification(async () => {
        const result = await bindEmail(email, code)

        if (result.success) {
          toast.success(t('profile.tips.emailBoundSuccessfully'))
          onOpenChange(false)
          onSuccess()
          // Reset form
          setEmail('')
          setCode('')
          resetCountdown()
        } else {
          toast.error(result.message || t('profile.errors.failedToBindEmail'))
        }
        return result
      })
      if (!response) {
        // 已触发安全验证，验证通过后将自动重试。
        return
      }
    } catch (_error) {
      toast.error(t('profile.errors.failedToBindEmail'))
    } finally {
      setLoading(false)
    }
  }

  const handleOpenChange = (open: boolean) => {
    if (!loading) {
      onOpenChange(open)
      if (!open) {
        // Reset form when closing
        setEmail('')
        setCode('')
        resetCountdown()
      }
    }
  }

  return (
    <>
      <Dialog open={open} onOpenChange={handleOpenChange}>
        <DialogContent className='sm:max-w-md'>
          <DialogHeader>
            <DialogTitle>{t('profile.actions.bindEmail')}</DialogTitle>
            <DialogDescription>
              {currentEmail
                ? t('profile.tips.currentEmailEmailEnterANewEmailToChange', {
                    email: currentEmail,
                  })
                : t('profile.actions.bindAnEmailAddressToYourAccount')}
            </DialogDescription>
          </DialogHeader>

          <div className='space-y-4 py-4'>
            <div className='space-y-2'>
              <Label htmlFor='email'>{t('profile.fields.emailAddress')}</Label>
              <Input
                id='email'
                type='email'
                value={email}
                onChange={(e) => setEmail(e.target.value)}
                placeholder={t('profile.placeholders.enterYourEmail')}
                disabled={loading}
              />
            </div>

            <div className='space-y-2'>
              <Label htmlFor='code'>{t('auth.fields.verificationCode')}</Label>
              <div className='flex gap-2'>
                <Input
                  id='code'
                  value={code}
                  onChange={(e) => setCode(e.target.value)}
                  placeholder={t('profile.placeholders.enterCode')}
                  disabled={loading}
                  maxLength={6}
                />
                <Button
                  type='button'
                  variant='outline'
                  onClick={handleSendCode}
                  disabled={sendingCode || isActive || !email}
                >
                  {isActive
                    ? `${secondsLeft}s`
                    : sendingCode
                      ? t('profile.tips.sending')
                      : t('playground.actions.send')}
                </Button>
              </div>
            </div>
          </div>

          <DialogFooter>
            <Button
              type='button'
              variant='outline'
              onClick={() => handleOpenChange(false)}
              disabled={loading}
            >
              {t('common.actions.cancel')}
            </Button>
            <Button
              type='button'
              onClick={handleBind}
              disabled={loading || !email || !code}
            >
              {loading && <Loader2 className='mr-2 h-4 w-4 animate-spin' />}
              {loading
                ? t('profile.tips.binding')
                : t('profile.actions.bindEmail')}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      <SecureVerificationDialog
        open={verificationOpen}
        onOpenChange={(open) => {
          if (!open) {
            cancelVerification()
          }
        }}
        methods={verificationMethods}
        state={verificationState}
        onVerify={async (method, code) => {
          await executeVerification(method, code)
        }}
        onCancel={cancelVerification}
        onCodeChange={setVerificationCode}
        onMethodChange={switchVerificationMethod}
      />
    </>
  )
}
