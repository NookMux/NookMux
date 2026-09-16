# internal/domain/AGENTS.md

`internal/domain/` 是核心领域层，承载系统核心领域模型、领域服务、契约常量与领域错误。本层被 controller、middleware、relay 共享调用，其设计目标是让业务包单向依赖 domain，消除不同业务包之间的交叉网状依赖。

## 子包职责

| 子包 | 核心职责与关键符号 |
|---|---|
| `billing/` | ★ **计费核心**：`service.go`（PreConsumeBilling / SettleBilling / BillingSession 会话生命周期）、`quota.go`（预扣/补扣/落账重试）、`usage.go`（`CalculateUsage` 通用文本用量计算 + `ApplyQuota` 落账）、`pricing.go`（上下文阶梯计价）、`violation_fee.go`、`funding_source.go`，详见 [billing/AGENTS.md](billing/AGENTS.md)。 |

| `billing/contract/` | **计费契约叶子包**：`PriceData`、`GroupRatioInfo`、`ContextPricing*`。供 config/ratio、store/pricing、relay 引用（见"依赖方向约束"）。 |
| `billing/plan_quota/` | 供应商套餐配额（GLM / Kimi / MiniMax）拉取与归一化解析。 |
| `channel/` | **渠道治理服务**：渠道自动禁用/启用、加权负载均衡、错误重试策略、渠道亲和性缓存；含 `channel_error.go`（`ChannelError`）。 |
| `channel/constant/` | **渠道常量叶子包**：`APIType*`、`ChannelType*`、`EndpointType`、`MultiKeyMode`、Azure 时间点、套餐 BaseURL 映射等纯契约常量。 |
| `audit/` | **审计日志服务**：`audit.RecordAudit` 唯一入口，详见 [audit/AGENTS.md](audit/AGENTS.md)。 |
| `rankings/` | 模型与供应商用量排行榜聚合。 |
| `ticket/` | 工单领域服务。 |
| `sensitive/` | 敏感词高效匹配（`AcSearch` / `SundaySearch`），供 relay 与渠道治理调用。 |
| `group/` | 用户分组与分组倍率动态解析。 |
| `shared/` | **过渡收容包（纪律：只出不进）**：跨模块暂存类型、运行时限值变量（`env.go`）与缓存键生成（`cache_key.go`）。新代码严禁向此包添加文件。 |

## `shared/` 的纪律：只出不进

- `shared/` 是历史遗留类型的过渡收容包，**不是长期归宿**。
- **新代码严禁向 `shared/` 添加文件**；新协议 DTO 必须放进对应领域子包（渠道与中继协议归 `internal/relay/`，计费相关归 `billing/`）。
- 每次涉及重构时优先把可归位的文件迁出 `shared/`，逐步收缩直至解散。

## 依赖方向约束

- **分层单向性**：`domain` 下的包**严禁 import** controller / middleware / router；数据持久层 store 严禁反向依赖 domain。
- **契约叶子包纯度**：`domain/channel/constant/` 与 `domain/billing/contract/` 必须保持**纯契约叶子包**（仅依赖标准库或 `pkg/`）。它们被 config/ratio、store/pricing、relay 广泛引用，一旦引入业务 import 即会与领域服务成环。
- **无环依赖网**：领域服务包（`billing/`、`channel/` 等）可以依赖 store、config、infra，但领域包之间及对 infra 的依赖必须严格无环。
  - 当前既定依赖方向：`billing → {channel, channel/constant, shared, contract, i18n, infra/log, infra/notify, infra/runtime, infra/tokenizer, httpapi根, config/*, relay/common, relay/constant, store/*}`；`channel → {sensitive, group, infra/notify}`。新增依赖前必须确认不成环。
- `domain/shared/` 仅单向依赖 `internal/common` 与 `internal/infra/log`；`internal/common` 严禁反向 import `shared`。

## 检查清单

- 领域服务承载核心业务规则；契约子包只定义结构体、常量、错误类型与纯函数，不产生 I/O、不直连数据库。
- 控制器输入已完成边界校验，但领域服务仍需保持防御性，跨系统边界继续保持状态可信校验。
- 计费核算、quota 预扣/结算、用量统计、渠道优选与动态倍率必须保持可追踪、可单测，严禁使用伪造数据掩盖失败。
- 外部 HTTP 请求必须复用 `infra/httpclient` 的安全代理客户端与超时配置，严禁裸调或吞掉上游错误。

## 验证

- `go build ./... && go test ./internal/domain/...`
- 修改计费、quota、倍率或渠道选择逻辑后，执行联动测试：
  `go test ./internal/relay/... ./internal/httpapi/controller/...`
