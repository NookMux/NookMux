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
/* eslint-disable react-refresh/only-export-components */
import { useState } from 'react'
import { useQueryClient } from '@tanstack/react-query'
import {
  AlertTriangle,
  ChevronDown,
  ChevronRight,
  ListOrdered,
  Shuffle,
} from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { getCurrencyDisplay } from '@/lib/currency'
import {
  formatTimestampToDate,
  formatQuota as formatQuotaValue,
  formatQuotaCompact,
} from '@/lib/format'
import { getLobeIcon } from '@/lib/lobe-icon'
import { type ColumnDef } from '@/lib/tanstack-table'
import { truncateText } from '@/lib/utils'
import { Button } from '@/components/ui/button'
import { Checkbox } from '@/components/ui/checkbox'
import {
  Tooltip,
  TooltipContent,
  TooltipProvider,
  TooltipTrigger,
} from '@/components/ui/tooltip'
import { DataTableColumnHeader } from '@/components/data-table/column-header'
import { GroupBadge } from '@/components/group-badge'
import { StatusBadge, StatusBadgeList } from '@/components/status-badge'
import { TableId } from '@/components/table-id'
import { TruncatedText } from '@/components/truncated-text'
import { CHANNEL_STATUS_CONFIG } from '../constants'
import {
  formatBalance,
  formatBalanceCompact,
  formatRelativeTime,
  formatResponseTime,
  getBalanceVariant,
  getChannelTypeIcon,
  getChannelTypeLabel,
  getResponseTimeConfig,
  isMultiKeyChannel,
  parseModelsList,
  parseGroupsList,
  parseChannelSettings,
  handleUpdateChannelField,
  handleUpdateChannelBalance,
  isTagAggregateRow,
  type TagRow,
} from '../lib'
import type { Channel } from '../types'
import { DataTableRowActions } from './data-table-row-actions'
import { DataTableTagRowActions } from './data-table-tag-row-actions'
import { NumericSpinnerInput } from './numeric-spinner-input'

function parseIonetMeta(otherInfo: string | null | undefined): null | {
  source?: string
  deployment_id?: string
} {
  if (!otherInfo) return null
  try {
    const parsed = JSON.parse(otherInfo)
    if (parsed && typeof parsed === 'object') {
      return parsed
    }
  } catch {
    return null
  }
  return null
}

/**
 * Render limited items with "and X more" indicator
 */
function renderLimitedItems(
  items: React.ReactNode[],
  maxDisplay: number = 2
): React.ReactNode {
  return (
    <StatusBadgeList
      items={items}
      max={maxDisplay}
      renderItem={(item) => item}
    />
  )
}

/**
 * Read-only cell for tag aggregate rows — priority/weight always render '-'
 * since they are not editable at the aggregate level.
 */
function TagAggregateValue() {
  return (
    <span className='text-muted-foreground font-mono text-sm tabular-nums'>
      -
    </span>
  )
}

/**
 * Priority cell component with inline editing
 */
function PriorityCell({ channel }: { channel: Channel }) {
  const queryClient = useQueryClient()
  const priority = channel.priority

  // Tag aggregate row - read-only, edit channels individually
  if (isTagAggregateRow(channel)) {
    return <TagAggregateValue />
  }

  // Regular channel row - editable
  return (
    <NumericSpinnerInput
      value={priority ?? 0}
      onChange={(value) => {
        handleUpdateChannelField(channel.id, 'priority', value, queryClient)
      }}
      min={-999}
    />
  )
}

/**
 * Weight cell component with inline editing
 */
function WeightCell({ channel }: { channel: Channel }) {
  const queryClient = useQueryClient()
  const weight = channel.weight

  // Tag aggregate row - read-only, edit channels individually
  if (isTagAggregateRow(channel)) {
    return <TagAggregateValue />
  }

  // Regular channel row - editable
  return (
    <NumericSpinnerInput
      value={weight ?? 0}
      onChange={(value) => {
        handleUpdateChannelField(channel.id, 'weight', value, queryClient)
      }}
      min={0}
    />
  )
}

/**
 * Balance cell component with click to update
 */
function BalanceCell({ channel }: { channel: Channel }) {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const isTagRow = isTagAggregateRow(channel)
  const balance = channel.balance || 0
  const usedQuota = channel.used_quota || 0
  const [isUpdating, setIsUpdating] = useState(false)
  const { meta: currencyMeta } = getCurrencyDisplay()
  const tokenSuffix = currencyMeta.kind === 'tokens' ? ' Tokens' : ''
  const withSuffix = (value: string) =>
    tokenSuffix && value !== '-' ? `${value}${tokenSuffix}` : value

  const usedDisplay = withSuffix(formatQuotaValue(usedQuota))
  const remainingDisplay = withSuffix(formatBalance(balance))
  // Compact K/M/B labels keep dense cells readable; tooltips show exact values
  const usedCompact = withSuffix(formatQuotaCompact(usedQuota))
  const remainingCompact = withSuffix(formatBalanceCompact(balance))

  // Tag row: only show cumulative used quota
  if (isTagRow) {
    return (
      <TooltipProvider>
        <Tooltip>
          <TooltipTrigger
            render={
              <StatusBadge
                label={`Used: ${usedCompact}`}
                variant='neutral'
                size='sm'
                copyable={false}
                className='cursor-help'
              />
            }
          />
          <TooltipContent>
            <p>
              {t('channels.fields.labelWithColon', {
                label: t('common.status.used'),
              })}{' '}
              {usedDisplay}
            </p>
          </TooltipContent>
        </Tooltip>
      </TooltipProvider>
    )
  }

  // Regular channel row: show used and remaining with click to update
  const variant = getBalanceVariant(balance)

  const handleClickUpdate = async () => {
    if (isUpdating) return

    setIsUpdating(true)
    await handleUpdateChannelBalance(channel.id, queryClient)
    setIsUpdating(false)
  }

  return (
    <TooltipProvider>
      <div className='flex items-center gap-1'>
        <Tooltip>
          <TooltipTrigger
            render={
              <StatusBadge
                label={usedCompact}
                variant='neutral'
                size='sm'
                copyable={false}
                className='cursor-help'
              />
            }
          />
          <TooltipContent>
            <p>
              {t('channels.fields.labelWithColon', {
                label: t('common.status.used'),
              })}{' '}
              {usedDisplay}
            </p>
          </TooltipContent>
        </Tooltip>
        <Tooltip>
          <TooltipTrigger
            render={
              <StatusBadge
                label={
                  isUpdating ? t('channels.status.updating') : remainingCompact
                }
                variant={isUpdating ? 'neutral' : variant}
                size='sm'
                copyable={false}
                className='cursor-pointer'
                onClick={handleClickUpdate}
              />
            }
          />
          <TooltipContent>
            <p>
              {t('channels.fields.labelWithColon', {
                label: t('channels.fields.remaining'),
              })}{' '}
              {remainingDisplay}
            </p>
            <p>{t('channels.fields.clickToUpdateBalance')}</p>
          </TooltipContent>
        </Tooltip>
      </div>
    </TooltipProvider>
  )
}

/**
 * Generate channels columns configuration
 */
export function useChannelsColumns(): ColumnDef<Channel>[] {
  const { t } = useTranslation()
  return [
    // Checkbox column
    {
      id: 'select',
      header: ({ table }) => (
        <Checkbox
          checked={table.getIsAllPageRowsSelected()}
          indeterminate={
            table.getIsSomePageRowsSelected() &&
            !table.getIsAllPageRowsSelected()
          }
          onCheckedChange={(value) => table.toggleAllPageRowsSelected(!!value)}
          aria-label={t('channels.placeholders.selectAll')}
        />
      ),
      cell: ({ row }) => {
        const isTagRow = isTagAggregateRow(row.original)

        // Don't show checkbox for tag rows
        if (isTagRow) {
          return null
        }

        return (
          <Checkbox
            checked={row.getIsSelected()}
            onCheckedChange={(value) => row.toggleSelected(!!value)}
            aria-label={t('channels.placeholders.selectRow')}
          />
        )
      },
      enableSorting: false,
      enableHiding: false,
      size: 40,
    },

    // ID column
    {
      accessorKey: 'id',
      meta: { label: t('channels.fields.id'), mobileHidden: true },
      header: ({ column }) => (
        <DataTableColumnHeader
          column={column}
          title={t('channels.fields.id')}
        />
      ),
      cell: ({ row }) => {
        const id = row.getValue('id') as number
        return <TableId value={id} />
      },
      size: 80,
    },

    // Name column
    {
      accessorKey: 'name',
      meta: { label: t('channels.fields.name'), mobileTitle: true },
      header: ({ column }) => (
        <DataTableColumnHeader
          column={column}
          title={t('channels.fields.name')}
        />
      ),
      cell: ({ row }) => {
        const isTagRow = isTagAggregateRow(row.original)
        const name = row.getValue('name') as string
        const channel = row.original
        const isMultiKey = isMultiKeyChannel(channel)

        // Tag row with expand/collapse
        if (isTagRow) {
          const tag = (row.original as TagRow).tag || name
          const childrenCount = (row.original as TagRow).children?.length || 0

          return (
            <div className='flex items-center gap-2'>
              <Button
                variant='ghost'
                size='sm'
                className='h-6 w-6 p-0'
                onClick={row.getToggleExpandedHandler()}
              >
                {row.getIsExpanded() ? (
                  <ChevronDown className='h-4 w-4' />
                ) : (
                  <ChevronRight className='h-4 w-4' />
                )}
              </Button>
              <div className='flex items-center gap-1.5'>
                <span className='font-semibold'>Tag：{tag}</span>
                <StatusBadge
                  label={`${childrenCount} channels`}
                  variant='blue'
                  size='sm'
                  copyable={false}
                />
              </div>
            </div>
          )
        }

        // Regular channel row
        const settings = parseChannelSettings(channel.setting)
        const isPassThrough = settings.pass_through_body_enabled === true

        return (
          <div className='flex items-center gap-2'>
            <div className='flex flex-col gap-1'>
              <div className='flex items-center gap-1.5'>
                <TruncatedText
                  text={name}
                  className='font-medium'
                  maxWidth='max-w-[180px]'
                />
                {isPassThrough && (
                  <TooltipProvider delay={100}>
                    <Tooltip>
                      <TooltipTrigger
                        render={
                          <AlertTriangle className='h-3.5 w-3.5 shrink-0 text-amber-500' />
                        }
                      ></TooltipTrigger>
                      <TooltipContent side='top'>
                        {t(
                          'channels.status.requestBodyPassThroughIsEnabledTheRequestBody'
                        )}
                      </TooltipContent>
                    </Tooltip>
                  </TooltipProvider>
                )}
                {isMultiKey && (
                  <StatusBadge
                    label={`${channel.channel_info.multi_key_size} keys`}
                    variant='purple'
                    size='sm'
                    copyable={false}
                  />
                )}
              </div>
              {channel.remark && (
                <TooltipProvider delay={200}>
                  <Tooltip>
                    <TooltipTrigger
                      render={
                        <span className='text-muted-foreground text-xs' />
                      }
                    >
                      {truncateText(channel.remark, 40)}
                    </TooltipTrigger>
                    <TooltipContent side='bottom' className='max-w-xs'>
                      {channel.remark}
                    </TooltipContent>
                  </Tooltip>
                </TooltipProvider>
              )}
            </div>
          </div>
        )
      },
      minSize: 200,
    },

    // Type column
    {
      accessorKey: 'type',
      meta: { label: t('channels.fields.type') },
      header: t('channels.fields.type'),
      cell: ({ row }) => {
        const isTagRow = isTagAggregateRow(row.original)

        if (isTagRow) {
          return (
            <StatusBadge
              label={t('channels.fields.tagAggregate')}
              variant='blue'
              size='sm'
              copyable={false}
            />
          )
        }

        const type = row.getValue('type') as number
        const typeNameKey = getChannelTypeLabel(type)
        const typeName = t(typeNameKey)
        const iconName = getChannelTypeIcon(type)
        const icon = getLobeIcon(`${iconName}.Color`, 20)
        const channel = row.original as Channel
        const isMultiKey = isMultiKeyChannel(channel)
        const multiKeyMode = channel.channel_info?.multi_key_mode ?? 'random'
        const MultiKeyModeIcon =
          multiKeyMode === 'random' ? Shuffle : ListOrdered
        const multiKeyTooltip =
          multiKeyMode === 'random'
            ? t('channels.fields.multiKeyRandomRotation')
            : t('channels.fields.multiKeyPollingRotation')

        const ionetMeta = parseIonetMeta(channel.other_info)
        const isIonet = ionetMeta?.source === 'ionet'
        const deploymentId =
          typeof ionetMeta?.deployment_id === 'string'
            ? ionetMeta?.deployment_id
            : undefined

        return (
          <div className='flex items-center gap-2'>
            <div className='flex items-center gap-1.5'>
              {isMultiKey && (
                <TooltipProvider delay={100}>
                  <Tooltip>
                    <TooltipTrigger
                      render={
                        <span className='border-border bg-muted text-primary inline-flex h-6 w-6 items-center justify-center rounded-md border' />
                      }
                    >
                      <MultiKeyModeIcon className='h-3.5 w-3.5' />
                    </TooltipTrigger>
                    <TooltipContent side='top'>
                      {multiKeyTooltip}
                    </TooltipContent>
                  </Tooltip>
                </TooltipProvider>
              )}
              {icon}
            </div>
            <StatusBadge
              label={typeName}
              autoColor={typeName}
              size='sm'
              copyable={false}
            />
            {isIonet && (
              <TooltipProvider delay={100}>
                <Tooltip>
                  <TooltipTrigger
                    render={
                      <span
                        className='flex cursor-pointer items-center gap-1.5 text-xs font-medium'
                        onClick={(e) => {
                          e.stopPropagation()
                          if (!deploymentId) return
                          const targetUrl = `/models/deployments?dFilter=${encodeURIComponent(String(deploymentId))}`
                          window.open(targetUrl, '_blank', 'noopener')
                        }}
                      />
                    }
                  >
                    <StatusBadge
                      label='IO.NET'
                      variant='purple'
                      size='sm'
                      copyable={false}
                      className='cursor-pointer'
                    />
                  </TooltipTrigger>
                  <TooltipContent side='top'>
                    <div className='max-w-xs space-y-1'>
                      <div className='text-xs'>
                        {t('channels.tips.ioNetDeployment')}
                      </div>
                      {deploymentId && (
                        <div className='text-muted-foreground font-mono text-xs'>
                          {t('channels.fields.deploymentId')}: {deploymentId}
                        </div>
                      )}
                      <div className='text-muted-foreground text-xs'>
                        {t('channels.fields.clickToOpenDeployment')}
                      </div>
                    </div>
                  </TooltipContent>
                </Tooltip>
              </TooltipProvider>
            )}
          </div>
        )
      },
      filterFn: (row, id, value) => {
        if (!value || value.length === 0 || value.includes('all')) return true
        return value.includes(String(row.getValue(id)))
      },
      size: 140,
      enableSorting: false,
    },

    // Status column
    {
      accessorKey: 'status',
      meta: { label: t('channels.fields.status'), mobileBadge: true },
      header: t('channels.fields.status'),
      cell: ({ row }) => {
        const isTagRow = isTagAggregateRow(row.original)
        const status = row.getValue('status') as number
        const channel = row.original as Channel

        // Tag row: show aggregated status
        if (isTagRow) {
          const childrenCount = (row.original as TagRow).children?.length || 0
          const hasEnabled = status === 1

          if (hasEnabled) {
            return (
              <StatusBadge
                label={`Active (${childrenCount})`}
                variant='success'
                size='sm'
                copyable={false}
              />
            )
          } else {
            return (
              <StatusBadge
                label={`Inactive (${childrenCount})`}
                variant='neutral'
                size='sm'
                copyable={false}
              />
            )
          }
        }

        // Regular channel row
        const config =
          CHANNEL_STATUS_CONFIG[status as keyof typeof CHANNEL_STATUS_CONFIG] ||
          CHANNEL_STATUS_CONFIG[0]

        const isMultiKey = isMultiKeyChannel(channel)
        const keySize = channel.channel_info?.multi_key_size ?? 0
        const disabledCount = channel.channel_info?.multi_key_status_list
          ? Object.keys(channel.channel_info.multi_key_status_list).length
          : 0
        const enabledCount = Math.max(0, keySize - disabledCount)
        const label =
          isMultiKey && keySize > 0
            ? `${t(config.label)} (${enabledCount}/${keySize})`
            : t(config.label)

        // Auto-disabled: show reason and time tooltip
        if (status === 3) {
          let statusReason = ''
          let statusTime = ''
          try {
            const otherInfo = channel.other_info
              ? JSON.parse(channel.other_info)
              : null
            if (otherInfo) {
              statusReason = otherInfo.status_reason || ''
              statusTime = otherInfo.status_time
                ? formatTimestampToDate(otherInfo.status_time)
                : ''
            }
          } catch {
            /* empty */
          }

          if (statusReason || statusTime) {
            return (
              <TooltipProvider delay={100}>
                <Tooltip>
                  <TooltipTrigger render={<span />}>
                    <StatusBadge
                      label={label}
                      variant={config.variant}
                      size='sm'
                      copyable={false}
                    />
                  </TooltipTrigger>
                  <TooltipContent side='top' className='max-w-xs'>
                    <div className='space-y-1 text-xs'>
                      {statusReason && (
                        <div>
                          {t('channels.fields.labelWithColon', {
                            label: t('channels.fields.reason'),
                          })}{' '}
                          {statusReason}
                        </div>
                      )}
                      {statusTime && (
                        <div>
                          {t('channels.fields.labelWithColon', {
                            label: t('auditLogs.fields.time'),
                          })}{' '}
                          {statusTime}
                        </div>
                      )}
                    </div>
                  </TooltipContent>
                </Tooltip>
              </TooltipProvider>
            )
          }
        }

        return (
          <StatusBadge
            label={label}
            variant={config.variant}
            size='sm'
            copyable={false}
          />
        )
      },
      filterFn: (row, id, value) => {
        if (!value || value.length === 0 || value.includes('all')) return true
        const status = row.getValue(id) as number
        if (value.includes('enabled')) return status === 1
        if (value.includes('disabled')) return status !== 1
        return false
      },
      size: 120,
      enableSorting: false,
    },

    // Models column
    {
      accessorKey: 'models',
      meta: { label: t('channels.titles.models'), mobileHidden: true },
      header: t('channels.titles.models'),
      cell: ({ row }) => {
        const models = row.getValue('models') as string
        const modelArray = parseModelsList(models)

        if (modelArray.length === 0) {
          return <span className='text-muted-foreground text-xs'>-</span>
        }

        const modelBadges = modelArray.map((model, idx) => (
          <StatusBadge
            key={idx}
            label={model}
            autoColor={model}
            size='sm'
            className='font-mono'
          />
        ))

        return (
          <TooltipProvider>
            <Tooltip>
              <TooltipTrigger render={<div />}>
                {renderLimitedItems(modelBadges, 2)}
              </TooltipTrigger>
              {modelArray.length > 2 && (
                <TooltipContent
                  side='top'
                  className='border-border bg-popover max-h-48 max-w-[320px] overflow-y-auto p-2'
                >
                  <div className='flex flex-wrap gap-1'>{modelBadges}</div>
                </TooltipContent>
              )}
            </Tooltip>
          </TooltipProvider>
        )
      },
      size: 200,
      enableSorting: false,
    },

    // Group column
    {
      accessorKey: 'group',
      meta: { label: t('channels.fields.groups'), mobileHidden: true },
      header: t('channels.fields.groups'),
      cell: ({ row }) => {
        const group = row.getValue('group') as string
        const groupArray = parseGroupsList(group)

        const groupBadges = groupArray.map((g) => (
          <GroupBadge key={g} group={g} size='sm' />
        ))

        return (
          <TooltipProvider>
            <Tooltip>
              <TooltipTrigger render={<div />}>
                {renderLimitedItems(groupBadges, 2)}
              </TooltipTrigger>
              {groupArray.length > 2 && (
                <TooltipContent
                  side='top'
                  className='border-border bg-popover max-h-48 max-w-[320px] overflow-y-auto p-2'
                >
                  <div className='flex flex-wrap gap-1'>{groupBadges}</div>
                </TooltipContent>
              )}
            </Tooltip>
          </TooltipProvider>
        )
      },
      filterFn: (row, id, value) => {
        if (!value || value.length === 0 || value.includes('all')) return true
        const group = row.getValue(id) as string
        const groupArray = parseGroupsList(group)
        return groupArray.some((g) => value.includes(g))
      },
      size: 150,
      enableSorting: false,
    },

    // Tag column
    {
      accessorKey: 'tag',
      meta: { label: t('channels.fields.tag'), mobileHidden: true },
      header: t('channels.fields.tag'),
      cell: ({ row }) => {
        const tag = row.getValue('tag') as string | null
        if (!tag)
          return <span className='text-muted-foreground text-xs'>-</span>

        return <StatusBadge label={tag} autoColor={tag} size='sm' />
      },
      size: 120,
      enableSorting: false,
    },

    // Priority column
    {
      accessorKey: 'priority',
      meta: { label: t('channels.fields.priority'), mobileHidden: true },
      header: ({ column }) => (
        <DataTableColumnHeader
          column={column}
          title={t('channels.fields.priority')}
        />
      ),
      cell: ({ row }) => <PriorityCell channel={row.original} />,
      size: 100,
    },

    // Weight column
    {
      accessorKey: 'weight',
      meta: { label: t('channels.fields.weight'), mobileHidden: true },
      header: t('channels.fields.weight'),
      cell: ({ row }) => <WeightCell channel={row.original} />,
      size: 90,
      enableSorting: false,
    },

    // Balance column (Used/Remaining)
    {
      accessorKey: 'balance',
      meta: { label: t('channels.status.usedRemaining') },
      header: ({ column }) => (
        <DataTableColumnHeader
          column={column}
          title={t('channels.status.usedRemaining')}
        />
      ),
      cell: ({ row }) => <BalanceCell channel={row.original} />,
      size: 180,
    },

    // Response Time column
    {
      accessorKey: 'response_time',
      meta: { label: t('channels.fields.response'), mobileHidden: true },
      header: ({ column }) => (
        <DataTableColumnHeader
          column={column}
          title={t('channels.fields.response')}
        />
      ),
      cell: ({ row }) => {
        const responseTime = row.getValue('response_time') as number
        const config = getResponseTimeConfig(responseTime)

        return (
          <StatusBadge
            label={formatResponseTime(responseTime, t)}
            variant={config.variant}
            size='sm'
            copyable={false}
          />
        )
      },
      size: 110,
    },

    // Test Time column
    {
      accessorKey: 'test_time',
      meta: { label: t('channels.fields.lastTested'), mobileHidden: true },
      header: ({ column }) => (
        <DataTableColumnHeader
          column={column}
          title={t('channels.fields.lastTested')}
        />
      ),
      cell: ({ row }) => {
        const testTime = row.getValue('test_time') as number

        // For invalid timestamps, show "Never" badge
        if (!testTime || testTime === 0) {
          return <span className='text-muted-foreground text-xs'>-</span>
        }

        const timeText = formatRelativeTime(testTime)
        const fullDate = formatTimestampToDate(testTime)

        // For valid timestamps, show tooltip with full date
        return (
          <TooltipProvider>
            <Tooltip>
              <TooltipTrigger
                render={
                  <span className='text-muted-foreground cursor-pointer font-mono text-sm' />
                }
              >
                {timeText}
              </TooltipTrigger>
              <TooltipContent side='top'>
                <p className='font-mono text-sm'>{fullDate}</p>
              </TooltipContent>
            </Tooltip>
          </TooltipProvider>
        )
      },
      size: 120,
      enableSorting: false,
    },

    // Actions column
    {
      id: 'actions',
      cell: ({ row }) => {
        // Check if this is a tag row (has children)
        const isTagRow = isTagAggregateRow(row.original)

        if (isTagRow) {
          return (
            <DataTableTagRowActions
              // eslint-disable-next-line @typescript-eslint/no-explicit-any
              row={row as any}
            />
          )
        }

        return <DataTableRowActions row={row} />
      },
      size: 132,
      enableSorting: false,
      enableHiding: false,
    },
  ]
}
