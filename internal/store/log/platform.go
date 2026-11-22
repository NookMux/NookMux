package logstore

import (
	"net/url"
	"strings"
)

// 客户端平台识别结果的受控 ID 集合（kebab-case）。
// 新增平台时：在此补充常量与识别规则，并在前端 platform-meta.ts 补充显示名与图标。
const (
	PlatformClaudeCode      = "claude-code"
	PlatformCodex           = "codex"
	PlatformOpenCode        = "opencode"
	PlatformHermesAgent     = "hermes-agent"
	PlatformDeepSeekHarness = "deepseek-harness"
	PlatformOpenClaw        = "openclaw"
	PlatformPi              = "pi"
	PlatformNewAPI          = "new-api"
	PlatformCherryStudio    = "cherry-studio"
	PlatformRikkaHub        = "rikkahub"
)

// AllPlatformIDs 返回全部受支持的平台 ID，供测试与维护检查使用。
func AllPlatformIDs() []string {
	return []string{
		PlatformClaudeCode,
		PlatformCodex,
		PlatformOpenCode,
		PlatformHermesAgent,
		PlatformDeepSeekHarness,
		PlatformOpenClaw,
		PlatformPi,
		PlatformNewAPI,
		PlatformCherryStudio,
		PlatformRikkaHub,
	}
}

// uaProductPrefixes 定义 UA product token 的锚定前缀（匹配前整体转小写，键不含
// 尾部分隔符）。命中后要求前缀之后紧跟 '/'、'-'、' '、'(' 或串尾，构成 token
// 边界，防止 pinecone/1.0 之类误命中 pi。
var uaProductPrefixes = map[string]string{
	"claude-cli":            PlatformClaudeCode,
	"claude-code":           PlatformClaudeCode,
	"codex_cli_rs":          PlatformCodex,
	"codex-cli-rs":          PlatformCodex,
	"codex_cli":             PlatformCodex,
	"codex-cli":             PlatformCodex,
	"codex_tui":             PlatformCodex,
	"codex-tui":             PlatformCodex,
	"codex_vscode":          PlatformCodex,
	"codex-vscode":          PlatformCodex,
	"codex_exec":            PlatformCodex,
	"codex-exec":            PlatformCodex,
	"codex_atlas":           PlatformCodex,
	"codex-atlas":           PlatformCodex,
	"codex_chatgpt_desktop": PlatformCodex,
	"codex-chatgpt-desktop": PlatformCodex,
	"codex desktop":         PlatformCodex,
	"opencode":              PlatformOpenCode,
	"hermesagent":           PlatformHermesAgent,
	"deepseek-harness":      PlatformDeepSeekHarness,
	"openclaw":              PlatformOpenClaw,
	"pi":                    PlatformPi,
	// new-api 网关自报 UA 为弱信号：其转发模型流量时默认不带 UA，
	// 仅显式配置或非模型出站请求可能携带。
	"new-api": PlatformNewAPI,
	// RikkaHub 默认 UA 为 RikkaHub-Android/{version}（OkHttp 拦截器对全部
	// 出站请求注入，用户可自定义覆盖），rikkahub 前缀锚定后同样命中
	// RikkaHub/{version} 等变体。
	"rikkahub": PlatformRikkaHub,
}

// uaEmbeddedProducts 定义 Chromium/Electron 系应用 UA 中位于串中的自报 product。
// 该类 UA 形如（应用自身 product 位于串中，uaProductPrefixes 的串首锚定无法
// 命中）：
// `Mozilla/5.0 (<os>) AppleWebKit/537.36 (KHTML, like Gecko) <product>/<version>`
// `Chrome/<version> Electron/<version> Safari/537.36`。
// 匹配时按空白切分 token，对首个 '/' 之前的 product 做精确比较；版本号随
// 发版变化，不参与判定。
var uaEmbeddedProducts = map[string]string{
	// Claude Desktop（Electron）桌面端进程发起的请求，归入 Claude Code。
	// product 须精确为 claude：Claude-User / Claude-Web / Claude-Agent 等
	// Anthropic 网页抓取 UA 的 product 不同，不受影响。
	"claude": PlatformClaudeCode,
}

// refererHostPlatforms 定义 HTTP-Referer 的 host 精确匹配表（host 已小写、去端口）。
var refererHostPlatforms = map[string]string{
	"cherry-ai.com":                 PlatformCherryStudio,
	"hermes-agent.nousresearch.com": PlatformHermesAgent,
	"openclaw.ai":                   PlatformOpenClaw,
	"pi.dev":                        PlatformPi,
	// rikka-ai.com 为 RikkaHub 官方站点，客户端在 OpenRouter 场景固定
	// 上报该 Referer，用户亦可在提供商自定义头中显式配置。
	"rikka-ai.com": PlatformRikkaHub,
}

// DetectPlatform 基于客户端请求头（User-Agent、X-Title、HTTP-Referer）识别客户端
// 平台，返回受控平台 ID；无法识别时返回空串。输入为 extractClientHeaders 产出的
// 已清洗值（空值统一为 "-"）。
//
// 优先级：UA（串首 product 前缀 > 串中自报 product token）> Referer host 精确
// 匹配。X-Title 可由客户端任意填写，仅在其他信号缺席时作弱佐证，不参与判定。
// UA 描述实际客户端进程，Referer 可能是用户在 GUI 中填写的任意值，冲突时以
// UA 为准。
func DetectPlatform(ua, xTitle, httpReferer string) string {
	if id := detectPlatformByUa(normalizePlatformSignal(ua)); id != "" {
		return id
	}
	return detectPlatformByReferer(normalizePlatformSignal(httpReferer))
}

// normalizePlatformSignal 把日志列中的占位值归一为空串，并统一小写便于匹配。
func normalizePlatformSignal(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	if value == "" || value == "-" {
		return ""
	}
	return value
}

func detectPlatformByUa(ua string) string {
	if ua == "" {
		return ""
	}
	for prefix, platform := range uaProductPrefixes {
		if !strings.HasPrefix(ua, prefix) {
			continue
		}
		rest := ua[len(prefix):]
		if rest == "" || strings.ContainsAny(rest[:1], "/- (") {
			return platform
		}
	}
	return detectPlatformByEmbeddedProduct(ua)
}

// detectPlatformByEmbeddedProduct 在 UA 的各空白分隔 token 中查找
// `product/version` 形态的自报 product，命中 uaEmbeddedProducts 即返回对应平台。
func detectPlatformByEmbeddedProduct(ua string) string {
	for _, token := range strings.Fields(ua) {
		product, _, found := strings.Cut(token, "/")
		if !found {
			continue
		}
		if platform, ok := uaEmbeddedProducts[product]; ok {
			return platform
		}
	}
	return ""
}

func detectPlatformByReferer(referer string) string {
	if referer == "" {
		return ""
	}
	parsed, err := url.Parse(referer)
	if err != nil || parsed.Host == "" {
		return ""
	}
	return refererHostPlatforms[strings.ToLower(parsed.Hostname())]
}
