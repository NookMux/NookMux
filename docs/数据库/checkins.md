# `checkins` 表

`checkins` 记录用户每日签到结果，并用唯一约束防止同一用户同一天重复签到。

| 字段 | 类型/存储 | 存放的数据 |
|---|---|---|
| `id` | `int`，自增主键 | 签到记录唯一 ID。 |
| `user_id` | `int` | 签到用户 ID；与 `checkin_date` 组成唯一索引。 |
| `checkin_date` | `varchar(10)` | 签到业务日期，格式 `YYYY-MM-DD`；与 `user_id` 组成唯一索引。 |
| `quota_awarded` | `int` | 本次签到实际发放的内部额度。 |
| `created_at` | `bigint`，Unix 秒 | 记录创建时间。 |
