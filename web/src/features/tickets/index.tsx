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
import { useCallback, useMemo, useState } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import {
  CheckCircle2,
  Clock3,
  Eye,
  Loader2,
  MoreHorizontal,
  Plus,
  RefreshCw,
  RotateCcw,
  Search,
  SendHorizontal,
} from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { useAuthStore } from '@/stores/auth-store'
import { formatTimestampToDate } from '@/lib/format'
import { ROLE } from '@/lib/roles'
import {
  appTableFeatures,
  type ColumnDef,
  flexRender,
  useTable,
} from '@/lib/tanstack-table'
import { cn } from '@/lib/utils'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import {
  Dialog,
  DialogClose,
  DialogContent,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import {
  Select,
  SelectContent,
  SelectGroup,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { TableCell, TableRow } from '@/components/ui/table'
import { Tabs, TabsList, TabsTrigger } from '@/components/ui/tabs'
import { Textarea } from '@/components/ui/textarea'
import { ConfirmDialog } from '@/components/confirm-dialog'
import { DataTablePage } from '@/components/data-table'
import { SectionPageLayout } from '@/components/layout'
import {
  closeTicket,
  createTicket,
  getTicketDetail,
  getTickets,
  reopenTicket,
  replyTicket,
} from './api'
import type {
  CreateTicketPayload,
  TicketBase,
  TicketDetail,
  TicketMessage,
  TicketStatus,
  TicketStatusFilter,
  TicketSummary,
  TicketType,
} from './types'

const DEFAULT_PAGE_SIZE = 20

const STATUS_OPTIONS: Array<{
  value: TicketStatusFilter
  label: string
}> = [
  { value: 'all', label: 'pricing.fields.all' },
  { value: 'pending', label: 'common.status.pending' },
  { value: 'processing', label: 'common.status.processing' },
  { value: 'completed', label: 'common.fields.completed' },
]

const TYPE_OPTIONS: Array<{
  value: TicketType
  label: string
}> = [
  { value: 'bug', label: 'common.fields.bugReport' },
  { value: 'feature', label: 'common.fields.featureRequest' },
  { value: 'question', label: 'systemSettings.fields.question' },
  { value: 'other', label: 'common.fields.other' },
]

const EMPTY_CREATE_FORM: CreateTicketPayload = {
  title: '',
  type: 'question',
  content: '',
}

type StatusActionTarget = {
  ticket: TicketBase
  action: 'close' | 'reopen'
}

function getTypeLabel(type: TicketType) {
  return TYPE_OPTIONS.find((item) => item.value === type)?.label ?? type
}

function getStatusMeta(status: TicketStatus) {
  if (status === 'pending') {
    return {
      label: 'common.status.pending',
      icon: Clock3,
      className:
        'border-amber-500/30 bg-amber-500/10 text-amber-700 dark:text-amber-300',
    }
  }
  if (status === 'processing') {
    return {
      label: 'common.status.processing',
      icon: RefreshCw,
      className:
        'border-sky-500/30 bg-sky-500/10 text-sky-700 dark:text-sky-300',
    }
  }
  return {
    label: 'common.fields.completed',
    icon: CheckCircle2,
    className:
      'border-emerald-500/30 bg-emerald-500/10 text-emerald-700 dark:text-emerald-300',
  }
}

function formatTime(timestamp: number) {
  return timestamp > 0 ? formatTimestampToDate(timestamp) : '-'
}

function StatusBadge({ status }: { status: TicketStatus }) {
  const { t } = useTranslation()
  const meta = getStatusMeta(status)
  const Icon = meta.icon

  return (
    <Badge variant='outline' className={meta.className}>
      <Icon className='size-3' />
      {t(meta.label)}
    </Badge>
  )
}

function TypeBadge({ type }: { type: TicketType }) {
  const { t } = useTranslation()
  return (
    <Badge variant='secondary' className='capitalize'>
      {t(getTypeLabel(type))}
    </Badge>
  )
}

function MessageItem({ message }: { message: TicketMessage }) {
  const { t } = useTranslation()

  if (message.type === 'status') {
    const status = message.value ?? 'pending'
    return (
      <div className='flex justify-center'>
        <div className='text-muted-foreground bg-muted/70 flex flex-wrap items-center justify-center gap-1 rounded-lg px-3 py-1.5 text-xs'>
          <span>{message.username}</span>
          <span>{t('tickets.fields.changedStatusTo')}</span>
          <StatusBadge status={status} />
          <span>{formatTime(message.time)}</span>
        </div>
      </div>
    )
  }

  const isAdmin = message.role === 'admin'

  return (
    <div
      className={cn(
        'flex flex-col gap-1',
        isAdmin ? 'items-start' : 'items-end'
      )}
    >
      <div className='text-muted-foreground flex max-w-full items-baseline gap-2 text-xs'>
        <span className={isAdmin ? 'text-primary font-medium' : 'font-medium'}>
          {message.username}
        </span>
        <span>{formatTime(message.time)}</span>
      </div>
      <div
        className={cn(
          'max-w-full rounded-lg px-3 py-2 text-sm leading-6 wrap-break-word whitespace-pre-wrap sm:max-w-[78%]',
          isAdmin ? 'bg-primary/10 text-foreground' : 'bg-muted text-foreground'
        )}
      >
        {message.content}
      </div>
    </div>
  )
}

type TicketDetailDialogProps = {
  open: boolean
  onOpenChange: (open: boolean) => void
  ticket: TicketBase | null
  detail: TicketDetail | null
  loading: boolean
  sending: boolean
  isAdmin: boolean
  replyText: string
  onReplyTextChange: (value: string) => void
  onSendReply: () => void
  onStatusAction: (ticket: TicketBase, action: 'close' | 'reopen') => void
}

function TicketDetailDialog({
  open,
  onOpenChange,
  ticket,
  detail,
  loading,
  sending,
  isAdmin,
  replyText,
  onReplyTextChange,
  onSendReply,
  onStatusAction,
}: TicketDetailDialogProps) {
  const { t } = useTranslation()
  const messages = detail?.messages ?? []
  const displayTicket = detail ?? ticket
  const canClose = Boolean(
    displayTicket && displayTicket.status !== 'completed'
  )
  const canReopen = Boolean(
    isAdmin && displayTicket && displayTicket.status === 'completed'
  )

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className='flex max-h-[calc(100vh-2rem)] max-w-3xl grid-rows-[auto_minmax(0,1fr)_auto] flex-col gap-0 p-0 sm:max-w-3xl'>
        <DialogHeader className='border-b p-4'>
          <DialogTitle className='flex min-w-0 flex-wrap items-center gap-2 pr-8'>
            <span className='truncate'>
              {displayTicket?.title || t('tickets.fields.ticketDetail')}
            </span>
            {displayTicket ? (
              <StatusBadge status={displayTicket.status} />
            ) : null}
            {displayTicket ? <TypeBadge type={displayTicket.type} /> : null}
          </DialogTitle>
        </DialogHeader>

        <div className='min-h-80 flex-1 overflow-y-auto p-4'>
          {loading ? (
            <div className='text-muted-foreground flex h-48 items-center justify-center gap-2 text-sm'>
              <Loader2 className='size-4 animate-spin' />
              {t('common.tips.loading')}
            </div>
          ) : messages.length === 0 ? (
            <div className='text-muted-foreground flex h-48 items-center justify-center text-sm'>
              {t('tickets.fields.noMessages')}
            </div>
          ) : (
            <div className='space-y-4'>
              {messages.map((message) => (
                <MessageItem key={message.id} message={message} />
              ))}
            </div>
          )}
        </div>

        <div className='border-t p-4'>
          <div className='flex flex-col gap-2'>
            <Label htmlFor='ticket-reply'>{t('tickets.fields.reply')}</Label>
            <div className='flex flex-col gap-2 sm:flex-row sm:items-end'>
              <Textarea
                id='ticket-reply'
                value={replyText}
                placeholder={t('tickets.tips.typeAReply')}
                className='min-h-20 flex-1 resize-none'
                disabled={loading || sending || !displayTicket}
                onChange={(event) => onReplyTextChange(event.target.value)}
                onKeyDown={(event) => {
                  if (event.key === 'Enter' && !event.shiftKey) {
                    event.preventDefault()
                    onSendReply()
                  }
                }}
              />
              <Button
                className='sm:w-auto'
                disabled={
                  !replyText.trim() || loading || sending || !displayTicket
                }
                onClick={onSendReply}
              >
                {sending ? (
                  <Loader2 className='size-4 animate-spin' />
                ) : (
                  <SendHorizontal className='size-4' />
                )}
                {t('playground.actions.send')}
              </Button>
            </div>
          </div>
        </div>

        <DialogFooter className='rounded-b-xl'>
          <DialogClose render={<Button variant='outline' />}>
            {t('common.actions.close')}
          </DialogClose>
          {canReopen && displayTicket ? (
            <Button
              variant='outline'
              onClick={() => onStatusAction(displayTicket, 'reopen')}
            >
              <RotateCcw className='size-4' />
              {t('tickets.fields.reopen')}
            </Button>
          ) : null}
          {canClose && displayTicket ? (
            <Button
              variant='destructive'
              onClick={() => onStatusAction(displayTicket, 'close')}
            >
              {t('tickets.actions.closeTicket')}
            </Button>
          ) : null}
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}

export function Tickets() {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const authUser = useAuthStore((state) => state.auth.user)
  const isAdmin = (authUser?.role ?? 0) >= ROLE.ADMIN
  const [status, setStatus] = useState<TicketStatusFilter>('all')
  const [keywordInput, setKeywordInput] = useState('')
  const [keyword, setKeyword] = useState('')
  const [page, setPage] = useState(1)
  const [pageSize, setPageSize] = useState(DEFAULT_PAGE_SIZE)
  const [createOpen, setCreateOpen] = useState(false)
  const [createForm, setCreateForm] =
    useState<CreateTicketPayload>(EMPTY_CREATE_FORM)
  const [detailOpen, setDetailOpen] = useState(false)
  const [previewTicket, setPreviewTicket] = useState<TicketSummary | null>(null)
  const [ticketDetail, setTicketDetail] = useState<TicketDetail | null>(null)
  const [replyText, setReplyText] = useState('')
  const [statusTarget, setStatusTarget] = useState<StatusActionTarget | null>(
    null
  )

  const ticketsQuery = useQuery({
    queryKey: ['tickets', isAdmin, status, keyword, page, pageSize],
    queryFn: () =>
      getTickets({
        isAdmin,
        status,
        keyword,
        page,
        pageSize,
      }),
  })

  const tickets = ticketsQuery.data?.items ?? []
  const total = ticketsQuery.data?.total ?? 0
  const totalPages = Math.max(1, Math.ceil(total / pageSize))
  const currentTicket = ticketDetail ?? previewTicket

  const refreshTickets = async () => {
    await queryClient.invalidateQueries({ queryKey: ['tickets'] })
  }

  const { mutate: mutateTicketDetail, isPending: detailLoading } = useMutation({
    mutationFn: getTicketDetail,
    onSuccess: (detail) => {
      setTicketDetail(detail)
      setDetailOpen(true)
    },
    onError: (error) => toast.error(t(error.message)),
  })

  const createMutation = useMutation({
    mutationFn: createTicket,
    onSuccess: async (detail) => {
      toast.success(t('tickets.status.ticketCreated'))
      setCreateOpen(false)
      setCreateForm(EMPTY_CREATE_FORM)
      setStatus('all')
      setKeyword('')
      setKeywordInput('')
      setPage(1)
      setPreviewTicket(null)
      setTicketDetail(detail)
      setDetailOpen(true)
      await refreshTickets()
    },
    onError: (error) => toast.error(t(error.message)),
  })

  const replyMutation = useMutation({
    mutationFn: async (payload: { ticketId: number; content: string }) => {
      await replyTicket(payload.ticketId, payload.content)
      return payload.ticketId
    },
    onSuccess: async (ticketId) => {
      toast.success(t('tickets.fields.replySent'))
      setReplyText('')
      await refreshTickets()
      mutateTicketDetail(ticketId)
    },
    onError: (error) => toast.error(t(error.message)),
  })

  const statusMutation = useMutation({
    mutationFn: async (target: StatusActionTarget) => {
      if (target.action === 'close') {
        await closeTicket(target.ticket.id)
      } else {
        await reopenTicket(target.ticket.id)
      }
      return target
    },
    onSuccess: async (target) => {
      toast.success(
        target.action === 'close'
          ? t('tickets.fields.ticketClosed')
          : t('tickets.fields.ticketReopened')
      )
      setStatusTarget(null)
      await refreshTickets()
      if (detailOpen && currentTicket?.id === target.ticket.id) {
        mutateTicketDetail(target.ticket.id)
      }
    },
    onError: (error) => toast.error(error.message),
  })

  const runSearch = () => {
    setKeyword(keywordInput.trim())
    setPage(1)
  }

  const resetFilters = () => {
    setStatus('all')
    setKeyword('')
    setKeywordInput('')
    setPage(1)
  }

  const openDetail = useCallback(
    (ticket: TicketSummary) => {
      setPreviewTicket(ticket)
      setTicketDetail(null)
      setReplyText('')
      setDetailOpen(true)
      mutateTicketDetail(ticket.id)
    },
    [mutateTicketDetail]
  )

  const openStatusAction = useCallback(
    (ticket: TicketSummary, action: 'close' | 'reopen') => {
      setStatusTarget({ ticket, action })
    },
    []
  )

  const columns = useMemo<ColumnDef<TicketSummary>[]>(
    () => [
      {
        accessorKey: 'title',
        header: () => t('tickets.fields.title'),
        meta: { label: t('tickets.fields.title'), mobileTitle: true },
        cell: ({ row }) => (
          <div className='max-w-96 truncate font-medium'>
            {row.original.title}
          </div>
        ),
      },
      {
        accessorKey: 'type',
        header: () => t('channels.fields.type'),
        meta: { label: t('channels.fields.type'), mobileHidden: true },
        cell: ({ row }) => <TypeBadge type={row.original.type} />,
      },
      {
        accessorKey: 'status',
        header: () => t('channels.fields.status'),
        meta: { label: t('channels.fields.status'), mobileBadge: true },
        cell: ({ row }) => <StatusBadge status={row.original.status} />,
      },
      {
        accessorKey: 'created_at',
        header: () => t('multimodalFiles.status.createdAt'),
        meta: { label: t('multimodalFiles.status.createdAt') },
        cell: ({ row }) => formatTime(row.original.created_at),
      },
      {
        accessorKey: 'updated_at',
        header: () => t('models.status.updatedAt'),
        meta: { label: t('models.status.updatedAt') },
        cell: ({ row }) => formatTime(row.original.updated_at),
      },
      {
        id: 'actions',
        header: () => (
          <div className='text-right'>{t('channels.fields.actions')}</div>
        ),
        meta: { label: t('channels.fields.actions') },
        size: 56,
        cell: ({ row }) => {
          const ticket = row.original
          return (
            <div className='text-right'>
              <DropdownMenu modal={false}>
                <DropdownMenuTrigger
                  render={
                    <Button
                      variant='ghost'
                      className='data-popup-open:bg-muted h-8 w-8 p-0'
                      onClick={(event) => event.stopPropagation()}
                    />
                  }
                >
                  <MoreHorizontal className='size-4' />
                  <span className='sr-only'>
                    {t('channels.actions.openMenu')}
                  </span>
                </DropdownMenuTrigger>
                <DropdownMenuContent align='end' className='w-40'>
                  <DropdownMenuItem
                    onClick={(event) => {
                      event.stopPropagation()
                      openDetail(ticket)
                    }}
                  >
                    <Eye className='size-4' />
                    {t('models.actions.viewDetails')}
                  </DropdownMenuItem>
                  {ticket.status === 'completed' && isAdmin ? (
                    <DropdownMenuItem
                      onClick={(event) => {
                        event.stopPropagation()
                        openStatusAction(ticket, 'reopen')
                      }}
                    >
                      <RotateCcw className='size-4' />
                      {t('tickets.fields.reopen')}
                    </DropdownMenuItem>
                  ) : null}
                  {ticket.status !== 'completed' ? (
                    <DropdownMenuItem
                      variant='destructive'
                      onClick={(event) => {
                        event.stopPropagation()
                        openStatusAction(ticket, 'close')
                      }}
                    >
                      {t('tickets.actions.closeTicket')}
                    </DropdownMenuItem>
                  ) : null}
                </DropdownMenuContent>
              </DropdownMenu>
            </div>
          )
        },
      },
    ],
    [t, isAdmin, openDetail, openStatusAction]
  )

  const table = useTable({
    features: appTableFeatures,
    data: tickets,
    columns,
    state: {
      pagination: { pageIndex: page - 1, pageSize },
    },
    enableRowSelection: false,
    manualPagination: true,
    manualFiltering: true,
    pageCount: totalPages,
    onPaginationChange: (updater) => {
      const next =
        typeof updater === 'function'
          ? updater({ pageIndex: page - 1, pageSize })
          : updater
      setPage(next.pageIndex + 1)
      setPageSize(next.pageSize)
    },
  })

  const submitCreate = () => {
    const title = createForm.title.trim()
    const content = createForm.content.trim()
    if (!title) {
      toast.error(t('tickets.errors.pleaseEnterATicketTitle'))
      return
    }
    if (!content) {
      toast.error(t('tickets.errors.pleaseEnterTicketContent'))
      return
    }
    createMutation.mutate({ ...createForm, title, content })
  }

  const sendReply = () => {
    if (!currentTicket || !replyText.trim()) return
    replyMutation.mutate({
      ticketId: currentTicket.id,
      content: replyText.trim(),
    })
  }

  const confirmTitle =
    statusTarget?.action === 'close'
      ? t('tickets.actions.closeTicket')
      : t('tickets.fields.reopenTicket')
  const confirmDescription =
    statusTarget?.action === 'close'
      ? t('tickets.tips.ticketStatusWillChangeToCompleted')
      : t('tickets.status.ticketStatusWillChangeToPending')

  const createDialog = (
    <Dialog open={createOpen} onOpenChange={setCreateOpen}>
      <DialogContent className='sm:max-w-lg'>
        <DialogHeader>
          <DialogTitle>{t('tickets.fields.newTicket')}</DialogTitle>
        </DialogHeader>

        <div className='space-y-4'>
          <div className='space-y-2'>
            <Label htmlFor='ticket-title'>{t('tickets.fields.title')}</Label>
            <Input
              id='ticket-title'
              value={createForm.title}
              placeholder={t('tickets.placeholders.enterAConciseTitle')}
              onChange={(event) =>
                setCreateForm((current) => ({
                  ...current,
                  title: event.target.value,
                }))
              }
            />
          </div>

          <div className='space-y-2'>
            <Label>{t('channels.fields.type')}</Label>
            <Select
              value={createForm.type}
              onValueChange={(value) =>
                setCreateForm((current) => ({
                  ...current,
                  type: value as TicketType,
                }))
              }
            >
              <SelectTrigger className='w-full'>
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                <SelectGroup>
                  {TYPE_OPTIONS.map((option) => (
                    <SelectItem key={option.value} value={option.value}>
                      {t(option.label)}
                    </SelectItem>
                  ))}
                </SelectGroup>
              </SelectContent>
            </Select>
          </div>

          <div className='space-y-2'>
            <Label htmlFor='ticket-content'>
              {t('dashboard.fields.content')}
            </Label>
            <Textarea
              id='ticket-content'
              value={createForm.content}
              placeholder={t('tickets.tips.describeTheQuestionOrRequest')}
              className='min-h-32 resize-none'
              onChange={(event) =>
                setCreateForm((current) => ({
                  ...current,
                  content: event.target.value,
                }))
              }
            />
          </div>
        </div>

        <DialogFooter>
          <DialogClose render={<Button variant='outline' />}>
            {t('common.actions.cancel')}
          </DialogClose>
          <Button onClick={submitCreate} disabled={createMutation.isPending}>
            {createMutation.isPending ? (
              <Loader2 className='size-4 animate-spin' />
            ) : (
              <Plus className='size-4' />
            )}
            {t('common.actions.submit')}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )

  return (
    <>
      <SectionPageLayout>
        <SectionPageLayout.Title>
          {t('systemSettings.fields.tickets')}
        </SectionPageLayout.Title>
        <SectionPageLayout.Actions>
          <Button variant='outline' onClick={() => ticketsQuery.refetch()}>
            <RefreshCw
              className={cn(
                'size-4',
                ticketsQuery.isFetching && 'animate-spin'
              )}
            />
            {t('channels.actions.refresh')}
          </Button>
          <Button
            onClick={() => {
              setCreateForm(EMPTY_CREATE_FORM)
              setCreateOpen(true)
            }}
          >
            <Plus className='size-4' />
            {t('tickets.fields.newTicket')}
          </Button>
        </SectionPageLayout.Actions>

        <SectionPageLayout.Content>
          <DataTablePage
            table={table}
            columns={columns}
            isLoading={ticketsQuery.isLoading}
            isFetching={ticketsQuery.isFetching}
            emptyTitle={t('tickets.fields.noTickets')}
            skeletonKeyPrefix='tickets-skeleton'
            toolbar={
              <div className='flex flex-col gap-3 lg:flex-row lg:items-center lg:justify-between'>
                <Tabs
                  value={status}
                  onValueChange={(value) => {
                    setStatus(value as TicketStatusFilter)
                    setPage(1)
                  }}
                >
                  <TabsList className='max-w-full flex-wrap'>
                    {STATUS_OPTIONS.map((option) => (
                      <TabsTrigger key={option.value} value={option.value}>
                        {t(option.label)}
                      </TabsTrigger>
                    ))}
                  </TabsList>
                </Tabs>

                <div className='flex flex-col gap-2 sm:flex-row'>
                  <div className='relative sm:w-72'>
                    <Search className='text-muted-foreground pointer-events-none absolute top-1/2 left-2.5 size-4 -translate-y-1/2' />
                    <Input
                      value={keywordInput}
                      className='pl-8'
                      placeholder={t('tickets.actions.searchTickets')}
                      onChange={(event) => setKeywordInput(event.target.value)}
                      onKeyDown={(event) => {
                        if (event.key === 'Enter') runSearch()
                      }}
                    />
                  </div>
                  <Button variant='outline' onClick={runSearch}>
                    {t('common.actions.search')}
                  </Button>
                  <Button variant='ghost' onClick={resetFilters}>
                    {t('common.actions.reset')}
                  </Button>
                </div>
              </div>
            }
            renderRow={(row) => (
              <TableRow
                key={row.id}
                className='cursor-pointer'
                onClick={() => openDetail(row.original)}
              >
                {row.getVisibleCells().map((cell) => (
                  <TableCell key={cell.id}>
                    {flexRender(cell.column.columnDef.cell, cell.getContext())}
                  </TableCell>
                ))}
              </TableRow>
            )}
          />
        </SectionPageLayout.Content>
      </SectionPageLayout>

      {createDialog}

      <TicketDetailDialog
        open={detailOpen}
        onOpenChange={(open) => {
          setDetailOpen(open)
          if (!open) {
            setPreviewTicket(null)
            setTicketDetail(null)
            setReplyText('')
          }
        }}
        ticket={previewTicket}
        detail={ticketDetail}
        loading={detailLoading}
        sending={replyMutation.isPending}
        isAdmin={isAdmin}
        replyText={replyText}
        onReplyTextChange={setReplyText}
        onSendReply={sendReply}
        onStatusAction={(ticket, action) => setStatusTarget({ ticket, action })}
      />

      <ConfirmDialog
        open={Boolean(statusTarget)}
        onOpenChange={(open) => {
          if (!open && !statusMutation.isPending) setStatusTarget(null)
        }}
        title={confirmTitle}
        desc={confirmDescription}
        destructive={statusTarget?.action === 'close'}
        confirmText={
          statusTarget?.action === 'close'
            ? t('tickets.actions.closeTicket')
            : t('tickets.fields.reopen')
        }
        isLoading={statusMutation.isPending}
        handleConfirm={() => {
          if (statusTarget) statusMutation.mutate(statusTarget)
        }}
      />
    </>
  )
}
