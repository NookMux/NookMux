package security

import (
	"fmt"
	"net"
	"net/url"
	"strconv"
	"strings"

	"github.com/NookMux/NookMux/internal/i18n"
)

// SSRFProtection SSRF防护配置
type SSRFProtection struct {
	AllowPrivateIp         bool
	DomainFilterMode       bool     // true: 白名单, false: 黑名单
	DomainList             []string // domain format, e.g. example.com, *.example.com
	IpFilterMode           bool     // true: 白名单, false: 黑名单
	IpList                 []string // CIDR or single IP
	AllowedPorts           []int    // 允许的端口范围
	ApplyIPFilterForDomain bool     // 对域名启用IP过滤
}

// DefaultSSRFProtection 默认SSRF防护配置
var DefaultSSRFProtection = &SSRFProtection{
	AllowPrivateIp:   false,
	DomainFilterMode: true,
	DomainList:       []string{},
	IpFilterMode:     true,
	IpList:           []string{},
	AllowedPorts:     []int{},
}

// privateIPv4Nets IPv4 私有/保留/特殊用途网段
// 参考 IANA IPv4 Special-Purpose Address Registry
var privateIPv4Nets = []net.IPNet{
	{IP: net.IPv4(0, 0, 0, 0), Mask: net.CIDRMask(8, 32)},       // 0.0.0.0/8
	{IP: net.IPv4(10, 0, 0, 0), Mask: net.CIDRMask(8, 32)},      // 10.0.0.0/8
	{IP: net.IPv4(100, 64, 0, 0), Mask: net.CIDRMask(10, 32)},   // 100.64.0.0/10
	{IP: net.IPv4(127, 0, 0, 0), Mask: net.CIDRMask(8, 32)},     // 127.0.0.0/8
	{IP: net.IPv4(169, 254, 0, 0), Mask: net.CIDRMask(16, 32)},  // 169.254.0.0/16
	{IP: net.IPv4(172, 16, 0, 0), Mask: net.CIDRMask(12, 32)},   // 172.16.0.0/12
	{IP: net.IPv4(192, 0, 0, 0), Mask: net.CIDRMask(24, 32)},    // 192.0.0.0/24
	{IP: net.IPv4(192, 0, 2, 0), Mask: net.CIDRMask(24, 32)},    // 192.0.2.0/24
	{IP: net.IPv4(192, 168, 0, 0), Mask: net.CIDRMask(16, 32)},  // 192.168.0.0/16
	{IP: net.IPv4(198, 18, 0, 0), Mask: net.CIDRMask(15, 32)},   // 198.18.0.0/15
	{IP: net.IPv4(198, 51, 100, 0), Mask: net.CIDRMask(24, 32)}, // 198.51.100.0/24
	{IP: net.IPv4(203, 0, 113, 0), Mask: net.CIDRMask(24, 32)},  // 203.0.113.0/24
	{IP: net.IPv4(224, 0, 0, 0), Mask: net.CIDRMask(4, 32)},     // 224.0.0.0/4
	{IP: net.IPv4(240, 0, 0, 0), Mask: net.CIDRMask(4, 32)},     // 240.0.0.0/4
	{IP: net.IPv4(255, 255, 255, 255), Mask: net.CIDRMask(32, 32)},
}

// privateIPv6Nets IPv6 私有/保留/特殊用途网段
var privateIPv6Nets = func() []net.IPNet {
	cidrs := []string{
		"::/128",
		"::1/128",
		"::ffff:0:0/96",
		"64:ff9b::/96",
		"100::/64",
		"2001::/23",
		"2001:db8::/32",
		"fc00::/7",
		"fe80::/10",
		"ff00::/8",
	}
	nets := make([]net.IPNet, 0, len(cidrs))
	for _, cidr := range cidrs {
		if _, network, err := net.ParseCIDR(cidr); err == nil && network != nil {
			nets = append(nets, *network)
		}
	}
	return nets
}()

// isPrivateIP 检查IP是否为私有/保留/特殊用途地址
func isPrivateIP(ip net.IP) bool {
	if ip == nil {
		return true
	}
	if ip.IsUnspecified() {
		return true
	}
	if ip.IsLoopback() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() {
		return true
	}
	if ip.IsInterfaceLocalMulticast() {
		return true
	}

	if v4 := ip.To4(); v4 != nil {
		for _, privateNet := range privateIPv4Nets {
			if privateNet.Contains(v4) {
				return true
			}
		}
		return false
	}

	for _, privateNet := range privateIPv6Nets {
		if privateNet.Contains(ip) {
			return true
		}
	}
	return ip.IsPrivate()
}

// parsePortRanges 解析端口范围配置
// 支持格式: "80", "443", "8000-9000"
func parsePortRanges(portConfigs []string) ([]int, error) {
	var ports []int

	for _, config := range portConfigs {
		config = strings.TrimSpace(config)
		if config == "" {
			continue
		}

		if strings.Contains(config, "-") {
			// 处理端口范围 "8000-9000"
			parts := strings.Split(config, "-")
			if len(parts) != 2 {
				return nil, fmt.Errorf("invalid port range format: %s", config)
			}

			startPort, err := strconv.Atoi(strings.TrimSpace(parts[0]))
			if err != nil {
				return nil, fmt.Errorf("invalid start port in range %s: %v", config, err)
			}

			endPort, err := strconv.Atoi(strings.TrimSpace(parts[1]))
			if err != nil {
				return nil, fmt.Errorf("invalid end port in range %s: %v", config, err)
			}

			if startPort > endPort {
				return nil, fmt.Errorf("invalid port range %s: start port cannot be greater than end port", config)
			}

			if startPort < 1 || startPort > 65535 || endPort < 1 || endPort > 65535 {
				return nil, fmt.Errorf("port range %s contains invalid port numbers (must be 1-65535)", config)
			}

			// 添加范围内的所有端口
			for port := startPort; port <= endPort; port++ {
				ports = append(ports, port)
			}
		} else {
			// 处理单个端口 "80"
			port, err := strconv.Atoi(config)
			if err != nil {
				return nil, fmt.Errorf("invalid port number: %s", config)
			}

			if port < 1 || port > 65535 {
				return nil, fmt.Errorf("invalid port number %d (must be 1-65535)", port)
			}

			ports = append(ports, port)
		}
	}

	return ports, nil
}

// isAllowedPort 检查端口是否被允许
func (p *SSRFProtection) isAllowedPort(port int) bool {
	if len(p.AllowedPorts) == 0 {
		return true // 如果没有配置端口限制，则允许所有端口
	}

	for _, allowedPort := range p.AllowedPorts {
		if port == allowedPort {
			return true
		}
	}
	return false
}

// isDomainWhitelisted 检查域名是否在白名单中
func isDomainListed(domain string, list []string) bool {
	if len(list) == 0 {
		return false
	}

	domain = strings.ToLower(domain)
	for _, item := range list {
		item = strings.ToLower(strings.TrimSpace(item))
		if item == "" {
			continue
		}
		// 精确匹配
		if domain == item {
			return true
		}
		// 通配符匹配 (*.example.com)
		if strings.HasPrefix(item, "*.") {
			suffix := strings.TrimPrefix(item, "*.")
			if strings.HasSuffix(domain, "."+suffix) || domain == suffix {
				return true
			}
		}
	}
	return false
}

func (p *SSRFProtection) isDomainAllowed(domain string) bool {
	listed := isDomainListed(domain, p.DomainList)
	if p.DomainFilterMode { // 白名单
		return listed
	}
	// 黑名单
	return !listed
}

// isIPWhitelisted 检查IP是否在白名单中

func isIPListed(ip net.IP, list []string) bool {
	if len(list) == 0 {
		return false
	}

	return IsIpInCIDRList(ip, list)
}

// IsIPAccessAllowed 检查IP是否允许访问
func (p *SSRFProtection) IsIPAccessAllowed(ip net.IP) bool {
	// 私有IP限制
	if isPrivateIP(ip) && !p.AllowPrivateIp {
		return false
	}

	listed := isIPListed(ip, p.IpList)
	if p.IpFilterMode { // 白名单
		return listed
	}
	// 黑名单
	return !listed
}

// ValidateURL 验证URL是否安全
func (p *SSRFProtection) ValidateURL(urlStr string) error {
	// 解析URL
	u, err := url.Parse(urlStr)
	if err != nil {
		return i18n.NewErrorWithFallback(fmt.Sprintf("invalid URL format: %v", err), i18n.MsgSSRFInvalidURL, map[string]any{"Error": err.Error()}).WithCause(err)
	}

	// 只允许HTTP/HTTPS协议
	if u.Scheme != "http" && u.Scheme != "https" {
		return i18n.NewErrorWithFallback(fmt.Sprintf("unsupported protocol: %s (only http/https allowed)", u.Scheme), i18n.MsgSSRFUnsupportedProtocol, map[string]any{"Scheme": u.Scheme})
	}

	// 解析主机和端口
	host, portStr, err := net.SplitHostPort(u.Host)
	if err != nil {
		// 没有端口，使用默认端口
		host = u.Hostname()
		if u.Scheme == "https" {
			portStr = "443"
		} else {
			portStr = "80"
		}
	}

	// 验证端口
	port, err := strconv.Atoi(portStr)
	if err != nil {
		return i18n.NewErrorWithFallback(fmt.Sprintf("invalid port: %s", portStr), i18n.MsgSSRFInvalidPort, map[string]any{"Port": portStr})
	}

	if !p.isAllowedPort(port) {
		return i18n.NewErrorWithFallback(fmt.Sprintf("port %d is not allowed", port), i18n.MsgSSRFPortNotAllowed, map[string]any{"Port": port})
	}

	// 如果 host 是 IP，则跳过域名检查
	if ip := net.ParseIP(host); ip != nil {
		if !p.IsIPAccessAllowed(ip) {
			if isPrivateIP(ip) {
				return i18n.NewErrorWithFallback(fmt.Sprintf("private IP address not allowed: %s", ip.String()), i18n.MsgSSRFPrivateIPNotAllowed, map[string]any{"IP": ip.String()})
			}
			if p.IpFilterMode {
				return i18n.NewErrorWithFallback(fmt.Sprintf("ip not in whitelist: %s", ip.String()), i18n.MsgSSRFIPNotInWhitelist, map[string]any{"IP": ip.String()})
			}
			return i18n.NewErrorWithFallback(fmt.Sprintf("ip in blacklist: %s", ip.String()), i18n.MsgSSRFIPInBlacklist, map[string]any{"IP": ip.String()})
		}
		return nil
	}

	// 先进行域名过滤
	if !p.isDomainAllowed(host) {
		if p.DomainFilterMode {
			return i18n.NewErrorWithFallback(fmt.Sprintf("domain not in whitelist: %s", host), i18n.MsgSSRFDomainNotInWhitelist, map[string]any{"Host": host})
		}
		return i18n.NewErrorWithFallback(fmt.Sprintf("domain in blacklist: %s", host), i18n.MsgSSRFDomainInBlacklist, map[string]any{"Host": host})
	}

	// 若未启用对域名应用IP过滤，则到此通过
	if !p.ApplyIPFilterForDomain {
		return nil
	}

	// 解析域名对应IP并检查
	ips, err := net.LookupIP(host)
	if err != nil {
		return i18n.NewErrorWithFallback(fmt.Sprintf("DNS resolution failed for %s: %v", host, err), i18n.MsgSSRFDNSResolutionFailed, map[string]any{"Host": host, "Error": err.Error()}).WithCause(err)
	}
	for _, ip := range ips {
		if !p.IsIPAccessAllowed(ip) {
			if isPrivateIP(ip) && !p.AllowPrivateIp {
				return i18n.NewErrorWithFallback(fmt.Sprintf("private IP address not allowed: %s resolves to %s", host, ip.String()), i18n.MsgSSRFResolvedPrivateIP, map[string]any{"Host": host, "IP": ip.String()})
			}
			if p.IpFilterMode {
				return i18n.NewErrorWithFallback(fmt.Sprintf("ip not in whitelist: %s resolves to %s", host, ip.String()), i18n.MsgSSRFResolvedIPNotInWhitelist, map[string]any{"Host": host, "IP": ip.String()})
			}
			return i18n.NewErrorWithFallback(fmt.Sprintf("ip in blacklist: %s resolves to %s", host, ip.String()), i18n.MsgSSRFResolvedIPInBlacklist, map[string]any{"Host": host, "IP": ip.String()})
		}
	}
	return nil
}

// BuildSSRFProtection 从 FetchSetting 形态的参数构建 SSRF 防护配置。
// allowedPorts 支持 "80"、"8000-9000" 端口范围格式，解析失败返回错误。
func BuildSSRFProtection(allowPrivateIp bool, domainFilterMode bool, ipFilterMode bool, domainList, ipList, allowedPorts []string, applyIPFilterForDomain bool) (*SSRFProtection, error) {
	allowedPortInts, err := parsePortRanges(allowedPorts)
	if err != nil {
		return nil, fmt.Errorf("invalid port configuration: %v", err)
	}
	return &SSRFProtection{
		AllowPrivateIp:         allowPrivateIp,
		DomainFilterMode:       domainFilterMode,
		DomainList:             domainList,
		IpFilterMode:           ipFilterMode,
		IpList:                 ipList,
		AllowedPorts:           allowedPortInts,
		ApplyIPFilterForDomain: applyIPFilterForDomain,
	}, nil
}

// CheckConnectedIP 在连接时刻复查实际连接的 IP 是否符合规则。
// ValidateURL 的 net.LookupIP 与 transport 实际拨号是两次独立解析，
// TTL=0 的权威 DNS 可以在校验后切换应答（DNS rebinding），使校验时返回
// 公网 IP、连接时返回内网 IP。本方法供拨号阶段对最终连接的 IP 复查，
// 确保校验规则与实际连接目标一致。
func (p *SSRFProtection) CheckConnectedIP(ip net.IP) error {
	if p.IsIPAccessAllowed(ip) {
		return nil
	}
	if isPrivateIP(ip) && !p.AllowPrivateIp {
		return i18n.NewErrorWithFallback(fmt.Sprintf("connected IP %s is not allowed (private address, possible DNS rebinding)", ip.String()), i18n.MsgSSRFResolvedPrivateIP, map[string]any{"Host": "connected", "IP": ip.String()})
	}
	if p.IpFilterMode {
		return i18n.NewErrorWithFallback(fmt.Sprintf("connected IP %s is not in the ip whitelist", ip.String()), i18n.MsgSSRFResolvedIPNotInWhitelist, map[string]any{"Host": "connected", "IP": ip.String()})
	}
	return i18n.NewErrorWithFallback(fmt.Sprintf("connected IP %s is in the ip blacklist", ip.String()), i18n.MsgSSRFResolvedIPInBlacklist, map[string]any{"Host": "connected", "IP": ip.String()})
}

// ValidateURLWithFetchSetting 使用FetchSetting配置验证URL
func ValidateURLWithFetchSetting(urlStr string, enableSSRFProtection, allowPrivateIp bool, domainFilterMode bool, ipFilterMode bool, domainList, ipList, allowedPorts []string, applyIPFilterForDomain bool) error {
	// 如果SSRF防护被禁用，直接返回成功
	if !enableSSRFProtection {
		return nil
	}

	protection, err := BuildSSRFProtection(allowPrivateIp, domainFilterMode, ipFilterMode, domainList, ipList, allowedPorts, applyIPFilterForDomain)
	if err != nil {
		return i18n.NewErrorWithFallback(fmt.Sprintf("request reject - invalid port configuration: %v", err), i18n.MsgSSRFInvalidPortConfig, map[string]any{"Error": err.Error()}).WithCause(err)
	}
	return protection.ValidateURL(urlStr)
}
