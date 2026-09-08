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
import { z } from 'zod'
import type { BillingPriceComponentSnapshotData } from '../types'

export const BILLING_TOKEN_FIELDS = [
  'text_input',
  'image_input',
  'audio_input',
  'video_input',
  'document_input',
  'text_output',
  'audio_output',
  'image_output',
  'reasoning_output',
  'accepted_prediction',
  'rejected_prediction',
  'read_cache',
  'write_cache',
  'write_cache_5m',
  'write_cache_1h',
] as const

export type BillingTokenField = (typeof BILLING_TOKEN_FIELDS)[number]
export type BillingDetailsErrorCode =
  | 'malformed_json'
  | 'unknown_version'
  | 'invalid_fields'
  | 'invalid_cache_splits'

export type BillingTokenValues = Record<BillingTokenField, number>

export type BillingDetails =
  | { status: 'missing' }
  | {
      status: 'invalid'
      code: BillingDetailsErrorCode
      errorKey: string
    }
  | { status: 'valid'; tokens: BillingTokenValues }

const nonnegativeToken = z
  .number()
  .int()
  .nonnegative()
  .refine(Number.isSafeInteger)

const billingDetailsSchema = z
  .object({
    schema_version: z.literal(1),
    tokens: z
      .object({
        input: z
          .object({
            text_input: nonnegativeToken,
            image_input: nonnegativeToken,
            audio_input: nonnegativeToken,
            video_input: nonnegativeToken,
            document_input: nonnegativeToken,
          })
          .strict(),
        output: z
          .object({
            text_output: nonnegativeToken,
            audio_output: nonnegativeToken,
            image_output: nonnegativeToken,
            reasoning_output: nonnegativeToken,
            accepted_prediction: nonnegativeToken,
            rejected_prediction: nonnegativeToken,
          })
          .strict(),
        cache: z
          .object({
            read_cache: nonnegativeToken,
            write_cache: nonnegativeToken,
            write_cache_5m: nonnegativeToken,
            write_cache_1h: nonnegativeToken,
          })
          .strict(),
      })
      .strict(),
  })
  .strict()

const parseCache = new Map<string, BillingDetails>()
const parseCacheLimit = 256

function cacheResult(raw: string, result: BillingDetails): BillingDetails {
  parseCache.delete(raw)
  parseCache.set(raw, result)
  if (parseCache.size > parseCacheLimit) {
    const oldest = parseCache.keys().next().value
    if (oldest !== undefined) parseCache.delete(oldest)
  }
  return result
}

function invalid(code: BillingDetailsErrorCode): BillingDetails {
  return {
    status: 'invalid',
    code,
    errorKey: `usageLogs.errors.billingDetails.${code}`,
  }
}

export function parseBillingDetails(raw: unknown): BillingDetails {
  if (raw == null || raw === '') return { status: 'missing' }
  if (typeof raw !== 'string') return invalid('invalid_fields')

  const cached = parseCache.get(raw)
  if (cached) return cached

  let data: unknown
  try {
    data = JSON.parse(raw)
  } catch {
    return cacheResult(raw, invalid('malformed_json'))
  }

  const parsed = billingDetailsSchema.safeParse(data)
  if (!parsed.success) {
    const unknownVersion = parsed.error.issues.some(
      (issue) => issue.path[0] === 'schema_version'
    )
    return cacheResult(
      raw,
      unknownVersion ? invalid('unknown_version') : invalid('invalid_fields')
    )
  }

  const tokens = {} as BillingTokenValues
  for (const group of [
    parsed.data.tokens.input,
    parsed.data.tokens.output,
    parsed.data.tokens.cache,
  ]) {
    for (const [field, value] of Object.entries(group)) {
      if (typeof value === 'number') {
        tokens[field as BillingTokenField] = value
      }
    }
  }
  if (tokens.write_cache - tokens.write_cache_5m < tokens.write_cache_1h) {
    return cacheResult(raw, invalid('invalid_cache_splits'))
  }

  return cacheResult(raw, { status: 'valid', tokens })
}

export function getBillingToken(
  tokens: BillingTokenValues | undefined,
  field: BillingTokenField
): number | null {
  const value = tokens?.[field]
  return typeof value === 'number' && Number.isFinite(value) && value >= 0
    ? value
    : null
}

export function hasOfficialBillingTokens(
  tokens: BillingTokenValues | undefined
): boolean {
  return BILLING_TOKEN_FIELDS.some((field) => tokens?.[field] != null)
}

export function hasOfficialCacheTokens(
  tokens: BillingTokenValues | undefined
): boolean {
  return (
    tokens?.read_cache != null ||
    tokens?.write_cache != null ||
    tokens?.write_cache_5m != null ||
    tokens?.write_cache_1h != null
  )
}

export interface DisplayTokenValues {
  input: number | null
  output: number | null
  cacheRead: number | null
  cacheWrite: number | null
  cacheWriteUnallocated: number | null
  cacheWrite5m: number | null
  cacheWrite1h: number | null
  textInput: number | null
  imageInput: number | null
  audioInput: number | null
  videoInput: number | null
  documentInput: number | null
  textOutput: number | null
  audioOutput: number | null
  imageOutput: number | null
  reasoningOutput: number | null
  acceptedPrediction: number | null
  rejectedPrediction: number | null
  hasValues: boolean
}

export function resolveDisplayTokens(
  billing: BillingDetails
): DisplayTokenValues {
  if (billing.status !== 'valid') {
    return {
      input: null,
      output: null,
      cacheRead: null,
      cacheWrite: null,
      cacheWriteUnallocated: null,
      cacheWrite5m: null,
      cacheWrite1h: null,
      textInput: null,
      imageInput: null,
      audioInput: null,
      videoInput: null,
      documentInput: null,
      textOutput: null,
      audioOutput: null,
      imageOutput: null,
      reasoningOutput: null,
      acceptedPrediction: null,
      rejectedPrediction: null,
      hasValues: false,
    }
  }

  return {
    input: getBillingToken(billing.tokens, 'text_input'),
    output: getBillingToken(billing.tokens, 'text_output'),
    cacheRead: getBillingToken(billing.tokens, 'read_cache'),
    cacheWrite: getBillingToken(billing.tokens, 'write_cache'),
    cacheWriteUnallocated: officialCacheWriteUnallocated(billing.tokens),
    cacheWrite5m: getBillingToken(billing.tokens, 'write_cache_5m'),
    cacheWrite1h: getBillingToken(billing.tokens, 'write_cache_1h'),
    textInput: getBillingToken(billing.tokens, 'text_input'),
    imageInput: getBillingToken(billing.tokens, 'image_input'),
    audioInput: getBillingToken(billing.tokens, 'audio_input'),
    videoInput: getBillingToken(billing.tokens, 'video_input'),
    documentInput: getBillingToken(billing.tokens, 'document_input'),
    textOutput: getBillingToken(billing.tokens, 'text_output'),
    audioOutput: getBillingToken(billing.tokens, 'audio_output'),
    imageOutput: getBillingToken(billing.tokens, 'image_output'),
    reasoningOutput: getBillingToken(billing.tokens, 'reasoning_output'),
    acceptedPrediction: getBillingToken(billing.tokens, 'accepted_prediction'),
    rejectedPrediction: getBillingToken(billing.tokens, 'rejected_prediction'),
    hasValues: hasOfficialBillingTokens(billing.tokens),
  }
}

function officialCacheWriteUnallocated(
  tokens: BillingTokenValues
): number | null {
  const total = getBillingToken(tokens, 'write_cache')
  if (total == null) return null
  const allocated =
    (getBillingToken(tokens, 'write_cache_5m') ?? 0) +
    (getBillingToken(tokens, 'write_cache_1h') ?? 0)
  return Math.max(total - allocated, 0)
}

export interface TokenTooltipRow {
  labelKey: string
  value: number
}

export function buildTokenTooltipRows(
  tokens: DisplayTokenValues
): TokenTooltipRow[] {
  const rows: TokenTooltipRow[] = []

  function add(labelKey: string, value: number | null) {
    if (value != null) {
      rows.push({ labelKey, value })
    }
  }

  add('usageLogs.fields.inputTokens', tokens.input)
  add('usageLogs.fields.outputTokens', tokens.output)
  add('systemSettings.fields.cacheRead', tokens.cacheRead)
  add('systemSettings.fields.cacheCreation', tokens.cacheWrite)
  add('usageLogs.fields.cacheCreation5m', tokens.cacheWrite5m)
  add('usageLogs.fields.cacheCreation1h', tokens.cacheWrite1h)
  if (
    tokens.cacheWriteUnallocated != null &&
    tokens.cacheWriteUnallocated > 0
  ) {
    add(
      'usageLogs.fields.cacheCreationUnallocated',
      tokens.cacheWriteUnallocated
    )
  }
  add('usageLogs.fields.textInput', tokens.textInput)
  add('usageLogs.fields.imageInput', tokens.imageInput)
  add('pricing.fields.audioInput', tokens.audioInput)
  add('usageLogs.fields.videoInput', tokens.videoInput)
  add('usageLogs.fields.documentInput', tokens.documentInput)
  add('usageLogs.fields.textOutput', tokens.textOutput)
  add('pricing.fields.audioOutput', tokens.audioOutput)
  add('usageLogs.fields.imageOutput', tokens.imageOutput)
  add('usageLogs.fields.reasoningOutput', tokens.reasoningOutput)
  add('usageLogs.fields.acceptedPrediction', tokens.acceptedPrediction)
  add('usageLogs.fields.rejectedPrediction', tokens.rejectedPrediction)

  return rows
}

export interface TokenBreakdownRow {
  labelKey: string
  value: string
}

export interface TokenBreakdownGroup {
  titleKey: string
  rows: TokenBreakdownRow[]
}

export function buildTokenBreakdownGroups(
  tokens: DisplayTokenValues,
  options: {
    aggregatePromptTokens: number
    formatTokens: (value: number) => string
  }
): TokenBreakdownGroup[] {
  const { aggregatePromptTokens, formatTokens } = options
  const cacheRead = tokens.cacheRead ?? 0
  const cacheWrite = tokens.cacheWrite ?? 0
  const cacheWrite5m = tokens.cacheWrite5m ?? 0
  const cacheWrite1h = tokens.cacheWrite1h ?? 0
  const cacheWriteUnallocated = tokens.cacheWriteUnallocated ?? 0

  const standardRows: TokenBreakdownRow[] = [
    {
      labelKey: 'usageLogs.fields.inputTokens',
      value: tokens.input == null ? '-' : formatTokens(tokens.input ?? 0),
    },
    {
      labelKey: 'usageLogs.fields.outputTokens',
      value: tokens.textOutput == null ? '-' : formatTokens(tokens.textOutput),
    },
  ]
  if (cacheRead > 0 || cacheWrite > 0) {
    standardRows.push({
      labelKey: 'usageLogs.fields.totalRequestInput',
      value: formatTokens(aggregatePromptTokens),
    })
  }

  const cacheRows: TokenBreakdownRow[] = []
  if (cacheRead > 0) {
    cacheRows.push({
      labelKey: 'systemSettings.fields.cacheRead',
      value: formatTokens(cacheRead),
    })
  }
  if (cacheWrite > 0 && cacheWrite5m === 0 && cacheWrite1h === 0) {
    cacheRows.push({
      labelKey: 'systemSettings.fields.cacheCreation',
      value: formatTokens(cacheWrite),
    })
  }
  if (cacheWrite5m > 0) {
    cacheRows.push({
      labelKey: 'usageLogs.fields.cacheCreation5m',
      value: formatTokens(cacheWrite5m),
    })
  }
  if (cacheWrite1h > 0) {
    cacheRows.push({
      labelKey: 'usageLogs.fields.cacheCreation1h',
      value: formatTokens(cacheWrite1h),
    })
  }
  if (cacheWriteUnallocated > 0) {
    cacheRows.push({
      labelKey: 'usageLogs.fields.cacheCreationUnallocated',
      value: formatTokens(cacheWriteUnallocated),
    })
  }

  const modalityRows: TokenBreakdownRow[] = []
  if ((tokens.textInput ?? 0) > 0) {
    modalityRows.push({
      labelKey: 'usageLogs.fields.textInput',
      value: formatTokens(tokens.textInput ?? 0),
    })
  }
  if ((tokens.imageInput ?? 0) > 0) {
    modalityRows.push({
      labelKey: 'usageLogs.fields.imageInput',
      value: formatTokens(tokens.imageInput ?? 0),
    })
  }
  if ((tokens.videoInput ?? 0) > 0) {
    modalityRows.push({
      labelKey: 'usageLogs.fields.videoInput',
      value: formatTokens(tokens.videoInput ?? 0),
    })
  }
  if ((tokens.documentInput ?? 0) > 0) {
    modalityRows.push({
      labelKey: 'usageLogs.fields.documentInput',
      value: formatTokens(tokens.documentInput ?? 0),
    })
  }
  if ((tokens.audioInput ?? 0) > 0) {
    modalityRows.push({
      labelKey: 'pricing.fields.audioInput',
      value: formatTokens(tokens.audioInput ?? 0),
    })
  }
  if ((tokens.audioOutput ?? 0) > 0) {
    modalityRows.push({
      labelKey: 'pricing.fields.audioOutput',
      value: formatTokens(tokens.audioOutput ?? 0),
    })
  }
  if ((tokens.imageOutput ?? 0) > 0) {
    modalityRows.push({
      labelKey: 'usageLogs.fields.imageOutput',
      value: formatTokens(tokens.imageOutput ?? 0),
    })
  }

  const outputSplitRows: TokenBreakdownRow[] = []
  if ((tokens.reasoningOutput ?? 0) > 0) {
    outputSplitRows.push({
      labelKey: 'usageLogs.fields.reasoningOutput',
      value: formatTokens(tokens.reasoningOutput ?? 0),
    })
  }
  if ((tokens.acceptedPrediction ?? 0) > 0) {
    outputSplitRows.push({
      labelKey: 'usageLogs.fields.acceptedPrediction',
      value: formatTokens(tokens.acceptedPrediction ?? 0),
    })
  }
  if ((tokens.rejectedPrediction ?? 0) > 0) {
    outputSplitRows.push({
      labelKey: 'usageLogs.fields.rejectedPrediction',
      value: formatTokens(tokens.rejectedPrediction ?? 0),
    })
  }

  return [
    {
      titleKey: 'usageLogs.fields.standardTokens',
      rows: standardRows,
    },
    { titleKey: 'usageLogs.fields.cacheTokens', rows: cacheRows },
    { titleKey: 'usageLogs.fields.multimodalTokens', rows: modalityRows },
    { titleKey: 'usageLogs.fields.outputSplitTokens', rows: outputSplitRows },
  ]
}

export function getPriceSnapshotComponentQuantity(
  component: string | undefined,
  tokens: DisplayTokenValues,
  formatTokens: (value: number) => string,
  savedQuantity?: number
): string {
  const tokenMap: Record<string, number | null | undefined> = {
    // Price-table snapshot component names (contract schema).
    input: tokens.input,
    output: tokens.output,
    cache_read: tokens.cacheRead,
    cache_write_5m: tokens.cacheWrite5m,
    cache_write_1h: tokens.cacheWrite1h,
    // billing_details field names (schema v1).
    text_input: tokens.textInput,
    image_input: tokens.imageInput,
    audio_input: tokens.audioInput,
    video_input: tokens.videoInput,
    document_input: tokens.documentInput,
    text_output: tokens.textOutput,
    audio_output: tokens.audioOutput,
    image_output: tokens.imageOutput,
    reasoning_output: tokens.reasoningOutput,
    accepted_prediction: tokens.acceptedPrediction,
    rejected_prediction: tokens.rejectedPrediction,
    read_cache: tokens.cacheRead,
    write_cache: tokens.cacheWrite,
    write_cache_5m: tokens.cacheWrite5m,
    write_cache_1h: tokens.cacheWrite1h,
  }
  if (component == null) return '—'
  if (component === 'request') return '1'
  if (
    savedQuantity !== undefined &&
    !(
      typeof savedQuantity === 'number' &&
      Number.isSafeInteger(savedQuantity) &&
      savedQuantity >= 0
    )
  ) {
    return '—'
  }
  if (
    typeof savedQuantity === 'number' &&
    Number.isSafeInteger(savedQuantity) &&
    savedQuantity >= 0
  ) {
    return formatTokens(savedQuantity)
  }
  const quantity = tokenMap[component]
  return quantity == null ? '—' : formatTokens(quantity)
}

export function getPriceSnapshotComponentLabelKey(
  component: string | undefined
): string {
  const labelMap: Record<string, string> = {
    input: 'usageLogs.fields.inputTokens',
    output: 'usageLogs.fields.outputTokens',
    cache_read: 'systemSettings.fields.cacheRead',
    cache_write_5m: 'usageLogs.fields.cacheCreation5m',
    cache_write_1h: 'usageLogs.fields.cacheCreation1h',
    text_input: 'usageLogs.fields.textInput',
    image_input: 'usageLogs.fields.imageInput',
    audio_input: 'pricing.fields.audioInput',
    video_input: 'usageLogs.fields.videoInput',
    document_input: 'usageLogs.fields.documentInput',
    text_output: 'usageLogs.fields.textOutput',
    audio_output: 'pricing.fields.audioOutput',
    image_output: 'usageLogs.fields.imageOutput',
    reasoning_output: 'usageLogs.fields.reasoningOutput',
    accepted_prediction: 'usageLogs.fields.acceptedPrediction',
    rejected_prediction: 'usageLogs.fields.rejectedPrediction',
    read_cache: 'systemSettings.fields.cacheRead',
    write_cache: 'systemSettings.fields.cacheCreation',
    write_cache_5m: 'usageLogs.fields.cacheCreation5m',
    write_cache_1h: 'usageLogs.fields.cacheCreation1h',
  }
  return component && labelMap[component]
    ? labelMap[component]
    : 'usageLogs.fields.billingItem'
}

export function formatPriceSnapshotUnitPrice(
  component: BillingPriceComponentSnapshotData | undefined
): string | null {
  const unitPrice = component?.unit_price?.trim()
  if (!unitPrice) return null

  // Unknown/missing units must not be silently reinterpreted as per-million.
  const unitSuffix =
    component?.unit === 'per_request'
      ? ''
      : component?.unit === 'per_1m_tokens'
        ? '/M'
        : ''
  const currency = component?.currency?.trim()
  return [`${unitPrice}${unitSuffix}`, currency].filter(Boolean).join(' ')
}
