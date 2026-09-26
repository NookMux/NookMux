# `abilities` 表

`abilities` 是渠道调度索引。它把渠道支持的“分组 + 模型”组合展开成独立行，让请求能按模型和分组快速筛选可用渠道。

| 字段 | 类型/存储 | 存放的数据 |
|---|---|---|
| `group` | `varchar(64)` | 渠道所属的业务分组；与 `model`、`channel_id` 组成复合主键。 |
| `model` | `varchar(255)` | 该渠道支持的一个模型名；与 `group`、`channel_id` 组成复合主键。 |
| `channel_id` | `int` | 关联的 `channels.id`；与 `group`、`model` 组成复合主键。 |
| `enabled` | `bool` | 这条分组/模型映射是否参与调度。 |
| `priority` | `int64`，可为 `NULL` | 渠道调度优先级，数值越大越优先；缺省或 `NULL` 按 0 处理。 |
| `weight` | `unsigned int` | 同优先级渠道内的加权随机权重，0 表示不额外加权。 |
| `tag` | `varchar`，可为 `NULL` | 冗余存储的渠道标签，便于按标签批量查询和调度。 |
