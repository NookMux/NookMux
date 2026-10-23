# docs/AGENTS.md

`docs/` 是项目的 Mintlify 文档站与项目内部文档目录。

## Mintlify 文档站

- `docs.json` 是站点唯一配置：导航、主题、多语言结构全部在此定义，目录内不再维护 `meta.json`。
- `docs/en/` 与 `docs/cn/` 是英文与简体中文内容，一一对应；页面路径在导航中须带语言前缀（如 `en/guide/home`、`cn/index`）。
- 新增/改名页面必须同步更新 `docs.json`，否则页面不会出现在侧栏。
- 内部链接使用根相对路径且不带语言前缀（如 `/installation/index`），由 Mintlify i18n 按语言解析。
- API 端点页使用 frontmatter `api: "METHOD /path"` 声明端点，页面正文只保留描述。

## 规范结构与渐进式披露

- 详细开发规范集中在 [docs/开发规范/](开发规范/)（如 [list-page-table-spec.md](开发规范/list-page-table-spec.md) 列表页规范）。
- 智能体配置与维护准则参见 [docs/AGENTS.md完全使用指南.md](AGENTS.md完全使用指南.md)。
- 避免在根 `AGENTS.md` 中堆砌长篇规范正文，通过 Markdown 链接按需引用本目录下的规范。

## 规则

- 文档示例严禁包含真实 secrets、API token、数据库 DSN 或 OAuth client secret。
- 中文文档保持中文语境；英文/日文文档按所在目录语言维护。
- 源码是唯一真实来源（Single Source of Truth）：文档中的配置项、环境变量、命令行参数必须与当前代码和实际配置严格一致。
- 外部参考项目（如 `参考项目/`）仅用于参考对比，切勿直接将外部结论写为本项目规范。
- 在用户未要求查看`docs/cn`和`docs/en`、`docs/开发规范`这三个文件夹以外的情况下，你只需要维护这两个文件夹的文档，无需在意其他文档

## 验证

- 修改 Markdown 相对链接后，核实目标文件及锚点真实存在。
- 修改构建、部署或脚本说明后，对照 `Dockerfile`、`.github/workflows/ci.yml` 和 `scripts/` 确认准确性。