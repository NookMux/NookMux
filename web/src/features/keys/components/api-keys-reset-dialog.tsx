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
import { Check, Copy } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { useCopyToClipboard } from '@/hooks/use-copy-to-clipboard'
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
import { Input } from '@/components/ui/input'
import { resetApiKey } from '../api'
import { ERROR_MESSAGES, SUCCESS_MESSAGES } from '../constants'
import { useApiKeys } from './api-keys-provider'

export function ApiKeysResetDialog() {
  const { t } = useTranslation()
  const {
    open,
    setOpen,
    currentRow,
    triggerRefresh,
    setResolvedKey,
    setResolvedKeyForId,
  } = useApiKeys()
  const { copiedText, copyToClipboard } = useCopyToClipboard()
  const [isResetting, setIsResetting] = useState(false)
  const [newKey, setNewKey] = useState('')

  const handleOpenChange = (nextOpen: boolean) => {
    if (nextOpen) return
    setOpen(null)
    setNewKey('')
  }

  const handleReset = async () => {
    if (!currentRow) return

    setIsResetting(true)
    try {
      const result = await resetApiKey(currentRow.id)
      if (result.success && result.data?.key) {
        const fullKey = result.data.key
        setNewKey(fullKey)
        setResolvedKey(fullKey)
        setResolvedKeyForId(currentRow.id, fullKey)
        toast.success(t(SUCCESS_MESSAGES.API_KEY_RESET))
        triggerRefresh()
      } else {
        toast.error(result.message || t(ERROR_MESSAGES.RESET_FAILED))
      }
    } catch {
      toast.error(t(ERROR_MESSAGES.UNEXPECTED))
    } finally {
      setIsResetting(false)
    }
  }

  return (
    <AlertDialog open={open === 'reset-key'} onOpenChange={handleOpenChange}>
      <AlertDialogContent>
        <AlertDialogHeader>
          <AlertDialogTitle>{t('keys.actions.resetApiKey')}</AlertDialogTitle>
          <AlertDialogDescription>
            {newKey ? (
              t('keys.tips.apiKeyHasBeenResetThePreviousKeyIs')
            ) : (
              <>
                {t('keys.tips.resetApiKey')}{' '}
                <span className='font-semibold'>{currentRow?.name}</span>
                {t(
                  'keys.errors.previousKeyWillBecomeInvalidImmediatelyAndThisAction'
                )}
              </>
            )}
          </AlertDialogDescription>
        </AlertDialogHeader>

        {newKey && (
          <div className='space-y-2'>
            <label className='text-sm font-medium'>
              {t('keys.fields.newApiKey')}
            </label>
            <Input
              readOnly
              value={newKey}
              className='font-mono text-xs'
              onFocus={(event) => event.target.select()}
            />
          </div>
        )}

        <AlertDialogFooter>
          <AlertDialogCancel disabled={isResetting}>
            {newKey ? t('common.actions.close') : t('common.actions.cancel')}
          </AlertDialogCancel>
          {newKey ? (
            <AlertDialogAction
              onClick={(event) => {
                event.preventDefault()
                void copyToClipboard(newKey)
              }}
            >
              {copiedText === newKey ? (
                <Check className='size-4' />
              ) : (
                <Copy className='size-4' />
              )}
              {t('channels.actions.copy')}
            </AlertDialogAction>
          ) : (
            <AlertDialogAction
              onClick={(event) => {
                event.preventDefault()
                void handleReset()
              }}
              disabled={isResetting}
              className='bg-destructive text-destructive-foreground hover:bg-destructive/90'
            >
              {isResetting
                ? t('keys.tips.resetting')
                : t('common.actions.reset')}
            </AlertDialogAction>
          )}
        </AlertDialogFooter>
      </AlertDialogContent>
    </AlertDialog>
  )
}
