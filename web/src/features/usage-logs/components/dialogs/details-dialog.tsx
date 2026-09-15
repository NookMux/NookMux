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
import {
  Copy,
  Check,
  Route,
  Settings2,
  AlertTriangle,
  Headphones,
  Monitor,
  Cloud,
  Globe,
  ShieldCheck,
  UserCog,
  Info,
} from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { formatBillingCurrencyFromUSD } from '@/lib/currency'
import { formatLogQuota, formatTokens, formatUseTime } from '@/lib/format'
import { cn } from '@/lib/utils'
import { useCopyToClipboard } from '@/hooks/use-copy-to-clipboard'
import { Button } from '@/components/ui/button'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import { Label } from '@/components/ui/label'
import { ScrollArea } from '@/components/ui/scroll-area'
import { StatusBadge, type StatusBadgeProps } from '@/components/status-badge'
import { DynamicPricingBreakdown } from '@/features/pricing/components/dynamic-pricing-breakdown'
import type { UsageLog } from '../../data/schema'
import { useUsageLogFieldVisibility } from '../../hooks/use-field-visibility'
import type { UsageLogFieldKey } from '../../lib/field-visibility'
import {
  parseLogOther,
  getParamOverrideActionLabel,
  parseAuditLine,
  decodeBillingExprB64,
  getTieredBillingSummary,
  hasAnyCacheTokens,
  isViolationFeeLog,
  getFirstResponseTimeColor,
  getTimeColor,
} from '../../lib/format'
import {
  getLogTypeConfig,
  isPerCallBilling,
  isTimingLogType,
  isClientHeadersLogType,
} from '../../lib/utils'
import type { LogOtherData } from '../../types'

function timingTextColorClass(
  variant: 'success' | 'warning' | 'danger'
): string {
  if (variant === 'success') return 'text-emerald-600'
  if (variant === 'warning') return 'text-amber-600'
  return 'text-rose-600'
}

function DetailRow(props: {
  label: React.ReactNode
  value: React.ReactNode
  mono?: boolean
  muted?: boolean
}) {
  return (
    <div className='grid min-w-0 grid-cols-[5.25rem_minmax(0,1fr)] gap-2 text-sm sm:grid-cols-[7rem_minmax(0,1fr)] sm:gap-3'>
      <span className='text-muted-foreground min-w-0 text-xs'>
        {props.label}
      </span>
      <span
        className={cn(
          'max-w-full min-w-0 text-xs break-all sm:wrap-break-word',
          props.mono && 'font-mono',
          props.muted && 'text-muted-foreground'
        )}
      >
        {props.value}
      </span>
    </div>
  )
}

function DetailSection(props: {
  icon?: React.ReactNode
  label: string
  variant?: 'default' | 'danger'
  children: React.ReactNode
}) {
  const isDanger = props.variant === 'danger'
  return (
    <div className='min-w-0 space-y-1.5'>
      <Label
        className={cn(
          'flex items-center gap-1.5 text-xs font-semibold',
          isDanger && 'text-red-500'
        )}
      >
        {props.icon}
        {props.label}
      </Label>
      <div
        className={cn(
          'min-w-0 space-y-1 overflow-hidden rounded-md border p-2.5 max-sm:p-2',
          isDanger
            ? 'border-red-200 bg-red-50 dark:border-red-900 dark:bg-red-950/20'
            : 'bg-muted/30'
        )}
      >
        {props.children}
      </div>
    </div>
  )
}

function compactRatio(ratio: number | undefined): string {
  if (ratio == null || !Number.isFinite(ratio)) return '-'
  return ratio % 1 === 0
    ? String(ratio)
    : ratio.toFixed(4).replace(/\.?0+$/, '')
}

function isValidRatio(ratio: number | undefined): boolean {
  return ratio != null && Number.isFinite(ratio) && ratio !== -1
}

function getEffectiveGroupRatio(other: LogOtherData): {
  labelKey: string
  value: number
} | null {
  if (isValidRatio(other.user_group_ratio)) {
    return {
      labelKey: 'usageLogs.fields.userExclusiveRatio',
      value: other.user_group_ratio!,
    }
  }
  if (isValidRatio(other.group_ratio)) {
    return {
      labelKey: 'systemSettings.fields.groupRatio',
      value: other.group_ratio!,
    }
  }
  return null
}

function getDynamicRatio(other: LogOtherData): number {
  if (
    other.dynamic_ratio != null &&
    Number.isFinite(other.dynamic_ratio) &&
    other.dynamic_ratio > 0
  ) {
    return other.dynamic_ratio
  }
  return 1
}

function getCacheCreationTotal(other: LogOtherData): number {
  const splitTotal =
    (other.cache_creation_tokens_5m || 0) +
    (other.cache_creation_tokens_1h || 0)
  if (splitTotal > 0) return splitTotal
  return other.cache_creation_tokens || 0
}

function getAudioInputTokens(other: LogOtherData): number {
  if (other.audio_input != null) return other.audio_input
  if (other.audio_input_token_count != null)
    return other.audio_input_token_count
  return 0
}

function getOrdinaryInputTokens(log: UsageLog, other: LogOtherData): number {
  if ((other.audio || other.ws) && other.text_input != null) {
    return Math.max(other.text_input, 0)
  }

  const cacheRead = other.cache_tokens || 0
  const cacheCreation = getCacheCreationTotal(other)
  const audioInput = other.audio_input_seperate_price
    ? other.audio_input_token_count || 0
    : 0
  const input =
    (log.prompt_tokens || 0) - cacheRead - cacheCreation - audioInput
  return Math.max(input, 0)
}

function formatExactTokens(tokens: number): string {
  return Number.isFinite(tokens) ? Math.max(tokens, 0).toLocaleString() : '-'
}

function getPriceFormatter() {
  const priceOpts = { digitsLarge: 4, digitsSmall: 6, abbreviate: false }
  return (usd: number | null | undefined) =>
    formatBillingCurrencyFromUSD(usd, priceOpts)
}

function formatTokenRange(min?: number, max?: number): string {
  const formatBound = (value?: number) => {
    if (value == null || !Number.isFinite(value)) return '-'
    if (value >= 1000000 && value % 1000000 === 0) {
      return `${value / 1000000}M`
    }
    if (value >= 1000 && value % 1000 === 0) {
      return `${value / 1000}K`
    }
    return value.toLocaleString()
  }

  const minValue = min || 0
  if (max == null) return `>=${formatBound(minValue)}`
  if (minValue <= 0) return `<${formatBound(max)}`
  return `${formatBound(minValue)}-${formatBound(max)}`
}

function getContextPricingRange(other: LogOtherData): string {
  const contextPricing = other.context_pricing
  return formatTokenRange(
    other.context_pricing_tier_min_tokens ?? contextPricing?.min_tokens,
    other.context_pricing_tier_max_tokens ?? contextPricing?.max_tokens
  )
}

function getContextPricingPrices(other: LogOtherData) {
  return other.context_pricing_prices || other.context_pricing?.prices || null
}

type BillingRow = {
  labelKey: string
  quantity: string
  unitPrice: string
  ratios: string
  subtotal: string
}

function pushTokenBillingRow(args: {
  rows: BillingRow[]
  labelKey: string
  tokens: number
  unitPriceUSD: number | undefined
  groupRatio: number
  dynamicRatio: number
  ratioText: string
  formatPrice: (usd: number | null | undefined) => string
}) {
  if (args.tokens <= 0) return
  if (args.unitPriceUSD == null || !Number.isFinite(args.unitPriceUSD)) return

  const subtotalUSD =
    (args.tokens / 1000000) *
    args.unitPriceUSD *
    args.groupRatio *
    args.dynamicRatio
  args.rows.push({
    labelKey: args.labelKey,
    quantity: formatExactTokens(args.tokens),
    unitPrice: `${args.formatPrice(args.unitPriceUSD)}/M`,
    ratios: args.ratioText,
    subtotal: args.formatPrice(subtotalUSD),
  })
}

function pushMeteredBillingRow(args: {
  rows: BillingRow[]
  labelKey: string
  quantity: number
  unitPriceUSD: number | undefined
  unitLabel: string
  divisor: number
  groupRatio: number
  dynamicRatio: number
  ratioText: string
  formatPrice: (usd: number | null | undefined) => string
}) {
  if (args.quantity <= 0) return
  if (args.unitPriceUSD == null || !Number.isFinite(args.unitPriceUSD)) return

  const subtotalUSD =
    (args.quantity / args.divisor) *
    args.unitPriceUSD *
    args.groupRatio *
    args.dynamicRatio
  args.rows.push({
    labelKey: args.labelKey,
    quantity: args.quantity.toLocaleString(),
    unitPrice: `${args.formatPrice(args.unitPriceUSD)}/${args.unitLabel}`,
    ratios: args.ratioText,
    subtotal: args.formatPrice(subtotalUSD),
  })
}

function buildBillingRows(
  log: UsageLog,
  other: LogOtherData,
  t: (key: string) => string
): BillingRow[] {
  const rows: BillingRow[] = []
  const formatPrice = getPriceFormatter()
  const contextPrices = getContextPricingPrices(other)
  const modelRatio = contextPrices?.model_ratio ?? other.model_ratio
  const completionRatio =
    contextPrices?.completion_ratio ?? other.completion_ratio
  const baseInputUSD =
    modelRatio != null && Number.isFinite(modelRatio) ? modelRatio * 2 : 0
  const effectiveGroupRatio = getEffectiveGroupRatio(other)
  const groupRatio = effectiveGroupRatio?.value ?? 1
  const dynamicRatio = getDynamicRatio(other)
  const ratioLabel =
    effectiveGroupRatio?.labelKey || 'systemSettings.fields.groupRatio'
  const ratioParts = [`${t(ratioLabel)} ${compactRatio(groupRatio)}x`]
  if (dynamicRatio !== 1) {
    ratioParts.push(
      `${t('dynamicRatio.fields.ratio')} ${compactRatio(dynamicRatio)}x`
    )
  }
  const ratioText = ratioParts.join(' * ')

  if (isPerCallBilling(other.model_price)) {
    const subtotalUSD = other.model_price! * groupRatio * dynamicRatio
    rows.push({
      labelKey: 'common.fields.modelPrice',
      quantity: '1',
      unitPrice: formatPrice(other.model_price),
      ratios: ratioText,
      subtotal: formatPrice(subtotalUSD),
    })
    return rows
  }

  pushTokenBillingRow({
    rows,
    labelKey: 'usageLogs.fields.inputTokens',
    tokens: getOrdinaryInputTokens(log, other),
    unitPriceUSD: baseInputUSD,
    groupRatio,
    dynamicRatio,
    ratioText,
    formatPrice,
  })
  pushTokenBillingRow({
    rows,
    labelKey: 'usageLogs.fields.outputTokens',
    tokens: log.completion_tokens || 0,
    unitPriceUSD: baseInputUSD * (completionRatio ?? 0),
    groupRatio,
    dynamicRatio,
    ratioText,
    formatPrice,
  })
  pushTokenBillingRow({
    rows,
    labelKey: 'systemSettings.fields.cacheRead',
    tokens: other.cache_tokens || 0,
    unitPriceUSD:
      baseInputUSD * (contextPrices?.cache_ratio ?? other.cache_ratio ?? 0),
    groupRatio,
    dynamicRatio,
    ratioText,
    formatPrice,
  })

  const cacheWrite5m = other.cache_creation_tokens_5m || 0
  const cacheWrite1h = other.cache_creation_tokens_1h || 0
  const hasSplitCacheWrite = cacheWrite5m > 0 || cacheWrite1h > 0
  if (hasSplitCacheWrite) {
    pushTokenBillingRow({
      rows,
      labelKey: 'usageLogs.fields.cacheCreation5m',
      tokens: cacheWrite5m,
      unitPriceUSD:
        baseInputUSD *
        (contextPrices?.cache_creation_ratio_5m ??
          other.cache_creation_ratio_5m ??
          other.cache_creation_ratio ??
          0),
      groupRatio,
      dynamicRatio,
      ratioText,
      formatPrice,
    })
    pushTokenBillingRow({
      rows,
      labelKey: 'usageLogs.fields.cacheCreation1h',
      tokens: cacheWrite1h,
      unitPriceUSD:
        baseInputUSD *
        (contextPrices?.cache_creation_ratio_1h ??
          other.cache_creation_ratio_1h ??
          other.cache_creation_ratio ??
          0),
      groupRatio,
      dynamicRatio,
      ratioText,
      formatPrice,
    })
  } else {
    pushTokenBillingRow({
      rows,
      labelKey: 'systemSettings.fields.cacheCreation',
      tokens: other.cache_creation_tokens || 0,
      unitPriceUSD:
        baseInputUSD *
        (contextPrices?.cache_creation_ratio ??
          other.cache_creation_ratio ??
          0),
      groupRatio,
      dynamicRatio,
      ratioText,
      formatPrice,
    })
  }

  const audioInputUnitPrice = other.audio_input_seperate_price
    ? other.audio_input_price
    : baseInputUSD * (contextPrices?.audio_ratio ?? other.audio_ratio ?? 0)
  const audioOutputUnitPrice =
    baseInputUSD *
    (contextPrices?.audio_ratio ?? other.audio_ratio ?? 0) *
    (contextPrices?.audio_completion_ratio ?? other.audio_completion_ratio ?? 0)
  pushTokenBillingRow({
    rows,
    labelKey: 'pricing.fields.audioInput',
    tokens: getAudioInputTokens(other),
    unitPriceUSD: audioInputUnitPrice,
    groupRatio,
    dynamicRatio,
    ratioText,
    formatPrice,
  })
  pushTokenBillingRow({
    rows,
    labelKey: 'pricing.fields.audioOutput',
    tokens: other.audio_output || 0,
    unitPriceUSD: audioOutputUnitPrice,
    groupRatio,
    dynamicRatio,
    ratioText,
    formatPrice,
  })
  pushTokenBillingRow({
    rows,
    labelKey: 'usageLogs.fields.imageOutput',
    tokens: other.image_output || 0,
    unitPriceUSD: baseInputUSD * (other.image_ratio ?? 0),
    groupRatio,
    dynamicRatio,
    ratioText,
    formatPrice,
  })
  pushMeteredBillingRow({
    rows,
    labelKey: 'common.fields.webSearch',
    quantity: other.web_search_call_count || 0,
    unitPriceUSD: other.web_search_price,
    unitLabel: t('usageLogs.placeholders.value1KCalls'),
    divisor: 1000,
    groupRatio,
    dynamicRatio,
    ratioText,
    formatPrice,
  })
  pushMeteredBillingRow({
    rows,
    labelKey: 'common.fields.fileSearch',
    quantity: other.file_search_call_count || 0,
    unitPriceUSD: other.file_search_price,
    unitLabel: t('usageLogs.placeholders.value1KCalls'),
    divisor: 1000,
    groupRatio,
    dynamicRatio,
    ratioText,
    formatPrice,
  })
  pushMeteredBillingRow({
    rows,
    labelKey: 'common.fields.imageGeneration',
    quantity: other.image_generation_call ? 1 : 0,
    unitPriceUSD: other.image_generation_call_price,
    unitLabel: t('usageLogs.fields.call'),
    divisor: 1,
    groupRatio,
    dynamicRatio,
    ratioText,
    formatPrice,
  })

  return rows
}

function BillingBreakdown(props: { log: UsageLog; other: LogOtherData }) {
  const { t } = useTranslation()
  const { isVisible } = useUsageLogFieldVisibility()
  const { log, other } = props
  const isPerCall = isPerCallBilling(other.model_price)
  const isTieredExpr = other.billing_mode === 'tiered_expr'
  const tieredSummary = getTieredBillingSummary(other)
  const isContextPricing = other.context_pricing_enabled === true
  const billingRows = buildBillingRows(log, other, t)
  const summaryRows: Array<{ label: string; value: string }> = []
  const multiplierRows: Array<{ label: string; value: string }> = []
  const contextPrices = getContextPricingPrices(other)
  const modelRatio = contextPrices?.model_ratio ?? other.model_ratio
  const completionRatio =
    contextPrices?.completion_ratio ?? other.completion_ratio

  if (isTieredExpr) {
    summaryRows.push({
      label: t('usageLogs.fields.billingMode'),
      value: t('pricing.fields.dynamicPricing'),
    })
    if (tieredSummary) {
      if (tieredSummary.tier.label) {
        summaryRows.push({
          label: t('usageLogs.fields.matchedTier'),
          value: tieredSummary.tier.label,
        })
      }
    } else {
      summaryRows.push({
        label: t('usageLogs.fields.matchedTier'),
        value: t('channels.fields.noMatchingResults'),
      })
    }
  } else if (isPerCall) {
    summaryRows.push({
      label: t('usageLogs.fields.billingMode'),
      value: t('usageLogs.fields.perCallBilling'),
    })
  } else {
    const modeKey = isContextPricing
      ? 'usageLogs.fields.perTokenSegmentedBilling'
      : 'usageLogs.fields.perTokenNonSegmentedBilling'
    summaryRows.push({
      label: t('usageLogs.fields.billingMode'),
      value: t(modeKey),
    })
  }

  if (isContextPricing) {
    summaryRows.push({
      label: t('usageLogs.fields.matchedSegment'),
      value: [
        getContextPricingRange(other),
        other.context_pricing_tier_name || other.context_pricing?.tier_name,
      ]
        .filter(Boolean)
        .join(' '),
    })
    summaryRows.push({
      label: t('usageLogs.fields.segmentContextTokens'),
      value: formatExactTokens(other.context_tokens_for_tier || 0),
    })
  }

  if (modelRatio != null && Number.isFinite(modelRatio)) {
    multiplierRows.push({
      label: t('usageLogs.fields.modelRatio'),
      value: `${compactRatio(modelRatio)}x`,
    })
  }
  if (completionRatio != null && Number.isFinite(completionRatio)) {
    multiplierRows.push({
      label: t('usageLogs.fields.completionRatio'),
      value: `${compactRatio(completionRatio)}x`,
    })
  }

  const effectiveGroupRatio = getEffectiveGroupRatio(other)
  if (effectiveGroupRatio) {
    multiplierRows.push({
      label: t(effectiveGroupRatio.labelKey),
      value: `${compactRatio(effectiveGroupRatio.value)}x`,
    })
  }

  const dynamicRatio = getDynamicRatio(other)
  if (dynamicRatio !== 1) {
    multiplierRows.push({
      label: t('dynamicRatio.fields.ratio'),
      value: `${compactRatio(dynamicRatio)}x`,
    })
  }

  const ratioEntries = [
    [
      'common.fields.cacheReadRatio',
      contextPrices?.cache_ratio ?? other.cache_ratio,
    ],
    [
      'common.fields.cacheCreationRatio',
      contextPrices?.cache_creation_ratio ?? other.cache_creation_ratio,
    ],
    [
      'common.fields.cacheCreation5mRatio',
      contextPrices?.cache_creation_ratio_5m ?? other.cache_creation_ratio_5m,
    ],
    [
      'common.fields.cacheCreation1hRatio',
      contextPrices?.cache_creation_ratio_1h ?? other.cache_creation_ratio_1h,
    ],
    [
      'common.fields.audioInputRatio',
      contextPrices?.audio_ratio ?? other.audio_ratio,
    ],
    [
      'common.fields.audioOutputRatio',
      contextPrices?.audio_completion_ratio ?? other.audio_completion_ratio,
    ],
  ] as const

  for (const [labelKey, value] of ratioEntries) {
    if (value != null && Number.isFinite(value)) {
      multiplierRows.push({
        label: t(labelKey),
        value: `${compactRatio(value)}x`,
      })
    }
  }

  if (isVisible('billing_source') && other.admin_info) {
    summaryRows.push({
      label: t('usageLogs.fields.billingSource'),
      value: other.admin_info.local_count_tokens
        ? t('usageLogs.fields.localBilling')
        : t('usageLogs.fields.upstreamResponse'),
    })
  }

  summaryRows.push({
    label: t('usageLogs.fields.totalCost'),
    value: formatLogQuota(log.quota),
  })

  if (summaryRows.length === 0 && billingRows.length === 0) return null

  return (
    <DetailSection label={t('usageLogs.titles.billingDetails')}>
      <div className='grid gap-1.5 md:grid-cols-2'>
        {summaryRows.map((row, idx) => (
          <DetailRow key={idx} label={row.label} value={row.value} mono />
        ))}
      </div>
      {multiplierRows.length > 0 && (
        <div className='border-border/70 mt-2 border-t pt-2'>
          <Label className='mb-1.5 block text-xs font-semibold'>
            {t('usageLogs.fields.multipliers')}
          </Label>
          <div className='grid gap-1.5 md:grid-cols-2'>
            {multiplierRows.map((row, idx) => (
              <DetailRow key={idx} label={row.label} value={row.value} mono />
            ))}
          </div>
        </div>
      )}
      {isVisible('price_table') && billingRows.length > 0 && (
        <div className='border-border/70 mt-2 min-w-0 border-t pt-2'>
          <Label className='mb-1.5 block text-xs font-semibold'>
            {t('usageLogs.fields.currentPriceTable')}
          </Label>
          <div className='overflow-x-auto rounded-md border'>
            <table className='w-full min-w-[680px] text-left text-xs'>
              <thead className='bg-muted/60 text-muted-foreground'>
                <tr>
                  <th className='px-2 py-1.5 font-medium'>
                    {t('usageLogs.fields.billingItem')}
                  </th>
                  <th className='px-2 py-1.5 font-medium'>
                    {t('keys.fields.quantity')}
                  </th>
                  <th className='px-2 py-1.5 font-medium'>
                    {t('usageLogs.fields.unitPrice')}
                  </th>
                  <th className='px-2 py-1.5 font-medium'>
                    {t('usageLogs.fields.ratios')}
                  </th>
                  <th className='px-2 py-1.5 font-medium'>
                    {t('usageLogs.fields.subtotal')}
                  </th>
                </tr>
              </thead>
              <tbody>
                {billingRows.map((row, idx) => (
                  <tr key={idx} className='border-t'>
                    <td className='px-2 py-1.5 font-medium'>
                      {t(row.labelKey)}
                    </td>
                    <td className='px-2 py-1.5 font-mono'>{row.quantity}</td>
                    <td className='px-2 py-1.5 font-mono'>{row.unitPrice}</td>
                    <td className='px-2 py-1.5 font-mono'>{row.ratios}</td>
                    <td className='px-2 py-1.5 font-mono'>{row.subtotal}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </div>
      )}
    </DetailSection>
  )
}

function TokenBreakdown(props: { log: UsageLog; other: LogOtherData }) {
  const { t } = useTranslation()
  const { log, other } = props

  const promptTokens = log.prompt_tokens || 0
  const completionTokens = log.completion_tokens || 0
  const cacheRead = other.cache_tokens || 0
  const cacheWrite = other.cache_creation_tokens || 0
  const cacheWrite5m = other.cache_creation_tokens_5m || 0
  const cacheWrite1h = other.cache_creation_tokens_1h || 0
  const ordinaryInput = getOrdinaryInputTokens(log, other)
  const audioInput = getAudioInputTokens(other)
  const audioOutput = other.audio_output || 0
  const textOutput = other.text_output || 0
  const imageOutput = other.image_output || 0
  const hasTokens =
    promptTokens > 0 ||
    completionTokens > 0 ||
    cacheRead > 0 ||
    getCacheCreationTotal(other) > 0 ||
    audioInput > 0 ||
    audioOutput > 0 ||
    imageOutput > 0

  if (!hasTokens) return null

  const standardRows = [
    {
      label: t('usageLogs.fields.inputTokens'),
      value: formatExactTokens(ordinaryInput),
    },
    {
      label: t('usageLogs.fields.outputTokens'),
      value: formatExactTokens(completionTokens),
    },
  ]
  if (cacheRead > 0 || getCacheCreationTotal(other) > 0) {
    standardRows.push({
      label: t('usageLogs.fields.totalRequestInput'),
      value: formatExactTokens(promptTokens),
    })
  }

  const cacheRows: Array<{ label: string; value: string }> = []
  if (cacheRead > 0) {
    cacheRows.push({
      label: t('systemSettings.fields.cacheRead'),
      value: formatExactTokens(cacheRead),
    })
  }
  if (cacheWrite > 0 && cacheWrite5m === 0 && cacheWrite1h === 0) {
    cacheRows.push({
      label: t('systemSettings.fields.cacheCreation'),
      value: formatExactTokens(cacheWrite),
    })
  }
  if (cacheWrite5m > 0) {
    cacheRows.push({
      label: t('usageLogs.fields.cacheCreation5m'),
      value: formatExactTokens(cacheWrite5m),
    })
  }
  if (cacheWrite1h > 0) {
    cacheRows.push({
      label: t('usageLogs.fields.cacheCreation1h'),
      value: formatExactTokens(cacheWrite1h),
    })
  }

  const multimodalRows: Array<{ label: string; value: string }> = []
  if ((other.audio || other.ws) && other.text_input != null) {
    multimodalRows.push({
      label: t('usageLogs.fields.textInput'),
      value: formatExactTokens(other.text_input),
    })
  }
  if ((other.audio || other.ws) && textOutput > 0) {
    multimodalRows.push({
      label: t('usageLogs.fields.textOutput'),
      value: formatExactTokens(textOutput),
    })
  }
  if (audioInput > 0) {
    multimodalRows.push({
      label: t('pricing.fields.audioInput'),
      value: formatExactTokens(audioInput),
    })
  }
  if (audioOutput > 0) {
    multimodalRows.push({
      label: t('pricing.fields.audioOutput'),
      value: formatExactTokens(audioOutput),
    })
  }
  if (imageOutput > 0) {
    multimodalRows.push({
      label: t('usageLogs.fields.imageOutput'),
      value: formatExactTokens(imageOutput),
    })
  }

  const groups = [
    { title: t('usageLogs.fields.standardTokens'), rows: standardRows },
    { title: t('usageLogs.fields.cacheTokens'), rows: cacheRows },
    { title: t('usageLogs.fields.multimodalTokens'), rows: multimodalRows },
  ]

  return (
    <DetailSection label={t('usageLogs.fields.tokenBreakdown')}>
      <div className='grid gap-2 md:grid-cols-3'>
        {groups.map((group) => (
          <div
            key={group.title}
            className='bg-background/50 min-w-0 rounded-md border p-2'
          >
            <div className='text-muted-foreground mb-1.5 text-xs font-medium'>
              {group.title}
            </div>
            {group.rows.length > 0 ? (
              <div className='space-y-1'>
                {group.rows.map((row, idx) => (
                  <DetailRow
                    key={idx}
                    label={row.label}
                    value={row.value}
                    mono
                  />
                ))}
              </div>
            ) : (
              <span className='text-muted-foreground text-xs'>-</span>
            )}
          </div>
        ))}
      </div>
    </DetailSection>
  )
}

export interface UsageLogDetailsVisibility {
  detailsEnabled: boolean
  isVisible: (field: UsageLogFieldKey) => boolean
}

interface DetailsDialogProps {
  log: UsageLog
  isAdmin: boolean
  open: boolean
  onOpenChange: (open: boolean) => void
  /**
   * 可选的字段可见性覆盖。未提供时回退到 useUsageLogFieldVisibility，
   * 即基于登录态 /api/user/self/usage_log_fields 判断。
   * 密钥用量查询页面使用 TokenAuthReadOnly 的 /api/log/token/usage_log_fields
   * 注入普通用户可见性，避免在公开页调用登录态接口。
   */
  visibilityOverride?: UsageLogDetailsVisibility
  /**
   * 内联模式：仅渲染详情主体（不包含 Dialog 外壳与 Header），
   * 供密钥用量查询页面的折叠行直接嵌入使用。
   * 此时 open / onOpenChange 被忽略。
   */
  inline?: boolean
}

export function DetailsDialog(props: DetailsDialogProps) {
  const { t } = useTranslation()
  const defaultVisibility = useUsageLogFieldVisibility()
  const visibility = props.visibilityOverride ?? defaultVisibility

  // 内联模式：直接返回主体，不渲染 Dialog 外壳。
  if (props.inline) {
    return (
      <DetailsDialogBody
        log={props.log}
        isAdmin={props.isAdmin}
        visibility={visibility}
      />
    )
  }

  const typeConfig = getLogTypeConfig(props.log.type)
  const other = parseLogOther(props.log.other)
  const isViolation = isViolationFeeLog(other)
  const isConsume = props.log.type === 2
  const isTieredBilling =
    isConsume &&
    !isViolation &&
    other?.billing_mode === 'tiered_expr' &&
    !!other?.expr_b64

  let dialogWidthClass = 'sm:max-w-lg'
  if (isConsume && !isViolation) {
    dialogWidthClass = 'sm:max-w-[64rem] lg:max-w-[72rem]'
  } else if (isTieredBilling) {
    dialogWidthClass = 'sm:max-w-4xl lg:max-w-5xl'
  }

  return (
    <Dialog open={props.open} onOpenChange={props.onOpenChange}>
      <DialogContent
        className={cn(
          'min-w-0 overflow-hidden',
          'max-sm:max-h-[calc(100dvh-1.5rem)] max-sm:w-[calc(100vw-1.5rem)] max-sm:max-w-[calc(100vw-1.5rem)] max-sm:p-4',
          dialogWidthClass
        )}
      >
        <DialogHeader className='max-sm:gap-1'>
          <DialogTitle className='flex items-center gap-2 text-base'>
            {t('usageLogs.titles.logDetails')}
            <StatusBadge
              label={t(typeConfig.label)}
              variant={typeConfig.color as StatusBadgeProps['variant']}
              size='sm'
              copyable={false}
            />
          </DialogTitle>
          <DialogDescription className='sr-only'>
            {t('usageLogs.actions.viewTheCompleteDetailsForThisLogEntry')}
          </DialogDescription>
        </DialogHeader>
        <ScrollArea className='max-h-[70vh] min-w-0 overflow-hidden pr-2 max-sm:max-h-[calc(100dvh-7rem)] sm:pr-4'>
          <DetailsDialogBody
            log={props.log}
            isAdmin={props.isAdmin}
            visibility={visibility}
          />
        </ScrollArea>
      </DialogContent>
    </Dialog>
  )
}

/**
 * 详情主体内容（不含 Dialog 外壳）。
 * 被 DetailsDialog（弹窗）和密钥用量查询页面的折叠行（inline）共同复用。
 */
function DetailsDialogBody(props: {
  log: UsageLog
  isAdmin: boolean
  visibility: UsageLogDetailsVisibility
}) {
  const { t } = useTranslation()
  const { isVisible } = props.visibility
  const { copiedText, copyToClipboard } = useCopyToClipboard({ notify: false })
  const details = props.log.content ?? ''
  const other = parseLogOther(props.log.other)

  const isViolation = isViolationFeeLog(other)
  const isRefund = props.log.type === 6
  const isConsume = props.log.type === 2
  const isTopup = props.log.type === 1
  const isManage = props.log.type === 3
  const isSubscription = other?.billing_source === 'subscription'
  const isTieredBilling =
    isConsume &&
    !isViolation &&
    other?.billing_mode === 'tiered_expr' &&
    !!other?.expr_b64
  const hasAudioTokens = other?.ws || other?.audio
  const showTiming = isTimingLogType(props.log.type)
  const showClientHeaders = isClientHeadersLogType(props.log.type)
  const showAdminIp =
    !!props.log.ip &&
    isVisible('ip_address') &&
    (showTiming || (props.isAdmin && isTopup))
  const adminInfo = other?.admin_info
  const topupAuditFields =
    isTopup && isVisible('topup_audit') && adminInfo
      ? ([
          adminInfo.payment_method && {
            label: t('usageLogs.fields.orderPaymentMethod'),
            value: adminInfo.payment_method,
          },
          adminInfo.callback_payment_method && {
            label: t('usageLogs.fields.callbackPaymentMethod'),
            value: adminInfo.callback_payment_method,
          },
          adminInfo.caller_ip && {
            label: t('usageLogs.fields.callbackCallerIp'),
            value: adminInfo.caller_ip,
          },
          adminInfo.server_ip && {
            label: t('usageLogs.fields.serverIp'),
            value: adminInfo.server_ip,
          },
          adminInfo.node_name && {
            label: t('usageLogs.fields.nodeName'),
            value: adminInfo.node_name,
          },
          adminInfo.version && {
            label: t('usageLogs.titles.systemVersion'),
            value: adminInfo.version,
          },
        ].filter(Boolean) as Array<{ label: string; value: string }>)
      : []
  const showLegacyTopupWarning =
    isTopup && isVisible('topup_audit') && !adminInfo
  const showTopupAuditSection =
    isTopup &&
    isVisible('topup_audit') &&
    (topupAuditFields.length > 0 || showLegacyTopupWarning)
  const manageOperator = (() => {
    if (!isManage || !isVisible('operator_admin') || !adminInfo) return null
    const username = adminInfo.admin_username
    const id = adminInfo.admin_id
    const hasUsername = username != null && String(username).trim() !== ''
    const hasId = id != null && String(id).trim() !== ''
    if (!hasUsername && !hasId) return null
    if (hasUsername && hasId) return `${username} (ID: ${id})`
    if (hasUsername) return String(username)
    return `ID: ${id}`
  })()

  const conversionChain =
    other && Array.isArray(other.request_conversion)
      ? other.request_conversion.filter(Boolean)
      : []
  const conversionLabel =
    conversionChain.length <= 1
      ? t('usageLogs.fields.nativeFormat')
      : conversionChain.join(' -> ')
  const showConversion =
    isVisible('request_conversion') &&
    props.log.type !== 6 &&
    (other?.request_path || conversionChain.length > 0)

  const useChannel = other?.admin_info?.use_channel
  const channelChain =
    useChannel && useChannel.length > 0 ? useChannel.join(' → ') : undefined

  return (
    <div className='w-full max-w-full min-w-0 space-y-2.5 overflow-hidden py-1 sm:space-y-3'>
      {/* Overview section - key identifiers */}
      <div className='min-w-0 space-y-1'>
        {isVisible('request_id') && props.log.request_id && (
          <DetailRow
            label={t('usageLogs.fields.requestId')}
            value={props.log.request_id}
            mono
          />
        )}
        {isVisible('upstream_request_id') && props.log.upstream_request_id && (
          <DetailRow
            label={t('usageLogs.fields.upstreamRequestId')}
            value={props.log.upstream_request_id}
            mono
          />
        )}

        {props.log.channel > 0 && (
          <DetailRow
            label={t('channels.fields.channel')}
            value={
              <span>
                {props.log.channel}
                {props.log.channel_name && (
                  <span className='text-muted-foreground'>
                    {' '}
                    ({props.log.channel_name})
                  </span>
                )}
              </span>
            }
            mono
          />
        )}

        {isVisible('retry_chain') && channelChain && (
          <DetailRow
            label={t('usageLogs.actions.retryChain')}
            value={channelChain}
            mono
          />
        )}

        {props.log.token_name && (
          <DetailRow
            label={t('pricing.fields.token')}
            value={props.log.token_name}
            mono
          />
        )}

        {(props.log.group || other?.group) && (
          <DetailRow
            label={t('common.fields.group')}
            value={props.log.group || other?.group || ''}
            mono
          />
        )}

        {showAdminIp && (
          <DetailRow
            label={t('usageLogs.fields.ipAddress')}
            value={
              <span className='flex items-center gap-1'>
                <Globe className='size-3 text-amber-500' aria-hidden='true' />
                {props.log.ip}
              </span>
            }
            mono
          />
        )}

        {showTiming && props.log.use_time > 0 && (
          <DetailRow
            label={t('usageLogs.fields.responseTime')}
            value={
              <span
                className={cn(
                  'font-medium',
                  timingTextColorClass(getTimeColor(props.log.use_time / 1000))
                )}
              >
                {formatUseTime(props.log.use_time / 1000)}
                {props.log.is_stream && other?.frt != null && other.frt > 0 && (
                  <span
                    className={cn(
                      'font-normal',
                      timingTextColorClass(
                        getFirstResponseTimeColor(other.frt / 1000)
                      )
                    )}
                  >
                    {' '}
                    (FRT: {formatUseTime(other.frt / 1000)})
                  </span>
                )}
              </span>
            }
          />
        )}
      </div>

      {/* Client request headers (consume & error logs) */}
      {isVisible('client_headers') &&
        showClientHeaders &&
        (props.log.http_referer || props.log.x_title || props.log.ua) && (
          <DetailSection
            icon={<Monitor className='size-3.5' aria-hidden='true' />}
            label={t('usageLogs.fields.clientRequestHeaders')}
          >
            {props.log.http_referer && (
              <DetailRow
                label='HTTP-Referer'
                value={props.log.http_referer}
                mono
              />
            )}
            {props.log.x_title && (
              <DetailRow label='X-Title' value={props.log.x_title} mono />
            )}
            {props.log.ua && <DetailRow label='UA' value={props.log.ua} mono />}
          </DetailSection>
        )}

      {/* Request conversion (admin only, not for refund) */}
      {showConversion && (
        <DetailSection label={t('usageLogs.fields.requestConversion')}>
          <div className='relative min-w-0'>
            <Button
              variant='ghost'
              size='sm'
              className='absolute top-0 right-0 h-5 w-5 p-0'
              onClick={() => copyToClipboard(conversionLabel)}
              title={t('common.actions.copyToClipboard')}
              aria-label={t('common.actions.copyToClipboard')}
            >
              {copiedText === conversionLabel ? (
                <Check className='size-3 text-green-600' />
              ) : (
                <Copy className='size-3' />
              )}
            </Button>
            <div className='min-w-0 space-y-1 pr-6'>
              {other?.request_path && (
                <DetailRow
                  label={t('usageLogs.fields.path')}
                  value={other.request_path}
                  mono
                />
              )}
              <div className='flex min-w-0 items-center gap-1.5 text-xs'>
                <Route
                  className='text-muted-foreground size-3'
                  aria-hidden='true'
                />
                <span className='min-w-0 break-all sm:wrap-break-word'>
                  {conversionLabel}
                </span>
              </div>
            </div>
          </div>
        </DetailSection>
      )}

      {/* Reject reason & error content */}
      {props.isAdmin && other?.reject_reason && (
        <DetailSection
          icon={<AlertTriangle className='size-3.5' aria-hidden='true' />}
          label={t('usageLogs.fields.rejectReason')}
          variant='danger'
        >
          <p className='text-xs wrap-break-word'>{other.reject_reason}</p>
        </DetailSection>
      )}

      {/* Violation fee info */}
      {isVisible('violation_fee') && isViolation && other && (
        <DetailSection
          icon={<AlertTriangle className='size-3.5' aria-hidden='true' />}
          label={t('usageLogs.fields.violationFee')}
          variant='danger'
        >
          {other.violation_fee_code && (
            <DetailRow
              label={t('usageLogs.fields.violationCode')}
              value={other.violation_fee_code}
              mono
            />
          )}
          {other.violation_fee_marker && (
            <DetailRow
              label={t('usageLogs.fields.violationMarker')}
              value={other.violation_fee_marker}
            />
          )}
          <DetailRow
            label={t('usageLogs.fields.feeAmount')}
            value={formatLogQuota(other.fee_quota ?? props.log.quota)}
            mono
          />
        </DetailSection>
      )}

      {/* Refund details (type=6) */}
      {isVisible('refund_details') &&
        isRefund &&
        other &&
        (other.task_id || other.reason) && (
          <DetailSection label={t('usageLogs.titles.refundDetails')}>
            {other.task_id && (
              <DetailRow
                label={t('systemSettings.fields.taskId')}
                value={other.task_id}
                mono
              />
            )}
            {other.reason && (
              <DetailRow
                label={t('channels.fields.reason')}
                value={other.reason}
              />
            )}
          </DetailSection>
        )}

      {/* Top-up audit info (type=1) */}
      {showTopupAuditSection && (
        <DetailSection
          icon={<ShieldCheck className='size-3.5' aria-hidden='true' />}
          label={t('usageLogs.fields.topUpAuditInfo')}
        >
          {topupAuditFields.map((field, idx) => (
            <DetailRow key={idx} label={field.label} value={field.value} mono />
          ))}
          {showLegacyTopupWarning && (
            <div className='flex items-start gap-1.5 text-xs text-amber-600 dark:text-amber-400'>
              <Info className='mt-0.5 size-3.5 shrink-0' aria-hidden='true' />
              <span>
                {t('usageLogs.tips.recordWasWrittenByAPreUpgradeInstanceAnd')}
              </span>
            </div>
          )}
        </DetailSection>
      )}

      {/* Manage operator (type=3) */}
      {manageOperator && (
        <DetailRow
          label={
            <span className='flex items-center gap-1.5'>
              <UserCog
                className='text-muted-foreground size-3.5'
                aria-hidden='true'
              />
              {t('usageLogs.fields.operatorAdmin')}
            </span>
          }
          value={manageOperator}
          mono
        />
      )}

      {/* Audio/WebSocket token breakdown */}
      {isVisible('audio_tokens') && hasAudioTokens && other && (
        <DetailSection
          icon={<Headphones className='size-3.5' aria-hidden='true' />}
          label={t('usageLogs.fields.audioTokens')}
        >
          {other.audio_input != null && other.audio_input > 0 && (
            <DetailRow
              label={t('pricing.fields.audioInput')}
              value={formatTokens(other.audio_input)}
              mono
            />
          )}
          {other.audio_output != null && other.audio_output > 0 && (
            <DetailRow
              label={t('pricing.fields.audioOutput')}
              value={formatTokens(other.audio_output)}
              mono
            />
          )}
          {other.text_input != null && other.text_input > 0 && (
            <DetailRow
              label={t('usageLogs.fields.textInput')}
              value={formatTokens(other.text_input)}
              mono
            />
          )}
          {other.text_output != null && other.text_output > 0 && (
            <DetailRow
              label={t('usageLogs.fields.textOutput')}
              value={formatTokens(other.text_output)}
              mono
            />
          )}
        </DetailSection>
      )}

      {/* Reasoning effort */}
      {isVisible('reasoning_effort') && other?.reasoning_effort && (
        <DetailRow
          label={t('usageLogs.fields.reasoningEffort')}
          value={
            <StatusBadge
              label={other.reasoning_effort}
              variant={
                other.reasoning_effort === 'max' ||
                other.reasoning_effort === 'xhigh' ||
                other.reasoning_effort === 'high'
                  ? 'red'
                  : other.reasoning_effort === 'medium'
                    ? 'orange'
                    : other.reasoning_effort === 'low' ||
                        other.reasoning_effort === 'minimal' ||
                        other.reasoning_effort === 'none'
                      ? 'green'
                      : 'blue'
              }
              size='sm'
              copyable={false}
            />
          }
        />
      )}

      {/* System prompt override */}
      {isVisible('system_prompt_override') &&
        other?.is_system_prompt_overwritten && (
          <DetailRow
            label={t('usageLogs.titles.systemPrompt')}
            value={
              <StatusBadge
                label={t('usageLogs.fields.overwritten')}
                variant='orange'
                size='sm'
                copyable={false}
              />
            }
          />
        )}

      {/* Model mapping */}
      {isVisible('model_mapping') &&
        other?.is_model_mapped &&
        other?.upstream_model_name && (
          <DetailSection label={t('channels.fields.modelMapping')}>
            <DetailRow
              label={t('usageLogs.fields.requestModel')}
              value={props.log.model_name}
              mono
            />
            <DetailRow
              label={t('usageLogs.fields.actualModel')}
              value={other.upstream_model_name}
              mono
            />
          </DetailSection>
        )}

      {/* Token breakdown (for consume/error types with token data) */}
      {isVisible('token_breakdown') &&
        isDisplayableType(props.log.type) &&
        other && <TokenBreakdown log={props.log} other={other} />}

      {/* Billing breakdown (consume type) */}
      {isVisible('billing_details') && isConsume && other && !isViolation && (
        <BillingBreakdown log={props.log} other={other} />
      )}

      {/* Tiered pricing breakdown (when billing_mode is tiered_expr) */}
      {isVisible('tiered_pricing') && isTieredBilling && other?.expr_b64 && (
        <div className='bg-muted/30 min-w-0 overflow-hidden rounded-md border px-3 max-sm:px-2'>
          <DynamicPricingBreakdown
            billingExpr={decodeBillingExprB64(other.expr_b64)}
            matchedTierLabel={other.matched_tier}
            hideCacheColumns={!hasAnyCacheTokens(other)}
          />
        </div>
      )}

      {/* Admin billing mode indicator for non-consume */}
      {isVisible('billing_source') &&
        props.isAdmin &&
        !isConsume &&
        props.log.type !== 6 &&
        other?.admin_info && (
          <DetailRow
            label={t('usageLogs.fields.billingSource')}
            value={
              <span className='flex items-center gap-1'>
                {other.admin_info.local_count_tokens ? (
                  <Monitor className='size-3 text-blue-500' />
                ) : (
                  <Cloud className='size-3 text-emerald-500' />
                )}
                <span className='text-xs'>
                  {other.admin_info.local_count_tokens
                    ? t('usageLogs.fields.localBilling')
                    : t('usageLogs.fields.upstreamResponse')}
                </span>
              </span>
            }
          />
        )}

      {/* Stream status details */}
      {isVisible('stream_status') &&
        props.isAdmin &&
        other?.stream_status &&
        other.stream_status.status !== 'ok' && (
          <DetailSection label={t('usageLogs.fields.streamStatus')}>
            <DetailRow
              label={t('channels.fields.status')}
              value={
                <StatusBadge
                  label={other.stream_status.status || t('common.errors.error')}
                  variant='red'
                  size='sm'
                  copyable={false}
                />
              }
            />
            {other.stream_status.end_reason && (
              <DetailRow
                label={t('usageLogs.fields.endReason')}
                value={other.stream_status.end_reason}
              />
            )}
            {(other.stream_status.error_count ?? 0) > 0 && (
              <DetailRow
                label={t('usageLogs.fields.softErrors')}
                value={String(other.stream_status.error_count)}
              />
            )}
            {other.stream_status.end_error && (
              <DetailRow
                label={t('usageLogs.fields.endError')}
                value={other.stream_status.end_error}
              />
            )}
            {Array.isArray(other.stream_status.errors) &&
              other.stream_status.errors.length > 0 && (
                <pre className='bg-background/60 mt-1 max-h-32 overflow-y-auto rounded border p-2 font-mono text-[11px] leading-relaxed wrap-break-word whitespace-pre-wrap'>
                  {other.stream_status.errors.join('\n')}
                </pre>
              )}
          </DetailSection>
        )}

      {/* Subscription billing details */}
      {isVisible('subscription_billing') && isSubscription && other && (
        <DetailSection label={t('usageLogs.fields.subscriptionBilling')}>
          {other.subscription_plan_id && (
            <DetailRow
              label={t('subscriptions.fields.plan')}
              value={`#${other.subscription_plan_id} ${other.subscription_plan_title || ''}`.trim()}
            />
          )}
          {other.subscription_id && (
            <DetailRow
              label={t('usageLogs.fields.instance')}
              value={`#${other.subscription_id}`}
              mono
            />
          )}
          {other.subscription_pre_consumed != null && (
            <DetailRow
              label={t('usageLogs.fields.preConsumed')}
              value={formatLogQuota(other.subscription_pre_consumed)}
              mono
            />
          )}
          {other.subscription_post_delta != null &&
            other.subscription_post_delta !== 0 && (
              <DetailRow
                label={t('usageLogs.fields.postDelta')}
                value={formatLogQuota(other.subscription_post_delta)}
                mono
              />
            )}
          {other.subscription_consumed != null && (
            <DetailRow
              label={t('usageLogs.fields.finalConsumed')}
              value={formatLogQuota(other.subscription_consumed)}
              mono
            />
          )}
          {other.subscription_remain != null && (
            <DetailRow
              label={t('channels.fields.remaining')}
              value={`${formatLogQuota(other.subscription_remain)}${other.subscription_total != null ? ` / ${formatLogQuota(other.subscription_total)}` : ''}`}
              mono
            />
          )}
        </DetailSection>
      )}

      {/* Param override */}
      {isVisible('parameter_override') &&
        other?.po &&
        Array.isArray(other.po) &&
        other.po.length > 0 && (
          <DetailSection
            icon={<Settings2 className='size-3.5' aria-hidden='true' />}
            label={`${t('channels.fields.parameterOverride')} (${other.po.length})`}
          >
            {other.po.filter(Boolean).map((line, idx) => {
              const parsed = parseAuditLine(line)
              if (!parsed) return null
              return (
                <div
                  key={idx}
                  className='bg-background/60 flex min-w-0 flex-col gap-1.5 rounded border p-2 sm:flex-row sm:items-start sm:gap-2'
                >
                  <StatusBadge
                    variant='neutral'
                    label={getParamOverrideActionLabel(parsed.action, t)}
                    className='shrink-0 font-medium'
                    copyable={false}
                  />
                  <span className='min-w-0 font-mono text-[11px] leading-relaxed break-all sm:wrap-break-word'>
                    {parsed.content}
                  </span>
                </div>
              )
            })}
          </DetailSection>
        )}

      {/* Content */}
      {details && (
        <div className='space-y-1.5'>
          <Label className='text-xs font-semibold'>
            {t('dashboard.fields.content')}
          </Label>
          <div className='bg-muted/30 relative min-w-0 overflow-hidden rounded-md border p-2.5'>
            <Button
              variant='ghost'
              size='sm'
              className='absolute top-1.5 right-1.5 h-5 w-5 p-0'
              onClick={() => copyToClipboard(details)}
              title={t('common.actions.copyToClipboard')}
              aria-label={t('common.actions.copyToClipboard')}
            >
              {copiedText === details ? (
                <Check className='size-3 text-green-600' />
              ) : (
                <Copy className='size-3' />
              )}
            </Button>
            <p className='min-w-0 pr-6 text-xs leading-relaxed break-all whitespace-pre-wrap sm:wrap-break-word'>
              {details}
            </p>
          </div>
        </div>
      )}
    </div>
  )
}

function isDisplayableType(type: number): boolean {
  return [0, 2, 5, 6].includes(type)
}
