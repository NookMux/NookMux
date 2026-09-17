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
  type ReactNode,
  useEffect,
  useState,
  useMemo,
  useCallback,
  useRef,
} from 'react'
import { useForm } from 'react-hook-form'
import { zodResolver } from '@hookform/resolvers/zod'
import { useQuery, useQueryClient } from '@tanstack/react-query'
import {
  ArrowRight,
  ChevronDown,
  HelpCircle,
  Loader2,
  Sparkles,
  Trash2,
  Copy,
  FileText,
  Eraser,
  Plus,
  Eye,
  Link2,
  Code,
  Boxes,
  KeyRound,
  Route,
  Server,
  Settings,
  SlidersHorizontal,
  Wand2,
} from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { getLobeIcon } from '@/lib/lobe-icon'
import { cn } from '@/lib/utils'
import { useCopyToClipboard } from '@/hooks/use-copy-to-clipboard'
import { Alert, AlertDescription } from '@/components/ui/alert'
import { Button } from '@/components/ui/button'
import {
  Collapsible,
  CollapsibleContent,
  CollapsibleTrigger,
} from '@/components/ui/collapsible'
import { Combobox } from '@/components/ui/combobox'
import {
  Form,
  FormControl,
  FormDescription,
  FormField,
  FormItem,
  FormLabel,
  FormMessage,
} from '@/components/ui/form'
import { Input } from '@/components/ui/input'
import {
  Select,
  SelectContent,
  SelectGroup,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import {
  Sheet,
  SheetClose,
  SheetContent,
  SheetDescription,
  SheetFooter,
  SheetHeader,
  SheetTitle,
} from '@/components/ui/sheet'
import { Skeleton } from '@/components/ui/skeleton'
import { Switch } from '@/components/ui/switch'
import { Textarea } from '@/components/ui/textarea'
import {
  Tooltip,
  TooltipContent,
  TooltipTrigger,
} from '@/components/ui/tooltip'
import {
  sideDrawerContentClassName,
  sideDrawerFooterClassName,
  sideDrawerFormClassName,
  sideDrawerHeaderClassName,
  sideDrawerSectionClassName,
  sideDrawerSwitchItemClassName,
} from '@/components/drawer-layout'
import { JsonEditor } from '@/components/json-editor'
import { MultiSelect } from '@/components/multi-select'
import {
  SecureVerificationDialog,
  useSecureVerification,
} from '@/features/auth/secure-verification'
import {
  createChannel,
  fetchModels,
  fetchProviders,
  getAllModels,
  getChannel,
  getChannelKey,
  getGroups,
  getPrefillGroups,
  testProxy,
  updateChannel,
} from '../../api'
import {
  ADD_MODE_OPTIONS,
  CHANNEL_TYPE_OPTIONS,
  CHANNEL_TYPE_WARNINGS,
  ERROR_MESSAGES,
  FIELD_DESCRIPTIONS,
  FIELD_PLACEHOLDERS,
  MODEL_FETCHABLE_TYPES,
  SUCCESS_MESSAGES,
} from '../../constants'
import {
  CHANNEL_FORM_DEFAULT_VALUES,
  channelFormSchema,
  channelsQueryKeys,
  transformChannelToFormDefaults,
  transformFormDataToCreatePayload,
  transformFormDataToUpdatePayload,
  type ChannelFormValues,
  deduplicateKeys,
  getChannelTypeIcon,
  getKeyPromptForType,
  parseModelsString,
  formatModelsArray,
  extractRedirectModels,
  extractMappingSourceModels,
  hasModelConfigChanged,
  findMissingModelsInMapping,
  validateModelMappingJson,
} from '../../lib'
import {
  collectInvalidStatusCodeEntries,
  collectNewDisallowedStatusCodeRedirects,
} from '../../lib/status-code-risk-guard'
import type { Channel, ProxyTestResultData } from '../../types'
import { FetchModelsDialog } from '../dialogs/fetch-models-dialog'
import {
  MissingModelsConfirmationDialog,
  type MissingModelsAction,
} from '../dialogs/missing-models-confirmation-dialog'
import { ParamOverrideEditorDialog } from '../dialogs/param-override-editor-dialog'
import {
  ProviderPickerDialog,
  type ProviderPickerMode,
} from '../dialogs/provider-picker-dialog'
import { StatusCodeRiskDialog } from '../dialogs/status-code-risk-dialog'
import { ModelMappingEditor } from '../model-mapping-editor'

type ChannelMutateDrawerProps = {
  open: boolean
  onOpenChange: (open: boolean) => void
  currentRow?: Channel | null
}

type ModelMappingGuardrail = {
  invalidJson: boolean
  entries: Array<{ source: string; target: string }>
  missingSourceModels: string[]
  exposedTargetModels: string[]
}

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === 'object' && value !== null
}

function getErrorMessage(error: unknown): string | undefined {
  if (error instanceof Error && typeof error.message === 'string') {
    return error.message
  }

  if (!isRecord(error)) return undefined

  const response = error.response
  if (isRecord(response)) {
    const data = response.data
    if (isRecord(data)) {
      const message = data.message
      if (typeof message === 'string') return message
    }
  }

  const message = error.message
  if (typeof message === 'string') return message
  return undefined
}

// Helper functions
const createEmptyModelMappingGuardrail = (): ModelMappingGuardrail => ({
  invalidJson: false,
  entries: [],
  missingSourceModels: [],
  exposedTargetModels: [],
})

const formatModelNames = (models: string[]): string =>
  models.map((model) => `"${model}"`).join(', ')

const MODEL_MAPPING_PREVIEW_FALLBACK: Array<{
  source: string
  target: string
}> = [{ source: 'client-model', target: 'upstream-model' }]

const OPENAI_WIRE_API_CHANNEL_TYPES = new Set([1, 4, 6, 25, 26, 35, 44])

function CardHeading({ title, icon }: { title: string; icon?: ReactNode }) {
  return (
    <div className='flex items-center gap-3'>
      {icon && (
        <span className='bg-muted text-muted-foreground flex size-8 shrink-0 items-center justify-center rounded-md'>
          {icon}
        </span>
      )}
      <h3 className='text-sm font-semibold tracking-tight'>{title}</h3>
    </div>
  )
}

function SubHeading({ title, icon }: { title: string; icon?: ReactNode }) {
  return (
    <div className='flex items-center gap-2'>
      {icon && <span className='text-muted-foreground'>{icon}</span>}
      <h4 className='text-muted-foreground text-xs font-medium tracking-wide uppercase'>
        {title}
      </h4>
    </div>
  )
}

// OpenRouter quantization levels accepted by the provider routing object.
const OPENROUTER_QUANTIZATION_OPTIONS = [
  'int4',
  'int8',
  'fp4',
  'mxfp4',
  'nvfp4',
  'fp6',
  'fp8',
  'mxfp8',
  'fp16',
  'bf16',
  'fp32',
  'unknown',
].map((value) => ({ value, label: value }))

function parseOpenRouterSlugList(value: string): string[] {
  return value
    .split(',')
    .map((slug) => slug.trim())
    .filter(Boolean)
}

export function ChannelMutateDrawer({
  open,
  onOpenChange,
  currentRow,
}: ChannelMutateDrawerProps) {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const [isSubmitting, setIsSubmitting] = useState(false)
  const [customModel, setCustomModel] = useState('')
  const [fetchModelsDialogOpen, setFetchModelsDialogOpen] = useState(false)
  const [channelKey, setChannelKey] = useState<string | null>(null)
  const [isChannelKeyLoading, setIsChannelKeyLoading] = useState(false)
  const initialModelsRef = useRef<string[]>([])
  const initialModelMappingRef = useRef<string>('')
  const initialStatusCodeMappingRef = useRef<string>('')
  const [statusCodeRiskOpen, setStatusCodeRiskOpen] = useState(false)
  const [statusCodeRiskDetailItems, setStatusCodeRiskDetailItems] = useState<
    string[]
  >([])
  const statusCodeRiskResolveRef = useRef<
    ((confirmed: boolean) => void) | null
  >(null)
  const [missingModelsDialogOpen, setMissingModelsDialogOpen] = useState(false)
  const [missingModelsList, setMissingModelsList] = useState<string[]>([])
  const missingModelsResolveRef = useRef<
    ((action: MissingModelsAction) => void) | null
  >(null)
  const [paramOverrideEditorOpen, setParamOverrideEditorOpen] = useState(false)
  const [providerPickerField, setProviderPickerField] =
    useState<ProviderPickerMode | null>(null)
  const [proxyTestLoading, setProxyTestLoading] = useState(false)
  const [proxyTestResult, setProxyTestResult] =
    useState<ProxyTestResultData | null>(null)

  const isEditing = Boolean(currentRow)
  const channelId = currentRow?.id ?? null

  // Fetch channel details if editing
  const { data: channelData } = useQuery({
    queryKey: channelsQueryKeys.detail(currentRow?.id || 0),
    queryFn: () => getChannel(currentRow!.id),
    enabled: isEditing && Boolean(currentRow?.id),
  })

  // Fetch available groups
  const { data: groupsData, isLoading: isLoadingGroups } = useQuery({
    queryKey: ['groups'],
    queryFn: getGroups,
  })

  // Fetch all available models
  const { data: allModelsData } = useQuery({
    queryKey: ['channel_models'],
    queryFn: getAllModels,
  })

  // Fetch prefill model groups
  const { data: prefillGroupsData } = useQuery({
    queryKey: ['prefill_groups', 'model'],
    queryFn: () => getPrefillGroups('model'),
  })

  const { copyToClipboard } = useCopyToClipboard()

  const {
    open: verificationOpen,
    methods: verificationMethods,
    state: verificationState,
    executeVerification,
    withVerification,
    cancel: cancelVerification,
    setCode: setVerificationCode,
    switchMethod: switchVerificationMethod,
  } = useSecureVerification()

  useEffect(() => {
    if (!open) {
      setChannelKey(null)
      setIsChannelKeyLoading(false)
    } else if (channelId) {
      setChannelKey(null)
    }
  }, [open, channelId])

  // Check if this is a multi-key channel
  const isMultiKeyChannel =
    isEditing && channelData?.data?.channel_info?.is_multi_key === true

  // Form setup
  const form = useForm<ChannelFormValues>({
    resolver: zodResolver(channelFormSchema),
    defaultValues: CHANNEL_FORM_DEFAULT_VALUES,
  })

  // Watch form values for conditional rendering
  const multiKeyMode = form.watch('multi_key_mode')
  const multiKeyType = form.watch('multi_key_type')
  const keyMode = form.watch('key_mode')
  const currentGroups = form.watch('group')
  const currentType = form.watch('type')
  const currentBaseUrl = form.watch('base_url')
  const currentModels = form.watch('models')
  const currentName = form.watch('name')
  const currentModelMapping = form.watch('model_mapping')
  const awsKeyType = form.watch('aws_key_type')

  // Helper computed values
  const isBatchMode =
    multiKeyMode === 'batch' || multiKeyMode === 'multi_to_single'

  // Get all models list
  const allModelsList = useMemo(
    () => allModelsData?.data?.map((model) => model.id).filter(Boolean) || [],
    [allModelsData]
  )

  // Get basic models for the current channel type
  const basicModels = useMemo(() => {
    if (!allModelsList.length) return []
    // Filter models based on common patterns for specific types
    if (currentType === 1) {
      return allModelsList.filter(
        (model) => model.startsWith('gpt-') || model.startsWith('text-')
      )
    }
    return allModelsList
  }, [allModelsList, currentType])

  // Get prefill groups
  const prefillGroups = useMemo(
    () => prefillGroupsData?.data || [],
    [prefillGroupsData]
  )

  // Transform groups to multi-select options
  const groupOptions = useMemo(() => {
    if (!groupsData?.data) return []
    const allGroups = new Set([...groupsData.data, ...(currentGroups || [])])
    return Array.from(allGroups).map((group) => ({
      value: group,
      label: group,
    }))
  }, [groupsData, currentGroups])

  // Parse current models as array
  const currentModelsArray = useMemo(
    () => parseModelsString(currentModels),
    [currentModels]
  )

  const currentTypeLabel = useMemo(
    () =>
      CHANNEL_TYPE_OPTIONS.find((option) => option.value === currentType)
        ?.label || `#${currentType}`,
    [currentType]
  )

  const channelTypeOptions = useMemo(() => {
    const options = CHANNEL_TYPE_OPTIONS.map((option) => ({
      value: String(option.value),
      label: t(option.label),
      icon: getLobeIcon(`${getChannelTypeIcon(option.value)}.Color`, 16),
    }))
    if (
      isEditing &&
      !options.some((option) => Number(option.value) === currentType)
    ) {
      options.push({
        value: String(currentType),
        label: `#${currentType}`,
        icon: getLobeIcon(`${getChannelTypeIcon(currentType)}.Color`, 16),
      })
    }
    return options
  }, [currentType, isEditing, t])

  // Extract redirect models from model_mapping (target values)
  const redirectModelList = useMemo(
    () => extractRedirectModels(currentModelMapping || ''),
    [currentModelMapping]
  )

  // Extract source keys from model_mapping (models being remapped FROM)
  const redirectModelKeyList = useMemo(
    () => extractMappingSourceModels(currentModelMapping || ''),
    [currentModelMapping]
  )

  // Transform models to multi-select options
  const modelOptions = useMemo(() => {
    const allModels = new Set([...allModelsList, ...currentModelsArray])
    return Array.from(allModels).map((model) => ({
      value: model,
      label: model,
    }))
  }, [allModelsList, currentModelsArray])

  const modelMappingGuardrail = useMemo<ModelMappingGuardrail>(() => {
    if (!currentModelMapping?.trim()) {
      return createEmptyModelMappingGuardrail()
    }

    try {
      const parsed = JSON.parse(currentModelMapping)
      if (!parsed || typeof parsed !== 'object' || Array.isArray(parsed)) {
        return { ...createEmptyModelMappingGuardrail(), invalidJson: true }
      }

      const entries = Object.entries(parsed).reduce<
        Array<{ source: string; target: string }>
      >((acc, [rawSource, rawTarget]) => {
        const source = String(rawSource).trim()
        const target = String(rawTarget ?? '').trim()

        if (!source || !target) {
          return acc
        }

        acc.push({ source, target })
        return acc
      }, [])

      const missingSourceModels = Array.from(
        new Set(
          entries
            .filter(
              (entry) =>
                Boolean(entry.source) &&
                !currentModelsArray.includes(entry.source)
            )
            .map((entry) => entry.source)
        )
      )

      const exposedTargetModels = Array.from(
        new Set(
          entries
            .filter(
              (entry) =>
                Boolean(entry.target) &&
                currentModelsArray.includes(entry.target)
            )
            .map((entry) => entry.target)
        )
      )

      return {
        invalidJson: false,
        entries,
        missingSourceModels,
        exposedTargetModels,
      }
    } catch {
      return { ...createEmptyModelMappingGuardrail(), invalidJson: true }
    }
  }, [currentModelMapping, currentModelsArray])

  const mappingPreviewPairs =
    modelMappingGuardrail.entries.length > 0
      ? modelMappingGuardrail.entries.slice(0, 3)
      : MODEL_MAPPING_PREVIEW_FALLBACK
  const remainingMappingCount =
    modelMappingGuardrail.entries.length > 3
      ? modelMappingGuardrail.entries.length - 3
      : 0

  // Load channel data into form when editing
  useEffect(() => {
    if (isEditing && channelData?.data) {
      const defaults = transformChannelToFormDefaults(channelData.data)
      form.reset(defaults)
      // Store initial values for comparison
      initialModelsRef.current = parseModelsString(
        channelData.data.models || ''
      )
      initialModelMappingRef.current = channelData.data.model_mapping || ''
      initialStatusCodeMappingRef.current =
        channelData.data.status_code_mapping || ''
    } else if (!isEditing) {
      form.reset(CHANNEL_FORM_DEFAULT_VALUES)
      initialModelsRef.current = []
      initialModelMappingRef.current = ''
      initialStatusCodeMappingRef.current = ''
    }
  }, [isEditing, channelData, form])

  // Validate base_url - warn if it ends with /v1
  useEffect(() => {
    if (!currentBaseUrl || !currentBaseUrl.endsWith('/v1')) return

    // Show warning toast
    const timer = setTimeout(() => {
      toast.warning(t('channels.tips.warningBaseUrlShouldNotEndWithV1New'), {
        duration: 5000,
      })
    }, 500)

    return () => clearTimeout(timer)
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [currentBaseUrl])

  // Handle key deduplication
  const handleDeduplicateKeys = () => {
    const currentKey = form.getValues('key')
    if (!currentKey || currentKey.trim() === '') {
      toast.info(t('channels.errors.pleaseEnterKeysFirst'))
      return
    }

    const result = deduplicateKeys(currentKey)

    if (result.removedCount === 0) {
      toast.info(t('channels.fields.noDuplicateKeysFound'))
    } else {
      form.setValue('key', result.deduplicatedText)
      toast.success(
        t('channels.status.removedRemovedDuplicateKeySBeforeBeforeAfterAfter', {
          removed: result.removedCount,
          before: result.beforeCount,
          after: result.afterCount,
        })
      )
    }
  }

  const fetchChannelKey = useCallback(async () => {
    if (!channelId) {
      throw new Error('Channel is not selected')
    }

    setIsChannelKeyLoading(true)
    try {
      const res = await getChannelKey(channelId)
      if (!res.success) {
        throw new Error(
          res.message || t('channels.errors.failedToFetchChannelKey')
        )
      }

      const keyValue = res.data?.key ?? ''
      setChannelKey(keyValue)
      toast.success(t('channels.fields.channelKeyUnlocked'))
      return res
    } finally {
      setIsChannelKeyLoading(false)
    }
  }, [channelId, t])

  const handleRevealKey = useCallback(async () => {
    if (!channelId) return

    try {
      await withVerification(fetchChannelKey, {
        preferredMethod: 'passkey',
        title: 'Verify to view channel key',
        description:
          'Use Passkey or 2FA to confirm your identity before revealing this channel key.',
      })
    } catch (error) {
      if (error instanceof Error) {
        toast.error(error.message)
      }
    }
  }, [channelId, withVerification, fetchChannelKey])

  // Unified function to update models
  const updateModels = useCallback(
    (newModels: string[], merge: boolean = false) => {
      const finalModels = merge
        ? formatModelsArray([...currentModelsArray, ...newModels])
        : formatModelsArray(newModels)
      form.setValue('models', finalModels)
      return newModels.length
    },
    [currentModelsArray, form]
  )

  const handleTestProxy = useCallback(async () => {
    const proxyValue = (form.getValues('proxy') || '').trim()
    setProxyTestLoading(true)
    try {
      const response = await testProxy(proxyValue)
      if (response.success && response.data) {
        setProxyTestResult(response.data)
      } else {
        setProxyTestResult({
          status: 'failed',
          message: response.message || t('channels.errors.proxyTestFailed'),
        })
      }
    } catch (error) {
      const detail = getErrorMessage(error)
      setProxyTestResult({
        status: 'failed',
        message: detail || t('channels.errors.proxyTestFailed'),
      })
    } finally {
      setProxyTestLoading(false)
    }
  }, [form, t])

  // Handle fetching models from upstream
  const handleFetchModels = useCallback(async () => {
    const type = form.getValues('type')

    if (!MODEL_FETCHABLE_TYPES.has(type)) {
      toast.error(t('channels.tips.channelTypeDoesNotSupportFetchingModels'))
      return
    }

    // For creation mode, validate key before opening dialog
    if (!isEditing) {
      const key = form.getValues('key')
      if (!key?.trim()) {
        toast.error(t('channels.errors.pleaseEnterApiKeyFirst'))
        return
      }
    }

    setFetchModelsDialogOpen(true)
  }, [isEditing, form, t])

  const createModeFetcher = useCallback(async (): Promise<string[]> => {
    const response = await fetchModels({
      type: form.getValues('type'),
      key: form.getValues('key'),
      base_url: form.getValues('base_url') || '',
      aws_key_type: form.getValues('aws_key_type'),
      proxy: form.getValues('proxy') || '',
    })
    if (response.success && response.data) {
      return response.data.map((m) =>
        typeof m === 'string' ? m : String(m ?? '')
      )
    }
    throw new Error(response.message || 'No models fetched from upstream')
  }, [form])

  const createModeProviderFetcher = useCallback(async () => {
    const response = await fetchProviders({
      type: form.getValues('type'),
      base_url: form.getValues('base_url') || '',
    })
    if (response.success && response.data) {
      return response.data
    }
    throw new Error(
      response.message || t('channels.errors.fetchProvidersFailed')
    )
  }, [form, t])

  // Handle adding custom models
  const handleAddCustomModels = useCallback(() => {
    if (!customModel?.trim()) return

    const modelArray = parseModelsString(customModel)
    const count = updateModels(modelArray, true)
    setCustomModel('')
    toast.success(t('channels.fields.addedCountCustomModelS', { count }))
  }, [customModel, t, updateModels])

  // Handle model operations
  const handleFillRelatedModels = useCallback(() => {
    if (!basicModels.length) {
      toast.info(t('channels.tips.noRelatedModelsAvailableForThisChannelType'))
      return
    }
    updateModels(basicModels)
    toast.success(
      t('channels.tips.filledCountRelatedModelS', { count: basicModels.length })
    )
  }, [basicModels, updateModels, t])

  const handleFillAllModels = useCallback(() => {
    if (!allModelsList.length) {
      toast.info(t('channels.titles.noModelsAvailable'))
      return
    }
    updateModels(allModelsList)
    toast.success(
      t('channels.fields.filledCountModelS', { count: allModelsList.length })
    )
  }, [allModelsList, updateModels, t])

  const handleClearModels = useCallback(() => {
    form.setValue('models', '')
    toast.success(t('channels.titles.clearedAllModels'))
  }, [form, t])

  const handleCopyModels = useCallback(async () => {
    const models = form.getValues('models')
    if (!models?.trim()) {
      toast.info(t('channels.titles.noModelsToCopy'))
      return
    }
    await copyToClipboard(models)
  }, [form, copyToClipboard, t])

  // Handle adding prefill group models
  const handleAddPrefillGroup = useCallback(
    (group: { id: number; name: string; items: string | string[] }) => {
      try {
        const items = Array.isArray(group.items)
          ? group.items
          : JSON.parse(group.items)

        if (!Array.isArray(items)) {
          throw new Error('Invalid items format')
        }

        const count = updateModels(items, true)
        toast.success(
          t('common.tips.addedCountModelsFromName', {
            count,
            name: group.name,
          })
        )
      } catch {
        toast.error(t('channels.errors.failedToParseGroupItems'))
      }
    },
    [updateModels, t]
  )

  // Handle model selection change from MultiSelect
  const handleModelsChange = useCallback(
    (selected: string[]) => {
      form.setValue('models', selected.join(','))
    },
    [form]
  )

  // Handle successful submission
  const handleSuccess = useCallback(() => {
    queryClient.invalidateQueries({ queryKey: channelsQueryKeys.lists() })
    onOpenChange(false)
  }, [queryClient, onOpenChange])

  // Show missing models confirmation dialog
  const confirmMissingModelMappings = useCallback(
    (missingModels: string[]): Promise<MissingModelsAction> => {
      return new Promise((resolve) => {
        setMissingModelsList(missingModels)
        setMissingModelsDialogOpen(true)
        missingModelsResolveRef.current = resolve
      })
    },
    []
  )

  // Handle missing models dialog action
  const handleMissingModelsAction = useCallback(
    (action: MissingModelsAction) => {
      setMissingModelsDialogOpen(false)
      if (missingModelsResolveRef.current) {
        missingModelsResolveRef.current(action)
        missingModelsResolveRef.current = null
      }
    },
    []
  )

  const confirmStatusCodeRisk = useCallback(
    (detailItems: string[]): Promise<boolean> =>
      new Promise((resolve) => {
        statusCodeRiskResolveRef.current = resolve
        setStatusCodeRiskDetailItems(detailItems)
        setStatusCodeRiskOpen(true)
      }),
    []
  )

  const handleStatusCodeRiskAction = useCallback((confirmed: boolean) => {
    setStatusCodeRiskOpen(false)
    setStatusCodeRiskDetailItems([])
    if (statusCodeRiskResolveRef.current) {
      statusCodeRiskResolveRef.current(confirmed)
      statusCodeRiskResolveRef.current = null
    }
  }, [])

  useEffect(() => {
    return () => {
      if (statusCodeRiskResolveRef.current) {
        statusCodeRiskResolveRef.current(false)
        statusCodeRiskResolveRef.current = null
      }
    }
  }, [])

  // Submit handler
  const onSubmit = useCallback(
    async (data: ChannelFormValues) => {
      // Validate key is required when creating
      if (!isEditing && !data.key?.trim()) {
        form.setError('key', {
          type: 'manual',
          message: 'dashboard.fields.apiKeyRequired',
        })
        return
      }

      // Validate status_code_mapping entries
      if (data.status_code_mapping?.trim()) {
        const invalidEntries = collectInvalidStatusCodeEntries(
          data.status_code_mapping
        )
        if (invalidEntries.length > 0) {
          toast.error(
            t('channels.errors.invalidStatusCodeMappingEntriesEntries', {
              entries: invalidEntries.join(', '),
            })
          )
          return
        }

        const riskyRedirects = collectNewDisallowedStatusCodeRedirects(
          initialStatusCodeMappingRef.current,
          data.status_code_mapping
        )
        if (riskyRedirects.length > 0) {
          const confirmed = await confirmStatusCodeRisk(riskyRedirects)
          if (!confirmed) return
        }
      }

      // Validate model_mapping JSON format
      const hasModelMapping =
        typeof data.model_mapping === 'string' &&
        data.model_mapping.trim() !== ''

      if (hasModelMapping) {
        const validation = validateModelMappingJson(data.model_mapping!)
        if (!validation.valid) {
          toast.error(t(validation.error || 'Invalid model mapping'))
          return
        }
      }

      // Normalize models array
      const normalizedModels = parseModelsString(data.models || '')

      // Check for missing models in model_mapping
      if (hasModelMapping) {
        const missingModels = findMissingModelsInMapping(
          data.model_mapping!,
          normalizedModels
        )

        const shouldPromptMissing =
          missingModels.length > 0 &&
          hasModelConfigChanged(
            normalizedModels,
            data.model_mapping || '',
            initialModelsRef.current,
            initialModelMappingRef.current
          )

        if (shouldPromptMissing) {
          const confirmAction = await confirmMissingModelMappings(missingModels)
          if (confirmAction === 'cancel') {
            return
          }
          if (confirmAction === 'add') {
            const updatedModels = Array.from(
              new Set([...normalizedModels, ...missingModels])
            )
            data.models = formatModelsArray(updatedModels)
            form.setValue('models', data.models)
          }
        }
      }

      setIsSubmitting(true)
      try {
        if (isEditing && currentRow) {
          // Update existing channel
          const payload = transformFormDataToUpdatePayload(
            data,
            currentRow.id,
            isMultiKeyChannel
          )
          const payloadWithKeyMode =
            isMultiKeyChannel && data.key_mode
              ? {
                  ...payload,
                  key_mode: data.key_mode,
                }
              : payload

          const response = await updateChannel(
            currentRow.id,
            payloadWithKeyMode
          )
          if (response.success) {
            toast.success(t(SUCCESS_MESSAGES.UPDATED))
            handleSuccess()
          }
        } else {
          // Create new channel(s)
          const payload = transformFormDataToCreatePayload(data)
          const response = await createChannel(payload)
          if (response.success) {
            toast.success(t(SUCCESS_MESSAGES.CREATED))
            handleSuccess()
          }
        }
      } catch (error: unknown) {
        toast.error(getErrorMessage(error) || t(ERROR_MESSAGES.CREATE_FAILED))
      } finally {
        setIsSubmitting(false)
      }
    },
    [
      isEditing,
      currentRow,
      isMultiKeyChannel,
      form,
      handleSuccess,
      confirmMissingModelMappings,
      confirmStatusCodeRisk,
      t,
    ]
  )

  // Handle drawer close
  const handleOpenChange = useCallback(
    (v: boolean) => {
      onOpenChange(v)
      if (!v) {
        form.reset(CHANNEL_FORM_DEFAULT_VALUES)
      }
    },
    [onOpenChange, form]
  )

  return (
    <>
      <Sheet open={open} onOpenChange={handleOpenChange}>
        <SheetContent className={sideDrawerContentClassName('sm:max-w-3xl')}>
          <SheetHeader className={sideDrawerHeaderClassName()}>
            <SheetTitle className='flex items-center gap-3'>
              <span className='bg-muted flex size-9 shrink-0 items-center justify-center rounded-md'>
                {getLobeIcon(`${getChannelTypeIcon(currentType)}.Color`, 22)}
              </span>
              <span>
                {isEditing
                  ? t('channels.actions.editChannel')
                  : t('channels.actions.createChannel')}
                <span className='text-muted-foreground ml-2 text-sm font-normal'>
                  {t(currentTypeLabel)}
                </span>
              </span>
            </SheetTitle>
            <SheetDescription>
              {isEditing
                ? t(
                    'common.tips.updateChannelConfigurationAndClickSaveWhenYouRe'
                  )
                : t(
                    'channels.actions.addANewChannelByProvidingTheNecessaryInformation'
                  )}
            </SheetDescription>
          </SheetHeader>

          <Form {...form}>
            <form
              id='channel-form'
              onSubmit={form.handleSubmit(onSubmit)}
              className={sideDrawerFormClassName('gap-5')}
            >
              {/* ── Basic Information ── */}
              <div className={sideDrawerSectionClassName()}>
                <CardHeading
                  title={t('channels.titles.basicInformation')}
                  icon={<Server className='h-4 w-4' />}
                />
                <div className='grid gap-4 sm:grid-cols-2'>
                  <FormField
                    control={form.control}
                    name='name'
                    render={({ field }) => (
                      <FormItem>
                        <FormLabel>{t('channels.fields.named145bb')}</FormLabel>
                        <FormControl>
                          <Input
                            placeholder={t(FIELD_PLACEHOLDERS.NAME)}
                            {...field}
                          />
                        </FormControl>
                        <FormMessage />
                      </FormItem>
                    )}
                  />

                  <FormField
                    control={form.control}
                    name='type'
                    render={({ field }) => (
                      <FormItem>
                        <FormLabel>{t('channels.fields.type9a8e75')}</FormLabel>
                        <FormControl>
                          <Combobox
                            options={channelTypeOptions}
                            value={String(field.value)}
                            onValueChange={(value) => {
                              const nextType = Number(value)
                              if (Number.isInteger(nextType) && nextType > 0) {
                                field.onChange(nextType)
                              }
                            }}
                            placeholder={t(
                              'channels.placeholders.selectChannelType'
                            )}
                            searchPlaceholder={t(
                              'channels.actions.searchChannelType'
                            )}
                            emptyText={t('channels.tips.noChannelTypeFound')}
                          />
                        </FormControl>
                        <FormMessage />
                      </FormItem>
                    )}
                  />
                </div>

                <FormField
                  control={form.control}
                  name='status'
                  render={({ field }) => (
                    <FormItem className={sideDrawerSwitchItemClassName()}>
                      <div className='flex flex-col gap-0.5'>
                        <FormLabel>{t('channels.status.enabled')}</FormLabel>
                        <FormDescription className='text-xs'>
                          {t('channels.actions.enableOrDisableThisChannel')}
                        </FormDescription>
                      </div>
                      <FormControl>
                        <Switch
                          checked={field.value === 1}
                          onCheckedChange={(checked) =>
                            field.onChange(checked ? 1 : 2)
                          }
                        />
                      </FormControl>
                    </FormItem>
                  )}
                />

                <FormField
                  control={form.control}
                  name='tag'
                  render={({ field }) => (
                    <FormItem>
                      <FormLabel>{t('channels.fields.tag')}</FormLabel>
                      <FormControl>
                        <Input
                          placeholder={t(FIELD_PLACEHOLDERS.TAG)}
                          {...field}
                        />
                      </FormControl>
                      <FormMessage />
                    </FormItem>
                  )}
                />

                <FormField
                  control={form.control}
                  name='remark'
                  render={({ field }) => (
                    <FormItem>
                      <FormLabel>{t('channels.fields.remark')}</FormLabel>
                      <FormControl>
                        <Textarea
                          placeholder={t(FIELD_PLACEHOLDERS.REMARK)}
                          rows={2}
                          {...field}
                        />
                      </FormControl>
                      <FormMessage />
                    </FormItem>
                  )}
                />

                {currentType === 1 && (
                  <FormField
                    control={form.control}
                    name='openai_organization'
                    render={({ field }) => (
                      <FormItem>
                        <FormLabel>
                          {t('channels.fields.openAiOrganization')}
                        </FormLabel>
                        <FormControl>
                          <Input
                            placeholder={t('channels.placeholders.org')}
                            {...field}
                          />
                        </FormControl>
                        <FormDescription>
                          {t(FIELD_DESCRIPTIONS.OPENAI_ORG)}
                        </FormDescription>
                        <FormMessage />
                      </FormItem>
                    )}
                  />
                )}
              </div>

              {/* ── API Access ── */}
              <div className={sideDrawerSectionClassName()}>
                <CardHeading
                  title={t('channels.fields.apiAccess')}
                  icon={<Link2 className='h-4 w-4' />}
                />
                {CHANNEL_TYPE_WARNINGS[currentType] && (
                  <Alert>
                    <AlertDescription>
                      {t(CHANNEL_TYPE_WARNINGS[currentType])}
                    </AlertDescription>
                  </Alert>
                )}

                {/* Azure (type 3) */}
                {currentType === 3 && (
                  <>
                    <FormField
                      control={form.control}
                      name='base_url'
                      render={({ field }) => (
                        <FormItem>
                          <FormLabel>
                            {t('channels.fields.azureOpenaiEndpoint')}
                          </FormLabel>
                          <FormControl>
                            <Input
                              placeholder={t(
                                'channels.placeholders.eGUrlDocsTest001OpenaiAzureCom'
                              )}
                              {...field}
                            />
                          </FormControl>
                          <FormDescription>
                            {t('channels.fields.azureOpenAiEndpointUrl')}
                          </FormDescription>
                          <FormMessage />
                        </FormItem>
                      )}
                    />
                    <FormField
                      control={form.control}
                      name='other'
                      render={({ field }) => (
                        <FormItem>
                          <FormLabel>
                            {t('channels.fields.defaultApiVersion')}
                          </FormLabel>
                          <FormControl>
                            <Input
                              placeholder={t(
                                'channels.placeholders.eG20250401Preview'
                              )}
                              {...field}
                            />
                          </FormControl>
                          <FormDescription>
                            {t('channels.tips.defaultApiVersionForThisChannel')}
                          </FormDescription>
                          <FormMessage />
                        </FormItem>
                      )}
                    />
                    <FormField
                      control={form.control}
                      name='azure_responses_version'
                      render={({ field }) => (
                        <FormItem>
                          <FormLabel>
                            {t('channels.fields.responsesApiVersion')}
                          </FormLabel>
                          <FormControl>
                            <Input
                              placeholder={t('channels.placeholders.eGPreview')}
                              {...field}
                            />
                          </FormControl>
                          <FormDescription>
                            {t(
                              'channels.tips.defaultResponsesApiVersionIfEmptyWillUseThe'
                            )}
                          </FormDescription>
                          <FormMessage />
                        </FormItem>
                      )}
                    />
                  </>
                )}

                {/* Custom (type 8) */}
                {currentType === 8 && (
                  <FormField
                    control={form.control}
                    name='base_url'
                    render={({ field }) => (
                      <FormItem>
                        <FormLabel>
                          {t('channels.tips.fullBaseUrlSupports')} {'{'}
                          model
                          {'}'} {t('channels.fields.variable6a61f4')}
                        </FormLabel>
                        <FormControl>
                          <Input
                            placeholder={t(
                              'channels.placeholders.eGUrlApiOpenaiComV1ChatCompletions'
                            )}
                            {...field}
                          />
                        </FormControl>
                        <FormDescription>
                          {t(
                            'channels.placeholders.enterTheCompleteUrlSupports'
                          )}{' '}
                          {'{'}
                          model
                          {'}'} {t('channels.fields.variable')}
                        </FormDescription>
                        <FormMessage />
                      </FormItem>
                    )}
                  />
                )}

                {/* OpenRouter (type 20) */}
                {currentType === 20 && (
                  <FormField
                    control={form.control}
                    name='is_enterprise_account'
                    render={({ field }) => (
                      <FormItem className='flex items-center justify-between'>
                        <div className='space-y-0.5'>
                          <FormLabel>
                            {t('channels.placeholders.enterpriseAccount')}
                          </FormLabel>
                          <FormDescription>
                            {t(
                              'channels.actions.enableIfThisIsAnOpenRouterEnterpriseAccount'
                            )}
                          </FormDescription>
                        </div>
                        <FormControl>
                          <Switch
                            checked={field.value}
                            onCheckedChange={field.onChange}
                          />
                        </FormControl>
                      </FormItem>
                    )}
                  />
                )}

                {/* AWS (type 33) */}
                {currentType === 33 && (
                  <FormField
                    control={form.control}
                    name='aws_key_type'
                    render={({ field }) => (
                      <FormItem>
                        <FormLabel>
                          {t('channels.fields.awsKeyFormat')}
                        </FormLabel>
                        <Select
                          items={[
                            {
                              value: 'ak_sk',
                              label: t(
                                'channels.fields.accessKeySecretAccessKey'
                              ),
                            },
                            {
                              value: 'api_key',
                              label: t('channels.fields.apiKey'),
                            },
                          ]}
                          onValueChange={field.onChange}
                          value={field.value}
                        >
                          <FormControl>
                            <SelectTrigger>
                              <SelectValue
                                placeholder={t(
                                  'channels.placeholders.selectKeyFormat'
                                )}
                              />
                            </SelectTrigger>
                          </FormControl>
                          <SelectContent alignItemWithTrigger={false}>
                            <SelectGroup>
                              <SelectItem value='ak_sk'>
                                {t('channels.fields.accessKeySecretAccessKey')}
                              </SelectItem>
                              <SelectItem value='api_key'>
                                {t('channels.fields.apiKey')}
                              </SelectItem>
                            </SelectGroup>
                          </SelectContent>
                        </Select>
                        <FormDescription>
                          {field.value === 'api_key'
                            ? t('channels.fields.apiKeyModeUseApikeyRegion')
                            : t(
                                'channels.tips.akSkModeUseAccessKeySecretAccessKey'
                              )}
                        </FormDescription>
                        <FormMessage />
                      </FormItem>
                    )}
                  />
                )}

                {/* SiliconFlow (type 40) */}
                {currentType === 40 && (
                  <Alert>
                    <AlertDescription>
                      {t('channels.fields.referralLink')}{' '}
                      <a
                        href='https://cloud.siliconflow.cn/i/hij0YNTZ'
                        target='_blank'
                        rel='noopener noreferrer'
                        className='text-primary underline'
                      >
                        {t(
                          'channels.placeholders.urlCloudSiliconflowCnIHij0Yntz'
                        )}
                      </a>
                    </AlertDescription>
                  </Alert>
                )}

                {/* Vertex AI (type 41) */}
                {currentType === 41 && (
                  <>
                    <FormField
                      control={form.control}
                      name='vertex_key_type'
                      render={({ field }) => (
                        <FormItem>
                          <FormLabel>
                            {t('channels.fields.vertexAiKeyFormat')}
                          </FormLabel>
                          <Select
                            items={[
                              {
                                value: 'json',
                                label: t('channels.fields.json'),
                              },
                              {
                                value: 'api_key',
                                label: t('channels.fields.apiKey'),
                              },
                            ]}
                            onValueChange={field.onChange}
                            value={field.value}
                          >
                            <FormControl>
                              <SelectTrigger>
                                <SelectValue />
                              </SelectTrigger>
                            </FormControl>
                            <SelectContent alignItemWithTrigger={false}>
                              <SelectGroup>
                                <SelectItem value='json'>
                                  {t('channels.fields.json')}
                                </SelectItem>
                                <SelectItem value='api_key'>
                                  {t('channels.fields.apiKey')}
                                </SelectItem>
                              </SelectGroup>
                            </SelectContent>
                          </Select>
                          <FormDescription>
                            {field.value === 'json'
                              ? t(
                                  'channels.tips.jsonFormatSupportsServiceAccountJsonFiles'
                                )
                              : t(
                                  'channels.tips.apiKeyModeDoesNotSupportBatchCreation'
                                )}
                          </FormDescription>
                          <FormMessage />
                        </FormItem>
                      )}
                    />
                    {form.watch('vertex_key_type') === 'json' && (
                      <FormItem>
                        <FormLabel>
                          {t('channels.fields.serviceAccountJsonFileS')}
                        </FormLabel>
                        <FormControl>
                          <Input
                            type='file'
                            accept='.json,application/json'
                            multiple={isBatchMode}
                            onChange={async (e) => {
                              const fileList = e.target.files
                              const files = fileList ? Array.from(fileList) : []
                              // allow re-selecting the same file
                              e.target.value = ''

                              if (files.length === 0) {
                                toast.info(
                                  t('channels.fields.pleaseUploadKeyFileS')
                                )
                                return
                              }

                              const keys: unknown[] = []
                              for (const file of files) {
                                try {
                                  const txt = await file.text()
                                  keys.push(JSON.parse(txt))
                                } catch {
                                  toast.error(
                                    t(
                                      'channels.errors.failedToParseJsonFileName',
                                      {
                                        name: file.name,
                                      }
                                    )
                                  )
                                  return
                                }
                              }

                              if (keys.length === 0) {
                                toast.info(
                                  t('channels.fields.pleaseUploadKeyFileS')
                                )
                                return
                              }

                              const keyValue = isBatchMode
                                ? JSON.stringify(keys)
                                : JSON.stringify(keys[0])

                              form.setValue('key', keyValue, {
                                shouldDirty: true,
                                shouldValidate: true,
                              })

                              toast.success(
                                t(
                                  'channels.tips.parsedCountServiceAccountFileS',
                                  {
                                    count: keys.length,
                                  }
                                )
                              )
                            }}
                          />
                        </FormControl>
                        <FormDescription>
                          {isBatchMode
                            ? t(
                                'channels.actions.uploadMultipleJsonFilesInBatchModes'
                              )
                            : t(
                                'channels.actions.uploadASingleServiceAccountJsonFile'
                              )}
                        </FormDescription>
                        <FormMessage />
                      </FormItem>
                    )}
                    <FormField
                      control={form.control}
                      name='other'
                      render={({ field }) => (
                        <FormItem>
                          <FormLabel>
                            {t('channels.fields.deploymentRegion')}
                          </FormLabel>
                          <FormControl>
                            <Textarea
                              placeholder={t(
                                'channels.placeholders.eGUsCentral1OrJsonFormatForModel'
                              )}
                              rows={3}
                              {...field}
                            />
                          </FormControl>
                          <FormDescription>
                            {t(
                              'channels.placeholders.enterDeploymentRegionOrJsonMapping'
                            )}{' '}
                            {'{'}
                            {t(
                              'common.tips.defaultUsCentral1Claude35Sonnet20240620Europe'
                            )}
                            {'}'}
                          </FormDescription>
                          <FormMessage />
                        </FormItem>
                      )}
                    />
                  </>
                )}

                {/* General base_url for other types */}
                {![3, 8].includes(currentType) && (
                  <FormField
                    control={form.control}
                    name='base_url'
                    render={({ field }) => (
                      <FormItem>
                        <FormLabel>{t('channels.fields.baseUrl')}</FormLabel>
                        <FormControl>
                          <Input
                            placeholder={t(FIELD_PLACEHOLDERS.BASE_URL)}
                            {...field}
                          />
                        </FormControl>
                        <FormDescription>
                          {t(
                            'channels.tips.customApiBaseUrlForOfficialChannelsNewApi'
                          )}
                        </FormDescription>
                        <FormMessage />
                      </FormItem>
                    )}
                  />
                )}

                <div className='border-border/60 border-t pt-4'>
                  <SubHeading
                    title={t('layout.fields.authentication')}
                    icon={<KeyRound className='h-3.5 w-3.5' />}
                  />
                </div>
                {!isEditing && (
                  <FormField
                    control={form.control}
                    name='multi_key_mode'
                    render={({ field }) => (
                      <FormItem>
                        <FormLabel>{t('channels.actions.addMode')}</FormLabel>
                        <Select
                          items={[
                            ...ADD_MODE_OPTIONS.map((option) => ({
                              value: option.value,
                              label: t(option.label),
                            })),
                          ]}
                          onValueChange={field.onChange}
                          value={field.value}
                        >
                          <FormControl>
                            <SelectTrigger>
                              <SelectValue />
                            </SelectTrigger>
                          </FormControl>
                          <SelectContent alignItemWithTrigger={false}>
                            <SelectGroup>
                              {ADD_MODE_OPTIONS.map((option) => (
                                <SelectItem
                                  key={option.value}
                                  value={option.value}
                                >
                                  {t(option.label)}
                                </SelectItem>
                              ))}
                            </SelectGroup>
                          </SelectContent>
                        </Select>
                        <FormDescription>
                          {t(FIELD_DESCRIPTIONS.BATCH_ADD)}
                        </FormDescription>
                        <FormMessage />
                      </FormItem>
                    )}
                  />
                )}

                <FormField
                  control={form.control}
                  name='key'
                  render={({ field }) => {
                    const keyPlaceholder = (() => {
                      if (isEditing) {
                        return t('channels.tips.leaveEmptyToKeepExistingKey')
                      }
                      if (currentType === 33) {
                        if (awsKeyType === 'api_key') {
                          return isBatchMode
                            ? t(
                                'channels.placeholders.enterApiKeyOnePerLineFormatApikeyRegion'
                              )
                            : t(
                                'channels.placeholders.enterApiKeyFormatApikeyRegion'
                              )
                        }
                        return isBatchMode
                          ? t(
                              'channels.placeholders.enterKeyOnePerLineFormatAccessKeySecret'
                            )
                          : t(
                              'channels.placeholders.enterKeyFormatAccessKeySecretAccessKeyRegion'
                            )
                      }
                      if (isBatchMode) {
                        return t(
                          'channels.placeholders.enterOneKeyPerLineForBatchCreation'
                        )
                      }
                      return t(getKeyPromptForType(currentType))
                    })()
                    return (
                      <FormItem>
                        <FormLabel>
                          {t('channels.fields.apiKey2019bd')}
                        </FormLabel>
                        <FormControl>
                          <Textarea
                            placeholder={keyPlaceholder}
                            rows={isBatchMode ? 8 : 4}
                            {...field}
                          />
                        </FormControl>
                        <FormDescription>
                          <div className='flex flex-col gap-2'>
                            <span>
                              {isEditing ? (
                                <>
                                  {t(
                                    'channels.placeholders.enterNewKeyToUpdateOrLeaveEmptyTo'
                                  )}
                                  {isMultiKeyChannel && (
                                    <span className='text-warning mt-1 block'>
                                      {t(
                                        'channels.fields.multiKeyChannelKeysWillBe'
                                      )}{' '}
                                      {keyMode === 'replace'
                                        ? t('channels.fields.replaced')
                                        : t('channels.fields.appended')}
                                    </span>
                                  )}
                                </>
                              ) : isBatchMode ? (
                                t(
                                  'channels.placeholders.enterOneApiKeyPerLineForBatchCreation'
                                )
                              ) : (
                                t(FIELD_DESCRIPTIONS.KEY)
                              )}
                            </span>
                            {isBatchMode && (
                              <Button
                                type='button'
                                variant='outline'
                                size='sm'
                                onClick={handleDeduplicateKeys}
                                className='w-fit'
                              >
                                <Trash2 className='mr-2 h-4 w-4' />
                                {t('channels.actions.removeDuplicates')}
                              </Button>
                            )}
                          </div>
                        </FormDescription>
                        {isEditing && (
                          <div className='border-border/60 mt-4 flex flex-col gap-3 border-y border-dashed py-4'>
                            <div className='flex flex-col gap-2 sm:flex-row sm:items-center sm:justify-between'>
                              <div>
                                <p className='text-sm font-medium'>
                                  {t('channels.fields.currentKey')}
                                </p>
                                <p className='text-muted-foreground text-xs'>
                                  {t(
                                    'channels.status.verificationRequiredToRevealTheSavedKey'
                                  )}
                                </p>
                              </div>
                              <div className='flex items-center gap-2'>
                                <Button
                                  type='button'
                                  variant='outline'
                                  size='sm'
                                  onClick={handleRevealKey}
                                  disabled={
                                    isChannelKeyLoading ||
                                    verificationState.loading
                                  }
                                >
                                  {isChannelKeyLoading ||
                                  verificationState.loading ? (
                                    <Loader2 className='mr-2 h-4 w-4 animate-spin' />
                                  ) : (
                                    <Eye className='mr-2 h-4 w-4' />
                                  )}
                                  {t('channels.fields.revealKey')}
                                </Button>
                                <Button
                                  type='button'
                                  variant='ghost'
                                  size='sm'
                                  onClick={async () => {
                                    if (channelKey) {
                                      await copyToClipboard(channelKey)
                                    }
                                  }}
                                  disabled={!channelKey}
                                >
                                  <Copy className='mr-2 h-4 w-4' />
                                  {t('channels.actions.copy')}
                                </Button>
                              </div>
                            </div>
                            <Input
                              readOnly
                              value={channelKey ?? ''}
                              placeholder={t(
                                'channels.fields.hiddenVerifyToReveal'
                              )}
                              className='font-mono'
                            />
                          </div>
                        )}
                        <FormMessage />
                      </FormItem>
                    )
                  }}
                />

                {isEditing && isMultiKeyChannel && (
                  <FormField
                    control={form.control}
                    name='key_mode'
                    render={({ field }) => (
                      <FormItem>
                        <FormLabel>
                          {t('channels.fields.keyUpdateMode')}
                        </FormLabel>
                        <Select
                          items={[
                            {
                              value: 'append',
                              label: t('channels.fields.appendToExistingKeys'),
                            },
                            {
                              value: 'replace',
                              label: t(
                                'channels.fields.replaceAllExistingKeys'
                              ),
                            },
                          ]}
                          onValueChange={field.onChange}
                          value={field.value}
                        >
                          <FormControl>
                            <SelectTrigger>
                              <SelectValue />
                            </SelectTrigger>
                          </FormControl>
                          <SelectContent alignItemWithTrigger={false}>
                            <SelectGroup>
                              <SelectItem value='append'>
                                {t('channels.fields.appendToExistingKeys')}
                              </SelectItem>
                              <SelectItem value='replace'>
                                {t('channels.fields.replaceAllExistingKeys')}
                              </SelectItem>
                            </SelectGroup>
                          </SelectContent>
                        </Select>
                        <FormDescription>
                          {field.value === 'replace'
                            ? t(
                                'channels.tips.replaceModeWillCompletelyReplaceAllExistingKeys'
                              )
                            : t(
                                'channels.tips.appendModeNewKeysWillBeAddedToThe'
                              )}
                        </FormDescription>
                        <FormMessage />
                      </FormItem>
                    )}
                  />
                )}

                {((!isEditing && multiKeyMode === 'multi_to_single') ||
                  (isEditing && isMultiKeyChannel)) && (
                  <FormField
                    control={form.control}
                    name='multi_key_type'
                    render={({ field }) => (
                      <FormItem>
                        <FormLabel>
                          {t('channels.fields.multiKeyStrategy')}
                        </FormLabel>
                        <Select
                          items={[
                            {
                              value: 'random',
                              label: t('channels.fields.random'),
                            },
                            {
                              value: 'polling',
                              label: t('channels.fields.polling'),
                            },
                          ]}
                          onValueChange={field.onChange}
                          value={field.value}
                        >
                          <FormControl>
                            <SelectTrigger>
                              <SelectValue />
                            </SelectTrigger>
                          </FormControl>
                          <SelectContent alignItemWithTrigger={false}>
                            <SelectGroup>
                              <SelectItem value='random'>
                                {t('channels.fields.random')}
                              </SelectItem>
                              <SelectItem value='polling'>
                                {t('channels.fields.polling')}
                              </SelectItem>
                            </SelectGroup>
                          </SelectContent>
                        </Select>
                        <FormDescription>
                          {multiKeyType === 'polling' ? (
                            <span className='text-warning'>
                              {t(
                                'channels.tips.pollingModeRequiresRedisAndMemoryCacheOtherwisePerformance'
                              )}
                            </span>
                          ) : (
                            t(
                              'channels.tips.randomlySelectAKeyFromThePoolForEach'
                            )
                          )}
                        </FormDescription>
                        <FormMessage />
                      </FormItem>
                    )}
                  />
                )}
              </div>

              {/* ── Models & Groups ── */}
              <div className={sideDrawerSectionClassName()}>
                <CardHeading
                  title={t('channels.titles.modelsGroups')}
                  icon={<Boxes className='h-4 w-4' />}
                />
                <FormField
                  control={form.control}
                  name='models'
                  render={() => (
                    <FormItem>
                      <FormLabel>{t('channels.titles.models160bfa')}</FormLabel>
                      <FormControl>
                        <MultiSelect
                          options={modelOptions}
                          selected={currentModelsArray}
                          onChange={handleModelsChange}
                          placeholder={t(
                            'channels.placeholders.selectModelsOrAddCustomOnes'
                          )}
                        />
                      </FormControl>
                      <FormDescription>
                        <div className='flex flex-col gap-2'>
                          <span>{t(FIELD_DESCRIPTIONS.MODELS)}</span>
                          <div className='flex flex-wrap gap-2'>
                            <Button
                              type='button'
                              variant='outline'
                              size='sm'
                              onClick={handleFillRelatedModels}
                              disabled={!basicModels.length}
                            >
                              <FileText className='mr-2 h-4 w-4' />
                              {t('channels.actions.fillRelatedModels')}
                            </Button>
                            <Button
                              type='button'
                              variant='outline'
                              size='sm'
                              onClick={handleFillAllModels}
                              disabled={!allModelsList.length}
                            >
                              <Plus className='mr-2 h-4 w-4' />
                              {t('channels.actions.fillAllModels')}
                            </Button>
                            {MODEL_FETCHABLE_TYPES.has(currentType) && (
                              <Button
                                type='button'
                                variant='outline'
                                size='sm'
                                onClick={handleFetchModels}
                              >
                                <Sparkles className='mr-2 h-4 w-4' />
                                {t('channels.fields.fetchFromUpstream')}
                              </Button>
                            )}
                            <Button
                              type='button'
                              variant='outline'
                              size='sm'
                              onClick={handleClearModels}
                            >
                              <Eraser className='mr-2 h-4 w-4' />
                              {t('channels.actions.clearAll')}
                            </Button>
                            <Button
                              type='button'
                              variant='outline'
                              size='sm'
                              onClick={handleCopyModels}
                            >
                              <Copy className='mr-2 h-4 w-4' />
                              {t('channels.actions.copyAll')}
                            </Button>
                            {prefillGroups.map((group) => (
                              <Button
                                key={group.id}
                                type='button'
                                variant='secondary'
                                size='sm'
                                onClick={() => handleAddPrefillGroup(group)}
                              >
                                {group.name}
                              </Button>
                            ))}
                          </div>
                        </div>
                      </FormDescription>
                      {modelMappingGuardrail.exposedTargetModels.length > 0 && (
                        <Alert className='mt-3 border-amber-200 bg-amber-50 text-amber-900 dark:border-amber-500/40 dark:bg-amber-500/10 dark:text-amber-50'>
                          <AlertDescription>
                            {t('channels.fields.mappedUpstreamModelS')}{' '}
                            {formatModelNames(
                              modelMappingGuardrail.exposedTargetModels
                            )}{' '}
                            {t(
                              'common.tips.alsoListedHereRemoveThemFromModelsToKeep'
                            )}
                          </AlertDescription>
                        </Alert>
                      )}
                      <FormMessage />
                    </FormItem>
                  )}
                />

                {/* Custom Model Input */}
                <div className='flex gap-2'>
                  <Input
                    placeholder={t(
                      'channels.actions.addCustomModelSCommaSeparated'
                    )}
                    value={customModel}
                    onChange={(e) => setCustomModel(e.target.value)}
                    onKeyDown={(e) => {
                      if (e.key === 'Enter') {
                        e.preventDefault()
                        handleAddCustomModels()
                      }
                    }}
                  />
                  <Button
                    type='button'
                    variant='secondary'
                    onClick={handleAddCustomModels}
                    disabled={!customModel}
                  >
                    {t('channels.actions.add')}
                  </Button>
                </div>

                <FormField
                  control={form.control}
                  name='model_mapping'
                  render={({ field }) => (
                    <FormItem>
                      <div className='flex items-center gap-2'>
                        <FormLabel className='mb-0'>
                          {t('channels.fields.modelMapping')}
                        </FormLabel>
                        <Tooltip>
                          <TooltipTrigger
                            render={
                              <Button
                                type='button'
                                variant='ghost'
                                size='icon-sm'
                                className='text-muted-foreground hover:text-foreground size-auto p-0'
                                aria-label={t(
                                  'channels.fields.howModelMappingWorks'
                                )}
                              />
                            }
                          >
                            <HelpCircle className='h-4 w-4' />
                          </TooltipTrigger>
                          <TooltipContent
                            side='top'
                            align='start'
                            className='max-w-xs space-y-2 text-left'
                          >
                            <p className='text-xs font-semibold tracking-wide uppercase'>
                              {t('channels.fields.requestFlow')}
                            </p>
                            <div className='space-y-1 font-mono text-xs'>
                              {mappingPreviewPairs.map((pair) => (
                                <div
                                  key={`${pair.source}-${pair.target}`}
                                  className='flex items-center gap-1'
                                >
                                  <span>{pair.source}</span>
                                  <ArrowRight className='h-3.5 w-3.5 opacity-70' />
                                  <span>{pair.target}</span>
                                </div>
                              ))}
                              {remainingMappingCount > 0 && (
                                <div className='text-[11px] opacity-70'>
                                  +{remainingMappingCount}{' '}
                                  {t('channels.fields.moreMapping')}
                                  {remainingMappingCount > 1 ? 's' : ''}
                                </div>
                              )}
                            </div>
                            <p className='text-[11px] leading-relaxed opacity-80'>
                              {t(
                                'channels.tips.usersCallTheModelOnTheLeftThePlatform'
                              )}
                            </p>
                          </TooltipContent>
                        </Tooltip>
                      </div>
                      <FormControl>
                        <ModelMappingEditor
                          value={field.value || ''}
                          onChange={field.onChange}
                          disabled={isSubmitting}
                        />
                      </FormControl>
                      <FormDescription>
                        {t(FIELD_DESCRIPTIONS.MODEL_MAPPING)}
                      </FormDescription>
                      {modelMappingGuardrail.invalidJson && (
                        <Alert variant='destructive' className='mt-3'>
                          <AlertDescription>
                            {t(
                              'channels.errors.modelMappingMustBeAJsonObjectLike'
                            )}{' '}
                            <code className='font-mono'>
                              {'{"gpt-4":"Azure-GPT4"}'}
                            </code>
                            {t('channels.tips.pleaseFixTheJsonBeforeSaving')}
                          </AlertDescription>
                        </Alert>
                      )}
                      {modelMappingGuardrail.missingSourceModels.length > 0 && (
                        <Alert className='mt-3 border-amber-200 bg-amber-50 text-amber-900 dark:border-amber-500/40 dark:bg-amber-500/10 dark:text-amber-50'>
                          <AlertDescription>
                            {t('channels.actions.add')}{' '}
                            {formatModelNames(
                              modelMappingGuardrail.missingSourceModels
                            )}{' '}
                            {t(
                              'channels.tips.modelsListSoUsersCanUseThemBeforeThe'
                            )}
                          </AlertDescription>
                        </Alert>
                      )}
                      <FormMessage />
                    </FormItem>
                  )}
                />

                <FormField
                  control={form.control}
                  name='group'
                  render={({ field }) => (
                    <FormItem>
                      <FormLabel>{t('channels.fields.groups6c415a')}</FormLabel>
                      <FormControl>
                        {isLoadingGroups ? (
                          <Skeleton className='h-10 w-full' />
                        ) : (
                          <MultiSelect
                            options={groupOptions}
                            selected={field.value}
                            onChange={field.onChange}
                            placeholder={t(FIELD_PLACEHOLDERS.GROUP)}
                          />
                        )}
                      </FormControl>
                      <FormDescription>
                        {t(FIELD_DESCRIPTIONS.GROUP)}
                      </FormDescription>
                      <FormMessage />
                    </FormItem>
                  )}
                />
              </div>

              <div className='mt-5 flex flex-col gap-5'>
                {/* ── Routing & Overrides ── */}
                <div className={sideDrawerSectionClassName()}>
                  <CardHeading
                    title={t('channels.fields.routingOverrides')}
                    icon={<Route className='h-4 w-4' />}
                  />
                  <div className='flex flex-col gap-4'>
                    <SubHeading
                      title={t('channels.fields.routingStrategy')}
                      icon={<Route className='h-3.5 w-3.5' />}
                    />
                    <div className='grid gap-4 sm:grid-cols-2'>
                      <FormField
                        control={form.control}
                        name='priority'
                        render={({ field }) => (
                          <FormItem>
                            <FormLabel>
                              {t('channels.fields.priority')}
                            </FormLabel>
                            <FormControl>
                              <Input
                                type='number'
                                placeholder='0'
                                {...field}
                                onChange={(e) =>
                                  field.onChange(Number(e.target.value))
                                }
                              />
                            </FormControl>
                            <FormDescription>
                              {t(FIELD_DESCRIPTIONS.PRIORITY)}
                            </FormDescription>
                            <FormMessage />
                          </FormItem>
                        )}
                      />

                      <FormField
                        control={form.control}
                        name='weight'
                        render={({ field }) => (
                          <FormItem>
                            <FormLabel>{t('channels.fields.weight')}</FormLabel>
                            <FormControl>
                              <Input
                                type='number'
                                placeholder='0'
                                {...field}
                                onChange={(e) =>
                                  field.onChange(Number(e.target.value))
                                }
                              />
                            </FormControl>
                            <FormDescription>
                              {t(FIELD_DESCRIPTIONS.WEIGHT)}
                            </FormDescription>
                            <FormMessage />
                          </FormItem>
                        )}
                      />
                    </div>

                    <FormField
                      control={form.control}
                      name='test_model'
                      render={({ field }) => (
                        <FormItem>
                          <FormLabel>
                            {t('channels.fields.testModel')}
                          </FormLabel>
                          <FormControl>
                            <Input
                              placeholder={t(FIELD_PLACEHOLDERS.TEST_MODEL)}
                              {...field}
                            />
                          </FormControl>
                          <FormDescription>
                            {t(FIELD_DESCRIPTIONS.TEST_MODEL)}
                          </FormDescription>
                          <FormMessage />
                        </FormItem>
                      )}
                    />

                    <FormField
                      control={form.control}
                      name='auto_ban'
                      render={({ field }) => (
                        <FormItem className='flex items-center justify-between'>
                          <div className='space-y-0.5'>
                            <FormLabel>
                              {t('channels.fields.autoBan')}
                            </FormLabel>
                            <FormDescription>
                              {t(FIELD_DESCRIPTIONS.AUTO_BAN)}
                            </FormDescription>
                          </div>
                          <FormControl>
                            <Switch
                              checked={field.value === 1}
                              onCheckedChange={(checked) =>
                                field.onChange(checked ? 1 : 0)
                              }
                            />
                          </FormControl>
                        </FormItem>
                      )}
                    />
                  </div>

                  {/* ── Provider Strategy (OpenRouter only) ── */}
                  {currentType === 20 && (
                    <div className='flex flex-col gap-4 border-t pt-4'>
                      <SubHeading
                        title={t('channels.fields.providerStrategy')}
                        icon={<Boxes className='h-3.5 w-3.5' />}
                      />
                      <p className='text-muted-foreground text-xs'>
                        {t('channels.tips.providerStrategyDescription')}
                      </p>

                      <div className='grid gap-4 sm:grid-cols-2'>
                        <FormField
                          control={form.control}
                          name='or_allow_fallbacks'
                          render={({ field }) => (
                            <FormItem>
                              <FormLabel>
                                {t('channels.fields.orAllowFallbacks')}
                              </FormLabel>
                              <Select
                                value={field.value || ''}
                                onValueChange={field.onChange}
                              >
                                <FormControl>
                                  <SelectTrigger>
                                    <SelectValue
                                      placeholder={t(
                                        'channels.fields.orTriStateUnset'
                                      )}
                                    />
                                  </SelectTrigger>
                                </FormControl>
                                <SelectContent alignItemWithTrigger={false}>
                                  <SelectGroup>
                                    <SelectItem value=''>
                                      {t('channels.fields.orTriStateUnset')}
                                    </SelectItem>
                                    <SelectItem value='true'>
                                      {t('channels.fields.orTriStateAllow')}
                                    </SelectItem>
                                    <SelectItem value='false'>
                                      {t('channels.fields.orTriStateDeny')}
                                    </SelectItem>
                                  </SelectGroup>
                                </SelectContent>
                              </Select>
                              <FormDescription>
                                {t(FIELD_DESCRIPTIONS.OR_ALLOW_FALLBACKS)}
                              </FormDescription>
                              <FormMessage />
                            </FormItem>
                          )}
                        />

                        <FormField
                          control={form.control}
                          name='or_require_parameters'
                          render={({ field }) => (
                            <FormItem>
                              <FormLabel>
                                {t('channels.fields.orRequireParameters')}
                              </FormLabel>
                              <Select
                                value={field.value || ''}
                                onValueChange={field.onChange}
                              >
                                <FormControl>
                                  <SelectTrigger>
                                    <SelectValue
                                      placeholder={t(
                                        'channels.fields.orTriStateUnset'
                                      )}
                                    />
                                  </SelectTrigger>
                                </FormControl>
                                <SelectContent alignItemWithTrigger={false}>
                                  <SelectGroup>
                                    <SelectItem value=''>
                                      {t('channels.fields.orTriStateUnset')}
                                    </SelectItem>
                                    <SelectItem value='true'>
                                      {t('channels.fields.orTriStateRequireOn')}
                                    </SelectItem>
                                    <SelectItem value='false'>
                                      {t(
                                        'channels.fields.orTriStateRequireOff'
                                      )}
                                    </SelectItem>
                                  </SelectGroup>
                                </SelectContent>
                              </Select>
                              <FormDescription>
                                {t(FIELD_DESCRIPTIONS.OR_REQUIRE_PARAMETERS)}
                              </FormDescription>
                              <FormMessage />
                            </FormItem>
                          )}
                        />

                        <FormField
                          control={form.control}
                          name='or_data_collection'
                          render={({ field }) => (
                            <FormItem>
                              <FormLabel>
                                {t('channels.fields.orDataCollection')}
                              </FormLabel>
                              <Select
                                value={field.value || ''}
                                onValueChange={field.onChange}
                              >
                                <FormControl>
                                  <SelectTrigger>
                                    <SelectValue
                                      placeholder={t(
                                        'channels.fields.orTriStateUnset'
                                      )}
                                    />
                                  </SelectTrigger>
                                </FormControl>
                                <SelectContent alignItemWithTrigger={false}>
                                  <SelectGroup>
                                    <SelectItem value=''>
                                      {t('channels.fields.orTriStateUnset')}
                                    </SelectItem>
                                    <SelectItem value='deny'>
                                      {t(
                                        'channels.fields.orDataCollectionDeny'
                                      )}
                                    </SelectItem>
                                    <SelectItem value='allow'>
                                      {t(
                                        'channels.fields.orDataCollectionAllow'
                                      )}
                                    </SelectItem>
                                  </SelectGroup>
                                </SelectContent>
                              </Select>
                              <FormDescription>
                                {t(FIELD_DESCRIPTIONS.OR_DATA_COLLECTION)}
                              </FormDescription>
                              <FormMessage />
                            </FormItem>
                          )}
                        />

                        <FormField
                          control={form.control}
                          name='or_quantizations'
                          render={({ field }) => (
                            <FormItem>
                              <FormLabel>
                                {t('channels.fields.orQuantizations')}
                              </FormLabel>
                              <FormControl>
                                <MultiSelect
                                  options={OPENROUTER_QUANTIZATION_OPTIONS}
                                  selected={parseOpenRouterSlugList(
                                    field.value || ''
                                  )}
                                  onChange={(values) =>
                                    field.onChange(values.join(', '))
                                  }
                                  placeholder={t(
                                    'channels.placeholders.orQuantizations'
                                  )}
                                />
                              </FormControl>
                              <FormDescription>
                                {t(FIELD_DESCRIPTIONS.OR_QUANTIZATIONS)}
                              </FormDescription>
                              <FormMessage />
                            </FormItem>
                          )}
                        />
                      </div>

                      {[
                        {
                          name: 'or_order' as const,
                          mode: 'order' as const,
                          labelKey: 'channels.fields.orOrder',
                          description: 'OR_ORDER' as const,
                        },
                        {
                          name: 'or_only' as const,
                          mode: 'only' as const,
                          labelKey: 'channels.fields.orOnly',
                          description: 'OR_ONLY' as const,
                        },
                        {
                          name: 'or_ignore' as const,
                          mode: 'ignore' as const,
                          labelKey: 'channels.fields.orIgnore',
                          description: 'OR_IGNORE' as const,
                        },
                      ].map(({ name, mode, labelKey, description }) => (
                        <FormField
                          key={name}
                          control={form.control}
                          name={name}
                          render={({ field }) => (
                            <FormItem>
                              <FormLabel>{t(labelKey)}</FormLabel>
                              <div className='flex gap-2'>
                                <FormControl>
                                  <Input
                                    placeholder={t(
                                      'channels.placeholders.orProviderSlugs'
                                    )}
                                    {...field}
                                  />
                                </FormControl>
                                <Button
                                  type='button'
                                  variant='outline'
                                  onClick={() => setProviderPickerField(mode)}
                                >
                                  {t('channels.actions.pickProviders')}
                                </Button>
                              </div>
                              <FormDescription>
                                {t(FIELD_DESCRIPTIONS[description])}
                              </FormDescription>
                              <FormMessage />
                            </FormItem>
                          )}
                        />
                      ))}

                      <Collapsible>
                        <CollapsibleTrigger className='text-muted-foreground hover:text-foreground flex items-center gap-1 text-xs'>
                          <ChevronDown className='h-3.5 w-3.5' />
                          {t('channels.titles.orAdvancedPreferences')}
                        </CollapsibleTrigger>
                        <CollapsibleContent className='space-y-4 pt-4'>
                          <div className='grid gap-4 sm:grid-cols-2'>
                            <FormField
                              control={form.control}
                              name='or_sort'
                              render={({ field }) => (
                                <FormItem>
                                  <FormLabel>
                                    {t('channels.fields.orSort')}
                                  </FormLabel>
                                  <Select
                                    value={field.value || ''}
                                    onValueChange={field.onChange}
                                  >
                                    <FormControl>
                                      <SelectTrigger>
                                        <SelectValue
                                          placeholder={t(
                                            'channels.fields.orTriStateUnset'
                                          )}
                                        />
                                      </SelectTrigger>
                                    </FormControl>
                                    <SelectContent alignItemWithTrigger={false}>
                                      <SelectGroup>
                                        <SelectItem value=''>
                                          {t('channels.fields.orTriStateUnset')}
                                        </SelectItem>
                                        <SelectItem value='price'>
                                          {t('channels.fields.orSortPrice')}
                                        </SelectItem>
                                        <SelectItem value='throughput'>
                                          {t(
                                            'channels.fields.orSortThroughput'
                                          )}
                                        </SelectItem>
                                        <SelectItem value='latency'>
                                          {t('channels.fields.orSortLatency')}
                                        </SelectItem>
                                      </SelectGroup>
                                    </SelectContent>
                                  </Select>
                                  <FormMessage />
                                </FormItem>
                              )}
                            />

                            <FormField
                              control={form.control}
                              name='or_sort_partition'
                              render={({ field }) => (
                                <FormItem>
                                  <FormLabel>
                                    {t('channels.fields.orSortPartition')}
                                  </FormLabel>
                                  <Select
                                    value={field.value || ''}
                                    onValueChange={field.onChange}
                                    disabled={!form.watch('or_sort')}
                                  >
                                    <FormControl>
                                      <SelectTrigger>
                                        <SelectValue
                                          placeholder={t(
                                            'channels.fields.orSortPartitionModel'
                                          )}
                                        />
                                      </SelectTrigger>
                                    </FormControl>
                                    <SelectContent alignItemWithTrigger={false}>
                                      <SelectGroup>
                                        <SelectItem value='model'>
                                          {t(
                                            'channels.fields.orSortPartitionModel'
                                          )}
                                        </SelectItem>
                                        <SelectItem value='none'>
                                          {t(
                                            'channels.fields.orSortPartitionNone'
                                          )}
                                        </SelectItem>
                                      </SelectGroup>
                                    </SelectContent>
                                  </Select>
                                  <FormDescription>
                                    {t(FIELD_DESCRIPTIONS.OR_SORT)}
                                  </FormDescription>
                                  <FormMessage />
                                </FormItem>
                              )}
                            />

                            <FormField
                              control={form.control}
                              name='or_pref_min_throughput'
                              render={({ field }) => (
                                <FormItem>
                                  <FormLabel>
                                    {t('channels.fields.orPrefMinThroughput')}
                                  </FormLabel>
                                  <div className='flex gap-2'>
                                    <FormControl>
                                      <Input
                                        type='number'
                                        min='0'
                                        placeholder='50'
                                        {...field}
                                      />
                                    </FormControl>
                                    <FormField
                                      control={form.control}
                                      name='or_pref_min_throughput_percentile'
                                      render={({ field: percentileField }) => (
                                        <FormItem className='w-28 space-y-0'>
                                          <FormLabel className='sr-only'>
                                            {t('channels.fields.orPercentile')}
                                          </FormLabel>
                                          <FormControl>
                                            <Select
                                              value={
                                                percentileField.value || ''
                                              }
                                              onValueChange={
                                                percentileField.onChange
                                              }
                                            >
                                              <SelectTrigger>
                                                <SelectValue placeholder='p50' />
                                              </SelectTrigger>
                                              <SelectContent
                                                alignItemWithTrigger={false}
                                              >
                                                <SelectGroup>
                                                  {[
                                                    'p50',
                                                    'p75',
                                                    'p90',
                                                    'p99',
                                                  ].map((percentile) => (
                                                    <SelectItem
                                                      key={percentile}
                                                      value={percentile}
                                                    >
                                                      {percentile}
                                                    </SelectItem>
                                                  ))}
                                                </SelectGroup>
                                              </SelectContent>
                                            </Select>
                                          </FormControl>
                                        </FormItem>
                                      )}
                                    />
                                  </div>
                                  <FormDescription>
                                    {t(
                                      FIELD_DESCRIPTIONS.OR_PREF_MIN_THROUGHPUT
                                    )}
                                  </FormDescription>
                                  <FormMessage />
                                </FormItem>
                              )}
                            />

                            <FormField
                              control={form.control}
                              name='or_pref_max_latency'
                              render={({ field }) => (
                                <FormItem>
                                  <FormLabel>
                                    {t('channels.fields.orPrefMaxLatency')}
                                  </FormLabel>
                                  <div className='flex gap-2'>
                                    <FormControl>
                                      <Input
                                        type='number'
                                        min='0'
                                        step='0.1'
                                        placeholder='2.5'
                                        {...field}
                                      />
                                    </FormControl>
                                    <FormField
                                      control={form.control}
                                      name='or_pref_max_latency_percentile'
                                      render={({ field: percentileField }) => (
                                        <FormItem className='w-28 space-y-0'>
                                          <FormLabel className='sr-only'>
                                            {t('channels.fields.orPercentile')}
                                          </FormLabel>
                                          <FormControl>
                                            <Select
                                              value={
                                                percentileField.value || ''
                                              }
                                              onValueChange={
                                                percentileField.onChange
                                              }
                                            >
                                              <SelectTrigger>
                                                <SelectValue placeholder='p50' />
                                              </SelectTrigger>
                                              <SelectContent
                                                alignItemWithTrigger={false}
                                              >
                                                <SelectGroup>
                                                  {[
                                                    'p50',
                                                    'p75',
                                                    'p90',
                                                    'p99',
                                                  ].map((percentile) => (
                                                    <SelectItem
                                                      key={percentile}
                                                      value={percentile}
                                                    >
                                                      {percentile}
                                                    </SelectItem>
                                                  ))}
                                                </SelectGroup>
                                              </SelectContent>
                                            </Select>
                                          </FormControl>
                                        </FormItem>
                                      )}
                                    />
                                  </div>
                                  <FormDescription>
                                    {t(FIELD_DESCRIPTIONS.OR_PREF_MAX_LATENCY)}
                                  </FormDescription>
                                  <FormMessage />
                                </FormItem>
                              )}
                            />
                          </div>

                          <div className='grid gap-4 sm:grid-cols-2'>
                            {(
                              [
                                [
                                  'or_max_price_prompt',
                                  'channels.fields.orMaxPricePrompt',
                                ],
                                [
                                  'or_max_price_completion',
                                  'channels.fields.orMaxPriceCompletion',
                                ],
                                [
                                  'or_max_price_request',
                                  'channels.fields.orMaxPriceRequest',
                                ],
                                [
                                  'or_max_price_image',
                                  'channels.fields.orMaxPriceImage',
                                ],
                              ] as const
                            ).map(([name, labelKey]) => (
                              <FormField
                                key={name}
                                control={form.control}
                                name={name}
                                render={({ field }) => (
                                  <FormItem>
                                    <FormLabel>{t(labelKey)}</FormLabel>
                                    <FormControl>
                                      <Input
                                        type='number'
                                        min='0'
                                        step='0.01'
                                        placeholder='0'
                                        {...field}
                                      />
                                    </FormControl>
                                    <FormMessage />
                                  </FormItem>
                                )}
                              />
                            ))}
                          </div>

                          <div className='grid gap-4 sm:grid-cols-2'>
                            <FormField
                              control={form.control}
                              name='or_zdr'
                              render={({ field }) => (
                                <FormItem>
                                  <FormLabel>
                                    {t('channels.fields.orZdr')}
                                  </FormLabel>
                                  <Select
                                    value={field.value || ''}
                                    onValueChange={field.onChange}
                                  >
                                    <FormControl>
                                      <SelectTrigger>
                                        <SelectValue
                                          placeholder={t(
                                            'channels.fields.orTriStateUnset'
                                          )}
                                        />
                                      </SelectTrigger>
                                    </FormControl>
                                    <SelectContent alignItemWithTrigger={false}>
                                      <SelectGroup>
                                        <SelectItem value=''>
                                          {t('channels.fields.orTriStateUnset')}
                                        </SelectItem>
                                        <SelectItem value='true'>
                                          {t(
                                            'channels.fields.orTriStateRequireOn'
                                          )}
                                        </SelectItem>
                                        <SelectItem value='false'>
                                          {t(
                                            'channels.fields.orTriStateRequireOff'
                                          )}
                                        </SelectItem>
                                      </SelectGroup>
                                    </SelectContent>
                                  </Select>
                                  <FormMessage />
                                </FormItem>
                              )}
                            />

                            <FormField
                              control={form.control}
                              name='or_enforce_distillable_text'
                              render={({ field }) => (
                                <FormItem>
                                  <FormLabel>
                                    {t(
                                      'channels.fields.orEnforceDistillableText'
                                    )}
                                  </FormLabel>
                                  <Select
                                    value={field.value || ''}
                                    onValueChange={field.onChange}
                                  >
                                    <FormControl>
                                      <SelectTrigger>
                                        <SelectValue
                                          placeholder={t(
                                            'channels.fields.orTriStateUnset'
                                          )}
                                        />
                                      </SelectTrigger>
                                    </FormControl>
                                    <SelectContent alignItemWithTrigger={false}>
                                      <SelectGroup>
                                        <SelectItem value=''>
                                          {t('channels.fields.orTriStateUnset')}
                                        </SelectItem>
                                        <SelectItem value='true'>
                                          {t(
                                            'channels.fields.orTriStateRequireOn'
                                          )}
                                        </SelectItem>
                                        <SelectItem value='false'>
                                          {t(
                                            'channels.fields.orTriStateRequireOff'
                                          )}
                                        </SelectItem>
                                      </SelectGroup>
                                    </SelectContent>
                                  </Select>
                                  <FormMessage />
                                </FormItem>
                              )}
                            />
                          </div>
                        </CollapsibleContent>
                      </Collapsible>
                    </div>
                  )}

                  <div className='flex flex-col gap-4 border-t pt-4'>
                    <SubHeading
                      title={t('channels.fields.overrideRules')}
                      icon={<Code className='h-3.5 w-3.5' />}
                    />

                    <FormField
                      control={form.control}
                      name='status_code_mapping'
                      render={({ field }) => (
                        <FormItem className='space-y-3'>
                          <div className='space-y-1'>
                            <FormLabel>
                              {t('channels.fields.statusCodeMapping')}
                            </FormLabel>
                            <FormDescription>
                              {t(
                                'channels.tips.mapUpstreamStatusCodesToDifferentCodes'
                              )}
                            </FormDescription>
                          </div>
                          <FormControl>
                            <JsonEditor
                              value={field.value || ''}
                              onChange={field.onChange}
                              disabled={isSubmitting}
                              keyPlaceholder='400'
                              valuePlaceholder='500'
                              keyLabel='Original Code'
                              valueLabel='Mapped Code'
                              emptyMessage={t(
                                'channels.tips.noStatusCodeMappingsConfigured'
                              )}
                              template={{ '400': '500', '429': '503' }}
                              valueType='string'
                            />
                          </FormControl>
                          <FormMessage />
                        </FormItem>
                      )}
                    />

                    <FormField
                      control={form.control}
                      name='param_override'
                      render={({ field }) => (
                        <FormItem className='space-y-3 border-t pt-4'>
                          <div className='flex flex-col gap-2 sm:flex-row sm:items-start sm:justify-between'>
                            <div className='space-y-1'>
                              <FormLabel>
                                {t('channels.fields.parameterOverride')}
                              </FormLabel>
                              <FormDescription>
                                {t(
                                  'channels.errors.overrideRequestParametersCannotOverrideStreamParameter'
                                )}
                              </FormDescription>
                            </div>
                            <div className='flex flex-wrap gap-2'>
                              <Button
                                type='button'
                                variant='outline'
                                size='sm'
                                onClick={() => setParamOverrideEditorOpen(true)}
                              >
                                <Wand2 className='mr-2 h-4 w-4' />
                                {t('channels.fields.visualEdit')}
                              </Button>
                              <Button
                                type='button'
                                variant='outline'
                                size='sm'
                                onClick={() => {
                                  field.onChange(
                                    JSON.stringify(
                                      {
                                        operations: [
                                          {
                                            path: 'temperature',
                                            mode: 'set',
                                            value: 0.7,
                                            conditions: [
                                              {
                                                path: 'model',
                                                mode: 'prefix',
                                                value: 'gpt',
                                              },
                                            ],
                                            logic: 'AND',
                                          },
                                        ],
                                      },
                                      null,
                                      2
                                    )
                                  )
                                }}
                              >
                                <Code className='mr-2 h-4 w-4' />
                                {t('channels.fields.newFormatTemplate')}
                              </Button>
                              <Button
                                type='button'
                                variant='ghost'
                                size='sm'
                                onClick={() => field.onChange('')}
                              >
                                {t('common.actions.clear')}
                              </Button>
                            </div>
                          </div>
                          <FormControl>
                            <JsonEditor
                              value={field.value || ''}
                              onChange={field.onChange}
                              disabled={isSubmitting}
                              keyPlaceholder='temperature'
                              valuePlaceholder='0.7'
                              keyLabel='Parameter'
                              valueLabel='Value'
                              emptyMessage={t(
                                'channels.tips.noParameterOverridesConfigured'
                              )}
                              template={{
                                temperature: 0.7,
                                max_tokens: 2000,
                                top_p: 1,
                              }}
                              valueType='any'
                            />
                          </FormControl>
                          <FormMessage />
                        </FormItem>
                      )}
                    />

                    <FormField
                      control={form.control}
                      name='header_override'
                      render={({ field }) => (
                        <FormItem className='space-y-3 border-t pt-4'>
                          <div className='flex flex-col gap-2 sm:flex-row sm:items-start sm:justify-between'>
                            <div className='space-y-1'>
                              <FormLabel>
                                {t('channels.fields.requestHeaderOverride')}
                              </FormLabel>
                              <FormDescription>
                                {t('channels.fields.overrideRequestHeaders')}
                              </FormDescription>
                            </div>
                            <div className='flex flex-wrap gap-2'>
                              <Button
                                type='button'
                                variant='outline'
                                size='sm'
                                onClick={() =>
                                  field.onChange(
                                    JSON.stringify(
                                      {
                                        '*': true,
                                        're:^X-Trace-.*$': true,
                                        'X-Foo': '{client_header:X-Foo}',
                                        Authorization: 'Bearer {api_key}',
                                      },
                                      null,
                                      2
                                    )
                                  )
                                }
                              >
                                {t('common.actions.fillTemplate')}
                              </Button>
                              <Button
                                type='button'
                                variant='outline'
                                size='sm'
                                onClick={() =>
                                  field.onChange(
                                    JSON.stringify({ '*': true }, null, 2)
                                  )
                                }
                              >
                                {t('channels.fields.passthroughTemplate')}
                              </Button>
                              <Button
                                type='button'
                                variant='outline'
                                size='sm'
                                onClick={() => {
                                  try {
                                    const parsed = JSON.parse(
                                      field.value || '{}'
                                    )
                                    field.onChange(
                                      JSON.stringify(parsed, null, 2)
                                    )
                                  } catch (_e) {
                                    /* ignore invalid JSON */
                                  }
                                }}
                              >
                                {t('channels.fields.format')}
                              </Button>
                              <Button
                                type='button'
                                variant='ghost'
                                size='sm'
                                onClick={() => field.onChange('')}
                              >
                                {t('common.actions.clear')}
                              </Button>
                            </div>
                          </div>
                          <FormControl>
                            <Textarea
                              className='font-mono text-sm'
                              rows={6}
                              value={field.value || ''}
                              onChange={field.onChange}
                              disabled={isSubmitting}
                              placeholder={t(
                                'channels.placeholders.enterJsonToOverrideRequestHeaders'
                              )}
                            />
                          </FormControl>
                          <FormDescription className='text-xs'>
                            {t('channels.fields.supportedVariables')}:{' '}
                            <code className='bg-muted rounded px-1 py-0.5'>
                              {'{api_key}'}
                            </code>{' '}
                            — {t('channels.fields.channelKey')},{' '}
                            <code className='bg-muted rounded px-1 py-0.5'>
                              {'{client_header:NAME}'}
                            </code>{' '}
                            — {t('channels.fields.clientHeaderValue')}
                          </FormDescription>
                          <FormMessage />
                        </FormItem>
                      )}
                    />
                  </div>
                </div>

                {/* ── Extra Settings ── */}
                <div className={sideDrawerSectionClassName()}>
                  <CardHeading
                    title={t('channels.titles.channelExtraSettings')}
                    icon={<Settings className='h-4 w-4' />}
                  />
                  {(currentType === 1 || currentType === 14) && (
                    <div className='border-border/60 flex flex-col gap-3 border-y py-4'>
                      <SubHeading
                        title={t('channels.tips.fieldPassthroughControls')}
                        icon={<SlidersHorizontal className='h-3.5 w-3.5' />}
                      />

                      <div className='divide-border space-y-0 divide-y border-y'>
                        <FormField
                          control={form.control}
                          name='allow_service_tier'
                          render={({ field }) => (
                            <FormItem className='flex items-center justify-between gap-3 px-4 py-3'>
                              <div className='space-y-0.5'>
                                <FormLabel className='text-sm'>
                                  {t(
                                    'channels.fields.allowServiceTierPassthrough'
                                  )}
                                </FormLabel>
                                <FormDescription>
                                  {t(
                                    'channels.tips.passThroughTheServiceTierField'
                                  )}
                                </FormDescription>
                              </div>
                              <FormControl>
                                <Switch
                                  checked={field.value}
                                  onCheckedChange={field.onChange}
                                />
                              </FormControl>
                            </FormItem>
                          )}
                        />

                        {currentType === 1 && (
                          <>
                            <FormField
                              control={form.control}
                              name='disable_store'
                              render={({ field }) => (
                                <FormItem className='flex items-center justify-between gap-3 px-4 py-3'>
                                  <div className='space-y-0.5'>
                                    <FormLabel className='text-sm'>
                                      {t(
                                        'channels.actions.disableStorePassthrough'
                                      )}
                                    </FormLabel>
                                    <FormDescription>
                                      {t(
                                        'channels.status.enabledTheStoreFieldWillBeBlocked'
                                      )}
                                    </FormDescription>
                                  </div>
                                  <FormControl>
                                    <Switch
                                      checked={field.value}
                                      onCheckedChange={field.onChange}
                                    />
                                  </FormControl>
                                </FormItem>
                              )}
                            />

                            <FormField
                              control={form.control}
                              name='allow_safety_identifier'
                              render={({ field }) => (
                                <FormItem className='flex items-center justify-between gap-3 px-4 py-3'>
                                  <div className='space-y-0.5'>
                                    <FormLabel className='text-sm'>
                                      {t(
                                        'channels.tips.allowSafetyIdentifierPassthrough'
                                      )}
                                    </FormLabel>
                                    <FormDescription>
                                      {t(
                                        'channels.tips.passThroughTheSafetyIdentifierField'
                                      )}
                                    </FormDescription>
                                  </div>
                                  <FormControl>
                                    <Switch
                                      checked={field.value}
                                      onCheckedChange={field.onChange}
                                    />
                                  </FormControl>
                                </FormItem>
                              )}
                            />
                          </>
                        )}

                        {currentType === 14 && (
                          <>
                            <FormField
                              control={form.control}
                              name='allow_cache_control'
                              render={({ field }) => (
                                <FormItem className='flex items-center justify-between gap-3 px-4 py-3'>
                                  <div className='space-y-0.5'>
                                    <FormLabel className='text-sm'>
                                      {t(
                                        'channels.actions.allowCacheControlPassthrough'
                                      )}
                                    </FormLabel>
                                    <FormDescription>
                                      {t(
                                        'channels.actions.passThroughClaudeCacheControlFields'
                                      )}
                                    </FormDescription>
                                  </div>
                                  <FormControl>
                                    <Switch
                                      checked={field.value}
                                      onCheckedChange={field.onChange}
                                    />
                                  </FormControl>
                                </FormItem>
                              )}
                            />

                            <FormField
                              control={form.control}
                              name='allow_speed'
                              render={({ field }) => (
                                <FormItem className='flex items-center justify-between gap-3 px-4 py-3'>
                                  <div className='space-y-0.5'>
                                    <FormLabel className='text-sm'>
                                      {t(
                                        'channels.actions.allowSpeedPassthrough'
                                      )}
                                    </FormLabel>
                                    <FormDescription>
                                      {t(
                                        'channels.actions.passThroughClaudeSpeedField'
                                      )}
                                    </FormDescription>
                                  </div>
                                  <FormControl>
                                    <Switch
                                      checked={field.value}
                                      onCheckedChange={field.onChange}
                                    />
                                  </FormControl>
                                </FormItem>
                              )}
                            />

                            <FormField
                              control={form.control}
                              name='claude_beta_query'
                              render={({ field }) => (
                                <FormItem className='flex items-center justify-between gap-3 px-4 py-3'>
                                  <div className='space-y-0.5'>
                                    <FormLabel className='text-sm'>
                                      {t(
                                        'channels.tips.allowClaudeBetaQueryPassthrough'
                                      )}
                                    </FormLabel>
                                    <FormDescription>
                                      {t(
                                        'channels.tips.passThroughTheAnthropicBetaHeaderForBetaFeatures'
                                      )}
                                    </FormDescription>
                                  </div>
                                  <FormControl>
                                    <Switch
                                      checked={field.value}
                                      onCheckedChange={field.onChange}
                                    />
                                  </FormControl>
                                </FormItem>
                              )}
                            />
                          </>
                        )}
                      </div>
                    </div>
                  )}

                  <div className='divide-border space-y-0 divide-y border-y'>
                    {currentType === 1 && (
                      <FormField
                        control={form.control}
                        name='force_format'
                        render={({ field }) => (
                          <FormItem className='flex items-center justify-between px-4 py-3'>
                            <div className='space-y-0.5'>
                              <FormLabel>
                                {t('channels.fields.forceFormat')}
                              </FormLabel>
                              <FormDescription>
                                {t(
                                  'channels.tips.forceFormatResponseToOpenAiStandardOpenAi'
                                )}
                              </FormDescription>
                            </div>
                            <FormControl>
                              <Switch
                                checked={field.value}
                                onCheckedChange={field.onChange}
                              />
                            </FormControl>
                          </FormItem>
                        )}
                      />
                    )}

                    <FormField
                      control={form.control}
                      name='pass_through_body_enabled'
                      render={({ field }) => (
                        <FormItem className='flex items-center justify-between px-4 py-3'>
                          <div className='space-y-0.5'>
                            <FormLabel>
                              {t('channels.fields.passThroughBody')}
                            </FormLabel>
                            <FormDescription>
                              {t(
                                'channels.tips.passRequestBodyDirectlyToUpstream'
                              )}
                            </FormDescription>
                          </div>
                          <FormControl>
                            <Switch
                              checked={field.value}
                              onCheckedChange={field.onChange}
                            />
                          </FormControl>
                        </FormItem>
                      )}
                    />

                    <FormField
                      control={form.control}
                      name='pass_through_headers_enabled'
                      render={({ field }) => (
                        <FormItem className='flex items-center justify-between px-4 py-3'>
                          <div className='space-y-0.5'>
                            <FormLabel>
                              {t('channels.fields.passThroughHeaders')}
                            </FormLabel>
                            <FormDescription>
                              {t(
                                'channels.actions.passClientRequestHeadersUpstreamAndMergeThemWithHeader'
                              )}
                            </FormDescription>
                          </div>
                          <FormControl>
                            <Switch
                              checked={field.value !== false}
                              onCheckedChange={field.onChange}
                            />
                          </FormControl>
                        </FormItem>
                      )}
                    />
                  </div>

                  {OPENAI_WIRE_API_CHANNEL_TYPES.has(currentType) && (
                    <FormField
                      control={form.control}
                      name='openai_wire_api'
                      render={({ field }) => (
                        <FormItem>
                          <FormLabel>
                            {t('channels.fields.openAiWireApi')}
                          </FormLabel>
                          <Select
                            onValueChange={field.onChange}
                            value={field.value || 'both'}
                          >
                            <FormControl>
                              <SelectTrigger className='w-44'>
                                <SelectValue />
                              </SelectTrigger>
                            </FormControl>
                            <SelectContent alignItemWithTrigger={false}>
                              <SelectGroup>
                                <SelectItem value='both'>
                                  {t(
                                    'channels.tips.bothChatCompletionsAndResponses'
                                  )}
                                </SelectItem>
                                <SelectItem value='chat'>
                                  {t('channels.fields.chatCompletions')}
                                </SelectItem>
                                <SelectItem value='responses'>
                                  {t('channels.fields.responses')}
                                </SelectItem>
                              </SelectGroup>
                            </SelectContent>
                          </Select>
                          <FormDescription>
                            {t(
                              'channels.actions.selectTheUpstreamOpenAiWireFormatUsedByThis'
                            )}
                          </FormDescription>
                          <FormMessage />
                        </FormItem>
                      )}
                    />
                  )}

                  <FormField
                    control={form.control}
                    name='image_auto_convert_to_url_mode'
                    render={({ field }) => (
                      <FormItem>
                        <FormLabel>
                          {t('channels.fields.multimodalConversion')}
                        </FormLabel>
                        <Select
                          onValueChange={field.onChange}
                          value={field.value || 'off'}
                        >
                          <FormControl>
                            <SelectTrigger>
                              <SelectValue />
                            </SelectTrigger>
                          </FormControl>
                          <SelectContent alignItemWithTrigger={false}>
                            <SelectGroup>
                              <SelectItem value='off'>
                                {t('channels.status.disabled')}
                              </SelectItem>
                              <SelectItem value='mcp'>
                                {t('channels.fields.mcpUrlMode')}
                              </SelectItem>
                            </SelectGroup>
                          </SelectContent>
                        </Select>
                        <FormDescription>
                          {t(
                            'channels.tips.forTextOnlyUpstreamModelsAppendMediaUrlsToThe'
                          )}
                        </FormDescription>
                        <FormMessage />
                      </FormItem>
                    )}
                  />

                  <FormField
                    control={form.control}
                    name='proxy'
                    render={({ field }) => (
                      <FormItem>
                        <FormLabel>
                          {t('channels.fields.proxyAddress')}
                        </FormLabel>
                        <div className='flex items-start gap-[2%]'>
                          <FormControl>
                            <Input
                              className='min-w-0 flex-1'
                              placeholder={t(
                                'channels.fields.socks5UserPassHostPort'
                              )}
                              {...field}
                              onChange={(event) => {
                                field.onChange(event)
                                setProxyTestResult(null)
                              }}
                            />
                          </FormControl>
                          <Button
                            type='button'
                            variant='outline'
                            className='w-[18%] min-w-fit'
                            onClick={handleTestProxy}
                            disabled={proxyTestLoading}
                          >
                            {proxyTestLoading ? (
                              <Loader2 className='size-4 animate-spin' />
                            ) : (
                              t('channels.actions.testProxy')
                            )}
                          </Button>
                        </div>
                        {proxyTestResult && (
                          <p
                            className={cn(
                              'text-xs font-medium',
                              proxyTestResult.status === 'success' &&
                                'text-emerald-600 dark:text-emerald-400',
                              proxyTestResult.status === 'invalid' &&
                                'text-amber-600 dark:text-amber-400',
                              proxyTestResult.status === 'failed' &&
                                'text-red-600 dark:text-red-400'
                            )}
                          >
                            {proxyTestResult.status === 'success'
                              ? t('channels.status.proxyTestIpIs', {
                                  ip: proxyTestResult.ip,
                                })
                              : proxyTestResult.message}
                          </p>
                        )}
                        <FormDescription>
                          {t(
                            'channels.tips.networkProxyForThisChannelSupportsSocks5Protocol'
                          )}
                        </FormDescription>
                        <FormMessage />
                      </FormItem>
                    )}
                  />
                </div>
              </div>
            </form>
          </Form>

          <SheetFooter className={sideDrawerFooterClassName()}>
            <SheetClose
              render={<Button variant='outline' disabled={isSubmitting} />}
            >
              {t('common.actions.cancel')}
            </SheetClose>
            <Button form='channel-form' type='submit' disabled={isSubmitting}>
              {isSubmitting && (
                <Loader2 className='mr-2 h-4 w-4 animate-spin' />
              )}
              {isEditing
                ? t('channels.fields.updateChannel')
                : t('channels.actions.saveChanges')}
            </Button>
          </SheetFooter>
        </SheetContent>
      </Sheet>

      {paramOverrideEditorOpen && (
        <ParamOverrideEditorDialog
          open={paramOverrideEditorOpen}
          value={form.watch('param_override') || ''}
          onOpenChange={setParamOverrideEditorOpen}
          onSave={(nextValue) => {
            form.setValue('param_override', nextValue, {
              shouldDirty: true,
              shouldValidate: true,
            })
          }}
        />
      )}

      {/* Fetch Models Dialog */}
      <FetchModelsDialog
        open={fetchModelsDialogOpen}
        onOpenChange={setFetchModelsDialogOpen}
        onModelsSelected={(models) => {
          form.setValue('models', formatModelsArray(models))
        }}
        redirectModels={redirectModelList}
        redirectSourceModels={redirectModelKeyList}
        customFetcher={!isEditing ? createModeFetcher : undefined}
        channelName={!isEditing ? currentName?.trim() : undefined}
        existingModelsOverride={
          !isEditing
            ? parseModelsString(form.getValues('models') || '')
            : undefined
        }
      />

      <ProviderPickerDialog
        open={providerPickerField !== null}
        onOpenChange={(open) => {
          if (!open) setProviderPickerField(null)
        }}
        mode={providerPickerField ?? 'order'}
        value={
          form.watch(
            providerPickerField === 'only'
              ? 'or_only'
              : providerPickerField === 'ignore'
                ? 'or_ignore'
                : 'or_order'
          ) || ''
        }
        onConfirm={(value) => {
          if (providerPickerField === 'only') {
            form.setValue('or_only', value, { shouldDirty: true })
          } else if (providerPickerField === 'ignore') {
            form.setValue('or_ignore', value, { shouldDirty: true })
          } else {
            form.setValue('or_order', value, { shouldDirty: true })
          }
        }}
        customFetcher={!isEditing ? createModeProviderFetcher : undefined}
      />

      <SecureVerificationDialog
        open={verificationOpen}
        onOpenChange={(open) => {
          if (!open) {
            cancelVerification()
          }
        }}
        methods={verificationMethods}
        state={verificationState}
        onVerify={async (method, code) => {
          await executeVerification(method, code)
        }}
        onCancel={cancelVerification}
        onCodeChange={setVerificationCode}
        onMethodChange={switchVerificationMethod}
      />

      {/* Missing Models Confirmation Dialog */}
      <MissingModelsConfirmationDialog
        open={missingModelsDialogOpen}
        missingModels={missingModelsList}
        onConfirm={handleMissingModelsAction}
        onOpenChange={setMissingModelsDialogOpen}
      />

      <StatusCodeRiskDialog
        open={statusCodeRiskOpen}
        onOpenChange={(v) => {
          if (!v) handleStatusCodeRiskAction(false)
        }}
        detailItems={statusCodeRiskDetailItems}
        onConfirm={() => handleStatusCodeRiskAction(true)}
      />
    </>
  )
}
