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
import {
  DEFAULT_QUOTA_WARNING_THRESHOLD,
  NOTIFICATION_METHODS,
} from '../constants'
import type {
  NotifyType,
  UpdateUserSettingsRequest,
  UserProfile,
} from '../types'
import { parseUserSettings } from './format'

/** 通知设置表单的取值。四种通知方式的字段都在表单里，切换方式时已填的值保留。 */
export interface NotificationFormValues {
  notify_type: NotifyType
  quota_warning_threshold: number
  notification_email: string
  webhook_url: string
  webhook_secret: string
  bark_url: string
  gotify_url: string
  gotify_token: string
  gotify_priority: number
}

const NOTIFY_TYPES: ReadonlyArray<unknown> = NOTIFICATION_METHODS.map(
  (method) => method.value
)

function isNotifyType(value: unknown): value is NotifyType {
  return NOTIFY_TYPES.includes(value)
}

/** 由用户资料里的设置得出表单取值；没有保存过的字段取默认值。 */
export function toNotificationFormValues(
  profile: UserProfile | null
): NotificationFormValues {
  const settings = profile ? parseUserSettings(profile.setting) : {}

  return {
    notify_type: isNotifyType(settings.notify_type)
      ? settings.notify_type
      : 'email',
    quota_warning_threshold:
      settings.quota_warning_threshold ?? DEFAULT_QUOTA_WARNING_THRESHOLD,
    notification_email: settings.notification_email ?? '',
    webhook_url: settings.webhook_url ?? '',
    webhook_secret: settings.webhook_secret ?? '',
    bark_url: settings.bark_url ?? '',
    gotify_url: settings.gotify_url ?? '',
    gotify_token: settings.gotify_token ?? '',
    gotify_priority: settings.gotify_priority ?? 5,
  }
}

const NOTIFY_PAYLOAD_KEYS: ReadonlyArray<keyof UpdateUserSettingsRequest> = [
  'notify_type',
  'quota_warning_threshold',
  'notification_email',
  'webhook_url',
  'webhook_secret',
  'bark_url',
  'gotify_url',
  'gotify_token',
  'gotify_priority',
]

/** 通知设置接口需要的字段。 */
export function toNotifyPayload(
  values: NotificationFormValues
): UpdateUserSettingsRequest {
  return {
    notify_type: values.notify_type,
    quota_warning_threshold: values.quota_warning_threshold,
    notification_email: values.notification_email,
    webhook_url: values.webhook_url,
    webhook_secret: values.webhook_secret,
    bark_url: values.bark_url,
    gotify_url: values.gotify_url,
    gotify_token: values.gotify_token,
    gotify_priority: values.gotify_priority,
  }
}

export function isSameNotifyPayload(
  a: UpdateUserSettingsRequest,
  b: UpdateUserSettingsRequest
): boolean {
  return NOTIFY_PAYLOAD_KEYS.every((key) => a[key] === b[key])
}

/** 是否为完整的 http / https 地址。 */
export function isHttpUrl(value: string): boolean {
  try {
    const url = new URL(value.trim())
    return url.protocol === 'http:' || url.protocol === 'https:'
  } catch {
    return false
  }
}
