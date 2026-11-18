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
import { Button } from '@/components/ui/button'
import { Combobox } from '@/components/ui/combobox'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
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

const createPaymentMethodDialogSchema = (t: (key: string) => string) =>
  z.object({
    name: z
      .string()
      .min(1, t('systemSettings.errors.paymentMethodNameIsRequired')),
    type: z
      .string()
      .min(1, t('systemSettings.errors.paymentMethodTypeIsRequired')),
    color: z.string().min(1, t('systemSettings.errors.colorIsRequired')),
    min_topup: z.string().optional(),
  })

type PaymentMethodDialogFormValues = z.infer<
  ReturnType<typeof createPaymentMethodDialogSchema>
>

export type PaymentMethodData = {
  name: string
  type: string
  color: string
  min_topup?: string
}

type PaymentMethodDialogProps = {
  open: boolean
  onOpenChange: (open: boolean) => void
  onSave: (data: PaymentMethodData) => void
  editData?: PaymentMethodData | null
}

const PAYMENT_TYPES = [
  { value: 'alipay', label: 'common.fields.alipay' },
  { value: 'wxpay', label: 'common.fields.chatPay' },
  { value: 'stripe', label: 'common.fields.stripe' },
]

const getColorPreview = (color: string) => {
  if (color.includes('var(--')) {
    return null
  }
  return color
}

const COLOR_PRESETS = [
  { value: '#1677FF', labelKey: 'systemSettings.fields.colorBlueAlipay' },
  { value: '#07C160', labelKey: 'systemSettings.fields.colorGreenWechat' },
  { value: '#635BFF', labelKey: 'systemSettings.fields.colorPurpleStripe' },
  { value: '#1890FF', labelKey: 'systemSettings.fields.colorSkyBlue' },
  { value: '#52C41A', labelKey: 'systemSettings.fields.colorLimeGreen' },
  { value: 'black', labelKey: 'systemSettings.fields.colorBlack' },
  { value: '#FF4D4F', labelKey: 'systemSettings.fields.colorRed' },
  { value: '#FFA940', labelKey: 'systemSettings.fields.colorOrange' },
].map((preset) => {
  const previewColor = getColorPreview(preset.value)
  return {
    ...preset,
    icon: previewColor ? (
      <div
        className='size-4 rounded border'
        style={{ backgroundColor: previewColor }}
      />
    ) : (
      <div className='bg-muted size-4 rounded border' />
    ),
  }
})

export function PaymentMethodDialog({
  open,
  onOpenChange,
  onSave,
  editData,
}: PaymentMethodDialogProps) {
  const { t } = useTranslation()
  const isEditMode = !!editData
  const paymentMethodDialogSchema = createPaymentMethodDialogSchema(t)
  const paymentTypeOptions = useMemo(
    () => PAYMENT_TYPES.map((item) => ({ ...item, label: t(item.label) })),
    [t]
  )
  const colorPresetOptions = useMemo(
    () =>
      COLOR_PRESETS.map((item) => ({
        value: item.value,
        label: t(item.labelKey),
        icon: item.icon,
      })),
    [t]
  )

  const form = useForm<PaymentMethodDialogFormValues>({
    resolver: zodResolver(paymentMethodDialogSchema),
    defaultValues: {
      name: '',
      type: '',
      color: '',
      min_topup: '',
    },
  })

  const colorValue = form.watch('color')

  const colorPreview = useMemo(() => {
    if (!colorValue) return null
    try {
      // For CSS variables like rgba(var(--semi-blue-5), 1), we can't preview accurately
      // but we can detect common patterns
      if (colorValue.includes('var(--')) {
        return null // Can't preview CSS variables reliably
      }
      return colorValue
    } catch {
      return null
    }
  }, [colorValue])

  useEffect(() => {
    if (editData) {
      form.reset(editData)
    } else {
      form.reset({
        name: '',
        type: '',
        color: '',
        min_topup: '',
      })
    }
  }, [editData, form, open])

  const handleSubmit = (values: PaymentMethodDialogFormValues) => {
    const data: PaymentMethodData = {
      name: values.name,
      type: values.type,
      color: values.color,
    }
    if (values.min_topup && values.min_topup.trim() !== '') {
      data.min_topup = values.min_topup
    }
    onSave(data)
    form.reset()
    onOpenChange(false)
  }

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className='sm:max-w-[500px]'>
        <DialogHeader>
          <DialogTitle>
            {isEditMode
              ? t('systemSettings.actions.editPaymentMethod')
              : t('systemSettings.actions.addPaymentMethod')}
          </DialogTitle>
          <DialogDescription>
            {t(
              'systemSettings.tips.configureAPaymentMethodForUserRechargeOptions'
            )}
          </DialogDescription>
        </DialogHeader>

        <Form {...form}>
          <form
            onSubmit={form.handleSubmit(handleSubmit)}
            className='space-y-4'
          >
            <FormField
              control={form.control}
              name='name'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t('channels.fields.name')}</FormLabel>
                  <FormControl>
                    <Input
                      placeholder={t(
                        'systemSettings.placeholders.eGAlipayWeChat'
                      )}
                      {...field}
                    />
                  </FormControl>
                  <FormDescription>
                    {t('systemSettings.tips.displayNameForThisPaymentMethod')}
                  </FormDescription>
                  <FormMessage />
                </FormItem>
              )}
            />

            <FormField
              control={form.control}
              name='type'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t('channels.fields.type')}</FormLabel>
                  <FormControl>
                    <Combobox
                      options={paymentTypeOptions}
                      value={field.value}
                      onValueChange={field.onChange}
                      placeholder={t(
                        'systemSettings.placeholders.selectOrEnterPaymentType'
                      )}
                      searchPlaceholder={t(
                        'systemSettings.actions.searchPaymentTypes'
                      )}
                      allowCustomValue
                    />
                  </FormControl>
                  <FormDescription>
                    {t(
                      'systemSettings.placeholders.selectFromPresetsOrTypeCustomIdentifier'
                    )}
                  </FormDescription>
                  <FormMessage />
                </FormItem>
              )}
            />

            <FormField
              control={form.control}
              name='color'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t('systemSettings.fields.color')}</FormLabel>
                  <FormControl>
                    <div className='flex items-center gap-2'>
                      <Combobox
                        options={colorPresetOptions}
                        value={field.value}
                        onValueChange={field.onChange}
                        placeholder={t(
                          'systemSettings.placeholders.selectOrEnterColorValue'
                        )}
                        searchPlaceholder={t(
                          'systemSettings.actions.searchColors'
                        )}
                        allowCustomValue
                        className='flex-1'
                      />
                      {colorPreview && (
                        <div
                          className='size-9 shrink-0 rounded border'
                          style={{ backgroundColor: colorPreview }}
                          title={colorPreview}
                        />
                      )}
                    </div>
                  </FormControl>
                  <FormDescription>
                    {t(
                      'systemSettings.placeholders.selectPresetOrEnterCustomCssColorValue'
                    )}
                  </FormDescription>
                  <FormMessage />
                </FormItem>
              )}
            />

            <FormField
              control={form.control}
              name='min_topup'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>
                    {t('systemSettings.fields.minimumTopUpOptional')}
                  </FormLabel>
                  <FormControl>
                    <Input
                      type='number'
                      step='0.01'
                      placeholder={t('systemSettings.placeholders.eG50')}
                      {...field}
                    />
                  </FormControl>
                  <FormDescription>
                    {t(
                      'systemSettings.tips.optionalMinimumRechargeAmountForThisMethod'
                    )}
                  </FormDescription>
                  <FormMessage />
                </FormItem>
              )}
            />

            <DialogFooter>
              <Button
                type='button'
                variant='outline'
                onClick={() => onOpenChange(false)}
              >
                {t('common.actions.cancel')}
              </Button>
              <Button type='submit'>
                {isEditMode
                  ? t('channels.fields.update')
                  : t('channels.actions.add')}
              </Button>
            </DialogFooter>
          </form>
        </Form>
      </DialogContent>
    </Dialog>
  )
}
