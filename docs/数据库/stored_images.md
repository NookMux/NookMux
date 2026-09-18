# `stored_images` 表

`stored_images` 保存“图片自动转 URL”功能接收到的原始图片字节，供短期工具访问，并可按日志清理流程批量删除。

| 字段 | 类型/存储 | 存放的数据 |
|---|---|---|
| `id` | `varchar(64)`，主键 | 图片公开访问 ID；为空时使用 UUID 填充。 |
| `user_id` | `int` | 上传者 `users.id`。 |
| `channel_id` | `int` | 产生或关联该图片的 `channels.id`。 |
| `created_at` | `bigint`，Unix 秒 | 图片入库时间。 |
| `mime_type` | `varchar(255)` | 图片 MIME 类型，例如 `image/png`。 |
| `size_bytes` | `int` | 图片字节数。 |
| `sha256` | `char(64)` | 图片内容的 SHA-256 十六进制摘要，用于同一用户去重。 |
| `data` | 二进制大对象 | 原始图片字节；MySQL 使用 `LONGBLOB`，PostgreSQL 使用 `BYTEA`，SQLite 使用 `BLOB`。 |
