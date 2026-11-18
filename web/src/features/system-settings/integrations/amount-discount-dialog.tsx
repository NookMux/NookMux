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

const createAmountDiscountDialogSchema = (t: (key: string) => string) =>
  z.object({
    amount: z
      .number()
      .positive(t('systemSettings.errors.amountMustBeGreaterThan0'))
      .int(t('systemSettings.errors.amountMustBeAWholeNumber')),
    discountRate: z
      .number()
      .positive(t('systemSettings.errors.discountRateMustBeGreaterThan0'))
      .max(1, t('systemSettings.errors.discountRateMustBe1')),
  })

type AmountDiscountDialogFormValues = z.infer<
  ReturnType<typeof createAmountDiscountDialogSchema>
>

export type AmountDiscountData = {
  amount: number
  discountRate: number
}

type AmountDiscountDialogProps = {
  open: boolean
  onOpenChange: (open: boolean) => void
  onSave: (data: AmountDiscountData) => void
  editData?: AmountDiscountData | null
}

export function AmountDiscountDialog({
  open,
  onOpenChange,
  onSave,
  editData,
}: AmountDiscountDialogProps) {
  const { t } = useTranslation()
  const isEditMode = !!editData
  const amountDiscountDialogSchema = createAmountDiscountDialogSchema(t)

  const form = useForm<AmountDiscountDialogFormValues>({
    resolver: zodResolver(amountDiscountDialogSchema),
    defaultValues: {
      amount: 0,
      discountRate: 1,
    },
  })

  const discountRate = form.watch('discountRate')

  const discountPercentage = useMemo(() => {
    if (!discountRate || discountRate >= 1) return 0
    return Math.round((1 - discountRate) * 100)
  }, [discountRate])

  useEffect(() => {
    if (editData) {
      form.reset(editData)
    } else {
      form.reset({
        amount: 0,
        discountRate: 1,
      })
    }
  }, [editData, form, open])

  const handleSubmit = (values: AmountDiscountDialogFormValues) => {
    onSave({
      amount: values.amount,
      discountRate: values.discountRate,
    })
    form.reset()
    onOpenChange(false)
  }

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className='sm:max-w-[500px]'>
        <DialogHeader>
          <DialogTitle>
            {isEditMode
              ? t('systemSettings.actions.editDiscountTier')
              : t('systemSettings.actions.addDiscountTier')}
          </DialogTitle>
          <DialogDescription>
            {t(
              'systemSettings.tips.setADiscountRateForASpecificRechargeAmount'
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
              name='amount'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>
                    {t('systemSettings.fields.rechargeAmountUsd')}
                  </FormLabel>
                  <FormControl>
                    <Input
                      type='number'
                      step='1'
                      min='1'
                      placeholder={t('systemSettings.placeholders.eG100')}
                      {...field}
                      onChange={(e) =>
                        field.onChange(parseInt(e.target.value) || 0)
                      }
                      disabled={isEditMode}
                    />
                  </FormControl>
                  <FormDescription>
                    {isEditMode
                      ? t(
                          'systemSettings.errors.amountCannotBeChangedWhenEditing'
                        )
                      : t(
                          'systemSettings.tips.minimumRechargeAmountToQualifyForThisDiscount'
                        )}
                  </FormDescription>
                  <FormMessage />
                </FormItem>
              )}
            />

            <FormField
              control={form.control}
              name='discountRate'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>
                    {t('systemSettings.fields.discountRate')}
                  </FormLabel>
                  <FormControl>
                    <Input
                      type='number'
                      step='0.01'
                      min='0.01'
                      max='1'
                      placeholder={t('systemSettings.placeholders.eG095')}
                      {...field}
                      onChange={(e) =>
                        field.onChange(parseFloat(e.target.value) || 0)
                      }
                    />
                  </FormControl>
                  <FormDescription>
                    {t('systemSettings.tips.finalPriceMultiplier0955Discount')}
                    {discountPercentage > 0 && (
                      <span className='ml-1 font-medium text-green-600 dark:text-green-400'>
                        ({discountPercentage}
                        {t('systemSettings.fields.off')})
                      </span>
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
