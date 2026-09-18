# `stored_videos` 表

`stored_videos` 保存视频转 URL 功能接收到的原始视频字节，结构与图片存储一致，并可按时间范围批量清理。

| 字段 | 类型/存储 | 存放的数据 |
|---|---|---|
| `id` | `varchar(64)`，主键 | 视频公开访问 ID；为空时使用 UUID 填充。 |
| `user_id` | `int` | 上传者 `users.id`。 |
| `channel_id` | `int` | 产生或关联该视频的 `channels.id`。 |
| `created_at` | `bigint`，Unix 秒 | 视频入库时间。 |
| `mime_type` | `varchar(255)` | 视频 MIME 类型，例如 `video/mp4`。 |
| `size_bytes` | `int` | 视频字节数。 |
| `sha256` | `char(64)` | 视频内容的 SHA-256 十六进制摘要。 |
| `data` | 二进制大对象 | 原始视频字节；MySQL 使用 `LONGBLOB`，PostgreSQL 使用 `BYTEA`，SQLite 使用 `BLOB`。 |
