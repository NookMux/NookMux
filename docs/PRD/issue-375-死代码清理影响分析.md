# Issue #375「fix: 死代码清理」影响分析报告

- **Issue**: [NookMux/NookMux#375](https://github.com/NookMux/NookMux/issues/375)（OPEN，标题：fix: 死代码清理）
- **分析日期**: 2026-09-28
- **分析性质**: 仅分析留痕，未落地任何代码修改（分析过程中的实证删除测试已全部还原）

---

## 一、Issue 内容与属实性结论

Issue 正文原文：

> knip 还报告了约 100 个未被引用的源文件（如 src/features/subscriptions/ 整个目录、部分 ai-elements 组件）和数百个未使用的导出

在 `web/` 目录实测运行 `bun run knip`（配置见 `web/knip.config.ts`，已 ignore `src/components/ui/**` 与 `src/routeTree.gen.ts`），结果：

| knip 报告项 | 实测数量 | 与 issue 描述对照 |
|---|---|---|
| 未引用文件（files） | **100** | ✅ 「约 100 个」属实 |
| 未使用导出（exports） | **361** | ✅ 「数百个」属实 |
| 未使用类型（types） | 131 | 属实（issue 未单列） |
| 其他细项 | `@tanstack/table-core` 未使用依赖 1 项；`hast` 未声明依赖 1 项；`useDialogState` 重复导出 1 项 | 补充发现 |

**结论 1：issue 所述数字与 knip 报告完全属实。** issue 中点名的两个例子均在清单内：`src/features/subscriptions/`（17 个文件）、ai-elements 组件（11 个）。

---

## 二、分析方法

1. 运行 knip 取得 100 个文件的精确清单（JSON 报告落盘 `/tmp/knip-report.json`）。
2. 按功能域拆分为 **8 个互不重叠的批次**，以最高 4 并发的 Sonnet 子代理循环调度核查（两波：4+4）。
3. 每个文件统一核查维度：模块说明符引用（`@/` 别名 + 相对路径双形式）、导出符号名引用（含同名甄别）、动态 `import()`/`lazy()`、字符串注册表/路由路径引用、re-export barrel 链、i18n key 消费、测试运行器（`bun test`）消费、构建入口（`rsbuild.config.ts`/`index.html`）、后端 `internal/` 与 `docs/` 交叉引用。
4. 主会话对子代理关键论断逐项抽查复核（use-mobile 双胞胎、static-keys 零消费、subscriptions.json 存活 key、user-binding-dialog 孤儿链、empty-state 同名甄别、billing-expr 等价实现、models 分区注册表、home barrel 导出），全部属实。
5. **实证验证**：临时删除全部 100 个文件后运行前端门禁，再精确还原（详见第五节）。

---

## 三、逐文件裁决汇总

裁决口径：**A** = 确认死代码，删除安全；**B** = 误报（存在真实引用，删除破坏功能）；**C** = 不破坏构建但建议保留；**D** = 删除安全但需连带清理。

| 批次 | 文件数 | A | C | D | B |
|---|---|---|---|---|---|
| subscriptions 生态（含 wallet/subscription-plans-card） | 18 | 18 | 0 | 0 | 0 |
| ai-elements（11）+ assets/clerk logo（2） | 13 | 13 | 0 | 0 | 0 |
| models deployments（11）+ channels codex/upstream（7） | 18 | 16 | 2（测试） | 0 | 0 |
| home 旧版首页（8）+ layout 营销组件（5） | 13 | 13 | 0 | 0 | 0 |
| 杂项通用组件（coming-soon 等 6） | 6 | 6 | 0 | 0 | 0 |
| system-settings（11） | 11 | 11 | 0 | 0 | 0 |
| 其余 features 杂项（barrel/类型/组件/测试共 12） | 12 | 9 | 2（测试） | 1 | 0 |
| 工具脚本与 hooks/测试（9） | 9 | 6 | 3 | 0 | 0 |
| **合计** | **100** | **92** | **7** | **1** | **0** |

> 最终建议：**93 个文件可安全删除**（92 个 A + 1 个 D 需按连带清单同步清理）；**7 个文件建议保留**（6 个测试文件 + `static-keys.ts`）。knip 的「未引用文件」报告在代码引用层面**零误报**（0 个 B），但对测试文件与约定文件的处置需要人工裁决。

---

## 四、「一刀切删除」风险评估（核心结论）

### 4.1 编译层：安全（已实证）

删除全部 100 个文件后 `bun run typecheck`（tsc -b）与 `bun run build`（rsbuild + 资产压缩）均通过（详见第五节）。静态依赖链上无悬空导入。

### 4.2 运行时层：8 批次均未发现动态加载/字符串注册引用

全部 100 个文件核查了动态 `import()`、`lazy()`、`import.meta.glob`、组件注册表、路由字符串等隐式引用形式，**零命中**。不存在「编译通过但运行时炸」的动态加载风险。

### 4.3 但「一刀切」在以下 6 处会误伤，必须剔除或连带处理

#### 误伤点 1（最严重）：6 个测试文件 —— 会砍掉 86% 的前端测试套件

`web/` 全仓共 7 个测试文件，其中 **6 个被 knip 列为「未引用文件」**（仅 `components/ui/dropdown-menu.test.tsx` 因 knip ignore 幸免）：

| 测试文件 | 被测对象状态 | 防护价值 |
|---|---|---|
| `src/lib/currency.test.ts` | `lib/currency.ts` 被 20+ 生产文件使用（`lib/format.ts:25` re-export，dashboard/pricing/channels/users/keys/wallet 等广泛 import） | 金额格式化（K/M/B 截断、CNY 换算、负号位置）回归防护 |
| `src/features/usage-logs/lib/filter.test.ts` | `buildSearchParams` 在 `common-logs-filter-bar.tsx:136,162`、`task-logs-filter-bar.tsx:120,143` 生产使用 | 回归 #310（清空筛选不得丢 pageSize）专项防护 |
| `src/features/wallet/lib/ui.test.tsx` | `getPaymentIcon` 在 `recharge-form-card.tsx:40,327,395`、`payment-confirm-dialog.tsx:34,132` 使用 | **安全防护**：http→https 图标升级、拒绝 `javascript:` 与带凭据 URL |
| `src/features/channels/lib/channel-form.test.ts` | `channel-form.ts` 经 `channels/lib/index.ts:21` re-export，活代码 | 渠道表单校验回归 |
| `src/features/channels/lib/multi-key-utils.test.ts` | `multi-key-utils.ts` 经 `channels/lib/index.ts:24` re-export，活代码 | 多密钥工具回归 |
| `src/components/data-table/use-persistent-column-visibility.test.ts` | 被 `channels-table.tsx:79`、`usage-logs-table.tsx:76`、`api-keys-table.tsx:177` 使用 | localStorage 脏数据防御回归 |

CI 证据：`.github/workflows/ci.yml:51` 与 `scripts/ci-check.sh:197` 均执行 `bun test`，测试文件会被真实运行。knip 报告它们是 **knip 配置缺口**（未配置测试入口），不是死代码。实证删除后 `bun test` 仅剩 1 个文件 2 个用例。
**处置建议**：保留 6 个测试文件，并在 `web/knip.config.ts` 增加 `entry: ['**/*.test.{ts,tsx}']`（或等效 ignore）消除此类误报。

#### 误伤点 2：`src/i18n/static-keys.ts` —— 仓库书面约定的登记表

代码侧零消费（`STATIC_I18N_KEYS` 全仓仅定义处命中，`sync-i18n.mjs` 也不读取它），**但**：
- `web/AGENTS.md:61` 明文约定「动态 Key 必须登记在 `src/i18n/static-keys.ts` 中」；
- git 历史显示仍在活跃维护（d22217c26 等 5+ 提交）；
- 内容与 locale 字典实际键一一对应。

**处置建议**：保留，并加入 `web/knip.config.ts` 的 `ignore` 降噪。

#### 误伤点 3：ai-elements 目录不可「整目录删除」

issue 的表述「部分 ai-elements 组件」容易诱导整目录删除，但该目录 22 个文件恰好**一半在用**：

- **在用 11 个**（不可删）：`response`、`code-block`、`message`、`conversation`、`prompt-input`、`reasoning`、`loader`、`branch`、`shimmer`、`sources`、`suggestion` —— 由 playground 消息渲染链路（`playground-chat.tsx`、`playground-input.tsx`）与 `pricing/model-details-api.tsx` 引用；`docs/PRD/差异性/安全性.md:47,51,171` 还以其行号为安全审计证据。
- **死代码 11 个**（可删）：`actions`、`artifact`、`chain-of-thought`、`confirmation`、`image`、`open-in-chat`、`plan`、`queue`、`task`、`tool`、`web-preview` —— 模块说明符与导出符号全仓零命中，无 barrel（目录无 index.ts），无动态引用。目录内引用是单向的（死文件引活文件），删除不产生新悬空。

#### 误伤点 4：`subscriptions.json` 字典不可随 feature 一并删除 —— 11 个 key 仍被存活文件消费

subscriptions feature 整体确认为**后端未实现的孤儿功能**（`internal/` 无任何 `/api/subscription/*` 路由；后端注释明确「Subscription billing is removed」—— `internal/domain/billing/funding_source.go:8`；`api.ts` 调用的 stripe/creem/epay 等支付端点全部不存在），18 个文件可删。**但** `src/i18n/locales/{zh,en}/subscriptions.json` 中 11 个 key 被 feature 外存活文件消费，整文件删除会静默丢失运行时翻译：

| 存活 key | 消费方（示例） |
|---|---|
| `subscriptions.status.active` | `components/status-badge.tsx:245`（共享 statusPresets） |
| `subscriptions.fields.plan` | `usage-logs/components/dialogs/details-dialog.tsx:1508` |
| `subscriptions.status.paymentRequestFailed` / `subscriptions.status.redirectingToPaymentPage` | wallet 4 个支付 hook（`use-payment.ts`、`use-creem-payment.ts`、`use-waffo-payment.ts`、`use-waffo-pancake-payment.ts`） |
| `subscriptions.status.requestFailed` | `system-settings/general/channel-affinity/cache-stats-dialog.tsx:71,75` |
| `subscriptions.fields.source` | `system-settings/maintenance/database-maintenance-section.tsx:517` |
| `subscriptions.actions.start` / `subscriptions.fields.end` | `system-settings/maintenance/log-settings-section.tsx:368,378` |
| `subscriptions.fields.minutes` | `system-settings/request-limits/rate-limit-section.tsx:176` |
| `subscriptions.actions.deleted` | `users/constants.ts:63`、`users/components/dialogs/user-ban-dialog.tsx:250` |
| `subscriptions.actions.resetPeriod` | `keys/components/api-keys-cells.tsx:182` |

**处置建议**：删 feature 时保留（或先迁移）该字典中上述 key，仅移除 `i18n/config.ts:50,78` 等处的注册与 feature 专属 key。

#### 误伤点 5：`user-binding-dialog.tsx`（唯一 D 类）需连带清理

对话框本身零引用、后端端点已随 `user_oauth_bindings` 表清理移除（`internal/store/db/cleanup/cleanup_removed_oauth.go:41-43` 直接 DROP 该表），可删；但其独占依赖 `users/api.ts:182-221`（`OAuthBinding` 接口、`getUserOAuthBindings`、`adminClearUserBinding`、`adminUnbindCustomOAuth`）会成为新孤儿，应一并删除。**注意**：与在用的自助绑定（`profile/api.ts` 的 `CustomOAuthBinding`/`getSelfOAuthBindings`）无关，勿误删。

#### 误伤点 6：`use-mobile.tsx` 是 `use-mobile.ts` 的字节级副本

`cmp` 验证两文件完全相同（1306 字节）。全部 7 处 import 经模块解析命中 `.ts`（解析顺序 .ts 优先），`.tsx` 被完全遮蔽。删除 `.tsx` 安全且**消除分叉隐患**（防止未来只改其一导致行为分叉）。

### 4.4 三组「孤儿功能」的定性（删除无功能损失）

| 孤儿功能 | 文件数 | 后端实现 | 定性 |
|---|---|---|---|
| subscriptions 套餐管理 | 17+1（wallet 卡片） | 无任何端点；后端已显式移除订阅计费 | 参考项目遗留的前端孤儿实现 |
| models deployments 部署管理 | 11 | 无 `/api/deployments/*`；`/models/deployments` 访问会被 `beforeLoad` 重定向到 `/models/metadata`（分区注册表仅含 `metadata`） | 参考项目遗留 |
| channels codex OAuth / upstream-update | 5 | 无 `/api/channel/codex/*`、`/api/channel/upstream_updates/*`（internal/ 中 codex/upstream 命中均为 relay 协议词汇） | 参考项目遗留，运行时必然 404 |

home 旧版首页 8 文件与 layout 营销组件 5 个为「经典 UI → 默认 UI」迁移（f928f9066）及首页重设计（39e5a5306）后的遗留，新版 `sections/` 首页在用；tier-expr、model-details-capabilities、common-logs-header-actions 等可证明被内联等价实现取代（现役实现分别在 `billing-expr.ts`、`model-details.tsx:83-158`、`common-logs-filter-bar.tsx`）。

### 4.5 建议的连带清理清单（删文件之外）

- `web/src/routes/_authenticated/subscriptions/index.tsx`：纯 redirect 跳板，feature 删除后成空壳，一并删（routeTree 构建时自动再生成）。
- `web/src/i18n/config.ts:50,78` 等处 subscriptions 字典 import/注册。
- `web/src/i18n/static-keys.ts` 中 72 条 `subscriptions.*` 条目、codex 相关条目（:502、:679-681）。
- `web/src/features/models/api.ts:297` 起 deployment 系列导出（含 `testDeploymentConnectionWithKey:323`）、`constants.ts:105`、`lib/query-keys.ts:56`、`types.ts` 的 `Deployment` 类型、`models.json`（zh/en）deployment 系列 key。
- `web/src/features/channels/api.ts:303-330` 三个 codex 函数、`channels.json`（zh/en :205-206 等）codex key。
- `web/src/features/channels/components/channels-columns.tsx:536` 指向 `/models/deployments` 的死导航入口（今日已被分区校验重定向，本就是坏链）。
- `web/src/features/system-settings/api.ts` 的 `getOptionJsonArray`/`upsertOptionJsonArrayEntry`/`deleteOptionJsonArrayEntry`（注意 `getSystemOptionValue` 仍被 `channels/model-mapping-editor.tsx:39` 使用，须保留）。
- `web/src/features/users/api.ts:182-221` 三函数一接口（见误伤点 5）。
- `web/knip.config.ts`：加测试入口与 `static-keys.ts` ignore。

### 4.6 knip 其余细项（issue 的「数百个未使用导出」）

361 个未使用导出与 131 个未使用类型**分布大量落在本次已判死的 92+1 个文件内**，随文件删除自然消解；存活文件内的剩余项建议作为二期单独批次处理（本次未逐项展开）。另建议处理：`@tanstack/table-core` 未使用直接依赖（经 `@tanstack/react-table` re-export 覆盖）、`hast` 未声明依赖（类型引用）、`useDialogState` 重复导出（同文件 :97/:186）。

---

## 五、实证验证记录（已全部还原）

| 步骤 | 结果 |
|---|---|
| 删除全部 100 个清单文件（`/tmp/knip-unused-files-only.txt`） | 成功删除 100，缺失 0 |
| `bun run typecheck`（tsc -b） | ✅ exit 0 |
| `bun run build`（rsbuild + 资产压缩，196 assets） | ✅ exit 0 |
| `bun test` | ✅ exit 0，但仅剩 1 文件 2 用例（佐证误伤点 1） |
| git 还原（逐文件 `git checkout`，不触碰用户脏改） | ✅ 工作区恢复至初始状态 |

协作过程：8 个批次 × Sonnet 子代理（两波，每波 ≤4 并行），全部只读核查、批次间文件零重叠；主会话完成抽查复核与实证测试。

---

## 六、最终结论

1. **Issue 属实**：knip 报告的 100 个未引用文件、361 个未使用导出、131 个未使用类型与实测完全一致。
2. **knip 零误报但有盲区**：100 个文件在代码引用层面全部属实（0 个 B 类）；盲区在于测试文件（6 个，CI 实跑）与约定文件（static-keys.ts）需要人工裁决，而非删除。
3. **「一刀切」可行性**：编译与运行时层面安全（实证 typecheck/build 通过，无动态加载/字符串引用）；但直接全删会误伤测试套件（-86%）、违反 i18n 约定、并在「整目录删 ai-elements / 连字典删 subscriptions.json」的错误扩大化操作下破坏 playground 与 11 处存活翻译。
4. **建议方案**：删除 93 个文件（92 A + 1 D），保留 7 个（6 测试 + static-keys.ts），按 4.5 连带清单同步清理，并修正 knip 配置（测试入口 + static-keys ignore）。删除后重跑 `./scripts/ci-check.sh --frontend` 与 `bun run i18n:sync` 收口。

## 附：100 个文件完整清单与裁决

> 格式：`文件 | 裁决`（A=确认死代码；C=建议保留；D=可删需连带）

```
scripts/split-i18n.mjs | A（一次性迁移脚本，任务已完成）
src/assets/clerk-full-logo.tsx | A
src/assets/clerk-logo.tsx | A
src/components/ai-elements/actions.tsx | A
src/components/ai-elements/artifact.tsx | A
src/components/ai-elements/chain-of-thought.tsx | A
src/components/ai-elements/confirmation.tsx | A
src/components/ai-elements/image.tsx | A
src/components/ai-elements/open-in-chat.tsx | A
src/components/ai-elements/plan.tsx | A
src/components/ai-elements/queue.tsx | A
src/components/ai-elements/task.tsx | A
src/components/ai-elements/tool.tsx | A
src/components/ai-elements/web-preview.tsx | A
src/components/coming-soon.tsx | A
src/components/data-table/use-persistent-column-visibility.test.ts | C（CI 实跑，回归防护）
src/components/date-picker.tsx | A
src/components/empty-state.tsx | A（pricing 有独立同名实现，勿混淆）
src/components/layout/components/glow.tsx | A
src/components/layout/components/logo.tsx | A（在用 Logo 在 @/assets/logo）
src/components/layout/components/mockup.tsx | A
src/components/layout/components/navbar.tsx | A
src/components/layout/components/section.tsx | A（在用为 SectionPageLayout）
src/components/learn-more.tsx | A
src/components/risk-acknowledgement-dialog.tsx | A
src/components/theme-quick-switcher.tsx | A
src/features/auth/index.ts | A（barrel，消费方全部深路径直引）
src/features/channels/components/dialogs/codex-oauth-dialog.tsx | A（后端无端点）
src/features/channels/components/dialogs/codex-usage-dialog.tsx | A
src/features/channels/components/dialogs/upstream-update-dialog.tsx | A
src/features/channels/hooks/use-channel-upstream-updates.ts | A（后端无端点）
src/features/channels/lib/channel-form.test.ts | C（CI 实跑，回归防护）
src/features/channels/lib/multi-key-utils.test.ts | C（CI 实跑，回归防护）
src/features/channels/lib/upstream-update-utils.ts | A
src/features/home/components/connection-line.tsx | A（旧版首页遗留）
src/features/home/components/feature-item.tsx | A
src/features/home/components/gateway-card.tsx | A
src/features/home/components/hero-buttons.tsx | A
src/features/home/components/icon-card.tsx | A
src/features/home/components/scrolling-icons.tsx | A
src/features/home/constants.ts | A
src/features/home/lib/icon-mapper.tsx | A
src/features/models/components/deployment-access-guard.tsx | A（后端无端点）
src/features/models/components/deployments-columns.tsx | A
src/features/models/components/deployments-table.tsx | A
src/features/models/components/dialogs/create-deployment-drawer.tsx | A
src/features/models/components/dialogs/extend-deployment-dialog.tsx | A
src/features/models/components/dialogs/rename-deployment-dialog.tsx | A
src/features/models/components/dialogs/update-config-dialog.tsx | A
src/features/models/components/dialogs/view-details-dialog.tsx | A
src/features/models/components/dialogs/view-logs-dialog.tsx | A
src/features/models/hooks/use-model-deployment-settings.ts | A
src/features/models/lib/deployments-utils.ts | A
src/features/performance-metrics/types.ts | A
src/features/pricing/components/model-details-capabilities.tsx | A（已被内联实现取代）
src/features/pricing/hooks/index.ts | A（barrel）
src/features/pricing/lib/index.ts | A（barrel）
src/features/pricing/lib/tier-expr.ts | A（现役等价实现在 billing-expr.ts）
src/features/rankings/lib/index.ts | A（barrel）
src/features/subscriptions/api.ts | A（后端无端点）
src/features/subscriptions/components/data-table-row-actions.tsx | A
src/features/subscriptions/components/dialogs/subscription-purchase-dialog.tsx | A
src/features/subscriptions/components/dialogs/toggle-status-dialog.tsx | A
src/features/subscriptions/components/dialogs/user-subscriptions-dialog.tsx | A
src/features/subscriptions/components/subscriptions-columns.tsx | A
src/features/subscriptions/components/subscriptions-dialogs.tsx | A
src/features/subscriptions/components/subscriptions-mutate-drawer.tsx | A
src/features/subscriptions/components/subscriptions-primary-buttons.tsx | A
src/features/subscriptions/components/subscriptions-provider.tsx | A
src/features/subscriptions/components/subscriptions-table.tsx | A
src/features/subscriptions/constants.ts | A
src/features/subscriptions/index.tsx | A
src/features/subscriptions/lib/format.ts | A
src/features/subscriptions/lib/index.ts | A
src/features/subscriptions/lib/plan-form.ts | A
src/features/subscriptions/types.ts | A
src/features/system-settings/components/json-array-editor.tsx | A
src/features/system-settings/components/settings-accordion.tsx | A
src/features/system-settings/components/settings-card.tsx | A
src/features/system-settings/content/chat-dialog.tsx | A
src/features/system-settings/content/json-toggle-section.tsx | A
src/features/system-settings/content/utils.ts | A
src/features/system-settings/dashboard/section-registry.tsx | A（与 features/dashboard 同名文件无关）
src/features/system-settings/hooks/use-accordion-state.ts | A
src/features/system-settings/hooks/use-form-dirty-guard.ts | A（自标 @deprecated，替代品在用）
src/features/system-settings/hooks/use-safe-json-state.ts | A
src/features/system-settings/utils/route-config.ts | A（从未被采用的早期抽象）
src/features/usage-logs/components/common-logs-header-actions.tsx | A（已被内联实现取代）
src/features/usage-logs/lib/filter.test.ts | C（CI 实跑，#310 回归防护）
src/features/usage-logs/lib/index.ts | A（barrel）
src/features/users/components/dialogs/user-binding-dialog.tsx | D（连带清理 users/api.ts:182-221）
src/features/wallet/components/subscription-plans-card.tsx | A（唯一引用 subscriptions 的死文件）
src/features/wallet/lib/ui.test.tsx | C（CI 实跑，含安全防护断言）
src/hooks/use-hidden-click-unlock.ts | A
src/hooks/use-minimum-loading-time.ts | A
src/hooks/use-mobile.tsx | A（.ts 的字节级副本，删除消除遮蔽隐患）
src/hooks/use-table-compact-mode.ts | A
src/i18n/static-keys.ts | C（AGENTS.md 约定登记表，建议加 knip ignore）
src/lib/currency.test.ts | C（CI 实跑，核心格式化回归防护）
src/lib/show-submitted-data.tsx | A（shadcn 示例残留）
```

统计校验：A 92 + C 7 + D 1 = 100 ✅
