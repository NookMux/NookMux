# NookMux

[English](README.en.md) · 简体中文

基于 [newapi](https://github.com/QuantumNous/new-api) 的自用定制版 AI API 网关/代理项目。

本项目主要用于个人学习、研究与自用场景。使用、部署或二次分发本项目时，应遵守 AGPL-3.0 许可证、本项目及上游项目的版权声明，并自行确保不违反相关上游服务提供商的服务条款；不得将本项目用于中转、分发、倒卖厂商 Plan 或其他违反第三方服务条款的行为。

[Zread](https://zread.ai/NookMux/NookMux) · [DeepWiki](https://deepwiki.com/NookMux/NookMux)

---

> ⚠️ 本项目仅供个人学习与自用，不保证稳定性、可用性与长期维护，也不提供任何形式的技术支持。

<details>
<summary>AI Coding 说明</summary>

本项目在开发过程中使用了 AI 辅助编程，主要用于代码阅读、功能改造、问题排查、重构建议与文档整理。

本项目并非从零开发，而是基于历史旧版本的 [newapi](https://github.com/QuantumNous/new-api) 进行自用方向的定制与调整。AI 参与了部分代码生成与修改流程，但所有改动均以个人使用需求为目标，不代表上游项目立场，也不保证与上游版本保持同步。

由于本项目包含 AI 辅助生成与人工调整内容，可能存在实现不完善、边界情况处理不足或潜在兼容性问题。若因部署、修改、使用本项目产生任何第三方争议、服务风险、账号风险或合规问题，均由使用者自行承担，与本项目维护者及上游项目无关。

### 使用的 AI Coding 工具

- OpenCode[Web UI]
- Codex[CLI & APP]
- Cursor[IDE]
- CodeBuddy[插件]
- ZCode[闲时任务]

### 当前使用的 AI 模型

> 关于模型，实际格式为`供应商/模型[思维强度/是否开启思考]`

- Zhipu/GLM-5.3[Max]
- Zhipu/GLM-5V-Turbo[Thinking]
- OpenAI/GPT-5.5[high]
- OpenAI/GPT-5.5[Xhigh]
- OpenAI/GPT-5.6-系列[Sol/Luna]-[Max]

### 历史使用的 AI Coding 工具

- Claude Code

### 历史使用模型

- OpenAI/GPT-5.2[Xhigh]
- OpenAI/GPT-5.4[Xhigh]
- Zhipu/GLM-5-Turbo[Thinking]
- Zhipu/GLM-5[Thinking]
- Zhipu/GLM-5.1[Thinking]
- Zhipu/GLM-5.2[Max]

> 在本项目开发完善期间，有部分模型仅使用本项目进行能力测试，并非主力开发，清单如下：`Minimax/Minimax-M3`、`Kimi/Kimi-K2.6`、`Kimi/Kimi-K3`、`CodeBuddy/DeepSeek-V4-Flash`、`OpenRouter/Ox Alpha`

</details>

## 致谢

感谢以下开源项目对本项目的启发与帮助：

- **[QuantumNous/new-api](https://github.com/QuantumNous/new-api)** — 本项目的上游基础项目。
- **[CuzTeam/new-api](https://github.com/CuzTeam/new-api)** — 首页UI的参考来源。
- **[ccbkkb/MicroWARP](https://github.com/ccbkkb/MicroWARP)** — 极简高性能的 Cloudflare WARP SOCKS5 Docker 代理，为 AI API 网关提供稳定的网络出口方案。
- **[looplj/AxonHub](https://github.com/looplj/axonhub)** — 优秀的 AI API 网关参考实现。
- **[openafw/Openafw](https://github.com/openafw/openafw)** — 本地 AI 流量安全过滤方案参考。
- **[zhongruan0522/zaicontrol](https://github.com/zhongruan0522/zaicontrol)（私有仓库）** — Z.AI 套餐查询相关接口。
- **[farion1231/cc-switch](https://github.com/farion1231/cc-switch)** — Kimi 套餐查询相关接口。
