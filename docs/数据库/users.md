# `users` 表

`users` 保存用户账户、角色、额度、邀请关系、通知凭证引用和媒体转换计数，支持软删除。

| 字段 | 类型/存储 | 存放的数据 |
|---|---|---|
| `id` | `int`，主键 | 用户唯一 ID。 |
| `username` | `string` | 登录用户名；有效记录唯一。 |
| `password` | `string` | bcrypt 密码哈希；不保存明文，普通查询会省略该列。 |
| `display_name` | `string` | 用户显示名称，最长 20 字符。 |
| `role` | `int` | 用户角色：1 普通用户，10 管理员，100 root。 |
| `status` | `int` | 账户状态：1 启用，2 禁用。 |
| `email` | `string` | 用户邮箱；可空或按业务唯一性校验。 |
| `github_id` | `string` | GitHub OAuth 关联的用户 ID；空表示未绑定。 |
| `access_token` | `char(32)`，可为 `NULL` | 系统管理 API 访问令牌；全局唯一，接口不直接返回。 |
| `quota` | `int` | 当前剩余内部额度。 |
| `used_quota` | `int` | 历史累计消耗内部额度。 |
| `request_count` | `int` | 历史累计成功请求次数。 |
| `group` | `string` | 用户默认业务分组；默认 `default`。 |
| `aff_code` | `varchar(32)` | 邀请码；唯一。 |
| `aff_count` | `int` | 通过该用户邀请注册的人数。 |
| `aff_quota` | `int` | 邀请奖励剩余可领取/可使用额度。 |
| `aff_history` | `int` | 邀请奖励历史累计额度。注意列名是 `aff_history`，Go 字段名为 `AffHistoryQuota`。 |
| `inviter_id` | `int` | 邀请人 `users.id`；0 表示无邀请人。 |
| `deleted_at` | 时间戳，可为 `NULL` | GORM 软删除时间；`NULL` 表示账户未删除。 |
| `linux_do_id` | `string` | LinuxDO OAuth 关联的用户 ID；空表示未绑定。 |
| `setting` | `text`，JSON 文本 | 用户通知和偏好设置，包括 Webhook、Gotify、Bark 等凭证；不对普通接口批量返回。 |
| `remark` | `varchar(255)` | 管理员备注，最长 255 字符。 |
| `stripe_customer` | `varchar(64)` | Stripe customer ID，用于支付身份关联。 |
| `image_converted_count` | `int` | 用户使用图片自动转 URL 功能的累计次数。 |
| `video_converted_count` | `int` | 用户使用视频转 URL 功能的累计次数。 |
| `created_at` | `bigint`，Unix 秒 | 用户创建时间。 |
| `last_login_at` | `bigint`，Unix 秒 | 最近登录时间。 |
