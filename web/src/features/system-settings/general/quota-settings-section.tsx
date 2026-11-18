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
import { useMemo, type ChangeEvent } from 'react'
import * as z from 'zod'
import type { Resolver } from 'react-hook-form'
import { zodResolver } from '@hookform/resolvers/zod'
import { useTranslation } from 'react-i18next'
import { getLocalizedCurrencyLabel } from '@/lib/currency'
import { parseQuotaFromDollars, quotaUnitsToDollars } from '@/lib/format'
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
import { FormDirtyIndicator } from '../components/form-dirty-indicator'
import { FormNavigationGuard } from '../components/form-navigation-guard'
import {
  SettingsForm,
  SettingsFormGrid,
} from '../components/settings-form-layout'
import { SettingsPageFormActions } from '../components/settings-page-context'
import { SettingsSection } from '../components/settings-section'
import { useSettingsForm } from '../hooks/use-settings-form'
import { useUpdateOption } from '../hooks/use-update-option'

const quotaSchema = z.object({
  QuotaForNewUser: z.coerce
    .number()
    .min(0, 'systemSettings.errors.mustBeNonNegative'),
  PreConsumedQuota: z.coerce
    .number()
    .min(0, 'systemSettings.errors.mustBeNonNegative'),
  QuotaForInviter: z.coerce
    .number()
    .min(0, 'systemSettings.errors.mustBeNonNegative'),
  QuotaForInvitee: z.coerce
    .number()
    .min(0, 'systemSettings.errors.mustBeNonNegative'),
  quota_setting: z.object({
    free_model_pre_consumed_quota: z.coerce
      .number()
      .min(0, 'systemSettings.errors.mustBeNonNegative'),
  }),
})

type QuotaFormValues = z.infer<typeof quotaSchema>

type QuotaSettingsSectionProps = {
  defaultValues: QuotaFormValues
}

export function QuotaSettingsSection({
  defaultValues,
}: QuotaSettingsSectionProps) {
  const { t } = useTranslation()
  const updateOption = useUpdateOption()
  const quotaUnitLabel = getLocalizedCurrencyLabel(t)
  const displayDefaultValues = useMemo(
    () => ({
      ...defaultValues,
      QuotaForNewUser: quotaUnitsToDollars(defaultValues.QuotaForNewUser),
      PreConsumedQuota: quotaUnitsToDollars(defaultValues.PreConsumedQuota),
      QuotaForInviter: quotaUnitsToDollars(defaultValues.QuotaForInviter),
      QuotaForInvitee: quotaUnitsToDollars(defaultValues.QuotaForInvitee),
      quota_setting: {
        ...defaultValues.quota_setting,
        free_model_pre_consumed_quota: quotaUnitsToDollars(
          defaultValues.quota_setting.free_model_pre_consumed_quota
        ),
      },
    }),
    [defaultValues]
  )
  const handleNumberChange =
    (onChange: (value: number | string) => void) =>
    (event: ChangeEvent<HTMLInputElement>) => {
      onChange(
        event.target.value === '' ? '' : event.currentTarget.valueAsNumber
      )
    }

  const { form, handleSubmit, isDirty, isSubmitting } =
    useSettingsForm<QuotaFormValues>({
      resolver: zodResolver(quotaSchema) as Resolver<
        QuotaFormValues,
        unknown,
        QuotaFormValues
      >,
      defaultValues: displayDefaultValues,
      onSubmit: async (_data, changedFields) => {
        const quotaFieldNames = new Set([
          'QuotaForNewUser',
          'PreConsumedQuota',
          'QuotaForInviter',
          'QuotaForInvitee',
          'quota_setting.free_model_pre_consumed_quota',
        ])

        for (const [key, value] of Object.entries(changedFields)) {
          const submitValue = quotaFieldNames.has(key)
            ? parseQuotaFromDollars(Number(value))
            : value
          await updateOption.mutateAsync({
            key,
            value: submitValue as string | number | boolean,
          })
        }
      },
    })

  return (
    <SettingsSection title={t('keys.titles.quotaSettings')}>
      <FormNavigationGuard when={isDirty} />

      <Form {...form}>
        <SettingsForm onSubmit={handleSubmit}>
          <SettingsPageFormActions
            onSave={handleSubmit}
            isSaving={updateOption.isPending || isSubmitting}
          />
          <FormDirtyIndicator isDirty={isDirty} />
          <SettingsFormGrid>
            <FormField
              control={form.control}
              name='QuotaForNewUser'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>
                    {t('systemSettings.fields.newUserQuota')}
                  </FormLabel>
                  <FormControl>
                    <Input
                      type='number'
                      step='any'
                      value={field.value ?? ''}
                      onChange={handleNumberChange(field.onChange)}
                      name={field.name}
                      onBlur={field.onBlur}
                      ref={field.ref}
                    />
                  </FormControl>
                  <FormDescription>
                    {t('systemSettings.tips.initialQuotaGivenToNewUsers')}
                    {' · '}
                    {t('systemSettings.fields.displayedInUnit', {
                      unit: quotaUnitLabel,
                    })}
                  </FormDescription>
                  <FormMessage />
                </FormItem>
              )}
            />

            <FormField
              control={form.control}
              name='PreConsumedQuota'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>
                    {t('systemSettings.fields.preConsumedQuota')}
                  </FormLabel>
                  <FormControl>
                    <Input
                      type='number'
                      step='any'
                      value={field.value ?? ''}
                      onChange={handleNumberChange(field.onChange)}
                      name={field.name}
                      onBlur={field.onBlur}
                      ref={field.ref}
                    />
                  </FormControl>
                  <FormDescription>
                    {t('systemSettings.tips.quotaConsumedBeforeChargingUsers')}
                    {' · '}
                    {t('systemSettings.fields.displayedInUnit', {
                      unit: quotaUnitLabel,
                    })}
                  </FormDescription>
                  <FormMessage />
                </FormItem>
              )}
            />

            <FormField
              control={form.control}
              name='QuotaForInviter'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>
                    {t('systemSettings.fields.inviterReward')}
                  </FormLabel>
                  <FormControl>
                    <Input
                      type='number'
                      step='any'
                      value={field.value ?? ''}
                      onChange={handleNumberChange(field.onChange)}
                      name={field.name}
                      onBlur={field.onBlur}
                      ref={field.ref}
                    />
                  </FormControl>
                  <FormDescription>
                    {t('systemSettings.tips.quotaGivenToUsersWhoInviteOthers')}
                    {' · '}
                    {t('systemSettings.fields.displayedInUnit', {
                      unit: quotaUnitLabel,
                    })}
                  </FormDescription>
                  <FormMessage />
                </FormItem>
              )}
            />

            <FormField
              control={form.control}
              name='QuotaForInvitee'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>
                    {t('systemSettings.fields.inviteeReward')}
                  </FormLabel>
                  <FormControl>
                    <Input
                      type='number'
                      step='any'
                      value={field.value ?? ''}
                      onChange={handleNumberChange(field.onChange)}
                      name={field.name}
                      onBlur={field.onBlur}
                      ref={field.ref}
                    />
                  </FormControl>
                  <FormDescription>
                    {t('systemSettings.titles.quotaGivenToInvitedUsers')}
                    {' · '}
                    {t('systemSettings.fields.displayedInUnit', {
                      unit: quotaUnitLabel,
                    })}
                  </FormDescription>
                  <FormMessage />
                </FormItem>
              )}
            />

            <FormField
              control={form.control}
              name='quota_setting.free_model_pre_consumed_quota'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>
                    {t('systemSettings.titles.preConsumeForFreeModels')}
                  </FormLabel>
                  <FormControl>
                    <Input
                      type='number'
                      step='any'
                      value={field.value ?? ''}
                      onChange={handleNumberChange(field.onChange)}
                      name={field.name}
                      onBlur={field.onBlur}
                      ref={field.ref}
                    />
                  </FormControl>
                  <FormDescription>
                    {t(
                      'systemSettings.tips.quotaPreConsumedForZeroCostModelsSet0'
                    )}
                    {' · '}
                    {t('systemSettings.fields.displayedInUnit', {
                      unit: quotaUnitLabel,
                    })}
                  </FormDescription>
                  <FormMessage />
                </FormItem>
              )}
            />
          </SettingsFormGrid>
        </SettingsForm>
      </Form>
    </SettingsSection>
  )
}
