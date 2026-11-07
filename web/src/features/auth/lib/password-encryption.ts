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
import { api } from '@/lib/api'

interface PasswordEncryptionKey {
  kid: string
  public_key: string
}

export interface EncryptedPassword {
  password_encrypted: string
  encryption_key_id: string
}

const KEY_CACHE_TTL_MS = 5 * 60_000

let cachedKey: PasswordEncryptionKey | null = null
let cachedAt = 0

export function clearPasswordEncryptionCache(): void {
  cachedKey = null
  cachedAt = 0
}

// 登录密码传输加密：优先 WebCrypto（安全上下文），HTTP 内网部署回退
// node-forge。属纵深防御开关，不能替代 HTTPS。
export async function encryptPassword(
  password: string
): Promise<EncryptedPassword> {
  try {
    const key = await getPasswordEncryptionKey()
    const ciphertext = await rsaOaepEncrypt(password, key.public_key)
    return {
      password_encrypted: ciphertext,
      encryption_key_id: key.kid,
    }
  } catch (error: unknown) {
    clearPasswordEncryptionCache()
    throw new Error(i18next.t('auth.status.loginFailed'), { cause: error })
  }
}

async function getPasswordEncryptionKey(): Promise<PasswordEncryptionKey> {
  const now = Date.now()
  if (cachedKey && now - cachedAt < KEY_CACHE_TTL_MS) {
    return cachedKey
  }

  const response = await api.get<{
    success: boolean
    data?: PasswordEncryptionKey & { enabled?: boolean }
  }>('/api/user/login/encryption-key')
  const key = response.data?.data
  if (
    !response.data?.success ||
    key?.enabled !== true ||
    !key?.kid ||
    !key.public_key
  ) {
    throw new Error('Password encryption key is unavailable')
  }
  cachedKey = key
  cachedAt = now
  return key
}

async function rsaOaepEncrypt(
  password: string,
  publicKeyPEM: string
): Promise<string> {
  if (typeof globalThis.crypto?.subtle !== 'undefined') {
    try {
      const publicKey = await globalThis.crypto.subtle.importKey(
        'spki',
        pemToDER(publicKeyPEM),
        { name: 'RSA-OAEP', hash: 'SHA-256' },
        false,
        ['encrypt']
      )
      const ciphertext = await globalThis.crypto.subtle.encrypt(
        { name: 'RSA-OAEP' },
        publicKey,
        new TextEncoder().encode(password)
      )
      return arrayBufferToBase64(ciphertext)
    } catch {
      // 旧实现可能暴露 SubtleCrypto 却不支持所需的 RSA-OAEP 参数，
      // 由下方 HTTP 兼容回退接管。
    }
  }

  // WebCrypto 在浏览器中仅限安全上下文；动态加载 forge 保持 HTTPS 包体
  // 精简的同时支持 HTTP 内网部署。
  const forge = await import('node-forge')
  const publicKey = forge.pki.publicKeyFromPem(publicKeyPEM)
  const ciphertext = publicKey.encrypt(
    forge.util.encodeUtf8(password),
    'RSA-OAEP',
    { md: forge.md.sha256.create() }
  )
  return forge.util.encode64(ciphertext)
}

function pemToDER(pem: string): ArrayBuffer {
  const body = pem
    .replace('-----BEGIN PUBLIC KEY-----', '')
    .replace('-----END PUBLIC KEY-----', '')
    .replaceAll(/\s+/g, '')
  const binary = atob(body)
  const bytes = new Uint8Array(binary.length)
  for (let index = 0; index < binary.length; index += 1) {
    bytes[index] = binary.charCodeAt(index)
  }
  return bytes.buffer
}

function arrayBufferToBase64(buffer: ArrayBuffer): string {
  const bytes = new Uint8Array(buffer)
  let binary = ''
  for (const byte of bytes) {
    binary += String.fromCharCode(byte)
  }
  return btoa(binary)
}
