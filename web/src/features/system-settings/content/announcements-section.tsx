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
import { useEffect, useMemo, useState } from 'react'
import * as z from 'zod'
import { useForm } from 'react-hook-form'
import { zodResolver } from '@hookform/resolvers/zod'
import { Plus, Edit, Trash2, Save } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import dayjs from '@/lib/dayjs'
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from '@/components/ui/alert-dialog'
import { Button } from '@/components/ui/button'
import { Checkbox } from '@/components/ui/checkbox'
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
import {
  Select,
  SelectContent,
  SelectGroup,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'
import { Textarea } from '@/components/ui/textarea'
import { DateTimePicker } from '@/components/datetime-picker'
import { StatusBadge } from '@/components/status-badge'
import { SettingsSection } from '../components/settings-section'
import { useUpdateOption } from '../hooks/use-update-option'

type Announcement = {
  id: number
  content: string
  publishDate: string
  type: 'default' | 'ongoing' | 'success' | 'warning' | 'error'
  extra?: string
}

type AnnouncementsSectionProps = {
  data: string
}

const announcementSchema = z.object({
  content: z
    .string()
    .min(1, 'systemSettings.errors.contentIsRequired')
    .max(500, 'systemSettings.errors.contentMax500Chars'),
  publishDate: z.string().min(1, 'systemSettings.errors.publishDateIsRequired'),
  type: z.enum(['default', 'ongoing', 'success', 'warning', 'error']),
  extra: z
    .string()
    .max(100, 'systemSettings.errors.extraMax100Chars')
    .optional(),
})

type AnnouncementFormValues = z.infer<typeof announcementSchema>

const typeOptions = [
  {
    value: 'default',
    label: 'common.fields.default',
    color: 'bg-gray-500',
    badgeVariant: 'neutral' as const,
  },
  {
    value: 'ongoing',
    label: 'common.fields.progress',
    color: 'bg-blue-500',
    badgeVariant: 'info' as const,
  },
  {
    value: 'success',
    label: 'channels.status.success',
    color: 'bg-green-500',
    badgeVariant: 'success' as const,
  },
  {
    value: 'warning',
    label: 'systemSettings.tips.warning',
    color: 'bg-orange-500',
    badgeVariant: 'warning' as const,
  },
  {
    value: 'error',
    label: 'common.errors.error',
    color: 'bg-red-500',
    badgeVariant: 'danger' as const,
  },
]

export function AnnouncementsSection({ data }: AnnouncementsSectionProps) {
  const { t } = useTranslation()
  const updateOption = useUpdateOption()
  const [announcements, setAnnouncements] = useState<Announcement[]>([])
  const [hasChanges, setHasChanges] = useState(false)
  const [selectedIds, setSelectedIds] = useState<number[]>([])
  const [showDialog, setShowDialog] = useState(false)
  const [showDeleteDialog, setShowDeleteDialog] = useState(false)
  const [editingAnnouncement, setEditingAnnouncement] =
    useState<Announcement | null>(null)
  const [deleteTarget, setDeleteTarget] = useState<'single' | 'batch'>('single')

  const form = useForm<AnnouncementFormValues>({
    resolver: zodResolver(announcementSchema),
    defaultValues: {
      content: '',
      publishDate: new Date().toISOString(),
      type: 'default',
      extra: '',
    },
  })

  useEffect(() => {
    try {
      const parsed = JSON.parse(data || '[]')
      if (Array.isArray(parsed)) {
        setAnnouncements(
          parsed.map((item, idx) => ({
            ...item,
            id: item.id || idx + 1,
          }))
        )
      }
    } catch {
      setAnnouncements([])
    }
  }, [data])

  const handleAdd = () => {
    setEditingAnnouncement(null)
    form.reset({
      content: '',
      publishDate: new Date().toISOString(),
      type: 'default',
      extra: '',
    })
    setShowDialog(true)
  }

  const handleEdit = (announcement: Announcement) => {
    setEditingAnnouncement(announcement)
    form.reset({
      content: announcement.content,
      publishDate: announcement.publishDate,
      type: announcement.type,
      extra: announcement.extra || '',
    })
    setShowDialog(true)
  }

  const handleDelete = (announcement: Announcement) => {
    setEditingAnnouncement(announcement)
    setDeleteTarget('single')
    setShowDeleteDialog(true)
  }

  const handleBatchDelete = () => {
    if (selectedIds.length === 0) {
      toast.error(t('systemSettings.errors.pleaseSelectItemsToDelete'))
      return
    }
    setDeleteTarget('batch')
    setShowDeleteDialog(true)
  }

  const confirmDelete = () => {
    if (deleteTarget === 'single' && editingAnnouncement) {
      setAnnouncements((prev) =>
        prev.filter((item) => item.id !== editingAnnouncement.id)
      )
      setHasChanges(true)
      toast.success(
        t('common.status.announcementDeletedClickSaveSettingsToApply')
      )
    } else if (deleteTarget === 'batch') {
      setAnnouncements((prev) =>
        prev.filter((item) => !selectedIds.includes(item.id))
      )
      setSelectedIds([])
      setHasChanges(true)
      toast.success(
        t(
          'systemSettings.status.countAnnouncementsDeletedClickSaveSettingsToApply',
          {
            count: selectedIds.length,
          }
        )
      )
    }
    setShowDeleteDialog(false)
    setEditingAnnouncement(null)
  }

  const handleSubmitForm = (values: AnnouncementFormValues) => {
    if (editingAnnouncement) {
      setAnnouncements((prev) =>
        prev.map((item) =>
          item.id === editingAnnouncement.id ? { ...item, ...values } : item
        )
      )
      toast.success(
        t('common.status.announcementUpdatedClickSaveSettingsToApply')
      )
    } else {
      const newId = Math.max(...announcements.map((item) => item.id), 0) + 1
      setAnnouncements((prev) => [...prev, { id: newId, ...values }])
      toast.success(t('common.tips.announcementAddedClickSaveSettingsToApply'))
    }
    setHasChanges(true)
    setShowDialog(false)
  }

  const handleSaveAll = async () => {
    try {
      await updateOption.mutateAsync({
        key: 'console_setting.announcements',
        value: JSON.stringify(announcements),
      })
      setHasChanges(false)
      toast.success(t('systemSettings.status.announcementsSavedSuccessfully'))
    } catch {
      toast.error(t('systemSettings.errors.failedToSaveAnnouncements'))
    }
  }

  const toggleSelectAll = (checked: boolean) => {
    setSelectedIds(checked ? announcements.map((item) => item.id) : [])
  }

  const toggleSelectOne = (id: number, checked: boolean) => {
    setSelectedIds((prev) =>
      checked ? [...prev, id] : prev.filter((item) => item !== id)
    )
  }

  const sortedAnnouncements = useMemo(() => {
    return [...announcements].sort((a, b) => {
      return (
        new Date(b.publishDate).getTime() - new Date(a.publishDate).getTime()
      )
    })
  }, [announcements])

  const getRelativeTime = (date: string) => {
    const now = new Date()
    const past = new Date(date)
    const diffMs = now.getTime() - past.getTime()
    const diffMins = Math.floor(diffMs / 60000)
    const diffHours = Math.floor(diffMins / 60)
    const diffDays = Math.floor(diffHours / 24)

    if (diffMins < 60) return `${diffMins}m ago`
    if (diffHours < 24) return `${diffHours}h ago`
    return `${diffDays}d ago`
  }

  return (
    <SettingsSection title={t('dashboard.fields.announcements')}>
      <div className='space-y-4'>
        <div className='flex flex-wrap items-center justify-between gap-2'>
          <div className='flex flex-wrap items-center gap-2'>
            <Button onClick={handleAdd} size='sm'>
              <Plus className='mr-2 h-4 w-4' />
              {t('systemSettings.actions.addAnnouncement')}
            </Button>
            <Button
              onClick={handleBatchDelete}
              size='sm'
              variant='destructive'
              disabled={selectedIds.length === 0}
            >
              <Trash2 className='mr-2 h-4 w-4' />
              {t('systemSettings.actions.deleteCount', {
                count: selectedIds.length,
              })}
            </Button>
            <Button
              onClick={handleSaveAll}
              size='sm'
              variant='secondary'
              disabled={!hasChanges || updateOption.isPending}
            >
              <Save className='mr-2 h-4 w-4' />
              {updateOption.isPending
                ? t('channels.tips.saving')
                : t('profile.actions.saveSettings')}
            </Button>
          </div>
        </div>

        <div className='rounded-md border'>
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead className='w-12'>
                  <Checkbox
                    checked={
                      selectedIds.length === announcements.length &&
                      announcements.length > 0
                    }
                    onCheckedChange={toggleSelectAll}
                  />
                </TableHead>
                <TableHead>{t('dashboard.fields.content')}</TableHead>
                <TableHead>{t('systemSettings.fields.publishDate')}</TableHead>
                <TableHead>{t('channels.fields.type')}</TableHead>
                <TableHead>{t('systemSettings.fields.extra')}</TableHead>
                <TableHead className='w-32'>
                  {t('channels.fields.actions')}
                </TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {sortedAnnouncements.length === 0 ? (
                <TableRow>
                  <TableCell colSpan={6} className='h-24 text-center'>
                    {t(
                      'common.tips.noAnnouncementsYetClickAddAnnouncementToCreateOne'
                    )}
                  </TableCell>
                </TableRow>
              ) : (
                sortedAnnouncements.map((announcement) => (
                  <TableRow key={announcement.id}>
                    <TableCell>
                      <Checkbox
                        checked={selectedIds.includes(announcement.id)}
                        onCheckedChange={(checked) =>
                          toggleSelectOne(announcement.id, checked as boolean)
                        }
                      />
                    </TableCell>
                    <TableCell
                      className='max-w-xs truncate'
                      title={announcement.content}
                    >
                      {announcement.content}
                    </TableCell>
                    <TableCell>
                      <div className='flex flex-col gap-1'>
                        <span className='text-sm font-medium'>
                          {getRelativeTime(announcement.publishDate)}
                        </span>
                        <span className='text-muted-foreground text-xs'>
                          {dayjs(announcement.publishDate).format(
                            'YYYY-MM-DD HH:mm:ss'
                          )}
                        </span>
                      </div>
                    </TableCell>
                    <TableCell>
                      <StatusBadge
                        label={t(
                          typeOptions.find(
                            (opt) => opt.value === announcement.type
                          )?.label ?? 'common.fields.default'
                        )}
                        variant={
                          typeOptions.find(
                            (opt) => opt.value === announcement.type
                          )?.badgeVariant ?? 'neutral'
                        }
                        copyable={false}
                      />
                    </TableCell>
                    <TableCell
                      className='text-muted-foreground max-w-xs truncate'
                      title={announcement.extra}
                    >
                      {announcement.extra || '-'}
                    </TableCell>
                    <TableCell>
                      <div className='flex gap-2'>
                        <Button
                          onClick={() => handleEdit(announcement)}
                          size='sm'
                          variant='ghost'
                        >
                          <Edit className='h-4 w-4' />
                        </Button>
                        <Button
                          onClick={() => handleDelete(announcement)}
                          size='sm'
                          variant='ghost'
                        >
                          <Trash2 className='h-4 w-4' />
                        </Button>
                      </div>
                    </TableCell>
                  </TableRow>
                ))
              )}
            </TableBody>
          </Table>
        </div>
      </div>

      <Dialog open={showDialog} onOpenChange={setShowDialog}>
        <DialogContent className='max-w-2xl'>
          <DialogHeader>
            <DialogTitle>
              {editingAnnouncement
                ? t('systemSettings.actions.editAnnouncement')
                : t('systemSettings.actions.addAnnouncement')}
            </DialogTitle>
            <DialogDescription>
              {t(
                'systemSettings.actions.createOrUpdateSystemAnnouncementsForTheDashboard'
              )}
            </DialogDescription>
          </DialogHeader>
          <Form {...form}>
            <form
              onSubmit={form.handleSubmit(handleSubmitForm)}
              className='space-y-4'
            >
              <FormField
                control={form.control}
                name='content'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>{t('dashboard.fields.content')}</FormLabel>
                    <FormControl>
                      <Textarea
                        placeholder={t(
                          'systemSettings.placeholders.enterAnnouncementContentSupportsMarkdownHtml'
                        )}
                        rows={4}
                        {...field}
                      />
                    </FormControl>
                    <FormDescription>
                      {t(
                        'systemSettings.tips.maximum500CharactersSupportsMarkdownAndHtml'
                      )}
                    </FormDescription>
                    <FormMessage />
                  </FormItem>
                )}
              />
              <FormField
                control={form.control}
                name='publishDate'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>
                      {t('systemSettings.fields.publishDate')}
                    </FormLabel>
                    <FormControl>
                      <DateTimePicker
                        value={field.value ? new Date(field.value) : undefined}
                        onChange={(date) =>
                          field.onChange(date ? date.toISOString() : '')
                        }
                        placeholder={t(
                          'systemSettings.placeholders.selectPublishDate'
                        )}
                      />
                    </FormControl>
                    <FormDescription>
                      {t(
                        'systemSettings.tips.dateAndTimeWhenThisAnnouncementShouldBeDisplayed'
                      )}
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
                    <Select
                      items={[
                        ...typeOptions.map((option) => ({
                          value: option.value,
                          label: (
                            <div className='flex items-center gap-2'>
                              <div
                                className={`h-3 w-3 rounded-full ${option.color}`}
                              />
                              {t(option.label)}
                            </div>
                          ),
                        })),
                      ]}
                      onValueChange={field.onChange}
                      value={field.value}
                    >
                      <FormControl>
                        <SelectTrigger>
                          <SelectValue
                            placeholder={t(
                              'systemSettings.placeholders.selectAnnouncementType'
                            )}
                          />
                        </SelectTrigger>
                      </FormControl>
                      <SelectContent alignItemWithTrigger={false}>
                        <SelectGroup>
                          {typeOptions.map((option) => (
                            <SelectItem key={option.value} value={option.value}>
                              <div className='flex items-center gap-2'>
                                <div
                                  className={`h-3 w-3 rounded-full ${option.color}`}
                                />
                                {t(option.label)}
                              </div>
                            </SelectItem>
                          ))}
                        </SelectGroup>
                      </SelectContent>
                    </Select>
                    <FormMessage />
                  </FormItem>
                )}
              />
              <FormField
                control={form.control}
                name='extra'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>
                      {t('systemSettings.fields.extraNotesOptional')}
                    </FormLabel>
                    <FormControl>
                      <Input
                        placeholder={t(
                          'dashboard.titles.additionalInformation'
                        )}
                        {...field}
                      />
                    </FormControl>
                    <FormDescription>
                      {t(
                        'systemSettings.tips.optionalSupplementaryInformationMax100Characters'
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
                  onClick={() => setShowDialog(false)}
                >
                  {t('common.actions.cancel')}
                </Button>
                <Button type='submit'>
                  {editingAnnouncement
                    ? t('channels.fields.update')
                    : t('channels.actions.add')}
                </Button>
              </DialogFooter>
            </form>
          </Form>
        </DialogContent>
      </Dialog>

      <AlertDialog open={showDeleteDialog} onOpenChange={setShowDeleteDialog}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>{t('keys.tips.sure')}</AlertDialogTitle>
            <AlertDialogDescription>
              {deleteTarget === 'single'
                ? t(
                    'systemSettings.status.announcementWillBeRemovedFromTheList'
                  )
                : t(
                    'systemSettings.status.countAnnouncementsWillBeRemovedFromTheList',
                    {
                      count: selectedIds.length,
                    }
                  )}
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel>{t('common.actions.cancel')}</AlertDialogCancel>
            <AlertDialogAction onClick={confirmDelete}>
              {t('common.actions.delete')}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </SettingsSection>
  )
}
