package common

import (
	"bytes"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestSanitizeLogLine_StripsCRLF(t *testing.T) {
	cases := []struct {
		name  string
		input string
		want  string
	}{
		{"crlf between text", "url=http://a.com\r\n[SYS] forged | line", "url=http://a.com[SYS] forged | line"},
		{"lone cr", "abc\rdef", "abcdef"},
		{"lone lf", "abc\ndef", "abcdef"},
		{"leading and trailing", "\r\ninjected\n", "injected"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := SanitizeLogLine(tc.input)
			if got != tc.want {
				t.Errorf("SanitizeLogLine(%q) = %q, want %q", tc.input, got, tc.want)
			}
		})
	}
}

func TestSanitizeLogLine_ReplacesOtherControlChars(t *testing.T) {
	cases := []struct {
		name  string
		input string
		want  string
	}{
		{"nul byte", "a\x00b", "a\\x00b"},
		{"esc terminal sequence", "\x1b[31mred\x1b[0m", "\\x1B[31mred\\x1B[0m"},
		{"del char", "x\x7fy", "x\\x7Fy"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := SanitizeLogLine(tc.input)
			if got != tc.want {
				t.Errorf("SanitizeLogLine(%q) = %q, want %q", tc.input, got, tc.want)
			}
		})
	}
}

func TestSanitizeLogLine_PreservesTabAndCleanText(t *testing.T) {
	cases := []struct {
		name  string
		input string
		want  string
	}{
		{"tab kept", "field\tvalue", "field\tvalue"},
		{"clean ascii", "downloading from origin: https://a.com/x.png", "downloading from origin: https://a.com/x.png"},
		{"empty string", "", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := SanitizeLogLine(tc.input)
			if got != tc.want {
				t.Errorf("SanitizeLogLine(%q) = %q, want %q", tc.input, got, tc.want)
			}
		})
	}
}

func TestSanitizeLogLine_PreservesMultibyteUTF8(t *testing.T) {
	cases := []struct {
		name  string
		input string
		want  string
	}{
		{"chinese text", "下载中文文件名.png", "下载中文文件名.png"},
		{"chinese with crlf", "中文URL\r\nhttps://例え.jp/画像.png\n", "中文URLhttps://例え.jp/画像.png"},
		{"emoji", "emoji 🚀 payload \r\ninjected", "emoji 🚀 payload injected"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := SanitizeLogLine(tc.input)
			if got != tc.want {
				t.Errorf("SanitizeLogLine(%q) = %q, want %q", tc.input, got, tc.want)
			}
		})
	}
}

// captureSysWriter 将 gin 输出 writer 临时替换为 buffer，返回还原函数。
func captureSysWriter(t *testing.T) (*bytes.Buffer, *bytes.Buffer, func()) {
	t.Helper()
	outBuf := &bytes.Buffer{}
	errBuf := &bytes.Buffer{}
	origOut := gin.DefaultWriter
	origErr := gin.DefaultErrorWriter
	gin.DefaultWriter = outBuf
	gin.DefaultErrorWriter = errBuf
	return outBuf, errBuf, func() {
		gin.DefaultWriter = origOut
		gin.DefaultErrorWriter = origErr
	}
}

func assertSingleLine(t *testing.T, name string, out string, wantContain string) {
	t.Helper()
	if strings.Count(out, "\n") != 1 {
		t.Errorf("%s: expected exactly 1 newline, got %d in %q", name, strings.Count(out, "\n"), out)
	}
	if strings.ContainsRune(out, '\r') {
		t.Errorf("%s: output still contains CR: %q", name, out)
	}
	if !strings.Contains(out, wantContain) {
		t.Errorf("%s: forged content not preserved in sanitized form: %q, want contain %q", name, out, wantContain)
	}
}

func TestSysLog_ForgedLineIsCollapsed(t *testing.T) {
	outBuf, _, restore := captureSysWriter(t)
	defer restore()

	SysLog("downloading from origin: http://a.com/x\r\n[SYS] 2026/01/01 - 00:00:00 | forged admin action \n")

	assertSingleLine(t, "SysLog", outBuf.String(), "http://a.com/x[SYS] 2026/01/01 - 00:00:00 | forged admin action ")
}

func TestSysError_ForgedLineIsCollapsed(t *testing.T) {
	_, errBuf, restore := captureSysWriter(t)
	defer restore()

	SysError("request failed\r\n[SYS] forged | critical error \n")

	assertSingleLine(t, "SysError", errBuf.String(), "request failed[SYS] forged | critical error ")
}
