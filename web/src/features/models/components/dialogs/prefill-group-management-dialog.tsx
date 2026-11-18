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
import { useQuery, useQueryClient } from '@tanstack/react-query'
import {
  Layers3,
  Loader2,
  Pencil,
  Plus,
  RefreshCcw,
  Trash2,
  X,
} from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { cn } from '@/lib/utils'
import { useIsMobile } from '@/hooks/use-mobile'
import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert'
import { Button } from '@/components/ui/button'
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from '@/components/ui/card'
import {
  Dialog,
  DialogClose,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import {
  Empty,
  EmptyDescription,
  EmptyHeader,
  EmptyMedia,
  EmptyTitle,
} from '@/components/ui/empty'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'
import { ConfirmDialog } from '@/components/confirm-dialog'
import { StatusBadge } from '@/components/status-badge'
import { TableId } from '@/components/table-id'
import { deletePrefillGroup, getPrefillGroups } from '../../api'
import { prefillGroupsQueryKeys } from '../../lib'
import type { PrefillGroup } from '../../types'
import {
  PREFILL_GROUP_TYPE_META,
  parseEndpointKeys,
  parseStringItems,
} from '../prefill-group-shared'

type PrefillGroupManagementDialogProps = {
  open: boolean
  onOpenChange: (open: boolean) => void
  onCreateGroup: () => void
  onEditGroup: (group: PrefillGroup) => void
}

export function PrefillGroupManagementDialog({
  open,
  onOpenChange,
  onCreateGroup,
  onEditGroup,
}: PrefillGroupManagementDialogProps) {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const isMobile = useIsMobile()
  const [deleteState, setDeleteState] = useState<{
    open: boolean
    group: PrefillGroup | null
  }>({ open: false, group: null })
  const [isDeleting, setIsDeleting] = useState(false)

  const {
    data,
    isLoading,
    isFetching,
    error,
    refetch: refetchGroups,
  } = useQuery({
    queryKey: prefillGroupsQueryKeys.list(),
    queryFn: () => getPrefillGroups(),
    enabled: open,
  })

  const groups = useMemo(() => data?.data ?? [], [data?.data])

  const sortedGroups = useMemo(
    () =>
      [...groups].sort((a, b) => {
        if (a.type === b.type) {
          return a.name.localeCompare(b.name)
        }
        return a.type.localeCompare(b.type)
      }),
    [groups]
  )

  const normalizedGroups = useMemo(
    () =>
      sortedGroups.map((group) => {
        const meta = PREFILL_GROUP_TYPE_META[group.type] || {
          label: group.type,
          badge: 'neutral' as const,
        }
        const parsedItems =
          group.type === 'endpoint'
            ? parseEndpointKeys(group.items)
            : parseStringItems(group.items)
        return { group, meta, parsedItems }
      }),
    [sortedGroups]
  )

  useEffect(() => {
    if (!open) {
      setDeleteState({ open: false, group: null })
      setIsDeleting(false)
    }
  }, [open])

  const handleDeleteClick = (group: PrefillGroup) => {
    setDeleteState({ open: true, group })
  }

  const handleDeleteConfirm = async () => {
    if (!deleteState.group) return
    setIsDeleting(true)
    try {
      const response = await deletePrefillGroup(deleteState.group.id)
      if (response.success) {
        toast.success(
          t('common.status.deletedName', { name: deleteState.group.name })
        )
        queryClient.invalidateQueries({
          queryKey: prefillGroupsQueryKeys.lists(),
        })
        setDeleteState({ open: false, group: null })
      } else {
        toast.error(response.message || t('models.errors.failedToDeleteGroup'))
      }
    } catch (err: unknown) {
      toast.error(
        (err as Error)?.message || t('models.errors.failedToDeleteGroup')
      )
    } finally {
      setIsDeleting(false)
    }
  }

  return (
    <>
      <Dialog open={open} onOpenChange={onOpenChange}>
        <DialogContent
          showCloseButton={false}
          className='prefill-dialog-content !top-4 !flex !-translate-y-0 !flex-col !gap-0 !border-none !bg-transparent !p-0 !shadow-none sm:!top-1/2 sm:!-translate-y-1/2'
          style={{ maxWidth: 'min(100vw, 64rem)' }}
        >
          <div
            className={cn(
              'prefill-dialog-panel border-border/70 bg-background flex max-h-[calc(100dvh-1.5rem)] flex-col overflow-hidden border shadow-2xl',
              isMobile ? 'rounded-none' : 'rounded-2xl'
            )}
          >
            <div
              className={cn(
                'relative flex flex-col gap-3 border-b px-4 py-4 sm:px-6 sm:py-5',
                isMobile && 'pt-[calc(env(safe-area-inset-top,0px)+1rem)]'
              )}
            >
              <DialogHeader className='max-w-3xl gap-3 pr-12 text-start sm:pr-0'>
                <DialogTitle className='flex flex-wrap items-center gap-2 text-xl'>
                  <Layers3 className='text-foreground/80 h-5 w-5' />
                  {t('models.titles.prefillGroupManagement')}
                </DialogTitle>
                <DialogDescription className='text-base leading-relaxed sm:text-sm'>
                  {t(
                    'models.actions.createReusableBundlesOfModelsTagsEndpointsAndUser'
                  )}
                </DialogDescription>
              </DialogHeader>

              <DialogClose
                render={
                  <Button
                    variant='ghost'
                    size='icon'
                    className='text-muted-foreground hover:text-foreground absolute top-4 right-4 border border-transparent sm:top-5 sm:right-6'
                  />
                }
              >
                <span className='sr-only'>
                  {t('models.actions.closeDialog')}
                </span>
                <X className='h-4 w-4' />
              </DialogClose>
            </div>

            <div className='flex flex-wrap items-center gap-3 border-b px-4 py-3 text-sm sm:px-6'>
              <div className='flex flex-wrap items-center gap-2'>
                <Button size='sm' onClick={onCreateGroup}>
                  <Plus className='mr-2 h-4 w-4' />
                  {t('models.fields.newGroup')}
                </Button>
                <Button
                  size='sm'
                  variant='ghost'
                  onClick={() => refetchGroups()}
                  disabled={isFetching}
                >
                  {isFetching ? (
                    <Loader2 className='mr-2 h-4 w-4 animate-spin' />
                  ) : (
                    <RefreshCcw className='mr-2 h-4 w-4' />
                  )}
                  {t('channels.actions.refresh')}
                </Button>
              </div>
              <StatusBadge
                label={`${groups.length} group${groups.length === 1 ? '' : 's'}`}
                variant='neutral'
                copyable={false}
              />
            </div>

            <div
              className={cn(
                'flex flex-1 flex-col overflow-hidden px-4 py-4 sm:px-6 sm:py-6',
                isMobile && 'pb-[calc(env(safe-area-inset-bottom,0px)+1.5rem)]'
              )}
            >
              <div className='flex-1 overflow-y-auto'>
                <div className='flex flex-col gap-4'>
                  {error && (
                    <Alert variant='destructive'>
                      <AlertTitle>
                        {t('models.errors.unableToLoadGroups')}
                      </AlertTitle>
                      <AlertDescription>
                        {(error as Error).message ||
                          t('models.tips.pleaseRetryOrRefreshPage')}
                      </AlertDescription>
                    </Alert>
                  )}

                  {isLoading ? (
                    <div className='flex flex-col items-center justify-center gap-2 py-16 text-center'>
                      <Loader2 className='text-muted-foreground h-6 w-6 animate-spin' />
                      <p className='text-muted-foreground text-sm'>
                        {t('models.tips.fetchingPrefillGroups')}
                      </p>
                    </div>
                  ) : normalizedGroups.length === 0 ? (
                    <Empty className='border border-dashed'>
                      <EmptyMedia variant='icon'>
                        <Layers3 className='h-6 w-6' />
                      </EmptyMedia>
                      <EmptyHeader>
                        <EmptyTitle>
                          {t('models.fields.noPrefillGroupsYet')}
                        </EmptyTitle>
                        <EmptyDescription>
                          {t(
                            'models.actions.createYourFirstGroupToReuseModelTagOr'
                          )}
                        </EmptyDescription>
                      </EmptyHeader>
                      <EmptyDescription>
                        {t(
                          'models.tips.prefillGroupsHelpYouKeepComplexConfigurationsInSync'
                        )}
                      </EmptyDescription>
                    </Empty>
                  ) : isMobile ? (
                    <div className='space-y-4'>
                      {normalizedGroups.map(({ group, meta, parsedItems }) => (
                        <Card key={group.id} className='border-border/60'>
                          <CardHeader className='flex flex-row items-start justify-between gap-4'>
                            <div className='space-y-2'>
                              <CardTitle className='flex flex-wrap items-center gap-2'>
                                {group.name}
                                <StatusBadge
                                  variant={meta.badge}
                                  size='sm'
                                  copyable={false}
                                >
                                  {t(meta.label)}
                                  <span className='text-muted-foreground/30'>
                                    ·
                                  </span>
                                  <span className='text-muted-foreground font-mono'>
                                    #{group.id}
                                  </span>
                                </StatusBadge>
                              </CardTitle>
                              {group.description ? (
                                <CardDescription className='line-clamp-2'>
                                  {group.description}
                                </CardDescription>
                              ) : (
                                <CardDescription className='text-muted-foreground italic'>
                                  {t('models.tips.noDescriptionProvided')}
                                </CardDescription>
                              )}
                            </div>

                            <div className='flex items-center gap-2'>
                              <Button
                                size='icon'
                                variant='outline'
                                onClick={() => onEditGroup(group)}
                              >
                                <Pencil className='h-4 w-4' />
                                <span className='sr-only'>
                                  {t('models.actions.editGroup')}
                                </span>
                              </Button>
                              <Button
                                size='icon'
                                variant='ghost'
                                className='text-destructive hover:text-destructive'
                                onClick={() => handleDeleteClick(group)}
                              >
                                <Trash2 className='h-4 w-4' />
                                <span className='sr-only'>
                                  {t('models.actions.deleteGroup')}
                                </span>
                              </Button>
                            </div>
                          </CardHeader>
                          <CardContent className='space-y-3'>
                            <div className='text-muted-foreground flex flex-wrap items-center gap-2 text-xs font-medium tracking-wide uppercase'>
                              <span>{t('models.fields.items')}</span>
                              <StatusBadge
                                label={t('models.fields.countItems', {
                                  count: parsedItems.length,
                                })}
                                variant='neutral'
                                size='sm'
                                copyable={false}
                              />
                            </div>
                            {parsedItems.length > 0 ? (
                              <div className='flex flex-wrap gap-2'>
                                {parsedItems.slice(0, 6).map((item) => (
                                  <StatusBadge
                                    key={item}
                                    label={item}
                                    autoColor={item}
                                    size='sm'
                                  />
                                ))}
                                {parsedItems.length > 6 && (
                                  <StatusBadge
                                    label={t('rankings.fields.countMore', {
                                      count: parsedItems.length - 6,
                                    })}
                                    variant='neutral'
                                    size='sm'
                                    copyable={false}
                                  />
                                )}
                              </div>
                            ) : (
                              <p className='text-muted-foreground text-sm'>
                                {group.type === 'endpoint'
                                  ? t(
                                      'models.tips.noEndpointMappingsConfigured'
                                    )
                                  : t('models.tips.noItemsConfiguredYet')}
                              </p>
                            )}
                          </CardContent>
                        </Card>
                      ))}
                    </div>
                  ) : (
                    <div className='rounded-md border'>
                      <div className='w-full overflow-x-auto'>
                        <Table className='min-w-[720px]'>
                          <TableHeader>
                            <TableRow>
                              <TableHead>{t('common.fields.group')}</TableHead>
                              <TableHead>{t('channels.fields.type')}</TableHead>
                              <TableHead className='min-w-[280px]'>
                                {t('models.fields.items')}
                              </TableHead>
                              <TableHead className='w-[120px] text-right'>
                                {t('channels.fields.actions')}
                              </TableHead>
                            </TableRow>
                          </TableHeader>
                          <TableBody>
                            {normalizedGroups.map(
                              ({ group, meta, parsedItems }) => (
                                <TableRow key={group.id}>
                                  <TableCell className='align-top whitespace-normal'>
                                    <div className='flex flex-col gap-1'>
                                      <div className='flex flex-wrap items-center gap-2'>
                                        <span className='font-medium'>
                                          {group.name}
                                        </span>
                                        <TableId value={group.id} />
                                      </div>
                                      {group.description ? (
                                        <p className='text-muted-foreground text-xs'>
                                          {group.description}
                                        </p>
                                      ) : (
                                        <p className='text-muted-foreground text-xs italic'>
                                          {t(
                                            'models.tips.noDescriptionProvided'
                                          )}
                                        </p>
                                      )}
                                    </div>
                                  </TableCell>
                                  <TableCell className='align-top'>
                                    <StatusBadge
                                      label={t(meta.label)}
                                      variant={meta.badge}
                                      size='sm'
                                      copyable={false}
                                    />
                                  </TableCell>
                                  <TableCell className='align-top whitespace-normal'>
                                    <div className='flex flex-wrap gap-2'>
                                      {parsedItems.length > 0 ? (
                                        <>
                                          {parsedItems
                                            .slice(0, 6)
                                            .map((item) => (
                                              <StatusBadge
                                                key={item}
                                                label={item}
                                                autoColor={item}
                                                size='sm'
                                              />
                                            ))}
                                          {parsedItems.length > 6 && (
                                            <StatusBadge
                                              label={t(
                                                'rankings.fields.countMore',
                                                {
                                                  count: parsedItems.length - 6,
                                                }
                                              )}
                                              variant='neutral'
                                              size='sm'
                                              copyable={false}
                                            />
                                          )}
                                        </>
                                      ) : (
                                        <p className='text-muted-foreground text-sm'>
                                          {group.type === 'endpoint'
                                            ? t(
                                                'models.tips.noEndpointMappingsConfigured'
                                              )
                                            : t(
                                                'models.tips.noItemsConfiguredYet'
                                              )}
                                        </p>
                                      )}
                                    </div>
                                    <div className='text-muted-foreground mt-2 text-xs font-medium tracking-wide uppercase'>
                                      {t('models.fields.countItems', {
                                        count: parsedItems.length,
                                      })}
                                    </div>
                                  </TableCell>
                                  <TableCell className='align-top'>
                                    <div className='flex justify-end gap-2'>
                                      <Button
                                        size='icon'
                                        variant='outline'
                                        onClick={() => onEditGroup(group)}
                                      >
                                        <Pencil className='h-4 w-4' />
                                        <span className='sr-only'>
                                          {t('models.actions.editGroup')}
                                        </span>
                                      </Button>
                                      <Button
                                        size='icon'
                                        variant='ghost'
                                        className='text-destructive hover:text-destructive'
                                        onClick={() => handleDeleteClick(group)}
                                      >
                                        <Trash2 className='h-4 w-4' />
                                        <span className='sr-only'>
                                          {t('models.actions.deleteGroup')}
                                        </span>
                                      </Button>
                                    </div>
                                  </TableCell>
                                </TableRow>
                              )
                            )}
                          </TableBody>
                        </Table>
                      </div>
                    </div>
                  )}
                </div>
              </div>
            </div>
          </div>
        </DialogContent>
      </Dialog>

      <ConfirmDialog
        open={deleteState.open}
        onOpenChange={(next) => setDeleteState({ open: next, group: null })}
        title={t('models.actions.deleteGroup')}
        desc={
          <p>
            {t('models.tips.confirmDeletePrefillGroup', {
              name: deleteState.group?.name ?? '',
            })}
          </p>
        }
        destructive
        confirmText={
          isDeleting ? t('keys.tips.deleting') : t('common.actions.delete')
        }
        isLoading={isDeleting}
        handleConfirm={handleDeleteConfirm}
      />
    </>
  )
}
