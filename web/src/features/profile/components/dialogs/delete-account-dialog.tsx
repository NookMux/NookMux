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
import { useNavigate } from '@tanstack/react-router'
import { AlertTriangle, Loader2 } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { useAuthStore } from '@/stores/auth-store'
import { api } from '@/lib/api'
import { clearSidebarModulesCaches } from '@/lib/nav-modules'
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
import {
  SecureVerificationDialog,
  useSecureVerification,
} from '@/features/auth/secure-verification'
import { deleteUserAccount } from '../../api'

// ============================================================================
// Delete Account Dialog Component
// ============================================================================

interface DeleteAccountDialogProps {
  open: boolean
  onOpenChange: (open: boolean) => void
  username: string
}

export function DeleteAccountDialog({
  open,
  onOpenChange,
  username,
}: DeleteAccountDialogProps) {
  const { t } = useTranslation()
  const navigate = useNavigate()
  const { reset } = useAuthStore((state) => state.auth)
  const [loading, setLoading] = useState(false)
  const [confirmation, setConfirmation] = useState('')
  const {
    open: verificationOpen,
    methods: verificationMethods,
    state: verificationState,
    executeVerification,
    withVerification,
    cancel: cancelVerification,
    setCode: setVerificationCode,
    switchMethod: switchVerificationMethod,
  } = useSecureVerification({ scope: 'account_delete' })

  const handleDelete = async () => {
    if (confirmation !== username) {
      toast.error(t('profile.tips.usernameConfirmationDoesNotMatch'))
      return
    }

    try {
      setLoading(true)
      const response = await withVerification(async () => {
        const result = await deleteUserAccount()

        if (result.success) {
          toast.success(t('profile.status.accountDeletedSuccessfully'))

          // Logout and redirect
          try {
            await api.get('/api/user/logout')
          } catch {
            // Ignore logout errors
          }

          reset()
          localStorage.removeItem('user')
          // 侧栏模块配置是登录用户/管理员受限数据，注销账号时一并清理
          clearSidebarModulesCaches()
          navigate({ to: '/sign-in' })
        } else {
          toast.error(
            result.message || t('profile.errors.failedToDeleteAccount')
          )
        }
        return result
      })
      if (!response) {
        // 已触发安全验证，验证通过后将自动重试。
        return
      }
    } catch (_error) {
      toast.error(t('profile.errors.failedToDeleteAccount'))
    } finally {
      setLoading(false)
    }
  }

  const handleOpenChange = (open: boolean) => {
    if (!loading) {
      onOpenChange(open)
      if (!open) {
        setConfirmation('')
      }
    }
  }

  return (
    <>
      <Dialog open={open} onOpenChange={handleOpenChange}>
        <DialogContent className='sm:max-w-md'>
          <DialogHeader>
            <DialogTitle className='text-destructive flex items-center gap-2'>
              <AlertTriangle className='h-5 w-5' />
              {t('profile.actions.deleteAccount')}
            </DialogTitle>
            <DialogDescription>
              {t(
                'profile.errors.actionCannotBeUndoneThisWillPermanentlyDeleteYour'
              )}
            </DialogDescription>
          </DialogHeader>

          <div className='my-6 space-y-4'>
            <Alert variant='destructive'>
              <AlertTriangle className='h-4 w-4' />
              <AlertDescription>
                {t('profile.tips.warningThisActionIsPermanentAndIrreversible')}
              </AlertDescription>
            </Alert>

            <div className='space-y-2'>
              <Label htmlFor='confirmation'>
                {t('channels.fields.type')} <strong>{username}</strong>{' '}
                {t('profile.fields.confirm')}
              </Label>
              <Input
                id='confirmation'
                type='text'
                value={confirmation}
                onChange={(e) => setConfirmation(e.target.value)}
                disabled={loading}
                placeholder={username}
                autoComplete='off'
              />
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
              variant='destructive'
              onClick={handleDelete}
              disabled={loading || confirmation !== username}
            >
              {loading && <Loader2 className='mr-2 h-4 w-4 animate-spin' />}
              {loading
                ? t('keys.tips.deleting')
                : t('profile.actions.deleteAccount')}
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
