# `dynamic_ratio_rules` 表

`dynamic_ratio_rules` 定义按时间、并发、分组和模型条件生效的动态倍率规则。

| 字段 | 类型/存储 | 存放的数据 |
|---|---|---|
| `id` | `int64`，主键 | 规则唯一 ID。 |
| `enable` | `bool` | 规则是否启用，默认启用。 |
| `group` | `string` | 生效的业务分组；必须存在于分组倍率配置中。 |
| `models` | `string`，JSON 数组文本 | 适用模型名数组；空字符串表示匹配该分组下所有模型。 |
| `concurrency` | `int64`，可为 `NULL` | 触发该倍率所需的并发阈值；`NULL` 或 0 表示不按并发判断。 |
| `weekdays` | `string` | 生效星期集合；空表示不限星期。 |
| `start_time` | `string` | 生效窗口开始时间，格式 `HH:MM`；空表示不限开始时间。 |
| `end_time` | `string` | 生效窗口结束时间，格式 `HH:MM`；空表示不限结束时间。 |
| `ratio` | `float64` | 命中规则后使用的倍率；非空。 |
| `priority` | `int` | 规则优先级，数值越大越优先；默认 0。 |
| `created_at` | `bigint`，Unix 秒 | 规则创建时间。 |
| `updated_at` | `bigint`，Unix 秒 | 规则最近更新时间。 |
