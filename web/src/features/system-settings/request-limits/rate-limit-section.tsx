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
import * as z from 'zod'
import { useForm } from 'react-hook-form'
import { zodResolver } from '@hookform/resolvers/zod'
import { useTranslation } from 'react-i18next'
import { Form, FormControl, FormField } from '@/components/ui/form'
import { Textarea } from '@/components/ui/textarea'
import { EditorModeButton } from '../components/editor-mode-button'
import { SettingsForm } from '../components/settings-form-layout'
import {
  SettingsGroup,
  SettingsGroupSwitch,
  SettingsRow,
  SettingsUnitField,
  SettingsUnitInput,
} from '../components/settings-group'
import { SettingsPageFormActions } from '../components/settings-page-context'
import { SettingsSection } from '../components/settings-section'
import { useUpdateOption } from '../hooks/use-update-option'
import { RateLimitVisualEditor } from './rate-limit-visual-editor'

const isValidJSON = (value: string | undefined) => {
  if (!value || value.trim() === '') return true
  try {
    const parsed = JSON.parse(value)
    if (typeof parsed !== 'object' || Array.isArray(parsed)) {
      return false
    }
    for (const [, val] of Object.entries(parsed)) {
      if (!Array.isArray(val) || val.length !== 2) return false
      if (typeof val[0] !== 'number' || typeof val[1] !== 'number') return false
      if (val[0] < 0 || val[1] < 1) return false
      if (val[0] > 2147483647 || val[1] > 2147483647) return false
    }
    return true
  } catch {
    return false
  }
}

const createRateLimitSchema = (t: (key: string) => string) =>
  z.object({
    ModelRequestRateLimitEnabled: z.boolean(),
    ModelRequestRateLimitDurationMinutes: z
      .number()
      .min(0, t('systemSettings.errors.mustBeNonNegative')),
    ModelRequestRateLimitCount: z
      .number()
      .min(0, t('systemSettings.errors.mustBeNonNegative'))
      .max(100000000, t('systemSettings.errors.mustBeAtMost100000000')),
    ModelRequestRateLimitSuccessCount: z
      .number()
      .min(1, t('systemSettings.errors.mustBeAtLeast1'))
      .max(100000000, t('systemSettings.errors.mustBeAtMost100000000')),
    ModelRequestRateLimitGroup: z
      .string()
      .optional()
      .refine(isValidJSON, {
        message: t(
          'systemSettings.errors.invalidJsonFormatOrValuesOutOfAllowedRange'
        ),
      }),
  })

type RateLimitFormValues = z.infer<ReturnType<typeof createRateLimitSchema>>

type RateLimitSectionProps = {
  defaultValues: RateLimitFormValues
}

export function RateLimitSection({ defaultValues }: RateLimitSectionProps) {
  const { t } = useTranslation()
  const updateOption = useUpdateOption()
  const [useVisualEditor, setUseVisualEditor] = useState(true)

  const rateLimitSchema = createRateLimitSchema(t)

  const form = useForm<RateLimitFormValues>({
    resolver: zodResolver(rateLimitSchema),
    mode: 'onChange', // Enable real-time validation
    defaultValues,
  })

  useEffect(() => {
    form.reset(defaultValues)
  }, [defaultValues, form])
  const enabled = form.watch('ModelRequestRateLimitEnabled')

  const onSubmit = async (values: RateLimitFormValues) => {
    const updates = Object.entries(values).filter(
      ([key, value]) =>
        value !== defaultValues[key as keyof RateLimitFormValues]
    )

    for (const [key, value] of updates) {
      await updateOption.mutateAsync({ key, value: value ?? '' })
    }
  }

  return (
    <SettingsSection title={t('home.fields.rateLimiting')}>
      <Form {...form}>
        <SettingsForm onSubmit={form.handleSubmit(onSubmit)}>
          <SettingsPageFormActions
            onSave={form.handleSubmit(onSubmit)}
            isSaving={updateOption.isPending}
            saveLabel='common.actions.saveRateLimits'
          />

          <SettingsGroup
            title={t('systemSettings.actions.enableRateLimiting')}
            description={t(
              'systemSettings.tips.controlsModelRequestRateLimitingWebApiRouteThrottling'
            )}
            control={
              <SettingsGroupSwitch name='ModelRequestRateLimitEnabled' />
            }
            enabled={enabled}
          >
            <FormField
              control={form.control}
              name='ModelRequestRateLimitDurationMinutes'
              render={({ field }) => (
                <SettingsRow
                  label={t('systemSettings.fields.limitPeriod')}
                  description={t(
                    'systemSettings.fields.timeWindowForRateLimiting'
                  )}
                >
                  <SettingsUnitField
                    input={
                      <FormControl>
                        <SettingsUnitInput
                          type='number'
                          min={0}
                          step={1}
                          {...field}
                          onChange={(event) =>
                            field.onChange(parseInt(event.target.value) || 0)
                          }
                        />
                      </FormControl>
                    }
                    unit={t('subscriptions.fields.minutes')}
                  />
                </SettingsRow>
              )}
            />

            <FormField
              control={form.control}
              name='ModelRequestRateLimitCount'
              render={({ field }) => (
                <SettingsRow
                  label={t('systemSettings.fields.maxRequestsPerPeriod')}
                  description={t(
                    'systemSettings.status.includingFailedRequests0Unlimited'
                  )}
                >
                  <SettingsUnitField
                    input={
                      <FormControl>
                        <SettingsUnitInput
                          type='number'
                          min={0}
                          max={100000000}
                          step={1}
                          {...field}
                          onChange={(event) =>
                            field.onChange(parseInt(event.target.value) || 0)
                          }
                        />
                      </FormControl>
                    }
                    unit={t('systemSettings.fields.times')}
                  />
                </SettingsRow>
              )}
            />

            <FormField
              control={form.control}
              name='ModelRequestRateLimitSuccessCount'
              render={({ field }) => (
                <SettingsRow
                  label={t('systemSettings.status.maxSuccessfulRequests')}
                  description={t(
                    'systemSettings.status.onlySuccessfulRequests'
                  )}
                >
                  <SettingsUnitField
                    input={
                      <FormControl>
                        <SettingsUnitInput
                          type='number'
                          min={1}
                          max={100000000}
                          step={1}
                          {...field}
                          onChange={(event) =>
                            field.onChange(parseInt(event.target.value) || 1)
                          }
                        />
                      </FormControl>
                    }
                    unit={t('systemSettings.fields.times')}
                  />
                </SettingsRow>
              )}
            />

            <FormField
              control={form.control}
              name='ModelRequestRateLimitGroup'
              render={({ field }) => (
                <SettingsRow
                  label={t('systemSettings.fields.groupBasedRateLimits')}
                  stacked
                  actions={
                    <EditorModeButton
                      visualMode={useVisualEditor}
                      onToggle={() => setUseVisualEditor((current) => !current)}
                    />
                  }
                  hint={
                    !useVisualEditor && (
                      <div className='space-y-1'>
                        <p className='font-semibold'>
                          {t('channels.fields.labelWithColon', {
                            label: t('channels.fields.format'),
                          })}
                        </p>
                        <ul className='list-inside list-disc space-y-0.5 pl-2'>
                          <li>
                            {t('systemSettings.fields.jsonObject')}{' '}
                            {`{"groupName": [maxRequests, maxSuccess]}`}
                          </li>
                          <li>
                            {t('channels.fields.labelWithColon', {
                              label: t('channels.placeholders.example'),
                            })}{' '}
                            {`{"default": [200, 100], "vip": [0, 1000]}`}
                          </li>
                          <li>
                            {t(
                              'systemSettings.tips.maxRequests0MaxSuccess1Both2147'
                            )}
                          </li>
                          <li>
                            {t(
                              'systemSettings.tips.groupConfigOverridesGlobalLimitsSharesTheSamePeriod'
                            )}
                          </li>
                        </ul>
                      </div>
                    )
                  }
                >
                  <FormControl>
                    {useVisualEditor ? (
                      <RateLimitVisualEditor
                        value={field.value || ''}
                        onChange={field.onChange}
                      />
                    ) : (
                      <Textarea
                        rows={8}
                        placeholder={`{
  "default": [200, 100],
  "vip": [0, 1000]
}`}
                        className='font-mono text-sm'
                        {...field}
                      />
                    )}
                  </FormControl>
                </SettingsRow>
              )}
            />
          </SettingsGroup>
        </SettingsForm>
      </Form>
    </SettingsSection>
  )
}
