# internal/domain/

领域层：承载被 controller / middleware / relay / store 多包共享的领域模型、错误、
常量与领域服务（阶段 5.3 起，原 `internal/service/` 的业务逻辑按资源迁入）。
目标方向是各业务包依赖 domain，而不是互相依赖。

## 子包

| 子包 | 内容 |
|---|---|
| `billing/` | ★ 计费核心：`service.go`（PreConsumeBilling / SettleBilling / BillingSession 会话）、`quota.go`（预扣/后扣/用量重试）、`usage.go`（`CalculateUsage` 通用文本路径 usage 计算 + `ApplyQuota` 落账，阶段 6 自 relay/handler 下沉）、`billing_usage.go`（三规范 Usage 归一化为 `BillingUsage`，计费 PRD 阶段 1；负数/溢出/缓存矛盾显式失败，矛盾明细保留诊断告警；官方显式 0 用 presence 保留，Gemini `TEXT` 明细先扣除缓存读取）、`billing_details_json.go`（`billing_details` canonical JSON 的序列化入口 `SerializeBillingUsage` 与 contract 类型/解析的门面 re-export；结构与解析在 `contract/`）、`billing_quota.go`（阶段 4 计价核心：四入口共享归一化用量、价格表/旧配置固定优先级、汇率、分组倍率、上下文档位、价格来源快照与聚合兜底构造）、`billing_shadow.go`（迁移期旧公式影子对拍，差异分类告警，迁移完成后整文件删除）、`pricing.go`（上下文阶梯计价，阶段 2 起档位 tokens = 普通输入+输出+缓存读写）、[price_table.go](billing/price_table.go)（组件价格表的归一化、验证与解析）、`violation_fee.go`、`usage_helper.go`、`gemini_usage.go`、`log_info.go`、`funding_source.go`。 |
| `billing/contract/` | 计费契约叶子包：`PriceData`、`GroupRatioInfo`、`ContextPricing*`、[price_table.go](billing/contract/price_table.go)（组件价格表模型、旧 ratio 只读投影）、`billing_details.go` / `billing_details_totals.go`（`billing_details` canonical JSON 的 schema、严格解析与唯一汇总公式——`logs` 旧 token 聚合列删除后的持久化 Token 用量唯一权威来源）。供 config/ratio、store/pricing、store/log、store/usedata、relay 引用（见"依赖方向约束"）。 |
| `billing/plan_quota/` | 供应商套餐配额（GLM / Kimi / MiniMax）拉取与归一化。 |
| `channel/` | 渠道域服务：自动禁用/启用、加权随机选择与重试、渠道亲和性缓存；含 `channel_error.go`（`ChannelError`）。 |
| `channel/constant/` | 渠道域常量（`APIType*`、`ChannelType*`、`EndpointType`、`MultiKeyMode`、Azure 时间点、套餐 BaseURL 表）。 |
| `audit/` | 审计日志服务（`RecordAudit` 唯一入口），见 [audit/AGENTS.md](audit/AGENTS.md)。 |
| `rankings/` | 模型/供应商用量排行榜聚合。 |
| `ticket/` | 工单领域服务。 |
| `sensitive/` | 敏感词匹配（`AcSearch` / `SundaySearch`），调用方 relay 与渠道自动禁用。 |
| `group/` | 用户分组与分组倍率解析。 |
| `shared/` | **过渡性收容包**：原 `dto/`+`types/` 中暂无明确归属的协议 DTO 与共享类型；另含自原 `internal/constant` 并入的运行时限值变量（`env.go`）、缓存键格式（`cache_key.go`）与 `Setup`。 |

## `shared/` 的纪律：只出不进

- `shared/` 是 `dto/`+`types/` 的过渡收容包，**不是长期归宿**。
- **新代码禁止向 `shared/` 添加文件**；新协议 DTO 优先放进对应领域子包（渠道协议族
  待 relay PRD 收口时归 `internal/relay/`，计费相关归 `billing/`）。
- 每次领域拆分时把可归位的文件迁出 `shared/`，持续收缩直至解散。

## 依赖方向约束

- `domain` 下的包**不得 import** controller / middleware / router；不得被 store 反向依赖。
- `domain/channel/constant/` 与 `domain/billing/contract/` 必须保持**契约叶子包**
  （仅依赖标准库或 `pkg/`）：config/ratio、store/pricing、relay 等引用方依赖它们取类型，
  一旦引入业务 import 就会与领域服务成环。这是契约从 `billing/` 根包下沉到
  `billing/contract/` 的原因（阶段 5.3，`billing` 服务依赖 store/config）。
- 领域服务包（`billing/`、`channel/` 等）可以依赖 store、config、infra，但领域包之间
  和对 infra 的依赖必须保持无环。当前既定方向（按 `go list` 实测）：
  `billing → {channel, channel/constant, shared, contract, i18n, infra/log, infra/notify, infra/runtime, infra/tokenizer, httpapi根, config/model|ratio|system|operation, relay/common, relay/constant, store/*}`、
  `channel → {sensitive, group, infra/notify}`。新增依赖前先确认不成环。
- `domain/shared/` 依赖 `internal/common` 与 `internal/infra/log`（原 dto/types 的既有
  单向依赖）。原 `internal/constant` 四个跨领域文件已在阶段 4 解散并入：运行时限值
  （`env.go`）、缓存键（`cache_key.go`）、`Setup` 落 `shared/`，`ContextKey` 注册表因被
  domain/store/infra/relay 全层引用且 `shared → common → shared` 会成环，落
  `internal/common/context_key.go`（详见根 AGENTS 与 `internal/common/AGENTS.md`）。
  `common` 不再 import `shared`，该方向必须保持。

## 检查清单

- 领域服务承载业务规则；契约子包只做契约（结构体、常量、错误类型、纯函数），不写 I/O、不碰数据库。
- 控制器输入已校验也不能假设内部状态可信；跨系统边界继续校验。
- 计费、quota、usage、渠道选择、动态倍率、违规费用等逻辑必须保持可追踪和可测试。
- 组件价格表的输入在 [billing/price_table.go](billing/price_table.go) 统一校验价格单位、精度、汇率、取整、作用域与父子组件冲突；`reasoning_output` 只能复用输出价格，不能成为独立收费组件。生产结算必须走 `CalculateNormalizedQuotaForRelay` 单点，显式价格表优先于旧 ratio 投影；计划解析与子组件匹配共用同一 `modelPricePlanPrecedes` 固定优先级（分组 > 端点 > service tier > 上下文档位 > 生效时间），不随持久化顺序漂移；未命中的组件只能走固定 legacy 回退或显式错误（配置缺失与价格表加载失败归类 `billing_config_missing`），禁止静默按 0。显式计划用自身 `rounding_mode` 做最终取整；legacy 投影保留各入口既有取整口径。价格组件、来源、倍率、文档位和 service tier 写入 `Other.billing_price_snapshot`，`billing_details` 仍只存 token 用量。WSS 按事件实扣后，成功收尾只把请求级初始 `BillingSession` 按 0 结算释放，不重复收取汇总 quota。
- 外部 HTTP 调用复用现有客户端和超时配置（`infra/httpclient`）；不要无超时请求或吞掉上游错误。
- 不要模拟成功或生成假 usage 来掩盖上游/业务失败。
- 迁移文件进/出时同步更新本文件子包表与根 `AGENTS.md` 的结构概览。

## 验证

- `go build ./... && go test ./internal/domain/...`。
- 改计费、quota、倍率、渠道选择逻辑后执行对应 billing/channel 测试与
  `go test ./internal/relay/... ./internal/httpapi/controller/...`。
