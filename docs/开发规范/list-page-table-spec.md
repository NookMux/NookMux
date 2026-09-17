# 列表/表格类页面开发规范

本文件是前端列表/表格类页面的详细开发规范。`web/AGENTS.md` 中的"列表/表格类页面规范"
小节仅包含强制摘要并链接到此处，完整约定、示例和检查清单以本文件为准。

## 适用范围

管理后台凡是以"分页/可筛选的数据行列表"为主体的页面，必须遵循本规范。典型页面包括：

- API 密钥列表（`features/keys/`）
- 使用日志（`features/usage-logs/`，标准参照）
- 多模态文件（`features/multimodal-files/`）
- 历史订单（`features/order-query/`）
- 动态倍率（`features/dynamic-ratio/`）
- 渠道（`features/channels/`）
- 模型（`features/models/`）
- 用户（`features/users/`）
- 审计日志（`features/audit-logs/`）
- 音色管理（`features/minimax/voice-management/`）

## 标准参照页面

**使用日志**（`features/usage-logs/components/usage-logs-table.tsx`）是唯一标准参照。
该页面实现了完整的标准布局：

- `SectionPageLayout` 三段式壳层（顶部固定 + 中间滚动 + 底部固定分页）
- `DataTablePage` 统一表格容器
- sticky 表头（`tableClassName='overflow-x-auto'` + `tableHeaderClassName='bg-muted/30 sticky top-0 z-10'`）
- 自定义 toolbar 筛选区（`CommonLogsFilterBar`）
- `renderRow` 自定义行渲染
- 内置骨架屏 / 空状态

## 核心组件

### SectionPageLayout

路径：`@/components/layout`

三段式 flex 布局，提供"顶部固定 + 中间滚动 + 底部固定"结构：

- `SectionPageLayout.Title`：页面标题（`shrink-0`，固定不滚动）
- `SectionPageLayout.Actions`：顶部操作按钮区（`shrink-0`，固定不滚动）
- `SectionPageLayout.Content`：中间内容区（`min-h-0 flex-1 overflow-auto`，内部滚动）
- 底部 footer：经 `PageFooterPortal` 渲染分页（`shrink-0 border-t`，固定不滚动）

### DataTablePage

路径：`@/components/data-table`

统一表格容器，封装了以下能力：

| 能力 | 说明 |
|------|------|
| toolbar | 筛选区（默认 `DataTableToolbar` 或自定义 `toolbar` slot） |
| 表格本体 | `overflow-hidden rounded-lg border` 边框表格（非 Card） |
| 分页 | 默认走 `PageFooterPortal` 渲染到固定底栏 |
| 骨架屏 | 内置 `TableSkeleton`（传 `isLoading`） |
| 空状态 | 内置 `TableEmpty`（传 `emptyTitle` / `emptyDescription`） |
| 移动端卡片 | 内置 `MobileCardList`（默认启用，可 `hideMobile` 关闭） |
| 批量操作 | `bulkActions` slot 渲染 `DataTableBulkActions` |
| afterTable | 表格与分页之间的额外内容区 |

### DataTablePageProps 完整列表

```typescript
type DataTablePageProps<TData> = {
  table: TanstackTable<TData>           // TanStack Table 实例
  columns: ColumnDef<TData>[]           // 列定义
  isLoading?: boolean                   // 初始加载（渲染骨架屏）
  isFetching?: boolean                  // 后台刷新（表格半透明）
  emptyTitle?: string                   // 空状态标题
  emptyDescription?: string             // 空状态描述
  emptyIcon?: React.ReactNode           // 空状态图标
  emptyAction?: React.ReactNode         // 空状态额外内容
  toolbar?: React.ReactNode             // 自定义 toolbar（替换默认）
  toolbarProps?: DataTablePageToolbarProps | null  // 默认 toolbar 配置
  bulkActions?: React.ReactNode         // 批量操作栏
  mobile?: React.ReactNode              // 自定义移动端列表（替换默认）
  mobileProps?: { getRowKey?, getRowClassName? }   // 移动端列表配置
  hideMobile?: boolean                  // 禁用移动端布局
  getRowClassName?: (row, ctx) => string | undefined  // 行 className
  renderRow?: (row) => React.ReactNode  // 自定义行渲染
  applyHeaderSize?: boolean             // 应用列 size 到表头宽度
  skeletonKeyPrefix?: string            // 骨架屏 key 前缀
  showPagination?: boolean              // 是否渲染分页（默认 true）
  paginationInFooter?: boolean          // 分页走 PageFooterPortal（默认 true）
  afterTable?: React.ReactNode          // 表格后额外内容
  className?: string                    // 外层 wrapper className
  tableClassName?: string               // 表格容器 className
  tableHeaderClassName?: string         // 表头 className
}
```

## 强制规则

### 1. 必须使用 DataTablePage

列表页必须使用 `DataTablePage` 作为表格容器。禁止：

- 自行用 `Table` / `TableHeader` / `TableBody` 手拼列表
- 用 `Card` / `CardContent` 包裹表格
- 自行用 `Select` + 上/下页 `Button` 拼装分页控件

### 2. 必须使用 SectionPageLayout

页面壳层使用 `SectionPageLayout`，表格渲染在 `SectionPageLayout.Content` 内。

### 3. Sticky 表头约定

传入以下属性使表头在内容区滚动时贴住容器顶部固定：

```tsx
<DataTablePage
  tableClassName='overflow-x-auto'
  tableHeaderClassName='bg-muted/30 sticky top-0 z-10'
  // ...
/>
```

列定义有 `size` 时同时传 `applyHeaderSize` 以生效列宽。

### 4. Card 禁用规则

- 表格区域不得用 `Card` / `CardContent` 包裹。`DataTablePage` 内置的
  `overflow-hidden rounded-lg border` 边框表格即为标准外观。
- 筛选区、分页区同样不得包成独立圆角卡片。
- 需要展示统计/状态信息时，用 `afterTable` slot 或 `SectionPageLayout.Actions` 承载。

### 5. 分页约定

- 分页默认走 `PageFooterPortal`（`DataTablePage` 默认行为）。
- 无分页需求的页面（如全量加载的规则列表）传 `showPagination={false}`。
- 需要行内 inline 分页时传 `paginationInFooter={false}`（一般不用）。

### 6. 筛选区约定

- 筛选区作为 `toolbar` slot 传入 `DataTablePage`。
- 简单搜索走默认 `DataTableToolbar`（通过 `toolbarProps` 传 `searchPlaceholder` / `filters`）。
- 复杂筛选（日期范围、多字段表单）传自定义节点到 `toolbar` prop。
- 筛选区不要内嵌在表格 `Card` 内，也不要混进 `Table` 之上自成一块。

### 7. 骨架屏与空状态

- loading 必须用 `DataTablePage` 内置的 `TableSkeleton`（传 `isLoading`），禁止纯文字"加载中"或 `Loader2` 占位。
- 空状态必须用内置 `TableEmpty`（传 `emptyTitle` / `emptyDescription`），禁止单行 `TableCell colSpan` 文字。
- 移动端用内置 `MobileCardList`，或通过 `mobile` slot 自定义；只读短表可传 `hideMobile`。

### 8. 批量操作

- 行选择用 TanStack Table `enableRowSelection`。
- 批量操作栏通过 `bulkActions` slot 传入 `DataTableBulkActions`，不要另起固定浮层。

## 豁免清单

以下页面无列表语义，不适用本规范，保持表单形态：

| 页面 | 路径 | 原因 |
|------|------|------|
| 系统设置-模型定价/工具定价 | `features/system-settings/models/` | `SettingsSection` + `Tabs` + RHF/Zod 表单 + 可视化规则编辑器 |
| 系统设置其他子页面 | `features/system-settings/` | 通用配置表单、JSON 编辑器 |
| 公开定价目录页 | `features/pricing/` | 使用 `PublicLayout` 的对外营销页面 |

## 标准模板

```tsx
import { SectionPageLayout } from '@/components/layout'
import { DataTablePage } from '@/components/data-table'

export function MyListPage() {
  // ... useTable setup（features: appTableFeatures）...

  return (
    <SectionPageLayout>
      <SectionPageLayout.Title>{t('myFeature.titles.list')}</SectionPageLayout.Title>
      <SectionPageLayout.Actions>
        <Button onClick={handleCreate}>
          <Plus />
          {t('myFeature.actions.create')}
        </Button>
      </SectionPageLayout.Actions>
      <SectionPageLayout.Content>
        <DataTablePage
          table={table}
          columns={columns}
          isLoading={isLoading}
          isFetching={isFetching}
          emptyTitle={t('myFeature.titles.noItemsFound')}
          emptyDescription={t('myFeature.tips.noItemsAvailable')}
          skeletonKeyPrefix='my-feature-skeleton'
          tableClassName='overflow-x-auto'
          tableHeaderClassName='bg-muted/30 sticky top-0 z-10'
          toolbar={<MyFilterBar table={table} />}
          bulkActions={<DataTableBulkActions table={table} />}
        />
      </SectionPageLayout.Content>
    </SectionPageLayout>
  )
}
```

## 新建/改造列表页检查清单

对照"使用日志"逐项验证：

- [ ] 使用 `SectionPageLayout` 作为页面壳
- [ ] 使用 `DataTablePage` 作为表格容器
- [ ] 传入 `tableClassName='overflow-x-auto'` + `tableHeaderClassName` sticky
- [ ] 表格为 `border` 边框，未被 `Card` 包裹
- [ ] 分页走 `PageFooterPortal`（默认）或显式 `showPagination={false}`
- [ ] loading 用 `TableSkeleton`，空状态用 `TableEmpty`
- [ ] 移动端用 `MobileCardList`（默认）或合理 `hideMobile`
- [ ] 筛选区为 `toolbar` slot，未内嵌在表格 Card 内
- [ ] 批量操作通过 `bulkActions` slot（如有行选择）
- [ ] 统计/状态信息走 `afterTable` 或 `SectionPageLayout.Actions`，未用 `Card`
