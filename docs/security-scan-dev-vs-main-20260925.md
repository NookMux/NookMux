#### H4 配额准入非原子，并发可无界透支

- **位置**：internal/domain/billing/service.go:351（符号 NewBillingSession）；相关：service.go:181-183 / 219-235 / 99-110、internal/domain/billing/quota.go:590、internal/store/user/user.go（decreaseUserQuota）
- **问题与利用条件**：准入判断是从 Redis 缓存先读后扣的非原子 check-then-act；对"受信"用户（余额超过硬编码的 10*QuotaPerUnit 即 $10，或令牌不限额）完全不预扣预留，结算经 store 层无下限的 `quota - ?` 无条件扣减，负余额从不被拒绝或回滚。攻击者充值约 $11 并建不限额令牌后并发大量高成本请求，全部被放行，结算时余额可被拖至大幅透支（示例约 -$4989），运营方已支付上游成本；$10 以下亦可借异步缓存更新竞态达到同类效果。该逻辑为 commit d8654d141 自原 service/billing_session.go 等价迁移而来，属 diff 带入而非新写。
- **修复建议**：将准入与预留合并为条件更新（UPDATE … SET quota = quota − ? WHERE quota >= ?，检查 RowsAffected），trust 模式同样执行或改为在途未预留用量上限；Settle 时复核余额；PreWssConsumeQuota（quota.go:105-155）同模式一并修复。