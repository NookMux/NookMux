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
import {
  useContext,
  type ComponentProps,
  type ReactNode,
  type RefObject,
} from 'react'
import { useFormContext } from 'react-hook-form'
import { RotateCcw, Save } from 'lucide-react'
import { createPortal } from 'react-dom'
import { useTranslation } from 'react-i18next'
import { Button } from '@/components/ui/button'
import { FormDirtyIndicator } from './form-dirty-indicator'
import { SettingsPageContext } from './settings-page-context-value'

type SettingsPageProviderProps = {
  actionsContainer: HTMLDivElement | null
  titleStatusContainer?: HTMLSpanElement | null
  children: ReactNode
  suppressSectionHeader?: boolean
}

export function SettingsPageProvider(props: SettingsPageProviderProps) {
  return (
    <SettingsPageContext.Provider
      value={{
        actionsContainer: props.actionsContainer,
        titleStatusContainer: props.titleStatusContainer ?? null,
        suppressSectionHeader: props.suppressSectionHeader ?? true,
      }}
    >
      {props.children}
    </SettingsPageContext.Provider>
  )
}

type SettingsPageActionsPortalProps = {
  children: ReactNode
}

export function SettingsPageActionsPortal(
  props: SettingsPageActionsPortalProps
) {
  const { actionsContainer } = useContext(SettingsPageContext)

  if (!actionsContainer) return null

  return createPortal(
    <div className='flex flex-wrap items-center justify-end gap-2'>
      {props.children}
    </div>,
    actionsContainer
  )
}

type SettingsPageFormActionsProps = {
  onSave: () => void
  /** 放弃未保存的改动；不传时用所在表单的 reset。 */
  onReset?: () => void
  isSaving?: boolean
  /** 页面自行管理状态、不在表单内时，直接告知是否有未保存的改动。 */
  isDirty?: boolean
  /** 额外的禁用保存原因，例如编辑器当前不合法；没有未保存的改动时总会禁用。 */
  isSaveDisabled?: boolean
  /** 额外的禁用重置原因。 */
  isResetDisabled?: boolean
  /** 「恢复默认」式页面：没有未保存的改动时也可以重置。 */
  resettableWhenClean?: boolean
  saveLabel?: string
  savingLabel?: string
  resetLabel?: string
  resetVariant?: ComponentProps<typeof Button>['variant']
  saveButtonRef?: RefObject<HTMLButtonElement | null>
}

/**
 * 设置页的页头操作栏：标题旁的未保存标记，以及「重置」「保存」两个按钮。
 * 按钮位于表单内时自动跟随未保存状态，有改动才可点击；
 * 「恢复默认」式页面用 resettableWhenClean 让重置按钮始终可点。
 */
export function SettingsPageFormActions(props: SettingsPageFormActionsProps) {
  const { t } = useTranslation()
  const form = useFormContext()
  const isDirty = props.isDirty ?? form?.formState.isDirty ?? false
  const clean = !isDirty
  const resetDisabled =
    props.isSaving ||
    (clean && !props.resettableWhenClean) ||
    (props.isResetDisabled ?? false)
  const saveDisabled =
    props.isSaving || clean || (props.isSaveDisabled ?? false)
  const handleReset = props.onReset ?? (form ? () => form.reset() : undefined)
  const saveLabel = props.isSaving
    ? (props.savingLabel ?? 'common.tips.saving')
    : (props.saveLabel ?? 'common.actions.saveChanges')

  return (
    <>
      <FormDirtyIndicator isDirty={isDirty} />
      <SettingsPageActionsPortal>
        {handleReset && (
          <Button
            type='button'
            size='sm'
            variant={props.resetVariant ?? 'outline'}
            onClick={handleReset}
            disabled={resetDisabled}
          >
            <RotateCcw data-icon='inline-start' />
            <span>{t(props.resetLabel ?? 'common.actions.reset')}</span>
          </Button>
        )}
        <Button
          ref={props.saveButtonRef}
          type='button'
          size='sm'
          onClick={props.onSave}
          disabled={saveDisabled}
        >
          <Save data-icon='inline-start' />
          <span>{t(saveLabel)}</span>
        </Button>
      </SettingsPageActionsPortal>
    </>
  )
}
