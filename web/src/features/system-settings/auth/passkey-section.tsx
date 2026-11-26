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
import { Input } from '@/components/ui/input'
import {
  Select,
  SelectContent,
  SelectGroup,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { Switch } from '@/components/ui/switch'
import { Textarea } from '@/components/ui/textarea'
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

const passkeySchema = z.object({
  'passkey.enabled': z.boolean(),
  'passkey.rp_display_name': z.string(),
  'passkey.rp_id': z.string(),
  'passkey.origins': z.string(),
  'passkey.allow_insecure_origin': z.boolean(),
  'passkey.user_verification': z.enum(['required', 'preferred', 'discouraged']),
  'passkey.attachment_preference': z.enum([
    'none',
    'platform',
    'cross-platform',
  ]),
  'passkey.max_passkeys_per_user': z
    .number()
    .int('common.errors.mustBeAnInteger')
    .min(1, 'common.errors.mustBeBetween1And20')
    .max(20, 'common.errors.mustBeBetween1And20'),
})

type PasskeyFormValues = z.infer<typeof passkeySchema>

interface PasskeySectionProps {
  defaultValues: PasskeyFormValues
}

export function PasskeySection({ defaultValues }: PasskeySectionProps) {
  const { t } = useTranslation()
  const updateOption = useUpdateOption()

  const formDefaults = useMemo<PasskeyFormValues>(
    () => ({
      ...defaultValues,
      'passkey.origins': (defaultValues['passkey.origins'] as string)
        .split(',')
        .map((origin: string) => origin.trim())
        .filter(Boolean)
        .join('\n'),
      'passkey.attachment_preference':
        (defaultValues['passkey.attachment_preference'] as string) === ''
          ? 'none'
          : (defaultValues['passkey.attachment_preference'] as
              | 'platform'
              | 'cross-platform'),
    }),
    [defaultValues]
  )

  const form = useForm<PasskeyFormValues>({
    resolver: zodResolver(passkeySchema),
    defaultValues: formDefaults,
  })
  const enabled = form.watch('passkey.enabled')

  useResetForm(form, formDefaults)

  const onSubmit = async () => {
    const rawData = form.getValues() as Record<string, unknown>
    const flattenedEntries: Array<
      [keyof PasskeyFormValues, PasskeyFormValues[keyof PasskeyFormValues]]
    > = []

    Object.entries(rawData).forEach(([key, value]) => {
      if (key === 'passkey' && value && typeof value === 'object') {
        Object.entries(value as Record<string, unknown>).forEach(
          ([nestedKey, nestedValue]) => {
            flattenedEntries.push([
              `passkey.${nestedKey}` as keyof PasskeyFormValues,
              nestedValue as PasskeyFormValues[keyof PasskeyFormValues],
            ])
          }
        )
      } else {
        flattenedEntries.push([
          key as keyof PasskeyFormValues,
          value as PasskeyFormValues[keyof PasskeyFormValues],
        ])
      }
    })

    const data = Object.fromEntries(flattenedEntries) as PasskeyFormValues
    const updates: Array<{ key: string; value: string | boolean | number }> = []

    Object.entries(data).forEach(([key, value]) => {
      if (key === 'passkey.origins') {
        const processed = (value as string)
          .split('\n')
          .map((origin: string) => origin.trim())
          .filter(Boolean)
          .join(',')
        const currentDefault = defaultValues['passkey.origins'] as string
        if (processed !== currentDefault) {
          updates.push({ key, value: processed })
        }
      } else if (key === 'passkey.attachment_preference') {
        const attachmentPreference =
          value as PasskeyFormValues['passkey.attachment_preference']
        const incoming =
          attachmentPreference === 'none' ? '' : attachmentPreference
        const currentDefault =
          defaultValues['passkey.attachment_preference'] === 'none'
            ? ''
            : defaultValues['passkey.attachment_preference']
        if (incoming !== currentDefault) {
          updates.push({ key, value: incoming })
        }
      } else if (value !== defaultValues[key as keyof PasskeyFormValues]) {
        updates.push({ key, value })
      }
    })

    for (const update of updates) {
      await updateOption.mutateAsync(update)
    }
  }

  return (
    <SettingsSection title={t('systemSettings.fields.passkeyAuthentication')}>
      <Form {...form}>
        <SettingsForm onSubmit={form.handleSubmit(onSubmit)}>
          <SettingsPageFormActions
            onSave={form.handleSubmit(onSubmit)}
            isSaving={updateOption.isPending}
          />
          <SettingsGroup
            title={t('profile.actions.enablePasskey')}
            description={t(
              'systemSettings.tips.allowUsersToRegisterAndSignInWithPasskey'
            )}
            control={<SettingsGroupSwitch name='passkey.enabled' />}
            enabled={enabled}
          >
            <FormField
              control={form.control}
              name='passkey.rp_display_name'
              render={({ field }) => (
                <SettingsRow
                  label={t('systemSettings.fields.relyingPartyDisplayName')}
                  description={t(
                    'systemSettings.tips.humanReadableNameShownToUsersDuringPasskeyPrompts'
                  )}
                >
                  <SettingsControl>
                    <FormControl>
                      <Input
                        placeholder={t(
                          'systemSettings.placeholders.eGNewApiConsole'
                        )}
                        {...field}
                        value={field.value ?? ''}
                      />
                    </FormControl>
                  </SettingsControl>
                </SettingsRow>
              )}
            />

            <FormField
              control={form.control}
              name='passkey.rp_id'
              render={({ field }) => (
                <SettingsRow
                  label={t('systemSettings.fields.relyingPartyId')}
                  description={t(
                    'systemSettings.errors.effectiveDomainForPasskeyRegistrationMustMatchTheCurrent'
                  )}
                >
                  <SettingsControl>
                    <FormControl>
                      <Input
                        placeholder={t(
                          'systemSettings.placeholders.eGExampleCom'
                        )}
                        {...field}
                        value={field.value ?? ''}
                      />
                    </FormControl>
                  </SettingsControl>
                </SettingsRow>
              )}
            />

            <FormField
              control={form.control}
              name='passkey.user_verification'
              render={({ field }) => (
                <SettingsRow
                  label={t('systemSettings.fields.userVerification')}
                  description={t(
                    'systemSettings.errors.controlsWhetherUserVerificationBiometricsPinIsRequiredDuring'
                  )}
                >
                  <SettingsControl size='sm'>
                    <FormControl>
                      <Select
                        items={[
                          {
                            value: 'required',
                            label: t(
                              'systemSettings.fields.userVerificationRequired'
                            ),
                          },
                          {
                            value: 'preferred',
                            label: t('systemSettings.tips.recommended'),
                          },
                          {
                            value: 'discouraged',
                            label: t('systemSettings.fields.discouraged'),
                          },
                        ]}
                        value={field.value}
                        onValueChange={field.onChange}
                      >
                        <SelectTrigger>
                          <SelectValue
                            placeholder={t(
                              'systemSettings.placeholders.selectRequirement'
                            )}
                          />
                        </SelectTrigger>
                        <SelectContent alignItemWithTrigger={false}>
                          <SelectGroup>
                            <SelectItem value='required'>
                              {t(
                                'systemSettings.fields.userVerificationRequired'
                              )}
                            </SelectItem>
                            <SelectItem value='preferred'>
                              {t('systemSettings.tips.recommended')}
                            </SelectItem>
                            <SelectItem value='discouraged'>
                              {t('systemSettings.fields.discouraged')}
                            </SelectItem>
                          </SelectGroup>
                        </SelectContent>
                      </Select>
                    </FormControl>
                  </SettingsControl>
                </SettingsRow>
              )}
            />

            <FormField
              control={form.control}
              name='passkey.attachment_preference'
              render={({ field }) => (
                <SettingsRow
                  label={t('systemSettings.fields.deviceTypePreference')}
                  description={t(
                    'systemSettings.tips.builtInPhoneFingerprintFaceOrWindowsHelloExternal'
                  )}
                >
                  <SettingsControl size='sm'>
                    <FormControl>
                      <Select
                        items={[
                          {
                            value: 'none',
                            label: t('keyQuery.fields.unlimited'),
                          },
                          {
                            value: 'platform',
                            label: t('systemSettings.fields.builtInDevice'),
                          },
                          {
                            value: 'cross-platform',
                            label: t('systemSettings.fields.externalDevice'),
                          },
                        ]}
                        value={field.value}
                        onValueChange={field.onChange}
                      >
                        <SelectTrigger>
                          <SelectValue
                            placeholder={t(
                              'systemSettings.fields.noPreference'
                            )}
                          />
                        </SelectTrigger>
                        <SelectContent alignItemWithTrigger={false}>
                          <SelectGroup>
                            <SelectItem value='none'>
                              {t('keyQuery.fields.unlimited')}
                            </SelectItem>
                            <SelectItem value='platform'>
                              {t('systemSettings.fields.builtInDevice')}
                            </SelectItem>
                            <SelectItem value='cross-platform'>
                              {t('systemSettings.fields.externalDevice')}
                            </SelectItem>
                          </SelectGroup>
                        </SelectContent>
                      </Select>
                    </FormControl>
                  </SettingsControl>
                </SettingsRow>
              )}
            />

            <FormField
              control={form.control}
              name='passkey.allow_insecure_origin'
              render={({ field }) => (
                <SettingsRow
                  label={t('systemSettings.fields.allowInsecureOrigins')}
                  description={t(
                    'systemSettings.tips.permitPasskeyRegistrationOnNonHttpsOriginsOnlyRecommended'
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
              name='passkey.max_passkeys_per_user'
              render={({ field }) => (
                <SettingsRow
                  label={t('systemSettings.fields.maxPasskeysPerUser')}
                  description={t(
                    'systemSettings.tips.maximumNumberOfPasskeysEachUserCanRegister1'
                  )}
                >
                  <SettingsControl>
                    <FormControl>
                      <Input
                        type='number'
                        min={1}
                        max={20}
                        {...field}
                        value={field.value ?? 1}
                        onChange={(e) =>
                          field.onChange(parseInt(e.target.value) || 1)
                        }
                      />
                    </FormControl>
                  </SettingsControl>
                </SettingsRow>
              )}
            />

            <FormField
              control={form.control}
              name='passkey.origins'
              render={({ field }) => (
                <SettingsRow
                  label={t('systemSettings.fields.allowedOrigins')}
                  description={t(
                    'systemSettings.tips.listOfOriginsOnePerLineAllowedForPasskey'
                  )}
                >
                  <FormControl>
                    <Textarea
                      rows={4}
                      placeholder={t(
                        'systemSettings.placeholders.urlExampleCom'
                      )}
                      {...field}
                      value={field.value ?? ''}
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
