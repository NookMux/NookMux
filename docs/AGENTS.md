# docs/AGENTS.md

`docs/` 是项目的架构设计、开发规范与用户文档目录。

## 规范结构与渐进式披露

- 详细开发规范集中在 [docs/开发规范/](开发规范/)（如 [list-page-table-spec.md](开发规范/list-page-table-spec.md) 列表页规范）。
- 智能体配置与维护准则参见 [docs/AGENTS.md完全使用指南.md](AGENTS.md完全使用指南.md)。
- 避免在根 `AGENTS.md` 中堆砌长篇规范正文，通过 Markdown 链接按需引用本目录下的规范。

## 规则

- 文档示例严禁包含真实 secrets、API token、数据库 DSN 或 OAuth client secret。
- 中文文档保持中文语境；英文/日文文档按所在目录语言维护。
- 源码是唯一真实来源（Single Source of Truth）：文档中的配置项、环境变量、命令行参数必须与当前代码和实际配置严格一致。
- 外部参考项目（如 `参考项目/`）仅用于参考对比，切勿直接将外部结论写为本项目规范。

## 验证

- 修改 Markdown 相对链接后，核实目标文件及锚点真实存在。
- 修改构建、部署或脚本说明后，对照 `Dockerfile`、`.github/workflows/ci.yml` 和 `scripts/` 确认准确性。