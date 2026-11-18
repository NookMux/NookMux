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
import { useEffect, useRef } from 'react'
import * as z from 'zod'
import { useForm } from 'react-hook-form'
import { zodResolver } from '@hookform/resolvers/zod'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import {
  Form,
  FormControl,
  FormDescription,
  FormField,
  FormItem,
  FormLabel,
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

const XAI_VIOLATION_FEE_DOC_URL =
  'https://docs.x.ai/docs/models#usage-guidelines-violation-fee'

// Fields are grouped under a `grok` object so that the dotted option keys
// (`grok.*`) map onto a real nested structure. React Hook Form treats dots in
// field names as nested paths; a flat defaultValues map with literal dotted
// keys desyncs from the registered paths and the submitted data never reflects
// user edits.
const schema = z.object({
  grok: z.object({
    violation_deduction_enabled: z.boolean(),
    violation_deduction_amount: z.coerce
      .number()
      .min(0, 'systemSettings.errors.mustBeNonNegative'),
  }),
})

type GrokFormValues = z.infer<typeof schema>

type FlatGrokSettings = {
  'grok.violation_deduction_enabled': boolean
  'grok.violation_deduction_amount': number
}

type GrokSettingsCardProps = {
  defaultValues: GrokFormValues
}

export function GrokSettingsCard({ defaultValues }: GrokSettingsCardProps) {
  const { t } = useTranslation()
  const updateOption = useUpdateOption()

  const buildDefaults = (values: GrokFormValues): GrokFormValues => ({
    grok: {
      violation_deduction_enabled: values.grok.violation_deduction_enabled,
      violation_deduction_amount: values.grok.violation_deduction_amount,
    },
  })

  const normalizedDefaultsRef = useRef<FlatGrokSettings>({
    'grok.violation_deduction_enabled':
      defaultValues.grok.violation_deduction_enabled,
    'grok.violation_deduction_amount':
      defaultValues.grok.violation_deduction_amount,
  })

  const form = useForm<GrokFormValues>({
    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    resolver: zodResolver(schema) as any,
    defaultValues: buildDefaults(defaultValues),
  })

  useEffect(() => {
    normalizedDefaultsRef.current = {
      'grok.violation_deduction_enabled':
        defaultValues.grok.violation_deduction_enabled,
      'grok.violation_deduction_amount':
        defaultValues.grok.violation_deduction_amount,
    }
    form.reset(buildDefaults(defaultValues))
  }, [defaultValues, form])

  const onSubmit = async (values: GrokFormValues) => {
    const normalized: FlatGrokSettings = {
      'grok.violation_deduction_enabled':
        values.grok.violation_deduction_enabled,
      'grok.violation_deduction_amount': values.grok.violation_deduction_amount,
    }

    const updates = (
      Object.keys(normalized) as Array<keyof FlatGrokSettings>
    ).filter((key) => normalized[key] !== normalizedDefaultsRef.current[key])

    if (updates.length === 0) {
      toast.info(t('channels.fields.noChangesToSave'))
      return
    }

    for (const key of updates) {
      await updateOption.mutateAsync({
        key,
        value: normalized[key],
      })
    }
  }

  const enabled = form.watch('grok.violation_deduction_enabled')

  return (
    <SettingsSection title={t('systemSettings.titles.grokSettings')}>
      <Form {...form}>
        <SettingsForm onSubmit={form.handleSubmit(onSubmit)}>
          <SettingsPageFormActions
            onSave={form.handleSubmit(onSubmit)}
            isSaving={updateOption.isPending}
          />
          <DisabledSettingsNotice enabled={enabled} />

          <FormField
            control={form.control}
            name='grok.violation_deduction_enabled'
            render={({ field }) => (
              <SettingsSwitchItem>
                <SettingsSwitchContent>
                  <FormLabel>
                    {t('systemSettings.actions.enableViolationDeduction')}
                  </FormLabel>
                  <FormDescription>
                    {t(
                      'systemSettings.status.enabledViolationRequestsWillIncurAdditionalCharges'
                    )}{' '}
                    <a
                      href={XAI_VIOLATION_FEE_DOC_URL}
                      target='_blank'
                      rel='noreferrer'
                      className='underline'
                    >
                      {t('systemSettings.fields.officialDocumentation')}
                    </a>
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

          <FormField
            control={form.control}
            name='grok.violation_deduction_amount'
            render={({ field }) => (
              <FormItem className='max-w-xs'>
                <FormLabel>
                  {t('systemSettings.fields.violationDeductionAmount')}
                </FormLabel>
                <FormControl>
                  <Input
                    type='number'
                    step={0.01}
                    min={0}
                    {...field}
                    disabled={!enabled}
                  />
                </FormControl>
                <FormDescription>
                  {t(
                    'systemSettings.tips.baseAmountActualDeductionBaseAmountSystemGroupRate'
                  )}
                </FormDescription>
              </FormItem>
            )}
          />
        </SettingsForm>
      </Form>
    </SettingsSection>
  )
}
