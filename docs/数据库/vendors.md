# `vendors` 表

`vendors` 保存模型供应商元数据，供 `models.vendor_id` 引用和前端展示，支持软删除。

| 字段 | 类型/存储 | 存放的数据 |
|---|---|---|
| `id` | `int`，主键 | 供应商唯一 ID。 |
| `name` | `varchar(128)` | 供应商名称；未软删除记录中唯一。 |
| `description` | `text` | 供应商描述。 |
| `icon` | `varchar(128)` | 供应商图标名，通常使用前端图标库可渲染的名称。 |
| `data_retention_days` | `int`，可为 `NULL` | 供应商声明的数据保留天数；`NULL` 表示未配置。 |
| `training_opt_out` | `bool`，可为 `NULL` | 是否声明禁止上游用数据训练；`NULL` 表示未配置。 |
| `status` | `int` | 供应商状态：1 启用，2 禁用。 |
| `created_time` | `bigint`，Unix 秒 | 供应商创建时间。 |
| `updated_time` | `bigint`，Unix 秒 | 供应商最近更新时间。 |
| `deleted_at` | 时间戳，可为 `NULL` | GORM 软删除时间；`NULL` 表示未删除，并与 `name` 参与唯一索引。 |
