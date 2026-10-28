# `two_fa_backup_codes` 表

`two_fa_backup_codes` 保存两步验证备用码的哈希和使用状态，原文不落库。

| 字段 | 类型/存储 | 存放的数据 |
|---|---|---|
| `id` | `int`，主键 | 备用码记录唯一 ID。 |
| `user_id` | `int` | 所属 `users.id`。 |
| `code_hash` | `varchar(255)` | 备用码哈希；不保存明文。 |
| `is_used` | `bool` | 备用码是否已经使用。 |
| `used_at` | 时间戳，可为 `NULL` | 备用码使用时间；未使用为 `NULL`。 |
| `created_at` | 时间戳 | 备用码生成时间。 |
| `deleted_at` | 时间戳，可为 `NULL` | GORM 软删除时间；`NULL` 表示记录未删除。 |
