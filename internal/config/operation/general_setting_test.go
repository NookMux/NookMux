package operation

import "testing"

func TestValidateTokenKeyPrefix(t *testing.T) {
	t.Run("valid prefixes", func(t *testing.T) {
		for _, prefix := range []string{
			"sk-",
			"自定义-",
			"sk-自定义-",
			"team-a-",
			"nook.mux-1-",
		} {
			if err := ValidateTokenKeyPrefix(prefix); err != nil {
				t.Fatalf("ValidateTokenKeyPrefix(%q) = %v, want nil", prefix, err)
			}
		}
	})

	t.Run("invalid prefixes", func(t *testing.T) {
		for _, prefix := range []string{
			"",                   // 空
			"-",                  // 仅分隔符
			"sk",                 // 未以 - 结尾
			"自定义",                // 未以 - 结尾
			"sk -",               // 含空格
			"sk\n-",              // 含控制字符
			"sk%-",               // 含 LIKE 通配符
			"sk-\t",              // 制表符
			"0123456789abcdefg-", // 超过 16 字符
		} {
			if err := ValidateTokenKeyPrefix(prefix); err == nil {
				t.Fatalf("ValidateTokenKeyPrefix(%q) = nil, want error", prefix)
			}
		}
	})
}
