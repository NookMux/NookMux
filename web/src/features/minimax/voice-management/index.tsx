/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, version 3 of the License.
*/
import { useMemo, useState } from 'react'
import { useQuery, useQueryClient } from '@tanstack/react-query'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { useAuthStore } from '@/stores/auth-store'
import type { OnChangeFn, PaginationState } from '@/lib/tanstack-table'
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
import { Button } from '@/components/ui/button'
import { SectionPageLayout } from '@/components/layout'
import { USER_ROLE } from '@/features/users/constants'
import {
  createVoice,
  deleteVoice,
  extractApiErrorMessage,
  listVoices,
  updateVoice,
  type VoiceListParams,
  type VoiceRecord,
  type VoiceUpsertParams,
} from './api'
import { VoiceDialog } from './components/voice-dialog'
import {
  VoiceFilterBar,
  type VoiceFilterState,
} from './components/voice-filter-bar'
import { VoiceManagementTable } from './components/voice-management-table'

const DEFAULT_PAGE_SIZE = 20

const EMPTY_FILTERS: VoiceFilterState = {
  startTime: '',
  endTime: '',
  type: '',
  operatorId: '',
  voiceId: '',
}

const EMPTY_FORM: VoiceUpsertParams = {
  voice_id: '',
  type: 'created',
  redirect_id: '',
  allowed: false,
  remark: '',
}

function toUnixSeconds(value: string): number | undefined {
  if (!value) return undefined

  const ms = new Date(value).getTime()
  if (!Number.isFinite(ms)) return undefined

  return Math.floor(ms / 1000)
}

function normalizeOperatorId(value: string): number | undefined {
  const trimmed = value.trim()
  if (!trimmed) return undefined

  const id = Number(trimmed)
  if (!Number.isInteger(id) || id <= 0) return undefined

  return id
}

function toListParams(
  filters: VoiceFilterState,
  pagination: PaginationState
): VoiceListParams {
  return {
    page: pagination.pageIndex + 1,
    page_size: pagination.pageSize,
    type: filters.type || undefined,
    operator_id: normalizeOperatorId(filters.operatorId),
    voice_id: filters.voiceId.trim() || undefined,
    start_timestamp: toUnixSeconds(filters.startTime),
    end_timestamp: toUnixSeconds(filters.endTime),
  }
}

function hasFilters(filters: VoiceFilterState): boolean {
  return Boolean(
    filters.startTime ||
    filters.endTime ||
    filters.type ||
    filters.operatorId.trim() ||
    filters.voiceId.trim()
  )
}

export function VoiceManagement() {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const role = useAuthStore((state) => state.auth.user?.role)
  const isRoot = role === USER_ROLE.ROOT

  const [pagination, setPagination] = useState<PaginationState>({
    pageIndex: 0,
    pageSize: DEFAULT_PAGE_SIZE,
  })
  const [draftFilters, setDraftFilters] =
    useState<VoiceFilterState>(EMPTY_FILTERS)
  const [appliedFilters, setAppliedFilters] =
    useState<VoiceFilterState>(EMPTY_FILTERS)
  const [dialogOpen, setDialogOpen] = useState(false)
  const [editing, setEditing] = useState<VoiceRecord | null>(null)
  const [form, setForm] = useState<VoiceUpsertParams>(EMPTY_FORM)
  const [deleteTarget, setDeleteTarget] = useState<VoiceRecord | null>(null)
  const [isSubmitting, setIsSubmitting] = useState(false)
  const [isDeleting, setIsDeleting] = useState(false)

  const queryParams = useMemo(
    () => toListParams(appliedFilters, pagination),
    [appliedFilters, pagination]
  )

  const { data, isLoading, isFetching } = useQuery({
    queryKey: ['minimax-voices', queryParams],
    queryFn: () => listVoices(queryParams),
  })

  const items = data?.data?.items ?? []
  const total = data?.data?.total ?? 0

  const invalidate = () =>
    queryClient.invalidateQueries({ queryKey: ['minimax-voices'] })

  const handlePaginationChange: OnChangeFn<PaginationState> = (updater) => {
    setPagination((current) =>
      typeof updater === 'function' ? updater(current) : updater
    )
  }

  const handleSearch = () => {
    if (
      draftFilters.operatorId.trim() &&
      !normalizeOperatorId(draftFilters.operatorId)
    ) {
      toast.error(t('minimax.errors.operatorIdMustBeAPositiveInteger'))
      return
    }

    const startTimestamp = toUnixSeconds(draftFilters.startTime)
    const endTimestamp = toUnixSeconds(draftFilters.endTime)
    if (startTimestamp && endTimestamp && startTimestamp > endTimestamp) {
      toast.error(t('minimax.errors.startTimeCannotBeLaterThanEndTime'))
      return
    }

    setAppliedFilters(draftFilters)
    setPagination((current) => ({ ...current, pageIndex: 0 }))
  }

  const handleResetFilters = () => {
    setDraftFilters(EMPTY_FILTERS)
    setAppliedFilters(EMPTY_FILTERS)
    setPagination((current) => ({ ...current, pageIndex: 0 }))
  }

  const openCreate = () => {
    setEditing(null)
    setForm(EMPTY_FORM)
    setDialogOpen(true)
  }

  const openEdit = (record: VoiceRecord) => {
    setEditing(record)
    setForm({
      voice_id: record.voice_id,
      type: record.type,
      redirect_id: record.redirect_id,
      allowed: record.allowed,
      remark: record.remark,
    })
    setDialogOpen(true)
  }

  const handleSubmit = async () => {
    if (!form.voice_id.trim()) {
      toast.error(t('minimax.errors.voiceIdIsRequired'))
      return
    }

    setIsSubmitting(true)
    try {
      if (editing) {
        const response = await updateVoice(editing.id, form)
        if (!response.success) {
          toast.error(response.message || t('minimax.status.updateFailed'))
          return
        }
        toast.success(t('minimax.status.voiceUpdated'))
      } else {
        const response = await createVoice(form)
        if (!response.success) {
          toast.error(response.message || t('minimax.actions.createFailed'))
          return
        }
        toast.success(t('minimax.status.voiceCreated'))
      }

      setDialogOpen(false)
      invalidate()
    } catch (error) {
      const message = extractApiErrorMessage(error)
      toast.error(message || t('channels.status.operationFailed'))
    } finally {
      setIsSubmitting(false)
    }
  }

  const handleDelete = async () => {
    if (!deleteTarget) return

    setIsDeleting(true)
    try {
      const response = await deleteVoice(deleteTarget.id)
      if (!response.success) {
        toast.error(response.message || t('minimax.actions.deleteFailed'))
        return
      }

      toast.success(t('minimax.status.voiceDeleted'))
      setDeleteTarget(null)
      invalidate()
    } catch (error) {
      const message = extractApiErrorMessage(error)
      toast.error(message || t('minimax.actions.deleteFailed'))
    } finally {
      setIsDeleting(false)
    }
  }

  return (
    <>
      <SectionPageLayout>
        <SectionPageLayout.Title>
          {t('minimax.titles.voiceManagement')}
        </SectionPageLayout.Title>
        <SectionPageLayout.Actions>
          <Button onClick={openCreate}>{t('minimax.actions.addVoice')}</Button>
        </SectionPageLayout.Actions>

        <SectionPageLayout.Content>
          <VoiceManagementTable
            items={items}
            total={total}
            isLoading={isLoading}
            isFetching={isFetching}
            isRoot={isRoot}
            pagination={pagination}
            onPaginationChange={handlePaginationChange}
            onEdit={openEdit}
            onRequestDelete={setDeleteTarget}
            toolbar={
              <VoiceFilterBar
                filters={draftFilters}
                hasActiveFilters={
                  hasFilters(draftFilters) || hasFilters(appliedFilters)
                }
                isSearching={isFetching}
                onFiltersChange={setDraftFilters}
                onSearch={handleSearch}
                onReset={handleResetFilters}
              />
            }
          />
        </SectionPageLayout.Content>
      </SectionPageLayout>

      <VoiceDialog
        open={dialogOpen}
        editing={editing}
        form={form}
        isSubmitting={isSubmitting}
        onOpenChange={setDialogOpen}
        onFormChange={setForm}
        onSubmit={handleSubmit}
      />

      <AlertDialog
        open={deleteTarget !== null}
        onOpenChange={(open) => !open && setDeleteTarget(null)}
      >
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>
              {t('minimax.actions.deleteVoice')}
            </AlertDialogTitle>
            <AlertDialogDescription>
              {t('minimax.errors.sureYouWantToDeleteVoiceVoiceIdThis', {
                voiceId: deleteTarget?.voice_id ?? '-',
              })}
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel disabled={isDeleting}>
              {t('common.actions.cancel')}
            </AlertDialogCancel>
            <AlertDialogAction
              variant='destructive'
              onClick={handleDelete}
              disabled={isDeleting}
            >
              {isDeleting
                ? t('keys.tips.deleting')
                : t('common.actions.delete')}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </>
  )
}
