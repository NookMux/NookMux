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
import { useState, type ComponentProps, type ReactNode } from 'react'
import { useMediaQuery } from '@/hooks'
import { ChevronDown, Loader2 } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { type RowData, type Table } from '@/lib/tanstack-table'
import { cn } from '@/lib/utils'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import {
  Drawer,
  DrawerContent,
  DrawerDescription,
  DrawerFooter,
  DrawerHeader,
  DrawerTitle,
  DrawerTrigger,
} from '@/components/ui/drawer'
import { Input } from '@/components/ui/input'
import { DataTableViewOptions } from '@/components/data-table'

interface LogsFilterToolbarProps<TData extends RowData> {
  table: Table<TData>
  primaryFilters: ReactNode
  advancedFilters?: ReactNode
  clientFilters?: ReactNode
  mobilePinnedFilters?: ReactNode
  mobileFilters?: ReactNode
  mobileFilterCount?: number
  stats?: ReactNode
  hasActiveFilters: boolean
  hasAdvancedActiveFilters?: boolean
  hasClientActiveFilters?: boolean
  advancedFilterCount?: number
  clientFilterCount?: number
  searchLoading?: boolean
  onReset: () => void
  onSearch: () => void
  className?: string
}

interface LogsFilterFieldProps {
  children: ReactNode
  wide?: boolean
  className?: string
}

export function LogsFilterField(props: LogsFilterFieldProps) {
  return (
    <div
      className={cn(
        'min-w-0 [&_[data-slot=select-trigger]]:w-full [&_[data-slot=select-trigger]]:text-sm [&_[data-slot=select-value]]:leading-5',
        props.wide && 'sm:col-span-2',
        props.className
      )}
    >
      {props.children}
    </div>
  )
}

export function LogsFilterInput(props: ComponentProps<typeof Input>) {
  return (
    <Input
      {...props}
      className={cn('h-8 min-w-0 text-sm leading-5', props.className)}
    />
  )
}

export function LogsFilterToolbar<TData extends RowData>(
  props: LogsFilterToolbarProps<TData>
) {
  const { t } = useTranslation()
  const [advancedOpen, setAdvancedOpen] = useState(false)
  const [clientOpen, setClientOpen] = useState(false)
  const [mobileFiltersOpen, setMobileFiltersOpen] = useState(false)
  const isMobile = useMediaQuery('(max-width: 640px)')

  const hasAdvancedFilters = props.advancedFilters != null
  const hasClientFilters = props.clientFilters != null
  const activeAdvancedCount =
    props.advancedFilterCount ?? (props.hasAdvancedActiveFilters ? 1 : 0)
  const activeClientCount =
    props.clientFilterCount ?? (props.hasClientActiveFilters ? 1 : 0)
  const activeMobileFilterCount =
    props.mobileFilterCount ?? activeAdvancedCount + activeClientCount

  const handleMobileReset = () => {
    props.onReset()
    setMobileFiltersOpen(false)
  }

  const handleMobileSearch = () => {
    props.onSearch()
    setMobileFiltersOpen(false)
  }

  if (isMobile && props.mobilePinnedFilters != null) {
    return (
      <Drawer open={mobileFiltersOpen} onOpenChange={setMobileFiltersOpen}>
        <div
          className={cn('bg-card/50 rounded-lg border p-2.5', props.className)}
        >
          <div className='grid gap-2'>{props.mobilePinnedFilters}</div>

          <div className='mt-2 flex flex-col gap-2'>
            {props.stats}
            <div className='flex items-center justify-end gap-1.5'>
              <DrawerTrigger asChild>
                <Button
                  type='button'
                  variant='ghost'
                  className={cn(
                    'text-muted-foreground hover:text-foreground gap-1 px-2',
                    activeMobileFilterCount > 0 &&
                      'text-primary hover:text-primary'
                  )}
                >
                  {t('dashboard.actions.filter')}
                  {activeMobileFilterCount > 0 && (
                    <Badge className='ml-0.5 size-5 justify-center p-0 text-[10px]'>
                      {activeMobileFilterCount}
                    </Badge>
                  )}
                </Button>
              </DrawerTrigger>
              <Button
                type='button'
                onClick={props.onSearch}
                disabled={props.searchLoading}
              >
                {props.searchLoading && <Loader2 className='animate-spin' />}
                {t('common.actions.search')}
              </Button>
              <DataTableViewOptions table={props.table} />
            </div>
          </div>
        </div>

        <DrawerContent className='max-h-[85dvh] p-0'>
          <div className='mx-auto flex w-full max-w-md flex-1 flex-col overflow-hidden'>
            <DrawerHeader className='border-border/70 border-b px-4 py-3 text-left'>
              <DrawerTitle>{t('dashboard.actions.filter')}</DrawerTitle>
              <DrawerDescription>
                {t('usageLogs.tips.adjustFiltersThenSearchToRefreshTheLogs')}
              </DrawerDescription>
            </DrawerHeader>
            <div className='flex min-h-0 flex-1 flex-col gap-2 overflow-y-auto px-4 py-3'>
              {props.mobileFilters ?? (
                <>
                  {props.primaryFilters}
                  {props.advancedFilters}
                  {props.clientFilters}
                </>
              )}
            </div>
            <DrawerFooter className='border-border/70 grid grid-cols-2 gap-2 border-t px-4 py-3'>
              <Button
                type='button'
                variant='outline'
                onClick={handleMobileReset}
                disabled={!props.hasActiveFilters}
              >
                {t('common.actions.reset')}
              </Button>
              <Button
                type='button'
                onClick={handleMobileSearch}
                disabled={props.searchLoading}
              >
                {props.searchLoading && <Loader2 className='animate-spin' />}
                {t('common.actions.search')}
              </Button>
            </DrawerFooter>
          </div>
        </DrawerContent>
      </Drawer>
    )
  }

  return (
    <div
      className={cn(
        'bg-card/50 rounded-lg border p-2.5 sm:p-3',
        props.className
      )}
    >
      <div className='grid grid-cols-1 gap-2 sm:grid-cols-5'>
        {props.primaryFilters}
      </div>
      {advancedOpen && (
        <div className='mt-2 grid grid-cols-1 gap-2 sm:grid-cols-5'>
          {props.advancedFilters}
        </div>
      )}
      {clientOpen && (
        <div className='mt-2 grid grid-cols-1 gap-2 sm:grid-cols-4'>
          {props.clientFilters}
        </div>
      )}

      <div className='mt-2 flex flex-wrap items-center gap-2'>
        {props.stats}
        <div className='ms-auto flex flex-wrap items-center justify-end gap-1.5 sm:gap-2'>
          {hasAdvancedFilters && (
            <Button
              type='button'
              variant='ghost'
              onClick={() => setAdvancedOpen((open) => !open)}
              aria-expanded={advancedOpen}
              className={cn(
                'text-muted-foreground hover:text-foreground gap-1 px-2',
                props.hasAdvancedActiveFilters &&
                  !advancedOpen &&
                  'text-primary hover:text-primary'
              )}
            >
              {advancedOpen
                ? t('common.fields.collapse')
                : t('common.fields.expand')}
              {activeAdvancedCount > 0 && (
                <Badge className='ml-0.5 size-5 justify-center p-0 text-[10px]'>
                  {activeAdvancedCount}
                </Badge>
              )}
              <ChevronDown
                className={cn(
                  'size-3.5 transition-transform duration-200',
                  advancedOpen && 'rotate-180'
                )}
              />
            </Button>
          )}
          {hasClientFilters && (
            <Button
              type='button'
              variant='ghost'
              onClick={() => setClientOpen((open) => !open)}
              aria-expanded={clientOpen}
              className={cn(
                'text-muted-foreground hover:text-foreground gap-1 px-2',
                props.hasClientActiveFilters &&
                  !clientOpen &&
                  'text-primary hover:text-primary'
              )}
            >
              {clientOpen
                ? t('common.fields.collapse')
                : t('usageLogs.fields.clientInfo')}
              {activeClientCount > 0 && (
                <Badge className='ml-0.5 size-5 justify-center p-0 text-[10px]'>
                  {activeClientCount}
                </Badge>
              )}
              <ChevronDown
                className={cn(
                  'size-3.5 transition-transform duration-200',
                  clientOpen && 'rotate-180'
                )}
              />
            </Button>
          )}

          <Button
            type='button'
            variant='outline'
            onClick={props.onReset}
            disabled={!props.hasActiveFilters}
          >
            {t('common.actions.reset')}
          </Button>
          <Button
            type='button'
            onClick={props.onSearch}
            disabled={props.searchLoading}
          >
            {props.searchLoading && <Loader2 className='animate-spin' />}
            {t('common.actions.search')}
          </Button>
          <DataTableViewOptions table={props.table} />
        </div>
      </div>
    </div>
  )
}
