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
export type VerificationMethod = '2fa' | 'passkey' | 'password'

export interface VerificationMethods {
  has2FA: boolean
  hasPasskey: boolean
  passkeySupported: boolean
}

/**
 * 验证方式策略：'any' 展示全部可用方式（无二因子时退回原密码）；
 * 'strongest' 按服务端 gate 规则只接受最强因子（已有 2FA→仅 2FA，仅有
 * passkey→仅 passkey，皆无→原密码），避免引导用户走服务端必拒的死路。
 */
export type VerificationMethodPolicy = 'any' | 'strongest'

export interface SecureVerificationState {
  method: VerificationMethod | null
  loading: boolean
  code: string
  title?: string
  description?: string
  /** 本次验证绑定的用途作用域，随 /api/verify 提交并在校验时逐字比对 */
  scope?: string
  /** 本次验证的方式策略，驱动对话框可选页签 */
  methodPolicy?: VerificationMethodPolicy
}

export interface UseSecureVerificationOptions {
  onSuccess?: (result: unknown, method: VerificationMethod) => void
  onError?: (error: unknown) => void
  successMessage?: string
  autoReset?: boolean
  /** hook 级默认作用域；startVerification 配置可覆盖 */
  scope?: string
  /** hook 级默认方式策略；startVerification 配置可覆盖 */
  methodPolicy?: VerificationMethodPolicy
}

export interface StartVerificationOptions {
  preferredMethod?: VerificationMethod
  title?: string
  description?: string
  /** 本次验证的作用域（如 'access_token'、'admin_user_manage'） */
  scope?: string
  /** 本次验证的方式策略 */
  methodPolicy?: VerificationMethodPolicy
}
