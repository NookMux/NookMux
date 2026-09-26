# `stored_media` 表

`stored_media` 保存“图片/视频自动转 URL”功能接收到的原始媒体字节，`media_type` 列区分图片与视频，供短期工具访问，并可按日志清理流程批量删除。由旧版 `stored_images` / `stored_videos` 两张同构表在启动迁移中合并而来。

| 字段 | 类型/存储 | 存放的数据 |
|---|---|---|
| `id` | `varchar(64)`，主键 | 媒体公开访问 ID；为空时使用 UUID 填充。 |
| `user_id` | `int` | 上传者 `users.id`，与 `media_type`、`sha256` 组成去重键。 |
| `media_type` | `varchar(16)` | 媒体类型判别列：`image` 或 `video`。 |
| `channel_id` | `int` | 产生或关联该媒体的 `channels.id`。 |
| `created_at` | `bigint`，Unix 秒 | 媒体入库时间。 |
| `mime_type` | `varchar(255)` | 媒体 MIME 类型，例如 `image/png`、`video/mp4`。 |
| `size_bytes` | `int` | 媒体字节数。 |
| `sha256` | `char(64)` | 媒体内容的 SHA-256 十六进制摘要，用于同一用户同类型去重。 |
| `data` | 二进制大对象 | 原始媒体字节；MySQL 使用 `LONGBLOB`，PostgreSQL 使用 `BYTEA`，SQLite 使用 `BLOB`。 |

索引：`idx_stored_media_user_type_sha`（`user_id` + `media_type` + `sha256`，去重查询）、`created_at` 单列索引（时间范围清理与列表排序）。
