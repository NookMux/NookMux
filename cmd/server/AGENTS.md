# cmd/server/AGENTS.md

`cmd/server/` 是主服务进程的唯一可执行入口，负责启动 API 网关与管理后台。

## 规则

- `main.go` 仅允许作为极薄的进程入口包装层：只处理进程退出码并调用 `internal/app.Run()`。
- 命令行参数解析、资源初始化、Gin 装配、后台 worker 和路由挂载严禁放入本目录，统一由 [internal/app/AGENTS.md](../../internal/app/AGENTS.md) 承载。
- 新增可执行命令时，先确认是否确有独立二进制进程边界；不要为内部库或一次性脚本创建空命令。

## 验证

- 修改入口后执行：`go test ./cmd/server/... ./internal/app/...` 和 `go build ./cmd/server`。
- 修改启动装配时额外执行：`go run ./cmd/server` 冒烟验证，确认服务可正常启动并接收系统信号退出。
