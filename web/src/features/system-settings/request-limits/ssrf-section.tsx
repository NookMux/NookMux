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
import {
  Form,
  FormControl,
  FormDescription,
  FormField,
  FormItem,
  FormLabel,
  FormMessage,
} from '@/components/ui/form'
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
import {
  SettingsForm,
  SettingsSwitchContent,
  SettingsSwitchItem,
} from '../components/settings-form-layout'
import { SettingsPageFormActions } from '../components/settings-page-context'
import { SettingsSection } from '../components/settings-section'
import { useUpdateOption } from '../hooks/use-update-option'

const ssrfSchema = z.object({
  fetch_setting: z.object({
    enable_ssrf_protection: z.boolean(),
    allow_private_ip: z.boolean(),
    domain_filter_mode: z.boolean(),
    ip_filter_mode: z.boolean(),
    domain_list: z.string(),
    ip_list: z.string(),
    allowed_ports: z.string(),
    apply_ip_filter_for_domain: z.boolean(),
  }),
})

type SSRFFormValues = z.output<typeof ssrfSchema>
type SSRFFormInput = z.input<typeof ssrfSchema>

type NormalizedSSRFValues = {
  'fetch_setting.enable_ssrf_protection': boolean
  'fetch_setting.allow_private_ip': boolean
  'fetch_setting.domain_filter_mode': boolean
  'fetch_setting.ip_filter_mode': boolean
  'fetch_setting.domain_list': string[]
  'fetch_setting.ip_list': string[]
  'fetch_setting.allowed_ports': string[]
  'fetch_setting.apply_ip_filter_for_domain': boolean
}

type SSRFSectionProps = {
  defaultValues: {
    'fetch_setting.enable_ssrf_protection': boolean
    'fetch_setting.allow_private_ip': boolean
    'fetch_setting.domain_filter_mode': boolean
    'fetch_setting.ip_filter_mode': boolean
    'fetch_setting.domain_list': string[]
    'fetch_setting.ip_list': string[]
    'fetch_setting.allowed_ports': string[]
    'fetch_setting.apply_ip_filter_for_domain': boolean
  }
}

const splitLines = (value: string) =>
  value
    .split('\n')
    .map((entry) => entry.trim())
    .filter(Boolean)

const splitPorts = (value: string) =>
  value
    .split(',')
    .map((item) => item.trim())
    .filter(Boolean)

const buildFormDefaults = (
  defaults: SSRFSectionProps['defaultValues']
): SSRFFormInput => ({
  fetch_setting: {
    enable_ssrf_protection: defaults['fetch_setting.enable_ssrf_protection'],
    allow_private_ip: defaults['fetch_setting.allow_private_ip'],
    domain_filter_mode: defaults['fetch_setting.domain_filter_mode'],
    ip_filter_mode: defaults['fetch_setting.ip_filter_mode'],
    domain_list: defaults['fetch_setting.domain_list'].join('\n'),
    ip_list: defaults['fetch_setting.ip_list'].join('\n'),
    allowed_ports: defaults['fetch_setting.allowed_ports'].join(','),
    apply_ip_filter_for_domain:
      defaults['fetch_setting.apply_ip_filter_for_domain'],
  },
})

const normalizeDefaults = (
  defaults: SSRFSectionProps['defaultValues']
): NormalizedSSRFValues => ({
  'fetch_setting.enable_ssrf_protection':
    defaults['fetch_setting.enable_ssrf_protection'],
  'fetch_setting.allow_private_ip': defaults['fetch_setting.allow_private_ip'],
  'fetch_setting.domain_filter_mode':
    defaults['fetch_setting.domain_filter_mode'],
  'fetch_setting.ip_filter_mode': defaults['fetch_setting.ip_filter_mode'],
  'fetch_setting.domain_list': defaults['fetch_setting.domain_list'],
  'fetch_setting.ip_list': defaults['fetch_setting.ip_list'],
  'fetch_setting.allowed_ports': defaults['fetch_setting.allowed_ports'],
  'fetch_setting.apply_ip_filter_for_domain':
    defaults['fetch_setting.apply_ip_filter_for_domain'],
})

const normalizeFormValues = (values: SSRFFormValues): NormalizedSSRFValues => ({
  'fetch_setting.enable_ssrf_protection':
    values.fetch_setting.enable_ssrf_protection,
  'fetch_setting.allow_private_ip': values.fetch_setting.allow_private_ip,
  'fetch_setting.domain_filter_mode': values.fetch_setting.domain_filter_mode,
  'fetch_setting.ip_filter_mode': values.fetch_setting.ip_filter_mode,
  'fetch_setting.domain_list': splitLines(values.fetch_setting.domain_list),
  'fetch_setting.ip_list': splitLines(values.fetch_setting.ip_list),
  'fetch_setting.allowed_ports': splitPorts(values.fetch_setting.allowed_ports),
  'fetch_setting.apply_ip_filter_for_domain':
    values.fetch_setting.apply_ip_filter_for_domain,
})

const isEqual = (a: unknown, b: unknown) => {
  if (Array.isArray(a) && Array.isArray(b)) {
    return JSON.stringify(a) === JSON.stringify(b)
  }
  return a === b
}

export function SSRFSection({ defaultValues }: SSRFSectionProps) {
  const { t } = useTranslation()
  const updateOption = useUpdateOption()
  const baselineRef = useRef<NormalizedSSRFValues>(
    normalizeDefaults(defaultValues)
  )

  const formDefaults = useMemo(
    () => buildFormDefaults(defaultValues),
    [defaultValues]
  )

  const form = useForm<SSRFFormInput, unknown, SSRFFormValues>({
    resolver: zodResolver(ssrfSchema),
    defaultValues: formDefaults,
  })

  useEffect(() => {
    baselineRef.current = normalizeDefaults(defaultValues)
    form.reset(buildFormDefaults(defaultValues))
  }, [defaultValues, form])

  const onSubmit = async (data: SSRFFormValues) => {
    const normalized = normalizeFormValues(data)
    const updates = (
      Object.keys(normalized) as Array<keyof NormalizedSSRFValues>
    ).filter((key) => !isEqual(normalized[key], baselineRef.current[key]))

    if (updates.length === 0) {
      toast.info(t('channels.fields.noChangesToSave'))
      return
    }

    for (const key of updates) {
      const value = normalized[key]
      await updateOption.mutateAsync({
        key,
        value: Array.isArray(value) ? JSON.stringify(value) : value,
      })
    }

    baselineRef.current = normalized
  }

  const domainFilterMode = form.watch('fetch_setting.domain_filter_mode')
  const ipFilterMode = form.watch('fetch_setting.ip_filter_mode')

  return (
    <SettingsSection title={t('systemSettings.fields.ssrfProtection')}>
      <Form {...form}>
        <SettingsForm onSubmit={form.handleSubmit(onSubmit)}>
          <SettingsPageFormActions
            onSave={form.handleSubmit(onSubmit)}
            isSaving={updateOption.isPending}
            saveLabel='common.actions.saveSsrfSettings'
          />
          <FormField
            control={form.control}
            name='fetch_setting.enable_ssrf_protection'
            render={({ field }) => (
              <SettingsSwitchItem>
                <SettingsSwitchContent>
                  <FormLabel>
                    {t('systemSettings.actions.enableSsrfProtection')}
                  </FormLabel>
                  <FormDescription>
                    {t(
                      'systemSettings.tips.preventServerSideRequestForgeryAttacks'
                    )}
                  </FormDescription>
                </SettingsSwitchContent>
                <FormControl>
                  <Switch
                    checked={field.value}
                    onCheckedChange={field.onChange}
                  />
                </FormControl>
              </SettingsSwitchItem>
            )}
          />

          <FormField
            control={form.control}
            name='fetch_setting.allow_private_ip'
            render={({ field }) => (
              <SettingsSwitchItem>
                <SettingsSwitchContent>
                  <FormLabel>
                    {t('systemSettings.fields.allowPrivateIps')}
                  </FormLabel>
                  <FormDescription>
                    {t(
                      'systemSettings.tips.allowRequestsToPrivateIpRanges1000'
                    )}
                  </FormDescription>
                </SettingsSwitchContent>
                <FormControl>
                  <Switch
                    checked={field.value}
                    onCheckedChange={field.onChange}
                  />
                </FormControl>
              </SettingsSwitchItem>
            )}
          />

          <FormField
            control={form.control}
            name='fetch_setting.domain_filter_mode'
            render={({ field }) => (
              <FormItem>
                <FormLabel>
                  {t('systemSettings.fields.domainFilterMode')}
                </FormLabel>
                <Select
                  items={[
                    {
                      value: 'false',
                      label: t(
                        'systemSettings.tips.blacklistBlockListedDomains'
                      ),
                    },
                    {
                      value: 'true',
                      label: t(
                        'systemSettings.tips.whitelistOnlyAllowListedDomains'
                      ),
                    },
                  ]}
                  onValueChange={(value) => field.onChange(value === 'true')}
                  value={field.value ? 'true' : 'false'}
                >
                  <FormControl>
                    <SelectTrigger>
                      <SelectValue />
                    </SelectTrigger>
                  </FormControl>
                  <SelectContent alignItemWithTrigger={false}>
                    <SelectGroup>
                      <SelectItem value='false'>
                        {t('systemSettings.tips.blacklistBlockListedDomains')}
                      </SelectItem>
                      <SelectItem value='true'>
                        {t(
                          'systemSettings.tips.whitelistOnlyAllowListedDomains'
                        )}
                      </SelectItem>
                    </SelectGroup>
                  </SelectContent>
                </Select>
                <FormDescription>
                  {t('systemSettings.placeholders.chooseHowToFilterDomains')}
                </FormDescription>
              </FormItem>
            )}
          />

          <FormField
            control={form.control}
            name='fetch_setting.domain_list'
            render={({ field }) => (
              <FormItem>
                <FormLabel>
                  {t('systemSettings.fields.domain')}{' '}
                  {domainFilterMode
                    ? t('systemSettings.fields.whitelist')
                    : t('systemSettings.fields.blacklist')}
                </FormLabel>
                <FormControl>
                  <Textarea
                    placeholder={t(
                      'systemSettings.placeholders.exampleComBlockedSiteCom'
                    )}
                    rows={4}
                    {...field}
                  />
                </FormControl>
                <FormDescription>
                  {t('systemSettings.placeholders.oneDomainPerLine')}
                </FormDescription>
                <FormMessage />
              </FormItem>
            )}
          />

          <FormField
            control={form.control}
            name='fetch_setting.ip_filter_mode'
            render={({ field }) => (
              <FormItem>
                <FormLabel>{t('systemSettings.fields.ipFilterMode')}</FormLabel>
                <Select
                  items={[
                    {
                      value: 'false',
                      label: t('systemSettings.fields.blacklistBlockListedIps'),
                    },
                    {
                      value: 'true',
                      label: t(
                        'systemSettings.tips.whitelistOnlyAllowListedIps'
                      ),
                    },
                  ]}
                  onValueChange={(value) => field.onChange(value === 'true')}
                  value={field.value ? 'true' : 'false'}
                >
                  <FormControl>
                    <SelectTrigger>
                      <SelectValue />
                    </SelectTrigger>
                  </FormControl>
                  <SelectContent alignItemWithTrigger={false}>
                    <SelectGroup>
                      <SelectItem value='false'>
                        {t('systemSettings.fields.blacklistBlockListedIps')}
                      </SelectItem>
                      <SelectItem value='true'>
                        {t('systemSettings.tips.whitelistOnlyAllowListedIps')}
                      </SelectItem>
                    </SelectGroup>
                  </SelectContent>
                </Select>
                <FormDescription>
                  {t(
                    'systemSettings.placeholders.chooseHowToFilterIpAddresses'
                  )}
                </FormDescription>
              </FormItem>
            )}
          />

          <FormField
            control={form.control}
            name='fetch_setting.ip_list'
            render={({ field }) => (
              <FormItem>
                <FormLabel>
                  {t('auditLogs.fields.ip')}{' '}
                  {ipFilterMode
                    ? t('systemSettings.fields.whitelist')
                    : t('systemSettings.fields.blacklist')}
                </FormLabel>
                <FormControl>
                  <Textarea
                    placeholder={t(
                      'systemSettings.placeholders.value1921681110000'
                    )}
                    rows={4}
                    {...field}
                  />
                </FormControl>
                <FormDescription>
                  {t('systemSettings.placeholders.oneIpOrCidrRangePerLine')}
                </FormDescription>
                <FormMessage />
              </FormItem>
            )}
          />

          <FormField
            control={form.control}
            name='fetch_setting.allowed_ports'
            render={({ field }) => (
              <FormItem>
                <FormLabel>{t('systemSettings.fields.allowedPorts')}</FormLabel>
                <FormControl>
                  <Input
                    placeholder={t(
                      'systemSettings.placeholders.value804438080'
                    )}
                    {...field}
                  />
                </FormControl>
                <FormDescription>
                  {t(
                    'systemSettings.tips.commaSeparatedListOfAllowedPortsEmptyAllPorts'
                  )}
                </FormDescription>
                <FormMessage />
              </FormItem>
            )}
          />

          <FormField
            control={form.control}
            name='fetch_setting.apply_ip_filter_for_domain'
            render={({ field }) => (
              <SettingsSwitchItem>
                <SettingsSwitchContent>
                  <FormLabel>
                    {t('systemSettings.tips.applyIpFilterToResolvedDomains')}
                  </FormLabel>
                  <FormDescription>
                    {t(
                      'systemSettings.tips.checkResolvedIpsAgainstIpFiltersEvenWhenAccessing'
                    )}
                  </FormDescription>
                </SettingsSwitchContent>
                <FormControl>
                  <Switch
                    checked={field.value}
                    onCheckedChange={field.onChange}
                  />
                </FormControl>
              </SettingsSwitchItem>
            )}
          />
        </SettingsForm>
      </Form>
    </SettingsSection>
  )
}
