package middleware

import (
	"fmt"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/NookMux/NookMux/internal/common"
	"github.com/NookMux/NookMux/internal/infra/security"
	"github.com/gin-gonic/gin"
)

// 同一来源地址两次提示之间的最小间隔、清理过期提示记录的最小间隔，以及提示记录的条数上限。
// 记录表只登记来源地址，上限防止大量不同来源的请求把内存撑大。
const (
	forwardedHeaderHintInterval = 10 * time.Minute
	forwardedHeaderHintSweep    = time.Minute
	forwardedHeaderHintLimit    = 1024
)

// ForwardedHeaderIgnoredHint 在请求带着转发头、直连来源却不在可信代理列表时，按来源地址限频
// 记录一条提示：转发头被忽略，经该反向代理或 CDN 进来的访客都按代理地址计数、共用同一限流额度。
// trusted 是 security.SetupGinTrustedProxies 应用的可信代理匹配器；为 nil 表示未启用，不提示。
func ForwardedHeaderIgnoredHint(trusted *security.IPMatcher) gin.HandlerFunc {
	if trusted == nil {
		return defNext
	}

	var (
		mutex     sync.Mutex
		lastHint  = make(map[string]time.Time)
		lastSweep time.Time
	)
	return func(c *gin.Context) {
		header := forwardedClientIPHeader(c.Request.Header)
		if header == "" {
			c.Next()
			return
		}
		address := c.RemoteIP()
		if address == "" || trusted.Contains(net.ParseIP(address)) {
			c.Next()
			return
		}

		now := time.Now()
		mutex.Lock()
		if now.Sub(lastSweep) >= forwardedHeaderHintSweep {
			for reported, at := range lastHint {
				if now.Sub(at) >= forwardedHeaderHintInterval {
					delete(lastHint, reported)
				}
			}
			lastSweep = now
		}
		if last, reported := lastHint[address]; reported && now.Sub(last) < forwardedHeaderHintInterval {
			mutex.Unlock()
			c.Next()
			return
		}
		if len(lastHint) < forwardedHeaderHintLimit {
			lastHint[address] = now
		}
		mutex.Unlock()

		common.SysError(fmt.Sprintf(
			"request from %s carries the %s header but the source address is not in TRUSTED_PROXIES: "+
				"the header is ignored and every client coming through this address shares one rate limit bucket; "+
				"add the address to TRUSTED_PROXIES if the reverse proxy or CDN is yours",
			address, header))
		c.Next()
	}
}

// forwardedClientIPHeader 返回请求携带的第一个用于解析真实客户端 IP 的转发头名称，没有则返回空串。
func forwardedClientIPHeader(header http.Header) string {
	for _, name := range security.ForwardedClientIPHeaders {
		if header.Get(name) != "" {
			return name
		}
	}
	return ""
}
