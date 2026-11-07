# `prefill_groups` 表

`prefill_groups` 保存可在管理界面复用的字符串集合，例如模型组、标签组或端点组。

| 字段 | 类型/存储 | 存放的数据 |
|---|---|---|
| `id` | `int`，主键 | 分组唯一 ID。 |
| `name` | `varchar(64)` | 分组名称；未软删除记录中唯一。 |
| `type` | `varchar(32)` | 分组类别，常用取值为 `model`、`tag`、`endpoint`；前端按该值决定集合用途。 |
| `items` | JSON 文本 | 该类别的字符串集合，通常为 JSON 数组，例如 `["gpt-4o","gpt-3.5-turbo"]`。 |
| `description` | `varchar(255)` | 分组用途说明。 |
| `created_time` | `bigint`，Unix 秒 | 分组创建时间。 |
| `updated_time` | `bigint`，Unix 秒 | 分组最近更新时间。 |
| `deleted_at` | 时间戳，可为 `NULL` | GORM 软删除时间；`NULL` 表示仍可用。 |
