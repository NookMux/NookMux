# DESIGN.md

New API 前端（`web/`）的视觉与交互规范，团队与 AI 代理共读：`AGENTS.md` 管"怎么构建"，本文件管"界面长什么样"。源码是唯一真实来源，冲突时以源码为准并修正本文件。

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
- 字号：`text-xs` 徽章 · `text-sm` 表格/表单/弹层（默认正文）· `text-base` 卡片标题 · `text-lg` 页面标题（`sm+`）/ `TitledCard` · `text-2xl+` 仅公开页
- 页面标题 `text-base font-bold tracking-tight sm:text-lg`；卡片标题 `text-base font-medium`
- 数据数字一律 `tabular-nums`
- 用户密度档 `data-theme-scale` 同时缩放字号与 `--spacing`，不写死刻度外间距

## 4. 组件

| 组件 | 规格 |
|------|------|
| Button | `h-8 rounded-lg px-2.5 text-sm`；default / outline / secondary / ghost / destructive（`-destructive/10` 淡化）/ link；`xs h-6` · `sm h-7` · `lg h-9` · `icon size-8` |
| Badge | `h-5 rounded-4xl border px-2 py-0.5 text-xs`，状态色只用语义令牌 |
| Input / Select | `h-8 rounded-lg text-sm`，暗色 `dark:bg-input/30`，错误态 `aria-invalid` |
| Card | `bg-card rounded-xl ring-1 ring-foreground/10 p-4`；hover 自动上浮 1px（≥641px）；设置页分区用 `TitledCard` |
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
- 页面壳 `SectionPageLayout` 三段式：`.Title` / `.Actions` 固定，`.Content` 唯一滚动区，分页走 `PageFooterPortal`
- 列表页强制 `DataTablePage` + `SectionPageLayout`，sticky 表头 `bg-muted/30 sticky top-0 z-10`，移动端 `MobileCardList`；细则见 `docs/开发规范/list-page-table-spec.md`
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
- 设置页：`TitledCard` 分区 + React Hook Form + Zod；全站个性化：`ConfigDrawer`（写入 `data-theme-*`）

## 8. 禁止

- 硬编码颜色；mock 数据 / 假分页 / 静默吞错；整页刷新替代 `invalidateQueries`
- 引入第二套 UI 库；绕过 `cn()` 手拼动态类名；新增未维护语言（仅 zh / en）
- 移除焦点环；图标按钮缺 `aria-label`；用颜色作为唯一信息载体

## 9. 维护

- 写数值和类名，不写"美观 / 合理"；单条 ≤2 行；形容词必附落地规则；只写终态
- 改令牌 → `theme.css` + 本文 §2；改 preset → `lib/theme-customization.ts` + `theme-presets.css` + i18n `common.json`；改动效 → `lib/motion.ts`
- 改组件 → `src/components/ui/*.tsx` + 本文 §4；改布局常量 → 对应布局组件 + 本文 §5
- 提交前：亮暗双模目检 · 移动端 <640px · 列表页走 `DataTablePage` · 三态齐备 · `typecheck` / `lint` / `build` 通过
