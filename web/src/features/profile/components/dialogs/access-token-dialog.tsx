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
import { useEffect } from 'react'
import { RefreshCw, Loader2 } from 'lucide-react'
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
import { Label } from '@/components/ui/label'
import { CopyButton } from '@/components/copy-button'
import {
  SecureVerificationDialog,
  useSecureVerification,
} from '@/features/auth/secure-verification'
import { useAccessToken } from '../../hooks'

// ============================================================================
// Access Token Dialog Component
// ============================================================================

interface AccessTokenDialogProps {
  open: boolean
  onOpenChange: (open: boolean) => void
}

export function AccessTokenDialog({
  open,
  onOpenChange,
}: AccessTokenDialogProps) {
  const { t } = useTranslation()
  const { token, generating, generate } = useAccessToken()
  const {
    open: verificationOpen,
    methods: verificationMethods,
    state: verificationState,
    executeVerification,
    withVerification,
    cancel: cancelVerification,
    setCode: setVerificationCode,
    switchMethod: switchVerificationMethod,
  } = useSecureVerification({ scope: 'access_token' })

  // Auto-generate token when dialog opens if no token exists
  useEffect(() => {
    if (open && !token) {
      void withVerification(generate)
    }
  }, [open, token, generate, withVerification])

  return (
    <>
      <Dialog open={open} onOpenChange={onOpenChange}>
        <DialogContent className='sm:max-w-md'>
          <DialogHeader>
            <DialogTitle>{t('profile.fields.accessToken')}</DialogTitle>
            <DialogDescription>
              {t(
                'common.tips.systemAccessTokenForApiAuthenticationKeepItSecure'
              )}
            </DialogDescription>
          </DialogHeader>

          <div className='my-6 space-y-4'>
            <div className='space-y-2'>
              <Label htmlFor='token'>{t('pricing.fields.token')}</Label>
              <div className='flex gap-2'>
                <Input
                  id='token'
                  type='text'
                  value={token}
                  readOnly
                  className='font-mono text-xs'
                  placeholder={t('common.tips.clickGenerateToCreateAToken')}
                />
                <CopyButton
                  value={token}
                  variant='outline'
                  className='size-9'
                  iconClassName='size-4'
                  tooltip={t('profile.actions.copyToken')}
                  aria-label={t('profile.actions.copyToken')}
                />
              </div>
              <p className='text-muted-foreground text-xs'>
                {t('profile.actions.useThisTokenForApiAuthentication')}
              </p>
            </div>
          </div>

          <DialogFooter>
            <Button
              type='button'
              variant='outline'
              onClick={() => onOpenChange(false)}
            >
              {t('common.actions.close')}
            </Button>
            <Button
              type='button'
              onClick={() => {
                void withVerification(generate)
              }}
              disabled={generating}
              className='gap-2'
            >
              {generating ? (
                <Loader2 className='h-4 w-4 animate-spin' />
              ) : (
                <RefreshCw className='h-4 w-4' />
              )}
              {generating
                ? t('channels.tips.generating')
                : t('profile.fields.regenerate')}
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
