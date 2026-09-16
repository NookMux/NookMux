# internal/domain/billing/AGENTS.md

`internal/domain/billing/` 是核心计费与额度核算领域层，承载模型定价、用量统计、预扣费（PreConsume）、后结算（Settle）、配额退款（Refund）与渠道套餐配额解析。

## 核心组件与职责

- `service.go`：`BillingSession` 会话生命周期管理门面。
  - `PreConsumeBilling(c, preConsumedQuota, relayInfo)`：解析用户计费偏好，创建会话并预扣额度，挂载到 `relayInfo.Billing`。
  - `SettleBilling(ctx, relayInfo, actualQuota)`：基于实际消费额度进行差额结算；多退少补，自动记录补扣或返还日志。
  - `RefundQuota(ctx, relayInfo)`：中继失败或异常终止时，全额回退预扣配额。
- `usage.go`：用量折算与落账核心管道。
  - `CalculateUsage(ctx, relayInfo, rawUsage, extraContent...)`：将 Token/多模态/字符计数折算为标准化配额，统一结算 Prompt Cache 折扣、用户分组倍率、补全倍率与上下文阶梯价格。
  - `ApplyQuota(ctx, relayInfo, settlement)`：执行实际账户配额扣除，并准备消费明细日志元数据。
- `pricing.go`：上下文阶梯计价（Context Tiered Pricing）计算模型。
- `contract/`：**计费契约叶子包**（`PriceData`、`GroupRatioInfo`、`ContextPricing*` 等数据结构）。
  - **纯度铁律**：必须保持纯契约，仅允许依赖 Go 标准库或 `pkg/`，零业务依赖。它被 config、store、relay 等广泛引用，严禁反向引用 `billing` 根包或其他业务包，防止循环依赖。
- `plan_quota/`：外部大模型供应商套餐配额查询与归一化（GLM / Kimi / MiniMax）。
- `violation_fee.go`：违规拦截费用核算。
- `funding_source.go`：资金来源解析与多账户扣费抽象。

## 规则

- **计费边界绝对收敛**：Relay 层及其他业务包**严禁直调 store 包**读写 Token / User 账户余额，所有配额变动必须统一委托本包处理。
- **精确核算与禁止静默**：计费倍率、Prompt Cache 比例、补全倍率必须严格按照系统配置计算，严禁随意估算或静默吞掉落账错误。
- **预扣与结算配对**：每次发起 AI 请求前必须完成 `PreConsumeBilling`，中继结束后无论成功或失败，必须显式调用 `SettleBilling` 或 `RefundQuota`，严禁悬挂未结算会话导致用户额度长期冻结。
- **防御性并发**：配额预扣、返还与补扣操作必须保持线程安全与原子性，防止高并发场景下的并发透支或负余额漏洞。

## 验证

- `go test -v ./internal/domain/billing/...`
- 联动验证中继与控制器：
  `go test ./internal/relay/... ./internal/httpapi/controller/...`
