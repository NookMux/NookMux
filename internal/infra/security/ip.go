package security

import (
	"fmt"
	"net"
)

func IsIP(s string) bool {
	ip := net.ParseIP(s)
	return ip != nil
}

func ParseIP(s string) net.IP {
	return net.ParseIP(s)
}

func IsPrivateIP(ip net.IP) bool {
	if ip.IsLoopback() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() {
		return true
	}

	private := []net.IPNet{
		{IP: net.IPv4(10, 0, 0, 0), Mask: net.CIDRMask(8, 32)},
		{IP: net.IPv4(172, 16, 0, 0), Mask: net.CIDRMask(12, 32)},
		{IP: net.IPv4(192, 168, 0, 0), Mask: net.CIDRMask(16, 32)},
	}

	for _, privateNet := range private {
		if privateNet.Contains(ip) {
			return true
		}
	}
	return false
}

func IsIpInCIDRList(ip net.IP, cidrList []string) bool {
	for _, cidr := range cidrList {
		_, network, err := net.ParseCIDR(cidr)
		if err != nil {
			// 尝试作为单个IP处理
			if whitelistIP := net.ParseIP(cidr); whitelistIP != nil {
				if ip.Equal(whitelistIP) {
					return true
				}
			}
			continue
		}

		if network.Contains(ip) {
			return true
		}
	}
	return false
}

// IPMatcher 是预解析的 IP / CIDR 列表，条目可以是单个地址或网段，供逐请求匹配使用。
type IPMatcher struct {
	networks  []*net.IPNet
	addresses []net.IP
}

// NewIPMatcher 解析 IP / CIDR 列表，任一条目既不是网段也不是合法地址时返回错误。
func NewIPMatcher(entries []string) (*IPMatcher, error) {
	matcher := &IPMatcher{}
	for _, entry := range entries {
		if _, network, err := net.ParseCIDR(entry); err == nil {
			matcher.networks = append(matcher.networks, network)
			continue
		}
		address := net.ParseIP(entry)
		if address == nil {
			return nil, fmt.Errorf("invalid IP or CIDR %q", entry)
		}
		matcher.addresses = append(matcher.addresses, address)
	}
	return matcher, nil
}

// Contains 判断地址是否命中列表；网段按包含关系判断，单个地址按等值判断。
func (m *IPMatcher) Contains(ip net.IP) bool {
	if m == nil || ip == nil {
		return false
	}
	for _, network := range m.networks {
		if network.Contains(ip) {
			return true
		}
	}
	for _, address := range m.addresses {
		if ip.Equal(address) {
			return true
		}
	}
	return false
}
