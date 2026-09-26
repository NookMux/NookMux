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
import { useState, useEffect, useMemo } from 'react'
import { useQueryClient } from '@tanstack/react-query'
import { Loader2, Search, Info, ChevronDown } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { Button } from '@/components/ui/button'
import { Checkbox } from '@/components/ui/checkbox'
import {
  Collapsible,
  CollapsibleContent,
  CollapsibleTrigger,
} from '@/components/ui/collapsible'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs'
import {
  Tooltip,
  TooltipContent,
  TooltipTrigger,
} from '@/components/ui/tooltip'
import { fetchUpstreamModels, updateChannel } from '../../api'
import {
  channelsQueryKeys,
  categorizeModelsWithRedirect,
  normalizeModelName,
  parseModelsString,
} from '../../lib'
import { useChannels } from '../channels-provider'

function normalizeModelNameList(models: readonly unknown[]): string[] {
  return Array.from(
    new Set(models.map((m) => normalizeModelName(m)).filter(Boolean))
  )
}

type FetchModelsDialogProps = {
  open: boolean
  onOpenChange: (open: boolean) => void
  onModelsSelected?: (models: string[]) => void
  redirectModels?: string[]
  redirectSourceModels?: string[]
  customFetcher?: () => Promise<string[]>
  existingModelsOverride?: string[]
  channelName?: string | null
}

type ModelsTabValue = 'new' | 'existing' | 'removed'

export function FetchModelsDialog({
  open,
  onOpenChange,
  onModelsSelected,
  redirectModels = [],
  redirectSourceModels = [],
  customFetcher,
  existingModelsOverride,
  channelName,
}: FetchModelsDialogProps) {
  const { t } = useTranslation()
  const { currentRow } = useChannels()
  const activeChannel = customFetcher ? null : currentRow
  const queryClient = useQueryClient()
  const [isFetching, setIsFetching] = useState(false)
  const [isSaving, setIsSaving] = useState(false)
  const [fetchedModels, setFetchedModels] = useState<string[]>([])
  const [selectedModels, setSelectedModels] = useState<string[]>([])
  const [searchKeyword, setSearchKeyword] = useState('')
  const [activeTab, setActiveTab] = useState<ModelsTabValue>('new')

  // Parse existing models
  const existingModels = useMemo(
    () =>
      existingModelsOverride ?? parseModelsString(activeChannel?.models || ''),
    [existingModelsOverride, activeChannel?.models]
  )

  // Categorize models with redirect models
  const modelCategories = useMemo(
    () => categorizeModelsWithRedirect(existingModels, redirectModels),
    [existingModels, redirectModels]
  )

  const { classificationSet, redirectOnlySet } = modelCategories

  const fetchedModelSet = useMemo(
    () => new Set(normalizeModelNameList(fetchedModels)),
    [fetchedModels]
  )

  // Source keys in model_mapping are aliases, not real upstream IDs, so we
  // must skip them when computing "removed upstream" entries to avoid false
  // positives.
  const redirectSourceKeysSet = useMemo(
    () => new Set(normalizeModelNameList(redirectSourceModels)),
    [redirectSourceModels]
  )

  const removedModels = useMemo(() => {
    const kw = searchKeyword.toLowerCase().trim()
    return normalizeModelNameList(selectedModels).filter((model) => {
      if (fetchedModelSet.has(model)) return false
      if (redirectSourceKeysSet.has(model)) return false
      if (!kw) return true
      return typeof model === 'string' && model.toLowerCase().includes(kw)
    })
  }, [fetchedModelSet, redirectSourceKeysSet, searchKeyword, selectedModels])

  useEffect(() => {
    if (open && (activeChannel || customFetcher)) {
      handleFetchModels()
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [open, activeChannel?.id, customFetcher])

  // 关键词过滤会同时改变 new/existing/removed 三个页签的数量；Tabs 必须保持受控，
  // 否则 Base UI 在页签瞬时全部禁用/消失时会把非受控值提交为 null 且不再自愈，
  // 表现为勾选清单永久空白、只能重新获取。这里在每次拉取成功后显式重置目标页签。
  const resetTabForFetchedModels = (normalized: string[]) => {
    const fetchedSet = new Set(normalized)
    const hasNew = normalized.some((m) => !classificationSet.has(m))
    if (hasNew) {
      setActiveTab('new')
      return
    }
    const hasRemoved = existingModels.some(
      (m) => !fetchedSet.has(normalizeModelName(m))
    )
    setActiveTab(hasRemoved ? 'removed' : 'existing')
  }

  const handleFetchModels = async () => {
    if (!activeChannel && !customFetcher) return

    setIsFetching(true)
    try {
      if (customFetcher) {
        const list = (await customFetcher()) as unknown[]
        const normalized = normalizeModelNameList(list)
        setFetchedModels(normalized)
        setSelectedModels(existingModels)
        resetTabForFetchedModels(normalized)
        toast.success(
          t('channels.titles.fetchedCountModels', { count: normalized.length })
        )
      } else {
        const response = await fetchUpstreamModels(activeChannel!.id)
        if (response.success) {
          const list = Array.isArray(response.data) ? response.data : []
          const normalized = normalizeModelNameList(list)
          setFetchedModels(normalized)
          setSelectedModels(existingModels)
          resetTabForFetchedModels(normalized)
          toast.success(
            t('channels.titles.fetchedCountModels', {
              count: normalized.length,
            })
          )
        } else {
          toast.error(
            response.message || t('channels.errors.failedToFetchModels')
          )
          setFetchedModels([])
        }
      }
    } catch (error: unknown) {
      toast.error(
        error instanceof Error
          ? error.message
          : t('channels.errors.failedToFetchModels')
      )
      setFetchedModels([])
    } finally {
      setIsFetching(false)
    }
  }

  const handleSave = async () => {
    // If onModelsSelected callback is provided, use it (form filling mode)
    if (onModelsSelected) {
      onModelsSelected(selectedModels)
      toast.success(t('channels.titles.modelsFilledToForm'))
      onOpenChange(false)
      return
    }

    // Otherwise, directly save to API (standalone mode)
    if (!activeChannel) return
    setIsSaving(true)
    try {
      const modelsString = selectedModels.join(',')
      const response = await updateChannel(activeChannel.id, {
        models: modelsString,
      })
      if (response.success) {
        toast.success(t('channels.status.modelsUpdatedSuccessfully'))
        queryClient.invalidateQueries({ queryKey: channelsQueryKeys.lists() })
        onOpenChange(false)
      } else {
        toast.error(
          response.message || t('channels.errors.failedToUpdateModels')
        )
      }
    } catch (error: unknown) {
      toast.error(
        error instanceof Error
          ? error.message
          : t('channels.errors.failedToUpdateModels')
      )
    } finally {
      setIsSaving(false)
    }
  }

  const handleClose = () => {
    setFetchedModels([])
    setSelectedModels([])
    setSearchKeyword('')
    setActiveTab('new')
    onOpenChange(false)
  }

  // Categorize models by common prefixes
  const categorizeModels = (models: string[]) => {
    const categories: Record<string, string[]> = {}

    models.forEach((model) => {
      if (typeof model !== 'string') return
      let category = t('common.fields.other')

      // Determine category based on model name
      if (
        model.toLowerCase().includes('gpt') ||
        model.toLowerCase().includes('o1') ||
        model.toLowerCase().includes('o3')
      ) {
        category = 'OpenAI'
      } else if (model.toLowerCase().includes('claude')) {
        category = 'Anthropic'
      } else if (model.toLowerCase().includes('gemini')) {
        category = 'Gemini'
      } else if (model.toLowerCase().includes('qwen')) {
        category = 'Qwen'
      } else if (model.toLowerCase().includes('deepseek')) {
        category = 'DeepSeek'
      } else if (model.toLowerCase().includes('glm')) {
        category = 'Zhipu'
      } else if (model.toLowerCase().includes('llama')) {
        category = 'Meta'
      } else if (model.toLowerCase().includes('mistral')) {
        category = 'Mistral'
      }

      if (!categories[category]) {
        categories[category] = []
      }
      categories[category].push(model)
    })

    return categories
  }

  // Filter models by search
  const filteredModels = useMemo(() => {
    if (!searchKeyword) return fetchedModels
    const kw = searchKeyword.toLowerCase()
    return fetchedModels.filter((model) =>
      typeof model === 'string' ? model.toLowerCase().includes(kw) : false
    )
  }, [fetchedModels, searchKeyword])

  // Helper to check if a model is considered "existing" (in selected or redirect)
  const isExistingModel = (model: string) =>
    classificationSet.has(normalizeModelName(model))

  // Separate new and existing models
  const newModels = filteredModels.filter((m) => !isExistingModel(m))
  const existingFilteredModels = filteredModels.filter((m) =>
    isExistingModel(m)
  )

  const newModelsByCategory = categorizeModels(newModels)
  const existingModelsByCategory = categorizeModels(existingFilteredModels)

  // 已移除页签会随关键词过滤出现/消失，受控值需要回退到仍然存在的页签
  const tabValue: ModelsTabValue =
    activeTab === 'removed' && removedModels.length === 0
      ? newModels.length > 0
        ? 'new'
        : 'existing'
      : activeTab

  // 厂商分类按 a-z 排序，Other 放最后，便于查找
  const getSortedCategoryEntries = (
    categories: Record<string, string[]>
  ): [string, string[]][] =>
    Object.entries(categories).sort(([a], [b]) => {
      if (a === 'Other') return 1
      if (b === 'Other') return -1
      return a.localeCompare(b, undefined, { sensitivity: 'base' })
    })

  const toggleModel = (model: string) => {
    setSelectedModels((prev) =>
      prev.includes(model) ? prev.filter((m) => m !== model) : [...prev, model]
    )
  }

  const toggleCategory = (categoryModels: string[], isChecked: boolean) => {
    setSelectedModels((prev) => {
      if (isChecked) {
        const newSelected = [...prev]
        categoryModels.forEach((model) => {
          if (!newSelected.includes(model)) {
            newSelected.push(model)
          }
        })
        return newSelected
      } else {
        return prev.filter((m) => !categoryModels.includes(m))
      }
    })
  }

  const isCategorySelected = (categoryModels: string[]) => {
    return categoryModels.every((m) => selectedModels.includes(m))
  }

  const renderModelCategory = (
    categoryName: string,
    categoryModels: string[]
  ) => {
    const allSelected = isCategorySelected(categoryModels)

    return (
      <Collapsible key={categoryName} defaultOpen>
        <CollapsibleTrigger className='hover:bg-muted/50 flex w-full items-center justify-between rounded-lg border p-3'>
          <div className='flex items-center gap-2'>
            <ChevronDown className='h-4 w-4' />
            <span className='font-medium'>
              {categoryName} ({categoryModels.length})
            </span>
          </div>
          <div className='flex items-center gap-2'>
            <span className='text-muted-foreground text-sm'>
              {t('channels.fields.selectedOutOfTotal', {
                selected: categoryModels.filter((m) =>
                  selectedModels.includes(m)
                ).length,
                total: categoryModels.length,
              })}
            </span>
            <Checkbox
              checked={allSelected}
              onCheckedChange={(checked) =>
                toggleCategory(categoryModels, !!checked)
              }
              onClick={(e) => e.stopPropagation()}
            />
          </div>
        </CollapsibleTrigger>
        <CollapsibleContent className='px-4 py-2'>
          <div className='grid grid-cols-2 gap-2'>
            {categoryModels.map((model) => (
              <div key={model} className='flex items-center space-x-2'>
                <Checkbox
                  id={model}
                  checked={selectedModels.includes(model)}
                  onCheckedChange={() => toggleModel(model)}
                />
                <Label
                  htmlFor={model}
                  className='flex cursor-pointer items-center gap-1.5 text-sm font-normal'
                >
                  <span>{model}</span>
                  {redirectOnlySet.has(normalizeModelName(model)) && (
                    <Tooltip>
                      <TooltipTrigger
                        render={<Info className='h-3.5 w-3.5 text-amber-500' />}
                      ></TooltipTrigger>
                      <TooltipContent>
                        {t(
                          'channels.tips.modelRedirectNotYetAddedToModelsList'
                        )}
                      </TooltipContent>
                    </Tooltip>
                  )}
                </Label>
              </div>
            ))}
          </div>
        </CollapsibleContent>
      </Collapsible>
    )
  }

  return (
    <Dialog open={open} onOpenChange={handleClose}>
      <DialogContent className='max-h-[90vh] sm:max-w-5xl'>
        <DialogHeader>
          <DialogTitle>{t('channels.titles.fetchModels')}</DialogTitle>
          <DialogDescription>
            {activeChannel ? (
              <>
                {t('channels.titles.fetchAvailableModelsFor')}{' '}
                <strong>{activeChannel.name}</strong>
              </>
            ) : channelName ? (
              <>
                {t('channels.titles.fetchAvailableModelsFor')}{' '}
                <strong>{channelName}</strong>
              </>
            ) : (
              t('channels.actions.fetchAvailableModelsFromUpstream')
            )}
          </DialogDescription>
        </DialogHeader>

        {!activeChannel && !customFetcher ? (
          <div className='text-muted-foreground py-8 text-center'>
            {t('channels.fields.noChannelSelected')}
          </div>
        ) : isFetching ? (
          <div className='flex items-center justify-center py-12'>
            <Loader2 className='text-muted-foreground h-8 w-8 animate-spin' />
          </div>
        ) : fetchedModels.length === 0 && removedModels.length === 0 ? (
          <div className='text-muted-foreground py-8 text-center'>
            <p>{t('channels.tips.noModelsFetchedYet')}</p>
            <Button
              className='mt-4'
              onClick={handleFetchModels}
              disabled={isFetching}
            >
              {t('channels.titles.fetchModels')}
            </Button>
          </div>
        ) : (
          <>
            <div className='space-y-4'>
              {/* Search Bar */}
              <div className='relative'>
                <Search className='text-muted-foreground absolute top-1/2 left-3 h-4 w-4 -translate-y-1/2' />
                <Input
                  placeholder={t('common.actions.searchModels')}
                  value={searchKeyword}
                  onChange={(e) => setSearchKeyword(e.target.value)}
                  className='pl-9'
                />
              </div>

              {/* Tabs for New vs Existing vs Removed */}
              {/* 页签保持受控：不能用 key/defaultValue 让组件随过滤结果重挂载，
                  否则关键词导致的数量抖动会重置甚至丢失选中页签。 */}
              <Tabs
                value={tabValue}
                onValueChange={(value) => setActiveTab(value as ModelsTabValue)}
              >
                <TabsList
                  className={`grid w-full ${removedModels.length > 0 ? 'grid-cols-3' : 'grid-cols-2'}`}
                >
                  <TabsTrigger value='new'>
                    {t('channels.titles.newModelsCount', {
                      count: newModels.length,
                    })}
                  </TabsTrigger>
                  <TabsTrigger value='existing'>
                    {t('channels.titles.existingModelsCount', {
                      count: existingFilteredModels.length,
                    })}
                  </TabsTrigger>
                  {removedModels.length > 0 && (
                    <TabsTrigger value='removed'>
                      {t('channels.status.removedModelsCount', {
                        count: removedModels.length,
                      })}
                    </TabsTrigger>
                  )}
                </TabsList>

                <TabsContent
                  value='new'
                  className='max-h-[calc(90vh-18rem)] space-y-2 overflow-y-auto'
                >
                  {newModels.length === 0 ? (
                    <p className='text-muted-foreground py-8 text-center text-sm'>
                      {t('channels.tips.noModelsMatchSearch')}
                    </p>
                  ) : (
                    getSortedCategoryEntries(newModelsByCategory).map(
                      ([category, models]) =>
                        renderModelCategory(category, models)
                    )
                  )}
                </TabsContent>

                <TabsContent
                  value='existing'
                  className='max-h-[calc(90vh-18rem)] space-y-2 overflow-y-auto'
                >
                  {existingFilteredModels.length === 0 ? (
                    <p className='text-muted-foreground py-8 text-center text-sm'>
                      {t('channels.tips.noModelsMatchSearch')}
                    </p>
                  ) : (
                    getSortedCategoryEntries(existingModelsByCategory).map(
                      ([category, models]) =>
                        renderModelCategory(category, models)
                    )
                  )}
                </TabsContent>

                {removedModels.length > 0 && (
                  <TabsContent
                    value='removed'
                    className='max-h-[calc(90vh-18rem)] space-y-2 overflow-y-auto'
                  >
                    <p className='text-muted-foreground text-xs'>
                      {t(
                        'channels.tips.modelsAreStillInYourSelectionButWereNot'
                      )}
                    </p>
                    {renderModelCategory(
                      t('channels.status.removed'),
                      removedModels
                    )}
                  </TabsContent>
                )}
              </Tabs>

              {/* Selection Summary */}
              <div className='bg-muted/50 rounded-lg border p-3 text-sm'>
                {t('channels.fields.nModelSSelected', {
                  n: selectedModels.length,
                })}
              </div>
            </div>

            <DialogFooter>
              <Button
                variant='outline'
                onClick={handleClose}
                disabled={isSaving}
              >
                {t('common.actions.cancel')}
              </Button>
              <Button onClick={handleSave} disabled={isSaving}>
                {isSaving && <Loader2 className='mr-2 h-4 w-4 animate-spin' />}
                {isSaving
                  ? t('channels.tips.saving')
                  : t('channels.actions.saveModels')}
              </Button>
            </DialogFooter>
          </>
        )}
      </DialogContent>
    </Dialog>
  )
}
