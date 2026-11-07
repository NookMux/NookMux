package app

import (
	"context"
	"errors"
	"fmt"
	"github.com/NookMux/NookMux/internal/app/webdist"
	"github.com/NookMux/NookMux/internal/common"
	"github.com/NookMux/NookMux/internal/httpapi/controller/channel"
	"github.com/NookMux/NookMux/internal/httpapi/middleware"
	"github.com/NookMux/NookMux/internal/httpapi/router"
	"github.com/NookMux/NookMux/internal/infra/redis"
	"github.com/NookMux/NookMux/internal/infra/runtime"
	"github.com/NookMux/NookMux/internal/infra/security"
	"github.com/NookMux/NookMux/internal/store/channel"
	"github.com/NookMux/NookMux/internal/store/db"
	"github.com/NookMux/NookMux/internal/store/option"
	"github.com/NookMux/NookMux/internal/store/usedata"
	"github.com/gin-contrib/sessions"
	"github.com/gin-contrib/sessions/cookie"
	"github.com/gin-gonic/gin"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"
)

func Run() int {
	startTime := time.Now()

	err := Bootstrap()
	if err != nil {
		common.FatalLog("failed to initialize resources: " + err.Error())
		return 1
	}

	common.SysLog("NookMux " + common.Version + " started")
	// 暴露历史遗留或异常写入造成的负值余额存量，避免超卖问题静默存在
	dbstore.CheckNegativeQuotaBalances()
	if os.Getenv("GIN_MODE") != "debug" {
		gin.SetMode(gin.ReleaseMode)
	}
	if common.DebugEnabled {
		common.SysLog("running in debug mode")
	}

	defer func() {
		err := dbstore.CloseDB()
		if err != nil {
			common.FatalLog("failed to close database: " + err.Error())
		}
	}()

	if redis.RedisEnabled {
		// for compatibility with old versions
		common.MemoryCacheEnabled = true
	}
	if common.MemoryCacheEnabled {
		common.SysLog("memory cache enabled")
		common.SysLog(fmt.Sprintf("sync frequency: %d seconds", common.SyncFrequency))

		// Add panic recovery and retry for InitChannelCache
		func() {
			defer func() {
				if r := recover(); r != nil {
					common.SysLog(fmt.Sprintf("InitChannelCache panic: %v, retrying once", r))
					// Retry once
					_, _, fixErr := channelstore.FixAbility()
					if fixErr != nil {
						common.FatalLog(fmt.Sprintf("InitChannelCache failed: %s", fixErr.Error()))
					}
				}
			}()
			channelstore.InitChannelCache()
		}()

		go channelstore.SyncChannelCache(common.SyncFrequency)
	}

	channelstore.InitDynamicRatioCache()

	// 动态倍率缓存同步
	go channelstore.SyncDynamicRatioCache(common.SyncFrequency)

	// 设置获取 relay 并发数的函数指针
	common.GetActiveConnectionsFunc = middleware.GetActiveConnectionCount

	// 热更新配置
	go optionstore.SyncOptions(common.SyncFrequency)

	// 数据看板
	go usedatastore.UpdateQuotaData()

	if os.Getenv("CHANNEL_UPDATE_FREQUENCY") != "" {
		frequency, err := strconv.Atoi(os.Getenv("CHANNEL_UPDATE_FREQUENCY"))
		if err != nil {
			common.FatalLog("failed to parse CHANNEL_UPDATE_FREQUENCY: " + err.Error())
		}
		go channelcontroller.AutomaticallyUpdateChannels(frequency)
	}

	go channelcontroller.AutomaticallyTestChannels()

	if os.Getenv("BATCH_UPDATE_ENABLED") == "true" {
		common.BatchUpdateEnabled = true
		common.SysLog("batch update enabled with interval " + strconv.Itoa(common.BatchUpdateInterval) + "s")
		dbstore.InitBatchUpdater()
	}

	if os.Getenv("ENABLE_PPROF") == "true" {
		go runtime.EnablePprofServer()
		go runtime.Monitor()
		common.SysLog("pprof enabled")
	}

	err = runtime.StartPyroScope()
	if err != nil {
		common.SysError(fmt.Sprintf("start pyroscope error : %v", err))
	}

	// Initialize HTTP server
	server := gin.New()
	security.SetupGinTrustedProxies(server)
	server.Use(gin.CustomRecovery(func(c *gin.Context, err any) {
		common.SysLog(fmt.Sprintf("panic detected: %v", err))
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": gin.H{
				"message": fmt.Sprintf("Panic detected, error: %v. Please submit a issue here: https://github.com/NookMux/NookMux", err),
				"type":    "nookmux_panic",
			},
		})
	}))
	// This will cause SSE not to work!!!
	//server.Use(gzip.Gzip(gzip.DefaultCompression))
	server.Use(middleware.RequestId())
	middleware.SetUpLogger(server)
	// Initialize session store
	store := cookie.NewStore([]byte(common.SessionSecret))
	store.Options(sessions.Options{
		Path:     "/",
		MaxAge:   2592000, // 30 days
		HttpOnly: true,
		// HTTPS 部署必须启用 COOKIE_SECURE，防止 30 天长期会话 cookie 经明文 HTTP 信道泄露
		Secure:   common.GetEnvOrDefaultBool("COOKIE_SECURE", false),
		SameSite: http.SameSiteStrictMode,
	})
	server.Use(sessions.Sessions("session", store))

	indexPage := InjectUmamiAnalytics(webdist.IndexPage)
	indexPage = InjectGoogleAnalytics(indexPage)

	// 设置路由
	router.SetRouter(server, router.WebAssets{
		BuildFS:   webdist.BuildFS,
		IndexPage: indexPage,
	})
	port := os.Getenv("PORT")
	if port == "" {
		port = strconv.Itoa(*Port)
	}

	// Log startup success message
	common.LogStartupSuccess(startTime, port)

	srv := &http.Server{Addr: ":" + port, Handler: server}
	serveErr := make(chan error, 1)
	go func() {
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			serveErr <- err
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	common.SysLog("press Ctrl+C to stop the server")
	select {
	case err := <-serveErr:
		common.FatalLog("failed to start HTTP server: " + err.Error())
		return 1
	case <-quit:
		common.SysLog("received exit signal, shutting down gracefully")
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		common.SysError("HTTP server shutdown: " + err.Error())
	}
	// 流量停止后把批量暂存的统计增量（已用额度、请求计数）落库，
	// 避免退出时丢失窗口内的数据。
	dbstore.FlushBatchUpdates()

	return 0
}
