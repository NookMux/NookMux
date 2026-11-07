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
import { useEffect, useMemo, useRef, useState } from 'react'
import { useQuery } from '@tanstack/react-query'
import { Plus, Search, Trash2, Loader2 } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { cn } from '@/lib/utils'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { RadioGroup, RadioGroupItem } from '@/components/ui/radio-group'
import {
  Select,
  SelectContent,
  SelectGroup,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'
import { getEnabledModels } from '@/features/channels/api'
import { safeJsonParse } from '../utils/json-parser'
import { formatPricingNumber } from './pricing-format'

type ModelRatioVisualEditorProps = {
  modelPrice: string
  modelRatio: string
  cacheRatio: string
  createCacheRatio: string
  completionRatio: string
  audioRatio: string
  audioCompletionRatio: string
  contextPricing: string
  onChange: (field: ModelRatioField, value: string) => void
  onValidityChange?: (isValid: boolean) => void
  isSaving?: boolean
}

export type ModelRatioField =
  | 'ModelPrice'
  | 'ModelRatio'
  | 'CacheRatio'
  | 'CreateCacheRatio'
  | 'CompletionRatio'
  | 'AudioRatio'
  | 'AudioCompletionRatio'
  | 'ContextPricing'

type NumericMap = Record<string, number>
type UnknownMap = Record<string, unknown>
type PricingMode =
  | 'per-request'
  | 'per-token'
  | 'per-token-length'
  | 'unconfigured'

type ContextTier = {
  name?: string
  min_tokens: number
  max_tokens: number | null
  tokenPrice: string
  completionTokenPrice: string
  cacheTokenPrice: string
  createCacheTokenPrice: string
  audioTokenPrice: string
  audioCompletionTokenPrice: string
}
type BackendContextTier = Record<string, unknown>

type ModelRow = {
  name: string
  mode: PricingMode
  fixedPrice?: number
  inputPrice?: number
  completionPrice?: number
  cachePrice?: number
  createCachePrice?: number
  audioInputPrice?: number
  audioOutputPrice?: number
  contextPricing?: unknown
  contextTiers?: ContextTier[]
}

const PAGE_SIZE_OPTIONS = [10, 20, 50, 100]
const numberInputPattern = /^(\d+(\.\d*)?|\.\d*)?$/
type ContextTierPriceField = Exclude<
  keyof ContextTier,
  'name' | 'min_tokens' | 'max_tokens'
>
const contextTierPriceFields = [
  'tokenPrice',
  'completionTokenPrice',
  'cacheTokenPrice',
  'createCacheTokenPrice',
  'audioTokenPrice',
  'audioCompletionTokenPrice',
] satisfies ReadonlyArray<ContextTierPriceField>

function getContextTierPriceLabel(
  field: ContextTierPriceField,
  t: (key: string) => string
) {
  switch (field) {
    case 'tokenPrice':
      return t('pricing.fields.input')
    case 'completionTokenPrice':
      return t('pricing.fields.output')
    case 'cacheTokenPrice':
      return t('systemSettings.fields.cacheRead')
    case 'createCacheTokenPrice':
      return t('systemSettings.fields.cacheCreation')
    case 'audioTokenPrice':
      return t('pricing.fields.audioInput')
    case 'audioCompletionTokenPrice':
      return t('pricing.fields.audioOutput')
  }
}

/** Values that look like an in-progress decimal input (e.g. "2.", ".") */
const isDeferredDecimal = (value: string) =>
  value === '.' || (typeof value === 'string' && /\.$/.test(value))

function hasValue(value: unknown) {
  return value !== '' && value !== undefined && value !== null
}

function parseNumber(value: string) {
  if (!hasValue(value)) return null
  const parsed = Number(value)
  return Number.isFinite(parsed) ? parsed : null
}

function toInputValue(value: number | undefined) {
  if (value === undefined || value === null || !Number.isFinite(value)) {
    return ''
  }
  return formatPricingNumber(value)
}

// 定价输入框的展示值：0 与未填写同义（免费/待填写），统一显示为空白，
// 由 placeholder 提示建议值；价格摘要等只读场景仍使用 toInputValue。
function toPricingInputValue(value: number | undefined) {
  if (value === 0) return ''
  return toInputValue(value)
}

function normalizeNumber(value: number) {
  return Number(formatPricingNumber(value))
}

function parseNumericMap(value: string, context: string): NumericMap {
  return safeJsonParse<NumericMap>(value, {
    fallback: {},
    context,
  })
}

function sortedJson<T extends Record<string, unknown>>(map: T) {
  const cleanedEntries = Object.entries(map).filter(([, value]) => {
    if (value === undefined || value === null || value === '') return false
    if (typeof value === 'number' && !Number.isFinite(value)) return false
    return true
  })

  const sorted = Object.fromEntries(
    cleanedEntries.sort(([left], [right]) =>
      left.localeCompare(right, 'en', {
        numeric: true,
        sensitivity: 'base',
      })
    )
  )
  return JSON.stringify(sorted, null, 2)
}

function buildRow(
  name: string,
  maps: {
    price: NumericMap
    ratio: NumericMap
    cache: NumericMap
    createCache: NumericMap
    completion: NumericMap
    audio: NumericMap
    audioCompletion: NumericMap
    context: UnknownMap
  }
): ModelRow {
  const fixedPrice = maps.price[name]
  const inputRatio = maps.ratio[name]
  const inputPrice =
    typeof inputRatio === 'number' ? normalizeNumber(inputRatio * 2) : undefined
  const audioInputPrice =
    typeof inputPrice === 'number' && typeof maps.audio[name] === 'number'
      ? normalizeNumber(inputPrice * maps.audio[name])
      : undefined

  // Check context pricing first
  const hasContextPricingConfig =
    maps.context[name] !== undefined &&
    typeof maps.context[name] === 'object' &&
    maps.context[name] !== null &&
    (maps.context[name] as Record<string, unknown>).enabled === true &&
    Array.isArray((maps.context[name] as Record<string, unknown>).tiers) &&
    ((maps.context[name] as Record<string, unknown>).tiers as unknown[])
      .length > 0

  const mode: PricingMode =
    fixedPrice !== undefined
      ? 'per-request'
      : hasContextPricingConfig
        ? 'per-token-length'
        : inputRatio !== undefined
          ? 'per-token'
          : 'unconfigured'

  // Parse context pricing tiers for display
  const contextConfig = maps.context[name]
  const isEnabled =
    contextConfig &&
    typeof contextConfig === 'object' &&
    (contextConfig as Record<string, unknown>).enabled === true
  const rawTiers = isEnabled
    ? (contextConfig as Record<string, unknown>).tiers
    : undefined
  const contextTiers: ContextTier[] = Array.isArray(rawTiers)
    ? rawTiers.map((tier: Record<string, unknown>) => {
        const modelRatio =
          typeof tier.model_ratio === 'number' ? tier.model_ratio : 0
        const completionRatio =
          typeof tier.completion_ratio === 'number' ? tier.completion_ratio : 0
        const cacheRatio =
          typeof tier.cache_ratio === 'number' ? tier.cache_ratio : 0
        const createCacheRatio =
          typeof tier.create_cache_ratio === 'number'
            ? tier.create_cache_ratio
            : 0
        const audioRatio =
          typeof tier.audio_ratio === 'number' ? tier.audio_ratio : 0
        const audioCompletionRatio =
          typeof tier.audio_completion_ratio === 'number'
            ? tier.audio_completion_ratio
            : 0
        const baseRatio = modelRatio
        const basePrice = normalizeNumber(baseRatio * 2)
        return {
          name: typeof tier.name === 'string' ? tier.name : '',
          min_tokens: typeof tier.min_tokens === 'number' ? tier.min_tokens : 0,
          max_tokens:
            typeof tier.max_tokens === 'number' ? tier.max_tokens : null,
          tokenPrice: basePrice ? formatPricingNumber(basePrice) : '0',
          completionTokenPrice:
            basePrice && completionRatio
              ? formatPricingNumber(basePrice * completionRatio)
              : '0',
          cacheTokenPrice:
            basePrice && cacheRatio
              ? formatPricingNumber(basePrice * cacheRatio)
              : '0',
          createCacheTokenPrice:
            basePrice && createCacheRatio
              ? formatPricingNumber(basePrice * createCacheRatio)
              : '0',
          audioTokenPrice:
            basePrice && audioRatio
              ? formatPricingNumber(basePrice * audioRatio)
              : '0',
          audioCompletionTokenPrice:
            basePrice && audioCompletionRatio
              ? formatPricingNumber(
                  basePrice * audioRatio * audioCompletionRatio
                )
              : '0',
        }
      })
    : []

  return {
    name,
    mode,
    fixedPrice,
    inputPrice,
    completionPrice:
      typeof inputPrice === 'number' &&
      typeof maps.completion[name] === 'number'
        ? normalizeNumber(inputPrice * maps.completion[name])
        : undefined,
    cachePrice:
      typeof inputPrice === 'number' && typeof maps.cache[name] === 'number'
        ? normalizeNumber(inputPrice * maps.cache[name])
        : undefined,
    createCachePrice:
      typeof inputPrice === 'number' &&
      typeof maps.createCache[name] === 'number'
        ? normalizeNumber(inputPrice * maps.createCache[name])
        : undefined,
    audioInputPrice,
    audioOutputPrice:
      typeof audioInputPrice === 'number' &&
      typeof maps.audioCompletion[name] === 'number'
        ? normalizeNumber(audioInputPrice * maps.audioCompletion[name])
        : undefined,
    contextPricing: maps.context[name],
    contextTiers,
  }
}

function getSortRank(mode: PricingMode) {
  if (mode === 'unconfigured') return 0
  if (mode === 'per-request') return 1
  if (mode === 'per-token') return 2
  return 3
}

function getRowSummary(
  row: ModelRow,
  t: (key: string, options?: Record<string, unknown>) => string
) {
  if (row.mode === 'unconfigured') {
    return t('systemSettings.errors.notConfigured')
  }
  if (row.mode === 'per-request') {
    return row.fixedPrice !== undefined
      ? `$${toInputValue(row.fixedPrice)} / ${t('home.fields.request')}`
      : t('pricing.fields.perRequest')
  }
  if (row.mode === 'per-token-length') {
    const tierCount = row.contextTiers?.length ?? 0
    return t('pricing.fields.countTiers', { count: tierCount })
  }
  return row.inputPrice !== undefined
    ? `$${toInputValue(row.inputPrice)} / 1M ${t('rankings.fields.tokens')}`
    : t('systemSettings.fields.perToken')
}

function PriceInput({
  label,
  value,
  disabled,
  placeholder,
  onChange,
}: {
  label: string
  value: string
  disabled?: boolean
  placeholder?: string
  onChange: (value: string) => void
}) {
  const [draft, setDraft] = useState(value)
  const [focused, setFocused] = useState(false)

  // Sync draft with external value when not focused
  useEffect(() => {
    if (!focused) setDraft(value)
  }, [focused, value])

  const handleChange = (next: string) => {
    if (!numberInputPattern.test(next)) return
    setDraft(next)
    // Defer normalization for in-progress decimal input like "2."
    if (!isDeferredDecimal(next)) {
      onChange(next)
    }
  }

  const handleBlur = () => {
    setFocused(false)
    // Flush the final value on blur
    onChange(draft)
  }

  return (
    <div className='space-y-1.5'>
      <Label className='text-xs'>{label}</Label>
      <div className='flex'>
        <span className='border-input bg-muted text-muted-foreground inline-flex h-8 items-center rounded-l-md border border-r-0 px-3 text-sm'>
          $
        </span>
        <Input
          className='rounded-l-none'
          inputMode='decimal'
          value={focused ? draft : value}
          disabled={disabled}
          placeholder={placeholder}
          onFocus={() => setFocused(true)}
          onChange={(event) => handleChange(event.target.value)}
          onBlur={handleBlur}
        />
      </div>
    </div>
  )
}

export function ModelRatioVisualEditor({
  modelPrice,
  modelRatio,
  cacheRatio,
  createCacheRatio,
  completionRatio,
  audioRatio,
  audioCompletionRatio,
  contextPricing,
  onChange,
  onValidityChange,
  isSaving = false,
}: ModelRatioVisualEditorProps) {
  const { t } = useTranslation()
  const [searchText, setSearchText] = useState('')
  const [pageIndex, setPageIndex] = useState(0)
  const [pageSize, setPageSize] = useState(20)
  const [selectedName, setSelectedName] = useState<string>('')
  const [customModelName, setCustomModelName] = useState('')

  const { data: enabledModelsData, isLoading: isLoadingEnabledModels } =
    useQuery({
      queryKey: ['channel', 'models-enabled'],
      queryFn: getEnabledModels,
      staleTime: 5 * 60 * 1000,
    })

  const maps = useMemo(
    () => ({
      price: parseNumericMap(modelPrice, 'model prices'),
      ratio: parseNumericMap(modelRatio, 'model ratios'),
      cache: parseNumericMap(cacheRatio, 'cache ratios'),
      createCache: parseNumericMap(createCacheRatio, 'create cache ratios'),
      completion: parseNumericMap(completionRatio, 'completion ratios'),
      audio: parseNumericMap(audioRatio, 'audio ratios'),
      audioCompletion: parseNumericMap(
        audioCompletionRatio,
        'audio completion ratios'
      ),
      context: safeJsonParse<UnknownMap>(contextPricing, {
        fallback: {},
        context: 'context pricing',
      }),
    }),
    [
      audioCompletionRatio,
      audioRatio,
      cacheRatio,
      completionRatio,
      contextPricing,
      createCacheRatio,
      modelPrice,
      modelRatio,
    ]
  )

  const rows = useMemo(() => {
    const names = new Set<string>([
      ...Object.keys(maps.price),
      ...Object.keys(maps.ratio),
      ...Object.keys(maps.cache),
      ...Object.keys(maps.createCache),
      ...Object.keys(maps.completion),
      ...Object.keys(maps.audio),
      ...Object.keys(maps.audioCompletion),
      ...Object.keys(maps.context),
      ...(enabledModelsData?.data || []),
    ])

    return Array.from(names)
      .filter((name) => name.trim())
      .map((name) => buildRow(name, maps))
      .sort((left, right) => {
        const rankCompare = getSortRank(left.mode) - getSortRank(right.mode)
        if (rankCompare !== 0) return rankCompare
        return left.name.localeCompare(right.name, 'en', {
          numeric: true,
          sensitivity: 'base',
        })
      })
  }, [enabledModelsData?.data, maps])

  const filteredRows = useMemo(() => {
    const keyword = searchText.trim().toLowerCase()
    if (!keyword) return rows
    return rows.filter((row) => row.name.toLowerCase().includes(keyword))
  }, [rows, searchText])

  const pageCount = Math.max(1, Math.ceil(filteredRows.length / pageSize))
  const safePageIndex = Math.min(pageIndex, pageCount - 1)
  const selectedMode = selectedName
    ? filteredRows.find((row) => row.name === selectedName)?.mode
    : undefined
  const followedKeyRef = useRef('')

  // 当选中模型因定价模式切换导致排序变化、落到当前可视页之外时，
  // 自动将分页指针跳到该模型所在的页，保持编辑连续性。
  // 仅在选中模型或其计费模式变化时触发一次，避免覆盖用户随后的手动翻页。
  useEffect(() => {
    if (!selectedName) return
    const key = `${selectedName}:${selectedMode ?? ''}`
    if (followedKeyRef.current === key) return
    followedKeyRef.current = key
    const targetIndex = filteredRows.findIndex(
      (row) => row.name === selectedName
    )
    if (targetIndex === -1) return
    const targetPage = Math.floor(targetIndex / pageSize)
    if (targetPage !== safePageIndex) {
      setPageIndex(targetPage)
    }
  }, [filteredRows, safePageIndex, selectedMode, selectedName, pageSize])

  const pageRows = filteredRows.slice(
    safePageIndex * pageSize,
    safePageIndex * pageSize + pageSize
  )
  const selectedRow =
    filteredRows.find((row) => row.name === selectedName) || pageRows[0] || null

  const selectModel = (name: string) => {
    setSelectedName(name)
    onValidityChange?.(true)
  }

  const writeMap = (field: ModelRatioField, map: Record<string, unknown>) => {
    onChange(field, sortedJson(map))
  }

  const clearModel = (name: string) => {
    const nextPrice = { ...maps.price }
    const nextRatio = { ...maps.ratio }
    const nextCache = { ...maps.cache }
    const nextCreateCache = { ...maps.createCache }
    const nextCompletion = { ...maps.completion }
    const nextAudio = { ...maps.audio }
    const nextAudioCompletion = { ...maps.audioCompletion }
    const nextContext = { ...maps.context }

    delete nextPrice[name]
    delete nextRatio[name]
    delete nextCache[name]
    delete nextCreateCache[name]
    delete nextCompletion[name]
    delete nextAudio[name]
    delete nextAudioCompletion[name]
    delete nextContext[name]

    writeMap('ModelPrice', nextPrice)
    writeMap('ModelRatio', nextRatio)
    writeMap('CacheRatio', nextCache)
    writeMap('CreateCacheRatio', nextCreateCache)
    writeMap('CompletionRatio', nextCompletion)
    writeMap('AudioRatio', nextAudio)
    writeMap('AudioCompletionRatio', nextAudioCompletion)
    writeMap('ContextPricing', nextContext)
  }

  const setMode = (name: string, mode: PricingMode) => {
    setSelectedName(name)
    clearModel(name)
    if (mode === 'per-request') {
      writeMap('ModelPrice', { ...maps.price, [name]: 0 })
    }
    if (mode === 'per-token') {
      writeMap('ModelRatio', { ...maps.ratio, [name]: 0 })
    }
    if (mode === 'per-token-length') {
      // Default: 0~200K tier with zero prices
      const defaultTiers = [
        {
          min_tokens: 0,
          max_tokens: 200000,
          model_ratio: 0,
          completion_ratio: 0,
          cache_ratio: 0,
          create_cache_ratio: 0,
          audio_ratio: 0,
          audio_completion_ratio: 0,
        },
      ]
      writeMap('ContextPricing', {
        ...maps.context,
        [name]: { enabled: true, tiers: defaultTiers },
      })
    }
  }

  const setFixedPrice = (name: string, value: string) => {
    const parsed = parseNumber(value)
    const next = { ...maps.price }
    // 清空输入写入 0：按次条目保留，已选计费类型不随输入清空而丢失
    next[name] = parsed ?? 0
    writeMap('ModelPrice', next)
  }

  const setInputPrice = (name: string, value: string) => {
    const parsed = parseNumber(value)
    const next = { ...maps.ratio }
    // 清空输入写入 0：按量条目保留，已选计费类型不随输入清空而丢失
    next[name] = parsed === null ? 0 : normalizeNumber(parsed / 2)
    writeMap('ModelRatio', next)
  }

  const setRelativePrice = (
    field: 'CompletionRatio' | 'CacheRatio' | 'CreateCacheRatio' | 'AudioRatio',
    sourceMap: NumericMap,
    name: string,
    value: string,
    basePrice: number | undefined
  ) => {
    const parsed = parseNumber(value)
    const next = { ...sourceMap }
    if (parsed === null) {
      delete next[name]
    } else if (basePrice && basePrice > 0) {
      next[name] = normalizeNumber(parsed / basePrice)
    }
    writeMap(field, next)
  }

  const setAudioOutputPrice = (
    name: string,
    value: string,
    audioInputPrice: number | undefined
  ) => {
    const parsed = parseNumber(value)
    const next = { ...maps.audioCompletion }
    if (parsed === null) {
      delete next[name]
    } else if (audioInputPrice && audioInputPrice > 0) {
      next[name] = normalizeNumber(parsed / audioInputPrice)
    }
    writeMap('AudioCompletionRatio', next)
  }

  const updateContextTier = (
    name: string,
    tierIndex: number,
    field: string,
    value: string
  ) => {
    const current = maps.context[name] as Record<string, unknown> | undefined
    if (!current) return
    const tiers = [...((current.tiers as BackendContextTier[]) || [])]
    if (!tiers[tierIndex]) return

    const tier = { ...tiers[tierIndex] }

    if (field === 'name') {
      if (value.trim()) {
        tier.name = value
      } else {
        delete tier.name
      }
    } else if (field === 'max_tokens') {
      if (value === '') {
        delete tier.max_tokens
      } else {
        const parsed = Number(value)
        if (Number.isFinite(parsed)) tier.max_tokens = parsed
      }
    } else if (field === 'min_tokens') {
      const parsed = Number(value)
      if (Number.isFinite(parsed)) tier.min_tokens = parsed
    } else {
      // Price fields: convert display price to ratio
      const priceValue = value === '' ? 0 : Number(value)
      if (!Number.isFinite(priceValue)) return
      const modelRatio =
        typeof tier.model_ratio === 'number' ? tier.model_ratio : 0
      const basePrice = modelRatio * 2
      switch (field) {
        case 'tokenPrice':
          tier.model_ratio =
            priceValue > 0 ? normalizeNumber(priceValue / 2) : 0
          break
        case 'completionTokenPrice':
          tier.completion_ratio =
            basePrice > 0 ? normalizeNumber(priceValue / basePrice) : 0
          break
        case 'cacheTokenPrice':
          tier.cache_ratio =
            basePrice > 0 ? normalizeNumber(priceValue / basePrice) : 0
          break
        case 'createCacheTokenPrice':
          tier.create_cache_ratio =
            basePrice > 0 ? normalizeNumber(priceValue / basePrice) : 0
          break
        case 'audioTokenPrice':
          tier.audio_ratio =
            basePrice > 0 ? normalizeNumber(priceValue / basePrice) : 0
          break
        case 'audioCompletionTokenPrice': {
          const audioRatio =
            typeof tier.audio_ratio === 'number' ? tier.audio_ratio : 0
          const audioInputPrice = basePrice * audioRatio
          tier.audio_completion_ratio =
            audioInputPrice > 0
              ? normalizeNumber(priceValue / audioInputPrice)
              : 0
          break
        }
      }
    }

    tiers[tierIndex] = tier
    writeMap('ContextPricing', {
      ...maps.context,
      [name]: { ...current, enabled: true, tiers },
    })
  }

  const addContextTier = (name: string) => {
    const current = maps.context[name] as Record<string, unknown> | undefined
    const existingTiers = (current?.tiers as BackendContextTier[]) || []
    const lastTier = existingTiers[existingTiers.length - 1]
    const minTokens =
      typeof lastTier?.max_tokens === 'number' ? lastTier.max_tokens : 200000

    const newTier: BackendContextTier = {
      min_tokens: minTokens,
      model_ratio: 0,
      completion_ratio: 0,
      cache_ratio: 0,
      create_cache_ratio: 0,
      audio_ratio: 0,
      audio_completion_ratio: 0,
    }

    writeMap('ContextPricing', {
      ...maps.context,
      [name]: {
        ...(current || {}),
        enabled: true,
        tiers: [...existingTiers, newTier],
      },
    })
  }

  const removeContextTier = (name: string, tierIndex: number) => {
    const current = maps.context[name] as Record<string, unknown> | undefined
    if (!current) return
    const tiers = ((current.tiers as BackendContextTier[]) || []).filter(
      (_, i) => i !== tierIndex
    )
    if (tiers.length === 0) {
      // Removing last tier disables context pricing
      const next = { ...maps.context }
      delete next[name]
      writeMap('ContextPricing', next)
      return
    }
    writeMap('ContextPricing', {
      ...maps.context,
      [name]: { ...current, enabled: true, tiers },
    })
  }

  const addCustomModel = () => {
    const name = customModelName.trim()
    if (!name) return
    setCustomModelName('')
    selectModel(name)
    setMode(name, 'per-token')
  }

  return (
    <div className='grid min-h-[560px] gap-4 lg:grid-cols-[minmax(0,1.25fr)_minmax(360px,0.75fr)]'>
      <div className='relative'>
        <Card
          className={cn(
            'min-w-0',
            isSaving &&
              'pointer-events-none opacity-60 transition-opacity duration-150'
          )}
        >
          <CardHeader className='border-b'>
            <CardTitle>{t('channels.titles.models')}</CardTitle>
            <div className='flex flex-col gap-2 sm:flex-row'>
              <div className='relative min-w-0 flex-1'>
                <Search className='text-muted-foreground pointer-events-none absolute top-1/2 left-3 h-4 w-4 -translate-y-1/2' />
                <Input
                  className='pl-9'
                  value={searchText}
                  placeholder={t('systemSettings.actions.searchModelName')}
                  onChange={(event) => {
                    setSearchText(event.target.value)
                    setPageIndex(0)
                  }}
                />
              </div>
              <div className='flex gap-2'>
                <Input
                  value={customModelName}
                  placeholder={t('systemSettings.fields.customModelName')}
                  onChange={(event) => setCustomModelName(event.target.value)}
                  onKeyDown={(event) => {
                    if (event.key === 'Enter') {
                      event.preventDefault()
                      addCustomModel()
                    }
                  }}
                />
                <Button
                  type='button'
                  variant='outline'
                  onClick={addCustomModel}
                >
                  <Plus className='h-4 w-4' />
                  {t('channels.actions.add')}
                </Button>
              </div>
            </div>
          </CardHeader>
          <CardContent className='min-h-0 px-0'>
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead className='pl-4'>
                    {t('models.fields.modelName')}
                  </TableHead>
                  <TableHead>
                    {t('systemSettings.fields.billingType')}
                  </TableHead>
                  <TableHead>
                    {t('systemSettings.fields.priceSummary')}
                  </TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {pageRows.length === 0 ? (
                  <TableRow>
                    <TableCell
                      colSpan={3}
                      className='text-muted-foreground h-32 text-center'
                    >
                      {isLoadingEnabledModels
                        ? t('common.tips.loading')
                        : t('common.titles.noModelsFound')}
                    </TableCell>
                  </TableRow>
                ) : (
                  pageRows.map((row) => (
                    <TableRow
                      key={row.name}
                      className={cn(
                        'cursor-pointer',
                        row.name === selectedRow?.name && 'bg-muted/70'
                      )}
                      onClick={() => selectModel(row.name)}
                    >
                      <TableCell className='max-w-[260px] truncate pl-4 font-medium'>
                        {row.name}
                      </TableCell>
                      <TableCell>
                        <Badge
                          variant={
                            row.mode === 'unconfigured'
                              ? 'destructive'
                              : row.mode === 'per-request'
                                ? 'secondary'
                                : row.mode === 'per-token-length'
                                  ? 'default'
                                  : 'outline'
                          }
                        >
                          {row.mode === 'unconfigured'
                            ? t('systemSettings.errors.notConfigured')
                            : row.mode === 'per-request'
                              ? t('pricing.fields.perRequest')
                              : row.mode === 'per-token-length'
                                ? t('systemSettings.fields.tiered')
                                : t('systemSettings.fields.perToken')}
                        </Badge>
                      </TableCell>
                      <TableCell className='max-w-[240px] truncate text-xs'>
                        {getRowSummary(row, t)}
                      </TableCell>
                    </TableRow>
                  ))
                )}
              </TableBody>
            </Table>
            <div className='flex flex-col gap-3 border-t px-4 py-3 sm:flex-row sm:items-center sm:justify-between'>
              <div className='text-muted-foreground text-xs'>
                {t('keys.titles.countModels', { count: filteredRows.length })}
              </div>
              <div className='flex flex-wrap items-center gap-2'>
                <span className='text-muted-foreground text-xs whitespace-nowrap'>
                  {t('common.fields.rowsPerPage')}
                </span>
                <Select
                  value={String(pageSize)}
                  onValueChange={(value) => {
                    setPageSize(Number(value))
                    setPageIndex(0)
                  }}
                >
                  <SelectTrigger className='h-8 w-[70px]'>
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent alignItemWithTrigger={false}>
                    <SelectGroup>
                      {PAGE_SIZE_OPTIONS.map((size) => (
                        <SelectItem key={size} value={String(size)}>
                          {size}
                        </SelectItem>
                      ))}
                    </SelectGroup>
                  </SelectContent>
                </Select>
                <Button
                  type='button'
                  variant='outline'
                  size='sm'
                  disabled={safePageIndex === 0}
                  onClick={() =>
                    setPageIndex(() => Math.max(0, safePageIndex - 1))
                  }
                >
                  {t('common.actions.previous')}
                </Button>
                <span className='text-muted-foreground text-xs'>
                  {safePageIndex + 1} / {pageCount}
                </span>
                <Button
                  type='button'
                  variant='outline'
                  size='sm'
                  disabled={safePageIndex >= pageCount - 1}
                  onClick={() =>
                    setPageIndex(() =>
                      Math.min(pageCount - 1, safePageIndex + 1)
                    )
                  }
                >
                  {t('common.actions.next')}
                </Button>
              </div>
            </div>
          </CardContent>
        </Card>
        {isSaving && (
          <div className='bg-background/40 absolute inset-0 z-10 flex items-center justify-center rounded-lg'>
            <Loader2 className='text-muted-foreground size-8 animate-spin' />
          </div>
        )}
      </div>

      <div className='relative'>
        <Card
          className={cn(
            'min-w-0',
            isSaving &&
              'pointer-events-none opacity-60 transition-opacity duration-150'
          )}
        >
          <CardHeader className='border-b'>
            <CardTitle className='truncate'>
              {selectedRow
                ? selectedRow.name
                : t('systemSettings.titles.priceSettings')}
            </CardTitle>
          </CardHeader>
          <CardContent>
            {!selectedRow ? (
              <div className='text-muted-foreground py-10 text-center text-sm'>
                {t('systemSettings.placeholders.selectAModelToEditPricing')}
              </div>
            ) : (
              <div className='space-y-5' key={selectedRow.name}>
                {selectedRow.mode === 'unconfigured' && (
                  <div className='bg-destructive/10 text-destructive rounded-md px-3 py-2 text-sm'>
                    {t(
                      'systemSettings.tips.modelHasNoPricingConfiguredPleaseSelectABilling'
                    )}
                  </div>
                )}
                <div className='space-y-2'>
                  <Label>{t('systemSettings.fields.billingType')}</Label>
                  <RadioGroup
                    value={selectedRow.mode}
                    onValueChange={(value) =>
                      setMode(selectedRow.name, value as PricingMode)
                    }
                    className='grid gap-2 sm:grid-cols-3'
                  >
                    {[
                      ['per-request', t('pricing.fields.perRequest')],
                      ['per-token', t('systemSettings.fields.perToken')],
                      [
                        'per-token-length',
                        t('systemSettings.fields.tieredPricing'),
                      ],
                    ].map(([value, label]) => (
                      <Label
                        key={value}
                        htmlFor={`billing-type-${value}`}
                        className='border-input bg-background hover:border-primary/40 focus-within:border-primary/50 has-data-checked:border-primary has-data-checked:ring-primary/20 flex cursor-pointer items-center gap-2 rounded-md border p-3 text-sm font-normal transition-colors has-data-checked:ring-2'
                      >
                        <RadioGroupItem
                          id={`billing-type-${value}`}
                          value={value}
                        />
                        {label}
                      </Label>
                    ))}
                  </RadioGroup>
                </div>

                {selectedRow.mode === 'per-request' && (
                  <PriceInput
                    label={t('systemSettings.fields.fixedPricePerRequest')}
                    value={toPricingInputValue(selectedRow.fixedPrice)}
                    placeholder='0.01'
                    onChange={(value) => setFixedPrice(selectedRow.name, value)}
                  />
                )}

                {selectedRow.mode === 'per-token' && (
                  <div className='space-y-4'>
                    <div className='grid gap-3 sm:grid-cols-2'>
                      <PriceInput
                        label={t('systemSettings.fields.inputPricePer1MTokens')}
                        value={toPricingInputValue(selectedRow.inputPrice)}
                        placeholder='2'
                        onChange={(value) =>
                          setInputPrice(selectedRow.name, value)
                        }
                      />
                      <PriceInput
                        label={t(
                          'systemSettings.fields.completionPricePer1MTokens'
                        )}
                        value={toPricingInputValue(selectedRow.completionPrice)}
                        placeholder='4'
                        disabled={!selectedRow.inputPrice}
                        onChange={(value) =>
                          setRelativePrice(
                            'CompletionRatio',
                            maps.completion,
                            selectedRow.name,
                            value,
                            selectedRow.inputPrice
                          )
                        }
                      />
                    </div>
                    <div className='grid gap-3 sm:grid-cols-2'>
                      <PriceInput
                        label={t(
                          'systemSettings.fields.cacheReadPricePer1MTokens'
                        )}
                        value={toPricingInputValue(selectedRow.cachePrice)}
                        placeholder='0.2'
                        disabled={!selectedRow.inputPrice}
                        onChange={(value) =>
                          setRelativePrice(
                            'CacheRatio',
                            maps.cache,
                            selectedRow.name,
                            value,
                            selectedRow.inputPrice
                          )
                        }
                      />
                      <PriceInput
                        label={t(
                          'systemSettings.fields.cacheWritePricePer1MTokens'
                        )}
                        value={toPricingInputValue(
                          selectedRow.createCachePrice
                        )}
                        placeholder='1'
                        disabled={!selectedRow.inputPrice}
                        onChange={(value) =>
                          setRelativePrice(
                            'CreateCacheRatio',
                            maps.createCache,
                            selectedRow.name,
                            value,
                            selectedRow.inputPrice
                          )
                        }
                      />
                    </div>
                    <div className='grid gap-3 sm:grid-cols-2'>
                      <PriceInput
                        label={t(
                          'systemSettings.fields.audioInputPricePer1MTokens'
                        )}
                        value={toPricingInputValue(selectedRow.audioInputPrice)}
                        placeholder='8'
                        disabled={!selectedRow.inputPrice}
                        onChange={(value) =>
                          setRelativePrice(
                            'AudioRatio',
                            maps.audio,
                            selectedRow.name,
                            value,
                            selectedRow.inputPrice
                          )
                        }
                      />
                      <PriceInput
                        label={t(
                          'systemSettings.tips.audioOutputPricePer1MTokens'
                        )}
                        value={toPricingInputValue(
                          selectedRow.audioOutputPrice
                        )}
                        placeholder='16'
                        disabled={!selectedRow.audioInputPrice}
                        onChange={(value) =>
                          setAudioOutputPrice(
                            selectedRow.name,
                            value,
                            selectedRow.audioInputPrice
                          )
                        }
                      />
                    </div>
                  </div>
                )}

                {selectedRow.mode === 'per-token-length' && (
                  <div className='space-y-4'>
                    <div className='flex items-center justify-between gap-2'>
                      <div>
                        <Label className='text-sm font-semibold'>
                          {t('systemSettings.fields.tieredPricing')}
                        </Label>
                        <p className='text-muted-foreground text-xs'>
                          {t(
                            'systemSettings.tips.pricingVariesByInputContextTokenRange'
                          )}
                        </p>
                      </div>
                      <Button
                        type='button'
                        variant='outline'
                        size='sm'
                        onClick={() => addContextTier(selectedRow.name)}
                      >
                        <Plus className='mr-1 h-3 w-3' />
                        {t('systemSettings.actions.addTier')}
                      </Button>
                    </div>

                    {(selectedRow.contextTiers || []).length > 0 ? (
                      <div className='space-y-3'>
                        {(selectedRow.contextTiers || []).map(
                          (tier, tierIdx) => {
                            const maxTokensStr =
                              tier.max_tokens === null
                                ? ''
                                : String(tier.max_tokens)
                            return (
                              <div
                                key={tierIdx}
                                className='bg-muted/20 rounded-md border p-3'
                              >
                                <div className='mb-3 flex items-center gap-2'>
                                  <div className='min-w-0 flex-1 space-y-1.5'>
                                    <Label className='text-xs'>
                                      {t('channels.fields.name')}
                                    </Label>
                                    <Input
                                      className='h-8'
                                      value={tier.name ?? ''}
                                      placeholder={`${t('systemSettings.fields.tierName')} ${tierIdx + 1}`}
                                      onChange={(e) =>
                                        updateContextTier(
                                          selectedRow.name,
                                          tierIdx,
                                          'name',
                                          e.target.value
                                        )
                                      }
                                    />
                                  </div>
                                  <Button
                                    type='button'
                                    variant='ghost'
                                    size='icon'
                                    className='mt-5 h-8 w-8 shrink-0'
                                    disabled={
                                      (selectedRow.contextTiers || []).length <=
                                      1
                                    }
                                    onClick={() =>
                                      removeContextTier(
                                        selectedRow.name,
                                        tierIdx
                                      )
                                    }
                                  >
                                    <Trash2 className='text-destructive h-4 w-4' />
                                  </Button>
                                </div>

                                <div className='mb-3 grid gap-3 sm:grid-cols-2'>
                                  <div className='space-y-1.5 sm:col-span-2'>
                                    <Label className='text-xs'>
                                      {t('models.fields.contextWindow')}
                                    </Label>
                                    <div className='grid gap-2 sm:grid-cols-2'>
                                      <Input
                                        className='h-8'
                                        type='number'
                                        value={tier.min_tokens}
                                        placeholder={t(
                                          'systemSettings.actions.startWindow'
                                        )}
                                        onChange={(e) =>
                                          updateContextTier(
                                            selectedRow.name,
                                            tierIdx,
                                            'min_tokens',
                                            e.target.value
                                          )
                                        }
                                      />
                                      <Input
                                        className='h-8'
                                        type='number'
                                        placeholder={t(
                                          'systemSettings.fields.endWindow'
                                        )}
                                        value={maxTokensStr}
                                        onChange={(e) =>
                                          updateContextTier(
                                            selectedRow.name,
                                            tierIdx,
                                            'max_tokens',
                                            e.target.value
                                          )
                                        }
                                      />
                                    </div>
                                  </div>

                                  {contextTierPriceFields.map((field) => (
                                    <PriceInput
                                      key={field}
                                      label={`${getContextTierPriceLabel(field, t)} ($/1M)`}
                                      value={tier[field]}
                                      onChange={(value) =>
                                        updateContextTier(
                                          selectedRow.name,
                                          tierIdx,
                                          field,
                                          value
                                        )
                                      }
                                    />
                                  ))}
                                </div>
                              </div>
                            )
                          }
                        )}
                      </div>
                    ) : (
                      <div className='text-muted-foreground rounded-md border border-dashed p-4 text-sm'>
                        {t('common.tips.noTiersConfiguredClickAddTierToStart')}
                      </div>
                    )}
                  </div>
                )}

                <Button
                  type='button'
                  variant='outline'
                  className='w-full'
                  onClick={() => clearModel(selectedRow.name)}
                >
                  <Trash2 className='h-4 w-4' />
                  {t('systemSettings.actions.clearThisModelPricing')}
                </Button>
              </div>
            )}
          </CardContent>
        </Card>
        {isSaving && (
          <div className='bg-background/40 absolute inset-0 z-10 flex items-center justify-center rounded-lg'>
            <Loader2 className='text-muted-foreground size-8 animate-spin' />
          </div>
        )}
      </div>
    </div>
  )
}
