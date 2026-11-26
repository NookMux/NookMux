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
import { RefreshCw, Loader2 } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { regenerate2FABackupCodes } from '@/lib/api'
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
import { CopyButton } from '@/components/copy-button'
import {
  SettingsControl,
  SettingsRow,
} from '@/features/system-settings/components/settings-group'
import { useSignInAgain } from '../../hooks'

interface TwoFABackupDialogProps {
  open: boolean
  onOpenChange: (open: boolean) => void
}

export function TwoFABackupDialog({
  open,
  onOpenChange,
}: TwoFABackupDialogProps) {
  const { t } = useTranslation()
  const signInAgain = useSignInAgain()
  const [loading, setLoading] = useState(false)
  const [code, setCode] = useState('')
  const [backupCodes, setBackupCodes] = useState<string[]>([])
  // 恢复码轮换后服务端会让该用户的全部会话失效，看过新恢复码后关闭即回到登录页。
  const [regenerated, setRegenerated] = useState(false)

  const handleRegenerate = async () => {
    if (!code) {
      toast.error(t('profile.errors.pleaseEnterYourVerificationCode'))
      return
    }

    try {
      setLoading(true)
      const response = await regenerate2FABackupCodes(code)

      if (response.success && response.data?.backup_codes) {
        setBackupCodes(response.data.backup_codes)
        setRegenerated(true)
        toast.success(t('profile.tips.backupCodesRegeneratedSuccessfully'))
      } else {
        toast.error(
          response.message || t('profile.errors.failedToRegenerateBackupCodes')
        )
      }
    } catch (_error) {
      toast.error(t('profile.errors.failedToRegenerateBackupCodes'))
    } finally {
      setLoading(false)
    }
  }

  const handleOpenChange = (open: boolean) => {
    if (loading) return
    if (!open && regenerated) {
      toast.success(t('profile.status.backupCodesRegeneratedSignInAgain'))
      signInAgain()
      return
    }
    if (!open) {
      setCode('')
      setBackupCodes([])
    }
    onOpenChange(open)
  }

  return (
    <Dialog open={open} onOpenChange={handleOpenChange}>
      <DialogContent className='sm:max-w-md'>
        <DialogHeader>
          <DialogTitle className='flex items-center gap-2'>
            <RefreshCw className='h-5 w-5' />
            {t('profile.fields.regenerateBackupCodes')}
          </DialogTitle>
          <DialogDescription>
            {backupCodes.length > 0
              ? t('profile.fields.newBackupCodesAreReady')
              : t('profile.tips.generateNewBackupCodesForAccountRecovery')}
          </DialogDescription>
        </DialogHeader>

        <div className='space-y-4 py-2'>
          {backupCodes.length === 0 ? (
            <>
              <Alert>
                <AlertDescription>
                  {t(
                    'profile.tips.generatingNewCodesWillInvalidateAllExistingBackupCodes'
                  )}
                </AlertDescription>
              </Alert>

              <SettingsRow
                label={t('auth.fields.verificationCode')}
                htmlFor='two-fa-backup-code'
              >
                <SettingsControl>
                  <Input
                    id='two-fa-backup-code'
                    value={code}
                    onChange={(e) => setCode(e.target.value)}
                    placeholder={t(
                      'profile.placeholders.enterAuthenticatorCode'
                    )}
                    maxLength={6}
                    disabled={loading}
                  />
                </SettingsControl>
              </SettingsRow>
            </>
          ) : (
            <>
              <Alert>
                <AlertDescription>
                  {t('profile.actions.saveTheseCodesInASafePlaceEachCode')}
                </AlertDescription>
              </Alert>

              <div className='rounded-lg border p-4'>
                <div className='grid grid-cols-2 gap-2'>
                  {backupCodes.map((backupCode, index) => (
                    <div
                      key={index}
                      className='bg-muted rounded-md p-2 text-center font-mono text-sm'
                    >
                      {backupCode}
                    </div>
                  ))}
                </div>
              </div>

              <CopyButton
                value={backupCodes.join('\n')}
                variant='outline'
                size='default'
                className='w-full'
                iconClassName='mr-2 size-4'
                tooltip={t('profile.actions.copyAllBackupCodes')}
                aria-label={t('profile.actions.copyAllBackupCodes')}
              >
                {t('profile.actions.copyAllCodes')}
              </CopyButton>
            </>
          )}
        </div>

        <DialogFooter>
          {backupCodes.length === 0 ? (
            <>
              <Button
                variant='outline'
                onClick={() => handleOpenChange(false)}
                disabled={loading}
              >
                {t('common.actions.cancel')}
              </Button>
              <Button onClick={handleRegenerate} disabled={loading || !code}>
                {loading && (
                  <Loader2 className='animate-spin' data-icon='inline-start' />
                )}
                {loading
                  ? t('channels.tips.generating')
                  : t('profile.fields.generateNewCodes')}
              </Button>
            </>
          ) : (
            <Button onClick={() => handleOpenChange(false)}>
              {t('profile.fields.done')}
            </Button>
          )}
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}
