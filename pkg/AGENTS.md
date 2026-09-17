# pkg/AGENTS.md

`pkg/` 存放可独立复用、零业务依赖的通用基础库。

## 进入门槛与依赖纯净度

进入 `pkg/` 的唯一铁律是**绝对无业务依赖**：
- 包内任何 Go 源文件（含 `_test.go` 测试代码）严禁 import `internal/` 下的任何业务代码（包括 `internal/common`、`internal/domain/`、`internal/infra/` 等）。
- 依赖核查必须执行符号级审查，不仅看 import 声明：即使源码只引用标准库，只要使用了业务定义的全局变量、特定错误值或配置结构体，亦严禁置于 `pkg/`。
- 基础库一旦因演进而引入业务依赖，必须立即降级迁移至 `internal/infra/` 或 `internal/common/`。

## 现有包

- `jsonx/`：全仓统一的 JSON 序列化/反序列化包装库（`Marshal` / `Unmarshal` / `UnmarshalJsonStr` / `DecodeJson` / `Valid` / `GetJsonType`）以及零拷贝转换 `StringToByteSlice`。项目内所有业务代码的 JSON 编解码必须统一调用 `jsonx`，禁止直接调用 `encoding/json`。
- `cachex/`：通用底层纯缓存原语，供上层领域服务（如渠道亲和性缓存）复用。

## 规则

- 保持库的通用性与原子性，严禁为迎合单一业务场景在 `pkg/` 中引入特化分支。
- 禁止为了规避依赖检查而在 `pkg/` 中硬塞反向依赖或空接口作掩护。

## 验证

- `go build ./... && go test ./pkg/...`
- 依赖纯净度核查命令（输出中绝不能包含 `internal/`）：
  `! go list -deps ./pkg/... | grep 'github.com/NookMux/NookMux/internal'`
