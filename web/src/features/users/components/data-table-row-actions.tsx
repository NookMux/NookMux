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
import {
  MoreHorizontal,
  Pencil,
  Trash2,
  Power,
  PowerOff,
  ArrowUp,
  ArrowDown,
  KeyRound,
  ShieldAlert,
} from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { type Row } from '@/lib/tanstack-table'
import { Button } from '@/components/ui/button'
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuShortcut,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu'
import { ConfirmDialog } from '@/components/confirm-dialog'
import { manageUser, resetUserPasskey, resetUserTwoFA } from '../api'
import {
  USER_STATUS,
  USER_ROLE,
  ERROR_MESSAGES,
  isUserDeleted,
} from '../constants'
import { getUserActionMessage } from '../lib'
import { type User, type ManageUserAction } from '../types'
import { useUsers } from './users-provider'

interface DataTableRowActionsProps {
  row: Row<User>
}

export function DataTableRowActions({ row }: DataTableRowActionsProps) {
  const { t } = useTranslation()
  const user = row.original
  const { setOpen, setCurrentRow, triggerRefresh } = useUsers()
  const [resetPasskeyOpen, setResetPasskeyOpen] = useState(false)
  const [resetTwoFAOpen, setResetTwoFAOpen] = useState(false)

  const handleEdit = () => {
    setCurrentRow(user)
    setOpen('update')
  }

  const handleDelete = () => {
    setCurrentRow(user)
    setOpen('delete')
  }

  const handleManage = async (action: Exclude<ManageUserAction, 'delete'>) => {
    try {
      const result = await manageUser(user.id, action)
      if (result.success) {
        toast.success(t(getUserActionMessage(action)))
        triggerRefresh()
      } else {
        toast.error(
          result.message || t('users.errors.failedToActionUser', { action })
        )
      }
    } catch (_error) {
      toast.error(t(ERROR_MESSAGES.UNEXPECTED))
    }
  }

  const handleResetPasskey = async () => {
    try {
      const result = await resetUserPasskey(user.id)
      if (result.success) {
        toast.success(t('users.fields.passkeyResetSuccessfully'))
        triggerRefresh()
      } else {
        toast.error(result.message || t('users.errors.failedToResetPasskey'))
      }
    } catch (_error) {
      toast.error(t(ERROR_MESSAGES.UNEXPECTED))
    } finally {
      setResetPasskeyOpen(false)
    }
  }

  const handleResetTwoFA = async () => {
    try {
      const result = await resetUserTwoFA(user.id)
      if (result.success) {
        toast.success(t('users.fields.twoFactorAuthenticationReset'))
        triggerRefresh()
      } else {
        toast.error(result.message || t('users.errors.failedToReset2Fa'))
      }
    } catch (_error) {
      toast.error(t(ERROR_MESSAGES.UNEXPECTED))
    } finally {
      setResetTwoFAOpen(false)
    }
  }

  const isDisabled = user.status === USER_STATUS.DISABLED
  const isAdmin = user.role >= USER_ROLE.ADMIN
  const isRoot = user.role === USER_ROLE.ROOT

  if (isUserDeleted(user)) {
    return null
  }

  return (
    <>
      <DropdownMenu>
        <DropdownMenuTrigger
          render={
            <Button
              variant='ghost'
              className='data-popup-open:bg-muted flex h-8 w-8 p-0'
            />
          }
        >
          <MoreHorizontal className='h-4 w-4' />
          <span className='sr-only'>{t('channels.actions.openMenu')}</span>
        </DropdownMenuTrigger>
        <DropdownMenuContent align='end' className='w-[180px]'>
          <DropdownMenuItem onClick={handleEdit}>
            {t('channels.actions.edit')}
            <DropdownMenuShortcut>
              <Pencil size={16} />
            </DropdownMenuShortcut>
          </DropdownMenuItem>

          <DropdownMenuSeparator />

          {isDisabled ? (
            <DropdownMenuItem onClick={() => handleManage('enable')}>
              {t('channels.actions.enable')}
              <DropdownMenuShortcut>
                <Power size={16} />
              </DropdownMenuShortcut>
            </DropdownMenuItem>
          ) : (
            <DropdownMenuItem
              onClick={() => handleManage('disable')}
              disabled={isRoot}
            >
              {t('channels.actions.disable')}
              <DropdownMenuShortcut>
                <PowerOff size={16} />
              </DropdownMenuShortcut>
            </DropdownMenuItem>
          )}

          {isAdmin && !isRoot && (
            <DropdownMenuItem onClick={() => handleManage('demote')}>
              {t('users.fields.demote')}
              <DropdownMenuShortcut>
                <ArrowDown size={16} />
              </DropdownMenuShortcut>
            </DropdownMenuItem>
          )}

          {!isAdmin && (
            <DropdownMenuItem onClick={() => handleManage('promote')}>
              {t('users.fields.promote')}
              <DropdownMenuShortcut>
                <ArrowUp size={16} />
              </DropdownMenuShortcut>
            </DropdownMenuItem>
          )}

          <DropdownMenuSeparator />

          <DropdownMenuItem
            onSelect={(event) => {
              event.preventDefault()
              setResetPasskeyOpen(true)
            }}
            disabled={isRoot}
          >
            {t('users.actions.resetPasskey')}
            <DropdownMenuShortcut>
              <KeyRound size={16} />
            </DropdownMenuShortcut>
          </DropdownMenuItem>

          <DropdownMenuItem
            onSelect={(event) => {
              event.preventDefault()
              setResetTwoFAOpen(true)
            }}
            disabled={isRoot}
          >
            {t('users.actions.resetTwoFactorAuthentication')}
            <DropdownMenuShortcut>
              <ShieldAlert size={16} />
            </DropdownMenuShortcut>
          </DropdownMenuItem>

          <DropdownMenuSeparator />

          <DropdownMenuItem
            onClick={handleDelete}
            className='text-destructive focus:text-destructive'
            disabled={isRoot}
          >
            {t('common.actions.delete')}
            <DropdownMenuShortcut>
              <Trash2 size={16} />
            </DropdownMenuShortcut>
          </DropdownMenuItem>
        </DropdownMenuContent>
      </DropdownMenu>

      <ConfirmDialog
        open={resetPasskeyOpen}
        onOpenChange={setResetPasskeyOpen}
        title={t('users.actions.resetPasskey')}
        desc={`Reset Passkey for ${user.username}? The user will need to register a new Passkey before using passwordless login.`}
        confirmText={t('users.actions.resetPasskey')}
        handleConfirm={handleResetPasskey}
      />

      <ConfirmDialog
        open={resetTwoFAOpen}
        onOpenChange={setResetTwoFAOpen}
        title={t('users.actions.resetTwoFactorAuthentication')}
        desc={`Reset 2FA for ${user.username}? The user must set up 2FA again to continue using it.`}
        confirmText={t('users.actions.resetTwoFactorAuthentication')}
        handleConfirm={handleResetTwoFA}
      />
    </>
  )
}
