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
import { memo, useCallback, useState } from 'react'
import { type UseFormReturn, useWatch } from 'react-hook-form'
import { useTranslation } from 'react-i18next'
import { Button } from '@/components/ui/button'
import { Form } from '@/components/ui/form'
import { SettingsForm } from '../components/settings-form-layout'
import { SettingsPageFormActions } from '../components/settings-page-context'
import { SettingsPanel } from '../components/settings-panel'
import {
  ModelRatioVisualEditor,
  type ModelRatioField,
} from './model-ratio-visual-editor'

type ModelFormValues = {
  ModelPrice: string
  ModelRatio: string
  CacheRatio: string
  CreateCacheRatio: string
  CompletionRatio: string
  AudioRatio: string
  AudioCompletionRatio: string
  ContextPricing: string
}

type ModelRatioFormProps = {
  form: UseFormReturn<ModelFormValues>
  onSave: (values: ModelFormValues) => Promise<void>
  /** 恢复为上游默认价格，操作立即生效。 */
  onRestoreDefaults: () => void
  isSaving: boolean
  isResetting: boolean
  /** 所在页签是否为当前页签；页头操作栏只在当前页签渲染。 */
  active: boolean
}

export const ModelRatioForm = memo(function ModelRatioForm({
  form,
  onSave,
  onRestoreDefaults,
  isSaving,
  isResetting,
  active,
}: ModelRatioFormProps) {
  const { t } = useTranslation()
  const [isEditorValid, setIsEditorValid] = useState(true)
  const watchedValues = useWatch({ control: form.control })
  const values: ModelFormValues = {
    ModelPrice: watchedValues.ModelPrice ?? '',
    ModelRatio: watchedValues.ModelRatio ?? '',
    CacheRatio: watchedValues.CacheRatio ?? '',
    CreateCacheRatio: watchedValues.CreateCacheRatio ?? '',
    CompletionRatio: watchedValues.CompletionRatio ?? '',
    AudioRatio: watchedValues.AudioRatio ?? '',
    AudioCompletionRatio: watchedValues.AudioCompletionRatio ?? '',
    ContextPricing: watchedValues.ContextPricing ?? '',
  }

  const handleFieldChange = useCallback(
    (field: ModelRatioField, value: string) => {
      form.setValue(field, value, {
        shouldDirty: true,
        shouldValidate: true,
      })
    },
    [form]
  )

  const handleValidityChange = useCallback((isValid: boolean) => {
    setIsEditorValid(isValid)
  }, [])

  return (
    <Form {...form}>
      {active && (
        <SettingsPageFormActions
          onSave={form.handleSubmit(onSave)}
          isSaving={isSaving}
          isSaveDisabled={!isEditorValid}
        />
      )}
      <SettingsForm
        onSubmit={form.handleSubmit(onSave)}
        className='lg:min-h-0 lg:flex-1'
      >
        <ModelRatioVisualEditor
          modelPrice={values.ModelPrice}
          modelRatio={values.ModelRatio}
          cacheRatio={values.CacheRatio}
          createCacheRatio={values.CreateCacheRatio}
          completionRatio={values.CompletionRatio}
          audioRatio={values.AudioRatio}
          audioCompletionRatio={values.AudioCompletionRatio}
          contextPricing={values.ContextPricing}
          onChange={handleFieldChange}
          onValidityChange={handleValidityChange}
          isSaving={isSaving}
        />
      </SettingsForm>

      <SettingsPanel
        tone='destructive'
        className='shrink-0'
        title={t('systemSettings.actions.resetModelRatios')}
        description={t(
          'systemSettings.tips.clearCustomPricingRatiosAndRevertToUpstreamDefaults'
        )}
        footer={
          <Button
            type='button'
            variant='destructive'
            size='sm'
            onClick={onRestoreDefaults}
            disabled={isResetting}
          >
            {t('common.actions.resetToDefault')}
          </Button>
        }
      />
    </Form>
  )
})
