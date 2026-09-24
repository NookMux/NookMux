# AGENTS.md

本文件是仓库级统一入口。按 `docs/AGENTS.md完全使用指南.md` 的约定，子目录中更近的
`AGENTS.md` 会补充或覆盖这里的规则；用户在对话中的明确要求优先级最高。

## 特殊工具提示

本项目配置了 Serena与 CodeGraph两个 MCP，与原生工具按任务形状分工，不要无条件只用某一个：

- 跨文件架构、功能入口、调用链、依赖关系、改动影响面，且当前项目已初始化
  CodeGraph 时，优先使用 CodeGraph 获取全局上下文。
- 已知具体符号后的定义查看、引用查找、精确修改、重命名，使用 Serena/LSP。
- 精确文本搜索（错误信息、日志字符串、配置键）、整文件阅读、行级编辑、
  命令执行，使用原生工具。
- 索引结果（CodeGraph 图数据、调用计数）与实际源码不一致时，以当前源码和
  可执行验证结果为准；逐点精确的引用清单以 Serena（LSP references）为准。
- Serena 返回的行号是 0-based，向用户报告时 +1。

## ⚠ 必读：分层规则

**修改某个包/目录下的代码前，必须先阅读该目录下的 `AGENTS.md`。** 根文件只包含
全局规则和概览，每个子目录的 `AGENTS.md` 包含该包特有的约定、模式和检查清单。
跳过子目录规则会导致违反项目约定（如遗漏审计埋点、数据库兼容性问题、前端 i18n 缺失等）。

阅读顺序：根 `AGENTS.md` → 目标目录的 `AGENTS.md` → 如有更深层级继续向下。

涉及跨包改动时，阅读所有受影响包的 `AGENTS.md`。例如改 controller 调用计费
的逻辑时，同时阅读 `internal/httpapi/controller/AGENTS.md`、`internal/domain/AGENTS.md`
和 `internal/domain/billing/` 相关子包规则。

## 架构层级与子规则索引

修改某个包/目录下的代码前，必须先阅读该目录下的 `AGENTS.md`。各目录特有约定按渐进式披露组织：

### 启动与装配
- [cmd/](cmd/AGENTS.md) / [cmd/server/](cmd/server/AGENTS.md)：进程启动入口，仅处理系统退出码并调用 `internal/app.Run()`。
- [internal/app/](internal/app/AGENTS.md)：应用启动装配层，负责初始化环境、依赖注入、后台任务装配、HTTP 服务生命周期及前端嵌入资产门面。

### HTTP 边界层
- [internal/httpapi/controller/](internal/httpapi/controller/AGENTS.md)：按资源垂直拆分的 HTTP 控制器。
- [internal/httpapi/router/](internal/httpapi/router/AGENTS.md) 与 [internal/httpapi/middleware/](internal/httpapi/middleware/AGENTS.md)：路由组装、鉴权、限流与上下文注入中间件。

### 核心领域层
- [internal/domain/](internal/domain/AGENTS.md)：领域服务总入口与公共业务契约。
- [internal/domain/billing/](internal/domain/billing/AGENTS.md)：计费核算、配额冻结/扣减与额度校验。
- [internal/domain/audit/](internal/domain/audit/AGENTS.md)：系统管理员资源操作审计埋点。

### 数据与基础设施层
- [internal/store/](internal/store/AGENTS.md)：数据持久层，基于 GORM 的多资源存储实现，兼容 SQLite / MySQL / PostgreSQL 三库。
- [internal/config/](internal/config/AGENTS.md)：系统、运营、模型、倍率与性能配置的集中注册与动态管理。
- [internal/infra/](internal/infra/AGENTS.md)：代理 HTTP 客户端、Redis/缓存、安全校验、业务日志、媒体解析与 Token 计数。
- [internal/relay/](internal/relay/AGENTS.md)（含 [channel/](internal/relay/channel/AGENTS.md)）：AI 请求中继与协议转换核心，多模态 Adaptor 调度、OpenAI wire 双向转换与流式改写。
- [internal/oauth/](internal/oauth/AGENTS.md)：第三方 OAuth 登录认证服务商扩展层。
- [internal/common/](internal/common/AGENTS.md)：跨层业务全局变量、上下文键（`ContextKey`）注册表及基础纯工具内核。
- [internal/i18n/](internal/i18n/AGENTS.md)：后端 API 响应消息国际化。
- [pkg/](pkg/AGENTS.md)：无业务依赖、可独立复用的底层库（`jsonx`、`cachex`）。

### 前端与文档
- [web/](web/AGENTS.md)：前端单页应用（React 19 + TypeScript + Rsbuild + Tailwind CSS 4）。
- [DESIGN.md](DESIGN.md)：前端外观与感受的唯一设计规范（调性、颜色、字体、组件、布局、动效），修改任何 UI 前必读。
- [docs/](docs/AGENTS.md)：跨模块详细规范、系统设计与开发参考文档。

`参考项目/` 是本地参考源码，已被忽略；除非用户明确要求，不要修改其中内容。

## 项目概览

这是基于 Go 与 React (Bun) 构建的企业级高性能 AI API 网关和分发管理平台。系统聚合 OpenAI、Claude、Gemini、Azure、AWS Bedrock 等多上游供应商，提供令牌、渠道、计费、模型重定向、限速、审计与控制台管理能力。

### 核心运行时与包管理器

- **后端**：Go（版本以 `go.mod` 为准），使用标准 Go 工具链。
- **前端**：Bun（工作目录 `web/`），严格使用 `bun` 作为唯一包管理器，禁止混用 npm/pnpm/yarn。

## 全局工作规则

- 先建立证据链再改代码：现象、入口、相关代码/配置、根因层级、最小修复点、验证方式。
- 终态干净交付：代码修改、注释、文档与提交信息只呈现最终目标设计，严禁残留"之前写错了/多加了逻辑，在此处删掉"等历史纠错痕迹与自我辩解（详见下文反模式警示）。
- 保持工作区脏改隔离。不要回滚、覆盖或格式化与当前任务无关的用户改动。
- 不做破坏性 Git 操作，不自动 commit/push；需要提交时只 add 相关具体文件。
- 严禁用 sed、awk、正则脚本或自动化批处理脚本盲改源码，一律使用行级精准编辑工具，防止隐式破坏。
- 禁止在命令行中内联（inline）拼接超长 Bash 或多行复杂脚本；复杂诊断、验证或工具脚本须先写入工作区临时文件再执行。
- 成熟数据格式（JSON、YAML、Markdown、HTML 等）解析必须使用标准库或成熟生态库，严禁手动通过正则或切片自造简易 parser。
- 任务执行必须完整闭环：实施、运行、测试并迭代直至正确可用，严禁初步改完代码就停下并转嫁给用户测试。
- 严禁未经本地 CI 等价门禁验证直接提交。提交时由 `.githooks/pre-commit`（秒级增量）、推送时由 `.githooks/pre-push`（全量，对齐 CI）自动把关，如遇拦截必须立刻定位并修复，严禁使用 `--no-verify` 绕过。
- 不写入 secrets。环境变量、数据库 DSN、OAuth 密钥、API key 都不得硬编码到源码或文档示例的真实值。
- 不用模拟成功、静默降级、吞错或假数据让流程"看起来能跑"。失败必须清晰暴露。
- 外部输入必须在系统边界校验：HTTP 参数、表单、文件、网络、数据库、缓存、权限、安全逻辑。
- 新增通用能力前先搜索现有工具函数；确有复用价值再放入 `internal/common/` 或对应前端 `lib/`。
- 不要顺手删除、替换或改名项目标识、AGPL/版权头、Go module path、Docker/CI 镜像名等元数据。

### 行为反模式警示：拒绝纠错残留

严厉禁止以下“纠偏后残留历史痕迹与自我辩解”的行为模式：

> 用户让做一盘“番茄炒蛋”，Agent 擅自加了“东坡肉”；被指出后虽然去掉了，但提交/PR 时写着「番茄炒蛋（无东坡肉）」，并在注释中大篇幅解释为什么本道菜不需要加东坡肉。

**交付要求**：任何代码、注释、文档、提交信息及回复，都必须直接呈现对齐后的**干净终态设计**（Clean final-state design），严禁包含任何前序错误、自我辩解或“在此删掉某逻辑”的纠错痕迹。

## 后端规则

- Go 版本以 `go.mod` 为准。
- JSON 序列化/反序列化调用使用 `pkg/jsonx` 的包装函数（`jsonx.Marshal` / `jsonx.Unmarshal` /
  `jsonx.UnmarshalJsonStr` / `jsonx.DecodeJson`）；不要在业务代码里直接调用
  `encoding/json` 的 marshal/unmarshal/decode。
- 数据库必须兼容 SQLite、MySQL >= 5.7.8、PostgreSQL >= 9.6。优先 GORM；原始 SQL 必须参数化并处理三库差异。
- 渠道相关的外网请求（中继、测试、模型拉取、余额/套餐查询、WebSocket 等）必须走该渠道配置的代理（`httpclient.NewProxyHttpClient` / `NewProxyWebSocketDialer`，包路径 `internal/infra/httpclient`）。
- 待机内存相关默认值必须保守：连接池 idle 上限、prepared statement 缓存、后台 worker/goroutine 池、
  ticker 唤醒频率等常驻资源不能为追求峰值吞吐随意调大。确需调大时必须保留环境变量覆盖、同步
  `.env.example` 和中英文环境变量文档，并说明低流量/待机场景的内存影响。
- 路由层不要承载业务逻辑；控制器只做边界处理；领域层（`internal/domain/`）承载业务；存储层（`internal/store/`）承载持久化。
- relay 改动要保护流式输出、usage 统计、错误映射、计费和供应商协议差异。
- relay 请求 DTO 中需要转发给上游的可选标量字段，优先用指针类型配合 `omitempty`，保留客户端显式传入的 `0`、`0.0`、`false`。
- 后端 API 响应消息的多语言翻译遵守 [internal/i18n/AGENTS.md](internal/i18n/AGENTS.md)：用户可见提示走 `i18n.Msg*` 常量，不要硬编码中英文字符串。

### 审计日志

管理员对系统资源（渠道、用户、令牌、系统设置等）的增删改操作必须接入审计日志。
通过 `audit.RecordAudit(...)`（包路径 `internal/domain/audit`）记录，详见 `internal/httpapi/controller/AGENTS.md`
和 `internal/domain/audit/AGENTS.md`。
新增需要审计的资源类型时，按 `internal/httpapi/controller/AGENTS.md` 中的检查清单同步更新 store（audit 常量）、
config、前端常量和 i18n。

## 本地 CI 门禁检查

提交代码前必须保证本地门禁全绿（严禁使用 `--no-verify` 绕过）。优先使用项目封装的一键脚本复核（已集成 fmt / tidy / vet / lint / test -race / build 及 Web 校验）：

- **一键复核脚本**：
  - `./scripts/ci-check.sh`：全量复核（对齐 ci.yml，由 pre-push hook 自动执行）。
  - `./scripts/ci-check.sh --staged`：增量复核（仅针对暂存区秒级校验，由 pre-commit hook 自动执行）。
  - `./scripts/ci-check.sh --backend` / `--frontend`：按端定向复核后端或前端门禁。
- **Git Hook 与提交闭环**：
  - 执行 `git config core.hooksPath .githooks` 激活本地拦截；推送后若 Actions 失败须当场修复，不留坏提交。


## 前端规则

- 前端包管理器使用 Bun。`web/` 目录有独立 `package.json` 和 `bun.lock`。
- 改 `web/` 后按影响执行 `bun run typecheck`、`bun run lint`、`bun run build`，适度使用knip。
- 不允许用 mock 数据替代真实后端能力。
- 列表/表格类页面必须使用 `DataTablePage` + `SectionPageLayout`，不得手拼 `Table`
  或用 `Card` 包裹表格。详见 [web/AGENTS.md](web/AGENTS.md) 和
  [docs/开发规范/list-page-table-spec.md](docs/开发规范/list-page-table-spec.md)。
- 前端组件优先复用 `src/components/ui/`、`src/components/data-table/`、
  `src/components/layout/` 等通用组件，避免重复造轮子。

## 文档与参考项目

- `参考项目/` 仅用于比对上游实现。复制代码前必须适配本项目 API 和配置。
- 跨模块详细开发规范文档放在 `docs/开发规范/`，根 `AGENTS.md` 和子目录
  `AGENTS.md` 通过链接引用，避免在 AGENTS.md 中堆砌长篇规范正文。
- `docs/AGENTS.md` 中的规则适用于 `docs/` 目录下的所有文档文件。

## AI协助开发声明

凡使用了 AI 辅助生成或修改的代码，必须在 **commit 信息结尾**（以及对应 **PR 描述**中）按以下格式注明所使用的工具环境和模型：

```
assisted-by：{agent_name}：{model}
```

- `{agent_name}`：使用的 AI 编码工具/环境，如 `opencode`、`codex`、`cursor`
- `{model}`：实际使用的模型（含供应商，格式可参考 `供应商/模型`），如 `Zhipu/GLM-5.3[Max]`

示例：

```bash
git commit -m "feat(channel): support batch model pulling

assisted-by：opencode：Zhipu/GLM-5.3[Max]"
```

多个模型/工具参与时逐行列出。纯人工改动无需此声明，但需在 PR 描述中说明。
