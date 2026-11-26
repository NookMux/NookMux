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
import { useCallback, useMemo } from 'react'
import * as z from 'zod'
import { useForm } from 'react-hook-form'
import { zodResolver } from '@hookform/resolvers/zod'
import { type LucideIcon, Bell, Mail, Server, Webhook } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { formatQuotaWithCurrency } from '@/lib/currency'
import { cn } from '@/lib/utils'
import { Form, FormControl, FormField } from '@/components/ui/form'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { RadioGroup, RadioGroupItem } from '@/components/ui/radio-group'
import { SettingsForm } from '@/features/system-settings/components/settings-form-layout'
import {
  SettingsControl,
  SettingsGroup,
  SettingsRow,
} from '@/features/system-settings/components/settings-group'
import { SettingsNotice } from '@/features/system-settings/components/settings-notice'
import { SettingsPageFormActions } from '@/features/system-settings/components/settings-page-context'
import { useResetForm } from '@/features/system-settings/hooks/use-reset-form'
import { updateUserSettings } from '../api'
import { NOTIFICATION_METHODS } from '../constants'
import {
  isHttpUrl,
  isSameNotifyPayload,
  toNotifyPayload,
  toNotificationFormValues,
  type NotificationFormValues,
} from '../lib/notification'
import type { NotifyType, UserProfile } from '../types'

const NOTIFICATION_ICONS: Record<NotifyType, LucideIcon> = {
  email: Mail,
  webhook: Webhook,
  bark: Bell,
  gotify: Server,
}

const NOTIFICATION_TILE_CLASS =
  'flex min-h-16 cursor-pointer flex-col items-center justify-center gap-2 rounded-lg border p-3 text-center transition-colors has-[[data-slot=radio-group-item]:focus-visible]:ring-3 has-[[data-slot=radio-group-item]:focus-visible]:ring-ring/50'

/** 与接口的校验一致：阈值大于 0，选中的通知方式地址必填且为 http / https。 */
const notificationSchema = z
  .object({
    notify_type: z.enum(['email', 'webhook', 'bark', 'gotify']),
    quota_warning_threshold: z
      .number({ error: 'profile.errors.thresholdMustBeGreaterThanZero' })
      .positive('profile.errors.thresholdMustBeGreaterThanZero'),
    notification_email: z.string(),
    webhook_url: z.string(),
    webhook_secret: z.string(),
    bark_url: z.string(),
    gotify_url: z.string(),
    gotify_token: z.string(),
    gotify_priority: z
      .number({ error: 'profile.errors.priorityMustBeBetween0And10' })
      .int('profile.errors.priorityMustBeBetween0And10')
      .min(0, 'profile.errors.priorityMustBeBetween0And10')
      .max(10, 'profile.errors.priorityMustBeBetween0And10'),
  })
  .superRefine((values, ctx) => {
    const checkUrl = (path: 'webhook_url' | 'bark_url' | 'gotify_url') => {
      const value = values[path].trim()
      if (value === '') {
        ctx.addIssue({
          code: 'custom',
          path: [path],
          message: 'profile.errors.urlRequired',
          input: value,
        })
      } else if (!isHttpUrl(value)) {
        ctx.addIssue({
          code: 'custom',
          path: [path],
          message: 'profile.errors.invalidUrl',
          input: value,
        })
      }
    }

    switch (values.notify_type) {
      case 'email': {
        const email = values.notification_email.trim()
        if (email !== '' && !z.email().safeParse(email).success) {
          ctx.addIssue({
            code: 'custom',
            path: ['notification_email'],
            message: 'profile.errors.pleaseEnterAValidEmailAddress',
            input: email,
          })
        }
        break
      }
      case 'webhook':
        checkUrl('webhook_url')
        break
      case 'bark':
        checkUrl('bark_url')
        break
      case 'gotify':
        checkUrl('gotify_url')
        if (values.gotify_token.trim() === '') {
          ctx.addIssue({
            code: 'custom',
            path: ['gotify_token'],
            message: 'profile.errors.tokenRequired',
            input: values.gotify_token,
          })
        }
        break
    }
  })

interface NotificationFormProps {
  profile: UserProfile
  /** 保存成功后拉取最新资料并返回。 */
  onSaved: () => Promise<UserProfile | null>
}

/** 通知设置，随页头的「保存更改」一起生效。 */
export function NotificationForm({ profile, onSaved }: NotificationFormProps) {
  const { t } = useTranslation()
  const defaultValues = useMemo(
    () => toNotificationFormValues(profile),
    [profile]
  )
  const form = useForm<NotificationFormValues>({
    resolver: zodResolver(notificationSchema),
    defaultValues,
  })
  useResetForm(form, defaultValues)
  const notifyType = form.watch('notify_type')

  const onSubmit = useCallback(
    async (values: NotificationFormValues) => {
      try {
        const payload = toNotifyPayload(values)
        if (isSameNotifyPayload(payload, toNotifyPayload(defaultValues))) {
          return
        }

        const response = await updateUserSettings(payload)
        if (!response.success) {
          toast.error(
            response.message || t('profile.errors.failedToUpdateSettings')
          )
          return
        }

        toast.success(t('channels.status.settingsUpdatedSuccessfully'))
        const saved = await onSaved()
        form.reset(toNotificationFormValues(saved ?? profile))
      } catch (error) {
        // eslint-disable-next-line no-console
        console.error('Failed to save notification settings:', error)
        toast.error(t('profile.errors.failedToUpdateSettings'))
      }
    },
    [defaultValues, form, onSaved, profile, t]
  )

  return (
    <Form {...form}>
      <SettingsForm onSubmit={form.handleSubmit(onSubmit)}>
        <SettingsPageFormActions
          onSave={form.handleSubmit(onSubmit)}
          isSaving={form.formState.isSubmitting}
        />
        <SettingsGroup title={t('profile.titles.notificationSettings')}>
          <FormField
            control={form.control}
            name='notify_type'
            render={({ field }) => (
              <SettingsRow label={t('profile.fields.notificationMethod')}>
                <FormControl>
                  <RadioGroup
                    value={field.value}
                    onValueChange={(value) =>
                      field.onChange(value as NotifyType)
                    }
                    className='grid-cols-2 gap-3 @xl:grid-cols-4'
                  >
                    {NOTIFICATION_METHODS.map((method) => {
                      const Icon = NOTIFICATION_ICONS[method.value]
                      const selected = field.value === method.value

                      return (
                        <Label
                          key={method.value}
                          htmlFor={method.value}
                          className={cn(
                            NOTIFICATION_TILE_CLASS,
                            selected
                              ? 'border-primary bg-primary/5 text-primary'
                              : 'border-border hover:bg-muted/50 dark:border-input'
                          )}
                        >
                          <RadioGroupItem
                            value={method.value}
                            id={method.value}
                            className='sr-only'
                          />
                          <Icon className='size-4' aria-hidden='true' />
                          <span className='text-sm leading-snug font-medium'>
                            {method.isI18nKey ? t(method.label) : method.label}
                          </span>
                        </Label>
                      )
                    })}
                  </RadioGroup>
                </FormControl>
              </SettingsRow>
            )}
          />

          {notifyType === 'email' && (
            <FormField
              control={form.control}
              name='notification_email'
              render={({ field }) => (
                <SettingsRow
                  label={t('profile.fields.notificationEmail')}
                  description={t('profile.tips.leaveEmptyToUseAccountEmail')}
                >
                  <SettingsControl>
                    <FormControl>
                      <Input
                        type='text'
                        inputMode='email'
                        placeholder={t('profile.placeholders.enterYourEmail')}
                        {...field}
                        value={field.value ?? ''}
                      />
                    </FormControl>
                  </SettingsControl>
                </SettingsRow>
              )}
            />
          )}

          {notifyType === 'webhook' && (
            <>
              <FormField
                control={form.control}
                name='webhook_url'
                render={({ field }) => (
                  <SettingsRow label={t('profile.fields.webhookUrl')}>
                    <SettingsControl>
                      <FormControl>
                        <Input
                          placeholder={t(
                            'profile.placeholders.urlExampleComWebhook'
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
                name='webhook_secret'
                render={({ field }) => (
                  <SettingsRow label={t('profile.fields.webhookSecret')}>
                    <SettingsControl>
                      <FormControl>
                        <Input
                          type='password'
                          autoComplete='new-password'
                          placeholder={t('profile.placeholders.enterSecretKey')}
                          {...field}
                          value={field.value ?? ''}
                        />
                      </FormControl>
                    </SettingsControl>
                  </SettingsRow>
                )}
              />
            </>
          )}

          {notifyType === 'bark' && (
            <FormField
              control={form.control}
              name='bark_url'
              render={({ field }) => (
                <SettingsRow
                  label={t('profile.fields.barkPushUrl')}
                  hint={
                    <>
                      {t('profile.fields.templateVariables')}
                      {'{{title}}'}, {'{{content}}'}
                    </>
                  }
                >
                  <SettingsControl>
                    <FormControl>
                      <Input
                        placeholder={t(
                          'profile.placeholders.urlApiDayAppYourkeyTitleContent'
                        )}
                        {...field}
                        value={field.value ?? ''}
                      />
                    </FormControl>
                  </SettingsControl>
                </SettingsRow>
              )}
            />
          )}

          {notifyType === 'gotify' && (
            <>
              <FormField
                control={form.control}
                name='gotify_url'
                render={({ field }) => (
                  <SettingsRow
                    label={t('profile.fields.gotifyServerUrl')}
                    hint={t(
                      'profile.placeholders.enterTheFullUrlOfYourGotifyServer'
                    )}
                  >
                    <SettingsControl>
                      <FormControl>
                        <Input
                          placeholder={t(
                            'profile.placeholders.urlGotifyExampleCom'
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
                name='gotify_token'
                render={({ field }) => (
                  <SettingsRow
                    label={t('profile.fields.gotifyApplicationToken')}
                    hint={t(
                      'profile.tips.tokenObtainedFromYourGotifyApplication'
                    )}
                  >
                    <SettingsControl>
                      <FormControl>
                        <Input
                          type='password'
                          autoComplete='new-password'
                          placeholder={t(
                            'profile.placeholders.enterApplicationToken'
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
                name='gotify_priority'
                render={({ field }) => (
                  <SettingsRow
                    label={t('profile.fields.messagePriority')}
                    hint={t(
                      'profile.tips.priorityLevelFrom0LowestTo10HighestDefault'
                    )}
                  >
                    <SettingsControl>
                      <FormControl>
                        <Input
                          type='number'
                          min={0}
                          max={10}
                          step={1}
                          placeholder='5'
                          {...field}
                          value={Number.isNaN(field.value) ? '' : field.value}
                          onChange={(event) =>
                            field.onChange(event.target.valueAsNumber)
                          }
                        />
                      </FormControl>
                    </SettingsControl>
                  </SettingsRow>
                )}
              />
              <SettingsNotice title={t('profile.titles.setupInstructions')}>
                {t(
                  'profile.placeholders.value1CreateAnApplicationInYourGotifyServer'
                )}{' '}
                {t('profile.placeholders.value2CopyTheApplicationToken')}{' '}
                {t(
                  'profile.placeholders.value3EnterYourGotifyServerUrlAndToken'
                )}{' '}
                <a
                  href='https://gotify.net/'
                  target='_blank'
                  rel='noopener noreferrer'
                  className='text-primary hover:underline'
                >
                  {t('profile.fields.gotifyDocumentation')}
                </a>
              </SettingsNotice>
            </>
          )}

          <FormField
            control={form.control}
            name='quota_warning_threshold'
            render={({ field }) => (
              <SettingsRow
                label={t('profile.tips.quotaWarningThreshold')}
                description={t(
                  'profile.tips.getNotifiedWhenBalanceFallsBelowThisValue'
                )}
                hint={
                  field.value > 0
                    ? t('profile.tips.thresholdEquivalent', {
                        amount: formatQuotaWithCurrency(field.value, {
                          digitsLarge: 2,
                          digitsSmall: 6,
                        }),
                      })
                    : undefined
                }
              >
                <SettingsControl>
                  <FormControl>
                    <Input
                      type='number'
                      min={1}
                      step={1}
                      placeholder={t('profile.placeholders.enterThreshold')}
                      {...field}
                      value={Number.isNaN(field.value) ? '' : field.value}
                      onChange={(event) =>
                        field.onChange(event.target.valueAsNumber)
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
  )
}
