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
import { useForm } from 'react-hook-form'
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
  SIDEBAR_MODULES_DEFAULT,
  type SidebarModulesAdminConfig,
  type SidebarSectionConfig,
  serializeSidebarModulesAdmin,
} from './config'

type SidebarModulesSectionProps = {
  config: SidebarModulesAdminConfig
  initialSerialized: string
  defaultCollapseSidebar: boolean
}

type SidebarFormValues = SidebarModulesAdminConfig

const toTitleCase = (value: string) =>
  value.replace(/[_-]+/g, ' ').replace(/\b\w/g, (char) => char.toUpperCase())

export function SidebarModulesSection({
  config,
  initialSerialized,
  defaultCollapseSidebar,
}: SidebarModulesSectionProps) {
  const { t } = useTranslation()
  const updateOption = useUpdateOption()

  const sectionMeta: Record<string, { title: string; description: string }> = {
    chat: {
      title: t('systemSettings.fields.chatArea'),
      description: t(
        'systemSettings.tips.playgroundExperimentsAndLiveConversations'
      ),
    },
    console: {
      title: t('systemSettings.titles.consoleArea'),
      description: t('systemSettings.tips.dashboardsTokensAndUsageAnalytics'),
    },
    personal: {
      title: t('systemSettings.fields.personalArea'),
      description: t(
        'systemSettings.tips.walletManagementAndPersonalPreferences'
      ),
    },
    support: {
      title: t('systemSettings.fields.supportArea'),
      description: t('systemSettings.tips.ticketSupportAndUserAssistance'),
    },
    admin: {
      title: t('systemSettings.fields.adminArea'),
      description: t(
        'systemSettings.tips.globalConfigurationAndAdministrativeTools'
      ),
    },
  }

  const moduleMeta: Record<
    string,
    Record<string, { title: string; description: string }>
  > = {
    chat: {
      playground: {
        title: t('systemSettings.titles.playground'),
        description: t(
          'systemSettings.tips.experimentWithPromptsAndModelsInRealTime'
        ),
      },
      custom_voice: {
        title: t('multimodal.fields.customVoice'),
        description: t('multimodal.tips.customizeVoiceConfigurations'),
      },
    },
    console: {
      detail: {
        title: t('systemSettings.titles.dashboard'),
        description: t(
          'systemSettings.tips.aggregatedUsageMetricsAndTrendCharts'
        ),
      },
      token: {
        title: t('systemSettings.titles.tokenManagement'),
        description: t('systemSettings.actions.createRevokeAndAuditApiTokens'),
      },
      log: {
        title: t('dashboard.titles.usageLogs'),
        description: t(
          'systemSettings.tips.detailedRequestLogsForInvestigations'
        ),
      },
      multimodal_files: {
        title: t('systemSettings.fields.multimodalFiles'),
        description: t(
          'systemSettings.tips.manageUploadedMultimodalFileResources'
        ),
      },
    },
    personal: {
      topup: {
        title: t('layout.titles.wallet'),
        description: t('systemSettings.tips.topUpBalanceAndViewBillingHistory'),
      },
      order_query: {
        title: t('systemSettings.fields.orderQuery'),
        description: t('systemSettings.actions.searchTopUpOrdersByOrderNumber'),
      },
      personal: {
        title: t('layout.titles.profile'),
        description: t(
          'systemSettings.tips.personalSettingsAndProfileManagement'
        ),
      },
    },
    support: {
      ticket: {
        title: t('systemSettings.fields.tickets'),
        description: t(
          'systemSettings.actions.createReplyToAndManageSupportTickets'
        ),
      },
    },
    admin: {
      dynamic_ratio: {
        title: t('systemSettings.fields.dynamicRatio'),
        description: t('systemSettings.tips.manageDynamicRatioRules'),
      },
      channel: {
        title: t('channels.titles.value'),
        description: t('dashboard.tips.configureUpstreamProvidersAndRouting'),
      },
      models: {
        title: t('channels.titles.models'),
        description: t('systemSettings.tips.manageCatalogVisibilityAndPricing'),
      },
      redemption: {
        title: t('redemptionCodes.fields.codes'),
        description: t(
          'systemSettings.actions.createAndReviewInviteOrCreditCodes'
        ),
      },
      user: {
        title: t('systemSettings.titles.users'),
        description: t('systemSettings.tips.administerUserAccountsAndRoles'),
      },
      setting: {
        title: t('common.titles.systemSettings'),
        description: t('systemSettings.tips.advancedPlatformConfiguration'),
      },
      audit_log: {
        title: t('auditLogs.titles.logs'),
        description: t(
          'systemSettings.tips.reviewAdministrativeOperationRecords'
        ),
      },
      voice_management: {
        title: t('voiceManagement.titles.value'),
        description: t('systemSettings.tips.manageVoiceWhitelistAndRedirects'),
      },
    },
  }
  const formDefaults = useMemo(() => config, [config])

  const form = useForm<SidebarFormValues>({
    defaultValues: formDefaults,
  })

  useEffect(() => {
    form.reset(formDefaults)
  }, [formDefaults, form])

  const onSubmit = async (values: SidebarFormValues) => {
    // Strip unknown sections and modules before saving, so stale entries
    // (e.g. "midjourney", "task" from old data) are cleaned up.
    const cleaned: SidebarFormValues = {}
    for (const [sectionKey, sectionConfig] of Object.entries(values)) {
      const defaultSection = SIDEBAR_MODULES_DEFAULT[sectionKey]
      if (!defaultSection) continue
      const cleanedSection: SidebarSectionConfig = {
        enabled: sectionConfig.enabled,
      }
      for (const [moduleKey, moduleValue] of Object.entries(sectionConfig)) {
        if (moduleKey === 'enabled') continue
        if (moduleKey in defaultSection) {
          cleanedSection[moduleKey] = moduleValue
        }
      }
      cleaned[sectionKey] = cleanedSection
    }

    const serialized = serializeSidebarModulesAdmin(cleaned)
    if (serialized === initialSerialized) {
      return
    }

    await updateOption.mutateAsync({
      key: 'SidebarModulesAdmin',
      value: serialized,
    })
  }

  const resetToDefault = () => {
    form.reset(SIDEBAR_MODULES_DEFAULT)
  }

  // Only render sections that exist in the default config.
  // Unknown sections (e.g. stale "midjourney", "task" from old data)
  // are silently stripped so the admin cannot toggle non-existent modules.
  const knownSectionKeys = new Set(Object.keys(SIDEBAR_MODULES_DEFAULT))
  const sections = Object.entries(config).filter(([sectionKey]) =>
    knownSectionKeys.has(sectionKey)
  )

  return (
    <SettingsSection title={t('systemSettings.fields.sidebarModules')}>
      <SettingsGroup>
        <SettingsRow
          label={t('systemSettings.fields.defaultCollapseSidebar')}
          description={t(
            'systemSettings.tips.sidebarCollapsedByDefaultForNewUsers'
          )}
        >
          <Switch
            checked={defaultCollapseSidebar}
            onCheckedChange={(checked) =>
              updateOption.mutate({
                key: 'DefaultCollapseSidebar',
                value: checked,
              })
            }
          />
        </SettingsRow>
      </SettingsGroup>
      <Form {...form}>
        <SettingsForm onSubmit={form.handleSubmit(onSubmit)}>
          <SettingsPageFormActions
            onSave={form.handleSubmit(onSubmit)}
            onReset={resetToDefault}
            isSaving={updateOption.isPending}
            resettableWhenClean
            resetLabel='common.actions.resetToDefault'
            saveLabel='common.actions.saveSidebarModules'
          />
          {sections.map(([sectionKey, sectionConfig]) => {
            const sectionInfo = sectionMeta[sectionKey] ?? {
              title: toTitleCase(sectionKey),
              description: t('systemSettings.fields.customSidebarSection'),
            }
            // Only render modules that exist in the default config for this section.
            const defaultModules = SIDEBAR_MODULES_DEFAULT[sectionKey]
            const knownModuleKeys = defaultModules
              ? new Set(Object.keys(defaultModules))
              : null
            const modules = Object.entries(sectionConfig).filter(
              ([moduleKey]) =>
                moduleKey !== 'enabled' &&
                (!knownModuleKeys || knownModuleKeys.has(moduleKey))
            )
            const sectionEnabled = Boolean(
              // eslint-disable-next-line @typescript-eslint/no-explicit-any
              form.watch(`${sectionKey}.enabled` as any)
            )

            return (
              <SettingsGroup
                key={sectionKey}
                title={sectionInfo.title}
                description={sectionInfo.description}
                control={<SettingsGroupSwitch name={`${sectionKey}.enabled`} />}
                enabled={sectionEnabled}
              >
                {modules.map(([moduleKey]) => {
                  const moduleInfo = moduleMeta[sectionKey]?.[moduleKey] ?? {
                    title: toTitleCase(moduleKey),
                    description: t('systemSettings.fields.customModule'),
                  }
                  return (
                    <FormField
                      key={`${sectionKey}.${moduleKey}`}
                      control={form.control}
                      // eslint-disable-next-line @typescript-eslint/no-explicit-any
                      name={`${sectionKey}.${moduleKey}` as any}
                      render={({ field }) => (
                        <SettingsRow
                          label={moduleInfo.title}
                          description={moduleInfo.description}
                        >
                          <FormControl>
                            <Switch
                              checked={Boolean(field.value)}
                              onCheckedChange={field.onChange}
                            />
                          </FormControl>
                        </SettingsRow>
                      )}
                    />
                  )
                })}
              </SettingsGroup>
            )
          })}
        </SettingsForm>
      </Form>
    </SettingsSection>
  )
}
