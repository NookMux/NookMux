# internal/oauth/AGENTS.md

`internal/oauth/` 是第三方 OAuth 登录认证服务商扩展层，负责集成各大 OAuth 平台（GitHub、Linux.do 等）的授权认证与系统用户绑定。

## 架构与 `Provider` 接口

各 OAuth 供应商文件集中在本目录（如 `github.go`、`linuxdo.go`），并实现 `provider.go` 中定义的 `Provider` 接口：

- `GetName() string`：获取服务商显示名称（如 `"GitHub"`、`"LinuxDo"`）。
- `IsEnabled() bool`：检查系统配置中该登录渠道是否启用。
- `ExchangeToken(ctx context.Context, code string, c *gin.Context) (*OAuthToken, error)`：通过回调 authorization code 换取 access token。
- `GetUserInfo(ctx context.Context, token *OAuthToken) (*OAuthUser, error)`：调用平台接口拉取第三方用户信息（用户唯一 ID、用户名、邮箱等）。
- `IsUserIDTaken(providerUserID string) bool`：查询该第三方账号是否已被平台现有用户绑定。
- `FillUserByProviderID(user *userstore.User, providerUserID string) error`：通过第三方 ID 加载对应的本地用户。
- `SetProviderUserID(user *userstore.User, providerUserID string)`：将第三方用户唯一标识绑定至用户实体。

### 注册新服务商

实现 `Provider` 接口后，在 `internal/oauth/registry.go` 的 `RegisterProvider(provider Provider)` 中统一注册。

## 规则

- **CSRF 状态校验**：OAuth 认证流程中必须严格校验 `state` 随机防伪令牌，防止跨站请求伪造。
- **密钥安全隔离**：Client Secret 仅通过配置系统（`config`）动态读取，严禁硬编码或在错误日志中打印。
- **错误显式暴露**：OAuth 平台网络不通、Token 换取失败或用户绑定冲突时，必须显式抛出具体错误，严禁静默忽略导致假登录。

## 验证

- `go test ./internal/oauth/...`
- `go build ./...`
