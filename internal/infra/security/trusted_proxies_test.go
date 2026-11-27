package security

import (
	"net"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

// clientIPFor 以指定的直连来源与请求头发一次请求，返回引擎解析出的客户端 IP。
func clientIPFor(engine *gin.Engine, remoteAddr, header, value string) string {
	var resolved string
	engine.GET("/", func(c *gin.Context) { resolved = c.ClientIP() })
	request := httptest.NewRequest(http.MethodGet, "/", nil)
	request.RemoteAddr = remoteAddr
	if header != "" {
		request.Header.Set(header, value)
	}
	engine.ServeHTTP(httptest.NewRecorder(), request)
	return resolved
}

func TestSetupGinTrustedProxiesReturnsAppliedProxies(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("default trusts private ranges and cloudflare", func(t *testing.T) {
		engine := gin.New()
		applied := SetupGinTrustedProxies(engine)
		if applied == nil {
			t.Fatal("applied trusted proxies = nil, want the default list")
		}
		for _, trusted := range []string{"172.18.0.2", "127.0.0.1", "104.16.1.1"} {
			if !applied.Contains(net.ParseIP(trusted)) {
				t.Errorf("default list does not trust %s", trusted)
			}
		}
		if applied.Contains(net.ParseIP("198.51.100.25")) {
			t.Error("default list must not trust an arbitrary public address")
		}
	})

	t.Run("explicitly disabled returns nil and ignores headers", func(t *testing.T) {
		t.Setenv(trustedProxiesEnvKey, "none")
		engine := gin.New()
		if applied := SetupGinTrustedProxies(engine); applied != nil {
			t.Fatalf("applied trusted proxies = %v, want nil when trusted proxies are disabled", applied)
		}
		if got, want := clientIPFor(engine, "104.16.1.1:54321", "CF-Connecting-IP", "203.0.113.7"), "104.16.1.1"; got != want {
			t.Fatalf("ClientIP = %q, want %q when trusted proxies are disabled", got, want)
		}
	})

	t.Run("custom list keeps only the configured entries", func(t *testing.T) {
		t.Setenv(trustedProxiesEnvKey, "cloudflare,203.0.113.10")
		engine := gin.New()
		applied := SetupGinTrustedProxies(engine)
		if applied == nil || !applied.Contains(net.ParseIP("203.0.113.10")) || !applied.Contains(net.ParseIP("104.16.1.1")) {
			t.Fatal("applied trusted proxies must contain the cloudflare ranges and 203.0.113.10")
		}
		if applied.Contains(net.ParseIP("10.0.0.1")) {
			t.Fatal("custom list must not trust private ranges that were not configured")
		}
	})

	t.Run("a list gin cannot apply yields no matcher", func(t *testing.T) {
		t.Setenv(trustedProxiesEnvKey, "10.0.0.0/8,not-an-address")
		engine := gin.New()
		if applied := SetupGinTrustedProxies(engine); applied != nil {
			t.Fatalf("applied trusted proxies = %v, want nil for a list that cannot be applied", applied)
		}
	})
}

// 匹配器对来源地址的判定与 Gin 解析客户端 IP 的规则一致：来源可信才采信转发头。
func TestClientIPFollowsTrustedProxies(t *testing.T) {
	gin.SetMode(gin.TestMode)

	cases := []struct {
		name, remoteAddr, header, value, want string
		trusted                               bool
	}{
		{"private proxy passes the visitor address through", "172.18.0.2:54321", "X-Forwarded-For", "203.0.113.7", "203.0.113.7", true},
		{"public address outside the list is counted as itself", "198.51.100.25:54321", "X-Forwarded-For", "203.0.113.7", "198.51.100.25", false},
		{"cloudflare edge passes CF-Connecting-IP through", "104.16.1.1:54321", "CF-Connecting-IP", "203.0.113.7", "203.0.113.7", true},
	}
	for _, tc := range cases {
		engine := gin.New()
		applied := SetupGinTrustedProxies(engine)
		peer, _, err := net.SplitHostPort(tc.remoteAddr)
		if err != nil {
			t.Fatalf("%s: split remote address: %v", tc.name, err)
		}
		if got := applied.Contains(net.ParseIP(peer)); got != tc.trusted {
			t.Errorf("%s: matcher trusts the source = %v, want %v", tc.name, got, tc.trusted)
		}
		if got := clientIPFor(engine, tc.remoteAddr, tc.header, tc.value); got != tc.want {
			t.Errorf("%s: ClientIP = %q, want %q", tc.name, got, tc.want)
		}
	}
}

func TestIPMatcher(t *testing.T) {
	matcher, err := NewIPMatcher([]string{"127.0.0.1", "::1", "10.0.0.0/8", "2400:cb00::/32"})
	if err != nil {
		t.Fatalf("NewIPMatcher: %v", err)
	}

	cases := map[string]bool{
		"127.0.0.1":    true,
		"::1":          true,
		"10.1.2.3":     true,
		"2400:cb00::1": true,
		"172.16.0.1":   false,
		"2400:cb01::1": false,
	}
	for address, want := range cases {
		if got := matcher.Contains(net.ParseIP(address)); got != want {
			t.Errorf("Contains(%s) = %v, want %v", address, got, want)
		}
	}
	if matcher.Contains(nil) {
		t.Error("Contains(nil) = true, want false")
	}

	if _, err := NewIPMatcher([]string{"10.0.0.0/8", "not-an-address"}); err == nil {
		t.Error("NewIPMatcher accepted an entry that is neither an address nor a CIDR")
	}
}
