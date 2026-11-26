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
import { Form, FormControl, FormField } from '@/components/ui/form'
import { Switch } from '@/components/ui/switch'
import { Textarea } from '@/components/ui/textarea'
import { SettingsForm } from '../components/settings-form-layout'
import { SettingsGroup, SettingsRow } from '../components/settings-group'
import { SettingsPageFormActions } from '../components/settings-page-context'
import { SettingsSection } from '../components/settings-section'
import { useResetForm } from '../hooks/use-reset-form'
import { useUpdateOption } from '../hooks/use-update-option'

const basicAuthSchema = z.object({
  PasswordLoginEnabled: z.boolean(),
  PasswordRegisterEnabled: z.boolean(),
  EmailVerificationEnabled: z.boolean(),
  RegisterEnabled: z.boolean(),
  EmailDomainRestrictionEnabled: z.boolean(),
  EmailAliasRestrictionEnabled: z.boolean(),
  EmailDomainWhitelist: z.string(),
})

type BasicAuthFormValues = z.infer<typeof basicAuthSchema>

type BasicAuthSectionProps = {
  defaultValues: BasicAuthFormValues
}

export function BasicAuthSection({ defaultValues }: BasicAuthSectionProps) {
  const { t } = useTranslation()
  const updateOption = useUpdateOption()

  const formDefaults = useMemo<BasicAuthFormValues>(
    () => ({
      ...defaultValues,
      EmailDomainWhitelist: defaultValues.EmailDomainWhitelist.split(',')
        .map((domain) => domain.trim())
        .filter(Boolean)
        .join('\n'),
    }),
    [defaultValues]
  )

  const form = useForm<BasicAuthFormValues>({
    resolver: zodResolver(basicAuthSchema),
    defaultValues: formDefaults,
  })

  useResetForm(form, formDefaults)

  const onSubmit = async (data: BasicAuthFormValues) => {
    const updates: Array<{ key: string; value: string | boolean }> = []

    Object.entries(data).forEach(([key, value]) => {
      if (key === 'EmailDomainWhitelist') {
        if (typeof value !== 'string') return
        const domains = value
          .split('\n')
          .map((domain) => domain.trim())
          .filter(Boolean)
          .join(',')
        if (domains !== defaultValues.EmailDomainWhitelist) {
          updates.push({ key, value: domains })
        }
      } else if (value !== defaultValues[key as keyof typeof defaultValues]) {
        updates.push({ key, value })
      }
    })

    for (const update of updates) {
      await updateOption.mutateAsync(update)
    }
  }

  return (
    <SettingsSection title={t('systemSettings.fields.basicAuthentication')}>
      <Form {...form}>
        <SettingsForm onSubmit={form.handleSubmit(onSubmit)}>
          <SettingsPageFormActions
            onSave={form.handleSubmit(onSubmit)}
            isSaving={updateOption.isPending}
          />
          <SettingsGroup>
            <FormField
              control={form.control}
              name='PasswordLoginEnabled'
              render={({ field }) => (
                <SettingsRow
                  label={t('systemSettings.fields.passwordLogin')}
                  description={t(
                    'systemSettings.tips.allowUsersToLogInWithPassword'
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
              name='RegisterEnabled'
              render={({ field }) => (
                <SettingsRow
                  label={t('systemSettings.status.registrationEnabled')}
                  description={t(
                    'systemSettings.titles.allowNewUsersToRegister'
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
              name='PasswordRegisterEnabled'
              render={({ field }) => (
                <SettingsRow
                  label={t('systemSettings.fields.passwordRegistration')}
                  description={t(
                    'systemSettings.tips.allowRegistrationWithPassword'
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
              name='EmailVerificationEnabled'
              render={({ field }) => (
                <SettingsRow
                  label={t('systemSettings.fields.emailVerification')}
                  description={t(
                    'systemSettings.tips.requireEmailVerificationForNewAccounts'
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
              name='EmailDomainRestrictionEnabled'
              render={({ field }) => (
                <SettingsRow
                  label={t('systemSettings.fields.emailDomainRestriction')}
                  description={t(
                    'systemSettings.tips.onlyAllowSpecificEmailDomains'
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
              name='EmailAliasRestrictionEnabled'
              render={({ field }) => (
                <SettingsRow
                  label={t('systemSettings.fields.emailAliasRestriction')}
                  description={t(
                    'systemSettings.tips.blockEmailAliasesEGUserAliasDomainCom'
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
              name='EmailDomainWhitelist'
              render={({ field }) => (
                <SettingsRow
                  label={t('systemSettings.fields.emailDomainWhitelist')}
                  description={t(
                    'systemSettings.placeholders.oneDomainPerLineOnlyUsedWhenDomainRestriction'
                  )}
                >
                  <FormControl>
                    <Textarea
                      placeholder={t(
                        'systemSettings.placeholders.exampleComCompanyCom'
                      )}
                      rows={4}
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
