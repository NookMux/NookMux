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
import type { ReactNode } from 'react'
import { useTranslation } from 'react-i18next'
import { Table } from '@/components/ui/table'
import { SettingsPanel } from './settings-panel'

type SettingsListPanelProps = {
  /** 标题行右侧的操作，通常是「添加」。 */
  actions: ReactNode
  /** 已勾选的行数；大于 0 时底部出现批量操作栏。 */
  selectedCount: number
  /** 底部批量操作栏里的操作，排在已选数量之后。 */
  bulkActions?: ReactNode
  className?: string
  /** 表头与表体，空列表的提示行也写在这里。 */
  children: ReactNode
}

/**
 * 列表面板：可以增删改、最后统一保存的列表，例如常见问题与公告。
 * 表格贴边铺满面板；勾选行后底部出现批量操作栏。
 */
export function SettingsListPanel({
  actions,
  selectedCount,
  bulkActions,
  className,
  children,
}: SettingsListPanelProps) {
  const { t } = useTranslation()

  return (
    <SettingsPanel
      className={className}
      actions={actions}
      flush
      footer={
        selectedCount > 0 ? (
          <>
            <span className='text-muted-foreground mr-auto text-sm'>
              {t('systemSettings.status.rowsSelected', {
                count: selectedCount,
              })}
            </span>
            {bulkActions}
          </>
        ) : undefined
      }
    >
      <Table className='[&_td:first-child]:pl-4 [&_td:last-child]:pr-4 [&_th:first-child]:pl-4 [&_th:last-child]:pr-4'>
        {children}
      </Table>
    </SettingsPanel>
  )
}
