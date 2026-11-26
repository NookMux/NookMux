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
import { useEffect, useMemo, useRef } from 'react'
import * as z from 'zod'
import { useForm } from 'react-hook-form'
import { zodResolver } from '@hookform/resolvers/zod'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { Form, FormControl, FormField } from '@/components/ui/form'
import { Switch } from '@/components/ui/switch'
import { SettingsForm } from '../components/settings-form-layout'
import {
  SettingsChoiceGrid,
  SettingsGroup,
  SettingsGroupSwitch,
  SettingsRow,
} from '../components/settings-group'
import { SettingsPageFormActions } from '../components/settings-page-context'
import { SettingsSection } from '../components/settings-section'
import { useUpdateOption } from '../hooks/use-update-option'

// Audit modules — kept in sync with backend audit module list.
// The value keys are also used as i18n translation keys.
const AUDIT_SETTING_MODULES = [
  'option',
  'channel',
  'user',
  'token',
  'redemption',
  'model',
  'vendor',
  'dynamic_ratio',
  'prefill_group',
  'db',
  'performance',
  'log',
  'setup',
] as const

type AuditModuleKey = (typeof AUDIT_SETTING_MODULES)[number]

// Map of module value -> existing i18n translation key. These keys are shared
// with the audit logs viewing page so module names stay consistent across the UI.
const AUDIT_MODULE_LABEL_KEYS: Record<AuditModuleKey, string> = {
  option: 'common.titles.auditModuleSystemSettings',
  channel: 'common.titles.auditModuleChannels',
  user: 'common.titles.auditModuleUsers',
  token: 'common.fields.auditModuleTokens',
  redemption: 'common.fields.auditModuleRedemptionCodes',
  model: 'common.titles.auditModuleModels',
  vendor: 'common.fields.auditModuleVendors',
  dynamic_ratio: 'common.fields.auditModuleDynamicRatio',
  prefill_group: 'common.fields.auditModulePrefillGroups',
  db: 'common.fields.auditModuleDatabaseMigration',
  performance: 'common.fields.auditModulePerformance',
  log: 'common.fields.auditModuleLogCleanup',
  setup: 'common.titles.auditModuleSystemSetup',
}

const auditSchema = z.object({
  enabled: z.boolean(),
  modules: z.record(z.string(), z.boolean()),
  record_ip: z.boolean(),
  record_diff: z.boolean(),
})

type AuditFormValues = z.output<typeof auditSchema>
type AuditFormInput = z.input<typeof auditSchema>

type NormalizedAuditValues = {
  'audit_setting.enabled': boolean
  'audit_setting.modules': string
  'audit_setting.record_ip': boolean
  'audit_setting.record_diff': boolean
}

type AuditSectionProps = {
  defaultValues: {
    'audit_setting.enabled': boolean
    'audit_setting.modules': string
    'audit_setting.record_ip': boolean
    'audit_setting.record_diff': boolean
  }
}

const DEFAULT_MODULES_JSON = JSON.stringify(
  Object.fromEntries(AUDIT_SETTING_MODULES.map((m) => [m, true]))
)

const parseModules = (raw: string): Record<string, boolean> => {
  if (!raw) {
    return Object.fromEntries(AUDIT_SETTING_MODULES.map((m) => [m, true]))
  }
  try {
    const parsed = JSON.parse(raw) as Record<string, unknown>
    const result: Record<string, boolean> = {}
    for (const m of AUDIT_SETTING_MODULES) {
      result[m] = parsed[m] !== false
    }
    return result
  } catch {
    return Object.fromEntries(AUDIT_SETTING_MODULES.map((m) => [m, true]))
  }
}

const buildFormDefaults = (
  defaults: AuditSectionProps['defaultValues']
): AuditFormInput => ({
  enabled: defaults['audit_setting.enabled'],
  modules: parseModules(defaults['audit_setting.modules']),
  record_ip: defaults['audit_setting.record_ip'],
  record_diff: defaults['audit_setting.record_diff'],
})

const normalizeDefaults = (
  defaults: AuditSectionProps['defaultValues']
): NormalizedAuditValues => ({
  'audit_setting.enabled': defaults['audit_setting.enabled'],
  'audit_setting.modules':
    defaults['audit_setting.modules'] || DEFAULT_MODULES_JSON,
  'audit_setting.record_ip': defaults['audit_setting.record_ip'],
  'audit_setting.record_diff': defaults['audit_setting.record_diff'],
})

const normalizeFormValues = (
  values: AuditFormValues
): NormalizedAuditValues => ({
  'audit_setting.enabled': values.enabled,
  'audit_setting.modules': JSON.stringify(values.modules),
  'audit_setting.record_ip': values.record_ip,
  'audit_setting.record_diff': values.record_diff,
})

export function AuditSection({ defaultValues }: AuditSectionProps) {
  const { t } = useTranslation()
  const updateOption = useUpdateOption()
  const baselineRef = useRef<NormalizedAuditValues>(
    normalizeDefaults(defaultValues)
  )

  const formDefaults = useMemo(
    () => buildFormDefaults(defaultValues),
    [defaultValues]
  )

  const form = useForm<AuditFormInput, unknown, AuditFormValues>({
    resolver: zodResolver(auditSchema),
    defaultValues: formDefaults,
  })
  const enabled = form.watch('enabled')
  const modules = form.watch('modules')

  useEffect(() => {
    baselineRef.current = normalizeDefaults(defaultValues)
    form.reset(buildFormDefaults(defaultValues))
  }, [defaultValues, form])

  const onSubmit = async (data: AuditFormValues) => {
    const normalized = normalizeFormValues(data)
    const updates = (
      Object.keys(normalized) as Array<keyof NormalizedAuditValues>
    ).filter((key) => normalized[key] !== baselineRef.current[key])

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

    baselineRef.current = normalized
  }

  const moduleOptions = AUDIT_SETTING_MODULES.map((moduleKey) => ({
    value: moduleKey,
    label: t(AUDIT_MODULE_LABEL_KEYS[moduleKey]),
  }))
  const enabledModules = AUDIT_SETTING_MODULES.filter(
    (moduleKey) => modules?.[moduleKey] !== false
  )

  return (
    <SettingsSection title={t('systemSettings.fields.auditLog')}>
      <Form {...form}>
        <SettingsForm onSubmit={form.handleSubmit(onSubmit)}>
          <SettingsPageFormActions
            onSave={form.handleSubmit(onSubmit)}
            isSaving={updateOption.isPending}
            saveLabel='common.actions.saveAuditSettings'
          />

          <SettingsGroup
            title={t('systemSettings.actions.enableAuditLog')}
            description={t(
              'systemSettings.tips.recordAdministratorOperationsOnSystemResourcesForTraceability'
            )}
            control={<SettingsGroupSwitch name='enabled' />}
            enabled={enabled}
          >
            <SettingsRow
              label={t('systemSettings.fields.recordedModules')}
              description={t(
                'systemSettings.placeholders.selectWhichModulesToRecordInTheAuditLog'
              )}
            >
              <SettingsChoiceGrid
                aria-label={t('systemSettings.fields.recordedModules')}
                columns={3}
                options={moduleOptions}
                value={enabledModules}
                onValueChange={(next) => {
                  const nextModules = Object.fromEntries(
                    AUDIT_SETTING_MODULES.map((moduleKey) => [
                      moduleKey,
                      next.includes(moduleKey),
                    ])
                  )
                  form.setValue('modules', nextModules, { shouldDirty: true })
                }}
              />
            </SettingsRow>
          </SettingsGroup>

          <SettingsGroup>
            <FormField
              control={form.control}
              name='record_ip'
              render={({ field }) => (
                <SettingsRow
                  label={t('systemSettings.fields.recordIp')}
                  description={t(
                    'systemSettings.tips.whetherToRecordTheOperatorIpAddress'
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
              name='record_diff'
              render={({ field }) => (
                <SettingsRow
                  label={t('systemSettings.fields.recordDiff')}
                  description={t(
                    'systemSettings.tips.whetherToRecordBeforeAfterDataDiff'
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
