# `channels` 表

`channels` 保存上游 AI 服务渠道的连接配置、可用模型、分组、调度参数、运行状态和多密钥信息。

| 字段 | 类型/存储 | 存放的数据 |
|---|---|---|
| `id` | `int`，主键 | 渠道唯一 ID。 |
| `type` | `int` | 上游适配器类型，例如 1 表示 OpenAI、3 表示 Azure、14 表示 Anthropic、24 表示 Gemini、33 表示 AWS、35 表示 MiniMax；取值由渠道常量表维护。 |
| `key` | `string` | 上游密钥。普通渠道是一个密钥；多密钥渠道可存换行分隔的多个密钥，部分场景（如 Vertex AI）也可能是 JSON 数组。接口返回前会脱敏。 |
| `open_ai_organization` | `string`，可为 `NULL` | OpenAI 组织 ID；仅需要组织头的上游使用。 |
| `test_model` | `string`，可为 `NULL` | 渠道连通性测试使用的模型名；为空时按业务默认模型处理。 |
| `status` | `int` | 渠道状态：1 启用，2 手动禁用，3 自动禁用。 |
| `name` | `string` | 渠道显示名称；有索引。 |
| `weight` | `unsigned int`，可为 `NULL` | 渠道调度权重；0 表示不加权。 |
| `created_time` | `bigint`，Unix 秒 | 渠道创建时间。 |
| `test_time` | `bigint`，Unix 秒 | 最近一次渠道测试时间。 |
| `response_time` | `int` | 最近一次测试的上游响应耗时，单位毫秒。 |
| `base_url` | `string`，可为 `NULL` | 上游 API 基础地址；为空表示使用该渠道类型的默认地址。 |
| `other` | `string` | 渠道类型专用的补充参数。多数渠道为空；Vertex AI 等场景存 JSON 配置，例如区域或服务账号信息。 |
| `balance` | `float64` | 最近一次查询到的上游余额，通常以美元记录。 |
| `balance_updated_time` | `bigint`，Unix 秒 | 上游余额更新时间。 |
| `models` | `string` | 渠道支持的模型名列表，使用英文逗号分隔。 |
| `group` | `varchar(64)` | 渠道可服务的分组列表，使用英文逗号分隔；默认 `default`。 |
| `used_quota` | `bigint` | 渠道累计消耗的内部额度。 |
| `model_mapping` | `text`，JSON 文本 | 请求模型到上游模型的映射规则；空或 `NULL` 表示不重定向。 |
| `status_code_mapping` | `varchar(1024)`，JSON 文本 | 上游 HTTP 状态码到内部状态码的映射规则。 |
| `priority` | `int64`，可为 `NULL` | 渠道调度优先级；越高越优先。 |
| `auto_ban` | `int`，可为 `NULL` | 是否允许失败后自动禁用渠道；1 表示允许，非 1 表示不允许。 |
| `other_info` | `string`，JSON 文本 | 运行状态补充信息，例如多密钥失败原因等非检索型状态数据。 |
| `tag` | `string`，可为 `NULL` | 渠道标签，用于批量启停、筛选和调度展示；有索引。 |
| `setting` | `text` | 渠道额外开关和策略设置，JSON 文本。 |
| `param_override` | `text` | 发往上游前的请求参数覆盖规则。 |
| `header_override` | `text` | 发往上游前的 HTTP 请求头覆盖规则。 |
| `remark` | `varchar(255)`，可为 `NULL` | 管理员备注，最长 255 字符。 |
| `channel_info` | JSON 对象文本 | 多密钥、轮询状态和套餐识别信息，包括 `is_multi_key`、`multi_key_size`、各密钥状态/禁用原因/禁用时间、轮询索引、多密钥模式、`is_plan` 和 `plan_name`。 |
| `settings` | `string` | 不需要数据库检索的其他渠道设置，JSON 文本；例如 Azure API 版本。 |
