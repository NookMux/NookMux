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
import { useMemo } from 'react'
import { ShieldCheck, KeyRound, Loader2 } from 'lucide-react'
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
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs'
import type {
  SecureVerificationState,
  VerificationMethod,
  VerificationMethods,
} from '../types'

interface SecureVerificationDialogProps {
  open: boolean
  onOpenChange: (open: boolean) => void
  methods: VerificationMethods
  state: SecureVerificationState
  onVerify: (method: VerificationMethod, code?: string) => void | Promise<void>
  onCancel: () => void
  onCodeChange: (code: string) => void
  onMethodChange: (method: VerificationMethod) => void
}

export function SecureVerificationDialog({
  open,
  onOpenChange,
  methods,
  state,
  onVerify,
  onCancel,
  onCodeChange,
  onMethodChange,
}: SecureVerificationDialogProps) {
  const { t } = useTranslation()
  const availableTabs: VerificationMethod[] = useMemo(() => {
    const policy = state.methodPolicy ?? 'any'
    const has2FA = methods.has2FA
    const hasPasskey = methods.hasPasskey && methods.passkeySupported
    if (policy === 'strongest') {
      // 按服务端 gate 规则只接受最强因子：已有 2FA 仅 2FA；仅有 passkey
      // 仅 passkey（浏览器不支持时进入不可用态，不诱导走必拒的原密码）；
      // 皆无退回原密码。
      if (has2FA) return ['2fa']
      if (methods.hasPasskey) return hasPasskey ? ['passkey'] : []
      return ['password']
    }
    const tabs: VerificationMethod[] = []
    if (has2FA) tabs.push('2fa')
    if (hasPasskey) tabs.push('passkey')
    // 无任何二因子时退路为原密码再认证（未设密码的账户由服务端明确报错）。
    if (tabs.length === 0) tabs.push('password')
    return tabs
  }, [methods, state.methodPolicy])

  const activeMethod =
    state.method ?? (availableTabs.length > 0 ? availableTabs[0] : null)

  const title =
    state.title ??
    (availableTabs.length
      ? 'Additional verification required'
      : 'Verification unavailable')

  const description =
    state.description ??
    (availableTabs.length
      ? 'Confirm your identity before accessing this sensitive action.'
      : 'Enable Two-factor Authentication or Passkey in your profile settings to continue.')

  const handleVerify = () => {
    if (!activeMethod) return
    const payload =
      activeMethod === '2fa' || activeMethod === 'password'
        ? state.code
        : undefined
    onVerify(activeMethod, payload)
  }

  const verifyDisabled =
    state.loading ||
    ((activeMethod === '2fa' || activeMethod === 'password') &&
      !state.code.trim()) ||
    (activeMethod === '2fa' && state.code.length < 6)

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent
        className='top-[8vh] max-w-[calc(100%-1.5rem)] translate-y-0 gap-0 overflow-hidden border-none p-0 shadow-xl sm:top-1/2 sm:max-w-md sm:translate-y-[-50%] sm:rounded-xl'
        showCloseButton={!state.loading}
      >
        <div className='bg-background flex max-h-[calc(100dvh-2rem)] flex-col'>
          <DialogHeader className='border-b px-6 py-5 text-left'>
            <DialogTitle className='flex items-center gap-2 text-lg font-semibold'>
              <ShieldCheck className='text-primary h-5 w-5' />
              {title}
            </DialogTitle>
            <DialogDescription className='text-left'>
              {description}
            </DialogDescription>
          </DialogHeader>

          <div className='flex-1 overflow-y-auto px-6 py-5'>
            {availableTabs.length === 0 ? (
              <div className='grid place-items-center gap-4 text-center'>
                <div className='bg-muted flex h-16 w-16 items-center justify-center rounded-2xl'>
                  <ShieldCheck className='text-muted-foreground h-8 w-8' />
                </div>
                <p className='text-muted-foreground text-sm'>
                  {t(
                    'auth.actions.enableTwoFactorAuthenticationOrPasskeyInYourProfile'
                  )}
                </p>
              </div>
            ) : (
              <Tabs
                value={activeMethod ?? availableTabs[0]}
                onValueChange={(value) =>
                  onMethodChange(value as VerificationMethod)
                }
                className='gap-4'
              >
                <TabsList>
                  {methods.has2FA && (
                    <TabsTrigger value='2fa'>
                      {t('auth.fields.authenticatorCode')}
                    </TabsTrigger>
                  )}
                  {methods.hasPasskey && methods.passkeySupported && (
                    <TabsTrigger value='passkey'>
                      {t('auth.fields.passkey')}
                    </TabsTrigger>
                  )}
                  {availableTabs.includes('password') && (
                    <TabsTrigger value='password'>
                      {t('auth.fields.currentPassword')}
                    </TabsTrigger>
                  )}
                </TabsList>

                <TabsContent value='2fa' className='space-y-3'>
                  <p className='text-muted-foreground text-sm'>
                    {t(
                      'auth.placeholders.enterThe6DigitTimeBasedOneTimePassword'
                    )}
                  </p>
                  <Input
                    inputMode='numeric'
                    maxLength={8}
                    value={state.code}
                    onChange={(event) => onCodeChange(event.target.value)}
                    placeholder={t('auth.placeholders.enterVerificationCode')}
                    disabled={state.loading}
                    autoFocus={activeMethod === '2fa'}
                    onKeyDown={(event) => {
                      if (event.key === 'Enter' && !verifyDisabled) {
                        event.preventDefault()
                        handleVerify()
                      }
                    }}
                  />
                </TabsContent>

                <TabsContent value='password' className='space-y-3'>
                  <p className='text-muted-foreground text-sm'>
                    {t('auth.tips.enterCurrentPasswordToConfirmIdentity')}
                  </p>
                  <Input
                    type='password'
                    value={state.code}
                    onChange={(event) => onCodeChange(event.target.value)}
                    placeholder={t('auth.placeholders.enterCurrentPassword')}
                    disabled={state.loading}
                    autoFocus={activeMethod === 'password'}
                    onKeyDown={(event) => {
                      if (event.key === 'Enter' && !verifyDisabled) {
                        event.preventDefault()
                        handleVerify()
                      }
                    }}
                  />
                </TabsContent>

                <TabsContent value='passkey' className='space-y-4'>
                  <div className='bg-muted/50 flex items-center justify-center rounded-lg p-4'>
                    <div className='text-muted-foreground flex items-center gap-3'>
                      <KeyRound className='text-primary h-6 w-6' />
                      <div className='text-left text-sm'>
                        <p className='text-foreground font-medium'>
                          {t('auth.actions.useYourPasskey')}
                        </p>
                        <p>
                          {t(
                            'auth.tips.promptYourDeviceToConfirmUsingBiometricsOrYour'
                          )}
                        </p>
                      </div>
                    </div>
                  </div>
                  {!methods.passkeySupported && (
                    <p className='text-destructive text-sm'>
                      {t('auth.tips.deviceDoesNotSupportPasskeyVerification')}
                    </p>
                  )}
                </TabsContent>
              </Tabs>
            )}
          </div>

          <DialogFooter className='bg-muted/30 border-t px-6 py-4 sm:flex-row sm:justify-end'>
            <Button
              type='button'
              variant='outline'
              disabled={state.loading}
              onClick={onCancel}
            >
              {t('common.actions.cancel')}
            </Button>
            <Button
              type='button'
              onClick={handleVerify}
              disabled={availableTabs.length === 0 || verifyDisabled}
            >
              {state.loading && <Loader2 className='h-4 w-4 animate-spin' />}
              {t('auth.actions.verify')}
            </Button>
          </DialogFooter>
        </div>
      </DialogContent>
    </Dialog>
  )
}
