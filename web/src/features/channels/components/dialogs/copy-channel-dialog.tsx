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
import { useQueryClient } from '@tanstack/react-query'
import { Loader2 } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { Button } from '@/components/ui/button'
import { Checkbox } from '@/components/ui/checkbox'
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
import { handleCopyChannel } from '../../lib'
import { useChannels } from '../channels-provider'

type CopyChannelDialogProps = {
  open: boolean
  onOpenChange: (open: boolean) => void
}

export function CopyChannelDialog({
  open,
  onOpenChange,
}: CopyChannelDialogProps) {
  const { t } = useTranslation()
  const { currentRow } = useChannels()
  const queryClient = useQueryClient()
  const [suffix, setSuffix] = useState('_copy')
  const [resetBalance, setResetBalance] = useState(true)
  const [isCopying, setIsCopying] = useState(false)

  if (!currentRow) return null

  const handleCopy = async () => {
    setIsCopying(true)

    await handleCopyChannel(
      currentRow.id,
      {
        suffix,
        reset_balance: resetBalance,
      },
      queryClient,
      () => {
        onOpenChange(false)
        setSuffix('_copy')
        setResetBalance(true)
      }
    )

    setIsCopying(false)
  }

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>{t('channels.actions.copyChannel')}</DialogTitle>
          <DialogDescription>
            {t('channels.actions.createACopyOf')}{' '}
            <strong>{currentRow.name}</strong>
          </DialogDescription>
        </DialogHeader>

        <div className='space-y-4 py-4'>
          <div className='space-y-2'>
            <Label htmlFor='suffix'>{t('channels.fields.nameSuffix')}</Label>
            <Input
              id='suffix'
              placeholder={t('channels.fields.copy')}
              value={suffix}
              onChange={(e) => setSuffix(e.target.value)}
              disabled={isCopying}
            />
            <p className='text-muted-foreground text-xs'>
              {t('channels.fields.newNameWillBe')} {currentRow.name}
              {suffix}
            </p>
          </div>

          <div className='flex items-center space-x-2'>
            <Checkbox
              id='reset-balance'
              checked={resetBalance}
              onCheckedChange={(checked) => setResetBalance(!!checked)}
              disabled={isCopying}
            />
            <Label htmlFor='reset-balance' className='text-sm font-normal'>
              {t('channels.actions.resetBalanceAndUsedQuota')}
            </Label>
          </div>
        </div>

        <DialogFooter>
          <Button
            variant='outline'
            onClick={() => onOpenChange(false)}
            disabled={isCopying}
          >
            {t('common.actions.cancel')}
          </Button>
          <Button onClick={handleCopy} disabled={isCopying}>
            {isCopying && <Loader2 className='mr-2 h-4 w-4 animate-spin' />}
            {isCopying
              ? t('channels.status.copying')
              : t('channels.actions.copyChannel')}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}
