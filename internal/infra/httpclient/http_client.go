package httpclient

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"sync"
	"time"

	"github.com/NookMux/NookMux/internal/common"
	"github.com/NookMux/NookMux/internal/config/system"

	"github.com/NookMux/NookMux/internal/infra/security"
	"github.com/gorilla/websocket"
	"golang.org/x/net/proxy"
)

var (
	httpClient      *http.Client
	proxyClientLock sync.Mutex
	proxyClients    = make(map[string]*http.Client)
)

// ssrfRecheckKey 是请求级 SSRF 复查标记的 context key。
// 携带该标记的请求会在 transport 拨号时刻对实际连接 IP 复查私网/IP 规则，
// 用于消除 ValidateURL（校验时解析）与实际连接（连接时解析）之间的
// DNS rebinding 窗口。未携带标记的请求（如管理员配置的渠道上游）不受影响。
type ssrfRecheckKey struct{}

// WithSSRFRecheck 返回携带 SSRF 连接时复查配置的 ctx。
// 后续对该 ctx 内发起的每一次 TCP 连接（含 redirect 后续跳转，redirect
// 请求继承初始请求的 context）都会在拨号前复查目标 IP。
func WithSSRFRecheck(ctx context.Context, protection *security.SSRFProtection) context.Context {
	if protection == nil {
		return ctx
	}
	return context.WithValue(ctx, ssrfRecheckKey{}, protection)
}

// recheckProtectionFromContext 取出拨号时复查配置；无标记返回 nil。
func recheckProtectionFromContext(ctx context.Context) *security.SSRFProtection {
	if ctx == nil {
		return nil
	}
	protection, _ := ctx.Value(ssrfRecheckKey{}).(*security.SSRFProtection)
	return protection
}

// dialContextWithSSRFRecheck 包装基础 DialContext，在连接时刻执行 SSRF 复查。
//
// Go transport 在调用 DialContext 前已完成目标域名的 DNS 解析，addr 参数
// 即最终要连接的 IP——在该点复查可保证"校验的 IP = 连接的 IP"，彻底消除
// ValidateURL（校验时解析）与实际连接（连接时再次解析）之间的 DNS
// rebinding 窗口。每次 redirect 跳转与 Happy Eyeballs 的每次拨号尝试
// 都会经过本函数，整条重定向链均在复查范围内。
//
// proxyFunc 非 nil 且该请求命中代理时跳过复查：此时 transport 拨的是代理
// 地址，目标域名由代理解析，本地复查既无意义、还会把常见的
// HTTP_PROXY=http://127.0.0.1:xxxx 本机代理误判为私网违规。
// 未携带复查标记的请求行为与基础 DialContext 完全一致。
func dialContextWithSSRFRecheck(base func(ctx context.Context, network, addr string) (net.Conn, error), proxyFunc func(*http.Request) (*url.URL, error)) func(ctx context.Context, network, addr string) (net.Conn, error) {
	return func(ctx context.Context, network, addr string) (net.Conn, error) {
		protection := recheckProtectionFromContext(ctx)
		if protection == nil {
			return base(ctx, network, addr)
		}
		if proxyFunc != nil {
			// 拨号阶段拿不到 *http.Request，无法向 proxyFunc 提供请求上下文。
			// HTTP(S)_PROXY 按主机匹配（NO_PROXY 豁免），此处无法逐请求判定；
			// 只要配置了代理函数就跳过复查，宁可漏检交给代理侧策略，
			// 也不误杀经代理出站的正常通知/下载流量。
			return base(ctx, network, addr)
		}
		host, _, err := net.SplitHostPort(addr)
		if err != nil {
			return nil, fmt.Errorf("ssrf recheck: invalid dial address %q: %v", addr, err)
		}
		ip := net.ParseIP(host)
		if ip == nil {
			// transport 已完成解析，addr 必然是 IP；防御异常路径。
			return nil, fmt.Errorf("ssrf recheck: dial address %q is not an IP", addr)
		}
		if err := protection.CheckConnectedIP(ip); err != nil {
			return nil, err
		}
		return base(ctx, network, addr)
	}
}

// buildFetchSSRFProtection 按 FetchSetting 构建 SSRF 防护配置。
func buildFetchSSRFProtection() (*security.SSRFProtection, error) {
	fetchSetting := system.GetFetchSetting()
	if !fetchSetting.EnableSSRFProtection {
		return nil, nil
	}
	return security.BuildSSRFProtection(
		fetchSetting.AllowPrivateIp,
		fetchSetting.DomainFilterMode,
		fetchSetting.IpFilterMode,
		fetchSetting.DomainList,
		fetchSetting.IpList,
		fetchSetting.AllowedPorts,
		fetchSetting.ApplyIPFilterForDomain,
	)
}

// NewSSRFValidatedRequest 构建经过 SSRF 校验并携带连接时复查标记的 HTTP 请求。
//
// 面向用户可控 URL（webhook / bark / gotify / 文件下载等）。除初始 URL 的
// 静态校验（ValidateURL）外，请求 context 携带复查配置，transport 拨号时刻
// 对实际连接 IP 再次复查，消除校验与连接两次 DNS 解析之间的 DNS rebinding
// 窗口；redirect 由 client 的 checkRedirect 复查后仍继承本标记，整条跳转链
// 均在防护范围内。
//
// SSRF 防护关闭时不附加标记，行为与 http.NewRequest 一致。
func NewSSRFValidatedRequest(method, url string, body io.Reader) (*http.Request, error) {
	req, err := http.NewRequest(method, url, body)
	if err != nil {
		return nil, err
	}
	if err := validateURLWithCurrentFetchSetting(url); err != nil {
		return nil, err
	}
	if protection, proErr := buildFetchSSRFProtection(); proErr == nil && protection != nil {
		req = req.WithContext(WithSSRFRecheck(req.Context(), protection))
	}
	return req, nil
}

// validateURLWithCurrentFetchSetting 用当前 FetchSetting 静态校验 URL。
func validateURLWithCurrentFetchSetting(urlStr string) error {
	fetchSetting := system.GetFetchSetting()
	return security.ValidateURLWithFetchSetting(urlStr, fetchSetting.EnableSSRFProtection, fetchSetting.AllowPrivateIp, fetchSetting.DomainFilterMode, fetchSetting.IpFilterMode, fetchSetting.DomainList, fetchSetting.IpList, fetchSetting.AllowedPorts, fetchSetting.ApplyIPFilterForDomain)
}

func checkRedirect(req *http.Request, via []*http.Request) error {
	urlStr := req.URL.String()
	if err := validateURLWithCurrentFetchSetting(urlStr); err != nil {
		return fmt.Errorf("redirect to %s blocked: %v", urlStr, err)
	}
	if len(via) >= 10 {
		return fmt.Errorf("stopped after 10 redirects")
	}
	return nil
}

func newRelayTransport(proxyFunc func(*http.Request) (*url.URL, error)) *http.Transport {
	baseDialer := &net.Dialer{
		Timeout:   30 * time.Second,
		KeepAlive: 30 * time.Second,
	}
	transport := &http.Transport{
		MaxIdleConns:        common.RelayMaxIdleConns,
		MaxIdleConnsPerHost: common.RelayMaxIdleConnsPerHost,
		IdleConnTimeout:     time.Duration(common.RelayIdleConnTimeout) * time.Second,
		ForceAttemptHTTP2:   true,
		Proxy:               proxyFunc,
		// 拨号时刻复查携带 SSRF 标记的请求实际连接的 IP，消除校验与连接
		// 两次 DNS 解析之间的 rebinding 窗口；未标记的请求零影响。
		// 命中代理的拨号跳过复查（见函数注释）。
		DialContext: dialContextWithSSRFRecheck(baseDialer.DialContext, proxyFunc),
	}
	if common.TLSInsecureSkipVerify {
		transport.TLSClientConfig = common.InsecureTLSConfig
	}
	return transport
}

func newRelayHTTPClient(transport *http.Transport) *http.Client {
	client := &http.Client{
		Transport:     transport,
		CheckRedirect: checkRedirect,
	}
	if common.RelayTimeout > 0 {
		client.Timeout = time.Duration(common.RelayTimeout) * time.Second
	}
	return client
}

func InitHttpClient() {
	httpClient = newRelayHTTPClient(newRelayTransport(http.ProxyFromEnvironment))
}

func GetHttpClient() *http.Client {
	return httpClient
}

// NewGatewayHttpClient 返回访问管理员显式配置的外部网关（如支付网关）的客户端：
// 超时固定为 timeout，不受 RELAY_TIMEOUT 等 relay 配置影响；目标为可信上游，
// 重定向不套用面向用户可控 URL 的 FetchSetting 过滤。底层复用 Go 默认
// Transport（保守连接池，遵循 HTTP_PROXY/HTTPS_PROXY 环境变量）。
func NewGatewayHttpClient(timeout time.Duration) *http.Client {
	return &http.Client{Timeout: timeout}
}

// GetHttpClientWithProxy returns the default client or a proxy-enabled one when proxyURL is provided.
func GetHttpClientWithProxy(proxyURL string) (*http.Client, error) {
	if proxyURL == "" {
		if client := GetHttpClient(); client != nil {
			return client, nil
		}
		return http.DefaultClient, nil
	}
	return NewProxyHttpClient(proxyURL)
}

// NewProxyWebSocketDialer 返回走指定代理的 WebSocket 拨号器；proxyURL 为空时返回默认拨号器。
// gorilla/websocket 的 Proxy 函数支持 http/https/socks5/socks5h（经 golang.org/x/net/proxy）。
func NewProxyWebSocketDialer(proxyURL string) (*websocket.Dialer, error) {
	if proxyURL == "" {
		return websocket.DefaultDialer, nil
	}

	parsedURL, err := url.Parse(proxyURL)
	if err != nil {
		return nil, err
	}

	switch parsedURL.Scheme {
	case "http", "https":
		return &websocket.Dialer{
			Proxy: http.ProxyURL(parsedURL),
		}, nil
	case "socks5", "socks5h":
		var auth *proxy.Auth
		if parsedURL.User != nil {
			auth = &proxy.Auth{
				User:     parsedURL.User.Username(),
				Password: "",
			}
			if password, ok := parsedURL.User.Password(); ok {
				auth.Password = password
			}
		}
		dialer, err := proxy.SOCKS5("tcp", parsedURL.Host, auth, proxy.Direct)
		if err != nil {
			return nil, err
		}
		return &websocket.Dialer{
			NetDial: dialer.Dial,
		}, nil
	default:
		return nil, fmt.Errorf("unsupported proxy scheme: %s, must be http, https, socks5 or socks5h", parsedURL.Scheme)
	}
}

// ResetProxyClientCache 清空代理客户端缓存，确保下次使用时重新初始化
func ResetProxyClientCache() {
	proxyClientLock.Lock()
	defer proxyClientLock.Unlock()
	for _, client := range proxyClients {
		if transport, ok := client.Transport.(*http.Transport); ok && transport != nil {
			transport.CloseIdleConnections()
		}
	}
	proxyClients = make(map[string]*http.Client)
}

// NewProxyHttpClient 创建支持代理的 HTTP 客户端
func NewProxyHttpClient(proxyURL string) (*http.Client, error) {
	if proxyURL == "" {
		if client := GetHttpClient(); client != nil {
			return client, nil
		}
		return http.DefaultClient, nil
	}

	proxyClientLock.Lock()
	if client, ok := proxyClients[proxyURL]; ok {
		proxyClientLock.Unlock()
		return client, nil
	}
	proxyClientLock.Unlock()

	parsedURL, err := url.Parse(proxyURL)
	if err != nil {
		return nil, err
	}

	switch parsedURL.Scheme {
	case "http", "https":
		client := newRelayHTTPClient(newRelayTransport(http.ProxyURL(parsedURL)))
		proxyClientLock.Lock()
		proxyClients[proxyURL] = client
		proxyClientLock.Unlock()
		return client, nil

	case "socks5", "socks5h":
		// 获取认证信息
		var auth *proxy.Auth
		if parsedURL.User != nil {
			auth = &proxy.Auth{
				User:     parsedURL.User.Username(),
				Password: "",
			}
			if password, ok := parsedURL.User.Password(); ok {
				auth.Password = password
			}
		}

		// 创建 SOCKS5 代理拨号器
		// proxy.SOCKS5 使用 tcp 参数，所有 TCP 连接包括 DNS 查询都将通过代理进行。行为与 socks5h 相同
		// 目标域名由代理解析，本地不附加 SSRF 复查标记（见 dialContextWithSSRFRecheck）。
		dialer, err := proxy.SOCKS5("tcp", parsedURL.Host, auth, proxy.Direct)
		if err != nil {
			return nil, err
		}

		transport := newRelayTransport(nil)
		transport.DialContext = func(ctx context.Context, network, addr string) (net.Conn, error) {
			return dialer.Dial(network, addr)
		}
		transport.Proxy = nil
		client := newRelayHTTPClient(transport)
		proxyClientLock.Lock()
		proxyClients[proxyURL] = client
		proxyClientLock.Unlock()
		return client, nil

	default:
		return nil, fmt.Errorf("unsupported proxy scheme: %s, must be http, https, socks5 or socks5h", parsedURL.Scheme)
	}
}
