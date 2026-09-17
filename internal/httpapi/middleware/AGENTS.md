# internal/httpapi/middleware/AGENTS.md

`internal/httpapi/middleware/` 是 HTTP 中间件层，负责统一的身份认证、角色鉴权、访问限速、渠道分发、安全防护、上下文注入及访问日志。

## 规则

- **显式失败拦截**：身份认证、角色检查、上下文提取、请求 ID 注入、限速 key 计算及安全防护校验失败时必须显式终止并返回对应状态码，严禁静默放行。
- **请求体可重用性**：在中间件中若需预读请求 Body，必须调用可重用读取助手并恢复 `c.Request.Body`，切勿破坏后续 relay 协议转发、多模态文件上传或签名校验。
- **保护流式与长连接**：严禁在全局中间件中引入可能破坏 SSE（Server-Sent Events）、WebSocket 或分块传输（Chunked）的缓冲式压缩或缓存机制。
- **缓存策略一致性**：限速与分发判定逻辑必须保证 Redis 集中模式与内存轻量模式行为严格一致。
- **脱敏日志规范**：访问日志严禁输出用户 Token、渠道 API Key、OAuth Secret 或包含敏感凭证的完整请求体。

## 验证

- 修改认证、限速、分发或请求体处理后执行：
  `go test ./internal/httpapi/middleware/... ./internal/httpapi/controller/... ./internal/relay/...`
- 影响流式传输机制时，必须验证 SSE / Relay 流式中继路径的完整性。
