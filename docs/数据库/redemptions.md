# `redemptions` 表

`redemptions` 保存额度兑换码的状态、面额和使用结果，支持软删除。

| 字段 | 类型/存储 | 存放的数据 |
|---|---|---|
| `id` | `int`，主键 | 兑换码记录唯一 ID。 |
| `user_id` | `int` | 创建兑换码的管理员或归属用户 ID。 |
| `key` | `char(32)` | 兑换码密钥，全局唯一。 |
| `status` | `int` | 状态：1 可用，2 禁用，3 已使用。 |
| `name` | `string` | 兑换码批次或显示名称。 |
| `quota` | `int` | 兑换成功后发放的内部额度。 |
| `created_time` | `bigint`，Unix 秒 | 兑换码创建时间。 |
| `redeemed_time` | `bigint`，Unix 秒 | 兑换成功时间；未兑换为 0。 |
| `used_user_id` | `int` | 实际兑换该码的用户 ID；未兑换为 0。 |
| `deleted_at` | 时间戳，可为 `NULL` | GORM 软删除时间；`NULL` 表示记录未删除。 |
| `expired_time` | `bigint`，Unix 秒 | 过期时间；0 表示永不过期。 |
