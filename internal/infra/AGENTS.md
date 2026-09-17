# internal/infra/AGENTS.md

`internal/infra/` 是基础设施层，为全系统提供技术型基础设施支撑（网络传输、持久化状态变量、通用缓存、安全防护、运行时监控、媒体解析等），不承载业务控制边界。

## 子包职责

| 子包 | 核心能力与关键职责 |
|---|---|
| `db/` | 数据库类型常量与连接状态变量（`UsingSQLite`/`LogSqlType`/`SQLitePath` 等；GORM 连接与模型映射位于 `internal/store/db`）。 |
| `redis/` | Redis 客户端句柄（`RDB`）与操作助手（`InitRedisClient`/`Redis*` 读写族）。 |
| `cache/` | 业务级缓存实现：磁盘缓存（`disk_cache*.go`）、请求体存储（`body_storage.go`，含 `ErrRequestBodyTooLarge`）与 Redis 限流器（`limiter/`）。注：底层无业务依赖缓存原语位于 `pkg/cachex`。 |
| `email/` | SMTP 邮件发送引擎（含 Outlook LOGIN 兼容认证）。 |
| `security/` | 安全防护体系：SSRF 防护与连接时复查（防 DNS rebinding）、IP/CIDR 校验、URL 白名单、HMAC/bcrypt 哈希、TOTP/备用码、验证码与 Gin 受信代理配置。 |
| `runtime/` | 运行时设施：系统资源监控（CPU/内存/磁盘）、pprof 剖析、Pyroscope、有界 Relay 协程池（`RelayGo`/`RelayCtxGo`）及安全 channel 发送。 |
| `log/` | 业务运行日志（`LogInfo`/`LogError` 等）；日志落盘目录通过 `Dir` 由 app 装配层注入。 |
| `httpclient/` | 通用 HTTP 传输层：全局客户端、连接建立时 SSRF 复查、代理客户端工厂与复用缓存（`NewProxyHttpClient` / `NewProxyWebSocketDialer`）。 |
| `media/` | 多模态媒体处理：文件/图片/音频下载、MIME 探测、格式转换与 base64 处理。 |
| `tokenizer/` | Token 计数与用量估算服务（`tokenizer.go`、`token_estimator.go`、`token_counter.go`）。 |
| `notify/` | 用户通知发送（邮件、webhook、bark、gotify）与频率控制。 |
| `payment/` | 支付网关集成：易支付（epay）回调、Stripe Checkout 会话生成与 webhook 事件处理、分布式订单锁（`LockOrder`/`UnlockOrder`）。 |
| `passkey/` | WebAuthn / Passkey 凭据解析与会话转换。 |
| `custom_voice/` | MiniMax 定制语音克隆、试听与计费确认。 |

## 依赖与设计规则

- **严禁承载 HTTP 边界**：除向后兼容的历史签名外，Gin Handler 必须留在 controller，本层只提供无 HTTP 侵入的可复用实现。
- **强制走安全代理出站**：所有渠道调用、模型探活与外部资源获取必须走 `httpclient`，严禁使用裸 `http.Client`，防止内网探测与 SSRF 攻击。
- **媒体与文件边界防御**：文件下载、解析与缓存前必须严格校验文件大小上限、MIME 类型及合法来源，严禁静默吞掉下载或解码错误。
- **清晰暴露失败**：严禁采用伪造成功、假用量或静默降级掩盖基础设施故障。
- **子包单向依赖**：infra 子包之间允许无环单向依赖（如 `tokenizer → media → httpclient`、`notify → {media, httpclient}`、`log → runtime`、`runtime → cache`）。
- **严格禁止反向依赖**：infra **严禁 import** controller / middleware / router / relay（历史遗留仅 `tokenizer/token_counter.go` 引用 relay 类型及 `httpapi` 根包纯工具，严禁新增其他反向引用）。
- **待机内存保守策略**：连接池空闲上限、本地缓存阈值、后台监控采集频率必须保持保守默认值。调大时必须提供环境变量覆盖，并同步更新 `.env.example` 与相关中英文档。

## 验证

- `go build ./... && go test ./internal/infra/...`
- 修改 SSRF 防护或代理客户端后执行：
  `go test ./internal/infra/httpclient/...`
- 修改请求体暂存或磁盘缓存后执行：
  `go test ./internal/httpapi/... ./internal/infra/cache/...`
