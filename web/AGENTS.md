# web/AGENTS.md

`web/` 是前端单页应用目录，基于 React 19、TypeScript、Rsbuild 与 Tailwind CSS 4 构建企业级 AI API 网关管理控制台。

## 包管理器与技术栈

- **包管理器**：**严格使用 Bun**。依赖锁文件为 `bun.lock`，严禁使用 npm、pnpm 或 yarn。
- **框架与运行时**：React 19、TypeScript、Rsbuild。
- **路由**：`@tanstack/react-router`，文件路由集中在 `src/routes/`。
- **状态与请求**：`@tanstack/react-query`、Zustand、统一 Axios 实例 `src/lib/api.ts`。
- **UI 组件库**：Base UI、Tailwind CSS 4、`src/components/ui/`、Hugeicons / Lucide 图标。
- **表单与校验**：React Hook Form + Zod。
- **数据图表**：VChart v2。
- **国际化**：i18next + react-i18next（支持 `zh` 与 `en`）。

## 后端嵌入载体

- `web/embed.go` 是后端专用 Go 源文件，用于在 `web/dist` 旁声明 `//go:embed dist`；Go embed 不支持跨层级 `..`，因此该文件必须保留在本目录下，不可移入 `internal/app/webdist/`。
- `internal/app/webdist/` 是启动装配层使用的统一门面包；修改前端静态产物路径时必须同步检查该包与 `internal/httpapi/router/web_router.go`。
- 修改 `web/embed.go` 后至少执行：`go test ./internal/app/... ./internal/httpapi/router/...`。

## 核心命令

- 安装依赖：`bun install`
- 本地开发：`bun run dev`
- 类型检查：`bun run typecheck`
- 静态检查：`bun run lint`
- 代码格式检查：`bun run format:check`
- 代码格式修复：`bun run format`
- 单元测试：`bun test`
- 生产打包：`bun run build`
- 完整打包检查：`bun run build:check`
- 国际化字典同步：`bun run i18n:sync`
- 依赖漏洞审计：`bun audit --audit-level=high`

改动 TS/TSX 文件后必须执行 `bun run typecheck`。涉及路由、API 契约、页面核心组件或构建配置修改时，必须执行 `bun run build` 或 `bun run build:check`。

## 文件组织规范

- 业务功能模块集中在 `src/features/<feature>/`，标准结构包含：`api.ts`、`types.ts`、`constants.ts`、`components/`、`hooks/`、`lib/`。
- 路由组件只负责装配页面布局与路由级守卫校验，具体业务逻辑与交互下沉至对应 feature。
- 通用组件置于 `src/components/`，原子级基础 UI 原语置于 `src/components/ui/`。
- 全局工具函数置于 `src/lib/`，跨模块全局状态置于 `src/stores/`。
- 纯类型导入必须显式使用 `import type`。

## API 请求与数据流

- 统一通过 `src/lib/api.ts` 的 `api` 实例发起请求，保留 Cookie 凭证、错误拦截、GET 请求防重以及 `New-Api-User` Header。
- 数据查询使用 `useQuery`，写操作使用 `useMutation`；query key 采用层级数组结构并保持语义一致。
- 变更成功后根据影响边界精确执行 `queryClient.invalidateQueries`，严禁使用整页刷新替代局部响应式更新。
- 服务端数据契约以本项目后端代码为单一真实来源。第三方参考项目 API 若在本项目未实现，应隐藏入口或在前端做兼容适配，严禁私自强加后端业务 API。
- 严禁用前端 Mock 数据、伪造分页或静默吞错让流程"看起来能跑"。

## 国际化 (i18n)

- React 组件中使用 `const { t } = useTranslation()`；非 React 逻辑模块使用 `i18next.t`。
- 文案按 feature 物理拆分存储在 `src/i18n/locales/<locale>/<section>.json`（`<locale>` 为 `zh` 或 `en`；`<section>` 对应功能模块，如 `auth`、`channels`、`system-settings`；通用与布局文案分别放入 `common.json` 和 `layout.json`）。每个 section 文件为扁平的 `Record<string, string>` 映射。
- 语义 Key 命名规范：`<section>.<group>.<name>[.<attr>]`，前缀 `<section>` 与文件名一致；`<group>` 归敛为 `fields/actions/status/errors/tips/titles/placeholders`；`<name>` 使用 camelCase。语义 key 必须保持扁平键名，禁止写成嵌套 JSON 对象。
- 新增文案规范：必须同步在对应 feature 的 `zh/<section>.json` 与 `en/<section>.json` 中添加相应 key；新增 section 需在 `src/i18n/config.ts` 中注册挂载。
- 字典对齐校验：提交前执行 `bun run i18n:sync` 自动对齐与校验中英文键集合。
- 动态 Key 必须登记在 `src/i18n/static-keys.ts` 中，或确保代码中包含静态字面量 `t('...')` 以供扫描。
- 目前仅维护 `zh` 与 `en` 两种语言，严禁新增未维护的语言入口。


## 类型、表单与错误

- 避免 `any`；确实无法确定时用 `unknown` 并在边界收窄。
- 表单 schema 放在 feature 的 `lib/` 或相邻模块，用 Zod 定义并通过 `z.infer` 导出类型。
- 组件 props 保持明确类型。复杂页面优先拆出小组件、hooks、纯函数。
- toast 使用 `sonner`，文案要走 i18n。
- 禁止 `no-console` 违规；调试日志不要留在生产路径。

## 样式与交互

- Tailwind 为主，动态类名用项目 `cn()`/`tailwind-merge` 体系。
- 组件优先复用 `src/components/ui/` 和现有 feature 组件。
- UI 应支持深浅色、主题 preset、移动端和键盘操作。
- 不要把页面章节做成嵌套卡片；管理后台优先信息密度、清晰扫描和稳定布局。

## 列表/表格类页面规范

管理后台凡是以"分页/可筛选的数据行列表"为主体的页面，必须使用 `DataTablePage`
（`@/components/data-table`）+ `SectionPageLayout`（`@/components/layout`），不得
手拼 `Table` 或用 `Card` 包裹表格。标准参照页面为"使用日志"
（`features/usage-logs/components/usage-logs-table.tsx`）。

关键约定：

- **Sticky 表头**：传 `tableClassName='overflow-x-auto'` +
  `tableHeaderClassName='bg-muted/30 sticky top-0 z-10'`，有 `size` 时配 `applyHeaderSize`。
- **分页**：默认走 `PageFooterPortal`；无分页传 `showPagination={false}`。
- **筛选区**：作为 `toolbar` slot 传入，简单走 `toolbarProps`，复杂传自定义节点。
- **骨架屏/空状态**：必须用内置 `TableSkeleton` / `TableEmpty` / `MobileCardList`，
  禁止纯文字或 `Loader2` 占位。
- **Card 禁用**：表格、筛选区、分页区均不得包成独立圆角卡片；统计信息走
  `afterTable` slot 或 `SectionPageLayout.Actions`。
- **批量操作**：`enableRowSelection` + `bulkActions` slot。

纯表单/配置页（如系统设置定价、公开定价目录页）不适用本规范。完整规范、Props
列表、标准模板和检查清单见
[docs/开发规范/list-page-table-spec.md](../docs/开发规范/list-page-table-spec.md)。

