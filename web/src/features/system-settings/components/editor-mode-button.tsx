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
import { Code2, Eye } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { Button } from '@/components/ui/button'

type EditorModeButtonProps = {
  /** 当前是否为可视化编辑；按钮的标签说明点下去会切到哪一种。 */
  visualMode: boolean
  onToggle: () => void
}

/** 可视化编辑器与 JSON 文本框之间的切换按钮。 */
export function EditorModeButton({
  visualMode,
  onToggle,
}: EditorModeButtonProps) {
  const { t } = useTranslation()

  return (
    <Button type='button' variant='outline' size='sm' onClick={onToggle}>
      {visualMode ? (
        <>
          <Code2 data-icon='inline-start' />
          {t('systemSettings.fields.jsonEditor')}
        </>
      ) : (
        <>
          <Eye data-icon='inline-start' />
          {t('systemSettings.fields.visualEditor')}
        </>
      )}
    </Button>
  )
}
