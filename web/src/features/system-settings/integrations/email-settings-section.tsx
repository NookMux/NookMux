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
import { Form, FormControl, FormField } from '@/components/ui/form'
import { Input } from '@/components/ui/input'
import { Switch } from '@/components/ui/switch'
import { SettingsForm } from '../components/settings-form-layout'
import {
  SettingsControl,
  SettingsGroup,
  SettingsRow,
} from '../components/settings-group'
import { SettingsPageFormActions } from '../components/settings-page-context'
import { SettingsSection } from '../components/settings-section'
import { useResetForm } from '../hooks/use-reset-form'
import { useUpdateOption } from '../hooks/use-update-option'

const createEmailSchema = (t: (key: string) => string) =>
  z.object({
    SMTPServer: z.string(),
    SMTPPort: z.string().refine((value) => {
      const trimmed = value.trim()
      if (!trimmed) return true
      return /^\d+$/.test(trimmed)
    }, t('systemSettings.errors.portMustBeAPositiveInteger')),
    SMTPAccount: z.string(),
    SMTPFrom: z.string().refine((value) => {
      const trimmed = value.trim()
      if (!trimmed) return true
      return /^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(trimmed)
    }, t('systemSettings.placeholders.enterAValidEmailOrLeaveBlank')),
    SMTPToken: z.string(),
    SMTPSSLEnabled: z.boolean(),
    SMTPForceLoginAuthEnabled: z.boolean(),
  })

type EmailFormValues = z.infer<ReturnType<typeof createEmailSchema>>

type EmailSettingsSectionProps = {
  defaultValues: EmailFormValues
}

export function EmailSettingsSection({
  defaultValues,
}: EmailSettingsSectionProps) {
  const { t } = useTranslation()
  const updateOption = useUpdateOption()
  const emailSchema = createEmailSchema(t)

  const form = useForm<EmailFormValues>({
    resolver: zodResolver(emailSchema),
    defaultValues,
  })

  useResetForm(form, defaultValues)

  const onSubmit = async (values: EmailFormValues) => {
    const sanitized = {
      SMTPServer: values.SMTPServer.trim(),
      SMTPPort: values.SMTPPort.trim(),
      SMTPAccount: values.SMTPAccount.trim(),
      SMTPFrom: values.SMTPFrom.trim(),
      SMTPToken: values.SMTPToken.trim(),
      SMTPSSLEnabled: values.SMTPSSLEnabled,
      SMTPForceLoginAuthEnabled: values.SMTPForceLoginAuthEnabled,
    }

    const initial = {
      SMTPServer: defaultValues.SMTPServer.trim(),
      SMTPPort: defaultValues.SMTPPort.trim(),
      SMTPAccount: defaultValues.SMTPAccount.trim(),
      SMTPFrom: defaultValues.SMTPFrom.trim(),
      SMTPToken: defaultValues.SMTPToken.trim(),
      SMTPSSLEnabled: defaultValues.SMTPSSLEnabled,
      SMTPForceLoginAuthEnabled: defaultValues.SMTPForceLoginAuthEnabled,
    }

    const updates: Array<{ key: string; value: string | boolean }> = []

    if (sanitized.SMTPServer !== initial.SMTPServer) {
      updates.push({ key: 'SMTPServer', value: sanitized.SMTPServer })
    }

    if (sanitized.SMTPPort !== initial.SMTPPort) {
      updates.push({ key: 'SMTPPort', value: sanitized.SMTPPort })
    }

    if (sanitized.SMTPAccount !== initial.SMTPAccount) {
      updates.push({ key: 'SMTPAccount', value: sanitized.SMTPAccount })
    }

    if (sanitized.SMTPFrom !== initial.SMTPFrom) {
      updates.push({ key: 'SMTPFrom', value: sanitized.SMTPFrom })
    }

    if (sanitized.SMTPToken && sanitized.SMTPToken !== initial.SMTPToken) {
      updates.push({ key: 'SMTPToken', value: sanitized.SMTPToken })
    }

    if (sanitized.SMTPSSLEnabled !== initial.SMTPSSLEnabled) {
      updates.push({
        key: 'SMTPSSLEnabled',
        value: sanitized.SMTPSSLEnabled,
      })
    }

    if (
      sanitized.SMTPForceLoginAuthEnabled !== initial.SMTPForceLoginAuthEnabled
    ) {
      updates.push({
        key: 'SMTPForceLoginAuthEnabled',
        value: sanitized.SMTPForceLoginAuthEnabled,
      })
    }

    for (const update of updates) {
      await updateOption.mutateAsync(update)
    }
  }

  return (
    <SettingsSection title={t('systemSettings.fields.smtpEmail')}>
      <Form {...form}>
        <SettingsForm onSubmit={form.handleSubmit(onSubmit)} autoComplete='off'>
          <SettingsPageFormActions
            onSave={form.handleSubmit(onSubmit)}
            isSaving={updateOption.isPending}
            saveLabel='common.actions.saveSmtpSettings'
          />
          <SettingsGroup>
            <FormField
              control={form.control}
              name='SMTPServer'
              render={({ field }) => (
                <SettingsRow
                  label={t('systemSettings.fields.smtpHost')}
                  description={t(
                    'systemSettings.tips.hostnameOrIpOfYourSmtpProvider'
                  )}
                >
                  <SettingsControl>
                    <FormControl>
                      <Input
                        autoComplete='off'
                        placeholder={t(
                          'systemSettings.placeholders.smtpExampleCom'
                        )}
                        {...field}
                        onChange={(event) => field.onChange(event.target.value)}
                      />
                    </FormControl>
                  </SettingsControl>
                </SettingsRow>
              )}
            />

            <FormField
              control={form.control}
              name='SMTPPort'
              render={({ field }) => (
                <SettingsRow
                  label={t('models.fields.port')}
                  description={t(
                    'systemSettings.tips.commonPortsInclude25465And587'
                  )}
                >
                  <SettingsControl size='xs'>
                    <FormControl>
                      <Input
                        autoComplete='off'
                        type='number'
                        placeholder='587'
                        {...field}
                        onChange={(event) => field.onChange(event.target.value)}
                      />
                    </FormControl>
                  </SettingsControl>
                </SettingsRow>
              )}
            />

            <FormField
              control={form.control}
              name='SMTPSSLEnabled'
              render={({ field }) => (
                <SettingsRow
                  label={t('systemSettings.actions.enableSslTls')}
                  description={t(
                    'systemSettings.actions.useSecureConnectionWhenSendingEmails'
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
              name='SMTPForceLoginAuthEnabled'
              render={({ field }) => (
                <SettingsRow
                  label={t('systemSettings.fields.forceAuthLogin')}
                  description={t(
                    'systemSettings.tips.alwaysUseSmtpLoginAuthForOutboundMail'
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
              name='SMTPAccount'
              render={({ field }) => (
                <SettingsRow
                  label={t('auth.fields.username')}
                  description={t(
                    'systemSettings.status.accountUsedWhenAuthenticatingWithTheSmtpServer'
                  )}
                >
                  <SettingsControl>
                    <FormControl>
                      <Input
                        autoComplete='off'
                        placeholder={t(
                          'systemSettings.placeholders.noreplyExampleCom'
                        )}
                        {...field}
                        onChange={(event) => field.onChange(event.target.value)}
                      />
                    </FormControl>
                  </SettingsControl>
                </SettingsRow>
              )}
            />

            <FormField
              control={form.control}
              name='SMTPFrom'
              render={({ field }) => (
                <SettingsRow
                  label={t('systemSettings.fields.address')}
                  description={t(
                    'systemSettings.status.displayNameAndEmailUsedInOutgoingMessages'
                  )}
                >
                  <SettingsControl>
                    <FormControl>
                      <Input
                        autoComplete='off'
                        placeholder={t(
                          'systemSettings.tips.newApiNoreplyExampleCom'
                        )}
                        {...field}
                        onChange={(event) => field.onChange(event.target.value)}
                      />
                    </FormControl>
                  </SettingsControl>
                </SettingsRow>
              )}
            />

            <FormField
              control={form.control}
              name='SMTPToken'
              render={({ field }) => (
                <SettingsRow
                  label={t('systemSettings.fields.passwordAccessToken')}
                  description={t(
                    'systemSettings.tips.leaveBlankToKeepTheExistingCredential'
                  )}
                >
                  <SettingsControl>
                    <FormControl>
                      <Input
                        autoComplete='off'
                        type='password'
                        placeholder={t(
                          'systemSettings.placeholders.enterNewTokenToUpdate'
                        )}
                        {...field}
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
