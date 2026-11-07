# `tickets` 表

`tickets` 保存用户提交的工单元数据和当前处理状态，支持软删除。

| 字段 | 类型/存储 | 存放的数据 |
|---|---|---|
| `id` | `int` | 工单唯一 ID；参与更新时间、用户和状态分页索引。 |
| `user_id` | `int` | 提交工单的用户 ID。 |
| `title` | `varchar(255)` | 工单标题；非空。 |
| `type` | `int` | 工单类型：1 bug，2 功能建议，3 咨询，4 其他。 |
| `status` | `int` | 当前状态：1 待处理，2 处理中，3 已完成。 |
| `created_at` | `bigint`，Unix 秒 | 工单创建时间。 |
| `updated_at` | `bigint`，Unix 秒 | 工单最近一次回复或状态变化时间。 |
| `closed_at` | `bigint`，Unix 秒 | 工单关闭时间；0 表示未关闭或历史数据未记录。 |
| `deleted_at` | 时间戳，可为 `NULL` | GORM 软删除时间；`NULL` 表示工单未删除。 |
