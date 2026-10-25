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
import * as z from 'zod'
import { useForm } from 'react-hook-form'
import { zodResolver } from '@hookform/resolvers/zod'
import { useTranslation } from 'react-i18next'
import { parseHttpStatusCodeRules } from '@/lib/http-status-code-rules'
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
  SettingsControlChildren,
  SettingsControlGroup,
  SettingsForm,
  SettingsSwitchContent,
  SettingsSwitchItem,
} from '../components/settings-form-layout'
import { SettingsPageFormActions } from '../components/settings-page-context'
import { SettingsSection } from '../components/settings-section'
import { useResetForm } from '../hooks/use-reset-form'
import { useUpdateOption } from '../hooks/use-update-option'

const behaviorSchema = z
  .object({
    RetryTimes: z.coerce.number().int().min(0).max(99),
    AutomaticRetryEnabled: z.boolean(),
    AutomaticRetryStatusCodes: z.string(),
  })
  .superRefine((values, ctx) => {
    if (values.AutomaticRetryEnabled && values.RetryTimes <= 0) {
      ctx.addIssue({
        code: 'custom',
        path: ['RetryTimes'],
        message: 'Retry times must be greater than 0 when retry is enabled',
      })
    }

    const parsed = parseHttpStatusCodeRules(values.AutomaticRetryStatusCodes)
    if (!parsed.ok) {
      ctx.addIssue({
        code: 'custom',
        path: ['AutomaticRetryStatusCodes'],
        message: `Invalid status code rules: ${parsed.invalidTokens.join(', ')}`,
      })
    }
  })

type BehaviorFormValues = z.output<typeof behaviorSchema>
type BehaviorFormInput = z.input<typeof behaviorSchema>

type SystemBehaviorSectionProps = {
  defaultValues: BehaviorFormValues
}

type OptionKey =
  | 'RetryTimes'
  | 'AutomaticRetryEnabled'
  | 'AutomaticRetryStatusCodes'

type NormalizedBehaviorValues = {
  RetryTimes: number
  AutomaticRetryEnabled: boolean
  AutomaticRetryStatusCodes: string
}

const normalizeDefaults = (
  defaults: BehaviorFormValues
): NormalizedBehaviorValues => ({
  RetryTimes: defaults.RetryTimes,
  AutomaticRetryEnabled: defaults.AutomaticRetryEnabled,
  AutomaticRetryStatusCodes: parseHttpStatusCodeRules(
    defaults.AutomaticRetryStatusCodes ?? ''
  ).normalized,
})

export function SystemBehaviorSection({
  defaultValues,
}: SystemBehaviorSectionProps) {
  const { t } = useTranslation()
  const updateOption = useUpdateOption()

  const form = useForm<BehaviorFormInput, unknown, BehaviorFormValues>({
    resolver: zodResolver(behaviorSchema),
    defaultValues,
  })

  useResetForm(form, defaultValues)

  const retryEnabled = form.watch('AutomaticRetryEnabled')
  const autoRetryStatusCodes = form.watch('AutomaticRetryStatusCodes')
  const autoRetryParsed = useMemo(
    () => parseHttpStatusCodeRules(autoRetryStatusCodes),
    [autoRetryStatusCodes]
  )

  const baseline = useMemo(
    () => normalizeDefaults(defaultValues),
    [defaultValues]
  )

  const onSubmit = async (data: BehaviorFormValues) => {
    const normalized: NormalizedBehaviorValues = {
      RetryTimes: data.RetryTimes,
      AutomaticRetryEnabled: data.AutomaticRetryEnabled,
      AutomaticRetryStatusCodes: parseHttpStatusCodeRules(
        data.AutomaticRetryStatusCodes
      ).normalized,
    }

    const changed = (Object.keys(normalized) as OptionKey[]).filter(
      (key) => normalized[key] !== baseline[key]
    )

    if (changed.length === 0) {
      return
    }

    // Save retry count and status codes before enabling retry so the backend
    // validation (RetryTimes > 0 when enabled) passes in the same batch.
    const orderedKeys = changed.sort((a) =>
      a === 'AutomaticRetryEnabled' ? 1 : -1
    )

    for (const key of orderedKeys) {
      await updateOption.mutateAsync({ key, value: normalized[key] })
    }

    baseline.RetryTimes = normalized.RetryTimes
    baseline.AutomaticRetryEnabled = normalized.AutomaticRetryEnabled
    baseline.AutomaticRetryStatusCodes = normalized.AutomaticRetryStatusCodes
  }

  return (
    <SettingsSection title={t('systemSettings.titles.behavior')}>
      <Form {...form}>
        <SettingsForm onSubmit={form.handleSubmit(onSubmit)}>
          <SettingsPageFormActions
            onSave={form.handleSubmit(onSubmit)}
            isSaving={updateOption.isPending}
          />

          <SettingsControlGroup>
            <DisabledSettingsNotice enabled={retryEnabled} />
            <FormField
              control={form.control}
              name='AutomaticRetryEnabled'
              render={({ field }) => (
                <SettingsSwitchItem>
                  <SettingsSwitchContent>
                    <FormLabel>
                      {t('systemSettings.fields.automaticRetry')}
                    </FormLabel>
                    <FormDescription>
                      {t(
                        'systemSettings.actions.retryFailedRequestsOnAlternateChannelsBeforeReturningAn'
                      )}
                    </FormDescription>
                  </SettingsSwitchContent>
                  <FormControl>
                    <Switch
                      checked={field.value}
                      onCheckedChange={field.onChange}
                    />
                  </FormControl>
                </SettingsSwitchItem>
              )}
            />

            <SettingsControlChildren>
              <FormField
                control={form.control}
                name='RetryTimes'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>
                      {t('systemSettings.actions.retryTimes')}
                    </FormLabel>
                    <FormControl>
                      <Input
                        type='number'
                        min='0'
                        max='99'
                        step='1'
                        value={
                          typeof field.value === 'number' &&
                          Number.isFinite(field.value)
                            ? field.value
                            : ''
                        }
                        onChange={(e) => field.onChange(e.target.valueAsNumber)}
                        name={field.name}
                        onBlur={field.onBlur}
                        ref={field.ref}
                        disabled={!retryEnabled}
                      />
                    </FormControl>
                    <FormDescription>
                      {t(
                        'systemSettings.tips.numberOfRetryAttemptsBeyondTheFirstRequest0'
                      )}
                    </FormDescription>
                    <FormMessage />
                  </FormItem>
                )}
              />

              <FormField
                control={form.control}
                name='AutomaticRetryStatusCodes'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>
                      {t('systemSettings.fields.autoRetryStatusCodes')}
                    </FormLabel>
                    <FormControl>
                      <Input
                        placeholder={t(
                          'systemSettings.placeholders.eG401403429500599'
                        )}
                        value={field.value}
                        onChange={(event) => field.onChange(event.target.value)}
                        disabled={!retryEnabled}
                      />
                    </FormControl>
                    <FormDescription>
                      {t(
                        'systemSettings.tips.acceptsCommaSeparatedStatusCodesAndInclusiveRanges'
                      )}{' '}
                      {autoRetryParsed.ok &&
                        autoRetryParsed.normalized &&
                        autoRetryParsed.normalized !== field.value.trim() && (
                          <span className='text-muted-foreground'>
                            {t('systemSettings.fields.normalized')}{' '}
                            {autoRetryParsed.normalized}
                          </span>
                        )}
                    </FormDescription>
                    <FormMessage />
                  </FormItem>
                )}
              />
            </SettingsControlChildren>
          </SettingsControlGroup>
        </SettingsForm>
      </Form>
    </SettingsSection>
  )
}
