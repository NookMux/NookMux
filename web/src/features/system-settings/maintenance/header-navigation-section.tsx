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
import { useEffect, useMemo } from 'react'
import * as z from 'zod'
import { useForm } from 'react-hook-form'
import { zodResolver } from '@hookform/resolvers/zod'
import { useTranslation } from 'react-i18next'
import { Form, FormControl, FormField } from '@/components/ui/form'
import { Switch } from '@/components/ui/switch'
import { SettingsForm } from '../components/settings-form-layout'
import {
  SettingsGroup,
  SettingsGroupSwitch,
  SettingsRow,
} from '../components/settings-group'
import { SettingsPageFormActions } from '../components/settings-page-context'
import { SettingsSection } from '../components/settings-section'
import { useUpdateOption } from '../hooks/use-update-option'
import {
  HEADER_NAV_DEFAULT,
  type HeaderNavModulesConfig,
  serializeHeaderNavModules,
} from './config'

const headerNavSchema = z.object({
  home: z.boolean(),
  console: z.boolean(),
  key_query: z.boolean(),
  pricingEnabled: z.boolean(),
  pricingRequireAuth: z.boolean(),
  rankingsEnabled: z.boolean(),
  rankingsRequireAuth: z.boolean(),
  docs: z.boolean(),
  about: z.boolean(),
})

type HeaderNavFormValues = z.infer<typeof headerNavSchema>

type HeaderNavigationSectionProps = {
  config: HeaderNavModulesConfig
  initialSerialized: string
}

const toFormValues = (config: HeaderNavModulesConfig): HeaderNavFormValues => ({
  home:
    config.home === undefined ? HEADER_NAV_DEFAULT.home : Boolean(config.home),
  console:
    config.console === undefined
      ? HEADER_NAV_DEFAULT.console
      : Boolean(config.console),
  key_query:
    config.key_query === undefined
      ? HEADER_NAV_DEFAULT.key_query
      : Boolean(config.key_query),
  pricingEnabled:
    config.pricing?.enabled === undefined
      ? HEADER_NAV_DEFAULT.pricing.enabled
      : Boolean(config.pricing.enabled),
  pricingRequireAuth:
    config.pricing?.requireAuth === undefined
      ? HEADER_NAV_DEFAULT.pricing.requireAuth
      : Boolean(config.pricing.requireAuth),
  rankingsEnabled:
    config.rankings?.enabled === undefined
      ? HEADER_NAV_DEFAULT.rankings.enabled
      : Boolean(config.rankings.enabled),
  rankingsRequireAuth:
    config.rankings?.requireAuth === undefined
      ? HEADER_NAV_DEFAULT.rankings.requireAuth
      : Boolean(config.rankings.requireAuth),
  docs:
    config.docs === undefined ? HEADER_NAV_DEFAULT.docs : Boolean(config.docs),
  about:
    config.about === undefined
      ? HEADER_NAV_DEFAULT.about
      : Boolean(config.about),
})

export function HeaderNavigationSection({
  config,
  initialSerialized,
}: HeaderNavigationSectionProps) {
  const { t } = useTranslation()
  const updateOption = useUpdateOption()
  const formDefaults = useMemo(() => toFormValues(config), [config])

  const form = useForm<HeaderNavFormValues>({
    resolver: zodResolver(headerNavSchema),
    defaultValues: formDefaults,
  })

  useEffect(() => {
    form.reset(formDefaults)
  }, [formDefaults, form])

  const onSubmit = async (values: HeaderNavFormValues) => {
    const payload: HeaderNavModulesConfig = {
      ...config,
      home: values.home,
      console: values.console,
      key_query: values.key_query,
      docs: values.docs,
      about: values.about,
      pricing: {
        ...(config.pricing ?? HEADER_NAV_DEFAULT.pricing),
        enabled: values.pricingEnabled,
        requireAuth: values.pricingRequireAuth,
      },
      rankings: {
        ...(config.rankings ?? HEADER_NAV_DEFAULT.rankings),
        enabled: values.rankingsEnabled,
        requireAuth: values.rankingsRequireAuth,
      },
    }

    const serialized = serializeHeaderNavModules(payload)
    if (serialized === initialSerialized) {
      return
    }

    await updateOption.mutateAsync({
      key: 'HeaderNavModules',
      value: serialized,
    })
  }

  const resetToDefault = () => {
    form.reset(toFormValues(HEADER_NAV_DEFAULT))
  }

  const simpleModules: Array<{
    key: keyof HeaderNavFormValues
    title: string
    description: string
  }> = [
    {
      key: 'home',
      title: t('systemSettings.fields.home'),
      description: t('systemSettings.tips.landingPageWithSystemOverview'),
    },
    {
      key: 'console',
      title: t('common.titles.console'),
      description: t('systemSettings.tips.userDashboardAndQuotaControls'),
    },
    {
      key: 'key_query',
      title: t('keyQuery.titles.usageQuery'),
      description: t(
        'systemSettings.actions.queryBalanceAndRecentUsageByApiKey'
      ),
    },
    {
      key: 'docs',
      title: t('pricing.fields.docs'),
      description: t(
        'systemSettings.tips.documentationOrExternalKnowledgeBase'
      ),
    },
    {
      key: 'about',
      title: t('about.titles.value'),
      description: t('systemSettings.tips.staticPageDescribingThePlatform'),
    },
  ]

  const accessModules: Array<{
    enabledKey: keyof HeaderNavFormValues
    requireAuthKey: keyof HeaderNavFormValues
    enabled: boolean
    title: string
    description: string
    requireAuthTitle: string
    requireAuthDescription: string
  }> = [
    {
      enabledKey: 'pricingEnabled',
      requireAuthKey: 'pricingRequireAuth',
      enabled: Boolean(form.watch('pricingEnabled')),
      title: t('pricing.fields.modelSquare'),
      description: t('systemSettings.tips.publicModelCatalogAndPricingPage'),
      requireAuthTitle: t('systemSettings.tips.requireLoginToViewModels'),
      requireAuthDescription: t(
        'systemSettings.errors.visitorsMustAuthenticateBeforeAccessingThePricingDirectory'
      ),
    },
    {
      enabledKey: 'rankingsEnabled',
      requireAuthKey: 'rankingsRequireAuth',
      enabled: Boolean(form.watch('rankingsEnabled')),
      title: t('rankings.titles.value'),
      description: t(
        'systemSettings.tips.publicRankingsPageBasedOnLiveUsageData'
      ),
      requireAuthTitle: t('systemSettings.tips.requireLoginToViewRankings'),
      requireAuthDescription: t(
        'systemSettings.errors.visitorsMustAuthenticateBeforeAccessingTheRankingsPage'
      ),
    },
  ]

  return (
    <SettingsSection title={t('systemSettings.fields.headerNavigation')}>
      <Form {...form}>
        <SettingsForm onSubmit={form.handleSubmit(onSubmit)}>
          <SettingsPageFormActions
            onSave={form.handleSubmit(onSubmit)}
            onReset={resetToDefault}
            isSaving={updateOption.isPending}
            resettableWhenClean
            resetLabel='common.actions.resetToDefault'
            saveLabel='common.actions.saveNavigation'
          />
          <SettingsGroup>
            {simpleModules.map((module) => (
              <FormField
                key={module.key}
                control={form.control}
                name={module.key}
                render={({ field }) => (
                  <SettingsRow
                    label={module.title}
                    description={module.description}
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
            ))}
          </SettingsGroup>

          {accessModules.map((module) => (
            <SettingsGroup
              key={module.enabledKey}
              title={module.title}
              description={module.description}
              control={<SettingsGroupSwitch name={module.enabledKey} />}
              enabled={module.enabled}
            >
              <FormField
                control={form.control}
                name={module.requireAuthKey}
                render={({ field }) => (
                  <SettingsRow
                    label={module.requireAuthTitle}
                    description={module.requireAuthDescription}
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
          ))}
        </SettingsForm>
      </Form>
    </SettingsSection>
  )
}
