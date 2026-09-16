# internal/config/AGENTS.md

`internal/config/` 是系统配置管理层，负责系统全局、运营参数、模型定价、分组倍率、性能调优及控制台展示配置的注册、动态读取与校验。

## 规则

- **统一注册模型**：配置必须通过 `internal/config/manager/` 的 ConfigManager（`manager.GlobalConfig`）集中注册和安全读取，杜绝散落在业务包中的无锁全局变量写入。
- **强制边界校验**：每个配置项必须具备明确默认值、合法范围校验和清晰错误信息。倍率、价格、状态码匹配区间、限流等敏感数值解析失败时必须显式返回错误，严禁静默回落导致资损。
- **前后端链路同步**：修改配置结构体或校验规则时，必须同步检查 controller 边界参数绑定、前端系统设置页面及 store 持久化逻辑。

## 审计配置

审计日志配置位于 `internal/config/operation/audit_setting.go`，由 `manager.GlobalConfig.Register("audit_setting", ...)` 注册：

- `Enabled`：审计总开关，默认关闭。
- `Modules`：各模块开关，JSON 字符串 `map[string]bool`，默认全部启用。
- `RecordIp`：是否记录操作 IP。
- `RecordDiff`：是否记录变更前后数据差异。

新增审计模块时必须在 `defaultAuditModules()` 中注册，联动更新规范见 [internal/httpapi/controller/AGENTS.md](../httpapi/controller/AGENTS.md#新增资源类型时的检查清单)。

## 验证

- `go test ./internal/config/...`
- 影响控制台系统设置时，同步执行前端检查：`cd web && bun run typecheck`。
