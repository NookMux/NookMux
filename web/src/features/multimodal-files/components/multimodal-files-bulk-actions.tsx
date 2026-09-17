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
import { Trash2 } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { type Table } from '@/lib/tanstack-table'
import { Button } from '@/components/ui/button'
import { DataTableBulkActions } from '@/components/data-table'
import type { StoredMediaItem } from '../types'

export interface MultimodalFilesBulkActionsProps {
  table: Table<StoredMediaItem>
  onDeleteSelected: () => void
}

export function MultimodalFilesBulkActions({
  table,
  onDeleteSelected,
}: MultimodalFilesBulkActionsProps) {
  const { t } = useTranslation()

  return (
    <DataTableBulkActions
      table={table}
      entityName={t('multimodalFiles.fields.files')}
    >
      <Button variant='destructive' size='sm' onClick={onDeleteSelected}>
        <Trash2 />
        {t('multimodalFiles.actions.deleteSelected')}
      </Button>
    </DataTableBulkActions>
  )
}
