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
import type { Resolver } from 'react-hook-form'
import { zodResolver } from '@hookform/resolvers/zod'
import { useTranslation } from 'react-i18next'
import { Form, FormControl, FormField } from '@/components/ui/form'
import { Input } from '@/components/ui/input'
import { Textarea } from '@/components/ui/textarea'
import { FormNavigationGuard } from '../components/form-navigation-guard'
import { SettingsForm } from '../components/settings-form-layout'
import {
  SettingsControl,
  SettingsGroup,
  SettingsRow,
} from '../components/settings-group'
import { SettingsPageFormActions } from '../components/settings-page-context'
import { SettingsSection } from '../components/settings-section'
import { useSettingsForm } from '../hooks/use-settings-form'
import { useUpdateOption } from '../hooks/use-update-option'

const _systemInfoSchema = z.object({
  SystemName: z.string().min(1),
  ServerAddress: z.string().optional(),
  Logo: z.string().url().optional().or(z.literal('')),
  Footer: z.string().optional(),
  About: z.string().optional(),
  HomePageContent: z.string().optional(),
  legal: z.object({
    user_agreement: z.string().optional(),
    privacy_policy: z.string().optional(),
  }),
  general_setting: z.object({
    docs_link: z.string().optional(),
    token_key_prefix: z
      .string()
      .min(1)
      .max(16)
      .regex(/^[^\s%]+-$/),
  }),
})

type SystemInfoFormValues = z.infer<typeof _systemInfoSchema>

type SystemInfoSectionProps = {
  defaultValues: SystemInfoFormValues
}

function normalizeValue(value: unknown): string {
  if (value === undefined || value === null) return ''
  return typeof value === 'string' ? value : String(value)
}

export function SystemInfoSection({ defaultValues }: SystemInfoSectionProps) {
  const { t } = useTranslation()
  const updateOption = useUpdateOption()

  const normalizedDefaults: SystemInfoFormValues = {
    SystemName: normalizeValue(defaultValues.SystemName),
    ServerAddress: normalizeValue(defaultValues.ServerAddress),
    Logo: normalizeValue(defaultValues.Logo),
    Footer: normalizeValue(defaultValues.Footer),
    About: normalizeValue(defaultValues.About),
    HomePageContent: normalizeValue(defaultValues.HomePageContent),
    legal: {
      user_agreement: normalizeValue(defaultValues.legal?.user_agreement),
      privacy_policy: normalizeValue(defaultValues.legal?.privacy_policy),
    },
    general_setting: {
      docs_link: normalizeValue(defaultValues.general_setting?.docs_link),
      token_key_prefix: normalizeValue(
        defaultValues.general_setting?.token_key_prefix
      ),
    },
  }

  const systemInfoSchemaWithI18n = z.object({
    SystemName: z.string().min(1, {
      error: () => t('systemSettings.errors.nameIsRequired'),
    }),
    ServerAddress: z.string().optional(),
    Logo: z
      .string()
      .url({
        error: () => t('systemSettings.errors.mustBeAValidUrl'),
      })
      .optional()
      .or(z.literal('')),
    Footer: z.string().optional(),
    About: z.string().optional(),
    HomePageContent: z.string().optional(),
    legal: z.object({
      user_agreement: z.string().optional(),
      privacy_policy: z.string().optional(),
    }),
    general_setting: z.object({
      docs_link: z.string().optional(),
      token_key_prefix: z
        .string()
        .min(1, {
          error: () => t('systemSettings.errors.apiKeyPrefixRequired'),
        })
        .max(16, {
          error: () => t('systemSettings.errors.apiKeyPrefixInvalid'),
        })
        .regex(/^[^\s%]+-$/, {
          error: () => t('systemSettings.errors.apiKeyPrefixInvalid'),
        }),
    }),
  })

  const { form, handleSubmit, handleReset, isDirty, isSubmitting } =
    useSettingsForm<SystemInfoFormValues>({
      resolver: zodResolver(systemInfoSchemaWithI18n) as Resolver<
        SystemInfoFormValues,
        unknown,
        SystemInfoFormValues
      >,
      defaultValues: normalizedDefaults,
      onSubmit: async (_data, changedFields) => {
        for (const [key, value] of Object.entries(changedFields)) {
          let v = normalizeValue(value)
          if (key === 'ServerAddress') {
            v = v.replace(/\/+$/, '')
          }
          await updateOption.mutateAsync({
            key,
            value: v,
          })
        }
      },
    })

  return (
    <>
      <FormNavigationGuard when={isDirty} />

      <SettingsSection title={t('systemSettings.titles.information')}>
        <Form {...form}>
          <SettingsForm onSubmit={handleSubmit}>
            <SettingsPageFormActions
              onSave={handleSubmit}
              onReset={handleReset}
              isSaving={isSubmitting || updateOption.isPending}
            />
            <SettingsGroup>
              <FormField
                control={form.control}
                name='SystemName'
                render={({ field }) => (
                  <SettingsRow
                    label={t('systemSettings.titles.name')}
                    description={t(
                      'systemSettings.tips.nameDisplayedAcrossTheApplication'
                    )}
                  >
                    <SettingsControl>
                      <FormControl>
                        <Input
                          placeholder={t('layout.fields.newApi')}
                          {...field}
                        />
                      </FormControl>
                    </SettingsControl>
                  </SettingsRow>
                )}
              />

              <FormField
                control={form.control}
                name='ServerAddress'
                render={({ field }) => (
                  <SettingsRow
                    label={t('systemSettings.fields.serverAddress')}
                    description={t(
                      'systemSettings.status.publicUrlOfYourServerUsedForOauthCallbacks'
                    )}
                  >
                    <SettingsControl>
                      <FormControl>
                        <Input
                          placeholder='https://yourdomain.com'
                          {...field}
                        />
                      </FormControl>
                    </SettingsControl>
                  </SettingsRow>
                )}
              />

              <FormField
                control={form.control}
                name='general_setting.docs_link'
                render={({ field }) => (
                  <SettingsRow
                    label={t('systemSettings.fields.documentationLink')}
                    description={t(
                      'systemSettings.fields.linkToYourDocumentationSite'
                    )}
                  >
                    <SettingsControl>
                      <FormControl>
                        <Input
                          placeholder={t(
                            'systemSettings.placeholders.urlDocsExampleCom'
                          )}
                          {...field}
                        />
                      </FormControl>
                    </SettingsControl>
                  </SettingsRow>
                )}
              />

              <FormField
                control={form.control}
                name='general_setting.token_key_prefix'
                render={({ field }) => (
                  <SettingsRow
                    label={t('systemSettings.fields.apiKeyPrefix')}
                    description={t('systemSettings.tips.apiKeyPrefixDesc')}
                  >
                    <SettingsControl>
                      <FormControl>
                        <Input
                          placeholder={t(
                            'systemSettings.placeholders.apiKeyPrefix'
                          )}
                          {...field}
                        />
                      </FormControl>
                    </SettingsControl>
                  </SettingsRow>
                )}
              />

              <FormField
                control={form.control}
                name='Logo'
                render={({ field }) => (
                  <SettingsRow
                    label={t('systemSettings.fields.logoUrl')}
                    description={t(
                      'systemSettings.tips.urlToYourLogoImageOptional'
                    )}
                  >
                    <SettingsControl>
                      <FormControl>
                        <Input
                          placeholder={t(
                            'systemSettings.placeholders.urlExampleComLogoPng'
                          )}
                          {...field}
                        />
                      </FormControl>
                    </SettingsControl>
                  </SettingsRow>
                )}
              />

              <FormField
                control={form.control}
                name='Footer'
                render={({ field }) => (
                  <SettingsRow
                    label={t('systemSettings.fields.footer')}
                    description={t(
                      'systemSettings.tips.footerTextDisplayedAtTheBottomOfPages'
                    )}
                  >
                    <FormControl>
                      <Textarea
                        placeholder={t(
                          'systemSettings.tips.value2025YourCompanyAllRightsReserved'
                        )}
                        rows={4}
                        {...field}
                      />
                    </FormControl>
                  </SettingsRow>
                )}
              />

              <FormField
                control={form.control}
                name='About'
                render={({ field }) => (
                  <SettingsRow
                    label={t('about.titles.value')}
                    description={t(
                      'systemSettings.tips.supportsHtmlMarkupOrIframeEmbeddingEnterHtmlCode'
                    )}
                  >
                    <FormControl>
                      <Textarea
                        placeholder={t(
                          'systemSettings.placeholders.enterHtmlCodeEGPAboutUsP'
                        )}
                        rows={4}
                        {...field}
                      />
                    </FormControl>
                  </SettingsRow>
                )}
              />

              <FormField
                control={form.control}
                name='HomePageContent'
                render={({ field }) => (
                  <SettingsRow
                    label={t('systemSettings.fields.homePageContent')}
                    description={t(
                      'systemSettings.tips.contentDisplayedOnTheHomePageSupportsMarkdown'
                    )}
                  >
                    <FormControl>
                      <Textarea
                        placeholder={t(
                          'systemSettings.tips.welcomeToOurNewApi'
                        )}
                        rows={6}
                        {...field}
                      />
                    </FormControl>
                  </SettingsRow>
                )}
              />

              <FormField
                control={form.control}
                name='legal.user_agreement'
                render={({ field }) => (
                  <SettingsRow
                    label={t('layout.fields.userAgreement')}
                    description={t(
                      'systemSettings.tips.leaveEmptyToDisableTheAgreementRequirementSupportsMarkdown'
                    )}
                  >
                    <FormControl>
                      <Textarea
                        placeholder={t(
                          'systemSettings.tips.provideMarkdownHtmlOrAnExternalUrlForThebface8'
                        )}
                        rows={6}
                        {...field}
                      />
                    </FormControl>
                  </SettingsRow>
                )}
              />

              <FormField
                control={form.control}
                name='legal.privacy_policy'
                render={({ field }) => (
                  <SettingsRow
                    label={t('layout.fields.privacyPolicy')}
                    description={t(
                      'systemSettings.tips.leaveEmptyToDisableThePrivacyPolicyRequirementSupports'
                    )}
                  >
                    <FormControl>
                      <Textarea
                        placeholder={t(
                          'systemSettings.tips.provideMarkdownHtmlOrAnExternalUrlForThe'
                        )}
                        rows={6}
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
    </>
  )
}
