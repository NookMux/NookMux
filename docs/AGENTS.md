# docs/AGENTS.md

`docs/` 是 NookMux 主仓库的项目内部文档目录。

## 文档站（独立仓库）

面向用户的 Mintlify 文档站已迁至独立仓库 `zhongruan0522/mintlify-docs`（本地与主仓库同级放置）：

- NookMux 英文文档在 `nookmux/`，中文在 `cn/nookmux/`，站点配置与导航收口于该仓库的 `docs.json`
- 修改用户文档请到该仓库操作；本目录不再维护站点内容

## 规则

- 文档示例严禁包含真实 secrets、API token、数据库 DSN 或 OAuth client secret。
- 中文文档保持中文语境；英文文档按所在目录语言维护。
- 源码是唯一真实来源（Single Source of Truth）：文档中的配置项、环境变量、命令行参数必须与当前代码和实际配置严格一致。
- 外部参考项目（如 `参考项目/`）仅用于参考对比，切勿直接将外部结论写为本项目规范。

## 验证

- 修改 Markdown 相对链接后，核实目标文件及锚点真实存在。
- 修改构建、部署或脚本说明后，对照 `Dockerfile`、`.github/workflows/ci.yml` 和 `scripts/` 确认准确性。
