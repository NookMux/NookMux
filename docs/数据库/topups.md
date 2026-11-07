# `topups` 表

`topups` 保存充值订单、支付渠道、金额和订单状态，支付回调按唯一订单号做幂等处理。

| 字段 | 类型/存储 | 存放的数据 |
|---|---|---|
| `id` | `int`，主键 | 充值订单唯一 ID。 |
| `user_id` | `int` | 发起充值的 `users.id`。 |
| `amount` | `bigint` | 订单应发放的内部额度。 |
| `money` | `float64` | 订单支付金额，业务层按两位小数处理。 |
| `trade_no` | `varchar(255)` | 支付订单号，唯一；用于回调定位和幂等判断。 |
| `payment_method` | `varchar(50)` | 支付方式标识，例如 `stripe`。 |
| `payment_provider` | `varchar(50)` | 支付提供商标识，例如 `epay` 或 `stripe`；回调会校验与订单一致。 |
| `create_time` | `bigint`，Unix 秒 | 订单创建时间。 |
| `complete_time` | `bigint`，Unix 秒 | 订单完成时间；未完成为 0。 |
| `status` | `string` | 订单状态：`pending` 待支付，`success` 成功，`expired` 过期，`failed` 失败。 |
