package userstore

import (
	"testing"
	"time"

	"github.com/NookMux/NookMux/internal/infra/security"
)

func TestUserBaseCacheMACBindsAuthMetadata(t *testing.T) {
	base := UserBase{
		Id:          1,
		Group:       "default",
		Email:       "user@example.com",
		Status:      1,
		Role:        1,
		Username:    "user",
		AuthVersion: 1,
	}
	if err := stampUserCacheMAC(&base); err != nil {
		t.Fatalf("stampUserCacheMAC: %v", err)
	}
	if base.CacheSchema != userCacheSchemaVersion {
		t.Fatalf("expected cache schema %d, got %d", userCacheSchemaVersion, base.CacheSchema)
	}
	if !security.ValidAuthCacheMAC(base.CacheMAC, base.CacheMAC, base.CacheExpiresAt) {
		t.Fatal("freshly stamped MAC must validate")
	}

	privilegeEscalation := base
	privilegeEscalation.Role = 100
	if mac, _ := privilegeEscalation.userCacheMAC(); mac == base.CacheMAC {
		t.Fatal("role tampering must change the MAC")
	}

	statusFlip := base
	statusFlip.Status = 2
	if mac, _ := statusFlip.userCacheMAC(); mac == base.CacheMAC {
		t.Fatal("status tampering must change the MAC")
	}

	versionBump := base
	versionBump.AuthVersion++
	if mac, _ := versionBump.userCacheMAC(); mac == base.CacheMAC {
		t.Fatal("auth version bump must change the MAC")
	}

	quotaChange := base
	quotaChange.Quota = 999999
	if mac, _ := quotaChange.userCacheMAC(); mac != base.CacheMAC {
		t.Fatal("quota is maintained by atomic increments and must be excluded from the MAC")
	}

	if security.ValidAuthCacheMAC(base.CacheMAC, base.CacheMAC, time.Now().Add(-time.Second).UnixMilli()) {
		t.Fatal("expired cache entry must not validate")
	}
}
