package log

import (
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"time"

	"github.com/NookMux/NookMux/internal/common"
	"github.com/NookMux/NookMux/internal/infra/runtime"
	"github.com/NookMux/NookMux/pkg/jsonx"
	"github.com/gin-gonic/gin"
)

const (
	loggerINFO  = "INFO"
	loggerWarn  = "WARN"
	loggerError = "ERR"
	loggerDebug = "DEBUG"
)

const maxLogCount = 1000000

// logCount/setupLogWorking 会被多 goroutine 并发读写（relay 各 handler、
// 计费结算循环与父 goroutine 并行打日志），必须原子访问；WSS -race 实跑
// 曾捕获并发 logCount++ 竞态。
var logCount atomic.Int64
var setupLogLock sync.Mutex
var setupLogWorking atomic.Bool

// Dir 是日志文件目录，由 app.InitEnv 在 flag.Parse 后注入（--log-dir flag
// 归 app 层持有；本包不得反向 import app，否则与启动装配成环）。
// 为空时 SetupLogger 不落盘，仅写 stdout/stderr。
var Dir string

func SetupLogger() {
	defer func() {
		setupLogWorking.Store(false)
	}()
	if Dir != "" {
		ok := setupLogLock.TryLock()
		if !ok {
			log.Println("setup log is already working")
			return
		}
		defer func() {
			setupLogLock.Unlock()
		}()
		logPath := filepath.Join(Dir, fmt.Sprintf("oneapi-%s.log", time.Now().Format("20060102150405")))
		fd, err := os.OpenFile(logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if err != nil {
			log.Fatal("failed to open log file")
		}
		gin.DefaultWriter = io.MultiWriter(os.Stdout, fd)
		gin.DefaultErrorWriter = io.MultiWriter(os.Stderr, fd)
	}
}

func LogInfo(ctx context.Context, msg string) {
	logHelper(ctx, loggerINFO, msg)
}

func LogWarn(ctx context.Context, msg string) {
	logHelper(ctx, loggerWarn, msg)
}

func LogError(ctx context.Context, msg string) {
	logHelper(ctx, loggerError, msg)
}

func LogDebug(ctx context.Context, msg string, args ...any) {
	if common.DebugEnabled {
		if len(args) > 0 {
			msg = fmt.Sprintf(msg, args...)
		}
		logHelper(ctx, loggerDebug, msg)
	}
}

func logHelper(ctx context.Context, level string, msg string) {
	writer := gin.DefaultErrorWriter
	if level == loggerINFO {
		writer = gin.DefaultWriter
	}
	id := ctx.Value(common.ContextKeyRequestId)
	if id == nil {
		id = "SYSTEM"
	}
	now := time.Now()
	_, _ = fmt.Fprintf(writer, "[%s] %v | %s | %s \n", level, now.Format("2006/01/02 - 15:04:05"), id, msg)
	logCount.Add(1) // we don't need accurate count, but concurrent logging requires atomic access
	if logCount.Load() > maxLogCount && !setupLogWorking.Load() {
		logCount.Store(0)
		setupLogWorking.Store(true)
		runtime.RelayGo(func() {
			SetupLogger()
		})
	}
}

func LogQuota(quota int) string {
	q := float64(quota)
	return fmt.Sprintf("＄%.6f 额度", q/common.QuotaPerUnit)
}

func FormatQuota(quota int) string {
	q := float64(quota)
	return fmt.Sprintf("＄%.6f", q/common.QuotaPerUnit)
}

// LogJson 仅供测试使用 only for test
func LogJson(ctx context.Context, msg string, obj any) {
	jsonStr, err := jsonx.Marshal(obj)
	if err != nil {
		LogError(ctx, fmt.Sprintf("json marshal failed: %s", err.Error()))
		return
	}
	LogDebug(ctx, fmt.Sprintf("%s | %s", msg, string(jsonStr)))
}
