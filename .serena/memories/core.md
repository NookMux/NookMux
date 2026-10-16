# NookMux — 项目核心

Go 实现的 AI API 网关与后台,聚合 OpenAI / Claude / Gemini / Azure / AWS Bedrock 等 40+ 上游,
对外提供统一 API、用户/渠道/计费/限速/认证/管理后台。前端为 React 19 + Rsbuild。

## 改代码前必读:AGENTS.md 体系

**所有规则、命令、技术栈、目录结构说明都在 `AGENTS.md` 体系里。** 根 `AGENTS.md` 明确要求:
修改某个包/目录下的代码前,必须先读该目录的 `AGENTS.md`,否则会违反项目约定(如遗漏审计埋点、
数据库兼容性问题、前端 i18n 缺失等)。

- 根 `AGENTS.md` — 全局工作规则、分层规则、子规则索引、项目概览、常用验证命令。入口,必读。
- `web/AGENTS.md` — 前端技术栈(React 19/Rsbuild/TanStack/Base UI/Tailwind 4)、命令、文件组织、i18n。
- `internal/common/AGENTS.md` — 业务全局变量与零碎工具内核（ContextKey 注册表、SysLog、env 读取）。
- `internal/infra/AGENTS.md` — 基础设施层（db/redis/cache/email/security/runtime/log 等）与依赖方向例外。
- `internal/httpapi/router/AGENTS.md` — 路由分层、web 静态资源、SSE 保护。
- `internal/httpapi/controller/AGENTS.md` — HTTP 边界、输入校验、**审计埋点检查清单**、使用日志字段可见性。
- `internal/httpapi/middleware/AGENTS.md` — 认证/限速/分发/请求体恢复/SSE 保护。
- `internal/domain/audit/AGENTS.md` — **`audit.RecordAudit` 签名与行为约定**;`internal/domain/AGENTS.md` — 计费/配额/渠道领域服务规则(阶段5.3起)。
- `internal/store/AGENTS.md` — GORM、**SQLite/MySQL/PostgreSQL 三库兼容**、缓存、幂等迁移。
- `internal/config/AGENTS.md` — 配置注册、默认值校验、**审计配置**。
- `internal/relay/AGENTS.md` — AI 中继、协议转换、流式输出/usage/计费保护、供应商适配。
- `internal/i18n/AGENTS.md` — 后端响应消息多语言(go-i18n,`i18n.Msg*` 常量,locales/en.yaml + zh.yaml)。
- `docs/AGENTS.md` — 文档规则。

技术栈精确版本、目录结构、构建命令均以 `go.mod`、`web/package.json`、`makefile` 和上述 AGENTS.md
为准,memory 不重复。

## memory 索引

- `mem:pitfalls` — 高频踩坑与易错点。开发中实际遇到的、违反后会产生具体 bug 或返工的陷阱,每条
  指向对应 `AGENTS.md` 规则。新踩坑追加到对应分组。

## 参考项目

`参考项目/` 下有 serena / new-api / axonhub / claude-code-hub / CLIProxyAPI / openafw / 旧版本TTS 等
本地参考源码,**已被根 `.gitignore` 和 `.serena/.gitignore` 双重忽略**,不要修改也不要让 Serena 索引。
复制代码前必须适配本项目 API 和配置。
