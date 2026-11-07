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
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from '@/components/ui/alert-dialog'
import {
  SecureVerificationDialog,
  useSecureVerification,
} from '@/features/auth/secure-verification'
import { deleteUser } from '../api'
import { ERROR_MESSAGES } from '../constants'
import { getUserActionMessage } from '../lib'
import { useUsers } from './users-provider'

export function UsersDeleteDialog() {
  const { t } = useTranslation()
  const { open, setOpen, currentRow, triggerRefresh } = useUsers()
  const [isDeleting, setIsDeleting] = useState(false)
  const {
    open: verificationOpen,
    methods: verificationMethods,
    state: verificationState,
    executeVerification,
    withVerification,
    cancel: cancelVerification,
    setCode: setVerificationCode,
    switchMethod: switchVerificationMethod,
  } = useSecureVerification({ scope: 'admin_user_manage' })

  const handleDelete = async () => {
    if (!currentRow) return

    setIsDeleting(true)
    try {
      const result = await withVerification(async () => {
        const response = await deleteUser(currentRow.id)
        if (response.success) {
          toast.success(t(getUserActionMessage('delete')))
          setOpen(null)
          triggerRefresh()
        } else {
          toast.error(response.message || t(ERROR_MESSAGES.DELETE_FAILED))
        }
        return response
      })
      if (!result) {
        // 已触发安全验证，验证通过后将自动重试。
        return
      }
    } catch (_error) {
      toast.error(t(ERROR_MESSAGES.UNEXPECTED))
    } finally {
      setIsDeleting(false)
    }
  }

  return (
    <>
      <AlertDialog
        open={open === 'delete'}
        onOpenChange={(open) => !open && setOpen(null)}
      >
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>{t('keys.tips.sure')}</AlertDialogTitle>
            <AlertDialogDescription>
              {t('users.tips.permanentlyDeleteUser')}{' '}
              <span className='font-semibold'>{currentRow?.username}</span>
              {t('keys.errors.actionCannotBeUndone')}
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel disabled={isDeleting}>
              {t('common.actions.cancel')}
            </AlertDialogCancel>
            <AlertDialogAction
              onClick={handleDelete}
              disabled={isDeleting}
              className='bg-destructive text-destructive-foreground hover:bg-destructive/90'
            >
              {isDeleting
                ? t('keys.tips.deleting')
                : t('common.actions.delete')}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>

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
