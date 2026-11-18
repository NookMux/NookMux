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
import { type ManageUserAction } from '../types'

// ============================================================================
// User Action Messages
// ============================================================================

const ACTION_MESSAGES: Record<ManageUserAction, string> = {
  enable: 'users.status.userEnabledSuccessfully',
  disable: 'users.status.userDisabledSuccessfully',
  promote: 'users.status.userPromotedSuccessfully',
  demote: 'users.status.userDemotedSuccessfully',
  delete: 'users.status.userDeletedSuccessfully',
  add_quota: 'users.fields.quotaAdjustedSuccessfully',
}

const ACTION_ERROR_MESSAGES: Record<ManageUserAction, string> = {
  enable: 'users.errors.failedToEnableUser',
  disable: 'users.errors.failedToDisableUser',
  promote: 'users.errors.failedToPromoteUser',
  demote: 'users.errors.failedToDemoteUser',
  delete: 'users.errors.failedToDeleteUser',
  add_quota: 'users.errors.failedToAdjustQuota',
}

/**
 * Get success message for user management action
 */
export function getUserActionMessage(action: ManageUserAction): string {
  return ACTION_MESSAGES[action]
}

/**
 * Get error message for user management action
 */
export function getUserActionErrorMessage(action: ManageUserAction): string {
  return ACTION_ERROR_MESSAGES[action]
}
