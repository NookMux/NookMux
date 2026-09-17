# internal/common/AGENTS.md

`internal/common/` 是业务全局变量与共享纯工具的基础内核层，其改动直接影响后端所有模块。基础设施能力位于 `internal/infra/`，HTTP 边界工具位于 `internal/httpapi/` 根包，本包保持纯净，仅收容全局业务状态、跨层上下文键（`ContextKey`）注册表与跨包通用纯工具。

## 内容划分

| 类别 | 核心职责 | 文件示例 |
|---|---|---|
| 业务全局变量 | 开关、限流阈值、SMTP/OAuth、角色/状态常量、配额基数 | `constants.go`、`quota.go`、`topup_ratio.go`、`performance_config.go`、`timezone.go` |
| 跨层上下文键 | 统一 `ContextKey` 类型与全部键名常量（供 domain/store/infra/relay/httpapi 跨层共享） | `context_key.go` |
| 环境变量读取 | 基础类型读取与默认值回落（供 app/infra/store 多层复用，防成环） | `env.go` |
| 系统日志输出 | 进程生命周期标准输出（写 Gin writer / stdout，与 `infra/log` 业务日志隔离防环） | `sys_log.go` |
| 模型与端点 | 基础端点类型映射与常量 | `model.go`、`api_type.go`、`endpoint_type.go`、`endpoint_defaults.go`、`audio.go` |
| 共享纯工具 | 字符串、哈希、分页、深拷贝、内存轻量限流纯函数 | `str.go`、`utils.go`、`hash.go`、`copy.go`、`rate_limit.go`、`page_info.go` |

## 依赖与设计规则

- **严格单向依赖**：本包**只允许依赖** `pkg/`、`internal/domain/channel/constant` 与标准库。
  严禁 import `internal/infra/`、`internal/httpapi/`、`internal/domain/shared/` 及任何业务/存储包（`shared → common` 与 `infra/* → common` 均为既定单向依赖，反向即成循环依赖）。
- **JSON 规范**：全包所有 JSON 序列化/反序列化操作必须统一调用 `pkg/jsonx` 包装函数，严禁在业务代码中直接调用 `encoding/json` 的 `Marshal`/`Unmarshal`/`NewDecoder`。
- **系统日志隔离**：`sys_log.go`（`SysLog`/`SysError`/`FatalLog`）负责底层与启动级日志，直接对接系统输出；`infra/log` 的业务日志（`LogInfo` 等）依赖本包状态及协程池。两者职责分明，不可合并以防形成循环依赖。
- **上下文键注册**：`ContextKey` 被全层引用，统一在此登记。新增键前先确认归属：渠道特定上下文置于 `domain/channel`，中继协议特定键置于 `relay/constant`。
- **工具纯度**：新增通用工具必须保持纯函数特性，严禁引入 controller/domain/store 的反向依赖。

## 验证

- `go test ./internal/common/...`
- 影响全局行为时执行：`go test ./...`
