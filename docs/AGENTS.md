# docs/AGENTS.md

`docs/` 是项目设计和用户文档目录。

其中在维护的时候只需要维护用户文档路面，`docs/zh`、`docs/en`这两个文件夹即可

## 规则

- 文档示例不要包含真实 secrets、token、DSN、OAuth client secret。
- 中文文档保持中文语境；英文/日文文档按所在目录语言维护。
- 参考项目路径只能作为线索，文档结论必须与当前仓库代码一致。

## 验证

- 修改链接后确认目标文件存在。
- 修改构建或部署说明后，对照 `Dockerfile`、`.github/workflows/` 和实际 package scripts。