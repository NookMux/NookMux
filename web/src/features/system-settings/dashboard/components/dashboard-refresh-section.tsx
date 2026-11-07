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

const refreshIntervalSchema = z.object({
  quota_data_refresh_interval: z.number().min(60).max(86400),
  user_analytics_refresh_interval: z.number().min(60).max(86400),
  rankings_refresh_interval: z.number().min(60).max(86400),
  uptime_kuma_refresh_interval: z.number().min(30).max(3600),
})

type RefreshIntervalFormData = z.infer<typeof refreshIntervalSchema>

export function DashboardRefreshSection() {
  const { t } = useTranslation()
  const { data: config, isLoading } = useDashboardConfig()
  const updateConfig = useUpdateDashboardConfig()

  const { form, handleSubmit, handleReset, isDirty } =
    useSettingsForm<RefreshIntervalFormData>({
      resolver: zodResolver(refreshIntervalSchema),
      defaultValues: config
        ? {
            quota_data_refresh_interval: config.quota_data_refresh_interval,
            user_analytics_refresh_interval:
              config.user_analytics_refresh_interval,
            rankings_refresh_interval: config.rankings_refresh_interval,
            uptime_kuma_refresh_interval: config.uptime_kuma_refresh_interval,
          }
        : undefined,
      onSubmit: async (_data, changedFields) => {
        await updateConfig.mutateAsync(changedFields)
      },
    })

  if (isLoading) {
    return (
      <SettingsSection title={t('systemSettings.actions.refreshIntervals')}>
        <div className='space-y-4'>
          <Skeleton className='h-16 w-full' />
          <Skeleton className='h-16 w-full' />
        </div>
      </SettingsSection>
    )
  }

  if (!config) return null

  return (
    <SettingsSection title={t('systemSettings.actions.refreshIntervals')}>
      <p className='text-muted-foreground mb-4 text-sm'>
        {t(
          'systemSettings.tips.configureHowOftenDashboardDataIsRefreshedLowerIntervals'
        )}
      </p>
      <Form {...form}>
        <SettingsForm onSubmit={handleSubmit}>
          <SettingsFormGrid>
            <SettingsFormGridItem>
              <FormField
                control={form.control}
                name='quota_data_refresh_interval'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>
                      {t('systemSettings.tips.quotaDataIntervalSeconds')}
                    </FormLabel>
                    <FormControl>
                      <Input
                        type='number'
                        min={60}
                        max={86400}
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
                name='user_analytics_refresh_interval'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>
                      {t('systemSettings.tips.userAnalyticsIntervalSeconds')}
                    </FormLabel>
                    <FormControl>
                      <Input
                        type='number'
                        min={60}
                        max={86400}
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
                name='rankings_refresh_interval'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>
                      {t('systemSettings.tips.rankingsIntervalSeconds')}
                    </FormLabel>
                    <FormControl>
                      <Input
                        type='number'
                        min={60}
                        max={86400}
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
                name='uptime_kuma_refresh_interval'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>
                      {t('systemSettings.tips.uptimeMonitoringIntervalSeconds')}
                    </FormLabel>
                    <FormControl>
                      <Input
                        type='number'
                        min={30}
                        max={3600}
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
