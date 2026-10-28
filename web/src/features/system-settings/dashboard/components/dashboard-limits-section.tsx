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
import {
  Form,
  FormControl,
  FormField,
  FormItem,
  FormLabel,
  FormMessage,
} from '@/components/ui/form'
import { Input } from '@/components/ui/input'
import { Skeleton } from '@/components/ui/skeleton'
import {
  SettingsForm,
  SettingsFormGrid,
  SettingsFormGridItem,
} from '../../components/settings-form-layout'
import { SettingsPageFormActions } from '../../components/settings-page-context'
import { SettingsSection } from '../../components/settings-section'
import { useSettingsForm } from '../../hooks/use-settings-form'
import {
  useDashboardConfig,
  useUpdateDashboardConfig,
} from '../hooks/use-dashboard-config'

const limitsSchema = z
  .object({
    default_time_range_days: z.number().min(1).max(365),
    max_time_range_days: z.number().min(1).max(365),
    rankings_model_limit: z.number().min(1).max(100),
    rankings_vendor_limit: z.number().min(1).max(50),
    rankings_platform_limit: z.number().min(1).max(50),
    user_analytics_top_n: z.number().min(1).max(100),
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

  const { form, handleSubmit, handleReset, isDirty } =
    useSettingsForm<LimitsFormData>({
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
      <p className='text-muted-foreground mb-4 text-sm'>
        {t(
          'systemSettings.tips.configureDataLimitsAndTimeRangeRestrictionsForDashboard'
        )}
      </p>
      <Form {...form}>
        <SettingsForm onSubmit={handleSubmit}>
          <SettingsFormGrid>
            <SettingsFormGridItem>
              <FormField
                control={form.control}
                name='default_time_range_days'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>
                      {t('systemSettings.tips.defaultTimeRangeDays')}
                    </FormLabel>
                    <FormControl>
                      <Input
                        type='number'
                        min={1}
                        max={365}
                        {...field}
                        onChange={(e) =>
                          field.onChange(Number.parseInt(e.target.value))
                        }
                      />
                    </FormControl>
                    <FormMessage />
                  </FormItem>
                )}
              />
            </SettingsFormGridItem>

            <SettingsFormGridItem>
              <FormField
                control={form.control}
                name='max_time_range_days'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>
                      {t('systemSettings.tips.maximumTimeRangeDays')}
                    </FormLabel>
                    <FormControl>
                      <Input
                        type='number'
                        min={1}
                        max={365}
                        {...field}
                        onChange={(e) =>
                          field.onChange(Number.parseInt(e.target.value))
                        }
                      />
                    </FormControl>
                    <FormMessage />
                  </FormItem>
                )}
              />
            </SettingsFormGridItem>

            <SettingsFormGridItem>
              <FormField
                control={form.control}
                name='rankings_model_limit'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>
                      {t('systemSettings.fields.rankingsModelLimit')}
                    </FormLabel>
                    <FormControl>
                      <Input
                        type='number'
                        min={1}
                        max={100}
                        {...field}
                        onChange={(e) =>
                          field.onChange(Number.parseInt(e.target.value))
                        }
                      />
                    </FormControl>
                    <FormMessage />
                  </FormItem>
                )}
              />
            </SettingsFormGridItem>

            <SettingsFormGridItem>
              <FormField
                control={form.control}
                name='rankings_vendor_limit'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>
                      {t('systemSettings.fields.rankingsVendorLimit')}
                    </FormLabel>
                    <FormControl>
                      <Input
                        type='number'
                        min={1}
                        max={50}
                        {...field}
                        onChange={(e) =>
                          field.onChange(Number.parseInt(e.target.value))
                        }
                      />
                    </FormControl>
                    <FormMessage />
                  </FormItem>
                )}
              />
            </SettingsFormGridItem>

            <SettingsFormGridItem>
              <FormField
                control={form.control}
                name='rankings_platform_limit'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>
                      {t('systemSettings.fields.rankingsPlatformLimit')}
                    </FormLabel>
                    <FormControl>
                      <Input
                        type='number'
                        min={1}
                        max={50}
                        {...field}
                        onChange={(e) =>
                          field.onChange(Number.parseInt(e.target.value))
                        }
                      />
                    </FormControl>
                    <FormMessage />
                  </FormItem>
                )}
              />
            </SettingsFormGridItem>

            <SettingsFormGridItem>
              <FormField
                control={form.control}
                name='user_analytics_top_n'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>
                      {t('systemSettings.fields.userAnalyticsTopN')}
                    </FormLabel>
                    <FormControl>
                      <Input
                        type='number'
                        min={1}
                        max={100}
                        {...field}
                        onChange={(e) =>
                          field.onChange(Number.parseInt(e.target.value))
                        }
                      />
                    </FormControl>
                    <FormMessage />
                  </FormItem>
                )}
              />
            </SettingsFormGridItem>
          </SettingsFormGrid>

          <SettingsPageFormActions
            onSave={handleSubmit}
            onReset={handleReset}
            isSaving={updateConfig.isPending}
            isResetDisabled={!isDirty}
          />
        </SettingsForm>
      </Form>
    </SettingsSection>
  )
}
