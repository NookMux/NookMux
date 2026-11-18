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
import { useEffect, useState } from 'react'
import * as z from 'zod'
import { useForm } from 'react-hook-form'
import { zodResolver } from '@hookform/resolvers/zod'
import { Plus, Edit, Trash2, Save } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { getBgColorClass } from '@/lib/colors'
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
import { StatusBadge } from '@/components/status-badge'
import { SettingsSection } from '../components/settings-section'
import { useUpdateOption } from '../hooks/use-update-option'

type ApiInfo = {
  id: number
  url: string
  route: string
  description: string
  color: string
}

type ApiInfoSectionProps = {
  data: string
}

const createApiInfoSchema = (t: (key: string) => string) =>
  z.object({
    url: z.string().url(t('systemSettings.errors.mustBeAValidUrl')),
    route: z.string().min(1, t('systemSettings.errors.routeIsRequired')),
    description: z
      .string()
      .min(1, t('systemSettings.errors.descriptionIsRequired')),
    color: z.string().min(1, t('systemSettings.errors.colorIsRequired')),
  })

type ApiInfoFormValues = z.infer<ReturnType<typeof createApiInfoSchema>>

const colorOptions = [
  { value: 'blue', label: 'systemSettings.fields.colorBlue' },
  { value: 'green', label: 'systemSettings.fields.colorGreen' },
  { value: 'cyan', label: 'systemSettings.fields.colorCyan' },
  { value: 'purple', label: 'systemSettings.fields.colorPurple' },
  { value: 'pink', label: 'systemSettings.fields.colorPink' },
  { value: 'red', label: 'systemSettings.fields.colorRed' },
  { value: 'orange', label: 'systemSettings.fields.colorOrange' },
  { value: 'amber', label: 'systemSettings.fields.colorAmber' },
  { value: 'yellow', label: 'systemSettings.fields.colorYellow' },
  { value: 'lime', label: 'systemSettings.fields.colorLime' },
  { value: 'teal', label: 'systemSettings.fields.colorTeal' },
  { value: 'indigo', label: 'systemSettings.fields.colorIndigo' },
  { value: 'violet', label: 'systemSettings.fields.colorViolet' },
  { value: 'slate', label: 'systemSettings.fields.colorSlate' },
]

export function ApiInfoSection({ data }: ApiInfoSectionProps) {
  const { t } = useTranslation()
  const updateOption = useUpdateOption()
  const apiInfoSchema = createApiInfoSchema(t)
  const [apiInfoList, setApiInfoList] = useState<ApiInfo[]>([])
  const [hasChanges, setHasChanges] = useState(false)
  const [selectedIds, setSelectedIds] = useState<number[]>([])
  const [showDialog, setShowDialog] = useState(false)
  const [showDeleteDialog, setShowDeleteDialog] = useState(false)
  const [editingApiInfo, setEditingApiInfo] = useState<ApiInfo | null>(null)
  const [deleteTarget, setDeleteTarget] = useState<'single' | 'batch'>('single')

  const form = useForm<ApiInfoFormValues>({
    resolver: zodResolver(apiInfoSchema),
    defaultValues: {
      url: '',
      route: '',
      description: '',
      color: 'blue',
    },
  })

  useEffect(() => {
    try {
      const parsed = JSON.parse(data || '[]')
      if (Array.isArray(parsed)) {
        setApiInfoList(
          parsed.map((item, idx) => ({
            ...item,
            id: item.id || idx + 1,
          }))
        )
      }
    } catch {
      setApiInfoList([])
    }
  }, [data])

  const handleAdd = () => {
    setEditingApiInfo(null)
    form.reset({
      url: '',
      route: '',
      description: '',
      color: 'blue',
    })
    setShowDialog(true)
  }

  const handleEdit = (apiInfo: ApiInfo) => {
    setEditingApiInfo(apiInfo)
    form.reset({
      url: apiInfo.url,
      route: apiInfo.route,
      description: apiInfo.description,
      color: apiInfo.color,
    })
    setShowDialog(true)
  }

  const handleDelete = (apiInfo: ApiInfo) => {
    setEditingApiInfo(apiInfo)
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
    if (deleteTarget === 'single' && editingApiInfo) {
      setApiInfoList((prev) =>
        prev.filter((item) => item.id !== editingApiInfo.id)
      )
      setHasChanges(true)
      toast.success(t('common.status.apiInfoDeletedClickSaveSettingsToApply'))
    } else if (deleteTarget === 'batch') {
      setApiInfoList((prev) =>
        prev.filter((item) => !selectedIds.includes(item.id))
      )
      setSelectedIds([])
      setHasChanges(true)
      toast.success(
        t(
          'systemSettings.status.countApiEntriesDeletedClickSaveSettingsToApply',
          {
            count: selectedIds.length,
          }
        )
      )
    }
    setShowDeleteDialog(false)
    setEditingApiInfo(null)
  }

  const handleSubmitForm = (values: ApiInfoFormValues) => {
    if (editingApiInfo) {
      setApiInfoList((prev) =>
        prev.map((item) =>
          item.id === editingApiInfo.id ? { ...item, ...values } : item
        )
      )
      toast.success(t('common.status.apiInfoUpdatedClickSaveSettingsToApply'))
    } else {
      const newId = Math.max(...apiInfoList.map((item) => item.id), 0) + 1
      setApiInfoList((prev) => [...prev, { id: newId, ...values }])
      toast.success(t('common.tips.apiInfoAddedClickSaveSettingsToApply'))
    }
    setHasChanges(true)
    setShowDialog(false)
  }

  const handleSaveAll = async () => {
    try {
      const result = await updateOption.mutateAsync({
        key: 'console_setting.api_info',
        value: JSON.stringify(apiInfoList),
      })
      if (result.success) {
        setHasChanges(false)
      }
    } catch {
      toast.error(t('systemSettings.errors.failedToSaveApiInfo'))
    }
  }

  const toggleSelectAll = (checked: boolean) => {
    setSelectedIds(checked ? apiInfoList.map((item) => item.id) : [])
  }

  const toggleSelectOne = (id: number, checked: boolean) => {
    setSelectedIds((prev) =>
      checked ? [...prev, id] : prev.filter((item) => item !== id)
    )
  }

  const getColorClass = (color: string) => getBgColorClass(color)

  return (
    <SettingsSection title={t('systemSettings.fields.apiAddresses')}>
      <div className='space-y-4'>
        <div className='flex flex-wrap items-center justify-between gap-2'>
          <div className='flex flex-wrap items-center gap-2'>
            <Button onClick={handleAdd} size='sm'>
              <Plus className='mr-2 h-4 w-4' />
              {t('systemSettings.actions.addApi')}
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
                      selectedIds.length === apiInfoList.length &&
                      apiInfoList.length > 0
                    }
                    onCheckedChange={toggleSelectAll}
                  />
                </TableHead>
                <TableHead>{t('systemSettings.fields.url')}</TableHead>
                <TableHead>{t('systemSettings.fields.route')}</TableHead>
                <TableHead>{t('auditLogs.tips.description')}</TableHead>
                <TableHead>{t('systemSettings.fields.color')}</TableHead>
                <TableHead className='w-32'>
                  {t('channels.fields.actions')}
                </TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {apiInfoList.length === 0 ? (
                <TableRow>
                  <TableCell colSpan={6} className='h-24 text-center'>
                    {t('common.tips.noApiDomainsYetClickAddApiToCreate')}
                  </TableCell>
                </TableRow>
              ) : (
                apiInfoList.map((apiInfo) => (
                  <TableRow key={apiInfo.id}>
                    <TableCell>
                      <Checkbox
                        checked={selectedIds.includes(apiInfo.id)}
                        onCheckedChange={(checked) =>
                          toggleSelectOne(apiInfo.id, checked as boolean)
                        }
                      />
                    </TableCell>
                    <TableCell
                      className='max-w-xs truncate font-mono text-sm'
                      title={apiInfo.url}
                    >
                      <StatusBadge
                        label={apiInfo.url}
                        variant='neutral'
                        copyable={false}
                      />
                    </TableCell>
                    <TableCell>
                      <StatusBadge
                        label={apiInfo.route}
                        variant='neutral'
                        copyable={false}
                      />
                    </TableCell>
                    <TableCell
                      className='max-w-xs truncate'
                      title={apiInfo.description}
                    >
                      {apiInfo.description}
                    </TableCell>
                    <TableCell>
                      <div className='flex items-center gap-2'>
                        <div
                          className={`h-4 w-4 rounded-full ${getColorClass(apiInfo.color)}`}
                        />
                        <span className='text-sm capitalize'>
                          {apiInfo.color}
                        </span>
                      </div>
                    </TableCell>
                    <TableCell>
                      <div className='flex gap-2'>
                        <Button
                          onClick={() => handleEdit(apiInfo)}
                          size='sm'
                          variant='ghost'
                        >
                          <Edit className='h-4 w-4' />
                        </Button>
                        <Button
                          onClick={() => handleDelete(apiInfo)}
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
        <DialogContent>
          <DialogHeader>
            <DialogTitle>
              {editingApiInfo
                ? t('systemSettings.actions.editApiShortcut')
                : t('systemSettings.actions.addApiShortcut')}
            </DialogTitle>
            <DialogDescription>
              {t(
                'systemSettings.tips.configureApiDocumentationLinksForTheDashboard'
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
                name='url'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>{t('systemSettings.fields.apiUrl')}</FormLabel>
                    <FormControl>
                      <Input
                        placeholder={t(
                          'systemSettings.placeholders.urlApiExampleCom'
                        )}
                        {...field}
                      />
                    </FormControl>
                    <FormMessage />
                  </FormItem>
                )}
              />
              <FormField
                control={form.control}
                name='route'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>
                      {t('systemSettings.tips.routeDescription')}
                    </FormLabel>
                    <FormControl>
                      <Input
                        placeholder={t('systemSettings.placeholders.eGCn2Gia')}
                        {...field}
                      />
                    </FormControl>
                    <FormMessage />
                  </FormItem>
                )}
              />
              <FormField
                control={form.control}
                name='description'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>{t('auditLogs.tips.description')}</FormLabel>
                    <FormControl>
                      <Input
                        placeholder={t(
                          'systemSettings.placeholders.eGRecommendedForChinaMainlandUsers'
                        )}
                        {...field}
                      />
                    </FormControl>
                    <FormMessage />
                  </FormItem>
                )}
              />
              <FormField
                control={form.control}
                name='color'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>
                      {t('systemSettings.fields.badgeColor')}
                    </FormLabel>
                    <Select
                      items={[
                        ...colorOptions.map((option) => ({
                          value: option.value,
                          label: (
                            <div className='flex items-center gap-2'>
                              <div
                                className={`h-4 w-4 rounded-full ${getBgColorClass(option.value)}`}
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
                              'systemSettings.placeholders.selectAColor'
                            )}
                          />
                        </SelectTrigger>
                      </FormControl>
                      <SelectContent alignItemWithTrigger={false}>
                        <SelectGroup>
                          {colorOptions.map((option) => (
                            <SelectItem key={option.value} value={option.value}>
                              <div className='flex items-center gap-2'>
                                <div
                                  className={`h-4 w-4 rounded-full ${getBgColorClass(option.value)}`}
                                />
                                {t(option.label)}
                              </div>
                            </SelectItem>
                          ))}
                        </SelectGroup>
                      </SelectContent>
                    </Select>
                    <FormDescription>
                      {t(
                        'systemSettings.tips.visualIndicatorColorForTheApiCard'
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
                  {editingApiInfo
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
                ? t('systemSettings.tips.confirmRemoveSingleApiShortcut')
                : t('systemSettings.tips.confirmRemoveCountApiShortcuts', {
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
