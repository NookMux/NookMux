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
import { useEffect } from 'react'
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

const rateLimitDialogSchema = z.object({
  groupName: z.string().min(1, 'common.errors.groupNameIsRequired'),
  maxRequests: z
    .number()
    .min(0, 'systemSettings.errors.mustBeNonNegative')
    .max(2147483647, 'common.errors.mustBe2147483647'),
  maxSuccess: z
    .number()
    .min(1, 'systemSettings.errors.mustBeAtLeast1')
    .max(2147483647, 'common.errors.mustBe2147483647'),
})

type RateLimitDialogFormValues = z.infer<typeof rateLimitDialogSchema>

export type RateLimitEntryData = {
  groupName: string
  maxRequests: number
  maxSuccess: number
}

type RateLimitDialogProps = {
  open: boolean
  onOpenChange: (open: boolean) => void
  onSave: (data: RateLimitEntryData) => void
  editData?: RateLimitEntryData | null
}

export function RateLimitDialog({
  open,
  onOpenChange,
  onSave,
  editData,
}: RateLimitDialogProps) {
  const { t } = useTranslation()
  const isEditMode = !!editData

  const form = useForm<RateLimitDialogFormValues>({
    resolver: zodResolver(rateLimitDialogSchema),
    defaultValues: {
      groupName: '',
      maxRequests: 0,
      maxSuccess: 1,
    },
  })

  useEffect(() => {
    if (editData) {
      form.reset(editData)
    } else {
      form.reset({
        groupName: '',
        maxRequests: 0,
        maxSuccess: 1,
      })
    }
  }, [editData, form, open])

  const handleSubmit = (values: RateLimitDialogFormValues) => {
    onSave(values)
    form.reset()
    onOpenChange(false)
  }

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className='sm:max-w-[500px]'>
        <DialogHeader>
          <DialogTitle>
            {isEditMode
              ? t('systemSettings.actions.editGroupRateLimit')
              : t('systemSettings.actions.addGroupRateLimit')}
          </DialogTitle>
          <DialogDescription>
            {t(
              'systemSettings.tips.configureRateLimitingRulesForASpecificUserGroup'
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
              name='groupName'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t('models.fields.groupName')}</FormLabel>
                  <FormControl>
                    <Input
                      placeholder={t(
                        'systemSettings.placeholders.eGDefaultVipPremium'
                      )}
                      {...field}
                      disabled={isEditMode}
                    />
                  </FormControl>
                  <FormDescription>
                    {isEditMode
                      ? t(
                          'systemSettings.errors.groupNameCannotBeChangedWhenEditing'
                        )
                      : t('systemSettings.tips.uniqueIdentifierForThisGroup')}
                  </FormDescription>
                  <FormMessage />
                </FormItem>
              )}
            />

            <FormField
              control={form.control}
              name='maxRequests'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>
                    {t('systemSettings.tips.maxRequestsIncludingFailures')}
                  </FormLabel>
                  <FormControl>
                    <div className='flex items-center gap-2'>
                      <Input
                        type='number'
                        min={0}
                        max={2147483647}
                        step={1}
                        {...field}
                        onChange={(e) =>
                          field.onChange(parseInt(e.target.value) || 0)
                        }
                      />
                      <span className='text-muted-foreground text-sm'>
                        {t('systemSettings.fields.times')}
                      </span>
                    </div>
                  </FormControl>
                  <FormDescription>
                    {t(
                      'systemSettings.tips.totalRequestsAllowedPerPeriod0Unlimited'
                    )}
                  </FormDescription>
                  <FormMessage />
                </FormItem>
              )}
            />

            <FormField
              control={form.control}
              name='maxSuccess'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>
                    {t('systemSettings.status.maxSuccessfulRequests')}
                  </FormLabel>
                  <FormControl>
                    <div className='flex items-center gap-2'>
                      <Input
                        type='number'
                        min={1}
                        max={2147483647}
                        step={1}
                        {...field}
                        onChange={(e) =>
                          field.onChange(parseInt(e.target.value) || 1)
                        }
                      />
                      <span className='text-muted-foreground text-sm'>
                        {t('systemSettings.fields.times')}
                      </span>
                    </div>
                  </FormControl>
                  <FormDescription>
                    {t(
                      'systemSettings.status.onlySuccessfulRequestsCountTowardThisLimit'
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
