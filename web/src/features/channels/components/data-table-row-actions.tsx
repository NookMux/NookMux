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
import {
  MoreHorizontal,
  Boxes,
  Pencil,
  Gauge,
  DollarSign,
  Download,
  Copy,
  Power,
  PowerOff,
  Key,
  Trash2,
  Loader2,
  PackageSearch,
  ShieldAlert,
  CreditCard,
  Phone,
} from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { type Row } from '@/lib/tanstack-table'
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
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuShortcut,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu'
import {
  Tooltip,
  TooltipContent,
  TooltipTrigger,
} from '@/components/ui/tooltip'
import { ConfirmDialog } from '@/components/confirm-dialog'
import { getGlmContactInfo, getGlmRiskStatus } from '../api'
import {
  handleDeleteChannel,
  handleToggleChannelStatus,
  isChannelEnabled,
  isMultiKeyChannel,
  isPlanChannel,
  isZhipuChannel,
} from '../lib'
import type { Channel, GlmContactData } from '../types'
import { useChannels } from './channels-provider'

interface DataTableRowActionsProps {
  row: Row<Channel>
}

export function DataTableRowActions({ row }: DataTableRowActionsProps) {
  const { t } = useTranslation()
  const channel = row.original
  const { setOpen, setCurrentRow } = useChannels()
  const queryClient = useQueryClient()
  const [deleteConfirmOpen, setDeleteConfirmOpen] = useState(false)
  const [riskDialogOpen, setRiskDialogOpen] = useState(false)
  const [riskDetected, setRiskDetected] = useState(false)
  const [isTogglingStatus, setIsTogglingStatus] = useState(false)
  const [isCheckingRisk, setIsCheckingRisk] = useState(false)
  const [contactDialogOpen, setContactDialogOpen] = useState(false)
  const [contactInfo, setContactInfo] = useState<GlmContactData | null>(null)
  const [isCheckingContact, setIsCheckingContact] = useState(false)

  const isEnabled = isChannelEnabled(channel)
  const isMultiKey = isMultiKeyChannel(channel)
  const isPlan = isPlanChannel(channel)
  const isZhipu = isZhipuChannel(channel)
  const isGlmPlan =
    channel.channel_info?.plan_name === 'glm-coding-plan' ||
    channel.channel_info?.plan_name === 'glm-coding-plan-international'

  const handleEdit = () => {
    setCurrentRow(channel)
    setOpen('update-channel')
  }

  const handleTest = () => {
    setCurrentRow(channel)
    setOpen('test-channel')
  }

  const handleQueryBalance = () => {
    setCurrentRow(channel)
    setOpen('balance-query')
  }

  const handleFetchModels = () => {
    setCurrentRow(channel)
    setOpen('fetch-models')
  }

  const handleManageOllamaModels = () => {
    setCurrentRow(channel)
    setOpen('ollama-models')
  }

  const handleCopy = () => {
    setCurrentRow(channel)
    setOpen('copy-channel')
  }

  const handleManageKeys = () => {
    setCurrentRow(channel)
    setOpen('multi-key-manage')
  }

  const handlePlanQuota = () => {
    setCurrentRow(channel)
    setOpen('plan-quota')
  }

  const handleResetCards = () => {
    setCurrentRow(channel)
    setOpen('reset-cards')
  }

  const handleCheckRisk = async () => {
    setIsCheckingRisk(true)
    try {
      const response = await getGlmRiskStatus(channel.id)
      if (!response.success) {
        throw new Error(
          response.message || t('channels.status.riskQueryFailed')
        )
      }
      setRiskDetected(Boolean(response.data?.is_risk))
      setRiskDialogOpen(true)
    } catch (error) {
      toast.error(
        error instanceof Error
          ? error.message
          : t('channels.status.riskQueryFailed')
      )
    } finally {
      setIsCheckingRisk(false)
    }
  }

  const handleQueryContact = async () => {
    setIsCheckingContact(true)
    try {
      const response = await getGlmContactInfo(channel.id)
      const data = response.data
      if (!response.success || !data) {
        throw new Error(
          response.message || t('channels.status.contactQueryFailed')
        )
      }
      setContactInfo(data)
      setContactDialogOpen(true)
    } catch (error) {
      toast.error(
        error instanceof Error
          ? error.message
          : t('channels.status.contactQueryFailed')
      )
    } finally {
      setIsCheckingContact(false)
    }
  }

  const handleToggleStatus = async (
    e?: React.MouseEvent<HTMLButtonElement>
  ) => {
    e?.stopPropagation()
    setIsTogglingStatus(true)
    try {
      await handleToggleChannelStatus(channel.id, channel.status, queryClient)
    } finally {
      setIsTogglingStatus(false)
    }
  }

  // 联系方式查询：仅智谱 GLM 渠道显示，不限套餐。套餐渠道放在【套餐】用量查询前，
  // 非套餐渠道放在复制渠道后；尺寸与风控查询菜单项保持一致。
  const contactMenuItem = isZhipu ? (
    <DropdownMenuItem onClick={handleQueryContact} disabled={isCheckingContact}>
      {isCheckingContact
        ? t('channels.tips.querying')
        : t('channels.titles.contactQuery')}
      <DropdownMenuShortcut>
        {isCheckingContact ? (
          <Loader2 size={16} className='animate-spin' />
        ) : (
          <Phone size={16} />
        )}
      </DropdownMenuShortcut>
    </DropdownMenuItem>
  ) : null

  return (
    <div className='flex items-center justify-end gap-1'>
      <Tooltip>
        <TooltipTrigger
          render={
            <Button
              variant='ghost'
              size='icon-sm'
              onClick={handleTest}
              aria-label={t('channels.fields.testConnection')}
            />
          }
        >
          <Gauge className='size-4' />
        </TooltipTrigger>
        <TooltipContent>{t('channels.fields.testConnection')}</TooltipContent>
      </Tooltip>

      <Tooltip>
        <TooltipTrigger
          render={
            <Button
              variant='ghost'
              size='icon-sm'
              onClick={handleToggleStatus}
              disabled={isTogglingStatus}
              aria-label={
                isEnabled
                  ? t('channels.actions.disable')
                  : t('channels.actions.enable')
              }
              className={
                isEnabled
                  ? 'text-destructive hover:text-destructive'
                  : 'text-emerald-600 hover:text-emerald-600 dark:text-emerald-400 dark:hover:text-emerald-400'
              }
            />
          }
        >
          {isTogglingStatus ? (
            <Loader2 className='size-4 animate-spin' />
          ) : isEnabled ? (
            <PowerOff className='size-4' />
          ) : (
            <Power className='size-4' />
          )}
        </TooltipTrigger>
        <TooltipContent>
          {isEnabled
            ? t('channels.actions.disable')
            : t('channels.actions.enable')}
        </TooltipContent>
      </Tooltip>

      <DropdownMenu>
        <DropdownMenuTrigger
          render={
            <Button
              variant='ghost'
              className='data-popup-open:bg-muted flex h-8 w-8 p-0'
            />
          }
        >
          <MoreHorizontal className='h-4 w-4' />
          <span className='sr-only'>{t('channels.actions.openMenu')}</span>
        </DropdownMenuTrigger>
        <DropdownMenuContent align='end' className='w-48'>
          {/* Edit */}
          <DropdownMenuItem onClick={handleEdit}>
            {t('channels.actions.edit')}
            <DropdownMenuShortcut>
              <Pencil size={16} />
            </DropdownMenuShortcut>
          </DropdownMenuItem>

          {/* Query Balance */}
          <DropdownMenuItem onClick={handleQueryBalance}>
            {t('channels.titles.queryBalance')}
            <DropdownMenuShortcut>
              <DollarSign size={16} />
            </DropdownMenuShortcut>
          </DropdownMenuItem>

          {/* Fetch Models */}
          <DropdownMenuItem onClick={handleFetchModels}>
            {t('channels.titles.fetchModels')}
            <DropdownMenuShortcut>
              <Download size={16} />
            </DropdownMenuShortcut>
          </DropdownMenuItem>

          {/* Ollama Models (only for Ollama channels) */}
          {channel.type === 4 && (
            <DropdownMenuItem onClick={handleManageOllamaModels}>
              {t('channels.titles.manageOllamaModels')}
              <DropdownMenuShortcut>
                <Boxes size={16} />
              </DropdownMenuShortcut>
            </DropdownMenuItem>
          )}

          <DropdownMenuSeparator />

          {/* Copy Channel */}
          <DropdownMenuItem onClick={handleCopy}>
            {t('channels.actions.copyChannel')}
            <DropdownMenuShortcut>
              <Copy size={16} />
            </DropdownMenuShortcut>
          </DropdownMenuItem>

          {/* Contact Query (non-plan Zhipu channels: after Copy Channel) */}
          {!isPlan && contactMenuItem}

          {/* Manage Keys (only for multi-key channels) */}
          {isMultiKey && (
            <DropdownMenuItem onClick={handleManageKeys}>
              {t('channels.fields.manageKeys')}
              <DropdownMenuShortcut>
                <Key size={16} />
              </DropdownMenuShortcut>
            </DropdownMenuItem>
          )}

          {/* Contact Query (plan Zhipu channels: before Plan Usage) */}
          {isPlan && contactMenuItem}

          {isPlan && (
            <DropdownMenuItem onClick={handlePlanQuota}>
              {t('channels.fields.planUsage')}
              <DropdownMenuShortcut>
                <PackageSearch size={16} />
              </DropdownMenuShortcut>
            </DropdownMenuItem>
          )}

          {isPlan && isGlmPlan && (
            <DropdownMenuItem
              onClick={handleCheckRisk}
              disabled={isCheckingRisk}
            >
              {isCheckingRisk
                ? t('channels.tips.querying')
                : t('channels.titles.riskQuery')}
              <DropdownMenuShortcut>
                {isCheckingRisk ? (
                  <Loader2 size={16} className='animate-spin' />
                ) : (
                  <ShieldAlert size={16} />
                )}
              </DropdownMenuShortcut>
            </DropdownMenuItem>
          )}

          {isPlan && isGlmPlan && (
            <DropdownMenuItem onClick={handleResetCards}>
              {t('channels.actions.resetCards')}
              <DropdownMenuShortcut>
                <CreditCard size={16} />
              </DropdownMenuShortcut>
            </DropdownMenuItem>
          )}

          <DropdownMenuSeparator />

          {/* Delete */}
          <DropdownMenuItem
            onSelect={(e) => {
              e.preventDefault()
              setDeleteConfirmOpen(true)
            }}
            className='text-destructive focus:text-destructive'
          >
            {t('common.actions.delete')}
            <DropdownMenuShortcut>
              <Trash2 size={16} />
            </DropdownMenuShortcut>
          </DropdownMenuItem>
        </DropdownMenuContent>
      </DropdownMenu>

      <ConfirmDialog
        open={deleteConfirmOpen}
        onOpenChange={setDeleteConfirmOpen}
        title={t('channels.actions.deleteChannel')}
        desc={`Are you sure you want to delete "${channel.name}"? This action cannot be undone.`}
        confirmText={t('common.actions.delete')}
        destructive
        handleConfirm={() => {
          handleDeleteChannel(channel.id, queryClient)
          setDeleteConfirmOpen(false)
        }}
      />

      <AlertDialog open={riskDialogOpen} onOpenChange={setRiskDialogOpen}>
        <AlertDialogContent className='sm:max-w-md'>
          <AlertDialogHeader className='text-start'>
            <AlertDialogTitle>
              {t('channels.titles.riskQuery')}
            </AlertDialogTitle>
            <AlertDialogDescription render={<div />}>
              {riskDetected ? (
                <div className='space-y-2'>
                  <p>{t('channels.tips.accountHasBeenRiskControlled')}</p>
                  <p>
                    {t('channels.fields.pleaseVisit')}{' '}
                    <a
                      href='https://open.bigmodel.cn'
                      target='_blank'
                      rel='noreferrer'
                      className='underline underline-offset-4'
                    >
                      智谱开发者平台
                    </a>{' '}
                    {t('channels.tips.resolutionSteps')}
                  </p>
                </div>
              ) : (
                <p>{t('channels.tips.accountStatusIsNormal')}</p>
              )}
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            {riskDetected && (
              <AlertDialogCancel
                onClick={() => {
                  window.open('https://open.bigmodel.cn', '_blank', 'noopener')
                }}
              >
                {t('channels.actions.openPlatform')}
              </AlertDialogCancel>
            )}
            <AlertDialogAction onClick={() => setRiskDialogOpen(false)}>
              {t('channels.actions.ok')}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>

      <AlertDialog open={contactDialogOpen} onOpenChange={setContactDialogOpen}>
        <AlertDialogContent className='sm:max-w-md'>
          <AlertDialogHeader className='text-start'>
            <AlertDialogTitle>
              {t('channels.titles.contactQuery')}
            </AlertDialogTitle>
            <AlertDialogDescription render={<div />}>
              {contactInfo ? (
                <div className='space-y-2'>
                  {contactInfo.name && (
                    <p>
                      {t('channels.fields.contactName')}: {contactInfo.name}
                    </p>
                  )}
                  {contactInfo.phone && (
                    <p>
                      {t('channels.fields.contactPhone')}: {contactInfo.phone}
                    </p>
                  )}
                  {!contactInfo.name && !contactInfo.phone && (
                    <p>{contactInfo.raw_name}</p>
                  )}
                </div>
              ) : (
                <p>{t('channels.status.contactQueryFailed')}</p>
              )}
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogAction onClick={() => setContactDialogOpen(false)}>
              {t('channels.actions.ok')}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </div>
  )
}
