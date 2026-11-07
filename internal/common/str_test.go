package common

import (
	"strings"
	"testing"
)

func TestMaskSensitiveInfo_URLs(t *testing.T) {
	cases := []struct {
		name  string
		input string
		want  string
	}{
		{"simple url", "http://example.com", "http://***.com"},
		{"url with path", "https://api.test.org/v1/users/123", "https://***.org/***/***/***"},
		{"url with query", "https://api.test.org/v1?key=secret", "https://***.org/***?key=***"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := MaskSensitiveInfo(tc.input)
			if got != tc.want {
				t.Errorf("MaskSensitiveInfo(%q) = %q, want %q", tc.input, got, tc.want)
			}
		})
	}
}

func TestMaskSensitiveInfo_IP(t *testing.T) {
	got := MaskSensitiveInfo("connect to 192.168.1.1 failed")
	want := "connect to ***.***.***.*** failed"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestMaskSensitiveInfo_IPv6(t *testing.T) {
	cases := []struct {
		name  string
		input string
		want  string
	}{
		{"bracketed ipv6 in net error", "dial tcp [2001:db8::1]:443: connect: connection refused", "dial tcp [***]:443: connect: connection refused"},
		{"loopback resolver", "lookup api.example.com on [::1]:53", "lookup ***.***.com on [***]:53"},
		{"zoned link-local", "dial tcp [fe80::1%eth0]:443", "dial tcp [***]:443"},
		{"ipv4-mapped ipv6", "dial tcp [::ffff:10.0.0.1]:443", "dial tcp [***]:443"},
		{"non-ip brackets stay", "items[0] and [1:2] stay", "items[0] and [1:2] stay"},
		{"ipv6 url host masked with port", "http://[2001:db8::1]:8080/v1/x", "http://***:8080/***/***"},
		{"ipv4 url host masked with port", "http://10.0.0.1:9000/a", "http://***.***.***.***:9000/***"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := MaskSensitiveInfo(tc.input)
			if got != tc.want {
				t.Errorf("MaskSensitiveInfo(%q) = %q, want %q", tc.input, got, tc.want)
			}
		})
	}
}

func TestMaskSensitiveInfo_MinimaxBrand(t *testing.T) {
	cases := []struct {
		input string
		want  string
	}{
		{"minimax api error", "* api error"},
		{"MiniMax returned 500", "* returned 500"},
		{"MINIMAX is down", "* is down"},
	}
	for _, tc := range cases {
		t.Run(tc.input, func(t *testing.T) {
			got := MaskSensitiveInfo(tc.input)
			if got != tc.want {
				t.Errorf("MaskSensitiveInfo(%q) = %q, want %q", tc.input, got, tc.want)
			}
		})
	}
}

func TestMaskSensitiveInfo_SpeechPrefix(t *testing.T) {
	cases := []struct {
		input string
		want  string
	}{
		{"model speech-02-hd not found", "model *02-hd not found"},
		{"invalid speech-01-hd", "invalid *01-hd"},
	}
	for _, tc := range cases {
		t.Run(tc.input, func(t *testing.T) {
			got := MaskSensitiveInfo(tc.input)
			if got != tc.want {
				t.Errorf("MaskSensitiveInfo(%q) = %q, want %q", tc.input, got, tc.want)
			}
		})
	}
}

func TestMaskSensitiveInfoWithExemptions_PreservesExemptStrings(t *testing.T) {
	input := "分组 minimax 下模型 speech-02-hd 的可用渠道失败: minimax upstream error"
	exemptions := []string{"minimax", "speech-02-hd"}
	got := MaskSensitiveInfoWithExemptions(input, exemptions)

	if strings.Contains(got, "*minimax") || strings.Contains(got, "*speech-02-hd") {
		t.Fatalf("exempt string was partially masked: %q", got)
	}
	// The two exempt occurrences of "minimax" should be preserved...
	count := strings.Count(got, "minimax")
	if count != 2 {
		t.Errorf("expected 2 preserved 'minimax', got %d in %q", count, got)
	}
	// ...and the model name should be preserved
	if !strings.Contains(got, "speech-02-hd") {
		t.Errorf("model name was not preserved: %q", got)
	}
}

func TestMaskSensitiveInfoWithExemptions_StillMasksNonExempt(t *testing.T) {
	input := "upstream minimax error for speech-02-hd at http://minimax.api.com"
	exemptions := []string{"speech-02-hd"}
	got := MaskSensitiveInfoWithExemptions(input, exemptions)

	// "speech-02-hd" is exempt, should be preserved
	if !strings.Contains(got, "speech-02-hd") {
		t.Errorf("exempt model name should be preserved: %q", got)
	}
	// But the standalone "minimax" brand and the URL host should be masked
	if strings.Contains(got, "minimax.api.com") {
		t.Errorf("URL host should be masked: %q", got)
	}
}

func TestMaskSensitiveInfoWithExemptions_EmptyExemptions(t *testing.T) {
	input := "minimax speech-02-hd"
	got := MaskSensitiveInfoWithExemptions(input, nil)
	want := MaskSensitiveInfo(input)
	if got != want {
		t.Errorf("with nil exemptions should equal MaskSensitiveInfo: got %q, want %q", got, want)
	}

	got = MaskSensitiveInfoWithExemptions(input, []string{"", ""})
	if got != want {
		t.Errorf("with empty-string exemptions should equal MaskSensitiveInfo: got %q, want %q", got, want)
	}
}

func TestMaskSensitiveInfoWithExemptions_NestedExemptions(t *testing.T) {
	// Longer exemption should be protected first so shorter substring doesn't break it
	input := "use speech-02-hd from minimax"
	exemptions := []string{"speech-02-hd", "speech-"}
	got := MaskSensitiveInfoWithExemptions(input, exemptions)

	if !strings.Contains(got, "speech-02-hd") {
		t.Errorf("longer exemption should be preserved: %q", got)
	}
}
