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
import { useCallback, useEffect, useMemo, useState } from 'react'
import i18next from 'i18next'
import { toast } from 'sonner'
import {
  extractVerificationInfo,
  isVerificationRequiredError,
} from '@/lib/secure-verification'
import { checkVerificationMethods, verify } from '../api'
import type {
  SecureVerificationState,
  StartVerificationOptions,
  UseSecureVerificationOptions,
  VerificationMethod,
  VerificationMethods,
} from '../types'

type ApiCall = (() => Promise<unknown>) | null

interface InternalState extends SecureVerificationState {
  apiCall: ApiCall
}

const defaultMethods: VerificationMethods = {
  has2FA: false,
  hasPasskey: false,
  passkeySupported: false,
}

const initialState: InternalState = {
  method: null,
  loading: false,
  code: '',
  title: undefined,
  description: undefined,
  scope: undefined,
  apiCall: null,
}

export function useSecureVerification(
  options: UseSecureVerificationOptions = {}
) {
  const {
    onSuccess,
    onError,
    successMessage,
    autoReset = true,
    scope: hookScope,
    methodPolicy: hookMethodPolicy,
  } = options

  const [methods, setMethods] = useState<VerificationMethods>(defaultMethods)
  const [state, setState] = useState<InternalState>(initialState)
  const [open, setOpen] = useState(false)

  const fetchVerificationMethods = useCallback(async () => {
    const result = await checkVerificationMethods()
    setMethods(result)
    return result
  }, [])

  useEffect(() => {
    fetchVerificationMethods()
  }, [fetchVerificationMethods])

  const reset = useCallback(() => {
    setState(initialState)
    setOpen(false)
  }, [])

  const startVerification = useCallback(
    async (
      apiCall: () => Promise<unknown>,
      config: StartVerificationOptions = {}
    ) => {
      const { preferredMethod, title, description, scope, methodPolicy } =
        config
      const effectiveScope = scope ?? hookScope
      const effectivePolicy = methodPolicy ?? hookMethodPolicy
      const availableMethods = await fetchVerificationMethods()

      let defaultMethod: VerificationMethod | null = preferredMethod ?? null
      if (!defaultMethod) {
        if (effectivePolicy === 'strongest') {
          // 按服务端 gate 规则取最强因子：已有 2FA 只接受 2FA；仅有
          // passkey 只接受 passkey；皆无退回原密码。
          if (availableMethods.has2FA) {
            defaultMethod = '2fa'
          } else if (availableMethods.hasPasskey) {
            defaultMethod = 'passkey'
          } else {
            defaultMethod = 'password'
          }
        } else if (
          availableMethods.hasPasskey &&
          availableMethods.passkeySupported
        ) {
          defaultMethod = 'passkey'
        } else if (availableMethods.has2FA) {
          defaultMethod = '2fa'
        } else {
          // 无二因子时退路为原密码再认证（未设密码的账户由服务端明确报错）。
          defaultMethod = 'password'
        }
      }

      setState((prev) => ({
        ...prev,
        apiCall,
        method: defaultMethod,
        title,
        description,
        scope: effectiveScope,
        methodPolicy: effectivePolicy,
      }))
      setOpen(true)
      return true
    },
    [fetchVerificationMethods, hookScope, hookMethodPolicy]
  )

  const executeVerification = useCallback(
    async (method?: VerificationMethod, code?: string) => {
      if (!state.apiCall) {
        toast.error(i18next.t('auth.tips.verificationIsNotConfiguredProperly'))
        return
      }

      const actualMethod = method ?? state.method
      if (!actualMethod) {
        toast.error(
          i18next.t('auth.placeholders.selectAVerificationMethodFirst')
        )
        return
      }

      setState((prev) => ({ ...prev, loading: true }))

      try {
        await verify(actualMethod, code ?? state.code, state.scope)
        const result = await state.apiCall()

        if (successMessage) {
          toast.success(successMessage)
        }

        onSuccess?.(result, actualMethod)

        if (autoReset) {
          reset()
        }

        return result
      } catch (error) {
        const message =
          error instanceof Error
            ? error.message
            : i18next.t('auth.status.verificationFailed')
        toast.error(message)
        onError?.(error)
        throw error
      } finally {
        setState((prev) => ({ ...prev, loading: false }))
      }
    },
    [state, successMessage, onSuccess, onError, autoReset, reset]
  )

  const setCode = useCallback((code: string) => {
    setState((prev) => ({ ...prev, code }))
  }, [])

  const switchMethod = useCallback((method: VerificationMethod) => {
    setState((prev) => ({ ...prev, method, code: '' }))
  }, [])

  const cancel = useCallback(() => {
    reset()
  }, [reset])

  const withVerification = useCallback(
    async (
      apiCall: () => Promise<unknown>,
      config: StartVerificationOptions = {}
    ) => {
      try {
        return await apiCall()
      } catch (error) {
        if (isVerificationRequiredError(error)) {
          const info = extractVerificationInfo(error)
          toast.info(info.message)
          await startVerification(apiCall, config)
          return null
        }
        throw error
      }
    },
    [startVerification]
  )

  const canUseMethod = useCallback(
    (method: VerificationMethod) => {
      if (method === '2fa') return methods.has2FA
      if (method === 'passkey') {
        return methods.hasPasskey && methods.passkeySupported
      }
      // 原密码再认证：始终可用作退路（未设密码由服务端拒绝）。
      if (method === 'password') return true
      return false
    },
    [methods]
  )

  const recommendedMethod = useMemo<VerificationMethod | null>(() => {
    if (methods.hasPasskey && methods.passkeySupported) return 'passkey'
    if (methods.has2FA) return '2fa'
    return null
  }, [methods])

  return {
    open,
    setOpen,
    methods,
    state,
    startVerification,
    executeVerification,
    cancel,
    reset,
    setCode,
    switchMethod,
    withVerification,
    fetchVerificationMethods,
    canUseMethod,
    recommendedMethod,
    hasAnyMethod: methods.has2FA || methods.hasPasskey,
    isLoading: state.loading,
    currentMethod: state.method,
    code: state.code,
  }
}
