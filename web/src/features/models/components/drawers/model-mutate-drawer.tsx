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
import { useEffect, useState, useCallback } from 'react'
import * as z from 'zod'
import { useForm } from 'react-hook-form'
import { zodResolver } from '@hookform/resolvers/zod'
import { useQuery, useQueryClient } from '@tanstack/react-query'
import type { TFunction } from 'i18next'
import { Loader2 } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { Button } from '@/components/ui/button'
import { Checkbox } from '@/components/ui/checkbox'
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
import {
  SideDrawerSection,
  sideDrawerContentClassName,
  sideDrawerFooterClassName,
  sideDrawerFormClassName,
  sideDrawerHeaderClassName,
  sideDrawerSwitchItemClassName,
} from '@/components/drawer-layout'
import { JsonEditor } from '@/components/json-editor'
import { TagInput } from '@/components/tag-input'
import { createModel, updateModel, getModel, getVendors } from '../../api'
import { getNameRuleOptions, ENDPOINT_TEMPLATES } from '../../constants'
import { modelsQueryKeys, vendorsQueryKeys, parseModelTags } from '../../lib'
import type { Model } from '../../types'

const MODALITY_OPTIONS = ['text', 'image', 'audio', 'video', 'file'] as const
const CAPABILITY_OPTIONS = [
  'function_calling',
  'streaming',
  'vision',
  'json_mode',
  'structured_output',
  'reasoning',
  'tools',
  'system_prompt',
  'web_search',
  'code_interpreter',
  'caching',
  'embeddings',
] as const

const MODEL_OPTION_LABEL_KEYS: Record<string, string> = {
  text: 'common.fields.text',
  image: 'models.fields.image',
  audio: 'common.fields.audio',
  video: 'pricing.fields.video',
  file: 'common.fields.file',
  function_calling: 'common.fields.functionCalling',
  streaming: 'common.fields.streaming',
  vision: 'common.fields.vision',
  json_mode: 'common.fields.jsonMode',
  structured_output: 'common.fields.structuredOutput',
  reasoning: 'common.fields.reasoning',
  tools: 'common.fields.tools',
  system_prompt: 'common.fields.systemPrompt',
  web_search: 'common.fields.webSearch',
  code_interpreter: 'common.fields.codeInterpreter',
  caching: 'common.fields.promptCaching',
  embeddings: 'pricing.fields.embeddings',
}

const extendedModelFormSchema = z.object({
  id: z.number().optional(),
  model_name: z.string().min(1, 'models.errors.modelNameRequired'),
  description: z.string(),
  icon: z.string(),
  tags: z.array(z.string()),
  vendor_id: z.number().optional(),
  endpoints: z.string(),
  context_length: z
    .number()
    .int('common.errors.mustBeAnInteger')
    .min(0, 'systemSettings.errors.mustBeNonNegative'),
  max_output_tokens: z
    .number()
    .int('common.errors.mustBeAnInteger')
    .min(0, 'systemSettings.errors.mustBeNonNegative'),
  input_modalities: z.array(z.string()),
  output_modalities: z.array(z.string()),
  capabilities: z.array(z.string()),
  knowledge_cutoff: z.string(),
  release_date: z.string(),
  parameter_count: z.string(),
  name_rule: z.number(),
  status: z.boolean(),
})

type ExtendedModelFormValues = z.infer<typeof extendedModelFormSchema>

type ModelMutateDrawerProps = {
  open: boolean
  onOpenChange: (open: boolean) => void
  currentRow?: Model | null
}

export function ModelMutateDrawer({
  open,
  onOpenChange,
  currentRow,
}: ModelMutateDrawerProps) {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const isEditing = Boolean(currentRow?.id)
  const [isSubmitting, setIsSubmitting] = useState(false)

  // Fetch vendors for dropdown
  const { data: vendorsData } = useQuery({
    queryKey: vendorsQueryKeys.list(),
    queryFn: () => getVendors({ page_size: 1000 }),
    enabled: open,
  })

  const vendors = vendorsData?.data?.items || []

  // Fetch model detail if editing
  const { data: modelData } = useQuery({
    queryKey: modelsQueryKeys.detail(currentRow?.id || 0),
    queryFn: () => getModel(currentRow!.id),
    enabled: open && isEditing,
  })

  const form = useForm<ExtendedModelFormValues>({
    resolver: zodResolver(extendedModelFormSchema),
    defaultValues: {
      model_name: '',
      description: '',
      icon: '',
      tags: [],
      vendor_id: undefined,
      endpoints: '',
      context_length: 0,
      max_output_tokens: 0,
      input_modalities: ['text'],
      output_modalities: ['text'],
      capabilities: [],
      knowledge_cutoff: '',
      release_date: '',
      parameter_count: '',
      name_rule: 0,
      status: true,
    },
  })

  // Load model data for editing
  useEffect(() => {
    if (open && isEditing && modelData?.data) {
      const model = modelData.data
      form.reset({
        id: model.id,
        model_name: model.model_name,
        description: model.description || '',
        icon: model.icon || '',
        tags: parseModelTags(model.tags),
        vendor_id: model.vendor_id,
        endpoints: model.endpoints || '',
        context_length: model.context_length || 0,
        max_output_tokens: model.max_output_tokens || 0,
        input_modalities: model.input_modalities?.length
          ? model.input_modalities
          : ['text'],
        output_modalities: model.output_modalities?.length
          ? model.output_modalities
          : ['text'],
        capabilities: model.capabilities || [],
        knowledge_cutoff: model.knowledge_cutoff || '',
        release_date: model.release_date || '',
        parameter_count: model.parameter_count || '',
        name_rule: model.name_rule || 0,
        status: model.status === 1,
      })
    } else if (open && !isEditing) {
      // Pre-fill model name if passed from missing models
      form.reset({
        model_name: currentRow?.model_name || '',
        description: '',
        icon: '',
        tags: [],
        vendor_id: undefined,
        endpoints: '',
        context_length: 0,
        max_output_tokens: 0,
        input_modalities: ['text'],
        output_modalities: ['text'],
        capabilities: [],
        knowledge_cutoff: '',
        release_date: '',
        parameter_count: '',
        name_rule: 0,
        status: true,
      })
    }
  }, [open, isEditing, modelData, currentRow, form])

  const onSubmit = useCallback(
    async (values: ExtendedModelFormValues): Promise<void> => {
      setIsSubmitting(true)
      try {
        const modelData = {
          ...values,
          id: isEditing ? currentRow!.id : undefined,
          tags: Array.isArray(values.tags) ? values.tags.join(',') : '',
          status: values.status ? 1 : 0,
        }

        const response = isEditing
          ? await updateModel({ ...modelData, id: currentRow!.id })
          : await createModel(modelData)

        if (response.success) {
          toast.success(
            isEditing
              ? t('models.status.modelUpdatedSuccessfully')
              : t('models.status.modelCreatedSuccessfully')
          )
          queryClient.invalidateQueries({ queryKey: modelsQueryKeys.lists() })
          queryClient.invalidateQueries({ queryKey: ['pricing'] })
          onOpenChange(false)
        } else {
          toast.error(response.message || t('channels.status.operationFailed'))
        }
      } catch (error: unknown) {
        toast.error(
          (error as Error)?.message || t('channels.status.operationFailed')
        )
      } finally {
        setIsSubmitting(false)
      }
    },
    [isEditing, currentRow, queryClient, onOpenChange, t]
  )

  const handleFillEndpointTemplate = (templateKey: string) => {
    const template = ENDPOINT_TEMPLATES[templateKey]
    if (template) {
      const templateJson = JSON.stringify({ [templateKey]: template }, null, 2)
      form.setValue('endpoints', templateJson)
    }
  }

  return (
    <Sheet open={open} onOpenChange={onOpenChange}>
      <SheetContent className={sideDrawerContentClassName('sm:max-w-2xl')}>
        <SheetHeader className={sideDrawerHeaderClassName()}>
          <SheetTitle>
            {isEditing
              ? t('models.actions.editModel')
              : t('models.actions.createModel')}
          </SheetTitle>
          <SheetDescription>
            {isEditing
              ? t('common.tips.updateModelConfigurationAndClickSaveWhenYouRe')
              : t('models.actions.addANewModelToTheSystemByProviding')}
          </SheetDescription>
        </SheetHeader>

        <Form {...form}>
          <form
            id='model-form'
            onSubmit={form.handleSubmit(
              onSubmit as Parameters<typeof form.handleSubmit>[0]
            )}
            className={sideDrawerFormClassName()}
          >
            {/* Basic Information */}
            <SideDrawerSection>
              <h3 className='text-sm font-semibold'>
                {t('channels.titles.basicInformation')}
              </h3>

              <FormField
                control={form.control}
                name='model_name'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>{t('models.fields.modelNamee83a31')}</FormLabel>
                    <FormControl>
                      <Input
                        placeholder={t('models.tips.gpt4Claude3OpusEtc')}
                        {...field}
                      />
                    </FormControl>
                    <FormDescription>
                      {t('models.tips.uniqueIdentifierForThisModel')}
                    </FormDescription>
                    <FormMessage />
                  </FormItem>
                )}
              />

              <FormField
                control={form.control}
                name='description'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>{t('auditLogs.tips.description')}</FormLabel>
                    <FormControl>
                      <Textarea
                        rows={3}
                        placeholder={t('models.tips.describeThisModel')}
                        {...field}
                      />
                    </FormControl>
                    <FormDescription>
                      {t('models.tips.displayedInTheModelMarketplaceDetails')}
                    </FormDescription>
                    <FormMessage />
                  </FormItem>
                )}
              />

              <FormField
                control={form.control}
                name='icon'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>{t('models.fields.icon')}</FormLabel>
                    <FormControl>
                      <Input
                        placeholder={t('models.tips.openAiAnthropicEtc')}
                        {...field}
                      />
                    </FormControl>
                    <FormDescription className='text-xs'>
                      {t('models.fields.lobehubIconsKey')}
                    </FormDescription>
                    <FormMessage />
                  </FormItem>
                )}
              />

              <FormField
                control={form.control}
                name='vendor_id'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>{t('models.fields.vendor')}</FormLabel>
                    <Select
                      items={[
                        ...vendors.map((vendor) => ({
                          value: String(vendor.id),
                          label: vendor.name,
                        })),
                      ]}
                      onValueChange={(value) =>
                        field.onChange(value ? parseInt(value) : undefined)
                      }
                      value={field.value ? String(field.value) : undefined}
                    >
                      <FormControl>
                        <SelectTrigger>
                          <SelectValue
                            placeholder={t('models.placeholders.selectVendor')}
                          />
                        </SelectTrigger>
                      </FormControl>
                      <SelectContent alignItemWithTrigger={false}>
                        <SelectGroup>
                          {vendors.map((vendor) => (
                            <SelectItem
                              key={vendor.id}
                              value={String(vendor.id)}
                            >
                              {vendor.name}
                            </SelectItem>
                          ))}
                        </SelectGroup>
                      </SelectContent>
                    </Select>
                    <FormMessage />
                  </FormItem>
                )}
              />

              <FormField
                control={form.control}
                name='tags'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>{t('models.fields.tags')}</FormLabel>
                    <FormControl>
                      <TagInput
                        value={field.value || []}
                        onChange={field.onChange}
                        placeholder={t('common.actions.addTags')}
                      />
                    </FormControl>
                    <FormDescription>
                      {t('models.tips.pressEnterOrCommaToAddTags')}
                    </FormDescription>
                    <FormMessage />
                  </FormItem>
                )}
              />
            </SideDrawerSection>

            {/* Matching Configuration */}
            <SideDrawerSection>
              <h3 className='text-sm font-semibold'>
                {t('models.fields.matchingRules')}
              </h3>

              <FormField
                control={form.control}
                name='name_rule'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>{t('models.fields.nameRule')}</FormLabel>
                    <FormControl>
                      <RadioGroup
                        onValueChange={(value) =>
                          field.onChange(parseInt(value))
                        }
                        value={String(field.value)}
                        className='grid grid-cols-2 gap-4'
                      >
                        {getNameRuleOptions(t).map((option) => (
                          <div
                            key={option.value}
                            className='flex items-center space-x-2'
                          >
                            <RadioGroupItem
                              value={String(option.value)}
                              id={`rule-${option.value}`}
                            />
                            <Label
                              htmlFor={`rule-${option.value}`}
                              className='cursor-pointer font-normal'
                            >
                              {option.label}
                            </Label>
                          </div>
                        ))}
                      </RadioGroup>
                    </FormControl>
                    <FormDescription>
                      {t('models.tips.howThisModelNameShouldMatchRequests')}
                    </FormDescription>
                    <FormMessage />
                  </FormItem>
                )}
              />
            </SideDrawerSection>

            {/* Endpoints Configuration */}
            <SideDrawerSection>
              <div className='flex items-center justify-between'>
                <h3 className='text-sm font-semibold'>
                  {t('models.fields.endpoints')}
                </h3>
                <Select<string>
                  items={[
                    ...Object.keys(ENDPOINT_TEMPLATES).map((key) => ({
                      value: key,
                      label: key,
                    })),
                  ]}
                  onValueChange={(v) =>
                    v !== null && handleFillEndpointTemplate(v)
                  }
                >
                  <SelectTrigger size='sm' className='w-[200px]'>
                    <SelectValue placeholder={t('models.tips.loadTemplate')} />
                  </SelectTrigger>
                  <SelectContent alignItemWithTrigger={false}>
                    <SelectGroup>
                      {Object.keys(ENDPOINT_TEMPLATES).map((key) => (
                        <SelectItem key={key} value={key}>
                          {key}
                        </SelectItem>
                      ))}
                    </SelectGroup>
                  </SelectContent>
                </Select>
              </div>

              <FormField
                control={form.control}
                name='endpoints'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>
                      {t('models.titles.endpointConfiguration')}
                    </FormLabel>
                    <FormControl>
                      <JsonEditor
                        value={field.value || ''}
                        onChange={field.onChange}
                        keyPlaceholder='endpoint_type'
                        valuePlaceholder='{"path": "/v1/...", "method": "POST"}'
                        keyLabel={t('models.fields.endpointType')}
                        valueLabel={t('models.fields.configuration')}
                        valueType='any'
                        emptyMessage={t(
                          'models.tips.noEndpointsConfiguredSwitchToJsonModeOrAdd'
                        )}
                      />
                    </FormControl>
                    <FormDescription>
                      {t(
                        'models.tips.defineApiEndpointsForThisModelJsonFormat'
                      )}
                    </FormDescription>
                    <FormMessage />
                  </FormItem>
                )}
              />
            </SideDrawerSection>

            {/* Metadata Configuration */}
            <SideDrawerSection>
              <h3 className='text-sm font-semibold'>
                {t('models.fields.modelMetadata')}
              </h3>

              <div className='grid gap-4 sm:grid-cols-2'>
                <FormField
                  control={form.control}
                  name='context_length'
                  render={({ field }) => (
                    <FormItem>
                      <FormLabel>{t('models.fields.contextWindow')}</FormLabel>
                      <FormControl>
                        <Input
                          type='number'
                          min={0}
                          step={1}
                          {...field}
                          onChange={(event) =>
                            field.onChange(Number(event.target.value || 0))
                          }
                        />
                      </FormControl>
                      <FormDescription>
                        {t(
                          'models.tips.maximumInputTokensSupportedByThisModel'
                        )}
                      </FormDescription>
                      <FormMessage />
                    </FormItem>
                  )}
                />

                <FormField
                  control={form.control}
                  name='max_output_tokens'
                  render={({ field }) => (
                    <FormItem>
                      <FormLabel>{t('models.fields.maxOutput')}</FormLabel>
                      <FormControl>
                        <Input
                          type='number'
                          min={0}
                          step={1}
                          {...field}
                          onChange={(event) =>
                            field.onChange(Number(event.target.value || 0))
                          }
                        />
                      </FormControl>
                      <FormDescription>
                        {t('models.tips.maximumTokensPerResponse')}
                      </FormDescription>
                      <FormMessage />
                    </FormItem>
                  )}
                />
              </div>

              <div className='grid gap-4 sm:grid-cols-3'>
                <FormField
                  control={form.control}
                  name='knowledge_cutoff'
                  render={({ field }) => (
                    <FormItem>
                      <FormLabel>
                        {t('models.fields.knowledgeCutoff')}
                      </FormLabel>
                      <FormControl>
                        <Input placeholder='2024-10' {...field} />
                      </FormControl>
                      <FormMessage />
                    </FormItem>
                  )}
                />

                <FormField
                  control={form.control}
                  name='release_date'
                  render={({ field }) => (
                    <FormItem>
                      <FormLabel>{t('models.fields.released')}</FormLabel>
                      <FormControl>
                        <Input placeholder='2025-05' {...field} />
                      </FormControl>
                      <FormMessage />
                    </FormItem>
                  )}
                />

                <FormField
                  control={form.control}
                  name='parameter_count'
                  render={({ field }) => (
                    <FormItem>
                      <FormLabel>{t('common.fields.parameters')}</FormLabel>
                      <FormControl>
                        <Input placeholder='70B' {...field} />
                      </FormControl>
                      <FormMessage />
                    </FormItem>
                  )}
                />
              </div>

              <FormField
                control={form.control}
                name='input_modalities'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>{t('models.fields.inputModalities')}</FormLabel>
                    <FormControl>
                      <CheckboxGrid
                        options={MODALITY_OPTIONS}
                        value={field.value}
                        onChange={field.onChange}
                        t={t}
                      />
                    </FormControl>
                    <FormMessage />
                  </FormItem>
                )}
              />

              <FormField
                control={form.control}
                name='output_modalities'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>{t('models.fields.outputModalities')}</FormLabel>
                    <FormControl>
                      <CheckboxGrid
                        options={MODALITY_OPTIONS}
                        value={field.value}
                        onChange={field.onChange}
                        t={t}
                      />
                    </FormControl>
                    <FormMessage />
                  </FormItem>
                )}
              />

              <FormField
                control={form.control}
                name='capabilities'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>{t('models.fields.capabilities')}</FormLabel>
                    <FormControl>
                      <CheckboxGrid
                        options={CAPABILITY_OPTIONS}
                        value={field.value}
                        onChange={field.onChange}
                        t={t}
                      />
                    </FormControl>
                    <FormMessage />
                  </FormItem>
                )}
              />
            </SideDrawerSection>

            {/* Status */}
            <SideDrawerSection>
              <h3 className='text-sm font-semibold'>
                {t('channels.fields.status')}
              </h3>

              <FormField
                control={form.control}
                name='status'
                render={({ field }) => (
                  <FormItem className={sideDrawerSwitchItemClassName()}>
                    <div className='flex flex-col gap-0.5'>
                      <FormLabel className='text-base'>
                        {t('channels.status.enabled')}
                      </FormLabel>
                      <FormDescription>
                        {t('models.actions.enableOrDisableThisModel')}
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
            </SideDrawerSection>
          </form>
        </Form>

        <SheetFooter className={sideDrawerFooterClassName()}>
          <SheetClose
            render={<Button variant='outline' disabled={isSubmitting} />}
          >
            {t('common.actions.cancel')}
          </SheetClose>
          <Button form='model-form' type='submit' disabled={isSubmitting}>
            {isSubmitting && <Loader2 className='mr-2 h-4 w-4 animate-spin' />}
            {isEditing
              ? t('models.fields.updateModel')
              : t('models.actions.createModel')}
          </Button>
        </SheetFooter>
      </SheetContent>
    </Sheet>
  )
}

function CheckboxGrid(props: {
  options: readonly string[]
  value?: string[]
  onChange: (value: string[]) => void
  t: TFunction
}) {
  const selected = new Set(props.value || [])

  return (
    <div className='grid grid-cols-2 gap-2 sm:grid-cols-3'>
      {props.options.map((option) => {
        const checked = selected.has(option)
        return (
          <label
            key={option}
            className='border-border/70 bg-muted/20 hover:bg-muted/40 flex cursor-pointer items-center gap-2 rounded-md border px-2.5 py-2 text-sm transition-colors'
          >
            <Checkbox
              checked={checked}
              onCheckedChange={(next) => {
                const values = new Set(selected)
                if (next) {
                  values.add(option)
                } else {
                  values.delete(option)
                }
                props.onChange(Array.from(values))
              }}
            />
            <span className='min-w-0 truncate'>
              {props.t(MODEL_OPTION_LABEL_KEYS[option] ?? option)}
            </span>
          </label>
        )
      })}
    </div>
  )
}
