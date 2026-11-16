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

## 日志清理配置

日志维护配置位于 `internal/config/operation/log_cleanup_setting.go`，由 `manager.GlobalConfig.Register("log_cleanup_setting", ...)` 注册：

- `auto_clean_enabled`：自动清理总开关，默认关闭。
- `retention_value` / `retention_unit`：数据最长保存时长（正整数 + `year`/`month`/`week`/`day` 单位，数值不做上限），默认保留近两年。
- `interval_value` / `interval_unit`：自动清理间隔，默认每周一次；执行时刻由 `internal/domain/logcleanup` 对齐到服务器本地时区自然边界 00:00（每周一 / 每月 1 日 / 每年 1 月 1 日）。
- `targets`：清理目标 JSON 数组（`logs`/`quota_data`/`stored_images`/`stored_videos`/`audit_logs`），默认 `["logs","quota_data"]`。

校验入口 `ValidateLogCleanupValue` / `ValidateLogCleanupUnit` / `ValidateLogCleanupTargets`，由 `UpdateOption` 控制器按 key 逐项调用（错误消息走 i18n `option.log_cleanup_*`）；消费方为 `internal/domain/logcleanup` 的后台清理 worker。

## 性能指标配置

模型性能指标配置位于 `internal/config/perfmetrics/config.go`，由 `manager.GlobalConfig.Register("perf_metrics_setting", ...)` 注册：`enabled`（采样开关，默认开启）、`flush_interval`（热桶落库间隔分钟数，最小 1，默认 5）、`bucket_time`（聚合桶粒度 `minute`/`5min`/`hour`，默认 `hour`）、`retention_days`（桶数据保留天数，`0` 永久保留，默认 30）。桶粒度校验入口 `BucketTimeSeconds`，数值范围校验由 `UpdateOption` 控制器按 key 逐项执行（错误消息走 i18n `option.perf_metrics_*`）；消费方为 `internal/domain/perfmetrics` 采样与 `internal/store/perfmetrics` 落库 worker。

## 验证

- `go test ./internal/config/...`
- 影响控制台系统设置时，同步执行前端检查：`cd web && bun run typecheck`。
