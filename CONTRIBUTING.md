# 贡献指南

感谢关注 NookMux（基于 [newapi](https://github.com/QuantumNous/new-api) 的自用定制版 AI API 网关）。欢迎代码、文档、测试、反馈等各类贡献。

## 开发环境设置

### 前置要求

- Go 1.26+（版本以 `go.mod` 为准）
- [Bun](https://bun.sh)（前端包管理器，`web/` 目录有独立 `package.json`）
- Git

### 常用命令

后端：

```bash
# 运行测试
go test ./...

# 运行部分包的测试
go test ./internal/domain/... ./internal/httpapi/...

# 构建
go build -o NookMux ./cmd/server
```

前端（在 `web/` 目录下执行）：

```bash
# 安装依赖
bun install

# 启动开发服务器
bun run dev

# 类型检查 / 代码检查 / 构建
bun run typecheck
bun run lint
bun run build
```

## 代码规范

- 遵循仓库根目录及各子目录 `AGENTS.md` 中的规则（分层、审计、数据库兼容、i18n 等），改动前必读对应目录的 `AGENTS.md`。
- JSON 序列化使用 `pkg/jsonx` 包装函数；数据库操作兼容 SQLite、MySQL、PostgreSQL。
- 前端复用 `src/components/` 下的通用组件；列表页使用 `DataTablePage` + `SectionPageLayout`。
- 用户可见的后端提示信息走 `internal/i18n` 多语言机制，不硬编码中英文字符串。

## 测试

- 后端：为修复的 bug 和新功能补充单元测试，测试文件与源码同目录（`_test.go`）。
- 前端：改动后确保 `bun run typecheck` 与 `bun run lint` 通过。
- 测试应能证明问题存在或行为正确，不写无效断言。

## 提交规范

使用 [Conventional Commits](https://www.conventionalcommits.org/) 规范：

```
<type>[optional scope]: <description>
```

常用类型：`feat`（新功能）、`fix`（修复）、`docs`（文档）、`refactor`（重构）、`perf`（性能）、`test`（测试）、`chore`（构建/工具）。

### AI 生成代码声明（必须）

凡使用了 AI 辅助生成或修改的代码，必须在 **commit 信息结尾**（以及对应 **PR 描述**中）按以下格式注明所使用的工具环境和模型：

```
assisted-by：{agent_name}：{model}
```

- `{agent_name}`：使用的 AI 编码工具/环境，如 `opencode`、`codex`、`cursor`
- `{model}`：实际使用的模型（含供应商，格式可参考 `供应商/模型`），如 `Zhipu/GLM-5.3[Max]`

示例：

```bash
git commit -m "feat(channel): support batch model pulling

assisted-by：opencode：Zhipu/GLM-5.3[Max]"
```

多个模型/工具参与时逐行列出。纯人工改动无需此声明，但需在 PR 描述中说明。

## Pull Request 流程

1. fork 仓库，创建功能分支（`git checkout -b feature/xxx`）
2. 完成开发并通过上述测试与检查
3. 提交 PR，描述中说明改动内容、验证方式；涉及 AI 生成代码时注明所用模型与环境（见上节）
4. 保持 PR 聚焦，避免混合不相关改动；配合审查意见及时更新

提交前检查：

- [ ] 测试通过（后端 `go test ./...`，前端 `bun run typecheck` / `bun run lint`）
- [ ] 遵守对应目录的 `AGENTS.md` 规范
- [ ] 更新了相关文档与 `.env.example`（涉及环境变量时）
- [ ] Commit 信息符合规范并包含 AI 声明（如适用）

## 获取帮助

- **GitHub Issues**：[报告问题或提出功能请求](https://github.com/NookMux/NookMux/issues)，提交前请先搜索现有 issues。

## 许可证

通过贡献代码，您同意您的贡献将在与存储库相同的 [AGPL-3.0 许可证](LICENSE) 下发布。
