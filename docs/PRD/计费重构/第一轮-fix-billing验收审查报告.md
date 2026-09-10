合计P0-6个，P1-9个，P2-9个，P3-3个

# fix/billing 分支验收审查报告

- 审查日期：2026-09-09
- 审查分支：`fix/billing`
- 审查 HEAD：`7ec9466581e0d686a69a337722070576043bbfe1`
- 与 `dev` 的合并基线：`bfc384be536b563c9f481c272a392dbe35d5a6bb`
- 验收依据：[验收标准.md](验收标准.md)
- 分级口径：P0 表示直接阻断强制验收，或直接破坏 Token 权威数据、资金结算与生产入口正确性的问题；P1 表示重要功能或账务语义不满足验收；P2 表示边界、运维、展示或一致性缺口；P3 表示低风险维护问题。该口径是验收分级，不等同于生产事故分级。

当前分支不能通过验收。阶段 0～5 中已有大量基础能力落地：`billing_details` schema、归一化入口、Claude/OpenAI/Gemini 语义收敛、价格表物理模型、Decimal 计价、快照和权限/审计均有实现；但验收标准最核心的“Token 唯一权威来源、旧列删除、统计导出切换、入口价格选择、失败显式暴露”仍未完成，并且存在上下文档位与 WSS 结算两类直接账务错误。

## 缺陷清单

1. P0-旧Token列仍是运行期权威且未删除
   `logs` 模型仍声明并写入 `prompt_tokens`、`completion_tokens`，迁移完成分支只写入完成标记，没有清理旧列；`billing_details` 序列化注释也继续声明核心总量由旧兼容列承载。因此 `billing_details` 尚未成为持久化 Token 用量的唯一权威来源，阶段 0 的删列、旧字段退役和运行期无回退要求均失败。
   影响文件：`internal/store/log/log.go`、`internal/store/db/migrate/log_billing_token_details_migration.go`、`internal/domain/billing/billing_details_json.go`

2. P0-阶段5统计、汇总、导出与部分展示仍读取旧列
   TPM SQL、`SumUsedToken`、`quota_data` 生成与重算、key-query CSV、详情总输入仍依赖旧聚合列；服务端也没有将 `billing_details` 解析并投影为必要 Token 总量，损坏 JSON 只能依赖前端识别。前端展示的 input/output 又只映射 `text_input`/`text_output`，漏掉图片、音频、视频、文档等模态；定制音色试听还把字符数同时写入输入、输出 Token 列，继续污染统计。阶段 5 的唯一汇总公式、导出、统计和展示切换要求失败。
   影响文件：`internal/store/log/log.go`、`internal/store/usedata/usedata.go`、`web/src/features/key-query/key-query-logs-table.tsx`、`web/src/features/usage-logs/lib/billing-details.ts`、`web/src/features/usage-logs/components/dialogs/details-dialog.tsx`、`internal/infra/custom_voice/custom_voice.go`

3. P0-上下文档位错误包含输出
   `ContextTokensForTier()` 使用 `TotalProcessedTokens()`，而后者等于输入侧总量加输出总量；该值同时进入旧上下文配置和新价格表计划查询。输出变长会把相同输入推入更高档位并改变价格计划或倍率，测试还明确固定了这一错误行为，直接违反“上下文档位只按普通输入＋缓存创建＋缓存读取，不含输出”的要求。
   影响文件：`internal/domain/billing/pricing.go`、`internal/domain/billing/billing_usage.go`、`internal/domain/billing/billing_quota.go`、`internal/domain/billing/pricing_test.go`

4. P0-Realtime/WSS按价格模式事件计费与收尾结算相互矛盾
   旧 `PriceData.UsePrice=true` 时事件预扣直接返回，收尾却把请求级会话按 0 结算，同时按汇总结果更新统计和日志，可能形成日志有费用而最终净扣为 0；反之，新价格表为按次模式而旧标志为 false 时，每个事件都可能生成一份按次费用，汇总却只计算一份，存在漏扣或重复扣两个方向。该缺陷不是简单缺少一次收尾扣款，而是会话级与事件级收费单位未统一。
   影响文件：`internal/domain/billing/quota.go`、`internal/domain/billing/billing_quota.go`、`internal/relay/channel/openai/relay_openai.go`、`internal/relay/core/websocket.go`

5. P0-历史迁移复用旧完成标记并跳过已有数据最终校验
   行级版本与全局完成标记都仍是 1；迁移看到 version=1 全局标记直接返回，行查询也只选择 `billing_details_version < 1`，slave 门禁只检查同一个标记和两列存在。对已有稀疏 schema v1、行版本 1 或旧完成标记的日志库，本次升级不会按最终完整数字结构、总量约束和删列状态重检，违反“已有 `billing_details` 与旧完成标记不得跳过本次升级”的要求。
   影响文件：`internal/store/log/log.go`、`internal/store/db/migrate/log_billing_token_details_migration.go`、`internal/store/db/migrate/log_billing_details_slave.go`

6. P0-仅配置新价格表的模型无法进入正式请求
   正式请求和 Responses 入口仍先调用 `ModelPriceHelper()`，该函数只读取旧模型价格、倍率和旧上下文配置；旧倍率不存在时直接报错并终止请求，尚未进入能读取新关系表的统一价格选择入口。因此配置有效 Token、按次或免费新价格表但未配置旧价格时，正式请求无法通过入口；预扣与免费判断也依赖旧投影，阶段 3/4 的实际生产接入失败。
   影响文件：`internal/relay/helper/price.go`、`internal/httpapi/controller/relay/relay.go`、`internal/relay/handler/responses_handler.go`

7. P1-明确全零usage被当作缺失usage处理
   通用路径在输入输出聚合为 0 时拒绝序列化合法全零 JSON，随后进入“上游没有计费信息”的重试/错误分支；Claude、audio、WSS 也以总量为 0 触发空 usage 重试。验收允许真实上游明确返回全零 usage 时保存合法全零 JSON，当前实现无法区分“明确全零”和“缺失 usage”，可能导致已完成请求被错误重试或记录为异常。
   影响文件：`internal/domain/billing/usage.go`、`internal/domain/billing/quota.go`、`internal/domain/billing/billing_usage.go`

8. P1-Gemini缓存模态未建模并固定从文本扣除
   `GeminiUsageMetadata` 未包含 `cacheTokensDetails`，归一化只拿到聚合 `cachedContentTokenCount` 并固定从 TEXT 输入扣除，音频、图片、视频、文档缓存无法精确扣除。对验收样例“文本缓存 100、音频缓存 100”，当前会得到文本少扣、音频未扣的错误模态数量；总量即使巧合守恒也不能视为通过。
   影响文件：`internal/domain/shared/gemini.go`、`internal/domain/billing/billing_usage.go`、`internal/domain/billing/gemini_usage.go`

9. P1-显式拆分绕过规范公式且包含关系校验不完整
   `deriveCanonicalTextSplits()` 在显式文本字段存在时直接采信，不再按总量扣除已知模态；归一化对部分矛盾只输出 warning，严格 JSON 校验只检查非负和缓存分档，不校验 `reasoning_output <= text_output` 等包含关系，前端也只补充缓存分档检查。Gemini 输出还存在具体漏量路径：输出总量包含 candidates 与 thoughts，但显式 TEXT 只取 candidates 明细，导致 reasoning 保留为拆分时输出计价量小于实际输出总量。
   影响文件：`internal/domain/billing/billing_usage.go`、`internal/domain/billing/billing_details_json.go`、`web/src/features/usage-logs/lib/billing-details.ts`

10. P1-历史迁移接受明细小于旧总量的矛盾数据
    `deriveRemainingTokenDetails()` 只在显式文本不存在时用旧聚合扣除已知明细；显式文本已存在时只拒绝明细之和大于旧聚合，不拒绝小于旧聚合。旧输入 100、显式文本 60、其他输入来源为 0 会被接受并迁移为只能还原 60 的 JSON，静默丢失旧总量，违反不同来源矛盾必须显式阻断的要求。
    影响文件：`internal/store/db/migrate/log_billing_token_details_migration.go`

11. P1-Token子项可见性设置未由服务端执行
    服务端裁剪已支持详情总开关和 `billing_details` 整体隐藏，但 `token_breakdown`、`audio_tokens` 是明确配置项却没有对应 JSON 子项裁剪；关闭后 HTTP 响应仍携带完整 Token 明细，只是前端选择不渲染。阶段 5 要求列表、详情和导出规则一致并由服务端执行字段设置，该配置边界失败。
    影响文件：`internal/config/console/config.go`、`internal/httpapi/controller/log/log.go`、`web/src/features/usage-logs/components/dialogs/details-dialog.tsx`

12. P1-结算失败恢复状态机不满足原子性与补偿要求
    调用方在 `SettleBilling()` 返回错误后仅记录日志并继续写成功消费日志；`BillingSession.Settle()` 先结算资金再调整 Token，Token 失败后仍置 `settled=true`，后续无法补齐；退款先置终态再异步执行，失败仅日志；预扣回滚失败后清零待补偿状态；无 BillingSession 的路径中用户余额成功后 Token 失败也不反向补偿。资金、用户钱包、Token 窗口/周期额度之间没有可靠原子处理或持久补偿，阶段 4 的失败重试和并发一致性要求失败。
    影响文件：`internal/domain/billing/usage.go`、`internal/domain/billing/quota.go`、`internal/domain/billing/service.go`、`internal/store/user/user.go`、`internal/store/token/token.go`

13. P1-Realtime失败事件可能重复累计且收尾错误被忽略
    `preConsumeUsage()` 先把事件累计进 summary 再扣款；事件处理失败时退出协程且不清空 usage，连接收尾会再次处理同一份数据，并用 `_ =` 丢弃计费错误，最终仍返回成功。失败事件可能重复累计，若部分扣款已成功还会再次触碰资金状态，违反失败可观测、不重复结算和失败重试要求。
    影响文件：`internal/relay/channel/openai/relay_openai.go`

14. P1-Realtime汇总日志quota不等于逐事件实扣总额
    WSS 每个事件按当时用量、档位和价格计算并取整后实扣，收尾又对汇总 usage 重算一次并写入日志与统计，没有以实际事件扣款之和作为结算结果。多次取整、会话中档位变化或价格修改后，日志 quota、价格快照与真实资金扣款可能不一致，不能仅视为几分钱取整差异。
    影响文件：`internal/domain/billing/quota.go`、`internal/domain/billing/billing_quota.go`、`internal/relay/channel/openai/relay_openai.go`

15. P1-Claude流式合并改写明确缓存分档并丢失显式零值语义
    流式 merge 只在 incoming 分档大于 0 时覆盖当前值，再调用规范化把剩余量填入 5m；明确 `5m=40,1h=30,total=100` 会被改写为 70/30，显式 1h=0 也无法覆盖此前非零值，可能保留更贵分档。持久化 JSON 与上游明确分布不等价，违反“明确返回分档时保留，包括显式 0”的要求。
    影响文件：`internal/relay/channel/claude/relay_claude.go`、`internal/relay/helper/convert.go`

16. P2-日志写入边界不校验billing_details JSON
    `RecordConsumeLog()` 只判断 `BillingDetails` 字符串非空，未在系统边界重新校验 schema、数值和包含关系，任意格式字符串都会随行级版本一起异步写入。调用方当前大多会先序列化，但该边界无法防止其他入口把坏 JSON 变成已迁移数据，违反新日志写入前必须校验的要求。
    影响文件：`internal/store/log/log.go`

17. P2-上下文计价错误传播不一致
    通用、Claude、WSS 收尾路径在上下文价格匹配失败时只记录日志后继续，错误是否最终阻断取决于后续价格计划是否恰好命中；WSS 预扣路径则直接返回错误。相同配置错误在不同入口有不同结果，可能生成缺少档位解释的成功账单，应统一显式失败或明确降级策略。
    影响文件：`internal/domain/billing/usage.go`、`internal/domain/billing/quota.go`、`internal/domain/billing/billing_quota.go`

18. P2-Claude转OpenAI Chat响应暴露内部缓存分档字段
    `shared.Usage` 的 Claude 5m/1h 字段带有公开 JSON tag 且无 `omitempty`，Claude 转 OpenAI Chat 响应直接复用该 Usage 并序列化，客户端会收到非 OpenAI 协议的内部字段。数据不是秘密，但这是客户端协议边界污染，影响响应语义保持要求。
    影响文件：`internal/domain/shared/openai_response.go`、`internal/relay/channel/claude/relay_claude.go`

19. P2-价格表缓存刷新只作用于当前进程
    价格计划缓存是进程内变量，TTL 为一分钟；保存成功后只调用本地失效函数，没有共享缓存失效或跨节点广播。多实例部署时其他节点最长可在 TTL 内继续用旧价格结算，单实例通过不能覆盖多实例“保存后缓存刷新”的一致性要求。
    影响文件：`internal/store/pricing/model_price_table.go`、`internal/store/pricing/pricing_refresh.go`

20. P2-key-query列表违反统一列表页规范
    key-query 仍是可筛选、分页的日志列表，却直接使用 `Card`、手拼 `Table` 和独立加载态，没有复用 `DataTablePage` 与 `SectionPageLayout`。这违反项目列表页规范和阶段 5 对日志列表的实现要求。
    影响文件：`web/src/features/key-query/key-query-logs-table.tsx`

21. P2-价格快照兼容数量与单位展示不完整
    快照缺少已保存数量时，input/output fallback 继承文本量代替普通输入/输出总量，`cache_write_5m` 也不能表达未分档但按 5m 收费的剩余数量；详情单价只显示数值和币种，不显示按次或每百万 Token 单位。正常新快照优先使用已保存数量，但旧快照缺失数量时的历史解释仍不完整。
    影响文件：`web/src/features/usage-logs/lib/billing-details.ts`、`web/src/features/usage-logs/components/dialogs/details-dialog.tsx`

22. P2-模型测试费用与Token明细使用两套算法
    模型测试 quota 继续直接用旧 prompt/completion、倍率和提前取整或旧 ModelPrice 计算，日志明细又调用归一化构建；新价格表、缓存或多模态场景下，测试日志的费用解释与正式请求不一致。明细归一化失败只返回空串，测试日志仍可作为成功日志写入，入口口径不一致。
    影响文件：`internal/httpapi/controller/channel/channel_test_handler.go`、`internal/domain/billing/billing_usage.go`

23. P2-消费日志写入失败后汇总仍继续增加
    消费日志异步 `Create` 失败只输出系统日志，不阻断同一 goroutine 后续的 `quota_data` 更新，也没有持久重试或补偿状态。数据库故障时可能出现资金已结算、消费日志缺失而汇总统计继续增加的对账断裂，影响阶段 4/5 的账务一致性。
    影响文件：`internal/store/log/log.go`、`internal/store/usedata/usedata.go`

24. P2-缺少最终删列结构的可执行备份恢复runbook
    仓库有升级顺序原则和通用备份提醒，但未见覆盖本次实际日志库备份、停止/排空旧写入、最终删列结构检查、恢复到旧二进制匹配备份、恢复校验和失败分支的完整可执行 runbook。验收要求删除列后的回滚必须包含数据库恢复步骤，该交付物缺失。
    影响文件：`docs/PRD/计费重构/验收标准.md`、`docs/PRD/计费重构/PRD.md`、`docs/zh/installation/config-maintenance/system-update.mdx`

25. P3-管理员模型筛选与统计LIKE语义不一致
    管理员列表把原始模型名直接用于参数化 LIKE，统计查询则先转换并转义通配符；模型名包含 `_` 或 `%` 时列表与统计可能匹配不同集合。查询仍是参数化的，不是 SQL 注入，问题仅是通配符语义一致性。
    影响文件：`internal/store/log/log.go`

26. P3-QuotaStat.Tpm命名承载区间Token总量
    预聚合结构把任意筛选区间的 `sum(token_used)` 命名为 `Tpm`，容易让调用方误以为是“每分钟 Token”。实时路径另有最近 60 秒查询，不能据此声称线上 TPM 时间窗已错，但该维护性命名仍会误导后续复用。
    影响文件：`internal/store/usedata/usedata.go`

27. P3-缺少schema_version与未知版本错误分类混淆
    前端把 schema_version 路径上的任何校验错误都归类为 unknown_version；后端在字段缺失导致 schema version 解析为 0 时也返回 unknown version。两边都会拒绝非法 JSON，因此不是安全边界失败，但错误诊断不能区分缺字段、非法类型和真正未知版本。
    影响文件：`web/src/features/usage-logs/lib/billing-details.ts`、`internal/domain/billing/billing_details_json.go`

## 审查过程与二次验证

本次审查按无重叠范围并行执行五个只读子代理审查，均使用 `glm-5.3-flash`、reasoning `max`：数据库与迁移、usage 归一化与 relay、阶段 3/4 价格表与结算、阶段 5 后端统计导出、前端展示与价格表 UI。每个子代理首次完成后均将相同提示词原文重新发送给同一代理执行第二轮，并要求输出两轮一致性对照。

两轮核心结论没有反转。数据库、前端、阶段 5 和价格表/结算代理两轮核心缺陷一致；归一化/relay 代理第二轮新增 WSS 收尾错误忽略与逐事件汇总差异，第一轮遗漏而非结论反转。数据库代理第二轮新增 JSON 总量约束与迁移 fixture 问题；前端代理第二轮新增 input/output 展示只取文本拆分的问题，并将快照 fallback 缺口细化。所有新增结论均由主线在当前 checkout 二次定位验证。

随后使用 `gpt-6-astra`、reasoning `medium` 的只读子代理统一复核全部结论。该复核确认了主线缺陷事实，并新增四项有效发现：旧完成标记跳过最终校验、仅新价格表模型无法进入正式请求、模型测试费用与明细算法不一致、日志写入失败后汇总继续增加。其建议的“生产事故分级”会把本报告 P0-1～P0-6 降为 P1；本报告按验收阻断口径保留 P0，事实证据本身与复核结论一致。复核建议将 Stripe 金额/币种校验移出本次验收范围，本报告未计入；该问题可另案审查。

## 验证边界

主线程在本当前 checkout 已执行并通过以下检查：

```bash
GOCACHE=/tmp/nookmux-go-cache GOMAXPROCS=2 go test -p 1 ./internal/domain/billing/... ./internal/store/... ./internal/httpapi/controller/... ./internal/relay/...
GOCACHE=/tmp/nookmux-go-cache GOMAXPROCS=2 go test -p 1 ./...
cd web && bun run typecheck
cd web && bun run lint
cd web && NODE_OPTIONS=--max-old-space-size=4096 bun run build
cd web && bun run i18n:sync
```

这些结果不能替代以下未验证项：`TEST_BILLING_DETAILS_DB_DSN` 未设置，真实 MySQL 与 PostgreSQL 迁移用例被跳过；未验证 SQLite/MySQL 5.7.8+/PostgreSQL 9.6+ 的空库、历史库升级、重复启动、删列前后中断和生产备份恢复；未发起真实 Claude、Bedrock、OpenRouter、OpenAI Chat/Responses、Gemini、audio、Realtime/WSS 上游请求；未启动生产 master/slave 或多实例环境；未验证真实并发扣款、改价、回调、历史库规模和代表性统计查询性能。浏览器交互未逐项实测。上述范围均不能根据本地测试通过推定为验收通过。