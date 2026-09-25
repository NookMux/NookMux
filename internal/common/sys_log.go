package common

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

const logHexDigits = "0123456789ABCDEF"

// SanitizeLogLine 净化单条日志文本，阻断用户可控内容伪造日志行：
// CR/LF 作为日志行伪造的直接载体直接剥离，保证输出恒为单行；
// 其余 C0 控制字符（0x00-0x1F 除 \t 外）与 DEL（0x7F）替换为 \xNN
// 转义表示，既保留原始字节的取证信息，又避免终端转义注入；
// \t 为合法的日志字段分隔符予以保留。仅按 ASCII 控制字节范围处理，
// 多字节 UTF-8 序列（各字节均 >= 0x80）不受影响。
func SanitizeLogLine(s string) string {
	needsSanitize := false
	for i := 0; i < len(s); i++ {
		if b := s[i]; (b < 0x20 && b != '\t') || b == 0x7F {
			needsSanitize = true
			break
		}
	}
	if !needsSanitize {
		return s
	}
	var b strings.Builder
	b.Grow(len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case c == '\r' || c == '\n':
		case c < 0x20 || c == 0x7F:
			b.WriteString("\\x")
			b.WriteByte(logHexDigits[c>>4])
			b.WriteByte(logHexDigits[c&0x0F])
		default:
			b.WriteByte(c)
		}
	}
	return b.String()
}

func SysLog(s string) {
	t := time.Now()
	_, _ = fmt.Fprintf(gin.DefaultWriter, "[SYS] %v | %s \n", t.Format("2006/01/02 - 15:04:05"), SanitizeLogLine(s))
}

func SysError(s string) {
	t := time.Now()
	_, _ = fmt.Fprintf(gin.DefaultErrorWriter, "[SYS] %v | %s \n", t.Format("2006/01/02 - 15:04:05"), SanitizeLogLine(s))
}

func FatalLog(v ...any) {
	t := time.Now()
	_, _ = fmt.Fprintf(gin.DefaultErrorWriter, "[FATAL] %v | %v \n", t.Format("2006/01/02 - 15:04:05"), v)
	os.Exit(1)
}

func LogStartupSuccess(startTime time.Time, port string) {

	duration := time.Since(startTime)
	durationMs := duration.Milliseconds()

	// Get network IPs
	networkIps := GetNetworkIps()

	// Print blank line for spacing
	fmt.Fprintf(gin.DefaultWriter, "\n")

	// Print the main success message
	fmt.Fprintf(gin.DefaultWriter, "  \033[32m%s %s\033[0m  ready in %d ms\n", SystemName, Version, durationMs)
	fmt.Fprintf(gin.DefaultWriter, "\n")

	// Skip fancy startup message in container environments
	if !IsRunningInContainer() {
		// Print local URL
		fmt.Fprintf(gin.DefaultWriter, "  ➜  \033[1mLocal:\033[0m   http://localhost:%s/\n", port)
	}

	// Print network URLs
	for _, ip := range networkIps {
		fmt.Fprintf(gin.DefaultWriter, "  ➜  \033[1mNetwork:\033[0m http://%s:%s/\n", ip, port)
	}

	// Print blank line for spacing
	fmt.Fprintf(gin.DefaultWriter, "\n")
}
