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
import i18next from 'i18next'
import { api, get2FAStatus } from '@/lib/api'
import {
  buildAssertionResult,
  prepareCredentialRequestOptions,
  isPasskeySupported as detectPasskeySupport,
} from '@/lib/passkey'
import {
  beginPasskeyVerification,
  finishPasskeyVerification,
  getPasskeyStatus,
} from '../passkey'
import type { VerificationMethod, VerificationMethods } from './types'

/**
 * Fetch available verification methods for the current user.
 */
export async function checkVerificationMethods(): Promise<VerificationMethods> {
  try {
    const [twoFAResponse, passkeyResponse, passkeySupported] =
      await Promise.all([
        get2FAStatus(),
        getPasskeyStatus(),
        detectPasskeySupport(),
      ])

    const has2FA =
      Boolean(twoFAResponse?.success) && Boolean(twoFAResponse?.data?.enabled)
    const hasPasskey =
      Boolean(passkeyResponse?.success) &&
      Boolean(passkeyResponse?.data?.enabled)

    return {
      has2FA,
      hasPasskey,
      passkeySupported,
    }
  } catch (error) {
    // eslint-disable-next-line no-console
    console.error('[Secure Verification] Failed to check methods', error)
    return {
      has2FA: false,
      hasPasskey: false,
      passkeySupported: false,
    }
  }
}

/**
 * Execute a verification flow based on the method type.
 * scope 绑定本次验证的用途，服务端消费时逐字比对，防止跨操作重放。
 */
export async function verify(
  method: VerificationMethod,
  code?: string,
  scope?: string
): Promise<void> {
  switch (method) {
    case '2fa':
      return verifyTwoFA(code, scope)
    case 'passkey':
      return verifyPasskey(scope)
    case 'password':
      return verifyPassword(code, scope)
    default:
      throw new Error(
        i18next.t('auth.errors.unsupportedVerificationMethod', { method })
      )
  }
}

/**
 * Perform 2FA verification flow.
 */
async function verifyTwoFA(
  code?: string | null,
  scope?: string
): Promise<void> {
  const trimmed = code?.trim()
  if (!trimmed) {
    throw new Error(
      i18next.t('profile.errors.pleaseEnterYourVerificationCodeOrBackupCode')
    )
  }

  const res = await api.post('/api/verify', {
    method: '2fa',
    code: trimmed,
    scope,
  })

  if (!res.data?.success) {
    throw new Error(
      res.data?.message || i18next.t('auth.errors.verificationFailed')
    )
  }
}

/**
 * Perform original-password re-authentication (fallback for accounts
 * without any second factor).
 */
async function verifyPassword(
  password?: string | null,
  scope?: string
): Promise<void> {
  const trimmed = password?.trim()
  if (!trimmed) {
    throw new Error(i18next.t('profile.errors.pleaseEnterYourCurrentPassword'))
  }

  const res = await api.post('/api/verify', {
    method: 'password',
    password: trimmed,
    scope,
  })

  if (!res.data?.success) {
    throw new Error(
      res.data?.message || i18next.t('auth.errors.verificationFailed')
    )
  }
}

/**
 * Perform Passkey verification flow.
 */
async function verifyPasskey(scope?: string): Promise<void> {
  if (typeof navigator === 'undefined' || !navigator.credentials) {
    throw new Error(i18next.t('auth.tips.passkeyIsNotSupportedOnThisDevice'))
  }

  try {
    const beginResponse = await beginPasskeyVerification()
    if (!beginResponse.success) {
      throw new Error(
        beginResponse.message ||
          i18next.t('auth.errors.failedToStartVerification')
      )
    }

    const publicKey = prepareCredentialRequestOptions(
      beginResponse.data?.options ?? beginResponse.data
    )

    const credential = (await navigator.credentials.get({
      publicKey,
    })) as PublicKeyCredential | null

    if (!credential) {
      throw new Error(
        i18next.t('auth.errors.passkeyVerificationCancelledOrTimedOut')
      )
    }

    const assertion = buildAssertionResult(credential)
    if (!assertion) {
      throw new Error(i18next.t('auth.errors.unableToBuildPasskeyAssertion'))
    }

    const finishResponse = await finishPasskeyVerification(assertion)
    if (!finishResponse.success) {
      throw new Error(
        finishResponse.message ||
          i18next.t('auth.errors.passkeyVerificationFailed')
      )
    }

    const verifyResponse = await api.post('/api/verify', {
      method: 'passkey',
      scope,
    })

    if (!verifyResponse.data?.success) {
      throw new Error(
        verifyResponse.data?.message ||
          i18next.t('auth.errors.verificationFailed')
      )
    }
  } catch (error: unknown) {
    if (error instanceof DOMException && error.name === 'NotAllowedError') {
      throw new Error(
        i18next.t('auth.errors.passkeyVerificationCancelledOrTimedOut'),
        {
          cause: error,
        }
      )
    }
    if (error instanceof DOMException && error.name === 'InvalidStateError') {
      throw new Error(
        i18next.t('auth.errors.passkeyVerificationUnavailableInState'),
        { cause: error }
      )
    }
    throw error
  }
}
