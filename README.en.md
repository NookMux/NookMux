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

English · [简体中文](README.md)

A self-hosted, customized AI API gateway / proxy project based on [newapi](https://github.com/QuantumNous/new-api). It ships with built-in support for major vendors' Coding Plan channels, making it easy to wire them up and query their quotas.

Compared with upstream NewAPI-v0.10.9-alpha.3, the following improvements have been made:

1. Refined UI
2. Adapted plan subscription and quota querying for a number of Chinese AI vendors
3. Improved channel integration — base URL handling, native multi-protocol adaptation, removal of redundant protocols
4. Improved protocol conversion, so channels that do not natively support the Responses API can convert Responses into the Chat API the upstream does support
...

There are likely further improvements not listed here in detail due to the limits of memory.

The shape I want this project to reach: something not too heavy, yet good enough for personal use and simple commercial operation.

Roadmap: see the issue area — that is where the tasks I am currently working on live. The full plan will be filed as issues over time, and discussion there is welcome.

This project is intended for personal learning, research, and self-hosted use. When you use, deploy, or redistribute it, you must comply with the AGPL-3.0 license and the copyright notices of this project and its upstream project, and you must make sure you do not violate the terms of service of the relevant upstream providers. Do not use this project to relay, distribute, or resell vendor plans, or for any other behavior that violates third-party terms of service.

---

> ⚠️ This project is for personal learning and self-use only. No guarantees are made regarding stability, availability, or long-term maintenance, and no technical support is provided.

<details>
<summary>AI Coding Notes</summary>

This project uses AI-assisted programming during development, mainly for code reading, feature modification, issue troubleshooting, refactoring suggestions, and documentation organization.

This project is a customization and adaptation of [newapi](https://github.com/QuantumNous/new-api). AI participated in part of the code generation and modification process, but all changes are aimed at personal needs, do not represent the upstream project's stance, and are not guaranteed to stay in sync with the upstream version in real time.

Since this project contains both AI-assisted generated and manually adjusted content, there may be imperfections, insufficient edge-case handling, or potential compatibility issues. Any third-party disputes, service risks, account risks, or compliance issues arising from deploying, modifying, or using this project are the sole responsibility of the user, and are not related to the maintainers of this project or the upstream project.

If you use AI coding to maintain and develop this project, it is recommended that you configure two MCPs: `serena` and `codegraph`. `serena` requires a running HTTP service first; see my cloud development Docker container environment for details: [OpenCode-Docker](https://github.com/zhongruan0522/opencode-docker).

### AI Coding Tools Used

- Codex[CLI & APP]
- Cursor[IDE]
- CodeBuddy[VS Code extension]
- ClaudeCode[Claude desktop]

### Models Currently in Use

> For models, the actual format is `vendor/model[reasoning effort/whether thinking is enabled]`

- Zhipu/GLM-5.3[Max]
- Zhipu/GLM-5.3-Flash[Max]
- CodeBuddy/DeepSeek-V4.1-Flash[Max]
- Anthropic/Claude-Opus-5-5[Max]

> An evaluation of Opus 5.5 on the Claude Max channel is planned for the near future, scoped to the visual introduction of the product
> 1. Because of how Claude Code is configured, many requests that are actually answered by GLM are labeled as Claude models. Until Claude's full lineup is formally declared as currently in use, every change labeled with a Claude model is basically handled by GLM.
>    The mapping rule is: Sonnet and below → GLM-5.3-Flash; Opus and above → GLM-5.3. This tracks the latest Zhipu models as they are released.

### Previously Used AI Coding Tools

- OpenCode[Web UI]
- Claude Code
- ZCode[off-peak tasks]

### Previously Used Models

- OpenAI/GPT-5.2[Xhigh]
- OpenAI/GPT-5.4[Xhigh]
- Zhipu/GLM-5-Turbo[Thinking]
- Zhipu/GLM-5V-Turbo[Thinking]
- Zhipu/GLM-5[Thinking]
- Zhipu/GLM-5.1[Thinking]
- Zhipu/GLM-5.2[Max]
- OpenAI/GPT-5.5[high]
- OpenAI/GPT-5.5[Xhigh]
- OpenAI/GPT-5.6 series[Sol/Luna]-[Max]

> During the development of this project, some models were used only for capability testing on this project rather than as primary development models: `Minimax/Minimax-M3`, `Kimi/Kimi-K2.6`, `Kimi/Kimi-K3`, `CodeBuddy/Kimi-K3`

</details>

## Acknowledgements

Thanks to the following open-source projects for their inspiration and help:

- **[QuantumNous/new-api](https://github.com/QuantumNous/new-api)** — The base project this project is built on.
- **[CuzTeam/new-api](https://github.com/CuzTeam/new-api)** — Homepage UI.
- **[looplj/AxonHub](https://github.com/looplj/axonhub)** — Protocol conversion.
- **[farion1231/cc-switch](https://github.com/farion1231/cc-switch)** — APIs for querying Chinese vendors' plan subscriptions, and protocol conversion.
