# AGENTS.md

本文件是仓库级统一入口。按 https://agents.md/ 的约定，子目录中更近的
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

## 子规则索引

前端:

- [web/AGENTS.md](web/AGENTS.md)

启动与装配:

- [cmd/AGENTS.md](cmd/AGENTS.md)
- [cmd/server/AGENTS.md](cmd/server/AGENTS.md)
- [internal/app/AGENTS.md](internal/app/AGENTS.md)

后端 Go 包:

- [internal/domain/AGENTS.md](internal/domain/AGENTS.md)
- [internal/domain/billing/AGENTS.md](internal/domain/billing/AGENTS.md)
- [internal/domain/audit/AGENTS.md](internal/domain/audit/AGENTS.md)
- [internal/common/AGENTS.md](internal/common/AGENTS.md)
- [internal/infra/AGENTS.md](internal/infra/AGENTS.md)
- [internal/httpapi/router/AGENTS.md](internal/httpapi/router/AGENTS.md)
- [internal/httpapi/controller/AGENTS.md](internal/httpapi/controller/AGENTS.md)
- [internal/httpapi/middleware/AGENTS.md](internal/httpapi/middleware/AGENTS.md)
- [internal/store/AGENTS.md](internal/store/AGENTS.md)
- [internal/config/AGENTS.md](internal/config/AGENTS.md)
- [internal/relay/AGENTS.md](internal/relay/AGENTS.md)
- [internal/relay/channel/AGENTS.md](internal/relay/channel/AGENTS.md)
- [internal/oauth/AGENTS.md](internal/oauth/AGENTS.md)
- [internal/i18n/AGENTS.md](internal/i18n/AGENTS.md)
- [pkg/AGENTS.md](pkg/AGENTS.md)

文档:

- [docs/AGENTS.md](docs/AGENTS.md)

`参考项目/` 是本地参考源码，已被忽略；除非用户明确要求，不要修改其中内容。

## 项目概览

这是基于 Go 与 React (Bun) 构建的企业级高性能 AI API 网关和分发管理平台。系统聚合 OpenAI、Claude、Gemini、Azure、AWS Bedrock 等多上游供应商，提供令牌、渠道、计费、模型重定向、限速、审计与控制台管理能力。

### 核心运行时与包管理器

- **后端**：Go（版本以 `go.mod` 为准），使用标准 Go 工具链。
- **前端**：Bun（工作目录 `web/`），严格使用 `bun` 作为唯一包管理器，禁止混用 npm/pnpm/yarn。

### 主要架构层级

子目录规则按渐进式披露组织，各包特有约定详见对应的 `AGENTS.md`：

- `cmd/server/`：进程启动入口，仅处理系统退出码并调用 `internal/app.Run()`，见 [cmd/server/AGENTS.md](cmd/server/AGENTS.md)。
- `internal/app/`：应用启动装配层，负责初始化环境、依赖注入、后台任务装配、HTTP 服务生命周期及前端嵌入资产门面，见 [internal/app/AGENTS.md](internal/app/AGENTS.md)。
- `internal/httpapi/`：HTTP 边界层，包含路由（`router/`）、中间件（`middleware/`）与按资源垂直拆分的控制器（`controller/`），见 [internal/httpapi/controller/AGENTS.md](internal/httpapi/controller/AGENTS.md)。
- `internal/domain/`：核心领域层，承载计费核算（`billing/`，见 [internal/domain/billing/AGENTS.md](internal/domain/billing/AGENTS.md)）、渠道调度与治理（`channel/`）、审计埋点（`audit/`，见 [internal/domain/audit/AGENTS.md](internal/domain/audit/AGENTS.md)）、敏感词过滤、分组倍率等领域服务及契约，见 [internal/domain/AGENTS.md](internal/domain/AGENTS.md)。
- `internal/store/`：数据持久层，基于 GORM 的多资源存储实现（`dbstore`、`channelstore`、`userstore`、`tokenstore` 等），支持 SQLite / MySQL / PostgreSQL 三库兼容，见 [internal/store/AGENTS.md](internal/store/AGENTS.md)。
- `internal/config/`：配置管理层，负责系统、运营、模型、倍率与性能配置的集中注册与动态管理，见 [internal/config/AGENTS.md](internal/config/AGENTS.md)。
- `internal/common/`：跨层业务全局变量、上下文键（`ContextKey`）注册表及基础纯工具内核，见 [internal/common/AGENTS.md](internal/common/AGENTS.md)。
- `internal/infra/`：基础设施层，提供代理 HTTP 客户端、Redis/缓存、安全校验、运行时监控、业务日志、媒体解析、Token 计数及支付通知，见 [internal/infra/AGENTS.md](internal/infra/AGENTS.md)。
- `internal/relay/`：AI 请求中继与协议转换核心，负责多模态 Adaptor 调度（`channel/`，见 [internal/relay/channel/AGENTS.md](internal/relay/channel/AGENTS.md)）、OpenAI wire 双向转换、流式改写与上游中继，见 [internal/relay/AGENTS.md](internal/relay/AGENTS.md)。
- `internal/oauth/`：第三方 OAuth 登录认证服务商扩展层，见 [internal/oauth/AGENTS.md](internal/oauth/AGENTS.md)。
- `internal/i18n/`：后端 API 响应消息国际化，见 [internal/i18n/AGENTS.md](internal/i18n/AGENTS.md)。
- `pkg/`：无业务依赖、可独立复用的底层库（`jsonx`、`cachex`），见 [pkg/AGENTS.md](pkg/AGENTS.md)。
- `web/`：前端单页应用（React 19 + TypeScript + Rsbuild + Tailwind CSS 4），见 [web/AGENTS.md](web/AGENTS.md)。

## 全局工作规则

- 先建立证据链再改代码：现象、入口、相关代码/配置、根因层级、最小修复点、验证方式。
- 保持工作区脏改隔离。不要回滚、覆盖或格式化与当前任务无关的用户改动。
- 不做破坏性 Git 操作，不自动 commit/push；需要提交时只 add 相关具体文件。
- 严禁未经本地 CI 等价门禁验证直接提交。提交时由 `.githooks/pre-commit`（秒级增量）、推送时由 `.githooks/pre-push`（全量，对齐 CI）自动把关，如遇拦截必须立刻定位并修复，严禁使用 `--no-verify` 绕过。
- 不写入 secrets。环境变量、数据库 DSN、OAuth 密钥、API key 都不得硬编码到源码或文档示例的真实值。
- 不用模拟成功、静默降级、吞错或假数据让流程"看起来能跑"。失败必须清晰暴露。
- 外部输入必须在系统边界校验：HTTP 参数、表单、文件、网络、数据库、缓存、权限、安全逻辑。
- 新增通用能力前先搜索现有工具函数；确有复用价值再放入 `internal/common/` 或对应前端 `lib/`。
- 不要顺手删除、替换或改名项目标识、AGPL/版权头、Go module path、Docker/CI 镜像名等元数据。

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

### 提交前 CI 门禁检查（Pre-Commit Checklist）

提交代码前，必须保证本地门禁全绿（与 `.github/workflows/ci.yml` 严格对齐），严禁未经全绿验证直接 commit：

- **一键复核脚本**：
  - `./scripts/ci-check.sh`：全量复核（对齐 ci.yml：Go fmt / tidy / vet / lint / test -race / build + Web typecheck / lint / format / audit / test；不含 rsbuild 生产构建，该项交给 CI）。由 pre-push hook 自动执行。
  - `./scripts/ci-check.sh --staged`：增量复核（仅针对 Git 暂存区改动，秒级：Go 为暂存文件 gofmt + 全仓 vet，前端按暂存文件做 prettier，TS 相关变更才触发 typecheck）。由 pre-commit hook 自动执行。
  - `./scripts/ci-check.sh --backend`：仅跑后端门禁（fmt / tidy / vet / lint / test -race / build）。
  - `./scripts/ci-check.sh --frontend`：仅跑前端门禁（typecheck / lint / format / audit / test）。
- **Go 必检项**（改动后端代码或根配置时必跑）：
  - `unformatted=$(gofmt -l $(git ls-files '*.go'))`：格式检查（必须 0 违规，若有未对齐文件执行 `gofmt -w <file>`）。
  - `go mod tidy -diff`：保证 `go.mod` / `go.sum` 干净无差异。
  - `go vet ./...`：无编译期静态隐患。
  - `go test -race ./...`（或改动受影响模块带 `-race`）：排查并发数据竞态（测试清理须注意排空异步协程如 `runtime.WaitRelayTasks()`）。
  - `golangci-lint run`：遵循 [.golangci.yml](.golangci.yml) 门禁契约。
  - `go build ./...`：全包构建通过。
- **Web 必检项**（改动 `web/` 目录时必跑）：
  - `cd web && bun run format:check`：Prettier 代码格式干净。
  - `cd web && bun run typecheck`：TypeScript 类型检查通过。
  - `cd web && bun run lint`：ESLint 静态规范通过。
  - `cd web && bun test`：前端单元测试通过。
- **Git Hook 与提交闭环**：
  - 本地 Git Hooks 位于 `.githooks/`，可通过 `git config core.hooksPath .githooks` 激活：`pre-commit` 在 `git commit` 时对暂存区改动执行秒级增量校验（`--staged` 模式）；`pre-push` 在 `git push` 时执行对齐 CI 的全量门禁（Go 与 Web 全套）。
  - 推送后，若具备权限，可通过 `gh run list --limit 1` 或 `gh run watch` 查看 GitHub Actions 流水线状态；如遇失败，须及时读取失败日志并当场修复，不留坏提交。


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
