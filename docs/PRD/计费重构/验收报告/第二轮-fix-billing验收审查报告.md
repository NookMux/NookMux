合计P0-0个，P1-16个，P2-12个，P3-6个（P1 含 2026-09-12 复核 `b17d321e` 新增的缺陷 33/34）；截至 2026-09-12（修复提交 `9bfedcb76` + 批次 A `b17d321e` + 批次 A 复核补齐修复 `6a5f7f61d`）已修复 8 项（P1-7：1、2、4、12、14、33、34；P2-1：24），未决 P0-0、P1-9、P2-11、P3-6

# 第二轮 fix/billing 验收审查报告（对首轮审查报告的审核）

- 审查日期：2026-09-09
- 审查对象：[fix-billing验收审查报告.md](fix-billing验收审查报告.md)（首轮，HEAD `7ec946658`）
- 审查分支与 HEAD：`fix/billing` @ `fe3ed4145`（与首轮 HEAD 之间仅环境与文档规则提交，无业务代码变化）
- 验收依据：[验收标准.md](验收标准.md)（标准与 PRD 冲突时以标准为准）
- 分级口径：按用户最终确认的**实际生产风险**分级，验收阻断不直接等同 P0；价格修改允许跨节点最多 60 秒缓存延迟；模型测试 Token 费用必须走统一新价格表；Token 子项隐藏时必须保留其他可见子项、由服务端提供独立展示投影。代码层通过与真实联调验证分开表述，单库或单测通过不代替三库与联调验证。
- 更新记录：2026-09-12 复核修复提交 `9bfedcb76`（删除旧 Token 聚合列，billing_details 成为唯一权威来源），缺陷 1、2、24 确认已修复并标记【已完成】；缺陷 5 复核后确认**未**修复（`LogBillingDetailsVersion` 仍为 1，完成标记短路行为保留），其余条目维持原状。受影响包（store/log、store/db/migrate、store/usedata、domain/billing）`go test` 通过。2026-09-12 同日落批次 A「WSS/Realtime 会话计费收口」（单 commit）：缺陷 4、12、14 确认已修复并标记【已完成】，记账模型统一为"事件实扣 + 收尾补差"（UsePrice 与 ratio 一致，按次价按每个 response.done 事件收取；`PostWssConsumeQuota` 以 `RelayInfo.WssEventConsumedQuota` 为补差基准，日志 quota = 实际净扣款，Other 快照补记 `pre_consumed_quota`/`reconcile_delta`），计量状态收敛到单一结算循环属主（reader 只转发），失败事件实扣成功才累计且错误以 skip-retry 显式上抛；验证含 WSS 计费用例与 `-race` 实跑（`internal/relay/channel/openai` 与 `internal/domain/billing` 双包通过，缺陷 14 的竞态实证补齐；顺带修复 `-race` 暴露的 `infra/log` `logCount++` 无锁计数与两处测试 fixture 全局变量恢复写竞态）。
- 更新记录（2026-09-12 复核批次 A 提交 `b17d321e`，更正上一条）：缺陷 4 的 billing 层机制与 7 个用例成立（`-race` 双包复跑通过），但缺陷 12、14 的完成标记**降级为未完成**——声称新增的回归用例 `TestOpenaiRealtimeHandlerFailedEventNotRebilled` 在提交中不存在（`TestOpenaiRealtimeHandlerConcurrentInterleaveIsLossless` 仅剩文件末尾孤儿注释、无函数体），上一条"缺陷 14 的竞态实证补齐"不成立，`-race` 通过仅覆盖既有用例；且复核发现 `b17d321e` 引入新缺陷 33（结算循环错误路径停止消费 `meterChan`，reader 阻塞在投递时 `readerWG.Wait()` 永久挂死）与 34（UsePrice 收尾对汇总重算单份按次价、补差退还 N−1 份，整场净扣仅一份，与"按每个 response.done 事件收取"的声明矛盾，无 UsePrice 收尾用例）。同日复核补齐修复 `6a5f7f61d`：结算循环出错转 drain 模式、会话元数据（音频格式）与本地计数标志收敛到结算循环/父 goroutine 单一写入者、UsePrice 收尾 finalQuota 改用 `WssEventConsumedQuota` 口径；补齐上述两个用例与 `TestPostWssConsumeQuotaUsePriceSessionNetsPerEvent`。死锁回归已做双向实证：临时还原 `return` 行为时 5 秒看门狗必挂、修复后通过；`-race` 双包与 relay/domain/infra/store/httpapi 全量 `go test`、`go build ./...` 均通过。缺陷 4/12/14/33/34 现均确认修复，其中 12/14/33/34 由复核补齐修复完成（非 `b17d321e`）。

## 对首轮报告的总体审核结论

首轮报告 27 条缺陷的事实证据经五个并行子代理两轮独立审查、主线在当前 checkout 逐条定位、以及统一复核代理逐条核对后：25 条确认成立（其中 9 条分级按生产风险口径调整、2 条表述修正），2 条否定（首轮 18、19）；另发现首轮遗漏缺陷 7 项（其中 1 项 P1）和 2 项对既有条目的实质细化补充。首轮 6 个 P0 在生产风险口径下均调整为 P1，其中"上下文档位含输出"与"WSS 计费单位不一致"附带条件升级条款：若目标生产已启用上下文分档定价或按次 realtime 模型，应按 P0 处理。合计本轮有效缺陷 34 项（含复核 `b17d321e` 新增的 33/34）：**P0-0、P1-16、P2-12、P3-6**。`9bfedcb76` 落地旧列删除与服务端投影后，原"`billing_details` 未成为唯一权威来源、旧列未退役、统计与导出数据源未切换"三项阻塞已解除；批次 A `b17d321e` 及其复核补齐修复 `6a5f7f61d`落地后，WSS 结算的账务错误（缺陷 4/12/14）与复核新增的 33/34 已解除。结论仍为当前分支不能通过验收：上下文档位计入输出、仅配新价格表的模型无法服务正式流量、旧迁移完成标记仍跳过最终结构校验（删列落地后该缺口的补救窗口进一步收窄）等 P1 项尚未解决。

## 缺陷清单

1. 【已完成】P1-旧 Token 总量列未退役，billing_details 未成为唯一权威存储
   `logs` 模型仍声明并写入 `prompt_tokens`/`completion_tokens`（internal/store/log/log.go:31-32、:329-330），迁移完成分支只写 version 完成标记（log_billing_token_details_migration.go:106），全仓库不存在对两个旧列的 DropColumn，`billing_details_json.go` 头部注释仍声明核心总量由旧兼容列承载。阶段 0"完成迁移后删除旧列、不保留旧 Token 存储和业务回退读取"未完成，`billing_details` 尚未成为持久化 Token 用量的唯一权威来源；存量生产路径当前仍可运行，但分支的核心验收目标未达成。
   影响文件：`internal/store/log/log.go`、`internal/store/db/migrate/log_billing_token_details_migration.go`、`internal/domain/billing/billing_details_json.go`
   【已完成 2026-09-12（`9bfedcb76`）】Log 模型移除旧列字段（改为 `gorm:"-"` 的 wire 投影字段）；新增 `dropLegacyLogTokenAggregateColumns` 在 backfill 成功后删除两旧列（原生 DROP COLUMN 优先＋Migrator 兜底＋删后复核，幂等、失败阻断启动，回滚约束要求恢复迁移前备份）；`billing_details_json.go` 注释改为声明 billing_details 为唯一权威来源，唯一汇总公式收敛至 `billing/contract` 契约叶子包。

2. 【已完成】P1-阶段 5 统计、汇总、导出仍读取旧 Token 列且服务端投影缺失
   实时 TPM SQL（log.go:675、:746）、`SumUsedToken`（:765-766）、quota_data 生成（:359）与重算（usedata.go:287、:329）、key-query CSV 导出（key-query-logs-table.tsx:184-185）全部读取旧聚合列或旧字段；日志 API 只透传 `billing_details` 原始字符串，未按环节四.1 由服务端投影必要 Token 总量，损坏 JSON 只能依赖前端识别。写入端同样未切换：定制音色试听把同一字符数同时写入输入、输出两个旧 Token 列（custom_voice.go:450-451），从源头污染 TPM 与汇总（Token 榜单读取的 `quota_data.token_used` 是其下游，不另计条）。最终删列后上述查询将直接失败。
   影响文件：`internal/store/log/log.go`、`internal/store/usedata/usedata.go`、`internal/infra/custom_voice/custom_voice.go`、`internal/httpapi/controller/log/log.go`、`internal/store/usedata/usedata_rankings.go`、`web/src/features/key-query/key-query-logs-table.tsx`
   【已完成 2026-09-12（`9bfedcb76`）】实时 TPM 拆为 RPM SQL＋应用层按唯一汇总公式从 `billing_details` 求和，删除零调用死代码 `SumUsedToken`；quota_data 生成改用与 billing_details 同源的归一化内存聚合，重算（含失败日志）改为读取 `billing_details`，损坏 JSON 带 log id 显式报错；定制音色试听不再写入伪 token 聚合；新增 `projectBillingTokenAggregates` 覆盖四个日志查询端点（含 key-query 使用的 `/api/log/token`）做服务端投影，key-query CSV 导出随投影自动切换，前端零改动。

3. P1-上下文档位错误计入输出量（条件升 P0）
   `ContextTokensForTier()` 使用 `TotalProcessedTokens()`＝输入侧总量＋输出总量（pricing.go:16-25、billing_usage.go:99-100），该值同时进入旧上下文配置匹配与新价格表计划查询（billing_quota.go:133），输出变长会把相同输入推入更高档位并改变计价，直接违反"上下文档位只按普通输入＋缓存创建＋缓存读取匹配，不含输出"；预扣侧只用输入（price.go:61）与结算侧不一致，影子对拍还将该差异标记为"预期"（billing_shadow.go:277），属掩盖问题。资金方向错误确定，但仅在启用上下文分档定价的模型上发生：若目标生产已启用分档定价，本条应按 P0 处理。
   影响文件：`internal/domain/billing/pricing.go`、`internal/domain/billing/billing_usage.go`、`internal/domain/billing/billing_quota.go`、`internal/relay/helper/price.go`、`internal/domain/billing/billing_shadow.go`

4. 【已完成】P1-WSS 事件实扣与会话汇总记账单位不一致（首轮 4＋14 合并，条件升 P0）
   `UsePrice=true` 时事件预扣直接返回（quota.go:71-73），收尾 `PostWssConsumeQuota` 全程无资金扣减并对会话按 0 结算（quota.go:199-210），按次配置下整场会话净扣为 0 而日志记有费用；反向（ratio 模式＋按次价格计划）每个事件各扣一次按次费、汇总只记一份。同时每个事件按当时档位与价格实扣取整，收尾又对汇总用量用最终 PriceData 重算一次写入日志与统计（quota.go:107、:163、:231），多事件取整、会话中档位变化或改价后，日志 quota 与真实资金扣款不相等。两个表现同根因（事件级实扣与会话级汇总记账未统一），合并为一条；需 WSS 流量与相应计费配置叠加，若目标生产存在按次 realtime 模型，本条应按 P0 处理。
   影响文件：`internal/domain/billing/quota.go`、`internal/domain/billing/billing_quota.go`、`internal/relay/channel/openai/relay_openai.go`
   【已完成 2026-09-12（批次 A：WSS/Realtime 会话计费收口）】删除 `PreWssConsumeQuota` 的 `UsePrice` 早退，UsePrice（按次价）与 ratio 统一按事件实扣（按次计划下每个 response.done 事件收一次按次费）；新增 `RelayInfo.WssEventConsumedQuota` 累计事件实扣，收尾 `PostWssConsumeQuota` 用最终 PriceData 对汇总重算 finalQuota 后经 `reconcileWssClosingQuota` 多退少补（复用 `PostConsumeQuota` 正/负路径），日志 Quota = finalQuota = 实际净扣款，Other 补记 `pre_consumed_quota`/`reconcile_delta` 可核对；补差失败（余额不足/DB 错误）落 LogTypeError（billing_cause=closing_reconcile_failed，含 settle_quota 与价格快照）并返回 skip-retry 错误。
   【复核附注 2026-09-12】`b17d321e` 的本条修复在 UsePrice（按次价）下不完整：按次价为固定价、与 token 无关，收尾对汇总用量重算的 finalQuota 恒为一份，`reconcileWssClosingQuota` 会把事件级多扣的 N−1 份退还，整场净扣只剩一份，与"按每个 response.done 事件收取"的声明矛盾（缺陷 34），且提交内无 UsePrice 收尾用例；ratio 路径（7 个用例）成立。复核补齐修复 `6a5f7f61d`将 UsePrice 收尾 finalQuota 改为 `WssEventConsumedQuota` 口径后本条完整成立。

5. P1-旧迁移完成标记跳过最终结构校验
   迁移看到 version=1 全局完成标记直接返回（log_billing_token_details_migration.go:77-88），行查询只选择 `billing_details_version < 1`（:98），slave 门禁只检查同一标记与两列存在（log_billing_details_slave.go:41-48）。`schema_version` 未升版而 JSON 语义已改为完整数字结构，持有旧完成标记或已写入稀疏 v1 数据的日志库在本次升级中不会被按最终字段语义、总量约束与删列状态重检，违反"不得仅因列非空或存在旧完成标记就跳过本次升级"；主要在中间构建升级路径触发。
   影响文件：`internal/store/db/migrate/log_billing_token_details_migration.go`、`internal/store/db/migrate/log_billing_details_slave.go`、`internal/store/log/log.go`

6. P1-仅配置新价格表的模型无法进入正式请求
   正式请求与 Responses 入口先调用 `ModelPriceHelper()`，该函数只读旧模型价格与倍率，旧倍率缺失即报错终止（price.go:123-125；relay.go:158-161、responses_handler.go:161-165 将其作为 `ErrorCodeModelPriceError` 返回），而结算侧本身能读取新关系表（billing_quota.go:120-121）。只配置有效 Token、按次或免费新价格表的模型无法服务正式流量，阶段 3/4 的生产接入失败；失败显式、不产生错账，属功能整体不可用。
   影响文件：`internal/relay/helper/price.go`、`internal/httpapi/controller/relay/relay.go`、`internal/relay/handler/responses_handler.go`、`internal/domain/billing/billing_quota.go`

7. P1-Gemini 缓存模态未建模并固定从文本扣除
   `GeminiUsageMetadata` 未定义 `cacheTokensDetails`，上游该字段被静默丢弃（internal/domain/shared/gemini.go:591 附近），归一化只拿到聚合 `cachedContentTokenCount` 并固定从 TEXT 输入扣除（billing_usage.go:316）。非文本缓存可能触发 textCount<0 的显式报错导致请求失败，其余情况模态错分；验收样例"文本缓存 100、音频缓存 100"无法正确表达，违反环节二"有明确缓存模态数据时精确扣除"的要求。
   影响文件：`internal/domain/shared/gemini.go`、`internal/domain/billing/billing_usage.go`

8. P1-显式文本拆分绕过规范公式，落库明细与总量可能不一致
   `deriveCanonicalTextSplits()` 在显式文本字段存在时直接采信、不再按总量扣除已知模态（billing_details_json.go:101、:132）；归一化对明细与总量的部分矛盾只输出告警不阻断（billing_usage.go:448-495），持久化 JSON 不含总量字段、读取端无法校验唯一汇总公式（billing_details_json.go:247-282 仅校验非负与缓存分档）。落库明细与计费用量口径可能不一致，违反"计费内存结果、落库 JSON 和读取端按公式还原的总量一致"。
   影响文件：`internal/domain/billing/billing_usage.go`、`internal/domain/billing/billing_details_json.go`、`web/src/features/usage-logs/lib/billing-details.ts`

9. P1-历史迁移接受明细小于旧总量的矛盾数据
   `deriveRemainingTokenDetails()` 在显式文本已存在时只拒绝明细之和大于旧聚合、不拒绝小于（log_billing_token_details_migration.go:408-416 输入、:431-438 输出）。旧输入 100、显式文本 60、其余来源为 0 会被接受并迁移为只能还原 60 的 JSON，旧总量被静默截断，违反"不同来源之间的矛盾必须显式阻断并给出可定位的记录信息"。
   影响文件：`internal/store/db/migrate/log_billing_token_details_migration.go`

10. P1-Token 子项可见性未由服务端投影执行
    `token_breakdown`/`audio_tokens` 是明确的服务端配置项（internal/config/console/config.go:84-85），但 `filterUsageLogFieldsForRole` 只裁剪顶层字段与 `Other`（controller/log/log.go:141、:266），没有 `billing_details` JSON 子项裁剪，普通用户 HTTP 响应仍携带完整 Token 明细；前端 `isVisible` 门控（details-dialog.tsx:1381）只是展示层裁剪。违反用户确认的"Token 隐藏需独立展示投影并保留其他可见子项"，且列表、详情、导出规则不一致。
    影响文件：`internal/httpapi/controller/log/log.go`、`internal/config/console/config.go`、`web/src/features/usage-logs/components/dialogs/details-dialog.tsx`

11. P1-结算失败恢复状态机不满足原子性与补偿要求
    `SettleBilling()` 返回错误后调用方仅记日志并继续写成功消费日志（usage.go:423）；`BillingSession.Settle()` 先结算资金再调整 Token，Token 失败仍置 `settled=true`（service.go:118-126）导致无法补齐；退款先置终态再异步执行、失败仅 SysLog（service.go:135 附近）；预扣回滚失败后清零待补偿状态（service.go:203-209）。资金扣款、用户钱包与 Token 窗口/周期额度之间没有可靠的原子处理或持久补偿，违反阶段 4 对失败重试与并发一致性的要求。
    影响文件：`internal/domain/billing/service.go`、`internal/domain/billing/usage.go`、`internal/domain/billing/quota.go`

12. 【已完成】P1-WSS 失败事件重复累计且收尾错误被忽略
    `preConsumeUsage()` 先把事件累入 `sumUsage` 再扣款（relay_openai.go:475-484）；事件扣费失败时该份 usage 未清空（:391 未到达），连接收尾会再次处理同一份数据并以 `_ =` 丢弃计费错误、最终返回成功（:459-467）。失败事件可能被重复累计，部分扣款已成功时还会再次触碰资金状态，违反"不重复结算、失败可观测"。
    影响文件：`internal/relay/channel/openai/relay_openai.go`
   【已完成 2026-09-12（批次 A：WSS/Realtime 会话计费收口）】`preConsumeUsage` 改为"实扣成功才累计进 sumUsage"；事件计费失败立即经 errChan 触发会话 teardown，该份用量不累计、不重试；连接收尾的剩余用量结算不再以 `_ =` 吞错，失败记 LogError（含 eventConsumedQuota）并返回带 `ErrOptionWithSkipRetry` 的错误（防止 relay controller 重跑整场会话造成双重计费）。新增用例 `TestOpenaiRealtimeHandlerFailedEventNotRebilled` 验证失败事件不进入汇总、不重复处理、错误可观测且 skip-retry。
   【复核降级 2026-09-12：`b17d321e` 未完成本条】声称新增的用例 `TestOpenaiRealtimeHandlerFailedEventNotRebilled` 在提交中不存在（billing_usage_tag_test.go 全文无该函数），"失败事件不被重复累计"未经任何验证；且结算循环计费失败分支 `settleErr` 赋值、`errChan` 投递后直接 `return`（relay_openai.go 结算循环各 error 分支），此后 `meterChan`（缓冲 64）无消费者，reader 阻塞在 `meterChan <-` 投递时父 goroutine 的 `readerWG.Wait()`（位于 `close(meterChan)` 之前）永久挂死——即缺陷 33；触发条件（事件计费失败＋事件积压超缓冲）正是本条引入逐事件实扣后更常见的路径。
   【已完成 2026-09-12（复核补齐修复 `6a5f7f61d`）】结算循环出错后不再 return，转 drain 模式继续消费 `meterChan` 直到 close（错误只记录/投递一次，后续事件只消费不处理、不重试计费）；补齐 `TestOpenaiRealtimeHandlerFailedEventNotRebilled`：事件 1 计费成功、事件 2 余额不足，断言 skip-retry 错误、sumUsage 仅含事件 1、钱包净扣与事件实扣相符，并以 300 个积压事件超过 `meterChan` 缓冲作死锁回归（临时还原 `return` 行为实证该用例 5 秒看门狗必挂，修复后 `-race` 通过）。

13. P1-Claude 流式合并改写明确缓存分档并丢失显式零值
    流式 merge 只在 incoming 分档大于 0 时覆盖当前值（relay_claude.go:795-804），显式 0 无法纠正先前非零值；随后的规范化把未分档剩余量折入 5m（helper/convert.go:228-236），上游显式 `5m=40、1h=30、total=100` 会被改写为 70/30，下行修正场景还会造成 5m+1h>total 使落库 JSON 在读取端显式报错。违反"上游明确返回分档时保留分档值，包括显式 0"。
    影响文件：`internal/relay/channel/claude/relay_claude.go`、`internal/relay/helper/convert.go`

14. 【已完成】P1-WSS 双向读取与收尾共享计量状态存在数据竞态（新增）
    client reader goroutine 写 `localUsage`（relay_openai.go:340-343）与 target reader goroutine 读写并复位同一 `localUsage` 指针（:393、:401-405、:434-437）无任何锁同步，双方还经 `preConsumeUsage` 并发写 `sumUsage`（:385、:409）；select 退出后父 goroutine 再读（:450-467）与可能未结束的子 goroutine 之间同样无同步。正常双向音频流即可形成交错，int 丢失更新导致少计；此为代码级冲突链推演结论，未用 `-race` 实跑复现。
    影响文件：`internal/relay/channel/openai/relay_openai.go`
   【已完成 2026-09-12（批次 A：WSS/Realtime 会话计费收口）】`OpenaiRealtimeHandler` 重构为计量状态单一属主：client/target reader 只负责读、解析与转发，token 计数与 `pendingUsage`/`localUsage`/`sumUsage` 全部经 `meterChan` 收敛到单一结算循环 goroutine（`info.RealtimeTools` 的读写随之同 goroutine 化）；父 goroutine 收尾先显式 Close 双向连接解除 reader 阻塞，`readerWG.Wait()` + `close(meterChan)` + `<-settleDone` join 全部子 goroutine 后独占执行收尾结算。`-race` 实跑通过（含双向并发交错用例 `TestOpenaiRealtimeHandlerConcurrentInterleaveIsLossless`，断言 sumUsage 无丢失更新——补齐本报告"验证边界"缺失的竞态实证；顺带修复实证暴露的 `infra/log` `logCount++` 无锁竞态）。
   【复核降级 2026-09-12：`b17d321e` 未完成本条】声称的 `-race` 实跑仅覆盖既有用例；"双向并发交错用例 `TestOpenaiRealtimeHandlerConcurrentInterleaveIsLossless`"在提交中不存在（billing_usage_tag_test.go 文件末尾仅剩三行孤儿注释、无函数体），"竞态实证补齐"不成立；且单一属主重构不完整——target reader 仍写 `info.InputAudioFormat`/`OutputAudioFormat`（relay_openai.go SessionUpdated/Created 分支），结算循环经 `CountTokenRealtime` 并发读且无同步；结算循环内 `SetContextKey` 与 reader 侧日志对 gin context 的并发读亦构成并发写风险；收尾 join 顺序还引入缺陷 33 的死锁。
   【已完成 2026-09-12（复核补齐修复 `6a5f7f61d`）】音频格式写入移入结算循环 SessionUpdated/Created case（reader 变为纯读/解析/转发）；本地计数标志改由结算循环记录 `mixedLocalCount` 布尔、父 goroutine join 后统一 `SetContextKey`（gin context 在 join 后只剩单一写入者）；补齐 `TestOpenaiRealtimeHandlerConcurrentInterleaveIsLossless`：client 噪声事件与 5 个官方 usage response.done 并发交错注入，断言 sumUsage 精确等于官方 usage 之和（60×5/40×5）、钱包净扣＝事件实扣累计（500），`-race` 实跑通过。

15. P2-明确全零 usage 被当作缺失 usage 处理
    通用路径在输入输出聚合为 0 时拒绝序列化合法全零 JSON，并进入"上游没有计费信息"的重试/错误分支（usage.go:174-175、:363-383；quota.go:187、:366、:490 以总量 0 触发空 usage 处理）。真实上游显式全零极罕见，失败方向是错误重试或异常记录而非错账，故按生产风险由首轮 P1 降为 P2；但"存在明确全零 usage 时允许保存合法全零 JSON"的验收条款不满足。
    影响文件：`internal/domain/billing/usage.go`、`internal/domain/billing/quota.go`、`internal/domain/billing/billing_usage.go`

16. P2-日志写入边界不校验 billing_details JSON
    `RecordConsumeLog()` 只判断 `BillingDetails` 字符串非空（log.go:312-317），未在系统边界重新校验 schema、数值与包含关系，任意坏字符串会随行级版本一起入库；当前调用方大多先序列化，但该边界违反"新日志写入前校验"的强制要求，也无法防止其他入口把坏 JSON 变成"已迁移"数据。
    影响文件：`internal/store/log/log.go`

17. P2-上下文计价配置错误在不同入口传播不一致
    通用与 WSS 收尾路径在上下文价格匹配失败时记录日志后继续（usage.go:158-163、quota.go:157-161），WSS 预扣路径则直接返回错误（quota.go:88-90）。相同配置错误在不同入口有不同结果，可能生成缺少档位解释的成功账单，应统一显式失败或明确降级策略。
    影响文件：`internal/domain/billing/usage.go`、`internal/domain/billing/quota.go`、`internal/domain/billing/billing_quota.go`

18. P2-模型测试费用与 Token 明细使用两套算法（验收必须修复项）
    模型测试入口走旧 `ModelPriceHelper`（channel_test_handler.go:317，仅读旧配置，仅配新价格表时同样无法进入），quota 按旧 prompt/completion 公式或旧按次价手算（:516-523），日志明细又调用新归一化构建，未经统一新价格表结算入口（billing_quota.go:107、:120、:198）。按用户最终口径"模型测试 Token 费用必须走统一新价格表"这是硬性验收要求、必须修复；因测试请求无用户钱包扣减、生产资金风险有限，按生产风险定为 P2 并标注。
    影响文件：`internal/httpapi/controller/channel/channel_test_handler.go`、`internal/relay/helper/price.go`、`internal/domain/billing/billing_quota.go`

19. P2-消费日志写入失败后汇总仍继续增加
    消费日志异步 `Create` 失败仅输出系统日志（log.go:350-354），随后无条件继续执行 `LogQuotaData`（:355-361），没有持久重试或补偿状态。数据库故障时会出现资金已结算、消费日志缺失而请求次数、quota、token_used 继续增加的对账断裂。
    影响文件：`internal/store/log/log.go`、`internal/store/usedata/usedata.go`

20. P2-管理员重算与待刷新缓存重复累计同一批消费（新增，SQLite 复现）
    管理员重算删除并重建时间范围内 quota_data 时，不清理也不合并在途的进程内待刷缓存（usedata.go:268-388 与 :153-173 无协调），窗口重叠时后台 flush 会把同一批请求再次累加。已在 /tmp overlay 中以生产函数级复现：单笔消费 150 tokens/20 quota，重算后再 flush 变为 300/40。仅统计失真、由管理操作触发，资金结算不受影响，故定 P2。
    影响文件：`internal/store/usedata/usedata.go`、`internal/httpapi/controller/usedata/usedata.go`

21. P2-quota_data 同桶重复行使后续增量更新放大（新增，机制修正）
    保存流程查到已存在行即按 WHERE 匹配全部同键行做增量 UPDATE（usedata.go:161-171），而 `quota_data` 没有唯一约束，重复行一旦存在（如多实例各自插入），后续每次 flush 都按行数放大聚合。复现为预置重复行后验证放大（count 2→4）；未复现双节点并发首次插入的竞态本身，故不按并发缺陷计。
    影响文件：`internal/store/usedata/usedata.go`

22. P2-汇总 flush 写失败仍清空缓存并报告保存成功（新增，失败注入复现）
    `SaveQuotaDataCache` 中 update/insert 错误既不收集也不重试（usedata.go:168、:183-184），遍历结束后无条件清空内存待刷桶并打印"保存成功"（:171-172）。已用关闭 SQLite 连接的真实失败注入复现：待刷聚合被丢弃且无任何失败表现，违反"不得吞错或伪造成功"。
    影响文件：`internal/store/usedata/usedata.go`

23. P2-管理员重算无界加载且入口无时间范围上限（新增）
    重算把范围内全部成功与失败日志一次性 `Find` 进内存、无 LIMIT（usedata.go:285-308），控制器只校验起止非零与先后、未限制最大天数（controller/usedata.go:134-139）。大规模日志库下有高内存与长事务风险，违反环节四.12"统计不得无界加载全部日志"。
    影响文件：`internal/store/usedata/usedata.go`、`internal/httpapi/controller/usedata/usedata.go`

24. 【已完成】P2-MySQL/PostgreSQL 迁移验收测试夹具失效（新增，测试缺陷）
    DSN 门控迁移测试仍期望稀疏 JSON 结构（log_billing_details_dsn_migrate_test.go:262），与现行完整数字结构输出矛盾；用例被 `TEST_BILLING_DETAILS_DB_DSN` 门控默认跳过，失败被掩盖，三库迁移验收实际不可执行。空库先写完成标记再 seed/删列/回填的夹具顺序还可能跳过回填路径。
    影响文件：`internal/store/db/migrate/log_billing_details_dsn_migrate_test.go`
    【已完成 2026-09-12（`9bfedcb76`）】迁移断言 `wantDetails` 改为期望完整数字结构；夹具先补旧聚合列并以 map 插入真实历史行，seed/删列前显式作废完成标记（`invalidateBillingMigrationMarker`），回填路径不再被跳过。

25. P2-前端输入输出主值仅取文本拆分，详情存在旧聚合回退（首轮 2 细化）
    主表合计投影只映射 `text_input`/`text_output`（billing-details.ts:243-244、common-logs-columns.tsx:719），音频、图像、视频、文档子项不进主值，多模态记录被低估；详情"总请求输入"在有缓存时回退旧 `log.prompt_tokens`（details-dialog.tsx:878），与新明细口径不一致。统一解析器本身已具备全部子项，问题集中在投影消费端。
    影响文件：`web/src/features/usage-logs/lib/billing-details.ts`、`web/src/features/usage-logs/components/columns/common-logs-columns.tsx`、`web/src/features/usage-logs/components/dialogs/details-dialog.tsx`

26. P2-缺少与最终删列结构匹配的可执行备份恢复 runbook
    仓库仅有升级顺序原则与通用备份提醒（PRD.md:211 一带），没有覆盖本次日志库实际备份、停止并排空旧写入节点、最终删列结构检查、恢复到与旧二进制匹配备份、恢复校验与失败分支的可执行 runbook，违反横切验收.3"删除列的回滚必须包含数据库恢复步骤"，属交付物缺口。
    影响文件：`docs/PRD/计费重构/PRD.md`（交付缺口，另涉及 `docs/zh/installation/` 升级维护文档）

27. P3-key-query 日志列表未使用统一列表组件
    key-query 日志列表仍使用 `Card` 包裹手拼 `Table`（key-query-logs-table.tsx:463、:522），未复用 `DataTablePage`＋`SectionPageLayout`，违反项目列表页规范与环节四.13；不影响计费正确性与权限，由首轮 P2 降为 P3。
    影响文件：`web/src/features/key-query/key-query-logs-table.tsx`

28. P3-价格快照缺数量 fallback 与详情单位展示不完整（首轮 21 细化）
    快照缺少已保存结算数量时，fallback 把 input/output 组件映射到文本量、无法表达未分档但按 5m 收费的缓存写入（billing-details.ts:470-490）；详情快照单元格只显示单价与币种、无 `/M` 或按次单位（details-dialog.tsx:796），而列表已正确显示单位。仅影响缺数量的旧快照解释，属展示一致性问题，降为 P3。
    影响文件：`web/src/features/usage-logs/lib/billing-details.ts`、`web/src/features/usage-logs/components/dialogs/details-dialog.tsx`

29. P3-管理员模型筛选与统计 LIKE 语义不一致
    管理员列表把原始模型名直接用于参数化 LIKE（log.go:444-445），统计查询先转换并转义通配符（:626-631）；模型名含 `_`/`%` 时两处匹配集合可能不同。查询仍为参数化，非 SQL 注入，仅语义一致性问题。
    影响文件：`internal/store/log/log.go`

30. P3-QuotaStat.Tpm 命名承载区间 Token 总量
    预聚合结构把任意筛选区间的 `sum(token_used)` 命名为 `Tpm`（usedata.go:233-261）；日志统计接口实际会用最近 60 秒实时查询覆盖 TPM，线上时间窗未错，但命名会误导后续复用。
    影响文件：`internal/store/usedata/usedata.go`

31. P3-schema_version 缺失与未知版本诊断混淆
    后端把字段缺失导致的 version=0 与真正未知版本统一报"unknown version: 0"（billing_details_json.go:171-173）；前端把 schema_version 路径上的任何校验错误都归类为 unknown_version（billing-details.ts:138-141）。两侧均拒绝非法 JSON，无安全边界失败，仅影响诊断精度。
    影响文件：`internal/domain/billing/billing_details_json.go`、`web/src/features/usage-logs/lib/billing-details.ts`

32. P3-详情弹窗重复解析同一 Other JSON（新增）
    详情弹窗两个组件各自调用 `parseLogOther` 且无缓存（details-dialog.tsx:962、:1026；format.ts:98），同一条记录在一次渲染中被重复解析。仅为不必要的重复开销，低风险。
    影响文件：`web/src/features/usage-logs/components/dialogs/details-dialog.tsx`

33. 【已完成】P1-WSS 结算循环错误路径停止消费 meterChan 导致会话 goroutine 死锁（复核 `b17d321e` 新增）
    `b17d321e` 重构后，结算循环计费/tokenizer 失败分支在 `settleErr` 赋值、`errChan` 投递后直接 `return`，此后 `meterChan`（缓冲 64）无消费者；任一 reader 阻塞在 `meterChan <-` 投递时，父 goroutine 的 `readerWG.Wait()`（位于 `close(meterChan)` 之前）永久阻塞，会话全部 goroutine 挂死、连接资源无法释放（连接 Close 无法解除 channel send 阻塞）。触发条件＝会话内事件计费失败（余额不足等，P1-4 逐事件实扣后更常见）＋事件积压超缓冲；`b17d321e` 声称的失败事件回归用例不存在，该缺陷未被提交内验证发现。
    影响文件：`internal/relay/channel/openai/relay_openai.go`
    【已完成 2026-09-12（复核补齐修复 `6a5f7f61d`）】结算循环出错后不再 return，转入 drain 模式继续消费 `meterChan` 直到 close（错误只记录/投递一次，后续事件只消费不处理、不重试计费，P1-12 语义不变）；`TestOpenaiRealtimeHandlerFailedEventNotRebilled` 以 300 个积压事件超过缓冲作死锁回归——临时还原 `return` 行为实证 5 秒看门狗必挂，修复后 `-race` 通过。

34. 【已完成】P1-WSS 按次价会话收尾净扣仅一份（复核 `b17d321e` 新增；条件升 P0，与缺陷 4 同口径）
    按次价为固定价、与 token 用量无关：事件级每个 response.done 实扣一份（`WssEventConsumedQuota` = 份数 × 当时单价），但 `b17d321e` 的 `PostWssConsumeQuota` 收尾仍对汇总用量经 `normalizedRealtimeQuota` 重算 finalQuota——固定价语义下恒为一份，`reconcileWssClosingQuota` 补差把事件级多扣的 N−1 份退还，整场会话净扣只剩一份按次价，属漏收（少计费），与 `b17d321e` 提交信息及本报告原更新记录声明的"按次价按每个 response.done 事件收取"直接矛盾；提交内无 UsePrice 收尾用例，缺陷未被验证发现。若目标生产存在按次 realtime 模型，本条应按 P0 处理（与缺陷 4 的条件升级条款同口径）。
    影响文件：`internal/domain/billing/quota.go`、`internal/domain/billing/billing_shadow.go`
    【已完成 2026-09-12（复核补齐修复 `6a5f7f61d`）】`PostWssConsumeQuota` 在 `PriceData.UsePrice` 时将 finalQuota 改为 `relayInfo.WssEventConsumedQuota`（份数 × 当时单价，含会话中改价），补差自然为 0、日志 quota = 钱包净扣 = 事件实扣累计；影子对拍补"realtime 按次价按每个 response.done 事件实扣（预期差异）"分类提示，避免落入 unclassified 告警；新增 `TestPostWssConsumeQuotaUsePriceSessionNetsPerEvent` 断言两事件净扣、日志 quota 均为 2 份且 `reconcile_delta=0`。

## 验证边界

主线与子代理已执行的检查：目标后端包 `go test`（billing、store、httpapi/controller、relay、app）通过；`bun run typecheck`、`bun run lint` 通过；前端解析器 `billing-details.test.ts` 21 项通过；统计重复累计与 flush 失通过 /tmp overlay 在 SQLite 上以生产函数级复现（未修改仓库代码）。`bun run build` 两次因环境内存上限被杀（exit 137），前端生产构建未获得通过结论；其后的 Go 整体构建被缺失的 `web/dist` 阻断，均不构成计费源码编译失败证据。

以下范围未验证，不得由本地测试通过推定为通过：SQLite/MySQL 5.7.8+/PostgreSQL 9.6+ 的真实空库初始化、历史库升级、重复启动、删列前后中断与备份恢复（且 MySQL/PG 迁移用例因缺陷 24 当前不可执行）；真实 Claude、Bedrock、OpenRouter、OpenAI Chat/Responses、Gemini、audio、Realtime/WSS 上游联调；master/slave 与多实例部署下的改价传播、迁移与 flush 行为；真实并发扣款、退款与回调；代表性日志规模下的统计与重算性能；浏览器交互逐项实测。WSS 计量竞态（缺陷 14）最初为代码级冲突链推演；`b17d321e` 声称的竞态实证用例当时不存在（仅孤儿注释），复核补齐修复 `6a5f7f61d`落地 `TestOpenaiRealtimeHandlerConcurrentInterleaveIsLossless`（双向并发交错、断言无丢失更新）与 `TestOpenaiRealtimeHandlerFailedEventNotRebilled`（含缺陷 33 死锁回归，旧行为下实证必挂）后，`-race` 双包实跑通过；UsePrice 收尾口径由 `TestPostWssConsumeQuotaUsePriceSessionNetsPerEvent` 钉住；真实 WSS 上游联调仍缺。附件 `/tmp` 下的子代理记录、候选清单与复现测试为过程产物，未纳入仓库。

## 建议合并修复

以下分组于 2026-09-12 对照 HEAD `9bfedcb76` 逐条核实了各未决缺陷之间的文件与函数重叠后整理。分组原则：改动同一批文件/函数的条目并入同一 commit；同主题且存在修复依赖的条目并入同一批次（同 PR 内顺序提交）。已完成的缺陷（1、2、24，以及批次 A 范围内的 4、12、14 与复核新增 33、34）不参与分组。

### 批次 A：WSS/Realtime 会话计费收口（1 个 commit）
- P1-4（WSS 事件实扣与会话汇总记账单位不一致，条件升 P0）
- P1-12（WSS 失败事件重复累计且收尾错误被忽略）
- P1-14（WSS 双向读取与收尾共享计量状态存在数据竞态）

聚合依据：三条共享 `relay_openai.go` 的 `localUsage`/`sumUsage`/`preConsumeUsage` 与连接收尾结算块，以及 `quota.go` 的 `PreWssConsumeQuota`/`PostWssConsumeQuota`；14 的状态所有权/同步重构必然重写 12、4 修复所在的同一段代码，分开修等于同段代码三次返工。统一验证：WSS 计费用例 + `-race` 实跑（顺带补上验证边界中缺失的竞态实证）。

落地记录（2026-09-12）：批次 A 已以 `b17d321e` 提交，但复核确认其只完成了 P1-4 的 ratio 路径与机制骨架——12/14 声称的验证用例不存在，且引入缺陷 33（结算错误路径死锁）与 34（UsePrice 收尾净扣一份）；同日复核补齐修复 `6a5f7f61d`在同一批次范围内收口：结算循环 drain 模式、会话元数据/本地计数标志单一写入者、UsePrice 收尾按事件份数结算，并补齐 `TestOpenaiRealtimeHandlerFailedEventNotRebilled`、`TestOpenaiRealtimeHandlerConcurrentInterleaveIsLossless` 与 `TestPostWssConsumeQuotaUsePriceSessionNetsPerEvent` 三个用例（死锁回归在旧行为下实证必挂）。4/12/14/33/34 全部确认修复；本批次无遗留收尾项。

### 批次 B：计价准入与上下文档位统一（1 个批次，3 个 commit 顺序提交）
- commit B1：P1-3（上下文档位错误计入输出量，条件升 P0）＋ P2-17（上下文计价配置错误在不同入口传播不一致）
- commit B2：P1-6（仅配置新价格表的模型无法进入正式请求）
- commit B3：P2-18（模型测试费用与 Token 明细使用两套算法，验收必须修复项）

聚合依据：3 的 `ContextTokensForTier` 修复需同步调整 `billing_usage.go`、`billing_quota.go` 档位查询与预扣侧 `price.go` 的档位口径，并撤销 `billing_shadow.go` 中"分段档位 tokens 现含输出维度（PRD 阶段 2）"这条把口径差异标注为预期的掩盖——该条与 17 的错误传播点（`usage.go` 与 `quota.go` 的 `ApplyContextPricingForBillingUsage` 调用点）是同一批调用处；6 与 18 前后依赖（`ModelPriceHelper` 不放行仅配新价格表的模型时，18 的统一结算入口无从验证），且 B1/B2/B3 均修改 `price.go` 与 `billing_quota.go`。注意 `quota.go` 与批次 A 重叠，A 先落。

### 批次 C：迁移最终结构校验与矛盾数据拒绝（1 个 commit）
- P1-5（旧迁移完成标记跳过最终结构校验）
- P1-9（历史迁移接受明细小于旧总量的矛盾数据）

聚合依据：同在 `log_billing_token_details_migration.go`（5 的完成标记短路位于 `backfillLogBillingTokenDetails` 入口，9 的 `deriveRemainingTokenDetails` 在同一校验链），slave 门禁 `log_billing_details_slave.go` 随 5 的升版一并调整；`9bfedcb76` 删列落地后补救窗口进一步收窄，本批次宜最先合入。

### 批次 D：消费日志写入边界与 quota_data 统计链路（1 个批次，2 个 commit）
- commit D1：P2-16（日志写入边界不校验 billing_details JSON）＋ P2-19（消费日志写入失败后汇总仍继续增加）＋ P3-29（管理员模型筛选与统计 LIKE 语义不一致）
- commit D2：P2-20（重算与待刷新缓存重复累计）＋ P2-21（quota_data 同桶重复行放大）＋ P2-22（flush 写失败仍清空缓存并报告成功）＋ P2-23（重算无界加载且入口无时间范围上限）＋ P3-30（QuotaStat.Tpm 命名承载区间总量）

聚合依据：16 与 19 同在 `store/log/log.go` 的 `RecordConsumeLog` 及其失败分支（29 也在该文件的查询侧），一个 commit 避免三次触碰同一函数；20-23 与 30 全部集中在 `usedata.go` 的 `SaveQuotaDataCache`/`increaseQuotaData`/`RecalculateQuotaData` 同批函数，23 的入口上限可直接复用 `controller/usedata.go` 既有 `isUserQuotaRangeTooLong` 的模式。验证统一回归报告中的 SQLite 函数级复现场景 + store 测试。

### 批次 E：billing_details 规范公式与 presence 语义（1-2 个 commit）
- P1-8（显式文本拆分绕过规范公式，落库明细与总量可能不一致）
- P2-15（明确全零 usage 被当作缺失 usage 处理）
- P3-31 后端部分（schema_version 缺失与未知版本诊断混淆）

聚合依据：8 与 15 都修改 `billing_usage.go` 的归一化与序列化门控（`usage.go` 的 `promptTokens == 0 && completionTokens == 0` 跳过序列化门控、`quota.go` 的全零分支），31 在 `billing_details_json.go` 同一解析路径顺路修正；因 `usage.go`/`quota.go` 与批次 A/B 重叠，落点排在 A、B 之后；31 的前端分类修正随批次 H 提交。

### 批次 F：上游 usage 明细保真（1 个批次，2 个 commit）
- commit F1：P1-7（Gemini 缓存模态未建模并固定从文本扣除）
- commit F2：P1-13（Claude 流式合并改写明确缓存分档并丢失显式零值）

聚合依据：同主题（上游明确明细与显式零值优先于启发式折算），验证路径相同（协议转换 roundtrip + billing 归一化用例）；7 与批次 E 在 `billing_usage.go` 有函数级重叠（`buildGeminiBillingUsage` 与归一化门控），E 先落。

### 批次 G：Token 子项服务端可见性投影（1 个 commit）
- P1-10（Token 子项可见性未由服务端投影执行）

聚合依据：自包含特性改动（`controller/log/log.go` 的 `filterUsageLogFieldsForRole` 扩展 + `console` 配置联动）；`details-dialog.tsx` 的前端 `isVisible` 门控降级为纵深防御，该部分可与批次 H 合并提交以避免同文件冲突。

### 批次 H：前端 usage-logs 展示一致性（1 个 commit）
- P2-25（输入输出主值仅取文本拆分，详情存在旧聚合回退）
- P3-28（价格快照缺数量 fallback 与详情单位展示不完整）
- P3-32（详情弹窗重复解析同一 Other JSON）
- P3-31 前端部分（unknown_version 分类）

聚合依据：`billing-details.ts`、`details-dialog.tsx`、`common-logs-columns.tsx` 三文件交叉重叠，一次提交统一过 `typecheck`/`lint`/`build`；25 的旧聚合回退在 `9bfedcb76` 删列后已实际失效，修复直接对齐缺陷 2 已落地的服务端投影。

### 独立修复项（不并入批次）
- P1-11（结算失败恢复状态机不满足原子性与补偿要求）：设计级重构（`service.go` 的 `settled`/`refunded` 终态语义、退款异步补偿、`usage.go:423` 结算失败后仍写成功消费日志），与 A/B/E 在 `usage.go`/`quota.go` 重叠最大，应最后落地，避免整批反复 rebase。
- P2-26（缺少与最终删列结构匹配的可执行备份恢复 runbook）：纯文档交付，可与批次 C 同批提交（同属删列交付完整性），无代码冲突。
- P3-27（key-query 日志列表未使用统一列表组件）：独立前端重构，无依赖。

落地顺序建议：C（补救窗口收窄，宜最先）→ A → B → D（与 A/B 无文件冲突，可随时并行）→ E → F → G/H（前端收尾）→ P1-11。
