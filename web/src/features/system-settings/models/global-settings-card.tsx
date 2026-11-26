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
import * as z from 'zod'
import { useForm } from 'react-hook-form'
import { zodResolver } from '@hookform/resolvers/zod'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { Form, FormControl, FormField } from '@/components/ui/form'
import { Input } from '@/components/ui/input'
import { SettingsForm } from '../components/settings-form-layout'
import {
  SettingsControl,
  SettingsGroup,
  SettingsGroupSwitch,
  SettingsRow,
} from '../components/settings-group'
import { SettingsPageFormActions } from '../components/settings-page-context'
import { SettingsSection } from '../components/settings-section'
import { useResetForm } from '../hooks/use-reset-form'
import { useUpdateOption } from '../hooks/use-update-option'

const schema = z.object({
  general_setting: z.object({
    ping_interval_enabled: z.boolean(),
    ping_interval_seconds: z
      .number()
      .min(1, 'systemSettings.errors.mustBeAtLeast1'),
  }),
})

type GlobalModelSettingsFormValues = z.infer<typeof schema>

type FlatGlobalModelSettings = {
  'general_setting.ping_interval_enabled': boolean
  'general_setting.ping_interval_seconds': number
}

const flattenGlobalValues = (
  values: GlobalModelSettingsFormValues
): FlatGlobalModelSettings => ({
  'general_setting.ping_interval_enabled':
    values.general_setting.ping_interval_enabled,
  'general_setting.ping_interval_seconds':
    values.general_setting.ping_interval_seconds,
})

type GlobalSettingsCardProps = {
  defaultValues: GlobalModelSettingsFormValues
}

export function GlobalSettingsCard({ defaultValues }: GlobalSettingsCardProps) {
  const { t } = useTranslation()
  const updateOption = useUpdateOption()

  const form = useForm<GlobalModelSettingsFormValues>({
    resolver: zodResolver(schema),
    defaultValues,
  })

  useResetForm(form, defaultValues)

  const pingEnabled = form.watch('general_setting.ping_interval_enabled')

  const onSubmit = async (values: GlobalModelSettingsFormValues) => {
    const flattenedDefaults = flattenGlobalValues(defaultValues)
    const flattenedValues = flattenGlobalValues(values)
    const updates = Object.entries(flattenedValues).filter(
      ([key, value]) =>
        value !== flattenedDefaults[key as keyof FlatGlobalModelSettings]
    )

    if (updates.length === 0) {
      toast.info(t('channels.fields.noChangesToSave'))
      return
    }

    for (const [key, value] of updates) {
      await updateOption.mutateAsync({ key, value })
    }
  }

  return (
    <SettingsSection
      title={t('systemSettings.titles.globalModelConfiguration')}
    >
      <Form {...form}>
        <SettingsForm onSubmit={form.handleSubmit(onSubmit)}>
          <SettingsPageFormActions
            onSave={form.handleSubmit(onSubmit)}
            isSaving={updateOption.isPending}
          />

          <SettingsGroup
            title={t('systemSettings.fields.keepAlivePing')}
            description={t(
              'systemSettings.status.periodicallySendPingFramesToKeepStreamingConnectionsActive'
            )}
            control={
              <SettingsGroupSwitch name='general_setting.ping_interval_enabled' />
            }
            enabled={pingEnabled}
          >
            <FormField
              control={form.control}
              name='general_setting.ping_interval_seconds'
              render={({ field }) => (
                <SettingsRow
                  label={t('systemSettings.fields.pingIntervalSeconds')}
                  description={t(
                    'systemSettings.tips.recommendedToKeepThisHighToAvoidUpstreamThrottling'
                  )}
                >
                  <SettingsControl>
                    <FormControl>
                      <Input
                        type='number'
                        min={1}
                        value={
                          field.value === undefined || field.value === null
                            ? ''
                            : String(field.value)
                        }
                        onChange={(event) => field.onChange(event.target.value)}
                        onBlur={field.onBlur}
                        name={field.name}
                        ref={field.ref}
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
