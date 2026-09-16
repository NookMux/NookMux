# internal/relay/AGENTS.md

`internal/relay/` 是 AI API 请求中继与供应商适配核心层，负责多模态协议转换、流式逐帧处理、用量统计及上游适配，改动风险极高。

## 包结构与分工

- `relay.go`：对外统一门面（Facade），仅 re-export 各子包入口（`GetAdaptor`、各 `*Helper`、`OpenAIWireHelper` 等），保持外部 import 稳定；禁止在门面层写入业务逻辑。
- `core/`：多供应商 Adaptor 调度（`GetAdaptor`）、WebSocket 中继（`WssHelper`）及存储资产签名 URL 生成与校验。
- `wire/`：OpenAI wire 协议族。`auto_convert.go` 负责 Chat ↔ Responses 自动转换调度；`wire/convert/` 承载请求/响应/用量转换与 tool 代理上下文，其中 `Converter` 是 chat ↔ responses 双向转换的单点入口；`wire/stream/` 是流式转换器与 SSE 改写 writer。
- `handler/`：各模态中继 handler（chat、claude、gemini、audio、image、embedding、rerank、responses、stored media）。
- `channel/`：各模型供应商适配器实现，按 `<provider>/` 划分一级子包，详见 [channel/AGENTS.md](channel/AGENTS.md)。
- `common/`：RelayInfo、`BillingSettler` 接口、参数覆写、reasoning effort 及中继横切上下文。
- `helper/`：Claude / Gemini ↔ OpenAI 协议双向转换、中继错误包装与响应透传工具。
- `common_handler/`、`reasonmap/`、`constant/`：基础中继辅助与常量。

**严格无环依赖链**：
`relay(门面) → {core, wire, handler}`；`wire → {handler, common, helper, wire/convert, wire/stream}`；`handler → {core, common, helper, constant}`；`common → wire/convert`；`wire/stream → wire/convert`；`wire/convert` 不依赖任何 relay 内部包（仅依赖 `domain/shared`、`internal/common` 与 `pkg/jsonx`）。

## 规则

- **协议边界隔离**：保持 OpenAI wire、Responses、Chat Completions、Claude、Gemini、AWS 等协议转换严格解耦。OpenAI wire 转换一律收口在 `wire/convert/` 与 `wire/stream/`。
- **单点转换入口**：Chat ↔ Responses 转换必须统一经单点入口调用：请求与非流式响应走 `convert.NewConverter(...)`，流式逐帧走 `stream.NewChatToResponsesStreamConverter` / `NewResponsesToChatStreamConverter`，严禁绕过门面直调底层或重复造转换器。
- **严格计费边界**：用量到额度的计算与落账必须且只能通过领域层 `domain/billing.CalculateUsage(...)` + `ApplyQuota(...)`（通用文本路径）或各模态专属结算入口完成；relay 各层**严禁 import store** 的 token/user/channel 包直接读写配额。
- **流式输出管道保真**：流式逐帧输出必须严格保证 chunk 顺序、异常终止事件推送、finish reason 映射、usage 统计准确性及连接及时释放。
- **可选标量字段指针化**：向客户端或上游转发的可选标量字段，DTO 必须采用 `*int`、`*uint`、`*float64`、`*bool` 等指针类型配合 `omitempty`，防止客户端显式传入的 `0`、`0.0`、`false` 被 Go 零值过滤抛弃。
- **错误诊断性与透传**：严禁吞掉上游错误；错误类型、HTTP 状态和用户可见信息必须保持可定位可排查。
- **凭据脱敏安全**：请求身份、渠道密钥、用户 Token、签名与敏感 header 严禁打印在业务日志中。
- **统一序列化**：全包 JSON 序列化/反序列化调用严格遵守项目规范，必须走 `pkg/jsonx`。

## 验证

- 改动协议转换后执行全量回归：`go test ./internal/relay/...`
- 修改具体供应商时，执行该 provider 子包测试与公共转换测试。
- 涉及用量统计或计费变动时，补充运行计费测试：`go test ./internal/domain/billing/...`
