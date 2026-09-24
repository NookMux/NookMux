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
import { Loader2 } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'

export type VoiceTypeFilter = '' | 'created' | 'preview'

export type VoiceFilterState = {
  startTime: string
  endTime: string
  type: VoiceTypeFilter
  operatorId: string
  voiceId: string
}

type VoiceFilterBarProps = {
  filters: VoiceFilterState
  hasActiveFilters: boolean
  isSearching?: boolean
  onFiltersChange: (filters: VoiceFilterState) => void
  onSearch: () => void
  onReset: () => void
}

const TYPE_OPTIONS: Array<{ value: VoiceTypeFilter; labelKey: string }> = [
  { value: '', labelKey: 'pricing.fields.all' },
  { value: 'created', labelKey: 'voiceManagement.fields.voiceStatusPaid' },
  { value: 'preview', labelKey: 'voiceManagement.fields.voiceStatusPreview' },
]

function updateFilter(
  filters: VoiceFilterState,
  key: keyof VoiceFilterState,
  value: string
): VoiceFilterState {
  return { ...filters, [key]: value }
}

export function VoiceFilterBar(props: VoiceFilterBarProps) {
  const { t } = useTranslation()

  return (
    <div className='bg-card/50 rounded-lg border p-2.5 sm:p-3'>
      <div className='grid grid-cols-1 gap-2 sm:grid-cols-12 sm:items-end'>
        <div className='min-w-0 sm:col-span-2'>
          <Label className='text-muted-foreground mb-1.5 block text-xs'>
            {t('dashboard.actions.startTime')}
          </Label>
          <Input
            className='h-8 min-w-0 text-sm leading-5'
            type='datetime-local'
            value={props.filters.startTime}
            onChange={(event) =>
              props.onFiltersChange(
                updateFilter(props.filters, 'startTime', event.target.value)
              )
            }
          />
        </div>

        <div className='min-w-0 sm:col-span-2'>
          <Label className='text-muted-foreground mb-1.5 block text-xs'>
            {t('dashboard.fields.endTime')}
          </Label>
          <Input
            className='h-8 min-w-0 text-sm leading-5'
            type='datetime-local'
            value={props.filters.endTime}
            onChange={(event) =>
              props.onFiltersChange(
                updateFilter(props.filters, 'endTime', event.target.value)
              )
            }
          />
        </div>

        <div className='min-w-0 sm:col-span-2'>
          <Label className='text-muted-foreground mb-1.5 block text-xs'>
            {t('channels.fields.type')}
          </Label>
          <Select
            value={props.filters.type || 'all'}
            onValueChange={(value) =>
              props.onFiltersChange({
                ...props.filters,
                type: value === 'all' ? '' : (value as VoiceTypeFilter),
              })
            }
          >
            <SelectTrigger className='h-8 min-w-0 text-sm leading-5'>
              <SelectValue />
            </SelectTrigger>
            <SelectContent alignItemWithTrigger={false}>
              {TYPE_OPTIONS.map((option) => (
                <SelectItem
                  key={option.value || 'all'}
                  value={option.value || 'all'}
                >
                  {t(option.labelKey)}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
        </div>

        <div className='min-w-0 sm:col-span-2'>
          <Label className='text-muted-foreground mb-1.5 block text-xs'>
            {t('voiceManagement.fields.operatorId')}
          </Label>
          <Input
            className='h-8 min-w-0 text-sm leading-5'
            inputMode='numeric'
            min={1}
            placeholder={t('voiceManagement.fields.operatorId')}
            type='number'
            value={props.filters.operatorId}
            onChange={(event) =>
              props.onFiltersChange(
                updateFilter(props.filters, 'operatorId', event.target.value)
              )
            }
          />
        </div>

        <div className='min-w-0 sm:col-span-2'>
          <Label className='text-muted-foreground mb-1.5 block text-xs'>
            {t('voiceManagement.fields.voiceId')}
          </Label>
          <Input
            className='h-8 min-w-0 font-mono text-sm leading-5'
            placeholder={t(
              'voiceManagement.actions.filterByVoiceIdNotRedirectId'
            )}
            value={props.filters.voiceId}
            onChange={(event) =>
              props.onFiltersChange(
                updateFilter(props.filters, 'voiceId', event.target.value)
              )
            }
          />
        </div>

        <div className='flex items-end justify-end gap-1.5 sm:col-span-2 sm:gap-2'>
          <Button
            type='button'
            variant='outline'
            onClick={props.onReset}
            disabled={!props.hasActiveFilters || props.isSearching}
          >
            {t('common.actions.reset')}
          </Button>
          <Button
            type='button'
            onClick={props.onSearch}
            disabled={props.isSearching}
          >
            {props.isSearching && <Loader2 className='animate-spin' />}
            {t('common.actions.search')}
          </Button>
        </div>
      </div>
    </div>
  )
}
