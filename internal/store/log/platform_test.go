package logstore

import "testing"

// TestDetectPlatform 覆盖首批平台的三头识别规则、优先级与误报防护。
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

		// 优先级：UA 描述实际客户端进程，冲突时优先于 Referer。
		{"ua wins over referer", "claude-cli/2.1.0 (external, cli)", "-", "https://cherry-ai.com", PlatformClaudeCode},
		{"x-title is not authoritative", "-", "Cherry Studio", "-", ""},
		{"x-title is not authoritative for openclaw", "-", "OpenClaw", "-", ""},

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
