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
import { Shield, User, Users } from 'lucide-react'
import type { User as UserType } from './types'

// ============================================================================
// User Utilities
// ============================================================================

export const isUserDeleted = (user: UserType): boolean => {
  return user.DeletedAt != null
}

// ============================================================================
// User Status Configuration
// ============================================================================

export const USER_STATUS = {
  ENABLED: 1,
  DISABLED: 2,
} as const

export const USER_STATUSES = {
  [USER_STATUS.ENABLED]: {
    labelKey: 'channels.status.enabled',
    variant: 'success' as const,
    value: USER_STATUS.ENABLED,
  },
  [USER_STATUS.DISABLED]: {
    labelKey: 'channels.status.disabled',
    variant: 'neutral' as const,
    value: USER_STATUS.DISABLED,
  },
  DELETED: {
    labelKey: 'Deleted',
    variant: 'danger' as const,
    value: -1,
  },
} as const

const USER_STATUS_DELETED = 3

export const getUserStatusOptions = (t: (key: string) => string) => [
  { label: t('channels.status.enabled'), value: String(USER_STATUS.ENABLED) },
  { label: t('channels.status.disabled'), value: String(USER_STATUS.DISABLED) },
  {
    label: t('subscriptions.actions.deleted'),
    value: String(USER_STATUS_DELETED),
  },
]

// ============================================================================
// User Role Configuration
// ============================================================================

export const USER_ROLE = {
  USER: 1,
  ADMIN: 10,
  ROOT: 100,
} as const

export const USER_ROLES = {
  [USER_ROLE.USER]: {
    labelKey: 'systemSettings.fields.user',
    value: USER_ROLE.USER,
    icon: User,
  },
  [USER_ROLE.ADMIN]: {
    labelKey: 'systemSettings.fields.admin',
    value: USER_ROLE.ADMIN,
    icon: Users,
  },
  [USER_ROLE.ROOT]: {
    labelKey: 'users.fields.root',
    value: USER_ROLE.ROOT,
    icon: Shield,
  },
} as const

export const getUserRoleOptions = (t: (key: string) => string) => [
  {
    label: t('systemSettings.fields.user'),
    value: String(USER_ROLE.USER),
    icon: User,
  },
  {
    label: t('systemSettings.fields.admin'),
    value: String(USER_ROLE.ADMIN),
    icon: Users,
  },
  {
    label: t('users.fields.root'),
    value: String(USER_ROLE.ROOT),
    icon: Shield,
  },
]

// ============================================================================
// Default Values
// ============================================================================

export const DEFAULT_GROUP = 'default' as const

// ============================================================================
// Third-party Binding Fields
// ============================================================================

export const BINDING_FIELDS = [
  { key: 'github_id', label: 'GitHub ID' },
  { key: 'discord_id', label: 'Discord ID' },
  { key: 'oidc_id', label: 'OIDC ID' },
  { key: 'wechat_id', label: 'WeChat ID' },
  { key: 'email', label: 'auth.fields.email' },
  { key: 'telegram_id', label: 'Telegram ID' },
] as const

// ============================================================================
// Error Messages (i18n keys: use t(ERROR_MESSAGES.xxx) when displaying)
// ============================================================================

export const ERROR_MESSAGES = {
  UNEXPECTED: 'common.fields.unexpectedErrorOccurred',
  NO_USER: 'common.errors.noUserSelected',
  LOAD_FAILED: 'common.errors.failedToLoadUsers',
  SEARCH_FAILED: 'common.errors.failedToSearchUsers',
  CREATE_FAILED: 'common.errors.failedToCreateUser',
  UPDATE_FAILED: 'common.errors.failedToUpdateUser',
  DELETE_FAILED: 'common.errors.failedToDeleteUser',
} as const

// ============================================================================
// Success Messages (i18n keys: use t(SUCCESS_MESSAGES.xxx) when displaying)
// ============================================================================

export const SUCCESS_MESSAGES = {
  USER_CREATED: 'common.status.userCreatedSuccessfully',
  USER_UPDATED: 'common.status.userUpdatedSuccessfully',
} as const
