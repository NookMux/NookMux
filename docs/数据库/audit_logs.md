# `audit_logs` 表

`audit_logs` 记录管理员对系统资源的创建、更新和删除操作，只写主库，不与请求消费日志混用。

| 字段 | 类型/存储 | 存放的数据 |
|---|---|---|
| `id` | `int64`，自增主键 | 审计记录唯一 ID。 |
| `created_at` | `bigint`，Unix 秒 | 操作发生时间；有独立时间索引。 |
| `username` | `varchar(255)` | 执行操作的管理员用户名。 |
| `ip` | `varchar(64)` | 操作来源客户端 IP。 |
| `module` | `varchar(32)` | 审计资源模块，稳定值包括 `option`、`channel`、`user`、`token`、`redemption`、`model`、`vendor`、`dynamic_ratio`、`prefill_group`、`db`、`performance`、`log`、`setup`、`dashboard_config`、`voice`。 |
| `action_type` | `varchar(16)` | 操作类型：`create`、`update` 或 `delete`。 |
| `description` | `varchar(255)` | 人类可读的操作描述。 |
| `before_data` | `text`，JSON 文本 | 变更前资源快照；创建操作通常为空。 |
| `after_data` | `text`，JSON 文本 | 变更后资源快照或本次请求参数；删除操作通常只保留定位信息。 |
