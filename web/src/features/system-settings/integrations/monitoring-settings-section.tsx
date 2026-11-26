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
import { useMemo, useRef } from 'react'
import * as z from 'zod'
import { useForm } from 'react-hook-form'
import { zodResolver } from '@hookform/resolvers/zod'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { parseHttpStatusCodeRules } from '@/lib/http-status-code-rules'
import { Form, FormControl, FormField } from '@/components/ui/form'
import { Input } from '@/components/ui/input'
import { Switch } from '@/components/ui/switch'
import { Textarea } from '@/components/ui/textarea'
import { SettingsForm } from '../components/settings-form-layout'
import {
  SettingsControl,
  SettingsDependent,
  SettingsGroup,
  SettingsRow,
} from '../components/settings-group'
import { SettingsPageFormActions } from '../components/settings-page-context'
import { SettingsSection } from '../components/settings-section'
import { useResetForm } from '../hooks/use-reset-form'
import { useUpdateOption } from '../hooks/use-update-option'

const numericString = z.string().refine((value) => {
  const trimmed = value.trim()
  if (!trimmed) return true
  return !Number.isNaN(Number(trimmed)) && Number(trimmed) >= 0
}, 'systemSettings.errors.enterNonNegativeNumberOrEmpty')

const monitoringSchema = z
  .object({
    ChannelDisableThreshold: numericString,
    QuotaRemindThreshold: numericString,
    AutomaticDisableChannelEnabled: z.boolean(),
    AutomaticEnableChannelEnabled: z.boolean(),
    AutomaticDisableKeywords: z.string(),
    AutomaticDisableStatusCodes: z.string(),
    monitor_setting: z.object({
      auto_test_channel_enabled: z.boolean(),
      auto_test_channel_minutes: z.coerce
        .number()
        .int('common.errors.mustBeAnInteger')
        .min(1, 'systemSettings.errors.intervalMustBeAtLeast1Minute'),
    }),
  })
  .superRefine((values, ctx) => {
    const disableParsed = parseHttpStatusCodeRules(
      values.AutomaticDisableStatusCodes
    )
    if (!disableParsed.ok) {
      ctx.addIssue({
        code: 'custom',
        path: ['AutomaticDisableStatusCodes'],
        message: `Invalid status code rules: ${disableParsed.invalidTokens.join(
          ', '
        )}`,
      })
    }
  })

type MonitoringFormValues = z.output<typeof monitoringSchema>
type MonitoringFormInput = z.input<typeof monitoringSchema>

type MonitoringSettingsSectionProps = {
  defaultValues: {
    ChannelDisableThreshold: string
    QuotaRemindThreshold: string
    AutomaticDisableChannelEnabled: boolean
    AutomaticEnableChannelEnabled: boolean
    AutomaticDisableKeywords: string
    AutomaticDisableStatusCodes: string
    'monitor_setting.auto_test_channel_enabled': boolean
    'monitor_setting.auto_test_channel_minutes': number
  }
}

function normalizeLineEndings(value: string) {
  return value.replace(/\r\n/g, '\n')
}

type NormalizedMonitoringValues = {
  ChannelDisableThreshold: string
  QuotaRemindThreshold: string
  AutomaticDisableChannelEnabled: boolean
  AutomaticEnableChannelEnabled: boolean
  AutomaticDisableKeywords: string
  AutomaticDisableStatusCodes: string
  'monitor_setting.auto_test_channel_enabled': boolean
  'monitor_setting.auto_test_channel_minutes': number
}

const buildFormDefaults = (
  defaults: MonitoringSettingsSectionProps['defaultValues']
): MonitoringFormInput => ({
  ChannelDisableThreshold: defaults.ChannelDisableThreshold ?? '',
  QuotaRemindThreshold: defaults.QuotaRemindThreshold ?? '',
  AutomaticDisableChannelEnabled: defaults.AutomaticDisableChannelEnabled,
  AutomaticEnableChannelEnabled: defaults.AutomaticEnableChannelEnabled,
  AutomaticDisableKeywords: normalizeLineEndings(
    defaults.AutomaticDisableKeywords ?? ''
  ),
  AutomaticDisableStatusCodes: defaults.AutomaticDisableStatusCodes ?? '',
  monitor_setting: {
    auto_test_channel_enabled:
      defaults['monitor_setting.auto_test_channel_enabled'],
    auto_test_channel_minutes:
      defaults['monitor_setting.auto_test_channel_minutes'],
  },
})

const normalizeDefaults = (
  defaults: MonitoringSettingsSectionProps['defaultValues']
): NormalizedMonitoringValues => ({
  ChannelDisableThreshold: (defaults.ChannelDisableThreshold ?? '').trim(),
  QuotaRemindThreshold: (defaults.QuotaRemindThreshold ?? '').trim(),
  AutomaticDisableChannelEnabled: defaults.AutomaticDisableChannelEnabled,
  AutomaticEnableChannelEnabled: defaults.AutomaticEnableChannelEnabled,
  AutomaticDisableKeywords: normalizeLineEndings(
    defaults.AutomaticDisableKeywords ?? ''
  ),
  AutomaticDisableStatusCodes: parseHttpStatusCodeRules(
    defaults.AutomaticDisableStatusCodes ?? ''
  ).normalized,
  'monitor_setting.auto_test_channel_enabled':
    defaults['monitor_setting.auto_test_channel_enabled'],
  'monitor_setting.auto_test_channel_minutes':
    defaults['monitor_setting.auto_test_channel_minutes'],
})

const normalizeFormValues = (
  values: MonitoringFormValues
): NormalizedMonitoringValues => ({
  ChannelDisableThreshold: values.ChannelDisableThreshold.trim(),
  QuotaRemindThreshold: values.QuotaRemindThreshold.trim(),
  AutomaticDisableChannelEnabled: values.AutomaticDisableChannelEnabled,
  AutomaticEnableChannelEnabled: values.AutomaticEnableChannelEnabled,
  AutomaticDisableKeywords: normalizeLineEndings(
    values.AutomaticDisableKeywords
  ),
  AutomaticDisableStatusCodes: parseHttpStatusCodeRules(
    values.AutomaticDisableStatusCodes
  ).normalized,
  'monitor_setting.auto_test_channel_enabled':
    values.monitor_setting.auto_test_channel_enabled,
  'monitor_setting.auto_test_channel_minutes':
    values.monitor_setting.auto_test_channel_minutes,
})

export function MonitoringSettingsSection({
  defaultValues,
}: MonitoringSettingsSectionProps) {
  const { t } = useTranslation()
  const updateOption = useUpdateOption()
  const baselineRef = useRef<NormalizedMonitoringValues>(
    normalizeDefaults(defaultValues)
  )

  const formDefaults = useMemo(
    () => buildFormDefaults(defaultValues),
    [defaultValues]
  )

  const form = useForm<MonitoringFormInput, unknown, MonitoringFormValues>({
    resolver: zodResolver(monitoringSchema),
    defaultValues: formDefaults,
  })

  useResetForm(form, formDefaults)

  const autoTestEnabled = form.watch(
    'monitor_setting.auto_test_channel_enabled'
  )
  const autoDisableStatusCodes = form.watch('AutomaticDisableStatusCodes')
  const autoDisableParsed = useMemo(
    () => parseHttpStatusCodeRules(autoDisableStatusCodes),
    [autoDisableStatusCodes]
  )

  const onSubmit = async (values: MonitoringFormValues) => {
    const normalized = normalizeFormValues(values)
    const updates = (
      Object.keys(normalized) as Array<keyof NormalizedMonitoringValues>
    ).filter((key) => normalized[key] !== baselineRef.current[key])

    if (updates.length === 0) {
      toast.info(t('channels.fields.noChangesToSave'))
      return
    }

    for (const key of updates) {
      const value = normalized[key]
      await updateOption.mutateAsync({
        key,
        value,
      })
    }

    baselineRef.current = normalized
  }

  return (
    <SettingsSection title={t('systemSettings.fields.monitoringAlerts')}>
      <Form {...form}>
        <SettingsForm onSubmit={form.handleSubmit(onSubmit)}>
          <SettingsPageFormActions
            onSave={form.handleSubmit(onSubmit)}
            isSaving={updateOption.isPending}
            saveLabel='common.actions.saveMonitoringRules'
          />

          <SettingsGroup title={t('systemSettings.fields.autoCheck')}>
            <div className='flex min-w-0 flex-col'>
              <FormField
                control={form.control}
                name='monitor_setting.auto_test_channel_enabled'
                render={({ field }) => (
                  <SettingsRow
                    label={t('systemSettings.fields.scheduledChannelTests')}
                    description={t(
                      'systemSettings.tips.automaticallyProbeAllChannelsInTheBackground'
                    )}
                  >
                    <FormControl>
                      <Switch
                        checked={field.value === true}
                        onCheckedChange={field.onChange}
                      />
                    </FormControl>
                  </SettingsRow>
                )}
              />
              <SettingsDependent enabled={autoTestEnabled}>
                <FormField
                  control={form.control}
                  name='monitor_setting.auto_test_channel_minutes'
                  render={({ field }) => (
                    <SettingsRow
                      label={t('systemSettings.fields.testIntervalMinutes')}
                      description={t(
                        'systemSettings.tips.howFrequentlyTheSystemTestsAllChannels'
                      )}
                    >
                      <SettingsControl>
                        <FormControl>
                          <Input
                            type='number'
                            min={1}
                            step={1}
                            value={
                              typeof field.value === 'number' &&
                              Number.isFinite(field.value)
                                ? field.value
                                : ''
                            }
                            onChange={(event) =>
                              field.onChange(event.target.valueAsNumber)
                            }
                            name={field.name}
                            onBlur={field.onBlur}
                            ref={field.ref}
                          />
                        </FormControl>
                      </SettingsControl>
                    </SettingsRow>
                  )}
                />
              </SettingsDependent>
            </div>
          </SettingsGroup>

          <SettingsGroup>
            <FormField
              control={form.control}
              name='ChannelDisableThreshold'
              render={({ field }) => (
                <SettingsRow
                  label={t('systemSettings.actions.disableThresholdSeconds')}
                  description={t(
                    'systemSettings.tips.automaticallyDisableChannelsExceedingThisResponseTime'
                  )}
                >
                  <SettingsControl>
                    <FormControl>
                      <Input
                        type='number'
                        min={0}
                        step={1}
                        value={field.value}
                        onChange={(event) => field.onChange(event.target.value)}
                      />
                    </FormControl>
                  </SettingsControl>
                </SettingsRow>
              )}
            />

            <FormField
              control={form.control}
              name='AutomaticDisableChannelEnabled'
              render={({ field }) => (
                <SettingsRow
                  label={t('systemSettings.actions.disableOnFailure')}
                  description={t(
                    'systemSettings.tips.automaticallyDisableChannelsWhenTestsFail'
                  )}
                >
                  <FormControl>
                    <Switch
                      checked={field.value}
                      onCheckedChange={field.onChange}
                    />
                  </FormControl>
                </SettingsRow>
              )}
            />

            <FormField
              control={form.control}
              name='AutomaticEnableChannelEnabled'
              render={({ field }) => (
                <SettingsRow
                  label={t('systemSettings.status.reEnableOnSuccess')}
                  description={t(
                    'systemSettings.status.bringChannelsBackOnlineAfterSuccessfulChecks'
                  )}
                >
                  <FormControl>
                    <Switch
                      checked={field.value}
                      onCheckedChange={field.onChange}
                    />
                  </FormControl>
                </SettingsRow>
              )}
            />

            <FormField
              control={form.control}
              name='AutomaticDisableKeywords'
              render={({ field }) => (
                <SettingsRow
                  label={t('systemSettings.errors.failureKeywords')}
                  description={t(
                    'systemSettings.status.ifAnUpstreamErrorContainsAnyOfTheseKeywords'
                  )}
                >
                  <FormControl>
                    <Textarea
                      rows={6}
                      placeholder={t(
                        'systemSettings.placeholders.oneKeywordPerLine'
                      )}
                      {...field}
                      onChange={(event) => field.onChange(event.target.value)}
                    />
                  </FormControl>
                </SettingsRow>
              )}
            />

            <FormField
              control={form.control}
              name='AutomaticDisableStatusCodes'
              render={({ field }) => (
                <SettingsRow
                  label={t('systemSettings.fields.autoDisableStatusCodes')}
                  description={t(
                    'systemSettings.tips.acceptsCommaSeparatedStatusCodesAndInclusiveRanges'
                  )}
                  hint={
                    autoDisableParsed.ok &&
                    autoDisableParsed.normalized &&
                    autoDisableParsed.normalized !== field.value.trim() ? (
                      <>
                        {t('systemSettings.fields.normalized')}{' '}
                        {autoDisableParsed.normalized}
                      </>
                    ) : undefined
                  }
                >
                  <SettingsControl>
                    <FormControl>
                      <Input
                        placeholder={t(
                          'systemSettings.placeholders.eG401403429500599'
                        )}
                        value={field.value}
                        onChange={(event) => field.onChange(event.target.value)}
                      />
                    </FormControl>
                  </SettingsControl>
                </SettingsRow>
              )}
            />
          </SettingsGroup>

          <SettingsGroup title={t('systemSettings.fields.quotaReminder')}>
            <FormField
              control={form.control}
              name='QuotaRemindThreshold'
              render={({ field }) => (
                <SettingsRow
                  label={t('systemSettings.tips.defaultQuotaReminderThreshold')}
                  description={t(
                    'systemSettings.tips.defaultReminderThresholdInBackendQuotaUnitsTokensUsers'
                  )}
                >
                  <SettingsControl>
                    <FormControl>
                      <Input
                        type='number'
                        min={0}
                        step={1}
                        value={field.value}
                        onChange={(event) => field.onChange(event.target.value)}
                      />
                    </FormControl>
                  </SettingsControl>
                </SettingsRow>
              )}
            />
          </SettingsGroup>
        </SettingsForm>
      </Form>
    </SettingsSection>
  )
}
