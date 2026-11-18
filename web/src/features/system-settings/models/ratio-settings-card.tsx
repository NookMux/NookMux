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
import { useCallback, useEffect, useRef, useState } from 'react'
import * as z from 'zod'
import { useForm } from 'react-hook-form'
import { zodResolver } from '@hookform/resolvers/zod'
import { useMutation, useQueryClient } from '@tanstack/react-query'
import i18next from 'i18next'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs'
import { ConfirmDialog } from '@/components/confirm-dialog'
import {
  deleteOptionJsonMapEntry,
  resetModelRatios,
  resetToolBillingRules,
  upsertOptionJsonMapEntry,
} from '../api'
import { SettingsSection } from '../components/settings-section'
import { useUpdateOption } from '../hooks/use-update-option'
import { GroupRatioForm } from './group-ratio-form'
import { ModelRatioForm } from './model-ratio-form'
import { ToolPriceSettings } from './tool-price-settings'
import {
  formatJsonForTextarea,
  normalizeJsonString,
  validateJsonString,
} from './utils'

const modelSchema = z.object({
  ModelPrice: z.string().superRefine((value, ctx) => {
    const result = validateJsonString(value)
    if (!result.valid) {
      ctx.addIssue({
        code: z.ZodIssueCode.custom,
        message: result.message || 'common.errors.invalidJson',
      })
    }
  }),
  ModelRatio: z.string().superRefine((value, ctx) => {
    const result = validateJsonString(value)
    if (!result.valid) {
      ctx.addIssue({
        code: z.ZodIssueCode.custom,
        message: result.message || 'common.errors.invalidJson',
      })
    }
  }),
  CacheRatio: z.string().superRefine((value, ctx) => {
    const result = validateJsonString(value)
    if (!result.valid) {
      ctx.addIssue({
        code: z.ZodIssueCode.custom,
        message: result.message || 'common.errors.invalidJson',
      })
    }
  }),
  CreateCacheRatio: z.string().superRefine((value, ctx) => {
    const result = validateJsonString(value)
    if (!result.valid) {
      ctx.addIssue({
        code: z.ZodIssueCode.custom,
        message: result.message || 'common.errors.invalidJson',
      })
    }
  }),
  CompletionRatio: z.string().superRefine((value, ctx) => {
    const result = validateJsonString(value)
    if (!result.valid) {
      ctx.addIssue({
        code: z.ZodIssueCode.custom,
        message: result.message || 'common.errors.invalidJson',
      })
    }
  }),
  AudioRatio: z.string().superRefine((value, ctx) => {
    const result = validateJsonString(value)
    if (!result.valid) {
      ctx.addIssue({
        code: z.ZodIssueCode.custom,
        message: result.message || 'common.errors.invalidJson',
      })
    }
  }),
  AudioCompletionRatio: z.string().superRefine((value, ctx) => {
    const result = validateJsonString(value)
    if (!result.valid) {
      ctx.addIssue({
        code: z.ZodIssueCode.custom,
        message: result.message || 'common.errors.invalidJson',
      })
    }
  }),
  ContextPricing: z.string().superRefine((value, ctx) => {
    const result = validateJsonString(value)
    if (!result.valid) {
      ctx.addIssue({
        code: z.ZodIssueCode.custom,
        message: result.message || 'common.errors.invalidJson',
      })
    }
  }),
})

const groupSchema = z.object({
  GroupRatio: z.string().superRefine((value, ctx) => {
    const result = validateJsonString(value)
    if (!result.valid) {
      ctx.addIssue({
        code: z.ZodIssueCode.custom,
        message: result.message || 'common.errors.invalidJson',
      })
    }
  }),
  TopupGroupRatio: z.string().superRefine((value, ctx) => {
    const result = validateJsonString(value)
    if (!result.valid) {
      ctx.addIssue({
        code: z.ZodIssueCode.custom,
        message: result.message || 'common.errors.invalidJson',
      })
    }
  }),
  UserUsableGroups: z.string().superRefine((value, ctx) => {
    const result = validateJsonString(value)
    if (!result.valid) {
      ctx.addIssue({
        code: z.ZodIssueCode.custom,
        message: result.message || 'common.errors.invalidJson',
      })
    }
  }),
  GroupGroupRatio: z.string().superRefine((value, ctx) => {
    const result = validateJsonString(value)
    if (!result.valid) {
      ctx.addIssue({
        code: z.ZodIssueCode.custom,
        message: result.message || 'common.errors.invalidJson',
      })
    }
  }),
  AutoGroups: z.string().superRefine((value, ctx) => {
    const result = validateJsonString(value, {
      predicate: (parsed) =>
        Array.isArray(parsed) &&
        parsed.every((item) => typeof item === 'string'),
      predicateMessage: 'systemSettings.errors.expectedJsonArrayOfGroups',
    })
    if (!result.valid) {
      ctx.addIssue({
        code: z.ZodIssueCode.custom,
        message: result.message || 'common.errors.invalidJsonArray',
      })
    }
  }),
  DefaultUseAutoGroup: z.boolean(),
  GroupSpecialUsableGroup: z.string().superRefine((value, ctx) => {
    const result = validateJsonString(value)
    if (!result.valid) {
      ctx.addIssue({
        code: z.ZodIssueCode.custom,
        message: result.message || 'common.errors.invalidJson',
      })
    }
  }),
})

type ModelFormValues = z.infer<typeof modelSchema>
type GroupFormValues = z.infer<typeof groupSchema>
type RatioTabId = 'models' | 'groups' | 'tool-prices'

type ModelJsonMapField =
  | 'ModelPrice'
  | 'ModelRatio'
  | 'CacheRatio'
  | 'CreateCacheRatio'
  | 'CompletionRatio'
  | 'AudioRatio'
  | 'AudioCompletionRatio'
  | 'ContextPricing'

function parseJsonObject(value: string): Record<string, unknown> {
  const trimmed = value.trim()
  if (!trimmed) {
    return {}
  }
  const parsed = JSON.parse(trimmed)
  if (parsed == null || typeof parsed !== 'object' || Array.isArray(parsed)) {
    throw new Error(i18next.t('systemSettings.errors.expectedJsonObject'))
  }
  return parsed as Record<string, unknown>
}

function jsonValueEquals(left: unknown, right: unknown) {
  return JSON.stringify(left) === JSON.stringify(right)
}

async function updateJsonMapFieldByDiff(
  field: ModelJsonMapField,
  previousValue: string,
  nextValue: string
) {
  const previous = parseJsonObject(previousValue)
  const next = parseJsonObject(nextValue)
  const keys = new Set([...Object.keys(previous), ...Object.keys(next)])

  for (const mapKey of keys) {
    const hadPrevious = Object.prototype.hasOwnProperty.call(previous, mapKey)
    const hasNext = Object.prototype.hasOwnProperty.call(next, mapKey)
    if (!hasNext) {
      if (hadPrevious) {
        const data = await deleteOptionJsonMapEntry({
          key: field,
          map_key: mapKey,
        })
        if (!data.success) {
          throw new Error(
            data.message ||
              i18next.t('systemSettings.errors.failedToUpdateSetting')
          )
        }
      }
      continue
    }

    if (!hadPrevious || !jsonValueEquals(previous[mapKey], next[mapKey])) {
      const data = await upsertOptionJsonMapEntry({
        key: field,
        map_key: mapKey,
        value: JSON.stringify(next[mapKey]),
      })
      if (!data.success) {
        throw new Error(
          data.message ||
            i18next.t('systemSettings.errors.failedToUpdateSetting')
        )
      }
    }
  }
}

type RatioSettingsCardProps = {
  modelDefaults: ModelFormValues
  groupDefaults: GroupFormValues
  toolPricesDefault: string
  titleKey?: string
  visibleTabs?: RatioTabId[]
}

export function RatioSettingsCard({
  modelDefaults,
  groupDefaults,
  toolPricesDefault,
  titleKey = 'systemSettings.titles.pricingRatios',
  visibleTabs = ['models', 'groups', 'tool-prices'],
}: RatioSettingsCardProps) {
  const { t } = useTranslation()
  const updateOption = useUpdateOption()
  const queryClient = useQueryClient()
  const [confirmOpen, setConfirmOpen] = useState(false)
  const [toolBillingConfirmOpen, setToolBillingConfirmOpen] = useState(false)
  // 模型价格的保存走的是 upsertOptionJsonMapEntry，不会触发 updateOption mutation，
  // 因此需要一个独立的本地状态来准确反映保存进行中。
  const [isSavingModelRatios, setIsSavingModelRatios] = useState(false)

  const resetMutation = useMutation({
    mutationFn: resetModelRatios,
    onSuccess: (data) => {
      if (data.success) {
        toast.success(t('systemSettings.fields.modelPricesResetSuccessfully'))
        queryClient.invalidateQueries({ queryKey: ['system-options'] })
        setConfirmOpen(false)
      } else {
        toast.error(
          data.message || t('systemSettings.errors.failedToResetModelRatios')
        )
      }
    },
    onError: (error: Error) => {
      toast.error(
        error.message || t('systemSettings.errors.failedToResetModelRatios')
      )
    },
  })

  const resetToolBillingMutation = useMutation({
    mutationFn: resetToolBillingRules,
    onSuccess: (data) => {
      if (data.success) {
        toast.success(
          t('systemSettings.fields.toolBillingRulesResetSuccessfully')
        )
        queryClient.invalidateQueries({ queryKey: ['system-options'] })
        setToolBillingConfirmOpen(false)
      } else {
        toast.error(
          data.message ||
            t('systemSettings.errors.failedToResetToolBillingRules')
        )
      }
    },
    onError: (error: Error) => {
      toast.error(
        error.message ||
          t('systemSettings.errors.failedToResetToolBillingRules')
      )
    },
  })

  const modelNormalizedDefaults = useRef({
    ModelPrice: normalizeJsonString(modelDefaults.ModelPrice),
    ModelRatio: normalizeJsonString(modelDefaults.ModelRatio),
    CacheRatio: normalizeJsonString(modelDefaults.CacheRatio),
    CreateCacheRatio: normalizeJsonString(modelDefaults.CreateCacheRatio),
    CompletionRatio: normalizeJsonString(modelDefaults.CompletionRatio),
    AudioRatio: normalizeJsonString(modelDefaults.AudioRatio),
    AudioCompletionRatio: normalizeJsonString(
      modelDefaults.AudioCompletionRatio
    ),
    ContextPricing: normalizeJsonString(modelDefaults.ContextPricing),
  })

  const groupNormalizedDefaults = useRef({
    GroupRatio: normalizeJsonString(groupDefaults.GroupRatio),
    TopupGroupRatio: normalizeJsonString(groupDefaults.TopupGroupRatio),
    UserUsableGroups: normalizeJsonString(groupDefaults.UserUsableGroups),
    GroupGroupRatio: normalizeJsonString(groupDefaults.GroupGroupRatio),
    AutoGroups: normalizeJsonString(groupDefaults.AutoGroups),
    DefaultUseAutoGroup: groupDefaults.DefaultUseAutoGroup,
    GroupSpecialUsableGroup: normalizeJsonString(
      groupDefaults.GroupSpecialUsableGroup
    ),
  })

  const modelForm = useForm<ModelFormValues>({
    resolver: zodResolver(modelSchema),
    mode: 'onChange',
    defaultValues: {
      ...modelDefaults,
      ModelPrice: formatJsonForTextarea(modelDefaults.ModelPrice),
      ModelRatio: formatJsonForTextarea(modelDefaults.ModelRatio),
      CacheRatio: formatJsonForTextarea(modelDefaults.CacheRatio),
      CreateCacheRatio: formatJsonForTextarea(modelDefaults.CreateCacheRatio),
      CompletionRatio: formatJsonForTextarea(modelDefaults.CompletionRatio),
      AudioRatio: formatJsonForTextarea(modelDefaults.AudioRatio),
      AudioCompletionRatio: formatJsonForTextarea(
        modelDefaults.AudioCompletionRatio
      ),
      ContextPricing: formatJsonForTextarea(modelDefaults.ContextPricing),
    },
  })

  const groupForm = useForm<GroupFormValues>({
    resolver: zodResolver(groupSchema),
    mode: 'onChange',
    defaultValues: {
      ...groupDefaults,
      GroupRatio: formatJsonForTextarea(groupDefaults.GroupRatio),
      TopupGroupRatio: formatJsonForTextarea(groupDefaults.TopupGroupRatio),
      UserUsableGroups: formatJsonForTextarea(groupDefaults.UserUsableGroups),
      GroupGroupRatio: formatJsonForTextarea(groupDefaults.GroupGroupRatio),
      AutoGroups: formatJsonForTextarea(groupDefaults.AutoGroups),
      GroupSpecialUsableGroup: formatJsonForTextarea(
        groupDefaults.GroupSpecialUsableGroup
      ),
    },
  })

  useEffect(() => {
    modelNormalizedDefaults.current = {
      ModelPrice: normalizeJsonString(modelDefaults.ModelPrice),
      ModelRatio: normalizeJsonString(modelDefaults.ModelRatio),
      CacheRatio: normalizeJsonString(modelDefaults.CacheRatio),
      CreateCacheRatio: normalizeJsonString(modelDefaults.CreateCacheRatio),
      CompletionRatio: normalizeJsonString(modelDefaults.CompletionRatio),
      AudioRatio: normalizeJsonString(modelDefaults.AudioRatio),
      AudioCompletionRatio: normalizeJsonString(
        modelDefaults.AudioCompletionRatio
      ),
      ContextPricing: normalizeJsonString(modelDefaults.ContextPricing),
    }

    modelForm.reset({
      ...modelDefaults,
      ModelPrice: formatJsonForTextarea(modelDefaults.ModelPrice),
      ModelRatio: formatJsonForTextarea(modelDefaults.ModelRatio),
      CacheRatio: formatJsonForTextarea(modelDefaults.CacheRatio),
      CreateCacheRatio: formatJsonForTextarea(modelDefaults.CreateCacheRatio),
      CompletionRatio: formatJsonForTextarea(modelDefaults.CompletionRatio),
      AudioRatio: formatJsonForTextarea(modelDefaults.AudioRatio),
      AudioCompletionRatio: formatJsonForTextarea(
        modelDefaults.AudioCompletionRatio
      ),
      ContextPricing: formatJsonForTextarea(modelDefaults.ContextPricing),
    })
  }, [modelDefaults, modelForm])

  useEffect(() => {
    groupNormalizedDefaults.current = {
      GroupRatio: normalizeJsonString(groupDefaults.GroupRatio),
      TopupGroupRatio: normalizeJsonString(groupDefaults.TopupGroupRatio),
      UserUsableGroups: normalizeJsonString(groupDefaults.UserUsableGroups),
      GroupGroupRatio: normalizeJsonString(groupDefaults.GroupGroupRatio),
      AutoGroups: normalizeJsonString(groupDefaults.AutoGroups),
      DefaultUseAutoGroup: groupDefaults.DefaultUseAutoGroup,
      GroupSpecialUsableGroup: normalizeJsonString(
        groupDefaults.GroupSpecialUsableGroup
      ),
    }

    groupForm.reset({
      ...groupDefaults,
      GroupRatio: formatJsonForTextarea(groupDefaults.GroupRatio),
      TopupGroupRatio: formatJsonForTextarea(groupDefaults.TopupGroupRatio),
      UserUsableGroups: formatJsonForTextarea(groupDefaults.UserUsableGroups),
      GroupGroupRatio: formatJsonForTextarea(groupDefaults.GroupGroupRatio),
      AutoGroups: formatJsonForTextarea(groupDefaults.AutoGroups),
      GroupSpecialUsableGroup: formatJsonForTextarea(
        groupDefaults.GroupSpecialUsableGroup
      ),
    })
  }, [groupDefaults, groupForm])

  const saveModelRatios = useCallback(
    async (values: ModelFormValues) => {
      const normalized = {
        ModelPrice: normalizeJsonString(values.ModelPrice),
        ModelRatio: normalizeJsonString(values.ModelRatio),
        CacheRatio: normalizeJsonString(values.CacheRatio),
        CreateCacheRatio: normalizeJsonString(values.CreateCacheRatio),
        CompletionRatio: normalizeJsonString(values.CompletionRatio),
        AudioRatio: normalizeJsonString(values.AudioRatio),
        AudioCompletionRatio: normalizeJsonString(values.AudioCompletionRatio),
        ContextPricing: normalizeJsonString(values.ContextPricing),
      }

      const updates = (
        Object.keys(normalized) as Array<keyof ModelFormValues>
      ).filter(
        (key) => normalized[key] !== modelNormalizedDefaults.current[key]
      )

      if (updates.length === 0) {
        toast.info(t('systemSettings.errors.noModelPriceChangesToSave'))
        return
      }

      setIsSavingModelRatios(true)
      try {
        for (const key of updates) {
          await updateJsonMapFieldByDiff(
            key,
            modelNormalizedDefaults.current[key],
            normalized[key]
          )
        }
        modelNormalizedDefaults.current = normalized
        await queryClient.invalidateQueries({ queryKey: ['system-options'] })
        toast.success(t('channels.status.settingsUpdatedSuccessfully'))
      } catch (error) {
        toast.error(
          error instanceof Error
            ? error.message
            : t('systemSettings.errors.failedToSaveModelPrices')
        )
        throw error
      } finally {
        setIsSavingModelRatios(false)
      }
    },
    [queryClient, t]
  )

  const saveGroupRatios = useCallback(
    async (values: GroupFormValues) => {
      const normalized = {
        GroupRatio: normalizeJsonString(values.GroupRatio),
        TopupGroupRatio: normalizeJsonString(values.TopupGroupRatio),
        UserUsableGroups: normalizeJsonString(values.UserUsableGroups),
        GroupGroupRatio: normalizeJsonString(values.GroupGroupRatio),
        AutoGroups: normalizeJsonString(values.AutoGroups),
        DefaultUseAutoGroup: values.DefaultUseAutoGroup,
        GroupSpecialUsableGroup: normalizeJsonString(
          values.GroupSpecialUsableGroup
        ),
      }

      // Map form field names to API keys (most are 1:1, except GroupSpecialUsableGroup)
      const apiKeyMap: Record<string, string> = {
        GroupSpecialUsableGroup:
          'group_ratio_setting.group_special_usable_group',
      }

      const updates = (
        Object.keys(normalized) as Array<keyof typeof normalized>
      ).filter(
        (key) => normalized[key] !== groupNormalizedDefaults.current[key]
      )

      for (const key of updates) {
        const apiKey = apiKeyMap[key] || key
        await updateOption.mutateAsync({ key: apiKey, value: normalized[key] })
      }
    },
    [updateOption]
  )

  const handleResetRatios = useCallback(() => {
    setConfirmOpen(true)
  }, [])

  const { mutate: resetMutate } = resetMutation
  const handleConfirmReset = useCallback(() => {
    resetMutate()
  }, [resetMutate])

  const handleResetToolBillingRules = useCallback(() => {
    setToolBillingConfirmOpen(true)
  }, [])

  const { mutate: resetToolBillingMutate } = resetToolBillingMutation
  const handleConfirmResetToolBilling = useCallback(() => {
    resetToolBillingMutate()
  }, [resetToolBillingMutate])

  const tabLabels: Record<RatioTabId, string> = {
    models: 'common.fields.modelPrices',
    groups: 'systemSettings.fields.groupRatios',
    'tool-prices': 'common.fields.toolPrices',
  }
  const tabsGridClass =
    {
      1: 'grid-cols-1',
      2: 'grid-cols-2',
      3: 'grid-cols-3',
    }[visibleTabs.length] ?? 'grid-cols-3'
  const defaultTab = visibleTabs[0] ?? 'models'

  const renderTabContent = (tab: RatioTabId) => {
    if (tab === 'models') {
      return (
        <ModelRatioForm
          form={modelForm}
          onSave={saveModelRatios}
          onReset={handleResetRatios}
          isSaving={isSavingModelRatios}
          isResetting={resetMutation.isPending}
        />
      )
    }
    if (tab === 'groups') {
      return (
        <GroupRatioForm
          form={groupForm}
          onSave={saveGroupRatios}
          isSaving={updateOption.isPending}
        />
      )
    }
    if (tab === 'tool-prices') {
      return (
        <ToolPriceSettings
          defaultValue={toolPricesDefault}
          onReset={handleResetToolBillingRules}
          isResetting={resetToolBillingMutation.isPending}
        />
      )
    }
    return null
  }

  return (
    <SettingsSection title={t(titleKey)}>
      {visibleTabs.length === 1 ? (
        renderTabContent(defaultTab)
      ) : (
        <Tabs defaultValue={defaultTab} className='space-y-6'>
          <TabsList className={`grid w-full ${tabsGridClass}`}>
            {visibleTabs.map((tab) => (
              <TabsTrigger key={tab} value={tab}>
                {t(tabLabels[tab])}
              </TabsTrigger>
            ))}
          </TabsList>

          {visibleTabs.map((tab) => (
            <TabsContent key={tab} value={tab}>
              {renderTabContent(tab)}
            </TabsContent>
          ))}
        </Tabs>
      )}

      <ConfirmDialog
        open={confirmOpen}
        onOpenChange={setConfirmOpen}
        title={t('systemSettings.actions.resetAllModelPrices')}
        desc={t(
          'systemSettings.tips.clearCustomPricingRatiosAndRevertToUpstreamDefaults'
        )}
        destructive
        isLoading={resetMutation.isPending}
        handleConfirm={handleConfirmReset}
        confirmText={t('common.actions.reset')}
      />

      <ConfirmDialog
        open={toolBillingConfirmOpen}
        onOpenChange={setToolBillingConfirmOpen}
        title={t('systemSettings.actions.resetAllToolBillingRules')}
        desc={t('systemSettings.tips.clearToolBillingRulesAndRestoreDefaults')}
        destructive
        isLoading={resetToolBillingMutation.isPending}
        handleConfirm={handleConfirmResetToolBilling}
        confirmText={t('common.actions.reset')}
      />
    </SettingsSection>
  )
}
