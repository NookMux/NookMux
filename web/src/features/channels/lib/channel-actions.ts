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
import type { QueryClient } from '@tanstack/react-query'
import i18next from 'i18next'
import { toast } from 'sonner'
import {
  copyChannel,
  deleteChannel,
  testChannel,
  updateChannel,
  batchDeleteChannels,
  batchSetChannelTag,
  enableTagChannels,
  disableTagChannels,
  deleteDisabledChannels,
  fixChannelAbilities,
  testAllChannels,
  updateAllChannelsBalance,
  updateChannelBalance,
} from '../api'
import { CHANNEL_STATUS, ERROR_MESSAGES, SUCCESS_MESSAGES } from '../constants'
import type { CopyChannelParams } from '../types'
import { formatBalanceByCurrency } from './channel-utils'

// ============================================================================
// Query Keys
// ============================================================================

export const channelsQueryKeys = {
  all: ['channels'] as const,
  lists: () => [...channelsQueryKeys.all, 'list'] as const,
  list: (params: Record<string, unknown>) =>
    [...channelsQueryKeys.lists(), params] as const,
  details: () => [...channelsQueryKeys.all, 'detail'] as const,
  detail: (id: number) => [...channelsQueryKeys.details(), id] as const,
}

// ============================================================================
// Single Channel Actions
// ============================================================================

/**
 * Enable a channel
 */
export async function handleEnableChannel(
  id: number,
  queryClient?: QueryClient,
  onSuccess?: () => void
): Promise<void> {
  try {
    const response = await updateChannel(id, { status: CHANNEL_STATUS.ENABLED })
    if (response.success) {
      toast.success(i18next.t(SUCCESS_MESSAGES.ENABLED))
      queryClient?.invalidateQueries({ queryKey: channelsQueryKeys.lists() })
      onSuccess?.()
    }
  } catch (_error) {
    toast.error(i18next.t(ERROR_MESSAGES.UPDATE_FAILED))
  }
}

/**
 * Disable a channel
 */
export async function handleDisableChannel(
  id: number,
  queryClient?: QueryClient,
  onSuccess?: () => void
): Promise<void> {
  try {
    const response = await updateChannel(id, {
      status: CHANNEL_STATUS.MANUAL_DISABLED,
    })
    if (response.success) {
      toast.success(i18next.t(SUCCESS_MESSAGES.DISABLED))
      queryClient?.invalidateQueries({ queryKey: channelsQueryKeys.lists() })
      onSuccess?.()
    }
  } catch (_error) {
    toast.error(i18next.t(ERROR_MESSAGES.UPDATE_FAILED))
  }
}

/**
 * Toggle channel status (enable/disable)
 */
export async function handleToggleChannelStatus(
  id: number,
  currentStatus: number,
  queryClient?: QueryClient,
  onSuccess?: () => void
): Promise<void> {
  if (currentStatus === CHANNEL_STATUS.ENABLED) {
    await handleDisableChannel(id, queryClient, onSuccess)
  } else {
    await handleEnableChannel(id, queryClient, onSuccess)
  }
}

/**
 * Delete a channel
 */
export async function handleDeleteChannel(
  id: number,
  queryClient?: QueryClient,
  onSuccess?: () => void
): Promise<void> {
  try {
    const response = await deleteChannel(id)
    if (response.success) {
      toast.success(i18next.t(SUCCESS_MESSAGES.DELETED))
      queryClient?.invalidateQueries({ queryKey: channelsQueryKeys.lists() })
      onSuccess?.()
    }
  } catch (_error) {
    toast.error(i18next.t(ERROR_MESSAGES.DELETE_FAILED))
  }
}

/**
 * Update a specific channel field (e.g., priority, weight)
 */
export async function handleUpdateChannelField(
  id: number,
  fieldName: string,
  value: number,
  queryClient?: QueryClient,
  onSuccess?: () => void
): Promise<void> {
  try {
    const response = await updateChannel(id, { [fieldName]: value })
    if (response.success) {
      // Show success toast with field name
      const fieldLabel =
        fieldName.charAt(0).toUpperCase() + fieldName.slice(1).toLowerCase()
      toast.success(
        i18next.t('channels.status.fieldUpdatedToValue', {
          field: fieldLabel,
          value,
        })
      )
      queryClient?.invalidateQueries({ queryKey: channelsQueryKeys.lists() })
      onSuccess?.()
    } else {
      toast.error(response.message || i18next.t(ERROR_MESSAGES.UPDATE_FAILED))
    }
  } catch (_error) {
    toast.error(i18next.t(ERROR_MESSAGES.UPDATE_FAILED))
  }
}

/**
 * Test channel connectivity
 */
export async function handleTestChannel(
  id: number,
  options?: {
    testModel?: string
    endpointType?: string
    stream?: boolean
    tool?: boolean
    silent?: boolean
  },
  onTestComplete?: (
    success: boolean,
    responseTime?: number,
    error?: string,
    errorCode?: string
  ) => void
): Promise<void> {
  const hasOptions =
    options &&
    (options.testModel ||
      options.endpointType ||
      options.stream ||
      options.tool)
  const payload = hasOptions
    ? {
        ...(options.testModel ? { model: options.testModel } : {}),
        ...(options.endpointType
          ? { endpoint_type: options.endpointType }
          : {}),
        ...(options.stream ? { stream: true } : {}),
        ...(options.tool ? { tool: true } : {}),
      }
    : undefined

  try {
    const response = await testChannel(id, payload)
    const responseTimeMs =
      response.data?.response_time ??
      (typeof response.time === 'number'
        ? Math.round(response.time * 1000)
        : undefined)

    if (response.success) {
      if (!options?.silent) {
        toast.success(i18next.t(SUCCESS_MESSAGES.TESTED))
      }
      onTestComplete?.(true, responseTimeMs)
    } else {
      if (!options?.silent) {
        toast.error(response.message || i18next.t(ERROR_MESSAGES.TEST_FAILED))
      }
      onTestComplete?.(
        false,
        responseTimeMs,
        response.message,
        response.error_code
      )
    }
  } catch (_error: unknown) {
    const err = _error as {
      response?: {
        data?: { message?: string; error_code?: string; time?: number }
      }
    }
    const errorMsg =
      err?.response?.data?.message || i18next.t(ERROR_MESSAGES.TEST_FAILED)
    const responseTimeMs =
      typeof err?.response?.data?.time === 'number'
        ? Math.round(err.response.data.time * 1000)
        : undefined
    if (!options?.silent) {
      toast.error(errorMsg)
    }
    onTestComplete?.(
      false,
      responseTimeMs,
      errorMsg,
      err?.response?.data?.error_code
    )
  }
}

/**
 * Copy a channel
 */
export async function handleCopyChannel(
  id: number,
  params: CopyChannelParams,
  queryClient?: QueryClient,
  onSuccess?: (newId: number) => void
): Promise<void> {
  try {
    const response = await copyChannel(id, params)
    if (response.success) {
      toast.success(i18next.t(SUCCESS_MESSAGES.COPIED))
      queryClient?.invalidateQueries({ queryKey: channelsQueryKeys.lists() })
      onSuccess?.(response.data?.id ?? 0)
    } else {
      toast.error(
        response.message || i18next.t('channels.errors.failedToCopyChannel')
      )
    }
  } catch (_error) {
    toast.error(i18next.t('channels.errors.failedToCopyChannel'))
  }
}

/**
 * Update channel balance
 */
export async function handleUpdateChannelBalance(
  id: number,
  queryClient?: QueryClient,
  onSuccess?: (balance: number) => void
): Promise<void> {
  try {
    const response = await updateChannelBalance(id)
    if (response.success && response.balance !== undefined) {
      const balance = response.balance
      toast.success(
        i18next.t('channels.status.balanceUpdatedBalance', {
          balance: formatBalanceByCurrency(balance, response.currency),
        })
      )
      queryClient?.invalidateQueries({ queryKey: channelsQueryKeys.lists() })
      onSuccess?.(balance)
    } else {
      toast.error(
        response.message || i18next.t('channels.errors.failedToUpdateBalance')
      )
    }
  } catch (_error: unknown) {
    toast.error(
      _error instanceof Error
        ? _error.message
        : i18next.t('channels.errors.failedToUpdateBalance')
    )
  }
}

// ============================================================================
// Batch Actions
// ============================================================================

/**
 * Batch delete channels
 */
export async function handleBatchDelete(
  ids: number[],
  queryClient?: QueryClient,
  onSuccess?: (deletedCount: number) => void
): Promise<void> {
  if (ids.length === 0) {
    toast.error(i18next.t('channels.titles.noChannelsSelected'))
    return
  }

  try {
    const response = await batchDeleteChannels({ ids })
    if (response.success) {
      toast.success(
        i18next.t('channels.status.countChannelSDeleted', {
          count: response.data || ids.length,
        })
      )
      queryClient?.invalidateQueries({ queryKey: channelsQueryKeys.lists() })
      onSuccess?.(response.data || ids.length)
    }
  } catch (_error) {
    toast.error(i18next.t(ERROR_MESSAGES.DELETE_FAILED))
  }
}

/**
 * Batch enable channels
 */
export async function handleBatchEnable(
  ids: number[],
  queryClient?: QueryClient,
  onSuccess?: () => void
): Promise<void> {
  if (ids.length === 0) {
    toast.error(i18next.t('channels.titles.noChannelsSelected'))
    return
  }

  try {
    // Update each channel individually
    const promises = ids.map((id) =>
      updateChannel(id, { status: CHANNEL_STATUS.ENABLED })
    )
    const results = await Promise.allSettled(promises)

    const successCount = results.filter((r) => r.status === 'fulfilled').length
    const failCount = results.filter((r) => r.status === 'rejected').length

    if (successCount > 0) {
      toast.success(
        i18next.t('channels.status.countChannelSEnabled', {
          count: successCount,
        })
      )
      queryClient?.invalidateQueries({ queryKey: channelsQueryKeys.lists() })
      onSuccess?.()
    }

    if (failCount > 0) {
      toast.error(
        i18next.t('channels.status.countChannelSFailedToEnable', {
          count: failCount,
        })
      )
    }
  } catch (_error) {
    toast.error(i18next.t('channels.errors.failedToEnableChannels'))
  }
}

/**
 * Batch disable channels
 */
export async function handleBatchDisable(
  ids: number[],
  queryClient?: QueryClient,
  onSuccess?: () => void
): Promise<void> {
  if (ids.length === 0) {
    toast.error(i18next.t('channels.titles.noChannelsSelected'))
    return
  }

  try {
    // Update each channel individually
    const promises = ids.map((id) =>
      updateChannel(id, { status: CHANNEL_STATUS.MANUAL_DISABLED })
    )
    const results = await Promise.allSettled(promises)

    const successCount = results.filter((r) => r.status === 'fulfilled').length
    const failCount = results.filter((r) => r.status === 'rejected').length

    if (successCount > 0) {
      toast.success(
        i18next.t('channels.status.countChannelSDisabled', {
          count: successCount,
        })
      )
      queryClient?.invalidateQueries({ queryKey: channelsQueryKeys.lists() })
      onSuccess?.()
    }

    if (failCount > 0) {
      toast.error(
        i18next.t('channels.status.countChannelSFailedToDisable', {
          count: failCount,
        })
      )
    }
  } catch (_error) {
    toast.error(i18next.t('channels.errors.failedToDisableChannels'))
  }
}

/**
 * Batch set tag
 */
export async function handleBatchSetTag(
  ids: number[],
  tag: string | null,
  queryClient?: QueryClient,
  onSuccess?: () => void
): Promise<void> {
  if (ids.length === 0) {
    toast.error(i18next.t('channels.titles.noChannelsSelected'))
    return
  }

  try {
    const response = await batchSetChannelTag({ ids, tag })
    if (response.success) {
      toast.success(i18next.t(SUCCESS_MESSAGES.TAG_SET))
      queryClient?.invalidateQueries({ queryKey: channelsQueryKeys.lists() })
      onSuccess?.()
    }
  } catch (_error) {
    toast.error(i18next.t('channels.errors.failedToSetTag'))
  }
}

// ============================================================================
// Tag-Based Actions
// ============================================================================

/**
 * Enable all channels with a tag
 */
export async function handleEnableTagChannels(
  tag: string,
  queryClient?: QueryClient,
  onSuccess?: () => void
): Promise<void> {
  try {
    const response = await enableTagChannels(tag)
    if (response.success) {
      toast.success(
        i18next.t('channels.status.enabledAllChannelsWithTagTag', { tag })
      )
      queryClient?.invalidateQueries({ queryKey: channelsQueryKeys.lists() })
      onSuccess?.()
    }
  } catch (_error) {
    toast.error(i18next.t('channels.errors.failedToEnableTagChannels'))
  }
}

/**
 * Disable all channels with a tag
 */
export async function handleDisableTagChannels(
  tag: string,
  queryClient?: QueryClient,
  onSuccess?: () => void
): Promise<void> {
  try {
    const response = await disableTagChannels(tag)
    if (response.success) {
      toast.success(
        i18next.t('channels.status.disabledAllChannelsWithTagTag', { tag })
      )
      queryClient?.invalidateQueries({ queryKey: channelsQueryKeys.lists() })
      onSuccess?.()
    }
  } catch (_error) {
    toast.error(i18next.t('channels.errors.failedToDisableTagChannels'))
  }
}

// ============================================================================
// System Actions
// ============================================================================

/**
 * Delete all disabled channels
 */
export async function handleDeleteAllDisabled(
  queryClient?: QueryClient,
  onSuccess?: (deletedCount: number) => void
): Promise<void> {
  try {
    const response = await deleteDisabledChannels()
    if (response.success) {
      toast.success(
        i18next.t('channels.status.countDisabledChannelSDeleted', {
          count: response.data || 0,
        })
      )
      queryClient?.invalidateQueries({ queryKey: channelsQueryKeys.lists() })
      onSuccess?.(response.data || 0)
    }
  } catch (_error) {
    toast.error(i18next.t('channels.errors.failedToDeleteDisabledChannels'))
  }
}

/**
 * Fix channel abilities
 */
export async function handleFixAbilities(
  queryClient?: QueryClient,
  onSuccess?: (result: { success: number; fails: number }) => void
): Promise<void> {
  try {
    const response = await fixChannelAbilities()
    if (response.success && response.data) {
      toast.success(
        i18next.t('channels.status.fixedAbilitiesSuccessSucceededFailsFailed', {
          success: response.data.success,
          fails: response.data.fails,
        })
      )
      queryClient?.invalidateQueries({ queryKey: channelsQueryKeys.lists() })
      onSuccess?.(response.data)
    }
  } catch (_error) {
    toast.error(i18next.t('channels.errors.failedToFixAbilities'))
  }
}

/**
 * Test all enabled channels
 */
export async function handleTestAllChannels(
  queryClient?: QueryClient,
  onSuccess?: () => void
): Promise<void> {
  try {
    const response = await testAllChannels()
    if (response.success) {
      toast.success(
        i18next.t(
          'channels.status.testingAllEnabledChannelsStartedPleaseRefreshToSee'
        )
      )
      queryClient?.invalidateQueries({ queryKey: channelsQueryKeys.lists() })
      onSuccess?.()
    } else {
      toast.error(
        response.message ||
          i18next.t('channels.errors.failedToStartTestingAllChannels')
      )
    }
  } catch (_error) {
    toast.error(i18next.t('channels.errors.failedToTestAllChannels'))
  }
}

/**
 * Update balance for all enabled channels
 */
export async function handleUpdateAllBalances(
  queryClient?: QueryClient,
  onSuccess?: () => void
): Promise<void> {
  try {
    const response = await updateAllChannelsBalance()
    if (response.success) {
      toast.success(
        i18next.t('channels.tips.updatingAllChannelBalancesThisMayTakeAWhile')
      )
      queryClient?.invalidateQueries({ queryKey: channelsQueryKeys.lists() })
      onSuccess?.()
    } else {
      toast.error(
        response.message ||
          i18next.t('channels.errors.failedToUpdateAllBalances')
      )
    }
  } catch (_error) {
    toast.error(i18next.t('channels.errors.failedToUpdateAllBalances'))
  }
}
