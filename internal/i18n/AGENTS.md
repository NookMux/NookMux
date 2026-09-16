# internal/i18n/AGENTS.md

`internal/i18n/` 负责后端 API 响应消息的多语言国际化（go-i18n）。

## 适用范围

仅处理后端向客户端返回的 JSON 响应 `message` 字段（错误/成功提示）。前端 UI 界面的文案国际化由 [web/AGENTS.md](../../web/AGENTS.md) 负责，二者职责隔离：后端负责生成用户请求的本地化提示信息，前端直接展示后端 message，严禁前端进行二次翻译。

## 规则

- **统一提示入口**：用户可见的 API 错误/成功提示必须走 i18n 体系，使用 `i18n.Msg*` 常量配合 `httpapi.ApiErrorI18n` / `httpapi.ApiSuccessI18n` / `i18n.T` 返回，严禁硬编码中英文字符串。
- **常量集中管理**：消息 key 必须统一定义在 `internal/i18n/keys.go` 中，并按模块（`MsgUser*`、`MsgToken*`、`MsgChannel*` 等）分类维护，禁止在业务调用处直接书写字面量字符串。
- **双语字典对齐**：翻译文件为 `locales/zh.yaml` 与 `locales/en.yaml`，采用扁平 `key: "value"` 结构。key 采用点分命名规范 `模块.场景`（如 `user.username_or_password_error`）。新增消息时两文件必须同步添加相同 key。
- **模板参数插值**：文案支持模板变量 `{{.Var}}`，通过 `i18n.T` 或 `httpapi.ApiErrorI18n` 的 args map 动态传入。
- **自动语言协商**：客户端语言由 `GetLangFromContext` 统一解析请求的 `Accept-Language` header（目前支持 `zh` 与 `en`，默认回落至中文 `DefaultLang`），业务层不得自行重复解析语言。
- **非翻译场景隔离**：内部系统级日志（`SysError` / `SysLog`）、调试输出及发送给上游大模型服务商的请求体严禁接入翻译。
- **底层依赖守则**：i18n 包不承载业务逻辑；JSON 序列化严格遵守项目规范，调用 `pkg/jsonx`。

## 初始化

`i18n.Init()` 在 `internal/app/bootstrap.go` 启动流程中加载嵌入的 YAML 翻译文件，并将翻译函数注入 `httpapi.TranslateMessage`。新增翻译文件需在 `Init` 中同步挂载。

## 验证

- `go build ./...`
- 人工核对 `locales/zh.yaml` 与 `locales/en.yaml` 的键集合是否对齐。
- 影响响应提示时执行关联控制器测试：`go test ./internal/httpapi/controller/...`
