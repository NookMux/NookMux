# internal/httpapi/router/AGENTS.md

`internal/httpapi/router/` 是路由挂载与静态资源分发层，负责 API、Relay、Dashboard 与前端 Web 静态站点的路由声明与绑定，不承载任何业务逻辑。

## Web 静态路由

- `SetWebRouter` 结合 `internal/app/webdist` 提供的 `WebAssets` 和 `EmbedFolder` 提供前端 SPA 静态资产服务。
- **404 隔离防护**：在 `NoRoute` 处理中，`/v1`、`/api`、`/assets` 必须返回标准 JSON 404 错误响应，严禁错误回退为前端 HTML 页面。
- 分析脚本注入位于 `internal/app/analytics.go`，处理的是 `internal/app/webdist` 提供的 index 字节副本。
- `FRONTEND_BASE_URL` 仅在独立前端分离部署模式下生效，保持现有 302 重定向语义。

## 路由边界规则

- **纯声明式路由**：严禁在路由层解析业务参数、处理权限计算或直连数据库；所有输入转由对应 controller 处理。
- **保护流式传输**：禁止挂载会破坏 SSE / 流式响应的全局 gzip 中间件；静态资产压缩仅限在 web 静态路由组内独立配置。
- **公开路由受众隔离**：挂载匿名路由（无 `AdminAuth()` / `UserAuth()`）时，必须确认对应控制器不返回角色受限数据，详细准则见 [controller/AGENTS.md](../controller/AGENTS.md#api-设计)。

## 验证

- `go test ./internal/httpapi/router/... ./internal/app/webdist/...`
- `go build ./...`
- 若涉及 Relay 路由，补充执行：`go test ./internal/relay/...` 并核对流式输出。
