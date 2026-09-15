# PRD：Relay 协议转换层 IR 重构（以 Chat 为中心）

> 版本：v2.1
> 日期：2026-08-26
> 状态：待评审

## 1. 背景

### 1.1 现状：隐式 Hub + 每渠道手工拼装

OpenAI Chat 是事实上的中间协议，但未形式化、无纪律约束。回程转换全部走 `上游协议 → OpenAI Chat → 下游协议` 两跳：


| 转换路径                           | 实现位置                                       | 实际做法                                                                 |
| ------------------------------ | ------------------------------------------ | -------------------------------------------------------------------- |
| Claude 上游 → Gemini 客户端         | `channel/claude/relay_claude.go:1007,1017` | `StreamResponseClaude2OpenAI` 再 `helper.StreamResponseOpenAI2Gemini` |
| Claude 上游 → Responses 客户端（流式）  | `channel/claude/relay_claude.go:993,1089`  | `StreamResponseClaude2OpenAI` 再 `NewChatToResponsesStreamConverter`  |
| Claude 上游 → Responses 客户端（非流式） | `channel/claude/relay_claude.go:1205-1211` | `ResponseClaude2OpenAI` 再 `convert.NewConverter(Chat, Responses)`    |
| Responses 客户端 → Claude 上游（去程）  | `channel/claude/adaptor.go:94-113`         | `ConvertResponsesToChatRequest` 再 `ConvertOpenAIRequest`             |
| Gemini/Claude 客户端 → OpenAI 上游  | `channel/openai/adaptor.go:40,62`          | `GeminiToOpenAIRequest` / `ClaudeToOpenAIRequest`                    |



### 1.2 隐式 Hub 造成的五个实际缺陷

**D1：转换矩阵有洞，Responses 客户端打到三类渠道直接报错**

`channel/gemini/adaptor.go:229`、`channel/vertex/adaptor.go:300`、`channel/aws/adaptor.go:143` 的 `ConvertOpenAIResponsesRequest` 全部是 `// TODO implement me` + `return nil, errors.New("not implemented")`。

**D2：每渠道手写四路格式分发，新增组合要改所有渠道**

`channel/claude/relay_claude.go:962-1028`（流式）和 `:1196-1230`（非流式）各有一个按 `info.RelayFormat` 的四路 switch（Claude/OpenAI/Responses/Gemini），`channel/openai/helper.go:54-199` 同构。新增一个协议组合，要在每个渠道的去程 `Convert*Request`、回程流式 switch、回程非流式 switch 三处各加一遍。

**D3：内建工具被静默丢弃**

`wire/convert/openai_wire_convert_request_tools.go:338-345`：Responses→Chat 转换时 `web_search`、`web_search_preview`、`image_generation` 三类工具直接 `return nil, nil`，不返回错误。客户端声明了搜索工具、期待按调用计费，实际上游没执行、也没有任何错误或标记告知客户端能力已降级。

**D4：计费语义标记只在一处设置，复用渠道漏设**

`FinalRequestRelayFormat`（`relay/common/relay_info.go:160`）全仓库仅 `channel/claude/adaptor.go:121` 一处赋值。该标记决定缓存 token 计费公式（`domain/billing/usage.go:108` 的 `isClaudeUsageSemantic`）：true 假设 `prompt_tokens` 不含缓存，false 假设含。AWS 渠道复用 Claude 处理函数（`channel/aws/relay_aws.go:222,235,254,266,279`）却不设置该标记，导致缓存计费偏差。定性疑点与修复方向见阶段 0.1、阶段 8。

**D5：错误出口漏了 Gemini，底子铺好却没接上**

`domain/shared/error.go:32` 定义了 `ErrorTypeGeminiError = "gemini_error"`，`domain/shared/gemini.go:562` 定义了 Gemini 标准错误结构 `GeminiErrorResponse{Code, Message, Status}`。但 `ErrorTypeGeminiError` 全仓库零使用，`NookMuxError` 没有 `ToGeminiError()`，出口分发（`httpapi/controller/relay/relay.go:98-110`）只有 Realtime/Claude/default 三路。Gemini 客户端报错时走 default 分支，收到 OpenAI 格式错误体——`status` 缺失，`code` 从数字变字符串，与 Gemini 规范不兼容。出口分发重复与跨格式枚举错塞问题见阶段 9。

## 2. 依据与边界

### 2.1 设计约束

1. **有损点显式化，不追求「IR 无损」。** `file_search`/`image_generation` 转到无等价能力的上游直接拒绝（见 §8 问题 4）、思考签名跨协议按提供者识别过滤会丢弃非目标提供者签名（见阶段 6）。降级写入后台日志与用户文档，不静默丢弃；降级信号不向客户端响应注入（见 §8 问题 2）。
2. **保留同协议直通，不追求「全量请求过 IR」。** 本项目已有同协议直通优化（`channel/claude/adaptor.go:33-35` 的 `ConvertClaudeRequest` 直接返回原请求；DeepSeek/Moonshot/OpenRouter 的原生 Anthropic 端点透传），必须保留，见 §3.3。
3. **复用而非重写流式转换器。** `wire/stream` 的 chat↔responses 逐帧转换器已处理大量边界情况，IR 化是复用与收口，不是重新实现。

### 2.2 非目标

- 不改渠道 URL 构建与认证逻辑（`GetRequestURL` / `SetupRequestHeader` 原样保留）
- 不改路由对外路径、数据库 schema、模型倍率数值、审计逻辑、前端
- 不改重试策略本身，只适配错误模型
- Realtime/WebSocket 不纳入本 PRD 范围（见 §8 问题 5 决策，继续走现有 WebSocket 旁路）
- 不引入官方 SDK。与 SDK 迁移的顺序结论：**先 IR 后 SDK**——IR 边界定清后，把 Outbound 内部实现换成官方 SDK 是局部替换，不影响上层；反之先做 SDK 迁移，接口改造会做两遍

### 2.3 协作纪律

<!-- TODO: 每阶段合入时必须同步更新的信息载体清单（AGENTS.md、docs/、i18n 常量等），从各阶段验收项中提取汇总 -->



## 3. 架构设计



### 3.1 IR 选型：以 Chat 规范为基础的扩展 IR

| 候选                 | 结论与理由                                                                                                                                                                 |
| ------------------ | --------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| **以 Chat 规范为基础的扩展 IR** | **采纳**。以 OpenAI Chat 规范为底座，补齐 §3.4 三类非 Chat 概念（思考签名、服务端工具计数、工具名代理上下文）。现有转换资产以 Chat 为轴（约八成：`helper/convert.go` 全部函数、各渠道 `ConvertOpenAIRequest`、`ResponseClaude2OpenAI` 系列），换成别的 IR 等于全部重写；`shared.ClaudeUsageToOpenAIUsage`（`domain/shared/claude.go:609-615`）已按 OpenAI 语义做 `input + cache_read + cache_creation` 归一，与 axonhub `llm.Usage` 同构 |
| OpenAI Responses   | 否。携带会话状态（`previous_response_id`）、服务端内建工具、item id 等产品语义，让 Claude/Gemini 请求先变成 Responses 等于强迫所有协议背负这些概念                                                                 |
| 全新中立 IR            | 否。等价于重写全部转换算子，收益不足以覆盖风险                                                                                                                                               |


**IR usage 语义唯一且逐字段独立**（输入、缓存读取、缓存写入/创建分档、输出、音频各自独立保留，不对 `prompt_tokens` 做先合并再减）。依据是各家官方本就把输入与缓存分开下发（见 §4.1.3 对照表），且与 axonhub 一致。落地后 `usage.go:108` 的 `isClaudeUsageSemantic` 分支可以删除——语义由边契约保证，不再依赖「渠道记得打标」，这是 D4 的根治方式而非打补丁。

### 3.2 三层正交分离

```
客户端 HTTP
    │
    ├─ Inbound（每协议一份）：客户端协议 ↔ IR
    │
  IR（以 Chat 规范为基础的扩展 IR）
    │
    ├─ Outbound（每协议一份，不是每渠道）：IR ↔ 上游协议
    │
    ├─ 渠道适配层（每渠道）：URL 构建 / 认证头 / 方言补丁 ← 本 PRD 不改
    │
上游 HTTP
```

关键边界：**协议转换只负责请求体与协议语义必需的头**（如 Claude 的 `anthropic-version`、`anthropic-beta`）。密钥、代理、渠道基址、组织头属于渠道适配层，与 IR 无关。出站边按标准协议实现，17 个渠道共用 4 条主干边。

### 3.3 同协议直通必须保留

当 Inbound 协议与 Outbound 协议相同、且无渠道级请求改写时，**跳过 IR 往返**，直接透传。

理由：现状已有此优化（见 §2.2 第 2 点），若 IR 化后强制 `Claude→IR→Claude` 往返，会引入两次 marshal 开销和保真风险，纯属倒退。

这也是 IR 化后 `NewEmptyUsageRetryError` 判定的复用点，见 §4.3。

### 3.4 IR 需承载的非 Chat 概念

Chat 作为 IR 底座缺三类概念，必须在超集中补齐，且不能塞进现有文本字段：


| 概念        | 各协议来源                                                                                                                                                                                                                           | 承载要求                                |
| --------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | ----------------------------------- |
| 思考签名      | Claude thinking block 的 signature（`domain/shared/claude.go`）；Gemini `thoughtSignature`（`domain/shared/gemini.go`，含 `GetThoughtSignature()`，可挂在含 functionCall 在内的任意 part 上）；Responses reasoning item 的 `encrypted_content`（`domain/shared/openai_response.go:404` 仅定义、全仓库零读写，属死字段；请求侧输入结构 `responsesReasoningInput` 无签名字段）；Chat 侧现有私有扩展 `reasoning_signature`（`openai_request.go:304`，非 OpenAI 官方字段，是本网关的事实签名载体） | 与所属内容块绑定，不可展平为全局字段（同协议 bit 级保真所需），工具调用块同样可挂签名（借鉴 axonhub 三层方案，见阶段 4）。跨协议按提供者识别过滤（阶段 6） |
| 服务端工具调用计数 | Responses 内建工具（`relay/common/relay_info.go:382-388` 初始化，`channel/openai/relay_responses.go:81,145,149` 递增）；Claude/Gemini 走 gin 字符串键 `claude_web_search_requests`、`gemini_web_search_requests`                                   | 升为 IR 一等字段，替代 gin context 旁路。详见阶段 7 |
| 工具名代理上下文  | `OpenAIWireToolContext`（挂在 `RelayInfo.OpenAIResponsesToolContext`），负责 namespace 扁平化与还原、custom tool 代理                                                                                                                           | 迁入 IR 会话态，不再挂 RelayInfo。详见阶段 1      |




## 4. 计费专项复核

这一节独立成章，因为它是本次重构风险最高的部分，且现状已有确认的偏差。

### 4.1 问题定性：计费读了转换后的数据

完整链路是：

```
用户请求 → [转换] → 上游请求 → 上游响应 → [转换] → 用户响应
                                    ↑
                          计费必须在此处取数（上游原始语义）
```

现状偏差分两类：

**类型 A：读到转换后的请求体。**

`relay/wire/auto_convert.go` 在协议转换后调用 `setTemporaryRequestBody(c, bodyBytes)` 直接覆写 gin 请求体，靠 `takeRequestBodySnapshot`/`restore` 与 `takeRelayInfoSnapshot`/`restore` 成对恢复。在这个窗口期内，任何通过 `GetRequestBody` 读请求体的逻辑（计费、日志、审计、token 估算）拿到的都是转换后的体，而不是用户原始输入。

**类型 B：语义标记丢失。**

即 D4。`FinalRequestRelayFormat` 漏设导致 usage 按错误语义计算，AWS Bedrock 的 Claude 模型缓存计费偏差（定性存疑：基准未经验证，不能默认直连 Claude 渠道为正确范本，见 D4 与阶段 0.1）。

**根因（不是 A/B 的补充，而是共同病灶）：计费层拿到的是「被转换 + 被猜语义」的 usage，而不是上游原值。**

`domain/billing/usage.go:75` 的 `CalculateUsage` 接收的 `rawUsage *shared.Usage` 是 Outbound 归一化后的值，且靠 `usage.go:108` 的 `isClaudeUsageSemantic`（= `FinalRequestRelayFormat == RelayFormatClaude`）去猜「`PromptTokens` 含不含缓存」来决定加减。上游官方本来就把输入、缓存读取、缓存写入、输出拆得明明白白（见 4.1.3 对照表），计费层本不需要猜——猜才会出 D4 双计这类错账。

#### 4.1.3 各渠道规范计费读取分析（根治方向）

**原则：上游原值只读一次、只归一一次、落成唯一标准值，此后任何消费者不得再按协议加加减减、不得再靠语义标记猜。**

计费正确性不取决于「合并式 vs 拆分式」，而取决于「计费层拿到的值是否与上游官方原值逐字段一致」。各家官方 usage 字段语义如下（以官方响应为准，非网关内部口径）：

| 协议 | 输入字段 | 是否含缓存 | 缓存字段 | 输出字段 |
| --- | --- | --- | --- | --- |
| OpenAI Chat | `prompt_tokens` | **含** | `prompt_tokens_details.cached_tokens` / `.audio_tokens` | `completion_tokens` |
| OpenAI Responses | `input_tokens` | **含** | `input_tokens_details.cached_tokens` / `.cache_write_tokens` | `output_tokens` |
| Claude | `input_tokens` | **不含** | `cache_read_input_tokens` + `cache_creation_input_tokens`（分 5m/1h 三档） | `output_tokens` |
| Gemini | `promptTokenCount` | **不含** | 无标准缓存拆分字段（走 context/tier 分档） | `candidatesTokenCount` |
| OpenRouter | `prompt_tokens` | **含**（与 OpenAI 同构） | `prompt_tokens_details.cached_tokens` | `completion_tokens` |

由此得出**读取规范**：

1. **Outbound 是唯一读取点**（对齐 P1）：每条边把自己的上游原值字段映射为 IR usage，逐字段保留（输入、缓存读取、缓存写入/创建分档、输出、音频），不做任何加减。
2. **IR usage 落库值语义唯一且自洽**：`ir.Usage.PromptTokens` 存「上游实际下发的输入字段原值」；缓存/音频拆分字段各自独立保存。**禁止**为「统一口径」先合并再减（`shared.ClaudeUsageToOpenAIUsage`/`OpenAIUsageToClaudeUsage` 的合并-还原往返是错误样板，见下）。
3. **计费层只做乘法、不做语义猜测**：按缓存倍率单独计价时，直接读 IR usage 的缓存字段，**不**再通过 `isClaudeUsageSemantic` 推断 `PromptTokens` 含不含缓存。P1 落地后该标记恒为无意义，阶段 8 删除。
4. **`OpenAIUsageToClaudeUsage`（`claude.go:664`）的 `promptTokens - cacheRead - cacheCreation` 是有损减法**：它假定 `PromptTokens` 是「合并式含缓存」，只对「归一后数据」成立。一旦上游原值是拆分式（Claude/Gemini），减过头。重构后该函数只应在 Claude Outbound「把 IR 的独立字段填回 Claude 官方拆分子段」时使用，且不得改变 `PromptTokens` 本体。

**落地形式（写入阶段 8）：**

- 每渠道一条「上游原值字段 → IR usage 字段」映射记录（对齐本文档 §2.1 的「本仓库代码为准」原则，对照上游官方响应样例逐一核实，不许凭记忆）。
- 阶段 0.3 的计费对拍框架，除断言「新旧路径 quota 一致」外，**追加断言「归一化后 IR usage 各缓存/输入/输出字段与上游官方 response 逐字段一致」**——对拍只保证「重构不改账」，本断言保证「账本身读对了」。
- `CalculateUsage` 输入从「猜语义的归一值」收敛为「逐字段自洽的标准值」，删除 `isClaudeUsageSemantic` 分支及 `BuildContextPricingUsage` 的语义参数。

### 4.2 五条强制原则


| 编号  | 原则                           | 说明                                                     |
| --- | ---------------------------- | ------------------------------------------------------ |
| P1  | usage 归一化在 Outbound 边完成      | 产出物必须是 IR usage 且语义唯一（OpenAI 语义），下游消费者不得再按协议分支判断       |
| P2  | 计费取数只认 Outbound 产出的 IR usage | 禁止从 gin context 请求体、转换后 DTO 反推                         |
| P3  | 原始请求体快照独立于转换链                | 供日志/审计/token 估算使用，不受 `setTemporaryRequestBody` 影响      |
| P4  | 服务端工具计数是 IR 一等公民             | 替代 `c.Set("claude_web_search_requests")` 这类 gin 字符串键旁路 |
| P5  | 工具计价的 provider 维度跟随实际上游      | 不是入站格式。对应 `GetToolBillingPrice` 的 `provider` 参数        |




### 4.3 需重新定义的既有耦合

**空 usage 重试判定会全局失效。** `domain/billing/quota.go:33-38` 的 `NewEmptyUsageRetryError` 以 `len(relayInfo.RequestConversionChain) > 1` 判断「发生过协议转换就不重试」。IR 化后几乎所有请求都有转换链，这个兜底会对所有请求失效。

必须改为显式的「上游原生直通」标记，复用 §3.3 的直通判定结果，而不是数转换链长度。

**其余两处：**

- `BuildContextPricingUsage(usage, isClaudeUsageSemantic)`：语义参数随 P1 落地后移除
- 预扣与 token 估算（`TokenCountMeta`、`SetEstimatePromptTokens`）：必须基于原始入站请求（P3），不得基于转换后请求



### 4.4 现有计费函数清单

`domain/billing/quota.go` 现有：`PreConsumeTokenQuota`、`PostConsumeQuota`、`PostClaudeConsumeQuota`、`PostAudioConsumeQuota`、`PreWssConsumeQuota`、`PostWssConsumeQuota`、`CalcOpenRouterCacheCreateTokens`。`domain/billing/usage.go:74` 的 `CalculateUsage` 是核心计算入口。

本 PRD **不合并**这些函数——它们的差异来自倍率与模态，是合理差异。本 PRD 只保证喂给它们的 usage 已按 P1 归一。函数合并如有必要另开 PRD。

## 5. 分阶段实施

每阶段独立可验证、一阶段一 PR 合入主干。阶段 1–8 按协议对推进，阶段 9 按协议做错误专项，阶段 0 与 10 为前置和收尾。

**发布约束（合入 ≠ 发布）：**阶段 1–7 的代码逐阶段合入主干，但**生产版本不在中间态发布**——由于 D3（工具静默丢弃）与跨协议工具计数缺失当前互相掩盖（见阶段 7 问题 1），阶段 1 修掉 D3 后若单独上线会出现「工具真实执行但漏计费」窗口。因此阶段 1–7 完成前只合不发布，待阶段 7（计数统一）就绪后作为一个大版本一次性上线；阶段 8 起恢复逐阶段发布。代价是发布窗口较长、整体回滚粒度变粗，作为对计费正确性的让步被显式接受。

### 阶段 0：基础准备与存量 bug 修复

本阶段不引入 IR 行为变更，目标是「把地基和安全网准备好」。0.2 与 0.3 可并行；0.1 的存量 bug 修复依赖 0.3 的对拍框架（D4 定性疑点需先以真实数字确认基准），顺延执行。

#### 0.1 修存量 bug（依赖 0.3 对拍框架，顺延执行）

**bug 1：AWS Bedrock 的 Claude 模型缓存计费偏差（D4）**

现状：AWS 的 `awsHandler`/`awsStreamHandler` 复用 `claude.HandleClaudeResponseData` 等函数处理响应，但绕过了 `claude.Adaptor.DoResponse`，不设置 `info.FinalRequestRelayFormat`，导致 `domain/billing/usage.go:108` 的 `isClaudeUsageSemantic` 为 false。

**为什么顺延（而不是先修先发）：**「谁是正确基准」未经验证——设置标记的 Claude 直连路径，其 usage 已被 `ClaudeUsageToOpenAIUsage` 归一为含缓存的 OpenAI 语义，与 `isClaudeUsageSemantic=true` 公式「不含缓存」的假设矛盾，直连渠道自身可能存在缓存 token 双计（见 D4 定性疑点）。在用对拍框架以真实数字确认基准之前就「对齐直连渠道」，存在把潜在双计复制到 AWS 的风险。因此本 bug 的修复放在 0.3 对拍框架就位之后执行。

修法（对拍确认基准后）：

- 若基准为「不减」语义：仅在 `awsHandler`/`awsStreamHandler`（`channel/aws/relay_aws.go`）补设 `info.FinalRequestRelayFormat = relayconstant.RelayFormatClaude`。**不得**在 `aws/adaptor.go` 的 `DoResponse` 顶部统一设置——那会波及 Nova 分支（OpenAI 语义 usage）与 ClientModeApiKey 分支（经 `claude.Adaptor.DoResponse` 已设置）。
- 若对拍发现直连渠道本身双计：定性反转，改为修 Claude 路径的计费公式与 usage 语义匹配问题（届时独立评估影响面，含已落账数据的处理）。
- 同步梳理 `PostClaudeConsumeQuota`（`domain/billing/quota.go:304`）基于 `ChannelType` 的独立语义判定与该标记的关系，两处判定在阶段 8 前不得出现语义分叉。

这是打补丁而非根治——根治在阶段 8（P1 让语义由边契约保证）。

**验收**：

- 阶段 0.3 对拍框架先产出 Claude 直连渠道（含 `cache_read_input_tokens` > 0 场景）的基准结论，本 bug 按结论方向修复
- `go test ./internal/domain/billing/... ./internal/relay/channel/aws/...`



#### 0.2 IR 包骨架

新建 IR 包，只定义结构不接线，本子阶段结束时无任何调用方。

```
internal/relay/ir/
├── request.go          ← IR 请求：以 Chat 规范为基础 + 签名承载 + 工具声明
├── response.go         ← IR 响应（非流式）
├── stream.go           ← IR 流式事件
├── usage.go            ← IR usage（OpenAI 语义）+ 服务端工具计数
├── error.go            ← IR 错误模型（阶段 9 使用）
├── signature.go        ← 思考签名：与内容块绑定的承载结构
├── toolcontext.go      ← 工具名代理上下文（自 wire/convert/openai_wire_tool_context.go 迁入）
└── capability.go       ← 能力降级声明：记录每条边丢弃了什么
```

字段来源不是凭空设计，而是从现有结构归并：


| IR 结构            | 来源                                                                                                                                                                 |
| ---------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------ |
| `ir.Request`     | `shared.GeneralOpenAIRequest` 为底，补 §3.4 三类概念                                                                                                                       |
| `ir.Usage`       | `shared.Usage` 为底（已含 `PromptTokensDetails.CachedTokens`/`CachedCreationTokens`/`ClaudeCacheCreation5mTokens`/`ClaudeCacheCreation1hTokens`、`AudioTokens`），补服务端工具计数 |
| `ir.Error`       | `shared.NookMuxError` 的 `RelayError any` 收敛——以 Chat 错误为中枢（code + message + status_code），跨协议转换只保证这三者双向可转，不做协议特化的强类型展开（见阶段 9）                                                                                                           |
| `ir.ToolContext` | `convert.OpenAIWireToolContext` 平移                                                                                                                                 |
| `ir.Capability`  | 新增，D3 的解药（承载降级记录，供后台日志与计费决策使用，不向客户端暴露）                                                                                                                                                          |


`capability.go` 是本次新增的核心概念：每条转换边在发生能力降级时（如签名跨协议过滤丢弃、内建工具拒绝等）写入降级记录，供后台日志和计费决策使用。降级信号不向客户端响应暴露（不加响应头、不加响应体扩展字段），降级的实际语义写入用户文档。这是 §2.2 第 1 点「有损点显式化」的落地载体。

**验收**：

- `go build ./internal/relay/ir/`
- IR 包不 import 任何 relay 内部包（对齐 `wire/convert` 现有约束）：`go list -deps ./internal/relay/ir/` 输出不含 `internal/relay/channel`、`internal/relay/handler`、`internal/relay/common`
- IR 结构字段与 `shared.Usage` 字段做覆盖性对照，列出未覆盖字段清单并逐项说明原因



#### 0.3 测试与计费对拍框架

三个框架，后续每个阶段都依赖它们：

**框架 1：roundtrip 测试。** 复用 `wire/convert/openai_wire_convert_request_roundtrip_test.go`、`openai_wire_convert_response_roundtrip_test.go` 的既有模式，扩展为「任意协议 → IR → 同协议」的通用断言，要求 bit 级等价。

**框架 2：流式 golden 测试。** 复用 `wire/stream/openai_wire_stream_conversion_test.go` 模式，为每条边固化 SSE 事件序列快照，断言 chunk 顺序、`finish_reason`、usage 帧位置、`[DONE]`、错误帧。参考项目 axonhub 用 `testdata/*.jsonl` 存流式样本，可借鉴该组织方式。

**框架 3：计费对拍。** 同一请求分别走新旧路径，断言喂给 `CalculateUsage` 的 usage 与最终 quota 完全一致。这是阶段 8 的验收依据，必须在阶段 1 之前就位，否则后续 8 个阶段的计费影响无从验证。

**框架 3 的补充断言（对应 §4.1.3「逐字段与上游原值一致」）：** 对拍只断言「重构不改账」，不足以保证「账本身读对了」。因此在每渠道对拍用例中，除 quota 一致外，**追加断言归一化后 IR usage 的输入/缓存读取/缓存写入/输出字段与上游官方 response 的原始 usage 逐字段一致**——逐渠道对照表见 §4.1.3。此为阶段 8「计费读取正确性」的验收依据。

**验收**：

- 三个框架各自能跑通至少一条现有链路（建议用 Chat↔Responses，因其既有测试最完整）
- 对拍框架能检出人为注入的偏差（故意改一个倍率，对拍必须失败）
- 框架就位后立即执行 D4 基准验证：对 Claude 直连渠道含缓存场景做计费对拍，产出「双计/正常」结论，据此落地 0.1 的修复方向
- 至少一个含缓存的渠道（建议 Claude 直连）产出「IR usage 各字段与上游原值逐字段一致」的对拍样本，作为 §4.1.3 对照表的可执行样例



### 阶段 1：Responses ↔ Chat

**现状。** 这条边是现有实现最完整的一条，已有单点入口约束：请求与非流式响应走 `wire/convert/converter.go` 的 `NewConverter` 四个方法，流式走 `wire/stream` 的 `NewChatToResponsesStreamConverter` / `NewResponsesToChatStreamConverter`。`internal/relay/AGENTS.md` 明文禁止绕过它们直调内部函数。

工具支持情况（已核实）：


| 工具类型                                                     | 现状                                                                                                                                                                                                                                                                                         |
| -------------------------------------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------ |
| `function`                                               | 支持，含 namespace 扁平化（`convert_request_tools.go:351`）与超长名 sha256 截断（`:485`）                                                                                                                                                                                                                   |
| `custom`                                                 | 支持（`convert_request_tools.go:381`、`tool_context.go:47`）                                                                                                                                                                                                                                    |
| `namespace`                                              | 支持（`convert_request_tools.go:412`）                                                                                                                                                                                                                                                         |
| `tool_search`                                            | 支持（`convert_request_tools.go:331`、`convert_response.go:370`）                                                                                                                                                                                                                               |
| `apply_patch`（此为Codex/Responses协议适配关键）                   | 支持，走 generic custom tool 通道：转为带 `input` 字段的 function call，`buildResponsesCustomToolChatDescription`（`:466`）把原 `format` 约束序列化附到 description，回程 `buildResponsesToolOutputFromChatCall`（`convert_response.go:351`）还原为 `custom_tool_call`，流式双向识别 `response.custom_tool_call_input.delta/.done` |
| `web_search` / `web_search_preview` / `image_generation` | **静默丢弃**（D3，`convert_request_tools.go:338-345`）                                                                                                                                                                                                                                            |


**问题。** 两点：一是转换器虽有单点入口但产出的是 Chat DTO 而非 IR，无法被其他协议复用；二是 D3 的静默丢弃。

**目标。** 把 `wire/convert` 与 `wire/stream` 收口为 Inbound/Outbound 形态，产出 IR 而非 Chat DTO。`OpenAIWireToolContext` 迁入 `ir.ToolContext`。内建工具按能力矩阵处置（详见阶段 7 矩阵）：`web_search`/`web_search_preview` 在 Claude/Gemini 上游映射到各自原生搜索能力（Claude 的 `ServerToolUse.WebSearchRequests`、Gemini 的 Google Search grounding），在 Chat 上游按渠道配置降级为函数工具或拒绝；`file_search`/`image_generation` 在无等价能力的上游直接拒绝请求（返回 400/404，明确告知「该上游不支持此内建工具」），不再静默丢弃，也不降级为函数工具（见 §8 问题 4 决策）。其他能映射的内建工具（如 `apply_patch` 走 generic custom tool 通道）继续按现状映射。降级行为（含签名跨协议过滤丢弃、其他能力丢失）仅在后台日志记录，不向客户端暴露响应头/响应体扩展字段；降级的实际语义写入用户文档（见 §8 问题 2 决策）。

**验收**：

- `Responses→IR→Responses` 与 `Chat→IR→Chat` roundtrip bit 级等价
- 六类工具各有用例，`apply_patch` 重点验证 `format` 约束经 description 传递后的还原保真，以及流式 `custom_tool_call_input` 分片边界
- 流式 golden：chunk 顺序、`finish_reason`、usage 帧、`[DONE]`、错误帧
- `go test ./internal/relay/wire/... ./internal/relay/ir/...`
- 计费对拍通过（此时应无差异，因为上游行为未变）



### 阶段 2：Claude ↔ Chat

**现状。** 转换算子在 `relay/helper/convert.go`：去程 `ClaudeToOpenAIRequest`，回程 `ResponseOpenAI2Claude`、`StreamResponseOpenAI2Claude`，另有 `NormalizeCacheCreationSplit`、`buildClaudeUsageFromOpenAIUsage` 等辅助。反向（Chat→Claude）在 `channel/claude/relay_claude.go` 的 `RequestOpenAI2ClaudeMessage`、`ResponseClaude2OpenAI`、`StreamResponseClaude2OpenAI`。

流式转换是有状态的：`RelayInfo.ClaudeConvertInfo.LastMessagesType`（`relay/common/relay_info.go` 的 `LastMessageTypeNone/Text/Tools/Thinking` 四态）跟踪上一个内容块类型，用于决定 Claude 事件序列（`content_block_start`/`delta`/`stop`）的边界。

**问题。** 三点：

1. 状态机挂在 `RelayInfo` 上而非转换器内部，跨渠道复用时容易漏初始化（`channel/claude/adaptor.go:104-106` 就有一处手动兜底初始化）
2. 思考签名无 IR 承载。Claude thinking block 的 signature 是上游返回的凭据，后续请求需回传，Chat 没有对应字段
3. usage 归一在 `shared.ClaudeUsageToOpenAIUsage`（正确），但语义标记靠 `FinalRequestRelayFormat` 单点赋值（D4）

**目标。** Claude Inbound/Outbound 各自实现，`LastMessagesType` 状态机迁入流式转换器内部。签名按 §3.4 与内容块绑定承载。Claude 三档缓存字段（`ClaudeCacheCreation5mTokens`、`ClaudeCacheCreation1hTokens` 及默认档）在 IR usage 中保持独立，不可合并——三档倍率不同。

**验收**：

- `Claude→IR→Claude` bit 级保真，含 thinking block 与 signature
- 三档缓存 usage 字段无损，倍率计算与现状一致（对拍）
- 状态机迁移后，连续 tool_use 块、thinking 与 text 混排、空 delta 等边界场景流式输出正确
- AWS 复用路径同步验证（`go test ./internal/relay/channel/aws/...`）
- `go test ./internal/relay/helper/... ./internal/relay/channel/claude/...`



### 阶段 3：Responses ↔ Claude

**现状。** 已通，但是手工拼装：去程 `channel/claude/adaptor.go:94-113` 串联两次转换并手动传递 `ToolContext`、初始化 `ClaudeConvertInfo`、追加 `RequestConversionChain`；回程流式在 `relay_claude.go:992-1005` 走 `writeClaudeChatChunkAsResponsesEvent`（`:1094`）+ `ensureClaudeResponsesStreamConverter`（`:1087`）+ `writeClaudeResponsesFinalEvent`（`:1116`）约 130 行；回程非流式在 `:1205-1218`。

**问题。** 这 130 行胶水是 D2 的典型样本——它做的事情本质上是「组合阶段 1 和阶段 2 的边」，但因为没有 IR 层，只能在渠道内部手写串联。每个想支持 Responses 的渠道都要抄一遍，这就是 Gemini/Vertex/AWS 至今 `not implemented` 的原因。

**目标。** 不新增任何点对点转换代码，纯粹由 `Responses↔IR`（阶段 1）与 `IR↔Claude`（阶段 2）组合得到。删除上述胶水。

**验收**：

- 现有 Claude + Responses 行为不回退（用现网请求样本对拍）
- 内建工具按能力矩阵处置：`web_search`/`web_search_preview` 在 Claude/Gemini 上游映射到原生搜索能力，在 Chat 上游按渠道配置降级或拒绝；`file_search`/`image_generation` 在无等价能力上游返回 400/404 错误不再静默丢弃；其他内建工具（如 `apply_patch`）继续按现状映射
- 降级事件（含工具拒绝、签名过滤丢弃等）写入后台日志，不向客户端暴露响应头/响应体扩展字段
- 删除的胶水代码无残留引用：`rg "writeClaudeChatChunkAsResponsesEvent|ensureClaudeResponsesStreamConverter|writeClaudeResponsesFinalEvent" internal/` 无输出
- `go test ./internal/relay/...`



### 阶段 4：Gemini ↔ Chat

**现状。** 去程 `helper.GeminiToOpenAIRequest`，回程 `helper.ResponseOpenAI2Gemini`、`helper.StreamResponseOpenAI2Gemini`。流式状态在 `RelayInfo.GeminiConvertInfo`，辅助函数 `ensureGeminiConvertInfo`、`ensureGeminiChoiceToolCallState`。

思考签名现状最脏：`channel/gemini/relay_gemini.go:32` 有一个绕行常量 `thoughtSignatureBypassValue`（值为 `context_engineering_is_the_way_to_go`，出处已查证为参考项目 axonhub `llm/transformer/gemini/thinking_signature.go:10`，依据 Gemini 3 迁移文档 `https://ai.google.dev/gemini-api/docs/gemini-3#migrating_from_other_models` 的「function call 没签名给默认值」实践，非 Google 官方文档明文推荐的占位符）和 `parseFunctionCallThoughtSignature`，仅在渠道类型为 Gemini/VertexAi **且**客户端显式传 `extra_body.google.thought_signature` 时，把签名附加到 assistant 消息的 functionCall/文本 part。这是既有 workaround，不是设计。

Gemini 还有一个特殊性：上游会把 429/5xx 转成 HTTP 200 + `{"error":{...}}` 下发，`domain/shared/gemini.go:558` 的 `GeminiChatResponse.Error` 字段专门保留这个载荷供 handler 识别真实错误。

**问题。** 除 §3.4 的签名承载缺失外，还有 D5 的一半：Gemini 客户端收到 OpenAI 格式错误（本阶段只做转换层，错误出口在阶段 9.3 处理，但本阶段需确保 IR 能承载 Gemini 错误载荷）。

**目标。** Gemini Inbound/Outbound 实现，`GeminiConvertInfo` 状态机迁入转换器内部。`thoughtSignature` 按 §3.4 承载，并补齐工具调用级签名（借鉴 axonhub 三层方案）：

1. **IR 层**：签名元数据可挂到工具调用块（对应 axonhub `ToolCalls[j].TransformerMetadata["google_thought_signature"]`），与消息级签名并存；跨提供者识别器（`GuessSignatureProvider` 等价物）放本层 `signature.go`，各协议 Outbound 复用——**识别策略为正向白名单 + unknown 丢弃**：只转发被正向识别为目标提供者的签名，识别不出（unknown）一律丢弃。此策略安全等价于一律丢弃（误判只丢不漏），依据为 axonhub `shared/README.md:74`；目的是防止下游模型尝试解密别家 blob 触发 `invalid_request_body` / `encrypted content could not be verified` 硬失败。
2. **Chat DTO 层**：tool_call 仿照现有 `reasoning_signature` 私有扩展的做法，增加签名字段（对应 axonhub `extra_content.google.thought_signature`），使「Chat 客户端 + Gemini 上游」的多轮 function calling 能靠客户端回传签名走通，不再依赖魔法值；回程 Gemini→Chat 时从 functionCall part 提取签名写入该字段。
3. **bypass 值的编码对齐（落地必读）**：现状 `thoughtSignatureBypassValue` 经 `strconv.Quote` 包成带引号的 `json.RawMessage`（`relay_gemini.go:131`），**未做 base64 编码**；axonhub 同名常量是 `base64.StdEncoding.EncodeToString([]byte("context_engineering_is_the_way_to_go"))`（真 base64）。引入识别器前必须先统一 bypass 的编码方式——否则自家 bypass 值会被自家识别器判定为 `unknown`（非合法标准 base64）而丢弃，bypass 失效。统一为真 base64 编码（对齐 axonhub），识别器无需对 bypass 特判。
4. **Anthropic 兼容出站例外**：axonhub 对非 Anthropic 官方平台（OpenRouter 等 Anthropic 兼容端点）的出站**不做解码、原样转发**（`shared/README.md:76`）。本仓库 Claude Outbound 需复用此例外——已有雏形（`channel/openai/adaptor.go:220` 的 OpenRouter Anthropic 端点 Bearer 认证特判），实施时扩展为「OpenRouter 等 Anthropic 兼容平台跳过签名过滤」。
5. **Gemini 上游未给签名也兜底**：除「客户端未回传签名时兜底」外，**Gemini Outbound 在上游本轮响应未携带 thoughtSignature、但本轮产生了 function call 时也补 bypass 值**（对齐 axonhub `gemini/outbound_convert.go:434-444`：tool call 存在但签名缺失即补默认值，引用同上 Gemini 3 迁移文档）。理由：Gemini 3 多轮 function calling 校验链要求 assistant turn 携带签名，缺签名会在下一轮报 `Function call is missing a thought_signature`，补 bypass 是兜住模型不报错，不是伪造凭据。

HTTP 200 携带错误载荷的识别逻辑保留，纳入 Outbound 的响应解析。

**验收**：

- `Gemini→IR→Gemini` bit 级保真，含 `thoughtSignature`
- Chat 客户端 + Gemini 上游的多轮 function calling：工具调用签名经 Chat DTO 私有字段往返传递；bypass 值经真 base64 编码对齐，能被识别器放行
- Gemini 上游本轮产生 function call 但未回传签名时，Outbound 补 bypass 值（与 axonhub 行为对齐），下一轮不触发 `missing thought_signature` 400
- Anthropic 兼容出站（OpenRouter 等）跳过签名过滤，原样转发
- HTTP 200 + error 载荷仍被正确识别为上游错误（不能被当成正常响应计费）
- Gemini audio token 计费不变（`GetGeminiInputAudioPricePerMillionTokens` 路径对拍）
- `go test ./internal/relay/channel/gemini/... ./internal/relay/helper/...`



### 阶段 5：Responses ↔ Gemini

**现状。** 不通。`channel/gemini/adaptor.go:229` 与 `channel/vertex/adaptor.go:300` 均为 `not implemented`（D1）。

**问题。** 零件齐备但无人组合，同阶段 3 的成因。

**目标。** 由阶段 1 与阶段 4 的边组合得到，消除两处 `not implemented`。同时处理 Gemini 原生 Google Search grounding 与 Responses `web_search` 的映射：Responses 客户端声明 `web_search` 时，映射到 Gemini 的 grounding 能力；计费按实际执行方（Gemini）取价，对应 P5。

**验收**：

- Responses 客户端可正常使用 Gemini 渠道（流式与非流式）
- Vertex 同步打通
- grounding 计费正确：`gemini_web_search_requests` 计数经 IR 传递，`GetToolBillingPrice(provider="gemini")` 取价（对拍）
- `rg "not implemented" internal/relay/channel/{gemini,vertex}/adaptor.go` 无 Responses 相关残留
- `go test ./internal/relay/channel/gemini/... ./internal/relay/channel/vertex/...`



### 阶段 6：Gemini ↔ Claude

**现状。** 部分通且实现分散：Gemini 客户端 → Claude 上游在 `channel/claude/adaptor.go:26` 经 `helper.GeminiToOpenAIRequest`；Claude 上游 → Gemini 客户端在 `channel/claude/relay_claude.go:1006-1027`（流式）与 `:1220-1229`（非流式）。反向（Claude 客户端 → Gemini 上游）在 `channel/gemini/adaptor.go:46` 经 `helper.ClaudeToOpenAIRequest`。

签名现状是**原值互塞**：Gemini 的 `thoughtSignature` 经 Chat 的 `reasoning_signature` 中转后塞进 Claude thinking block 的 `signature`，反向同理（每条消息只保留一个非空签名，多块场景有信息损失）。现有测试固化了这一透传行为（`claude/adaptor_test.go`、`gemini/adaptor_test.go` 均断言签名原样到达对方签名字段）。

**问题。** 这是唯一一条两端都非 Chat 的边。两套签名是互不兼容的上游凭据，无任何厂商规范定义跨厂商签名语义——互塞的签名对目标上游只是无效字节（被拒绝或被忽略，行为未定），属已知的脏行为，现有测试把脏行为固化成了契约。

**决策：按提供者识别过滤。** 跨协议边只转发「正向识别为目标提供者」的签名，其余（含 unknown）丢弃并记 `ir.Capability`（本决策借鉴 axonhub `GuessSignatureProvider` 启发式：OpenAI `gAAA*` 前缀 / Anthropic `EqQ*` 系前缀 / Gemini base64+protobuf 特征；识别器放 IR 层 `signature.go`，各协议 Outbound 复用，正向白名单 + unknown 丢弃，安全等价于一律丢弃，依据 axonhub `shared/README.md:74`）。这是**行为翻转**——从现状的互塞翻转为按归属过滤：自家签名自家协议内保真，跨协议不再互塞无效凭据。Anthropic 兼容出站（OpenRouter 等）跳过过滤、原样转发，对齐 axonhub 例外（`shared/README.md:76`）。

**目标。** 由阶段 2 与阶段 4 的边组合得到。签名按上述过滤策略处理，识别器与承载结构落 IR。删除 `relay_claude.go` 的 Gemini 分支胶水。

**验收**：

- 双向可用（流式与非流式）
- 过滤行为正确：`Gemini→IR→Claude` 与 `Claude→IR→Gemini` 用例断言——目标提供者自己的签名原样透传，非目标提供者签名被丢弃且 `ir.Capability` 有降级记录；**现有断言互塞的测试同步翻转为断言过滤**
- 同协议往返 bit 级保真（多 thinking 块/多 part 的签名不丢失；多块合并为单签名的信息损失仅存在于跨协议路径，维持现状）
- 识别器对三家真实签名样本（含多轮工具调用）识别率验证通过，未识别（unknown）一律丢弃
- 删除的胶水无残留引用
- `go test ./internal/relay/...`



### 阶段 7：内建工具全矩阵复核

**现状。** 服务端工具的计数与计价分三套互不相通的机制：


| 机制                                    | 位置                                                                                                                | 覆盖范围             |
| ------------------------------------- | ----------------------------------------------------------------------------------------------------------------- | ---------------- |
| `ResponsesUsageInfo.BuiltInTools` map | `relay/common/relay_info.go:382-388` 按请求声明初始化（CallCount=0），`channel/openai/relay_responses.go:81,145,149` 按上游事件递增 | 仅原生 Responses 上游 |
| gin 字符串键 `claude_web_search_requests` | `channel/claude/relay_claude.go:960,1233` 从 Claude usage 的 `ServerToolUse.WebSearchRequests` 写入                   | 仅 Claude 上游      |
| gin 字符串键 `gemini_web_search_requests` | 由 Gemini 侧写入                                                                                                      | 仅 Gemini 上游      |


计价在 `domain/billing/usage.go:140-211`，三套分支各自读上述来源，`GetToolBillingPrice` 按 `provider` 参数（openai/claude/gemini）区分单价。

**问题。** 三点：

1. **跨协议组合下计数必然丢失。** 例如 Responses 客户端 + Claude 上游：`BuiltInTools` 被请求声明初始化，但 Claude 上游不会返回 `response.output_item.done` 事件，CallCount 永远为 0。当前之所以没造成错账，是因为 D3 把工具丢弃了，上游根本没执行——两个 bug 互相掩盖。阶段 1 修掉 D3 之后，如果计数机制不统一，就会变成真实漏计费。
2. **gin 字符串键无类型约束**，拼错静默失效。
3. **能力矩阵无文档**，哪些组合支持搜索、哪些降级，只能靠读代码推断。

**目标。** 落地 P4 与 P5：服务端工具调用记录升为 IR 一等字段，由各协议 Outbound 负责把自己的上游事件/usage 字段映射进来；计费层只读 IR，按记录中的实际执行方取价。

产出完整能力矩阵文档，每格明确三种结果之一：原生映射、降级为函数工具、拒绝请求。


| 客户端声明 → 上游协议       | Chat                    | Claude                  | Gemini                     | Responses |
| ------------------ | ----------------------- | ----------------------- | -------------------------- | --------- |
| `web_search`       | 降级为函数工具（客户端执行）或拒绝，按渠道配置 | 映射 Claude 原生 web search | 映射 Google Search grounding | 原生        |
| `file_search`      | 拒绝                      | 拒绝                      | 拒绝（注：Gemini 有独立 File Search API，未来可另立 PRD 接入） | 原生        |
| `image_generation` | 拒绝                      | 拒绝                      | 拒绝                         | 原生        |


矩阵每格的最终结论以阶段执行时查证的上游官方文档为准，上表是待验证草案。

**验收**：

- 矩阵每格有测试用例
- 跨协议组合的工具计数不丢：Responses 客户端 + Claude 上游声明 `web_search` 映射到 Claude 原生 web search，按 Claude 单价计费（对拍）；`file_search`/`image_generation` 在无等价能力上游直接 400 不计费
- gin 字符串键旁路清除：`rg "claude_web_search_requests|gemini_web_search_requests" internal/` 仅在兼容层出现或无输出
- `go test ./internal/domain/billing/... ./internal/relay/...`



### 阶段 8：计费专项复核

**现状。** 见 §4.1 的类型 A 与类型 B。

**问题。** 这是本次重构最容易埋雷的地方：前七个阶段每改一条边，都可能改变喂给 `CalculateUsage` 的数据。阶段 0.3 的对拍框架就是为此准备的，但对拍只能发现偏差，不能保证语义正确——需要本阶段做系统性收口。

**目标。** 逐条落地 §4.2 的 P1–P5，并处理 §4.3 的三处耦合：

1. **删除** `isClaudeUsageSemantic` **分支**（`usage.go:108` 及其在 `BuildContextPricingUsage` 的传参，连同 `PostClaudeConsumeQuota` 中 `quota.go:304` 基于 `ChannelType` 的独立同名判定）。前置条件是所有 Outbound 已按 P1 归一 usage 语义，届时这些分支恒为一个值，属死代码。这同时根治 D4——不再依赖渠道记得打标。
2. **逐渠道计费读取收口（对应 §4.1.3）**：按 §4.1.3 对照表，把每家上游原值字段读入 IR usage 后落库，计费层 `CalculateUsage` 只做乘法（按缓存倍率读独立缓存字段），不做语义猜测与加减。`shared.ClaudeUsageToOpenAIUsage`/`OpenAIUsageToClaudeUsage` 的「合并-还原」往返收缩为 Claude Outbound 内部的字段搬运，不得再作为全局 usage 中间口径。
3. **重新定义空 usage 重试判定**（`quota.go:33-38`）。从「转换链长度 > 1」改为「非上游原生直通」，复用 §3.3 的直通判定。
4. **原始请求体快照独立化**（P3）。确保 token 估算与日志读的是用户原始输入，不受 `setTemporaryRequestBody` 窗口影响。

**验收**：

- 全渠道 × 全协议组合计费对拍全绿（这是本阶段核心验收项）
- 每渠道 IR usage 各字段与上游官方 response 原值逐字段一致（§4.1.3 对照表 + 阶段 0.3 补充断言）
- Claude 三档缓存、Gemini audio token、Responses 内建工具、OpenRouter 缓存创建（`CalcOpenRouterCacheCreateTokens`）逐项回归
- 空 usage 重试语义符合预期：原生直通请求空 usage 应重试，跨协议转换请求不重试
- 死代码清除：`rg "isClaudeUsageSemantic" internal/` 无输出
- `go test ./internal/domain/billing/...` 含 `usage_test.go` 全部用例



### 阶段 9：三家协议错误处理专项

错误路径与转换路径同层但独立收口，按协议拆三个子阶段。共同前提：`ir.Error`（阶段 0.2 已定义骨架）作为唯一内部错误模型，各协议 Inbound 负责 IR→客户端格式，Outbound 负责上游格式→IR。

**错误模型策略：以 Chat 错误为中枢，跨协议只保 code+message。** `ir.Error` 不做协议特化的强类型展开——三家协议的错误体结构虽有差异（OpenAI 的 `error.{message,type,code}`、Claude 的 `error.{type,message}` 且 `type` 是固定枚举、Gemini 的 `error.{code,message,status}` 且 `status` 是 gRPC 枚举），但网关层需要跨协议传递的核心信息只有 HTTP 状态码、错误码、错误消息三者。`ir.Error` 以 Chat 错误结构（`message` + `code` + `http_status`）为内部表示，各协议 Inbound/Outbound 负责自家枚举与这三者的双向映射：

- Chat ↔ IR：直通（Chat 是中枢）
- Claude ↔ IR：`type` 枚举 ↔ `code`，`message` 直传
- Gemini ↔ IR：`status` 枚举 ↔ `code`（上游已有 status 时透传，无则按 HTTP 码逆射，见 9.3），`message` 直传

不做「把 Claude 的 `type` 完整还原给 Gemini 客户端」之类的协议特化还原——跨协议场景下客户端拿到的是目标协议合法的错误体（含可读 message），足够定位问题。

三家共有的两个问题先说清楚：

**共有问题 1：出口分发重复两处。** `httpapi/controller/relay/relay.go:98-110` 按 `RelayFormat` 分发；`httpapi/middleware/performance.go:20-35` 按 URL 前缀 `/v1/messages` 分发。两处判定依据不同，仅靠路径约定保持一致，改一处漏另一处。

**共有问题 2：脱敏分散三处。** `domain/shared/error.go` 的 `ToOpenAIError`、`ToClaudeError` 各调一次 `MaskSensitiveInfoWithExemptions`，`relay/helper/error.go:47` 的 `ClaudeErrorWrapper` 另有一套手写替换逻辑（把含 post/dial/http 的错误文本替换为「请求上游地址失败」）。

#### 9.1 OpenAI 系错误（Chat + Responses）

**现状。** 入站解析在 `relay/helper/error.go:71` 的 `RelayErrorHandler`：读上游错误体，尝试解析为 `shared.GeneralErrorResponse`，再用 `TryToOpenAIError()`（`domain/shared/error_response.go:40`）提取。解析失败则退化为 body 文本拼接。出站是 `NookMuxError.ToOpenAIError()`（`error.go:197`）。

上游状态码还原有专门逻辑：`helper/error.go` 的 `UpstreamErrorStatusCode`、`ResetStatusCode` 与 `upstreamErrorStatusCodeByCode` 映射表，`NookMuxError.OriginalStatusCode` 保留渠道级状态码映射前的原始值（用于重试与渠道禁用判定）。

**问题。** OpenAI 系是当前的「归一中心」，本身问题最少。主要问题是它承担了所有非 Claude 格式的兜底出口——包括 Gemini（见 9.3）。另外 `GeneralErrorResponse.Error` 是 `json.RawMessage`，`TryToOpenAIError` 只处理 object 类型，string/number 类型的 `error` 字段走 `ToMessage()` 降级为纯文本，丢失结构。

**目标。** `RelayErrorHandler` 拆为 OpenAI Outbound 的 `TransformError`（上游 OpenAI 格式 → `ir.Error`），出口拆为 OpenAI Inbound 的 `TransformError`（`ir.Error` → OpenAI 格式）。`ir.Error` 保留 `OriginalStatusCode` 与错误码，确保重试与渠道禁用判定行为不变。Responses 格式的错误出口与 Chat 共用（同属 OpenAI 系，错误体结构一致）。

**验收**：

- 上游各类错误体（标准 object、error 为 string、error 为 number、非 JSON body、空 body）解析结果与现状一致
- `StatusCode` / `OriginalStatusCode` 映射不变，重试与渠道自动禁用判定回归
- `go test ./internal/relay/helper/... ./internal/domain/shared/...`



#### 9.2 Claude 错误

**现状。** 上游错误识别在 `channel/claude/relay_claude.go:1184-1187`：从响应体取 `GetClaudeError()`，用 `shared.ClaudeErrorStatusCode(claudeError.Type)` 按 Anthropic 官方文档把错误类型还原为真实 HTTP 状态码（注释明确说明这是为了保持重试与渠道禁用判断准确）。流式错误拦截另有 `stream_error_interception_test.go` 覆盖。

出站是 `ToClaudeError()`（`error.go:230`）。本地错误包装是 `helper/error.go:47` 的 `ClaudeErrorWrapper`。

**问题。** 跨格式映射错误：`ToClaudeError()` 在源错误类型为 OpenAI 时，执行 `Type: fmt.Sprintf("%v", openAIError.Code)`——把 OpenAI 的 `code`（如 `invalid_model`、数字 400）塞进 Anthropic 的 `type` 字段。而 Anthropic 的 `type` 是固定枚举（`invalid_request_error`、`authentication_error`、`rate_limit_error`、`api_error`、`overloaded_error` 等），塞入非枚举值会导致严格按 Anthropic 规范解析的客户端行为异常。

反向也有问题：`ToOpenAIError()` 在源类型为 Claude 时执行 `Type: claudeError.Type`，把 `overloaded_error` 这种 Anthropic 专有类型原样丢给 OpenAI 客户端。

**目标。** 建立显式的错误类型映射表（HTTP 状态码 + 语义 → 各协议合法枚举值），替代当前的字段直塞。映射表要双向且只产出目标协议的合法枚举值。`ClaudeErrorStatusCode` 的状态码还原逻辑保留并纳入 Claude Outbound。`ClaudeErrorWrapper` 的文本替换逻辑并入统一脱敏单点。

**验收**：

- Claude 上游各类错误类型的状态码还原与现状一致
- 跨格式映射产出合法枚举：`OpenAI→ir.Error→Claude` 的 `type` 字段断言在 Anthropic 枚举集合内；`Claude→ir.Error→OpenAI` 同理
- 流式错误拦截行为不变（`stream_error_interception_test.go` 通过）
- `go test ./internal/relay/channel/claude/...`



#### 9.3 Gemini 错误

**现状。** 这是三家里问题最大的一家，D5 的主体。

已经铺好的底子：`domain/shared/error.go:32` 定义了 `ErrorTypeGeminiError = "gemini_error"`；`domain/shared/gemini.go:562` 定义了 `GeminiErrorResponse{Code int, Message string, Status string}`，即 Gemini API 标准错误载荷；`GeminiChatResponse.Error`（`gemini.go:558`）专门保留 HTTP 200 中携带的错误载荷。

没接上的部分：`ErrorTypeGeminiError` 全仓库零使用；`NookMuxError` 没有 `ToGeminiError()` 方法；出口分发（`relay.go:98-110`）只有 Realtime/Claude/default 三路。

**问题。** Gemini 客户端报错时走 default 分支，收到 OpenAI 格式错误体：

- 期待 `{"error":{"code":429,"message":"...","status":"RESOURCE_EXHAUSTED"}}`
- 实际 `{"error":{"message":"...","type":"...","code":"..."}}`

`status` 字段完全缺失，`code` 类型从数字变字符串。按 Gemini 规范解析的客户端会拿到空 `status` 或解析失败。这不是「不够优雅」，是协议不兼容。

**目标。** 补齐 Gemini 错误出口：新增 `ToGeminiError()`（或 IR 化后的 Gemini Inbound `TransformError`），启用 `ErrorTypeGeminiError`，出口分发增加 Gemini 分支。`status` 字段映射策略（见 §8 问题 1 决策）：上游本来就是 Google API（响应体含 `error.status`）时原样透传 status 字符串；只有上游是 OpenAI/Anthropic 错误体（无 `error.status`）时才按 HTTP 状态码用映射表逆射。逆射映射表依据 Google API 统一错误模型规范（17 个 gRPC status 枚举，REST 多对一映射到 HTTP），本阶段实施时写入代码。HTTP 200 携带错误载荷的识别逻辑纳入 Gemini Outbound。

**验收**：

- Gemini 客户端收到符合 Gemini 规范的错误体：`code` 为数字、`status` 为合法枚举、字段结构正确
- `ErrorTypeGeminiError` 已被使用：`rg "ErrorTypeGeminiError" internal/` 有实际引用
- HTTP 200 + error 载荷仍被识别为错误且不计费
- 出口分发合并为单点，`relay.go` 与 `performance.go` 不再各写一份
- `go test ./internal/relay/channel/gemini/... ./internal/httpapi/...`



### 阶段 10：实机测试与收尾

**现状。** 前九个阶段的验收都基于单元测试与对拍，未经真实上游验证。

**问题。** 协议转换的边界情况（上游返回非文档化字段、流式分片位置异常、超长内容截断行为、真实网络中断）无法靠单测覆盖。

**目标。** 三件事：

1. **实机矩阵验证。** 每个 Inbound × 每个 Outbound 组合，流式与非流式各跑一遍真实请求。矩阵规模为 4 × 4 × 2 = 32 个组合（Chat/Responses/Claude/Gemini 四种客户端格式 × 四种上游协议 × 流式/非流式），逐格记录结果。`file_search`/`image_generation` 声明到无等价能力上游的组合直接返回 400（见阶段 1 决策），单独跑一遍确认错误码与消息即可，不纳入转换矩阵。
2. **旧代码残留扫描。** 旧代码不保留、不设 deprecated 观察期——各阶段重写完成时立即物理删除该阶段涉及的旧代码（`relay/helper/convert.go` 的协议转换函数、各渠道 `Convert*Request` 的冗余实现、各渠道 `DoResponse` 的四路 switch、`wire/auto_convert.go` 的快照恢复机制等随对应阶段删除）。阶段 10 只做全仓库残留扫描，确认无遗漏。
3. **文档同步。** `internal/relay/AGENTS.md` 的单点入口规则从「chat↔responses 必须走 `convert.NewConverter`」推广为「所有协议转换必须走 IR 边」；依赖方向段落按新包结构更新；`docs/` 下涉及协议转换的开发文档同步。

**Realtime 处置。** `relay/core/websocket.go` 的 `WssHelper` 目前是独立入口，不走 adaptor 的 Convert 方法，计费走 `PreWssConsumeQuota`/`PostWssConsumeQuota`。**结论（见 §8 问题 5 决策）：Realtime 不纳入本 PRD 范围**——WebSocket 的双向流模型与 IR 的请求-响应模型不匹配，纳入会让 IR 抽象复杂度大幅上升。Realtime 继续走现有 WebSocket 旁路，不在 IR 边抽象内；未来如需统一另立 PRD。本阶段不涉及 Realtime 代码改动。

**验收**：

- 32 格实机矩阵全绿（拒绝路径单独记录），逐格留记录（含请求样本与响应快照）
- `go build ./...` 与 `go test ./...` 全绿
- 旧代码残留扫描无遗漏：`rg "relay/helper/convert\.|ConvertClaudeRequest|ConvertGeminiRequest|ConvertOpenAIRequest|ConvertOpenAIResponsesRequest" internal/` 仅命中 IR 边或无输出
- `gofmt -l` 与 `go vet ./...` 干净
- `internal/relay/AGENTS.md` 与代码一致
- Realtime 处置结论已写入本文档



## 6. 风险与缓解


| 风险                      | 影响             | 缓解                                                                                   |
| ----------------------- | -------------- | ------------------------------------------------------------------------------------ |
| 流式状态机迁移导致 chunk 顺序或事件错乱 | 客户端流式输出异常      | 每阶段流式 golden 测试（阶段 0.3 框架 2）；复用既有 `wire/stream` 实现而非重写（§2.2 第 3 点）                   |
| 思考签名跨协议保真失败             | 上游拒绝后续请求，推理链断裂 | 阶段 2/4 要求同协议 bit 级 roundtrip；阶段 6 跨协议按提供者识别过滤（正向白名单，误判只丢不漏），丢弃记 `ir.Capability`；上线前用三家真实签名样本验证识别率 |
| 计费对拍覆盖不全导致偏差            | 直接资金影响         | 阶段 0.3 先建对拍框架再动转换代码；阶段 8 全组合对拍；上线分渠道灰度                                               |
| 同协议直通被 IR 往返破坏          | 延迟上升，保真风险      | §3.3 直通判定，同协议跳过 IR                                                                   |
| 阶段 1 修掉 D3 后暴露漏计费       | 服务端工具漏计费       | D3 与工具计数缺失当前互相掩盖（阶段 7 问题 1）；阶段 1–7 代码逐阶段合入主干但生产发布整体压至阶段 7 完成后一次性上线（见 §5 发布约束） |
| 空 usage 重试判定全局失效        | 上游异常时不再重试      | 阶段 8 明确改为直通判定，并补用例覆盖两种分支                                                             |
| 存量线上用户回归                | 服务中断           | 旧代码随各阶段重写完成时立即删除（不保留 deprecated 观察期）；阶段 1–7 生产发布整体压至阶段 7 完成后一次性上线（见 §5 发布约束），给存量用户一个完整迁移窗口 |
| IR 表达力不足                | 上游能力丢失         | 以 Chat 规范为基础的扩展 IR + §3.4 三类概念显式承载；`ir.Capability` 记录降级。**禁止**用通用 `extra map` 承载已知协议概念，避免退化为垃圾桶 |




## 7. 完成标准

- 全部协议互转组合可用，无 `not implemented` 残留
- 每个 Inbound × Outbound 组合有 roundtrip 与流式 golden 测试
- 内建工具能力矩阵文档化，每格结论明确，无静默丢弃
- 思考签名同协议往返 bit 级保真，跨协议按提供者识别过滤（非目标提供者签名丢弃并记 `ir.Capability`）
- 三家协议错误出口符合各自规范，Gemini 客户端收到 Gemini 格式错误
- 计费对拍全绿；计费取数不依赖转换后请求体；`isClaudeUsageSemantic` 类语义分支已删除
- 旧协议转换与胶水代码物理删除
- `go build ./...`、`go test ./...`、`gofmt -l`、`go vet ./...` 全部干净
- `internal/relay/AGENTS.md` 已更新



## 8. 待决问题

1. **BaseURL 双重前缀 bug 是否真实存在**：原始描述来自一份已不在仓库内的 SDK 迁移 PRD，无原始依据可核对，已移出本 PRD 范围；若后续有用户报告，独立立项复现再修。

2. **D4 计费基准疑点**：`isClaudeUsageSemantic=true` 的公式假设（PromptTokens 不含缓存）与所有设置该标记路径实际传入的归一后 usage 语义（含缓存）存在矛盾——Claude 直连渠道可能存在缓存 token 双计（多收），需阶段 0.3 对拍以真实数字确认；结论决定阶段 0.1 bug 1 的修复方向，若确认双计，需独立评估存量账目影响。

3. **本 PRD 暂时搁置，前置依赖计费 PRD 先落地**：当前 IR 选型为「以 Chat 规范为基础的扩展 IR」，但实际只有 Responses↔Chat 一条转换有现成经验，其余协议组合均为空白。基于单一经验在 Chat 上做扩展，维护成本高于新建中立 IR——Claude→Responses 等跨协议路径会反复碰到 Chat 层不兼容问题，每条边都要为 Chat 底座的缺陷打补丁。同时，本 PRD 将协议转换层 IR 化与计费语义重定义捆绑推进（§4 计费专项复核、阶段 0.1/0.3/8），而计费基准本身存疑（见问题 2），在基准未确认前改计费语义风险过高。决策：本 PRD 暂时搁置，先独立编写计费修复 PRD 解决问题 2 与现有计费偏差；计费 PRD 落地后重启本 PRD，届时 IR 选型重新评估，倾向全新中立 IR。

