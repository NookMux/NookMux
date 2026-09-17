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
import { useCallback, useState } from 'react'
import {
  Trash2,
  Edit,
  Power,
  PowerOff,
  ArrowRightLeft,
  Copy,
  Link,
  Loader2,
  RotateCcw,
  MoreHorizontal as DotsHorizontalIcon,
} from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { copyToClipboard } from '@/lib/copy-to-clipboard'
import { type RowData, type Row } from '@/lib/tanstack-table'
import { Button } from '@/components/ui/button'
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuShortcut,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu'
import {
  Tooltip,
  TooltipContent,
  TooltipTrigger,
} from '@/components/ui/tooltip'
import { updateApiKeyStatus } from '../api'
import { API_KEY_STATUS, ERROR_MESSAGES, SUCCESS_MESSAGES } from '../constants'
import { apiKeySchema } from '../types'
import { useApiKeys } from './api-keys-provider'

function getServerAddress(): string {
  try {
    const raw = localStorage.getItem('status')
    if (raw) {
      const status = JSON.parse(raw)
      if (status.server_address) return status.server_address as string
    }
  } catch {
    /* empty */
  }
  return window.location.origin
}

function encodeConnectionString(key: string, url: string): string {
  return JSON.stringify({
    _type: 'newapi_channel_conn',
    key,
    url,
  })
}

type DataTableRowActionsProps<TData extends RowData> = {
  row: Row<TData>
}

export function DataTableRowActions<TData extends RowData>({
  row,
}: DataTableRowActionsProps<TData>) {
  const { t } = useTranslation()
  const apiKey = apiKeySchema.parse(row.original)
  const {
    setOpen,
    setCurrentRow,
    triggerRefresh,
    setResolvedKey,
    resolveRealKey,
    resolvedKeys,
    loadingKeys,
  } = useApiKeys()
  const isEnabled = apiKey.status === API_KEY_STATUS.ENABLED
  const [isTogglingStatus, setIsTogglingStatus] = useState(false)
  const resolvedRealKey = resolvedKeys[apiKey.id]
  const isRealKeyLoading = Boolean(loadingKeys[apiKey.id])

  const handleMenuOpenChange = useCallback(
    (open: boolean) => {
      if (open && !resolvedRealKey && !isRealKeyLoading) {
        void resolveRealKey(apiKey.id)
      }
    },
    [apiKey.id, isRealKeyLoading, resolvedRealKey, resolveRealKey]
  )

  const getRealKeyOnDemand = useCallback(async () => {
    if (resolvedRealKey) return resolvedRealKey
    return resolveRealKey(apiKey.id)
  }, [apiKey.id, resolvedRealKey, resolveRealKey])

  const handleToggleStatus = async (
    e?: React.MouseEvent<HTMLButtonElement>
  ) => {
    e?.stopPropagation()
    const newStatus = isEnabled
      ? API_KEY_STATUS.DISABLED
      : API_KEY_STATUS.ENABLED

    setIsTogglingStatus(true)
    try {
      const result = await updateApiKeyStatus(apiKey.id, newStatus)
      if (result.success) {
        const message = isEnabled
          ? t(SUCCESS_MESSAGES.API_KEY_DISABLED)
          : t(SUCCESS_MESSAGES.API_KEY_ENABLED)
        toast.success(message)
        triggerRefresh()
      } else {
        toast.error(result.message || t(ERROR_MESSAGES.STATUS_UPDATE_FAILED))
      }
    } catch {
      toast.error(t(ERROR_MESSAGES.UNEXPECTED))
    } finally {
      setIsTogglingStatus(false)
    }
  }

  return (
    <div className='flex items-center justify-end gap-1'>
      <Tooltip>
        <TooltipTrigger
          render={
            <Button
              variant='ghost'
              size='icon-sm'
              onClick={handleToggleStatus}
              disabled={isTogglingStatus}
              aria-label={
                isEnabled
                  ? t('channels.actions.disable')
                  : t('channels.actions.enable')
              }
              className={
                isEnabled
                  ? 'text-destructive hover:text-destructive'
                  : 'text-emerald-600 hover:text-emerald-600 dark:text-emerald-400 dark:hover:text-emerald-400'
              }
            />
          }
        >
          {isTogglingStatus ? (
            <Loader2 className='size-4 animate-spin' />
          ) : isEnabled ? (
            <PowerOff className='size-4' />
          ) : (
            <Power className='size-4' />
          )}
        </TooltipTrigger>
        <TooltipContent>
          {isEnabled
            ? t('channels.actions.disable')
            : t('channels.actions.enable')}
        </TooltipContent>
      </Tooltip>

      <Tooltip>
        <TooltipTrigger
          render={
            <Button
              variant='ghost'
              size='icon-sm'
              onClick={() => {
                setCurrentRow(apiKey)
                setOpen('update')
              }}
              aria-label={t('channels.actions.edit')}
            />
          }
        >
          <Edit className='size-4' />
        </TooltipTrigger>
        <TooltipContent>{t('channels.actions.edit')}</TooltipContent>
      </Tooltip>

      <Tooltip>
        <TooltipTrigger
          render={
            <Button
              variant='ghost'
              size='icon-sm'
              onClick={() => {
                setCurrentRow(apiKey)
                setOpen('reset-key')
              }}
              aria-label={t('keys.actions.resetKey')}
            />
          }
        >
          <RotateCcw className='size-4' />
        </TooltipTrigger>
        <TooltipContent>{t('keys.actions.resetKey')}</TooltipContent>
      </Tooltip>

      <DropdownMenu modal={false} onOpenChange={handleMenuOpenChange}>
        <DropdownMenuTrigger
          render={
            <Button
              variant='ghost'
              className='data-popup-open:bg-muted flex h-8 w-8 p-0'
            />
          }
        >
          <DotsHorizontalIcon className='h-4 w-4' />
          <span className='sr-only'>{t('channels.actions.openMenu')}</span>
        </DropdownMenuTrigger>
        <DropdownMenuContent align='end' className='w-[200px]'>
          <DropdownMenuItem
            onClick={async () => {
              const realKey = await getRealKeyOnDemand()
              if (!realKey) return
              const ok = await copyToClipboard(realKey)
              if (ok) {
                toast.success(t('common.status.copied'))
              } else {
                toast.error(t('keyQuery.actions.copyFailed'))
              }
            }}
          >
            {t('keys.actions.copyKey')}
            <DropdownMenuShortcut>
              <Copy size={16} />
            </DropdownMenuShortcut>
          </DropdownMenuItem>
          <DropdownMenuItem
            onClick={async () => {
              const realKey = await getRealKeyOnDemand()
              if (!realKey) return
              const connStr = encodeConnectionString(
                realKey,
                getServerAddress()
              )
              const ok = await copyToClipboard(connStr)
              if (ok) {
                toast.success(t('common.status.copied'))
              } else {
                toast.error(t('keyQuery.actions.copyFailed'))
              }
            }}
          >
            {t('keys.actions.copyConnectionInfo')}
            <DropdownMenuShortcut>
              <Link size={16} />
            </DropdownMenuShortcut>
          </DropdownMenuItem>
          <DropdownMenuSeparator />
          <DropdownMenuItem
            onClick={async () => {
              const realKey = await resolveRealKey(apiKey.id)
              if (!realKey) return
              setResolvedKey(realKey)
              setCurrentRow(apiKey)
              setOpen('cc-switch')
            }}
          >
            {t('keys.fields.ccSwitch')}
            <DropdownMenuShortcut>
              <ArrowRightLeft size={16} />
            </DropdownMenuShortcut>
          </DropdownMenuItem>
          <DropdownMenuSeparator />
          <DropdownMenuItem
            onClick={() => {
              setCurrentRow(apiKey)
              setOpen('delete')
            }}
            className='text-destructive focus:text-destructive'
          >
            {t('common.actions.delete')}
            <DropdownMenuShortcut>
              <Trash2 size={16} />
            </DropdownMenuShortcut>
          </DropdownMenuItem>
        </DropdownMenuContent>
      </DropdownMenu>
    </div>
  )
}
