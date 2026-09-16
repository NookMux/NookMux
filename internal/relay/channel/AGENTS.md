# internal/relay/channel/AGENTS.md

`internal/relay/channel/` 是多上游 AI 模型供应商适配器（Provider Adaptor）的集合目录，负责将标准化请求转换为各服务商的专有 API 格式（OpenAI、Claude、Gemini、DeepSeek、AWS Bedrock、Vertex AI、MiniMax 等）并处理响应。

## 适配器架构与 `Adaptor` 接口

每个供应商在独立的一级子包（如 `openai/`、`claude/`、`gemini/`、`deepseek/`）中维护自身适配逻辑，并实现 `adapter.go` 中定义的 `channel.Adaptor` 接口：

- `Init(info *relaycommon.RelayInfo)`：解析请求上下文，初始化流式中继等关键参数。
- `GetRequestURL(info *relaycommon.RelayInfo) (string, error)`：动态计算目标请求 URL（包含 BaseURL 解析、端点路径拼接、模型重定向与供应商专有版本号）。
- `SetupRequestHeader(c *gin.Context, req *http.Header, info *relaycommon.RelayInfo) error`：注入上游鉴权头（如 `Authorization: Bearer <key>`、`x-api-key` 等）。
- `ConvertOpenAIRequest(...)` / `ConvertClaudeRequest(...)` / `ConvertGeminiRequest(...)`：将统一协议请求转换为供应商专有入参格式。
- `DoRequest(c *gin.Context, info *relaycommon.RelayInfo, requestBody io.Reader) (any, error)`：执行出站 HTTP 请求。
- `DoResponse(c *gin.Context, resp *http.Response, info *relaycommon.RelayInfo) (usage any, err *shared.NookMuxError)`：解析上游响应体，提取生成文本与真实 Token 用量。
- `GetModelList() []string`：声明该渠道支持的默认模型名列表。
- `GetChannelName() string`：声明供应商的唯一标识名称。

### 注册新适配器

新增供应商渠道时，必须在 `internal/relay/core/relay_adaptor.go` 的 `GetAdaptor(apiType int)` 中增加对应的 switch case 分支映射。

## 规则

- **强制走安全代理出站**：所有外网请求必须通过 `internal/infra/httpclient` 构造的代理客户端（`NewProxyHttpClient` / `NewProxyWebSocketDialer`）发起，严禁使用裸 `http.Client` 直连。
- **流式用量选项支持**：检查供应商是否支持流式模式下返回用量（如 OpenAI `stream_options.include_usage`），适时透传或修正。
- **敏感凭据绝不泄露**：上游 API Key、签名密钥严禁写入任何日志或暴露在报错信息中；错误统一包装为 `*shared.NookMuxError` 并保留上游原始 HTTP 状态码与可诊断信息。
- **可选字段指针化**：转发给上游的请求结构中，可选标量字段必须使用指针类型（如 `*int`、`*bool`）配合 `omitempty`，避免客户端显式传入的零值（如 `temperature: 0`）被丢弃。
- **统一 JSON 序列化**：处理请求与响应数据时，严格调用 `pkg/jsonx` 包装函数，禁止直接使用 `encoding/json`。

## 验证

- 测试特定供应商：`go test -v ./internal/relay/channel/<provider>/...`
- 全量中继回归：`go test ./internal/relay/...`
