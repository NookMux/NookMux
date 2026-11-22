package logstore

import "testing"

// TestDetectPlatform 覆盖受支持平台的三头识别规则、优先级与误报防护。
func TestDetectPlatform(t *testing.T) {
	cases := []struct {
		name        string
		ua          string
		xTitle      string
		httpReferer string
		want        string
	}{
		// Claude Code：UA product 前缀。
		{"claude-cli standard", "claude-cli/2.1.308 (external, claude-desktop-3p, agent-sdk/0.3.308)", "-", "-", PlatformClaudeCode},
		{"claude-cli minimal", "claude-cli/2.1.78 (external, cli)", "-", "-", PlatformClaudeCode},
		{"claude-cli case insensitive", "Claude-CLI/2.1.0 (external, cli)", "-", "-", PlatformClaudeCode},
		{"claude-code mcp ua", "claude-code/1.0.0", "-", "-", PlatformClaudeCode},

		// Claude Desktop：Electron 应用 UA，自报的 Claude/{version} 位于串中；
		// 系统段随运行环境变化、三段版本号随发版变化，均不参与判定。
		{"claude desktop linux", "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Claude/6.28899.2 Chrome/156.0.8078.100 Electron/45.3.3 Safari/537.36", "-", "-", PlatformClaudeCode},
		{"claude desktop windows", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Claude/1.26832.0 Chrome/148.0.7778.280 Electron/42.7.0 Safari/537.36", "-", "-", PlatformClaudeCode},
		{"claude desktop macos", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Claude/1.17377.2 Chrome/148.0.7560.100 Electron/42.7.0 Safari/537.36", "-", "-", PlatformClaudeCode},
		{"claude desktop leading product", "Claude/1.0", "-", "-", PlatformClaudeCode},
		// Anthropic 网页抓取类 UA 的 product 与 claude 不同，不得误命中。
		{"claude-user must not match", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko; compatible; Claude-User/1.0; +https://www.anthropic.com/claude-user) Chrome/131.0.6216.202 Safari/537.36", "-", "-", ""},
		{"claude-web must not match", "Claude-Web/1.0 (external, anthropic)", "-", "-", ""},
		{"claude path in comment must not match", "MyBot/1.0 (+https://example.com/claude/)", "-", "-", ""},
		{"other electron app must not match", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) OtherStudio/1.6.0 Chrome/130.0.6723.58 Electron/33.0.2 Safari/537.36", "-", "-", ""},

		// Codex：originator 家族的 UA 前缀。
		{"codex default originator", "codex_cli_rs/0.114.0 (Windows 10.0.26200; x86_64) xterm-256color", "-", "-", PlatformCodex},
		{"codex tui", "codex-tui/0.1.0 (Mac OS 15.0; arm64)", "-", "-", PlatformCodex},
		{"codex vscode", "codex_vscode/1.0 (Linux 6.1; x86_64)", "-", "-", PlatformCodex},
		{"codex exec", "codex_exec/0.2.0", "-", "-", PlatformCodex},
		{"codex chatgpt desktop", "codex_chatgpt_desktop/1.0.2 (Windows)", "-", "-", PlatformCodex},

		// OpenCode：四段式 UA。
		{"opencode ua", "opencode/stable/1.0.234/opencode", "-", "-", PlatformOpenCode},
		{"opencode variant name", "opencode/dev/2.0.1/opencode-x", "-", "-", PlatformOpenCode},

		// Hermes Agent。
		{"hermes ua", "HermesAgent/0.7.2 (linux; x86_64)", "-", "-", PlatformHermesAgent},
		{"hermes referer", "-", "-", "https://hermes-agent.nousresearch.com", PlatformHermesAgent},

		// DeepSeek Harness：RFC 9110 product+comment。
		{"deepseek harness ua", "deepseek-harness/1.2.0 (+https://github.com/deepseek-ai/deepseek-harness)", "-", "-", PlatformDeepSeekHarness},

		// OpenClaw。
		{"openclaw ua", "openclaw/2026.3.22 (node)", "-", "-", PlatformOpenClaw},
		{"openclaw referer", "-", "-", "https://openclaw.ai", PlatformOpenClaw},

		// pi：精确 token 或 pi/ 前缀，且不得误报近似前缀。
		{"pi bare", "pi", "-", "-", PlatformPi},
		{"pi with version", "pi/1.5.2 (linux; bun/1.2; x64)", "-", "-", PlatformPi},
		{"pi browser", "pi/1.5.2 (browser)", "-", "-", PlatformPi},
		{"pi referer", "-", "-", "https://pi.dev", PlatformPi},
		{"pinecone must not match", "pinecone/1.0", "-", "-", ""},
		{"pilot must not match", "pilot/2.0", "-", "-", ""},

		// New-API：网关自报弱信号。
		{"new-api self report", "new-api", "-", "-", PlatformNewAPI},

		// Cherry Studio：Referer host 精确匹配（带 scheme/path/端口均可）。
		{"cherry referer", "-", "-", "https://cherry-ai.com", PlatformCherryStudio},
		{"cherry referer with path", "-", "-", "https://cherry-ai.com/docs", PlatformCherryStudio},

		// RikkaHub：默认 UA 为 RikkaHub-Android/{version}，Referer 指向官方站点。
		{"rikka default ua", "RikkaHub-Android/1.5.0", "-", "-", PlatformRikkaHub},
		{"rikka bare ua", "rikkahub", "-", "-", PlatformRikkaHub},
		{"rikka ua variant", "RikkaHub/2.0.0", "-", "-", PlatformRikkaHub},
		{"rikka referer", "-", "-", "https://rikka-ai.com", PlatformRikkaHub},
		{"rikka referer with path", "-", "-", "https://rikka-ai.com/docs", PlatformRikkaHub},
		{"rikkahubite must not match", "rikkahubite/1.0", "-", "-", ""},

		// 优先级：UA 描述实际客户端进程，冲突时优先于 Referer。
		{"ua wins over referer", "claude-cli/2.1.0 (external, cli)", "-", "https://cherry-ai.com", PlatformClaudeCode},
		{"rikka ua wins over referer", "RikkaHub-Android/1.5.0", "-", "https://cherry-ai.com", PlatformRikkaHub},
		{"claude desktop ua wins over referer", "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Claude/6.28899.2 Chrome/156.0.8078.100 Electron/45.3.3 Safari/537.36", "-", "https://cherry-ai.com", PlatformClaudeCode},
		{"x-title is not authoritative", "-", "Cherry Studio", "-", ""},
		{"x-title is not authoritative for openclaw", "-", "OpenClaw", "-", ""},
		{"x-title is not authoritative for rikkahub", "-", "RikkaHub", "-", ""},

		// 无信号与占位值。
		{"all dashes", "-", "-", "-", ""},
		{"all empty", "", "", "", ""},
		{"whitespace only", "  ", " ", " ", ""},
		{"unrelated browser ua", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36", "-", "https://example.com", ""},
	}
	for _, tc := range cases {
		if got := DetectPlatform(tc.ua, tc.xTitle, tc.httpReferer); got != tc.want {
			t.Errorf("%s: DetectPlatform(%q, %q, %q) = %q, want %q", tc.name, tc.ua, tc.xTitle, tc.httpReferer, got, tc.want)
		}
	}
}

// TestAllPlatformIDs 保证平台常量与识别规则保持同步：每条 UA 规则都能产出受控 ID。
func TestAllPlatformIDs(t *testing.T) {
	seen := make(map[string]struct{}, len(uaProductPrefixes))
	for _, platform := range uaProductPrefixes {
		seen[platform] = struct{}{}
	}
	for _, platform := range uaEmbeddedProducts {
		seen[platform] = struct{}{}
	}
	for _, platform := range refererHostPlatforms {
		seen[platform] = struct{}{}
	}
	known := make(map[string]struct{})
	for _, id := range AllPlatformIDs() {
		known[id] = struct{}{}
	}
	for platform := range seen {
		if _, ok := known[platform]; !ok {
			t.Errorf("platform %q produced by rules is missing from AllPlatformIDs", platform)
		}
	}
}
