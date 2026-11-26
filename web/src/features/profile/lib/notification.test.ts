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
import assert from 'node:assert/strict'
import { describe, test } from 'node:test'
import { DEFAULT_QUOTA_WARNING_THRESHOLD } from '../constants'
import type { UserProfile } from '../types'
import {
  isHttpUrl,
  isSameNotifyPayload,
  toNotifyPayload,
  toNotificationFormValues,
} from './notification'

function profileWithSettings(setting?: string): UserProfile {
  return {
    id: 1,
    username: 'alice',
    display_name: 'Alice',
    role: 1,
    group: 'default',
    quota: 0,
    used_quota: 0,
    request_count: 0,
    status: 1,
    aff_count: 0,
    aff_quota: 0,
    aff_history_quota: 0,
    created_time: 0,
    setting,
  }
}

describe('toNotificationFormValues', () => {
  test('falls back to defaults when there is no profile', () => {
    assert.deepEqual(toNotificationFormValues(null), {
      notify_type: 'email',
      quota_warning_threshold: DEFAULT_QUOTA_WARNING_THRESHOLD,
      notification_email: '',
      webhook_url: '',
      webhook_secret: '',
      bark_url: '',
      gotify_url: '',
      gotify_token: '',
      gotify_priority: 5,
    })
  })

  test('reads the saved notification settings', () => {
    const values = toNotificationFormValues(
      profileWithSettings(
        JSON.stringify({
          notify_type: 'gotify',
          quota_warning_threshold: 100,
          gotify_url: 'https://gotify.example.com',
          gotify_token: 't0k',
          gotify_priority: 8,
        })
      )
    )

    assert.equal(values.notify_type, 'gotify')
    assert.equal(values.quota_warning_threshold, 100)
    assert.equal(values.gotify_url, 'https://gotify.example.com')
    assert.equal(values.gotify_token, 't0k')
    assert.equal(values.gotify_priority, 8)
  })

  test('ignores a malformed settings string and an unknown notify type', () => {
    const malformed = toNotificationFormValues(profileWithSettings('not json'))
    assert.equal(malformed.notify_type, 'email')
    assert.equal(
      malformed.quota_warning_threshold,
      DEFAULT_QUOTA_WARNING_THRESHOLD
    )

    const unknownType = toNotificationFormValues(
      profileWithSettings(JSON.stringify({ notify_type: 'sms' }))
    )
    assert.equal(unknownType.notify_type, 'email')
  })
})

describe('toNotifyPayload / isSameNotifyPayload', () => {
  const values = toNotificationFormValues(null)

  test('submits exactly the nine notification fields', () => {
    const payload = toNotifyPayload(values)

    assert.deepEqual(Object.keys(payload).sort(), [
      'bark_url',
      'gotify_priority',
      'gotify_token',
      'gotify_url',
      'notification_email',
      'notify_type',
      'quota_warning_threshold',
      'webhook_secret',
      'webhook_url',
    ])
    assert.equal(payload.notify_type, 'email')
  })

  test('detects a changed notification field', () => {
    const changed = { ...values, quota_warning_threshold: 1 }

    assert.equal(
      isSameNotifyPayload(toNotifyPayload(values), toNotifyPayload(values)),
      true
    )
    assert.equal(
      isSameNotifyPayload(toNotifyPayload(changed), toNotifyPayload(values)),
      false
    )
  })
})

describe('isHttpUrl', () => {
  test('accepts only full http and https URLs', () => {
    assert.equal(isHttpUrl('https://example.com/hook'), true)
    assert.equal(isHttpUrl('http://127.0.0.1:8080/hook'), true)
    assert.equal(isHttpUrl('example.com/hook'), false)
    assert.equal(isHttpUrl('ftp://example.com'), false)
    assert.equal(isHttpUrl(''), false)
  })
})
