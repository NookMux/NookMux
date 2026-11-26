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
import { useCallback, useEffect, useState } from 'react'
import * as z from 'zod'
import { useForm } from 'react-hook-form'
import { zodResolver } from '@hookform/resolvers/zod'
import { Plus, Edit, Trash2 } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
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
import { Form, FormControl, FormField } from '@/components/ui/form'
import { Input } from '@/components/ui/input'
import {
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'
import { SettingsControl, SettingsRow } from '../components/settings-group'
import { SettingsListPanel } from '../components/settings-list-panel'
import { SettingsPageFormActions } from '../components/settings-page-context'
import { SettingsSection } from '../components/settings-section'
import { useUpdateOption } from '../hooks/use-update-option'

type UptimeKumaGroup = {
  id: number
  categoryName: string
  url: string
  slug: string
}

type UptimeKumaSectionProps = {
  data: string
}

const createUptimeKumaSchema = (t: (key: string) => string) =>
  z.object({
    categoryName: z
      .string()
      .min(1, { error: t('systemSettings.errors.categoryNameIsRequired') })
      .max(50, {
        error: t(
          'systemSettings.errors.categoryNameMustBeLessThan50Characters'
        ),
      }),
    url: z.string().url({ error: t('systemSettings.errors.mustBeAValidUrl') }),
    slug: z
      .string()
      .min(1, { error: t('systemSettings.errors.slugIsRequired') })
      .max(100, {
        error: t('systemSettings.errors.slugMustBeLessThan100Characters'),
      })
      .regex(/^[a-zA-Z0-9_-]+$/, {
        error: t(
          'systemSettings.tips.slugCanOnlyContainLettersNumbersHyphensAndUnderscores'
        ),
      }),
  })

type UptimeKumaFormValues = z.infer<ReturnType<typeof createUptimeKumaSchema>>

export function UptimeKumaSection({ data }: UptimeKumaSectionProps) {
  const { t } = useTranslation()
  const updateOption = useUpdateOption()
  const uptimeKumaSchema = createUptimeKumaSchema(t)
  const [groups, setGroups] = useState<UptimeKumaGroup[]>([])
  const [hasChanges, setHasChanges] = useState(false)
  const [selectedIds, setSelectedIds] = useState<number[]>([])
  const [showDialog, setShowDialog] = useState(false)
  const [showDeleteDialog, setShowDeleteDialog] = useState(false)
  const [editingGroup, setEditingGroup] = useState<UptimeKumaGroup | null>(null)
  const [deleteTarget, setDeleteTarget] = useState<'single' | 'batch'>('single')

  const form = useForm<UptimeKumaFormValues>({
    resolver: zodResolver(uptimeKumaSchema),
    defaultValues: {
      categoryName: '',
      url: '',
      slug: '',
    },
  })

  // 从已保存的值载入列表；「重置」放弃未保存的改动时也走这里。
  const loadSavedList = useCallback(() => {
    try {
      const parsed = JSON.parse(data || '[]')
      if (Array.isArray(parsed)) {
        setGroups(
          parsed.map((item, idx) => ({
            ...item,
            id: item.id || idx + 1,
          }))
        )
      }
    } catch {
      setGroups([])
    }
  }, [data])

  useEffect(() => {
    loadSavedList()
  }, [loadSavedList])

  const handleAdd = () => {
    setEditingGroup(null)
    form.reset({
      categoryName: '',
      url: '',
      slug: '',
    })
    setShowDialog(true)
  }

  const handleEdit = (group: UptimeKumaGroup) => {
    setEditingGroup(group)
    form.reset({
      categoryName: group.categoryName,
      url: group.url,
      slug: group.slug,
    })
    setShowDialog(true)
  }

  const handleDelete = (group: UptimeKumaGroup) => {
    setEditingGroup(group)
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
    if (deleteTarget === 'single' && editingGroup) {
      setGroups((prev) => prev.filter((item) => item.id !== editingGroup.id))
      setHasChanges(true)
      toast.success(t('common.status.groupDeletedClickSaveSettingsToApply'))
    } else if (deleteTarget === 'batch') {
      setGroups((prev) => prev.filter((item) => !selectedIds.includes(item.id)))
      setSelectedIds([])
      setHasChanges(true)
      toast.success(
        t('systemSettings.status.countGroupsDeletedClickSaveSettingsToApply', {
          count: selectedIds.length,
        })
      )
    }
    setShowDeleteDialog(false)
    setEditingGroup(null)
  }

  const handleSubmitForm = (values: UptimeKumaFormValues) => {
    if (editingGroup) {
      setGroups((prev) =>
        prev.map((item) =>
          item.id === editingGroup.id ? { ...item, ...values } : item
        )
      )
      toast.success(t('common.status.groupUpdatedClickSaveSettingsToApply'))
    } else {
      const newId = Math.max(...groups.map((item) => item.id), 0) + 1
      setGroups((prev) => [...prev, { id: newId, ...values }])
      toast.success(t('common.tips.groupAddedClickSaveSettingsToApply'))
    }
    setHasChanges(true)
    setShowDialog(false)
  }

  const handleSaveAll = async () => {
    try {
      await updateOption.mutateAsync({
        key: 'console_setting.uptime_kuma_groups',
        value: JSON.stringify(groups),
      })
      setHasChanges(false)
      toast.success(
        t('systemSettings.status.uptimeKumaGroupsSavedSuccessfully')
      )
    } catch {
      toast.error(t('systemSettings.errors.failedToSaveUptimeKumaGroups'))
    }
  }

  const handleDiscard = () => {
    loadSavedList()
    setSelectedIds([])
    setHasChanges(false)
  }

  const toggleSelectAll = (checked: boolean) => {
    setSelectedIds(checked ? groups.map((item) => item.id) : [])
  }

  const toggleSelectOne = (id: number, checked: boolean) => {
    setSelectedIds((prev) =>
      checked ? [...prev, id] : prev.filter((item) => item !== id)
    )
  }

  return (
    <SettingsSection title={t('systemSettings.fields.uptimeKuma')}>
      <SettingsPageFormActions
        onSave={handleSaveAll}
        onReset={handleDiscard}
        isSaving={updateOption.isPending}
        isDirty={hasChanges}
        saveLabel='profile.actions.saveSettings'
      />
      <SettingsListPanel
        actions={
          <Button type='button' variant='outline' size='sm' onClick={handleAdd}>
            <Plus data-icon='inline-start' />
            {t('systemSettings.actions.addGroup')}
          </Button>
        }
        selectedCount={selectedIds.length}
        bulkActions={
          <Button
            type='button'
            variant='destructive'
            size='sm'
            onClick={handleBatchDelete}
          >
            <Trash2 data-icon='inline-start' />
            {t('systemSettings.actions.deleteSelected')}
          </Button>
        }
      >
        <TableHeader>
          <TableRow>
            <TableHead className='w-12'>
              <Checkbox
                checked={
                  selectedIds.length === groups.length && groups.length > 0
                }
                onCheckedChange={toggleSelectAll}
                aria-label={t('channels.placeholders.selectAll')}
              />
            </TableHead>
            <TableHead>{t('systemSettings.fields.categoryName')}</TableHead>
            <TableHead>{t('systemSettings.fields.uptimeKumaUrl')}</TableHead>
            <TableHead>{t('systemSettings.fields.statusPageSlug')}</TableHead>
            <TableHead className='w-32'>
              {t('channels.fields.actions')}
            </TableHead>
          </TableRow>
        </TableHeader>
        <TableBody>
          {groups.length === 0 ? (
            <TableRow>
              <TableCell
                colSpan={5}
                className='text-muted-foreground h-24 text-center'
              >
                {t('common.tips.noUptimeKumaGroupsYetClickAddGroupTo')}
              </TableCell>
            </TableRow>
          ) : (
            groups.map((group) => (
              <TableRow key={group.id}>
                <TableCell>
                  <Checkbox
                    checked={selectedIds.includes(group.id)}
                    onCheckedChange={(checked) =>
                      toggleSelectOne(group.id, checked as boolean)
                    }
                    aria-label={t('systemSettings.actions.selectRow')}
                  />
                </TableCell>
                <TableCell className='font-medium'>
                  {group.categoryName}
                </TableCell>
                <TableCell
                  className='text-primary max-w-xs truncate font-mono text-sm'
                  title={group.url}
                >
                  {group.url}
                </TableCell>
                <TableCell className='text-muted-foreground font-mono text-sm'>
                  {group.slug}
                </TableCell>
                <TableCell>
                  <div className='flex gap-2'>
                    <Button
                      type='button'
                      onClick={() => handleEdit(group)}
                      size='sm'
                      variant='ghost'
                      aria-label={t('channels.actions.edit')}
                    >
                      <Edit className='h-4 w-4' />
                    </Button>
                    <Button
                      type='button'
                      onClick={() => handleDelete(group)}
                      size='sm'
                      variant='ghost'
                      aria-label={t('common.actions.delete')}
                    >
                      <Trash2 className='h-4 w-4' />
                    </Button>
                  </div>
                </TableCell>
              </TableRow>
            ))
          )}
        </TableBody>
      </SettingsListPanel>

      <Dialog open={showDialog} onOpenChange={setShowDialog}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>
              {editingGroup
                ? t('systemSettings.actions.editUptimeKumaGroup')
                : t('systemSettings.actions.addUptimeKumaGroup')}
            </DialogTitle>
            <DialogDescription>
              {t(
                'systemSettings.tips.configureMonitoringStatusPageGroupsForTheDashboard'
              )}
            </DialogDescription>
          </DialogHeader>
          <Form {...form}>
            <form
              onSubmit={form.handleSubmit(handleSubmitForm)}
              className='@container space-y-4'
            >
              <FormField
                control={form.control}
                name='categoryName'
                render={({ field }) => (
                  <SettingsRow
                    label={t('systemSettings.fields.categoryName')}
                    description={t(
                      'systemSettings.tips.displayNameForThisMonitoringGroupMax50Characters'
                    )}
                  >
                    <SettingsControl>
                      <FormControl>
                        <Input
                          placeholder={t(
                            'systemSettings.placeholders.eGCoreApisOpenAiClaude'
                          )}
                          {...field}
                        />
                      </FormControl>
                    </SettingsControl>
                  </SettingsRow>
                )}
              />
              <FormField
                control={form.control}
                name='url'
                render={({ field }) => (
                  <SettingsRow
                    label={t('systemSettings.fields.uptimeKumaUrl')}
                    description={t(
                      'systemSettings.tips.baseUrlOfYourUptimeKumaInstance'
                    )}
                  >
                    <SettingsControl>
                      <FormControl>
                        <Input
                          placeholder={t(
                            'systemSettings.placeholders.urlStatusExampleCom'
                          )}
                          {...field}
                        />
                      </FormControl>
                    </SettingsControl>
                  </SettingsRow>
                )}
              />
              <FormField
                control={form.control}
                name='slug'
                render={({ field }) => (
                  <SettingsRow
                    label={t('systemSettings.fields.statusPageSlug')}
                    description={
                      <>
                        {t('systemSettings.tips.slugIsAppendedToTheUrl')}{' '}
                        {'{url}'}
                        {t('systemSettings.placeholders.status')}
                        {'{slug}'}
                      </>
                    }
                  >
                    <SettingsControl>
                      <FormControl>
                        <Input
                          placeholder={t('systemSettings.fields.myStatus')}
                          {...field}
                        />
                      </FormControl>
                    </SettingsControl>
                  </SettingsRow>
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
                  {editingGroup
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
                ? t('systemSettings.tips.confirmRemoveSingleUptimeKumaGroup')
                : t('systemSettings.tips.confirmRemoveCountUptimeKumaGroups', {
                    count: selectedIds.length,
                  })}
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
