# NookMux

[English](README.en.md) · 简体中文

基于 [newapi](https://github.com/QuantumNous/new-api) 的自用定制版 AI API 网关/代理项目。内置支持各大厂商CodingPlan套餐渠道，方便便捷对接使用、额度查询等服务

相较于原版NewAPI-v0.10.9-alpha.3，我做了下列优化

1. 优化UI
2. 适配了中国一部分AI厂商的套餐对接、额度查询等功能
3. 优化了渠道对接，例如基础URL、多协议原生适配、移除部分冗余协议
4. 优化了协议转换，使得原本不支持Responses的渠道，也能自主的将Responses转换为上游支持的Chat接口
5. 丰富了TTS的使用适配，可以变成商业化平台进行运营
...

或许还有更多功能的优化，只是由于记忆原因没法较为详细的在此地方表述。

本项目主要用于个人学习、研究与自用场景。使用、部署或二次分发本项目时，应遵守 AGPL-3.0 许可证、本项目及上游项目的版权声明，并自行确保不违反相关上游服务提供商的服务条款；不得将本项目用于中转、分发、倒卖厂商 Plan 或其他违反第三方服务条款的行为。

[Zread](https://zread.ai/NookMux/NookMux) · [DeepWiki](https://deepwiki.com/NookMux/NookMux)

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


### 当前使用的 AI 模型

> 关于模型，实际格式为`供应商/模型[思维强度/是否开启思考]`

- Zhipu/GLM-5.3[Max]
- Zhipu/GLM-5.3-Flash[Max]
- CodeBuddy/DeepSeek-V4.1-Flash[Max]

> 我们将在近期对GPT全新的Sol和Luna做评估

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

> 在本项目开发完善期间，有部分模型仅使用本项目进行能力测试，并非主力开发，清单如下：`Minimax/Minimax-M3`、`Kimi/Kimi-K2.6`、`Kimi/Kimi-K3`、`CodeBuddy/DeepSeek-V4-Flash`

</details>

## 致谢

感谢以下开源项目对本项目的启发与帮助：

- **[QuantumNous/new-api](https://github.com/QuantumNous/new-api)** — 本项目的上游基础项目。
- **[CuzTeam/new-api](https://github.com/CuzTeam/new-api)** — 首页UI的参考来源。
- **[looplj/AxonHub](https://github.com/looplj/axonhub)** — 优秀的 AI API 网关参考实现。
- **[farion1231/cc-switch](https://github.com/farion1231/cc-switch)** — 中国模型厂商的套餐查询相关接口。
