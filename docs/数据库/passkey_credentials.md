# `passkey_credentials` 表

`passkey_credentials` 保存 WebAuthn / Passkey 登录凭证和安全状态，支持软删除。

| 字段 | 类型/存储 | 存放的数据 |
|---|---|---|
| `id` | `int`，主键 | 凭证记录唯一 ID。 |
| `user_id` | `int` | 凭证所属 `users.id`。 |
| `credential_id` | `varchar(512)` | WebAuthn credential ID 的 base64 文本，全局唯一。 |
| `public_key` | `text` | 凭证公钥的 base64 文本。 |
| `device_name` | `varchar(255)` | 用户或系统给出的认证器显示名称。 |
| `attestation_type` | `varchar(255)` | WebAuthn 注册时得到的 attestation 类型。 |
| `aa_guid` | `varchar(512)` | 认证器 AAGUID 的 base64 文本，用于识别 authenticator 型号类别。 |
| `sign_count` | `unsigned int32` | 认证器签名计数，用于检测克隆凭证。 |
| `clone_warning` | `bool` | 是否检测到签名计数异常或疑似克隆。 |
| `user_present` | `bool` | 上次验证时用户存在性标志（UP）。 |
| `user_verified` | `bool` | 上次验证时用户身份验证标志（UV）。 |
| `backup_eligible` | `bool` | 凭证是否允许云备份（BE）。 |
| `backup_state` | `bool` | 凭证当前是否处于已备份状态（BS）。 |
| `transports` | `text`，JSON 数组 | 认证器支持的传输方式，例如 `internal`、`hybrid`、`usb`。 |
| `attachment` | `varchar(32)` | 认证器 attachment 类型，例如平台或跨平台。 |
| `last_used_at` | 时间戳，可为 `NULL` | 最近一次成功使用时间；数据库按时间类型存储。 |
| `created_at` | 时间戳 | 凭证注册时间；数据库按时间类型存储。 |
| `updated_at` | 时间戳 | 凭证最近更新时间；数据库按时间类型存储。 |
| `deleted_at` | 时间戳，可为 `NULL` | GORM 软删除时间；`NULL` 表示凭证仍存在。 |
