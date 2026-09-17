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
import type { CellData, RowData, TableFeatures } from '@tanstack/table-core'

// v9 中 ColumnMeta 由 @tanstack/table-core 声明（@tanstack/react-table 仅
// re-export），声明合并必须指向声明方模块；泛型签名与 v8 不同。
declare module '@tanstack/table-core' {
  // Extended column metadata for enhanced table functionality
  // 泛型名加 _ 前缀仅为满足 ESLint unused 规则；TS 声明合并仍按位置匹配。
  interface ColumnMeta<
    _TFeatures extends TableFeatures,
    _TData extends RowData,
    _TValue extends CellData = CellData,
  > {
    // Human-readable label for the column
    label?: string
    // Optional description shown in tooltips or help text
    description?: string
    // Whether this column can be sorted (overrides default behavior)
    sortable?: boolean
    // Custom CSS classes to apply to the column cells
    className?: string
    // Hide this column in the mobile card list layout
    mobileHidden?: boolean
    // Mark this column as the title in the mobile card list
    // (true) or override the title text with a string
    mobileTitle?: boolean | string
    // Render this column as an inline badge in the mobile card list
    mobileBadge?: boolean
  }
}
