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
import { Loader2, RefreshCw } from 'lucide-react'
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
import { CopyButton } from '@/components/copy-button'
import {
  SecureVerificationDialog,
  useSecureVerification,
} from '@/features/auth/secure-verification'
import {
  SettingsControl,
  SettingsRow,
} from '@/features/system-settings/components/settings-group'
import { useAccessToken } from '../../hooks'

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

  // 打开对话框时还没有令牌就自动生成；安全验证按需弹出。
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

          <SettingsRow
            label={t('profile.actions.useThisTokenForApiAuthentication')}
            htmlFor='system-access-token'
          >
            <SettingsControl>
              <div className='flex items-center gap-2'>
                <Input
                  id='system-access-token'
                  type='text'
                  value={token}
                  readOnly
                  className='font-mono text-xs'
                  placeholder={t('common.tips.clickGenerateToCreateAToken')}
                />
                <CopyButton
                  value={token}
                  variant='outline'
                  tooltip={t('profile.actions.copyToken')}
                  aria-label={t('profile.actions.copyToken')}
                />
              </div>
            </SettingsControl>
          </SettingsRow>

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
                <Loader2 className='animate-spin' data-icon='inline-start' />
              ) : (
                <RefreshCw data-icon='inline-start' />
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
