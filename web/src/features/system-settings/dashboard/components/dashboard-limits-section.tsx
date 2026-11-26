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
import { z } from 'zod'
import { zodResolver } from '@hookform/resolvers/zod'
import { useTranslation } from 'react-i18next'
import { Form, FormControl, FormField } from '@/components/ui/form'
import { Input } from '@/components/ui/input'
import { Skeleton } from '@/components/ui/skeleton'
import { SettingsForm } from '../../components/settings-form-layout'
import {
  SettingsControl,
  SettingsGroup,
  SettingsRow,
} from '../../components/settings-group'
import { SettingsPageFormActions } from '../../components/settings-page-context'
import { SettingsSection } from '../../components/settings-section'
import { useSettingsForm } from '../../hooks/use-settings-form'
import {
  useDashboardConfig,
  useUpdateDashboardConfig,
} from '../hooks/use-dashboard-config'

const limitsSchema = z
  .object({
    default_time_range_days: z
      .number()
      .min(1, 'common.errors.mustBeBetween1And365')
      .max(365, 'common.errors.mustBeBetween1And365'),
    max_time_range_days: z
      .number()
      .min(1, 'common.errors.mustBeBetween1And365')
      .max(365, 'common.errors.mustBeBetween1And365'),
    rankings_model_limit: z
      .number()
      .min(1, 'common.errors.mustBeBetween1And100')
      .max(100, 'common.errors.mustBeBetween1And100'),
    rankings_vendor_limit: z
      .number()
      .min(1, 'common.errors.mustBeBetween1And50')
      .max(50, 'common.errors.mustBeBetween1And50'),
    rankings_platform_limit: z
      .number()
      .min(1, 'common.errors.mustBeBetween1And50')
      .max(50, 'common.errors.mustBeBetween1And50'),
    user_analytics_top_n: z
      .number()
      .min(1, 'common.errors.mustBeBetween1And100')
      .max(100, 'common.errors.mustBeBetween1And100'),
  })
  .refine((data) => data.default_time_range_days <= data.max_time_range_days, {
    message: 'common.errors.defaultTimeRangeExceedsMax',
    path: ['default_time_range_days'],
  })

type LimitsFormData = z.infer<typeof limitsSchema>

export function DashboardLimitsSection() {
  const { t } = useTranslation()
  const { data: config, isLoading } = useDashboardConfig()
  const updateConfig = useUpdateDashboardConfig()

  const { form, handleSubmit, handleReset } = useSettingsForm<LimitsFormData>({
    resolver: zodResolver(limitsSchema),
    defaultValues: config
      ? {
          default_time_range_days: config.default_time_range_days,
          max_time_range_days: config.max_time_range_days,
          rankings_model_limit: config.rankings_model_limit,
          rankings_vendor_limit: config.rankings_vendor_limit,
          rankings_platform_limit: config.rankings_platform_limit,
          user_analytics_top_n: config.user_analytics_top_n,
        }
      : undefined,
    onSubmit: async (_data, changedFields) => {
      await updateConfig.mutateAsync(changedFields)
    },
  })

  if (isLoading) {
    return (
      <SettingsSection title={t('systemSettings.fields.dataLimits')}>
        <div className='space-y-4'>
          <Skeleton className='h-16 w-full' />
          <Skeleton className='h-16 w-full' />
        </div>
      </SettingsSection>
    )
  }

  if (!config) return null

  return (
    <SettingsSection title={t('systemSettings.fields.dataLimits')}>
      <Form {...form}>
        <SettingsForm onSubmit={handleSubmit}>
          <SettingsPageFormActions
            onSave={handleSubmit}
            onReset={handleReset}
            isSaving={updateConfig.isPending}
          />

          <SettingsGroup
            description={t(
              'systemSettings.tips.configureDataLimitsAndTimeRangeRestrictionsForDashboard'
            )}
          >
            <FormField
              control={form.control}
              name='default_time_range_days'
              render={({ field }) => (
                <SettingsRow
                  label={t('systemSettings.tips.defaultTimeRangeDays')}
                >
                  <SettingsControl size='xs'>
                    <FormControl>
                      <Input
                        type='number'
                        min={1}
                        max={365}
                        {...field}
                        onChange={(event) =>
                          field.onChange(Number.parseInt(event.target.value))
                        }
                      />
                    </FormControl>
                  </SettingsControl>
                </SettingsRow>
              )}
            />

            <FormField
              control={form.control}
              name='max_time_range_days'
              render={({ field }) => (
                <SettingsRow
                  label={t('systemSettings.tips.maximumTimeRangeDays')}
                >
                  <SettingsControl size='xs'>
                    <FormControl>
                      <Input
                        type='number'
                        min={1}
                        max={365}
                        {...field}
                        onChange={(event) =>
                          field.onChange(Number.parseInt(event.target.value))
                        }
                      />
                    </FormControl>
                  </SettingsControl>
                </SettingsRow>
              )}
            />

            <FormField
              control={form.control}
              name='rankings_model_limit'
              render={({ field }) => (
                <SettingsRow
                  label={t('systemSettings.fields.rankingsModelLimit')}
                >
                  <SettingsControl size='xs'>
                    <FormControl>
                      <Input
                        type='number'
                        min={1}
                        max={100}
                        {...field}
                        onChange={(event) =>
                          field.onChange(Number.parseInt(event.target.value))
                        }
                      />
                    </FormControl>
                  </SettingsControl>
                </SettingsRow>
              )}
            />

            <FormField
              control={form.control}
              name='rankings_vendor_limit'
              render={({ field }) => (
                <SettingsRow
                  label={t('systemSettings.fields.rankingsVendorLimit')}
                >
                  <SettingsControl size='xs'>
                    <FormControl>
                      <Input
                        type='number'
                        min={1}
                        max={50}
                        {...field}
                        onChange={(event) =>
                          field.onChange(Number.parseInt(event.target.value))
                        }
                      />
                    </FormControl>
                  </SettingsControl>
                </SettingsRow>
              )}
            />

            <FormField
              control={form.control}
              name='rankings_platform_limit'
              render={({ field }) => (
                <SettingsRow
                  label={t('systemSettings.fields.rankingsPlatformLimit')}
                >
                  <SettingsControl size='xs'>
                    <FormControl>
                      <Input
                        type='number'
                        min={1}
                        max={50}
                        {...field}
                        onChange={(event) =>
                          field.onChange(Number.parseInt(event.target.value))
                        }
                      />
                    </FormControl>
                  </SettingsControl>
                </SettingsRow>
              )}
            />

            <FormField
              control={form.control}
              name='user_analytics_top_n'
              render={({ field }) => (
                <SettingsRow
                  label={t('systemSettings.fields.userAnalyticsTopN')}
                >
                  <SettingsControl size='xs'>
                    <FormControl>
                      <Input
                        type='number'
                        min={1}
                        max={100}
                        {...field}
                        onChange={(event) =>
                          field.onChange(Number.parseInt(event.target.value))
                        }
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
