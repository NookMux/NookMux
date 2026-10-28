# `ticket_entries` 表

`ticket_entries` 保存工单内的用户/管理员回复以及状态流转记录。

| 字段 | 类型/存储 | 存放的数据 |
|---|---|---|
| `id` | `int` | 工单条目唯一 ID；与 `ticket_id` 组成分页复合索引。 |
| `ticket_id` | `int` | 所属 `tickets.id`。 |
| `entry_type` | `int` | 条目类型：1 消息回复，2 状态变更。 |
| `sender_user_id` | `int` | 发送者 `users.id`；系统或匿名展示场景可为 0。 |
| `sender_name` | `varchar(64)` | 条目展示用发送者名称。 |
| `sender_role` | `int` | 发送时用户角色，取值与用户角色一致：1 普通用户，10 管理员，100 root。 |
| `content` | `text` | 消息内容；状态变更条目可为空。 |
| `from_status` | `int` | 状态变更前工单状态；非状态条目为 0。 |
| `to_status` | `int` | 状态变更后工单状态；非状态条目为 0。 |
| `created_at` | `bigint`，Unix 秒 | 条目创建时间。 |
