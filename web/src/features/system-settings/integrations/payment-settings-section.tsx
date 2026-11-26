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
import * as React from 'react'
import * as z from 'zod'
import { useForm } from 'react-hook-form'
import { zodResolver } from '@hookform/resolvers/zod'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { Form, FormControl, FormField } from '@/components/ui/form'
import { Input } from '@/components/ui/input'
import { Switch } from '@/components/ui/switch'
import { Textarea } from '@/components/ui/textarea'
import { CopyButton } from '@/components/copy-button'
import { EditorModeButton } from '../components/editor-mode-button'
import { SettingsForm } from '../components/settings-form-layout'
import {
  SettingsControl,
  SettingsGroup,
  SettingsRow,
} from '../components/settings-group'
import { SettingsPageFormActions } from '../components/settings-page-context'
import { SettingsSection } from '../components/settings-section'
import { useUpdateOption } from '../hooks/use-update-option'
import { AmountDiscountVisualEditor } from './amount-discount-visual-editor'
import { AmountOptionsVisualEditor } from './amount-options-visual-editor'
import { PaymentMethodsVisualEditor } from './payment-methods-visual-editor'
import {
  formatJsonForEditor,
  getJsonError,
  normalizeJsonForComparison,
  removeTrailingSlash,
} from './utils'

const paymentSchema = z.object({
  PayAddress: z.string().refine((value) => {
    const trimmed = value.trim()
    if (!trimmed) return true
    return /^https?:\/\//.test(trimmed)
  }, 'systemSettings.errors.provideValidPaymentUrl'),
  EpayId: z.string(),
  EpayKey: z.string(),
  Price: z.number().min(0, 'systemSettings.errors.mustBeNonNegative'),
  MinTopUp: z.number().min(0, 'systemSettings.errors.mustBeNonNegative'),
  CustomCallbackAddress: z.string().refine((value) => {
    const trimmed = value.trim()
    if (!trimmed) return true
    return /^https?:\/\//.test(trimmed)
  }, 'systemSettings.errors.provideValidCallbackUrl'),
  PayMethods: z.string().superRefine((value, ctx) => {
    const error = getJsonError(value)
    if (error) {
      ctx.addIssue({ code: z.ZodIssueCode.custom, message: error })
    }
  }),
  AmountOptions: z.string().superRefine((value, ctx) => {
    const error = getJsonError(value, (parsed) => Array.isArray(parsed))
    if (error) {
      ctx.addIssue({ code: z.ZodIssueCode.custom, message: error })
    }
  }),
  AmountDiscount: z.string().superRefine((value, ctx) => {
    const error = getJsonError(
      value,
      (parsed) =>
        !!parsed && typeof parsed === 'object' && !Array.isArray(parsed)
    )
    if (error) {
      ctx.addIssue({ code: z.ZodIssueCode.custom, message: error })
    }
  }),
  StripeApiSecret: z.string(),
  StripeWebhookSecret: z.string(),
  StripePriceId: z.string(),
  StripeUnitPrice: z.number().min(0, 'systemSettings.errors.mustBeNonNegative'),
  StripeMinTopUp: z.number().min(0, 'systemSettings.errors.mustBeNonNegative'),
  StripePromotionCodesEnabled: z.boolean(),
  TopUpLink: z.string(),
})

type PaymentFormValues = z.infer<typeof paymentSchema>

type PaymentSettingsSectionProps = {
  defaultValues: PaymentFormValues
  serverAddress: string
}

export function PaymentSettingsSection({
  defaultValues,
  serverAddress,
}: PaymentSettingsSectionProps) {
  const { t } = useTranslation()
  const updateOption = useUpdateOption()
  const initialRef = React.useRef(defaultValues)
  const defaultsSignature = React.useMemo(
    () => JSON.stringify(defaultValues),
    [defaultValues]
  )

  const [payMethodsVisualMode, setPayMethodsVisualMode] = React.useState(true)
  const [amountOptionsVisualMode, setAmountOptionsVisualMode] =
    React.useState(true)
  const [amountDiscountVisualMode, setAmountDiscountVisualMode] =
    React.useState(true)

  const form = useForm<PaymentFormValues>({
    resolver: zodResolver(paymentSchema),
    mode: 'onChange',
    defaultValues: {
      ...defaultValues,
      PayMethods: formatJsonForEditor(defaultValues.PayMethods),
      AmountOptions: formatJsonForEditor(defaultValues.AmountOptions),
      AmountDiscount: formatJsonForEditor(defaultValues.AmountDiscount),
    },
  })

  React.useEffect(() => {
    const parsedDefaults = JSON.parse(defaultsSignature) as PaymentFormValues
    initialRef.current = parsedDefaults
    form.reset({
      ...parsedDefaults,
      PayMethods: formatJsonForEditor(parsedDefaults.PayMethods),
      AmountOptions: formatJsonForEditor(parsedDefaults.AmountOptions),
      AmountDiscount: formatJsonForEditor(parsedDefaults.AmountDiscount),
    })
  }, [defaultsSignature, form])

  const onSubmit = async (values: PaymentFormValues) => {
    if (!serverAddress.trim()) {
      toast.error(t('systemSettings.tips.pleaseConfigureServerAddressFirst'))
      return
    }

    const sanitized = {
      PayAddress: removeTrailingSlash(values.PayAddress),
      EpayId: values.EpayId.trim(),
      EpayKey: values.EpayKey.trim(),
      Price: values.Price,
      MinTopUp: values.MinTopUp,
      CustomCallbackAddress: removeTrailingSlash(values.CustomCallbackAddress),
      PayMethods: values.PayMethods.trim(),
      AmountOptions: values.AmountOptions.trim(),
      AmountDiscount: values.AmountDiscount.trim(),
      StripeApiSecret: values.StripeApiSecret.trim(),
      StripeWebhookSecret: values.StripeWebhookSecret.trim(),
      StripePriceId: values.StripePriceId.trim(),
      StripeUnitPrice: values.StripeUnitPrice,
      StripeMinTopUp: values.StripeMinTopUp,
      StripePromotionCodesEnabled: values.StripePromotionCodesEnabled,
      TopUpLink: values.TopUpLink,
    }

    const initial = {
      PayAddress: removeTrailingSlash(initialRef.current.PayAddress),
      EpayId: initialRef.current.EpayId.trim(),
      EpayKey: initialRef.current.EpayKey.trim(),
      Price: initialRef.current.Price,
      MinTopUp: initialRef.current.MinTopUp,
      CustomCallbackAddress: removeTrailingSlash(
        initialRef.current.CustomCallbackAddress
      ),
      PayMethods: initialRef.current.PayMethods.trim(),
      AmountOptions: initialRef.current.AmountOptions.trim(),
      AmountDiscount: initialRef.current.AmountDiscount.trim(),
      StripePriceId: initialRef.current.StripePriceId.trim(),
      StripeUnitPrice: initialRef.current.StripeUnitPrice,
      StripeMinTopUp: initialRef.current.StripeMinTopUp,
      StripePromotionCodesEnabled:
        initialRef.current.StripePromotionCodesEnabled,
      TopUpLink: initialRef.current.TopUpLink,
    }

    const updates: Array<{ key: string; value: string | number | boolean }> = []

    if (sanitized.PayAddress !== initial.PayAddress) {
      updates.push({ key: 'PayAddress', value: sanitized.PayAddress })
    }
    if (sanitized.EpayId !== initial.EpayId) {
      updates.push({ key: 'EpayId', value: sanitized.EpayId })
    }
    if (sanitized.EpayKey) {
      updates.push({ key: 'EpayKey', value: sanitized.EpayKey })
    }
    if (sanitized.Price !== initial.Price) {
      updates.push({ key: 'Price', value: sanitized.Price })
    }
    if (sanitized.MinTopUp !== initial.MinTopUp) {
      updates.push({ key: 'MinTopUp', value: sanitized.MinTopUp })
    }
    if (sanitized.CustomCallbackAddress !== initial.CustomCallbackAddress) {
      updates.push({
        key: 'CustomCallbackAddress',
        value: sanitized.CustomCallbackAddress,
      })
    }
    if (
      normalizeJsonForComparison(sanitized.PayMethods) !==
      normalizeJsonForComparison(initial.PayMethods)
    ) {
      updates.push({ key: 'PayMethods', value: sanitized.PayMethods })
    }
    if (
      normalizeJsonForComparison(sanitized.AmountOptions) !==
      normalizeJsonForComparison(initial.AmountOptions)
    ) {
      updates.push({
        key: 'payment_setting.amount_options',
        value: sanitized.AmountOptions,
      })
    }
    if (
      normalizeJsonForComparison(sanitized.AmountDiscount) !==
      normalizeJsonForComparison(initial.AmountDiscount)
    ) {
      updates.push({
        key: 'payment_setting.amount_discount',
        value: sanitized.AmountDiscount,
      })
    }
    if (sanitized.StripeApiSecret) {
      updates.push({ key: 'StripeApiSecret', value: sanitized.StripeApiSecret })
    }
    if (sanitized.StripeWebhookSecret) {
      updates.push({
        key: 'StripeWebhookSecret',
        value: sanitized.StripeWebhookSecret,
      })
    }
    if (sanitized.StripePriceId !== initial.StripePriceId) {
      updates.push({ key: 'StripePriceId', value: sanitized.StripePriceId })
    }
    if (sanitized.StripeUnitPrice !== initial.StripeUnitPrice) {
      updates.push({ key: 'StripeUnitPrice', value: sanitized.StripeUnitPrice })
    }
    if (sanitized.StripeMinTopUp !== initial.StripeMinTopUp) {
      updates.push({ key: 'StripeMinTopUp', value: sanitized.StripeMinTopUp })
    }
    if (
      sanitized.StripePromotionCodesEnabled !==
      initial.StripePromotionCodesEnabled
    ) {
      updates.push({
        key: 'StripePromotionCodesEnabled',
        value: sanitized.StripePromotionCodesEnabled,
      })
    }
    if (sanitized.TopUpLink !== initial.TopUpLink) {
      updates.push({ key: 'TopUpLink', value: sanitized.TopUpLink })
    }

    if (updates.length === 0) {
      toast.info(t('channels.fields.noChangesToSave'))
      return
    }

    for (const update of updates) {
      await updateOption.mutateAsync(update)
    }
  }

  const hasServerAddress = serverAddress.trim() !== ''
  const stripeWebhookUrl = hasServerAddress
    ? `${removeTrailingSlash(serverAddress)}/api/stripe/webhook`
    : '<ServerAddress>/api/stripe/webhook'

  return (
    <SettingsSection title={t('systemSettings.fields.paymentGateway')}>
      <Form {...form}>
        <SettingsForm
          onSubmit={form.handleSubmit(onSubmit)}
          data-no-autosubmit='true'
        >
          <SettingsPageFormActions
            onSave={form.handleSubmit(onSubmit)}
            isSaving={updateOption.isPending}
            saveLabel='systemSettings.actions.savePaymentSettings'
          />

          <SettingsGroup
            title={t('systemSettings.fields.epayGateway')}
            description={t(
              'systemSettings.tips.classicPaymentGatewayConfiguration'
            )}
          >
            <FormField
              control={form.control}
              name='PayAddress'
              render={({ field }) => (
                <SettingsRow
                  label={t('systemSettings.fields.paymentAddress')}
                  description={t(
                    'systemSettings.fields.epayServiceBaseAddress'
                  )}
                >
                  <SettingsControl>
                    <FormControl>
                      <Input
                        placeholder={t(
                          'systemSettings.placeholders.urlPayExampleCom'
                        )}
                        {...field}
                        onChange={(event) => field.onChange(event.target.value)}
                      />
                    </FormControl>
                  </SettingsControl>
                </SettingsRow>
              )}
            />

            <FormField
              control={form.control}
              name='CustomCallbackAddress'
              render={({ field }) => (
                <SettingsRow
                  label={t('systemSettings.fields.callbackAddress')}
                  description={t(
                    'systemSettings.fields.optionalCallbackOverride'
                  )}
                >
                  <SettingsControl>
                    <FormControl>
                      <Input
                        placeholder={t(
                          'systemSettings.placeholders.urlGatewayExampleCom'
                        )}
                        {...field}
                        onChange={(event) => field.onChange(event.target.value)}
                      />
                    </FormControl>
                  </SettingsControl>
                </SettingsRow>
              )}
            />

            <FormField
              control={form.control}
              name='EpayId'
              render={({ field }) => (
                <SettingsRow label={t('systemSettings.fields.epayMerchantId')}>
                  <SettingsControl>
                    <FormControl>
                      <Input
                        placeholder='10001'
                        autoComplete='off'
                        {...field}
                        onChange={(event) => field.onChange(event.target.value)}
                      />
                    </FormControl>
                  </SettingsControl>
                </SettingsRow>
              )}
            />

            <FormField
              control={form.control}
              name='EpayKey'
              render={({ field }) => (
                <SettingsRow
                  label={t('systemSettings.fields.epayMerchantKey')}
                  description={t(
                    'systemSettings.tips.leaveBlankUnlessRotatingTheSecret'
                  )}
                >
                  <SettingsControl>
                    <FormControl>
                      <Input
                        type='password'
                        placeholder={t(
                          'systemSettings.placeholders.enterNewKeyToUpdate'
                        )}
                        autoComplete='new-password'
                        {...field}
                        onChange={(event) => field.onChange(event.target.value)}
                      />
                    </FormControl>
                  </SettingsControl>
                </SettingsRow>
              )}
            />

            <FormField
              control={form.control}
              name='Price'
              render={({ field }) => (
                <SettingsRow
                  label={t('systemSettings.fields.topUpPriceLocalUsd')}
                >
                  <SettingsControl size='xs'>
                    <FormControl>
                      <Input
                        type='number'
                        step='0.01'
                        min={0}
                        value={(field.value ?? 0) as number}
                        onChange={(event) =>
                          field.onChange(event.target.valueAsNumber)
                        }
                      />
                    </FormControl>
                  </SettingsControl>
                </SettingsRow>
              )}
            />

            <FormField
              control={form.control}
              name='MinTopUp'
              render={({ field }) => (
                <SettingsRow label={t('systemSettings.fields.minimumTopUpUsd')}>
                  <SettingsControl size='xs'>
                    <FormControl>
                      <Input
                        type='number'
                        step='0.01'
                        min={0}
                        value={(field.value ?? 0) as number}
                        onChange={(event) =>
                          field.onChange(event.target.valueAsNumber)
                        }
                      />
                    </FormControl>
                  </SettingsControl>
                </SettingsRow>
              )}
            />

            <FormField
              control={form.control}
              name='PayMethods'
              render={({ field }) => (
                <SettingsRow
                  label={t('orderQuery.fields.paymentMethod')}
                  description={
                    payMethodsVisualMode
                      ? t('systemSettings.tips.paymentMethodsDisplayedToUsers')
                      : t(
                          'systemSettings.tips.rechargeMethodConfigurationAsJson'
                        )
                  }
                  stacked
                  actions={
                    <EditorModeButton
                      visualMode={payMethodsVisualMode}
                      onToggle={() =>
                        setPayMethodsVisualMode((current) => !current)
                      }
                    />
                  }
                >
                  <FormControl>
                    {payMethodsVisualMode ? (
                      <PaymentMethodsVisualEditor
                        value={field.value}
                        onChange={field.onChange}
                      />
                    ) : (
                      <Textarea
                        rows={4}
                        placeholder={t('common.tips.nameTypeAlipayColor1677Ff')}
                        {...field}
                        onChange={(event) => field.onChange(event.target.value)}
                      />
                    )}
                  </FormControl>
                </SettingsRow>
              )}
            />

            <FormField
              control={form.control}
              name='AmountOptions'
              render={({ field }) => (
                <SettingsRow
                  label={t('systemSettings.fields.topUpAmountOptions')}
                  description={
                    amountOptionsVisualMode
                      ? t(
                          'systemSettings.tips.presetRechargeAmountsDisplayedToUsers'
                        )
                      : t(
                          'systemSettings.tips.presetRechargeAmountsAsAJsonArray'
                        )
                  }
                  stacked
                  actions={
                    <EditorModeButton
                      visualMode={amountOptionsVisualMode}
                      onToggle={() =>
                        setAmountOptionsVisualMode((current) => !current)
                      }
                    />
                  }
                >
                  <FormControl>
                    {amountOptionsVisualMode ? (
                      <AmountOptionsVisualEditor
                        value={field.value}
                        onChange={field.onChange}
                      />
                    ) : (
                      <Textarea
                        rows={4}
                        placeholder='[10, 20, 50, 100, 200, 500]'
                        {...field}
                        onChange={(event) => field.onChange(event.target.value)}
                      />
                    )}
                  </FormControl>
                </SettingsRow>
              )}
            />

            <FormField
              control={form.control}
              name='AmountDiscount'
              render={({ field }) => (
                <SettingsRow
                  label={t('systemSettings.fields.amountDiscount')}
                  description={
                    amountDiscountVisualMode
                      ? t(
                          'systemSettings.tips.configureDiscountRatesBasedOnRechargeAmounts'
                        )
                      : t(
                          'systemSettings.tips.discountMapByRechargeAmountAsAJsonObject'
                        )
                  }
                  stacked
                  actions={
                    <EditorModeButton
                      visualMode={amountDiscountVisualMode}
                      onToggle={() =>
                        setAmountDiscountVisualMode((current) => !current)
                      }
                    />
                  }
                >
                  <FormControl>
                    {amountDiscountVisualMode ? (
                      <AmountDiscountVisualEditor
                        value={field.value}
                        onChange={field.onChange}
                      />
                    ) : (
                      <Textarea
                        rows={4}
                        placeholder='{"100":0.95,"200":0.9,"500":0.85}'
                        {...field}
                        onChange={(event) => field.onChange(event.target.value)}
                      />
                    )}
                  </FormControl>
                </SettingsRow>
              )}
            />
          </SettingsGroup>

          <SettingsGroup
            title={t('systemSettings.fields.stripeGateway')}
            description={t(
              'systemSettings.tips.stripePaymentGatewayConfiguration'
            )}
          >
            <SettingsRow
              label={t('systemSettings.titles.webhookConfiguration')}
              hint={
                <div className='space-y-1'>
                  <p className='font-semibold'>
                    {t('systemSettings.errors.requiredEvents')}
                  </p>
                  <ul className='list-inside list-disc space-y-0.5 pl-2'>
                    <li>
                      <code>checkout.session.completed</code>
                    </li>
                    <li>
                      <code>checkout.session.expired</code>
                    </li>
                  </ul>
                </div>
              }
            >
              <SettingsControl size='lg'>
                <div className='flex min-w-0 items-center gap-2'>
                  <span className='text-muted-foreground shrink-0 text-sm'>
                    {t('profile.fields.webhookUrl')}
                  </span>
                  <code
                    className='bg-muted min-w-0 truncate rounded-md px-2 py-1'
                    title={stripeWebhookUrl}
                  >
                    {stripeWebhookUrl}
                  </code>
                  {hasServerAddress && (
                    <CopyButton value={stripeWebhookUrl} size='sm' />
                  )}
                </div>
              </SettingsControl>
            </SettingsRow>

            <FormField
              control={form.control}
              name='StripeApiSecret'
              render={({ field }) => (
                <SettingsRow
                  label={t('systemSettings.fields.apiSecret')}
                  description={t(
                    'systemSettings.tips.leaveBlankUnlessRotatingTheSecret'
                  )}
                >
                  <SettingsControl>
                    <FormControl>
                      <Input
                        type='password'
                        placeholder={t(
                          'systemSettings.placeholders.skXxxOrRkXxx'
                        )}
                        autoComplete='new-password'
                        {...field}
                        onChange={(event) => field.onChange(event.target.value)}
                      />
                    </FormControl>
                  </SettingsControl>
                </SettingsRow>
              )}
            />

            <FormField
              control={form.control}
              name='StripeWebhookSecret'
              render={({ field }) => (
                <SettingsRow
                  label={t('profile.fields.webhookSecret')}
                  description={t(
                    'systemSettings.tips.leaveBlankUnlessRotatingTheSecret'
                  )}
                >
                  <SettingsControl>
                    <FormControl>
                      <Input
                        type='password'
                        placeholder={t('systemSettings.placeholders.whsecXxx')}
                        autoComplete='new-password'
                        {...field}
                        onChange={(event) => field.onChange(event.target.value)}
                      />
                    </FormControl>
                  </SettingsControl>
                </SettingsRow>
              )}
            />

            <FormField
              control={form.control}
              name='StripePriceId'
              render={({ field }) => (
                <SettingsRow label={t('systemSettings.fields.priceId')}>
                  <SettingsControl>
                    <FormControl>
                      <Input
                        placeholder='price_xxx'
                        {...field}
                        onChange={(event) => field.onChange(event.target.value)}
                      />
                    </FormControl>
                  </SettingsControl>
                </SettingsRow>
              )}
            />

            <FormField
              control={form.control}
              name='StripeUnitPrice'
              render={({ field }) => (
                <SettingsRow
                  label={t('systemSettings.fields.topUpPriceLocalUsd')}
                >
                  <SettingsControl size='xs'>
                    <FormControl>
                      <Input
                        type='number'
                        step='0.01'
                        min={0}
                        value={(field.value ?? 0) as number}
                        onChange={(event) =>
                          field.onChange(event.target.valueAsNumber)
                        }
                      />
                    </FormControl>
                  </SettingsControl>
                </SettingsRow>
              )}
            />

            <FormField
              control={form.control}
              name='StripeMinTopUp'
              render={({ field }) => (
                <SettingsRow label={t('systemSettings.fields.minimumTopUpUsd')}>
                  <SettingsControl size='xs'>
                    <FormControl>
                      <Input
                        type='number'
                        step='0.01'
                        min={0}
                        value={(field.value ?? 0) as number}
                        onChange={(event) =>
                          field.onChange(event.target.valueAsNumber)
                        }
                      />
                    </FormControl>
                  </SettingsControl>
                </SettingsRow>
              )}
            />

            <FormField
              control={form.control}
              name='StripePromotionCodesEnabled'
              render={({ field }) => (
                <SettingsRow
                  label={t('systemSettings.fields.promotionCodes')}
                  description={t(
                    'systemSettings.tips.allowUsersToEnterStripePromotionCodes'
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

          <SettingsGroup title={t('systemSettings.fields.cdkRedemption')}>
            <FormField
              control={form.control}
              name='TopUpLink'
              render={({ field }) => (
                <SettingsRow label={t('systemSettings.fields.topUpLink')}>
                  <SettingsControl>
                    <FormControl>
                      <Input
                        placeholder={t(
                          'systemSettings.placeholders.urlExampleComTopup'
                        )}
                        {...field}
                        onChange={(event) => field.onChange(event.target.value)}
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
