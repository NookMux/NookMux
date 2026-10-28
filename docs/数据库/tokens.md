# `tokens` 表

`tokens` 保存 API 访问令牌、可用分组、模型限制、永久/时段/周期配额和运行统计，支持软删除。

| 字段 | 类型/存储 | 存放的数据 |
|---|---|---|
| `id` | `int`，主键 | 令牌唯一 ID。 |
| `user_id` | `int` | 令牌所属 `users.id`。 |
| `key` | `char(48)` | 令牌密钥，全局唯一；数据库不保存 `sk-` 前缀，接口返回时脱敏。 |
| `status` | `int` | 状态：1 启用，2 禁用，3 已过期，4 已耗尽。 |
| `name` | `string` | 令牌显示名称。 |
| `created_time` | `bigint`，Unix 秒 | 令牌创建时间。 |
| `accessed_time` | `bigint`，Unix 秒 | 最近一次使用时间。 |
| `expired_time` | `bigint`，Unix 秒 | 过期时间；-1 表示永不过期。 |
| `remain_quota` | `int` | 剩余永久额度；配合 `unlimited_quota` 判断是否限额。 |
| `unlimited_quota` | `bool` | 是否不限永久额度。 |
| `model_limits_enabled` | `bool` | 是否启用模型白名单限制。 |
| `model_limits` | `text` | 启用限制时允许的模型名集合，逗号分隔文本；空表示未配置。 |
| `model_mapping` | `text`，JSON 文本 | 仅该令牌生效的模型重定向规则；`NULL` 表示不映射。 |
| `allow_ips` | `string`，可为 `NULL` | 允许访问的 IP 列表，换行分隔；空表示不限制 IP。 |
| `used_quota` | `int` | 令牌累计消耗的内部额度。 |
| `group` | `string` | 令牌覆盖的用户分组；空表示使用用户默认分组。 |
| `cross_group_retry` | `bool` | 自动分组令牌是否允许跨分组重试。 |
| `quota_type` | `int` | 限额类型：0 不限额，1 永久限额，2 时段限额，3 时段加周期限额。 |
| `window_hours` | `int` | 时段窗口长度，单位小时；`quota_type=2,3` 时生效。 |
| `window_quota` | `int` | 每个时段窗口允许消耗的额度。 |
| `window_start_hour` | `int` | 时段窗口起始小时，取值 0-23。 |
| `cycle_days` | `int` | 周期长度，单位天；`quota_type=3` 时生效。 |
| `cycle_quota` | `int` | 每个周期允许消耗的总额度。 |
| `window_used_quota` | `int` | 当前时段窗口已用额度，由系统维护。 |
| `window_start_time` | `bigint`，Unix 秒 | 当前时段窗口开始时间，由系统维护。 |
| `cycle_used_quota` | `int` | 当前周期已用额度，由系统维护。 |
| `cycle_start_time` | `bigint`，Unix 秒 | 当前周期开始时间，由系统维护。 |
| `deleted_at` | 时间戳，可为 `NULL` | GORM 软删除时间；`NULL` 表示令牌未删除。 |
