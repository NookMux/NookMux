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
import type { Modality, ModelCapability, PricingModel } from '../types'

// ----------------------------------------------------------------------------
// Model metadata inference
// ----------------------------------------------------------------------------
//
// The backend does not currently return every descriptive field. This module
// only derives capability and modality signals from existing backend data
// (endpoint types, ratios, tags, model name); unknown numeric/date fields stay
// empty instead of being invented client-side.

const TEXT_INPUT_ENDPOINTS = new Set([
  'openai',
  'openai-response',
  'anthropic',
  'gemini',
  'embeddings',
  'jina-rerank',
])

const IMAGE_OUTPUT_ENDPOINTS = new Set(['image-generation'])
const VIDEO_OUTPUT_ENDPOINTS = new Set(['openai-video'])
const EMBEDDING_ENDPOINTS = new Set(['embeddings', 'jina-rerank'])

const REASONING_NAME_PATTERNS = [
  /^o[1-4](?:[-:_].+)?$/i,
  /reasoning/i,
  /thinking/i,
  /qwq/i,
  /deepseek-r\d/i,
  /grok.*-(?:thinking|reasoning)/i,
]

const VISION_NAME_PATTERNS = [
  /vision/i,
  /vl(?:[-_]|$)/i,
  /multimodal/i,
  /-omni/i,
]

const AUDIO_NAME_PATTERNS = [
  /audio/i,
  /whisper/i,
  /tts/i,
  /voice/i,
  /-realtime/i,
]

const VIDEO_NAME_PATTERNS = [/video/i, /sora/i, /veo/i, /kling/i, /pika/i]

const CODE_NAME_PATTERNS = [/code/i, /-coder/i]

const WEB_SEARCH_PATTERNS = [/web[-_ ]?search/i, /-online/i, /perplexity/i]

const TAG_TO_CAPABILITY: Record<string, ModelCapability> = {
  vision: 'vision',
  multimodal: 'vision',
  reasoning: 'reasoning',
  thinking: 'reasoning',
  tools: 'tools',
  function: 'function_calling',
  'function-calling': 'function_calling',
  streaming: 'streaming',
  json: 'json_mode',
  structured: 'structured_output',
  search: 'web_search',
  code: 'code_interpreter',
  embedding: 'embeddings',
}

const TAG_TO_MODALITY: Record<string, Modality> = {
  text: 'text',
  image: 'image',
  audio: 'audio',
  video: 'video',
  file: 'file',
  document: 'file',
  pdf: 'file',
}

function parseModelTags(tagsString?: string): string[] {
  if (!tagsString) return []
  return tagsString
    .split(/[,;|\s]+/)
    .map((t) => t.trim().toLowerCase())
    .filter(Boolean)
}

function nameMatches(name: string, patterns: RegExp[]): boolean {
  return patterns.some((re) => re.test(name))
}

function inferInputModalities(
  model: PricingModel,
  tags: string[],
  endpoints: string[],
  name: string
): Modality[] {
  const set = new Set<Modality>()

  if (
    endpoints.length === 0 ||
    endpoints.some((e) => TEXT_INPUT_ENDPOINTS.has(e))
  ) {
    set.add('text')
  }

  if (model.image_ratio != null || nameMatches(name, VISION_NAME_PATTERNS)) {
    set.add('image')
  }
  if (model.audio_ratio != null || nameMatches(name, AUDIO_NAME_PATTERNS)) {
    set.add('audio')
  }
  if (nameMatches(name, VIDEO_NAME_PATTERNS)) {
    set.add('video')
  }

  for (const tag of tags) {
    const m = TAG_TO_MODALITY[tag]
    if (m) set.add(m)
  }

  if (set.size === 0) set.add('text')
  return ordered(set)
}

function inferOutputModalities(
  model: PricingModel,
  endpoints: string[],
  name: string
): Modality[] {
  const set = new Set<Modality>()

  if (endpoints.some((e) => IMAGE_OUTPUT_ENDPOINTS.has(e))) set.add('image')
  if (endpoints.some((e) => VIDEO_OUTPUT_ENDPOINTS.has(e))) set.add('video')
  if (endpoints.some((e) => EMBEDDING_ENDPOINTS.has(e))) set.add('text')

  if (
    model.audio_completion_ratio != null ||
    /tts|voice|audio-out/i.test(name)
  ) {
    set.add('audio')
  }

  if (set.size === 0) set.add('text')
  return ordered(set)
}

function inferCapabilities(
  model: PricingModel,
  tags: string[],
  endpoints: string[],
  name: string,
  outputs: Modality[],
  inputs: Modality[]
): ModelCapability[] {
  const set = new Set<ModelCapability>()

  if (outputs.includes('text') && !endpoints.includes('image-generation')) {
    set.add('streaming')
    set.add('system_prompt')
  }
  if (
    !endpoints.includes('image-generation') &&
    !endpoints.includes('embeddings') &&
    !endpoints.includes('jina-rerank')
  ) {
    set.add('function_calling')
    set.add('tools')
    set.add('json_mode')
    set.add('structured_output')
  }
  if (inputs.includes('image')) set.add('vision')
  if (model.cache_ratio != null) set.add('caching')
  if (endpoints.some((e) => EMBEDDING_ENDPOINTS.has(e))) set.add('embeddings')
  if (nameMatches(name, REASONING_NAME_PATTERNS)) set.add('reasoning')
  if (nameMatches(name, CODE_NAME_PATTERNS)) set.add('code_interpreter')
  if (nameMatches(name, WEB_SEARCH_PATTERNS)) set.add('web_search')

  for (const tag of tags) {
    const cap = TAG_TO_CAPABILITY[tag]
    if (cap) set.add(cap)
  }

  return Array.from(set)
}

function ordered(modalities: Set<Modality>): Modality[] {
  const order: Modality[] = ['text', 'image', 'audio', 'video', 'file']
  return order.filter((m) => modalities.has(m))
}

export type ModelMetadata = {
  context_length: number
  max_output_tokens: number
  knowledge_cutoff: string
  release_date: string
  parameter_count: string
  input_modalities: Modality[]
  output_modalities: Modality[]
  capabilities: ModelCapability[]
}

/**
 * Infer model metadata from backend-provided fields and stable model traits.
 */
export function inferModelMetadata(model: PricingModel): ModelMetadata {
  const name = model.model_name || ''
  const tags = parseModelTags(model.tags)
  const endpoints = model.supported_endpoint_types || []

  const inputs =
    model.input_modalities ?? inferInputModalities(model, tags, endpoints, name)
  const outputs =
    model.output_modalities ?? inferOutputModalities(model, endpoints, name)
  const capabilities =
    model.capabilities ??
    inferCapabilities(model, tags, endpoints, name, outputs, inputs)

  return {
    context_length: model.context_length ?? 0,
    max_output_tokens: model.max_output_tokens ?? 0,
    knowledge_cutoff: model.knowledge_cutoff ?? '',
    release_date: model.release_date ?? '',
    parameter_count: model.parameter_count ?? '',
    input_modalities: inputs,
    output_modalities: outputs,
    capabilities,
  }
}

const TOKEN_FORMAT = new Intl.NumberFormat(undefined, {
  maximumFractionDigits: 1,
})

/** Format a token count compactly: 128_000 → "128K", 1_000_000 → "1M". */
export function formatTokenCount(tokens: number): string {
  if (!Number.isFinite(tokens) || tokens <= 0) return '—'
  if (tokens >= 1_000_000) {
    const value = tokens / 1_000_000
    return `${TOKEN_FORMAT.format(value)}M`
  }
  if (tokens >= 1_000) {
    const value = tokens / 1_000
    return `${TOKEN_FORMAT.format(value)}K`
  }
  return TOKEN_FORMAT.format(tokens)
}

/** Format a YYYY-MM (or YYYY-MM-DD) date as `Mon YYYY` for display. */
export function formatYearMonth(value: string): string {
  if (!value) return '—'
  const [yearStr, monthStr] = value.split('-')
  const year = Number(yearStr)
  const month = Number(monthStr)
  if (!Number.isFinite(year) || !Number.isFinite(month)) return value
  const date = new Date(Date.UTC(year, month - 1, 1))
  return date.toLocaleString(undefined, { year: 'numeric', month: 'short' })
}

// ---------------------------------------------------------------------------
// Provider / vendor / tokenizer / license inference
// ---------------------------------------------------------------------------
//
// These helpers derive vendor-style metadata from the model name. They are
// purely heuristic and serve only the API-info display until the backend
// returns explicit fields.

type ModelVendor =
  | 'openai'
  | 'anthropic'
  | 'google'
  | 'meta'
  | 'mistral'
  | 'qwen'
  | 'deepseek'
  | 'xai'
  | 'cohere'
  | 'baidu'
  | 'zhipu'
  | 'moonshot'
  | 'minimax'
  | 'tencent'
  | 'bytedance'
  | 'midjourney'
  | 'stability'
  | 'unknown'

export type ApiInfo = {
  vendor: ModelVendor
  vendor_label: string
  tokenizer: string
  tokenizer_note?: string
  license: string
  license_kind: 'proprietary' | 'open' | 'open-weight' | 'unknown'
  data_retention_days?: number
  training_opt_out?: boolean
  homepage?: string
}

const VENDOR_LABELS: Record<ModelVendor, string> = {
  openai: 'common.fields.openAi',
  anthropic: 'common.fields.anthropic',
  google: 'Google',
  meta: 'Meta',
  mistral: 'Mistral AI',
  qwen: 'Alibaba (Qwen)',
  deepseek: 'common.fields.deepSeek',
  xai: 'common.fields.xAi',
  cohere: 'common.fields.cohere',
  baidu: 'common.fields.baidu',
  zhipu: 'Zhipu AI',
  moonshot: 'Moonshot AI',
  minimax: 'common.fields.miniMax',
  tencent: 'common.fields.tencent',
  bytedance: 'common.fields.byteDance',
  midjourney: 'common.fields.midjourney',
  stability: 'Stability AI',
  unknown: 'channels.fields.unknown',
}

function detectVendor(name: string): ModelVendor {
  const n = name.toLowerCase()
  if (/^gpt|^o[1-4]|davinci|babbage|whisper|tts|dall.?e|sora|^omni/.test(n))
    return 'openai'
  if (/claude/.test(n)) return 'anthropic'
  if (/gemini|gemma|imagen|veo|palm/.test(n)) return 'google'
  if (/llama|^codellama/.test(n)) return 'meta'
  if (/mistral|mixtral|codestral|magistral|pixtral/.test(n)) return 'mistral'
  if (/qwen|qwq|qvq/.test(n)) return 'qwen'
  if (/deepseek/.test(n)) return 'deepseek'
  if (/grok/.test(n)) return 'xai'
  if (/command|cohere|aya/.test(n)) return 'cohere'
  if (/ernie|wenxin/.test(n)) return 'baidu'
  if (/glm|chatglm|cogview|cogvideo/.test(n)) return 'zhipu'
  if (/kimi|moonshot/.test(n)) return 'moonshot'
  if (/abab|minimax|hailuo/.test(n)) return 'minimax'
  if (/hunyuan/.test(n)) return 'tencent'
  if (/doubao|seed|jimeng/.test(n)) return 'bytedance'
  if (/midjourney|niji/.test(n)) return 'midjourney'
  if (/^sd-|stable[-_]?diffusion|sdxl/.test(n)) return 'stability'
  return 'unknown'
}

const TOKENIZER_BY_VENDOR: Partial<Record<ModelVendor, string>> = {
  openai: 'o200k_base',
  anthropic: 'Anthropic Claude tokenizer',
  google: 'SentencePiece (Gemini)',
  meta: 'Llama 3 tokenizer',
  mistral: 'Mistral tokenizer (BPE)',
  qwen: 'Qwen tokenizer (tiktoken-compat)',
  deepseek: 'DeepSeek tokenizer (BPE)',
  xai: 'Grok tokenizer (BPE)',
  cohere: 'Cohere tokenizer',
  baidu: 'Ernie tokenizer',
  zhipu: 'GLM tokenizer',
  moonshot: 'Kimi tokenizer',
  minimax: 'ABAB tokenizer',
  tencent: 'Hunyuan tokenizer',
  bytedance: 'Doubao tokenizer',
}

function inferTokenizer(
  model: PricingModel,
  vendor: ModelVendor
): {
  tokenizer: string
  note?: string
} {
  const name = model.model_name.toLowerCase()
  if (vendor === 'openai') {
    if (/gpt-3|davinci|babbage|whisper|tts/.test(name)) {
      return { tokenizer: 'cl100k_base', note: 'Older GPT-3.5 family' }
    }
    return { tokenizer: 'o200k_base' }
  }
  return { tokenizer: TOKENIZER_BY_VENDOR[vendor] ?? 'BPE (vendor-specific)' }
}

const LICENSE_BY_VENDOR: Record<
  ModelVendor,
  { license: string; kind: ApiInfo['license_kind'] }
> = {
  openai: { license: 'Proprietary (commercial)', kind: 'proprietary' },
  anthropic: { license: 'Proprietary (commercial)', kind: 'proprietary' },
  google: { license: 'Proprietary (commercial)', kind: 'proprietary' },
  meta: { license: 'Llama Community License', kind: 'open-weight' },
  mistral: { license: 'Apache 2.0 / Commercial', kind: 'open-weight' },
  qwen: { license: 'Tongyi Qianwen License', kind: 'open-weight' },
  deepseek: { license: 'DeepSeek License', kind: 'open-weight' },
  xai: { license: 'Proprietary (commercial)', kind: 'proprietary' },
  cohere: { license: 'Proprietary (commercial)', kind: 'proprietary' },
  baidu: { license: 'Proprietary (commercial)', kind: 'proprietary' },
  zhipu: { license: 'GLM-4 License', kind: 'open-weight' },
  moonshot: { license: 'Proprietary (commercial)', kind: 'proprietary' },
  minimax: { license: 'Proprietary (commercial)', kind: 'proprietary' },
  tencent: { license: 'Hunyuan License', kind: 'open-weight' },
  bytedance: { license: 'Proprietary (commercial)', kind: 'proprietary' },
  midjourney: { license: 'Proprietary (commercial)', kind: 'proprietary' },
  stability: { license: 'Stability AI Community License', kind: 'open-weight' },
  unknown: { license: 'Provider-specific', kind: 'unknown' },
}

const HOMEPAGE_BY_VENDOR: Partial<Record<ModelVendor, string>> = {
  openai: 'https://platform.openai.com/docs/models',
  anthropic: 'https://docs.anthropic.com/claude/docs/models-overview',
  google: 'https://ai.google.dev/models',
  meta: 'https://llama.meta.com/',
  mistral: 'https://docs.mistral.ai/getting-started/models/',
  qwen: 'https://qwenlm.github.io/',
  deepseek: 'https://api-docs.deepseek.com/',
  xai: 'https://x.ai/api',
  cohere: 'https://docs.cohere.com/docs/models',
  baidu: 'https://cloud.baidu.com/product/wenxinworkshop',
  zhipu: 'https://open.bigmodel.cn/dev/api',
  moonshot: 'https://platform.moonshot.cn/docs',
  minimax: 'https://platform.minimaxi.com/document/notice',
  tencent: 'https://cloud.tencent.com/document/product/1729',
  bytedance: 'https://www.volcengine.com/docs/82379',
  midjourney: 'https://www.midjourney.com/',
  stability: 'https://platform.stability.ai/',
}

/**
 * Build vendor / tokenizer / license metadata for the model.
 */
export function inferApiInfo(model: PricingModel): ApiInfo {
  const vendor = detectVendor(model.model_name || '')
  const tk = inferTokenizer(model, vendor)
  const license = LICENSE_BY_VENDOR[vendor]
  return {
    vendor,
    vendor_label: model.vendor_name || VENDOR_LABELS[vendor],
    tokenizer: tk.tokenizer,
    tokenizer_note: tk.note,
    license: license.license,
    license_kind: license.kind,
    data_retention_days: model.data_retention_days ?? undefined,
    training_opt_out: model.training_opt_out ?? undefined,
    homepage: HOMEPAGE_BY_VENDOR[vendor],
  }
}
