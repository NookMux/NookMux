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
import { api } from '@/lib/api'

export type VoiceRecord = {
  id: number
  created_at: number
  updated_at: number
  type: string
  operator_id: number
  operator_kind: string
  voice_id: string
  quota_cost: number
  redirect_id: string
  allowed: boolean
  remark: string
}

export type VoiceListParams = {
  page?: number
  page_size?: number
  type?: string
  operator_id?: number
  voice_id?: string
  start_timestamp?: number
  end_timestamp?: number
}

export type VoiceListResponse = {
  success: boolean
  message: string
  data: {
    items: VoiceRecord[]
    total: number
    page: number
    page_size: number
  }
}

export async function listVoices(
  params: VoiceListParams
): Promise<VoiceListResponse> {
  const res = await api.get<VoiceListResponse>('/api/custom_voice/voices/', {
    params,
  })
  return res.data
}

export type VoiceUpsertParams = {
  voice_id: string
  type?: string
  redirect_id?: string
  allowed?: boolean
  remark?: string
}

export async function createVoice(
  params: VoiceUpsertParams
): Promise<{ success: boolean; message: string; data: VoiceRecord }> {
  const res = await api.post('/api/custom_voice/voices/', params)
  return res.data
}

export async function updateVoice(
  id: number,
  params: VoiceUpsertParams
): Promise<{ success: boolean; message: string; data: VoiceRecord }> {
  const res = await api.put(`/api/custom_voice/voices/${id}`, params)
  return res.data
}

export async function deleteVoice(
  id: number
): Promise<{ success: boolean; message: string }> {
  const res = await api.delete(`/api/custom_voice/voices/${id}`)
  return res.data
}

// 从 axios 错误对象中安全提取后端业务错误信息，避免在组件中使用 any。
export function extractApiErrorMessage(e: unknown): string | undefined {
  if (typeof e !== 'object' || e === null) return undefined
  const err = e as {
    response?: { data?: { message?: string } }
    message?: string
  }
  return err.response?.data?.message ?? err.message
}
