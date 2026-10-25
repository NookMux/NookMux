import { api } from '@/lib/api'

export type CustomVoicePreviewParams = {
  file: File
  model: string
  voice_id: string
  text?: string
  need_noise_reduction?: boolean
  need_volume_normalization?: boolean
}

export type CustomVoicePreviewResult = {
  voice_id: string
  demo_audio: string
  file_id: string
  record_id: number
}

export type CustomVoicePreviewResponse = {
  success: boolean
  message: string
  data: CustomVoicePreviewResult
}

export async function previewCustomVoice(
  params: CustomVoicePreviewParams
): Promise<CustomVoicePreviewResponse> {
  const form = new FormData()
  form.append('file', params.file)
  form.append('model', params.model)
  form.append('voice_id', params.voice_id)
  if (params.text) form.append('text', params.text)
  if (params.need_noise_reduction) form.append('need_noise_reduction', 'true')
  if (params.need_volume_normalization)
    form.append('need_volume_normalization', 'true')

  const res = await api.post<CustomVoicePreviewResponse>(
    '/api/custom_voice/preview',
    form,
    {
      headers: { 'Content-Type': 'multipart/form-data' },
    }
  )
  return res.data
}

export type CustomVoiceConfirmResponse = {
  success: boolean
  message: string
  data: {
    voice_id: string
    status: string
  }
}

export type CustomVoiceConfirmQuote = {
  voice_id: string
  quota_cost: number
}

export type CustomVoiceConfirmQuoteResponse = {
  success: boolean
  message: string
  data: CustomVoiceConfirmQuote
}

export type TtsModelOption = {
  value: string
  label: string
}

// 定制音色试听可用的 TTS 增强标签快照（仅暴露 redirect map 的 key）。
export type CustomVoiceTags = {
  enabled: boolean
  emotion_pattern: string
  tone_word_pattern: string
  emotion_tags: string[] | null
  tone_word_tags: string[] | null
}

type CustomVoiceTagsResponse = {
  success: boolean
  message: string
  data: CustomVoiceTags
}

// 仅展示以 tts- 开头的可用模型，与游乐场保持一致地复用 /api/user/models。
export async function getTtsModels(): Promise<TtsModelOption[]> {
  const res = await api.get<{ success: boolean; data?: unknown }>(
    '/api/user/models'
  )
  const { data } = res
  if (!data?.success || !Array.isArray(data.data)) return []

  return (data.data as string[])
    .filter((m) => typeof m === 'string' && m.startsWith('tts-'))
    .map((m) => ({ label: m, value: m }))
}

// 拉取用户侧可见的情绪/语气词标签源值。后端只返回 redirect map 的 key。
export async function getCustomVoiceTags(): Promise<CustomVoiceTags> {
  const res = await api.get<CustomVoiceTagsResponse>('/api/custom_voice/tags')
  const { data } = res
  if (!data?.success || !data.data) {
    return {
      enabled: false,
      emotion_pattern: '',
      tone_word_pattern: '',
      emotion_tags: null,
      tone_word_tags: null,
    }
  }
  return data.data
}

export async function confirmCustomVoice(
  voiceId: string
): Promise<CustomVoiceConfirmResponse> {
  const res = await api.post<CustomVoiceConfirmResponse>(
    '/api/custom_voice/confirm',
    { voice_id: voiceId }
  )
  return res.data
}

export async function getCustomVoiceConfirmQuote(
  voiceId: string
): Promise<CustomVoiceConfirmQuoteResponse> {
  const res = await api.post<CustomVoiceConfirmQuoteResponse>(
    '/api/custom_voice/confirm_quote',
    { voice_id: voiceId }
  )
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
