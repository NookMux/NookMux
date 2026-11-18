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
import { useMemo, useRef } from 'react'
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
} from '@/components/ui/form'
import { Input } from '@/components/ui/input'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { Switch } from '@/components/ui/switch'
import { DisabledSettingsNotice } from '../components/disabled-settings-notice'
import {
  SettingsForm,
  SettingsSwitchContent,
  SettingsSwitchItem,
} from '../components/settings-form-layout'
import { SettingsPageFormActions } from '../components/settings-page-context'
import { SettingsSection } from '../components/settings-section'
import { useResetForm } from '../hooks/use-reset-form'
import { useUpdateOption } from '../hooks/use-update-option'

/**
 * Perf metrics form uses a nested schema because field names contain dots.
 * React Hook Form treats dotted names as nested paths internally
 * (IS_KEY_RE = /^\w*$/), so the schema must mirror that nested structure.
 */
const perfMetricsSchema = z.object({
  perf_metrics_setting: z.object({
    enabled: z.boolean(),
    flush_interval: z.coerce
      .number()
      .int('common.errors.mustBeAnInteger')
      .min(1, 'systemSettings.errors.mustBeAtLeast1'),
    bucket_time: z.enum(['minute', '5min', 'hour']),
    retention_days: z.coerce
      .number()
      .int('common.errors.mustBeAnInteger')
      .min(0, 'systemSettings.errors.mustBeNonNegative'),
  }),
})

type PerfMetricsFormValues = z.output<typeof perfMetricsSchema>
type PerfMetricsFormInput = z.input<typeof perfMetricsSchema>

type FlatPerfMetricsOptionKey =
  | 'perf_metrics_setting.enabled'
  | 'perf_metrics_setting.flush_interval'
  | 'perf_metrics_setting.bucket_time'
  | 'perf_metrics_setting.retention_days'

type FlatPerfMetricsValues = {
  'perf_metrics_setting.enabled': boolean
  'perf_metrics_setting.flush_interval': number
  'perf_metrics_setting.bucket_time': string
  'perf_metrics_setting.retention_days': number
}

const BUCKET_TIME_OPTIONS: Array<{
  value: PerfMetricsFormValues['perf_metrics_setting']['bucket_time']
  labelKey: string
}> = [
  {
    value: 'minute',
    labelKey: 'systemSettings.fields.perfMetricsBucketMinute',
  },
  { value: '5min', labelKey: 'systemSettings.fields.perfMetricsBucket5Min' },
  { value: 'hour', labelKey: 'systemSettings.fields.perfMetricsBucketHour' },
]

/** Props use flat keys (matching backend option names); the form uses nested shape. */
interface Props {
  defaultValues: FlatPerfMetricsValues
}

const buildFormDefaults = (
  defaults: Props['defaultValues']
): PerfMetricsFormInput => ({
  perf_metrics_setting: {
    enabled: defaults['perf_metrics_setting.enabled'],
    flush_interval: defaults['perf_metrics_setting.flush_interval'],
    bucket_time: defaults[
      'perf_metrics_setting.bucket_time'
    ] as PerfMetricsFormInput['perf_metrics_setting']['bucket_time'],
    retention_days: defaults['perf_metrics_setting.retention_days'],
  },
})

const flattenFormValues = (
  values: PerfMetricsFormValues
): FlatPerfMetricsValues => ({
  'perf_metrics_setting.enabled': values.perf_metrics_setting.enabled,
  'perf_metrics_setting.flush_interval':
    values.perf_metrics_setting.flush_interval,
  'perf_metrics_setting.bucket_time': values.perf_metrics_setting.bucket_time,
  'perf_metrics_setting.retention_days':
    values.perf_metrics_setting.retention_days,
})

export function PerfMetricsSection(props: Props) {
  const { t } = useTranslation()
  const updateOption = useUpdateOption()

  const formDefaults = useMemo(
    () => buildFormDefaults(props.defaultValues),
    [props.defaultValues]
  )

  const form = useForm<PerfMetricsFormInput, unknown, PerfMetricsFormValues>({
    resolver: zodResolver(perfMetricsSchema),
    defaultValues: formDefaults,
  })

  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  useResetForm(form as any, formDefaults)

  const baselineRef = useRef<FlatPerfMetricsValues>({ ...props.defaultValues })

  const onSubmit = async (data: PerfMetricsFormValues) => {
    const normalized = flattenFormValues(data)
    const updates = (
      Object.keys(normalized) as FlatPerfMetricsOptionKey[]
    ).filter((key) => normalized[key] !== baselineRef.current[key])
    if (updates.length === 0) {
      toast.info(t('channels.fields.noChangesToSave'))
      return
    }
    for (const key of updates) {
      await updateOption.mutateAsync({ key, value: normalized[key] })
    }
    baselineRef.current = normalized
    toast.success(t('systemSettings.status.savedSuccessfully'))
  }

  const enabled = form.watch('perf_metrics_setting.enabled')

  return (
    <SettingsSection title={t('systemSettings.titles.perfMetricsSettings')}>
      <Form {...form}>
        <SettingsForm onSubmit={form.handleSubmit(onSubmit)}>
          <SettingsPageFormActions
            onSave={form.handleSubmit(onSubmit)}
            isSaving={updateOption.isPending}
            saveLabel='common.actions.saveChanges'
            savingLabel='common.tips.saving'
          />

          <div>
            <h4 className='font-medium'>
              {t('systemSettings.titles.perfMetricsSettings')}
            </h4>
            <p className='text-muted-foreground mt-1 text-xs'>
              {t('systemSettings.tips.perfMetricsDesc')}
            </p>
          </div>

          <DisabledSettingsNotice enabled={enabled} />

          <div data-settings-form-span='full' className='space-y-4'>
            <FormField
              control={form.control}
              name='perf_metrics_setting.enabled'
              render={({ field }) => (
                <SettingsSwitchItem className='border-b-2 pb-3'>
                  <SettingsSwitchContent>
                    <FormLabel>
                      {t('systemSettings.actions.enablePerfMetrics')}
                    </FormLabel>
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

            <div className='border-border/40 grid grid-cols-1 gap-4 border-l pl-4 md:grid-cols-3'>
              <FormField
                control={form.control}
                name='perf_metrics_setting.flush_interval'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>
                      {t('systemSettings.fields.perfMetricsFlushInterval')}
                    </FormLabel>
                    <FormControl>
                      <Input
                        type='number'
                        min={1}
                        step={1}
                        value={
                          typeof field.value === 'number' &&
                          Number.isFinite(field.value)
                            ? field.value
                            : ''
                        }
                        onChange={(event) =>
                          field.onChange(event.target.valueAsNumber)
                        }
                        name={field.name}
                        onBlur={field.onBlur}
                        ref={field.ref}
                        disabled={!enabled}
                      />
                    </FormControl>
                    <FormDescription>
                      {t('systemSettings.tips.perfMetricsFlushIntervalTip')}
                    </FormDescription>
                  </FormItem>
                )}
              />

              <FormField
                control={form.control}
                name='perf_metrics_setting.bucket_time'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>
                      {t('systemSettings.fields.perfMetricsBucketTime')}
                    </FormLabel>
                    <Select
                      value={field.value}
                      onValueChange={(value) => field.onChange(value)}
                      disabled={!enabled}
                    >
                      <FormControl>
                        <SelectTrigger className='w-full'>
                          <SelectValue />
                        </SelectTrigger>
                      </FormControl>
                      <SelectContent alignItemWithTrigger={false}>
                        {BUCKET_TIME_OPTIONS.map((option) => (
                          <SelectItem key={option.value} value={option.value}>
                            {t(option.labelKey)}
                          </SelectItem>
                        ))}
                      </SelectContent>
                    </Select>
                  </FormItem>
                )}
              />

              <FormField
                control={form.control}
                name='perf_metrics_setting.retention_days'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>
                      {t('systemSettings.fields.perfMetricsRetentionDays')}
                    </FormLabel>
                    <FormControl>
                      <Input
                        type='number'
                        min={0}
                        step={1}
                        value={
                          typeof field.value === 'number' &&
                          Number.isFinite(field.value)
                            ? field.value
                            : ''
                        }
                        onChange={(event) =>
                          field.onChange(event.target.valueAsNumber)
                        }
                        name={field.name}
                        onBlur={field.onBlur}
                        ref={field.ref}
                        disabled={!enabled}
                      />
                    </FormControl>
                    <FormDescription>
                      {t('systemSettings.tips.perfMetricsRetentionDaysTip')}
                    </FormDescription>
                  </FormItem>
                )}
              />
            </div>
          </div>
        </SettingsForm>
      </Form>
    </SettingsSection>
  )
}
