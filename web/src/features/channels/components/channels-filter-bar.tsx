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
import { useState, useEffect, useCallback } from 'react'
import { useQueryClient, useIsFetching } from '@tanstack/react-query'
import { getRouteApi, useNavigate } from '@tanstack/react-router'
import { useTranslation } from 'react-i18next'
import { type RowData, type Table } from '@/lib/tanstack-table'
import {
  Select,
  SelectContent,
  SelectGroup,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import {
  LogsFilterField,
  LogsFilterInput,
  LogsFilterToolbar,
} from '@/features/usage-logs/components/logs-filter-toolbar'
import { CHANNEL_STATUS_OPTIONS } from '../constants'
import { channelsQueryKeys } from '../lib'

const route = getRouteApi('/_authenticated/channels/')

export interface ChannelFilterOption {
  label: string
  value: string
  iconNode?: React.ReactNode
}

interface ChannelsFilterBarProps<TData extends RowData> {
  table: Table<TData>
  typeOptions: ChannelFilterOption[]
  groupOptions: ChannelFilterOption[]
}

export function ChannelsFilterBar<TData extends RowData>(
  props: ChannelsFilterBarProps<TData>
) {
  const { t } = useTranslation()
  const navigate = useNavigate()
  const queryClient = useQueryClient()
  const searchParams = route.useSearch()
  const fetchingChannels = useIsFetching({ queryKey: channelsQueryKeys.all })

  const [idInput, setIdInput] = useState(searchParams.id ?? '')
  const [nameInput, setNameInput] = useState(searchParams.name ?? '')
  const [modelInput, setModelInput] = useState(searchParams.model ?? '')
  const [tagInput, setTagInput] = useState(searchParams.tag ?? '')
  const [typeInput, setTypeInput] = useState(searchParams.type?.[0] ?? '')
  const [statusInput, setStatusInput] = useState(searchParams.status?.[0] ?? '')
  const [groupInput, setGroupInput] = useState(searchParams.group?.[0] ?? '')

  // Sync local state when URL search params change (e.g. browser back/forward)
  useEffect(() => {
    setIdInput(searchParams.id ?? '')
    setNameInput(searchParams.name ?? '')
    setModelInput(searchParams.model ?? '')
    setTagInput(searchParams.tag ?? '')
    setTypeInput(searchParams.type?.[0] ?? '')
    setStatusInput(searchParams.status?.[0] ?? '')
    setGroupInput(searchParams.group?.[0] ?? '')
  }, [
    searchParams.id,
    searchParams.name,
    searchParams.model,
    searchParams.tag,
    searchParams.type,
    searchParams.status,
    searchParams.group,
  ])

  const statusOptions = CHANNEL_STATUS_OPTIONS.map((o) => ({
    label: t(o.label),
    value: o.value,
  }))

  const typeLabel =
    props.typeOptions.find((o) => o.value === typeInput)?.label ??
    t('pricing.fields.allTypes')
  const statusLabel =
    statusOptions.find((o) => o.value === statusInput)?.label ??
    t('channels.fields.allStatus')
  const groupLabel =
    props.groupOptions.find((o) => o.value === groupInput)?.label ??
    t('channels.fields.allGroups')

  const handleApply = useCallback(() => {
    navigate({
      to: '/channels',
      search: (prev) => ({
        ...prev,
        id: idInput.trim() || undefined,
        name: nameInput.trim() || undefined,
        model: modelInput.trim() || undefined,
        tag: tagInput.trim() || undefined,
        type: typeInput ? [typeInput] : [],
        status: statusInput ? [statusInput] : [],
        group: groupInput ? [groupInput] : [],
        page: 1,
      }),
    })
    queryClient.invalidateQueries({ queryKey: channelsQueryKeys.lists() })
  }, [
    idInput,
    nameInput,
    modelInput,
    tagInput,
    typeInput,
    statusInput,
    groupInput,
    navigate,
    queryClient,
  ])

  const handleReset = useCallback(() => {
    setIdInput('')
    setNameInput('')
    setModelInput('')
    setTagInput('')
    setTypeInput('')
    setStatusInput('')
    setGroupInput('')
    navigate({
      to: '/channels',
      search: (prev) => ({
        ...prev,
        id: undefined,
        name: undefined,
        model: undefined,
        tag: undefined,
        type: [],
        status: [],
        group: [],
        page: 1,
      }),
    })
    queryClient.invalidateQueries({ queryKey: channelsQueryKeys.lists() })
  }, [navigate, queryClient])

  const handleKeyDown = useCallback(
    (e: React.KeyboardEvent) => {
      if (e.key === 'Enter') handleApply()
    },
    [handleApply]
  )

  const hasActiveFilters =
    !!idInput ||
    !!nameInput ||
    !!modelInput ||
    !!tagInput ||
    !!typeInput ||
    !!statusInput ||
    !!groupInput

  const hasExpandedFilters = !!modelInput || !!tagInput
  const expandedFilterCount = [modelInput, tagInput].filter(Boolean).length
  const mobileFilterCount = [
    typeInput,
    statusInput,
    groupInput,
    modelInput,
    tagInput,
  ].filter(Boolean).length

  const idField = (
    <LogsFilterField>
      <LogsFilterInput
        placeholder={t('channels.fields.id')}
        type='number'
        autoComplete='off'
        value={idInput}
        onChange={(e) => setIdInput(e.target.value)}
        onKeyDown={handleKeyDown}
      />
    </LogsFilterField>
  )
  const nameField = (
    <LogsFilterField>
      <LogsFilterInput
        placeholder={t('channels.fields.name')}
        autoComplete='off'
        value={nameInput}
        onChange={(e) => setNameInput(e.target.value)}
        onKeyDown={handleKeyDown}
      />
    </LogsFilterField>
  )
  const typeField = (
    <LogsFilterField>
      <Select
        items={props.typeOptions}
        value={typeInput || ''}
        onValueChange={(value) => {
          setTypeInput(value ?? '')
        }}
      >
        <SelectTrigger>
          <SelectValue>{typeLabel}</SelectValue>
        </SelectTrigger>
        <SelectContent alignItemWithTrigger={false}>
          <SelectGroup>
            <SelectItem value=''>{t('pricing.fields.allTypes')}</SelectItem>
            {props.typeOptions.map((o) => (
              <SelectItem key={o.value} value={o.value}>
                {o.iconNode ? (
                  <span className='inline-flex items-center gap-1.5'>
                    {o.iconNode}
                    <span>{t(o.label)}</span>
                  </span>
                ) : (
                  t(o.label)
                )}
              </SelectItem>
            ))}
          </SelectGroup>
        </SelectContent>
      </Select>
    </LogsFilterField>
  )
  const statusField = (
    <LogsFilterField>
      <Select
        items={statusOptions}
        value={statusInput || ''}
        onValueChange={(value) => {
          setStatusInput(value ?? '')
        }}
      >
        <SelectTrigger>
          <SelectValue>{statusLabel}</SelectValue>
        </SelectTrigger>
        <SelectContent alignItemWithTrigger={false}>
          <SelectGroup>
            <SelectItem value=''>{t('channels.fields.allStatus')}</SelectItem>
            {statusOptions.map((o) => (
              <SelectItem key={o.value} value={o.value}>
                {o.label}
              </SelectItem>
            ))}
          </SelectGroup>
        </SelectContent>
      </Select>
    </LogsFilterField>
  )
  const groupField = (
    <LogsFilterField>
      <Select
        items={props.groupOptions}
        value={groupInput || ''}
        onValueChange={(value) => {
          setGroupInput(value ?? '')
        }}
      >
        <SelectTrigger>
          <SelectValue>{groupLabel}</SelectValue>
        </SelectTrigger>
        <SelectContent alignItemWithTrigger={false}>
          <SelectGroup>
            <SelectItem value=''>{t('channels.fields.allGroups')}</SelectItem>
            {props.groupOptions.map((o) => (
              <SelectItem key={o.value} value={o.value}>
                {o.label}
              </SelectItem>
            ))}
          </SelectGroup>
        </SelectContent>
      </Select>
    </LogsFilterField>
  )
  const modelField = (
    <LogsFilterField>
      <LogsFilterInput
        placeholder={t('channels.actions.filterByModel')}
        autoComplete='off'
        value={modelInput}
        onChange={(e) => setModelInput(e.target.value)}
        onKeyDown={handleKeyDown}
      />
    </LogsFilterField>
  )
  const tagField = (
    <LogsFilterField>
      <LogsFilterInput
        placeholder={t('channels.fields.tag')}
        autoComplete='off'
        value={tagInput}
        onChange={(e) => setTagInput(e.target.value)}
        onKeyDown={handleKeyDown}
      />
    </LogsFilterField>
  )

  return (
    <LogsFilterToolbar
      table={props.table}
      primaryFilters={
        <>
          {idField}
          {nameField}
          {typeField}
          {statusField}
          {groupField}
        </>
      }
      advancedFilters={
        <>
          {modelField}
          {tagField}
        </>
      }
      mobilePinnedFilters={
        <>
          {idField}
          {nameField}
        </>
      }
      mobileFilters={
        <>
          {typeField}
          {statusField}
          {groupField}
          {modelField}
          {tagField}
        </>
      }
      mobileFilterCount={mobileFilterCount}
      hasAdvancedActiveFilters={hasExpandedFilters}
      advancedFilterCount={expandedFilterCount}
      hasActiveFilters={hasActiveFilters}
      onSearch={handleApply}
      searchLoading={fetchingChannels > 0}
      onReset={handleReset}
    />
  )
}
