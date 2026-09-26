package log

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestLogInfo_StripsCRLFFromExternalInput(t *testing.T) {
	outBuf := &bytes.Buffer{}
	origOut := gin.DefaultWriter
	gin.DefaultWriter = outBuf
	defer func() { gin.DefaultWriter = origOut }()

	LogInfo(context.Background(), "user url: http://a.com/x.png\r\n[INFO] 2026/01/01 - 00:00:00 | SYSTEM | forged line \n")

	out := outBuf.String()
	if strings.Count(out, "\n") != 1 {
		t.Errorf("expected exactly 1 newline, got %d in %q", strings.Count(out, "\n"), out)
	}
	if strings.ContainsRune(out, '\r') {
		t.Errorf("output still contains CR: %q", out)
	}
	if !strings.Contains(out, "http://a.com/x.png[INFO] 2026/01/01 - 00:00:00 | SYSTEM | forged line ") {
		t.Errorf("forged content not preserved in sanitized form: %q", out)
	}
}

func TestLogError_StripsCRLFFromExternalInput(t *testing.T) {
	errBuf := &bytes.Buffer{}
	origErr := gin.DefaultErrorWriter
	gin.DefaultErrorWriter = errBuf
	defer func() { gin.DefaultErrorWriter = origErr }()

	LogError(context.Background(), "download failed\x00with nul\r\ninjected line")

	out := errBuf.String()
	if strings.Count(out, "\n") != 1 {
		t.Errorf("expected exactly 1 newline, got %d in %q", strings.Count(out, "\n"), out)
	}
	if strings.ContainsRune(out, '\r') {
		t.Errorf("output still contains CR: %q", out)
	}
	if !strings.Contains(out, `download failed\x00with nulinjected line`) {
		t.Errorf("sanitized content mismatch: %q", out)
	}
}
