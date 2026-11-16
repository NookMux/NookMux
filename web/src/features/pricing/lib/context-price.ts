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
import { formatCurrencyFromUSD } from '@/lib/currency'
import { TOKEN_UNIT_DIVISORS } from '../constants'
import type { ContextPricingTier, TokenUnit } from '../types'

// ----------------------------------------------------------------------------
// Context-based tiered price utilities
// ----------------------------------------------------------------------------

export type ContextPriceOptions = {
  tokenUnit: TokenUnit
  showRechargePrice?: boolean
  priceRate?: number
  usdExchangeRate?: number
  groupRatioMultiplier?: number
}

/** Price kinds exposed by a context pricing tier. */
export type ContextPriceField =
  | 'input'
  | 'output'
  | 'cache_read'
  | 'cache_write'
  | 'audio_input'
  | 'audio_output'

export type ContextPriceEntry = {
  key: string
  field: ContextPriceField
  label: string
  shortLabel: string
  /** USD price per 1M tokens with the group ratio applied (before token-unit and recharge conversion). */
  value: number
  formatted: string
}

const CONTEXT_PRICE_FIELDS: {
  key: string
  field: ContextPriceField
  label: string
  shortLabel: string
  compute: (base: number, tier: ContextPricingTier) => number
}[] = [
  {
    key: 'input',
    field: 'input',
    label: 'common.fields.inputPrice',
    shortLabel: 'pricing.fields.input',
    compute: (base) => base,
  },
  {
    key: 'output',
    field: 'output',
    label: 'common.fields.completionPrice',
    shortLabel: 'pricing.fields.output',
    compute: (base, tier) => base * tier.completion_ratio,
  },
  {
    key: 'cache_read',
    field: 'cache_read',
    label: 'common.fields.cacheReadPrice',
    shortLabel: 'pricing.fields.cache',
    compute: (base, tier) => base * tier.cache_ratio,
  },
  {
    key: 'cache_write',
    field: 'cache_write',
    label: 'common.fields.cacheCreatePrice',
    shortLabel: 'pricing.fields.cacheWrite',
    compute: (base, tier) => base * tier.create_cache_ratio,
  },
  {
    key: 'audio_input',
    field: 'audio_input',
    label: 'common.fields.audioInputPrice',
    shortLabel: 'pricing.fields.audioInput',
    compute: (base, tier) => base * tier.audio_ratio,
  },
  {
    key: 'audio_output',
    field: 'audio_output',
    label: 'common.fields.audioOutputPrice',
    shortLabel: 'pricing.fields.audioOutput',
    compute: (base, tier) =>
      base * tier.audio_ratio * tier.audio_completion_ratio,
  },
]

/**
 * Compute all display prices of a context pricing tier.
 *
 * The base input price follows the token-price semantics used across the
 * pricing feature: `model_ratio * 2 * groupRatioMultiplier` is the USD price
 * per 1M tokens; every other kind multiplies that base by its tier ratio.
 * Fields whose computed value is missing or zero are omitted, so callers can
 * hide all-zero columns by rendering only the union of returned fields.
 */
export function getContextPriceEntries(
  tier: ContextPricingTier,
  options: ContextPriceOptions
): ContextPriceEntry[] {
  const base = tier.model_ratio * 2 * (options.groupRatioMultiplier ?? 1)

  return CONTEXT_PRICE_FIELDS.flatMap((field) => {
    const value = field.compute(base, tier)
    if (!Number.isFinite(value) || value <= 0) return []

    return [
      {
        key: field.key,
        field: field.field,
        label: field.label,
        shortLabel: field.shortLabel,
        value,
        formatted: formatContextPrice(value, options),
      },
    ]
  })
}

function formatContextPrice(
  valuePerMillionTokens: number,
  options: ContextPriceOptions
): string {
  const priceRate = options.priceRate ?? 1
  const usdExchangeRate = options.usdExchangeRate ?? 1
  const priceUSD =
    valuePerMillionTokens / TOKEN_UNIT_DIVISORS[options.tokenUnit]
  const displayPrice = options.showRechargePrice
    ? (priceUSD * priceRate) / usdExchangeRate
    : priceUSD

  return formatCurrencyFromUSD(displayPrice, {
    digitsLarge: 4,
    digitsSmall: 6,
    abbreviate: false,
  })
}

/**
 * Compact token-range label for a context pricing tier, e.g. "0~128K" or ">=1M".
 */
export function formatTokenRangeShort(
  minTokens: number,
  maxTokens?: number | null
): string {
  const fmt = (n: number) =>
    n >= 1_000_000
      ? `${(n / 1_000_000).toFixed(n % 1_000_000 === 0 ? 0 : 1)}M`
      : n >= 1000
        ? `${(n / 1000).toFixed(n % 1000 === 0 ? 0 : 1)}K`
        : String(n)
  if (maxTokens == null) return `>=${fmt(minTokens)}`
  return `${fmt(minTokens)}~${fmt(maxTokens)}`
}
