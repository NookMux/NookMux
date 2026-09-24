# `voices` 表

`voices` 管理定制音色的白名单、计费留痕和对外音色到上游真实音色的重定向。定制音色不绑定单一供应商，首个支持的上游为 MiniMax。

> 历史说明：该表原名为 `minimax_voices`，随音色管理去供应商化在启动迁移中自动重命名为 `voices`，列结构不变。

| 字段 | 类型/存储 | 存放的数据 |
|---|---|---|
| `id` | `int64`，自增主键 | 音色记录唯一 ID。 |
| `created_at` | `bigint`，Unix 秒 | 记录创建时间。 |
| `updated_at` | `bigint`，Unix 秒 | 记录最近更新时间。 |
| `type` | `varchar(16)` | 音色状态：`preview` 表示试听/定制中，`created` 表示已创建并可用于 TTS。 |
| `operator_id` | `int` | 创建或维护该音色的操作人 ID；用户流程存用户 ID，管理员流程存管理员 ID。 |
| `operator_kind` | `varchar(16)` | 操作人类别：`user` 或 `admin`。 |
| `voice_id` | `varchar(256)` | 对外暴露的音色 ID；全局唯一，是用户发起 TTS 时使用的白名单 ID。 |
| `quota_cost` | `bigint` | 创建音色时扣减的内部额度，用于审计展示；实际扣费在服务层完成。 |
| `redirect_id` | `varchar(256)` | 映射到上游的真实音色 ID；为空则直接把 `voice_id` 发给上游。 |
| `allowed` | `bool` | 是否允许用于 TTS；仅 `type=created` 时生效。 |
| `remark` | `varchar(255)` | 管理员备注。 |
