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
import { useState, useEffect, useCallback } from 'react'
import { Loader2 } from 'lucide-react'
import { QRCodeSVG } from 'qrcode.react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { setup2FA, enable2FA } from '@/lib/api'
import { Alert, AlertDescription } from '@/components/ui/alert'
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
import { CopyButton } from '@/components/copy-button'
import {
  SecureVerificationDialog,
  useSecureVerification,
} from '@/features/auth/secure-verification'
import type { TwoFASetupData } from '../../types'

// ============================================================================
// Two-FA Setup Dialog Component
// ============================================================================

interface TwoFASetupDialogProps {
  open: boolean
  onOpenChange: (open: boolean) => void
  onSuccess: () => void
}

export function TwoFASetupDialog({
  open,
  onOpenChange,
  onSuccess,
}: TwoFASetupDialogProps) {
  const { t } = useTranslation()
  const [loading, setLoading] = useState(false)
  const [initializing, setInitializing] = useState(false)
  const [step, setStep] = useState(0)
  const [setupData, setSetupData] = useState<TwoFASetupData | null>(null)
  const [code, setCode] = useState('')
  const {
    open: verificationOpen,
    methods: verificationMethods,
    state: verificationState,
    executeVerification,
    withVerification,
    cancel: cancelVerification,
    setCode: setVerificationCode,
    switchMethod: switchVerificationMethod,
  } = useSecureVerification({ scope: '2fa_setup', methodPolicy: 'strongest' })
  // 请求闩锁：403 触发验证对话框后阻止自动初始化 effect 反复重发。
  const [setupAttempted, setSetupAttempted] = useState(false)
  const stepLabels = [
    t('profile.fields.scanQrCode'),
    t('profile.actions.saveBackupCodes'),
    t('profile.actions.verifySetup'),
  ]

  const handleSetup = useCallback(async () => {
    try {
      setInitializing(true)
      const response = await withVerification(async () => {
        const result = await setup2FA()
        if (result.success && result.data) {
          setSetupData(result.data)
          setStep(0)
        } else {
          toast.error(result.message || t('profile.errors.failedToSetup2Fa'))
          onOpenChange(false)
        }
        return result
      })
      if (!response) {
        // 已触发安全验证，验证通过后将自动重试。
        return
      }
    } catch (error) {
      // eslint-disable-next-line no-console
      console.error('Setup 2FA error:', error)
      toast.error(t('profile.errors.failedToSetup2Fa'))
      onOpenChange(false)
    } finally {
      setInitializing(false)
    }
  }, [onOpenChange, t, withVerification])

  const handleEnable = async () => {
    if (!code) {
      toast.error(t('auth.errors.pleaseEnterTheVerificationCode'))
      return
    }

    try {
      setLoading(true)
      const response = await enable2FA(code)

      if (response.success) {
        toast.success(
          t('profile.status.twoFactorAuthenticationEnabledSuccessfully')
        )
        onOpenChange(false)
        onSuccess()
        // Reset
        setStep(0)
        setCode('')
        setSetupData(null)
      } else {
        toast.error(response.message || t('profile.errors.failedToEnable2Fa'))
      }
    } catch (_error) {
      toast.error(t('profile.errors.failedToEnable2Fa'))
    } finally {
      setLoading(false)
    }
  }

  const handleOpenChange = (open: boolean) => {
    if (!loading && !initializing) {
      if (open && !setupData && !setupAttempted) {
        setSetupAttempted(true)
        handleSetup()
      }
      if (!open) {
        setStep(0)
        setCode('')
        setSetupData(null)
        setSetupAttempted(false)
      }
      onOpenChange(open)
    }
  }

  // Initialize when dialog opens；闩锁防止 403 触发验证后 effect 反复重发
  // （验证通过后由挂起闭包自动完成初始化）。
  useEffect(() => {
    if (open && !setupData && !initializing && !setupAttempted) {
      setSetupAttempted(true)
      handleSetup()
    }
  }, [open, setupData, initializing, setupAttempted, handleSetup])

  return (
    <>
      <Dialog open={open} onOpenChange={handleOpenChange}>
        <DialogContent className='sm:max-w-lg'>
          <DialogHeader>
            <DialogTitle>
              {t('profile.titles.setupTwoFactorAuthentication')}
            </DialogTitle>
            <DialogDescription>
              {t('profile.fields.step')} {step + 1} {t('profile.fields.value3')}{' '}
              {stepLabels[step]}
            </DialogDescription>
          </DialogHeader>

          <div className='space-y-4 py-4'>
            {initializing ? (
              <div className='flex flex-col items-center justify-center gap-3 py-8'>
                <div className='border-primary h-8 w-8 animate-spin rounded-full border-4 border-t-transparent' />
                <div className='text-muted-foreground text-sm'>
                  {t('profile.tips.settingUp2Fa')}
                </div>
              </div>
            ) : !setupData ? (
              <div className='flex justify-center py-8'>
                <div className='text-muted-foreground'>
                  {t('profile.errors.failedToLoadSetupData')}
                </div>
              </div>
            ) : (
              <>
                {/* Step 0: QR Code */}
                {step === 0 && (
                  <div className='space-y-4'>
                    <p className='text-muted-foreground text-sm'>
                      {t(
                        'profile.tips.scanThisQrCodeWithYourAuthenticatorAppGoogle'
                      )}
                    </p>
                    <div className='flex justify-center rounded-lg bg-white p-4'>
                      <QRCodeSVG value={setupData.qr_code_data} size={200} />
                    </div>
                    <div className='bg-muted rounded-lg p-3'>
                      <div className='flex items-center justify-between'>
                        <div>
                          <p className='text-muted-foreground text-xs'>
                            {t('profile.fields.enterThisKeyManually')}
                          </p>
                          <code className='font-mono text-sm'>
                            {setupData.secret}
                          </code>
                        </div>
                        <CopyButton
                          value={setupData.secret}
                          variant='ghost'
                          tooltip={t('profile.actions.copySecretKey')}
                          aria-label={t('profile.actions.copySecretKey')}
                        />
                      </div>
                    </div>
                  </div>
                )}

                {/* Step 1: Backup Codes */}
                {step === 1 && (
                  <div className='space-y-4'>
                    <Alert>
                      <AlertDescription>
                        {t(
                          'profile.actions.saveTheseBackupCodesInASafePlaceEach'
                        )}
                      </AlertDescription>
                    </Alert>
                    <div className='rounded-lg border p-4'>
                      <div className='grid grid-cols-2 gap-2'>
                        {setupData.backup_codes.map((code, index) => (
                          <div
                            key={index}
                            className='bg-muted rounded-md p-2 text-center font-mono text-sm'
                          >
                            {code}
                          </div>
                        ))}
                      </div>
                    </div>
                    <CopyButton
                      value={setupData.backup_codes.join('\n')}
                      variant='outline'
                      size='default'
                      className='w-full'
                      iconClassName='mr-2 size-4'
                      tooltip={t('profile.actions.copyAllBackupCodes')}
                      aria-label={t('profile.actions.copyAllBackupCodes')}
                    >
                      {t('profile.actions.copyAllCodes')}
                    </CopyButton>
                  </div>
                )}

                {/* Step 2: Verify */}
                {step === 2 && (
                  <div className='space-y-4'>
                    <div className='space-y-2'>
                      <Label htmlFor='code'>
                        {t('auth.fields.verificationCode')}
                      </Label>
                      <Input
                        id='code'
                        value={code}
                        onChange={(e) => setCode(e.target.value)}
                        placeholder={t('profile.placeholders.enter6DigitCode')}
                        maxLength={6}
                        disabled={loading}
                      />
                      <p className='text-muted-foreground text-xs'>
                        {t(
                          'profile.placeholders.enterThe6DigitCodeFromYourAuthenticatorApp'
                        )}
                      </p>
                    </div>
                  </div>
                )}
              </>
            )}
          </div>

          <DialogFooter>
            {step > 0 && (
              <Button
                variant='outline'
                onClick={() => setStep(step - 1)}
                disabled={initializing || loading}
              >
                {t('common.fields.goBack')}
              </Button>
            )}
            {step < 2 ? (
              <Button
                onClick={() => setStep(step + 1)}
                disabled={initializing || !setupData}
              >
                {t('common.fields.next')}
              </Button>
            ) : (
              <Button
                onClick={handleEnable}
                disabled={initializing || loading || !code}
              >
                {loading && <Loader2 className='mr-2 h-4 w-4 animate-spin' />}
                {loading
                  ? t('profile.tips.enabling')
                  : t('profile.actions.enable2Fa')}
              </Button>
            )}
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
