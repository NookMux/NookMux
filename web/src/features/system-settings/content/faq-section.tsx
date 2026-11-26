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
import { Textarea } from '@/components/ui/textarea'
import { SettingsControl, SettingsRow } from '../components/settings-group'
import { SettingsListPanel } from '../components/settings-list-panel'
import { SettingsPageFormActions } from '../components/settings-page-context'
import { SettingsSection } from '../components/settings-section'
import { useUpdateOption } from '../hooks/use-update-option'

type FAQ = {
  id: number
  question: string
  answer: string
}

type FAQSectionProps = {
  data: string
}

const faqSchema = z.object({
  question: z
    .string()
    .min(1, 'systemSettings.errors.questionIsRequired')
    .max(200, 'systemSettings.errors.questionMax200Chars'),
  answer: z
    .string()
    .min(1, 'systemSettings.errors.answerIsRequired')
    .max(1000, 'systemSettings.errors.answerMax1000Chars'),
})

type FAQFormValues = z.infer<typeof faqSchema>

export function FAQSection({ data }: FAQSectionProps) {
  const { t } = useTranslation()
  const updateOption = useUpdateOption()
  const [faqList, setFaqList] = useState<FAQ[]>([])
  const [hasChanges, setHasChanges] = useState(false)
  const [selectedIds, setSelectedIds] = useState<number[]>([])
  const [showDialog, setShowDialog] = useState(false)
  const [showDeleteDialog, setShowDeleteDialog] = useState(false)
  const [editingFaq, setEditingFaq] = useState<FAQ | null>(null)
  const [deleteTarget, setDeleteTarget] = useState<'single' | 'batch'>('single')

  const form = useForm<FAQFormValues>({
    resolver: zodResolver(faqSchema),
    defaultValues: {
      question: '',
      answer: '',
    },
  })

  // 从已保存的值载入列表；「重置」放弃未保存的改动时也走这里。
  const loadSavedList = useCallback(() => {
    try {
      const parsed = JSON.parse(data || '[]')
      if (Array.isArray(parsed)) {
        setFaqList(
          parsed.map((item, idx) => ({
            ...item,
            id: item.id || idx + 1,
          }))
        )
      }
    } catch {
      setFaqList([])
    }
  }, [data])

  useEffect(() => {
    loadSavedList()
  }, [loadSavedList])

  const handleAdd = () => {
    setEditingFaq(null)
    form.reset({
      question: '',
      answer: '',
    })
    setShowDialog(true)
  }

  const handleEdit = (faq: FAQ) => {
    setEditingFaq(faq)
    form.reset({
      question: faq.question,
      answer: faq.answer,
    })
    setShowDialog(true)
  }

  const handleDelete = (faq: FAQ) => {
    setEditingFaq(faq)
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
    if (deleteTarget === 'single' && editingFaq) {
      setFaqList((prev) => prev.filter((item) => item.id !== editingFaq.id))
      setHasChanges(true)
      toast.success(t('common.status.faqDeletedClickSaveSettingsToApply'))
    } else if (deleteTarget === 'batch') {
      setFaqList((prev) =>
        prev.filter((item) => !selectedIds.includes(item.id))
      )
      setSelectedIds([])
      setHasChanges(true)
      toast.success(
        t('systemSettings.status.countFaqsDeletedClickSaveSettingsToApply', {
          count: selectedIds.length,
        })
      )
    }
    setShowDeleteDialog(false)
    setEditingFaq(null)
  }

  const handleSubmitForm = (values: FAQFormValues) => {
    if (editingFaq) {
      setFaqList((prev) =>
        prev.map((item) =>
          item.id === editingFaq.id ? { ...item, ...values } : item
        )
      )
      toast.success(t('common.status.faqUpdatedClickSaveSettingsToApply'))
    } else {
      const newId = Math.max(...faqList.map((item) => item.id), 0) + 1
      setFaqList((prev) => [...prev, { id: newId, ...values }])
      toast.success(t('common.tips.faqAddedClickSaveSettingsToApply'))
    }
    setHasChanges(true)
    setShowDialog(false)
  }

  const handleSaveAll = async () => {
    try {
      await updateOption.mutateAsync({
        key: 'console_setting.faq',
        value: JSON.stringify(faqList),
      })
      setHasChanges(false)
      toast.success(t('systemSettings.status.faqSavedSuccessfully'))
    } catch {
      toast.error(t('systemSettings.errors.failedToSaveFaq'))
    }
  }

  const handleDiscard = () => {
    loadSavedList()
    setSelectedIds([])
    setHasChanges(false)
  }

  const toggleSelectAll = (checked: boolean) => {
    setSelectedIds(checked ? faqList.map((item) => item.id) : [])
  }

  const toggleSelectOne = (id: number, checked: boolean) => {
    setSelectedIds((prev) =>
      checked ? [...prev, id] : prev.filter((item) => item !== id)
    )
  }

  return (
    <SettingsSection title={t('dashboard.fields.faq')}>
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
            {t('systemSettings.actions.addFaq')}
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
                  selectedIds.length === faqList.length && faqList.length > 0
                }
                onCheckedChange={toggleSelectAll}
                aria-label={t('channels.placeholders.selectAll')}
              />
            </TableHead>
            <TableHead>{t('systemSettings.fields.question')}</TableHead>
            <TableHead>{t('systemSettings.fields.answer')}</TableHead>
            <TableHead className='w-32'>
              {t('channels.fields.actions')}
            </TableHead>
          </TableRow>
        </TableHeader>
        <TableBody>
          {faqList.length === 0 ? (
            <TableRow>
              <TableCell
                colSpan={4}
                className='text-muted-foreground h-24 text-center'
              >
                {t('common.tips.noFaqEntriesYetClickAddFaqToCreate')}
              </TableCell>
            </TableRow>
          ) : (
            faqList.map((faq) => (
              <TableRow key={faq.id}>
                <TableCell>
                  <Checkbox
                    checked={selectedIds.includes(faq.id)}
                    onCheckedChange={(checked) =>
                      toggleSelectOne(faq.id, checked as boolean)
                    }
                    aria-label={t('systemSettings.actions.selectRow')}
                  />
                </TableCell>
                <TableCell
                  className='max-w-xs truncate font-medium'
                  title={faq.question}
                >
                  {faq.question}
                </TableCell>
                <TableCell
                  className='text-muted-foreground max-w-md truncate'
                  title={faq.answer}
                >
                  {faq.answer}
                </TableCell>
                <TableCell>
                  <div className='flex gap-2'>
                    <Button
                      type='button'
                      onClick={() => handleEdit(faq)}
                      size='sm'
                      variant='ghost'
                      aria-label={t('channels.actions.edit')}
                    >
                      <Edit className='h-4 w-4' />
                    </Button>
                    <Button
                      type='button'
                      onClick={() => handleDelete(faq)}
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
        <DialogContent className='max-w-2xl'>
          <DialogHeader>
            <DialogTitle>
              {editingFaq
                ? t('systemSettings.actions.editFaq')
                : t('systemSettings.actions.addFaq')}
            </DialogTitle>
            <DialogDescription>
              {t(
                'systemSettings.actions.createOrUpdateFrequentlyAskedQuestionsForUsers'
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
                name='question'
                render={({ field }) => (
                  <SettingsRow
                    label={t('systemSettings.fields.question')}
                    description={t(
                      'systemSettings.fields.maximum200Characters'
                    )}
                  >
                    <SettingsControl>
                      <FormControl>
                        <Input
                          placeholder={t(
                            'systemSettings.tips.howToResetMyQuota'
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
                name='answer'
                render={({ field }) => (
                  <SettingsRow
                    label={t('systemSettings.fields.answer')}
                    description={t(
                      'systemSettings.tips.maximum1000CharactersSupportsMarkdownAndHtml'
                    )}
                  >
                    <FormControl>
                      <Textarea
                        placeholder={t(
                          'systemSettings.actions.visitSettingsGeneralAndAdjustQuotaOptions'
                        )}
                        rows={8}
                        {...field}
                      />
                    </FormControl>
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
                  {editingFaq
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
                ? t('systemSettings.status.faqEntryWillBeRemovedFromTheList')
                : t(
                    'systemSettings.status.countFaqEntriesWillBeRemovedFromTheList',
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
