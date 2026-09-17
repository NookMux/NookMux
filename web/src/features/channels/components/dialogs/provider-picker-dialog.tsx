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
import { useState, useEffect, useMemo } from 'react'
import { ArrowDown, ArrowUp, Loader2, Plus, Search, X } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { Badge } from '@/components/ui/badge'
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
import { fetchUpstreamProviders } from '../../api'
import type { OpenRouterProviderInfo } from '../../types'
import { useChannels } from '../channels-provider'

export type ProviderPickerMode = 'order' | 'only' | 'ignore'

type ProviderPickerDialogProps = {
  open: boolean
  onOpenChange: (open: boolean) => void
  mode: ProviderPickerMode
  /** Current slug list from the form (comma-separated string). */
  value: string
  onConfirm: (value: string) => void
  /** Create-mode fetcher; when omitted the current channel is used. */
  customFetcher?: () => Promise<OpenRouterProviderInfo[]>
}

function parseSlugs(value: string): string[] {
  return Array.from(
    new Set(
      value
        .split(',')
        .map((slug) => slug.trim())
        .filter(Boolean)
    )
  )
}

export function ProviderPickerDialog({
  open,
  onOpenChange,
  mode,
  value,
  onConfirm,
  customFetcher,
}: ProviderPickerDialogProps) {
  const { t } = useTranslation()
  const { currentRow } = useChannels()
  const activeChannel = customFetcher ? null : currentRow
  const [isFetching, setIsFetching] = useState(false)
  const [fetchFailed, setFetchFailed] = useState(false)
  const [providers, setProviders] = useState<OpenRouterProviderInfo[]>([])
  const [selected, setSelected] = useState<string[]>([])
  const [searchKeyword, setSearchKeyword] = useState('')
  const [customSlug, setCustomSlug] = useState('')

  useEffect(() => {
    if (open && (activeChannel || customFetcher)) {
      handleFetchProviders()
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [open, activeChannel?.id, customFetcher])

  useEffect(() => {
    if (open) {
      setSelected(parseSlugs(value))
      setSearchKeyword('')
      setCustomSlug('')
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [open])

  const handleFetchProviders = async () => {
    if (!activeChannel && !customFetcher) return

    setIsFetching(true)
    setFetchFailed(false)
    try {
      if (customFetcher) {
        setProviders(await customFetcher())
      } else {
        const response = await fetchUpstreamProviders(activeChannel!.id)
        if (!response.success) {
          toast.error(
            response.message || t('channels.errors.fetchProvidersFailed')
          )
          setFetchFailed(true)
          return
        }
        setProviders(response.data || [])
      }
    } catch (error) {
      setFetchFailed(true)
      toast.error(
        error instanceof Error
          ? error.message
          : t('channels.errors.fetchProvidersFailed')
      )
    } finally {
      setIsFetching(false)
    }
  }

  const toggleProvider = (slug: string) => {
    setSelected((prev) =>
      prev.includes(slug)
        ? prev.filter((entry) => entry !== slug)
        : [...prev, slug]
    )
  }

  const moveSlug = (index: number, delta: number) => {
    setSelected((prev) => {
      const next = [...prev]
      const target = index + delta
      if (target < 0 || target >= next.length) return prev
      ;[next[index], next[target]] = [next[target], next[index]]
      return next
    })
  }

  const addCustomSlug = () => {
    const slug = customSlug.trim()
    if (!slug) return
    if (!selected.includes(slug)) {
      setSelected((prev) => [...prev, slug])
    }
    setCustomSlug('')
  }

  const keyword = searchKeyword.toLowerCase().trim()
  const filteredProviders = useMemo(
    () =>
      providers.filter(
        (provider) =>
          !keyword ||
          provider.slug.toLowerCase().includes(keyword) ||
          provider.name.toLowerCase().includes(keyword)
      ),
    [providers, keyword]
  )

  const knownSlugs = useMemo(
    () => new Set(providers.map((provider) => provider.slug)),
    [providers]
  )

  const handleClose = () => {
    onOpenChange(false)
  }

  const handleConfirm = () => {
    onConfirm(selected.join(', '))
    onOpenChange(false)
  }

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className='max-h-[90vh] sm:max-w-3xl'>
        <DialogHeader>
          <DialogTitle>{t('channels.titles.fetchProviders')}</DialogTitle>
          <DialogDescription>
            {mode === 'order'
              ? t('channels.tips.providerPickerOrderDescription')
              : t('channels.tips.providerPickerListDescription')}
          </DialogDescription>
        </DialogHeader>

        {!activeChannel && !customFetcher ? (
          <div className='text-muted-foreground py-8 text-center'>
            {t('channels.fields.noChannelSelected')}
          </div>
        ) : isFetching ? (
          <div className='flex items-center justify-center py-12'>
            <Loader2 className='text-muted-foreground h-8 w-8 animate-spin' />
          </div>
        ) : fetchFailed && providers.length === 0 ? (
          <div className='text-muted-foreground py-8 text-center'>
            <p>{t('channels.tips.fetchProvidersEmptyRetry')}</p>
            <Button
              className='mt-4'
              onClick={handleFetchProviders}
              disabled={isFetching}
            >
              {t('channels.titles.fetchProviders')}
            </Button>
          </div>
        ) : (
          <div className='space-y-4'>
            <div className='relative'>
              <Search className='text-muted-foreground absolute top-1/2 left-3 h-4 w-4 -translate-y-1/2' />
              <Input
                placeholder={t('channels.placeholders.searchProviders')}
                value={searchKeyword}
                onChange={(e) => setSearchKeyword(e.target.value)}
                className='pl-9'
              />
            </div>

            <div className='max-h-[40vh] space-y-1 overflow-y-auto'>
              {filteredProviders.length === 0 ? (
                <p className='text-muted-foreground py-8 text-center text-sm'>
                  {t('channels.tips.noProvidersMatchSearch')}
                </p>
              ) : (
                filteredProviders.map((provider) => (
                  <label
                    key={provider.slug}
                    className='hover:bg-muted/50 flex cursor-pointer items-center gap-3 rounded-md px-3 py-2'
                  >
                    <Checkbox
                      checked={selected.includes(provider.slug)}
                      onCheckedChange={() => toggleProvider(provider.slug)}
                    />
                    <span className='text-sm font-medium'>
                      {provider.name || provider.slug}
                    </span>
                    <span className='text-muted-foreground font-mono text-xs'>
                      {provider.slug}
                    </span>
                    {provider.headquarters && (
                      <Badge variant='outline' className='ml-auto shrink-0'>
                        {provider.headquarters}
                      </Badge>
                    )}
                  </label>
                ))
              )}
            </div>

            <div className='flex items-center gap-2'>
              <Input
                placeholder={t('channels.placeholders.customProviderSlug')}
                value={customSlug}
                onChange={(e) => setCustomSlug(e.target.value)}
                onKeyDown={(e) => {
                  if (e.key === 'Enter') {
                    e.preventDefault()
                    addCustomSlug()
                  }
                }}
              />
              <Button
                type='button'
                variant='outline'
                size='sm'
                onClick={addCustomSlug}
                disabled={!customSlug.trim()}
                aria-label={t('channels.actions.addCustomSlug')}
              >
                <Plus className='h-4 w-4' />
              </Button>
            </div>

            {selected.length > 0 && (
              <div className='space-y-2'>
                <p className='text-muted-foreground text-xs'>
                  {mode === 'order'
                    ? t('channels.tips.providerOrderHint')
                    : t('channels.tips.providerListHint')}
                </p>
                <div className='flex flex-wrap gap-2'>
                  {selected.map((slug, index) => (
                    <div
                      key={slug}
                      className='bg-muted/50 flex items-center gap-1 rounded-md border px-2 py-1'
                    >
                      {mode === 'order' && (
                        <span className='text-muted-foreground w-4 text-right text-xs'>
                          {index + 1}
                        </span>
                      )}
                      <span className='font-mono text-xs'>{slug}</span>
                      {!knownSlugs.has(slug) && (
                        <Badge variant='secondary' className='px-1 text-[10px]'>
                          {t('channels.status.customSlug')}
                        </Badge>
                      )}
                      {mode === 'order' && (
                        <>
                          <Button
                            type='button'
                            variant='ghost'
                            size='icon'
                            className='h-5 w-5'
                            onClick={() => moveSlug(index, -1)}
                            disabled={index === 0}
                            aria-label={t('channels.actions.moveUp')}
                          >
                            <ArrowUp className='h-3 w-3' />
                          </Button>
                          <Button
                            type='button'
                            variant='ghost'
                            size='icon'
                            className='h-5 w-5'
                            onClick={() => moveSlug(index, 1)}
                            disabled={index === selected.length - 1}
                            aria-label={t('channels.actions.moveDown')}
                          >
                            <ArrowDown className='h-3 w-3' />
                          </Button>
                        </>
                      )}
                      <Button
                        type='button'
                        variant='ghost'
                        size='icon'
                        className='h-5 w-5'
                        onClick={() => toggleProvider(slug)}
                        aria-label={t('channels.actions.removeProvider')}
                      >
                        <X className='h-3 w-3' />
                      </Button>
                    </div>
                  ))}
                </div>
              </div>
            )}
          </div>
        )}

        <DialogFooter>
          <Button variant='outline' onClick={handleClose}>
            {t('common.actions.cancel')}
          </Button>
          <Button onClick={handleConfirm}>{t('common.actions.confirm')}</Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}
