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
import { useEffect, useMemo, useState } from 'react'
import { ExternalLink, Copy, Check, Loader2 } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { tryPrettyJson } from '@/lib/utils'
import { useCopyToClipboard } from '@/hooks/use-copy-to-clipboard'
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
import { completeCodexOAuth, startCodexOAuth } from '../../api'

type CodexOAuthDialogProps = {
  open: boolean
  onOpenChange: (open: boolean) => void
  onKeyGenerated: (key: string) => void
}

export function CodexOAuthDialog({
  open,
  onOpenChange,
  onKeyGenerated,
}: CodexOAuthDialogProps) {
  const { t } = useTranslation()
  const { copiedText, copyToClipboard } = useCopyToClipboard({ notify: false })

  const [state, setState] = useState({
    authorizeUrl: '',
    callbackUrl: '',
    isStarting: false,
    isCompleting: false,
  })

  useEffect(() => {
    if (!open) {
      setState({
        authorizeUrl: '',
        callbackUrl: '',
        isStarting: false,
        isCompleting: false,
      })
    }
  }, [open])

  const canCopyAuthorizeUrl = Boolean(state.authorizeUrl && !state.isStarting)
  const canComplete = useMemo(
    () => Boolean(state.callbackUrl.trim()) && !state.isCompleting,
    [state.callbackUrl, state.isCompleting]
  )

  const handleStart = async () => {
    setState((prev) => ({ ...prev, isStarting: true }))
    try {
      const res = await startCodexOAuth()
      if (!res.success) {
        throw new Error(res.message || t('channels.status.oauthStartFailed'))
      }

      const url = res.data?.authorize_url || ''
      if (!url) {
        throw new Error(t('channels.errors.missingAuthorizeUrl'))
      }

      setState((prev) => ({ ...prev, authorizeUrl: url }))
      try {
        window.open(url, '_blank', 'noopener,noreferrer')
        toast.success(t('channels.fields.openedAuthorizationPage'))
      } catch (error) {
        // eslint-disable-next-line no-console
        console.warn('Failed to open authorization page:', error)
        toast.warning(
          t('channels.tips.pleaseManuallyCopyAndOpenTheAuthorizationLink')
        )
      }
    } catch (error) {
      toast.error(
        error instanceof Error
          ? error.message
          : t('channels.status.oauthStartFailed')
      )
    } finally {
      setState((prev) => ({ ...prev, isStarting: false }))
    }
  }

  const handleComplete = async () => {
    if (!state.callbackUrl.trim()) return
    setState((prev) => ({ ...prev, isCompleting: true }))
    try {
      const res = await completeCodexOAuth(state.callbackUrl.trim())
      if (!res.success) {
        throw new Error(res.message || t('channels.status.oauthFailed'))
      }

      const rawKey = res.data?.key || ''
      if (!rawKey) {
        throw new Error(t('channels.errors.missingKeyInResponse'))
      }

      onKeyGenerated(tryPrettyJson(rawKey))
      toast.success(t('channels.fields.credentialGenerated'))
      onOpenChange(false)
    } catch (error) {
      toast.error(
        error instanceof Error
          ? error.message
          : t('channels.status.oauthFailed')
      )
    } finally {
      setState((prev) => ({ ...prev, isCompleting: false }))
    }
  }

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className='sm:max-w-2xl'>
        <DialogHeader>
          <DialogTitle>{t('channels.fields.codexAuthorization')}</DialogTitle>
          <DialogDescription>
            {t('channels.tips.generateACodexOauthCredentialAndPasteItInto')}
          </DialogDescription>
        </DialogHeader>

        <div className='space-y-4'>
          <Alert>
            <AlertDescription>
              {t(
                'common.placeholders.value1ClickOpenAuthorizationPageAndCompleteLogin'
              )}
            </AlertDescription>
          </Alert>

          <div className='flex flex-wrap gap-2'>
            <Button onClick={handleStart} disabled={state.isStarting}>
              {state.isStarting ? (
                <Loader2 className='mr-2 h-4 w-4 animate-spin' />
              ) : (
                <ExternalLink className='mr-2 h-4 w-4' />
              )}
              {t('channels.actions.openAuthorizationPage')}
            </Button>

            <Button
              type='button'
              variant='outline'
              disabled={!canCopyAuthorizeUrl}
              onClick={async () => {
                if (!state.authorizeUrl) return
                await copyToClipboard(state.authorizeUrl)
              }}
              aria-label={t('channels.actions.copyAuthorizationLink')}
              title={t('channels.actions.copyAuthorizationLink')}
            >
              {copiedText === state.authorizeUrl ? (
                <Check className='mr-2 h-4 w-4 text-green-600' />
              ) : (
                <Copy className='mr-2 h-4 w-4' />
              )}
              {t('channels.actions.copyAuthorizationLink')}
            </Button>
          </div>

          <div className='space-y-2'>
            <div className='text-sm font-medium'>
              {t('channels.fields.callbackUrl')}
            </div>
            <Input
              value={state.callbackUrl}
              onChange={(e) =>
                setState((prev) => ({ ...prev, callbackUrl: e.target.value }))
              }
              placeholder={t(
                'channels.tips.pasteTheFullCallbackUrlIncludesCodeState'
              )}
              autoComplete='off'
              spellCheck={false}
            />
            <div className='text-muted-foreground text-xs'>
              {t('channels.tips.tipTheGeneratedKeyIsAJsonCredentialIncluding')}
            </div>
          </div>
        </div>

        <DialogFooter>
          <Button
            type='button'
            variant='outline'
            onClick={() => onOpenChange(false)}
            disabled={state.isStarting || state.isCompleting}
          >
            {t('common.actions.cancel')}
          </Button>
          <Button onClick={handleComplete} disabled={!canComplete}>
            {state.isCompleting && (
              <Loader2 className='mr-2 h-4 w-4 animate-spin' />
            )}
            {state.isCompleting
              ? t('channels.tips.generating')
              : t('channels.fields.generateCredential')}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}
