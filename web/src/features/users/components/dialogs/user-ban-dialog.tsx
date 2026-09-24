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
import { useForm } from 'react-hook-form'
import { zodResolver } from '@hookform/resolvers/zod'
import { Loader2 } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { ROLE } from '@/lib/roles'
import { Button } from '@/components/ui/button'
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
import {
  Select,
  SelectContent,
  SelectGroup,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import {
  Tooltip,
  TooltipContent,
  TooltipProvider,
  TooltipTrigger,
} from '@/components/ui/tooltip'
import { StatusBadge } from '@/components/status-badge'
import { banUserByIdentifier, manageUser } from '../../api'
import { USER_STATUS } from '../../constants'
import {
  BAN_USER_FORM_DEFAULT_VALUES,
  banUserSchema,
  banValuePattern,
  type BanUserFormValues,
} from '../../lib/user-ban-schema'
import type { BanCandidateUser, BanResult } from '../../types'
import { useUsers } from '../users-provider'

type BanDialogView = 'form' | 'candidates' | 'done'

const identifierTypeOptions = [
  { value: 'github_id', labelKey: 'users.ban.type.githubId' },
  { value: 'linuxdo_id', labelKey: 'users.ban.type.linuxdoId' },
  { value: 'email', labelKey: 'users.ban.type.email' },
  { value: 'github_username', labelKey: 'users.ban.type.githubUsername' },
] as const

const identifierPlaceholderKeys = {
  github_id: 'users.placeholders.banGithubId',
  linuxdo_id: 'users.placeholders.banLinuxdoId',
  email: 'users.placeholders.banEmail',
  github_username: 'users.placeholders.banGithubUsername',
} as const

const resultMessageKeys = {
  banned_existing: 'users.ban.result.bannedExisting',
  created_placeholder: 'users.ban.result.createdPlaceholder',
  already_banned: 'users.ban.result.alreadyBanned',
  already_deleted: 'users.ban.result.alreadyDeleted',
  ambiguous: 'users.ban.result.ambiguous',
} as const

export function UserBanDialog() {
  const { t } = useTranslation()
  const { open, setOpen, triggerRefresh } = useUsers()

  const [view, setView] = useState<BanDialogView>('form')
  const [candidates, setCandidates] = useState<BanCandidateUser[]>([])
  const [bannedIds, setBannedIds] = useState<Set<number>>(new Set())
  const [banningId, setBanningId] = useState<number | null>(null)
  const [banningAll, setBanningAll] = useState(false)
  const [hasBanned, setHasBanned] = useState(false)
  const [doneResult, setDoneResult] = useState<{
    result: BanResult
    user?: BanCandidateUser
  } | null>(null)

  const form = useForm<BanUserFormValues>({
    resolver: zodResolver(banUserSchema),
    defaultValues: BAN_USER_FORM_DEFAULT_VALUES,
  })
  const selectedType = form.watch('type')
  const submitting = form.formState.isSubmitting

  const resetState = () => {
    setView('form')
    setCandidates([])
    setBannedIds(new Set())
    setBanningId(null)
    setBanningAll(false)
    setHasBanned(false)
    setDoneResult(null)
    form.reset(BAN_USER_FORM_DEFAULT_VALUES)
  }

  const handleOpenChange = (isOpen: boolean) => {
    if (!isOpen) {
      if (hasBanned) {
        triggerRefresh()
      }
      resetState()
      setOpen(null)
    }
  }

  const submitHandler = form.handleSubmit(async (values) => {
    const value = values.value.trim()
    if (!banValuePattern[values.type].test(value)) {
      form.setError('value', { message: t('users.ban.errors.invalidValue') })
      return
    }
    try {
      const result = await banUserByIdentifier({ type: values.type, value })
      if (!result.success || !result.data) {
        toast.error(result.message || t('users.ban.errors.failedToBan'))
        return
      }
      const data = result.data
      if (data.result === 'ambiguous') {
        setCandidates(data.candidates ?? [])
        setView('candidates')
        return
      }
      if (
        data.result === 'banned_existing' ||
        data.result === 'created_placeholder'
      ) {
        setHasBanned(true)
      }
      setDoneResult({ result: data.result, user: data.user })
      setView('done')
    } catch (e: unknown) {
      toast.error(
        e instanceof Error ? e.message : t('users.ban.errors.failedToBan')
      )
    }
  })

  const canBan = (candidate: BanCandidateUser) =>
    !candidate.deleted &&
    candidate.status !== USER_STATUS.DISABLED &&
    candidate.role < ROLE.SUPER_ADMIN

  const banCandidate = async (
    candidate: BanCandidateUser
  ): Promise<boolean> => {
    try {
      const result = await manageUser(candidate.id, 'disable')
      if (result.success) {
        setBannedIds((prev) => new Set(prev).add(candidate.id))
        setHasBanned(true)
        return true
      }
      toast.error(result.message || t('users.ban.errors.failedToBan'))
    } catch (e: unknown) {
      toast.error(
        e instanceof Error ? e.message : t('users.ban.errors.failedToBan')
      )
    }
    return false
  }

  const handleBanOne = async (candidate: BanCandidateUser) => {
    setBanningId(candidate.id)
    await banCandidate(candidate)
    setBanningId(null)
  }

  const pendingCandidates = candidates.filter(
    (candidate) => canBan(candidate) && !bannedIds.has(candidate.id)
  )

  const handleBanAll = async () => {
    setBanningAll(true)
    for (const candidate of pendingCandidates) {
      await banCandidate(candidate)
    }
    setBanningAll(false)
  }

  const getBindingText = (candidate: BanCandidateUser) => {
    const parts: string[] = []
    if (candidate.github_id) {
      parts.push(`${t('users.fields.gitHubId')}: ${candidate.github_id}`)
    }
    if (candidate.linux_do_id) {
      parts.push(`${t('users.fields.linuxDoId')}: ${candidate.linux_do_id}`)
    }
    if (candidate.email) {
      parts.push(`${t('users.ban.fields.email')}: ${candidate.email}`)
    }
    return parts.join(' · ')
  }

  const renderCandidateRow = (candidate: BanCandidateUser) => {
    const banned = bannedIds.has(candidate.id)
    const bindingText = getBindingText(candidate)
    return (
      <div
        key={candidate.id}
        className='flex items-center justify-between gap-3 py-2.5'
      >
        <div className='min-w-0 flex-1 space-y-1'>
          <div className='flex items-center gap-2'>
            <span className='truncate text-sm font-medium'>
              {candidate.username}
            </span>
            {candidate.deleted ? (
              <StatusBadge
                label={t('subscriptions.actions.deleted')}
                variant='danger'
                copyable={false}
              />
            ) : (
              <StatusBadge
                label={t(
                  candidate.status === USER_STATUS.DISABLED
                    ? 'channels.status.disabled'
                    : 'channels.status.enabled'
                )}
                variant={
                  candidate.status === USER_STATUS.DISABLED
                    ? 'neutral'
                    : 'success'
                }
                copyable={false}
              />
            )}
          </div>
          {bindingText && (
            <TooltipProvider>
              <Tooltip>
                <TooltipTrigger
                  render={
                    <p className='text-muted-foreground w-full truncate text-xs' />
                  }
                >
                  {bindingText}
                </TooltipTrigger>
                <TooltipContent>{bindingText}</TooltipContent>
              </Tooltip>
            </TooltipProvider>
          )}
        </div>
        <Button
          type='button'
          size='sm'
          variant={banned ? 'outline' : 'destructive'}
          disabled={
            banned ||
            !canBan(candidate) ||
            banningId === candidate.id ||
            banningAll
          }
          onClick={() => handleBanOne(candidate)}
        >
          {banningId === candidate.id && (
            <Loader2 className='h-4 w-4 animate-spin' />
          )}
          {banned ? t('users.ban.status.banned') : t('users.ban.actions.ban')}
        </Button>
      </div>
    )
  }

  return (
    <Dialog open={open === 'ban'} onOpenChange={handleOpenChange}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>{t('users.titles.banUser')}</DialogTitle>
          <DialogDescription>
            {view === 'form' && t('users.ban.description')}
            {view === 'candidates' && t('users.ban.result.ambiguous')}
            {view === 'done' &&
              doneResult &&
              t(resultMessageKeys[doneResult.result])}
          </DialogDescription>
        </DialogHeader>

        {view === 'form' && (
          <form
            className='space-y-4'
            onSubmit={(e) => {
              e.preventDefault()
              void submitHandler(e)
            }}
          >
            <div className='space-y-2'>
              <Label>{t('users.ban.fields.identifierType')}</Label>
              <Select
                value={selectedType}
                onValueChange={(value) =>
                  form.setValue('type', value as BanUserFormValues['type'])
                }
              >
                <SelectTrigger>
                  <SelectValue>
                    {t(
                      identifierTypeOptions.find(
                        (option) => option.value === selectedType
                      )?.labelKey ?? ''
                    )}
                  </SelectValue>
                </SelectTrigger>
                <SelectContent alignItemWithTrigger={false}>
                  <SelectGroup>
                    {identifierTypeOptions.map((option) => (
                      <SelectItem key={option.value} value={option.value}>
                        {t(option.labelKey)}
                      </SelectItem>
                    ))}
                  </SelectGroup>
                </SelectContent>
              </Select>
            </div>
            <div className='space-y-2'>
              <Label>{t('users.ban.fields.identifierValue')}</Label>
              <Input
                placeholder={t(identifierPlaceholderKeys[selectedType])}
                {...form.register('value')}
              />
              {form.formState.errors.value && (
                <p className='text-destructive text-sm'>
                  {form.formState.errors.value.message}
                </p>
              )}
            </div>
            <p className='text-muted-foreground text-xs'>
              {t('users.ban.tips.emailLimitation')}
            </p>
          </form>
        )}

        {view === 'candidates' && (
          <div className='max-h-[50vh] divide-y overflow-y-auto pr-1'>
            {candidates.map(renderCandidateRow)}
          </div>
        )}

        {view === 'done' && doneResult && (
          <div className='space-y-2'>
            {doneResult.user && (
              <div className='flex items-center gap-2'>
                <span className='text-sm font-medium'>
                  {doneResult.user.username}
                </span>
                <StatusBadge
                  label={t(resultMessageKeys[doneResult.result])}
                  variant={
                    doneResult.result === 'already_banned' ||
                    doneResult.result === 'already_deleted'
                      ? 'neutral'
                      : 'danger'
                  }
                  copyable={false}
                />
              </div>
            )}
          </div>
        )}

        <DialogFooter>
          <Button variant='outline' onClick={() => handleOpenChange(false)}>
            {t('common.actions.cancel')}
          </Button>
          {view === 'form' && (
            <Button
              variant='destructive'
              disabled={submitting}
              onClick={() => {
                void submitHandler()
              }}
            >
              {submitting && <Loader2 className='h-4 w-4 animate-spin' />}
              {t('users.ban.actions.ban')}
            </Button>
          )}
          {view === 'candidates' && (
            <Button
              variant='destructive'
              disabled={banningAll || pendingCandidates.length === 0}
              onClick={() => {
                void handleBanAll()
              }}
            >
              {banningAll && <Loader2 className='h-4 w-4 animate-spin' />}
              {t('users.ban.actions.banAll')}
            </Button>
          )}
          {view !== 'form' && (
            <Button onClick={() => handleOpenChange(false)}>
              {t('users.ban.actions.done')}
            </Button>
          )}
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}
