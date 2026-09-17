// TanStack Table v9 统一入口：项目所有表格相关导入都从这里走。
//
// 历史背景：dependabot 升级到 v9 后，此文件曾把 v8 API 集中映射到官方
// deprecated 的 `/legacy` 入口完成止血，现已全面切换到 v9 原生 API。
//
// `appTableFeatures` 在这里一次性注册项目用到的 feature 与 row model
// 工厂，所有 `useTable` 实例共享；Table/Row/Column/Cell/ColumnDef 类型
// 别名预绑定 AppTableFeatures，业务代码保持 v8 的单参数写法。
import {
  columnFacetingFeature,
  columnFilteringFeature,
  columnSizingFeature,
  columnVisibilityFeature,
  createExpandedRowModel,
  createFacetedRowModel,
  createFacetedUniqueValues,
  createFilteredRowModel,
  createPaginatedRowModel,
  createSortedRowModel,
  globalFilteringFeature,
  rowExpandingFeature,
  rowPaginationFeature,
  rowSelectionFeature,
  rowSortingFeature,
  tableFeatures,
  type Cell as TanstackCell,
  type CellData,
  type Column as TanstackColumn,
  type ColumnDef as TanstackColumnDef,
  type ReactTable,
  type Row as TanstackRow,
  type RowData,
} from '@tanstack/react-table'

/**
 * 项目级 feature 注册表，供所有 `useTable` 实例通过
 * `features: appTableFeatures` 共享。
 *
 * 覆盖项目全部用法：列宽（size/getSize）/ 列筛选 / 全局筛选 / 列可见性 /
 * faceted / 行展开 / 行分页 / 行选择 / 行排序；core row model 由 v9 内置，
 * 无需注册。row model 工厂从 v8 的每表 options 搬进 features 槽位。
 */
export const appTableFeatures = tableFeatures({
  columnFacetingFeature,
  columnFilteringFeature,
  columnSizingFeature,
  columnVisibilityFeature,
  globalFilteringFeature,
  rowExpandingFeature,
  rowPaginationFeature,
  rowSelectionFeature,
  rowSortingFeature,
  expandedRowModel: createExpandedRowModel(),
  facetedRowModel: createFacetedRowModel(),
  facetedUniqueValues: createFacetedUniqueValues(),
  filteredRowModel: createFilteredRowModel(),
  paginatedRowModel: createPaginatedRowModel(),
  sortedRowModel: createSortedRowModel(),
})

export type AppTableFeatures = typeof appTableFeatures

export * from '@tanstack/react-table'

/**
 * 预绑定 AppTableFeatures 的常用类型别名。
 *
 * v9 泛型第一参数固定为项目级 feature 注册表，业务代码统一使用这些别名，
 * 与 v8 的单参数写法保持一致；需要不同 feature 集合的表格可直接使用
 * 原生泛型类型。
 */
export type Table<TData extends RowData> = ReactTable<AppTableFeatures, TData>
export type Row<TData extends RowData> = TanstackRow<AppTableFeatures, TData>
export type Column<
  TData extends RowData,
  TValue extends CellData = CellData,
> = TanstackColumn<AppTableFeatures, TData, TValue>
export type Cell<
  TData extends RowData,
  TValue extends CellData = CellData,
> = TanstackCell<AppTableFeatures, TData, TValue>
export type ColumnDef<
  TData extends RowData,
  TValue extends CellData = CellData,
> = TanstackColumnDef<AppTableFeatures, TData, TValue>
export type { ColumnVisibilityState as VisibilityState } from '@tanstack/react-table'
