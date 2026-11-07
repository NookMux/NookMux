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
| `voices` | [voices.md](voices.md) | 定制音色白名单与映射（原 `minimax_voices`，随去供应商化改名） |
| `models` | [models.md](models.md) | 模型元数据与供应商关联 |
| `options` | [options.md](options.md) | 系统配置键值和一次性迁移标记 |
| `passkey_credentials` | [passkey_credentials.md](passkey_credentials.md) | WebAuthn / Passkey 凭证 |
| `prefill_groups` | [prefill_groups.md](prefill_groups.md) | 可复用的模型、标签或端点预填充分组 |
| `quota_data` | [quota_data.md](quota_data.md) | 数据看板按小时聚合的用量 |
| `redemptions` | [redemptions.md](redemptions.md) | 额度兑换码 |
| `setups` | [setups.md](setups.md) | 系统初始化完成标记 |
| `stored_media` | [stored_media.md](stored_media.md) | 图片/视频转 URL 功能的二进制媒体（原 `stored_images` + `stored_videos`，已合并） |
| `ticket_entries` | [ticket_entries.md](ticket_entries.md) | 工单回复和状态变更记录 |
| `tickets` | [tickets.md](tickets.md) | 用户提交的工单 |
| `tokens` | [tokens.md](tokens.md) | API 令牌、配额和调用限制 |
| `topups` | [topups.md](topups.md) | 充值订单与支付状态 |
| `two_fa_backup_codes` | [two_fa_backup_codes.md](two_fa_backup_codes.md) | 两步验证备用码哈希 |
| `two_fas` | [two_fas.md](two_fas.md) | 用户两步验证配置和锁定状态 |
| `users` | [users.md](users.md) | 用户账户、角色、配额和邀请信息 |
| `vendors` | [vendors.md](vendors.md) | 模型供应商元数据 |

除 `logs` 会在配置独立日志库时写入 `LOG_DB` 外，其余表都写入主库 `DB`。同一张表的准确 DDL 可能随数据库方言不同而不同；文档中的“类型/存储”描述业务数据形态，不替代某一个数据库的实际 `SHOW CREATE TABLE` 结果。

## 表结构优化建议

以下结论来自对照 `internal/store/` 各 GORM 实体与历史迁移代码的结构审查（2026-09）。落地任何一项均可复用现有基建：`internal/store/db/cleanup/` 的幂等清理模式、`options` 表的 `data_migration.<name>.v<n>.done` 一次性迁移标记、`dbmigrate` 的同类型迁移与回填框架。

### 可合并的表

- **已完成**：`stored_images` + `stored_videos` 已合并为 `stored_media`（`media_type` 判别列），启动迁移自动搬数据并删除旧表。
- **明确不合并**：`two_fas` 与 `two_fa_backup_codes`（1:N，备用码需要行级 `is_used` 原子标记）；`tickets` 与 `ticket_entries`（主表/流水）；`logs` 与 `quota_data`（明细 vs 小时聚合）；`options` 与 `setups`（初始化占位锁）；`abilities` 并回 `channels`（调度索引核心，改动风险远大于收益）；`audit_logs` 并入 `logs`（审计要求独立保留策略）。

### 可优化的列

1. **`channels` 的 JSON 杂项列收敛**
   - `setting` 与 `settings` 合并为一列：两列同为渠道 JSON 配置（`ChannelSettings` / `ChannelOtherSettings`），命名仅差一个字母，声称的边界是“是否需要检索”，实际都不参与 SQL 检索，校验永远一起执行，前端也要两个解析函数。
   - `other_info` 可消灭：多密钥信息已迁入 `channel_info`，后端现只写入 `status_reason`、`status_time` 两个键。建议升级为独立列 `status_reason varchar(255)` + `status_time bigint`（顺带支持按禁用原因筛选），或并入 `channel_info` 后删列。
   - `other`（渠道类型专用参数，如 Vertex 区域/服务账号）保留，但与 `other_info` 命名易混淆，长远可考虑改名。
2. **`tokens.unlimited_quota` 与 `quota_type` 双真值收编**：`quota_type=0` 与 `unlimited_quota=true` 语义重叠，代码中存在多处 `quotaType == 0 && !token.UnlimitedQuota` 兼容判断，写入侧也要同步维护两份。建议一次性数据迁移（`quota_type=0 && unlimited_quota=false` 的行改为 `quota_type=1`）后，`unlimited_quota` 降级为只读兼容列，新代码只认 `quota_type`。另外 `tokens.model_limits` 用逗号分隔文本而 `dynamic_ratio_rules.models` 用 JSON 数组，同为“模型集合”建议统一为 JSON 数组。
3. **`logs.id` 升为 `bigint`**：`logs` 是写入量最大的表，`int` 自增 ID 上限约 21.4 亿，按每日 1000 万条约 7 个月耗尽。同项目 `audit_logs`、`dynamic_ratio_rules`、`voices` 的主键均已是 `int64`。MySQL 存量大表需评估 ALTER 锁表成本。
4. **约定统一**（存量不强改，新表遵循）：状态列存储方式（`topups.status` 用字符串枚举，其余表用 int + 常量，新表统一 int）；时间哨兵（`0` 与 `NULL` 两种风格并存，如 `tickets.closed_at` vs `two_fas.locked_until`）；时间列命名（`created_at` / `created_time` / `create_time` 三种后缀并存）；`users.aff_history` 列名与 Go 字段 `AffHistoryQuota` 不一致；`two_fa_backup_codes.deleted_at` 软删除几乎没有业务场景（`is_used` 已足够）；`users.image_converted_count` / `video_converted_count` 与 `stored_media` 表的 COUNT 统计功能重叠（仅展示用，不参与限额判断），可顺势收敛。

### 明确不建议动的项

- `tokens` 的 `window_used_quota` / `window_start_time` / `cycle_used_quota` / `cycle_start_time`：高频原子自增更新，独立列是正确设计，不要 JSON 化。
- `logs` 的 `ua` / `x_title` / `http_referer`：从 `other` JSON 刻意拆出以便检索与裁剪，不要合回。
- `checkins.checkin_date varchar(10)`：ISO 日期字符串对唯一索引和排序完全正确且三库可移植。
- `users` 的 `github_id` / `linux_do_id` 按供应商加列：项目方向已从独立绑定表（`user_oauth_bindings`，已删除）回退为列，仅当接入大量 OAuth 供应商时才值得改为 JSON 绑定列。

### 优先级

`channels.setting` / `settings` 合并与 `other_info` 消灭 > `tokens.unlimited_quota` 收编 > `logs.id` 升 `bigint` > 约定统一。
