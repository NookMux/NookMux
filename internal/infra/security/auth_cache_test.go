package security

import (
	"testing"
	"time"
)

func TestAuthCacheMACIsDeterministicAndKeyBound(t *testing.T) {
	metadata := struct {
		Role int `json:"role"`
	}{Role: 10}

	first, err := AuthCacheMAC("user:1", metadata)
	if err != nil {
		t.Fatalf("AuthCacheMAC: %v", err)
	}
	second, err := AuthCacheMAC("user:1", metadata)
	if err != nil {
		t.Fatalf("AuthCacheMAC: %v", err)
	}
	if first != second {
		t.Fatal("MAC must be deterministic for identical key and metadata")
	}

	otherKey, err := AuthCacheMAC("user:2", metadata)
	if err != nil {
		t.Fatalf("AuthCacheMAC: %v", err)
	}
	if first == otherKey {
		t.Fatal("MAC must bind the cache key name")
	}
}

func TestAuthCacheMACDetectsMetadataTampering(t *testing.T) {
	base := struct {
		Role   int `json:"role"`
		Status int `json:"status"`
	}{Role: 1, Status: 1}
	tampered := base
	tampered.Role = 100

	baseMAC, err := AuthCacheMAC("user:1", base)
	if err != nil {
		t.Fatalf("AuthCacheMAC: %v", err)
	}
	tamperedMAC, err := AuthCacheMAC("user:1", tampered)
	if err != nil {
		t.Fatalf("AuthCacheMAC: %v", err)
	}
	if baseMAC == tamperedMAC {
		t.Fatal("MAC must change when auth metadata changes")
	}
	if !ValidAuthCacheMAC(baseMAC, baseMAC, time.Now().Add(time.Minute).UnixMilli()) {
		t.Fatal("valid MAC with future expiry must pass")
	}
	if ValidAuthCacheMAC(baseMAC, tamperedMAC, time.Now().Add(time.Minute).UnixMilli()) {
		t.Fatal("tampered MAC must not validate")
	}
}

func TestValidAuthCacheMACRejectsExpiredEntries(t *testing.T) {
	mac, err := AuthCacheMAC("user:1", map[string]int{"role": 1})
	if err != nil {
		t.Fatalf("AuthCacheMAC: %v", err)
	}
	if ValidAuthCacheMAC(mac, mac, time.Now().Add(-time.Second).UnixMilli()) {
		t.Fatal("expired cache entry must not validate")
	}
	if ValidAuthCacheMAC(mac, mac, 0) {
		t.Fatal("zero expiry must not validate")
	}
}
