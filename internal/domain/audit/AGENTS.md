# internal/domain/audit/AGENTS.md

`internal/domain/audit/` 承载管理员操作审计日志服务，负责对管理面系统资源的变更进行统一审计落库。

## 审计日志服务

`audit.RecordAudit` 是审计日志记录的唯一全局入口，由各类业务 controller 在管理员写操作成功后调用。

### 签名

```go
func RecordAudit(c *gin.Context, module, actionType, description string, before, after interface{}, forceRecord ...bool)
```

- `module`：`auditstore.AuditModule*` 常量（如 `auditstore.AuditModuleChannel`，定义于 `internal/store/audit`）。
- `actionType`：`auditstore.AuditActionCreate` / `auditstore.AuditActionUpdate` / `auditstore.AuditActionDelete`。
- `before`/`after`：变更前后的数据实体（支持 struct、map 或 nil）。服务层会自动：
  - 归一化为 map 并计算字段级 diff，仅持久化发生变更的字段。
  - 递归深度脱敏敏感字段（包含 `key`、`password`、`token`、`secret`、`credential`、`authorization`、`private_key` 的值自动替换为 `[REDACTED]`）。
  - 自动解析内嵌的 JSON 字符串（如渠道的 `other`、`header_override`）并递归脱敏。
- `forceRecord`：传 `true` 时绕过审计总开关和模块开关检查。专用于审计配置本身的变更（`audit_setting.*`），确保"关闭审计"操作自身必然被如实记录。

### 核心行为契约

- 审计总开关关闭（`audit_setting.enabled=false`）或对应模块未启用时，`RecordAudit` 立即静默返回。
- `record_ip=false` 时不记录 IP；`record_diff=false` 时不序列化 before/after 数据。
- 数据库写入通过有界协程池 `runtime.RelayGo` 异步执行，写库失败仅记录 `common.SysError`，严禁阻塞或回滚主业务流程。
- 无会话认证接口（如初始引导 `PostSetup`）需在调用前通过 `c.Set("username", ...)` 显式注入操作人。

## 验证

- `go build ./... && go vet ./internal/domain/audit/...`
- 联动测试：`go test ./internal/domain/... ./internal/httpapi/controller/...`
