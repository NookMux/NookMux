#### H4 配额准入非原子，并发可无界透支

- **位置**：internal/domain/billing/service.go:351（符号 NewBillingSession）；相关：service.go:181-183 / 219-235 / 99-110、internal/domain/billing/quota.go:590、internal/store/user/user.go（decreaseUserQuota）
- **问题与利用条件**：准入判断是从 Redis 缓存先读后扣的非原子 check-then-act；对"受信"用户（余额超过硬编码的 10*QuotaPerUnit 即 $10，或令牌不限额）完全不预扣预留，结算经 store 层无下限的 `quota - ?` 无条件扣减，负余额从不被拒绝或回滚。攻击者充值约 $11 并建不限额令牌后并发大量高成本请求，全部被放行，结算时余额可被拖至大幅透支（示例约 -$4989），运营方已支付上游成本；$10 以下亦可借异步缓存更新竞态达到同类效果。该逻辑为 commit d8654d141 自原 service/billing_session.go 等价迁移而来，属 diff 带入而非新写。
- **修复建议**：将准入与预留合并为条件更新（UPDATE … SET quota = quota − ? WHERE quota >= ?，检查 RowsAffected），trust 模式同样执行或改为在途未预留用量上限；Settle 时复核余额；PreWssConsumeQuota（quota.go:105-155）同模式一并修复。

---

### H4 修复最终方案（2026-09-26 确认）

- **总体方案**：方案一——统一原子预扣。删除信任直通（`BillingSession.shouldTrust` 及阈值 `common.GetTrustQuota`），所有用户（含高余额用户、不限额令牌）一律先原子预留再放行，并发透支上界收敛为"单请求预扣额 × 在途并发数"（预扣额按 prompt + max_tokens 估算，本身有界）。
- **原子门实现**：新增 `userstore.TryDecreaseUserQuota`（准入与预留合并的单步原子操作）。Redis 启用时以配额缓存（ freshest 视图，包含尚未落库的批量在途扣减）执行 Lua 脚本原子 check-and-decr：返回 1 成功、0 余额不足（零副作用）、-1 冷缓存回退；未启用 Redis 或冷缓存时回退数据库条件更新 `UPDATE users SET quota = quota - ? WHERE id = ? AND quota >= ?` 并检查 `RowsAffected`（同步落库、绕过批量队列，保证数据库权威一致）。DB 落库沿用批量队列（Redis 路径，缓存已被 Lua 同步扣减，不得再次异步扣缓存）或同步直写（DB 权威路径）。Redis 求值故障显式报错、由准入门拒绝放行，严禁静默。
- **结算兜底**：`Settle`/`PostConsumeQuota` 补扣按实际消耗执行，余额允许扣至负数，不另设下限、不钳制；此后新请求由原子门按余额不足拒绝（`ErrorCodeInsufficientUserQuota`），账户充值后自动恢复。
- **同步修复点**：`PreWssConsumeQuota`（quota.go）由"快照 check-then-act 后无条件下扣"改为同一原子门 + 令牌扣减失败回滚钱包预留；提取按令牌配额类型扣减/回退的助手函数供 `PostConsumeQuota` 复用；`BillingSession.preConsume` 中令牌预扣先行、资金不足或扣减失败时回滚令牌额度，资金不足映射为 `i18n.MsgBillingPrepaidFailed`。
- **透支上界与残余风险**：准入原子化后，负余额仅可能来自结算差额（实际用量超出预扣估算）在并发下的累积，上界为单请求估算误差 × 在途并发；Redis 缓存键 TTL 与批量落库间隔之间的固有缓存漂移维持现状，原子门已将以缓存为准的准入变为单步原子操作。