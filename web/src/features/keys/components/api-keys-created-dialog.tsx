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
import { Check, Copy, KeyRound } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { useCopyToClipboard } from '@/hooks/use-copy-to-clipboard'
import { Button } from '@/components/ui/button'
import {
  Dialog,
  DialogContent,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import { Textarea } from '@/components/ui/textarea'
import { useApiKeys } from './api-keys-provider'

export function ApiKeysCreatedDialog() {
  const { t } = useTranslation()
  const { open, setOpen, createdKeys, setCreatedKeys } = useApiKeys()
  const { copiedText, copyToClipboard } = useCopyToClipboard()
  const content = createdKeys.join('\n')

  const handleOpenChange = (nextOpen: boolean) => {
    if (nextOpen) return
    setOpen(null)
    setCreatedKeys([])
  }

  return (
    <Dialog open={open === 'created-keys'} onOpenChange={handleOpenChange}>
      <DialogContent className='sm:max-w-xl'>
        <DialogHeader>
          <DialogTitle className='flex items-center gap-2'>
            <KeyRound className='size-4' aria-hidden='true' />
            {t('keys.fields.newApiKey')}
          </DialogTitle>
        </DialogHeader>

        <Textarea
          readOnly
          value={content}
          rows={Math.min(Math.max(createdKeys.length, 3), 8)}
          className='font-mono text-xs'
          onFocus={(event) => event.target.select()}
        />

        <DialogFooter>
          <Button
            type='button'
            variant='outline'
            onClick={() => copyToClipboard(content)}
            disabled={createdKeys.length === 0}
          >
            {copiedText === content ? (
              <Check className='size-4' />
            ) : (
              <Copy className='size-4' />
            )}
            {t('channels.actions.copy')}
          </Button>
          <Button type='button' onClick={() => handleOpenChange(false)}>
            {t('common.actions.close')}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}
