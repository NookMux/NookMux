package app

import (
	"github.com/NookMux/NookMux/internal/common"
	"github.com/NookMux/NookMux/internal/config/dashboard"
	_ "github.com/NookMux/NookMux/internal/config/performance"
	"github.com/NookMux/NookMux/internal/config/ratio"
	"github.com/NookMux/NookMux/internal/i18n"
	"github.com/NookMux/NookMux/internal/infra/cache"
	httpclient "github.com/NookMux/NookMux/internal/infra/httpclient"
	"github.com/NookMux/NookMux/internal/infra/log"
	"github.com/NookMux/NookMux/internal/infra/redis"
	"github.com/NookMux/NookMux/internal/infra/runtime"
	tokenizer "github.com/NookMux/NookMux/internal/infra/tokenizer"
	"github.com/NookMux/NookMux/internal/store/db/migrate"
	"github.com/NookMux/NookMux/internal/store/loginkey"
	"github.com/NookMux/NookMux/internal/store/option"
	"github.com/NookMux/NookMux/internal/store/pricing"
	"github.com/joho/godotenv"
	"strings"
)

func Bootstrap() error {
	// Initialize resources here if needed
	// This is a placeholder function for future resource initialization
	err := godotenv.Load(".env")
	if err != nil {
		if common.DebugEnabled {
			common.SysLog("No .env file found, using default environment variables. If needed, please create a .env file and set the relevant variables.")
		}
	}

	// 加载环境变量
	InitEnv()

	log.SetupLogger()

	// Initialize model settings
	ratio.InitRatioSettings()

	httpclient.InitHttpClient()

	tokenizer.InitTokenEncoders()

	// Initialize SQL Database
	err = dbmigrate.InitDB()
	if err != nil {
		common.FatalLog("failed to initialize database: " + err.Error())
		return err
	}

	optionstore.CheckSetup()

	// Initialize options, should after dbmigrate.InitDB()
	optionstore.InitOptionMap()

	// 登录密码传输加密（opt-in）：开启时加载/生成服务端 RSA 密钥，失败快
	// 速失败，不做静默降级。
	if common.PasswordLoginEncryptionEnabled {
		if err := loginkeystore.InitPasswordEncryptionKey(); err != nil {
			common.FatalLog("failed to initialize password encryption: " + err.Error())
			return err
		}
	}

	// 迁移 console_setting 面板开关到 dashboard_config（一次性，幂等）
	// 失败仅记日志不阻断启动，console_setting 仍可作为 fallback 直到双源统一
	if err := dashboard.MigrateFromConsoleSetting(); err != nil {
		common.SysError("dashboard config migration failed: " + err.Error())
	}

	// 清理旧的磁盘缓存文件
	cache.CleanupOldCacheFiles()

	// 初始化模型
	pricingstore.GetPricing()

	// Initialize SQL Database
	err = dbmigrate.InitLogDB()
	if err != nil {
		return err
	}

	// Initialize Redis
	err = redis.InitRedisClient()
	if err != nil {
		return err
	}

	// 启动系统监控
	runtime.StartSystemMonitor()

	// Initialize i18n
	err = i18n.Init()
	if err != nil {
		common.SysError("failed to initialize i18n: " + err.Error())
		// Don't return error, i18n is not critical
	} else {
		common.SysLog("i18n initialized with languages: " + strings.Join(i18n.SupportedLanguages(), ", "))
	}

	return nil
}
