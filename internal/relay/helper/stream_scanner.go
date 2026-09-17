package helper

import (
	"bufio"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/NookMux/NookMux/internal/common"
	"github.com/NookMux/NookMux/internal/config/operation"
	"github.com/NookMux/NookMux/internal/infra/log"
	relaycommon "github.com/NookMux/NookMux/internal/relay/common"

	"github.com/NookMux/NookMux/internal/domain/shared"
	"github.com/gin-gonic/gin"
)

const (
	// InitialScannerBufferSize is the starting buffer size for each SSE stream
	// scanner. The previous value (64KB) was far larger than typical SSE data
	// frames (a few hundred bytes). At 100+ concurrent streams this alone
	// reserves 6.4MB+ of heap that is never used. 8KB comfortably covers most
	// SSE lines while keeping per-request footprint small. The scanner still
	// grows on demand up to the configured maximum.
	InitialScannerBufferSize    = 8 << 10 // 8KB
	DefaultMaxScannerBufferSize = 8 << 20 // 8MB default SSE buffer size
	DefaultPingInterval         = 10 * time.Second
)

// scannerBufferPool reuses the initial scanner buffer across concurrent stream
// requests. Without this pool every streaming relay allocates a fresh
// InitialScannerBufferSize buffer that lives until the scanner is garbage
// collected. At 100 concurrent streams with a 64KB buffer that is ~6.4MB of
// transient allocations. The pool returns already-grown buffers to subsequent
// callers so the scanner rarely needs to reallocate.
//
// Buffers that grew beyond initialBufferReclaimCap are still returned: the
// scanner copies the buffer when it grows, so reusing a larger one is safe and
// avoids re-growing on the next request.
var scannerBufferPool = sync.Pool{
	New: func() interface{} {
		b := make([]byte, InitialScannerBufferSize)
		return &b
	},
}

func getScannerBufferSize() int {
	if shared.StreamScannerMaxBufferMB > 0 {
		return shared.StreamScannerMaxBufferMB << 20
	}
	return DefaultMaxScannerBufferSize
}

// StreamScannerHandler 逐行读取上游 SSE 流并转发给 dataHandler。
// 返回值标记流的异常终止原因（超时 / 连接错误），正常结束（EOF、[DONE]、
// 客户端断开、handler 主动停止）返回 nil。调用方应将异常终止作为上游错误
// 上报，而不是把半途而废的响应当作成功，导致计费阶段因 usage 为空伪造
// 「502 上游没有返回计费信息」掩盖真实错误。
func StreamScannerHandler(c *gin.Context, resp *http.Response, info *relaycommon.RelayInfo, dataHandler func(data string) bool) *shared.NookMuxError {

	if resp == nil || dataHandler == nil {
		return nil
	}

	// 确保响应体总是被关闭
	defer func() {
		if resp.Body != nil {
			resp.Body.Close()
		}
	}()

	streamingTimeout := time.Duration(shared.StreamingTimeout) * time.Second
	if streamingTimeout <= 0 {
		streamingTimeout = time.Duration(common.RelayTimeout) * time.Second
	}
	var timeout <-chan time.Time
	var timeoutTimer *time.Timer
	if streamingTimeout > 0 {
		timeoutTimer = time.NewTimer(streamingTimeout)
		defer timeoutTimer.Stop()
		timeout = timeoutTimer.C
	}

	scanner := bufio.NewScanner(resp.Body)

	generalSettings := operation.GetGeneralSetting()
	pingEnabled := generalSettings.PingIntervalEnabled && !info.DisablePing
	pingInterval := time.Duration(generalSettings.PingIntervalSeconds) * time.Second
	if pingInterval <= 0 {
		pingInterval = DefaultPingInterval
	}

	var ping <-chan time.Time
	var pingTicker *time.Ticker
	if pingEnabled {
		pingTicker = time.NewTicker(pingInterval)
		defer pingTicker.Stop()
		ping = pingTicker.C
	}

	if common.DebugEnabled {
		// print timeout and ping interval for debugging
		println("relay timeout seconds:", common.RelayTimeout)
		println("relay max idle conns:", common.RelayMaxIdleConns)
		println("relay max idle conns per host:", common.RelayMaxIdleConnsPerHost)
		println("streaming timeout seconds:", int64(streamingTimeout.Seconds()))
		println("ping interval seconds:", int64(pingInterval.Seconds()))
	}

	bufPtr := scannerBufferPool.Get().(*[]byte)
	scanner.Buffer(*bufPtr, getScannerBufferSize())
	scanner.Split(bufio.ScanLines)
	SetEventStreamHeaders(c)

	stop := make(chan struct{})
	defer close(stop)
	dataChan := make(chan string, 16)
	done := make(chan struct{})
	// scannerErr 在 close(done) 前写入、在 <-done 后读取，close/receive 建立
	// happens-before，无需额外同步。
	var scannerErr error
	go func() {
		defer func() {
			if r := recover(); r != nil {
				log.LogError(c, fmt.Sprintf("stream scanner panic: %v", r))
			}
			scannerBufferPool.Put(bufPtr)
			close(done)
		}()
		for scanner.Scan() {
			data := scanner.Text()
			if common.DebugEnabled {
				println(data)
			}

			if len(data) < 6 {
				continue
			}
			if data[:5] != "data:" && data[:6] != "[DONE]" {
				continue
			}
			data = data[5:]
			data = strings.TrimSpace(data)
			if data == "" {
				continue
			}
			if !strings.HasPrefix(data, "[DONE]") {
				info.SetFirstResponseTime()
				info.ReceivedResponseCount++
				select {
				case dataChan <- data:
				case <-stop:
					return
				case <-c.Request.Context().Done():
					return
				}
			} else {
				// done, 处理完成标志，直接退出停止读取剩余数据防止出错
				if common.DebugEnabled {
					println("received [DONE], stopping scanner")
				}
				return
			}
		}

		if err := scanner.Err(); err != nil && err != io.EOF {
			log.LogError(c, "scanner error: "+err.Error())
			scannerErr = err
		}
	}()

	for {
		select {
		case data := <-dataChan:
			resetTimer(timeoutTimer, streamingTimeout)
			if !safeStreamDataHandler(c, dataHandler, data) {
				return nil
			}
		case <-done:
			for {
				select {
				case data := <-dataChan:
					resetTimer(timeoutTimer, streamingTimeout)
					if !safeStreamDataHandler(c, dataHandler, data) {
						return nil
					}
				default:
					log.LogDebug(c, "streaming finished")
					if scannerErr != nil {
						return shared.NewErrorWithStatusCode(
							fmt.Errorf("upstream stream terminated with read error: %w", scannerErr),
							shared.ErrorCodeBadResponse, http.StatusBadGateway)
					}
					return nil
				}
			}
		case <-ping:
			if err := PingData(c); err != nil {
				log.LogError(c, "ping data error: "+err.Error())
				return nil
			}
			if common.DebugEnabled {
				println("ping data sent")
			}
		case <-timeout:
			log.LogError(c, "streaming timeout")
			return shared.NewErrorWithStatusCode(
				fmt.Errorf("upstream stream timeout after no data for %s", streamingTimeout),
				shared.ErrorCodeBadResponse, http.StatusGatewayTimeout)
		case <-c.Request.Context().Done():
			log.LogDebug(c, "client disconnected")
			return nil
		}
	}
}

func safeStreamDataHandler(c *gin.Context, dataHandler func(string) bool, data string) (ok bool) {
	ok = true
	defer func() {
		if r := recover(); r != nil {
			log.LogError(c, fmt.Sprintf("stream data handler panic: %v", r))
			ok = false
		}
	}()
	ok = dataHandler(data)
	return ok
}

func resetTimer(timer *time.Timer, timeout time.Duration) {
	if timer == nil || timeout <= 0 {
		return
	}
	if !timer.Stop() {
		select {
		case <-timer.C:
		default:
		}
	}
	timer.Reset(timeout)
}
