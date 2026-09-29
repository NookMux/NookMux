<h1 align="center">NookMux</h1>

<p align="center">
  <a href="https://github.com/NookMux/NookMux/stargazers">
    <img src="https://img.shields.io/github/stars/NookMux/NookMux?style=for-the-badge&logo=github&logoColor=white&labelColor=24292e&color=ffc107" alt="GitHub Stars" />
  </a>
  <a href="https://zread.ai/NookMux/NookMux">
    <img src="https://img.shields.io/badge/Zread-Ask_AI-00b0aa?style=for-the-badge&logo=data%3Aimage%2Fsvg%2Bxml%3Bbase64%2CPHN2ZyB3aWR0aD0iMTYiIGhlaWdodD0iMTYiIHZpZXdCb3g9IjAgMCAxNiAxNiIgZmlsbD0ibm9uZSIgeG1sbnM9Imh0dHA6Ly93d3cudzMub3JnLzIwMDAvc3ZnIj4KPHBhdGggZD0iTTQuOTYxNTYgMS42MDAxSDIuMjQxNTZDMS44ODgxIDEuNjAwMSAxLjYwMTU2IDEuODg2NjQgMS42MDE1NiAyLjI0MDFWNC45NjAxQzEuNjAxNTYgNS4zMTM1NiAxLjg4ODEgNS42MDAxIDIuMjQxNTYgNS42MDAxSDQuOTYxNTZDNS4zMTUwMiA1LjYwMDEgNS42MDE1NiA1LjMxMzU2IDUuNjAxNTYgNC45NjAxVjIuMjQwMUM1LjYwMTU2IDEuODg2NjQgNS4zMTUwMiAxLjYwMDEgNC45NjE1NiAxLjYwMDFaIiBmaWxsPSIjZmZmIi8%2BCjxwYXRoIGQ9Ik00Ljk2MTU2IDEwLjM5OTlIMi4yNDE1NkMxLjg4ODEgMTAuMzk5OSAxLjYwMTU2IDEwLjY4NjQgMS42MDE1NiAxMS4wMzk5VjEzLjc1OTlDMS42MDE1NiAxNC4xMTM0IDEuODg4MSAxNC4zOTk5IDIuMjQxNTYgMTQuMzk5OUg0Ljk2MTU2QzUuMzE1MDIgMTQuMzk5OSA1LjYwMTU2IDE0LjExMzQgNS42MDE1NiAxMy43NTk5VjExLjAzOTlDNS42MDE1NiAxMC42ODY0IDUuMzE1MDIgMTAuMzk5OSA0Ljk2MTU2IDEwLjM5OTlaIiBmaWxsPSIjZmZmIi8%2BCjxwYXRoIGQ9Ik0xMy43NTg0IDEuNjAwMUgxMS4wMzg0QzEwLjY4NSAxLjYwMDEgMTAuMzk4NCAxLjg4NjY0IDEwLjM5ODQgMi4yNDAxVjQuOTYwMUMxMC4zOTg0IDUuMzEzNTYgMTAuNjg1IDUuNjAwMSAxMS4wMzg0IDUuNjAwMUgxMy43NTg0QzE0LjExMTkgNS42MDAxIDE0LjM5ODQgNS4zMTM1NiAxNC4zOTg0IDQuOTYwMVYyLjI0MDFDMTQuMzk4NCAxLjg4NjY0IDE0LjExMTkgMS42MDAxIDEzLjc1ODQgMS42MDAxWiIgZmlsbD0iI2ZmZiIvPgo8cGF0aCBkPSJNNCAxMkwxMiA0TDQgMTJaIiBmaWxsPSIjZmZmIi8%2BCjxwYXRoIGQ9Ik00IDEyTDEyIDQiIHN0cm9rZT0iI2ZmZiIgc3Ryb2tlLXdpZHRoPSIxLjUiIHN0cm9rZS1saW5lY2FwPSJyb3VuZCIvPgo8L3N2Zz4K&logoColor=white" alt="Zread AI" />
  </a>
  <a href="https://deepwiki.com/NookMux/NookMux">
    <img src="https://img.shields.io/badge/DeepWiki-Docs-6366f1?style=for-the-badge&logo=gitbook&logoColor=white" alt="DeepWiki" />
  </a>
</p>

[English](README.en.md) · 简体中文

基于 [newapi](https://github.com/QuantumNous/new-api) 的自用定制版 AI API 网关/代理项目。内置支持各大厂商CodingPlan套餐渠道，方便便捷对接使用、额度查询等服务

相较于原版NewAPI-v0.10.9-alpha.3，我做了下列优化

1. 优化UI
2. 适配了中国一部分AI厂商的套餐对接、额度查询等功能
3. 优化了渠道对接，例如基础URL、多协议原生适配、移除部分冗余协议
4. 优化了协议转换，使得原本不支持Responses的渠道，也能自主的将Responses转换为上游支持的Chat接口
...

或许还有更多功能的优化，只是由于记忆原因没法较为详细的在此地方表述。

我期望这个项目的最终形态：一个不太重但是又足够个人使用、能作为简单商业化运营的项目

后续计划：可以见issue区域，那里是我当前正在处理的任务，未来会将全部计划逐步在issue建立，也欢迎各位在issue交流讨论

本项目主要用于个人学习、研究与自用场景。使用、部署或二次分发本项目时，应遵守 AGPL-3.0 许可证、本项目及上游项目的版权声明，并自行确保不违反相关上游服务提供商的服务条款；不得将本项目用于中转、分发、倒卖厂商 Plan 或其他违反第三方服务条款的行为。

---

> ⚠️ 本项目仅供个人学习与自用，不保证稳定性、可用性与长期维护，也不提供任何形式的技术支持。

<details>
<summary>AI Coding 说明</summary>

本项目在开发过程中使用了 AI 辅助编程，主要用于代码阅读、功能改造、问题排查、重构建议与文档整理。

本项目基于 [newapi](https://github.com/QuantumNous/new-api) 进行定制与调整。AI 参与了部分代码生成与修改流程，但所有改动均以个人需求为目标，不代表上游项目立场，也不保证与上游版本保持实时同步。

由于本项目包含 AI 辅助生成与人工调整内容，可能存在实现不完善、边界情况处理不足或潜在兼容性问题。若因部署、修改、使用本项目产生任何第三方争议、服务风险、账号风险或合规问题，均由使用者自行承担，与本项目维护者及上游项目无关。

如果你使用AI Coding对本项目进行维护开发，建议您配置好两个MCP，`serena`和`codegraph`，其中`serena`需要先拉起HTTP服务，详情可以参考我的云端开发Docker容器环境：[OpenCode-Docker](https://github.com/zhongruan0522/opencode-docker)

### 使用的 AI Coding 工具

- Codex[CLI & APP]
- Cursor[IDE]
- CodeBuddy[VS Code插件]
- ClaudeCode[Claude桌面端]

### 当前使用的 AI 模型

> 关于模型，实际格式为`供应商/模型[思维强度/是否开启思考]`

- Zhipu/GLM-5.3[Max]
- Zhipu/GLM-5.3-Flash[Max]
- CodeBuddy/DeepSeek-V4.1-Flash[Max]
- Anthropic/Claude-Opus-5-5[Max]

> 我们将在近期对ClaudeMax渠道的Opus5.5做评估，评估范围为产品视觉介绍
> 1. 由于ClaudeCode的配置导致很多本身是GLM响应的请求被写成了Claude系列模型，在正式声明Claude全系进入当前使用模型之前一切Claude模型标识的修改基本都为GLM负责
>    对应规则如下：Sonnet及其以下为GLM-5.3-Flash，Opus及其以上为GLM-5.3，会随着动态更新至智谱最新的模型

### 历史使用的 AI Coding 工具

- OpenCode[Web UI]
- Claude Code
- ZCode[闲时任务]

### 历史使用模型

- OpenAI/GPT-5.2[Xhigh]
- OpenAI/GPT-5.4[Xhigh]
- Zhipu/GLM-5-Turbo[Thinking]
- Zhipu/GLM-5V-Turbo[Thinking]
- Zhipu/GLM-5[Thinking]
- Zhipu/GLM-5.1[Thinking]
- Zhipu/GLM-5.2[Max]
- OpenAI/GPT-5.5[high]
- OpenAI/GPT-5.5[Xhigh]
- OpenAI/GPT-5.6-系列[Sol/Luna]-[Max]

> 在本项目开发完善期间，有部分模型仅使用本项目进行能力测试，并非主力开发，清单如下：`Minimax/Minimax-M3`、`Kimi/Kimi-K2.6`、`Kimi/Kimi-K3`、`CodeBuddy/Kimi-K3`

</details>

## 致谢

感谢以下开源项目对本项目的启发与帮助：

- **[QuantumNous/new-api](https://github.com/QuantumNous/new-api)** — 本项目基座。
- **[CuzTeam/new-api](https://github.com/CuzTeam/new-api)** — 首页UI。
- **[looplj/AxonHub](https://github.com/looplj/axonhub)** — 协议转换。
- **[farion1231/cc-switch](https://github.com/farion1231/cc-switch)** — 中国模型厂商的套餐查询相关接口、协议转换。
