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
import { removeTrailingSlash } from './utils'

const createWorkerSchema = (t: (key: string) => string) =>
  z.object({
    WorkerUrl: z.string().refine((value) => {
      const trimmed = value.trim()
      if (!trimmed) return true
      return /^https?:\/\//.test(trimmed)
    }, t('systemSettings.tips.provideAValidUrlStartingWithUrlOrUrl')),
    WorkerValidKey: z.string(),
    WorkerAllowHttpImageRequestEnabled: z.boolean(),
  })

type WorkerFormValues = z.infer<ReturnType<typeof createWorkerSchema>>

type WorkerSettingsSectionProps = {
  defaultValues: WorkerFormValues
}

export function WorkerSettingsSection({
  defaultValues,
}: WorkerSettingsSectionProps) {
  const { t } = useTranslation()
  const updateOption = useUpdateOption()
  const workerSchema = createWorkerSchema(t)

  const form = useForm<WorkerFormValues>({
    resolver: zodResolver(workerSchema),
    defaultValues,
  })

  useResetForm(form, defaultValues)

  const onSubmit = async (values: WorkerFormValues) => {
    const sanitizedUrl = removeTrailingSlash(values.WorkerUrl)
    const sanitizedKey = values.WorkerValidKey.trim()
    const initialUrl = removeTrailingSlash(defaultValues.WorkerUrl)
    const initialKey = defaultValues.WorkerValidKey.trim()

    const updates: Array<{ key: string; value: string | boolean }> = []

    if (sanitizedUrl !== initialUrl) {
      updates.push({ key: 'WorkerUrl', value: sanitizedUrl })
    }

    if (sanitizedKey !== initialKey || sanitizedUrl === '') {
      updates.push({ key: 'WorkerValidKey', value: sanitizedKey })
    }

    if (
      values.WorkerAllowHttpImageRequestEnabled !==
      defaultValues.WorkerAllowHttpImageRequestEnabled
    ) {
      updates.push({
        key: 'WorkerAllowHttpImageRequestEnabled',
        value: values.WorkerAllowHttpImageRequestEnabled,
      })
    }

    for (const update of updates) {
      await updateOption.mutateAsync(update)
    }
  }

  return (
    <SettingsSection title={t('systemSettings.fields.workerProxy')}>
      <Form {...form}>
        <SettingsForm onSubmit={form.handleSubmit(onSubmit)} autoComplete='off'>
          <SettingsPageFormActions
            onSave={form.handleSubmit(onSubmit)}
            isSaving={updateOption.isPending}
            saveLabel='common.actions.saveWorkerSettings'
          />
          <SettingsGroup>
            <FormField
              control={form.control}
              name='WorkerUrl'
              render={({ field }) => (
                <SettingsRow
                  label={t('systemSettings.fields.workerUrl')}
                  description={t(
                    'systemSettings.status.requestsWillBeForwardedToThisWorkerTrailingSlashes'
                  )}
                >
                  <SettingsControl>
                    <FormControl>
                      <Input
                        type='url'
                        inputMode='url'
                        placeholder={t(
                          'systemSettings.placeholders.urlWorkerExampleWorkersDev'
                        )}
                        autoComplete='off'
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
              name='WorkerValidKey'
              render={({ field }) => (
                <SettingsRow
                  label={t('systemSettings.fields.workerAccessKey')}
                  description={t(
                    'systemSettings.status.usedToAuthenticateWithTheWorkerLeaveBlankTo'
                  )}
                >
                  <SettingsControl>
                    <FormControl>
                      <Input
                        type='password'
                        placeholder={t(
                          'systemSettings.placeholders.enterNewKeyToUpdate'
                        )}
                        autoComplete='new-password'
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
              name='WorkerAllowHttpImageRequestEnabled'
              render={({ field }) => (
                <SettingsRow
                  label={t('systemSettings.fields.allowHttpImageRequests')}
                  description={t(
                    'systemSettings.actions.enableWhenProxyingWorkersThatFetchImagesOverHttp'
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
          </SettingsGroup>
        </SettingsForm>
      </Form>
    </SettingsSection>
  )
}
