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
import { useContext } from 'react'
import { createPortal } from 'react-dom'
import { useTranslation } from 'react-i18next'
import { SettingsPageContext } from './settings-page-context-value'

type FormDirtyIndicatorProps = {
  isDirty: boolean
  message?: string
}

/**
 * 页头标题旁的「未保存的更改」标记，表单有未保存改动时出现。
 *
 * @example
 * ```tsx
 * <FormDirtyIndicator isDirty={form.formState.isDirty} />
 * ```
 */
export function FormDirtyIndicator({
  isDirty,
  message,
}: FormDirtyIndicatorProps) {
  const { titleStatusContainer } = useContext(SettingsPageContext)
  const { t } = useTranslation()

  if (!isDirty || !titleStatusContainer) return null

  return createPortal(
    <span className='bg-warning/10 text-foreground ring-warning/30 inline-flex h-5 items-center gap-1.5 rounded-full px-2 text-xs font-medium whitespace-nowrap ring-1'>
      <span className='bg-warning size-1.5 rounded-full' />
      {message ? t(message) : t('systemSettings.fields.unsavedChanges')}
    </span>,
    titleStatusContainer
  )
}
