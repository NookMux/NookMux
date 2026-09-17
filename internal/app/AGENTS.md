# internal/app/AGENTS.md

`internal/app/` 是应用级启动装配层，负责系统环境初始化、依赖注入编排、后台任务装配、HTTP Server 生命周期管理与前端嵌入资产门面，不承载具体业务规则。

## 结构

- `bootstrap.go`：系统基础设施（环境、日志、数据库、配置、缓存、监控、i18n）启动顺序编排。
- `env.go`：命令行启动 flag（`--port`/`--version`/`--help`/`--log-dir`）解析与 `InitEnv` 环境装配。`--log-dir` 解析结果通过 `infra/log.Dir` 注入日志包（infra 不得反向 import app）。
- `server.go`：Gin server、session、后台 worker 与路由挂载编排；进程退出码在此统一返回给 `cmd/server`。
- `analytics.go`：Umami / Google Analytics 统计脚本注入；纯函数式处理 HTML index 字节，不持有可变全局状态。
- `webdist/`：为应用层提供前端嵌入资产门面，导出静态资源 `EmbedFolder`（供 `httpapi/router` 构建 Web 文件系统）。因 Go `//go:embed` 不支持跨层 `..`，真实嵌入声明必须位于 `web/embed.go`，由本包对外暴露内部 API；禁止将 `web/dist` 复制到本目录。

## 规则

- 本层仅做依赖装配与生命周期管理，严禁侵入业务逻辑或反向依赖业务控制器。
- 启动顺序变更必须逐项核对依赖链：确保按照数据库连接与迁移 → 配置热更新 → 渠道缓存预热 → i18n 翻译注入 → 系统监控启动的时序执行。
- 分析脚本注入必须保持占位符替换语义，严禁破坏 `/v1`、`/api` 等后端路由的独立性，避免被前端静态路由吞掉。
- 修改前端嵌入资产载体时，需同步检查 `web/embed.go`、`web/dist/index.html` 及 `internal/httpapi/router/web_router.go`。

## 验证

- `go test ./internal/app/... ./internal/httpapi/router/... ./internal/common/...`
- `go build ./...`
- 修改启动装配或嵌入路径后执行：`go run ./cmd/server` 冒烟验证。
