# `models` 表

`models` 保存模型元数据、供应商归属、能力标签和名称匹配规则，供模型目录、日志图标和计费元信息使用。

| 字段 | 类型/存储 | 存放的数据 |
|---|---|---|
| `id` | `int`，主键 | 模型元数据唯一 ID。 |
| `model_name` | `varchar(128)` | 模型名或匹配模板。未删除记录中唯一；匹配语义由 `name_rule` 决定。 |
| `description` | `text` | 模型描述。 |
| `icon` | `varchar(128)` | 模型图标名，通常使用前端图标库可渲染的名称。 |
| `tags` | `varchar(255)` | 模型标签集合，用于展示和筛选。 |
| `vendor_id` | `int` | 关联的 `vendors.id`；0 或空表示未指定供应商。 |
| `endpoints` | `text` | 模型支持的端点或协议标识集合，JSON 文本。 |
| `context_length` | `int` | 模型上下文窗口长度，单位 token；0 表示未配置。 |
| `max_output_tokens` | `int` | 单次最大输出 token 数；0 表示未配置。 |
| `input_modalities` | `text`，JSON 数组 | 支持的输入模态，例如文本、图片、音频。 |
| `output_modalities` | `text`，JSON 数组 | 支持的输出模态，例如文本、图片、音频。 |
| `capabilities` | `text`，JSON 数组 | 模型能力标签集合。 |
| `knowledge_cutoff` | `varchar(32)` | 模型知识截止日期或描述。 |
| `release_date` | `varchar(32)` | 模型发布日期。 |
| `parameter_count` | `varchar(64)` | 参数规模描述，例如 `175B`。 |
| `status` | `int` | 元数据是否可用：1 启用，2 禁用。 |
| `sync_official` | `int` | 是否跟随官方/内置模型数据同步：1 表示同步，其他值表示本地优先。 |
| `created_time` | `bigint`，Unix 秒 | 记录创建时间。 |
| `updated_time` | `bigint`，Unix 秒 | 记录最近更新时间。 |
| `deleted_at` | 时间戳，可为 `NULL` | GORM 软删除时间；`NULL` 表示未删除，并与 `model_name` 参与唯一索引。 |
| `name_rule` | `int` | 请求模型与 `model_name` 的匹配方式：0 精确等于，1 前缀，2 包含，3 后缀。 |
