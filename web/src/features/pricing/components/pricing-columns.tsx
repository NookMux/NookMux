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
import { useTranslation } from 'react-i18next'
import { getLobeIcon } from '@/lib/lobe-icon'
import { type ColumnDef } from '@/lib/tanstack-table'
import {
  Tooltip,
  TooltipContent,
  TooltipProvider,
  TooltipTrigger,
} from '@/components/ui/tooltip'
import { DataTableColumnHeader } from '@/components/data-table/column-header'
import { GroupBadge } from '@/components/group-badge'
import { StatusBadge, StatusBadgeList } from '@/components/status-badge'
import { DEFAULT_TOKEN_UNIT, QUOTA_TYPE_VALUES } from '../constants'
import {
  getDynamicDisplayGroupRatio,
  getDynamicPricingSummary,
} from '../lib/dynamic-price'
import { parseTags } from '../lib/filters'
import { isTokenBasedModel } from '../lib/model-helpers'
import {
  formatPrice,
  formatRequestPrice,
  stripTrailingZeros,
} from '../lib/price'
import type { PricingModel, TokenUnit } from '../types'

// ----------------------------------------------------------------------------
// Pricing Table Columns
// ----------------------------------------------------------------------------

const TOKEN_UNIT_DIVISORS: Record<TokenUnit, number> = { M: 1, K: 1000 }

function formatTokenRangeShort(min: number, max?: number | null): string {
  const fmt = (n: number) =>
    n >= 1_000_000
      ? `${(n / 1_000_000).toFixed(n % 1_000_000 === 0 ? 0 : 1)}M`
      : n >= 1000
        ? `${(n / 1000).toFixed(n % 1000 === 0 ? 0 : 1)}K`
        : String(n)
  if (max == null) return `>=${fmt(min)}`
  return `${fmt(min)}~${fmt(max)}`
}

function computeTierDisplayPrice(
  ratio: number,
  model: PricingModel,
  tokenUnit: TokenUnit,
  showRechargePrice: boolean,
  priceRate: number,
  usdExchangeRate: number
): string {
  const groups = Array.isArray(model.enable_groups) ? model.enable_groups : []
  const ratios = model.group_ratio || {}
  let groupRatio = 1
  if (groups.length > 0) {
    let minR = Infinity
    for (const g of groups) {
      if (ratios[g] !== undefined && ratios[g] < minR) minR = ratios[g]
    }
    if (minR !== Infinity) groupRatio = minR
  }
  const base = ratio * 2 * groupRatio
  let priceUSD = base / TOKEN_UNIT_DIVISORS[tokenUnit]
  if (showRechargePrice) {
    priceUSD = (priceUSD * priceRate) / usdExchangeRate
  }
  return priceUSD < 0.0001
    ? priceUSD.toExponential(2)
    : priceUSD.toFixed(priceUSD >= 1 ? 2 : 4)
}

export interface PricingColumnsOptions {
  tokenUnit?: TokenUnit
  priceRate?: number
  usdExchangeRate?: number
  showRechargePrice?: boolean
}

function renderLimitedTags(
  items: string[],
  maxDisplay: number = 3
): React.ReactNode {
  return (
    <StatusBadgeList
      items={items}
      max={maxDisplay}
      getKey={(item) => item}
      renderItem={(item) => (
        <StatusBadge label={item} autoColor={item} size='sm' copyable={false} />
      )}
    />
  )
}

function renderLimitedGroupBadges(
  groups: string[],
  maxDisplay: number = 2
): React.ReactNode {
  return (
    <StatusBadgeList
      items={groups}
      max={maxDisplay}
      getKey={(group) => group}
      renderItem={(group) => <GroupBadge group={group} size='sm' />}
    />
  )
}

export function usePricingColumns(
  options: PricingColumnsOptions = {}
): ColumnDef<PricingModel>[] {
  const { t } = useTranslation()
  const {
    tokenUnit = DEFAULT_TOKEN_UNIT,
    priceRate = 1,
    usdExchangeRate = 1,
    showRechargePrice = false,
  } = options

  const tokenUnitLabel = tokenUnit === 'K' ? '1K' : '1M'

  return [
    // Model column
    {
      accessorKey: 'model_name',
      meta: { label: t('common.fields.model') },
      header: ({ column }) => (
        <DataTableColumnHeader
          column={column}
          title={t('common.fields.model')}
        />
      ),
      cell: ({ row }) => {
        const model = row.original
        const vendorIcon = model.vendor_icon
          ? getLobeIcon(model.vendor_icon, 14)
          : null

        return (
          <div className='flex min-w-[200px] items-center gap-2'>
            {vendorIcon}
            <span className='truncate font-mono text-sm font-medium'>
              {model.model_name}
            </span>
          </div>
        )
      },
      minSize: 200,
    },

    // Type column
    {
      accessorKey: 'quota_type',
      meta: { label: t('channels.fields.type') },
      header: t('channels.fields.type'),
      cell: ({ row }) => {
        const isTokenBased = row.original.quota_type === QUOTA_TYPE_VALUES.TOKEN
        return (
          <StatusBadge
            label={
              isTokenBased
                ? t('pricing.fields.token')
                : t('home.fields.request')
            }
            variant={isTokenBased ? 'info' : 'neutral'}
            copyable={false}
          />
        )
      },
      size: 80,
      enableSorting: false,
    },

    // Price column
    {
      accessorKey: 'price',
      meta: { label: t('pricing.fields.price') },
      header: ({ column }) => (
        <DataTableColumnHeader
          column={column}
          title={t('pricing.fields.price')}
        />
      ),
      cell: ({ row }) => {
        const model = row.original
        const dynamicSummary = getDynamicPricingSummary(model, {
          tokenUnit,
          showRechargePrice,
          priceRate,
          usdExchangeRate,
          groupRatioMultiplier: getDynamicDisplayGroupRatio(model),
        })

        if (dynamicSummary) {
          if (dynamicSummary.isSpecialExpression) {
            return (
              <div className='max-w-[320px] min-w-[200px]'>
                <div className='text-xs font-medium text-amber-700 dark:text-amber-300'>
                  {t('pricing.fields.specialBillingExpression')}
                </div>
                <div className='text-muted-foreground text-[11px]'>
                  {t('pricing.errors.unableToParseStructuredPricing')}
                </div>
                <code className='text-muted-foreground/70 mt-1 line-clamp-2 block font-mono text-[10px] leading-relaxed break-all'>
                  {dynamicSummary.rawExpression}
                </code>
              </div>
            )
          }

          const primaryEntries = dynamicSummary.primaryEntries.slice(0, 2)
          if (primaryEntries.length === 0) {
            return (
              <span className='text-muted-foreground text-xs'>
                {t('pricing.fields.dynamicPricing')}
              </span>
            )
          }

          return (
            <div className='min-w-[180px]'>
              <span className='font-mono text-sm tabular-nums'>
                {primaryEntries.map((entry, index) => (
                  <span key={entry.key}>
                    {index > 0 && (
                      <span className='text-muted-foreground/40 mx-1'>/</span>
                    )}
                    {stripTrailingZeros(entry.formatted)}
                  </span>
                ))}
              </span>
              <div className='text-muted-foreground/50 text-[10px]'>
                / {tokenUnitLabel} tokens
                {dynamicSummary.tierCount > 1 &&
                  ` · ${t('pricing.fields.countTiers', {
                    count: dynamicSummary.tierCount,
                  })}`}
              </div>
            </div>
          )
        }

        const isTokenBased = isTokenBasedModel(model)
        const contextPricing = model.context_pricing
        const hasContextPricing =
          contextPricing?.enabled && (contextPricing.tiers?.length ?? 0) > 1

        if (!model.available) {
          return (
            <div className='min-w-[100px]'>
              <span className='text-muted-foreground/60 text-sm italic'>
                {t('pricing.fields.unpriced')}
              </span>
            </div>
          )
        }

        if (isTokenBased && hasContextPricing) {
          return (
            <div className='min-w-[200px]'>
              <div className='flex flex-wrap gap-x-3 gap-y-0.5'>
                {contextPricing!.tiers.map((tier, i) => (
                  <span key={i} className='font-mono text-xs tabular-nums'>
                    <span className='text-muted-foreground/50'>
                      {formatTokenRangeShort(tier.min_tokens, tier.max_tokens)}
                    </span>{' '}
                    {computeTierDisplayPrice(
                      tier.model_ratio,
                      model,
                      tokenUnit,
                      showRechargePrice,
                      priceRate,
                      usdExchangeRate
                    )}
                  </span>
                ))}
              </div>
              <div className='text-muted-foreground/50 text-[10px]'>
                / {tokenUnitLabel} tokens · {contextPricing!.tiers.length}{' '}
                {t('pricing.fields.tiers')}
              </div>
            </div>
          )
        }

        if (isTokenBased) {
          const inputPrice = stripTrailingZeros(
            formatPrice(
              model,
              'input',
              tokenUnit,
              showRechargePrice,
              priceRate,
              usdExchangeRate
            )
          )
          const outputPrice = stripTrailingZeros(
            formatPrice(
              model,
              'output',
              tokenUnit,
              showRechargePrice,
              priceRate,
              usdExchangeRate
            )
          )

          return (
            <div className='min-w-[160px]'>
              <span className='font-mono text-sm tabular-nums'>
                {inputPrice}
                <span className='text-muted-foreground/40 mx-1'>/</span>
                {outputPrice}
              </span>
              <div className='text-muted-foreground/50 text-[10px]'>
                / {tokenUnitLabel} tokens
              </div>
            </div>
          )
        }

        const price = stripTrailingZeros(
          formatRequestPrice(
            model,
            showRechargePrice,
            priceRate,
            usdExchangeRate
          )
        )

        return (
          <div className='min-w-[100px]'>
            <span className='font-mono text-sm tabular-nums'>{price}</span>
            <div className='text-muted-foreground/50 text-[10px]'>
              / {t('home.fields.request')}
            </div>
          </div>
        )
      },
      size: 180,
      enableSorting: false,
    },

    // Cached price column (Vercel AI Gateway style)
    {
      id: 'cached_price',
      meta: { label: t('pricing.fields.cached') },
      header: t('pricing.fields.cached'),
      cell: ({ row }) => {
        const model = row.original
        const dynamicSummary = getDynamicPricingSummary(model, {
          tokenUnit,
          showRechargePrice,
          priceRate,
          usdExchangeRate,
          groupRatioMultiplier: getDynamicDisplayGroupRatio(model),
        })

        if (dynamicSummary) {
          if (dynamicSummary.isSpecialExpression) {
            return (
              <span className='text-muted-foreground/50 text-xs'>
                {t('pricing.fields.specialBillingExpression')}
              </span>
            )
          }

          const cacheEntry = dynamicSummary.entries.find(
            (entry) => entry.field === 'cacheReadPrice'
          )
          if (!cacheEntry) {
            return <span className='text-muted-foreground/30 text-xs'>—</span>
          }

          return (
            <div className='min-w-[80px]'>
              <span className='font-mono text-sm tabular-nums'>
                {stripTrailingZeros(cacheEntry.formatted)}
              </span>
              <div className='text-muted-foreground/50 text-[10px]'>
                / {tokenUnitLabel}
              </div>
            </div>
          )
        }

        const isTokenBased = isTokenBasedModel(model)

        if (!isTokenBased || model.cache_ratio == null) {
          return <span className='text-muted-foreground/30 text-xs'>—</span>
        }

        const cachedPrice = stripTrailingZeros(
          formatPrice(
            model,
            'cache',
            tokenUnit,
            showRechargePrice,
            priceRate,
            usdExchangeRate
          )
        )

        return (
          <div className='min-w-[80px]'>
            <span className='font-mono text-sm tabular-nums'>
              {cachedPrice}
            </span>
            <div className='text-muted-foreground/50 text-[10px]'>
              / {tokenUnitLabel}
            </div>
          </div>
        )
      },
      size: 110,
      enableSorting: false,
    },

    // Vendor column
    {
      accessorKey: 'vendor_name',
      meta: { label: t('models.fields.vendor') },
      header: t('models.fields.vendor'),
      cell: ({ row }) => {
        const model = row.original
        if (!model.vendor_name) {
          return <span className='text-muted-foreground/50 text-xs'>—</span>
        }
        const vendorIcon = model.vendor_icon
          ? getLobeIcon(model.vendor_icon, 12)
          : null
        return (
          <span className='flex items-center gap-1.5'>
            {vendorIcon}
            <StatusBadge
              label={model.vendor_name}
              autoColor={model.vendor_name}
              size='sm'
              copyable={false}
            />
          </span>
        )
      },
      size: 130,
      enableSorting: false,
    },

    // Tags column
    {
      accessorKey: 'tags',
      meta: { label: t('models.fields.tags') },
      header: t('models.fields.tags'),
      cell: ({ row }) => {
        const tags = parseTags(row.original.tags)
        if (tags.length === 0) {
          return <span className='text-muted-foreground/50 text-xs'>—</span>
        }

        return (
          <TooltipProvider>
            <Tooltip>
              <TooltipTrigger render={<div />}>
                {renderLimitedTags(tags, 2)}
              </TooltipTrigger>
              {tags.length > 2 && (
                <TooltipContent side='top' className='max-w-[280px] p-2'>
                  <span className='text-xs'>{tags.join(', ')}</span>
                </TooltipContent>
              )}
            </Tooltip>
          </TooltipProvider>
        )
      },
      size: 140,
      enableSorting: false,
    },

    // Endpoints column
    {
      accessorKey: 'supported_endpoint_types',
      meta: { label: t('models.fields.endpoints') },
      header: t('models.fields.endpoints'),
      cell: ({ row }) => {
        const endpoints = row.original.supported_endpoint_types || []
        if (endpoints.length === 0) {
          return <span className='text-muted-foreground/50 text-xs'>—</span>
        }

        return (
          <TooltipProvider>
            <Tooltip>
              <TooltipTrigger render={<div />}>
                {renderLimitedTags(endpoints, 2)}
              </TooltipTrigger>
              {endpoints.length > 2 && (
                <TooltipContent side='top' className='max-w-[280px] p-2'>
                  <span className='text-xs'>{endpoints.join(', ')}</span>
                </TooltipContent>
              )}
            </Tooltip>
          </TooltipProvider>
        )
      },
      size: 130,
      enableSorting: false,
    },

    // Enable Groups column
    {
      accessorKey: 'enable_groups',
      meta: { label: t('channels.fields.groups') },
      header: t('channels.fields.groups'),
      cell: ({ row }) => {
        const groups = row.original.enable_groups || []
        if (groups.length === 0) {
          return <span className='text-muted-foreground/50 text-xs'>—</span>
        }

        return (
          <TooltipProvider>
            <Tooltip>
              <TooltipTrigger render={<div />}>
                {renderLimitedGroupBadges(groups, 2)}
              </TooltipTrigger>
              {groups.length > 2 && (
                <TooltipContent side='top' className='max-w-[280px] p-2'>
                  <div className='flex flex-wrap gap-1'>
                    {groups.map((group) => (
                      <GroupBadge key={group} group={group} size='sm' />
                    ))}
                  </div>
                </TooltipContent>
              )}
            </Tooltip>
          </TooltipProvider>
        )
      },
      size: 130,
      enableSorting: false,
    },
  ]
}
