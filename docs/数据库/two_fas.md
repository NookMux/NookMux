# `two_fas` 表

`two_fas` 保存用户 TOTP 两步验证配置、启用状态和失败锁定状态，每个用户最多一条有效配置。

| 字段 | 类型/存储 | 存放的数据 |
|---|---|---|
| `id` | `int`，主键 | 两步验证配置唯一 ID。 |
| `user_id` | `int` | 所属 `users.id`；有效记录唯一。 |
| `secret` | `varchar(255)` | 加密或编码后的 TOTP 密钥；不返回给前端。 |
| `is_enabled` | `bool` | 两步验证是否已启用。 |
| `failed_attempts` | `int` | 连续验证失败次数，成功验证后重置。 |
| `locked_until` | 时间戳，可为 `NULL` | 验证锁定截止时间；未锁定为 `NULL`。 |
| `last_used_at` | 时间戳，可为 `NULL` | 最近一次成功验证时间；未使用为 `NULL`。 |
| `created_at` | 时间戳 | 配置创建时间。 |
| `updated_at` | 时间戳 | 配置最近更新时间。 |
| `deleted_at` | 时间戳，可为 `NULL` | GORM 软删除时间；`NULL` 表示配置未删除。 |
