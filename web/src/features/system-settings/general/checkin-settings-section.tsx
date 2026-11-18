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
import { useMemo } from 'react'
import { z } from 'zod'
import { useForm, type Resolver } from 'react-hook-form'
import { zodResolver } from '@hookform/resolvers/zod'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
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
import { Switch } from '@/components/ui/switch'
import { DisabledSettingsNotice } from '../components/disabled-settings-notice'
import {
  SettingsForm,
  SettingsSwitchContent,
  SettingsSwitchItem,
} from '../components/settings-form-layout'
import { SettingsPageFormActions } from '../components/settings-page-context'
import { SettingsSection } from '../components/settings-section'
import { useUpdateOption } from '../hooks/use-update-option'

const schema = z.object({
  enabled: z.boolean(),
  minQuota: z.coerce.number().min(0, 'systemSettings.errors.mustBeNonNegative'),
  maxQuota: z.coerce.number().min(0, 'systemSettings.errors.mustBeNonNegative'),
})

type Values = z.infer<typeof schema>

export function CheckinSettingsSection({
  defaultValues,
}: {
  defaultValues: {
    enabled: boolean
    minQuota: number
    maxQuota: number
  }
}) {
  const { t } = useTranslation()
  const updateOption = useUpdateOption()
  const quotaUnitLabel = getLocalizedCurrencyLabel(t)
  const displayDefaults = useMemo(
    () => ({
      enabled: defaultValues.enabled,
      minQuota: quotaUnitsToDollars(defaultValues.minQuota),
      maxQuota: quotaUnitsToDollars(defaultValues.maxQuota),
    }),
    [defaultValues.enabled, defaultValues.maxQuota, defaultValues.minQuota]
  )

  const form = useForm<Values>({
    resolver: zodResolver(schema) as unknown as Resolver<Values>,
    defaultValues: displayDefaults,
  })

  const { isDirty, isSubmitting } = form.formState
  const enabled = form.watch('enabled')

  async function onSubmit(values: Values) {
    const updates: Array<{ key: string; value: string }> = []

    if (values.enabled !== displayDefaults.enabled) {
      updates.push({
        key: 'checkin_setting.enabled',
        value: String(values.enabled),
      })
    }

    if (values.minQuota !== displayDefaults.minQuota) {
      updates.push({
        key: 'checkin_setting.min_quota',
        value: String(parseQuotaFromDollars(values.minQuota)),
      })
    }

    if (values.maxQuota !== displayDefaults.maxQuota) {
      updates.push({
        key: 'checkin_setting.max_quota',
        value: String(parseQuotaFromDollars(values.maxQuota)),
      })
    }

    if (updates.length === 0) {
      toast.info(t('channels.fields.noChangesToSave'))
      return
    }

    for (const update of updates) {
      await updateOption.mutateAsync(update)
    }

    form.reset(values)
  }

  return (
    <SettingsSection title={t('systemSettings.titles.checkInSettings')}>
      <Form {...form}>
        <SettingsForm onSubmit={form.handleSubmit(onSubmit)} autoComplete='off'>
          <SettingsPageFormActions
            onSave={form.handleSubmit(onSubmit)}
            isSaving={updateOption.isPending || isSubmitting}
            isSaveDisabled={!isDirty}
            saveLabel='common.actions.saveCheckInSettings'
          />
          <DisabledSettingsNotice enabled={enabled} />

          <FormField
            control={form.control}
            name='enabled'
            render={({ field }) => (
              <SettingsSwitchItem>
                <SettingsSwitchContent>
                  <FormLabel>
                    {t('systemSettings.actions.enableCheckInFeature')}
                  </FormLabel>
                  <FormDescription>
                    {t(
                      'systemSettings.tips.allowUsersToCheckInDailyForRandomQuota'
                    )}
                  </FormDescription>
                </SettingsSwitchContent>
                <FormControl>
                  <Switch
                    checked={field.value}
                    onCheckedChange={field.onChange}
                    disabled={updateOption.isPending || isSubmitting}
                  />
                </FormControl>
              </SettingsSwitchItem>
            )}
          />

          {enabled && (
            <div className='grid gap-6 sm:grid-cols-2'>
              <FormField
                control={form.control}
                name='minQuota'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>
                      {t('systemSettings.fields.minimumCheckInQuota')}
                    </FormLabel>
                    <FormControl>
                      <Input
                        type='number'
                        min={0}
                        step='any'
                        placeholder={t('systemSettings.placeholders.value1000')}
                        {...field}
                      />
                    </FormControl>
                    <FormDescription>
                      {t(
                        'systemSettings.tips.minimumQuotaAmountAwardedForCheckIn'
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

              <FormField
                control={form.control}
                name='maxQuota'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>
                      {t('systemSettings.fields.maximumCheckInQuota')}
                    </FormLabel>
                    <FormControl>
                      <Input
                        type='number'
                        min={0}
                        step='any'
                        placeholder={t(
                          'systemSettings.placeholders.value10000'
                        )}
                        {...field}
                      />
                    </FormControl>
                    <FormDescription>
                      {t(
                        'systemSettings.tips.maximumQuotaAmountAwardedForCheckIn'
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
            </div>
          )}
        </SettingsForm>
      </Form>
    </SettingsSection>
  )
}
