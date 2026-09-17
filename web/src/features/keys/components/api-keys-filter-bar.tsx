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
import { useState, useEffect, useCallback, useMemo } from 'react'
import { useQueryClient, useIsFetching, useQuery } from '@tanstack/react-query'
import { getRouteApi, useNavigate } from '@tanstack/react-router'
import { Loader2 } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { getUserGroups } from '@/lib/api'
import { type RowData, type Table } from '@/lib/tanstack-table'
import { Button } from '@/components/ui/button'
import {
  Select,
  SelectContent,
  SelectGroup,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { DataTableViewOptions } from '@/components/data-table'
import {
  LogsFilterField,
  LogsFilterInput,
} from '@/features/usage-logs/components/logs-filter-toolbar'
import { API_KEY_STATUS_OPTIONS } from '../constants'

const route = getRouteApi('/_authenticated/keys/')

interface ApiKeysFilterBarProps<TData extends RowData> {
  table: Table<TData>
}

export function ApiKeysFilterBar<TData extends RowData>(
  props: ApiKeysFilterBarProps<TData>
) {
  const { t } = useTranslation()
  const navigate = useNavigate()
  const queryClient = useQueryClient()
  const searchParams = route.useSearch()
  const fetchingKeys = useIsFetching({ queryKey: ['keys'] })

  const [nameInput, setNameInput] = useState(searchParams.name ?? '')
  const [keyInput, setKeyInput] = useState(searchParams.key ?? '')
  const [groupInput, setGroupInput] = useState(searchParams.group ?? '')
  const [statusInput, setStatusInput] = useState<`${number}` | ''>(
    searchParams.status && searchParams.status.length > 0
      ? searchParams.status[0]
      : ''
  )

  // Sync local state when URL search params change (e.g. browser back/forward)
  useEffect(() => {
    setNameInput(searchParams.name ?? '')
    setKeyInput(searchParams.key ?? '')
    setGroupInput(searchParams.group ?? '')
    setStatusInput(
      searchParams.status && searchParams.status.length > 0
        ? searchParams.status[0]
        : ''
    )
  }, [
    searchParams.name,
    searchParams.key,
    searchParams.group,
    searchParams.status,
  ])

  // Fetch user groups for the group dropdown
  const { data: groupsData } = useQuery({
    queryKey: ['user-self-groups'],
    queryFn: getUserGroups,
    staleTime: 5 * 60 * 1000,
  })

  const groupOptions = useMemo(() => {
    const groups = groupsData?.success ? groupsData.data : undefined
    if (!groups) return []
    return Object.keys(groups).map((g) => ({ label: g, value: g }))
  }, [groupsData])

  const statusOptions = useMemo(
    () =>
      API_KEY_STATUS_OPTIONS.map((o) => ({
        label: t(o.label),
        value: o.value,
      })),
    [t]
  )
  const statusValueSet = useMemo(
    () => new Set(statusOptions.map((o) => o.value)),
    [statusOptions]
  )

  const groupLabel =
    groupOptions.find((o) => o.value === groupInput)?.label ??
    t('channels.fields.allGroups')
  const statusLabel =
    statusOptions.find((o) => o.value === statusInput)?.label ??
    t('channels.fields.allStatus')

  const handleApply = useCallback(() => {
    navigate({
      to: '/keys',
      search: (prev) => ({
        ...prev,
        name: nameInput.trim() || undefined,
        key: keyInput.trim() || undefined,
        group: groupInput || undefined,
        status: statusInput ? [statusInput] : [],
        page: 1,
      }),
    })
    queryClient.invalidateQueries({ queryKey: ['keys'] })
  }, [nameInput, keyInput, groupInput, statusInput, navigate, queryClient])

  const handleReset = useCallback(() => {
    setNameInput('')
    setKeyInput('')
    setGroupInput('')
    setStatusInput('')
    navigate({
      to: '/keys',
      search: (prev) => ({
        ...prev,
        name: undefined,
        key: undefined,
        group: undefined,
        status: [],
        page: 1,
      }),
    })
    queryClient.invalidateQueries({ queryKey: ['keys'] })
  }, [navigate, queryClient])

  const handleKeyDown = useCallback(
    (e: React.KeyboardEvent) => {
      if (e.key === 'Enter') handleApply()
    },
    [handleApply]
  )

  const hasActiveFilters =
    !!nameInput || !!keyInput || !!groupInput || !!statusInput

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
  const keyField = (
    <LogsFilterField>
      <LogsFilterInput
        placeholder={t('channels.fields.apiKey')}
        autoComplete='off'
        value={keyInput}
        onChange={(e) => setKeyInput(e.target.value)}
        onKeyDown={handleKeyDown}
      />
    </LogsFilterField>
  )
  const groupField = (
    <LogsFilterField>
      <Select
        items={groupOptions}
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
            {groupOptions.map((o) => (
              <SelectItem key={o.value} value={o.value}>
                {o.label}
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
          setStatusInput(
            value && statusValueSet.has(value) ? (value as `${number}`) : ''
          )
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

  return (
    <div className='bg-card/50 rounded-lg border p-2.5 sm:p-3'>
      <div className='grid grid-cols-1 gap-2 sm:grid-cols-4'>
        {nameField}
        {keyField}
        {groupField}
        {statusField}
      </div>

      <div className='mt-2 flex flex-wrap items-center gap-2'>
        <div className='ms-auto flex flex-wrap items-center justify-end gap-1.5 sm:gap-2'>
          <Button
            type='button'
            variant='outline'
            onClick={handleReset}
            disabled={!hasActiveFilters}
          >
            {t('common.actions.reset')}
          </Button>
          <Button
            type='button'
            onClick={handleApply}
            disabled={fetchingKeys > 0}
          >
            {fetchingKeys > 0 && <Loader2 className='animate-spin' />}
            {t('common.actions.search')}
          </Button>
          <DataTableViewOptions table={props.table} />
        </div>
      </div>
    </div>
  )
}
