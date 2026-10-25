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
import { type QueryClient } from '@tanstack/react-query'
import i18next from 'i18next'
import { toast } from 'sonner'
import { updateModelStatus, deleteModel as deleteModelAPI } from '../api'
import { modelsQueryKeys } from './query-keys'

// ============================================================================
// Model Status Actions
// ============================================================================

/**
 * Enable a model
 */
async function handleEnableModel(
  id: number,
  queryClient?: QueryClient,
  onSuccess?: () => void
): Promise<void> {
  try {
    const response = await updateModelStatus(id, 1)
    if (response.success) {
      toast.success(i18next.t('models.status.modelEnabledSuccessfully'))
      queryClient?.invalidateQueries({ queryKey: modelsQueryKeys.lists() })
      onSuccess?.()
    } else {
      toast.error(
        response.message || i18next.t('models.errors.failedToEnableModel')
      )
    }
  } catch (error: unknown) {
    toast.error(
      (error as Error)?.message ||
        i18next.t('models.errors.failedToEnableModel')
    )
  }
}

/**
 * Disable a model
 */
async function handleDisableModel(
  id: number,
  queryClient?: QueryClient,
  onSuccess?: () => void
): Promise<void> {
  try {
    const response = await updateModelStatus(id, 0)
    if (response.success) {
      toast.success(i18next.t('models.status.modelDisabledSuccessfully'))
      queryClient?.invalidateQueries({ queryKey: modelsQueryKeys.lists() })
      onSuccess?.()
    } else {
      toast.error(
        response.message || i18next.t('models.errors.failedToDisableModel')
      )
    }
  } catch (error: unknown) {
    toast.error(
      (error as Error)?.message ||
        i18next.t('models.errors.failedToDisableModel')
    )
  }
}

/**
 * Toggle model status
 */
export async function handleToggleModelStatus(
  id: number,
  currentStatus: number,
  queryClient?: QueryClient,
  onSuccess?: () => void
): Promise<void> {
  if (currentStatus === 1) {
    await handleDisableModel(id, queryClient, onSuccess)
  } else {
    await handleEnableModel(id, queryClient, onSuccess)
  }
}

// ============================================================================
// Model Delete Actions
// ============================================================================

/**
 * Delete a single model
 */
export async function handleDeleteModel(
  id: number,
  queryClient?: QueryClient,
  onSuccess?: () => void
): Promise<void> {
  try {
    const response = await deleteModelAPI(id)
    if (response.success) {
      toast.success(i18next.t('models.status.modelDeletedSuccessfully'))
      queryClient?.invalidateQueries({ queryKey: modelsQueryKeys.lists() })
      onSuccess?.()
    } else {
      toast.error(
        response.message || i18next.t('channels.errors.failedToDeleteModel')
      )
    }
  } catch (error: unknown) {
    toast.error(
      (error as Error)?.message ||
        i18next.t('channels.errors.failedToDeleteModel')
    )
  }
}

/**
 * Batch delete models
 */
export async function handleBatchDeleteModels(
  ids: number[],
  queryClient?: QueryClient,
  onSuccess?: (deletedCount: number) => void
): Promise<void> {
  if (ids.length === 0) {
    toast.error(i18next.t('models.errors.pleaseSelectAtLeastOneModel'))
    return
  }

  try {
    const deletePromises = ids.map((id) => deleteModelAPI(id))
    const results = await Promise.all(deletePromises)

    let successCount = 0
    let failedCount = 0

    results.forEach((res, index) => {
      if (res.success) {
        successCount++
      } else {
        failedCount++
        // eslint-disable-next-line no-console
        console.error(`Failed to delete model ${ids[index]}:`, res.message)
      }
    })

    if (successCount > 0) {
      toast.success(
        i18next.t('models.status.successfullyDeletedCountModelS', {
          count: successCount,
        })
      )
      queryClient?.invalidateQueries({ queryKey: modelsQueryKeys.lists() })
      onSuccess?.(successCount)
    }

    if (failedCount > 0) {
      toast.error(
        i18next.t('models.errors.failedToDeleteCountModelS', {
          count: failedCount,
        })
      )
    }
  } catch (error: unknown) {
    toast.error(
      (error as Error)?.message || i18next.t('models.status.batchDeleteFailed')
    )
  }
}

// ============================================================================
// Batch Status Actions
// ============================================================================

/**
 * Batch enable models
 */
export async function handleBatchEnableModels(
  ids: number[],
  queryClient?: QueryClient,
  onSuccess?: () => void
): Promise<void> {
  if (ids.length === 0) {
    toast.error(i18next.t('models.errors.pleaseSelectAtLeastOneModel'))
    return
  }

  try {
    const enablePromises = ids.map((id) => updateModelStatus(id, 1))
    const results = await Promise.all(enablePromises)

    let successCount = 0
    let failedCount = 0

    results.forEach((res) => {
      if (res.success) {
        successCount++
      } else {
        failedCount++
      }
    })

    if (successCount > 0) {
      toast.success(
        i18next.t('models.status.successfullyEnabledCountModelS', {
          count: successCount,
        })
      )
      queryClient?.invalidateQueries({ queryKey: modelsQueryKeys.lists() })
      onSuccess?.()
    }

    if (failedCount > 0) {
      toast.error(
        i18next.t('models.errors.failedToEnableCountModelS', {
          count: failedCount,
        })
      )
    }
  } catch (error: unknown) {
    toast.error(
      (error as Error)?.message || i18next.t('models.status.batchEnableFailed')
    )
  }
}

/**
 * Batch disable models
 */
export async function handleBatchDisableModels(
  ids: number[],
  queryClient?: QueryClient,
  onSuccess?: () => void
): Promise<void> {
  if (ids.length === 0) {
    toast.error(i18next.t('models.errors.pleaseSelectAtLeastOneModel'))
    return
  }

  try {
    const disablePromises = ids.map((id) => updateModelStatus(id, 0))
    const results = await Promise.all(disablePromises)

    let successCount = 0
    let failedCount = 0

    results.forEach((res) => {
      if (res.success) {
        successCount++
      } else {
        failedCount++
      }
    })

    if (successCount > 0) {
      toast.success(
        i18next.t('models.status.successfullyDisabledCountModelS', {
          count: successCount,
        })
      )
      queryClient?.invalidateQueries({ queryKey: modelsQueryKeys.lists() })
      onSuccess?.()
    }

    if (failedCount > 0) {
      toast.error(
        i18next.t('models.errors.failedToDisableCountModelS', {
          count: failedCount,
        })
      )
    }
  } catch (error: unknown) {
    toast.error(
      (error as Error)?.message || i18next.t('models.status.batchDisableFailed')
    )
  }
}
