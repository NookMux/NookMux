# `logs` 表

`logs` 记录充值、消费、管理、系统、错误和退款等业务事件。项目支持把该表放入独立日志库 `LOG_DB`。

| 字段 | 类型/存储 | 存放的数据 |
|---|---|---|
| `id` | `int` | 日志行 ID；参与时间和用户分页复合索引。 |
| `user_id` | `int` | 事件关联用户 ID。 |
| `created_at` | `bigint`，Unix 秒 | 事件发生时间。 |
| `type` | `int` | 事件类型：0 未知，1 充值，2 消费，3 管理，4 系统，5 错误，6 退款。 |
| `content` | `string` | 事件主描述；错误日志存错误内容，消费日志可存补充说明。 |
| `username` | `string` | 冗余用户名，用于列表展示和按用户搜索。 |
| `token_name` | `string` | 消费请求使用的 API 令牌名称。 |
| `model_name` | `string` | 请求模型名；非模型事件可为空。 |
| `quota` | `int` | 本条事件变动的内部额度；消费为扣减量，充值/退款为对应数额。 |
| `prompt_tokens` | `int` | 输入 prompt token 数。 |
| `completion_tokens` | `int` | 输出 completion token 数。 |
| `use_time` | `int` | 请求耗时，单位毫秒。 |
| `is_stream` | `bool` | 请求是否为流式响应。 |
| `channel_id` | `int` | 实际使用的 `channels.id`。 |
| `token_id` | `int` | 实际使用的 `tokens.id`。 |
| `group` | `string` | 请求命中的用户或令牌业务分组。 |
| `ip` | `string` | 请求来源客户端 IP。 |
| `ua` | `text` | 客户端 `User-Agent` 请求头。 |
| `x_title` | `text` | 客户端 `X-Title` 请求头。 |
| `http_referer` | `text` | 客户端 `Referer` 请求头。 |
| `request_id` | `varchar(64)` | 平台生成的请求 ID。 |
| `upstream_request_id` | `varchar(128)` | 上游返回的请求追踪 ID。 |
| `other` | `string`，JSON 文本 | 计费倍率、模型价格、流式速度、工具调用、计费来源、管理员调试信息等扩展字段。普通用户查询时会剔除 `admin_info` 和 `reject_reason`。 |
