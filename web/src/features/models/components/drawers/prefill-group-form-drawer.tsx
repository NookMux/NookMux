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
import { useForm } from 'react-hook-form'
import { zodResolver } from '@hookform/resolvers/zod'
import { useQueryClient } from '@tanstack/react-query'
import { Loader2 } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
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
import { Textarea } from '@/components/ui/textarea'
import {
  SideDrawerSection,
  sideDrawerContentClassName,
  sideDrawerFooterClassName,
  sideDrawerFormClassName,
  sideDrawerHeaderClassName,
} from '@/components/drawer-layout'
import { JsonEditor } from '@/components/json-editor'
import { StatusBadge } from '@/components/status-badge'
import { TagInput } from '@/components/tag-input'
import { createPrefillGroup, updatePrefillGroup } from '../../api'
import { ENDPOINT_TEMPLATES } from '../../constants'
import { prefillGroupsQueryKeys } from '../../lib'
import {
  prefillGroupFormSchema,
  type PrefillGroup,
  type PrefillGroupFormValues,
} from '../../types'
import {
  DEFAULT_FORM_VALUES,
  PREFILL_GROUP_TYPE_META,
  PREFILL_GROUP_TYPES,
  type PrefillGroupType,
  parseStringItems,
  serializeEndpointItems,
} from '../prefill-group-shared'

type PrefillGroupFormDrawerProps = {
  open: boolean
  onClose: () => void
  currentGroup: PrefillGroup | null
}

export function PrefillGroupFormDrawer({
  open,
  onClose,
  currentGroup,
}: PrefillGroupFormDrawerProps) {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const isEdit = Boolean(currentGroup?.id)
  const [isSaving, setIsSaving] = useState(false)

  const form = useForm<PrefillGroupFormValues>({
    resolver: zodResolver(prefillGroupFormSchema),
    defaultValues: DEFAULT_FORM_VALUES,
  })

  const selectedType = form.watch('type')

  useEffect(() => {
    if (open) {
      if (isEdit && currentGroup) {
        form.reset({
          id: currentGroup.id,
          name: currentGroup.name,
          description: currentGroup.description || '',
          type: currentGroup.type,
          items:
            currentGroup.type === 'endpoint'
              ? serializeEndpointItems(currentGroup.items)
              : parseStringItems(currentGroup.items),
        })
      } else {
        form.reset(DEFAULT_FORM_VALUES)
      }
    }
  }, [open, isEdit, currentGroup, form])

  useEffect(() => {
    const currentItems = form.getValues('items')
    if (selectedType === 'endpoint' && Array.isArray(currentItems)) {
      form.setValue('items', '', { shouldValidate: false })
    } else if (
      selectedType !== 'endpoint' &&
      typeof currentItems === 'string'
    ) {
      form.setValue('items', [], { shouldValidate: false })
    }
  }, [selectedType, form])

  const handleOpenChange = (nextOpen: boolean) => {
    if (!nextOpen) {
      onClose()
    }
  }

  const handleSubmit = async (values: PrefillGroupFormValues) => {
    setIsSaving(true)
    const payload = {
      name: values.name.trim(),
      type: values.type,
      description: values.description?.trim() || '',
      items:
        values.type === 'endpoint'
          ? typeof values.items === 'string'
            ? values.items
            : ''
          : Array.isArray(values.items)
            ? values.items
            : [],
    }

    try {
      const response = isEdit
        ? await updatePrefillGroup({
            id: currentGroup!.id,
            ...payload,
          })
        : await createPrefillGroup(payload)

      if (response.success) {
        toast.success(
          isEdit
            ? t('models.status.prefillGroupUpdated')
            : t('models.status.prefillGroupCreated')
        )
        queryClient.invalidateQueries({
          queryKey: prefillGroupsQueryKeys.lists(),
        })
        onClose()
      } else {
        toast.error(response.message || t('channels.status.operationFailed'))
      }
    } catch (err: unknown) {
      toast.error(
        (err as Error)?.message || t('channels.status.operationFailed')
      )
    } finally {
      setIsSaving(false)
    }
  }

  const meta =
    PREFILL_GROUP_TYPE_META[selectedType] || PREFILL_GROUP_TYPE_META.model

  return (
    <Sheet open={open} onOpenChange={handleOpenChange}>
      <SheetContent className={sideDrawerContentClassName('sm:max-w-2xl')}>
        <SheetHeader className={sideDrawerHeaderClassName()}>
          <SheetTitle>
            {isEdit
              ? t('models.actions.editPrefillGroup')
              : t('models.actions.createPrefillGroup')}
          </SheetTitle>
          <SheetDescription>
            {isEdit
              ? t('models.tips.updateTheReusableBundleBelow')
              : t('models.tips.captureAReusableBundleOfModelsTagsOrEndpoints')}
          </SheetDescription>
        </SheetHeader>

        <Form {...form}>
          <form
            id='prefill-group-form'
            onSubmit={form.handleSubmit(handleSubmit)}
            className={sideDrawerFormClassName()}
          >
            <SideDrawerSection>
              <div className='flex flex-col gap-1'>
                <h3 className='text-sm font-semibold'>
                  {t('models.titles.groupDetails')}
                </h3>
                <p className='text-muted-foreground text-sm'>
                  {t(
                    'models.tips.giveTheGroupARecognizableNameAndOptionalDescription'
                  )}
                </p>
              </div>

              <FormField
                control={form.control}
                name='name'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>{t('models.fields.groupName')}</FormLabel>
                    <FormControl>
                      <Input
                        placeholder={t('models.titles.premiumChatModels')}
                        {...field}
                      />
                    </FormControl>
                    <FormDescription>
                      {t('models.tips.giveThisGroupARecognizableName')}
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
                        placeholder={t(
                          'models.tips.optionalNotesAboutWhenToUseThisGroup'
                        )}
                        rows={3}
                        {...field}
                      />
                    </FormControl>
                    <FormDescription>
                      {t('models.tips.makeItEasierForTeammatesToPickTheRight')}
                    </FormDescription>
                    <FormMessage />
                  </FormItem>
                )}
              />
            </SideDrawerSection>

            <SideDrawerSection>
              <div className='flex flex-col gap-1'>
                <h3 className='text-sm font-semibold'>
                  {t('models.titles.configuration')}
                </h3>
                <p className='text-muted-foreground text-sm'>
                  {t(
                    'models.placeholders.chooseTheBundleTypeAndDefineTheItemsInside'
                  )}
                </p>
              </div>

              <FormField
                control={form.control}
                name='type'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>{t('models.fields.groupType')}</FormLabel>
                    <Select
                      items={[
                        ...PREFILL_GROUP_TYPES.map((type) => ({
                          value: type.value,
                          label: (
                            <div className='flex flex-col text-left'>
                              <span className='font-medium'>
                                {t(type.label)}
                              </span>
                              <span
                                data-prefill-description
                                className='text-muted-foreground text-xs'
                              >
                                {t(type.description)}
                              </span>
                            </div>
                          ),
                        })),
                      ]}
                      value={field.value}
                      onValueChange={(value) =>
                        value !== null &&
                        field.onChange(value as PrefillGroupType)
                      }
                    >
                      <FormControl>
                        <SelectTrigger className='[&_[data-slot=select-value]_[data-prefill-description]]:hidden'>
                          <SelectValue
                            placeholder={t(
                              'models.placeholders.selectAGroupType'
                            )}
                          />
                        </SelectTrigger>
                      </FormControl>
                      <SelectContent alignItemWithTrigger={false}>
                        <SelectGroup>
                          {PREFILL_GROUP_TYPES.map((type) => (
                            <SelectItem key={type.value} value={type.value}>
                              <div className='flex flex-col text-left'>
                                <span className='font-medium'>
                                  {t(type.label)}
                                </span>
                                <span
                                  data-prefill-description
                                  className='text-muted-foreground text-xs'
                                >
                                  {t(type.description)}
                                </span>
                              </div>
                            </SelectItem>
                          ))}
                        </SelectGroup>
                      </SelectContent>
                    </Select>
                    <FormDescription>
                      {t(
                        'models.tips.determinesHowThisGroupIsAppliedElsewhere'
                      )}
                    </FormDescription>
                    <FormMessage />
                  </FormItem>
                )}
              />

              <div className='border-border/60 flex flex-col gap-3 border-y py-4'>
                <div className='flex items-center gap-2'>
                  <h4 className='text-sm font-medium'>
                    {t('models.fields.items')}
                  </h4>
                  <StatusBadge
                    label={t(meta.label)}
                    variant={meta.badge}
                    size='sm'
                    copyable={false}
                  />
                </div>
                <FormField
                  control={form.control}
                  name='items'
                  render={({ field }) => (
                    <FormItem>
                      <FormLabel className='sr-only'>
                        {t('models.fields.items')}
                      </FormLabel>
                      <FormControl>
                        {selectedType === 'endpoint' ? (
                          <JsonEditor
                            value={(field.value as string) || ''}
                            onChange={field.onChange}
                            keyPlaceholder='provider'
                            valuePlaceholder='{"path": "/v1/...","method": "POST"}'
                            keyLabel={t('models.fields.provider')}
                            valueLabel={t(
                              'models.titles.endpointConfiguration'
                            )}
                            valueType='any'
                            template={ENDPOINT_TEMPLATES}
                            emptyMessage={t(
                              'models.tips.defineEndpointMappingsForEachProvider'
                            )}
                          />
                        ) : (
                          <TagInput
                            value={
                              Array.isArray(field.value) ? field.value : []
                            }
                            onChange={field.onChange}
                            placeholder={t(
                              'models.placeholders.enterAValueAndPressEnter'
                            )}
                          />
                        )}
                      </FormControl>
                      <FormDescription>
                        {selectedType === 'endpoint'
                          ? t(
                              'models.tips.provideAJsonObjectWhereEachKeyMapsTo'
                            )
                          : t(
                              'models.actions.addEachModelOrTagYouWantToInclude'
                            )}
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
            render={
              <Button type='button' variant='outline' disabled={isSaving} />
            }
          >
            {t('common.actions.cancel')}
          </SheetClose>
          <Button type='submit' form='prefill-group-form' disabled={isSaving}>
            {isSaving && <Loader2 className='mr-2 h-4 w-4 animate-spin' />}
            {isSaving
              ? t('channels.tips.saving')
              : isEdit
                ? t('channels.actions.saveChanges')
                : t('channels.actions.create')}
          </Button>
        </SheetFooter>
      </SheetContent>
    </Sheet>
  )
}
