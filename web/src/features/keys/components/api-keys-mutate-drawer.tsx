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
import { useEffect, useState } from 'react'
import { useForm, type SubmitErrorHandler } from 'react-hook-form'
import { zodResolver } from '@hookform/resolvers/zod'
import { useQuery } from '@tanstack/react-query'
import { KeyRound, Settings2, WalletCards } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { getUserModels, getUserGroups } from '@/lib/api'
import { getCurrencyDisplay, getCurrencyLabel } from '@/lib/currency'
import { useStatus } from '@/hooks/use-status'
import { Button } from '@/components/ui/button'
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
  Sheet,
  SheetClose,
  SheetContent,
  SheetDescription,
  SheetFooter,
  SheetHeader,
  SheetTitle,
} from '@/components/ui/sheet'
import { Switch } from '@/components/ui/switch'
import { Textarea } from '@/components/ui/textarea'
import { DateTimePicker } from '@/components/datetime-picker'
import {
  SideDrawerSection,
  SideDrawerSectionHeader,
  sideDrawerContentClassName,
  sideDrawerFooterClassName,
  sideDrawerFormClassName,
  sideDrawerHeaderClassName,
  sideDrawerSwitchItemClassName,
} from '@/components/drawer-layout'
import { MultiSelect } from '@/components/multi-select'
import { ModelMappingEditor } from '@/features/channels/components/model-mapping-editor'
import { createApiKey, updateApiKey, getApiKey } from '../api'
import { ERROR_MESSAGES, SUCCESS_MESSAGES } from '../constants'
import {
  getApiKeyFormSchema,
  type ApiKeyFormValues,
  getApiKeyFormDefaultValues,
  transformFormDataToPayload,
  transformApiKeyToFormDefaults,
} from '../lib'
import { type ApiKey } from '../types'
import {
  ApiKeyGroupCombobox,
  type ApiKeyGroupOption,
} from './api-key-group-combobox'
import { ApiKeyQuotaTypeCombobox } from './api-key-quota-type-combobox'
import { useApiKeys } from './api-keys-provider'

type ApiKeyMutateDrawerProps = {
  open: boolean
  onOpenChange: (open: boolean) => void
  currentRow?: ApiKey
}

export function ApiKeysMutateDrawer({
  open,
  onOpenChange,
  currentRow,
}: ApiKeyMutateDrawerProps) {
  const { t } = useTranslation()
  const isUpdate = !!currentRow
  const { setCreatedKeys, setOpen, triggerRefresh } = useApiKeys()
  const { status } = useStatus()
  const [isSubmitting, setIsSubmitting] = useState(false)
  const defaultUseAutoGroup = status?.default_use_auto_group === true

  // Fetch models
  const { data: modelsData } = useQuery({
    queryKey: ['user-models'],
    queryFn: getUserModels,
    staleTime: 5 * 60 * 1000, // Cache for 5 minutes
  })

  // Fetch groups
  const { data: groupsData } = useQuery({
    queryKey: ['user-groups'],
    queryFn: getUserGroups,
    staleTime: 5 * 60 * 1000,
  })

  const models = modelsData?.data || []
  const groupsRaw = groupsData?.data || {}
  const groups: ApiKeyGroupOption[] = Object.entries(groupsRaw).map(
    ([key, info]) => ({
      value: key,
      label: key,
      desc: info.desc || key,
      ratio: info.ratio,
    })
  )
  const backendHasAuto = groups.some((g) => g.value === 'auto')
  const schema = getApiKeyFormSchema(t)

  const form = useForm<ApiKeyFormValues>({
    resolver: zodResolver(schema),
    defaultValues: getApiKeyFormDefaultValues(defaultUseAutoGroup),
  })

  // Load existing data when updating
  useEffect(() => {
    if (open && isUpdate && currentRow) {
      getApiKey(currentRow.id)
        .then((result) => {
          if (result.success && result.data) {
            form.reset(transformApiKeyToFormDefaults(result.data))
          } else {
            toast.error(result.message || t(ERROR_MESSAGES.LOAD_FAILED))
          }
        })
        .catch(() => toast.error(t(ERROR_MESSAGES.UNEXPECTED)))
    } else if (open && !isUpdate) {
      form.reset(
        getApiKeyFormDefaultValues(defaultUseAutoGroup && backendHasAuto)
      )
    }
  }, [open, isUpdate, currentRow, form, defaultUseAutoGroup, backendHasAuto, t])

  // Correct group after groups load: if the form value is not in available groups, fall back
  useEffect(() => {
    if (groups.length === 0) return
    const currentGroup = form.getValues('group')
    if (currentGroup && !groups.some((g) => g.value === currentGroup)) {
      const fallback =
        groups.find((g) => g.value === 'default')?.value ??
        groups[0]?.value ??
        ''
      form.setValue('group', fallback)
      if (currentGroup === 'auto') {
        form.setValue('cross_group_retry', false)
      }
    }
  }, [groups, form])

  const onSubmit = async (data: ApiKeyFormValues) => {
    setIsSubmitting(true)
    try {
      const basePayload = transformFormDataToPayload(data)

      if (isUpdate && currentRow) {
        const result = await updateApiKey({
          ...basePayload,
          id: currentRow.id,
        })
        if (result.success) {
          toast.success(t(SUCCESS_MESSAGES.API_KEY_UPDATED))
          onOpenChange(false)
          triggerRefresh()
        } else {
          toast.error(result.message || t(ERROR_MESSAGES.UPDATE_FAILED))
        }
      } else {
        // Create mode - handle batch creation
        const count = data.tokenCount || 1
        let successCount = 0
        const newKeys: string[] = []

        for (let i = 0; i < count; i++) {
          const result = await createApiKey({
            ...basePayload,
            name:
              i === 0 && data.name
                ? data.name
                : `${data.name || 'default'}-${Math.random().toString(36).slice(2, 8)}`,
          })
          if (result.success && result.data?.key) {
            successCount++
            newKeys.push(`sk-${result.data.key}`)
          } else if (result.success) {
            toast.error(t('keys.status.createdApiKeyResponseDidNotIncludeAKey'))
            break
          } else {
            toast.error(result.message || t(ERROR_MESSAGES.CREATE_FAILED))
            break
          }
        }

        if (successCount > 0) {
          toast.success(
            t('keys.status.successfullyCreatedCountApiKeyS', {
              count: successCount,
            })
          )
          onOpenChange(false)
          triggerRefresh()
          setCreatedKeys(newKeys)
          setOpen('created-keys')
        }
      }
    } catch (_error) {
      toast.error(t(ERROR_MESSAGES.UNEXPECTED))
    } finally {
      setIsSubmitting(false)
    }
  }

  const onInvalid: SubmitErrorHandler<ApiKeyFormValues> = () => {
    toast.error(t('keys.tips.pleaseFixTheHighlightedFieldsBeforeSaving'))
  }

  const handleSetExpiry = (months: number, days: number, hours: number) => {
    if (months === 0 && days === 0 && hours === 0) {
      form.setValue('expired_time', undefined)
      return
    }

    const now = new Date()
    now.setMonth(now.getMonth() + months)
    now.setDate(now.getDate() + days)
    now.setHours(now.getHours() + hours)

    form.setValue('expired_time', now)
  }

  const { meta: currencyMeta } = getCurrencyDisplay()
  const currencyLabel = getCurrencyLabel()
  const tokensOnly = currencyMeta.kind === 'tokens'
  const quotaLabel = `${t('keys.fields.quota')} (${t(currencyLabel)})`
  const quotaPlaceholder = tokensOnly
    ? t('keys.placeholders.enterQuotaInTokens')
    : t('keys.placeholders.enterQuotaInCurrency', {
        currency: t(currencyLabel),
      })
  const quotaInputStep = tokensOnly ? 1 : 0.01
  const selectedGroup = form.watch('group')
  const quotaType = form.watch('quota_type')
  const quotaTypeOptions = [
    {
      value: 0,
      label: t('keys.fields.unlimitedQuota'),
      description: t('keys.tips.noQuotaCapUsageStillDependsOnAccountBalance'),
    },
    {
      value: 1,
      label: t('keys.fields.permanentQuota'),
      description: t('keys.tips.fixedQuotaThatRemainsAvailableUntilConsumed'),
    },
    {
      value: 2,
      label: t('keys.fields.hourlyResetQuota'),
      description: t('keys.tips.quotaResetsAfterEachConfiguredHourWindow'),
    },
    {
      value: 3,
      label: t('keys.fields.hourlyDaysResetQuota'),
      description: t('keys.tips.quotaResetsByHourWindowAndDayCycle'),
    },
  ]

  return (
    <Sheet
      open={open}
      onOpenChange={(v) => {
        onOpenChange(v)
        if (!v) {
          form.reset()
        }
      }}
    >
      <SheetContent
        className={sideDrawerContentClassName('max-w-none sm:!max-w-[620px]')}
      >
        <SheetHeader className={sideDrawerHeaderClassName()}>
          <SheetTitle>
            {isUpdate
              ? t('keys.fields.updateApiKey')
              : t('dashboard.actions.createApiKey')}
          </SheetTitle>
          <SheetDescription>
            {isUpdate
              ? t('keys.tips.updateTheApiKeyByProvidingNecessaryInfo')
              : t('keys.actions.addANewApiKeyByProvidingNecessaryInfo')}
          </SheetDescription>
        </SheetHeader>
        <Form {...form}>
          <form
            id='api-key-form'
            onSubmit={form.handleSubmit(onSubmit, onInvalid)}
            className={sideDrawerFormClassName('gap-5')}
          >
            <SideDrawerSection>
              <SideDrawerSectionHeader
                title={t('channels.titles.basicInformation')}
                description={t('keys.titles.setApiKeyBasicInformation')}
                icon={<KeyRound className='size-4' />}
              />
              <FormField
                control={form.control}
                name='name'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>{t('channels.fields.name')}</FormLabel>
                    <FormControl>
                      <Input
                        {...field}
                        placeholder={t('keys.placeholders.enterAName')}
                      />
                    </FormControl>
                    <FormMessage />
                  </FormItem>
                )}
              />

              <FormField
                control={form.control}
                name='group'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>{t('common.fields.group')}</FormLabel>
                    <FormControl>
                      <ApiKeyGroupCombobox
                        options={groups}
                        value={field.value}
                        onValueChange={field.onChange}
                        placeholder={t(
                          'dynamicRatio.placeholders.selectAGroup'
                        )}
                      />
                    </FormControl>
                    <FormMessage />
                  </FormItem>
                )}
              />

              {selectedGroup === 'auto' && (
                <FormField
                  control={form.control}
                  name='cross_group_retry'
                  render={({ field }) => (
                    <FormItem className={sideDrawerSwitchItemClassName()}>
                      <div className='flex flex-col gap-0.5'>
                        <FormLabel className='text-sm'>
                          {t('keys.fields.crossGroupRetry')}
                        </FormLabel>
                        <FormDescription className='line-clamp-2 text-xs sm:line-clamp-none'>
                          {t(
                            'keys.status.enabledIfChannelsInTheCurrentGroupFailIt'
                          )}
                        </FormDescription>
                      </div>
                      <FormControl>
                        <Switch
                          checked={!!field.value}
                          onCheckedChange={field.onChange}
                        />
                      </FormControl>
                    </FormItem>
                  )}
                />
              )}

              <FormField
                control={form.control}
                name='expired_time'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>{t('keyQuery.fields.expirationTime')}</FormLabel>
                    <div className='grid gap-2'>
                      <FormControl>
                        <DateTimePicker
                          value={field.value}
                          onChange={field.onChange}
                          placeholder={t('keys.fields.neverExpires')}
                          className='min-w-0 [&_input[type=time]]:w-24 sm:[&_input[type=time]]:w-32'
                        />
                      </FormControl>
                      <div className='grid grid-cols-2 gap-2 sm:flex sm:flex-wrap'>
                        <Button
                          type='button'
                          variant='outline'
                          size='sm'
                          className='px-2 text-xs sm:px-3 sm:text-sm'
                          onClick={() => handleSetExpiry(0, 0, 0)}
                        >
                          {t('keyQuery.fields.never')}
                        </Button>
                        <Button
                          type='button'
                          variant='outline'
                          size='sm'
                          className='px-2 text-xs sm:px-3 sm:text-sm'
                          onClick={() => handleSetExpiry(1, 0, 0)}
                        >
                          {t('keys.placeholders.value1Month')}
                        </Button>
                        <Button
                          type='button'
                          variant='outline'
                          size='sm'
                          className='px-2 text-xs sm:px-3 sm:text-sm'
                          onClick={() => handleSetExpiry(0, 1, 0)}
                        >
                          {t('keys.placeholders.value1Day')}
                        </Button>
                        <Button
                          type='button'
                          variant='outline'
                          size='sm'
                          className='px-2 text-xs sm:px-3 sm:text-sm'
                          onClick={() => handleSetExpiry(0, 0, 1)}
                        >
                          {t('keys.placeholders.value1Hour')}
                        </Button>
                      </div>
                    </div>
                    <FormMessage />
                  </FormItem>
                )}
              />

              {!isUpdate && (
                <FormField
                  control={form.control}
                  name='tokenCount'
                  render={({ field }) => (
                    <FormItem>
                      <FormLabel>{t('keys.fields.quantity')}</FormLabel>
                      <FormControl>
                        <Input
                          {...field}
                          type='number'
                          min='1'
                          placeholder={t('keys.fields.numberOfKeysToCreate')}
                          onChange={(e) =>
                            field.onChange(parseInt(e.target.value, 10) || 1)
                          }
                        />
                      </FormControl>
                      <FormDescription>
                        {t(
                          'keys.actions.createMultipleApiKeysAtOnceRandomSuffixWill'
                        )}
                      </FormDescription>
                      <FormMessage />
                    </FormItem>
                  )}
                />
              )}
            </SideDrawerSection>

            <SideDrawerSection>
              <SideDrawerSectionHeader
                title={t('keys.titles.quotaSettings')}
                description={t('keys.fields.setQuotaAmountAndLimits')}
                icon={<WalletCards className='size-4' />}
              />

              <FormField
                control={form.control}
                name='quota_type'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>{t('keys.fields.quotaType')}</FormLabel>
                    <FormControl>
                      <ApiKeyQuotaTypeCombobox
                        options={quotaTypeOptions}
                        value={field.value}
                        placeholder={t('keys.placeholders.selectQuotaType')}
                        onValueChange={(nextQuotaType) => {
                          field.onChange(nextQuotaType)
                          form.setValue(
                            'unlimited_quota',
                            nextQuotaType === 0,
                            { shouldDirty: true }
                          )
                        }}
                      />
                    </FormControl>
                    <FormDescription>
                      {t(
                        'keys.placeholders.chooseWhetherThisKeyIsUnlimitedHasAFixed'
                      )}
                    </FormDescription>
                    <FormMessage />
                  </FormItem>
                )}
              />

              {quotaType === 0 && (
                <p className='text-muted-foreground text-sm'>
                  {t('keys.tips.keyItselfHasNoQuotaCapButActualUsage')}
                </p>
              )}

              {quotaType === 1 && (
                <FormField
                  control={form.control}
                  name='remain_quota_dollars'
                  render={({ field }) => (
                    <FormItem>
                      <FormLabel>{quotaLabel}</FormLabel>
                      <FormControl>
                        <Input
                          value={field.value ?? ''}
                          type='number'
                          step={quotaInputStep}
                          placeholder={quotaPlaceholder}
                          onChange={(e) =>
                            field.onChange(parseFloat(e.target.value) || 0)
                          }
                        />
                      </FormControl>
                      <FormDescription>
                        {tokensOnly
                          ? t('keys.placeholders.enterTheQuotaAmountInTokens')
                          : t(
                              'keys.placeholders.enterTheQuotaAmountInCurrency',
                              {
                                currency: t(currencyLabel),
                              }
                            )}
                      </FormDescription>
                      <FormMessage />
                    </FormItem>
                  )}
                />
              )}

              {quotaType >= 2 && (
                <div className='grid gap-4 sm:grid-cols-3'>
                  <FormField
                    control={form.control}
                    name='window_hours'
                    render={({ field }) => (
                      <FormItem>
                        <FormLabel>{t('keys.fields.windowHours')}</FormLabel>
                        <FormControl>
                          <Input
                            value={field.value ?? ''}
                            type='number'
                            min='1'
                            max='720'
                            placeholder={t('channels.fields.hours')}
                            onChange={(e) =>
                              field.onChange(parseInt(e.target.value, 10) || 1)
                            }
                          />
                        </FormControl>
                        <FormDescription>
                          {t('keys.actions.resetWindowDurationInHours')}
                        </FormDescription>
                        <FormMessage />
                      </FormItem>
                    )}
                  />

                  <FormField
                    control={form.control}
                    name='window_quota_dollars'
                    render={({ field }) => (
                      <FormItem>
                        <FormLabel>
                          {t('keys.fields.windowQuota')} ({t(currencyLabel)})
                        </FormLabel>
                        <FormControl>
                          <Input
                            value={field.value ?? ''}
                            type='number'
                            step={quotaInputStep}
                            placeholder={quotaPlaceholder}
                            onChange={(e) =>
                              field.onChange(parseFloat(e.target.value) || 0)
                            }
                          />
                        </FormControl>
                        <FormDescription>
                          {t('keys.fields.quotaAvailableInEachWindow')}
                        </FormDescription>
                        <FormMessage />
                      </FormItem>
                    )}
                  />

                  <FormField
                    control={form.control}
                    name='window_start_hour'
                    render={({ field }) => (
                      <FormItem>
                        <FormLabel>{t('keys.actions.startHour')}</FormLabel>
                        <FormControl>
                          <Input
                            value={field.value ?? ''}
                            type='number'
                            min='0'
                            max='23'
                            placeholder='0-23'
                            onChange={(e) =>
                              field.onChange(parseInt(e.target.value, 10) || 0)
                            }
                          />
                        </FormControl>
                        <FormDescription>
                          {t('keys.fields.windowStartHourFrom0To23')}
                        </FormDescription>
                        <FormMessage />
                      </FormItem>
                    )}
                  />
                </div>
              )}

              {quotaType === 3 && (
                <div className='grid gap-4 sm:grid-cols-2'>
                  <FormField
                    control={form.control}
                    name='cycle_days'
                    render={({ field }) => (
                      <FormItem>
                        <FormLabel>{t('keys.fields.cycleDays')}</FormLabel>
                        <FormControl>
                          <Input
                            value={field.value ?? ''}
                            type='number'
                            min='1'
                            placeholder={t('keys.fields.days')}
                            onChange={(e) =>
                              field.onChange(parseInt(e.target.value, 10) || 1)
                            }
                          />
                        </FormControl>
                        <FormDescription>
                          {t('keys.fields.cycleDurationInDays')}
                        </FormDescription>
                        <FormMessage />
                      </FormItem>
                    )}
                  />

                  <FormField
                    control={form.control}
                    name='cycle_quota_dollars'
                    render={({ field }) => (
                      <FormItem>
                        <FormLabel>
                          {t('keys.fields.cycleQuota')} ({t(currencyLabel)})
                        </FormLabel>
                        <FormControl>
                          <Input
                            value={field.value ?? ''}
                            type='number'
                            step={quotaInputStep}
                            placeholder={quotaPlaceholder}
                            onChange={(e) =>
                              field.onChange(parseFloat(e.target.value) || 0)
                            }
                          />
                        </FormControl>
                        <FormDescription>
                          {t('keys.tips.totalQuotaAvailableInEachDayCycle')}
                        </FormDescription>
                        <FormMessage />
                      </FormItem>
                    )}
                  />
                </div>
              )}
            </SideDrawerSection>

            <SideDrawerSection>
              <SideDrawerSectionHeader
                title={t('channels.titles.advancedSettings')}
                description={t('keys.fields.setApiKeyAccessRestrictions')}
                icon={<Settings2 className='size-4' />}
              />
              <div className='flex flex-col gap-4'>
                <FormField
                  control={form.control}
                  name='model_limits'
                  render={({ field }) => (
                    <FormItem>
                      <FormLabel>{t('keys.fields.modelLimits')}</FormLabel>
                      <FormControl>
                        <MultiSelect
                          options={models.map((m) => ({
                            label: m,
                            value: m,
                          }))}
                          selected={field.value}
                          onChange={field.onChange}
                          placeholder={t(
                            'keys.placeholders.selectModelsEmptyForAllowAll'
                          )}
                        />
                      </FormControl>
                      <FormDescription>
                        {t('keys.status.limitWhichModelsCanBeUsedWithThisKey')}
                      </FormDescription>
                      <FormMessage />
                    </FormItem>
                  )}
                />

                <FormField
                  control={form.control}
                  name='model_mapping'
                  render={({ field }) => (
                    <FormItem>
                      <FormLabel>{t('keys.fields.modelMapping')}</FormLabel>
                      <FormControl>
                        <ModelMappingEditor
                          value={field.value || ''}
                          onChange={field.onChange}
                          disabled={isSubmitting}
                          fromLabel='keys.fields.clientRequestModel'
                          toLabel='keys.fields.redirectTargetModel'
                          fromPlaceholder='claude-3-5-sonnet'
                          toPlaceholder='glm-4-plus'
                          template='{"claude-3-5-sonnet": "glm-4-plus", "claude-3-7-sonnet": "glm-4-plus"}'
                        />
                      </FormControl>
                      <FormDescription>
                        {t(
                          'keys.tips.modelMappingRedirectsRequestsToTheTarget'
                        )}
                      </FormDescription>
                      <FormMessage />
                    </FormItem>
                  )}
                />

                <FormField
                  control={form.control}
                  name='allow_ips'
                  render={({ field }) => (
                    <FormItem>
                      <FormLabel>
                        {t('keys.tips.ipWhitelistSupportsCidr')}
                      </FormLabel>
                      <FormControl>
                        <Textarea
                          {...field}
                          className='min-h-20 resize-none'
                          placeholder={t(
                            'keys.placeholders.oneIpPerLineEmptyForNoRestriction'
                          )}
                          rows={3}
                        />
                      </FormControl>
                      <FormDescription>
                        {t('keys.tips.doNotOverTrustThisFeatureIpMayBe')}
                      </FormDescription>
                      <FormMessage />
                    </FormItem>
                  )}
                />
              </div>
            </SideDrawerSection>
          </form>
        </Form>
        <SheetFooter className={sideDrawerFooterClassName()}>
          <SheetClose
            render={<Button variant='outline' className='w-full sm:w-auto' />}
          >
            {t('common.actions.close')}
          </SheetClose>
          <Button
            type='button'
            onClick={form.handleSubmit(onSubmit, onInvalid)}
            disabled={isSubmitting}
            className='w-full sm:w-auto'
          >
            {isSubmitting
              ? t('channels.tips.saving')
              : t('channels.actions.saveChanges')}
          </Button>
        </SheetFooter>
      </SheetContent>
    </Sheet>
  )
}
