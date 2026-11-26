# DESIGN.md

NookMux 前端（`web/`）的视觉与交互规范，团队与 AI 代理共读：`AGENTS.md` 管"怎么构建"，本文件管"界面长什么样"。源码是唯一真实来源，冲突时以源码为准并修正本文件。

令牌：`web/src/styles/theme.css`（颜色/圆角）、`theme-presets.css`（主题）、`web/src/lib/motion.ts`（动效）。基座：React 19 + Tailwind CSS 4 + Base UI + cva。

## 1. 概览

克制的单色现代主义：近黑/近白中性基底承载高密度数据，边界用极淡 `ring` 表达，颜色只出现在状态与数据处。管理台对标 Linear / Vercel 的工具感，公开页允许轻光泽感。

- 数据优先：数字 `tabular-nums`，表格强制 `text-sm`，数据区不加装饰容器
- 密度克制：紧凑间距，层级靠留白与分隔线，不做嵌套卡片
- 单一强调：主色为中性近黑/近白，颜色只给状态、图表、主题 preset
- 边界克制：卡片/弹层用 `ring-1 ring-foreground/10`，阴影只给浮层
- 双模一等公民：组件必须同时适配亮/暗、移动端、键盘
- 稳定不跳动：初载骨架屏，刷新用透明度，禁止整页刷新替代局部更新
- 复用优先：原语只用 `src/components/ui/`，列表页必须走 `DataTablePage`

## 2. 颜色

| 令牌 | 亮色 | 暗色 |
|------|------|------|
| `--background` | `oklch(1 0 0)` | `oklch(0.185 0 0)` |
| `--foreground` | `oklch(0.145 0 0)` | `oklch(0.965 0 0)` |
| `--card` / `--popover` | `oklch(1 0 0)` | `oklch(0.235 0 0)` / `oklch(0.255 0 0)` |
| `--primary` | `oklch(0.13 0 0)` | `oklch(0.965 0 0)` |
| `--secondary` / `--accent` | `oklch(0.95 0 0)` | `oklch(0.285 0 0)` / `oklch(0.325 0 0)` |
| `--muted` / `--muted-foreground` | `oklch(0.97 0 0)` / `oklch(0.49 0 0)` | `oklch(0.255 0 0)` / `oklch(0.76 0 0)` |
| `--border` / `--input` | `oklch(0.93 0 0)` | `oklch(1 0 0 / 9%)` / `oklch(1 0 0 / 16%)` |
| `--ring` | `oklch(0.708 0 0)` | `oklch(0.68 0 0)` |
| `--sidebar` | `oklch(0.975 0 0)` | `oklch(0.115 0 0)` |
| `--success` | `oklch(0.596 0.145 163.225)` | `oklch(0.696 0.17 162.48)` |
| `--warning` | `oklch(0.681 0.162 75.834)` | `oklch(0.769 0.188 70.08)` |
| `--info` | `oklch(0.588 0.158 241.966)` | `oklch(0.68 0.17 237.323)` |
| `--destructive` | `oklch(0.577 0.245 27.325)` | `oklch(0.704 0.191 22.216)` |
| `--neutral` | `oklch(0.708 0 0)` | `oklch(0.76 0 0)` |

- 只用语义令牌，禁止硬编码色值。唯二例外：弹层遮罩 `bg-black/10`、移动抽屉 `bg-black/50`，不得扩散
- 状态色分工：success 成功/启用 · warning 告警/限流 · info 提示/进行中 · destructive 失败/删除 · neutral 未知/离线
- `--chart-1..5` 只给图表；不在 `muted-foreground` 上再降透明度
- 暗色表面按 `0.185 → 0.235 → 0.255` 抬升，border/input 为 alpha 白，不用实色
- 8 套 preset 按 `color-mix(primary, background)` 重算表层令牌，硬编码颜色换肤后必然出错

## 3. 字体

- 默认 Public Sans（`--font-sans`，自托管）；可选 inter / manrope / system
- 字号：`text-xs` 徽章与辅助信息 · `text-sm` 表格/表单/弹层/设置页分区与面板标题（默认正文）· `text-base` 卡片标题 · `text-lg` 页面标题（`sm+`）/ `TitledCard`（个人中心、钱包）· `text-2xl+` 仅公开页
- 页面标题 `text-base font-bold tracking-tight sm:text-lg`；卡片标题 `text-base font-medium`；设置页分区与面板标题 `text-sm font-semibold`，行标题 `text-sm font-medium`
- 数据数字一律 `tabular-nums`
- 用户密度档 `data-theme-scale` 同时缩放字号与 `--spacing`，不写死刻度外间距

## 4. 组件

| 组件 | 规格 |
|------|------|
| Button | `h-8 rounded-lg px-2.5 text-sm`；default / outline / secondary / ghost / destructive（`-destructive/10` 淡化）/ link；`xs h-6` · `sm h-7` · `lg h-9` · `icon size-8` |
| Badge | `h-5 rounded-4xl border px-2 py-0.5 text-xs`，状态色只用语义令牌 |
| Input / Select | `h-8 rounded-lg text-sm`，暗色 `dark:bg-input/30`，错误态 `aria-invalid` |
| Card | `bg-card rounded-xl ring-1 ring-foreground/10 p-4`；hover 自动上浮 1px（≥641px）；设置页内带边框的容器只有 `SettingsPanel`（§7） |
| Dialog | `bg-popover rounded-xl ring-1 ring-foreground/10 p-4`，默认 `sm:max-w-sm`（表单需加宽），遮罩 `bg-black/10 backdrop-blur-xs` |
| 浮层 | Popover / Select / Dropdown 用 `rounded-lg shadow-md ring-1` |
| Table | 表头 `h-10 px-2 font-medium`，单元格 `p-2`，行 hover `bg-muted/50` |
| Empty / Skeleton | `Empty`：`rounded-xl border-dashed p-6`；`Skeleton`：`bg-muted animate-pulse` |
| 图标 | 原语层 Hugeicons（`strokeWidth={2}`），业务页 lucide-react；默认 `size-4`；图标按钮必须 `aria-label` |

- 圆角由 `--radius: 1rem` 乘法推导（`rounded-lg` 用于控件、`rounded-xl` 用于卡片/弹层），不写死像素
- 类名合并一律 `cn()`；优先扩展现有 variant，不 fork；新原语放 `src/components/ui/`
- 禁止：`Card` 包裹表格/筛选/分页，嵌套卡片，纯文字或 `Loader2` 占位

## 5. 布局与响应式

- Header 高 `3rem`（sticky，z-40）+ 侧栏 `13rem`（折叠 `2.75rem`，移动 `17rem`，`Ctrl/Cmd+B`）
- 页面壳 `SectionPageLayout` 三段式：`.Title` / `.Actions` 固定，`.Content` 承载内容，分页走 `PageFooterPortal`；`wrapActionsUnderTitle`（默认关闭，设置页启用）让标题至少占 12rem、标题与操作栏放不下同一行时操作栏整体换到标题下一行
- 列表页强制 `DataTablePage` + `SectionPageLayout`：页面高度固定不随数据伸缩，表格区顶到固定分页栏并在容器内滚动，sticky 表头由 `DataTablePage` 内置；移动端 `MobileCardList`；细则见 `docs/开发规范/list-page-table-spec.md`
- 断点用 Tailwind 默认值（sm 640 / md 768 / lg 1024 / xl 1280），≤640px 表格转移动卡片
- 公开页用 `PublicLayout` + `container`；管理页不得使用 Landing 的玻璃、渐变与入场动效

## 6. 动效

- 弹层用 `tw-animate-css`：`fade-in-0 zoom-in-95`，统一 `duration-100`
- 编排用 `motion/react`（`lib/motion.ts`）：`0.25s`，缓动 `cubic-bezier(0.33, 1, 0.68, 1)`；spring 仅用于侧栏
- 时长：卡片 hover 150ms · 表格行 120ms · 页面转场 250ms · 表格行入场 200ms（逐行 +25ms）
- 复用 `pageEnter` / `tableRow` / `cardItem` 变体与 stagger（列表 0.04 / 表格 0.03 / 卡片 0.05），不自定曲线
- 所有动效支持 `prefers-reduced-motion` 降级；按钮按下 `active:translate-y-px`

## 7. 模式

- 列表页：`SectionPageLayout` + `DataTablePage`（参照 `features/usage-logs/`）
- 统计概览：网格 + `tabular-nums` + `chart-*` 迷你图
- 状态徽章：`Badge` + 状态语义令牌；空态/骨架：`Empty` / `TableSkeleton` / `.skeleton-shimmer`
- 设置页：`SettingsGroup` 分区 + `SettingsRow` 行 + React Hook Form / Zod（细则见下）
- 全站个性化：`ConfigDrawer`（写入 `data-theme-*`）

### 设置页

页面由分区组成，分区由行组成，基础组件在 `features/system-settings/components/`；每页是一个 `SettingsSection`，页面标题与页头操作栏由 `SettingsPage` 框架提供。

- 一页只用一种行结构：随「保存」生效的设置写成 `SettingsGroup` 分区 + `SettingsRow` 行，行与行之间只用细横线；设置页里带边框的容器只有 `SettingsPanel`
- 分区 `SettingsGroup`：标题行（`title` / `description` / `actions`）+ 以横线分隔的行；功能的启用开关连同它的从属设置写成带 `control` 的分区：开关贴标题行右端（表单内 `control={<SettingsGroupSwitch name=… />} enabled={…}`，页面自管状态时传 `checked` / `onCheckedChange`），从属行缩进到一条竖线之下，开关关闭时整块禁用；彼此独立的选项开关写成开关行（见下）
- 行 `SettingsRow`：宽容器里左列 `label` + `description`，右列控件 + `hint`，标题与控件同处一条 32px 横带；是否分两列看所在容器宽度（容器查询 `@container`：42rem 起两列、标签列 14rem，72rem 起标签列 16rem），与视口无关，对话框等窄容器自动单列，页面宽度跟随主题的内容布局设置；表单容器用 `SettingsForm`
- 开关行两端对齐：只放一个开关的行由行组件自动识别，文字在左、最宽 48rem，开关贴行的右边缘；开关被其他组件包住时传 `trailing`；分区标题上的总开关同样贴右端；窄容器里开关行同样是标题与开关同处一行、说明在下一行；开关与其他控件同格的行仍从控件列左端开始
- 控件宽度 `SettingsControl size`：数值 `xs` · 下拉 `sm` · 单行文本 `md` · 长文本 `lg` · 通栏 `full`；多行文本域直接填满控件列；数值带单位用 `SettingsUnitField`，多选用 `SettingsChoiceGrid`（复选框 + 标题与说明，不逐项套卡片）
- 通栏行 `SettingsRow stacked`：自带工具栏、表格或编辑器的行，标题一行（右侧 `actions`）、内容占满下一行；「可视化 / JSON」编辑器的切换用 `EditorModeButton` 挂在标题行右侧，编辑器工具栏搜索靠左、添加靠右（`variant='outline' size='sm'` + `Plus`）；表格贴边对齐，不加第二层边框；表格里可省略的文本列写 `w-full min-w-… max-w-0 truncate` 占满剩余宽度，输入框列用 `min-w-…` 定下限，放不下时表格在面板内横向滚动；需要换行的表头写 `whitespace-normal [word-break:keep-all]`，按词换行、不在词内断开；页面里列表与编辑器左右分栏同样按容器宽度切换（模型定价页 56rem 起分两栏）
- 面板 `SettingsPanel`：只放不随「保存」逐项生效的内容——立即执行的运维操作、只读监控数据、危险操作、自带列表或需内部滚动的编辑器；`divided` 子元素间画横线 · `flush` 表格贴边 · `scroll` 内部滚动 · `footer` 底部操作栏 · `tone='destructive'` 危险区；只读指标用 `SettingsStatGrid` / `SettingsStat` / `SettingsMeter`（列数同样看容器宽度，2 列固定）
- 列表面板 `SettingsListPanel`：公告、API 地址、常见问题、Uptime Kuma 分组这类增删改后统一保存的列表；标题行右侧「添加」（`actions`），勾选行后面板底部出现「已选 n 项」与批量操作（`selectedCount` / `bulkActions`）
- 提示 `SettingsNotice`：无框；分区或面板里的提示作为其中一个条目、用横线与上下分隔，整页都要知道的提示（如高风险操作警告）放在所有分区之前；`tone` 取 `info | warning | danger`，警告与危险带图标并用语义色
- 页头操作栏 `SettingsPageFormActions`：有未保存改动时页面标题旁出现「未保存的更改」标记，「重置」「保存」才可点；「恢复默认」式页面传 `resettableWhenClean` + `resetLabel`；标题至少占 12rem，标题与操作栏放不下同一行时操作栏整体换到标题下一行（`SectionPageLayout wrapActionsUnderTitle`）
- 页签统一 `TabsList variant='line'`；空状态面板外用 `Empty`，面板内用无框的居中灰字（`text-muted-foreground` + `text-center`）；配色只用语义令牌
- 对话框里的表单同样写成 `SettingsRow`（`<form className='@container space-y-4'>`）：窄对话框自动单列，字段多的编辑对话框用 `sm:max-w-3xl` 加宽后自动分两列

## 8. 禁止

- 硬编码颜色；mock 数据 / 假分页 / 静默吞错；整页刷新替代 `invalidateQueries`
- 引入第二套 UI 库；绕过 `cn()` 手拼动态类名；新增未维护语言（仅 zh / en）
- 移除焦点环；图标按钮缺 `aria-label`；用颜色作为唯一信息载体

## 9. 维护

- 写数值和类名，不写"美观 / 合理"；单条 ≤2 行；形容词必附落地规则；只写终态
- 改令牌 → `theme.css` + 本文 §2；改 preset → `lib/theme-customization.ts` + `theme-presets.css` + i18n `common.json`；改动效 → `lib/motion.ts`
- 改组件 → `src/components/ui/*.tsx` + 本文 §4；改布局常量 → 对应布局组件 + 本文 §5；改设置页结构 → `features/system-settings/components/` + 本文 §7
- 提交前：亮暗双模目检 · 移动端 <640px · 列表页走 `DataTablePage` · 三态齐备 · `typecheck` / `lint` / `build` 通过
