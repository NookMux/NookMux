package tokenstore

import (
	"testing"
	"time"

	"github.com/NookMux/NookMux/internal/infra/security"
)

func TestTokenCacheMACBindsAuthMetadata(t *testing.T) {
	base := Token{
		Id:          1,
		UserId:      1,
		Key:         "sk-test-key",
		Status:      1,
		Name:        "test",
		Group:       "default",
		QuotaType:   1,
		RemainQuota: 5000,
	}
	if err := stampTokenCacheMAC(&base, tokenCacheKey(base.Key)); err != nil {
		t.Fatalf("stampTokenCacheMAC: %v", err)
	}
	if !security.ValidAuthCacheMAC(base.CacheMAC, base.CacheMAC, base.CacheExpiresAt) {
		t.Fatal("freshly stamped MAC must validate")
	}

	statusFlip := base
	statusFlip.Status = 3
	if mac, _ := statusFlip.tokenCacheMAC(tokenCacheKey(base.Key)); mac == base.CacheMAC {
		t.Fatal("status tampering must change the MAC")
	}

	groupChange := base
	groupChange.Group = "vip"
	if mac, _ := groupChange.tokenCacheMAC(tokenCacheKey(base.Key)); mac == base.CacheMAC {
		t.Fatal("group tampering must change the MAC")
	}

	counterChange := base
	counterChange.RemainQuota = 0
	counterChange.UsedQuota = 12345
	if mac, _ := counterChange.tokenCacheMAC(tokenCacheKey(base.Key)); mac != base.CacheMAC {
		t.Fatal("runtime counters are maintained by atomic increments and must be excluded from the MAC")
	}

	if security.ValidAuthCacheMAC(base.CacheMAC, base.CacheMAC, time.Now().Add(-time.Second).UnixMilli()) {
		t.Fatal("expired cache entry must not validate")
	}
}

func TestTokenCacheMACNormalizesEmptyPointerFields(t *testing.T) {
	nilPtr := Token{Id: 1, Key: "sk-x", Status: 1}
	if err := stampTokenCacheMAC(&nilPtr, tokenCacheKey(nilPtr.Key)); err != nil {
		t.Fatalf("stampTokenCacheMAC: %v", err)
	}

	empty := ""
	withEmptyPtr := nilPtr
	withEmptyPtr.AllowIps = &empty
	withEmptyPtr.ModelMapping = &empty
	// 哈希往返会把空字符串指针读回为 nil，两种形态的签名必须一致。
	if mac, _ := withEmptyPtr.tokenCacheMAC(tokenCacheKey(nilPtr.Key)); mac != nilPtr.CacheMAC {
		t.Fatal("empty-string pointers and nil must produce the same MAC")
	}
}
