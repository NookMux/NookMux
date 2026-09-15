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
- [internal/domain/audit/AGENTS.md](internal/domain/audit/AGENTS.md)
- [internal/common/AGENTS.md](internal/common/AGENTS.md)
- [internal/infra/AGENTS.md](internal/infra/AGENTS.md)
- [internal/httpapi/router/AGENTS.md](internal/httpapi/router/AGENTS.md)
- [internal/httpapi/controller/AGENTS.md](internal/httpapi/controller/AGENTS.md)
- [internal/httpapi/middleware/AGENTS.md](internal/httpapi/middleware/AGENTS.md)
- [internal/store/AGENTS.md](internal/store/AGENTS.md)
- [internal/config/AGENTS.md](internal/config/AGENTS.md)
- [internal/relay/AGENTS.md](internal/relay/AGENTS.md)
- [internal/i18n/AGENTS.md](internal/i18n/AGENTS.md)
- [pkg/AGENTS.md](pkg/AGENTS.md)

文档:

- [docs/AGENTS.md](docs/AGENTS.md)

`参考项目/` 是本地参考源码，已被忽略；除非用户明确要求，不要修改其中内容。

## 项目概览

这是 Go 实现的 AI API 网关和管理后台。后端聚合 OpenAI、Claude、Gemini、
Azure、AWS Bedrock 等上游能力，提供用户、渠道、计费、限速、认证和管理接口。

主要结构:

- `cmd/server/`: 进程入口，只处理退出码并调用 `internal/app.Run()`。
- `internal/app/`: 启动资源初始化、Gin 装配、路由挂载和分析脚本注入；`env.go` 承载启动
  flag 与 `InitEnv` 环境装配（阶段 4 自 `common/init.go` 迁入）。
- `internal/httpapi/`: HTTP 边界聚合层（阶段 5.4 起）：根包为 gin 边界工具（`ApiError*` 响应族、`GetRequestBody`/`UnmarshalBodyReusable` 请求体读取、`SetContextKey`/`GetContextKey*` 上下文键助手，阶段 4 自 `common/gin.go` 迁入，仅依赖 `infra/cache`、`domain/shared`、`internal/common`（ContextKey）与 `pkg/jsonx`）；`router/`（API、relay、dashboard、web 静态路由）、`middleware/`（认证、限速、日志、分发、安全校验）、`controller/`（HTTP 边界、请求校验、响应组织，按资源拆子包：`channel/`、`user/`、`token/`、`billing/`、`relay/` 等，包名带 `controller` 后缀；`testsupport/` 为测试共享 fixture，仅 `_test.go` 可导入）。
- `internal/domain/`: 领域层（阶段 5.1/5.3 落地）：`billing/`（计费核心服务 + `contract/` 契约叶子包 + `plan_quota/` 套餐配额）、`channel/`（渠道服务与自动禁用，含 `constant/`）、`audit/`（RecordAudit 入口）、`rankings/`、`ticket/`、`sensitive/`（敏感词匹配）、`group/`（分组倍率）、`shared/`（原 `dto/`+`types/` 合并的过渡收容包，只出不进）。
- `internal/infra/`: 基础设施层（阶段 5.3/4 起）：`db/`（数据库类型与连接状态变量）、`redis/`（Redis 客户端与读写族）、`cache/`（磁盘缓存、请求体存储、Redis 限流器）、`email/`（SMTP 发送）、`security/`（SSRF/IP/URL/哈希/TOTP/验证码/受信代理）、`runtime/`（系统监控、pprof、pyroscope、有界 goroutine 池）、`log/`（业务日志）、`httpclient/`（通用 HTTP 传输层与 SSRF 复查、代理客户端构造）、`media/`（文件/图片/音频解码下载）、`tokenizer/`（token 计数与估算）、`notify/`（用户通知：邮件/webhook/bark/gotify 与频控）、`payment/`（epay 回调地址、stripe 集成与订单锁）、`passkey/`、`custom_voice/`。
- `internal/store/`: 持久层（原 `model/`，阶段 5.2 按资源拆）：GORM 模型、迁移、缓存、数据库访问；子包按资源垂直拆分（`db/`、`channel/`、`user/`、`token/`、`log/` 等，包名带 `store` 后缀）。
- `internal/config/`: 系统、运营、模型、倍率、性能、审计等配置（原 `setting/`；ConfigManager 在 `internal/config/manager/`）。
- `internal/common/`: 业务全局变量与零碎工具内核（阶段 4 拆解后）：全局开关/限流/SMTP/OAuth 变量、`ContextKey` 注册表、`SysLog` 族输出、`GetEnvOrDefault*`、模型/端点与字符串工具；基础设施能力已迁 `internal/infra/`，HTTP 边界工具已迁 `internal/httpapi/` 根包（JSON 包装此前已迁至 `pkg/jsonx`）。
- `internal/relay/`: AI 请求中继、协议转换、供应商适配；`relay.go` 为对外门面（re-export 各子包入口），`core/` 承载 adaptor 调度与 websocket 中继，`wire/`（含 `wire/convert/`、`wire/stream/`）承载 OpenAI wire 协议族转换（阶段 5.5 自 relay 顶层与 `relay/common/` 收口），`handler/` 承载各模态 handler；`helper/` 含协议转换（Claude/Gemini ↔ OpenAI）、relay 错误包装、响应透传工具（阶段 5.3 自原 `service/` 并入）与上游响应体限额读取（阶段 4 自 `common/response.go` 迁入）。
- `internal/oauth/`: OAuth 供应商（原 `internal/constant/` 跨领域常量包已于阶段 4 解散：`ContextKey` 归 `internal/common`，运行时限值/缓存键/Setup 归 `internal/domain/shared/`；渠道域常量在 `domain/channel/constant/`，`FinishReason`/`RelayFormat` 在 `relay/constant/`）。
- `internal/i18n/`: 后端 API 响应消息多语言翻译。
- `pkg/`: 可独立复用且无业务依赖的基础库（`jsonx`、`cachex`），进入前必须通过依赖核查，详见 [pkg/AGENTS.md](pkg/AGENTS.md)。
- `web/`: 前端 UI，React 19 + TypeScript + Rsbuild；`web/embed.go` 是 `web/dist` 的 Go embed 声明载体，经 `internal/app/webdist` 暴露给启动装配层。

## 全局工作规则

- 先建立证据链再改代码：现象、入口、相关代码/配置、根因层级、最小修复点、验证方式。
- 保持工作区脏改隔离。不要回滚、覆盖或格式化与当前任务无关的用户改动。
- 不做破坏性 Git 操作，不自动 commit/push；需要提交时只 add 相关具体文件。
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

常用验证:

- `go test ./...`
- `go test ./internal/domain/... ./internal/infra/... ./internal/relay/... ./internal/httpapi/...`
- `go build -ldflags "-X 'github.com/NookMux/NookMux/internal/common.Version=$(git rev-parse HEAD)'" -o NookMux ./cmd/server`

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
