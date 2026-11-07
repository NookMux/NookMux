# `quota_data` 表

`quota_data` 保存数据看板使用的按小时聚合用量。写入前会根据看板配置决定是否保留用户、模型和 token 维度。

| 字段 | 类型/存储 | 存放的数据 |
|---|---|---|
| `id` | `int`，主键 | 聚合记录唯一 ID。 |
| `user_id` | `int` | 用户 ID；关闭按用户统计时会聚合到 0。 |
| `username` | `varchar(64)` | 冗余用户名；关闭按用户统计时为空字符串。 |
| `model_name` | `varchar(64)` | 模型名；关闭按模型统计时为空字符串。 |
| `created_at` | `bigint` | 聚合小时起点，Unix 秒；秒数对 3600 取整。 |
| `token_used` | `int` | 聚合周期内使用的总 token 数；关闭 token 统计或错误请求为 0。 |
| `count` | `int` | 聚合周期内的成功请求数。 |
| `fail_count` | `int` | 聚合周期内的失败请求数。 |
| `quota` | `int` | 聚合周期内成功请求消耗的内部额度。 |
