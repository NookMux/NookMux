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
import { useEffect } from 'react'
import * as z from 'zod'
import { useForm } from 'react-hook-form'
import { zodResolver } from '@hookform/resolvers/zod'
import { useTranslation } from 'react-i18next'
import { Form, FormControl, FormField } from '@/components/ui/form'
import { Switch } from '@/components/ui/switch'
import { Textarea } from '@/components/ui/textarea'
import { SettingsForm } from '../components/settings-form-layout'
import {
  SettingsGroup,
  SettingsGroupSwitch,
  SettingsRow,
} from '../components/settings-group'
import { SettingsPageFormActions } from '../components/settings-page-context'
import { SettingsSection } from '../components/settings-section'
import { useUpdateOption } from '../hooks/use-update-option'

const sensitiveSchema = z.object({
  CheckSensitiveEnabled: z.boolean(),
  CheckSensitiveOnPromptEnabled: z.boolean(),
  SensitiveWords: z.string().optional(),
})

type SensitiveFormValues = z.infer<typeof sensitiveSchema>

type SensitiveWordsSectionProps = {
  defaultValues: SensitiveFormValues
}

export function SensitiveWordsSection({
  defaultValues,
}: SensitiveWordsSectionProps) {
  const { t } = useTranslation()
  const updateOption = useUpdateOption()
  const form = useForm<SensitiveFormValues>({
    resolver: zodResolver(sensitiveSchema),
    defaultValues,
  })

  useEffect(() => {
    form.reset(defaultValues)
  }, [defaultValues, form])

  const filteringEnabled = form.watch('CheckSensitiveEnabled')

  const onSubmit = async (values: SensitiveFormValues) => {
    const updates = Object.entries(values).filter(
      ([key, value]) =>
        value !== defaultValues[key as keyof SensitiveFormValues]
    )

    for (const [key, value] of updates) {
      await updateOption.mutateAsync({ key, value: value ?? '' })
    }
  }

  return (
    <SettingsSection title={t('systemSettings.fields.sensitiveWords')}>
      <Form {...form}>
        <SettingsForm onSubmit={form.handleSubmit(onSubmit)}>
          <SettingsPageFormActions
            onSave={form.handleSubmit(onSubmit)}
            isSaving={updateOption.isPending}
            saveLabel='common.actions.saveSensitiveWords'
          />
          <SettingsGroup
            title={t('systemSettings.actions.enableFiltering')}
            description={t(
              'systemSettings.tips.blocksMessagesWhenSensitiveKeywordsAreDetected'
            )}
            control={<SettingsGroupSwitch name='CheckSensitiveEnabled' />}
            enabled={filteringEnabled}
          >
            <FormField
              control={form.control}
              name='CheckSensitiveOnPromptEnabled'
              render={({ field }) => (
                <SettingsRow
                  label={t('systemSettings.fields.inspectUserPrompts')}
                  description={t(
                    'systemSettings.status.enabledPromptsAreScannedBeforeReachingUpstreamModels'
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
              name='SensitiveWords'
              render={({ field }) => (
                <SettingsRow
                  label={t('systemSettings.fields.blockedKeywords')}
                  description={t(
                    'systemSettings.tips.eachLineRepresentsOneKeywordLeaveBlankToDisable'
                  )}
                >
                  <FormControl>
                    <Textarea
                      rows={12}
                      placeholder={t(
                        'systemSettings.placeholders.enterOneKeywordPerLine'
                      )}
                      {...field}
                    />
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
