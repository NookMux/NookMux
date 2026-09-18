# 数据库表文档索引

本文档索引覆盖主库与独立日志库中由启动迁移正式创建的全部表。字段清单来自 `internal/store/db/migrate/init.go` 的 `AutoMigrate` 目标和对应 GORM 实体，不包含 `gorm:"-"` 标记的内存运行时字段。

| 表名 | 文档 | 用途 |
|---|---|---|
| `abilities` | [abilities.md](abilities.md) | 渠道、分组和模型的调度映射 |
| `audit_logs` | [audit_logs.md](audit_logs.md) | 管理员资源操作审计 |
| `channels` | [channels.md](channels.md) | 上游渠道配置与运行状态 |
| `checkins` | [checkins.md](checkins.md) | 用户每日签到与奖励 |
| `dynamic_ratio_rules` | [dynamic_ratio_rules.md](dynamic_ratio_rules.md) | 分组和模型的动态倍率规则 |
| `logs` | [logs.md](logs.md) | 消费、充值、管理、系统和错误日志 |
| `minimax_voices` | [minimax_voices.md](minimax_voices.md) | MiniMax 定制音色白名单与映射 |
| `models` | [models.md](models.md) | 模型元数据与供应商关联 |
| `options` | [options.md](options.md) | 系统配置键值和一次性迁移标记 |
| `passkey_credentials` | [passkey_credentials.md](passkey_credentials.md) | WebAuthn / Passkey 凭证 |
| `prefill_groups` | [prefill_groups.md](prefill_groups.md) | 可复用的模型、标签或端点预填充分组 |
| `quota_data` | [quota_data.md](quota_data.md) | 数据看板按小时聚合的用量 |
| `redemptions` | [redemptions.md](redemptions.md) | 额度兑换码 |
| `setups` | [setups.md](setups.md) | 系统初始化完成标记 |
| `stored_images` | [stored_images.md](stored_images.md) | 图片转 URL 功能的二进制图片 |
| `stored_videos` | [stored_videos.md](stored_videos.md) | 视频转 URL 功能的二进制视频 |
| `ticket_entries` | [ticket_entries.md](ticket_entries.md) | 工单回复和状态变更记录 |
| `tickets` | [tickets.md](tickets.md) | 用户提交的工单 |
| `tokens` | [tokens.md](tokens.md) | API 令牌、配额和调用限制 |
| `topups` | [topups.md](topups.md) | 充值订单与支付状态 |
| `two_fa_backup_codes` | [two_fa_backup_codes.md](two_fa_backup_codes.md) | 两步验证备用码哈希 |
| `two_fas` | [two_fas.md](two_fas.md) | 用户两步验证配置和锁定状态 |
| `users` | [users.md](users.md) | 用户账户、角色、配额和邀请信息 |
| `vendors` | [vendors.md](vendors.md) | 模型供应商元数据 |

除 `logs` 会在配置独立日志库时写入 `LOG_DB` 外，其余表都写入主库 `DB`。同一张表的准确 DDL 可能随数据库方言不同而不同；文档中的“类型/存储”描述业务数据形态，不替代某一个数据库的实际 `SHOW CREATE TABLE` 结果。
