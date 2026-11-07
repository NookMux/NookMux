package app

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/NookMux/NookMux/internal/common"
	"github.com/NookMux/NookMux/internal/domain/shared"
	infradb "github.com/NookMux/NookMux/internal/infra/db"
	infralog "github.com/NookMux/NookMux/internal/infra/log"
)

var (
	Port         = flag.Int("port", 3000, "the listening port")
	PrintVersion = flag.Bool("version", false, "print version and exit")
	PrintHelp    = flag.Bool("help", false, "print help and exit")
	LogDir       = flag.String("log-dir", "./logs", "specify the log directory")
)

func printHelp() {
	fmt.Println("NookMux(Based OneAPI) " + common.Version + " - The next-generation LLM gateway and AI asset management system supports multiple languages.")
	fmt.Println("Original Project: OneAPI by JustSong - https://github.com/songquanpeng/one-api")
	fmt.Println("Maintainer: NookMux - https://github.com/NookMux/NookMux")
	fmt.Println("Usage: nookmux [--port <port>] [--log-dir <log directory>] [--version] [--help]")
}

func InitEnv() {
	flag.Parse()
	common.InitStartupTimezone()

	// VERSION env overrides the build-time git commit hash injected via -ldflags.
	// Use this to set a human-readable version string (e.g. "v1.2.3") if needed.
	envVersion := os.Getenv("VERSION")
	if envVersion != "" {
		common.Version = envVersion
	}

	if *PrintVersion {
		fmt.Println(common.Version)
		os.Exit(0)
	}

	if *PrintHelp {
		printHelp()
		os.Exit(0)
	}

	if os.Getenv("SESSION_SECRET") != "" {
		ss := os.Getenv("SESSION_SECRET")
		if ss == "random_string" {
			log.Println("WARNING: SESSION_SECRET is set to the default value 'random_string', please change it to a random string.")
			log.Println("警告：SESSION_SECRET被设置为默认值'random_string'，请修改为随机字符串。")
			log.Fatal("Please set SESSION_SECRET to a random string.")
		} else {
			common.SessionSecret = ss
		}
	}
	if os.Getenv("CRYPTO_SECRET") != "" {
		common.CryptoSecret = os.Getenv("CRYPTO_SECRET")
	} else {
		common.CryptoSecret = common.SessionSecret
		log.Println("WARNING: CRYPTO_SECRET not set, falling back to SESSION_SECRET. Multi-instance deployments must set CRYPTO_SECRET explicitly, otherwise signed URLs (/mcp/image|video) will be rejected across instances after restart.")
		log.Println("警告：未设置 CRYPTO_SECRET，将回落使用 SESSION_SECRET。多实例部署必须显式配置 CRYPTO_SECRET，否则重启后签名 URL（/mcp/image|video）会跨实例互拒。")
	}
	if os.Getenv("SQLITE_PATH") != "" {
		infradb.SQLitePath = os.Getenv("SQLITE_PATH")
	}
	if *LogDir != "" {
		var err error
		*LogDir, err = filepath.Abs(*LogDir)
		if err != nil {
			log.Fatal(err)
		}
		if _, err := os.Stat(*LogDir); os.IsNotExist(err) {
			err = os.Mkdir(*LogDir, 0777)
			if err != nil {
				log.Fatal(err)
			}
		}
	}
	// 日志目录由 app 层的 flag 持有，注入给 infra/log 供滚动重建时读取
	// （infra/log 不能反向 import app，否则成环）。
	infralog.Dir = *LogDir

	// Initialize variables from constants.go that were using environment variables
	common.DebugEnabled = os.Getenv("DEBUG") == "true"
	common.MemoryCacheEnabled = os.Getenv("MEMORY_CACHE_ENABLED") == "true"
	common.IsMasterNode = os.Getenv("NODE_TYPE") != "slave"
	common.PasswordLoginEncryptionEnabled = common.GetEnvOrDefaultBool("PASSWORD_LOGIN_ENCRYPTION_ENABLED", false)
	common.TLSInsecureSkipVerify = common.GetEnvOrDefaultBool("TLS_INSECURE_SKIP_VERIFY", false)
	if common.TLSInsecureSkipVerify {
		if tr, ok := http.DefaultTransport.(*http.Transport); ok && tr != nil {
			if tr.TLSClientConfig != nil {
				tr.TLSClientConfig.InsecureSkipVerify = true
			} else {
				tr.TLSClientConfig = common.NewInsecureTLSConfig()
			}
		}
	}

	// Parse requestInterval and set common.RequestInterval
	requestInterval, _ := strconv.Atoi(os.Getenv("POLLING_INTERVAL"))
	common.RequestInterval = time.Duration(requestInterval) * time.Second

	// Initialize variables with common.GetEnvOrDefault
	common.SyncFrequency = common.GetEnvOrDefault("SYNC_FREQUENCY", 60)
	common.BatchUpdateInterval = common.GetEnvOrDefault("BATCH_UPDATE_INTERVAL", 5)
	common.RelayTimeout = common.GetEnvOrDefault("RELAY_TIMEOUT", 0)
	// common.RelayMaxIdleConns caps the global idle connection pool. The default of
	// 200 is adequate for most deployments.
	common.RelayMaxIdleConns = common.GetEnvOrDefault("RELAY_MAX_IDLE_CONNS", 200)
	// Keep the per-host idle pool modest by default to avoid retaining warm TLS
	// connections in low-traffic deployments. High-concurrency gateways can raise
	// this via RELAY_MAX_IDLE_CONNS_PER_HOST.
	common.RelayMaxIdleConnsPerHost = common.GetEnvOrDefault("RELAY_MAX_IDLE_CONNS_PER_HOST", 32)
	common.RelayIdleConnTimeout = common.GetEnvOrDefault("RELAY_IDLE_CONN_TIMEOUT", 90)

	// Initialize string variables with common.GetEnvOrDefaultString
	common.GeminiSafetySetting = common.GetEnvOrDefaultString("GEMINI_SAFETY_SETTING", "BLOCK_NONE")
	common.CohereSafetySetting = common.GetEnvOrDefaultString("COHERE_SAFETY_SETTING", "NONE")

	// Initialize rate limit variables
	common.GlobalApiRateLimitEnable = common.GetEnvOrDefaultBool("GLOBAL_API_RATE_LIMIT_ENABLE", true)
	common.GlobalApiRateLimitNum = common.GetEnvOrDefault("GLOBAL_API_RATE_LIMIT", 180)
	common.GlobalApiRateLimitDuration = int64(common.GetEnvOrDefault("GLOBAL_API_RATE_LIMIT_DURATION", 180))

	common.GlobalWebRateLimitEnable = common.GetEnvOrDefaultBool("GLOBAL_WEB_RATE_LIMIT_ENABLE", true)
	common.GlobalWebRateLimitNum = common.GetEnvOrDefault("GLOBAL_WEB_RATE_LIMIT", 60)
	common.GlobalWebRateLimitDuration = int64(common.GetEnvOrDefault("GLOBAL_WEB_RATE_LIMIT_DURATION", 180))

	common.CriticalRateLimitEnable = common.GetEnvOrDefaultBool("CRITICAL_RATE_LIMIT_ENABLE", true)
	common.CriticalRateLimitNum = common.GetEnvOrDefault("CRITICAL_RATE_LIMIT", 20)
	common.CriticalRateLimitDuration = int64(common.GetEnvOrDefault("CRITICAL_RATE_LIMIT_DURATION", 20*60))

	common.SearchRateLimitEnable = common.GetEnvOrDefaultBool("SEARCH_RATE_LIMIT_ENABLE", true)
	common.SearchRateLimitNum = common.GetEnvOrDefault("SEARCH_RATE_LIMIT", 10)
	common.SearchRateLimitDuration = int64(common.GetEnvOrDefault("SEARCH_RATE_LIMIT_DURATION", 60))

	initConstantEnv()
}

func initConstantEnv() {
	shared.StreamingTimeout = common.GetEnvOrDefault("STREAMING_TIMEOUT", 300)
	shared.MaxFileDownloadMB = common.GetEnvOrDefault("MAX_FILE_DOWNLOAD_MB", 64)
	// For multimodal auto-convert-to-URL storage.
	shared.MaxImageUploadMB = common.GetEnvOrDefault("MAX_IMAGE_UPLOAD_MB", 64)
	shared.MaxVideoUploadMB = common.GetEnvOrDefault("MAX_VIDEO_UPLOAD_MB", 128)
	shared.StoredImagePoolMB = common.GetEnvOrDefault("STORED_IMAGE_POOL_MB", 512)
	shared.StoredVideoPoolMB = common.GetEnvOrDefault("STORED_VIDEO_POOL_MB", 1024) // 1GB
	shared.StreamScannerMaxBufferMB = common.GetEnvOrDefault("STREAM_SCANNER_MAX_BUFFER_MB", 8)
	// MaxRequestBodyMB 请求体最大大小（解压后），用于防止超大请求/zip bomb导致内存暴涨
	shared.MaxRequestBodyMB = common.GetEnvOrDefault("MAX_REQUEST_BODY_MB", 128)
	shared.AnonymousRequestBodyLimitKB = common.GetEnvOrDefault("ANONYMOUS_REQUEST_BODY_LIMIT_KB", 512)
	// MaxResponseBodyMB is kept for compatibility. Typed response caps below
	// keep common text/error paths much smaller while allowing larger vector or
	// media payloads where the protocol legitimately needs them.
	shared.MaxResponseBodyMB = common.GetEnvOrDefault("MAX_RESPONSE_BODY_MB", 16)
	shared.MaxTextResponseBodyMB = common.GetEnvOrDefault("MAX_TEXT_RESPONSE_BODY_MB", shared.MaxResponseBodyMB)
	shared.MaxEmbeddingResponseBodyMB = common.GetEnvOrDefault("MAX_EMBEDDING_RESPONSE_BODY_MB", 64)
	shared.MaxMediaResponseBodyMB = common.GetEnvOrDefault("MAX_MEDIA_RESPONSE_BODY_MB", 64)
	shared.MaxErrorResponseBodyMB = common.GetEnvOrDefault("MAX_ERROR_RESPONSE_BODY_MB", 4)
	shared.MaxModelListResponseBodyMB = common.GetEnvOrDefault("MAX_MODEL_LIST_RESPONSE_BODY_MB", 8)
	// ForceStreamOption 覆盖请求参数，强制返回usage信息
	shared.ForceStreamOption = common.GetEnvOrDefaultBool("FORCE_STREAM_OPTION", true)
	shared.CountToken = common.GetEnvOrDefaultBool("CountToken", true)
	shared.GetMediaToken = common.GetEnvOrDefaultBool("GET_MEDIA_TOKEN", true)
	shared.GetMediaTokenNotStream = common.GetEnvOrDefaultBool("GET_MEDIA_TOKEN_NOT_STREAM", false)
	shared.AzureDefaultAPIVersion = common.GetEnvOrDefaultString("AZURE_DEFAULT_API_VERSION", "2025-04-01-preview")
	shared.NotifyLimitCount = common.GetEnvOrDefault("NOTIFY_LIMIT_COUNT", 2)
	shared.NotificationLimitDurationMinute = common.GetEnvOrDefault("NOTIFICATION_LIMIT_DURATION_MINUTE", 10)
	// GenerateDefaultToken 是否生成初始令牌，默认关闭。
	shared.GenerateDefaultToken = common.GetEnvOrDefaultBool("GENERATE_DEFAULT_TOKEN", false)
	// 是否启用错误日志
	shared.ErrorLogEnabled = common.GetEnvOrDefaultBool("ERROR_LOG_ENABLED", false)

	// Initialize trusted redirect domains for URL validation
	trustedDomainsStr := common.GetEnvOrDefaultString("TRUSTED_REDIRECT_DOMAINS", "")
	var trustedDomains []string
	domains := strings.Split(trustedDomainsStr, ",")
	for _, domain := range domains {
		trimmedDomain := strings.TrimSpace(domain)
		if trimmedDomain != "" {
			// Normalize domain to lowercase
			trustedDomains = append(trustedDomains, strings.ToLower(trimmedDomain))
		}
	}
	shared.TrustedRedirectDomains = trustedDomains
}
