# PRD：计费 Usage 归一化与结构化落库

## 1. 背景

### 1.2 目标

在计费和落库前增加一次明确的语义归一化：

1. 引入内部的 `BillingUsage` 语义模型，字段按“普通输入、输出、缓存读取、缓存写入、audio、text、image、reasoning”等语义命名。
2. 供应商归一化只发生一次：上游原值进入 `BillingUsage` 后，计费、日志结构化、前端展示都不再按供应商协议加加减减。
3. `Log` 只新增一个专用 `billing_details` JSON 列，存放本次请求的归一化 Token 用量；不新增一串 token 独立列，也不存放 quota、价格、倍率或 service tier。它服务普通 relay 和 audio/realtime/wss 等有 Token 用量的消费入口。
4. 现有 `Quota`、`PromptTokens`、`CompletionTokens` 作为兼容和统计聚合列保留。`PromptTokens` 明确定义为“输入侧处理总量”兼容口径，不再代表普通输入。
5. 前端使用日志只读 `billing_details` 获取 Token 明细；历史 Token 明细由启动前置迁移回填，聚合统计列继续保留。

### 1.3 非目标

1. 不修改客户端可见的 OpenAI、Claude、Gemini 响应体 usage 语义。
2. 不改写计费结果、价格快照、倍率和诊断数据；历史行只迁移 Token 明细并从 `Other` 移除已接管字段。
3. 不把 `Other` JSON 整体删除；`Other` 继续承载价格、倍率、诊断和管理排障信息。
4. 不在本 PRD 中新增面向 SQL 查询的 token 明细列或 JSON 查询索引。
5. 不把 quota、价格、倍率、service tier、工具费、图片生成调用费、文件检索费、定制音色按次/按字符费、模型测试费和违规费迁入 `billing_details`。计费结果和计价依据继续保留在现有位置。
6. 不在 `billing_details` 中重复 `Log` 已有的用户、渠道、模型、入口类型和时间等元数据；供应商/协议来源、usage 来源、请求和上游格式、转换链继续属于日志诊断字段或现有 `Other`。

## 3. 统一 Usage 字段映射

### 3.1 语义映射表

| 语义 | 统一字段 | Claude Messages 官方来源 | OpenAI Chat Completions 官方来源 | OpenAI Responses 官方来源 | Gemini 官方来源 | 口径与归一化 |
| --- | --- | --- | --- | --- | --- | --- |
| 普通输入 | `InputTokens` | `input_tokens` | `prompt_tokens - prompt_tokens_details.cached_tokens - prompt_tokens_details.cache_write_tokens` | `input_tokens - input_tokens_details.cached_tokens - input_tokens_details.cache_write_tokens` | `promptTokenCount - cachedContentTokenCount` | 只扣除缓存读取和缓存写入；audio/image/text 等模态明细不默认从普通输入中二次扣除 |
| 输出总量 | `OutputTokens` | `output_tokens` | `completion_tokens` | `output_tokens` | `candidatesTokenCount` + `thoughtsTokenCount` | 保持官方输出总量；reasoning、audio、text 等是拆分或审计维度，不额外累加 |
| 缓存读取 | `CacheReadTokens` | `cache_read_input_tokens` | `prompt_tokens_details.cached_tokens` | `input_tokens_details.cached_tokens` | `cachedContentTokenCount` | 属于缓存维度；OpenAI 与 Gemini 中已包含在 raw 输入总量内，Claude 中与 `input_tokens` 相加 |
| 缓存写入 | `CacheWriteTokens` | `cache_creation_input_tokens` | `prompt_tokens_details.cache_write_tokens` | `input_tokens_details.cache_write_tokens` | 无标准字段 | 与缓存读取分列；无分档时只填总量 |
| 缓存写入 5 分钟 | `CacheWrite5mTokens` | `cache_creation.ephemeral_5m_input_tokens` | 无标准字段 | 无标准字段 | 无标准字段 | 只在官方明确返回分档时写入 |
| 缓存写入 1 小时 | `CacheWrite1hTokens` | `cache_creation.ephemeral_1h_input_tokens` | 无标准字段 | 无标准字段 | 无标准字段 | 只在官方明确返回分档时写入 |
| 输入音频 | `AudioInputTokens` | 无标准字段 | `prompt_tokens_details.audio_tokens` | 无标准字段 | `promptTokensDetails[modality=AUDIO].tokenCount` | 模态维度明细；不默认与 cache 或普通输入互斥 |
| 输入图像 | `ImageInputTokens` | 无标准字段 | `prompt_tokens_details.image_tokens` | 无标准字段 | `promptTokensDetails[modality=IMAGE].tokenCount` | 同上 |
| 输入文本 | `TextInputTokens` | 无标准字段 | `prompt_tokens_details.text_tokens` | 无标准字段 | `promptTokensDetails[modality=TEXT].tokenCount` | 同上 |
| 输入视频 | `VideoInputTokens` | 无标准字段 | 无标准字段 | 无标准字段 | `promptTokensDetails[modality=VIDEO].tokenCount` | Gemini 模态枚举支持；仅作审计或官方差异化计价依据 |
| 输入文档 | `DocumentInputTokens` | 无标准字段 | 无标准字段 | 无标准字段 | `promptTokensDetails[modality=DOCUMENT].tokenCount` | 同上 |
| 输出音频 | `AudioOutputTokens` | 无标准字段 | `completion_tokens_details.audio_tokens` | 无标准字段 | `candidatesTokensDetails[modality=AUDIO].tokenCount` | 输出总量子集或官方拆分；不额外累加 |
| 输出图像 | `ImageOutputTokens` | 无标准字段 | 无标准字段 | 无标准字段 | `candidatesTokensDetails[modality=IMAGE].tokenCount` | 同上 |
| 输出文本 | `TextOutputTokens` | 无标准字段 | `completion_tokens_details.text_tokens` | 无标准字段 | `candidatesTokensDetails[modality=TEXT].tokenCount` | 同上 |
| 输出推理 | `ReasoningTokens` | `output_tokens_details.thinking_tokens` | `completion_tokens_details.reasoning_tokens` | `output_tokens_details.reasoning_tokens` | `thoughtsTokenCount` | 全部是输出总量子集，不得加回 `OutputTokens` |
| 接受预测 | `AcceptedPredictionTokens` | 无标准字段 | `completion_tokens_details.accepted_prediction_tokens` | 无标准字段 | 无标准字段 | 仅透传为审计拆分 |
| 拒绝预测 | `RejectedPredictionTokens` | 无标准字段 | `completion_tokens_details.rejected_prediction_tokens` | 无标准字段 | 无标准字段 | 仅透传为审计拆分 |
| 工具输入 | `ToolUsePromptTokens` | 无标准字段 | 无标准字段 | 无标准字段 | `toolUsePromptTokenCount` | 仅作 Gemini 独立审计字段；官方总量公式未把它加入，不纳入输入总量、TPM 或处理总量 |
| 输入总量 | `PromptAggregateTokens` | `input_tokens` + `cache_creation_input_tokens` + `cache_read_input_tokens` | `prompt_tokens` | `input_tokens` | `promptTokenCount` | 保持 raw 输入总量；OpenAI 和 Gemini 已包含 cache，Claude 按官方三项相加 |
| 处理总量 | `TotalProcessedTokens` | 输入总量 + `output_tokens` | `prompt_tokens` + `completion_tokens` | `input_tokens` + `output_tokens` | `promptTokenCount` + `thoughtsTokenCount` + candidates token 总量 | 只用于 TPM；按官方总量公式去重，不加入 Gemini `toolUsePromptTokenCount` |
| 实际服务层级 | `ServiceTierEffective` | 无响应回显 | 响应顶层 `service_tier` | 响应顶层 `service_tier` | `usageMetadata.serviceTier` | 保存响应侧实际生效层级；请求侧值另存，不得覆盖响应值 |

### 3.3 官方复核结论与来源

官方来源：[Anthropic Messages API reference](https://platform.claude.com/docs/en/api/messages)、[OpenAI Chat Completions create reference](https://developers.openai.com/api/reference/resources/chat/subresources/completions/methods/create/)、[OpenAI Responses create reference](https://developers.openai.com/api/reference/resources/responses/methods/create/)、[OpenAI Prompt caching guide](https://developers.openai.com/api/docs/guides/prompt-caching)、[Gemini GenerateContent API reference](https://ai.google.dev/api/generate-content)、[Gemini token counting guide](https://ai.google.dev/gemini-api/docs/generate-content/tokens)。

### 3.4 每个规范对应的计费规则

> 提供一串单位方便你编写计费规则：输入文本单价、输出文本单价、5Min缓存创建单价、1H缓存创建单价、缓存读取单价、图片输入单价、音频输入单价、音频输入缓存单价、图片输出单价、音频输出单价
>
> 举例：XXX：输入文本\*输入文本单价+输出文本\*输出文本单价
>
> 不得重复计算部分Token的费用，例如有的输入其实是文本+缓存，这个时候需要移除缓存，一般上面的表格有写最终的语义化统一标签

所有规范只按 3.1 的语义字段计费：`InputTokens` 是扣除缓存后的普通输入，`OutputTokens` 是官方输出总量，`PromptAggregateTokens` 和 `TotalProcessedTokens` 只用于层级选择与 TPM，不得再进入费用公式。分层或实际服务层级只决定所选单价；`ServiceTierEffective` 本身不是一个计费行项。

当输入模态明细是 `InputTokens` 的子集时，先从 `InputTokens` 中移除已单独计价的图片/音频/视频/文档，再按输入文本单价计剩余输入；不能把明细再叠加到 `InputTokens` 上。同理，当输出模态明细是 `OutputTokens` 的子集时，也先移除已单独计价的音频/图像，剩余输出与 reasoning 一并按输出文本单价计；reasoning 是输出总量子集，不得加回输出。未分档的 `CacheWriteTokens` 表示 `CacheWriteTokens - CacheWrite5mTokens - CacheWrite1hTokens`，按 5Min 缓存创建单价计。映射表没有统一音频缓存字段，因此在补齐该映射前不得凭空使用音频输入缓存单价。

- Claude：

Claude 的 `InputTokens` 已不含缓存，输入明细按映射表无标准字段，因此普通输入整体按输入文本单价计；输出整体按输出文本单价计。缓存读取、5 分钟写入和 1 小时写入分列结算，未分档写入按 5 分钟写入结算。

```text
费用 = InputTokens * 输入文本单价
     + CacheReadTokens * 缓存读取单价
     + CacheWrite5mTokens * 5Min缓存创建单价
     + CacheWrite1hTokens * 1H缓存创建单价
     + (CacheWriteTokens - CacheWrite5mTokens - CacheWrite1hTokens) * 5Min缓存创建单价
     + OutputTokens * 输出文本单价
```

- OpenAI Chat：

`InputTokens` 已从 raw 输入中扣除缓存读取和缓存写入。若上游返回图片/音频输入明细，这些明细从 `InputTokens` 移出后分别按图片/音频单价计；返回输出音频/图像明细时，同样从输出总量移出。输出中的 text、reasoning、accepted prediction 和 rejected prediction 都使用输出文本单价。缓存读取和缓存写入分开结算；写入无分档时归入未分档写入。

```text
普通输入文本 = InputTokens - ImageInputTokens - AudioInputTokens
非音频/图像输出 = OutputTokens - ImageOutputTokens - AudioOutputTokens

费用 = 普通输入文本 * 输入文本单价
     + ImageInputTokens * 图片输入单价
     + AudioInputTokens * 音频输入单价
     + CacheReadTokens * 缓存读取单价
     + CacheWrite5mTokens * 5Min缓存创建单价
     + CacheWrite1hTokens * 1H缓存创建单价
     + (CacheWriteTokens - CacheWrite5mTokens - CacheWrite1hTokens) * 5Min缓存创建单价
     + 非音频/图像输出 * 输出文本单价
     + ImageOutputTokens * 图片输出单价
     + AudioOutputTokens * 音频输出单价
```

- OpenAI Responses：

`InputTokens` 已从 raw 输入中扣除缓存读取和缓存写入。Responses 侧没有标准输入图片、输入音频、输出图像、输出音频字段，因此不得从其他入口补造这些明细；普通输入和全部输出（含 reasoning）分别按输入文本单价和输出文本单价计。缓存读取和缓存写入分列结算。

```text
费用 = InputTokens * 输入文本单价
     + CacheReadTokens * 缓存读取单价
     + CacheWrite5mTokens * 5Min缓存创建单价
     + CacheWrite1hTokens * 1H缓存创建单价
     + (CacheWriteTokens - CacheWrite5mTokens - CacheWrite1hTokens) * 5Min缓存创建单价
     + OutputTokens * 输出文本单价
```

- Gemini：

`InputTokens = PromptAggregateTokens - CacheReadTokens`，已不包含缓存读取。图片、音频、视频、文档输入明细从 `InputTokens` 移出；图片和音频有独立单价，视频和文档没有单独单价，暂与剩余输入一起按输入文本单价计。输出中的文本、thoughts/reasoning 与非音频/图像输出一起按输出文本单价计，图片和音频输出分列结算。Gemini 没有标准缓存写入字段，因此不得计缓存写入费用；`ToolUsePromptTokens` 只作审计，不进入费用或 TPM。

```text
普通输入文本 = InputTokens - ImageInputTokens - AudioInputTokens - VideoInputTokens - DocumentInputTokens
非音频/图像输出 = OutputTokens - ImageOutputTokens - AudioOutputTokens

费用 = 普通输入文本 * 输入文本单价
     + ImageInputTokens * 图片输入单价
     + AudioInputTokens * 音频输入单价
     + VideoInputTokens * 输入文本单价
     + DocumentInputTokens * 输入文本单价
     + CacheReadTokens * 缓存读取单价
     + 非音频/图像输出 * 输出文本单价
     + ImageOutputTokens * 图片输出单价
     + AudioOutputTokens * 音频输出单价
```

## 4. `billing_details` 设计

本列的范围是“归一化 Token 用量”。它只回答“这次用了哪些 token”，不回答“这些 token 值多少钱”。供应商、入口、转换链、quota、价格、倍率、工具费、按次费和违规费都不放入这个 JSON。

### 4.1 存储方式

`Log` 新增一个列：

| 列名 | 存储类型 | 含义 |
| --- | --- | --- |
| `billing_details` | text，内容为 UTF-8 JSON | 本次请求的语义化 Token 用量明细 |

实现要求：

1. 使用 text 存 JSON，避免依赖 SQLite、MySQL、PostgreSQL 的 native JSON 类型差异。
2. 写入前必须序列化为 canonical JSON，字段名统一 snake_case。
3. 上游未返回的可选字段写 `null` 或省略，不能用 0 伪装成“官方返回了零”；核心总量字段必须存在。
4. JSON 失败时计费和落库必须显式失败或进入清晰错误路径，不能伪造空拆分。
5. `Quota`、`PromptTokens`、`CompletionTokens` 以及现有计费快照字段继续保留在原位，供计费、排序、统计和旧前端使用。
6. `billing_details` 是所有新旧日志 Token 明细的权威来源；`Other` 中的价格、倍率和诊断 key 保持现有职责，不再承载本列已接管的 Token 明细。

### 4.2 JSON 顶层结构

新日志的 `billing_details` 固定为 `tokens` 一段；输入、输出和缓存的明细直接挂在对应分组内。没有 Token 用量的消费入口不写该列。

~~~json
{
  "schema_version": 1,
  "tokens": {
    "input": {
      "text_input": 0,
      "image_input": 0,
      "audio_input": 0,
      "video_input": 0,
      "document_input": 0
    },
    "output": {
      "text_output": 0,
      "audio_output": 0,
      "image_output": 0,
      "reasoning_output": 0,
      "accepted_prediction": 0,
      "rejected_prediction": 0
    },
    "cache": {
      "read_cache": 0,
      "write_cache": 0,
      "write_cache_5m": 0,
      "write_cache_1h": 0
    }
  }
}
~~~

约束：

1. `schema_version` 必须是非负整数，当前版本固定为 `1`；未知版本必须显式降级或报错，不能按相似字段猜测。
2. 顶层只允许 `schema_version` 和 `tokens`。`tokens` 下只允许 `input`、`output`、`cache` 三组；扩展字段必须先升级 schema 版本。
3. 该 JSON 只存放归一化 Token 用量，不写 quota、单价、倍率、分组、供应商、service tier、错误信息或请求元数据。
4. 所有 token 字段必须是非负整数。上游没有返回的可选拆分写 `null` 或省略；只有上游明确返回 `0` 时才写 `0`，不能用 `0` 伪装成真实明细。示例中的 `0` 只表示字段位置。
5. `input` 里的输入明细不包含缓存；缓存读取和写入只能放在 `cache` 组。同一 token 不得同时计入输入普通模态和缓存。
6. `write_cache` 是缓存写入总量，`write_cache_5m` 和 `write_cache_1h` 是它的官方分档子集；必须满足 `write_cache_5m + write_cache_1h <= write_cache`，差值表示未分档写入。
7. `reasoning_output`、accepted prediction 和 rejected prediction 是输出审计拆分，不得额外累加成第二份输出费用。
8. 缺失字段、负数、分档大于总量、未知版本和未知字段都必须走显式错误路径；不能静默裁剪、补零或伪造空明细。
9. 写入前必须序列化成 canonical JSON，字段名保持 snake_case，不保留调试注释和未知供应商字段。

### 4.3 迁移与兼容

1. GORM AutoMigrate 加 `billing_details` 列和内部行级 `billing_details_version` 列；三库都要验证空列、历史数据和重启迁移路径。
2. master 启动后在服务可写流量前执行一次性历史迁移；按旧 `Other` Token key 回填，缺失普通输入/输出时用聚合列扣除已知拆分，矛盾数据显式阻断。
3. 已有合法 `billing_details` 不覆盖现有值；与旧 `Other` 同名字段冲突时迁移失败并要求人工处理。
4. 新日志保留现有聚合列、计费快照和 `Other` 非 Token key；本列只接管 Token 用量，不改变计费快照的写入位置。
5. 读取端不使用 `Other` 或聚合列推导 Token 明细；无 `billing_details` 显示明确空态，损坏 JSON、未知版本和非法字段显式报错。
6. 没有真实 Token 用量的存量消费入口不生成该列；只有它们携带上游 Token usage 时才生成。
7. 本版本不支持旧、新写入节点混跑：升级前备份日志库，停止并排空全部旧版 master/slave、后台日志写入和批量队列，再启动唯一新版 master；确认迁移完成后才启动新版 slave 并恢复流量。旧二进制仍能写入 v0 行，完成标记无法隔离旧写入；不得只依赖“先 master 后 slave”的部署顺序。导入旧数据也必须先停止全部读写节点，在实际日志库执行 `DELETE FROM log_billing_migration_states WHERE version = 1` 清除完成标记，导入完成后再运行 master 迁移。
8. 完成标记保存在实际日志库的 `log_billing_migration_states` 表（`version=1`）；全部批次提交后才写入，master 后续启动命中标记直接跳过回填。slave 必须验证日志表、两列及标记，不执行迁移；独立 `LOG_SQL_DSN` 只以该日志库标记为准。
9. 回填按主键 `id > lastID` 游标、每批 1000 行执行，不增加长期索引或运行期扫描；未完成迁移的启动至多顺序扫描一次已有前缀，已完成后重启不再扫描日志表。查询/事务每次尝试最多 2 分钟，数据库失败最多尝试 3 次，间隔 250/500 毫秒；非法历史数据立即阻断。已提交批次不回滚，重启按行版本继续；进度输出已提交行数、last_id 和耗时，标记写入失败同样阻断启动。
10. 没有明细且聚合为零的历史消费日志保持 NULL；已有合法但无值的明细同样展示明确空态，显式记录的 0 仍为有效值。写入层对聚合非零但缺失明细发出告警，保留账务与错误日志，调用方负责记录无 usage 或归一化失败原因。
11. 回滚顺序是先关闭新版本读写，再确认迁移前备份可恢复，最后才允许评估列清理；迁移已经回填的数据不得当作旧格式降级。

## 5. 实施阶段

本章合并原计费层、前端、任务拆分和完成标准。阶段顺序表示依赖关系；每个阶段必须能独立编译、测试和回滚，不能把未验证的中间状态留在主干。

### 阶段 0：数据库自动迁移与存储适配

#### 任务

- 在 `internal/store/log` 的 `Log` 模型新增 `billing_details` 列，数据库类型为 text，应用层保存 canonical JSON 字符串。
- `RecordConsumeLogParams` 增加对应参数；`RecordConsumeLog` 只在参数非空时写入该列，空入口保持 NULL/空值。
- 同库日志和独立 `LOG_SQL_DSN` 两条路径都必须接入 GORM AutoMigrate；主库迁移列表与独立日志库迁移列表不能遗漏模型。
- 数据库初始化、历史库启动、空库初始化、重复启动迁移和主/从节点行为都要适配。
- 序列化和反序列化统一使用 `pkg/jsonx`，不直接调用通用标准库的 JSON 编解码入口。

#### 注意事项

- Token 明细迁移只在服务可写流量前执行；非法 JSON、非整数、负数、明细超过聚合或同字段冲突都必须失败，不改写计费结果、价格快照和诊断字段。
- 三库差异按现有迁移约定处理：优先 text，不引入 JSONB 或 MySQL 专有能力。
- 迁移必须幂等；任何迁移失败必须阻断启动，不能留下半迁移状态后继续写日志。

#### 验收标准

- SQLite、MySQL 和 PostgreSQL 的历史库、空库、重复启动迁移全部通过。
- 同库日志和独立日志库都能创建并读写 `billing_details`。
- 历史消费日志具备 `billing_details` 或明确的“未记录 Token 明细”空态；`Other` 不再保留已迁移 Token key，旧统计列和计费结果不变。

#### 编译校验清单

- 后端：`go test ./internal/store/...`
- 后端：覆盖至少一条 SQLite 迁移路径；能配置 MySQL/PostgreSQL 时补充对应验证。
- 前端：本阶段无结构改动时运行 typecheck 即可。

### 阶段 1：三规范 Usage 归一化并落 JSON

#### 任务

- 建立 Claude Messages、OpenAI Chat/Responses、Gemini 三个规范族的统一归一化入口，先产出内部 `BillingUsage`，再序列化成第 4 章定义的 `billing_details` JSON。
- 每个来源必须显式标识 Claude、OpenAI Chat、OpenAI Responses、Gemini 或对应兼容渠道来源；不通过 `FinalRequestRelayFormat` 反推语义。
- Claude 原生、AWS Bedrock 复用路径、OpenRouter Claude 风格返回收敛到同一条 Claude 规则。
- 按第 3 章公式拆出普通输入、输出、缓存读取、缓存写入、模态明细和审计拆分；同一 token 不得重复计入两个计价维度。
- 缓存写入转换规则：存在 `write_cache` 且不存在官方 `write_cache_5m`、`write_cache_1h` 时，令 `write_cache_5m = write_cache`；两个分档存在时校验其和不大于 `write_cache`，差值为未分档写入。
- 有 Token 用量的 `RecordConsumeLog` 调用点写入归一化 JSON；无 Token 用量、纯工具费、违规费、定制音色等入口不写该列。

#### 注意事项

- 本阶段只稳定“转换、校验、落库”和新旧读取兼容，不切换 quota 公式；现有计费快照和 `Other` 继续原样写入。
- 负数、关键字段缺失、明细大于官方总量、未知 schema 字段和未知版本必须显式失败或保留可诊断错误路径。
- OpenAI Responses 不虚构不存在的模态明细；Gemini `toolUsePromptTokensDetails` 只做审计，不进入计价输入。
- 估算 usage、上游缺失 usage、流中断、失败重试的真实行为不变。

#### 验收标准

- 四个来源在无缓存、缓存读取、缓存写入、5m/1h、text/image/audio/video/document、reasoning、上下文分段样本下生成相同语义的 JSON。
- 同一 Claude 返回经原生、AWS、OpenRouter 路径转换后的语义字段一致。
- 新日志和历史迁移日志都能读出 JSON；关闭写入后的日志按缺失或错误状态显示，不回退旧路径。

#### 编译校验清单

- 后端：`go test ./internal/domain/... ./internal/relay/... ./internal/store/log/...`
- 后端：补齐表驱动归一化、schema 校验、缓存分档和三库落库测试。
- 前端：本阶段不切换读取时运行 typecheck。

### 阶段 2：计费计算切换到归一化结果

#### 任务

- `CalculateUsage`、Claude 计费、audio、realtime、WSS 计费改为读取同一次请求已生成的内存 `BillingUsage`；落库 JSON 与计费输入来自同一次转换，不再从聚合字段反推普通输入，也不重读日志 JSON。
- 删除 `isClaudeUsageSemantic`、OpenRouter 特殊减法、前端语义参数和重复的 cache/audio 减法。
- 上下文分段计费改用普通输入、输出、缓存读取、缓存写入分档；命中档位后的价格快照仍写入现有位置。
- service tier 继续从响应侧读取并写入现有计费快照；请求侧 tier 不进入 token 明细。
- 工具费、图片生成调用费、文件检索费、违规费继续走独立路径；同请求有 Token 消耗时，Token 部分只来自归一化结果。
- 上线前保留旧公式影子对拍，确认迁移期间 quota 相同；差异必须先定位为旧公式 bug 或新语义 bug，不允许直接吸收差异。

#### 注意事项

- reasoning 和 accepted/rejected prediction 是输出审计拆分，不额外累加输出费用。
- audio、image、video、document 只有价格表明确差异化计价时才参与费用；否则不因为明细存在而重复扣费。
- 客户端可见的各协议 usage 响应体保持兼容，除非确认为 bug 修复并单独记录。
- 计费配置缺失、归一化失败和上游无 usage 必须分成三类可观测原因。

#### 验收标准

- 新旧路径对拍覆盖流式、非流式、失败重试、预扣、退款、缓存命中、cache 与模态重叠、分段命中和无 usage。
- Claude、OpenAI Chat、OpenAI Responses、Gemini 的 quota 与第 3 章公式一致。
- 同一条日志中的 quota、Token 明细、现有计费快照和独立费用能分别解释。

#### 编译校验清单

- 后端：`go test ./internal/domain/billing/... ./internal/domain/... ./internal/relay/...`
- 后端：构建服务并检查 JSON 工具调用。
- 前端：本阶段未改 UI 时运行 typecheck。

### 阶段 3：新增组件化价格表与前端配置

#### 任务

- 新增模型价格表的领域模型和持久化模型，按模型、端点、生效分组和组件保存价格；组件至少覆盖 text/image/audio/video/document 输入，text/audio/image 输出，cache 读取，cache 写入 5m/1h。reasoning 复用输出基础价，只作为用量展示拆分。
- 保留按次价格、上下文分档、service tier 差异价、免模型、分组倍率来源和生效时间等配置能力。
- 提供向后兼容映射：旧 `model_ratio`、`completion_ratio`、`cache_ratio`、`cache_creation_ratio`、audio ratio 和按次价格继续可用，并能转换为等价组件价格。
- 补齐管理端价格编辑接口、权限校验、审计日志、配置校验和缓存刷新。
- 前端补齐价格配置表单、组件价格编辑、上下文分档编辑、校验提示、空态和中英文 i18n。

#### 注意事项

- 价格表属于计价配置，不写入 `billing_details`；日志只保存结算时实际使用价格的快照。
- 组件间存在子集关系时禁止把父项和子项同时配置成两份费用；reasoning 属于输出拆分，不是额外组件。
- 价格单位、汇率、精度、取整规则和分组倍率必须在保存时显式校验，不能依赖前端重算。
- 历史配置必须可无损回滚到旧 ratio 存储；新表上线不代表立即删除旧配置。

#### 验收标准

- 同一模型能同时配置普通输入、多模态、缓存、音频、按次和上下文分档价格，且接口返回的模型广场数据一致。
- 旧模型配置迁移后价格不变，新组件价格未配置时按兼容规则回退。
- 管理端保存触发审计，普通用户无法访问写接口。

#### 编译校验清单

- 后端：`go test ./internal/domain/billing/... ./internal/store/... ./internal/httpapi/...`
- 前端：`bun run typecheck`、`bun run lint`、`bun run build`

### 阶段 4：按价格表结算并优化落库

#### 任务

- 将归一化 token 组件与价格表组件逐项匹配，统一精度、汇率、分组倍率、service tier 和上下文分档后计算 quota。
- 删除通过旧 ratio 间接拼装多模态和缓存价格的分散逻辑；未配置组件必须命中显式回退规则，而不是隐式按 0 计费。
- 结算结果写入现有 quota、计费快照和 `Other` 价格快照位置；`billing_details` 仍只保存 token 用量。
- 保存实际生效的价格组件、分组倍率、上下文档位、service tier 和价格来源，保证历史日志不因当前配置变化而无法解释。
- 保持预扣、补扣、退款、失败重试、工具费和违规费的原子性和现有行为。

#### 注意事项

- 不用 `billing_details` 反推 quota；quota 是结算结果，token 明细是用量输入。
- Decimal 中间结果不得提前取整；最终 quota 取整规则与现有口径一致并写入测试。
- 旧价格配置、新价格表和分组倍率变化的优先级必须固定，禁止调用方各自选择。

#### 验收标准

- 旧价格路径与价格表路径在等价配置下 quota 一致。
- 新组件价格、旧 ratio、按次价格、免模型、上下文分档和 service tier 都有端到端计费用例。
- 日志价格快照在价格被修改后仍能解释当时结算结果。

#### 编译校验清单

- 后端：`go test ./internal/domain/billing/... ./internal/store/... ./internal/httpapi/...`
- 后端：补充 decimal 精度、取整、退款和并发结算测试。
- 前端：本阶段无 UI 改动时运行 typecheck。

### 阶段 5：计费逻辑收口与使用日志展示优化

#### 任务

- `usage-logs` 类型新增 `billing_details` 版本化结构，建立集中解析器，表格、tooltip 和详情弹窗复用同一结果。
- 新日志直接读取 `tokens.input`、`tokens.output`、`tokens.cache.read_cache`、`write_cache`、`write_cache_5m`、`write_cache_1h`、模态和 `reasoning_output`，删除从 `prompt_tokens` 减 cache、cache creation、audio 的逻辑。
- 空值显示“未记录 Token 明细”；损坏 JSON、未知版本和非法字段显示明确错误，不做启发式猜测或回退。
- 金额、倍率、价格来源、上下文档位和 service tier 继续读结算快照；不从 token 明细或当前配置重算历史价格。
- 工具费、违规费、管理端调试字段、普通用户权限过滤和现有统计口径保持不变。
- 新增文案补齐英文和中文 i18n；表格和详情继续遵守 `DataTablePage` 与 `SectionPageLayout` 规范。

#### 注意事项

- 字段名使用第 4 章最终命名：`text_input`、`reasoning_output`、`read_cache`、`write_cache`、`write_cache_5m`、`write_cache_1h`，不保留临时别名。
- 没有上游字段时显示空态或明确零值，不编造模态和缓存拆分。
- 短期 TPM 继续使用 `prompt_tokens + completion_tokens`；按明细聚合必须由服务端提供。
- 列表接口避免逐行重复解析 JSON，必要时在服务端投影常用字段。

#### 验收标准

- 新旧迁移日志同屏展示正确，导出、筛选、权限过滤和空态不回退。
- 缓存、模态、reasoning、价格快照和缺失明细场景都有前端测试。
- 管理端和普通用户端可见字段与现有权限规则一致。

#### 编译校验清单

- 前端：`bun run typecheck`、`bun run lint`、`bun run build`
- 后端：`go test ./internal/httpapi/... ./internal/store/log/...`
- 后端：验证日志 API 投影、权限过滤和 admin-only 字段裁剪。

### 阶段 6：优化模型广场价格显示

### 阶段 7：联调测试，review

## 6. 上线校验清单

### 语义与计费

- [ ] Claude、OpenAI Chat、OpenAI Responses、Gemini 在无缓存、缓存读取、缓存写入 5m/1h、text/image/audio/video/document、reasoning 和上下文分段场景下与第 3 章映射一致。
- [ ] 普通 `InputTokens` 不含缓存，也不默认扣除模态明细；`PromptAggregateTokens` 保留官方 raw 输入总量。
- [ ] 缓存、模态、reasoning、prediction 和 tool-use 子集没有重复计价；Gemini tool-use prompt 只审计。
- [ ] Claude 原生、AWS 复用路径和 OpenRouter 兼容路径生成同一语义结果，不再依赖 `FinalRequestRelayFormat`。
- [ ] `Log.Quota` 由价格表和归一化 token 计算得出，不由 `billing_details` 反推。
- [ ] 工具费、图片生成调用费、文件检索费、违规费和定制音色等独立费用走原路径，不混入 `billing_details`。

### 数据与兼容

- [ ] 新日志 `billing_details` 只有 `schema_version` 和 `tokens`；字段命名、非负整数、null/省略、缓存总量和分档约束全部符合第 4 章。
- [ ] 存在无分档 `write_cache` 时，归一化结果将 `write_cache_5m` 设为同一总量；存在分档时满足分档和不大于总量。
- [ ] `billing_details` 为空、JSON 损坏、版本未知时，后端计费和前端展示都有明确空态或错误路径。
- [ ] 历史日志 Token 明细完成幂等迁移；计费结果、价格快照、倍率、诊断、筛选、导出、统计和权限过滤不变。
- [ ] SQLite、MySQL、PostgreSQL 的空库初始化、历史库启动、重复启动迁移和独立日志库迁移通过。
- [ ] 回滚演练通过：先关闭新版本读写，使用迁移前备份恢复，最后才能评估内部版本列清理。

### 价格与结算

- [ ] 旧 ratio、按次价格、免模型配置能映射到组件价格表且 quota 不变。
- [ ] 价格组件、分组倍率、上下文档位、service tier、汇率、精度和最终取整都有显式规则与测试。
- [ ] 未配置组件只允许命中文档化的回退规则，不允许静默按 0 计费。
- [ ] 日志保存实际生效价格快照，价格配置变化后历史账单仍可解释。

### 质量、可观测与交付

- [ ] 后端目标包测试覆盖 billing、shared usage normalization、store/log、relay 渠道转换和 HTTP 边界。
- [ ] 计费对拍覆盖流式、非流式、失败重试、预扣、退款、上下文分段、cache 与模态重叠、无 usage，以及同日志包含 Token 用量和独立费用。
- [ ] 前端 typecheck、lint、build 通过，覆盖新旧日志、空值、零值、缓存分档、模态明细、未知版本和金额展示并存。
- [ ] 归一化失败、上游无 usage、计费配置缺失三类原因可区分定位，错误信息不含 secret。
- [ ] 列表接口没有逐行重复 JSON 解析或明显延迟；TPM、RPM、用户消耗和渠道消耗口径迁移前后可解释。
- [ ] 3.1 映射表、第 4 章 schema、价格表文档和实施结论一致；对外 API、前端可见变化和历史兼容策略写入发布说明。
- [ ] 每个 PR 可独立回滚，不留下半开状态。
